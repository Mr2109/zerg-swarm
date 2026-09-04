package api

// ============ v2.5.5 T3 主控总调度器——骨架 ============
// 两级调度（Mr2109原理）:
//   主控总调度: 管所有内部任务 + 接入的外部任务（全局决策/派发）
//   CA 子调度: 管分派任务的执行（agent 侧已有 scheduler）
// 借鉴: Kubernetes 优先级/抢占、LowPriority LLM顾问+策略法律、Zabbix 优先级堆

import (
	"container/heap"
	"log"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TaskPriority 任务优先级（外部高——内部低）
type TaskPriority int

const (
	PriorityInternal TaskPriority = 10 // 内部任务（进化——低优先级）
	PriorityExternal TaskPriority = 50 // 外部任务（服务——高优先级）
)

// Task 总调度任务
type Task struct {
	ID          string       `json:"id"`       // 任务 ID（唯一）
	Description string       `json:"description"` // 任务描述
	Priority    TaskPriority `json:"priority"` // 优先级（外部高/内部低）
	Type        string       `json:"type"`     // internal/external
	Status      string       `json:"status"`   // queued/running/paused/done/failed
	FailReason  string       `json:"fail_reason,omitempty"` // 失败原因（确定性验证/执行失败——UI 显示）
	RefTaskID   string       `json:"ref_task_id,omitempty"` // 关联任务（复查任务 → 执行任务）
	// v2.5.7 对话→任务集成: 来源对话/消息关联（UI"派任务"——记录任务从哪个对话发起）
	ParentSessionID  string `json:"parent_session_id,omitempty"`
	ParentMessageID  string `json:"parent_message_id,omitempty"`
	RefWorktree string       `json:"ref_worktree,omitempty"` // 关联 worktree（复查用）
	ReplanCount int          `json:"replan_count,omitempty"` // 重做次数（阶段3 Replan 循环——上限 3）
	ReviewCount int          `json:"review_count,omitempty"` // 复查次数（阶段3——上限 5）
	RetryCount  int          `json:"retry_count,omitempty"` // 自动重跑次数（失败自动重跑——上限 3——Mr2109 2026-08-20）
	Model       string       `json:"model"`    // 派发用模型
	Workdir     string       `json:"workdir"`  // 工作区
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt time.Time    `json:"completed_at"` // v2.5.5 虫族UI: 完成时间（耗时计算）
	Checkpoint  string       `json:"checkpoint"` // 检查点（打断恢复用——.zerg/logs/<ID>/checkpoint.json）
	IssuePath   string       `json:"issue_path"` // v2.5.5 P1-3: 任务单路径（idle_detector 建的 docs/issues/*.md——完成时更新状态）
	Flow        string       `json:"flow,omitempty"` // v2.5.6: 执行流程（"zerg"=程序定量驱动新流程——空=旧 CA 流程）
	Machine     string       `json:"machine,omitempty"` // v2.5.6: 执行设备（模型所在机器——handlers 按 modelMachineMap 推算——UI 详情显示）
	SkillKey    string       `json:"skill_key,omitempty"` // v2.5.6: skill 归属 key（内部任务=def.ID——独属 skill；空=外部任务按类型共享）
	cmd         *exec.Cmd    // 运行中的 CA 进程（打断发信号用——非导出）
}

// TaskQueue 优先级队列（container/heap）
type TaskQueue []*Task

func (q TaskQueue) Len() int            { return len(q) }
func (q TaskQueue) Less(i, j int) bool  { return q[i].Priority > q[j].Priority } // 高优先先出
func (q TaskQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *TaskQueue) Push(x interface{}) { *q = append(*q, x.(*Task)) }
func (q *TaskQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}

// MasterScheduler 主控总调度器
type MasterScheduler struct {
	mu       sync.Mutex
	queue    TaskQueue // 优先级队列（外部高——内部低）
	running  map[string]*Task // 运行中任务
	history  map[string]*Task // v2.5.5 虫族UI: 已完成/失败任务历史（UI 列表显示——不丢）
	waiting  map[string]*Task // v2.5.6 故障自愈: 环境故障挂起任务（waiting_retry——机器恢复自动重派）
	maxConcurrent int    // 最大并发（单槽——1 个任务在跑）
	agentCmd string   // CA 启动命令（zerg-agent 路径）
	agentEnv []string // CA 环境变量（网关地址/token）
	taskDirForCall string // v2.5.6: 当前 zerg 流程任务目录（write_file 工具写文件用）
	pingMu    sync.Mutex        // v2.5.6 故障自愈: 模型探活缓存锁
	pingCache map[string]pingResult // v2.5.6 模型探活结果缓存（10s——排队任务不重复 ping）
	store     StoreReader       // v2.5.6 ping 三级漏斗: 快照读取（第1级——快照优先——0ms）
}

// StoreReader 快照读取接口（注入——解耦——测试可 mock）
// 第1级快照优先: 机器 healthy + 已加载目标模型 → 直接通过（不发请求）
type StoreReader interface {
	// MachineSnapshot 查机器快照（nil=无快照）
	MachineSnapshot(machine string) *FleetSnapshotLite
}

// FleetSnapshotLite 快照精简视图（ping 判定用——不依赖 store 包）
type FleetSnapshotLite struct {
	Healthy bool
	Model   string // 当前加载模型（文件名形式或逻辑名）
	Models  []string
}

// NewMasterScheduler 创建主控总调度器
// v2.5.6: store 参数可选（nil=无快照读取——ping 退化两级：响应头探测+完整推理）
func NewMasterScheduler(agentCmd string, maxConcurrent int, store ...StoreReader) *MasterScheduler {
	s := &MasterScheduler{
		queue:         TaskQueue{},
		running:       make(map[string]*Task),
		history:       make(map[string]*Task), // v2.5.5 虫族UI: 任务历史
		waiting:       make(map[string]*Task), // v2.5.6 故障自愈: 环境故障挂起任务
		maxConcurrent: maxConcurrent,
		agentCmd:      agentCmd,
	}
	if len(store) > 0 {
		s.store = store[0]
	}
	heap.Init(&s.queue)
	// v2.5.5 P1-5 补: 恢复历史任务（重启后保留——UI 显示所有任务）
	s.loadPersistedHistory()
	// v2.5.6 故障自愈: 启动恢复调度器（周期扫 waiting_retry——机器恢复自动重派）
	s.startRecoveryLoop()
	return s
}

// startRecoveryLoop 启动恢复调度器（v2.5.6 故障自愈——Mr2109 2026-08-28）
// 周期性扫描 waiting 任务——双条件判定（熔断冷却过 + 机器 healthy）——满足回队列重派
// 先例: main.go 本机心跳 goroutine（30s ticker）——同模式
func (s *MasterScheduler) startRecoveryLoop() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.recoverWaitingLocked()
		}
	}()
}

