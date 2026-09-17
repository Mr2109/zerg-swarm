// obs_prompt_version_test.go — T3.4（H4）+ T3.6（H7）实证
//
// 要治的盲区（改本文件前先读）：
//   - 「这次劣化是哪个提示版本造成的」——分段落哈希只说"变了"，说不出"是哪一版"、更说不出
//     "当时那一版挂着哪个 label"（H4）；本文件把**版本外键 + label→version 快照**钉成可判定的事实；
//   - 「给大目标⇒只读不交差；给一步一验的小目标⇒真动手」——这是**提示脚手架（策略）**差异，
//     事件里此前一个字段都没有 ⇒ 只能凭印象（H7）；本文件把策略的四个字段钉住。
//
// 用例 5 条（含反例）：
//
//	① 同一模板两次装配 ⇒ prompt_version 相同；模板改一字 ⇒ 变；**只改记忆块**（模板外动态注入）
//	  ⇒ 版本不变（但 rendered_prefix_hash 变）——版本标识对"模板"取指纹，与 T3.3 的哈希分工不混；
//	  版本值由测试侧**独立复算**（sha256 前 8 字节），不用被测 helper 自证。
//	② label→version 快照可回查：dev→v1、canary→v2 并存（回滚后能回答"当时线上是哪版"），
//	  事件原文里带快照；没登记过的标签/名字查不到（不编造）；缺键的登记被拒。
//	③ 策略四字段能落盘且能读回（含 **显式 false 照落**）；strategy_id 同配置同值（集合语义、顺序无关）、
//	  改一个参数即换版；全空配置 ⇒ 不编造 id。
//	④【反例】无脚手架信息 ⇒ 四个策略键**一个都不出现**（不得写 0/空串冒充）；名字/标签同理；
//	  而 prompt_version 该有（它算得出来，不是编造）。
//	⑤ step_index 随会话轮次递增（会话级步序：1,2,3…），别的会话各自从 1 起，显式值优先。
//
// 纪律（与 obs_prompt_test.go 同源）：
//   - 观测一律落隔离目录；本任务的**进程内表**（标签快照/步数）必须在用例间清空（pvIsolate）。
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里）+ **read 层**（解回来的值是真的）。
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// ── 隔离与读回助手 ────────────────────────────────────────────────────

// pvIsolate — 隔离状态目录 + 丢弃本任务的进程内表（标签/步数是进程级的，跨用例必须干净起步）
func pvIsolate(t *testing.T) {
	t.Helper()
	promptIsolate(t)
	resetPromptVersionTables()
	t.Cleanup(resetPromptVersionTables)
}

// pvRawLinesOf — 原始 JSONL 里含某标记（会话名/模型名）的行（**wire 层**判据：键真的在原文里）
func pvRawLinesOf(t *testing.T, mark string) []string {
	t.Helper()
	var out []string
	for _, ln := range strings.Split(strings.TrimSpace(readObs(t)), "\n") {
		if ln != "" && strings.Contains(ln, mark) {
			out = append(out, ln)
		}
	}
	return out
}

// pvPromptLineOf — 取某标记对应的 constraint_check 读回行（read 层判据）
func pvPromptLineOf(t *testing.T, mark string) promptObsLine {
	t.Helper()
	for _, ln := range readPromptObsLines(t) {
		if ln.EventName == obsEventConstraintCheck && (ln.Session == mark || ln.Model == mark) {
			return ln
		}
	}
	t.Fatalf("没找到标记 %q 的 %s 行", mark, obsEventConstraintCheck)
	return promptObsLine{}
}

// pvVersionOf — 测试侧**独立复算**版本标识（sha256 前 8 字节十六进制；不复用被测 helper）
func pvVersionOf(template string) string {
	sum := sha256.Sum256([]byte(template))
	return hex.EncodeToString(sum[:8])
}

