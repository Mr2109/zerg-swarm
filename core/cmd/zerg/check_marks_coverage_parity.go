// check_marks_coverage_parity.go —— 判据件 `scripts/gates/check-marks-coverage-parity.py`（690 行）的 **Go 真搬**
// （M2 一枚 · 标记覆盖对称族 · 入口 `checkMarksCoverageParityGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从 `__file__` 上溯 2 层推 `REPO`（仓根）；Go 无 `__file__` 语境
//
//	⇒ 只能从 **cwd 上溯**找「含 `scripts/公开标记.tsv` 的目录」当默认根。两态在
//	「脚本位置 == cwd 上溯根」时逐字同构（两环境对拍即按此布置）。
//
// 声明 (b) 自指：本件**不落在本判据自己的扫描面内**（本判据扫的是 `scripts/` 域面 + 两张登记表；
//
//	本件落 `core/cmd/zerg/`）。本件正文只出现「仓相对路径字面量」（`scripts/…` / `publish/…`），
//	**零**私有卷/家目录绝对路径字面量 ⇒ 不给 `check-hardcoded-private-paths.py`（SCAN_DIRS 含 `core/`）
//	一类他门造新命中。见交付④的自指四形态核查。
//
// 覆盖口径：
//
//	· R1 域对拍（表侧域集 A ⟷ 门侧域集 B · 只数实测有件的 glob）
//	· R2 入库前两器排除预警（侧甲 = publish-public.sh 的 EXCLUDES ⟷ 侧乙 = mirror-public-lib.py 的
//	  DROP_EXACT / DROP_PREFIX）
//	· 登记豁免（EXEMPT_DOMAINS · 缺 reason|date ⇒ rc=2）
//	· 覆盖缺口（门侧域不在 `check-public-marks.py` 的 DOMAINS 里 ⇒ 只报数 · 不入退码）
//	· `--json` 一律拒（本门无机器读面 ⇒ 缺口 Q-176）
//
// 复用面（py 用 importlib 执行三个仓内模块取它们；Go 不能执行 py ⇒ 逐函数**同构重写**，
//
//	并在注释里逐处标注源行号；这是本件唯一的「语义第二实现」面，见交付⑤未覆盖面）：
//	· `check-public-marks.py` 的 `load_table`(L183) / `TableError` / `face_domains`(L217) /
//	  `_derive_domains`(L246) / `COLS`(L80) / `TOP`(L102) / `OFF_FACE_DOMAINS`(L97) /
//	  `SKIP_DIRS`(L103) / `SKIP_SUFFIX`(L105)
//	· `check-history-secrets.py` 的 `load_excludes`(L101) / `excluded`(L120) /
//	  `load_whitelist`(L85) / `published_path`(L169) / `NOISE_SUFFIX`(L97) / `NOISE_DIRS`(L98)
//	· `publish/mirror-public-lib.py` 的 `DROP_EXACT` / `DROP_PREFIX`（按 Python 字面量现读解析）
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	mkcpTableRel     = "scripts/公开标记.tsv"
	mkcpConfigRel    = "scripts/gates/gate-coverage.config.json"
	mkcpPublishShRel = "scripts/build/publish-public.sh"
	mkcpMirrorLibRel = "publish/mirror-public-lib.py"
	mkcpWhitelistRel = "publish/whitelist.txt"
	mkcpTop          = "" // 顶层 glob（`scripts/*.sh`）的「域」标记：顶层（无第二段）
)

// ── 域豁免登记表（**例外表一档** · 逐条 reason + date；缺任一 ⇒ rc=2，不是红）────────────────
type mkcpExempt struct{ domain, reason, date string }

var mkcpExemptDomains = []mkcpExempt{
	{"exportnames",
		"面外域·分域前的工作目录（Go module，非门覆盖面）：表逐件登记 public=no，" +
			"gate-coverage.config.json 的 script_globs 从设计上不扫它",
		"2026-09-26"},
	{"sandbox-probes",
		"面外域·分域前的实验目录（18 件探针）：表逐件登记 public=no，门覆盖面按设计不含它",
		"2026-09-26"},
	{"plugins",
		"面外域·插件示例件（scripts/plugins/zerg-hello）：表逐件登记 public=no，" +
			"不属「过门禁的脚本」那一批",
		"2026-09-26"},
}

type mkcpUnsupported struct{ msg string }

func (e mkcpUnsupported) Error() string { return e.msg }

// ── 域口径常量（逐字照 check-public-marks.py）──────────────────────────────────────────
var mkcpCols = []string{"文件", "public", "理由"}

var mkcpOffFaceDomains = []string{"exportnames", "sandbox-probes", "plugins"}

var mkcpSkipDirs = map[string]bool{
	"__pycache__": true, ".git": true, ".history": true, ".mypy_cache": true,
	".pytest_cache": true, "node_modules": true, "target": true, "dist": true,
	"venv": true, ".venv": true, "build": true,
}

var mkcpSkipSuffix = []string{"~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store"}

var mkcpNoiseSuffix = []string{".bak", ".orig", ".rej"}
var mkcpNoiseDirs = []string{"__pycache__"}

// mkcpTableError 对应 TableError（表自身不合规 ⇒ rc=2）。
type mkcpTableError struct{ msg string }

func (e mkcpTableError) Error() string { return e.msg }

