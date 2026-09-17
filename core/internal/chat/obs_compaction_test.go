// obs_compaction_test.go — T3.1 实证：压缩**三态**事件是**可判定**的观测（含「不压缩不落事件」反例）
//
// 要治的盲区（改本文件前先读）：一次压缩在旧观测面里只有一条终态行 ——
// 压到什么程度（tokens/ratio）、第几次压（depth）、压掉了哪些消息、留下的与压掉的能不能把全部消息分完、
// 摘要指纹（sha256）、结构化六段的覆盖（field_coverage）**全答不上**；而「开始压了却没有终态」
// 与「根本没压」在事件流里**同形**。本文件把这三类问题钉成可判定的用例：
//
//	① 一次成功压缩 ⇒ started + completed **成对**（同一 span）· tokens_before > tokens_after ·
//	   ratio 按口径算对 · depth/trigger/计数/指纹/覆盖齐全
//	② 失败路径 ⇒ 只有 started + failed（**不得**有 completed）· 错误原文在（超长则截断 ≤300）
//	③ **不压缩 ⇒ 一条 compaction 事件都不落**（反例：防误报；含「冷却中不重试」那条路）
//	④ depth 递增：同会话连续两次压缩 ⇒ 第二次 depth=2（且与配对的 started 同 span）
//	⑤ kept/summarized **不重叠且并集 = 原消息集合**（小固定集断言 + 用库的真实归档状态做独立裁判）
//	⑥ field_coverage 只在**结构化**摘要上出现；散文/删除式压缩一律**缺席**（拿不到就缺席，不编造全 false）
//
// 纪律（与 obs_test.go / obs_behavior_test.go 同源）：
//   - 观测一律落隔离目录：t.Setenv("ZERG_STATE_DIR", t.TempDir())（由 newCompactStore 做），绝不写用户真实状态。
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里）+ **read 层**（解回来的值是真的）——只断 Go 字段=自证。
//   - 摘要指纹用 **crypto/sha256 直接复算**（不调用被测代码的 helper），否则是自证。
package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"testing"
)

// compactObsLine — 三态事件读回结构（键缺席 ⇒ 指针为 nil，这正是「这类量没落」的判据）
type compactObsLine struct {
	TS           string         `json:"ts"`
	Kind         string         `json:"kind"`
	Session      string         `json:"session"`
	Model        string         `json:"model"`
	EventName    string         `json:"event_name"`
	TraceID      string         `json:"trace_id"`
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id"`
	SpanKind     string         `json:"span_kind"`
	StartedAtMS  int64          `json:"started_at_unix_ms"`
	EndedAtMS    int64          `json:"ended_at_unix_ms"`
	Result       string         `json:"result"`
	SummaryChars int            `json:"summary_chars"`
	FailReason   string         `json:"fail_reason"`
	Compaction   *CompactionObs `json:"compaction"`
}

// compactEventPrefix — 本族事件的前缀（读侧按它挑行；**不**按 kind=compact —— 那会把旧 OBS-4 行一起数上）
const compactEventPrefix = "compaction_"

// readCompactObsLines — 读回观测文件全部行（读侧必须宽容：别的 kind 的行也解，不认识的键忽略）
func readCompactObsLines(t *testing.T) []compactObsLine {
	t.Helper()
	raw := readObs(t)
	if raw == "" {
		return nil
	}
	var out []compactObsLine
	for _, ln := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r compactObsLine
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON（读侧必须宽容）：%v\n%s", err, ln)
		}
		out = append(out, r)
	}
	return out
}

// eventsOf — 只挑本族事件（按 event_name 前缀），保持落盘顺序
func eventsOf(lines []compactObsLine) []compactObsLine {
	var out []compactObsLine
	for _, ln := range lines {
		if strings.HasPrefix(ln.EventName, compactEventPrefix) {
			out = append(out, ln)
		}
	}
	return out
}

// countEvent — 某个状态的事件条数
func countEvent(lines []compactObsLine, event string) int {
	n := 0
	for _, ln := range lines {
		if ln.EventName == event {
			n++
		}
	}
	return n
}

