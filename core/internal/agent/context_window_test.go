package agent

import (
	"strings"
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

// ── GAP-20261018-02：触发判据双条件（轮数 或 token）────────────────────────────

// buildTokenHistory — 构造 n 轮 history（每轮 assistant+tool），每轮约 tokensPerRound token。
// 估算口径 = bytes/3 ⇒ 每轮字节数 = tokensPerRound×3（ASCII 'x'，无多字节干扰）。
func buildTokenHistory(n, tokensPerRound int) []Message {
	body := strings.Repeat("x", (tokensPerRound/2)*3)
	var h []Message
	for i := 0; i < n; i++ {
		h = append(h, Message{Role: "assistant", Content: "决策:" + body})
		h = append(h, Message{Role: "tool", ToolCallID: "t1", Content: body})
	}
	return h
}

// TestEstimateTextTokensFormula — 纯函数公式（bytes/3 向上取整）
func TestEstimateTextTokensFormula(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},                          // (1+2)/3
		{"ab", 1},                         // 4/3
		{"abc", 1},                        // 5/3
		{"abcd", 2},                       // 6/3
		{strings.Repeat("x", 9000), 3000}, // 9002/3
	}
	for _, c := range cases {
		if got := estimateTextTokens(c.in); got != c.want {
			t.Errorf("estimateTextTokens(len=%d) = %d（期望 %d）", len(c.in), got, c.want)
		}
	}
}

// TestCompactThresholdDefaults — 阈值默认 0.5（可被环境变量覆盖；覆盖值非法则回落默认）
func TestCompactThresholdDefaults(t *testing.T) {
	t.Setenv("ZERG_AGENT_CTX_TOKENS", "")
	t.Setenv("ZERG_AGENT_COMPACT_RATIO", "")
	if got := compactCtxBudgetTokens(); got != 32768 {
		t.Errorf("默认上下文预算应为 32768，实得 %d", got)
	}
	if got := compactTriggerRatio(); got != 0.5 {
		t.Errorf("默认阈值比例应为 0.5，实得 %v", got)
	}
	if got := compactTokenThreshold(); got != 16384 {
		t.Errorf("默认触发线应为 16384（32768×0.5），实得 %d", got)
	}
	// 覆盖生效
	t.Setenv("ZERG_AGENT_CTX_TOKENS", "8192")
	if got := compactTokenThreshold(); got != 4096 {
		t.Errorf("覆盖后触发线应为 4096，实得 %d", got)
	}
	// 非法覆盖 ⇒ 回落默认（既不许永真也不许永假）
	t.Setenv("ZERG_AGENT_COMPACT_RATIO", "2")
	if got := compactTriggerRatio(); got != 0.5 {
		t.Errorf("非法比例应回落 0.5，实得 %v", got)
	}
}

// TestCompressByTokens_TenRounds3KTriggers — 验收①：10 轮 × 3K token，轮数远不到窗口（100），
// token 已过阈值（16384）⇒ 必须压缩；且压完摘要区在、最近轮保留。
func TestCompressByTokens_TenRounds3KTriggers(t *testing.T) {
	t.Setenv("ZERG_AGENT_CTX_TOKENS", "32768")
	t.Setenv("ZERG_AGENT_COMPACT_RATIO", "0.5")

	a := NewAgent(Config{ContextWindow: 100}) // 轮数条件故意放宽 ⇒ 只剩 token 条件可用
	a.history = buildTokenHistory(10, 3000)

	before := a.compactionTokens()
	if got := countRounds(a.history); got != 10 || got > a.cfg.ContextWindow {
		t.Fatalf("夹具有问题：轮数 %d（窗口 %d）—— 本格要的是「轮数远不到窗口」", got, a.cfg.ContextWindow)
	}
	if before <= compactTokenThreshold() {
		t.Fatalf("夹具有问题：%d token 未超阈值 %d", before, compactTokenThreshold())
	}

	if !a.compressOldest() {
		t.Fatalf("%d 轮远不到窗口但 %d token > 阈值 %d ⇒ 必须触发压缩",
			countRounds(a.history), before, compactTokenThreshold())
	}
	after := a.compactionTokens()
	if after >= before {
		t.Fatalf("压缩后 token 应下降：%d → %d", before, after)
	}
	if left := countRounds(a.history); left >= 10 {
		t.Errorf("压缩后轮数应减少（原 10，实 %d）", left)
	}
	// 摘要区在（模型还能参考早期上下文）
	sum := a.buildSummaryBlock()
	if sum == nil || !strings.HasPrefix(sum.Content, summaryPrefix) {
		t.Fatalf("压缩后摘要区应在：%+v", sum)
	}
	// 最近轮保留（最后一条 tool 回执原样还在 ⇒ 能继续对话）
	last := a.history[len(a.history)-1]
	if last.Role != "tool" || len(last.Content) < 1000 {
		t.Errorf("最近轮应完整保留（末条 role=%s len=%d）", last.Role, len(last.Content))
	}
	drop := float64(before-after) / float64(before) * 100
	t.Logf("✓ 10 轮 × 3K：压缩 %d 轮 · token %d → %d（降 %.1f%%）· 剩 %d 轮 · 摘要区 %d 字节",
		a.rollingSummary.compacted, before, after, drop, countRounds(a.history), len(sum.Content))
}

// TestCompressByTokens_TenRounds100NoTrigger — 验收①对照组：10 轮 × 100 token ⇒ 不压
// （token 远低于阈值，轮数也远不到窗口）。
func TestCompressByTokens_TenRounds100NoTrigger(t *testing.T) {
	t.Setenv("ZERG_AGENT_CTX_TOKENS", "32768")
	t.Setenv("ZERG_AGENT_COMPACT_RATIO", "0.5")

	a := NewAgent(Config{ContextWindow: 100})
	a.history = buildTokenHistory(10, 100) // 合计 ≈1000 token

	if got := a.compactionTokens(); got > compactTokenThreshold() {
		t.Fatalf("夹具有问题：%d token 已超阈值", got)
	}
	if a.compressOldest() {
		t.Fatalf("10 轮 × 100 token（合计 %d，阈值 %d）不该压缩", a.compactionTokens(), compactTokenThreshold())
	}
	if a.rollingSummary.compacted != 0 || a.buildSummaryBlock() != nil {
		t.Error("未压缩时摘要区应为空")
	}
	t.Logf("✓ 对照：10 轮 × 100 token（合计 %d < 阈值 %d）⇒ 不压", a.compactionTokens(), compactTokenThreshold())
}

// TestCompressByRoundsStillWorks — 保留旧条件：轮数超窗口仍压（几十条短消息是另一种形态）
func TestCompressByRoundsStillWorks(t *testing.T) {
	t.Setenv("ZERG_AGENT_CTX_TOKENS", "32768")
	a := NewAgent(Config{ContextWindow: 5})
	a.history = buildTokenHistory(30, 10) // token 很小，只有轮数条件成立
	if a.compactionTokens() > compactTokenThreshold() {
		t.Fatalf("夹具有问题：token 已超阈值，测不出轮数条件")
	}
	if !a.compressOldest() {
		t.Fatal("30 轮 > 窗口 5 ⇒ 必须压")
	}
	if got := countRounds(a.history); got > 5 {
		t.Errorf("压缩后轮数应 ≤ 窗口 5，实得 %d", got)
	}
	t.Logf("✓ 轮数条件仍在：30 轮 ⇒ 压到 %d 轮", countRounds(a.history))
}