// ── 表解析（check-public-marks.py :: load_table L183）────────────────────────────────
type mkcpRow struct {
	line int
	rel  string
	pub  string
	why  string
}

func mkcpLoadTable(path string) ([]mkcpRow, error) {
	if !mkcpIsFile(path) {
		return nil, mkcpTableError{"标记表不存在：" + path}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, mkcpTableError{"标记表不存在：" + path}
	}
	lines := strings.Split(string(b), "\n")
	var rows []mkcpRow
	dup := map[string]int{}
	for i, line := range lines {
		ln := i + 1
		if ln == 1 {
			header := strings.Split(strings.TrimRight(line, "\r"), "\t")
			if !mkcpEqStrs(header, mkcpCols) {
				return nil, mkcpTableError{fmt.Sprintf("表头不符：%s（应为 %s）",
					mkcpPyListRepr(header), strings.Join(mkcpCols, "\t"))}
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != len(mkcpCols) {
			return nil, mkcpTableError{fmt.Sprintf("第 %d 行字段数 %d ≠ %d（列口径：%s）",
				ln, len(f), len(mkcpCols), strings.Join(mkcpCols, "\t"))}
		}
		rel, pub, why := strings.TrimSpace(f[0]), strings.TrimSpace(f[1]), strings.TrimSpace(f[2])
		if rel == "" {
			return nil, mkcpTableError{fmt.Sprintf("第 %d 行「文件」列为空", ln)}
		}
		if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "./") || mkcpHasDotDotSegment(rel) {
			return nil, mkcpTableError{fmt.Sprintf("第 %d 行的路径不是仓相对路径（禁用绝对/`./`/`..`）：%s", ln, rel)}
		}
		if pub != "yes" && pub != "no" {
			return nil, mkcpTableError{fmt.Sprintf("第 %d 行 public 不是 yes|no：%s", ln, mkcpPyReprStr(pub))}
		}
		if why == "" {
			return nil, mkcpTableError{fmt.Sprintf("第 %d 行「理由」为空（登记必须说清为什么 —— 写不清理由的标记就是洗白）", ln)}
		}
		if p, ok := dup[rel]; ok {
			return nil, mkcpTableError{fmt.Sprintf("同一件重复登记：%s（第 %d 行与第 %d 行）", rel, p, ln)}
		}
		dup[rel] = ln
		rows = append(rows, mkcpRow{ln, rel, pub, why})
	}
	if len(rows) == 0 {
		return nil, mkcpTableError{"表零数据行（只有表头）—— 空表不许当绿"}
	}
	return rows, nil
}

// ── 域面现读派生（check-public-marks.py :: face_domains L217 / _derive_domains L246）────
func mkcpFaceDomains(rows []mkcpRow, root string) []string {
	doms := map[string]bool{}
	for _, r := range rows {
		p := strings.Split(r.rel, "/")
		if len(p) >= 2 && p[0] == "scripts" {
			if len(p) >= 3 {
				doms[p[1]] = true
			} else {
				doms[mkcpTop] = true
			}
		}
	}
	if root != "" {
		sdir := filepath.Join(root, "scripts")
		if mkcpIsDir(sdir) {
			names, _ := os.ReadDir(sdir)
			for _, e := range names {
				name := e.Name()
				if strings.HasPrefix(name, ".") || mkcpSkipDirs[name] || mkcpHasSuffix(name, mkcpSkipSuffix) {
					continue
				}
				if mkcpIsDir(filepath.Join(sdir, name)) {
					doms[name] = true
				} else if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py") {
					doms[mkcpTop] = true
				}
			}
		}
	}
	out := []string{}
	for d := range doms {
		if mkcpContains(mkcpOffFaceDomains, d) {
			continue
		}
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// ── EXCLUDES / 白名单（check-history-secrets.py L101 / L85 / L120 / L169）──────────────
func mkcpLoadExcludes(path string) []string {
	var ex []string
	if !mkcpExists(path) {
		return ex
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ex
	}
	text := string(b)
	key := "EXCLUDES=("
	i := strings.Index(text, key)
	if i < 0 {
		return ex
	}
	rest := text[i+len(key):]
	j := strings.Index(rest, "\n)")
	if j < 0 {
		return ex
	}
	for _, raw := range strings.Split(rest[:j], "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k := strings.Index(line, "#"); k >= 0 {
			line = line[:k]
		}
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "\"")
		line = strings.Trim(line, "'")
		if line != "" {
			ex = append(ex, line)
		}
	}
	return ex
}

func mkcpExcluded(rel string, ex []string) bool {
	if mkcpHasSuffix(rel, mkcpNoiseSuffix) {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		if mkcpContains(mkcpNoiseDirs, part) {
			return true
		}
	}
	for _, e := range ex {
		if rel == e || strings.HasPrefix(rel, strings.TrimRight(e, "/")+"/") {
			return true
		}
	}
	return false
}

func mkcpLoadWhitelist(path string) []string {
	var entries []string
	b, err := os.ReadFile(path)
	if err != nil {
		return entries
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries
}

func mkcpPublishedPath(rel string, entries []string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			if strings.HasPrefix(rel, e) {
				return true
			}
		} else if rel == e {
			return true
		}
	}
	return false
}

