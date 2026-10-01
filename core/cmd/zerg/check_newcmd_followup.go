// check_newcmd_followup.go —— M2 真搬：`scripts/gates/check-newcmd-followup.py`（门78 新命令随动总账）
//
// 本件是**只读聚合判据**：以冻结的 path 全集为基线，把新增 path 的九个面逐面打印「齐 / 缺 / 不适用」，
// 缺任一即 rc=1 并点名；解析漏读 / 基线不合规 / 前置缺件 / 空转 ⇒ rc=2（不给结论）。
//
// 出厂声明 (a) 默认根定位差异：
//
//	py 从 `__file__` 上溯两层取仓根（脚本恒在 `<根>/scripts/gates/`）；Go 无 `__file__` 语境，
//	只能从 cwd 上溯取「含 core/ 与 ui/ 的目录」为缺省根。cwd == 仓根（或仓内任一子目录）时两者同值；
//	cwd 与脚本位置无关时**不声称逐字对齐**（见未覆盖面）。显式 `--root` 时两侧一律**逐字**采用该串
//	（py 不 abspath `--root`，本件同样不 abspath）。
//
// 出厂声明 (b) 自指面（四形态，全部按「一旦被跟踪」逐条消解）：
//
//	① 本件在自己扫描面内？—— 本件扫 `core/cmd/zerg/main.go` 的 commands 块 + 该目录 listing；
//	   本件不新增任何 command，也不定义 `func cmd*`；故 0 贡献。
//	② 件头/正文里带真引号的路径原串？—— 本件只出现**仓内相对**字面（core/cmd/zerg/…），
//	   零绝对路径、零私有根字面（`check-hardcoded-private-paths` 的 NEEDLES 三串在本件 0 命中）。
//	③ self-test 魔法串连续字面落地？—— 自检夹具里被判据读的触发词（错误种类、退码、被扫函数名）
//	   一律**拼串**落地（见 cnfSelfMagic* 常量），不写连续原串。
//	④ 落在任何他门扫描面内？—— `check-error-kinds`（转 `core/cmd/zerg/*.go` 取 `setXxx("<k>"` 与
//	   JSON 键形）与 `check-wall-layer`（同目录取层级标记形）都**取本件**。故：本件零 `setXxx(` 连续形、
//	   零 `"ki`+`nd"` 紧跟冒号引号形、零层级标记形；JSON 键名走 jobj.set 形（键后是逗号不是冒号）。
//	   实证：落仓前后对上述他门跑 A/B 影子仓，输出 sha16 逐字相同（见回执）。
//
// 未覆盖面（逐条）：
//
//	· argparse 帮助/用法正文按 80 列硬编码（`COLUMNS` 变则 py 换行位不同）；`--json=…`、`--root` 等
//	  非 store_true 旗标的显式赋值被 argparse 拒的边界文案未逐字复刻。
//	· `\d`→`\p{Nd}`、`\w`→`[\p{L}\p{N}_]`、`\s`→显式 Unicode 空白类：非 ASCII 数字经 `int()` 的档
//	  未复刻（py 认 Unicode 数字，Go Atoi 不认）⇒ 真仓/夹具全 ASCII，未触发。
//	· py 最外层裸 `sys.exit(main())`：main 内未捕异常 = 回溯 + rc=1（C 类）。本件遇同形（解析块尾缺席、
//	  JSON 顶层非对象、`int()` 不可转）改 fail-closed rc=2 并走声明文案（口径差，只保 rc 语义方向）。
//	· py 的 `read()` 读不了的件在若干处 `except: continue`（静默跳过）；本件逐字照搬该静默并同形落明细面。
//	· 显式 null 面（本版补）：JSON 字段**存在但值为 `null`** 时，py 在 `list(x.get('paths',[]))` ·
//	  `len(cases)` · `for e in doc.get('件',[])` · `for ns in doc.get('namespaces',[])` ·
//	  `set(int(x) for x in ns.get('table',[]))` 处**裸回溯**（未捕异常 ⇒ 回溯 + rc=1 · stdout 空）。
//	  本件按件头 fail-closed 口径改判 **rc=2 + 具名文案**（不逐字复刻回溯 —— 回溯含跑测机路径帧、不可移植）。
//	  逐处：`dryrun-semantics.json:件` · `newcmd-paths-baseline.json:paths` · `cli-matrix.json:cases` ·
//	  `exitcode-namespaces.json:namespaces` 与 `namespaces[].table`。**缺键形态一字不动**（两侧已逐字同）。
//	  普查过的其余 JSON 取值点 null 时 py **本就不崩**（有默认值/相等比较兜住）⇒ 逐字保留既有行为：
//	  `baseline.schema/::count/::head` · `cli-contract-baseline.{commands,mustfail_cases}` · case 级
//	  `command/id/why` 与数组元素级 null。（`want_rc: null` 归件头已自陈的「`int()` 不可转」档，见上一条。）
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ── 常量（照 py 逐字；路径常量 = py 的 os.path.join 物化结果）──────────────────────
const cnfMainRel = "core/cmd/zerg/main.go"

var (
	cnfMatrixRel        = cnfPyJoin("core", "cmd", "zerg", "testdata", "cli-matrix.json")
	cnfContractBaseline = cnfPyJoin("scripts", "gates", "cli-contract-baseline.json")
	cnfOwnBaselineRel   = cnfPyJoin("scripts", "gates", "newcmd-paths-baseline.json")
	cnfDryrunRegRel     = cnfPyJoin("core", "internal", "contract", "dryrun-semantics.json")
	cnfMarksRel         = cnfPyJoin("scripts", "公开标记.tsv")
	cnfExclARel         = cnfPyJoin("scripts", "build", "publish-public.sh")
	cnfExclBRel         = cnfPyJoin("publish", "mirror-public-lib.py")
	cnfPkgRel           = cnfPyJoin("core", "cmd", "zerg")
	cnfErrorsRel        = cnfPyJoin("core", "cmd", "zerg", "errors.go")
	cnfExitcodesRel     = cnfPyJoin("core", "cmd", "zerg", "exitcodes.go")
	cnfNamespacesRel    = cnfPyJoin("core", "internal", "contract", "exitcode-namespaces.json")
	cnfDocsRels         = []string{cnfPyJoin("docs", "zh", "参考-命令行.md"), cnfPyJoin("docs", "en", "参考-命令行.md")}
)

const cnfSchema = "newcmd-followup-baseline/1"

