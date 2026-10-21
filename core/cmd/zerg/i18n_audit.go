// i18n_audit.go —— `ui/scripts/i18n_audit.py`（225 行）的 **Go 真搬**（M2 · 批 · 入口 `i18nAuditGateMain`）。
//
// 本件形态：静态分析器 · 纯静态只读源面（扫 ui/src/**/*.rs 与 ui/locales/{zh-CN,en}.yml）
// ＋ 生成器（只往**显式 outdir** 写两份报告：TSV + MD）。外部依赖仅 Python 标准库
// （os/re/sys/glob/collections）⇒ 无子进程 · 无联网 · 无真 TTY · 无第三方包 ⇒ 适格。
// 现读 225 行全文核：零 `subprocess`/`socket`/`urllib`/`time.`/`datetime`/`random`/
// `tempfile`；唯二 IO 面是「读源件（ui/src·ui/locales）」与「往 outdir 写报告」。
//
// 验收判据 = 同输入（同一棵仓树 / 同一 outdir）下，py 原件 ⟷ 本入口 产出
// **stdout 逐字节相同** ＋ 两份报告件字节相同 ＋ 逐格 rc/stdout/stderr 三面相同。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① 遍历序：py `glob.glob(<root>/ui/src/**/*.rs, recursive=True)` → **sorted**；
//	   本件保 readdir 后 `sort.Strings`（全路径串升序 = py sorted 的码点序，ASCII 面）。
//	   py glob 默认**跳过隐藏项**（名以 `.` 起）⇒ 本件同步跳过；`/target/` 段剔除照 py。
//	② 文本行迭代 = py 文本模式（universal newlines：`\r\n`/裸 `\r` 都折 `\n`）＋
//	   按 `\n` 切、**每行带尾换行**（py `for line in f`）⇒ 自写 `iaudLines` 逐字复刻。
//	③ `re.sub(r'//.*$', line, ...)`：py 非多行 `$` 匹配「串尾换行之前」，而 Go RE2 `$`
//	   只匹配串尾 ⇒ **不可照译**，自写 `iaudStripLineComment`（删首个 `//` 到行内容末、
//	   保留尾 `\n`），并在未覆盖面登记该语义差来源。
//	④ `.strip()` = py Unicode 空白（含 `\x1c-\x1f`）：自写 `iaudStrip`，✗ 不用 strings.TrimSpace。
//	⑤ 切片 `s[:N]` 按**码点**：自写 `iaudCut`（按 rune 切），✗ 不按 byte 切。
//	⑥ Counter.most_common = 计数降序 + **同数保首次遇到序** ⇒ `sort.SliceStable` 复现；
//	   `sorted(dict, key=-len)` 同理。
//	⑦ py `%s` 印 **list repr**（`['a', 'b']`，空表走 `or "无"`）⇒ 自写 `iaudPyReprStrList`。
//	⑧ tlit/tdyn 带**负向后顾** `(?<![A-Za-z_])`（RE2 不支持）⇒ 自写：先取内层匹配，
//	   再按「起址前一字节 ∉ [A-Za-z_]」过滤，与零宽后顾同集。
//	⑨ 拼路径一律自写 `iaudPyJoin`（posixpath.join 语义），✗ 不用 filepath.Join。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `repo_root(dirname(abspath(__file__)))` ⇒ 起点是
//	    **脚本所在目录**；本件无 `__file__` 语境 ⇒ 从 **cwd** 上溯找「同时含 ui/ 与 core/」的祖先。
//	    「脚本位置 == cwd」（仓根执行 `python3 ui/scripts/i18n_audit.py`）时两者读数逐字相同；
//	    否则**路径推导面不同** ⇒ 不属逐字对齐承诺。`abspath` 的 `..`/`.` 归一未复刻（见未覆盖面）。
//	(b) **自指检查**：本件扫描面 = `ui/src/**/*.rs`（现读 19 件）＋ `ui/locales/{zh-CN,en}.yml`
//	    ⇒ **零 `.go` 件**，本件（落 `core/cmd/zerg/`）**不在自身扫描面内**。量化：全件零绝对私有路径
//	    字面量 · 退码仅 0/1/2 · 不写形如「JSON 键名连续引号原串」的写法（一律描述式）。
//
// 未覆盖面（照实，逐条）：见回执「未做到」栏。要点：① py `\s`/`\d`/`\w` Unicode 语义
// （Go RE2 默认 ASCII）在夹 U+00A0 类空白时才分叉，本件按 ASCII 译并登记；② py 抛未捕
// 异常（读不了的件/locales 非法 UTF-8）＝回溯 rc=1，本件按新口径 fail-closed rc=2（声明口径差）；
// ③ `errors="replace"` 的替换粒度差（py 按非法序列、Go 按 run 各一 U+FFFD）；④ list repr 对含
// 单引号/反斜杠的串的转义未复刻（本件实面为路径/键名，不触发）。
//
// 本单边界（照实）：**已接线（inproc 表分派）** —— 不在 `main.go` 注册命令；不删 py 原件；
// **已接线**：入口 `i18nAuditGateMain` 由 `gateCheckGoEntry["i18n-audit"]` 分派（`zerg gate check i18n-audit`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 正则（照 py 逐字；`\s`/`\d`/`\w` 走 Go RE2 的 ASCII 语义 —— 见未覆盖面①）──────

