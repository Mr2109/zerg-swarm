// constraints_test.go — 重注入（治本）实证：**头尾各一份 / 幂等不堆叠 / 每 N 轮定时重注**
//
// 要治的盲区（改本文件前先读；与 constraints.go 文末「重注入」节同源）：
//   - 实测缺陷②：`constraint_missing_alert` 抓到过**真缺失**——缺的正是任务书那句
//     「只做 G2，不要动别的缺陷，也不要动 git」（must_survive=true, inject_where=history），
//     而那一刻**实际发出的提示里确实检索不到它**；老实现只有"检测 + 报警"、没有"治疗"⇒
//     告警响着、模型照样不照做。本文件把"治疗"钉成**可判定的字节**：缺了真的补上（头尾各一份）、
//     补了不堆叠（幂等）、没缺也每 N 轮补一次。
//
// 用例（三条；判据全部打在**返回的字节**上，不接受"函数自称注入了"）：
//
//	① missing ⇒ 重注入后**检索得到**，且开头与结尾**各一份**；原字节逐字留在中间，一个字符不动
//	② 幂等：同一 (提示, 约束集, 轮次) 再调一次 ⇒ 逐字节相同；块不堆叠（标记恰好两处、约束原文恰好两份）；
//	    先剔后放的根基是 strip 的**逐字节可逆**（用例里独立断言）
//	③ 定时：约束都在场（missing 恒空）也**每 N 轮**补一次（N 可配：默认 4；环境变量与入参两条路都钉）——
//	    用**注入的 会话/轮次/时钟**断言，不靠墙上时钟（H6：渲染不吃时钟）
//
// 纪律（与 obs_prompt_test.go 同源）：
//   - 周期状态是**进程内**的 ⇒ 每条用例自己清（ResetReinjectState），会话名互不相同；
//   - 独立复算：指纹/标记处数/约束份数都用测试侧的独立口径重算，不调用被测 helper 自证。
package chat

import (
	"strings"
	"testing"
	"time"
)

// reinjectTestCS — 实测缺陷②那条约束 + 一条**软**约束（软约束不得进注入块：它会被"浓缩规则提醒"退化）
func reinjectTestCS() (must Constraint, soft Constraint, cs []Constraint) {
	must = NewConstraint("只做 G2，不要动别的缺陷，也不要动 git", 42, true, injectWhereHistory)
	soft = NewConstraint("建议顺手把日志整理一下", 43, false, injectWhereHistory)
	return must, soft, []Constraint{must, soft}
}

// reinjectFixedClock — 用例的注入时钟（固定值 ⇒ 断言可复算；**不进提示字节**，只进事件/结果）
const reinjectFixedClock = "2026-09-18T10:00:00Z"

// ── 用例 ①：missing ⇒ 头尾各一份，原字节不动 ──────────────────────────

