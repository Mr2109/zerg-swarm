// family_fs_tools.go —— `zerg fs` 族三条**正门**：`fs read` / `fs write` / `fs run`（缺口 `GAP-20261017-10` · P0）。
//
// 动因（缺口正文逐字）：对话面要撤掉 `bash`/`read`/`write`/`edit`，但虫族 CLI 当时**没有对应正门** ——
// `zerg run` / `exec` / `shell` / `sh` 四条全是未知命令（rc=2），`fs` 族只有 `unprotected` 一个动作
// （`fs read` / `fs write` 报「是族名/中间节点」）⇒ **先造三条正门，才撤得动对话面**。
//
// 三条规格（逐字落在这里 = 本件的判据面）：
//
//	`fs read`   只读 · 人面出带行号文本（同 `code show` 的渲染面）· `--json` 字段闭集
//	            `path`/`content`/`total_lines`/`offset`/`limit` · 件不存在 ⇒ rc=2（不给结论）· 危险档 0
//	`fs write`  D2（照 `script inventory sync` 那一套：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 ·
//	            写面进审计 `edit_audit.jsonl`）· 禁绝对路径出仓 · 禁符号链接 · 父目录不存在 ⇒ 2（✗ 不自建）
//	            · `--replace <补丁件>`（`GAP-20261018-09`）：**定点替换**，每格 `old` 恰好命中 1 次才改
//	            （0 次 / 多次 ⇒ rc=2 具名）—— 走这条路**不必先把全文读进上下文**（拼全文改一行正是死锁点）
//	`fs run`    D3（最高 · **fail-closed**）：首词硬白名单只许 `zerg`/`go`/`git`/`make`/`npm`/`cargo` 六枚
//	            ⇒ 其余一律 rc=2 拒执 · ✗ **不经 shell 解释一个字**（argv 直投）· `--cwd` 必须在仓内 ·
//	            超时缺省 **120s** · 输出超 **64KB** 截断并标截断
//
// ★ 三条都**不另立第二套口径**：仓根走 `repoRoot()`（唯一真源）· 出仓判据走 `pathWithinDir()`（路径段口径，
// 与 `code find --path` 同源）· 两道确认档自相矛盾走 `dryRunYesConflict()`（已签共享判定）·
// 审计落点走 `editAuditPath()`（`dev edit` 同一件 · 三级优先）—— 一条都不重抄式子。
//
// ★ 审计事件名**自己的**（`fs_file_written`）：`edit_audit.jsonl` 里既有那几格讲的是「改件」
// （`before_sha256`/`after_sha256`）与 `gap_ledger_written`（`gap_*` 九格）⇒ 本件按同一条先例
// **只增自己的字段名**，既有的读面（`event == "edit"` / `event == "approve_signed"` 各自过滤）一位不受影响。

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── `--json` 字段表（**闭集**：表外的键一律由 `emitSelected` 逐字拒收，退码 2）───────────────
//
// ★ 三张表是**唯一真源**：命令树 `fields:` 引用它们（帮助面 / `requireFields` 的「可选字段」行 /
// 收受端白名单三处同读这一份）⇒ ✗ 不在第二处再抄一遍字段名。
var (
	fsReadFields  = []string{"path", "content", "total_lines", "offset", "limit"}
	fsWriteFields = []string{"path", "result", "bytes", "before_sha256", "after_sha256", "append", "audit_path"}
	fsRunFields   = []string{"cmd", "cwd", "timeout_s", "rc", "bytes", "truncated", "output"}
)

const (
	// fsRunDefaultTimeoutS —— `fs run` 的墙钟上界缺省（规格逐字：**120 秒**）。
	fsRunDefaultTimeoutS = 120
	// fsRunOutputCap —— 输出截断上界（规格逐字：**64KB** · 超即截断并标截断）。
	fsRunOutputCap = 64 << 10
	// fsReadOffsetDefault —— `--offset` 缺省 = 第 1 行（1 起）。
	fsReadOffsetDefault = 1
	// fsReadDefaultMaxLines —— `fs read` 的**缺省窗口**（GAP-20261018-08）：缺省只读这么多行。
	//
	// 病灶（对话里真撞 · 已亲核）：缺省「读整件」⇒ 一篇 400+ 行 md 加上后续内容就把模型 ctx 顶爆
	// （连续两次 `system_warning: Input tokens have exceeded the limit`），而正文里同时出现**未解析的
	// 工具调用原文**（模型拿不到结果 ⇒ 同一调用重发两次），两篇文档一篇未改。
	//
	// 定 2000 行（与 `tool_output.max_lines` 同量级）—— 两个硬要求：
	//  ① **不静默截断**：超窗口时最后一行逐字报「已截断：显示 a–b / 共 N 行，续读用 --offset b+1」
	//     （✗ 不许只说「已截断」—— 模型拿不到续读起点就会重发同一条调用，正是本 GAP 的原始病灶）；
	//  ② **显式即授权**：`--limit N` 显式给值 ⇒ 按 N（**不封顶**，✗ 不偷偷再夹一次）。
	fsReadDefaultMaxLines = 2000
)

// fsRunHeadAllow —— `fs run` 的**首词硬白名单**（规格逐字六枚 · fail-closed：不在表里一律 rc=2）。
// ★ 这是一张**闭集**：新增一枚 = 改规格，✗ 不许「顺手放行」。
var fsRunHeadAllow = []string{"zerg", "go", "git", "make", "npm", "cargo"}

// fsRunSubAllow —— 首词的**子命令闭集**（第二层 fail-closed）。
//
// 规格逐字：`go run` / `git push` / `git reset --hard` 这类**单独禁** ⇒ 先只开
// `go build|test|vet|fmt` 与 `git status|log|diff|show|cat-file|ls-files`。
// 其余四枚同一种形态给闭集（✗ 不许「首词对了就全放」）：
//
//	`zerg`  —— 只开两条**只读自证面**（`version` / `help`）：本命令的用途就是「跑仓内正门」，
//	          但把整个 `zerg` 命令树当可跑面会把 `repo commit`/`dev edit` 这类写面绕成一条 argv
//	          ⇒ 首版只开这两条，放开是下一版的规格变更（先拍后开）。
//	`make`  —— 只开构建/判据类靶子（`-n`/`--dry-run` 是干跑靶）。
//	`npm`   —— 只开 `ls`/`test`（`npm install`/`publish` 一律拒）。
//	`cargo` —— 只开 `--version`/`version`/`check`/`build`/`test`/`fmt`/`tree`（`publish`/`install` 拒）。
var fsRunSubAllow = map[string][]string{
	"zerg":  {"version", "help"},
	"go":    {"build", "test", "vet", "fmt"},
	"git":   {"status", "log", "diff", "show", "cat-file", "ls-files"},
	"make":  {"-n", "--dry-run", "-C", "build", "test", "check", "vet", "fmt", "help", "--version"},
	"npm":   {"ls", "test", "--version"},
	"cargo": {"--version", "version", "check", "build", "test", "fmt", "tree"},
}

// fsToolRoot —— 三条正门共用的仓根解析（`repoRoot()` = 唯一真源）。
// 取不到 ⇒ rc=8（不给结论：判不了「件在不在仓内」就一个字节都不动）。
func fsToolRoot(inv *invocation, stderr io.Writer) (string, int) {
	root := repoRoot()
	if strings.TrimSpace(root) == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 判不了仓内/仓外（**不给结论** · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return "", exitBlocked
	}
	return root, exitOK
}

