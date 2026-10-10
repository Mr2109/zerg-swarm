// check_devdocs_root_single_source.go —— 门禁族「开发文档根单一真源」· **只报告**判据
// （缺口 GAP-20260927-47 甲(a)）。
//
// 病（缺口逐字）：同一个「开发文档面」被多道判据各判一遍，而其中一些判据的**默认根**里
// 硬写着一个**版本目录字面**（如 `项目文档/v2.5.10`）—— 与版号真源（`core/internal/version/version.go`
// 的 `const Version`，即 `v2.5.14`）**不一致** ⇒ 同一趟门判的是**两棵不同的树**，而 rc 都看不出。
//
// 本门做什么：逐格点名「判据件默认根 ⟷ 版号真源」不符者，输出给**两侧逐字读数**
// （件:行 ＋ 取的路径 ＋ 真源值）；相符的位点**不**点名。
//
// 口径（逐条）：
//
//	· ★ **只报告**：命中不阻断（接线档 `tri-report`）。rc = 1（有不符）/ 0（全相符）；
//	  真源取不到 / 扫描面空转 ⇒ rc=2（不给结论 · ✗ 不当绿）。
//	· ★ 真源**现读**（`core/internal/version/version.go` 的 `const Version = "X.Y.Z"`）；
//	  本件**不硬写版本目录真值**（✗ 源码中不出现「项目文档 + 版号」的连续字面）。
//	· ★ 取不到真源 ⇒ **不给结论（rc=2）**，✗ 不静默回落（fail-closed）。
//	· ★ **只读**扫描：不起子进程 / 不联网 / 不写任何件（仅仓外只读临时夹具）。
//
// 扫描面与「位点」定义（两种形态 · 都是「默认根 = <同级文档仓>/项目文档/<版>」这一构造）：
//
//	形态 A（直接字面）：`<join>(..., "项目文档", "vX.Y.Z")` ⇒ 报该字面所在行；
//	形态 B（标识符）  ：`<join>(..., "项目文档", <标识>)`，而 `<标识>` 是**本件内**赋值为
//	  裸版号字面 `"vX.Y.Z"` 的常量/变量（如 `cdfSlugVersionDir`）⇒ 报**声明行**。
//
// ✗ 不碰被点名件的**任何字节**（那属 GAP-20260927-47 的 (b) 半 · 另一批带收敛的活）。
//
// ★ 不入 `main.go` 命令树；入 `gate_inproc_table.go` 的键 `check-devdocs-root-single-source`（进程内路由）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	cdrssRCok    = 0
	cdrssRCfail  = 1
	cdrssRCblock = 2

	cdrssScanDirRel = "core/cmd/zerg"                    // 判据件目录（扫描面）
	cdrssVersionRel = "core/internal/version/version.go" // 版号真源（单一来源）
	cdrssDocSeg     = "项目文档"                             // 开发文档树段（真源版号目录的上一级）
)

// cdrssGoVerConstRe —— 真源解析：`const Version = "2.5.14"` ⇒ 捕获 `2.5.14`。
var cdrssGoVerConstRe = regexp.MustCompile(`(?m)^\s*const\s+Version\s*=\s*"([0-9][0-9.]*)"`)

// 两个「默认根」构造形态的定位正则（★ 由**部件**拼出 ⇒ 本件源码里不出现连续的
// 「项目文档 + 版号」字面 ⇒ 本件对自己的扫描面 0 命中）。
var (
	cdrssJoinLitRe   = regexp.MustCompile(`"` + cdrssDocSeg + `"\s*,\s*"(v[0-9]+(?:\.[0-9]+)+)"`)
	cdrssJoinIdentRe = regexp.MustCompile(`"` + cdrssDocSeg + `"\s*,\s*([A-Za-z_][A-Za-z0-9_]*)`)
)

// cdrssSite —— 一处「默认根版号位点」。
type cdrssSite struct {
	file    string // 仓内相对路径
	line    int    // 位点行（形态 B 报声明行）
	path    string // 取的路径（`项目文档/<版>`）
	version string // 取的版号目录字面
	text    string // 该行原文（截断）
}

func cdrssTrim(s string) string {
	r := []rune(strings.TrimRight(s, " \t\r"))
	if len(r) > 160 {
		r = append(r[:160], '…')
	}
	return string(r)
}

// cdrssTrueSource —— 现读版号真源（带 `v` 前缀，如 `v2.5.14`）。
func cdrssTrueSource(root string) (string, bool, string) {
	p := filepath.Join(root, filepath.FromSlash(cdrssVersionRel))
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false, fmt.Sprintf("读不了 %s（%v）", cdrssVersionRel, err)
	}
	m := cdrssGoVerConstRe.FindSubmatch(b)
	if m == nil {
		return "", false, fmt.Sprintf("%s 里找不到 `const Version = \"X\"`", cdrssVersionRel)
	}
	return "v" + string(m[1]), true, ""
}

