package gateway

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// 机器种类档序的单测（设计 v1.7 §二/§三 · 判据 20/22）——正 1 负 3。

// 正：溢出序 ai(0) < mini(1) < work(2)。
func TestKindRankOrder(t *testing.T) {
	if !(kindRank(kindAI) < kindRank(kindMini) && kindRank(kindMini) < kindRank(kindWork)) {
		t.Fatalf("档序错：ai=%d mini=%d work=%d", kindRank(kindAI), kindRank(kindMini), kindRank(kindWork))
	}
}

// 负 1：未标 kind / 未知值 ⇒ 一律按最保守的 work（不得被当专用机优先用）。
func TestKindUnsetDefaultsToWork(t *testing.T) {
	for _, raw := range []string{"", "  ", "bogus", "LOCAL"} {
		if got := normalizeKind(raw); got != kindWork {
			t.Fatalf("未标/未知 %q ⇒ 应按 %q，实得 %q", raw, kindWork, got)
		}
	}
}

// 负 2：大小写/空白容错（" AI " = ai · "Mini" = mini）。
func TestKindNormalization(t *testing.T) {
	cases := map[string]string{" AI ": kindAI, "Mini": kindMini, "\twork\n": kindWork}
	for raw, want := range cases {
		if got := normalizeKind(raw); got != want {
			t.Fatalf("归一化错 %q ⇒ 期望 %q，实得 %q", raw, want, got)
		}
	}
}

// 正 + 负 3：从机队配置取档序；**未知 host（含 `local` 这类非机队名）按 work**。
func TestKindTierFromFleet(t *testing.T) {
	g := &Gateway{config: &config.FleetConfig{Fleet: map[string]config.FleetNode{
		"x3":    {Host: "<worker-ip>", Kind: kindAI},
		"Mr2109":  {Host: "127.0.0.1", Kind: kindWork},
		"mini1": {Host: "<worker-ip>", Kind: kindMini},
		"raw":   {Host: "<cluster-ip>"}, // 未标 kind
	}}}
	if g.kindTier("x3") != 0 || g.kindTier("mini1") != 1 || g.kindTier("Mr2109") != 2 {
		t.Fatalf("档序错：x3=%d mini1=%d Mr2109=%d", g.kindTier("x3"), g.kindTier("mini1"), g.kindTier("Mr2109"))
	}
	if g.kindTier("raw") != 2 {
		t.Fatalf("未标 kind 的机器应落 work 档（2），实得 %d", g.kindTier("raw"))
	}
	if g.kindTier("local") != 2 || g.kindTier("不存在的机器") != 2 {
		t.Fatalf("机队外的 host 应落 work 档（2）：local=%d", g.kindTier("local"))
	}
	if g.kindName("x3") != kindAI || g.kindName("Mr2109") != kindWork {
		t.Fatalf("kindName 错：x3=%q Mr2109=%q", g.kindName("x3"), g.kindName("Mr2109"))
	}
	// nil 安全（不许 panic）
	var nilGW *Gateway
	if nilGW.kindTier("x3") != 2 {
		t.Fatalf("nil Gateway 应安全落 work 档")
	}
}