// ── GAP-20261018-07（2026-10-18）：兄弟仓读写正门 `--allow-sibling-repo <仓名>` ──────────────────
//
// 病灶（现读 · 对话里真撞）：目标件在**仓根的兄弟目录** `Zerg-内部文档/`（`../Zerg-内部文档/项目文档/…md`）时，
// `fs read` / `fs write` 一律 rc=2「件出仓了 ⇒ ✗ 只碰本仓工作树」⇒ **文档面在对话里既读不到原文、
// 也写不回去**（判红看得见、原文够不着），只能由人手工搬。
//
// 本条照 `dev edit --allow-sibling-repo <名>` 的既有先例（`GAP-20261001-07` 甲案 · `family_dev_edit.go`）
// 开一道**正门**，口径逐条对齐：
//
//	· 旗标**只收仓名、不收路径**；名必须**逐字** ∈ `devEditSiblingRepos` —— 那是**同一份编译期闭集**
//	  （✗ 不另立第二张白名单）。给路径形态（`../Zerg-内部文档`）/ 别家仓名 / 空名 ⇒ rc=2 且**具名**报
//	  「非白名单兄弟仓」。
//	· **默认态逐字不变**：不给旗标 ⇒ `fsToolResolvePath` 那条旧判据、退码、文案一位不动（仍 fail-closed）。
//	· 给旗标 ⇒ 解析根换成 `<仓根>/../<仓名>`（= `filepath.Join(repoRoot, "..", 仓名, rel)`），且对**解析结果**
//	  再校一次 `filepath.Rel(兄弟仓根, 结果)`：以 `..` 开头 / 是绝对路径 ⇒ rc=2（✗ 不许
//	  `--allow-sibling-repo Zerg-内部文档 ../../etc/passwd` 从兄弟仓再绕出去）；另加「路径段版」`pathWithinDir` 复核。
//	· 兄弟仓根不在盘上 ⇒ rc=8（**不给结论** · 与 `dev edit` 同码同判词）。
//	· 人面落点写成 `<仓名>/<仓内相对>`（`Zerg-内部文档/项目文档/…`）；**机器面 `path` 仍是兄弟仓内相对路径**
//	  （与 `dev edit` 的 `File` 同口径 ⇒ 回读时原样喂回这个相对路径即可），兄弟仓名与根进审计行。
var fsToolSiblingRepos = devEditSiblingRepos // 唯一真源：与 `dev edit` 同一份闭集（✗ 不抄第二份）

// fsToolSibling —— 解析 `--allow-sibling-repo <仓名>`。
//
// 返回 (仓名, 兄弟仓根, rc)。**不给旗标 ⇒ ("","",exitOK) 且零输出** —— 调用方随后走的仍是旧那条
// 仓内判据，所以默认态的输出/退码逐字不变（这条是 GAP 的硬要求）。
func fsToolSibling(inv *invocation, stderr io.Writer, root string) (string, string, int) {
	if !inv.hasFlag("--allow-sibling-repo") {
		return "", "", exitOK
	}
	name := strings.TrimSpace(inv.flagVal("--allow-sibling-repo"))
	if !devEditSiblingRepoKnown(name) {
		inv.setErr("usage", "unknown_sibling_repo", "同级仓名不在编译期闭集里")
		fmt.Fprintf(stderr, "%s: ★ **拒执** —— `--allow-sibling-repo %s`：**非白名单兄弟仓**（只认 %s · 只收仓名、✗ 不收路径 —— 路径形态一律拒）⇒ 退码 2（GAP-20261018-07）\n",
			progName, name, strings.Join(fsToolSiblingRepos, " / "))
		return "", "", exitUsage
	}
	dir := filepath.Join(filepath.Dir(root), name)
	if st, serr := os.Stat(dir); serr != nil || !st.IsDir() {
		inv.setErr("blocked", "sibling_root_absent", "同级仓根不在盘上")
		fmt.Fprintf(stderr, "%s: 兄弟仓 %s 的根不在盘上（%s）⇒ 不给结论（退码 8 · 不猜 · GAP-20261018-07）\n",
			progName, name, dir)
		return "", "", exitBlocked
	}
	return name, dir, exitOK
}

// fsToolLabel —— 人面用的**可读落点**（兄弟仓 ⇒ `Zerg-内部文档/…`；本仓 ⇒ 原样 rel）。
// ★ 只动**人面**；机器面 `path` 走 `rel`（兄弟仓内相对 · 与 `dev edit` 的 `File` 同口径）。
func fsToolLabel(sibName, rel string) string {
	if sibName == "" {
		return rel
	}
	return sibName + "/" + rel
}

// fsToolResolvePathIn —— 换根版解析：`sibName == ""` ⇒ **逐字**转给 `fsToolResolvePath`（旧路一位不动）。
//
// 给了兄弟仓名时三种写法都收，但**一律再判一次「解析结果在不在兄弟仓根下」**：
//
//	`项目文档/x.md`          —— 相对**兄弟仓根**（主用法 · 与 `dev edit --file` 同口径）
//	`../Zerg-内部文档/项目文档/x.md` —— 相对**主仓根**的旧写法（人已经习惯这么敲 · 收到就能用）
//	`/abs/…/Zerg-内部文档/项目文档/x.md` —— 绝对路径（含空格也能用 —— argv 是一枚实参）
//
// 越界判据 = `filepath.Rel` 结果不以 `..` 开头 + 非绝对 + 路径段版 `pathWithinDir`（两道都过才收）。
func fsToolResolvePathIn(inv *invocation, stderr io.Writer, root, sibName, sibRoot, raw string) (string, string, int) {
	if sibName == "" {
		return fsToolResolvePath(inv, stderr, root, raw)
	}
	s := strings.TrimSpace(raw)
	if s == "" {
		inv.setErr("usage", "path_required", "缺件路径")
		fmt.Fprintf(stderr, "%s: 要给一个件路径（仓根相对；✗ 空串）⇒ 退码 2\n", progName)
		return "", "", exitUsage
	}
	clean := filepath.Clean(filepath.FromSlash(s))
	var cand string
	switch {
	case filepath.IsAbs(clean):
		cand = clean
	case clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)):
		// 「相对主仓根」的旧写法：先按主仓根拼，落点判据在下面统一判（✗ 这里不算放行）。
		cand = filepath.Clean(filepath.Join(root, clean))
	default:
		cand = filepath.Clean(filepath.Join(sibRoot, clean))
	}
	rel, rerr := filepath.Rel(sibRoot, cand)
	outside := rerr != nil || filepath.IsAbs(rel) ||
		rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	if outside || !pathWithinDir(sibRoot, cand) {
		inv.setErr("usage", "file_outside_repo", "件不在同级仓根下")
		fmt.Fprintf(stderr, "%s: 件**越出**兄弟仓 %s 的根（%s → %s）⇒ 退码 2（`--allow-sibling-repo` 只开这一棵兄弟树 · ✗ 不许 `..` 绕出 · GAP-20261018-07）\n",
			progName, sibName, s, cand)
		return "", "", exitUsage
	}
	return cand, filepath.ToSlash(rel), exitOK
}

