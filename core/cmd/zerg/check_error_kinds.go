// check_error_kinds.go —— 门禁件 `scripts/gates/check-error-kinds.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 第一枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的**第一枚样板**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-error-kinds.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `errorKindsGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func errorKindsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 全绿 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：目录根输入（默认根 · 指定根 · 成对负控 · 空闭集 · 取不到输入）·
// `--json` 拒收（rc=2 · stderr）· `--self-test` 成对负控（6 格 · 含 R3 代码位/注释位成对对拍）。
// 未覆盖面（✗ 不声称逐字对齐）：`-h/--help` 的 argparse 帮助正文 · 未知旗标的 argparse usage 正文 ·
// `exitcodes.go` 缺件时的异常路径（py 抛 traceback ⇒ rc=1；本件落「退码表空」⇒ rc=2）。
//
//	—— 这三面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 判据正则（与 py 逐条对齐 · 名字前缀 `cek` 防包内同名）──────────────
var (
	// KIND_RE = ^[a-z][a-z0-9_]*$
	cekKindRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	// ROW_RE = ^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,   （re.M）
	cekRowRE = regexp.MustCompile(`(?m)^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,`)
	// USE_RE = setErr\(\s*"([^"]+)"
	cekUseRE = regexp.MustCompile(`setErr\(\s*"([^"]+)"`)
	// 退码表行 = ^\s*\{(\d+),\s*"
	cekDeclRE = regexp.MustCompile(`^\s*\{(\d+),\s*"`)
)

// cekRow —— `errorKinds` 闭集表的一行（与 py dict 三键同形）。
type cekRow struct {
	Kind      string
	Code      int
	Retryable bool
}