// pvObserve — 一次提示装配（同一条渲染路径的入参形态）
func pvObserve(r PromptRender) {
	if r.Identity.ClockISO == "" {
		r.Identity = FixedRequestIdentity("2026-09-17T12:00:00Z", 1)
	}
	if r.History == nil {
		r.History = []map[string]any{{"role": "user", "content": "你好"}}
	}
	ObservePromptCheck(r)
}

// ── ① 版本标识：对**模板**取指纹（同模板同版、改一字换版、只改记忆块不换版）──

func TestPromptTemplateVersionTracksTemplateOnly(t *testing.T) {
	pvIsolate(t)

	const (
		tmplA = "模板-A：你是对话助手\n"
		tmplB = "模板-B：你是对话助理\n" // 逐字相对 tmplA 改了一处（"手"→"理"）
		memA  = "记忆块：Mr2109 要求报告用中文"
		memB  = "记忆块：Mr2109 要求报告用英文"
	)
	pvObserve(PromptRender{Session: "sess-T34-a", Round: 1, Model: "m",
		System: tmplA + memA, Memory: memA})
	pvObserve(PromptRender{Session: "sess-T34-b", Round: 2, Model: "m",
		System: tmplA + memA, Memory: memA}) // 同一模板、第二次装配（会话/轮次/时钟都不同）
	pvObserve(PromptRender{Session: "sess-T34-c", Round: 1, Model: "m",
		System: tmplB + memA, Memory: memA}) // 模板改一字
	pvObserve(PromptRender{Session: "sess-T34-d", Round: 1, Model: "m",
		System: tmplA + memB, Memory: memB}) // 只改记忆块（模板外动态注入）

	a := pvPromptLineOf(t, "sess-T34-a")
	b := pvPromptLineOf(t, "sess-T34-b")
	c := pvPromptLineOf(t, "sess-T34-c")
	d := pvPromptLineOf(t, "sess-T34-d")

	if a.PromptVersion == "" {
		t.Fatal("prompt_version 必须在场（它由渲染后模板算出，不是编造的）")
	}
	if a.PromptVersion != pvVersionOf(tmplA) {
		t.Errorf("版本标识口径不符：got %s want %s（sha256(模板)[:8]）", a.PromptVersion, pvVersionOf(tmplA))
	}
	if len(a.PromptVersion) != 16 {
		t.Errorf("版本标识应为 sha256 前 8 字节=16 字符十六进制：%q", a.PromptVersion)
	}
	if a.PromptVersion != b.PromptVersion {
		t.Errorf("同一模板两次装配必须同版：%s vs %s", a.PromptVersion, b.PromptVersion)
	}
	if c.PromptVersion == a.PromptVersion {
		t.Errorf("模板改一字必须换版（否则版本外键回答不了\"是哪一版\"）：%s", c.PromptVersion)
	}
	if c.PromptVersion != pvVersionOf(tmplB) {
		t.Errorf("改字后的版本应为 %s：got %s", pvVersionOf(tmplB), c.PromptVersion)
	}
	// 关键分工：只改**记忆块**（模板外动态注入）⇒ 模板没变 ⇒ 版本不变；但提示真的变了（前缀哈希必变）
	if d.PromptVersion != a.PromptVersion {
		t.Errorf("只改记忆块不得换版（版本对模板取指纹）：%s vs %s", d.PromptVersion, a.PromptVersion)
	}
	if d.Prompt == nil || a.Prompt == nil || d.Prompt.RenderedPrefixHash == a.Prompt.RenderedPrefixHash {
		t.Error("改了记忆块 ⇒ 渲染后前缀哈希必须变（否则\"版本不变\"就成了没看见变更的借口）")
	}
}

// ── ② label→version 快照可回查（回滚也能复现当时线上状态）──────────────

