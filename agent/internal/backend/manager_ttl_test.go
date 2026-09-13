package backend

// manager_ttl_test.go —— T7（《设计-资源管理器》§七 / §3.3(c) 触发①）故障注入式验收：
// 空闲超过 TTL 的驻留模型被卸载、从 resident 清单移除；未被 pin 的才卸。
//
// 本项产品侧此前**没有实现**（只有 pin 的 TTL，没有"空闲到期自动卸载"的路径）——
// 本次才实现 Manager.ReapIdle / StartIdleReaper（见 residency.go 的 TTL 段）。
// 本文件是反例优先：每条保护都先注入"本该不卸却被卸"的负例。

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// 注入固定时钟：TTL 判定用相对量，now 由测试给（产品代码不读时钟才有确定性）。
func ttlNow() time.Time { return time.Unix(1_700_000_000, 0) }

// ttlManager 造一个 TTL=300s 的测试管理器（nil proc：卸载路径安全空转，不起真进程）。
func ttlManager(ttl time.Duration, procs map[string]*subproc) *Manager {
	m := newEvictTestManager(3, procs)
	m.SetIdleTTL(ttl)
	return m
}

// 正例 + 反例：超 TTL 的空闲驻留被卸且从 resident 移除；未超 TTL 的留着。
func TestTTL_ReapsIdleExpiredOnly(t *testing.T) {
	now := ttlNow()
	m := ttlManager(300*time.Second, map[string]*subproc{
		"stale": {model: "stale", state: StateReady, lastUsed: now.Add(-time.Hour)},   // 空闲 3600s > 300s → 卸
		"fresh": {model: "fresh", state: StateReady, lastUsed: now.Add(-time.Minute)}, // 空闲 60s < 300s → 留
	})
	// 前置断言：两者此刻都在 resident 清单里（否则"移除"无从谈起）
	if got := m.ResidentDetail(); len(got) != 2 {
		t.Fatalf("前置：应有 2 个驻留，实得 %v", got)
	}

	reaped := m.ReapIdle(now)
	if len(reaped) != 1 || reaped[0] != "stale" {
		t.Fatalf("只该卸超 TTL 的空闲项 stale，实得 %v", reaped)
	}
	got := m.ResidentDetail()
	if len(got) != 1 || got[0].Alias != "fresh" {
		t.Fatalf("resident 应只剩 fresh（stale 被移除），实得 %v", got)
	}
	if _, still := m.procs["stale"]; still {
		t.Fatal("超 TTL 的驻留未被卸载，仍在 procs 里")
	}
}

// 反例：pin 未到期的驻留即便空闲超 TTL 也不卸；pin 到期后立刻恢复可卸（Q5）。
func TestTTL_PinnedNotReapedUntilExpiry(t *testing.T) {
	now := ttlNow()
	m := ttlManager(300*time.Second, map[string]*subproc{
		"pinned": {model: "pinned", state: StateReady, lastUsed: now.Add(-time.Hour), pinUntil: now.Add(10 * time.Minute)},
	})
	if reaped := m.ReapIdle(now); len(reaped) != 0 {
		t.Fatalf("pin 未到期的驻留不得被 TTL 卸载，实得 %v", reaped)
	}
	if _, still := m.procs["pinned"]; !still {
		t.Fatal("红线③被破：pin 未到期的驻留被 TTL 回收卸载了")
	}

	// pin 到期（now 越过 pinUntil）→ 恢复可卸
	later := now.Add(11 * time.Minute)
	if reaped := m.ReapIdle(later); len(reaped) != 1 || reaped[0] != "pinned" {
		t.Fatalf("pin 到期后应可被 TTL 卸载，实得 %v", reaped)
	}
	if _, still := m.procs["pinned"]; still {
		t.Fatal("pin 到期后仍未被卸载")
	}
}

