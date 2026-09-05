package loopcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"zerg/core/internal/agent"
)

// mockInfer — 逐轮脚本化响应
func mockInfer(responses []agent.ModelResponse) (Infer, *int) {
	i := 0
	return func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*agent.ModelResponse, error) {
		r := responses[min(i, len(responses)-1)]
		i++
		cp := r
		return &cp, nil
	}, &i
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRunNaturalFinish(t *testing.T) {
	infer, _ := mockInfer([]agent.ModelResponse{{Content: "答案"}})
	d := Deps{Infer: infer}
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, d)
	if res.ExitKind != "natural" || res.Content != "答案" {
		t.Fatalf("natural: %+v", res)
	}
}

func TestRunToolThenFinish(t *testing.T) {
	infer, _ := mockInfer([]agent.ModelResponse{
		{ToolCalls: []agent.ToolCall{{ID: "c1", Name: "bash", Args: map[string]any{"command": "ls"}, RawArgs: `{"command":"ls"}`}}},
		{Content: "完成"},
	})
	execCount := 0
	d := Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execCount++
			return "file1\nfile2", "10ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 5}, "m", "sys", nil, d)
	if res.ExitKind != "natural" || execCount != 1 || len(res.Traces) != 1 {
		t.Fatalf("tool+finish: kind=%s exec=%d traces=%d", res.ExitKind, execCount, len(res.Traces))
	}
	// [成功·N字] 标注在消息流
	if !strings.Contains(res.Traces[0].Result, "file1") {
		t.Fatal("trace result 应含工具输出")
	}
}

func TestRunLoopguardEscalate(t *testing.T) {
	same := agent.ModelResponse{ToolCalls: []agent.ToolCall{{ID: "c", Name: "bash", Args: map[string]any{"command": "same"}, RawArgs: "{}"}}}
	infer, _ := mockInfer([]agent.ModelResponse{same, same, same, same, same})
	d := Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "out", "1ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 10}, "m", "sys", nil, d)
	if res.ExitKind != "loopguard_escalate" {
		t.Fatalf("重复 3 次应升级收尾: kind=%s", res.ExitKind)
	}
}

func TestRunEmptyArgsEscalate(t *testing.T) {
	empty := agent.ModelResponse{ToolCalls: []agent.ToolCall{{ID: "c", Name: "tool_a", Args: map[string]any{}}}}
	infer, _ := mockInfer([]agent.ModelResponse{empty, empty, empty, empty})
	d := Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "out", "1ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 10}, "m", "sys", nil, d)
	if res.ExitKind != "empty_args" {
		t.Fatalf("连续空参数应收尾: kind=%s", res.ExitKind)
	}
}

func TestRunWallClock(t *testing.T) {
	slow := agent.ModelResponse{ToolCalls: []agent.ToolCall{{ID: "c", Name: "tool_a", Args: map[string]any{"x": 1}}}}
	infer, _ := mockInfer([]agent.ModelResponse{slow, slow, {Content: "done"}})
	d := Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			time.Sleep(30 * time.Millisecond)
			return "out", "30ms", nil
		},
	}
	res := Run(context.Background(), Config{MaxRounds: 10, WallClock: 50 * time.Millisecond}, "m", "sys", nil, d)
	if res.ExitKind != "wall_clock" {
		t.Fatalf("墙钟应触发: kind=%s", res.ExitKind)
	}
}

func TestCompactToolResults(t *testing.T) {
	var msgs []map[string]any
	for i := 0; i < 10; i++ {
		msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": string(rune('a'+i)), "content": strings.Repeat("x", 200)})
	}
	out := CompactToolResults(msgs, 3)
	full := 0
	for _, m := range out {
		if c, _ := m["content"].(string); len(c) == 200 {
			full++
		}
	}
	if full != 3 {
		t.Fatalf("最近 3 个保留完整: %d", full)
	}
}
