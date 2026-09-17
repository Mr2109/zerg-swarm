// ring_test.go — T6.4 的用例（缓冲层⑥–⑨）。
//
// 用例清单（每条自带**反例或对照**）：
//
//	⑥ TestProbationOverturn                宽限期内晚到的判词能翻案（append-only 追加 amendment）；
//	                                       恰等于到期时刻仍有效；过期即翻不动；重复翻案必失败
//	⑦ TestGateEvictsByPrecedenceNeverBlocks 字节/速率超限 ⇒ 按优先级淘汰且**绝不阻塞**（三路：条数闸/字节闸/
//	                                       必留破例 + sink 报满时立刻返回 + 容量淘汰遵守淘汰序）
//	⑧ TestDryRunEmitsEverything            dry-run ⇒ 落盘条数 = 全量（判词照记、tombstone 恒 false）
//	⑨ TestTombstoneForDropped              被裁者有墓碑（让"缺席"不再歧义）；留下的 key **不许**有墓碑
package sampling

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// ── ⑥ probation：宽限期内可翻案 ──────────────────────────────────────────

func TestProbationOverturn(t *testing.T) {
	clk := newFakeClock()
	cap := &capSink{}
	r := New(Config{DiscardGrace: 10 * time.Minute, Clock: clk.now, Sink: cap.sink},
		NewDrop("noise", "噪声全丢", Always()))

	d := r.Admit(baseEvent("k1"))
	if d.Verdict != VerdictDrop || !d.Tombstone || d.Emitted {
		t.Fatalf("判丢应留碑且不落盘，实际 verdict=%s tombstone=%v emitted=%v", d.Verdict, d.Tombstone, d.Emitted)
	}
	if cap.n() != 0 || r.ProbationCount() != 1 {
		t.Fatalf("被裁者不该落盘（实落 %d 条），但必须进 probation（实际 %d 条）", cap.n(), r.ProbationCount())
	}

	// 墓碑**立刻**在账上，且带宽限截止（读侧据此判断"还有没有翻案窗口"）
	tomb, ok := r.Ledger("r1").TombstoneFor("k1")
	if !ok {
		t.Fatal("被裁者必须立刻留碑（C5）")
	}
	if tomb.Decision != string(VerdictDrop) || tomb.DecidingPolicy != "noise" || tomb.Tombstone != true {
		t.Fatalf("墓碑内容不合口径：%+v", tomb)
	}
	until := tomb.ProbationUntil
	if until == "" {
		t.Fatal("判丢的墓碑必须带 probation_until（否则'还能不能翻案'无从判断）")
	}
	want := clk.now().Add(10 * time.Minute).Format(time.RFC3339Nano)
	if until != want {
		t.Fatalf("宽限截止应为 %s，实际 %s", want, until)
	}
	if pu, ok := r.ProbationUntil("r1", "k1"); !ok || !pu.Equal(clk.now().Add(10*time.Minute)) {
		t.Fatalf("probation 截止时刻不对：%v %v", pu, ok)
	}

	// 宽限期内翻案 ⇒ 改判为留并真的交付
	if !r.Overturn("r1", "k1", "晚到的判词：本 run 以错误收尾 ⇒ 必留") {
		t.Fatal("宽限期内必须能翻案")
	}
	if cap.n() != 1 || cap.keys()[0] != "k1" {
		t.Fatalf("翻案后必须真的落盘，实际 %v", cap.keys())
	}
	if r.Overturns() != 1 || r.ProbationCount() != 0 {
		t.Fatalf("翻案计数/未决数不对：overturns=%d probation=%d", r.Overturns(), r.ProbationCount())
	}
	led := r.Ledger("r1")
	ams := led.Amendments()
	if len(ams) != 1 || ams[0].FromState != StateProbation || ams[0].ToState != StateOverturned {
		t.Fatalf("翻案必须**追加**一条 probation→overturned 的 amendment，实际 %+v", ams)
	}
	if led.Emitted() != 1 {
		t.Fatalf("账上的落盘条数应为 1，实际 %d", led.Emitted())
	}
	// append-only：旧墓碑行还在（不许回头涂改行）
	if _, ok := led.TombstoneFor("k1"); !ok {
		t.Fatal("旧行不许被改掉（append-only）")
	}

	// 对照①：同一 key 再翻 ⇒ false（翻案失败必须可判定，不许"看起来翻了"）
	if r.Overturn("r1", "k1", "重复翻案") {
		t.Fatal("重复翻案必须失败")
	}
	// 对照②：边界 —— **恰等于**到期时刻仍算宽限内；越过一秒就翻不动
	r.Admit(baseEvent("k2"))
	clk.advance(10 * time.Minute)
	if !r.Overturn("r1", "k2", "压线的判词") {
		t.Fatal("恰等于到期时刻仍应算在宽限期内（边界写死）")
	}
	r.Admit(baseEvent("k3"))
	clk.advance(10*time.Minute + time.Nanosecond)
	if r.Overturn("r1", "k3", "过期后翻案") {
		t.Fatal("过了宽限期必须翻不动（否则'宽限'没有意义）")
	}
	if got := len(led.Amendments()); got != 2 {
		t.Fatalf("应有 2 条 amendment（k1、k2 各一；k3 失败不留痕），实际 %d", got)
	}
	// 过期条目被清出内存（账不改：probation_until 就是终局判据）
	if r.ProbationCount() != 0 || r.Sweep() != 0 {
		t.Fatalf("过期条目应已清出，实际 probation=%d sweep=%d", r.ProbationCount(), r.Sweep())
	}
	if cap.n() != 2 {
		t.Fatalf("落盘条数应为 2（k1/k2 翻案），实际 %d", cap.n())
	}
}

