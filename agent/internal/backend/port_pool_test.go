// port_pool_test.go —— 端口池隔离（设计 §7 S6 / §11 M11）。
//
// 用例一律用高位区间（58000+），绝不触碰真机端口。
// （「跳过 baseline 声明端口」的用例已随该机制退场——附录 C·C8。）
package backend

import (
	"testing"
	"time"
)

func TestFindFreePort_UsesPoolWhenFree(t *testing.T) {
	t.Setenv(EnvPortPool, "58030-58040")
	m := newEvictTestManager(1, map[string]*subproc{})
	got := m.findFreePort()
	if got < 58030 || got > 58040 {
		t.Fatalf("应落在池内，实得 %d", got)
	}
}

func TestFindFreePort_SkipsOwnResidents(t *testing.T) {
	t.Setenv(EnvPortPool, "58050-58052")
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
	m := newEvictTestManager(1, map[string]*subproc{})
	if got := m.findFreePort(); got == 0 {
		t.Fatal("非法池配置下仍应能分到端口（回落到默认区间）")
	}
}
