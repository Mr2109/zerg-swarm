package resources

import "testing"

const gib = int64(1) << 30

// realQuery 返回一组"全部真值"的估算入参：KV 三参齐全 → 不应标 estimated。
// 其 KV = 2×32×8×128×2×8192 = 1 GiB；权重 4 GiB；开销 2 GiB ⇒ need = 7 GiB。
func realQuery() FitQuery {
	return FitQuery{
		Model:        "test-model",
		Ctx:          8192,
		WeightsBytes: 4 * gib,
		NLayer:       32,
		NKvHeads:     8,
		HeadDim:      128,
		BytesPerElem: 2,
		ArchFamily:   "llama",
	}
}

// realMachine 返回一台提供引擎开销的机器（统一内存：显存即内存，§3.1）。
func realMachine(memAvailGb float64) MachineLedger {
	return MachineLedger{Machine: "local", MemTotalGb: 64, MemAvailGb: memAvailGb, EngineOverheadGb: 2, UnifiedMemory: true}
}

// evictFixture 返回一台"现在装不下、驱逐一（够用的）后装得下"的机器：
// need 7 GiB；freeNow 3 GiB；仅 a 可腾退 4 GiB；b 在飞、c pin 未到期。（统一内存机器）
func evictFixture() MachineLedger {
	return MachineLedger{
		Machine: "local", MemTotalGb: 32, MemAvailGb: 3, EngineOverheadGb: 2, UnifiedMemory: true,
		Resident: []ResidentEntry{
			{Digest: "sha256-a", State: StateReady, Managed: true, MemGb: 4, WeightsBytes: 4 * gib, LastUsedAgoS: 100},
			{Digest: "sha256-b", State: StateReady, Managed: true, MemGb: 4, ReqCount: 2},
			{Digest: "sha256-c", State: StateReady, Managed: true, MemGb: 4, Pinned: true, PinRemainS: 60},
		},
	}
}

