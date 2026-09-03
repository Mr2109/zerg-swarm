package agent

import (
	"testing"
)

// 构造 N 轮 history（每轮 assistant + tool）
func buildHistory(n int) []Message {
	var h []Message
	for i := 0; i < n; i++ {
		h = append(h, Message{Role: "assistant", Content: "决策"})
		h = append(h, Message{Role: "tool", ToolCallID: "t1", Content: "结果"})
	}
	return h
}

// TestRollingWindow_Compress — 超过窗口压最早——窗口稳定
func TestRollingWindow_Compress(t *testing.T) {
	a := NewAgent(Config{})
	a.history = buildHistory(8) // 8 轮——窗口 5——应压 3 轮
	// 连续压缩（每轮一次）
	a.compressOldest()
	a.compressOldest()
	a.compressOldest()
	// 验证：history 剩 5 轮（8-3）
	rounds := 0
	for _, m := range a.history {
		if m.Role == "assistant" {
			rounds++
		}
	}
	if rounds != 5 {
		t.Errorf("压缩后应剩 5 轮——实际 %d", rounds)
	}
	// 验证摘要累积
	if a.rollingSummary.compacted != 3 {
		t.Errorf("应压缩 3 轮——实际 %d", a.rollingSummary.compacted)
	}
}

// TestRollingWindow_NoCompressUnderWindow — 窗口内不压缩
func TestRollingWindow_NoCompressUnderWindow(t *testing.T) {
	a := NewAgent(Config{})
	a.history = buildHistory(3) // 3 轮 < 5——不压缩
	if a.compressOldest() {
		t.Error("窗口内不应压缩")
	}
	if a.rollingSummary.compacted != 0 {
		t.Error("窗口内摘要应为空")
	}
}

// TestRollingWindow_SummaryBlock — 摘要区构造
func TestRollingWindow_SummaryBlock(t *testing.T) {
	a := NewAgent(Config{})
	a.history = buildHistory(6)
	a.compressOldest() // 压 1 轮
	sum := a.buildSummaryBlock()
	if sum == nil {
		t.Fatal("摘要区应为非 nil")
	}
	if sum.Role != "system" {
		t.Errorf("摘要区 role 应为 system——实际 %s", sum.Role)
	}
	// 摘要含压缩标记
	if len(sum.Content) < 10 {
		t.Errorf("摘要内容太短: %q", sum.Content)
	}
}

// TestRollingWindow_MaybeCompact — maybeCompact 整合（loop 调用）
func TestRollingWindow_MaybeCompact(t *testing.T) {
	a := NewAgent(Config{})
	a.history = buildHistory(6) // 6 > 5——应压 1
	a.maybeCompact()
	rounds := 0
	for _, m := range a.history {
		if m.Role == "assistant" {
			rounds++
		}
	}
	if rounds != 5 {
		t.Errorf("maybeCompact 后应 5 轮——实际 %d", rounds)
	}
}
