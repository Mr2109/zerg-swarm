package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// ① 小输出原样返回（不做无谓的落盘）。
func TestUnloadToolResultSmallUnchanged(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	in := strings.Repeat("a", ctxUnloadBytes)
	if got := unloadToolResult(in); got != in {
		t.Fatalf("小输出被改了：%q", got)
	}
}

// ② 大输出 ⇒ 指针 ＋ 原文逐字节可回查（这是「可逆」的牙）。
func TestUnloadToolResultLargeRoundTrips(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	in := strings.Repeat("工具输出x", ctxUnloadBytes) // 远超 4 KiB

	got := unloadToolResult(in)
	if got == in {
		t.Fatal("大输出没被卸载")
	}
	if !strings.Contains(got, "已卸载") {
		t.Fatalf("指针没写明卸载：%q", got)
	}
	idx := strings.Index(got, state)
	if idx < 0 {
		t.Fatalf("指针没带落点路径（%s）：%q", state, got)
	}
	tail := got[idx:]
	tail = strings.TrimSuffix(tail, "〕")
	full := strings.TrimSpace(tail)

	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("原文读不回来：%v（%s）", err, full)
	}
	if string(b) != in {
		t.Fatalf("原文与输入不一致：%d B vs %d B", len(b), len(in))
	}
}

// ③ 内容寻址：同一内容卸两次只落一份。
func TestUnloadToolResultContentAddressed(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	in := strings.Repeat("z", ctxUnloadBytes+1)
	a := unloadToolResult(in)
	b := unloadToolResult(in)
	if a != b {
		t.Fatalf("同内容两次指针不同：%q vs %q", a, b)
	}
	ents, err := os.ReadDir(filepath.Join(state, "ctx-unload"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("应只落一份，实得 %d 份", len(ents))
	}
}

// ④ 卸载失败 ⇒ 退回原样（宁可按老口径截断，也不丢证据）。
func TestUnloadToolResultFallbackOnWriteError(t *testing.T) {
	// 把状态目录指到一个「已存在的普通文件」⇒ MkdirAll 必失败
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_STATE_DIR", blocker)
	in := strings.Repeat("q", ctxUnloadBytes+10)
	if got := unloadToolResult(in); got != in {
		t.Fatalf("卸载失败时应原样返回，实得 %q", got)
	}
}

// ⑤ 审计接线：真卸一次 ⇒ `compress-audit.jsonl` **追加**一行，字段与 compress.Event 一致。
//
// 这一条是 events 恒 0 的反证：不写事件时 events 永远是 0（本项目 116 笔只落了卸载件、
// 117 笔只落了只读呈现，写侧一直没人 ⇒ 读侧恒 0）。这里直接断言落盘行的**内容**，
// 不只看文件存在：行数、Parse 得回 compress.Event、Before > After、Mechanism=unload。
func TestUnloadToolResultWritesCompressAudit(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	audit := filepath.Join(state, "compress-audit.jsonl")

	in := strings.Repeat("工具输出a", ctxUnloadBytes) // 远超 4 KiB
	unloadToolResult(in)

	lines := readAuditLines(t, audit)
	if len(lines) != 1 {
		t.Fatalf("应落 1 行审计，实得 %d 行", len(lines))
	}
	var ev compress.Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("审计行不是合法 compress.Event：%v（原文 %q）", err, lines[0])
	}
	if ev.Phase != "completed" || ev.Mechanism != "unload" {
		t.Fatalf("事件三维不对：Phase=%q Mechanism=%q", ev.Phase, ev.Mechanism)
	}
	if ev.At.IsZero() {
		t.Fatal("审计行没有时间戳（At 空）⇒ 不可回查")
	}
	if ev.Before <= 0 || ev.After >= ev.Before {
		t.Fatalf("Before/After 不成立：%d → %d", ev.Before, ev.After)
	}
	if !strings.Contains(ev.Reason, state) {
		t.Fatalf("Reason 没带落点路径：%q", ev.Reason)
	}

	// 追加而非重写：再卸一份**不同**内容 ⇒ 2 行，且第 1 行逐字节不变。
	first := lines[0]
	unloadToolResult(strings.Repeat("工具输出b", ctxUnloadBytes))
	lines = readAuditLines(t, audit)
	if len(lines) != 2 {
		t.Fatalf("第二次卸载后应为 2 行（追加），实得 %d 行", len(lines))
	}
	if lines[0] != first {
		t.Fatalf("追加写动了旧行（第 1 行被重写）：\n旧 %q\n新 %q", first, lines[0])
	}
}

// ⑥ 没卸载就别写事件（小输出 / 卸载失败都不算一次压缩动作）——events 不能靠灌水变大。
func TestUnloadSmallOutputWritesNoAudit(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	unloadToolResult(strings.Repeat("s", ctxUnloadBytes)) // 恰在阈值上 = 不卸
	if _, err := os.Stat(filepath.Join(state, "compress-audit.jsonl")); err == nil {
		t.Fatal("小输出不应产生审计行")
	}
	// 跨阈值一次 + 阈值上一次 ⇒ 只 1 行
	unloadToolResult(strings.Repeat("t", ctxUnloadBytes+1))
	unloadToolResult(strings.Repeat("t", ctxUnloadBytes))
	if n := len(readAuditLines(t, filepath.Join(state, "compress-audit.jsonl"))); n != 1 {
		t.Fatalf("应只 1 行审计（只有跨阈值那次算），实得 %d 行", n)
	}
}

// ⑦ 端到端烟测（默认 skip · 只有点名 `ZERG_CTX_AUDIT_SMOKE_STATE=<状态目录>` 才跑）：
// 走**真**压缩入口 compressOldest → summarizeRound → unloadToolResult，事件落进点名的状态目录。
// 点名的目录就是 `zerg context stats` 读的那个目录 ⇒ 跑完 events 必须 > 0。
//
// 为什么默认 skip：本包普通用例一律 t.TempDir() 隔离，不许往用户真实状态目录里灌审计行；
// 这条只用于验收「写侧接线是否真通到 CLI 读数」这一步（跑法见文件末尾注释）。
func TestUnloadAuditSmokeAmbientState(t *testing.T) {
	state := os.Getenv("ZERG_CTX_AUDIT_SMOKE_STATE")
	if state == "" {
		t.Skip("未点名 ZERG_CTX_AUDIT_SMOKE_STATE ⇒ 跳过端到端烟测")
	}
	t.Setenv("ZERG_STATE_DIR", state)
	audit := filepath.Join(state, "compress-audit.jsonl")
	before := countJSONLines(t, audit)

	a := NewAgent(Config{ContextWindow: 1})
	a.history = []Message{
		{Role: "assistant", Content: "决策：读大文件"},
		{Role: "tool", ToolCallID: "t1", Content: strings.Repeat("大输出", ctxUnloadBytes)},
		{Role: "assistant", Content: "决策：继续"},
	}
	if !a.compressOldest() {
		t.Fatal("没压成 —— 烟测前提不成立（应压最早一轮）")
	}
	after := countJSONLines(t, audit)
	if after <= before {
		t.Fatalf("压缩后审计面没长（%d → %d）⇒ 写侧没接到 compress-audit.jsonl", before, after)
	}
}

// countJSONLines —— 数一件 JSONL 的行数（文件不在 = 0）。
func countJSONLines(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("审计件读不回来：%v（%s）", err, path)
	}
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func readAuditLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("审计件读不回来：%v（%s）", err, path)
	}
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
