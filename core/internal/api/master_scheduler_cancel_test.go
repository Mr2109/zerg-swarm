package api

// master_scheduler_cancel_test.go — 任务退场正门 · 刀1（主控侧）定向判据件
// 2026-10-03 · GAP-20261003-04 · 覆盖任务清单 T-03 / T-04 / T-05 / T-06 / T-10。
// （T-09 是登记↔实现分叉的只读对读，无可执行判据，见 master_scheduler.go TerminateTask 文档注释。）
//
// 只跑本件（前缀 ^TestTaskExit 即本件全集，禁截断）:
//   cd core && go test ./internal/api/ -run '^TestTaskExit' -count=1
//
// 成对读数原则（禁放宽断言）:
//   - T-04: mock save 返 error ⇒ 断言 dispatch 决策函数【未被进入】且 CancelTask 明确失败（红控）；
//           真 save 成功 ⇒ 断言 dispatch 决策函数【被进入】（绿控）。
//   - T-05: 对 done 调 CancelTask ⇒ status 一字未变（"done"）+ 只打 closed_* 正交标记。
//   - T-06: 打标后【跨实例读回】四条齐 ⟷ 清空后读回四条空（覆盖式非 write-once）。

import (
	"container/heap"
	"errors"
	"strings"
	"testing"
)

// newTaskExitTestScheduler 构造一个不 Start、不派发后台循环的裸调度器（判据可控）。
func newTaskExitTestScheduler(t *testing.T, maxConc int) *MasterScheduler {
	t.Helper()
	isolateTasksFile(t)
	isolateTaskRoot(t)
	return NewMasterScheduler("/bin/echo", maxConc)
}

// taskExitQueueIndex 在优先级队列里按 ID 找下标。
func taskExitQueueIndex(q TaskQueue, id string) (int, bool) {
	for i, t := range q {
		if t.ID == id {
			return i, true
		}
	}
	return -1, false
}

// TestTaskExitIsTerminalPredicate — T-03: is_terminal 派生谓词（三终态各判一次 · 非终态各判一次）。
func TestTaskExitIsTerminalPredicate(t *testing.T) {
	for _, st := range []string{StatusDone, StatusFailed, StatusCancelled} {
		if !(&Task{Status: st}).IsTerminal() {
			t.Errorf("IsTerminal(%q) 应为 true（终态 done|failed|cancelled）", st)
		}
	}
	for _, st := range []string{StatusQueued, StatusRunning, StatusPaused, StatusWaitingRetry, "", "archived", "terminated"} {
		if (&Task{Status: st}).IsTerminal() {
			t.Errorf("IsTerminal(%q) 应为 false（非终态 + 未知态）", st)
		}
	}
}

// TestTaskExitCancelFromQueued — T-04 绿控/容器一致性: queued → cancelled，摘 queue 落 history，
// save 成功后进入 dispatch 决策（占位）。
func TestTaskExitCancelFromQueued(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	probe := 0
	dispatchDecideProbe = func() { probe++ }
	defer func() { dispatchDecideProbe = nil }()

	a := &Task{ID: "cancel-a", Description: "x", Priority: PriorityExternal, Status: StatusQueued}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-a", "superseded", "前提已变"); err != nil {
		t.Fatalf("取消 queued 任务应成功，实际 err=%v", err)
	}

	s.mu.Lock()
	_, inQueue := taskExitQueueIndex(s.queue, "cancel-a")
	_, inRunning := s.running["cancel-a"]
	_, inWaiting := s.waiting["cancel-a"]
	h, inHist := s.history["cancel-a"]
	s.mu.Unlock()

	if inQueue {
		t.Errorf("cancelled 件不应还在 queue")
	}
	if inRunning {
		t.Errorf("cancelled 件不应在 running")
	}
	if inWaiting {
		t.Errorf("cancelled 件不应在 waiting")
	}
	if !inHist || h == nil {
		t.Fatalf("cancelled 件必须在 history（容器一致性·血证 §2.6）")
	}
	if h.Status != StatusCancelled {
		t.Errorf("status 应为 cancelled，实际 %q", h.Status)
	}
	if !h.IsTerminal() {
		t.Errorf("cancelled 件应判终态")
	}
	if probe == 0 {
		t.Errorf("取消成功路径必须进入 dispatch 决策（占位下一个），探针未被触发")
	}
}

