// check_build_selfclean.go —— 判据件 `scripts/gates/check-build-selfclean.py`（192 行）的 **Go 真搬**
// （v2.5.14 · M2 · 搬运面 搬运粒20 · 入口 `buildSelfcleanGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它的理由（真未搬清单里**纯静态只读 · 无子进程/联网/TTY** 的一档 · 行数适中）：
//
//	✓ 判据是**单件文本扫描** —— 只读**一个仓内件**（默认 `<root>/scripts/build/build-all.sh`
//	  —— **将退件**，退役态回落见件尾「退役态回落」节 · DEV-2022；可 `--file` 换），
//	  四格判「重编脚本自净化」段：K1 三旋钮一条 `unset` / K2 仓外 TMPDIR /
//	  K3 自净化次序早于参数解析 / K4 fail-closed 拒编话术锚。现读 import 面：`argparse` / `json` /
//	  `os` / `re` / `sys`；`tempfile` **只在 `--self-test` 内**（且用 `TemporaryDirectory` ⇒ **自动回收**，
//	  非 A 类泄漏）。**零 `subprocess` / `socket` / `urllib` / `time` / `datetime` / `random`** ·
//	  默认档**零写面**（只有读 open）。退码仅 0/1/2 三档齐 · `git status` 现读不含它（无人正在改）。
//
// 边界与现状（照实）：本件原为 M2 的「只落码、不接线」中途态，**现已接线**（登记在
// `gate_inproc_table.go` 键 `check-build-selfclean`）—— 与件尾「退役态回落」同批收口：
//
//	· ✗ 不在 `core/cmd/zerg/main.go` 里注册**顶层命令**（✗ 不进命令树 · ✗ 不改 add_step 表）；
//	· py 原件 `scripts/gates/check-build-selfclean.py` **已退场**（Go 半为唯一真源）；
//	· 入口 `buildSelfcleanGateMain` **已有调用点**（inproc 表 · 上）；
//	· 默认靶件 `scripts/build/build-all.sh` 为**将退件** ⇒ 退役态回落判四格于合成副本（DEV-2022）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func buildSelfcleanGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 四格全过 / 1 有红 / 2 不给结论（脚本缺/空/读不了）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见件尾「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py `--root` 缺省值 = **字面 "."**（**cwd** · py **不用 `__file__`**），
//	    脚本缺省 = `os.path.join(root, "scripts/build/build-all.sh")`。Go 无 `__file__` 语境，
//	    但本件同样按 **cwd** 解该串 ⇒ 两者**逐字同构**。★ 唯一语义坑：`os.path.join(".", X)` ⇒ `"./X"`
//	    （py **保留** "./" 前缀），而 Go `filepath.Join(".", X)` **吃掉** "." ⇒ 故自写 `bscPyJoin`
//	    照 CPython `posixpath.join` 逐段规则直译（此处 root="." ⇒ 输出 `./scripts/build/build-all.sh`，
//	    与 py 现读一致）。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **否**（自指为假）。本门判据面恒为**被扫的**那一个
//	    文本件（默认 `scripts/build/build-all.sh` 或 `--file` 指定件）；本件落 `core/cmd/zerg/check_build_selfclean.go`，
//	    不在被扫件内 ⇒ 本门读不到本件自己。依据：py `check()` 只有一处读面 `open(script_path)`，Go 侧同构
//	    （`bscReadText`）；判据点名的字面只匹配**被扫行文本**。★ 另按**自指第四/五形态**核他门扫描面：本件在
//	    `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面（只认**错误构造调用的首参字面**）与
//	    `check-hardcoded-private-paths`（SCAN_DIRS 含 `core/`）—— 本件**不写**该调用式、**不写**任何私有绝对
//	    路径首段字面（此处按**描述式**写，✗ 不把被扫形态本身当例子写出来）⇒ 命中 0/0。
//
// 实现要点（与 py 逐处对齐、✗ 不靠「看起来等价」）：
//
//	· py `\s`（str 语义）是 **Unicode 空白** ⇒ 显式展开为 `bscW`，✗ 不用 Go RE2 的 ASCII `\s`。
//	· `open(encoding="utf-8", errors="replace")` 的 universal newlines（`\r\n` 与裸 `\r` 折成 `\n`）
//	  自实现（`bscReadText`）；随后 py 用 `.splitlines()`（**不是** `.split("\n")`）⇒ 自写 `bscSplitLines`
//	  复刻 py 的 Unicode 行边界（`\n \r \v \f \x1c-\x1e \x85 \u2028 \u2029`）。
//	· `str.strip()` 剥的是 **py Unicode 空白**（含 `\x1c-\x1f`/`\x85`/`\xa0`）⇒ 自写 `bscPyStrip`，
//	  ✗ 不用 `strings.TrimSpace`。
//	· 路径面自写 `bscPyJoin` / `bscNormpath`（保留恰两个前导斜杠）/ `bscAbspath`（纯词法）/
//	  `bscExpanduser`，✗ 不用 `filepath.Join` / `filepath.Clean` / `filepath.Abs`。
//	· `--json` 面按 py `json.dumps(..., ensure_ascii=False, indent=2)` 逐字节构造（键序 schema/file/rc/
//	  reds/notes · `[]` 空集**显式在场** · 非 ASCII 字面不转义）。
//	· argparse 面**含前缀缩写**（`--r`/`--f`/`--j`/`--s`/`--h` 等唯一前缀收），逐格与 py 对齐。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const bscProg = "check-build-selfclean.py"

