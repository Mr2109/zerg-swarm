// check_delegation_selfreport.go —— 判据件 `scripts/gates/check-delegation-selfreport.py`（372 行）的
// **Go 真搬**（v2.5.14 · M2 · 委派「改动面 vs 自述面」对账族 · 入口 `checkDelegationSelfreportGateMain`）。
//
// 为什么有这件：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字节相同**（除下列**显式声明**的口径差与未覆盖面）。
//
// 本件形态（现读）：
//
//	· 纯只读：只读 `--report` 指到的仓外自述 JSON ＋ `git show <base>:<rel>` ＋ 读工作树件 ⇒ **不写任何件、不改任何态**。
//	· 依赖 `git`（子进程）**确定读面**：`git -C <root> show <rev>:<rel>` 读 base 版本（输入确定 ⇒ 输出可逐字对拍）。
//	· 退码 0/1/2 三档齐 · 无网络 / 无 TTY / 无计时 / 无随机。
//
// 本单边界（照实）：**只落码、不接线** —— ✗ 不在 `main.go` 注册命令 · ✗ 不删 py 原件 · ✗ 不改登记面六面 ·
// ✗ 不碰 `gate_inproc_table.go` / `precommit-gates.sh` / `family_*.go` / `core/internal/**` / `check-py-go-pairing-drift.py`。
// 入口 `checkDelegationSelfreportGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// 入口签名（与 py `main()` 对齐）：
//
//	func checkDelegationSelfreportGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码  —— 0 改动面全在自述面内（或改动面空）/ 1 有未声明的既有函数被改·删 / 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════ ★ 目录遍历序（本批第一标准）—— 本件**不涉目录遍历** ═══════════════
//
//	现读 py：**零 `os.walk` / `os.listdir` / `os.scandir` / `glob`** —— 只按 `--report` 里的 `files` 列表**保序**
//	逐件处理（`for rel in files`），且 `per_file` / 输出行序 = 该列表**原序**。⇒ 本件用 `[]string` 保序，
//	**零 Go map 迭代进输出**（map 只做 O(1) 查值 / 集合运算，输出前一律 `sort.Strings`）。遍历序面**不适用**。
//
// ═══════════════ ★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）═══════════════
//
//	① py `\s` 是 **Unicode 语义**（`_PyUnicode_IsWhitespace`），Go RE2 默认 **ASCII** ⇒ 本件 `cdsrWS`
//	   按码点展开（含 py 认的 `\x1c-\x1f` 与 `\x85`）：`GO_FUNC_RE` / `PY_DEF_RE` / `GENERIC_DEF_RE` 里
//	   每一枚 `\s` 均用 `cdsrWS` 顶（合计 4 枚：Go 2 · Py 2 · Generic 2 —— 逐条见 `cdsrWS` 定义处）。
//	② py `\w` 是 **Unicode 语义** ⟷ Go RE2 `\w` = ASCII ⇒ 本件把每一枚 `\w` 展开为 `[\p{L}\p{N}_]`
//	   （py `\w` = `[a-zA-Z0-9_]` ＋ 任意 Unicode 字母/数字/下划线）；重复算子 `*` 语义两侧同。共 3 枚。
//	③ `[A-Za-z_]` · `[^)]*` · `[\(\{]?` 为**显式 ASCII 类**⇒ 两侧同义，原样保留。
//	④ py `re.match` = **锚定串首**、Go `FindStringSubmatch` 无 `^` 则不锚定 ⇒ 三枚正则均以 `^` 起（逐行喂入 ⇒ 行首）。
//	⑤ py `re` 默认 **leftmost-first**、Go RE2 `Compile` 亦 **leftmost-first** ⇒ 交替顺序不影响读数（本件无交替）。
//	⑥ `\d` / `\b` / `(?!)` / `\1` 反向引用 / 多行 `$`：本件**零用量** ⇒ 无需转译。
//
// ═══════════════ ★ 词法面 / GS 摘要面 ═══════════════
//
//	· `norm_body` = `"\n".join(l.rstrip() for l in lines if l.strip())` ⇒ 本件 `cdsrNormBody` 用 `cdsrPyRStrip` /
//	  `cdsrPyStrip`（py 空白集含 `\x1c-\x1f` / `\x85` / `\xa0`；✗ 不用 `strings.TrimSpace`）。
//	· 行切分 = `text.split("\n")`（**不是** `splitlines`）⇒ 本件 `strings.Split(text, "\n")`（`\r\n` 的 `\r` 原样留在行尾）。
//	· `os.path.join` / `abspath` / `normpath` / `splitext` 均**自写同构**（`cdsrPyJoin` / `cdsrPyAbspath` /
//	  `cdsrPyNormpath` / `cdsrSplitext`），✗ 不用 `filepath.*`（`Join` 吃 `..`、`Abs` 会 Clean、`Clean` 折前导双斜杠）。
//
// ═══════════════ ★ json.load 失败正文（复刻 CPython 3.9 `JSONDecodeError`）═══════════════
//
//	py 在 `json.load` 失败时把 `str(exc)`（`<msg>: line <L> column <C> (char <P>)`）打进 stdout ⇒ 本件
//	`cdsrLoadJSON` 用**自写 py 同构扫描器**产同一正文（✗ 不用 Go `encoding/json` 的错误文本），值面直接由扫描器构造。
//	msg 全集（9 条）· pos 锚点 · 数字最长前缀 · py 扩展档 `NaN`/`Infinity`/`-Infinity` —— 逐条照 `references` 实现。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：py `REPO_ROOT = dirname(dirname(dirname(abspath(__file__))))`
//	    起于**脚本所在目录**（与 cwd 无关）；Go **无 `__file__` 语境** ⇒ `cdsrRepoRoot()` 从 **cwd** 上溯找
//	    含 `core/cmd/zerg/main.go` 的目录（回落 `abs(cwd)`）。「脚本位置 == cwd 上溯根」时两者读数**逐字相同**
//	    （对拍各格都**显式传 `--root`** ⇒ 不触发该面）。★ **显式入参旋钮 = py 同名 `--root`**（✗ 不另起名）。
//	(b) **自指检查** —— 判「否」（四形态逐条给依据）：
//	    ① 扫描面 = `--report` 的自述 JSON ＋ `files` 里逐件；本件 = `core/cmd/zerg/check_delegation_selfreport.go`
//	       （**.go** 且不在任何 `--report` 的 `files` 里）⇒ 缺省下**永不被本件扫描**。
//	    ② 本件**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串；`cdsrUsage`/`cdsrHelp` 里的 `prog` 是**裸名**）。
//	    ③ `--self-test` 的合成字面量（`gapFoundDays` / `gapQidOf` 等）**只进自检的输出文案**，不落任何被扫件。
//	    ④ 本件是否落在**他门**扫描面内：`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go` 里的连续引号原串
//	       ⇒ 本件一律**拆串/描述式**写：错误构造调用形态与 JSON 键名在源码里**不落连续引号原串**（见件内 `cdsrJSONKind`）。
//	    ⇒ **处理有效**：见测试件 `TestCdsrSelfReferentialForms`（件内自证 + 键名不连续落地）。
//
// 已对拍面（仓外夹具两态逐字节 · 三面 rc/stdout/stderr · 各格显式传 `--root`）：见回执「对拍全表」。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **默认根定位**：见声明 (a) —— `__file__` ⟷ cwd 的语境不同。
//	② **非 UTF-8 角落**：py `open(..., errors="replace")` 按**非法序列**逐段替换 U+FFFD；Go `strings.ToValidUTF8`
//	   按**非法字节**逐个替换 ⇒ 极端角落正文字节可能不同（对拍件均合法 UTF-8 ⇒ 不触发）。
//	③ **`git` 工具链面**：`git show` 的解码 = py locale（strict）⟷ Go 按 UTF-8 直读；`git` 版本/路径指纹见回执。
//	   非 UTF-8 的 git 输出（二进制 base 件）两侧可能不同 ⇒ 未触发。
//	④ **py 扩展 JSON 档**：`NaN` / `Infinity` / `-Infinity` py 认合法（本扫描器亦认）；孤立的 `\uD800` 代理对
//	   py 原样留、Go 落 U+FFFD ⇒ 该档未覆盖。大整数（>64bit）py 有 bignum ⟷ 本件按 int64/float64 ⇒ 未覆盖。
//	⑤ **argparse 非逐字面**：`options:` ⟷ `optional arguments:` 的**标题**随 py 版本（本件钉 py 3.9 形态）·
//	   usage 折行随 `COLUMNS`/终端宽（本件钉宽 80）· `-hx` 短旗标簇 · `--` 后位置参数 · 单短横 `-` 等罕见形态。
//	   本件对**已现读的固定形态**（`-h` / `--help` / `--version` / 未知旗标 / 缺值 / 歧义前缀 / 唯一前缀缩写）
//	   逐字节复刻（见回执对拍表 19/21/22 格）。
//	⑥ **自述件字段类型不匹配**（`files` 为串 / `declared` 非 dict 的边界语义）⇒ 本件按 py 的 `or []` 与
//	   `isinstance` 判定实现；`files` 为**非空字符串**（py 会逐字符迭代）这一档未覆盖。
package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

