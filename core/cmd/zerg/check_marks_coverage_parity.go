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
	mkcpTableRel     = pathMarksTable
	mkcpConfigRel    = pathGateCoverageCfg
	mkcpExcludesRel  = pathPublishExcludes
	mkcpMirrorLibRel = pathMirrorLibPy
	mkcpWhitelistRel = pathPublishWhitelist
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
// mkcpLoadExcludes：读行式真源 `publish/excludes.txt`（切源 2026-10-08 · `GAP-20260926-223` 账[3]）。
// 行口径 = 唯一 Go 解析件 `excludes_rules.go::parseExcludesRules`（strip 传本件原 `strings.TrimSpace`）。
// 缺件 / 读不到 ⇒ 空切片（调用方据 len==0 fail-closed）。
func mkcpLoadExcludes(path string) []string {
	if !mkcpExists(path) {
		return []string{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	return parseExcludesRules(strings.Split(string(b), "\n"), strings.TrimSpace)
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
	return excludesRulesMatch(rel, ex)
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

// ══════════════════════════════════════════════════════════════════════════════
// 自检（py `self_test` L492 逐格同构 · 四档 18 格 · 仓外夹具 · 进程内跑真入口）
//   · py 走子进程跑真命令行（`_run`）；Go 进程内调 `mkcpRun`（**同一入口**）—— stdout/stderr
//     **拼接序**（stdout 在前）与 py `_run` 的 `proc.stdout + proc.stderr` 逐字同。
//   · 夹具全落**仓外**临时目录（`os.MkdirTemp`），跑完 `defer os.RemoveAll` 自回收（A 类：不泄漏）。
// ══════════════════════════════════════════════════════════════════════════════

const mkcpSelfWantCases = 18

const mkcpSelfTableHead = "文件	public	理由\n"

// 夹具侧甲：排除表新真源（行式 · 裸条目；解析器 = 唯一 Go 件 excludes_rules.go / loadExcludesTable）
const mkcpSelfExcludesHead = "# 夹具：最小排除表（`publish/excludes.txt` 行式真源 · 裸条目）\n"

// 夹具侧乙：最小镜像器（只提供本门读的两个名字）
const mkcpSelfMirrorLibHead = "# 夹具：最小镜像器（只提供本门读的两个名字）\n" +
	"DROP_PREFIX = ()\nDROP_EXACT = {"

// mkcpSelfTmpDirs —— 自检建出的全部临时根（跑完逐个 RemoveAll，✗ 不泄漏）。
var mkcpSelfTmpDirs []string

func mkcpSelfMkroot(base, name string) string {
	d, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return ""
	}
	mkcpSelfTmpDirs = append(mkcpSelfTmpDirs, d)
	return d
}

func mkcpSelfWrite(path, text string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(text), 0o644)
}

// mkcpSelfRun —— 进程内跑真入口（等价 py `_run` 的子进程档）；⇒ (rc, stdout+stderr)。
func mkcpSelfRun(root string) (int, string) {
	var so, se bytes.Buffer
	old := stderrW
	stderrW = &se
	rc := mkcpRun(&so, mkcpAbsPath(root), "", "", 40)
	stderrW = old
	return rc, so.String() + se.String()
}

// mkcpSelfFixture —— py `_fixture` L444 同构：造一棵最小树（表 + config + 两器排除表 + 白名单 + 盘上件）。
func mkcpSelfFixture(base, name string, tsvRows []string, globs, files, excludes, dropExact []string, whitelist string, git bool, add []string) string {
	root := mkcpSelfMkroot(base, name)
	if root == "" {
		return ""
	}
	mkcpSelfWrite(filepath.Join(root, mkcpTableRel), mkcpSelfTableHead+strings.Join(tsvRows, ""))
	var cb strings.Builder
	cb.WriteString(`{"schema":"gate-coverage/1","baseline":0,"scan":{"script_globs":[`)
	for i, g := range globs {
		if i > 0 {
			cb.WriteByte(',')
		}
		b, _ := json.Marshal(g)
		cb.Write(b)
	}
	cb.WriteString(`]},"whitelist":{"build_dirs":[],"scripts":[]}}`)
	mkcpSelfWrite(filepath.Join(root, mkcpConfigRel), cb.String())
	var exb strings.Builder
	for _, e := range excludes {
		exb.WriteString(e + "\n")
	}
	mkcpSelfWrite(filepath.Join(root, mkcpExcludesRel), mkcpSelfExcludesHead+exb.String())
	var db strings.Builder
	for i, d := range dropExact {
		if i > 0 {
			db.WriteString(", ")
		}
		db.WriteString("\"" + d + "\"")
	}
	mkcpSelfWrite(filepath.Join(root, mkcpMirrorLibRel), mkcpSelfMirrorLibHead+db.String()+"}\n")
	mkcpSelfWrite(filepath.Join(root, mkcpWhitelistRel), whitelist)
	for _, rel := range files {
		mkcpSelfWrite(filepath.Join(root, rel), "#!/usr/bin/env python3\nprint('fixture')\n")
	}
	if git {
		c := exec.Command("git", "init", "-q")
		c.Dir = root
		_ = c.Run()
		for _, rel := range add {
			a := exec.Command("git", "add", "-f", rel)
			a.Dir = root
			_ = a.Run()
		}
	}
	return root
}

// mkcpSelfTest —— py `self_test`(L492) 逐格同构：正控 ＋ 逐档负控 ＋ 防空绿 ＋ 退码 0-1-2 钳 ＋ 真目标。
func mkcpSelfTest(stdout, stderr io.Writer) int {
	d := mkcpSelfMkroot("", "mcp-selftest")
	if d == "" {
		fmt.Fprintln(stderr, "✗ 不给结论（rc=2）：自检夹具根建不出来")
		return 2
	}
	defer func() {
		for _, t := range mkcpSelfTmpDirs {
			_ = os.RemoveAll(t)
		}
		mkcpSelfTmpDirs = nil
	}()

	bad, total := 0, 0
	caseF := func(tag, desc string, wantRC int, wantText []string, root string) (int, string) {
		total++
		rc, text := mkcpSelfRun(root)
		var miss []string
		for _, s := range wantText {
			if !strings.Contains(text, s) {
				miss = append(miss, s)
			}
		}
		ok := rc == wantRC && len(miss) == 0
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		ms := "无"
		if len(miss) > 0 {
			ms = mkcpPyListRepr(miss)
		}
		fmt.Fprintf(stdout, "  %s %s %s\n       rc=%d（期望 %d）· 缺字样 %s\n", mark, tag, desc, rc, wantRC, ms)
		return rc, text
	}

	fmt.Fprintln(stdout, "── 自检（合成夹具 · 仓外临时目录 · 不碰真目标）──")

	baseExcludes := []string{"scripts/gates/excluded.py", "scripts/exportnames/plug.py"}
	devRow := "scripts/dev/lone.py	no	B 类：合成夹具（dev 域）\n"
	devGlobs := []string{"scripts/dev/*.sh", "scripts/dev/*.py"}
	baseFiles := []string{"scripts/build/kept.py", "scripts/gates/excluded.py", "scripts/exportnames/plug.py"}
	baseGlobs := []string{"scripts/*.sh", "scripts/*.py", "scripts/build/*.sh", "scripts/build/*.py", "scripts/gates/*.sh", "scripts/gates/*.py"}
	baseTSV := "scripts/build/kept.py	yes	A 类：合成夹具（构建链）\n" +
		"scripts/gates/excluded.py	no	B 类：合成夹具（门禁）\n" +
		"scripts/exportnames/plug.py	no	面外域·合成夹具（豁免登记表里那一个）\n"

	cat := func(a, b []string) []string {
		out := make([]string, 0, len(a)+len(b))
		out = append(out, a...)
		out = append(out, b...)
		return out
	}
	type fxo struct {
		tsv                          []string
		globs, files, exc, drop, add []string
		noGit                        bool
	}
	fx := func(name string, o fxo) string {
		tsv := o.tsv
		if tsv == nil {
			tsv = []string{baseTSV}
		}
		globs := o.globs
		if globs == nil {
			globs = baseGlobs
		}
		files := o.files
		if files == nil {
			files = baseFiles
		}
		exc := o.exc
		if exc == nil {
			exc = baseExcludes
		}
		drop := o.drop
		if drop == nil {
			drop = baseExcludes
		}
		add := o.add
		if add == nil {
			add = files
		}
		return mkcpSelfFixture(d, name, tsv, globs, files, exc, drop, "scripts/\n", !o.noGit, add)
	}

	fmt.Fprintln(stdout, "[正控] 域两侧对齐 + 面外域走登记豁免 + 已跟踪 no 件两侧都收 ⇒ rc=0")
	good := fx("good", fxo{})
	caseF("正控①", "两侧域对齐（豁免域 exportnames 单侧出现）⇒ 绿", 0,
		[]string{"R1-a 门扫的域、表里一件都没登记：0 个", "R1-b 表登记的多余域（未登记豁免）：0 个",
			"R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：0 件", "全绿（rc=0）"}, good)

	fmt.Fprintln(stdout, "[逐档负控] 每型一个用例 ⇒ 必须 rc=1 且**逐字点名**")
	caseF("R1-a", "门侧多一个域（dev）而表里 0 行 ⇒ 红且点名 `dev` + 两侧现值", 1,
		[]string{"R1-a 门扫的域、表里一件都没登记：1 个",
			"dev  ← `script_globs` 有 `scripts/dev/*.sh|*.py`（实测 1 件），`公开标记.tsv` 里 0 行",
			"违规 1 处（rc=1）"},
		fx("r1a", fxo{globs: cat(baseGlobs, devGlobs), files: cat(baseFiles, []string{"scripts/dev/lone.py"}), exc: cat(baseExcludes, []string{"scripts/dev/lone.py"})}))
	caseF("成对①", "把 dev 补进表 + 收进两器 ⇒ 回绿（与 R1-a 成对 · 证明不是恒红）", 0,
		[]string{"R1-a 门扫的域、表里一件都没登记：0 个", "全绿（rc=0）"},
		fx("r1a-fixed", fxo{tsv: []string{baseTSV + devRow}, globs: cat(baseGlobs, devGlobs), files: cat(baseFiles, []string{"scripts/dev/lone.py"}), exc: cat(baseExcludes, []string{"scripts/dev/lone.py"})}))
	caseF("R1-b", "表侧多一个域（zzz）且不在登记豁免表 ⇒ 红且点名 `zzz` + 表行数", 1,
		[]string{"R1-b 表登记的多余域（未登记豁免）：1 个", "zzz  ← 表里 1 行 `scripts/zzz/…`，`script_globs` 里 0 条"},
		fx("r1b", fxo{tsv: []string{baseTSV + "scripts/zzz/other.py	no	B 类：合成夹具（zzz 域）\n"}, files: cat(baseFiles, []string{"scripts/zzz/other.py"}), exc: cat(baseExcludes, []string{"scripts/zzz/other.py"})}))
	caseF("R2-a", "已跟踪的 no 件两器都缺（缺侧甲 + 侧乙）⇒ 红且点名件 + 缺哪一侧", 1,
		[]string{"R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：1 件",
			"scripts/gates/excluded.py（表第 3 行）· 缺侧甲 + 侧乙（两器都会发它）"},
		fx("r2a", fxo{exc: []string{"scripts/exportnames/plug.py"}, drop: []string{"scripts/exportnames/plug.py"}}))
	caseF("成对②", "同一棵树把该件收进侧甲 ⇒ 回绿（与 R2-a 成对）", 0,
		[]string{"R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：0 件", "全绿（rc=0）"}, fx("r2a-fixed", fxo{}))

	fmt.Fprintln(stdout, "[防空绿档] 必须「构造就能复现」，且改正后必须退回绿（证明判据有牙齿、不是恒红/恒绿）")
	caseF("防空绿①", "未跟踪件 + 两器都缺 ⇒ 点名预警（入库那一刻判据⑥ 才红的那一格）", 0,
		[]string{"R2-b 入库前预警点名", "scripts/dev/lone.py（表第 5 行）· 缺侧甲 + 侧乙（两器都会发它）",
			"⇒ 入库那一刻判据⑥ 报「表说 no、链上却仍会发」",
			"R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：1 件"},
		fx("r2b-untracked", fxo{tsv: []string{baseTSV + devRow}, globs: cat(baseGlobs, devGlobs), files: cat(baseFiles, []string{"scripts/dev/lone.py"}), add: baseFiles}))
	caseF("防空绿②", "同一棵树把该件收进两器 ⇒ 预警消失（证明预警不是恒亮）", 0,
		[]string{"R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：0 件"},
		fx("r2b-fixed", fxo{tsv: []string{baseTSV + devRow}, globs: cat(baseGlobs, devGlobs), files: cat(baseFiles, []string{"scripts/dev/lone.py"}), add: baseFiles, exc: cat(baseExcludes, []string{"scripts/dev/lone.py"}), drop: cat(baseExcludes, []string{"scripts/dev/lone.py"})}))
	caseF("防空绿③", "两侧清单漂移（侧甲有、侧乙无）⇒ 点名漂移（只报 · 不入退码）", 0,
		[]string{"R2-c 两器清单漂移（只在一侧 · 不入退码）：1 件", "只在一侧：侧甲有、侧乙无", "全绿（rc=0）"},
		fx("r2c-drift", fxo{drop: []string{"scripts/exportnames/plug.py"}}))

	fmt.Fprintln(stdout, "[退码 0-1-2 钳] 缺件 / 空转 / 豁免条目不齐 / 非 git 树 ⇒ 一律 rc=2（判不了 ≠ 绿）")
	noTbl := fx("no-table", fxo{})
	_ = os.Remove(filepath.Join(noTbl, mkcpTableRel))
	caseF("钳①", "标记表缺件 ⇒ rc=2", 2, []string{"不给结论"}, noTbl)
	noCfg := fx("no-config", fxo{})
	_ = os.Remove(filepath.Join(noCfg, mkcpConfigRel))
	caseF("钳②", "config 缺件 ⇒ rc=2", 2, []string{"config 不在"}, noCfg)
	caseF("钳③", "侧甲解析为空 ⇒ rc=2（清单没了所以全绿 = 假绿）", 2, []string{"解析不出任何排除条目"}, fx("empty-side", fxo{exc: []string{}}))
	caseF("钳④", "script_globs 为空 ⇒ rc=2（空转）", 2, []string{"scan.script_globs 不是非空列表"}, fx("bad-globs", fxo{globs: []string{}}))
	caseF("钳⑤", "非 git 树（跟踪面读不到）⇒ rc=2（缺件 ≠ 绿）", 2, []string{"git ls-files 失败"}, fx("no-git", fxo{noGit: true}))
	caseF("钳⑥", "表里一条 `public=no` 都没有 ⇒ rc=2（R2 无区分度）", 2, []string{"一条 `public=no` 都没有"},
		fx("no-no-row", fxo{tsv: []string{"scripts/build/kept.py	yes	A 类：合成夹具\n"}}))

	// 钳⑦ ×2：豁免条目缺 date / 缺 reason ⇒ rc=2（**进程内**调 run：豁免表是模块级真源）
	savedEx := mkcpExemptDomains
	for _, c := range []struct {
		label string
		ent   mkcpExempt
	}{
		{"缺 date", mkcpExempt{"exportnames", "夹具", ""}},
		{"缺 reason", mkcpExempt{"exportnames", "", "2026-09-26"}},
	} {
		mkcpExemptDomains = []mkcpExempt{c.ent}
		var ebuf bytes.Buffer
		old := stderrW
		stderrW = &ebuf
		rcIn := mkcpRun(io.Discard, mkcpAbsPath(good), "", "", 40)
		stderrW = old
		mkcpExemptDomains = savedEx
		got := ebuf.String()
		total++
		ok := rcIn == 2 && strings.Contains(got, "域豁免登记表不合规") && strings.Contains(got, c.label)
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s 钳⑦ · 豁免条目%s ⇒ rc=2 且点名（判不了 ≠ 绿）· rc=%d\n", mark, c.label, rcIn)
	}

	fmt.Fprintln(stdout, "[真目标] 现读仓内（本件推得的仓根）⇒ 结构断言（rc ∈ {0,1} · 表侧域 > 0 · 门侧域 > 0）＋ 计数照实报")
	total++
	repo := mkcpRepoRoot()
	rcReal, textReal := mkcpSelfRun(repo)
	okReal := (rcReal == 0 || rcReal == 1) && strings.Contains(textReal, "R1-a 门扫的域、表里一件都没登记")
	if !okReal {
		bad++
	}
	sumLine := "（没有合计行）"
	for _, ln := range strings.Split(textReal, "\n") {
		if strings.HasPrefix(ln, "合计：") {
			sumLine = ln
			break
		}
	}
	markReal := "✓"
	if !okReal {
		markReal = "✗"
	}
	fmt.Fprintf(stdout, "  %s 真目标 %s ⇒ rc=%d · %s\n", markReal, repo, rcReal, sumLine)
	for _, ln := range strings.Split(textReal, "\n") {
		s := strings.TrimSpace(ln)
		if strings.HasPrefix(s, "✗ R1-") || strings.HasPrefix(s, "✗ R2-") || strings.HasPrefix(s, "· R2-b") ||
			strings.HasPrefix(s, "· R2-c") || strings.HasPrefix(s, "· 覆盖缺口") || strings.HasPrefix(s, "   ← ") {
			fmt.Fprintf(stdout, "     %s\n", s)
		}
	}
	if rcReal == 1 {
		fmt.Fprintln(stdout, "     ★ 真目标现红：逐条见上 —— 本门**不改别人件来凑绿**（改判据/改表都要走拍板）")
	}

	fmt.Fprintf(stdout, "  自证用例：%d 条 · 通过 %d 条（断言用例条数 %d，少跑即失效）\n", total, total-bad, mkcpSelfWantCases)
	if total != mkcpSelfWantCases {
		fmt.Fprintf(stderr, "AssertionError: 自证用例数必须为 %d，实际 %d\n", mkcpSelfWantCases, total)
		return 2
	}
	res := fmt.Sprintf("有 %d 条断言不过（rc=1）", bad)
	if bad == 0 {
		res = "全过（rc=0）"
	}
	fmt.Fprintf(stdout, "⇒ 自检 %s\n", res)
	if bad == 0 {
		return 0
	}
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
	publishSh := filepath.Join(root, mkcpExcludesRel)
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
		fmt.Fprintf(stderrW, "✗ 侧甲：从 %s 解析不出任何排除条目 ⇒ 不给结论（rc=2）\n", publishSh)
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
	fmt.Fprintf(out, "  侧甲 = `%s`（排除表行式真源 · %d 条 · 两器共用那把闸）\n", mkcpExcludesRel, len(excludes))
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
	// ★ argv **不含程序名**（inproc 档 `gateCheckRunInproc`(family_gate_check.go:244) 逐字传 rest；
	//   同名口径见 `checkPublicMarksGateMain`「argv 不含程序名」）。旧版按 py `sys.argv` 口径吃掉
	//   argv[0] ⇒ 经 `gate check` 时**首枚真旗标被当程序名丢掉**（`--self-test` 永到不了自检、
	//   真目标照跑 ⇒ rc 随工作树漂；`--root`/`-h` 同理全废）—— 本刀修此 no-op。
	a := mkcpArgs{root: mkcpRepoRoot(), maxExamples: 40}
	prog := "check-marks-coverage-parity.py"
	a.prog = prog
	i := 0
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
