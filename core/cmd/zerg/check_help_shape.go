// check_help_shape.go —— 判据件 `scripts/gates/check-help-shape.py`（863 行 · 甲案 C 类第 13 枚）
// 的 **Go 真搬**。判 `help` 面的**形状**：族级段闭集 · 条数/行数关系式 + 精确闭集常量 ·
// 末行现算重建 · 危险档面（S4）逐条形状 + 表尾自洽 + 两处同源交叉 · 配置主题面（S5）同面交叉。
//
// 为什么这一枚在甲案下**适格**（旧口径曾把「真跑子进程 / 吃制品」判为不适格）：
//
//	甲案口径 = 判据件可真跑子进程 / 吃制品，**条件 = 输入确定 ⇒ 输出可逐字对拍**。
//	本件跑现役 CLI 制品 `bin/zerg help --all` / `help` / `help dangerous` / `help config`
//	四面 + 读命令树矩阵 `core/cmd/zerg/testdata/cli-matrix.json`（`cases[].command` 首段）。
//	输入两件都确定（制品只读复制锁定 sha256 · 矩阵锁 sha256）⇒ 四面输出逐字确定 ⇒ 可对拍。
//
// 外部交互点（逐行现读 py · 全件只有这些）：
//
//	· `subprocess`（146 行 import · 218 行 `subprocess.run([binz]+argv, capture_output=True,
//	  text=True, stdin=DEVNULL)`）—— 只跑制品四面，不联网、不写盘。
//	· 文件读面：`os.path.isfile` + `open(matrix, encoding="utf-8")`（`run_face` / `matrix_commands`）。
//	· `os.path.abspath(__file__)`（741 行）—— 只用于默认根推导（见声明 (a)）。
//	· 遍历面：`os.walk` / `listdir` / `scandir` / `glob` **命中 0**（现读 `zerg code find`）⇒ 无遍历序问题。
//	· 计时/随机/环境变量面：`time` / `datetime` / `random` / `os.environ` / `socket` / `urllib` / `tempfile`
//	  **命中 0** ⇒ 输出不含时刻/随机 ⇒ **零掩码逐字节直比**。
//
// 出厂声明 (a) **默认根定位差异**：py 的默认根 = `dirname(dirname(dirname(abspath(__file__))))`
// （脚本上溯三级 = 仓根）；Go 入口无 `__file__` 语境 ⇒ 本件默认从**当前工作目录**上溯 ≤6 层、
// 找含 `scripts/gates/` 的目录，找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者读数逐字相同；
// 「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--repo-root`。
//
// 出厂声明 (b) **自指检查 —— 判定为「真」**：本件落 `core/cmd/zerg/`，正落在下列三门的扫描面内：
//
//	· 私有绝对路径门（扫 `core/` 全树 · 命中词 = 三类私有绝对路径前缀）：本件**代码行一处不写**，
//	  注释亦不写私有绝对路径前缀 ⇒ 遗留悬空字面计数 = 0。
//	· error.kind 闭集门（扫 `core/cmd/zerg/*.go` 的 setErr 首参字面量与 JSON kind 键值两形态）：
//	  本件**一处不写** kind 字段（机器面 JSON 无 kind 这一格）⇒ 遗留悬空字面计数 = 0。
//	· 茧壁层级门（扫 `core/cmd/zerg/*.go` 的层级常量赋值与命令树层级标记键值）：本件**一处不写**
//	  层级常量/标记键值 ⇒ 遗留悬空字面计数 = 0。
//	「处理有效」的硬取证 = 同根串影子仓两态（本件在/不在）跑上述三门 ⇒ 输出 sha16 逐字相同（见回执）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkHelpShapeGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 五条都合 / 1 判红 / 2 不给结论（与 py 三码逐字对齐）
//
// 词法面逐处现读：本件**无 `fnmatch` / `glob`**（不适用）。正则六条逐条读，凡 py 用 `\s` / `\S` /
// `\d` 的一律展开为显式 Unicode 类（`\p{Nd}` · `\p{Zs}\p{Zl}\p{Zp}` + 0x1c-0x1f/0x85/0xa0），
// ✗ 不用 Go RE2 的 ASCII 默认口径（见 `chsSpaceCls`）。路径拼接自写 `chsPyJoin`（py `os.path.join`
// 三语义差）、取目录 `chsPyDirname`；`str.splitlines()` / `str.strip()` / `str.split()` 都按 py
// Unicode 语义自写（`chsSplitLines` / `chsPyTrim` / `chsPyIsSpace` / `chsSplitWS`）。
// `str.splitlines()` 未覆盖面：py 的分段规则只覆盖本件会遇到的 `\n` 与 `\r` / `\r\n`（详见件尾未覆盖面）。
// 符号链接面：`isfile` 一律 `os.Stat`（跟随链），✗ 不用 `DirEntry.IsDir()`。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const chsVersion = "1.3.0"

// chsMatrixRel = py `MATRIX_REL = os.path.join("core","cmd","zerg","testdata","cli-matrix.json")`。
var chsMatrixRelParts = []string{"core", "cmd", "zerg", "testdata", "cli-matrix.json"}

// chsProg —— py argparse 的 prog（= 原件基名 · 帮助/报错正文里逐字出现）。
const chsProg = "check-help-shape.py"

// chsSecHeaderPrefix —— py `SEC_HEADER_PREFIX = "族级面（"`。
const chsSecHeaderPrefix = "族级面（"

// 布局常量（源件 `core/cmd/zerg/main.go` 一族名清单段）：段头占 1 行 · 族名每行 8 族。
const (
	chsSectionScaffold = 1
	chsFamilyPerLine   = 8
)

// 危险档面逐条行尾标记（py `DANGER_OPEN_MARK` / `DANGER_CLOSED_MARK`）。
const (
	chsDangerOpenMark   = "← **已开放**"
	chsDangerClosedMark = "← 本版未开放"
)

