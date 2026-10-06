// check_write_authz.go —— 判据件 `scripts/gates/check-write-authz.py`（257 行）的
// **Go 真搬**（M2 · 搬运面 · 入口 `writeAuthzGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 core/cmd/zerg/）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse
//	面差异与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**纯静态只读**一批）：
//
//	✓ 选：判据只读 `core/cmd/zerg/*.go`（排除 `_test.go`），逐件静态抽取「写点调用（见 cwaWriteRes
//	  的 13 种 os 写面原语）→ 所属顶层函数」，再判「该写点有无交代（本件写点登记表命中 或 所属命令面
//	  在 main.go 的 catalog() 里有 danger）」。**不真跑子进程、不碰网络、零计时、默认档不写盘**
//	  （`--self-test` 只在 `tempfile.TemporaryDirectory()` 里造件 —— `with` 形自带回收，
//	  **非** A 类泄漏形态）。
//	· 无外部树依赖：`--root`（env `ZERG_ROOT`）可显式指到任意合成假仓根（外部树走显式旋钮，
//	  旋钮名照 py 同名旗标 `--root` / env `ZERG_ROOT`）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面（默认档）· 落件前 `git status` 现读不含它。
//	· 现读 import 面：argparse / json / os / re / sys —— **零子进程 / time / datetime / random /
//	  socket / urllib / shutil / glob**（tempfile 只在 self_test 内 import）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 main.go 里注册命令（✗ 不进命令树 · ✗ 不改 gate_inproc_table.go）；
//	· **不**删 `scripts/gates/check-write-authz.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `writeAuthzGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main(argv) 对齐）：
//
//	func writeAuthzGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码  —— 0 全有交代 / 1 有裸写点（逐条点名） / 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序与「键序」（本件有目录遍历 + 正则块）═══════════════════════
//
//	① 目录遍历：py `sorted(os.listdir(scopedir))` —— **显式排序**（按码点；UTF-8 下 == Go 字节序）
//	  ⇒ Go 侧用 sort.Strings 复现。★ 输出行序的决定者 = 这份 sorted(...)（hits 明细行序全由它承运）。
//	② catalog 块：py `re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text)` = 左最先·非重叠·非贪婪 ⇒
//	  Go RE2 同为左最先·非贪婪（`*?` 受支持）⇒ 逐块同构；块内再 `re.search(r"run:\s*…")` 取首个。
//	③ ★ 无 Go map 迭代进输出：declared 集合只做布尔「含否」判；hits 走切片保 py 追加序。
//
// ═══════════════════════ ★ py 字符串 / 行原语（逐处现读）═══════════════════════
//
//	① `f.read().splitlines()` = universal-newlines（`\r\n`/`\r`→`\n`）＋ py splitlines 边界集
//	  （`\n \v \f \x1c \x1d \x1e \x85 U+2028 U+2029`，且尾界不产空末行）⇒ 自写 cwaSplitLines。
//	② 逐行读档（declared_write_calls 的 `for line in f`）只按 `\n` 切 ⇒ 自写 cwaIterLines。
//	③ `line.strip()` = **py str.strip（含 `\x1c-\x1f`）** ⇒ 自写 cwaPyStrip（非 strings.TrimSpace）。
//	④ 切 `[:120]` 按**码点** ⇒ 自写 cwaPySliceRunes。
//	⑤ `json.dumps(ensure_ascii=False)` 默认分隔符 `", "` / `": "` ⇒ 本件手搓 JSON（✗ 不用 json.Marshal）。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异（本件逐字同构）═══════════════════════
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **起于 cwd**（不是 __file__ 上溯）。
//	Go 无 __file__ 语境，但本件 py 本就用 cwd ⇒ 走 os.LookupEnv + os.Getwd 逐字同构。
//
// ═══════════════════════ ★ 声明 (b) 自指面（本件落在扫描面内 ⇒ 逐条消解）═══════════════════════
//
//	① 本判据扫 `core/cmd/zerg/*.go`（非测试）⇒ **本件自身落在扫描面内**。消解：
//	  · 写点正则（cwaWriteRes）用 raw string 写 `\bos\.WriteFile\(` 形 ⇒ 源码正文里**无**连续
//	    `os.WriteFile(` 原串（反斜杠断开）⇒ 不被自己的正则命中。
//	  · 自检夹具的**写点字面**（含真实括号）**一律收进 cwaSelfTest 的局部变量**（✗ 不放包级 const）
//	    ⇒ 那些行所属函数 = cwaSelfTest（名含 selftest + 体含 os.MkdirTemp）⇒ 被「夹具函数」判据剔出。
//	  · 运行档（cwaRun）只读、零写点字面 ⇒ 本件对真仓「裸写点」计数贡献 0（落件后影子仓 A/B 核 sha16）。
//	② 件头注释无真引号路径原串（描述式）。
//
// ═══════════════════════ ★ 未覆盖面（逐条）═══════════════════════
//
//	· argparse `optional arguments:` 面按 **py ≤3.9** 硬编码（≥3.10 印 `options:`）⇒ 环境依赖，见件尾。
//	· py `\w`（FUNC_RE）是 Unicode 语义；Go RE2 为 ASCII ⇒ 仅当 Go 顶层函数名含非 ASCII 字母才分叉。
//	· py 读档 UTF-8 严格模式：非法字节 ⇒ 回溯 rc=1；Go 侧按 replacement 走、不回溯（真仓无障碍，仅声明）。
//	· 不可读件：py open 无 try ⇒ OSError 回溯 rc=1；Go 侧跳过该件（声明为未覆盖）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量（照 py 同名量逐字）────────────────────────────────────────────────
const (
	cwaProg      = "check-write-authz.py"
	cwaHelpDesc  = "写点授权只读静态元门（GAP-20260927-140）"
	cwaRunHeader = "写点授权元门（GAP-20260927-140）"

	cwaBlockedNoScope = "BLOCKED —— 缺 `core/cmd/zerg/`（扫描面取不到）⇒ rc=2 不给结论"
	cwaBlockedNoMain  = "BLOCKED —— 缺 `main.go`（无法解析 catalog）⇒ rc=2 不给结论"
)

