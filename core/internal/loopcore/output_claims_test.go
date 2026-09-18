// output_claims_test.go — B 项⑥（B6）实证：**引用命中校验挂在成员产出路径上**。
//
// 取证纪律（与 sendback_code_test.go / slice_events_test.go 同一套）：
//
//	· 裁决用**真实**的四态裁决（`VerifyClaims`，一行没改）算，不手搓假结果；
//	· 终局用**真实的**四终局选择（`ChooseTerminal` + `TerminalNote`，一行没改）判，不手搓状态名；
//	· 执行走**真实的** `Run`（成员执行循环——`internal/agent/ca_loop.go:215` RunWithKernel 调的就是它），
//	  不是"调一下辅助函数"就完事。
//
// 覆盖：
//
//	① 产出引用**全中** ⇒ 正常交付（不打回、不改道：正文逐字节等于接线前）
//	② 产出引用**有假** ⇒ 按**既有四终局**处理（假回执+有回执 ⇒ degrade；假回执+零进展 ⇒ escalate），
//	   且**退出原因不变**（不新造状态名）
//	③ 产出**根本没引用 / 空产出** ⇒ 行为明确且不崩（零回归：正文逐字节等于接线前）
//	④ 边界：`revise`（缺引用）**不算**命中失败（判据闭集 = {retract, send_back}）
//	⑤ 边界：回执集口径（无 ID 的轨迹不收；空原文的轨迹照收 ⇒ 判「无据」而非「假回执」）
package loopcore

import (
	"context"
	"strings"
	"testing"
)

// ── 用例装置：真实 Run（成员交付轮：round1 调一次工具 → round2 交正文）──────────────

const memberExecOut = "exec.go:12: x := strings.TrimSpace(s)"

// runMemberOneToolThenDeliver — 成员跑一轮工具后交正文（自然收尾）。
func runMemberOneToolThenDeliver(t *testing.T, claims []Claim, maxRounds int) *Result {
	t.Helper()
	infer, calls := mockInfer([]Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "read", Args: map[string]any{"path": "exec.go"}, RawArgs: `{"path":"exec.go"}`}}},
		{Content: "已完成"},
	})
	res := Run(context.Background(), Config{MaxRounds: maxRounds}, "m", "sys", nil, Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return memberExecOut, "3ms", nil
		},
		OutputClaims: claims,
	})
	if *calls < 2 {
		t.Fatalf("用例装置不成立：推理只被调用 %d 次（应≥2：调工具一轮 + 交正文一轮）", *calls)
	}
	return res
}

// ── ① 产出引用全中 ⇒ 正常交付（不打回、不改道）────────────────────────────────

func TestOutputClaims_AllHitDeliversNormally(t *testing.T) {
	claims := []Claim{{Text: "我读了 exec.go 并看到了 TrimSpace", ReceiptID: "1", Quote: "strings.TrimSpace(s)"}}
	res := runMemberOneToolThenDeliver(t, claims, 3)

	if res.ExitKind != "natural" {
		t.Fatalf("产出引用全中 ⇒ 正常收尾，实际 kind=%q", res.ExitKind)
	}
	// 零改道：正文**逐字节**等于接线前（接线前自然收尾不带任何终局说明）
	if res.Content != "已完成" {
		t.Fatalf("引用全中不得动产出正文：得到 %q", res.Content)
	}
	if strings.Contains(res.Content, "【") {
		t.Fatalf("引用全中不得出现终局说明：%q", res.Content)
	}
	// 真裁决：keep 一条、无命中失败
	chk := VerifyOutputClaims(claims, res.Traces)
	if !chk.Declared || len(chk.Results) != 1 || chk.Results[0].Verdict != VerdictKeep {
		t.Fatalf("应判 keep 一条，实际 %+v", chk.Results)
	}
	if chk.HasFailure() {
		t.Fatalf("引用全中 ⇒ 无命中失败，实际 %d 条", len(chk.Failed))
	}
	if out, changed := ApplyOutputClaimFacts(chk, TerminalInput{CompletedSteps: 1, Checkpointable: true}); changed {
		t.Fatalf("引用全中不得改任何事实位：%+v", out)
	}
	t.Logf("引用全中：kind=%s 正文=%q（未改道）", res.ExitKind, res.Content)
}

// ── ② 产出引用有假 ⇒ 按既有四终局处理（断言具体结局）──────────────────────────