// fsToolResolvePath —— 把用户给的件路径解析成 (绝对路径, 仓内相对路径)。
//
// 出仓判据**按路径段**判（`pathWithinDir` · 与 `code find --path` 同一条口径）：
// 字符串前缀会把 `…/模型类/Zerg-内部文档` 当成 `…/模型类/Zerg` 的「仓内」—— 那正是 `W-03` 的病根。
// ★ 绝对路径**不出仓**才收（`fs write` 的规格逐字：「禁绝对路径出仓」）；相对路径 `..` 逃逸同样 2。
func fsToolResolvePath(inv *invocation, stderr io.Writer, root, raw string) (string, string, int) {
	s := strings.TrimSpace(raw)
	if s == "" {
		inv.setErr("usage", "path_required", "缺件路径")
		fmt.Fprintf(stderr, "%s: 要给一个件路径（仓根相对；✗ 空串）⇒ 退码 2\n", progName)
		return "", "", exitUsage
	}
	clean := filepath.Clean(filepath.FromSlash(s))
	abs := clean
	if !filepath.IsAbs(clean) {
		abs = filepath.Clean(filepath.Join(root, clean))
	}
	if !pathWithinDir(root, abs) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2（✗ 只碰本仓工作树）\n", progName, s, root)
		return "", "", exitUsage
	}
	rel, rerr := filepath.Rel(root, abs)
	if rerr != nil {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件路径解析不出来（%s）：%v ⇒ 退码 2\n", progName, s, rerr)
		return "", "", exitUsage
	}
	return abs, filepath.ToSlash(rel), exitOK
}

// fsToolSHA256 —— 整件 sha256（十六进制全长 · 审计与 `--json` 共用一处算法）。
func fsToolSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fsToolSHA16 —— 人面读法（前 16 位 · 与仓内 `sha16` 体例一致）。
func fsToolSHA16(hexFull string) string {
	if len(hexFull) <= 16 {
		return hexFull
	}
	return hexFull[:16]
}

// fsToolIsText —— 文本判据（非 UTF-8 ⇒ 不猜行号 · 与 `code show` 同一条口径）。
func fsToolIsText(b []byte) bool { return utf8.Valid(b) }

