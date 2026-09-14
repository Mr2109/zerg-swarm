// port_pool_test.go —— P4 端口池隔离（设计 §7 S6 / §11 M11）。
//
// 背景：端口分配器原本扫 9000-9999，会把 Mr2109 手工服务停摆期间的坑当成"空闲"占掉
// （今天实测 ds4 抢占了 :9000，恢复 Qwen 时才发现）。
// 用例一律用高位区间（58000+），绝不触碰真机上的 9000/9001。
package backend

import (
	"testing"
	"time"
)

func TestFindFreePort_SkipsDeclaredBaseline(t *testing.T) {
	t.Setenv(EnvPortPool, "58000-58020")
	t.Setenv(EnvBaselinePorts, "58000,58001") // 声明它们属于手工服务
	m := newEvictTestManager(1, map[string]*subproc{})

	got := m.findFreePort()
	if got == 0 {
		t.Fatal("应能在区间内分到端口")
	}
	if got == 58000 || got == 58001 {
		t.Fatalf("分到了 baseline 声明的端口 %d（绝不能占用手工服务的坑）", got)
	}
	if got < 58000 || got > 58020 {
		t.Fatalf("应落在 ZERG_PORT_POOL 区间内，实得 %d", got)
	}
}

func TestFindFreePort_UsesPoolWhenFree(t *testing.T) {
	t.Setenv(EnvPortPool, "58030-58040")
	t.Setenv(EnvBaselinePorts, "")
	m := newEvictTestManager(1, map[string]*subproc{})
	got := m.findFreePort()
	if got < 58030 || got > 58040 {
		t.Fatalf("应落在池内，实得 %d", got)
	}
}

func TestFindFreePort_SkipsOwnResidents(t *testing.T) {
	t.Setenv(EnvPortPool, "58050-58052")
	t.Setenv(EnvBaselinePorts, "")
	m := newEvictTestManager(3, map[string]*subproc{
		"a": {model: "a", state: StateReady, port: 58050, lastUsed: time.Now()},
		"b": {model: "b", state: StateReady, port: 58051, lastUsed: time.Now()},
	})
	got := m.findFreePort()
	if got != 58052 {
		t.Fatalf("应跳过本端已占用的 58050/58051，实得 %d", got)
	}
}

func TestFindFreePort_InvalidPoolFallsBack(t *testing.T) {
	// 非法池配置不该让分配器整体失效（回落到默认区间逻辑；这里只断言"仍能分到端口"）。
	t.Setenv(EnvPortPool, "不是区间")
	t.Setenv(EnvBaselinePorts, "")
	m := newEvictTestManager(1, map[string]*subproc{})
	if got := m.findFreePort(); got == 0 {
		t.Fatal("非法池配置下仍应能分到端口（回落到默认 9000-9999）")
	}
}
