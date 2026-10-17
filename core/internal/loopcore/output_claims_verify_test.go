// output_claims_verify_test.go — GAP-20261018-01 第二半（续）实证：
// A 类同义形态 `退出码 N`、B 类 ③「引文必须真在所指的那一行」、C 类软核「已取证 / 未取证（只进 warning）」。
//
// 取证纪律（与 output_claims_derive_test.go / claims_numbers_test.go 同一套）：
//
//	· 判据用**真实的** `VerifyNumericGranular`（以及既有 `VerifyNumericClaims`）算，裁决走**真实的**
//	  `VerifyClaims`（一处口径都不在本件里复述）；
//	· e2e 用**真实的** `Run` 跑，终局用既有 `ChooseTerminal` / 终局说明判；
//	· 负控在**仓外**跑（`go test -overlay` 把 `VerifyNumericGranular` 哑化成返回空 ⇒ 引文那一格必须转绿）。
package loopcore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// granCombined — 与 run.go 同一条接线：既有三档（claims_numbers.go）＋ 本文件增量，硬核命中失败合流。
func granCombined(content string, traces []Trace, root string) []ClaimVerdictResult {
	out := VerifyNumericClaims(content, traces, root).HardFailed
	return append(out, VerifyNumericGranular(content, traces, root).HardFailed...)
}

// granTokens — 读数集合里是否含某条（软核断言用）。
func granTokens(list []string, want string) bool {
	for _, s := range list {
		if strings.TrimSpace(s) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

// ── ① A 类：正控（真话）＋ 反控（不符）────────────────────────────────────────

type granCase struct {
	name    string
	want    int // 期望命中失败条数
	verdict ClaimVerdict
	quote   string
}

func assertGranFailures(t *testing.T, label string, got []ClaimVerdictResult, c granCase) {
	t.Helper()
	if len(got) != c.want {
		t.Fatalf("★ %s：期望 %d 条命中失败，实际 %d 条：%+v", label, c.want, len(got), got)
	}
	if c.want == 0 {
		return
	}
	if got[0].Verdict != c.verdict {
		t.Fatalf("★ %s：期望 %s，实际 %s（%s）", label, c.verdict, got[0].Verdict, got[0].Reason)
	}
	if c.quote != "" && got[0].Claim.Quote != c.quote {
		t.Fatalf("★ %s：片段应逐字为 %q，实际 %q", label, c.quote, got[0].Claim.Quote)
	}
}

func TestVerifyGranular_RCTruthfulNoClaim(t *testing.T) {
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	// ★ 形态取自判据原文（`已跑 <命令> rc=0`）：命令串与断言之间只隔排版符 ⇒ 该腿判得出来。
	body := "我已跑 `zerg gate check --fast` rc=0。"
	assertGranFailures(t, "A 正控（回执 rc=0 ∧ 正文 rc=0）", granCombined(body, traces, ""), granCase{want: 0})
	assertGranFailures(t, "A 正控（既有 rc=N 腿）", VerifyNumericClaims(body, traces, "").HardFailed, granCase{want: 0})
	t.Log("(①正控) 真话 ⇒ 0 条断言")
}

func TestVerifyGranular_RCMismatchSendBack(t *testing.T) {
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckFailOut)} // 回执首行 rc=2
	body := "我已跑 `zerg gate check --fast` rc=0。"
	got := granCombined(body, traces, "")
	assertGranFailures(t, "A 反控（回执 rc=2 ∧ 正文 rc=0）", got, granCase{want: 1, verdict: VerdictSendBack, quote: "rc=0"})
	if got[0].Claim.ReceiptID != "7" {
		t.Fatalf("★ 必须引用那条真回执（ID=7），实际 %q", got[0].Claim.ReceiptID)
	}
	t.Logf("(①反控) %s（%s）", got[0].Verdict, got[0].Reason)
}

