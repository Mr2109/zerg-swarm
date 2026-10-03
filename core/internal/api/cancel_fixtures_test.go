package api

// cancel_fixtures_test.go — 任务退场正门 · 刀 5 夹具 F-1…F-5
// 2026-10-03 · GAP-20261003-04 · 取代设计 v0.2 §4.3「四夹具」（依 §11.6 六条取 F-1…F-5）。
//
// 覆盖（逐条对齐任务清单 刀 5）:
//   F-1 原子性/断电仿真 ......... TestCancelFxF1AtomicNoTornWrite
//   F-2 提交点顺序 .............. 已由 master_scheduler_cancel_test.go 的
//                                 TestTaskExitCancelSaveFailureSkipsDispatch 覆盖
//                                 （mock save 返 error ⇒ dispatch 决策函数未被进入 + 明确失败），
//                                 本件**不重写**（路 1 已有同类用例）。
//   F-3 队列卡死回归 ............ TestCancelFxF3CancelRestartNoDuplicate
//   F-4 落盘时机回归 ............ TestCancelFxF4ExactlyOneSavePerTransition
//   F-5 W3 窗口 ................. TestCancelFxF5CrashBeforeDispatchKeepsFinalOnDisk
//                                 （子进程真 SIGKILL：TestCancelFxF5CrashChild）
//   F-6 幂等重放 ................ 属刀 2（CLI 面），本单不做；幂等的「不再落盘」侧面在 F-4 已顺带咬住。
//
// 只跑本件（前缀 ^TestCancelFx 即本件全集，禁截断）:
//   cd core && go test -mod=mod ./internal/api/ -run '^TestCancelFx' -count=1
//
// 隔离纪律: 复用包内既有隔离体例 —— setTasksPaths（切包级 tasksFile/legacyTasksFile，
// t.Cleanup 严格恢复）+ isolateTaskRoot（切 ZERG_TASK_ROOT）；全部落 t.TempDir()，
// 绝不碰真机 ~/.zerg/state 或 /tmp/zerg-tasks.json。
//
// 真咬行为（每条夹具的牙由 go test -overlay 变异负控证明，见交付回执）:
//   F-1 负控 = 把 saveTasksLocked 的 temp+rename 换成 os.WriteFile 直写（O_TRUNC）⇒ 读者抓到 0 字节/半文件 ⇒ 红；
//   F-3 负控 = ① CancelTask 忘落盘（O-7 血证）⇒ 重启后被取消件复活进 queue ⇒ 红；
//              ② 只标态不摘容器（去掉出队）⇒ 盘上件同时在 queue 与 history（重复）⇒ 红；
//   F-4 负控 = ① 去掉 save 调用 ⇒ 计数 0 ⇒ 红；② 双调 save ⇒ 计数 2 ⇒ 红；
//   F-5 负控 = 把 save 挪到 dispatch 之后 ⇒ SIGKILL 时盘上仍是被取消件在 queue ⇒ 红。

import (
	"bytes"
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ============ 夹具公共体例（fx 前缀，避免与包内既有测试件撞名）============

// fxIsolatedPaths 把包级持久化路径切到 t.TempDir()（复用 setTasksPaths 体例），返回（目标件路径，目录）。
func fxIsolatedPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "zerg-tasks.json")
	setTasksPaths(t, dst, filepath.Join(dir, "legacy-never-used.json"))
	return dst, dir
}

// fxNewScheduler 构造一个不 Start、不派发后台循环的裸调度器（构造期只从盘恢复状态）。
func fxNewScheduler(t *testing.T, maxConc int) *MasterScheduler {
	t.Helper()
	isolateTaskRoot(t)
	return NewMasterScheduler("/bin/echo", maxConc)
}

