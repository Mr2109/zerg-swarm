package gateway

// gateway_breaker_test.go — 网关卡表只读快照 + 手动复位测试。
//
// 覆盖：初始态 / 失败计数与原因可见 / 达阈值熔断（open）/ 冷却到期（half-open）/
// 手动复位（全部/单机/未知机器）/ host 排序。

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// breakerTestGateway 构造一个带熔断相关 map 的网关实例（照抄 failover_test.go 的构造方式）。
func breakerTestGateway() *Gateway {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"ornith": {
				{Host: "x3", File: "/data/ornith.gguf"},
				{Host: "local", File: "~/ornith.gguf"},
			},
		},
		Fleet: map[string]config.FleetNode{
			"x3":    {Host: "<worker-ip>", Port: 8100},
			"mini1": {Host: "<worker-ip>", Port: 8100},
		},
	}
	return &Gateway{
		config:     cfg,
		store:      store.NewStore(),
		failCounts: map[string]int{},
		failSince:  map[string]time.Time{},
		tripCounts: map[string]int{},
		lastErr:    map[string]string{},
		lastErrAt:  map[string]time.Time{},
	}
}

// findBreaker 在快照里按 host 取一条（找不到返回 nil）。
func findBreaker(list []BreakerInfo, host string) *BreakerInfo {
	for i := range list {
		if list[i].Host == host {
			return &list[i]
		}
	}
	return nil
}

// TestBreakerSnapshot_EmptyGateway 零值 Gateway（无 config/store/无计数）——快照为空且不 panic。
func TestBreakerSnapshot_EmptyGateway(t *testing.T) {
	g := &Gateway{}
	snap := g.BreakerSnapshot()
	if len(snap) != 0 {
		t.Fatalf("零值 Gateway 快照应为空——实际 %d 条: %+v", len(snap), snap)
	}
	// 复位也应安全（无 map 时不 panic）
	if n := g.ResetBreakers(""); n != 0 {
		t.Fatalf("零值 Gateway 全量复位应返回 0——实际 %d", n)
	}
}

// TestBreakerSnapshot_InitialAllClosed 初始态：已知候选全部出现在快照里，state=closed、计数 0、冷却 0。
func TestBreakerSnapshot_InitialAllClosed(t *testing.T) {
	g := breakerTestGateway()
	snap := g.BreakerSnapshot()
	if len(snap) == 0 {
		t.Fatal("初始快照不应为空（fleet 配置里的候选机器应列出）")
	}
	for _, b := range snap {
		if b.State != "closed" {
			t.Errorf("初始 state 应为 closed——host=%s 实际 %s", b.Host, b.State)
		}
		if b.FailCount != 0 {
			t.Errorf("初始 fail_count 应为 0——host=%s 实际 %d", b.Host, b.FailCount)
		}
		if b.CooldownRemainingS != 0 {
			t.Errorf("closed 时 cooldown_remaining_s 应为 0——host=%s 实际 %v", b.Host, b.CooldownRemainingS)
		}
		if b.LastError != "" {
			t.Errorf("初始 last_error 应为空——host=%s 实际 %q", b.Host, b.LastError)
		}
		if b.FailThreshold != circuitFailThreshold {
			t.Errorf("fail_threshold 应为 %d——host=%s 实际 %d", circuitFailThreshold, b.Host, b.FailThreshold)
		}
	}
	if got := len(snap); got != 3 { // x3 / local / mini1
		t.Errorf("初始快照应有 3 台已知机器——实际 %d: %+v", got, snap)
	}
}

// TestBreakerSnapshot_SortedByHost 快照按 host 升序。
func TestBreakerSnapshot_SortedByHost(t *testing.T) {
	g := breakerTestGateway()
	snap := g.BreakerSnapshot()
	for i := 1; i < len(snap); i++ {
		if snap[i-1].Host > snap[i].Host {
			t.Fatalf("快照未按 host 升序: %q 在 %q 之前", snap[i-1].Host, snap[i].Host)
		}
	}
	// 明确断言顺序内容（local < mini1 < x3）
	want := []string{"local", "mini1", "x3"}
	if len(snap) != len(want) {
		t.Fatalf("快照条数应为 %d——实际 %d", len(want), len(snap))
	}
	for i, h := range want {
		if snap[i].Host != h {
			t.Errorf("排序第 %d 位应为 %s——实际 %s", i, h, snap[i].Host)
		}
	}
}

