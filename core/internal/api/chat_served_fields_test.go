package api

// chat_served_fields_test.go —— 缺口 GAP-20261019-01 的流式可分辨面。
//
// 流式客户端只看得到 SSE 帧（响应头在浏览器/桌面端拿不到）⇒ 「谁在应答」五格必须进
// `done` 帧；缺席时**不写空串**（不编造）。

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

func TestChatDonePayload_CarriesServedIdentity(t *testing.T) {
	res := &chat.InferResult{
		InputTokens: 1, OutputTokens: 2, ReasoningTokens: 0,
		ServedModel: "Qwen3.8-Flash-Next", ServedHost: "edge-node",
		ServedPort: "9400", ServedPid: "1913944", ServedPinned: "default",
	}
	p := chatDonePayload(7, "t", "正文", "", res)
	for k, want := range map[string]string{
		"served_model": "Qwen3.8-Flash-Next",
		"served_host":  "edge-node",
		"served_port":  "9400",
		"served_pid":   "1913944",
		"pinned":       "default",
	} {
		if got, _ := p[k].(string); got != want {
			t.Fatalf("done 帧 %s 应为 %q，实得 %#v", k, want, p[k])
		}
	}
}

func TestChatDonePayload_OmitsServedWhenAbsent(t *testing.T) {
	p := chatDonePayload(7, "t", "正文", "", &chat.InferResult{})
	for _, k := range []string{"served_model", "served_host", "served_port", "served_pid", "pinned"} {
		if _, ok := p[k]; ok {
			t.Fatalf("上游没给时不得写 %s（缺席 = 缺席，不写空串充数）", k)
		}
	}
}