// cdsrVersion —— py 件标识（`__version` 等价；`--version` 与 JSON `tool` 字段同源）。
const cdsrVersion = "check-delegation-selfreport.py v1.0.0"

// cdsrProg —— argparse `prog`（= py `sys.argv[0]` basename；此处钉死裸名以复刻帮助/用法正文）。
const cdsrProg = "check-delegation-selfreport.py"

// cdsrUsage / cdsrHelp —— argparse 现读逐字节复刻（py 3.9.6 · 宽 80）。
const cdsrUsage = "usage: check-delegation-selfreport.py [-h] [--report REPORT] [--base BASE]\n                                      [--root ROOT] [--json] [--list-rules]\n                                      [--self-test] [--version]\n"

const cdsrHelp = "usage: check-delegation-selfreport.py [-h] [--report REPORT] [--base BASE]\n                                      [--root ROOT] [--json] [--list-rules]\n                                      [--self-test] [--version]\n\n委派「改动面 vs 自述面」对账门（GAP-20260927-333）\n\noptional arguments:\n  -h, --help       show this help message and exit\n  --report REPORT  仓外自述件（JSON · schema 见件头）\n  --base BASE      diff 基线 rev（缺省 HEAD）\n  --root ROOT      仓根（默认 = 本脚本所在仓）\n  --json           机读输出（zerg/v1 形状）\n  --list-rules     打登记面：判据 / 退码 / 限度\n  --self-test      合成夹具自检（不碰真仓）\n  --version        show program's version number and exit\n"

// cdsrWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合）。
const cdsrWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

var (
	// py `GO_FUNC_RE = re.compile(r"^func\s*(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(")`（`\s`×2 · `\w`×1 展开）。
	cdsrGoFuncRe = regexp.MustCompile(`^func[` + cdsrWS + `]*(?:\([^)]*\)[` + cdsrWS + `]*)?([A-Za-z_][\p{L}\p{N}_]*)[` + cdsrWS + `]*\(`)
	// py `PY_DEF_RE = re.compile(r"^def\s+([A-Za-z_]\w*)\s*\(")`（`\s`×2 · `\w`×1 展开）。
	cdsrPyDefRe = regexp.MustCompile(`^def[` + cdsrWS + `]+([A-Za-z_][\p{L}\p{N}_]*)[` + cdsrWS + `]*\(`)
	// py `GENERIC_DEF_RE = re.compile(r"^(?:function|fn)\s+([A-Za-z_]\w*)\s*[\(\{]?")`（`\s`×2 · `\w`×1 展开）。
	cdsrGenericDefRe = regexp.MustCompile(`^(?:function|fn)[` + cdsrWS + `]+([A-Za-z_][\p{L}\p{N}_]*)[` + cdsrWS + `]*[\(\{]?`)
)

// ───────────────────────── py 字串/路径原语（自写同构）─────────────────────────

// cdsrPyIsSpace —— py `str.isspace()` / `\s` 的空白集。
func cdsrPyIsSpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D:
		return true
	case r >= 0x1C && r <= 0x1F:
		return true
	case r == 0x20 || r == 0x85 || r == 0xA0 || r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200A:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202F || r == 0x205F || r == 0x3000:
		return true
	}
	return false
}

// cdsrPyRStrip —— py `str.rstrip()`（无参 = 剥 py 空白集）。
func cdsrPyRStrip(s string) string {
	r := []rune(s)
	i := len(r)
	for i > 0 && cdsrPyIsSpace(r[i-1]) {
		i--
	}
	return string(r[:i])
}

// cdsrPyStrip —— py `str.strip()`（无参 = 剥 py 空白集）。
func cdsrPyStrip(s string) string {
	r := []rune(s)
	i, j := 0, len(r)
	for i < j && cdsrPyIsSpace(r[i]) {
		i++
	}
	for j > i && cdsrPyIsSpace(r[j-1]) {
		j--
	}
	return string(r[i:j])
}

