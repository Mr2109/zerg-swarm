package adapter

// served_headers_test.go —— 缺口 GAP-20261019-01 的网关侧两态：
// 非流式本来就走 CopyResponse 全量复制；**流式 CopySSE 原本一个上游头都不复制**
// ⇒ 这里钉死「X-Zerg-Served-* 五格必须出现在流式响应头里」，否则 chat/ask 流式面
// 永远拿不到「谁在应答」。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func servedResp() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":         []string{"text/event-stream"},
			"X-Zerg-Served-Model":  []string{"Qwen3.8-Flash-Next"},
			"X-Zerg-Served-Host":   []string{"edge-node"},
			"X-Zerg-Served-Port":   []string{"9400"},
			"X-Zerg-Served-Pid":    []string{"1913944"},
			"X-Zerg-Served-Pinned": []string{"default"},
			"X-Other-Upstream":     []string{"should-not-leak"},
		},
		Body: io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}
}

func TestCopySSE_CarriesServedHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	CopySSE(rec, servedResp())
	h := rec.Header()
	for k, want := range map[string]string{
		"X-Zerg-Served-Model":  "Qwen3.8-Flash-Next",
		"X-Zerg-Served-Host":   "edge-node",
		"X-Zerg-Served-Port":   "9400",
		"X-Zerg-Served-Pid":    "1913944",
		"X-Zerg-Served-Pinned": "default",
	} {
		if got := h.Get(k); got != want {
			t.Fatalf("流式响应头 %s 应为 %q，实得 %q", k, want, got)
		}
	}
	// 只挑 X-Zerg-Served-*：别的上游头不跟着进（流式头由本层自己定）。
	if got := h.Get("X-Other-Upstream"); got != "" {
		t.Fatalf("不该复制非 Served 的上游头，实得 %q", got)
	}
}

func TestCopyResponse_CarriesServedHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	CopyResponse(rec, servedResp())
	if got := rec.Header().Get("X-Zerg-Served-Model"); got != "Qwen3.8-Flash-Next" {
		t.Fatalf("非流式由全量复制带出，实得 %q", got)
	}
}
