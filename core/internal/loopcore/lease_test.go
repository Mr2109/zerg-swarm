// lease_test.go — T5.6 实证：**session 级 lease 并发恢复互斥（执行前置门）**。
//
// 三条用例（每条都带反例，防"把一切都拒了"这种假绿）：
//
//	④ ★**并发两次恢复 ⇒ 效果恰好一次**：假"受闸副作用"计数器为证；落败者被拒 ⇒ 计数仍为 1，
//	  且落败者**一条节点都没执行**、**一个字节状态都没写**，判词点名当前持有者。
//	  另附顺序重放（时间错开）与"不同 session 互不牵连"两条反例。
//	⑤ TTL 到期 ⇒ 另一方**可接管**（fence 递增）；续期真的顺延；过期后**拒绝**续期（不悄悄复活）；
//	  非持有者**不许**释放别人的租约。时间全部**注入**（不读系统钟）。
//	⑥ 反例：**不同 session 互不影响**（各拿各的门，fence 各自独立；一个会话的记录坏掉不牵连别的会话）。
package loopcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── 用例④：并发两次恢复 ⇒ 效果恰好一次 ──────────────────────────────────────

func TestLease_ConcurrentRecoverExactlyOnce(t *testing.T) {
	base := t.TempDir()
	cpDir := filepath.Join(base, "cp")
	leaseDir := filepath.Join(base, "lease")
	const runID, sess = "run-conc", "sess-conc"

	// ── 造一个"被杀在半路"的 run：停在 step=1、可继续（max_rounds）──
	seed, err := NewCheckpointStore(cpDir, DurabilitySync)
	if err != nil {
		t.Fatalf("建 store：%v", err)
	}
	res0 := Run(context.Background(), Config{MaxRounds: 1}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{toolRound("c0", "read", "path", "seed.txt")}, nil),
		Exec: func(ctx context.Context, name string, args map[string]any) (string, string, error) {
			return "种子输出", "1ms", nil
		},
		Checkpoints: seed, RunID: runID, Session: sess,
	})
	if res0.ExitKind != "max_rounds" {
		t.Fatalf("前置条件：应停在可继续的终局，实得 %q", res0.ExitKind)
	}
	seedSnap, err := seed.Load(runID)
	if err != nil || seedSnap.StepIndex != 1 {
		t.Fatalf("前置条件：应停在 step=1，实得 %+v err=%v", seedSnap, err)
	}
	seedRaw, err := os.ReadFile(filepath.Join(cpDir, runID+".jsonl"))
	if err != nil {
		t.Fatalf("读种子快照：%v", err)
	}
	seedLines := strings.Count(string(seedRaw), "\n") // 种子：step1 步快照 + step1 终局快照（max_rounds）
	if err := seed.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}

	// ── 假"受闸副作用"：两次恢复都跑它的话，计数就会变成 2（这正是实测里 saturation=1.0 的形状）──
	var effects int32
	firstExec := make(chan struct{}) // 第一次执行已经开始
	attempted := make(chan struct{}) // 两次恢复都已尝试过上门（保证时间上真的重叠）
	var attemptedOnce sync.Once
	exec := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		n := atomic.AddInt32(&effects, 1)
		if n == 1 {
			// 窗口 = 节点自身执行时间（设计稿实测的双跑窗口）。第一次执行停在这里等，
			// 于是"另一个恢复者"必定在持有者还在执行时就上门 ⇒ 它必须被前置门拒绝。
			close(firstExec)
			<-attempted
		}
		return "副作用输出", "1ms", nil
	}

	holders := []string{"recoverer-1", "recoverer-2"}
	results := make([]*Result, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// 两个恢复者各持**自己的 store 实例**（= 两个进程，各有各的内存态与句柄）
			cp, err := NewCheckpointStore(cpDir, DurabilitySync)
			if err != nil {
				errs[i] = err
				return
			}
			up, err := cp.Load(runID)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = Run(context.Background(), Config{MaxRounds: 2}, "m1", "sys", nil, Deps{
				Infer: scriptedInfer([]Response{
					toolRound("c1", "write", "path", "work.txt"),
					{Content: "恢复完成-" + holders[i]},
				}, nil),
				Exec: exec, Checkpoints: cp, RunID: runID, Session: sess, Resume: up,
				Lease: NewLeaseStore(leaseDir), Holder: holders[i], LeaseTTL: time.Minute,
			})
			attemptedOnce.Do(func() { close(attempted) })
		}(i)
	}
	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("超时：并发恢复没有收敛（死锁或等锁过久）")
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("第 %d 个恢复者建/读失败：%v", i, err)
		}
	}

	// ── 核心断言 ①：效果**恰好一次** ──
	if n := atomic.LoadInt32(&effects); n != 1 {
		t.Fatalf("并发两次恢复 ⇒ 受闸副作用必须恰好一次：实得 %d 次", n)
	}
	// ── 核心断言 ②：恰好一个执行者，落败者是**前置门**拒的（不是跑完才去重）──
	executed, rejected := -1, -1
	for i, r := range results {
		if r == nil {
			t.Fatalf("第 %d 个恢复者没有返回结果", i)
		}
		if r.ExitKind == "lease_rejected" {
			rejected = i
			continue
		}
		if executed != -1 {
			t.Fatalf("两个恢复者都执行了（kind=%q / %q）", results[0].ExitKind, results[1].ExitKind)
		}
		executed = i
	}
	if executed == -1 || rejected == -1 {
		t.Fatalf("必须恰好一个执行者 + 一个被拒：executed=%d rejected=%d（%q / %q）",
			executed, rejected, results[0].ExitKind, results[1].ExitKind)
	}
	lr := results[rejected]
	if len(lr.Traces) != 0 {
		t.Fatalf("落败者不得执行任何节点（traces=%d）", len(lr.Traces))
	}
	if !strings.Contains(lr.Err, holders[executed]) {
		t.Fatalf("落败者的判词必须点名当前持有者 %q：%s", holders[executed], lr.Err)
	}
	// ── 核心断言 ③：落败者**没写** run 状态（3 行 = 种子 step1 + 赢家 step2 + 赢家终局 step3）──
	raw, err := os.ReadFile(filepath.Join(cpDir, runID+".jsonl"))
	if err != nil {
		t.Fatalf("读快照：%v", err)
	}
	if n := strings.Count(string(raw), "\n"); n != seedLines+2 {
		t.Fatalf("落败者不该写状态：期望 %d 行（种子 %d + 赢家的步快照/终局快照），实得 %d 行",
			seedLines+2, seedLines, n)
	}
	// ── 赢家确实"接着跑"（不是从零重来）：终局状态里轨迹 = 种子 1 条 + 新 1 条 ──
	after, _ := NewCheckpointStore(cpDir, DurabilitySync)
	final, err := after.Load(runID)
	if err != nil {
		t.Fatalf("赢家终局快照应可读：%v", err)
	}
	if len(final.State.Traces) != 2 || Resumable(final.State.ExitKind) {
		t.Fatalf("赢家应从 step=1 接到 step≥2 并给终答：traces=%d kind=%q",
			len(final.State.Traces), final.State.ExitKind)
	}
	if final.StepIndex < 2 {
		t.Fatalf("赢家应推进到第 2 步之后：%d", final.StepIndex)
	}

	// ── 反例 A：**时间错开**的顺序重放（lease 早已释放）⇒ 仍不得再执行一次 ──
	// 第二道闸：完成过的 run（已给终答）不再恢复 —— 这是"效果恰好一次"在非并发形状下的那一半。
	cp3, _ := NewCheckpointStore(cpDir, DurabilitySync)
	up3, err := cp3.Load(runID)
	if err != nil {
		t.Fatalf("Load：%v", err)
	}
	res3 := Run(context.Background(), Config{MaxRounds: 2}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{toolRound("c9", "write", "path", "again.txt")}, nil),
		Exec:  exec, Checkpoints: cp3, RunID: runID, Session: sess, Resume: up3,
		Lease: NewLeaseStore(leaseDir), Holder: "recoverer-3", LeaseTTL: time.Minute,
	})
	if res3.ExitKind != "already_done" {
		t.Fatalf("已完成 run 的顺序重放必须被拒：%q", res3.ExitKind)
	}
	if n := atomic.LoadInt32(&effects); n != 1 {
		t.Fatalf("顺序重放也不得追加副作用：实得 %d 次", n)
	}

	// ── 反例 B：**不同 session** 的门互不牵连（证明拒绝是"按会话"，不是"一律拒"）──
	other := Run(context.Background(), Config{MaxRounds: 1}, "m1", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{{Content: "别的会话"}}, nil),
		Exec:  exec, Checkpoints: cp3, RunID: "run-other", Session: "sess-other",
		Lease: NewLeaseStore(leaseDir), Holder: "recoverer-4", LeaseTTL: time.Minute,
	})
	if other.ExitKind != "natural" {
		t.Fatalf("另一个会话必须能拿到自己的门：%q（%s）", other.ExitKind, other.Err)
	}
}