// recoverWaitingLocked 扫描 waiting 任务——恢复条件满足则回队列重派
// 调用方无需持锁（内部自己加锁）
// v2.5.6 修复（Mr2109 2026-08-28）: ping 网络请求移出锁外——之前持锁调 pingModel
// （最坏 30s/任务）→ 恢复扫描时整个调度器卡死——先快照任务列表——释放锁——再逐个 ping
func (s *MasterScheduler) recoverWaitingLocked() {
	// 快照等待列表（锁内只做快照——不做网络调用）
	s.mu.Lock()
	if len(s.waiting) == 0 {
		s.mu.Unlock()
		return
	}
	now := time.Now()
	var expired []*Task   // 超上限——降级 failed
	var candidates []*Task // 待 ping 探测
	for _, task := range s.waiting {
		if now.Sub(task.CompletedAt) > maxWaitRetry {
			expired = append(expired, task)
		} else {
			candidates = append(candidates, task)
		}
	}
	s.mu.Unlock()

	// 锁外 ping 探测（网络请求——不阻塞调度器）
	var recovered []*Task
	for _, task := range candidates {
		// 恢复条件①: 网关可用（ping 通=模型可推理——环境恢复）
		if ok, _ := s.pingModel(task.Model); ok {
			recovered = append(recovered, task)
		}
	}

	// 回锁——处理结果
	s.mu.Lock()
	for _, task := range expired {
		task.Status = "failed"
		task.FailReason = "环境故障持续超过 30 分钟——自动放弃（需人工介入）: " + task.FailReason
		delete(s.waiting, task.ID)
		s.history[task.ID] = task
		log.Printf("❌ 总调度: 任务 %s 环境故障超时放弃（>30min——需人工介入）", task.ID)
	}
	for _, task := range recovered {
		delete(s.waiting, task.ID)
		task.Status = "queued"
		heap.Push(&s.queue, task)
		log.Printf("♻️ 总调度: 任务 %s 环境恢复——重新入队（等待重派）", task.ID)
	}
	// 有恢复——持久化 + 派发
	if len(s.queue) > 0 {
		saveTasksLocked(s.queue, s.running, s.history, s.waiting)
		s.dispatchLocked()
	}
	s.mu.Unlock()
}

// maxWaitRetry 环境故障最大等待时间（30 分钟——超限降级 failed）
const maxWaitRetry = 30 * time.Minute

// Submit 提交任务（外部/内部都走这——优先级排队）
func (s *MasterScheduler) Submit(task *Task) {
	s.mu.Lock()
	// v2.5.5 虫族UI: 任务创建时间（UI 显示执行时长用——之前是零值）
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	// v2.5.5 打断/恢复: 外部任务提交——打断内部任务（内部让路——外部立即执行）
	// 内部任务挂起（SIGUSR1 存 checkpoint 退出）→ 重新入队（外部完恢复续接）
	if task.Priority >= PriorityExternal {
		s.pauseInternalsLocked()
	}
	heap.Push(&s.queue, task)
	log.Printf("🔄 总调度: 任务 %s 入队（%s——优先级 %d）", task.ID, task.Type, task.Priority)
	// v2.5.5 持久化完整版: 任务提交即落盘（queued 任务重启恢复——不再丢——2026-08-20）
	saveTasksLocked(s.queue, s.running, s.history)
	s.dispatchLocked()
	s.mu.Unlock()
}