// errorKindsGateMain —— 入口（= py `main()`）。
func errorKindsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于位置参数解析）。
	if rc := errorKindsGateRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	// ② 位置参数等价解析（py argparse：root 可选 · 默认 "." · --self-test 布尔）。
	root := "."
	selfTest := false
	var positional []string
	for _, a := range argv {
		switch {
		case a == "--self-test":
			selfTest = true
		case strings.HasPrefix(a, "-") && a != "-":
			// 未知旗标：py argparse 也 rc=2（文案另论 · 见件头「未覆盖面」）。
			fmt.Fprintf(stderr, "usage: check-error-kinds.py [-h] [--self-test] [root]\n")
			fmt.Fprintf(stderr, "check-error-kinds.py: error: unrecognized arguments: %s\n", a)
			return 2
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 0 {
		root = positional[len(positional)-1]
	}
	if selfTest {
		return errorKindsGateSelfTest(stdout)
	}
	// ③ 找真源树。
	r, ok := errorKindsGateFindRoot(root)
	if !ok {
		fmt.Fprintln(stdout, "⛔ 不给结论：找不到 core/cmd/zerg/errors.go（本门只认命令面真源所在的那棵树）")
		return 2
	}
	rc, lines := errorKindsGateCheck(r)
	fmt.Fprintln(stdout, "── 命令面 error.kind 闭集（§九 M7）──")
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	switch rc {
	case 0:
		fmt.Fprintln(stdout, "⇒ 全绿（rc=0）")
	case 1:
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：kind 闭集或命名纪律被破坏")
	default:
		fmt.Fprintln(stdout, "⇒ 不给结论（rc=2）")
	}
	return rc
}

// errorKindsGateRefuseJSON —— py `refuse_json_without_machine_face()`。
// 给了 `--json` ⇒ 逐字说明后返 2；没给 ⇒ 返 nil（原路照走）。
func errorKindsGateRefuseJSON(stderr io.Writer, argv []string) *int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	// JSON_FIELDS 为空 ⇒ 字段表固定为「无（本门只出人读面）」（唯一真源：py 的 JSON_FIELDS）。
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// errorKindsGateFindRoot —— py `find_root()`（自 start 向上 ≤8 层找 core/cmd/zerg/errors.go）。
func errorKindsGateFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "errors.go")); err == nil && !fi.IsDir() {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// errorKindsGateParseClosedSet —— py `parse_closed_set()`（只读真源，不另抄一份）。
func errorKindsGateParseClosedSet(text string) []cekRow {
	start := strings.Index(text, "var errorKinds = []errorKindRow{")
	if start < 0 {
		return nil
	}
	rel := strings.Index(text[start:], "\n}")
	if rel < 0 {
		return nil
	}
	seg := text[start : start+rel]
	var rows []cekRow
	for _, m := range cekRowRE.FindAllStringSubmatch(seg, -1) {
		code, _ := strconv.Atoi(m[2])
		rows = append(rows, cekRow{Kind: m[1], Code: code, Retryable: m[3] == "true"})
	}
	return rows
}

// cekStripGoComments —— py `strip_go_comments()`：剥掉 Go 的 `//` 行注释与 `/* … */` 块注释
// （**保留字符串 / 字符 / 原生字符串字面量内的字符**）。
//
// 目的 = 让 R3 只认**代码位**的 kind 用法：散文 / 注释里提一句 `setErr("…")` 是文档，不是
// 「用了个闭集外的 kind」（本仓大量门脚本的自指说明正是这种形态 ⇒ 不剥即误报）。
// ★ 不剥字符串字面量：`setErr("<kind>"…)` 的 kind 本身就写在字符串里，剥了就等于空转。
func cekStripGoComments(text string) string {
	var out strings.Builder
	i, n := 0, len(text)
	var quote byte // 0 = 不在字面量内；'"' / '\'' / '`'
	for i < n {
		c := text[i]
		if quote != 0 {
			out.WriteByte(c)
			if quote == '`' {
				if c == '`' {
					quote = 0
				}
			} else if c == '\\' {
				if i+1 < n {
					out.WriteByte(text[i+1])
					i += 2
					continue
				}
			} else if c == quote {
				quote = 0
			}
			i++
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			out.WriteByte(c)
			i++
			continue
		}
		if c == '/' && i+1 < n && text[i+1] == '/' {
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				break
			}
			out.WriteByte('\n')
			i += j + 1
			continue
		}
		if c == '/' && i+1 < n && text[i+1] == '*' {
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				i = n
			} else {
				i = i + 2 + j + 2
			}
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

// errorKindsGateCheck —— py `check()`：返回 (rc, 报告行列表)。rc 0=绿 1=红 2=不给结论。
func errorKindsGateCheck(root string) (int, []string) {
	var errs []string

	rows, err := errorKindsGateReadClosedSet(filepath.Join(root, "core", "cmd", "zerg", "errors.go"))
	if err != nil {
		// py 在此处会抛异常 ⇒ rc=1；本件按「闭集表读不到 = 空转」落 rc=2（见件头「未覆盖面」）。
		return 2, []string{"⛔ 不给结论：闭集表为空（`errorKinds` 一个 kind 都没解析到 ⇒ 空转 = 假覆盖）"}
	}
	if len(rows) == 0 {
		return 2, []string{"⛔ 不给结论：闭集表为空（`errorKinds` 一个 kind 都没解析到 ⇒ 空转 = 假覆盖）"}
	}

	seen := map[string]cekRow{}
	for _, r := range rows {
		if !cekKindRE.MatchString(r.Kind) {
			errs = append(errs, fmt.Sprintf("R1 kind 命名不合规（必须 lower_snake_case）：'%s'", r.Kind))
		}
		if _, dup := seen[r.Kind]; dup {
			errs = append(errs, fmt.Sprintf("R2 kind 重复：%s", r.Kind))
		}
		seen[r.Kind] = r
		if r.Code < 0 {
			errs = append(errs, fmt.Sprintf("R2 退码为负：%s → %d", r.Kind, r.Code))
		}
	}

	// R2 的另一半：每个 kind 的码必须在退码表里登记过。
	declared, err := errorKindsGateReadDeclaredCodes(filepath.Join(root, "core", "cmd", "zerg", "exitcodes.go"))
	if err != nil || len(declared) == 0 {
		return 2, []string{"⛔ 不给结论：退码表一个码都没解析到（空转 = 假覆盖）"}
	}
	for _, r := range rows {
		if !declared[r.Code] {
			errs = append(errs, fmt.Sprintf("R2 kind '%s' 用的码 %d **不在退码表里**（先登记号再启用：占号纪律 §九 M7 `E8`）", r.Kind, r.Code))
		}
	}

	// R3 用法面。
	dir := filepath.Join(root, "core", "cmd", "zerg")
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			names = append(names, n)
		}
	}
	sort.Strings(names) // py: for name in sorted(os.listdir(dirpath))
	used := map[string]bool{}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		// 只认**代码位**：先剥注释（注释里提一句 setErr("…") 是散文不是用法；对应 py 的 strip_go_comments）。
		txt := cekStripGoComments(string(b))
		for _, m := range cekUseRE.FindAllStringSubmatch(txt, -1) {
			used[m[1]] = true
		}
	}
	delete(used, "kind")
	var usedSorted []string
	for u := range used {
		usedSorted = append(usedSorted, u)
	}
	sort.Strings(usedSorted)
	for _, u := range usedSorted {
		if _, ok := seen[u]; !ok {
			errs = append(errs, fmt.Sprintf("R3 用到闭集外的 kind：'%s'（先登记进 `errorKinds` 再用）", u))
		}
	}

	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("  kind=%-22s code=%-4d retryable=%-5s", r.Kind, r.Code, cekPyBool(r.Retryable)))
	}
	lines = append(lines, fmt.Sprintf("闭集 %d 个 kind · 用法面命中 %d 个字面量", len(rows), len(used)))
	if len(errs) > 0 {
		for _, e := range errs {
			lines = append(lines, "✗ "+e)
		}
		return 1, lines
	}
	return 0, lines
}