// ── 用例⑤：TTL 到期 ⇒ 可接管；续期/释放的边界 ───────────────────────────────

func TestLease_TTLExpiryAllowsTakeover(t *testing.T) {
	dir := t.TempDir()
	ls := NewLeaseStore(dir)
	t0 := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) // 冻结时钟：注入的 now 就是唯一时间来源

	a, err := ls.Acquire("sess-1", "A", 30*time.Second, t0)
	if err != nil {
		t.Fatalf("A 首次认领应成功：%v", err)
	}
	if a.Fence != 1 || !a.AcquiredAt.Equal(t0) || !a.ExpiresAt.Equal(t0.Add(30*time.Second)) {
		t.Fatalf("租约时间必须来自**注入的钟**（不读系统钟）：%+v", a)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sess-1.lease.json"))
	if err != nil {
		t.Fatalf("租约必须落盘（要跨进程/跨崩溃存活）：%v", err)
	}
	if !strings.Contains(string(raw), "2026-09-17T12:00:00Z") {
		t.Fatalf("落盘内容应带注入的时间：%s", raw)
	}

	// 未到期 ⇒ 第二方**直接拒绝**（专项错误码 + 当前持有者）
	_, err = ls.Acquire("sess-1", "B", 30*time.Second, t0.Add(time.Second))
	var he *HeldError
	if !errors.As(err, &he) || !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("未到期时抢占必须返回 ErrLeaseHeld：%v", err)
	}
	if he.Lease == nil || he.Lease.Holder != "A" {
		t.Fatalf("错误里必须带当前持有者：%+v", he)
	}
	if !strings.Contains(err.Error(), "A") || !strings.Contains(err.Error(), "2026-09-17T12:00:30Z") {
		t.Fatalf("判词应点名持有者与到期时刻：%s", err.Error())
	}

	// 持有者可续期（fence 不变：还是同一次持有）
	a2, err := ls.Renew("sess-1", "A", 30*time.Second, t0.Add(10*time.Second))
	if err != nil {
		t.Fatalf("持有者续期应成功：%v", err)
	}
	if a2.Fence != 1 || !a2.ExpiresAt.Equal(t0.Add(40*time.Second)) {
		t.Fatalf("续期应把到期时间顺延、fence 不变：%+v", a2)
	}
	// 续期真的生效（不是只写了个新时间戳）：t0+39s 仍被拒
	if _, err := ls.Acquire("sess-1", "B", 30*time.Second, t0.Add(39*time.Second)); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("续期后未到期 ⇒ 仍必须拒绝：%v", err)
	}

	// ── TTL 到期（now ≥ ExpiresAt）⇒ 另一方**可以接管**，fence 递增 ──
	b, err := ls.Acquire("sess-1", "B", 30*time.Second, t0.Add(40*time.Second))
	if err != nil {
		t.Fatalf("到期后应可接管：%v", err)
	}
	if b.Holder != "B" || b.Fence != 2 {
		t.Fatalf("接管后持有者=B、fence 递增到 2：%+v", b)
	}

	// 被接管者：续期与释放都**被拒**（且判词点名新的持有者）
	_, err = ls.Renew("sess-1", "A", 30*time.Second, t0.Add(41*time.Second))
	if !errors.As(err, &he) || !errors.Is(err, ErrLeaseNotHolder) || he.Lease.Holder != "B" {
		t.Fatalf("被接管者续期必须 ErrLeaseNotHolder 且带新持有者：%v", err)
	}
	if err := ls.Release("sess-1", "A"); !errors.Is(err, ErrLeaseNotHolder) {
		t.Fatalf("非持有者不得释放别人的租约（那等于把门拆了）：%v", err)
	}
	cur, err := ls.Holder("sess-1")
	if err != nil || cur == nil || cur.Holder != "B" {
		t.Fatalf("上面那次释放不得动到 B 的租约：%+v err=%v", cur, err)
	}
	if !cur.Active(t0.Add(69*time.Second)) || cur.Active(t0.Add(70*time.Second)) {
		t.Fatalf("Active 的判据只有时间：%+v", cur)
	}

	// ── 过期后**拒绝续期**（不悄悄复活：期间座位可能已被接管）──
	if _, err := func() (*Lease, error) {
		if _, err := ls.Acquire("sess-1", "A", 10*time.Second, t0.Add(100*time.Second)); err != nil {
			return nil, err
		}
		return ls.Renew("sess-1", "A", 10*time.Second, t0.Add(200*time.Second))
	}(); !errors.Is(err, ErrLeaseNotHolder) {
		t.Fatalf("过期后续期必须拒绝（须重新 Acquire）：%v", err)
	}

	// Release 幂等；释放后座位空出
	if err := ls.Release("sess-1", "A"); err != nil {
		t.Fatalf("持有者释放应成功：%v", err)
	}
	if err := ls.Release("sess-1", "A"); err != nil {
		t.Fatalf("重复释放必须幂等：%v", err)
	}
	if cur, err := ls.Holder("sess-1"); err != nil || cur != nil {
		t.Fatalf("释放后应无租约：%+v err=%v", cur, err)
	}
	// 释放后另一方可以立刻认领；fence 归 1（**语义如实**：Release 之后座位空出、代数重新开始 ——
	// 见 Lease 的"单调性的确切范围"；挡陈旧持有者靠的是持有者身份，下面那条断言就是它）
	c, err := ls.Acquire("sess-1", "C", time.Minute, t0.Add(300*time.Second))
	if err != nil || c.Fence != 1 || c.Holder != "C" {
		t.Fatalf("释放后应可认领且代数重新开始：%+v err=%v", c, err)
	}
	// 陈旧持有者（A，上一段序列里的 fence=3）在 C 持有期间**必须被挡**（不依赖 fence 的那一道）
	if _, err := ls.Renew("sess-1", "A", time.Minute, t0.Add(301*time.Second)); !errors.Is(err, ErrLeaseNotHolder) {
		t.Fatalf("上一段序列的陈旧持有者必须被挡：%v", err)
	}
}

