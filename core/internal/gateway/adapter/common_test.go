package adapter

// common_test.go — 公共函数测试（2026-08-29 q5 覆盖补齐）
// 目标: ContainsTools/ExtractToolNames/IsStreamRequest/ExtractAuthToken/JsonSetField/SSE改写（0% 包首测）

import (
	"net/http"
	"strings"
	"testing"
)

// TestContainsTools 工具调用检测
func TestContainsTools(t *testing.T) {
	if !ContainsTools([]byte(`{"tools":[{"type":"function"}]}`)) {
		t.Fatal("含 tools 应 true")
	}
	if ContainsTools([]byte(`{"messages":[]}`)) {
		t.Fatal("无 tools 应 false")
	}
}

// TestExtractToolNames 工具名提取（function.name + name 两种格式）
func TestExtractToolNames(t *testing.T) {
	body := []byte(`{"tools":[
		{"type":"function","function":{"name":"search"}},
		{"type":"function","name":"read_file"}
	]}`)
	names := ExtractToolNames(body)
	if len(names) != 2 || names[0] != "search" || names[1] != "read_file" {
		t.Fatalf("提取工具名失败: %v", names)
	}
	// 非法 JSON → nil
	if ExtractToolNames([]byte(`{bad`)) != nil {
		t.Fatal("非法 JSON 应 nil")
	}
}

// TestIsStreamRequest 流式判断
func TestIsStreamRequest(t *testing.T) {
	if !IsStreamRequest([]byte(`{"stream":true}`)) {
		t.Fatal("stream:true 应 true")
	}
	if IsStreamRequest([]byte(`{"stream":false}`)) {
		t.Fatal("stream:false 应 false")
	}
	if IsStreamRequest([]byte(`{bad`)) {
		t.Fatal("非法 JSON 应 false")
	}
}

// TestExtractAuthToken 三种认证方式
func TestExtractAuthToken(t *testing.T) {
	h := http.Header{}
	h.Set("X-Auth-Token", "token1")
	if ExtractAuthToken(h) != "token1" {
		t.Fatal("X-Auth-Token 提取失败")
	}
	h = http.Header{}
	h.Set("Authorization", "Bearer token2")
	if ExtractAuthToken(h) != "token2" {
		t.Fatal("Bearer 提取失败")
	}
	h = http.Header{}
	h.Set("x-api-key", "token3")
	if ExtractAuthToken(h) != "token3" {
		t.Fatal("x-api-key 提取失败")
	}
	h = http.Header{}
	if ExtractAuthToken(h) != "" {
		t.Fatal("空头应返回空")
	}
}

// TestJsonSetField 字段设置
func TestJsonSetField(t *testing.T) {
	body := []byte(`{"model":"old","stream":false}`)
	newBody := JsonSetField(body, "model", "new-model")
	if !strings.Contains(string(newBody), `"model":"new-model"`) {
		t.Fatalf("字段未设置: %s", newBody)
	}
	if strings.Contains(string(newBody), "old") {
		t.Fatal("旧值未替换")
	}
	// 非法 JSON → 原样返回
	if string(JsonSetField([]byte(`{bad`), "k", "v")) != "{bad" {
		t.Fatal("非法 JSON 应原样")
	}
}

// TestRewriteReasoningToContent reasoning_content → content
func TestRewriteReasoningToContent(t *testing.T) {
	// data 行 → 改写
	out := RewriteReasoningToContent([]byte(`data: {"reasoning_content":"thinking..."}`))
	if out == nil || !strings.Contains(string(out), `"content"`) || strings.Contains(string(out), "reasoning_content") {
		t.Fatalf("改写失败: %v", out)
	}
	// 无 reasoning_content → nil
	if RewriteReasoningToContent([]byte(`data: {"content":"ok"}`)) != nil {
		t.Fatal("无 reasoning 应 nil")
	}
	// 非 data 行 → nil
	if RewriteReasoningToContent([]byte(`event: reasoning_content`)) != nil {
		t.Fatal("非 data 行应 nil")
	}
}

// TestRewriteReasoningDeltaToOutput reasoning delta → output delta
func TestRewriteReasoningDeltaToOutput(t *testing.T) {
	out := RewriteReasoningDeltaToOutput([]byte(`data: {"type":"response.reasoning_text.delta","delta":"hi"}`))
	if !strings.Contains(string(out), "response.output_text.delta") {
		t.Fatalf("改写失败: %s", out)
	}
	if strings.Contains(string(out), "reasoning_text.delta") {
		t.Fatal("reasoning 应被替换")
	}
}

// TestRewriteReasoningItemToMessage reasoning item → message item
func TestRewriteReasoningItemToMessage(t *testing.T) {
	// 含 role → 只改 type
	out := RewriteReasoningItemToMessage([]byte(`data: {"type":"reasoning","role":"assistant","summary":[]}`))
	if !strings.Contains(string(out), `"type":"message"`) {
		t.Fatalf("type 未改: %s", out)
	}
	if strings.Contains(string(out), `"type":"reasoning"`) {
		t.Fatal("reasoning 应被替换")
	}
	// 无 role → 补 role
	out2 := RewriteReasoningItemToMessage([]byte(`data: {"type":"reasoning","summary":[]}`))
	if !strings.Contains(string(out2), `"role":"assistant"`) {
		t.Fatalf("role 未补: %s", out2)
	}
	// 无 reasoning → 原样
	if string(RewriteReasoningItemToMessage([]byte(`data: {"type":"message"}`))) != `data: {"type":"message"}` {
		t.Fatal("无 reasoning 应原样")
	}
}

// TestUUIDHex 32 位 hex
func TestUUIDHex(t *testing.T) {
	id := UUIDHex()
	if len(id) != 32 {
		t.Fatalf("UUID 长度 = %d，应 32", len(id))
	}
	if id == "00000000000000000000000000000000" {
		t.Fatal("UUID 不应全零")
	}
}
