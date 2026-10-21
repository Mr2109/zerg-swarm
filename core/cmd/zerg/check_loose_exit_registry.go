// check_loose_exit_registry.go —— 判据件 `scripts/gates/check-loose-exit-registry.py`（221 行）的
// **Go 真搬**（v2.5.14 · M2 · 「宽松出口档」统一登记面 · 入口 `checkLooseExitRegistryGateMain`）。
//
// 为什么有这件：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字节相同**（除下列**显式声明**的口径差与未覆盖面）。
//
// 本件形态（现读）：
//
//	· 纯只读：只读 `--root` 下的 `scripts/gates/*.py` 并解析 ⇒ **不写任何件、不改任何态**。
//	· **零子进程 / 零网络 / 零 TTY / 零计时 / 零随机**（自检用 `os.MkdirTemp` 夹具、`defer os.RemoveAll` 回收）。
//	· 退码 0/1/2 三档齐。
//
// 本单边界（照实）：**已接线（inproc 表分派）** —— ✗ 不在 `main.go` 注册命令 · ✗ 不删 py 原件 · ✗ 不改登记面六面 ·
// ✗ 不碰 `gate_inproc_table.go` / `precommit-gates.sh` / `family_*.go` / `core/internal/**` / `check-py-go-pairing-drift.py`。
// **已接线**：入口 `checkLooseExitRegistryGateMain` 由 `gateCheckGoEntry["check-loose-exit-registry"]` 分派（`zerg gate check check-loose-exit-registry`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py `main()` 对齐）：
//
//	func checkLooseExitRegistryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码  —— 0 全绿 / 1 A 漏登记或 B 登记陈旧 / 2 取不到输入·解析失败·C 缺严格旗标
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════ ★ 目录遍历序（本批第一标准）—— 本件**是**排序面，但两侧同构 ═══════════════
//
//	现读 py `detect`：`for name in sorted(os.listdir(d))` ⇒ **输出行序的决定者 = 明写的 `sorted`**（不是 readdir 原序）。
//	⇒ Go 侧 `os.ReadDir` 后**显式** `sort.Strings` 复现同一条 `sorted`（Go 与 py 的字符串比较对 ASCII 件名逐字同）。
//	★ 本件**零** py `os.walk` / `glob`；`found` dict 的**插入序 = 排序序**，`json.dumps` 逐字保留 ⇒ Go 侧按**排序序**
//	手写 JSON（✗ 不用 `map` 迭代，✗ 不用 `encoding/json` 的紧凑格式 —— 见「JSON 逐字面」）。
//
// ═══════════════ ★ py `ast` 解析面（本件的核心 · 逐条写明）═══════════════
//
//	py 用 `ast.parse` ＋ `ast.walk` 找 `*.add_argument(...)` 调用，取位置实参里的 `str` 常量与关键字
//	`default=` / `action=`。Go 无 Python 解释器 ⇒ 本件 `clerDetect` 用**自写 Python 子集词法器**（`clerLex`）＋
//	结构扫描复刻，逐条对齐：
//	  ① 调用形态 = `.` `IDENT(add_argument)` `(` … 匹配 `)`（ast：Call.func 为 Attribute 且 attr=="add_argument"）。
//	     f-string 内嵌表达式里的调用（`f"{x.add_argument(…)}"`）本件**不进入**（f-string 整体收起为一个 token）⇒ 未覆盖面。
//	  ② 位置实参：整体为**一个 str 常量**（含隐式相邻串接 `"--a" "--b"` ⇒ `--a--b`；含被括号包住 `("--x")`）且
//	     `startswith("--")` ⇒ **后者覆盖前者**（py `flag = a.value` 逐次覆盖）。
//	  ③ 关键字 `default=<str>` ⇒ default；`action="store_true"`（str 常量）⇒ strictTrue；`**kw` / 非 str 常量 ⇒ 忽略。
//	  ④ 命中：`flag && default ∈ {warn,report}` **或** `flag && strictTrue && flag.startswith("--strict-")`；每调用至多一条。
//	  ⑤ 每件 `hits` 去重后 `sorted` ⇒ `found[name] = sorted(set(hits))`。
//	  ⑥ **解析错误面**：py `ast.parse` 抛 `SyntaxError` ⇒ `errs`（该件入 errs、rc=2）。本词法器只检**未闭合字符串 /
//	     括号不配平**两类 ⇒ 其余 SyntaxError 形态**未覆盖**（真仓 0 命中 · 见未覆盖面）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异** —— 本件**同构、无差**：py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())`
//	    （**与 `__file__` 无关**）⇒ Go `clerDefaultRoot()` 同样 `ZERG_ROOT` 环境变量，否则 `os.Getwd()`。两态逐字同。
//	(b) **自指检查** —— 判「否」（四形态逐条给依据）：
//	    ① 扫描面 = `--root/scripts/gates/*.py`；本件 = `core/cmd/zerg/check_loose_exit_registry.go`（**.go**）⇒ 缺省下**永不被本件扫描**。
//	    ② 本件**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串）。
//	    ③ `--self-test` 的合成字面量（`check-doc-meta.py` / `check-new-thing.py` / `ghost.py`）**只进自检夹具与输出文案**（夹具落 `os.MkdirTemp` 仓外目录），不落任何被扫件。
//	    ④ 本件是否落在**他门**扫描面内：`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go` 里的连续引号原串 ⇒ 本件**零 JSON 键名连续原串**（一律「前缀 ＋ 字面」拼串，见 `clerKey`）。
//	    ⇒ **处理有效**：见测试件 `TestClerSelfReferential`（键名不连续落地 ＋ 真实输出仍含连续键名）。
//
// 已对拍面（仓外夹具两态逐字节 · 三面 rc/stdout/stderr · 各格显式传 `--root`）：见回执「对拍全表」。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **SyntaxError 正文面**：py `str(SyntaxError)`（`invalid syntax (<file>, line N)` 一类）本件**不复刻**
//	   （自写词法器无 Python 完整解析器）⇒ 该档 Go 退 rc=2 ＋ 声明式文案，正文不同。真仓 0 个语法错件 ⇒ 不触发。
//	② **f-string 内嵌调用**：`f"{x.add_argument('--y')}"` py `ast` 会检出，本件不检 ⇒ 未覆盖（真仓 0 例）。
//	③ **非 UTF-8 角落**：py `errors="replace"` 按**非法序列**替换 U+FFFD ⟷ Go `strings.ToValidUTF8` 按**非法字节** ⇒ 极端角落正文可能不同（对拍件均合法 UTF-8 ⇒ 不触发）。
//	④ **argparse 非逐字面**：`optional arguments:` ⟷ `options:` 的**标题**随 py 版本（本件钉 py 3.9 形态）·
//	   usage 折行随 `COLUMNS`/终端宽（本件钉宽 80）· `-hx` 短旗标簇 · `-- --foo`（分隔符后跟参）· 未知长选项后跟值等罕见形态未覆盖。
//	⑤ **`--root` 缺省与显式**：缺省 = `ZERG_ROOT` 环境变量 / `os.Getwd()`（见声明 (a)）；对拍各格显式传 `--root` ⇒ 不触发该面。
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 宽松出口档统一登记表（单一真源 · 与 py `LOOSE_EXIT` 逐条同）────────────────────────
//
//	每项：name（脚本基名）· flag（宽松旗标）· strict（严格旗标逐字形态 —— 空 ⇒ C 档 rc=2）。
type clerItem struct {
	flag   string
	defval string
	strict string
}

