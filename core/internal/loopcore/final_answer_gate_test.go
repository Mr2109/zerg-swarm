// final_answer_gate_test.go — GAP-20261017-12（P0）：最终轮不交答案的回归牙
//
// 三条正向（真循环，不是只调判据函数）：
//   - 打算收尾 ⇒ 补一轮、最终 Content 是结论；
//   - 正常多轮（工具 → 结论）⇒ **不多跑**（infer 次数 = 2）；
//   - 只给一次机会 ⇒ 第二次仍写打算也不再追加（次数 = 3，不死循环）；
//   - 真要继续查 ⇒ 带工具调用的那一轮**不被截**（两轮工具都执行、轨迹 2 条）。
//
// 负控（改前版本）：把 run.go 映回改前态跑本件 ⇒ 头一条必红（Content 是「让我…」、次数 2）。
package loopcore

import (
	"context"
	"strings"
	"testing"
)

// scriptInfer — 逐轮脚本化 ＋ 记录「每轮模型看到的 messages」（用来断言注入的提示确实到了模型手里）
func scriptInfer(resp []Response) (Infer, *[]Response, *[][]map[string]any) {
	calls := &[]Response{}
	seen := &[][]map[string]any{}
	i := 0
	infer := func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*Response, error) {
		cp := msgs
		*seen = append(*seen, cp)
		r := resp[min(i, len(resp)-1)]
		i++
		out := r
		*calls = append(*calls, out)
		return &out, nil
	}
	return infer, calls, seen
}

func zergToolCall(id string) Response {
	return Response{ToolCalls: []ToolCall{{
		ID: id, Name: "zerg", Args: map[string]any{"command": "zerg code find recentRequests"},
		RawArgs: `{"command":"zerg code find recentRequests"}`,
	}}}
}

func okExec(ctx context.Context, name string, args map[string]any) (string, string, error) {
	return "零命中（真读到的结果）", "3ms", nil
}

// TestForcedAnswer_PlanIsNotDelivery — 头号症状：natural 但正文是「让我…」。
func TestForcedAnswer_PlanIsNotDelivery(t *testing.T) {
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		{Content: "零命中。看 family_agent.go 第62行附近，以及 agent show 实现里有没有「请求次数」「来源」列。让我看源码上下文。"},
		{Content: "① want：agent show 输出「请求次数」与「来源」两列 ② 取证命令：zerg gap show GAP-20261010-16／zerg code find recentRequests " +
			"／zerg code show core/cmd/zerg/family_agent.go:62 ③ 两列都不存在 ④ 结论：仍缺 ⑤ 缺的工具：无"},
	})
	d := Deps{Infer: infer, Exec: okExec, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", []map[string]any{{"role": "user", "content": "查 GAP"}}, d)
	if len(*calls) != 3 {
		t.Fatalf("打算收尾后应补一轮（infer 3 次），实得 %d 次", len(*calls))
	}
	if res.ExitKind != "natural" {
		t.Fatalf("补轮后应自然收尾，实得 ExitKind=%q", res.ExitKind)
	}
	if strings.Contains(res.Content, "让我看源码上下文") {
		t.Fatalf("交付正文里仍是「打算」：%q", res.Content)
	}
	if !strings.Contains(res.Content, "仍缺") || !strings.Contains(res.Content, "① ") {
		t.Fatalf("交付正文应是结论：%q", res.Content)
	}
	gotHint := false
	for _, m := range (*seen)[2] {
		if c, _ := m["content"].(string); strings.Contains(c, ForcedAnswerHintSourceMark) {
			gotHint = true
		}
	}
	if !gotHint {
		t.Fatal("补轮那一发里没有注入「停手交答案」提示 —— 模型不可能知道要交结论")
	}
}

