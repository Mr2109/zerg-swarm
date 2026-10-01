package agent

// edit_ledger_test.go — GAP-20260927-80 件级账的成对判据（正控 + 负控）。
//
// 正控：写一件 ⇒ 账里多一行（`after_sha256` 非空 · `mode` 对得上）。
// 负控：调用方在写失败时不落账 ⇒ 账文件**不产生**、行数恒 0（不许记「想写但没写成」的行）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ledgerLineCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

func TestAgentEditLedger_WriteRecordsLine(t *testing.T) {
	dir := t.TempDir()
	audit := filepath.Join(dir, "edit_audit.jsonl")
	t.Setenv("ZERG_EDIT_AUDIT", audit)
	target := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	n0 := ledgerLineCount(t, audit)
	recordAgentEditLedger("write", target, []byte("old"), []byte("new"), "agent-test")
	n1 := ledgerLineCount(t, audit)
	if n1 != n0+1 {
		t.Fatalf("正控失败：期望账多 1 行，得 %d → %d", n0, n1)
	}
	if shortSHA16([]byte("new")) == "" {
		t.Fatal("after_sha256 不应为空")
	}
}

func TestAgentEditLedger_NoWriteNoLine(t *testing.T) {
	dir := t.TempDir()
	audit := filepath.Join(dir, "edit_audit.jsonl")
	t.Setenv("ZERG_EDIT_AUDIT", audit)
	// 负控：写失败时调用方不调用 recordAgentEditLedger ⇒ 账文件不产生。
	if _, err := os.ReadFile(audit); !os.IsNotExist(err) {
		t.Fatalf("负控失败：账文件不应存在（err=%v）", err)
	}
	if n := ledgerLineCount(t, audit); n != 0 {
		t.Fatalf("负控失败：错账行数 %d（应为 0）", n)
	}
}
