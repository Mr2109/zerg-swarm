// claims_numbers_test.go — GAP-20261018-01 第二半（P0）实证：正文里的**硬读数**逐字核对真实回执 / 真实件。
//
// 取证纪律（与 output_claims_derive_test.go 同一套）：
//
//	· 判据用**真实的** `VerifyNumericClaims` 算；裁决走**真实的** `VerifyClaims`（不在本件里复述口径）；
//	· e2e 用**真实的** `Run`（成员执行循环就是它）跑，终局用既有 `ChooseTerminal` / 终局说明判；
//	· 负控在**仓外**跑（`go test -overlay` 把 `VerifyNumericClaims` 哑化成返回空 ⇒ 数字/件:行那条必须消失）。
//
// 覆盖（四组硬核 + 软核不误杀 + 真实回放）：
//
//	(a) 命令真跑 rc=0、正文写 `rc=2` ⇒ 硬核 1 条 ⇒ send_back
//	(b) 正文引 `core/xxx/不存在.go:12` ⇒ 硬核 1 条 ⇒ retract
//	(c) 正文引真实件但行号超界（`:999999`）⇒ 硬核 1 条 ⇒ retract
//	(d) 真话组：`rc=0` 与回执一致 ＋ 真实存在的 `文件:行` ⇒ HardFailed **空**
//	(e) 软核不误杀：`扫了 136 件` / `1114 断言` / `陈旧 7` ⇒ SoftNotes 有记录、HardFailed **空**、
//	    正文**逐字节不变**
//	(f) 真实回放：编造的 rc=0 + 件:行 ⇒ 必须同时出「命令没跑过」（第一半）与「件:行」（本单）
//	(g) 调用方显式给了 `Deps.OutputClaims` ⇒ 本单的硬核**不介入**（尊重调用方）
package loopcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// numRepoRoot — 仓根（件:行 的解析根；本包目录是 core/internal/loopcore ⇒ 上溯三级）。
func numRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败：%v", err)
	}
	return filepath.Join(wd, "..", "..", "..")
}

// numRootWith — 造一个仓外解析根，逐件写内容（相对路径 ⇒ 内容；父目录自动建）。
func numRootWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("建目录失败（%s）：%v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("写件失败（%s）：%v", rel, err)
		}
	}
	return root
}

// ── (a) 命令真跑 rc=0、正文写 rc=2 ⇒ send_back ────────────────────────────────

func TestVerifyNumericClaims_RCMismatchGoesSendBack(t *testing.T) {
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)} // 回执首行 rc=0
	content := "结论：zerg gate check --fast rc=2，扫了 136 件。"

	got := VerifyNumericClaims(content, traces, "")
	if len(got.HardFailed) != 1 {
		t.Fatalf("★ 正文 rc=2 与回执 rc=0 不符 ⇒ 应判 1 条，实际 %d 条：%+v", len(got.HardFailed), got.HardFailed)
	}
	r := got.HardFailed[0]
	if r.Verdict != VerdictSendBack {
		t.Fatalf("★ 回执存在但片段（rc=2）不在其原文里 ⇒ 必须判 send_back，实际 %s（%s）", r.Verdict, r.Reason)
	}
	if r.Claim.ReceiptID != "7" || r.Claim.Quote != "rc=2" {
		t.Fatalf("主张必须引用**那条回执**且片段逐字为 rc=2：%+v", r.Claim)
	}
	if !strings.Contains(r.Claim.Text, "zerg gate check --fast") {
		t.Fatalf("主张正文应是那句话：%+v", r.Claim)
	}
	t.Logf("(a) 判红：%s（%s）", r.Verdict, r.Reason)
}

// ── (b) 引用不存在的件 ⇒ retract ──────────────────────────────────────────────

func TestVerifyNumericClaims_MissingFileGoesRetract(t *testing.T) {
	root := numRootWith(t, map[string]string{"core/real.go": "package core\n"})
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	content := "交付：core/xxx/不存在.go:12 已读过，zerg gate check --fast rc=0。"

	got := VerifyNumericClaims(content, traces, root)
	if len(got.HardFailed) != 1 {
		t.Fatalf("★ 引用不存在的件 ⇒ 应判 1 条，实际 %d 条：%+v", len(got.HardFailed), got.HardFailed)
	}
	r := got.HardFailed[0]
	if r.Verdict != VerdictRetract {
		t.Fatalf("★ 引用了不存在的东西 ⇒ 必须判 retract（假回执），实际 %s（%s）", r.Verdict, r.Reason)
	}
	if !strings.HasPrefix(r.Claim.ReceiptID, ClaimNonexistentPrefix) ||
		!strings.Contains(r.Claim.ReceiptID, "core/xxx/不存在.go:12") {
		t.Fatalf("合成回执号应为 %q+引用串，实际 %q", ClaimNonexistentPrefix, r.Claim.ReceiptID)
	}
	if r.Claim.Quote != "core/xxx/不存在.go:12" {
		t.Fatalf("片段应是引用串本身，实际 %q", r.Claim.Quote)
	}
	t.Logf("(b) 判红：%s（%s）", r.Verdict, r.Reason)
}

