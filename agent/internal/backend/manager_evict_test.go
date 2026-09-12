package backend

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/shared/resources"
)

// ── 批 3：驻留上限（Q1 默认单槽且可配） ───────────────────────────────────────

// 反例优先：不配环境变量时上限必须是 1（单槽），不是旧硬编码的 3。
func TestMaxResident_DefaultsToSingleSlot(t *testing.T) {
	t.Setenv(EnvMaxResident, "")
	if got := resolveMaxResidentFromEnv(nil); got != resources.DefaultMaxResident {
		t.Fatalf("无 getenv 时应为默认单槽 %d，实得 %d", resources.DefaultMaxResident, got)
	}
	m := NewManager(nil, "x3")
	if got := m.MaxResident(); got != 1 {
		t.Fatalf("默认驻留上限必须是 1（单槽，Q1），实得 %d", got)
	}
}

// 显式配置多槽时生效；非法值回落默认（不许因为配错就放开上限）。
func TestMaxResident_ExplicitAndInvalid(t *testing.T) {
	cases := []struct {
		env  string
		want int
	}{
		{"3", 3},
		{" 2 ", 2},
		{"0", 1},   // <=0 → 默认（不解释成"无限"）
		{"-1", 1},  // 负数 → 默认
		{"abc", 1}, // 非数字 → 默认
		{"", 1},    // 空 → 默认
	}
	for _, c := range cases {
		got := resolveMaxResidentFromEnv(func(string) string { return c.env })
		if got != c.want {
			t.Fatalf("%s=%q 应得 %d，实得 %d", EnvMaxResident, c.env, c.want, got)
		}
	}
	t.Setenv(EnvMaxResident, "3")
	if got := NewManager(nil, "x3").MaxResident(); got != 3 {
		t.Fatalf("显式多槽配置未生效：实得 %d", got)
	}
	m := &Manager{procs: map[string]*subproc{}, loading: map[string]*loadWaiter{}, machine: "x3"}
	m.SetMaxResident(2)
	if got := m.MaxResident(); got != 2 {
		t.Fatalf("SetMaxResident 未生效：实得 %d", got)
	}
	m.SetMaxResident(0)
	if got := m.MaxResident(); got != 1 {
		t.Fatalf("SetMaxResident(0) 应回到默认单槽，实得 %d", got)
	}
}

// 单槽现实：默认上限 1 时，来第二个模型先把空闲的那个淘汰掉（上限真实生效）。
func TestEvict_CountLimit_SingleSlotEvictsIdle(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{
		"old": {model: "old", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-time.Hour)},
	})
	m.evictIfNeededLocked()
	if len(m.procs) != 0 {
		t.Fatalf("单槽上限下应淘汰空闲驻留，实得 %v", keysOf(m.procs))
	}

	// 多槽（显式 3）：只驻留 2 个 → 不动
	m3 := newEvictTestManager(3, map[string]*subproc{
		"a": {model: "a", state: StateReady, lastUsed: time.Now()},
		"b": {model: "b", state: StateReady, lastUsed: time.Now()},
	})
	m3.evictIfNeededLocked()
	if len(m3.procs) != 2 {
		t.Fatalf("多槽上限下不该淘汰（2<3），实得 %v", keysOf(m3.procs))
	}
}

// ── 红线①：有在飞请求（reqCount>0）的驻留绝不驱逐 ─────────────────────────────

