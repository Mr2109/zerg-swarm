// check_devdocs_root_single_source.go —— 门禁族「开发文档根单一真源」· **只报告**判据
// （缺口 GAP-20260927-47 甲(a) · 口径「派生字面量 = 0」= 缺口 GAP-20260927-88）。
//
// 病（缺口逐字）：同一个「开发文档面」被多道判据各判一遍，而其中一些判据的**默认根**里
// 硬写着一个**版本目录字面**（如 `项目文档/v2.5.10`）—— 与版号真源（`core/internal/version/version.go`
// 的 `const Version`）**不一致** ⇒ 同一趟门判的是**两棵不同的树**，而 rc 都看不出。
//
// ★ 口径（GAP-20260927-88 改）：旧口径「默认根 == 真源」**恒绿** —— 写死的版号只要不落在
// 本件认得的构造形态里就永不落网，而写一个**等于真源**的版号更是一路绿（== 真源 ⇒ 绿）。
// 本门改为断言 **「派生默认根字面量 = 0（位点全登记）」**：
//
//	· 凡「默认根 = <同级文档仓>/项目文档/<版号字面>」这一构造的**版本字面量**，不论它是否
//	  等于现读真源，一律判红 —— 只有两条出路：**改成 derive**（从版号真源现算）或**登记进
//	  `cdrssRegistered` 的带日期历史台账/语料白名单**（那些位的版目录是台账的固定落点，
//	  跟着真源现算反而指错件）。
//	· 真源（`const Version`）仍现读，只用于**报告行**与 fail-closed；**不再**作绿/红的判据
//	  （否则退回恒绿）。
//
// 本门做什么：逐格点名「判据件里的派生默认根版本字面量」，输出给**两侧逐字读数**
// （件:行 ＋ 字面 ＋ 真源值）；登记在册的带日期台账位**不**点名。
//
// 口径（逐条）：
//
//	· ★ **只报告**：命中不阻断（接线档 `tri-report`）。rc = 1（有未登记字面量）/ 0（全绿）；
//	  真源取不到 / 扫描面空转 ⇒ rc=2（不给结论 · ✗ 不当绿）。
//	· ★ 真源**现读**（`core/internal/version/version.go` 的 `const Version = "X.Y.Z"`）；
//	  本件**不硬写版本目录真值**（✗ 源码中不出现「项目文档 + 版号」的连续字面）。
//	· ★ 取不到真源 ⇒ **不给结论（rc=2）**，✗ 不静默回落（fail-closed）。
//	· ★ **只读**扫描：不起子进程 / 不联网 / 不写任何件（仅仓外只读临时夹具）。
//
// 扫描面与「位点」定义（三种形态 · 都是「默认根 = <同级文档仓>/项目文档/<版>」这一构造）：
//
//	形态 A（直接字面）：`<join>(..., "项目文档", "vX.Y.Z")` ⇒ 报该字面所在行；
//	形态 B（标识符）  ：`<join>(..., "项目文档", <标识>)`，而 `<标识>` 是**本件内**赋值为
//	  裸版号字面 `"vX.Y.Z"` 的常量/变量（如 `cdfSlugVersionDir`）⇒ 报**声明行**；
//	形态 C（同级根串）：单串里写死同级文档仓根尾段 `<文档仓名>/项目文档/vX.Y.Z`，且版目录即
//	  **串尾**（可选尾斜杠）—— 只认形态 A/B 时这一形态漏网（写死同级默认根的老写法）。
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

	cdrssScanDirRel = pathCmdZergDir                     // 判据件目录（扫描面）
	cdrssVersionRel = "core/internal/version/version.go" // 版号真源（单一来源）
	cdrssDocSeg     = "项目文档"                             // 开发文档树段（真源版号目录的上一级）
	cdrssDocsTree   = "Zerg-" + "docs"                   // 同级文档仓名（形态 C 用 · 由部件拼出 ⇒ 本件源码不出现连续字面）
)

// cdrssGoVerConstRe —— 真源解析：`const Version = "2.5.14"` ⇒ 捕获 `2.5.14`。
var cdrssGoVerConstRe = regexp.MustCompile(`(?m)^\s*const\s+Version\s*=\s*"([0-9][0-9.]*)"`)