// ── 镜像器自带表（publish/mirror-public-lib.py 的 DROP_EXACT / DROP_PREFIX）：按 Python
//
//	字面量现读解析（Go 不能 import py ⇒ 现读同一件，保「单一真源」的输入面）──────────────
func mkcpPyStringSet(path, name string) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(string(b), "\n")
	start, end := -1, -1
	open := name + " = {"
	for i, ln := range lines {
		if start < 0 && strings.HasPrefix(strings.TrimSpace(ln), open) {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(strings.TrimSpace(ln), "}") {
			end = i
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	if end < 0 {
		end = start // 单行字面量（夹具即此形态）
	}
	return mkcpQuotedOut(lines[start : end+1]), true
}

func mkcpPyStringTuple(path, name string) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(string(b), "\n")
	start, end := -1, -1
	for i, ln := range lines {
		if start < 0 && strings.HasPrefix(strings.TrimSpace(ln), name+" = (") {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(strings.TrimSpace(ln), ")") {
			end = i
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	if end < 0 {
		end = start
	}
	return mkcpQuotedOut(lines[start : end+1]), true
}

// mkcpQuotedOut 逐行去掉行内注释（`#` 之后）后抽双引号串；**去重**（py 侧装进 set）。
func mkcpQuotedOut(lines []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, ln := range lines {
		if k := strings.Index(ln, "#"); k >= 0 {
			ln = ln[:k]
		}
		rest := ln
		for {
			a := strings.Index(rest, "\"")
			if a < 0 {
				break
			}
			b := strings.Index(rest[a+1:], "\"")
			if b < 0 {
				break
			}
			s := rest[a+1 : a+1+b]
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
			rest = rest[a+1+b+1:]
		}
	}
	return out
}

// ── glob（py globmod.glob · 只取**匹配件数**，故遍历序无影响；跳过隐藏项）──────────────
func mkcpGlob(root, pattern string) []string {
	parts := strings.Split(pattern, "/")
	dir := root
	var out []string
	for i, p := range parts {
		if i == len(parts)-1 {
			entries, err := os.ReadDir(dir)
			if err != nil {
				return out
			}
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				if mkcpFnmatch(name, p) {
					out = append(out, filepath.Join(dir, name))
				}
			}
			return out
		}
		dir = filepath.Join(dir, p)
	}
	return out
}

// ── git 索引（py subprocess git -C root -c core.quotepath=false ls-files）─────────────
func mkcpTrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		rc := 1
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		}
		return nil, mkcpUnsupported{fmt.Sprintf(
			"git ls-files 失败：%s（rc=%d ⇒ 跟踪面读不到 = 判不了，缺件 ≠ 绿）", root, rc)}
	}
	var files []string
	for _, ln := range strings.Split(out.String(), "\n") {
		if ln != "" {
			files = append(files, ln)
		}
	}
	return files, nil
}

// ── 表侧 / 门侧域集 ───────────────────────────────────────────────────────────────────
func mkcpTableDomains(rows []mkcpRow) (map[string][]string, []string) {
	doms := map[string][]string{}
	var top []string
	for _, r := range rows {
		p := strings.Split(r.rel, "/")
		if p[0] != "scripts" {
			continue
		}
		if len(p) >= 3 {
			doms[p[1]] = append(doms[p[1]], r.rel)
		} else {
			top = append(top, r.rel)
		}
	}
	return doms, top
}

func mkcpGlobDomains(root string, globs []interface{}) (map[string]bool, []string, map[string]int, error) {
	doms := map[string]bool{}
	var empty []string
	counts := map[string]int{}
	for _, gi := range globs {
		g, ok := gi.(string)
		if !ok || strings.TrimSpace(g) == "" {
			return nil, nil, nil, mkcpUnsupported{fmt.Sprintf(
				"script_globs 里有非字符串/空条目：%s", mkcpPyReprAny(gi))}
		}
		g = strings.TrimSpace(g)
		if !strings.HasPrefix(g, "scripts/") {
			return nil, nil, nil, mkcpUnsupported{fmt.Sprintf(
				"script_globs 条目不在 scripts/ 面：%s（本门的面 = scripts/）", mkcpPyReprStr(g))}
		}
		p := strings.Split(g, "/")
		dom := mkcpTop
		if len(p) >= 3 && !strings.Contains(p[1], "*") {
			dom = p[1]
		}
		n := len(mkcpGlob(root, g))
		if n == 0 {
			empty = append(empty, g)
			continue
		}
		doms[dom] = true
		counts[dom] = counts[dom] + n
	}
	return doms, empty, counts, nil
}

// ── 配置（py load_globs L212）────────────────────────────────────────────────────────
func mkcpLoadGlobs(cfgPath string) ([]interface{}, error) {
	if !mkcpIsFile(cfgPath) {
		return nil, mkcpUnsupported{"config 不在：" + cfgPath}
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, mkcpUnsupported{fmt.Sprintf("config 读不了/不是合法 JSON：%s（%s）", cfgPath, err)}
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, mkcpUnsupported{fmt.Sprintf("config 读不了/不是合法 JSON：%s（%s）", cfgPath, err.Error())}
	}
	var globs []interface{}
	var raw interface{}
	if sc, ok := cfg["scan"].(map[string]interface{}); ok {
		raw = sc["script_globs"]
		if v, ok := raw.([]interface{}); ok {
			globs = v
		}
	}
	if len(globs) == 0 {
		return nil, mkcpUnsupported{fmt.Sprintf(
			"config 的 scan.script_globs 不是非空列表：%s ⇒ 门侧域集读不到 = 空转，拒绝给结论", mkcpPyReprAny(raw))}
	}
	return globs, nil
}

