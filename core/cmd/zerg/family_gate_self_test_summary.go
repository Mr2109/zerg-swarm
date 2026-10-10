// family_gate_self_test_summary.go —— `zerg gate self-test-summary`：**判据自检的统一机器面**
// （缺口 `GAP-20260926-183` · 路 B · 门禁面）。
//
// 病（缺口逐字）：判据体的自检面各打各的「格/断言 × 失败」末行（现读 132 条 Go 自检函数 · 分
// ~30 种格式 · 含若干处**连数字都不打**只印「有断言失败」），**无共享助手**、**无统一机器面** ⇒
// 「格数」这类汇总读数只能靠人读、且**无法被机检**（「格数不减」的假绿由此而生）。
//
// 治法（**路 B · 只加汇总面 · 各判据件一字不动**）：本命令**一个入口**把仓内**已在进程内可跑**的
// 判据自检跑齐、逐件**集中解析**末行 ⇒ 汇出**三态**（通过 / 失败 / 判不了）＋ 两读数（格数 · 失败数）。
//
// ★ 三条纪律（与缺口口径逐字对应）：
//
//	① **各判据件不动**：本命令**不读也不改**任何判据件的自检实现 —— 它只**跑**它们、**解析**它们的末行
//	   （✗ 不去改那 132 处内联 `fmt.Fprintf`）。
//	② **集中解析必须 fail-loud**：末行**解析不出**（取不到 格数 ∧ 失败数）⇒ 记 **判不了**、**计入非 0 结果**
//	   —— ✗ **不许静默记 0**（「无数字的末行」按 0 记正是本缺口要防的「格数不减」假绿）。
//	③ **三态照本仓三值纪律**：`pass` / `fail` / `unknown(判不了)`；退码 `0` 全通过 · `1` 有失败 ·
//	   `2` 有判不了（不给结论）。
//
// 判据名录（**从哪来**）：本命令走**现读的进程内判据臂键表** `gateCheckGoEntry`
// （`gate_inproc_table.go` · 真搬成 Go、可在进程内跑的判据 ⇒ 它们的自检可在**本进程内**直调：
// 无 exec、无壳级危险档盾）。★ 与 `zerg gate check --json` 的全表机器面**同源**：那一面枚举的是
// **文件名录**（`scripts/` 递归列举 · 含大量非判据脚本 ⇒ 大半不认识 `--self-test`）；本面枚举的是
// **Go 判据臂**（自检函数的真身所在）⇒ 两者是**互补**的两个名录，不是同一份。py/sh 判据的自检
// 不在本面（它们要 exec ＋ 走壳级危险档，留给后续面）。
//
// 机器面形状**照** `gate check --json`（`{count, …, entries[]}`）：本面给
// `{count, pass, fail, unknown, grids, fails, entries[{name,state,grids,fails,rc,summary}]}`。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gateSelfTestSummaryTimeout —— **逐件**自检的墙钟上界。超时 ⇒ 该件记「判不了」（三值纪律：
// 跑不动 / 等不起 ≠ 通过）。取值照同族单件档（`gateFourStatesRun` 用 60s；自检是合成夹具、
// 多半毫秒级，收紧到 30s 以免一件卡死拖垮整面）。
const gateSelfTestSummaryTimeout = 30 * time.Second

// 三态取值（闭集）。
const (
	gstsPass    = "pass"
	gstsFail    = "fail"
	gstsUnknown = "unknown"
)

// gateSelfTestSummaryFields —— `--json` 的闭集字段（与 `main.go` 登记 `fields:` 逐字同源）。
var gateSelfTestSummaryFields = []string{"count", "pass", "fail", "unknown", "grids", "fails", "entries"}

