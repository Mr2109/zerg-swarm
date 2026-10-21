// check_genflag_surface.go —— 判据件 `scripts/gates/check-genflag-surface.py` 的 Go 真搬
// （M2 · 入口 `checkGenflagSurfaceGateMain` · 承接缺口 GAP-20260928-59）。
//
// 义（缺口逐字）：生成器/导出器族（`tools/*.py` ＋ `scripts/docs/gen-*.py`）必须装「旗标面」——
//
//	① 有写面者必须有旗标面（闭集）；② 未知旗标必须**拒执**（rc≠0），✗ 不许静默吞后直落默认写动作；
//	③ 必须有 `--help`/`-h` 面（只印用法 · ✗ 不许把帮助请求落成写动作）。
//
// 三态诊断（任一即命中；一件只报一态 · 优先级 甲 > 丙 > 乙）：
//
//	甲·默认动作写盘       = 有写面 ∧ 无旗标面（无 argparse ∧ 无 sys.argv 处理）
//	丙·未知旗标 rc=0 且写盘 = 有写面 ∧ 无「未知旗标 ⇒ 拒执」面
//	乙·认旗标仍写盘       = 有写面 ∧ 有旗标面但无 `--help`/`-h` 面
//
// 口径：**静态只读**（✗ 禁真跑写面 —— 本族含硬编码仓外绝对路径者，真跑会改仓外件）。
//
//	手工旗标面的「拒执面」以字面词表（未知/不认/unrecognized/unknown）识别；argparse 面
//	（`ArgumentParser(` 且未 `add_help=False` 且 `parse_args(` 且未 `parse_known_args`）自带给两面。
//
// ★ 本件已接线（inproc 键 `check-genflag-surface` · `precommit-gates.sh:3906` 门117 tri-report 档）、
// 顶层命令树不改（不改 main.go · 不删 py 原件）。键登记见 gate_inproc_table.go。
//
// 退码（与 py 逐字同）：0 无**新**命中（存量棘轮内） · 1 有新命中（tri-report ⇒ 落 REPORT） ·
//
//	2 取数面拿不到（仓根取不到 / tools 或 scripts/docs 不在盘 ⇒ 不给结论）。
//
// 未覆盖面（✗ 不声称逐字对齐）：argparse 的用法错正文（未知旗标 / `--root` 缺值等）—— 本件只对拍
// `--self-test` / 默认档 / `--json` / `--baseline` 四面；pgpd 防波门的 probe_argv 亦只取 `--self-test`。
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
	"unicode"
)

const (
	gfsTool     = "check-genflag-surface.py"
	gfsVersion  = "v1.0.0"
	gfsGateName = "生成/导出器 旗标面（默认动作写盘 / 认旗标仍写盘 / 未知旗标 rc=0 且写盘）"
)

var gfsSurfaces = []string{"tools/*.py", "scripts/docs/gen-*.py"}

const (
	gfsRCOK     = 0
	gfsRCReport = 1
	gfsRCBlock  = 2
)

// ── 冷启动棘轮基线（存量表）：首读即命中的 (件, 态) 对 ⇒ 只打印、不进退码。只增不删 ──
// ★ 2026-10-08 首读现读（扫 6 件 · 有写面 5 件）：本族两件病史（kb_docs_sync/gen_tools_md）
// 已于 2026-09-28 治好；其余三件走 argparse ⇒ 首读命中 0 ⇒ 基线空。
var gfsBaseline = [][2]string{}

// ── 静态取数规则（RE2 兼容 · 两半逐条同源）────────────────────────────────────
var gfsWriteRes = []*regexp.Regexp{
	regexp.MustCompile(`open\s*\([^\n]*,\s*["'][wax]`),
	regexp.MustCompile(`\.write_text\s*\(`),
	regexp.MustCompile(`\.write_bytes\s*\(`),
	regexp.MustCompile(`json\.dump\s*\(`),
	regexp.MustCompile(`shutil\.copy`),
	regexp.MustCompile(`os\.replace\s*\(`),
}

var (
	gfsREArgv      = regexp.MustCompile(`sys\.argv`)
	gfsREParser    = regexp.MustCompile(`ArgumentParser\s*\(`)
	gfsREParse     = regexp.MustCompile(`parse_args\s*\(`)
	gfsRENoHelp    = regexp.MustCompile(`add_help\s*=\s*False`)
	gfsREKnownArgs = regexp.MustCompile(`parse_known_args`)
	gfsREHelp      = regexp.MustCompile(`--help|["']-h["']`)
	gfsREReject    = regexp.MustCompile(`未知|不认|unrecognized|unknown`)
)