// ── Unicode 空白面（py `\s` / `str.isspace()` / `str.split()` 的等价类）────────────────
//
//	py `\s`（str 模式）= [ \t\n\r\f\v] ∪ Unicode 空白：额外含 0x1c-0x1f（文件/组/记录/单元分隔符）、
//	0x85（NEL）、0xa0（NBSP）与 Zs/Zl/Zp 三族。展开成显式类（✗ 不用 Go 的 ASCII `\s`）。
const chsSpaceCls = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Zs}\p{Zl}\p{Zp}]`
const chsNonSpaceCls = `[^\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Zs}\p{Zl}\p{Zp}]`

var (
	// py `SEC_HEADER_RE = re.compile(r"^族级面（(\d+) 族 ·.*）:$")`
	chsSecHeaderRE = regexp.MustCompile("^" + chsSecHeaderPrefix + `(\p{Nd}+) 族 ·.*）:$`)
	// py `ROW_RE = re.compile(r"^\s{2}zerg\s+\S")`
	chsRowRE = regexp.MustCompile("^" + chsSpaceCls + "{2}zerg" + chsSpaceCls + "+" + chsNonSpaceCls)
	// py `DANGER_ROW_RE`（命名组 path/level/flags 按序取 1/2/3）
	chsDangerRowRE = regexp.MustCompile("^" + chsSpaceCls + "{2}zerg" + chsSpaceCls +
		`+(?P<path>[a-z][a-z0-9-]*(?:` + chsSpaceCls + `+[a-z][a-z0-9-]*)*)` + chsSpaceCls +
		`+(?P<level>D[23])` + chsSpaceCls + `+(?P<flags>--dry-run · (?:--confirm=<[^>]*> · )?--yes)` + chsSpaceCls + `+`)
	// py `DANGER_TAIL_RE = re.compile(r"^上表 (\d+) 条：\*\*已开放 (\d+) 条\*\*.*\*\*未开放 (\d+) 条\*\*")`
	chsDangerTailRE = regexp.MustCompile(`^上表 (\p{Nd}+) 条：\*\*已开放 (\p{Nd}+) 条\*\*.*\*\*未开放 (\p{Nd}+) 条\*\*`)
	// py `DANGER_CONFIRM_RE = re.compile(r"--confirm=<([^>]*)>")`
	chsDangerConfirmRE = regexp.MustCompile(`--confirm=<([^>]*)>`)
	// py `DANGER_PAREN_RE = re.compile(r"（[^）]*）")`
	chsDangerParenRE = regexp.MustCompile(`（[^）]*）`)
	// py `CFG_CHAIN_RE = re.compile(r"^  \S.*? > .+ > .+$")`
	chsCfgChainRE = regexp.MustCompile("^  " + chsNonSpaceCls + `.*? > .+ > .+$`)
	// py `CFG_LEVEL_RE = re.compile(r"^  (\d+)\. \*\*(.+?)\*\*")`
	chsCfgLevelRE = regexp.MustCompile(`^  (\p{Nd}+)\. \*\*(.+?)\*\*`)
	// py `usage_index` 的行匹配：`^\s{2}zerg\s+([a-z][a-z0-9-]*(?:\s+[a-z][a-z0-9-]*)*)\s`
	chsUsageRE = regexp.MustCompile("^" + chsSpaceCls + "{2}zerg" + chsSpaceCls +
		`+([a-z][a-z0-9-]*(?:` + chsSpaceCls + `+[a-z][a-z0-9-]*)*)` + chsSpaceCls)
)

// ── py 字符串原语（逐处自写 · ✗ 不用 strings.TrimSpace / filepath.Join 一类）──

// chsIsLineBoundary —— py `str.splitlines()` 的行边界集合。
func chsIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// chsSplitLines —— py `str.splitlines()`（keepends=False）：末尾边界不产空尾项，空串 ⇒ []。
func chsSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if chsIsLineBoundary(r) {
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// chsPyIsSpace —— py `str.isspace()`（含 0x1c-0x1f · 0x85 · 0xa0 · Zs/Zl/Zp）。
func chsPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	}
	if r < 0x80 {
		return false
	}
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	if r == 0x85 || r == 0xa0 {
		return true
	}
	if r < 0x1000 {
		return false
	}
	return unicode.Is(unicode.Zs, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

// chsPyTrim —— py `str.strip()`（默认无参 = 去两端 Unicode 空白）。
func chsPyTrim(s string) string { return strings.TrimFunc(s, chsPyIsSpace) }

// chsPyStripChars —— py `s.strip(chars)`（去两端属 chars 的码点）。
func chsPyStripChars(s, chars string) string { return strings.Trim(s, chars) }

// chsSplitWS —— py `str.split()`（无参）：按 Unicode 空白段切、去首尾空白。
func chsSplitWS(s string) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if chsPyIsSpace(r) {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = nil
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// chsFoldNewlines —— py `subprocess(text=True)` / `open(newline=None)` 的 universal newlines。
func chsFoldNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// chsPyJoin —— py `os.path.join`（三语义差全保：吃 `..` 不折 · 后段绝对丢前段 · 空段补尾 `/`）。
func chsPyJoin(a, b string) string {
	if b == "" {
		if a == "" {
			return ""
		}
		return a + "/"
	}
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// chsPyJoinAll —— 逐段 `os.path.join`。
func chsPyJoinAll(parts ...string) string {
	out := ""
	for _, p := range parts {
		out = chsPyJoin(out, p)
	}
	return out
}

// chsPyDirname —— py `os.path.dirname`（与 `filepath.Dir` 在尾斜杠/空串处有别）。
func chsPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	switch {
	case i < 0:
		return ""
	case i == 0:
		return "/"
	default:
		return strings.TrimRight(p[:i], "/")
	}
}

// chsIsFile —— py `os.path.isfile`：跟随符号链接（os.Stat，✗ 非 Lstat/DirEntry.IsDir）。
func chsIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// chsPyStr —— py `str(v)` 的**已覆盖**子集（None / 字符串 / 布尔 / 整数型数值）。
//
//	★ 缺键 vs 显式 null 的逐字对齐靠它：缺键 ⇒ 调用方给默认 `""`（跳过）· 显式 null ⇒ `"None"`。
func chsPyStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// chsPyReprStr —— py `%r` 对 str 的呈现（单引号优先 · 含 `'` 且不含 `"` 时改双引号）。
func chsPyReprStr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if byte(r) == quote && r < 0x80 {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// chsPyReprNum —— py `%r` 对 int / None 的呈现（本件只用这两种）。
func chsPyReprNum(v int, present bool) string {
	if !present {
		return "None"
	}
	return strconv.Itoa(v)
}

// chsPyInt —— py `int(<仅含数字的串>)`：先认 ASCII 快路径，非 ASCII 数字退化为 0（未覆盖面）。
func chsPyInt(s string) (int, bool) {
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// chsFirstRunes —— py `s[:N]`（按码点切）。
func chsFirstRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// ── py json 复刻（ensure_ascii=False · 默认分隔符 `, ` / `: `）──

func chsJSONString(s string) string {
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

func chsJSONListStrs(xs []string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = chsJSONString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func chsJSONListLists(xs [][]string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = chsJSONListStrs(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── 现跑 / 读文件（都不是判据，只负责取数）──

// chsRunFace —— py `run_face(binz, argv)`：⇒ (文本 | "", 取不到的理由)。
func chsRunFace(binz string, argv []string) (string, string) {
	if !chsIsFile(binz) {
		return "", "`" + binz + "` 不在盘"
	}
	cmd := exec.Command(binz, argv...)
	cmd.Stdin = nil // DEVNULL 等价：无输入供给
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Sprintf("现跑 `%s` 退码 %d", strings.Join(argv, " "), ee.ExitCode())
		}
		// py 于此裸回溯（PermissionError 一类 ⇒ 未捕异常 rc=1）：甲案口径改 fail-closed（rc=2）。
		return "", fmt.Sprintf("现跑 `%s` 起不来（py 于此裸回溯 ⇒ fail-closed）", strings.Join(argv, " "))
	}
	stdout := chsFoldNewlines(out.String())
	if chsPyTrim(stdout) == "" {
		return "", fmt.Sprintf("现跑 `%s` 零输出", strings.Join(argv, " "))
	}
	return stdout, ""
}

// ── A 档取数口（缺口 `GAP-20261001-47` · 拆档）：读**落盘的 help 面制品**、不起子进程 ──
// 与 `chsRunFace` 的**唯一**不同：判据只吃文件 ⇒ 结论逐字可复现（与 HEAD 处编译制品解耦）。
var chsFaceArtifactFiles = map[string]string{
	"help --all":     "help-all.txt",
	"help":           "help.txt",
	"help dangerous": "help-dangerous.txt",
	"help config":    "help-config.txt",
}

// chsReadFaceFile —— py `read_face_file(path)`：读一件落盘的 help 面制品 ⇒ (文本 | "", 理由)。空件 ⇒ 判不了（空 ≠ 合法帮助面）。
func chsReadFaceFile(path string) (string, string) {
	if !chsIsFile(path) {
		return "", "help 面制品不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "help 面制品读不了（" + chsOSErrStr(path, err) + "）"
	}
	text := chsFoldNewlines(string(raw))
	if chsPyTrim(text) == "" {
		return "", "help 面制品零输出：" + path
	}
	return text, ""
}

// chsArtCase —— A 档自检小格：读件 + 断言（py `read_face_file(...)[0]/[1]` 取值形态）。
func chsArtCase(read func() (string, string), ok func(text, why string) bool) bool {
	text, why := read()
	return ok(text, why)
}

// chsJSONErrStr —— py `json.decoder.JSONDecodeError` 的 str 逐字复刻（msg: line L column C (char N)）。
func chsJSONErrStr(raw []byte, _ error) string {
	return chsPyJSONErr(raw)
}

// ── py `json` 报错正文的复刻（scanner 只定位错误，不产出值）──

type chsJSONP struct {
	rs  []rune
	i   int
	msg string
	pos int
}

func (p *chsJSONP) fail(msg string, pos int) {
	if p.msg == "" {
		p.msg, p.pos = msg, pos
	}
}

func (p *chsJSONP) skipWS() {
	for p.i < len(p.rs) {
		switch p.rs[p.i] {
		case ' ', '	', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func chsIsHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (p *chsJSONP) strLit() {
	start := p.i
	if p.i >= len(p.rs) || p.rs[p.i] != '"' {
		return
	}
	p.i++
	for p.i < len(p.rs) {
		c := p.rs[p.i]
		if c == '"' {
			p.i++
			return
		}
		if c == '\\' {
			p.i++
			if p.i >= len(p.rs) {
				p.fail("Unterminated string starting at", start)
				return
			}
			switch p.rs[p.i] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				p.i++
			case 'u':
				p.i++
				for k := 0; k < 4; k++ {
					if p.i >= len(p.rs) || !chsIsHexRune(p.rs[p.i]) {
						p.fail(`Invalid \uXXXX escape`, p.i)
						return
					}
					p.i++
				}
			default:
				p.fail(`Invalid \escape`, p.i)
				return
			}
			continue
		}
		if c < 0x20 {
			p.fail("Invalid control character at", p.i)
			return
		}
		p.i++
	}
	p.fail("Unterminated string starting at", start)
}

func (p *chsJSONP) lit(word string, start int) {
	w := []rune(word)
	if p.i+len(w) <= len(p.rs) {
		ok := true
		for k, r := range w {
			if p.rs[p.i+k] != r {
				ok = false
				break
			}
		}
		if ok {
			p.i += len(w)
			return
		}
	}
	p.fail("Expecting value", start)
}

func (p *chsJSONP) number(start int) {
	if p.i < len(p.rs) && p.rs[p.i] == '-' {
		p.i++
	}
	if p.i >= len(p.rs) || p.rs[p.i] < '0' || p.rs[p.i] > '9' {
		p.fail("Expecting value", start)
		return
	}
	if p.rs[p.i] == '0' {
		p.i++
	} else {
		for p.i < len(p.rs) && p.rs[p.i] >= '0' && p.rs[p.i] <= '9' {
			p.i++
		}
	}
	if p.i < len(p.rs) && p.rs[p.i] == '.' {
		p.i++
		if p.i >= len(p.rs) || p.rs[p.i] < '0' || p.rs[p.i] > '9' {
			p.fail("Expecting value", start)
			return
		}
		for p.i < len(p.rs) && p.rs[p.i] >= '0' && p.rs[p.i] <= '9' {
			p.i++
		}
	}
	if p.i < len(p.rs) && (p.rs[p.i] == 'e' || p.rs[p.i] == 'E') {
		p.i++
		if p.i < len(p.rs) && (p.rs[p.i] == '+' || p.rs[p.i] == '-') {
			p.i++
		}
		if p.i >= len(p.rs) || p.rs[p.i] < '0' || p.rs[p.i] > '9' {
			p.fail("Expecting value", start)
			return
		}
		for p.i < len(p.rs) && p.rs[p.i] >= '0' && p.rs[p.i] <= '9' {
			p.i++
		}
	}
}

func (p *chsJSONP) value() {
	if p.msg != "" {
		return
	}
	p.skipWS()
	if p.i >= len(p.rs) {
		p.fail("Expecting value", p.i)
		return
	}
	start := p.i
	switch c := p.rs[p.i]; {
	case c == '{':
		p.object()
	case c == '[':
		p.array()
	case c == '"':
		p.strLit()
	case c == 't':
		p.lit("true", start)
	case c == 'f':
		p.lit("false", start)
	case c == 'n':
		p.lit("null", start)
	case c == '-' || (c >= '0' && c <= '9'):
		p.number(start)
	default:
		p.fail("Expecting value", start)
	}
}

const chsErrPropName = "Expecting property name enclosed in double quotes"

func (p *chsJSONP) object() {
	p.i++ // '{'
	p.skipWS()
	if p.i >= len(p.rs) {
		p.fail(chsErrPropName, p.i)
		return
	}
	if p.rs[p.i] == '}' {
		p.i++
		return
	}
	for {
		if p.i >= len(p.rs) || p.rs[p.i] != '"' {
			p.fail(chsErrPropName, p.i)
			return
		}
		p.strLit()
		if p.msg != "" {
			return
		}
		p.skipWS()
		if p.i >= len(p.rs) || p.rs[p.i] != ':' {
			p.fail("Expecting ':' delimiter", p.i)
			return
		}
		p.i++ // ':'
		p.value()
		if p.msg != "" {
			return
		}
		p.skipWS()
		if p.i >= len(p.rs) {
			p.fail("Expecting ',' delimiter", p.i)
			return
		}
		if p.rs[p.i] == ',' {
			p.i++
			p.skipWS()
			if p.i >= len(p.rs) {
				p.fail(chsErrPropName, p.i)
				return
			}
			continue
		}
		if p.rs[p.i] == '}' {
			p.i++
			return
		}
		p.fail("Expecting ',' delimiter", p.i)
		return
	}
}

func (p *chsJSONP) array() {
	p.i++ // '['
	p.skipWS()
	if p.i >= len(p.rs) {
		p.fail("Expecting value", p.i)
		return
	}
	if p.rs[p.i] == ']' {
		p.i++
		return
	}
	for {
		p.value()
		if p.msg != "" {
			return
		}
		p.skipWS()
		if p.i >= len(p.rs) {
			p.fail("Expecting ',' delimiter", p.i)
			return
		}
		if p.rs[p.i] == ',' {
			p.i++
			continue
		}
		if p.rs[p.i] == ']' {
			p.i++
			return
		}
		p.fail("Expecting ',' delimiter", p.i)
		return
	}
}

// chsPyJSONErr —— py `json.loads` 失败时的正文（合法 JSON 则返回空串）。
func chsPyJSONErr(raw []byte) string {
	p := &chsJSONP{rs: []rune(string(raw))}
	p.skipWS()
	if p.i >= len(p.rs) {
		p.fail("Expecting value", p.i)
	} else {
		p.value()
		if p.msg == "" {
			p.skipWS()
			if p.i < len(p.rs) {
				p.fail("Extra data", p.i)
			}
		}
	}
	if p.msg == "" {
		return ""
	}
	line, lastNL := 1, -1
	for i := 0; i < p.pos && i < len(p.rs); i++ {
		if p.rs[i] == '\n' {
			line++
			lastNL = i
		}
	}
	col := p.pos - lastNL
	return fmt.Sprintf("%s: line %d column %d (char %d)", p.msg, line, col, p.pos)
}

// chsOSErrStr —— py `OSError` 的 str 复刻（常见 errno 档）。
func chsOSErrStr(path string, err error) string {
	switch {
	case os.IsNotExist(err):
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	case os.IsPermission(err):
		return fmt.Sprintf("[Errno 13] Permission denied: '%s'", path)
	}
	return err.Error()
}

// chsMatrixCommands —— py `matrix_commands(path)`：⇒ (去重排序的 command 串 | nil, 理由)。
func chsMatrixCommands(path string) ([]string, string) {
	if !chsIsFile(path) {
		return nil, "命令树矩阵不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("命令树矩阵读不回来（%s）", chsOSErrStr(path, err))
	}
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Sprintf("命令树矩阵读不回来（%s）", chsJSONErrStr(raw, err))
	}
	root, isObj := obj.(map[string]interface{})
	if !isObj {
		// py `obj.get("cases")` 打在非 dict 上 ⇒ AttributeError ⇒ 裸回溯。甲案：fail-closed。
		return nil, "命令树矩阵顶层不是对象（py 于此裸回溯 ⇒ fail-closed）"
	}
	var cases []interface{}
	if v, present := root["cases"]; present && v != nil {
		if arr, isArr := v.([]interface{}); isArr {
			cases = arr
		} else {
			return nil, "命令树矩阵 `cases` 不是数组（py 于此裸回溯 ⇒ fail-closed）"
		}
	}
	set := map[string]bool{}
	for _, c := range cases {
		m, isMap := c.(map[string]interface{})
		if !isMap {
			return nil, "命令树矩阵 `cases` 里有非对象元素（py 于此裸回溯 ⇒ fail-closed）"
		}
		v, present := m["command"]
		s := ""
		if present {
			s = chsPyStr(v) // 缺键 ⇒ ""（默认）· 显式 null ⇒ "None"
		}
		s = chsPyTrim(s)
		if s != "" {
			set[s] = true
		}
	}
	if len(set) == 0 {
		return nil, "命令树矩阵里一条 `command` 都没有（矩阵被改坏 / 换口径了？）"
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, ""
}

// ── 纯函数：形状抽取 ──

func chsRowsOf(text string) []string {
	var out []string
	for _, l := range chsSplitLines(text) {
		if chsRowRE.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// chsFamilySection —— py `family_section(text)`：
// ⇒ (段头行号, 有段头, 括注族数, 有族数, token 表, 段行数, 有段行数)。
func chsFamilySection(text string) (int, bool, int, bool, []string, int, bool) {
	lines := chsSplitLines(text)
	idx := -1
	for i, l := range lines {
		if strings.HasPrefix(l, chsSecHeaderPrefix) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, false, 0, false, nil, 0, false
	}
	var num int
	hasNum := false
	if m := chsSecHeaderRE.FindStringSubmatch(chsPyTrim(lines[idx])); m != nil {
		if n, ok := chsPyInt(m[1]); ok {
			num, hasNum = n, true
		}
	}
	var toks []string
	for _, l := range lines[idx+1:] {
		for _, t := range strings.Split(l, "·") {
			t = chsPyTrim(t)
			if t != "" {
				toks = append(toks, t)
			}
		}
	}
	return idx, true, num, hasNum, toks, len(lines) - idx, true
}

// chsFamiliesOf —— py `families_of(cmds)`：{c.split()[0]} ⇒ 字典序。
func chsFamiliesOf(cmds []string) []string {
	set := map[string]bool{}
	for _, c := range cmds {
		f := chsSplitWS(c)
		if len(f) > 0 {
			set[f[0]] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── 纯函数：危险档面（S4）的形状抽取 ──

func chsDangerRows(text string) []string {
	var out []string
	for _, l := range chsSplitLines(text) {
		if chsRowRE.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// chsDangerTail —— py `danger_tail(text)`：⇒ (原行, N, X, Y, 有数)。
func chsDangerTail(text string) (string, int, int, int, bool) {
	var nonEmpty []string
	for _, l := range chsSplitLines(text) {
		if chsPyTrim(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) == 0 {
		return "", 0, 0, 0, false
	}
	last := nonEmpty[len(nonEmpty)-1]
	m := chsDangerTailRE.FindStringSubmatch(chsPyTrim(last))
	if m == nil {
		return last, 0, 0, 0, false
	}
	n, nok := chsPyInt(m[1])
	x, xok := chsPyInt(m[2])
	y, yok := chsPyInt(m[3])
	if !nok || !xok || !yok {
		return last, 0, 0, 0, false
	}
	return last, n, x, y, true
}

// chsUsageIndex —— py `usage_index(all_text)`：{命令路径: 整行}（取首个命中）。
func chsUsageIndex(allText string) map[string]string {
	out := map[string]string{}
	for _, l := range chsSplitLines(allText) {
		if m := chsUsageRE.FindStringSubmatch(l); m != nil {
			if _, seen := out[m[1]]; !seen {
				out[m[1]] = l
			}
		}
	}
	return out
}

// chsDangerConfirmGap —— py `danger_confirm_gap`：印了 `--confirm=<目标>` 但核心名不在用法串里的名单。
func chsDangerConfirmGap(dangerText, allText string) []string {
	used := chsUsageIndex(allText)
	set := map[string]bool{}
	for _, l := range chsDangerRows(dangerText) {
		m := chsDangerRowRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		c := chsDangerConfirmRE.FindStringSubmatch(m[3])
		if c == nil {
			continue
		}
		core := chsPyTrim(chsPyStripChars(chsDangerParenRE.ReplaceAllString(c[1], ""), "`"))
		if core == "" || !strings.Contains(used[m[1]], core) {
			set[m[1]] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// chsDangerConfirmPairs —— py `danger_confirm_pairs`：两面**都印** `--confirm=` ⇒ [[路径,险面值,用法值]]。
func chsDangerConfirmPairs(dangerText, allText string) [][]string {
	used := chsUsageIndex(allText)
	var pairs [][]string
	for _, l := range chsDangerRows(dangerText) {
		m := chsDangerRowRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		c := chsDangerConfirmRE.FindStringSubmatch(m[3])
		if c == nil {
			continue
		}
		uc := chsDangerConfirmRE.FindStringSubmatch(used[m[1]])
		if uc == nil {
			continue
		}
		pairs = append(pairs, []string{m[1], c[1], uc[1]})
	}
	chsSortPairs(pairs)
	return pairs
}

// chsDangerConfirmValueMismatch —— py `danger_confirm_value_mismatch`：两面值不逐字相等的条目。
func chsDangerConfirmValueMismatch(dangerText, allText string) [][]string {
	var out [][]string
	for _, p := range chsDangerConfirmPairs(dangerText, allText) {
		if p[1] != p[2] {
			out = append(out, p)
		}
	}
	return out
}

// chsSortPairs —— py `sorted(list_of_list_of_str)`（逐元素字典序）。
func chsSortPairs(pairs [][]string) {
	sort.SliceStable(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
}

// ── 纯函数：配置主题面（S5）的形状抽取 ──

func chsCfgChainLines(text string) []string {
	var out []string
	for _, l := range chsSplitLines(text) {
		if chsCfgChainRE.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// chsCfgChainTokens —— py `cfg_chain_tokens`：按 ` > ` 切 · 剥反引号。
func chsCfgChainTokens(line string) []string {
	parts := strings.Split(chsPyTrim(line), " > ")
	out := make([]string, len(parts))
	for i, t := range parts {
		out[i] = strings.ReplaceAll(chsPyTrim(t), "`", "")
	}
	return out
}

// chsCfgLevels —— py `cfg_levels(text)`：⇒ [(编号, 名字)]（名字剥反引号）。
func chsCfgLevels(text string) [][2]interface{} {
	var out [][2]interface{}
	for _, l := range chsSplitLines(text) {
		m := chsCfgLevelRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		n, ok := chsPyInt(m[1])
		if !ok {
			continue
		}
		out = append(out, [2]interface{}{n, strings.ReplaceAll(chsPyTrim(m[2]), "`", "")})
	}
	return out
}

// chsRebuiltLastFamilyLine —— py `rebuilt_last_family_line`：族名清单末行的现算重建。
func chsRebuiltLastFamilyLine(fams []string) string {
	if len(fams) == 0 {
		return "  "
	}
	start := ((len(fams) - 1) / chsFamilyPerLine) * chsFamilyPerLine
	if start > len(fams) {
		start = len(fams)
	}
	return "  " + strings.Join(fams[start:], " · ")
}

// chsEmitBaseline —— py `--emit-baseline` 块的正文。
func chsEmitBaseline(stdout io.Writer, faces map[string]string, order []string) int {
	for _, face := range order {
		text := faces[face]
		last := ""
		if ls := chsSplitLines(text); len(ls) > 0 {
			last = ls[len(ls)-1]
		}
		fmt.Fprintf(stdout, "%s\tlines=%d\tbytes=%d\tlast=%s\n", face, len(chsSplitLines(text)), len(text), last)
	}
	d := faces["help dangerous"]
	drows := chsDangerRows(d)
	tail, _, _, _, _ := chsDangerTail(d)
	fmt.Fprintf(stdout, "  # ↑ 前四行照抄进 BASELINE。危险档面（S4）那一条照抄下面这三行；配置主题面（S5）照抄末行给出的那一条：\n")
	fmt.Fprintf(stdout, "    \"help dangerous\": {\"lines\": %d, \"bytes\": %d, \"rows\": %d,\n"+
		"                       \"tail\": %s,\n"+
		"                       \"confirm_not_in_usage\": %s,\n"+
		"                       \"confirm_value_mismatch\": %s},\n",
		len(chsSplitLines(d)), len(d), len(drows),
		chsPyReprStr(tail),
		chsJSONListStrs(chsDangerConfirmGap(d, faces["help --all"])),
		chsJSONListLists(chsDangerConfirmValueMismatch(d, faces["help --all"])))
	cf := faces["help config"]
	fmt.Fprintf(stdout, "    \"help config\": {\"lines\": %d, \"bytes\": %d},\n",
		len(chsSplitLines(cf)), len(cf))
	return 0
}

// chsCeilDiv —— py `math.ceil(a/b)`。
func chsCeilDiv(a, b int) int {
	if a%b == 0 {
		return a / b
	}
	return a/b + 1
}

func chsStringsEqual(a, b []string) bool {
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

func chsPairsEqual(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !chsStringsEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func chsJoinDot(xs []string) string { return strings.Join(xs, " · ") }

func chsHead5(xs []string) []string {
	if len(xs) > 5 {
		return xs[:5]
	}
	return xs
}

// ── 冻结常量（A 类：现跑取数 · 禁手写；刷新走 py 的 `--emit-baseline`）──

const chsListRules = "判据（三条 · 现算 + 对界 · 缺口 GAP-20260927-401）\n-----------------------------------------------\n  S1 族级段（闭集相等）：现跑 `help --all` 末尾段 token ⟷ 命令树（`cli-matrix.json` 的\n     `cases[].command` 首段）· 段头在 · 括注族数 == 现算族数 · 双向差集空 · 裸 `help` 不带段\n  S2 条数与行数：rows(--all) == rows(help) + rows(help dangerous) ·\n     族级段行数 == 1 + ceil(族数/8)（段头之前必须是空行）；两个面的 **lines == 冻结常量**；\n     **bytes 降级为「观察值 + 归因」**（✗ 不进 rc · 2026-10-14 本轮）—— 字节是**派生量**\n     （随用法串自然增长）⇒ 现读值与「与冻结值的差」逐趟打印，字节不等**不判红**\n  闸A 逐字覆盖（`help --all` 面 ⟷ 命令树源码 `core/cmd/zerg/main.go` 的 `commands[].usage`）：\n     树里每条的 `\"  \" + usage` 必须**逐字**在面里（多行 usage 由子串口径自然覆盖）⇒ 不在即点名：\n     面里有该命令的用法行 ⇒「文本不符」；连行都没有 ⇒「漏印」· 面里以 `zerg <小写词>` 起头的\n     每条用法行必须能归到某条树命令（按**最长命令名**前缀）⇒ 归不到即「多印」· 树取数为 0 ⇒ fail-closed\n  闸B 无重复（`help --all` / 裸 `help`）：两面**不得有重复行、不得有第二份族级段**（段头 `族级面（`）\n     ⇒ rc=1 时逐一指出重复内容与次数\n\n  S3 末行：`--all` 末行 == 由族名清单现算重建的那一行；裸 `help` 末行 == 冻结读数（逐字）\n  S4 危险档面（`help dangerous` · 2026-09-27 补 · 缺口 GAP-20260927-418）：\n     S4a 表尾计数（`上表 N 条：已开放 X / 未开放 Y` ⇒ N == X + Y == 逐条行数）·\n     S4b 逐条行尾标记出现次数 == 表尾 X / Y ·\n     S4c 逐条行形状闭集（`zerg <路径>  D2|D3  --dry-run · [--confirm=<目标> · ]--yes  …  ← 标记`）·\n     S4d 逐条 `--confirm=<目标>` 的核心名 ⟷ 该命令在 `help --all` 的用法串（现算脱钩集合 == 冻结名单）·\n     S4f 两面**同印** `--confirm=` 的值须**逐字相等**（现算值冲突集合 == 冻结名单 · 现读空集 —— 独立一条腿：\n         S4d 只判「名字在不在用法串里」⇒ 值冲突但名字集合不变时 S4d 看不见）·\n     S4e 该面的 lines / bytes / 条数 / 表尾逐字 == 冻结读数（现跑取数 · --emit-baseline 刷）\n  S5 配置主题面（`help config` · 2026-09-27 补 · 缺口 GAP-20260927-440）：\n     S5a 同面交叉：优先级链行现算的级名表 == 逐级说明 `N. **名字**` 的名字表（同序逐字 · 两边剥反引号）·\n         编号必须 `1..N` 连号无重复 · 链行恰好 1 条 ·\n     S5b 同源跨面：`help config` 的链行**逐字等于**裸 `help` 的那一行（同源 = `helptext.go` 的\n         `configPriorityLine` 常量 —— 分家 = 有人抄了第二份）·\n     S5c 该面的 lines / bytes == 冻结读数（现跑取数 · --emit-baseline 刷）\n\n不假装覆盖（照实登记）\n----------------------\n  · 只判**形状**（段 / 条数 / 行数 / 末行）；段的**说明文字**逐字、危险档判词、层级列不在本格\n  · 两态 rc 语义（`zerg <族> --help` 退 2 等）**不在本件**（那是 `cli_help_twostate_test.go` 的面，禁碰）\n  · `help config` 只判**形状/结构**：链 ⟷ 逐级说明同序 · 同源链行逐字 · lines/bytes 常量；\n    「凭据」那批条目的**文字**与主题表 `topics.go` 的 `Brief` 串同源不同源**不在本格**\n    （要另开一处读 Go 源的口径 —— 本单不开第三份口径）\n  · 建件时本件**未挂门**（挂门 + `scripts/公开标记.tsv` + `dryrun-semantics.json` 三处登记是随动口）\n"

const chsFixTail = "上表 5 条：**已开放 1 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 4 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。"
const chsFixChain = "旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认"

var chsFixCmds = []string{"agent ls", "gap ls", "gate run", "help", "version", "watch ls", "agent reap", "agent load", "model start", "task pause", "core restart"}
var chsFixCfgLevels = []string{"旗标", "`ZERG_*` 环境变量", "项目 `.env`", "`~/.zerg/config.yaml`", "内置默认"}
var chsFixDanger = [][5]string{
	{"agent reap", "D3", "机器名", "  zerg agent reap --dry-run --min-age-days <n>   # 干跑单（带 plan_id）", "← 本版未开放"},
	{"agent load", "D2", "机器名", "  zerg agent load <机器名> <模型> --yes [--dry-run]   装模型", "← 本版未开放"},
	{"model start", "D2", "模型 id", "  zerg model start <模型 id> --yes [--dry-run]   装载", "← 本版未开放"},
	{"task pause", "D2", "任务 id", "  zerg task pause <任务 id> --yes [--dry-run]   暂停", "← 本版未开放"},
	{"core restart", "D3", "主机名", "  zerg core restart --host <机器名> --confirm=<主机名> --yes [--dry-run]   停起主控", "← **已开放**（确认档齐就真执行）"},
}

// chsFaceBl —— 一个面的冻结读数（`None`/缺键 用 Has* 三态表达）。
type chsFaceBl struct {
	HasLines, HasBytes, HasRows bool
	Lines, Bytes, Rows          int
	HasLast, HasTail            bool
	Last, Tail                  string
	ConfirmNotInUsage           []string
	ConfirmValueMismatch        [][]string
}

type chsBaselineSet map[string]chsFaceBl

var chsBaseline = chsBaselineSet{
	// ★ 2026-10-17 · `GAP-20261010-20`（`zerg audit hardcoded-addr` 一条新 path）：`help --all` 与裸 `help`
	//   各 +2 行（族内一条新命令 ⇒ 两处各多一行用法行 + 与族级段相关的换行）；字节由现跑
	//   `zerg gate check check-help-shape --emit-baseline` 取数照抄（**禁手写** —— 件头纪律）。
	//   危险档面（`help dangerous`）与 `help config` **不动**：本命令零危险档。
	// ★ 2026-10-17（本枚 · `chat ask` ＋ `chat alias set|ls|rm` 四条新 path）：`help --all` 与裸 `help`
	//   各 +8 行（四条命令 × 各一条用法行 ＋ 族级段/空行随动）；字节由现跑
	//   `zerg gate check check-help-shape --emit-baseline --bin <新制品>` 取数照抄（**禁手写** —— 件头纪律）。
	//   危险档面（`help dangerous`）与 `help config` **不动**：本批四条命令零危险档（别名是本地状态、
	//   `chat ask` 只发一轮对话）。
	// ★ 2026-10-17（本枚 · `GAP-20261001-69` 的前置工具 `zerg code const-dup` 一条新 path）：
	//   `help --all` 与裸 `help` 各 **+2 行**（族内一条新命令 ⇒ 两处各多一行用法行 ＋ 族级段随动）；
	//   字节由现跑 `zerg gate check check-help-shape --emit-baseline` 取数照抄（**禁手写** —— 件头纪律）。
	//   危险档面（`help dangerous` 127 行 / 48993 字节 · 115 条）与 `help config`（21 / 1748）**一分未变**：
	//   本命令零危险档、零配置项 ⇒ 这是「没有顺手放宽」的反证，不是省了没量。
	// ★ 2026-10-18（本枚 · `GAP-20261018-09`：`fs write` 加 `--replace <补丁件>`）：**行数不变**
	//   （464 / 339 —— 只是同一条 usage 串变长，没有新增命令/用法行）；字节由现跑
	//   `zerg gate check check-help-shape --emit-baseline --bin <新制品>` 取数照抄（**禁手写**）：
	//   `help --all` 248949 → 249360（+411）· 裸 `help` 85286 → 85426（+140）。
	//   危险档面（`help dangerous` 127 行 / 48993 字节 · 115 条）与 `help config`（21 / 1748）**一分未变**：
	//   `fs write` 早就在危险档表里、本单只加内容来源旗标 ⇒ 表尾「已开放/未开放」两数不动。
	// ★ 2026-10-18（本枚 · `GAP-20261018-13`：卵面新增 `egg doctor` 一条 path）：`help --all` 与裸 `help`
	//   各 **+2 行**（族内一条新命令 ⇒ 两处各多一行用法行 ＋ 族级段随动）；字节由现跑
	//   `zerg gate check check-help-shape --emit-baseline` 取数照抄（**禁手写** —— 件头纪律）：
	//   `help --all` 464 → 466 行 / 249360 → 249921 字节 · 裸 `help` 339 → 341 行 / 85426 → 85987 字节。
	//   危险档面（`help dangerous` 127 行 / 48993 字节 · 115 条）与 `help config`（21 / 1748）**一分未变**：
	//   `egg doctor` 只读零危险档 ⇒ 这是「没有顺手放宽」的反证，不是省了没量。
	// ★ 2026-10-19（本枚 · `GAP-20261018-20`：`agent deploy` 一条新 path · D3 危险档）：`help --all` 与裸 `help`
	//   各 **+1 行**（族内一条新命令 ⇒ 两处各多一行用法行；族级段 57 族不变）；`help dangerous` **+1 行 ＋ +1 条**
	//   （新命令入危险档表 ⇒ 已开放 102 → 103 · 未开放 13 不变）—— 字节与表尾由现跑
	//   `zerg gate check check-help-shape --emit-baseline` 取数照抄（**禁手写** —— 件头纪律）：
	//   `help --all` 466 → 467 行 / 249921 → 251527 字节 · 裸 `help` 341 行 / 85987 → 86029 字节 ·
	//   `help dangerous` 127 → 128 行 / 48993 → 49360 字节 / rows 115 → 116；`help config`（21 / 1748）**一分未变**。
	// ★ 2026-10-20（本枚 · 批 B1：`trace` 一条新 path · 只读零危险档 · 源件 `core/cmd/zerg-trace`）：
	//   `help --all` 与裸 `help` 各 **+2 行**（一条新命令的用法行 ＋ **新族 `trace` 进族级段** ⇒ 族数 57 → 58）；
	//   字节与行数由现跑 `zerg gate check check-help-shape --emit-baseline --bin <新制品>` 取数照抄
	//   （**禁手写** —— 件头纪律）：`help --all` 467 → 469 行 / 251527 → 252062 字节 · 裸 `help` 341 → 343 行 /
	//   86029 → 86551 字节。★ 照实记一笔：字节面在改前已有 **+54 / +50** 的存量漂（`--emit-baseline` 拿旧制品
	//   `bin/zerg` 现读 251581 / 86079 ⟷ 冻结 251527 / 86029 ⇒ 那 +54/+50 **不是本笔**）；这里照抄的是**现跑真读数**，
	//   不是把它抹平 —— 字节面本就不进 rc（只是观察档），故不替别的改动改账。
	//   危险档面（`help dangerous` 128 行 / 49360 字节 · 116 条）与 `help config`（21 / 1748）**一分未变**：
	//   `trace` 只读、零危险档 ⇒ 这是「没有顺手放宽」的反证，不是省了没量。
	// ★ 2026-10-20（本枚 · 批 B2：`sched run` 一条新 path · 常驻档零危险档 · 源件 `core/cmd/zerg-scheduler`）：
	//   `help --all` 与裸 `help` 各 **+2 行**（族内一条新动作 ⇒ 两处各多一行用法行 ＋ 族级段随动；
	//   族数 58 不变 —— `sched` 是既有族）；字节与行数由现跑
	//   `zerg gate check check-help-shape --emit-baseline --bin <新制品>` 取数照抄（**禁手写** —— 件头纪律）：
	//   `help --all` 469 → 471 行 / 252062 → 252642 字节 · 裸 `help` 343 → 345 行 / 86551 → 87131 字节。
	//   危险档面（`help dangerous` 128 行 / 49360 字节 · 116 条）与 `help config`（21 / 1748）**一分未变**：
	//   `sched run` 起进程、不进危险表（理由在 main.go 那一格的注释里）⇒ 这是「没有顺手放宽」的反证。
	"help --all":     {HasLines: true, Lines: 471, HasBytes: true, Bytes: 252642},
	"help":           {HasLines: true, Lines: 345, HasBytes: true, Bytes: 87131, HasLast: true, Last: "      非交互用 `--token-stdin`；详见 `zerg help credentials`）。"},
	"help dangerous": {HasLines: true, Lines: 128, HasBytes: true, Bytes: 49360, HasRows: true, Rows: 116, HasTail: true, Tail: "上表 116 条：**已开放 103 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 13 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。"},
	"help config":    {HasLines: true, Lines: 21, HasBytes: true, Bytes: 1748},
}

// ── 判据本体（唯一一处 · 纯函数 · 不现跑、不读盘）──

func chsContains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func chsJudge(allText, bareText, dangerText string, cmds []string, bl chsBaselineSet, configText *string) []string {
	bad := []string{}
	fams := chsFamiliesOf(cmds)

	// ---- S1 族级段（闭集相等）----
	idx, hasIdx, num, hasNum, toks, secLines, _ := chsFamilySection(allText)
	if !hasIdx {
		bad = append(bad, "S1 族级段：`help --all` 末尾**没有**族级段（段头 `"+chsSecHeaderPrefix+"` 找不到）—— 族名清单消失（这正是本件要防的病）")
	} else {
		if !hasNum || num != len(fams) {
			bad = append(bad, fmt.Sprintf("S1 族级段：段头括注 %s 族 ≠ 现算命令树族数 %d", chsPyReprNum(num, hasNum), len(fams)))
		}
		var miss, extra []string
		for _, f := range fams {
			if !chsContains(toks, f) {
				miss = append(miss, f)
			}
		}
		for _, t := range toks {
			if !chsContains(fams, t) {
				extra = append(extra, t)
			}
		}
		if len(miss) > 0 {
			bad = append(bad, fmt.Sprintf("S1 族级段：漏了 %d 族（前 5：%s）—— 命令树里有、清单里没有", len(miss), chsJoinDot(chsHead5(miss))))
		}
		if len(extra) > 0 {
			bad = append(bad, fmt.Sprintf("S1 族级段：多了 %d 项（前 5：%s）—— 清单里有、命令树里没有", len(extra), chsJoinDot(chsHead5(extra))))
		}
		if len(miss) == 0 && len(extra) == 0 && !chsStringsEqual(toks, fams) {
			bad = append(bad, "S1 族级段：token 集合相同但**次序不同**（族名清单要按字典序，`help --all` 与命令树的序必须一致）")
		}
		if bareText != "" {
			if _, bHas, _, _, _, _, _ := chsFamilySection(bareText); bHas {
				bad = append(bad, "S1 边界：裸 `help` 里出现了族级段 —— 族名清单**只许**在 `--all` 这一档末尾")
			}
		}
		var prev string
		prevNone := false
		if idx > 0 {
			prev = chsSplitLines(allText)[idx-1]
		} else {
			prevNone = true
		}
		if prevNone || prev != "" {
			pr := chsPyReprStr(prev)
			if prevNone {
				pr = "None"
			}
			bad = append(bad, fmt.Sprintf("S1 族级段：段头之前那一行不是空行（%s）—— 段的形状漂了", pr))
		}
	}

	// ---- S2 条数与行数（关系式 + 常量）----
	rAll, rBare, rDang := len(chsRowsOf(allText)), len(chsRowsOf(bareText)), len(chsRowsOf(dangerText))
	if rAll != rBare+rDang {
		bad = append(bad, fmt.Sprintf("S2 关系式：rows(`help --all`)=%d ≠ rows(`help`)=%d + rows(`help dangerous`)=%d（= %d）",
			rAll, rBare, rDang, rBare+rDang))
	}
	if hasIdx {
		want := chsSectionScaffold + chsCeilDiv(len(fams), chsFamilyPerLine)
		if secLines != want {
			bad = append(bad, fmt.Sprintf("S2 关系式：族级段 %d 行 ≠ 2 + ceil(%d 族 / %d 族一行) = %d",
				secLines, len(fams), chsFamilyPerLine, want))
		}
	}
	for _, face := range []string{"help --all", "help"} {
		text := allText
		if face == "help" {
			text = bareText
		}
		w := bl[face]
		gotL := len(chsSplitLines(text))
		if w.HasLines && gotL != w.Lines {
			bad = append(bad, fmt.Sprintf("S2 常量：`%s` 行数 %d ≠ 冻结读数 %d（形状漂了 —— 取法见件头 `BASELINE` 注释）",
				face, gotL, w.Lines))
		}
		// ★ 字节**降级为观察值**（2026-10-14 · 本轮）：字节是派生量（随用法串自然增长）⇒ 正当改动
		//   也会判红、逼人手刷冻结常量（同一格一晚漂三次的由来）。⇒ 现读值与「与冻结值的差」改由
		//   `chsByteDrift` 打在观察行上，**✗ 不进 `bad`、✗ 不进退码**；真假鉴别力交给闸A/闸B。
	}

	// ---- S3 末行 ----
	allLines := chsSplitLines(allText)
	allLast := ""
	if len(allLines) > 0 {
		allLast = allLines[len(allLines)-1]
	}
	wantLast := chsRebuiltLastFamilyLine(fams)
	if allLast != wantLast {
		bad = append(bad, fmt.Sprintf("S3 末行：`help --all` 末行 %s ≠ 由族名清单现算重建的 %s",
			chsPyReprStr(allLast), chsPyReprStr(wantLast)))
	}
	bareLines := chsSplitLines(bareText)
	bareLast := ""
	if len(bareLines) > 0 {
		bareLast = bareLines[len(bareLines)-1]
	}
	if wb := bl["help"]; wb.HasLast && bareLast != wb.Last {
		bad = append(bad, fmt.Sprintf("S3 末行：裸 `help` 末行 %s ≠ 冻结读数 %s",
			chsPyReprStr(bareLast), chsPyReprStr(wb.Last)))
	}

	// ---- S4 危险档面（`help dangerous` 的形状）----
	dbl := bl["help dangerous"]
	drows := chsDangerRows(dangerText)
	tail, nTotal, nOpen, nClosed, hasTN := chsDangerTail(dangerText)
	if !hasTN {
		bad = append(bad, fmt.Sprintf("S4a 表尾：`help dangerous` 末非空行不是表尾形状（`上表 N 条：**已开放 X 条**…**未开放 Y 条**`）"+
			"—— 现读 %s（表尾被删 / 改字 / 换口径？）", chsPyReprStr(chsFirstRunes(tail, 80))))
	} else {
		if nTotal != nOpen+nClosed {
			bad = append(bad, fmt.Sprintf("S4a 表尾自洽：上表 %d 条 ≠ 已开放 %d + 未开放 %d（= %d）",
				nTotal, nOpen, nClosed, nOpen+nClosed))
		}
		if nTotal != len(drows) {
			bad = append(bad, fmt.Sprintf("S4a 表尾 ⟷ 逐条：表尾称 %d 条 ≠ 逐条行数 %d（两处不许各说各话）", nTotal, len(drows)))
		}
	}
	nOm, nCm := strings.Count(dangerText, chsDangerOpenMark), strings.Count(dangerText, chsDangerClosedMark)
	if hasTN {
		if nOm != nOpen {
			bad = append(bad, fmt.Sprintf("S4b 逐条标记 ⟷ 表尾：`%s` 出现 %d 次 ≠ 表尾「已开放 %d 条」",
				chsDangerOpenMark, nOm, nOpen))
		}
		if nCm != nClosed {
			bad = append(bad, fmt.Sprintf("S4b 逐条标记 ⟷ 表尾：`%s` 出现 %d 次 ≠ 表尾「未开放 %d 条」",
				chsDangerClosedMark, nCm, nClosed))
		}
	}
	if nOm+nCm != len(drows) {
		bad = append(bad, fmt.Sprintf("S4b 逐条标记：逐条行 %d 条，带标记的只有 %d 条（有行没印 ← 已开放 / ← 本版未开放）",
			len(drows), nOm+nCm))
	}
	for i, l := range drows {
		if !chsDangerRowRE.MatchString(l) {
			bad = append(bad, fmt.Sprintf("S4c 逐条行形状：第 %d 条不符（旗标串只许 `--dry-run · [--confirm=<目标> · ]--yes` · "+
				"档位只许 D2/D3）—— %s", i+1, chsPyReprStr(chsFirstRunes(l, 110))))
		}
	}
	gap := chsDangerConfirmGap(dangerText, allText)
	wantGap := append([]string{}, dbl.ConfirmNotInUsage...)
	sort.Strings(wantGap)
	if len(wantGap) > 0 && !chsStringsEqual(gap, wantGap) {
		var newOnes, fixed []string
		for _, p := range gap {
			if !chsContains(wantGap, p) {
				newOnes = append(newOnes, p)
			}
		}
		for _, p := range wantGap {
			if !chsContains(gap, p) {
				fixed = append(fixed, p)
			}
		}
		if len(newOnes) > 0 {
			bad = append(bad, fmt.Sprintf("S4d 旗标串 ⟷ 真实用法串：新增 %d 处脱钩（印了 `--confirm=<目标>`，但该命令在 `help --all` "+
				"的用法串里没有这个目标的核心名）—— %s", len(newOnes), chsJoinDot(chsHead5(newOnes))))
		}
		if len(fixed) > 0 {
			bad = append(bad, fmt.Sprintf("S4d 旗标串 ⟷ 真实用法串：%d 处脱钩**已消失**（缺口真被修了 ⇒ 用 `--emit-baseline` "+
				"刷新冻结名单，别让判据静默漂）—— %s", len(fixed), chsJoinDot(chsHead5(fixed))))
		}
	}
	vm := chsDangerConfirmValueMismatch(dangerText, allText)
	wantVm := append([][]string{}, dbl.ConfirmValueMismatch...)
	chsSortPairs(wantVm)
	if !chsPairsEqual(vm, wantVm) {
		var newOnes, gone [][]string
		for _, x := range vm {
			if !chsPairsHas(wantVm, x) {
				newOnes = append(newOnes, x)
			}
		}
		for _, x := range wantVm {
			if !chsPairsHas(vm, x) {
				gone = append(gone, x)
			}
		}
		if len(newOnes) > 0 {
			parts := make([]string, 0, 5)
			for _, x := range chsHead5Pairs(newOnes) {
				parts = append(parts, fmt.Sprintf("%s（危险面 `%s` ≠ 用法串 `%s`）", x[0], x[1], x[2]))
			}
			bad = append(bad, fmt.Sprintf("S4f 两面同印 `--confirm=` ⟷ 值须逐字相等：新增 %d 处**值冲突**（危险面印的名字 ≠ "+
				"用法串里印的名字 ⇒ 用户照哪面敲都可能被拒）—— %s", len(newOnes), chsJoinDot(parts)))
		}
		if len(gone) > 0 {
			parts := make([]string, 0, 5)
			for _, x := range chsHead5Pairs(gone) {
				parts = append(parts, fmt.Sprintf("%s（危险面 `%s` ⟷ 用法串 `%s`）", x[0], x[1], x[2]))
			}
			bad = append(bad, fmt.Sprintf("S4f 两面同印 `--confirm=` ⟷ 值须逐字相等：%d 处值冲突**已消失**（缺口真被修了 ⇒ 用 "+
				"`--emit-baseline` 刷新冻结值，别让判据静默漂）—— %s", len(gone), chsJoinDot(parts)))
		}
	}
	for _, kk := range []struct {
		key  string
		got  int
		has  bool
		want int
	}{
		{"lines", len(chsSplitLines(dangerText)), dbl.HasLines, dbl.Lines},
		{"bytes", len(dangerText), dbl.HasBytes, dbl.Bytes},
		{"rows", len(drows), dbl.HasRows, dbl.Rows},
	} {
		if kk.has && kk.got != kk.want {
			bad = append(bad, fmt.Sprintf("S4e 常量：`help dangerous` %s %d ≠ 冻结读数 %d（该面漂了 —— 取法见件头 `BASELINE` 注释）",
				kk.key, kk.got, kk.want))
		}
	}
	if dbl.HasTail && tail != dbl.Tail {
		bad = append(bad, fmt.Sprintf("S4e 常量：`help dangerous` 表尾逐字 %s ≠ 冻结读数 %s",
			chsPyReprStr(tail), chsPyReprStr(dbl.Tail)))
	}

	// ---- S5 配置主题面（`help config` 的形状）----
	if configText != nil {
		cb := bl["help config"]
		ct := *configText
		chains := chsCfgChainLines(ct)
		chain := ""
		hasChain := false
		if len(chains) != 1 {
			bad = append(bad, fmt.Sprintf("S5a 优先级链：`help config` 里那条链行现读 %d 条（期望恰好 1 条 —— "+
				"链被删 / 被拆行 / 被抄出第二份？）", len(chains)))
		} else {
			chain, hasChain = chains[0], true
		}
		lv := chsCfgLevels(ct)
		var nums []int
		for _, e := range lv {
			nums = append(nums, e[0].(int))
		}
		sequential := true
		for i, n := range nums {
			if n != i+1 {
				sequential = false
			}
		}
		if !sequential {
			s := "一条都没有"
			if len(nums) > 0 {
				parts := make([]string, len(nums))
				for i, n := range nums {
					parts[i] = strconv.Itoa(n)
				}
				s = chsJoinDot(parts)
			}
			bad = append(bad, fmt.Sprintf("S5a 逐级说明：编号必须是 `1..N` 连号无重复 —— 现读 %s", s))
		}
		if hasChain {
			toks := chsCfgChainTokens(chain)
			var names []string
			for _, e := range lv {
				names = append(names, e[1].(string))
			}
			if !chsStringsEqual(names, toks) {
				bad = append(bad, fmt.Sprintf("S5a 同面交叉：优先级链 %d 级（%s）≠ 逐级说明 %d 条（%s）—— "+
					"同一个面上的两处必须同序逐字相等", len(toks), chsJoinDot(toks), len(names), chsJoinDot(names)))
			}
			var hchains []string
			if bareText != "" {
				hchains = chsCfgChainLines(bareText)
			}
			if len(hchains) != 1 {
				bad = append(bad, fmt.Sprintf("S5b 同源跨面：裸 `help` 里那条同源链行现读 %d 条（期望恰好 1 条 —— "+
					"真源常量 `configPriorityLine` 的那一行只能有一处出口）", len(hchains)))
			} else if hchains[0] != chain {
				bad = append(bad, fmt.Sprintf("S5b 同源跨面：`help config` 的链行 %s ≠ 裸 `help` 的链行 %s（两处同源 "+
					"= `helptext.go` 的 `configPriorityLine` 常量 ⇒ 字面必须逐字相等；"+
					"分家 = 有人把链抄成了第二份）", chsPyReprStr(chain), chsPyReprStr(hchains[0])))
			}
		}
		for _, kk := range []struct {
			key  string
			got  int
			has  bool
			want int
		}{
			{"lines", len(chsSplitLines(ct)), cb.HasLines, cb.Lines},
			{"bytes", len(ct), cb.HasBytes, cb.Bytes},
		} {
			if kk.has && kk.got != kk.want {
				bad = append(bad, fmt.Sprintf("S5c 常量：`help config` %s %d ≠ 冻结读数 %d（该面漂了 —— "+
					"取法见件头 `BASELINE` 注释 · `--emit-baseline` 刷）", kk.key, kk.got, kk.want))
			}
		}
	}
	return bad
}

func chsPairsHas(xs [][]string, v []string) bool {
	for _, x := range xs {
		if chsStringsEqual(x, v) {
			return true
		}
	}
	return false
}

func chsHead5Pairs(xs [][]string) [][]string {
	if len(xs) > 5 {
		return xs[:5]
	}
	return xs
}

// ── 闸A【逐字覆盖】/ 闸B【无重复】/ 字节面观察（2026-10-14 · 本轮加）──────────────────
//
// 病灶（现读拄证）：S2 曾把 `help --all` / 裸 `help` 的**字节数**冻成常量 ⇒ 字节是**派生量**
// （随用法串自然增长）⇒ 正当改动也判红、逼人手刷冻结常量（同一格几小时内漂
// 221239 → 222763 → 222814 三次）。⇒ 本轮把字节**降级为观察值 + 归因**（✗ 不进 rc），真假
// 鉴别力交给下面两条闸：闸A 判「用法串对不对」（逐字覆盖），闸B 判「面里有没有重影」。
//
// 闸A 的第二份真源 = 命令树源码的 `usage` 串（口径照 `gen_cli_reference.go` 的
// `gcrTreeFromSource`／`gcrConstUsages`）；★ 与它的一处分别：本处认 `\"` 转义 ——
// `usage: "… [{\"old\":…}] …"` 那类字面量在 `[^"]*` 口径下会被截断（闸A 会误报「文本不符」）。
var (
	chsTreeMainRel  = []string{"core", "cmd", "zerg", "main.go"}
	chsTreeHeadLit  = "commands = []*command{"
	chsTreeTailLit  = "\n\t}\n"
	chsPathRE       = regexp.MustCompile(`path:\s*\[\]string\{([^}]*)\}`)
	chsPathPartRE   = regexp.MustCompile(`"([^"]+)"`)
	chsUsageLitRE   = regexp.MustCompile(`usage:\s*"((?:\\.|[^"\\])*)"`)
	chsUsageIdentRE = regexp.MustCompile(`usage:\s*([A-Za-z_][A-Za-z0-9_]*)`)
	chsConstLitRE   = regexp.MustCompile(`(?m)^[ \t]*(?:const[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*:?=[ \t]*"((?:\\.|[^"\\])*)"[ \t]*$`)
)

// chsGoUnescape —— Go 字符串字面量转义还原（本件只遇到 `\"` / `\\` / `\n` / `\t` / `\uXXXX`）。
func chsGoUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\\' || i+1 >= len(rs) {
			b.WriteRune(rs[i])
			continue
		}
		i++
		switch rs[i] {
		case 'n':
			b.WriteRune('\n')
		case 't':
			b.WriteRune('\t')
		case 'r':
			b.WriteRune('\r')
		case '"':
			b.WriteRune('"')
		case '\'':
			b.WriteRune('\'')
		case '\\':
			b.WriteRune('\\')
		case 'u':
			if i+4 < len(rs) {
				if n, err := strconv.ParseInt(string(rs[i+1:i+5]), 16, 32); err == nil {
					b.WriteRune(rune(n))
					i += 4
					continue
				}
			}
			b.WriteRune('u')
		default:
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

// chsConstUsages —— 包内「名字 = "字面量"」表（首见为准 · 遍历序 = 排序后的 *.go · 口径同
// `gen_cli_reference.go` 的 `gcrConstUsages`）。
func chsConstUsages(pkgDir string) map[string]string {
	out := map[string]string{}
	ents, err := os.ReadDir(pkgDir)
	if err != nil {
		return out
	}
	var files []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, n := range files {
		raw, err := os.ReadFile(chsPyJoin(pkgDir, n))
		if err != nil {
			continue
		}
		for _, m := range chsConstLitRE.FindAllStringSubmatch(string(raw), -1) {
			if _, seen := out[m[1]]; !seen {
				out[m[1]] = chsGoUnescape(m[2])
			}
		}
	}
	return out
}

// chsTreeUsage —— 命令树源码（`core/cmd/zerg/main.go` 的 `commands` 切片）⇒ [(命令名, usage 串)]。
// ✗ 取不到（件不在盘 / 切片头尾标记找不到 / 一条都没有）⇒ (nil, 理由) ⇒ 调用方 fail-closed。
func chsTreeUsage(root string) ([][2]string, string) {
	mainPath := chsPyJoinAll(append([]string{root}, chsTreeMainRel...)...)
	if !chsIsFile(mainPath) {
		return nil, "命令树源码不在盘：" + mainPath
	}
	raw, err := os.ReadFile(mainPath)
	if err != nil {
		return nil, "命令树源码读不回来（" + chsOSErrStr(mainPath, err) + "）"
	}
	src := string(raw)
	start := strings.Index(src, chsTreeHeadLit)
	if start < 0 {
		return nil, "命令树源码里找不到切片头 `" + chsTreeHeadLit + "`（口径漂了）"
	}
	rel := strings.Index(src[start:], chsTreeTailLit)
	if rel < 0 {
		return nil, "命令树源码里找不到 `commands` 切片的收尾标记（口径漂了）"
	}
	seg := src[start : start+rel]
	consts := chsConstUsages(chsPyDirname(mainPath))
	matches := chsPathRE.FindAllStringSubmatchIndex(seg, -1)
	var out [][2]string
	for i, m := range matches {
		var parts []string
		for _, p := range chsPathPartRE.FindAllStringSubmatch(seg[m[2]:m[3]], -1) {
			parts = append(parts, p[1])
		}
		if len(parts) == 0 {
			continue
		}
		nxt := len(seg)
		if i+1 < len(matches) {
			nxt = matches[i+1][0]
		}
		block := seg[m[1]:nxt]
		usage := ""
		if u := chsUsageLitRE.FindStringSubmatch(block); u != nil {
			usage = chsGoUnescape(u[1])
		} else if id := chsUsageIdentRE.FindStringSubmatch(block); id != nil {
			usage = consts[id[1]]
		}
		out = append(out, [2]string{strings.Join(parts, " "), usage})
	}
	if len(out) == 0 {
		return nil, "命令树源码里一条 `path:` 都没有（口径漂了？）"
	}
	return out, ""
}

// chsTreeNamesLongestFirst —— 命令名按**词数**降序（多印归属用：最长者先认）。
func chsTreeNamesLongestFirst(tree [][2]string) []string {
	names := make([]string, 0, len(tree))
	for _, e := range tree {
		names = append(names, e[0])
	}
	sort.SliceStable(names, func(i, j int) bool { return len(chsSplitWS(names[i])) > len(chsSplitWS(names[j])) })
	return names
}

// chsFaceUsageRowFor —— 面里「该命令自己的用法行」首见原文（判词点名用 · 无则空串）。
func chsFaceUsageRowFor(text, name string) string {
	for _, l := range chsSplitLines(text) {
		s := chsPyTrim(l)
		if s == "zerg "+name || strings.HasPrefix(s, "zerg "+name+" ") {
			return s
		}
	}
	return ""
}

// chsUsageCover —— 闸A【逐字覆盖】（`help --all` 面 ⟷ 命令树源码的 `usage` 串）：
//
//	① 树里每条的 `"  " + usage` 必须**逐字**在面里（多行 usage 由子串口径自然覆盖）⇒ 不在即点名：
//	   面里有该命令的用法行 ⇒「文本不符」；连行都没有 ⇒「漏印」。
//	② 面里以 `zerg <小写词>` 起头的每条用法行必须能归到某条树命令（按**最长命令名**前缀）
//	   ⇒ 归不到：「多印」（面里有 · 命令树里没有这条命令）。
//
// ★ 树空（取数失败 / 抽空）⇒ fail-closed 判红（✗ 不许当「面里也没问题」）。
func chsUsageCover(allText string, tree [][2]string) []string {
	bad := []string{}
	if len(tree) == 0 {
		bad = append(bad, "闸A 逐字覆盖：命令树 usage 取数为 0 条（抽空 ⇒ fail-closed · ✗ 不许当「面里也没问题」）")
		return bad
	}
	var miss, wrong []string
	for _, e := range tree {
		name, usage := e[0], e[1]
		if usage == "" {
			wrong = append(wrong, name+"（树里这条没有 usage 串）")
			continue
		}
		if strings.Contains(allText, "  "+usage) {
			continue
		}
		if row := chsFaceUsageRowFor(allText, name); row != "" {
			wrong = append(wrong, fmt.Sprintf("%s（树 `%s` ≠ 面 `%s`）", name, usage, chsFirstRunes(row, 110)))
			continue
		}
		miss = append(miss, name)
	}
	if len(miss) > 0 {
		bad = append(bad, fmt.Sprintf("闸A 逐字覆盖：命令树里 %d 条命令在 `help --all` 面**漏印**（树里有 · 面上没有逐字用法串）—— %s",
			len(miss), chsJoinDot(chsHead5(miss))))
	}
	if len(wrong) > 0 {
		bad = append(bad, fmt.Sprintf("闸A 逐字覆盖：%d 条命令的用法串**文本不符**（面上那一行的字 ≠ 树里 `usage` 逐字）—— %s",
			len(wrong), chsJoinDot(chsHead5(wrong))))
	}
	names := chsTreeNamesLongestFirst(tree)
	var extra []string
	for _, l := range chsSplitLines(allText) {
		s := chsPyTrim(l)
		if !strings.HasPrefix(s, "zerg ") || len(s) < 6 {
			continue
		}
		if c := s[5]; c < 'a' || c > 'z' {
			continue // `zerg <对象> <动作> …` 一类占位行不属于任何命令
		}
		hit := false
		for _, n := range names {
			if s == "zerg "+n || strings.HasPrefix(s, "zerg "+n+" ") {
				hit = true
				break
			}
		}
		if !hit {
			extra = append(extra, chsFirstRunes(s, 110))
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		bad = append(bad, fmt.Sprintf("闸A 逐字覆盖：`help --all` 面里 %d 条用法行**多印**（面里有 · 命令树里没有这条命令）—— %s",
			len(extra), chsJoinDot(chsHead5(extra))))
	}
	return bad
}

// chsDupScan —— 闸B【无重复】：面里① 非空行不得重复 ② 族级段段头不得出现第二次（= 第二份族级段）。
// ⇒ rc=1 时逐一指出重复内容与次数。
func chsDupScan(face, text string) []string {
	bad := []string{}
	counts := map[string]int{}
	for _, l := range chsSplitLines(text) {
		if chsPyTrim(l) == "" {
			continue // 空行天然重复 ⇒ ✗ 不入账
		}
		counts[l]++
	}
	var dups []string
	for l, n := range counts {
		if n > 1 {
			dups = append(dups, fmt.Sprintf("`%s`（%d 次）", chsFirstRunes(l, 110), n))
		}
	}
	if len(dups) > 0 {
		sort.Strings(dups)
		bad = append(bad, fmt.Sprintf("闸B 无重复：`%s` 里 %d 条重复行（逐条各指一次出现次数）—— %s",
			face, len(dups), chsJoinDot(chsHead5(dups))))
	}
	nHdr := 0
	for _, l := range chsSplitLines(text) {
		if strings.HasPrefix(l, chsSecHeaderPrefix) {
			nHdr++
		}
	}
	if nHdr > 1 {
		bad = append(bad, fmt.Sprintf("闸B 无重复：`%s` 里族级段段头（`%s`）出现 %d 次 —— 族级段只许一份",
			face, chsSecHeaderPrefix, nHdr))
	}
	return bad
}

// chsByteDrift —— 字节面「观察值 + 归因」（✗ 不进 rc）：现读字节 · 冻结读数 · 两者之差。
func chsByteDrift(pairs [][2]string, bl chsBaselineSet) []string {
	out := []string{}
	for _, p := range pairs {
		face, text := p[0], p[1]
		w := bl[face]
		if !w.HasBytes {
			continue
		}
		got := len(text)
		linePart := fmt.Sprintf("行数 %d", len(chsSplitLines(text)))
		if w.HasLines {
			linePart = fmt.Sprintf("行数 %d ⟷ 冻结 %d", len(chsSplitLines(text)), w.Lines)
		}
		out = append(out, fmt.Sprintf("字节观察（✗ 不进 rc）：`%s` 现读 %d ⟷ 冻结 %d（差 %+d）· %s",
			face, got, w.Bytes, got-w.Bytes, linePart))
	}
	return out
}

// chsAnyHasSub —— 一片里有没有含该子串的一项（自检断言用）。
func chsAnyHasSub(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// ── 夹具（合成 · 不碰真目标）──

func chsFixDangerRows() []string {
	rows := make([]string, 0, len(chsFixDanger))
	for _, t := range chsFixDanger {
		rows = append(rows, "  zerg "+t[0]+"  "+t[1]+"  --dry-run · --confirm=<"+t[2]+"> · --yes  它会动什么  [出处]  "+t[4])
	}
	return rows
}

func chsFixConfigText() string {
	body := []string{"配置优先级（契约 §八 · §4.1 `K10` · clig.dev 五级序）：", "",
		"  " + chsFixChain, "", "逐级说明（左高右低，高者覆盖低者）："}
	for i, n := range chsFixCfgLevels {
		body = append(body, fmt.Sprintf("  %d. **%s**  夹具第 %d 级", i+1, n, i+1))
	}
	body = append(body, "", "凭据（夹具）：", "  · 夹具凭据一", "  · 夹具凭据二")
	return strings.Join(body, "\n") + "\n"
}

// chsFixTree —— 夹具命令树（[命令名, usage 串]）：夹具 `help --all` 面的用法行**逐字由它生成**
// （闸A 的正控要「面 ⟷ 树」逐字相等 ⇒ 夹具只留**一份**来源，✗ 不许两处各写一遍）。
var chsFixTree = chsFixTreeOf()

func chsFixTreeOf() [][2]string {
	out := [][2]string{
		{"agent ls", "zerg agent ls ｜ 茧壁:host ｜ 说明"},
		{"gap ls", "zerg gap ls ｜ 茧壁:host ｜ 说明"},
		{"gate run", "zerg gate run ｜ 茧壁:host ｜ 说明"},
		{"help", "zerg help ｜ 茧壁:host ｜ 说明"},
		{"version", "zerg version ｜ 茧壁:host ｜ 说明"},
	}
	// 危险档那几条的用法串 = 面里那几行**去缩进**（同一份来源 · 见 chsFixDanger 的第四格）。
	for _, t := range chsFixDanger {
		out = append(out, [2]string{t[0], chsPyTrim(t[3])})
	}
	return out
}

func chsFixTexts() (string, string, string, string) {
	fams := chsFamiliesOf(chsFixCmds)
	var rowsBare []string
	for _, e := range chsFixTree[:5] {
		rowsBare = append(rowsBare, "  "+e[1])
	}
	rowsAll := append([]string{}, rowsBare...)
	for _, e := range chsFixTree[5:] {
		rowsAll = append(rowsAll, "  "+e[1])
	}
	var sb strings.Builder
	sb.WriteString("zerg —— 形状夹具 · 深度 ≤ 3 层\n\n")
	sb.WriteString(strings.Join(rowsAll, "\n"))
	sb.WriteString(fmt.Sprintf("\n\n族级面（%d 族 · 不必猜族名 · 逐族 `zerg help <族名>` 列该族全部动作）:\n", len(fams)))
	var chunks []string
	for i := 0; i < len(fams); i += chsFamilyPerLine {
		end := i + chsFamilyPerLine
		if end > len(fams) {
			end = len(fams)
		}
		chunks = append(chunks, "  "+strings.Join(fams[i:end], " · "))
	}
	sb.WriteString(strings.Join(chunks, "\n"))
	sb.WriteString("\n")
	allText := sb.String()

	bareText := "zerg —— 形状夹具\n\n" + strings.Join(rowsBare, "\n") +
		"\n配置优先级:\n  " + chsFixChain + "\n" + chsBaseline["help"].Last + "\n"
	dangText := strings.Join(chsFixDangerRows(), "\n") + "\n\n" + chsFixTail + "\n"
	return allText, bareText, dangText, chsFixConfigText()
}

var chsCaseNames = []string{
	"正控：原样 ⇒ 必绿",
	"负控⑤：改一条的 `--confirm` 目标（等长换字 · 字节不变）⇒ 必红",
	"负控⑥：表尾计数对不上（等长改数）⇒ 必红",
	"负控⑦：删掉表尾一行 ⇒ 必红",
	"负控⑧：抹掉一条的 `--yes`（旗标串破形）⇒ 必红",
	"负控⑨：两面同印 `--confirm=` 但值不等（等长换字 · 名字仍在用法串里 ⇒ S4d/字节常量都看不见）⇒ S4f 必红",
	"负控⑩：危险面值挪出用法串（名字集合变化 = S4d 的账）⇒ 旧 S4d 仍必红、S4f 不许抢账",
	"对界：夹具正控上 S4f 不许误报（两面同印的值逐字相等 · 现读 1 条）⇒ 绿且无 S4f 断言",
	"正控②：配置面原样（链 ⟷ 逐级说明同序 · 与裸 `help` 同源）⇒ 必绿且不许有 S5 断言",
	"负控⑪：只改链行一个字（`内置默认`→`内置缺省` · **等长换字** ⇒ 字节常量看不见）⇒ S5a/S5b 必红、S5c 不许响",
	"负控⑫：只改**裸 `help`**那条同源链行（配置面一字不动）⇒ S5b 必红、S5a 不许抢账",
	"负控⑬：配置面掉一条逐级说明 ⇒ S5a 必红（编号连号 ⟷ 链级数）",
	"对界：配置面喂**空串** ⇒ 必红（证明「这个面没了」不是静默绿）",
	"负控①：删掉族级段 ⇒ 必红",
	"负控②a：族级段掉一行 ⇒ 必红",
	"负控②b：整面多一行 ⇒ 必红",
	"负控③a：`--all` 末行改写 ⇒ 必红",
	"负控③b：裸 `help` 末行改写 ⇒ 必红",
	"负控④：清单漏一族 ⇒ 必红",
	"对界：命令树多一族 ⇒ 必红",
	"负控⑭：抹掉一条危险档行的**逐条标记**（行还在 · 标记没了）⇒ S4b 必红",
	// ── 2026-10-14 本轮加：闸A / 闸B / 字节降级（各配成对负控）──
	"闸A 正控：面 ⟷ 命令树逐字（夹具原样）⇒ 必绿且不许有 闸A 断言",
	"闸A 负控：面上改一个命令名（`agent ls`→`agent lsx`）⇒ 必须点名「漏印」",
	"闸A 负控：面上改一条用法串（`gate run` 那行）⇒ 必须点名「文本不符」",
	"闸A 负控：面上多印一条命令（`zzz ls`）⇒ 必须点名「多印」",
	"闸A 抽空：命令树喂 nil（取数失败）⇒ fail-closed 必红（✗ 不许静默绿）",
	"闸B 正控：两面（`--all` / 裸 `help`）无重复行 ⇒ 必绿且不许有 闸B 断言",
	"闸B 负控：面里多加一条**重复行**（`version` 那行）⇒ 必须点名重复内容与次数",
	"闸B 负控：面里多印**第二份族级段**段头 ⇒ 必须点名「族级段」",
}

// chsCase —— py `self_test` 内 `case(...)` 的复刻。
func chsCase(out io.Writer, total, failed *int, name string, wantRed bool, bad []string, wantSub, forbidSub string) {
	*total++
	gotRed := len(bad) > 0
	ok := gotRed == wantRed
	if ok && wantSub != "" {
		found := false
		for _, b := range bad {
			if strings.Contains(b, wantSub) {
				found = true
				break
			}
		}
		if !found {
			ok = false
		}
	}
	if ok && forbidSub != "" {
		for _, b := range bad {
			if strings.Contains(b, forbidSub) {
				ok = false
				break
			}
		}
	}
	if !ok {
		*failed++
	}
	mark := "✓"
	if !ok {
		mark = "✗"
	}
	redGreen := func(r bool) string {
		if r {
			return "红"
		}
		return "绿"
	}
	fmt.Fprintf(out, "  %s %s（期望%s · 实得%s）\n", mark, name, redGreen(wantRed), redGreen(gotRed))
	lim := len(bad)
	if lim > 3 {
		lim = 3
	}
	for _, b := range bad[:lim] {
		fmt.Fprintf(out, "      ↳ %s\n", b)
	}
}

// chsSelfTest —— py `self_test(out)` 的复刻 ＋ 本轮新增的闸A/闸B/字节降级格
// （35 格 · 正控 + 负控 + 对界 + 闸A/闸B 成对负控 + 字节降级三格 + A 档取数口）。
func chsSelfTest(out io.Writer) bool {
	total, failed := 0, 0
	allT, bareT, dangT, cfgT := chsFixTexts()
	dRows := chsDangerRows(dangT)
	bl := chsBaselineSet{
		"help --all":  {HasLines: true, Lines: len(chsSplitLines(allT)), HasBytes: true, Bytes: len(allT)},
		"help":        {HasLines: true, Lines: len(chsSplitLines(bareT)), HasBytes: true, Bytes: len(bareT), HasLast: true, Last: chsBaseline["help"].Last},
		"help config": {HasLines: true, Lines: len(chsSplitLines(cfgT)), HasBytes: true, Bytes: len(cfgT)},
		"help dangerous": {HasLines: true, Lines: len(chsSplitLines(dangT)), HasBytes: true, Bytes: len(dangT),
			HasRows: true, Rows: len(dRows), HasTail: true, Tail: chsFixTail,
			ConfirmNotInUsage: []string{"agent reap"}, ConfirmValueMismatch: [][]string{}},
	}
	names := chsCaseNames
	c := func(i int, wantRed bool, bad []string, wantSub, forbidSub string) {
		chsCase(out, &total, &failed, names[i], wantRed, bad, wantSub, forbidSub)
	}
	c(0, false, chsJudge(allT, bareT, dangT, chsFixCmds, bl, nil), "", "")
	c(1, true, chsJudge(allT, bareT, strings.ReplaceAll(dangT, "--confirm=<模型 id>", "--confirm=<主机 id>"), chsFixCmds, bl, nil), "", "")
	c(2, true, chsJudge(allT, bareT, strings.ReplaceAll(dangT, "上表 5 条", "上表 6 条"), chsFixCmds, bl, nil), "", "")
	dl := chsSplitLines(dangT)
	c(3, true, chsJudge(allT, bareT, strings.Join(dl[:len(dl)-2], "\n")+"\n", chsFixCmds, bl, nil), "", "")
	c(4, true, chsJudge(allT, bareT, strings.ReplaceAll(dangT, "--dry-run · --confirm=<主机名> · --yes", "--confirm=<主机名> --yes"), chsFixCmds, bl, nil), "", "")
	c(5, true, chsJudge(allT, bareT, strings.ReplaceAll(dangT, "zerg core restart  D3  --dry-run · --confirm=<主机名>", "zerg core restart  D3  --dry-run · --confirm=<机器名>"), chsFixCmds, bl, nil), "S4f", "S4d")
	c(6, true, chsJudge(allT, bareT, strings.ReplaceAll(dangT, "zerg agent load  D2  --dry-run · --confirm=<机器名>", "zerg agent load  D2  --dry-run · --confirm=<主机名>"), chsFixCmds, bl, nil), "S4d", "S4f")
	c(7, false, chsJudge(allT, bareT, dangT, chsFixCmds, bl, nil), "", "S4f")
	cfg := cfgT
	c(8, false, chsJudge(allT, bareT, dangT, chsFixCmds, bl, &cfg), "", "S5")
	badChain := "  " + strings.ReplaceAll(chsFixChain, "内置默认", "内置缺省")
	cfgChainMut := strings.ReplaceAll(cfgT, "  "+chsFixChain, badChain)
	c(9, true, chsJudge(allT, bareT, dangT, chsFixCmds, bl, &cfgChainMut), "S5a", "S5c")
	bareChainMut := strings.ReplaceAll(bareT, chsFixChain, strings.ReplaceAll(chsFixChain, "内置默认", "内置缺省"))
	c(10, true, chsJudge(allT, bareChainMut, dangT, chsFixCmds, bl, &cfg), "S5b", "S5a")
	cfgLines := chsSplitLines(cfgT)
	cfgDrop := strings.Join(append(append([]string{}, cfgLines[:5]...), cfgLines[6:]...), "\n") + "\n"
	c(11, true, chsJudge(allT, bareT, dangT, chsFixCmds, bl, &cfgDrop), "S5a", "")
	empty := ""
	c(12, true, chsJudge(allT, bareT, dangT, chsFixCmds, bl, &empty), "S5a", "")

	lines := chsSplitLines(allT)
	idx := 0
	for i, l := range lines {
		if strings.HasPrefix(l, chsSecHeaderPrefix) {
			idx = i
			break
		}
	}
	c(13, true, chsJudge(strings.Join(lines[:idx-1], "\n")+"\n", bareT, dangT, chsFixCmds, bl, nil), "", "")
	c(14, true, chsJudge(strings.Join(append(append([]string{}, lines[:idx+2]...), lines[idx+3:]...), "\n")+"\n", bareT, dangT, chsFixCmds, bl, nil), "", "")
	c(15, true, chsJudge(allT+"多余一行\n", bareT, dangT, chsFixCmds, bl, nil), "", "")
	c(16, true, chsJudge(strings.Join(append(append([]string{}, lines[:len(lines)-1]...), "  task · ui · update · version · watch-XX"), "\n")+"\n", bareT, dangT, chsFixCmds, bl, nil), "", "")
	bareLines := chsSplitLines(bareT)
	c(17, true, chsJudge(allT, strings.Join(append(append([]string{}, bareLines[:len(bareLines)-1]...), "凭据：改过的一句。"), "\n")+"\n", dangT, chsFixCmds, bl, nil), "", "")
	c(18, true, chsJudge(strings.ReplaceAll(allT, "watch", "watchx"), bareT, dangT, chsFixCmds, bl, nil), "", "")
	cmdsMore := append(append([]string{}, chsFixCmds...), "zzz ls")
	c(19, true, chsJudge(allT, bareT, dangT, cmdsMore, bl, nil), "", "")
	c(20, true, chsJudge(allT, bareT, strings.Replace(dangT, chsDangerOpenMark, "", 1), chsFixCmds, bl, nil), "S4b", "")

	// ── 闸A【逐字覆盖】/ 闸B【无重复】/ 字节降级（2026-10-14 · 本轮加：成对负控）──────
	usageMutName := strings.Replace(allT, "  zerg agent ls ｜ 茧壁:host ｜ 说明", "  zerg agent lsx ｜ 茧壁:host ｜ 说明", 1)
	usageMutText := strings.Replace(allT, "  zerg gate run ｜ 茧壁:host ｜ 说明", "  zerg gate run [--fast] ｜ 茧壁:host ｜ 说明", 1)
	usageMutMore := strings.Replace(allT, "  zerg version ｜ 茧壁:host ｜ 说明",
		"  zerg version ｜ 茧壁:host ｜ 说明\n  zerg zzz ls ｜ 茧壁:host ｜ 说明", 1)
	c(21, false, chsUsageCover(allT, chsFixTree), "", "闸A")
	c(22, true, chsUsageCover(usageMutName, chsFixTree), "漏印", "")
	c(23, true, chsUsageCover(usageMutText, chsFixTree), "文本不符", "")
	c(24, true, chsUsageCover(usageMutMore, chsFixTree), "多印", "")
	c(25, true, chsUsageCover(allT, nil), "抽空", "")
	c(26, false, append(chsDupScan("help --all", allT), chsDupScan("help", bareT)...), "", "闸B")
	c(27, true, chsDupScan("help --all", allT+"  zerg version ｜ 茧壁:host ｜ 说明\n"), "重复行", "")
	c(28, true, chsDupScan("help --all", allT+"族级面（9 族 · 第二份段）:\n"), "族级段", "")

	// 字节降级：面**字节变了（行数不变）⇒ 判据仍绿** ＋ 观察行给归因（成对：行数变 ⇒ 必红 = 上面的负控②b）
	byteMut := strings.Replace(allT, "zerg —— 形状夹具 · 深度 ≤ 3 层", "zerg —— 形状夹具 · 深度 ≤ 3 层（字节降级夹具）", 1)
	byteBad := chsJudge(byteMut, bareT, dangT, chsFixCmds, bl, nil)
	byteObs := chsByteDrift([][2]string{{"help --all", byteMut}, {"help", bareT}}, bl)
	byteCases := []struct {
		name string
		ok   bool
	}{
		{"字节降级①：面字节变了（行数不变）⇒ 判据仍绿（✗ 不进 rc）", len(byteBad) == 0},
		{"字节降级②：观察行打印「与冻结值的差」", chsAnyHasSub(byteObs, "（差 +")},
		{"字节降级③：观察行点明冻结读数（现读 ⟷ 冻结）", chsAnyHasSub(byteObs, "⟷ 冻结")},
	}
	total += len(byteCases)
	for _, bc := range byteCases {
		if !bc.ok {
			failed++
		}
		mark := "✓"
		if !bc.ok {
			mark = "✗"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, bc.name)
	}

	// ── A 档取数口（`GAP-20261001-47` · 拆档）：读落盘 help 面制品 ──────────────
	artDir, aerr := os.MkdirTemp("", "hshape-artifact-")
	if aerr != nil {
		artDir = os.TempDir()
	} else {
		defer os.RemoveAll(artDir)
	}
	art := chsPyJoin(artDir, "help.txt")
	_ = os.WriteFile(art, []byte("zerg —— 夹具\n"), 0o644)
	emptyArt := chsPyJoin(artDir, "empty.txt")
	_ = os.WriteFile(emptyArt, []byte(""), 0o644)
	artCases := []struct {
		name string
		ok   bool
	}{
		{"A 档①制品读得出", chsArtCase(func() (string, string) { return chsReadFaceFile(art) }, func(t, w string) bool { return t == "zerg —— 夹具\n" })},
		{"A 档②制品不在盘 ⇒ 判不了（不许当空面）", chsArtCase(func() (string, string) { return chsReadFaceFile(chsPyJoin(artDir, "nope.txt")) }, func(t, w string) bool { return w != "" })},
		{"A 档③制品零输出 ⇒ 判不了", chsArtCase(func() (string, string) { return chsReadFaceFile(emptyArt) }, func(t, w string) bool { return w != "" })},
	}
	total += len(artCases)
	for _, ac := range artCases {
		if !ac.ok {
			failed++
		}
		mark := "✓"
		if !ac.ok {
			mark = "✗"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, ac.name)
	}

	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total, failed)
	return failed == 0
}

// chsFaceBlJSON —— `--baseline <件>` 覆盖件的逐面形状（指针三态：缺键 ⇒ 沿用内置读数）。
type chsFaceBlJSON struct {
	Lines                *int        `json:"lines"`
	Bytes                *int        `json:"bytes"`
	Rows                 *int        `json:"rows"`
	Last                 *string     `json:"last"`
	Tail                 *string     `json:"tail"`
	ConfirmNotInUsage    *[]string   `json:"confirm_not_in_usage"`
	ConfirmValueMismatch *[][]string `json:"confirm_value_mismatch"`
}

// chsLoadBaseline —— 只读覆盖口（`--baseline <件>` · 缺口 GAP-20260927-427）：读 JSON 覆盖件，逐面
// 把被点名的格替换进冻结基线 `chsBaseline` 的副本；缺键面沿用内置（✗ 不写盘 · ✗ 不改冻结常量）。
// 件不在盘 / 读不回 / 非 JSON / 顶层非对象 / 某面非对象 ⇒ fail-closed（返回理由 ⇒ 调用方不给结论）。
func chsLoadBaseline(base chsBaselineSet, path string) (chsBaselineSet, string) {
	if !chsIsFile(path) {
		return nil, "基线覆盖件不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "基线覆盖件读不回来（" + chsOSErrStr(path, err) + "）"
	}
	var obj map[string]chsFaceBlJSON
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "基线覆盖件读不回来（" + chsJSONErrStr(raw, err) + "）"
	}
	out := chsBaselineSet{}
	for k, v := range base {
		out[k] = v
	}
	for face, fj := range obj {
		cur := out[face]
		if fj.Lines != nil {
			cur.HasLines, cur.Lines = true, *fj.Lines
		}
		if fj.Bytes != nil {
			cur.HasBytes, cur.Bytes = true, *fj.Bytes
		}
		if fj.Rows != nil {
			cur.HasRows, cur.Rows = true, *fj.Rows
		}
		if fj.Last != nil {
			cur.HasLast, cur.Last = true, *fj.Last
		}
		if fj.Tail != nil {
			cur.HasTail, cur.Tail = true, *fj.Tail
		}
		if fj.ConfirmNotInUsage != nil {
			cur.ConfirmNotInUsage = *fj.ConfirmNotInUsage
		}
		if fj.ConfirmValueMismatch != nil {
			cur.ConfirmValueMismatch = *fj.ConfirmValueMismatch
		}
		out[face] = cur
	}
	return out, ""
}

// ── argparse 复刻（`-h/--help` · 长旗标前缀匹配 · `=` 取值 · 缺值报话 · 未知旗标）──

const chsUsageText = "usage: check-help-shape.py [-h] [--repo-root REPO_ROOT] [--bin BIN]\n" +
	"                           [--matrix MATRIX] [--faces-dir FACES_DIR]\n" +
	"                           [--baseline BASELINE] [--json] [--emit-baseline]\n" +
	"                           [--list-rules] [--self-test]\n"

const chsHelpText = chsUsageText + "\n" +
	"`help` 面的形状判据（族级段 / 条数 / 末行）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo-root REPO_ROOT\n" +
	"  --bin BIN\n" +
	"  --matrix MATRIX\n" +
	"  --faces-dir FACES_DIR\n" +
	"                        A 档：四个 help 面改「读落盘制品目录」（help-all.txt/help.txt/help-\n" +
	"                        dangerous.txt/help-config.txt · 缺口 GAP-20261001-47）\n" +
	"  --baseline BASELINE\n" +
	"                        只读覆盖口：读 JSON 覆盖件替代冻结基线（逐面 lines/bytes/\n" +
	"                        rows/last/tail/confirm_*；缺键面沿用内置）· 不写盘\n" +
	"  --json\n" +
	"  --emit-baseline\n" +
	"  --list-rules\n" +
	"  --self-test\n"

type chsArgs struct {
	repoRoot    string
	repoSet     bool
	binVal      string
	binSet      bool
	matrix      string
	matrixSet   bool
	facesDir    string
	facesDirSet bool
	baseline    string
	baselineSet bool
	json        bool
	emitBase    bool
	listRules   bool
	selfTest    bool
	help        bool
	badArg      string
	missingVal  string
}

func chsParseArgs(argv []string) chsArgs {
	var a chsArgs
	longs := []string{"--repo-root", "--bin", "--matrix", "--faces-dir", "--baseline", "--json", "--emit-baseline", "--list-rules", "--self-test", "--help"}
	takesVal := map[string]bool{"--repo-root": true, "--bin": true, "--matrix": true, "--faces-dir": true, "--baseline": true}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if s == "-h" || s == "--help" {
			a.help = true
			return a
		}
		if !strings.HasPrefix(s, "-") || s == "-" || s == "--" {
			if a.badArg == "" {
				a.badArg = s
			}
			continue
		}
		name, inline := s, ""
		hasInline := false
		if j := strings.Index(s, "="); j >= 0 {
			name, inline, hasInline = s[:j], s[j+1:], true
		}
		var cand []string
		for _, L := range longs {
			if L == name {
				cand = []string{L}
				break
			}
			if strings.HasPrefix(L, name) {
				cand = append(cand, L)
			}
		}
		if len(cand) == 0 {
			if a.badArg == "" {
				a.badArg = s
			}
			continue
		}
		if len(cand) > 1 {
			// py argparse 报「ambiguous option」；本入口归入未知档（未覆盖面）。
			if a.badArg == "" {
				a.badArg = s
			}
			continue
		}
		full := cand[0]
		if takesVal[full] {
			val := inline
			if !hasInline {
				if i+1 < len(argv) {
					i++
					val = argv[i]
				} else {
					if a.missingVal == "" {
						a.missingVal = full
					}
					continue
				}
			}
			switch full {
			case "--repo-root":
				a.repoRoot, a.repoSet = val, true
			case "--bin":
				a.binVal, a.binSet = val, true
			case "--matrix":
				a.matrix, a.matrixSet = val, true
			case "--faces-dir":
				a.facesDir, a.facesDirSet = val, true
			case "--baseline":
				a.baseline, a.baselineSet = val, true
			}
			continue
		}
		if hasInline {
			if a.badArg == "" {
				a.badArg = s
			}
			continue
		}
		switch full {
		case "--json":
			a.json = true
		case "--emit-baseline":
			a.emitBase = true
		case "--list-rules":
			a.listRules = true
		case "--self-test":
			a.selfTest = true
		}
	}
	return a
}

// chsDefaultRoot —— 声明 (a)：从 cwd 上溯 ≤6 层找含 `scripts/gates/` 的目录（py 用 `__file__` 上溯三级）。
func chsDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	start, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	d := start
	for i := 0; i < 6; i++ {
		if fi, serr := os.Stat(chsPyJoin(d, "scripts/gates")); serr == nil && fi.IsDir() {
			return d
		}
		parent := chsPyDirname(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return wd
}

// checkHelpShapeGateMain —— 入口（= py `main()`）。
func checkHelpShapeGateMain(stdout, stderr io.Writer, argv []string) int {
	a := chsParseArgs(argv)
	if a.help {
		fmt.Fprint(stdout, chsHelpText)
		return 0
	}
	if a.badArg != "" || a.missingVal != "" {
		fmt.Fprint(stderr, chsUsageText)
		if a.missingVal != "" {
			fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", chsProg, a.missingVal)
		} else {
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", chsProg, a.badArg)
		}
		return 2
	}

	if a.listRules {
		fmt.Fprint(stdout, chsListRules)
		return 0
	}
	if a.selfTest {
		fmt.Fprint(stdout, "── 自检（合成夹具 · 不碰真目标）──\n")
		if chsSelfTest(stdout) {
			return 0
		}
		return 2
	}

	repoRoot := a.repoRoot
	if !a.repoSet {
		repoRoot = chsDefaultRoot()
	}
	binz := a.binVal
	if !a.binSet {
		binz = chsPyJoin(chsPyJoin(repoRoot, "bin"), "zerg")
	}
	matrix := a.matrix
	if !a.matrixSet {
		matrix = chsPyJoinAll(append([]string{repoRoot}, chsMatrixRelParts...)...)
	}

	cmds, cwhy := chsMatrixCommands(matrix)
	if cmds == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", cwhy)
		return 2
	}
	// 闸A 的第二份真源 = 命令树源码的 `usage` 串：取不到 ⇒ fail-closed（✗ 不许静默跳过闸A）。
	tree, twhy := chsTreeUsage(repoRoot)
	if tree == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", twhy)
		return 2
	}
	order := []string{"help --all", "help", "help dangerous", "help config"}
	argvs := [][]string{{"help", "--all"}, {"help"}, {"help", "dangerous"}, {"help", "config"}}
	faces := map[string]string{}
	for i, face := range order {
		var text, why string
		if a.facesDir != "" {
			// A 档（`GAP-20261001-47` · 拆档）：只读落盘 help 面制品 · 不起子进程。
			text, why = chsReadFaceFile(chsPyJoin(a.facesDir, chsFaceArtifactFiles[face]))
		} else {
			text, why = chsRunFace(binz, argvs[i])
		}
		if text == "" {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", why)
			return 2
		}
		faces[face] = text
	}
	if a.facesDir != "" {
		fmt.Fprintf(stdout, "  取数档 = A 档（只读 help 面制品 %s）—— 不起 bin/zerg 子进程\n", a.facesDir)
	}

	if a.emitBase {
		return chsEmitBaseline(stdout, faces, order)
	}
	bl := chsBaseline
	if a.baselineSet {
		ov, why := chsLoadBaseline(chsBaseline, a.baseline)
		if why != "" {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", why)
			return 2
		}
		bl = ov
		fmt.Fprintf(stdout, "  基线档 = 只读覆盖件（%s）—— 冻结基线被替代（✗ 不写盘）\n", a.baseline)
	}

	fmt.Fprintf(stdout, "`help` 面形状判据 · v%s（族级段 / 条数 / 末行 / 危险档 / 配置主题）\n", chsVersion)
	fams := chsFamiliesOf(cmds)
	_, hasIdx, _, _, toks, secLines, _ := chsFamilySection(faces["help --all"])
	secStr := "缺"
	if hasIdx {
		secStr = strconv.Itoa(secLines)
	}
	fmt.Fprintf(stdout, "  命令树 %d 条 ⇒ 现算 %d 族 · `help --all` %d 行 / %d 字节 · 裸 `help` %d 行 · "+
		"族级段 %s 行（token %d）\n",
		len(cmds), len(fams), len(chsSplitLines(faces["help --all"])),
		len(faces["help --all"]), len(chsSplitLines(faces["help"])), secStr, len(toks))

	// 字节面：观察值 + 归因（✗ 不进 rc —— 字节是派生量，随用法串自然增长）。
	for _, o := range chsByteDrift([][2]string{{"help --all", faces["help --all"]}, {"help", faces["help"]}}, bl) {
		fmt.Fprintf(stdout, "  %s\n", o)
	}
	cfg := faces["help config"]
	bad := chsJudge(faces["help --all"], faces["help"], faces["help dangerous"], cmds, bl, &cfg)
	// 闸A【逐字覆盖】· 闸B【无重复】：本轮新增的两条真鉴别力闸（进退码）。
	bad = append(bad, chsUsageCover(faces["help --all"], tree)...)
	bad = append(bad, chsDupScan("help --all", faces["help --all"])...)
	bad = append(bad, chsDupScan("help", faces["help"])...)
	d := faces["help dangerous"]
	if len(bad) > 0 {
		fmt.Fprint(stdout, "⇒ 判红（rc=1）：\n")
		for _, b := range bad {
			fmt.Fprintf(stdout, "  ✗ %s\n", b)
		}
	} else {
		secInt := -1
		if hasIdx {
			secInt = secLines
		}
		fmt.Fprintf(stdout, "  ✓ S1 族级段 %d 族 ⟷ 命令树逐 token 相等（裸 `help` 不带段）\n", len(fams))
		fmt.Fprintf(stdout, "  ✓ S2 rows(--all)=%d == rows(help)=%d + rows(dangerous)=%d（危险档净增）；"+
			"两个面**行数** == 冻结读数（段 %d 行 · 命令树 %d 条为族名另一源；字节已降级为上面的观察值）\n",
			len(chsRowsOf(faces["help --all"])), len(chsRowsOf(faces["help"])),
			len(chsRowsOf(d)), secInt, len(cmds))
		fmt.Fprintf(stdout, "  ✓ 闸A 逐字覆盖：命令树源码 %d 条 `usage` 串 ⟷ `help --all` 面逐字（漏印 0 · 多印 0 · 文本不符 0）\n", len(tree))
		fmt.Fprint(stdout, "  ✓ 闸B 无重复：`help --all` / 裸 `help` 两面 0 条重复行 · 族级段各一份\n")
		fmt.Fprint(stdout, "  ✓ S3 末行 = 族名清单现算重建的那一行（裸 `help` 末行逐字符合）\n")
		_, nT, nO, nC, hasTN := chsDangerTail(d)
		nTv, nOv, nCv := -1, -1, -1
		if hasTN {
			nTv, nOv, nCv = nT, nO, nC
		}
		gapN := len(chsDangerConfirmGap(d, faces["help --all"]))
		pairN := len(chsDangerConfirmPairs(d, faces["help --all"]))
		vmN := len(chsDangerConfirmValueMismatch(d, faces["help --all"]))
		fmt.Fprintf(stdout, "  ✓ S4 危险档面：表尾 %d 条 == 已开放 %d + 未开放 %d == 逐条行数 %d；逐条标记逐数相符；"+
			"逐条行形状全合；`--confirm` 目标 ⟷ 用法串脱钩 %d 处（= 冻结名单）；"+
			"两面同印 `--confirm=` 的 %d 条值逐字相等（值冲突 %d 处 = 冻结值）；"+
			"行/字节/条数 == 冻结读数 %d/%d/%d\n",
			nTv, nOv, nCv, len(chsDangerRows(d)), gapN, pairN, vmN,
			len(chsSplitLines(d)), len(d), len(chsDangerRows(d)))
		cf := faces["help config"]
		chainLevels := -1
		if ch := chsCfgChainLines(cf); len(ch) > 0 {
			chainLevels = len(chsCfgChainTokens(ch[0]))
		}
		fmt.Fprintf(stdout, "  ✓ S5 配置主题面：链行 ⟷ 逐级说明 %d 级同序逐字相等；链行 ⟷ 裸 `help` 同源逐字；"+
			"行/字节 == 冻结读数 %d/%d\n", chainLevels, len(chsSplitLines(cf)), len(cf))
		fmt.Fprint(stdout, "⇒ 五条（S1–S5）＋ 闸A/闸B 都合（rc=0）\n")
	}

	if a.json {
		famSecLines := "null"
		if hasIdx {
			famSecLines = strconv.Itoa(secLines)
		}
		fmt.Fprintf(stdout, `{"schema": "zerg-gate/v1", "gate": "check-help-shape", "version": %s, `+
			`"commands": %d, "families": %d, `+
			`"all_lines": %d, "all_bytes": %d, "bare_lines": %d, `+
			`"family_section_lines": %s, "family_tokens": %d, `+
			`"danger_lines": %d, "danger_bytes": %d, "danger_rows": %d, `+
			`"danger_confirm_not_in_usage": %s, "danger_confirm_both_faces": %d, `+
			`"danger_confirm_value_mismatch": %s, `+
			`"config_lines": %d, "config_bytes": %d, "config_chain_lines": %d, `+
			`"bad": %s, "mode": "judge"}`+"\n",
			chsJSONString(chsVersion), len(cmds), len(fams),
			len(chsSplitLines(faces["help --all"])), len(faces["help --all"]),
			len(chsSplitLines(faces["help"])),
			famSecLines, len(toks),
			len(chsSplitLines(d)), len(d), len(chsDangerRows(d)),
			chsJSONListStrs(chsDangerConfirmGap(d, faces["help --all"])),
			len(chsDangerConfirmPairs(d, faces["help --all"])),
			chsJSONListLists(chsDangerConfirmValueMismatch(d, faces["help --all"])),
			len(chsSplitLines(faces["help config"])), len(faces["help config"]),
			len(chsCfgChainLines(faces["help config"])),
			chsJSONListStrs(bad))
	}
	if len(bad) > 0 {
		return 1
	}
	return 0
}
