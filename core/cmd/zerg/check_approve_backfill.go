// check_approve_backfill.go —— 门禁件 `scripts/gates/check-approve-backfill.py`（683 行 · v1.1.0）的 **Go 真搬**
// （M2 · 路B469 · 「批准档位族」纯静态补件 · 门⑰ 人签补签面）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一件跑两态
//	（py 原件 ↔ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 本件形态（选件读数）：
//
//	683 行 · 默认档**纯静态只读**（读 `core/cmd/zerg/family_approve.go` + 真审计件 + 真在册批准件，
//	一个字节都不写）· 退码三档齐（0 全绿 / 1 有红 / 2 不给结论）· 输出定序（读数是固定序，
//	零 map 迭代进输出）· **零子进程 / 零计时读数 / 零网络 / 零外部工具**。
//	★ 唯一的写面是**自带的常量刷新通道**（`--refresh-constant [--yes]`）：默认**干跑不写**，
//	  带 `--yes` 才做「等长最小文本替换 + 仓外备份」。对拍时写面一律指向**仓外夹具**
//	  （见回执「状态目录摆位处理」），✗ 绝不碰真仓原件。
//
// ⚠ 本单边界（照实）：只落码、不接线 ——
//
//	✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件显式点名）；
//	入口目前**无调用点**（M2 的刻意状态）；✗ 不碰登记面六面。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkApproveBackfillGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全绿（B1–B5 全过且自检全过）/ 1 有红 / 2 不给结论（取不到读数 / 用法错）
//	  · 输出 —— stdout 与 py 逐字节对齐（见下「未覆盖面」）
//
// 出厂声明行（两条 · 每枚必写）：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`（脚本位置上溯两级）；
//	    Go 入口**无 `__file__` 语境** ⇒ 默认取**当前工作目录**（`cafbDefaultRoot`）。「脚本位置 ≠ cwd」时
//	    路径推导面不同 ⇒ **✗ 不声称逐字对齐**；对拍一律**显式传 `--root`**（乙环境另令 cwd==夹具根）。
//	    ★ 同源第二处：py 的**目标件自身**取 `os.path.abspath(__file__)`（常量刷新的写点）。Go 无 `__file__`
//	    ⇒ 这是**有意旋钮面差**：新增 `--gate <件>`（env `ZERG_BACKFILL_GATE`）显式入参；缺省 = `--root`
//	    推出来的 `scripts/gates/check-approve-backfill.py`（root==仓根时与 py 同值）。对拍凡传 `--root` 必同传 `--gate`。
//	(b) **自指检查**：本件**不落在**它自己扫描的面内 —— 本件只读 ① `core/cmd/zerg/family_approve.go`
//	    ② `<状态目录>/edit_audit.jsonl` ③ `<状态目录>/approvals/dev_edit.json`；本件自身是
//	    `check_approve_backfill.go`，不被任何一面的逐字串匹配。四形态自查：① 件内**无**仓内绝对路径原串
//	    （全文只有相对 `core/cmd/zerg/…` 串，读点是 `cafbPyJoin(repo, rel)`）；② 夹具/自检标记串
//	    （`backfill-selftest-scan-`、`backfill-const-selftest-`、`ttl_seconds` 等）**均为运行期拼串**，
//	    ✗ 不以「会被扫到的形态」直写他门扫描面；③ self-test 无「魔法环境变量串」；④ 本件**未接线**、
//	    不在任何 `add_step` 步骤表内。
//	    前瞻消解（三条已跟踪面，逐条按「一旦被跟踪」消解 —— ★ 以下描述**故意不写触发子串**：
//	    三条门都是**正则扫源码文本**，注释**不豁免** ⇒ 把触发形态写进注释 = 自投罗网，
//	    本件头一稿正是这么被 `check-wall-layer` 判红的）：
//	      · `check-hardcoded-private-paths`（扫 `core/` 逐行找三枚私有绝对路径字面量）
//	        ⇒ 本件**零命中**：全文不写那三枚卷根/主目录字面量（相对串走 `cafbPyJoin` · HOME 走 `os.UserHomeDir`）。
//	      · `check-error-kinds`（扫同目录非测试件，正则取 `setErr` 调用后的引号串 与 JSON 里的 kind 键值）
//	        ⇒ 本件**零命中**：全文不出现该调用形态 · 不出现该 JSON 键。
//	      · `check-wall-layer`（扫同目录，正则取层级常量赋值 与 命令树里的层级标记）
//	        ⇒ 本件**零命中**：全文不出现层级常量名 · 不出现该标记形态。
//
// 遍历 / 排序点（★ 逐处交代）：
//
//	① 本件**无目录遍历**（✗ 无 `os.walk` / `glob` / `os.listdir` 参与输出）—— 取数全是**定点读**：
//	   `family_approve.go` / `<state>/edit_audit.jsonl` / `<state>/approvals/dev_edit.json`。
//	   唯一的 `os.listdir` 出现在 py **自检⑨**里（`_baks = os.listdir(_bdir)`），只用 `len(_baks)` 与
//	   `_baks[0]` 取备份名 ⇒ **行序不决定任何输出字节**（✗ 无 map 迭代 · ✗ 无排序面）。
//	② 输出行序的决定者 = py 源码里的**固定 print 序**（scan 的 ①②③④ + 判据行按 `judge_all` 的
//	   `problems` 追加序）+ 自检的**固定 16 格序**。✗ 零 map 迭代进输出。
//	③ `judge_mark_lines` / `judge_event_count` 按审计件**原行序**（`splitlines()` 顺序）遍历 ⇒ Go 保切片序。
//
// 已对拍面（两环境 · 见回执「对拍全表」）：甲（真仓 + 显式传根/状态/gate · py 走真仓原件绝对路径）·
// 乙（仓外夹具 `/private/var/tmp/…` + 每夹具自放 py 副本）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助正文 / 未知旗标：py 用 `argparse.ArgumentParser` ⇒ `-h/--help` 打 argparse 帮助正文、
//	   未知旗标打 `usage…error: unrecognized arguments` —— 本入口按「未知旗标 ⇒ rc=2 · `-h/--help` ⇒
//	   打一行简短 usage」处理，**正文与 py 不同**（对拍一律不喂帮助 / 未知旗标）。
//	② 默认根定位与**目标件自身**定位：见声明行 (a)（`--gate` 是有意旋钮面差）。
//	③ 异常路径（py **C 类无最外层兜底** · ✗ 不改 py）：读不到的件（权限 / 是目录）/ 非 UTF-8 ——
//	   py 抛 traceback 且 **stderr 含跑测机自身的路径帧**（如 CPython 的 `codecs.py`）⇒ 本入口
//	   **不写死环境路径**，只对齐 rc/stdout，stderr 走声明文案 `读不了的件（<相对件>）：…` ⇒ **rc 面同、正文不同**。
//	④ 正则语义：py 的四个取数正则（`type approvalTicketFile struct \{(.*?)\n\}` 等）逐字对齐为 Go
//	   `regexp`（RE2 也是 leftmost-first，`.*?` 非贪婪同语义）；`[0-9a-f]{64}` 是 ASCII 字符类 ⇒ 无 `\d`
//	   Unicode 展开问题。**常量行正则 `(?m)^(DEV_EDIT_SHA256 = ")([0-9a-f]{64})(")$` 逐字对齐**
//	   （含前导 `^` / 尾随 `"` 的空白类：本 pattern **无 `\s`**，故无 Unicode 空白展开面）。
//	⑤ `open()` 文本模式折行：`emit_constant` 用 `open(..., encoding="utf-8")`（**折 `\r\n`/裸 `\r` 为 `\n`**），
//	   而 `refresh_constant` 用 `open(..., "rb")` + `decode`（**不折**）—— 本入口逐处跟随（见 `cafbReadText`
//	   ⟷ 直读字节）。真仓两件均 LF ⇒ 未触发。
//	⑥ py `str.strip()` 是 **Unicode 语义**（含 `\x1c-\x1f`）⟷ Go `strings.TrimSpace`（不含）⇒ 本入口自写
//	   `cafbPyStrip`（`unicode.IsSpace` ∪ `\x1c-\x1f`）。
//	⑦ `os.path.abspath`（词法）⟷ `filepath.Abs`（`Clean`）在 `//a`（POSIX 保留前导双斜杠）一类极端串上不同
//	   （`posixpath.normpath('//a/b')` 保 `//a/b` · `filepath.Clean` 折成 `/a/b`）⇒ 真仓/夹具路径均单斜杠起头 ⇒ 未触发。
//	⑧ `mark_of` 的**异常正文**：py 的 `json.loads` 失败串（`json.JSONDecodeError`）与 `%r` 元组形态
//	   （`明标不是布尔（(…,)）`）本入口只能近似 ⇒ 该路径在真仓/夹具/自检里**均不触发**（见回执「未覆盖面」）。
//	⑨ 自检里的 `tempfile.mkdtemp(prefix=…)` ⟷ Go `os.MkdirTemp("", prefix)`：两者都读 `TMPDIR`；临时目录名
//	   **不进 stdout**（夹具内 `scan()` 输出被捕获后只做 `in` 断言）⇒ 逐字对齐不受影响。
//	⑩ **B5 登记常量的取数口（缺口 `GAP-20261004-09` 的治法 A）**：py 半判 B5 读的是**本件模块级常量**
//	   `DEV_EDIT_SHA256`（值在 py 件第 79 行）；本入口改为**从同一件现读**（`cafbReadRegisteredConst`
//	   读 `gatePath` = `--gate` / env `ZERG_BACKFILL_GATE` / 缺省 `cafbGateRel` 指向的
//	   `scripts/gates/check-approve-backfill.py`）—— 与 `--refresh-constant` 的取数口/写入点同处。
//	   ★ 两态结论一致；**唯一有意差**：py 的 scan 不受 `--gate` 影响（读模块常量），而本入口走 `gatePath`
//	   ⇒ 唯「显式传一个**常量与之不同**的 `--gate`」时两态才可能不同（正常档：缺省，或对拍传同一件 ⇒ 无差）。
//	   因此**多出一条 py 侧没有的 fail-closed 分支**：`gatePath` 读不到 / 常量行解不出 ⇒ rc=2「不出结论」
//	   （✗ 不回落编译期快照 —— 那正是本缺口要治的分叉源）。正常档该件恒在场 ⇒ 未触发；
//	   自检夹具为此**多写一份合成常量件**（py 夹具无需文件 —— 其常量是模块级；两态 stdout 仍逐字同）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const cafbVersion = "check-approve-backfill.py v1.1.0"
const cafbConstName = "DEV_EDIT_SHA256"
const cafbMarkField = "signed_not_in_person"
const cafbEvent = "approve_signed"

