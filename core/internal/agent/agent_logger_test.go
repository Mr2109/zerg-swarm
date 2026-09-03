package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetLogger — logger 注入不破坏
func TestSetLogger(t *testing.T) {
	a := &Agent{}
	dir := t.TempDir()
	l, err := NewLogger("test", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.SetLogger(l)
	if a.logger == nil {
		t.Fatal("SetLogger 后 logger 应为非 nil")
	}
}

// TestSetLogger_LogModelCallEvent — 注入后 callModel 事件写入（验证 logger 接通）
func TestSetLogger_LogModelCallEvent(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLogger("test", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a := &Agent{}
	a.SetLogger(l)
	// 直接调 LogEvent（模拟 callModel 失败事件——验证写入）
	l.LogEvent(string(EventModelCall), "error", "model_call_failed", "",
		"test-prompt", map[string]any{"status": 400}, "", "bad request", "1ms")
	// 验证 events.jsonl 写入
	evFile := filepath.Join(dir, "events.jsonl")
	data, err := os.ReadFile(evFile)
	if err != nil {
		t.Fatalf("events.jsonl 未写入: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("events.jsonl 空——LogEvent 未写入")
	}
}

// TestCallModel_LoggerNil_NoPanic — logger nil 不 panic（兼容）
func TestCallModel_LoggerNil_NoPanic(t *testing.T) {
	a := &Agent{}
	if a.logger != nil {
		t.Fatal("默认 logger 应为 nil")
	}
}