// ── 态码 ─────────────────────────────────────────────────────────────────────
const (
	gfsStateA = "甲" // 默认动作写盘
	gfsStateB = "乙" // 认旗标仍写盘
	gfsStateC = "丙" // 未知旗标 rc=0 且写盘
)

func gfsStateLabel(s string) string {
	switch s {
	case gfsStateA:
		return "甲·默认动作写盘"
	case gfsStateB:
		return "乙·认旗标仍写盘"
	case gfsStateC:
		return "丙·未知旗标 rc=0 且写盘"
	}
	return s
}

// gfsClass —— 一件的静态分类读数。
type gfsClass struct {
	write  bool
	argv   bool
	ap     bool
	face   bool
	help   bool
	reject bool
	state  string
}

type gfsRow struct {
	rel string
	c   gfsClass
}

// gfsStripComments —— 剥注释：整行注释丢弃；行尾 ` #…` 截断（两半同形 · 定序）。
func gfsStripComments(text string) string {
	out := []string{}
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimLeftFunc(ln, unicode.IsSpace), "#") {
			continue
		}
		if i := strings.Index(ln, " #"); i >= 0 {
			ln = ln[:i]
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// gfsClassify —— 静态判一件（输入 = 已剥注释正文）。
func gfsClassify(body string) gfsClass {
	var c gfsClass
	for _, re := range gfsWriteRes {
		if re.MatchString(body) {
			c.write = true
			break
		}
	}
	c.argv = gfsREArgv.MatchString(body)
	parser := gfsREParser.MatchString(body)
	parse := gfsREParse.MatchString(body)
	c.ap = parser && parse && !gfsRENoHelp.MatchString(body) && !gfsREKnownArgs.MatchString(body)
	c.face = c.ap || c.argv
	c.help = c.ap || gfsREHelp.MatchString(body)
	c.reject = c.ap || gfsREReject.MatchString(body)
	if c.write {
		switch {
		case !c.face:
			c.state = gfsStateA
		case !c.reject:
			c.state = gfsStateC
		case !c.help:
			c.state = gfsStateB
		}
	}
	return c
}

// gfsFamily —— 本族件（仓相对路径 · 按名升序 · 去重）＋ 面缺说明（非空 ⇒ rc=2）。
func gfsFamily(root string) ([]string, []string) {
	rels := []string{}
	errs := []string{}
	for _, pat := range gfsSurfaces {
		d := filepath.Dir(pat)
		base := filepath.Base(pat)
		dpath := filepath.Join(root, d)
		info, err := os.Stat(dpath)
		if err != nil || !info.IsDir() {
			errs = append(errs, "面不在盘："+d)
			continue
		}
		ents, err := os.ReadDir(dpath)
		if err != nil {
			errs = append(errs, "面读不到："+d)
			continue
		}
		names := []string{}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			ok, _ := filepath.Match(base, e.Name())
			if ok {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			rels = append(rels, d+"/"+n)
		}
	}
	seen := map[string]bool{}
	uniq := []string{}
	for _, r := range rels {
		if !seen[r] {
			seen[r] = true
			uniq = append(uniq, r)
		}
	}
	sort.Strings(uniq)
	return uniq, errs
}

// gfsCompute —— 现读全部件 ⇒ 行读数 ＋ 取数面错（非空 ⇒ rc=2）。
func gfsCompute(root string) ([]gfsRow, []string) {
	rels, errs := gfsFamily(root)
	rows := []gfsRow{}
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			errs = append(errs, "读不到："+rel)
			continue
		}
		rows = append(rows, gfsRow{rel, gfsClassify(gfsStripComments(string(b)))})
	}
	return rows, errs
}

// gfsAudit —— 按棘轮基线分「新命中 / 存量」。
func gfsAudit(rows []gfsRow, base [][2]string) (hits, stock [][2]string) {
	b := map[[2]string]bool{}
	if base == nil {
		base = gfsBaseline
	}
	for _, p := range base {
		b[p] = true
	}
	for _, r := range rows {
		if r.c.state == "" {
			continue
		}
		p := [2]string{r.rel, r.c.state}
		if b[p] {
			stock = append(stock, p)
		} else {
			hits = append(hits, p)
		}
	}
	return hits, stock
}