// fxQueueIDs 快照队列 ID（单线程读——用例内自持锁）。
func fxQueueIDs(q TaskQueue) []string {
	ids := make([]string, 0, len(q))
	for _, t := range q {
		if t != nil {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// fxContainsStr 字符串切片是否含某值（零命中即假）。
func fxContainsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// fxIndexOf 在优先级队列里按 ID 找下标。
func fxIndexOf(q TaskQueue, id string) (int, bool) {
	for i, t := range q {
		if t.ID == id {
			return i, true
		}
	}
	return -1, false
}

// fxSignature 盘上状态的**内容签名**（忽略可变 SavedAt）: q=<排序ID>、r=<排序runningID>、h=<排序historyID>。
// 用于 F-1 的「读到的一定是某一个完整状态」判定（半文件 ⇒ JSON 解析失败，先于签名比对即红）。
func fxSignature(queue []*Task, running, hist map[string]*Task) string {
	qi := make([]string, 0, len(queue))
	for _, t := range queue {
		if t != nil {
			qi = append(qi, t.ID)
		}
	}
	sort.Strings(qi)
	ri := make([]string, 0, len(running))
	for id := range running {
		ri = append(ri, id)
	}
	sort.Strings(ri)
	hi := make([]string, 0, len(hist))
	for id := range hist {
		hi = append(hi, id)
	}
	sort.Strings(hi)
	return "q:" + strings.Join(qi, ",") + "|r:" + strings.Join(ri, ",") + "|h:" + strings.Join(hi, ",")
}

// fxReadPersisted 读盘并解析（解析失败 = 半文件/0 字节 ⇒ 直接 Fatal，正是 F-1 要咬的形态）。
func fxReadPersisted(t *testing.T, path string) persistedTasks {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读盘失败 %s: %v", path, err)
	}
	if len(buf) == 0 {
		t.Fatalf("盘上件为 0 字节（F-1 咬点）: %s", path)
	}
	var d persistedTasks
	if err := json.Unmarshal(buf, &d); err != nil {
		t.Fatalf("盘上件非完整 JSON（半文件——F-1 咬点）: %v", err)
	}
	return d
}

// ============ F-1 · 原子性 / 断电仿真 ============

// fxBigQueue 造一份体积可观的排队状态（撑宽直写负控的「截断→写入」窗口）。
func fxBigQueue(round, n int) TaskQueue {
	q := make(TaskQueue, 0, n)
	for i := 0; i < n; i++ {
		heap.Push(&q, &Task{
			ID:          fmt.Sprintf("fxq-%03d-%05d", round, i),
			Description: strings.Repeat("y", 48),
			Status:      StatusQueued,
			Model:       "m",
			Workdir:     "/tmp",
			CreatedAt:   time.Now(),
		})
	}
	return q
}

// fxTmpLeftovers 目录里残留的原子写临时件（正常路径恒 0 个）。
func fxTmpLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败 %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tasks-") && strings.HasSuffix(e.Name(), ".tmp") {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestCancelFxF1AtomicNoTornWrite — 断电仿真（§11.6 #1）:
//
//	① 【确定性】覆盖写**绝不原地改写**目标件 —— 旧 inode 上的旧完整内容必须原封不动
//	   （覆盖前打开的旧 fd 仍读到旧字节）；目标 inode 每次覆盖必被**替换**（临时件 + rename）。
//	   直写（O_TRUNC + write，同 inode 截断）⇒ 旧 fd 读到新字节 / inode 不变 ⇒ 红。
//	② 【确定性】每次覆盖后目标件完整可解析、无 .tasks-*.tmp 残留。
//	③ 【并发面】持续覆盖 + 紧密读盘 —— 每一发读到的都必须是「旧完整」或「新完整」之一，
//	   绝无 0 字节 / 半文件（解析必过且签名落在已知集）。
//
// ① 断言的是断电安全的**机制**（原子替换 = 任何时刻盘上都是一个完整 inode），是确定性判据；
// ③ 是 §11.6 原义的并发仿真，作补强（原子写恒满足 ⇒ 确定性绿）。
func TestCancelFxF1AtomicNoTornWrite(t *testing.T) {
	dst, dir := fxIsolatedPaths(t)

	// 旧完整状态 A: 空队列 + 一条 history。
	histA := map[string]*Task{"fx1-seed": {ID: "fx1-seed", Status: StatusDone, Model: "m", Workdir: "/tmp", CreatedAt: time.Now()}}
	if err := saveTasksLocked(TaskQueue{}, map[string]*Task{}, histA, map[string]*Task{}); err != nil {
		t.Fatalf("预置旧完整状态失败: %v", err)
	}
	bufA, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读旧状态失败: %v", err)
	}
	fiA, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat 旧状态失败: %v", err)
	}

	// ---- ① 确定性: 覆盖写不得原地改写旧 inode ----
	oldFD, err := os.Open(dst)
	if err != nil {
		t.Fatalf("打开旧 inode 失败: %v", err)
	}
	defer oldFD.Close()
	if err := saveTasksLocked(fxBigQueue(1, 4000), map[string]*Task{}, histA, map[string]*Task{}); err != nil {
		t.Fatalf("覆盖写失败: %v", err)
	}
	gotOld, err := io.ReadAll(oldFD)
	if err != nil {
		t.Fatalf("经旧 fd 读失败: %v", err)
	}
	if !bytes.Equal(gotOld, bufA) {
		t.Errorf("① 覆盖写原地改写了旧 inode: 旧 fd 实得 %d 字节，旧完整应为 %d 字节 —— 断电/并发读者会看到半文件或 0 字节", len(gotOld), len(bufA))
	}
	fiB, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat 新状态失败: %v", err)
	}
	if os.SameFile(fiA, fiB) {
		t.Errorf("① 覆盖写未替换 inode（SameFile=true）—— 目标件被原地改写，非原子替换")
	}

	// ---- ② 确定性: 覆盖后完整可解析 + 无临时件残留 ----
	fxReadPersisted(t, dst)
	if lo := fxTmpLeftovers(t, dir); len(lo) != 0 {
		t.Errorf("② 覆盖写留下临时件 %v（失败/成功路径都必须自清或 rename 掉）", lo)
	}

	// ---- ③ 并发面: 持续覆盖 + 紧密读 ----
	const nPerRound = 1500
	const rounds = 30
	known := map[string]bool{}
	known[fxSignature(nil, map[string]*Task{}, map[string]*Task{"fx1-seed": nil})] = true
	seedHist := func() map[string]*Task {
		return map[string]*Task{"fx1-seed": {ID: "fx1-seed", Status: StatusDone, Model: "m", CreatedAt: time.Now()}}
	}
	for r := 2; r < 2+rounds; r++ {
		ids := make([]string, 0, nPerRound)
		for i := 0; i < nPerRound; i++ {
			ids = append(ids, fmt.Sprintf("fxq-%03d-%05d", r, i))
		}
		sort.Strings(ids)
		known["q:"+strings.Join(ids, ",")+"|r:|h:fx1-seed"] = true
	}
	{ // ① 的覆盖态（round 1, 4000 件）也要登记，否则采样会误判为「混合状态」
		ids := make([]string, 0, 4000)
		for i := 0; i < 4000; i++ {
			ids = append(ids, fmt.Sprintf("fxq-%03d-%05d", 1, i))
		}
		sort.Strings(ids)
		known["q:"+strings.Join(ids, ",")+"|r:|h:fx1-seed"] = true
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for r := 2; r < 2+rounds; r++ {
			_ = saveTasksLocked(fxBigQueue(r, nPerRound), map[string]*Task{}, seedHist(), map[string]*Task{})
		}
	}()
	var samples, torn, unknown int
	var firstTorn string
	finished := false
	for !finished {
		select {
		case <-done:
			finished = true
		default:
		}
		buf, err := os.ReadFile(dst)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("读目标件失败: %v", err)
		}
		samples++
		if len(buf) == 0 {
			torn++
			if firstTorn == "" {
				firstTorn = "读到 0 字节"
			}
			continue
		}
		var d persistedTasks
		if err := json.Unmarshal(buf, &d); err != nil {
			torn++
			if firstTorn == "" {
				firstTorn = fmt.Sprintf("读到 %d 字节的残缺 JSON: %v", len(buf), err)
			}
			continue
		}
		if !known[fxSignature(d.Queue, d.Running, d.History)] {
			unknown++
			if firstTorn == "" {
				firstTorn = "读到既非旧也非新的混合状态"
			}
		}
	}
	if torn > 0 || unknown > 0 {
		t.Fatalf("③ F-1 咬点: 盘上出现非完整状态 torn=%d unknown=%d（共采样 %d）: %s", torn, unknown, samples, firstTorn)
	}
}