// cdrssResolveIdent —— 形态 B：把标识符解析为「本件内赋值为裸版号字面」的声明行与版号。
func cdrssResolveIdent(lines []string, ident string) (int, string, bool) {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(ident) + `\s*=\s*"(v[0-9]+(?:\.[0-9]+)+)"`)
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "//") {
			continue
		}
		if m := re.FindStringSubmatch(ln); m != nil {
			return i + 1, m[1], true
		}
	}
	return 0, "", false
}

// cdrssScan —— 扫判据件目录，返回（读到的判据件数, 位点表）。
func cdrssScan(root string) (int, []cdrssSite) {
	dir := filepath.Join(root, filepath.FromSlash(cdrssScanDirRel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}
	var files []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		files = append(files, n)
	}
	sort.Strings(files)
	seen := map[string]bool{}
	var sites []cdrssSite
	for _, n := range files {
		rel := cdrssScanDirRel + "/" + n
		raw, rerr := os.ReadFile(filepath.Join(dir, n))
		if rerr != nil {
			continue
		}
		lines := strings.Split(string(raw), "\n")
		for i, ln := range lines {
			tr := strings.TrimSpace(ln)
			if strings.HasPrefix(tr, "//") || strings.HasPrefix(tr, "/*") || strings.HasPrefix(tr, "*") {
				continue // 注释行不作位点（说明文字不算硬写）
			}
			if m := cdrssJoinLitRe.FindStringSubmatch(ln); m != nil {
				key := fmt.Sprintf("%s:%d", rel, i+1)
				if !seen[key] {
					seen[key] = true
					sites = append(sites, cdrssSite{file: rel, line: i + 1,
						path: cdrssDocSeg + "/" + m[1], version: m[1], text: cdrssTrim(ln)})
				}
			}
			for _, im := range cdrssJoinIdentRe.FindAllStringSubmatch(ln, -1) {
				declLine, ver, ok := cdrssResolveIdent(lines, im[1])
				if !ok {
					continue
				}
				key := fmt.Sprintf("%s:%d", rel, declLine)
				if !seen[key] {
					seen[key] = true
					sites = append(sites, cdrssSite{file: rel, line: declLine,
						path: cdrssDocSeg + "/" + ver, version: ver, text: cdrssTrim(lines[declLine-1])})
				}
			}
		}
	}
	sort.Slice(sites, func(a, b int) bool {
		if sites[a].file != sites[b].file {
			return sites[a].file < sites[b].file
		}
		return sites[a].line < sites[b].line
	})
	return len(files), sites
}

