package gateway

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestPickRoute_AllTripped — v2.5.5 #9：所有候选熔断时——返回明确错误（不兜底空 URL）
func TestPickRoute_AllTripped(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"example-35b-v2": {
				{Host: "x3", File: "/m.gguf", MemGb: 22},
			},
		},
		Fleet: map[string]config.FleetNode{
			"x3": {Host: "<worker-ip>", Port: 8100},
		},
	}
	g := &Gateway{
		config:     cfg,
		roundRobin: map[string]int{},
		failCounts: map[string]int{"x3": 99},
		failSince:  map[string]time.Time{"x3": time.Now()},
		store:      &store.Store{},
	}

	_, err := g.pickRoute("example-35b-v2", "", "")
	if err == nil {
		t.Fatal("期望错误（全熔断无候选）——但返回了 nil（可能空 URL 兜底）")
	}
	t.Logf("✅ 正确返回错误: %v", err)
}

// TestPickRoute_SingleExcluded — v2.5.5 #9：唯一候选是 local 且排除本机——明确错误
func TestPickRoute_SingleExcluded(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"example-35b-v2": {
				{Host: "local", File: "/m.gguf", MemGb: 22},
			},
		},
	}
	g := &Gateway{
		config:     cfg,
		roundRobin: map[string]int{},
		failCounts: map[string]int{},
		failSince:  map[string]time.Time{},
		store:      &store.Store{},
		excludeLocal: true,
	}

	_, err := g.pickRoute("example-35b-v2", "", "")
	if err == nil {
		t.Fatal("期望错误（local 被排除）——但返回了 nil（可能空 URL 兜底）")
	}
	t.Logf("✅ 正确返回错误: %v", err)
}
