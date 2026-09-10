package inference

// registry_test.go — 运行时注册表测试（2026-08-29 q5 覆盖补齐）
// 目标: 注册/获取/全量（0% 包首测）

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestRegistry_Local 本机特殊处理——Get("local") 返回 local
func TestRegistry_Local(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	reg := NewRegistry(local)
	rt := reg.Get("local")
	if rt == nil || rt != local {
		t.Fatal("Get(local) 应返回本机 runtime")
	}
	if reg.Local() != local {
		t.Fatal("Local() 应返回本机")
	}
}

// TestRegistry_RegisterRemote 注册远端 → 可获取
func TestRegistry_RegisterRemote(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	reg := NewRegistry(local)
	st := store.NewStore()
	reg.RegisterRemote("x3", "http://<worker-ip>:8100", "test-token", st)
	rt := reg.Get("x3")
	if rt == nil {
		t.Fatal("x3 应可获取")
	}
	// RemoteRuntime 快照依赖 store 心跳——未心跳时 nil；用具体类型 Host 字段验证
	if rrt, ok := rt.(*RemoteRuntime); !ok || rrt.Host != "x3" {
		t.Fatalf("应为 RemoteRuntime 且 Host=x3: %T", rt)
	}
}

// TestRegistry_UnknownHost 未注册机器 → nil
func TestRegistry_UnknownHost(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	reg := NewRegistry(local)
	if rt := reg.Get("mini1"); rt != nil {
		t.Fatal("未注册机器应 nil")
	}
}

// TestRegistry_All local + remotes 全量
func TestRegistry_All(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	reg := NewRegistry(local)
	st := store.NewStore()
	reg.RegisterRemote("x3", "http://<worker-ip>:8100", "test-token", st)
	reg.RegisterRemote("mini1", "http://<worker-ip>:8100", "test-token", st)
	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All 应 3 个（local+x3+mini1）: %d", len(all))
	}
	if all["local"] == nil || all["x3"] == nil || all["mini1"] == nil {
		t.Fatal("All 应包含全部 runtime")
	}
}

// TestRegistry_LocalSnapshot 本机快照（未加载模型）
func TestRegistry_LocalSnapshot(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	snap := local.Snapshot()
	if snap == nil {
		t.Fatal("快照不应 nil")
	}
	if snap.Host != "local" {
		t.Fatalf("快照 Host = %s，应 local", snap.Host)
	}
	// 未加载 → 不健康/空模型
	if snap.Healthy {
		t.Fatal("未加载模型不应 healthy")
	}
}

// TestRegistry_LocalState 本机状态（未初始化 → idle/stopped）
func TestRegistry_LocalState(t *testing.T) {
	local := NewLocalRuntime("/tmp/zerg-test-infer.log")
	state := local.State()
	if state == "" {
		t.Fatal("State 不应为空")
	}
	if local.IsReady() {
		t.Fatal("未加载不应 ready")
	}
}
