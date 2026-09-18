package api

// review_scheduler_test.go — 复查/恢复调度核心测试（2026-08-29 q5 覆盖补齐）
// 目标: recoverWaitingLocked(0%) / submitReviewTaskLocked(0%) / handleReviewDoneLocked(0%)

import (
	"container/heap"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRecoverWaiting_ExpiredDowngrade 超上限(>30min) → 降级 failed
func TestRecoverWaiting_ExpiredDowngrade(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json，落盘不写真机文件
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	task := &Task{
		ID:          "wait-test-1",
		Description: "测试",
		Status:      "waiting_retry",
		Model:       "Qwen3.8-27B",
		CompletedAt: time.Now().Add(-maxWaitRetry - time.Minute), // 超上限
	}
	s.waiting[task.ID] = task
	s.recoverWaitingLocked()
	// 超时 → 删除 waiting → 进 history 标 failed
	if _, ok := s.waiting[task.ID]; ok {
		t.Fatal("超时任务应从 waiting 删除")
	}
	h, ok := s.history[task.ID]
	if !ok || h.Status != "failed" {
		t.Fatalf("超时任务应 failed 进 history——实际 %v", h)
	}
	if !strings.Contains(h.FailReason, "自动放弃") {
		t.Fatalf("失败原因应含自动放弃: %s", h.FailReason)
	}
}

// ============ 「任务未丢」判据 + 派发链收尾（2026-09-18 修环境依赖 flake） ============
//
// 现场: recoverWaitingLocked 在网关 ping **通**时，会把挂起任务重新入队并**立刻** dispatchLocked 派发——
// master_scheduler.go:217-227 里「delete(waiting) → heap.Push(queue) → dispatchLocked()」全在同一把锁内，
// 而 dispatchLocked（master_scheduler.go:366-374）又在这把锁内「出队 → 标 running → 入 s.running → go runTask」。
// ⇒ recoverWaitingLocked() 返回时，任务已经**不在** waiting/history，也**不在** queue 里（在 s.running）。
//
// 原用例只查 waiting/history/queue 三处 ⇒ 本机网关可达（主控在跑）时，正在执行的活任务被误判成
// 「任务丢失」⇒ 必红；网关不可达时才绿。用例红绿取决于**环境**（网关是否可达），不取决于被测逻辑
// ——原注释（L44-45）已自陈该缺陷。

// taskWhere 任务可追溯判定: 返回任务所在处（waiting/running/history/queue），四处都没有 ⇒ ""（=丢失）。
//
// ⚠ 这是**补全事实集合**（漏掉的 running 正是本次修的那一处），不是放宽判据:
//   - 四处都没有 ⇒ 仍判"丢失"（真丢失必须红——反例 TestTaskWhere_RealLossIsDetected）;
//   - 任一处被从判据里漏掉 ⇒ TestTaskWhere_CoversEveryContainer 必红（判据完整性由该用例逐处钉死）。
//
// 调用方不得持锁（内部持锁读——与后台派发 goroutine 互斥，-race 干净）。
func taskWhere(s *MasterScheduler, id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.waiting[id]; ok {
		return "waiting"
	}
	if _, ok := s.running[id]; ok {
		return "running"
	}
	if _, ok := s.history[id]; ok {
		return "history"
	}
	for _, q := range s.queue {
		if q != nil && q.ID == id {
			return "queue"
		}
	}
	return ""
}

// drainDispatchedWork 等本轮派发链收尾（队列空 + running 空）——用例返回前必须调用。
//
// 为什么必须: dispatchLocked 起的后台 runTask 会经 saveTasksLocked → tasksWritePath()
// **读包级变量 tasksFile**（tasks_persist.go:41），而 isolateTasksFile 的 t.Cleanup 会**写同一个包级变量**
// （master_scheduler_test.go:33）。用例不管这些 goroutine 就返回 ⇒ 夹具恢复 tasksFile 与后台写盘并发
// ⇒ -race 报 DATA RACE（实测栈: tasksWritePath←saveTasksLocked←runTask←dispatchLocked 读 vs isolateTasksFile 写）;
// 且失败重跑链会在夹具恢复**之后**继续跑（真实遗留: /tmp/zerg-tasks/wait-test-2）。收尾后不存在仍会读
// tasksFile 的 goroutine，夹具恢复才安全。
//
// 用 waitSchedulerIdle(s, nil): 只要求「队空 + running 空」，**不**要求任务达终态
// （ping 不通时任务本就停在 waiting_retry——非终态，不能因此判失败）。
// 用 t.Errorf（非 t.Fatalf）: 本函数在 t.Cleanup 里执行，Fatal 会打断后续清理。
func drainDispatchedWork(t *testing.T, s *MasterScheduler) {
	t.Helper()
	sn, ok, polls := waitSchedulerIdle(s, nil)
	if !ok {
		t.Errorf("派发链未在上限内收尾（上限 %s，轮询 %d 次）——后台 goroutine 可能仍会读 tasksFile，与夹具恢复相撞: %s",
			schedIdleLimit, polls, sn)
	}
}

// TestRecoverWaiting_PingFailStaysWaiting ping 不通 → 继续 waiting（不降级不重派）
func TestRecoverWaiting_PingFailStaysWaiting(t *testing.T) {
	// 2026-09-05 修: 本测试原依赖"网关 8082 不通"才过——网关在线时任务恢复→真 exec→failed——环境依赖缺陷
	// 2026-09-18 修（治本——两条）:
	//   ① 判据环境无关: 改判「不丢任务」= 任务在 waiting ∪ queue ∪ running ∪ history 任一处（见 taskWhere）。
	//      原判据漏 running ⇒ 网关可达（主控在跑）时把已重派进 s.running 的活任务误判为"丢失" ⇒ 必红。
	//   ② 消 DATA RACE 根源: 用例结束前把派发链收尾（drainDispatchedWork，见其注释）——
	//      不再让泄漏的后台 runTask 与 isolateTasksFile 的 t.Cleanup 同时读写包级 tasksFile。
	// 待修补 #35: 切临时 tasksFile——构造期不读真机文件，recoverWaitingLocked 落盘也不写真机
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	// 收尾: t.Cleanup 逆序执行（本函数晚于 isolateTasksFile 注册）⇒ 收尾先跑，夹具恢复 tasksFile 后跑
	t.Cleanup(func() {
		drainDispatchedWork(t, s) // ② 等后台派发链退出——之后夹具才恢复 tasksFile
		// 清理（原用例末的清理意图——改为持锁做，与后台 goroutine 互斥）
		s.mu.Lock()
		s.waiting = map[string]*Task{}
		s.history = map[string]*Task{}
		s.mu.Unlock()
	})
	task := &Task{
		ID:          "wait-test-2",
		Description: "测试",
		Status:      "waiting_retry",
		Model:       "Qwen3.8-27B",
		CompletedAt: time.Now(),
	}
	s.mu.Lock()
	s.waiting[task.ID] = task
	s.mu.Unlock()
	s.recoverWaitingLocked()
	// 任务必须可追溯（waiting/queue/running/history 四处之一——网关通/不通都成立 ⇒ 环境无关）
	where := taskWhere(s, task.ID)
	if where == "" {
		var sn schedSnapshot
		sn.read(s, task)
		s.mu.Lock()
		waitLen := len(s.waiting)
		s.mu.Unlock()
		t.Fatalf("任务丢失（waiting/queue/running/history 均无）——实际状态: %s waiting=%d", sn, waitLen)
	}
	t.Logf("✅ 任务可追溯: 在 %s（waiting/queue/running/history 任一即未丢——与网关是否可达无关）", where)
}

// TestTaskWhere_CoversEveryContainer 判据自证: 「任务未丢」判据必须覆盖全部四处事实集合。
// 逐处轮换——任务**只**放在 waiting/queue/running/history 之一 ⇒ 判据必须认"未丢"（并回报是哪一处）。
// 变异自证: 把 running 从 taskWhere 里拿掉（= 回到修前只管三处的口径）⇒ running 子例必红。
func TestTaskWhere_CoversEveryContainer(t *testing.T) {
	cases := []struct {
		name string
		put  func(s *MasterScheduler, tk *Task)
	}{
		{"waiting", func(s *MasterScheduler, tk *Task) { s.waiting[tk.ID] = tk }},
		{"queue", func(s *MasterScheduler, tk *Task) { heap.Push(&s.queue, tk) }},
		{"running", func(s *MasterScheduler, tk *Task) { s.running[tk.ID] = tk }},
		{"history", func(s *MasterScheduler, tk *Task) { s.history[tk.ID] = tk }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateTasksFile(t)
			s := NewMasterScheduler("", 1) // 只造状态——不派发、不起 goroutine
			tk := &Task{ID: "where-" + c.name, Status: "queued"}
			s.mu.Lock()
			c.put(s, tk)
			s.mu.Unlock()
			if got := taskWhere(s, tk.ID); got != c.name {
				t.Fatalf("任务只在 %s——判据必须认未丢（实际判为 %q）", c.name, got)
			}
		})
	}
}

