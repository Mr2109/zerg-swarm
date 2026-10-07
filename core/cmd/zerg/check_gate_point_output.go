// check_gate_point_output.go —— 判据件 `scripts/gates/check-gate-point-output.py`（102 行）的 **Go 真搬**
// （v2.5.14 · M2 · 搬运面第 1 粒 · 入口 `checkGatePointOutputGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的
//	argparse 面差异与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**行数最小的一批** · 纯静态无外部树依赖）：
//
//	✓ 选：判据是**纯字符串 / 正则面** —— 只读 `<root>/scripts/gates/precommit-gates.sh` 一件，
//	  逐行按 `add_step <挂点> "<名>" <档位> <工作目录> "<命令>"` 正则抽 (挂点, 档位) 聚合成表。
//	  **不真跑子进程、不碰网络、零计时、默认档不写盘**（`--self-test` 只在 tempfile 里造件，见下）。
//	· 无外部树依赖：`--root` / env `ZERG_ROOT` 可显式指到任意合成假仓根（本批新口径：外部树走
//	  显式入参旋钮，旋钮名照 py 同名旗标 `--root` / env `ZERG_ROOT`）。
//	· 退码 0/2 两档齐 · 纯只读无写面 · 行数最小（102 行）· `git status` 现读不含它（无人正在改）。
//	· 现读 import 面：argparse / json / os / re / sys / tempfile —— **零 `subprocess` / `time` /
//	  `datetime` / `random` / `socket` / `urllib`**。`tempfile.TemporaryDirectory` 本身**自带回收**
//	  （`with` 形）⇒ **非** A 类泄漏形态（逐字保留其自回收语义）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 add_step 表）；
//	· **不**删 `scripts/gates/check-gate-point-output.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkGatePointOutputGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkGatePointOutputGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 成表 / 2 不给结论（脚本取不到 · 空转 0 步 · 非 UTF-8）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序（本件无目录遍历）—— 逐点交代 ═══════════════════════
//
//	py 本件**零 `os.walk` / `os.listdir` / `glob`**：只 `open()` 单个已知路径件、按行（`splitlines`）
//	扫描。Go 侧同为**单件读 + 逐行扫描** ⇒ **无遍历序面**、**零 Go map 迭代进输出**（见下）。
//	★ 输出行序的决定者 = py `order` 列表（`(挂点,档位)` 首遇序，list 保序）——本件用**切片对**复现，
//	  `cnt` 只做 O(1) 计数查值、**不进输出**（✗ 无 Go map 迭代）。
//
// ═══════════════════════ ★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明）═══════════════════════
//
//	py `ADD_STEP_RE = ^\s*add_step\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+(\S+)\s+(\S+)\s+"((?:[^"\\]|\\.)*)"`
//	① py `\s` 是 **Unicode 语义** ⇒ 本件按码点展开 `cgpoWS`（含 `\x1c-\x1f` / `\x85` / `\xa0` /
//	   `\x2028` / `\x2029` / `\x3000` …），`\S` 展开为 `[^cgpoWS]`。逐处于 `cgpoAddStepRe`。
//	② py `\d` / `\b` / 非多行 `$` / `(?!)` / `\1`：本件**零用量**（无锚尾 / 无边界 / 无环视 / 无组引用）
//	   ⇒ ✗ 无需转译。`re.match` = 从串首锚定 ⇒ 本件正则带前导 `^` 且逐行喂入（单行内无 `\n`）等价。
//	③ `\\.`：py 的 `.` 默认不匹配 `\n`，Go RE2 `.` 亦不匹配 `\n`；逐行喂入下两者等价。
//	④ `[^"\\]`：两语言同形（class 内 `\\` 表一个反斜杠）。RE2 支持非捕获组 `(?:…)`。
//	⑤ 交替 / leftmost：本件全为**单串匹配**（`FindStringSubmatch`）⇒ 无交替顺序面。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异（本件近似逐字同构）═══════════════════════
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **起于 cwd**（不是 `__file__` 上溯）。
//	Go 无 `__file__` 语境，但本件 py 本就用 cwd ⇒ 走 **`os.LookupEnv("ZERG_ROOT")` + `os.Getwd()`**
//	逐字同构（`LookupEnv` 保真「键在但空串」档：py `.get(k, default)` 对显式空串回 `""`、
//	Go 亦回 `""` —— ✗ 不用 `os.Getenv`（它把「键在空串」与「键缺」并成同形））。
//	⚠ 未覆盖面：py `os.getcwd()` ⟷ Go `os.Getwd()`（后者优先 `$PWD` 若其 `stat` 命中 `.`）
//	在**符号链接路径**下可能不同 ⇒ 对拍/夹具一律落**非符号链接路径**。
//
// ═══════════════════════ ★ 声明 (b) 自指检查 —— 本件**不在**自己的扫描面内（四形态逐条）═══════════════════════
//
//	① 扫描面 = `<root>/scripts/gates/precommit-gates.sh` **单件** ⇒ 本件（`core/cmd/zerg/*.go`）不在内 ⇒ 自指 = 假。
//	② 件头/正文**零真引号路径原串**落进 precommit-gates.sh 的扫描输出（本件只把该路径当**输入**读）。
//	③ self-test 魔法串（`门a/门b/门c`）落点 = **临时目录**，不进仓树 ⇒ 真仓 `--self-test` 不受影响。
//	④ 是否落在**他门**扫描面内：本件在 `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面。
//	   R3 正则只认「错误构造调用的首参字面」（即真源 API 的调用式，**带左括号**），**✗ 不扫 JSON 信封的
//	   kind 键**（该门 2026-10-02 已剔除该正则，现读其源码 `USE_RE` 为证）⇒ 本件**不写该调用式** ⇒ 命中 0。
//	   ⚠ 也落 `check-hardcoded-private-paths`（其 `SCAN_DIRS` 含 `core/`）⇒ 本件**不写任何私有绝对路径的
//	   首段字面**（首段即那两个常见私有挂载前缀 —— ★ 此处按**描述式**写，✗ 不把原串当例子写出来，
//	   否则「我没写 X」这句本身就含 X）⇒ 命中 0。
//
// ═══════════════════════ ★ py 字符串原语逐点交代 ═══════════════════════
//
//	· `text.splitlines()`：py 在**多边界**上切（`\n \r \r\n \v \f \x1c \x1d \x1e \x85 \u2028 \u2029`）
//	  ⇒ 本件 `cgpoSplitLines` 逐码点复刻（✗ 不用 `strings.Split(s,"\n")`）；★ 不append尾空行（同 py）。
//	· `open()` 文本模式 = **universal newlines**（`\r\n`/裸 `\r` 折 `\n`）：`cgpoSplitLines` 对 `\r`
//	  与 `\r\n` 都当边界 ⇒ 与「先折行再切」等价（逐字对齐，见上「正则语义差①」）。
//	· 路径拼接：py `os.path.join(root, "scripts","gates","precommit-gates.sh")` ⇒ 本件 `cgpoPyJoin`
//	  按 CPython posixpath.join 逐段规则直译（空段 / 后段绝对路径 / 尾斜杠三态）。
//	· `%-16s` / `%-12s`：py 按**码点**宽度；Go `fmt` 对字符串按**rune** 宽度（已实测同值，见单测）。
//	  ★ 表内 scope/mode 为 ASCII（字节 == 码点），表头中文亦然 —— 两语言逐字同。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// cgpoScriptRel —— py `SCRIPT_REL`（`os.path.join("scripts","gates","precommit-gates.sh")`）。
const cgpoScriptRel = zergPrecommitGatesRel

