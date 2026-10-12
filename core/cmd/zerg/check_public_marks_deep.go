// check_public_marks_deep.go —— 「真实出树面 ⨯ 私有面模式表」**只报告**档（Go 半 · 无 py 半）。
//
// 为什么有这件（不是「顺手加一格」）：
//
//	现 `check-public-marks` 的扫描面 = ① 标记表登记行（public=yes）＋ ② `scripts/<域>/` 现读派生的
//	域面。而**真实会出树的件**（= 发布镜像按『白名单 ∘ EXCLUDES/DROP_* ∘ 映射丢弃』投出的那批）
//	绝大多数**既不在标记表登记行里、也不在 `scripts/<域>/` 面内**（那是四棵源码树 core/agent/ui/…
//	的件）⇒ 现门对「真实出树面里的私有面」**基本看不见**（前路只读量化：出树面命中 2057 处里仅 1 处
//	落在它眼里）。本件把扫描面**换成真实出树面**，按名族汇总命中，并单列「真出树且无 replace 兜底」
//	（= 镜像器的替换规则也吃不掉的那些 —— 它们会**原样**进公开仓）。
//
// 判据面纪律（本档**只报告** ✗ 不升阻断档）：
//
//   - 入口 `checkPublicMarksDeepGateMain`；命令串 = `zerg gate check check-public-marks-deep`（只读档）。
//   - rc 0 = 出树面干净；rc 1 = 有命中（**只报告**：挂它的那一步申报 `tri-report` ⇒ 落 REPORT，
//     不计失败项、不改退出码）；rc 2 = 算不出面 / 缺件 / 空转（**不给结论** ≠ 绿）。
//   - ✗ 不改 `check-public-marks` 的任何判定 / 退码（两件各判各的；共用**同一张** `cpmPrivatePatterns`）。
//
// 关键：**不写死第二份名单** —— 出树面与替换规则都从现读数据件算：
//
//	· 白名单     `publish/whitelist.txt`        （经 `cptpLoadWhitelist`）
//	· 排除表     `publish/excludes.txt`         （经 `cptpLoadExcludes` · 唯一 Go 解析件 excludes_rules.go）
//	· 映射表     `publish/mirror-public-lib.py` （现读 `DROP_EXACT`/`DROP_PREFIX`/`MAP_EXACT` ＋ `map_path` 真身）
//	· 替换规则   `publish/replace-rules.tsv`    （本件现读 · 判定「有无兜底」）
//	· 私有面模式 **同一张** `cpmPrivatePatterns`（`check_public_marks.go` 的真源 · 本件零复制）
//
//	★ 出树面算法**接**既有 Go 半（`check_public_tree_private.go` 的 `cptp*` 件族）—— ✗ 不自创第二实现：
//	  白名单命中 `cptpPublishedPath` · 排除命中 `cptpExcluded` · 映射 `cptpMapPath` · 源 `cptpPublishedSources`。
//	  本件只在它之上**多收一层**：把用「映射后是否进 docs/」丢掉的件（面比 docs 大得多）全部留下。
//
// 出厂声明 (a)：源面差异（照实 · 见交付⑤）。既有 Go 半 `cptpPublishedSources` 用 `git ls-files -z`
// （**索引/工作树面**）；镜像器 `--list-only` 用 `git ls-tree -r HEAD`（**HEAD 面**）。二者在「有已暂存
// 删除（`git rm`）的件」时不同 ⇒ 本件读数与镜像器读数可能差几件（差额逐条可归因：HEAD 有、索引无的
// 退役件）。⇒ ✗ 不声称与镜像器逐件相同；差额在报告里现算现印。
//
// 出厂声明 (b)：自指 —— 本件扫的是**真实出树面**（private workspace 里那些会出树的件），本件自身落
// `core/cmd/zerg/` 且是 `.go`（不在白名单的发布语义里带 `_test` 除外无关） ⇒ 本件对自己读数贡献恒 0。
// 正文里凡「被判红的私有面词面」一律**拆串拼**（`/Users/` + 尾串 一类），免被 `check-hardcoded-private-paths`
// 一类他门扫成新命中。
//
// 遍历/排序点：面件 `sort.Strings`；名族汇总按「处数降序 · 同名族按 why 字典序」进 slice ⇒ **零 Go map 迭代进输出**。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

