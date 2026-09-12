package api

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// isolateTaskRoot 测试隔离: 把任务目录根切到 t.TempDir()。
// 任务目录根（默认 /tmp/zerg-tasks，产品侧经 statepath.TaskRoot() 读 ZERG_TASK_ROOT 可覆盖）——
// 测试若用固定 ID 建任务（如 t1 / task-flow-test-1 / task-review-test-1），任务目录会落到真机共享根，
// 与真机在跑的任务互相污染、遗留垃圾、排查时难分真假（待修补 #36——#35 同族第三处）。
// t.Setenv 在用例结束后自动严格恢复原值（不依赖手写 defer）。
func isolateTaskRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_TASK_ROOT", dir)
	return dir
}

// isolateTasksFile 测试隔离: 把包级 tasksFile 切到 t.TempDir() 下的临时文件。
// tasksFile 是主控调度器的持久化文件（默认指向真机 /tmp/zerg-tasks.json）——
// 测试若不解耦，构造 MasterScheduler（loadPersistedHistory 读）或任何落盘路径
// （saveTasksLocked 写）都会读写真机主控在用的同一文件，与真机任务互相污染（待修补 #35）。
// 返回临时路径；t.Cleanup 在用例结束后严格恢复原值（不依赖手写 defer 防漏）。
func isolateTasksFile(t *testing.T) string {
	t.Helper()
	old := tasksFile
	p := filepath.Join(t.TempDir(), "zerg-tasks-test.json")
	tasksFile = p
	t.Cleanup(func() { tasksFile = old })
	return p
}

// ============ 测试时序稳健性辅助（测试专用——绝不改产品代码） ============
//
// 背景: 旧用例在固定 time.Sleep(100ms) 后立即断言「队空 + running 空」。
// 本机整链（1 次执行 + 3 次验证不过重跑 = 4 轮，Mr2109的设计不得删）实测约 37ms——100ms 够；
// 但慢机/CI 约 130ms——100ms 时第 4 轮还在跑——断言断的是「机器快慢」不是逻辑——
// 同一提交在 CI 上时红时绿。
// 改为「上限内轮询到静止」: 达到即通过；超时失败并打印当时真实状态（queue/running/任务态）供 CI 诊断。
// 断言口径一字不改（仍要求队空 + running 空），只是不再假设固定时长够快——不 skip、不放宽、不加 sleep 了事。

const (
	// schedIdleLimit 轮询上限（慢机/忙机 CI 也有充足余量——本机整链约 37ms，正常路径几乎不等待）
	schedIdleLimit = 5 * time.Second
	// schedIdlePoll 轮询间隔（小——正常路径只轮询十余次）
	schedIdlePoll = 5 * time.Millisecond
)

// schedSnapshot 调度器状态快照（轮询判定 + 超时诊断用——全部持锁读——-race 干净）
type schedSnapshot struct {
	queueLen   int
	runningLen int
	runningIDs []string
	taskStatus string
}

// read 持锁取快照（与调度器 goroutine 的字段写入互斥）
func (sn *schedSnapshot) read(s *MasterScheduler, task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn.queueLen = len(s.queue)
	sn.runningLen = len(s.running)
	sn.runningIDs = sn.runningIDs[:0]
	for id := range s.running {
		sn.runningIDs = append(sn.runningIDs, id)
	}
	sort.Strings(sn.runningIDs) // 稳定输出（诊断/日志可比对）
	if task != nil {
		sn.taskStatus = task.Status
	}
}

// idle 静止判定: 队空 + running 空（与旧断言同口径）
func (sn schedSnapshot) idle() bool { return sn.queueLen == 0 && sn.runningLen == 0 }

func (sn schedSnapshot) String() string {
	status := sn.taskStatus
	if status == "" {
		status = "<unknown>"
	}
	return fmt.Sprintf("queue=%d running=%d %v task_status=%q",
		sn.queueLen, sn.runningLen, sn.runningIDs, status)
}

// isTerminalTaskStatus 任务终态（done/failed）——确认任务真跑完而非停在中间态
func isTerminalTaskStatus(status string) bool {
	return status == "done" || status == "failed"
}

// waitSchedulerIdle 上限内轮询到静止: 队空 + running 空 +（给了 task 时）任务已达终态。
// 达到 → 返回最终快照 true；超时 → 返回最后一次快照 false（调用方打印——CI 上可诊断根因）。
// 返回轮询次数便于报告「正常路径几乎不等待」。
func waitSchedulerIdle(s *MasterScheduler, task *Task) (sn schedSnapshot, ok bool, polls int) {
	deadline := time.Now().Add(schedIdleLimit)
	for {
		polls++
		sn.read(s, task)
		if sn.idle() && (task == nil || isTerminalTaskStatus(sn.taskStatus)) {
			return sn, true, polls
		}
		if !time.Now().Before(deadline) {
			return sn, false, polls
		}
		time.Sleep(schedIdlePoll)
	}
}

// TestMasterScheduler_Priority — 外部任务优先（高优先级先出队）
func TestMasterScheduler_Priority(t *testing.T) {
	q := TaskQueue{}
	q = append(q, &Task{ID: "internal-1", Priority: PriorityInternal})
	q = append(q, &Task{ID: "external-1", Priority: PriorityExternal})
	q = append(q, &Task{ID: "internal-2", Priority: PriorityInternal})

	// 手动用 heap 验证（不启动 goroutine——只看队列顺序）
	// 这里直接用 heap 逻辑验证 Less
	if !q.Less(1, 0) {
		t.Fatal("外部任务应优先于内部任务（PriorityExternal > PriorityInternal）")
	}
	t.Log("✅ 外部任务优先级高于内部任务")
}

