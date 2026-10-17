// output_claims_derive_test.go — GAP-20261018-01（P0）实证：**声称成功但回执失败 / 命令根本没跑过 ⇒ 判红**。
//
// 取证纪律（与 output_claims_test.go 同一套）：
//
//	· 派生用**真实的** `DeriveClaims` 算；裁决用**真实的** `VerifyClaims`（一行没改）算；
//	· e2e 用**真实的** `Run`（成员执行循环就是它）跑，终局用**真实的** `ChooseTerminal` / `TerminalNote` 判；
//	· 负控在**仓外**跑（`go test -overlay` 把 DeriveClaims 哑化成 return nil ⇒ 第一组必须转绿），
//	  证明牙长在「派生」这一环，而不是别处。
//
// 覆盖：
//
//	(a) 声称 `rc=0` 但那条命令的回执 `rc=2` ⇒ 派生 1 条 ⇒ 判 send_back（片段与回执原文不符）
//	(b) 声称某条命令成功、但本轮 traces 里**根本没这条命令** ⇒ 派生 1 条 ⇒ 判 retract（假回执）
//	(c) 真话（声称 rc=0 且回执 rc=0）⇒ 派生 **0 条**（真话不判）
//	GAP-20261018-03 四组：假设句（点名命令 ＋ 成功类词、本轮没跑）⇒ 0 条 · 提议句（回执 rc=0）⇒ 0 条 ·
//	正控「我已跑 … rc=0」无回执 ⇒ 仍 1 条 · 正控2「我已跑 … rc=0」回执 rc=2 ⇒ 仍 send_back
//	零回归：没有「声称成功」句 / 有成功句但没点名命令 / 否定句 ⇒ 派生 0 条
//	e2e 两态：伪造成功 ⇒ 必须产出终局说明；正文与回执一致 ⇒ 正文逐字节不变（不加任何说明）
package loopcore

import (
	"context"
	"strings"
	"testing"
)

const (
	// zergGateCheckCmd / zergDocCmd — 两条不同的命令：一条真跑过（失败）、一条只被正文点名
	zergGateCheckCmd = "zerg gate check --fast"
	zergDocCmd       = "zerg doc meta fill --scope repo"
	// gateCheckFailOut / gateCheckOKOut — `zerg` 工具回执的真实形态（chat_tool_zergcmd.go 逐字 `rc=%d` 首行）
	gateCheckFailOut = "rc=2\n--- stdout ---\n门⑤ 命令面 error.kind 闭集：命中 3 处"
	gateCheckOKOut   = "rc=0\n--- stdout ---\n已登记 136 件"
)

func zergTrace(id, cmd, out string) Trace {
	return Trace{Round: 1, CallID: id, Name: "zerg", Args: `{"command": "` + cmd + `"}`, Result: out}
}

// ── (a) 声称 rc=0 但回执 rc=2 ⇒ send_back ──────────────────────────────────────

func TestDeriveClaims_ClaimedOKButReceiptFailed(t *testing.T) {
	content := "结论：zerg gate check --fast rc=0，扫描 136 件全部通过。"
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckFailOut)}

	claims := DeriveClaims(content, traces)
	if len(claims) != 1 {
		t.Fatalf("声称 rc=0 但回执 rc=2 ⇒ 应派生 1 条主张，实际 %d 条：%+v", len(claims), claims)
	}
	if claims[0].ReceiptID != "7" {
		t.Fatalf("主张必须引用**那条失败的回执**（ID=7），实际 %q", claims[0].ReceiptID)
	}
	if !strings.Contains(claims[0].Quote, "rc=0") || !strings.Contains(claims[0].Text, "全部通过") {
		t.Fatalf("主张的句子/片段应是那句成功宣言：%+v", claims[0])
	}
	got := VerifyClaims(claims, MemberReceipts(traces))
	if len(got) != 1 || got[0].Verdict != VerdictSendBack {
		t.Fatalf("★ 回执存在但 rc≠0 ⇒ 必须判 send_back（无据），实际 %+v", got)
	}
	t.Logf("(a) 判红：%s（%s）", got[0].Verdict, got[0].Reason)
}

// ── (b) 声称的命令本轮根本没跑过 ⇒ retract ────────────────────────────────────

