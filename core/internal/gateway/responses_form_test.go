package gateway

import "testing"

// 丙批 C2 补（2026-09-10）：OpenAI Responses 形态解析
// 来源：未知形态探针在真实流量里抓到 12 条（键名 output/usage.input_tokens_details）
func TestParseResponsesForm(t *testing.T) {
	body := []byte(`{"id":"resp_1","object":"response","status":"completed","model":"example-35b-v2",
		"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],
		"usage":{"input_tokens":1200,"input_tokens_details":{"cached_tokens":1024},
		"output_tokens":8,"total_tokens":1208}}`)
	read, miss, form, _, ok := parsePrefixCacheUsage(body)
	if !ok {
		t.Fatal("Responses 形态应可解析（原先计为未知形态）")
	}
	if form != "openai-responses" || read != 1024 || miss != 176 {
		t.Fatalf("解析结果 form=%s read=%d miss=%d，期望 openai-responses/1024/176", form, read, miss)
	}
	// 未缓存场景（cached=0）
	body2 := []byte(`{"object":"response","output":[],"usage":{"input_tokens":300,"input_tokens_details":{"cached_tokens":0}}}`)
	read2, miss2, _, _, ok2 := parsePrefixCacheUsage(body2)
	if !ok2 || read2 != 0 || miss2 != 300 {
		t.Fatalf("未缓存解析 read=%d miss=%d ok=%v，期望 0/300/true", read2, miss2, ok2)
	}
	// 回归：chat.completions 形态不受影响
	body3 := []byte(`{"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":64}}}`)
	r3, m3, f3, _, ok3 := parsePrefixCacheUsage(body3)
	if !ok3 || f3 != "openai" || r3 != 64 || m3 != 36 {
		t.Fatalf("回归失败 form=%s read=%d miss=%d", f3, r3, m3)
	}
}
