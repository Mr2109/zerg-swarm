// sampling_test.go — T6.4 的用例（判定层①–⑤、⑩）。
//
// 用例清单（编号 ↔ 任务 T6.4 的十项要求；每条自带**反例或对照**——只断"该发生的发生了"会漏掉
// "什么都没发生也照样绿"这种假绿，本仓库的用例纪律）：
//
//	① TestMustKeepSurvivesDrop          must_keep 必留（即使命中 drop）· dry-run 下必留召回 100% ·
//	                                    显式开关下 drop 可否决**且否决必被记录**
//	② TestDropVetoesProbabilistic       drop 定案 ⇒ 概率采样不参与（哪怕抽签必中）；阈值/权值缺席不写 0
//	③ TestPrecedenceMustKeepBeatsAll    写死序：MUSTKEEP 压一切；matched_policies 按序输出（与传入顺序无关）
//	④ TestDefaultDeny                   无任何策略命中 ⇒ **默认拒绝**（不是默认全采）；dry-run 下判词与执行分离
//	⑤ TestProbabilisticDeterministic    确定性哈希（换实例/重复调用同结果）；换盐必翻转；分布均匀
//	⑩ TestSamplingOffLosesNothing       全采模式一条不丢（对照：同策略集正常模式下全丢）
package sampling

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── 公共夹具 ─────────────────────────────────────────────────────────────

// baseEvent — 一条普通事件（RunID/Session 固定，抽签的盐就来自它们）。
func baseEvent(key string) Event {
	return Event{Key: key, Name: "turn", Session: "s1", RunID: "r1", Bytes: 100, Attrs: map[string]string{}}
}

// errEvent — 带 err=1 属性的事件（必留类判据的载体）。
func errEvent(key string) Event {
	e := baseEvent(key)
	e.Attrs["err"] = "1"
	return e
}

func hasName(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func eqNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// fakeClock — 冻结时钟（本包只从 Config.Clock 取时间 ⇒ 宽限期/窗口都可判定，用例不抖）。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// capSink — 记账用的落盘出口（"落盘条数"就是它的条数）。
type capSink struct {
	mu  sync.Mutex
	evs []Event
	err error // 非 nil ⇒ 每次都失败（用来钉"落盘失败也必须留碑"）
}

func (s *capSink) sink(ev Event) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, ev)
	return nil
}

func (s *capSink) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.evs)
}

func (s *capSink) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.evs))
	for _, ev := range s.evs {
		out = append(out, ev.Key)
	}
	return out
}

// ── ① must_keep 必留（即使命中 drop）──────────────────────────────────────