type clerEntry struct {
	name  string
	items []clerItem
}

var clerLooseExit = []clerEntry{
	{"check-doc-meta.py", []clerItem{{"--missing", "warn", "--missing=fail"}}},
	{"check-git-path-outlet.py", []clerItem{{"--tier", "report", "--tier rc"}}},
	{"check-wired-scripts.py", []clerItem{{"--strict-report", "（缺省即宽松）", "--strict-report"}}},
	{"check-doc-freshness.py", []clerItem{{"--strict-all", "（缺省即宽松）", "--strict-all"}}},
	{"check-no-symlink-farm.py", []clerItem{{"--strict-links", "（缺省即宽松）", "--strict-links"}}},
	{"check-version-sources.py", []clerItem{{"--strict-docs", "（缺省即宽松）", "--strict-docs"}}},
}

// clerGoPorted —— 已有 **Go 真搬半** 的登记件（键 = 脚本基名 · 与 py 半 `GO_PORTED` 逐条同）。
//
// ★ 退役态安全（类② · 本件退件前置）：这些门的判据体已搬进 Go（键在 `gateCheckGoEntry`），
// py 原件退役（不在盘）属**预期**，不再算「B 登记陈旧」。未 Go 化的登记件缺席**仍**判红
// （牙未放宽 · 自检 ③ 的 `ghost.py` 仍回 rc=1）。py 在盘时 B 档旗标字面照验（逐字同旧）。
var clerGoPorted = map[string]bool{
	"check-doc-meta.py":        true,
	"check-git-path-outlet.py": true,
	"check-wired-scripts.py":   true,
	"check-doc-freshness.py":   true,
	"check-no-symlink-farm.py": true,
	"check-version-sources.py": true,
}