var (
	iaudCJK      = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	iaudUICall   = regexp.MustCompile(`\b(?:text|label|heading|Button::new|button|title|tooltip|on_hover_text|RichText::new|SelectableLabel::new|TextEdit::singleline|TextEdit::multiline|ComboBox::from_label|CollapsingHeader::new|Window::new|menu_button|checkbox)\s*\(`)
	iaudLogCall  = regexp.MustCompile(`\b(?:println!|eprintln!|log::|tracing::|dbg!)`)
	iaudErrCall  = regexp.MustCompile(`\b(?:panic!|unreachable!|todo!|expect\(|unwrap_or_else\(|Err\()`)
	iaudTMacro   = regexp.MustCompile(`t!\(`)
	iaudLiteral  = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	iaudKeyDef   = regexp.MustCompile(`^([A-Za-z_][\w.]*):\s*`)
	iaudTLit     = regexp.MustCompile(`t!\(\s*"((?:[^"\\]|\\.)*)"`)
	iaudTDyn     = regexp.MustCompile(`t!\(\s*[^")\s]`)
	iaudLitSlash = regexp.MustCompile(`^\s*/`)
	iaudHTTP     = regexp.MustCompile(`^https?://`)
	iaudFmtOnly  = regexp.MustCompile(`^[\s{}:.<>\d%\-]*$`)
	iaudAsset    = regexp.MustCompile(`(?i)\.(?:ttf|ttc|png|svg|json|md|db|log|so|dylib)$`)
	iaudData     = regexp.MustCompile(`\b(?:const|static)\b`)
	iaudKeywords = regexp.MustCompile(`Keywords|\[\s*$`)
)

// ── posixpath 语义（照 py os.path.*，✗ 不用 filepath.Join）──────────────────

// iaudPyJoin —— posixpath.join：空段忽略 · 后段绝对路径丢弃前段 · `..` 原样留串。
func iaudPyJoin(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "/") {
			b.Reset()
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "/") {
			b.WriteString("/")
		}
		b.WriteString(p)
	}
	return b.String()
}

// iaudPyAbs —— posixpath.abspath 的**简化**：绝对路径原样返回，否则 join(cwd, p)。
// ✗ 未复刻 `..`/`.` 归一（现读调用点输入均为干净绝对路径 ⇒ 不触发；见未覆盖面）。
func iaudPyAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	return iaudPyJoin(wd, p)
}

