package loopcore_test

// gap15_loop_e2e_test.go — GAP-20261017-15 主牙（端到端，走真内核 + 真 chat 解析面）。
//
// 走的是**生产接线**：loopcore.Deps.Tools 不给（chat 侧就是 nil），推理经 chat.InferStream
// 打到假引擎，由 chat 的正文兜底认工具；内核执行工具后进入下一轮。
// 判据：连续两轮都吐「正文 + 裸参数对象」⇒ 两轮各执行 1 次（旧版只有第 1 轮能执行）。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/loopcore"
)

const gap15Bare = "我先跑判据。\n{\"command\":\"zerg gate check gen-gap-ledger --check\"}"

func TestGap15TwoRoundsBothBareArgsEachExecutes(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if _, hasTools := m["tools"]; hasTools {
			t.Errorf("生产接线不该带 tools 字段（第 %d 次请求）", n+1)
		}
		n++
		content := gap15Bare
		if n >= 3 { // 第三轮起模型交结论
			content = "rc=0，仍缺。"
		}
		esc, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "text/event-stream")
		var sb strings.Builder
		sb.WriteString(`data: {"choices":[{"delta":{"role":"assistant","content":` + string(esc) + `},"finish_reason":null}]}` + "\n\n")
		sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		sb.WriteString("data: [DONE]\n\n")
		_, _ = w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	ci := chat.NewChatInfer(srv.URL, "")
	infer := func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*loopcore.Response, error) {
		ir, err := ci.InferStream(chat.WithSessionID(ctx, ""), model, sysPrompt, msgs, onDelta, tools)
		if err != nil {
			return nil, err
		}
		kr := &loopcore.Response{Content: ir.Content, Reasoning: ir.Reasoning, Finish: "stop"}
		for _, tc := range ir.ToolCalls {
			kr.ToolCalls = append(kr.ToolCalls, loopcore.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args, RawArgs: tc.RawArgs})
		}
		return kr, nil
	}
	execN := 0
	exec := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		execN++
		if cmd, _ := args["command"].(string); !strings.HasPrefix(cmd, "zerg ") {
			t.Errorf("工具参数取错: %v", args)
		}
		return "rc=0", "5ms", nil
	}
	res := loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 6}, "m", "sys",
		[]map[string]any{{"role": "user", "content": "跑：zerg gate check gen-gap-ledger --check"}},
		loopcore.Deps{Infer: infer, Exec: exec}) // Tools 不给 = 生产现状

	if execN != 2 {
		t.Fatalf("连续两轮裸参数对象 ⇒ 应各执行 1 次（共 2 次），实际 %d 次；exit=%s", execN, res.ExitKind)
	}
	if res.ExitKind != "natural" {
		t.Fatalf("第三轮交结论后应 natural，实际 %q", res.ExitKind)
	}
}