// TestEstimateFit_Table —— "跑得动吗"全场景（含反例）：真值/回退/fail-closed/显存/边界/驱逐。
func TestEstimateFit_Table(t *testing.T) {
	cases := []struct {
		name          string
		mut           func(*FitQuery)
		machine       MachineLedger
		wantVerdict   string
		wantEstimated bool
		wantNeed      int64    // 0 = 不校验
		wantKv        int64    // 0 = 不校验
		wantPlan      []string // nil = 不校验；非 nil 时须逐项相等
	}{
		{"真值-装得下-estimated=false", func(q *FitQuery) {}, realMachine(8), VerdictFit, false, 7 * gib, 1 * gib, nil},
		{"缺KV三参-按族回退且标估", func(q *FitQuery) { q.NKvHeads = 0; q.HeadDim = 0; q.BytesPerElem = 0 }, realMachine(8), VerdictFit, true, 7 * gib, 1 * gib, nil},
		{"缺权重-fail-closed", func(q *FitQuery) { q.WeightsBytes = 0 }, realMachine(64), VerdictNoFit, true, 0, 0, nil},
		{"缺上下文-fail-closed", func(q *FitQuery) { q.Ctx = 0 }, realMachine(64), VerdictNoFit, true, 0, 0, nil},
		{"缺层数-fail-closed", func(q *FitQuery) { q.NLayer = 0 }, realMachine(64), VerdictNoFit, true, 0, 0, nil},
		{"KV缺失且架构族未登记-fail-closed", func(q *FitQuery) { q.NKvHeads = 0; q.HeadDim = 0; q.ArchFamily = "unknown-fam" }, realMachine(64), VerdictNoFit, true, 0, 0, nil},
		{"显存不足-no_fit(内存够)", func(q *FitQuery) {}, MachineLedger{Machine: "x3", MemTotalGb: 128, MemAvailGb: 64, VramTotalGb: 24, VramFreeGb: 4, EngineOverheadGb: 2}, VerdictNoFit, false, 7 * gib, 1 * gib, nil},
		{"显存未知且非统一内存-fail-closed(内存够)", func(q *FitQuery) {}, MachineLedger{Machine: "x3", MemTotalGb: 128, MemAvailGb: 120, EngineOverheadGb: 2}, VerdictNoFit, true, 7 * gib, 1 * gib, nil},
		{"刚好边界-fit", func(q *FitQuery) {}, realMachine(7), VerdictFit, false, 7 * gib, 0, nil},
		{"差1GiB-no_fit", func(q *FitQuery) {}, realMachine(6), VerdictNoFit, false, 0, 0, nil},
		{"现在装不下-驱逐够用即evict(计划有序、排除在飞/pin)", func(q *FitQuery) {}, evictFixture(), VerdictEvict, false, 7 * gib, 1 * gib, []string{"sha256-a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := realQuery()
			c.mut(&q)
			est := EstimateFit(q, c.machine)
			if est.Verdict != c.wantVerdict {
				t.Fatalf("verdict want %s got %s（basis=%s）", c.wantVerdict, est.Verdict, est.Basis)
			}
			if est.Estimated != c.wantEstimated {
				t.Fatalf("estimated want %v got %v（basis=%s）", c.wantEstimated, est.Estimated, est.Basis)
			}
			if c.wantNeed != 0 && est.NeedBytes != c.wantNeed {
				t.Fatalf("need want %d got %d", c.wantNeed, est.NeedBytes)
			}
			if c.wantKv != 0 && est.KvCacheBytes != c.wantKv {
				t.Fatalf("kv want %d got %d", c.wantKv, est.KvCacheBytes)
			}
			if c.wantPlan != nil {
				if len(est.EvictPlan) != len(c.wantPlan) {
					t.Fatalf("evict_plan want %v got %v", c.wantPlan, est.EvictPlan)
				}
				for i := range c.wantPlan {
					if est.EvictPlan[i] != c.wantPlan[i] {
						t.Fatalf("evict_plan want %v got %v", c.wantPlan, est.EvictPlan)
					}
				}
			}
			if est.Basis == "" {
				t.Fatal("basis 不得为空（须可解释）")
			}
		})
	}
}

// TestEstimateFit_Helpers —— 方法便捷入口、Fits、族枚举（纯函数辅助）。
func TestEstimateFit_Helpers(t *testing.T) {
	q := realQuery()
	m := realMachine(8)
	a, b := m.EstimateFit(q), EstimateFit(q, m)
	if a.Verdict != b.Verdict || a.NeedBytes != b.NeedBytes || a.KvCacheBytes != b.KvCacheBytes || a.Estimated != b.Estimated {
		t.Fatalf("方法便捷入口应与包函式一致: %+v vs %+v", a, b)
	}
	if !EstimateFit(q, m).Fits() {
		t.Fatal("Fits() 应与 verdict=fit 一致")
	}
	if len(SupportedArchFamilies()) == 0 {
		t.Fatal("架构族表不得为空")
	}
	if _, ok := ArchFallback("llama"); !ok {
		t.Fatal("llama 应已登记回退常量")
	}
	if _, ok := ArchFallback("no-such-family"); ok {
		t.Fatal("未登记族不应命中")
	}
}

// TestEstimateFit_EstimatesActualBytes —— 回退常量确实进入了 KV 数值（不是摆设）。
func TestEstimateFit_EstimatesActualBytes(t *testing.T) {
	q := realQuery()
	q.NKvHeads, q.HeadDim, q.BytesPerElem = 0, 0, 0
	fb, ok := ArchFallback("llama")
	if !ok {
		t.Fatal("llama 应已登记")
	}
	want := int64(2.0 * float64(q.NLayer) * float64(fb.KVHeads) * float64(fb.HeadDim) * DefaultBytesPerElem * float64(q.Ctx))
	if got := EstimateFit(q, realMachine(8)).KvCacheBytes; got != want {
		t.Fatalf("回退 KV 应为 %d，实得 %d", want, got)
	}
}

