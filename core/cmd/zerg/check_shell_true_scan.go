// check_shell_true_scan.go —— 「shell 键赋 True」扫面 · **只报告档**门（缺口 GAP-20260927-183）。
//
// 行为：扫仓内「shell 键赋 True」的执行位（`.py/.sh/.go/.rs`），三分类计数：
//
//	· 真执行位 —— shell 键赋 True 出现在**代码行**（非注释 / 非字符串字面量 / 非 heredoc 体内）；
//	· 字面量   —— 同一形态落在**注释** / **字符串字面量** / **heredoc 体内**（夹具与文档 · 假阳面的登记）；
//	· 外部可控 —— 真执行位里命令串由**仓外输入**拼出的（下述三信号任一）。
//
// 外部可控信号（口径 · 逐条）：
//
//	(A) 命令串是 f-string，且含**未加引号**的 `{var}` 变量插值（`"{var}"` 视作已加引号 ⇒ 不计 A）；
//	(B) 所在函数窗口内取 env（`os.environ` / `os.getenv` / `getenv(`）；
//	(C) 命令表达式**首标识符是所在函数的形参**（命令经函数入参传入 —— 静态不可判值来源）。
//
// ★ 只报告档：命中**不驱动 rc**（恒 rc=0；扫面 0 件 ⇒ rc=2 不给结论）。本门**无牙** ——
//
//	与 `tri-report` 档一致（提交链上既不阻断也不留痕，只为取证/报告）。✗ 不改任何判据、✗ 不阻断。
//
// ★ ✗ 不入 `main.go` 命令树；入 `gate_inproc_table.go` 的键 `check-shell-true-scan`（进程内路由）。
//
// ★ 自指消解（本件落在它自己扫描的面内 · `.go` 在 EXTS 里）：本件**代码行/注释**里不出现任何连续的
//
//	「shell 键 = True」字面形态 —— 正则模板用字符串拼接（`"shell" + casWS + "*=..."`）、
//	自检夹具内容走 **base64**（运行期解码 · 源码无连续夹具字面）。⇒ 本件对自身 0 命中。
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const casWS = "[ \t]"

// casShellRe —— 命中形态：shell 键 = True（宽松空白）。✗ 源码里不写成一段连续字面。
var casShellRe = regexp.MustCompile(`shell` + casWS + `*=` + casWS + `*True`)

