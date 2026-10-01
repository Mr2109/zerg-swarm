// check_build_tags.go —— 门禁件 `scripts/gates/check-build-tags.py`（863 行）的 Go 真搬（M2）。
//
// 判据面（照 py 逐字）：a1/a2/a3/a4（tag 命名）· b（默认/调试两个构建都过）· c（vet 成对）
// · d（导出面奇偶）· e（GOFLAGS 断言）· Q-176（--json 无机器读面 ⇒ rc=2）。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func checkBuildTagsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全绿 / 1 有 FAIL 项 / 2 用法错·前置缺件·自检不过·判不了（不给结论）
//
// ── 两条出厂声明 ─────────────────────────────────────────────────────────────
// (a) 默认根定位差异：py 由 `__file__` 上溯两级得 REPO_ROOT（脚本位置决定）、助手目录由
//
//	`dirname(__file__)/exportnames` 推出；Go 无 `__file__` 语境 ⇒ 缺省根与助手目录由
//	**当前工作目录上溯**找到含 `scripts/exportnames/go.mod` 的目录（仓根）推出（再退回
//	可执行文件目录上溯）⇒ 「脚本位置 ≠ cwd」时路径推导面与 py 不同（✗ 不声称逐字对齐）。
//	对拍两侧一律在仓根启动，故此差异在当前对拍面上不可观测。
//
// (b) 自指检查：本件落在**自己扫描的面**内（a1/a4 逐模块扫 .go 文件）。
//
//		· 本件首 30 行内零「构建约束行」；正文里的夹具内容一律写成「双引号 + 转义串」形态
//		  ⇒ 约束形态前总有 `"` ⇒ 解析面命中 0；
//	    · a3 只扫 SCAN_EXTS（`.go` 不在内）；自指排除面 = py 原件 `scripts/gates/check-build-tags.py`
//	      （py 侧 SELF_PATH）＋ 本件 ⇒ 本件与 py 原件命中 0；
//		· 可观测变化仅 a1/a2/a4 的 note 行「N 个 .go 文件」计数 +2（本件 + 其测试件）。
//
// 读数见回执 selfref 面。
//
// 不写盘（除临时目录：构建产物落点 MkdirTemp + 退出即 RemoveAll）；不联网；不起后台进程。
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
	"unicode/utf8"
)

// ══════════════════════════════════════════════════════════════════
// 门禁契约（改这里 = 改契约）
// ══════════════════════════════════════════════════════════════════

var cbtAllowedCustomTags = []string{"debug"}

var cbtExportParityPkgs = []string{
	"core/internal/toolobs",
	"core/internal/infergeom",
	"core/internal/obs/redact",
}

var cbtFixedReserved = []string{
	"unix", "cgo", "gc", "gccgo", "ignore",
	"race", "msan", "asan",
	"boringcrypto", "openssl", "purego", "osusergo", "netgo", "fuzz",
}

var cbtSkipDirs = map[string]bool{
	"vendor": true, ".git": true, ".zerg": true, ".hermes": true,
	"testdata": true, "node_modules": true, "target": true, "dist": true,
	"bin": true, "data": true, "docs": true, "venv": true, ".venv": true,
}

const (
	cbtMaxModuleDepth = 2
	cbtBuildTimeout   = 900 * time.Second
	cbtTailLines      = 15
)

var cbtScanExts = []string{".sh", ".bash", ".yml", ".yaml", ".py", ".mk", ".toml", ".cfg", ".conf"}

var cbtCommandHints = []string{"go ", "go\t", "/go", "golangci", "build", "vet", "test", "list", "cmd/"}

const cbtTagValueTrim = ";)]}'\""

// py `\s` 的 Unicode 语义（含 \x1c-\x1f、\x85、\xa0 等）——Go RE2 `\s` 只认 ASCII ⇒ 显式展开
const cbtPySpaceClass = `\t\n\v\f\r\x{1c}\x{1d}\x{1e}\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000} `

var (
	cbtReleaseTagRe = regexp.MustCompile(`^go1\.\p{Nd}+$`)
	cbtBuildRe      = regexp.MustCompile(`^[` + cbtPySpaceClass + `]*//go:build[` + cbtPySpaceClass + `]+(.*)`)
	cbtLegacyRe     = regexp.MustCompile(`^[` + cbtPySpaceClass + `]*//[` + cbtPySpaceClass + `]*\+build[` + cbtPySpaceClass + `]+(.*)`)
	cbtTokenSplitRe = regexp.MustCompile(`[` + cbtPySpaceClass + `()]+`)
	cbtModuleLineRe = regexp.MustCompile(`^module[` + cbtPySpaceClass + `]+([^` + cbtPySpaceClass + `]+)[` + cbtPySpaceClass + `]*$`)
	cbtTagArgRe1    = regexp.MustCompile(`-{1,2}(?:build-)?tags=([^` + cbtPySpaceClass + `"'\x60]+)`)
	cbtTagArgRe2    = regexp.MustCompile(`-{1,2}(?:build-)?tags[` + cbtPySpaceClass + `]+([^` + cbtPySpaceClass + `"'\x60]+)`)
	cbtTagValueRe   = regexp.MustCompile(`^[A-Za-z0-9_.+\-]+$`)
	cbtGoflagsTagRe = regexp.MustCompile(`(?:^|[` + cbtPySpaceClass + `])-{1,2}(?:build-)?tags(?:[` + cbtPySpaceClass + `]+|=)`)
)

// ══════════════════════════════════════════════════════════════════
// 报告器
// ══════════════════════════════════════════════════════════════════

type cbtFinding struct{ judgement, text string }

type cbtReport struct {
	findings    []cbtFinding
	undecidable []string
	notes       []string
}

func (r *cbtReport) fail(j, t string) { r.findings = append(r.findings, cbtFinding{j, t}) }
func (r *cbtReport) cannot(t string)  { r.undecidable = append(r.undecidable, t) }
func (r *cbtReport) note(t string)    { r.notes = append(r.notes, t) }
func (r *cbtReport) rc() int {
	if len(r.findings) > 0 {
		return 1
	}
	if len(r.undecidable) > 0 {
		return 2
	}
	return 0
}

type cbtSink struct{ out, err io.Writer }

func (s cbtSink) p(f string, a ...any)  { fmt.Fprintf(s.out, f+"\n", a...) }
func (s cbtSink) pe(f string, a ...any) { fmt.Fprintf(s.err, f+"\n", a...) }

// ══════════════════════════════════════════════════════════════════
// 仓根 / 助手目录（出厂声明 (a)）
// ══════════════════════════════════════════════════════════════════

var (
	cbtRepoRootResolved  string
	cbtHelperDirResolved string
	cbtHomeResolved      bool
)