// cafbDevEditSHA256 —— B5 登记常量 `DEV_EDIT_SHA256` 的**内化真源**（G10 · 2026-10-07 · 退役批）。
//
// ★ 为什么内化：原真源件 `scripts/gates/check-approve-backfill.py`（py 半）正被退役（M2 判据件 Go 化）。
//
//	本件（Go 半）判 B5 时原先**现读**该 py 件取常量（`cafbGateRel` · `cafbReadRegisteredConst`）；
//	该 py 件一退役即读不到 ⇒ 原先 fail-closed（rc=2）。⇒ 把该真源**内化进本件**：
//	py 件在盘时仍现读（保持与 py 半同一真源 · 两态逐字同）；py 件**退役（不在盘）**时回落到本常量
//	⇒ 删 py 不再 fail-closed（✗ 不因缺 py 判不出）。
//	值 = 原 py 件 `DEV_EDIT_SHA256` 现读值（2026-09-28 Mr2109 重签 `approvals/dev_edit.json` 后）。
//	★ 人签重签后该常量须跟新：py 件在盘时走 `--refresh-constant` 刷 py 件；py 件退役后改刷本行。
const cafbDevEditSHA256 = "6131d5af3baa8583d623b3be264a451a2546f0ecbb3662e93a19ca1c1180ad03"

// cafbVersion 等件级常量见下；★ B5 的登记常量**不再冻在件里**（缺口 `GAP-20261004-09`）：
//
//	两入口分叉的根因就是 Go 侧编译期 `const` 与 py 件内常量**各自硬编** —— 人签重签后
//	py 走 `--refresh-constant` 跟新值，而 Go 侧编译期快照不跟 ⇒ 门⑰ 步骤（py）rc=0 绿 /
//	`zerg gate check`（Go inproc）rc=1「B5 破」，同一枚判据两个入口给出相反结论。
//	治法（已拍 = 方案 A）：**Go 半判 B5 时读同一真源** —— 即与 `--refresh-constant` 的
//	取数口/写入点**同一处**（`cafbGateRel` 指向的那件 `scripts/gates/check-approve-backfill.py`，
//	其第 79 行就是 py 半判 B5 读的同一个 `DEV_EDIT_SHA256`）⇒ 单源、两入口永不脱钩。
//	（本入口的 emit/refresh 通道本就以该件为默认目标 —— 见 main 的 `gatePath` 缺省。）

// cafbTicketEight —— 批准件八格（B1 的判据常量 · 序即判据序）。
var cafbTicketEight = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "sig"}

// cafbLegacySix —— 现网 `edit_audit.jsonl` 六格（`I-6`：新概念不许蹭）。
var cafbLegacySix = []string{"before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval", "approver"}

var (
	cafbTicketGoRel   = cafbPyJoin("core", "cmd", "zerg", "family_approve.go")
	cafbTicketRel     = cafbPyJoin("approvals", "dev_edit.json")
	cafbAuditFileName = "edit_audit.jsonl"
	// cafbGateRel —— B5 登记常量的**单一真源件**：与 `--refresh-constant` 的取数口/写入点同处
	// （`scripts/gates/check-approve-backfill.py`）。py 半的 B5 读的就是这一份件内常量 ⇒ 两入口不脱钩。
	cafbGateRel = cafbPyJoin("scripts", "gates", "check-approve-backfill.py")
)

var (
	// py `re.compile(r'(?m)^(DEV_EDIT_SHA256 = ")([0-9a-f]{64})(")$')` —— 逐字对齐（无 `\d`/`\s`/后顾）。
	cafbConstRE = regexp.MustCompile(`(?m)^(DEV_EDIT_SHA256 = ")([0-9a-f]{64})(")$`)
	// py `re.search(r"type approvalTicketFile struct \{(.*?)\n\}", src, re.S)`
	cafbTicketStructRE = regexp.MustCompile(`(?s)type approvalTicketFile struct \{(.*?)\n\}`)
	// py `re.search(r"type approvalAuditLine struct \{(.*?)\n\}", src, re.S)`
	cafbAuditStructRE = regexp.MustCompile(`(?s)type approvalAuditLine struct \{(.*?)\n\}`)
	// py `re.search(r"var approveAuditOwnFields = \[\]string\{([^}]*)\}", src)`
	cafbOwnFieldsRE = regexp.MustCompile(`var approveAuditOwnFields = \[\]string\{([^}]*)\}`)
	// py `re.findall(r'json:"([^"]+)"', …)`
	cafbJSONTagRE = regexp.MustCompile(`json:"([^"]+)"`)
	// py `re.findall(r'"([^"]+)"', …)`
	cafbQuotedRE = regexp.MustCompile(`"([^"]+)"`)
)

// cafbPyJoin —— 逐字复刻 py `posixpath.join`（★ 三类语义差一次写全）：
//
//	① **空段**：`join('a','')` = `'a/'`（py 会补分隔符）· `join('','a')` = `'a'` —— `filepath.Join` 两态都得 `'a'`；
//	② **后段绝对丢前段**：`join('a','/b')` = `'/b'` —— `filepath.Join` 得 `'a/b'`；
//	③ **不归一 `..`**：`join('a','..','b')` = `'a/../b'` —— `filepath.Join` 归一成 `'b'`。
//
// ★ 本件所有 py `os.path.join(...)` 调用点改用本函数（`cafbReadAll` / 相对串常量 / 自检夹具 / `cafbDefaultBackupDir`
// 的 `os.path.join(expanduser("~"), ".zerg", "state", "gate-backups")`），逐处见回执「Join 类逐处交代」。
// 本件缺省串**不含 `..` 段**、也**不以 `/` 开头的后段**接续 ⇒ 与 `filepath.Join` 在真仓/夹具两态实际同结果，
// 但为**语义对齐**仍全走本函数。
func cafbPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// cafbAbs —— py `os.path.abspath`（词法 · ✗ 不解析符号链接）。见未覆盖面⑦。
func cafbAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// cafbDefaultRoot —— py `DEFAULT_ROOT`（脚本位置上溯两级）。Go 无 `__file__` ⇒ 取 cwd（声明行 (a)）。
func cafbDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// cafbPyIsSpace —— py `str.isspace()` 的字符集：Unicode White_Space ∪ `\x1c-\x1f`（Go `unicode.IsSpace` 不含后者）。
func cafbPyIsSpace(r rune) bool {
	switch r {
	case 0x1c, 0x1d, 0x1e, 0x1f:
		return true
	}
	return unicode.IsSpace(r)
}