func TestPromptLabelVersionSnapshotQueryable(t *testing.T) {
	pvIsolate(t)

	const (
		name   = PromptNameChatTieredSystem
		tmplV1 = "模板 v1\n"
		tmplV2 = "模板 v2\n"
		mem    = "记忆块：X"
	)
	v1, v2 := pvVersionOf(tmplV1), pvVersionOf(tmplV2)
	if v1 == v2 {
		t.Fatal("用例前提不成立：两版模板的指纹相同")
	}
	pvObserve(PromptRender{Session: "sess-T34-lv1", Round: 1, Model: "m",
		System: tmplV1 + mem, Memory: mem, PromptName: name, PromptLabel: "dev"})
	pvObserve(PromptRender{Session: "sess-T34-lv2", Round: 1, Model: "m",
		System: tmplV2 + mem, Memory: mem, PromptName: name, PromptLabel: "canary"})

	// read 层：两个标签各自指向自己那一版（**旧标签仍指旧版** —— 回滚要的正是这个）
	if got, ok := PromptLabelVersion(name, "dev"); !ok || got != v1 {
		t.Errorf("dev 应仍指 v1=%s：got %q ok=%v", v1, got, ok)
	}
	if got, ok := PromptLabelVersion(name, "canary"); !ok || got != v2 {
		t.Errorf("canary 应指 v2=%s：got %q ok=%v", v2, got, ok)
	}
	snap := PromptLabelVersionsOf(name)
	if snap["dev"] != v1 || snap["canary"] != v2 {
		t.Errorf("label→version 快照不完整：%v", snap)
	}
	// 反例：没登记过的标签/名字 ⇒ 查不到（不编造一个"看起来合理"的版本）
	if _, ok := PromptLabelVersion(name, "prod"); ok {
		t.Error("没登记过的标签必须查不到")
	}
	if _, ok := PromptLabelVersion("chat.never_seen", "dev"); ok {
		t.Error("没登记过的提示名必须查不到")
	}
	// 登记纪律：缺键（名字/标签/版本任一为空）不登记
	if RegisterPromptLabelVersion("", "dev", v1) || RegisterPromptLabelVersion(name, "", v1) ||
		RegisterPromptLabelVersion(name, "dev", "") {
		t.Error("缺键的登记必须被拒（否则表里全是无从检索的行）")
	}

	// wire 层：**事件自己带快照**（回滚后翻当时的行即可复现，不依赖进程内表）
	raw := strings.Join(pvRawLinesOf(t, "sess-T34-lv2"), "\n")
	if !strings.Contains(raw, `"prompt_name":"`+name+`"`) || !strings.Contains(raw, `"prompt_label":"canary"`) {
		t.Errorf("事件必须带 prompt_name/prompt_label：%s", raw)
	}
	if !strings.Contains(raw, `"dev":"`+v1+`"`) || !strings.Contains(raw, `"canary":"`+v2+`"`) {
		t.Errorf("事件必须带 label→version 快照（dev→v1 与 canary→v2）：%s", raw)
	}
	if !strings.Contains(raw, `"prompt_version":"`+v2+`"`) {
		t.Errorf("事件必须带版本外键 prompt_version=%s：%s", v2, raw)
	}
}

// ── ③ 策略四字段落盘 + 读回（含显式 false）+ strategy_id 的确定性与版本性 ──

