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

// TestTaskExitCancelWaitingRetryRejected — T-05 守卫: waiting_retry 拒绝（等机器恢复）。
func TestTaskExitCancelWaitingRetryRejected(t *testing.T) {
	s := newTaskExitTestScheduler(t, 1)
	w := &Task{ID: "cancel-wait", Status: StatusWaitingRetry, Priority: PriorityExternal}
	s.mu.Lock()
	s.waiting["cancel-wait"] = w
	s.mu.Unlock()

	if _, err := s.CancelTask("cancel-wait", "abandoned", ""); err == nil {
		t.Fatalf("对 waiting_retry 应拒绝，实际 err=nil")
	}
	s.mu.Lock()
	st := w.Status
	s.mu.Unlock()
	if st != StatusWaitingRetry {
		t.Errorf("拒绝后 waiting_retry 件 status 不应变，实际 %q", st)
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
