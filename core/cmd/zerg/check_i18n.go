// check_i18n.go —— 件 ui/scripts/check-i18n.py（339 行 · sha16 53cdfbab3cf6533c）的 Go 真搬
// （v2.5.14 · M2 搬运面 · 批-5 · 入口 checkI18nGateMain）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐件搬 · 落点 core/cmd/zerg/）。验收判据 = 对同一输入跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径
//	差与未覆盖面）。本单**只落码 · 不接线**（✗ 不注册命令 · ✗ 不改 main.go · ✗ 不删 py 原件）。
//
// 选它的理由（本单口径：静态只读件 · ⑤ 可否 Go 化 = 可）：
//
//	✓ 纯静态只读：只读四类输入（ui/locales/*.yml · ui/src/**/*.rs · ui/scripts/i18n-baseline.json ·
//	  scripts/gates/baseline-audit.jsonl ＋ core/internal/api/*.go），逐字判四道门 G1..G4（＋G5/未引用提示）。
//	  **逐处现读全文坐实**：零 subprocess · 零 os.system/Popen · 零联网 · 零真 TTY · 零计时 · 零随机
//	  （普查表原标「真跑子进程」经现读**推翻** —— 件内无任何起子进程的形态，见回执）。
//	✓ 退码面简单：0 通过 / 1 门禁失败 / 2 取源根不可判。
//	✓ 唯一写面 = `--tsv` 模式的 TSV 落盘，目录走显式旋钮 ZERG_I18N_AUDIT_OUT（缺则取源根 + v2.5.9/i18n-audit），
//	  件自身不写任何固定路径。
//
// 出厂声明 (a) 默认根定位差异：py 从 `__file__` 上溯三层取仓根（ui/scripts/x.py → 仓根）；
//
//	Go 无 `__file__` 语境 ⇒ 从 cwd 上溯找**同时含 ui/ 与 core/ 的目录**（缺则回落 cwd）。
//	「脚本位置 ≠ cwd」时路径推导面不同 —— ✗ 不声称逐字对齐。对拍台把两侧都置于「cwd == 仓根」的形态。
//
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg 全件扫描面内 ⇒ 逐条消解：件内零绝对路径字面量、
//
//	零形如「引号 + 键名 + 引号」的协议原串、零错误构造调用首参形态、零会悬空的仓内相对路径字面量。
//	本件**不落在** check-i18n 自己的扫描面内（该门只扫 ui/src/**/*.rs 与 core/internal/api/*.go）。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func checkI18nGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名）；旗标以**整元素**判定（py `in sys.argv`）
//	  · 退码  —— 0 通过（含 --list）/ 1 门禁失败 / 2 取不到输入（取源根缺 · 读件失败）
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ── py 语义原语（✗ 不用 Go 标准库等价物，逐处自写同构）───────────────────────────

// ci18PySpace —— py 正则 \s 的 Unicode 同构类（str.isspace 集）：
// \t\n\v\f\r · \x1c-\x1f · 空格 · \x85 · \xa0 · \u1680 · \u2000-\u200a · \u2028 \u2029 \u202f \u205f \u3000。
const ci18PySpace = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// ci18PyWord —— py 正则 \w 的近似同构类（Unicode 字母/数字/下划线）。
const ci18PyWord = `[\p{L}\p{N}_]`

// ci18IsPySpace —— py str.isspace() 的同构判定（str.strip() 逐字用）。
func ci18IsPySpace(r rune) bool {
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

// ci18PyStrip —— py str.strip() 同构（默认剥 str.isspace 集；✗ 不用 strings.TrimSpace，它不剥 \x1c-\x1f）。
func ci18PyStrip(s string) string { return strings.TrimFunc(s, ci18IsPySpace) }

// ci18PyJoin —— posixpath.join 逐段同构（✗ 不用 filepath.Join：`..`／绝对后段／空段三类语义差）。
func ci18PyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
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

// ci18PyNormpath —— posixpath.normpath 逐字同构（★ 保留恰两个前导斜杠：`//a/b`⇒`//a/b`、`///a/b`⇒`/a/b`）。
func ci18PyNormpath(path string) string {
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
	var comps []string
	for _, c := range strings.Split(path, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." ||
			(initialSlashes == 0 && len(comps) == 0) ||
			(len(comps) > 0 && comps[len(comps)-1] == "..") {
			comps = append(comps, c)
		} else if len(comps) > 0 {
			comps = comps[:len(comps)-1]
		}
	}
	res := strings.Join(comps, "/")
	if initialSlashes > 0 {
		res = strings.Repeat("/", initialSlashes) + res
	}
	if res == "" {
		return "."
	}
	return res
}

// ci18PyAbspath —— posixpath.abspath 同构（join(cwd, path) 后词法 normpath）。
func ci18PyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "/"
		}
		p = ci18PyJoin(wd, p)
	}
	return ci18PyNormpath(p)
}