// cwaUsageLine —— py argparse 用法行（续行 28 空格，逐字）。
const cwaUsageLine = "usage: check-write-authz.py [-h] [--root ROOT] [--allow ALLOW] [--json]\n" +
	"                            [--self-test]\n"

// cwaHelpText —— py argparse --help 全文（逐字）。
const cwaHelpText = cwaUsageLine +
	"\n" + cwaHelpDesc + "\n\n" +
	"optional arguments:\n" +
	"  -h, --help     show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --allow ALLOW\n" +
	"  --json\n" +
	"  --self-test\n"

// cwaWriteRes —— py WRITE_RES（有序；顺序决定 write_call_of 的返回）。
var cwaWriteRes = []*regexp.Regexp{
	regexp.MustCompile(`\bos\.WriteFile\(`),
	regexp.MustCompile(`\bos\.Create\(`),
	regexp.MustCompile(`\bos\.Remove(?:All)?\(`),
	regexp.MustCompile(`\bos\.Rename\(`),
	regexp.MustCompile(`\bos\.Mkdir(?:All)?\(`),
	regexp.MustCompile(`\bos\.OpenFile\(`),
	regexp.MustCompile(`\bos\.Truncate\(`),
	regexp.MustCompile(`\bos\.Chmod\(`),
	regexp.MustCompile(`\bos\.Chtimes\(`),
	regexp.MustCompile(`\bos\.Symlink\(`),
	regexp.MustCompile(`\bos\.Link\(`),
	regexp.MustCompile(`\bioutil\.WriteFile\(`),
}

// cwaFuncRe —— py FUNC_RE。
var cwaFuncRe = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(`)

// cwaCallRe —— py CALL_RE（写点登记表里的 Call 行）。
var cwaCallRe = regexp.MustCompile(`\{?\s*Call:\s*"([^"]+)"`)

// cwaFixtureNameRe / cwaFixtureBodyRe —— py FIXTURE_NAME_RE / FIXTURE_BODY_RE。
var cwaFixtureNameRe = regexp.MustCompile(`(?i)(selftest|self_test|fixture|fixt|^mk|^newfixture)`)
var cwaFixtureBodyRe = regexp.MustCompile(`(os\.MkdirTemp|ioutil\.TempDir|\.TempDir\(\)|os\.TempDir\(\))`)