// cafbPyStrip —— py `s.strip()`（Unicode 语义 · ✗ 非 `strings.TrimSpace`）。见未覆盖面⑥。
func cafbPyStrip(s string) string {
	return strings.TrimFunc(s, cafbPyIsSpace)
}

// cafbPySplitLines —— py `str.splitlines()`：按 `\n` / `\r` / `\v` / `\f` / `\x1c-\x1e` / `\x85` / `\u2028` / `\u2029`
// 断行，**不留尾空行**。★ py 读件走文本模式（`cafbReadText` 已折 `\r`）⇒ `\r` 支实际不触发，仍逐字保留。
func cafbPySplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur.WriteRune(rs[i])
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cafbUniversalNewlines —— py `open()` 文本模式的换行折平：`\r\n` → `\n` · 裸 `\r` → `\n`（逐字节安全，`\r`/`\n` 皆 ASCII）。
func cafbUniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			b.WriteByte('\n')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// cafbReadText —— py `open(path, encoding="utf-8")`（文本模式 · 折行）。非 UTF-8 的抛错面见未覆盖面③。
func cafbReadText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return cafbUniversalNewlines(string(raw)), nil
}

// cafbExists —— py `os.path.exists`（**跟随符号链接** ⇒ Go 用 `os.Stat`，✗ 非 `os.Lstat`）。见标准条第⑨点。
func cafbExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// cafbIsDir —— py `os.path.isdir`（**跟随符号链接** ⇒ `os.Stat`，✗ 非 `Entry.IsDir()` 的 lstat 语义）。
func cafbIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// cafbSha256File —— py `sha256_file`（整件字节 · 含末尾换行）。
func cafbSha256File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// cafbPyBool —— py `%s` 格式化布尔 ⇒ `True`/`False`。
func cafbPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cafbPyRepr —— py `repr(str)`（判据文案里的 `%r`）：默认单引号；串内含 `'` 且不含 `"` 时用双引号。
func cafbPyRepr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")
	q := "'"
	if hasSingle && !hasDouble {
		q = "\""
	}
	body := strings.ReplaceAll(s, `\`, `\\`)
	body = strings.ReplaceAll(body, "\n", `\n`)
	body = strings.ReplaceAll(body, "\r", `\r`)
	body = strings.ReplaceAll(body, "\t", `\t`)
	body = strings.ReplaceAll(body, q, `\`+q)
	return q + body + q
}

// cafbPyReprTuple1 —— py `%r` of `(v,)`（单元素元组 · 仅 `mark_of` 的「明标不是布尔」一路用到）。
func cafbPyReprTuple1(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "(None,)"
	case bool:
		return "(" + cafbPyBool(x) + ",)"
	case float64:
		return "(" + cafbPyNumber(x) + ",)"
	case string:
		return "(" + cafbPyRepr(x) + ",)"
	}
	return "(" + fmt.Sprintf("%v", v) + ",)"
}

// cafbPyNumber —— py `%s`/`repr` 对 JSON 数字的形态（整数不带 `.0`）。
func cafbPyNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%v", f)
}

// cafbPyListRepr —— py `%s` 格式化列表（元素用 `repr`）：`[]` / `['B1 破']`。
func cafbPyListRepr(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, cafbPyRepr(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cafbStrEq —— py 列表逐字比较。
func cafbStrEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cafbContains —— py `x in list`。
func cafbContains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// cafbFindConstant —— py `find_constant`：返回 `(现读字面量, 行号, start, end)`；解不出来 ⇒ ok=false。
// ★ 行号 = `src[:m.start()].count("\n") + 1`（按字节数跟按码点数等价 —— 只数 `\n`）。
func cafbFindConstant(src string) (string, int, int, int, bool) {
	m := cafbConstRE.FindStringSubmatchIndex(src)
	if m == nil {
		return "", 0, 0, 0, false
	}
	lit := src[m[4]:m[5]]
	line := strings.Count(src[:m[0]], "\n") + 1
	return lit, line, m[4], m[5], true
}

// cafbReadRegisteredConst —— 判 B5 时取**登记常量**：读 `gatePath` 指向的那件（= py 半那一份，
// 见 `cafbGateRel` 注释），并用与 `--emit-constant`/`--refresh-constant` **同一套** `cafbFindConstant`
// 解析。★ 与 py 半「读本件模块级 `DEV_EDIT_SHA256`」同一真源 ⇒ 两入口结论一致（`GAP-20261004-09`）。
//
//	取不到（件不在 / 常量行解不出）⇒ ok=false。调用方 `cafbScan` 据此 fail-closed（rc=2 不给结论），
//	✗ 不回落编译期快照 —— 那正是本缺口要治的分叉源（见文件头「常量的单一真源」注）。
func cafbReadRegisteredConst(gatePath string) (string, bool) {
	src, err := cafbReadText(gatePath)
	if err != nil {
		return "", false
	}
	lit, _, _, _, ok := cafbFindConstant(src)
	if !ok {
		return "", false
	}
	return lit, true
}

// cafbParseTicketJSONTags —— py `parse_ticket_json_tags`：`(None,…)` ⟷ `ok=false`；命中但 0 串 ⇒ **空切片 + ok=true**。
func cafbParseTicketJSONTags(src string) ([]string, bool) {
	m := cafbTicketStructRE.FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	return cafbFindall1(cafbJSONTagRE, m[1]), true
}

// cafbParseAuditTags —— py `parse_audit_tags`。
func cafbParseAuditTags(src string) ([]string, bool) {
	m := cafbAuditStructRE.FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	return cafbFindall1(cafbJSONTagRE, m[1]), true
}

// cafbParseOwnFields —— py `parse_own_fields`。
func cafbParseOwnFields(src string) ([]string, bool) {
	m := cafbOwnFieldsRE.FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	return cafbFindall1(cafbQuotedRE, m[1]), true
}

// cafbFindall1 —— py `re.findall` 单一捕获组 ⇒ 命中串切片（**保原序** · ✗ 零 map）。
func cafbFindall1(re *regexp.Regexp, s string) []string {
	hits := re.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h[1])
	}
	return out
}

// ── 判定口（纯函数：自检与真目标**共用同一套**）────────────────────────────────────────

// cafbJudgeTicketFields —— py `judge_ticket_fields`（B1）。
func cafbJudgeTicketFields(tags []string, have bool) []string {
	if !have {
		return []string{"B1 未取数：`approvalTicketFile` 的结构体解不出来"}
	}
	problems := []string{}
	if !cafbStrEq(tags, cafbTicketEight) {
		problems = append(problems, fmt.Sprintf("B1 破：`approvalTicketFile` 标签 = %s ≠ 八格 %s",
			strings.Join(tags, ","), strings.Join(cafbTicketEight, ",")))
	}
	if cafbContains(tags, cafbMarkField) {
		problems = append(problems, fmt.Sprintf("B1 破：明标 %s 进了**件字段面** —— 那是 A 案（已拍 = B 案 · 不许改件字段面 ✗）",
			cafbPyRepr(cafbMarkField)))
	}
	if cafbContains(tags, "in_person") {
		problems = append(problems, "B1 破：`in_person` 进了件字段面（同上 · A 案形态）")
	}
	return problems
}

// cafbJudgeAuditSide —— py `judge_audit_side`（B2 · `own` 缺省 = 空切片 ⟷ py `own or []`）。
func cafbJudgeAuditSide(tags []string, have bool, own []string) []string {
	if !have {
		return []string{"B2 未取数：`approvalAuditLine` 的结构体解不出来"}
	}
	problems := []string{}
	if !cafbContains(tags, cafbMarkField) {
		problems = append(problems, fmt.Sprintf("B2 破：审计行里**没有**明标 %s（B 案的明标就该落在这里）", cafbPyRepr(cafbMarkField)))
	}
	for _, bad := range []string{"before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval"} {
		if cafbContains(tags, bad) {
			problems = append(problems, fmt.Sprintf("B2 破：审计行蹭了现网那一格 %s（`I-6`：新概念要**自己的字段名**）", cafbPyRepr(bad)))
		}
	}
	for _, t := range own {
		if cafbContains(cafbLegacySix, t) {
			problems = append(problems, fmt.Sprintf("B2 破：**新**字段名 %s 与现网六格同名（`I-6`）", cafbPyRepr(t)))
		}
	}
	return problems
}

// cafbMarkOf —— py `mark_of`：一行审计原始 JSON ⇒ ("在场"|"补签", "", true) / ("", 理由, false)。
func cafbMarkOf(rawLine string) (string, string, bool) {
	var row map[string]interface{}
	if err := json.Unmarshal([]byte(rawLine), &row); err != nil {
		return "", fmt.Sprintf("审计行解不动：%s", err), false
	}
	if row == nil {
		// py：`json.loads` 成功但**不是** dict（`null`/标量/数组）⇒ `MARK_FIELD not in row` 抛 TypeError（C 类无兜底）。
		// 该路径在真仓/夹具/自检里均不触发；此处按「不可分」如实报（见未覆盖面⑧）。
		return "", "审计行解不动：顶层不是对象", false
	}
	v, present := row[cafbMarkField]
	if !present {
		return "", fmt.Sprintf("这一行**没有明标**（%s 不在）⇒ 不可分：当场判红", cafbPyRepr(cafbMarkField)), false
	}
	b, isBool := v.(bool)
	if !isBool {
		return "", fmt.Sprintf("明标不是布尔（%s）⇒ 判红", cafbPyReprTuple1(v)), false
	}
	if b {
		return "补签", "", true
	}
	return "在场", "", true
}

// cafbReading —— 判定口的读数（py `readings` 的元素 `(行号, 判定)`）。
type cafbReading struct {
	line    int
	verdict string
}

// cafbJudgeMarkLines —— py `judge_mark_lines`（B3）：返回 (问题行, 读数[(行号, 判定)])。
func cafbJudgeMarkLines(lines []string) ([]string, []cafbReading) {
	problems := []string{}
	readings := []cafbReading{}
	for i, ln := range lines {
		if cafbPyStrip(ln) == "" {
			continue
		}
		verdict, errMsg, ok := cafbMarkOf(ln)
		if !ok {
			problems = append(problems, fmt.Sprintf("B3 破（第 %d 行）：%s", i+1, errMsg))
			continue
		}
		readings = append(readings, cafbReading{line: i + 1, verdict: verdict})
	}
	if len(readings) == 0 && len(problems) == 0 {
		problems = append(problems, "B3 破：一行都没判到（空夹具 ⇒ 不许当「全在场」）")
	}
	return problems, readings
}

// cafbJudgeEventCount —— py `judge_event_count`（B4）：现网审计行数 + 事件条数 + 其中补签条数。
func cafbJudgeEventCount(rows []string) (int, int, int) {
	n, ev, bf := 0, 0, 0
	for _, r := range rows {
		if cafbPyStrip(r) != "" {
			n++
		}
	}
	for _, r := range rows {
		if cafbPyStrip(r) == "" {
			continue
		}
		var d map[string]interface{}
		if err := json.Unmarshal([]byte(r), &d); err != nil {
			continue
		}
		if d == nil {
			continue // py：`None.get` ⇒ AttributeError ⇒ 被 `except` 吞掉（同效果）
		}
		if s, ok := d["event"].(string); ok && s == cafbEvent {
			ev++
			if v, ok := d[cafbMarkField].(bool); ok && v {
				bf++
			}
		}
	}
	return n, ev, bf
}

// cafbJSONLine —— py `json.dumps({...})` 的等价物（判据口喂构造行；语义等价即可，✗ 不比字面串）。
func cafbJSONLine(kv map[string]interface{}) string {
	b, err := json.Marshal(kv)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ── 取数 ─────────────────────────────────────────────────────────────────────────

// cafbStateDirOf —— py `state_dir_of`（旋钮名照 py 同名旗标 + env；缺省推导逐字同构）。
func cafbStateDirOf(given string) string {
	if given != "" {
		return cafbAbs(given)
	}
	if env := os.Getenv("ZERG_STATE_DIR"); env != "" {
		return cafbAbs(env)
	}
	home := os.Getenv("HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	return cafbPyJoin(home, ".zerg", "state")
}

// cafbDefaultBackupDir —— py `default_backup_dir()`：`join(expanduser("~"), ".zerg", "state", "gate-backups")`。
func cafbDefaultBackupDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	return cafbPyJoin(home, ".zerg", "state", "gate-backups")
}

// cafbInside —— py `_inside`：`abspath` 后 `path == root or path.startswith(root + os.sep)`。
func cafbInside(path, root string) bool {
	p, r := cafbAbs(path), cafbAbs(root)
	return p == r || strings.HasPrefix(p, r+"/")
}

// cafbReadings —— py `read_all` 的返回 dict。
type cafbReadings struct {
	ticketTags []string
	ticketHave bool
	auditTags  []string
	auditHave  bool
	ownFields  []string
	rows       []string
	state      string
	auditPath  string
	ticket     string
	devSHA     string
}

// cafbReadAll —— py `read_all`：任一面取不到 ⇒ (nil, 理由)。
//
//	★ py **C 类无最外层兜底**：读不到的件（权限 / 是目录）/ 非 UTF-8 ⇒ py 抛 traceback（rc=1）；
//	本入口 fail-closed（同一出口 rc=2，**stderr 走声明文案** —— 见未覆盖面③：不写死跑测机路径）。
func cafbReadAll(repo, state string) (*cafbReadings, string) {
	goPath := cafbPyJoin(repo, cafbTicketGoRel)
	if !cafbExists(goPath) {
		return nil, fmt.Sprintf("读不到 `%s`", cafbTicketGoRel)
	}
	src, err := cafbReadText(goPath)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", cafbTicketGoRel, err)
	}
	ticketTags, haveT := cafbParseTicketJSONTags(src)
	if !haveT {
		return nil, fmt.Sprintf("`approvalTicketFile` 在 %s 里解不出来", cafbTicketGoRel)
	}
	ownFields, haveO := cafbParseOwnFields(src)
	if !haveO {
		return nil, fmt.Sprintf("`approveAuditOwnFields` 在 %s 里解不出来（`E1` 自己的字段名那一份不在）", cafbTicketGoRel)
	}
	auditTags, haveA := cafbParseAuditTags(src)
	if !haveA {
		return nil, fmt.Sprintf("`approvalAuditLine` 在 %s 里解不出来（`E1` 的审计行结构不在 ⇒ 明标无处落）", cafbTicketGoRel)
	}
	audit := cafbPyJoin(state, cafbAuditFileName)
	if !cafbIsDir(state) {
		return nil, fmt.Sprintf("状态目录不在（%s）", state)
	}
	var rows []string
	if cafbExists(audit) {
		txt, err := cafbReadText(audit)
		if err != nil {
			return nil, fmt.Sprintf("读不了的件（%s）：%v", audit, err)
		}
		rows = cafbPySplitLines(txt)
	} else {
		return nil, fmt.Sprintf("审计件不在（%s）⇒ 未取数（**不许**把「文件不存在」当没问题）", audit)
	}
	ticket := cafbPyJoin(state, cafbTicketRel)
	if !cafbExists(ticket) {
		return nil, fmt.Sprintf("在册那枚批准件不在（%s）⇒ 未取数", ticket)
	}
	devSHA, err := cafbSha256File(ticket)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", ticket, err)
	}
	return &cafbReadings{
		ticketTags: ticketTags, ticketHave: true,
		auditTags: auditTags, auditHave: true,
		ownFields: ownFields, rows: rows,
		state: state, auditPath: audit, ticket: ticket, devSHA: devSHA,
	}, ""
}

// cafbJudgeAll —— py `judge_all`。返回 (问题行, (行数, 事件数, 补签数))。
// ★ `regConst` = B5 的**登记常量**（由 `cafbReadRegisteredConst` 从同一真源读出后传入；
// ✗ 不再是件内编译期快照 —— 见 `GAP-20261004-09`）。
func cafbJudgeAll(r *cafbReadings, regConst string) ([]string, [3]int) {
	problems := []string{}
	problems = append(problems, cafbJudgeTicketFields(r.ticketTags, r.ticketHave)...)
	problems = append(problems, cafbJudgeAuditSide(r.auditTags, r.auditHave, r.ownFields)...)
	// B3 的**真目标**那一半：正控两行要过，**负控**（抹掉明标）要**判红**。
	rowIn := cafbJSONLine(map[string]interface{}{"event": cafbEvent, cafbMarkField: false})
	rowBack := cafbJSONLine(map[string]interface{}{"event": cafbEvent, cafbMarkField: true})
	mp1, _ := cafbJudgeMarkLines([]string{rowIn, rowBack})
	problems = append(problems, mp1...)
	rowNo := cafbJSONLine(map[string]interface{}{"event": cafbEvent})
	mp2, _ := cafbJudgeMarkLines([]string{rowNo})
	if len(mp2) == 0 {
		problems = append(problems, "B3 破：抹掉明标的那一行**没判红**（判定口没有区分度）")
	}
	n, ev, bf := cafbJudgeEventCount(r.rows)
	if bf > 0 {
		problems = append(problems, fmt.Sprintf("B4 破：现网审计里有 %d 条**补签**事件（`%s=true`）—— 本版补签那一路**不实现** ⇒ 该是 0 条（在场签名含 `%s` 事件属正常，不判）",
			bf, cafbMarkField, cafbEvent))
	}
	// ★ 缺口 `GAP-20260928-88`：红行**必须印出应有值**（判据一字不放宽 · 仍逐字比 sha256）。
	// ★ 登记值 `regConst` 由同一真源现读（缺口 `GAP-20261004-09`），✗ 不是件内编译期快照。
	if r.devSHA != regConst {
		problems = append(problems, fmt.Sprintf("B5 破：在册 `dev_edit.json` 的 sha256 = %s ≠ 登记值（明标没进件 = 它一个字节不该动）· **应改为** %s = \"%s\" —— 正门刷值：`zerg gate check check-approve-backfill --refresh-constant`（先干跑）· 加 `--yes` 才落（只读看值：`--emit-constant`）",
			r.devSHA, cafbConstName, r.devSHA))
	}
	return problems, [3]int{n, ev, bf}
}

// ── 常量刷新通道（只读 emit + 最小替换写入 · 缺口 `GAP-20260928-88`）──────────────────

// cafbEmitConstant —— py `emit_constant`（只读档 · 一个字节都不写）。rc：0 一致 / 1 不一致 / 2 未取数。
func cafbEmitConstant(out io.Writer, gatePath string, devSHA *string) int {
	fmt.Fprintln(out, "── 常量发射档（只读 · 一个字节都不写）")
	fmt.Fprintf(out, "   常量       : %s（件内 = %s）\n", cafbConstName, gatePath)
	fmt.Fprintln(out, "   口径       : 在册批准件的**整件文件** sha256（含末尾换行）—— 人做一次合法重签")
	fmt.Fprintln(out, "                （`zerg approve new`）该值必变 ⇒ 冻在件里就会陈腐（缺口 GAP-20260928-88）")
	if devSHA == nil {
		fmt.Fprintln(out, "⛔ 未取数：真源件读不到 ⇒ **不出结论**（rc=2）")
		return 2
	}
	fmt.Fprintf(out, "   真源件     : %s\n", cafbTicketRel)
	fmt.Fprintf(out, "   应有值     : %s\n", *devSHA)
	src, err := cafbReadText(gatePath)
	if err != nil {
		fmt.Fprintf(out, "⛔ 未取数：件读不到（%v）⇒ **不出结论**（rc=2）\n", err)
		return 2
	}
	cur, line, _, _, ok := cafbFindConstant(src)
	if !ok {
		fmt.Fprintf(out, "⛔ 未取数：常量行 `%s = \"…\"` 在件里解不出来 ⇒ **不出结论**（rc=2）\n", cafbConstName)
		return 2
	}
	fmt.Fprintf(out, "   件内现值   : %s（第 %d 行）\n", cur, line)
	if cur == *devSHA {
		fmt.Fprintln(out, "   判定       : 一致 ⇒ 门⑰ 的 B5 该过")
		fmt.Fprintf(out, "   %s = \"%s\"\n", cafbConstName, cur)
		return 0
	}
	fmt.Fprintln(out, "   判定       : **不一致** ⇒ 门⑰ 现读 B5 破 · 该刷成下面这一行")
	fmt.Fprintf(out, "   %s = \"%s\"\n", cafbConstName, *devSHA)
	fmt.Fprintln(out, "   正门       : zerg gate check check-approve-backfill --refresh-constant --yes（先干跑不带 `--yes` · 只读再看一遍用 `--emit-constant`）")
	return 1
}