// TestMasterScheduler_SubmitAndQueue — 提交任务（echo 快速完成——验证状态流转不崩）
func TestMasterScheduler_SubmitAndQueue(t *testing.T) {
	// v2.5.6 2026-08-28 测试隔离: 主控在跑会写 /tmp/zerg-tasks.json——测试切到临时文件（防恢复真实任务干扰）
	// 待修补 #35: 统一走 isolateTasksFile（t.Cleanup 严格恢复）——不用手写 defer
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("/bin/echo", 2) // 用 echo（不真跑 CA——测试流程）
	task := &Task{ID: "t1", Description: "test", Priority: PriorityInternal}
	s.Submit(task)
	// 时序稳健（2026-09-13 修）: 原为固定 time.Sleep(100ms) + 立即断言——
	// 慢机/CI 上整链（4 轮）>100ms 时第 4 轮仍在跑 → 误判失败（断时序非逻辑）。
	// 改为上限内轮询到静止: 队空 + running 空 + 任务达终态；超时打印当时真实状态。
	sn, ok, polls := waitSchedulerIdle(s, task)
	if !ok {
		t.Fatalf("scheduler did not become idle within %s (polled %d time(s) every %s) — actual state: %s",
			schedIdleLimit, polls, schedIdlePoll, sn)
	}
	t.Logf("scheduler idle after %d poll(s) every %s (limit %s): %s", polls, schedIdlePoll, schedIdleLimit, sn)
	if s.QueueLen() != 0 {
		t.Fatalf("echo 任务应快速完成——队列应空（实际 %d）", s.QueueLen())
	}
	if len(s.running) != 0 {
		t.Fatal("任务完成后 running 应清空")
	}
	t.Log("✅ 任务提交→派发→完成流程跑通（状态流转正常）")
}

// TestMasterScheduler_ConcurrentLimit — 并发限制（maxConcurrent）
func TestMasterScheduler_ConcurrentLimit(t *testing.T) {
	// 测试隔离（同 SubmitAndQueue）: 持久化文件切临时目录——不写/不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("/bin/echo", 1) // 单槽
	task := &Task{ID: "t1", Description: "test1", Priority: PriorityInternal}
	s.Submit(task)
	// 时序稳健（2026-09-13 修）: 原为固定 time.Sleep(100ms) + 立即断言（旧断言 QueueLen() < 0 恒真，
	// 不构成断言）——现改为上限内轮询到静止后断言单槽任务确实跑完。慢机/CI 不再误判。
	sn, ok, polls := waitSchedulerIdle(s, task)
	if !ok {
		t.Fatalf("scheduler did not become idle within %s (polled %d time(s) every %s) — actual state: %s",
			schedIdleLimit, polls, schedIdlePoll, sn)
	}
	t.Logf("scheduler idle after %d poll(s) every %s (limit %s): %s", polls, schedIdlePoll, schedIdleLimit, sn)
	// 单槽串行: 任务跑完（含重跑）后队空 + running 空
	if s.QueueLen() != 0 {
		t.Fatalf("单槽任务应跑完——队列应空（实际 %d）", s.QueueLen())
	}
	if len(s.running) != 0 {
		t.Fatal("任务完成后 running 应清空")
	}
	t.Log("✅ 并发限制逻辑跑通（单槽——任务串行）")
}

// TestMasterScheduler_Pause — 外部任务提交——内部任务挂起重新入队（外部优先执行）
func TestMasterScheduler_Pause(t *testing.T) {
	// 待修补 #35: 构造 MasterScheduler 会 loadPersistedHistory 读 tasksFile——
	// 切临时文件，防真机 /tmp/zerg-tasks.json 的真实任务被恢复入队干扰本用例
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("/bin/echo", 2)
	// 2026-09-05 修: 清全局持久化残留（其他测试/真实运行写 /tmp/zerg-tasks.json——
	// NewMasterScheduler 恢复旧任务入队致 QueueLen 断言污染）——再清一次本测试入队的
	defer func() {
		s.mu.Lock()
		s.queue = s.queue[:0]
		s.mu.Unlock()
	}()
	s.mu.Lock()
	s.queue = s.queue[:0] // 清恢复入队的旧任务——只测 pause 逻辑
	s.mu.Unlock()
	// 直接构造 running 状态（不真跑——验证 pauseInternalsLocked 逻辑）
	internal := &Task{ID: "internal-1", Description: "进化", Priority: PriorityInternal, Status: "running"}
	s.running[internal.ID] = internal

	s.mu.Lock()
	s.pauseInternalsLocked() // 模拟外部提交触发
	s.mu.Unlock()

	if internal.Status != "queued" {
		t.Fatalf("内部任务应重新入队（queued）——实际 %s", internal.Status)
	}
	if _, ok := s.running[internal.ID]; ok {
		t.Fatal("内部任务应从 running 移除（挂起）")
	}
	if s.QueueLen() != 1 {
		t.Fatalf("内部任务应在队列（等待恢复）——实际 %d", s.QueueLen())
	}
	t.Log("✅ 外部任务提交——内部任务挂起重新入队（外部完恢复续接）")
}

// TestSanitizeID — 任务 ID 清洗（git 分支名合法）
func TestSanitizeID(t *testing.T) {
	if got := sanitizeID("task-1787033215947143000"); got != "task-1787033215947143000" {
		t.Fatalf("正常 ID 应保留——实际 %s", got)
	}
	if got := sanitizeID("task/foo:bar"); got != "task-foo-bar" {
		t.Fatalf("非法字符应替换——实际 %s", got)
	}
	t.Log("✅ 任务 ID 清洗正常（git 分支名合法）")
}