func cbtResolveHomeOnce() {
	if cbtHomeResolved {
		return
	}
	cbtHomeResolved = true
	try := func(start string) bool {
		d := start
		for {
			if fi, err := os.Stat(cbtPyJoin(d, "scripts", "exportnames", "go.mod")); err == nil && !fi.IsDir() {
				cbtRepoRootResolved = d
				cbtHelperDirResolved = cbtPyJoin(d, "scripts", "exportnames")
				return true
			}
			parent := cbtPyDirname(d)
			if parent == d || parent == "" {
				return false
			}
			d = parent
		}
	}
	if try(cbtPyGetcwd()) {
		return
	}
	if exe, err := os.Executable(); err == nil {
		_ = try(cbtPyDirname(exe))
	}
}

func cbtRepoRootDefault() string {
	if cbtRepoRootResolved != "" {
		return cbtRepoRootResolved
	}
	return cbtPyGetcwd()
}

// ══════════════════════════════════════════════════════════════════
// CPython posixpath 同构（✗ 不用 filepath.Join/Abs/Rel/Clean）
// ══════════════════════════════════════════════════════════════════

func cbtPyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
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

func cbtPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" {
		if trimmed := strings.TrimRight(head, "/"); trimmed != "" {
			head = trimmed
		}
	}
	return head, tail
}

func cbtPyDirname(p string) string  { h, _ := cbtPySplit(p); return h }
func cbtPyBasename(p string) string { _, t := cbtPySplit(p); return t }

func cbtPyNormpath(path string) string {
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
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	p := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		p = strings.Repeat("/", initialSlashes) + p
	}
	if p == "" {
		return "."
	}
	return p
}

func cbtPyGetcwd() string {
	if d, err := os.Getwd(); err == nil {
		return d
	}
	return "/"
}

func cbtPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = cbtPyJoin(cbtPyGetcwd(), p)
	}
	return cbtPyNormpath(p)
}

func cbtPyRelpath(path, start string) string {
	if path == "" {
		path = "."
	}
	if start == "" {
		start = "."
	}
	startAbs := cbtPyAbspath(start)
	pathAbs := cbtPyAbspath(path)
	var startList, pathList []string
	for _, x := range strings.Split(startAbs, "/") {
		if x != "" {
			startList = append(startList, x)
		}
	}
	for _, x := range strings.Split(pathAbs, "/") {
		if x != "" {
			pathList = append(pathList, x)
		}
	}
	m := len(startList)
	if len(pathList) < m {
		m = len(pathList)
	}
	i := 0
	for i = 0; i < m; i++ {
		if startList[i] != pathList[i] {
			break
		}
	}
	if m == 0 {
		i = 1
	}
	var relList []string
	for j := i; j < len(startList); j++ {
		relList = append(relList, "..")
	}
	relList = append(relList, pathList[i:]...)
	if len(relList) == 0 {
		return "."
	}
	return cbtPyJoin(relList[0], relList[1:]...)
}

// ══════════════════════════════════════════════════════════════════
// py 字符串原语同构
// ══════════════════════════════════════════════════════════════════

func cbtPyIsSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func cbtPyStrip(s string) string  { return strings.TrimFunc(s, cbtPyIsSpace) }
func cbtPyRStrip(s string) string { return strings.TrimRightFunc(s, cbtPyIsSpace) }

func cbtIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// py str.splitlines()
func cbtPySplitLines(s string) []string {
	var out []string
	start, i := 0, 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if cbtIsLineBoundary(r) {
			out = append(out, s[start:i])
			if r == '\r' && i+sz < len(s) && s[i+sz] == '\n' {
				i += sz + 1
			} else {
				i += sz
			}
			start = i
			continue
		}
		i += sz
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// py str.split()（按空白串切、丢空段）
func cbtPySplitWS(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if cbtPyIsSpace(r) {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func cbtPyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = "\""
	}
	body := strings.ReplaceAll(s, "\\", "\\\\")
	body = strings.ReplaceAll(body, q, "\\"+q)
	return q + body + q
}

func cbtPyListRepr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, cbtPyRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func cbtUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// py open(...).readlines()（通用换行 + 行尾保留）
func cbtReadLines(path string, replaceErr bool) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	s := string(b)
	if !utf8.ValidString(s) {
		if !replaceErr {
			return nil, false
		}
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	s = cbtUniversalNewlines(s)
	if s == "" {
		return nil, true
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out, true
}

// ══════════════════════════════════════════════════════════════════
// 小工具
// ══════════════════════════════════════════════════════════════════

func cbtContainsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func cbtSortedStrs(xs []string) []string {
	out := append([]string{}, xs...)
	sort.Strings(out)
	return out
}

func cbtSortedKeysBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cbtSortedKeysSlice(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cbtSortedKeysTup(m map[string][2]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cbtUniqSorted(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func cbtAnyOther(m map[string]bool, t string) bool {
	for k := range m {
		if k != t {
			return true
		}
	}
	return false
}

func cbtHasScanExt(fn string) bool {
	for _, e := range cbtScanExts {
		if strings.HasSuffix(fn, e) {
			return true
		}
	}
	return false
}

func cbtRCStr(rc int, ok bool) string {
	if !ok {
		return "None"
	}
	return strconv.Itoa(rc)
}

// py 真值判定（`if r.get('err')` / `or []` 一类）
func cbtPyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		return err == nil && f != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func cbtAnyList(v any) []string {
	lst, isList := v.([]any)
	if !isList {
		return nil
	}
	out := make([]string, 0, len(lst))
	for _, e := range lst {
		out = append(out, cbtAnyStr(e))
	}
	return out
}

// py `%s` 对 helper 值的语义：None→"None" · int→十进制 · list→repr
func cbtAnyStr(v any) string {
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
	case []any:
		ss := make([]string, 0, len(x))
		for _, e := range x {
			ss = append(ss, cbtAnyRepr(e))
		}
		return "[" + strings.Join(ss, ", ") + "]"
	default:
		return fmt.Sprintf("%v", x)
	}
}

func cbtAnyRepr(v any) string {
	if s, isStr := v.(string); isStr {
		return cbtPyRepr(s)
	}
	return cbtAnyStr(v)
}

// ══════════════════════════════════════════════════════════════════
// 进程 / 环境
// ══════════════════════════════════════════════════════════════════

func cbtGoBin() string {
	p, err := exec.LookPath("go")
	if err != nil || p == "" {
		return ""
	}
	return p
}

// 所有 go 调用的环境：显式覆盖 GOFLAGS（A6：绝不靠环境继承）
func cbtTagEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOFLAGS=") {
			out = append(out, "GOFLAGS=")
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "GOFLAGS=")
	}
	return out
}

// 返回 (rc, ok, out)。ok=false 表示 rc 取不到（超时 / 起不来）⇒ 判不了。
func cbtRun(argv []string, cwd string, env []string, stdinData *string, timeout time.Duration) (int, bool, string) {
	var buf bytes.Buffer
	if len(argv) == 0 {
		return 0, false, "!! 起不来:  ()"
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	if stdinData != nil {
		cmd.Stdin = strings.NewReader(*stdinData)
	}
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return 0, false, "!! 起不来: " + strings.Join(argv, " ") + " (" + err.Error() + ")"
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return cmd.ProcessState.ExitCode(), true, buf.String()
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return 0, false, buf.String() + "\n!! 超时（>" + strconv.Itoa(int(timeout.Seconds())) + "s）⇒ 判不了，不给结论"
	}
}

func cbtTail(text string, n int) string {
	var lines []string
	for _, ln := range cbtPySplitLines(text) {
		if cbtPyStrip(ln) != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ══════════════════════════════════════════════════════════════════
// 模块与保留名
// ══════════════════════════════════════════════════════════════════

type cbtModule struct{ path, dir string }

func cbtDistPlatforms(goBin string) (map[string]bool, map[string]bool, string, bool) {
	rc, ok, out := cbtRun([]string{goBin, "tool", "dist", "list"}, cbtRepoRootDefault(), cbtTagEnv(), nil, 120*time.Second)
	if !ok || rc != 0 {
		return nil, nil, out, false
	}
	goos := map[string]bool{}
	goarch := map[string]bool{}
	for _, ln := range cbtPySplitLines(out) {
		ln = cbtPyStrip(ln)
		if ln == "" || !strings.Contains(ln, "/") {
			continue
		}
		idx := strings.Index(ln, "/")
		goos[ln[:idx]] = true
		goarch[ln[idx+1:]] = true
	}
	if len(goos) == 0 || len(goarch) == 0 {
		return nil, nil, "go tool dist list 输出为空", false
	}
	return goos, goarch, "", true
}

func cbtReservedNames(goos, goarch map[string]bool) map[string]bool {
	s := map[string]bool{}
	for k := range goos {
		s[k] = true
	}
	for k := range goarch {
		s[k] = true
	}
	for _, k := range cbtFixedReserved {
		s[k] = true
	}
	return s
}

func cbtIsReserved(tag string, reserved map[string]bool) bool {
	return reserved[tag] || cbtReleaseTagRe.MatchString(tag)
}

type cbtWalkCb func(dir string, dirs []string, files []string) []string

func cbtWalk(root string, cb cbtWalkCb) {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return
	}
	var rec func(dir string)
	rec = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, _ := f.ReadDir(-1)
		f.Close()
		var dirs, files []string
		for _, e := range ents {
			p := cbtPyJoin(dir, e.Name())
			if fi2, err := os.Stat(p); err == nil && fi2.IsDir() {
				dirs = append(dirs, e.Name())
			} else {
				files = append(files, e.Name())
			}
		}
		sort.Strings(dirs)
		sort.Strings(files)
		nd := cb(dir, dirs, files)
		for _, dn := range nd {
			p := cbtPyJoin(dir, dn)
			if fi3, err := os.Lstat(p); err == nil && fi3.Mode()&os.ModeSymlink != 0 {
				continue
			}
			rec(p)
		}
	}
	rec(root)
}

func cbtDiscoverModules(root string) []cbtModule {
	root = cbtPyAbspath(root)
	var out []cbtModule
	cbtWalk(root, func(dir string, dirs []string, files []string) []string {
		rel := cbtPyRelpath(dir, root)
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, "/") + 1
		}
		var nd []string
		for _, d := range dirs {
			if !cbtSkipDirs[d] {
				nd = append(nd, d)
			}
		}
		if depth > cbtMaxModuleDepth {
			return []string{}
		}
		if cbtContainsStr(files, "go.mod") {
			if mp := cbtModulePath(cbtPyJoin(dir, "go.mod")); mp != "" {
				out = append(out, cbtModule{mp, dir})
			}
			return []string{}
		}
		return nd
	})
	return out
}

