package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// ─── parseModelResponse 单元测试 ──────────────────────────────

// TestParseModelResponse_MessageType — 解析 message 类型（output_text）
func TestParseModelResponse_MessageType(t *testing.T) {
	raw := `{
		"output": [{
			"type": "message",
			"content": [
				{"type": "output_text", "text": "Hello, I can help with that."}
			]
		}],
		"usage": {"total_tokens": 42}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello, I can help with that." {
		t.Errorf("expected content 'Hello, I can help with that.', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 42 {
		t.Errorf("expected 42 tokens, got %d", resp.Usage.TotalTokens)
	}
}

// TestParseModelResponse_MultipleContentParts — 多段 content（text + image）
func TestParseModelResponse_MultipleContentParts(t *testing.T) {
	raw := `{
		"output": [{
			"type": "message",
			"content": [
				{"type": "output_text", "text": "Part 1: "},
				{"type": "text", "text": "Part 2"},
				{"type": "image_url", "image_url": "http://example.com/img.png"}
			]
		}]
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Part 1: Part 2" {
		t.Errorf("expected 'Part 1: Part 2', got %q", resp.Content)
	}
}

// TestParseModelResponse_ReasoningType — 解析 reasoning 类型（reasoning_text）
func TestParseModelResponse_ReasoningType(t *testing.T) {
	raw := `{
		"output": [{
			"type": "reasoning",
			"content": [
				{"type": "reasoning_text", "text": "Let me think about this..."}
			]
		}],
		"usage": {"total_tokens": 100}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reasoning != "Let me think about this..." {
		t.Errorf("expected reasoning 'Let me think about this...', got %q", resp.Reasoning)
	}
}

// TestParseModelResponse_FunctionCall — 解析 function_call 类型
func TestParseModelResponse_FunctionCall(t *testing.T) {
	raw := `{
		"output": [{
			"type": "function_call",
			"name": "read",
			"arguments": "{\"path\": \"main.go\", \"offset\": 1}",
			"call_id": "call_abc123"
		}],
		"usage": {"total_tokens": 50}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "read" {
		t.Errorf("expected tool name 'read', got %q", tc.Name)
	}
	if tc.ID != "call_abc123" {
		t.Errorf("expected call ID 'call_abc123', got %q", tc.ID)
	}
	if tc.RawArgs != `{"path": "main.go", "offset": 1}` {
		t.Errorf("unexpected RawArgs: %q", tc.RawArgs)
	}
	// 验证 Args 被解析为 map
	path, ok := tc.Args["path"].(string)
	if !ok || path != "main.go" {
		t.Errorf("expected Args[path]=main.go, got %v", tc.Args["path"])
	}
}

// TestParseModelResponse_MixedOutputs — 混合输出（message + reasoning + function_call）
func TestParseModelResponse_MixedOutputs(t *testing.T) {
	raw := `{
		"output": [
			{"type": "reasoning", "content": [{"type": "reasoning_text", "text": "Hmm..."}]},
			{"type": "message", "content": [{"type": "output_text", "text": "Done!"}]},
			{"type": "function_call", "name": "bash", "arguments": "{}", "call_id": "call_2"}
		],
		"usage": {"total_tokens": 99}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Done!" {
		t.Errorf("expected content 'Done!', got %q", resp.Content)
	}
	if resp.Reasoning != "Hmm..." {
		t.Errorf("expected reasoning 'Hmm...', got %q", resp.Reasoning)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("expected tool name 'bash', got %q", resp.ToolCalls[0].Name)
	}
}

// TestParseModelResponse_EmptyOutput — 空 output 数组
func TestParseModelResponse_EmptyOutput(t *testing.T) {
	raw := `{"output": [], "usage": {"total_tokens": 0}}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 0 {
		t.Errorf("expected 0 tokens, got %d", resp.Usage.TotalTokens)
	}
}

// TestParseModelResponse_MalformedJSON — 无效 JSON
func TestParseModelResponse_MalformedJSON(t *testing.T) {
	_, err := parseModelResponse([]byte(`not valid json`))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// TestParseModelResponse_ReasoningOnly — content 为空、只有思考时：**只标记、不搬运**（缺陷 A 的契约）
//
// 历史（2026-09-17）：旧行为是 `if Content == "" && Reasoning != "" { Content = Reasoning }`，
// 结果思考被当成正文回灌进历史 ⇒ 下一轮模型照抄该口吻 ⇒ 自我延续（缺陷 A）。已废。
// 新契约：Content 保持为空 + ReasoningOnly=true（思考照跑、不算正文、不入正文）。
func TestParseModelResponse_ReasoningOnly(t *testing.T) {
	raw := `{
		"output": [
			{"type": "reasoning", "content": [{"type": "reasoning_text", "text": "Thinking..."}]}
		],
		"usage": {"total_tokens": 10}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	// 契约：只标记、不搬运
	if resp.Content != "" {
		t.Errorf("思考不得被搬进正文（缺陷 A）：Content=%q", resp.Content)
	}
	if resp.Reasoning != "Thinking..." {
		t.Errorf("思考应保留在思考通道：Reasoning=%q", resp.Reasoning)
	}
	if !resp.ReasoningOnly {
		t.Error("应标记 ReasoningOnly=true（供上层决定回灌策略）")
	}
}

// TestParseModelResponse_UnknownType — 未知类型跳过
func TestParseModelResponse_UnknownType(t *testing.T) {
	raw := `{
		"output": [
			{"type": "thinking", "content": [{"type": "unknown", "text": "skip me"}]}
		]
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("unknown type should be skipped, got content %q", resp.Content)
	}
}

// TestParseModelResponse_BadFunctionCallArgs — function_call 参数解析失败
func TestParseModelResponse_BadFunctionCallArgs(t *testing.T) {
	raw := `{
		"output": [{
			"type": "function_call",
			"name": "bash",
			"arguments": "not valid json",
			"call_id": "call_x"
		}]
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	// 参数解析失败不应导致 panic，RawArgs 保留原文
	if resp.ToolCalls[0].RawArgs != "not valid json" {
		t.Errorf("RawArgs should preserve original: %q", resp.ToolCalls[0].RawArgs)
	}
}

// TestParseModelResponse_NoUsage — 无 usage 字段
func TestParseModelResponse_NoUsage(t *testing.T) {
	raw := `{"output": [{"type": "message", "content": [{"type": "output_text", "text": "hi"}]}]}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi" {
		t.Errorf("expected 'hi', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 0 {
		t.Errorf("expected 0 tokens when no usage, got %d", resp.Usage.TotalTokens)
	}
}

// TestParseModelResponse_RealWorldResponse — 真实世界响应（thinking model）
func TestParseModelResponse_RealWorldResponse(t *testing.T) {
	raw := `{
		"output": [
			{"type": "reasoning", "content": [{"type": "reasoning_text", "text": "The user wants to fix a bug. Let me check the file."}]},
			{"type": "message", "content": [{"type": "output_text", "text": "I'll read the file to find the bug."}]},
			{"type": "function_call", "name": "read", "arguments": "{\"path\": \"bug.go\"}", "call_id": "call_real"}
		],
		"usage": {"total_tokens": 256}
	}`
	resp, err := parseModelResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "I'll read the file to find the bug." {
		t.Errorf("content mismatch: %q", resp.Content)
	}
	if resp.Reasoning != "The user wants to fix a bug. Let me check the file." {
		t.Errorf("reasoning mismatch: %q", resp.Reasoning)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "read" {
		t.Errorf("expected read tool call, got %+v", resp.ToolCalls)
	}
	if resp.Usage.TotalTokens != 256 {
		t.Errorf("expected 256 tokens, got %d", resp.Usage.TotalTokens)
	}
}

// ─── toolNames 单元测试 ──────────────────────────────────────

func TestToolNames(t *testing.T) {
	tools := []ToolDef{
		{Function: FunctionDef{Name: "read"}},
		{Function: FunctionDef{Name: "grep"}},
		{Function: FunctionDef{Name: "bash"}},
	}
	got := toolNames(tools)
	if len(got) != 3 {
		t.Fatalf("expected 3 names, got %d", len(got))
	}
	expected := []string{"read", "grep", "bash"}
	for i, e := range expected {
		if got[i] != e {
			t.Errorf("got[%d]=%q, want %q", i, got[i], e)
		}
	}
}

func TestToolNames_Empty(t *testing.T) {
	got := toolNames(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ─── truncate 单元测试 ──────────────────────────────────────

func TestTruncate_Short(t *testing.T) {
	got := truncate("hello", 100)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := truncate(long, 100)
	if !strings.Contains(got, "...(output truncated — 300 chars total)") {
		t.Errorf("expected truncation suffix, got %q", got)
	}
	// truncated output = prefix + suffix (suffix itself ~38 chars) — relaxed to limit+60
	if len(got) > 100+60 {
		t.Errorf("truncated output too long: %d chars", len(got))
	}
}

func TestTruncate_ExactlyLimit(t *testing.T) {
	s := strings.Repeat("x", 50)
	got := truncate(s, 50)
	if got != s {
		t.Errorf("exact limit should not truncate, got %q", got)
	}
}

// ─── 辅助：构造真实 JSON ────────────────────────────────────

func TestParseModelResponse_ViaJSON(t *testing.T) {
	// 验证序列化/反序列化一致性
	raw := map[string]any{
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "output_text", "text": "test"},
				},
			},
		},
		"usage": map[string]any{"total_tokens": float64(10)},
	}
	data, _ := json.Marshal(raw)
	resp, err := parseModelResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "test" {
		t.Errorf("expected 'test', got %q", resp.Content)
	}
}
