// family_gap_apply_route_test.go —— 缺口 `GAP-20261010-12` 的**写路换线回归牙**（本单新增件）。
//
// 面（病 · 改前）：本族另有三条真写路 —— `add` / `verify` / `backfill-unit` —— 只走乐观闸
// `gapConcurGate`（写前对一次身份 = check-then-act）；闸与「审计先落盘（fsync）」+「整件重写」
// 之间**无锁** ⇒ 两个写者可先后过闸、后写整件覆盖先写 ⇒ **静默丢更新**。
//
// 牙（逐处一枚 · 缺一即失败）：改后三处换线到账写通用唯一收口 `gapLedgerApply` ⇒ **写路必须落在
// 那把 `flock(LOCK_EX)` 里**。判法 = 测试**先持账写排他锁**，再跑真命令：
//
//	① 持锁期间命令**必须不返回**（真互斥：命令卡在取锁上；未换线的命令照跑照回 ⇒ 这一格真红）；
//	② 放锁后命令**必须返回 rc=0 且真源真落定**（不是死锁、也不是拒写）。
//
// ★ 负控（件外 · `go test -overlay` 把三处换回 `gapConcurGate`）⇒ 持锁期间命令照跑 ⇒ 三枚用例真红。
// ★ 锁的进程语义：`gapLedgerLock` 每次**各自 `os.OpenFile`**（各自独立的 open file description）⇒
// 同一进程里两次 open 之间 `flock` **真冲突**（已实测：第二把 `LOCK_NB` 得 `EWOULDBLOCK`）；只有
// 「重锁同一个 fd」才假绿 —— 本牙不落那个形态。
//
// 落点：`package main`（要摸未导出的 `gapLedgerLock` / `gapLedgerUnlock` / `gapLedgerPath`），
// 复用同包 `family_gap_state_apply_test.go` 的 `seedGapLedger`（`t.TempDir()` 当状态目录 ⇒
// **不碰真 `~/.zerg/state`**、不碰任何仓内件）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// gapLedgerLinesForTest —— 真源现读的**非空行**数（本牙判「真源真落定」用）。
func gapLedgerLinesForTest(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(gapLedgerPath())
	if err != nil {
		t.Fatalf("读真源失败：%v", err)
	}
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

// gapHoldLockAndRun —— 本进程先持账写排他锁，再跑一条真命令；返回
// (持锁期间命令是否已返回, 放锁后命令的退码)。★ 持锁期间返回 ⇒ 写路**没走**那把锁。
func gapHoldLockAndRun(t *testing.T, argv []string) (ranWhileLocked bool, rc int) {
	t.Helper()
	lk, err := gapLedgerLock(gapLedgerPath())
	if err != nil {
		t.Fatalf("测试进程取账写锁失败：%v", err)
	}
	done := make(chan int, 1)
	go func() {
		var o, e bytes.Buffer
		done <- RunForTest(argv, &o, &e)
	}()
	select {
	case r := <-done:
		gapLedgerUnlock(lk)
		return true, r
	case <-time.After(600 * time.Millisecond):
	}
	gapLedgerUnlock(lk) // 放锁 ⇒ 命令应取到锁、继续跑完
	select {
	case r := <-done:
		return false, r
	case <-time.After(20 * time.Second):
		t.Fatalf("放锁后命令仍未返回（疑似死锁 / 未取锁）：argv=%v", argv)
		return false, 0
	}
}

// ① `gap add`：写路必须落在账写排他锁里。
func TestGapAddWriteIsUnderLedgerLock(t *testing.T) {
	seedGapLedger(t, `{"id":"GAP-20261010-12-SEED"}`)
	argv := []string{"gap", "add", "--symptom", "锁牙：add", "--handmade", "锁牙",
		"--impact", "命令面", "--want-family", "gap", "--want-action", "ls",
		"--repro-cmd", "zerg gap verify-one --dry-run", "--verify-cmd", "zerg version --json name", "--yes"}
	locked, rc := gapHoldLockAndRun(t, argv)
	if locked {
		t.Fatalf("`gap add` 在测试进程持锁期间就返回了 ⇒ 写路**没走**账写排他锁（换线到 gapLedgerApply 未生效）")
	}
	if rc != 0 {
		t.Fatalf("放锁后 `gap add` 应 rc=0，得到 %d", rc)
	}
	if n := gapLedgerLinesForTest(t); n != 2 {
		t.Fatalf("`gap add` 应把真源 1 行写成 2 行，得到 %d 行", n)
	}
}

// ② `gap verify`：写路必须落在账写排他锁里。
// 靶子取 `state=已解` + 判据 `zerg version --json name`（rc=0）⇒ 判决 `已解→已解`（不触发销案两态闸）
// 且 `red=0` ⇒ 真跑到写路且 rc=0（不会在写之前以 2 早退）。
func TestGapVerifyWriteIsUnderLedgerLock(t *testing.T) {
	seedGapLedger(t, `{"id":"GAP-20261010-12-SEED","state":"已解","verify_cmd":"zerg version --json name"}`)
	argv := []string{"gap", "verify", "GAP-20261010-12-SEED", "--yes"}
	locked, rc := gapHoldLockAndRun(t, argv)
	if locked {
		t.Fatalf("`gap verify` 在测试进程持锁期间就返回了 ⇒ 写路**没走**账写排他锁（换线未生效）")
	}
	if rc != 0 {
		t.Fatalf("放锁后 `gap verify` 应 rc=0，得到 %d", rc)
	}
}

// ③ `gap backfill-unit`：写路必须落在账写排他锁里。
// 靶子一条**缺三键**的账（`unit` / `module` / `unit_source` 皆无）⇒ `need>0` ⇒ 真跑到写路。
func TestGapBackfillUnitWriteIsUnderLedgerLock(t *testing.T) {
	seedGapLedger(t, `{"id":"GAP-20261010-12-SEED","symptom":"锁牙：backfill","handmade":"锁牙"}`)
	argv := []string{"gap", "backfill-unit", "--yes"}
	locked, rc := gapHoldLockAndRun(t, argv)
	if locked {
		t.Fatalf("`gap backfill-unit` 在测试进程持锁期间就返回了 ⇒ 写路**没走**账写排他锁（换线未生效）")
	}
	if rc != 0 {
		t.Fatalf("放锁后 `gap backfill-unit` 应 rc=0，得到 %d", rc)
	}
	b, err := os.ReadFile(gapLedgerPath())
	if err != nil {
		t.Fatalf("读真源失败：%v", err)
	}
	if !strings.Contains(string(b), `"unit"`) {
		t.Fatalf("backfill 后真源仍无 unit 键：%s", string(b))
	}
}