func TestPromptStrategyFieldsRoundTrip(t *testing.T) {
	pvIsolate(t)

	spec := PromptScaffoldSpec{
		Strategy: "chat.loop",
		Features: []string{"no_terminator", "hermes_xml_tools"},
		Params:   []string{"max_rounds=10", "wall_clock=10m0s", "round_timeout=2m0s"},
	}
	sid := StrategyIDOf(spec)
	if sid == "" || len(sid) != 16 {
		t.Fatalf("strategy_id 应为 16 字符指纹：%q", sid)
	}
	// 集合语义（书写顺序无关）⇒ 同 id；改一个参数 ⇒ 换版；全空 ⇒ 缺席（不编造）
	reordered := PromptScaffoldSpec{
		Strategy: "chat.loop",
		Features: []string{"hermes_xml_tools", "no_terminator"},
		Params:   []string{"round_timeout=2m0s", "max_rounds=10", "wall_clock=10m0s"},
	}
	if StrategyIDOf(reordered) != sid {
		t.Error("同一份配置（顺序不同）必须得到同一 strategy_id（集合语义）")
	}
	changed := spec
	changed.Params = []string{"max_rounds=20", "wall_clock=10m0s", "round_timeout=2m0s"}
	if StrategyIDOf(changed) == sid {
		t.Error("配置改一个字节必须换 strategy_id（否则\"换版\"不可判定）")
	}
	if StrategyIDOf(PromptScaffoldSpec{}) != "" {
		t.Error("一项配置事实都没有 ⇒ strategy_id 必须缺席（不编造一个看着像 id 的值）")
	}

	// 观测一：显式步序 7；验收标准=有（true）、要求先出工具调用=没有（**显式 false 必须照落**）
	pvObserve(PromptRender{Session: "sess-T36-rt1", Round: 3, Model: "m",
		System: "模板\n", Strategy: &PromptStrategyIn{
			StrategyID:            sid,
			StepIndex:             7,
			HasAcceptanceCriteria: PromptStrategyFlag(true),
			RequiresToolCallFirst: PromptStrategyFlag(false),
		}})
	// 观测二：两个布尔反过来（证明落的是**声明值**，不是常量/默认值）
	pvObserve(PromptRender{Session: "sess-T36-rt2", Round: 4, Model: "m",
		System: "模板\n", Strategy: &PromptStrategyIn{
			StrategyID:            sid,
			StepIndex:             8,
			HasAcceptanceCriteria: PromptStrategyFlag(false),
			RequiresToolCallFirst: PromptStrategyFlag(true),
		}})

	l1 := pvPromptLineOf(t, "sess-T36-rt1")
	if l1.StrategyID != sid || l1.StepIndex == nil || *l1.StepIndex != 7 ||
		l1.HasAcceptanceCriteria == nil || !*l1.HasAcceptanceCriteria ||
		l1.RequiresToolCallFirst == nil || *l1.RequiresToolCallFirst {
		t.Errorf("策略四字段读回不符：id=%q step=%v acc=%v tool=%v",
			l1.StrategyID, l1.StepIndex, l1.HasAcceptanceCriteria, l1.RequiresToolCallFirst)
	}
	l2 := pvPromptLineOf(t, "sess-T36-rt2")
	if l2.HasAcceptanceCriteria == nil || *l2.HasAcceptanceCriteria ||
		l2.RequiresToolCallFirst == nil || !*l2.RequiresToolCallFirst {
		t.Errorf("布尔必须按**声明值**落盘（不是常量）：acc=%v tool=%v",
			l2.HasAcceptanceCriteria, l2.RequiresToolCallFirst)
	}
	// wire 层：键与值真的在原文里（读侧按 jq 查得到）
	raw := strings.Join(pvRawLinesOf(t, "sess-T36-rt1"), "\n")
	for _, frag := range []string{
		`"strategy_id":"` + sid + `"`,
		`"step_index":7`,
		`"has_acceptance_criteria":true`,
		`"requires_tool_call_first":false`,
	} {
		if !strings.Contains(raw, frag) {
			t.Errorf("wire 层缺 %s：%s", frag, raw)
		}
	}
}

// ── ④【反例】无脚手架信息 ⇒ 字段缺席（不得写 0/空串冒充）──────────────────

