package gateway

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestPickFallbackRoute_SwitchMachine — failover 换机器（EDGE-NODE 失败→换 mini-node-1）
func TestPickFallbackRoute_SwitchMachine(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"example-35b-v2": {
				{Host: "edge-node", File: "/data/example-35b-v2.gguf"},
				{Host: "mini-node-1", File: "/mini-node-1/example-35b-v2.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"edge-node":    {Host: "<worker-ip>", Port: 8100},
			"mini-node-1": {Host: "<worker-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	injectSnap(st, "edge-node", "example-35b-v2", 0, true, 0.1)
	injectSnap(st, "mini-node-1", "example-35b-v2", 0, true, 0.1)
	g := &Gateway{config: cfg, roundRobin: map[string]int{"example-35b-v2": 0}, excludeLocal: false, store: st,
		failCounts: map[string]int{}, failSince: map[string]time.Time{}}
	// 模拟 EDGE-NODE 转发失败——failover
	failed := &RouteResult{Host: "edge-node"}
	route, err := g.pickFallbackRoute(failed, "example-35b-v2")
	if err != nil {
		t.Fatal(err)
	}
	if route.Host == "edge-node" {
		t.Errorf("failover 不应选回 EDGE-NODE——实际 %s", route.Host)
	}
	if route.Host != "mini-node-1" {
		t.Errorf("failover 应换 mini-node-1——实际 %s", route.Host)
	}
	// EDGE-NODE 失败计数增加（trip 1 次 + 选回后强制熔断 = fail 3）
	if g.failCounts["edge-node"] < 1 {
		t.Errorf("X3 失败计数应 >=1——实际 %d", g.failCounts["edge-node"])
	}
}

// TestTripMachine_Threshold — 连续失败 3 次触发熔断
func TestTripMachine_Threshold(t *testing.T) {
	g := &Gateway{failCounts: map[string]int{}, failSince: map[string]time.Time{}, tripCounts: map[string]int{}}
	for i := 0; i < 3; i++ {
		g.tripMachine("edge-node")
	}
	if g.failCounts["edge-node"] != 3 {
		t.Errorf("fail 计数应 3——实际 %d", g.failCounts["edge-node"])
	}
	if _, ok := g.failSince["edge-node"]; !ok {
		t.Error("连续 3 次失败应记录熔断时间")
	}
}