// 三种「默认根」构造形态的定位正则（★ 由**部件**拼出 ⇒ 本件源码里不出现连续的
// 「项目文档 + 版号」字面 ⇒ 本件对自己的扫描面 0 命中）。
var (
	cdrssJoinLitRe   = regexp.MustCompile(`"` + cdrssDocSeg + `"\s*,\s*"(v[0-9]+(?:\.[0-9]+)+)"`)
	cdrssJoinIdentRe = regexp.MustCompile(`"` + cdrssDocSeg + `"\s*,\s*([A-Za-z_][A-Za-z0-9_]*)`)
	cdrssRootStrRe   = regexp.MustCompile(`"[^"]*` + cdrssDocsTree + `/` + cdrssDocSeg + `/(v[0-9]+(?:\.[0-9]+)+)/?"`)
)

// cdrssReg —— 白名单条目：**带日期的历史台账/语料**所在版目录（登记 ⇒ 允许保留该版号字面）。
// ★ 只收「版目录是台账固定落点、跟着真源现算会指错件」的位；✗ 不收新写死的默认根（那些位正解 = derive）。
type cdrssReg struct {
	file   string // 仓内相对路径
	anchor string // 该位点 ±3 行窗口内必现的特征串（改名 / 搬行会让它失效 ⇒ 重新登记）
	date   string // 台账日期（YYYYMMDD）—— ★ 白名单只收带日期的历史台账/语料
	reason string
}

// cdrssRegistered —— 白名单真源（逐条带 date=理由）。
var cdrssRegistered = []cdrssReg{
	{file: "core/cmd/zerg/check_idspace_map.go", anchor: "idmDefaultDoc", date: "20260924",
		reason: "号段映射登记件（登记-号段映射-v2.5.12-20260924.md）的固定落点：带日期的历史台账所在版目录，随真源现算会指到没有该件的版本目录（✗ 不是派生默认值）"},
}

// cdrssRegisteredSite —— 位点是否在带日期台账白名单内（file ＋ ±3 行窗口内 anchor 双命中）。
func cdrssRegisteredSite(s cdrssSite) bool {
	for _, r := range cdrssRegistered {
		if r.file == s.file && strings.Contains(s.ctx, r.anchor) {
			return true
		}
	}
	return false
}