// legacyCompactRows — 旧 OBS-4 终态行：kind=compact 且**没有显式事件名**
// （骨架会把空 event_name 填成 kind ⇒ 旧行的 event_name 就等于 "compact"；三态行则各有已命名的事件）
func legacyCompactRows(lines []compactObsLine) []compactObsLine {
	var out []compactObsLine
	for _, ln := range lines {
		if ln.Kind == "compact" && (ln.EventName == "" || ln.EventName == ln.Kind) {
			out = append(out, ln)
		}
	}
	return out
}

// mustOneEvent — 恰好一条该状态的事件（0 条或重复都是缺陷：重复落盘会让"压了几次"失真）
func mustOneEvent(t *testing.T, lines []compactObsLine, event string) compactObsLine {
	t.Helper()
	var hit []compactObsLine
	for _, ln := range lines {
		if ln.EventName == event {
			hit = append(hit, ln)
		}
	}
	if len(hit) != 1 {
		t.Fatalf("%s 应恰好 1 条，实得 %d 条\n全部事件：%+v", event, len(hit), eventsOf(lines))
	}
	if hit[0].Compaction == nil {
		t.Fatalf("%s 缺 compaction 事实块（三态必须带块）：%+v", event, hit[0])
	}
	return hit[0]
}

// assertIDPartition — 集合不变式：不重叠 + 并集 = 原集合（口径见 compactObsFactsOf）
func assertIDPartition(t *testing.T, cp *CompactionObs, all []int64) {
	t.Helper()
	kept := map[int64]bool{}
	for _, id := range cp.KeptIDs {
		if kept[id] {
			t.Errorf("kept_message_ids 有重复 id %d", id)
		}
		kept[id] = true
	}
	summ := map[int64]bool{}
	for _, id := range cp.SummarizedIDs {
		if summ[id] {
			t.Errorf("summarized_message_ids 有重复 id %d", id)
		}
		if kept[id] {
			t.Errorf("id %d 同时出现在保留段与被压缩段（必须互补）", id)
		}
		summ[id] = true
	}
	if len(kept)+len(summ) != len(all) {
		t.Errorf("并集大小 %d+%d 应等于原消息数 %d", len(kept), len(summ), len(all))
	}
	for _, id := range all {
		if !kept[id] && !summ[id] {
			t.Errorf("原消息 id %d 既不在保留段也不在被压缩段（并集应等于原集合）", id)
		}
	}
}

