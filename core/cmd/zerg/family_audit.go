// family_audit.go —— `zerg audit cover`：**工作树脏件 ↔ 审计行** 的对拍（只报告档 · 纯只读）。
//
// 出处：laneGJ《非正门写者「零审计痕迹」→ 可机检归因/可见面》设计稿（本夜，仓外）§二 乙档 ⇒ 用户拍「按推荐」。
// 标的缺口 `GAP-20260928-98` 族：**压根没有审计行**（与 `GAP-20260928-79` 的对偶 ——
// 79 = 有审计行但回指不到授权物（已治），本格 = 一件被改过、审计里一条 `edit` 行都没有）。
//
// 形态：`zerg audit cover [--root <仓根>] [--file <件>]… [--json <字段>]`
//
// 四条写死的口径（改这里 = 改判据）：
//
//	① **分母** = `git status --porcelain=v1` 里**已跟踪**的 `M`/`A` 行（逐件取相对路径）。
//	   ★ 分母**必须由 git porcelain 定，不能由审计定** —— 若分母只取「审计里出现过的件」，
//	     那么审计命中 0 行的件**永远无对拍**（那就是最硬的假绿面：GAP-98 本体）。
//	   真源纪律照 `family_repo.go` 既有那句：「脏件清单：真源就是 git 自己的 porcelain 面」。
//	   `??` 未跟踪件不进分母（无 HEAD 基线）· 其余状态（`D`/`R`/`C`/`U`）不进分母、只作**旁栏**照实印。
//	② **对拍式子（唯一一处）**：`W(f)` = 工作树该件的 `sha256`（64-hex，口径与 `repo status` 同源）；
//	   `A(f)` = 审计里 `event == "edit"` 且 `file == f` 的**最新一条**（按 `at`）的 `after_sha256`。
//	   `A(f)` 不存在 ⇒ `无记录` · `W(f) != A(f)` ⇒ `记录过期` · 相等 ⇒ `有记录` · 读不到 ⇒ `读不到`。
//	③ **只报告档（起手档）**：有未覆盖件**仍退 0**（「未记录」是状态不是错 —— 本机现读绝大多数脏件是
//	   合法人工编辑）；`2` = 用法错 · `8` = 读不到/判不了（**不当绿**）。★ 不进任何必跑路径（不进提交闸）。
//	④ **本命令纯只读**：不许写任何件、不启进程、不改仓内状态；`repo status` 那条「写旗标一律拒」同款
//	   第一道牙（`repoWriteHits` 复用，不另立第二份写词表）。
//
// 照实两条（本版**给不出**的）：
//
//	· 「谁写的」**给不出**（归因不可机检 ⇒ 照 79 族既有上限，留 OS 层）；
//	· 写者**按原内容回写**（`before == after`）⇒ 无审计行且与 HEAD 同 ⇒ 本命令也看不见（最硬假绿面，
//	  设计稿 §三 假绿 3；真治要 OS 层痕迹，超仓内可机检范围）。
//
// 六面孔（新命令落地的随动口 · 照实自陈 · 见本单回执）：
//
//	面① 命令树登记（`main.go` 的 `commands`）—— **本单已做**（同批）。
//	面② 契约矩阵 `core/cmd/zerg/testdata/cli-matrix.json` + 基线 `scripts/gates/cli-contract-baseline.json`
//	     —— ★ **未做**（本单禁碰面）⇒ `TestCLIContractMatrixNoBlank` 会把本条报成 1 条**空白**
//	     （命令没有 must-fail case 也没有带 reason 的豁免）⇒ **该红属预期**，须**同批补**（先例逐字：
//	     `family_lsface.go` 件头「新增命令未入矩阵 ⇒ 契约/门禁本轮**必红**，属预期，不是缺陷」）。
//	面③ 文档两树（`docs/*/参考-命令行.md` · `publish/docs/CLI.*.md`）—— **未做**（禁碰面）。
//	面④ 门69 `help` 面形状常量（`scripts/gates/check-help-shape.py`）—— ★ **动了 help 面字节**
//	     （新命令多一行用法行 ⇒ `help --all` 的行数/字节/条数常量随动）⇒ **需人签刷常量**，本单不碰该件。
//	面⑤ 干跑语义登记表（门⑩ 那件登记表 · `core/internal/contract/` 下）—— **不适用**：
//	     本件**无干跑档**、且全件 ASCII 干跑字样**零出现** ⇒ 门⑩ 的 `R1 未登记件` 扫描口径对本件**不命中**
//	     （登记表里挂一个不被扫描命中的件反而会踩 `R2 幽灵`：登记了但现跑命中里没有它）。
//	     ★ 本单实测脚注：件头若**写出登记表的件名本身**，那个件名里的 ASCII 词会被扫描口径（`grep -E`）
//	     认成命中 ⇒ 本行改写为不带该词（改写前：本件 1 行命中 ⇒ 门⑩ 必红；改写后：0 行命中）。
//	面⑥ 脚本台账 / 公开标记（`scripts/公开标记.tsv` · 仓外台账）—— **不适用**：那两张表管的是
//	     `scripts/` 面下的脚本件，本件是 `core/` 下的 Go 件（门76 `check-newfile-registry.py` 的分母
//	     口径 = `scripts/` 面 ⇒ 本件不在其分母里）。
//
// 退码（真源 `zerg help exit-codes` / `exitcodes.go`）：`0` 报告跑通（含「有未覆盖件」—— 只报告档）·
// `2` 用法错（写旗标 / 不认的字段）· `8` 不给结论（仓根解析不到 · `git` 跑不动 · 审计读不到 · **分母 0**）。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// auditCoverUsage —— 本条命令的**完整用法串**（唯一真源：登记条目与各条用法错路径都读它）。
const auditCoverUsage = "zerg audit cover [--root <仓根>] [--file <件>] [--json <字段>]"

