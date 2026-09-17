// obs_trace_test.go — T1.1 追踪骨架（九字段）实证：trace/span/parent 能拼成父子、时间戳与两个布尔可读回。
//
// 纪律（与 obs_test.go 同源）：测试绝不写用户真实状态 ⇒ 一律 t.Setenv("ZERG_STATE_DIR", t.TempDir()) 隔离。
// 判据分两层，缺一不可：**wire 层**（键真的出现在 JSONL 原文里）与 **read 层**（能被解回来、值是真的）。
// 只断言 Go 结构体字段 = 自证，不算证据。
package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// obsIDHexLen — crypto/rand 16 字节 ⇒ 32 位十六进制
const obsIDHexLen = 32

// obsTraceLine — 读回用的宽解结构（键缺失 ⇒ 零值/nil，这正是「缺席」的判据）
type obsTraceLine struct {
	TS           string `json:"ts"`
	Kind         string `json:"kind"`
	Session      string `json:"session"`
	Round        int    `json:"round"`
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id"`
	SpanKind     string `json:"span_kind"`
	StartedAtMS  int64  `json:"started_at_unix_ms"`
	EndedAtMS    int64  `json:"ended_at_unix_ms"`
	Recorded     *bool  `json:"recorded"`
	Sampled      *bool  `json:"sampled"`
	EventName    string `json:"event_name"`
}

// readObsTraceLines — 逐行解 chat_obs.jsonl；**解不动即红**（读侧宽容这一条由它守住）
func readObsTraceLines(t *testing.T) []obsTraceLine {
	t.Helper()
	raw := readObs(t)
	if raw == "" {
		t.Fatal("观测文件未写出")
	}
	var out []obsTraceLine
	for _, ln := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r obsTraceLine
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON（旧行与新行都必须能解析）：%v\n%s", err, ln)
		}
		out = append(out, r)
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

