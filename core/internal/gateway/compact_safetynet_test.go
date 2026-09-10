// compact_safetynet_test.go — 丙批 §4.2 验收④：网关侧是 85% 安全网（不再 50% 抢跑）+ history ≥ 4 门槛（2026-09-10）
//
// 运行：cd core && go test ./internal/gateway/ -run CompactSafetyNet -v

package gateway

import (
	"testing"

	"zerg/core/internal/config"
)

// 阈值必须是 85%（安全网）——而不是旧实现的 50%（会与对话侧主压缩抢跑）
func TestCompactSafetyNetThresholdIs85Percent(t *testing.T) {
	g := &Gateway{config: &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"test-20k": {{CtxWindow: 20000}},
		},
	}}
	got := g.compactThreshold("test-20k")
	if want := 17000; got != want {
		t.Fatalf("网关安全网阈值应为 ctx×85%% = %d，实际 %d（旧实现是 50%%=%d）", want, got, 10000)
	}
	t.Logf("✓ 验收④-1：ctx=20000 → 安全网阈值 %d（85%%），不再是 50%%", got)
}

// 未注册模型 → 退回全局 maxSessionTokens 的 85%
func TestCompactSafetyNetThresholdFallback(t *testing.T) {
	g := &Gateway{config: &config.FleetConfig{Models: map[string][]config.ModelCandidate{}}}
	got := g.compactThreshold("未注册模型")
	if want := maxSessionTokens * 85 / 100; got != want {
		t.Fatalf("未注册模型应退回全局 85%% = %d，实际 %d", want, got)
	}
	t.Logf("✓ 验收④-2：未注册模型 → 全局 85%% = %d", got)
}

// len(history) ≥ 4 门槛：3 条不动、4 条才允许
func TestCompactSafetyNetHistoryGate(t *testing.T) {
	three := []byte(`{"messages":[{"role":"user","content":"1"},{"role":"assistant","content":"2"},{"role":"user","content":"3"}]}`)
	four := []byte(`{"messages":[{"role":"user","content":"1"},{"role":"assistant","content":"2"},{"role":"user","content":"3"},{"role":"assistant","content":"4"}]}`)
	if n := requestMessageCount(three); n != 3 {
		t.Fatalf("解析历史条数应为 3，实际 %d", n)
	}
	if n := requestMessageCount(four); n != 4 {
		t.Fatalf("解析历史条数应为 4，实际 %d", n)
	}
	if requestMessageCount([]byte("不是 json")) != 0 {
		t.Fatal("坏 JSON 应返回 0（宁可不动手）")
	}
	t.Log("✓ 验收④-3：history ≥ 4 门槛解析正确（坏 JSON → 0，不触发）")
}
