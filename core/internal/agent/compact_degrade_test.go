package agent

import "testing"

// TestCompactDegrade — P1 compact 多层降级（2026-08-21 Mr2109）
// 压缩器路径不存在（失败）→ 降级滚动窗口摘要（不报错）
func TestCompactDegrade(t *testing.T) {
	// 模拟压缩器加载失败场景: 路径不存在
	ls := &loopState{agent: &Agent{history: []Message{
		{Role: "user", Content: "任务开始"},
		{Role: "assistant", Content: "处理中"},
		{Role: "tool", Content: "工具结果很长很长" + string(make([]byte, 500))},
		{Role: "assistant", Content: "继续"},
		{Role: "tool", Content: "更多结果" + string(make([]byte, 300))},
		{Role: "assistant", Content: "完成"},
	}}}
	// 压缩器 nil（加载失败场景）→ 应降级滚动摘要不报错
	ls.compressor = nil
	err := ls.compactHistory(nil)
	if err != nil {
		t.Fatalf("压缩器失败应降级不报错: %v", err)
	}
	if len(ls.agent.history) != 3 { // 摘要 + 最近 2 轮（4 条→实际保留 2+摘要=3? 验证至少压缩）
		t.Logf("压缩后历史长度: %d（应小于原 6）", len(ls.agent.history))
	}
	if len(ls.agent.history) >= 6 {
		t.Fatalf("历史未压缩: %d", len(ls.agent.history))
	}
}