// TestEstimateFit_VramStrictest —— 显存三态下的"二者取严"（§3.3d，反例优先）：
//
//	(a) 内存够 + 显存够 → fit；
//	(b) 内存够 + 显存不够 → no_fit（显存独立生效，不得只看内存）；
//	(c) 内存够 + 显存未知且非统一内存 → no_fit 且 estimated=true（fail-closed，不得默默放行）；
//	(d) 内存够 + 统一内存（显存即内存）→ fit，且 estimated=false（这是"知道"，不是"猜"）。
func TestEstimateFit_VramStrictest(t *testing.T) {
	q := realQuery() // need = 7 GiB
	gpu := func(free float64) MachineLedger {
		return MachineLedger{Machine: "x3", MemTotalGb: 128, MemAvailGb: 64, VramTotalGb: 24, VramFreeGb: free, EngineOverheadGb: 2}
	}
	cases := []struct {
		name          string
		machine       MachineLedger
		wantVerdict   string
		wantEstimated bool
	}{
		{"内存够+显存够-fit", gpu(32), VerdictFit, false},
		{"内存够+显存不够-no_fit", gpu(4), VerdictNoFit, false},
		{"内存够+显存未知(非统一内存)-fail-closed", MachineLedger{Machine: "x3", MemTotalGb: 128, MemAvailGb: 64, EngineOverheadGb: 2}, VerdictNoFit, true},
		{"内存够+统一内存-按内存口径判", MachineLedger{Machine: "local", MemTotalGb: 64, MemAvailGb: 64, EngineOverheadGb: 2, UnifiedMemory: true}, VerdictFit, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			est := EstimateFit(q, c.machine)
			if est.Verdict != c.wantVerdict {
				t.Fatalf("verdict want %s got %s（basis=%s）", c.wantVerdict, est.Verdict, est.Basis)
			}
			if est.Estimated != c.wantEstimated {
				t.Fatalf("estimated want %v got %v（basis=%s）", c.wantEstimated, est.Estimated, est.Basis)
			}
			if est.Basis == "" {
				t.Fatal("basis 不得为空（须可解释）")
			}
		})
	}
	// 反例守卫：统一内存机器不得被当成"显存未知"而 fail-closed
	if got := EstimateFit(q, MachineLedger{Machine: "local", MemTotalGb: 64, MemAvailGb: 64, EngineOverheadGb: 2, UnifiedMemory: true}).Verdict; got != VerdictFit {
		t.Fatalf("统一内存（显存即内存）不得判 no_fit，实得 %s", got)
	}
}

// TestEstimateFit_VramEvictPlanUsesStrictestDeficit —— 显存受限时的"驱逐可跑"必须给出**够用的**计划：
// 内存充裕、显存不够 → verdict=evict 且计划点名能让位的驻留（而不是负缺口/空计划）。
func TestEstimateFit_VramEvictPlanUsesStrictestDeficit(t *testing.T) {
	m := MachineLedger{
		Machine: "x3", MemTotalGb: 128, MemAvailGb: 100, EngineOverheadGb: 2,
		VramTotalGb: 24, VramFreeGb: 4, // 显存只剩 4 GiB，need 7 GiB
		Resident: []ResidentEntry{{Digest: "sha256-a", State: StateReady, Managed: true, MemGb: 8, LastUsedAgoS: 100}},
	}
	est := EstimateFit(realQuery(), m)
	if est.Verdict != VerdictEvict {
		t.Fatalf("内存够、显存不够但腾退后够 → 应判 evict，实得 %s（basis=%s）", est.Verdict, est.Basis)
	}
	if len(est.EvictPlan) != 1 || est.EvictPlan[0] != "sha256-a" {
		t.Fatalf("显存缺口应由腾退该驻留补上（计划=[sha256-a]），实得 %v（basis=%s）", est.EvictPlan, est.Basis)
	}
}