// cwaBlockRe / cwaRunRe —— py catalog_funcs_with_danger 的两条正则。
var cwaBlockRe = regexp.MustCompile(`\{(?:[^{}]|\{[^{}]*\})*?\}`)
var cwaRunRe = regexp.MustCompile(`run:\s*([A-Za-z_]\w*)`)

// cwaExemptEntry —— py EXEMPT 的条目（package 级；须在入口自清，见 writeAuthzGateMain）。
type cwaExemptEntry struct {
	File   string
	Call   string
	Reason string
	Source string
}

// cwaExempt —— py 顶部 EXEMPT（生产恒空；self-test 临时置后复位）。
var cwaExempt []cwaExemptEntry

// cwaHit —— 一条裸写点（file, line, func, call, text）。
type cwaHit struct {
	File string
	Line int
	Func string
	Call string
	Text string
}

// ── py 路径原语：posixpath.join（三态逐段直译）────────────────────────────
func cwaPyJoin(parts ...string) string {
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

// ── py 字符串原语 ──────────────────────────────────────────────────────────
func cwaIsPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func cwaPyStrip(s string) string {
	return strings.TrimFunc(s, cwaIsPySpace)
}

func cwaPySliceRunes(s string, n int) string {
	if n < 0 {
		return ""
	}
	cnt := 0
	for i := range s {
		if cnt == n {
			return s[:i]
		}
		cnt++
	}
	return s
}

// cwaSplitLines —— py `str.splitlines()`（输入已做 universal-newlines 归一）。
func cwaSplitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' || r == '\v' || r == '\f' || r == '\x1c' || r == '\x1d' ||
			r == '\x1e' || r == '\u0085' || r == '\u2028' || r == '\u2029' {
			out = append(out, s[start:i])
			i += size
			start = i
		} else {
			i += size
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// cwaIterLines —— py `for line in f`（只按 `\n` 切；有则带尾 \\n）。
func cwaIterLines(s string) []string {
	var out []string
	for s != "" {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:idx+1])
		s = s[idx+1:]
	}
	return out
}

// cwaReadText —— py `open(p,"r",encoding="utf-8").read()`：universal newlines 归一。
func cwaReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, nil
}

// cwaWriteCallOf —— py write_call_of：按 WRITE_RES 顺序取首个命中，返回去尾括号的调用名。
func cwaWriteCallOf(line string) string {
	for _, r := range cwaWriteRes {
		if m := r.FindString(line); m != "" {
			return m[:len(m)-1]
		}
	}
	return ""
}

// cwaFixtureFuncs —— py _fixture_funcs：判为「自检夹具」的顶层函数名集合。
func cwaFixtureFuncs(lines []string) map[string]bool {
	funcs := map[string]bool{}
	cur, start := "<file-scope>", 0
	type bound struct {
		name string
		s, e int
	}
	var bounds []bound
	for i, line := range lines {
		if fm := cwaFuncRe.FindStringSubmatch(line); fm != nil {
			if start != 0 {
				bounds = append(bounds, bound{cur, start, i})
			}
			cur, start = fm[1], i
		}
	}
	if start != 0 {
		bounds = append(bounds, bound{cur, start, len(lines)})
	}
	for _, bd := range bounds {
		body := strings.Join(lines[bd.s:bd.e], "\n")
		if cwaFixtureNameRe.MatchString(bd.name) || cwaFixtureBodyRe.MatchString(body) {
			funcs[bd.name] = true
		}
	}
	return funcs
}

// cwaScanFile —— py scan_file：回该件的裸写点（已剔夹具函数）。
func cwaScanFile(path string) ([]cwaHit, error) {
	text, err := cwaReadText(path)
	if err != nil {
		return nil, err
	}
	lines := cwaSplitLines(text)
	fix := cwaFixtureFuncs(lines)
	var out []cwaHit
	fn := "<file-scope>"
	for i, line := range lines {
		if fm := cwaFuncRe.FindStringSubmatch(line); fm != nil {
			fn = fm[1]
		}
		if fix[fn] {
			continue
		}
		stripped := cwaPyStrip(line)
		if strings.HasPrefix(stripped, "//") {
			continue
		}
		if c := cwaWriteCallOf(line); c != "" {
			out = append(out, cwaHit{Line: i + 1, Func: fn, Call: c, Text: cwaPySliceRunes(stripped, 120)})
		}
	}
	return out, nil
}