// TestForcedAnswer_NormalRoundDoesNotRunExtra — 反向断言①：正常多轮不多跑一轮。
func TestForcedAnswer_NormalRoundDoesNotRunExtra(t *testing.T) {
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		{Content: "① want：agent show 输出「请求次数」与「来源」两列。② 取证命令：zerg code show core/cmd/zerg/family_agent.go:62。" +
			"③ 源码里这两列都不存在。④ 结论：仍缺。⑤ 缺的工具：无。"},
	})
	d := Deps{Infer: infer, Exec: okExec, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 2 {
		t.Fatalf("正常「工具→结论」应恰好 2 轮（不额外多跑），实得 %d 轮", len(*calls))
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "结论：仍缺") {
		t.Fatalf("正常收尾被改动：kind=%q content=%q", res.ExitKind, res.Content)
	}
	for i, ms := range *seen {
		for _, m := range ms {
			if c, _ := m["content"].(string); strings.Contains(c, ForcedAnswerHintSourceMark) {
				t.Fatalf("第 %d 轮被注入了强制交答案提示 —— 正常交付不该被推着再跑", i+1)
			}
		}
	}
}

// TestForcedAnswer_OnlyOneExtraRound — 反向断言③：只给一次机会，不死循环。
func TestForcedAnswer_OnlyOneExtraRound(t *testing.T) {
	plan := Response{Content: "让我再查一下 agent show 的实现，然后再说结论。"}
	infer, calls, _ := scriptInfer([]Response{zergToolCall("c1"), plan, plan, plan, plan})
	d := Deps{Infer: infer, Exec: okExec, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 3 {
		t.Fatalf("只许补一轮（共 3 次），实得 %d 次 —— 有死循环风险", len(*calls))
	}
	if res.ExitKind != "natural" {
		t.Fatalf("补过一轮后应照旧收尾，实得 %q", res.ExitKind)
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Fatal("第二次仍不交时不得把正文掏空（应退回那份「打算」）")
	}
}

// TestForcedAnswer_ToolRoundsNotCut — 反向断言②：真要继续查时不被误截。
func TestForcedAnswer_ToolRoundsNotCut(t *testing.T) {
	infer, calls, _ := scriptInfer([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: "① want：两列。② 命令：zerg code show core/cmd/zerg/family_agent.go:62。③ 两列都不存在。④ 仍缺。⑤ 无。"},
	})
	execN := 0
	d := Deps{Infer: infer, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execN++
			return "（工具结果）", "1ms", nil
		}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("要继续查的两轮工具必须都执行：exec=%d traces=%d", execN, len(res.Traces))
	}
	if len(*calls) != 3 {
		t.Fatalf("应 3 轮（两轮工具 + 一轮结论），实得 %d", len(*calls))
	}
	if !strings.Contains(res.Content, "仍缺") {
		t.Fatalf("结论缺失：%q", res.Content)
	}
}

// TestForcedAnswer_TerseDeliveryNotForced — 反向断言①的极端格：一字真答也不许被推着重答。
// 本闸第一版按「正文太短」也判未交付 ⇒ 既有 3 条用例的终答「续跑完成」「系统正常」当场红，
// 这条路正是那次的回归牙：短 ≠ 没交付。
func TestForcedAnswer_TerseDeliveryNotForced(t *testing.T) {
	infer, calls, _ := scriptInfer([]Response{
		zergToolCall("c1"),
		{Content: "零命中。"},
	})
	d := Deps{Infer: infer, Exec: okExec, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 2 {
		t.Fatalf("极短但真实的交付不该被补轮，实得 %d 轮", len(*calls))
	}
	if res.ExitKind != "natural" || res.Content != "零命中。" {
		t.Fatalf("短答被改动：kind=%q content=%q", res.ExitKind, res.Content)
	}
}

// TestNeedsForcedAnswerTable — 判据本身的两态表（正例 / 反例都要钉住）。
func TestNeedsForcedAnswerTable(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"空正文", "   ", true},
		{"打算:让我", "零命中。让我看源码上下文。", true},
		{"打算:开头下一步", "下一步我准备去读 family_agent.go。", true},
		{"打算:let me", "Let me check the source first.", true},
		{"结论:五行", "① want：两列 ② 命令：zerg code show x:62 ③ 两列不存在 ④ 仍缺 ⑤ 无", false},
		{"结论末尾带建议(不误判)", "①②③④ 结论：仍缺。", false},
		// 极短句**是**合法交付（实测既有用例的终答就是「续跑完成」这类）⇒ 长度不作判据
		{"极短终答不误判", "续跑完成", false},
		{"零命中一句话交付", "零命中。", false},
	}
	for _, c := range cases {
		got, why := NeedsForcedAnswer(c.content)
		if got != c.want {
			t.Errorf("%s：NeedsForcedAnswer=%v(why=%q)，want %v", c.name, got, why, c.want)
		}
	}
}