func TestEvict_RedLine_InflightNeverEvicted(t *testing.T) {
	// 干扰条件给满：最旧、最大、状态是 crashed——仍不许动它
	m := newEvictTestManager(1, map[string]*subproc{
		"busy": {
			model:    "busy",
			state:    StateCrashed,
			entry:    &registry.ModelEntry{MemGB: 86},
			lastUsed: time.Now().Add(-24 * time.Hour),
			reqCount: 2,
		},
		"idle": {model: "idle", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now()},
	})
	m.evictIfNeededLocked()
	if _, still := m.procs["busy"]; !still {
		t.Fatal("红线①被破：有在飞请求的驻留被驱逐了")
	}
	if _, gone := m.procs["idle"]; gone {
		t.Fatal("应淘汰的是空闲项 idle，它却还在")
	}

	// 内存线：要腾 100G 也不许动在飞项
	m2 := newEvictTestManager(1, map[string]*subproc{
		"busy": {model: "busy", state: StateReady, entry: &registry.ModelEntry{MemGB: 86}, lastUsed: time.Now().Add(-time.Hour), reqCount: 1},
	})
	if freed := m2.evictForMemoryLocked(100); freed != 0 {
		t.Fatalf("红线①被破：在飞项被算进腾退量 %.1fGB", freed)
	}
	if _, still := m2.procs["busy"]; !still {
		t.Fatal("红线①被破：内存腾退把在飞项卸了")
	}
}

// ── 红线③：pin 且 TTL 未到期者不驱逐；无 TTL 的 pin 直接拒绝（Q5） ─────────────

func TestEvict_RedLine_PinTTL(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{
		"review": {model: "review", state: StateReady, entry: &registry.ModelEntry{MemGB: 17}, lastUsed: time.Now().Add(-time.Hour)},
	})
	if err := m.Pin("review", time.Hour); err != nil {
		t.Fatalf("带 TTL 的 pin 应成功：%v", err)
	}
	if _, ok := m.PinRemainS("review"); !ok {
		t.Fatal("pin 后应能读到剩余 TTL")
	}
	m.evictIfNeededLocked()
	if _, still := m.procs["review"]; !still {
		t.Fatal("红线③被破：pin 未到期的驻留被上限淘汰了")
	}
	if freed := m.evictForMemoryLocked(100); freed != 0 {
		t.Fatalf("红线③被破：pin 未到期项被算进腾退量 %.1fGB", freed)
	}
	if _, still := m.procs["review"]; !still {
		t.Fatal("红线③被破：pin 未到期项被内存腾退卸了")
	}

	// TTL 到期 → 立刻恢复可驱逐
	m.procs["review"].pinUntil = time.Now().Add(-time.Second)
	if _, ok := m.PinRemainS("review"); ok {
		t.Fatal("pin 到期后不应再报有效")
	}
	m.evictIfNeededLocked()
	if _, still := m.procs["review"]; still {
		t.Fatal("pin 到期后应可被淘汰")
	}

	// Q5：无 TTL 的 pin 一律拒绝（无 TTL 的 pin 等同内存泄漏）
	m2 := newEvictTestManager(1, map[string]*subproc{
		"x": {model: "x", state: StateReady, lastUsed: time.Now()},
	})
	if err := m2.Pin("x", 0); err == nil {
		t.Fatal("TTL=0 的 pin 必须被拒绝（Q5）")
	}
	if err := m2.Pin("x", -time.Minute); err == nil {
		t.Fatal("负 TTL 的 pin 必须被拒绝（Q5）")
	}
	if _, ok := m2.PinRemainS("x"); ok {
		t.Fatal("被拒绝的 pin 不得留下生效状态")
	}
	if err := m2.Unpin("不存在"); err == nil {
		t.Fatal("未驻留的模型无法 unpin，应报错")
	}
}

// ── 红线②：未托管进程绝不被接管、绝不被杀（Q6） ──────────────────────────────

