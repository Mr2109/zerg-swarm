// doc_index_gen.go —— 独立件 `tools/doc_index_gen.py` 的 **Go 真搬**（M2 · v2.5.14 · **不接线 / 不注册 / 不删原件**）。
//
// 入口签名（与 py `main()` 同形）:
//
//	func docIndexGenGateMain(stdout, stderr io.Writer, argv []string) int
//
// 标准声明 (a) 默认根定位差异（**本件不涉**）:
//
//	py 无 `__file__` 上溯根 —— 输入/输出目录**全部由命令行显式给**（位置参数 `dir` ＋ 可选 `--out`，
//	默认 `os.path.join(dir, "INDEX.md")`）。Go 同法逐字同构（cwd 不参与路径推导）⇒ 无定位差。
//
// 标准声明 (b) 自指检查（先真判）:
//
//	① 本件在自己的扫描面内？—— 否。本件扫描面 = 调用方点名的 `<dir>` 里的 `.md` 件（且只取其**首行**做
//	   说明）；Go 件（`core/cmd/zerg/`）既不在该面、也无绝对路径字面量（量化：本件源码零 `/Volumes`、零 `/Users`）。
//	② 落在任何他门扫描面内？—— `check-hardcoded-private-paths`（NEEDLES 为三条绝对路径、扫 `core/`）：
//	   本件不出这类原串。`check-error-kinds` 的 R3（错误登记函数 + 括弧 + 引号形态）：本件不写该形态。
//	③ 写面：本件**唯一写出** = 其**指定产物**（`--out`，默认 `<dir>/INDEX.md`）；对拍与运行一律指**仓外**，
//	   零仓内写、零副作用写、零 subprocess / 联网 / TTY（同批 `gen_glossary_exports` / `gen_tools_md` 同型）。
//
// 对拍面 / 未覆盖面见件尾。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const gdixSelfName = "doc_index_gen.py"

var gdixSkip = []string{"INDEX.md", ".DS_Store"}

type gdixGroup struct {
	prefixes []string
	name     string
}

// GROUPS —— 逐字对齐 py。
var gdixGroups = []gdixGroup{
	{[]string{"00-架构", "01-主控", "02-网关", "03-对话", "04-任务", "05-内部", "06-子端",
		"07-模块-UI", "08-模块", "09-", "10-设计", "11-", "12-", "13-", "14-"}, "状态文档"},
	{[]string{"使用-"}, "使用指南"},
	{[]string{"变更-", "CHANGELOG"}, "版本变更"},
	{[]string{"设计-"}, "设计"},
	{[]string{"调研-", "研究-", "展示-"}, "调研"},
	{[]string{"计划-", "backlog", "清单-", "v2-任务"}, "计划"},
	{[]string{"实施-", "实施完成", "测试报告", "测试方案", "交接-", "修复报告", "分析-", "界面-", "汇总-", "稳定期"}, "实施/记录"},
	{[]string{"问题-", "issue", "修复-", "经验-"}, "问题/经验"},
}

// ★ py 的 `\d` 是 **Unicode 数字** ⇒ 逐条展开为 `\p{Nd}`（✗ RE2 默认 ASCII）。
var (
	gdixRexModule  = regexp.MustCompile("^\\p{Nd}{2}-模块")
	gdixRexVersion = regexp.MustCompile("^v\\p{Nd}+\\.\\p{Nd}+\\.\\p{Nd}+$")
	gdixRexDigits  = regexp.MustCompile("\\p{Nd}+")
	gdixRexMDStrip = regexp.MustCompile("[#>*`\\[\\]]")
)

const gdixUsageLine = "usage: doc_index_gen.py [-h] [--out OUT] dir\n"
const gdixHelpText = gdixUsageLine +
	"\npositional arguments:\n" +
	"  dir\n" +
	"\noptional arguments:\n" +
	"  -h, --help  show this help message and exit\n" +
	"  --out OUT\n"

func gdixInList(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// gdixIsPySpace —— py str.isspace()/re `\s` 的**全套 Unicode 空白**（✗ 不用 Go unicode.IsSpace）。
func gdixIsPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d:
		return true
	case r >= 0x1c && r <= 0x20:
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

func gdixPyStrip(s string) string { return strings.TrimFunc(s, gdixIsPySpace) }

// gdixUniversalNL —— py `open()` 文本模式 = universal newlines。
func gdixUniversalNL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// gdixPyJoin / gdixPyNormpath / gdixPySplit —— posixpath 自写同构（✗ 不用 filepath.Join/Clean）。
func gdixPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path = path + b
		} else {
			path = path + "/" + b
		}
	}
	return path
}