// 反例：有在飞请求 / 加载中的驻留空闲再久也不卸（红线①；加载中正被请求等待）。
func TestTTL_InflightAndLoadingNotReaped(t *testing.T) {
	now := ttlNow()
	m := ttlManager(300*time.Second, map[string]*subproc{
		"busy":    {model: "busy", state: StateReady, lastUsed: now.Add(-24 * time.Hour), reqCount: 2},
		"loading": {model: "loading", state: StateLoading, lastUsed: now.Add(-24 * time.Hour)},
	})
	if reaped := m.ReapIdle(now); len(reaped) != 0 {
		t.Fatalf("在飞/加载中的驻留不得被 TTL 卸载，实得 %v", reaped)
	}
	if len(m.procs) != 2 {
		t.Fatalf("红线①被破：在飞/加载中的驻留被卸了，实得 %v", keysOf(m.procs))
	}
}

// 反例：TTL 显式设成 0（关闭）→ TTL 回收是空操作（逃生门：不想让空闲卸载生效时用它）。
// 注：默认值已由Mr2109 2026-09-13 定为 300 秒（DefaultIdleTTL），所以"关"必须显式表达。
func TestTTL_ExplicitZeroDisables(t *testing.T) {
	now := ttlNow()
	m := newEvictTestManager(3, map[string]*subproc{
		"stale": {model: "stale", state: StateReady, lastUsed: now.Add(-24 * time.Hour)},
	})
	m.SetIdleTTL(0)
	if m.IdleTTL() != 0 {
		t.Fatalf("显式设 0 后应为 0（未启用），实得 %v", m.IdleTTL())
	}
	if reaped := m.ReapIdle(now); len(reaped) != 0 {
		t.Fatalf("TTL 未启用时不得卸载任何驻留，实得 %v", reaped)
	}
	if _, still := m.procs["stale"]; !still {
		t.Fatal("TTL 未启用却把驻留卸了（默认行为被改变）")
	}
}

// 配置解析：缺省/空=默认 300 秒；正数=秒；0=关闭；非法（非数字/负数）=关闭（不静默接受怪值）。
func TestTTL_EnvParse(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"", DefaultIdleTTL}, // 缺省 = 300 秒（Mr2109 2026-09-13 拍板）
		{"300", 300 * time.Second},
		{" 600 ", 600 * time.Second},
		{"0", 0},
		{"-5", 0},
		{"abc", 0},
	}
	for _, c := range cases {
		got := resolveIdleTTLFromEnv(func(string) string { return c.env })
		if got != c.want {
			t.Fatalf("%s=%q 应得 %v，实得 %v", EnvModelTTL, c.env, c.want, got)
		}
	}
	if got := resolveIdleTTLFromEnv(nil); got != DefaultIdleTTL {
		t.Fatalf("nil getenv（读不到环境）应回落到默认 %v，实得 %v", DefaultIdleTTL, got)
	}
}

// 接线：缺省 = DefaultIdleTTL（300 秒，Mr2109 2026-09-13 拍板）；ZERG_MODEL_TTL_S 可覆盖；设 0 = 关闭。
func TestTTL_WiredThroughNewManager(t *testing.T) {
	t.Setenv(EnvMaxResident, "")
	t.Setenv(EnvModelTTL, "")
	if got := NewManager(nil, "x3").IdleTTL(); got != DefaultIdleTTL {
		t.Fatalf("未配置 TTL 时应取默认 %v，实得 %v", DefaultIdleTTL, got)
	}
	t.Setenv(EnvModelTTL, "0")
	if got := NewManager(nil, "x3").IdleTTL(); got != 0 {
		t.Fatalf("显式设 0 应关闭 TTL（0），实得 %v", got)
	}
	t.Setenv(EnvModelTTL, "")
	t.Setenv(EnvModelTTL, "300")
	m := NewManager(nil, "x3")
	defer m.StopIdleReaper()
	if got := m.IdleTTL(); got != 300*time.Second {
		t.Fatalf("EnvModelTTL=300 应生效为 300s，实得 %v", got)
	}
	// 已驻留的空闲模型在启用后能被回收（接线可用，不只是变量被读）
	m.mu.Lock()
	m.procs["idle"] = &subproc{model: "idle", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now().Add(-time.Hour)}
	m.mu.Unlock()
	if reaped := m.ReapIdle(time.Now()); len(reaped) != 1 || reaped[0] != "idle" {
		t.Fatalf("启用 TTL 后空闲置留应被回收，实得 %v", reaped)
	}
}
