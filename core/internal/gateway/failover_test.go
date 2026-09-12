package gateway

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestPickFallbackRoute_SwitchMachine — failover 换机器（X3 失败→换 mini1）
func TestPickFallbackRoute_SwitchMachine(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"ornith": {
				{Host: "x3", File: "/data/ornith.gguf"},
				{Host: "mini1", File: "/mini1/ornith.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"x3":    {Host: "<worker-ip>", Port: 8100},
			"mini1": {Host: "<worker-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	injectSnap(st, "x3", "ornith", 0, true, 0.1)
	injectSnap(st, "mini1", "ornith", 0, true, 0.1)
	g := &Gateway{config: cfg, roundRobin: map[string]int{"ornith": 0}, excludeLocal: false, store: st,
		failCounts: map[string]int{}, failSince: map[string]time.Time{}}
	// 模拟 X3 转发失败——failover
	failed := &RouteResult{Host: "x3"}
	// required 显式传 nil（待修补 #38 ②：该参数改必填，漏传在编译期即不可能）
	route, err := g.pickFallbackRoute(failed, "ornith", "backend x3 forward failed: dial tcp <worker-ip>:8100: i/o timeout", nil)
	if err != nil {
		t.Fatal(err)
	}
	if route.Host == "x3" {
		t.Errorf("failover 不应选回 X3——实际 %s", route.Host)
	}
	if route.Host != "mini1" {
		t.Errorf("failover 应换 mini1——实际 %s", route.Host)
	}
	// X3 失败计数增加（trip 1 次 + 选回后强制熔断 = fail 3）
	if g.failCounts["x3"] < 1 {
		t.Errorf("X3 失败计数应 >=1——实际 %d", g.failCounts["x3"])
	}
	// 本路径必须留下原因：failover 是最典型的「涨计数」路径（tripMachine + 强制熔断）
	if g.lastErr["x3"] == "" {
		t.Error("failover 路径必须写 lastErr（否则快照出现「计数涨了原因空」）")
	}
}

// TestTripMachine_Threshold — 连续失败达真实阈值（circuitFailThreshold）才记录熔断时间
//
// 历史缺陷 #32：本用例曾断言「连续 3 次失败即记录熔断时间」，而真实阈值是 circuitFailThreshold=8
// 且 isTripped 只在达该阈值后才读 failSince——断言与判定不一致，已按真实语义修正。
func TestTripMachine_Threshold(t *testing.T) {
	g := &Gateway{failCounts: map[string]int{}, failSince: map[string]time.Time{}, tripCounts: map[string]int{}}
	// 阈值之下：计数照涨，但不记录熔断时间（否则 30s 冷却计时提前起跑）
	for i := 0; i < circuitFailThreshold-1; i++ {
		g.tripMachine("x3", "backend x3 forward failed: connection reset by peer")
	}
	if g.failCounts["x3"] != circuitFailThreshold-1 {
		t.Errorf("fail 计数应 %d——实际 %d", circuitFailThreshold-1, g.failCounts["x3"])
	}
	if _, ok := g.failSince["x3"]; ok {
		t.Errorf("失败数 < 阈值 %d 时不应记录熔断时间", circuitFailThreshold)
	}
	// 达阈值：记录熔断时间
	g.tripMachine("x3", "backend x3 forward failed: connection reset by peer")
	if g.failCounts["x3"] != circuitFailThreshold {
		t.Errorf("fail 计数应 %d——实际 %d", circuitFailThreshold, g.failCounts["x3"])
	}
	if _, ok := g.failSince["x3"]; !ok {
		t.Errorf("连续失败达阈值 %d 应记录熔断时间", circuitFailThreshold)
	}
}