func TestVerifyGranular_ExitWordMismatchSendBack(t *testing.T) {
	bad := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckFailOut)}
	body := "我已跑 `zerg gate check --fast` 退出码 0。"
	got := VerifyNumericGranular(body, bad, "")
	assertGranFailures(t, "A 同义形态（退出码 0 vs 回执 rc=2）", got.HardFailed, granCase{want: 1, verdict: VerdictSendBack, quote: "退出码 0"})

	// 正控：回执 rc=0 ⇒ 同义写法也算真话
	ok := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	assertGranFailures(t, "A 同义形态正控（退出码 0 vs 回执 rc=0）",
		VerifyNumericGranular(body, ok, "").HardFailed, granCase{want: 0})
	t.Log("(①同义形态) 反控 send_back · 正控 0 条")
}

// ── ② B 类三格 ──────────────────────────────────────────────────────────────

func TestVerifyGranular_FileLineThreeCells(t *testing.T) {
	// 解析根用**仓外夹具**：件内容与行号由本用例自己钉住，不随仓内件增删漂移
	// （真仓形态那一格在 ④「真话全绿」里：真件 ＋ 真行 ＋ 真符号）。
	root := numRootWith(t, map[string]string{"core/x/y.go": "package x\n\nfunc A() {}\n\nfunc B() {}\n"}) // 5 行
	ok := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}

	// (a) 件不存在 ⇒ 判红（假引用；态归既有「引用了不存在的回执」那一格）
	missing := "见 `core/x/nonexistent_gran.go:42` 的函数体。"
	gotA := granCombined(missing, ok, root)
	if len(gotA) != 1 || gotA[0].Verdict != VerdictRetract {
		t.Fatalf("★ (a) 引用不存在的件 ⇒ 应判 1 条 retract，实际 %+v", gotA)
	}
	if !strings.Contains(gotA[0].Claim.ReceiptID, "nonexistent_gran.go:42") {
		t.Fatalf("★ (a) 合成回执号应点名该引用：%+v", gotA[0].Claim)
	}

	// (b) 行号超界 ⇒ 判红（件在、行不在）
	outRange := "见 `core/x/y.go:99999` 的 `DeriveClaims`。"
	gotB := granCombined(outRange, ok, root)
	if len(gotB) != 1 || gotB[0].Verdict != VerdictRetract {
		t.Fatalf("★ (b) 行号超界 ⇒ 应判 1 条 retract，实际 %+v", gotB)
	}
	// 同一条不许被两腿各判一票：本文件那腿必须让位（超界归 claims_numbers.go）
	assertGranFailures(t, "(b) 本文件那腿不重复判", VerifyNumericGranular(outRange, ok, root).HardFailed, granCase{want: 0})

	// (c) 件与行都在、但引的符号不在该行（第 3 行是 func A，不是 B）⇒ send_back（本文件补的那一格）
	badQuote := "我已读了 `core/x/y.go:3`，该行是 `B` 的定义。"
	gotC := granCombined(badQuote, ok, root)
	assertGranFailures(t, "(c) 引文不在该行", gotC, granCase{want: 1, verdict: VerdictSendBack, quote: "B"})
	if !strings.HasPrefix(gotC[0].Claim.ReceiptID, claimLineEvidencePrefix) {
		t.Fatalf("★ (c) 应引用「行证据回执」（%s…），实际 %q", claimLineEvidencePrefix, gotC[0].Claim.ReceiptID)
	}

	// 对照：同一行、引该行**真有**的符号 ⇒ 不判（证明这一格不是「凡引文必红」）
	goodQuote := "我已读了 `core/x/y.go:3`，该行是 `A` 的定义。"
	assertGranFailures(t, "(c) 对照：引文真在该行", granCombined(goodQuote, ok, root), granCase{want: 0})

	// e2e：真 Run 走同一条接线 ⇒ 终局说明必须点名 send_back 1 条，且原正文还在（只标记、不搬运）
	body := badQuote
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: body},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		CWD:   root,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckOKOut, "12ms", nil
		},
	})
	if res.ExitKind != "natural" {
		t.Fatalf("退出原因不得被改动：%q", res.ExitKind)
	}
	if !strings.Contains(res.Content, body) {
		t.Fatalf("★ 原正文必须还在（只标记、不搬运），实际 %q", res.Content)
	}
	if !strings.Contains(res.Content, "send_back 1") {
		t.Fatalf("★ e2e 终局说明应点名 send_back 1 条，实际 %q", res.Content)
	}
	t.Logf("(②) a=%s · b=%s · c=%s", gotA[0].Verdict, gotB[0].Verdict, gotC[0].Verdict)
}