// ============ F-3 · 队列卡死回归（真跑取消 + 落盘 + 重建实例）============

// TestCancelFxF3CancelRestartNoDuplicate — 用被测代码真跑一遍取消 + 落盘 + 重建实例:
//
//	① 盘上件: 被取消件恰在一处（history），**不在 queue**（不会既在 queue 又在 history）；
//	② 盘上件: 下一个排队件仍在 queue；
//	③ 重启（NewMasterScheduler 走 loadPersistedHistory）后被取消件**不复活进 queue**、
//	   status=cancelled 且落在 history；下一个排队件被正确恢复进 queue。
func TestCancelFxF3CancelRestartNoDuplicate(t *testing.T) {
	dst, _ := fxIsolatedPaths(t)
	// maxConcurrent=0: 让迁移后的 dispatchDecideLocked 决策**不真派发**（不 spawn runTask），
	// 只在盘上/内存态上判「摘容器 + 落盘 + 重建恢复」；避免后台 goroutine 越出用例污染后续判据。
	s := fxNewScheduler(t, 0)

	victim := &Task{ID: "fx3-victim", Description: "待撤", Status: StatusQueued, Priority: PriorityExternal, Model: "m", CreatedAt: time.Now()}
	follower := &Task{ID: "fx3-follower", Description: "下一个", Status: StatusQueued, Priority: PriorityInternal, Model: "m", CreatedAt: time.Now()}
	s.mu.Lock()
	heap.Push(&s.queue, victim)
	heap.Push(&s.queue, follower)
	s.mu.Unlock()

	// 预置盘上初始队列（模拟「上一次检查点已落盘」）: 传副本避免 drain 扰动 s.queue 的堆序。
	if err := saveTasksLocked(append(TaskQueue{}, s.queue...), s.running, s.history, s.waiting); err != nil {
		t.Fatalf("预置盘上初始队列失败: %v", err)
	}

	outcome, err := s.CancelTask("fx3-victim", "superseded", "前提已变")
	if err != nil {
		t.Fatalf("取消排队件应成功，实得 err=%v", err)
	}
	if outcome != CancelOutcomeCancelled {
		t.Fatalf("结局应为 cancelled，实得 %q", outcome)
	}
	if s.QueueLen() != 1 {
		t.Fatalf("取消后队列应只剩 1 件（被取消件必须被摘出），实得 %d", s.QueueLen())
	}

	// ---- ① + ② 盘上文件（真序列化）----
	data := fxReadPersisted(t, dst)
	if fxContainsStr(idsOfTasks(data.Queue), "fx3-victim") {
		t.Errorf("盘上 queue 仍含被取消件 ⇒ 既在 queue 又在 history（重复/卡死血证 §2.6）")
	}
	if _, ok := data.History["fx3-victim"]; !ok {
		t.Errorf("盘上 history 应含被取消件")
	}
	if !fxContainsStr(idsOfTasks(data.Queue), "fx3-follower") {
		t.Errorf("盘上 queue 应含下一个排队件 fx3-follower，实得 %v", idsOfTasks(data.Queue))
	}

	// ---- ③ 重建实例（重启）----
	s2 := fxNewScheduler(t, 0)
	s2.mu.Lock()
	_, vInQueue := fxIndexOf(s2.queue, "fx3-victim")
	vHist, vOK := s2.history["fx3-victim"]
	_, fInQueue := fxIndexOf(s2.queue, "fx3-follower")
	s2.mu.Unlock()

	if vInQueue {
		t.Errorf("重启后被取消件复活进 queue（终态不复活失守）")
	}
	if !vOK || vHist == nil || vHist.Status != StatusCancelled {
		t.Errorf("重启后被取消件应在 history 且 status=cancelled，实得 ok=%v task=%+v", vOK, vHist)
	}
	if !fInQueue {
		t.Errorf("重启后下一个排队件 fx3-follower 应被恢复进 queue")
	}
}