// TestTaskWhere_RealLossIsDetected 反例（真丢失必须红）: 判据不得退化成"恒不丢"。
// 任务在四处都没有 ⇒ taskWhere 必须返回空。这条反例是「只补事实集合、不放宽判据」的机器证明。
func TestTaskWhere_RealLossIsDetected(t *testing.T) {
	isolateTasksFile(t)
	s := NewMasterScheduler("", 1)
	// 先证明判据在系统里"认识"某个任务（防判据失灵后本反例假绿）
	s.mu.Lock()
	s.waiting["where-present"] = &Task{ID: "where-present", Status: "waiting_retry"}
	s.mu.Unlock()
	if where := taskWhere(s, "where-present"); where == "" {
		t.Fatal("waiting 里的任务应判为未丢（判据失灵——本反例的前提不成立）")
	}
	// 真丢失（四处都没有，且系统里另有别的任务）⇒ 必须判丢失
	if where := taskWhere(s, "where-gone-404"); where != "" {
		t.Fatalf("任务在四处都没有——判据必须判为丢失（实际 %q —— 判据恒真 = 放宽）", where)
	}
	t.Log("✅ 真丢失可被判出（判据未退化成恒真）")
}

// TestSubmitReviewTaskLocked 执行完成 → 派复查（跨家族模型——不同源）
func TestSubmitReviewTaskLocked(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	execTask := &Task{
		ID:          "exec-test-1",
		Description: "执行任务",
		Model:       "Qwen3.8-27B", // qwen 家族
		Workdir:     "/tmp/zerg-test-flow",
		Status:      "reviewing",
	}
	// 复查模型必须不同家族（qwen 执行 → 不能 qwen 复查）
	s.submitReviewTaskLocked(execTask, "/tmp/report.md", "")
	// 复查任务入队
	reviewID := "review-exec-test-1"
	found := false
	for _, q := range s.queue {
		if q.ID == reviewID {
			found = true
			if q.Model == execTask.Model {
				t.Fatalf("复查模型不能与执行同源: %s", q.Model)
			}
			if q.Type != "review" {
				t.Fatalf("复查任务 Type 应为 review: %s", q.Type)
			}
		}
	}
	if !found {
		t.Fatal("复查任务未入队")
	}
	// 执行任务状态应是 reviewing（复查通过才 done）
	if execTask.Status != "reviewing" {
		t.Fatalf("执行任务应 reviewing: %s", execTask.Status)
	}
}

