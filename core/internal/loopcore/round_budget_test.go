// round_budget_test.go — GAP-20261018-23：轮数预算可配 ＋ 提前一轮提示 的两态回归牙
//
// 两态（验收①）：
//   - 预算内正常多步：不注入、不多跑（infer 次数 = 步数 + 1，正文是结论）；
//   - 预算将尽：**提前一轮**（还剩 2 轮含本轮时）注入预算提示，且**不硬砍正在跑的工具调用**
//     （那一轮的工具照样执行、轨迹照样落，下一轮才收尾）。
//
// 负控（改前版本）：把 run.go 映回改前态跑本件 ⇒ 头两条必红（第 1 条请求里没有提示、
// 最后一轮的开工提示不存在），读数正是缺口本身。
package loopcore

import (
	"context"
	"strings"
	"testing"
)

func budgetTestTools() []map[string]any {
	return []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}
}

func hasBudgetWarn(msgs []map[string]any) bool {
	for _, m := range msgs {
		if c, _ := m["content"].(string); strings.Contains(c, RoundBudgetWarnMark) {
			return true
		}
	}
	return false
}

// TestResolveRoundBudgetTable — 旋钮的三态：默认 / 显式配置 / env 覆盖（含非法值不覆盖）。
func TestResolveRoundBudgetTable(t *testing.T) {
	cases := []struct {
		cfg  int
		env  string
		want int
	}{
		{0, "", DefaultMaxRounds},  // 未给预算 ⇒ 默认 10（与改前逐字同）
		{-3, "", DefaultMaxRounds}, // 非法配置 ⇒ 默认
		{8, "", 8},                 // 调用方显式给 ⇒ 用它
		{8, "20", 20},              // env 覆盖
		{8, " 6 ", 6},              // 前后空白照样认
		{8, "abc", 8},              // 非数字 ⇒ 不覆盖
		{8, "0", 8},                // ≤0 ⇒ 不覆盖
		{8, "-4", 8},               // 负数 ⇒ 不覆盖
		{0, "3", 3},                // 未给 + env ⇒ env
	}
	for _, c := range cases {
		if got := ResolveRoundBudget(c.cfg, c.env); got != c.want {
			t.Errorf("ResolveRoundBudget(%d, %q)=%d，want %d", c.cfg, c.env, got, c.want)
		}
	}
}

// TestRoundBudgetWarnWording — 提示必须自报来源、说清「还剩两轮」与「把手上这步做完」，
// 且**不得**与强制交答案那条同形（同一句话既说「不要调工具」又说「把这步做完」会自相矛盾）。
func TestRoundBudgetWarnWording(t *testing.T) {
	for _, want := range []string{"系统提示", "不是用户的新要求", "只剩 2 轮", "手上这一步", "做完", "下一轮请直接给出结论"} {
		if !strings.Contains(RoundBudgetWarnHint, want) {
			t.Fatalf("预算提示丢了含义 %q：%q", want, RoundBudgetWarnHint)
		}
	}
	if strings.Contains(RoundBudgetWarnHint, "不要再调工具") {
		t.Fatalf("预算提示与强制交答案那条同形（含「不要再调工具」）⇒ 模型只能读到硬停：%q", RoundBudgetWarnHint)
	}
	// 两条提示互不认：混用会把这条当那条剥掉 / 把那条当这条留下。
	if IsForcedAnswerHint(roundBudgetWarnMsg()) {
		t.Fatal("预算提示被认成了强制交答案提示（生命周期会串）")
	}
	if IsRoundBudgetWarnHint(forcedAnswerHintMsg()) {
		t.Fatal("强制交答案提示被认成了预算提示（生命周期会串）")
	}
	if ForcedAnswerHint != "" && strings.Contains(RoundBudgetWarnHint, ForcedAnswerHintSourceMark) {
		t.Fatal("预算提示用了 ForcedAnswerHint 的来源标注（IsForcedAnswerHint 会把它当那条剥掉）")
	}
}

// TestRoundBudget_NormalNoWarn — 两态之一：预算内正常多步 ⇒ 不注入、不多跑。
func TestRoundBudget_NormalNoWarn(t *testing.T) {
	execN := 0
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: "① 取到 want ② 跑了 zerg code find/code show ③ 两列都不存在 ④ 结论：仍缺"},
	})
	d := Deps{Infer: infer, Tools: budgetTestTools(),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execN++
			return "（工具结果）", "1ms", nil
		}}
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 3 || execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("预算内应 3 轮 / 2 次执行 / 2 条轨迹，实得 infer=%d exec=%d traces=%d", len(*calls), execN, len(res.Traces))
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "仍缺") {
		t.Fatalf("正常收尾被改动：kind=%q content=%q", res.ExitKind, res.Content)
	}
	for i, ms := range *seen {
		if hasBudgetWarn(ms) {
			t.Fatalf("第 %d 轮被注入了预算提示 —— 预算还够（8 轮）时不该推着模型收尾", i+1)
		}
	}
}

