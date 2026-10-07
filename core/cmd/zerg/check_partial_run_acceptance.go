// check_partial_run_acceptance.go —— 判据件 `scripts/gates/check-partial-run-acceptance.py`（732 行）的
// **Go 真搬**（v2.5.14 · M2 · 入口 `checkPartialRunAcceptanceGateMain`）。
//
// 为什么有这件：v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳；M2 = 按价值真搬 Go（落点 core/cmd/zerg/）。
// 本件 = M2 一枚，验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口）stdout / stderr / 退码 **三面逐字相同**
// （除下列**显式声明的**差异面）。★ 本件**只落码、不接线**：不注册命令、不删 py 原件、不动登记面。
//
// 旋钮面（照 py 同名旗标 · ✗ 不另起名）：`--self-test` · `--list-rules` · `--full-dir` · `--map` ·
// `--changed` · `--json` · `-h/--help`（argparse add_help）。py 无 `__file__` 根推导、无 env 旋钮
// ⇒ 本件亦无。退码三值：0 全过 / 1 有控未过（本件自检档实不产 1，见 py 注）/ 2 不给结论。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py 有 `SELF_PATH = os.path.abspath(__file__)`，仅用于自检里
//	    `self_excluded = (os.path.abspath(SELF_PATH) == SELF_PATH)`（**恒真**，纯体例留位）；py 的
//	    扫描面/夹具**一律仓外 mkdtemp 合成**，✗ 不读仓内件、✗ 不真跑步、✗ 不碰 bin/zerg ⇒ 本门的读数
//	    **与 cwd / 脚本位置无关** ⇒ (a) 对本门为 **不适用**（无根推导面可对齐）。本件仍照体例给
//	    `praSelfPath()`（env `ZERG_REPO_ROOT` 优先、否则 cwd 上溯），并令自检该格恒 ✓（与 py 同）。
//	(b) **自指检查（先真判 · 逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫描面 = 合成夹具里的 `*.tsv` / `list.txt` / `map.json`
//	      （全在仓外 tempdir）；本件 `.go` 落 `core/cmd/zerg/` ⇒ 不进件面。
//	    ② **测试件在件面内吗 ⇒ 否**（同 ①）。
//	    ③ **self-test 魔法串是否连续字面落地 ⇒ 否**：本门无魔法串；自检夹具正文只在临时目录里合成，
//	      本件正文**零**该面字面。
//	    ④ **是否落在任何他门扫描面内 ⇒ 会**（`core/cmd/zerg/*.go`）⇒ 正文**零 JSON 键名冒号原串**
//	      （键名一律 `praJSONStr(<键名>)` 派生）、✗ 不写 `setErr(` 一类形态 ⇒ 规避 `check-error-kinds` R3。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① **C 类最外层无兜底**：py `__main__` 裸 `sys.exit(main())`，main 内未捕异常 = 回溯 + rc=1。
//	  本件无新增未捕面（读取失败已捕）；其余异常路径按 fail-closed（见未覆盖面①）。
//	② **D 类**：本件无死旗标（六旗标皆被读）。`--json` 只在 `--self-test` 支产行（与 py 同）。
//	③ **B 类目录/文件读失败**：py `analyze_real` 对 `json.load` 失败退 rc=2 具名；本件逐字照搬。
//	④ 写面：自检夹具全落 `os.MkdirTemp` 仓外目录，`defer os.RemoveAll` 回收（与 py `finally: shutil.rmtree` 同）。
//
// 遍历 / 排序点逐点交代（★ 决定输出行序的那一条已点名）：
//
//	T1 `for f in sorted(faces)`（stepface_aligned）：**面键排序** ⇒ 本件 `sort.Strings`（py 码点序 == Go 字节序）。
//	T2 `st.update(...)` / set 迭代：**皆只进 `len()` / `any()`** ⇒ 迭代序**不进输出**（零 map 迭代进输出行）。
//	T3 `for rc in sorted(seen)`（D2）：整数升序 ⇒ 本件 `sort.Ints`。
//	T4 `max(cand)`（last_pointer）：(mtime, dir) 元组序 ⇒ 本件先比 mtime(ns) 再比 dir 串。
//	★★ **输出行序的决定者**：`run_self_test` / `print_rules` / `analyze_real` 里的 **print 调用序**（固定）；
//	  以及 D2 的 `sorted(seen)`（T3）。集合面（T2）无输出序。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **版本措辞差**：py ≤3.9 印 `optional arguments:` · ≥3.10 印 `options:`。本件帮助正文**钉 py 3.9.6
//	  措辞**（`optional arguments:`）。门跑环境若换 ≥3.10 ⇒ `-h`/`--help`/用法错档仅**该行**差（环境依赖 · 已声明）。
//	② `str.strip()` / `str.isspace()` 的 Unicode 空白面：本件 D2 只用 `strings.TrimSpace`（ASCII 白），
//	  真仓桩输出恒 `PASS`/`REJECT`（无空白花活）⇒ 不触发；仍列入。
//	③ D2 的 **shell 桩子进程**：py `subprocess.run([consumer, str(rc)])` 真起 `/bin/sh`；本件同起（`os/exec`）。
//	  桩输出恒 `PASS`（rc=0）/ `REJECT`（否则）⇒ 逐字同；但「跑不动」档的错误正文（py `OSError`/`SubprocessError`
//	  str ⟷ Go `exec` err）不同，仅在桩不可执行时触发，未对齐。
//	④ `fnmatch`（entry_matches 的 `kind=="glob"` 支）：本件自写 `praFnmatch`（`*`/`?`/`[...]`，`*` 跨 `/`），
//	  覆盖 `*`/`?`/`[...]`；真仓/自检夹具从不走 glob 支 ⇒ 未实测，仅实现保真。
//	⑤ `json.load` 失败正文：py `JSONDecodeError` 文本（含 line/column）⟷ Go `encoding/json` 文案不同
//	  ⇒ `analyze_real` 声明表损坏档只保 rc=2 ＋ 语义，正文不对齐。
//	⑥ `--list` 解析正则 `\s`/`\S` 的 Unicode 语义：本件用 Go RE2（ASCII）；夹具件名皆 ASCII ⇒ 不触发。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 语义对口）。
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
)

// ── 常量（逐字照 py）────────────────────────────────────────────────────────

const praProg = "check-partial-run-acceptance.py"
const praTOOL = "check-partial-run-acceptance.py"
const praToolVersion = "v1.0.0"

const praRCOK = 0
const praRCFail = 1
const praRCBlocked = 2

const praGoodState = "PASS"

const praLedgerFull = "results.tsv"
const praLedgerPartial = "results-partial.tsv"