// errorKindsGateReadClosedSet —— 读 errors.go 并解析闭集（py 内联的 open+parse）。
func errorKindsGateReadClosedSet(path string) ([]cekRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return errorKindsGateParseClosedSet(string(b)), nil
}

// errorKindsGateReadDeclaredCodes —— py 内联：逐行 `^\s*\{(\d+),\s*"` 取已登记退码集。
func errorKindsGateReadDeclaredCodes(path string) (map[int]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[int]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if m := cekDeclRE.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			out[n] = true
		}
	}
	return out, nil
}

// cekPyBool —— py `str(bool)` ⇒ "True"/"False"（✗ 不是 Go 的 "true"/"false"）。
func cekPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cekPyReprStr —— 仅覆盖本件用到的字符串形态（含 ' 不含 " ⇒ py repr 用双引号）。
func cekPyReprStr(s string) string {
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + esc + "\""
	}
	return "'" + esc + "'"
}

// cekPyReprList —— py 列表 repr：`[repr(a), repr(b)]`（供 --self-test 逐字回放）。
func cekPyReprList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cekPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// errorKindsGateSelfTest —— py `self_test()`：成对负控（好件必绿 · 坏件必红 · 空转不给结论 · 未登记码必红）。
func errorKindsGateSelfTest(stdout io.Writer) int {
	ok := true
	mk := func(body string) string {
		t, err := os.MkdirTemp("", "cek")
		if err != nil {
			return ""
		}
		p := filepath.Join(t, "core", "cmd", "zerg")
		_ = os.MkdirAll(p, 0o755)
		_ = os.WriteFile(filepath.Join(p, "errors.go"), []byte(body), 0o644)
		_ = os.WriteFile(filepath.Join(p, "exitcodes.go"), []byte("var exitcodeTable = []exitcodeRow{\n"+
			"\t{0, \"ok\", \"成功\", \"—\"},\n\t{1, \"failed\", \"一般失败\", \"看 kind\"},\n"+
			"\t{2, \"usage\", \"用法错\", \"改用法\"},\n\t{14, \"conflict\", \"冲突\", \"排队\"},\n}\n"), 0o644)
		return t
	}
	filter := func(lines []string, sub string) []string {
		var out []string
		for _, l := range lines {
			if strings.Contains(l, sub) {
				out = append(out, l)
			}
		}
		return out
	}

	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, _ := errorKindsGateCheck(mk(good))
	fmt.Fprintf(stdout, "  自检① 好件（lower_snake）⇒ rc=%d（要 0）\n", rc)
	ok = ok && rc == 0

	bad := "var errorKinds = []errorKindRow{\n\t{\"CONFIRM_REQUIRED\", 2, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, lines := errorKindsGateCheck(mk(bad))
	fmt.Fprintf(stdout, "  自检② 大写 kind ⇒ rc=%d（要 1）· 命中: %s\n", rc, cekPyReprList(filter(lines, "R1")))
	ok = ok && rc == 1

	empty := "var errorKinds = []errorKindRow{\n}\n"
	rc, _ = errorKindsGateCheck(mk(empty))
	fmt.Fprintf(stdout, "  自检③ 空闭集 ⇒ rc=%d（要 2 —— 空转 = 假覆盖，不给结论）\n", rc)
	ok = ok && rc == 2

	ghost := "var errorKinds = []errorKindRow{\n\t{\"ghost_kind\", 99, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, lines = errorKindsGateCheck(mk(ghost))
	fmt.Fprintf(stdout, "  自检④ 未登记的码 99 ⇒ rc=%d（要 1）· %s\n", rc, cekPyReprList(filter(lines, "R2")))
	ok = ok && rc == 1

	// R3 成对负控（真写错 kind 必须抓到）：代码位写了个闭集外的 kind ⇒ 红。
	r3bad := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\nfunc f() { setErr(\"bogus_kind\", \"x\", \"y\") }\n"
	rc, lines = errorKindsGateCheck(mk(r3bad))
	fmt.Fprintf(stdout, "  自检⑤ R3 代码位用了闭集外的 kind ⇒ rc=%d（要 1）· %s\n", rc, cekPyReprList(filter(lines, "R3")))
	ok = ok && rc == 1

	// R3 反误报对拍：同一句只写在**注释**里 ⇒ 不红（注释已剥）。
	r3comment := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n// 说明：勿用 setErr(\"bogus_kind\", ...)（散文，不是用法）\n"
	rc, _ = errorKindsGateCheck(mk(r3comment))
	fmt.Fprintf(stdout, "  自检⑥ R3 注释位出现同句 ⇒ rc=%d（要 0 —— 注释剥掉后不误报）\n", rc)
	ok = ok && rc == 0

	if ok {
		fmt.Fprintln(stdout, "自检结论: 全过")
	} else {
		fmt.Fprintln(stdout, "自检结论: **有断言失败**")
	}
	if ok {
		return 0
	}
	return 1
}