// TestRoundBudget_WarnOneRoundEarly — 两态之二：预算 3 轮、任务两步 ⇒
// 第 2 轮（＝最后一轮的前一轮）那一发请求里带提示；那一轮的工具**照样执行**；第 3 轮正常交结论。
func TestRoundBudget_WarnOneRoundEarly(t *testing.T) {
	execN := 0
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: "① 取到 want ② 跑了 zerg code find 与 zerg code show ③ 两列都不存在 ④ 结论：仍缺"},
	})
	d := Deps{Infer: infer, Tools: budgetTestTools(),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execN++
			return "（工具结果）", "1ms", nil
		}}
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, d)
	if len(*calls) != 3 {
		t.Fatalf("应 3 轮（两步工具 + 结论），实得 %d", len(*calls))
	}
	if hasBudgetWarn((*seen)[0]) {
		t.Fatal("第 1 轮（还有 2 轮余量之外）就注入了预算提示 —— 应当只在将尽那一轮提前一轮注入")
	}
	if !hasBudgetWarn((*seen)[1]) {
		t.Fatal("第 2 轮（最后一轮的前一轮）没有预算提示 —— 模型只会在最后一轮被硬砍")
	}
	if hasBudgetWarn((*seen)[2]) {
		t.Fatal("第 3 轮的请求里还带着预算提示 —— 它只服务注入它的那一发")
	}
	// 不硬砍正在跑的工具调用：提示那一轮的工具必须真执行、真落轨迹。
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("预算将尽那一轮的工具调用被砍了：exec=%d traces=%d（应 2/2）", execN, len(res.Traces))
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "仍缺") {
		t.Fatalf("提前一轮提示后应正常交结论：kind=%q content=%q", res.ExitKind, res.Content)
	}
}

// TestRoundBudget_LastRoundToolStillExecutes — 预算 2 轮、两步任务 ⇒
// 提示在第 1 轮就送出；第 2 轮的工具**仍然执行**（只补一轮只许文本，不重拉、不丢结果）。
func TestRoundBudget_LastRoundToolStillExecutes(t *testing.T) {
	execN := 0
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: "① want ② 跑了 zerg code find 与 zerg code show ③ 零命中 ④ 结论：仍缺"},
	})
	d := Deps{Infer: infer, Tools: budgetTestTools(),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			execN++
			return "（工具结果）", "1ms", nil
		}}
	res := Run(context.Background(), Config{MaxRounds: 2}, "m", "sys", nil, d)
	if len(*calls) != 3 {
		t.Fatalf("末轮工具 + 只许文本补轮 = 3 次推理，实得 %d", len(*calls))
	}
	if !hasBudgetWarn((*seen)[0]) {
		t.Fatal("预算 2 轮时第 1 轮就该看到预算提示（提前一轮的唯一可能位置）")
	}
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("末轮工具调用必须执行：exec=%d traces=%d（应 2/2）", execN, len(res.Traces))
	}
	if res.ExitKind != "max_rounds" {
		t.Fatalf("预算确实用尽 ⇒ 仍记 max_rounds（不虚报 natural），实得 %q", res.ExitKind)
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Fatal("补轮后给了空产出 —— 已有内容不该丢")
	}
}

// TestRoundBudget_EnvOverrideReachesRun — 旋钮真的接到 Run：env=2 + 配置 8 ⇒ 第 1 轮就提示。
func TestRoundBudget_EnvOverrideReachesRun(t *testing.T) {
	t.Setenv(RoundBudgetEnv, "2")
	infer, calls, seen := scriptInfer([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: "① want ② 命令 ③ 零命中 ④ 结论：仍缺"},
	})
	d := Deps{Infer: infer, Tools: budgetTestTools(), Exec: okExec}
	_ = Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, d)
	if len(*calls) != 3 {
		t.Fatalf("env 覆盖成 2 轮 ⇒ 两步任务必然走满预算（3 次推理），实得 %d", len(*calls))
	}
	if !hasBudgetWarn((*seen)[0]) {
		t.Fatal("env 旋钮没接到 Run（预算仍是 8 ⇒ 第 1 轮不该提示）")
	}
}

// TestRoundBudget_HintDroppedOnEntry — 历史里的残留（旧过程崩在注入那一轮）不得带进本程第一发。
func TestRoundBudget_HintDroppedOnEntry(t *testing.T) {
	msgs := []map[string]any{
		{"role": "user", "content": "上一程的任务"},
		roundBudgetWarnMsg(),
	}
	infer, _, seen := scriptInfer([]Response{{Content: "系统正常。"}})
	d := Deps{Infer: infer, Tools: budgetTestTools(), Exec: okExec}
	_ = Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", msgs, d)
	if hasBudgetWarn((*seen)[0]) {
		t.Fatal("进 Run 没剥掉历史里的预算提示 —— 模型会把上一程的预算当本程的命令")
	}
	if _, n := DropRoundBudgetWarnHints(append(append([]map[string]any(nil), msgs...), forcedAnswerHintMsg())); n != 1 {
		t.Fatalf("DropRoundBudgetWarnHints 应只剥 1 条预算提示（不碰强制交答案那条），实剥 %d", n)
	}
}