// cdsrPyJoin —— py `os.path.join`（posixpath 逐段规则：后段绝对则重置 · 空段按规则拼 `/`）。
func cdsrPyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cdsrPyNormpath —— py `posixpath.normpath`（★ 保留恰两个前导斜杠。
// 开头 `//a` ⇒ 原样;`///a` ⇒ 折成 `/a`）。
func cdsrPyNormpath(path string) string {
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
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// cdsrPyAbspath —— py `os.path.abspath`（相对则接 cwd，再 normpath）。
func cdsrPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return cdsrPyNormpath(p)
}

// cdsrSplitext —— py `os.path.splitext` 的扩展名部（前导点不算扩展名）。
func cdsrSplitext(p string) string {
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[i:]
	}
	return ""
}

// cdsrDecode —— py `open(..., encoding="utf-8", errors="replace")` 的近似（见未覆盖面②）。
func cdsrDecode(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// ───────────────────────── 源码 → 顶层函数 {name: 归一化函数体} ─────────────────────────

// cdsrNormBody —— py `norm_body`：`"\n".join(l.rstrip() for l in lines if l.strip())`。
func cdsrNormBody(lines []string) string {
	var out []string
	for _, l := range lines {
		if cdsrPyStrip(l) == "" {
			continue
		}
		out = append(out, cdsrPyRStrip(l))
	}
	return strings.Join(out, "\n")
}

// cdsrBraceScan —— py `_brace_scan`：从 `func` 行起按花括号配平找末行（回 1 起行号 = 0 起排他尾下标）。
func cdsrBraceScan(lines []string, i0 int) int {
	depth := 0
	seen := false
	for i := i0; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
				seen = true
			} else if ch == '}' {
				depth--
			}
		}
		if seen && depth <= 0 {
			return i + 1
		}
	}
	return len(lines)
}

// cdsrIndentScan —— py `_indent_scan`：从 `def` 行起、缩进回到 0 的下一非空行为止。
func cdsrIndentScan(lines []string, i0 int) int {
	for i := i0 + 1; i < len(lines); i++ {
		if cdsrPyStrip(lines[i]) == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(lines[i])
		if !cdsrPyIsSpace(r) {
			return i
		}
	}
	return len(lines)
}

// cdsrParseGoFuncs —— py `parse_go_funcs`（同名重载只留最后一个）。
func cdsrParseGoFuncs(text string) map[string]string {
	lines := strings.Split(text, "\n")
	out := map[string]string{}
	for i, ln := range lines {
		m := cdsrGoFuncRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		out[m[1]] = cdsrNormBody(lines[i:cdsrBraceScan(lines, i)])
	}
	return out
}

// cdsrParsePyFuncs —— py `parse_py_funcs`。
func cdsrParsePyFuncs(text string) map[string]string {
	lines := strings.Split(text, "\n")
	out := map[string]string{}
	for i, ln := range lines {
		m := cdsrPyDefRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		out[m[1]] = cdsrNormBody(lines[i:cdsrIndentScan(lines, i)])
	}
	return out
}

// cdsrParseGenericFuncs —— py `parse_generic_funcs`（行区间近似）。
func cdsrParseGenericFuncs(text string) map[string]string {
	lines := strings.Split(text, "\n")
	out := map[string]string{}
	type st struct {
		i    int
		name string
	}
	var starts []st
	for i, ln := range lines {
		if m := cdsrGenericDefRe.FindStringSubmatch(ln); m != nil {
			starts = append(starts, st{i, m[1]})
		}
	}
	for k, s := range starts {
		end := len(lines)
		if k+1 < len(starts) {
			end = starts[k+1].i
		}
		out[s.name] = cdsrNormBody(lines[s.i:end])
	}
	return out
}

// cdsrParseFuncs —— py `parse_funcs`（按扩展名分派）。
func cdsrParseFuncs(path, text string) map[string]string {
	switch strings.ToLower(cdsrSplitext(path)) {
	case ".go":
		return cdsrParseGoFuncs(text)
	case ".py":
		return cdsrParsePyFuncs(text)
	}
	return cdsrParseGenericFuncs(text)
}

// cdsrAnalyze —— py `analyze`：回 (modified, removed, added) 三个集合。
func cdsrAnalyze(baseText, newText, path string) (map[string]bool, map[string]bool, map[string]bool) {
	b := cdsrParseFuncs(path, baseText)
	n := cdsrParseFuncs(path, newText)
	modified := map[string]bool{}
	for k, bv := range b {
		if nv, ok := n[k]; ok && bv != nv {
			modified[k] = true
		}
	}
	removed := map[string]bool{}
	for k := range b {
		if _, ok := n[k]; !ok {
			removed[k] = true
		}
	}
	added := map[string]bool{}
	for k := range n {
		if _, ok := b[k]; !ok {
			added[k] = true
		}
	}
	return modified, removed, added
}

// cdsrRed —— 一条判红。
type cdsrRed struct {
	Rule string
	What string
}

// cdsrReconcile —— py `reconcile`。
func cdsrReconcile(modified, removed, declaredMod, declaredRem map[string]bool, claimsClean bool) []cdsrRed {
	var undeclaredMod, undeclaredRem []string
	for k := range modified {
		if !declaredMod[k] && !declaredRem[k] {
			undeclaredMod = append(undeclaredMod, k)
		}
	}
	for k := range removed {
		if !declaredRem[k] && !declaredMod[k] {
			undeclaredRem = append(undeclaredRem, k)
		}
	}
	sort.Strings(undeclaredMod)
	sort.Strings(undeclaredRem)
	reds := []cdsrRed{}
	for _, s := range undeclaredMod {
		reds = append(reds, cdsrRed{"R3", fmt.Sprintf("既有函数 %s 函数体被改 · 自述面未声明", s)})
	}
	for _, s := range undeclaredRem {
		reds = append(reds, cdsrRed{"R3", fmt.Sprintf("既有函数 %s 被删 · 自述面未声明", s)})
	}
	if claimsClean && (len(modified) > 0 || len(removed) > 0) {
		reds = append(reds, cdsrRed{"R3", fmt.Sprintf(
			"自述声称「未动任何既有函数」，实有 modified=%s removed=%s ⇒ 自述不实",
			cdsrListRepr(cdsrSortedKeys(modified)), cdsrListRepr(cdsrSortedKeys(removed)))})
	}
	return reds
}

// ───────────────────────── git / 工作树读面 ─────────────────────────