// ── (c) 引真实件但行号超界 ⇒ retract ──────────────────────────────────────────

func TestVerifyNumericClaims_LineOutOfRangeGoesRetract(t *testing.T) {
	root := numRootWith(t, map[string]string{"core/x/y.go": "package x\n\nfunc A() {}\n"}) // 3 行
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	content := "见 core/x/y.go:999999 的函数体（完整）。"

	got := VerifyNumericClaims(content, traces, root)
	if len(got.HardFailed) != 1 {
		t.Fatalf("★ 行号超出总行数（件 3 行、引 999999）⇒ 应判 1 条，实际 %d 条：%+v", len(got.HardFailed), got.HardFailed)
	}
	if v := got.HardFailed[0].Verdict; v != VerdictRetract {
		t.Fatalf("★ 行号超界 ⇒ 必须判 retract，实际 %s", v)
	}
	t.Logf("(c) 判红：%s · 引用串 %q", got.HardFailed[0].Verdict, got.HardFailed[0].Claim.Quote)
}

// ── (d) 真话组：rc 一致 + 件:行 真实在范围 ⇒ HardFailed 空 ────────────────────

func TestVerifyNumericClaims_TruthStaysClean(t *testing.T) {
	root := numRootWith(t, map[string]string{"core/x/y.go": "package x\n\nfunc A() {}\n\nfunc B() {}\n"})
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckOKOut)}
	content := "结论：zerg gate check --fast rc=0 · core/x/y.go:3 的 A 已核。"

	got := VerifyNumericClaims(content, traces, root)
	if len(got.HardFailed) != 0 {
		t.Fatalf("★ 真话（rc 一致 ＋ 件:行 在范围内）⇒ HardFailed 必须为空，实际 %+v", got.HardFailed)
	}
	t.Log("(d) 真话：硬核 0 条")
}

// ── (e) 软核不误杀：其它计数只记录、不判红、正文逐字节不变 ────────────────────

func TestVerifyNumericClaims_SoftCountsNeverGoRed(t *testing.T) {
	// 单位：软核记录有、硬核空（traces 里没有这些数，也不该去找）
	traces := []Trace{zergTrace("7", zergGateCheckCmd, gateCheckFailOut)}
	content := "结论：zerg gate check --fast rc=2 · 扫了 136 件 / 1114 断言 / 陈旧 7。"
	got := VerifyNumericClaims(content, traces, "")
	if len(got.HardFailed) != 0 {
		t.Fatalf("★ 其它计数不得判红（软核），实际 %+v", got.HardFailed)
	}
	if len(got.SoftNotes) < 3 {
		t.Fatalf("软核应记下 136 件 / 1114 断言 / 陈旧 7，实际 %d 条：%+v", len(got.SoftNotes), got.SoftNotes)
	}
	joined := strings.Join(got.SoftNotes, "\n")
	for _, want := range []string{"136 件", "1114 断言", "陈旧 7"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("软核记录里应含 %q：%+v", want, got.SoftNotes)
		}
	}

	// e2e：同一句话走真实 Run（回执 rc=0 ⇒ 真话）⇒ 正文必须**逐字节不变**（一个说明都不许加）
	body := "交付：zerg gate check --fast rc=0，扫了 136 件 · 1114 断言 · 陈旧 7。"
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: body},
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
	if res.Content != body {
		t.Fatalf("★ 软核一律不进正文 ⇒ 正文必须逐字节不变，实际 %q", res.Content)
	}
	t.Logf("(e) 软核 %d 条、硬核 0 条、正文逐字节未变（%d 字节）", len(got.SoftNotes), len(res.Content))
}

// 真实那一发的正文（逐字形状）：命令真跑过、读数与件:行是编的。
const numRealReplayBody = "交付：`zerg doc check-doc-number-live --format json` rc=0 · 扫了 136 件 / 1114 断言 / 陈旧 7 · `%s` 函数体完整"

