// sse_usage_tee_test.go — 丙批 N4 补齐：流式末块缓存计量提取（2026-09-10）
package gateway

import (
	"io"
	"strings"
	"testing"
)

type fakeRC struct{ io.Reader }

func (fakeRC) Close() error { return nil }

func TestSSEUsageTeeExtractsLastTimings(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\n" +
		"data: {\"choices\":[{\"finish_reason\":\"stop\",\"delta\":{}}],\"timings\":{\"cache_n\":65,\"prompt_n\":4}}\n\n" +
		"data: [DONE]\n\n"
	tee := newSSEUsageTee(fakeRC{strings.NewReader(sse)})
	if _, err := io.ReadAll(tee); err != nil {
		t.Fatal(err)
	}
	got := string(tee.UsageJSON())
	if !strings.Contains(got, "\"cache_n\":65") {
		t.Fatalf("应提取末块 timings，实际: %q", got)
	}
	if strings.Contains(got, "\"content\":\"a\"") {
		t.Fatalf("不应取到普通内容块: %q", got)
	}
	t.Logf("✓ 流式末块计量提取: %s", got)
}

func TestSSEUsageTeeNoUsageReturnsNil(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n\n"
	tee := newSSEUsageTee(fakeRC{strings.NewReader(sse)})
	_, _ = io.ReadAll(tee)
	if b := tee.UsageJSON(); b != nil {
		t.Fatalf("无 usage/timings 应返回 nil，实际 %q", string(b))
	}
	t.Log("✓ 无计量流式响应 → nil（不误记）")
}

func TestSSEUsageTeeKeepsTailWindow(t *testing.T) {
	// 前面塞远超尾窗的内容，末块计量仍在
	var sb strings.Builder
	for i := 0; i < 3000; i++ {
		sb.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"填充\"}}]}\n\n")
	}
	sb.WriteString("data: {\"usage\":{\"prompt_tokens\":100,\"prompt_tokens_details\":{\"cached_tokens\":90}}}\n\n")
	tee := newSSEUsageTee(fakeRC{strings.NewReader(sb.String())})
	_, _ = io.ReadAll(tee)
	got := string(tee.UsageJSON())
	if !strings.Contains(got, "\"cached_tokens\":90") {
		t.Fatalf("尾窗应保住末块计量，实际: %q", got)
	}
	t.Log("✓ 大流式响应下尾窗截断仍能取到末块计量")
}
