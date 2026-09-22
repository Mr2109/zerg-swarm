package redact

import (
	"strings"
	"testing"
)

// ── fail-closed（T2.4）：脱敏 panic / 超预算 ⇒ 只写 marker，绝不回退写原文 ──────

const markerLine = `{"event":"redaction_failed"}`

// TestFailClosed_InjectPanic：注入 panic，断言落盘形态与原文的**不可回退性**。
//
// 这是本文件存在的原因：一个 panic 写路径上没人测的脱敏器，出事那天会以「原始事件照写」
// 的形式退化为静默明文（D1）。
func TestFailClosed_InjectPanic(t *testing.T) {
	orig := redactEventFn
	redactEventFn = func(map[string]any) map[string]any { panic("注入：脱敏器崩了") }
	defer func() { redactEventFn = orig }()

	ev := map[string]any{"err": "open " + home + ": permission denied", "model": "qwen3-30b-a3b"}

	before := RedactionFailures()
	line := string(MarshalRedacted(ev))
	if line != markerLine {
		t.Fatalf("fail-closed 形态错误：%s（应为 %s）", line, markerLine)
	}
	if got := RedactionFailures(); got != before+1 {
		t.Fatalf("单调计数器未 +1：%d → %d", before, got)
	}
	if strings.Contains(line, foldUser) || strings.Contains(line, "qwen3") || strings.Contains(line, "permission") {
		t.Fatalf("fallback 写回了原文片段：%s", line)
	}

	// map 入口同样 fail-closed。
	if out := RedactEventFailClosed(ev); out["event"] != EventRedactionFailed {
		t.Fatalf("map 入口未 fail-closed：%+v", out)
	}

	// 复原注入点后必须恢复正常（否则测试自己把门关了，后面全是假绿）。
	redactEventFn = orig
	if got := string(MarshalRedacted(map[string]any{"model": "qwen3-30b-a3b"})); got != `{"model":"qwen3-30b-a3b"}` {
		t.Fatalf("恢复注入点后行为未复原：%s", got)
	}
}

// TestFailClosed_OverBudget：超预算（不是截断）⇒ 整条 marker，绝不全量落盘。
func TestFailClosed_OverBudget(t *testing.T) {
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)

	SetLimits(DefaultMaxValueBytes, 256)
	ev := map[string]any{"err": "open " + home + ": " + strings.Repeat("x", 512)}
	before := RedactionFailures()
	line := string(MarshalRedacted(ev))
	if line != markerLine {
		t.Fatalf("超预算未 fail-closed：%s", line)
	}
	if got := RedactionFailures(); got != before+1 {
		t.Fatalf("单调计数器未 +1：%d → %d", before, got)
	}
	if strings.Contains(line, foldUser) || strings.Contains(line, "xxxx") {
		t.Fatalf("超预算回退写回了原文：%s", line)
	}

	// 边界：预算内的正常事件必须照常落盘 —— fail-closed 不能变成「恒关」。
	SetLimits(DefaultMaxValueBytes, 4096)
	ok := string(MarshalRedacted(map[string]any{"err": "open " + home + " failed"}))
	if strings.Contains(ok, EventRedactionFailed) {
		t.Fatalf("预算内事件被误判超预算：%s", ok)
	}
	mustVanish(t, ok, home)
}