func cbtModulePath(goMod string) string {
	lines, ok := cbtReadLines(goMod, false)
	if !ok {
		return ""
	}
	for _, ln := range lines {
		if m := cbtModuleLineRe.FindStringSubmatch(cbtPyStrip(ln)); m != nil {
			return m[1]
		}
	}
	return ""
}

func cbtModuleOf(mods []cbtModule, path string) (cbtModule, bool) {
	var best cbtModule
	found := false
	for _, m := range mods {
		if path == m.dir || strings.HasPrefix(path, m.dir+"/") {
			if !found || len(m.dir) > len(best.dir) {
				best = m
				found = true
			}
		}
	}
	return best, found
}

func cbtScanGoFiles(moduleDir string) []string {
	var files []string
	cbtWalk(moduleDir, func(dir string, dirs []string, fls []string) []string {
		var nd []string
		for _, d := range dirs {
			if !cbtSkipDirs[d] {
				nd = append(nd, d)
			}
		}
		if dir != moduleDir && cbtContainsStr(fls, "go.mod") {
			return []string{}
		}
		for _, fn := range fls {
			if strings.HasSuffix(fn, ".go") {
				files = append(files, cbtPyJoin(dir, fn))
			}
		}
		return nd
	})
	sort.Strings(files)
	return files
}

// ══════════════════════════════════════════════════════════════════
// 构建约束解析
// ══════════════════════════════════════════════════════════════════

func cbtParseConstraints(path string) (map[string]bool, map[string]bool, bool) {
	lines, ok := cbtReadLines(path, true)
	if !ok {
		return nil, nil, false
	}
	pos := map[string]bool{}
	neg := map[string]bool{}
	lim := len(lines)
	if lim > 30 {
		lim = 30
	}
	for i := 0; i < lim; i++ {
		ln := lines[i]
		if m := cbtBuildRe.FindStringSubmatch(ln); m != nil {
			expr := m[1]
			j := i
			for cbtHasContinuation(expr) && j+1 < len(lines) {
				j++
				expr += " " + cbtPyStrip(lines[j])
			}
			for _, tok := range cbtTokenSplitRe.Split(expr, -1) {
				if tok == "" || tok == "&&" || tok == "||" || tok == "!" {
					continue
				}
				if strings.HasPrefix(tok, "!") {
					neg[tok[1:]] = true
				} else {
					pos[tok] = true
				}
			}
			continue
		}
		if m := cbtLegacyRe.FindStringSubmatch(ln); m != nil {
			for _, grp := range cbtPySplitWS(m[1]) {
				for _, part := range strings.Split(grp, ",") {
					part = cbtPyStrip(part)
					if part == "" {
						continue
					}
					if strings.HasPrefix(part, "!") {
						neg[part[1:]] = true
					} else {
						pos[part] = true
					}
				}
			}
		}
	}
	return pos, neg, true
}

func cbtHasContinuation(expr string) bool {
	r := cbtPyRStrip(expr)
	return strings.HasSuffix(r, "&&") || strings.HasSuffix(r, "||") || strings.HasSuffix(r, "(")
}

