package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ⑧ 只读面自报必须与**外部读数**逐字相符（缺口 `GAP-20260929-02`）。
//
// 口径：只读面在 stderr 打的那一行 sha16，必须等于外部 `shasum -a 256 <账>` 的前 16 位
// —— 否则「我没写账」就还是「你信我」，不是机读可核。
func TestGapReadonlySelfReportMatchesExternal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	body := []byte(`{"id":"GAP-TEST-1","state":"已立项"}` + "\n")
	led := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	if err := os.WriteFile(led, body, 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	gapReadonlySelfReport(&buf)
	want := fmt.Sprintf("%x", sha256.Sum256(body))[:16]
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("自报里的 sha16 与外部读数不符：想要 %s，实得 %q", want, buf.String())
	}
	if !strings.Contains(buf.String(), "只读") || !strings.Contains(buf.String(), "未变") {
		t.Fatalf("自报那一行缺关键词「只读 / 未变」：%q", buf.String())
	}

	// 反例：真源读不到 ⇒ **一行都不许打**（不许打假的自证）。
	if err := os.Remove(led); err != nil {
		t.Fatal(err)
	}
	var b2 bytes.Buffer
	gapReadonlySelfReport(&b2)
	if b2.Len() != 0 {
		t.Fatalf("读不到真源时仍打了自报（假自证）：%q", b2.String())
	}
}