// ci18PyDirname —— posixpath.dirname 同构（= split(p)[0]，剥尾斜杠）。
func ci18PyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ci18PyRelpath —— posixpath.relpath 词法同构（两端先 abspath，再求公共前缀 + `..`）。
func ci18PyRelpath(path, start string) string {
	pa := ci18PyAbspath(path)
	sa := ci18PyAbspath(start)
	if pa == sa {
		return "."
	}
	split := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	pl, sl := split(pa), split(sa)
	i := 0
	for i < len(pl) && i < len(sl) && pl[i] == sl[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(sl)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// ci18ReadLines —— py `open(path, encoding="utf-8")`（strict）＋ universal newlines ＋ `.read().split("\n")`。
// 返回 (行, ok)；ok=false 表示取不到输入（读失败 / 非 UTF-8）⇒ 调用方 rc=2。
func ci18ReadLines(path string) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !utf8.Valid(b) {
		return nil, false
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n"), true
}

// ci18CutLineComment —— py `re.sub(r'//.*$', ”, line)`：line 无内嵌换行 ⇒ 等价于「截到首个 //」。
func ci18CutLineComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// ci18IsWordAscii — py 前瞻 `(?<![A-Za-z_])` 的字符类（ASCII）。
func ci18IsWordAscii(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// ── 正则（py 原式逐条直译；✗ 不照抄 Go 默认 ASCII 语义）───────────────────────────

var (
	ci18CJK = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	// KEY_LINE = ^([A-Za-z][\w.]*):\s*(.*)$
	ci18KeyLine = regexp.MustCompile(`^([A-Za-z][\w.]*):` + ci18PySpace + `*(.*)$`)
	// T_STATIC = (?<![A-Za-z_])t!\(\s*"([^"]+)"  —— 前瞻无 RE2 支持 ⇒ 去前瞻 + 手工判前一字符
	ci18TStatic = regexp.MustCompile(`t!\(` + ci18PySpace + `*"([^"]+)"`)
	// placeholders = %\{(\w+)[^}]*\}
	ci18Placeholder = regexp.MustCompile(`%\{(` + ci18PyWord + `+)[^}]*\}`)
	// fn 定义 = ^\s*(?:pub )?fn (\w+)
	ci18FnDef = regexp.MustCompile(`^` + ci18PySpace + `*(?:pub )?fn (` + ci18PyWord + `+)`)
	// #[cfg(test)]
	ci18CfgTest = regexp.MustCompile(`^` + ci18PySpace + `*#\[cfg\(test\)\]`)
	// writeErrorCode\(w,\s*[^,]+,\s*"([A-Z0-9_]+)"
	ci18WriteErr = regexp.MustCompile(`writeErrorCode\(w,` + ci18PySpace + `*[^,]+` + `,` + ci18PySpace + `*"([A-Z0-9_]+)"`)
	// literals_in_line 不用（check-i18n 无此函数）
)

// ci18FindStaticKeys —— py `T_STATIC.findall(line)` 同构：去前瞻（判前一字符非 [A-Za-z_]）。
func ci18FindStaticKeys(line string) []string {
	var out []string
	for _, m := range ci18TStatic.FindAllStringSubmatchIndex(line, -1) {
		start := m[0]
		if start > 0 && ci18IsWordAscii(line[start-1]) {
			continue
		}
		out = append(out, line[m[2]:m[3]])
	}
	return out
}

// ── 读面 ─────────────────────────────────────────────────────────────────────

// ci18ReadFails —— B 类读失败登记面（目录遍历 / 件读 失败 ⇒ 逐处登记「件/目录 + 原因」）。
// ★ B 类真口子修（与 py **有意分叉** · 同族先例 check_error_kinds.go 的 cekReadFails · check_doc_cmds.go 的 dcmdReadFails）：
//
//	原 py 语义 = `os.walk(onerror=None)` / 逐件 `open` 失败**静默跳过** ⇒ 判据面被抽空也不出声。
//	本件收紧：读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 ci18Run 起手清空 ⇒ 不许跨调用残留。目录面自成一句（标「读不了目录」）。
var ci18ReadFails []string

// ci18NoteReadFail —— 登记一处读失败（同一件只登一次 ⇒ ✗ 不重复点名）。
func ci18NoteReadFail(what string) {
	for _, u := range ci18ReadFails {
		if u == what {
			return
		}
	}
	ci18ReadFails = append(ci18ReadFails, what)
}

// ci18BlockedIfReadFails —— 有登记 ⇒ 打 BLOCKED 头 + 逐件点名 ⇒ rc=2；无 ⇒ 返回 -1（照常走）。
func ci18BlockedIfReadFails(stderr io.Writer) int {
	if len(ci18ReadFails) == 0 {
		return -1
	}
	fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(ci18ReadFails))
	for _, u := range ci18ReadFails {
		fmt.Fprintf(stderr, "  [读失败] %s\n", u)
	}
	return 2
}

// ci18RepoRoot —— py `dirname×3(__file__)` 的 Go 代理：从 cwd 上溯找同时含 ui/ 与 core/ 的目录（缺则 cwd）。
func ci18RepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	wd = ci18PyAbspath(wd)
	d := wd
	for {
		if d == "/" || d == "" {
			break
		}
		if ci18IsDir(ci18PyJoin(d, "ui")) && ci18IsDir(ci18PyJoin(d, "core")) {
			return d
		}
		nd := ci18PyDirname(d)
		if nd == d || nd == "" {
			break
		}
		d = nd
	}
	return wd
}

func ci18IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ci18RustFiles —— py rust_files()：os.walk(ROOT/ui/src) 顶层优先 · 每层文件名 sorted · 子目录按 readdir 原序下钻 ·
// followlinks=False（符号链接目录整个跳过，既不下钻也不当文件产出）。
func ci18RustFiles(root string) []string {
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			// ★ B 类真口子修（原 py os.walk onerror=None ⇒ 读不了的目录静默跳过 · ✗ 不再静默）。
			// ★ 只有「在盘却读不了」才是真读失败（✗ 不把「目录本就缺席」记成读失败 · 与 py 语义及同族先例 cek 一致）。
			if fi, serr := os.Stat(dir); serr == nil && fi.IsDir() {
				ci18NoteReadFail(ci18PyRelpath(dir, root) + "（读不了目录：" + err.Error() + "）")
			}
			return
		}
		entries, err := f.ReadDir(-1) // 保 readdir 原序
		f.Close()
		if err != nil {
			if fi, serr := os.Stat(dir); serr == nil && fi.IsDir() {
				ci18NoteReadFail(ci18PyRelpath(dir, root) + "（读不了目录：" + err.Error() + "）")
			}
			return
		}
		var files, dirs []string
		for _, e := range entries {
			name := e.Name()
			isLink := e.Type()&os.ModeSymlink != 0
			full := ci18PyJoin(dir, name)
			st, serr := os.Stat(full) // py entry.is_dir() 跟随符号链接
			isDir := serr == nil && st.IsDir()
			switch {
			case isDir && !isLink:
				dirs = append(dirs, name)
			case isDir && isLink:
				// py：符号链接目录进 dirnames，followlinks=False ⇒ 不下钻也不当文件
			default:
				files = append(files, name)
			}
		}
		sort.Strings(files) // py：for n in sorted(names)
		for _, n := range files {
			if strings.HasSuffix(n, ".rs") {
				out = append(out, ci18PyJoin(dir, n))
			}
		}
		for _, d := range dirs {
			walk(ci18PyJoin(dir, d))
		}
	}
	walk(ci18PyJoin(root, "ui", "src"))
	return out
}