// ① 九字段都写入且能读回（走真实轮次路径：NewObsTimer → Finish）
func TestObsTraceSkeletonNineFields(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	before := time.Now().UnixMilli()
	tm := NewObsTimer("sess-trace-a", 1, "gemma-4-26B")
	tm.MarkChunk()
	tm.Finish("finish")
	after := time.Now().UnixMilli()

	// ①-1 wire 层：九个键必须在落盘原文里出现
	raw := readObs(t)
	for _, key := range []string{`"trace_id"`, `"span_id"`, `"parent_span_id"`, `"span_kind"`,
		`"started_at_unix_ms"`, `"ended_at_unix_ms"`, `"recorded"`, `"sampled"`, `"event_name"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("落盘行缺少键 %s\n原文：%s", key, raw)
		}
	}

	// ①-2 read 层：值是实的，不是空串/零值占位
	lines := readObsTraceLines(t)
	if len(lines) != 1 {
		t.Fatalf("应落 1 行，实际 %d 行：%s", len(lines), raw)
	}
	r := lines[0]
	if len(r.TraceID) != obsIDHexLen || len(r.SpanID) != obsIDHexLen {
		t.Errorf("trace_id/span_id 应是 %d 位 hex：trace=%q span=%q", obsIDHexLen, r.TraceID, r.SpanID)
	}
	if r.ParentSpanID == "" || r.ParentSpanID == r.SpanID {
		t.Errorf("首轮的 parent_span_id 应指会话根（非空且不同于本轮 span）：parent=%q span=%q", r.ParentSpanID, r.SpanID)
	}
	if r.SpanKind != ObsSpanKindServer {
		t.Errorf("turn 的 span_kind 应为 %q，实际 %q", ObsSpanKindServer, r.SpanKind)
	}
	if r.StartedAtMS < before || r.EndedAtMS > after || r.EndedAtMS < r.StartedAtMS {
		t.Errorf("时间戳不自洽：started=%d ended=%d，窗口=[%d,%d]", r.StartedAtMS, r.EndedAtMS, before, after)
	}
	if r.Recorded == nil || !*r.Recorded || r.Sampled == nil || !*r.Sampled {
		t.Errorf("默认应「已记录 + 已采样」：recorded=%v sampled=%v", r.Recorded, r.Sampled)
	}
	if r.EventName != "turn" {
		t.Errorf("event_name 默认应取 kind（turn），实际 %q", r.EventName)
	}
}

// ② 同会话两轮：trace 不变、span 每轮新生成、parent 指上一轮（首轮指会话根）；工具事件挂当前轮且不顶掉父指针
func TestObsTraceParentChildAcrossRounds(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	NewObsTimer("sess-trace-b", 1, "m").Finish("finish")
	// 第 2 轮：计时器先开 ⇒ 轮次进行中发生工具事件（真实的调用顺序；事件写在 Finish 之前）
	t2 := NewObsTimer("sess-trace-b", 2, "m")
	ObsTool("sess-trace-b", 2, "bash", "12ms", true, 1, MaxToolRounds)
	t2.Finish("finish")
	NewObsTimer("sess-trace-b", 3, "m").Finish("finish")
	NewObsTimer("sess-trace-c", 1, "m").Finish("finish") // 另一会话 ⇒ 必须另起一条 trace

	lines := readObsTraceLines(t)
	if len(lines) != 5 {
		t.Fatalf("应落 5 行，实际 %d 行", len(lines))
	}
	turns := map[int]obsTraceLine{}
	var tool obsTraceLine
	for _, l := range lines {
		switch {
		case l.Session == "sess-trace-b" && l.Kind == "turn":
			turns[l.Round] = l
		case l.Session == "sess-trace-b":
			tool = l
		}
	}
	r1, r2, r3 := turns[1], turns[2], turns[3]
	if r1.TraceID == "" {
		t.Fatalf("会话轮次没有 trace_id：%+v", r1)
	}
	if r1.TraceID != r2.TraceID || r2.TraceID != r3.TraceID {
		t.Errorf("同一会话的 trace_id 必须保持不变：%q / %q / %q", r1.TraceID, r2.TraceID, r3.TraceID)
	}
	if r1.SpanID == r2.SpanID || r2.SpanID == r3.SpanID || r1.SpanID == r3.SpanID {
		t.Errorf("每轮必须新生成 span_id：%q / %q / %q", r1.SpanID, r2.SpanID, r3.SpanID)
	}
	if r2.ParentSpanID != r1.SpanID {
		t.Errorf("第 2 轮的 parent 应指第 1 轮的 span：parent=%q r1.span=%q", r2.ParentSpanID, r1.SpanID)
	}
	if r3.ParentSpanID != r2.SpanID {
		t.Errorf("第 3 轮的 parent 应指第 2 轮的 span（工具事件不得顶掉父指针）：parent=%q r2.span=%q", r3.ParentSpanID, r2.SpanID)
	}
	if r1.ParentSpanID == "" || r1.ParentSpanID == r1.SpanID ||
		r1.ParentSpanID == r2.SpanID || r1.ParentSpanID == r3.SpanID {
		t.Errorf("第 1 轮的 parent 应指会话根（独立于各轮 span）：parent=%q", r1.ParentSpanID)
	}
	if tool.TraceID != r1.TraceID {
		t.Errorf("工具事件应属同一 trace：%q vs %q", tool.TraceID, r1.TraceID)
	}
	if tool.ParentSpanID != r2.SpanID {
		t.Errorf("工具事件应挂在**发生时的当前轮**（第 2 轮）之下：parent=%q r2.span=%q", tool.ParentSpanID, r2.SpanID)
	}
	if tool.SpanKind != ObsSpanKindInternal || tool.EventName != "tool" {
		t.Errorf("工具事件的 span_kind/event_name 不符：%q / %q", tool.SpanKind, tool.EventName)
	}
	if tool.SpanID == "" || tool.SpanID == r2.SpanID {
		t.Errorf("工具事件自己也要有独立 span：%q（r2.span=%q）", tool.SpanID, r2.SpanID)
	}
	for _, l := range lines {
		if l.Session != "sess-trace-c" {
			continue
		}
		if l.TraceID == "" || l.TraceID == r1.TraceID {
			t.Errorf("不同会话必须另起 trace：得到 %q（sess-trace-b=%q）", l.TraceID, r1.TraceID)
		}
	}
}

// ③ recorded / sampled 两布尔独立可写（显式 false 也必须落盘，不被默认值吞掉）
func TestObsRecordedSampledIndependent(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	obsWrite(ObsRecord{Kind: "debug", Session: "sess-trace-d", EventName: "zerg.debug.a",
		Recorded: boolPtr(true), Sampled: boolPtr(false)})
	obsWrite(ObsRecord{Kind: "debug", Session: "sess-trace-d", EventName: "zerg.debug.b",
		Recorded: boolPtr(false), Sampled: boolPtr(true)})

	raw := readObs(t)
	if strings.Count(raw, `"recorded"`) != 2 || strings.Count(raw, `"sampled"`) != 2 {
		t.Errorf("显式给出的布尔必须逐行落盘（缺席就等于不可独立写）：%s", raw)
	}
	lines := readObsTraceLines(t)
	if len(lines) != 2 {
		t.Fatalf("应落 2 行，实际 %d 行：%s", len(lines), raw)
	}
	a, b := lines[0], lines[1]
	if a.Recorded == nil || !*a.Recorded || a.Sampled == nil || *a.Sampled {
		t.Errorf("A 应为 recorded=true sampled=false，实际 recorded=%v sampled=%v", a.Recorded, a.Sampled)
	}
	if b.Recorded == nil || *b.Recorded || b.Sampled == nil || !*b.Sampled {
		t.Errorf("B 应为 recorded=false sampled=true，实际 recorded=%v sampled=%v", b.Recorded, b.Sampled)
	}
	// 「非空 event_name 即事件」+ 调用方给的事件名不得被覆盖
	if a.EventName != "zerg.debug.a" || b.EventName != "zerg.debug.b" {
		t.Errorf("调用方给的 event_name 不得被覆盖：%q / %q", a.EventName, b.EventName)
	}
}

// ④ 缺席而不是编造：无会话 ⇒ 九字段**整块**缺席（不做半套骨架）；且旧格式行读侧必须照样能解析
func TestObsTraceAbsentWithoutSession(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 口径：九字段里七个都是会话级的 ⇒ 会话都拿不到时不做「半套骨架」，
	// 整块缺席（连 event_name 一起），这条记录就不算事件 —— 宁可缺席，不拼一条半真的出来。
	obsWrite(ObsRecord{Kind: "debug"})
	raw := readObs(t)
	for _, key := range []string{`"trace_id"`, `"span_id"`, `"parent_span_id"`, `"span_kind"`,
		`"started_at_unix_ms"`, `"ended_at_unix_ms"`, `"recorded"`, `"sampled"`, `"event_name"`} {
		if strings.Contains(raw, key) {
			t.Errorf("无会话时 %s 必须缺席（不编造、也不拼半套骨架）：%s", key, raw)
		}
	}
	lines := readObsTraceLines(t)
	if len(lines) != 1 {
		t.Fatalf("应落 1 行，实际 %d 行：%s", len(lines), raw)
	}
	if !strings.Contains(raw, `"kind":"debug"`) {
		t.Errorf("既有字段不受影响，仍须落盘：%s", raw)
	}

	// 兼容纪律：新增键只是**附加**键 ⇒ 旧行（无九字段）必须原样可解析
	legacy := `{"ts":"2026-01-01T00:00:00Z","kind":"turn","session":"sess-old","round":1,"end_reason":"finish"}`
	var got obsTraceLine
	if err := json.Unmarshal([]byte(legacy), &got); err != nil {
		t.Fatalf("旧格式行必须照样能解析：%v", err)
	}
	if got.Session != "sess-old" || got.Round != 1 || got.TraceID != "" || got.Recorded != nil {
		t.Errorf("旧行应读成「原字段在、骨架字段缺席」：%+v", got)
	}
}