// TestVerifyGranular_QuotedSpanBoundaries — 保守边界三格：远处的反引号 / 路径 / 命令都不判。
func TestVerifyGranular_QuotedSpanBoundaries(t *testing.T) {
	root := numRootWith(t, map[string]string{"core/x/y.go": "package x\n\nfunc A() {}\n\nfunc B() {}\n"}) // 5 行
	cases := []struct{ name, body string }{
		{"远处反引号不算对这一行的断言",
			"我已读了 `core/x/y.go:5`，而它在收尾时其实还会经过好几层别的调用路径与守卫分支，最终才落到同一份包里的那个同名工具函数 `splitClaimSentences` 上。"},
		{"反引号里是路径",
			"我已读了 `core/x/y.go:5`，同目录的 `output_claims.go` 也翻过。"},
		{"反引号里是命令",
			"我已读了 `core/x/y.go:5`，并跑过 `zerg gate check --fast`。"},
	}
	// 装置自检：第一格的引文确实在窗口**之外**（否则测到的不是「远处」）
	const farRef, farSpan = "core/x/y.go:5", "`splitClaimSentences`"
	body0 := cases[0].body
	ri, si := strings.Index(body0, farRef), strings.Index(body0, farSpan)
	if ri < 0 || si < 0 {
		t.Fatalf("装置不成立：引用 %q 或引文 %q 不在用例句里", farRef, farSpan)
	}
	if d := utf8.RuneCountInString(body0[ri+len(farRef) : si]); d <= numericQuoteWindow {
		t.Fatalf("装置不成立：引文与引用的间距 %d 应 > 窗口 %d（否则测的不是「远处」）", d, numericQuoteWindow)
	}
	for _, c := range cases {
		if got := VerifyNumericGranular(c.body, nil, root).HardFailed; len(got) != 0 {
			t.Fatalf("★ %s ⇒ 不判，实际 %d 条：%+v", c.name, len(got), got)
		}
	}
	t.Logf("(②边界) %d 格全部不判", len(cases))
}

// ── ③ C 类：未取证只进 warning，不判红 ──────────────────────────────────────

func TestVerifyGranular_SoftCountsUnbackedOnlyWarn(t *testing.T) {
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)} // 回执里没有 1474
	body := "交付：`zerg gate check --fast` rc=0，共 1474 行。"

	got := VerifyNumericGranular(body, traces, "")
	assertGranFailures(t, "C 类不得判红", got.HardFailed, granCase{want: 0})
	if !granTokens(got.SoftUnbacked, "1474 行") {
		t.Fatalf("★ 「共 1474 行」在回执里找不到 ⇒ 应标「未取证」，实际 %+v", got.SoftUnbacked)
	}
	if granTokens(got.SoftBacked, "1474 行") {
		t.Fatalf("★ 回执里没有这个数 ⇒ 不得标「已取证」：%+v", got.SoftBacked)
	}

	payload := got.WarningPayload()
	if payload == "" {
		t.Fatalf("★ 未取证读数必须进 warning 载荷")
	}
	var w numericSoftWarning
	if err := json.Unmarshal([]byte(payload), &w); err != nil {
		t.Fatalf("warning 载荷不是合法 JSON：%q（%v）", payload, err)
	}
	if w.Unbacked < 1 || !granTokens(w.UnbackedTokens, "1474 行") {
		t.Fatalf("★ warning 载荷应点名未取证的读数，实际 %s", payload)
	}

	// e2e：正文**逐字节不变**（软核恒不进正文）＋ 事件真发出来
	var evSeen []string
	var softPayload string
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: body},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		Events: func(event, payload string) {
			evSeen = append(evSeen, event)
			if event == "output_claim_soft" {
				softPayload = payload
			}
		},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckOKOut, "12ms", nil
		},
	})
	if res.Content != body {
		t.Fatalf("★ 软核一律不进正文 ⇒ 正文必须逐字节不变，实际 %q", res.Content)
	}
	if softPayload == "" {
		t.Fatalf("★ 未取证读数应落 output_claim_soft 事件，实际事件 %v", evSeen)
	}
	t.Logf("(③) 0 条判红 · 未取证 %d 条 · 正文 %d 字节未变 · 事件 %s", len(got.SoftUnbacked), len(res.Content), softPayload)
}