// idsOfTasks 从盘上 []*Task 取 ID 切片（供断言用）。
func idsOfTasks(ts []*Task) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		if t != nil {
			out = append(out, t.ID)
		}
	}
	return out
}

// ============ F-4 · 落盘时机回归（每次迁移恰好一次 save）============

// TestCancelFxF4ExactlyOneSavePerTransition — save 计数器断言「每次迁移恰好一次 save（不多不少）」:
//
//	① queued→cancelled 真取消: 恰 1 次；
//	② done→结案（只打标，正交面迁移）: 再恰 1 次；
//	③ 已 cancelled 幂等重放（no-op，非迁移）: 0 次（计数不变）。
func TestCancelFxF4ExactlyOneSavePerTransition(t *testing.T) {
	fxIsolatedPaths(t)
	s := fxNewScheduler(t, 0)

	count := 0
	orig := saveTasksForCancel
	saveTasksForCancel = func(q TaskQueue, r map[string]*Task, h map[string]*Task, w ...map[string]*Task) error {
		count++
		return saveTasksLocked(q, r, h, w...)
	}
	defer func() { saveTasksForCancel = orig }()

	// ① queued→cancelled
	a := &Task{ID: "fx4-a", Status: StatusQueued, Priority: PriorityExternal, Model: "m"}
	s.mu.Lock()
	heap.Push(&s.queue, a)
	s.mu.Unlock()
	if _, err := s.CancelTask("fx4-a", "obsolete", ""); err != nil {
		t.Fatalf("真取消应成功: %v", err)
	}
	if count != 1 {
		t.Fatalf("① queued→cancelled 迁移应恰好 1 次 save，实得 %d", count)
	}

	// ② done→结案（打标迁移）
	d := &Task{ID: "fx4-done", Status: StatusDone, Priority: PriorityExternal, Model: "m"}
	s.mu.Lock()
	s.history["fx4-done"] = d
	s.mu.Unlock()
	if _, err := s.CancelTask("fx4-done", "superseded", ""); err != nil {
		t.Fatalf("对 done 结案应成功: %v", err)
	}
	if count != 2 {
		t.Fatalf("② 结案（打标）迁移应使计数恰好 +1 ⇒ 2，实得 %d", count)
	}

	// ③ 已 cancelled 幂等重放: no-op，不迁移 ⇒ 不再 save
	c := &Task{ID: "fx4-c", Status: StatusCancelled, Priority: PriorityExternal, Model: "m"}
	c.markClosed("obsolete", "首跑")
	s.mu.Lock()
	s.history["fx4-c"] = c
	s.mu.Unlock()
	if _, err := s.CancelTask("fx4-c", "abandoned", "重放"); err != nil {
		t.Fatalf("幂等重放应静默成功: %v", err)
	}
	if count != 2 {
		t.Fatalf("③ 幂等重放（无状态迁移）不得新增 save，实得 %d", count)
	}
}