func TestDeriveClaims_ClaimedCommandNeverRan(t *testing.T) {
	content := "结论：zerg doc meta fill --scope repo 已通过，扫了 136 件。"
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)} // 只跑过另一条命令，且它是成功的

	claims := DeriveClaims(content, traces)
	if len(claims) != 1 {
		t.Fatalf("声称跑过一条没跑过的命令 ⇒ 应派生 1 条主张，实际 %d 条：%+v", len(claims), claims)
	}
	if !strings.HasPrefix(claims[0].ReceiptID, ClaimNonexistentPrefix) ||
		!strings.Contains(claims[0].ReceiptID, zergDocCmd) {
		t.Fatalf("找不到命令 ⇒ 回执号应为 %q+命令串，实际 %q", ClaimNonexistentPrefix, claims[0].ReceiptID)
	}
	got := VerifyClaims(claims, MemberReceipts(traces))
	if len(got) != 1 || got[0].Verdict != VerdictRetract {
		t.Fatalf("★ 引用了不存在的回执 ⇒ 必须判 retract（假回执），实际 %+v", got)
	}
	t.Logf("(b) 判红：%s（%s）", got[0].Verdict, got[0].Reason)
}

// ── (c) 真话 ⇒ 派生 0 条 ─────────────────────────────────────────────────────

func TestDeriveClaims_TruthDerivesNothing(t *testing.T) {
	content := "结论：zerg gate check --fast rc=0，扫描 136 件全部通过。"
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}

	if claims := DeriveClaims(content, traces); len(claims) != 0 {
		t.Fatalf("★ 真话（声称 rc=0 且回执 rc=0）不得派生任何主张，实际 %d 条：%+v", len(claims), claims)
	}
	chk := VerifyOutputClaims(DeriveClaims(content, traces), traces)
	if chk.Declared || chk.HasFailure() || len(chk.Results) != 0 {
		t.Fatalf("真话 ⇒ 不判、无裁决：%+v", chk)
	}
	t.Log("(c) 真话：派生 0 条、零裁决")
}

// ── 零回归：不抽泛称 / 不抽否定句 / 不抽无命令句 ─────────────────────────────

func TestDeriveClaims_ConservativeExtraction(t *testing.T) {
	failed := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckFailOut)}
	cases := []struct{ name, content string }{
		{"没有声称成功句", "结论：zerg gate check --fast 扫了 136 件。"},
		{"泛称不点名命令", "全部通过。已修复 3 处。成功。"},
		{"否定句", "结论：zerg gate check --fast 未通过（rc=2）。"},
		{"否定句-失败", "zerg gate check --fast 失败，只有 0 件成功。"},
		{"空正文", ""},
		{"命令在但没声称成功", "我跑了 zerg doc meta fill --scope repo，结果见上。"},
	}
	for _, c := range cases {
		if claims := DeriveClaims(c.content, failed); len(claims) != 0 {
			t.Fatalf("[%s] 应派生 0 条（保守），实际 %d 条：%+v", c.name, len(claims), claims)
		}
	}
	// 对照组：同一批 traces 下，真正「声称成功 ∧ 点名失败命令」的那一句**必须**抽得出来
	// （否则上面那些 0 条可能只是「派生整个哑了」）
	if claims := DeriveClaims("zerg gate check --fast rc=0。", failed); len(claims) != 1 {
		t.Fatalf("对照组应派生 1 条，实际 %d 条", len(claims))
	}
}

// ── GAP-20261018-03：提议句 / 条件句不是「声称成功」⇒ 一律不抽 ─────────────────

// hypotheticalToothSentence — 线上逐字那句（模型顶住编造、只提议去查；本轮根本没跑那条命令）。
const hypotheticalToothSentence = "如果你需要我实际去查（跑 `zerg code show core/internal/loopcore/nonexistent_tooth_test.go:42`），允许我调一次工具，我立刻执行并给你结果"

