package loopcore

import (
	"context"
	"strings"
	"testing"
)

// TestReasoningAccumulatesAcrossRounds — 修复"思考内容不全"的回归测试:
// 多轮工具调用时,每轮思考必须累积(此前每轮覆盖 → 只剩末轮)
func TestReasoningAccumulatesAcrossRounds(t *testing.T) {
	calls := 0
	d := Deps{
		Infer: func(ctx context.Context, model, sys string, msgs []map[string]any,
			onDelta func(string, string), tools []map[string]any) (*Response, error) {
			calls++
			if calls == 1 {
				return &Response{
					Content:   "",
					Reasoning: "第一轮思考:需要先查系统信息",
					Finish:    "tool_calls",
					ToolCalls: []ToolCall{{ID: "c1", Name: "sys_info", Args: map[string]any{}}},
				}, nil
			}
			return &Response{
				Content:   "最终回答:系统正常",
				Reasoning: "第二轮思考:信息够了,可以总结",
				Finish:    "stop",
			}, nil
		},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "ok", "1ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 5}, "m", "s", []map[string]any{{"role": "user", "content": "查系统"}}, d)
	if !strings.Contains(res.Reasoning, "第一轮思考") || !strings.Contains(res.Reasoning, "第二轮思考") {
		t.Fatalf("思考未跨轮累积（不全）: %q", res.Reasoning)
	}
	if res.Reasoning != "第一轮思考:需要先查系统信息\n\n第二轮思考:信息够了,可以总结" {
		t.Fatalf("累积格式异常: %q", res.Reasoning)
	}
	t.Logf("✓ 跨轮思考累积: %d 字", len([]rune(res.Reasoning)))
}

// TestIntermediateRoundContentKeptAsReasoning — 中间轮正文(思考)不丢
func TestIntermediateRoundContentKeptAsReasoning(t *testing.T) {
	calls := 0
	d := Deps{
		Infer: func(ctx context.Context, model, sys string, msgs []map[string]any,
			onDelta func(string, string), tools []map[string]any) (*Response, error) {
			calls++
			if calls == 1 {
				return &Response{
					Content:   "先看看系统信息再回答——这是过程文本",
					Reasoning: "轮1思考",
					Finish:    "tool_calls",
					ToolCalls: []ToolCall{{ID: "c1", Name: "sys_info", Args: map[string]any{}}},
				}, nil
			}
			return &Response{Content: "最终答案：一切正常", Reasoning: "轮2思考", Finish: "stop"}, nil
		},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "ok", "1ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 5}, "m", "s", []map[string]any{{"role": "user", "content": "查系统"}}, d)
	if res.Content != "最终答案：一切正常" {
		t.Fatalf("正文应为末轮回答: %q", res.Content)
	}
	for _, want := range []string{"轮1思考", "先看看系统信息再回答", "轮2思考"} {
		if !strings.Contains(res.Reasoning, want) {
			t.Fatalf("过程文本丢失 %q: %q", want, res.Reasoning)
		}
	}
	t.Logf("✓ 中间轮正文归入思考: %q", res.Reasoning)
}