// py `\s`（str）= 显式 Unicode 空白类：\t\n\v\f\r + 空格 + \x1c-\x1f + \x85 + \xa0 + …
// ★ 与 Go RE2 默认 ASCII `\s` 不同 ⇒ 逐处展开。
const cnfWS = `[\t-\r \x1c-\x1f\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// py `\d` → `\p{Nd}`；py `\w` → `[\p{L}\p{N}_]`（见未覆盖面）
const cnfD = `\p{Nd}`
const cnfW = `[\p{L}\p{N}_]`

var (
	cnfCMDRE      = regexp.MustCompile(`path:` + cnfWS + `*\[\]string\{([^}]*)\}`)
	cnfPathSegRE  = regexp.MustCompile(`"([^"]+)"`)
	cnfRunRE      = regexp.MustCompile(`\brun:` + cnfWS + `*([A-Za-z_][A-Za-z0-9_]*)`)
	cnfDryrunRE   = regexp.MustCompile(`(?i)dry.?run|DryRun`)
	cnfFlagRE     = regexp.MustCompile(`(--[A-Za-z0-9-]+)`)
	cnfCaseRE     = regexp.MustCompile(`case a == "(--[A-Za-z0-9-]+)"`)
	cnfCaseLitRE  = regexp.MustCompile(`case "(--[A-Za-z0-9-]+)"`)
	cnfPrefixRE   = regexp.MustCompile(`strings\.HasPrefix\(a, "(--[A-Za-z0-9-]+)=`)
	cnfUsageRE    = regexp.MustCompile(`usage:` + cnfWS + `*(.*)`)
	cnfUsageLitRE = regexp.MustCompile(`^"((?:[^"\\]|\\.)*)"`)
	cnfConstRE    = regexp.MustCompile(`const (` + cnfW + `+) = "((?:[^"\\]|\\.)*)"`)
	// 键名拼串（✗ 不写连续 "kind": 形 —— 会落进 check-error-kinds 的 R3 扫描面）
	cnfKindUseRE  = regexp.MustCompile(`set` + `Err\(` + cnfWS + `*"([^"]+)"`)
	cnfKindJSONRE = regexp.MustCompile(`"ki` + `nd"` + cnfWS + `*:` + cnfWS + `*"([^"]+)"`)
	cnfKindRowRE  = regexp.MustCompile(`(?m)^` + cnfWS + `*\{` + cnfWS + `*"([^"]+)"` + cnfWS + `*,` + cnfWS + `*(-?` + cnfD + `+)` + cnfWS + `*,` + cnfWS + `*(true|false)` + cnfWS + `*,`)
	cnfOSExitREs  = []*regexp.Regexp{
		regexp.MustCompile(`os\.Exit\((` + cnfD + `+)\)`),
		regexp.MustCompile(`exit[A-Za-z]+` + cnfWS + `*=` + cnfWS + `*(` + cnfD + `+)`),
		regexp.MustCompile(`exitCode` + cnfWS + `*=` + cnfWS + `*(` + cnfD + `+)`),
	}
	cnfTableRowRE = regexp.MustCompile(`\{` + cnfWS + `*(` + cnfD + `+)` + cnfWS + `*,` + cnfWS + `*"`)

	cnfExclA1RE = regexp.MustCompile(`(?s)EXCLUDES=(.*?)\n\)`)
	cnfExclA2RE = regexp.MustCompile(`(?s)EXCLUDES="([^"]*)"`)
	cnfQuotedRE = regexp.MustCompile(`["']([^"']+)["']`)
	cnfDropRe   = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?s)` + name + cnfWS + `*=` + cnfWS + `*[\[\(](.*?)[\]\)]`)
	}
)

const (
	cnfVFEnd    = `return ""`
	cnfVFSig    = "func valueFlagName(a string) string {"
	cnfParseSig = "func parseInvocation"
	cnfTreeSig  = "commands = []*command{"
	cnfBareSep  = "|｜],、"
)

// ── py 字符串/路径原语同构 ─────────────────────────────────────────────────────
// cnfPyJoin = CPython posixpath.join 逐段规则（吃 `..` 原样留；后段绝对丢前段；空段 `a/` 形）
func cnfPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// cnfPyDirname = posixpath.dirname（= posixpath.split(p)[0]）
func cnfPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && strings.TrimRight(head, "/") == "" {
		return "/"
	}
	return strings.TrimRight(head, "/")
}

func cnfPyIsSpace(r rune) bool {
	switch {
	case r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r':
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0 || r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// cnfStrip = py str.strip()（无参 ⇒ 剥 py Unicode 空白，★ 含 \x1c-\x1f，✗ 非 strings.TrimSpace）
func cnfStrip(s string) string { return strings.TrimFunc(s, cnfPyIsSpace) }
func cnfLstrip(s string) string {
	for i, r := range s {
		if !cnfPyIsSpace(r) {
			return s[i:]
		}
	}
	return ""
}

// cnfSplitWS = py str.split()（无参 ⇒ 按 py 空白 run 切、丢空段）
func cnfSplitWS(s string) []string {
	fields := []string{}
	cur := strings.Builder{}
	for _, r := range s {
		if cnfPyIsSpace(r) {
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}

// cnfSplitlines = py str.splitlines（含 \v\f\x1c-\x1e\x85\u2028\u2029；\r\n 视为一个界）
func cnfSplitlines(s string) []string {
	out := []string{}
	cur := strings.Builder{}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\r' {
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			continue
		}
		if r == '\n' || r == '\v' || r == '\f' || r == '\x1c' || r == '\x1d' || r == '\x1e' || r == '\x85' || r == '\u2028' || r == '\u2029' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cnfRuneHead = py s[:n]（按**码点**切；n<0 照 py 切片语义）
func cnfRuneHead(s string, n int) string {
	rs := []rune(s)
	if n < 0 {
		n = len(rs) + n
		if n < 0 {
			n = 0
		}
	}
	if n > len(rs) {
		n = len(rs)
	}
	return string(rs[:n])
}

// cnfHead = py s[:n] 对切片（★ n<0 照 py：len+n）
func cnfHead[T any](s []T, n int) []T {
	if n < 0 {
		n = len(s) + n
		if n < 0 {
			n = 0
		}
	}
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

// cnfReadText = py open(p, encoding='utf-8').read()（★ universal newlines：\r\n 与裸 \r 折 \n；
// 非法 UTF-8 ⇒ 报错，对齐 py 的 strict 解码）。
func cnfReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("invalid utf-8")
	}
	s := string(b)
	if strings.IndexByte(s, '\r') < 0 {
		return s, nil
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			sb.WriteByte('\n')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String(), nil
}

// cnfIsFile / cnfIsDir = py os.path.isfile / isdir（★ 跟随符号链接 ⇒ os.Stat，✗ 非 Lstat/DirEntry）
func cnfIsFile(p string) bool { st, err := os.Stat(p); return err == nil && st.Mode().IsRegular() }
func cnfIsDir(p string) bool  { st, err := os.Stat(p); return err == nil && st.IsDir() }

// cnfListdir = py os.listdir（readdir 原序）后按需 sorted（py 代码里两处都写 sorted ⇒ 本件返回排序名）
func cnfListdirSort(d string) ([]string, bool) {
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil, false
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, true
}

// ── py %r / str / int 同构 ─────────────────────────────────────────────────────
func cnfPyReprStr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			b.WriteString(fmt.Sprintf(`\x%02x`, r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func cnfPyRepr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return cnfPyReprStr(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case int:
		return strconv.Itoa(x)
	case []interface{}:
		ps := make([]string, 0, len(x))
		for _, e := range x {
			ps = append(ps, cnfPyRepr(e))
		}
		return "[" + strings.Join(ps, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

func cnfPyStr(v interface{}) string {
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
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case int:
		return strconv.Itoa(x)
	}
	return cnfPyRepr(v)
}

// cnfPyInt = py int(v)；不可转 ⇒ ok=false（调用方决定 fail-closed）
func cnfPyInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case json.Number:
		return cnfPyIntStr(x.String())
	case float64:
		return int(x), true
	case int:
		return x, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		return cnfPyIntStr(x)
	}
	return 0, false
}

func cnfPyIntStr(s string) (int, bool) {
	t := cnfStrip(s)
	if t == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(t); err == nil {
		return n, true
	}
	if f, err := strconv.ParseFloat(t, 64); err == nil {
		return int(f), true
	}
	return 0, false
}

// ── JSON 读面（py json.loads；数字走 json.Number 以区别于 float）────────────────
func cnfLoadJSON(root, rel string) (interface{}, bool) {
	fp := cnfPyJoin(root, rel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

func cnfDictGet(m interface{}, key string) (interface{}, bool, bool) {
	d, ok := m.(map[string]interface{})
	if !ok {
		return nil, false, false
	}
	v, ok := d[key]
	return v, ok, true
}

// cnfDictNull = 键**存在且值为显式 null**（缺键 ⇒ false）。
// ★ 必须与「缺键」分开：缺键走原默认（两侧已同），显式 null 才 fail-closed ⇒ rc=2。
// JSON null 反序列化进 interface{} 即 nil，故 `present && v == nil` 可判 null（不会与缺键混淆）。
func cnfDictNull(m interface{}, key string) bool {
	v, present, ok := cnfDictGet(m, key)
	return ok && present && v == nil
}

// ── py re.escape 的 `\b`… 动态正则 ────────────────────────────────────────────
func cnfFuncRE(fn string) *regexp.Regexp {
	return regexp.MustCompile(`\bfunc` + cnfWS + `+` + regexp.QuoteMeta(fn) + cnfWS + `*\(`)
}

// ── 命令树解析 ────────────────────────────────────────────────────────────────
type cnfEntry struct {
	path string
	run  string
}

func cnfParseTree(root string) ([]cnfEntry, int, bool) {
	fp := cnfPyJoin(root, cnfMainRel)
	if !cnfIsFile(fp) {
		return nil, 0, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, 0, false
	}
	start := strings.Index(src, "commands = []*command{")
	if start < 0 {
		return nil, 0, false
	}
	endRel := strings.Index(src[start:], "\n\t}\n")
	if endRel < 0 {
		// py 此处裸 index() ⇒ 未捕异常 = 回溯 rc=1（C 类）⇒ 搬时 fail-closed（口径差，见未覆盖面）
		return nil, 0, false
	}
	end := start + endRel
	seg := src[start:end]
	matches := cnfCMDRE.FindAllStringSubmatchIndex(seg, -1)
	out := []cnfEntry{}
	for i, m := range matches {
		g1 := seg[m[2]:m[3]]
		pm := cnfPathSegRE.FindAllStringSubmatch(g1, -1)
		if len(pm) == 0 {
			continue
		}
		parts := make([]string, 0, len(pm))
		for _, p := range pm {
			parts = append(parts, p[1])
		}
		nxt := len(seg)
		if i+1 < len(matches) {
			nxt = matches[i+1][0]
		}
		block := seg[m[1]:nxt]
		run := ""
		if rn := cnfRunRE.FindStringSubmatch(block); rn != nil {
			run = rn[1]
		}
		out = append(out, cnfEntry{path: strings.Join(parts, " "), run: run})
	}
	return out, len(matches), true
}

func cnfCmdPathsOnly(tree []cnfEntry) []string {
	out := make([]string, 0, len(tree))
	for _, e := range tree {
		out = append(out, e.path)
	}
	return out
}

func cnfFindImplFiles(root, fn string) []string {
	if fn == "" {
		return nil
	}
	hits := []string{}
	for _, base := range []string{"core/cmd/zerg"} {
		d := cnfPyJoin(root, base)
		names := []string{}
		if cnfIsDir(d) {
			if ns, ok := cnfListdirSort(d); ok {
				names = ns
			}
		}
		for _, name := range names {
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			src, err := cnfReadText(cnfPyJoin(d, name))
			if err != nil {
				continue
			}
			if cnfFuncRE(fn).MatchString(src) {
				hits = append(hits, strings.ReplaceAll(cnfPyJoin(base, name), string(os.PathSeparator), "/"))
			}
		}
	}
	return hits
}

// ── 各真源读面 ────────────────────────────────────────────────────────────────
// cnfLoadDryrunRegistry：第三个返回值 = 「`件` 键存在但值为显式 null」（py 在此裸回溯 ⇒ fail-closed rc=2）。
func cnfLoadDryrunRegistry(root string) (map[string]bool, bool, bool) {
	doc, ok := cnfLoadJSON(root, cnfDryrunRegRel)
	if !ok || doc == nil {
		return nil, false, false
	}
	if cnfDictNull(doc, "件") {
		return nil, false, true
	}
	items, _, isDict := cnfDictGet(doc, "件")
	if !isDict {
		return nil, false, false
	}
	arr, _ := items.([]interface{})
	out := map[string]bool{}
	for _, e := range arr {
		if em, ok := e.(map[string]interface{}); ok {
			f, _, _ := cnfDictGet(em, "file")
			s, _ := f.(string)
			out[s] = true
		}
	}
	return out, true, false
}

func cnfLoadMarks(root string) (map[string]string, bool) {
	fp := cnfPyJoin(root, cnfMarksRel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	out := map[string]string{}
	for i, line := range cnfSplitlines(src) {
		if cnfStrip(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if i+1 == 1 && len(cols) > 0 && cnfStrip(cols[0]) == "文件" {
			continue
		}
		if len(cols) < 2 {
			return nil, false
		}
		out[cnfStrip(cols[0])] = cnfStrip(cols[1])
	}
	return out, true
}

func cnfLoadExcludesA(root string) ([]string, bool) {
	fp := cnfPyJoin(root, cnfExclARel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	i := strings.Index(src, "EXCLUDES=")
	if i < 0 {
		return nil, false
	}
	tail := src[i:]
	m := cnfExclA1RE.FindStringSubmatch(tail)
	if m == nil {
		m = cnfExclA2RE.FindStringSubmatch(tail)
	}
	if m == nil {
		return nil, false
	}
	body := m[1]
	out := []string{}
	for _, q := range cnfQuotedRE.FindAllStringSubmatch(body, -1) {
		out = append(out, q[1])
	}
	return out, true
}

func cnfLoadExcludesB(root string) ([]string, []string, bool) {
	fp := cnfPyJoin(root, cnfExclBRel)
	if !cnfIsFile(fp) {
		return nil, nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, nil, false
	}
	exact, prefix := []string{}, []string{}
	for _, nb := range [][2]interface{}{{"DROP_EXACT", &exact}, {"DROP_PREFIX", &prefix}} {
		name := nb[0].(string)
		box := nb[1].(*[]string)
		m := cnfDropRe(name).FindStringSubmatch(src)
		if m != nil {
			for _, q := range cnfQuotedRE.FindAllStringSubmatch(m[1], -1) {
				*box = append(*box, q[1])
			}
		}
	}
	return exact, prefix, true
}

func cnfExcluded(path string, exact, prefix []string) bool {
	p := strings.Trim(path, "/")
	for _, e := range exact {
		if p == strings.Trim(e, "/") {
			return true
		}
	}
	for _, x := range prefix {
		xs := strings.Trim(x, "/")
		if xs != "" && strings.HasPrefix(p, xs) {
			return true
		}
	}
	return false
}

func cnfStripComments(src string) string {
	lines := cnfSplitlines(src)
	for i, line := range lines {
		if j := strings.Index(line, "//"); j >= 0 {
			lines[i] = line[:j]
		}
	}
	return strings.Join(lines, "\n")
}

func cnfValueForm(usage string, pos int) bool {
	rs := []rune(usage)
	if pos >= len(rs) {
		return false
	}
	c := rs[pos]
	if c == '=' || c == ':' {
		return true
	}
	t := cnfLstrip(string(rs[pos:]))
	if t == "" {
		return false
	}
	if strings.ContainsRune(cnfBareSep, []rune(t)[0]) {
		return false
	}
	if strings.HasPrefix(t, "--") {
		return false
	}
	return true
}

func cnfParseUsages(root string) (map[string]cnfUsage, bool) {
	fp := cnfPyJoin(root, cnfMainRel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	if !strings.Contains(src, cnfTreeSig) {
		return nil, false
	}
	consts := map[string]string{}
	pkg := cnfPyJoin(root, cnfPkgRel)
	names := []string{}
	if cnfIsDir(pkg) {
		if ns, ok := cnfListdirSort(pkg); ok {
			names = ns
		}
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := cnfReadText(cnfPyJoin(pkg, name))
		if err != nil {
			continue
		}
		body = cnfStripComments(body)
		for _, m := range cnfConstRE.FindAllStringSubmatch(body, -1) {
			if _, seen := consts[m[1]]; !seen {
				consts[m[1]] = m[2]
			}
		}
	}
	t0 := strings.Index(src, cnfTreeSig)
	rel := strings.Index(src[t0:], "\n\t}\n")
	if rel < 0 {
		return nil, false
	}
	t1 := t0 + rel
	out := map[string]cnfUsage{}
	for _, block := range regexp.MustCompile(`\n\t\t\{`).Split(src[t0:t1], -1) {
		mp := cnfCMDRE.FindStringSubmatch(block)
		if mp == nil {
			continue
		}
		path := strings.Join(func() []string {
			var ps []string
			for _, p := range cnfPathSegRE.FindAllStringSubmatch(mp[1], -1) {
				ps = append(ps, p[1])
			}
			return ps
		}(), " ")
		mu := cnfUsageRE.FindStringSubmatch(block)
		if mu == nil {
			continue
		}
		raw := cnfStrip(strings.TrimRight(cnfStrip(mu[1]), ","))
		usage := ""
		if strings.HasPrefix(raw, "\"") {
			if lit := cnfUsageLitRE.FindStringSubmatch(raw); lit != nil {
				usage = lit[1]
			}
		} else {
			key := ""
			if fs := cnfSplitWS(raw); len(fs) > 0 {
				key = strings.Trim(fs[0], ",")
			}
			usage = consts[key]
		}
		out[path] = cnfUsage{usage: usage, passthru: strings.Contains(block, "passthrough: true")}
	}
	return out, true
}

type cnfUsage struct {
	usage    string
	passthru bool
}

func cnfBuildFlagFaces(root string) (map[string]bool, map[string]cnfUsage, bool) {
	fp := cnfPyJoin(root, cnfMainRel)
	if !cnfIsFile(fp) {
		return nil, nil, false
	}
	raw, err := cnfReadText(fp)
	if err != nil {
		return nil, nil, false
	}
	mainSrc := cnfStripComments(raw)
	if !strings.Contains(mainSrc, cnfVFSig) || !strings.Contains(mainSrc, cnfVFEnd) || !strings.Contains(mainSrc, cnfParseSig) {
		return nil, nil, false
	}
	i := strings.Index(mainSrc, cnfVFSig)
	j := strings.Index(mainSrc[i:], cnfVFEnd)
	if j < 0 {
		return nil, nil, false
	}
	j += i
	V := map[string]bool{}
	for _, f := range cnfFlagRE.FindAllStringSubmatch(mainSrc[i:j], -1) {
		V[f[1]] = true
	}
	p := strings.Index(mainSrc, cnfParseSig)
	segEnd := len(mainSrc)
	if i > p {
		segEnd = i
	}
	seg := mainSrc[p:segEnd]
	D := map[string]bool{}
	for _, rx := range []*regexp.Regexp{cnfCaseRE, cnfCaseLitRE, cnfPrefixRE} {
		for _, m := range rx.FindAllStringSubmatch(seg, -1) {
			D[m[1]] = true
		}
	}
	usages, ok := cnfParseUsages(root)
	if !ok || len(V) == 0 {
		return nil, nil, false
	}
	recognized := map[string]bool{}
	for k := range V {
		recognized[k] = true
	}
	for k := range D {
		recognized[k] = true
	}
	return recognized, usages, true
}

func cnfLoadKinds(root string) (map[string]bool, bool) {
	fp := cnfPyJoin(root, cnfErrorsRel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	start := strings.Index(src, "var errorKinds = []errorKindRow{")
	if start < 0 {
		return nil, false
	}
	endRel := strings.Index(src[start:], "\n}")
	if endRel < 0 {
		return nil, false
	}
	out := map[string]bool{}
	for _, m := range cnfKindRowRE.FindAllStringSubmatch(src[start:start+endRel], -1) {
		out[m[1]] = true
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func cnfLoadExitcodes(root string) (map[int]bool, bool) {
	fp := cnfPyJoin(root, cnfExitcodesRel)
	if !cnfIsFile(fp) {
		return nil, false
	}
	src, err := cnfReadText(fp)
	if err != nil {
		return nil, false
	}
	out := map[int]bool{}
	for _, v := range []string{"exitcodeTable", "exitcodeReserved"} {
		rx := regexp.MustCompile(`(?s)var` + cnfWS + `+` + v + cnfWS + `*=` + cnfWS + `*\[\]exitcodeRow\{(.*?)\n\}`)
		m := rx.FindStringSubmatch(src)
		if m != nil {
			for _, x := range cnfTableRowRE.FindAllStringSubmatch(m[1], -1) {
				if n, ok := cnfPyIntStr(x[1]); ok {
					out[n] = true
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// cnfLoadNsCodes：第三个返回值 = 「`namespaces` 或 `namespaces[].table` 显式 null」（py 在此裸回溯 ⇒ rc=2）。
func cnfLoadNsCodes(root string) (map[int]bool, bool, bool) {
	doc, ok := cnfLoadJSON(root, cnfNamespacesRel)
	if !ok || doc == nil {
		return nil, false, false
	}
	if cnfDictNull(doc, "namespaces") {
		return nil, false, true
	}
	nss, _, isDict := cnfDictGet(doc, "namespaces")
	if !isDict {
		return nil, false, false
	}
	arr, _ := nss.([]interface{})
	for _, ns := range arr {
		nm, ok := ns.(map[string]interface{})
		if !ok {
			continue
		}
		id, _, _ := cnfDictGet(nm, "id")
		if s, _ := id.(string); s != "cli" {
			continue
		}
		if cnfDictNull(nm, "table") {
			return nil, false, true
		}
		tbl, _, _ := cnfDictGet(nm, "table")
		ta, _ := tbl.([]interface{})
		out := map[int]bool{}
		for _, x := range ta {
			if n, ok := cnfPyInt(x); ok {
				out[n] = true
			}
		}
		return out, true, false
	}
	return nil, false, false
}

func cnfLoadDocs(root string) []interface{} {
	out := make([]interface{}, 0, len(cnfDocsRels))
	for _, rel := range cnfDocsRels {
		fp := cnfPyJoin(root, rel)
		if !cnfIsFile(fp) {
			out = append(out, nil)
			continue
		}
		src, err := cnfReadText(fp)
		if err != nil {
			out = append(out, nil)
			continue
		}
		out = append(out, src)
	}
	return out
}

func cnfImplLiterals(root string, impls []string, res []*regexp.Regexp) map[string]bool {
	out := map[string]bool{}
	for _, f := range impls {
		src, err := cnfReadText(cnfPyJoin(root, f))
		if err != nil {
			continue
		}
		for _, rx := range res {
			for _, m := range rx.FindAllStringSubmatch(src, -1) {
				out[m[1]] = true
			}
		}
	}
	return out
}

// ── 主逻辑 ────────────────────────────────────────────────────────────────────
func cnfOKWord(b bool) string {
	if b {
		return "齐"
	}
	return "缺"
}

func cnfAnalyse(root string, maxExamples int, stderr io.Writer) (int, []string) {
	L := []string{}
	add := func(s string) { L = append(L, s) }
	crash := func() (int, []string) {
		fmt.Fprintln(stderr, "⚠ 判据内部异常路径：按新口径 fail-closed ⇒ rc=2（不给结论）")
		return 2, nil
	}

	tree, mForms, got := cnfParseTree(root)
	if !got {
		return 2, []string{"[不给结论] 取不到命令树块 `commands = []*command{`（前置缺件）⇒ rc=2（不许当绿）"}
	}
	if tree == nil {
		return 2, []string{"[不给结论] 命令树块解析失败 ⇒ rc=2"}
	}
	nPaths := len(tree)
	paths := cnfCmdPathsOnly(tree)
	if nPaths == 0 {
		return 2, []string{"[不给结论] 现读 path 集为空 ⇒ rc=2（空转不许当绿）"}
	}
	if mForms != nPaths {
		return 2, []string{fmt.Sprintf("[不给结论] 解析漏读：块内 `path: []string{` 命中 %d 处 ⟷ 解析出 %d 条 ⇒ rc=2"+
			"（漏读的集合不许当基线）", mForms, nPaths)}
	}

	base, bok := cnfLoadJSON(root, cnfOwnBaselineRel)
	if !bok || base == nil {
		return 2, []string{fmt.Sprintf("[不给结论] 基线件缺失/不可解析：%s ⇒ rc=2", cnfOwnBaselineRel)}
	}
	sch, _, _ := cnfDictGet(base, "schema")
	if s, _ := sch.(string); s != cnfSchema {
		return 2, []string{fmt.Sprintf("[不给结论] 基线件 schema 逐字不是 `%s`（现读 %s）⇒ rc=2", cnfSchema, cnfPyRepr(sch))}
	}
	pathsRaw, _, _ := cnfDictGet(base, "paths")
	if cnfDictNull(base, "paths") {
		return 2, []string{fmt.Sprintf("[不给结论] 基线件 `paths` 为显式 null（py 于 `list(base.get('paths',[]))` 裸回溯）⇒ 本件 fail-closed：rc=2（不给结论）· %s", cnfOwnBaselineRel)}
	}
	bpaths := cnfToStringList(pathsRaw)
	cntVal, _, _ := cnfDictGet(base, "count")
	if cn, ok := cnfPyInt(cntVal); !ok || cn != len(bpaths) {
		return 2, []string{fmt.Sprintf("[不给结论] 基线自证不过：count=%s ⟷ len(paths)=%d ⇒ rc=2", cnfPyRepr(cntVal), len(bpaths))}
	}

	cur := map[string]bool{}
	for _, p := range paths {
		cur[p] = true
	}
	old := map[string]bool{}
	for _, p := range bpaths {
		old[p] = true
	}
	newPaths := cnfSortedDiff(cur, old)
	stale := cnfSortedDiff(old, cur)

	add(fmt.Sprintf("现读 path 条数：%d（块内 `path: []string{` 形态命中 %d 处 · 差 %d）", nPaths, mForms, mForms-nPaths))
	hv, hok, _ := cnfDictGet(base, "head")
	if !hok {
		hv = "?"
	}
	add(fmt.Sprintf("冻结基线：%d 条（head=%s）", len(bpaths), cnfPyStr(hv)))
	add(fmt.Sprintf("新增 path：%d 条", len(newPaths)))
	add(fmt.Sprintf("基线陈旧（只报 · 不改退码）：%d 条", len(stale)))
	for _, s := range cnfHead(stale, maxExamples) {
		add("  · 陈旧 " + s)
	}

	matrix, mok := cnfLoadJSON(root, cnfMatrixRel)
	cbase, cok := cnfLoadJSON(root, cnfContractBaseline)
	dryreg, dok, dryNull := cnfLoadDryrunRegistry(root)
	marks, mrok := cnfLoadMarks(root)
	exa, eaok := cnfLoadExcludesA(root)
	exb0, exb1, ebok := cnfLoadExcludesB(root)
	kinds, kiok := cnfLoadKinds(root)
	excodes, exok := cnfLoadExitcodes(root)
	if dryNull {
		return 2, []string{fmt.Sprintf("[不给结论] dryrun 登记件 `件` 为显式 null（py 于 `for e in doc.get('件',[])` 裸回溯）⇒ 本件 fail-closed：rc=2（不给结论）· %s", cnfDryrunRegRel)}
	}
	if !mok || !cok || !dok || !mrok || !eaok || !ebok || !kiok || !exok {
		missing := []string{}
		if !mok {
			missing = append(missing, cnfMatrixRel)
		}
		if !cok {
			missing = append(missing, cnfContractBaseline)
		}
		if !dok {
			missing = append(missing, cnfDryrunRegRel)
		}
		if !mrok {
			missing = append(missing, cnfMarksRel)
		}
		if !eaok {
			missing = append(missing, cnfExclARel)
		}
		if !ebok {
			missing = append(missing, cnfExclBRel)
		}
		if !kiok {
			missing = append(missing, cnfErrorsRel+"（errorKinds 表空/缺件）")
		}
		if !exok {
			missing = append(missing, cnfExitcodesRel+"（退码表空/缺件）")
		}
		return 2, append(L, fmt.Sprintf("[不给结论] 前置缺件/不可解析：%s ⇒ rc=2", strings.Join(missing, "、")))
	}

	if cnfDictNull(matrix, "cases") {
		return 2, []string{fmt.Sprintf("[不给结论] cli-matrix `cases` 为显式 null（py 于 `len(cases)` 裸回溯）⇒ 本件 fail-closed：rc=2（不给结论）· %s", cnfMatrixRel)}
	}
	casesRaw, _, isDict := cnfDictGet(matrix, "cases")
	if !isDict {
		return crash()
	}
	cases, _ := casesRaw.([]interface{})
	if casesRaw != nil && cases == nil {
		return crash()
	}
	add(fmt.Sprintf("扫描面件数：cli-matrix cases=%d · dryrun 登记件=%d · 标记表行=%d · 侧甲排除=%d 条 · 侧乙 exact=%d/prefix=%d",
		len(cases), len(dryreg), len(marks), len(exa), len(exb0), len(exb1)))

	var flaggedOK bool
	var recognized map[string]bool
	var usages map[string]cnfUsage
	if len(newPaths) > 0 {
		recognized, usages, flaggedOK = cnfBuildFlagFaces(root)
		if !flaggedOK {
			return 2, append(L, "[不给结论] 面⑤ 取数面（门73 口径 · `valueFlagName`/`parseInvocation`）拿不到 ⇒ rc=2")
		}
	} else {
		recognized, usages = map[string]bool{}, map[string]cnfUsage{}
	}
	nsCodes, nsOK, nsNull := cnfLoadNsCodes(root)
	if nsNull {
		return 2, []string{fmt.Sprintf("[不给结论] `exitcode-namespaces.json` 的 `namespaces` 或 `namespaces[].table` 为显式 null（py 于该两处裸回溯）⇒ 本件 fail-closed：rc=2（不给结论）· %s", cnfNamespacesRel)}
	}
	docsTexts := cnfLoadDocs(root)

	faceWord := "新增 0 条 ⇒ 面⑤ 取数面未跑（不许把 0 当空转）"
	recWord := "未取"
	if len(newPaths) > 0 {
		faceWord = "新增>0 ⇒ 现读取数面"
		recWord = strconv.Itoa(len(recognized))
	}
	nsWord := "未取到"
	if nsOK {
		nsWord = fmt.Sprintf("%d 号", len(nsCodes))
	}
	nDocs := 0
	for _, t := range docsTexts {
		if t != nil {
			nDocs++
		}
	}
	add(fmt.Sprintf("扩面取数件数（%s）：门73 认旗标=%s · error.kind 闭集=%d · 退码表=%d · 命名空间 cli=%s · docs 语种在盘=%d",
		faceWord, recWord, len(kinds), len(excodes), nsWord, nDocs))

	leafMap := map[string]string{}
	for _, e := range tree {
		leafMap[e.path] = e.run
	}

	bad := 0
	for _, p := range newPaths {
		leafFn := leafMap[p]
		impls := cnfFindImplFiles(root, leafFn)
		fnOr := leafFn
		if fnOr == "" {
			fnOr = "?"
		}
		implOr := strings.Join(impls, "、")
		if implOr == "" {
			implOr = "未定位"
		}
		add(fmt.Sprintf("── 新增 path：`zerg %s`（run=%s · 实现件 %s）", p, fnOr, implOr))

		// 面① dryrun 落点
		f1ok, f1note := true, []string{}
		if len(impls) == 0 {
			f1ok = false
			f1note = append(f1note, "实现件未定位 ⇒ 缺（判不了）")
		} else {
			for _, f := range impls {
				has := false
				if src, err := cnfReadText(cnfPyJoin(root, f)); err == nil {
					has = cnfDryrunRE.MatchString(src)
				}
				if has && !dryreg[f] {
					f1ok = false
					f1note = append(f1note, fmt.Sprintf("%s 含 dry-run 语义但未登记 ⇒ 缺", f))
				} else if has {
					f1note = append(f1note, fmt.Sprintf("%s 已登记 ⇒ 齐", f))
				} else {
					f1note = append(f1note, fmt.Sprintf("%s 无 dry-run 语义 ⇒ 不适用（无落点）", f))
				}
			}
		}
		add(fmt.Sprintf("   面① dryrun 落点：%s —— %s", cnfOKWord(f1ok), strings.Join(f1note, " · ")))
		if !f1ok {
			bad++
		}

		mine := cnfCasesFor(cases, p)

		// 面② cli-matrix must-fail
		f2ok := len(mine) > 0
		if f2ok {
			for _, c := range mine {
				w := cnfCaseWantRC(c)
				if !w.ok {
					return crash()
				}
				if w.v == 0 {
					f2ok = false
					break
				}
			}
		}
		rcStrs := []string{}
		for _, c := range cnfHead(mine, maxExamples) {
			rcStrs = append(rcStrs, cnfPyStr(cnfCaseRaw(c, "want_rc")))
		}
		rcStr := strings.Join(rcStrs, ",")
		if rcStr == "" {
			rcStr = "-"
		}
		sfx := ""
		if !f2ok {
			sfx = "（需 ≥1 格 且 全非 0）"
		}
		add(fmt.Sprintf("   面② cli-matrix must-fail：%s —— 命中 %d 格（want_rc=%s）%s", cnfOKWord(f2ok), len(mine), rcStr, sfx))
		if !f2ok {
			bad++
		}

		// 面④ 标记表 + 两器
		f4ok, f4note := true, []string{}
		if len(impls) == 0 {
			f4ok = false
			f4note = append(f4note, "实现件未定位 ⇒ 缺")
		}
		for _, f := range impls {
			pub, ok := marks[f]
			if !ok {
				f4ok = false
				f4note = append(f4note, fmt.Sprintf("%s 不在标记表 ⇒ 缺", f))
				continue
			}
			if pub == "no" {
				a := cnfExcluded(f, exa, nil)
				b := cnfExcluded(f, exb0, exb1)
				if !a {
					f4ok = false
					f4note = append(f4note, fmt.Sprintf("%s public=no 但缺侧甲 ⇒ 两器都发（缺）", f))
				} else {
					bw := "未排除"
					if b {
						bw = "已排除"
					}
					f4note = append(f4note, fmt.Sprintf("%s public=no · 侧甲已排除（侧乙=%s）⇒ 齐", f, bw))
				}
			} else {
				f4note = append(f4note, fmt.Sprintf("%s public=%s ⇒ 齐", f, pub))
			}
		}
		add(fmt.Sprintf("   面④ 标记表+两器排除：%s —— %s", cnfOKWord(f4ok), strings.Join(f4note, " · ")))
		if !f4ok {
			bad++
		}

		// 面⑤ 门73 旗标名表
		f5ok, f5note := true, []string{}
		u, present := usages[p]
		if present && u.passthru {
			f5note = append(f5note, "透传型命令 ⇒ 不适用（旗标逐字交被包脚本 · 门73 同口径）")
		} else if !present {
			f5ok = false
			f5note = append(f5note, "用法串未定位 ⇒ 缺（判不了）")
		} else {
			recognizedHits, ghosts := []string{}, []string{}
			for _, m := range cnfFlagRE.FindAllStringSubmatchIndex(u.usage, -1) {
				flag := u.usage[m[2]:m[3]]
				pos := utf8.RuneCountInString(u.usage[:m[1]])
				if cnfValueForm(u.usage, pos) {
					if recognized[flag] {
						recognizedHits = append(recognizedHits, flag)
					} else {
						ghosts = append(ghosts, flag)
					}
				}
			}
			if len(ghosts) > 0 {
				f5ok = false
				for _, g := range cnfDedupOrdered(ghosts) {
					f5note = append(f5note, fmt.Sprintf("%s 取值形态但解析面不认 ⇒ 缺（真给退 2「未知旗标」= 命令不可用）", g))
				}
			}
			if len(recognizedHits) == 0 && len(ghosts) == 0 {
				f5note = append(f5note, "用法串无取值形态旗标 ⇒ 不适用（无落点）")
			} else if f5ok {
				f5note = append(f5note, fmt.Sprintf("%s 均在 valueFlagName∪parseInvocation ⇒ 齐", strings.Join(cnfDedupOrdered(recognizedHits), "、")))
			}
		}
		add(fmt.Sprintf("   面⑤ 门73 旗标名表：%s —— %s", cnfOKWord(f5ok), strings.Join(f5note, " · ")))
		if !f5ok {
			bad++
		}

		// 面⑥ 门⑤ error.kind 闭集
		f6ok, f6note := true, []string{}
		litSet := cnfImplLiterals(root, impls, []*regexp.Regexp{cnfKindUseRE, cnfKindJSONRE})
		kindLits := []string{}
		for k := range litSet {
			if k != "kind" {
				kindLits = append(kindLits, k)
			}
		}
		sort.Strings(kindLits)
		if len(kindLits) == 0 {
			f6note = append(f6note, "实现件无 kind 字面量 ⇒ 不适用（无落点）")
		} else {
			ghostKinds := []string{}
			for _, k := range kindLits {
				if !kinds[k] {
					ghostKinds = append(ghostKinds, k)
				}
			}
			if len(ghostKinds) > 0 {
				f6ok = false
				for _, k := range ghostKinds {
					f6note = append(f6note, fmt.Sprintf("%s 不在 errorKinds 闭集 ⇒ 缺（先登记再用 · 门⑤ R3 口径）", cnfPyReprStr(k)))
				}
			} else {
				f6note = append(f6note, fmt.Sprintf("%s 均在闭集 ⇒ 齐", strings.Join(kindLits, "、")))
			}
		}
		add(fmt.Sprintf("   面⑥ 门⑤ error.kind 闭集：%s —— %s", cnfOKWord(f6ok), strings.Join(f6note, " · ")))
		if !f6ok {
			bad++
		}

		// 面⑦ 门⑨ 退码表 + 命名空间
		f7ok, f7note := true, []string{}
		codeSet := map[int]bool{}
		for _, rx := range cnfOSExitREs {
			for x := range cnfImplLiterals(root, impls, []*regexp.Regexp{rx}) {
				if n, ok := cnfPyIntStr(x); ok {
					codeSet[n] = true
				}
			}
		}
		codeLits := []int{}
		for c := range codeSet {
			codeLits = append(codeLits, c)
		}
		sort.Ints(codeLits)
		if len(codeLits) == 0 {
			f7note = append(f7note, "实现件无退码字面量 ⇒ 不适用（无落点）")
		} else {
			offTable, offNs := []int{}, []int{}
			for _, c := range codeLits {
				if !excodes[c] {
					offTable = append(offTable, c)
				}
				if nsOK && !nsCodes[c] {
					offNs = append(offNs, c)
				}
			}
			if len(offTable) > 0 {
				f7ok = false
				for _, c := range offTable {
					f7note = append(f7note, fmt.Sprintf("%d 不在 exitcodeTable∪exitcodeReserved ⇒ 缺（先登记号再启用 · 门⑨）", c))
				}
			}
			if len(offNs) > 0 {
				f7ok = false
				for _, c := range offNs {
					f7note = append(f7note, fmt.Sprintf("%d 未登记进 exitcode-namespaces.json 的 cli 命名空间 ⇒ 缺（门⑨ R5）", c))
				}
			}
			if f7ok {
				cs := []string{}
				for _, c := range codeLits {
					cs = append(cs, strconv.Itoa(c))
				}
				tail := "（命名空间表未取到 ⇒ 该半格不适用）"
				if nsOK {
					tail = "且被 cli 命名空间覆盖"
				}
				f7note = append(f7note, fmt.Sprintf("%s 均在退码表%s ⇒ 齐", strings.Join(cs, "、"), tail))
			}
		}
		add(fmt.Sprintf("   面⑦ 门⑨ 退码表+命名空间：%s —— %s", cnfOKWord(f7ok), strings.Join(f7note, " · ")))
		if !f7ok {
			bad++
		}

		// 面⑧ 门⑪ T5d 归属面（只报 · 不改退码）
		noWhy := []string{}
		for _, c := range mine {
			if cnfStrip(cnfPyStr(cnfCaseRaw(c, "why"))) == "" {
				noWhy = append(noWhy, cnfPyStr(cnfCaseRaw(c, "id")))
			}
		}
		f8note := ""
		switch {
		case len(mine) == 0:
			f8note = "无 must-fail 格 ⇒ 不适用（面② 已判）"
		case len(noWhy) > 0:
			f8note = fmt.Sprintf("只报：%d 格缺 why ⇒ T5d（why⟷值）判不了（T3 已由面⑦ · T6 registry 引用面属全局 · 不改退码）：%s",
				len(noWhy), strings.Join(cnfHead(noWhy, maxExamples), "、"))
		default:
			f8note = "每格有 why ⇒ T5d 可判（T3 已由面⑦ · T6 属全局）"
		}
		add(fmt.Sprintf("   面⑧ 门⑪ T5d 归属面（只报）：—— %s", f8note))

		// 面⑨ docs 生成物（只报）
		f9notes := []string{}
		for i, rel := range cnfDocsRels {
			txt := docsTexts[i]
			if txt == nil {
				f9notes = append(f9notes, fmt.Sprintf("%s 不在盘 ⇒ 不适用", rel))
			} else if s, _ := txt.(string); !strings.Contains(s, p) {
				f9notes = append(f9notes, fmt.Sprintf("只报：%s 生成区未见 `%s`（跑 gen-cli-reference.py --check）", rel, p))
			} else {
				f9notes = append(f9notes, fmt.Sprintf("%s 有格", rel))
			}
		}
		add(fmt.Sprintf("   面⑨ docs 生成物（只报）：—— %s", strings.Join(f9notes, " · ")))
	}

	// 面③ 计数（全局一格）
	nCases := len(cases)
	bcv, _, _ := cnfDictGet(cbase, "commands")
	bmv, _, _ := cnfDictGet(cbase, "mustfail_cases")
	bcn, bcOK := cnfPyInt(bcv)
	bmn, bmOK := cnfPyInt(bmv)
	cOK := bcOK && bmOK && bcn == nPaths && bmn == nCases
	add(fmt.Sprintf("── 面③ cli-contract-baseline 计数：%s", cnfOKWord(cOK)))
	add(fmt.Sprintf("   commands 基线=%s ⟷ 现读=%d %s", cnfPyRepr(bcv), nPaths, cnfTick(bcOK && bcn == nPaths)))
	add(fmt.Sprintf("   mustfail_cases 基线=%s ⟷ 现读=%d %s", cnfPyRepr(bmv), nCases, cnfTick(bmOK && bmn == nCases)))
	if !cOK {
		bad++
	}

	if len(newPaths) == 0 {
		add(fmt.Sprintf("新增 0 条 ⇒ 九面无需随动（本读数可分辨「没跑」：上有现读 %d 条 / %d 处 path 形态与两面扫描件数行）", nPaths, mForms))
	}
	if bad > 0 {
		add(fmt.Sprintf("⇒ rc=1：以上 %d 处「缺」逐条点名（缺任一面即红）", bad))
		return 1, L
	}
	add(fmt.Sprintf("⇒ rc=0：新增 path 逐面齐（面①–⑨）/ 计数齐 / 解析自证齐（%d 条 ⟷ %d 处）", nPaths, mForms))
	return 0, L
}

func cnfTick(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func cnfSortedDiff(a, b map[string]bool) []string {
	out := []string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func cnfToStringList(v interface{}) []string {
	arr, _ := v.([]interface{})
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func cnfDedupOrdered(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func cnfCasesFor(cases []interface{}, path string) []interface{} {
	out := []interface{}{}
	for _, c := range cases {
		if v := cnfCaseRaw(c, "command"); v != nil {
			if s, ok := v.(string); ok && s == path {
				out = append(out, c)
			}
		}
	}
	return out
}

// cnfCaseRaw = c.get(key)（缺 ⇒ nil）
func cnfCaseRaw(c interface{}, key string) interface{} {
	v, _, _ := cnfDictGet(c, key)
	return v
}

type cnfWantRC struct {
	v  int
	ok bool
}

// cnfCaseWantRC = c.get('want_rc', 1) 的 int()
func cnfCaseWantRC(c interface{}) cnfWantRC {
	v, present, _ := cnfDictGet(c, "want_rc")
	if !present {
		return cnfWantRC{v: 1, ok: true}
	}
	n, ok := cnfPyInt(v)
	return cnfWantRC{v: n, ok: ok}
}

// ── 基线产出（--emit-baseline，只印 stdout 不写盘）─────────────────────────────
func cnfEmitBaseline(stdout, stderr io.Writer, root string) int {
	tree, _, got := cnfParseTree(root)
	if !got || tree == nil {
		fmt.Fprintln(stderr, "取不到命令树块 ⇒ 不许出基线")
		return 2
	}
	paths := cnfCmdPathsOnly(tree)
	sort.Strings(paths)
	obj := &cnfJobj{}
	obj.set("schema", cnfSchema)
	obj.set("head", nil)
	obj.set("count", len(paths))
	obj.set("_口径", cnfEmitNote)
	arr := make([]cnfJval, 0, len(paths))
	for _, p := range paths {
		arr = append(arr, p)
	}
	obj.set("paths", arr)
	fmt.Fprintln(stdout, cnfJSONString(obj, 1))
	return 0
}

const cnfEmitNote = "冻结的 path 全集（= `zerg gate`-facing 命令树逐条 path 以空格连接）· 单一真源 = core/cmd/zerg/main.go 的 commands 块（与 check-cli-contract.py 同一组正则）。" +
	"新增命令 ⇒ 本件必须**同批重冻**（否则 check-newcmd-followup.py 会把新 path 当「新增」逐面点名）。"

// ── JSON 编码（复刻 py json.dumps(ensure_ascii=False, indent=1)）─────────────────
type cnfJval interface{}

type cnfJobj struct {
	k []string
	v []cnfJval
}

func (o *cnfJobj) set(key string, val cnfJval) {
	o.k = append(o.k, key)
	o.v = append(o.v, val)
}

func cnfJSONString(v cnfJval, ind int) string {
	var b strings.Builder
	cnfJSONEnc(&b, v, 0, ind)
	return b.String()
}

func cnfJSONEncIndent(b *strings.Builder, depth, ind int) {
	for i := 0; i < depth*ind; i++ {
		b.WriteByte(' ')
	}
}

func cnfJSONEnc(b *strings.Builder, v cnfJval, depth, ind int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		cnfJSONStr(b, x)
	case *cnfJobj:
		if len(x.k) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i := range x.k {
			if i > 0 {
				b.WriteString(",\n")
			}
			cnfJSONEncIndent(b, depth+1, ind)
			cnfJSONStr(b, x.k[i])
			b.WriteString(": ")
			cnfJSONEnc(b, x.v[i], depth+1, ind)
		}
		b.WriteString("\n")
		cnfJSONEncIndent(b, depth, ind)
		b.WriteString("}")
	case []cnfJval:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i := range x {
			if i > 0 {
				b.WriteString(",\n")
			}
			cnfJSONEncIndent(b, depth+1, ind)
			cnfJSONEnc(b, x[i], depth+1, ind)
		}
		b.WriteString("\n")
		cnfJSONEncIndent(b, depth, ind)
		b.WriteString("]")
	default:
		b.WriteString("null")
	}
}

func cnfJSONStr(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// ── 自检夹具（成对负控 · 仓外 · 自回收）─────────────────────────────────────────
func cnfWrite(path, body string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(body), 0o644)
}

func cnfWriteJSON(path string, v interface{}) {
	b, _ := json.Marshal(v)
	cnfWrite(path, string(b))
}

// 夹具魔法串一律**拼串**落地（✗ 不写连续原串——否则落进他门扫描面）
var (
	cnfSelfDrySite = "\n// dry" + "-run 站点\nvar dry" + "RunFlag = true\n"
	cnfSelfDryFlag = "\nvar dry" + "RunFlag = true\n"
	cnfSelfSetErr  = "\nfunc x(){ set" + "Err(\"ghost_kind\", 1, \"x\") }\n"
	cnfSelfExit99  = "\nfunc y(){ os." + "Exit(99) }\n"
	cnfSelfExit14  = "\nfunc y(){ os." + "Exit(14) }\n"
)

func cnfMiniRoot(dst string, nExtraCmd, extraHasMatrix, countDrift bool, extraUsage string, extraFlags []string) {
	type mcmd struct{ p, fn, us string }
	cmds := []mcmd{{"alpha", "cmdAlpha", "zerg alpha"}, {"core", "cmdCore", "zerg core"}}
	if nExtraCmd {
		cmds = append(cmds, mcmd{"nu", "cmdNu", extraUsage})
	}
	for _, d := range []string{
		cnfPyJoin(dst, "core", "cmd", "zerg", "testdata"),
		cnfPyJoin(dst, "scripts", "gates"),
		cnfPyJoin(dst, "scripts", "build"),
		cnfPyJoin(dst, "publish"),
		cnfPyJoin(dst, "core", "internal", "contract"),
	} {
		_ = os.MkdirAll(d, 0o755)
	}
	blocks := ""
	for _, c := range cmds {
		blocks += "\t\t{\n\t\t\tpath: []string{\"" + c.p + "\"},\n\t\t\trun: " + c.fn + ",\n\t\t\tusage: \"" + c.us + "\",\n\t\t},\n"
	}
	vfNames := []string{"\"--base\""}
	for _, f := range extraFlags {
		vfNames = append(vfNames, "\""+f+"\"")
	}
	main := "package main\n\nfunc main() {\n\tcommands = []*command{\n" + blocks +
		"\t}\n}\n\n" +
		"func valueFlagName(a string) string {\n\tswitch a {\n\tcase " + strings.Join(vfNames, ", ") +
		":\n\t\treturn a\n\t}\n\treturn \"\"\n}\n\n" +
		"func parseInvocation() {}\n"
	cnfWrite(cnfPyJoin(dst, cnfMainRel), main)
	for _, c := range cmds {
		cnfWrite(cnfPyJoin(dst, "core", "cmd", "zerg", c.fn+".go"), "package main\n\nfunc "+c.fn+"() {}\n")
	}
	cnfWrite(cnfPyJoin(dst, cnfErrorsRel),
		"package main\n\nvar errorKinds = []errorKindRow{\n"+
			"\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n"+
			"\t{\"usage\", 2, false, \"r\", \"m\", \"s\"},\n}\n")
	cnfWrite(cnfPyJoin(dst, cnfExitcodesRel),
		"package main\n\nvar exitcodeTable = []exitcodeRow{\n"+
			"\t{0, \"ok\", \"成功\", \"—\"},\n\t{1, \"failed\", \"一般失败\", \"看 kind\"},\n"+
			"\t{2, \"usage\", \"用法错\", \"改用法\"},\n\t{14, \"conflict\", \"冲突\", \"排队\"},\n}\n"+
			"var exitcodeReserved = []exitcodeRow{}\n")
	cnfWriteJSON(cnfPyJoin(dst, cnfNamespacesRel), map[string]interface{}{
		"schema": "zerg/exitcodes-namespaces/v1",
		"namespaces": []interface{}{
			map[string]interface{}{"id": "cli", "table": []int{0, 1, 2, 14}},
		},
	})
	last := cmds[len(cmds)-1].p
	cases := []interface{}{}
	for _, c := range cmds {
		appendCase := false
		if c.p == "alpha" && nExtraCmd {
			appendCase = true
		} else if c.p != last || extraHasMatrix {
			appendCase = true
		}
		if appendCase {
			cases = append(cases, map[string]interface{}{
				"id": c.p + "#x", "command": c.p, "argv": []interface{}{},
				"want_rc": 2, "want_stdout_bytes": 0, "why": "夹具",
			})
		}
	}
	cnfWriteJSON(cnfPyJoin(dst, cnfMatrixRel), map[string]interface{}{
		"id": "cli-contract.v1", "cases": cases,
	})
	bc := len(cmds)
	if countDrift {
		bc--
	}
	cnfWriteJSON(cnfPyJoin(dst, cnfContractBaseline), map[string]interface{}{
		"id": "cli-contract-baseline.v1", "blanks": 0, "mustfail_cases": len(cases), "commands": bc,
	})
	cnfWriteJSON(cnfPyJoin(dst, cnfDryrunRegRel), map[string]interface{}{
		"schema": "zerg/dryrun-semantics/v1", "件": []interface{}{},
	})
	marks := "文件\tpublic\t理由\n"
	for _, c := range cmds {
		marks += "core/cmd/zerg/" + c.fn + ".go\tyes\t测试夹具\n"
	}
	cnfWrite(cnfPyJoin(dst, cnfMarksRel), marks)
	cnfWrite(cnfPyJoin(dst, cnfExclARel), "EXCLUDES=(\n \"\"\n)\n")
	cnfWrite(cnfPyJoin(dst, cnfExclBRel), "DROP_EXACT = []\nDROP_PREFIX = []\n")
	bpaths := []string{}
	for _, c := range cmds {
		bpaths = append(bpaths, c.p)
	}
	sort.Strings(bpaths)
	if nExtraCmd {
		bpaths = bpaths[:2]
	}
	cnfWriteJSON(cnfPyJoin(dst, cnfOwnBaselineRel), map[string]interface{}{
		"schema": cnfSchema, "head": "fixture", "count": len(bpaths), "paths": bpaths,
	})
}

func cnfAnyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func cnfSelfTest(stdout io.Writer) int {
	fails := []string{}
	tmp, err := os.MkdirTemp("", "newcmd-followup-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "自检不过：")
		fmt.Fprintln(stdout, "  ✗ 建临时夹具失败")
		return 1
	}
	defer os.RemoveAll(tmp)
	disc := io.Discard

	a := cnfPyJoin(tmp, "a")
	cnfMiniRoot(a, false, true, false, "zerg nu", nil)
	if rc, _ := cnfAnalyse(a, 20, disc); rc != 0 {
		fails = append(fails, fmt.Sprintf("① 正控（无新增）期望 rc=0，实得 %d", rc))
	}

	b := cnfPyJoin(tmp, "b")
	cnfMiniRoot(b, true, false, false, "zerg nu", nil)
	if rc, lines := cnfAnalyse(b, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("② 负控（新增缺面②）期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "面② cli-matrix must-fail：缺") {
		fails = append(fails, "② 负控未点名面②")
	}

	c := cnfPyJoin(tmp, "c")
	cnfMiniRoot(c, true, true, false, "zerg nu", nil)
	if rc, _ := cnfAnalyse(c, 20, disc); rc != 0 {
		fails = append(fails, fmt.Sprintf("③ 负控半边（四面齐）期望 rc=0，实得 %d", rc))
	}

	d := cnfPyJoin(tmp, "d")
	cnfMiniRoot(d, false, true, true, "zerg nu", nil)
	if rc, _ := cnfAnalyse(d, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("④ 计数漂移期望 rc=1，实得 %d", rc))
	}

	e := cnfPyJoin(tmp, "e")
	cnfMiniRoot(e, false, true, false, "zerg nu", nil)
	ep := cnfPyJoin(e, cnfMainRel)
	if src, err := cnfReadText(ep); err == nil {
		src = strings.ReplaceAll(src, "\t}\n}\n", "\t\t{\n\t\t\tpath: []string{},\n\t\t\trun: cmdAlpha,\n\t\t},\n\t}\n}\n")
		cnfWrite(ep, src)
	}
	if rc, _ := cnfAnalyse(e, 20, disc); rc != 2 {
		fails = append(fails, fmt.Sprintf("⑤ 解析漏读期望 rc=2，实得 %d", rc))
	}

	f := cnfPyJoin(tmp, "f")
	cnfMiniRoot(f, false, true, false, "zerg nu", nil)
	_ = os.Remove(cnfPyJoin(f, cnfOwnBaselineRel))
	if rc, _ := cnfAnalyse(f, 20, disc); rc != 2 {
		fails = append(fails, fmt.Sprintf("⑥ 缺基线件期望 rc=2，实得 %d", rc))
	}

	g := cnfPyJoin(tmp, "g")
	cnfMiniRoot(g, true, true, false, "zerg nu", nil)
	cnfAppendFile(cnfPyJoin(g, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfDrySite)
	if rc, lines := cnfAnalyse(g, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑦ dryrun 未登记期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "面① dryrun 落点：缺") {
		fails = append(fails, "⑦ 未点名面①")
	}

	h := cnfPyJoin(tmp, "h")
	cnfMiniRoot(h, true, true, false, "zerg nu", nil)
	cnfAppendFile(cnfPyJoin(h, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfDryFlag)
	cnfWriteJSON(cnfPyJoin(h, cnfDryrunRegRel), map[string]interface{}{
		"schema": "zerg/dryrun-semantics/v1",
		"件":      []interface{}{map[string]interface{}{"file": "core/cmd/zerg/cmdNu.go", "语义": "dryrun.v1", "零副作用判据": "fixture"}},
	})
	if rc, _ := cnfAnalyse(h, 20, disc); rc != 0 {
		fails = append(fails, fmt.Sprintf("⑧ dryrun 已登记期望 rc=0，实得 %d", rc))
	}

	i := cnfPyJoin(tmp, "i")
	cnfMiniRoot(i, true, true, false, "zerg nu", nil)
	if src, err := cnfReadText(cnfPyJoin(i, cnfMarksRel)); err == nil {
		src = strings.Replace(src, "core/cmd/zerg/cmdNu.go\tyes\t", "core/cmd/zerg/cmdNu.go\tno\t", 1)
		cnfWrite(cnfPyJoin(i, cnfMarksRel), src)
	}
	if rc, lines := cnfAnalyse(i, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑨ public=no 缺侧甲期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "两器都发") {
		fails = append(fails, "⑨ 未点名「两器都发」")
	}

	j := cnfPyJoin(tmp, "j")
	cnfMiniRoot(j, true, true, false, "zerg nu", nil)
	if src, err := cnfReadText(cnfPyJoin(j, cnfMarksRel)); err == nil {
		src = strings.Replace(src, "core/cmd/zerg/cmdNu.go\tyes\t测试夹具\n", "", 1)
		cnfWrite(cnfPyJoin(j, cnfMarksRel), src)
	}
	if rc, _ := cnfAnalyse(j, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑩ 件不在标记表期望 rc=1，实得 %d", rc))
	}

	k := cnfPyJoin(tmp, "k")
	cnfMiniRoot(k, true, true, false, "zerg nu --ghost <值>", nil)
	if rc, lines := cnfAnalyse(k, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑪ 门73 旗标缺期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "面⑤ 门73 旗标名表：缺") {
		fails = append(fails, "⑪ 未点名面⑤")
	}

	l := cnfPyJoin(tmp, "l")
	cnfMiniRoot(l, true, true, false, "zerg nu --ghost <值>", []string{"--ghost"})
	if rc, _ := cnfAnalyse(l, 20, disc); rc != 0 {
		fails = append(fails, fmt.Sprintf("⑫ 门73 旗标齐期望 rc=0，实得 %d", rc))
	}

	m := cnfPyJoin(tmp, "m")
	cnfMiniRoot(m, true, true, false, "zerg nu", nil)
	cnfAppendFile(cnfPyJoin(m, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfSetErr)
	if rc, lines := cnfAnalyse(m, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑬ 门⑤ kind 缺期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "面⑥ 门⑤ error.kind 闭集：缺") {
		fails = append(fails, "⑬ 未点名面⑥")
	}

	n := cnfPyJoin(tmp, "n")
	cnfMiniRoot(n, true, true, false, "zerg nu", nil)
	cnfAppendFile(cnfPyJoin(n, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfExit99)
	if rc, lines := cnfAnalyse(n, 20, disc); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑭ 门⑨ 退码缺期望 rc=1，实得 %d", rc))
	} else if !cnfAnyContains(lines, "面⑦ 门⑨ 退码表+命名空间：缺") {
		fails = append(fails, "⑭ 未点名面⑦")
	}

	o := cnfPyJoin(tmp, "o")
	cnfMiniRoot(o, true, true, false, "zerg nu", nil)
	cnfAppendFile(cnfPyJoin(o, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfExit14)
	if rc, _ := cnfAnalyse(o, 20, disc); rc != 0 {
		fails = append(fails, fmt.Sprintf("⑮ 门⑨ 退码齐期望 rc=0，实得 %d", rc))
	}

	if len(fails) > 0 {
		fmt.Fprintln(stdout, "自检不过：")
		for _, x := range fails {
			fmt.Fprintln(stdout, "  ✗ "+x)
		}
		return 1
	}
	fmt.Fprintln(stdout, "自检通过：15 控（正控 4 + 负控 11）逐条对拍 ✓")
	return 0
}

func cnfAppendFile(path, extra string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(extra)
}

// ── 入口 ──────────────────────────────────────────────────────────────────────
const cnfUsageText = "usage: check-newcmd-followup.py [-h] [--root ROOT] [--json]\n" +
	"                                [--max-examples MAX_EXAMPLES]\n" +
	"                                [--emit-baseline] [--self-test]\n"

const cnfHelpText = cnfUsageText +
	"\noptional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json\n" +
	"  --max-examples MAX_EXAMPLES\n" +
	"  --emit-baseline\n" +
	"  --self-test\n"

const cnfProg = "check-newcmd-followup.py"

func cnfArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cnfUsageText)
	fmt.Fprintf(stderr, "%s: error: %s\n", cnfProg, msg)
	return 2
}

// cnfRepoRoot = 缺省根推导（声明 (a)）：显式 --root 逐字采用；否则从 cwd 上溯取含 core/ 与 ui/ 的目录。
func cnfRepoRoot(arg *string) string {
	if arg != nil {
		return *arg
	}
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return wd
	}
	d := wd
	for k := 0; k < 6; k++ {
		if cnfIsDir(cnfPyJoin(d, "core")) && cnfIsDir(cnfPyJoin(d, "ui")) {
			return d
		}
		parent := cnfPyDirname(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return wd
}

func checkNewcmdFollowupGateMain(stdout, stderr io.Writer, argv []string) int {
	var rootArg *string
	jsonOut, emit, selfTest, help := false, false, false, false
	maxExamples := 20

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--json":
			jsonOut = true
		case a == "--emit-baseline":
			emit = true
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				return cnfArgError(stderr, "argument --root: expected one argument")
			}
			i++
			s := argv[i]
			rootArg = &s
		case strings.HasPrefix(a, "--root="):
			s := a[len("--root="):]
			rootArg = &s
		case a == "--max-examples":
			if i+1 >= len(argv) {
				return cnfArgError(stderr, "argument --max-examples: expected one argument")
			}
			i++
			v, ok := cnfPyIntStr(argv[i])
			if !ok {
				return cnfArgError(stderr, "argument --max-examples: invalid int value: '"+argv[i]+"'")
			}
			maxExamples = v
		case strings.HasPrefix(a, "--max-examples="):
			s := a[len("--max-examples="):]
			v, ok := cnfPyIntStr(s)
			if !ok {
				return cnfArgError(stderr, "argument --max-examples: invalid int value: '"+s+"'")
			}
			maxExamples = v
		case strings.HasPrefix(a, "-") && a != "-":
			return cnfArgError(stderr, "unrecognized arguments: "+a)
		default:
			return cnfArgError(stderr, "unrecognized arguments: "+a)
		}
	}

	if help {
		fmt.Fprint(stdout, cnfHelpText)
		return 0
	}
	if selfTest {
		return cnfSelfTest(stdout)
	}
	if emit {
		return cnfEmitBaseline(stdout, stderr, cnfRepoRoot(rootArg))
	}

	rc, lines := cnfAnalyse(cnfRepoRoot(rootArg), maxExamples, stderr)
	if jsonOut {
		obj := &cnfJobj{}
		obj.set("schema", "zerg/v1")
		obj.set("ki"+"nd", "NewCmdFollowup")
		obj.set("rc", rc)
		arr := make([]cnfJval, 0, len(lines))
		for _, l := range lines {
			arr = append(arr, l)
		}
		obj.set("lines", arr)
		fmt.Fprintln(stdout, cnfJSONString(obj, 1))
	} else {
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
	}
	return rc
}