func TestReinjectMissingConstraintHeadAndTail(t *testing.T) {
	ResetReinjectState("")
	t.Cleanup(func() { ResetReinjectState("") })

	must, soft, cs := reinjectTestCS()
	prompt := "TEMPLATE-BASE\n\n# 你的身份\n- 虫族本地集群\n\n（历史若干轮——那句硬约束已被压缩吃掉，检索不到）"

	got := ReinjectConstraints(prompt, cs, ReinjectOptions{
		Session: "sess-reinj-1", Round: 1, ClockISO: reinjectFixedClock,
	})

	if !got.Injected || got.Reason != ReinjectReasonMissing {
		t.Fatalf("缺 must_survive ⇒ 应立刻重注入（reason=missing）：%+v", got)
	}
	if got.Count != 1 || len(got.IDs) != 1 || got.IDs[0] != ConstraintID(must.Canonical) {
		t.Errorf("注入的应是那一条 must_survive（id 由 canonical 唯一决定）：%+v", got.IDs)
	}
	if got.InjectionCount != 1 || got.Round != 1 || got.ClockISO != reinjectFixedClock {
		t.Errorf("累计次数/轮次/时钟应如实回传：count=%d round=%d clock=%q", got.InjectionCount, got.Round, got.ClockISO)
	}
	if got.InjectionID != got.InjectionSHA256[:16] || got.InjectionSHA256 == "" {
		t.Errorf("注入块指纹口径（前 16 位 = 全串前 16 位）不对：%q vs %q", got.InjectionID, got.InjectionSHA256)
	}
	if got.InjectionSHA256 != ConstraintFingerprint(buildReinjectBlock([]Constraint{must})) {
		t.Error("注入块指纹必须就是 sha256(块原文)——事件里的 fingerprint 要能被读侧独立复算")
	}

	// ② 头尾各一份：开头是开标记、结尾是闭标记；块出现两处、约束原文出现两份
	if !strings.HasPrefix(got.Prompt, reinjectMarkOpen) {
		t.Error("头一份必须在**开头**（HasPrefix 开标记）")
	}
	if !strings.HasSuffix(got.Prompt, reinjectMarkClose) {
		t.Error("尾一份必须在**结尾**（HasSuffix 闭标记）")
	}
	if n := strings.Count(got.Prompt, reinjectMarkOpen); n != 2 {
		t.Errorf("头尾各一份 ⇒ 开标记恰好 2 处，实得 %d", n)
	}
	if n := strings.Count(got.Prompt, must.Canonical); n != 2 {
		t.Errorf("头尾各一份 ⇒ 约束原文恰好 2 份，实得 %d", n)
	}

	// ① 原字节一个不差地留在中间（"原字节不动地插入"）
	if !strings.Contains(got.Prompt, prompt) {
		t.Fatal("原字节必须逐字留在中间（插入，不是重写）")
	}
	head := strings.Index(got.Prompt, must.Canonical)
	body := strings.Index(got.Prompt, prompt)
	tail := strings.LastIndex(got.Prompt, must.Canonical)
	if !(head < body) || !(tail >= body+len(prompt)) {
		t.Errorf("头份应在原文之前、尾份应在原文之后：head=%d body=%d tail=%d len(body)=%d", head, body, tail, len(prompt))
	}

	// 注入块只放 must_survive（软约束不进块——否则"浓缩规则提醒"退化成摘要）
	if strings.Contains(got.Prompt, soft.Canonical) {
		t.Error("软约束不得进注入块（must_survive=false）")
	}

	// 治本的验收判据：在**重注入后的字节**上，那条 must_survive 约束**检索得到**了（下一轮 check 就该 present=1）
	if rest := missingConstraints([]Constraint{must}, CanonicalConstraint(got.Prompt)); len(rest) != 0 {
		t.Errorf("重注入后必须检索得到那条约束（实得仍缺 %+v）", rest)
	}
	// 不注入时逐字节等于入参（三态中的"没注入"）
	if same := ReinjectConstraints(prompt, nil, ReinjectOptions{Session: "sess-reinj-0", Round: 9}); same.Prompt != prompt {
		t.Error("没有约束 ⇒ 一个字都不许改（Prompt 应逐字节等于入参）")
	}
}

// ── 用例 ②：幂等（已存在的不得重复堆叠）＋ strip 逐字节可逆 ────────────────