// obsSortedIDs — 排序副本（比较集合用）
func obsSortedIDs(ids []int64) []int64 {
	out := append([]int64{}, ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// obsStructuredSummary — 六段齐的结构化摘要（贴模板 compactSystemPrompt 的段名）
func obsStructuredSummary() string {
	return strings.Join([]string{
		"## Goal", "把长会话压成结构化摘要",
		"## Progress", "- 已完成: 甲；- 进行中: 乙",
		"## Key Decisions", "- 决定：不分叉",
		"## Relevant Files", "- /tmp/obs.go",
		"## Next Steps", "- 跑门禁",
		"## Critical Context", "- 精确值 42",
	}, "\n")
}

// ── 用例 ①：成功压缩 ⇒ started + completed 成对，量算对 ────────────────

func TestCompactObsSuccessPair(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.1 成功压缩")
	if err != nil {
		t.Fatal(err)
	}
	const n = 30
	body := strings.Repeat("测", 300) // ≈428 token/条
	ids := compactSeed(t, st, se.ID, n, body)
	msgs := compactActive(t, st, se.ID)
	wantBefore := 0
	for _, m := range msgs {
		wantBefore += compactMessageTokens(*m)
	}

	did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", msgs, nil,
		func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false)
	if cerr != nil || !did {
		t.Fatalf("达阈值应压缩成功: did=%v err=%v", did, cerr)
	}

	lines := readCompactObsLines(t)
	started := mustOneEvent(t, lines, obsEventCompactionStarted)
	completed := mustOneEvent(t, lines, obsEventCompactionCompleted)
	if got := countEvent(lines, obsEventCompactionFailed); got != 0 {
		t.Errorf("成功路径不得有 compaction_failed（实得 %d 条）", got)
	}

	// 成对：同一 span / 同一 trace / 同一父（口径④：成对可校验，不靠自称）
	if started.SpanID == "" || started.SpanID != completed.SpanID {
		t.Errorf("started 与 completed 必须共用同一个 span_id（成对可校验）：%q vs %q", started.SpanID, completed.SpanID)
	}
	if started.TraceID == "" || started.TraceID != completed.TraceID {
		t.Errorf("trace_id 应同源：%q vs %q", started.TraceID, completed.TraceID)
	}
	if started.ParentSpanID != completed.ParentSpanID {
		t.Errorf("父 span 应相同：%q vs %q", started.ParentSpanID, completed.ParentSpanID)
	}
	if started.SpanKind != ObsSpanKindInternal || completed.SpanKind != ObsSpanKindInternal {
		t.Errorf("压缩是进程内步骤 ⇒ span_kind 应为 internal：%q / %q", started.SpanKind, completed.SpanKind)
	}
	if started.StartedAtMS == 0 || completed.EndedAtMS < started.StartedAtMS {
		t.Errorf("时间戳应齐备且单调：%d → %d", started.StartedAtMS, completed.EndedAtMS)
	}
	if started.Session != se.ID || completed.Session != se.ID {
		t.Errorf("会话应落在事件上：%q / %q（期望 %q）", started.Session, completed.Session, se.ID)
	}

	// wire 层：任务要求与本族口径的字段名必须真的在 JSONL 原文里
	raw := readObs(t)
	for _, key := range []string{
		`"event_name":"compaction_started"`, `"event_name":"compaction_completed"`,
		`"tokens_before"`, `"tokens_after"`, `"messages_before"`, `"messages_after"`,
		`"compression_ratio"`, `"turns_summarized"`, `"summarizer_model"`, `"compaction_depth"`,
		`"kept_message_ids"`, `"summarized_message_ids"`, `"summary_sha256"`, `"field_coverage"`,
		`"latency_ms"`, `"trigger_reason"`,
		// T1.1 追踪骨架（事件必须带）
		`"trace_id"`, `"span_id"`, `"parent_span_id"`, `"span_kind"`,
		`"started_at_unix_ms"`, `"ended_at_unix_ms"`, `"recorded":true`, `"sampled":true`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("观测原文缺字段 %s\n原文：%s", key, raw)
		}
	}

	cp := completed.Compaction
	// tokens_before：与独立复算一致（同一估算器口径）；且压缩后必须真的变少
	if cp.TokensBefore != wantBefore {
		t.Errorf("tokens_before=%d，独立复算 %d", cp.TokensBefore, wantBefore)
	}
	if cp.TokensAfter == nil {
		t.Fatal("completed 必须带 tokens_after（终态才知道的量）")
	}
	if !(cp.TokensBefore > *cp.TokensAfter) {
		t.Errorf("压缩后 token 应变少：before=%d after=%d（不减小 ⇒ 压了个寂寞）", cp.TokensBefore, *cp.TokensAfter)
	}
	// compression_ratio = tokens_before / tokens_after（4 位小数）——口径写死在 obs_compaction.go
	if cp.CompressionRatio == nil {
		t.Fatal("completed 必须带 compression_ratio")
	}
	wantRatio := math.Round(float64(cp.TokensBefore)/float64(*cp.TokensAfter)*10000) / 10000
	if *cp.CompressionRatio != wantRatio {
		t.Errorf("compression_ratio=%v，按口径(before/after)应为 %v", *cp.CompressionRatio, wantRatio)
	}
	if *cp.CompressionRatio <= 1 {
		t.Errorf("确实压下去了 ⇒ ratio 应 >1（=1 表示没缩小）；实得 %v", *cp.CompressionRatio)
	}
	// 计数/序次/触发原因
	if cp.MessagesBefore != n {
		t.Errorf("messages_before=%d，期望 %d", cp.MessagesBefore, n)
	}
	if cp.MessagesAfter == nil || *cp.MessagesAfter != n-7+1 {
		t.Errorf("messages_after 应为 %d（30-7 软归档 + 1 条摘要）；实得 %v", n-7+1, cp.MessagesAfter)
	}
	if cp.Depth != 1 {
		t.Errorf("首次压缩 compaction_depth 应为 1，实得 %d", cp.Depth)
	}
	if cp.TriggerReason != CompactTriggerTokenBudget {
		t.Errorf("阈值触发 ⇒ trigger_reason 应为 %q，实得 %q", CompactTriggerTokenBudget, cp.TriggerReason)
	}
	if cp.SummarizerModel != "gemma-12B" {
		t.Errorf("summarizer_model 应为会话模型，实得 %q", cp.SummarizerModel)
	}
	if cp.LatencyMS == nil {
		t.Error("completed 必须带 latency_ms")
	}
	if cp.TurnsSummarized != 3 {
		t.Errorf("被压段（索引 3..9，角色交替）里 user 应为 3 条，实得 %d", cp.TurnsSummarized)
	}
	// started 不得带终态才知道的量（缺席=未知，不许写 0 顶替）
	if sc := started.Compaction; sc.TokensAfter != nil || sc.MessagesAfter != nil ||
		sc.CompressionRatio != nil || sc.LatencyMS != nil || sc.SummarySHA256 != "" || sc.FieldCoverage != nil {
		t.Errorf("started 不得带终态独有的量（tokens_after/ratio/messages_after/latency/sha/coverage）：%+v", sc)
	}
	// 集合不变式（详细断言见用例⑤）
	assertIDPartition(t, cp, ids)

	// summary_sha256：与**库里那条摘要消息**（去掉角色前缀）逐字对齐 —— 用标准库直接复算
	sumMsg := compactFindSummary(t, st, se.ID)
	const obsSumPrefix = "【历史摘要】"
	if sumMsg == nil || !strings.HasPrefix(sumMsg.Content, obsSumPrefix) {
		t.Fatalf("库里应有摘要消息且带角色前缀：%+v", sumMsg)
	}
	inserted := strings.TrimPrefix(sumMsg.Content, obsSumPrefix)
	want := sha256.Sum256([]byte(inserted))
	if cp.SummarySHA256 != hex.EncodeToString(want[:]) {
		t.Errorf("summary_sha256=%q，对落库正文独立复算 %q", cp.SummarySHA256, hex.EncodeToString(want[:]))
	}
	if !strings.Contains(inserted, "可搜回:session_search(") {
		t.Errorf("摘要应含尾部召回指针（指纹是对落库正文取的）：%s", inserted)
	}

	// field_coverage：结构化摘要六段齐 ⇒ 6/6 且逐字段 true
	if cp.FieldCoverage == nil {
		t.Fatal("结构化摘要应带 field_coverage")
	}
	if cp.FieldCoverage.Present != 6 || cp.FieldCoverage.Total != 6 {
		t.Errorf("六段齐 ⇒ present/total 应为 6/6，实得 %d/%d", cp.FieldCoverage.Present, cp.FieldCoverage.Total)
	}
	if !cp.FieldCoverage.Goal || !cp.FieldCoverage.Progress || !cp.FieldCoverage.KeyDecisions ||
		!cp.FieldCoverage.RelevantFiles || !cp.FieldCoverage.NextSteps || !cp.FieldCoverage.CriticalContext {
		t.Errorf("六段应逐字段 true：%+v", *cp.FieldCoverage)
	}

	// 旧 OBS-4 行**原样保留**（不删信息；也证明终态行带上了 result/summary_chars 的老口径）
	legacy := legacyCompactRows(lines)
	if len(legacy) != 1 || legacy[0].Result != "ok" {
		t.Errorf("旧 OBS-4 终态行应原样保留 1 条（result=ok），实得 %+v", legacy)
	}
	if completed.Result != "ok" || completed.SummaryChars != len([]rune(obsStructuredSummary())) {
		t.Errorf("新终态行应带老口径 result/summary_chars：result=%q summary_chars=%d",
			completed.Result, completed.SummaryChars)
	}
	t.Logf("✓ 成功压缩：started+completed 成对（span %s…）· tokens %d→%d · ratio %v · depth %d",
		started.SpanID[:8], cp.TokensBefore, *cp.TokensAfter, *cp.CompressionRatio, cp.Depth)
}