// fsToolSplitLines —— 分行（行尾换行不算一个新行 · 与 `code show` 整件读同一条口径）。
func fsToolSplitLines(body string) []string {
	lines := strings.Split(body, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// ── `fs read` ────────────────────────────────────────────────────────────────

// cmdFSRead —— `zerg fs read <件> [--offset N] [--limit N] [--json <字段>]`：**只读**。
//
// 退码三态（判据面）：
//
//	0 —— 读出来了（人面带行号 / 机器面闭集五格）
//	2 —— **不给结论**：用法错 / 件不存在 / 件出仓 / 点是目录 / 窗口越界（✗ 不假绿 · 不冒充「空件」）
//	8 —— 判不了：仓根解析不到 / 件读不动 / 件不是文本
//
// ★ 只读 ⇒ 危险档 0（命令树里 `danger` 为空）· 零写盘 · 不落审计。
//
// ★ GAP-20261018-08（2026-10-18）：**缺省不再读整件** —— 缺省窗口 `fsReadDefaultMaxLines`（2000 行），
// 超窗口时**不静默截断**：人面末行逐字给「已截断：显示 a–b / 共 N 行，续读用 --offset b+1」，
// 机器面 `limit` 报**实际生效**的窗口（拿它与 `total_lines`/`offset` 一比就知道后面还有没有）。
// `--limit N` 显式给值 ⇒ 按 N（不封顶）。
func cmdFSRead(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 1 {
		inv.setErr("usage", "bad_arity", "`fs read` 只收一个件路径")
		fmt.Fprintf(stderr, "%s: `fs read` 只收**一个**件路径（收到 %d 枚）⇒ 退码 2\n", progName, len(inv.args))
		fmt.Fprintf(stderr, "用法：%s fs read <件> [--offset N] [--limit N] [--allow-sibling-repo <名>] [--json <字段>]\n", progName)
		return exitUsage
	}
	root, rc := fsToolRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	// ★ GAP-20261018-07：不给 `--allow-sibling-repo` ⇒ 这一支零输出、旧判据一位不动。
	sibName, sibRoot, rc := fsToolSibling(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	abs, rel, rc := fsToolResolvePathIn(inv, stderr, root, sibName, sibRoot, inv.args[0])
	if rc != exitOK {
		return rc
	}
	label := fsToolLabel(sibName, rel)
	st, serr := os.Stat(abs)
	if serr != nil {
		// 规格逐字：件不存在 ⇒ rc=2 **不给结论**（✗ 不当成「空件 read 成功」）。
		inv.setErr("usage", "file_absent", "件不存在")
		fmt.Fprintf(stderr, "%s: 件不存在：%s ⇒ **不给结论**（退码 2 · ✗ 不把「没有」读成「空件」）\n", progName, label)
		return exitUsage
	}
	if st.IsDir() {
		inv.setErr("usage", "target_is_dir", "点的是目录")
		fmt.Fprintf(stderr, "%s: 点的是目录、不是件：%s ⇒ 退码 2\n", progName, label)
		return exitUsage
	}
	body, rerr := os.ReadFile(abs)
	if rerr != nil {
		inv.setErr("blocked", "file_unreadable", "件读不动")
		fmt.Fprintf(stderr, "%s: 件读不动（%v）⇒ 不给结论（退码 8）\n", progName, rerr)
		return exitBlocked
	}
	if !fsToolIsText(body) {
		inv.setErr("blocked", "not_text", "件不是文本")
		fmt.Fprintf(stderr, "%s: 件不是文本（非 UTF-8 ⇒ 不猜它的行号）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	lines := fsToolSplitLines(string(body))
	total := len(lines)
	offset := fsReadOffsetDefault
	if inv.hasFlag("--offset") {
		n, perr := strconv.Atoi(strings.TrimSpace(inv.flagVal("--offset")))
		if perr != nil || n < 1 {
			inv.setErr("usage", "bad_offset", "--offset 要是正整数（1 起）")
			fmt.Fprintf(stderr, "%s: `--offset` 要是**正整数**（行号 1 起）：%q ⇒ 退码 2\n", progName, inv.flagVal("--offset"))
			return exitUsage
		}
		offset = n
	}
	limit := 0 // 0 = 未显式给（≠「到件尾」：见下面 effLimit）
	if inv.hasFlag("--limit") {
		n, perr := strconv.Atoi(strings.TrimSpace(inv.flagVal("--limit")))
		if perr != nil || n < 1 {
			inv.setErr("usage", "bad_limit", "--limit 要是正整数")
			fmt.Fprintf(stderr, "%s: `--limit` 要是**正整数**：%q ⇒ 退码 2\n", progName, inv.flagVal("--limit"))
			return exitUsage
		}
		limit = n
	}
	// ★ GAP-20261018-08：缺省窗口 = fsReadDefaultMaxLines（**不再是「到件尾」**）；
	//   显式 `--limit N` 按 N（显式即授权 ⇒ 不封顶 · 也不拿缺省去二次夹）。
	effLimit := fsReadDefaultMaxLines
	if inv.hasFlag("--limit") {
		effLimit = limit
	}
	if offset > total && total > 0 {
		inv.setErr("usage", "window_out_of_range", "窗口越界")
		fmt.Fprintf(stderr, "%s: `--offset %d` 越过件尾（共 %d 行）⇒ **不给结论**（退码 2 · ✗ 不冒充空页）\n",
			progName, offset, total)
		return exitUsage
	}
	start := offset - 1
	if start < 0 {
		start = 0
	}
	end := total
	if effLimit > 0 && start+effLimit < end {
		end = start + effLimit
	}
	truncated := end < total
	window := lines[start:end]
	// ★ 不静默截断（GAP-20261018-08）：stderr 始终点明「被夹了、夹到哪」——
	//   机器面（`--json`）不带这一句（stdout 必须是纯 JSON），它靠 `total_lines` / `offset` / `limit`
	//   三格自己算得出来；人面则在**末行**逐字给出续读命令（模型看到的就是 stdout）。
	if truncated {
		fmt.Fprintf(stderr, "%s: ★ **已截断** —— 显示 %d–%d 行 / 共 %d 行（缺省窗口 %d 行 · 续读 `--offset %d`）\n",
			progName, offset, end, total, fsReadDefaultMaxLines, end+1)
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		row := map[string]string{
			"path":        rel,
			"content":     strings.Join(window, "\n"),
			"total_lines": strconv.Itoa(total),
			"offset":      strconv.Itoa(offset),
			"limit":       strconv.Itoa(effLimit),
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row})
	}
	// 人面：**带行号文本**（与 `code show` 同一个渲染器 / 同一对列名 ⇒ 两个面不会长歪）。
	fmt.Fprintf(stderr, "%s: %s 第 %d 行起 %d 行（共 %d 行）\n", progName, label, offset, len(window), total)
	table := make([][]string, 0, len(window))
	for i, ln := range window {
		table = append(table, []string{strconv.Itoa(start + i + 1), ln})
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, []string{"line", "text"}, table)
	if truncated {
		// ★ 末行三件：已截断 ／ 显示范围与总行数 ／ **续读命令**（逐字可照抄）。
		fmt.Fprintf(stdout, "已截断：显示 %d–%d / 共 %d 行，续读用 --offset %d（显式 `--limit N` 可放宽窗口）\n",
			offset, end, total, end+1)
	}
	return exitOK
}

// ── `fs write` ───────────────────────────────────────────────────────────────

// fsWriteAuditLine —— `event=fs_file_written` 的一行（**自己的**字段名 · 缺必需格 ⇒ 这一行不许写）。
//
// ★ GAP-20261018-07（只增两格 · 均 `omitempty`）：写**兄弟仓**时明写「哪个仓 + 它的根」——
// 写兄弟仓这种事必须留痕（`FsPath` 仍是兄弟仓内相对路径 ⇒ 绝对落点 = `FsSiblingRoot/FsPath`）。
// 不给旗标 ⇒ 两格都不写 ⇒ 既有行 / 历史行逐字节不变（只增键，既有键不动）。
type fsWriteAuditLine struct {
	At         string `json:"at"`
	Event      string `json:"event"`
	FsCmd      string `json:"fs_cmd"`
	FsPath     string `json:"fs_path"`
	FsBefore   string `json:"fs_before_sha256"`
	FsAfter    string `json:"fs_after_sha256"`
	FsBytes    int    `json:"fs_bytes"`
	FsAppend   bool   `json:"fs_append"`
	FsExisted  bool   `json:"fs_existed"`
	By         string `json:"by"`
	Confirm    string `json:"confirm"`
	RollbackTo string `json:"rollback_hint"`
	Sibling    string `json:"sibling_repo,omitempty"`
	SiblingAt  string `json:"sibling_root,omitempty"`
}

// fsWriteAppendAudit —— 追加一行（`O_APPEND` · 历史行逐字节不变 · **失败即拒**）。
func fsWriteAppendAudit(path string, l fsWriteAuditLine) error {
	if strings.TrimSpace(l.At) == "" || strings.TrimSpace(l.Event) == "" ||
		strings.TrimSpace(l.FsPath) == "" || strings.TrimSpace(l.FsAfter) == "" {
		return fmt.Errorf("审计行缺必需格（at/event/fs_path/fs_after_sha256）⇒ 这一行不许写")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// cmdFSWrite —— `zerg fs write <件> [--content <文本> | --content-stdin | --replace <补丁件>] [--append]
// [--by <谁>] [--json <字段>] [--dry-run | --yes]`：**D2 写面**（照 `script inventory sync` 那一套）。
//
// 三段式（与 `script inventory sync` 逐条同形）：
//
//	`--dry-run`  ⇒ 只出计划件 · **零副作用**（不写件 / 不落审计）· 退 0
//	缺 `--yes`   ⇒ 计划件 + **不执行** · 退 2（fail-closed · 从不提问）
//	`--yes`      ⇒ 审计先落盘 → 同目录临时件 + rename 原子写 → 写后读回对拍 → 不等即逐字节写回
//
// 内容来源三选一（✗ 不许同给）：
//
//	`--content <文本>` / `--content-stdin` —— **整件写**（覆盖 / `--append` 追加）
//	`--replace <补丁件>` —— **定点替换**（`GAP-20261018-09`）：补丁件 = `[{"old":…,"new":…}]`
//	  （与 `dev edit --replace` **同一形态、同一把尺**：每格 `old` 必须恰好命中 1 次，
//	  0 次 / 多次 ⇒ rc=2 具名拒）。★ 这条路**不必先把全文读进上下文** —— 死锁点就在这里：
//	  整件覆盖要求「完整原文 + 只改一行」，而读面有两层截断（2000 行 / 8000 字符）。
//
// 三条硬禁（规格逐字 · 全在**唯一写盘点之前**判掉）：
//
//	① 绝对路径出仓 / `..` 逃逸 ⇒ 2（`fsToolResolvePath`）
//	② 目标是符号链接（含父目录经符号链接出仓）⇒ 2
//	③ 父目录不存在 ⇒ 2（**✗ 不自建** —— 自建目录是第二个写面）
func cmdFSWrite(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) != 1 {
		inv.setErr("usage", "bad_arity", "`fs write` 只收一个件路径")
		fmt.Fprintf(stderr, "%s: `fs write` 只收**一个**件路径（收到 %d 枚）⇒ 退码 2\n", progName, len(inv.args))
		fmt.Fprintf(stderr, "用法：%s fs write <件> [--content <文本> | --content-stdin | --replace <补丁件>] [--append] [--by <谁>] [--allow-sibling-repo <名>] [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	contentGiven := inv.hasFlag("--content")
	stdinGiven := inv.hasFlag("--content-stdin")
	replaceGiven := inv.hasFlag("--replace")
	// ★ GAP-20261018-09（2026-10-18）：`--replace <补丁件>` = **第三种内容来源**（定点替换）。
	//
	// 病灶（对话里真撞 · 已亲核）：`fs write` 是**整件覆盖** ⇒ 改一行必须先拿到全文；而读面有两层截断
	// （`fs read` 缺省 2000 行 ＋ 对话面 `zergCmdOutLimit` 8000 字符）⇒ 19265 字节的真件只能取到约 8KB，
	// 模型判定对、`--dry-run` 也说可写，**却拼不出「完整原文 + 只改一行」** ⇒ 停手不写。
	// 解法 = 照 `dev edit --replace` **同一把尺**（`applyReplacements` · 逐格 `old` 恰好命中 1 次）：
	// 只把「要改的那一小段」喂进来，全文由**进程内**读盘 ⇒ 不必先把整件读进上下文。
	//
	// 互斥三态（全在**唯一写盘点之前**判掉）：`--replace` 与 `--content` / `--content-stdin` /
	// `--append` 两两不许同给（同给即两种内容来源 ⇒ 退码 2 · 具名）。
	if replaceGiven {
		if contentGiven || stdinGiven {
			inv.setErr("usage", "replace_conflict", "`--replace` 与整件写互斥")
			fmt.Fprintf(stderr, "%s: ★ **拒执** —— `--replace <补丁件>`（定点替换）与整件写**互斥**：\n", progName)
			fmt.Fprintf(stderr, "  收到 --replace（%v）＋ --content（%v）／--content-stdin（%v）⇒ 只许给**一种**内容来源（退码 2 · GAP-20261018-09）\n",
				replaceGiven, contentGiven, stdinGiven)
			return exitUsage
		}
		if inv.hasFlag("--append") {
			inv.setErr("usage", "replace_conflict", "`--replace` 与 --append 互斥")
			fmt.Fprintf(stderr, "%s: ★ **拒执** —— `--replace <补丁件>`（在**既有件**上定点替换）与 `--append`（追加）**互斥** ⇒ 退码 2（GAP-20261018-09）\n", progName)
			return exitUsage
		}
	} else if contentGiven == stdinGiven {
		inv.setErr("usage", "content_source", "内容来源要二选一（--content / --content-stdin）")
		fmt.Fprintf(stderr, "%s: 内容来源**二选一**：`--content <文本>` 或 `--content-stdin`（收到 %d 个）⇒ 退码 2\n",
			progName, boolCount(contentGiven, stdinGiven))
		if inv.jsonGiven && len(inv.fields) == 0 {
			_ = requireFields(inv, stderr)
		}
		return exitUsage
	}
	root, rc := fsToolRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	// ★ GAP-20261018-07：兄弟仓正门（不给旗标 ⇒ 零输出 · 旧判据一位不动）。
	sibName, sibRoot, rc := fsToolSibling(inv, stderr, root)
	if rc != exitOK {
		return rc
	}
	// 写面的「根」= 实际落点那棵树（本仓 / 兄弟仓）—— 符号链接父目录判据必须跟着换根，
	// 否则兄弟仓的每一次写都会被「父目录经符号链接出仓」误杀（兄弟仓本来就在主仓根外）。
	base := root
	if sibName != "" {
		base = sibRoot
	}
	// ★ 兄弟仓模式下另取**解析后的根**：`/tmp` 这类根本身就是符号链接（macOS `/tmp -> /private/tmp`），
	//   只拿未解析的 base 去比已解析的 parent ⇒ 兄弟仓每一次写都会被误判「父目录经符号链接出仓」。
	//   默认态（sibName 空）baseReal == base ⇒ 两个判据同一条，行为**逐字不变**。
	baseReal := base
	if sibName != "" {
		if r, eerr := filepath.EvalSymlinks(base); eerr == nil {
			baseReal = r
		}
	}
	abs, rel, rc := fsToolResolvePathIn(inv, stderr, root, sibName, sibRoot, inv.args[0])
	if rc != exitOK {
		return rc
	}
	label := fsToolLabel(sibName, rel)
	// ② 符号链接：终段 lstat 直接判；父目录按**解析后的真实路径**判「没出仓」。
	if st, lerr := os.Lstat(abs); lerr == nil && st.Mode()&os.ModeSymlink != 0 {
		inv.setErr("usage", "target_is_symlink", "目标件是符号链接")
		fmt.Fprintf(stderr, "%s: 目标件是**符号链接**：%s ⇒ 退码 2（写面只写真件 · ✗ 不跟链接出仓）\n", progName, label)
		return exitUsage
	}
	parent := filepath.Dir(abs)
	pst, perr := os.Stat(parent)
	if perr != nil || !pst.IsDir() {
		// ③ 父目录不存在 ⇒ 2 **不自建**（自建目录 = 第二个写面 · 规格逐字禁）。
		inv.setErr("usage", "parent_absent", "父目录不存在")
		fmt.Fprintf(stderr, "%s: 父目录不存在：%s ⇒ 退码 2（**✗ 不自建目录** —— 先由人/正门把目录建好）\n",
			progName, filepath.Dir(label))
		return exitUsage
	}
	if realParent, eerr := filepath.EvalSymlinks(parent); eerr == nil && !pathWithinDir(base, realParent) && !pathWithinDir(baseReal, realParent) {
		inv.setErr("usage", "path_outside_repo", "父目录经符号链接出仓")
		fmt.Fprintf(stderr, "%s: 父目录经符号链接指向仓外（%s）⇒ 退码 2（✗ 不跟链接出仓）\n", progName, realParent)
		return exitUsage
	}
	appendMode := inv.hasFlag("--append")
	var before []byte
	existed := false
	if st, serr := os.Lstat(abs); serr == nil {
		if st.IsDir() {
			inv.setErr("usage", "target_is_dir", "点的是目录")
			fmt.Fprintf(stderr, "%s: 点的是目录、不是件：%s ⇒ 退码 2\n", progName, rel)
			return exitUsage
		}
		b, rerr := os.ReadFile(abs)
		if rerr != nil {
			inv.setErr("blocked", "file_unreadable", "既有件读不动")
			fmt.Fprintf(stderr, "%s: 既有件读不动（%v）⇒ 不给结论（退码 8 · 盲写会毁掉它）\n", progName, rerr)
			return exitBlocked
		}
		before, existed = b, true
	}
	// ★ GAP-20261018-09：`--replace` 走**定点替换**支 —— 内容由「盘上原文 ＋ 补丁件」在**进程内**算出
	//   （✗ 不读 stdin · ✗ 不需要调用方先把全文读进上下文）。
	var after []byte
	replaceNote := ""
	payloadN := 0
	if replaceGiven {
		if !existed {
			inv.setErr("usage", "file_absent", "定点替换的靶件不存在")
			fmt.Fprintf(stderr, "%s: `--replace` 的**靶件不存在**：%s ⇒ 退码 2（定点替换只在**既有件**上做 —— 新建走 `--content`/`--content-stdin`）\n",
				progName, label)
			return exitUsage
		}
		replSpec := strings.TrimSpace(inv.flagVal("--replace"))
		if replSpec == "" {
			inv.setErr("usage", "replace_failed", "`--replace` 缺补丁件路径")
			fmt.Fprintf(stderr, "%s: `--replace` 给了**空值** ⇒ 退码 2（要一枚补丁件路径：`[{\"old\":…,\"new\":…}]` · GAP-20261018-09）\n", progName)
			return exitUsage
		}
		// 补丁件路径口径 = `dev edit --replace` 那一套（**仓内相对按仓根** · 绝对路径按原样）；
		// 兄弟仓模式下按**兄弟仓根**解析（写哪棵树就在哪棵树里找补丁件）。
		replPath := devPathResolveFrom(base, replSpec)
		replaced, rerr := applyReplacements(before, replPath)
		if rerr != nil {
			inv.setErr("usage", "replace_failed", rerr.Error())
			fmt.Fprintf(stderr, "%s: 定点替换**拒执**：%v\n", progName, rerr)
			fmt.Fprintf(stderr, "（判据 = 每格 `old` 在目标件里**恰好命中 1 次**；0 次 / 多次都到此退码 2 · 一个字节都没写 · GAP-20261018-09）\n")
			return exitUsage
		}
		after = replaced
		replaceNote = replSpec
	} else {
		var payload []byte
		if stdinGiven {
			b, rerr := io.ReadAll(os.Stdin)
			if rerr != nil {
				inv.setErr("blocked", "stdin_unreadable", "stdin 读不动")
				fmt.Fprintf(stderr, "%s: `--content-stdin` 读 stdin 失败（%v）⇒ 不给结论（退码 8）\n", progName, rerr)
				return exitBlocked
			}
			payload = b
		} else {
			payload = []byte(inv.flagVal("--content"))
		}
		payloadN = len(payload)
		after = payload
		if appendMode {
			after = append(append([]byte{}, before...), payload...)
		}
	}
	beforeSHA := ""
	if existed {
		beforeSHA = fsToolSHA256(before)
	}
	afterSHA := fsToolSHA256(after)
	auditPath := editAuditPath()
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未声明）"
	}

	// 计划件（干跑与「缺 --yes」两档打印同一份 · 逐条点名；真写档**不**打计划件 ——
	// 与 `script inventory sync` 同一条口径：计划面只在「不执行」的那两档出现）。
	planOut := stdout
	if inv.jsonGiven {
		planOut = stderr // 机器面分家：stdout 只留包封
	}

	if inv.dryRun {
		inv.changed = boolPtr(false) // 干跑零副作用 ⇒ 幂等面说真话
		fsWritePrintPlan(planOut, stderr, label, sibName, sibRoot, existed, appendMode, payloadN, len(after), beforeSHA, afterSHA, auditPath, true, replaceNote)
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{fsWriteRow(rel, "dry_run", len(after), beforeSHA, afterSHA, appendMode, auditPath)})
		}
		return exitOK
	}
	if !inv.yes {
		inv.changed = boolPtr(false) // 未执行 ⇒ 零变化
		fsWritePrintPlan(planOut, stderr, label, sibName, sibRoot, existed, appendMode, payloadN, len(after), beforeSHA, afterSHA, auditPath, false, replaceNote)
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{fsWriteRow(rel, "need_yes", len(after), beforeSHA, afterSHA, appendMode, auditPath)}); rc != exitOK {
				return rc
			}
		}
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `fs write` 是 D2 档 —— **缺 --yes ⇒ 不执行**（照 `repo commit` 的 fail-closed · 从不提问）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s fs write <件> --dry-run\n", progName)
		return exitUsage
	}

	// ---- 真写：审计先落盘 → 临时件 + rename 原子写 → 写后读回对拍 → 不等即写回 ----
	audit := fsWriteAuditLine{
		At: time.Now().Format(time.RFC3339), Event: "fs_file_written", FsCmd: "fs write",
		FsPath: rel, FsBefore: beforeSHA, FsAfter: afterSHA, FsBytes: len(after),
		FsAppend: appendMode, FsExisted: existed, By: by, Confirm: "--yes",
		RollbackTo: "把该件恢复成审计行里的 fs_before_sha256（不存在 ⇒ 删件）",
		Sibling:    sibName, SiblingAt: sibRoot,
	}
	if err := fsWriteAppendAudit(auditPath, audit); err != nil {
		inv.setErr("blocked", "audit_unwritable", "审计写不进 ⇒ 不写件")
		fmt.Fprintf(stderr, "%s: 审计写不进（%s）：%v ⇒ **不写件**（没有审计的写 = 之后判不了「谁改的」）\n",
			progName, auditPath, err)
		return exitBlocked
	}
	mode := os.FileMode(0o644)
	if st, serr := os.Lstat(abs); serr == nil {
		mode = st.Mode().Perm()
	}
	tmp := abs + fmt.Sprintf(".zergfs-tmp-%d", os.Getpid())
	if err := os.WriteFile(tmp, after, mode); err != nil {
		inv.setErr("failed", "write_failed", "临时件写不进")
		fmt.Fprintf(stderr, "%s: 临时件写不进（%s）：%v ⇒ 一个字节都没落\n", progName, tmp, err)
		return exitFail
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		inv.setErr("failed", "write_failed", "rename 落件失败")
		fmt.Fprintf(stderr, "%s: rename 落件失败（%s）：%v ⇒ 临时件已删、目标件未动\n", progName, abs, err)
		return exitFail
	}
	back, rerr := os.ReadFile(abs)
	if rerr != nil || fsToolSHA256(back) != afterSHA {
		// 写后读回对拍不等 ⇒ 逐字节写回前态（可证回滚）。
		if existed {
			_ = os.WriteFile(abs, before, mode)
		} else {
			_ = os.Remove(abs)
		}
		inv.setErr("failed", "readback_mismatch", "写后读回对拍不等 ⇒ 已回滚")
		fmt.Fprintf(stderr, "%s: 写后读回对拍不等（现读 %s ≠ 期望 %s）⇒ 已按前态写回\n",
			progName, fsToolSHA16(fsToolSHA256(back)), fsToolSHA16(afterSHA))
		return exitFail
	}
	inv.changed = boolPtr(true)
	modeSuffix := ""
	switch {
	case replaceGiven:
		modeSuffix = "（定点替换）"
	case appendMode:
		modeSuffix = "（追加）"
	}
	fmt.Fprintf(stdout, "已写 %s · %d 字节 · sha %s → %s%s\n", label, len(after),
		fsToolSHA16(beforeSHA), fsToolSHA16(afterSHA), modeSuffix)
	// ★ GAP-20261018-07：写**兄弟仓**必须在 stderr 留一行痕（谁都能在管道输出里看见「这一笔出仓了」），
	// 并与审计行的 `sibling_repo`/`sibling_root` 两格同源（人面与机器面不会各说一套）。
	if sibName != "" {
		fmt.Fprintf(stderr, "%s: ★ 写了兄弟仓 %s 的件 %s（根 %s · 审计行带 sibling_repo/sibling_root · GAP-20261018-07）\n",
			progName, sibName, label, sibRoot)
	}
	fmt.Fprintf(stdout, "审计落点 %s（事件 fs_file_written · 追加只写）\n", auditPath)
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{fsWriteRow(rel, "written", len(after), beforeSHA, afterSHA, appendMode, auditPath)})
	}
	return exitOK
}