// ── GAP-20261018-06（P0）：注入提示的来源归属 + 跨轮清理 ──────────────────────────
//
// 症状（铁证 · 同会话连发两条任务）：第一条真跑 4 次工具；触发一次 `loop_hint forced_answer`
// 之后，模型自述「**用户第一条消息即要求停手、不再调用工具**」，整轮零命令零事实；再发任务
// 仍一条命令未跑 ⇒ 旧形态把一条 **role=user** 的内核提示留在 `msgs` 里**跨轮生效**。
// 本组三条牙：② 文案（含来源标注、无「停手」）③ 同一程内不留到下一位 ／ 跨任务进来先剥。

// hasForcedHint — 按**来源标注**认（不看 role：万一有人把角色改成 system，这条牙也要能认出来）。
func hasForcedHint(msgs []map[string]any) bool {
	for _, m := range msgs {
		if c, _ := m["content"].(string); strings.Contains(c, ForcedAnswerHintSourceMark) {
			return true
		}
	}
	return false
}

// TestForcedAnswerHintWording — 验收②：注入的那条消息必须写明「来源是系统」，且**不得**出现「停手」。
func TestForcedAnswerHintWording(t *testing.T) {
	if !strings.Contains(ForcedAnswerHint, "系统提示") {
		t.Fatalf("注入文案没写来源（须含「系统提示」）：%q", ForcedAnswerHint)
	}
	if !strings.Contains(ForcedAnswerHint, "不是用户") {
		t.Fatalf("注入文案没否定「用户的新要求」：%q", ForcedAnswerHint)
	}
	if strings.Contains(ForcedAnswerHint, "停手") {
		t.Fatalf("注入文案仍含「停手」（会被读成用户下达的命令）：%q", ForcedAnswerHint)
	}
	// 含义四条：不调工具 / 给结论 / 报白不编 / 禁「下一步打算」——改写文案不许把它们丢掉。
	for _, want := range []string{"不要再调工具", "已经拿到的事实与结论", "直说没查到", "不要写「下一步打算"} {
		if !strings.Contains(ForcedAnswerHint, want) {
			t.Fatalf("文案改丢了含义 %q：%q", want, ForcedAnswerHint)
		}
	}
	// 角色：保留 user（依据见常量注释 —— 上游 system 须唯一在开头，中途插 system 无兼容保证）。
	if ForcedAnswerHintRole != "user" {
		t.Fatalf("注入角色是 %q —— 中途插 system 无上游兼容保证，且退出验收口径", ForcedAnswerHintRole)
	}
	if got, _ := forcedAnswerHintMsg()["role"].(string); got != "user" {
		t.Fatalf("注入消息的角色是 %q，want \"user\"", got)
	}
}

// TestForcedAnswerHintNotLeakedToNextRequest — 验收③（同一程内）：补轮那一发看得到提示，
// 它**之后**的请求一律看不到（旧形态会一直躺在 msgs 里，模型下一轮仍当用户命令读）。
func TestForcedAnswerHintNotLeakedToNextRequest(t *testing.T) {
	// 四发：① 工具 ② 打算（触发闸并注入）③ 补轮无视提示、又调工具 ④ 结论
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		{Content: "零命中。让我再看看 family_agent.go 的实现。"},
		zergToolCall("c2"),
		{Content: "① want：两列 ② 命令：zerg code show x:62 ③ 两列不存在 ④ 仍缺 ⑤ 无"},
	})
	d := Deps{Infer: infer, Exec: okExec, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}}
	Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 4 {
		t.Fatalf("脚本应跑满 4 发（工具/打算/补轮工具/结论），实得 %d", len(*calls))
	}
	if !hasForcedHint((*seen)[2]) {
		t.Fatal("补轮那一发没有拿到提示 —— 模型不可能知道要交结论（闸被改坏了）")
	}
	if hasForcedHint((*seen)[3]) {
		t.Fatal("补轮**之后**的请求仍带着这条提示 —— 同程跨轮污染没治住")
	}
}

