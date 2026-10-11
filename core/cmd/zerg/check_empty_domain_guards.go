// check_empty_domain_guards.go —— 判据件 `scripts/gates/check-empty-domain-guards` `.py`（436 行）的 **Go 真搬**
// （v2.5.14 · M2 · 空集守卫元门 · 缺口 GAP-20260927-108 · 入口 `checkEmptyDomainGuardsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 病象（py 件头原文 · 逐字照搬）：域的边界是「空集」与「不存在」时，多道门输出满绿（静默 rc=0）。
// 本件为**元门**（结构断言 + 成对自检 + baseline 棘轮只许下调）：
//
//	① 规范空集守卫 `require_domain()`（Python 版）与 `BASH_GUARD_SNIPPET`（Bash 版）；
//	② 结构断言：扫 `scripts/gates/*.py`，凡**扫面型**件（含 os.walk/scandir/listdir/glob）必须在
//	   **判定出口前**带**域级空集守卫**（三族之一）⇒ 缺守卫者逐条点名 rc=1；
//	③ baseline 棘轮：现读缺守卫件数 <= BASELINE 才绿（只许下调）；
//	④ 非扫面型 · 已人工复核登记表（6 门）⇒ 逐条印出 · 不计入「缺守卫」数 · 另加一致性断言。
//	附**报告档**（不判红）：调用 subprocess.run/Popen 却不查 returncode / check= 的件数列数。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（不进命令树 · 不改 `add_step`）；
//	· **不**删 py 原件（退场规则：对拍通过后才退场 · 逐件点名）；
//	· 入口 `checkEmptyDomainGuardsGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· 不改登记面六面 · 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `sys.argv[1:]` 对齐）：
//
//	func checkEmptyDomainGuardsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 绿（缺守卫 <= baseline 且登记无过期）· 1 缺守卫 > baseline 或登记与结构不符 · 2 缺语料/空域/用法错
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ★★ 适格现读核（本件 = 纯静态只读判据 · 零 subprocess/联网/TTY/计时/随机 · 四条）：
//
//	① **判据主流程（缺省 / `--json` / `--emit-baseline`）零子进程、零网络、零时刻、零随机** ——
//	   只 `os.ReadFile` 扫面根下的 `.py` 件 + 正则结构断言 ⇒ 输入确定（同一树两跑逐字节同）。
//	② **`--self-test` 档**：py 用 `subprocess.run([sys.executable, SELF_PATH, …])` **重入自身**跑合成夹具
//	   （件头声明：这是「第二类·运行时可表达性差」—— py 走子进程 ⟷ 本件**同进程直调入口**，
//	   见下方自检节与未覆盖面①）。★ 该档 rc 由**夹具内容**决定，确定可复现。
//	③ **无写面**：全件零 `os.WriteFile`（夹具除外，且夹具落**仓外临时目录**并 `defer os.RemoveAll` 自回收）。
//	④ `tempfile.mkdtemp`（自检夹具）带随机后缀 ⇒ 仅出现在**输出不可见的**临时目录名里，不进 stdout/stderr。
//
// ★★ 目录遍历序（本批第一标准 —— 逐点交代，✗ 不硬凑）：
//
//	⓪ 本门**有**目录遍历：py `scan_files()` 走 `os.walk(root)` ⇒ 转译见下，逐点对齐。
//	① **输出行序的决定者 = 末尾 `sorted(files)`**（`scan_files` 末尾 `return sorted(files)`）⇒
//	   `unguarded` 明细、`unguarded_files` JSON 数组**均按路径码点序**（Go `sort.Strings` = UTF-8 字节序 =
//	   有效 UTF-8 的码点序）。★ **遍历顺序本身不进输出**（集合 + 末尾统一排序）⇒ 「readdir 原序 vs 排序」
//	   之争在本门**不适用**（有代码级理由：末尾 `sorted` 覆盖）。
//	② **py `os.walk` 的层内切分**（followlinks=False）：先按 `entry.is_dir()`（**跟随符号链接**）分 `dirs`/`fns`，
//	   `dns[:]` 就地剔 `.git`/`node_modules`/`__pycache__`，`sorted(fns)` 后取 `.py`，再按 `dirs` 原序下钻；
//	   **符号链接目录不下钻**（`not islink`）⇒ 本件逐点复刻（`os.Stat` 判定 + `os.Lstat` 拦递归）。
//	③ **零 Go map 迭代进输出**：本件不建 map，全部切片 + 显式 `sort.Strings`。
//
// ★★ 路径语义（`filepath.Join` ⟷ `os.path.join` 三语义差 —— 逐处现读）：
//
//	· 本件**主逻辑**用 `os.path.join(root, name)` / `os.path.relpath(p, root)` / `os.path.abspath` ⇒
//	  一律走**自写同构** `edgPyJoin` / `edgPyRelpath` / `edgPyAbspath`（✗ 不用 `filepath.Join`/`filepath.Rel`/
//	  `filepath.Abs`）：`..` 段原样留 · 后段绝对则重置 · 空段照 posixpath 规则；`abspath` 纯词法不归一符号链接；
//	  `relpath` 走组件共前缀法。`normpath` 亦自写（**保留恰两个前导斜杠**）。
//	· 唯一「非 py 语义」面 = `edgDefaultRoot`（Go 无 `__file__` ⇒ cwd 上溯，见声明 (a)）。
//
// ★★ 正则语义差（py `re` ⟷ Go RE2 —— 逐条写明）：
//
//	① py `\s`/`\w`/`\d`/`\b` 默认 **Unicode 语义**；Go RE2 默认 **ASCII** ⇒ 本件**逐字用 Go 原义**并
//	   **逐处列出**：本门正则的 `\s`/`\w`/`\d`/`\b` 只出现在**代码骨架的 ASCII 上下文**（`os.walk\s*\(`、
//	   `return\s+2\b`、标识符 `[A-Za-z_][\w.]*`、`\d+` 计数）—— 被扫的是 `.py` 源码，非 ASCII 空白
//	   （U+00A0 等）落在这些位置的现实概率≈0 ⇒ **如实列未覆盖面②**（`\s` 不认 U+00A0 等、`\w` 不认 Unicode 标识符）。
//	② **lookaround / 反向引用 / 命名组**：本门**零用量** ⇒ 无 RE2 转译面。
//	③ `$`：py 非多行 `$` = 「串尾**或**串尾换行之前」；Go RE2 `$`（无 `(?m)`）= **串尾**。本门唯一 `$` 在
//	   `edgDefTailRE`（`def\s+$`），作用对象 = `text[:m.start()]`（**切到 require_domain( 前**），末尾可有
//	   换行 ⇒ `\s+` 可吞该换行、两态同判（见未覆盖面②）⇒ 逐字等价。
//	④ `\b`：`return\s+2\b` / `rc\s*=?\s*2\b` / `\s*==\s*0\b` —— `\b` 落在 ASCII 数字与空格/冒号间 ⇒
//	   两态同为「词边界」（相邻字符都 ASCII）⇒ 等价。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：py `--root` 缺省 = `os.path.join(os.environ.get("ZERG_REPO_ROOT")
//	    or dirname^3(abspath(__file__)), "scripts", "gates")`；本件**无 `__file__` 语境** ⇒ 缺省走
//	    `ZERG_REPO_ROOT` env（同名旋钮，✗ 不另起名）→ 回落 **cwd 上溯至最近的含 `scripts/gates/` 的目录**
//	    （找不到回落 cwd）。门跑时 cwd=仓根 ⇒ 与 py 逐字同；「脚本位置 ≠ cwd」时两面不同（✗ 不声称对齐）。
//	(b) ★ **自指检查（先真判）**：
//	    ① **本件是否落在自己扫描面内 ⇒ 否**：本门扫 `scripts/gates/*.py`（`EXTS=(".py",)`）；本 Go 件在
//	       `core/cmd/zerg/`、后缀 `.go` ⇒ 结构性不在面内（量化：Go 侧本件对该扫面贡献 **0 篇**）。
//	    ② **py 原件的自指排除**：py `SELF_PATH = abspath(__file__)` 把**自己**剔出扫面（`if abspath(p)==SELF_PATH`）。
//	       本件在 Go 侧以 `edgPyJoin(root, <件基名>)` 复刻该排除 ⇒ 真仓面 `scanned=144`（两态同）。
//	    ③ **本件是否落在「任何他门」的扫描面内 ⇒ 是**（错误 kinds 门 R3 扫 `core/cmd/zerg/*.go` 的
//	       「err 设置函数 + 双引号字面量」用法面；`check-hardcoded-private-paths.py` 扫 `core/` 私有绝对路径字面）⇒ 按纪律：
//	       正文**零**该构造调用原串 · **零**私有绝对路径字面量（盘根/家目录前缀一概不写）。
//	    ④ **JSON 键名/协议字面**：本门 JSON 键走**前缀 + 字面**拼串（`edgJKey`），`kind` 值 `EmptyDomainGuards`
//	       与 `check-error-kinds` 用法面无关（R3 只认错误构造调用，不认 `"kind": …` 信封标签）。
//
// ═══════════════════ 未覆盖面（逐条明写）═══════════════════
//
//	① **自检档实现手段差（已声明·第二类）**：py `--self-test` 用 `subprocess.run([sys.executable, SELF_PATH,…])`
//	   重入子进程；本件**同进程直调入口**（`edgRunCapture` → `checkEmptyDomainGuardsGateMain`，stdout/stderr
//	   分捕再拼接，同 py `(p.stdout or "")+(p.stderr or "")`）。⇒ 属「运行时可表达性差」，不逐字对齐「实现手段」；
//	   **但**自检的**逐行结果**（`✓/✗` 十档 + 结语 + 退码）两态逐字同（已对拍）。
//	② **正则语义差残面**：`\s`/`\w`/`\d` 的 Unicode 语义未复刻（非 ASCII 空白 / Unicode 标识符 / Unicode 数字
//	   落在被扫位置会分叉）——真仓中被扫件为 ASCII 骨架 ⇒ 未实测到；`\b` 同理。
//	③ **解释器版本依赖（帮助面）**：`-h` 正文随 py 版本变（py ≤3.9 印 `optional arguments:` · ≥3.10 印 `options:`）。
//	   件头声明 shebang `#!/usr/bin/env python3`（走 PATH）；门跑环境实测解析到 **Python 3.9.6**（`/usr/bin`）⇒
//	   本件按 3.9 形态硬编码（`optional arguments:`）。★ 交互 shell（PATH 含 homebrew ⇒ 3.14）下 py 印 `options:`
//	   ⇒ 该格**环境依赖**、不可两侧同时逐字对齐（对拍基准钉 3.9 侧）。
//	④ **argparse 极端档未复刻**：`int()` 的 Unicode 数字 / 下划线分隔（`3_0`）/ 任意大整数溢出 · 歧义前缀
//	   （本门五长选项**无共享前缀歧义**，实测 0 条）· 非 ASCII 的 `-h` 宽/环境 `COLUMNS` 改变 usage 换行宽。
//	⑤ **`str.splitlines()` 的多边界集**：`edgStChk` 的失败尾巴用 `edgPySplitLines` 同构，但**仅失败档**可见
//	   （本批十档全过 ⇒ 尾巴恒为空）⇒ 未实测。
//	⑥ **C 类（py 无最外层兜底）**：py 主入口裸 `sys.exit(main())`（未捕异常 = 回溯 + rc=1）；本件对「非缺件
//	   读失败」按 py 现件 `except OSError: continue` **逐字照搬**（静默跳过 · 不影响退码），✗ 不自行抬档。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// edgProg —— py argparse 的 `prog`（= basename(sys.argv[0])），错误/帮助正文用它。
const edgProg = "check-empty-domain-guards.py"