// fsWriteRow —— `fs write` 的机器面一行（人面与机器面**同源**：同一份读数两处渲染）。
func fsWriteRow(rel, result string, n int, beforeSHA, afterSHA string, appendMode bool, auditPath string) map[string]string {
	return map[string]string{
		"path":          rel,
		"result":        result,
		"bytes":         strconv.Itoa(n),
		"before_sha256": beforeSHA,
		"after_sha256":  afterSHA,
		"append":        map[bool]string{true: "true", false: "false"}[appendMode],
		"audit_path":    auditPath,
	}
}

// fsWritePrintPlan —— 计划件（干跑与「缺 --yes」两档**打印同一份** · 零副作用）。
//
// ★ GAP-20261018-09：`replaceNote` 非空 ⇒ 本次是**定点替换**（`--replace <补丁件>`）：
// 计划件里明写补丁件与「每格 old 恰好命中 1 次」这把尺，并把「本次内容」那一行换掉
// （替换模式没有「要写进去的新内容」，只有「在哪一版原文上改哪几处」）。
func fsWritePrintPlan(out, stderr io.Writer, label, sibName, sibRoot string, existed, appendMode bool, payloadN, afterN int, beforeSHA, afterSHA, auditPath string, dryRun bool, replaceNote string) {
	head := "--yes 前的一次预览"
	if dryRun {
		head = "--dry-run"
	}
	fmt.Fprintln(out, "计划件（"+head+" · 零副作用 —— 未写件、未写审计）")
	fmt.Fprintf(out, "  落点     : %s\n", label)
	if sibName != "" {
		fmt.Fprintf(out, "  兄弟仓   : %s（根 %s · 显式开闸 `--allow-sibling-repo` · 写面留痕进审计 `sibling_repo`/`sibling_root`）\n", sibName, sibRoot)
	}
	fmt.Fprintf(out, "  既有件   : %s\n", map[bool]string{true: "在（append 时以它为基础）", false: "不在（新建）"}[existed])
	if replaceNote != "" {
		fmt.Fprintf(out, "  模式     : 定点替换（`--replace`）—— 在**盘上原文**上逐格替换，✗ 不需要先把全文读进上下文\n")
		fmt.Fprintf(out, "  补丁件   : %s（每格 `old` 必须在目标件里**恰好命中 1 次**；0 次 / 多次 ⇒ 退码 2）\n", replaceNote)
	} else {
		fmt.Fprintf(out, "  模式     : %s\n", map[bool]string{true: "追加（--append）", false: "整件覆盖"}[appendMode])
		fmt.Fprintf(out, "  本次内容 : %d 字节\n", payloadN)
	}
	fmt.Fprintf(out, "  写后大小 : %d 字节\n", afterN)
	if beforeSHA == "" {
		fmt.Fprintf(out, "  写前 sha : （件不在）\n")
	} else {
		fmt.Fprintf(out, "  写前 sha : %s\n", fsToolSHA16(beforeSHA))
	}
	fmt.Fprintf(out, "  写后 sha : %s（按**将要写的内容**现算）\n", fsToolSHA16(afterSHA))
	fmt.Fprintf(out, "  审计落点 : %s\n", auditPath)
	fmt.Fprintln(stderr, "（"+head+"：只出计划件 · 零副作用）")
}

