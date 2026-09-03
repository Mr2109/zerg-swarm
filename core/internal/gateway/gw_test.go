package gateway

import (
	"testing"

	"zerg/core/internal/config"
	"zerg/core/internal/store"
)

// helper: 注入快照（模拟机器心跳）
func injectSnap(st *store.Store, machine, model string, active int, healthy bool, load float64) {
	st.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine:        machine,
		Model:          &model,
		ActiveRequests: active,
		Healthy:        healthy,
		Load:           load,
	})
}

func testGateway(snaps map[string]func(*store.Store)) *Gateway {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"example-35b-v2": {
				{Host: "x3", File: "/data/example-35b-v2.gguf"},
				{Host: "local", File: "~/example-35b-v2.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"x3": {Host: "<worker-ip>", Port: 8100},
		},
	}
	st := store.NewStore()
	for _, f := range snaps {
		f(st)
	}
	return &Gateway{
		config:       cfg,
		roundRobin:   map[string]int{"example-35b-v2": 0},
		excludeLocal: false,
		store:        st,
	}
}

// TestPickRoute_ActiveDowngrade — A 改进：X3 满负载（active>=4）时转其他机器
func TestPickRoute_ActiveDowngrade(t *testing.T) {
	g := testGateway(map[string]func(*store.Store){
		"x3": func(st *store.Store) { injectSnap(st, "x3", "example-35b-v2", 4, true, 0.8) }, // 满负载
	})
	route, err := g.pickRoute("example-35b-v2", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// X3 active=4 >= parallel 4 → 扣 8——若只有 x3/local（local 无快照不计分）——x3 仍会被选（无更好）
	// 但若快照显示"已加载+8 + 健康+1 - 满负载8 = 1"——local 无快照 0——x3 胜（没有更优选择）
	// 注：此测试验证"不 panic + 有路由"——降权逻辑在 TestParallelSlots 覆盖
	if route.Host == "" {
		t.Error("路由为空——pickRoute 失败")
	}
	t.Logf("满负载 X3 路由: %s (降权后仍可能是 x3——无更优候选时)", route.Host)
}

// TestPickRoute_ActiveDowngrade_WithIdle — X3 满负载 + mini1 空闲——应转 mini1（有更好选择时）
func TestPickRoute_ActiveDowngrade_WithIdle(t *testing.T) {
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
	injectSnap(st, "x3", "example-35b-v2", 4, true, 0.8)    // X3 满负载（已加载+8 健康+1 满-8 = 1）
	injectSnap(st, "mini1", "", 0, true, 0.1)        // mini1 空闲（健康+1 空闲+1 = 2）
	g := &Gateway{config: cfg, roundRobin: map[string]int{"example-35b-v2": 0}, excludeLocal: false, store: st}
	route, err := g.pickRoute("example-35b-v2", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if route.Host == "x3" {
		t.Errorf("X3 满负载且 mini1 空闲——仍选 X3——A 降权未生效")
	}
	if route.Host != "mini1" {
		t.Errorf("应转 mini1（空闲）——实际 %s", route.Host)
	}
}

// TestPickRoute_X3Busy_Transfers — Mr2109认知纠正（2026-08-15）：
//   X3 -np 4 ≠ 并行 4 任务（= 显存大能加载多模型——执行单任务 GPU 满）
//   → 执行层面单槽——X3 active=1 即忙——mini1 空闲时转走
func TestPickRoute_X3Busy_Transfers(t *testing.T) {
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
	injectSnap(st, "x3", "example-35b-v2", 1, true, 0.8)    // X3 忙（active=1 执行中——GPU 满）
	injectSnap(st, "mini1", "example-35b-v2", 0, true, 0.1)  // mini1 空闲（已加载）
	g := &Gateway{config: cfg, roundRobin: map[string]int{"example-35b-v2": 0}, excludeLocal: false, store: st}
	route, err := g.pickRoute("example-35b-v2", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if route.Host != "mini1" {
		t.Errorf("X3 忙（执行中）应转 mini1——实际 %s", route.Host)
	}
}

// TestParallelSlots — 执行层面统一单槽（Mr2109认知纠正——GPU 满=忙）
func TestParallelSlots(t *testing.T) {
	g := &Gateway{}
	if got := g.parallelSlots("local"); got != 1 {
		t.Errorf("local 并行度 = %d, want 1", got)
	}
	if got := g.parallelSlots("x3"); got != 1 {
		t.Errorf("x3 并行度 = %d, want 1（执行层面单任务）", got)
	}
}
