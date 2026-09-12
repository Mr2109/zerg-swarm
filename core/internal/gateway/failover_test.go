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
			"example-35b-v2": {
				{Host: "x3", File: "/data/example-35b-v2.gguf"},
				{Host: "mini1", File: "/mini1/example-35b-v2.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"x3":    {Host: "<worker-ip>", Port: 8100},
			"mini1": {Host: "<worker-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	injectSnap(st, "x3", "example-35b-v2", 0, true, 0.1)
	injectSnap(st, "mini1", "example-35b-v2", 0, true, 0.1)
	g := &Gateway{config: cfg, roundRobin: map[string]int{"example-35b-v2": 0}, excludeLocal: false, store: st,
		failCounts: map[string]int{}, failSince: map[string]time.Time{}}
	// 模拟 X3 转发失败——failover
	failed := &RouteResult{Host: "x3"}
	route, err := g.pickFallbackRoute(failed, "example-35b-v2", "backend x3 forward failed: dial tcp <worker-ip>:8100: i/o timeout")
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

// TestTripMachine_Threshold — 连续失败 3 次触发熔断
func TestTripMachine_Threshold(t *testing.T) {
	g := &Gateway{failCounts: map[string]int{}, failSince: map[string]time.Time{}, tripCounts: map[string]int{}}
	for i := 0; i < 3; i++ {
		g.tripMachine("x3", "backend x3 forward failed: connection reset by peer")
	}
	if g.failCounts["x3"] != 3 {
		t.Errorf("fail 计数应 3——实际 %d", g.failCounts["x3"])
	}
	if _, ok := g.failSince["x3"]; !ok {
		t.Error("连续 3 次失败应记录熔断时间")
	}
}