var casDefRe = regexp.MustCompile(`^\s*def\s+\w+\s*\(([^)]*)\)`)
var casEnvRe = regexp.MustCompile(`os\.environ|os\.getenv|getenv\(`)
var casSubcallRe = regexp.MustCompile(`subprocess\s*\.\s*\w+\s*\(`)
var casIdentRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
var casHeredocRe = regexp.MustCompile(`<<-?[ \t]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

var casCommentPrefixes = []string{"#", "//", "/*", "*", "<!--"}
var casScanSubdirs = []string{"scripts", "core", "agent", "ui", "publish"}
var casExts = []string{".py", ".sh", ".go", ".rs"}
var casSkipDirs = map[string]bool{".git": true, "node_modules": true, "target": true, ".stage": true, "__pycache__": true, "vendor": true}

// casHit —— 一处命中。
type casHit struct {
	file     string
	line     int
	literal  bool   // true = 字面量（注释/字符串/heredoc）；false = 真执行位
	external bool   // 仅字面量=false 时有意义
	signal   string // 外部可控信号说明（可空）
}

func casHasExt(name string) bool {
	for _, e := range casExts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

func casIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func casIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func casWalk(dir string, emit func(string)) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return
	}
	var files, dirs []string
	for _, e := range entries {
		name := e.Name()
		if fi, serr := os.Stat(filepath.Join(dir, name)); serr == nil && fi.IsDir() {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, name := range files {
		if casHasExt(name) {
			emit(filepath.Join(dir, name))
		}
	}
	for _, name := range dirs {
		if casSkipDirs[name] {
			continue
		}
		np := filepath.Join(dir, name)
		if !casIsLink(np) {
			casWalk(np, emit)
		}
	}
}

func casIterFiles(root string, emit func(string)) {
	for _, sub := range casScanSubdirs {
		base := filepath.Join(root, sub)
		if casIsDir(base) {
			casWalk(base, emit)
		}
	}
}

func casSplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// casHeredocBody —— heredoc 体内的物理行号集（1 起），.sh 用。
func casHeredocBody(lines []string) map[int]bool {
	out := map[int]bool{}
	delim := ""
	active := false
	for i, line := range lines {
		n := i + 1
		if !active {
			if m := casHeredocRe.FindStringSubmatch(line); m != nil {
				delim = m[1]
				active = true
			}
			continue
		}
		out[n] = true
		if strings.TrimSpace(line) == delim {
			active = false
		}
	}
	return out
}

// casInQuote —— pos 之前是否有未闭合引号（" ' ` 任一奇数次 ⇒ 落在字符串字面量里）。
func casInQuote(line string, pos int) bool {
	if pos > len(line) {
		pos = len(line)
	}
	seg := line[:pos]
	for _, q := range []string{`"`, `'`, "`"} {
		if strings.Count(seg, q)%2 == 1 {
			return true
		}
	}
	return false
}

func casStartsWithAny(s string, prefixes []string) bool {
	t := strings.TrimLeft(s, " \t")
	for _, p := range prefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// casDefParams —— 命中行向上找最近的 `def`，回其形参名集（找不到回 nil）。
func casDefParams(lines []string, matchLine int) (int, []string) {
	for i := matchLine; i >= 0; i-- {
		if m := casDefRe.FindStringSubmatch(lines[i]); m != nil {
			var ps []string
			for _, p := range strings.Split(m[1], ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if j := strings.IndexAny(p, ":="); j >= 0 {
					p = strings.TrimSpace(p[:j])
				}
				ps = append(ps, p)
			}
			return i, ps
		}
	}
	return -1, nil
}

// casUnquotedInterp —— f-string 里有未加引号的 `{var}` 插值（`"{var}"` 不计）。
func casUnquotedInterp(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		if i == 0 || s[i-1] != '"' {
			return true
		}
	}
	return false
}

// casExternal —— 判一处真执行位是否外部可控，回 (bool, 信号)。
func casExternal(lines []string, matchLine int) (bool, string) {
	defLine, params := casDefParams(lines, matchLine)
	lo := defLine
	if lo < 0 {
		lo = 0
	}
	win := strings.Join(lines[lo:matchLine+1], "\n")
	// (A) f-string 未加引号插值
	if casUnquotedInterp(win) {
		return true, "A: f-string 未加引号插值"
	}
	// (B) env 取值
	if casEnvRe.MatchString(win) {
		return true, "B: env 取值"
	}
	// (C) 命令表达式首标识符是函数形参
	if len(params) > 0 {
		line := lines[matchLine]
		loc := casShellRe.FindStringIndex(line)
		if loc != nil {
			head := line
			if m := casSubcallRe.FindAllStringIndex(line[:loc[0]], -1); len(m) > 0 {
				head = line[m[len(m)-1][1]:loc[0]]
			} else if defLine >= 0 {
				head = strings.Join(lines[defLine:matchLine], "\n")
			}
			if id := casIdentRe.FindString(head); id != "" {
				for _, p := range params {
					if p == id {
						return true, "C: 命令经函数入参（" + id + "）"
					}
				}
			}
		}
	}
	return false, ""
}

// casScanFile —— 扫一件，回该件命中集。
func casScanFile(path string) []casHit {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := casSplitLines(string(b))
	hd := casHeredocBody(lines)
	var hits []casHit
	for i, line := range lines {
		locs := casShellRe.FindAllStringIndex(line, -1)
		if len(locs) == 0 {
			continue
		}
		literal := false
		if casStartsWithAny(line, casCommentPrefixes) {
			literal = true
		} else if hd[i+1] {
			literal = true
		} else if casInQuote(line, locs[0][0]) {
			literal = true
		}
		h := casHit{file: path, line: i + 1, literal: literal}
		if !literal {
			h.external, h.signal = casExternal(lines, i)
		}
		hits = append(hits, h)
	}
	return hits
}

// casScanRoot —— 全仓（SCAN_SUBDIRS）扫面。
func casScanRoot(root string) ([]casHit, int) {
	var hits []casHit
	seen := 0
	casIterFiles(root, func(p string) {
		seen++
		hits = append(hits, casScanFile(p)...)
	})
	return hits, seen
}

func casRel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

type casCounts struct{ real, literal, external int }

func casTally(hits []casHit) casCounts {
	var c casCounts
	for _, h := range hits {
		if h.literal {
			c.literal++
		} else {
			c.real++
			if h.external {
				c.external++
			}
		}
	}
	return c
}

// casRun —— 报告档主逻辑：恒 rc=0；空域（扫面 0 件）⇒ rc=2 不给结论。
func casRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	if !casIsDir(root) {
		fmt.Fprintf(stderr, "BLOCKED —— 扫面根不存在: %s ⇒ rc=2\n", root)
		return 2
	}
	hits, seen := casScanRoot(root)
	if seen == 0 {
		fmt.Fprint(stderr, "BLOCKED —— 扫面 0 件（空域）⇒ rc=2 不给结论\n")
		return 2
	}
	c := casTally(hits)
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].file != hits[j].file {
			return hits[i].file < hits[j].file
		}
		return hits[i].line < hits[j].line
	})
	if asJSON {
		fmt.Fprintf(stdout, `{"rc": 0, "files": %d, "real": %d, "literal": %d, "external": %d}`, seen, c.real, c.literal, c.external)
		fmt.Fprintln(stdout)
		return 0
	}
	fmt.Fprintf(stdout, "「shell"+"=True」扫面（只报告 · GAP-20260927-183）—— 扫 %d 件\n", seen)
	fmt.Fprintf(stdout, "  真执行位 %d · 字面量 %d · 外部可控 %d（外部可控 ⊂ 真执行位）\n", c.real, c.literal, c.external)
	for _, h := range hits {
		rel := casRel(root, h.file)
		switch {
		case h.literal:
			fmt.Fprintf(stdout, "  · %s:%d  字面量（注释/字符串/heredoc）\n", rel, h.line)
		case h.external:
			fmt.Fprintf(stdout, "  ✗ %s:%d  真执行位 · 外部可控（%s）\n", rel, h.line, h.signal)
		default:
			fmt.Fprintf(stdout, "  · %s:%d  真执行位（件内字面量命令）\n", rel, h.line)
		}
	}
	return 0
}