// pauseInternalsLocked 打断所有运行中的内部任务（外部任务优先——Mr2109原理）
// 调用方必须持有 s.mu——内部任务发 SIGUSR1（CA 存 checkpoint 退出）——重新入队（外部完恢复）
func (s *MasterScheduler) pauseInternalsLocked() {
	for id, task := range s.running {
		if task.Priority < PriorityExternal {
			// 内部任务被外部打断——标记暂停 + 发信号（CA 收信号存 checkpoint 退出）
			log.Printf("⏸️ 总调度: 内部任务 %s 挂起（外部任务优先——SIGKILL 强制——2026-08-22 修单槽双跑）", id)
			if task.cmd != nil && task.cmd.Process != nil {
				// v2.5.5 修复（2026-08-22 Mr2109发现——单槽双跑）: SIGUSR1 优雅暂停 CA 可能不响应——
				// 旧进程还占槽+新任务又派发=两个同时跑——改 SIGKILL 强制（旧进程立即死——单槽保住）
				_ = task.cmd.Process.Kill()
			}
			// 挂起任务重新入队（外部完——恢复时重派——重新执行）
			task.Status = "queued"
			heap.Push(&s.queue, task)
			delete(s.running, id)
		}
	}
}

// dispatchLocked 派发循环（队列非空 + 并发未满 → 派发）
// 调用方必须持有 s.mu
func (s *MasterScheduler) dispatchLocked() {
	for len(s.queue) > 0 && len(s.running) < s.maxConcurrent {
		task := heap.Pop(&s.queue).(*Task)
		if task.Status == "cancelled" {
			continue
		}
		task.Status = "running"
		s.running[task.ID] = task
		log.Printf("🔄 总调度: 派发任务 %s（%s）", task.ID, task.Description)
		go s.runTask(task)
	}
}