func cbtImplicitPlatforms(filename string, goos, goarch map[string]bool) map[string]bool {
	base := filename
	if strings.HasSuffix(base, ".go") {
		base = base[:len(base)-3]
	}
	if strings.HasSuffix(base, "_test") {
		base = base[:len(base)-5]
	}
	parts := strings.Split(base, "_")
	out := map[string]bool{}
	if len(parts) >= 3 {
		a, b := parts[len(parts)-2], parts[len(parts)-1]
		if goos[a] && goarch[b] {
			out[a] = true
			out[b] = true
		}
	}
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if goos[last] || goarch[last] {
			out[last] = true
		}
	}
	return out
}

// ══════════════════════════════════════════════════════════════════
// 判据 (a)：tag 命名
// ══════════════════════════════════════════════════════════════════

func cbtTagArgCandidates(line string) [][2]string {
	hit := false
	for _, h := range cbtCommandHints {
		if strings.Contains(line, h) {
			hit = true
			break
		}
	}
	if !hit {
		return nil
	}
	var out [][2]string
	for _, re := range []*regexp.Regexp{cbtTagArgRe1, cbtTagArgRe2} {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			for _, raw := range strings.Split(m[1], ",") {
				name := strings.Trim(cbtPyStrip(raw), cbtTagValueTrim)
				if name == "" || strings.HasPrefix(name, "-") {
					continue
				}
				if !cbtTagValueRe.MatchString(name) {
					continue
				}
				out = append(out, [2]string{name, m[0]})
			}
		}
	}
	return out
}

func cbtCheckTagNaming(root string, mods []cbtModule, goos, goarch map[string]bool, rep *cbtReport) {
	reserved := cbtReservedNames(goos, goarch)
	allowed := map[string]bool{}
	for _, t := range cbtAllowedCustomTags {
		allowed[t] = true
	}

	for _, t := range cbtSortedStrs(cbtAllowedCustomTags) {
		if cbtIsReserved(t, reserved) {
			rep.fail("a2", fmt.Sprintf("白名单里的自定义 tag %s 撞上了工具链保留名 ⇒ 语义会被静默接管（改 tag 名，别改这条规矩）", cbtPyRepr(t)))
		}
	}

	nfiles := 0
	customSeen := map[string][]string{}
	for _, mod := range mods {
		files := cbtScanGoFiles(mod.dir)
		nfiles += len(files)
		bydir := map[string][]string{}
		for _, f := range files {
			d := cbtPyDirname(f)
			bydir[d] = append(bydir[d], f)
		}
		for _, d := range cbtSortedKeysSlice(bydir) {
			dirImplied := map[string]bool{}
			dirNeg := map[string]bool{}
			parsed := map[string][2]map[string]bool{}
			for _, f := range bydir[d] {
				p, n, _ := cbtParseConstraints(f)
				parsed[f] = [2]map[string]bool{p, n}
				for k := range cbtImplicitPlatforms(cbtPyBasename(f), goos, goarch) {
					dirImplied[k] = true
				}
				for k := range n {
					dirNeg[k] = true
				}
			}
			for _, f := range cbtSortedKeysTup(parsed) {
				p := parsed[f][0]
				rel := cbtPyRelpath(f, root)
				for _, t := range cbtSortedKeysBool(p) {
					if cbtIsReserved(t, reserved) {
						continue
					}
					if !allowed[t] {
						customSeen[t] = append(customSeen[t], rel)
					}
				}
				own := cbtImplicitPlatforms(cbtPyBasename(f), goos, goarch)
				var inter []string
				for t := range p {
					if goos[t] || goarch[t] {
						inter = append(inter, t)
					}
				}
				for _, t := range cbtSortedStrs(inter) {
					if own[t] {
						continue
					}
					if !(dirNeg[t] || cbtAnyOther(dirImplied, t)) {
						rep.fail("a4", fmt.Sprintf("%s 里用裸平台名 %s 当构建约束，而该目录既没有反面（!%s）约束、也没有别的平台隐含文件 ⇒ 别的平台上一条同名 tag 就能把它编进来（实测形态）", rel, cbtPyRepr(t), t))
					}
				}
			}
		}
	}
	for _, t := range cbtSortedKeysSlice(customSeen) {
		uniq := cbtUniqSorted(customSeen[t])
		if len(uniq) > 5 {
			uniq = uniq[:5]
		}
		rep.fail("a1", fmt.Sprintf("未登记的自定义 tag %s（出现在 %s）—— 新 tag = 新构建配置，必须在检查脚本的 ALLOWED_CUSTOM_TAGS 里登记", cbtPyRepr(t), strings.Join(uniq, "、")))
	}
	rep.note(fmt.Sprintf("a1/a2/a4 扫描面：%d 个 .go 文件 · %d 个模块 · 保留名 %d 个（GOOS %d + GOARCH %d + 固定 %d + go1.x 形态）",
		nfiles, len(mods), len(reserved), len(goos), len(goarch), len(cbtFixedReserved)))

	nargs := 0
	// 自指排除面 = py 原件（同一判据的源，正文写着「判据的形态」字面量）+ 本 Go 件。
	// py 侧 SELF_PATH 就是这一处；`.go` 不在 SCAN_EXTS，故 Go 件那一条是冗余保险。
	selfPaths := map[string]bool{}
	if cbtRepoRootResolved != "" {
		selfPaths[cbtPyAbspath(cbtPyJoin(cbtRepoRootResolved, "scripts", "gates", "check-build-tags.py"))] = true
		selfPaths[cbtPyAbspath(cbtPyJoin(cbtRepoRootResolved, "core", "cmd", "zerg", "check_build_tags.go"))] = true
	}
	cbtWalk(root, func(dir string, dirs []string, files []string) []string {
		var nd []string
		for _, d := range dirs {
			if !cbtSkipDirs[d] {
				nd = append(nd, d)
			}
		}
		sort.Strings(files)
		for _, fn := range files {
			if !(cbtHasScanExt(fn) || strings.HasPrefix(fn, "Makefile")) {
				continue
			}
			path := cbtPyJoin(dir, fn)
			if len(selfPaths) > 0 && selfPaths[cbtPyAbspath(path)] {
				continue
			}
			lines, ok := cbtReadLines(path, true)
			if !ok {
				continue
			}
			for i, ln := range lines {
				for _, cand := range cbtTagArgCandidates(ln) {
					nargs++
					name, shown := cand[0], cand[1]
					if cbtIsReserved(name, reserved) {
						rep.fail("a3", fmt.Sprintf("%s:%d 把保留名 %s 当 tag 实参传（%s）—— 同名 tag 能顶替平台约束", cbtPyRelpath(path, root), i+1, cbtPyRepr(name), shown))
					} else if !allowed[name] {
						rep.fail("a3", fmt.Sprintf("%s:%d 传了未登记的自定义 tag %s（%s）", cbtPyRelpath(path, root), i+1, cbtPyRepr(name), shown))
					}
				}
			}
		}
		return nd
	})
	rep.note(fmt.Sprintf("a3 扫描面：脚本/CI 里 %d 个 tag 实参", nargs))
}