func gdixPyNormpath(path string) string {
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
	var nc []string
	for _, c := range strings.Split(path, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(nc) == 0) || (len(nc) > 0 && nc[len(nc)-1] == "..") {
			nc = append(nc, c)
		} else if len(nc) > 0 {
			nc = nc[:len(nc)-1]
		}
	}
	out := strings.Join(nc, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func gdixPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func gdixGroupOf(name string) string {
	if gdixRexModule.MatchString(name) {
		return "状态文档"
	}
	for _, g := range gdixGroups {
		for _, p := range g.prefixes {
			if strings.HasPrefix(name, p) {
				return g.name
			}
		}
	}
	return "其他"
}

// gdixFirstLines —— 逐字复刻 py first_lines（去首标题行 · 剥 markdown 符号 · 60 码点截断 + `…` · 去重）。
// 读不了 / 非法 UTF-8 ⇒ 空串（py 用 `except Exception: pass`）。
func gdixFirstLines(path string, n int) string {
	var desc []string
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if !utf8.Valid(raw) {
		return ""
	}
	for _, line := range strings.Split(gdixUniversalNL(string(raw)), "\n") {
		line = gdixPyStrip(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") && len(desc) == 0 {
			continue
		}
		line = gdixPyStrip(gdixRexMDStrip.ReplaceAllString(line, ""))
		r := []rune(line)
		if len(r) > 60 {
			line = string(r[:60]) + "…"
		}
		if line != "" {
			dup := false
			for _, d := range desc {
				if d == line {
					dup = true
					break
				}
			}
			if !dup {
				desc = append(desc, line)
			}
		}
		if len(desc) >= n {
			break
		}
	}
	return strings.Join(desc, " | ")
}

func gdixVerKey(v string) []int {
	ms := gdixRexDigits.FindAllString(v, -1)
	var out []int
	for i, m := range ms {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(m) // ★ py `int()` 认 Unicode 数字 ⇒ 本件只覆盖 ASCII（见未覆盖面）
		out = append(out, n)
	}
	return out
}

func gdixKeyLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func gdixKeyEq(a, b []int) bool {
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

func gdixLiveMDCount(parent, d string) (int, bool) {
	ents, err := os.ReadDir(gdixPyJoin(parent, d))
	if err != nil {
		return 0, false
	}
	n := 0
	for _, e := range ents {
		fn := e.Name()
		if strings.HasSuffix(fn, ".md") && !gdixInList(gdixSkip, fn) {
			n++
		}
	}
	return n, true
}

// gdixComputeCur —— 复刻 py 的「当前版」判定（版本号最大且 ≥3 篇者）。取不到 ⇒ false（py 同）。
func gdixComputeCur(ver, parent string) bool {
	if parent == "" {
		return false
	}
	st, err := os.Stat(parent)
	if err != nil || !st.IsDir() {
		return false
	}
	ents, err := os.ReadDir(parent)
	if err != nil {
		return false
	}
	var vs []string
	for _, e := range ents {
		d := e.Name()
		if !gdixRexVersion.MatchString(d) {
			continue
		}
		if st2, err2 := os.Stat(gdixPyJoin(parent, d)); err2 != nil || !st2.IsDir() {
			continue
		}
		vs = append(vs, d)
	}
	var vsF []string
	for _, v := range vs {
		c, ok := gdixLiveMDCount(parent, v)
		if !ok {
			return false
		}
		if c >= 3 {
			vsF = append(vsF, v)
		}
	}
	if len(vsF) == 0 {
		return false
	}
	mx := gdixVerKey(vsF[0])
	for _, v := range vsF[1:] {
		k := gdixVerKey(v)
		if gdixKeyLess(mx, k) {
			mx = k
		}
	}
	return gdixKeyEq(gdixVerKey(ver), mx)
}

// gdixGenIndex —— 复刻 py gen_index。返回 (内容, error)；error 非空 = 输入目录读不了（py 此处抛未捕异常）。
func gdixGenIndex(dirpath string) (string, error) {
	ents, err := os.ReadDir(dirpath)
	if err != nil {
		return "", err
	}
	type fg struct{ g, fn string }
	var files []fg
	for _, e := range ents {
		fn := e.Name()
		if !strings.HasSuffix(fn, ".md") || gdixInList(gdixSkip, fn) {
			continue
		}
		files = append(files, fg{gdixGroupOf(fn), fn})
	}
	np := gdixPyNormpath(dirpath)
	parent, ver := gdixPySplit(np)
	cur := gdixComputeCur(ver, parent)

	lines := []string{"# " + ver + " 版本索引（INDEX——模型查本文档目录入口）", ""}
	if cur {
		lines = append(lines, "> 当前版 = 最新档案（模型默认读这层）——历史版查 git/对应目录（勿读全文）", "")
	} else {
		lines = append(lines, "> 历史档案（完成态——查旧版仅参考——当前真相见最新版 INDEX）", "")
	}
	byG := map[string][]string{}
	for _, f := range files {
		byG[f.g] = append(byG[f.g], f.fn)
	}
	var gnames []string
	for _, g := range gdixGroups {
		gnames = append(gnames, g.name)
	}
	gnames = append(gnames, "其他")
	for _, gname := range gnames {
		rows, ok := byG[gname]
		if !ok {
			continue
		}
		lines = append(lines, "## "+gname, "", "| 文件 | 说明 |", "|---|---|")
		for _, fn := range rows {
			desc := gdixFirstLines(gdixPyJoin(dirpath, fn), 1)
			lines = append(lines, "| ["+fn+"]("+fn+") | "+desc+" |")
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n"), nil
}

// ── argparse 语义近似（✗ 非真 argparse；覆盖本件声明面：-h/--help · --out[=] · 位置 dir）──
func gdixIsHelpPrefix(s string) bool { return len(s) >= 3 && strings.HasPrefix("--help", s) }
func gdixIsOutPrefix(s string) bool  { return len(s) >= 3 && strings.HasPrefix("--out", s) }

func gdixParseArgs(argv []string) (dir, out string, help bool, errMsg string) {
	haveDir := false
	var unrec []string
	noMoreOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if !noMoreOpts {
			if tok == "--" {
				noMoreOpts = true
				continue
			}
			if tok == "-h" || gdixIsHelpPrefix(tok) {
				return "", "", true, ""
			}
			if strings.HasPrefix(tok, "--") {
				name, val := tok, ""
				hasVal := false
				if eq := strings.Index(tok, "="); eq >= 0 {
					name, val, hasVal = tok[:eq], tok[eq+1:], true
				}
				if gdixIsOutPrefix(name) {
					if hasVal {
						out = val
					} else if i+1 < len(argv) {
						i++
						out = argv[i]
					} else {
						return "", "", false, "argument --out: expected one argument"
					}
					continue
				}
				// ★ `--timeout` / `--cancel-on-interrupt` 是**全局旗标**：由 `main.go` 全局面
				//   消费（墙钟上界 / Ctrl-C 取消）；此处只登记「吞下」、**不另立语义** ——
				//   `--timeout` 的值（含以连字符开头者）一并跳过（同族先例：`gate run` 闭白名单）。
				if name == "--timeout" {
					if !hasVal && i+1 < len(argv) {
						i++
					}
					continue
				}
				if name == "--cancel-on-interrupt" {
					continue
				}
				// ★ 全局旗标「吞下」键：`--node` 由全局面（`main.go` 的机器名册校验）消费；此处只登记
				//   「吞下」、**不另立语义** —— `--node` 是**取值旗标** ⇒ 连同它的值一并跳过
				//   （同族先例：`gate run` 闭白名单）。
				if name == "--node" {
					if !hasVal && i+1 < len(argv) {
						i++
					}
					continue
				}
				unrec = append(unrec, tok)
				continue
			}
			if strings.HasPrefix(tok, "-") && tok != "-" {
				unrec = append(unrec, tok)
				continue
			}
		}
		if !haveDir {
			dir = tok
			haveDir = true
		} else {
			unrec = append(unrec, tok)
		}
	}
	if !haveDir {
		return dir, out, false, "the following arguments are required: dir"
	}
	if len(unrec) > 0 {
		return dir, out, false, "unrecognized arguments: " + strings.Join(unrec, " ")
	}
	return dir, out, false, ""
}

func docIndexGenGateMain(stdout, stderr io.Writer, argv []string) int {
	dir, out, help, errMsg := gdixParseArgs(argv)
	if errMsg != "" {
		fmt.Fprint(stderr, gdixUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", gdixSelfName, errMsg)
		return 2
	}
	if help {
		fmt.Fprint(stdout, gdixHelpText)
		return 0
	}
	if out == "" {
		out = gdixPyJoin(dir, "INDEX.md")
	}
	content, err := gdixGenIndex(dir)
	if err != nil {
		fmt.Fprintf(stderr, "doc_index_gen: 输入目录读不了：%s ⇒ 不给结论（rc=2）\n", dir)
		return 2
	}
	if werr := os.WriteFile(out, []byte(content+"\n"), 0o644); werr != nil {
		fmt.Fprintf(stderr, "doc_index_gen: 输出写不进：%s ⇒ 不给结论（rc=2）\n", out)
		return 2
	}
	fmt.Fprintf(stdout, "INDEX 生成: %s（%d 字）\n", out, len([]rune(content)))
	return 0
}