// iaudPyDirname —— posixpath.dirname（剥尾斜杠后去末段）。
func iaudPyDirname(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	head := path[:i+1]
	// posixpath.split 语义：去掉 head 末尾多余斜杠，但根 "/" 保留。
	for len(head) > 1 && strings.HasSuffix(head, "/") {
		head = head[:len(head)-1]
	}
	if head == "" {
		return ""
	}
	if head != "/" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// iaudPyRelpath —— posixpath.relpath（绝对路径公共前缀法；输入均为绝对路径）。
func iaudPyRelpath(path, start string) string {
	p := iaudPyAbs(path)
	s := iaudPyAbs(start)
	if p == s {
		return "."
	}
	pp := strings.Split(p, "/")
	sp := strings.Split(s, "/")
	i := 0
	for i < len(pp) && i < len(sp) && pp[i] == sp[i] {
		i++
	}
	out := make([]string, 0, len(sp)-i+len(pp)-i)
	for j := i; j < len(sp); j++ {
		out = append(out, "..")
	}
	out = append(out, pp[i:]...)
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

func iaudIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ── py 字符串原语（✗ 不用 strings.TrimSpace / 按 byte 切片）────────────────

// iaudIsPySpace —— py str.strip() 的空白集：Unicode 空白 ＋ ASCII 的 `\x1c-\x1f`。
func iaudIsPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// iaudStrip —— py str.strip() 等价（按 rune 迭代）。
func iaudStrip(s string) string {
	i, j := 0, len(s)
	for i < j {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !iaudIsPySpace(r) {
			break
		}
		i += sz
	}
	for j > i {
		r, sz := utf8.DecodeLastRuneInString(s[:j])
		if !iaudIsPySpace(r) {
			break
		}
		j -= sz
	}
	return s[i:j]
}

// iaudCut —— py 切片 `s[:n]`（按**码点**截前 n 个；不足则全取）。
func iaudCut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	c := 0
	for i := range s {
		if c == n {
			return s[:i]
		}
		c++
	}
	return s
}

// ── 仓根 / 遍历 / 行迭代 ─────────────────────────────────────────────────

// iaudRepoRoot —— py `repo_root`：从 start 上溯，首个「同时含 ui/ 与 core/」的祖先。
func iaudRepoRoot(start string) string {
	d := iaudPyAbs(start)
	for d != "/" {
		if iaudIsDir(iaudPyJoin(d, "ui")) && iaudIsDir(iaudPyJoin(d, "core")) {
			return d
		}
		nd := iaudPyDirname(d)
		if nd == d || nd == "" {
			break
		}
		d = nd
	}
	return iaudPyAbs(start)
}

// iaudRustFiles —— py `rust_files`：glob `<root>/ui/src/**/*.rs`（recursive）→ 剔 `/target/` → sorted。
// 保 glob 语义：跳过隐藏项（名以 `.` 起）；目录跟随符号链接（py glob 语义）。
func iaudRustFiles(root string) []string {
	base := iaudPyJoin(root, "ui", "src")
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			full := iaudPyJoin(dir, name)
			st, err := os.Stat(full)
			if err != nil {
				continue
			}
			if st.IsDir() {
				walk(full)
				continue
			}
			if strings.HasSuffix(name, ".rs") {
				if strings.Contains(full, "/target/") {
					continue
				}
				out = append(out, full)
			}
		}
	}
	walk(base)
	sort.Strings(out)
	return out
}