func TestOutputClaims_FakeReceiptGoesToFourTerminals(t *testing.T) {
	// 假回执（引用了不存在的回执 "999"）+ 本轮有真实回执 ⇒ 既有优先级：有进展且仍有未覆盖部分 ⇒ degrade
	claims := []Claim{
		{Text: "我改了 exec.go 的第 12 行", ReceiptID: "999", Quote: "x := strings.TrimSpace(s)"},
	}
	res := runMemberOneToolThenDeliver(t, claims, 3)

	if res.ExitKind != "natural" {
		t.Fatalf("退出原因不得被改动（不新造状态名）：实际 kind=%q", res.ExitKind)
	}
	if !strings.Contains(res.Content, "【降级完成】") {
		t.Fatalf("命中失败 ⇒ 必须落既有四终局之一（此事实集为 degrade）：正文=%q", res.Content)
	}
	if !strings.Contains(res.Content, "命中失败") || !strings.Contains(res.Content, "退出原因：natural") {
		t.Fatalf("终局说明必须点明「凭什么改道」与退出原因：%q", res.Content)
	}
	if !strings.Contains(res.Content, "已完成") {
		t.Fatalf("只标记、不搬运：原产出正文必须还在：%q", res.Content)
	}
	if strings.Count(res.Content, "退出原因") != 1 {
		t.Fatalf("同一次退出只许出一条终局说明（防重复）：%q", res.Content)
	}
	// 真终局：同一事实集过**既有**四终局选择 ⇒ 断言具体结局
	chk := VerifyOutputClaims(claims, res.Traces)
	if !chk.HasFailure() || chk.Failed[0].Verdict != VerdictRetract {
		t.Fatalf("应判 retract 一条，实际 %+v", chk.Results)
	}
	in, changed := ApplyOutputClaimFacts(chk, TerminalInput{CompletedSteps: len(res.Traces), Checkpointable: len(res.Traces) > 0})
	if !changed {
		t.Fatal("命中失败必须置事实位")
	}
	if st, reason := ChooseTerminal(in); st != TerminalDegrade {
		t.Fatalf("该事实集应落 degrade（设计稿 §5.4-4「判无据 ⇒ 降级/拆小步」），实际 %s（%s）", st, reason)
	}
	t.Logf("命中失败 ⇒ 终局 = degrade：%q", res.Content)
}

func TestOutputClaims_FakeReceiptWithoutProgressEscalates(t *testing.T) {
	// 零进展（本轮没有任何回执可引）+ 假回执 ⇒ 既有优先级：零进展/无可继续路径 ⇒ escalate
	infer, _ := mockInfer([]Response{{Content: "我改了 exec.go"}})
	claims := []Claim{{Text: "我改了 exec.go", ReceiptID: "1", Quote: "x"}}
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil,
		Deps{Infer: infer, OutputClaims: claims})

	if len(res.Traces) != 0 {
		t.Fatalf("用例装置不成立：本轮不应有工具轨迹，实际 %d 条", len(res.Traces))
	}
	if res.ExitKind != "natural" || !strings.Contains(res.Content, "【已升级给人】") {
		t.Fatalf("零进展 + 假回执 ⇒ escalate（既有四终局）：kind=%q 正文=%q", res.ExitKind, res.Content)
	}
	t.Logf("零进展 + 假回执 ⇒ 终局 = escalate：%q", res.Content)
}

// 非自然出口同样过这道校验，且**只出一条**终局说明（防两处出口各写一遍）
func TestOutputClaims_NonNaturalExitSingleTerminalNote(t *testing.T) {
	always := Response{ToolCalls: []ToolCall{{ID: "1", Name: "read", Args: map[string]any{"path": "exec.go"}}}}
	infer, _ := mockInfer([]Response{always, always, always})
	claims := []Claim{{Text: "我读了 exec.go", ReceiptID: "404", Quote: "什么都行"}}
	res := Run(context.Background(), Config{MaxRounds: 1}, "m", "sys", nil, Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return memberExecOut, "1ms", nil
		},
		OutputClaims: claims,
	})
	if res.ExitKind != "max_rounds" {
		t.Fatalf("用例装置不成立：应因轮数用尽退出，实际 %q", res.ExitKind)
	}
	if !strings.Contains(res.Content, "命中失败") || !strings.Contains(res.Content, "退出原因：max_rounds") {
		t.Fatalf("非自然出口也要带上引用命中失败：%q", res.Content)
	}
	if strings.Count(res.Content, "退出原因") != 1 {
		t.Fatalf("两处出口不得各写一条终局说明：%q", res.Content)
	}
	t.Logf("非自然出口 + 命中失败（单条说明）：%q", res.Content)
}

// ── ③ 产出根本没引用 / 空产出 ⇒ 行为明确且不崩（零回归）──────────────────────