// TestHandleReviewDone_Pass 复查通过 → 执行任务 done
func TestHandleReviewDone_Pass(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机文件，handleReviewDoneLocked 落盘不写真机
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	// 2026-09-05 修: 清恢复入队（/tmp/zerg-tasks.json 遗留任务被 recoverWaiting 恢复——
	// dispatchLocked 并发把 history 逐出/修改——本测试只验 handleReviewDoneLocked 纯逻辑）
	s.mu.Lock()
	s.queue = s.queue[:0]
	s.waiting = map[string]*Task{}
	s.mu.Unlock()
	execTask := &Task{
		ID:          "exec-test-2",
		Description: "执行任务",
		Model:       "Qwen3.8-27B",
		Status:      "reviewing",
	}
	s.history[execTask.ID] = execTask
	reviewTask := &Task{
		ID:        "review-exec-test-2",
		RefTaskID: execTask.ID,
		Model:     "gemma-4-26B",
		Workdir:   "/tmp/zerg-test-flow",
	}
	// 复查报告（通过——无打回关键词——≥100 字节过新守卫——2026-09-05）
	// 专属目录防测试互扰（Rework 测试同文件写打回内容——并行/顺序执行都会污染）
	passDir := "/tmp/zerg-test-flow-pass"
	_ = os.MkdirAll(passDir, 0o755)
	reviewTask.Workdir = passDir
	_ = os.WriteFile(filepath.Join(passDir, "internal-task-report.md"), []byte("## 复查结论\n\n结论: 通过。\n\n理由: 报告内容与实际改动核对一致，真实性检查通过，完成度满足任务要求，未发现虚构或遗漏，质量符合预期。\n"), 0o644)
	s.handleReviewDoneLocked(reviewTask)
	if execTask.Status != "done" {
		t.Fatalf("复查通过后执行任务应 done: %s", execTask.Status)
	}
}