// TestTaskExitCancelFromPaused — T-05 守卫: paused 也可取消。
func TestTaskExitCancelFromPaused(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	p := &Task{ID: "cancel-p", Status: StatusPaused, Priority: PriorityInternal}
	s.mu.Lock()
	heap.Push(&s.queue, p)
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-p", "obsolete", "前提已错"); err != nil {
		t.Fatalf("取消 paused 任务应成功，实际 err=%v", err)
	}
	s.mu.Lock()
	h := s.history["cancel-p"]
	_, inQueue := taskExitQueueIndex(s.queue, "cancel-p")
	s.mu.Unlock()
	if h == nil || h.Status != StatusCancelled {
		t.Fatalf("paused→cancelled 失败: %+v", h)
	}
	if inQueue {
		t.Errorf("cancelled 件不应还在 queue")
	}
}

// TestTaskExitCancelDoneMarkerOnly — T-05 负控: 对 done 调 CancelTask ⇒ status 一字未变，
// 只打 closed_* 正交标记（§4.1「不是不给销案，而是不能覆写」）。
func TestTaskExitCancelDoneMarkerOnly(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	probe := 0
	dispatchDecideProbe = func() { probe++ }
	defer func() { dispatchDecideProbe = nil }()

	done := &Task{ID: "cancel-done", Status: StatusDone, Priority: PriorityExternal}
	s.mu.Lock()
	s.history["cancel-done"] = done
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-done", "superseded", "已被后续取代"); err != nil {
		t.Fatalf("对 done 销案应成功（只打标），实际 err=%v", err)
	}
	s.mu.Lock()
	st := done.Status
	at, by, reason, note := done.ClosedAt, done.ClosedBy, done.ClosedReason, done.ClosedNote
	_, inQueue := taskExitQueueIndex(s.queue, "cancel-done")
	_, inRunning := s.running["cancel-done"]
	s.mu.Unlock()

	if st != StatusDone {
		t.Fatalf("负控: 对 done 调 CancelTask 后 status 必须一字未变，实际 %q（覆写会永久丢掉「到底成没成」）", st)
	}
	if at == nil || by == "" || reason != "superseded" || note != "已被后续取代" {
		t.Errorf("只打标路径应写齐 closed_* 四条: at=%v by=%q reason=%q note=%q", at, by, reason, note)
	}
	if inQueue || inRunning {
		t.Errorf("done 销案不应改容器归属（inQueue=%v inRunning=%v）", inQueue, inRunning)
	}
	if probe != 0 {
		t.Errorf("只打标路径不空出并发槽，不应进入派发决策，探针被触发 %d 次", probe)
	}
}

// TestTaskExitCancelFailedMarkerOnly — T-05 守卫: failed 同样只打标、不覆写 status。
func TestTaskExitCancelFailedMarkerOnly(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	f := &Task{ID: "cancel-failed", Status: StatusFailed, Priority: PriorityExternal}
	s.mu.Lock()
	s.history["cancel-failed"] = f
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-failed", "abandoned", "不再跟进"); err != nil {
		t.Fatalf("对 failed 销案应成功（只打标），实际 err=%v", err)
	}
	s.mu.Lock()
	st := f.Status
	at, reason := f.ClosedAt, f.ClosedReason
	s.mu.Unlock()
	if st != StatusFailed {
		t.Fatalf("负控: 对 failed 调 CancelTask 后 status 必须一字未变，实际 %q", st)
	}
	if at == nil || reason != "abandoned" {
		t.Errorf("只打标路径应写 closed_*: at=%v reason=%q", at, reason)
	}
}