// ── ⑦ 闸与容量：按优先级淘汰，且绝不阻塞 ──────────────────────────────────

func TestGateEvictsByPrecedenceNeverBlocks(t *testing.T) {
	// (a) 条数闸：预算 2 条/s ⇒ 第 3 条起被裁（且注明是 rate 而不是别的）
	clk := newFakeClock()
	capA := &capSink{}
	keepAll := NewProbabilistic("p_all", "全量概率采样", 1.0, Always())
	rA := New(Config{RateEventsPerSec: 2, RateBytesPerSec: 1 << 20, Clock: clk.now, Sink: capA.sink}, keepAll)
	for i := 0; i < 2; i++ {
		if d := rA.Admit(baseEvent(fmt.Sprintf("g%d", i))); !d.Emitted {
			t.Fatalf("预算内应放行，第 %d 条被裁", i)
		}
	}
	d3 := rA.Admit(baseEvent("g2"))
	if d3.Verdict != VerdictDrop || !d3.GateEnforced || d3.DegradeLevel != DegradeRate {
		t.Fatalf("超限应被条数闸裁掉：verdict=%s gate=%v degrade=%s", d3.Verdict, d3.GateEnforced, d3.DegradeLevel)
	}
	if d3.DecidingPolicy != "p_all" {
		t.Fatalf("被闸覆盖时 deciding_policy 仍是'谁想留它'，实际 %s", d3.DecidingPolicy)
	}
	if !d3.Tombstone || d3.Emitted {
		t.Fatalf("被闸裁者必须留碑且不落盘：tombstone=%v emitted=%v", d3.Tombstone, d3.Emitted)
	}
	if capA.n() != 2 {
		t.Fatalf("落盘条数应恰为预算 2，实际 %d", capA.n())
	}
	if rA.ProbationCount() != 0 {
		t.Fatalf("被闸裁的不进 probation（闸的意义是立刻减负），实际 %d 条未决", rA.ProbationCount())
	}
	if rec, ok := rA.Ledger("r1").TombstoneFor("g2"); !ok || rec.ProbationUntil != "" || !rec.GateEnforced {
		t.Fatalf("闸裁的墓碑应终局（无 probation_until）且标明 gate_enforced：%+v", rec)
	}

	// (b) 字节闸：条数没超、字节超 ⇒ 注明 bytes 而不是 rate
	capB := &capSink{}
	rB := New(Config{RateBytesPerSec: 250, Clock: clk.now, Sink: capB.sink}, keepAll)
	rB.Admit(baseEvent("b0")) // 100B
	rB.Admit(baseEvent("b1")) // 200B
	if d := rB.Admit(baseEvent("b2")); d.DegradeLevel != DegradeBytes {
		t.Fatalf("字节超限应记 degrade=bytes，实际 %s", d.DegradeLevel)
	}

	// (c) 必留破例：闸饱和时必留**放行**且破例可量化（不是看不见的例外）
	capC := &capSink{}
	rC := New(Config{RateEventsPerSec: 1, Clock: clk.now, Sink: capC.sink},
		NewMustKeep("mk", "出错必留", MatchAttr("err", "1")))
	rC.Admit(errEvent("m0"))
	dm := rC.Admit(errEvent("m1"))
	if !dm.Emitted || dm.DegradeLevel != DegradeRate {
		t.Fatalf("必留应破例放行并记 degrade=rate，实际 emitted=%v degrade=%s", dm.Emitted, dm.DegradeLevel)
	}
	if rC.MustKeepBypasses() != 1 || capC.n() != 2 {
		t.Fatalf("必留破例应计数 1 且两条都落盘，实际 bypass=%d 落盘=%d", rC.MustKeepBypasses(), capC.n())
	}

	// (d) 绝不阻塞：sink 报满 ⇒ **立刻**返回（不是等、不是重试），且失败也留碑
	rD := New(Config{Clock: clk.now, Sink: ChanSink(make(chan Event))}, keepAll) // 无缓冲、无读者
	done := make(chan Decision, 1)
	go func() { done <- rD.Admit(baseEvent("nb")) }()
	select {
	case dd := <-done:
		if !dd.Tombstone || dd.DegradeLevel != DegradeSink || dd.Emitted {
			t.Fatalf("sink 报满应记 degrade=sink 并留碑：tombstone=%v degrade=%s", dd.Tombstone, dd.DegradeLevel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Admit 被阻塞了（C2：绝不阻塞 agent）")
	}
	if rD.SinkFailures() != 1 {
		t.Fatalf("落盘失败应计数，实际 %d", rD.SinkFailures())
	}
	// 落盘失败 = 事实缺席 ⇒ 必须有墓碑（否则"缺席"的歧义从另一条路回来了）
	if _, ok := rD.Ledger("r1").TombstoneFor("nb"); !ok {
		t.Fatal("落盘失败者必须留碑")
	}

	// (e) 容量淘汰遵守**淘汰序**（低档先牺牲；必留档永不淘汰；低档不许挤掉高档）
	capE := &capSink{}
	rE := New(Config{RingBytes: 250, Clock: clk.now, Sink: capE.sink, DiscardGrace: 10 * time.Minute},
		NewDrop("noise", "噪声全丢", MatchAttr("err", "1")))
	rE.Admit(errEvent("e1"))  // 100B，drop 命中（淘汰序 2）
	rE.Admit(baseEvent("n1")) // 100B，默认拒绝（淘汰序 0）
	rE.Admit(baseEvent("n2")) // 装不下 ⇒ 先牺牲最低档、同档最早的 n1
	if _, ok := rE.ProbationUntil("r1", "n1"); ok {
		t.Fatal("容量不够时应先淘汰最低档（默认拒绝）的条目")
	}
	if _, ok := rE.ProbationUntil("r1", "e1"); !ok {
		t.Fatal("drop 档条目不该先于默认拒绝档被淘汰")
	}
	am := rE.Ledger("r1").Amendments()
	if len(am) != 1 || am[0].ToState != StateEvicted || am[0].DegradeLevel != DegradeRing || am[0].Key != "n1" {
		t.Fatalf("提前终结必须留痕（probation→evicted, degrade=ring）：%+v", am)
	}
	rE.Admit(errEvent("e2")) // 再淘汰一个默认拒绝档（n2）
	if rE.Evictions() != 2 {
		t.Fatalf("淘汰计数应为 2，实际 %d", rE.Evictions())
	}
	// 低档不许挤掉高档：新来的默认拒绝档条目**不进 probation**（不超容、不阻塞）
	dE := rE.Admit(baseEvent("n3"))
	if dE.DegradeLevel != DegradeRing || !dE.Tombstone {
		t.Fatalf("装不下且自己更低档的条目应直接终局留碑：degrade=%s tombstone=%v", dE.DegradeLevel, dE.Tombstone)
	}
	if rE.ProbationSkips() != 1 || rE.Evictions() != 2 {
		t.Fatalf("skip=1 且不许再多淘汰，实际 skip=%d evictions=%d", rE.ProbationSkips(), rE.Evictions())
	}
	if _, ok := rE.ProbationUntil("r1", "e1"); !ok {
		t.Fatal("高档次条目必须还在 probation 里（容量该留给更值钱的东西）")
	}
	if rE.ProbationBytes() > rE.Config().RingBytes {
		t.Fatalf("不许超容：%d > %d", rE.ProbationBytes(), rE.Config().RingBytes)
	}

	// (f) 必留档永不淘汰（含被显式否决的必留）
	capF := &capSink{}
	rF := New(Config{RingBytes: 100, Clock: clk.now, Sink: capF.sink, DropVetoesMustKeep: true},
		NewMustKeep("mk", "出错必留", Always()),
		NewDrop("dp", "全丢", Always()))
	if d := rF.Admit(baseEvent("m1")); !d.OverriddenMustKeep {
		t.Fatal("前置条件：显式开关下这条应是被否决的必留")
	}
	rF.Admit(baseEvent("m2")) // 装不下 ⇒ 淘汰不动（剩下的全是必留档）⇒ 新条目不进
	if _, ok := rF.ProbationUntil("r1", "m1"); !ok {
		t.Fatal("必留档（含被否决的必留）永不被容量淘汰")
	}
	if rF.Evictions() != 0 || rF.ProbationSkips() != 1 {
		t.Fatalf("必留档不该被淘汰：evictions=%d skips=%d", rF.Evictions(), rF.ProbationSkips())
	}
	if rF.ProbationBytes() > rF.Config().RingBytes {
		t.Fatalf("不许超容：%d > %d", rF.ProbationBytes(), rF.Config().RingBytes)
	}
}

// ── ⑧ dry-run：只记录不裁 ⇒ 落盘条数 = 全量 ───────────────────────────────

func TestDryRunEmitsEverything(t *testing.T) {
	cap := &capSink{}
	r := New(Config{DryRun: true, Sink: cap.sink}, NewDrop("noise", "噪声全丢", Always()))
	const N = 50
	for i := 0; i < N; i++ {
		d := r.Admit(baseEvent(fmt.Sprintf("k%d", i)))
		if d.Verdict != VerdictDrop {
			t.Fatalf("dry-run 下判词照记（应 drop），实际 %s", d.Verdict)
		}
		if d.EffectiveVerdict() != VerdictKeep || !d.Emitted || d.Tombstone {
			t.Fatalf("dry-run 不得真裁：effective=%s emitted=%v tombstone=%v", d.EffectiveVerdict(), d.Emitted, d.Tombstone)
		}
	}
	led := r.Ledger("r1")
	if cap.n() != N || led.Emitted() != N || led.Count() != N {
		t.Fatalf("dry-run 落盘条数应 = 全量 %d：sink=%d emitted=%d 账=%d", N, cap.n(), led.Emitted(), led.Count())
	}
	if led.TombstoneCount() != 0 || r.ProbationCount() != 0 {
		t.Fatalf("dry-run 不该有墓碑/未决：tombstone=%d probation=%d", led.TombstoneCount(), r.ProbationCount())
	}
	for _, rec := range led.Records() {
		if !rec.DryRun || rec.Tombstone || rec.Decision != string(VerdictDrop) {
			t.Fatalf("dry-run 的账行必须 dry_run=true / tombstone=false / 判词照记：%+v", rec)
		}
	}
	// C5 点名的字段名（点分键）必须在盘上找得到 —— 读侧靠它们取件
	line := led.JSONL()
	var raw map[string]any
	if err := json.Unmarshal([]byte(firstLine(string(line))), &raw); err != nil {
		t.Fatalf("账行不是合法 JSON：%v", err)
	}
	for _, k := range []string{"sampling.decision", "sampling.deciding_policy", "sampling.degrade_level", "tombstone"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("账行缺字段 %q（C5 点名）", k)
		}
	}
	// 证据：dry-run 下的真实账行（判词是 drop，但没裁 —— 两个事实都在一行里）
	t.Logf("dry-run 账行样例：%s", firstLine(string(line)))

	// 反例对照：同策略集**关掉 dry-run** ⇒ 一条都不落（证明"全量"是 dry-run 带来的，不是策略没生效）
	cap2 := &capSink{}
	r2 := New(Config{Sink: cap2.sink}, NewDrop("noise", "噪声全丢", Always()))
	for i := 0; i < N; i++ {
		r2.Admit(baseEvent(fmt.Sprintf("d%d", i)))
	}
	if cap2.n() != 0 {
		t.Fatalf("关掉 dry-run 后应全裁，实际落了 %d 条", cap2.n())
	}
	if r2.Ledger("r1").TombstoneCount() != N {
		t.Fatalf("关掉 dry-run 后应留 %d 块墓碑，实际 %d", N, r2.Ledger("r1").TombstoneCount())
	}
}

// ── ⑨ 墓碑：被裁者有记录（"缺席"不再歧义）────────────────────────────────

func TestTombstoneForDropped(t *testing.T) {
	clk := newFakeClock()
	cap := &capSink{}
	r := New(Config{Clock: clk.now, Sink: cap.sink, DiscardGrace: 10 * time.Minute},
		NewMustKeep("err_keep", "出错必留", MatchAttr("err", "1")),
		NewProbabilistic("p_half", "半量概率采样", 0.5, Always()))

	const nErr, nPlain = 20, 60
	for i := 0; i < nErr; i++ {
		r.Admit(errEvent(fmt.Sprintf("err%d", i)))
	}
	for i := 0; i < nPlain; i++ {
		r.Admit(baseEvent(fmt.Sprintf("p%d", i)))
	}
	total := nErr + nPlain
	led := r.Ledger("r1")
	if led.Count() != total {
		t.Fatalf("每条事件都应有一条判定行：期望 %d，实际 %d", total, led.Count())
	}
	dropped, kept := map[string]bool{}, map[string]bool{}
	for _, rec := range led.Records() {
		if rec.Tombstone {
			dropped[rec.Key] = true
			// 墓碑必须自带取证要素（否则只是"少了一行"，不是证据）
			if rec.DecidingPolicy == "" || rec.DegradeLevel == "" || rec.Decision != string(VerdictDrop) {
				t.Fatalf("墓碑字段不全：%+v", rec)
			}
			if rec.ProbationUntil == "" {
				t.Fatal("判丢的墓碑应带宽限截止（说明还有没有翻案窗口）")
			}
			if rec.Bytes != 100 || rec.EventName != "turn" {
				t.Fatalf("墓碑应带事件身份要素（bytes/event_name）：%+v", rec)
			}
		} else {
			kept[rec.Key] = true
		}
	}
	// 三种命运必须闭合：留的 ∪ 丢的 = 全集，且不相交（没有第三种命运，也没有人两份）
	if len(kept)+len(dropped) != total {
		t.Fatalf("留(%d)+丢(%d) 应等于全集 %d", len(kept), len(dropped), total)
	}
	for k := range kept {
		if dropped[k] {
			t.Fatalf("key %s 同时被判留与判丢（闭合性破裂）", k)
		}
	}
	if len(dropped) == 0 || len(kept) == 0 {
		t.Fatalf("本用例需要留/丢都有：kept=%d dropped=%d", len(kept), len(dropped))
	}
	if cap.n() != len(kept) || led.Emitted() != len(kept) {
		t.Fatalf("落盘条数应等于'留'的条数：sink=%d emitted=%d kept=%d", cap.n(), led.Emitted(), len(kept))
	}
	// 必留 20 条 100% 落在"留"里（墓碑分不出来就白做）
	for i := 0; i < nErr; i++ {
		if !kept[fmt.Sprintf("err%d", i)] {
			t.Fatalf("必留 err%d 竟被判丢", i)
		}
	}
	// 反例：留下的 key **不许**有墓碑（否则墓碑失去区分力）
	for k := range kept {
		if _, ok := led.TombstoneFor(k); ok {
			t.Fatalf("留下的 key %s 不该有墓碑", k)
		}
	}
	// 按 key 查得到（消费侧的"它到哪去了"接口）
	var someDropped string
	for k := range dropped {
		someDropped = k
		break
	}
	if rec, ok := led.TombstoneFor(someDropped); !ok || !rec.Tombstone {
		t.Fatalf("墓碑按 key 查不到：%s", someDropped)
	}
	// 证据：真实墓碑行（被裁者不是"少了一行"，而是一条可检索的取证件）
	for _, rec := range led.Tombstones()[:1] {
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("墓碑不可序列化：%v", err)
		}
		t.Logf("墓碑样例（kept=%d dropped=%d）：%s", len(kept), len(dropped), b)
	}

	// 尺寸约束自检（C3）只给数字、不报错：不满足 ⇒ 宽限期会被提前终结（degrade_level=ring）
	cfg := Config{RingBytes: 250, DiscardGrace: 10 * time.Minute}
	ok, want := cfg.SizingOK(109, 512)
	if !ok && want <= cfg.RingBytes {
		t.Fatalf("SizingOK 自相矛盾：ok=%v want=%d ring=%d", ok, want, cfg.RingBytes)
	}
	if want != RequiredRingBytes(109, 10*time.Minute, 512) {
		t.Fatal("SizingOK 与 RequiredRingBytes 口径不一致")
	}
}

// firstLine — 取首行（账行按 JSONL 写，一行一条）。
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

// 用例⑦ (d) 用到 ErrQueueFull 的语义 ⇒ 这里把"它必须是个可判定的错误值"钉住。
func TestErrQueueFullIsTyped(t *testing.T) {
	if !errors.Is(ErrQueueFull, ErrQueueFull) {
		t.Fatal("ErrQueueFull 必须是个可比较的错误值")
	}
	ch := make(chan Event, 1)
	s := ChanSink(ch)
	if err := s(baseEvent("x")); err != nil {
		t.Fatalf("有空间时应投递成功：%v", err)
	}
	if err := s(baseEvent("y")); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("满了应返回 ErrQueueFull，实际 %v", err)
	}
}
