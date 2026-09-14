// gateway_model_failure_test.go —— P5：熔断口径分家（设计-子端服务切换 §11 M4）。
//
// 背景（2026-09-14 实测）：DS4@1M 遭遇引擎自报的 rocm prefill failed，被记为
// machine x3 forward failed (8/8) 并 demote —— 而问题只在"该模型 + 该上下文"，
// 不该牵连整机。修法：模型级失败走 modelFailCounts（只记原因、不涨熔断）。
package gateway

import (
	"testing"
	"time"
)

func newModelFailureTestGateway() *Gateway {
	return &Gateway{
		failCounts:      map[string]int{},
		modelFailCounts: map[string]int{},
		failSince:       map[string]time.Time{},
		lastErr:         map[string]string{},
		lastErrAt:       map[string]time.Time{},
	}
}

func TestMarkModelFailure_DoesNotCountTowardBreaker(t *testing.T) {
	g := newModelFailureTestGateway()
	// 真实响应体（今天实测原文）
	g.markModelFailure("x3", `backend x3 returned 500: {"error":{"message":"rocm prefill failed"}}`)

	if got := g.failCounts["x3"]; got != 0 {
		t.Fatalf("模型级失败**不得**涨熔断计数（§11 M4），实得 failCounts=%d", got)
	}
	if got := g.modelFailCounts["x3"]; got != 1 {
		t.Fatalf("应记入模型级计数，实得 %d", got)
	}
	if g.lastErr["x3"] == "" {
		t.Fatal("应记录失败原因（供 /api/gateway/breakers 取证）")
	}
}

func TestMarkModelFailure_ManyTimesStillNoTrip(t *testing.T) {
	// 今天正是 8 次模型级失败触发了 demote；这里断言"再多次也不会"。
	g := newModelFailureTestGateway()
	for i := 0; i < 50; i++ {
		g.markModelFailure("x3", "rocm prefill failed")
	}
	if got := g.failCounts["x3"]; got != 0 {
		t.Fatalf("50 次模型级失败也不该涨熔断计数，实得 %d", got)
	}
	if got := g.modelFailCounts["x3"]; got != 50 {
		t.Fatalf("模型级计数应为 50，实得 %d", got)
	}
}

func TestMarkFailure_StillCountsTowardBreaker(t *testing.T) {
	// 对照组：真正的上游失败仍走 markFailure 并涨计数——口径分家不得把熔断架空。
	g := newModelFailureTestGateway()
	g.markFailure("x3", "dial tcp <worker-ip>:8100: connect: no route to host")
	if got := g.failCounts["x3"]; got != 1 {
		t.Fatalf("上游失败应涨熔断计数，实得 %d", got)
	}
	if got := g.modelFailCounts["x3"]; got != 0 {
		t.Fatalf("上游失败不该记入模型级计数，实得 %d", got)
	}
}