// gfsRender —— 人读面（与 py `render` 逐字同）。
func gfsRender(rows []gfsRow, hits, stock [][2]string, unknown string) string {
	w := 0
	for _, r := range rows {
		if r.c.write {
			w++
		}
	}
	out := []string{}
	out = append(out, "── "+gfsGateName+" ──")
	out = append(out, "取数面 : "+strings.Join(gfsSurfaces, " ＋ "))
	out = append(out, fmt.Sprintf("现读   : 扫 %d 件 ｜ 有写面 %d 件 ｜ 命中 %d 件 ｜ 存量（棘轮内）%d 件",
		len(rows), w, len(hits), len(stock)))
	out = append(out, "")
	out = append(out, "每件读数（件 ｜ 有写面 ｜ 甲默认写盘 ｜ 乙认旗标仍写 ｜ 丙未知旗标 rc=0）：")
	if len(rows) == 0 {
		out = append(out, "  （无）")
	}
	for _, r := range rows {
		out = append(out, fmt.Sprintf("  %s ｜ 写面 %s ｜ 甲 %s ｜ 乙 %s ｜ 丙 %s",
			r.rel, gfsYN(r.c.write), gfsYN(r.c.state == gfsStateA),
			gfsYN(r.c.state == gfsStateB), gfsYN(r.c.state == gfsStateC)))
	}
	out = append(out, "")
	out = append(out, "命中（新 · 棘轮表外）：")
	if len(hits) == 0 {
		out = append(out, "  （无）")
	}
	for _, p := range hits {
		out = append(out, "  · "+gfsStateLabel(p[1])+" ← "+p[0])
	}
	out = append(out, "")
	out = append(out, "存量（棘轮内 · 只打印）：")
	if len(stock) == 0 {
		out = append(out, "  （无）")
	}
	for _, p := range stock {
		out = append(out, "  · "+gfsStateLabel(p[1])+" ← "+p[0])
	}
	out = append(out, "")
	out = append(out, "档 : tri-report（只报告 · 命中 rc=1 落 REPORT · ✗ 不阻断）")
	if unknown != "" {
		out = append(out, "✗ 取数面拿不到："+unknown+" ⇒ **不给结论**（rc=2）")
	}
	return strings.Join(out, "\n")
}