// auditCoverFields —— `--json` 字段**闭集**（照同族体例：登记条目的 `fields` 与本表同源一处）。
var auditCoverFields = []string{
	"head", "branch", "path", "status",
	"worktree_sha16", "audit_rows", "audit_last_at", "audit_last_after_sha16", "verdict",
}

// auditCoverVerdict* —— 四种判词（**逐字**，与设计稿 §二 乙档那条式子一一对应）。
const (
	auditVerdictCovered    = "有记录"
	auditVerdictNoRow      = "无记录"
	auditVerdictStale      = "记录过期"
	auditVerdictUnreadable = "读不到"
)

// auditEditRow —— 审计里 `edit` 事件**本命令取用的那几键**（只读 · 不重抄整行）。
type auditEditRow struct {
	Event string `json:"event"`
	File  string `json:"file"`
	At    string `json:"at"`
	After string `json:"after_sha256"`
}

// auditCoverIndex —— 审计侧索引：`file` → 该件**最新**一条 `edit` 行的（按 `at`）。
type auditCoverIndex struct {
	latest map[string]auditEditRow
	rows   map[string]int // file → 该件的 `edit` 行条数（「审计有无行」的读数）
	total  int            // `edit` 事件总行数（可读性交叉对拍用）
	bad    int            // 解析不了的行（照实印 · 不当 0 顶替）
}

