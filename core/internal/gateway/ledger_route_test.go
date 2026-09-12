package gateway

// ledger_route_test.go —— 批 5：资源账本接进候选路由的反例优先测试（《设计-资源管理器》§三.5）。
//
// 覆盖：
//   ① 账本缺席（旧子端只报内存/状态）→ 增量 0，路由行为与改动前逐字一致（旧公式基准断言）；
//   ② 账本可用但"不知道"（显存未知 / 机器没报内存）→ 不降权（绝不因不知道排候选）；
//   ③ 确定装不下（内存或显存任一不够，二者取严）→ 降权；未托管进程的内存不算我们的；
//   ④ 冷/温/热三态与 loading → 降权档位正确；
//   ⑤ 既有单槽降权语义未被破坏（回归钉住）。

import (
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// ledgerTestCand 造一个"要 22G"的候选。
func ledgerTestCand(host, file string) config.ModelCandidate {
	return config.ModelCandidate{Host: host, File: file, MemGb: 22}
}

// ① 账本缺席 → 增量 0（红线：绝不因"不知道"降权/排掉候选）。
func TestLedgerAdjust_AbsentLedgerIsNoOp(t *testing.T) {
	g := &Gateway{}
	cand := ledgerTestCand("x3", "/models/target.gguf")
	// 旧版子端快照：只有内存/状态，没有 resident[]、没有 vram_*（账本缺席）。
	// 注意内存故意给得很小（4G < 22G）——若账本缺席时仍去判内存，就会误降权。
	snap := &store.FleetSnapshot{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 4, Healthy: true, BackendState: "ready"}
	if d, why := g.ledgerAdjust(snap, cand); d != 0 {
		t.Fatalf("账本缺席必须增量 0（不得判内存不足），实得 %d (%s)", d, why)
	}
	if d, why := g.ledgerAdjust(nil, cand); d != 0 {
		t.Fatalf("无快照必须增量 0，实得 %d (%s)", d, why)
	}
}

// ② 账本在、但装不下的判据"不确定" → 不降权。
func TestLedgerAdjust_UnknownIsNotPenalized(t *testing.T) {
	g := &Gateway{}
	cand := ledgerTestCand("x3", "/models/target.gguf")
	// 机器报了驻留账本（所以 ledgerPresent=true），但显存未知且非统一内存：
	// 显存未知既不是"够"也不是"不够"——只允许按内存口径判；内存够 → 只该吃"冷"档。
	snap := &store.FleetSnapshot{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 100, Healthy: true,
		Resident: []resources.ResidentEntry{resident("other", 8, 10, true, 0)},
	}
	d, why := g.ledgerAdjust(snap, cand)
	if d != ledgerColdPenalty {
		t.Fatalf("显存未知不得判 no_fit（内存够），应只降冷档 %d，实得 %d (%s)", ledgerColdPenalty, d, why)
	}
	if !strings.Contains(why, "cold") {
		t.Fatalf("原因应写明冷态，实得 %q", why)
	}
	// 机器连内存总量都没报 → 内存判不了；它报了驻留账本、目标不在其中 → 只该吃"冷"档
	narrow := &store.FleetSnapshot{
		Machine: "x3", Resident: []resources.ResidentEntry{resident("other", 8, 10, true, 0)},
	}
	if d, why := g.ledgerAdjust(narrow, cand); d != ledgerColdPenalty {
		t.Fatalf("内存未知时不得判 no_fit（内存判不了），只该吃冷档 %d，实得 %d (%s)", ledgerColdPenalty, d, why)
	}
}

// ③ 确定装不下（内存不够）→ 降权，且**未托管进程的内存不算我们的**（Q6）。
func TestLedgerAdjust_NoFitByMemory(t *testing.T) {
	g := &Gateway{}
	cand := ledgerTestCand("x3", "/models/target.gguf")
	// 可用 4G + 可腾退 2G（自己的）= 6G < 22G → 确定装不下
	snap := &store.FleetSnapshot{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 4, Healthy: true,
		Resident: []resources.ResidentEntry{resident("ours", 2, 10, true, 0)},
	}
	d, why := g.ledgerAdjust(snap, cand)
	if d != ledgerNoFitPenalty || !strings.Contains(why, "no_fit(mem") {
		t.Fatalf("内存确定不够应降 no_fit 档 %d，实得 %d (%s)", ledgerNoFitPenalty, d, why)
	}
	// 反例：把 90G 记在**未托管**进程名下 → 那不是我们的内存，不许拿来"腾"→ 照样确定装不下
	snap2 := &store.FleetSnapshot{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 4, Healthy: true,
		Resident: []resources.ResidentEntry{resident("manual@127.0.0.1:9001", 90, 9999, false, 0)},
	}
	if d2, why2 := g.ledgerAdjust(snap2, cand); d2 != ledgerNoFitPenalty {
		t.Fatalf("未托管进程的内存不得算作可腾退（Q6），实得 %d (%s)", d2, why2)
	}
}

// ③ 确定装不下（显存不够，内存够）→ 降权（§3.3d 二者取严）。
func TestLedgerAdjust_NoFitByVram(t *testing.T) {
	g := &Gateway{}
	cand := ledgerTestCand("x3", "/models/target.gguf")
	snap := &store.FleetSnapshot{
		Machine: "x3", MemTotalGb: 128, MemAvailableGb: 100, Healthy: true,
		VramKnown: true, VramTotalGb: 24, VramFreeGb: 4,
	}
	d, why := g.ledgerAdjust(snap, cand)
	if d != ledgerNoFitPenalty || !strings.Contains(why, "no_fit(vram") {
		t.Fatalf("内存够但显存不够应降 no_fit 档 %d（二者取严），实得 %d (%s)", ledgerNoFitPenalty, d, why)
	}
}

// ④ 冷/温/热 + loading 档位。
func TestLedgerAdjust_ResidencyStates(t *testing.T) {
	g := &Gateway{}
	cand := ledgerTestCand("x3", "/models/target.gguf")
	target := resources.ResidentEntry{
		Alias: "target", File: "/models/target.gguf", State: resources.StateReady, MemGb: 22, Managed: true,
	}
	base := func(rs []resources.ResidentEntry) *store.FleetSnapshot {
		return &store.FleetSnapshot{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 100, Healthy: true, Resident: rs}
	}
	// 热态（目标 ready 已驻留）→ 0：既有"已加载 +8"已表达优先，账本不叠加
	if d, why := g.ledgerAdjust(base([]resources.ResidentEntry{target}), cand); d != 0 {
		t.Fatalf("目标已驻留（ready）应增量 0，实得 %d (%s)", d, why)
	}
	// 加载中 → -3
	loading := target
	loading.State = resources.StateLoading
	if d, why := g.ledgerAdjust(base([]resources.ResidentEntry{loading}), cand); d != ledgerLoadingPenalty {
		t.Fatalf("目标加载中应降 %d，实得 %d (%s)", ledgerLoadingPenalty, d, why)
	}
	// 驻留的是别人（账本在、目标不在）→ 冷 -2
	if d, why := g.ledgerAdjust(base([]resources.ResidentEntry{resident("other", 8, 10, true, 0)}), cand); d != ledgerColdPenalty {
		t.Fatalf("目标不在驻留清单应降冷档 %d，实得 %d (%s)", ledgerColdPenalty, d, why)
	}
	// 账本说目标 crashed（不可用，要重载）→ 按冷档降权，不让"假热态"占优
	crashed := target
	crashed.State = resources.StateCrashed
	if d, why := g.ledgerAdjust(base([]resources.ResidentEntry{crashed}), cand); d != ledgerColdPenalty {
		t.Fatalf("目标 crashed 应按冷档降 %d，实得 %d (%s)", ledgerColdPenalty, d, why)
	}
	// 账本在但没有驻留项、显存也未知 → 没有冷态信号 → 0
	if d, why := g.ledgerAdjust(base(nil), cand); d != 0 {
		t.Fatalf("空驻留清单 + 显存未知应增量 0，实得 %d (%s)", d, why)
	}
}

// ⑤ 路由回归：账本缺席 → 与改动前逐字一致（用旧公式的基准断言）。
// 旧公式（gateway.go 打分循环）：健康+1 / 已加载+8 / 空闲+1 / 负载低+1（<0.5）/ 轮询微倾向±2。
func TestPickRoute_LedgerAbsentKeepsLegacyScoring(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"m": {
				{Host: "local", File: "/models/local-m.gguf", MemGb: 22},
				{Host: "x3", File: "/models/x3-m.gguf", MemGb: 22},
			},
		},
		Fleet: map[string]config.FleetNode{"x3": {Host: "<worker-ip>", Port: 8100}},
	}
	st := store.NewStore()
	// 账本缺席的快照（旧子端/本机未接账本）：只有内存/健康/状态
	st.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", Healthy: true, MemTotalGb: 64, MemAvailableGb: 40})
	st.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "x3", Healthy: true, MemTotalGb: 128, MemAvailableGb: 100})
	g := &Gateway{config: cfg, store: st, roundRobin: map[string]int{}, failCounts: map[string]int{}, failSince: map[string]time.Time{}}

	// 账本缺席的断言：两台候选的账本增量都必须是 0
	if d, why := g.ledgerAdjust(st.GetSnapshot("local"), cfg.Models["m"][0]); d != 0 {
		t.Fatalf("local 账本缺席必须增量 0，实得 %d (%s)", d, why)
	}
	if d, why := g.ledgerAdjust(st.GetSnapshot("x3"), cfg.Models["m"][1]); d != 0 {
		t.Fatalf("x3 账本缺席必须增量 0，实得 %d (%s)", d, why)
	}

	// rr=0（偶数轮）：local 微倾向 +2 ⇒ local 5 / x3 3 ⇒ 旧行为选 local
	r, err := g.pickRoute("m", "", "")
	if err != nil {
		t.Fatalf("pickRoute 失败: %v", err)
	}
	if r.Host != "local" {
		t.Fatalf("账本缺席时行为必须与改动前一致（旧公式 rr=0 选 local），实得 %s", r.Host)
	}
	// rr=1（奇数轮）：x3 微倾向 +2 ⇒ x3 5 / local 3 ⇒ 旧行为选 x3（证明打分仍敏感）
	g.roundRobin["m"] = 1
	r2, err := g.pickRoute("m", "", "")
	if err != nil {
		t.Fatalf("pickRoute 失败: %v", err)
	}
	if r2.Host != "x3" {
		t.Fatalf("账本缺席时行为必须与改动前一致（旧公式 rr=1 选 x3），实得 %s", r2.Host)
	}
}