func TestMustKeepSurvivesDrop(t *testing.T) {
	policies := []Policy{
		NewMustKeep("err_keep", "本 run 以错误收尾", MatchAttr("err", "1")),
		NewDrop("noise", "噪声全丢", Always()),
	}

	// (a) 默认：必留压 drop，且"drop 命中"这件事**不许被吞掉**（进 matched_policies）
	s := NewSampler(Config{DryRun: true}, policies...)
	d := s.Decide(errEvent("k1"))
	if d.Verdict != VerdictKeep {
		t.Fatalf("必留命中却判 %s（C1：MUSTKEEP 压一切）", d.Verdict)
	}
	if d.DecidingKind != KindMustKeep || d.DecidingPolicy != "err_keep" {
		t.Fatalf("定案者应为 err_keep/must_keep，实际 %s/%s", d.DecidingPolicy, d.DecidingKind)
	}
	if !hasName(d.MatchedPolicies, "noise") {
		t.Fatalf("命中的 drop 必须进 matched_policies（否则无法取证），实际 %v", d.MatchedPolicies)
	}
	if d.OverriddenMustKeep {
		t.Fatal("默认开关下不得出现否决（OverriddenMustKeep 只属于显式开关）")
	}
	if !strings.Contains(d.Reason, "不得否决必留") {
		t.Fatalf("判词理由应写明 drop 命中但不得否决，实际 %q", d.Reason)
	}
	if d.EffectiveVerdict() != VerdictKeep {
		t.Fatalf("dry-run 下必留的执行判词也必须是 keep，实际 %s", d.EffectiveVerdict())
	}

	// (b) 显式开关：drop 可否决 —— 但**否决必须被记录**
	s2 := NewSampler(Config{DryRun: true, DropVetoesMustKeep: true}, policies...)
	d2 := s2.Decide(errEvent("k1"))
	if d2.Verdict != VerdictDrop || d2.DecidingPolicy != "noise" {
		t.Fatalf("显式开关下应被 drop 否决，实际 %s/%s", d2.Verdict, d2.DecidingPolicy)
	}
	if !d2.OverriddenMustKeep {
		t.Fatal("显式否决必须置 OverriddenMustKeep（C1：记录该否决）")
	}
	if !hasName(d2.MatchedPolicies, "err_keep") {
		t.Fatalf("被否决的必留也必须留在 matched_policies 里，实际 %v", d2.MatchedPolicies)
	}

	// (c) 账上看得见：正常模式下否决 ⇒ 墓碑带 overridden_must_keep
	cap := &capSink{}
	r := New(Config{DropVetoesMustKeep: true, Sink: cap.sink}, policies...)
	rd := r.Admit(errEvent("k1"))
	if rd.Verdict != VerdictDrop || !rd.Tombstone {
		t.Fatalf("被否决的必留应缺席并留碑，实际 verdict=%s tombstone=%v", rd.Verdict, rd.Tombstone)
	}
	tomb, ok := r.Ledger("r1").TombstoneFor("k1")
	if !ok {
		t.Fatal("否决没落账 ⇒ 无法取证（C5）")
	}
	if !tomb.OverriddenMustKeep || tomb.DecidingPolicy != "noise" {
		t.Fatalf("墓碑应记录否决与定案者，实际 overridden=%v deciding=%s", tomb.OverriddenMustKeep, tomb.DecidingPolicy)
	}
	if cap.n() != 0 {
		t.Fatalf("被否决者不得落盘，实际落了 %d 条", cap.n())
	}

	// (d) dry-run 下的必须召回 = 100%（任务表 T6.4 的验收口径）
	cap2 := &capSink{}
	r2 := New(Config{DryRun: true, Sink: cap2.sink}, policies...)
	const N = 100
	for i := 0; i < N; i++ {
		r2.Admit(errEvent(fmt.Sprintf("e%d", i)))
	}
	hit := 0
	for _, rec := range r2.Ledger("r1").Records() {
		if rec.DecidingPolicy == "err_keep" && rec.Decision == string(VerdictKeep) {
			hit++
		}
	}
	if hit != N {
		t.Fatalf("dry-run 下必留召回 %d/%d（必须 100%%）", hit, N)
	}
	if cap2.n() != N {
		t.Fatalf("dry-run 下落盘条数应为全量 %d，实际 %d", N, cap2.n())
	}
}

// ── ② drop 否决概率采样 ──────────────────────────────────────────────────