// ── `fs run` ─────────────────────────────────────────────────────────────────

// fsRunSubAllowed —— 首词在闭集里、且子命令在**该首词**的闭集里才算放行。
// 返回 (是否放行, 拒执理由)。
func fsRunSubAllowed(head string, rest []string) (bool, string) {
	okHead := false
	for _, h := range fsRunHeadAllow {
		if h == head {
			okHead = true
			break
		}
	}
	if !okHead {
		return false, fmt.Sprintf("首词 %q 不在硬白名单 %s 里", head, strings.Join(fsRunHeadAllow, "/"))
	}
	allow, has := fsRunSubAllow[head]
	if !has || len(allow) == 0 {
		return false, fmt.Sprintf("首词 %q 没有子命令闭集（fail-closed ⇒ 拒执）", head)
	}
	sub := ""
	if len(rest) > 0 {
		sub = rest[0]
	}
	if sub == "" {
		// 裸首词（如 `zerg`）：只许它在闭集里出现过的「无子命令」形态 —— 一律拒（fail-closed），
		// 免得 `zerg` 裸跑落进全局帮助之外的分支。
		return false, fmt.Sprintf("%q 后面必须跟闭集里的子命令（%s）", head, strings.Join(allow, "|"))
	}
	for _, s := range allow {
		if s == sub {
			return true, ""
		}
	}
	return false, fmt.Sprintf("%q 的子命令 %q 不在闭集（%s）里", head, sub, strings.Join(allow, "|"))
}