// gateSelfTestSummaryKeys —— 判据名录：现读进程内判据臂键表 `gateCheckGoEntry` 的键（排序稳定）。
func gateSelfTestSummaryKeys() []string {
	keys := make([]string, 0, len(gateCheckGoEntry))
	for k := range gateCheckGoEntry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// gateSelfTestSummaryEntry —— 一格的机器面（人面与 JSON 同源）。
type gateSelfTestSummaryEntry struct {
	Name    string `json:"name"`
	State   string `json:"state"` // pass | fail | unknown（判不了）
	Grids   int    `json:"grids"`
	Fails   int    `json:"fails"`
	RC      int    `json:"rc"`
	Summary string `json:"summary"` // 被判的末行（逐字留痕；解析不出时给「（无末行）」一类）
}

// gateSelfTestSummaryLastLine —— 判「末行」：最后一条**非空**行（缺口口径就是末行；空行不算）。
func gateSelfTestSummaryLastLine(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// gateSelfTestSummaryStrip —— 解析前的规范化：把 markdown 强调符与制表装饰符**换成空白**。
// （末行里常夹 `**`、`──`、`——`、`⇒`、`✓`、`✗` 等装饰，它们对读数无意义、却会把正则割断。）
// ★ **`=` / `＝` 不动**：它们是**算式/百分比的分隔符**（`0/6=39.30%`）—— 若把 `=` 去掉，`0/6=39.30%`
// 会被并成 `0/639.30%`、被分式支读成「639 格 639 失败」的**假红**（实测踩过一次）。
var gateSelfTestSummaryStrip = strings.NewReplacer(
	"*", " ", "━", " ", "—", " ", "─", " ",
)

func gateSelfTestSummaryNorm(line string) string {
	return strings.Join(strings.Fields(gateSelfTestSummaryStrip.Replace(line)), " ")
}

// gstsUnits —— 「格/条/项…」量词（**多字**量词必须整词匹配：「用例」是两字，用 `[格条项]` 那样的
// 字符类会把「用例」错切成一字 ⇒ 读成另一个数）。
const gstsUnits = `(?:用例|条|项|格|组|个|控|档|例)`

// gstsFailWords —— 「失败」义族的词面。
const gstsFailWords = `(?:失败|不过|红|未过|败)`

// gateSelfTestSummaryPats —— 集中解析的格式族（**按序**取第一个命中）。
//
// ★ 这是「末行解析器」的全部牙齿：命不中 ⇒ 判不了（fail-loud）。新增判据件若引入新末行格式，
// 要么添进本表、要么**如实判不了**（✗ 不许放宽成「没读到数就当 0」）。
var gateSelfTestSummaryPats = []struct {
	re      *regexp.Regexp
	combine int // 0=前格后失；1=两数和⇒格、后数为失；2=前为总后为通过⇒失=差；3=前为通过后为总⇒格=后
}{
	// F0：`N 格 · 失败 M`（如 `自检 28 格 · 失败 0 格`）
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*·\s*` + gstsFailWords + `\s*(\d+)`), 0},
	// F1：`N 格 · M 格红`（如 `── 自检：15 格过 · 0 格红`）
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*·\s*(\d+)\s*` + gstsUnits + `?\s*` + gstsFailWords), 0},
	// F2：`P 格过 · M 格不过`（如 `自检：16 格过 · 0 格不过`）⇒ 格数 = P+M
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `过\s*·\s*(\d+)\s*` + gstsUnits + `?\s*(?:不过|红)`), 1},
	// F3：`P 过 / M 红`（如 `自检：74 过 / 0 红`）⇒ 格数 = P+M
	{regexp.MustCompile(`(\d+)\s*过\s*/\s*(\d+)\s*红`), 1},
	// F4：`断言 N 条 · 失败 M`（如 `── 自检结论：断言 19 条 · 失败 0 条 ──`）
	{regexp.MustCompile(`断言\s*(\d+)\s*` + gstsUnits + `\s*·\s*` + gstsFailWords + `\s*(\d+)`), 0},
	// F4b：`N 条，失败 M`（如 `自检结论: 全过（用例 5 条，失败 0 条）` —— 分隔是中文逗号）
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*[,，]\s*` + gstsFailWords + `\s*(\d+)`), 0},
	// F5：`N 条通过 / M 条失败`（如 `self-test：45 条通过 / 0 条失败`）
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `通过\s*/\s*(\d+)\s*` + gstsUnits + `失败`), 0},
	// F6：`N 项通过，M 项失败`（如 `自测：71 项通过，0 项失败`）⇒ 格数 = N+M
	{regexp.MustCompile(`(\d+)\s*[项个]\s*通过\s*[,，]\s*(\d+)\s*[项个]\s*失败`), 1},
	// F7：`N 用例 · M 败`（如 `（5 用例 · 0 败）`）
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*·\s*(\d+)\s*败`), 0},
	// F8：`N 条 · 通过 M`（如 `自证用例：33 条 · 通过 33 条`）⇒ 失败 = N-M
	{regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*·\s*通过\s*(\d+)`), 2},
}

// gstsFractionPat —— `N/M` 分式（如 `self-test: 6/6` · `通过（20/20 条断言通过）`）。
// ★ **单独一支**（不在 pats 表里）：分式最易误伤 —— `0/6=39.30%` 这类**百分比表**（如
// `stability-stats` 的「已知答案」行）会被读成「6 格 6 失败」的假红 ⇒ 必须逐个匹配、跳过那些
// **紧跟 `=`/`%`** 的分式（它们是算式/百分比，不是「通过/总数」）。
var gstsFractionPat = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)

// gateSelfTestSummaryParseFraction —— 取**第一个**不被 `=`/`%` 跟随的分式；无 ⇒ ok=false。
func gateSelfTestSummaryParseFraction(s string) (grids, fails int, ok bool) {
	for _, m := range gstsFractionPat.FindAllStringSubmatchIndex(s, -1) {
		rest := strings.TrimLeft(s[m[1]:], " ")
		if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "%") {
			continue // 算式/百分比（`a/b=c%`）⇒ 不是「通过/总数」
		}
		a, _ := strconv.Atoi(s[m[2]:m[3]])
		b, _ := strconv.Atoi(s[m[4]:m[5]])
		if a > b {
			continue
		}
		return b, b - a, true
	}
	return 0, 0, false
}

// gstsVerdictPat —— 「全过/全绿/全对/全胜/通过/PASS」裁决词（仅兜底支用）。
var gstsVerdictPat = regexp.MustCompile(`全过|全绿|全对|全胜|通过|PASS`)

// gstsVerdictSinglePat —— 只有**裁决词**、无并列失败读数的末行的兜底（**仅此支**才允许把失败归 0：
// 裁决词明写全过/全绿/全对/全胜/通过/PASS ＝ 逐格对拍全符）。★ 取到裁决词但**取不到格数** ⇒
// 仍返回 ok=false（「无格数」不得当 0 —— 缺口要防的正是这一格）。
var gstsVerdictSinglePat = []*regexp.Regexp{
	regexp.MustCompile(`(?:全过|全绿|全对|全胜|通过|PASS)[^\d（(]{0,6}[（(]\s*(\d+)\s*` + gstsUnits),
	regexp.MustCompile(`(\d+)\s*` + gstsUnits + `\s*(?:全过|全绿|全对)`),
	regexp.MustCompile(`(\d+)\s*(?:控|组)`),
	regexp.MustCompile(`合计\s*(\d+)\s*` + gstsUnits),
}

// gateSelfTestSummaryParse —— 集中解析一条末行 ⇒ (格数, 失败数, 解析成功)。
//
// ★ fail-loud（缺口口径逐字）：返回 ok=false 的三种情形一律**不许**在调用方折算成 0：
//   - 末行**无任何数字**（如 `自检结论: **有断言失败**` / `自检结论: 全过`）；
//   - 有数字但**构不成一对**（只读到格数或只读到失败数）；
//   - 格式不认识。
func gateSelfTestSummaryParse(line string) (grids, fails int, ok bool) {
	s := gateSelfTestSummaryNorm(line)
	if s == "" {
		return 0, 0, false
	}
	for _, p := range gateSelfTestSummaryPats {
		m := p.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		a, err1 := strconv.Atoi(m[1])
		b, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			continue
		}
		switch p.combine {
		case 1: // 两数和 = 格数、后数 = 失败数
			return a + b, b, true
		case 2: // 前为总、后为通过 ⇒ 失败 = 差
			if a < b {
				return 0, 0, false
			}
			return a, a - b, true
		default: // 前格后失
			return a, b, true
		}
	}
	// 分式支（`N/M`）：单独判、须跳过 `a/b=c%` 这类算式/百分比。
	if g, f, ok := gateSelfTestSummaryParseFraction(s); ok {
		return g, f, true
	}
	// 兜底：裁决词 ＋ 单读数 ⇒ 格数取到、失败归 0（**仅此支**允许；取不到格数 ⇒ ok=false）。
	if gstsVerdictPat.MatchString(s) {
		for _, re := range gstsVerdictSinglePat {
			if m := re.FindStringSubmatch(s); m != nil {
				if g, err := strconv.Atoi(m[1]); err == nil {
					return g, 0, true
				}
			}
		}
	}
	return 0, 0, false
}

// gateSelfTestSummaryClassify —— 逐件分类：解析末行 ⇒ (态, 格数, 失败数, 末行正文)。
func gateSelfTestSummaryClassify(out string) (state string, grids, fails int, summary string) {
	summary = gateSelfTestSummaryLastLine(out)
	if summary == "" {
		return gstsUnknown, 0, 0, "（无末行）"
	}
	g, f, ok := gateSelfTestSummaryParse(summary)
	if !ok {
		return gstsUnknown, 0, 0, summary
	}
	if f > 0 {
		return gstsFail, g, f, summary
	}
	return gstsPass, g, f, summary
}

// gstsSyncBuf —— 并发安全的缓冲（判据自检在与本函数不同的 goroutine 里跑；超时后仍可能继续写 ⇒
// 读快照必须加锁，避免数据竞争）。
type gstsSyncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *gstsSyncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *gstsSyncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// gateSelfTestSummaryRunOne —— 跑一件判据的 `--self-test`（**进程内** · 复用 `gateCheckRunInproc`
// 的 cwd/env 口径 ⇒ 与 `zerg gate check <键> --self-test` 的执行面同规），捕获它的 stdout。
// 超时 / panic ⇒ timedOut / crashed 真值（调用方据此记「判不了」）。
func gateSelfTestSummaryRunOne(root, name string, fn func(stdout, stderr io.Writer, argv []string) int) (out string, rc int, timedOut, crashed bool) {
	var so, se gstsSyncBuf
	done := make(chan int, 1)
	go func() {
		code := 0
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(&se, "panic: %v\n", r)
					crashed = true
					code = exitBlocked
				}
			}()
			code = gateCheckRunInproc(fn, root, &so, &se, []string{"--self-test"})
		}()
		done <- code
	}()
	select {
	case rc = <-done:
		return so.String(), rc, false, crashed
	case <-time.After(gateSelfTestSummaryTimeout):
		return so.String(), 0, true, false
	}
}

// cmdGateSelfTestSummary —— 命令入口（`run:` 直挂本函数 · 不走 `cmdGate`/自举件）。
func cmdGateSelfTestSummary(inv *invocation, stdout, stderr io.Writer) int {
	if inv.selfTest {
		return gateSelfTestSummarySelfTest(stdout, stderr)
	}
	// 字段面（执行前判）：闭集外 ⇒ 2 + 同族出口。
	if inv.jsonGiven {
		fields := inv.fields
		if len(fields) == 0 {
			fields = gateSelfTestSummaryFields
		}
		for _, f := range fields {
			if !gateSelfTestSummaryFieldLegal(f) {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
	}

	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 判据自检需要仓根（给 ZERG_REPO 或在本仓内跑）\n", progName)
		inv.setErr("blocked", "no_root", "仓根解析不到")
		return exitBlocked
	}

	keys := gateSelfTestSummaryKeys()
	entries := make([]gateSelfTestSummaryEntry, 0, len(keys))
	pass, fail, unknown, totG, totF := 0, 0, 0, 0, 0

	// ★ 泄漏捕获：少数判据的自检**直写 `os.Stdout`**（不走传入的 writer，如 `check-fleet-loadable`）
	// ⇒ 若不管，机器面（stdout 的 JSON）会被它们污染、且它们的末行也读不到。治法 = 全程把
	// `os.Stdout` 指向一枚临时件，逐件读「本次新增」并并入该件的捕获内容（✗ 不换 `os.Stderr`
	// —— 那会丢掉它们的末行）。全程只换一次（不逐件换）⇒ 无 `os.Stdout` 数据竞争。
	lf, lerr := os.CreateTemp("", "gsts-stdout-")
	var savedStdout *os.File
	leakPos := int64(0)
	if lerr == nil {
		savedStdout = os.Stdout
		os.Stdout = lf
		defer func() {
			os.Stdout = savedStdout
			_ = lf.Close()
			_ = os.Remove(lf.Name())
		}()
	}
	readLeak := func() string {
		if lf == nil {
			return ""
		}
		end, _ := lf.Seek(0, io.SeekCurrent)
		if end <= leakPos {
			return ""
		}
		buf := make([]byte, end-leakPos)
		_, _ = lf.ReadAt(buf, leakPos)
		leakPos = end
		return string(buf)
	}

	for _, name := range keys {
		fn := gateCheckGoEntry[name]
		if fn == nil {
			continue
		}
		out, rc, timedOut, crashed := gateSelfTestSummaryRunOne(root, name, fn)
		out += readLeak() // 并入该件直写 os.Stdout 的那部分
		state, g, f, summ := gateSelfTestSummaryClassify(out)
		if timedOut || crashed {
			state = gstsUnknown
			if timedOut && strings.TrimSpace(summ) == "" {
				summ = "（超时 ⇒ 判不了）"
			}
			if crashed && strings.TrimSpace(summ) == "" {
				summ = "（自检崩 ⇒ 判不了）"
			}
			if timedOut {
				summ = fmt.Sprintf("（超时 %s ⇒ 判不了）", gateSelfTestSummaryTimeout)
			}
			g, f = 0, 0
		}
		switch state {
		case gstsPass, gstsFail:
			if state == gstsPass {
				pass++
			} else {
				fail++
			}
			totG += g
			totF += f
		default:
			unknown++
		}
		entries = append(entries, gateSelfTestSummaryEntry{
			Name: name, State: state, Grids: g, Fails: f, RC: rc, Summary: summ,
		})
	}

	// 出口三态（照本仓三值纪律）：有判不了 ⇒ 2（不给结论）；否则有失败 ⇒ 1；否则 0。
	rc := exitOK
	switch {
	case unknown > 0:
		rc = exitBlocked
	case fail > 0:
		rc = exitFail
	}

	if inv.jsonGiven {
		gateSelfTestSummaryJSON(inv, stdout, stderr, entries, pass, fail, unknown, totG, totF)
		return rc
	}
	gateSelfTestSummaryHuman(stdout, entries, pass, fail, unknown, totG, totF)
	return rc
}

// gateSelfTestSummaryFieldLegal —— 字段是否在闭集内。
func gateSelfTestSummaryFieldLegal(f string) bool {
	for _, ok := range gateSelfTestSummaryFields {
		if f == ok {
			return true
		}
	}
	return false
}

// gateSelfTestSummaryHuman —— 人面（与机器面同源）。
func gateSelfTestSummaryHuman(stdout io.Writer, entries []gateSelfTestSummaryEntry, pass, fail, unknown, grids, fails int) {
	fmt.Fprintf(stdout, "判据自检汇总（名录 = 进程内判据臂 gateCheckGoEntry · 三态）：件 %d · 通过 %d · 失败 %d · 判不了 %d\n",
		len(entries), pass, fail, unknown)
	fmt.Fprintf(stdout, "读数：格数 %d · 失败数 %d（只统计解析成功的两态；判不了**不计入**，✗ 不当 0）\n", grids, fails)
	for _, e := range entries {
		mark := map[string]string{gstsPass: "✓", gstsFail: "✗", gstsUnknown: "?"}[e.State]
		switch e.State {
		case gstsPass, gstsFail:
			fmt.Fprintf(stdout, "  %s %-40s %s · 格数 %d · 失败 %d · rc=%d\n", mark, e.Name, e.State, e.Grids, e.Fails, e.RC)
		default:
			fmt.Fprintf(stdout, "  %s %-40s 判不了 · rc=%d · 末行：%s\n", mark, e.Name, e.RC, e.Summary)
		}
	}
}

// gateSelfTestSummaryJSON —— 机器面（形状照 `gate check --json`：`{count,…,entries[]}`）。
func gateSelfTestSummaryJSON(inv *invocation, stdout, stderr io.Writer, entries []gateSelfTestSummaryEntry, pass, fail, unknown, grids, fails int) {
	base := map[string]any{
		"count":   len(entries),
		"pass":    pass,
		"fail":    fail,
		"unknown": unknown,
		"grids":   grids,
		"fails":   fails,
		"entries": entries,
	}
	if len(inv.fields) == 0 {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(base)
		return
	}
	// 点名档：只出点到的几格（序即用户给的序）。
	var b strings.Builder
	b.WriteString("{")
	n := 0
	for _, f := range inv.fields {
		if n > 0 {
			b.WriteString(",")
		}
		n++
		if f == "entries" {
			raw, _ := json.Marshal(entries)
			fmt.Fprintf(&b, "\"entries\":%s", raw)
			continue
		}
		fmt.Fprintf(&b, "%q:%d", f, base[f])
	}
	b.WriteString("}\n")
	if _, err := io.WriteString(stdout, b.String()); err != nil {
		fmt.Fprintf(stderr, "%s: 出 JSON 失败：%v\n", progName, err)
	}
}

// gateSelfTestSummarySelfTest —— 成对自检（正控 2 ＋ 负控 3 · 合成夹具 · 不碰真目标）。
//
// ★ 牙口（负控**真红**：行为一旦退回假绿，这些格当场 ✗）：
//   - 正控①：跑**真判据** `check-doc-consumers` 的自检 ⇒ 必须机器读到 格数>0 ∧ 失败数==0 ∧ 态=通过；
//   - 正控②：一条规范两数末行 ⇒ 格数/失败数机器可读；
//   - 负控①：喂一条**无数字**失败末行（`自检结论: **有断言失败**`）⇒ 必须记 **判不了**（✗ 不许折算 0）；
//   - 负控①b：喂一条**无数字**通过末行（`自检结论: 全过`）⇒ 仍**判不了**（无格数 ⇒ 不得当 0）；
//   - 负控②：喂一条**真失败**末行（`自检结论: 全过（断言 5 条 · 失败 2 条）`）⇒ 失败数必须**非 0**。
func gateSelfTestSummarySelfTest(stdout, stderr io.Writer) int {
	_ = stderr
	fails := 0
	type row struct {
		name string
		ok   bool
		got  string
		want string
	}
	rows := []row{}
	add := func(name string, got, want bool) {
		g, w := "否", "否"
		if got {
			g = "是"
		}
		if want {
			w = "是"
		}
		if got != want {
			fails++
		}
		rows = append(rows, row{name, got == want, g, w})
	}

	// 正控①：真判据 check-doc-consumers 的自检 ⇒ 格数>0 ∧ 失败数==0 ∧ 态=通过。
	if root := repoRoot(); root != "" {
		if fn := gateCheckGoEntry["check-doc-consumers"]; fn != nil {
			out, _, timedOut, crashed := gateSelfTestSummaryRunOne(root, "check-doc-consumers", fn)
			st, g, f, _ := gateSelfTestSummaryClassify(out)
			add("正控①·真判据 check-doc-consumers：机器读到格数>0 且未超时/未崩", g > 0 && !timedOut && !crashed, true)
			add("正控①·真判据 check-doc-consumers：失败数==0 且态=通过", f == 0 && st == gstsPass, true)
		}
	}

	// 正控②：规范两数末行 ⇒ 格数=12 · 失败数=0 · 态=通过。
	st0, g0, f0, _ := gateSelfTestSummaryClassify("self-test：12 格 · 0 失败\n")
	add("正控②·规范末行：格数=12 ∧ 失败数=0 ∧ 态=通过", g0 == 12 && f0 == 0 && st0 == gstsPass, true)

	// 负控①：无数字失败末行 ⇒ 判不了（✗ 不许折算成通过/0）。
	st1, g1, f1, _ := gateSelfTestSummaryClassify("自检结论: **有断言失败**\n")
	add("负控①·无数字失败末行：态为判不了（不折算通过）", st1 == gstsUnknown, true)
	add("负控①·无数字失败末行：格数与失败数都不许记 0", g1 == 0 && f1 == 0, true)

	// 负控①b：无数字通过末行 ⇒ 仍判不了（无格数 ⇒ 不得当 0）。
	st1b, _, _, _ := gateSelfTestSummaryClassify("自检结论: 全过\n")
	add("负控①b·无数字「全过」末行：仍判不了（无格数不得当 0）", st1b == gstsUnknown, true)

	// 负控②：真失败末行 ⇒ 失败数非 0。
	st2, g2, f2, _ := gateSelfTestSummaryClassify("自检结论: 全过（断言 5 条 · 失败 2 条）\n")
	add("负控②·真失败末行：态为失败", st2 == gstsFail, true)
	add("负控②·真失败末行：格数=5 ∧ 失败数=2（非 0）", g2 == 5 && f2 == 2, true)

	for _, r := range rows {
		mark := "✓"
		if !r.ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "%s %s：got=%s want=%s\n", mark, r.name, r.got, r.want)
	}
	fmt.Fprintf(stdout, "self-test：%d 格 · %d 失败\n", len(rows), fails)
	if fails == 0 {
		return exitOK
	}
	return exitFail
}