// runTask 执行任务（spawn zerg-agent——CA 子调度接管执行）
// v2.5.5 打断/恢复: 记录 cmd 进程——PauseRunning 可发信号打断（SIGUSR1——CA 存 checkpoint 退出）
// v2.5.5 T2 worktree: 内部任务用 git worktree 隔离开发（独立目录——merge 回 main——任务 git 底座）
// v2.5.6 新流程: task.Flow=="zerg" → 程序定量驱动（状态机——内部调模型——不 spawn CA）
func (s *MasterScheduler) runTask(task *Task) {
	// v2.5.6 新流程（程序定量驱动 Agent——不 spawn CA——状态机驱动）
	if task.Flow == "zerg" {
		s.runZergFlow(task)
		return
	}
	// v2.5.5 T2: 内部任务建 worktree（隔离开发——任务 git 底座——Mr2109）
	// 外部任务不建（独立 git 管理——后续方案）
	worktreeDir := ""
	if task.Type == "internal" && task.Workdir != "" {
		// v2.5.5 修复（2026-08-24）: 分支名去重——任务 ID 已含 task- 前缀——不再加
		branch := sanitizeID(task.ID)
		wt, err := createWorktree(task.Workdir, branch)
		if err != nil {
			// v2.5.5 修复（2026-08-21 发现——重跑任务分支残留冲突）: 强删旧分支重试一次
			log.Printf("⚠️ 总调度: 任务 %s worktree 创建失败（清旧分支重试）: %v", task.ID, err)
			_ = exec.Command("git", "worktree", "remove", "--force", filepath.Join(task.Workdir, "zerg-wt", branch)).Run()
			_ = exec.Command("git", "branch", "-D", branch).Run()
			wt2, err2 := createWorktree(task.Workdir, branch)
			if err2 != nil {
				log.Printf("⚠️ 总调度: 任务 %s worktree 重试仍失败（继续原目录）: %v", task.ID, err2)
			} else {
				worktreeDir = wt2
				log.Printf("🌿 总调度: 任务 %s worktree 重试成功: %s", task.ID, wt2)
			}
		} else {
			worktreeDir = wt
			log.Printf("🌿 总调度: 任务 %s worktree 已建: %s", task.ID, wt)
		}
	}

	// v2.5.5 任务目录唯一化（2026-08-20 设计——Mr2109）: 任务目录 /tmp/zerg-tasks/<任务ID>/
	// 任务所有产出（代码/临时脚本/报告/日志）集中此目录——隔离+归档
	// 注: 不用 /var（Mac 普通用户无权限）——用 /tmp/zerg-tasks（可写——与 zerg-* 约定一致）
	taskDir := filepath.Join("/tmp/zerg-tasks", sanitizeID(task.ID))
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		log.Printf("⚠️ 总调度: 任务 %s 目录创建失败: %v", task.ID, err)
	}
	os.MkdirAll(filepath.Join(taskDir, "work"), 0o755)
	os.MkdirAll(filepath.Join(taskDir, "logs"), 0o755)
	// 审计文件（任务元数据）
	audit := map[string]interface{}{
		"task_id":   task.ID,
		"type":      task.Type,
		"model":     task.Model,
		"priority":  task.Priority,
		"created":   task.CreatedAt,
		"desc":      task.Description,
	}
	if ab, err := json.Marshal(audit); err == nil {
		os.WriteFile(filepath.Join(taskDir, "audit.jsonl"), append(ab, '\n'), 0o644)
	}
	log.Printf("📁 总调度: 任务 %s 目录已建: %s", task.ID, taskDir)

	// 构建 CA 命令: zerg-agent -model <模型> -task "<描述>" -workdir <工作区>
	// v2.5.5 修复（2026-08-21 Mr2109发现——735000 假成功）: 必须带 -json——jsonOut 模式退出码语义生效
	// 模型失败（ReasonModelError）退出码 3 → 主控 err → 自动重跑（不带 -json 时 CLI 分支退出 0——假成功）
	args := []string{"-json"}
	if task.Model != "" {
		args = append(args, "-model", task.Model)
	}
	// v2.5.5 任务目录唯一化（2026-08-20 设计——Mr2109）: workdir 指向任务目录/work
	// CA 在任务目录干活——报告/产物写任务目录内——天然隔离（不共享覆盖——治报告错位）
	workdir := filepath.Join(taskDir, "work")
	// 兼容: 任务需要原 workdir（内部任务 worktree 已建——用 worktree）
	if worktreeDir != "" {
		workdir = worktreeDir // worktree 隔离（git 任务）
	}
	if workdir != "" {
		args = append(args, "-workdir", workdir)
	}
	// v2.5.5 任务目录唯一化（2026-08-20 设计）: 任务描述附加报告路径提示（CA 写任务目录——不写共享）
	// 任务目录 = /tmp/zerg-tasks/<任务ID>/——报告写此目录（隔离——防覆盖）
	reportHint := fmt.Sprintf("\n\n【报告要求（硬性）】: 报告写到任务目录: %s/internal-task-report.md（绝对路径——用 write 工具写这个路径——不要写共享路径 /tmp/internal-task-report.md——这是硬性要求——不写=任务失败）", taskDir)
	taskDesc := task.Description + reportHint
	args = append(args, "-task", taskDesc)

	cmd := exec.Command(s.agentCmd, args...)
	// v2.5.5 P1-2 修复: 日志写固定位置（/tmp/zerg-ca-logs/）——防 worktree merge 后日志丢失
	// 默认写 workdir/.zerg/logs（worktree 内）——merge 删 worktree——跟踪数据断
	baseEnv := s.agentEnv
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	cmd.Env = append(baseEnv, "ZERG_LOG_DIR=/tmp/zerg-ca-logs")
	// v2.5.5 任务目录唯一化（2026-08-20 设计）: 注入任务目录——CA 写报告到任务目录（不共享）
	cmd.Env = append(cmd.Env, "ZERG_TASK_DIR="+taskDir)

	// 记录进程（打断用）
	s.mu.Lock()
	task.cmd = cmd
	s.mu.Unlock()

	out, err := cmd.CombinedOutput()
	s.mu.Lock()
	defer s.mu.Unlock()
	task.cmd = nil
	delete(s.running, task.ID)
	// v2.5.5 虫族UI: 完成/失败任务存历史（UI 列表显示——不丢）
	task.CompletedAt = time.Now()
	s.history[task.ID] = task
	// 历史上限（防无限增长——保留最近 100 个）
	if len(s.history) > 100 {
		oldest := ""
		var oldestTime time.Time
		for id, t := range s.history {
			if oldest == "" || t.CompletedAt.Before(oldestTime) {
				oldest = id
				oldestTime = t.CompletedAt
			}
		}
		if oldest != "" {
			delete(s.history, oldest)
		}
	}

	// 资源信任度（2026-08-21 Mr2109）: 任务完成→模型使用计数（成功）/失败→故障计数
	if task.Model != "" {
		if err != nil {
			resourceTrust.FaultResource("models", task.Model)
		} else {
			resourceTrust.UseResource("models", task.Model)
		}
	}

	if err != nil {
		task.Status = "failed"
		log.Printf("🔄 总调度: 任务 %s 失败: %v\n%s", task.ID, err, tail(string(out), 500))
		// v2.5.5 失败自动重跑（Mr2109 2026-08-20）: 失败任务自动重排队（同 ID——不新建记录）
		// 瞬时故障（熔断/网络——大概率恢复）——重跑 3 次仍失败才放执行完成标 failed
		task.RetryCount++
		if task.RetryCount <= 3 {
			task.Status = "queued"
			// v2.5.5 修复（2026-08-24 Mr2109）: 同一个任务不换模型——重跑保持原模型
			// 之前换模型（nextModelForRetry）导致: 每次换模型→X3 加载切换窗口→请求失败→熔断累计→X3 误熔断
			// 失败根因是系统（加载切换/超时）不是模型——换模型反而引入新失败点——保持原模型重跑（同配置同后端——恢复概率高）
			task.FailReason = fmt.Sprintf("第 %d 次失败自动重跑（保持原模型 %s——同 ID）", task.RetryCount, task.Model)
			heap.Push(&s.queue, task)
			log.Printf("🔁 总调度: 任务 %s 失败自动重跑（第 %d/3 次——保持原模型 %s——同 ID）", task.ID, task.RetryCount, task.Model)
		} else {
			task.FailReason = fmt.Sprintf("重跑 3 次仍失败: %v", err)
			log.Printf("❌ 总调度: 任务 %s 重跑 3 次仍失败——放执行完成（查原因）", task.ID)
		}
	} else {
		task.Status = "done"
		log.Printf("🔄 总调度: 任务 %s 完成", task.ID)
		// v2.5.5 任务git全生命周期（阶段2——设计-20260820）: 复查任务完成 → 决策（通过/打回）
		if task.Type == "review" {
			s.handleReviewDoneLocked(task)
			// 复查任务本身不进确定性验证（复查是判断——不是执行）
			saveTasksLocked(s.queue, s.running, s.history)
			if task.IssuePath != "" {
				updateIssueStatus(task.IssuePath, task.Status)
			}
			s.dispatchLocked()
			return
		}
		// v2.5.5 任务git全生命周期（阶段1——设计-20260820）: 确定性验证执行结果
		// 机器检查（git diff/报告/产物）——不过直接 failed（不浪费复查——治假完成）
		reportPath := FindTaskReport(workdir)
		verifyDir := worktreeDir
		if verifyDir == "" {
			verifyDir = workdir // 外部任务（无 worktree）用工作目录
		}
		// 2026-09-05 TaskContract: 有契约的任务按契约逐条核验（机器判据——治本）——无契约走旧启发式
		var vres *VerifyResult
		if contract := ParseContract(task.Description); contract != nil {
			vres = contract.Verify(worktreeDir, workdir)
			log.Printf("📋 总调度: 任务 %s 按契约验证（%d 文件/%d 命令/%d diff）",
				task.ID, len(contract.MustWriteFiles), len(contract.MustPassCmds), len(contract.MustDiffPaths))
		} else {
			vres = VerifyTaskOutput(verifyDir, reportPath, task.Description)
		}
		if !vres.Pass {
			task.Status = "failed"
			task.FailReason = "确定性验证不过: " + strings.Join(vres.Failures, "; ")
			log.Printf("❌ 总调度: 任务 %s 确定性验证不过（假完成拦截）: %s", task.ID, strings.Join(vres.Failures, "; "))
			// v2.5.5 失败自动重跑（Mr2109 2026-08-20）: 验证不过也自动重跑（可能瞬时模型问题——重跑恢复）
			task.RetryCount++
			if task.RetryCount <= 3 {
				task.Status = "queued"
				task.FailReason = fmt.Sprintf("第 %d 次验证不过自动重跑（上次: %s）", task.RetryCount, strings.Join(vres.Failures, "; "))
				heap.Push(&s.queue, task)
				log.Printf("🔁 总调度: 任务 %s 验证不过自动重跑（第 %d/3 次——同 ID）", task.ID, task.RetryCount)
			} else {
				task.FailReason = fmt.Sprintf("重跑 3 次仍验证不过: %s", strings.Join(vres.Failures, "; "))
				log.Printf("❌ 总调度: 任务 %s 重跑 3 次仍验证不过——放执行完成（查原因）", task.ID)
			}
		} else {
			log.Printf("✅ 总调度: 任务 %s 确定性验证通过（%d 项检查）", task.ID, len(vres.Checks))
			// v2.5.5 任务git全生命周期（阶段2——设计-20260820）: 自动派复查任务（跨家族模型 B）
			// 复查决定通过/打回——不是执行完直接 done（Mr2109: 成功与否由复查模型决定）
			s.submitReviewTaskLocked(task, reportPath, worktreeDir)
		}
	}
	// v2.5.5 P1-5 补: 任务状态落盘（重启后保留——UI 早上能看到所有任务）
	saveTasksLocked(s.queue, s.running, s.history)
	// v2.5.5 P1-3: 更新任务单状态（任务单文件——idle_detector 建的 docs/issues/*.md）
	if task.IssuePath != "" {
		updateIssueStatus(task.IssuePath, task.Status)
	}
	// v2.5.5 T2: worktree 完成后 merge 回 main + 清理（任务 git 底座）
	// v2.5.5 阶段2改: 执行完成不 merge——复查通过才 merge（复查任务 handleReviewDoneLocked 里做）
	// 但: 确定性验证不过/执行失败的任务——worktree 直接清理（不 merge——防脏分支堆积）
	if worktreeDir != "" && (task.Status == "failed" || task.FailReason != "") {
		// 失败任务——worktree 强清（无复查——不 merge）
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
		_ = exec.Command("git", "branch", "-D", "task-"+sanitizeID(task.ID)).Run()
		log.Printf("🗑️ 总调度: 任务 %s 失败——worktree 清理（不 merge）", task.ID)
	}
	// 完成后继续派发（队列里还有任务）
	s.dispatchLocked()
}