func TestDropVetoesProbabilistic(t *testing.T) {
	prob := NewProbabilistic("p_all", "全量概率采样", 1.0, Always()) // rate=1 ⇒ 抽签必中（weight ∈ [0,1) < 1）
	drop := NewDrop("noise", "噪声全丢", Always())

	s := NewSampler(Config{DryRun: true}, prob, drop)
	d := s.Decide(baseEvent("k"))
	if d.Verdict != VerdictDrop || d.DecidingKind != KindDrop || d.DecidingPolicy != "noise" {
		t.Fatalf("drop 命中应直接定案，实际 %s/%s/%s", d.Verdict, d.DecidingKind, d.DecidingPolicy)
	}
	if d.Threshold != nil || d.Weight != nil {
		t.Fatalf("drop 定案 ⇒ 阈值/权值**不适用**，必须缺席（不许写 0 顶替）：threshold=%v weight=%v", d.Threshold, d.Weight)
	}
	if !hasName(d.MatchedPolicies, "p_all") {
		t.Fatalf("命中判据的概率策略仍要进 matched_policies，实际 %v", d.MatchedPolicies)
	}

	// 对照（防"概率根本没生效"的假绿）：抽掉 drop ⇒ 同一事件被概率采样留住
	s2 := NewSampler(Config{DryRun: true}, prob)
	d2 := s2.Decide(baseEvent("k"))
	if d2.Verdict != VerdictKeep || d2.DecidingKind != KindProbabilistic {
		t.Fatalf("对照应被概率采样留住，实际 %s/%s", d2.Verdict, d2.DecidingKind)
	}
	if d2.Threshold == nil || d2.Weight == nil {
		t.Fatal("概率定案必须落 threshold/weight（'凭什么丢的'必须可核）")
	}
	if *d2.Threshold != 1.0 || !(*d2.Weight < 1.0) {
		t.Fatalf("阈值/权值不合口径：threshold=%v weight=%v", *d2.Threshold, *d2.Weight)
	}

	// 另一面：rate=0 ⇒ 永不中签（不是"默认全采"）
	s3 := NewSampler(Config{DryRun: true}, NewProbabilistic("p_zero", "零采样率", 0, Always()))
	for i := 0; i < 20; i++ {
		if d3 := s3.Decide(baseEvent(fmt.Sprintf("k%d", i))); d3.Verdict != VerdictDrop {
			t.Fatalf("rate=0 竟判 %s（应永不中签）", d3.Verdict)
		}
	}
}

// ── ③ 优先级序：MUSTKEEP 压一切；账按写死序 ───────────────────────────────

func TestPrecedenceMustKeepBeatsAll(t *testing.T) {
	// 写死序本身（删掉任何一档 ⇒ 本断言红）
	want := []PolicyKind{KindMustKeep, KindDrop, KindProbabilistic, KindDefaultDeny}
	got := PolicyPrecedence
	if len(got) != len(want) {
		t.Fatalf("判定序长度应为 %d，实际 %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("判定序第 %d 位应为 %s，实际 %s（C1：写死序）", i, want[i], got[i])
		}
	}

	// **故意倒序传入**（概率 → drop → 必留）：账里必须按写死序输出
	policies := []Policy{
		NewProbabilistic("p_zero", "永不中签", 0, Always()),
		NewDrop("noise", "噪声全丢", Always()),
		NewMustKeep("err_keep", "出错必留", Always()),
	}
	s := NewSampler(Config{DryRun: true}, policies...)
	d := s.Decide(errEvent("k"))
	if d.Verdict != VerdictKeep || d.DecidingPolicy != "err_keep" {
		t.Fatalf("MUSTKEEP 应压一切，实际 %s/%s", d.Verdict, d.DecidingPolicy)
	}
	if !eqNames(d.MatchedPolicies, []string{"err_keep", "noise", "p_zero"}) {
		t.Fatalf("matched_policies 必须按写死序（与传入顺序无关），实际 %v", d.MatchedPolicies)
	}

	// 逐档压下一档（造"只命中一档"的事件）
	chain := []Policy{
		NewMustKeep("mk", "必留", MatchAttr("err", "1")),
		NewDrop("dp", "丢弃", MatchAttr("noisy", "1")),
		NewProbabilistic("pb", "概率", 1.0, Always()),
	}
	sc := NewSampler(Config{DryRun: true}, chain...)
	e1 := errEvent("k1")
	if d1 := sc.Decide(e1); d1.Verdict != VerdictKeep || d1.DecidingKind != KindMustKeep {
		t.Fatalf("①档应被必留定案，实际 %s/%s", d1.Verdict, d1.DecidingKind)
	}
	e2 := baseEvent("k2")
	e2.Attrs["noisy"] = "1"
	if d2 := sc.Decide(e2); d2.Verdict != VerdictDrop || d2.DecidingKind != KindDrop {
		t.Fatalf("②档应被 drop 定案，实际 %s/%s", d2.Verdict, d2.DecidingKind)
	}
	if d3 := sc.Decide(baseEvent("k3")); d3.Verdict != VerdictKeep || d3.DecidingKind != KindProbabilistic {
		t.Fatalf("③档应被概率定案，实际 %s/%s", d3.Verdict, d3.DecidingKind)
	}
	// ④档留到用例④
}

// ── ④ 默认拒绝 ───────────────────────────────────────────────────────────

