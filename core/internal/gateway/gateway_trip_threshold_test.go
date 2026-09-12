// gateway_trip_threshold_test.go — 待修补 #32：tripMachine 写 failSince 的阈位必须引用真实常量 circuitFailThreshold。
//
// 历史缺陷：tripMachine 里写死过 `if g.failCounts[host] >= 3 { g.failSince[host] = time.Now() }`。
// failSince 是「首次熔断时间」，也是 circuitCooldown（30s 自动恢复）的计时起点；而真实熔断阈值是
// circuitFailThreshold=8——isTripped 只在 failCounts 达到该阈值之后（gateway.go 的阈值门）才会去读
// failSince 做冷却判定。于是在第 3 次失败就盖章：若第 3→第 8 次失败跨越超过 30s（failover 路径的偶发
// 失败正是这种节奏），第 8 次失败达阈值的那一刻就命中「冷却已过」→ 计数清零放行，熔断从未真正生效。
//
// 用例设计：既能区分阈位=3（过早）也能区分阈位=healthyTripLimit=10（过晚），把常量钉死在
// circuitFailThreshold 上；并固定住 `>=`（持续失败期间刷新计时起点）这一既有语义。

package gateway

import (
	"testing"
	"time"
)

// TestTripMachine_FailSinceNotStampedBeforeThreshold 阈值之下不得写 failSince（旧硬编码 3 会在此失败）。
func TestTripMachine_FailSinceNotStampedBeforeThreshold(t *testing.T) {
	g := breakerTestGateway()
	// 前 circuitFailThreshold-1 次 trip 全在阈值之下——failSince 必须一直为空。
	for i := 1; i < circuitFailThreshold; i++ {
		g.tripMachine("x3", "backend x3 forward failed: dial tcp <worker-ip>:8100: connect: connection refused")
		if fc := g.failCounts["x3"]; fc != i {
			t.Fatalf("前置条件：第 %d 次 trip 后 failCounts 应为 %d——实际 %d", i, i, fc)
		}
		if since, ok := g.failSince["x3"]; ok {
			t.Fatalf("failCounts=%d < 阈值 %d：failSince 不该被写"+
				"（旧硬编码 3 会在 failCounts=3 时盖章，使 30s 冷却计时提前起跑）——实际 %v",
				g.failCounts["x3"], circuitFailThreshold, since)
		}
	}
}

// TestTripMachine_FailSinceStampedExactlyAtThreshold 达阈值那一刻必须盖章（区分 8 与 healthyTripLimit=10）。
func TestTripMachine_FailSinceStampedExactlyAtThreshold(t *testing.T) {
	g := breakerTestGateway()
	for i := 0; i < circuitFailThreshold; i++ {
		g.tripMachine("x3", "backend x3 forward failed: context deadline exceeded")
	}
	since, ok := g.failSince["x3"]
	if !ok {
		t.Fatalf("failCounts=%d 已达 circuitFailThreshold：failSince 应被写"+
			"（此处为空说明阈位被写成 healthyTripLimit=%d）", g.failCounts["x3"], healthyTripLimit)
	}
	if since.IsZero() {
		t.Fatal("failSince 不应是零值时间")
	}
	// 计时起点=真实熔断时刻 ⇒ isTripped 必须判熔断，且剩余冷却为满额区间 (0, circuitCooldown]。
	if !g.isTripped("x3") {
		t.Fatal("failCounts 达阈值且 failSince 刚盖章——isTripped 应为 true")
	}
	b := findBreaker(g.BreakerSnapshot(), "x3")
	if b == nil {
		t.Fatal("快照里没有 x3")
	}
	if b.State != "open" {
		t.Errorf("达阈值后 state 应为 open——实际 %s", b.State)
	}
	if b.CooldownRemainingS <= 0 || b.CooldownRemainingS > circuitCooldown.Seconds() {
		t.Errorf("剩余冷却应落在 (0, %v] 秒——实际 %v", circuitCooldown.Seconds(), b.CooldownRemainingS)
	}
	t.Logf("达阈值: fail_count=%d state=%s open_since=%s 剩余冷却=%.1fs",
		b.FailCount, b.State, b.OpenSince, b.CooldownRemainingS)
}

// TestTripMachine_RepeatedTripRefreshesFailSince 已达阈值后再失败要刷新计时起点（固定 `>=` 而非 `==`）。
//
// 语义：持续失败的机器在每次 failover 都会被 tripMachine 计数，计时起点随之刷新，因此冷却期不会
// 在故障持续期间被跨过——不能退回「只在第 8 次盖一次章」的写法（那会让持续故障被自动半开放回）。
func TestTripMachine_RepeatedTripRefreshesFailSince(t *testing.T) {
	g := breakerTestGateway()
	for i := 0; i < circuitFailThreshold; i++ {
		g.tripMachine("x3", "backend x3 forward failed: 503")
	}
	// 模拟「冷却窗口已过」（旧的提前盖章正是把这一状态在达阈值前就制造出来）。
	g.failSince["x3"] = time.Now().Add(-(circuitCooldown + time.Second))
	// 再失败一次（failover 重试）——计时起点应回到现在。
	g.tripMachine("x3", "backend x3 forward failed: 503")
	if age := time.Since(g.failSince["x3"]); age > circuitCooldown {
		t.Fatalf("已过阈值的机器再失败时应刷新 failSince（保持熔断）——实际已过 %v", age)
	}
	if !g.isTripped("x3") {
		t.Fatal("刷新计时后仍在阈值之上且未过冷却——isTripped 应为 true")
	}
}

// TestIsTripped_StaleFailSinceBypassesTrip 反例取证：陈旧 failSince 会让达阈值那一刻直接放行。
//
// 这正是旧硬编码 3 在活系统上制造的现场——第 3 次失败盖章、第 8 次失败迟到 >30s 时，
// isTripped 命中「冷却已过」而清零放行，熔断形同虚设。本用例固定该语义，说明「计时起点必须落在
// 首次达阈值时刻」是承重条件，而不是可有可无的时间戳。
func TestIsTripped_StaleFailSinceBypassesTrip(t *testing.T) {
	g := breakerTestGateway()
	g.failCounts["x3"] = circuitFailThreshold
	g.failSince["x3"] = time.Now().Add(-(circuitCooldown + time.Second))
	if g.isTripped("x3") {
		t.Fatal("前置语义：冷却期已过，isTripped 应放行一次试错（返回 false）而不是熔断")
	}
	if n := g.failCounts["x3"]; n != 0 {
		t.Fatalf("放行时应清零 failCounts——实际 %d", n)
	}
	if _, ok := g.failSince["x3"]; ok {
		t.Fatal("放行时应清掉 failSince（冷却计时归零）")
	}
}
