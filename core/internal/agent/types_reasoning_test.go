package agent

import "testing"

// 事故背景（2026-09-17 实测）：解析层曾有「content 空读 reasoning」的 fallback，把思考搬进 Content
// ⇒ 落库后 reasoning 列空 ⇒ 下一轮回灌时思考成了「上一轮助手说的话」⇒ 模型照抄口吻 ⇒ 正文污染 + 复读。
// 铁律（Mr2109 定）：思考不能关，也不能顶替正文——各归各位。
func TestParseDoesNotMoveReasoningIntoContent(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"用户在问我文件用途，我先读文件。"}}],` +
		`"usage":{"total_tokens":3}}`)
	resp, err := parseChatModelResponse(raw)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if resp.Content != "" {
		t.Errorf("正文不得被思考顶替（得到 %q）", resp.Content)
	}
	if resp.Reasoning == "" {
		t.Errorf("思考应留在 Reasoning")
	}
	if !resp.ReasoningOnly {
		t.Errorf("应置 ReasoningOnly 标记（只有思考、无正文）")
	}
}