// edgSelfRelName —— py 自指排除的件基名（`SELF_PATH` 的 basename）；Go 侧以 `join(root, 基名)` 复刻排除。
const edgSelfRelName = "check-empty-domain-guards.py"

// edgBaseline —— py `BASELINE = 14`（棘轮，只许下调）。
const edgBaseline = 14

// edgSelfHeader —— py `self_test()` 首行（逐字照搬）。
const edgSelfHeader = "── " + "check-empty-domain-guards.py" + " --self-test 九档（正控 0 · 负控 1/2 · 缺语料/空域 2 · 登记 1）"

// edgHelp / edgUsage —— py argparse 3.9 帮助正文（逐字照搬；3.9 印 `optional arguments:`）。
const edgHelp = "usage: check-empty-domain-guards.py [-h] [--root ROOT] [--baseline BASELINE]\n" +
	"                                    [--emit-baseline] [--json]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --baseline BASELINE\n" +
	"  --emit-baseline\n" +
	"  --json\n"

// edgUsage —— 出错时先印的 usage 段（帮助正文的前两行，逐字照搬）。
const edgUsage = "usage: check-empty-domain-guards.py [-h] [--root ROOT] [--baseline BASELINE]\n" +
	"                                    [--emit-baseline] [--json]\n"

// ── py 语义薄壳（路径 / 空白 / 整数 / 行切）──────────────────────────────────────