// ── 用例 ②：失败路径 ⇒ 只有 started + failed（不得有 completed）────────

func TestCompactObsFailureOnly(t *testing.T) {
	st, _ := newCompactStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.1 失败路径")
	if err != nil {
		t.Fatal(err)
	}
	compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
	sentinel := "摘要模型返回 HTTP 500: boom"

	did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil,
		func(context.Context, []map[string]any) (string, error) { return "", errors.New(sentinel) }, false)
	if did || cerr == nil {
		t.Fatalf("摘要失败 ⇒ did=false 且 err 非空：did=%v err=%v", did, cerr)
	}

	lines := readCompactObsLines(t)
	started := mustOneEvent(t, lines, obsEventCompactionStarted)
	failed := mustOneEvent(t, lines, obsEventCompactionFailed)
	if got := countEvent(lines, obsEventCompactionCompleted); got != 0 {
		t.Errorf("失败路径**不得**落 compaction_completed（实得 %d 条）——否则事件流说谎", got)
	}

	// 成对：失败也要能挂回同一个 span（悬空/半途而废据此可见）
	if started.SpanID == "" || started.SpanID != failed.SpanID {
		t.Errorf("started 与 failed 必须共用同一个 span_id：%q vs %q", started.SpanID, failed.SpanID)
	}
	// 起点事实两侧一致（终态原样复用起点算出的布局）
	if sc, fc := started.Compaction, failed.Compaction; sc.TokensBefore != fc.TokensBefore ||
		sc.MessagesBefore != fc.MessagesBefore || sc.Depth != fc.Depth {
		t.Errorf("起点事实应一致：started=%+v failed=%+v", sc, fc)
	}
	// 错误原文在（截断 ≤300 字，与 turn.err_text 同一写法）
	if !strings.Contains(failed.Compaction.ErrText, sentinel) {
		t.Errorf("failed 应带错误原文（含 %q）：%q", sentinel, failed.Compaction.ErrText)
	}
	if failed.Compaction.LatencyMS == nil {
		t.Error("failed 应带 latency_ms（失败也要知道耗了多久）")
	}
	// 失败路径不得出现摘要相关的终态量（缺席 = 未知，不许写 0/空串顶替）
	if fc := failed.Compaction; fc.TokensAfter != nil || fc.MessagesAfter != nil || fc.CompressionRatio != nil ||
		fc.SummarySHA256 != "" || fc.FieldCoverage != nil {
		t.Errorf("failed 不得带摘要相关终态量：%+v", fc)
	}
	// 老口径兼容：result=fail + fail_reason（含 streak，与旧 OBS-4 行同文本）
	if failed.Result != "fail" || !strings.Contains(failed.FailReason, sentinel) || !strings.Contains(failed.FailReason, "streak=1") {
		t.Errorf("失败终态行应带老口径 result=fail + fail_reason(含 streak)：result=%q fail_reason=%q",
			failed.Result, failed.FailReason)
	}

	// 子例：错误原文超长 ⇒ 截断 ≤300（字节口径，与 trunca 一致）+ 截断标记
	se2, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 超长错误")
	compactSeed(t, st, se2.ID, 30, strings.Repeat("测", 300))
	longErr := strings.Repeat("e", 400)
	if _, cerr := st.MaybeCompact(context.Background(), se2.ID, "gemma-12B", compactActive(t, st, se2.ID), nil,
		func(context.Context, []map[string]any) (string, error) { return "", errors.New(longErr) }, false); cerr == nil {
		t.Fatal("超长错误也应有 err")
	}
	lines2 := readCompactObsLines(t)
	var failed2 *CompactionObs
	for i := range lines2 {
		if lines2[i].EventName == obsEventCompactionFailed && lines2[i].Session == se2.ID {
			failed2 = lines2[i].Compaction
		}
	}
	if failed2 == nil {
		t.Fatal("应能按会话找到第二条 failed 事件")
	}
	const truncMark = "…（已截断）"
	if !strings.HasSuffix(failed2.ErrText, truncMark) {
		t.Errorf("超长错误原文应带截断标记：%q", failed2.ErrText)
	}
	if len(failed2.ErrText) > 300+len(truncMark) {
		t.Errorf("错误原文截断后应 ≤300 字（+标记）：%d 字节", len(failed2.ErrText))
	}
	t.Logf("✓ 失败路径：started+failed（无 completed）· err=%q", failed.Compaction.ErrText)
}