// cdrssRun —— 默认档：现读真源 ⇒ 扫面 ⇒ 逐格对拍并报告。
func cdrssRun(stdout, stderr io.Writer, root string) int {
	canonical, ok, why := cdrssTrueSource(root)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 不给结论：版号真源取不到 —— %s\n", why)
		return cdrssRCblock
	}
	nfiles, sites := cdrssScan(root)
	if nfiles == 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论：扫描面 0 件判据件（%s/*.go 读不到 ⇒ 空转）\n", cdrssScanDirRel)
		return cdrssRCblock
	}
	var bad []cdrssSite
	for _, s := range sites {
		if s.version != canonical {
			bad = append(bad, s)
		}
	}
	fmt.Fprintf(stdout, "判据件默认根 ⟷ 版号真源 对拍：真源 = %s（%s）｜扫描 %d 件判据件｜位点 %d 处｜不符 %d 处\n",
		canonical, cdrssVersionRel, nfiles, len(sites), len(bad))
	for _, s := range bad {
		fmt.Fprintf(stdout, "✗ 不符 %s:%d  取的路径=%s  真源=%s\n", s.file, s.line, s.path, canonical)
		fmt.Fprintf(stdout, "      行文: %s\n", s.text)
	}
	if len(bad) == 0 {
		fmt.Fprintf(stdout, "⇒ 绿：%d 处开发文档默认根的版号目录 == 真源 %s\n", len(sites), canonical)
		return cdrssRCok
	}
	fmt.Fprintf(stdout, "⇒ 不符 %d 处（★ 只报告档 · tri-report：不阻断提交；这几处硬写的收口归 (b) 批）\n", len(bad))
	return cdrssRCfail
}

func cdrssWriteFile(root, rel, content string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

// cdrssSelfTest —— 正/负控（**合成树 · 不碰真目标**）。
func cdrssSelfTest(stdout, stderr io.Writer) int {
	ok := true
	chk := func(cond bool, okMsg, badMsg string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", okMsg)
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", badMsg)
			ok = false
		}
	}
	td, err := os.MkdirTemp("", "cdrss-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }()
	// 夹具：真源 9.9.9；两件判据件 —— 一件相符（v9.9.9）· 一件不符（v1.0.0 · 且经标识符形态）。
	cdrssWriteFile(td, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	cdrssWriteFile(td, cdrssScanDirRel+"/check_ok.go",
		"package main\n\nfunc ok() string { return join(sib, \"项目文档\", \"v9.9.9\") }\n")
	cdrssWriteFile(td, cdrssScanDirRel+"/check_bad.go",
		"package main\n\nconst slug = \"v1.0.0\"\n\nfunc bad() string { return join(sib, \"项目文档\", slug) }\n")
	var out bytes.Buffer
	rc := cdrssRun(&out, stderr, td)
	got := out.String()
	chk(rc == cdrssRCfail, "正控：一处不符 ⇒ rc=1（只报告档 · 命中即报）",
		fmt.Sprintf("正控失败：期望 rc=1，实得 %d（out=%q）", rc, got))
	chk(strings.Contains(got, "check_bad.go:3"), "正控：逐条点名不符件（含 件:行 · 形态 B 报声明行）",
		"正控失败：输出未点名 check_bad.go:3")
	chk(strings.Contains(got, "取的路径=项目文档/v1.0.0") && strings.Contains(got, "真源=v9.9.9"),
		"正控：两侧逐字读数（取的路径 + 真源值）齐", "正控失败：两侧读数不齐")
	chk(!strings.Contains(got, "check_ok.go"), "正控：相符的位点**不**被点名（判据有区分度）",
		"正控失败：相符位点也被点名（判据无区分度）")
	// 负控①：把不符处改成 == 真源 ⇒ 翻绿（判据对内容敏感 · ✗ 非永真）。
	cdrssWriteFile(td, cdrssScanDirRel+"/check_bad.go",
		"package main\n\nconst slug = \"v9.9.9\"\n\nfunc bad() string { return join(sib, \"项目文档\", slug) }\n")
	var out2 bytes.Buffer
	rc2 := cdrssRun(&out2, stderr, td)
	chk(rc2 == cdrssRCok, "负控①：该处改成 == 真源 ⇒ rc=0（非永真）",
		fmt.Sprintf("负控①失败：期望 rc=0，实得 %d（out=%q）", rc2, out2.String()))
	// 负控②：真源取不到 ⇒ rc=2（不给结论 · ✗ 不静默回落）。
	td2, e2 := os.MkdirTemp("", "cdrss-selftest2-")
	if e2 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td2) }()
	cdrssWriteFile(td2, cdrssScanDirRel+"/check_bad.go",
		"package main\n\nfunc bad() string { return join(sib, \"项目文档\", \"v1.0.0\") }\n")
	var out3 bytes.Buffer
	rc3 := cdrssRun(&out3, stderr, td2)
	chk(rc3 == cdrssRCblock && strings.Contains(out3.String(), "不给结论"),
		"负控②：真源取不到 ⇒ rc=2 且明说「不给结论」（✗ 不静默回落）",
		fmt.Sprintf("负控②失败：rc=%d out=%q", rc3, out3.String()))
	// 负控③：判据件目录 0 件 ⇒ rc=2（空转不给结论）。
	td3, e3 := os.MkdirTemp("", "cdrss-selftest3-")
	if e3 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td3) }()
	cdrssWriteFile(td3, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	var out4 bytes.Buffer
	rc4 := cdrssRun(&out4, stderr, td3)
	chk(rc4 == cdrssRCblock, "负控③：判据件目录 0 件 ⇒ rc=2（空转不给结论）",
		fmt.Sprintf("负控③失败：期望 rc=2，实得 %d", rc4))

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（7 格）\n", res)
	if ok {
		return 0
	}
	return 1
}

const cdrssUsage = "usage: check-devdocs-root-single-source [--root ROOT] [--self-test]\n"

// checkDevdocsRootSingleSourceGateMain —— 进程内入口（`gate_inproc_table.go` 键
// `check-devdocs-root-single-source`）。
func checkDevdocsRootSingleSourceGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	hasRoot := false
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cdrssUsage)
			return cdrssRCok
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, cdrssUsage)
				return cdrssRCblock
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
			hasRoot = true
		default:
			fmt.Fprint(stderr, cdrssUsage)
			fmt.Fprintf(stderr, "check-devdocs-root-single-source: error: unrecognized arguments: %s\n", a)
			return cdrssRCblock
		}
	}
	if selfTest {
		return cdrssSelfTest(stdout, stderr)
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
	return cdrssRun(stdout, stderr, root)
}