// ── 用例⑥：反例——不同 session 互不影响 ─────────────────────────────────────

func TestLease_DifferentSessionsIndependent(t *testing.T) {
	dir := t.TempDir()
	ls := NewLeaseStore(dir)
	t0 := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)

	s1, err := ls.Acquire("sess-1", "A", time.Minute, t0)
	if err != nil {
		t.Fatalf("sess-1：%v", err)
	}
	s2, err := ls.Acquire("sess-2", "A", time.Minute, t0)
	if err != nil {
		t.Fatalf("同一持有者拿另一个会话的门应成功：%v", err)
	}
	s3, err := ls.Acquire("sess-3", "B", time.Minute, t0)
	if err != nil {
		t.Fatalf("另一个持有者拿第三个会话的门应成功：%v", err)
	}
	if s1.Fence != 1 || s2.Fence != 1 || s3.Fence != 1 {
		t.Fatalf("各会话的认领代数互不串：%d/%d/%d", s1.Fence, s2.Fence, s3.Fence)
	}

	// 反例（证明不是"一律拒"）：**同一个**会话仍然互斥
	if _, err := ls.Acquire("sess-2", "B", time.Minute, t0); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("同一会话的第二方必须被拒：%v", err)
	}
	if _, err := ls.Acquire("sess-1", "B", time.Minute, t0); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("同一会话的第二方必须被拒（sess-1）：%v", err)
	}

	// 释放 sess-1 不牵连 sess-2
	if err := ls.Release("sess-1", "A"); err != nil {
		t.Fatalf("释放 sess-1：%v", err)
	}
	if got, err := ls.Holder("sess-1"); err != nil || got != nil {
		t.Fatalf("sess-1 应已释放：%+v %v", got, err)
	}
	if got, err := ls.Holder("sess-2"); err != nil || got == nil || got.Holder != "A" {
		t.Fatalf("sess-2 不该被牵连：%+v %v", got, err)
	}

	// 一个会话的记录坏掉 ⇒ **只**让该会话 fail-closed，别的会话照常工作
	badPath := filepath.Join(dir, "sess-3.lease.json")
	if err := os.WriteFile(badPath, []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatalf("造损坏夹具：%v", err)
	}
	if _, err := ls.Acquire("sess-3", "C", time.Minute, t0); !errors.Is(err, ErrLeaseCorrupt) {
		t.Fatalf("损坏记录必须 fail-closed（宁可都不跑，也不放两个执行者进来）：%v", err)
	}
	if _, err := ls.Holder("sess-3"); !errors.Is(err, ErrLeaseCorrupt) {
		t.Fatalf("查询也必须报损坏（而不是假装没有租约）：%v", err)
	}
	if _, err := ls.Acquire("sess-2", "B", time.Minute, t0); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("别的会话的正常判定不该受影响：%v", err)
	}
	if err := ls.Release("sess-2", "A"); err != nil {
		t.Fatalf("别的会话应能正常释放：%v", err)
	}

	// 参数非法：空 session / 空 holder / ttl ≤0 一律报错（不静默当成"拿到了"）
	if _, err := ls.Acquire("", "A", time.Minute, t0); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("空 session 应报 ErrLeaseInvalid：%v", err)
	}
	if _, err := ls.Acquire("sess-x", "  ", time.Minute, t0); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("空 holder 应报 ErrLeaseInvalid：%v", err)
	}
	if _, err := ls.Acquire("sess-x", "A", 0, t0); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("ttl=0 应报 ErrLeaseInvalid（否则崩溃一次就永久锁死）：%v", err)
	}
	// 时间注入：零值 now ⇒ 用 store 的 Now（**不是**直接读系统钟）
	frozen := NewLeaseStore(t.TempDir())
	frozen.Now = func() time.Time { return t0 }
	l, err := frozen.Acquire("sess-z", "Z", time.Minute, time.Time{})
	if err != nil || !l.AcquiredAt.Equal(t0) {
		t.Fatalf("零值 now 应走 store 注入的时钟：%+v err=%v", l, err)
	}
	// Enter：拿到门才有执行机会（这是内核用的形态）
	release, lease, err := frozen.Enter("sess-e", "E", time.Minute, t0)
	if err != nil || lease == nil {
		t.Fatalf("Enter 应成功：%v", err)
	}
	if _, _, err := frozen.Enter("sess-e", "F", time.Minute, t0); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("第二个 Enter 必须失败（拿不到执行机会）：%v", err)
	}
	release()
	if got, err := frozen.Holder("sess-e"); err != nil || got != nil {
		t.Fatalf("release 后应释放：%+v %v", got, err)
	}
}

// Run 侧的接线：没有 Session 时不认领（CA/纯计算路径不该被 session 门影响）。
func TestLease_NoSessionNoGate(t *testing.T) {
	dir := t.TempDir()
	ls := NewLeaseStore(dir)
	// 先让"别的进程"占住空会话之外的门；无会话的 run 照样能跑（会话级互斥按会话归属）
	res := Run(context.Background(), Config{MaxRounds: 1}, "m", "sys", nil, Deps{
		Infer: scriptedInfer([]Response{{Content: "ok"}}, nil),
		Lease: ls, Holder: "h",
	})
	if res.ExitKind != "natural" {
		t.Fatalf("无会话时应照常跑（不认领）：%q %s", res.ExitKind, res.Err)
	}
	if cur, err := ls.Holder("dummy"); err != nil || cur != nil {
		t.Fatalf("不该写出任何租约：%+v %v", cur, err)
	}
}