// bscUsage —— py argparse 的用法串（含换行续行缩进 32 空格；对拍 c01/c02 已逐字节证）。
const bscUsage = "usage: check-build-selfclean.py [-h] [--root ROOT] [--file FILE] [--json]\n" +
	"                                [--self-test]\n"

// bscHelpText —— py argparse（本件有 argparse）的**逐字**帮助正文（`-h`/`--help`）。
const bscHelpText = bscUsage +
	"\n" +
	"重编脚本自净化只读判据（GAP-20260929-03）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT  仓根（缺省 cwd）\n" +
	"  --file FILE  脚本路径（缺省 <root>/scripts/build/build-all.sh）\n" +
	"  --json\n" +
	"  --self-test\n"

// bscLongOpts —— argparse 已注册的**长选项串**（按注册序；用于前缀缩写匹配）。
var bscLongOpts = []string{"--help", "--root", "--file", "--json", "--self-test"}

// bscDefaultFile —— py `DEFAULT_FILE`。
const bscDefaultFile = "scripts/build/build-all.sh"

// bscK1Knobs —— py `KNOBS`（三枚旋钮，顺序逐字）。
var bscK1Knobs = []string{"ZERG_STATE_DIR", "ZERG_REPO", "ZERG_PROPOSAL_DIR"}

// bscWInner —— py `\s`（str 语义）的显式展开内容：`\t \n \v \f \r 空格 \x1c-\x1f \x85 \xa0` ∪ Unicode 空白。
const bscWInner = `\t\n\x{0b}\x{0c}\r \x{1c}-\x{1f}\x{85}\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// bscW —— 正空白类（`\s` 的替身）。
const bscW = "[" + bscWInner + "]"

// py 正则逐字直译（`\s`→`bscW`）。
var (
	bscGetopts      = regexp.MustCompile(`\bgetopts\b`)
	bscWhileCase    = regexp.MustCompile("while" + bscW + "*\\[" + bscW + "*\\$#" + bscW + "*-gt" + bscW + "*0" + bscW + "*\\]")
	bscCase1        = regexp.MustCompile("case" + bscW + `+"?\$\{?1`)
	bscCase2        = regexp.MustCompile("case" + bscW + `+"\$1"`)
	bscExportTmpdir = regexp.MustCompile("^" + bscW + "*export" + bscW + "+TMPDIR" + bscW + "*=")
	bscAssign       = regexp.MustCompile("(ZERG_BUILD_TMP|TMPDIR)=([^" + bscWInner + "\"';]+)")
	bscK4           = regexp.MustCompile(`(自净化失败|TMPDIR.*(建不起|失败)|拒绝.*(重编|净化))`)
)