// ══════════════════════════════════════════════════════════════════
// 判据 (e)：GOFLAGS 断言
// ══════════════════════════════════════════════════════════════════

func cbtCheckGoflags(rep *cbtReport) {
	inherited := os.Getenv("GOFLAGS")
	if cbtGoflagsTagRe.MatchString(inherited) {
		rep.fail("e", fmt.Sprintf("继承环境 GOFLAGS=%s 里带 tag 旗标 —— 一个环境变量就能把生产构建变成调试构建；生产侧必须显式置空并断言（本脚本自己的 go 调用已显式覆盖 GOFLAGS='')", cbtPyRepr(inherited)))
	}
	rep.note(fmt.Sprintf("e 状态：继承 GOFLAGS=%s（本脚本所有 go 调用已显式覆盖为空）", cbtPyRepr(inherited)))
}

// ══════════════════════════════════════════════════════════════════
// 判据 (b)(c)：两个构建都过 + vet 成对跑
// ══════════════════════════════════════════════════════════════════

type cbtStep struct {
	judgement, label string
	args             []string
}

var cbtSteps = []cbtStep{
	{"b", "默认构建 go build ./...", []string{"build", "-buildvcs=false", "-o", "@OUT@", "./..."}},
	{"b", "调试构建 go build -tags=debug ./...", []string{"build", "-buildvcs=false", "-tags=debug", "-o", "@OUT@", "./..."}},
	{"c", "默认 vet  go vet ./...", []string{"vet", "./..."}},
	{"c", "调试 vet  go vet -tags=debug ./...", []string{"vet", "-tags=debug", "./..."}},
}

var cbtBuildOutDirs []string

func cbtBuildOutDir() string {
	if len(cbtBuildOutDirs) == 0 {
		if d, err := os.MkdirTemp("", "zerg-buildtags-out-"); err == nil {
			cbtBuildOutDirs = append(cbtBuildOutDirs, d)
		}
	}
	if len(cbtBuildOutDirs) == 0 {
		return os.TempDir() + "/"
	}
	return cbtBuildOutDirs[0] + "/"
}

func cbtCleanupOutdir() {
	for _, d := range cbtBuildOutDirs {
		_ = os.RemoveAll(d)
	}
	cbtBuildOutDirs = nil
}

func cbtCheckBuilds(mods []cbtModule, goBin string, rep *cbtReport, sink cbtSink) bool {
	outdir := cbtBuildOutDir()
	for _, mod := range mods {
		name := cbtPyBasename(mod.dir)
		for _, st := range cbtSteps {
			args := make([]string, 0, len(st.args))
			for _, a := range st.args {
				if a == "@OUT@" {
					args = append(args, outdir)
				} else {
					args = append(args, a)
				}
			}
			rc, ok, out := cbtRun(append([]string{goBin}, args...), mod.dir, cbtTagEnv(), nil, cbtBuildTimeout)
			extra := ""
			if ok && rc != 0 && strings.Contains(out, "no main packages to build") {
				var plain []string
				skip := false
				for _, a := range args {
					if skip {
						skip = false
						continue
					}
					if a == "-o" {
						skip = true
						continue
					}
					plain = append(plain, a)
				}
				rc, ok, out = cbtRun(append([]string{goBin}, plain...), mod.dir, cbtTagEnv(), nil, cbtBuildTimeout)
				extra = "（该模块无 main 包 ⇒ 不带 -o 重跑，无产物可落）"
			}
			if !ok {
				rep.cannot(fmt.Sprintf("[%s] %s: %s —— %s", name, st.label, "超时/起不来", cbtTail(out, 3)))
				continue
			}
			mark := "✓"
			if rc != 0 {
				mark = "✗"
			}
			sink.p("     %s [%s] %-38s rc=%s%s", mark, name, st.label, strconv.Itoa(rc), extra)
			if rc != 0 {
				rep.fail(st.judgement, fmt.Sprintf("[%s] %s 真实退出码=%d；输出尾部:\n%s", name, st.label, rc, cbtTail(out, cbtTailLines)))
			}
		}
	}
	rep.note(fmt.Sprintf("构建产物落点：%s（显式 -o 到临时目录，仓内不留散件；退出即清理）", strings.TrimRight(outdir, "/")))
	return true
}

// ══════════════════════════════════════════════════════════════════
// 判据 (d)：导出面奇偶校验
// ══════════════════════════════════════════════════════════════════

func cbtExportFilePath(goBin, importpath string, tags []string, moduleDir string) (string, string, bool) {
	args := []string{"list", "-export", "-f", "{{.Export}}"}
	args = append(args, tags...)
	args = append(args, importpath)
	rc, ok, out := cbtRun(append([]string{goBin}, args...), moduleDir, cbtTagEnv(), nil, cbtBuildTimeout)
	if !ok {
		return "", fmt.Sprintf("（%s）超时/起不来", importpath), false
	}
	if rc != 0 {
		return "", fmt.Sprintf("（%s）go list 失败 rc=%d: %s", importpath, rc, cbtTail(out, 5)), false
	}
	trimmed := cbtPyStrip(out)
	path := ""
	if trimmed != "" {
		l := cbtPySplitLines(trimmed)
		path = cbtPyStrip(l[len(l)-1])
	}
	if path == "" {
		return "", fmt.Sprintf("（%s）go list 没给出可读的导出数据文件: %s", importpath, cbtPyRepr(path)), false
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return "", fmt.Sprintf("（%s）go list 没给出可读的导出数据文件: %s", importpath, cbtPyRepr(path)), false
	}
	return path, "", true
}

func cbtReverseLines(s string) []string {
	lines := cbtPySplitLines(s)
	out := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		out = append(out, lines[i])
	}
	return out
}

func cbtJSONDecode(s string, v *map[string]any) error {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	return dec.Decode(v)
}