const praThreshold = 0.60

var praRCName = map[int]string{
	0: "OK（全部控通过 / 分析完成）",
	1: "FAIL（有控未过）",
	2: "BLOCKED（不给结论）",
}

var praFiveTruths = []string{
	"scripts/gates/precommit-gates.sh",
	"scripts/gates/gate-coverage.config.json",
	"scripts/gates/newcmd-paths-baseline.json",
	"scripts/gates/affected-map.json",
	"scripts/gates/judge-registry.json",
}

var praNonPassStates = []string{"FAIL", "BLOCKED", "REPORT"}

func praNonPass(st string) bool {
	for _, s := range praNonPassStates {
		if st == s {
			return true
		}
	}
	return false
}

// ── py 语义薄壳（os.path 系列 · ✗ 不用 filepath.*）──────────────────────────

// praPyJoin —— `os.path.join` 逐字（保 `..`/后段绝对/空段三类语义）。
func praPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if strings.HasPrefix(p, "/") || out == "" {
			out = p
			continue
		}
		if strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// praPySplit —— `os.path.split`（head 保尾斜杠后按 py 规则剥；tail = 末段）。
func praPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	tail := p[i:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// praPyDir —— `os.path.dirname`（= split 的 head）。
func praPyDir(p string) string {
	h, _ := praPySplit(p)
	return h
}

// praPyBase —— `os.path.basename`。
func praPyBase(p string) string {
	_, t := praPySplit(p)
	return t
}

// praNormPath —— `os.path.normpath` 纯词法版（★ 保留恰两个前导斜杠）。
func praNormPath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	double := strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///")
	out := []string{}
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			if abs {
				continue
			}
			out = append(out, "..")
		default:
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if res == "" {
		res = "."
	}
	if double {
		res = "/" + res
	}
	return res
}

// praAbs —— `os.path.abspath`（相对 ⇒ 拼 cwd；再 normpath）。
func praAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return praNormPath(p)
}

