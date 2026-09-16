package agent

import (
	"strings"
	"testing"
)

// 事故背景（2026-09-17 实测）：教学示例里写着占位词「工具名」，模型照抄成工具名 ⇒ 调用必败 ⇒ 反复空转。
// 本用例守住"占位词必被识别""可用工具名必能列出"两条，教学式错误才有信息量。
func TestIsPlaceholderToolName(t *testing.T) {
	for _, bad := range []string{"工具名", "参数名", "参数值", "tool_name", "name", "工具", " 工具名 "} {
		if !isPlaceholderToolName(bad) {
			t.Errorf("占位词未被识别：%q", bad)
		}
	}
	for _, good := range []string{"read", "grep", "bash", "file_count", "tool_search"} {
		if isPlaceholderToolName(good) {
			t.Errorf("真实工具名被误判为占位词：%q", good)
		}
	}
}

func TestAvailableToolNamesNonEmpty(t *testing.T) {
	got := availableToolNames()
	if strings.TrimSpace(got) == "" {
		t.Fatal("可用工具名为空 ⇒ 教学式错误没有信息量")
	}
	if !strings.Contains(got, "read") {
		t.Errorf("可用工具名里应含 read，实际：%s", got)
	}
}