func TestDefaultDeny(t *testing.T) {
	// 只有"带 err 才命中"的策略 ⇒ 普通事件什么都不命中
	s := NewSampler(Config{}, NewMustKeep("err_keep", "出错必留", MatchAttr("err", "1")))
	d := s.Decide(baseEvent("k"))
	if d.Verdict != VerdictDrop || d.DecidingKind != KindDefaultDeny || d.DecidingPolicy != PolicyDefaultDeny {
		t.Fatalf("无命中应默认拒绝，实际 %s/%s/%s", d.Verdict, d.DecidingKind, d.DecidingPolicy)
	}
	if d.Threshold != nil || d.Weight != nil {
		t.Fatalf("默认拒绝没有判词依据 ⇒ threshold/weight 必须缺席：%v/%v", d.Threshold, d.Weight)
	}
	if len(d.MatchedPolicies) != 0 {
		t.Fatalf("默认拒绝时 matched_policies 应为空，实际 %v", d.MatchedPolicies)
	}
	// 空策略集同样默认拒绝（**不是**"没策略就全采"）
	if d0 := NewSampler(Config{}).Decide(baseEvent("k")); d0.DecidingPolicy != PolicyDefaultDeny {
		t.Fatalf("空策略集应默认拒绝，实际 %s", d0.DecidingPolicy)
	}
	// Match=nil 的"忘了写判据"策略也是永不命中（fail-closed 方向，不是全放行）
	if dn := NewSampler(Config{}, Policy{Name: "half_written", Kind: KindMustKeep}).Decide(baseEvent("k")); dn.Verdict != VerdictDrop {
		t.Fatalf("Match=nil 必须永不命中（不是全命中），实际 %s", dn.Verdict)
	}

	// dry-run 下判词与执行**分开**：判词是 drop，执行是全留 —— 混了就等于 dry-run 没意义
	s2 := NewSampler(Config{DryRun: true}, NewMustKeep("err_keep", "出错必留", MatchAttr("err", "1")))
	d2 := s2.Decide(baseEvent("k"))
	if d2.Verdict != VerdictDrop {
		t.Fatalf("dry-run 下判词照记（应为 drop），实际 %s", d2.Verdict)
	}
	if d2.EffectiveVerdict() != VerdictKeep {
		t.Fatalf("dry-run 下执行判词必须是 keep，实际 %s", d2.EffectiveVerdict())
	}
	if !d2.DryRun {
		t.Fatal("dry-run 标记必须落进判定结果（读侧靠它分辨'判了 drop'与'真丢了'）")
	}
}

// ── ⑤ 概率采样确定性 + 分布均匀 ──────────────────────────────────────────