// ── 豁免表合规（py check_exempt L226）────────────────────────────────────────────────
func mkcpCheckExempt(entries []mkcpExempt) ([]string, map[string]int) {
	var bad []string
	doms := map[string]int{}
	for k0, ent := range entries {
		k := k0 + 1
		if ent.domain == "" {
			bad = append(bad, fmt.Sprintf("第 %d 条缺 domain", k))
			continue
		}
		if ent.reason == "" {
			bad = append(bad, fmt.Sprintf("第 %d 条（%s）缺 reason（写不清理由的豁免就是洗白）", k, ent.domain))
		}
		if ent.date == "" {
			bad = append(bad, fmt.Sprintf("第 %d 条（%s）缺 date", k, ent.domain))
		}
		if p, ok := doms[ent.domain]; ok {
			bad = append(bad, fmt.Sprintf("域重复登记：%s（第 %d 条与第 %d 条）", ent.domain, p, k))
		}
		doms[ent.domain] = k
	}
	return bad, doms
}

// mkcpSelfTest —— **未覆盖面**：py 原件的 `--self-test`（18 条自证用例 · 仓外夹具）在 Go 版
// **尚未搬**（本枚时限内只落判据主体面）。此处 fail-closed 返 rc=2 并逐字说明，✗ 不静默绿。
// ⇒ 对拍表里 `--self-test` 一格为**已知口径差**（不是逐字对齐承诺内的格），见交付⑤。
func mkcpSelfTest(stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "✗ 不给结论（rc=2）：本件 Go 版**未搬** `--self-test` 面（18 条自证用例）—— 未覆盖面，✗ 不静默绿")
	return 2
}

// ── 主体（py run L247）────────────────────────────────────────────────────────────
func mkcpRun(stdout io.Writer, root string, table, config string, maxExamples int) int {
	tablePath := table
	if tablePath == "" {
		tablePath = filepath.Join(root, mkcpTableRel)
	}
	configPath := config
	if configPath == "" {
		configPath = filepath.Join(root, mkcpConfigRel)
	}
	publishSh := filepath.Join(root, mkcpPublishShRel)
	mirrorLib := filepath.Join(root, mkcpMirrorLibRel)
	whitelist := filepath.Join(root, mkcpWhitelistRel)
	if !mkcpIsDir(root) {
		return mkcpBlocked(stderrW, mkcpUnsupported{"树根不存在：" + root})
	}
	err := mkcpRunInner(stdout, root, tablePath, configPath, publishSh, mirrorLib, whitelist, maxExamples)
	switch err.(type) {
	case nil:
		return 0
	case mkcpFail:
		return 1
	case mkcpSilent:
		return 2
	default:
		return mkcpBlocked(stderrW, err)
	}
}

func mkcpBlocked(w io.Writer, err error) int {
	fmt.Fprintf(w, "✗ 不给结论（rc=2）：%s\n", err.Error())
	return 2
}