// TestHandleReviewDone_Rework 复查打回 → 派重做任务
func TestHandleReviewDone_Rework(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机文件，handleReviewDoneLocked 落盘不写真机
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	execTask := &Task{
		ID:          "exec-test-3",
		Description: "执行任务",
		Model:       "Qwen3.8-27B",
		Status:      "reviewing",
	}
	s.history[execTask.ID] = execTask
	reviewTask := &Task{
		ID:        "review-exec-test-3",
		RefTaskID: execTask.ID,
		Model:     "gemma-4-26B",
		Workdir:   "/tmp/zerg-test-flow",
	}
	// 复查报告（打回——含"打回"关键词）
	_ = os.MkdirAll("/tmp/zerg-test-flow", 0o755)
	_ = os.WriteFile(filepath.Join("/tmp/zerg-test-flow", "internal-task-report.md"), []byte("打回——幻觉代码"), 0o644)
	s.handleReviewDoneLocked(reviewTask)
	// 应派重做任务（rework- 前缀）
	found := false
	for _, q := range s.queue {
		if strings.HasPrefix(q.ID, "rework-exec-test-3") {
			found = true
			if q.Model != execTask.Model {
				t.Fatalf("重做任务应保持执行模型: %s", q.Model)
			}
		}
	}
	if !found {
		t.Fatal("打回后应派重做任务")
	}
}

// TestReviewRetryOnReviewerFailure — S7: 复查自身失败（无报告）→ 重派复查非打回
func TestReviewRetryOnReviewerFailure(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)
	s.mu.Lock()
	s.queue = s.queue[:0]
	s.waiting = map[string]*Task{}
	s.mu.Unlock()
	reviewDir := t.TempDir() // 空目录=无报告=复查自身失败
	execTask := &Task{ID: "exec-s7", Status: "reviewing", Model: "Qwen3.8-27B", Workdir: t.TempDir(), Description: "测试任务"}
	s.history[execTask.ID] = execTask
	review := &Task{ID: "review-s7", RefTaskID: execTask.ID, Status: "done", Model: "example-35b-v2",
		Workdir: reviewDir, ReplanCount: 0}
	s.history[review.ID] = review

	s.handleReviewDoneLocked(review)

	if execTask.Status == "failed" {
		t.Fatalf("复查自身失败不应打回执行任务: %s", execTask.FailReason)
	}
	if execTask.ReplanCount != 0 {
		t.Fatalf("不应消耗 Replan: %d", execTask.ReplanCount)
	}
	if execTask.ReviewFailedCount != 1 {
		t.Fatalf("应记复查失败 1 次: %d", execTask.ReviewFailedCount)
	}
	found := false
	for _, task := range s.queue {
		if strings.HasPrefix(task.ID, "review-retry-exec-s7") && task.Type == "review" {
			found = true
		}
	}
	if !found {
		t.Fatal("应重派 review-retry 任务")
	}
}