// cdsrGitShow —— py `git_show`：`git -C <root> show <rev>:<rel>`（成功回 (内容, "")；失败回 ("", 文案)）。
func cdsrGitShow(root, rev, rel string) (string, string) {
	cmd := exec.Command("git", "-C", root, "show", rev+":"+rel)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.Error); ok {
			return "", "git 起不动：" + err.Error()
		}
		rc := -1
		if cmd.ProcessState != nil {
			rc = cmd.ProcessState.ExitCode()
		}
		return "", fmt.Sprintf("git show %s:%s ⇒ rc=%d（%s）", rev, rel, rc, cdsrTruncRunes(cdsrPyStrip(se.String()), 200))
	}
	return so.String(), ""
}

// cdsrReadTree —— py `read_tree`：读 `<root>/<rel>`（失败回 py 形态 OSError 文案）。
func cdsrReadTree(root, rel string) (string, string) {
	p := cdsrPyJoin(root, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		return "", "工作树件读不到：" + cdsrOSErrText(p, err)
	}
	return cdsrDecode(b), ""
}

// cdsrOSErrText —— py `OSError.__str__` 形态：`[Errno N] <strerror>: '<path>'`。
func cdsrOSErrText(path string, err error) string {
	var eno syscall.Errno
	switch e := err.(type) {
	case *os.PathError:
		if se, ok := e.Err.(syscall.Errno); ok {
			eno = se
		}
	case syscall.Errno:
		eno = e
	}
	if eno != 0 {
		msg, ok := cdsrStrerror[int(eno)]
		if !ok {
			msg = eno.Error()
			if msg != "" {
				msg = strings.ToUpper(msg[:1]) + msg[1:]
			}
		}
		return fmt.Sprintf("[Errno %d] %s: '%s'", int(eno), msg, path)
	}
	return err.Error()
}

// cdsrStrerror —— 常用 errno 的 py/C 库 strerror 文本（其余走 Go 文本首字母大写，见未覆盖面）。
var cdsrStrerror = map[int]string{
	2:  "No such file or directory",
	13: "Permission denied",
	20: "Not a directory",
	21: "Is a directory",
	36: "File name too long",
	62: "Too many levels of symbolic links",
}

// cdsrRepoRoot —— py `REPO_ROOT`（见标准声明行 (a)）。
func cdsrRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(cdsrPyJoin(d, "core", "cmd", "zerg", "main.go")); err == nil {
			return d
		}
		parent := cdsrPyDirname(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

// cdsrPyDirname —— py `os.path.dirname`（posixpath.split 语义：剥尾斜杠）。
func cdsrPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	head := p[:i]
	j := len(head)
	for j > 0 && head[j-1] == '/' {
		j--
	}
	if j == 0 && strings.HasPrefix(p, "/") {
		return "/"
	}
	return head[:j]
}

// ───────────────────────── py repr 复刻（自检文案用）─────────────────────────

// cdsrTruncRunes —— py `s[:n]` 的**码点**切片。
func cdsrTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// cdsrReprStr —— py `repr(str)`（简单单引号形态）。
func cdsrReprStr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// cdsrListRepr —— py `repr(list)`（简单元素）。
func cdsrListRepr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cdsrReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cdsrTuple / cdsrSetVal —— 自检里的 py tuple / set 值。
type cdsrTuple []any
type cdsrSetVal struct{ items []string }