// TestTaskExitCancelRunningRejected — T-05 守卫: running 拒绝（在飞——先 terminate）。
func TestTaskExitCancelRunningRejected(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	probe := 0
	dispatchDecideProbe = func() { probe++ }
	defer func() { dispatchDecideProbe = nil }()

	r := &Task{ID: "cancel-run", Status: StatusRunning, Priority: PriorityExternal}
	s.mu.Lock()
	s.running["cancel-run"] = r
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-run", "abandoned", ""); err == nil {
		t.Fatalf("对 running 应拒绝，实际 err=nil")
	}
	s.mu.Lock()
	st := r.Status
	_, stillRunning := s.running["cancel-run"]
	s.mu.Unlock()
	if st != StatusRunning {
		t.Errorf("拒绝后 running 件 status 不应变，实际 %q", st)
	}
	if !stillRunning {
		t.Errorf("拒绝后 running 件应仍在 running")
	}
	if probe != 0 {
		t.Errorf("拒绝路径不得进入派发决策，探针被触发 %d 次", probe)
	}
}

// TestTaskExitCancelWaitingRetry — GAP-20261022-02 契约（2026-10-22 改口径）:
// waiting_retry 原先 cancel / terminate **两个门都不收** ⇒ 用户从命令面关不掉它，只能等
// 30 分钟超时降级 failed 或手改状态文件。新口径: waiting_retry 是停着的（未派发、不占并发槽），
// 撤销它与 queued/paused 同语义（cancelled ≠ 完成）⇒ 就地结案。
//
// ★ 两条硬断言: ① 必须从 s.waiting 摘掉；② 跨实例读回（重启恢复路径）不得回队。
func TestTaskExitCancelWaitingRetry(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	w := &Task{ID: "cancel-wait", Status: StatusWaitingRetry, Priority: PriorityExternal}
	s.mu.Lock()
	s.waiting["cancel-wait"] = w
	s.mu.Unlock()

	outcome, err := s.CancelTask("cancel-wait", "abandoned", "等不来恢复，撤掉")
	if err != nil {
		t.Fatalf("waiting_retry 必须可撤（修前回「waiting for environment recovery (waiting_retry tasks cannot be cancelled)」，而 terminate 回「is not running」⇒ 两门皆堵）: err=%v", err)
	}
	if outcome != CancelOutcomeCancelled {
		t.Errorf("outcome 应为 %q，实际 %q", CancelOutcomeCancelled, outcome)
	}
	s.mu.Lock()
	_, inWaiting := s.waiting["cancel-wait"]
	_, inQueue := taskExitQueueIndex(s.queue, "cancel-wait")
	h := s.history["cancel-wait"]
	s.mu.Unlock()
	if inWaiting {
		t.Fatalf("★ 撤销后必须摘出 waiting 分区（留着会被恢复循环捞回去重新派发）")
	}
	if inQueue {
		t.Errorf("撤销后不得在队")
	}
	if h == nil || h.Status != StatusCancelled {
		t.Fatalf("应在 history 里结案为 cancelled，实际 %+v", h)
	}
	if h.ClosedAt == nil || h.ClosedReason != "abandoned" {
		t.Errorf("应写 closed_* 标记: at=%v reason=%q", h.ClosedAt, h.ClosedReason)
	}

	// 跨实例读回: 重启后既不在 waiting 也不在 queue（否则复活）。
	s2 := NewMasterScheduler("/bin/echo", 1)
	s2.mu.Lock()
	got := s2.history["cancel-wait"]
	_, rw := s2.waiting["cancel-wait"]
	_, rq := taskExitQueueIndex(s2.queue, "cancel-wait")
	s2.mu.Unlock()
	if got == nil || got.Status != StatusCancelled {
		t.Fatalf("重启读回应为 cancelled，实际 %+v", got)
	}
	if rw || rq {
		t.Fatalf("★ 撤销后重启不得回 waiting/queue: waiting=%v queue=%v", rw, rq)
	}
}