const cpmdProg = "check-public-marks-deep"

const cpmdUsage = "usage: " + cpmdProg + " [-h] [--self-test]\n"

// ── 替换规则（`publish/replace-rules.tsv`）现读 ───────────────────────────────────────

// cpmdRule —— 一条替换规则（`regex:` 前缀 ⇒ 正则（sed -E 语义 · Go RE2 覆盖本表全部式样）；否则字面量）。
type cpmdRule struct {
	lit string
	re  *regexp.Regexp
}

func cpmdLoadReplaceRules(path string) ([]cpmdRule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules []cpmdRule
	for _, raw := range strings.Split(cptpNormalizeNewlines(string(b)), "\n") {
		line := cptpPyStrip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pat := line
		if i := strings.IndexByte(line, '\t'); i >= 0 {
			pat = line[:i]
		}
		pat = cptpPyStrip(pat)
		if pat == "" || strings.HasPrefix(pat, "!") { // `!夹具<TAB><glob>` 豁免条（现读 0 条生效）
			continue
		}
		if strings.HasPrefix(pat, "regex:") {
			if re, e := regexp.Compile(pat[len("regex:"):]); e == nil {
				rules = append(rules, cpmdRule{re: re})
			}
			continue
		}
		rules = append(rules, cpmdRule{lit: pat})
	}
	return rules, nil
}

// cpmdRuleSpans —— 全部规则在 txt 里的命中区间（判定「有无兜底」用：命中区间与规则区间**重叠** ⇒ 有兜底）。
func cpmdRuleSpans(txt string, rules []cpmdRule) [][2]int {
	var spans [][2]int
	for _, r := range rules {
		if r.re != nil {
			for _, m := range r.re.FindAllStringIndex(txt, -1) {
				spans = append(spans, [2]int{m[0], m[1]})
			}
			continue
		}
		if r.lit == "" {
			continue
		}
		off := 0
		for {
			i := strings.Index(txt[off:], r.lit)
			if i < 0 {
				break
			}
			spans = append(spans, [2]int{off + i, off + i + len(r.lit)})
			off += i + len(r.lit)
		}
	}
	return spans
}

// cpmdCovered —— 命中 [s,e) 是否被某条规则区间**重叠覆盖**（有兜底）。
func cpmdCovered(s, e int, ruleSpans [][2]int) bool {
	for _, sp := range ruleSpans {
		if s < sp[1] && sp[0] < e {
			return true
		}
	}
	return false
}

// ── 出树面（接既有 Go 半 `cptp*` 件族 · ✗ 不自创第二实现）────────────────────────────

// cpmdEntryHits —— 排除表条目是否命中（镜像器 `_entry_hits_anything` 同构：盘上存在 · 索引里有同件/子件 ·
// **或祖先历史里出现过**）。
//
// ★ C19（`GAP-20261013-07`）：原只查「工作树 ＋ HEAD 索引」⇒ 结构性把「历史向保险行」（曾存在、今不在盘、
// 也不在 HEAD 的守卫行）误报成「空指」。本笔并入**祖先历史面**（`cpmdHistoricPaths` · 只读
// `git rev-list --all --objects`）。✗ 不放宽「真空指」判定：**三面**（工作树 / HEAD 索引 / 祖先历史）
// 全无才算空指 —— 路径写残那类（历史里也没有）照旧判空指。
func cpmdEntryHits(entry, root string, tracked, hist, histDirs map[string]bool) bool {
	if cptpExists(cptpPyJoin(root, entry)) {
		return true
	}
	e := strings.TrimSuffix(entry, "/")
	pre := e + "/"
	for p := range tracked {
		if p == e || strings.HasPrefix(p, pre) {
			return true
		}
	}
	if hist[e] || histDirs[e] {
		return true
	}
	return false
}