func cbtCheckParity(root string, mods []cbtModule, goBin string, pkgs []string, rep *cbtReport, sink cbtSink) {
	type target struct{ rel, pkg, a, b string }
	var targets []target
	for _, rel := range pkgs {
		d := cbtPyAbspath(cbtPyJoin(root, rel))
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			rep.cannot(fmt.Sprintf("导出面：登记的受关注包不存在（名单过期？）: %s", rel))
			continue
		}
		mod, found := cbtModuleOf(mods, d)
		if !found {
			rep.cannot(fmt.Sprintf("导出面：%s 不在任何已发现的 Go 模块里", rel))
			continue
		}
		sub := cbtPyRelpath(d, mod.dir)
		importpath := mod.path
		if sub != "." {
			importpath = mod.path + "/" + sub
		}
		pa, ea, oka := cbtExportFilePath(goBin, importpath, nil, mod.dir)
		pb, eb, okb := cbtExportFilePath(goBin, importpath, []string{"-tags=debug"}, mod.dir)
		if !oka || !okb {
			rep.cannot(fmt.Sprintf("导出面：%s 的两套导出数据取不齐 —— %s %s", rel, ea, eb))
			continue
		}
		targets = append(targets, target{rel, importpath, pa, pb})
	}
	if len(targets) == 0 {
		return
	}
	type jt struct {
		Pkg string `json:"pkg"`
		A   string `json:"a"`
		B   string `json:"b"`
	}
	jts := make([]jt, 0, len(targets))
	for _, t := range targets {
		jts = append(jts, jt{t.pkg, t.a, t.b})
	}
	raw, _ := json.Marshal(map[string]any{"targets": jts})
	payload := string(raw)
	rc, ok, out := cbtRun([]string{goBin, "run", "."}, cbtHelperDirResolved, cbtTagEnv(), &payload, cbtBuildTimeout)
	var blob map[string]any
	for _, ln := range cbtReverseLines(out) {
		ln = cbtPyStrip(ln)
		if strings.HasPrefix(ln, "{") && strings.HasSuffix(ln, "}") {
			blob = nil
			if err := cbtJSONDecode(ln, &blob); err == nil {
				break
			}
			blob = nil
		}
	}
	if blob == nil {
		rep.cannot(fmt.Sprintf("导出面：scripts/exportnames 没产出可解析的 JSON（rc=%s）: %s", cbtRCStr(rc, ok), cbtTail(out, 5)))
		return
	}
	bypkg := map[string]map[string]any{}
	if results, isList := blob["results"].([]any); isList {
		for _, r := range results {
			if rm, isMap := r.(map[string]any); isMap {
				if k, isStr := rm["pkg"].(string); isStr {
					bypkg[k] = rm
				}
			}
		}
	}
	for _, t := range targets {
		r := bypkg[t.pkg]
		if r == nil {
			rep.cannot(fmt.Sprintf("导出面：%s 的结果缺失（helper 输出里没有该包）", t.pkg))
			continue
		}
		if errv, has := r["err"]; has && cbtPyTruthy(errv) {
			rep.cannot(fmt.Sprintf("导出面：%s 导出数据读不出来 ⇒ 判不了：%s", t.rel, cbtAnyStr(errv)))
			continue
		}
		onlyA := cbtAnyList(r["only_a"])
		onlyB := cbtAnyList(r["only_b"])
		if len(onlyA) > 0 || len(onlyB) > 0 {
			sideA, sideB := "无", "无"
			if len(onlyA) > 0 {
				sideA = cbtPyListRepr(onlyA)
			}
			if len(onlyB) > 0 {
				sideB = cbtPyListRepr(onlyB)
			}
			rep.fail("d", fmt.Sprintf("导出面不一致 %s（%s）：默认侧共 %s 名 / debug 侧共 %s 名；只在默认侧 %s ｜ 只在 debug 侧 %s",
				t.rel, t.pkg, cbtAnyStr(r["n_a"]), cbtAnyStr(r["n_b"]), sideA, sideB))
		} else {
			sink.p("     ✓ [导出面] %-34s 两侧各 %s 名，逐名一致", t.rel, cbtAnyStr(r["n_a"]))
		}
	}
	var rels []string
	for _, t := range targets {
		rels = append(rels, t.rel)
	}
	rep.note("导出面受关注包：" + strings.Join(rels, "、"))
}

// ══════════════════════════════════════════════════════════════════
// 报告与主流程
// ══════════════════════════════════════════════════════════════════

func cbtPrintContract(sink cbtSink) {
	sink.p("门禁契约（改脚本顶部常量 = 改契约）")
	sink.p("  允许的自定义 tag : %s", strings.Join(cbtAllowedCustomTags, ", "))
	sink.p("  导出面受关注包   : %s", strings.Join(cbtExportParityPkgs, ", "))
	sink.p("  保留名固定部分   : %s", strings.Join(cbtFixedReserved, ", "))
	sink.p("  平台名           : 现取自 `go tool dist list`（GOOS/GOARCH 全集）")
	sink.p("  判据             : a1 a2 a3 a4（tag 命名）· b（两构建都过）· c（vet 成对）· d（导出面奇偶）· e（GOFLAGS）")
}

func cbtPrintReport(rep *cbtReport, title string, sink cbtSink) int {
	sink.p(strings.Repeat("─", 60))
	if len(rep.findings) > 0 {
		sink.p("判红项 %d 条：", len(rep.findings))
		for _, f := range rep.findings {
			sink.p("  ✗ [判据(%s)] %s", f.judgement, f.text)
		}
	} else {
		sink.p("判红项 0 条")
	}
	if len(rep.undecidable) > 0 {
		sink.p("判不了项 %d 条（不给结论）：", len(rep.undecidable))
		for _, t := range rep.undecidable {
			sink.p("  ⚠ %s", t)
		}
	}
	for _, n := range rep.notes {
		sink.p("  · %s", n)
	}
	rc := rep.rc()
	if rc == 0 {
		sink.p("结论[%s]: 全绿 —— 两个构建配置的工程面一致（tag 命名 / 构建 / vet / 导出面 / GOFLAGS）", title)
	} else if rc == 1 {
		sink.p("结论[%s]: 红灯 —— 上面每条都点名了文件与判定依据", title)
	} else {
		sink.p("结论[%s]: 不给结论（rc=2）—— 有条目判不了（缺件 / 超时 / 读不出导出数据）", title)
	}
	return rc
}

func cbtScanReal(args *cbtArgs, rep *cbtReport, sink cbtSink) *cbtReport {
	goBin := cbtGoBin()
	if goBin == "" {
		rep.cannot("本机 PATH 里没有 go —— 判据 b/c/d 全部无法判定（绝不把「判不了」当绿）")
		return rep
	}
	goos, goarch, errOut, ok := cbtDistPlatforms(goBin)
	if !ok {
		rep.cannot(fmt.Sprintf("取不到平台名集合（go tool dist list 失败）：%s", cbtTail(errOut, 3)))
		return rep
	}
	mods := cbtDiscoverModules(args.root)
	if len(mods) == 0 {
		rep.cannot(fmt.Sprintf("在 %s 下没有发现任何 Go 模块（go.mod）—— 仓不完整？", args.root))
		return rep
	}
	sink.p("── (a)(e) 静态断言（红了就不烧构建时间）──")
	cbtCheckTagNaming(args.root, mods, goos, goarch, rep)
	cbtCheckGoflags(rep)
	if len(rep.findings) > 0 {
		rep.note("静态断言已红 ⇒ 跳过构建/vet/导出面（先修命名与 GOFLAGS，再谈成对跑）")
		return rep
	}
	sink.p("── (b)(c) 成对构建与 vet（每模块 4 条，各取真实退出码）──")
	cbtCheckBuilds(mods, goBin, rep, sink)
	sink.p("── (d) 导出面奇偶校验（go list -export + go/importer）──")
	if len(rep.findings) > 0 {
		rep.note("构建/vet 已红 ⇒ 导出面仍照跑（同一批读数，便于一次看清）")
	}
	cbtCheckParity(args.root, mods, goBin, args.parity, rep, sink)
	return rep
}