// TestTaskExitWaitingRecoveryStaleSnapshot — 「取消后不在队」的竞态腿。
//
// 恢复循环在**锁外** ping 时才拍快照；ping 期间用户下了 cancel。快照回锁后若照旧处理:
//   - recovered 腿会把已撤件 applyStatus(queued) + heap.Push ⇒ 已撤件复活回队列；
//   - expired 腿会把 cancelled 覆写成 failed。
//
// 判据: 陈旧快照必须被整条跳过（不覆写、不回队）。
func TestTaskExitWaitingRecoveryStaleSnapshot(t *testing.T) {
	s := newTaskExitTestScheduler(t, 0) // 并发 0 ⇒ 恢复后不真派发，读数稳定在容器面
	w := &Task{ID: "wait-race", Status: StatusWaitingRetry, Priority: PriorityExternal}
	// 另一条仍在等待分区的件（对照: 陈旧判定只能跳过已撤的那条，不能一刀切成 no-op）。
	alive := &Task{ID: "wait-alive", Status: StatusWaitingRetry, Priority: PriorityExternal}
	s.mu.Lock()
	s.waiting["wait-race"] = w
	s.waiting["wait-alive"] = alive
	s.mu.Unlock()

	// 用户撤销其中的 wait-race（删除 s.waiting + 落 history=cancelled）。
	if _, err := s.CancelTask("wait-race", "obsolete", "竞态用例"); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}

	// 陈旧快照回锁：两条分别以「已恢复」与「已超时」身份回来——都不得动已撤件。
	s.mu.Lock()
	s.applyWaitingRecoveryLocked([]*Task{w}, []*Task{alive})
	_, raceInQueue := taskExitQueueIndex(s.queue, "wait-race")
	_, raceInWaiting := s.waiting["wait-race"]
	raceStatus := w.Status
	_, aliveInQueue := taskExitQueueIndex(s.queue, "wait-alive")
	_, aliveInWaiting := s.waiting["wait-alive"]
	aliveStatus := alive.Status
	s.mu.Unlock()

	if raceInQueue {
		t.Fatalf("★ 已撤件被恢复快照回队（复活）: wait-race 现读 queued —— 「取消后不在队」被破")
	}
	if raceInWaiting {
		t.Errorf("已撤件不得回到 waiting 分区")
	}
	if raceStatus != StatusCancelled {
		t.Errorf("★ 已撤件状态被覆写: 期望 cancelled，实际 %q（T-05「绝不覆写终态」）", raceStatus)
	}
	// 正控: 仍在等待分区的那条必须照常处理（否则「跳过」写成了「整轮 no-op」）。
	// 注意: 恢复后它可能已被 dispatchLocked 拉进 running（并发位空着），故两处都认。
	_, aliveInRunning := s.running["wait-alive"]
	if !aliveInQueue && !aliveInRunning {
		t.Errorf("对照件 wait-alive 应被恢复出等待分区进 queue（正控: 过滤不能一刀切）")
	}
	if aliveInWaiting || aliveStatus == StatusWaitingRetry {
		t.Errorf("对照件应离开 waiting 并被重置为 queued: inWaiting=%v status=%q", aliveInWaiting, aliveStatus)
	}

	// ---- 第二腿: 已撤件以「已恢复」身份回到陈旧快照 ⇒ 绝不得回队（复活）----
	w2 := &Task{ID: "wait-race-recovered", Status: StatusWaitingRetry, Priority: PriorityExternal}
	s.mu.Lock()
	s.waiting["wait-race-recovered"] = w2
	s.mu.Unlock()
	if _, err := s.CancelTask("wait-race-recovered", "obsolete", "竞态用例(恢复腿)"); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	s.mu.Lock()
	s.applyWaitingRecoveryLocked(nil, []*Task{w2})
	_, rq2 := taskExitQueueIndex(s.queue, "wait-race-recovered")
	rw2 := s.waiting["wait-race-recovered"] != nil
	st2 := w2.Status
	s.mu.Unlock()
	if rq2 || rw2 {
		t.Fatalf("★ 已撤件被恢复快照复活回队: inQueue=%v inWaiting=%v —— 「取消后不在队」被破", rq2, rw2)
	}
	if st2 != StatusCancelled {
		t.Errorf("已撤件状态被覆写: 期望 cancelled，实际 %q", st2)
	}
}

// TestTaskExitCancelNotFound — §11.2: id 不存在 ⇒ 报错。
func TestTaskExitCancelNotFound(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	if _, err := s.CancelTask("no-such-task", "duplicate", ""); err == nil {
		t.Fatalf("不存在的 id 应报错（不得静默吞）")
	}
}

