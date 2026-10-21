package api

// taskstate_invariant_test.go — 任务状态机三条**落盘不变式**的判据（2026-10-21 草案）
//
// 判据只看**盘上的文件**（persistedTasks），不看内存字段——内存对了盘上错，重启即复发。
//   I1 时间与状态自洽: 非终态（queued/paused/running/waiting_retry）⟹ completed_at 为零值；
//      终态（done/failed/cancelled）⟹ 保留完成时刻（不许被顺手清掉）。
//   I2 created_at 只补零值、**永不覆写**: 重启恢复后 created_at 与落盘前逐字相等。
//   I3 同一 ID 只落一个分区: queue/running/waiting 与 history 互不相交。
//
// 红/绿实测（2026-10-21，go test -overlay 打补丁前/后）:
//   补丁前: TestInvariantI1I3_* / TestInvariantI2_* 三条全红（真机数据 87 件 queued 带 completed_at 同因）；
//   补丁后: 全绿（本包既有 200+ 用例同时全绿）。

import (
	"container/heap"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// readPersistedForTest 读回**写路径**上的落盘件（tasksWritePath 已由 isolateTasksFile 切到临时目录）。
func readPersistedForTest(t *testing.T) persistedTasks {
	t.Helper()
	buf, err := os.ReadFile(tasksWritePath())
	if err != nil {
		t.Fatalf("读落盘件失败（%s）: %v", tasksWritePath(), err)
	}
	var data persistedTasks
	if err := json.Unmarshal(buf, &data); err != nil {
		t.Fatalf("落盘件不是合法 JSON: %v", err)
	}
	return data
}

// TestInvariantI1I3_RequeuedTaskCarriesNoCompletionStamp
// 现场复刻: runTask 失败 → 回队（same ID）+ 旧实现已写过 CompletedAt + 无条件进过 history。
func TestInvariantI1I3_RequeuedTaskCarriesNoCompletionStamp(t *testing.T) {
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)

	task := &Task{
		ID:          "inv-requeue-1",
		Status:      StatusQueued,
		Model:       "m",
		Workdir:     t.TempDir(),
		CreatedAt:   time.Now().Add(-48 * time.Hour),
		CompletedAt: time.Now(), // 旧 runTask 在「分支之前」就写了；回队时没人清
	}
	heap.Push(&s.queue, task)
	s.history[task.ID] = task // 旧 runTask 无条件入档 ⇒ 同一指针落在两个容器里

	if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	data := readPersistedForTest(t)

	var inQueue bool
	for _, q := range data.Queue {
		if q == nil || q.ID != task.ID {
			continue
		}
		inQueue = true
		if !q.CompletedAt.IsZero() {
			t.Errorf("I1 破: queued 任务 %s 落盘带 completed_at=%s（回队必须清零——UI 耗时会算成负数）",
				q.ID, q.CompletedAt.Format(time.RFC3339))
		}
		if q.CreatedAt.IsZero() {
			t.Errorf("I2 破: created_at 被清空")
		}
	}
	if !inQueue {
		t.Fatalf("queued 任务 %s 没落进 queue 分区", task.ID)
	}
	if _, dup := data.History[task.ID]; dup {
		t.Errorf("I3 破: 同一 ID %s 同时在 history 与 queue 落盘（存量 77 件重复即此形态）", task.ID)
	}
}

// TestInvariantI2_CreatedAtSurvivesRestart
// 现场复刻: 盘上 queued 任务的 created_at 是 9 月，进程 10 月重启 —— 旧实现把它覆写成「现在」。
func TestInvariantI2_CreatedAtSurvivesRestart(t *testing.T) {
	p := isolateTasksFile(t)
	isolateTaskRoot(t)

	created := time.Date(2026, 9, 25, 15, 12, 9, 0, time.UTC)
	seed := persistedTasks{
		SavedAt: time.Now(),
		History: map[string]*Task{},
		Running: map[string]*Task{},
		Waiting: map[string]*Task{},
		Queue: []*Task{{
			ID:        "inv-restore-1",
			Status:    StatusQueued,
			Model:     "m",
			CreatedAt: created,
			// 旧件遗留: 非终态却带着 completed_at（本次修法要把它清掉，并回退成 WaitingSince 的来源）
			CompletedAt: time.Date(2026, 9, 27, 5, 43, 57, 0, time.UTC),
		}},
	}
	buf, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewMasterScheduler("", 1) // 构造即 loadPersistedHistory
	var got *Task
	for _, q := range s.queue {
		if q.ID == "inv-restore-1" {
			got = q
		}
	}
	if got == nil {
		t.Fatalf("queued 任务没被恢复")
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("I2 破: 重启把 created_at 从 %s 改成 %s（差 %s）",
			created.Format(time.RFC3339), got.CreatedAt.Format(time.RFC3339),
			got.CreatedAt.Sub(created).Round(time.Second))
	}
	if !got.CompletedAt.IsZero() {
		t.Errorf("I1 破: 恢复后非终态仍带 completed_at=%s", got.CompletedAt.Format(time.RFC3339))
	}
}

// TestInvariantI1_TerminalKeepsStamp — 反向守卫: 别把终态件的完成时刻顺手清掉（改狠了同样是错）。
func TestInvariantI1_TerminalKeepsStamp(t *testing.T) {
	isolateTasksFile(t)
	isolateTaskRoot(t)
	s := NewMasterScheduler("", 1)

	done := time.Now().Add(-time.Hour).Truncate(time.Second)
	task := &Task{ID: "inv-done-1", Status: StatusDone, Model: "m", CreatedAt: done.Add(-time.Minute), CompletedAt: done}
	s.history[task.ID] = task

	if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	data := readPersistedForTest(t)
	got := data.History["inv-done-1"]
	if got == nil {
		t.Fatalf("终态件没落进 history 分区")
	}
	if !got.CompletedAt.Equal(done) {
		t.Errorf("I1 破: 终态件完成时刻被改/清（want %s, got %s）",
			done.Format(time.RFC3339), got.CompletedAt.Format(time.RFC3339))
	}
}