// ⑤ 账本可用 → 打分确实受影响（可断言的具体差异）：x3 确定装不下 → 即使轮询/健康全占优也不选它。
func TestPickRoute_LedgerNoFitFlipsChoice(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"m": {
				{Host: "local", File: "/models/local-m.gguf", MemGb: 22},
				{Host: "x3", File: "/models/x3-m.gguf", MemGb: 22},
			},
		},
		Fleet: map[string]config.FleetNode{"x3": {Host: "<worker-ip>", Port: 8100}},
	}
	st := store.NewStore()
	// local：统一内存平台（显存即内存）+ 内存充裕 → 账本在、不降权
	st.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "local", Healthy: true, MemTotalGb: 64, MemAvailableGb: 40, VramUnified: true,
	})
	// x3：有驻留账本且"可用 4G + 可腾退 2G = 6G < 22G" → 确定装不下
	st.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine: "x3", Healthy: true, MemTotalGb: 128, MemAvailableGb: 4,
		Resident: []resources.ResidentEntry{resident("ours", 2, 10, true, 0)},
	})
	g := &Gateway{config: cfg, store: st, roundRobin: map[string]int{}, failCounts: map[string]int{}, failSince: map[string]time.Time{}}

	if d, why := g.ledgerAdjust(st.GetSnapshot("x3"), cfg.Models["m"][1]); d != ledgerNoFitPenalty {
		t.Fatalf("x3 账本应判 no_fit 降 %d，实得 %d (%s)", ledgerNoFitPenalty, d, why)
	}
	// rr=1（本来是 x3 占优的那一轮）：现在 x3 = 3 - 6 = -3 < local 3 → 必须选 local
	g.roundRobin["m"] = 1
	r, err := g.pickRoute("m", "", "")
	if err != nil {
		t.Fatalf("pickRoute 失败: %v", err)
	}
	if r.Host != "local" {
		t.Fatalf("账本判 x3 装不下时应改选 local（账本可用则打分受影响），实得 %s", r.Host)
	}
}

// ⑤ 既有单槽降权语义未被破坏（回归钉住）：忙机器（active>=1，单槽）被 -8。
func TestPickRoute_SingleSlotDegradeStillApplies(t *testing.T) {
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"m": {
				{Host: "busy", File: "/models/busy.gguf", MemGb: 22},
				{Host: "idle", File: "/models/idle.gguf", MemGb: 22},
			},
		},
		Fleet: map[string]config.FleetNode{},
	}
	st := store.NewStore()
	st.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "busy", Healthy: true, MemTotalGb: 128, MemAvailableGb: 100, ActiveRequests: 1})
	st.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "idle", Healthy: true, MemTotalGb: 128, MemAvailableGb: 100, ActiveRequests: 0})
	g := &Gateway{config: cfg, store: st, roundRobin: map[string]int{}, failCounts: map[string]int{}, failSince: map[string]time.Time{}}

	r, err := g.pickRoute("m", "", "")
	if err != nil {
		t.Fatalf("pickRoute 失败: %v", err)
	}
	if r.Host != "idle" {
		t.Fatalf("单槽忙机器（active>=1）应被降权 -8，选空闲机；实得 %s", r.Host)
	}
}