// cgpoWS —— py `\s` 的 Unicode 展开（同 py `_PyUnicode_IsWhitespace` 认可集）。
const cgpoWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// cgpoAddStepRe —— py `ADD_STEP_RE` 的直译（`\s`→`[cgpoWS]`、`\S`→`[^cgpoWS]`、`^` 逐行锚定）。
var cgpoAddStepRe = regexp.MustCompile(`^(?:[` + cgpoWS + `])*add_step(?:[` + cgpoWS + `])+([^` + cgpoWS + `]+)(?:[` + cgpoWS + `])+"((?:[^"\\]|\\.)*)"(?:[` + cgpoWS + `])+([^` + cgpoWS + `]+)(?:[` + cgpoWS + `])+([^` + cgpoWS + `]+)(?:[` + cgpoWS + `])+"((?:[^"\\]|\\.)*)"`)

// cgpoOutputFace —— py `OUTPUT_FACE`（档位→输出面归类）。
var cgpoOutputFace = map[string]string{
	"rc":         "阻断面",
	"empty":      "阻断面",
	"tri":        "阻断面",
	"tri-report": "只报告面",
}

// cgpoUnknownMode —— py `OUTPUT_FACE.get(k[1], "…")` 的缺省串。
const cgpoUnknownMode = "（档位不在四档闭集内）"