// iaudLines —— py 文本模式行迭代：先 universal newlines 折行，再按 `\n` 切、**每行带尾换行**。
func iaudLines(content string) []string {
	s := strings.ReplaceAll(content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// iaudStripLineComment —— py `re.sub(r'//.*$', ”, line)` 的等价（py 非多行 `$` 匹配尾换行之前）：
// 删首个 `//` 到「行内容末」（保留唯一尾 `\n`）。
func iaudStripLineComment(s string) string {
	idx := strings.Index(s, "//")
	if idx < 0 {
		return s
	}
	end := len(s)
	if strings.HasSuffix(s, "\n") {
		end = len(s) - 1
	}
	if idx > end {
		return s
	}
	return s[:idx] + s[end:]
}

// iaudLiterals —— py `literals_in_line`（findall 组 1）。
func iaudLiterals(line string) []string {
	ms := iaudLiteral.FindAllStringSubmatch(line, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

// ── 读面（取不到/不可读一律不静默）────────────────────────────────────────

// iaudReadReplace —— py `open(..., encoding="utf-8", errors="replace")` 的等价读面（非法字节 → U+FFFD）。
func iaudReadReplace(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return strings.ToValidUTF8(string(b), string(utf8.RuneError)), nil
	}
	return string(b), nil
}

// iaudReadStrict —— py `open(..., encoding="utf-8")`（默认 strict）的等价读面。
func iaudReadStrict(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("件不是合法 UTF-8：%s", path)
	}
	return string(b), nil
}

// ── classify（py 逐段）────────────────────────────────────────────────────

type iaudRow struct {
	rel    string
	lineNo int
	cls    string
	lit    string
	code   string
}

func iaudClassify(root string) ([]iaudRow, error) {
	var rows []iaudRow
	for _, f := range iaudRustFiles(root) {
		rel := iaudPyRelpath(f, root)
		content, err := iaudReadReplace(f)
		if err != nil {
			return nil, err
		}
		for idx, line := range iaudLines(content) {
			i := idx + 1
			stripped := iaudStrip(line)
			if !iaudCJK.MatchString(iaudStripLineComment(line)) {
				continue
			}
			if strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "*") || strings.HasPrefix(stripped, "/*") {
				rows = append(rows, iaudRow{rel, i, "注释", "(comment)", iaudCut(stripped, 100)})
				continue
			}
			code := iaudStripLineComment(line)
			for _, lit := range iaudLiterals(code) {
				if !iaudCJK.MatchString(lit) {
					continue
				}
				if iaudTMacro.MatchString(line) && lit == "" {
					continue
				}
				var cls string
				switch {
				case iaudLogCall.MatchString(line):
					cls = "日志"
				case iaudErrCall.MatchString(line):
					cls = "错误/panic"
				case iaudLitSlash.MatchString(lit) || iaudHTTP.MatchString(lit):
					cls = "接口路径/URL"
				case iaudFmtOnly.MatchString(lit):
					cls = "格式串"
				case iaudAsset.MatchString(lit):
					cls = "资产路径"
				case iaudData.MatchString(line) || iaudKeywords.MatchString(line):
					cls = "数据/常量"
				case iaudUICall.MatchString(line) || iaudTMacro.MatchString(line):
					cls = "UI文案"
				default:
					cls = "UI文案(待确认)"
				}
				rows = append(rows, iaudRow{rel, i, cls, iaudCut(lit, 120), iaudCut(stripped, 100)})
			}
		}
	}
	return rows, nil
}

// ── keys_audit（py 逐段）──────────────────────────────────────────────────

type iaudSite struct {
	file string
	line int
	ctx  string
}

func iaudIsWordByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

// iaudFindTLit —— py `tlit.findall(line)`：内层匹配 + 负向后顾 `(?<![A-Za-z_])` 过滤。
func iaudFindTLit(line string) []string {
	var out []string
	for _, loc := range iaudTLit.FindAllStringSubmatchIndex(line, -1) {
		if loc[0] > 0 && iaudIsWordByte(line[loc[0]-1]) {
			continue
		}
		out = append(out, line[loc[2]:loc[3]])
	}
	return out
}

// iaudTDynPresent —— py `tdyn.search(line)`：内层匹配 + 负向后顾过滤。
func iaudTDynPresent(line string) bool {
	for _, loc := range iaudTDyn.FindAllStringIndex(line, -1) {
		if loc[0] > 0 && iaudIsWordByte(line[loc[0]-1]) {
			continue
		}
		return true
	}
	return false
}

func iaudKeysAudit(root string) (defs map[string]map[string]bool, usedOrder []string, used map[string][]iaudSite, dyn []iaudSite, err error) {
	loc := iaudPyJoin(root, "ui", "locales")
	defs = map[string]map[string]bool{}
	for _, name := range []string{"zh-CN", "en"} {
		p := iaudPyJoin(loc, name+".yml")
		content, e := iaudReadStrict(p)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		ks := map[string]bool{}
		for _, line := range iaudLines(content) {
			if m := iaudKeyDef.FindStringSubmatch(line); m != nil {
				ks[m[1]] = true
			}
		}
		defs[name] = ks
	}
	used = map[string][]iaudSite{}
	for _, f := range iaudRustFiles(root) {
		rel := iaudPyRelpath(f, root)
		content, e := iaudReadReplace(f)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		for idx, line := range iaudLines(content) {
			i := idx + 1
			if strings.HasPrefix(iaudStrip(line), "//") {
				continue
			}
			for _, k := range iaudFindTLit(line) {
				if _, ok := used[k]; !ok {
					usedOrder = append(usedOrder, k)
				}
				used[k] = append(used[k], iaudSite{rel, i, iaudCut(iaudStrip(line), 90)})
			}
			if iaudTDynPresent(line) {
				dyn = append(dyn, iaudSite{rel, i, iaudCut(iaudStrip(line), 90)})
			}
		}
	}
	return defs, usedOrder, used, dyn, nil
}

// ── 集合 / 排序 / repr 小件 ──────────────────────────────────────────────

func iaudSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// iaudDiffSorted —— `sorted(a - b)`。
func iaudDiffSorted(a, b map[string]bool) []string {
	out := []string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// iaudPyReprStrList —— Python list repr：`['a', 'b']`（空 → `[]`）。
func iaudPyReprStrList(ss []string) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, "'"+s+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func iaudOrNoneList(ss []string) string {
	if len(ss) == 0 {
		return "无"
	}
	return iaudPyReprStrList(ss)
}

type iaudCount struct {
	key string
	n   int
}

// iaudMostCommon —— py Counter.most_common(limit)：计数降序 + 同数保首次遇到序（limit<0 = 全取）。
func iaudMostCommon(order []string, counts map[string]int, limit int) []iaudCount {
	idx := map[string]int{}
	for i, k := range order {
		idx[k] = i
	}
	ord := append([]string(nil), order...)
	sort.SliceStable(ord, func(i, j int) bool {
		ci, cj := counts[ord[i]], counts[ord[j]]
		if ci != cj {
			return ci > cj
		}
		return idx[ord[i]] < idx[ord[j]]
	})
	if limit >= 0 && len(ord) > limit {
		ord = ord[:limit]
	}
	out := make([]iaudCount, 0, len(ord))
	for _, k := range ord {
		out = append(out, iaudCount{k, counts[k]})
	}
	return out
}

// ── docs 取源根（py docs_out_root）────────────────────────────────────────

func iaudDocsOutRoot(root string) (string, string) {
	r := iaudPyAbs(root)
	alt := os.Getenv("ZERG_DOCS_ALT")
	if alt == "" {
		alt = iaudPyJoin(iaudPyDirname(r), "Zerg-内部文档")
	}
	cands := []string{iaudPyJoin(r, "docs", "项目文档"), iaudPyJoin(alt, "项目文档")}
	for _, p := range cands {
		if iaudIsDir(p) {
			return p, ""
		}
	}
	return "", fmt.Sprintf("项目文档取源根未找到（仓内根 %s ✗ · 仓外根 %s ✗）", cands[0], cands[1])
}

// ── 入口编排（py main()）────────────────────────────────────────────────

func iaudWriteTSV(path string, rows []iaudRow) error {
	var b strings.Builder
	b.WriteString("文件\t行\t分类\t字面量\t该行代码\n")
	for _, r := range rows {
		fields := []string{r.rel, strconv.Itoa(r.lineNo), r.cls, r.lit, r.code}
		for i := range fields {
			fields[i] = strings.ReplaceAll(fields[i], "\t", " ")
		}
		b.WriteString(strings.Join(fields, "\t"))
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func iaudMain(stdout, stderr io.Writer, argv []string) int {
	cwd, _ := os.Getwd()
	return iaudMainWithRoot(stdout, stderr, iaudRepoRoot(cwd), argv)
}

// iaudMainWithRoot —— py main()，根**显式传入**（供单测/对拍；✗ 不依赖 cwd）。
func iaudMainWithRoot(stdout, stderr io.Writer, root string, argv []string) int {
	mode := "all"
	if len(argv) > 0 {
		mode = argv[0]
	}
	// ★ 8 的静默吞（本单）：补回 py 的**严格解析面** —— 位置参（模式）可任意（`bogus` 仍只出
	// 头两行 · rc=0，既有测试件 `TestI18nAuditUnknownModeHeaderOnly` 钉着），但**旗标一律要登记**：
	// py 侧 argparse 对未登记选项报「unrecognized arguments」退 2，本 Go 真搬此前**丢了这一档**
	// ⇒ `zerg ui i18n-audit --nosuch` 被读成「模式 = --nosuch」⇒ 两个 if 都不进 ⇒ **静默 rc=0**。
	// 现补：argv 出现旗标形态、或多于一个位置参 ⇒ 用法错退 2（✗ 不改正常输出/语义）。
	for i, a := range argv {
		if strings.HasPrefix(a, "-") && a != "-" {
			fmt.Fprintf(stderr, "i18n_audit: 未登记旗标 %q（本命令只收位置参 classify|keys|all）⇒ 退码 2\n", a)
			return 2
		}
		if i >= 1 {
			fmt.Fprintf(stderr, "i18n_audit: 多余参数 %q（本命令只收一个位置参 classify|keys|all）⇒ 退码 2\n", a)
			return 2
		}
	}

	outdir := strings.TrimSpace(os.Getenv("ZERG_I18N_AUDIT_OUT"))
	if outdir == "" {
		base, missing := iaudDocsOutRoot(root)
		if base == "" {
			msg := missing
			if msg == "" {
				msg = "项目文档取源根未找到"
			}
			fmt.Fprintf(stdout, "i18n_audit: %s\n", msg)
			fmt.Fprintf(stdout, "⇒ 输出目录不可判（不给结论）· rc=2\n")
			return 2
		}
		outdir = iaudPyJoin(base, "v2.5.9", "i18n-audit")
	}
	if err := os.MkdirAll(outdir, 0o755); err != nil {
		fmt.Fprintf(stderr, "i18n_audit: 创建输出目录失败: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "repo = %s\n", root)
	fmt.Fprintf(stdout, "outdir = %s\n", outdir)

	if mode == "classify" || mode == "all" {
		rows, err := iaudClassify(root)
		if err != nil {
			fmt.Fprintf(stderr, "i18n_audit: %v\n", err)
			return 2
		}
		p := iaudPyJoin(outdir, "L2-现状-中文字面量-审计口径.tsv")
		if err := iaudWriteTSV(p, rows); err != nil {
			fmt.Fprintf(stderr, "i18n_audit: %v\n", err)
			return 2
		}
		orderCls := []string{}
		cntCls := map[string]int{}
		for _, r := range rows {
			if _, ok := cntCls[r.cls]; !ok {
				orderCls = append(orderCls, r.cls)
			}
			cntCls[r.cls]++
		}
		uiLike := map[string]bool{"UI文案": true, "UI文案(待确认)": true}
		orderUI := []string{}
		cntUI := map[string]int{}
		for _, r := range rows {
			if uiLike[r.cls] {
				if _, ok := cntUI[r.rel]; !ok {
					orderUI = append(orderUI, r.rel)
				}
				cntUI[r.rel]++
			}
		}
		fmt.Fprintf(stdout, "\n=== P0-2 分拣结果（%d 条含 CJK 行）===\n", len(rows))
		for _, c := range iaudMostCommon(orderCls, cntCls, -1) {
			fmt.Fprintf(stdout, "   %-12s %5d\n", c.key, c.n)
		}
		fmt.Fprintf(stdout, "   --- UI 文案候选 top 文件（含待确认）---\n")
		for _, c := range iaudMostCommon(orderUI, cntUI, 10) {
			fmt.Fprintf(stdout, "   %-45s %4d\n", c.key, c.n)
		}
		fmt.Fprintf(stdout, "   写出: %s\n", iaudPyRelpath(p, root))
	}

	if mode == "keys" || mode == "all" {
		defs, usedOrder, usedSites, dyn, err := iaudKeysAudit(root)
		if err != nil {
			fmt.Fprintf(stderr, "i18n_audit: %v\n", err)
			return 2
		}
		zh, en := defs["zh-CN"], defs["en"]
		usedSet := map[string]bool{}
		for k := range usedSites {
			usedSet[k] = true
		}
		onlyZh := iaudDiffSorted(zh, en)
		onlyEn := iaudDiffSorted(en, zh)
		refUndef := iaudDiffSorted(usedSet, zh)
		defUnref := iaudDiffSorted(zh, usedSet)

		p := iaudPyJoin(outdir, "P0-3-键审计.md")
		lines := []string{
			"# P0-3 i18n 键健康度审计", "",
			"| 检查 | 结果 |", "|---|---|",
			fmt.Sprintf("| zh-CN 键数 | %d |", len(zh)),
			fmt.Sprintf("| en 键数 | %d |", len(en)),
			fmt.Sprintf("| 仅 zh 有 | %s |", iaudOrNoneList(onlyZh)),
			fmt.Sprintf("| 仅 en 有 | %s |", iaudOrNoneList(onlyEn)),
			fmt.Sprintf("| 代码引用但未定义 | %s |", iaudOrNoneList(refUndef)),
			fmt.Sprintf("| 定义了但未被引用 | %s |", iaudOrNoneList(defUnref)), "",
			"## 复用点（疑似一键多义；同一键跨文件/跨语义使用）", "",
			"| 键 | 引用次数 | 涉及文件数 | 引用点 |", "|---|---|---|---|",
		}
		multi := 0
		ord := append([]string(nil), usedOrder...)
		sort.SliceStable(ord, func(i, j int) bool {
			return len(usedSites[ord[i]]) > len(usedSites[ord[j]])
		})
		for _, k := range ord {
			sites := usedSites[k]
			if len(sites) <= 1 {
				continue
			}
			multi++
			fset := map[string]bool{}
			for _, s := range sites {
				fset[s.file] = true
			}
			lim := sites
			if len(lim) > 6 {
				lim = lim[:6]
			}
			parts := make([]string, 0, len(lim))
			for _, s := range lim {
				parts = append(parts, fmt.Sprintf("%s:%d", s.file, s.line))
			}
			lines = append(lines, fmt.Sprintf("| `%s` | %d | %d | %s |", k, len(sites), len(fset), strings.Join(parts, "<br>")))
		}
		lines = append(lines, "", "## 动态键调用（静态分析不可见）", "",
			fmt.Sprintf("`t!(<表达式>)` 形式的调用 %d 处——这些键无法被静态扫描确认，因此上方「定义了但未被引用」清单**可能包含被动态引用的键**：", len(dyn)), "",
			"| 位置 | 代码 |", "|---|---|")
		for _, s := range dyn {
			ctx := strings.ReplaceAll(s.ctx, "|", "\\|")
			lines = append(lines, fmt.Sprintf("| `%s:%d` | `%s` |", s.file, s.line, ctx))
		}
		lines = append(lines, "", "> 判读：同一键出现在**语义不同**的位置即为一键多义（GPUI 官方明令禁止）。同一文件内相邻行的重复调用通常正常（列表逐行渲染）。", "")
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			fmt.Fprintf(stderr, "i18n_audit: %v\n", err)
			return 2
		}

		fmt.Fprintf(stdout, "\n=== P0-3 键审计 ===\n")
		fmt.Fprintf(stdout, "   zh=%d en=%d 仅zh=%s 仅en=%s\n", len(zh), len(en), iaudOrNoneList(onlyZh), iaudOrNoneList(onlyEn))
		fmt.Fprintf(stdout, "   引用未定义: %s\n", iaudOrNoneList(refUndef))
		fmt.Fprintf(stdout, "   定义未引用(%d,注:动态键调用 %d 处不可见): %s\n", len(defUnref), len(dyn), iaudPyReprStrList(defUnref))
		dynPts := make([]string, 0, len(dyn))
		for _, s := range dyn {
			dynPts = append(dynPts, fmt.Sprintf("%s:%d", s.file, s.line))
		}
		fmt.Fprintf(stdout, "   动态键调用点: %s\n", iaudPyReprStrList(dynPts))
		fmt.Fprintf(stdout, "   多次引用的键: %d 个（详见报告）\n", multi)
		fmt.Fprintf(stdout, "   写出: %s\n", iaudPyRelpath(p, root))
	}
	return 0
}

// i18nAuditGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func i18nAuditGateMain(stdout, stderr io.Writer, argv []string) int {
	return iaudMain(stdout, stderr, argv)
}
