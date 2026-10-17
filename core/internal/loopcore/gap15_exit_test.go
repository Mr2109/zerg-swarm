package loopcore

// gap15_exit_test.go — GAP-20261017-15（P0）退出判定面回归。
//
// 两条必给的反向断言：
//  ② 真无 JSON 的真坏格式仍判 bad_format（不静默吞、不记 natural）；
//  ④ 零调用 + 零结论 ⇒ **不得**记 natural（natural 的语义是「已交付终答」）。

import (
	"context"
	"testing"
)

// TestGap15ZeroCallsZeroConclusionNotNatural — 补轮之后仍零正文 ⇒ ExitKind 必须不是 natural。
func TestGap15ZeroCallsZeroConclusionNotNatural(t *testing.T) {
	infer, calls := mockInfer([]Response{{Content: ""}})
	res := Run(context.Background(), Config{MaxRounds: 5}, "m", "sys", nil, Deps{Infer: infer})
	if res.ExitKind == "natural" {
		t.Fatalf("零调用零结论不得记 natural（Content=%q，推理次数=%d）", res.Content, *calls)
	}
	if res.ExitKind != "no_conclusion" {
		t.Fatalf("应记 no_conclusion，实际 %q", res.ExitKind)
	}
	// 零回归：空产出**不加料**（既有契约 output_claims_test 钉着「空产出 ⇒ 原样返回」）——
	// 报白走 ExitKind（观测面 / CLI 侧读字段），不往正文里塞人造文本。
	if res.Content != "" {
		t.Fatalf("空产出不应被加料，实际 %q", res.Content)
	}
}

// TestGap15ZeroCallsWithAnswerStaysNatural — 反例：真交了结论仍记 natural（防误伤）。
func TestGap15ZeroCallsWithAnswerStaysNatural(t *testing.T) {
	infer, _ := mockInfer([]Response{{Content: "rc=0；结论：仍缺。"}})
	res := Run(context.Background(), Config{MaxRounds: 5}, "m", "sys", nil, Deps{Infer: infer})
	if res.ExitKind != "natural" {
		t.Fatalf("交了结论应仍为 natural，实际 %q", res.ExitKind)
	}
}

// TestGap15RealBadFormatStillBadFormat — 反向②：模型一直吐解不出的工具调用 ⇒ 连续 3 次后判 bad_format。
func TestGap15RealBadFormatStillBadFormat(t *testing.T) {
	infer, _ := mockInfer([]Response{{Content: "<tool_call>\n{不是 JSON\n</tool_call>"}})
	res := Run(context.Background(), Config{MaxRounds: 6}, "m", "sys", nil, Deps{Infer: infer})
	if res.ExitKind != "bad_format" {
		t.Fatalf("真坏格式应判 bad_format，实际 %q", res.ExitKind)
	}
}