// TestForcedAnswerHintStrippedOnEntry — 验收③（跨任务）：上一程遗留在历史里的提示喂进来，
// 本程**第一发**请求就不得再看到它；且闸本身照旧能用（该轮仍注入一次新提示）。
func TestForcedAnswerHintStrippedOnEntry(t *testing.T) {
	old := []map[string]any{
		{"role": "user", "content": "第一条任务"},
		{"role": "assistant", "content": "① 结论：仍缺。"},
		{"role": "user", "content": ForcedAnswerHint}, // 上一程遗留（旧形态会一直躺着）
		{"role": "assistant", "content": "① 结论：仍缺。"},
		{"role": "user", "content": "第二条任务"},
	}
	// 第二任务的第一发**真调工具**（这正是旧形态做不到的事：模型把遗留提示读成
	// 「用户要求停手」⇒ 一条命令都不跑）⇒ 这里必须真的执行到工具。
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("t1"),
		{Content: "让我先看看上一轮的结论。"},
		{Content: "① want：两列 ② 命令：zerg code find recentRequests ③ 零命中 ④ 仍缺 ⑤ 无"},
	})
	execN := 0
	d := Deps{Infer: infer, Tools: []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execN++
			return "零命中（真读到的结果）", "3ms", nil
		}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", old, d)
	if hasForcedHint((*seen)[0]) {
		t.Fatal("本程第一发请求仍看到上一程遗留的提示 —— 跨任务污染没治住（模型会照旧当用户命令读）")
	}
	if execN != 1 || len(res.Traces) != 1 {
		t.Fatalf("第二任务必须真调工具（旧形态零命令）：exec=%d traces=%d", execN, len(res.Traces))
	}
	if len(*calls) != 3 {
		t.Fatalf("第二任务应 3 发（工具→打算→补轮结论），实得 %d", len(*calls))
	}
	if !hasForcedHint((*seen)[2]) {
		t.Fatal("本程触发 forced_answer 后没有注入提示 —— 闸被改坏了")
	}
	if hasForcedHint((*seen)[1]) {
		t.Fatal("工具轮就看到了提示 —— 它只该出现在补轮那一发")
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "仍缺") {
		t.Fatalf("补轮后应自然收尾且正文是结论：kind=%q content=%q", res.ExitKind, res.Content)
	}
}

// TestForcedAnswerHintNotInCheckpoint — 快照是「**可续跑的会话历史**」⇒ 里面不许有这条提示
// （续跑/下一程读到它就会复现「用户要求停手」）。
func TestForcedAnswerHintNotInCheckpoint(t *testing.T) {
	st, err := NewCheckpointStore(t.TempDir(), DurabilitySync)
	if err != nil {
		t.Fatalf("建检查点存储失败：%v", err)
	}
	infer, calls, _ := scriptInfer([]Response{
		zergToolCall("c1"),
		{Content: "让我再查一下。"},
		{Content: "① want：两列 ② 命令：zerg code show x:62 ③ 两列不存在 ④ 仍缺 ⑤ 无"},
	})
	d := Deps{Infer: infer, Exec: okExec, Checkpoints: st, RunID: "run-gap06"}
	Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 3 {
		t.Fatalf("应 3 发（工具/打算/补轮结论），实得 %d", len(*calls))
	}
	snap, err := st.Load("run-gap06")
	if err != nil {
		t.Fatalf("读快照失败：%v", err)
	}
	if hasForcedHint(snap.State.Messages) {
		t.Fatalf("快照里留着强制答题提示（%d 条消息）—— 续跑会被它毒化", len(snap.State.Messages))
	}
}
