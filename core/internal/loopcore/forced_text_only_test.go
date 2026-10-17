// forced_text_only_test.go — GAP-20261017-13（P0）：末轮带工具调用 ⇒ 补一轮「只许文本」的回归牙
//
// 症状（实测两次）：末轮 content 带 tool call ⇒ 工具执行完循环就到头了 ⇒ 模型**再无轮次**写结论
// （一次是名册没写上、一次是五行结论没交出来）。四条用例全部走**真循环**（假推断器，不是只调判据函数）：
//
//	㋐ TestForcedTextOnly_LastRoundToolCallGetsOneTextRound — 末轮带工具调用 ⇒ 补一轮、正文含结论、补轮无工具
//	㋑ TestForcedTextOnly_MidRoundToolCallNotCut         — rounds < maxRounds 带工具调用 ⇒ 不补、正常继续
//	㋒ TestForcedTextOnly_TextOnLastRoundRunsNoExtra     — 末轮直接给文本 ⇒ 不补、infer 次数严格
//	㋓ TestForcedTextOnly_SecondToolCallIsNotExecuted    — 补轮又发工具调用 ⇒ 不执行、不再补、标 forced_text_only
//
// 负控（改前版本）：把 run.go 用 -overlay 映回改前态跑本件 ⇒ ㋐ 与 ㋓ 必红（infer 次数少一轮 / 结论缺失）。
package loopcore

import (
	"context"
	"strings"
	"testing"
)

// inferStep — 一发推理的现场：模型看到的 messages ＋ 这一发给它的 tools（判「补轮有没有带工具」用它）。
type inferStep struct {
	msgs  []map[string]any
	tools []map[string]any
}

// scriptInferSteps — 逐轮脚本化 ＋ 记录每发的现场（resp 用尽后重复末条，与既有 mockInfer 同口径）。
func scriptInferSteps(resp []Response) (Infer, *[]inferStep) {
	steps := &[]inferStep{}
	i := 0
	infer := func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*Response, error) {
		*steps = append(*steps, inferStep{msgs: msgs, tools: tools})
		r := resp[min(i, len(resp)-1)]
		i++
		out := r
		return &out, nil
	}
	return infer, steps
}

func forcedTestTools() []map[string]any {
	return []map[string]any{{"type": "function", "function": map[string]any{"name": "zerg"}}}
}

// forcedTestExec — 计数 ＋ 固定结果（工具结果要能被正文引用，才判得出「结论是在工具结果之后写出来的」）。
func forcedTestExec(n *int) func(context.Context, string, map[string]any) (string, string, error) {
	return func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		*n++
		return "1133: case \"chat\": return cmdChat(inv)", "1ms", nil
	}
}

const forcedTestConclusion = "① want：agent show 输出「请求次数」与「来源」两列 " +
	"② 命令：zerg code show core/cmd/zerg/main.go:1133 ③ 该行是 chat 子命令入口、不含这两列 " +
	"④ 结论：仍缺 ⑤ 缺的工具：无"

// hintSeenIn — 某一发里有没有注入「工具已执行完毕…不要再调工具」这条 user 提示
func hintSeenIn(step inferStep) bool {
	for _, m := range step.msgs {
		if c, _ := m["content"].(string); strings.Contains(c, "工具已执行完毕，现将结果附上") &&
			strings.Contains(c, "不要再调工具") {
			return true
		}
	}
	return false
}

// ㋐ 末轮带 tool call ⇒ 执行它、补一轮纯文本，最终正文是结论（不是「工具跑了」）。
func TestForcedTextOnly_LastRoundToolCallGetsOneTextRound(t *testing.T) {
	infer, steps := scriptInferSteps([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"), // 这一轮就是最后一轮（MaxRounds=2），且它带工具调用
		{Content: forcedTestConclusion},
	})
	execN := 0
	res := Run(context.Background(), Config{MaxRounds: 2}, "m", "sys", nil, Deps{
		Infer: infer, Exec: forcedTestExec(&execN), Tools: forcedTestTools(),
	})
	if len(*steps) != 3 {
		t.Fatalf("末轮带工具调用后应补一轮（infer 3 次：2 轮 + 补 1 轮），实得 %d 次 —— 模型又没机会写结论", len(*steps))
	}
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("末轮的工具调用必须真执行（不许被截）：exec=%d traces=%d", execN, len(res.Traces))
	}
	if !strings.Contains(res.Content, "结论：仍缺") {
		t.Fatalf("交付正文应是结论，实得 %q（补轮没把结论文本带出来）", res.Content)
	}
	last := (*steps)[2]
	if len(last.tools) != 0 {
		t.Fatalf("补轮必须**只许文本**（tools 应为空），实得 %d 个工具", len(last.tools))
	}
	if !hintSeenIn(last) {
		t.Fatal("补轮那一发里没有注入「工具已执行完毕…不要再调工具，直接给结论」提示 —— 模型不可能知道该收尾")
	}
	// 轮次预算确实用尽 ⇒ 终局口径仍是 max_rounds（不虚报 natural；正文照样是结论）
	if res.ExitKind != "max_rounds" {
		t.Fatalf("补轮吃的是最后一轮预算 ⇒ 终局应为 max_rounds，实得 %q", res.ExitKind)
	}
}