// cafbRefreshConstant —— py `refresh_constant`：**默认干跑不写**；apply=true 才做等长最小替换 + 仓外备份。
// ★ 读件走 **`open(..., "rb")` + decode**（**不折行**）⟷ py `emit_constant` 走文本模式（**折行**）—— 逐处跟随。
func cafbRefreshConstant(out io.Writer, gatePath string, devSHA *string, apply bool, backupDir string, repoRoot string) int {
	if devSHA == nil {
		fmt.Fprintln(out, "⛔ 未取数：真源件读不到 ⇒ 拒写（rc=2）")
		return 2
	}
	raw, err := os.ReadFile(gatePath)
	if err != nil {
		fmt.Fprintf(out, "⛔ 未取数：件读不到（%v）⇒ 拒写（rc=2）\n", err)
		return 2
	}
	src := string(raw)
	cur, line, start, end, ok := cafbFindConstant(src)
	if !ok {
		fmt.Fprintf(out, "⛔ 未取数：常量行 `%s = \"…\"` 解不出来 ⇒ 拒写（rc=2）\n", cafbConstName)
		return 2
	}
	if apply {
		fmt.Fprintln(out, "── 常量刷新档（**真写**）")
	} else {
		fmt.Fprintln(out, "── 常量刷新档（**干跑** · 未写件）")
	}
	fmt.Fprintf(out, "   目标件 : %s\n", gatePath)
	fmt.Fprintf(out, "   替换点 : 第 %d 行 · 最小文本替换（只动那 64 位字面量 · 其余逐字节不动）\n", line)
	fmt.Fprintf(out, "   现值   : %s\n", cur)
	fmt.Fprintf(out, "   应有值 : %s\n", *devSHA)
	if cur == *devSHA {
		fmt.Fprintln(out, "   ⇒ 现值已 == 应有值 ⇒ 无需动笔（rc=0）")
		return 0
	}
	if !apply {
		fmt.Fprintln(out, "   ⇒ **干跑未写**：加 `--yes` 才落（本门默认不写件）")
		return 0
	}
	bdir := cafbAbs(backupDir)
	if backupDir == "" {
		bdir = cafbAbs(cafbDefaultBackupDir())
	}
	if repoRoot != "" && cafbInside(bdir, repoRoot) {
		fmt.Fprintf(out, "⛔ 拒写：备份落点在**仓内**（%s ⊂ %s）—— 备份必须落仓外（rc=2）\n", bdir, repoRoot)
		return 2
	}
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		fmt.Fprintf(out, "⛔ 未取数：备份目录建不起来（%v）⇒ 拒写（rc=2）\n", err)
		return 2
	}
	bpath := cafbPyJoin(bdir, filepath.Base(gatePath)+"."+time.Now().Format("20060102-150405")+".bak")
	if err := os.WriteFile(bpath, raw, 0o644); err != nil {
		fmt.Fprintf(out, "⛔ 未取数：备份落不下去（%v）⇒ 拒写（rc=2）\n", err)
		return 2
	}
	newSrc := src[:start] + *devSHA + src[end:]
	newRaw := []byte(newSrc)
	if err := os.WriteFile(gatePath, newRaw, 0o644); err != nil {
		fmt.Fprintf(out, "⛔ 未取数：件落不下去（%v）⇒ 拒写（rc=2）\n", err)
		return 2
	}
	fmt.Fprintf(out, "   备份   : %s（仓外 · 改前整件 %d B）\n", bpath, len(raw))
	fmt.Fprintf(out, "   字节   : 前 %d B → 后 %d B（等长替换 ⇒ 除那一个字面量外一字未动）\n", len(raw), len(newRaw))
	fmt.Fprintln(out, "   ⇒ 已落（rc=0）：复跑门⑰ 单步该转绿（自检格数不变）")
	return 0
}

// ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

// cafbSelfTest —— py `self_test`（16 格 · 真目标只读）。返 false ⟷ py 返 `bad != 0`（main 据 `not` 判 rc）。
func cafbSelfTest(stdout io.Writer, repo, state string) bool {
	_ = repo
	_ = state
	ok, bad := 0, 0
	check := func(cond bool, msgOK, msgBad string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", msgOK)
			ok++
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", msgBad)
			bad++
		}
	}

	check(len(cafbJudgeTicketFields(cafbTicketEight, true)) == 0,
		"① 正控：件八格**逐字**在 ⇒ 不判红（B1）",
		"① 正控失败：八格齐全还被判红")
	withMark := append(append([]string{}, cafbTicketEight...), cafbMarkField)
	check(len(cafbJudgeTicketFields(withMark, true)) > 0,
		"② 负控：明标进了**件字段面** ⇒ 判红（A 案形态 · 已拍 B 案）",
		"② 负控失败：明标进件还判绿")
	check(len(cafbJudgeAuditSide([]string{"at", "event", "tool", cafbMarkField, "approval_path"}, true, []string{"approval_path"})) == 0,
		"③ 正控：明标在**审计侧**且不蹭六格 ⇒ 不判红（B2）",
		"③ 正控失败：B 案形态被判红")
	check(len(cafbJudgeAuditSide([]string{"at", "event", "before_sha256", cafbMarkField}, true, []string{"approval_path"})) > 0,
		"④ 负控：审计行蹭了 `before_sha256` ⇒ 判红（`I-6`）",
		"④ 负控失败：蹭了现网那格还判绿")

	vIn, eIn, okIn := cafbMarkOf(cafbJSONLine(map[string]interface{}{cafbMarkField: false}))
	vBack, eBack, okBack := cafbMarkOf(cafbJSONLine(map[string]interface{}{cafbMarkField: true}))
	check(okIn && vIn == "在场" && eIn == "" && okBack && vBack == "补签" && eBack == "",
		"⑤ 正控：明标 `false` ⇒ 在场 · `true` ⇒ 补签（有区分度）",
		fmt.Sprintf("⑤ 正控失败：两种明标判不出区别（%s/%s）", vIn, vBack))

	vNo, eNo, okNo := cafbMarkOf(cafbJSONLine(map[string]interface{}{"event": cafbEvent}))
	check(!okNo && vNo == "" && eNo != "",
		"⑥ 负控：**抹掉明标** ⇒ 判红（不可分不许蒙过去）",
		"⑥ 负控失败：没明标还判绿")

	// ── ★ 回归自测格（缺口 `GAP-20260928-51`）：**真判据面**不许哑化 ──
	//   拿一份按现读判据面形状造的合成夹具（仓外 `mkdtemp`：假 `family_approve.go` + 假 `edit_audit.jsonl`
	//   + 假在册件）喂给**真判据面** `scan()`，断言它**真判红并逐字点名**。
	fieldsGo := ""
	for _, t := range cafbTicketEight {
		fieldsGo += fmt.Sprintf("\t%s string `json:\"%s\"`\n", t, t)
	}
	sigGo := "\tsig string `json:\"sig\"`\n"
	markGo := fmt.Sprintf("\t%s bool `json:\"%s\"`\n", cafbMarkField, cafbMarkField)
	auditGo := "type approvalAuditLine struct {\n" +
		"\tat string `json:\"at\"`\n" +
		"\tevent string `json:\"event\"`\n" +
		"\ttool string `json:\"tool\"`\n" +
		"\tapproval_path string `json:\"approval_path\"`\n" +
		markGo +
		"}\n"
	ownGo := fmt.Sprintf("var approveAuditOwnFields = []string{\"at\", \"event\", \"tool\", \"approval_path\", \"%s\"}\n", cafbMarkField)
	goOK := "package main\n\n" + "type approvalTicketFile struct {\n" + fieldsGo + "}\n\n" + auditGo + "\n" + ownGo
	// ⓐ 正控夹具一（A 案形态）：把明标**塞进件字段面**
	goBad := strings.ReplaceAll(goOK, sigGo, sigGo+markGo)

	rowIn := cafbJSONLine(map[string]interface{}{"event": cafbEvent, cafbMarkField: false})
	rowBack := cafbJSONLine(map[string]interface{}{"event": cafbEvent, cafbMarkField: true})

	fxRoot, fxErr := os.MkdirTemp("", "backfill-selftest-scan-")
	clean := [2]interface{}{"夹具建不起来", -99}
	hzTicket := [2]interface{}{"夹具建不起来", -99}
	hzRow := [2]interface{}{"夹具建不起来", -99}
	noaudit := [2]interface{}{"夹具建不起来", -99}

	mkFixture := func(name, goSrc string, rows []string) (string, string, error) {
		root := cafbPyJoin(fxRoot, name)
		fxRepo := cafbPyJoin(root, "repo")
		fxState := cafbPyJoin(root, "state")
		if err := os.MkdirAll(cafbPyJoin(fxRepo, "core", "cmd", "zerg"), 0o755); err != nil {
			return "", "", err
		}
		if err := os.MkdirAll(cafbPyJoin(fxState, "approvals"), 0o755); err != nil {
			return "", "", err
		}
		if err := os.WriteFile(cafbPyJoin(fxRepo, cafbTicketGoRel), []byte(goSrc), 0o644); err != nil {
			return "", "", err
		}
		// ★ 合成 B5 真源件（`cafbGateRel`）：`cafbScan` 现读登记常量用（缺口 `GAP-20261004-09`）。
		//   常量取**全 `2`**（≠ 合成在册件 `"{}\n"` 的 sha ⇒ B5 该判红，与 py 夹具同结论）。
		if err := os.MkdirAll(cafbPyJoin(fxRepo, "scripts", "gates"), 0o755); err != nil {
			return "", "", err
		}
		if err := os.WriteFile(cafbPyJoin(fxRepo, cafbGateRel),
			[]byte("#!/usr/bin/env python3\n"+cafbConstName+" = \""+strings.Repeat("2", 64)+"\"\n"), 0o644); err != nil {
			return "", "", err
		}
		if rows != nil {
			var b strings.Builder
			for _, r := range rows {
				b.WriteString(r + "\n")
			}
			if err := os.WriteFile(cafbPyJoin(fxState, cafbAuditFileName), []byte(b.String()), 0o644); err != nil {
				return "", "", err
			}
		}
		if err := os.WriteFile(cafbPyJoin(fxState, "approvals", "dev_edit.json"), []byte("{}\n"), 0o644); err != nil {
			return "", "", err
		}
		return fxRepo, fxState, nil
	}

	if fxErr == nil {
		fxCleanRepo, fxCleanState, e1 := mkFixture("clean", goOK, []string{rowIn})
		fxTicketRepo, fxTicketState, e2 := mkFixture("ticket-mark", goBad, []string{rowIn})
		fxRowRepo, fxRowState, e3 := mkFixture("audit-backfill", goOK, []string{rowIn, rowBack})
		fxNoRepo, fxNoState, e4 := mkFixture("no-audit", goOK, nil)
		if e := firstErr(e1, e2, e3, e4); e != nil {
			fxErr = e
		} else {
			clean = cafbRunFixture(fxCleanRepo, fxCleanState)
			hzTicket = cafbRunFixture(fxTicketRepo, fxTicketState)
			hzRow = cafbRunFixture(fxRowRepo, fxRowState)
			noaudit = cafbRunFixture(fxNoRepo, fxNoState)
		}
	}
	if fxErr != nil {
		fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%v）⇒ 不许静默绿（下列格全红）\n", "backfill-fixture", fxErr)
	}

	cell := func(label string, cond bool, why string) {
		msg := label + " | " + why
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", msg)
			ok++
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", msg)
			bad++
		}
	}
	rcOf := func(x [2]interface{}) int { return x[1].(int) }
	outOf := func(x [2]interface{}) string { return x[0].(string) }

	// ⓪ 先证夹具**真喂到了**真判据面（否则下列格是空转）
	cleanOut, cleanRC := outOf(clean), rcOf(clean)
	cell("scan-fixture-reaches-judge",
		cleanRC == 1 && strings.Contains(cleanOut, "B5 破") && strings.Contains(cleanOut, "件字段面"),
		fmt.Sprintf("真判据面 `scan()` 在合成夹具上取到数并走到判据（现读 rc=%d · 读到 B5 破=%s · 读到件字段面=%s）",
			cleanRC, cafbPyBool(strings.Contains(cleanOut, "B5 破")), cafbPyBool(strings.Contains(cleanOut, "件字段面"))))
	hzTicketOut, hzTicketRC := outOf(hzTicket), rcOf(hzTicket)
	tkHasB1 := strings.Contains(hzTicketOut, "B1 破")
	cell("scan-catches-mark-in-ticket",
		hzTicketRC == 1 && tkHasB1,
		fmt.Sprintf("`scan()` 对「明标进件」真判红并逐字点名 `B1 破`（现读 rc=%d · 点名=%s）",
			hzTicketRC, cafbPyBool(tkHasB1)))
	hzRowOut, hzRowRC := outOf(hzRow), rcOf(hzRow)
	rowHasB4 := strings.Contains(hzRowOut, "B4 破")
	rowHasOne := strings.Contains(hzRowOut, "1 条")
	cell("scan-catches-backfill-row",
		hzRowRC == 1 && rowHasB4 && rowHasOne,
		fmt.Sprintf("`scan()` 对台帐里的补签行真判红并点名条数（现读 rc=%d · 点名 `B4 破`=%s · 1 条=%s）",
			hzRowRC, cafbPyBool(rowHasB4), cafbPyBool(rowHasOne)))
	cleanHits := []string{}
	for _, k := range []string{"B1 破", "B3 破", "B4 破"} {
		if strings.Contains(cleanOut, k) {
			cleanHits = append(cleanHits, k)
		}
	}
	cell("scan-clean-has-no-induced-hit",
		cleanRC == 1 && len(cleanHits) == 0,
		fmt.Sprintf("干净夹具上植入面零命中（唯一红 = 合成在册件 sha 无前像的 B5 · 现读 rc=%d · 命中=%s）",
			cleanRC, cafbPyListRepr(cleanHits)))
	cleanHasB1 := strings.Contains(cleanOut, "B1 破")
	cleanHasB4 := strings.Contains(cleanOut, "B4 破")
	cell("scan-face-not-blind",
		!((tkHasB1 == cleanHasB1) && (rowHasB4 == cleanHasB4)),
		fmt.Sprintf("判据面哑化（空结果 ⇒ 两态同形）时本格判红 —— 现读两态已分开（正控命中 %s/%s · 对照命中 %s/%s）",
			cafbPyBool(tkHasB1), cafbPyBool(rowHasB4), cafbPyBool(cleanHasB1), cafbPyBool(cleanHasB4)))
	noOut, noRC := outOf(noaudit), rcOf(noaudit)
	noMissing := strings.Contains(noOut, "未取数")
	cell("scan-refuses-without-corpus",
		noRC == 2 && noMissing,
		fmt.Sprintf("审计件缺失时 rc=2 且写明「未取数」（现读 rc=%d · 未取数=%s）",
			noRC, cafbPyBool(noMissing)))
	if fxRoot != "" {
		_ = os.RemoveAll(fxRoot)
	}

	// ── ★ 缺口 `GAP-20260928-88` 的自证格：刷新通道**两态**都要有牙 ──
	fx2, _ := os.MkdirTemp("", "backfill-const-selftest-")
	gateFx := cafbPyJoin(fx2, "check-approve-backfill.py")
	oldSHA := strings.Repeat("0", 64)
	newSHA := strings.Repeat("1", 64)
	gateSrc := "#!/usr/bin/env python3\n# 注释行（非 ASCII · 锚定逐字节不动）\n" +
		fmt.Sprintf("%s = \"%s\"\nprint(\"tail\")\n", cafbConstName, oldSHA)
	_ = os.WriteFile(gateFx, []byte(gateSrc), 0o644)
	before, _ := os.ReadFile(gateFx)

	var buf strings.Builder
	buf.Reset()
	rcEmit := cafbEmitConstant(&buf, gateFx, &newSHA)
	emitOut := buf.String()
	afterEmit, _ := os.ReadFile(gateFx)
	emitSame := bytes.Equal(afterEmit, before)
	emitHasNew := strings.Contains(emitOut, newSHA)
	check(rcEmit == 1 && emitHasNew && strings.Contains(emitOut, oldSHA) && emitSame,
		fmt.Sprintf("⑦ 正控（`--emit-constant`）：**只读不写件**且印出**应有值**（现读 rc=%d · 应有值在=%s · 件未动=%s）",
			rcEmit, cafbPyBool(emitHasNew), cafbPyBool(emitSame)),
		fmt.Sprintf("⑦ 正控失败：emit 档没印出应有值或**动了件**（rc=%d · 应有值在=%s · 件未动=%s）",
			rcEmit, cafbPyBool(emitHasNew), cafbPyBool(emitSame)))

	dryBdir := cafbPyJoin(fx2, "bak-dry")
	buf.Reset()
	rcDry := cafbRefreshConstant(&buf, gateFx, &newSHA, false, dryBdir, "")
	dryOut := buf.String()
	afterDry, _ := os.ReadFile(gateFx)
	drySame := bytes.Equal(afterDry, before)
	dryBuilt := cafbExists(dryBdir)
	check(rcDry == 0 && drySame && !dryBuilt && strings.Contains(dryOut, "干跑"),
		fmt.Sprintf("⑧ 负控（干跑默认不写）：不带 `--yes` ⇒ **零写入**（rc=%d · 件未动=%s · 备份目录未建=%s）",
			rcDry, cafbPyBool(drySame), cafbPyBool(!dryBuilt)),
		fmt.Sprintf("⑧ 负控失败：干跑档**写了件**或建了备份（rc=%d · 件未动=%s）",
			rcDry, cafbPyBool(drySame)))

	bdir := cafbPyJoin(fx2, "bak")
	repoFx := cafbPyJoin(fx2, "repo")
	buf.Reset()
	rcWr := cafbRefreshConstant(&buf, gateFx, &newSHA, true, bdir, repoFx)
	after, _ := os.ReadFile(gateFx)
	bakEnts, _ := os.ReadDir(bdir)
	nBaks := len(bakEnts)
	bakPath := ""
	if nBaks > 0 {
		bakPath = cafbPyJoin(bdir, bakEnts[0].Name())
	}
	bakContent, _ := os.ReadFile(bakPath)
	idx := strings.Index(gateSrc, oldSHA)
	expect := []byte(gateSrc[:idx] + newSHA + gateSrc[idx+len(oldSHA):])
	byteEq := bytes.Equal(after, expect)
	lenEq := len(after) == len(before)
	bakEq := nBaks == 1 && bytes.Equal(bakContent, before)
	check(rcWr == 0 && byteEq && lenEq && nBaks == 1 && bakEq,
		fmt.Sprintf("⑨ 正控（`--refresh-constant --yes`）：最小替换（等长 %d→%d B · 常量外逐字节不动 · 字节级==期望）+ 仓外备份 1 份=改前态",
			len(before), len(after)),
		fmt.Sprintf("⑨ 正控失败：替换不是最小 / 未落备份（rc=%d · 字节级相等=%s · 等长=%s · 备份数=%d）",
			rcWr, cafbPyBool(byteEq), cafbPyBool(lenEq), nBaks))

	buf.Reset()
	inRepo := cafbPyJoin(repoFx, "bak-in-repo")
	rcIn := cafbRefreshConstant(&buf, gateFx, &oldSHA, true, inRepo, repoFx)
	inOut := buf.String()
	check(rcIn == 2 && strings.Contains(inOut, "仓内"),
		fmt.Sprintf("⑩ 负控（备份落仓内 ⇒ 拒写 rc=2 并逐字点名）：现读 rc=%d", rcIn),
		fmt.Sprintf("⑩ 负控失败：备份落仓内还写了（rc=%d）", rcIn))

	if fx2 != "" {
		_ = os.RemoveAll(fx2)
	}

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）\n", ok, bad)
	return bad == 0
}