// auditCoverLoad 读审计落点并建索引（**只读**）。
//
// 落点解析**复用** `editAuditPath()`（唯一真源：`ZERG_EDIT_AUDIT` > `<ZERG_STATE_DIR>/edit_audit.jsonl`
// > `~/.zerg/state/edit_audit.jsonl`）—— 本件**不另抄一条落点式子**（抄了就是第二套口径）。
func auditCoverLoad(path string) (*auditCoverIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	idx := &auditCoverIndex{latest: map[string]auditEditRow{}, rows: map[string]int{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		ln := strings.TrimSpace(sc.Text())
		if ln == "" {
			continue
		}
		var r auditEditRow
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			idx.bad++
			continue
		}
		if r.Event != "edit" {
			continue
		}
		idx.total++
		idx.rows[r.File]++
		if prev, ok := idx.latest[r.File]; !ok || r.At > prev.At {
			idx.latest[r.File] = r
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

// repoPorcelainTrackedMA —— 分母的唯一式子：porcelain 里**已跟踪**的 `M`/`A` 行。
// 返回（进分母的 path · 旁栏：其它已跟踪状态逐条）。
func repoPorcelainTrackedMA(porcelain string) (paths, otherTracked []string) {
	for _, ln := range strings.Split(porcelain, "\n") {
		if ln == "" || strings.HasPrefix(ln, "## ") || len(ln) < 4 {
			continue
		}
		code := strings.TrimSpace(ln[:2])
		path := strings.TrimSpace(ln[3:])
		if strings.HasPrefix(code, "?") {
			continue // `??` 未跟踪件：无 HEAD 基线 ⇒ 不进分母
		}
		if strings.ContainsAny(code, "MA") {
			paths = append(paths, path)
			continue
		}
		otherTracked = append(otherTracked, code+" "+path)
	}
	sort.Strings(paths)
	sort.Strings(otherTracked)
	return paths, otherTracked
}

// cmdAuditCover —— 只报告档的入口。
func cmdAuditCover(inv *invocation, stdout, stderr io.Writer) int {
	// 第一道牙：写旗标一律拒（复用 `family_repo.go` 的写词闭集 —— 不另立第二份）。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读对拍 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `audit cover` **纯只读**（不写件、不起进程、不改仓内状态）· 用法 `%s`\n",
			progName, auditCoverUsage)
		return exitUsage
	}
	root := ""
	for _, r := range inv.flagVals("--root") {
		if s := strings.TrimSpace(r); s != "" {
			root = s
			break
		}
	}
	if root == "" {
		root = repoRoot()
	}
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到工作树（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或 `--root <仓根>`\n")
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		inv.setErr("blocked", "not_a_git_repo", "仓根下没有 .git")
		fmt.Fprintf(stderr, "%s: %s 下没有 .git ⇒ 这不是一个 git 工作树（不给结论 · 退码 8）\n", progName, root)
		return exitBlocked
	}
	porcelain, err := gitRun(root, "status", "--porcelain=v1", "--branch", "--untracked-files=normal")
	if err != nil {
		inv.setErr("blocked", "git_status_failed", err.Error())
		fmt.Fprintf(stderr, "%s: `git status` 跑不动 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	head, herr := gitRun(root, "rev-parse", "--short", "HEAD")
	if herr != nil {
		inv.setErr("blocked", "git_rev_parse_failed", herr.Error())
		fmt.Fprintf(stderr, "%s: `rev-parse --short HEAD` 跑不动 ⇒ 不给结论（退码 8）：%v\n", progName, herr)
		return exitBlocked
	}
	head = strings.TrimSpace(head)
	branch := ""
	for _, ln := range strings.Split(porcelain, "\n") {
		if strings.HasPrefix(ln, "## ") {
			branch = parsePorcelainBranch(strings.TrimPrefix(ln, "## "))
			break
		}
	}
	if branch == "" {
		branch = "（detached / 读不到）"
	}

	denom, otherTracked := repoPorcelainTrackedMA(porcelain)

	// `--file`：**收窄分母**（只对点名的件对拍）；点名的件不在分母里 ⇒ 逐条点名（照实 · 不静默丢）。
	named := []string{}
	for _, v := range inv.flagVals("--file") {
		if s := strings.TrimSpace(v); s != "" {
			named = append(named, s)
		}
	}
	if len(named) > 0 {
		keep := map[string]bool{}
		for _, n := range named {
			keep[n] = true
		}
		filtered := []string{}
		for _, p := range denom {
			if keep[p] {
				filtered = append(filtered, p)
			}
		}
		have := map[string]bool{}
		for _, p := range filtered {
			have[p] = true
		}
		for _, n := range named {
			if !have[n] {
				fmt.Fprintf(stderr, "%s: `--file %s` 点名的件**不在分母**（已跟踪 M/A 面）里 ⇒ 本跑不对拍它（照实点名 · 不当绿）\n",
					progName, n)
			}
		}
		denom = filtered
	}

	ap := editAuditPath()
	if ap == "" {
		inv.setErr("blocked", "audit_path_absent", "审计落点解析不到")
		fmt.Fprintf(stderr, "%s: 审计落点解析不到 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	idx, aerr := auditCoverLoad(ap)
	if aerr != nil {
		inv.setErr("blocked", "audit_unreadable", aerr.Error())
		fmt.Fprintf(stderr, "%s: 审计读不到（%s）⇒ 不给结论（退码 8）：%v\n", progName, ap, aerr)
		fmt.Fprintf(stderr, "%s: ★ 审计读不到**不当绿也不当红**：一条行也读不到 ≠ 没有未覆盖件\n", progName)
		return exitBlocked
	}

	fmt.Fprintf(stderr, "%s: 仓 %s · HEAD %s · 分支 %s\n", progName, root, head, branch)
	fmt.Fprintf(stderr, "%s: **分母 = %d 件**（`git status --porcelain=v1` 的**已跟踪 `M`/`A` 行** —— 真源是 git 自己的面，不是审计）\n",
		progName, len(denom))
	if len(otherTracked) > 0 {
		fmt.Fprintf(stderr, "%s: 旁栏（**不进分母**）：另有 %d 件已跟踪但状态不是 M/A：%s\n",
			progName, len(otherTracked), strings.Join(otherTracked, " · "))
	}
	fmt.Fprintf(stderr, "%s: 审计落点 %s · `edit` 行 %d 条 · 解析不了 %d 条\n", progName, ap, idx.total, idx.bad)
	if len(denom) == 0 {
		inv.setErr("blocked", "empty_denominator", "分母 0 件")
		fmt.Fprintf(stderr, "%s: **分母 0 ⇒ 判不了**（不给结论 · 退码 8）—— 「真的没有已跟踪改动」与「读不到那个面」是两态，\n", progName)
		fmt.Fprintf(stderr, "%s: 本条**不许**在这一态上报绿（照门① 空转口径：分母 0 ⇒ 不给结论）\n", progName)
		return exitBlocked
	}

	rows := []map[string]string{}
	covered, noRow, stale, unread := 0, 0, 0, 0
	uncovered := []string{}
	for _, p := range denom {
		_, sh := repoFileIdentity(root, p)
		w16, verdict := "", ""
		if sh == repoIdentityAbsent || sh == "" {
			verdict = auditVerdictUnreadable
			unread++
		} else {
			w16 = sh
			if len(w16) > 16 {
				w16 = w16[:16]
			}
			last, ok := idx.latest[p]
			switch {
			case !ok:
				verdict = auditVerdictNoRow
				noRow++
			case last.After != sh:
				verdict = auditVerdictStale
				stale++
			default:
				verdict = auditVerdictCovered
				covered++
			}
		}
		if verdict == auditVerdictNoRow || verdict == auditVerdictStale {
			uncovered = append(uncovered, fmt.Sprintf("%s（%s · 工作树 %s · 审计 %s 行）",
				p, verdict, w16, auditRowWord(idx.rows[p])))
		}
		lastAt, lastAfter := "—", "—"
		if last, ok := idx.latest[p]; ok {
			lastAt = last.At
			lastAfter = last.After
			if len(lastAfter) > 16 {
				lastAfter = lastAfter[:16]
			}
		}
		rows = append(rows, map[string]string{
			"head":                   head,
			"branch":                 branch,
			"path":                   p,
			"status":                 "M/A",
			"worktree_sha16":         auditOrDash(w16),
			"audit_rows":             fmt.Sprintf("%d", idx.rows[p]),
			"audit_last_at":          lastAt,
			"audit_last_after_sha16": auditOrDash(lastAfter),
			"verdict":                verdict,
		})
	}

	fmt.Fprintf(stderr, "%s: 计数：分母 %d ｜ **有记录 %d** ｜ **无记录 %d** ｜ **记录过期 %d** ｜ 读不到 %d\n",
		progName, len(denom), covered, noRow, stale, unread)
	if len(uncovered) > 0 {
		fmt.Fprintf(stderr, "%s: **未覆盖件逐条点名**（%d 件 · 件·状态·工作树 sha16·审计有无行）：\n", progName, len(uncovered))
		for _, u := range uncovered {
			fmt.Fprintf(stderr, "%s:   · %s\n", progName, u)
		}
	} else {
		fmt.Fprintf(stderr, "%s: 未覆盖件 0 件（分母内每件都能对上审计最新 after_sha256）\n", progName)
	}
	fmt.Fprintf(stderr, "%s: ★ 只报告档（起手档）：有未覆盖件**仍退 0** —— 「无记录」是状态不是错；\n", progName)
	fmt.Fprintf(stderr, "%s:   本机现读绝大多数脏件是**合法人工/AI 直接编辑**（未经正门 ⇒ 天然无审计行），\n", progName)
	fmt.Fprintf(stderr, "%s:   判「破坏性」不是本命令的活儿（意图不可机检）· 本命令也**看不见**「写者按原内容回写」那一态\n", progName)
	if rc := listCmd(inv, stdout, stderr, auditCoverFields, rows); rc != exitOK {
		return rc
	}
	return exitOK
}

// auditOrDash —— 空值占位（**不编造、不拿 0 顶替**，口径照 `repoIdentityAbsent`）。
func auditOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// auditRowWord —— 「审计有无行」的中文读数（0 ⇒ 逐字「无」）。
func auditRowWord(n int) string {
	if n <= 0 {
		return "无"
	}
	return fmt.Sprintf("有 %d", n)
}