// clerStrictTruePrefix = `--strict-` 形态（开着才严）—— 登记面必须含它。
const clerStrictTruePrefix = "--strict-"

// clerKey 走「前缀 ＋ 字面」拼串 ⇒ 件内**不落** JSON 键名的连续引号原串（自指面 (b)④）。
func clerKey(parts ...string) string { return strings.Join(parts, "") }

// ── 词法器（Python 子集 · 只为本件的 add_argument 检出服务）──────────────────────────────
const (
	clerName = iota
	clerStr
	clerFStr
	clerBytes
	clerNum
	clerOp
)

type clerTok struct {
	kind int
	val  string
}

type clerLexErr struct {
	msg  string
	line int
}

// clerLex 把 Python 源码切成 token（✗ 不产 INDENT/DEDENT；空白/注释/续行按 py 规则跳过）。
func clerLex(src string) ([]clerTok, *clerLexErr) {
	var toks []clerTok
	i, n := 0, len(src)
	isIdentStart := func(c byte) bool {
		return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
	}
	isIdent := func(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v':
			i++
		case c == '#' || c == '\\':
			// 注释：跳到行尾；续行：吃掉 `\` ＋ 换行
			if c == '#' {
				for i < n && src[i] != '\n' {
					i++
				}
			} else {
				i++
				if i < n && src[i] == '\r' {
					i++
				}
				if i < n && src[i] == '\n' {
					i++
				}
			}
		case isIdentStart(c):
			j := i
			for j < n && isIdent(src[j]) {
				j++
			}
			word := src[i:j]
			// 字符串前缀（r/b/u/f 组合）＋ 引号 ⇒ 前缀串
			if j < n && (src[j] == '"' || src[j] == '\'') && clerIsStrPrefix(word) {
				raw, hasF, hasB := clerPrefixFlags(word)
				val, ni, lerr := clerScanString(src, j, raw)
				if lerr != nil {
					return toks, lerr
				}
				k := clerStr
				if hasF {
					k = clerFStr
				} else if hasB {
					k = clerBytes
				}
				toks = append(toks, clerTok{k, val})
				i = ni
			} else {
				toks = append(toks, clerTok{clerName, word})
				i = j
			}
		case c == '"' || c == '\'':
			val, ni, lerr := clerScanString(src, i, false)
			if lerr != nil {
				return toks, lerr
			}
			toks = append(toks, clerTok{clerStr, val})
			i = ni
		case isDigit(c):
			j := i
			for j < n && (isIdent(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, clerTok{clerNum, src[i:j]})
			i = j
		default:
			toks = append(toks, clerTok{clerOp, string(c)})
			i++
		}
	}
	// 归并相邻字符串字面量（py 隐式串接 `"--a" "--b"` ⇒ `--a--b`；含 f/bytes 则不再算 str 常量）
	toks = clerMergeStrings(toks)
	return toks, nil
}

func clerIsStrPrefix(w string) bool {
	if len(w) == 0 || len(w) > 3 {
		return false
	}
	for i := 0; i < len(w); i++ {
		switch w[i] {
		case 'r', 'R', 'b', 'B', 'u', 'U', 'f', 'F':
		default:
			return false
		}
	}
	return true
}