// ── 用例 ③：**不压缩 ⇒ 一条 compaction 事件都不落**（反例，防误报）────

func TestCompactObsNothingWhenNotCompacting(t *testing.T) {
	st, _ := newCompactStore(t)
	ctx := context.Background()

	// A) 消息数不足（10 条长消息）——token 虽超阈值也不压
	seA, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 反例A 条数不足")
	compactSeed(t, st, seA.ID, 10, strings.Repeat("测", 300))
	if did, err := st.MaybeCompact(ctx, seA.ID, "gemma-12B", compactActive(t, st, seA.ID), nil,
		func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false); did || err != nil {
		t.Fatalf("A 不应压缩：did=%v err=%v", did, err)
	}
	// B) 条数够但 token 不足（30 条极短消息）
	seB, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 反例B token不足")
	compactSeed(t, st, seB.ID, 30, "hi")
	if did, err := st.MaybeCompact(ctx, seB.ID, "gemma-12B", compactActive(t, st, seB.ID), nil,
		func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false); did || err != nil {
		t.Fatalf("B 不应压缩：did=%v err=%v", did, err)
	}

	if got := eventsOf(readCompactObsLines(t)); len(got) != 0 {
		t.Errorf("不压缩 ⇒ 一条 compaction 事件都不许落（防误报），实得 %d 条：%+v", len(got), got)
	}
	if got := legacyCompactRows(readCompactObsLines(t)); len(got) != 0 {
		t.Errorf("不压缩 ⇒ 连旧 OBS-4 终态行也不该有，实得 %d 条", len(got))
	}

	// C) 冷却中的第二次尝试 ⇒ 同样一条都不落（"失败过了在等冷却"≠"又压了一次"）
	origJitter := compactJitterRand
	compactJitterRand = func() float64 { return 1 } // 满档抖动 ⇒ 冷却 60s（否则 jitter≈0 时冷却可能瞬间过期）
	t.Cleanup(func() { compactJitterRand = origJitter })

	seC, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 反例C 冷却中")
	compactSeed(t, st, seC.ID, 30, strings.Repeat("测", 300))
	if _, err := st.MaybeCompact(ctx, seC.ID, "gemma-12B", compactActive(t, st, seC.ID), nil, nil, false); err == nil {
		t.Fatal("C 第一次（无摘要器）应失败并记冷却")
	}
	before := len(eventsOf(readCompactObsLines(t))) // 第一次失败已落 started+failed
	did, err := st.MaybeCompact(ctx, seC.ID, "gemma-12B", compactActive(t, st, seC.ID), nil, nil, false)
	if did || err != nil {
		t.Fatalf("C 冷却中不应再压：did=%v err=%v", did, err)
	}
	if after := len(eventsOf(readCompactObsLines(t))); after != before {
		t.Errorf("冷却中重试不得新增事件：前 %d 条，后 %d 条", before, after)
	}
	t.Logf("✓ 反例：条数不足/token 不足/冷却中三种「不压缩」路径 ⇒ 零条 compaction 事件")
}

