// family_gap_state_apply_test.go —— 缺口账 `GAP-20261010-12` 的两态回归：gap 族写面的并发互斥。
//
// 病（前态）：三条未走 `gapRewriteOne` 的写路（`add` / `verify` / `backfill-unit`）只靠乐观闸
// `gapConcurGate`（写前对一次身份 = check-then-act）—— 闸与「审计先落盘（fsync）」+「整件重写」
// 之间**无锁** ⇒ 两个写者可先后过闸、后写覆盖先写 ⇒ **静默丢更新**。
//
// 两态（缺一即失败）：
//
//	① 前态（负控 · 证「乐观闸」有洞）：两写者**都**在各自写之前过了闸 ⇒ 各自按**自己读入的旧基线**
//	   整件重写 ⇒ 后写覆盖先写 ⇒ 先写那一行**丢了**（本用例断言这个洞**在**）。
//	② 后态（本笔修 · 证「锁内现读」无洞）：同样两写者改走 `gapLedgerApply`（取排他锁 → 锁内现读 →
//	   原子替换 → 回读）⇒ 后写者在锁内看到先写者已落的行 ⇒ 两行**都在**（零丢更新）。
package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// seedGapLedger —— 落一份临时真源（每行一条 JSON 记录）并把状态目录指过去。
func seedGapLedger(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	p := gapLedgerPath()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("seed 真源失败：%v", err)
	}
	return p
}

// TestGapOptimisticGateLosesUpdate —— 前态（负控）：乐观闸放行的两写者，后写**覆盖**先写 ⇒ 丢更新。
func TestGapOptimisticGateLosesUpdate(t *testing.T) {
	p := seedGapLedger(t, `{"id":"A"}`)

	// 两写者各自**读入**同一份旧基线。
	led1, err := readGapLedger()
	if err != nil {
		t.Fatalf("读真源失败：%v", err)
	}
	led2, err := readGapLedger()
	if err != nil {
		t.Fatalf("读真源失败：%v", err)
	}

	// 两写者各自过乐观闸 —— 此刻盘上仍是旧基线 ⇒ **都放行**（这正是 check-then-act 的洞：
	// 闸放行之后、整件重写之前，没有任何东西挡住第二个写者）。
	if d, rc, _ := gapConcurGate(led1, "verify"); rc != exitOK {
		t.Fatalf("写者1 过闸应放行，得到 detail=%q rc=%d", d, rc)
	}
	if d, rc, _ := gapConcurGate(led2, "verify"); rc != exitOK {
		t.Fatalf("写者2 过闸应放行，得到 detail=%q rc=%d", d, rc)
	}

	// 各自按**自己读入那一刻的旧基线**构建整件新行（照 `verify` / `backfill-unit` 真路）。
	if err := gapWriteLedger(p, append(append([]string{}, led1.Lines...), `{"id":"B"}`)); err != nil {
		t.Fatalf("写者1 落盘失败：%v", err)
	}
	if err := gapWriteLedger(p, append(append([]string{}, led2.Lines...), `{"id":"C"}`)); err != nil {
		t.Fatalf("写者2 落盘失败：%v", err)
	}

	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), `{"id":"C"}`) {
		t.Fatalf("后写者的行应在盘上，实际=%q", string(got))
	}
	if strings.Contains(string(got), `{"id":"B"}`) {
		t.Fatalf("前态应**丢更新**（B 被后写者整件覆盖），实际盘上=%q —— B 还在说明闸/写之间已有真互斥", string(got))
	}
}

// TestGapApplyLockedSerialWriters —— 后态（本笔修）：两写者走 `gapLedgerApply` ⇒ 都在（零丢更新）。
func TestGapApplyLockedSerialWriters(t *testing.T) {
	seedGapLedger(t, `{"id":"A"}`)
	for _, id := range []string{"B", "C"} {
		line := `{"id":"` + id + `"}`
		if d, rc, m := gapLedgerApply(func(fresh gapLedger) ([]string, *gapApplyFail) {
			return append(append([]string{}, fresh.Lines...), line), nil
		}); rc != exitOK {
			t.Fatalf("写者 %s 应成功，得到 detail=%q rc=%d msg=%q", id, d, rc, m)
		}
	}
	got, _ := os.ReadFile(gapLedgerPath())
	if !strings.Contains(string(got), `{"id":"B"}`) || !strings.Contains(string(got), `{"id":"C"}`) {
		t.Fatalf("后态应零丢更新（B 与 C 都在），实际=%q", string(got))
	}
}

// TestGapApplyLockedConcurrentNoLostUpdate —— 后态（本笔修）：N 个并发写者走 `gapLedgerApply` ⇒ 零丢更新。
func TestGapApplyLockedConcurrentNoLostUpdate(t *testing.T) {
	seedGapLedger(t, `{"id":"A"}`)
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			line := `{"id":"R` + strconv.Itoa(i) + `"}`
			if d, rc, m := gapLedgerApply(func(fresh gapLedger) ([]string, *gapApplyFail) {
				return append(append([]string{}, fresh.Lines...), line), nil
			}); rc != exitOK {
				t.Errorf("写者%d 应成功，得到 detail=%q rc=%d msg=%q", i, d, rc, m)
			}
		}(i)
	}
	wg.Wait()

	got, _ := os.ReadFile(gapLedgerPath())
	s := string(got)
	for i := 0; i < n; i++ {
		need := `{"id":"R` + strconv.Itoa(i) + `"}`
		if !strings.Contains(s, need) {
			t.Fatalf("并发后态应零丢更新，缺 %s（盘上=%q）", need, s)
		}
	}
}

// TestGapApplySurfacesBuildFail —— `build` 的提前拒写要**原样透出** (detail, rc, msg)，✗ 不塌陷成单一码。
func TestGapApplySurfacesBuildFail(t *testing.T) {
	seedGapLedger(t, `{"id":"A"}`)
	d, rc, m := gapLedgerApply(func(fresh gapLedger) ([]string, *gapApplyFail) {
		return nil, &gapApplyFail{"concurrent_write", exitConflict, "撞车了"}
	})
	if d != "concurrent_write" || rc != exitConflict || m != "撞车了" {
		t.Fatalf("build 拒写应原样透出，得到 detail=%q rc=%d msg=%q", d, rc, m)
	}
}