func TestReinjectIdempotentNoStacking(t *testing.T) {
	ResetReinjectState("")
	t.Cleanup(func() { ResetReinjectState("") })

	must, _, cs := reinjectTestCS()
	prompt := "TEMPLATE-BASE\n（历史——约束被压掉了）"
	opt := ReinjectOptions{Session: "sess-reinj-2", Round: 1, ClockISO: reinjectFixedClock}

	first := ReinjectConstraints(prompt, cs, opt)
	if !first.Injected {
		t.Fatalf("前提不成立：第一轮应注入：%+v", first)
	}
	// 同一轮、同一份待发提示（= 上一轮补完的字节）再调一次 ⇒ 已存在 ⇒ 不得再堆一层
	second := ReinjectConstraints(first.Prompt, cs, opt)
	if second.Injected || second.Reason != ReinjectReasonNone {
		t.Errorf("已存在的约束不得重复注入（应 reason=none）：%+v", second)
	}
	if second.Prompt != first.Prompt {
		t.Error("幂等：同一 (提示, 约束集, 轮次) 再调一次必须**逐字节相同**")
	}
	if n := strings.Count(second.Prompt, reinjectMarkOpen); n != 2 {
		t.Errorf("块不得堆叠：开标记应恒为 2 处，实得 %d", n)
	}
	if n := strings.Count(second.Prompt, must.Canonical); n != 2 {
		t.Errorf("约束原文不得堆叠：应恒为 2 份，实得 %d", n)
	}
	// 幂等的那次**没有**再记一次注入（计数只在真的放下块时 +1）
	if n, last := ReinjectStats("sess-reinj-2"); n != 1 || last != 1 {
		t.Errorf("重复调用不得让累计次数涨（count=%d lastRound=%d）", n, last)
	}

	// 先剔后放的根基：strip 必须**逐字节可逆**（否则每轮都会漂一个字符，前缀缓存白拆）
	if base, ok := stripReinjectBlocks(first.Prompt); !ok || base != prompt {
		t.Errorf("strip(块+原文+块) 必须逐字节还原原文：ok=%v %q", ok, base)
	}
	// 可逆性还得扛住"装配点在尾份之后又接了自己的文本"（下一轮拼接 + 新增行——实测踩过的坑：
	// 若把尾份**两侧**的换行都剔掉，会吃掉调用方那个换行）
	appended := first.Prompt + "\n# 会话环境\n- 模型: gemma-12B"
	if base, ok := stripReinjectBlocks(appended); !ok || base != prompt+"\n# 会话环境\n- 模型: gemma-12B" {
		t.Errorf("尾份之后接的调用方文本必须逐字保留：ok=%v %q", ok, base)
	}
	// strip 的两条防误删条件：别人的同形标记不许动
	foreign := reinjectMarkOpen + "\n别人的内容（不带头行）\n" + reinjectMarkClose + "\n" + prompt
	if out, ok := stripReinjectBlocks(foreign); ok || out != foreign {
		t.Errorf("同形标记但不是我们的块 ⇒ 一个字节都不许动：ok=%v %q", ok, out)
	}

	// "只有头一份"（半块）在应当重注的轮次里被**规整回头尾各一份**，而不是叠成四份：
	// 场景 = 装配点只采纳了一半（人为构造）＋ 到周期 ⇒ 先剔后放
	ResetReinjectState("sess-reinj-2b")
	half := buildReinjectBlock([]Constraint{must}) + "\n" + prompt
	fixed := ReinjectConstraints(half, cs, ReinjectOptions{
		Session: "sess-reinj-2b", Round: 4, ClockISO: reinjectFixedClock, // 起点锚 0 ⇒ 4-0 ≥ 4 到周期
	})
	if !fixed.Injected || fixed.Reason != ReinjectReasonPeriodic {
		t.Fatalf("半块 + 到周期 ⇒ 应周期重注（先剔后放）：%+v", fixed)
	}
	if n := strings.Count(fixed.Prompt, reinjectMarkOpen); n != 2 {
		t.Errorf("半块应被规整回头尾各一份（不是叠成 %d 处）", n)
	}
	if !strings.Contains(fixed.Prompt, prompt) {
		t.Error("规整后原字节仍须逐字在中间")
	}
}

// ── 用例 ③：每 N 轮定时重注（约束都在场也补；N 可配：默认 4）────────────────