// TestBreakerSnapshot_FailureCountAndReasonVisible 一次失败：计数 1、原因可见、仍未熔断（阈值 8）。
// 注：本仓真实熔断阈值是 circuitFailThreshold=8（healthy 机器还有 healthyTripLimit=10 保护），
// 因此「一次失败」不会立刻 open——这是如实断言，不改成传说里的 3 次熔断。
func TestBreakerSnapshot_FailureCountAndReasonVisible(t *testing.T) {
	g := breakerTestGateway()
	g.markFailure("x3", "backend x3 returned 500: {\"error\":\"inference timeout\"}")

	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.FailCount != 1 {
		t.Errorf("fail_count 应为 1——实际 %d", b.FailCount)
	}
	if b.State != "closed" {
		t.Errorf("失败数 1 < 阈值 %d——state 应为 closed——实际 %s", circuitFailThreshold, b.State)
	}
	if b.CooldownRemainingS != 0 {
		t.Errorf("closed 时冷却应为 0——实际 %v", b.CooldownRemainingS)
	}
	if b.LastError == "" {
		t.Error("last_error 应非空（原因可见——本次修补重点）")
	}
	if b.LastErrorAt == "" {
		t.Error("last_error_at 应非空（原因发生时间）")
	}
	t.Logf("原因可见: host=%s fail_count=%d state=%s last_error=%q", b.Host, b.FailCount, b.State, b.LastError)
}

// TestBreakerSnapshot_OpensAtThreshold 失败数达阈值（circuitFailThreshold）→ state=open + 冷却 > 0。
func TestBreakerSnapshot_OpensAtThreshold(t *testing.T) {
	g := breakerTestGateway()
	for i := 0; i < circuitFailThreshold; i++ {
		g.markFailure("x3", "backend x3 returned 503")
	}
	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.FailCount != circuitFailThreshold {
		t.Errorf("fail_count 应为 %d——实际 %d", circuitFailThreshold, b.FailCount)
	}
	if b.State != "open" {
		t.Errorf("失败数达阈值 %d——state 应为 open——实际 %s", circuitFailThreshold, b.State)
	}
	if b.CooldownRemainingS <= 0 {
		t.Errorf("open 时 cooldown_remaining_s 应 > 0——实际 %v", b.CooldownRemainingS)
	}
	if b.CooldownRemainingS > circuitCooldown.Seconds() {
		t.Errorf("剩余冷却不应超过 %v 秒——实际 %v", circuitCooldown.Seconds(), b.CooldownRemainingS)
	}
	if b.LastError == "" {
		t.Error("熔断中 last_error 应非空（用户可见「为什么熔断」）")
	}
	if raw, err := json.Marshal(b); err == nil {
		t.Logf("熔断 JSON（GET /api/gateway/breakers 里单条的形状）: %s", raw)
	}
	t.Logf("熔断: host=%s state=%s fail_count=%d 剩余冷却=%.1fs last_error=%q",
		b.Host, b.State, b.FailCount, b.CooldownRemainingS, b.LastError)
}

// TestBreakerSnapshot_OpenSinceAfterTrip 走一次真实路由判定（isTripped）后，open_since 被记录（RFC3339）且快照与判定一致。
func TestBreakerSnapshot_OpenSinceAfterTrip(t *testing.T) {
	g := breakerTestGateway()
	for i := 0; i < circuitFailThreshold; i++ {
		g.markFailure("x3", "backend x3 returned 500")
	}
	// 达阈值但还没有任何路由判定 → 尚无首次熔断时间
	if b := findBreaker(g.BreakerSnapshot(), "x3"); b == nil || b.OpenSince != "" {
		t.Fatalf("尚未路由判定时 open_since 应为空——实际 %+v", b)
	}
	if tripped := g.isTripped("x3"); !tripped {
		t.Fatal("失败数达阈值后 isTripped 应为 true")
	}
	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.State != "open" {
		t.Errorf("isTripped=true 时 state 应为 open——实际 %s", b.State)
	}
	if b.OpenSince == "" {
		t.Error("isTripped 记录熔断后 open_since 应非空")
	} else if _, err := time.Parse(time.RFC3339, b.OpenSince); err != nil {
		t.Errorf("open_since 应为 RFC3339——实际 %q err=%v", b.OpenSince, err)
	}
	t.Logf("open_since=%s cooldown=%.1fs", b.OpenSince, b.CooldownRemainingS)
}

// TestBreakerSnapshot_HalfOpenAfterCooldown 冷却期（circuitCooldown）过后 → half-open 且剩余冷却 0。
func TestBreakerSnapshot_HalfOpenAfterCooldown(t *testing.T) {
	g := breakerTestGateway()
	g.failCounts["x3"] = circuitFailThreshold
	g.failSince["x3"] = time.Now().Add(-(circuitCooldown + time.Second)) // 已过冷却
	g.lastErr["x3"] = "backend x3 returned 504"

	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.State != "half-open" {
		t.Errorf("冷却到期后 state 应为 half-open——实际 %s", b.State)
	}
	if b.CooldownRemainingS != 0 {
		t.Errorf("half-open 剩余冷却应为 0——实际 %v", b.CooldownRemainingS)
	}
	if b.OpenSince == "" {
		t.Error("有过熔断时间时 open_since 应非空")
	}
}