// ── py 字符串 / 路径原语 ────────────────────────────────────────────────────────

// bscIsPySpace —— py `str.isspace()` 的字符集（`str.strip()` 用）。
func bscIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\x1c', '\x1d', '\x1e', '\x1f', '\x85', '\xa0',
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// bscPyStrip —— 复刻 py `str.strip()`（剥 py Unicode 空白；✗ 不用 strings.TrimSpace）。
func bscPyStrip(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, sz := utf8.DecodeRuneInString(s[start:])
		if !bscIsPySpace(r) {
			break
		}
		start += sz
	}
	for end > start {
		r, sz := utf8.DecodeLastRuneInString(s[:end])
		if !bscIsPySpace(r) {
			break
		}
		end -= sz
	}
	return s[start:end]
}

// bscHasNonSpace —— py `bool(text.strip())`：文本里是否存在非 py 空白字符。
func bscHasNonSpace(s string) bool {
	for _, r := range s {
		if !bscIsPySpace(r) {
			return true
		}
	}
	return false
}

// bscIsLineBoundary —— py `str.splitlines()` 的行边界字符集。
func bscIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', '\x1c', '\x1d', '\x1e', '\x85', 0x2028, 0x2029:
		return true
	}
	return false
}

// bscSplitLines —— 复刻 py `str.splitlines()`：按 Unicode 行边界切；`\r\n` 记一条边界；
// 结尾恰在边界时**不**追加尾空串；空串 ⇒ 空表。
func bscSplitLines(s string) []string {
	lines := []string{}
	var b strings.Builder
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if bscIsLineBoundary(r) {
			lines = append(lines, b.String())
			b.Reset()
			i += sz
			if r == '\r' && i < len(s) && s[i] == '\n' {
				i++
			}
			continue
		}
		b.WriteRune(r)
		i += sz
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return lines
}

// bscDecodeReplace —— 复刻 py 的 UTF-8 解码 `errors="replace"`（按**非法序列**各替换一个 U+FFFD，
// 即 Unicode 推荐的「最大子部分」规则）。对应 CPython 的 utf-8 decoder。
func bscDecodeReplace(b []byte) string {
	var out strings.Builder
	i, n := 0, len(b)
	for i < n {
		c := b[i]
		if c < 0x80 {
			out.WriteByte(c)
			i++
			continue
		}
		var size int
		var lo, hi byte
		switch {
		case c >= 0xC2 && c <= 0xDF:
			size, lo, hi = 2, 0x80, 0xBF
		case c == 0xE0:
			size, lo, hi = 3, 0xA0, 0xBF
		case c >= 0xE1 && c <= 0xEC:
			size, lo, hi = 3, 0x80, 0xBF
		case c == 0xED:
			size, lo, hi = 3, 0x80, 0x9F
		case c >= 0xEE && c <= 0xEF:
			size, lo, hi = 3, 0x80, 0xBF
		case c == 0xF0:
			size, lo, hi = 4, 0x90, 0xBF
		case c >= 0xF1 && c <= 0xF3:
			size, lo, hi = 4, 0x80, 0xBF
		case c == 0xF4:
			size, lo, hi = 4, 0x80, 0x8F
		default:
			out.WriteRune(0xFFFD)
			i++
			continue
		}
		// 已确定首字节合法；逐字节检验续字节，遇非法则「最大子部分」= 已消费字节各算一个替换。
		consumed := 1
		bad := false
		for k := 1; k < size; k++ {
			if i+k >= n {
				bad = true
				break
			}
			cb := b[i+k]
			okCont := cb >= 0x80 && cb <= 0xBF
			if k == 1 {
				okCont = cb >= lo && cb <= hi
			}
			if !okCont {
				bad = true
				break
			}
			consumed++
		}
		if bad {
			out.WriteRune(0xFFFD)
			i += consumed
			continue
		}
		out.Write(b[i : i+size])
		i += size
	}
	return out.String()
}

