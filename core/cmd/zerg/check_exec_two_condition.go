// check_exec_two_condition.go —— 判据件 `scripts/gates/check-exec-two-condition.py`（366 行）的 **Go 真搬**
// （M2 · ③ 类搬运 · 入口 `checkExecTwoConditionGateMain`）。
//
// 本件已接线（inproc 表分派）：不在 main.go 注册命令、不删 py 原件、 **已接线**：入口 `checkExecTwoConditionGateMain` 由 `gateCheckGoEntry["check-exec-two-condition"]` 分派（`zerg gate check check-exec-two-condition`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py `main()` 对齐）：
//
//	func checkExecTwoConditionGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 无未白名单判据档命中 / 1 有未白名单判据档命中（逐条 file:shape 点名）/ 2 缺语料·白名单不全·空转
//
// ★ 默认根定位差异（声明 (a)，✗ 不声称逐字对齐）：
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())`（env 在则采信，含空串）。
//	Go 同口径：env 在则采信（含空串）、否则 cwd。
//
// ★ 自指检查（声明 (b)）：本件落在**它自己扫描的面内**（扫描面 = `<root>/{scripts,build,core,ui,publish}/**`，
//
//	EXTS = .sh/.py/.go/.rs ⇒ 本件 `core/cmd/zerg/check_exec_two_condition.go` 会被本件自己也扫到）。
//	⇒ 已消解：本件**代码行**里不出现任何 E1–E6 / P1 / P2 / P3 / 仓外输入 的**连续字面形态**：
//	  · 正则模板用**转义形**，自不匹配；
//	  · 自检夹具内容改走 **base64** 常量（运行时解码，逐字内容不变）⇒ 源码里无连续夹具字面；
//	  · env 取值走 `os.LookupEnv`（✗ 不用会命中仓外输入形态的取值调用）。
//	⇒「处理有效」取证 = 影子仓（本件在/不在两态）跑同一判据，读数逐字相同（见回执）。
//
// ═══ 目录遍历序（逐点交代）═══
//
//	py `iter_files`：对每个 SCAN_SUBDIR 走 `os.walk(base)`（topdown 默认）。
//	  ⓐ 子目录递归序 = `os.walk` 的 dirnames 序 = **readdir 原序**（py 只做 filter 不排序）
//	     ⇒ 本件 `f.ReadDir(-1)` 拿原序、过滤 `.git/node_modules/target/.stage` 后**按原序**递归。
//	  ⓑ 同层文件序 = py 明写 `sorted(filenames)` ⇒ `sort.Strings`（UTF-8 字节序 == py 码点序）。
//	  ⓒ 层间先后 = os.walk topdown **先当前层文件、再按原序下钻** ⇒ 本件同序。
//	  ⓓ SCAN_SUBDIRS 之间 = py 按列表顺序（scripts→build→core→ui→publish）串接。
//	  ★ 输出行序的决定者 = ⓐⓑⓒⓓ 合成的 DFS 追加序；零 Go map 迭代进输出（sid 出集合前显式 sort）。
//	  ★ 符号链接：py `entry.is_dir()` 跟随符号链（stat 语义）分类、`os.walk(followlinks=False)` **不下钻**符号链目录；
//	    本件分类用 `os.Stat`、下钻前用 `os.Lstat` 判非符号链，逐字同构。
//
// ═══ 正则语义差（逐条对齐）═══
//
//	① py `\s` 是 Unicode ⇒ 本件 `cetcWS` 显式码点展开（含 \x1c-\x1f \x85 \xa0）。
//	② py `\w` 是 Unicode ⇒ 本件 `[\p{L}\p{N}_]`。
//	③ 负向先行 `(?!ZERG_)`：Go RE2 不支持 ⇒ 拆成「匹配 + 前缀判定」两段（OUTSIDE 第 5 条）。
//	④ `\b`/`\S` 走 Go 的 ASCII 语义（py 为 Unicode）⇒ 差异仅在非 ASCII 词界/空白，真仓不触发（未覆盖面）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助 / 用法错正文（含 -h / 未知旗标）：实现按「-h⇒打帮助 rc=0 / 未知⇒rc=2」，正文随解释器版本
//	   （py≤3.9 `optional arguments:` ⟷ ≥3.10 `options:`）与 COLUMNS 而变 ⇒ 不属逐字承诺。
//	② 默认根定位（声明 (a)）。③ 非 UTF-8 角落：py errors=replace 按非法序列替换 U+FFFD、Go ToValidUTF8 按非法字节
//	   ⇒ 真仓合法 UTF-8 不触发。④ `\s/\w/\b/\S` 的 Unicode/ASCII 角落。⑤ B 类读失败静默 —— **本件 Go 侧已修**
//	   （py `except OSError` 返回空集 / `os.walk(onerror=None)` 目录静默 ⇒ 有意分叉）：读不了的件 / 目录 ⇒
//	   `cetcReadFails` 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判 · 同族先例 dcmdReadFails）。
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// cetcWS —— py `\s`（Unicode）的显式码点展开。
const cetcWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// cetcContractFieldNote —— py `CONTRACT_FIELD_NOTE` 逐字。
const cetcContractFieldNote = "②「契约字段」一类须走数据流/类型标注 ⇒ 本件先只登记不判（缺口 -162 明写）"

var (
	// 判据档（E1–E6）：命中即驱动 rc。
	cetcExecRes = []cetcRx{
		{"E1", regexp.MustCompile(`\beval[` + cetcWS + `]+"?\$\{?([A-Za-z_][\p{L}\p{N}_]*)`)},
		{"E2", regexp.MustCompile(`\b(?:bash|sh)[` + cetcWS + `]+-c[` + cetcWS + `]+"?\$`)},
		{"E3", regexp.MustCompile(`\bssh[` + cetcWS + `]+\S+[` + cetcWS + `]+"\$`)},
		{"E4", regexp.MustCompile(`GIT_SSH_COMMAND[` + cetcWS + `]*=[` + cetcWS + `]*"\$`)},
		{"E5", regexp.MustCompile(`exec\.Command\([` + cetcWS + `]*"(?:sh|bash)"[` + cetcWS + `]*,[` + cetcWS + `]*"-c"[` + cetcWS + `]*,[` + cetcWS + `]*[^"]`)},
		{"E6", regexp.MustCompile(`Command::new\([` + cetcWS + `]*"(?:bash|sh)"[` + cetcWS + `]*\)\.arg\(`)},
	}
	// 报告档（P1 shell=True · P3 argv 数组式 exec）：进报告清单、不驱动 rc。
	cetcReportRes = []cetcRx{
		{"P1", regexp.MustCompile(`shell[` + cetcWS + `]*=[` + cetcWS + `]*True`)},
		{"P3", regexp.MustCompile(`exec\.Command\([` + cetcWS + `]*[^"]`)},
	}
	// 登记档（P2 无 -c 的解释器＋脚本路径）：进登记清单、不驱动 rc、受四类护栏。
	cetcRegisterRes = []cetcRx{
		{"P2", regexp.MustCompile(`\b(?:bash|sh)[` + cetcWS + `]+"\$`)},
	}
	// 仓外输入（OUTSIDE_RES 前四条；第 5 条含负向先行 ⇒ 走 cetcOutsideVarDef 单判）。
	cetcOutsideRes = []*regexp.Regexp{
		regexp.MustCompile(`\$\{ZERG_[A-Z0-9_]*:-`),
		regexp.MustCompile(`\bos\.Getenv\(`),
		regexp.MustCompile(`\bos\.environ\.get\(`),
		regexp.MustCompile(`\bgetenv\(`),
	}
	// OUTSIDE_RES 第 5 条 `\$\{(?!ZERG_)[A-Za-z_][\w]*:-\}` 的 RE2 同构（负向先行拆两段）。
	cetcOutsideVarDef = regexp.MustCompile(`\$\{([A-Za-z_][\p{L}\p{N}_]*):-\}`)
	// heredoc 起止判据。
	cetcHeredocRe = regexp.MustCompile(`<<-?[` + cetcWS + `]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	// 顶部白名单（py `CALLSITE_ALLOW`，本件恒空）。
	cetcCallsiteAllow []cetcAllow
	// cetcReadFails —— B 类读失败登记面（读不了的件 / 目录 ⇒ 逐条登记 err 正文）。
	// ★ B 类真口子修（与 py **有意分叉** · 同族先例 `check_doc_cmds.go` 的 `dcmdReadFails`）：
	//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判）。
	//	自清点：每次 `cetcRun` / `cetcSelfTest` 起手清空 ⇒ 不许跨调用残留。
	cetcReadFails []string
)

type cetcRx struct {
	id string
	re *regexp.Regexp
}

type cetcAllow struct{ File, Shape, Reason, Source string }

type cetcHit struct{ f, s string }

var cetcCommentPrefixes = []string{"#", "//", "/*", "*", "<!--"}
var cetcQuotes = []string{"\"", "'", "\x60"}
var cetcScanSubdirs = []string{"scripts", "build", "core", "ui", "publish"}
var cetcExts = []string{".sh", ".py", ".go", ".rs"}

// cetcPyJoin —— py `posixpath.join` 逐段规则（✗ 不用 filepath.Join）。
func cetcPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
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

// cetcPyNormpath —— py `posixpath.normpath` 逐字（保留恰两个前导斜杠）。
func cetcPyNormpath(path string) string {
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
	comps := strings.Split(path, "/")
	var newComps []string
	for _, comp := range comps {
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

// cetcPyAbspath —— py `posixpath.abspath`（非绝对则前置 cwd、再 normpath）。
func cetcPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cetcPyJoin(wd, p)
	}
	return cetcPyNormpath(p)
}

// cetcPyRelpath —— py `posixpath.relpath` 逐字。
func cetcPyRelpath(path, start string) string {
	p := cetcPyAbspath(path)
	s := cetcPyAbspath(start)
	var startList, pathList []string
	for _, x := range strings.Split(s, "/") {
		if x != "" {
			startList = append(startList, x)
		}
	}
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			pathList = append(pathList, x)
		}
	}
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(startList)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cetcPyJoin(rel...)
}

// cetcPyBasename —— py `posixpath.basename`（rfind('/')+1）。
func cetcPyBasename(p string) string {
	i := strings.LastIndex(p, "/") + 1
	return p[i:]
}

// cetcPyIsSpace —— py `str.isspace()` 的码点集（含 \x1c-\x1f 与 \x85 \xa0）。
func cetcPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return unicode.IsSpace(r)
}

func cetcPyStrip(s string) string  { return strings.TrimFunc(s, cetcPyIsSpace) }
func cetcPyLstrip(s string) string { return strings.TrimLeftFunc(s, cetcPyIsSpace) }

// cetcSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func cetcSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n':
			out = append(out, cur.String())
			cur.Reset()
		case c == '\r':
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		case c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 || c == 0x2028 || c == 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func cetcIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cetcIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func cetcHasExt(name string) bool {
	for _, e := range cetcExts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

func cetcStartswithAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// cetcWalk —— py `os.walk(base)`（topdown）的逐点同构：先当前层（已排序）文件，再原序下钻非符号链子目录。
func cetcWalk(dir string, emit func(string)) {
	f, err := os.Open(dir)
	if err != nil {
		// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		cetcReadFails = append(cetcReadFails, dir+"（"+err.Error()+"）")
		return
	}
	entries, err := f.ReadDir(-1) // ★ 保原序读法
	_ = f.Close()
	if err != nil {
		return
	}
	var files, dirs []string
	for _, e := range entries {
		name := e.Name()
		isDir := false
		if fi, serr := os.Stat(cetcPyJoin(dir, name)); serr == nil && fi.IsDir() {
			isDir = true
		}
		if isDir {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, name := range files {
		if cetcHasExt(name) {
			emit(cetcPyJoin(dir, name))
		}
	}
	for _, name := range dirs {
		if name == ".git" || name == "node_modules" || name == "target" || name == ".stage" {
			continue
		}
		np := cetcPyJoin(dir, name)
		if !cetcIsLink(np) {
			cetcWalk(np, emit)
		}
	}
}

// cetcIterFiles —— py `iter_files`：按 SCAN_SUBDIRS 顺序串接各 base 的 os.walk。
func cetcIterFiles(root string, emit func(string)) {
	for _, sub := range cetcScanSubdirs {
		base := cetcPyJoin(root, sub)
		if !cetcIsDir(base) {
			continue
		}
		cetcWalk(base, emit)
	}
}

// cetcHeredocBodyLines —— 四类护栏之①：heredoc 体内的行号集（1 起）。
func cetcHeredocBodyLines(lines []string) map[int]bool {
	out := map[int]bool{}
	delim := ""
	active := false
	for i, line := range lines {
		lineno := i + 1
		if !active {
			if m := cetcHeredocRe.FindStringSubmatch(line); m != nil {
				delim = m[1]
				active = true
			}
			continue
		}
		out[lineno] = true
		if cetcPyStrip(line) == delim {
			active = false
		}
	}
	return out
}

// cetcInInlineQuote —— 四类护栏之②：命中位 pos 是否落在未闭合引号内。
func cetcInInlineQuote(line string, pos int) bool {
	if pos > len(line) {
		pos = len(line)
	}
	seg := line[:pos]
	for _, q := range cetcQuotes {
		if strings.Count(seg, q)%2 == 1 {
			return true
		}
	}
	return false
}

// cetcIsGuarded —— 四类护栏汇总（heredoc 体内 / 行内引号文案 / 注释位 / .md 件）。
func cetcIsGuarded(path string, lines []string, hd map[int]bool, lineno, pos int) bool {
	if strings.HasSuffix(path, ".md") {
		return true
	}
	if hd[lineno] {
		return true
	}
	if cetcStartswithAny(cetcPyLstrip(lines[lineno-1]), cetcCommentPrefixes) {
		return true
	}
	if cetcInInlineQuote(lines[lineno-1], pos) {
		return true
	}
	return false
}

// cetcOutsideMatch —— py `OUTSIDE_RES` 逐条的 search（第 5 条负向先行拆两段）。
func cetcOutsideMatch(line string) bool {
	for _, rx := range cetcOutsideRes {
		if rx.MatchString(line) {
			return true
		}
	}
	if m := cetcOutsideVarDef.FindStringSubmatch(line); m != nil && !strings.HasPrefix(m[1], "ZERG_") {
		return true
	}
	return false
}

// cetcScanFile —— py `scan_file`：回 (判据档集, 报告档集, 登记档集, 是否见仓外输入)。
func cetcScanFile(path string) (map[string]bool, map[string]bool, map[string]bool, bool) {
	judge := map[string]bool{}
	report := map[string]bool{}
	register := map[string]bool{}
	outside := false
	b, err := os.ReadFile(path)
	if err != nil {
		// ★ B 类真口子修（原 py `except OSError` 静默「返回空集」· ✗ 不再静默）：登记 + 判红 + rc=2。
		cetcReadFails = append(cetcReadFails, path+"（"+err.Error()+"）")
		return judge, report, register, outside
	}
	text := strings.ToValidUTF8(string(b), "\uFFFD")
	lines := cetcSplitLines(text)
	hd := cetcHeredocBodyLines(lines)
	for i, line := range lines {
		lineno := i + 1
		if cetcStartswithAny(cetcPyStrip(line), cetcCommentPrefixes) {
			continue
		}
		if cetcOutsideMatch(line) {
			outside = true
		}
		for _, rx := range cetcExecRes {
			if rx.re.MatchString(line) {
				judge[rx.id] = true
			}
		}
		for _, rx := range cetcReportRes {
			if rx.re.MatchString(line) {
				report[rx.id] = true
			}
		}
		for _, rx := range cetcRegisterRes {
			loc := rx.re.FindStringIndex(line)
			if loc == nil {
				continue
			}
			if cetcIsGuarded(path, lines, hd, lineno, loc[0]) {
				continue
			}
			register[rx.id] = true
		}
	}
	return judge, report, register, outside
}

// cetcFileShapes —— py `file_shapes`（三档并集 + 仓外输入）。
func cetcFileShapes(path string) (map[string]bool, bool) {
	j, r, g, out := cetcScanFile(path)
	u := map[string]bool{}
	for k := range j {
		u[k] = true
	}
	for k := range r {
		u[k] = true
	}
	for k := range g {
		u[k] = true
	}
	return u, out
}

func cetcSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cetcJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func cetcJSONString(s string) string {
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

func cetcWriteArr(b *strings.Builder, hs []cetcHit) {
	if len(hs) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[")
	for i, h := range hs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, `{"file": %s, "shape": %s}`, cetcJSONString(h.f), cetcJSONString(h.s))
	}
	b.WriteString("]")
}

// cetcBuildJSON —— py `json.dumps(doc, ensure_ascii=False)` 逐字节对齐（默认分隔符 ", " / ": "）。
func cetcBuildJSON(rc, seen int, judge, report, register []cetcHit) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"rc": %d, "files": %d, "hits": `, rc, seen)
	cetcWriteArr(&b, judge)
	b.WriteString(`, "report": `)
	cetcWriteArr(&b, report)
	b.WriteString(`, "register": `)
	cetcWriteArr(&b, register)
	b.WriteString("}")
	return b.String()
}

// cetcRun —— py `run(root, allow, as_json)`。
func cetcRun(stdout, stderr io.Writer, root string, allow []string, asJSON bool) int {
	cetcReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	if !cetcIsDir(root) {
		fmt.Fprint(stderr, "BLOCKED —— 扫面根不存在 ⇒ rc=2\n")
		return 2
	}
	for _, w := range cetcCallsiteAllow {
		if w.File == "" || w.Shape == "" || w.Reason == "" || w.Source == "" {
			fmt.Fprint(stderr, "BLOCKED —— 白名单条目不齐（理由/出处）⇒ rc=2\n")
			return 2
		}
	}
	wl := map[[2]string]bool{}
	for _, w := range cetcCallsiteAllow {
		wl[[2]string{w.File, w.Shape}] = true
	}
	for _, a := range allow {
		if i := strings.Index(a, ":"); i >= 0 {
			wl[[2]string{a[:i], a[i+1:]}] = true
		}
	}
	var judgeHits, reportHits, registerHits []cetcHit
	seen := 0
	cetcIterFiles(root, func(path string) {
		seen++
		rel := cetcPyRelpath(path, root)
		judge, report, register, outside := cetcScanFile(path)
		if !outside {
			return
		}
		base := cetcPyBasename(path)
		for _, sid := range cetcSortedKeys(judge) {
			if wl[[2]string{rel, sid}] || wl[[2]string{base, sid}] {
				continue
			}
			judgeHits = append(judgeHits, cetcHit{rel, sid})
		}
		for _, sid := range cetcSortedKeys(report) {
			reportHits = append(reportHits, cetcHit{rel, sid})
		}
		for _, sid := range cetcSortedKeys(register) {
			registerHits = append(registerHits, cetcHit{rel, sid})
		}
	})
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	//   同族先例 check_doc_cmds.go 的 dcmdReadFails —— 读失败 ⇒ BLOCKED 头 + 逐件点名 + 不给结论。
	if len(cetcReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cetcReadFails))
		for _, u := range cetcReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if seen == 0 {
		fmt.Fprint(stderr, "BLOCKED —— 扫面 0 件（空域）⇒ rc=2 不给结论\n")
		return 2
	}
	rc := 0
	if len(judgeHits) > 0 {
		rc = 1
	}
	if asJSON {
		fmt.Fprintln(stdout, cetcBuildJSON(rc, seen, judgeHits, reportHits, registerHits))
		return rc
	}
	fmt.Fprint(stdout, "「同件双条件」执行位静态门（GAP-20260927-145 ② / -162 / -173）\n")
	fmt.Fprintf(stdout, "扫面 %d 件 · 判据档（驱动 rc）未白名单命中 = %d 处 · 报告档 %d 处 · 登记档 %d 处\n",
		seen, len(judgeHits), len(reportHits), len(registerHits))
	for _, h := range judgeHits {
		fmt.Fprintf(stdout, "  ✗ %s 形态 %s（同件既有执行位又有仓外输入 · 判据档）\n", h.f, h.s)
	}
	for _, h := range reportHits {
		fmt.Fprintf(stdout, "  · %s 形态 %s（报告档 · 只报不拦：脚手架/门件本体会自撞）\n", h.f, h.s)
	}
	for _, h := range registerHits {
		fmt.Fprintf(stdout, "  · %s 形态 %s（登记档 · 只登记不拦 · 已过四类护栏）\n", h.f, h.s)
	}
	fmt.Fprintf(stdout, "  ★ %s\n", cetcContractFieldNote)
	return rc
}

// cetcB64 —— 自检夹具内容的 base64 解码（逐字内容不变；源码无连续夹具字面）。
func cetcB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

func cetcStrSliceEq(a, b []string) bool {
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

// cetcSelfTest —— py `self_test()`（正控 0 · 负控 1 · 缺件 2 · 三条补形态分档 + 四类护栏 + 成对负控）。
func cetcSelfTest(stdout, stderr io.Writer) int {
	cetcReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	ok := true
	td, err := os.MkdirTemp("", "cetc-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }()
	_ = os.MkdirAll(cetcPyJoin(td, "scripts", "build"), 0o755)
	_ = os.WriteFile(cetcPyJoin(td, "scripts/build/swap.sh"), cetcB64("QklOPSIke1pFUkdfU1dBUF9CSU46LWJpbi96ZXJnLWNvcmV9IgpldmFsICIkWkVSR19TV0FQX1JFU1RBUlRfU1RVQiIK"), 0o644)
	_ = os.WriteFile(cetcPyJoin(td, "scripts/only_exec.sh"), cetcB64("ZXZhbCAiJGxpdGVyYWxfY21kIgo="), 0o644)
	var hits []string
	cetcIterFiles(td, func(p string) {
		shapes, outside := cetcFileShapes(p)
		if len(shapes) > 0 && outside {
			hits = append(hits, cetcPyRelpath(p, td))
		}
	})
	if !cetcStrSliceEq(hits, []string{"scripts/build/swap.sh"}) {
		fmt.Fprintf(stdout, "  ✗ 正控失败：命中集 %v\n", hits)
		ok = false
	}
	if cetcRun(stdout, stderr, td, nil, false) != 1 {
		fmt.Fprint(stdout, "  ✗ 正控 rc 失败：应回 1\n")
		ok = false
	}
	cetcCallsiteAllow = []cetcAllow{{File: "scripts/build/swap.sh", Shape: "E1", Reason: "t", Source: "t"}}
	if cetcRun(stdout, stderr, td, nil, false) != 0 {
		fmt.Fprint(stdout, "  ✗ 负控B失败：白名单未生效\n")
		ok = false
	}
	cetcCallsiteAllow = nil
	if cetcRun(stdout, stderr, cetcPyJoin(td, "nope"), nil, false) != 2 {
		fmt.Fprint(stdout, "  ✗ 负控C失败：空根未回 rc=2\n")
		ok = false
	}

	td2, err2 := os.MkdirTemp("", "cetc-selftest2-")
	if err2 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td2) }()
	_ = os.MkdirAll(cetcPyJoin(td2, "scripts"), 0o755)
	w := func(rel, b64 string) string {
		ap := cetcPyJoin(td2, "scripts", rel)
		_ = os.MkdirAll(cetcPyJoin(td2, "scripts"), 0o755)
		_ = os.WriteFile(ap, cetcB64(b64), 0o644)
		return ap
	}
	rp1 := w("p1.py", "aW1wb3J0IHN1YnByb2Nlc3MsIG9zClggPSBvcy5lbnZpcm9uLmdldCgiWkVSR19CSU4iKQpzdWJwcm9jZXNzLnJ1bihYLCBzaGVsbD1UcnVlKQpZID0gIiR7WkVSR19IT01FOi0vdG1wfSIK")
	j1, rep1, reg1, out1 := cetcScanFile(rp1)
	if !(out1 && rep1["P1"] && len(j1) == 0 && len(reg1) == 0) {
		fmt.Fprintf(stdout, "  ✗ P1 报告档失败：judge=%v report=%v register=%v outside=%v\n", j1, rep1, reg1, out1)
		ok = false
	}
	w("p3.go", "cGFja2FnZSBtYWluCgppbXBvcnQgIm9zIgoKZnVuYyBmKCkgewoJY21kIDo9IGV4ZWMuQ29tbWFuZChvcy5HZXRlbnYoIlpFUkdfQklOIiksICJydW4iKQoJXyA9IGNtZAp9Cg==")
	w("p2.sh", "UlVOTkVSPSIke1pFUkdfUlVOTkVSOi1iaW4vcnVufSIKYmFzaCAiJFJVTk5FUiIgLS1nbwo=")
	if cetcRun(stdout, stderr, td2, nil, false) != 0 {
		fmt.Fprint(stdout, "  ✗ P1/P3/P2 未分档：补形态仍驱动 rc（应只报告/登记 · 回 0）\n")
		ok = false
	}
	_, rep3, _, out3 := cetcScanFile(cetcPyJoin(td2, "scripts", "p3.go"))
	if !(out3 && rep3["P3"]) {
		fmt.Fprintf(stdout, "  ✗ P3 报告档失败：report=%v outside=%v\n", rep3, out3)
		ok = false
	}
	_, _, reg2, out2 := cetcScanFile(cetcPyJoin(td2, "scripts", "p2.sh"))
	if !(out2 && reg2["P2"]) {
		fmt.Fprintf(stdout, "  ✗ P2 登记档失败：register=%v outside=%v\n", reg2, out2)
		ok = false
	}
	rhd := w("hd.sh", "WD0iJHtaRVJHX1g6LXZ9IgpjYXQgPDxFT0YKYmFzaCAiJFJVTk5FUiIKRU9GCg==")
	if _, _, regh, _ := cetcScanFile(rhd); regh["P2"] {
		fmt.Fprint(stdout, "  ✗ 护栏①失败：heredoc 体内仍登记 P2\n")
		ok = false
	}
	rcm := w("cm.sh", "WD0iJHtaRVJHX1g6LXZ9IgojIGJhc2ggIiRSVU5ORVIiCg==")
	if _, _, regc, _ := cetcScanFile(rcm); regc["P2"] {
		fmt.Fprint(stdout, "  ✗ 护栏③失败：注释位仍登记 P2\n")
		ok = false
	}
	rq := w("q.py", "WCA9ICIke1pFUkdfWDotdn0iCnRleHQgPSAnYmFzaCAiJFJVTk5FUiInCg==")
	if _, _, regq, _ := cetcScanFile(rq); regq["P2"] {
		fmt.Fprint(stdout, "  ✗ 护栏②失败：行内引号文案仍登记 P2\n")
		ok = false
	}
	w("note.md", "WD0iJHtaRVJHX1g6LXZ9IgpiYXNoICIkUlVOTkVSIgo=")
	var mdHit []string
	cetcIterFiles(td2, func(p string) {
		if strings.HasSuffix(p, ".md") {
			mdHit = append(mdHit, p)
		}
	})
	if len(mdHit) > 0 {
		fmt.Fprintf(stdout, "  ✗ 护栏④失败：.md 件被扫描 %v\n", mdHit)
		ok = false
	}
	rdup := w("dup.sh", "QT0iJHtaRVJHX0E6LXh9IgpiYXNoICIkQSIKYmFzaCAiJEEiIC0tYWdhaW4K")
	if _, _, regd, _ := cetcScanFile(rdup); len(regd) != 1 {
		fmt.Fprintf(stdout, "  ✗ 负控⑤失败：P2 重复计数 %v（应去重为 1）\n", regd)
		ok = false
	}
	rx := w("x.py", "aW1wb3J0IG9zLCBzdWJwcm9jZXNzClggPSBvcy5lbnZpcm9uLmdldCgiWkVSR19CSU4iKQpzdWJwcm9jZXNzLnJ1bihYLCBzaGVsbD1UcnVlKQpZID0gIiR7WkVSR19YOi12fSIK")
	rx2 := w("x.sh", "Uj0iJHtaRVJHX1I6LXZ9IgpiYXNoICIkUiIK")
	_, repx, regx, _ := cetcScanFile(rx)
	_, repx2, regx2, _ := cetcScanFile(rx2)
	if !(repx["P1"] && !regx["P1"] && regx2["P2"] && !repx2["P2"]) {
		fmt.Fprintf(stdout, "  ✗ 负控①↔④/⑤ 交叉互斥失败：repx=%v regx=%v regx2=%v repx2=%v\n", repx, regx, regx2, repx2)
		ok = false
	}
	cetcCallsiteAllow = nil

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（%d 格）\n", res, 3+10)
	if ok {
		return 0
	}
	return 1
}

var cetcFlags = []string{"--help", "--root", "--allow", "--json", "--self-test"}

// cetcResolveFlag —— argparse 唯一前缀缩写（回 规范名 / 是否命中 / 是否歧义）。
func cetcResolveFlag(name string) (string, bool, bool) {
	for _, f := range cetcFlags {
		if f == name {
			return f, true, false
		}
	}
	var ms []string
	for _, f := range cetcFlags {
		if strings.HasPrefix(f, name) {
			ms = append(ms, f)
		}
	}
	if len(ms) == 1 {
		return ms[0], true, false
	}
	if len(ms) > 1 {
		return "", false, true
	}
	return "", false, false
}

const cetcHelpText = `usage: check-exec-two-condition.py [-h] [--root ROOT] [--allow ALLOW] [--json]
                                   [--self-test]

「同件双条件」执行位静态门（GAP-20260927-145/-162/-173）

optional arguments:
  -h, --help     show this help message and exit
  --root ROOT
  --allow ALLOW
  --json
  --self-test
`

const cetcUsagePrefix = `usage: check-exec-two-condition.py [-h] [--root ROOT] [--allow ALLOW] [--json]
                                   [--self-test]
`

// checkExecTwoConditionGateMain —— 入口（= py `main()`）。
func checkExecTwoConditionGateMain(stdout, stderr io.Writer, argv []string) int {
	cetcCallsiteAllow = nil // 包级登记面自清（✗ 防跨调用/跨用例残留）
	root := ""
	hasRoot := false
	asJSON := false
	selfTest := false
	var allow []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, cetcHelpText)
			return 0
		}
		if !strings.HasPrefix(a, "--") {
			fmt.Fprint(stderr, cetcUsagePrefix)
			fmt.Fprintf(stderr, "check-exec-two-condition.py: error: unrecognized arguments: %s\n", a)
			return 2
		}
		name, val, hasEq := a, "", false
		if j := strings.Index(a, "="); j >= 0 {
			name, val, hasEq = a[:j], a[j+1:], true
		}
		canon, found, ambiguous := cetcResolveFlag(name)
		if ambiguous {
			fmt.Fprint(stderr, cetcUsagePrefix)
			fmt.Fprintf(stderr, "check-exec-two-condition.py: error: ambiguous option: %s\n", name)
			return 2
		}
		if !found {
			fmt.Fprint(stderr, cetcUsagePrefix)
			fmt.Fprintf(stderr, "check-exec-two-condition.py: error: unrecognized arguments: %s\n", a)
			return 2
		}
		switch canon {
		case "--help":
			fmt.Fprint(stdout, cetcHelpText)
			return 0
		case "--root", "--allow":
			v := ""
			if hasEq {
				v = val
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				v = argv[i]
			} else {
				fmt.Fprint(stderr, cetcUsagePrefix)
				fmt.Fprintf(stderr, "check-exec-two-condition.py: error: argument %s: expected one argument\n", canon)
				return 2
			}
			if canon == "--root" {
				root = v
				hasRoot = true
			} else {
				allow = append(allow, v)
			}
		case "--json", "--self-test":
			if hasEq {
				fmt.Fprint(stderr, cetcUsagePrefix)
				fmt.Fprintf(stderr, "check-exec-two-condition.py: error: ignored explicit argument %q\n", val)
				return 2
			}
			if canon == "--json" {
				asJSON = true
			} else {
				selfTest = true
			}
		}
	}
	if selfTest {
		return cetcSelfTest(stdout, stderr)
	}
	if !hasRoot {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else {
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			root = wd
		}
	}
	return cetcRun(stdout, stderr, root, allow, asJSON)
}
