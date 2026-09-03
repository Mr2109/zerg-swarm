package gateway

import (
	"bytes"
	"io"
	"net/http"
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