func gfsYN(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// gfsBaselineTSV —— 基线表（与 py `baseline_tsv` 逐字同）。
func gfsBaselineTSV(hits, stock [][2]string) string {
	out := []string{"kind\tfile\tstate"}
	for _, p := range hits {
		out = append(out, "new\t"+p[0]+"\t"+p[1])
	}
	for _, p := range stock {
		out = append(out, "stock\t"+p[0]+"\t"+p[1])
	}
	return strings.Join(out, "\n")
}

// ── Python repr 复刻（self-test 输出逐字同所需）──────────────────────────────
func gfsReprBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func gfsReprStrList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, "'"+x+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gfsReprBoolList(xs []bool) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, gfsReprBool(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gfsReprPairList(ps [][2]string) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, "('"+p[0]+"', '"+p[1]+"')")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

type gfsCheck struct {
	name, got, want string
	ok              bool
}

// gfsSelfTest —— 合成夹具自检（与 py `self_test` 逐字同）。
func gfsSelfTest(stdout io.Writer) int {
	checks := []gfsCheck{}
	ck := func(name, got, want string) {
		checks = append(checks, gfsCheck{name, got, want, got == want})
	}

	tmp, err := os.MkdirTemp("", "genflag-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "self-test：夹具根建不出：%v\n", err)
		return gfsRCReport
	}
	defer os.RemoveAll(tmp)

	gfsWrite(tmp, "tools/danger.py", "def main():\n    open('x.md', 'w').write('a')\nmain()\n")
	gfsWrite(tmp, "tools/half.py",
		"import sys\nknown = {'--out'}\nif '--help' in sys.argv:\n    print('u')\n    sys.exit(0)\nopen('y.md', 'w').write('b')\n")
	gfsWrite(tmp, "tools/nohelp.py",
		"import sys\nbad = [a for a in sys.argv[1:] if a.startswith('-') and a not in {'--mark'}]\nif bad:\n    print('✗ 未知参数: %s' % bad)\n    sys.exit(2)\nopen('z.md', 'w').write('c')\n")
	gfsWrite(tmp, "scripts/docs/gen-ok.py",
		"import argparse\nap = argparse.ArgumentParser()\nap.add_argument('--out')\na = ap.parse_args()\nopen(a.out, 'w').write('d')\n")
	gfsWrite(tmp, "scripts/docs/gen-readonly.py",
		"import sys\nprint('only stdout')\n")

	rows, errs := gfsCompute(tmp)
	ck("夹具·取数面无缺", gfsReprStrList(errs), gfsReprStrList([]string{}))
	relList := []string{}
	for _, r := range rows {
		relList = append(relList, r.rel)
	}
	ck("夹具·扫件数", gfsReprStrList(relList), gfsReprStrList([]string{
		"scripts/docs/gen-ok.py", "scripts/docs/gen-readonly.py", "tools/danger.py",
		"tools/half.py", "tools/nohelp.py"}))
	w := 0
	for _, r := range rows {
		if r.c.write {
			w++
		}
	}
	ck("夹具·有写面件数", strconv.Itoa(w), "4")
	hits, _ := gfsAudit(rows, nil)
	ck("夹具·命中件数", strconv.Itoa(len(hits)), "3")
	ck("夹具·命中态与件", gfsReprPairList(hits), gfsReprPairList([][2]string{
		{"tools/danger.py", gfsStateA}, {"tools/half.py", gfsStateC}, {"tools/nohelp.py", gfsStateB}}))
	stateOf := func(rel string) []string {
		for _, r := range rows {
			if r.rel == rel {
				return []string{r.c.state}
			}
		}
		return []string{}
	}
	writeOf := func(rel string) []bool {
		for _, r := range rows {
			if r.rel == rel {
				return []bool{r.c.write}
			}
		}
		return []bool{}
	}
	ck("夹具·干净件无态", gfsReprStrList(stateOf("scripts/docs/gen-ok.py")), gfsReprStrList([]string{""}))
	ck("夹具·只 stdout 件不入写面", gfsReprBoolList(writeOf("scripts/docs/gen-readonly.py")), gfsReprBoolList([]bool{false}))

	// 哑化证牙（负控）：把干净件的旗标面摘掉 ⇒ 必红（甲）
	gfsWrite(tmp, "scripts/docs/gen-ok.py", "open('d.md', 'w').write('d')\n")
	rows2, _ := gfsCompute(tmp)
	hits2, _ := gfsAudit(rows2, nil)
	state2 := []string{}
	for _, r := range rows2 {
		if r.rel == "scripts/docs/gen-ok.py" {
			state2 = []string{r.c.state}
		}
	}
	ck("哑化·摘掉 argparse ⇒ 该件转甲", gfsReprStrList(state2), gfsReprStrList([]string{gfsStateA}))
	ck("哑化·命中升到 4", strconv.Itoa(len(hits2)), "4")

	// 棘轮（正控）：把 danger.py 那一对收进基线 ⇒ 该命中转存量
	hits3, stock3 := gfsAudit(rows2, [][2]string{{"tools/danger.py", gfsStateA}})
	ck("棘轮·收进基线 ⇒ 命中 3", strconv.Itoa(len(hits3)), "3")
	ck("棘轮·存量 1", gfsReprPairList(stock3), gfsReprPairList([][2]string{{"tools/danger.py", gfsStateA}}))

	// 取数面拿不到（面缺）⇒ errs 非空（rc=2 档）
	_, errs4 := gfsCompute(filepath.Join(tmp, "没有此根"))
	ck("缺面 ⇒ 报错非空", gfsReprBool(len(errs4) != 0), gfsReprBool(true))

	fails := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
			fails++
		}
		fmt.Fprintf(stdout, "%s %s：got=%s want=%s\n", mark, c.name, c.got, c.want)
	}
	fmt.Fprintf(stdout, "self-test：%d 格 · %d 失败\n", len(checks), fails)
	if fails == 0 {
		return gfsRCOK
	}
	return gfsRCReport
}

// gfsWrite —— 自检夹具写件（自动建目录）。
func gfsWrite(root, rel, body string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(body), 0o644)
}