func mkcpRunInner(out io.Writer, root, table, config, publishSh, mirrorLib, whitelist string, maxExamples int) error {
	dump := func(title string, items []string, tail string) {
		fmt.Fprintf(out, "\n── %s：%d 处%s\n", title, len(items), tail)
		lim := len(items)
		if lim > maxExamples {
			lim = maxExamples
		}
		for _, x := range items[:lim] {
			fmt.Fprintf(out, "   %s\n", x)
		}
		if len(items) > maxExamples {
			fmt.Fprintf(out, "   …另 %d 处\n", len(items)-maxExamples)
		}
	}

	// ── 真源加载 ─────────────────────────────────────────────────────────────
	rows, err := mkcpLoadMarksRows(table)
	if err != nil {
		return err
	}
	globs, err := mkcpLoadGlobs(config)
	if err != nil {
		return err
	}
	exemptBad, exemptMap := mkcpCheckExempt(mkcpExemptDomains)
	if len(exemptBad) > 0 {
		for _, b := range exemptBad {
			fmt.Fprintf(stderrW, "✗ 域豁免登记表不合规：%s\n", b)
		}
		fmt.Fprintln(stderrW, "⇒ 不给结论（rc=2）：豁免写不清 = 判不了（不静默放绿）")
		return mkcpSilent{}
	}
	for _, p := range []string{publishSh, mirrorLib, whitelist} {
		if !mkcpIsFile(p) {
			fmt.Fprintf(stderrW, "✗ 缺件：%s ⇒ 不给结论（rc=2）\n", p)
			return mkcpSilent{}
		}
	}
	excludes := mkcpLoadExcludes(publishSh)
	if len(excludes) == 0 {
		fmt.Fprintf(stderrW, "✗ 侧甲：从 %s 解析不出任何 EXCLUDES 条目 ⇒ 不给结论（rc=2）\n", publishSh)
		fmt.Fprintln(stderrW, "   （「清单没了所以全绿」是最难发现的一类假绿）")
		return mkcpSilent{}
	}
	entries := mkcpLoadWhitelist(whitelist)
	if len(entries) == 0 {
		fmt.Fprintf(stderrW, "✗ 白名单解析为空：%s ⇒ 不给结论（rc=2）\n", whitelist)
		return mkcpSilent{}
	}
	dropExact, okExact := mkcpPyStringSet(mirrorLib, "DROP_EXACT")
	dropPrefix, okPrefix := mkcpPyStringTuple(mirrorLib, "DROP_PREFIX")
	if !okExact || !okPrefix {
		fmt.Fprintf(stderrW, "✗ 侧乙：%s 里没有 DROP_EXACT / DROP_PREFIX ⇒ 不给结论（rc=2）\n", mirrorLib)
		return mkcpSilent{}
	}

	// ── R1 域对拍 ────────────────────────────────────────────────────────────
	aDoms, topFiles := mkcpTableDomains(rows)
	bDoms, emptyGlobs, bCounts, err := mkcpGlobDomains(root, globs)
	if err != nil {
		return err
	}
	if len(bDoms) == 0 {
		fmt.Fprintf(stderrW, "✗ 门侧域集为空（script_globs %d 条一条都没匹配到件）⇒ 空转，拒绝给结论（rc=2）\n", len(globs))
		return mkcpSilent{}
	}
	if len(aDoms) == 0 {
		fmt.Fprintln(stderrW, "✗ 表侧域集为空（表里没有一条 `scripts/<域>/…` 行）⇒ 空转，拒绝给结论（rc=2）")
		return mkcpSilent{}
	}
	var aSorted []string
	for d := range aDoms {
		aSorted = append(aSorted, d)
	}
	sort.Strings(aSorted)
	var r1a []string
	for d := range bDoms {
		if _, ok := aDoms[d]; !ok {
			r1a = append(r1a, d)
		}
	}
	sort.Strings(r1a)
	var extra []string
	for _, d := range aSorted {
		if !bDoms[d] {
			extra = append(extra, d)
		}
	}
	var r1b, exemptHit []string
	for _, d := range extra {
		if _, ok := exemptMap[d]; ok {
			exemptHit = append(exemptHit, d)
		} else {
			r1b = append(r1b, d)
		}
	}
	var exemptSorted []string
	for d := range exemptMap {
		exemptSorted = append(exemptSorted, d)
	}
	sort.Strings(exemptSorted)
	var exemptDangle []string
	for _, d := range exemptSorted {
		if !mkcpContains(extra, d) {
			exemptDangle = append(exemptDangle, d)
		}
	}
	_ = exemptHit
	faceDoms := map[string]bool{}
	if repo := mkcpRepoRoot(); repo != "" {
		if faceRows, e := mkcpLoadTable(filepath.Join(repo, mkcpTableRel)); e == nil {
			for _, d := range mkcpFaceDomains(faceRows, repo) {
				faceDoms[d] = true
			}
		}
	}
	var gap []string
	for d := range bDoms {
		if len(faceDoms) > 0 && !faceDoms[d] {
			gap = append(gap, d)
		}
	}
	sort.Strings(gap)

	// ── R2 两器排除面 ───────────────────────────────────────────────────────
	trackedList, err := mkcpTrackedFiles(root)
	if err != nil {
		return err
	}
	tracked := map[string]bool{}
	for _, t := range trackedList {
		tracked[t] = true
	}
	type noRow struct {
		line int
		rel  string
		why  string
	}
	var noRows []noRow
	for _, r := range rows {
		if r.pub == "no" {
			noRows = append(noRows, noRow{r.line, r.rel, r.why})
		}
	}
	if len(noRows) == 0 {
		fmt.Fprintln(stderrW, "✗ 标记表里一条 `public=no` 都没有 ⇒ R2 无区分度（空转 = 假覆盖）⇒ 拒绝给结论（rc=2）")
		return mkcpSilent{}
	}
	type r2item struct {
		line int
		rel  string
		miss string
		why  string
		isc  bool
	}
	var r2a, r2b, r2c []r2item
	trackedNo := 0
	for _, nr := range noRows {
		if tracked[nr.rel] {
			trackedNo++
		}
	}
	_ = trackedNo
	for _, nr := range noRows {
		sideA := mkcpExcluded(nr.rel, excludes)
		sideB := mkcpContains(dropExact, nr.rel)
		if !sideB {
			for _, p := range dropPrefix {
				if strings.HasPrefix(nr.rel, p) {
					sideB = true
					break
				}
			}
		}
		miss := ""
		if !sideA && !sideB {
			miss = "缺侧甲 + 侧乙（两器都会发它）"
		} else if !sideA {
			miss = "缺侧甲（旧器闸没收 ⇒ 两器共用那把闸都放行）"
		} else if !sideB {
			miss = "缺侧乙（镜像器自带表没登记）"
		}
		isCand := mkcpPublishedPath(nr.rel, entries)
		if tracked[nr.rel] && !sideA && isCand {
			r2a = append(r2a, r2item{nr.line, nr.rel, miss, nr.why, isCand})
		} else if !tracked[nr.rel] && miss != "" {
			r2b = append(r2b, r2item{nr.line, nr.rel, miss, nr.why, isCand})
		}
		if miss != "" && sideA != sideB {
			only := "侧乙有、侧甲无"
			if sideA {
				only = "侧甲有、侧乙无"
			}
			r2c = append(r2c, r2item{nr.line, nr.rel, "只在一侧：" + only, nr.why, isCand})
		}
	}

	// ── 报告 ─────────────────────────────────────────────────────────────────
	fmt.Fprintf(out, "标记表：%s（%d 行 · public=yes %d · public=no %d · 域 %d 个 · 顶层件 %d 件）\n",
		table, len(rows), len(rows)-len(noRows), len(noRows), len(aDoms), len(topFiles))
	fmt.Fprintf(out, "门覆盖面：%s（script_globs %d 条 · 域 %d 个 · 空指 glob %d 条）\n",
		config, len(globs), len(bDoms), len(emptyGlobs))
	fmt.Fprintf(out, "\n══ R1 域对拍（域 = `scripts/<域>/` 的第二段）══\n")
	fmt.Fprintf(out, "  表侧域（%d）：%s\n", len(aDoms), mkcpJoinOr(aSorted, " · ", "（空）"))
	bsorted := []string{}
	for d := range bDoms {
		bsorted = append(bsorted, d)
	}
	sort.Strings(bsorted)
	bshow := make([]string, len(bsorted))
	for i, d := range bsorted {
		if d == mkcpTop {
			bshow[i] = "顶层"
		} else {
			bshow[i] = d
		}
	}
	fmt.Fprintf(out, "  门侧域（%d）：%s\n", len(bDoms), strings.Join(bshow, " · "))
	fmt.Fprintf(out, "  %s R1-a 门扫的域、表里一件都没登记：%d 个%s\n",
		mkcpMark(len(r1a) > 0), len(r1a), mkcpSuffixList(r1a))
	fmt.Fprintf(out, "  %s R1-b 表登记的多余域（未登记豁免）：%d 个%s\n",
		mkcpMark(len(r1b) > 0), len(r1b), mkcpSuffixList(r1b))
	fmt.Fprintf(out, "  · 登记豁免域（不入退码 · 逐条写在 EXEMPT_DOMAINS）：%d 个%s\n",
		len(exemptMap), mkcpSuffixListKeys(exemptMap))
	if len(exemptDangle) > 0 {
		fmt.Fprintf(out, "  · 悬空豁免（照报 · 不改退码）：%d 个　%s\n", len(exemptDangle), strings.Join(exemptDangle, " · "))
	}
	if len(emptyGlobs) > 0 {
		fmt.Fprintf(out, "  · 空指 glob（声明了覆盖面却一件都匹配不到 ⇒ 不计进 B）：%d 条　%s\n",
			len(emptyGlobs), strings.Join(emptyGlobs, " · "))
	}
	if len(gap) > 0 {
		fmt.Fprintf(out, "  · 覆盖缺口（**只报数 · 不入退码**）：门侧 %d 个域不在 `check-public-marks.py` 的 9 域面（DOMAINS）"+
			"内 ⇒ 该域件的「漏登记」今天无人管：%s\n", len(gap), strings.Join(gap, " · "))
	}
	if len(r1a) > 0 {
		items := []string{}
		for _, d := range r1a {
			items = append(items, fmt.Sprintf("%s  ← `script_globs` 有 `scripts/%s/*.sh|*.py`（实测 %d 件），`公开标记.tsv` 里 0 行",
				d, d, bCounts[d]))
		}
		dump("R1-a 点名（门扫这个域、表里一件都没登记 ⇒ 补表 或 从 globs 撤掉该域 · 不许放宽本判据 ✗）", items, "")
	}
	if len(r1b) > 0 {
		items := []string{}
		for _, d := range r1b {
			items = append(items, fmt.Sprintf("%s  ← 表里 %d 行 `scripts/%s/…`，`script_globs` 里 0 条", d, len(aDoms[d]), d))
		}
		dump("R1-b 点名（表登记了这个域、门不扫它 ⇒ 补 `script_globs` 或按域豁免登记表写明为什么）", items, "")
	}

	fmt.Fprintf(out, "\n══ R2 两器排除面（tsv `public=no` %d 行 · 已跟踪 %d · 未在索引 %d）══\n",
		len(noRows), trackedNo, len(noRows)-trackedNo)
	fmt.Fprintf(out, "  侧甲 = `%s` 的 EXCLUDES（%d 条 · 两器共用那把闸）\n", mkcpPublishShRel, len(excludes))
	fmt.Fprintf(out, "  侧乙 = `%s` 的 DROP_EXACT %d 条 + DROP_PREFIX %d 条（镜像器自带表）\n",
		mkcpMirrorLibRel, len(dropExact), len(dropPrefix))
	fmt.Fprintf(out, "  %s R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：%d 件\n", mkcpMark(len(r2a) > 0), len(r2a))
	fmt.Fprintf(out, "  · R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：%d 件\n", len(r2b))
	fmt.Fprintf(out, "  · R2-c 两器清单漂移（只在一侧 · 不入退码）：%d 件\n", len(r2c))
	if len(r2a) > 0 {
		items := []string{}
		for _, x := range r2a {
			items = append(items, fmt.Sprintf("%s（表第 %d 行）· %s", x.rel, x.line, x.miss))
		}
		dump("R2-a 点名（补两器排除表 或 改表 · 不许放宽本判据凑绿 ✗）", items, "")
	}
	if len(r2b) > 0 {
		items := []string{}
		for _, x := range r2b {
			tail := ""
			if !x.isc {
				tail = "　★ 且不在导出白名单候选集内（真入库也触不到 ⑥）"
			}
			items = append(items, fmt.Sprintf("%s（表第 %d 行）· %s%s ⇒ 入库那一刻判据⑥ 报「表说 no、链上却仍会发」",
				x.rel, x.line, x.miss, tail))
		}
		dump("R2-b 入库前预警点名（这条件**一 `git add` 就会让判据⑥ 红**；入库前先补两器排除表）", items, "")
	}
	if len(r2c) > 0 {
		items := []string{}
		for _, x := range r2c {
			items = append(items, fmt.Sprintf("%s（表第 %d 行）· %s", x.rel, x.line, x.miss))
		}
		dump("R2-c 点名（两器清单必须逐条一致；有效判定仍安全 ⇒ 只报）", items, "")
	}

	bad := len(r1a) + len(r1b) + len(r2a)
	fmt.Fprintf(out, "\n合计：R1-a %d · R1-b %d · R2-a %d（以上判红）· R2-b 预警 %d · R2-c 漂移 %d（以上只报）\n",
		len(r1a), len(r1b), len(r2a), len(r2b), len(r2c))
	if bad != 0 {
		fmt.Fprintf(out, "⇒ 违规 %d 处（rc=1）：① R1-a ⇒ 补表（附理由）或撤 glob；② R1-b ⇒ 补 glob 或走域豁免登记；\n"+
			"   ③ R2-a ⇒ 收进 `EXCLUDES` + `DROP_EXACT`，或**改表** —— **不许**放宽本判据来凑绿 ✗\n", bad)
		return mkcpFail{}
	}
	fmt.Fprintf(out, "⇒ 全绿（rc=0）：域两侧逐域对齐（豁免 %d 个已登记）· 已跟踪的 `no` 件两器都已排除 · "+
		"未入库件预警 %d 条（只报）\n", len(exemptMap), len(r2b))
	return nil
}