// TestBreakerSnapshot_TripCountFromTripMachine tripMachine 触发次数（trip_count）在快照里可见。
func TestBreakerSnapshot_TripCountFromTripMachine(t *testing.T) {
	g := breakerTestGateway()
	g.tripMachine("mini1")
	b := findBreaker(g.BreakerSnapshot(), "mini1")
	if b == nil {
		t.Fatal("快照里没有 mini1")
	}
	if b.TripCount != 1 {
		t.Errorf("tripMachine 一次后 trip_count 应为 1——实际 %d", b.TripCount)
	}
}

// TestBreakerReset_All 全量复位：清掉计数/状态/原因，返回正确条数，复位后全 closed。
func TestBreakerReset_All(t *testing.T) {
	g := breakerTestGateway()
	for i := 0; i < circuitFailThreshold; i++ {
		g.markFailure("x3", "backend x3 returned 500")
	}
	g.markFailure("mini1", "backend mini1 returned 500")
	g.tripMachine("x3")
	if n := g.isTripped("x3"); !n {
		t.Fatal("前置条件：x3 应处于熔断")
	}

	cleared := g.ResetBreakers("")
	// 有过状态的机器：x3（failCounts/failSince/lastErr/tripCounts）+ mini1（failCounts/lastErr）= 2
	if cleared != 2 {
		t.Errorf("全量复位应清掉 2 台——实际 %d", cleared)
	}
	for _, b := range g.BreakerSnapshot() {
		if b.State != "closed" {
			t.Errorf("复位后 state 应为 closed——host=%s 实际 %s", b.Host, b.State)
		}
		if b.FailCount != 0 {
			t.Errorf("复位后 fail_count 应为 0——host=%s 实际 %d", b.Host, b.FailCount)
		}
		if b.TripCount != 0 {
			t.Errorf("复位后 trip_count 应为 0——host=%s 实际 %d", b.Host, b.TripCount)
		}
		if b.LastError != "" {
			t.Errorf("复位后 last_error 应为空——host=%s 实际 %q", b.Host, b.LastError)
		}
		if b.CooldownRemainingS != 0 {
			t.Errorf("复位后冷却应为 0——host=%s 实际 %v", b.Host, b.CooldownRemainingS)
		}
	}
	if g.failCounts["x3"] != 0 || g.failCounts["mini1"] != 0 {
		t.Errorf("复位后 failCounts 应清零——实际 x3=%d mini1=%d", g.failCounts["x3"], g.failCounts["mini1"])
	}
	if len(g.failSince) != 0 || len(g.tripCounts) != 0 || len(g.lastErr) != 0 || len(g.lastErrAt) != 0 {
		t.Errorf("复位后各 map 应为空——failSince=%v tripCounts=%v lastErr=%v lastErrAt=%v",
			g.failSince, g.tripCounts, g.lastErr, g.lastErrAt)
	}
	// 幂等：再复位一次返回 0
	if n := g.ResetBreakers(""); n != 0 {
		t.Errorf("重复复位应返回 0——实际 %d", n)
	}
	// 复位后 isTripped 应为 false（恢复路由）
	if g.isTripped("x3") {
		t.Error("复位后 x3 不应再被视为熔断")
	}
}

// TestBreakerReset_Single 单机复位：只清目标 host，其他机器状态保留。
func TestBreakerReset_Single(t *testing.T) {
	g := breakerTestGateway()
	g.markFailure("x3", "backend x3 returned 500")
	g.markFailure("mini1", "backend mini1 returned 500")

	if n := g.ResetBreakers("x3"); n != 1 {
		t.Errorf("单机复位应返回 1——实际 %d", n)
	}
	if b := findBreaker(g.BreakerSnapshot(), "x3"); b == nil || b.FailCount != 0 || b.LastError != "" {
		t.Errorf("x3 应已复位——实际 %+v", b)
	}
	if b := findBreaker(g.BreakerSnapshot(), "mini1"); b == nil || b.FailCount != 1 || b.LastError == "" {
		t.Errorf("mini1 不应被复位——实际 %+v", b)
	}
}

// TestBreakerReset_UnknownHost 不存在的 host：返回 0 且不 panic（幂等）。
func TestBreakerReset_UnknownHost(t *testing.T) {
	g := breakerTestGateway()
	if n := g.ResetBreakers("no-such-host"); n != 0 {
		t.Errorf("未参与熔断的 host 复位应返回 0——实际 %d", n)
	}
	// 空 map / 零值实例也不能 panic
	g2 := &Gateway{}
	if n := g2.ResetBreakers("x3"); n != 0 {
		t.Errorf("零值 Gateway 单机复位应返回 0——实际 %d", n)
	}
}
