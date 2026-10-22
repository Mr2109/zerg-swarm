// gap_20260929_41_lock_test.go —— 缺口 `GAP-20260929-41` **残留**（本笔）的自检。
//
// 残留逐字（收一条）：「`gapRewriteOne` 先按 `led.Path` 取锁 ⇒ 在 `ZERG_GAP_LEDGER_BORROW`
// 只读借用档下误跑 `--yes` 写命令，会在**真源旁边**留一枚 0 字节 `<借源>.lock`（账内容未变，
// 但写了别人的盘）」。
//
// 本件断言：
//
//	① `gapLedgerWritePath()`（写面落点）在借用档下**没有落点**（返回错 + 判词点名借用档）；
//	② 借用档下 `gap note <id> --yes` ⇒ **rc=2**（用法错 · 取锁之前就拒），判词点名「只读借用档」；
//	③ 借源目录**零新增文件**（前后 `ls` 逐名对比 · 且显式 stat `<借源>.lock` 不存在）；
//	④ 状态目录（沙箱）同样零落点（✗ 不许把锁挪个地方还是造出来）；
//	⑤ 借来的真源**逐字节不变**（sha256 对拍）。
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gap41LockNames —— 目录下全部文件名（升序 · 前后对拍用）。目录读不到 ⇒ 当场炸。
func gap41LockNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不了目录 %s：%v", dir, err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestGapBorrowWriteLeavesNoLockBesideSource —— 残留的主判据（借档下写面拒执 · 借源目录零新增）。
func TestGapBorrowWriteLeavesNoLockBesideSource(t *testing.T) {
	sandbox := t.TempDir()   // 探针状态目录
	borrowDir := t.TempDir() // 借来的真源（真账的替身）
	recs := []map[string]any{
		gap41Rec("GAP-20990004-A", "core/a.go", "modA", "famA", "仍缺", "P1", "2026-09-01T00:00:00+08:00"),
	}
	ledger := gap41Write(t, borrowDir, recs)
	beforeSHA := gap41SHA(t, ledger)
	beforeFiles := gap41LockNames(t, borrowDir)

	t.Setenv("ZERG_STATE_DIR", sandbox)
	t.Setenv(gapBorrowEnv, borrowDir)

	// ① 写面落点函数本身：借用档下**没有落点**（判词点名借用档）。
	if p, err := gapLedgerWritePath(); err == nil {
		t.Fatalf("借用档下写面落点必须拒（无落点），却给了 %q", p)
	} else if !strings.Contains(err.Error(), "只读借用档") {
		t.Errorf("写面落点的判词要点名只读借用档，得到 %q", err.Error())
	}

	// ② 借用档下 `gap note … --yes` ⇒ rc=2（取锁之前就拒）。
	rc, out, errb := gapShowAxisRun(t, "gap", "note", "GAP-20990004-A", "--text", "借用档下不该写得进去", "--yes")
	if rc != exitUsage {
		t.Fatalf("借用档下 `gap note --yes` 要 rc=2（用法错 · 取锁前拒），得到 %d · stdout=%q · stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "只读借用档") {
		t.Errorf("拒执判词要点名「只读借用档」，stderr=%q", errb)
	}

	// ③ 借源目录**零新增文件**（这是本笔的题眼：不再留一枚 0 字节 .lock）。
	afterFiles := gap41LockNames(t, borrowDir)
	if strings.Join(beforeFiles, "\x00") != strings.Join(afterFiles, "\x00") {
		t.Errorf("借源目录被写了！前后文件清单不符：before=%v after=%v", beforeFiles, afterFiles)
	}
	if _, err := os.Stat(filepath.Join(borrowDir, gapLedgerFile+".lock")); err == nil {
		t.Errorf("借源旁边不该出现锁件（%s 出现了）", filepath.Join(borrowDir, gapLedgerFile+".lock"))
	}

	// ④ 状态目录（沙箱）也不许有落点 —— 借用档下写面连锁都不该造。
	if got := gap41LockNames(t, sandbox); len(got) != 0 {
		t.Errorf("状态目录不该有任何新增文件，得到 %v", got)
	}

	// ⑤ 借来的真源逐字节不变。
	if afterSHA := gap41SHA(t, ledger); afterSHA != beforeSHA {
		t.Errorf("借来的真源被改了！before=%s after=%s", beforeSHA, afterSHA)
	}
}