func TestProbabilisticDeterministic(t *testing.T) {
	const (
		N    = 20000
		rate = 0.25
		want = N * rate
	)
	key := func(i int) string { return fmt.Sprintf("k%d", i) }
	salt := "s1\x00r1"

	// (a) 同一 (盐, 身份) 重复调用结果一致；值域 [0,1)
	w0 := drawWeight(salt, key(0))
	for i := 0; i < 100; i++ {
		if got := drawWeight(salt, key(0)); got != w0 {
			t.Fatalf("同输入抽签应稳定：第 %d 次 %v ≠ 首次 %v", i, got, w0)
		}
	}
	hits := 0
	for i := 0; i < N; i++ {
		w := drawWeight(salt, key(i))
		if w < 0 || w >= 1 {
			t.Fatalf("抽签值越界：%v", w)
		}
		if w < rate {
			hits++
		}
	}
	// (b) 分布均匀：N=20000、p=0.25 的 3σ ≈ 0.0092 ⇒ 容差 0.02（约 6.5σ，不是靠运气）
	if diff := float64(hits) - want; diff < -0.02*N || diff > 0.02*N {
		t.Fatalf("分布不均：命中 %d，期望 %.0f（容差 ±%d）", hits, want, int(0.02*N))
	}

	// (c) 盐**必须**进哈希：换盐后至少翻转一条判词（否则"盐"是摆设，会话间无法独立抽样）
	flips := 0
	for i := 0; i < 2000; i++ {
		if (drawWeight("saltA\x00s1", key(i)) < rate) != (drawWeight("saltB\x00s1", key(i)) < rate) {
			flips++
		}
	}
	if flips == 0 {
		t.Fatal("换盐一条都没翻转 ⇒ 盐没进哈希输入")
	}

	// (d) 端到端：**两个独立实例**对同一批事件判词完全一致（rand 实现会在这里露馅）
	cfg := Config{DryRun: true, Salt: "Z", DiscardGrace: time.Minute}
	sa := NewSampler(cfg, NewProbabilistic("p", "四分之一采样", rate, Always()))
	sb := NewSampler(cfg, NewProbabilistic("p", "四分之一采样", rate, Always()))
	kept := 0
	for i := 0; i < 2000; i++ {
		da, db := sa.Decide(baseEvent(key(i))), sb.Decide(baseEvent(key(i)))
		if da.Verdict != db.Verdict {
			t.Fatalf("两个同配置实例判词不一致（确定性哈希不成立）：%s vs %s", da.Verdict, db.Verdict)
		}
		if da.Weight == nil || db.Weight == nil || *da.Weight != *db.Weight {
			t.Fatalf("权值应可复算：%v vs %v", da.Weight, db.Weight)
		}
		if da.Verdict == VerdictKeep {
			kept++
		}
	}
	if kept == 0 || kept == 2000 {
		t.Fatalf("端到端采样退化成全留或全丢：kept=%d/2000", kept)
	}
	// 反复调同一条 ⇒ 判词与权值稳定
	if d1, d2 := sa.Decide(baseEvent("stable")), sa.Decide(baseEvent("stable")); d1.Verdict != d2.Verdict || *d1.Weight != *d2.Weight {
		t.Fatal("同一条事件重复判定必须完全一致（账要可复算）")
	}
}

// ── ⑩ 反例：关掉采样（全采模式）⇒ 一条不丢 ────────────────────────────────

func TestSamplingOffLosesNothing(t *testing.T) {
	policies := []Policy{NewDrop("noise", "噪声全丢", Always())}
	const N = 20

	// 对照：正常模式下同一策略集**全丢**（证明"没丢"不是因为策略没生效）
	capA := &capSink{}
	rA := New(Config{Sink: capA.sink}, policies...)
	for i := 0; i < N; i++ {
		rA.Admit(baseEvent(fmt.Sprintf("a%d", i)))
	}
	if capA.n() != 0 {
		t.Fatalf("对照组应全丢，实际落了 %d 条", capA.n())
	}
	if rA.Ledger("r1").TombstoneCount() != N {
		t.Fatalf("对照组应留 %d 块墓碑，实际 %d", N, rA.Ledger("r1").TombstoneCount())
	}

	// 全采模式：一条不丢
	capB := &capSink{}
	rB := New(Config{SamplingOff: true, Sink: capB.sink}, policies...)
	for i := 0; i < N; i++ {
		d := rB.Admit(baseEvent(fmt.Sprintf("b%d", i)))
		if d.Verdict != VerdictKeep || d.DecidingKind != KindOff || d.DecidingPolicy != PolicyOff {
			t.Fatalf("全采模式应判 keep/off，实际 %s/%s/%s", d.Verdict, d.DecidingKind, d.DecidingPolicy)
		}
		if !d.Emitted || d.Tombstone {
			t.Fatalf("全采模式必须落盘且无墓碑，实际 emitted=%v tombstone=%v", d.Emitted, d.Tombstone)
		}
	}
	if capB.n() != N {
		t.Fatalf("全采模式落盘条数应为 %d，实际 %d", N, capB.n())
	}
	led := rB.Ledger("r1")
	if led.Emitted() != N || led.TombstoneCount() != 0 || led.Count() != N {
		t.Fatalf("全采模式账目应全留：emitted=%d tombstone=%d count=%d", led.Emitted(), led.TombstoneCount(), led.Count())
	}
	if rB.ProbationCount() != 0 {
		t.Fatalf("全采模式不该有任何未决条目，实际 %d", rB.ProbationCount())
	}
}