// ══════════════════════════════════════════════════════════════════
// 自检：合成夹具（正控 + 四类负控）
// ══════════════════════════════════════════════════════════════════

const cbtCleanP = "package p\n\n// Foo 是夹具里唯一的导出函数。\nfunc Foo() int { return 1 }\n\n// T 是夹具里的导出类型。\ntype T struct{}\n\n// M 是 T 的导出方法。\nfunc (T) M() int { return 2 }\n"

func cbtFixtureBase(base string) {
	_ = os.MkdirAll(cbtPyJoin(base, "p"), 0o755)
	_ = os.WriteFile(cbtPyJoin(base, "go.mod"), []byte("module zergfixture\n\ngo 1.21\n"), 0o644)
	_ = os.WriteFile(cbtPyJoin(base, "p", "p.go"), []byte(cbtCleanP), 0o644)
}

func cbtMakeFixture(kind, tmproot string) string {
	base, err := os.MkdirTemp(tmproot, "fx-"+kind+"-")
	if err != nil {
		return ""
	}
	cbtFixtureBase(base)
	switch kind {
	case "reserved-tag":
		_ = os.MkdirAll(cbtPyJoin(base, "q"), 0o755)
		_ = os.WriteFile(cbtPyJoin(base, "q", "zz_bare.go"), []byte("//go:build linux\n\npackage q\n"), 0o644)
		_ = os.WriteFile(cbtPyJoin(base, "q", "zz_custom.go"), []byte("//go:build zergprobe\n\npackage q\n"), 0o644)
		_ = os.MkdirAll(cbtPyJoin(base, "scripts"), 0o755)
		_ = os.WriteFile(cbtPyJoin(base, "scripts", "fake-gate.sh"), []byte("#!/bin/sh\n# 夹具：把保留名当 tag 传\ngo build -tags=linux ./...\n"), 0o644)
	case "one-side":
		_ = os.MkdirAll(cbtPyJoin(base, "q"), 0o755)
		_ = os.WriteFile(cbtPyJoin(base, "q", "ok.go"), []byte("package q\n\nvar OK = 1\n"), 0o644)
		_ = os.WriteFile(cbtPyJoin(base, "q", "only_debug.go"), []byte("//go:build debug\n\npackage q\n\nvar Bad int = \"这不是数字\"\n"), 0o644)
	case "parity":
		_ = os.WriteFile(cbtPyJoin(base, "p", "extra_debug.go"), []byte("//go:build debug\n\npackage p\n\n// DebugOnly 只在调试构建里存在。\nfunc DebugOnly() int { return 3 }\n"), 0o644)
	}
	return base
}

var cbtMainDepth int

func cbtRunFixture(root string, parityPkgs []string, extraEnv map[string]string) (int, string) {
	argv := []string{"--root", root, "--no-self-test"}
	for _, p := range parityPkgs {
		argv = append(argv, "--parity-pkg", p)
	}
	type save struct {
		k, v string
		had  bool
	}
	var saves []save
	set := func(k, v string) {
		old, had := os.LookupEnv(k)
		saves = append(saves, save{k, old, had})
		_ = os.Setenv(k, v)
	}
	set("GOFLAGS", "")
	for k, v := range extraEnv {
		set(k, v)
	}
	var buf bytes.Buffer
	inner := cbtSink{out: &buf, err: &buf}
	rc := cbtMainWith(inner, argv)
	for _, sv := range saves {
		if sv.had {
			_ = os.Setenv(sv.k, sv.v)
		} else {
			_ = os.Unsetenv(sv.k)
		}
	}
	return rc, buf.String()
}

func cbtSelfTest(sink cbtSink) int {
	tmproot, err := os.MkdirTemp("", "zerg-buildtags-selftest-")
	if err != nil {
		tmproot = os.TempDir()
	}
	defer func() { _ = os.RemoveAll(tmproot) }()
	bad, total := 0, 0
	sink.p("── 门禁自检（合成夹具 · 不碰真目标）──")
	type tcase struct {
		kind   string
		pkgs   []string
		envx   map[string]string
		wantRC int
		wants  []string
		desc   string
	}
	cases := []tcase{
		{"clean", []string{"p"}, nil, 0, []string{"全绿"}, "正控：干净夹具必须全绿（镜子不是恒红）"},
		{"reserved-tag", []string{"p"}, nil, 1, []string{"zz_bare.go", "'linux'", "zergprobe", "fake-gate.sh"}, "负控①：保留名 tag / 未登记 tag / 保留名当实参"},
		{"one-side", []string{"p"}, nil, 1, []string{"-tags=debug", "判据(b)", "判据(c)"}, "负控②：只在 debug 一侧编译（且编不过）"},
		{"parity", []string{"p"}, nil, 1, []string{"DebugOnly", "判据(d)"}, "负控③：debug 一侧多出导出符号 ⇒ 导出面不一致"},
		{"clean", []string{"p"}, map[string]string{"GOFLAGS": "-tags=debug"}, 1, []string{"判据(e)"}, "负控④：GOFLAGS 污染 ⇒ 生产构建静默变调试构建"},
	}
	for _, c := range cases {
		root := cbtMakeFixture(c.kind, tmproot)
		rc, out := cbtRunFixture(root, c.pkgs, c.envx)
		total++
		var missing []string
		for _, w := range c.wants {
			if !strings.Contains(out, w) {
				missing = append(missing, w)
			}
		}
		okCase := (rc == c.wantRC) && len(missing) == 0
		if !okCase {
			bad++
		}
		mark := "✓"
		if !okCase {
			mark = "✗"
		}
		suffix := ""
		if !okCase {
			suffix = "；缺关键字 " + cbtPyListRepr(missing)
		}
		sink.p("  %s %s：期望 rc=%d 实得 rc=%d%s", mark, c.desc, c.wantRC, rc, suffix)
		for _, ln := range cbtPySplitLines(out) {
			st := cbtPyStrip(ln)
			if strings.HasPrefix(st, "✗ [判据") {
				sink.p("      ↳ %s", st)
			}
		}
		if !okCase {
			sink.p("      ↳ 输出尾部：")
			for _, ln := range cbtPySplitLines(cbtTail(out, 12)) {
				sink.p("      | %s", ln)
			}
		}
	}
	concl := "不过"
	if bad == 0 {
		concl = "全过"
	}
	sink.p("自检结论: %s（用例 %d 条，失败 %d 条）", concl, total, bad)
	if bad != 0 {
		return 2
	}
	return 0
}

// ══════════════════════════════════════════════════════════════════
// 参数 / Q-176 / 主入口
// ══════════════════════════════════════════════════════════════════

