package api

// tasks_persist.go — 任务状态持久化（v2.5.5 P1-5 补 + 2026-08-20 完整版）
// 2026-09-18 修（实调发现缺陷）: 原写死 /tmp/zerg-tasks.json —— 多实例共享同一份文件、
// 互相覆盖任务历史（沙箱金丝雀实测把它回写了）。改为 statepath 统一状态目录派生
// （ZERG_STATE_DIR → ~/.zerg/state/zerg-tasks.json）。
// 迁移兼容（首次）: 新路径不存在而旧 /tmp/zerg-tasks.json 存在 ⇒ **读旧一次**（不丢历史）；
// **写只写新路径**；旧文件**不删、不改**。新路径已存在 ⇒ 旧路径完全不看。
// 启动 → 加载回 history + 恢复排队任务（重启不丢——AutoCUT 反复丢问题治本）

import (
	"container/heap"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// tasksStateFileName 任务持久化文件在统一状态目录下的文件名。
const tasksStateFileName = "zerg-tasks.json"

// tasksLegacyDefaultPath 旧硬编码落点字面量（v2.5.5 起写死 —— 多实例共享同一份、互相覆盖历史）。
// 只作**首次迁移读取**来源：读一次；不删、不改、永不写入。
const tasksLegacyDefaultPath = "/tmp/zerg-tasks.json"

// legacyTasksFile 旧路径（包级变量 = 上面的字面量；迁移用例/测试进程隔离可切换）。
var legacyTasksFile = tasksLegacyDefaultPath

// tasksFile 任务持久化文件（包级变量——测试可切换隔离路径——2026-08-28）
// 语义（2026-09-18）: 非空 ⇒ 直接用该路径（测试隔离/显式覆盖）；
// 空 ⇒ 走 statepath 统一状态目录派生（生产默认，见 tasksWritePath）。
var tasksFile = ""

// tasksWritePath 写路径：永远是统一状态目录（ZERG_STATE_DIR → ~/.zerg/state/<tasksStateFileName>）。
// 永不写旧 /tmp 路径——多实例各写各的（不再互相覆盖历史）。
func tasksWritePath() string {
	if p := strings.TrimSpace(tasksFile); p != "" {
		return p
	}
	return statepath.File(tasksStateFileName)
}

// tasksReadPath 读路径：统一状态目录优先；新路径不存在且旧 /tmp 存在 ⇒ 读旧一次（迁移兼容）。
// 新路径存在 ⇒ 旧路径完全不看（不 stat、不读——旧内容不夹除）。
// 显式覆盖 tasksFile（测试隔离）时不退旧路径：覆盖即「我已指定唯一来源」，避免测试读真机 /tmp。
func tasksReadPath() string {
	p := tasksWritePath()
	if _, err := os.Stat(p); err == nil {
		return p // 新路径已存在——旧路径完全不看
	}
	if strings.TrimSpace(tasksFile) != "" {
		return p
	}
	legacy := strings.TrimSpace(legacyTasksFile)
	if legacy == "" {
		return p
	}
	if _, err := os.Stat(legacy); err != nil {
		return p // 无旧文件——首次运行（空历史）
	}
	log.Printf("📜 任务持久化首次迁移: 读旧 %s（只读一次——旧文件保留不删；写入只落 %s）\n", legacy, p)
	return legacy
}

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
	// 写只写新路径（统一状态目录）——旧 /tmp 路径永不写（2026-09-18）
	dst := tasksWritePath()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		log.Printf("⚠️ 任务持久化目录不可用（%s）: %v\n", dst, err)
		return
	}
	if err := os.WriteFile(dst, buf, 0644); err != nil {
		log.Printf("⚠️ 任务持久化写盘失败（%s）: %v\n", dst, err)
	}
}

// loadPersistedHistory 加载历史任务 + 恢复排队（启动时调）
func (s *MasterScheduler) loadPersistedHistory() {
	// 读路径：新（统一状态目录）优先；新缺失 + 旧 /tmp 在 ⇒ 读旧一次（2026-09-18 迁移兼容）
	buf, err := os.ReadFile(tasksReadPath())
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
	// 2026-09-13 并发修复: 恢复只入队——不在此处派发。
	// 构造期派发 = 实例还没交给调用方就已有 goroutine 在跑任务、改 queue/running/history
	// （数据竞争 + 无停止手段）；排队任务改由调用方显式 Start() 派发（生产: main.go 装配完成后）。
	// 恢复调度器（waiting 扫描）同样由 Start() 启动。
}