func TestReinjectPeriodicEveryNRounds(t *testing.T) {
	ResetReinjectState("")
	t.Cleanup(func() { ResetReinjectState("") })

	must, _, cs := reinjectTestCS()
	// 前提：约束**在原文里在场**（missing 恒为空）⇒ 唯一可能的触发就是"到周期"
	prompt := "TEMPLATE-BASE\n- " + must.Canonical + "\n# 会话环境\n- 模型: gemma-12B"
	if len(missingConstraints([]Constraint{must}, CanonicalConstraint(prompt))) != 0 {
		t.Fatal("前提不成立：本用例要的是 missing 为空（约束在原文里在场）")
	}

	type roundFact struct {
		round      int
		injected   bool
		reason     string
		count      int
		lastRound  int
		occurrence int // 标记处数（每轮都不许堆叠）
	}
	var got []roundFact
	cur := prompt // 模拟装配点采纳：把补过的字节当下一轮的待发提示
	for round := 1; round <= 8; round++ {
		r := ReinjectConstraints(cur, cs, ReinjectOptions{
			Session: "sess-reinj-3", Round: round, ClockISO: reinjectFixedClock,
		})
		n, last := ReinjectStats("sess-reinj-3")
		got = append(got, roundFact{round, r.Injected, r.Reason, n, last, strings.Count(r.Prompt, reinjectMarkOpen)})
		cur = r.Prompt
	}

	// 默认 N=4、锚点=会话起点(0) 与最近一次注入的轮次 ⇒ 第 4、8 轮各注一次，其余轮一次都不注
	wantInject := map[int]int{4: 1, 8: 2} // 轮次 → 该轮之后的累计注入次数
	for _, f := range got {
		// 没注过 ⇒ 提示里没有块（0 处）；注过之后 ⇒ 恒为头尾两份（**不许堆叠**：注入两次也还是 2 处）
		wantOcc := 0
		if f.count > 0 {
			wantOcc = 2
		}
		if f.occurrence != wantOcc {
			t.Fatalf("第 %d 轮：块处数应为 %d（累计注入 %d 次），实得 %d 处", f.round, wantOcc, f.count, f.occurrence)
		}
		if want, shouldInject := wantInject[f.round]; shouldInject {
			if !f.injected || f.reason != ReinjectReasonPeriodic {
				t.Errorf("第 %d 轮：应**定时重注**（reason=periodic）：%+v", f.round, f)
			}
			if f.count != want || f.lastRound != f.round {
				t.Errorf("第 %d 轮：累计次数应为 %d、锚点应变到本轮：%+v", f.round, want, f)
			}
			continue
		}
		if f.injected || f.reason != ReinjectReasonNone {
			t.Errorf("第 %d 轮：未到周期且无缺失 ⇒ 一个字都不许改（reinjected 应为 false）：%+v", f.round, f)
		}
	}
	// 每轮的字节仍逐字包含原提示（含约束那句）—— 定时重注不是重写提示
	if !strings.Contains(cur, prompt) {
		t.Error("定时重注也必须保留原字节")
	}

	// N 可配（环境变量那条路）：设 2 ⇒ 第 2、4、6 轮各注一次（锚点同样从会话起点起算）
	t.Setenv("ZERG_CONSTRAINT_REINJECT_EVERY", "2")
	ResetReinjectState("sess-reinj-3-env")
	cur = prompt
	var injectedAt []int
	for round := 1; round <= 6; round++ {
		r := ReinjectConstraints(cur, cs, ReinjectOptions{Session: "sess-reinj-3-env", Round: round, ClockISO: reinjectFixedClock})
		if r.Injected {
			injectedAt = append(injectedAt, round)
		}
		cur = r.Prompt
	}
	if want := []int{2, 4, 6}; len(injectedAt) != len(want) || injectedAt[0] != want[0] || injectedAt[1] != want[1] || injectedAt[2] != want[2] {
		t.Errorf("ZERG_CONSTRAINT_REINJECT_EVERY=2 ⇒ 每 2 轮注一次（第 2/4/6 轮），实得 %v", injectedAt)
	}
	// N 可配（入参那条路）：显式给 3 ⇒ 第 3、6 轮
	ResetReinjectState("sess-reinj-3-arg")
	cur = prompt
	injectedAt = nil
	for round := 1; round <= 6; round++ {
		r := ReinjectConstraints(cur, cs, ReinjectOptions{
			Session: "sess-reinj-3-arg", Round: round, EveryNRounds: 3, ClockISO: reinjectFixedClock})
		if r.Injected {
			injectedAt = append(injectedAt, round)
		}
		cur = r.Prompt
	}
	if len(injectedAt) != 2 || injectedAt[0] != 3 || injectedAt[1] != 6 {
		t.Errorf("EveryNRounds=3 ⇒ 第 3/6 轮，实得 %v", injectedAt)
	}

	// 无轮次/无会话 ⇒ 定时不触发（不编造节奏），且**不编造累计计数**
	ResetReinjectState("")
	r := ReinjectConstraints(prompt, cs, ReinjectOptions{Round: 0, ClockISO: reinjectFixedClock})
	if r.Injected || r.InjectionCount != 0 {
		t.Errorf("没给轮次/会话 ⇒ 只允许「刚检到缺失」那条路的即时补充：%+v", r)
	}
	// 非法 N（0/负数）⇒ 回落默认值（不许被读成"关掉定时重注"）
	if reinjectEveryNRounds() != 2 { // 上面的 Setenv 还在生效
		t.Errorf("非法/未设环境变量时的回落口径不对：%d", reinjectEveryNRounds())
	}
	t.Setenv("ZERG_CONSTRAINT_REINJECT_EVERY", "0")
	if reinjectEveryNRounds() != reinjectEveryDefault {
		t.Errorf("N=0 必须回落默认 %d（0 会把定时重注整个关掉）：%d", reinjectEveryDefault, reinjectEveryNRounds())
	}
	t.Setenv("ZERG_CONSTRAINT_REINJECT_EVERY", "bogus")
	if reinjectEveryNRounds() != reinjectEveryDefault {
		t.Error("非法取值必须回落默认值（不猜）")
	}

	// 时钟口径：零值 ⇒ 空（不编造时钟）；非零 ⇒ UTC RFC3339Nano
	if ReinjectClockISO(time.Time{}) != "" {
		t.Error("零值时钟应落空（不编造）")
	}
	if got := ReinjectClockISO(mustParseTime(t, "2026-09-18T18:00:00+08:00")); got != "2026-09-18T10:00:00Z" {
		t.Errorf("时钟口径应是 UTC RFC3339Nano：%q", got)
	}
}
