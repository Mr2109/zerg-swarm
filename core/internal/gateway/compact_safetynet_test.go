// compact_safetynet_test.go — compactThreshold 的阈值口径（ctx×85% 安全网，不是旧实现的 50% 抢跑）
//
// GAP-20261021-05：原先这里另有一条「history ≥ 4 才动」的门槛用例 —— 它服务的网关侧自动压缩
// 已整段删除、那个解析函数也随之消失，故该用例一并删除。compactThreshold 仍被
// GET /v1/context/{id} 用着，两条阈值用例保留。
//
// 运行：cd core && go test ./internal/gateway/ -run CompactSafetyNet -v

package gateway

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
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