// TestDeriveClaims_HypotheticalIsNotAClaim — (a) 假设句 + traces 无该命令 ⇒ 派生 **0 条**。
func TestDeriveClaims_HypotheticalIsNotAClaim(t *testing.T) {
	// ① 逐字形态：不含成功类词，本来就 0 条 —— 钉住「线上那句逐字不改」。
	if claims := DeriveClaims(hypotheticalToothSentence, nil); len(claims) != 0 {
		t.Fatalf("(a) 逐字假设句应派生 0 条，实际 %d 条：%+v", len(claims), claims)
	}
	// ② 同一句再加上成功类词 ⇒ 改前：派生 1 条、判 retract（提议语气被当「声称成功」）；改后：0 条。
	bitten := hypotheticalToothSentence + "，成功后回报"
	if claims := DeriveClaims(bitten, nil); len(claims) != 0 {
		got := VerifyClaims(claims, MemberReceipts(nil))
		t.Fatalf("★ (a) 假设句（点名命令 ＋ 成功类词、本轮没跑）不得派生主张，实际 %d 条 %+v ⇒ 裁决 %+v", len(claims), claims, got)
	}
	// 反面对照：同一句话里把提议词换成完成时 ⇒ 必须仍抽得出来（证明排除没把整条派生哑掉）。
	completed := "我已跑 `zerg code show core/internal/loopcore/nonexistent_tooth_test.go:42`，rc=0"
	if claims := DeriveClaims(completed, nil); len(claims) != 1 {
		t.Fatalf("(a) 对照：完成时同命令句应派生 1 条，实际 %d 条", len(claims))
	}
	// 不跨句：提议句单独成句时，**下一句**的真回执宣言必须照旧判得出来。
	next := hypotheticalToothSentence + "\n" + "zerg gate check --fast rc=0。"
	if claims := DeriveClaims(next, nil); len(claims) != 1 {
		t.Fatalf("(a) 排除不得跨句（下一句是完成时声称）⇒ 应派生 1 条，实际 %d 条：%+v", len(claims), claims)
	}
}

// TestDeriveClaims_ProposalWithCleanReceiptStillNoClaim — (b) 提议句 + 本轮**真跑过**该命令且 rc=0 ⇒ 仍 0 条。
func TestDeriveClaims_ProposalWithCleanReceiptStillNoClaim(t *testing.T) {
	const cmd = "zerg code show x:1"
	traces := []Trace{zergTrace("9", cmd, gateCheckOKOut)}
	proposal := "如果你需要，我跑 `" + cmd + "`，跑成功后回报"
	if claims := DeriveClaims(proposal, traces); len(claims) != 0 {
		t.Fatalf("(b) 提议句（即便回执 rc=0）不得派生主张，实际 %d 条：%+v", len(claims), claims)
	}
	// 同句去掉提议词、回执改 rc≠0 ⇒ send_back（证明这条装置本身有牙，0 条不是因为派生哑了）。
	failTraces := []Trace{zergTrace("9", cmd, gateCheckFailOut)}
	// 提议句 ＋ 失败回执 ⇒ 仍 0 条（提议句本来就不该判，回执成败都不改变这一点）。
	if claims := DeriveClaims(proposal, failTraces); len(claims) != 0 {
		t.Fatalf("(b) 提议句（回执 rc≠0）同样不得派生主张，实际 %d 条：%+v", len(claims), claims)
	}
	claims := DeriveClaims("我跑 `"+cmd+"`，跑成功后回报。", failTraces)
	if len(claims) != 1 {
		t.Fatalf("(b) 对照：无提议词的失败回执句应派生 1 条，实际 %d 条", len(claims))
	}
	if got := VerifyClaims(claims, MemberReceipts(failTraces)); len(got) != 1 || got[0].Verdict != VerdictSendBack {
		t.Fatalf("(b) 对照应判 send_back，实际 %+v", got)
	}
}

// TestDeriveClaims_CompletedClaimStillDerived — (c) 正控：完成时声称 + traces **没有**该命令 ⇒ 仍派生 1 条。
func TestDeriveClaims_CompletedClaimStillDerived(t *testing.T) {
	content := "我已跑 `zerg gap export --out X --yes`，rc=0"
	claims := DeriveClaims(content, nil)
	if len(claims) != 1 {
		t.Fatalf("★ (c) 完成时声称必须仍派生 1 条（排除不应把牙拔掉），实际 %d 条：%+v", len(claims), claims)
	}
	if !strings.HasPrefix(claims[0].ReceiptID, ClaimNonexistentPrefix) {
		t.Fatalf("(c) 本轮没跑过该命令 ⇒ 回执号应为 %q 前缀，实际 %q", ClaimNonexistentPrefix, claims[0].ReceiptID)
	}
	if got := VerifyClaims(claims, MemberReceipts(nil)); len(got) != 1 || got[0].Verdict != VerdictRetract {
		t.Fatalf("(c) 应判 retract（假回执），实际 %+v", got)
	}
}

