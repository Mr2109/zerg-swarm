// check_wired_scripts.go —— 判据件 `scripts/gates/check-wired-scripts.py`（1158 行 · 甲案 C 类）
// 的 **Go 真搬**。判「脚本接线自检」：断言 A（门脚本必须出现在至少一个闸里 · 命令位调用形态）
// ＋ 断言 B（scripts/ 顶层零引用脚本必须登记白名单）。三档退码 0/1/2。
//
// 外部交互点（逐行现读 py · 全件只有这些）：
//
//	· `subprocess`（393 行 `git_out`）：只跑 `git ls-files -z` / `git rev-parse --short HEAD` /
//	  `git status --porcelain` / `git ls-files -o --exclude-standard -z -- scripts` 四条**只读**命令，
//	  不联网、不写盘。非 git 根（自检夹具）走文件系统遍历、零子进程。
//	· `sys.executable` re-exec（899 行）—— **仅自检内部**：py 自检把本件当子进程跑取 rc/stdout。
//	  ★ Go 侧**改为进程内调用**（同一 core 入口 · 同 rc/stdout 捕获）⇒ 这是**已声明的口径差**
//	  （自检手段差异），见未覆盖面。
//	· 遍历面：`os.walk`（非 git 根）· `os.listdir` 语义；**有序遍历点**见出厂声明 (b)。
//	· 计时/随机面：`datetime.datetime.now()`（709 行 · 输出「快照」时刻）· `tempfile.mkdtemp`
//	  （自检夹具目录）⇒ 两处**非确定**，对拍须掩码。
//
// 出厂声明 (a) **默认根定位差异**：py 的默认根 = `abspath(join(dirname(abspath(__file__)), "..", ".."))`
// （脚本上溯两级 = 仓根）；Go 入口无 `__file__` 语境 ⇒ 本件默认从**当前工作目录**上溯 ≤6 层、
// 找含 `scripts/gates/` 的目录，找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者读数逐字相同；
// 「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--root`。
//
// 出厂声明 (b) **自指检查 —— 判定为「真」**：本件落 `core/cmd/zerg/`，而本门判据的**引用面 =
// `git ls-files`（全仓已跟踪件）⇒ 本件一旦被跟踪就落进自己的扫描面**。处理：本件内**全部白名单件名
// 的「.」以 `\u002E` 落地**（见尾部数据块，程序化生成），源码正文不含这些件名的字面 ⇒ 对引用面的计数
// 增量为 0（除本件自身作为「引用面文件」+1，这是任何新件都不可避免的）。命中面（断言 A）与候选面
// （scripts/）与本件无关 ⇒ 判定读数不受影响。受控对照读数见回执。
//
// 有序遍历点（逐处交代 · ✗ 不许只写「碰不到」）：① 非 git 根的 `os.walk` 产出序 = scandir 原序、
// 先出当前层全部文件再按目录原序下钻（`cwsWalkDir` 逐字复刻 · 用 `File.ReadDir(-1)` 保原序，✗ 不用排
// 序读法）② `sorted(tops)`（508 行）＝输出行序的**决定者**：候选明细/断言 A 表/断言 B 分档全部按它
// ③ `sorted({...})`（781 行 · 排除域深一层）④ `sorted(samples, key=(-dw, s))`（652 行 · 样本序）
// ⑤ `git ls-files` 原序（git 索引序 · 只影响未排序的中间集合、不进输出行序）。⇒ 输出行序决定者 =
// `sorted(tops)`（候选）＋各档的**过滤保序**；引用面文件序**不进**输出行序（samples 会重排）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkWiredScriptsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 数据块类型（值由尾部程序化生成的「数据块」填充）─────────────────────────────
type cwsGateT struct{ path, kind, label, how string }
type cwsNotGateT struct{ path, why string }
type cwsWLEntry struct{ name, reason, date string }
type cwsSuggestT struct{ sug, why string }

type cwsCand struct{ name, path, kind, evidence string }

type cwsRow struct {
	name    string
	total   int
	code    int
	doc     int
	samples []string
	wl      bool
}

// ── 常量（逐字对齐 py 模块级常量）─────────────────────────────────────────────
var cwsExclSubdirs = []string{"exportnames", "sandbox-probes"}

var cwsSkipDirs = map[string]bool{
	".git": true, ".zerg": true, "node_modules": true, "__pycache__": true,
	"target": true, "dist": true, "vendor": true, "venv": true, ".venv": true,
	".idea": true, ".vscode": true,
}

var cwsSkipExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".pdf": true,
	".gguf": true, ".bin": true, ".db": true, ".zip": true, ".tar": true, ".gz": true,
	".so": true, ".dylib": true, ".a": true, ".o": true, ".woff": true, ".woff2": true,
	".ttf": true, ".mp4": true, ".webp": true, ".pt": true, ".safetensors": true,
	".lock": true, ".pyc": true,
}

const cwsMaxBytes = 3 * 1024 * 1024
const cwsDocMark = "门"
const cwsDocHeadLines = 60

var cwsDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ── 入口 ─────────────────────────────────────────────────────────────────────
func checkWiredScriptsGateMain(stdout, stderr io.Writer, argv []string) int {
	return cwsRunMain(stdout, stderr, argv)
}

type cwsOpts struct {
	strictReport bool
	wiredOnly    bool
	listRules    bool
	selfTest     bool
	noSelfTest   bool
	help         bool
	root         string
	whitelist    string
}

func cwsRunMain(stdout, stderr io.Writer, argv []string) int {
	opts, err := cwsParseArgs(argv)
	if err != nil {
		fmt.Fprintf(stderr, "usage: check-wired-scripts.py [-h] [--strict-report] [--list-rules]\n")
		fmt.Fprintf(stderr, "                              [--self-test] [--no-self-test] [--root ROOT]\n")
		fmt.Fprintf(stderr, "                              [--whitelist WHITELIST]\n")
		fmt.Fprintf(stderr, "check-wired-scripts.py: error: %s\n", err)
		return 2
	}
	if opts.help {
		fmt.Fprintf(stdout, "usage: check-wired-scripts.py [-h] [--strict-report] [--list-rules]\n")
		return 0
	}
	return cwsRun(stdout, opts)
}

// cwsParseArgs —— argparse 子集：长旗标 + `--k=v` 两态；✗ 不实现唯一前缀缩写（写未覆盖面）。
func cwsParseArgs(argv []string) (cwsOpts, error) {
	var o cwsOpts
	i := 0
	takeVal := func(name string) (string, error) {
		i++
		if i >= len(argv) {
			return "", fmt.Errorf("argument %s: expected one argument", name)
		}
		return argv[i], nil
	}
	for ; i < len(argv); i++ {
		a := argv[i]
		if a == "-h" || a == "--help" {
			o.help = true
			continue
		}
		key, val, hasEq := a, "", false
		if strings.HasPrefix(a, "--") {
			if k := strings.IndexByte(a, '='); k >= 0 {
				key, val, hasEq = a[:k], a[k+1:], true
			}
		}
		switch key {
		case "--strict-report":
			o.strictReport = true
		case "--wired-only":
			o.wiredOnly = true
		case "--list-rules":
			o.listRules = true
		case "--self-test":
			o.selfTest = true
		case "--no-self-test":
			o.noSelfTest = true
		case "--root":
			if hasEq {
				o.root = val
			} else {
				v, err := takeVal("--root")
				if err != nil {
					return o, err
				}
				o.root = v
			}
		case "--whitelist":
			if hasEq {
				o.whitelist = val
			} else {
				v, err := takeVal("--whitelist")
				if err != nil {
					return o, err
				}
				o.whitelist = v
			}
		default:
			return o, fmt.Errorf("unrecognized arguments: %s", a)
		}
	}
	return o, nil
}

// ── 主流程（py main 的主干 · 去掉 argparse）────────────────────────────────────
func cwsRun(stdout io.Writer, opts cwsOpts) int {
	if opts.listRules {
		fmt.Fprintln(stdout, cwsListRules())
		return 0
	}
	if opts.selfTest {
		return cwsSelftest(stdout)
	}
	root := opts.root
	if root == "" {
		root = cwsDefaultRoot()
	}
	root = cwsPyAbspath(cwsCwd(), root)
	if !cwsIsDir(cwsPyJoin(root, "scripts")) {
		fmt.Fprintf(stdout, "FATAL: 仓根缺 scripts/ 目录（root=%s）⇒ rc=2（不给结论）\n", root)
		return 2
	}
	if !opts.noSelfTest {
		rc := cwsSelftest(stdout)
		fmt.Fprintln(stdout, "")
		if rc != 0 {
			return 2
		}
	}

	files, mode := cwsListFiles(root)
	if len(files) == 0 {
		fmt.Fprintln(stdout, "FATAL: 引用面 0 个文件（空转）⇒ rc=2")
		return 2
	}
	candFiles := cwsCandidateFiles(root, files, mode)
	cands := cwsBuildCandidates(root, candFiles)
	if len(cands) == 0 {
		fmt.Fprintln(stdout, "FATAL: scripts/ 顶层脚本候选 0 只（空转 ⇒ 不给结论）⇒ rc=2")
		return 2
	}
	var aCands []cwsCand
	for _, c := range cands {
		if c.kind != "" {
			aCands = append(aCands, c)
		}
	}
	if len(aCands) == 0 {
		fmt.Fprintln(stdout, "FATAL: 门脚本候选 0 只（断言 A 无对象 ⇒ 空转，不给结论）⇒ rc=2")
		return 2
	}

	wl := cwsWhitelist
	if opts.whitelist != "" {
		b, err := os.ReadFile(opts.whitelist)
		var raw any
		if err == nil {
			err = json.Unmarshal(b, &raw)
		}
		if err != nil {
			fmt.Fprintf(stdout, "FATAL: 读 --whitelist 失败：%s ⇒ rc=2\n", err)
			return 2
		}
		arr, ok := raw.([]any)
		if !ok {
			fmt.Fprintln(stdout, "FATAL: --whitelist 必须是 JSON 数组 ⇒ rc=2")
			return 2
		}
		wl = nil
		for _, e := range arr {
			m, _ := e.(map[string]any)
			wl = append(wl, cwsWLEntry{cwsAnyStr(m["name"]), cwsAnyStr(m["reason"]), cwsAnyStr(m["date"])})
		}
	}

	hitsA, nWired, nGatePresent, missingGates := cwsJudgeA(root, aCands)
	// ★ GAP-20260927-154：缺一条即不给结论（旧口径「全缺才 rc=2」⇒ 改名/搬走 1~5 条恒绿）
	if len(missingGates) > 0 {
		fmt.Fprintf(stdout, "FATAL: 闸清单有 %d 个**缺件**（判据无依据 ⇒ 不给结论）⇒ rc=2\n", len(missingGates))
		for _, g := range missingGates {
			fmt.Fprintf(stdout, "  缺：%s\n", g)
		}
		return 2
	}

	candNames := map[string]bool{}
	for _, c := range cands {
		candNames[c.name] = true
	}
	errs, wlNames := cwsValidateWhitelist(candNames, wl)
	// ★ --wired-only：只判断言 A/C，**跳过**零引用普查（断言 B 的贵腿 cwsJudgeB）—— B 面在本档**未判**：
	//   报告里明写「未判」，✗ 不把空表印成「零引用 0 · 未登记 0」（那是假绿）。
	var zero, unreg, reg, stale, docOnly, rows []cwsRow
	if !opts.wiredOnly {
		zero, unreg, reg, stale, docOnly, rows = cwsJudgeB(root, files, cands, wlNames)
	}

	wiredNames := map[string]bool{}
	for _, c := range aCands {
		if cwsCandidateWired(root, c) {
			wiredNames[c.name] = true
		}
	}
	hitsC, cRows, cCounts := cwsJudgeC(root, aCands, wiredNames)

	fmt.Fprintln(stdout, cwsReport(root, opts, files, mode, aCands, cands, hitsA, nWired, nGatePresent,
		missingGates, zero, unreg, reg, stale, docOnly, rows, wl, errs,
		hitsC, cRows, cCounts))
	if len(errs) > 0 {
		return 2
	}
	if opts.strictReport && (len(hitsA) > 0 || len(unreg) > 0 || len(hitsC) > 0) {
		return 1
	}
	return 0
}

func cwsAnyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ── py 路径 / 原语语义家族（自写 · ✗ 不用 filepath.Join/Abs/Rel/Match）───────────
func cwsPyJoin(parts ...string) string {
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

func cwsPyNormpath(path string) string {
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
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func cwsPyAbspath(cwd, path string) string {
	return cwsPyNormpath(cwsPyJoin(cwd, path))
}

func cwsPyRelpath(cwd, path, start string) string {
	if path == "" {
		return ""
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
	s, p := split(cwsPyAbspath(cwd, start)), split(cwsPyAbspath(cwd, path))
	i := 0
	for i < len(s) && i < len(p) && s[i] == p[i] {
		i++
	}
	var rel []string
	for k := 0; k < len(s)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, p[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// cwsDirname = posixpath.split 语义（剥尾斜杠）。
func cwsDirname(p string) string {
	i := strings.LastIndexByte(p, '/')
	head := p[:cwsMaxInt(i, 0)]
	if i < 0 {
		return ""
	}
	head = p[:i+1]
	for len(head) > 1 && strings.HasSuffix(head, "/") {
		head = head[:len(head)-1]
	}
	if head == "/" {
		return "/"
	}
	return strings.TrimSuffix(head, "/")
}

func cwsBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// cwsSplitext = posixpath.splitext：尾段最后一个 `.` 起拆（`.x` 不是扩展名）。
func cwsSplitext(base string) (string, string) {
	i := strings.LastIndexByte(base, '.')
	if i <= 0 {
		return base, ""
	}
	// 只在最后一个 `/` 之后找点（此处入参已是一段）
	return base[:i], base[i:]
}

func cwsFnmatchCase(name, pat string) bool {
	n, p := []rune(name), []rune(pat)
	ni, pi, star, mark := 0, 0, -1, 0
	for ni < len(n) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == n[ni]):
			ni++
			pi++
		case pi < len(p) && p[pi] == '*':
			star, mark, pi = pi, ni, pi+1
		case star >= 0:
			pi = star + 1
			mark++
			ni = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func cwsIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', '\x1c', '\x1d', '\x1e', '\x1f', '\x85', '\xa0':
		return true
	}
	return unicode.IsSpace(r)
}

func cwsPyStrip(s string) string {
	return strings.TrimFunc(s, cwsIsPySpace)
}

// cwsSplitLines = py str.splitlines（覆盖 \n / \r\n / \r；其余 unicode 断行符写未覆盖面）。
func cwsSplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		if c == '\n' {
			out = append(out, string(cur))
			cur = nil
			continue
		}
		if c == '\r' {
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func cwsRepr(s string) string {
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

// ── 宽度（east_asian_width ∈ W,F ⇒ 2 列）──────────────────────────────────────
func cwsDW(s string) int {
	w := 0
	for _, r := range s {
		if cwsEAWWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func cwsEAWWide(r rune) bool {
	cp := uint32(r)
	lo, hi := 0, len(cwsEAWRanges)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if cp < cwsEAWRanges[mid][0] {
			hi = mid - 1
		} else if cp > cwsEAWRanges[mid][1] {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}

func cwsPad(s string, n int) string {
	w := cwsDW(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func cwsPadInt(i, n int) string { return cwsPad(strconv.Itoa(i), n) }

// cwsPadChars 复刻 Python `%-Ns`：按**码点数**（✗ 非显示宽度）左对齐填充。
func cwsPadChars(s string, n int) string {
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func cwsTrunc(s string, n int) string {
	if cwsDW(s) <= n {
		return s
	}
	out := ""
	for _, c := range s {
		if cwsDW(out)+cwsDW(string(c)) > n-1 {
			return out + "…"
		}
		out += string(c)
	}
	return out
}

// ── 文件系统小工具 ────────────────────────────────────────────────────────────
func cwsCwd() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}

func cwsDefaultRoot() string {
	cur := cwsCwd()
	for i := 0; i < 6; i++ {
		if cwsIsDir(cwsPyJoin(cur, "scripts", "gates")) {
			return cur
		}
		parent := cwsDirname(cur)
		if parent == "" || parent == cur || parent == "/" {
			break
		}
		cur = parent
	}
	return cwsCwd()
}

func cwsIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cwsIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func cwsIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func cwsIn(s string, arr []string) bool {
	for _, x := range arr {
		if x == s {
			return true
		}
	}
	return false
}

func cwsMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cwsGitOut(root string, args []string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return out.String(), true
}

// ── 扫描面 ────────────────────────────────────────────────────────────────────
func cwsListFiles(root string) ([]string, string) {
	if cwsIsDir(cwsPyJoin(root, ".git")) {
		out, ok := cwsGitOut(root, []string{"ls-files", "-z"})
		if ok && out != "" {
			var fs []string
			for _, p := range strings.Split(out, "\x00") {
				if p != "" {
					fs = append(fs, p)
				}
			}
			return fs, "git-ls-files（已跟踪 = 索引）"
		}
	}
	var fs []string
	cwsWalkDir(root, root, &fs)
	sort.Strings(fs)
	return fs, "文件系统遍历（非 git 目录）"
}

// cwsWalkDir 复刻 os.walk(followlinks=False) 的产出序：先出当前层文件，再按原序下钻子目录；
// 符号链接一律不进（既不落文件也不下钻 —— py 侧 islink 跳过 + 不跟随）。
func cwsWalkDir(dir, root string, out *[]string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}
	var dirs []string
	for _, e := range ents {
		name := e.Name()
		full := cwsPyJoin(dir, name)
		if cwsIsSymlink(full) {
			continue
		}
		if cwsIsDir(full) {
			if cwsSkipDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
			dirs = append(dirs, full)
			continue
		}
		*out = append(*out, cwsPyRelpath(cwsCwd(), full, root))
	}
	for _, d := range dirs {
		cwsWalkDir(d, root, out)
	}
}

func cwsReadText(path string) (string, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if st.Size() > cwsMaxBytes {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	h := b
	if len(h) > 8192 {
		h = h[:8192]
	}
	if bytes.IndexByte(h, 0) >= 0 {
		return "", false
	}
	return string(b), true
}

func cwsIsRecordSurface(p string) bool {
	for _, pat := range cwsRecordSurfaces {
		if cwsFnmatchCase(p, pat) {
			return true
		}
		seg := strings.Split(pat, "/")
		parts := strings.Split(p, "/")
		if len(parts) == len(seg) {
			ok := true
			for i := range parts {
				if !cwsFnmatchCase(parts[i], seg[i]) {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// ── 候选（门脚本）─────────────────────────────────────────────────────────────
func cwsIsGateFile(p string) bool {
	for _, g := range cwsGates {
		if g.path == p {
			return true
		}
	}
	return false
}

func cwsCandidatePaths(files []string) []string {
	var out []string
	for _, p := range files {
		if !strings.HasPrefix(p, "scripts/") || p == "scripts/" {
			continue
		}
		parts := strings.Split(p, "/")
		if len(parts) == 2 {
			out = append(out, p)
		} else if len(parts) == 3 && !cwsIn(parts[1], cwsExclSubdirs) {
			out = append(out, p)
		}
	}
	return out
}

func cwsBuildCandidates(root string, files []string) []cwsCand {
	var cands []cwsCand
	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}
	tops := cwsCandidatePaths(files)
	sort.Strings(tops)
	for _, p := range tops {
		ap := cwsPyJoin(root, p)
		base := cwsBase(p)
		if !cwsIsFile(ap) {
			continue
		}
		kind := ""
		evidence := ""
		if !strings.HasSuffix(base, ".py") && !strings.HasSuffix(base, ".sh") {
			head := ""
			if f, err := os.Open(ap); err == nil {
				buf := make([]byte, 2)
				n, _ := f.Read(buf)
				f.Close()
				head = string(buf[:n])
			}
			if head != "#!" {
				continue
			}
		}
		if strings.HasPrefix(base, "check-") {
			kind = "命名 check-"
			evidence = "名以 check- 开头"
		} else {
			stem, _ := cwsSplitext(base)
			dirn := cwsDirname(p)
			md := ""
			if dirn != "" {
				md = fmt.Sprintf("%s/%s.md", dirn, stem)
			} else {
				md = fmt.Sprintf("%s.md", stem)
			}
			if fileSet[md] {
				if t, ok := cwsReadText(cwsPyJoin(root, md)); ok {
					lines := cwsSplitLines(t)
					if len(lines) > cwsDocHeadLines {
						lines = lines[:cwsDocHeadLines]
					}
					for i, line := range lines {
						if strings.Contains(line, cwsDocMark) {
							kind = "说明书自称门"
							evidence = fmt.Sprintf("%s:%d 「%s」", md, i+1, cwsTrunc(cwsPyStrip(line), 46))
							break
						}
					}
				}
			}
		}
		cands = append(cands, cwsCand{base, p, kind, evidence})
	}
	return cands
}

// ── 断言 A：命令位调用形态 ─────────────────────────────────────────────────────
const cwsRunnerRe = `\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*|python3|python|bash|sh|zsh|dash|node|env|exec|time|nice|\./`
const cwsPathPrefix = `(?:[\w.${}/-]*/)`

func cwsBareItemRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`^["']?(?:\./)?(?:scripts/)?(?:[A-Za-z0-9_.\-]+/)*` +
		regexp.QuoteMeta(name) + `["']?,?$`)
}

func cwsCallRe(name string) *regexp.Regexp {
	head := `(?:^|[\s"'=(;|&])(?:` + cwsRunnerRe + `)\s+(?:-[^\s]+\s+)*`
	n := regexp.QuoteMeta(name)
	return regexp.MustCompile(head + `"` + cwsPathPrefix + `*` + n + `"` +
		`|` + head + cwsPathPrefix + `*` + n + `(?:[\s"';|)&]|$)`)
}

// cwsZergGateRe —— `zerg gate check <判据名>` 接线形（`GAP-20261001-74`）。
//
//	★ 光把 `zerg` 加进 cwsRunnerRe **不解**：`gate check` 两词夹在启动器与判据名之间 ⇒ 另写专用
//	  规则（`gate check` 不是旗标）。判据名 = 件名**去扩展名**（与 gate_inproc_table.go 的键同口径）。
const cwsZergGatePrefix = `(?:[^\s]*/)?zerg\s+gate\s+check\s+(?:[^\s]*/)*`

func cwsZergGateRe(name string) *regexp.Regexp {
	stem, _ := cwsSplitext(name)
	head := `(?:^|[\s"'=(;|&])`
	return regexp.MustCompile(head + cwsZergGatePrefix + regexp.QuoteMeta(stem) + `(?:[\s"';|)&]|$)`)
}

// cwsCandidateFiles —— 候选面（可换 fs）`GAP-20260927-159/169`：git 档下 = 索引件 ∪ 未跟踪未忽略件；
// 非 git 档原样返回。**引用面（files）一字不动**（在途件里的字面引用会把零引用死件洗白 = 松判）。
func cwsCandidateFiles(root string, files []string, mode string) []string {
	if !strings.HasPrefix(mode, "git") {
		return files
	}
	out, ok := cwsGitOut(root, []string{"ls-files", "--others", "--exclude-standard", "-z"})
	if !ok {
		return files
	}
	var extra []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			extra = append(extra, p)
		}
	}
	if len(extra) == 0 {
		return files
	}
	set := map[string]bool{}
	for _, p := range files {
		set[p] = true
	}
	for _, p := range extra {
		set[p] = true
	}
	all := make([]string, 0, len(set))
	for p := range set {
		all = append(all, p)
	}
	sort.Strings(all)
	return all
}

func cwsWiringHit(root, gatePath, name string) (int, string, bool) {
	t, ok := cwsReadText(cwsPyJoin(root, gatePath))
	if !ok {
		return 0, "", false
	}
	stem, _ := cwsSplitext(name)
	bare := cwsBareItemRe(name)
	call := cwsCallRe(name)
	zerg := cwsZergGateRe(name)
	for i, line := range cwsSplitLines(t) {
		// ★ 预筛放宽：`zerg gate check` 形里名字**不带扩展名**（行里无 name、有去扩展名 stem）
		if !strings.Contains(line, name) && !strings.Contains(line, stem) {
			continue
		}
		s := cwsPyStrip(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if bare.MatchString(s) {
			continue
		}
		if call.FindStringIndex(line) == nil && zerg.FindStringIndex(line) == nil {
			continue
		}
		return i + 1, s, true
	}
	return 0, "", false
}

func cwsJudgeA(root string, cands []cwsCand) (hits []cwsCand, nWired, nGatePresent int, missingGates []string) {
	for _, g := range cwsGates {
		if cwsIsFile(cwsPyJoin(root, g.path)) {
			nGatePresent++
		} else {
			missingGates = append(missingGates, g.path)
		}
	}
	for _, c := range cands {
		if cwsIsGateFile(c.path) {
			nWired++
			continue
		}
		found := false
		for _, g := range cwsGates {
			if _, _, ok := cwsWiringHit(root, g.path, c.name); ok {
				found = true
				break
			}
		}
		if found {
			nWired++
		} else {
			hits = append(hits, c)
		}
	}
	return hits, nWired, nGatePresent, missingGates
}

// ─────────────────────────────────────────────────────────────────────────────
// 断言 C：件头自述三态（**显式 token** · 首版 UNSTATED-only）
//
//	★ ✗ 不解析自然语言：件头现读里 `不挂死`（=不卡住）/`只报告档起步`（历史档位）/`未挂`（描述历史）
//	  都曾字面碰撞（见设计稿 §1.6）⇒ 任何「按词面抽」的判据都会把 `不挂死` 读成「未接线」。
//	  显式 token 把「语义」搬出判据、搬进件头作者的手；判据只做**逐字枚举**（闭集＞自由文本）。
//	★ 形（写死）：行首（可带缩进）`#` ＋ 空白 ＋ `zerg-wire:` ＋ 空白 ＋ <WIRED|UNWIRED|UNSTATED>；
//	  位置：件头**前 40 行内**、**全件唯一**（0 个 / ≥2 个 ⇒ 判 UNSTATED，不猜）。
//	★ 首版 UNSTATED-only：`UNSTATED` **只打印+计数、不进 rc**（≈ 门① 基线棘轮先例，不锁死提交闸）；
//	  只有**失真**（`WIRED`×真未挂 ／ `UNWIRED`×真已挂）才计 `hitsC`，其 rc 同断言 A/B 受
//	  `--strict-report` 门控 ⇒ **默认档不因新增面而转红**（现读全仓 0 token ⇒ hitsC 恒 0）。
//
// ─────────────────────────────────────────────────────────────────────────────
const cwsWireMark = "zerg-wire:"
const cwsWireHeadLines = 40

var cwsWireTokenRe = regexp.MustCompile(`^\s*#\s*zerg-wire:\s*(WIRED|UNWIRED|UNSTATED)\b`)

// cwsReadWireToken 读件头前 cwsWireHeadLines 行内**唯一**的显式 token。返回 (state, note)。
func cwsReadWireToken(root, relpath string) (string, string) {
	t, ok := cwsReadText(cwsPyJoin(root, relpath))
	if !ok {
		return "UNSTATED", "读不到件"
	}
	lines := cwsSplitLines(t)
	if len(lines) > cwsWireHeadLines {
		lines = lines[:cwsWireHeadLines]
	}
	var vals []string
	for _, line := range lines {
		if m := cwsWireTokenRe.FindStringSubmatch(line); m != nil {
			vals = append(vals, m[1])
		}
	}
	if len(vals) == 1 {
		return vals[0], ""
	}
	if len(vals) == 0 {
		return "UNSTATED", fmt.Sprintf("件头前 %d 行无 token", cwsWireHeadLines)
	}
	return "UNSTATED", fmt.Sprintf("件头 token 多重（%s）⇒ 不猜", strings.Join(vals, "/"))
}

// cwsCandidateWired —— 真接线判定（与 cwsJudgeA 同口径）：闸自身 ／ 命中任一闸命令位。
func cwsCandidateWired(root string, c cwsCand) bool {
	if cwsIsGateFile(c.path) {
		return true
	}
	for _, g := range cwsGates {
		if _, _, ok := cwsWiringHit(root, g.path, c.name); ok {
			return true
		}
	}
	return false
}

type cwsWireHit struct{ name, declared, why string }

type cwsWireRow struct {
	name, declared string
	wired          bool
	note           string
}

// cwsJudgeC 断言 C：件头自述三态 × 真接线。hits = **失真**（WIRED×未挂 ／ UNWIRED×已挂）；
// `UNSTATED` 一律不进 hits（首版不判红）。
func cwsJudgeC(root string, cands []cwsCand, wiredNames map[string]bool) (hits []cwsWireHit, rows []cwsWireRow, counts map[string]int) {
	counts = map[string]int{"WIRED": 0, "UNWIRED": 0, "UNSTATED": 0}
	for _, c := range cands {
		declared, note := cwsReadWireToken(root, c.path)
		counts[declared]++
		wired := wiredNames[c.name]
		rows = append(rows, cwsWireRow{c.name, declared, wired, note})
		if declared == "WIRED" && !wired {
			hits = append(hits, cwsWireHit{c.name, "WIRED", "自称已接线，但闸清单命令位里找不到它"})
		} else if declared == "UNWIRED" && wired {
			hits = append(hits, cwsWireHit{c.name, "UNWIRED", "自称未接线，但闸清单里已按名调用它"})
		}
	}
	return hits, rows, counts
}

// ── 断言 B：零引用件必须登记白名单 ─────────────────────────────────────────────
func cwsRefCounts(root string, files []string, name, selfRel string) (int, int, int, []string) {
	total, code, doc := 0, 0, 0
	var samples []string
	for _, p := range files {
		if p == selfRel {
			continue
		}
		if cwsIn(p, cwsSelfFiles) {
			continue
		}
		if cwsIsRecordSurface(p) {
			continue
		}
		t, ok := cwsReadText(cwsPyJoin(root, p))
		if !ok {
			continue
		}
		if !strings.Contains(t, name) {
			continue
		}
		n := strings.Count(t, name)
		total += n
		if strings.HasPrefix(p, "docs/") || strings.Contains(p, "/docs/") {
			doc += n
		} else {
			code += n
		}
		samples = append(samples, fmt.Sprintf("%s(x%d)", cwsTrunc(p, 52), n))
	}
	sort.SliceStable(samples, func(i, j int) bool {
		di, dj := cwsDW(samples[i]), cwsDW(samples[j])
		if di != dj {
			return di > dj
		}
		return samples[i] < samples[j]
	})
	return total, code, doc, samples
}

func cwsValidateWhitelist(candNames map[string]bool, wl []cwsWLEntry) ([]string, map[string]int) {
	var errs []string
	seen := map[string]int{}
	for i, e := range wl {
		n := cwsPyStrip(e.name)
		r := cwsPyStrip(e.reason)
		d := cwsPyStrip(e.date)
		if n == "" {
			errs = append(errs, fmt.Sprintf("白名单第 %d 项缺 name", i+1))
			continue
		}
		if r == "" {
			errs = append(errs, fmt.Sprintf("白名单「%s」缺 reason（登记必须说清它为什么还该在仓里）", n))
		}
		if d == "" {
			errs = append(errs, fmt.Sprintf("白名单「%s」缺 date", n))
		} else if !cwsDateRe.MatchString(d) {
			errs = append(errs, fmt.Sprintf("白名单「%s」的 date 形态不对（要 YYYY-MM-DD，现为 %s）", n, cwsRepr(d)))
		}
		if prev, dup := seen[n]; dup {
			errs = append(errs, fmt.Sprintf("白名单重名「%s」（第 %d 项与第 %d 项）", n, prev, i+1))
		}
		seen[n] = i + 1
		if !candNames[n] {
			errs = append(errs, fmt.Sprintf("白名单**悬空**「%s」：scripts/ 顶层此刻没有这只脚本（删件后必须同步下架）", n))
		}
	}
	return errs, seen
}

func cwsJudgeB(root string, files []string, cands []cwsCand, wlNames map[string]int) (
	zero, unregistered, registered, stale, docOnly []cwsRow, rows []cwsRow) {
	for _, c := range cands {
		total, code, doc, samples := cwsRefCounts(root, files, c.name, c.path)
		_, isWL := wlNames[c.name]
		rows = append(rows, cwsRow{name: c.name, total: total, code: code, doc: doc,
			samples: samples, wl: isWL})
	}
	for _, r := range rows {
		if r.total == 0 {
			zero = append(zero, r)
			if r.wl {
				registered = append(registered, r)
			} else {
				unregistered = append(unregistered, r)
			}
		}
		if r.wl && r.total > 0 {
			stale = append(stale, r)
		}
		if r.total > 0 && r.code == 0 {
			docOnly = append(docOnly, r)
		}
	}
	return zero, unregistered, registered, stale, docOnly, rows
}

// ── 报告 ─────────────────────────────────────────────────────────────────────
func cwsHeader(root string) (string, string) {
	head := "?"
	dirty := "?"
	if cwsIsDir(cwsPyJoin(root, ".git")) {
		h, _ := cwsGitOut(root, []string{"rev-parse", "--short", "HEAD"})
		if hs := cwsPyStrip(h); hs != "" {
			head = hs
		}
		s, _ := cwsGitOut(root, []string{"status", "--porcelain"})
		n := 0
		for _, x := range cwsSplitLines(s) {
			if cwsPyStrip(x) != "" {
				n++
			}
		}
		dirty = strconv.Itoa(n)
	}
	return head, dirty
}

func cwsReport(root string, opts cwsOpts, files []string, mode string, aCands, bCands []cwsCand,
	hitsA []cwsCand, nWired, nGatePresent int, missingGates []string,
	zero, unreg, reg, stale, docOnly, rows []cwsRow, wl []cwsWLEntry, errs []string,
	hitsC []cwsWireHit, cRows []cwsWireRow, cCounts map[string]int) string {
	var out []string
	ts := time.Now().Format("2006-01-02 15:04:05")
	head, dirty := cwsHeader(root)
	nCheck, nDecl := 0, 0
	for _, c := range aCands {
		if c.kind == "命名 check-" {
			nCheck++
		} else if c.kind == "说明书自称门" {
			nDecl++
		}
	}
	out = append(out, "门③ check-wired-scripts — 脚本接线自检（挂闸档 `tri` + `--strict-report` ⇒ 阻断；不带开关时默认只报告）")
	out = append(out, "")
	out = append(out, fmt.Sprintf("仓根   : %s", root))
	out = append(out, fmt.Sprintf("快照   : %s ｜ HEAD %s ｜ 工作树改动 %s 件", ts, head, dirty))
	out = append(out, fmt.Sprintf("扫描面 : A 候选（门脚本）%d 只（命名 check- %d · 说明书自称门 %d）",
		len(aCands), nCheck, nDecl))
	out = append(out, fmt.Sprintf("         B 候选（scripts/ 顶层脚本）%d 只", len(bCands)))
	nRecord := 0
	for _, p := range files {
		if cwsIsRecordSurface(p) {
			nRecord++
		}
	}
	out = append(out, fmt.Sprintf("         引用面 %d 个文件（%s；排除 自指 %d · 记录面 %d · 候选自身）",
		len(files), mode, len(cwsSelfFiles), nRecord))
	nCommit, nRelease := 0, 0
	for _, g := range cwsGates {
		if g.kind == "commit" {
			nCommit++
		} else {
			nRelease++
		}
	}
	missStr := ""
	if len(missingGates) > 0 {
		missStr = "  ⚠ 不在位：" + strings.Join(missingGates, " · ")
	}
	out = append(out, fmt.Sprintf("闸清单 : 在位 %d / %d（提交闸 %d · 发布闸 %d）%s",
		nGatePresent, len(cwsGates), nCommit, nRelease, missStr))
	for _, g := range cwsGates {
		mark := "✗"
		if cwsIsFile(cwsPyJoin(root, g.path)) {
			mark = "✓"
		}
		out = append(out, fmt.Sprintf("         %s %s %s", mark, cwsPadChars(g.label, 14), g.path))
	}

	out = append(out, "")
	out = append(out, fmt.Sprintf("【断言 A】门脚本必须出现在至少一个闸里 —— 命中 %d 只（未挂）", len(hitsA)))
	if len(hitsA) == 0 {
		out = append(out, "  ✓ 无未挂门脚本")
	} else {
		out = append(out, fmt.Sprintf("   %s %s %s %s %s", cwsPad("#", 4), cwsPad("脚本", 30),
			cwsPad("类型", 14), cwsPad("闸里按名调用", 12), "建议挂到哪里"))
		for i, c := range hitsA {
			sg, ok := cwsSuggest[c.name]
			if !ok {
				sg = cwsSuggestT{"未登记建议 ⇒ 人工判定（该补哪条闸由人定）", ""}
			}
			out = append(out, fmt.Sprintf("   %s %s %s %s %s", cwsPadInt(i+1, 4), cwsPad(c.name, 30),
				cwsPad(c.kind, 14), cwsPad("0 处", 12), cwsTrunc(sg.sug, 60)))
			if sg.why != "" {
				for j, seg := range strings.Split(sg.why, "；") {
					prefix := "      "
					if j == 0 {
						prefix = "理由："
					}
					out = append(out, fmt.Sprintf("       %s %s", prefix, cwsTrunc(seg, 96)))
				}
			}
		}
	}
	out = append(out, fmt.Sprintf("  已挂门脚本 %d 只（不打印；--list-rules 可看闸清单与调用方式）", nWired))

	out = append(out, "")
	if opts.wiredOnly {
		out = append(out, "【断言 B】**未判**（--wired-only：零引用普查**未执行**）—— 本档**不给出** B 的任何数字（≠「零引用 0 · 未登记 0」）")
	} else {
		out = append(out, fmt.Sprintf("【断言 B】零引用件必须登记白名单 —— 候选 %d 只 · 零引用 %d 只 · **未登记 %d 只**",
			len(rows), len(zero), len(unreg)))
	}
	byName := map[string]cwsWLEntry{}
	for _, e := range wl {
		if e.name != "" {
			byName[e.name] = e
		}
	}
	if len(reg) > 0 {
		out = append(out, fmt.Sprintf("  已登记（%d 只）：", len(reg)))
		for _, r := range reg {
			e, ok := byName[r.name]
			date, reason := "?", "?"
			if ok {
				date = e.date
				reason = e.reason
			}
			out = append(out, fmt.Sprintf("    %s  date=%s", cwsPad(r.name, 26), date))
			out = append(out, fmt.Sprintf("      reason: %s", cwsTrunc(reason, 200)))
		}
	}
	if len(unreg) > 0 {
		out = append(out, fmt.Sprintf("  ✗ 未登记（%d 只）—— 要么接线（被谁调用）、要么登记白名单（附理由）：", len(unreg)))
		for _, r := range unreg {
			out = append(out, fmt.Sprintf("    %s  refs=0（引用面里零命中）", cwsPad(r.name, 26)))
		}
	} else if len(zero) > 0 {
		out = append(out, fmt.Sprintf("  ✓ 零引用件全部已登记（%d/%d）", len(reg), len(zero)))
	}
	if len(stale) > 0 {
		var parts []string
		for _, r := range stale {
			parts = append(parts, fmt.Sprintf("%s(refs=%d)", r.name, r.total))
		}
		out = append(out, fmt.Sprintf("  ⚠ 白名单可下架（已有引用 ⇒ 不再是死件；棘轮只许减）：%s",
			strings.Join(parts, " · ")))
	}
	if len(docOnly) > 0 {
		out = append(out, fmt.Sprintf("  ⚠ 仅文档点名（total>0 但引用面里没有代码侧调用 ⇒ 不是死件、也不是活件；供人工判）：共 %d 只", len(docOnly)))
		lim := len(docOnly)
		if lim > 12 {
			lim = 12
		}
		for _, r := range docOnly[:lim] {
			sn := r.samples
			if len(sn) > 3 {
				sn = sn[:3]
			}
			out = append(out, fmt.Sprintf("    %s  refs=%d（文档 %d / 代码侧 %d）例：%s",
				cwsPad(r.name, 26), r.total, r.doc, r.code, strings.Join(sn, " · ")))
		}
		if len(docOnly) > 12 {
			out = append(out, fmt.Sprintf("    …另有 %d 只同为「仅文档点名」（口径相同，为控篇幅不逐条打印）", len(docOnly)-12))
		}
	}

	// ── 断言 C：件头自述三态（首版 UNSTATED-only）────────────────────────────────
	out = append(out, "")
	cw := cCounts["WIRED"]
	cu := cCounts["UNWIRED"]
	cn := cCounts["UNSTATED"]
	out = append(out, fmt.Sprintf("【断言 C】件头自述三态（`# zerg-wire: WIRED|UNWIRED|UNSTATED` · 行首 · 件头前 %d 行 · 全件唯一）—— WIRED %d · UNWIRED %d · **UNSTATED %d**", cwsWireHeadLines, cw, cu, cn))
	out = append(out, "   ★ 首版 UNSTATED-only：UNSTATED 只打印+计数、**不判红**；真接线 = 命中闸清单命令位（同断言 A）")
	out = append(out, "   ★ 失真（WIRED×未挂 ／ UNWIRED×已挂）计 hits_c，rc 同 A/B 受 --strict-report 门控（默认档不因此转红）")
	if len(hitsC) > 0 {
		out = append(out, fmt.Sprintf("  ✗ 失真 %d 只（自称 ≠ 真接线）：", len(hitsC)))
		for _, h := range hitsC {
			out = append(out, fmt.Sprintf("    %s  自述=%s · %s", cwsPad(h.name, 30), h.declared, h.why))
		}
	} else {
		out = append(out, fmt.Sprintf("  ✓ 无失真（已自述 %d 只均与真接线一致）", cw+cu))
	}
	if cn > 0 {
		out = append(out, fmt.Sprintf("  · UNSTATED %d 只（不判红；逐件打印）：", cn))
		for _, r := range cRows {
			if r.declared == "UNSTATED" {
				out = append(out, fmt.Sprintf("    %s  %s", cwsPad(r.name, 30), r.note))
			}
		}
	}

	out = append(out, "")
	out = append(out, "【排除域】不在本门判据里（现读，非记忆）")
	deepSet := map[string]bool{}
	for _, p := range files {
		if strings.HasPrefix(p, "scripts/") && strings.Count(p, "/") > 3 {
			parts := strings.Split(p, "/")
			deepSet[strings.Join(parts[:3], "/")] = true
		}
	}
	var deep []string
	for d := range deepSet {
		deep = append(deep, d)
	}
	sort.Strings(deep)
	for _, d := range deep {
		n := 0
		for _, p := range files {
			if strings.HasPrefix(p, d+"/") {
				n++
			}
		}
		out = append(out, fmt.Sprintf("  · 更深一层：%s/（%d 件，候选面只到 scripts/<域>/ 这一层）", d, n))
	}
	for _, d := range cwsExclSubdirs {
		n := 0
		for _, p := range files {
			if strings.HasPrefix(p, "scripts/"+d+"/") {
				n++
			}
		}
		if n > 0 {
			out = append(out, fmt.Sprintf("  · 排除域：scripts/%s/（%d 件，分域前就有的工作/实验目录，不进判据）", d, n))
		}
	}
	if cwsIsDir(cwsPyJoin(root, ".git")) {
		o, _ := cwsGitOut(root, []string{"ls-files", "-o", "--exclude-standard", "-z", "--", "scripts"})
		var untracked []string
		for _, p := range strings.Split(o, "\x00") {
			if p != "" {
				untracked = append(untracked, p)
			}
		}
		if len(untracked) > 0 {
			lim := len(untracked)
			if lim > 6 {
				lim = 6
			}
			var parts []string
			for _, p := range untracked[:lim] {
				parts = append(parts, cwsTrunc(p, 40))
			}
			out = append(out, fmt.Sprintf("  · 未跟踪件 %d 件（未入库 ⇒ 不在判据内；入库后首跑会看到它）：%s",
				len(untracked), strings.Join(parts, " · ")))
		}
	}
	for _, ng := range cwsNotGates {
		if cwsIsFile(cwsPyJoin(root, ng.path)) {
			out = append(out, fmt.Sprintf("  · 相关但不是闸：%s（%s）", ng.path, ng.why))
		}
	}
	out = append(out, "  · 候选面 = scripts/ **顶层 + 一层子目录**（scripts/<件> 与 scripts/<域>/<件>；.py/.sh/无后缀 shebang）；更深的树不在判据内")

	if len(errs) > 0 {
		out = append(out, "")
		out = append(out, fmt.Sprintf("【配置错】%d 条（⇒ rc=2，不给结论）", len(errs)))
		for _, e := range errs {
			out = append(out, fmt.Sprintf("  ✗ %s", e))
		}
	}

	out = append(out, "")
	hitN := len(hitsA) + len(unreg) + len(hitsC)
	out = append(out, fmt.Sprintf("【口径】引用 = 脚本名在引用面文件里出现；引用面 = 已跟踪文本文件 − 自指 2 件 − 记录面（现行债务台账，glob %s）− 候选自身。",
		strings.Join(cwsRecordSurfaces, " · ")))
	out = append(out, "        「记录不能充作引用」：台账点名不让死件复活（否则写下这条债就把债抹掉了）。")
	out = append(out, "        「在闸里」= **命令位调用**（真接线面）：名字紧跟解释器/启动器（`python3`/`bash`/`${PY}`/`./`…），中间只允许选项旗标。")
	out = append(out, "        不算接线的三类：纯注释行 · 纯清单项（`\"scripts/<…>/<名>\"`，**含被排除表/白名单点名**）· 只被文案/说明/grep 模式串提到。")
	out = append(out, "")
	bField := "未判（--wired-only）"
	if !opts.wiredOnly {
		bField = fmt.Sprintf("未登记 %d", len(unreg))
	}
	out = append(out, fmt.Sprintf("【结论】A 命中 %d · B %s · C 失真 %d · 配置错 %d", len(hitsA), bField, len(hitsC), len(errs)))
	if len(errs) > 0 {
		out = append(out, "        rc=2（缺件/配置错/空转 ⇒ 不给结论，不许静默）")
	} else if opts.strictReport && hitN > 0 {
		out = append(out, fmt.Sprintf("        --strict-report 且命中 %d ⇒ rc=1", hitN))
	} else if opts.strictReport {
		out = append(out, "        阻断档（`--strict-report`）且**零命中** ⇒ rc=0（挂闸档：此后新增一只未挂/未登记即 rc=1）")
	} else {
		out = append(out, "        只报告档 ⇒ rc=0（命中不阻断；挂闸时用 --strict-report 或按 .md §五 走棘轮）")
	}
	out = append(out, "升阻断：**已执行**（2026-09-18 ③批次 · 提交闸 `gates` scope · `tri` + `--strict-report`）—— 存量 A 命中 8→0 后升档，此后新增一只未挂/未登记即红；全过程见 scripts/gates/check-wired-scripts.md §五")
	return strings.Join(out, "\n")
}

// ── --list-rules ────────────────────────────────────────────────────────────
func cwsListRules() string {
	var L []string
	L = append(L, "门③ check-wired-scripts — 判据 / 闸清单 / 口径（--list-rules）")
	L = append(L, "")
	L = append(L, fmt.Sprintf("断言 A（门脚本必须上岗）：凡门脚本 —— ① 名以 `check-` 开头（scripts/ 顶层 · .py/.sh/无后缀 shebang）或 ② 说明书自称门（同干 .md 头部 %d 行内出现「门」）—— 必须出现在至少一个闸里。", cwsDocHeadLines))
	L = append(L, "  「在闸里」判据 = **命令位调用**（真接线面）：名字紧跟解释器/启动器（`python3` / `bash` / `${PY}` / `./` …），中间只允许选项旗标；可带引号与路径前缀（`python3 \"$Z/scripts/gates/<名>\" \"$D\"`）。")
	L = append(L, "    不算接线：① 纯注释行（`#` 开头）② 纯清单项（整行就是 `\"scripts/<…>/<名>\",` 这类数组/数据项—— **含被 `EXCLUDES` / `DROP_EXACT` 这类排除表点名**，任意层深）③ 只被文案（`echo \"…\"`）/ 说明文字 / grep 模式串提到。")
	L = append(L, "断言 B（零引用件必须登记）：scripts/ 顶层零引用（引用面里出现 0 次）⇒ 必须登记白名单（每项 name + reason + date；缺一即 rc=2；点名的脚本已不存在即 rc=2 悬空）。")
	L = append(L, "")
	L = append(L, "闸清单（判据只吃这份表；实际文件名与调用方式 = 2026-09-18 现读）：")
	for _, g := range cwsGates {
		kindLabel := "发布"
		if g.kind == "commit" {
			kindLabel = "提交"
		}
		L = append(L, fmt.Sprintf("  [%s] %s %s", kindLabel, cwsPadChars(g.path, 36), g.label))
		L = append(L, fmt.Sprintf("       调用方式：%s", g.how))
	}
	L = append(L, "相关但不是闸（不进判据）：")
	for _, ng := range cwsNotGates {
		L = append(L, fmt.Sprintf("  · %s %s", cwsPadChars(ng.path, 36), ng.why))
	}
	L = append(L, "")
	L = append(L, "扫描面与排除域：")
	L = append(L, "  · 候选 = scripts/ 顶层（glob `scripts/*`，**不递归**；子目录如 scripts/exportnames/ 打印进排除域）")
	L = append(L, "  · 引用面 = git 索引里的已跟踪文件（`git ls-files`）；未跟踪件不在判据内")
	L = append(L, fmt.Sprintf("  · 排除 ① 本门自指 %s（否则我自己的名单会把零引用写成有引用）", strings.Join(cwsSelfFiles, " · ")))
	L = append(L, fmt.Sprintf("  · 排除 ② 记录面 glob：%s —— 「记录不能充作引用」", strings.Join(cwsRecordSurfaces, " · ")))
	L = append(L, "  · 排除 ③ 候选脚本自身")
	L = append(L, "")
	L = append(L, fmt.Sprintf("白名单（%d 条 · 只许减不许增；有引用后提示可下架）：", len(cwsWhitelist)))
	for _, e := range cwsWhitelist {
		L = append(L, fmt.Sprintf("  · %s date=%s  reason=%s", cwsPadChars(e.name, 24), e.date, cwsTrunc(e.reason, 80)))
	}
	L = append(L, "")
	L = append(L, "退码：0 = 只报告档正常出口（命中不阻断）· 1 = 仅 --strict-report 且有命中 · 2 = 缺件/配置错/空转/自检不过（不给结论）")
	L = append(L, "升级路径（.md §五 · **已执行 2026-09-18 ③批次**）：清存量债 → 记基线快照（逐文件计数）→ --strict-report 挂进闸（新增一条即红）→ 只许减不许增，每还掉 N 条下调一格 → 降到 0 转完全阻断、基线表退役。")
	L = append(L, "  现状：8 只未挂门脚本逐只上岗 ⇒ A 命中 8→0 · B 未登记 0 ⇒ 已挂 `tri` + `--strict-report`（阻断档）。")
	L = append(L, "  ★ 与第 2 步的差异（如实登记）：**基线快照表未建** —— 存量是**当批清到 0** 而不是逐格下调，")
	L = append(L, "    正落在 §五 第 5 步「A 命中 0 且 B 未登记 0 ⇒ 转完全阻断、撤掉基线表」那一格，故不需要棘轮表。")
	L = append(L, "  ★ 阻断面由**闸的步骤**给（模式 `tri` + 开关 `--strict-report`）；本脚本的默认档位**未改**")
	L = append(L, "    （默认仍只报告 ⇒ 手工跑一下不会突然拦人），谁摘掉那个开关就会退化成恒绿步（自检 ⑩ 已钉住）。")
	return strings.Join(L, "\n")
}

var _ = cwsSkipExt
var _ = cwsIsPySpace

// 本块由正门 code show 的 TSV 现读程序化生成（✗ 非手抄），
// 白名单件名的「.」以 \u002E 转义落地 ⇒ 源码正文不含该件名的字面（自指面消解）。

var cwsGates = []cwsGateT{
	{"scripts/gates/precommit-gates.sh", "commit", "提交闸", "bash scripts/gates/precommit-gates.sh [--scope go|rust|pub|tags|docs]（步骤表 add_step 的 STEP_CMD）"},
	{"scripts/build/publish-preflight.sh", "release", "发布闸·推送前置（闸⓪~⑥，含闸②b）", "bash scripts/build/publish-preflight.sh <产物目录>（推送前硬闸，八道全过才允许推；rc=2 = 有闸缺件不给结论）"},
	{"publish/mirror-public.sh", "release", "发布闸·逐提交镜像器", "publish/mirror-public.sh --out DIR [--push]（对产出树跑门禁）"},
	{"publish/mirror-public-lib.py", "release", "发布闸·镜像器库（跑 checker 的那半边）", "被 publish/mirror-public.sh / publish/parity-compare.py 调用（check-history-secrets.py / check-public-tree-private.py 的调用方）"},
	{"publish/ci/ci.yml", "release", "发布闸·CI（公开仓推后闸）", "GitHub Actions：run: python3 scripts/... （check_version / check-gotoolchain / check-compat-manifest / check_docs / check-shell-unicode-vars）"},
	{"publish/ci/release-agent.yml", "release", "发布闸·CI（Release 重切）", "GitHub Actions：python3 scripts/gates/check_version.py · scripts/evals/make-manifest.py"},
	{"publish/mirror-acceptance.sh", "release", "发布闸·验收套件（推送前置）", "SKIP_BUILD=1 bash publish/mirror-acceptance.sh（`publish/mirror-public.sh` PUSH=1 与 `publish/mirror-cn.sh` 推送段前各一行；rc=1 拦截 · `GAP-20260927-138`）"},
}

var cwsNotGates = []cwsNotGateT{
	{"scripts/build/publish-public.sh", "压平快照发布器 · 已弃用（Q5 待废弃；现役为 mirror-public.sh）"},
	{"scripts/build/test-publish-parity.sh", "两器一致性测试（测试面）"},
}

var cwsSelfFiles = []string{
	"scripts/gates/check-wired-scripts.py",
	"scripts/gates/check-wired-scripts.md",
}

var cwsRecordSurfaces = []string{
	"docs/项目文档/*/债务台账-*",
	//   ★ `GAP-20260927-139`（记录面不得充作引用）：`scripts/公开标记.tsv` 点名了全部候选脚本、
	//     却 0 处命令位调用 ⇒ 旧口径把它算作「代码侧引用」⇒「未登记」档结构上永不红。本条把这张
	//     记录类表接进记录面（与 py `RECORD_SURFACES` 同口径）。
	"scripts/公开标记.tsv",
}

var cwsWhitelist = []cwsWLEntry{
	{"v25_more_classes\u002Epy", "一次性历史评测脚本：v2.5 补充任务类测试（example-35b-v2 · 2026-08-13），结果已并入当版文档；保留作评测口径的可复核原件。", "2026-09-18"},
	{"v25_more_tasks\u002Epy", "一次性历史评测脚本：v2.5 更多实战任务（2026-08-13），同族口径。", "2026-09-18"},
	{"v25_real10_v2\u002Epy", "一次性历史评测脚本：v2.5 真实任务集重测（responses 后 · 2026-08-13），同族口径。", "2026-09-18"},
	{"v25_real10\u002Epy", "一次性历史评测脚本：v2.5 真实任务集（2026-08-13，同族口径见 v25_real10_v2\u002Epy）。它原本唯一的引用 = `docs/issues/live-task2.log`（引擎运行态日志，非跟踪件）——该目录已于 2026-09-19 随开发文档分家整目录迁出工作树（`Zerg-内部文档/issues/`）⇒ 引用面归零，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 1 只」= FAIL(rc=1) ↔ 登记后 白名单 5 条、未登记 0 只 = rc=0（豁免数只增这 1 条，逐条带 reason+date）。", "2026-09-19"},
	{"v24_full_test\u002Epy", "零引用件（登记，非死件）：v2.4 全量评测脚本（测试存档体系的 v2.4 那一代）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.9/09-模块-Swift壳工具链-20260829.md` 与 `…/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_agent_tasks\u002Epy", "零引用件（登记，非死件）：v2.5 子端/agent 类任务评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`（另见 `v2.5.0/设计-测试存档体系.md`）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_benchmark\u002Epy", "零引用件（登记，非死件）：v2.5 基准/对比评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_eval_task\u002Epy", "零引用件（登记，非死件）：v2.5 单任务评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md` 与 `v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_full_test\u002Epy", "零引用件（登记，非死件）：v2.5 全量测试脚本（v2.5.0 时代测试存档体系那一代）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md` 与 `v2.5.0/测试报告-v2.5.md`、`v2.5.0/INDEX.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_m1_accept\u002Epy", "零引用件（登记，非死件）：v2.5 M1 里程碑验收脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_real_tasks\u002Epy", "零引用件（登记，非死件）：v2.5 真实任务集评测脚本（同族见 v25_real10\u002Epy / v25_real10_v2\u002Epy）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`（另见 `v2.5.0/设计-测试存档体系.md`）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_realproj\u002Epy", "零引用件（登记，非死件）：v2.5 真实项目集评测脚本（同族口径）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"v25_research\u002Epy", "零引用件（登记，非死件）：v2.5 调研类任务评测脚本（同族口径）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"run_all\u002Esh", "零引用件（登记，非死件）：v2.4/v2.5 评测批跑总入口。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.9/09-模块-Swift壳工具链-20260829.md` 与 `…/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"zerg-cron-submit\u002Esh", "零引用件（登记，非死件）：主控内部任务（cron）提交器。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.9/09-模块-Swift壳工具链-20260829.md` 与 `…/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"calib-x3-model\u002Esh", "零引用件（登记，非死件）：EDGE-NODE 模型标定脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`、`v2.5.9/茧壁-自主作业日志.md`、`v2.5.10/09-模块-Swift壳工具链-20260829.md`、`v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"test-fleet-sandbox\u002Esh", "零引用件（登记，非死件）：舰队面沙箱端到端夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"test-relay-sandbox\u002Esh", "零引用件（登记，非死件）：中转（relay）沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"test-setup-sandbox\u002Esh", "零引用件（登记，非死件）：安装/初始化沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`（唯一一处点名）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"test-update-sandbox\u002Esh", "零引用件（登记，非死件）：升级/换件沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"stability-stats\u002Epy", "零引用件（登记，非死件）：稳定性统计脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
	{"ab-campaigns\u002Epy", "零引用件（登记，非死件）：A/B 活动编排脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。", "2026-09-19"},
}

var cwsSuggest = map[string]cwsSuggestT{
	"check-glossary.py":                {"提交闸 · docs scope（模式 tri-report 起步）", "与 check-doc-* 同族（文档/术语面），docs 已在默认 scope 集里（precommit-gates.sh:47-50）；现用法 `python3 scripts/gates/check-glossary.py --lang zh|en [--enforce]`（docs/zh/README.md:56 · docs/en/README.md:98）。术语档 T2 现为非阻塞 ⇒ 起步按「只报告」挂，同 D2 先例。"},
	"check-manifest-freshness.py":      {"发布闸 · scripts/build/publish-preflight.sh 新增一道（对产物目录判清单新鲜度，模式 tri）", "它判的是**发布产物**（manifest 的新鲜度），归发布面；现形态是「只告警不拒绝」，退出码恒 0 除 `--strict`（Zerg-内部文档/调研/调研-文档分块落地-门禁与迁移.md:373 —— 该件 2026-09-19 已随开发文档分家迁出工作树，原路径 docs/调研/）。"},
	"check-slice.py":                   {"提交闸 · 新 scope（例 slice，模式 tri-report 起步）", "自带 `--probe` / `--selftest` / `--list-rules` 三档与「自检不过 rc=2」的口径（Zerg-内部文档/01-设计/报告模板-六项成对.md:69-70 —— 该件 2026-09-19 已随开发文档分家迁出工作树，原路径 docs/01-设计/）⇒ 可直接当步骤；但体量大（109 KB + slice-probes.json）⇒ 成本未标定，起步只报告。"},
	"check-i18n-drift.sh":              {"提交闸 · docs scope（模式 tri-report 起步）", "漂移三件套（配对 / 提交级陈旧 / 源注释），①②阻塞、③告警（docs/en/README.md:99）；用法 `bash scripts/gates/check-i18n-drift.sh`（docs/site/README.md:189）；存量债未清 ⇒ 起步只报告。"},
	"check-tool-version-sync.sh":       {"提交闸 · 新 scope（例 tools，模式 tri）", "它就是为「发布前勾选」造的（docs/skills/tool-upgrade-checklist.md:52 列在手检清单里：台账 ↔ 工具文档抬头 ↔ 履历）；自带前置自检（缺 tools/versions.json 即 rc=2）⇒ 与 tri 三档一一对应，可直接阻断。"},
	"check-hardcoded-private-paths.py": {"发布闸 · scripts/build/publish-preflight.sh 新增一道（紧跟闸② 私有面，模式 tri）", "它判私有绝对路径，与 闸②（私有面）/ 闸④（危险路径）同族；今天它只被 scripts/build/publish-public.sh:130 的 EXCLUDES **清单项** 与 precommit-gates.sh:500 的**注释**提到 —— 从没被任何闸按名执行 ⇒ 按本门判据必须报出来（子串口径会漏掉这一只）。"},
	"edit-assert":                      {"提交闸 · pub scope 增一步 `python3 scripts/docs/edit-assert --self-test`（模式 tri）", "它今天只被 pub scope 的「scripts/*（无后缀 + 首行 #!）按 shebang 语法」步按 **glob** 做语法检查（precommit-gates.sh:521），**没有按名跑过它的自检**；自检自带真命令行 + 真退出码 16 条（scripts/docs/edit-assert.md:34）⇒ 有牙齿。"},
	"mutate-scan":                      {"提交闸 · pub scope 增一步 `python3 scripts/gates/mutate-scan --self-test`（模式 tri）", "同族：只在 glob 语法检查里被覆盖；自检不过即拒绝跑真目标（scripts/gates/mutate-scan.md:27, 48）⇒ 适合当步骤。"},
	"check-registry-vs-disk.py":        {"提交闸 · gates scope 增一步 `python3 scripts/gates/check-registry-vs-disk.py --strict`（模式 tri）", "纯仓内只读：登记表（`publish/whitelist.txt` / `EXCLUDES`）↔ 盘上真物对拍（空指 / 悬空 / 文案不符）；与闸②b / 闸④ 同族，但它的目标面在仓内 ⇒ 也可放发布闸。★ 现读 `--strict` = **rc=1（三格合计 5 处）**，而今天没有任何真闸会跑到它 ⇒ 这 5 处红是**隐形的**（`all.sh` 逐门自检档只跑它的夹具自检 rc=0；`real-gates.sh` 是按变量名泛跑的档，不是按名接线）。"},
	"check-git-env-hygiene.py":         {"提交闸 · gates scope 增一步 `python3 scripts/gates/check-git-env-hygiene.py --strict`（模式 tri）", "它判「hook / 门禁脚本对外仓·合成仓调 git 前先清环境」= **门禁自身的卫生面**，与 gates scope 同族；现档 = 报告档（件内自称与 `check-registry-vs-disk.py` 同规）⇒ 起步可 tri-report。"},
	"check-public-face-anchor.py":      {"发布闸 · `scripts/build/publish-preflight.sh` ⑧ 系列新增一道（紧跟 ⑧c 命令名 / ⑧d 成对闸，模式 tri）", "公开面**漂移锚**（两版窗口对拍）与 ⑧c/⑧d 同面同规（同属「公开面判据件」）；今天它只被 `publish-preflight.sh:187` 的一句**文案**（`.py` 名字出现在那行的尾注里）与`publish/mirror-public-lib.py:346` 的 `DROP_EXACT` 清单项提到 ⇒ 0 次真调用。"},
	"check-help-snapshot.py":           {"提交闸 · docs scope 增一步 `python3 scripts/gates/check-help-snapshot.py`（模式 tri-report 起步）", "判「`--help` 现跑 ↔ 最新快照」；读同干文档仓（默认同级 `Zerg-内部文档`）⇒ 落 docs 面。与文档类门同族，存量差异未清 ⇒ 起步只报告（同 `check-glossary.py` 先例）。"},
}

// cwsEAWRanges —— 现读 /usr/bin/python3 (3.9.6) unicodedata.east_asian_width ∈ (W,F) 的码点区间
// （= dw() 判 2 列宽的集合）。由程序化脚本生成，✗ 非手抄。
var cwsEAWRanges = [][2]uint32{
	{888, 889}, {896, 899}, {907, 907}, {909, 909}, {930, 930}, {1328, 1328}, {1367, 1368},
	{1419, 1420}, {1424, 1424}, {1480, 1487}, {1515, 1518}, {1525, 1535}, {1565, 1565}, {1806, 1806},
	{1867, 1868}, {1970, 1983}, {2043, 2044}, {2094, 2095}, {2111, 2111}, {2140, 2141}, {2143, 2143},
	{2155, 2207}, {2229, 2229}, {2248, 2258}, {2436, 2436}, {2445, 2446}, {2449, 2450}, {2473, 2473},
	{2481, 2481}, {2483, 2485}, {2490, 2491}, {2501, 2502}, {2505, 2506}, {2511, 2518}, {2520, 2523},
	{2526, 2526}, {2532, 2533}, {2559, 2560}, {2564, 2564}, {2571, 2574}, {2577, 2578}, {2601, 2601},
	{2609, 2609}, {2612, 2612}, {2615, 2615}, {2618, 2619}, {2621, 2621}, {2627, 2630}, {2633, 2634},
	{2638, 2640}, {2642, 2648}, {2653, 2653}, {2655, 2661}, {2679, 2688}, {2692, 2692}, {2702, 2702},
	{2706, 2706}, {2729, 2729}, {2737, 2737}, {2740, 2740}, {2746, 2747}, {2758, 2758}, {2762, 2762},
	{2766, 2767}, {2769, 2783}, {2788, 2789}, {2802, 2808}, {2816, 2816}, {2820, 2820}, {2829, 2830},
	{2833, 2834}, {2857, 2857}, {2865, 2865}, {2868, 2868}, {2874, 2875}, {2885, 2886}, {2889, 2890},
	{2894, 2900}, {2904, 2907}, {2910, 2910}, {2916, 2917}, {2936, 2945}, {2948, 2948}, {2955, 2957},
	{2961, 2961}, {2966, 2968}, {2971, 2971}, {2973, 2973}, {2976, 2978}, {2981, 2983}, {2987, 2989},
	{3002, 3005}, {3011, 3013}, {3017, 3017}, {3022, 3023}, {3025, 3030}, {3032, 3045}, {3067, 3071},
	{3085, 3085}, {3089, 3089}, {3113, 3113}, {3130, 3132}, {3141, 3141}, {3145, 3145}, {3150, 3156},
	{3159, 3159}, {3163, 3167}, {3172, 3173}, {3184, 3190}, {3213, 3213}, {3217, 3217}, {3241, 3241},
	{3252, 3252}, {3258, 3259}, {3269, 3269}, {3273, 3273}, {3278, 3284}, {3287, 3293}, {3295, 3295},
	{3300, 3301}, {3312, 3312}, {3315, 3327}, {3341, 3341}, {3345, 3345}, {3397, 3397}, {3401, 3401},
	{3408, 3411}, {3428, 3429}, {3456, 3456}, {3460, 3460}, {3479, 3481}, {3506, 3506}, {3516, 3516},
	{3518, 3519}, {3527, 3529}, {3531, 3534}, {3541, 3541}, {3543, 3543}, {3552, 3557}, {3568, 3569},
	{3573, 3584}, {3643, 3646}, {3676, 3712}, {3715, 3715}, {3717, 3717}, {3723, 3723}, {3748, 3748},
	{3750, 3750}, {3774, 3775}, {3781, 3781}, {3783, 3783}, {3790, 3791}, {3802, 3803}, {3808, 3839},
	{3912, 3912}, {3949, 3952}, {3992, 3992}, {4029, 4029}, {4045, 4045}, {4059, 4095}, {4294, 4294},
	{4296, 4300}, {4302, 4303}, {4352, 4447}, {4681, 4681}, {4686, 4687}, {4695, 4695}, {4697, 4697},
	{4702, 4703}, {4745, 4745}, {4750, 4751}, {4785, 4785}, {4790, 4791}, {4799, 4799}, {4801, 4801},
	{4806, 4807}, {4823, 4823}, {4881, 4881}, {4886, 4887}, {4955, 4956}, {4989, 4991}, {5018, 5023},
	{5110, 5111}, {5118, 5119}, {5789, 5791}, {5881, 5887}, {5901, 5901}, {5909, 5919}, {5943, 5951},
	{5972, 5983}, {5997, 5997}, {6001, 6001}, {6004, 6015}, {6110, 6111}, {6122, 6127}, {6138, 6143},
	{6159, 6159}, {6170, 6175}, {6265, 6271}, {6315, 6319}, {6390, 6399}, {6431, 6431}, {6444, 6447},
	{6460, 6463}, {6465, 6467}, {6510, 6511}, {6517, 6527}, {6572, 6575}, {6602, 6607}, {6619, 6621},
	{6684, 6685}, {6751, 6751}, {6781, 6782}, {6794, 6799}, {6810, 6815}, {6830, 6831}, {6849, 6911},
	{6988, 6991}, {7037, 7039}, {7156, 7163}, {7224, 7226}, {7242, 7244}, {7305, 7311}, {7355, 7356},
	{7368, 7375}, {7419, 7423}, {7674, 7674}, {7958, 7959}, {7966, 7967}, {8006, 8007}, {8014, 8015},
	{8024, 8024}, {8026, 8026}, {8028, 8028}, {8030, 8030}, {8062, 8063}, {8117, 8117}, {8133, 8133},
	{8148, 8149}, {8156, 8156}, {8176, 8177}, {8181, 8181}, {8191, 8191}, {8293, 8293}, {8306, 8307},
	{8335, 8335}, {8349, 8351}, {8384, 8399}, {8433, 8447}, {8588, 8591}, {8986, 8987}, {9001, 9002},
	{9193, 9196}, {9200, 9200}, {9203, 9203}, {9255, 9279}, {9291, 9311}, {9725, 9726}, {9748, 9749},
	{9800, 9811}, {9855, 9855}, {9875, 9875}, {9889, 9889}, {9898, 9899}, {9917, 9918}, {9924, 9925},
	{9934, 9934}, {9940, 9940}, {9962, 9962}, {9970, 9971}, {9973, 9973}, {9978, 9978}, {9981, 9981},
	{9989, 9989}, {9994, 9995}, {10024, 10024}, {10060, 10060}, {10062, 10062}, {10067, 10069},
	{10071, 10071}, {10133, 10135}, {10160, 10160}, {10175, 10175}, {11035, 11036}, {11088, 11088},
	{11093, 11093}, {11124, 11125}, {11158, 11158}, {11311, 11311}, {11359, 11359}, {11508, 11512},
	{11558, 11558}, {11560, 11564}, {11566, 11567}, {11624, 11630}, {11633, 11646}, {11671, 11679},
	{11687, 11687}, {11695, 11695}, {11703, 11703}, {11711, 11711}, {11719, 11719}, {11727, 11727},
	{11735, 11735}, {11743, 11743}, {11859, 12350}, {12352, 12871}, {12880, 19903}, {19968, 42191},
	{42540, 42559}, {42744, 42751}, {42944, 42945}, {42955, 42996}, {43053, 43055}, {43066, 43071},
	{43128, 43135}, {43206, 43213}, {43226, 43231}, {43348, 43358}, {43360, 43391}, {43470, 43470},
	{43482, 43485}, {43519, 43519}, {43575, 43583}, {43598, 43599}, {43610, 43611}, {43715, 43738},
	{43767, 43776}, {43783, 43784}, {43791, 43792}, {43799, 43807}, {43815, 43815}, {43823, 43823},
	{43884, 43887}, {44014, 44015}, {44026, 55215}, {55239, 55242}, {55292, 55295}, {63744, 64255},
	{64263, 64274}, {64280, 64284}, {64311, 64311}, {64317, 64317}, {64319, 64319}, {64322, 64322},
	{64325, 64325}, {64450, 64466}, {64832, 64847}, {64912, 64913}, {64968, 65007}, {65022, 65023},
	{65040, 65055}, {65072, 65135}, {65141, 65141}, {65277, 65278}, {65280, 65376}, {65471, 65473},
	{65480, 65481}, {65488, 65489}, {65496, 65497}, {65501, 65511}, {65519, 65528}, {65534, 65535},
	{65548, 65548}, {65575, 65575}, {65595, 65595}, {65598, 65598}, {65614, 65615}, {65630, 65663},
	{65787, 65791}, {65795, 65798}, {65844, 65846}, {65935, 65935}, {65949, 65951}, {65953, 65999},
	{66046, 66175}, {66205, 66207}, {66257, 66271}, {66300, 66303}, {66340, 66348}, {66379, 66383},
	{66427, 66431}, {66462, 66462}, {66500, 66503}, {66518, 66559}, {66718, 66719}, {66730, 66735},
	{66772, 66775}, {66812, 66815}, {66856, 66863}, {66916, 66926}, {66928, 67071}, {67383, 67391},
	{67414, 67423}, {67432, 67583}, {67590, 67591}, {67593, 67593}, {67638, 67638}, {67641, 67643},
	{67645, 67646}, {67670, 67670}, {67743, 67750}, {67760, 67807}, {67827, 67827}, {67830, 67834},
	{67868, 67870}, {67898, 67902}, {67904, 67967}, {68024, 68027}, {68048, 68049}, {68100, 68100},
	{68103, 68107}, {68116, 68116}, {68120, 68120}, {68150, 68151}, {68155, 68158}, {68169, 68175},
	{68185, 68191}, {68256, 68287}, {68327, 68330}, {68343, 68351}, {68406, 68408}, {68438, 68439},
	{68467, 68471}, {68498, 68504}, {68509, 68520}, {68528, 68607}, {68681, 68735}, {68787, 68799},
	{68851, 68857}, {68904, 68911}, {68922, 69215}, {69247, 69247}, {69290, 69290}, {69294, 69295},
	{69298, 69375}, {69416, 69423}, {69466, 69551}, {69580, 69599}, {69623, 69631}, {69710, 69713},
	{69744, 69758}, {69826, 69836}, {69838, 69839}, {69865, 69871}, {69882, 69887}, {69941, 69941},
	{69960, 69967}, {70007, 70015}, {70112, 70112}, {70133, 70143}, {70162, 70162}, {70207, 70271},
	{70279, 70279}, {70281, 70281}, {70286, 70286}, {70302, 70302}, {70314, 70319}, {70379, 70383},
	{70394, 70399}, {70404, 70404}, {70413, 70414}, {70417, 70418}, {70441, 70441}, {70449, 70449},
	{70452, 70452}, {70458, 70458}, {70469, 70470}, {70473, 70474}, {70478, 70479}, {70481, 70486},
	{70488, 70492}, {70500, 70501}, {70509, 70511}, {70517, 70655}, {70748, 70748}, {70754, 70783},
	{70856, 70863}, {70874, 71039}, {71094, 71095}, {71134, 71167}, {71237, 71247}, {71258, 71263},
	{71277, 71295}, {71353, 71359}, {71370, 71423}, {71451, 71452}, {71468, 71471}, {71488, 71679},
	{71740, 71839}, {71923, 71934}, {71943, 71944}, {71946, 71947}, {71956, 71956}, {71959, 71959},
	{71990, 71990}, {71993, 71994}, {72007, 72015}, {72026, 72095}, {72104, 72105}, {72152, 72153},
	{72165, 72191}, {72264, 72271}, {72355, 72383}, {72441, 72703}, {72713, 72713}, {72759, 72759},
	{72774, 72783}, {72813, 72815}, {72848, 72849}, {72872, 72872}, {72887, 72959}, {72967, 72967},
	{72970, 72970}, {73015, 73017}, {73019, 73019}, {73022, 73022}, {73032, 73039}, {73050, 73055},
	{73062, 73062}, {73065, 73065}, {73103, 73103}, {73106, 73106}, {73113, 73119}, {73130, 73439},
	{73465, 73647}, {73649, 73663}, {73714, 73726}, {74650, 74751}, {74863, 74863}, {74869, 74879},
	{75076, 77823}, {78895, 78895}, {78905, 82943}, {83527, 92159}, {92729, 92735}, {92767, 92767},
	{92778, 92781}, {92784, 92879}, {92910, 92911}, {92918, 92927}, {92998, 93007}, {93018, 93018},
	{93026, 93026}, {93048, 93052}, {93072, 93759}, {93851, 93951}, {94027, 94030}, {94088, 94094},
	{94112, 113663}, {113771, 113775}, {113789, 113791}, {113801, 113807}, {113818, 113819},
	{113828, 118783}, {119030, 119039}, {119079, 119080}, {119273, 119295}, {119366, 119519},
	{119540, 119551}, {119639, 119647}, {119673, 119807}, {119893, 119893}, {119965, 119965},
	{119968, 119969}, {119971, 119972}, {119975, 119976}, {119981, 119981}, {119994, 119994},
	{119996, 119996}, {120004, 120004}, {120070, 120070}, {120075, 120076}, {120085, 120085},
	{120093, 120093}, {120122, 120122}, {120127, 120127}, {120133, 120133}, {120135, 120137},
	{120145, 120145}, {120486, 120487}, {120780, 120781}, {121484, 121498}, {121504, 121504},
	{121520, 122879}, {122887, 122887}, {122905, 122906}, {122914, 122914}, {122917, 122917},
	{122923, 123135}, {123181, 123183}, {123198, 123199}, {123210, 123213}, {123216, 123583},
	{123642, 123646}, {123648, 124927}, {125125, 125126}, {125143, 125183}, {125260, 125263},
	{125274, 125277}, {125280, 126064}, {126133, 126208}, {126270, 126463}, {126468, 126468},
	{126496, 126496}, {126499, 126499}, {126501, 126502}, {126504, 126504}, {126515, 126515},
	{126520, 126520}, {126522, 126522}, {126524, 126529}, {126531, 126534}, {126536, 126536},
	{126538, 126538}, {126540, 126540}, {126544, 126544}, {126547, 126547}, {126549, 126550},
	{126552, 126552}, {126554, 126554}, {126556, 126556}, {126558, 126558}, {126560, 126560},
	{126563, 126563}, {126565, 126566}, {126571, 126571}, {126579, 126579}, {126584, 126584},
	{126589, 126589}, {126591, 126591}, {126602, 126602}, {126620, 126624}, {126628, 126628},
	{126634, 126634}, {126652, 126703}, {126706, 126975}, {126980, 126980}, {127020, 127023},
	{127124, 127135}, {127151, 127152}, {127168, 127168}, {127183, 127184}, {127222, 127231},
	{127374, 127374}, {127377, 127386}, {127406, 127461}, {127488, 127776}, {127789, 127797},
	{127799, 127868}, {127870, 127891}, {127904, 127946}, {127951, 127955}, {127968, 127984},
	{127988, 127988}, {127992, 128062}, {128064, 128064}, {128066, 128252}, {128255, 128317},
	{128331, 128334}, {128336, 128359}, {128378, 128378}, {128405, 128406}, {128420, 128420},
	{128507, 128591}, {128640, 128709}, {128716, 128716}, {128720, 128722}, {128725, 128735},
	{128747, 128751}, {128756, 128767}, {128884, 128895}, {128985, 129023}, {129036, 129039},
	{129096, 129103}, {129114, 129119}, {129160, 129167}, {129198, 129199}, {129202, 129279},
	{129292, 129338}, {129340, 129349}, {129351, 129535}, {129620, 129631}, {129646, 129791},
	{129939, 129939}, {129995, 130031}, {130042, 917504}, {917506, 917535}, {917632, 917759},
	{918000, 983039}, {1048574, 1048575}, {1114110, 1114111},
}

// ── 自检（成对负控 · 进程内版）─────────────────────────────────────────────────
// ★ py 侧通过 `sys.executable` re-exec 本件取「真命令行 + 真退出码」；Go 侧**改为进程内调用**
// 同一 core 入口（同 rc/stdout 捕获）⇒ **已声明的口径差**（自检手段差异），见未覆盖面。
// ★ py 的 `tempfile.mkdtemp` **不回收**（A 类真口子）；Go 侧 `defer os.RemoveAll` 自回收
// （输出不可见差异 · 跑后同前缀残留 delta=0）。
const cwsGateA = "#!/usr/bin/env bash\nset -u\n# 闸（夹具）\npython3 scripts/check-alpha.py --scope x\n"
const cwsGateB = "#!/usr/bin/env bash\npython3 scripts/check-beta.py\n"

type cwsCaseRes struct {
	label string
	ok    bool
	why   string
}

func cwsFixture(base, name string, files map[string]string) string {
	d := cwsPyJoin(base, name)
	for relp, content := range files {
		p := cwsPyJoin(d, relp)
		os.MkdirAll(cwsDirname(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	return d
}

func cwsCloneFiles(m map[string]string, over map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

func cwsGitInitAdd(dir string) {
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		_ = cmd.Run()
	}
}

func cwsRunCase(label, fixture string, wl []map[string]string, extra []string,
	expectRc int, must, mustNot []string, results *[]cwsCaseRes) string {
	argv := []string{"--root", fixture, "--no-self-test"}
	p := cwsPyJoin(cwsDirname(fixture), "_wl_"+cwsBase(fixture)+".json")
	b, _ := json.Marshal(wl)
	os.WriteFile(p, b, 0o644)
	argv = append(argv, "--whitelist", p)
	argv = append(argv, extra...)
	var buf bytes.Buffer
	rc := cwsRunMain(&buf, &buf, argv)
	out := buf.String()
	var bad []string
	if rc != expectRc {
		bad = append(bad, fmt.Sprintf("rc=%d（期望 %d）", rc, expectRc))
	}
	for _, m := range must {
		if !strings.Contains(out, m) {
			bad = append(bad, fmt.Sprintf("缺 %s", cwsRepr(m)))
		}
	}
	for _, m := range mustNot {
		if strings.Contains(out, m) {
			bad = append(bad, fmt.Sprintf("不该出现 %s", cwsRepr(m)))
		}
	}
	*results = append(*results, cwsCaseRes{label, len(bad) == 0, strings.Join(bad, "; ")})
	return out
}

func cwsSelftest(stdout io.Writer) int {
	base, err := os.MkdirTemp("", "wired-scripts-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "  ✗ 自检夹具目录建不出：%s ⇒ rc=2\n", err)
		return 2
	}
	defer os.RemoveAll(base)
	var results []cwsCaseRes
	wlOK := []map[string]string{{"name": "helper.py", "reason": "夹具：一次性", "date": "2026-09-18"}}

	okFiles := map[string]string{
		"scripts/gates/precommit-gates.sh":   cwsGateA,
		"scripts/build/publish-preflight.sh": cwsGateB,
		"scripts/check-alpha.py":             "# ck\n",
		"scripts/check-beta.py":              "# ck\n",
		"scripts/helper.py":                  "# zero-ref but whitelisted\n",
		"scripts/run-gates.sh": "#!/usr/bin/env bash\n" +
			"bash scripts/gates/precommit-gates.sh\n" +
			"bash scripts/build/publish-preflight.sh /tmp/out\n",
		"Makefile": "check:\n\tbash scripts/run-gates.sh\n",
		// 闸清单里每一条都必须「在位」（GitHub 发布闸也补上，与 py ok_files 同口径）
		"publish/mirror-public.sh":        "#!/usr/bin/env bash\n",
		"publish/mirror-public-lib.py":    "# lib\n",
		"publish/ci/ci.yml":               "name: ci\n",
		"publish/ci/release-agent.yml":    "name: rel\n",
		"publish/mirror-acceptance.sh":    "#!/usr/bin/env bash\n",
		"docs/项目文档/v9.9/债务台账-20990101.md": "helper.py 是零引用\n",
	}
	fOK := cwsFixture(base, "ok", okFiles)

	cwsRunCase("① 正控：全挂且白名单齐 ⇒ rc=0 且 0 命中", fOK, wlOK, nil,
		0, []string{"A 命中 0", "未登记 0"}, nil, &results)
	cwsRunCase("② 记录面不计引用（判据区分度）⇒ 零引用 1 只且已登记", fOK, wlOK, nil,
		0, []string{"零引用 1 只", "已登记（1 只）"}, []string{"零引用 0 只"}, &results)

	fNoA := cwsFixture(base, "neg-a", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/precommit-gates.sh": "#!/usr/bin/env bash\nset -u\necho 没有按名调用\n"}))
	cwsRunCase("③ 负控 A：拿掉 check-alpha 的接线 ⇒ 报出它，只报告档 rc=0",
		fNoA, wlOK, nil, 0, []string{"check-alpha.py", "A 命中 1"}, nil, &results)
	cwsRunCase("④ 负控 A'：同一夹具 + --strict-report ⇒ rc=1",
		fNoA, wlOK, []string{"--strict-report"}, 1, []string{"rc=1"}, nil, &results)

	fCmt := cwsFixture(base, "neg-a-cmt", cwsCloneFiles(okFiles, map[string]string{
		"scripts/check-gamma.py": "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA + "# python3 scripts/check-gamma.py\n" +
			"EXCLUDES=(\n  \"scripts/check-gamma.py\"\n)\n"}))
	cwsRunCase("⑤ 负控 A''：只在注释行/纯清单项里出现 ⇒ 仍算未挂（判据有牙齿）",
		fCmt, wlOK, nil, 0, []string{"check-gamma.py", "A 命中 1"}, nil, &results)

	cwsRunCase("⑥ 负控 B：零引用且未登记 ⇒ 报出它（只报告档 rc=0）",
		fOK, []map[string]string{}, nil, 0, []string{"未登记 1", "helper.py"}, nil, &results)
	cwsRunCase("⑦ 负控 B'：同夹具 + --strict-report ⇒ rc=1",
		fOK, []map[string]string{}, []string{"--strict-report"}, 1, []string{"rc=1"}, nil, &results)

	cwsRunCase("⑧a 白名单缺 reason ⇒ rc=2", fOK,
		[]map[string]string{{"name": "helper.py", "date": "2026-09-18"}}, nil, 2, []string{"缺 reason"}, nil, &results)
	cwsRunCase("⑧b 白名单缺 date ⇒ rc=2", fOK,
		[]map[string]string{{"name": "helper.py", "reason": "夹具"}}, nil, 2, []string{"缺 date"}, nil, &results)
	cwsRunCase("⑧c 白名单 date 形态不对 ⇒ rc=2", fOK,
		[]map[string]string{{"name": "helper.py", "reason": "夹具", "date": "09/18/2026"}}, nil, 2,
		[]string{"date 形态不对"}, nil, &results)
	cwsRunCase("⑧d 白名单悬空（点名的脚本不存在）⇒ rc=2", fOK,
		append(append([]map[string]string{}, wlOK...),
			map[string]string{"name": "ghost.py", "reason": "夹具", "date": "2026-09-18"}), nil, 2,
		[]string{"悬空"}, nil, &results)

	fEmpty := cwsFixture(base, "empty", map[string]string{
		"publish/mirror-public.sh": "#!/usr/bin/env bash\n",
		"scripts/tool-x.md": "夹具占位：scripts/ 目录在、里面一只**脚本**都没有" +
			"（.md 不进候选面）\n"})
	cwsRunCase("⑨a 空转：候选 0 只 ⇒ rc=2", fEmpty, []map[string]string{}, nil, 2,
		[]string{"空转", "rc=2"}, nil, &results)

	fNoGate := cwsFixture(base, "no-gate", map[string]string{
		"scripts/check-solo.py": "# ck\n",
		"README.md":             "x\n",
	})
	cwsRunCase("⑩ 缺件：闸清单一个都不在位 ⇒ rc=2（不给结论）", fNoGate, []map[string]string{}, nil, 2,
		[]string{"不给结论", "rc=2"}, nil, &results)

	// ⑩b 负控（与 ⑩ 成对 · GAP-20260927-154）：闸清单**只缺一条**（非全缺）⇒ 也必须 rc=2。
	//   旧口径 nGatePresent == 0 ⇒ 缺 1~5 条恒绿（改名/搬走一条闸即静默）⇒ 这条正是它的牙。
	fPartial := cwsFixture(base, "one-gate-missing", map[string]string{
		"scripts/gates/precommit-gates.sh":   cwsGateA,
		"scripts/build/publish-preflight.sh": cwsGateB,
		"publish/mirror-public.sh":           "#!/usr/bin/env bash\n",
		"publish/mirror-public-lib.py":       "# lib\n",
		"publish/ci/ci.yml":                  "name: ci\n",
		"scripts/check-solo.py":              "# ck\n",
	})
	cwsRunCase("⑩b 闸清单只缺 1 条（非全缺）⇒ 必 rc=2 并点名缺件", fPartial, []map[string]string{}, nil, 2,
		[]string{"不给结论", "缺件", "release-agent.yml"}, nil, &results)

	fDecl := cwsFixture(base, "decl", cwsCloneFiles(okFiles, map[string]string{
		"scripts/tool-x.sh": "#!/usr/bin/env bash\necho x\n",
		"scripts/tool-x.md": "# tool-x — 本门说明书\n\n写盘门。\n",
	}))
	cwsRunCase("⑪ 说明书自称门（同干 .md 头部含「门」）⇒ 成为候选并报未挂",
		fDecl, wlOK, nil, 0, []string{"tool-x.sh", "说明书自称门"}, nil, &results)

	fNoDecl := cwsFixture(base, "nodecl", cwsCloneFiles(okFiles, map[string]string{
		"scripts/tool-y.sh": "#!/usr/bin/env bash\necho y\n",
		"scripts/tool-y.md": "# tool-y —— 一个普通工具\n\n没什么特别的。\n",
		"scripts/run-gates.sh": "#!/usr/bin/env bash\n" +
			"bash scripts/tool-y.sh\n" +
			"bash scripts/gates/precommit-gates.sh\n" +
			"bash scripts/build/publish-preflight.sh /tmp/out\n",
	}))
	cwsRunCase("⑫ 说明书不提「门」⇒ 不成为候选（区分度）",
		fNoDecl, wlOK, nil, 0, []string{"A 命中 0"}, []string{"tool-y.sh"}, &results)

	var buf bytes.Buffer
	rc13 := cwsRunMain(&buf, &buf, []string{"--list-rules"})
	o13 := buf.String()
	var bad13 []string
	if rc13 != 0 {
		bad13 = append(bad13, fmt.Sprintf("rc=%d", rc13))
	}
	for _, m := range []string{"断言 A", "断言 B", "precommit-gates.sh", "publish-preflight.sh", "升级路径", "白名单"} {
		if !strings.Contains(o13, m) {
			bad13 = append(bad13, fmt.Sprintf("缺 %s", cwsRepr(m)))
		}
	}
	results = append(results, cwsCaseRes{"⑬ --list-rules 可打印判据/闸清单/升级路径", len(bad13) == 0, strings.Join(bad13, "; ")})

	fLi := cwsFixture(base, "neg-a-list", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-delta.py": "# ck\n",
		"publish/mirror-public-lib.py": "DROP_EXACT = (\n    \"scripts/gates/check-delta.py\",   # 公开面排除表\n)\n",
	}))
	cwsRunCase("⑭ 负控 A3: 只被排除表清单项点名（多段路径）⇒ 仍算未挂（判据有牙齿）",
		fLi, wlOK, nil, 0, []string{"check-delta.py", "A 命中 1"}, nil, &results)

	fCall := cwsFixture(base, "pos-call", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-epsilon.py": "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA +
			"python3 \"$Z/scripts/gates/check-epsilon.py\" \"$D\" > \"$D/x.log\" 2>&1\n" +
			"echo \"   ⑧c ── 已接线（按名调用 check-epsilon.py）\"\n"}))
	cwsRunCase("⑮ 正控：命令位调用（引号 + 变量前缀）⇒ 已挂（A 命中 0）",
		fCall, wlOK, nil, 0, []string{"A 命中 0", "已挂门脚本 3 只"}, []string{"0 处"}, &results)

	fEcho := cwsFixture(base, "neg-a-echo", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-zeta.py": "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA +
			"echo \"   ⑧d 成对闸 ── 按名调用 check-zeta.py · 明细 /tmp/x.log\"\n"}))
	cwsRunCase("⑯ 负控 A4: 只在文案里被提到（0 次命令位调用）⇒ 仍算未挂",
		fEcho, wlOK, nil, 0, []string{"check-zeta.py", "A 命中 1"}, nil, &results)

	// ── ⑰/⑱ 候选面换 fs、引用面不换（`GAP-20260927-159/169`）成对负控 ────────────────
	//   ⑰ 正控：git 夹具 + 一个**未跟踪**的新 check- 脚本 ⇒ 必进候选面并被断言 A 报出。
	fUnt := cwsFixture(base, "untracked-cand", cwsCloneFiles(okFiles, nil))
	cwsGitInitAdd(fUnt)
	os.WriteFile(cwsPyJoin(fUnt, "scripts", "check-untracked.py"), []byte("# ck（未跟踪新件）\n"), 0o644)
	cwsRunCase("⑰ 未跟踪的新门脚本必进候选面（候选面换 fs · 索引只分档）",
		fUnt, wlOK, nil, 0, []string{"check-untracked.py", "A 命中 1"}, []string{"A 命中 0"}, &results)
	//   ⑱ 单元负控（与 ⑰ 成对 · 方向性）：非 git 档原样返回（换面只写在候选面这一半）。
	cs18 := cwsCandidateFiles(base, []string{"a.py"}, "文件系统遍历（非 git 目录）")
	ok18 := len(cs18) == 1 && cs18[0] == "a.py"
	results = append(results, cwsCaseRes{"⑱ candidate_files 非 git 档原样返回（换面只在候选面这一半）", ok18, ""})

	// ── ⑲/⑲' 记录类表不得充作引用（`GAP-20260927-139`）成对负控 ─────────────────────
	fMark := cwsFixture(base, "neg-record-marks", cwsCloneFiles(okFiles, map[string]string{
		"scripts/check-omega.py": "# ck\n",
		"scripts/公开标记.tsv":       "scripts/check-omega.py	public=no\n",
	}))
	cwsRunCase("⑲ 只被记录类表（公开标记.tsv）点名 ⇒ 仍判「未登记」（记录不能充作引用）",
		fMark, wlOK, nil, 0, []string{"check-omega.py", "未登记 1"}, []string{"未登记 0"}, &results)
	cwsRunCase("⑲' 同上 + --strict-report ⇒ rc=1（记录类引用不成牙）",
		fMark, wlOK, []string{"--strict-report"}, 1, []string{"rc=1"}, nil, &results)

	// ── ⑳/㉑ `zerg gate check <判据名>` 接线形（`GAP-20261001-74` · 与 cwsZergGateRe 成对）──
	zrgRef := "#!/usr/bin/env bash\n" +
		"bash scripts/gates/precommit-gates.sh\n" +
		"bash scripts/build/publish-preflight.sh /tmp/out\n" +
		"python3 scripts/check-eta.py\n"
	fZG := cwsFixture(base, "pos-zerg", cwsCloneFiles(okFiles, map[string]string{
		"scripts/check-eta.py": "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA +
			"add_step g \"门X\" tri \"$R\" \"zerg gate check check-eta\"\n",
		"scripts/run-gates.sh": zrgRef,
	}))
	cwsRunCase("⑳ 正控：`zerg gate check check-eta` 命令位 ⇒ 已挂（A 命中 0）",
		fZG, wlOK, nil, 0, []string{"A 命中 0", "已挂门脚本 3 只"}, []string{"0 处"}, &results)
	fNG := cwsFixture(base, "neg-zerg-subcmd", cwsCloneFiles(okFiles, map[string]string{
		"scripts/check-eta.py": "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA +
			"add_step g \"门X\" tri \"$R\" \"zerg check check-eta\"\n",
		"scripts/run-gates.sh": zrgRef,
	}))
	cwsRunCase("㉑ 负控：`zerg check check-eta`（缺 gate 子命令）⇒ 仍算未挂（防「只加 zerg 就放行」）",
		fNG, wlOK, nil, 0, []string{"check-eta.py", "A 命中 1"}, nil, &results)

	// ── 断言 C：件头自述三态（首版 UNSTATED-only · 显式 token `# zerg-wire:`）成对负控 ──
	//   ★ token 形写死：行首 `#` ＋ `zerg-wire:` ＋ <WIRED|UNWIRED|UNSTATED>；件头前 40 行、全件唯一。
	//   C1（= M2 方向）：件头 `WIRED` 但闸里查无此件 ⇒ **失真**（默认档只报，--strict-report 下 rc=1）。
	fC1 := cwsFixture(base, "c-wired-notwired", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-iota.py": "# zerg-wire: WIRED\n# ck\n",
		"scripts/run-gates.sh": "#!/usr/bin/env bash\n" +
			"bash scripts/gates/precommit-gates.sh\n" +
			"bash scripts/build/publish-preflight.sh /tmp/out\n" +
			"python3 scripts/gates/check-iota.py\n",
	}))
	cwsRunCase("C1 负控（M2 方向）：件头 WIRED 但闸清单查无此件 ⇒ 报失真（默认档 rc=0 不转红）",
		fC1, wlOK, nil, 0, []string{"失真 1", "check-iota.py"}, nil, &results)
	cwsRunCase("C1' 同上 + --strict-report ⇒ rc=1（失真成牙）",
		fC1, wlOK, []string{"--strict-report"}, 1, []string{"rc=1"}, nil, &results)

	//   C2（= M1 方向 · 与 C1 反向）：件头 `UNWIRED` 但闸里**已**按名调用 ⇒ **失真**（A 命中 0，
	//     这正是断言 C 独有、断言 A 结构上抓不到的一档）。
	fC2 := cwsFixture(base, "c-unwired-butwired", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-kappa.py":     "# zerg-wire: UNWIRED\n# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA + "python3 scripts/gates/check-kappa.py\n",
	}))
	cwsRunCase("C2 负控（M1 方向）：件头 UNWIRED 但闸里已按名调用 ⇒ 报失真（A 命中 0 · 默认档 rc=0）",
		fC2, wlOK, nil, 0, []string{"失真 1", "check-kappa.py", "A 命中 0"}, nil, &results)
	cwsRunCase("C2' 同上 + --strict-report ⇒ rc=1",
		fC2, wlOK, []string{"--strict-report"}, 1, []string{"rc=1"}, nil, &results)

	//   C3（= M3）：件头**无** token ⇒ 只计入 UNSTATED、**不判红**（默认档与 --strict-report 均 rc=0）。
	//     件**已**接线（闸里按名调用、A 命中 0）—— 这样 --strict-report 下 rc 只由 C 决定，才测得准。
	fC3 := cwsFixture(base, "c-unstated", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-lambda.py":    "# ck\n",
		"scripts/gates/precommit-gates.sh": cwsGateA + "python3 scripts/gates/check-lambda.py\n",
	}))
	cwsRunCase("C3 正控（M3）：件头无 token ⇒ 只计入 UNSTATED，不判红（默认档 rc=0）",
		fC3, wlOK, nil, 0, []string{"UNSTATED 3", "check-lambda.py", "无失真"}, nil, &results)
	cwsRunCase("C3' 同上 + --strict-report ⇒ 仍 rc=0（UNSTATED 不锁门）",
		fC3, wlOK, []string{"--strict-report"}, 0, []string{"rc=0"}, nil, &results)

	//   C4（= M4 歧义）：件头两行 token（WIRED + UNWIRED）⇒ 判 UNSTATED 并打印（✗ 不猜）。
	fC4 := cwsFixture(base, "c-ambiguous", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-mu.py": "# zerg-wire: WIRED\n# zerg-wire: UNWIRED\n# ck\n",
		"scripts/run-gates.sh": "#!/usr/bin/env bash\n" +
			"bash scripts/gates/precommit-gates.sh\n" +
			"bash scripts/build/publish-preflight.sh /tmp/out\n" +
			"python3 scripts/gates/check-mu.py\n",
	}))
	cwsRunCase("C4 负控（M4 歧义）：件头两行 token ⇒ 判 UNSTATED 并打印（✗ 不猜 · 失真 0）",
		fC4, wlOK, nil, 0, []string{"token 多重", "check-mu.py", "无失真"}, nil, &results)

	//   C5（= M5 词面碰撞回归）：件头含自然语言 `不挂死` / `只报告档起步` 但**无** token ⇒ UNSTATED
	//     （专治设计稿 §1.6 噪声：按词面抽会把 `不挂死` 读成「未接线」）。
	fC5 := cwsFixture(base, "c-collision", cwsCloneFiles(okFiles, map[string]string{
		"scripts/gates/check-nu.py": "# ck\n" +
			"# 本件不挂死：无器件即立即失败、不卡住\n" +
			"# 起步只报告档起步（历史档位，与「未接线」无关）\n",
		"scripts/run-gates.sh": "#!/usr/bin/env bash\n" +
			"bash scripts/gates/precommit-gates.sh\n" +
			"bash scripts/build/publish-preflight.sh /tmp/out\n" +
			"python3 scripts/gates/check-nu.py\n",
	}))
	cwsRunCase("C5 负控（M5 词面碰撞回归）：件头含 `不挂死`/`只报告档起步` 但无 token ⇒ UNSTATED（✗ 不被读成 UNWIRED）",
		fC5, wlOK, nil, 0, []string{"UNSTATED 3", "check-nu.py"}, []string{"UNWIRED 1"}, &results)

	npass := 0
	for _, r := range results {
		if r.ok {
			npass++
		}
	}
	fmt.Fprintf(stdout, "门③ --self-test（成对负控 · 真命令行 + 真退出码）：%d/%d 通过\n", npass, len(results))
	for _, r := range results {
		mark := "✓"
		extra := ""
		if !r.ok {
			mark = "✗"
			extra = "   ← " + r.why
		}
		fmt.Fprintf(stdout, "  %s %s%s\n", mark, r.label, extra)
	}
	fmt.Fprintf(stdout, "  夹具目录：%s\n", base)
	if npass != len(results) {
		fmt.Fprintln(stdout, "  ✗ 自检不过 ⇒ 拒绝扫真目标（rc=2）")
		return 2
	}
	return 0
}