func TestVerifyGranular_SoftBackedMarked(t *testing.T) {
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)} // 回执含「已登记 136 件」
	body := "已登记 136 件。"
	got := VerifyNumericGranular(body, traces, "")
	assertGranFailures(t, "C 类已取证不判红", got.HardFailed, granCase{want: 0})
	if !granTokens(got.SoftBacked, "136 件") {
		t.Fatalf("★ 回执里有「136 件」⇒ 应标「已取证」，实际 backed=%+v unbacked=%+v", got.SoftBacked, got.SoftUnbacked)
	}
	t.Logf("(③) 已取证：%+v", got.SoftBacked)
}

// ── ④ 真话全绿：真 rc ＋ 真 file:line ＋ 真符号 ⇒ 断言 0 条 ──────────────────

func TestVerifyGranular_TruthfulDraftAllGreen(t *testing.T) {
	root := numRepoRoot(t)
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	body := "结论：我已跑 `zerg gate check --fast` rc=0，退出码 0（两处口径一致）。\n" +
		"引用：`core/internal/loopcore/output_claims_derive.go:262` 的 `DeriveClaims`，已登记 136 件。"

	// 装置自检：被引的那一行确实含这个符号（否则件被改过之后这一格会以「判红」的形态假红）
	if b, err := os.ReadFile(filepath.Join(root, "core/internal/loopcore/output_claims_derive.go")); err != nil {
		t.Fatalf("装置不成立：读不到被引件：%v", err)
	} else if lines := strings.Split(string(b), "\n"); len(lines) < 262 || !strings.Contains(lines[261], "DeriveClaims") {
		t.Fatalf("装置不成立：output_claims_derive.go:262 已不含 DeriveClaims ⇒ 换一行重钉本格")
	}

	if got := granCombined(body, traces, root); len(got) != 0 {
		t.Fatalf("★ 全部属实的成稿不得判红（改了它就是逼模型不敢报数/不敢引行），实际 %+v", got)
	}
	if got := DeriveClaims(body, traces); len(got) != 0 {
		t.Fatalf("★ 第一半也不得对真话派生主张，实际 %+v", got)
	}
	gran := VerifyNumericGranular(body, traces, root)
	if len(gran.SoftUnbacked) != 0 {
		t.Fatalf("★ 软核读数都在回执里（136 件）⇒ 不应有未取证项：%+v", gran.SoftUnbacked)
	}
	if p := gran.WarningPayload(); p != "" {
		t.Fatalf("★ 没有未取证读数 ⇒ 不发 warning（真话连事件都不惊动）：%q", p)
	}

	// e2e：真 Run ⇒ 正文逐字节不变（一个终局说明都不许加）
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: body},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		CWD:   root,
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckOKOut, "12ms", nil
		},
	})
	if res.Content != body {
		t.Fatalf("★ 真话 ⇒ 正文必须逐字节不变，实际 %q", res.Content)
	}
	t.Logf("(④) 真话全绿：断言 0 条 · 正文 %d 字节未变", len(res.Content))
}
