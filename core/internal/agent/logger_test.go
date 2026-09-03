package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLogger_WriteAll 4 类日志写入验证
func TestLogger_WriteAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs", "task_test")
	l, err := NewLogger("task_test", dir)
	if err != nil {
		t.Fatalf("NewLogger 失败: %v", err)
	}
	defer l.Close()

	l.LogEvent(string(EventToolCall), "info", "execute", "bash", "测试", map[string]any{"cmd": "ls"}, "成功", "", "0.1s")
	l.LogError("tool_error", "命令超时", "bash ls -la（30s 超时）")
	l.LogAudit("write", map[string]any{"path": "/tmp/x"}, "写配置文件", "allow", "成功", "", "agent-1")
	l.LogTranscript("user", "修复 bug", "", "", nil, "", "", "")
	l.LogTranscript("assistant", "调用工具", "bash", "call_1", map[string]any{"cmd": "go build"}, "成功", "", "1.2s")

	// 验证 4 文件存在 + 内容非空
	for _, name := range []string{"events.jsonl", "errors.jsonl", "audit.jsonl", "transcript.jsonl"} {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s 读取失败: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s 为空", name)
		}
		// 每行都是合法 JSON
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if !strings.HasPrefix(line, "{") {
				t.Fatalf("%s 非法 JSON 行: %s", name, line)
			}
		}
	}
}

// TestLogger_Close 关闭后不再写
func TestLogger_Close(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs", "task_close")
	l, err := NewLogger("task_close", dir)
	if err != nil {
		t.Fatalf("NewLogger 失败: %v", err)
	}
	l.Close()
	// 关闭后再写不 panic（writeLine 检查 closed）
	l.LogEvent(string(EventLoopEnd), "info", "end", "", "", nil, "", "", "0s")
}