// updateIssueStatus 更新任务单文件状态（v2.5.5 P1-3: 总调度器完成/失败时同步任务单）
// 任务单格式: "- 状态: open" → "- 状态: done/failed"
func updateIssueStatus(issuePath, status string) {
	content, err := os.ReadFile(issuePath)
	if err != nil {
		log.Printf("⚠️ 总调度: 任务单更新失败（读取）: %v", err)
		return
	}
	newContent := regexp.MustCompile(`- 状态: \w+`).ReplaceAllString(string(content), "- 状态: "+status)
	if newContent != string(content) {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			log.Printf("⚠️ 总调度: 任务单更新失败（写入）: %v", err)
			return
		}
		log.Printf("📝 总调度: 任务单 %s 状态 → %s", filepath.Base(issuePath), status)
	}
}

// sanitizeID 任务 ID 清洗（git 分支名合法）
func sanitizeID(id string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return re.ReplaceAllString(id, "-")
}

// createWorktree 建 git worktree（repo 同级——独立目录 + 分支）
// v2.5.5 T2——任务 git 底座——内部任务隔离开发
func createWorktree(repoDir, branch string) (string, error) {
	wtDir := filepath.Join(filepath.Dir(repoDir), "zerg-wt", branch)
	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", wtDir, "-b", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("worktree add 失败: %v\n%s", err, tail(string(out), 300))
	}
	return wtDir, nil
}