// cpmdHistoricPaths —— 祖先历史面（只读）：`git -C repo rev-list --all --objects` ⇒ `(路径集, 目录集)`。
// 目录集 = 每条历史路径的**全部祖先目录**（含中间层）⇒ 目录级条目也能 O(1) 命中。
//
// ★ 取不到历史（无 HEAD / git 失败 / 空仓）⇒ 返回**空集**：退化到本笔之前的「工作树 ＋ 索引」两面口径
// ＝**更严**的一侧（把「读不到历史」当「历史里有」会造假绿 ⇒ ✗）。本面只喂「空指」计数，不进命中面。
func cpmdHistoricPaths(repo string) (map[string]bool, map[string]bool) {
	paths, dirs := map[string]bool{}, map[string]bool{}
	cmd := exec.Command("git", "-C", repo, "-c", "core.quotepath=false", "rev-list", "--all", "--objects")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return paths, dirs
	}
	for _, line := range strings.Split(out.String(), "\n") {
		i := strings.IndexByte(line, ' ')
		if i < 0 {
			continue
		}
		p := line[i+1:]
		if p == "" {
			continue
		}
		paths[p] = true
		parts := strings.Split(p, "/")
		for k := 1; k < len(parts); k++ {
			dirs[strings.Join(parts[:k], "/")] = true
		}
	}
	return paths, dirs
}

// cpmdComputeFace —— 真实出树面（private workspace 里会出树的件 · 仓相对路径排序）。
// 返回 (face, 白名单命中数, 跳过数, 空指排除条数, 错因串)。错因串非空 ⇒ 不给结论。
func cpmdComputeFace(root string) ([]string, int, int, int, string) {
	wh := cptpPyJoin(root, "publish", "whitelist.txt")
	ex := cptpPyJoin(root, "publish", "excludes.txt")
	mirrorLib := cptpPyJoin(root, "publish", "mirror-public-lib.py")
	overlay := cptpPyJoin(root, "publish", "docs")
	for _, p := range []string{wh, ex, mirrorLib} {
		if !cptpExists(p) {
			return nil, 0, 0, 0, "缺件：" + p
		}
	}
	entries := cptpLoadWhitelist(wh)
	if len(entries) == 0 {
		return nil, 0, 0, 0, "白名单解析失配 / 0 条（" + wh + "）"
	}
	excludes := cptpLoadExcludes(ex)
	if len(excludes) == 0 {
		return nil, 0, 0, 0, "排除表解析失配 / 0 条（" + ex + "）"
	}
	mlib, err := os.ReadFile(mirrorLib)
	if err != nil {
		return nil, 0, 0, 0, "镜像器读不了：" + err.Error()
	}
	ms := string(mlib)
	if !cptpHasDef(ms, "map_path") {
		return nil, 0, 0, 0, "镜像器映射真身读不出（无 map_path）"
	}
	dropExact := cptpParseStrSet(ms, "DROP_EXACT")
	dropPrefix := cptpParseStrTuple(ms, "DROP_PREFIX")
	mapExact := cptpParseStrMap(ms, "MAP_EXACT")
	cwd, _ := os.Getwd()
	rels, _ := cptpPublishedSources(cwd, root, overlay)
	tracked := map[string]bool{}
	for _, r := range rels {
		tracked[r] = true
	}
	whitelisted, skipped := 0, 0
	face := []string{}
	for _, rel := range rels {
		if !cptpPublishedPath(rel, entries) {
			continue
		}
		whitelisted++
		if cptpExcluded(rel, excludes) || dropExact[rel] || cptpHasPrefixAny(rel, dropPrefix) {
			skipped++
			continue
		}
		if cptpMapPath(rel, dropExact, dropPrefix, mapExact) == "" { // 映射丢弃
			skipped++
			continue
		}
		face = append(face, rel)
	}
	sort.Strings(face)
	// 排除表空指（镜像器对账面口径：盘上无此件、索引里也无同件/子件、**祖先历史里也没有**）——
	// 只报数，不进命中。★ C19（`GAP-20261013-07`）：判定并入祖先历史面（只读 `rev-list --all --objects`）。
	hist, histDirs := cpmdHistoricPaths(root)
	empty := 0
	for _, e := range excludes {
		if !cpmdEntryHits(e, root, tracked, hist, histDirs) {
			empty++
		}
	}
	return face, whitelisted, skipped, empty, ""
}

// ── 扫描与汇总 ───────────────────────────────────────────────────────────────────

type cpmdHit struct {
	why  string
	ln   int
	s, e int
}