type mkcpSilent struct{}
type mkcpFail struct{}

func (mkcpSilent) Error() string { return "" }
func (mkcpFail) Error() string   { return "" }

func mkcpLoadMarksRows(table string) ([]mkcpRow, error) {
	if !mkcpIsFile(table) {
		return nil, mkcpUnsupported{"标记表不在：" + table}
	}
	rows, err := mkcpLoadTable(table)
	if err != nil {
		if te, ok := err.(mkcpTableError); ok {
			return nil, mkcpUnsupported{"标记表不合规：" + te.msg}
		}
		return nil, mkcpUnsupported{err.Error()}
	}
	return rows, nil
}

// ── 入口 ─────────────────────────────────────────────────────────────────────────
var stderrW io.Writer = os.Stderr

func checkMarksCoverageParityGateMain(stdout, stderr io.Writer, argv []string) int {
	stderrW = stderr
	// Q-176：本门无机器读面 ⇒ 给了 --json 不许沉默（在 argparse 之前判）
	if rc, hit := mkcpRefuseJSON(argv); hit != "" {
		fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", hit)
		fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
		fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
		return rc
	}
	args, rc := mkcpParseArgs(stdout, stderr, argv)
	if rc != 0 {
		return rc
	}
	if args.help {
		mkcpPrintHelp(stdout, args.prog)
		return 0
	}
	if args.selftest {
		fmt.Fprintln(stdout, "── check-marks-coverage-parity.py 自证（四档 · 含 must-fail 负控 · 仓外夹具）")
		return mkcpSelfTest(stdout, stderr)
	}
	root := mkcpAbsPath(args.root)
	code := mkcpRun(stdout, root, args.table, args.config, args.maxExamples)
	if code == 2 {
		return 2
	}
	return code
}