func TestOutputClaims_NoClaimsOrEmptyOutputKeepsBehavior(t *testing.T) {
	// ③-a 根本没有引用载体（现网全部如此）：自然收尾，正文逐字节不变
	plain := runMemberOneToolThenDeliver(t, nil, 3)
	if plain.ExitKind != "natural" || plain.Content != "已完成" {
		t.Fatalf("无引用载体 ⇒ 行为必须与接线前一致：kind=%q 正文=%q", plain.ExitKind, plain.Content)
	}
	if chk := VerifyOutputClaims(nil, plain.Traces); chk.Declared || chk.HasFailure() || len(chk.Results) != 0 {
		t.Fatalf("无引用载体 ⇒ 不产生任何裁决：%+v", chk)
	}
	// 零回归：非自然出口的说明形态也不变（无「命中失败」字样）
	always := Response{ToolCalls: []ToolCall{{ID: "1", Name: "read", Args: map[string]any{"path": "exec.go"}}}}
	infer, _ := mockInfer([]Response{always, always, always})
	res := Run(context.Background(), Config{MaxRounds: 1}, "m", "sys", nil, Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return memberExecOut, "1ms", nil
		},
	})
	if !strings.Contains(res.Content, "（退出原因：max_rounds）") || strings.Contains(res.Content, "命中失败") {
		t.Fatalf("无引用载体 ⇒ 非自然出口说明形态不得变：%q", res.Content)
	}

	// ③-b 空产出（正文为空、零回执）：不崩、不加说明、仍算自然收尾
	empty, _ := mockInfer([]Response{{Content: ""}})
	eres := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{Infer: empty})
	if eres.ExitKind != "natural" || eres.Content != "" {
		t.Fatalf("空产出 ⇒ 原样返回（不崩不加料）：kind=%q 正文=%q", eres.ExitKind, eres.Content)
	}
	if chk := VerifyOutputClaims([]Claim{}, eres.Traces); chk.Declared || chk.HasFailure() {
		t.Fatalf("空引用列表 ⇒ 不判：%+v", chk)
	}
	// 空产出 + 声明的引用（引用了不存在的回执）⇒ 走四终局，且不得 panic
	eres2 := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil,
		Deps{Infer: empty, OutputClaims: []Claim{{Text: "x", ReceiptID: "1"}}})
	if !strings.Contains(eres2.Content, "【已升级给人】") {
		t.Fatalf("空产出 + 假回执 ⇒ 走四终局：%q", eres2.Content)
	}
	t.Logf("③ 无引用/空产出：plain=%q / empty=%q", plain.Content, eres.Content)
}

// ── ④ 边界：revise（缺引用）不算命中失败 ──────────────────────────────────────

func TestOutputClaims_ReviseIsNotHitFailure(t *testing.T) {
	claims := []Claim{{Text: "我读了 exec.go"}} // 无回执ID ⇒ revise（缺引用，要求补）
	res := runMemberOneToolThenDeliver(t, claims, 3)

	if res.ExitKind != "natural" || res.Content != "已完成" {
		t.Fatalf("缺引用（revise）不得触发改道：kind=%q 正文=%q", res.ExitKind, res.Content)
	}
	chk := VerifyOutputClaims(claims, res.Traces)
	if !chk.Declared || len(chk.Results) != 1 || chk.Results[0].Verdict != VerdictRevise {
		t.Fatalf("应判 revise 一条：%+v", chk.Results)
	}
	if chk.HasFailure() {
		t.Fatalf("★ revise（缺引用）不是命中失败（判据闭集 = {retract, send_back}）：%+v", chk.Failed)
	}
	t.Logf("revise 不改道：正文=%q", res.Content)
}

// ── ⑤ 边界：回执集口径（无 ID 不收 / 空原文照收 ⇒ 判「无据」而非「假回执」）──────

func TestMemberReceipts_TraceToReceipt(t *testing.T) {
	traces := []Trace{
		{CallID: "1", Name: "read", Result: memberExecOut},
		{CallID: " ", Name: "grep", Result: "无 ID ⇒ 引用不到，不收"},
		{CallID: "3", Name: "bash", Result: ""}, // 空原文是真实事实（那次调用没有原文）⇒ 照收
	}
	rs := MemberReceipts(traces)
	if len(rs) != 2 {
		t.Fatalf("回执集应为 2（无 ID 的不收、空原文的照收），实际 %d：%+v", len(rs), rs)
	}
	if rs[0].ID != "1" || rs[0].Tool != "read" || !strings.Contains(rs[0].Output, "TrimSpace") {
		t.Fatalf("回执 ID/工具/原文映射不对：%+v", rs[0])
	}
	// 空原文的回执照收 ⇒ 引用它的主张判「无据（send_back）」，不误判成「假回执（retract）」
	got := VerifyClaims([]Claim{{Text: "c", ReceiptID: "3", Quote: "任何片段"}}, rs)
	if len(got) != 1 || got[0].Verdict != VerdictSendBack {
		t.Fatalf("引用有回执但无原文者 ⇒ send_back（无据），实际 %+v", got)
	}
	// 无 ID 的轨迹不在回执集里 ⇒ 引用它只能是 retract（假回执）
	got2 := VerifyClaims([]Claim{{Text: "c", ReceiptID: "2", Quote: "x"}}, rs)
	if got2[0].Verdict != VerdictRetract {
		t.Fatalf("回执集里没有的 ID ⇒ retract，实际 %+v", got2[0])
	}
}