// mergeWorktree 合并 worktree 分支回 main + 清理
// v2.5.5 T2——任务完成 merge 回 main——删 worktree + 分支
// v2.5.5 P1-1: merge 前检查报告文件（内部任务——没报告=假完成——不 merge）
func mergeWorktree(wtDir, branch string) error {
	repoDir := filepath.Dir(filepath.Dir(wtDir)) // zerg-wt/<branch> → repo
	// P1-1 验证: 报告文件存在（内部任务——防假完成）
	// v2.5.5 P1-6 修复: 只认任务报告名（internal-task-report.md 等）——不认 README.md（worktree 自带项目文件——误判）
	reportExists := false
	entries, _ := os.ReadDir(wtDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		// 任务报告文件名（明确匹配——不是任何 .md——README.md 不算）
		if name == "internal-task-report.md" || name == "internal-health-report.md" ||
			name == "report.md" || name == "result.md" ||
			(strings.HasSuffix(name, ".md") && strings.Contains(name, "report")) {
			reportExists = true
			break
		}
	}
	if !reportExists {
		return fmt.Errorf("worktree 无任务报告文件（P1-1 防假完成）——任务未产出报告——不 merge")
	}
	// 0. 先提交 worktree 未跟踪/未提交改动（任务产物——CA 可能只写没 commit）
	// v2.5.5 测试发现: CA 写文件没 commit——merge 丢产物 + remove 拒绝（未跟踪文件）
	addCmd := exec.Command("git", "-C", wtDir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("worktree add -A 失败: %v\n%s", err, tail(string(out), 300))
	}
	commitCmd := exec.Command("git", "-C", wtDir, "-c", "user.email=zerg@local", "-c", "user.name=Zerg AI", "commit", "-m", "task: 任务产物（worktree 自动提交）")
	if out, err := commitCmd.CombinedOutput(); err != nil {
		// 没有改动（commit 失败——nothing to commit）不算错——继续
		log.Printf("   （worktree 无新改动——跳过 commit——%s）\n", tail(string(out), 80))
	}
	// 1. 切回 main + merge 分支
	cmd := exec.Command("git", "-C", repoDir, "checkout", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout main 失败: %v\n%s", err, tail(string(out), 300))
	}
	cmd = exec.Command("git", "-C", repoDir, "merge", "--no-edit", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s 失败: %v\n%s", branch, err, tail(string(out), 300))
	}
	// 2. 删 worktree + 分支
	cmd = exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", wtDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("worktree remove 失败: %v\n%s", err, tail(string(out), 300))
	}
	cmd = exec.Command("git", "-C", repoDir, "branch", "-d", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("branch -d 失败: %v\n%s", err, tail(string(out), 300))
	}
	return nil
}