// cpmdScanText —— 逐模式扫整件文本（**复用** `cpmPrivatePatterns` · ✗ 不另立白名单）。
func cpmdScanText(txt string) []cpmdHit {
	var hits []cpmdHit
	for _, p := range cpmPrivatePatterns {
		for _, sp := range p.find(txt) {
			ln := strings.Count(txt[:sp[0]], "\n") + 1
			hits = append(hits, cpmdHit{why: p.why, ln: ln, s: sp[0], e: sp[1]})
		}
	}
	return hits
}

type cpmdFam struct {
	why      string
	files    map[string]bool
	hits     int
	bareFile map[string]bool
	bareHits int
}

// cpmdRun —— 判一遍真实出树面 ⇒ rc（0 干净 · 1 有命中【只报告】· 2 不给结论）。
func cpmdRun(root string, out io.Writer) int {
	face, whitelisted, skipped, empty, errMsg := cpmdComputeFace(root)
	if errMsg != "" {
		fmt.Fprintf(out, "✗ %s\n", errMsg)
		fmt.Fprint(out, "⇒ 不给结论（rc=2）：出树面算不出来时**不判** —— 缺件 ≠ 绿\n")
		return 2
	}
	if len(face) == 0 {
		fmt.Fprint(out, "✗ 真实出树面 0 件 ⇒ 空转，拒绝给结论（rc=2）\n")
		return 2
	}
	rules, err := cpmdLoadReplaceRules(cptpPyJoin(root, "publish", "replace-rules.tsv"))
	if err != nil {
		fmt.Fprintf(out, "✗ 替换规则读不了：%s\n", err.Error())
		fmt.Fprint(out, "⇒ 不给结论（rc=2）：无替换规则面 ⇒ 判不了「无兜底」\n")
		return 2
	}
	fams := map[string]*cpmdFam{}
	totalFiles := map[string]bool{}
	totalHits := 0
	bareFiles := map[string]bool{}
	bareHits := 0
	for _, rel := range face {
		txt, rerr := cpmReadText(cptpPyJoin(root, rel))
		if rerr != nil {
			continue // 读不了的件不计入（镜像器同样读不到 ⇒ 不假装判过）
		}
		hits := cpmdScanText(txt)
		if len(hits) == 0 {
			continue
		}
		totalFiles[rel] = true
		ruleSpans := cpmdRuleSpans(txt, rules)
		for _, h := range hits {
			totalHits++
			f := fams[h.why]
			if f == nil {
				f = &cpmdFam{why: h.why, files: map[string]bool{}, bareFile: map[string]bool{}}
				fams[h.why] = f
			}
			f.files[rel] = true
			f.hits++
			if !cpmdCovered(h.s, h.e, ruleSpans) {
				f.bareFile[rel] = true
				f.bareHits++
				bareFiles[rel] = true
				bareHits++
			}
		}
	}
	order := make([]*cpmdFam, 0, len(fams))
	for _, f := range fams {
		order = append(order, f)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].hits != order[j].hits {
			return order[i].hits > order[j].hits
		}
		return order[i].why < order[j].why
	})

	fmt.Fprintf(out, "出树面：%d 件（白名单命中 %d · 跳过 %d · 空指排除条 %d）· 源=git ls-files · 替换规则 %d 条\n",
		len(face), whitelisted, skipped, empty, len(rules))
	fmt.Fprint(out, "\n按名族汇总（命中件/处 · 无 replace 兜底件/处）：\n")
	for _, f := range order {
		fmt.Fprintf(out, "  %-34s %4d 件 / %5d 处   · 无兜底 %4d 件 / %5d 处\n",
			f.why, len(f.files), f.hits, len(f.bareFile), f.bareHits)
	}
	fmt.Fprintf(out, "\n合计：命中 %d 件 / %d 处\n", len(totalFiles), totalHits)
	fmt.Fprintf(out, "真出树且无 replace 兜底：%d 件 / %d 处\n", len(bareFiles), bareHits)
	if bareHits != 0 {
		fmt.Fprintf(out, "⇒ 命中 %d 处无兜底（rc=1 · **本档只报告**：挂它的那一步申报 `tri-report` ⇒ 落 REPORT，\n"+
			"   不计失败项、不改退出码）；处置只有两条路：① 改件（去私有面 / 拼干净）② 补 replace 规则（有兜底）\n"+
			"   —— **不许**放宽本档来凑绿 ✗\n", bareHits)
		return 1
	}
	if totalHits != 0 {
		fmt.Fprintf(out, "⇒ 命中 %d 处但**皆有 replace 兜底**（rc=1 · 只报告）：镜像器会遮掉它们 —— 逐条核对兜底仍在\n", totalHits)
		return 1
	}
	fmt.Fprintf(out, "⇒ 全绿（rc=0）：出树面 %d 件未发现私有面模式命中\n", len(face))
	return 0
}