// TestTaskExitCancelUnknownReasonRejected — §11.5: 理由必须落在闭集内，非法即返回、不落任何写。
func TestTaskExitCancelUnknownReasonRejected(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	a := &Task{ID: "cancel-badreason", Status: StatusQueued, Priority: PriorityExternal}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-badreason", "because", ""); err == nil {
		t.Fatalf("闭集外理由应拒绝，实际 err=nil")
	}
	s.mu.Lock()
	st := a.Status
	s.mu.Unlock()
	if st != StatusQueued {
		t.Errorf("理由非法应不落任何写，status 应仍 queued，实际 %q", st)
	}
}

// TestTaskExitCancelAlreadyCancelledNoOp — §11.2/F-6: 关已关的 ⇒ 幂等静默成功、无副作用。
func TestTaskExitCancelAlreadyCancelledNoOp(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	c := &Task{ID: "cancel-idem", Status: StatusCancelled, Priority: PriorityExternal}
	c.markClosed("superseded", "首跑")
	before := *c.ClosedAt
	s.mu.Lock()
	s.history["cancel-idem"] = c
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-idem", "obsolete", "重放"); err != nil {
		t.Fatalf("已 cancelled 重放应静默成功，实际 err=%v", err)
	}
	s.mu.Lock()
	st, reason, note, at := c.Status, c.ClosedReason, c.ClosedNote, c.ClosedAt
	s.mu.Unlock()
	if st != StatusCancelled || reason != "superseded" || note != "首跑" || at == nil || !at.Equal(before) {
		t.Errorf("幂等重放必须无副作用: status=%q reason=%q note=%q at=%v(before %v)", st, reason, note, at, before)
	}
}

// TestTaskExitCancelSaveFailureSkipsDispatch — T-04 【红控】: mock save 返 error ⇒
// CancelTask 明确失败 + dispatch 决策函数【未被进入】（不得带着内存已改继续派发）。
func TestTaskExitCancelSaveFailureSkipsDispatch(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	probe := 0
	dispatchDecideProbe = func() { probe++ }
	defer func() { dispatchDecideProbe = nil }()

	saveTasksForCancel = func(TaskQueue, map[string]*Task, map[string]*Task, ...map[string]*Task) error {
		return errors.New("注入的落盘失败（红控）")
	}
	defer func() { saveTasksForCancel = saveTasksLocked }()

	a := &Task{ID: "cancel-savefail", Status: StatusQueued, Priority: PriorityExternal}
	// 另放一条可派发件: 若误派发，它会被移进 running（行为面也能咬住）。
	b := &Task{ID: "cancel-bystander", Status: StatusQueued, Priority: PriorityInternal}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	heap.Push(&s.queue, b)
	s.mu.Unlock()

	_, err := s.CancelTask("cancel-savefail", "abandoned", "")
	if err == nil {
		t.Fatalf("save 失败必须明确失败（不得静默继续），实际 err=nil")
	}
	if probe != 0 {
		t.Fatalf("save 失败后 dispatch 决策函数【不得被进入】，实际被触发 %d 次", probe)
	}
	s.mu.Lock()
	_, bInRunning := s.running["cancel-bystander"]
	_, bInQueue := taskExitQueueIndex(s.queue, "cancel-bystander")
	s.mu.Unlock()
	if bInRunning {
		t.Errorf("误派发旁证: 旁观件 cancel-bystander 不应被派进 running")
	}
	if !bInQueue {
		t.Errorf("save 失败不得派发下一个: 旁观件应仍在 queue")
	}
}

// TestTaskExitCancelSuccessEntersDispatch — T-04 【绿控】: 真 save 成功 ⇒ dispatch 决策函数被进入。
func TestTaskExitCancelSuccessEntersDispatch(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	probe := 0
	dispatchDecideProbe = func() { probe++ }
	defer func() { dispatchDecideProbe = nil }()

	a := &Task{ID: "cancel-saveok", Status: StatusQueued, Priority: PriorityExternal}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-saveok", "abandoned", ""); err != nil {
		t.Fatalf("正常取消应成功，实际 err=%v", err)
	}
	if probe == 0 {
		t.Fatalf("save 成功后必须进入 dispatch 决策（占位下一个），探针未被触发")
	}
}