// cwaDeclaredWriteCalls —— py declared_write_calls：本件登记表登记的 Call 集合。
func cwaDeclaredWriteCalls(path string) map[string]bool {
	calls := map[string]bool{}
	text, err := cwaReadText(path)
	if err != nil {
		return calls
	}
	for _, line := range cwaIterLines(text) {
		for _, m := range cwaCallRe.FindAllStringSubmatch(line, -1) {
			calls[m[1]] = true
		}
	}
	return calls
}

// cwaCatalogFuncsWithDanger —— py catalog_funcs_with_danger：main.go 里有 danger 的 run 函数名。
func cwaCatalogFuncsWithDanger(mainGo string) (map[string]bool, bool) {
	if fi, err := os.Stat(mainGo); err != nil || fi.IsDir() {
		return nil, false
	}
	text, err := cwaReadText(mainGo)
	if err != nil {
		return nil, false
	}
	funcs := map[string]bool{}
	for _, block := range cwaBlockRe.FindAllString(text, -1) {
		if strings.Contains(block, "danger:") {
			if m := cwaRunRe.FindStringSubmatch(block); m != nil {
				funcs[m[1]] = true
			}
		}
	}
	return funcs, true
}

func cwaCallMatches(declared map[string]bool, call string) bool {
	for d := range declared {
		if strings.Contains(d, call) || strings.ReplaceAll(d, " ", "") == strings.ReplaceAll(call, " ", "") {
			return true
		}
	}
	return false
}

// ── py json.dumps(ensure_ascii=False) 字符串字面 ────────────────────────────
func cwaJSONStr(s string) string {
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
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cwaRun —— py run(root, allow, as_json)。
func cwaRun(stdout, stderr io.Writer, root string, allow []string, asJSON bool) int {
	scopedir := cwaPyJoin(root, "core", "cmd", "zerg")
	if fi, err := os.Stat(scopedir); err != nil || !fi.IsDir() {
		fmt.Fprintln(stderr, cwaBlockedNoScope)
		return 2
	}
	mainGo := cwaPyJoin(scopedir, "main.go")
	dangerFuncs, ok := cwaCatalogFuncsWithDanger(mainGo)
	if !ok {
		fmt.Fprintln(stderr, cwaBlockedNoMain)
		return 2
	}
	// 豁免完整性（生产 EXEMPT 恒空 ⇒ 该支死；仍逐字保留）
	var bad []string
	for _, e := range cwaExempt {
		if e.File == "" || e.Call == "" || e.Reason == "" || e.Source == "" {
			bad = append(bad, "?")
		}
	}
	if len(bad) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 豁免条目不齐（理由/出处）：%s ⇒ rc=2\n", cwaPyReprList(bad))
		return 2
	}
	exempt := map[[2]string]bool{}
	for _, e := range cwaExempt {
		exempt[[2]string{e.File, e.Call}] = true
	}
	for _, a := range allow {
		if idx := strings.Index(a, ":"); idx >= 0 {
			exempt[[2]string{a[:idx], a[idx+1:]}] = true
		}
	}
	var hits []cwaHit
	names, _ := os.ReadDir(scopedir)
	var list []string
	for _, de := range names {
		list = append(list, de.Name())
	}
	sort.Strings(list)
	for _, name := range list {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := cwaPyJoin(scopedir, name)
		declared := cwaDeclaredWriteCalls(path)
		fileHits, err := cwaScanFile(path)
		if err != nil {
			continue
		}
		for _, h := range fileHits {
			if exempt[[2]string{name, h.Call}] {
				continue
			}
			if cwaCallMatches(declared, h.Call) {
				continue
			}
			if dangerFuncs[h.Func] {
				continue
			}
			h.File = name
			hits = append(hits, h)
		}
	}
	rc := 0
	if len(hits) > 0 {
		rc = 1
	}
	if asJSON {
		var b strings.Builder
		b.WriteString("{\"rc\": ")
		b.WriteString(fmt.Sprintf("%d", rc))
		b.WriteString(", \"bare_write_count\": ")
		b.WriteString(fmt.Sprintf("%d", len(hits)))
		b.WriteString(", \"hits\": [")
		for i, h := range hits {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{\"file\": " + cwaJSONStr(h.File))
			b.WriteString(", \"line\": " + fmt.Sprintf("%d", h.Line))
			b.WriteString(", \"func\": " + cwaJSONStr(h.Func))
			b.WriteString(", \"call\": " + cwaJSONStr(h.Call) + "}")
		}
		b.WriteString("]}")
		fmt.Fprintln(stdout, b.String())
	} else {
		fmt.Fprintln(stdout, cwaRunHeader)
		fmt.Fprintf(stdout, "扫面 core/cmd/zerg/*.go（非测试）· 裸写点（既无写点登记、所属命令面又无 danger）= %d 条\n", len(hits))
		for _, h := range hits {
			fmt.Fprintf(stdout, "  ✗ %s:%d func=%s call=%s ⇒ %s\n", h.File, h.Line, h.Func, h.Call, h.Text)
		}
		fmt.Fprintln(stdout, "  口径：写点前须有交代 —— 写点登记表命中 或 所属命令面在 catalog() 有 danger。")
	}
	return rc
}