// gfsRepoRoot —— 仓根：env ZERG_REPO_ROOT/ZERG_REPO 优先，否则 cwd 上溯找 `.git`（与 py 逐字同）。
func gfsRepoRoot(arg string) string {
	if arg != "" {
		if abs, err := filepath.Abs(arg); err == nil {
			return abs
		}
		return arg
	}
	if env := os.Getenv("ZERG_REPO_ROOT"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	if env := os.Getenv("ZERG_REPO"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	p, _ := os.Getwd()
	for {
		if info, err := os.Stat(filepath.Join(p, ".git")); err == nil && info.IsDir() {
			return p
		}
		nxt := filepath.Dir(p)
		if nxt == p {
			cwd, _ := os.Getwd()
			return cwd
		}
		p = nxt
	}
}

// checkGenflagSurfaceGateMain —— inproc 入口（argv 不含程序名 · 与 py `sys.argv[1:]` 同形）。
func checkGenflagSurfaceGateMain(stdout, stderr io.Writer, argv []string) int {
	var (
		selfTest bool
		asJSON   bool
		baseline bool
		version  bool
		root     string
	)
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--check":
		case a == "--self-test":
			selfTest = true
		case a == "--json":
			asJSON = true
		case a == "--baseline":
			baseline = true
		case a == "--version":
			version = true
		case a == "--repo-root" || a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "%s: 参数 %s 缺值 ⇒ rc=2\n", gfsTool, a)
				return gfsRCBlock
			}
			i++
			root = argv[i]
		default:
			fmt.Fprintf(stderr, "usage: %s [-h] [--check] [--self-test] [--baseline] [--json] [--repo-root ROOT] [--version]\n", gfsTool)
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", gfsTool, a)
			return gfsRCBlock
		}
		i++
	}

	if version {
		fmt.Fprintf(stdout, "%s %s\n", gfsTool, gfsVersion)
		return gfsRCOK
	}
	if selfTest {
		return gfsSelfTest(stdout)
	}

	rt := gfsRepoRoot(root)
	rows, errs := gfsCompute(rt)
	if len(errs) != 0 {
		fmt.Fprintf(stderr, "%s: %s\n", gfsTool, strings.Join(errs, " ｜ "))
		fmt.Fprintf(stderr, "档 : tri-report · ✗ 取数面拿不到 ⇒ 不给结论（rc=2）\n")
		return gfsRCBlock
	}
	// ★ 域级空集守卫（GAP-20260927-108）：本族取数面 0 件 ⇒ fail-closed（✗ 不许空域当绿）
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 空域 —— 本族取数面 0 件（tools/*.py ＋ scripts/docs/gen-*.py）⇒ 不给结论（rc=2）\n", gfsTool)
		return gfsRCBlock
	}
	hits, stock := gfsAudit(rows, nil)
	if asJSON {
		out := []string{}
		out = append(out, "{")
		out = append(out, " \"schema\": \"zerg/genflag-surface/v1\",")
		out = append(out, fmt.Sprintf(" \"scanned\": %d,", len(rows)))
		w := 0
		for _, r := range rows {
			if r.c.write {
				w++
			}
		}
		out = append(out, fmt.Sprintf(" \"with_write\": %d,", w))
		out = append(out, fmt.Sprintf(" \"hit_rows\": %d,", len(hits)))
		out = append(out, fmt.Sprintf(" \"stock_rows\": %d,", len(stock)))
		out = append(out, " \"hits\": "+gfsJSONPairs(hits)+",")
		out = append(out, " \"stock\": "+gfsJSONPairs(stock))
		out = append(out, "}")
		fmt.Fprintln(stdout, strings.Join(out, "\n"))
		if len(hits) != 0 {
			return gfsRCReport
		}
		return gfsRCOK
	}
	if baseline {
		fmt.Fprintln(stdout, gfsBaselineTSV(hits, stock))
		if len(hits) != 0 {
			return gfsRCReport
		}
		return gfsRCOK
	}
	fmt.Fprintln(stdout, gfsRender(rows, hits, stock, ""))
	if len(hits) != 0 {
		return gfsRCReport
	}
	return gfsRCOK
}

// gfsJSONPairs —— `--json` 的 `[{"file":…,"state":…}]`（缩进 1 空格 · 与 py json.dumps(indent=1) 同）。
func gfsJSONPairs(ps [][2]string) string {
	if len(ps) == 0 {
		return "[]"
	}
	out := []string{"["}
	for i, p := range ps {
		comma := ","
		if i == len(ps)-1 {
			comma = ""
		}
		out = append(out, "  {")
		out = append(out, "   \"file\": "+gfsJSONStr(p[0])+",")
		out = append(out, "   \"state\": "+gfsJSONStr(p[1]))
		out = append(out, "  }"+comma)
	}
	out = append(out, " ]")
	return strings.Join(out, "\n")
}

// gfsJSONStr —— JSON 字符串（本族取值只含 ASCII 与 CJK ⇒ 直引号转义足够）。
func gfsJSONStr(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return "\"" + r.Replace(s) + "\""
}
