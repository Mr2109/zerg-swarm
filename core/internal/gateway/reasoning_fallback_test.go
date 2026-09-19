package gateway

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"
)

// TestApplyReasoningFallback_ContentEmpty — content 空 + reasoning 有 → 兜底
func TestApplyReasoningFallback_ContentEmpty(t *testing.T) {
	g := &Gateway{}
	body := `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"思考过程"}}]}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(body)))}
	out := g.applyReasoningFallback(resp)
	if out == nil {
		t.Fatal("resp 不应为 nil")
	}
	newBody, _ := io.ReadAll(out.Body)
	if !bytes.Contains(newBody, []byte(`"content":"思考过程"`)) {
		t.Errorf("content 应用 reasoning 兜底——实际 %s", newBody)
	}
}

// TestApplyReasoningFallback_ToolCallsNotFilled — 2026-09-19 ⑥ 工具轮不做兜底：
// content 空 + reasoning 有 + 带 tool_calls ⇒ content 必须保持空、tool_calls 原样保留。
// （反面证据：Hermes 子代理实测 content 与 reasoning_content 逐字节相同 ⇒ 它把内心话当正文 ⇒ 判"被截断"。）
func TestApplyReasoningFallback_ToolCallsNotFilled(t *testing.T) {
	g := &Gateway{}
	body := `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"内心话","tool_calls":[{"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"date\"}"}}]},"finish_reason":"tool_calls"}]}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(body)))}
	out := g.applyReasoningFallback(resp)
	if out == nil {
		t.Fatal("resp 不应为 nil")
	}
	newBody, _ := io.ReadAll(out.Body)
	if !bytes.Contains(newBody, []byte(`"content":""`)) {
		t.Errorf("工具轮的 content 应保持空——实际 %s", newBody)
	}
	if bytes.Contains(newBody, []byte(`"content":"内心话"`)) {
		t.Error("工具轮不得把 reasoning_content 搬进 content（思考不得顶替正文）")
	}
	if !bytes.Contains(newBody, []byte(`"tool_calls"`)) {
		t.Error("tool_calls 必须原样保留")
	}
	if !bytes.Contains(newBody, []byte(`"reasoning_content":"内心话"`)) {
		t.Error("reasoning_content 必须原样保留")
	}
}

// TestApplyReasoningFallback_ContentOK — content 有 → 不改
func TestApplyReasoningFallback_ContentOK(t *testing.T) {
	g := &Gateway{}
	body := `{"choices":[{"message":{"role":"assistant","content":"正常回答","reasoning_content":"思考"}}]}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(body)))}
	out := g.applyReasoningFallback(resp)
	newBody, _ := io.ReadAll(out.Body)
	if !bytes.Contains(newBody, []byte(`"content":"正常回答"`)) {
		t.Errorf("content 有——不应改——实际 %s", newBody)
	}
	if bytes.Contains(newBody, []byte(`"content":"思考"`)) {
		t.Error("content 有——不应被 reasoning 覆盖")
	}
}

// TestApplyReasoningFallback_NoReasoning — content 空 + reasoning 空 → 不改
func TestApplyReasoningFallback_NoReasoning(t *testing.T) {
	g := &Gateway{}
	body := `{"choices":[{"message":{"role":"assistant","content":""}}]}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(body)))}
	out := g.applyReasoningFallback(resp)
	newBody, _ := io.ReadAll(out.Body)
	if !bytes.Contains(newBody, []byte(`"content":""`)) {
		t.Errorf("都空——不应改——实际 %s", newBody)
	}
}

// TestApplyReasoningFallback_SyncsContentLength — 2026-09-14 修复回归：
// 兜底改写 body 后必须同步 ContentLength 与 Content-Length 头，
// 否则转发层按旧长度抄写 ⇒ "wrote more than the declared Content-Length" ⇒ 客户端空响应。
func TestApplyReasoningFallback_SyncsContentLength(t *testing.T) {
	g := &Gateway{}
	body := `{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"思考过程"}}]}`
	resp := &http.Response{
		Body:          io.NopCloser(bytes.NewReader([]byte(body))),
		ContentLength: int64(len(body)), // 模拟上游给的旧长度
		Header:        http.Header{"Content-Length": []string{strconv.Itoa(len(body))}},
	}
	out := g.applyReasoningFallback(resp)
	newBody, _ := io.ReadAll(out.Body)
	if out.ContentLength != int64(len(newBody)) {
		t.Fatalf("ContentLength 必须等于新 body 长度：got %d want %d", out.ContentLength, len(newBody))
	}
	if h := out.Header.Get("Content-Length"); h != strconv.Itoa(len(newBody)) {
		t.Fatalf("Content-Length 头必须同步：got %q want %q", h, strconv.Itoa(len(newBody)))
	}
}

// TestApplyReasoningFallback_NoChangePlainBody — 非 JSON 原样返回时长度也要对
func TestApplyReasoningFallback_NoChangePlainBody(t *testing.T) {
	g := &Gateway{}
	body := "not-json-body"
	resp := &http.Response{
		Body:          io.NopCloser(bytes.NewReader([]byte(body))),
		ContentLength: 999, // 故意给错
		Header:        http.Header{},
	}
	out := g.applyReasoningFallback(resp)
	newBody, _ := io.ReadAll(out.Body)
	if len(newBody) != len(body) || out.ContentLength != int64(len(body)) {
		t.Fatalf("非 JSON 路径长度必须同步：len(body)=%d ContentLength=%d", len(newBody), out.ContentLength)
	}
}
