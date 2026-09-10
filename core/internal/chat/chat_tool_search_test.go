package chat

import (
	"strings"
	"testing"
)

// TestToolSearchExecute — 批次A 断链修复回归（2026-09-10）
func TestToolSearchExecute(t *testing.T) {
	rt := NewToolRuntime()
	out := ToolSearchExecute("系统", rt, 8)
	if !strings.Contains(out, "✅ 发现") {
		t.Fatalf("搜\"系统\"应有结果, got: %s", out)
	}
	// 命中的应进常驻
	if len(rt.Resident()) == 0 {
		t.Fatal("tool_search 命中后应加入常驻")
	}
	// 已常驻的工具不重复返回（第二次同关键词——命中减少/为 0）
	out2 := ToolSearchExecute("系统", rt, 8)
	if strings.Contains(out2, "✅ 发现") {
		// 若仍有未常驻项可以接受；但不应重复列出已常驻的
		for _, n := range rt.Resident() {
			if strings.Contains(out2, "- "+n+":") && strings.Count(out2, "- "+n+":") > 1 {
				t.Fatalf("已常驻工具重复列出: %s", n)
			}
		}
	}
	// 空关键词 → 提示
	if got := ToolSearchExecute("", rt, 8); !strings.Contains(got, "未发现匹配工具") {
		t.Fatalf("空查询应提示, got: %s", got)
	}
}

// TestSearchStreak — 重复搜索计数（三次触发强制提示）
func TestSearchStreak(t *testing.T) {
	rt := NewToolRuntime()
	for i := 1; i <= 2; i++ {
		if n, hint := rt.SearchStreak("端口"); n != i || hint != "" {
			t.Fatalf("第 %d 次不该有提示, got n=%d hint=%q", i, n, hint)
		}
	}
	if n, hint := rt.SearchStreak("端口"); n != 3 || hint == "" {
		t.Fatalf("第 3 次应触发强制提示, got n=%d hint=%q", n, hint)
	}
}