func mkcpRefuseJSON(argv []string) (int, string) {
	var hits []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hits = append(hits, x)
		}
	}
	if len(hits) == 0 {
		return 0, ""
	}
	parts := make([]string, len(hits))
	for i, h := range hits {
		parts[i] = "`" + h + "`"
	}
	return 2, strings.Join(parts, " · ")
}

type mkcpArgs struct {
	prog        string
	root        string
	table       string
	config      string
	maxExamples int
	selftest    bool
	help        bool
}

func mkcpParseArgs(stdout, stderr io.Writer, argv []string) (mkcpArgs, int) {
	repo := mkcpRepoRoot()
	a := mkcpArgs{root: repo, maxExamples: 40}
	prog := "check-marks-coverage-parity.py"
	i := 0
	if len(argv) > 0 {
		prog = filepath.Base(argv[0])
		i = 1
	}
	a.prog = prog
	var unknown []string
	need := func(name string, j int) (string, bool) {
		if j+1 >= len(argv) {
			fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--table TABLE] [--config CONFIG]\n"+
				"                      [--max-examples MAX_EXAMPLES] [--self-test]\n", prog)
			fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", prog, name)
			return "", false
		}
		return argv[j+1], true
	}
	for ; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			a.help = true
		case x == "--self-test" || x == "--selftest":
			a.selftest = true
		case x == "--root":
			v, ok := need("--root", i)
			if !ok {
				return a, 2
			}
			a.root = v
			i++
		case strings.HasPrefix(x, "--root="):
			a.root = x[len("--root="):]
		case x == "--table":
			v, ok := need("--table", i)
			if !ok {
				return a, 2
			}
			a.table = v
			i++
		case strings.HasPrefix(x, "--table="):
			a.table = x[len("--table="):]
		case x == "--config":
			v, ok := need("--config", i)
			if !ok {
				return a, 2
			}
			a.config = v
			i++
		case strings.HasPrefix(x, "--config="):
			a.config = x[len("--config="):]
		case x == "--max-examples":
			v, ok := need("--max-examples", i)
			if !ok {
				return a, 2
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--table TABLE] [--config CONFIG]\n"+
					"                      [--max-examples MAX_EXAMPLES] [--self-test]\n", prog)
				fmt.Fprintf(stderr, "%s: error: argument --max-examples: invalid int value: '%s'\n", prog, v)
				return a, 2
			}
			a.maxExamples = n
			i++
		case strings.HasPrefix(x, "--max-examples="):
			n, err := strconv.Atoi(x[len("--max-examples="):])
			if err != nil {
				fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--table TABLE] [--config CONFIG]\n"+
					"                      [--max-examples MAX_EXAMPLES] [--self-test]\n", prog)
				fmt.Fprintf(stderr, "%s: error: argument --max-examples: invalid int value: '%s'\n", prog, x[len("--max-examples="):])
				return a, 2
			}
			a.maxExamples = n
		default:
			unknown = append(unknown, x)
		}
	}
	if len(unknown) > 0 {
		mkcpPrintUsage(stderr, prog)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", prog, strings.Join(unknown, " "))
		return a, 2
	}
	return a, 0
}