// fsRunPlanLine —— `fs run` 的 argv 回显（人面与计划件**同源**）。
func fsRunPlanLine(argv []string) string { return strings.Join(argv, " ") }

// fsRunBinary —— 首词 ⇒ 真执行体的**绝对路径**。
//
// ★ `zerg` 走**自己**（`os.Executable()`）：`fs run zerg version` 要跑的就是本制品，
// ✗ 不靠 PATH 猜（PATH 上那份可能不是仓内这一份 ⇒ 读数不是本仓的读数）。
func fsRunBinary(head string) (string, error) {
	if head != "zerg" {
		return head, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}

// cmdFSRun —— `zerg fs run <命令…> [--cwd <目录>] [--timeout <秒>] [--json <字段>] [--dry-run | --yes]`：**D3**。
//
// fail-closed 三道（全在**真跑之前**判掉 · ✗ 不得静默降级 · ✗ 不得 shell 解释）：
//
//	① 首词闭集六枚（`fsRunHeadAllow`）—— 其余一律 rc=2；
//	② 子命令闭集（`fsRunSubAllow`）—— `go run` / `git push` / `git reset --hard` 这一族一律 2；
//	③ `--cwd` 必须在仓内、且是真目录（经符号链接出仓同样 2）。
//
// argv **直投** `exec.CommandContext`：一个 shell 都不起 ⇒ `|` `>` `;` `$()` 全是**字面实参**，
// 既不会被解释，也就不存在「注入」这条面。
//
// 退码：子进程退码**原样转出**（0 干净 / 非 0 即它的读数）；超时 ⇒ 11（等不起 ≠ 等到坏结果）；
// 起不来 ⇒ 8（不给结论）。
func cmdFSRun(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "bad_arity", "`fs run` 缺命令")
		fmt.Fprintf(stderr, "%s: `fs run` 要给一条命令（首词只许 %s）⇒ 退码 2\n", progName, strings.Join(fsRunHeadAllow, "/"))
		fmt.Fprintf(stderr, "用法：%s fs run <命令…> [--cwd <目录>] [--timeout <秒>] [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	argv := append([]string{}, inv.args...)
	head := argv[0]
	if ok, why := fsRunSubAllowed(head, argv[1:]); !ok {
		inv.setErr("usage", "command_not_allowed", "命令不在硬白名单闭集")
		fmt.Fprintf(stderr, "%s: `fs run` **拒执**：%s ⇒ 退码 2（硬白名单 fail-closed · ✗ 不静默降级 · ✗ 不经 shell）\n", progName, why)
		if head == "git" {
			fmt.Fprintf(stderr, "（`git push` / `git reset --hard` / `git checkout` 一类写远端/毁工作树的子命令**单独禁** —— 首版只开只读面）\n")
		}
		fmt.Fprintf(stderr, "闭集逐条：%s\n", fsRunClosureLine())
		return exitUsage
	}
	root, rc := fsToolRoot(inv, stderr)
	if rc != exitOK {
		return rc
	}
	workDir := root
	if inv.hasFlag("--cwd") {
		raw := strings.TrimSpace(inv.flagVal("--cwd"))
		if raw == "" {
			inv.setErr("usage", "bad_cwd", "--cwd 给了空值")
			fmt.Fprintf(stderr, "%s: `--cwd` 给了空值 ⇒ 退码 2\n", progName)
			return exitUsage
		}
		cand := filepath.Clean(filepath.FromSlash(raw))
		if !filepath.IsAbs(cand) {
			cand = filepath.Clean(filepath.Join(root, cand))
		}
		if !pathWithinDir(root, cand) {
			inv.setErr("usage", "cwd_outside_repo", "--cwd 出仓")
			fmt.Fprintf(stderr, "%s: `--cwd` 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, raw, root)
			return exitUsage
		}
		st, serr := os.Stat(cand)
		if serr != nil || !st.IsDir() {
			inv.setErr("usage", "cwd_absent", "--cwd 不是目录")
			fmt.Fprintf(stderr, "%s: `--cwd` 指的不是目录：%s ⇒ 退码 2\n", progName, raw)
			return exitUsage
		}
		if real, eerr := filepath.EvalSymlinks(cand); eerr == nil && !pathWithinDir(root, real) {
			inv.setErr("usage", "cwd_outside_repo", "--cwd 经符号链接出仓")
			fmt.Fprintf(stderr, "%s: `--cwd` 经符号链接指向仓外（%s）⇒ 退码 2\n", progName, real)
			return exitUsage
		}
		workDir = cand
	}
	timeoutS := fsRunDefaultTimeoutS
	if inv.hasFlag("--timeout") {
		d, werr := parseWallclock(inv.flagVal("--timeout"))
		if werr != nil {
			inv.setErr("usage", "bad_wallclock", "--timeout 时长读不出来")
			fmt.Fprintf(stderr, "%s: %v ⇒ 退码 2\n", progName, werr)
			return exitUsage
		}
		timeoutS = int(d / time.Second)
		if timeoutS < 1 {
			timeoutS = 1
		}
	}
	relCWD, _ := filepath.Rel(root, workDir)

	planOut := stdout
	if inv.jsonGiven {
		planOut = stderr
	}
	if inv.dryRun {
		inv.changed = boolPtr(false) // 干跑零副作用
		fsRunPrintPlan(planOut, stderr, argv, relCWD, timeoutS, true)
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			row := map[string]string{
				"cmd": fsRunPlanLine(argv), "cwd": filepath.ToSlash(relCWD),
				"timeout_s": strconv.Itoa(timeoutS), "rc": "", "bytes": "0",
				"truncated": "false", "output": "",
			}
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row})
		}
		return exitOK
	}
	if !inv.yes {
		inv.changed = boolPtr(false) // 未执行 ⇒ 零变化
		fsRunPrintPlan(planOut, stderr, argv, relCWD, timeoutS, false)
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			row := map[string]string{
				"cmd": fsRunPlanLine(argv), "cwd": filepath.ToSlash(relCWD),
				"timeout_s": strconv.Itoa(timeoutS), "rc": "", "bytes": "0",
				"truncated": "false", "output": "",
			}
			if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row}); rc != exitOK {
				return rc
			}
		}
		inv.setErr("usage", "yes_required", "D3 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `fs run` 是 D3 档（最高）—— **缺 --yes ⇒ 不执行**（fail-closed · 从不提问）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s fs run %s --dry-run\n", progName, fsRunPlanLine(argv))
		return exitUsage
	}

	bin, berr := fsRunBinary(head)
	if berr != nil {
		inv.setErr("blocked", "binary_unresolved", "执行体解析不到")
		fmt.Fprintf(stderr, "%s: 执行体解析不到（%s）：%v ⇒ 不给结论（退码 8）\n", progName, head, berr)
		return exitBlocked
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Dir = workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf // 两条流合一份：命令行跑出来的「读数」不分家
	runErr := cmd.Run()
	outBytes := buf.Bytes()
	full := len(outBytes)
	cut := false
	if full > fsRunOutputCap {
		outBytes = outBytes[:fsRunOutputCap]
		for len(outBytes) > 0 && !utf8.Valid(outBytes) {
			outBytes = outBytes[:len(outBytes)-1] // 截断落在多字节字符中间 ⇒ 退到合法边界
		}
		cut = true
	}
	childRC := 0
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		childRC = exitTimeout
		inv.setErr("timeout", "run_timeout", "命令超时")
		fmt.Fprintf(stderr, "%s: 命令超时（>%ds）⇒ **已中止**（退码 %d = 等不起，不是「等到了坏结果」）\n",
			progName, timeoutS, exitTimeout)
	case runErr != nil:
		if ee, ok := runErr.(*exec.ExitError); ok {
			childRC = ee.ExitCode()
			if childRC < 0 {
				childRC = exitFail
			}
		} else {
			inv.setErr("blocked", "exec_failed", "子进程起不来")
			fmt.Fprintf(stderr, "%s: 子进程起不来（%s）：%v ⇒ 不给结论（退码 8）\n", progName, bin, runErr)
			return exitBlocked
		}
	}
	if cut {
		inv.markTruncated()
		inv.warnf("输出已截断：只显示前 %d 字节（共 %d 字节）", fsRunOutputCap, full)
		fmt.Fprintf(stderr, "【截断】输出共 %d 字节，本页只显示前 %d 字节（`--json output` 同此截断 · 数字不省略）\n",
			full, fsRunOutputCap)
	}
	// 原样输出：人面走 stdout；`--json` 档**改投 stderr**（stdout 只留六键包封 —— 混进子进程原文
	// 会让消费侧解析不到包封，与 `script inventory sync` 的「机器面分家」同一条口径）。
	rawOut := stdout
	if inv.jsonGiven {
		rawOut = stderr
	}
	if len(outBytes) > 0 {
		_, _ = rawOut.Write(outBytes)
		if outBytes[len(outBytes)-1] != '\n' {
			fmt.Fprintln(rawOut)
		}
	}
	if inv.jsonGiven {
		row := map[string]string{
			"cmd": fsRunPlanLine(argv), "cwd": filepath.ToSlash(relCWD),
			"timeout_s": strconv.Itoa(timeoutS), "rc": strconv.Itoa(childRC),
			"bytes": strconv.Itoa(full), "truncated": map[bool]string{true: "true", false: "false"}[cut],
			"output": string(outBytes),
		}
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row}); rc != exitOK {
			return rc
		}
	}
	return childRC
}