// TestTaskExitClosedMarkersRoundTrip — T-06: 打标后【跨实例读回】四条齐 ⟷ 清空后读回四条空
// （真序列化往返: 另一个 MasterScheduler 走 loadPersistedHistory 从盘读回）。
func TestTaskExitClosedMarkersRoundTrip(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	a := &Task{ID: "cancel-rt", Status: StatusQueued, Priority: PriorityExternal}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-rt", "duplicate", "与 X 重复"); err != nil {
		t.Fatalf("取消失败: %v", err)
	}

	// ---- 打标后: 跨实例读回四条齐 ----
	s2 := NewMasterScheduler("/bin/echo", 1)
	s2.mu.Lock()
	got := s2.history["cancel-rt"]
	s2.mu.Unlock()
	if got == nil {
		t.Fatalf("跨实例读回: 盘上 history 缺 cancel-rt")
	}
	if got.ClosedAt == nil || got.ClosedBy != closedByLocal || got.ClosedReason != "duplicate" || got.ClosedNote != "与 X 重复" {
		t.Fatalf("打标后读回四条不齐: at=%v by=%q reason=%q note=%q",
			got.ClosedAt, got.ClosedBy, got.ClosedReason, got.ClosedNote)
	}
	if !got.IsClosed() {
		t.Errorf("打标后 IsClosed 应为 true")
	}

	// ---- 清空（重开）→ 落盘 → 跨实例读回四条空 ----
	s.mu.Lock()
	if h := s.history["cancel-rt"]; h != nil {
		h.clearClosed()
	}
	err := saveTasksForCancel(s.queue, s.running, s.history, s.waiting)
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("清空落盘失败: %v", err)
	}
	s3 := NewMasterScheduler("/bin/echo", 1)
	s3.mu.Lock()
	got3 := s3.history["cancel-rt"]
	s3.mu.Unlock()
	if got3 == nil {
		t.Fatalf("跨实例读回（清空后）: 盘上 history 缺 cancel-rt")
	}
	if got3.ClosedAt != nil || got3.ClosedBy != "" || got3.ClosedReason != "" || got3.ClosedNote != "" {
		t.Fatalf("清空后读回四条非空: at=%v by=%q reason=%q note=%q",
			got3.ClosedAt, got3.ClosedBy, got3.ClosedReason, got3.ClosedNote)
	}
	if got3.IsClosed() {
		t.Errorf("清空后 IsClosed 应为 false")
	}

	// ---- 覆盖式（非 write-once）: 再打标 ⇒ 覆盖为最新 ----
	s.mu.Lock()
	h := s.history["cancel-rt"]
	h.markClosed("obsolete", "第二次结案")
	err = saveTasksForCancel(s.queue, s.running, s.history, s.waiting)
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("覆盖式落盘失败: %v", err)
	}
	s4 := NewMasterScheduler("/bin/echo", 1)
	s4.mu.Lock()
	got4 := s4.history["cancel-rt"]
	s4.mu.Unlock()
	if got4 == nil || got4.ClosedReason != "obsolete" || got4.ClosedNote != "第二次结案" {
		t.Fatalf("覆盖式打标失败: got=%+v", got4)
	}
}

// TestTaskExitRequeueTaskPersists — T-10: RequeueTask 补一次 saveTasksLocked
// （修前漏落盘 ⇒ 重启后盘上仍是 running ⇒ 恢复分区标 failed、续作丢失）。
func TestTaskExitRequeueTaskPersists(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	r := &Task{ID: "requeue-1", Description: "x", Priority: PriorityExternal, Status: StatusRunning}
	s.mu.Lock()
	s.running["requeue-1"] = r
	s.mu.Unlock()

	if err := s.RequeueTask("requeue-1"); err != nil {
		t.Fatalf("requeue 失败: %v", err)
	}
	s.mu.Lock()
	_, stillRunning := s.running["requeue-1"]
	_, inQueue := taskExitQueueIndex(s.queue, "requeue-1")
	s.mu.Unlock()
	if stillRunning {
		t.Errorf("requeue 后不应还在 running")
	}
	if !inQueue {
		t.Errorf("requeue 后应在 queue")
	}

	// 跨实例读回: 修前漏 save ⇒ 盘上无此件；修后应落盘。
	s2 := NewMasterScheduler("/bin/echo", 1)
	s2.mu.Lock()
	var found *Task
	for _, q := range s2.queue {
		if q.ID == "requeue-1" {
			found = q
		}
	}
	s2.mu.Unlock()
	if found == nil {
		t.Fatalf("RequeueTask 后盘上应恢复出排队件 requeue-1（T-10: 补 saveTasksLocked 后才成立）")
	}
	if found.Status != StatusQueued {
		t.Errorf("恢复出的件应为 queued，实际 %q", found.Status)
	}
}