// PauseRunning 挂起运行中任务（外部高优来——打断内部）——v2.5.5 已并入 Submit（pauseInternalsLocked）
// 保留兼容（外部显式调用）
func (s *MasterScheduler) PauseRunning(exceptID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pauseInternalsLocked()
}

// TerminateTask 终止执行中任务（右键功能——2026-08-22 Mr2109——kill CA 进程——任务标 failed）
func (s *MasterScheduler) TerminateTask(taskID string) error {
	s.mu.Lock()
	t, ok := s.running[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("任务 %s 不在运行中（仅 running 可终止）", taskID)
	}
	t.Status = "failed"
	t.FailReason = "用户手动终止（右键）"
	// v2.5.6 修复（2026-08-28 实证）: 终止后必须删除 running + 派发下一个——
	// 之前只标 failed 不删 running → 调度器认为并发满 → 排队任务永不派发（任务队列卡死）
	delete(s.running, taskID)
	s.history[taskID] = t
	saveTasksLocked(s.queue, s.running, s.history, s.waiting)
	s.dispatchLocked()
	s.mu.Unlock()
	// 杀 CA 进程（SIGKILL——强制终止——2026-08-22 Mr2109）
	s.killTaskProcess(taskID)
	log.Printf("🛑 任务 %s 被用户终止\n", taskID)
	return nil
}

// killTaskProcess 杀任务 CA 进程（SIGKILL——强制终止）
func (s *MasterScheduler) killTaskProcess(taskID string) {
	s.mu.Lock()
	t, ok := s.running[taskID]
	s.mu.Unlock()
	if ok && t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
}

// RequeueTask 重回队列（running→queued——右键功能——2026-08-22 Mr2109）
// 终止当前执行 → 任务重新排队（等下次执行——同 ID）
func (s *MasterScheduler) RequeueTask(taskID string) error {
	s.mu.Lock()
	t, ok := s.running[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("任务 %s 不在运行中（仅 running 可重回队列）", taskID)
	}
	t.Status = "queued"
	t.FailReason = ""
	s.queue = append(s.queue, t)
	delete(s.running, taskID)
	s.mu.Unlock()
	// 杀旧 CA 进程（停止当前执行——任务重回队列等下次）
	s.killTaskProcess(taskID)
	log.Printf("🔁 任务 %s 重回队列（用户操作）\n", taskID)
	return nil
}

// RunningCount 运行中任务数（空闲检测用——外部任务队列长度）
func (s *MasterScheduler) RunningCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running)
}

// QueueLen 队列长度
func (s *MasterScheduler) QueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// PauseTask 暂停/继续排队任务（queued→paused / paused→queued——2026-08-21 Mr2109右键功能）
func (s *MasterScheduler) PauseTask(taskID string, pause bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.queue {
		if t.ID == taskID {
			if pause && t.Status == "queued" {
				t.Status = "paused"
				log.Printf("⏸️ 总调度: 任务 %s 暂停", taskID)
				return nil
			}
			if !pause && t.Status == "paused" {
				t.Status = "queued"
				log.Printf("▶️ 总调度: 任务 %s 继续", taskID)
				return nil
			}
			return fmt.Errorf("任务 %s 状态 %s 不可%s", taskID, t.Status, map[bool]string{true: "暂停", false: "继续"}[pause])
		}
	}
	return fmt.Errorf("任务 %s 不在队列", taskID)
}

// RetryTask 重跑任务（failed→queued——同 ID——RetryCount 重置——2026-08-21 Mr2109右键功能）
func (s *MasterScheduler) RetryTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.history {
		if t.ID == taskID && t.Status == "failed" {
			t.Status = "queued"
			t.RetryCount = 0
			t.FailReason = ""
			heap.Push(&s.queue, t)
			log.Printf("🔁 总调度: 任务 %s 手动重跑（右键——同 ID）", taskID)
			return nil
		}
	}
	return fmt.Errorf("任务 %s 未找到或不可重跑", taskID)
}

// MoveTask 重排任务（置顶/置底/上移/下移——队列内——2026-08-21 Mr2109右键功能）
// action: top/bottom/up/down
func (s *MasterScheduler) MoveTask(taskID, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, t := range s.queue {
		if t.ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("任务 %s 不在队列（仅 queued 可重排）", taskID)
	}
	q := s.queue
	switch action {
	case "top":
		q[0], q[idx] = q[idx], q[0]
		heap.Fix(&s.queue, 0)
		heap.Fix(&s.queue, idx)
	case "bottom":
		q[len(q)-1], q[idx] = q[idx], q[len(q)-1]
		heap.Fix(&s.queue, len(q)-1)
		heap.Fix(&s.queue, idx)
	case "up":
		if idx > 0 {
			q[idx-1], q[idx] = q[idx], q[idx-1]
			heap.Fix(&s.queue, idx-1)
			heap.Fix(&s.queue, idx)
		}
	case "down":
		if idx < len(q)-1 {
			q[idx+1], q[idx] = q[idx], q[idx+1]
			heap.Fix(&s.queue, idx+1)
			heap.Fix(&s.queue, idx)
		}
	default:
		return fmt.Errorf("未知操作: %s", action)
	}
	log.Printf("↕️ 总调度: 任务 %s 重排（%s）", taskID, action)
	return nil
}