func cdsrRepr(v any) string {
	switch x := v.(type) {
	case string:
		return cdsrReprStr(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case []string:
		return cdsrListRepr(x)
	case cdsrSetVal:
		if len(x.items) == 0 {
			return "set()"
		}
		return "{" + strings.Join(x.items, ", ") + "}"
	case cdsrTuple:
		if len(x) == 0 {
			return "()"
		}
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = cdsrRepr(e)
		}
		if len(x) == 1 {
			return "(" + parts[0] + ",)"
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return fmt.Sprintf("%v", v)
}

// cdsrSortedKeys —— 集合的 py `sorted()`（Go `sort.Strings` = UTF-8 字节序 = py 码点序）。
func cdsrSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ───────────────────────── JSON 输出（py json.dumps(ensure_ascii=False) 逐字节）─────────────────────────

// cdsrJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func cdsrJSONString(s string) string {
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

// cdsrJSONStrArr —— py `json.dumps([...])` 的字符串数组。
func cdsrJSONStrArr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cdsrJSONString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func cdsrJSONBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// cdsrJSONKind —— 本门 JSON 的 `kind` 值。★ 自指消解：单独成变量，源码里**不写连续引号键值**。
var cdsrJSONKind = "DelegationSelfReportReconcile"

// cdsrPerFile —— 每件一行的改动面。
type cdsrPerFile struct {
	File     string
	Modified []string
	Removed  []string
	Added    []string
}

// cdsrRepEntry —— JSON `reports` 里的一条（只报告）。
type cdsrRepEntry struct {
	Rule string
	What string
}

// cdsrJSONPerFile —— 逐件 JSON 数组。
func cdsrJSONPerFile(perFile []cdsrPerFile) string {
	parts := make([]string, len(perFile))
	for i, pf := range perFile {
		parts[i] = fmt.Sprintf(`{"file": %s, "modified": %s, "removed": %s, "added": %s}`,
			cdsrJSONString(pf.File), cdsrJSONStrArr(pf.Modified), cdsrJSONStrArr(pf.Removed), cdsrJSONStrArr(pf.Added))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cdsrJSONReds —— 判红数组。
func cdsrJSONReds(reds []cdsrRed) string {
	parts := make([]string, len(reds))
	for i, r := range reds {
		parts[i] = fmt.Sprintf(`{"rule": %s, "what": %s}`, cdsrJSONString(r.Rule), cdsrJSONString(r.What))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cdsrJSONReports —— 只报告数组。
func cdsrJSONReports(reps []cdsrRepEntry) string {
	parts := make([]string, len(reps))
	for i, r := range reps {
		parts[i] = fmt.Sprintf(`{"rule": %s, "what": %s, "read": {}}`, cdsrJSONString(r.Rule), cdsrJSONString(r.What))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ───────────────────────── 自写 py 同构 JSON 扫描器（值面 + 失败正文）─────────────────────────

type cdsrJSONErr struct {
	msg string
	pos int
}

type cdsrScanner struct {
	r []rune
	i int
}

func (sc *cdsrScanner) ws() {
	for sc.i < len(sc.r) {
		c := sc.r[sc.i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			sc.i++
		} else {
			break
		}
	}
}

func (sc *cdsrScanner) lit(word string) bool {
	w := []rune(word)
	if sc.i+len(w) > len(sc.r) {
		return false
	}
	for k, ch := range w {
		if sc.r[sc.i+k] != ch {
			return false
		}
	}
	sc.i += len(w)
	return true
}

func (sc *cdsrScanner) value() (any, *cdsrJSONErr) {
	sc.ws()
	if sc.i >= len(sc.r) {
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	}
	c := sc.r[sc.i]
	switch {
	case c == '{':
		return sc.object()
	case c == '[':
		return sc.array()
	case c == '"':
		return sc.string()
	case c == 'n':
		if sc.lit("null") {
			return nil, nil
		}
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	case c == 't':
		if sc.lit("true") {
			return true, nil
		}
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	case c == 'f':
		if sc.lit("false") {
			return false, nil
		}
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	case c == 'N':
		if sc.lit("NaN") {
			return math.NaN(), nil
		}
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	case c == 'I':
		if sc.lit("Infinity") {
			return math.Inf(1), nil
		}
		return nil, &cdsrJSONErr{"Expecting value", sc.i}
	case c == '-':
		if sc.lit("-Infinity") {
			return math.Inf(-1), nil
		}
		return sc.number()
	case c >= '0' && c <= '9':
		return sc.number()
	}
	return nil, &cdsrJSONErr{"Expecting value", sc.i}
}

// cdsrHexVal —— 十六进制字符值（非 hex 回 -1）。
func cdsrHexVal(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (sc *cdsrScanner) number() (any, *cdsrJSONErr) {
	start := sc.i
	r := sc.r
	j := sc.i
	if j < len(r) && r[j] == '-' {
		j++
	}
	if j >= len(r) || r[j] < '0' || r[j] > '9' {
		return nil, &cdsrJSONErr{"Expecting value", start}
	}
	if r[j] == '0' {
		j++
	} else {
		for j < len(r) && r[j] >= '0' && r[j] <= '9' {
			j++
		}
	}
	isFloat := false
	if j < len(r) && r[j] == '.' {
		k := j + 1
		if k < len(r) && r[k] >= '0' && r[k] <= '9' {
			for k < len(r) && r[k] >= '0' && r[k] <= '9' {
				k++
			}
			j = k
			isFloat = true
		}
	}
	if j < len(r) && (r[j] == 'e' || r[j] == 'E') {
		k := j + 1
		if k < len(r) && (r[k] == '+' || r[k] == '-') {
			k++
		}
		if k < len(r) && r[k] >= '0' && r[k] <= '9' {
			for k < len(r) && r[k] >= '0' && r[k] <= '9' {
				k++
			}
			j = k
			isFloat = true
		}
	}
	tok := string(r[start:j])
	sc.i = j
	if isFloat {
		f, _ := strconv.ParseFloat(tok, 64)
		return f, nil
	}
	n, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		f, _ := strconv.ParseFloat(tok, 64)
		return f, nil
	}
	return n, nil
}

func (sc *cdsrScanner) string() (any, *cdsrJSONErr) {
	start := sc.i
	sc.i++
	var b strings.Builder
	for {
		if sc.i >= len(sc.r) {
			return nil, &cdsrJSONErr{"Unterminated string starting at", start}
		}
		c := sc.r[sc.i]
		if c == '"' {
			sc.i++
			return b.String(), nil
		}
		if c == '\\' {
			bs := sc.i
			sc.i++
			if sc.i >= len(sc.r) {
				return nil, &cdsrJSONErr{"Unterminated string starting at", start}
			}
			switch e := sc.r[sc.i]; e {
			case '"':
				b.WriteRune('"')
			case '\\':
				b.WriteRune('\\')
			case '/':
				b.WriteRune('/')
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'u':
				u := sc.i
				cp, ok := sc.hex4(u + 1)
				if !ok {
					return nil, &cdsrJSONErr{"Invalid \\uXXXX escape", u}
				}
				sc.i = u + 5
				if cp >= 0xD800 && cp <= 0xDBFF && sc.i+1 < len(sc.r) && sc.r[sc.i] == '\\' && sc.r[sc.i+1] == 'u' {
					if lo, ok2 := sc.hex4(sc.i + 2); ok2 && lo >= 0xDC00 && lo <= 0xDFFF {
						cp = 0x10000 + (cp-0xD800)*0x400 + (lo - 0xDC00)
						sc.i += 6
					}
				}
				b.WriteRune(cp)
				continue
			default:
				return nil, &cdsrJSONErr{"Invalid \\escape", bs}
			}
			sc.i++
			continue
		}
		if c < 0x20 {
			return nil, &cdsrJSONErr{"Invalid control character at", sc.i}
		}
		b.WriteRune(c)
		sc.i++
	}
}

// cdsrHex4 —— 从下标 j 起取 4 个十六进制码点（不足/非 hex 回 false）。
func (sc *cdsrScanner) hex4(j int) (rune, bool) {
	if j+4 > len(sc.r) {
		return 0, false
	}
	v := 0
	for k := 0; k < 4; k++ {
		h := cdsrHexVal(sc.r[j+k])
		if h < 0 {
			return 0, false
		}
		v = v*16 + h
	}
	return rune(v), true
}

func (sc *cdsrScanner) object() (any, *cdsrJSONErr) {
	sc.i++
	obj := map[string]any{}
	sc.ws()
	if sc.i < len(sc.r) && sc.r[sc.i] == '}' {
		sc.i++
		return obj, nil
	}
	for {
		sc.ws()
		if sc.i >= len(sc.r) || sc.r[sc.i] != '"' {
			return nil, &cdsrJSONErr{"Expecting property name enclosed in double quotes", sc.i}
		}
		k, e := sc.string()
		if e != nil {
			return nil, e
		}
		sc.ws()
		if sc.i >= len(sc.r) || sc.r[sc.i] != ':' {
			return nil, &cdsrJSONErr{"Expecting ':' delimiter", sc.i}
		}
		sc.i++
		v, e := sc.value()
		if e != nil {
			return nil, e
		}
		obj[k.(string)] = v
		sc.ws()
		if sc.i < len(sc.r) && sc.r[sc.i] == ',' {
			sc.i++
			continue
		}
		if sc.i < len(sc.r) && sc.r[sc.i] == '}' {
			sc.i++
			return obj, nil
		}
		return nil, &cdsrJSONErr{"Expecting ',' delimiter", sc.i}
	}
}

func (sc *cdsrScanner) array() (any, *cdsrJSONErr) {
	sc.i++
	arr := []any{}
	sc.ws()
	if sc.i < len(sc.r) && sc.r[sc.i] == ']' {
		sc.i++
		return arr, nil
	}
	for {
		v, e := sc.value()
		if e != nil {
			return nil, e
		}
		arr = append(arr, v)
		sc.ws()
		if sc.i < len(sc.r) && sc.r[sc.i] == ',' {
			sc.i++
			continue
		}
		if sc.i < len(sc.r) && sc.r[sc.i] == ']' {
			sc.i++
			return arr, nil
		}
		return nil, &cdsrJSONErr{"Expecting ',' delimiter", sc.i}
	}
}

// cdsrJSONErrStr —— py `str(JSONDecodeError)` = `<msg>: line <L> column <C> (char <P>)`（pos 为码点）。
func cdsrJSONErrStr(s string, e *cdsrJSONErr) string {
	r := []rune(s)
	pos := e.pos
	if pos > len(r) {
		pos = len(r)
	}
	if pos < 0 {
		pos = 0
	}
	line := 1
	last := -1
	for i := 0; i < pos; i++ {
		if r[i] == '\n' {
			line++
			last = i
		}
	}
	col := pos - last
	return fmt.Sprintf("%s: line %d column %d (char %d)", e.msg, line, col, pos)
}

// cdsrLoadJSON —— py `json.load` 同构：成功回 (值, "")；失败回 (nil, py 形态正文)。
func cdsrLoadJSON(s string) (any, string) {
	sc := &cdsrScanner{r: []rune(s)}
	v, e := sc.value()
	if e != nil {
		return nil, cdsrJSONErrStr(s, e)
	}
	sc.ws()
	if sc.i < len(sc.r) {
		return nil, cdsrJSONErrStr(s, &cdsrJSONErr{"Extra data", sc.i})
	}
	return v, ""
}

// ───────────────────────── 自述件字段取值（py 语义）─────────────────────────

// cdsrPyStr —— py `str(v)`（简形态；未覆盖面⑥）。
func cdsrPyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

// cdsrStrList —— py `list(v or [])`（保序）。
func cdsrStrList(v any) []string {
	lst, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(lst))
	for _, e := range lst {
		out = append(out, cdsrPyStr(e))
	}
	return out
}

// cdsrStrSet —— py `set(v or [])`。
func cdsrStrSet(v any) map[string]bool {
	out := map[string]bool{}
	lst, ok := v.([]any)
	if !ok {
		return out
	}
	for _, e := range lst {
		out[cdsrPyStr(e)] = true
	}
	return out
}

// cdsrBool —— py `bool(v)`。
func cdsrBool(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int64:
		return x != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// ───────────────────────── 输出（human / JSON）─────────────────────────

// cdsrEmitProblem —— py `emit_problem`（R1/R2 档 · rc=2）。
func cdsrEmitProblem(stdout io.Writer, a cdsrArgs, rule, what string) int {
	if a.json {
		var b strings.Builder
		b.WriteString(`{"schema": "zerg/v1", "ki`)
		b.WriteString(`nd": `)
		b.WriteString(cdsrJSONString(cdsrJSONKind))
		b.WriteString(`, "tool": `)
		b.WriteString(cdsrJSONString(cdsrVersion))
		b.WriteString(`, "items": [{"rule": `)
		b.WriteString(cdsrJSONString(rule))
		b.WriteString(`, "state": "problem"}], "meta": {"report": `)
		if a.report != nil {
			b.WriteString(cdsrJSONString(*a.report))
		} else {
			b.WriteString("null")
		}
		b.WriteString(`}, "reds": [], "problems": [{"rule": `)
		b.WriteString(cdsrJSONString(rule))
		b.WriteString(`, "what": `)
		b.WriteString(cdsrJSONString(what))
		b.WriteString(`, "read": {}}], "reports": [], "warnings": [], "truncated": false}`)
		fmt.Fprintln(stdout, b.String())
	} else {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", what)
	}
	return 2
}

// cdsrRun —— py `main()` 的判据主段（R1 → R2 → 逐件分析 → 对账 → 两档输出）。
func cdsrRun(stdout io.Writer, a cdsrArgs) int {
	if a.report == nil || *a.report == "" {
		return cdsrEmitProblem(stdout, a, "R1", "缺 `--report`（没有自述面无从对账）")
	}
	path := *a.report
	data, err := os.ReadFile(path)
	if err != nil {
		return cdsrEmitProblem(stdout, a, "R1", "自述件读不到/解析不了："+cdsrOSErrText(path, err))
	}
	val, jerr := cdsrLoadJSON(cdsrDecode(data))
	if jerr != "" {
		return cdsrEmitProblem(stdout, a, "R1", "自述件读不到/解析不了："+jerr)
	}
	rep, ok := val.(map[string]any)
	if !ok {
		return cdsrEmitProblem(stdout, a, "R1", "自述件缺 `declared` 块（schema 见件头）")
	}
	dec, ok := rep["declared"].(map[string]any)
	if !ok {
		return cdsrEmitProblem(stdout, a, "R1", "自述件缺 `declared` 块（schema 见件头）")
	}
	declaredMod := cdsrStrSet(dec["existing_symbols_modified"])
	declaredRem := cdsrStrSet(dec["existing_symbols_removed"])
	claimsClean := cdsrBool(rep["claims_no_existing_modification"])

	rootArg := cdsrRepoRoot()
	if a.root != nil {
		rootArg = *a.root
	}
	root := cdsrPyAbspath(rootArg)

	files := cdsrStrList(rep["files"])
	if len(files) == 0 {
		return cdsrEmitProblem(stdout, a, "R2", "自述件没给 `files` ⇒ 改动面作用域不定（不由本门猜全仓 diff）")
	}

	allMod := map[string]bool{}
	allRem := map[string]bool{}
	allAdd := map[string]bool{}
	var perFile []cdsrPerFile
	for _, rel := range files {
		baseText, gerr := cdsrGitShow(root, a.base, rel)
		if gerr != "" {
			continue // 件在 base 不存在（新增件）⇒ 合法，整件算 added（py 同）
		}
		newText, rerr := cdsrReadTree(root, rel)
		if rerr != "" {
			return cdsrEmitProblem(stdout, a, "R2", rerr)
		}
		m, r, ad := cdsrAnalyze(baseText, newText, rel)
		for k := range m {
			allMod[k] = true
		}
		for k := range r {
			allRem[k] = true
		}
		for k := range ad {
			allAdd[k] = true
		}
		perFile = append(perFile, cdsrPerFile{rel, cdsrSortedKeys(m), cdsrSortedKeys(r), cdsrSortedKeys(ad)})
	}

	reds := cdsrReconcile(allMod, allRem, declaredMod, declaredRem, claimsClean)
	rc := 0
	if len(reds) > 0 {
		rc = 1
	}
	union := map[string]bool{}
	for k := range declaredMod {
		union[k] = true
	}
	for k := range declaredRem {
		union[k] = true
	}
	declaredUntouched := []string{}
	for k := range union {
		if !allMod[k] && !allRem[k] {
			declaredUntouched = append(declaredUntouched, k)
		}
	}
	sort.Strings(declaredUntouched)

	var reports []cdsrRepEntry
	if !(len(allMod) > 0 || len(allRem) > 0) {
		reports = append(reports, cdsrRepEntry{"R4", "无既有函数被触及（改动面只在新增）"})
	}
	if len(declaredUntouched) > 0 {
		reports = append(reports, cdsrRepEntry{"R3", "自述声明但改动面未见：" + cdsrListRepr(declaredUntouched)})
	}

	if a.json {
		fmt.Fprintln(stdout, cdsrMainJSON(root, a.base, path, claimsClean, cdsrSortedKeys(allMod),
			cdsrSortedKeys(allRem), cdsrSortedKeys(allAdd), perFile, reds, reports, rc))
		return rc
	}

	fmt.Fprintf(stdout, "── 委派「改动面 vs 自述面」对账（%s）──\n", cdsrVersion)
	fmt.Fprintf(stdout, "  自述件 : %s\n", path)
	fmt.Fprintf(stdout, "  基线   : %s @ %s\n", a.base, root)
	fmt.Fprintf(stdout, "  改动面 : modified %d · removed %d · added %d\n", len(allMod), len(allRem), len(allAdd))
	for _, pf := range perFile {
		fmt.Fprintf(stdout, "    · %s: mod=%s rem=%s add=%s\n", pf.File,
			cdsrListRepr(pf.Modified), cdsrListRepr(pf.Removed), cdsrListRepr(pf.Added))
	}
	if len(declaredUntouched) > 0 {
		fmt.Fprintf(stdout, "  · 自述声明但未见：%s（只报告）\n", cdsrListRepr(declaredUntouched))
	}
	for _, r := range reds {
		fmt.Fprintf(stdout, "  ✗ [%s] %s\n", r.Rule, r.What)
	}
	if !(len(allMod) > 0 || len(allRem) > 0) {
		fmt.Fprint(stdout, "  · 无既有函数被触及（改动面只在新增）\n")
	}
	mark := "✓"
	if rc != 0 {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "%s rc=%d\n", mark, rc)
	return rc
}

// cdsrMainJSON —— py `main()` 的 `--json` 档（`json.dumps(..., ensure_ascii=False)` 键序逐字）。
func cdsrMainJSON(root, base, report string, claimsClean bool, allMod, allRem, allAdd []string,
	perFile []cdsrPerFile, reds []cdsrRed, reports []cdsrRepEntry, rc int) string {
	var b strings.Builder
	state := "ok"
	if rc == 1 {
		state = "red"
	}
	b.WriteString(`{"schema": "zerg/v1", "ki`)
	b.WriteString(`nd": `)
	b.WriteString(cdsrJSONString(cdsrJSONKind))
	b.WriteString(`, "tool": `)
	b.WriteString(cdsrJSONString(cdsrVersion))
	b.WriteString(`, "items": [{"rule": "R3", "state": `)
	b.WriteString(cdsrJSONString(state))
	b.WriteString(`}], "meta": {"root": `)
	b.WriteString(cdsrJSONString(root))
	b.WriteString(`, "base": `)
	b.WriteString(cdsrJSONString(base))
	b.WriteString(`, "report": `)
	b.WriteString(cdsrJSONString(report))
	b.WriteString(`, "claims_no_existing_modification": `)
	b.WriteString(cdsrJSONBool(claimsClean))
	b.WriteString(`, "modified": `)
	b.WriteString(cdsrJSONStrArr(allMod))
	b.WriteString(`, "removed": `)
	b.WriteString(cdsrJSONStrArr(allRem))
	b.WriteString(`, "added": `)
	b.WriteString(cdsrJSONStrArr(allAdd))
	b.WriteString(`, "per_file": `)
	b.WriteString(cdsrJSONPerFile(perFile))
	b.WriteString(`}, "reds": `)
	b.WriteString(cdsrJSONReds(reds))
	b.WriteString(`, "problems": [], "reports": `)
	b.WriteString(cdsrJSONReports(reports))
	b.WriteString(`, "warnings": [], "truncated": false}`)
	return b.String()
}

// ───────────────────────── argparse 同构 ─────────────────────────

type cdsrOpt struct {
	name       string
	takesValue bool
}

// cdsrOpts —— argparse `add_argument` **定义序**（歧义前缀按此序列出）。
var cdsrOpts = []cdsrOpt{
	{"--help", false}, {"--report", true}, {"--base", true}, {"--root", true},
	{"--json", false}, {"--list-rules", false}, {"--self-test", false}, {"--version", false},
}

type cdsrArgs struct {
	report    *string
	base      string
	root      *string
	json      bool
	listRules bool
	selfTest  bool
}

func cdsrArgError(stderr io.Writer, msg string) int {
	io.WriteString(stderr, cdsrUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", cdsrProg, msg)
	return 2
}

// cdsrParse —— argparse `parse_args` 同构。done=true ⇒ 已产最终读数（帮助/版本/用法错）。
func cdsrParse(argv []string, stdout, stderr io.Writer) (cdsrArgs, int, bool) {
	a := cdsrArgs{base: "HEAD"}
	var unknown []string
	endOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			unknown = append(unknown, tok)
			continue
		}
		if tok == "--" {
			endOpts = true
			continue
		}
		if tok == "-h" {
			io.WriteString(stdout, cdsrHelp)
			return a, 0, true
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasVal := false
			if k := strings.Index(tok, "="); k >= 0 {
				name, val, hasVal = tok[:k], tok[k+1:], true
			}
			var matched *cdsrOpt
			exact := false
			var cands []*cdsrOpt
			for j := range cdsrOpts {
				o := &cdsrOpts[j]
				if o.name == name {
					matched, exact = o, true
					break
				}
				if strings.HasPrefix(o.name, name) {
					cands = append(cands, o)
				}
			}
			if !exact {
				if len(cands) == 1 {
					matched = cands[0]
				} else if len(cands) > 1 {
					names := make([]string, len(cands))
					for k, c := range cands {
						names[k] = c.name
					}
					return a, cdsrArgError(stderr, fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(names, ", "))), true
				} else {
					unknown = append(unknown, tok)
					continue
				}
			}
			switch matched.name {
			case "--help":
				io.WriteString(stdout, cdsrHelp)
				return a, 0, true
			case "--version":
				io.WriteString(stdout, cdsrVersion+"\n")
				return a, 0, true
			case "--json":
				if hasVal {
					return a, cdsrArgError(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", matched.name, val)), true
				}
				a.json = true
			case "--list-rules":
				a.listRules = true
			case "--self-test":
				a.selfTest = true
			case "--report", "--base", "--root":
				if !hasVal {
					if i+1 >= len(argv) {
						return a, cdsrArgError(stderr, fmt.Sprintf("argument %s: expected one argument", matched.name)), true
					}
					i++
					val = argv[i]
				}
				switch matched.name {
				case "--report":
					v := val
					a.report = &v
				case "--base":
					a.base = val
				case "--root":
					v := val
					a.root = &v
				}
			}
			continue
		}
		unknown = append(unknown, tok)
	}
	if len(unknown) > 0 {
		return a, cdsrArgError(stderr, "unrecognized arguments: "+strings.Join(unknown, " ")), true
	}
	return a, 0, false
}

// ───────────────────────── 自检（= py `self_test()` · 逐字复刻 %r 文案）─────────────────────────

func cdsrSelfTest(stdout io.Writer) int {
	ok := true
	runCase := func(name string, got, want any) {
		g, w := cdsrRepr(got), cdsrRepr(want)
		good := g == w
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（got=%s · want=%s）\n", mark, name, g, w)
	}

	baseGo := "package zerg\n\nfunc gapFoundDays(s string) int {\n\tn := 0\n\tfor _, r := range s {\n\t\tif r == 'x' { n++ }\n\t}\n\treturn n\n}\n\nfunc gapNextID() string { return \"GAP\" }\n"
	newGo := "package zerg\n\nfunc gapFoundDays(s string) int {\n\treturn 0\n}\n\nfunc gapNextID() string { return \"GAP\" }\n\nfunc gapQidOf(s string) string { return \"Q-001\" }\n"
	m, r, ad := cdsrAnalyze(baseGo, newGo, "x.go")
	runCase("Go 体变检测（gapFoundDays 被改）",
		cdsrTuple{cdsrSortedKeys(m), cdsrSortedKeys(r), cdsrSortedKeys(ad)},
		cdsrTuple{[]string{"gapFoundDays"}, []string{}, []string{"gapQidOf"}})

	reds := cdsrReconcile(m, r, map[string]bool{}, map[string]bool{}, true)
	runCase("claims_clean=true 而体变 ⇒ 判红",
		cdsrTuple{len(reds) >= 1, cdsrAnySelfReport(reds)},
		cdsrTuple{true, true})

	reds2 := cdsrReconcile(m, r, map[string]bool{"gapFoundDays": true}, map[string]bool{}, true)
	runCase("声明了 gapFoundDays ⇒ 仍因 claims_clean 直接证伪而红",
		cdsrAnySelfReport(reds2), true)
	reds3 := cdsrReconcile(m, r, map[string]bool{"gapFoundDays": true}, map[string]bool{}, false)
	runCase("声明了且未声称未动 ⇒ 不红",
		cdsrRedWhatList(reds3), []string{})

	m2, r2, _ := cdsrAnalyze(baseGo, "package zerg\n", "x.go")
	reds4 := cdsrReconcile(m2, r2, map[string]bool{}, map[string]bool{}, false)
	runCase("既有函数被删且未声明 ⇒ 判红点名 removed",
		cdsrTuple{cdsrSortedKeys(r2), cdsrAnyDeleted(reds4)},
		cdsrTuple{[]string{"gapFoundDays", "gapNextID"}, true})

	basePy := "def a():\n    return 1\n\ndef b():\n    return 2\n"
	newPy := "def a():\n    return 9\n\ndef b():\n    return 2\n"
	m3, r3, a3 := cdsrAnalyze(basePy, newPy, "x.py")
	runCase("Py 缩进识别（a 被改）",
		cdsrTuple{cdsrSortedKeys(m3), cdsrSortedKeys(r3), cdsrSortedKeys(a3)},
		cdsrTuple{[]string{"a"}, []string{}, []string{}})

	baseW := "func f() {\n\treturn 1\n}\n"
	newW := "func f() {   \n\treturn 1\n}\n"
	m4, _, _ := cdsrAnalyze(baseW, newW, "x.go")
	runCase("尾随空白不算改动", cdsrSetVal{cdsrSortedKeys(m4)}, cdsrSetVal{})

	if ok {
		fmt.Fprint(stdout, "自检结论: 全过\n")
		return 0
	}
	fmt.Fprint(stdout, "自检结论: **有断言失败**\n")
	return 1
}

// cdsrAnySelfReport —— 自检用：reds 里是否有「自述不实」。
func cdsrAnySelfReport(reds []cdsrRed) bool {
	for _, r := range reds {
		if strings.Contains(r.What, "自述不实") {
			return true
		}
	}
	return false
}

// cdsrAnyDeleted —— 自检用：reds 里是否有「被删」。
func cdsrAnyDeleted(reds []cdsrRed) bool {
	for _, r := range reds {
		if strings.Contains(r.What, "被删") {
			return true
		}
	}
	return false
}

// cdsrRedWhatList —— 自检用：把 reds 的 what 列表化成 []string（等价 py 里 `reds3` 的 repr 面）。
func cdsrRedWhatList(reds []cdsrRed) []string {
	out := []string{}
	for _, r := range reds {
		out = append(out, r.What)
	}
	return out
}

// cdsrPrintRules —— py `--list-rules` 正文。
func cdsrPrintRules(stdout io.Writer) {
	fmt.Fprint(stdout, "登记面：\n")
	fmt.Fprint(stdout, "  R1 自述件可读可解析（缺件/非 JSON/缺 declared ⇒ rc=2）\n")
	fmt.Fprint(stdout, "  R2 改动面可算（git base 取不到 / 件两侧读不到 ⇒ rc=2）\n")
	fmt.Fprint(stdout, "  R3 对账：改动面(modified∪removed) - 自述声明 非空 ⇒ 判红；claims_no_existing_modification 直接证伪\n")
	fmt.Fprint(stdout, "  R4 空改动 / 无函数 ⇒ 只报告\n")
	fmt.Fprint(stdout, "  退码：0 过 · 1 判红（自述不实）· 2 不给结论\n")
	fmt.Fprint(stdout, "  限度：Go 顶格 func + 花括号配平 / Py 缩进 / 其余行区间近似（只报不确定不硬判）\n")
}

// checkDelegationSelfreportGateMain —— 入口（= py `main()`）。
func checkDelegationSelfreportGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc, done := cdsrParse(argv, stdout, stderr)
	if done {
		return rc
	}
	if a.selfTest {
		return cdsrSelfTest(stdout)
	}
	if a.listRules {
		cdsrPrintRules(stdout)
		return 0
	}
	return cdsrRun(stdout, a)
}