// ============ GAP-20261022-01/02: history 分区里的「可恢复态孤儿件」============
//
// 病灶: tasks_persist.go::parkUnrecovered 把「属可恢复集却落在非对应分区」的件保守留进
// history（不派发、不跨分区重路由），而 loadPersistedHistory 只从 queue 回队 ⇒ 这些件
// 永不派发、投影恒报 queued。撤销面原先只从 queue 认 queued/paused ⇒ 它们落进 switch 的
// default，报出「status queued cannot be cancelled (only queued/paused tasks can be cancelled)」
// ——文案与状态自相矛盾，且件永远撤不掉（实测 task-1790355446454773000 /
// task-1790355447190844000，zerg task cancel rc=2、task show 仍 queued）。
//
// 两条判据（成对，禁放宽）:
//  ① 「取消后不在队」: 撤完 queue/running/waiting 三处都不得有该 id，且**跨实例读回**
//     （重启恢复路径）仍是 cancelled、仍不在 queue —— 否则重启后被回队 = 复活重新派发。
//  ② 「报话与状态一致」: 对 history 里的 queued/paused **不得拒绝**；对真该拒的状态，
//     文案不得再出现「status queued/paused … only queued/paused」这种自打脸的组合。

// TestTaskExitCancelHistoryOrphanQueued — GAP-20261022-01: history 里 status=queued 的
// 孤儿件（parkUnrecovered 造）必须能撤，且撤后不在任何可派发分区、重启也不回队。
func TestTaskExitCancelHistoryOrphanQueued(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	orphan := &Task{
		ID:          "orphan-queued",
		Description: "属可恢复集却落在 history 的孤儿件（GAP-20261022-01）",
		Priority:    PriorityInternal,
		Status:      StatusQueued,
		FailReason:  "model/backend unavailable (attempt 2 — waiting for environment recovery to auto-retry)",
	}
	s.mu.Lock()
	s.history["orphan-queued"] = orphan // 只在 history —— 与真机现场同形
	s.mu.Unlock()

	outcome, err := s.CancelTask("orphan-queued", "obsolete", "GAP-20261022-01")
	if err != nil {
		t.Fatalf("history 里的 queued 孤儿件必须可撤（修前报「status queued cannot be cancelled (only queued/paused tasks can be cancelled)」）: err=%v", err)
	}
	if outcome != CancelOutcomeCancelled {
		t.Errorf("outcome 应为 %q，实际 %q（状态迁移成功，不是 already_terminal）", CancelOutcomeCancelled, outcome)
	}

	s.mu.Lock()
	_, inQueue := taskExitQueueIndex(s.queue, "orphan-queued")
	_, inRunning := s.running["orphan-queued"]
	_, inWaiting := s.waiting["orphan-queued"]
	h := s.history["orphan-queued"]
	s.mu.Unlock()

	if inQueue || inRunning || inWaiting {
		t.Fatalf("★ 取消后不得在队: inQueue=%v inRunning=%v inWaiting=%v", inQueue, inRunning, inWaiting)
	}
	if h == nil || h.Status != StatusCancelled {
		t.Fatalf("history 里的件应就地结案为 cancelled，实际 %+v", h)
	}
	if h.ClosedAt == nil || h.ClosedReason != "obsolete" || h.ClosedNote != "GAP-20261022-01" || h.ClosedBy != closedByLocal {
		t.Errorf("应写齐 closed_* 四条: at=%v by=%q reason=%q note=%q", h.ClosedAt, h.ClosedBy, h.ClosedReason, h.ClosedNote)
	}

	// 跨实例读回（= 主控重启的恢复路径）: still cancelled、仍不在 queue。
	s2 := NewMasterScheduler("/bin/echo", 1)
	s2.mu.Lock()
	got := s2.history["orphan-queued"]
	_, requeued := taskExitQueueIndex(s2.queue, "orphan-queued")
	s2.mu.Unlock()
	if got == nil || got.Status != StatusCancelled {
		t.Fatalf("重启读回后应为 history 里的 cancelled，实际 %+v", got)
	}
	if requeued {
		t.Fatalf("★ 取消后重启不得回队（回队 = 孤儿复活并重新派发）")
	}
}