// DeleteTask 删除排队任务（queued 移除——done/failed 保留记录——2026-08-21 Mr2109右键功能）
func (s *MasterScheduler) DeleteTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.queue {
		if t.ID == taskID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			heap.Init(&s.queue)
			log.Printf("🗑️ 总调度: 任务 %s 删除（右键）", taskID)
			return nil
		}
	}
	return fmt.Errorf("任务 %s 不在队列（仅 queued 可删除）", taskID)
}

// copyReportToTaskDir 复制 worktree 报告到任务目录（merge 后报告保留——2026-08-21 Mr2109发现修复）
// worktree 删除后报告丢——详情读不到——复制 internal-task-report.md/review-report.md 到任务目录
func copyReportToTaskDir(taskID, wtDir string) {
	if wtDir == "" {
		return
	}
	taskDir := filepath.Join("/tmp/zerg-tasks", sanitizeID(taskID))
	os.MkdirAll(taskDir, 0o755)
	// 复制常见报告文件（worktree 里 → 任务目录）
	for _, name := range []string{"internal-task-report.md", "review-report.md", "report.md", "result.md"} {
		src := filepath.Join(wtDir, name)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(taskDir, name)
			data, _ := os.ReadFile(src)
			os.WriteFile(dst, data, 0o644)
		}
	}
}

// nextModelForRetry 失败重跑换模型（找原模型在池里的下一个——跨模型重试）
// v2.5.5 修复（2026-08-21 Mr2109）: 失败=模型可能有问题——重跑不固定原模型
func nextModelForRetry(current string) string {
	// 执行模型池（与 main.go internalModelPool 一致——8 模型）
	pool := []string{"Qwen3.8-27B", "example-35b-v2", "gemma-4-26B", "GLM-4.7-Flash", "example-8b-quant", "deepseek-v4-flash", "example-30b", "Nemotron-3.5-Lightning"}
	for i, m := range pool {
		if m == current {
			return pool[(i+1)%len(pool)] // 下一个
		}
	}
	return pool[0] // 不在池——用第一个
}

// tail 取输出末尾（日志用）
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// String 便于打印
func (t *Task) String() string {
	return fmt.Sprintf("%s[%s/%s]", t.ID, t.Type, t.Status)
}

// CleanupStaleWorktrees 启动清理残留任务 worktree（P1-5 治本）
// 扫描 git worktree list——所有 task-* 分支的 worktree（不在当前运行任务列表中的=残留）
// 有报告 → merge 回 main（保留产物）；无报告 → 强制清理（任务已死）
// 返回清理数量
func CleanupStaleWorktrees(repoDir string) int {
	cmd := exec.Command("git", "-C", repoDir, "worktree", "list", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("  worktree list 失败: %v\n", err)
		return 0
	}
	lines := strings.Split(string(out), "\n")
	cleaned := 0
	curWorktree := ""
	curBranch := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if curWorktree != "" && strings.Contains(curBranch, "task-") {
				if cleanupOneStaleWorktree(repoDir, curWorktree, curBranch) {
					cleaned++
				}
			}
			curWorktree = ""
			curBranch = ""
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			curWorktree = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			curBranch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	if curWorktree != "" && strings.Contains(curBranch, "task-") {
		if cleanupOneStaleWorktree(repoDir, curWorktree, curBranch) {
			cleaned++
		}
	}
	return cleaned
}

// cleanupOneStaleWorktree 清理单个残留 worktree（有报告 merge——无报告强清）
func cleanupOneStaleWorktree(repoDir, wtDir, branch string) bool {
	reportExists := false
	if entries, err := os.ReadDir(wtDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if name == "internal-task-report.md" || name == "internal-health-report.md" ||
				name == "report.md" || name == "result.md" ||
				(strings.HasSuffix(name, ".md") && strings.Contains(name, "report")) {
				reportExists = true
				break
			}
		}
	}
	if reportExists {
		if err := mergeWorktree(wtDir, branch); err != nil {
			log.Printf("  残留 %s merge 失败——强制清理: %v\n", branch, err)
			exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", wtDir).Run()
			exec.Command("git", "-C", repoDir, "branch", "-D", branch).Run()
			return true
		}
		log.Printf("  残留 %s 已 merge 回 main（保留任务产物）\n", branch)
		return true
	}
	exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", wtDir).Run()
	exec.Command("git", "-C", repoDir, "branch", "-D", branch).Run()
	log.Printf("  残留 %s 无报告——已清理（任务已死）\n", branch)
	return true
}