// ㋑ 反向断言②：还有轮次时带工具调用 ⇒ 不补、正常继续（不能每轮都补）。
func TestForcedTextOnly_MidRoundToolCallNotCut(t *testing.T) {
	infer, steps := scriptInferSteps([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		{Content: forcedTestConclusion},
	})
	execN := 0
	res := Run(context.Background(), Config{MaxRounds: 8}, "m", "sys", nil, Deps{
		Infer: infer, Exec: forcedTestExec(&execN), Tools: forcedTestTools(),
	})
	if len(*steps) != 3 {
		t.Fatalf("非末轮不该补轮：应 3 次（工具 / 工具 / 结论），实得 %d 次", len(*steps))
	}
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("两轮工具都应真执行：exec=%d traces=%d", execN, len(res.Traces))
	}
	for i, st := range *steps {
		if hintSeenIn(st) {
			t.Fatalf("第 %d 发被注入了强制收尾提示 —— 还有轮次时不许误截", i+1)
		}
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "结论：仍缺") {
		t.Fatalf("正常收尾被改动：kind=%q content=%q", res.ExitKind, res.Content)
	}
}

// ㋒ 反向断言①：末轮直接给文本结论 ⇒ 不多跑（infer 次数严格等于轮数）。
func TestForcedTextOnly_TextOnLastRoundRunsNoExtra(t *testing.T) {
	infer, steps := scriptInferSteps([]Response{
		zergToolCall("c1"),
		{Content: forcedTestConclusion}, // 最后一轮（MaxRounds=2）直接给文本
	})
	execN := 0
	res := Run(context.Background(), Config{MaxRounds: 2}, "m", "sys", nil, Deps{
		Infer: infer, Exec: forcedTestExec(&execN), Tools: forcedTestTools(),
	})
	if len(*steps) != 2 {
		t.Fatalf("末轮直接给文本 ⇒ 必须恰好 2 次推理（不许多跑），实得 %d 次", len(*steps))
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "结论：仍缺") {
		t.Fatalf("正常收尾被改动：kind=%q content=%q", res.ExitKind, res.Content)
	}
	for i, st := range *steps {
		if hintSeenIn(st) {
			t.Fatalf("第 %d 发被注入强制收尾提示 —— 正常交付不该被推着再跑", i+1)
		}
	}
}

// ㋓ 反向断言③：补轮又发工具调用 ⇒ 不执行、不再补，标 forced_text_only（绝不死循环）。
func TestForcedTextOnly_SecondToolCallIsNotExecuted(t *testing.T) {
	infer, steps := scriptInferSteps([]Response{
		zergToolCall("c1"),
		zergToolCall("c2"),
		zergToolCall("c3"), // 补轮仍然要工具 —— 必须被挡在「不执行」这一侧
	})
	execN := 0
	res := Run(context.Background(), Config{MaxRounds: 2}, "m", "sys", nil, Deps{
		Infer: infer, Exec: forcedTestExec(&execN), Tools: forcedTestTools(),
	})
	if len(*steps) != 3 {
		t.Fatalf("补轮只许一次：共 3 次推理（2 轮 + 补 1 轮），实得 %d 次 —— 有死循环风险", len(*steps))
	}
	if execN != 2 || len(res.Traces) != 2 {
		t.Fatalf("补轮请求的工具**不许执行**：exec=%d traces=%d（want 2/2）", execN, len(res.Traces))
	}
	if !strings.Contains(res.Content, ForcedTextOnlyMark) {
		t.Fatalf("补轮又发工具调用时结果必须标 %q，实得 %q", ForcedTextOnlyMark, res.Content)
	}
	if res.ExitKind != "max_rounds" {
		t.Fatalf("终局应为 max_rounds，实得 %q", res.ExitKind)
	}
}

// 判据件的两态表：补轮仍请求工具时交付正文怎么取（有正文用它 / 空则退回本轮 / 全空则如实说明）。
func TestForcedTextOnlyContentTable(t *testing.T) {
	if got := forcedTextOnlyContent("补轮正文", "本轮正文"); got != "补轮正文" {
		t.Errorf("有补轮正文时应优先用它，实得 %q", got)
	}
	if got := forcedTextOnlyContent("  ", "本轮正文"); got != "本轮正文" {
		t.Errorf("补轮正文为空时应退回本轮正文，实得 %q", got)
	}
	got := forcedTextOnlyContent("", "")
	if strings.TrimSpace(got) == "" || strings.Contains(got, "结论：") {
		t.Errorf("两处都空时只能如实说明、不许编造结论，实得 %q", got)
	}
}