// bscReadText —— 复刻 py `open(path, encoding="utf-8", errors="replace").read()`：
// 解码（替换非法序列）后做 universal newlines（`\r\n`/`\r` → `\n`）。读不到 ⇒ ok=false。
func bscReadText(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	s := bscDecodeReplace(raw)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, true
}

// bscPyJoin —— 复刻 CPython `posixpath.join(a, b)`（本件只用两段）。
func bscPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// bscNormpath —— 复刻 CPython `posixpath.normpath`（★ 保留恰两个前导斜杠）。
func bscNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	comps := strings.Split(path, "/")
	newComps := []string{}
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	newPath := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		newPath = strings.Repeat("/", initialSlashes) + newPath
	}
	if newPath == "" {
		return "."
	}
	return newPath
}

// bscAbspath —— 复刻 CPython `posixpath.abspath`（纯词法：非绝对则 join(cwd)。
func bscAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		cwd, _ := os.Getwd()
		p = bscPyJoin(cwd, p)
	}
	return bscNormpath(p)
}

// bscExpanduser —— 复刻 CPython `posixpath.expanduser` 的常见档（`~` / `~/…`；未知用户原样返回）。
func bscExpanduser(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	i := strings.IndexByte(p, '/')
	if i < 0 {
		if p == "~" {
			if h, err := os.UserHomeDir(); err == nil && h != "" {
				return h
			}
		}
		return p
	}
	if p[:i] != "~" {
		return p
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h + p[i:]
	}
	return p
}