// ci18ParseYML —— py parse_yml：逐行 KEY_LINE.match ⇒ {键: 值.strip()}。
func ci18ParseYML(path string) (map[string]string, bool) {
	lines, ok := ci18ReadLines(path)
	if !ok {
		return nil, false
	}
	out := map[string]string{}
	for _, line := range lines {
		if m := ci18KeyLine.FindStringSubmatch(line); m != nil {
			out[m[1]] = ci18PyStrip(m[2])
		}
	}
	return out, true
}

// ci18Placeholders —— py placeholders(value)：%{name} 的 name 集合。
func ci18Placeholders(value string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range ci18Placeholder.FindAllStringSubmatch(value, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

type ci18Row struct {
	rel  string
	line int
	fn   string
	code string
}

// ci18ChineseLiterals —— py chinese_literals()。
func ci18ChineseLiterals(root string) ([]ci18Row, bool) {
	var rows []ci18Row
	for _, path := range ci18RustFiles(root) {
		rel := ci18PyRelpath(path, root)
		fn, inTest := "?", false
		lines, ok := ci18ReadLines(path)
		if !ok {
			return nil, false
		}
		for i, line := range lines {
			if m := ci18FnDef.FindStringSubmatch(line); m != nil {
				fn = m[1]
			}
			if ci18CfgTest.MatchString(line) {
				inTest = true
			}
			code := ci18CutLineComment(line)
			if !ci18CJK.MatchString(code) || !strings.Contains(code, `"`) {
				continue
			}
			if inTest || strings.HasPrefix(fn, "test_") || strings.HasPrefix(fn, "parses") ||
				strings.HasPrefix(fn, "bench") || strings.HasPrefix(fn, "sample") {
				continue
			}
			rows = append(rows, ci18Row{rel, i + 1, fn, ci18PyStrip(code)})
		}
	}
	return rows, true
}

// ── py json 原语 ──────────────────────────────────────────────────────────────

// ci18JSONString —— py json.dumps(<str>, ensure_ascii=False) 的字符串编码同构。
func ci18JSONString(s string) string {
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

type ci18KV struct {
	K string
	V any
}
type ci18JObj []ci18KV

func ci18Indent(sb *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		sb.WriteString("  ")
	}
}

// ci18JSONDump —— py json.dumps(obj, indent=2, ensure_ascii=False, sort_keys=False) 同构（仅覆盖本件用到的形状）。
func ci18JSONDump(sb *strings.Builder, v any, depth int) {
	switch t := v.(type) {
	case ci18JObj:
		if len(t) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for i, kv := range t {
			ci18Indent(sb, depth+1)
			sb.WriteString(ci18JSONString(kv.K))
			sb.WriteString(": ")
			ci18JSONDump(sb, kv.V, depth+1)
			if i < len(t)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		ci18Indent(sb, depth)
		sb.WriteString("}")
	case []string:
		if len(t) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, s := range t {
			ci18Indent(sb, depth+1)
			sb.WriteString(ci18JSONString(s))
			if i < len(t)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		ci18Indent(sb, depth)
		sb.WriteString("]")
	case string:
		sb.WriteString(ci18JSONString(t))
	case int:
		sb.WriteString(strconv.Itoa(t))
	case nil:
		sb.WriteString("null")
	default:
		sb.WriteString("null")
	}
}

// ci18PyRepr —— py repr() 的近似同构（覆盖本件会触发的值：None/bool/int/float/str）。
func ci18PyRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	case string:
		return "'" + strings.ReplaceAll(t, "'", "\\'") + "'"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ── 主逻辑 ───────────────────────────────────────────────────────────────────

func ci18HasArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

func ci18SortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ci18OnlyIn —— sorted(a - b)（码点序；UTF-8 字节序与码点序对串同序）。
func ci18OnlyIn(a, b map[string]string) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func ci18Head(items []string, n int) []string {
	if len(items) < n {
		return items
	}
	return items[:n]
}

// ci18SortListStr —— py `sorted(list) or "-"` 在 %s 下的字符串形态（str(['a','b']) == "['a', 'b']"）。
func ci18SortListStr(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "'" + s + "'"
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// ci18Baseline —— py load_baseline 的返回面。
type ci18Baseline struct {
	raw     map[string]any
	allowed []any
}

func ci18LoadBaseline(path string) (ci18Baseline, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return ci18Baseline{raw: map[string]any{}, allowed: nil}, true // 不存在 ⇒ {"allowed": []}
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return ci18Baseline{}, false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return ci18Baseline{}, false // py json.load 抛 ⇒ 取不到输入（C 类 ⇒ rc=2）
	}
	var allowed []any
	if v, ok := m["allowed"]; ok {
		if lst, ok := v.([]any); ok {
			allowed = lst
		}
	}
	return ci18Baseline{raw: m, allowed: allowed}, true
}

func (b ci18Baseline) declaredCount() (int, bool) {
	v, ok := b.raw["allowed_count"]
	if !ok || v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok {
		return 0, false // bool / string 等
	}
	if f < 0 || f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

func ci18EntryFile(a map[string]any) string {
	if v, ok := a["file"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func ci18EntryContains(a map[string]any) (string, bool) {
	v, ok := a["contains"]
	if !ok {
		return "", true // 缺键 ⇒ py .get(...,"") ⇒ ""
	}
	if v == nil {
		return "", false // 显式 null 在 py 里 != str（`None in code` 会抛）⇒ 跳过该条
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func ci18IsAllowed(allowed []any, rel, code string) bool {
	for _, e := range allowed {
		a, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if ci18EntryFile(a) != rel {
			continue
		}
		c, ok := ci18EntryContains(a)
		if !ok {
			continue
		}
		if strings.Contains(code, c) {
			return true
		}
	}
	return false
}

// ci18RatchetLedgerLast —— py ratchet_ledger_last(name)：台账里该条件的最后一条 `new`；读不到 ⇒ (nil,false)。
func ci18RatchetLedgerLast(path, name string) (any, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, false
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, false
	}
	var last any
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		line = ci18PyStrip(line)
		if line == "" {
			continue
		}
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			return nil, false // py ValueError ⇒ None
		}
		if nv, ok := rec["name"]; ok {
			if ns, ok := nv.(string); ok && ns == name {
				if v, ok := rec["new"]; ok {
					last = v
					found = true
				}
			}
		}
	}
	return last, found
}

// ci18JSONNumEq —— py `ledger_last != declared` 的近似同构。
func ci18JSONNumEq(v any, declared int) bool {
	f, ok := v.(float64)
	if !ok {
		return false
	}
	return f == float64(declared)
}

// ci18GoCodes —— py G5：扫 core/internal/api/*.go 里的 writeErrorCode(w, _, "CODE")。
func ci18GoCodes(root string) []string {
	apiDir := ci18PyJoin(root, "core", "internal", "api")
	if !ci18IsDir(apiDir) {
		return nil
	}
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		// ★ B 类真口子修（原读不了目录即静默 return nil · ✗ 不再静默）：登记 + 入口判红 + rc=2。
		ci18NoteReadFail(ci18PyRelpath(apiDir, root) + "（读不了目录：" + err.Error() + "）")
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names) // py sorted(os.listdir(...))
	set := map[string]bool{}
	for _, fn := range names {
		if !strings.HasSuffix(fn, ".go") || strings.HasSuffix(fn, "_test.go") {
			continue
		}
		b, rerr := os.ReadFile(ci18PyJoin(apiDir, fn))
		if rerr != nil {
			// ★ B 类真口子修（原读不了件即静默 continue · ✗ 不再静默）：登记 + 入口判红 + rc=2。
			ci18NoteReadFail(ci18PyRelpath(ci18PyJoin(apiDir, fn), root) + "（" + rerr.Error() + "）")
			continue
		}
		for _, m := range ci18WriteErr.FindAllStringSubmatch(string(b), -1) {
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

// ci18DocsOutRoot —— py docs_out_root(root)：仓内 <root>/docs/项目文档 优先，缺则仓外 <ZERG_DOCS_ALT|../Zerg-内部文档>/项目文档。
func ci18DocsOutRoot(root string) (string, string) {
	r := ci18PyAbspath(root)
	alt := os.Getenv("ZERG_DOCS_ALT")
	if ci18PyStrip(alt) == "" {
		alt = ci18PyJoin(ci18PyDirname(r), "Zerg-内部文档")
	}
	c0 := ci18PyJoin(r, "docs", "项目文档")
	c1 := ci18PyJoin(alt, "项目文档")
	for _, p := range []string{c0, c1} {
		if ci18IsDir(p) {
			return p, ""
		}
	}
	return "", fmt.Sprintf("项目文档取源根未找到（仓内根 %s ✗ · 仓外根 %s ✗）", c0, c1)
}

// checkI18nGateMain —— 入口（从 cwd 推仓根）。
func checkI18nGateMain(stdout, stderr io.Writer, argv []string) int {
	return ci18Run(ci18RepoRoot(), stdout, stderr, argv)
}

// ci18Run —— 与 py main() 逐字对齐的实现体（root 显式传入 ⇒ 单测可注入夹具根）。
func ci18Run(root string, stdout, stderr io.Writer, argv []string) int {
	ci18ReadFails = nil // ★ 自清：包级登记面不许跨调用残留（否则上一 Test 残留一进门就 BLOCKED）
	listOnly := ci18HasArg(argv, "--list")

	zh, ok := ci18ParseYML(ci18PyJoin(root, "ui", "locales", "zh-CN.yml"))
	if !ok {
		fmt.Fprintf(stderr, "check-i18n: 读 ui/locales/zh-CN.yml 失败（取不到输入 · 不给结论）· rc=2\n")
		return 2
	}
	en, ok := ci18ParseYML(ci18PyJoin(root, "ui", "locales", "en.yml"))
	if !ok {
		fmt.Fprintf(stderr, "check-i18n: 读 ui/locales/en.yml 失败（取不到输入 · 不给结论）· rc=2\n")
		return 2
	}

	var errors, warns []string

	// G1 键对称
	onlyZh := ci18OnlyIn(zh, en)
	onlyEn := ci18OnlyIn(en, zh)
	if len(onlyZh) > 0 {
		errors = append(errors, fmt.Sprintf("G1 仅 zh-CN 有（%d）：%s", len(onlyZh), strings.Join(ci18Head(onlyZh, 8), ", ")))
	}
	if len(onlyEn) > 0 {
		errors = append(errors, fmt.Sprintf("G1 仅 en 有（%d）：%s", len(onlyEn), strings.Join(ci18Head(onlyEn, 8), ", ")))
	}

	// G2 引用未定义
	used := map[string]bool{}
	for _, path := range ci18RustFiles(root) {
		lines, ok := ci18ReadLines(path)
		if !ok {
			fmt.Fprintf(stderr, "check-i18n: 读 %s 失败（取不到输入 · 不给结论）· rc=2\n", path)
			return 2
		}
		for _, line := range lines {
			for _, k := range ci18FindStaticKeys(ci18CutLineComment(line)) {
				used[k] = true
			}
		}
	}
	var missing []string
	for k := range used {
		if _, ok := zh[k]; !ok {
			missing = append(missing, k)
		} else if _, ok := en[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		errors = append(errors, fmt.Sprintf("G2 代码引用但未定义（%d）：%s", len(missing), strings.Join(ci18Head(missing, 8), ", ")))
	}

	// G3 占位符一致
	var diff []string
	both := map[string]bool{}
	for k := range zh {
		if _, ok := en[k]; ok {
			both[k] = true
		}
	}
	bothKeys := make([]string, 0, len(both))
	for k := range both {
		bothKeys = append(bothKeys, k)
	}
	sort.Strings(bothKeys)
	for _, k := range bothKeys {
		pz, pe := ci18Placeholders(zh[k]), ci18Placeholders(en[k])
		if strings.Join(pz, "\x00") != strings.Join(pe, "\x00") {
			diff = append(diff, fmt.Sprintf("%s（zh=%s en=%s）", k, ci18SortListStr(pz), ci18SortListStr(pe)))
		}
	}
	if len(diff) > 0 {
		errors = append(errors, fmt.Sprintf("G3 占位符不一致（%d）：%s", len(diff), strings.Join(ci18Head(diff, 5), "; ")))
	}

	// G4 中文门禁
	baselinePath := ci18PyJoin(root, "ui", "scripts", "i18n-baseline.json")
	bl, ok := ci18LoadBaseline(baselinePath)
	if !ok {
		fmt.Fprintf(stderr, "check-i18n: 读 ui/scripts/i18n-baseline.json 失败（取不到输入 · 不给结论）· rc=2\n")
		return 2
	}
	allowed := bl.allowed
	rows, ok := ci18ChineseLiterals(root)
	if !ok {
		fmt.Fprintf(stderr, "check-i18n: 读 ui/src/**/*.rs 失败（取不到输入 · 不给结论）· rc=2\n")
		return 2
	}

	var extra []ci18Row
	for _, r := range rows {
		if !ci18IsAllowed(allowed, r.rel, r.code) {
			extra = append(extra, r)
		}
	}
	var stale []map[string]any
	for _, a := range allowed {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}
		hit := false
		for _, r := range rows {
			if ci18EntryFile(am) == r.rel {
				if c, ok := ci18EntryContains(am); ok && strings.Contains(r.code, c) {
					hit = true
					break
				}
			}
		}
		if !hit {
			stale = append(stale, am)
		}
	}

	if listOnly {
		if rc := ci18BlockedIfReadFails(stderr); rc >= 0 {
			return rc
		}
		fmt.Fprintf(stdout, "剩余中文字面量 %d 行（基线允许 %d 行）：\n", len(rows), len(allowed))
		for _, r := range rows {
			mark := "★新增"
			if ci18IsAllowed(allowed, r.rel, r.code) {
				mark = "允许"
			}
			fmt.Fprintf(stdout, "  [%s] %s:%d %s | %s\n", mark, r.rel, r.line, r.fn, ci18HeadRunes(r.code, 90))
		}
		return 0
	}

	if ci18HasArg(argv, "--tsv") {
		if rc := ci18BlockedIfReadFails(stderr); rc >= 0 {
			return rc
		}
		tsvDir := ci18PyStrip(os.Getenv("ZERG_I18N_AUDIT_OUT"))
		if tsvDir == "" {
			base, missingMsg := ci18DocsOutRoot(root)
			if base == "" {
				fmt.Fprintf(stdout, "check-i18n: %s\n", chooseStr(missingMsg, "项目文档取源根未找到"))
				fmt.Fprintf(stdout, "⇒ 输出目录不可判（不给结论）· rc=2\n")
				return 2
			}
			tsvDir = ci18PyJoin(base, "v2.5.9", "i18n-audit")
		}
		out := ci18PyJoin(tsvDir, "L2-现状-中文字面量.tsv")
		if err := os.MkdirAll(ci18PyDirname(out), 0o777); err != nil {
			fmt.Fprintf(stderr, "check-i18n: 建输出目录失败（取不到输入 · 不给结论）· rc=2\n")
			return 2
		}
		var sb strings.Builder
		sb.WriteString("状态\t文件\t行号\t函数\t代码\n")
		for _, r := range rows {
			st := "★未登记"
			if ci18IsAllowed(allowed, r.rel, r.code) {
				st = "允许(基线)"
			}
			fmt.Fprintf(&sb, "%s\t%s\t%d\t%s\t%s\n", st, r.rel, r.line, r.fn, r.code)
		}
		if err := os.WriteFile(out, []byte(sb.String()), 0o666); err != nil {
			fmt.Fprintf(stderr, "check-i18n: 写 %s 失败（取不到输入 · 不给结论）· rc=2\n", out)
			return 2
		}
		fmt.Fprintf(stdout, "写出: %s（%d 行）\n", out, len(rows))
		if len(extra) > 0 {
			return 1
		}
		return 0
	}

	if len(extra) > 0 {
		errors = append(errors, fmt.Sprintf("G4 出现未登记的中文 UI 字面量（%d 行，须抽成 i18n 键或登记进基线）：", len(extra)))
		for _, r := range extra {
			errors = append(errors, fmt.Sprintf("      %s:%d %s | %s", r.rel, r.line, r.fn, ci18HeadRunes(r.code, 90)))
		}
	}
	if len(stale) > 0 {
		var parts []string
		for _, a := range ci18HeadMap(stale, 5) {
			c, _ := ci18EntryContains(a)
			parts = append(parts, fmt.Sprintf("%s|%s", ci18EntryFile(a), ci18HeadRunes(c, 24)))
		}
		warns = append(warns, fmt.Sprintf("G4 基线里有 %d 条已不存在（可收紧基线）：%s", len(stale), strings.Join(parts, ", ")))
	}

	// G4 方向判据
	declared, declaredOK := bl.declaredCount()
	if _, err := os.Stat(baselinePath); err == nil {
		if !declaredOK {
			errors = append(errors, fmt.Sprintf("G4 基线件缺 `allowed_count` 声明数（现 %s）：`baseline-ratchet.py` 的条件 "+
				"`i18n.allowed_count` 读的就是这一格 ⇒ 只许收不许放的方向判据无锚 ⇒ fail-closed 判红", ci18PyRepr(bl.raw["allowed_count"])))
		} else if declared < len(allowed) {
			errors = append(errors, fmt.Sprintf("G4 **放开被拦**：白名单实条数 %d > 声明数 %d ⇒ 基线只许收不许放；"+
				"唯一通道 = `python3 scripts/gates/baseline-ratchet.py --write --reason '…' --allow-raise`"+
				"（改数并留台账）", len(allowed), declared))
		} else if declared > len(allowed) {
			ledgerLast, found := ci18RatchetLedgerLast(ci18PyJoin(root, "scripts", "gates", "baseline-audit.jsonl"), "i18n.allowed_count")
			if found && !ci18JSONNumEq(ledgerLast, declared) {
				errors = append(errors, fmt.Sprintf("G4 **放宽未登记**：声明数 %d 与棘轮台账（scripts/gates/baseline-audit.jsonl）"+
					"里 `i18n.allowed_count` 最后一条 new=%s 不符 ⇒ 上调只许走 "+
					"`python3 scripts/gates/baseline-ratchet.py --write --reason '…' --allow-raise`", declared, ci18PyRepr(ledgerLast)))
			} else {
				warns = append(warns, fmt.Sprintf("G4 声明数 %d > 白名单实条数 %d（收的方向不拦，只提示）：请走唯一写入口把声明数同降 —— "+
					"`python3 scripts/gates/baseline-ratchet.py --write --reason '…'`", declared, len(allowed)))
			}
		}
	}

	// G5 错误码 ↔ UI 键
	goCodes := ci18GoCodes(root)
	// ★ B 类真口子修：所有读面（目录遍历 / 件读）跑完 ⇒ 有登记即 BLOCKED rc=2（✗ 不给结论）。
	if rc := ci18BlockedIfReadFails(stderr); rc >= 0 {
		return rc
	}
	uiCodes := map[string]bool{}
	for k := range zh {
		if strings.HasPrefix(k, "apierr.") {
			uiCodes[strings.ToUpper(k[len("apierr."):])] = true
		}
	}
	var missingCodes, extraCodes []string
	for _, c := range goCodes {
		if !uiCodes[c] {
			missingCodes = append(missingCodes, c)
		}
	}
	for c := range uiCodes {
		found := false
		for _, g := range goCodes {
			if g == c {
				found = true
				break
			}
		}
		if !found {
			extraCodes = append(extraCodes, c)
		}
	}
	sort.Strings(extraCodes)
	if len(missingCodes) > 0 {
		warns = append(warns, fmt.Sprintf("G5 服务端错误码缺 UI 本地化键（%d）：%s", len(missingCodes), strings.Join(ci18Head(missingCodes, 8), ", ")))
	}
	if len(extraCodes) > 0 {
		warns = append(warns, fmt.Sprintf("G5 UI 有 apierr.* 键但服务端已无该码（%d）：%s", len(extraCodes), strings.Join(ci18Head(extraCodes, 8), ", ")))
	}

	// 未引用键
	var unused []string
	zhKeys := ci18SortedKeys(zh)
	for _, k := range zhKeys {
		if used[k] || strings.Contains(k, "[") || strings.HasPrefix(k, "apierr.") {
			continue
		}
		unused = append(unused, k)
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		warns = append(warns, fmt.Sprintf("未引用键 %d 个（可能被动态引用 t!(变量)，据此删键前先确认）：%s", len(unused), strings.Join(ci18Head(unused, 8), ", ")))
	}

	// 机器面
	if ci18HasArg(argv, "--json") {
		var declaredVal any
		if declaredOK {
			declaredVal = declared
		}
		obj := ci18JObj{
			{"id", "i18n-gate.v1"},
			{"counts", ci18JObj{
				{"keys_zh", len(zh)},
				{"keys_en", len(en)},
				{"static_refs", len(used)},
				{"chinese_literals", len(rows)},
				{"baseline_allowed", len(allowed)},
				{"baseline_declared", declaredVal},
			}},
			{"errors", errors},
			{"warns", warns},
			{"extra", ci18ExtraStrs(extra)},
			{"stale", ci18StaleStrs(stale)},
		}
		var sb strings.Builder
		ci18JSONDump(&sb, obj, 0)
		fmt.Fprintf(stdout, "%s\n", sb.String())
		if len(errors) > 0 {
			return 1
		}
		return 0
	}

	// 人面
	fmt.Fprintf(stdout, "=== i18n 门禁 ===\n")
	fmt.Fprintf(stdout, "  键数 zh=%d en=%d ｜ 静态引用 %d ｜ 剩余中文字面量 %d 行（基线 %d）\n",
		len(zh), len(en), len(used), len(rows), len(allowed))
	for _, w := range warns {
		fmt.Fprintf(stdout, "  [warn] %s\n", w)
	}
	for _, e := range errors {
		fmt.Fprintf(stdout, "  [ERROR] %s\n", e)
	}
	if len(errors) > 0 {
		fmt.Fprintf(stdout, "→ 门禁失败（%d 项）\n", len(errors))
		return 1
	}
	fmt.Fprintf(stdout, "→ 门禁通过\n")
	return 0
}

// ci18HeadRunes —— py `s[:N]`（按码点切；✗ 不按 byte 切，否则多字节字符被劈开）。
func ci18HeadRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func ci18HeadMap(m []map[string]any, n int) []map[string]any {
	if len(m) <= n {
		return m
	}
	return m[:n]
}

func chooseStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func ci18ExtraStrs(extra []ci18Row) []string {
	out := make([]string, 0, len(extra))
	for _, r := range extra {
		out = append(out, fmt.Sprintf("%s:%d | %s", r.rel, r.line, r.code))
	}
	return out
}

func ci18StaleStrs(stale []map[string]any) []string {
	out := make([]string, 0, len(stale))
	for _, a := range stale {
		c, _ := ci18EntryContains(a)
		out = append(out, fmt.Sprintf("%s|%s", ci18EntryFile(a), c))
	}
	return out
}