// ── 自检（成对正负控 · 合成文本 · 不碰真目标）──────────────────────────────────────────

func cpmdSelfTest(out io.Writer) int {
	ok := true
	n := 0
	check := func(label string, got, want int) {
		n++
		if got != want {
			fmt.Fprintf(out, "  ✗ 格%d %s：得 %d（要 %d）\n", n, label, got, want)
			ok = false
		}
	}
	// 夹具一律**拼串构造**（✗ 不在正文留完整私有串）。
	litHost := "ms" + "01"
	litVol := "/Volumes/" + "AHZ"
	txt := "#!/bin/sh\nssh " + litHost + " true\ncd " + litVol + "/x\n"
	hits := cpmdScanText(txt)
	// 格1 正控：含两种私有面 ⇒ ≥2 处命中（机型名 + 私有卷）。
	check("正控·命中数 ≥ 2", cpmBoolToInt(len(hits) >= 2), 1)
	// 格2 正控：规则表**不含**任何命中 ⇒ 全部「无兜底」。
	check("正控·无兜底（规则表不含命中）", cpmdBare(hits, []cpmdRule{{lit: "nothing-here"}}, txt), len(hits))
	// 格3 负控（牙）：规则 = 命中本身（字面量）⇒ 区间重叠 ⇒ 「无兜底」降为 0。
	check("负控·字面规则遮住命中 ⇒ 无兜底 0", cpmdBare(hits, []cpmdRule{{lit: litHost}, {lit: litVol}}, txt), 0)
	// 格4 负控（牙 · 正则档）：规则以正则形态命中 ⇒ 同样算有兜底。
	check("负控·正则规则遮住命中 ⇒ 无兜底 0",
		cpmdBare(hits, []cpmdRule{{re: regexp.MustCompile(regexp.QuoteMeta(litHost))}}, txt), len(hits)-1)
	// 格5 正控：干净文本 ⇒ 0 命中。
	check("正控·干净文本 ⇒ 0 命中", len(cpmdScanText("#!/bin/sh\necho hi\n")), 0)
	res := "PASS"
	if !ok {
		res = "FAIL"
	}
	fmt.Fprintf(out, "自检 %s（%d 格）\n", res, n)
	if ok {
		return 0
	}
	return 1
}

// cpmdBare —— 自检用薄壳：返回「无兜底」命中数。
func cpmdBare(hits []cpmdHit, rules []cpmdRule, txt string) int {
	spans := cpmdRuleSpans(txt, rules)
	c := 0
	for _, h := range hits {
		if !cpmdCovered(h.s, h.e, spans) {
			c++
		}
	}
	return c
}

// ── 入口 ─────────────────────────────────────────────────────────────────────────

func checkPublicMarksDeepGateMain(stdout, stderr io.Writer, argv []string) int {
	selftest := false
	for _, a := range argv {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, cpmdUsage)
			return 0
		case "--self-test", "--selftest":
			selftest = true
		default:
			fmt.Fprint(stderr, cpmdUsage)
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", cpmdProg, a)
			return 2
		}
	}
	if selftest {
		fmt.Fprint(stdout, "── "+cpmdProg+" 自证（合成夹具 · 不碰真目标）\n")
		return cpmdSelfTest(stdout)
	}
	fmt.Fprint(stdout, "── "+cpmdProg+"（真实出树面 ⨯ 私有面模式表 · **只报告**：命中不改退出码）──\n")
	return cpmdRun(cpmDefaultRoot(), stdout)
}