// cafbRunFixture —— py `_run_fixture`：捕获 `scan()` 的 stdout 取 (输出, rc)。
// ★ 夹具仓里同放一份 `cafbGateRel`（合成常量件）供 `scan` 现读 B5 的登记值 —— py 侧
// 该常量是模块级常量、无需文件，故夹具形态不同，但两态**输出逐字同**（见 self_test 注释）。
func cafbRunFixture(repo, state string) [2]interface{} {
	var buf strings.Builder
	rc := cafbScan(&buf, repo, state, cafbPyJoin(repo, cafbGateRel))
	return [2]interface{}{buf.String(), rc}
}

// firstErr —— 返回首个非 nil 错（自检夹具建不起来的那一路）。
func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// ── 主 ───────────────────────────────────────────────────────────────────────────

// cafbScan —— py `scan`。
// ★ `gatePath` = B5 登记常量的真源件（与 `--refresh-constant` 同处，见 `cafbGateRel`）。
// py 侧该常量是**模块级常量**（其值就在 py 件里）⇒ 本入口从同一件现读，两入口结论一致
// （缺口 `GAP-20261004-09`）。
func cafbScan(stdout io.Writer, repo, state, gatePath string) int {
	r, why := cafbReadAll(repo, state)
	if r == nil {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2）\n", why)
		return 2
	}
	// ★ B5 的登记值走**单一真源**（同一件现读）。
	// ★ 退役态（G10 · 2026-10-07）：真源件（原 py）**已退役（不在盘）** ⇒ 回落到**内化进本件**的常量
	//   `cafbDevEditSHA256`（✗ 不 fail-closed）；件仍在盘但常量行解不出（坏件）仍 fail-closed（rc=2）。
	regConst, ok := cafbReadRegisteredConst(gatePath)
	if !ok && !cafbExists(gatePath) {
		regConst, ok = cafbDevEditSHA256, true
	}
	if !ok {
		fmt.Fprintf(stdout, "⛔ 未取数：登记常量件（%s）读不到 / 常量行解不出 ⇒ **不出结论**（rc=2）\n", gatePath)
		return 2
	}
	problems, counts := cafbJudgeAll(r, regConst)
	n, ev, bf := counts[0], counts[1], counts[2]
	fmt.Fprintln(stdout, "── 读数（每处带口径）")
	fmt.Fprintf(stdout, "   ① 件字段面（%s · `approvalTicketFile`）= %s\n", cafbTicketGoRel, strings.Join(r.ticketTags, ","))
	fmt.Fprintf(stdout, "   ② 审计行字段名（同件 · `approvalAuditLine`）= %s\n", strings.Join(r.auditTags, ","))
	fmt.Fprintf(stdout, "   ②-补 本事件**自己的**字段名（%s · `approveAuditOwnFields`）= %s\n", cafbTicketGoRel, strings.Join(r.ownFields, ","))
	fmt.Fprintf(stdout, "   ③ 审计件（%s）现跑 %d 行 · `%s` 事件 %d 条（其中**补签** %d 条）\n", r.auditPath, n, cafbEvent, ev, bf)
	fmt.Fprintf(stdout, "   ④ 在册件（%s）sha256 = %s\n", r.ticket, r.devSHA)
	if ev == 0 {
		fmt.Fprintln(stdout, "   ·  口径：本版**尚无补签件**（补签那一路本版不实现）—— 明标只在**审计侧**记「不是当场签的」")
	}
	if len(problems) > 0 {
		fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：%d 条\n", len(problems))
		for _, p := range problems {
			fmt.Fprintf(stdout, "   ✗ %s\n", p)
		}
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：件字段面八格不动 · 明标在审计侧 · 判定口有区分度 · 在册件零改")
	return 0
}

// cafbRefuseJSONWithoutMachineFace —— py `refuse_json_without_machine_face`（缺口 `Q-176` 统一口）。
func cafbRefuseJSONWithoutMachineFace(argv []string, stderr io.Writer) *int {
	hit := make([]string, 0)
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, "`"+x+"`")
		}
	}
	if len(hit) == 0 {
		return nil
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hit, " · "))
	// py `JSON_FIELDS` 为空 ⇒ 走「无（本门只出人读面）」那一支。
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// checkApproveBackfillGateMain —— 入口（= py `main()`）。
func checkApproveBackfillGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := cafbRefuseJSONWithoutMachineFace(argv, stderr); rc != nil {
		return *rc
	}
	root := cafbDefaultRoot()
	stateFlag := ""
	gateFlag := ""
	backupDir := ""
	selfTest := false
	noSelfTest := false
	emitC := false
	refreshC := false
	yes := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--root` 缺值")
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--state":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--state` 缺值")
				return 2
			}
			i++
			stateFlag = argv[i]
		case strings.HasPrefix(a, "--state="):
			stateFlag = a[len("--state="):]
		case a == "--gate":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--gate` 缺值")
				return 2
			}
			i++
			gateFlag = argv[i]
		case strings.HasPrefix(a, "--gate="):
			gateFlag = a[len("--gate="):]
		case a == "--backup-dir":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--backup-dir` 缺值")
				return 2
			}
			i++
			backupDir = argv[i]
		case strings.HasPrefix(a, "--backup-dir="):
			backupDir = a[len("--backup-dir="):]
		case a == "--self-test":
			selfTest = true
		case a == "--no-self-test":
			noSelfTest = true
		case a == "--emit-constant":
			emitC = true
		case a == "--refresh-constant":
			refreshC = true
		case a == "--yes":
			yes = true
		case a == "-h" || a == "--help":
			// argparse 面不逐字对齐（见未覆盖面①）：打一行简短 usage。
			fmt.Fprintln(stdout, cafbVersion)
			fmt.Fprintln(stdout, "usage: check-approve-backfill.py [--root R] [--state S] [--self-test] [--no-self-test] [--emit-constant] [--refresh-constant] [--yes] [--backup-dir D]")
			return 0
		default:
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 %s\n", a)
			return 2
		}
	}
	repo := cafbAbs(root)
	state := cafbStateDirOf(stateFlag)
	gatePath := ""
	if gateFlag != "" {
		gatePath = cafbAbs(gateFlag)
	} else if env := os.Getenv("ZERG_BACKFILL_GATE"); env != "" {
		gatePath = cafbAbs(env)
	} else {
		// 缺省：root 推出的同族路径（root==仓根时与 py `abspath(__file__)` 同值）—— 声明行 (a)。
		gatePath = cafbPyJoin(repo, "scripts", "gates", "check-approve-backfill.py")
	}
	ticket := cafbPyJoin(state, cafbTicketRel)
	var devSHA *string
	if cafbExists(ticket) {
		if h, err := cafbSha256File(ticket); err == nil {
			devSHA = &h
		}
	}

	if emitC && refreshC {
		fmt.Fprintln(stderr, "⛔ 用法错（rc=2）：`--emit-constant` 与 `--refresh-constant` 互斥（一个只读、一个可能写）")
		return 2
	}
	if yes && !refreshC {
		fmt.Fprintln(stderr, "⛔ 用法错（rc=2）：`--yes` 只跟 `--refresh-constant` 连用（本门**默认干跑不写**）")
		return 2
	}
	if emitC {
		fmt.Fprintln(stdout, cafbVersion)
		return cafbEmitConstant(stdout, gatePath, devSHA)
	}
	if refreshC {
		fmt.Fprintln(stdout, cafbVersion)
		if !noSelfTest && !cafbSelfTest(stdout, repo, state) {
			fmt.Fprintln(stdout, "⛔ 自检不过 ⇒ **拒写**（rc=1）：先修自检再谈刷值")
			return 1
		}
		if devSHA == nil {
			fmt.Fprintf(stdout, "⛔ 未取数：在册批准件不在（%s）⇒ **拒写**（rc=2）\n", ticket)
			return 2
		}
		return cafbRefreshConstant(stdout, gatePath, devSHA, yes, backupDir, repo)
	}
	fmt.Fprintln(stdout, cafbVersion)
	if !noSelfTest && !cafbSelfTest(stdout, repo, state) {
		return 1
	}
	if selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：%s（状态目录 %s）\n", repo, state)
	return cafbScan(stdout, repo, state, gatePath)
}