func TestEvict_RedLine_UnmanagedUntouched(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("起临时监听失败: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// 只读探测：它只该被"看见"（managed=false），不进托管清单
	m := newEvictTestManager(2, map[string]*subproc{
		"managed": {model: "managed", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now().Add(-time.Hour)},
	})
	got := m.UnmanagedListeners([]int{port})
	if len(got) != 1 || got[0].Managed {
		t.Fatalf("未托管监听应被如实报告且 managed=false，实得 %+v", got)
	}
	if len(m.procs) != 1 {
		t.Fatalf("探测不得写进托管清单，实得 %v", keysOf(m.procs))
	}

	// 驱逐/腾退全流程跑一遍：未托管端口必须还活着
	m.evictIfNeededLocked()
	if freed := m.evictForMemoryLocked(1000); freed == 0 {
		t.Fatal("托管项应被腾退（否则本用例没测到驱逐路径）")
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("红线②被破：驱逐流程之后未托管监听不可用（说明被杀了）: %v", err)
	}
	conn.Close()
	// 托管清单里只剩我们自己起的进程；未托管端口从来不在其中
	for name, sp := range m.procs {
		if sp.port == port {
			t.Fatalf("红线②被破：未托管端口 %d 被登记成托管项 %s", port, name)
		}
	}
}

// ── 只卸够：按五档顺序腾出"刚好够"的量，不是一把全清 ─────────────────────────

func TestEvictForMemory_JustEnoughNotAll(t *testing.T) {
	m := newEvictTestManager(3, map[string]*subproc{
		"A": {model: "A", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-300 * time.Second)},
		"B": {model: "B", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-200 * time.Second)},
		"C": {model: "C", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-100 * time.Second)},
	})
	freed := m.evictForMemoryLocked(9) // 缺口 9G：A(8) + B(8) = 16G ≥ 9 → 只该卸两个
	if freed != 16 {
		t.Fatalf("应腾出 16GB（A+B），实得 %.1fGB", freed)
	}
	if _, still := m.procs["C"]; !still {
		t.Fatalf("只卸够：C 不该被卸（实得剩余 %v）", keysOf(m.procs))
	}
	if len(m.procs) != 1 {
		t.Fatalf("应只剩 C，实得 %v", keysOf(m.procs))
	}

	// 反例：缺口小于第一个项的占用 → 只卸一个
	m2 := newEvictTestManager(3, map[string]*subproc{
		"A": {model: "A", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-300 * time.Second)},
		"B": {model: "B", state: StateReady, entry: &registry.ModelEntry{MemGB: 8}, lastUsed: time.Now().Add(-200 * time.Second)},
	})
	if freed := m2.evictForMemoryLocked(1); freed != 8 {
		t.Fatalf("腾 1G 只需最旧一个（8G），实得 %.1fGB", freed)
	}
	if len(m2.procs) != 1 {
		t.Fatalf("只该卸一个，实得 %v", keysOf(m2.procs))
	}
}

// 加载中的驻留不驱逐（正被请求等待——杀它等于让那个请求失败）。
func TestEvict_LoadingNotEvicted(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{
		"loading": {model: "loading", state: StateLoading, entry: &registry.ModelEntry{MemGB: 20}, lastUsed: time.Now().Add(-time.Hour)},
	})
	m.evictIfNeededLocked()
	if _, still := m.procs["loading"]; !still {
		t.Fatal("加载中的驻留被驱逐了（正被请求等待）")
	}
	if freed := m.evictForMemoryLocked(100); freed != 0 {
		t.Fatalf("加载中的驻留不得计入腾退量，实得 %.1fGB", freed)
	}
}

// ── 红线④：内存预检 fail-closed（腾不出缺口 → 507 拒装） ─────────────────────