// numRealReplayRun — 真实回放装置：一轮 zerg 工具（`zerg gate check --fast`，回执 **rc=2**）→ 一轮交正文。
func numRealReplayRun(t *testing.T, body, root string) *Result {
	t.Helper()
	infer, calls := mockInfer([]Response{
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
			return gateCheckFailOut, "42ms", nil // 本轮只有这一条失败回执（正文点名的那条命令**没跑过**）
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

// ── (f) 真实回放：命令那条（第一半）＋ 件:行 那条（本单）必须同时出 ────────────

func TestVerifyNumericClaims_RealReplayBothLegsGoRed(t *testing.T) {
	root := numRepoRoot(t)

	// 逐字那一发的件:行 是 `core/cmd/zerg/check_doc_freshness.go:261`。现读该件**确实存在**（3746 行）⇒
	// 按本单口径（只判存在 / 行号在范围内，✗ 不判该行内容）它**不该**判红 —— 这一格是「不误杀」的正控。
	exact := strings.Replace(numRealReplayBody, "%s", "core/cmd/zerg/check_doc_freshness.go:261", 1)
	traces := []Trace{zergTrace("9", zergGateCheckCmd, gateCheckFailOut)}
	if got := VerifyNumericClaims(exact, traces, root); len(got.HardFailed) != 0 {
		t.Fatalf("★ 该件现读存在且 261 行在范围内 ⇒ 件:行 腿**不得**判红（只判范围、不判内容），实际 %+v", got.HardFailed)
	}

	// 同一条命令、同一批读数，把件:行 换成超界形态 ⇒ 件:行 腿必须判红（这是「有牙」那一格）。
	outOfRange := strings.Replace(numRealReplayBody, "%s", "core/cmd/zerg/check_doc_freshness.go:999999", 1)
	got := VerifyNumericClaims(outOfRange, traces, root)
	if len(got.HardFailed) != 1 || got.HardFailed[0].Verdict != VerdictRetract {
		t.Fatalf("★ 行号超界 ⇒ 件:行 腿应判 retract 1 条，实际 %+v", got.HardFailed)
	}
	if !strings.Contains(got.HardFailed[0].Claim.ReceiptID, "check_doc_freshness.go:999999") {
		t.Fatalf("件:行 那条的合成回执号应点名该引用：%+v", got.HardFailed[0].Claim)
	}
	if len(got.SoftNotes) < 3 {
		t.Fatalf("软核应记下那一堆计数（136 件 / 1114 断言 / 陈旧 7）：%+v", got.SoftNotes)
	}

	// e2e：走真实 Run ⇒ 终局说明里必须**同时**有命令那条（retract）与件:行 那条（retract）= 2 条命中失败。
	res := numRealReplayRun(t, outOfRange, root)
	if res.ExitKind != "natural" {
		t.Fatalf("退出原因不得被改动（不新造状态名）：%q", res.ExitKind)
	}
	if !strings.Contains(res.Content, outOfRange) {
		t.Fatalf("只标记、不搬运：原正文必须还在，实际 %q", res.Content)
	}
	if !strings.Contains(res.Content, "命中失败 2 条") || !strings.Contains(res.Content, "retract 2") {
		t.Fatalf("★ 命令没跑过（第一半）+ 件:行 假引用（本单）⇒ 终局说明应为「命中失败 2 条 · retract 2」，实际 %q", res.Content)
	}
	if strings.Count(res.Content, "退出原因") != 1 {
		t.Fatalf("同一次退出只许出一条终局说明：%q", res.Content)
	}
	t.Logf("(f) 真实回放逐字：%s", res.Content)
}

// ── (g) 调用方显式给了 OutputClaims ⇒ 本单硬核不介入 ──────────────────────────

func TestVerifyNumericClaims_CallerClaimsWin(t *testing.T) {
	body := "交付：zerg gate check --fast rc=0，扫了 136 件。" // 与回执（rc=2）不符 —— 若派生+硬核介入必判红
	infer, _ := mockInfer([]Response{
		{ToolCalls: []ToolCall{{
			ID: "5", Name: "zerg", Args: map[string]any{"command": zergGateCheckCmd},
			RawArgs: `{"command":"` + zergGateCheckCmd + `"}`,
		}}},
		{Content: body},
	})
	res := Run(context.Background(), Config{MaxRounds: 3}, "m", "sys", nil, Deps{
		Infer: infer,
		// 调用方自己给的引用：片段 rc=2 **确实**在回执原文里 ⇒ keep（有据）⇒ 不该判红
		OutputClaims: []Claim{{Text: "命令以 rc=2 结束", ReceiptID: "5", Quote: "rc=2"}},
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return gateCheckFailOut, "42ms", nil
		},
	})
	if res.Content != body {
		t.Fatalf("★ d.OutputClaims 非空 ⇒ 派生与本单硬核一律不介入（尊重调用方），实际 %q", res.Content)
	}
	t.Log("(g) 调用方主张优先：正文未变")
}

// TestVerifyNumericClaims_ReplayNoteCount — 真实回放的**终局读数**单独一格：说明里点明的命中失败条数。
//
// 这一格是负控（`go test -overlay` 把 `VerifyNumericClaims` 哑化成返回空）的读数面：
// 有牙 ⇒ `命中失败 2 条（假回执 retract 2 …）`；哑化 ⇒ `命中失败 1 条（假回执 retract 1 …）`
// —— 只剩「命令没跑过」那一条（第一半），本单的件:行 那条消失。
func TestVerifyNumericClaims_ReplayNoteCount(t *testing.T) {
	root := numRepoRoot(t)
	body := strings.Replace(numRealReplayBody, "%s", "core/cmd/zerg/check_doc_freshness.go:999999", 1)
	res := numRealReplayRun(t, body, root)
	t.Logf("终局说明逐字：%s", res.Content)
	if !strings.Contains(res.Content, "命中失败 2 条（假回执 retract 2") {
		t.Fatalf("★ 有牙应有 2 条命中失败（命令那条 + 件:行 那条），实际 %q", res.Content)
	}
}