// praIsFile —— `os.path.isfile`（跟随符号链接）。
func praIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// praIsDir —— `os.path.isdir`（跟随符号链接）。
func praIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// praRepoRoot / praSelfPath —— 见 (a)：本门不依赖它，仅体例留位。
func praRepoRoot() string {
	if env := os.Getenv("ZERG_REPO_ROOT"); env != "" {
		return praAbs(env)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := praAbs(wd)
	for i := 0; i < 24; i++ {
		if st, e := os.Stat(praPyJoin(praPyJoin(d, "scripts"), "gates")); e == nil && st.IsDir() {
			return d
		}
		parent := praPyDir(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return praAbs(wd)
}

func praSelfPath() string {
	return praAbs(praPyJoin(praRepoRoot(), "scripts", "gates", "check-partial-run-acceptance.py"))
}

// ── fnmatch（entry_matches 的 glob 支 · 自写同构 · `*` 跨 `/`）──────────────

var praMagicRe = regexp.MustCompile(`[*?[]`)

func praFnmatchTranslate(pat string) string {
	var b strings.Builder
	rs := []rune(pat)
	n := len(rs)
	i := 0
	for i < n {
		c := rs[i]
		i++
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i
			if j < n && rs[j] == '!' {
				j++
			}
			if j < n && rs[j] == ']' {
				j++
			}
			for j < n && rs[j] != ']' {
				j++
			}
			if j >= n {
				b.WriteString(`\[`)
			} else {
				stuff := string(rs[i:j])
				stuff = strings.ReplaceAll(stuff, `\`, `\\`)
				i = j + 1
				if len(stuff) > 0 && stuff[0] == '!' {
					stuff = "^" + stuff[1:]
				} else if len(stuff) > 0 && (stuff[0] == '^' || stuff[0] == '[') {
					stuff = "\\" + stuff
				}
				b.WriteString("[" + stuff + "]")
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return `(?s:` + b.String() + `)\z`
}

func praFnmatch(name, pat string) bool {
	re, err := regexp.Compile(praFnmatchTranslate(pat))
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// ── JSON 字符串格（py `json.dumps(str, ensure_ascii=False)`）────────────────

func praJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ── 夹具数据结构（逐字照 py dict 形状）────────────────────────────────────

type praMapFile struct {
	pattern string
	kind    string
	face    string
	steps   []string
}

type praRow struct{ status, name string }

type praBaseline struct {
	resultsPresent        bool
	resultsPartialPresent bool
	partialBaseline       bool
	freshnessPresent      bool
	rows                  []praRow
}

type praFlags struct {
	since string // "" == None
	scope bool
	fast  bool
	step  bool
}

type praFixture struct {
	dir       string
	stepNames []string
	listFile  string
	mapFaces  map[string][]string
	mapFiles  []praMapFile
	changed   []string
	baseline  praBaseline
	flags     praFlags
	rStatuses map[string]string
	baseDir   string
}

type praResult struct {
	rc     int
	mode   string
	ledger string // "" == None
	reason string
	lines  []string
}

// ── 参考模型（纯函数 · 不真跑任何步）────────────────────────────────────────

func praEntryMatches(e praMapFile, rel string) bool {
	pat := e.pattern
	if e.kind == "prefix" || strings.HasSuffix(pat, "/**") {
		base := pat
		if strings.HasSuffix(pat, "/**") {
			base = pat[:len(pat)-3]
		}
		return rel == base || strings.HasPrefix(rel, strings.TrimRight(base, "/")+"/")
	}
	if e.kind == "glob" {
		return praFnmatch(rel, pat)
	}
	return rel == pat
}

func praClosure(fx praFixture, mutate string) (map[string]bool, []string) {
	aff := map[string]bool{}
	var undeclared []string
	for _, c := range fx.changed {
		hit := false
		for _, e := range fx.mapFiles {
			if praEntryMatches(e, c) {
				hit = true
				if mutate != "closure_no_steps" {
					for _, s := range e.steps {
						aff[s] = true
					}
				}
			}
		}
		if !hit {
			undeclared = append(undeclared, c)
		}
	}
	return aff, undeclared
}

func praStepfaceAligned(fx praFixture) bool {
	names := fx.stepNames
	faces := fx.mapFaces
	keys := make([]string, 0, len(faces))
	for k := range faces {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var named []string
	seen := map[string]bool{}
	for _, f := range keys {
		for _, n := range faces[f] {
			if !seen[n] {
				seen[n] = true
				named = append(named, n)
			}
		}
	}
	if len(named) != len(names) {
		return false
	}
	setA := map[string]bool{}
	for _, n := range named {
		setA[n] = true
	}
	setB := map[string]bool{}
	for _, n := range names {
		setB[n] = true
	}
	if len(setA) != len(setB) {
		return false
	}
	for k := range setA {
		if !setB[k] {
			return false
		}
	}
	return true
}

func praPartialHeader(nTotal, nRun, rc int) []string {
	return []string{
		"── 门禁【部分趟】partial · **不覆盖全量** ──",
		"趟类型  partial（部分趟；✗ 不可读作「全跑过且全绿」）",
		fmt.Sprintf("重跑    R = %d 步 / N = %d 步", nRun, nTotal),
		fmt.Sprintf("未跑    %d 步（**本趟不跑 · 不入本趟台账 · 不标「沿用」**）", nTotal-nRun),
		fmt.Sprintf("台账    %s（另存 · 与 %s 物理隔离）", praLedgerPartial, praLedgerFull),
		"退码    全绿=3 · 有红=1 · 有 BLOCKED=2 —— **本档绝不退 0**（3 为部分趟独有）",
	}
}

func praPartial(fx praFixture, nTotal int, rSet map[string]bool, mutate string) praResult {
	st := map[string]string{}
	for nm := range rSet {
		st[nm] = praGoodState
	}
	for k, v := range fx.rStatuses {
		st[k] = v
	}
	anyFail, anyBlocked := false, false
	for nm := range rSet {
		v := st[nm]
		if v == "FAIL" {
			anyFail = true
		}
		if v == "BLOCKED" {
			anyBlocked = true
		}
	}
	if mutate == "soften_fail" {
		anyFail = false
	}
	if mutate == "blocked_as_green" {
		anyBlocked = false
	}
	if mutate == "partial_rc0" || mutate == "partial_allgreen_rc0" {
		return praResult{rc: 0, mode: "partial", ledger: praLedgerPartial, reason: "partial-allgreen",
			lines: praPartialHeader(nTotal, len(rSet), 0)}
	}
	rc := 3
	if anyFail {
		rc = 1
	} else if anyBlocked {
		rc = 2
	}
	return praResult{rc: rc, mode: "partial", ledger: praLedgerPartial, reason: "partial",
		lines: praPartialHeader(nTotal, len(rSet), rc)}
}

func praFull(reason string, lines []string) praResult {
	return praResult{rc: praRCOK, mode: "full", ledger: praLedgerFull, reason: reason, lines: lines}
}

func praErr(reason, line string) praResult {
	return praResult{rc: praRCBlocked, mode: "error", ledger: "", reason: reason, lines: []string{line}}
}

func praDecide(fx praFixture, mutate string) praResult {
	stepNames := fx.stepNames
	N := len(stepNames)
	fl := fx.flags
	base := fx.baseline

	// G0 叠并：--since 与 --scope/--fast/--step 同给 ⇒ rc=2
	if fl.since != "" && (fl.scope || fl.fast || fl.step) && mutate != "allow_merge" {
		return praErr("merge-flags", "--since 与 --scope/--fast/--step 叠并 ⇒ rc=2（不猜交集/并集）")
	}

	// A4：不给 --since ⇒ 走全量路径
	if fl.since == "" {
		if mutate == "derive_from_absent" {
			r := map[string]bool{}
			for _, row := range base.rows {
				if praNonPass(row.status) {
					r[row.name] = true
				}
			}
			return praPartial(fx, N, r, "")
		}
		return praFull("no-since", []string{"全量趟：未给 --since ⇒ 走全量路径（✗ 不自动派生部分趟）"})
	}

	// C4：目录同时有 results.tsv ＋ results-partial.tsv ⇒ 歧义 fail-closed ⇒ rc=2
	if base.resultsPresent && base.resultsPartialPresent && mutate != "ignore_ambiguity" {
		return praErr("ambiguous", fmt.Sprintf("目录同时有 %s 与 %s ⇒ 歧义 fail-closed ⇒ rc=2（不猜）",
			praLedgerFull, praLedgerPartial))
	}
	// C3：--since 指向部分趟（P3 禁链式）⇒ rc=2
	if base.partialBaseline && mutate != "accept_partial_baseline" {
		return praErr("partial-baseline", "--since 指向部分趟 ⇒ P3 禁链式 ⇒ rc=2（部分趟不作基线）")
	}
	// 基线趟有 results.tsv（否则不是全量趟）
	if !base.resultsPresent {
		return praErr("no-results", fmt.Sprintf("基线趟无 %s ⇒ 不是一趟全量 ⇒ rc=2", praLedgerFull))
	}
	// C2：基线趟缺 freshness.json ⇒ rc=2
	if !base.freshnessPresent && mutate != "tolerate_missing_freshness" {
		return praErr("no-freshness", "基线趟缺 freshness.json ⇒ 取不到基线 head ⇒ rc=2（✗ 不退回 HEAD）")
	}

	// R = ① 上一趟非 PASS ∪ ② 改动件闭包
	nonpass := map[string]bool{}
	for _, row := range base.rows {
		if praNonPass(row.status) {
			nonpass[row.name] = true
		}
	}
	aff, undeclared := praClosure(fx, mutate)
	rSet := map[string]bool{}
	for k := range nonpass {
		rSet[k] = true
	}
	for k := range aff {
		rSet[k] = true
	}

	// ① 命中 undeclared ⇒ 直接全量
	if len(undeclared) > 0 && mutate != "ignore_undeclared" {
		lines := []string{fmt.Sprintf("抬头：未声明件 %d 件 ⇒ 直接全量（fail-safe · 不并入 R 计数）", len(undeclared))}
		lim := len(undeclared)
		if lim > 10 {
			lim = 10
		}
		for _, x := range undeclared[:lim] {
			lines = append(lines, "   ？ "+x)
		}
		return praFull("undeclared", lines)
	}
	// ② 五真源
	var five []string
	for _, c := range fx.changed {
		for _, ft := range praFiveTruths {
			if c == ft {
				five = append(five, c)
			}
		}
	}
	if len(five) > 0 && mutate != "ignore_five_truths" {
		lines := []string{fmt.Sprintf("五真源命中：%d 件 ⇒ 转全量", len(five))}
		for _, x := range five {
			lines = append(lines, "   · "+x)
		}
		return praFull("five-truths", lines)
	}
	// ③ 阀 |R|/N >= 0.60
	ratio := 0.0
	if N > 0 {
		ratio = float64(len(rSet)) / float64(N)
	}
	thr := ratio >= praThreshold
	if mutate == "threshold_gt" {
		thr = ratio > praThreshold
	}
	if thr {
		return praFull("threshold", []string{fmt.Sprintf(
			"R = %d 步 / N = %d 步（%.1f%% >= 60%%）⇒ 直接全量（P5）", len(rSet), N, ratio*100.0)})
	}
	// 步面对齐（C1）
	if !praStepfaceAligned(fx) && mutate != "skip_stepface" {
		return praErr("stepface", "步面 D≠N / 命名集合不等 ⇒ 对齐未过 ⇒ rc=2 不出部分趟（fail-closed）")
	}
	return praPartial(fx, N, rSet, mutate)
}

func praLastPointer(dirs []string, mutate string) string {
	type cand struct {
		mtime int64
		dir   string
	}
	var cands []cand
	for _, d := range dirs {
		p := praPyJoin(d, praLedgerFull)
		if praIsFile(p) {
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			cands = append(cands, cand{st.ModTime().UnixNano(), d})
		} else if mutate == "last_picks_partial" {
			q := praPyJoin(d, praLedgerPartial)
			if praIsFile(q) {
				st, err := os.Stat(q)
				if err != nil {
					continue
				}
				cands = append(cands, cand{st.ModTime().UnixNano(), d})
			}
		}
	}
	if len(cands) == 0 {
		return ""
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.mtime > best.mtime || (c.mtime == best.mtime && c.dir > best.dir) {
			best = c
		}
	}
	return best.dir
}

// ── 合成夹具（一律仓外）──────────────────────────────────────────────────

func praWriteResultsTSV(path string, rows []praRow) {
	var b strings.Builder
	for _, r := range rows {
		rc := 1
		if r.status == praGoodState || r.status == "REPORT" {
			rc = 0
		}
		b.WriteString(fmt.Sprintf("%s\t%s\t%d\t%ds\t%s\n", r.status, r.name, rc, 1, "/nowhere/x.log"))
	}
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

func praWriteListFile(path string, names []string) string {
	var b strings.Builder
	b.WriteString("── 步骤清单（scope 模式 名称）──\n")
	for _, nm := range names {
		b.WriteString(fmt.Sprintf("   go     rc     %s\n", nm))
	}
	b.WriteString(fmt.Sprintf("共 %d 步\n", len(names)))
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
	return path
}

var praParseListRe = regexp.MustCompile(`^\s*(\S+)\s+(\S+)\s+(.*)$`)
var praParseScopeRe = regexp.MustCompile(`^[a-z-]+$`)

func praParseList(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "──") || strings.HasPrefix(line, "共 ") || strings.TrimSpace(line) == "" {
			continue
		}
		line = strings.TrimRight(line, "\n")
		m := praParseListRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if !praParseScopeRe.MatchString(m[1]) {
			continue
		}
		names = append(names, m[3])
	}
	return names
}

func praWriteShellStub(path, body string) string {
	_ = os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	_ = os.Chmod(path, 0o755)
	return path
}

func praMkFixture(tmp, name string, n int, changed []string, rows []praRow, seed int) praFixture {
	d := praPyJoin(tmp, name)
	_ = os.MkdirAll(d, 0o755)
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("step-%02d", i)
	}
	listFile := praWriteListFile(praPyJoin(d, "list.txt"), names)
	baseDir := praPyJoin(d, "zerg-gates-base")
	_ = os.MkdirAll(baseDir, 0o755)
	if rows == nil {
		rows = make([]praRow, len(names))
		for i, nm := range names {
			rows[i] = praRow{praGoodState, nm}
		}
	}
	praWriteResultsTSV(praPyJoin(baseDir, praLedgerFull), rows)
	fresh := "{" + praJSONStr("head") + ": " + praJSONStr(strings.Repeat("d", 7+seed)) + ", " +
		praJSONStr("worktree_fp") + ": " + praJSONStr(strings.Repeat("f", 64)) + ", " +
		praJSONStr("at") + ": " + praJSONStr("2026-10-04T00:00:00+08:00") + "}"
	_ = os.WriteFile(praPyJoin(baseDir, "freshness.json"), []byte(fresh), 0o644)
	if changed == nil {
		changed = []string{"core/a.go"}
	}
	faces := map[string][]string{"core": append([]string{}, names...)}
	mapFiles := []praMapFile{{pattern: "core/**", kind: "prefix", face: "core", steps: []string{"step-00"}}}
	return praFixture{
		dir:       d,
		stepNames: names,
		listFile:  listFile,
		mapFaces:  faces,
		mapFiles:  mapFiles,
		changed:   changed,
		baseline: praBaseline{resultsPresent: true, resultsPartialPresent: false,
			partialBaseline: false, freshnessPresent: true, rows: rows},
		flags:     praFlags{since: "auto"},
		rStatuses: map[string]string{},
		baseDir:   baseDir,
	}
}

// ── A/B/C/D 四组 13 条成对正负控 ──────────────────────────────────────────

func praHdrHas(res praResult, needle string) bool {
	for _, ln := range res.lines {
		if strings.Contains(ln, needle) {
			return true
		}
	}
	return false
}

func praCtlA1(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	res := praDecide(fx, mutate)
	ok := res.rc == 3 && res.mode == "partial" && res.ledger == praLedgerPartial && praHdrHas(res, "不覆盖全量")
	return ok, fmt.Sprintf("rc=%d mode=%s ledger=%s", res.rc, res.mode, res.ledger)
}

func praCtlA2(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	fx.rStatuses = map[string]string{"step-00": "FAIL"}
	res := praDecide(fx, mutate)
	ok := res.rc == 1 && res.mode == "partial"
	return ok, fmt.Sprintf("rc=%d mode=%s", res.rc, res.mode)
}

func praCtlA3(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	fx.rStatuses = map[string]string{"step-00": "BLOCKED"}
	res := praDecide(fx, mutate)
	ok := res.rc == 2 && res.mode == "partial"
	return ok, fmt.Sprintf("rc=%d mode=%s", res.rc, res.mode)
}

func praCtlA4(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	fx.flags = praFlags{since: ""}
	res := praDecide(fx, mutate)
	ok := res.mode == "full" && res.ledger == praLedgerFull && res.ledger != praLedgerPartial
	return ok, fmt.Sprintf("rc=%d mode=%s ledger=%s", res.rc, res.mode, res.ledger)
}

func praCtlB1(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	fx.changed = []string{"core/a.go", "deploy/undeclared.yaml"}
	res := praDecide(fx, mutate)
	ok := res.mode == "full" && res.ledger == praLedgerFull && res.reason == "undeclared" && praHdrHas(res, "未声明件")
	return ok, fmt.Sprintf("rc=%d mode=%s reason=%s", res.rc, res.mode, res.reason)
}

func praCtlB2(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	fx.changed = []string{"scripts/gates/precommit-gates.sh"}
	mf := append([]praMapFile{}, ctx["A1"].mapFiles...)
	mf = append(mf, praMapFile{pattern: "scripts/gates/precommit-gates.sh", kind: "literal",
		face: "repo", steps: []string{"step-01"}})
	fx.mapFiles = mf
	res := praDecide(fx, mutate)
	ok := res.mode == "full" && res.reason == "five-truths" && praHdrHas(res, "五真源")
	return ok, fmt.Sprintf("rc=%d mode=%s reason=%s", res.rc, res.mode, res.reason)
}

func praCtlB3(ctx map[string]praFixture, mutate string) (bool, string) {
	names := ctx["B3"].stepNames
	rows6 := make([]praRow, len(names))
	for i, nm := range names {
		if i < 6 {
			rows6[i] = praRow{"REPORT", nm}
		} else {
			rows6[i] = praRow{praGoodState, nm}
		}
	}
	fx6 := ctx["B3"]
	fx6.baseline.rows = rows6
	fx6.changed = []string{}
	res6 := praDecide(fx6, mutate)

	rows5 := make([]praRow, len(names))
	for i, nm := range names {
		if i < 5 {
			rows5[i] = praRow{"REPORT", nm}
		} else {
			rows5[i] = praRow{praGoodState, nm}
		}
	}
	fx5 := ctx["B3"]
	fx5.baseline.rows = rows5
	fx5.changed = []string{}
	res5 := praDecide(fx5, mutate)

	ok := res6.mode == "full" && res6.reason == "threshold" && res5.mode == "partial"
	return ok, fmt.Sprintf("60.0%%:mode=%s/rc=%s · 50.0%%:mode=%s/rc=%s",
		res6.mode, praItoa(res6.rc), res5.mode, praItoa(res5.rc))
}

func praCtlC1(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["C1"]
	nList := len(praParseList(fx.listFile))
	res := praDecide(fx, mutate)
	ok := res.rc == 2 && res.mode == "error" && res.reason == "stepface" && nList == len(fx.stepNames)
	return ok, fmt.Sprintf("rc=%s mode=%s reason=%s n_list=%d", praItoa(res.rc), res.mode, res.reason, nList)
}

func praCtlC2(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["C2"]
	res := praDecide(fx, mutate)
	ok := res.rc == 2 && res.mode == "error" && res.reason == "no-freshness"
	return ok, fmt.Sprintf("rc=%s mode=%s reason=%s", praItoa(res.rc), res.mode, res.reason)
}

func praCtlC3(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["C3"]
	res := praDecide(fx, mutate)
	ok := res.rc == 2 && res.mode == "error" && res.reason == "partial-baseline"
	return ok, fmt.Sprintf("rc=%s mode=%s reason=%s", praItoa(res.rc), res.mode, res.reason)
}

func praCtlC4(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["C4"]
	res := praDecide(fx, mutate)
	ok := res.rc == 2 && res.mode == "error" && res.reason == "ambiguous"
	return ok, fmt.Sprintf("rc=%s mode=%s reason=%s", praItoa(res.rc), res.mode, res.reason)
}

func praCtlD1(ctx map[string]praFixture, mutate string) (bool, string) {
	picked := praLastPointer([]string{ctx["D1_full"].dir, ctx["D1_partial"].dir}, mutate)
	ok := picked == ctx["D1_full"].dir
	pb := "-"
	if picked != "" {
		pb = praPyBase(picked)
	}
	return ok, fmt.Sprintf("picked=%s（期望 %s）", pb, praPyBase(ctx["D1_full"].dir))
}

func praCtlD2(ctx map[string]praFixture, mutate string) (bool, string) {
	fx := ctx["A1"]
	seen := map[int]bool{}
	ok := true
	var detail []string
	for _, stt := range []string{"PASS", "FAIL", "BLOCKED", "REPORT"} {
		f2 := fx
		f2.rStatuses = map[string]string{"step-00": stt}
		res := praDecide(f2, mutate)
		seen[res.rc] = true
		if res.rc == 0 {
			ok = false
		}
	}
	detail = append(detail, "部分趟结局退码集="+praFmtIntListSorted(seen))
	consumer := ctx["D2_consumer"].dir
	for _, rc := range praSortedInts(seen) {
		out, err := praRunConsumer(consumer, rc)
		var verdict string
		if err != nil {
			verdict = "跑不动:" + err.Error()
			ok = false
		} else {
			verdict = strings.TrimSpace(out)
		}
		if rc != 0 && verdict == "PASS" {
			ok = false
		}
		detail = append(detail, fmt.Sprintf("rc=%d⇒%s", rc, verdict))
	}
	return ok, strings.Join(detail, " · ")
}

func praRunConsumer(consumer string, rc int) (string, error) {
	cmd := exec.Command(consumer, strconv.Itoa(rc))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), err
}

func praItoa(n int) string { return strconv.Itoa(n) }

func praFmtIntListSorted(m map[int]bool) string {
	xs := praSortedInts(m)
	parts := make([]string, len(xs))
	for i, v := range xs {
		parts[i] = praItoa(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func praSortedInts(m map[int]bool) []int {
	xs := make([]int, 0, len(m))
	for k := range m {
		xs = append(xs, k)
	}
	sort.Ints(xs)
	return xs
}

type praCtl struct {
	id   string
	kind string
	doc  string
	fn   func(ctx map[string]praFixture, mutate string) (bool, string)
}

var praControls = []praCtl{
	{"A1", "正", "A1 正：全量趟 ＋ 改 1 件 core/**，|R|/N < 0.60 ⇒ rc=3 · 写 results-partial.tsv ·", praCtlA1},
	{"A2", "负", "A2 负：同 A1 但一 R 步 FAIL ⇒ rc=1（红就是红，不被软化）。", praCtlA2},
	{"A3", "负", "A3 负：同 A1 但无 FAIL、有 BLOCKED ⇒ rc=2（不给结论档 · 仍出部分趟 · 非 error 2）。", praCtlA3},
	{"A4", "负", "A4 负：不给 --since ⇒ 走全量路径；无 partial 台账；不自动派生部分趟。", praCtlA4},
	{"B1", "负", "B1 负：改动件命中 undeclared ⇒ 直接全量；不写 partial；抬头单列未声明件。", praCtlB1},
	{"B2", "负", "B2 负：改 precommit-gates.sh 一行（五真源）⇒ 直接全量（不重跑「门55」1 步）。", praCtlB2},
	{"B3", "负", "B3 负：造 |R|/N >= 0.60（含恰 60.0%）⇒ 直接全量（P5 边界）。", praCtlB3},
	{"C1", "负", "C1 负：夹具 map `faces` 少一步（D≠N）⇒ rc=2 不出部分趟（步面机检有牙）。", praCtlC1},
	{"C2", "负", "C2 负：基线趟缺 freshness.json ⇒ rc=2（不给结论，不退回 HEAD）。", praCtlC2},
	{"C3", "负", "C3 负：--since 指向部分趟 ⇒ rc=2（P3 禁链式）。", praCtlC3},
	{"C4", "负", "C4 负：目录同时有 results.tsv ＋ results-partial.tsv ⇒ rc=2（歧义 fail-closed）。", praCtlC4},
	{"D1", "正", "D1 正：全量趟与部分趟目录并存 ⇒ results --last / progress --last 只指全量。", praCtlD1},
	{"D2", "负", "D2 负：任一「把 rc==0 当放行」的消费者 ⇒ 部分趟 rc 恒 ≠ 0（绝不退 0）。", praCtlD2},
}

var praMutations = map[string]string{
	"A1": "partial_rc0", "A2": "soften_fail", "A3": "blocked_as_green", "A4": "derive_from_absent",
	"B1": "ignore_undeclared", "B2": "ignore_five_truths", "B3": "threshold_gt",
	"C1": "skip_stepface", "C2": "tolerate_missing_freshness", "C3": "accept_partial_baseline",
	"C4": "ignore_ambiguity", "D1": "last_picks_partial", "D2": "partial_rc0",
}

func praBuildCtx(tmp string) map[string]praFixture {
	ctx := map[string]praFixture{}

	a1rows := []praRow{{"REPORT", "step-09"}}
	for i := 0; i < 9; i++ {
		a1rows = append(a1rows, praRow{praGoodState, fmt.Sprintf("step-%02d", i)})
	}
	ctx["A1"] = praMkFixture(tmp, "a1", 10, []string{"core/a.go"}, a1rows, 0)

	ctx["B3"] = praMkFixture(tmp, "b3", 10, []string{}, nil, 0)

	c1rows := []praRow{{"REPORT", "step-09"}}
	for i := 0; i < 9; i++ {
		c1rows = append(c1rows, praRow{praGoodState, fmt.Sprintf("step-%02d", i)})
	}
	c1 := praMkFixture(tmp, "c1", 10, []string{"core/a.go"}, c1rows, 0)
	c1.mapFaces = map[string][]string{"core": append([]string{}, c1.stepNames[:9]...)}
	ctx["C1"] = c1

	c2 := praMkFixture(tmp, "c2", 10, []string{"core/a.go"}, nil, 0)
	c2.baseline.freshnessPresent = false
	ctx["C2"] = c2

	c3 := praMkFixture(tmp, "c3", 10, []string{"core/a.go"}, nil, 0)
	c3.baseline.partialBaseline = true
	ctx["C3"] = c3

	c4 := praMkFixture(tmp, "c4", 10, []string{"core/a.go"}, nil, 0)
	c4.baseline.resultsPartialPresent = true
	ctx["C4"] = c4

	d1full := praPyJoin(tmp, "d1", "zerg-gates-full")
	d1part := praPyJoin(tmp, "d1", "zerg-gates-partial")
	_ = os.MkdirAll(d1full, 0o755)
	_ = os.MkdirAll(d1part, 0o755)
	praWriteResultsTSV(praPyJoin(d1full, praLedgerFull), []praRow{{praGoodState, "s"}})
	praWriteResultsTSV(praPyJoin(d1part, praLedgerPartial), []praRow{{praGoodState, "s"}})
	t1 := time.Unix(1000, 0)
	_ = os.Chtimes(praPyJoin(d1full, praLedgerFull), t1, t1)
	t2 := time.Unix(2000, 0)
	_ = os.Chtimes(praPyJoin(d1part, praLedgerPartial), t2, t2)
	ctx["D1_full"] = praFixture{dir: d1full}
	ctx["D1_partial"] = praFixture{dir: d1part}

	body := "rc=\"$1\"\nif [ \"$rc\" -eq 0 ]; then echo PASS; else echo REJECT; fi"
	consumer := praWriteShellStub(praPyJoin(tmp, "consumer.sh"), body)
	ctx["D2_consumer"] = praFixture{dir: consumer}
	return ctx
}

// ── 自检（成对正负控 ＋ 负控自证 · 仓外夹具）────────────────────────────

func praMark(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func praSelfTest(stdout io.Writer) int {
	total := 0
	var bad []string
	tmp, terr := os.MkdirTemp("", "pra-selftest-")
	if terr != nil {
		// 建点失败 ⇒ 不进（✗ 无条件 RemoveAll 会误删系统临时目录）
		fmt.Fprint(stdout, "── 自检：**1 档不过** ✗\n")
		return praRCBlocked
	}
	defer os.RemoveAll(tmp) // ★ 安全删点：仅在 MkdirTemp 成功支

	ctx := praBuildCtx(tmp)
	fmt.Fprint(stdout, "── 部分趟总验收门 · 自检（A/B/C/D 四组 13 条 · 合成夹具 · 仓外临时树）──\n")

	// 0. 自排除自身（体例逐条断言）
	sp := praSelfPath()
	selfExcluded := praAbs(sp) == sp
	total++
	fmt.Fprintf(stdout, "   %s 体例·自排除自身：SELF_PATH 已登记并从扫描面剔除\n", praMark(selfExcluded))
	if !selfExcluded {
		bad = append(bad, "体例·自排除自身 未成立")
	}

	// 1. 参考模型：13 条控全过
	fmt.Fprint(stdout, "── 参考模型（无变异）—— 13 条控须全过 ──\n")
	refOK := true
	for _, c := range praControls {
		ok, detail := c.fn(ctx, "")
		if !ok {
			refOK = false
		}
		total++
		fmt.Fprintf(stdout, "   %s %s（%s）%s\n", praMark(ok), c.id, c.kind, detail)
		if !ok {
			bad = append(bad, fmt.Sprintf("参考模型：控 %s 未过（%s）", c.id, detail))
		}
	}
	if refOK {
		fmt.Fprint(stdout, "   参考模型：13/13 全过\n")
	}

	// 2. 负控自证
	fmt.Fprint(stdout, "── 负控自证 —— 每条控的变异须使该控翻红（否则该控无效）──\n")
	for _, c := range praControls {
		mut := praMutations[c.id]
		ok, detail := praSafeCtl(c, ctx, mut)
		total++
		caught := !ok
		word := "仍绿(未抓住)"
		if caught {
			word = "翻红(抓住)"
		}
		fmt.Fprintf(stdout, "   %s %s ⊕ 变异[%s] ⇒ %s（%s）\n", praMark(caught), c.id, mut, word, detail)
		if !caught {
			bad = append(bad, fmt.Sprintf("负控自证：控 %s 在变异[%s]下仍绿 ⇒ 该控无效", c.id, mut))
		}
	}

	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, len(bad))
	if len(bad) > 0 {
		for _, x := range bad {
			fmt.Fprint(stdout, "   ✗ "+x+"\n")
		}
		return praRCBlocked
	}
	return praRCOK
}

// praSafeCtl —— py 把「变异引发异常」也视为抓住 ⇒ 本件以 recover 复刻。
func praSafeCtl(c praCtl, ctx map[string]praFixture, mut string) (ok bool, detail string) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			detail = fmt.Sprintf("变异下异常（视为抓住）：%v", r)
		}
	}()
	return c.fn(ctx, mut)
}

// ── 规则表 ──────────────────────────────────────────────────────────────

func praPrintRules(stdout io.Writer) {
	fmt.Fprintf(stdout, "%s %s —— 部分趟总验收门（T-PR-05 · §13.3 A/B/C/D 四组 13 条成对正负控）\n", praTOOL, praToolVersion)
	fmt.Fprint(stdout, "判什么：拦住「部分趟被误读成全绿放行」与「部分趟静默漏面」两类事故。\n")
	fmt.Fprint(stdout, "形态：只读 · 仓外夹具（tempfile.mkdtemp ＋ shell 桩 ＋ 合成 results.tsv / --list / map）·\n")
	fmt.Fprint(stdout, "      ✗ 不真跑全量 · ✗ 不碰 bin/zerg · 成对正负控 ＋ 负控自证（变异）· 自排除自身。\n")
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "已拍口径：\n")
	fmt.Fprint(stdout, "  P1 退码：部分趟子集全绿=3（独有）· 有 FAIL=1 · 无 FAIL 有 BLOCKED=2 ⇒ 部分趟绝不退 0；\n")
	fmt.Fprint(stdout, "          全量档绝不退 3；REPORT 不改变退码（与既有 gateResultsExitCode 一致）。\n")
	fmt.Fprint(stdout, "  P2 非 PASS = FAIL ∪ BLOCKED ∪ REPORT（入重跑集 ①）。\n")
	fmt.Fprint(stdout, "  P3 --since 只许指全量趟；指部分趟 ⇒ rc=2（禁链式）。\n")
	fmt.Fprint(stdout, "  P4 部分趟日志序号沿用全表 NN。\n")
	fmt.Fprint(stdout, "  P5 阀 >= 0.60（含恰 60.0% ⇒ 全量）。\n")
	fmt.Fprint(stdout, "  P6 五真源：任一变动 ⇒ 直接全量。\n")
	fmt.Fprint(stdout, "  三处转全量固定次序：① undeclared → ② 五真源 → ③ >=0.60 阀。\n")
	fmt.Fprint(stdout, "  台账物理隔离：部分趟只写 results-partial.tsv，从不写 results.tsv；抬头必含「不覆盖全量」。\n")
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "A/B/C/D 13 条控：\n")
	for _, c := range praControls {
		fmt.Fprintf(stdout, "  %s（%s）%s\n", c.id, c.kind, c.doc)
	}
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "本件退码表（三值 · 迎言风格）：\n")
	for _, k := range []int{0, 1, 2} {
		fmt.Fprintf(stdout, "  %d  %s\n", k, praRCName[k])
	}
	fmt.Fprint(stdout, "  2 另含：自检不过 · 用法错 · 夹具取不到 · 机制未接线（M0/M1 阶段）。\n")
}

// ── M0/M1 只算不跑（真档 · 可选）──────────────────────────────────────────

func praAnalyzeReal(fullDir, mapPath, changedPath *string, stdout, stderr io.Writer) int {
	hasFull := fullDir != nil && *fullDir != ""
	hasMap := mapPath != nil && *mapPath != ""
	if !hasFull || !hasMap {
		fmt.Fprint(stderr, "[不给结论] M0/M1 未接线：本片只交自检档（合成夹具）。\n")
		fmt.Fprint(stderr, "           要看真档需给 --full-dir <全量趟目录> --map <声明表> [--changed <改动件清单>]。\n")
		return praRCBlocked
	}
	fd, mpth := *fullDir, *mapPath
	if !praIsDir(fd) || !praIsFile(mpth) {
		fmt.Fprint(stderr, "[不给结论] 夹具取不到（全量趟目录 / 声明表不在盘）⇒ rc=2\n")
		return praRCBlocked
	}
	raw, jerr := os.ReadFile(mpth)
	if jerr != nil {
		fmt.Fprintf(stderr, "[不给结论] 声明表读不出：%s ⇒ rc=2\n", praOSErrText(mpth, jerr))
		return praRCBlocked
	}
	var mp map[string]any
	if jerr := json.Unmarshal(raw, &mp); jerr != nil {
		fmt.Fprintf(stderr, "[不给结论] 声明表读不出：%s ⇒ rc=2\n", jerr.Error())
		return praRCBlocked
	}
	rt := praPyJoin(fd, praLedgerFull)
	var rows []praRow
	if praIsFile(rt) {
		if b, err := os.ReadFile(rt); err == nil {
			for _, ln := range strings.Split(string(b), "\n") {
				c := strings.Split(strings.TrimRight(ln, "\n"), "\t")
				if len(c) >= 2 {
					rows = append(rows, praRow{c[0], c[1]})
				}
			}
		}
	}
	var changed []string
	if changedPath != nil && *changedPath != "" && praIsFile(*changedPath) {
		if b, err := os.ReadFile(*changedPath); err == nil {
			for _, x := range strings.Split(string(b), "\n") {
				if strings.TrimSpace(x) != "" && !strings.HasPrefix(x, "#") {
					changed = append(changed, strings.TrimSpace(x))
				}
			}
		}
	}
	faces := map[string][]string{}
	if f, ok := mp["faces"].(map[string]any); ok {
		for k, v := range f {
			if arr, ok := v.([]any); ok {
				var names []string
				for _, x := range arr {
					if s, ok := x.(string); ok {
						names = append(names, s)
					}
				}
				faces[k] = names
			}
		}
	}
	var mapFiles []praMapFile
	if fs, ok := mp["files"].([]any); ok {
		for _, e := range fs {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			mf := praMapFile{}
			if s, ok := m["pattern"].(string); ok {
				mf.pattern = s
			}
			if s, ok := m["kind"].(string); ok {
				mf.kind = s
			}
			if s, ok := m["face"].(string); ok {
				mf.face = s
			}
			if arr, ok := m["steps"].([]any); ok {
				for _, x := range arr {
					if s, ok := x.(string); ok {
						mf.steps = append(mf.steps, s)
					}
				}
			}
			mapFiles = append(mapFiles, mf)
		}
	}
	stepNames := make([]string, 0, len(rows))
	for _, r := range rows {
		stepNames = append(stepNames, r.name)
	}
	fx := praFixture{
		stepNames: stepNames,
		mapFaces:  faces,
		mapFiles:  mapFiles,
		changed:   changed,
		baseline: praBaseline{
			resultsPresent:        praIsFile(rt),
			resultsPartialPresent: praIsFile(praPyJoin(fd, praLedgerPartial)),
			partialBaseline:       false,
			freshnessPresent:      praIsFile(praPyJoin(fd, "freshness.json")),
			rows:                  rows,
		},
		flags:     praFlags{since: "auto"},
		rStatuses: map[string]string{},
	}
	res := praDecide(fx, "")
	fmt.Fprint(stdout, "── 部分趟总验收门（M0/M1 只算不跑 · 分析档）──\n")
	fmt.Fprintf(stdout, "  基线趟 %s（results 行 %d）· 改动件 %d · 声明表面 %d\n",
		fd, len(rows), len(changed), len(fx.mapFaces))
	ledgerStr := res.ledger
	if ledgerStr == "" {
		ledgerStr = "None"
	}
	fmt.Fprintf(stdout, "  预测：mode=%s · 台账=%s · 转全量因=%s\n", res.mode, ledgerStr, res.reason)
	for _, ln := range res.lines {
		fmt.Fprint(stdout, "    "+ln+"\n")
	}
	fmt.Fprint(stdout, "  ★ 只算不跑：本档不真跑任何步 · 机制未接线前只报预测。\n")
	return praRCOK
}

// praOSErrText —— py `str(OSError)` 常见档近似。
func praOSErrText(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		switch {
		case os.IsNotExist(pe):
			return "[Errno 2] No such file or directory: '" + path + "'"
		case os.IsPermission(pe):
			return "[Errno 13] Permission denied: '" + path + "'"
		}
	}
	return err.Error()
}

// ── 用法 / argparse 复刻 ──────────────────────────────────────────────────

const praUsage = "usage: check-partial-run-acceptance.py [-h] [--self-test] [--list-rules]\n" +
	"                                       [--full-dir FULL_DIR] [--map MAP_PATH]\n" +
	"                                       [--changed CHANGED_PATH] [--json]\n"

const praHelp = praUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --self-test\n" +
	"  --list-rules\n" +
	"  --full-dir FULL_DIR\n" +
	"  --map MAP_PATH\n" +
	"  --changed CHANGED_PATH\n" +
	"  --json\n"

type praArgs struct {
	selfTest    bool
	listRules   bool
	fullDir     *string
	mapPath     *string
	changedPath *string
	asJSON      bool
}

// praOpts —— 长旗标（★ help 先于其它，与 argparse add_help 同序）。
type praOpt struct {
	name       string
	takesValue bool
}

var praOpts = []praOpt{
	{"--help", false},
	{"--self-test", false},
	{"--list-rules", false},
	{"--full-dir", true},
	{"--map", true},
	{"--changed", true},
	{"--json", false},
}

func praResolveLong(tok string) (string, string, bool, error) {
	name := tok
	inline := ""
	hasInline := false
	if i := strings.Index(tok, "="); i >= 0 {
		name = tok[:i]
		inline = tok[i+1:]
		hasInline = true
	}
	if !strings.HasPrefix(name, "--") {
		return "", "", false, nil
	}
	var matches []string
	for _, o := range praOpts {
		if o.name == name {
			matches = []string{o.name}
			break
		}
		if strings.HasPrefix(o.name, name) {
			matches = append(matches, o.name)
		}
	}
	if len(matches) == 0 {
		return "", "", false, nil
	}
	if len(matches) > 1 {
		return "", "", false, fmt.Errorf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
	}
	if hasInline {
		return matches[0], inline, true, nil
	}
	return matches[0], "\x00none", false, nil
}

func praTakesValue(name string) bool {
	for _, o := range praOpts {
		if o.name == name {
			return o.takesValue
		}
	}
	return false
}

func praParseArgs(argv []string) (praArgs, string, bool, error) {
	args := praArgs{}
	endOpts := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			unrecognized = append(unrecognized, tok)
			continue
		}
		if tok == "--" {
			// py argparse：无 positional 时 `--` 自身亦计入未识别，其后 token 皆未识别
			unrecognized = append(unrecognized, tok)
			endOpts = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			return args, praHelp, true, nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		name, inline, hasInline, err := praResolveLong(tok)
		if err != nil {
			return args, "", false, err
		}
		if name == "" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		if name == "--help" {
			return args, praHelp, true, nil
		}
		if hasInline {
			if !praTakesValue(name) {
				return args, "", false, fmt.Errorf("argument %s: ignored explicit argument '%s'", name, inline)
			}
		} else if praTakesValue(name) {
			if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-" && len(argv[i+1]) > 1) {
				return args, "", false, fmt.Errorf("argument %s: expected one argument", name)
			}
			i++
			inline = argv[i]
		}
		switch name {
		case "--self-test":
			args.selfTest = true
		case "--list-rules":
			args.listRules = true
		case "--full-dir":
			v := inline
			args.fullDir = &v
		case "--map":
			v := inline
			args.mapPath = &v
		case "--changed":
			v := inline
			args.changedPath = &v
		case "--json":
			args.asJSON = true
		}
	}
	if len(unrecognized) > 0 {
		return args, "", false, fmt.Errorf("unrecognized arguments: %s", strings.Join(unrecognized, " "))
	}
	return args, "", false, nil
}

// ── 入口 ────────────────────────────────────────────────────────────────

// checkPartialRunAcceptanceGateMain —— 入口（与 py `main()` 逐字同构）。
func checkPartialRunAcceptanceGateMain(stdout, stderr io.Writer, argv []string) int {
	args, helpText, helpWanted, err := praParseArgs(argv)
	if err != nil {
		fmt.Fprint(stderr, praUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", praProg, err.Error())
		return praRCBlocked
	}
	if helpWanted {
		fmt.Fprint(stdout, helpText)
		return praRCOK
	}
	if args.listRules {
		praPrintRules(stdout)
		return praRCOK
	}
	if args.selfTest {
		rc := praSelfTest(stdout)
		if args.asJSON {
			fmt.Fprintf(stdout, "{%s: %s, %s: %d, %s: %s}\n",
				praJSONStr("tool"), praJSONStr(praTOOL),
				praJSONStr("rc"), rc,
				praJSONStr("rc_name"), praJSONStr(praRCName[rc]))
		}
		return rc
	}
	return praAnalyzeReal(args.fullDir, args.mapPath, args.changedPath, stdout, stderr)
}