func TestMemoryPrecheck_FailClosed507(t *testing.T) {
	// 反例：可用 1G，需要一个 86G 的模型，且唯一可腾退项被 pin 保护 → 腾不出 → 拒装
	m := newEvictTestManager(1, map[string]*subproc{
		"pinned": {model: "pinned", state: StateReady, entry: &registry.ModelEntry{MemGB: 20}, lastUsed: time.Now().Add(-time.Hour)},
	})
	if err := m.Pin("pinned", time.Hour); err != nil {
		t.Fatal(err)
	}
	ok, have, need := m.ensureMemoryForLocked(86, 1)
	if ok {
		t.Fatalf("腾不出缺口时必须拒装（fail-closed），实得 ok=true have=%.1f need=%.1f", have, need)
	}
	resp := m.rejectInsufficientMemory(have, need)
	if resp["status"] != 507 || resp["code"] != "insufficient memory" {
		t.Fatalf("拒装响应必须保持 507/insufficient memory 语义，实得 %+v", resp)
	}
	if _, still := m.procs["pinned"]; !still {
		t.Fatal("拒装过程中不许卸掉被保护的驻留")
	}

	// 正例：缺口能被腾出来 → 通过（且只卸够）
	m2 := newEvictTestManager(2, map[string]*subproc{
		"idle": {model: "idle", state: StateReady, entry: &registry.ModelEntry{MemGB: 20}, lastUsed: time.Now().Add(-time.Hour)},
	})
	// 需要 22*1.1=24.2G，可用 4G → 缺口 20.2G，空闲的 idle 正好 20G……不够 → 拒装
	if ok, _, _ := m2.ensureMemoryForLocked(22, 4); ok {
		t.Fatal("腾 20G < 缺口 20.2G，必须拒装（不许四舍五入放行）")
	}

	m3 := newEvictTestManager(2, map[string]*subproc{
		"idle": {model: "idle", state: StateReady, entry: &registry.ModelEntry{MemGB: 25}, lastUsed: time.Now().Add(-time.Hour)},
	})
	if ok, have, need := m3.ensureMemoryForLocked(22, 4); !ok {
		t.Fatalf("腾 25G ≥ 缺口 20.2G，应放行，实得 have=%.1f need=%.1f", have, need)
	}
	if len(m3.procs) != 0 {
		t.Fatalf("放行前应已卸掉空闲项，实得 %v", keysOf(m3.procs))
	}
}

// ── 定向卸载（主控"只卸够"的动作侧） ──────────────────────────────────────────

func TestUnload_TargetedAndGuards(t *testing.T) {
	m := newEvictTestManager(3, map[string]*subproc{
		"a":     {model: "a", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now()},
		"b":     {model: "b", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now()},
		"busy":  {model: "busy", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now(), reqCount: 1},
		"pinnd": {model: "pinnd", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now()},
	})
	if err := m.Pin("pinnd", time.Hour); err != nil {
		t.Fatal(err)
	}
	res := m.Unload([]string{"a", "busy", "pinnd", "ghost"})
	stopped, _ := res["stopped"].([]string)
	skipped, _ := res["skipped"].([]string)
	reasons, _ := res["reasons"].(map[string]string)
	if len(stopped) != 1 || stopped[0] != "a" {
		t.Fatalf("只该卸 a，实得 stopped=%v", stopped)
	}
	if len(skipped) != 3 {
		t.Fatalf("busy/pinnd/ghost 都应被跳过，实得 %v", skipped)
	}
	if reasons["busy"] != "inflight" || reasons["pinnd"] != "pin_active" || reasons["ghost"] != "not_resident" {
		t.Fatalf("跳过原因不对: %+v", reasons)
	}
	if _, still := m.procs["busy"]; !still {
		t.Fatal("红线①被破：定向卸载杀掉了在飞项")
	}
	if _, still := m.procs["pinnd"]; !still {
		t.Fatal("红线③被破：定向卸载杀掉了 pin 未到期项")
	}
}

// ── 工具 ────────────────────────────────────────────────────────────────────

// newEvictTestManager 造一个测试管理器：procs 里的进程用 nil proc（stopSubproc 会安全空转），
// 这样驱逐"裁决+执行"路径可以在不起真进程的情况下被验证。
func newEvictTestManager(limit int, procs map[string]*subproc) *Manager {
	return &Manager{
		procs:       procs,
		loading:     map[string]*loadWaiter{},
		machine:     "x3",
		maxResident: limit,
	}
}

func keysOf(procs map[string]*subproc) []string {
	out := make([]string, 0, len(procs))
	for k := range procs {
		out = append(out, k)
	}
	return out
}