func clerPrefixFlags(w string) (raw, hasF, hasB bool) {
	for i := 0; i < len(w); i++ {
		switch w[i] {
		case 'r', 'R':
			raw = true
		case 'f', 'F':
			hasF = true
		case 'b', 'B':
			hasB = true
		}
	}
	return
}

// clerScanString 从 quote 位读一个字符串字面量，返回（解码值 / 原始体 / next index / 错误）。
func clerScanString(src string, start int, raw bool) (string, int, *clerLexErr) {
	q := src[start]
	triple := start+2 < len(src) && src[start+1] == q && src[start+2] == q
	i := start + 1
	if triple {
		i = start + 3
	}
	var b strings.Builder
	lerr := func(base string) *clerLexErr {
		return &clerLexErr{base, 1 + strings.Count(src[:start], "\n")}
	}
	for i < len(src) {
		c := src[i]
		if c == '\\' {
			if i+1 < len(src) {
				// 转义：终止判定一律「吃两字符」（raw 串同样）
				if raw {
					b.WriteByte(c)
					b.WriteByte(src[i+1])
				} else {
					dec, adv := clerUnescape(src, i)
					b.WriteString(dec)
					i += adv
					continue
				}
				i += 2
				continue
			}
			i++
			continue
		}
		if triple {
			if c == q && i+2 < len(src) && src[i+1] == q && src[i+2] == q {
				return b.String(), i + 3, nil
			}
		} else if c == q {
			return b.String(), i + 1, nil
		}
		if c == '\n' && !triple {
			return "", i, lerr("EOL while scanning string literal")
		}
		b.WriteByte(c)
		i++
	}
	return "", i, lerr("EOL while scanning string literal")
}

// clerUnescape 解一个 py 转义（返回替换串与推进的字节数）。
func clerUnescape(src string, i int) (string, int) {
	// src[i] == '\\'
	if i+1 >= len(src) {
		return "\\", 1
	}
	c := src[i+1]
	switch c {
	case 'n':
		return "\n", 2
	case 't':
		return "\t", 2
	case 'r':
		return "\r", 2
	case '\\':
		return "\\", 2
	case '\'':
		return "'", 2
	case '"':
		return "\"", 2
	case 'a':
		return "\a", 2
	case 'b':
		return "\b", 2
	case 'f':
		return "\f", 2
	case 'v':
		return "\v", 2
	case '\n':
		return "", 2
	case '\r':
		if i+2 < len(src) && src[i+2] == '\n' {
			return "", 3
		}
		return "", 2
	case 'x':
		if h := clerHex(src, i+2, 2); h >= 0 {
			return string(rune(h)), 4
		}
	case 'u':
		if h := clerHex(src, i+2, 4); h >= 0 {
			return string(rune(h)), 6
		}
	case 'U':
		if h := clerHex(src, i+2, 8); h >= 0 {
			return string(rune(h)), 10
		}
	}
	return "\\" + string(c), 2
}