func mkcpPrintUsage(w io.Writer, prog string) {
	fmt.Fprintf(w, "usage: %s [-h] [--root ROOT] [--table TABLE] [--config CONFIG]\n"+
		"                      [--max-examples MAX_EXAMPLES] [--self-test]\n", prog)
}

func mkcpPrintHelp(w io.Writer, prog string) {
	mkcpPrintUsage(w, prog)
	fmt.Fprintf(w, "\n标记表域 ⟷ 门覆盖面域 对拍 + 入库前两器排除预警（只读一致性门）\n\n")
	fmt.Fprintf(w, "optional arguments:\n")
	fmt.Fprintf(w, "  -h, --help            show this help message and exit\n")
	fmt.Fprintf(w, "  --root ROOT           树根（默认 = 仓根）\n")
	fmt.Fprintf(w, "  --table TABLE         标记表（默认 = <root>/%s）\n", mkcpTableRel)
	fmt.Fprintf(w, "  --config CONFIG       门覆盖 config（默认 = <root>/%s）\n", mkcpConfigRel)
	fmt.Fprintf(w, "  --max-examples MAX_EXAMPLES\n")
	fmt.Fprintf(w, "  --self-test, --selftest\n                        只跑成对负控自检（合成夹具，不碰真目标）\n")
}

// ── 基础工具 ─────────────────────────────────────────────────────────────────────
func mkcpRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := wd
	for {
		if mkcpIsFile(filepath.Join(d, mkcpTableRel)) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return wd
		}
		d = parent
	}
}

func mkcpAbsPath(p string) string {
	if p == "" {
		return p
	}
	if !filepath.IsAbs(p) {
		if wd, err := os.Getwd(); err == nil {
			p = filepath.Join(wd, p)
		}
	}
	return filepath.Clean(p)
}

func mkcpIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func mkcpIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func mkcpExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func mkcpHasDotDotSegment(rel string) bool {
	for _, s := range strings.Split(rel, "/") {
		if s == ".." {
			return true
		}
	}
	return false
}

func mkcpContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func mkcpHasSuffix(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func mkcpEqStrs(a, b []string) bool {
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

func mkcpMark(bad bool) string {
	if bad {
		return "✗"
	}
	return "✅"
}

func mkcpJoinOr(list []string, sep, empty string) string {
	if len(list) == 0 {
		return empty
	}
	return strings.Join(list, sep)
}

func mkcpSuffixList(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return "　" + strings.Join(list, " · ")
}

func mkcpSuffixListKeys(m map[string]int) string {
	if len(m) == 0 {
		return "（无）"
	}
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "　" + strings.Join(keys, " · ")
}

// mkcpPyReprStr 对应 py `%r` 对 str：单引号包裹（本件只用在 ASCII 路径/条目上）。
func mkcpPyReprStr(s string) string {
	return "'" + s + "'"
}

func mkcpPyReprAny(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return "'" + t + "'"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case []interface{}:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = mkcpPyReprAny(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func mkcpPyListRepr(list []string) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// mkcpFnmatch：`*` 通配（不跨 `/`）—— glob 面只用 `*<ext>` 形态。
func mkcpFnmatch(name, pat string) bool {
	if pat == "*" {
		return true
	}
	if !strings.Contains(pat, "*") {
		return name == pat
	}
	parts := strings.Split(pat, "*")
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}
	rest := name[len(parts[0]):]
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		idx := strings.Index(rest, p)
		if idx < 0 {
			return false
		}
		if i == len(parts)-1 && !strings.HasSuffix(rest, p) {
			return false
		}
		rest = rest[idx+len(p):]
	}
	return true
}