// cwaPyReprList —— py `repr(list[str])`（仅死支用）。
func cwaPyReprList(items []string) string {
	var parts []string
	for _, s := range items {
		parts = append(parts, "'"+strings.ReplaceAll(s, "'", "\\'")+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cwaSelfTest —— py self_test()。
func cwaSelfTest(stdout, stderr io.Writer) int {
	ok := true
	cwaExempt = nil
	td, err := os.MkdirTemp("", "")
	if err != nil {
		fmt.Fprintf(stdout, "自检 FAIL（4 格）\n")
		return 1
	}
	defer os.RemoveAll(td)
	d := cwaPyJoin(td, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		fmt.Fprintf(stdout, "自检 FAIL（4 格）\n")
		return 1
	}
	// 夹具字面**收进本函数局部**（✗ 不放包级）⇒ 别被本件自己的扫描面判成裸写点。
	mainSrc := "var x = []C{{danger: &dangerSpec{...}, run: cmdGuardedX}}\n"
	bareSrc := "func bareWriter(inv *invocation) int {\n\t" + "os.WriteFile" + "(\"x\", b, 0o644)\n\treturn 0\n}\n"
	okSrc := "type w struct{ Call string }\n" +
		"var writePoints = []w{\n\t{Call: \"os.WriteFile(tmp, b, 0o644)\"},\n}\n" +
		"func okWriter() { " + "os.WriteFile" + "(tmp, b, 0o644) }\n"
	_ = os.WriteFile(cwaPyJoin(d, "main.go"), []byte(mainSrc), 0o644)
	_ = os.WriteFile(cwaPyJoin(d, "family_bare.go"), []byte(bareSrc), 0o644)
	_ = os.WriteFile(cwaPyJoin(d, "family_ok.go"), []byte(okSrc), 0o644)

	if cwaRun(stdout, stderr, td, nil, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 正控失败：应回 rc=1")
		ok = false
	}
	// 复核 hits 只含 family_bare.go
	var hitNames []string
	names, _ := os.ReadDir(d)
	var list []string
	for _, de := range names {
		list = append(list, de.Name())
	}
	sort.Strings(list)
	for _, name := range list {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "main.go" {
			continue
		}
		declared := cwaDeclaredWriteCalls(cwaPyJoin(d, name))
		fh, _ := cwaScanFile(cwaPyJoin(d, name))
		for _, h := range fh {
			if !cwaCallMatches(declared, h.Call) {
				hitNames = append(hitNames, name+"|"+h.Call)
			}
		}
	}
	if len(hitNames) != 1 || hitNames[0] != "family_bare.go|os.WriteFile" {
		fmt.Fprintf(stdout, "  ✗ 正控命中集不符：%s\n", cwaPyReprList(hitNames))
		ok = false
	}
	// 负控：豁免掉 ⇒ 0
	cwaExempt = []cwaExemptEntry{{File: "family_bare.go", Call: "os.WriteFile", Reason: "t", Source: "t"}}
	if cwaRun(stdout, stderr, td, nil, false) != 0 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：豁免未生效")
		ok = false
	}
	cwaExempt = nil
	// 负控 B：缺扫描面 ⇒ rc=2
	if cwaRun(stdout, stderr, cwaPyJoin(td, "nope"), nil, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控B失败：缺语料未回 rc=2")
		ok = false
	}
	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（4 格）\n", res)
	if ok {
		return 0
	}
	return 1
}

// ── argparse 仿（--root / --allow / --json / --self-test / -h）─────────────
type cwaArgs struct {
	root     string
	allow    []string
	asJSON   bool
	selfTest bool
}

// cwaArgErr —— 打用法行 + 报错行到 stderr，回 rc=2。
func cwaArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cwaUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", cwaProg, msg)
	return 2
}

func cwaParseArgs(argv []string) (cwaArgs, int, bool) {
	var a cwaArgs
	envRoot, haveEnv := os.LookupEnv("ZERG_ROOT")
	if haveEnv {
		a.root = envRoot
	} else if wd, err := os.Getwd(); err == nil {
		a.root = wd
	}
	resolve := func(p string) (string, bool) { // 唯一前缀 → 全名
		cands := []string{"--root", "--allow", "--json", "--self-test", "--help"}
		var hits []string
		for _, c := range cands {
			if strings.HasPrefix(c, p) {
				hits = append(hits, c)
			}
		}
		if len(hits) == 1 {
			return hits[0], true
		}
		return "", false
	}
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "-h" || arg == "--help" {
			fmt.Fprint(aRootOut, cwaHelpText)
			return a, 0, false
		}
		if strings.HasPrefix(arg, "--") {
			name := arg
			val := ""
			hasVal := false
			if eq := strings.Index(arg, "="); eq >= 0 {
				name = arg[:eq]
				val = arg[eq+1:]
				hasVal = true
			}
			full, okName := resolve(name)
			if !okName {
				return a, cwaArgErr(aStderr, "unrecognized arguments: "+arg), false
			}
			switch full {
			case "--root":
				if !hasVal {
					if i+1 >= len(argv) {
						return a, cwaArgErr(aStderr, "argument --root: expected one argument"), false
					}
					val = argv[i+1]
					i++
				}
				a.root = val
			case "--allow":
				if !hasVal {
					if i+1 >= len(argv) {
						return a, cwaArgErr(aStderr, "argument --allow: expected one argument"), false
					}
					val = argv[i+1]
					i++
				}
				a.allow = append(a.allow, val)
			case "--json":
				a.asJSON = true
			case "--self-test":
				a.selfTest = true
			case "--help":
				fmt.Fprint(aRootOut, cwaHelpText)
				return a, 0, false
			}
		} else {
			return a, cwaArgErr(aStderr, "unrecognized arguments: "+arg), false
		}
		i++
	}
	return a, 0, true
}

// aStdout/aStderr/aRootOut —— 解析期输出落点（由入口按入参设好，避免传参爆炸）。
var (
	aStdout  io.Writer = os.Stdout
	aStderr  io.Writer = os.Stderr
	aRootOut io.Writer = os.Stdout
)

// writeAuthzGateMain —— 入口（与 py main(argv) 对齐）。
func writeAuthzGateMain(stdout, stderr io.Writer, argv []string) int {
	cwaExempt = nil // ★ 包级登记面自清（防跨调用残留）
	aStdout, aStderr, aRootOut = stdout, stderr, stdout
	args, rc, cont := cwaParseArgs(argv)
	if !cont {
		return rc
	}
	if args.selfTest {
		return cwaSelfTest(stdout, stderr)
	}
	return cwaRun(stdout, stderr, args.root, args.allow, args.asJSON)
}

// 保留 encoding/json 引用（py 走 json 模块；本件手搓同构 JSON，此处仅为显式登记依赖面）。
var _ = json.Marshal
