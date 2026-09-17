// obs_trace_prop_test.go —— T1.6：入站沿用的上游 trace 必须写进**本轮事件**（轮次记录同一条 trace_id）。
//
// 验收落点：任务表 T1.6「事件带 W3C traceparent 语义」+ 本任务「入站读取并写进事件」。
// 两条判据分开：
//
//	① 正面：会话被绑定上游 trace ⇒ 该会话的轮次事件（kind=turn）trace_id == 上游那条（且逐轮不变）。
//	② 负控：**没绑定**的会话 ⇒ 必须自己生成（不能等于上游那条）——证明①不是"全都会等于"的假绿。
package chat

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// 上游给的 traceparent（W3C 规范示例）—— 与 gateway/tracectx 用例同一常量值，便于人工对账。
const propUpstreamTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"

func TestObsTurnAdoptsUpstreamTrace(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	upstream := tracectx.TraceContext{
		TraceID: propUpstreamTraceID,
		SpanID:  "00f067aa0ba902b7",
		Flags:   tracectx.FlagSampled,
	}
	session := "sess-prop-upstream-" + t.Name()
	if !tracectx.BindSession(session, upstream) {
		t.Fatal("绑定上游 trace 失败")
	}

	// 入站后的第一轮 + 第二轮：两轮的 trace_id 都必须等于上游那条（第一轮采纳、第二轮沿用）
	NewObsTimer(session, 1, "m").Finish("finish")
	NewObsTimer(session, 2, "m").Finish("finish")

	// 负控：另一个**没有**绑定的会话（证明下一条断言不是恒真）
	other := "sess-prop-nobind-" + t.Name()
	NewObsTimer(other, 1, "m").Finish("finish")

	lines := readObsTraceLines(t)
	if len(lines) != 3 {
		t.Fatalf("应落 3 行，实际 %d", len(lines))
	}
	var turns []obsTraceLine
	var otherLine obsTraceLine
	for _, l := range lines {
		if l.Session == session {
			turns = append(turns, l)
		} else {
			otherLine = l
		}
	}
	if len(turns) != 2 {
		t.Fatalf("本会话应落 2 行轮次事件，实际 %d", len(turns))
	}
	for i, l := range turns {
		if l.TraceID != propUpstreamTraceID {
			t.Errorf("第 %d 轮的 trace_id 没落在上游那条链上：%q（应 %q）—— 这就是“写进本轮事件”的判据",
				i+1, l.TraceID, propUpstreamTraceID)
		}
	}
	if turns[0].TraceID != turns[1].TraceID {
		t.Errorf("同会话逐轮 trace_id 必须不变（T1.1 不变式）：%q / %q", turns[0].TraceID, turns[1].TraceID)
	}
	if otherLine.TraceID == "" || otherLine.TraceID == propUpstreamTraceID {
		t.Errorf("没绑定的会话必须自己生成（负控）：%q", otherLine.TraceID)
	}
	// 轮次事件自身的 32 位内部 span 链不受影响（传播只统一 trace_id，不混两个 id 空间）
	if len(turns[0].SpanID) != obsIDHexLen || turns[0].ParentSpanID == "" {
		t.Errorf("内部 span 链应保持既有口径：%+v", turns[0])
	}
}