type cgpoStep struct {
	scope string
	name  string
	mode  string
}

// cgpoKey —— 聚合键 `(挂点, 档位)`（切片保序承运，✗ 不作 map 迭代输出）。
type cgpoKey struct {
	scope string
	mode  string
}

// cgpoSplitLines —— 复刻 py `str.splitlines()`（keepends=False）的多边界集。
func cgpoSplitLines(s string) []string {
	rs := []rune(s)
	var out []string
	var cur []rune
	flush := func() {
		out = append(out, string(cur))
		cur = cur[:0]
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\n' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d || r == 0x1e ||
			r == 0x85 || r == 0x2028 || r == 0x2029:
			flush()
		case r == '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			flush()
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// cgpoPyJoin —— 复刻 CPython `posixpath.join`（空段 / 后段绝对路径 / 尾斜杠三态逐段直译）。
func cgpoPyJoin(parts ...string) string {
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

// cgpoParseSteps —— py `parse_steps()`：逐行 match、保序追加 (scope,name,mode)。
func cgpoParseSteps(text string) []cgpoStep {
	var rows []cgpoStep
	for _, ln := range cgpoSplitLines(text) {
		m := cgpoAddStepRe.FindStringSubmatch(ln)
		if m != nil {
			rows = append(rows, cgpoStep{scope: m[1], name: m[2], mode: m[3]})
		}
	}
	return rows
}

// cgpoFace —— py `OUTPUT_FACE.get(mode, "…")`。
func cgpoFace(mode string) string {
	if v, ok := cgpoOutputFace[mode]; ok {
		return v
	}
	return cgpoUnknownMode
}

// cgpoJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串字面（保 UTF-8、逃控制符）。
func cgpoJSONStr(s string) string {
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

// cgpoBuildJSON —— py `json.dumps({"schema":…,"kind":…,"steps":…,"rows":…}, ensure_ascii=False)`
// 逐字节复刻（键序 = 插入序 · 默认分隔符 `, ` 与 `: ` · 行内无缩进）。
func cgpoBuildJSON(steps int, order []cgpoKey, cnt map[cgpoKey]int) string {
	var b strings.Builder
	b.WriteString(`{"schema": "zerg/v1", "kind": "GatePointOutput", "steps": `)
	b.WriteString(strconv.Itoa(steps))
	b.WriteString(`, "rows": [`)
	for i, k := range order {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"scope": `)
		b.WriteString(cgpoJSONStr(k.scope))
		b.WriteString(`, "mode": `)
		b.WriteString(cgpoJSONStr(k.mode))
		b.WriteString(`, "output_face": `)
		b.WriteString(cgpoJSONStr(cgpoFace(k.mode)))
		b.WriteString(`, "count": `)
		b.WriteString(strconv.Itoa(cnt[k]))
		b.WriteString(`}`)
	}
	b.WriteString("]}")
	return b.String()
}

// cgpoRun —— py `run(root, as_json)` 的逐字复刻。
func cgpoRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	path := cgpoPyJoin(root, cgpoScriptRel)
	st, statErr := os.Stat(path)
	if statErr != nil || !st.Mode().IsRegular() {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门禁脚本不在盘：%s\n", path)
		return 2
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		// py `except OSError as e` ⇒ 正文 `% e`（`[Errno N] <strerror>: '<path>'`）。
		if os.IsPermission(readErr) {
			fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门禁脚本读不到：[Errno 13] Permission denied: '%s'\n", path)
		} else {
			fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门禁脚本读不到：%s\n", readErr)
		}
		return 2
	}
	if !utf8.Valid(data) {
		// py 该档 = 未捕 `UnicodeDecodeError`（`except OSError` 兜不住）⇒ 回溯 + rc=1（C 类无兜底）；
		// 本件按新口径 fail-closed rc=2，并列入未覆盖面（见件尾）。
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门禁脚本读不到：非 UTF-8 输入（py 原件此档回溯 rc=1 · 见未覆盖面）\n")
		return 2
	}
	steps := cgpoParseSteps(string(data))
	if len(steps) == 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：现读 0 步（空转 = 假绿）—— %s\n", path)
		return 2
	}
	var order []cgpoKey
	cnt := map[cgpoKey]int{}
	for _, s := range steps {
		k := cgpoKey{scope: s.scope, mode: s.mode}
		if _, ok := cnt[k]; !ok {
			order = append(order, k)
		}
		cnt[k]++
	}
	if asJSON {
		fmt.Fprintln(stdout, cgpoBuildJSON(len(steps), order, cnt))
		return 0
	}
	fmt.Fprintln(stdout, "── 逐挂点输出表（挂点×档位×输出面）──")
	fmt.Fprintf(stdout, "  %-16s %-12s %-12s %s\n", "挂点", "档位", "输出面", "门数")
	for _, k := range order {
		fmt.Fprintf(stdout, "  %-16s %-12s %-12s %d\n", k.scope, k.mode, cgpoFace(k.mode), cnt[k])
	}
	fmt.Fprintf(stdout, "  —— 共 %d 步 · %d 个（挂点,档位）组\n", len(steps), len(order))
	return 0
}

// cgpoPyRepr —— py `%r` 的近似（仅失败档打印 steps 列表；结构 = [{'scope': '…', 'name': '…', 'mode': '…'}]）。
func cgpoPyRepr(steps []cgpoStep) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range steps {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{'scope': ")
		b.WriteString(cgpoPyReprStr(s.scope))
		b.WriteString(", 'name': ")
		b.WriteString(cgpoPyReprStr(s.name))
		b.WriteString(", 'mode': ")
		b.WriteString(cgpoPyReprStr(s.mode))
		b.WriteString("}")
	}
	b.WriteByte(']')
	return b.String()
}

// cgpoPyReprStr —— py `repr(str)` 的单引号形态（仅失败档用）。
func cgpoPyReprStr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
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

// cgpoSelfTest —— py `self_test()`：正控解析/成表 rc=0 · 负控空脚本 rc=2（合成件全在临时目录）。
// ★ 临时目录前缀用 `tmp`（逐字对齐 py `tempfile.TemporaryDirectory()` 的缺省 prefix）⇒ 两态
//
//	stderr 里只差随机后缀（对拍掩码该段，见未覆盖面）。
func cgpoSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintln(stdout, "自检 FAIL（3 格）")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }() // TemporaryDirectory 语义：自带回收
	rel := cgpoPyJoin(td, cgpoScriptRel)
	_ = os.MkdirAll(filepath.Dir(rel), 0o755)
	_ = os.WriteFile(rel, []byte(
		"add_step docs \"门a\" tri \"$R\" \"x.py\"\n"+
			"add_step docs \"门b\" tri-report \"$R\" \"y.py\"\n"+
			"add_step gates \"门c\" rc \"$R\" \"z.sh\"\n"), 0o644)
	data, _ := os.ReadFile(rel)
	steps := cgpoParseSteps(string(data))
	if len(steps) != 3 || steps[0].scope != "docs" || steps[2].mode != "rc" {
		fmt.Fprintf(stdout, "  ✗ 正控失败：%s\n", cgpoPyRepr(steps))
		ok = false
	}
	if cgpoRun(stdout, stderr, td, false) != 0 {
		fmt.Fprintln(stdout, "  ✗ 正控 rc 失败")
		ok = false
	}
	_ = os.WriteFile(rel, []byte("# 无 add_step\n"), 0o644)
	if cgpoRun(stdout, stderr, td, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：空脚本应 rc=2")
		ok = false
	}
	if ok {
		fmt.Fprintln(stdout, "自检 PASS（3 格）")
		return 0
	}
	fmt.Fprintln(stdout, "自检 FAIL（3 格）")
	return 1
}

// cgpoProg / cgpoUsage —— argparse 的 prog 名（= py 原件基名）与 usage 行。
const cgpoProg = "check-gate-point-output.py"

func cgpoUsage() string {
	return "usage: " + cgpoProg + " [-h] [--root ROOT] [--json] [--self-test]"
}

func cgpoArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", cgpoUsage(), cgpoProg, msg)
	return 2
}

// cgpoHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻；★ py≥3.10 印 `options:` —— 见未覆盖面）。
func cgpoHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, cgpoUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "逐挂点输出表元门（GAP-20260927-152）")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help   show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --self-test")
}

// cgpoLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var cgpoLongOpts = []string{"--help", "--root", "--json", "--self-test"}

// cgpoResolveLong —— argparse 规则：精确名优先，否则取**唯一前缀**；多匹 = 歧义。
func cgpoResolveLong(name string) (string, bool, bool) {
	for _, o := range cgpoLongOpts {
		if o == name {
			return o, false, true
		}
	}
	var matches []string
	for _, o := range cgpoLongOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], false, true
	}
	if len(matches) > 1 {
		return "", true, false
	}
	return "", false, false
}

// cgpoLooksOption —— argparse 判定「下一 token 是否像旗标」（`-` 单字与负数不算）。
func cgpoLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if cgpoNegNumRe.MatchString(s) {
		return false
	}
	return true
}

var cgpoNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

// checkGatePointOutputGateMain —— 入口（= py `main()`，argparse 面逐字复刻）。
func checkGatePointOutputGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	rootSet := false
	jsonOut := false
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			cgpoHelp(stdout)
			return 0
		case a == "--":
			unrecognized = append(unrecognized, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if j := strings.Index(a, "="); j >= 0 {
				name, val, hasVal = a[:j], a[j+1:], true
			}
			canon, amb, ok := cgpoResolveLong(name)
			if amb {
				return cgpoArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return cgpoArgErr(stderr, "argument --help: ignored explicit argument '"+val+"'")
				}
				cgpoHelp(stdout)
				return 0
			case "--root":
				if hasVal {
					root, rootSet = val, true
				} else {
					if i+1 >= len(argv) || cgpoLooksOption(argv[i+1]) {
						return cgpoArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root, rootSet = argv[i], true
				}
			case "--json":
				if hasVal {
					return cgpoArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--self-test":
				if hasVal {
					return cgpoArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				selfTest = true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			unrecognized = append(unrecognized, a)
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		return cgpoArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		return cgpoSelfTest(stdout, stderr)
	}
	if !rootSet {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	return cgpoRun(stdout, stderr, root, jsonOut)
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3` = 3.9.6）复刻 ⇒ 门跑环境若用 py≥3.10，帮助面 stdout 分叉（rc 恒 0）。
//	   未知旗标 / 缺值 / `--self-test=1` 的 error 正文同为 py 版本相关措辞。
//	② **非 UTF-8 输入件**：py `open(encoding="utf-8")` errors 缺省 strict ⇒ 抛 `UnicodeDecodeError`，
//	   而 py 只 `except OSError` ⇒ **未捕 → 回溯 + rc=1**（C 类无兜底）；本件按新口径 fail-closed rc=2
//	   ＋ 文案不同 ⇒ 该格只保证「不给结论」语义、正文不逐字对齐（已声明为口径差）。
//	③ **符号链接路径下的缺省根**：py `os.getcwd()` ⟷ Go `os.Getwd()`（后者优先 `$PWD`）⇒ 对拍/夹具一律
//	   落**非符号链接路径**以对齐（真仓 cwd 非软链 ⇒ 不触发）。
//	④ **OS 错误串**：读失败档只对 EACCES 复刻 py 的 `[Errno 13] Permission denied: '<path>'`；其余 errno
//	   （EISDIR / ELOOP …）不复刻正文，仅保 rc=2（`os.Stat().Mode().IsRegular()` 前置守卫已把目录档归入
//	   「不在盘」）。
//	⑤ **argparse 面**：本件复刻唯一前缀缩写（`--ro`/`--r` → `--root` 等）＋歧义/缺值/未识别三档文案；
//	   **未**复刻 `-h` 与短旗标捆绑式（`-hj`）与 `--` 之后的 positional 细则（本程序无 positionals）。
//	⑥ **`str.splitlines()`**：全边界集已复刻（含 `\x1c-\x1f` / `\x85` / `\u2028` / `\u2029` / `\v` / `\f`），
//	   已用 fx4 夹具（`\x0c` 夹于非换行处 ＋ CRLF ＋ 裸 `\r`）坐实逐字节同；本件零 `str.strip` 类原语用量。
//	⑦ **`--self-test` 档非确定面**：临时目录随机后缀（掩码规则 = 掩掉 `tmp<随机>` 整段）⇒ 两态 stderr
//	   掩码后逐字节同；已自证 py 连跑两发该段不同、掩码后相同。
