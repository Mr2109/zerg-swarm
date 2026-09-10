package chat

import "testing"

// 批次D1 守卫回归（2026-09-10）
func TestSanitizeMessages(t *testing.T) {
	// 孤立 tool 结果 → 丢弃
	msgs := []map[string]any{
		{"role": "user", "content": "hi"},
		{"role": "tool", "content": "孤儿结果"},
		{"role": "assistant", "content": "答"},
	}
	out, fixes := SanitizeMessages(msgs)
	if len(out) != 2 || len(fixes) == 0 {
		t.Fatalf("孤立 tool 应被丢弃: out=%d fixes=%v", len(out), fixes)
	}
	// 相邻同角色 → 合并
	msgs2 := []map[string]any{
		{"role": "user", "content": "a"},
		{"role": "user", "content": "b"},
	}
	out2, fixes2 := SanitizeMessages(msgs2)
	if len(out2) != 1 || out2[0]["content"] != "a\nb" {
		t.Fatalf("相邻 user 应合并: %v fixes=%v", out2, fixes2)
	}
	// assistant tool_calls 后的 tool 保留
	msgs3 := []map[string]any{
		{"role": "user", "content": "q"},
		{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "1"}}},
		{"role": "tool", "content": "结果"},
	}
	out3, _ := SanitizeMessages(msgs3)
	if len(out3) != 3 {
		t.Fatalf("合法 tool 配对不应被改: %d", len(out3))
	}
}