// ── 用例 ④：depth 递增（连续两次压缩 ⇒ 第二次 depth=2）────────────────

func TestCompactObsDepthIncrements(t *testing.T) {
	st, _ := newCompactStore(t)
	ctx := context.Background()
	se, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 depth 递增")

	body := strings.Repeat("测", 300)
	compactSeed(t, st, se.ID, 30, body)
	structured := obsStructuredSummary()
	sum := func(context.Context, []map[string]any) (string, error) { return structured, nil }

	if did, err := st.MaybeCompact(ctx, se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, false); err != nil || !did {
		t.Fatalf("第一次压缩应成功：did=%v err=%v", did, err)
	}
	// 再补 10 条长消息（时间戳接在既有之后）⇒ 条数与 token 双双再次超阈值
	ssAddMsgs(t, st, se.ID, 1757501000, body, body, body, body, body, body, body, body, body, body)
	if did, err := st.MaybeCompact(ctx, se.ID, "gemma-12B", compactActive(t, st, se.ID), nil, sum, false); err != nil || !did {
		t.Fatalf("第二次压缩应成功：did=%v err=%v", did, err)
	}

	lines := readCompactObsLines(t)
	var startedSpans []string
	var startedDepths, completedDepths []int
	for _, ln := range lines {
		switch ln.EventName {
		case obsEventCompactionStarted:
			startedSpans = append(startedSpans, ln.SpanID)
			startedDepths = append(startedDepths, ln.Compaction.Depth)
		case obsEventCompactionCompleted:
			completedDepths = append(completedDepths, ln.Compaction.Depth)
		}
	}
	if len(startedDepths) != 2 || len(completedDepths) != 2 {
		t.Fatalf("应各有 2 条 started/completed，实得 %d/%d", len(startedDepths), len(completedDepths))
	}
	if startedDepths[0] != 1 || startedDepths[1] != 2 {
		t.Errorf("compaction_depth 应 1→2 递增，实得 %v", startedDepths)
	}
	if completedDepths[0] != 1 || completedDepths[1] != 2 {
		t.Errorf("completed 的 depth 应与各自 started 配平（1→2），实得 %v", completedDepths)
	}
	if startedSpans[0] == startedSpans[1] {
		t.Errorf("两次压缩是两个原子 ⇒ span 必须不同（实得同一个 %q）", startedSpans[0])
	}
	t.Logf("✓ depth 递增：%v（span 各自独立）", startedDepths)
}

