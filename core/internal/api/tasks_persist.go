package api

// tasks_persist.go — 任务状态持久化（v2.5.5 P1-5 补 + 2026-08-20 完整版）
// 任务完成/失败 → 写 /tmp/zerg-tasks.json（含历史 + 队列 + 运行中）
// 启动 → 加载回 history + 恢复排队任务（重启不丢——AutoCUT 反复丢问题治本）

import (
	"container/heap"
	"log"
	"encoding/json"
	"os"
	"time"
)

// tasksFile 任务持久化文件（包级变量——测试可切换隔离路径——2026-08-28）
var tasksFile = "/tmp/zerg-tasks.json"

// persistedTasks 落盘结构（完整版——history + queue + running + waiting）
type persistedTasks struct {
	SavedAt time.Time        `json:"saved_at"`
	History map[string]*Task `json:"history"`
	Queue   []*Task          `json:"queue"`   // 排队中任务（重启恢复重新派发）
	Running map[string]*Task `json:"running"` // 运行中任务（重启标 failed——无法恢复执行——但记录）
	Waiting map[string]*Task `json:"waiting"` // v2.5.6 故障自愈: 环境故障挂起任务（重启恢复——继续等）
}

// saveTasksLocked 保存任务状态（调用方持锁）
// v2.5.6 故障自愈: 增加 waiting 参数（环境故障挂起任务——重启恢复继续等）
func saveTasksLocked(queue TaskQueue, running map[string]*Task, history map[string]*Task, waiting ...map[string]*Task) {
	// 完整版（2026-08-20）: history + queue + running 都存——重启不丢任何任务
	data := persistedTasks{
		SavedAt: time.Now(),
		History: make(map[string]*Task, len(history)),
		Queue:   []*Task{},
		Running: make(map[string]*Task, len(running)),
		Waiting: make(map[string]*Task),
	}
	for id, t := range history {
		data.History[id] = t
	}
	// 队列（heap 遍历——按优先级序）
	for queue.Len() > 0 {
		t := queue.Pop().(*Task)
		data.Queue = append(data.Queue, t)
	}
	// 重新 push 回 heap（遍历会清空——恢复）
	for _, t := range data.Queue {
		queue.Push(t)
	}
	for id, t := range running {
		data.Running[id] = t
	}
	// waiting（可选参数——兼容旧调用）
	if len(waiting) > 0 {
		for id, t := range waiting[0] {
			data.Waiting[id] = t
		}
	}
	buf, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = os.WriteFile(tasksFile, buf, 0644)
}

// loadPersistedHistory 加载历史任务 + 恢复排队（启动时调）
func (s *MasterScheduler) loadPersistedHistory() {
	buf, err := os.ReadFile(tasksFile)
	if err != nil {
		return // 首次运行——无历史
	}
	var data persistedTasks
	if err := json.Unmarshal(buf, &data); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range data.History {
		if _, exists := s.history[id]; !exists {
			// v2.5.6 2026-08-28: 重启时运行中任务无法恢复执行——标 failed（防 UI 残留 running 误导）
			if t.Status == "running" {
				t.Status = "failed"
				t.FailReason = "重启中断（运行中任务无法恢复——重新触发）"
			}
			s.history[id] = t
		}
	}
	// 恢复排队任务（重新入队——重启后继续执行——不停丢）
	restored := 0
	for _, t := range data.Queue {
		if t.Status == "queued" || t.Status == "paused" {
			t.Status = "queued"
			t.CreatedAt = time.Now() // 重启时间作为新创建（避免时长异常）
			heap.Push(&s.queue, t)
			restored++
		}
	}
	// 运行中任务（重启前在跑——无法恢复执行——标 failed 记录——防悬空）
	for id, t := range data.Running {
		if _, exists := s.history[id]; !exists {
			t.Status = "failed"
			t.FailReason = "主控重启——运行中任务中断（未完成）"
			t.CompletedAt = time.Now()
			s.history[id] = t
		}
	}
	// v2.5.6 故障自愈: 恢复挂起任务（waiting_retry——重启后继续等环境恢复——不丢）
	waitingRestored := 0
	for id, t := range data.Waiting {
		if t.Status == "waiting_retry" {
			s.waiting[id] = t
			waitingRestored++
		}
	}
	if len(data.History) > 0 || restored > 0 || waitingRestored > 0 {
		log.Printf("📜 恢复任务: 历史 %d + 排队 %d + 挂起 %d（持久化完整版——重启不丢）\n", len(data.History), restored, waitingRestored)
	}
	// 恢复后触发派发（排队任务开始执行）
	if restored > 0 {
		s.dispatchLocked()
	}
	// 恢复调度器已启动（NewMasterScheduler 内）——waiting 任务由它扫描重派
}