// ============ F-5 · W3 窗口（dispatch 之前中断 ⇒ 重启后盘上已是最终正确状态）============

const (
	fxF5CrashEnv = "ZERG_FX_F5_CRASH"
	fxF5StateEnv = "ZERG_FX_F5_STATE"
)

// TestCancelFxF5CrashChild — F-5 的**崩溃载体**（真 SIGKILL 子进程）。
// 无环境变量时 Skip（不参与常规 -run）；父用例带 ZERG_FX_F5_CRASH=1 拉起它。
//
// 崩溃点 = dispatchDecideLocked 的入口探针（save **之后**、任何派发状态变更**之前**）——
// 正是设计 §11.6 #5 的「dispatch 之前」窗口。此处 kill -9 本进程 ⇒ 盘上应当已是提交后
// （save 已完成）的最终正确状态: 被取消件进 history、出 queue、带 closed_*。
func TestCancelFxF5CrashChild(t *testing.T) {
	if os.Getenv(fxF5CrashEnv) != "1" {
		t.Skip("F-5 崩溃载体：由 TestCancelFxF5CrashBeforeDispatchKeepsFinalOnDisk 带环境变量拉起")
	}
	path := os.Getenv(fxF5StateEnv)
	if path == "" {
		t.Fatalf("子进程缺 %s", fxF5StateEnv)
	}
	tasksFile = path
	legacyTasksFile = filepath.Join(filepath.Dir(path), "legacy-never-used.json")

	s := NewMasterScheduler("/bin/echo", 0) // 从盘恢复父用例预置的初始状态（0 并发: 决策不真 spawn）
	dispatchDecideProbe = func() {
		// dispatch 之前中断: 真 SIGKILL 本进程（不可被 defer/recover 吞掉）。
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	}
	_, _ = s.CancelTask("fx5-victim", "superseded", "W3 窗口")
	// 只有探针**未触发**才会走到这里 ⇒ 夹具构建有误（夹具自身失败，不算被测代码绿）。
	t.Fatalf("dispatch 探针未被触发 —— CancelTask 未在 save 后进入派发决策，F-5 崩溃点无效")
}