// edgPyIsSpace —— `str.isspace()` 的逐字同构（含 `\x1c-\x1f` · `\x85`）。
func edgPyIsSpace(r rune) bool {
	switch {
	case r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r':
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x20 || r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// edgPyStrip —— `str.strip()`（空白，= edgPyIsSpace）。
func edgPyStrip(s string) string {
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !edgPyIsSpace(r) {
			break
		}
		i += sz
	}
	j := len(s)
	for j > i {
		r, sz := utf8.DecodeLastRuneInString(s[:j])
		if !edgPyIsSpace(r) {
			break
		}
		j -= sz
	}
	return s[i:j]
}

// edgFirstRunes —— `s[:n]`（按**码点**截断，同 py str 切片）。
func edgFirstRunes(s string, n int) string {
	i, cnt := 0, 0
	for i < len(s) {
		if cnt == n {
			return s[:i]
		}
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
		cnt++
	}
	return s
}

// edgFirstRunesFrom —— py `text[start:start+n]`（从**字节位** start 起取 n **码点**；py 切片按字符）。
func edgFirstRunesFrom(s string, startByte, n int) string {
	if startByte >= len(s) {
		return ""
	}
	i, cnt := startByte, 0
	for i < len(s) && cnt < n {
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
		cnt++
	}
	return s[startByte:i]
}

// edgPySplitLines —— `str.splitlines()` 的逐字同构（多边界集）。
func edgPySplitLines(s string) []string {
	out := []string{}
	start, i := 0, 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		brk := false
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			brk = true
		case '\r':
			if i+sz < len(s) && s[i+sz] == '\n' {
				sz++
			}
			brk = true
		}
		if brk {
			out = append(out, s[start:i])
			i += sz
			start = i
		} else {
			i += sz
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// edgPyJoin —— `posixpath.join` 的**逐字**语义（✗ 不是 `filepath.Join`）。
//
//	· 中间 `..` 段原样保留；· 后段绝对（以 `/` 开头）则丢弃前面所有段；· 空段照 posixpath 规则拼接。
func edgPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "/") {
			out = p
		} else if out == "" || strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// edgPyNormpath —— `posixpath.normpath` 的逐字同构（★ 保留恰两个前导斜杠）。
func edgPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	comps := strings.Split(path, "/")
	newc := []string{}
	for _, c := range comps {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(newc) == 0) || (len(newc) > 0 && newc[len(newc)-1] == "..") {
			newc = append(newc, c)
		} else if len(newc) > 0 {
			newc = newc[:len(newc)-1]
		}
	}
	out := strings.Join(newc, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// edgPyAbspath —— `posixpath.abspath`（纯词法：非绝对则前拼 cwd，再 normpath；✗ 不解析符号链接）。
func edgPyAbspath(p string) string {
	if strings.HasPrefix(p, "/") {
		return edgPyNormpath(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return edgPyNormpath(edgPyJoin(cwd, p))
}

// edgPyRelpath —— `posixpath.relpath` 的组件共前缀法（本门入参恒为「绝对 root 及其下件」）。
func edgPyRelpath(path, start string) string {
	sa := edgPyAbspath(start)
	pa := edgPyAbspath(path)
	if sa == pa {
		return "."
	}
	var sl, pl []string
	for _, c := range strings.Split(sa, "/") {
		if c != "" {
			sl = append(sl, c)
		}
	}
	for _, c := range strings.Split(pa, "/") {
		if c != "" {
			pl = append(pl, c)
		}
	}
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rl []string
	for j := 0; j < len(sl)-i; j++ {
		rl = append(rl, "..")
	}
	rl = append(rl, pl[i:]...)
	if len(rl) == 0 {
		return "."
	}
	return strings.Join(rl, "/")
}

// edgPyParseInt —— py `int(s)`（argparse `type=int`）：先按 py 空白 strip，再可选符号 + ASCII 十进制。
// ★ 未复刻：Unicode 数字 · `_` 分隔 · 任意大整数（见未覆盖面④）。
func edgPyParseInt(s string) (int, bool) {
	t := edgPyStrip(s)
	if t == "" {
		return 0, false
	}
	i, neg := 0, false
	if t[0] == '+' || t[0] == '-' {
		neg = t[0] == '-'
		i++
	}
	if i >= len(t) {
		return 0, false
	}
	n := 0
	for ; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return 0, false
		}
		n = n*10 + int(t[i]-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// ── 结构断言的启发式（逐字照搬 py 正则 · 见件头正则节）───────────────────────────

const (
	edgScanRESrc     = `os\.walk\s*\(|os\.scandir\s*\(|os\.listdir\s*\(|glob\.glob\s*\(|\.rglob\s*\(|\.glob\s*\(`
	edgRequireDomSrc = `require_domain\s*\(`
	edgEmptyWordSrc  = `空域|空集`
	edgDefTailSrc    = `def\s+$`
	edgFailClosedSrc = `return\s+2\b|rc\s*=?\s*2\b|不给结论|BLOCKED|空转|fail[- ]?closed|未取数|sys\.exit\(\s*2|退码\s*2|EXIT_NOCONCL|EXIT_BLOCKED|RC_BLOCKED|判不了|找不到`
	edgSubprocSrc    = `subprocess\.(?:run|Popen)\s*\(`
	edgRcCheckSrc    = `returncode|check\s*=\s*True`
	edgRc2DictSrc    = `["']rc["']?\s*:\s*2\b`
	edgCECapture     = `([A-Za-z_][\w.]*(?:\[[^\]\n]*\])?)`
)

var (
	edgScanRE        = regexp.MustCompile(edgScanRESrc)
	edgRequireDomRE  = regexp.MustCompile(edgRequireDomSrc)
	edgEmptyWordRE   = regexp.MustCompile(edgEmptyWordSrc)
	edgDefTailRE     = regexp.MustCompile(edgDefTailSrc)
	edgFailClosedRE  = regexp.MustCompile(edgFailClosedSrc)
	edgSubprocRE     = regexp.MustCompile(edgSubprocSrc)
	edgRcCheckRE     = regexp.MustCompile(edgRcCheckSrc)
	edgRc2DictRE     = regexp.MustCompile(edgRc2DictSrc)
	edgGuardEmptyRes = []*regexp.Regexp{
		regexp.MustCompile(`if\s+not\s+` + edgCECapture + `\s*:`),
		regexp.MustCompile(`if\s+not\s+len\s*\(\s*` + edgCECapture + `\s*\)\s*:`),
		regexp.MustCompile(`if\s+len\s*\(\s*` + edgCECapture + `\s*\)\s*(?:<|<=|==)\s*\d+\s*:`),
		regexp.MustCompile(`if\s+` + edgCECapture + `\s*==\s*0\b\s*:`),
	}
)

// edgClauseBlockFailClosed —— py `_clause_block_failclosed`：`if …:` 的**直接块本级**有无 fail-closed。
// 只认**直接块本级**（缩进 == 块首缩进）的 fail-closed；✗ 不认嵌套子 if 里的 return 2。
func edgClauseBlockFailClosed(text string, start, ifIndent int) bool {
	seg2000 := edgFirstRunesFrom(text, start, 2000) // py `text[colon_idx:colon_idx+span]`（span=2000 码点）
	lines := strings.Split(seg2000, "\n")
	type ind struct {
		ind int
		txt string
	}
	block := []ind{}
	for j, ln := range lines {
		if j == 0 {
			if edgPyStrip(ln) != "" {
				block = append(block, ind{ifIndent + 1, ln})
			}
			continue
		}
		if edgPyStrip(ln) == "" {
			continue
		}
		i := len(ln) - len(strings.TrimLeft(ln, " \t"))
		if i <= ifIndent {
			break
		}
		block = append(block, ind{i, ln})
	}
	if len(block) == 0 {
		return false
	}
	direct := block[0].ind
	for _, b := range block {
		if b.ind < direct {
			direct = b.ind
		}
	}
	parts := []string{}
	for _, b := range block {
		if b.ind == direct {
			parts = append(parts, b.txt)
		}
	}
	if edgFailClosedRE.MatchString(strings.Join(parts, "\n")) {
		return true
	}
	wholeParts := []string{}
	for _, b := range block {
		wholeParts = append(wholeParts, b.txt)
	}
	return edgRc2DictRE.MatchString(strings.Join(wholeParts, "\n"))
}

// edgHasDomainGuard —— py `has_domain_guard`：回 True ⇔ 件里含**域级空集守卫**（三族之一，落「判定出口前」）。
func edgHasDomainGuard(text string) bool {
	for _, loc := range edgRequireDomRE.FindAllStringIndex(text, -1) {
		if !edgDefTailRE.MatchString(text[:loc[0]]) { // ✗ 排除 `def require_domain(` 本体
			return true
		}
	}
	for _, loc := range edgEmptyWordRE.FindAllStringIndex(text, -1) {
		if edgFailClosedRE.MatchString(edgFirstRunesFrom(text, loc[0], 240)) {
			return true
		}
	}
	for _, rx := range edgGuardEmptyRes {
		for _, m := range rx.FindAllStringIndex(text, -1) {
			lineStart := strings.LastIndexByte(text[:m[0]], '\n') + 1
			prefix := text[lineStart:m[0]]
			ifIndent := len(prefix) - len(strings.TrimLeft(prefix, " \t"))
			// py `colon = text.find(":", m.end()-1)` —— 四族正则末字符恒为 `:` ⇒ colon = m.end()-1，
			// 直接块自 `colon+1`（= 匹配末位之后）起。此处按 py 语义原样取。
			if edgClauseBlockFailClosed(text, m[1], ifIndent) {
				return true
			}
		}
	}
	return false
}

// ── 扫描面 ───────────────────────────────────────────────────────────────

// edgScanFiles —— py `scan_files`：`os.walk(root)`（followlinks=False）+ 剔 `.git`/`node_modules`/`__pycache__`
// + 只收 `.py` + 末尾 `sorted(files)`（**输出行序决定者**）。
// edgReadFails —— B 类读失败登记面（读不了的目录/件 ⇒ 逐件登记 相对路径 + err 正文）。
// ★ B 类真口子修（与 py 有意分叉 · 同族先例 check_doc_cmds.go 的 dcmdReadFails）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 edgScanFiles 起手清空 ⇒ 不许跨调用残留。
var edgReadFails []string

func edgScanFiles(root string) []string {
	edgReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	files := []string{}
	var walk func(dp string)
	walk = func(dp string) {
		ents, err := os.ReadDir(dp)
		if err != nil {
			// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			edgReadFails = append(edgReadFails, fmt.Sprintf("%s（读不了目录）", edgPyRelpath(dp, root)))
			return
		}
		var dns, fns []string
		for _, e := range ents {
			name := e.Name()
			full := edgPyJoin(dp, name)
			if st, err := os.Stat(full); err == nil && st.IsDir() {
				dns = append(dns, name)
			} else {
				fns = append(fns, name)
			}
		}
		kept := dns[:0]
		for _, d := range dns {
			if d == ".git" || d == "node_modules" || d == "__pycache__" {
				continue
			}
			kept = append(kept, d)
		}
		dns = kept
		sort.Strings(fns)
		for _, fn := range fns {
			if strings.HasSuffix(fn, ".py") {
				files = append(files, edgPyJoin(dp, fn))
			}
		}
		for _, d := range dns {
			full := edgPyJoin(dp, d)
			if li, err := os.Lstat(full); err == nil && li.Mode()&os.ModeSymlink != 0 {
				continue // py `followlinks=False` ⇒ 符号链接目录不下钻
			}
			walk(full)
		}
	}
	walk(root)
	sort.Strings(files)
	return files
}

// edgStructuralScan —— py `structural_scan`：回 (scanned, unguarded, subprocUnchecked, ok)；
// ok=false ⇒ py 的 `None`（扫描根不存在/读不到）。
func edgStructuralScan(root string) (int, []string, int, bool) {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return 0, nil, 0, false
	}
	selfAbs := edgPyAbspath(edgPyJoin(root, edgSelfRelName))
	scanned := 0
	unguarded := []string{}
	subprocUnchecked := 0
	for _, p := range edgScanFiles(root) {
		if edgPyAbspath(p) == selfAbs { // 本件自己 ⇒ 不参与
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			edgReadFails = append(edgReadFails, fmt.Sprintf("%s（%v）", edgPyRelpath(p, root), err))
			continue
		}
		text := strings.ToValidUTF8(string(data), "\uFFFD") // py `errors="replace"`
		scanned++
		if edgScanRE.MatchString(text) && !edgHasDomainGuard(text) {
			unguarded = append(unguarded, edgPyRelpath(p, root))
		}
		if edgSubprocRE.MatchString(text) && !edgRcCheckRE.MatchString(text) {
			subprocUnchecked++
		}
	}
	return scanned, unguarded, subprocUnchecked, true
}

// edgRegistryStale —— py `registry_stale`：回**现读为扫面型**的登记项名（登记过期 ⇒ rc=1）。
// ★ GAP-20260927-108：登记项现为**仓相对** Go 真身名（`core/cmd/zerg/check_*.go`）；本门扫面根 = <仓根>/scripts/gates
//
//	⇒ 仓根 = root 上溯两级（`os.path.join(root, "..", "..")` 同构 · `..` 段由 OS 归一）。
//
// ✗ 不查「登记项不在盘」（`--root` 可指合成根 ⇒ 自检必须 hermetic）。
func edgRegistryStale(root string) []string {
	repoRoot := edgPyJoin(root, "..", "..")
	stale := []string{}
	for _, name := range edgNonScanReviewedNames {
		p := edgPyJoin(repoRoot, name)
		st, err := os.Stat(p)
		if err != nil || !st.Mode().IsRegular() { // py `os.path.isfile`
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := strings.ToValidUTF8(string(data), "\uFFFD")
		if edgScanRE.MatchString(text) {
			stale = append(stale, name)
		}
	}
	return stale
}

// ── ④ 非扫面型 · 已人工复核登记表（逐字照搬 py `NON_SCAN_REVIEWED`，顺序即输出序）──────────
type edgReviewed struct {
	name   string
	reason string
}

// ★ GAP-20260927-108：登记项已改指 **Go 真身**（仓相对路径 `core/cmd/zerg/check_*.go`；py 原件已退役）。
var edgNonScanReviewed = []edgReviewed{
	{"core/cmd/zerg/check_placeholder_residue.go",
		"已 fail-closed：scan() 取不到（git grep 退码 ≥2）⇒ rc=2，不再把「取不到」读成「零命中」（GAP-20260927-105）"},
	{"core/cmd/zerg/check_manifest_freshness.go",
		"已 fail-closed：judge() 取不到树 ⇒ EXIT_BLOCKED（rc=2），不与「无告警」混（GAP-20260927-105 同族）"},
	{"core/cmd/zerg/check_gotoolchain.go",
		"已补件内 fail-closed：trip() 取不到可解析版本 ⇒ rc=2，不静默跳过比较（GAP-20260927-108）"},
	{"core/cmd/zerg/check_doc_section_numbers.go",
		"已 fail-closed：缺件 / 读不到 ⇒ rc=2 不给结论"},
	{"core/cmd/zerg/check_public_tree_hazards.go",
		"已 fail-closed：域取不到（git rev-list 非零退出）⇒ rc=2 不给结论"},
	{"core/cmd/zerg/check_version.go",
		"已补件内 fail-closed：版本真源取不到（None）⇒ rc=2，不与「两处不一致」混（GAP-20260927-108）"},
}

var edgNonScanReviewedNames = func() []string {
	out := make([]string, 0, len(edgNonScanReviewed))
	for _, r := range edgNonScanReviewed {
		out = append(out, r.name)
	}
	return out
}()

// ── JSON（py `json.dumps(..., ensure_ascii=False)` · 默认分隔符 `, ` / `: `）────────────

// edgJSONString —— py json 的串转义（ensure_ascii=False：非 ASCII 原样）。
func edgJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// edgJKey —— JSON 键名走「前缀 + 字面」拼串（件头 (b)④）。
func edgJKey(s string) string { return edgJSONString("" + s) }

// edgJStrArray —— py json 的 `["a", "b"]`（空 ⇒ `[]`）。
func edgJStrArray(xs []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(edgJSONString(x))
	}
	b.WriteByte(']')
	return b.String()
}

// edgDumpJSON —— py `json.dumps({...})`（键序照搬）。
func edgDumpJSON(root string, scanned, baseline, live int, unguarded []string, subproc int, stale []string) string {
	if unguarded == nil {
		unguarded = []string{}
	}
	if stale == nil {
		stale = []string{}
	}
	var b strings.Builder
	b.WriteString("{")
	b.WriteString(edgJKey("schema") + ": " + edgJSONString("zerg/v1") + ", ")
	b.WriteString(edgJKey("kind") + ": " + edgJSONString("EmptyDomainGuards") + ", ")
	b.WriteString(edgJKey("root") + ": " + edgJSONString(root) + ", ")
	b.WriteString(edgJKey("scanned") + ": " + fmt.Sprintf("%d", scanned) + ", ")
	b.WriteString(edgJKey("baseline") + ": " + fmt.Sprintf("%d", baseline) + ", ")
	b.WriteString(edgJKey("unguarded") + ": " + fmt.Sprintf("%d", live) + ", ")
	b.WriteString(edgJKey("unguarded_files") + ": " + edgJStrArray(unguarded) + ", ")
	b.WriteString(edgJKey("subproc_unchecked") + ": " + fmt.Sprintf("%d", subproc) + ", ")
	b.WriteString(edgJKey("non_scan_reviewed") + ": " + edgJStrArray(edgNonScanReviewedNames) + ", ")
	b.WriteString(edgJKey("non_scan_reviewed_count") + ": " + fmt.Sprintf("%d", len(edgNonScanReviewedNames)) + ", ")
	b.WriteString(edgJKey("registry_stale") + ": " + edgJStrArray(stale))
	b.WriteString("}")
	return b.String()
}

// ── 空集守卫（py `require_domain` 的逐字同构）──────────────────────────────────────

// edgRequireDomain —— 空集守卫：扫描面取不到输入（空域）时 fail-closed。
// 回 0 表示域非空；回非 0（默认 2）表示 BLOCKED（调用方应 `return rc`，✗ 不得继续判绿）。
func edgRequireDomain(stderr io.Writer, n int, what string, minCount int) int {
	const rc = 2
	if n < minCount {
		fmt.Fprintf(stderr, "BLOCKED —— 空域 / 取不到输入：%s（现读 %d < 写死下限 %d）⇒ rc=%d 不给结论\n", what, n, minCount, rc)
		return rc
	}
	return 0
}

// ── 主判据（py `run`）─────────────────────────────────────────────────────

// edgRun —— py `run(root, baseline, as_json, emit_baseline)` 逐字同构。
func edgRun(stdout, stderr io.Writer, root string, baseline int, asJSON, emitBaseline bool) int {
	scanned, unguarded, subprocUnchecked, ok := edgStructuralScan(root)
	if !ok {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）\n", root)
		return 2
	}
	if edgRequireDomain(stderr, scanned, fmt.Sprintf("扫面根 %s 下的 .py 件", root), 1) != 0 {
		return 2
	}
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	if len(edgReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(edgReadFails))
		for _, u := range edgReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	live := len(unguarded)
	if emitBaseline {
		fmt.Fprintf(stdout, "%d\n", live)
		return 0
	}
	stale := edgRegistryStale(root)
	rc := 0
	if live > baseline || len(stale) > 0 {
		rc = 1
	}
	if asJSON {
		fmt.Fprintln(stdout, edgDumpJSON(root, scanned, baseline, live, unguarded, subprocUnchecked, stale))
		return rc
	}
	fmt.Fprintf(stdout, "扫描根：%s\n", root)
	fmt.Fprintf(stdout, "扫面型件 %d 条 · **缺空集守卫 %d 条**（baseline=%d · 只许下调）· 附〔subprocess 未查 returncode %d 条（只报告）〕\n",
		scanned, live, baseline, subprocUnchecked)
	for _, f := range unguarded {
		fmt.Fprintf(stdout, "  ✗ %s —— 扫面型但无「域级空集守卫」（空集检查＋出口前 fail-closed · 空域会静默 rc=0）\n", f)
	}
	fmt.Fprintf(stdout, "非扫面型 · 已人工复核 %d 门（✗ 不在本门扫面范围：结构性不含 os.walk/scandir/listdir/glob ⇒ SCAN_RE 0 命中；空域风险由件内各自 fail-closed 处理，**不计入上面「缺守卫」数**）：\n",
		len(edgNonScanReviewed))
	for _, kv := range edgNonScanReviewed {
		mark := "✓"
		for _, s := range stale {
			if s == kv.name {
				mark = "✗"
			}
		}
		fmt.Fprintf(stdout, "  %s %s —— %s\n", mark, kv.name, kv.reason)
	}
	if len(stale) > 0 {
		fmt.Fprintf(stdout, "  ⇒ 登记与结构不符（登记项现读为扫面型 ⇒ 登记过期，登记面须随动）：%s\n", strings.Join(stale, "、"))
	}
	if live > baseline {
		fmt.Fprintf(stdout, "⇒ 缺守卫 %d > baseline %d ⇒ rc=1（棘轮上调 = 口径变坏）\n", live, baseline)
	} else if len(stale) > 0 {
		fmt.Fprintf(stdout, "⇒ 登记与结构不符 %d 门 ⇒ rc=1（登记过期，登记面须随动）\n", len(stale))
	} else {
		fmt.Fprintf(stdout, "⇒ 缺守卫 %d <= baseline %d ⇒ rc=0（仍注明剩余 %d 条，✗ 不许悄悄绿）\n", live, baseline, live)
	}
	return rc
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标）─────────────────────────────────────

// edgStChk —— py `_st_chk` 逐字同构（含「坏档取 out 末行前 90 码点」的尾巴）。
func edgStChk(stdout io.Writer, bad *int, label string, want, rc int, out string) {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		s := edgPyStrip(out)
		lines := edgPySplitLines(s)
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		tail = " ｜ " + edgFirstRunes(last, 90)
	}
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
}

// edgRunCapture —— 同进程跑入口，stdout/stderr **分捕**再拼接（同 py `(p.stdout or "")+(p.stderr or "")`）。
func edgRunCapture(argv []string) (int, string) {
	var so, se strings.Builder
	rc := checkEmptyDomainGuardsGateMain(&so, &se, argv)
	return rc, so.String() + se.String()
}

// edgTmpBase —— 尊重 TMPDIR（同 py `tempfile.mkdtemp` 的取点）。
func edgTmpBase() string {
	d := os.Getenv("TMPDIR")
	if d != "" {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}

// edgPut —— py 自检内 `put(sub, name, body)`（回夹具目录）。
func edgPut(tmp, sub, name, body string) string {
	d := edgPyJoin(tmp, sub)
	p := edgPyJoin(d, name)
	_ = os.MkdirAll(filepath.Dir(p), 0o755) // 登记项现为仓相对路径（含子目录）⇒ 建到件所在目录
	_ = os.WriteFile(p, []byte(body), 0o644)
	return d
}

// 夹具正文（逐字照搬 py 自检内的三段拼接；件基名走拼串，✗ 不落连续魔法串）。
func edgFixtureGuarded() string {
	return "import os\n" +
		"def main():\n" +
		"    found = []\n" +
		"    for dp, _d, fs in os.walk('.'):\n        found += fs\n" +
		"    if require_domain(found, what='x', min_count=1):\n        return 2\n" +
		"    return 0\n"
}
func edgFixtureUnguarded() string {
	return "import os\n" +
		"def main():\n" +
		"    found = []\n" +
		"    for dp, _d, fs in os.walk('.'):\n        found += fs\n" +
		"    return 0\n"
}
func edgFixtureUnrelatedLen() string {
	return "import os\n" +
		"def main():\n" +
		"    found = []\n" +
		"    for dp, _d, fs in os.walk('.'):\n        found += fs\n" +
		"    parts = 'a_b_c'.split('_')\n" +
		"    if len(parts) >= 3:\n        return 0\n" +
		"    return 0\n"
}
func edgFixtureEmptyNoFc() string {
	return "import os\n" +
		"def main():\n" +
		"    found = []\n" +
		"    for dp, _d, fs in os.walk('.'):\n        found += fs\n" +
		"    if not found:\n        return 0\n" +
		"    return 0\n"
}

// edgSelfTest —— py `self_test()`（十档：①② 守卫 0/2 · ③ 元门正控 0 · ④④′④″ 元门负控 1 · ⑤⑤′ 2 · ⑥⑥′ 登记 1）。
func edgSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, edgSelfHeader)
	tmp, err := os.MkdirTemp(edgTmpBase(), "emptydomain-selftest-")
	if err != nil {
		// py 此路未捕异常（C 类）⇒ 本件 fail-closed 打具名文案 + rc=1（见未覆盖面⑥）。
		fmt.Fprintf(stdout, "✗ 建不了自检临时目录：%s\n", err.Error())
		fmt.Fprint(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 自回收（TMPDIR 前后 delta=0 · A 类）

	// ① 守卫正控：有件 ⇒ 0
	r0 := edgRequireDomain(stderr, 2, "合成·有件", 1)
	edgStChk(stdout, &bad, "档① require_domain 正控：有件 ⇒ rc=0", 0, r0, "")
	// ② 守卫负控：空域 ⇒ 2（BLOCKED 印到 stderr · 同 py）
	r2 := edgRequireDomain(stderr, 0, "合成·空域", 1)
	edgStChk(stdout, &bad, "档② require_domain 负控：空域 ⇒ rc=2", 2, r2, "")

	dOK := edgPut(tmp, "ok", "guarded.py", edgFixtureGuarded())
	dBad := edgPut(tmp, "bad", "unguarded.py", edgFixtureUnguarded())
	// 登记项现为仓相对 Go 真身名 ⇒ 扫面根 = <合成仓根>/scripts/gates（本门据 root 上溯两级定仓根）。
	dStale := edgPut(tmp, edgPyJoin("regstale", "scripts", "gates"), "a_scan.py", edgFixtureUnguarded())
	_ = edgPut(tmp, edgPyJoin("regstale", "core", "cmd", "zerg"), "check_version.go", edgFixtureUnguarded())

	// ③ 元门正控：扫面型 + 有守卫 ⇒ rc=0
	rc3, out3 := edgRunCapture([]string{"--root", dOK, "--baseline", "0"})
	edgStChk(stdout, &bad, "档③ 元门正控：扫面型件带守卫 ⇒ rc=0", 0, rc3, out3)
	// ④ 元门负控：扫面型 + 无守卫 ⇒ rc=1（点名）
	rc4, out4 := edgRunCapture([]string{"--root", dBad, "--baseline", "0"})
	edgStChk(stdout, &bad, "档④ 元门负控：扫面型件缺守卫 ⇒ rc=1（点名）", 1, rc4, out4)
	// ④′ 域级负控：只含无关 `len(x) >= 3` ⇒ rc=1（不靠无关比较洗绿）
	dUnrel := edgPut(tmp, "unrel", "unrelated_len.py", edgFixtureUnrelatedLen())
	rc4a, out4a := edgRunCapture([]string{"--root", dUnrel, "--baseline", "0"})
	edgStChk(stdout, &bad, "档④′ 域级负控：扫面型 + 只含无关 \x60len(x) >= 3\x60 ⇒ rc=1（不靠无关比较洗绿）", 1, rc4a, out4a)
	// ④″ 域级负控：空集检查但未 fail-closed（`if not found: return 0`）⇒ rc=1
	dNofc := edgPut(tmp, "nofc", "empty_no_fc.py", edgFixtureEmptyNoFc())
	rc4b, out4b := edgRunCapture([]string{"--root", dNofc, "--baseline", "0"})
	edgStChk(stdout, &bad, "档④″ 域级负控：扫面型 + 空集检查但未 fail-closed（\x60if not found: return 0\x60）⇒ rc=1", 1, rc4b, out4b)
	// ⑤ 缺语料：`--root` 不存在 ⇒ rc=2
	rc5, out5 := edgRunCapture([]string{"--root", edgPyJoin(tmp, "no-such-root")})
	edgStChk(stdout, &bad, "档⑤ 缺语料：\x60--root\x60 不存在 ⇒ rc=2（不给结论）", 2, rc5, out5)
	// ⑤′ 空域：`--root` 空目录（0 件）⇒ rc=2（空转 ≠ 绿）
	dEmpty := edgPyJoin(tmp, "empty-root")
	_ = os.MkdirAll(dEmpty, 0o755)
	rc5b, out5b := edgRunCapture([]string{"--root", dEmpty, "--baseline", "0"})
	edgStChk(stdout, &bad, "档⑤′ 空域：\x60--root\x60 空目录（0 件）⇒ rc=2（空转 ≠ 绿）", 2, rc5b, out5b)
	// ⑥ 登记表：登记项现读为扫面型 ⇒ rc=1（登记与结构不符 · 未被豁免扫面）
	rc6, out6 := edgRunCapture([]string{"--root", dStale, "--baseline", "0"})
	edgStChk(stdout, &bad, "档⑥ 登记表：登记项现读为扫面型 ⇒ rc=1（登记与结构不符 · 未被豁免扫面）", 1, rc6, out6)
	// ⑥′ 登记表报话自证
	cond := strings.Contains(out6, "非扫面型 · 已人工复核") && strings.Contains(out6, "登记与结构不符")
	want6 := 1
	if cond {
		want6 = 0
	}
	edgStChk(stdout, &bad, "档⑥′ 登记表：报话明说「非扫面型 · 已人工复核」且点名「登记与结构不符」", 0, want6, out6)

	if bad == 0 {
		fmt.Fprint(stdout, "── 自检：全过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// ── argv 解析（argparse 子集：-h/--help · --root · --baseline · --emit-baseline · --json · 缩写 · --）──

type edgOpts struct {
	root         string
	baseline     int
	emitBaseline bool
	json         bool
}

const (
	edgActOK = iota
	edgActHelp
	edgActErr
)

var edgLongOpts = []string{"--help", "--root", "--baseline", "--emit-baseline", "--json"}

var edgNegNumRE = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

// edgLooksLikeOption —— argparse `_parse_optional` 的「这枚 token 会不会被当成选项」判定。
func edgLooksLikeOption(s string) bool {
	if s == "" || s[0] != '-' {
		return false
	}
	if s == "-" {
		return false
	}
	if edgNegNumRE.MatchString(s) {
		return false
	}
	return true
}

// edgParseArgs —— argparse 子集（单遍左到右）：回 (opts, action, errMsg)。
// action ∈ {edgActOK, edgActHelp, edgActErr}；errMsg 非空 ⇒ 走 usage+error（rc=2）。
func edgParseArgs(argv []string) (edgOpts, int, string) {
	opts := edgOpts{root: edgDefaultRoot(), baseline: edgBaseline}
	extras := []string{}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			extras = append(extras, "--")
			extras = append(extras, argv[i+1:]...)
			break
		}
		if tok == "-h" {
			return opts, edgActHelp, ""
		}
		if !edgLooksLikeOption(tok) {
			extras = append(extras, tok)
			i++
			continue
		}
		name, inline, hasInline := tok, "", false
		if idx := strings.IndexByte(tok, '='); idx >= 0 {
			name, inline, hasInline = tok[:idx], tok[idx+1:], true
		}
		canon := ""
		if strings.HasPrefix(name, "--") {
			matches := []string{}
			for _, o := range edgLongOpts {
				if strings.HasPrefix(o, name) {
					matches = append(matches, o)
				}
			}
			switch {
			case len(matches) == 1:
				canon = matches[0]
			case len(matches) > 1:
				return opts, edgActErr, "ambiguous option: " + name + " could match " + strings.Join(matches, ", ")
			}
		}
		if canon == "" {
			extras = append(extras, tok)
			i++
			continue
		}
		switch canon {
		case "--help":
			return opts, edgActHelp, ""
		case "--json":
			if hasInline {
				return opts, edgActErr, "argument --json: ignored explicit argument '" + inline + "'"
			}
			opts.json = true
		case "--emit-baseline":
			if hasInline {
				return opts, edgActErr, "argument --emit-baseline: ignored explicit argument '" + inline + "'"
			}
			opts.emitBaseline = true
		case "--root", "--baseline":
			val := ""
			if hasInline {
				val = inline
			} else {
				if i+1 >= len(argv) || edgLooksLikeOption(argv[i+1]) {
					return opts, edgActErr, "argument " + canon + ": expected one argument"
				}
				val = argv[i+1]
				i++
			}
			if canon == "--root" {
				opts.root = val
			} else {
				n, ok := edgPyParseInt(val)
				if !ok {
					return opts, edgActErr, "argument --baseline: invalid int value: '" + val + "'"
				}
				opts.baseline = n
			}
		}
		i++
	}
	if len(extras) > 0 {
		return opts, edgActErr, "unrecognized arguments: " + strings.Join(extras, " ")
	}
	return opts, edgActOK, ""
}

// ── 默认根（声明 (a)：Go 无 `__file__` ⇒ cwd 上溯 · env 同名旋钮优先）─────────────────

// edgFindRepoRoot —— cwd 上溯至最近的含 `scripts/gates/` 的目录，找不到回落 cwd。
func edgFindRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := cwd
	for i := 0; i < 12 && d != "" && d != "/"; i++ {
		if st, err := os.Stat(edgPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return cwd
}

// edgDefaultRoot —— py `--root` 缺省：`ZERG_REPO_ROOT`（同名旋钮）优先，回落 cwd 上溯仓根 + scripts/gates。
func edgDefaultRoot() string {
	repo := os.Getenv("ZERG_REPO_ROOT")
	if repo == "" {
		repo = edgFindRepoRoot()
	}
	return edgPyJoin(repo, "scripts", "gates")
}

// ── 入口 ───────────────────────────────────────────────────────────────

// checkEmptyDomainGuardsGateMain —— 入口（与 py `main()` 逐字同构）。
func checkEmptyDomainGuardsGateMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv { // py：在 argparse 之前先嗅 `--self-test` / `--selftest`
		if a == "--self-test" || a == "--selftest" {
			return edgSelfTest(stdout, stderr)
		}
	}
	opts, action, errMsg := edgParseArgs(argv)
	switch action {
	case edgActHelp:
		fmt.Fprint(stdout, edgHelp)
		return 0
	case edgActErr:
		fmt.Fprint(stderr, edgUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", edgProg, errMsg)
		return 2
	}
	return edgRun(stdout, stderr, opts.root, opts.baseline, opts.json, opts.emitBaseline)
}