// ── 用例 ⑤：kept/summarized 不重叠且并集 = 原消息集合（小固定集）───────

func TestCompactObsKeptSummarizedPartition(t *testing.T) {
	st, _ := newCompactStore(t)
	se, _ := st.CreateSession("gemma-12B", "desktop", "", "T3.1 集合划分")
	const n = 31                                                  // 首 3 + 尾 20 保护 ⇒ 中间段 = 索引 3..10（8 条）
	ids := compactSeed(t, st, se.ID, n, strings.Repeat("测", 140)) // 31×200=6200 token > 4096 阈值

	wantSummarized := ids[CompactProtectFirstN : n-CompactProtectLastN]
	wantKept := append(append([]int64{}, ids[:CompactProtectFirstN]...), ids[n-CompactProtectLastN:]...)

	if did, err := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID), nil,
		func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false); err != nil || !did {
		t.Fatalf("应压缩成功：did=%v err=%v", did, err)
	}
	cp := mustOneEvent(t, readCompactObsLines(t), obsEventCompactionCompleted).Compaction

	// 小固定集：事件给的集合 == 按保护段算出来的集合
	if got, want := obsSortedIDs(cp.SummarizedIDs), obsSortedIDs(wantSummarized); !obsSameIDs(got, want) {
		t.Errorf("summarized_message_ids=%v，期望 %v", got, want)
	}
	if got, want := obsSortedIDs(cp.KeptIDs), obsSortedIDs(wantKept); !obsSameIDs(got, want) {
		t.Errorf("kept_message_ids=%v，期望 %v", got, want)
	}
	// 不重叠 + 并集 = 原集合
	assertIDPartition(t, cp, ids)

	// 独立裁判：库里的真实归档状态必须与事件里的划分一致（事件不是自说自话）
	all, err := st.ListMessages(se.ID)
	if err != nil {
		t.Fatal(err)
	}
	summSet := map[int64]bool{}
	for _, id := range cp.SummarizedIDs {
		summSet[id] = true
	}
	for _, m := range all {
		if _, tracked := summSet[m.ID]; !tracked {
			// 不在被压集合里的原消息：必须仍在活动窗口且未被标压缩
			if m.Compacted || !m.Active {
				t.Errorf("保留段消息 #%d 不应被归档（active=%v compacted=%v）", m.ID, m.Active, m.Compacted)
			}
			continue
		}
		if !m.Compacted || m.Active {
			t.Errorf("被压缩消息 #%d 应归档（active=%v compacted=%v）", m.ID, m.Active, m.Compacted)
		}
	}
	// turns_summarized：段内 user 条数（compactSeed 角色交替，偶数索引=user）
	wantTurns := 0
	for i := CompactProtectFirstN; i < n-CompactProtectLastN; i++ {
		if i%2 == 0 {
			wantTurns++
		}
	}
	if cp.TurnsSummarized != wantTurns {
		t.Errorf("turns_summarized=%d，期望 %d", cp.TurnsSummarized, wantTurns)
	}
	t.Logf("✓ 划分：保留 %d 条 / 压掉 %d 条（不重叠，并集=%d 条原消息，与库内归档状态一致）",
		len(cp.KeptIDs), len(cp.SummarizedIDs), n)
}