// casB64 —— 自检夹具走 base64（源码无连续夹具字面 ⇒ 本件对自身 0 命中）。
func casB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

// casSelfTest —— 正/负控，含**仓外合成树**（MkdirTemp）。
func casSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "cas-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }()
	ast := filepath.Join(td, "scripts")
	_ = os.MkdirAll(ast, 0o755)
	wr := func(rel, b64 string) string {
		p := filepath.Join(td, rel)
		_ = os.WriteFile(p, casB64(b64), 0o644)
		return p
	}
	// a.py：真执行位 + 外部可控（f-string 未加引号插值）
	wr("scripts/a.py", "aW1wb3J0IHN1YnByb2Nlc3MKY21kID0gZiJydW4gLXRhc2sge3Rhc2t9IDI+JjEiCnN1YnByb2Nlc3MucnVuKGNtZCwgc2hlbGw9VHJ1ZSkK")
	// b.py：真执行位 · 件内字面量命令（无仓外输入）
	wr("scripts/b.py", "aW1wb3J0IHN1YnByb2Nlc3MKc3VicHJvY2Vzcy5ydW4oImxzIC1sYSAyPiYxIiwgc2hlbGw9VHJ1ZSkK")
	// c.py：字面量（在字符串里）
	wr("scripts/c.py", "eCA9ICdzdWJwcm9jZXNzLnJ1bihjbWQsIHNoZWxsPVRydWUpJwo=")
	// d.sh：字面量（注释里）
	wr("scripts/d.sh", "IyBzdWJwcm9jZXNzLnJ1bihjbWQsIHNoZWxsPVRydWUpCmVjaG8gaGkK")
	hits, seen := casScanRoot(td)
	c := casTally(hits)
	if seen != 4 || c.real != 2 || c.literal != 2 || c.external != 1 {
		fmt.Fprintf(stdout, "  ✗ 正控失败：seen=%d real=%d literal=%d external=%d（期望 4/2/2/1）\n", seen, c.real, c.literal, c.external)
		ok = false
	}
	if rc := casRun(stdout, stderr, td, false); rc != 0 {
		fmt.Fprintf(stdout, "  ✗ 正控 rc 失败：应回 0（报告档 · 命中不驱动 rc）\n")
		ok = false
	}
	// 负控①：空根 ⇒ rc=2（不给结论）
	if rc := casRun(stdout, stderr, filepath.Join(td, "nope"), false); rc != 2 {
		fmt.Fprint(stdout, "  ✗ 负控①失败：空根未回 rc=2\n")
		ok = false
	}
	// 负控②：扫面存在但 0 命中 ⇒ rc=0（不是 2）
	td2, err2 := os.MkdirTemp("", "cas-selftest2-")
	if err2 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td2) }()
	_ = os.MkdirAll(filepath.Join(td2, "scripts"), 0o755)
	_ = os.WriteFile(filepath.Join(td2, "scripts/e.py"), []byte("import os\nprint(1)\n"), 0o644)
	if rc := casRun(stdout, stderr, td2, false); rc != 0 {
		fmt.Fprint(stdout, "  ✗ 负控②失败：干净件未回 rc=0\n")
		ok = false
	}
	// 负控③：`"{var}"` 已加引号 ⇒ 不判外部可控（C 信号亦不触发）
	td3, err3 := os.MkdirTemp("", "cas-selftest3-")
	if err3 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td3) }()
	_ = os.MkdirAll(filepath.Join(td3, "scripts"), 0o755)
	_ = os.WriteFile(filepath.Join(td3, "scripts/f.py"), casB64("aW1wb3J0IHN1YnByb2Nlc3MKY21kID0gZiJydW4ge3Rhc2t9IDI+JjEiCnN1YnByb2Nlc3MucnVuKGNtZCwgc2hlbGw9VHJ1ZSkK"), 0o644)
	h3, _ := casScanRoot(td3)
	c3 := casTally(h3)
	if c3.real != 1 || c3.external != 1 {
		fmt.Fprintf(stdout, "  ✗ 负控③失败：real=%d external=%d（应 1/1）\n", c3.real, c3.external)
		ok = false
	}
	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（%d 格）\n", res, 6)
	if ok {
		return 0
	}
	return 1
}

const casUsage = `usage: check-shell-true-scan [--root ROOT] [--json] [--self-test]
`

// checkShellTrueScanGateMain —— 进程内入口（gate_inproc_table.go 键 `check-shell-true-scan`）。
func checkShellTrueScanGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	hasRoot := false
	asJSON := false
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, casUsage)
			return 0
		case a == "--json":
			asJSON = true
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, casUsage)
				return 2
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
			hasRoot = true
		default:
			fmt.Fprint(stderr, casUsage)
			fmt.Fprintf(stderr, "check-shell-true-scan: error: unrecognized arguments: %s\n", a)
			return 2
		}
	}
	if selfTest {
		return casSelfTest(stdout, stderr)
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
	return casRun(stdout, stderr, root, asJSON)
}