func TestPromptStrategyAbsentWhenUndeclared(t *testing.T) {
	pvIsolate(t)

	strategyKeys := []string{"strategy_id", "step_index", "has_acceptance_criteria", "requires_tool_call_first"}
	absence := func(raw, why string) {
		t.Helper()
		for _, k := range strategyKeys {
			if strings.Contains(raw, k) {
				t.Errorf("%s：%s 必须缺席（不得写 0/空串冒充）：%s", why, k, raw)
			}
		}
	}

	// 反例一：调用方**完全没声明**策略/名字/标签
	pvObserve(PromptRender{Session: "sess-T36-none", Round: 1, Model: "m-none", System: "模板\n" + "记忆块", Memory: "记忆块"})
	raw1 := strings.Join(pvRawLinesOf(t, "sess-T36-none"), "\n")
	if raw1 == "" {
		t.Fatal("这条观测没落盘")
	}
	absence(raw1, "没声明脚手架")
	for _, frag := range []string{`"prompt_name"`, `"prompt_label"`, `"prompt_label_versions"`} {
		if strings.Contains(raw1, frag) {
			t.Errorf("没声明名字/标签 ⇒ 键必须缺席（不是空串）：%s：%s", frag, raw1)
		}
	}
	// 版本键**必须**在（算得出来 ⇒ 不是编造）
	if !strings.Contains(raw1, `"prompt_version":"`) {
		t.Errorf("prompt_version 该有（渲染后模板可复算）：%s", raw1)
	}
	// read 层：指针全 nil（"没声明"与"声明了 false"可分）
	if l := pvPromptLineOf(t, "sess-T36-none"); l.StepIndex != nil || l.HasAcceptanceCriteria != nil || l.RequiresToolCallFirst != nil || l.StrategyID != "" {
		t.Errorf("没声明脚手架 ⇒ 读回必须是零值/缺席：%+v", l)
	}

	// 反例二：声明了策略，但**什么事实都没给**（且无会话 ⇒ 步序也数不出来）⇒ 四个键同样一个不落
	pvObserve(PromptRender{Session: "", Round: 1, Model: "m-empty-decl", System: "模板\n", Strategy: &PromptStrategyIn{}})
	raw2 := strings.Join(pvRawLinesOf(t, "m-empty-decl"), "\n")
	if raw2 == "" {
		t.Fatal("这条观测没落盘")
	}
	absence(raw2, "声明了空策略")
}

// ── ⑤ step_index 随会话轮次递增（会话级步序）────────────────────────────

func TestPromptStepIndexIncrementsPerSession(t *testing.T) {
	pvIsolate(t)

	sid := StrategyIDOf(PromptScaffoldSpec{Strategy: "chat.loop"})
	obs := func(session string, round, explicit int) {
		pvObserve(PromptRender{Session: session, Round: round, Model: "m",
			System: "模板\n", Strategy: &PromptStrategyIn{StrategyID: sid, StepIndex: explicit}})
	}
	obs("sess-T36-step-A", 1, 0)
	obs("sess-T36-step-A", 2, 0)
	obs("sess-T36-step-A", 3, 0)
	obs("sess-T36-step-B", 1, 0)  // 另一个会话：各自从 1 起
	obs("sess-T36-step-A", 4, 42) // 调用方给了显式值 ⇒ 用显式值

	steps := map[string][]int{}
	for _, ln := range readPromptObsLines(t) {
		if ln.EventName != obsEventConstraintCheck || ln.StepIndex == nil {
			continue
		}
		steps[ln.Session] = append(steps[ln.Session], *ln.StepIndex)
	}
	gotA := steps["sess-T36-step-A"]
	wantA := []int{1, 2, 3, 42}
	if len(gotA) != len(wantA) {
		t.Fatalf("A 会话步序条数不符：%v", gotA)
	}
	for i := range wantA {
		if gotA[i] != wantA[i] {
			t.Fatalf("A 会话步序应为 %v：got %v（必须随轮次递增；显式值优先）", wantA, gotA)
		}
	}
	if gotB := steps["sess-T36-step-B"]; len(gotB) != 1 || gotB[0] != 1 {
		t.Errorf("B 会话步序应从 1 起：%v", gotB)
	}
	// 计数**恒推进**（数的是"本会话装配了几次提示"）⇒ 显式值那一轮也计入（4 次装配 = 4 步）
	if n := PromptStepCountOf("sess-T36-step-A"); n != 4 {
		t.Errorf("A 会话应记 4 步（装配计数恒推进，显式值轮也计入）：%d", n)
	}
	// 反例：无会话 ⇒ 数不出步序（缺席，不写 0 冒充 —— 已由反例二覆盖，这里钉住 API 层口径）
	if PromptStepIndex("") != 0 {
		t.Error("无会话 ⇒ PromptStepIndex 返回 0（缺席），不编造")
	}
}