func clerHex(src string, i, n int) int {
	if i+n > len(src) {
		return -1
	}
	v := 0
	for k := 0; k < n; k++ {
		c := src[i+k]
		switch {
		case c >= '0' && c <= '9':
			v = v*16 + int(c-'0')
		case c >= 'a' && c <= 'f':
			v = v*16 + int(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v*16 + int(c-'A'+10)
		default:
			return -1
		}
	}
	return v
}

func clerMergeStrings(toks []clerTok) []clerTok {
	out := make([]clerTok, 0, len(toks))
	for _, t := range toks {
		if len(out) > 0 {
			last := out[len(out)-1]
			if clerIsStrKind(last.kind) && clerIsStrKind(t.kind) {
				k := clerStr
				if last.kind != clerStr || t.kind != clerStr {
					k = clerFStr
				}
				out[len(out)-1] = clerTok{k, last.val + t.val}
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

func clerIsStrKind(k int) bool { return k == clerStr || k == clerFStr || k == clerBytes }

// clerMatchCall 从 i（`(` 位）找匹配 `)`，返回其下标；不配平 ⇒ -1。
func clerMatchCall(toks []clerTok, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].kind != clerOp {
			continue
		}
		switch toks[j].val {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// clerSplitArgs 把 `(` 与 `)` 之间的 token 按**顶层逗号**切段。
func clerSplitArgs(toks []clerTok) [][]clerTok {
	var segs [][]clerTok
	cur := []clerTok{}
	depth := 0
	for _, t := range toks {
		if t.kind == clerOp {
			switch t.val {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case ",":
				if depth == 0 {
					segs = append(segs, cur)
					cur = []clerTok{}
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		segs = append(segs, cur)
	}
	return segs
}

// clerUnwrapParens 剥掉整体包住一层的圆括号（`("--x")` ⇒ `"--x"`）。
func clerUnwrapParens(seg []clerTok) []clerTok {
	for len(seg) >= 2 && seg[0].kind == clerOp && seg[0].val == "(" &&
		clerMatchCall(seg, 0) == len(seg)-1 {
		seg = seg[1 : len(seg)-1]
	}
	return seg
}

// clerIsPlainStr 判该段是否为「恰一个 str 常量」并给出其值。
func clerIsPlainStr(seg []clerTok) (string, bool) {
	if len(seg) == 1 && seg[0].kind == clerStr {
		return seg[0].val, true
	}
	return "", false
}

// clerDetect 现读并检出有宽松出口档的脚本。ok=false ⇒ scripts/gates 不存在（py found=None）。
func clerDetect(root string) (found map[string][]string, ok bool, errs []clerErr) {
	d := clerPyJoin(root, "scripts", "gates")
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		return nil, false, nil
	}
	names := clerSortedListdir(d)
	found = map[string][]string{}
	for _, name := range names {
		if !strings.HasSuffix(name, ".py") {
			continue
		}
		p := clerPyJoin(d, name)
		src, rerr := clerReadReplace(p)
		if rerr != nil {
			// py 该档：`open()` 抛 OSError（未捕）⇒ 回溯 ＋ rc=1。本件按 C 类口径 fail-closed rc=2 并声明（见未覆盖面）。
			errs = append(errs, clerErr{name, "read failed: " + rerr.Error()})
			continue
		}
		toks, lerr := clerLex(src)
		if lerr != nil {
			// 逐字复刻 py 的 SyntaxError 正文形态：`<msg> (<unknown>, line <N>)`
			errs = append(errs, clerErr{name, fmt.Sprintf("%s (<unknown>, line %d)", lerr.msg, lerr.line)})
			continue
		}
		hits := clerHits(toks)
		if len(hits) > 0 {
			found[name] = hits
		}
	}
	return found, true, errs
}

// clerHits 复刻 py 对单个文件的 ast.walk：逐 `.add_argument(` 调用取 flag/default/strictTrue。
func clerHits(toks []clerTok) []string {
	set := map[string]bool{}
	for i := 0; i+2 < len(toks); i++ {
		if toks[i].kind != clerOp || toks[i].val != "." {
			continue
		}
		if toks[i+1].kind != clerName || toks[i+1].val != "add_argument" {
			continue
		}
		if toks[i+2].kind != clerOp || toks[i+2].val != "(" {
			continue
		}
		end := clerMatchCall(toks, i+2)
		if end < 0 {
			continue
		}
		flag, defval, strictTrue := "", "", false
		for _, seg := range clerSplitArgs(toks[i+3 : end]) {
			if len(seg) == 0 {
				continue
			}
			seg = clerUnwrapParens(seg)
			if len(seg) >= 2 && seg[0].kind == clerName && seg[1].kind == clerOp && seg[1].val == "=" &&
				(len(seg) < 3 || seg[2].kind != clerOp || seg[2].val != "=") {
				kw := seg[0].val
				vt := clerUnwrapParens(seg[2:])
				if s, isStr := clerIsPlainStr(vt); isStr {
					if kw == "default" {
						defval = s
					}
					if kw == "action" && s == "store_true" {
						strictTrue = true
					}
				}
				continue
			}
			if s, isStr := clerIsPlainStr(seg); isStr && strings.HasPrefix(s, "--") {
				flag = s
			}
		}
		if flag != "" && (defval == "warn" || defval == "report") {
			set[flag] = true
		} else if flag != "" && strictTrue && strings.HasPrefix(flag, clerStrictTruePrefix) {
			set[flag] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type clerErr struct {
	file string
	msg  string
}

// ── 现算 / 输出 ────────────────────────────────────────────────────────────────
func clerRun(root string, asJSON bool, stdout, stderr io.Writer) int {
	found, ok, errs := clerDetect(root)
	if !ok {
		if asJSON {
			fmt.Fprintf(stdout, "{%s: 2, %s: %s}\n", clerJStr("rc"), clerJStr("error"), clerJStr("scripts/gates 不存在"))
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— scripts/gates 不存在 ⇒ rc=2，不给结论")
		}
		return 2
	}
	if len(errs) > 0 {
		if asJSON {
			var b strings.Builder
			b.WriteString("{" + clerJStr("rc") + ": 2, " + clerJStr("errs") + ": [")
			for i, e := range errs {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("{" + clerJStr("file") + ": " + clerJStr(e.file) + ", " + clerJStr("err") + ": " + clerJStr(e.msg) + "}")
			}
			b.WriteString("]}\n")
			io.WriteString(stdout, b.String())
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— 脚本解析失败（取不到 ≠ 无宽松出口）⇒ rc=2：")
			for _, e := range errs {
				fmt.Fprintf(stderr, "    %s ← %s\n", e.file, e.msg)
			}
		}
		return 2
	}
	// C：登记项未给严格旗标 ⇒ rc=2
	badReg := clerBadRegistry()
	if len(badReg) > 0 {
		if asJSON {
			var b strings.Builder
			b.WriteString("{" + clerJStr("rc") + ": 2, " + clerJStr("bad_registry") + ": [")
			for i, s := range badReg {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(clerJStr(s))
			}
			b.WriteString("]}\n")
			io.WriteString(stdout, b.String())
		} else {
			fmt.Fprintf(stderr, "BLOCKED —— 登记项缺「严格旗标」：%s ⇒ rc=2（写不清严格旗标 = 洗白）\n", strings.Join(badReg, "、"))
		}
		return 2
	}
	unregistered := clerUnregistered(found)
	stale := clerStale(root)
	bad := len(unregistered) + len(stale)
	if asJSON {
		var b strings.Builder
		b.WriteString("{" + clerJStr("rc") + fmt.Sprintf(": %d, ", boolToInt(bad > 0)) + clerJStr("detected") + ": {")
		keys := make([]string, 0, len(found))
		for k := range found {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(clerJStr(k) + ": [")
			for j, v := range found[k] {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(clerJStr(v))
			}
			b.WriteString("]")
		}
		b.WriteString("}, " + clerJStr("unregistered") + ": ")
		b.WriteString(clerJStrArr(unregistered))
		b.WriteString(", " + clerJStr("stale") + ": ")
		b.WriteString(clerJStrArr(stale))
		b.WriteString("}\n")
		io.WriteString(stdout, b.String())
	} else {
		fmt.Fprintln(stdout, "「宽松出口档」门脚本统一登记面（GAP-20260927-269）")
		fmt.Fprintf(stdout, "检出有宽松出口档的脚本 = %d 只 · 登记表 = %d 行\n", len(found), len(clerLooseExit))
		fmt.Fprintf(stdout, "── A 漏登记（检出却没进登记表）：%d 处\n", len(unregistered))
		for _, x := range unregistered {
			fmt.Fprintf(stdout, "   ✗ %s（检出宽松旗标：%s）\n", x, strings.Join(found[x], "、"))
		}
		fmt.Fprintf(stdout, "── B 登记陈旧（表里点名、脚本/旗标查无）：%d 处\n", len(stale))
		for _, x := range stale {
			fmt.Fprintf(stdout, "   ✗ %s\n", x)
		}
		if bad > 0 {
			fmt.Fprintf(stdout, "⇒ 违规 %d 处（rc=1）：宽松出口档必须逐条登记（谁有、默认怎么算、严格旗标叫什么）\n", bad)
		} else {
			fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 只脚本的宽松出口档逐条登记 · 严格旗标齐全\n", len(found))
		}
	}
	return boolToInt(bad > 0)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func clerBadRegistry() []string {
	set := map[string]bool{}
	for _, e := range clerLooseExit {
		for _, it := range e.items {
			if it.strict == "" {
				set[e.name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func clerUnregistered(found map[string][]string) []string {
	in := map[string]bool{}
	for _, e := range clerLooseExit {
		in[e.name] = true
	}
	var out []string
	for k := range found {
		if !in[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// clerStale 复刻 py B 档：登记项点名的脚本/旗标字面在盘上查无。
func clerStale(root string) []string {
	entries := append([]clerEntry(nil), clerLooseExit...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	var stale []string
	for _, e := range entries {
		p := clerPyJoin(root, "scripts", "gates", e.name)
		if fi, err := os.Stat(p); err != nil || fi.IsDir() {
			// ★ 退役态安全（类②）：已 Go 真搬（clerGoPorted）的登记件，py 退役（不在盘）属**预期**
			// ⇒ 不判「登记陈旧」；未 Go 化件缺席**仍**判红（牙不放松）。
			if !clerGoPorted[e.name] {
				stale = append(stale, e.name+"（脚本不在盘）")
			}
			continue
		}
		src, _ := clerReadReplace(p)
		for _, it := range e.items {
			if !strings.Contains(src, it.flag) {
				stale = append(stale, e.name+" 的 "+it.flag+"（旗标字面查无）")
			}
		}
	}
	return stale
}

// ── 自检（成对负控 · 仓外夹具 · 不碰真目标）──────────────────────────────────────────
func clerSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintln(stderr, "BLOCKED —— 建夹具失败 ⇒ rc=2")
		return 2
	}
	defer os.RemoveAll(td)
	p := clerPyJoin(td, "scripts", "gates")
	if err := os.MkdirAll(p, 0o755); err != nil {
		fmt.Fprintln(stderr, "BLOCKED —— 建夹具失败 ⇒ rc=2")
		return 2
	}
	clerWriteFile(clerPyJoin(p, "check-doc-meta.py"),
		"import argparse\nap=argparse.ArgumentParser()\n"+
			"ap.add_argument('--missing', choices=['warn','fail'], default='warn')\n")
	saved := clerDeepCopy(clerLooseExit)

	doc := []clerEntry{{"check-doc-meta.py", []clerItem{{"--missing", "warn", "--missing=fail"}}}}

	// ① 正控：检出 default=warn 且已登记 ⇒ rc=0
	clerLooseExit = clerDeepCopy(doc)
	if clerRun(td, false, stdout, stderr) != 0 {
		fmt.Fprintln(stdout, "  ✗ ① 正控失败：已登记应回 0")
		ok = false
	}
	// ② must-fail A：另一脚本 default=report 未登记 ⇒ rc=1
	clerWriteFile(clerPyJoin(p, "check-new-thing.py"),
		"import argparse\nap=argparse.ArgumentParser()\nap.add_argument('--tier', default='report')\n")
	if clerRun(td, false, stdout, stderr) != 1 {
		fmt.Fprintln(stdout, "  ✗ ② A 失败：未登记宽松出口应回 1")
		ok = false
	}
	// ③ must-fail B：登记表点名一个盘上没有的 ⇒ rc=1
	os.Remove(clerPyJoin(p, "check-new-thing.py"))
	reg := clerDeepCopy(doc)
	reg = append(reg, clerEntry{"ghost.py", []clerItem{{"--x", "report", "--x rc"}}})
	clerLooseExit = reg
	if clerRun(td, false, stdout, stderr) != 1 {
		fmt.Fprintln(stdout, "  ✗ ③ B 失败：陈旧登记应回 1")
		ok = false
	}
	// ④ must-fail C：登记项缺严格旗标 ⇒ rc=2
	clerLooseExit = []clerEntry{{"check-doc-meta.py", []clerItem{{"--missing", "warn", ""}}}}
	if clerRun(td, false, stdout, stderr) != 2 {
		fmt.Fprintln(stdout, "  ✗ ④ C 失败：缺严格旗标应回 2")
		ok = false
	}
	clerLooseExit = saved
	if ok {
		fmt.Fprintln(stdout, "自检 PASS（4 格）")
		return 0
	}
	fmt.Fprintln(stdout, "自检 FAIL（4 格）")
	return 1
}

func clerDeepCopy(in []clerEntry) []clerEntry {
	out := make([]clerEntry, len(in))
	for i, e := range in {
		items := make([]clerItem, len(e.items))
		copy(items, e.items)
		out[i] = clerEntry{e.name, items}
	}
	return out
}

// ── 词法/文件/JSON 小工具（全部自写同构 · 前缀 cler）────────────────────────────────────
func clerReadReplace(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(b), "\uFFFD"), nil
}

func clerWriteFile(p, body string) {
	_ = os.WriteFile(p, []byte(body), 0o644)
}

func clerSortedListdir(d string) []string {
	f, err := os.Open(d)
	if err != nil {
		return nil
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// clerPyJoin 照 CPython posixpath.join 的逐段规则（✗ 不用 filepath.Join）。
func clerPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
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

// clerDefaultRoot = py `os.environ.get("ZERG_ROOT", os.getcwd())`。
func clerDefaultRoot() string {
	if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// clerJStr 照 py `json.dumps(..., ensure_ascii=False)` 的字符串转义。
func clerJStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else if r == utf8.RuneError {
				b.WriteString(string(r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func clerJStrArr(xs []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(clerJStr(x))
	}
	b.WriteByte(']')
	return b.String()
}

// ── 入口 ──────────────────────────────────────────────────────────────────────
const clerProg = "check-loose-exit-registry.py"

func clerUsage() string {
	return "usage: " + clerProg + " [-h] [--root ROOT] [--json] [--self-test]"
}

func clerHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, clerUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "「宽松出口档」门脚本统一登记面（GAP-20260927-269）")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help   show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --self-test")
}

func clerArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, clerUsage())
	fmt.Fprintf(stderr, "%s: error: %s\n", clerProg, msg)
	return 2
}

// checkLooseExitRegistryGateMain = py `main()` 的真搬入口（argv 不含程序名）。
func checkLooseExitRegistryGateMain(stdout, stderr io.Writer, argv []string) int {
	root := clerDefaultRoot()
	jsonFlag := false
	selfTest := false
	var extras []string
	longs := []string{"help", "root", "json", "self-test"}
	i := 0
	for i < len(argv) {
		a := argv[i]
		i++
		switch {
		case a == "--":
			extras = append(extras, a)
		case strings.HasPrefix(a, "--"):
			body := a[2:]
			name := body
			val := ""
			hasVal := false
			if k := strings.IndexByte(body, '='); k >= 0 {
				name, val, hasVal = body[:k], body[k+1:], true
			}
			opt, amb := clerMatchLong(longs, name)
			if amb {
				return clerArgErr(stderr, "ambiguous option: --"+name+" could match "+clerAmbCands(longs, name))
			}
			if opt == "" {
				extras = append(extras, a)
				continue
			}
			switch opt {
			case "help":
				clerHelp(stdout)
				return 0
			case "json":
				if hasVal {
					return clerArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonFlag = true
			case "self-test":
				if hasVal {
					return clerArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				selfTest = true
			case "root":
				if hasVal {
					root = val
				} else if i < len(argv) {
					root = argv[i]
					i++
				} else {
					return clerArgErr(stderr, "argument --root: expected one argument")
				}
			}
		case len(a) >= 2 && a[0] == '-':
			if a == "-h" {
				clerHelp(stdout)
				return 0
			}
			extras = append(extras, a)
		default:
			extras = append(extras, a)
		}
	}
	if len(extras) > 0 {
		return clerArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}
	if selfTest {
		return clerSelfTest(stdout, stderr)
	}
	return clerRun(root, jsonFlag, stdout, stderr)
}

// clerMatchLong 复刻 argparse 的「唯一前缀缩写」（allow_abbrev=True）。返回 (canonical, ambiguous)。
func clerMatchLong(longs []string, name string) (string, bool) {
	for _, o := range longs {
		if o == name {
			return o, false
		}
	}
	if name == "" {
		return "", false
	}
	var hits []string
	for _, o := range longs {
		if strings.HasPrefix(o, name) {
			hits = append(hits, o)
		}
	}
	if len(hits) == 1 {
		return hits[0], false
	}
	if len(hits) > 1 {
		return "", true
	}
	return "", false
}

func clerAmbCands(longs []string, name string) string {
	var hits []string
	for _, o := range longs {
		if strings.HasPrefix(o, name) {
			hits = append(hits, "--"+o)
		}
	}
	return strings.Join(hits, ", ")
}