// TestTaskExitCancelHistoryOrphanPaused — GAP-20261022-02 同族: history 里的 paused 孤儿件。
func TestTaskExitCancelHistoryOrphanPaused(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	p := &Task{ID: "orphan-paused", Description: "history 里的 paused 孤儿件", Priority: PriorityInternal, Status: StatusPaused}
	s.mu.Lock()
	s.history["orphan-paused"] = p
	s.mu.Unlock()

	if _, err := s.CancelTask("orphan-paused", "obsolete", "GAP-20261022-02"); err != nil {
		t.Fatalf("history 里的 paused 件必须可撤: err=%v", err)
	}
	s.mu.Lock()
	_, inQueue := taskExitQueueIndex(s.queue, "orphan-paused")
	st := p.Status
	s.mu.Unlock()
	if inQueue {
		t.Errorf("取消后不得在队")
	}
	if st != StatusCancelled {
		t.Errorf("status 应为 cancelled，实际 %q", st)
	}
}

// TestTaskExitCancelRefusalMessageNeverContradictsStatus — 「报话与状态一致」判据。
//
// 两腿:
//   - 可撤态（queued/paused，在 history 里）⇒ **不得拒绝**；
//   - 真该拒的态（waiting_retry / 未知态）⇒ 可以拒，但文案**不得**出现
//     「status queued」或「status paused」（那正是修前的自打脸组合），且必须点名真实状态。
//
// ★ 负控: 把修前代码放回来跑本用例 ⇒ 第一腿必红（err != nil，且文案含 "status queued"）。
func TestTaskExitCancelRefusalMessageNeverContradictsStatus(t *testing.T) {
	for _, st := range []string{StatusQueued, StatusPaused} {
		s := newTaskExitTestScheduler(t, 1)
		tk := &Task{ID: "msg-" + st, Status: st, Priority: PriorityInternal}
		s.mu.Lock()
		s.history[tk.ID] = tk
		s.mu.Unlock()
		if _, err := s.CancelTask(tk.ID, "obsolete", "报话一致性"); err != nil {
			t.Fatalf("可撤态 %q 落在 history 时不得拒绝（修前会回「status %s cannot be cancelled (only queued/paused tasks can be cancelled)」= 自相矛盾）: %v", st, st, err)
		}
	}

	// 拒收腿: 这些状态确实不该被撤销，但文案必须诚实（点名真实状态、不宣称 queued/paused 不可撤）。
	for _, st := range []string{"archived", "reviewing"} {
		s := newTaskExitTestScheduler(t, 1)
		tk := &Task{ID: "msg-" + st, Status: st, Priority: PriorityInternal}
		s.mu.Lock()
		s.history[tk.ID] = tk
		s.mu.Unlock()
		_, err := s.CancelTask(tk.ID, "obsolete", "报话一致性")
		if err == nil {
			t.Fatalf("拒收态 %q 应拒绝，实际 err=nil", st)
		}
		msg := err.Error()
		if strings.Contains(msg, "status "+StatusQueued) || strings.Contains(msg, "status "+StatusPaused) {
			t.Errorf("拒收文案自相矛盾（实际状态是 %q，文案却说 queued/paused 不可撤）: %s", st, msg)
		}
		if !strings.Contains(msg, st) {
			t.Errorf("拒收文案必须点名真实状态 %q: %s", st, msg)
		}
		if tk.Status != st {
			t.Errorf("拒收后 status 不得变，实际 %q", tk.Status)
		}
	}
}
