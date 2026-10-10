// family_gap_concur_rc_test.go —— 缺口账 `GAP-20260928-192` 的回归（并发拒写退码取定 14）。
//
// 病：并发闸判「本进程读入之后账件被别人改过」时退 2（用法错）—— 让机器消费方把
// 「并发冲突（可重试）」误当「用法错（不可重试）」。治：本族口径统一定为退 14
// （`exitConflict` · kind=`conflict`）—— 与同族 `repo commit` 的 `repo_busy` /
// `guard` 的 `rebuild_blocked` 同码。
//
// 两控（缺一即失败）：
//
//	① 负控（本缺口靶）：并发拒写 ⇒ 退 14（改前实现退 2 ⇒ 本断言真红）；
//	② 正控：无关的普通用法错仍退 2（证明本修只收敛「并发拒写」这一路，未把用法错一并推成 14）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGapConcurWriteExitConflict —— 负控：并发拒写 ⇒ 14。
func TestGapConcurWriteExitConflict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gaps.jsonl")
	// 盘上现状（写前现读）≠ 本进程读入那一刻的字节 ⇒ 闸判并发撞车。
	if err := os.WriteFile(p, []byte("disk-now\n"), 0o644); err != nil {
		t.Fatalf("写现读件失败：%v", err)
	}
	led := gapLedger{Path: p, Raw: []byte("read-earlier\n")}
	detail, rc, msg := gapConcurGate(led, "set-state")
	if detail != "concurrent_write" {
		t.Fatalf("要判 `concurrent_write`，得到 detail=%q msg=%q", detail, msg)
	}
	if rc != exitConflict {
		t.Fatalf("并发拒写要退 %d（`exitConflict` · kind=conflict）—— GAP-20260928-192；得到 rc=%d", exitConflict, rc)
	}
	// 相符 ⇒ 放行（证明闸没被一刀切成恒 14）。
	led2 := gapLedger{Path: p, Raw: []byte("disk-now\n")}
	if d2, rc2, _ := gapConcurGate(led2, "set-state"); d2 != "" || rc2 != exitOK {
		t.Fatalf("基线相符应放行 rc=%d，得到 rc=%d detail=%q", exitOK, rc2, d2)
	}
}

// TestGapPlainUsageExit2 —— 正控：普通用法错仍退 2。
func TestGapPlainUsageExit2(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	var out, errb strings.Builder
	rc := RunForTest([]string{"gap", "set-state", "GAP-20260101-0001", "--state", "已派", "--evidence", "x"}, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("普通用法错（闭集外 `--state`）仍要退 %d，得到 rc=%d stderr=%q", exitUsage, rc, errb.String())
	}
}
