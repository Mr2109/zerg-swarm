package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