// TestCancelFxF5CrashBeforeDispatchKeepsFinalOnDisk — 拉起子进程真 SIGKILL（在 dispatch 之前），
// 断言崩溃瞬间盘上**已是最终正确状态**，且重启读回同一最终状态。
func TestCancelFxF5CrashBeforeDispatchKeepsFinalOnDisk(t *testing.T) {
	dst, dir := fxIsolatedPaths(t)

	// 预置盘上初始状态: victim(queued, 高优先) + follower(queued)。
	initQ := TaskQueue{}
	heap.Push(&initQ, &Task{ID: "fx5-victim", Description: "待撤", Status: StatusQueued, Priority: PriorityExternal, Model: "m", CreatedAt: time.Now()})
	heap.Push(&initQ, &Task{ID: "fx5-follower", Description: "下一个", Status: StatusQueued, Priority: PriorityInternal, Model: "m", CreatedAt: time.Now()})
	if err := saveTasksLocked(initQ, map[string]*Task{}, map[string]*Task{}, map[string]*Task{}); err != nil {
		t.Fatalf("预置初始状态失败: %v", err)
	}

	// 真子进程 + 真 SIGKILL（不经 test 进程）。
	cmd := exec.Command(os.Args[0], "-test.run=^TestCancelFxF5CrashChild$")
	cmd.Env = append(os.Environ(), fxF5CrashEnv+"=1", fxF5StateEnv+"="+dst, "TMPDIR="+dir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("子进程应被 SIGKILL 杀死（不得正常退出 0）；out=%s", out)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("子进程非 ExitError: %v (out=%s)", err, out)
	}
	ws, ok := ee.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("子进程应被 SIGKILL 杀死，实得 status=%v (out=%s)", ee.Sys(), out)
	}

	// ---- 崩溃瞬间: 盘上已是最终正确状态（提交点在 dispatch 之前）----
	data := fxReadPersisted(t, dst)
	if fxContainsStr(idsOfTasks(data.Queue), "fx5-victim") {
		t.Errorf("崩溃后盘上 queue 仍含被取消件 ⇒ 提交点落在 dispatch 之后（W3 窗口丢提交）")
	}
	v, vOK := data.History["fx5-victim"]
	if !vOK || v == nil || v.Status != StatusCancelled || v.ClosedAt == nil {
		t.Errorf("崩溃后盘上 history 应含已 cancelled + closed_* 的被取消件，实得 ok=%v task=%+v", vOK, v)
	}
	if !fxContainsStr(idsOfTasks(data.Queue), "fx5-follower") {
		t.Errorf("崩溃后盘上 queue 应含下一个排队件 fx5-follower，实得 %v", idsOfTasks(data.Queue))
	}

	// ---- 重启: 读回最终正确状态 ----
	s := fxNewScheduler(t, 0)
	s.mu.Lock()
	_, vInQueue := fxIndexOf(s.queue, "fx5-victim")
	vHist, vHOK := s.history["fx5-victim"]
	_, fInQueue := fxIndexOf(s.queue, "fx5-follower")
	s.mu.Unlock()
	if vInQueue {
		t.Errorf("重启后被取消件不得复活进 queue")
	}
	if !vHOK || vHist == nil || vHist.Status != StatusCancelled {
		t.Errorf("重启后被取消件应在 history 且 cancelled，实得 ok=%v task=%+v", vHOK, vHist)
	}
	if !fInQueue {
		t.Errorf("重启后下一个排队件 fx5-follower 应被恢复进 queue")
	}
}
