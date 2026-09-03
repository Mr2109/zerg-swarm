package gateway

import (
	"testing"
)

// ─── detectAction 单元测试 ──────────────────────────────────────

func TestDetectAction_NilBody(t *testing.T) {
	result := detectAction(nil, "/api/v1/completions")
	if result != ActionDefault {
		t.Errorf("nil body → expected ActionDefault, got %v", result)
	}
}

func TestDetectAction_EmptyBody(t *testing.T) {
	result := detectAction([]byte{}, "/api/v1/completions")
	if result != ActionDefault {
		t.Errorf("empty body → expected ActionDefault, got %v", result)
	}
}

func TestDetectAction_ToolCall(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function"}]}`)
	result := detectAction(body, "/api/v1/chat/completions")
	if result != ActionToolCall {
		t.Errorf("body with tools → expected ActionToolCall, got %v", result)
	}
}

func TestDetectAction_Coding(t *testing.T) {
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"fix this bug"}]}`)
	result := detectAction(body, "/api/v1/chat/completions")
	if result != ActionCoding {
		t.Errorf("body with 'fix' keyword → expected ActionCoding, got %v", result)
	}
}

func TestDetectAction_Writing(t *testing.T) {
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"写一个报告"}]}`)
	result := detectAction(body, "/api/v1/chat/completions")
	if result != ActionWriting {
		t.Errorf("body with '写' keyword → expected ActionWriting, got %v", result)
	}
}

func TestDetectAction_Research(t *testing.T) {
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"研究一下这个技术"}]}`)
	result := detectAction(body, "/api/v1/chat/completions")
	if result != ActionResearch {
		t.Errorf("body with '研究' keyword → expected ActionResearch, got %v", result)
	}
}

func TestDetectAction_PathPriority(t *testing.T) {
	// path 匹配优先于 body 关键词
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"fix something"}]}`)
	result := detectAction(body, "/api/v1/research")
	if result != ActionResearch {
		t.Errorf("path /research → expected ActionResearch, got %v", result)
	}
}

// ─── detectAction_String 单元测试 ───────────────────────────────

func TestActionType_String(t *testing.T) {
	tests := []struct {
		action ActionType
		expect string
	}{
		{ActionDefault, "默认"},
		{ActionCoding, "编码"},
		{ActionWriting, "写作"},
		{ActionResearch, "研究"},
		{ActionToolCall, "工具调用"},
		{ActionType(99), "默认"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expect {
			t.Errorf("%v.String() = %q, want %q", tt.action, got, tt.expect)
		}
	}
}

// ─── hasToolCalls 单元测试 ──────────────────────────────────────

func TestHasToolCalls_Present(t *testing.T) {
	m := map[string]interface{}{
		"tool_calls": []interface{}{},
	}
	if !hasToolCalls(m) {
		t.Error("expected hasToolCalls to return true when tool_calls key exists")
	}
}

func TestHasToolCalls_Absent(t *testing.T) {
	m := map[string]interface{}{
		"content": "hello",
	}
	if hasToolCalls(m) {
		t.Error("expected hasToolCalls to return false when no tool_calls key")
	}
}

func TestHasToolCalls_NilMap(t *testing.T) {
	if hasToolCalls(nil) {
		t.Error("nil map should return false")
	}
}

// ─── matchKeywords 单元测试 ─────────────────────────────────────

func TestMatchKeywords_Match(t *testing.T) {
	if !matchKeywords("fix this bug", []string{"fix", "bug"}) {
		t.Error("should match 'fix' keyword")
	}
}

func TestMatchKeywords_NoMatch(t *testing.T) {
	if matchKeywords("hello world", []string{"xyz"}) {
		t.Error("should not match")
	}
}

func TestMatchKeywords_EmptyText(t *testing.T) {
	if matchKeywords("", []string{"hello"}) {
		t.Error("empty text should not match")
	}
}

func TestMatchKeywords_EmptyKeywords(t *testing.T) {
	if matchKeywords("hello world", nil) {
		t.Error("empty keywords should not match")
	}
}

func TestMatchKeywords_CaseInsensitive(t *testing.T) {
	if !matchKeywords("HELLO FIX BUG", []string{"fix"}) {
		t.Error("should be case insensitive")
	}
}

// ─── extractContentString 单元测试 ──────────────────────────────

func TestExtractContentString_String(t *testing.T) {
	result := extractContentString("hello world")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestExtractContentString_Slice(t *testing.T) {
	v := []interface{}{
		map[string]interface{}{
			"text": "part1",
		},
		map[string]interface{}{
			"text": "part2",
		},
	}
	result := extractContentString(v)
	if result != "part1 part2 " {
		t.Errorf("expected 'part1 part2 ', got %q", result)
	}
}

func TestExtractContentString_Nil(t *testing.T) {
	result := extractContentString(nil)
	if result != "" {
		t.Errorf("nil → expected empty, got %q", result)
	}
}

// ─── extractAllText 单元测试 ────────────────────────────────────

func TestExtractAllText_Messages(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"world"}]}`)
	result := extractAllText(body)
	if result != "hello world " {
		t.Errorf("expected 'hello world ', got %q", result)
	}
}

func TestExtractAllText_Empty(t *testing.T) {
	if extractAllText(nil) != "" {
		t.Error("nil should return empty")
	}
	if extractAllText([]byte{}) != "" {
		t.Error("empty should return empty")
	}
}

func TestExtractAllText_SystemPrompt(t *testing.T) {
	body := []byte(`{"system":"be helpful","messages":[{"role":"user","content":"hi"}]}`)
	result := extractAllText(body)
	if result != "be helpful hi " {
		t.Errorf("expected 'be helpful hi ', got %q", result)
	}
}

// ─── detectAction 边界条件 ──────────────────────────────────────

func TestDetectAction_MalformedJSON(t *testing.T) {
	// 非JSON body 应走关键词匹配
	body := []byte(`not json at all fix bug`)
	result := detectAction(body, "/api/v1/chat")
	// 非JSON时 extractAllText 返回空，但 path 参数不参与关键词匹配
	// 只走 body 关键词，应该匹配到 ActionCoding
	if result != ActionCoding {
		t.Errorf("malformed JSON with 'fix' keyword → expected ActionCoding, got %v", result)
	}
}

func TestDetectAction_PathOverridesBody(t *testing.T) {
	// 当 path 包含已知动作关键词时，path 优先
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"fix this"}]}`)
	result := detectAction(body, "/api/v1/research")
	if result != ActionResearch {
		t.Errorf("path /research should override body keyword 'fix', got %v", result)
	}
}