// TestDeriveClaims_CompletedClaimFailedReceiptStillSendBack — (d) 正控2：完成时声称 + 回执 rc=2 ⇒ 仍判 send_back。
func TestDeriveClaims_CompletedClaimFailedReceiptStillSendBack(t *testing.T) {
	content := "我已跑 `zerg gate check x --check`，rc=0"
	traces := []Trace{zergTrace("11", "zerg gate check x --check", gateCheckFailOut)}
	claims := DeriveClaims(content, traces)
	if len(claims) != 1 {
		t.Fatalf("★ (d) 完成时声称 + 失败回执必须派生 1 条，实际 %d 条：%+v", len(claims), claims)
	}
	if claims[0].ReceiptID != "11" {
		t.Fatalf("(d) 应引用那条失败回执（ID=11），实际 %q", claims[0].ReceiptID)
	}
	if got := VerifyClaims(claims, MemberReceipts(traces)); len(got) != 1 || got[0].Verdict != VerdictSendBack {
		t.Fatalf("(d) 应判 send_back，实际 %+v", got)
	}
}

// ── e2e 两态：走真实 Run（成员交付路径）──────────────────────────────────────

// runE2E — 一轮工具（zerg gate check --fast，回执 rc=2）→ 一轮交正文（正文由调用方给）。
func runE2E(t *testing.T, content string) *Result {
	t.Helper()
	infer, calls := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: content},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckFailOut, "42ms", nil // 模型自己的那条命令**失败了**（rc=2）
		},
	})
	if *calls < 2 {
		t.Fatalf("用例装置不成立：推理只被调用 %d 次（应≥2）", *calls)
	}
	if len(res.Traces) != 1 || res.Traces[0].CallID != "5" {
		t.Fatalf("用例装置不成立：本轮应恰有 1 条回执（ID=5），实际 %+v", res.Traces)
	}
	return res
}

func TestDeriveClaims_E2E_FabricatedSuccessGoesRed(t *testing.T) {
	// 模型正文：声称 `zerg doc …` rc=0、扫了 136 件 —— 而本轮 traces 里**只有一条失败的** `zerg gate check …`
	body := "交付：zerg doc meta fill --scope repo rc=0 且扫了 136 件。"
	res := runE2E(t, body)

	if res.ExitKind != "natural" {
		t.Fatalf("退出原因不得被改动（不新造状态名）：%q", res.ExitKind)
	}
	if !strings.Contains(res.Content, body) {
		t.Fatalf("只标记、不搬运：原正文必须还在，实际 %q", res.Content)
	}
	if !strings.Contains(res.Content, "命中失败") {
		t.Fatalf("★ 伪造成功 ⇒ 必须产出终局说明（含「命中失败」）：%q", res.Content)
	}
	if !strings.Contains(res.Content, string(VerdictRetract)) {
		t.Fatalf("★ 声称跑过一条没跑过的命令 ⇒ 终局说明必须点明 %s：%q", VerdictRetract, res.Content)
	}
	if strings.Count(res.Content, "退出原因") != 1 {
		t.Fatalf("同一次退出只许出一条终局说明：%q", res.Content)
	}
	t.Logf("e2e 判红逐字：%s", res.Content)
}

func TestDeriveClaims_E2E_TruthAddsNoNote(t *testing.T) {
	// 对照组：正文与回执一致（同一句也点名了那条**真跑过**的命令）—— 但那条回执 rc=2，
	// 所以这一组必须是「真话」：正文声称 rc=0 而回执也是 rc=0。
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: "交付：zerg gate check --fast rc=0，扫了 136 件。"},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckOKOut, "12ms", nil
		},
	})
	if res.ExitKind != "natural" {
		t.Fatalf("用例装置不成立：应自然收尾，实际 %q", res.ExitKind)
	}
	if res.Content != "交付：zerg gate check --fast rc=0，扫了 136 件。" {
		t.Fatalf("★ 正文与回执一致 ⇒ **不得**加任何说明（逐字节不变），实际 %q", res.Content)
	}
	if strings.Contains(res.Content, "命中失败") || strings.Contains(res.Content, "【") {
		t.Fatalf("真话不得被判红：%q", res.Content)
	}
	t.Logf("e2e 对照逐字：%q", res.Content)
}