// cdrssSite —— 一处「默认根版号位点」。
type cdrssSite struct {
	file    string // 仓内相对路径
	line    int    // 位点行（形态 B 报声明行）
	path    string // 取的路径（`项目文档/<版>`）
	version string // 取的版号目录字面
	text    string // 该行原文（截断）
	ctx     string // 该行 ±3 行窗口原文（白名单 anchor 判定用）
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

// cdrssReadFails —— B 类读失败登记面（扫描面里读不了的目录/ .go 件 ⇒ 逐件登记 相对路径 + err 正文）。
// ★ B 类真口子修（与 py 有意分叉 · 同族先例 check_doc_cmds.go 的 dcmdReadFails）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 cdrssScan 起手清空 ⇒ 不许跨调用残留。
var cdrssReadFails []string

// cdrssScan —— 扫判据件目录，返回（读到的判据件数, 位点表）。
func cdrssScan(root string) (int, []cdrssSite) {
	cdrssReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	dir := filepath.Join(root, filepath.FromSlash(cdrssScanDirRel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		// ★ B 类真口子修（原静默返回 0 件 · ✗ 不再静默）：**在盘却读不了**才是真读失败 ⇒ 登记 + 判红 + rc=2
		//	（目录不存在 = 本就不该扫 ⇒ 交后续「0 件空转」档）。
		if fi, serr := os.Stat(dir); serr == nil && fi.IsDir() {
			cdrssReadFails = append(cdrssReadFails, cdrssScanDirRel+"（读不了目录）")
		}
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
			// ★ B 类真口子修（原 py 静默跳过 · ✗ 不再静默）：登记 + 判红 + rc=2。
			cdrssReadFails = append(cdrssReadFails, rel+"（"+rerr.Error()+"）")
			continue
		}
		lines := strings.Split(string(raw), "\n")
		win := func(i int) string { // 该行 ±3 行窗口（白名单 anchor 判定用）
			lo, hi := i-3, i+4
			if lo < 0 {
				lo = 0
			}
			if hi > len(lines) {
				hi = len(lines)
			}
			return strings.Join(lines[lo:hi], "\n")
		}
		add := func(line int, mpath, ver, text string) {
			key := fmt.Sprintf("%s:%d", rel, line)
			if seen[key] {
				return
			}
			seen[key] = true
			sites = append(sites, cdrssSite{file: rel, line: line,
				path: mpath, version: ver, text: text, ctx: win(line - 1)})
		}
		for i, ln := range lines {
			tr := strings.TrimSpace(ln)
			if strings.HasPrefix(tr, "//") || strings.HasPrefix(tr, "/*") || strings.HasPrefix(tr, "*") {
				continue // 注释行不作位点（说明文字不算硬写）
			}
			if m := cdrssJoinLitRe.FindStringSubmatch(ln); m != nil {
				add(i+1, cdrssDocSeg+"/"+m[1], m[1], cdrssTrim(ln))
			}
			for _, im := range cdrssJoinIdentRe.FindAllStringSubmatch(ln, -1) {
				declLine, ver, ok := cdrssResolveIdent(lines, im[1])
				if !ok {
					continue
				}
				add(declLine, cdrssDocSeg+"/"+ver, ver, cdrssTrim(lines[declLine-1]))
			}
			// 形态 C（同级根串）：单串里写死同级文档仓根尾段、且版目录即**串尾**。
			for _, cm := range cdrssRootStrRe.FindAllStringSubmatch(ln, -1) {
				add(i+1, cdrssDocSeg+"/"+cm[1], cm[1], cdrssTrim(ln))
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
// ★ 绿/红判据 = 「派生默认根字面量 = 0（位点全登记）」；真源只入报告行（✗ 不再作判据 ⇒ 不退回恒绿）。
func cdrssRun(stdout, stderr io.Writer, root string) int {
	canonical, ok, why := cdrssTrueSource(root)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 不给结论：版号真源取不到 —— %s\n", why)
		return cdrssRCblock
	}
	nfiles, sites := cdrssScan(root)
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	if len(cdrssReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cdrssReadFails))
		for _, u := range cdrssReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return cdrssRCblock
	}
	if nfiles == 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论：扫描面 0 件判据件（%s/*.go 读不到 ⇒ 空转）\n", cdrssScanDirRel)
		return cdrssRCblock
	}
	var unreg []cdrssSite
	for _, s := range sites {
		if !cdrssRegisteredSite(s) {
			unreg = append(unreg, s)
		}
	}
	fmt.Fprintf(stdout, "判据件派生默认根字面量 = 0 对拍：版号真源 = %s（%s）｜扫描 %d 件判据件｜派生字面量 %d 处（登记 %d 处 · 未登记 %d 处）\n",
		canonical, cdrssVersionRel, nfiles, len(sites), len(sites)-len(unreg), len(unreg))
	for _, s := range unreg {
		fmt.Fprintf(stdout, "✗ 未登记 %s:%d  字面=%s  真源=%s\n", s.file, s.line, s.path, canonical)
		fmt.Fprintf(stdout, "      行文: %s\n", s.text)
	}
	if len(unreg) == 0 {
		fmt.Fprintf(stdout, "⇒ 绿：开发文档默认根的版本字面量 = 0 处未登记（本面 %d 处全为带日期的历史台账/语料 · 已逐条登记）\n", len(sites))
		return cdrssRCok
	}
	fmt.Fprintf(stdout, "⇒ 派生默认根版本字面量 %d 处**未登记**（★ 只报告档 · tri-report：不阻断提交；正解 = 改成 derive 或按带日期台账登记）\n", len(unreg))
	return cdrssRCfail
}

func cdrssWriteFile(root, rel, content string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

// cdrssSelfTest —— 正/负控（**合成树 · 不碰真目标**）。★ 9 格。
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
	// 夹具：真源 9.9.9；形态 A 的位点字面 **v9.9.9 恰等于真源**（旧口径的恒绿点）；
	// 形态 B 的标识符赋值为裸版号字面（报声明行）。
	cdrssWriteFile(td, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	cdrssWriteFile(td, cdrssScanDirRel+"/check_a.go",
		"package main\n\nfunc a() string { return join(sib, \"项目文档\", \"v9.9.9\") }\n")
	cdrssWriteFile(td, cdrssScanDirRel+"/check_b.go",
		"package main\n\nconst slug = \"v2.5.10\"\n\nfunc b() string { return join(sib, \"项目文档\", slug) }\n")
	var outA bytes.Buffer
	rcA := cdrssRun(&outA, stderr, td)
	gotA := outA.String()
	chk(rcA == cdrssRCfail, "① 正控：未登记的派生默认根字面量 ⇒ rc=1（只报告档 · 命中即报）",
		fmt.Sprintf("①失败：期望 rc=1，实得 %d（out=%q）", rcA, gotA))
	chk(strings.Contains(gotA, "check_a.go:3"), "② 正控：逐条点名未登记件（含 件:行）",
		"②失败：输出未点名 check_a.go:3")
	chk(strings.Contains(gotA, "字面="+cdrssDocSeg+"/v9.9.9") && strings.Contains(gotA, "真源=v9.9.9"),
		"③ ★ 恒绿反证：字面**等于真源**（v9.9.9）仍判红（旧口径「== 真源 ⇒ 绿」已废）",
		"③失败：等于真源的字面被放行（退回恒绿）")
	chk(strings.Contains(gotA, "check_b.go:3") && strings.Contains(gotA, "字面=项目文档/v2.5.10"),
		"④ 正控：形态 B（标识符）报**声明行**（check_b.go:3）",
		"④失败：形态 B 未点名声明行")
	// 负控①：把 A/B 两处都改成 derive（本件不出现版号字面）⇒ 翻绿（判据对内容敏感 · ✗ 非永真）。
	cdrssWriteFile(td, cdrssScanDirRel+"/check_a.go",
		"package main\n\nfunc a() string { return join(sib, \"项目文档\", mapVer(Version)) }\n")
	cdrssWriteFile(td, cdrssScanDirRel+"/check_b.go",
		"package main\n\nfunc b() string { return mapVer(Version) }\n")
	var outB bytes.Buffer
	rcB := cdrssRun(&outB, stderr, td)
	chk(rcB == cdrssRCok, "⑤ 负控①：该两处改成 derive（无字面）⇒ rc=0（非永真）",
		fmt.Sprintf("⑤失败：期望 rc=0，实得 %d（out=%q）", rcB, outB.String()))
	// 负控②：形态 C（同级根串）· **已登记**的带日期历史台账位 ⇒ 不红（★ 口径未被弄成「凡版号字面就红」）。
	cdrssWriteFile(td, cdrssScanDirRel+"/check_idspace_map.go",
		"package main\n\n// idmDefaultDoc —— 台账固定落点\nfunc idmDefaultDoc() string { return root + \"/../Zerg-内部文档/项目文档/v2.5.10/\" + leaf }\n")
	// 负控③：带日期的历史台账**文件引用**（串尾是件名 · 不是版目录）⇒ 不是派生默认根 ⇒ 不红。
	cdrssWriteFile(td, cdrssScanDirRel+"/check_c.go",
		"package main\n\nconst ledgerRel = \"项目文档/v2.5.10/清单-脚本现状-20260920.tsv\"\n")
	var outC bytes.Buffer
	rcC := cdrssRun(&outC, stderr, td)
	gotC := outC.String()
	chk(rcC == cdrssRCok && strings.Contains(gotC, "未登记 0 处"),
		"⑥ 负控②：形态 C 的位**已登记**（带日期历史台账）⇒ 不红；⑦ 负控③：带日期的台账文件引用（串尾是件名）⇒ 不红",
		fmt.Sprintf("⑥⑦失败：期望 rc=0 且未登记 0 处，实得 rc=%d out=%q", rcC, gotC))
	// 负控④：真源取不到 ⇒ rc=2（不给结论 · ✗ 不静默回落）。
	td2, e2 := os.MkdirTemp("", "cdrss-selftest2-")
	if e2 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td2) }()
	cdrssWriteFile(td2, cdrssScanDirRel+"/check_a.go",
		"package main\n\nfunc a() string { return join(sib, \"项目文档\", \"v1.0.0\") }\n")
	var outD bytes.Buffer
	rcD := cdrssRun(&outD, stderr, td2)
	chk(rcD == cdrssRCblock && strings.Contains(outD.String(), "不给结论"),
		"⑧ 负控④：真源取不到 ⇒ rc=2 且明说「不给结论」（✗ 不静默回落）",
		fmt.Sprintf("⑧失败：rc=%d out=%q", rcD, outD.String()))
	// 负控⑤：判据件目录 0 件 ⇒ rc=2（空转不给结论）。
	td3, e3 := os.MkdirTemp("", "cdrss-selftest3-")
	if e3 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td3) }()
	cdrssWriteFile(td3, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	var outE bytes.Buffer
	rcE := cdrssRun(&outE, stderr, td3)
	chk(rcE == cdrssRCblock, "⑨ 负控⑤：判据件目录 0 件 ⇒ rc=2（空转不给结论）",
		fmt.Sprintf("⑨失败：期望 rc=2，实得 %d", rcE))

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（9 格）\n", res)
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