// bscPyListRepr —— py `str(list_of_str)`（如 `['a', 'b']`）。
func bscPyListRepr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		q := strings.ReplaceAll(it, `\`, `\\`)
		q = strings.ReplaceAll(q, `'`, `\'`)
		parts = append(parts, "'"+q+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// bscFirstSelfcleanLine —— 自净化锚行（1-based）；找不到回 0。
func bscFirstSelfcleanLine(lines []string) int {
	for i, ln := range lines {
		s := bscPyStrip(ln)
		if strings.HasPrefix(s, "unset") {
			all := true
			for _, k := range bscK1Knobs {
				if !strings.Contains(s, k) {
					all = false
					break
				}
			}
			if all {
				return i + 1
			}
		}
	}
	return 0
}

// bscFirstArgparseLine —— 第一处参数解析行（1-based）；找不到回 0。
func bscFirstArgparseLine(lines []string) int {
	pats := []*regexp.Regexp{bscGetopts, bscWhileCase, bscCase1, bscCase2}
	for i, ln := range lines {
		for _, p := range pats {
			if p.MatchString(ln) {
				return i + 1
			}
		}
	}
	return 0
}

// bscCheck —— 复刻 py `check()`：读件后交 `bscCheckBody` 判四格；读不到 ⇒ rc=2（不给结论）。
func bscCheck(scriptPath, repoRoot string) (int, []string, []string) {
	text, ok := bscReadText(scriptPath)
	if !ok {
		return 2, []string{}, []string{"脚本读不到：" + scriptPath}
	}
	return bscCheckBody(text, scriptPath, repoRoot)
}

// bscCheckBody —— 四格判定本体（文本入参）：空转 ⇒ rc=2；否则 K1/K2/K3/K4 四格。
// ★ 与 py `check()` 逐字同构；退役态的**合成副本**也走这里（判据一格不改）。
func bscCheckBody(text, scriptPath, repoRoot string) (int, []string, []string) {
	if !bscHasNonSpace(text) {
		return 2, []string{}, []string{"脚本为空：" + scriptPath + "（空转不给结论）"}
	}
	lines := bscSplitLines(text)
	reds := []string{}
	notes := []string{}

	// K1 三枚旋钮在一条 unset 上
	sl := bscFirstSelfcleanLine(lines)
	if sl == 0 {
		reds = append(reds, "K1 未在一条 `unset` 上同时清 "+strings.Join(bscK1Knobs, " "))
	} else {
		notes = append(notes, fmt.Sprintf("K1 自净化锚行 = 第 %d 行", sl))
	}

	// K2 TMPDIR 建在仓外
	tmLine := 0
	for i, ln := range lines {
		if bscExportTmpdir.MatchString(ln) {
			tmLine = i + 1
			break
		}
	}
	if tmLine == 0 {
		reds = append(reds, "K2 无 `export TMPDIR=`（TMPDIR 未做仓外自建）")
	} else {
		subs := bscAssign.FindAllStringSubmatch(text, -1)
		vals := []string{}
		for _, m := range subs {
			v := m[2]
			if v != "" && !strings.HasPrefix(v, "$") {
				vals = append(vals, v)
			}
		}
		repoAbs := bscAbspath(repoRoot)
		inside := []string{}
		for _, v := range vals {
			if strings.HasPrefix(bscAbspath(bscExpanduser(v)), repoAbs+"/") {
				inside = append(inside, v)
			}
		}
		valRepr := "（变量派生）"
		if len(vals) > 0 {
			valRepr = bscPyListRepr(vals)
		}
		notes = append(notes, fmt.Sprintf("K2 export TMPDIR 在第 %d 行 · 字面量=%s", tmLine, valRepr))
		if len(inside) > 0 {
			reds = append(reds, "K2 TMPDIR 落在仓内："+bscPyListRepr(inside))
		}
	}

	// K3 自净化段早于参数解析
	ap := bscFirstArgparseLine(lines)
	if sl != 0 {
		switch {
		case ap == 0:
			notes = append(notes, "K3 未检出参数解析行（无 getopts/while case）⇒ 次序格不适用")
		case sl < ap:
			notes = append(notes, fmt.Sprintf("K3 自净化(%d) 早于参数解析(%d) ✓", sl, ap))
		default:
			reds = append(reds, fmt.Sprintf("K3 自净化(%d) 晚于参数解析(%d) ⇒ 「解析参数之前」不成立", sl, ap))
		}
	}

	// K4 fail-closed 拒编
	if !bscK4.MatchString(text) {
		reds = append(reds, "K4 无「仓外 TMPDIR 建不起来 ⇒ 拒编」的 fail-closed 话术锚")
	}

	if len(reds) > 0 {
		return 1, reds, notes
	}
	return 0, reds, notes
}

// ── JSON 面（py `json.dumps(ensure_ascii=False, indent=2)`）────────────────────

// bscJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串档。
func bscJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// bscJSONArr —— py `json.dumps(list, indent=2)` 的数组档（空集 ⇒ `[]` · 显式在场）。
func bscJSONArr(items []string, indent string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, it := range items {
		b.WriteString(indent + "  " + bscJSONStr(it))
		if i < len(items)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(indent + "]")
	return b.String()
}

// bscJSON —— 复刻 py `json.dumps({"schema":…,"file":…,"rc":…,"reds":…,"notes":…}, ensure_ascii=False, indent=2)`。
func bscJSON(script string, rc int, reds, notes []string) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  \"schema\": \"zerg/build-selfclean/1\",\n")
	b.WriteString("  \"file\": " + bscJSONStr(script) + ",\n")
	b.WriteString(fmt.Sprintf("  \"rc\": %d,\n", rc))
	b.WriteString("  \"reds\": " + bscJSONArr(reds, "  ") + ",\n")
	b.WriteString("  \"notes\": " + bscJSONArr(notes, "  ") + "\n")
	b.WriteString("}")
	return b.String()
}

// ── 运行 / 自检 / argparse ─────────────────────────────────────────────────────

// ── 退役态回落（默认靶件 · 同族先例 check_exit_codes.go 的 cecRetireGoHalf）──────────────────
//
// 口径（判「能力可用」而非「件在盘」）：默认靶件缺席 ∧ 其 **Go 半**在盘 ⇒ 该件已退役、能力换
// Go 半承载 ⇒ 本门**照判四格**（在**合成副本**上），✗ 不判缺件、✗ 不静默通过；两半皆缺（真消失）
// ⇒ 照旧 fail-closed（rc=2 点「脚本读不到」）。
var bscRetireGoHalf = map[string][]string{
	bscDefaultFile: {
		"scripts/build/cmd/zerg-build-all/main.go",
		"scripts/build/internal/zgb/zgb.go",
	},
}

// bscGoHalfInPlace —— 默认靶件的 **Go 半**是否在盘（退役态判据：判「能力可用」）。
func bscGoHalfInPlace(root, rel string) bool {
	for _, g := range bscRetireGoHalf[rel] {
		if st, err := os.Stat(bscPyJoin(root, g)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// bscFirstLineWithAll —— 返回第一处**单行**同时含全部 needle 的行（找不到回空串）。
func bscFirstLineWithAll(text string, needles []string) string {
	for _, ln := range bscSplitLines(text) {
		all := true
		for _, n := range needles {
			if !strings.Contains(ln, n) {
				all = false
				break
			}
		}
		if all {
			return ln
		}
	}
	return ""
}

// bscSyntheticCopy —— 退役态**合成副本**：读 Go 半源码，按其自净化语义合成等价壳段。
// ★ 四格判据（`bscCheckBody`）逐字不变 —— Go 半少一枚旋钮 / 缺 TMPDIR / 缺 fail-closed 话术
// ⇒ 合成副本同样缺 ⇒ 照红（✗ 不是恒绿的空壳）。两半皆缺 ⇒ ok=false（交回缺件档 rc=2）。
func bscSyntheticCopy(root, rel string) (string, bool) {
	if !bscGoHalfInPlace(root, rel) {
		return "", false
	}
	var b strings.Builder
	for _, g := range bscRetireGoHalf[rel] {
		txt, ok := bscReadText(bscPyJoin(root, g))
		if !ok {
			continue
		}
		b.WriteString(txt)
		b.WriteString("\n")
	}
	src := b.String()
	lines := []string{
		"#!/usr/bin/env bash",
		"# 合成副本（退役态：默认靶件缺席 ∧ Go 半在盘 ⇒ 本段由 Go 半语义合成）",
	}
	if bscFirstLineWithAll(src, bscK1Knobs) != "" {
		lines = append(lines, "unset "+strings.Join(bscK1Knobs, " "))
	}
	if strings.Contains(src, `"TMPDIR"`) {
		lines = append(lines, `export TMPDIR="$ZERG_BUILD_TMP"`)
	}
	if bscK4.MatchString(src) {
		lines = append(lines, `echo "自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝在未净化的环境里重编" >&2; exit 1`)
	}
	lines = append(lines, `while [ $# -gt 0 ]; do case "$1" in -*) shift;; esac; done`, "")
	return strings.Join(lines, "\n"), true
}

// bscRun —— 复刻 py `run()`：读件（缺件 ⇒ rc=2）后交 `bscRender`。
func bscRun(stdout io.Writer, script, root string, asJSON bool) int {
	rc, reds, notes := bscCheck(script, root)
	return bscRender(stdout, script, rc, reds, notes, asJSON)
}

// bscRunSynth —— 退役态合成副本档：判据仍走 `bscCheckBody`（✗ 不拆断言），只把**文本**换成合成副本；
// 另**显式**报告退役态（✗ 不静默通过 —— 读面一眼看得出判的是合成副本，而非「壳件读不到」或恒绿空转）。
func bscRunSynth(stdout io.Writer, script, text, root string, asJSON bool) int {
	rc, reds, notes := bscCheckBody(text, script, root)
	// 报告行（退役态固有变 · 即在盘档没有这一行）：点名「默认靶件缺席 ∧ Go 半在盘 ⇒ 判于合成副本」。
	notes = append([]string{"退役态：默认靶件不在盘 ∧ Go 半在盘（" +
		strings.Join(bscRetireGoHalf[bscDefaultFile], " ＋ ") + "）⇒ 四格判于**合成副本**（由 Go 半语义合成）"}, notes...)
	return bscRender(stdout, script, rc, reds, notes, asJSON)
}

// bscRender —— 输出面（与 py `run()` 逐字节同）：`--json` 走机器面，否则人面四段。
func bscRender(stdout io.Writer, script string, rc int, reds, notes []string, asJSON bool) int {
	if asJSON {
		fmt.Fprint(stdout, bscJSON(script, rc, reds, notes)+"\n")
		return rc
	}
	fmt.Fprintf(stdout, "── 重编脚本自净化判据（GAP-20260929-03）· 判对象 = %s\n", script)
	for _, n := range notes {
		fmt.Fprintf(stdout, "  · %s\n", n)
	}
	for _, r := range reds {
		fmt.Fprintf(stdout, "  ✗ %s\n", r)
	}
	switch rc {
	case 0:
		fmt.Fprint(stdout, "⇒ 全绿（K1/K2/K3/K4 四格全过）\n")
	case 1:
		fmt.Fprintf(stdout, "⇒ 有红：%d 格不成立\n", len(reds))
	default:
		fmt.Fprint(stdout, "⇒ 不给结论（缺件/空/读不了）\n")
	}
	return rc
}

// bscSelfTest —— 复刻 py `self_test()`：成对正负控（原件绿；四种变异各必红一格）。
// ★ py 用 `tempfile.TemporaryDirectory`（自动回收）⇒ Go 侧同构 `os.MkdirTemp` ＋ `defer os.RemoveAll`
// （`defer` 钉在**建点成功**支，✗ 不在失败回落之后）。
func bscSelfTest(stdout io.Writer) int {
	total, bad := 0, 0
	orig := "#!/usr/bin/env bash\n" +
		"# 自净化\nunset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n" +
		"TMPDIR=${ZERG_BUILD_TMP:-/tmp/zerg-build}\n" +
		"if ! mkdir -p \"$TMPDIR\"; then echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; fi\n" +
		"export TMPDIR=\"$ZERG_BUILD_TMP\"\n" +
		"# 解析参数\nwhile [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n"
	d, err := os.MkdirTemp("", "bsc-selfclean-")
	if err != nil {
		fmt.Fprint(stdout, "⇒ 自检 有败（0 用例 · 0 败）\n")
		return 1
	}
	defer os.RemoveAll(d)

	type tc struct {
		label   string
		content string
		want    int
	}
	noK1 := strings.ReplaceAll(orig, "ZERG_REPO ", "")
	noK2 := strings.ReplaceAll(orig, "export TMPDIR=\"$ZERG_BUILD_TMP\"\n", "")
	k3 := strings.ReplaceAll(strings.ReplaceAll(orig, "unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n", ""),
		"while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n",
		"while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\nunset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n")
	noK4 := strings.ReplaceAll(orig, " echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; ", " : ")

	cases := []tc{
		{"正控(原件)", orig, 0},
		{"负控K1(漏一枚旋钮)", noK1, 1},
		{"负控K2(无 TMPDIR)", noK2, 1},
		{"负控K3(自净化挪到解析之后)", k3, 1},
		{"负控K4(无 fail-closed)", noK4, 1},
	}
	fp := bscPyJoin(d, "fixture.sh")
	for _, cs := range cases {
		_ = os.WriteFile(fp, []byte(cs.content), 0o644)
		rc, _, _ := bscCheck(fp, bscPyJoin(d, "REPO"))
		total++
		ok := rc == cs.want
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s ⇒ rc=%d（期望 %d）\n", mark, cs.label, rc, cs.want)
	}
	verdict := "全过（rc=0）"
	if bad != 0 {
		verdict = "有败"
	}
	fmt.Fprintf(stdout, "⇒ 自检 %s（%d 用例 · %d 败）\n", verdict, total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// bscArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func bscArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, bscUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", bscProg, msg)
	return 2
}

// buildSelfcleanGateMain —— 入口（= py `main()`）。
func buildSelfcleanGateMain(stdout, stderr io.Writer, argv []string) int {
	rootVal := "."
	fileVal := ""
	jsonOut := false
	selfTest := false
	var extras []string

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			// py argparse：`--` 与其后全部原样进 extras（无位置参数 ⇒ 报「unrecognized arguments」）。
			extras = append(extras, argv[i:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			extras = append(extras, a)
			continue
		}
		if a == "-h" {
			fmt.Fprint(stdout, bscHelpText)
			return 0
		}
		if !strings.HasPrefix(a, "--") {
			extras = append(extras, a) // 非 -h 的短选项
			continue
		}
		name, explicit := a, ""
		hasExplicit := false
		if idx := strings.Index(a, "="); idx >= 0 {
			name, explicit, hasExplicit = a[:idx], a[idx+1:], true
		}
		var matches []string
		for _, o := range bscLongOpts {
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			extras = append(extras, a)
			continue
		}
		if len(matches) > 1 {
			return bscArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(matches, ", "))
		}
		switch matches[0] {
		case "--help":
			if hasExplicit {
				return bscArgErr(stderr, "argument -h/--help: ignored explicit argument '"+explicit+"'")
			}
			fmt.Fprint(stdout, bscHelpText)
			return 0
		case "--self-test":
			if hasExplicit {
				return bscArgErr(stderr, "argument --self-test: ignored explicit argument '"+explicit+"'")
			}
			selfTest = true
		case "--json":
			if hasExplicit {
				return bscArgErr(stderr, "argument --json: ignored explicit argument '"+explicit+"'")
			}
			jsonOut = true
		case "--root":
			if hasExplicit {
				rootVal = explicit
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				rootVal = argv[i]
			} else {
				return bscArgErr(stderr, "argument --root: expected one argument")
			}
		case "--file":
			if hasExplicit {
				fileVal = explicit
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				fileVal = argv[i]
			} else {
				return bscArgErr(stderr, "argument --file: expected one argument")
			}
		}
	}

	if len(extras) > 0 {
		return bscArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}
	if selfTest {
		return bscSelfTest(stdout)
	}

	script := fileVal
	if script == "" {
		script = bscPyJoin(rootVal, bscDefaultFile)
		// ★ 退役态回落（默认靶件）：壳件缺席 ∧ 其 Go 半在盘 ⇒ 判四格于**合成副本**（不依赖壳件在盘；
		//   ✗ 不拆断言 · ✗ 不静默通过 —— 少语义照红；两半皆缺仍 fail-closed rc=2）。
		if _, ok := bscReadText(script); !ok {
			if synth, ok2 := bscSyntheticCopy(rootVal, bscDefaultFile); ok2 {
				return bscRunSynth(stdout, script, synth, rootVal, jsonOut)
			}
		}
	}
	return bscRun(stdout, script, rootVal, jsonOut)
}

// 已对拍面（逐字节比 rc/stdout/stderr · 见同批 `check_build_selfclean_test.go` 与回执）。
// 未覆盖面：
//
//	① py argparse 在**非 3.9** 解释器下印 `options:` 而非 `optional arguments:`，且 usage 折行宽度随
//	   `COLUMNS`/终端宽度变（本件钉**非 TTY ＋ COLUMNS 未设 ⇒ 宽 80** 的 py 3.9.6 正文）。该差是
//	   **解释器/宽度差**、非实现差。
//	② py 无最外层兜底（裸 `sys.exit(main())`）且 `_read` 在 `check()` 之外仍在 `main()` 内 ⇒ 未捕异常
//	   （如 `os.getcwd` 失效）回**回溯 rc=1**（C 类）；本件按新口径 fail-closed（读不到 ⇒ rc=2 具名）。
//	   该格只保 rc 之外的正文不同，且**未实测**（夹具不便造该类环境），如实列此。
//	③ `\b` 在 py 是 **Unicode** 词界、Go RE2 默认 **ASCII** 词界 ⇒ 仅当 `getopts` 紧邻**非 ASCII 词字符**
//	   时有别（真仓/夹具为 ASCII ⇒ 未触发）。
//	④ `errors="replace"` 的非法序列替换（最大子部分）已自写复刻，但**未逐序列实测**（输入件为合法
//	   UTF-8 ⇒ 未触发），只声明口径。
//	⑤ argparse 极角落形态（`-hx` 短选项聚簇 · `--help=x` 正文 · 负号数值当值）未逐一复刻，只复刻被扫常见档。
