package agent

import (
	"os"
	"testing"
)

// TestSessionLogPersist — P1 会话日志不变量（2026-08-21 Mr2109）
// 消息写日志 → 从日志恢复（日志=上下文真相）
func TestSessionLogPersist(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 写 3 条消息
	msgs := []Message{
		{Role: "user", Content: "任务: 调研 X"},
		{Role: "assistant", Content: "我读文件"},
		{Role: "tool", Content: "文件内容...", ToolCallID: "t1"},
	}
	for _, m := range msgs {
		if err := sl.Append(m); err != nil {
			t.Fatal(err)
		}
	}
	sl.Close()

	// 重新打开（模拟崩溃恢复）——从日志恢复
	sl2, _ := NewSessionLog(dir)
	defer sl2.Close()
	restored, err := sl2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 3 {
		t.Fatalf("应恢复 3 条: got %d", len(restored))
	}
	if restored[0].Content != "任务: 调研 X" {
		t.Fatalf("第一条内容不符: %q", restored[0].Content)
	}
	if restored[2].ToolCallID != "t1" {
		t.Fatalf("工具消息 ID 不符: %q", restored[2].ToolCallID)
	}
	_ = os.RemoveAll(dir)
}