// obsSameIDs — 两个已排序 id 列表是否相同
func obsSameIDs(a, b []int64) bool {
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

// ── 用例 ⑥：field_coverage 只在结构化摘要上出现（拿不到就缺席）─────────

func TestCompactObsFieldCoverageOnlyWhenStructured(t *testing.T) {
	cases := []struct {
		name      string
		lingua    CompactFn
		sum       SummarizeFn
		wantCov   bool
		wantPres  int
		wantTotal int
	}{
		{
			name:      "六段齐（结构化）",
			sum:       func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil },
			wantCov:   true,
			wantPres:  6,
			wantTotal: 6,
		},
		{
			name: "只给三段",
			sum: func(context.Context, []map[string]any) (string, error) {
				return "## Goal 压短\n## Progress - 甲\n## Next Steps - 乙", nil
			},
			wantCov:   true,
			wantPres:  3,
			wantTotal: 6,
		},
		{
			name: "散文（一段都不认）⇒ 缺席，不编造六段全缺",
			sum: func(context.Context, []map[string]any) (string, error) {
				return "这次聊了很多，先这样。", nil
			},
			wantCov: false,
		},
		{
			name: "LLMLingua 删除式（非结构化）⇒ 缺席（拿不到就是拿不到）",
			lingua: func(context.Context, string) (string, error) {
				return "删除式压缩正文（没有任何段名）", nil
			},
			sum:     func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil },
			wantCov: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := newCompactStore(t)
			se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.1 覆盖-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			compactSeed(t, st, se.ID, 30, strings.Repeat("测", 300))
			if did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", compactActive(t, st, se.ID),
				tc.lingua, tc.sum, false); cerr != nil || !did {
				t.Fatalf("应压缩成功：did=%v err=%v", did, cerr)
			}
			cp := mustOneEvent(t, readCompactObsLines(t), obsEventCompactionCompleted).Compaction
			if !tc.wantCov {
				if cp.FieldCoverage != nil {
					t.Errorf("拿不到结构化段名时 field_coverage 必须**缺席**，实得 %+v", *cp.FieldCoverage)
				}
				return
			}
			if cp.FieldCoverage == nil {
				t.Fatal("结构化摘要应带 field_coverage")
			}
			if cp.FieldCoverage.Present != tc.wantPres || cp.FieldCoverage.Total != tc.wantTotal {
				t.Errorf("present/total=%d/%d，期望 %d/%d",
					cp.FieldCoverage.Present, cp.FieldCoverage.Total, tc.wantPres, tc.wantTotal)
			}
		})
	}
}

// ── 口径守卫：三态事件名与触发原因闭集是**公开契约**（改动须同步观测面文档）──

func TestCompactObsContractStrings(t *testing.T) {
	if obsEventCompactionStarted != "compaction_started" ||
		obsEventCompactionCompleted != "compaction_completed" ||
		obsEventCompactionFailed != "compaction_failed" {
		t.Fatalf("三态事件名已变：%s / %s / %s",
			obsEventCompactionStarted, obsEventCompactionCompleted, obsEventCompactionFailed)
	}
	if CompactTriggerTokenBudget != "token_budget" ||
		CompactTriggerManual != "manual" ||
		CompactTriggerErrorRecovery != "error_recovery" {
		t.Fatalf("trigger_reason 闭集已变：%s / %s / %s",
			CompactTriggerTokenBudget, CompactTriggerManual, CompactTriggerErrorRecovery)
	}
	// 三态事件名必须与整族前缀一致（读侧按前缀挑行）
	for _, e := range []string{obsEventCompactionStarted, obsEventCompactionCompleted, obsEventCompactionFailed} {
		if !strings.HasPrefix(e, compactEventPrefix) {
			t.Errorf("事件名 %q 不满足本族前缀 %q", e, compactEventPrefix)
		}
	}
	// 显式触发原因：非法取值被忽略（不编造），合法取值生效
	if got := compactTriggerReasonOf(true, nil); got != CompactTriggerManual {
		t.Errorf("force 且未声明 ⇒ manual，实得 %q", got)
	}
	if got := compactTriggerReasonOf(false, nil); got != CompactTriggerTokenBudget {
		t.Errorf("非 force 且未声明 ⇒ token_budget，实得 %q", got)
	}
	if got := compactTriggerReasonOf(false, []CompactOption{WithTriggerReason(CompactTriggerErrorRecovery)}); got != CompactTriggerErrorRecovery {
		t.Errorf("显式声明应优先，实得 %q", got)
	}
	if got := compactTriggerReasonOf(false, []CompactOption{WithTriggerReason("随便编的")}); got != CompactTriggerTokenBudget {
		t.Errorf("非法取值应被忽略并退回推导，实得 %q", got)
	}
}