// fsRunPrintPlan —— `fs run` 的计划件（干跑与「缺 --yes」两档打印同一份 · 零副作用）。
func fsRunPrintPlan(out, stderr io.Writer, argv []string, relCWD string, timeoutS int, dryRun bool) {
	head := "--yes 前的一次预览"
	if dryRun {
		head = "--dry-run"
	}
	fmt.Fprintln(out, "计划件（"+head+" · 零副作用 —— 未起任何子进程）")
	fmt.Fprintf(out, "  命令     : %s\n", fsRunPlanLine(argv))
	fmt.Fprintf(out, "  工作目录 : %s\n", filepath.ToSlash(relCWD))
	fmt.Fprintf(out, "  超时     : %ds\n", timeoutS)
	fmt.Fprintf(out, "  执行形态 : argv 直投（**不经 shell** —— `|` `>` `;` `$()` 一律是字面实参）\n")
	fmt.Fprintf(out, "  白名单   : 首词 ∈ %s · 子命令闭集见 `zerg help fs`\n", strings.Join(fsRunHeadAllow, "/"))
	fmt.Fprintln(stderr, "（"+head+"：只出计划件 · 零副作用）")
}

// fsRunClosureLine —— 拒执时**逐条**回显闭集（可照抄的下一步，而不是只说「不行」）。
func fsRunClosureLine() string {
	// 稳定序：按白名单次序出（✗ 不用 map 序 —— 两次跑输出会长歪）。
	out := make([]string, 0, len(fsRunHeadAllow))
	for _, h := range fsRunHeadAllow {
		if allow, ok := fsRunSubAllow[h]; ok {
			out = append(out, h+" ⇒ "+strings.Join(allow, "|"))
		}
	}
	return strings.Join(out, " · ")
}