type cbtArgs struct {
	root        string
	parity      []string
	parityGiven bool
	selfOnly    bool
	noSelf      bool
	list        bool
}

const cbtDocBody = "check-build-tags.py — 双构建工程门禁（任务表 T6.3；设计稿 §〇 A3/A4/A5/A6）\n\n一句话：把「默认构建」与「-tags=debug 构建」当成**两个必须等价的工程配置**来守。\n背景（已实测，勿推翻）见 skill `references/debug-build-and-verification.md` 与\n`docs/01-设计/设计-内建调试版-v1.2.md`（第〇节 A 组）。\n\n六条判据\n--------\n(a) tag 命名（A3）\n    a1 未登记的自定义 tag：构建约束里出现「既不是工具链保留名、也不在白名单」的 tag ⇒ FAIL（列 文件:tag）\n    a2 白名单自身撞保留名：允许的自定义 tag 一旦取名撞上 GOOS/GOARCH/unix/cgo/gc/gccgo/go1.x ⇒ FAIL\n    a3 把保留名当 tag 传：脚本/CI 里的 tag 实参（`-tags` / `--build-tags` / `--tags`，`=` 或空格形式）\n       含保留名或未登记名 ⇒ FAIL（列 文件:行:实参）—— 实测就是这个形态能把平台文件编进来\n    a4 裸平台 tag 未配对：无隐含后缀的 .go 里出现**正向** GOOS/GOARCH，而所在目录既没有反面\n       （`!同名`）约束、也没有任何别的平台隐含文件名 ⇒ FAIL（列 文件:tag）\n(b) 两个构建都过（A4 前半）：每个 Go 模块 `go build ./...` 与 `go build -tags=debug ./...` 各取**真实退出码**\n(c) vet 成对跑（A4）：`go vet ./...` 与 `go vet -tags=debug ./...` 各取**真实退出码**\n    —— 被 tag 排除的文件在默认配置下 vet/lint/gofmt 全部看不见，只跑一侧等于没跑\n(d) 导出面奇偶校验（A5）：对受关注包在两种配置下取导出数据（`go list -export`），\n    交给 `scripts/exportnames`（go/importer）读出两套导出名集合，**不一致即 FAIL**\n    —— 在多写一个导出符号时两个构建都会成功，编译器抓不到「多」\n(e) GOFLAGS 断言（A6）：继承环境里若带 tag 相关旗标（一个环境变量就能把生产构建变成调试构建）⇒ FAIL；\n    且本脚本**所有** go 调用显式覆盖 `GOFLAGS`（绝不靠环境继承）\n\n用法\n----\n    python3 scripts/gates/check-build-tags.py                    # 自检 + 扫真目标（仓根 = 本脚本上级目录）\n    python3 scripts/gates/check-build-tags.py --self-test        # 只跑自检（合成夹具，不碰真目标）\n    python3 scripts/gates/check-build-tags.py --root <目录>       # 扫指定仓根（自检夹具也走这条入口）\n    python3 scripts/gates/check-build-tags.py --parity-pkg <相对路径>  # 指定导出面校验包（可重复；缺省用脚本顶部名单）\n    python3 scripts/gates/check-build-tags.py --no-self-test     # 跳过自检（迭代调试用；门禁别用）\n    python3 scripts/gates/check-build-tags.py --list             # 只列判据与名单，不跑\n\n退出码：0 全绿 · 1 有 FAIL 项 · 2 用法错/前置缺件/自检不过（**不给结论**）\n\n纪律：无 sudo · 不联网（只用本机模块缓存）· 临时目录 mktemp -d + 退出即清理 · 前台跑、不起后台进程。"

// py 的 __doc__ 由三引号形态决定（首尾各一个换行）；print(__doc__) 再补一个换行
const cbtDocstring = "\n" + cbtDocBody + "\n"

var cbtJSONFields = []string{}

func cbtRefuseJSON(argv []string, stderr io.Writer) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	if len(cbtJSONFields) > 0 {
		fs := make([]string, 0, len(cbtJSONFields))
		for _, f := range cbtJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	} else {
		fmt.Fprintf(stderr, "   字段表：无（本门只出人读面）\n")
	}
	fmt.Fprintf(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

func cbtParseArgs(argv []string, sink cbtSink) (*cbtArgs, bool) {
	a := &cbtArgs{root: cbtRepoRootDefault(), parity: append([]string{}, cbtExportParityPkgs...)}
	i := 0
	for i < len(argv) {
		v := argv[i]
		if v == "--root" && i+1 < len(argv) {
			a.root = cbtPyAbspath(argv[i+1])
			i += 2
		} else if v == "--parity-pkg" && i+1 < len(argv) {
			p := argv[i+1]
			if !a.parityGiven {
				a.parity = []string{}
				a.parityGiven = true
			}
			if !cbtContainsStr(a.parity, p) {
				a.parity = append(a.parity, p)
			}
			i += 2
		} else if v == "--self-test" {
			a.selfOnly = true
			i++
		} else if v == "--no-self-test" {
			a.noSelf = true
			i++
		} else if v == "--list" {
			a.list = true
			i++
		} else if v == "-h" || v == "--help" {
			sink.p("%s", cbtDocstring)
			return nil, false
		} else {
			sink.pe("✗ 未知参数: %s", v)
			return nil, false
		}
	}
	return a, true
}

func cbtMainWith(sink cbtSink, argv []string) int {
	cbtMainDepth++
	me := cbtMainDepth
	defer func() {
		cbtMainDepth--
		if me == 1 {
			cbtCleanupOutdir()
		}
	}()
	if rc, hit := cbtRefuseJSON(argv, sink.err); hit {
		return rc
	}
	args, ok := cbtParseArgs(argv, sink)
	if !ok {
		return 2
	}
	if args.selfOnly && args.noSelf {
		sink.pe("✗ --self-test 与 --no-self-test 不能同用")
		return 2
	}
	if args.list {
		cbtPrintContract(sink)
		return 0
	}
	if !args.noSelf {
		if rc := cbtSelfTest(sink); rc != 0 {
			sink.pe("✗ 自检不过 ⇒ 拒绝扫真目标（rc=2）")
			return 2
		}
		if args.selfOnly {
			return 0
		}
	}
	root := args.root
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		sink.pe("✗ 仓根不存在: %s", root)
		return 2
	}
	sink.p(strings.Repeat("═", 60))
	sink.p("双构建工程门禁 · 仓根 %s", root)
	cbtPrintContract(sink)
	sink.p(strings.Repeat("═", 60))
	rep := &cbtReport{}
	cbtScanReal(args, rep, sink)
	return cbtPrintReport(rep, cbtPyBasename(root), sink)
}

func checkBuildTagsGateMain(stdout, stderr io.Writer, argv []string) int {
	cbtResolveHomeOnce()
	sink := cbtSink{out: stdout, err: stderr}
	return cbtMainWith(sink, argv)
}
