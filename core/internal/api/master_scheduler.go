package api

// ============ v2.5.5 T3 主控总调度器——骨架 ============
// 两级调度（Mr2109原理）:
//   主控总调度: 管所有内部任务 + 接入的外部任务（全局决策/派发）
//   CA 子调度: 管分派任务的执行（agent 侧已有 scheduler）
// 借鉴: Kubernetes 优先级/抢占、LowPriority LLM顾问+策略法律、Zabbix 优先级堆

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// 任务态字面量（**代码面全集——七态**）。
// 2026-10-03 GAP-20261003-04 刀1（T-03）: ① 补登 waiting_retry（旧注释只列五态、过期）；
// ② 新增终态 cancelled。终态由**派生谓词** IsTerminal 判（done|failed|cancelled）——不新增合并态。
// 注: 持久化四分区的白名单与命令树登记面是另两面；三面交叉口径见设计 §O-23。
const (
	StatusQueued       = "queued"        // 排队中（可取消）
	StatusRunning      = "running"       // 运行中（不可取消——先 terminate）
	StatusPaused       = "paused"        // 已暂停（可取消）
	StatusDone         = "done"          // 终态: 已完成
	StatusFailed       = "failed"        // 终态: 失败
	StatusWaitingRetry = "waiting_retry" // 等环境恢复（可恢复——非终态）
	StatusCancelled    = "cancelled"     // 终态: 已撤（本刀新增）
)

// closedReasonSet 结案理由闭集（§11.5 定稿: 小闭集必填——superseded/obsolete/duplicate/abandoned + other 兜底）。
// ★ 单一常量枚举（§11.5③）；落盘存字符串值不存序号（§11.5①）；读取端遇未知值原样保留、绝不 panic（§11.5②）。
var closedReasonSet = []string{"superseded", "obsolete", "duplicate", "abandoned", "other"}

// isClosedReason 判定结案理由是否在闭集内（CancelTask 的 ①校验 用它）。
func isClosedReason(reason string) bool {
	for _, r := range closedReasonSet {
		if r == reason {
			return true
		}
	}
	return false
}

// 撤下/结案**结局闭集**（2026-10-03 GAP-20261003-04 刀1 · 设计 §11.2 幂等三成因）。
//
// ★ 铁律（§11.2 · §4.4）: **退码只回答「命令能不能执行」，不回答「业务属于哪种状态」** ——
// 三种结局（真取消 / 已取消幂等重放 / 已是终态只打标）都返 nil（退码 0），靠本返回值区分。
// 逐字对齐 §11.2 stdout 结构化字段: cancelled=true / already_cancelled=true / already_terminal=true。
const (
	CancelOutcomeCancelled        = "cancelled"         // 真取消: queued/paused → cancelled（§11.2 cancelled=true）
	CancelOutcomeAlreadyCancelled = "already_cancelled" // 已 cancelled 幂等重放 no-op（§11.2 already_cancelled=true）
	CancelOutcomeAlreadyTerminal  = "already_terminal"  // 已 done/failed 只打标、status 未变（§11.2 already_terminal=true + 原终态）
)

// closedByLocal 结案主体（单机单用户 ⇒ 恒为同一主体——§11.5「closed_by 可省或留 nullable 位」）。
const closedByLocal = "local"

// Task 总调度任务
type Task struct {
	ID          string       `json:"id"`                    // 任务 ID（唯一）
	Description string       `json:"description"`           // 任务描述
	Priority    TaskPriority `json:"priority"`              // 优先级（外部高/内部低）
	Type        string       `json:"type"`                  // internal/external
	Status      string       `json:"status"`                // queued/running/paused/done/failed/waiting_retry/cancelled（终态 done/failed/cancelled——见 IsTerminal）
	FailReason  string       `json:"fail_reason,omitempty"` // 失败原因（确定性验证/执行失败——UI 显示）
	// ============ 正交标记面（2026-10-03 GAP-20261003-04 刀1 · T-06 · 设计 §O-9/§11.1）============
	// 「态」与「结案标记」两层: 四条同生同灭——非空即已结案 · 清空即重开 · 覆盖式（非 write-once）。
	// 对 done/failed/cancelled 打标【不动 status】；对 queued/paused 先由 CancelTask 改态为 cancelled 再打标。
	// 单机单用户 ⇒ closed_by 恒 closedByLocal（§11.5「可省或留 nullable 位」）。
	ClosedAt     *time.Time `json:"closed_at,omitempty"`     // 结案时间（RFC3339 · UTC · 带 Z）
	ClosedBy     string     `json:"closed_by,omitempty"`     // 结案主体（单机单用户——恒 local）
	ClosedReason string     `json:"closed_reason,omitempty"` // 结案理由（闭集: superseded/obsolete/duplicate/abandoned/other）
	ClosedNote   string     `json:"closed_note,omitempty"`   // 结案备注（自由文本——可空）
	RefTaskID    string     `json:"ref_task_id,omitempty"`   // 关联任务（复查任务 → 执行任务）
	// v2.5.7 对话→任务集成: 来源对话/消息关联（UI"派任务"——记录任务从哪个对话发起）
	ParentSessionID string `json:"parent_session_id,omitempty"`
	ParentMessageID string `json:"parent_message_id,omitempty"`
	RefWorktree     string `json:"ref_worktree,omitempty"` // 关联 worktree（复查用）
	ReplanCount     int    `json:"replan_count,omitempty"` // 重做次数（阶段3 Replan 循环——上限 3）
	ReviewCount     int    `json:"review_count,omitempty"` // 复查次数（阶段3——上限 5）
	RetryCount      int    `json:"retry_count,omitempty"`  // 自动重跑次数（失败自动重跑——上限 3——Mr2109 2026-08-20）
	Model           string `json:"model"`                  // 派发用模型
	Workdir         string `json:"workdir"`                // 工作区
	// EggMaxS 单卵时限（秒 · GAP-20260926-201 全通路）：CLI `--timeout` ⇒ `/api/tasks` 体 ⇒
	// 落进任务（主控**透传** · ✗ 不丢）。空 = 未设（回落子端 env `ZERG_EGG_MAX_S`）。
	EggMaxS           string    `json:"egg_max_s,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	CompletedAt       time.Time `json:"completed_at"`                  // v2.5.5 虫族UI: 完成时间（耗时计算）
	Checkpoint        string    `json:"checkpoint"`                    // 检查点（打断恢复用——.zerg/logs/<ID>/checkpoint.json）
	IssuePath         string    `json:"issue_path"`                    // v2.5.5 P1-3: 任务单路径（idle_detector 建的任务区 *.md——落点见 statepath.IssuesDir()；完成时更新状态）
	Flow              string    `json:"flow,omitempty"`                // v2.5.6: 执行流程（"zerg"=程序定量驱动新流程——空=旧 CA 流程）
	Machine           string    `json:"machine,omitempty"`             // v2.5.6: 执行设备（模型所在机器——handlers 按 modelMachineMap 推算——UI 详情显示）
	SkillKey          string    `json:"skill_key,omitempty"`           // v2.5.6: skill 归属 key（内部任务=def.ID——独属 skill；空=外部任务按类型共享）
	ExtraEnv          []string  `json:"extra_env,omitempty"`           // S6: 任务级 env 透传（rework 续作——ZERG_TASK_DIR/ZERG_REVIEW_NOTE）
	ReviewFailedCount int       `json:"review_failed_count,omitempty"` // S7: 复查自身失败重派次数（上限 2）
	// ============ B 项③ 片（单子）最小 schema（2026-09-18）============
	// 术语（设计-任务模块骨架 v1.0 §0.2）: 任务(mission_id) → 片/单子(slice_id) → 步。
	// 片 = 单子（同一实体）；本组字段**只在声明了片字段的任务上**生效——全 omitempty
	// ⇒ 现有任务序列化形态逐字节不变（零回归）。校验/环检测/悬空依赖见 slice_schema.go。
	SliceID    string    `json:"slice_id,omitempty"`   // 片真源 id（空 ⇒ 挂板校验拒绝）
	DependsOn  []string  `json:"depends_on,omitempty"` // 该片依赖的其它片 id（有环 / 悬空 ⇒ 拒绝入队）
	Owner      string    `json:"owner,omitempty"`      // 片归属者（本轮不参与判定——只随片记录）
	Acceptance *[]string `json:"acceptance,omitempty"` // 验收判据（**必须显式声明**: nil=没写（拒）；&[]=显式空（允许））
	// ---- B 项 B5（2026-09-18）: 黑板三字段 —— Lead / 分工方案 / 等确认闸（出处与命名口径见 slice_schema.go 头部）----
	Lead         *string       `json:"lead,omitempty"`          // Lead（负责的模型）: 声明了就必须非空白（空白 ⇒ 拒）
	Members      []SliceMember `json:"members,omitempty"`       // 分工方案: 每个成员干什么（非空 ⇒ 必须过「等确认闸」）
	PlanApproved *bool         `json:"plan_approved,omitempty"` // 等确认闸: nil=没写（=未确认 ⇒ 拒）；&false 显式未确认 ⇒ 拒；&true ⇒ 放
	// ============ R7 缺口 GAP-20260929-26: 领取 / 心跳（claim / heartbeat）============
	// 「谁、何时、从队里把这条取走跑」的现读面: 派发（dispatchDecideLocked）即写入。
	// 空 = 尚未被领取（旧件零值 ⇒ 序列化形态不变）。心跳在 runTask 起跑时刷新，
	// 供 ReclaimDeadLetters 判「进程已丢 + 心跳超时」的死信。
	ClaimedAt   *time.Time `json:"claimed_at,omitempty"`   // 领取时刻（派发即写）
	ClaimedBy   string     `json:"claimed_by,omitempty"`   // 领取者（本机主控单槽 ⇒ "local"）
	HeartbeatAt *time.Time `json:"heartbeat_at,omitempty"` // 最近心跳（起跑/存活期刷新）
	// WaitingSince 进入 waiting_retry 的时刻（30 分钟等待窗的**唯一**计时源）。
	// ★ 不许拿 CompletedAt 顶替: 修后 CompletedAt 只在终态写、回队/挂起即清零，
	//   拿它计时就会「一清零 ⇒ 立刻判超 30 分钟 ⇒ 整批弃件」。旧件无本字段 ⇒ 回退读 CompletedAt（见 sinceWaiting）。
	WaitingSince *time.Time `json:"waiting_since,omitempty"`
	cmd          *exec.Cmd  // 运行中的 CA 进程（打断发信号用——非导出）
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
	mu             sync.Mutex
	queue          TaskQueue             // 优先级队列（外部高——内部低）
	running        map[string]*Task      // 运行中任务
	history        map[string]*Task      // v2.5.5 虫族UI: 已完成/失败任务历史（UI 列表显示——不丢）
	waiting        map[string]*Task      // v2.5.6 故障自愈: 环境故障挂起任务（waiting_retry——机器恢复自动重派）
	maxConcurrent  int                   // 最大并发（**有效值** —— paused 时为 0；见 applySchedLocked）
	configuredMax  int                   // 用户设定的并发上限（持久化真源 —— pause 不改它）
	dispatchPaused bool                  // 暂停派活（一等开关 · 持久化 —— 与 maxConcurrent=0 等价但记住原值）
	agentCmd       string                // CA 启动命令（zerg-agent 路径）
	agentEnv       []string              // CA 环境变量（网关地址/token）
	taskDirForCall string                // v2.5.6: 当前 zerg 流程任务目录（write_file 工具写文件用）
	pingMu         sync.Mutex            // v2.5.6 故障自愈: 模型探活缓存锁
	pingCache      map[string]pingResult // v2.5.6 模型探活结果缓存（10s——排队任务不重复 ping）
	store          StoreReader           // v2.5.6 ping 三级漏斗: 快照读取（第1级——快照优先——0ms）
	started        bool                  // 2026-09-13 并发修复: 是否已显式 Start（构造期不执行任务——防重复启动后台循环）
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
		configuredMax: maxConcurrent,
		agentCmd:      agentCmd,
	}
	if len(store) > 0 {
		s.store = store[0]
	}
	heap.Init(&s.queue)
	// v2.5.5 P1-5 补: 恢复历史任务（重启后保留——UI 显示所有任务）
	// 2026-09-13 并发修复: 构造期只恢复状态（history/queue/waiting）——不派发、不启动后台 goroutine。
	// 之前构造期就 dispatchLocked 排队任务（tasks_persist.go）并 startRecoveryLoop → 实例在返回给调用方
	// 之前已有 goroutine 在改自己的 queue/running/history（无所有权交接、无停止手段），调用方把返回值
	// 当作“刚构造好的静止对象”访问这些字段即为数据竞争。何时开始执行改由调用方显式 Start() 决定。
	s.loadPersistedHistory()
	return s
}

// Start 显式启动调度器（所有权交接——调用方装配完成后调用一次）
// 2026-09-13 并发修复: 构造期不再执行任务/起后台循环，改由本方法一次性接手:
//  1. 派发恢复出来的排队任务（重启续跑——之前是构造期自动做）
//  2. 启动故障自愈恢复循环（周期扫 waiting_retry——机器恢复自动重派）
//
// 幂等: 重复调用只生效一次（防恢复循环重复起 goroutine）。
func (s *MasterScheduler) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	// R7 GAP-20260927-210 ②: 载入持久化的并发上限/暂停位（首跑无件 ⇒ 保持构造参数给的默认）。
	s.loadSchedConfigLocked()
	restored := s.queue.Len()
	s.dispatchLocked()
	s.mu.Unlock()
	if restored > 0 {
		log.Printf("▶️ scheduler: start — resuming %d restored queued task(s)", restored)
	}
	s.startRecoveryLoop()
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
			// R7 GAP-20260929-26 ③: 死信回收 —— 先给存活进程刷心跳，再收「进程已丢 + 心跳超时」的件。
			s.heartbeatSweep()
			s.ReclaimDeadLetters()
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
	var expired []*Task    // 超上限——降级 failed
	var candidates []*Task // 待 ping 探测
	for _, task := range s.waiting {
		if now.Sub(sinceWaiting(task, now)) > maxWaitRetry {
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

	// 回锁——处理结果（★ 快照是锁外拍的：只处理「此刻仍在等待分区」的件，见本函数注释）
	s.mu.Lock()
	s.applyWaitingRecoveryLocked(expired, recovered)
	s.mu.Unlock()
}

// applyWaitingRecoveryLocked 恢复循环第二阶段（调用方持锁）: 处理锁外 ping 得到的快照结果。
//
// ★ 2026-10-22 GAP-20261022-02: `expired`/`recovered` 是**锁外** ping 期间拍的快照，而
// ping 会阻塞数秒——这期间用户完全可能对该件下 cancel（撤销面现已收 waiting_retry）。
// 照旧无条件处理会出两种坏结局:
//   - recovered 一侧: 对已撤件 applyStatus(queued) + heap.Push ⇒ **已撤件复活回队列**
//     并被重新派发（正是「取消后又在队」这一族）；
//   - expired 一侧: 把 cancelled 覆写成 failed ⇒ 丢掉「退场」语义（T-05「绝不覆写终态」）。
//
// 故两个循环都只处理「此刻仍在 s.waiting」的件；不在 = 快照过期 ⇒ 无动作（不覆写、不回队）。
func (s *MasterScheduler) applyWaitingRecoveryLocked(expired, recovered []*Task) {
	for _, task := range expired {
		if _, stillWaiting := s.waiting[task.ID]; !stillWaiting {
			continue // 快照过期: 期间已被撤销/结案/摘出 —— 不得覆写其终态
		}
		task.applyStatus(StatusFailed, time.Now())
		task.FailReason = "环境故障持续超过 30 分钟——自动放弃（需人工介入）: " + task.FailReason
		delete(s.waiting, task.ID)
		s.history[task.ID] = task
		log.Printf("❌ scheduler: task %s environment-failure timeout — abandoned (>30min, manual intervention needed)", task.ID)
	}
	for _, task := range recovered {
		if _, stillWaiting := s.waiting[task.ID]; !stillWaiting {
			continue // 快照过期: 期间已被撤销 —— 不得回队（回队 = 复活并重新派发）
		}
		delete(s.waiting, task.ID)
		task.applyStatus(StatusQueued, time.Now())
		heap.Push(&s.queue, task)
		log.Printf("♻️ scheduler: task %s environment recovered — re-queued (awaiting re-dispatch)", task.ID)
	}
	// 有恢复——持久化 + 派发
	if len(s.queue) > 0 {
		if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
			log.Printf("⚠️ 任务落盘失败: %v\n", err)
		}
		s.dispatchLocked()
	}
}

// maxWaitRetry 环境故障最大等待时间（30 分钟——超限降级 failed）
const maxWaitRetry = 30 * time.Minute

// isBackendExit — 2026-09-09(诊断 R3): agent exit 3 = ReasonModelError(模型/后端类失败)
// 与代码注释约定一致: -json 模式退出码语义(模型失败 exit 3 → 主控 err → 自动重跑)
func isBackendExit(err error) bool {
	if err == nil {
		return false
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode() == 3
	}
	return false
}

// Submit 提交任务（外部/内部都走这——优先级排队）
func (s *MasterScheduler) Submit(task *Task) {
	s.mu.Lock()
	// ---- B 项③ 挂板校验（2026-09-18）: **只对声明了片字段的任务生效** ----
	// 未声明片字段（= 现有全部任务）⇒ 闸函数第一行返回 nil ⇒ 下面的路径与改动前逐字节一致（零回归）。
	// 拒绝时**不入队**；且**不在锁内做落盘 I/O**（观测事件在解锁后打——与 recoverWaitingLocked 同规）。
	if serr := validateSliceMountLocked(task, s.sliceBoardLocked()); serr != nil {
		s.mu.Unlock()
		emitSliceMount(task, serr)
		return // 拒绝入队: 缺 slice_id / 缺 acceptance 声明 / lead 空白 / 分工方案缺项 / 等确认闸未过 / depends_on 环 / 悬空依赖
	}
	// 是不是片（过了闸 ⇒ 声明了且 slice_id 非空）——决定发不发那条放行观测事件
	isSlice := task != nil && task.SliceID != ""
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
	log.Printf("🔄 scheduler: task %s queued (%s — priority %d)", task.ID, task.Type, task.Priority)
	// v2.5.5 持久化完整版: 任务提交即落盘（queued 任务重启恢复——不再丢——2026-08-20）
	if err := saveTasksLocked(s.queue, s.running, s.history); err != nil {
		log.Printf("⚠️ 任务落盘失败: %v\n", err)
	}
	s.dispatchLocked()
	s.mu.Unlock()
	// ---- B 项③ 观测: 片放行入队（**锁外**——落盘 I/O 不进锁；非片任务零事件、零开销）----
	if isSlice {
		emitSliceMount(task, nil)
	}
}

// SubmitSlice 挂板提交（片专用入口；返回**可行动错误**——非 HTTP 调用方用它）。
//
// 与 Submit 的关系: 两者**共用同一道闸**（validateSliceMountLocked），判定只有一处实现。
// 本方法把同一判定提前到「调用方看得见」的位置（拿到 *SliceValidationError，而不是「静默没入队」），
// 供 HTTP 侧回 400 + 机器可判错误码；Submit 内那道闸仍是最后防线（兜住其它入队调用方）。
//
// 未声明片字段的任务 ⇒ 与直接调 Submit 等价（SubmitSlice 对它们不加任何判定）。
func (s *MasterScheduler) SubmitSlice(task *Task) error {
	s.mu.Lock()
	serr := validateSliceMountLocked(task, s.sliceBoardLocked())
	s.mu.Unlock()
	if serr != nil {
		emitSliceMount(task, serr) // 拒绝放行 —— 每次挂板尝试恰好一条事件
		return serr
	}
	s.Submit(task) // 放行（放行事件由 Submit 打——同一次尝试只有一条）
	return nil
}

// validateSliceMountLocked 挂板闸（**调用方持锁**；唯一实现点，Submit / SubmitSlice 都走它）。
// 未声明片字段 ⇒ 第一行就返回 nil（不进任何图计算、不落任何事件）——这就是「现有任务零回归」的机制。
func validateSliceMountLocked(task *Task, board SliceBoard) *SliceValidationError {
	in := SliceInputFromTask(task)
	if !in.Declared() {
		return nil
	}
	return ValidateSliceMount(in, board)
}

// sliceBoardLocked 挂板视图（**调用方持锁**）: 板上已挂片的 slice_id → depends_on。
// 组成 = queue + running + waiting + history 四处里声明过 slice_id 的片。
// 为什么含 history: 本轮只判「依赖的片是否**存在**」与「依赖图是否**无环**」，
// 不判完成度（完成度属状态机——本轮 ✗ 不实现）。去掉 history 会让「依赖的片已跑完」变成悬空误拒。
func (s *MasterScheduler) sliceBoardLocked() SliceBoard {
	board := make(SliceBoard)
	add := func(t *Task) {
		if t != nil && t.SliceID != "" {
			board[t.SliceID] = append([]string(nil), t.DependsOn...)
		}
	}
	for _, t := range s.queue {
		add(t)
	}
	for _, t := range s.running {
		add(t)
	}
	for _, t := range s.waiting {
		add(t)
	}
	for _, t := range s.history {
		add(t)
	}
	return board
}

// SliceBoard 挂板视图（外部只读查询——持锁取副本，调用方可安全遍历）。
func (s *MasterScheduler) SliceBoard() SliceBoard {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sliceBoardLocked()
}

// pauseInternalsLocked 打断所有运行中的内部任务（外部任务优先——Mr2109原理）
// 调用方必须持有 s.mu——内部任务发 SIGUSR1（CA 存 checkpoint 退出）——重新入队（外部完恢复）
func (s *MasterScheduler) pauseInternalsLocked() {
	for id, task := range s.running {
		if task.Priority < PriorityExternal {
			// 内部任务被外部打断——标记暂停 + 发信号（CA 收信号存 checkpoint 退出）
			log.Printf("⏸️ scheduler: internal task %s suspended (external tasks first — forced SIGKILL — 2026-08-22 single-slot double-run fix)", id)
			if task.cmd != nil && task.cmd.Process != nil {
				// v2.5.5 修复（2026-08-22 Mr2109发现——单槽双跑）: SIGUSR1 优雅暂停 CA 可能不响应——
				// 旧进程还占槽+新任务又派发=两个同时跑——改 SIGKILL 强制（旧进程立即死——单槽保住）
				_ = task.cmd.Process.Kill()
			}
			// 挂起任务重新入队（外部完——恢复时重派——重新执行）
			task.applyStatus(StatusQueued, time.Now())
			heap.Push(&s.queue, task)
			delete(s.running, id)
		}
	}
}

// dispatchDecideProbe 测试探针（nil = 生产零开销）: dispatchDecideLocked 每次**进入**时触发。
// 供 T-04/F-2「save 失败 ⇒ dispatch 未被调用」的成对读数直接观测派发决策函数是否被进入（禁放宽断言）。
var dispatchDecideProbe func()

// saveTasksForCancel 测试缝（默认 = 真原子写 saveTasksLocked）: CancelTask 的**唯一持久点**经它落盘，
// 供 T-04/F-2 注入 save 失败（mock saveTasksLocked 返 error）。生产路径等价于直接调 saveTasksLocked。
var saveTasksForCancel = saveTasksLocked

// dispatchLocked 派发循环（队列非空 + 并发未满 → 派发）。
// 调用方必须持有 s.mu。
// 2026-10-03 GAP-20261003-04 刀1（设计 §11.4 六步）: 拆成「锁内纯内存决策 + 锁外真拉起」两半——
// spawn 是慢 IO，必须在锁外（CERT LCK09-J: 持锁不做阻塞 IO）。
func (s *MasterScheduler) dispatchLocked() {
	for _, task := range s.dispatchDecideLocked() {
		go s.runTask(task)
	}
}

// dispatchDecideLocked 派发的**纯内存决策**（调用方持锁）: 弹出可派发任务 → 标 running →
// 记入 running 容器 → 返回待拉起清单。**不 spawn**（§11.4 步骤⑤: 只占位）。
// 已 cancelled 的件（若因任何路径余留在 queue）跳过不派发（旧有防线，保留——2026-09-04）。
func (s *MasterScheduler) dispatchDecideLocked() []*Task {
	if dispatchDecideProbe != nil {
		dispatchDecideProbe()
	}
	var pending []*Task
	now := time.Now()
	for len(s.queue) > 0 && len(s.running) < s.maxConcurrent {
		task := heap.Pop(&s.queue).(*Task)
		if task.Status == StatusCancelled {
			continue
		}
		task.Status = StatusRunning
		// R7 GAP-20260929-26 ①: 领取面 —— 派发即盖「谁/何时取走 + 心跳起算」。
		claimAt := now
		task.ClaimedAt = &claimAt
		task.ClaimedBy = "local"
		task.HeartbeatAt = &claimAt
		s.running[task.ID] = task
		log.Printf("🔄 scheduler: dispatching task %s (%s)", task.ID, task.Description)
		pending = append(pending, task)
	}
	return pending
}

// caLogEnv — 派发 CA 任务时注入的日志根 env（v2.5.5 P1-2 修复: 日志写固定位置防 worktree merge 后丢失）。
//
// 2026-09-18 硬编码治理批 D: 原先直接写死字面量 "ZERG_LOG_DIR=/tmp/zerg-ca-logs"，
// 与「读取侧」口径不一致——handlers.findTaskLogDir 走 statepath.CAEventLogRoot()
// （覆盖顺序 ZERG_CA_LOG_DIR → <ZERG_TMP_DIR|/tmp>/zerg-ca-logs）。
// 于是设了 ZERG_CA_LOG_DIR（publish/docs/CONFIGURATION 记载的官方开关）时:
// 读在新根、写还在旧根 ⇒ CA 日志（UI 轮次/耗时/取证）读不到。
// 改由 statepath.CAEventLogRoot() 派生 ⇒ 写读同源；默认值仍等价（tmpBase 默认字面 "/tmp"）。
// 变量名必须仍是 ZERG_LOG_DIR: CA 子进程（core/cmd/zerg-agent/main.go）只认这个字面量，
// 不认 ZERG_CA_LOG_DIR，改错一侧等于丢日志（生产链路——不得改）。
func caLogEnv() string { return "ZERG_LOG_DIR=" + statepath.CAEventLogRoot() }

// runTask 执行任务（spawn zerg-agent——CA 子调度接管执行）
// v2.5.5 打断/恢复: 记录 cmd 进程——PauseRunning 可发信号打断（SIGUSR1——CA 存 checkpoint 退出）
// v2.5.5 T2 worktree: 内部任务用 git worktree 隔离开发（独立目录——merge 回 main——任务 git 底座）
// v2.5.6 新流程: task.Flow=="zerg" → 程序定量驱动（状态机——内部调模型——不 spawn CA）
func (s *MasterScheduler) runTask(task *Task) {
	// R7 GAP-20260929-26 ①: 领到即刷心跳（claim/heartbeat 现读面）。
	s.touchHeartbeat(task.ID)
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
		branch := worktreeBranchFor(task.ID)
		wt, err := createWorktree(task.Workdir, branch)
		if err != nil {
			// v2.5.5 修复（2026-08-21 发现——重跑任务分支残留冲突）: 强删旧分支重试一次
			log.Printf("⚠️ scheduler: task %s worktree create failed (clearing stale branch, retrying): %v", task.ID, err)
			_ = exec.Command("git", "-c", "core.quotepath=false", "worktree", "remove", "--force", filepath.Join(task.Workdir, "zerg-wt", branch)).Run()
			_ = exec.Command("git", "-c", "core.quotepath=false", "branch", "-D", branch).Run()
			wt2, err2 := createWorktree(task.Workdir, branch)
			if err2 != nil {
				log.Printf("⚠️ scheduler: task %s worktree retry failed (continuing in original dir): %v", task.ID, err2)
			} else {
				worktreeDir = wt2
				log.Printf("🌿 scheduler: task %s worktree retry succeeded: %s", task.ID, wt2)
			}
		} else {
			worktreeDir = wt
			log.Printf("🌿 scheduler: task %s worktree created: %s", task.ID, wt)
		}
	}

	// v2.5.5 任务目录唯一化（2026-08-20 设计——Mr2109）: 任务目录 /tmp/zerg-tasks/<任务ID>/
	// 任务所有产出（代码/临时脚本/报告/日志）集中此目录——隔离+归档
	// 注: 不用 /var（Mac 普通用户无权限）——用 /tmp/zerg-tasks（可写——与 zerg-* 约定一致）
	taskDir := filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		log.Printf("⚠️ scheduler: task %s directory create failed: %v", task.ID, err)
	}
	os.MkdirAll(filepath.Join(taskDir, "work"), 0o755)
	os.MkdirAll(filepath.Join(taskDir, "logs"), 0o755)
	// 审计文件（任务元数据）
	audit := map[string]interface{}{
		"task_id":  task.ID,
		"type":     task.Type,
		"model":    task.Model,
		"priority": task.Priority,
		"created":  task.CreatedAt,
		"desc":     task.Description,
	}
	if ab, err := json.Marshal(audit); err == nil {
		os.WriteFile(filepath.Join(taskDir, "audit.jsonl"), append(ab, '\n'), 0o644)
	}
	log.Printf("📁 scheduler: task %s directory created: %s", task.ID, taskDir)

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

	// 子任务结晶模式: ZERG_SUBTASK=1 → CA 带 -subtask flag（设计-子任务结晶模式-20260905 S5——修: 必须在 exec.Command 前 append）
	if os.Getenv("ZERG_SUBTASK") == "1" {
		args = append(args, "-subtask")
	}
	cmd := exec.Command(s.agentCmd, args...)
	// v2.5.5 P1-2 修复: 日志写固定位置（默认 /tmp/zerg-ca-logs/）——防 worktree merge 后日志丢失
	// 默认写 workdir/.zerg/logs（worktree 内）——merge 删 worktree——跟踪数据断
	baseEnv := s.agentEnv
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	// Hermes 协议转正（2026-09-08——C3 实测: 原生 tools 字段畸形 arguments→空参数归零——
	// 外部 CA 任务一律 Hermes 化(XML 模板工具——不带 tools 字段)——Mr2109确认——与内核转正同线）
	baseEnv = append(baseEnv, "ZERG_HERMES_TOOLS=1")
	if os.Getenv("ZERG_SUBTASK") != "1" {
		log.Printf("ℹ️ scheduler: ZERG_SUBTASK=%q (not 1 — task %s skips crystallization mode — Hermes-style is default on)", os.Getenv("ZERG_SUBTASK"), task.ID)
	}
	// 2026-09-05 对照验证: ZERG_LOOPCORE 显式转发给 CA（core 进程 env 或 s.agentEnv 任一配置即生效）
	if os.Getenv("ZERG_LOOPCORE") != "" {
		baseEnv = append(baseEnv, "ZERG_LOOPCORE="+os.Getenv("ZERG_LOOPCORE"))
	}
	// S6: 任务级 env 透传（rework 续作断点数据）
	baseEnv = append(baseEnv, task.ExtraEnv...)
	// 2026-09-18 硬编码治理批 D: 原写死 "ZERG_LOG_DIR=/tmp/zerg-ca-logs" → 走 statepath.CAEventLogRoot()
	cmd.Env = append(baseEnv, caLogEnv())
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
	// 2026-10-21 任务状态机修（CompletedAt 污染 / 双份写）:
	//   原实现此处**无条件** `task.CompletedAt = time.Now()` 且**无条件** `s.history[task.ID] = task`，
	//   且都在「成功/失败分支之前」⇒ ① 失败回队（queued）与挂起（waiting_retry）带着完成时间落盘；
	//   ② 同一件同时躺在 history 与 queue，落盘两份同 ID（存量 252 原始条 / 175 去重）。
	//   现在: 完成时间与 history 都只属于**真终态**（各分支走 applyStatus；入档见下方 IsTerminal 守卫）。
	//   历史上限抽成 trimHistoryLocked()，只在该件确认为终态后调用。
	now := time.Now() // 本次收尾的唯一时间源（各分支不再各取一次 time.Now）

	// 资源信任度（2026-08-21 Mr2109）: 任务完成→模型使用计数（成功）/失败→故障计数
	if task.Model != "" {
		if err != nil {
			resourceTrust.FaultResource("models", task.Model)
		} else {
			resourceTrust.UseResource("models", task.Model)
		}
	}

	if err != nil {
		task.applyStatus(StatusFailed, now)
		log.Printf("🔄 scheduler: task %s failed: %v\n%s", task.ID, err, tail(string(out), 500))
		task.RetryCount++
		// 2026-09-09(诊断 R3): 模型/后端类失败(exit 3=ReasonModelError——熔断/超时/实例挂死)
		// 进 waiting_retry(环境恢复自动重派 30s 扫描)——不秒级热重试烧次数——治"卡 queued/秒 failed"连锁
		if isBackendExit(err) && task.RetryCount <= 3 {
			task.applyStatus(StatusWaitingRetry, now)
			task.FailReason = fmt.Sprintf("model/backend unavailable (attempt %d — %v) — waiting for environment recovery to auto-retry", task.RetryCount, err)
			s.waiting[task.ID] = task
			if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
				log.Printf("⚠️ 任务落盘失败: %v\n", err)
			}
			log.Printf("⏳ scheduler: task %s backend unavailable — moved to wait queue (auto re-dispatch on recovery — attempt %d/3)", task.ID, task.RetryCount)
			s.dispatchLocked()
			return
		}
		// v2.5.5 失败自动重跑（Mr2109 2026-08-20）: 失败任务自动重排队（同 ID——不新建记录）
		// 瞬时故障（熔断/网络——大概率恢复）——重跑 3 次仍失败才放执行完成标 failed
		if task.RetryCount <= 3 {
			task.applyStatus(StatusQueued, now)
			// v2.5.5 修复（2026-08-24 Mr2109）: 同一个任务不换模型——重跑保持原模型
			// 之前换模型（nextModelForRetry）导致: 每次换模型→node-a 加载切换窗口→请求失败→熔断累计→node-a 误熔断
			// 失败根因是系统（加载切换/超时）不是模型——换模型反而引入新失败点——保持原模型重跑（同配置同后端——恢复概率高）
			task.FailReason = fmt.Sprintf("auto-retry %d after failure (same model %s — same ID)", task.RetryCount, task.Model)
			heap.Push(&s.queue, task)
			log.Printf("🔁 scheduler: task %s auto-retry after failure (attempt %d/3 — same model %s — same ID)", task.ID, task.RetryCount, task.Model)
		} else {
			task.FailReason = fmt.Sprintf("still failing after 3 retries: %v", err)
			log.Printf("❌ scheduler: task %s still failing after 3 retries — releasing as finished (investigate)", task.ID)
		}
	} else {
		task.applyStatus(StatusDone, now)
		log.Printf("🔄 scheduler: task %s done", task.ID)
		// v2.5.5 任务git全生命周期（阶段2——设计-20260820）: 复查任务完成 → 决策（通过/打回）
		if task.Type == "review" {
			s.handleReviewDoneLocked(task)
			// 复查任务本身不进确定性验证（复查是判断——不是执行）
			// 终态才留档: history 与 queue/running/waiting 是**互不相交**的分区（落盘只一份）
			s.history[task.ID] = task
			s.trimHistoryLocked()
			if err := saveTasksLocked(s.queue, s.running, s.history); err != nil {
				log.Printf("⚠️ 任务落盘失败: %v\n", err)
			}
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
			log.Printf("📋 scheduler: task %s contract validation (%d files/%d commands/%d diff)",
				task.ID, len(contract.MustWriteFiles), len(contract.MustPassCmds), len(contract.MustDiffPaths))
		} else {
			vres = VerifyTaskOutput(verifyDir, reportPath, task.Description)
		}
		if !vres.Pass {
			task.applyStatus(StatusFailed, now)
			task.FailReason = "确定性验证不过: " + strings.Join(vres.Failures, "; ")
			log.Printf("❌ scheduler: task %s failed deterministic validation (fake-completion blocked): %s", task.ID, strings.Join(vres.Failures, "; "))
			// v2.5.5 失败自动重跑（Mr2109 2026-08-20）: 验证不过也自动重跑（可能瞬时模型问题——重跑恢复）
			task.RetryCount++
			if task.RetryCount <= 3 {
				task.applyStatus(StatusQueued, now)
				task.FailReason = fmt.Sprintf("auto-retry %d after validation failure (last: %s)", task.RetryCount, strings.Join(vres.Failures, "; "))
				heap.Push(&s.queue, task)
				log.Printf("🔁 scheduler: task %s auto-retry after validation failure (attempt %d/3 — same ID)", task.ID, task.RetryCount)
			} else {
				task.FailReason = fmt.Sprintf("still failing validation after 3 retries: %s", strings.Join(vres.Failures, "; "))
				log.Printf("❌ scheduler: task %s still failing validation after 3 retries — releasing as finished (investigate)", task.ID)
			}
		} else {
			log.Printf("✅ scheduler: task %s deterministic validation passed (%d checks)", task.ID, len(vres.Checks))
			// v2.5.5 任务git全生命周期（阶段2——设计-20260820）: 自动派复查任务（跨家族模型 B）
			// 复查决定通过/打回——不是执行完直接 done（Mr2109: 成功与否由复查模型决定）
			s.submitReviewTaskLocked(task, reportPath, worktreeDir)
		}
	}
	// v2.5.5 P1-5 补: 任务状态落盘（重启后保留——UI 早上能看到所有任务）
	// 只有真终态才进 history（回队/挂起件只活在 queue/waiting 里——见 persistedTasks 的分区不变式）
	if task.IsTerminal() {
		s.history[task.ID] = task
		s.trimHistoryLocked()
	}
	if err := saveTasksLocked(s.queue, s.running, s.history); err != nil {
		log.Printf("⚠️ 任务落盘失败: %v\n", err)
	}
	// v2.5.5 P1-3: 更新任务单状态（任务单文件——idle_detector 建的任务区 *.md）
	if task.IssuePath != "" {
		updateIssueStatus(task.IssuePath, task.Status)
	}
	// v2.5.5 T2: worktree 完成后 merge 回 main + 清理（任务 git 底座）
	// v2.5.5 阶段2改: 执行完成不 merge——复查通过才 merge（复查任务 handleReviewDoneLocked 里做）
	// 但: 确定性验证不过/执行失败的任务——worktree 直接清理（不 merge——防脏分支堆积）
	if worktreeDir != "" && (task.Status == "failed" || task.FailReason != "") {
		// 失败任务——worktree 强清（无复查——不 merge）
		_ = exec.Command("git", "-c", "core.quotepath=false", "worktree", "remove", "--force", worktreeDir).Run()
		_ = exec.Command("git", "-c", "core.quotepath=false", "branch", "-D", "task-"+sanitizeID(task.ID)).Run()
		log.Printf("🗑️ scheduler: task %s failed — worktree cleanup (no merge)", task.ID)
	}
	// 完成后继续派发（队列里还有任务）
	s.dispatchLocked()
}

// updateIssueStatus 更新任务单文件状态（v2.5.5 P1-3: 总调度器完成/失败时同步任务单）
// 任务单格式: "- 状态: open" → "- 状态: done/failed"
func updateIssueStatus(issuePath, status string) {
	content, err := os.ReadFile(issuePath)
	if err != nil {
		log.Printf("⚠️ scheduler: issue update failed (read): %v", err)
		return
	}
	newContent := regexp.MustCompile(`- 状态: \w+`).ReplaceAllString(string(content), "- 状态: "+status)
	if newContent != string(content) {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			log.Printf("⚠️ scheduler: issue update failed (write): %v", err)
			return
		}
		log.Printf("📝 scheduler: issue %s status → %s", filepath.Base(issuePath), status)
	}
}

// sanitizeID 任务 ID 清洗（git 分支名合法）
func sanitizeID(id string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return re.ReplaceAllString(id, "-")
}

// worktreeBranchFor —— worktree 分支名的**唯一算法**（§二十一 已红第 8 条 · 批 C 的 T-30）。
//
// 病灶（照已红第 8 条）：建树那一侧用 `sanitizeID(task.ID)`（得到 `internal-health-check-1787…`），
// 而合并那一侧用 `"task-"+sanitizeID(task.ID)`（得到 `task-internal-health-check-1787…`）——
// **两个算法** ⇒ `git merge <branch>` 找不到分支 ⇒ 合并失败，而任务产物**留在孤儿分支上**
// （`git log --all --grep='任务产物（worktree 自动提交）'` 一堆条、`main` 一条都没有）。
//
// 口径：分支名只从这一处出。建树（`runTask` 与它的重试）、合并（`mergeWorktree`）、
// 就地读报告（`gitShowReport`）、残留清理（`cleanupOneStaleWorktree`）**全走它**。
// 另：`mergeWorktree` 现在连这个名字都不再收 —— 它从 worktree 目录名取（目录名就是分支名，
// 见 `createWorktree` 的 `zerg-wt/<branch>` 布局），调用方**再也**传不进第二个名字。
func worktreeBranchFor(taskID string) string {
	return "task-" + sanitizeID(taskID)
}

// mergeRequest —— 合并请求（§二十一 已红第 8 条判据③「合并请求带 `base_sha`」的落点）。
//
//   - `BaseSHA` = **发起合并时 `main` 的 commit sha**（乐观锁：`main` 在合并前后不一致 ⇒ 拒合，
//     免得把别人的改动顺手卷进来 —— 同 §九 M3 `C3` 的 `--expect=<旧值>` 语义）；
//   - `HeadSHA` = 分支尖端的 sha（真正要并进来的那一条）；
//   - `REQID`   = `MR-<base8>-<head8>`（日志与回执里带它，事后能对上「哪一次合并、基于哪一版」）。
type mergeRequest struct {
	Branch  string
	BaseSHA string
	HeadSHA string
	REQID   string
}

// newMergeRequest 组一条合并请求（分支名从 worktree 目录名取 —— 唯一算法，见 worktreeBranchFor）。
func newMergeRequest(repoDir, wtDir string) (mergeRequest, error) {
	mr := mergeRequest{Branch: filepath.Base(wtDir)}
	var err error
	if mr.BaseSHA, err = gitOut(repoDir, "rev-parse", "main"); err != nil {
		return mr, fmt.Errorf("merge request: 取 main 的 base_sha 失败：%w", err)
	}
	if mr.HeadSHA, err = gitOut(repoDir, "rev-parse", mr.Branch); err != nil {
		return mr, fmt.Errorf("merge request: 分支 %q 不存在（建树侧与合并侧的分支名必须同一个算法）：%w", mr.Branch, err)
	}
	b, h := mr.BaseSHA, mr.HeadSHA
	if len(b) > 8 {
		b = b[:8]
	}
	if len(h) > 8 {
		h = h[:8]
	}
	mr.REQID = "MR-" + b + "-" + h
	return mr, nil
}

// guardBaseSHA —— 乐观锁：合并前再核一次 `main` 还是不是发起时那一版。
// 不是 ⇒ 拒合（**不 merge、不删分支**）：产物仍在分支上，人可以重发起。
func guardBaseSHA(repoDir, want string) error {
	cur, err := gitOut(repoDir, "rev-parse", "main")
	if err != nil {
		return fmt.Errorf("guard base_sha: 读 main 失败：%w", err)
	}
	if cur != want {
		return fmt.Errorf("guard base_sha: main 已被并发改动（base_sha=%s · now=%s）⇒ 拒合（重发起一次）", want, cur)
	}
	return nil
}

// gitOut 跑一条 git 命令并返回**去空白**的 stdout（失败带原文）。
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotepath=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, tail(string(out), 300))
	}
	return strings.TrimSpace(string(out)), nil
}

// repoRootForWorktree —— 从一个 worktree 目录**反推主仓根**（§二十一 已红第 8 条的第二个成因 · T-30）。
//
// ★ 为什么必须有它：老代码算的是 `filepath.Dir(filepath.Dir(wtDir))`，注释写「zerg-wt/<branch> → repo」——
// 但 `createWorktree` 把树建在 `Dir(repo)/zerg-wt/<branch>`，于是 `Dir(Dir(wtDir))` 得到的是 **repo 的父目录**，
// 不是 repo。`git -C <repo 的父目录> checkout main` 当场失败 ⇒ **合并从来没成功过**，产物全留在分支上
// （判据②实测：全仓 28 条「任务产物（worktree 自动提交）」· 进 `main` 0 条 —— 分支名那半 + 这一半，两个病叠在一起）。
//
// 口径：不问路径长什么样，**问 git 自己** —— `rev-parse --git-common-dir` 在 worktree 里返回主仓的 `.git`，
// 它的父目录就是主仓根；再用 `--show-toplevel` 复核一次（不一致 ⇒ 报错，fail-closed 不猜）。
func repoRootForWorktree(wtDir string) (string, error) {
	common, err := gitOut(wtDir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("repoRootForWorktree: 取 git-common-dir 失败（%s 是 worktree 吗）：%w", wtDir, err)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(wtDir, common)
	}
	repo := filepath.Clean(filepath.Dir(filepath.Clean(common)))
	top, err := gitOut(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("repoRootForWorktree: 反推出的主仓根 %q 不是 git 顶层：%w", repo, err)
	}
	if filepath.Clean(top) != repo {
		return "", fmt.Errorf("repoRootForWorktree: 反推不一致（--show-toplevel=%q · 反推=%q）⇒ 不猜、不合", top, repo)
	}
	return repo, nil
}

// createWorktree 建 git worktree（repo 同级——独立目录 + 分支）
// v2.5.5 T2——任务 git 底座——内部任务隔离开发
func createWorktree(repoDir, branch string) (string, error) {
	wtDir := filepath.Join(filepath.Dir(repoDir), "zerg-wt", branch)
	cmd := exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "worktree", "add", wtDir, "-b", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("worktree add failed: %v\n%s", err, tail(string(out), 300))
	}
	return wtDir, nil
}

// mergeWorktree 合并 worktree 分支回 main + 清理
// v2.5.5 T2——任务完成 merge 回 main——删 worktree + 分支
// v2.5.5 P1-1: merge 前检查报告文件（内部任务——没报告=假完成——不 merge）
// mergeWorktree 合并 worktree 分支回 main + 清理
// v2.5.5 T2——任务完成 merge 回 main——删 worktree + 分支
// v2.5.5 P1-1: merge 前检查报告文件（内部任务——没报告=假完成——不 merge）
//
// ★ 2026-09-20 批 C · T-30 改（§二十一 已红第 8 条「两套分支名算法 ⇒ 产物落孤儿分支」）——
// 三条纪律，每条都对着这条已红的一个成因：
//
//	① **不收分支名**（签名由 `(wtDir, branch)` 变成 `(wtDir)`）：分支名从 `filepath.Base(wtDir)` 取。
//	   `createWorktree` 把树建在 `zerg-wt/<branch>` ⇒ **目录名就是分支名**，这是构造上的单一真源；
//	   调用方**再也**传不进第二个名字（建树用 `sanitizeID(id)`、合并用 `"task-"+sanitizeID(id)`
//	   那一对就是这样漂起来的 —— 漂的代价是一批产物落在没人看的孤儿分支上）。
//	② **合并请求带 `base_sha`**：进 main 之前核一次「main 还是不是发起时那一版」，不是 ⇒ **拒合**
//	   （不 merge、不删分支；产物留在分支上可重发起，比悄悄并进去干净）。
//	③ **先证明产物真进了 main，再删分支**：`git merge-base --is-ancestor <branch> main` 不过 ⇒
//	   返回错误并**不删分支**（孤儿分支的成因从来不是「合并不上」，是「合并不上还照删」）。
func mergeWorktree(wtDir string) error {
	repoDir, err := repoRootForWorktree(wtDir)
	if err != nil {
		return err
	}
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
		return fmt.Errorf("worktree has no task report file (P1-1 anti fake-completion) — task produced no report — not merging")
	}
	// 0. 先提交 worktree 未跟踪/未提交改动（任务产物——CA 可能只写没 commit）
	// v2.5.5 测试发现: CA 写文件没 commit——merge 丢产物 + remove 拒绝（未跟踪文件）
	addCmd := exec.Command("git", "-C", wtDir, "-c", "core.quotepath=false", "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("worktree add -A failed: %v\n%s", err, tail(string(out), 300))
	}
	commitCmd := exec.Command("git", "-C", wtDir, "-c", "core.quotepath=false", "-c", "user.email=zerg@local", "-c", "user.name=Zerg AI", "commit", "-m", "task: 任务产物（worktree 自动提交）")
	if out, err := commitCmd.CombinedOutput(); err != nil {
		// 没有改动（commit 失败——nothing to commit）不算错——继续
		log.Printf("   (worktree has no new changes — skipping commit — %s)\n", tail(string(out), 80))
	}
	// 0′. 合并请求（判据③）：分支名从目录名取 + base_sha = 现在的 main
	mr, err := newMergeRequest(repoDir, wtDir)
	if err != nil {
		return err
	}
	log.Printf("📨 merge request %s: branch=%s base_sha=%s head_sha=%s\n", mr.REQID, mr.Branch, mr.BaseSHA, mr.HeadSHA)
	// 1. 切回 main + merge 分支
	cmd := exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "checkout", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout main failed: %v\n%s", err, tail(string(out), 300))
	}
	// 1′. 乐观锁：切回 main 之后**再核一次** base_sha（切之前可能已被并发写）
	if err := guardBaseSHA(repoDir, mr.BaseSHA); err != nil {
		return err
	}
	cmd = exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "merge", "--no-edit", mr.Branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s failed (base_sha=%s): %v\n%s", mr.Branch, mr.BaseSHA, err, tail(string(out), 300))
	}
	// 1″. 先证明真进了 main（product 上了 main 才允许删分支）
	if _, err := gitOut(repoDir, "merge-base", "--is-ancestor", mr.Branch, "main"); err != nil {
		return fmt.Errorf("merge %s 报成功但 %s 不是 main 的祖先 ⇒ **不删分支**（孤儿分支就是这么来的）：%v", mr.REQID, mr.Branch, err)
	}
	// 2. 删 worktree + 分支
	cmd = exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "worktree", "remove", "--force", wtDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("worktree remove failed: %v\n%s", err, tail(string(out), 300))
	}
	cmd = exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "branch", "-d", mr.Branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("branch -d failed: %v\n%s", err, tail(string(out), 300))
	}
	log.Printf("✅ merge %s 完成：%s → main（产物已确证在 main 上，分支已删）\n", mr.REQID, mr.Branch)
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
//
// ★ T-09 登记↔实现分叉（2026-10-03 现读，两面逐字对读）:
//
//	登记面 `zerg help dangerous` 对 `task terminate` 逐字写「任务状态置 **terminated**」
//	  （core/cmd/zerg/main.go:1653）；而**实现**（本函数）写 `t.Status = "failed"`。两态名不一致。
//
// ★ 本刀**只回报、不改**: 对齐须改 main.go——那是【禁改三件】之一；且让实现真写 terminated
//
//	会多出一个代码面七态、持久化四分区、UI 27 处按态分支都不认的态（恢复/复活面直接踩空）。
//	设计 §O-23① 已把此处另立一笔账（不混进本笔）。故此处保持 failed 不动。
func (s *MasterScheduler) TerminateTask(taskID string) error {
	s.mu.Lock()
	t, ok := s.running[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s is not running (only running tasks can be stopped)", taskID)
	}
	t.applyStatus(StatusFailed, time.Now())
	t.FailReason = "用户手动终止（右键）"
	// v2.5.6 修复（2026-08-28 实证）: 终止后必须删除 running + 派发下一个——
	// 之前只标 failed 不删 running → 调度器认为并发满 → 排队任务永不派发（任务队列卡死）
	delete(s.running, taskID)
	s.history[taskID] = t
	if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
		log.Printf("⚠️ 任务落盘失败: %v\n", err)
	}
	s.dispatchLocked()
	s.mu.Unlock()
	// 杀 CA 进程（SIGKILL——强制终止——2026-08-22 Mr2109）
	s.killTaskProcess(taskID)
	log.Printf("🛑 task %s stopped by user\n", taskID)
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
		return fmt.Errorf("task %s is not running (only running tasks can be re-queued)", taskID)
	}
	t.applyStatus(StatusQueued, time.Now())
	t.FailReason = ""
	s.queue = append(s.queue, t)
	delete(s.running, taskID)
	// 2026-10-03 GAP-20261003-04 刀1（T-10 · §O-7③）: 补一次 saveTasksLocked——
	// 同件 TerminateTask 调了它而本函数漏调。「迁移」与「落盘」是两个可被漏调的独立步骤:
	// 只改内存不落盘 ⇒ 重启后盘上仍是 running ⇒ 恢复分区把它标 failed，续作丢失。
	if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
		log.Printf("⚠️ 任务落盘失败: %v\n", err)
	}
	s.mu.Unlock()
	// 杀旧 CA 进程（停止当前执行——任务重回队列等下次）
	s.killTaskProcess(taskID)
	log.Printf("🔁 task %s re-queued (user action)\n", taskID)
	return nil
}

// CancelTask 取消/结案任务（2026-10-03 GAP-20261003-04 刀1 · T-04/T-05/T-06）。
//
// 两层语义（设计 §4.1 · §O-8 · §O-9）:
//   - 「已取消」（状态迁移）: status ∈ {queued,paused} → cancelled，**且**打 closed_* 标记；
//   - 「已销」（只打标）    : status ∈ {done,failed}        → **不动 status**，只打 closed_* 标记；
//   - 已 cancelled（重复关）: 幂等 no-op，静默成功（§11.2「关已关的 ⇒ 静默成功」）。
//
// 六步顺序（设计 §11.4，**只有一个持久点**）——锁内 ①校验 ②改态 ③迁容器 ④saveTasksLocked
// ⑤dispatch 只占位不 spawn；解锁后 ⑥真拉起。★ save 失败必须致命: 不带「内存已改、盘未落」继续派发。
//
// 守卫（T-05）: cancelled 只从 queued/paused 进；running 拒绝（在飞——先 terminate）；
// waiting_retry 拒绝（等机器恢复）；done/failed 拒绝**覆写 status**（走正交面打标——§4.1
// 「不是不给销案，而是不能覆写」；覆写会永久丢掉「到底成没成」）。
//
// 返回: outcome ∈ 三值闭集（CancelOutcomeCancelled / AlreadyCancelled / AlreadyTerminal——§11.2）；
// 真失败返 error 且 outcome 为空（找不到 / running / waiting_retry / 理由不在闭集 / 落盘失败）。
// ★ 退码只答「命令能否执行」: 三种成功结局**都返 nil**，状态差异走 outcome 返回值（§11.2 铁律）。
func (s *MasterScheduler) CancelTask(taskID, reason, note string) (outcome string, err error) {
	// ① 校验: 结案理由必须在闭集内（§11.5——闭集必填，落盘存字符串值）。
	if !isClosedReason(reason) {
		return "", fmt.Errorf("cancel task %s: 结案理由 %q 不在闭集 %v 内", taskID, reason, closedReasonSet)
	}
	s.mu.Lock()

	// ---- ① 校验 + 定位（在 queue 里找——paused 件也留在 queue，只改 status）----
	for i, t := range s.queue {
		if t.ID != taskID {
			continue
		}
		if t.Status != StatusQueued && t.Status != StatusPaused {
			st := t.Status
			s.mu.Unlock()
			return "", fmt.Errorf("task %s status %s cannot be cancelled (cancellable statuses: queued, paused, waiting_retry)", taskID, st)
		}
		// ② 改态
		t.applyStatus(StatusCancelled, time.Now())
		// ③ 迁容器: 从 queue 摘出 → 落 history（血证 §2.6——只改态不摘容器 ⇒ 并发满 ⇒ 队列卡死）
		s.queue = append(s.queue[:i], s.queue[i+1:]...)
		heap.Init(&s.queue)
		t.markClosed(reason, note)
		s.history[taskID] = t
		// ④ **唯一持久点**（commit point）
		if err := saveTasksForCancel(s.queue, s.running, s.history, s.waiting); err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("取消任务 %s 落盘失败（**未派发下一个**——本次取消未提交）: %w", taskID, err)
		}
		// ⑤ 只占位不 spawn（锁内纯内存决策）
		pending := s.dispatchDecideLocked()
		s.mu.Unlock()
		// ⑥ 解锁后真拉起（慢 IO 在锁外）
		for _, next := range pending {
			go s.runTask(next)
		}
		log.Printf("🚫 scheduler: task %s cancelled (reason=%s)\n", taskID, reason)
		return CancelOutcomeCancelled, nil
	}

	// ---- 不在 queue: 依次排掉 running / waiting，再看 history ----
	if _, ok := s.running[taskID]; ok {
		s.mu.Unlock()
		return "", fmt.Errorf("task %s is running (running tasks cannot be cancelled — terminate first)", taskID)
	}
	if t, ok := s.waiting[taskID]; ok {
		// 2026-10-22 GAP-20261022-02 —— waiting_retry 原先**两个门都不收**
		// （cancel 回「waiting for environment recovery … cannot be cancelled」、terminate 回
		// 「is not running (only running tasks can be stopped)」）⇒ 用户从命令面根本关不掉它，
		// 只能等 30 分钟超时降级 failed 或手改状态文件。
		// 现口径: waiting_retry 是**停着的**（未派发、未占并发槽），撤销它就是「退场」，
		// 与 queued/paused 同一个语义（cancelled ≠ 完成，plan §O-8）⇒ 就地结案。
		// ★ 必须**同时**从 s.waiting 摘掉 —— 留着就会被恢复循环捞回去重新派发（复活）。
		if t.Status != StatusWaitingRetry {
			st := t.Status
			s.mu.Unlock()
			return "", fmt.Errorf("task %s status %s in the waiting partition cannot be cancelled (cancellable statuses: queued, paused, waiting_retry)", taskID, st)
		}
		t.applyStatus(StatusCancelled, time.Now())
		t.markClosed(reason, note)
		delete(s.waiting, taskID)
		s.history[taskID] = t
		if err := saveTasksForCancel(s.queue, s.running, s.history, s.waiting); err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("取消任务 %s 落盘失败（本次取消未提交）: %w", taskID, err)
		}
		s.mu.Unlock()
		log.Printf("🚫 scheduler: task %s cancelled (reason=%s · 原 waiting_retry，已摘出等待分区)\n", taskID, reason)
		return CancelOutcomeCancelled, nil
	}
	t, ok := s.history[taskID]
	if !ok {
		s.mu.Unlock()
		return "", fmt.Errorf("task %s not found", taskID)
	}
	switch t.Status {
	case StatusQueued, StatusPaused, StatusWaitingRetry:
		// 2026-10-22 GAP-20261022-01/02 —— 「可恢复态却落在 history」的孤儿件。
		//
		// 制造者: tasks_persist.go::parkUnrecovered（属可恢复集却落在非对应分区 ⇒ 保守留 history、
		// 不派发、**不跨分区重路由**），而 loadPersistedHistory 只从 queue 回队 ⇒ 这些件
		// **永不派发**、投影恒报 queued。撤销面原先只从 queue 认 queued/paused，于是它们一路
		// 落进本 switch 的 default，报出「status queued cannot be cancelled (only queued/paused
		// tasks can be cancelled)」——文案与状态自相矛盾，且件**永远撤不掉**。
		//
		// 处置: 按 status 就地结案（件已在 history，不动容器），写 closed_* 正交标记。
		// ★ 绝不回填 queue —— 回填等于让一件从未派发的孤儿复活并重新派发（§2.5 白名单不动）。
		t.applyStatus(StatusCancelled, time.Now())
		t.markClosed(reason, note)
		if err := saveTasksForCancel(s.queue, s.running, s.history, s.waiting); err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("取消任务 %s 落盘失败（本次取消未提交）: %w", taskID, err)
		}
		s.mu.Unlock()
		log.Printf("🚫 scheduler: task %s cancelled (reason=%s · history 里的可恢复态孤儿件: 未派发、未回队)\n", taskID, reason)
		return CancelOutcomeCancelled, nil
	case StatusCancelled:
		// 幂等: 关已关的 ⇒ 静默成功、无副作用（§11.2 · F-6）
		s.mu.Unlock()
		return CancelOutcomeAlreadyCancelled, nil
	case StatusDone, StatusFailed:
		// 「已销」: 只打正交标记，**不覆写 status**（§O-8/§O-9——覆写会永久丢掉「到底成没成」）
		t.markClosed(reason, note)
		if err := saveTasksForCancel(s.queue, s.running, s.history, s.waiting); err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("销案任务 %s 落盘失败: %w", taskID, err)
		}
		st := t.Status
		s.mu.Unlock()
		log.Printf("🗂️ scheduler: task %s closure marked (status=%s unchanged — 已销)\n", taskID, st)
		return CancelOutcomeAlreadyTerminal, nil
	default:
		st := t.Status
		s.mu.Unlock()
		return "", fmt.Errorf("task %s status %s cannot be cancelled (cancellable statuses: queued, paused, waiting_retry)", taskID, st)
	}
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

// ============ R7 · 调度器控制面: 暂停派活 / 并发旋钮 / 状态 / 死信回收 ============
// 缺口 GAP-20260927-210（P0 · 暂停派活一等入口 + maxConcurrent 持久化）与
// GAP-20260929-26（P1 · claim/心跳面 + 并发旋钮 + 死信回队）的**服务端落地**。
// 口径（三条硬线）:
//  ① 「暂停派活」与「终止在跑」是**两件事** —— 暂停只把**有效并发**置 0（不再起新的），
//     已在跑的进程一动不动；要停在跑的走既有 TerminateTask。
//  ② 并发上限（configuredMax）与暂停位（dispatchPaused）**都持久化**到统一状态目录的
//     zerg-sched.json —— 重启后 `zerg sched show` 读到的还是上次设的值（不靠人记）。
//  ③ 有效并发 = paused ? 0 : configuredMax —— 单一派生点 applySchedLocked，别处不重算。

// schedStateFileName 调度器配置持久化文件名（统一状态目录下）。
const schedStateFileName = "zerg-sched.json"

// schedConfigFile 测试隔离（照 tasksFile 体例）: 非空 ⇒ 用它；空 ⇒ 走 statepath 状态目录。
var schedConfigFile = ""

// schedConfigPath 调度器配置读写路径。
func schedConfigPath() string {
	if p := strings.TrimSpace(schedConfigFile); p != "" {
		return p
	}
	return statepath.File(schedStateFileName)
}

// persistedSched 调度器配置落盘结构（只存「用户设定」的两个旋钮 —— 不含运行态计数）。
type persistedSched struct {
	SavedAt        time.Time `json:"saved_at"`
	MaxConcurrent  int       `json:"max_concurrent"`  // 用户设定的并发上限
	DispatchPaused bool      `json:"dispatch_paused"` // 是否暂停派活
}

// applySchedLocked 由 configuredMax/dispatchPaused 派生**有效** maxConcurrent（调用方持锁）。
func (s *MasterScheduler) applySchedLocked() {
	if s.dispatchPaused {
		s.maxConcurrent = 0
		return
	}
	s.maxConcurrent = s.configuredMax
}

// saveSchedConfigLocked 原子写调度器配置（调用方持锁）—— 同 tasks_persist.go 的落盘纪律
// （同目录临时件 → fsync → rename → fsync 目录；失败返回 error，不静默吞）。
func (s *MasterScheduler) saveSchedConfigLocked() error {
	data := persistedSched{SavedAt: time.Now(), MaxConcurrent: s.configuredMax, DispatchPaused: s.dispatchPaused}
	buf, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("调度器配置序列化失败: %w", err)
	}
	dst := schedConfigPath()
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("调度器配置目录不可用（%s）: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".sched-*.tmp")
	if err != nil {
		return fmt.Errorf("调度器配置临时件创建失败（%s）: %w", dir, err)
	}
	tmpName := tmp.Name()
	fail := func(step string, e error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("调度器配置%s失败（%s）: %w", step, tmpName, e)
	}
	if _, err := tmp.Write(buf); err != nil {
		return fail("写入临时件", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return fail("设置权限", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("fsync", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("调度器配置关闭临时件失败（%s）: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("调度器配置原子替换失败（%s → %s）: %w", tmpName, dst, err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// loadSchedConfigLocked 读调度器配置并应用（调用方持锁 —— 由 Start 调一次）。
// 件不存在（首跑）⇒ 保持构造参数给的默认（与改动前逐字同行为）；件损坏/非法值 ⇒ 响亮报 + 用默认。
func (s *MasterScheduler) loadSchedConfigLocked() {
	readPath := schedConfigPath()
	buf, err := os.ReadFile(readPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️ 调度器配置读盘失败（%s）: %v —— 用构造参数的默认并发 %d", readPath, err, s.configuredMax)
		}
		s.applySchedLocked()
		return
	}
	var data persistedSched
	if err := json.Unmarshal(buf, &data); err != nil {
		log.Printf("❌ 调度器配置件损坏（%s · %d 字节）: %v —— 用构造参数的默认并发 %d", readPath, len(buf), err, s.configuredMax)
		s.applySchedLocked()
		return
	}
	if data.MaxConcurrent < 0 {
		log.Printf("⚠️ 调度器配置并发为负（%d）—— 忽略、用默认 %d", data.MaxConcurrent, s.configuredMax)
		s.applySchedLocked()
		return
	}
	s.configuredMax = data.MaxConcurrent
	s.dispatchPaused = data.DispatchPaused
	s.applySchedLocked()
	pausedWord := "运行中"
	if s.dispatchPaused {
		pausedWord = "已暂停"
	}
	log.Printf("⚙️ 调度器配置已加载（%s）: 并发上限 %d · 派活%s · 有效并发 %d", readPath, s.configuredMax, pausedWord, s.maxConcurrent)
}

// SchedClaim 一条在跑任务的领取面（claim/heartbeat 现读行）。
type SchedClaim struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Model       string `json:"model"`
	ClaimedAt   string `json:"claimed_at"`
	ClaimedBy   string `json:"claimed_by"`
	HeartbeatAt string `json:"heartbeat_at"`
}

// SchedState 调度器状态快照（只读面 —— GAP-20260927-210 ② · GAP-20260929-26 ①）。
type SchedState struct {
	MaxConcurrent  int          `json:"max_concurrent"`  // 用户设定的并发上限
	EffectiveMax   int          `json:"effective_max"`   // 有效并发（paused ⇒ 0）
	DispatchPaused bool         `json:"dispatch_paused"` // 是否已暂停派活
	Running        int          `json:"running"`         // 在跑条数
	Queued         int          `json:"queued"`          // 队列条数
	WaitingRetry   int          `json:"waiting_retry"`   // 等环境恢复条数
	Claimed        []SchedClaim `json:"claimed"`         // 在跑条目的领取面
}

// SchedState 取调度器状态快照（只读 · 加锁一拍）。
func (s *MasterScheduler) SchedState() SchedState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := SchedState{
		MaxConcurrent:  s.configuredMax,
		EffectiveMax:   s.maxConcurrent,
		DispatchPaused: s.dispatchPaused,
		Running:        len(s.running),
		Queued:         len(s.queue),
		WaitingRetry:   len(s.waiting),
		Claimed:        []SchedClaim{},
	}
	for id, t := range s.running {
		c := SchedClaim{ID: id, Status: t.Status, Model: t.Model, ClaimedBy: t.ClaimedBy}
		if t.ClaimedAt != nil {
			c.ClaimedAt = t.ClaimedAt.UTC().Format(time.RFC3339)
		}
		if t.HeartbeatAt != nil {
			c.HeartbeatAt = t.HeartbeatAt.UTC().Format(time.RFC3339)
		}
		st.Claimed = append(st.Claimed, c)
	}
	sort.Slice(st.Claimed, func(i, j int) bool { return st.Claimed[i].ID < st.Claimed[j].ID })
	return st
}

// PauseDispatch 暂停派活（一等入口 —— 缺口 GAP-20260927-210 ①）。
// 只把**有效并发**置 0（不再起新的）；在跑的进程一动不动（要停在跑的 ⇒ TerminateTask）。
// 持久化: 写 configuredMax/dispatchPaused ⇒ 重启后仍是暂停态。
func (s *MasterScheduler) PauseDispatch() error {
	s.mu.Lock()
	s.dispatchPaused = true
	s.applySchedLocked()
	if err := s.saveSchedConfigLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	log.Printf("⏸️ scheduler: dispatch paused (configured max=%d — running tasks untouched)", s.configuredMax)
	return nil
}

// ResumeDispatch 恢复派活（回到用户设定的并发上限并立即派发积压）。
func (s *MasterScheduler) ResumeDispatch() error {
	s.mu.Lock()
	s.dispatchPaused = false
	s.applySchedLocked()
	if err := s.saveSchedConfigLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.dispatchLocked()
	s.mu.Unlock()
	log.Printf("▶️ scheduler: dispatch resumed (effective max=%d)", s.maxConcurrent)
	return nil
}

// SetMaxConcurrent 设并发上限（显式旋钮 —— 缺口 GAP-20260929-26 ②；非负整数）。
// 不解除暂停（只改记住的上限）；未暂停时立即按新上限派发。
func (s *MasterScheduler) SetMaxConcurrent(n int) error {
	if n < 0 {
		return fmt.Errorf("并发上限要非负整数（给的是 %d）", n)
	}
	s.mu.Lock()
	s.configuredMax = n
	s.applySchedLocked()
	if err := s.saveSchedConfigLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if !s.dispatchPaused {
		s.dispatchLocked()
	}
	s.mu.Unlock()
	log.Printf("⚙️ scheduler: max concurrency set to %d (paused=%t)", n, s.dispatchPaused)
	return nil
}

// deadLetterTimeout 心跳超时（超过它无刷新且进程已丢 ⇒ 判死信 —— 缺口 GAP-20260929-26 ③）。
const deadLetterTimeout = 10 * time.Minute

// touchHeartbeat 刷新一条在跑任务的心跳（runTask 起跑调）。
func (s *MasterScheduler) touchHeartbeat(taskID string) {
	s.mu.Lock()
	if t, ok := s.running[taskID]; ok {
		now := time.Now()
		t.HeartbeatAt = &now
		if t.ClaimedAt == nil {
			t.ClaimedAt = &now
		}
		if t.ClaimedBy == "" {
			t.ClaimedBy = "local"
		}
	}
	s.mu.Unlock()
}

// heartbeatSweep 给「进程仍活着」的在跑任务刷心跳（自加锁）。
// 依据 = CA 进程存活（cmd.Process 非空且 ProcessState 未置 ⇒ 未回收）。flow=zerg 是进程内
// 状态机（无外部进程可探）⇒ 不计入死信判定（保守：不据心跳收它）。返回刷新的条数。
func (s *MasterScheduler) heartbeatSweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	alive := 0
	for _, t := range s.running {
		if t.Flow == "zerg" {
			continue
		}
		if t.cmd != nil && t.cmd.Process != nil && t.cmd.ProcessState == nil {
			hb := now
			t.HeartbeatAt = &hb
			alive++
		}
	}
	return alive
}

// ReclaimDeadLetters 死信回收（缺口 GAP-20260929-26 ③）—— 扫在跑任务里
// 「非 zerg-flow · 进程已丢 · 心跳距上次刷新超过 deadLetterTimeout」的件:
// 尝试 ≤3 次 ⇒ 回队重派；超过 ⇒ 标 failed 落因。返回被处理的 id 清单（空 = 无死信）。
// 调用方无需持锁。★ 存活进程的心跳由 heartbeatSweep 持续刷新 ⇒ 长任务不会被误收。
func (s *MasterScheduler) ReclaimDeadLetters() []string {
	return s.reclaimDeadLettersAfter(deadLetterTimeout)
}

func (s *MasterScheduler) reclaimDeadLettersAfter(timeout time.Duration) []string {
	s.mu.Lock()
	now := time.Now()
	var dead []*Task
	for _, t := range s.running {
		if t.Flow == "zerg" {
			continue // 进程内状态机 —— 无外部进程、不据心跳收（保守）
		}
		if t.cmd != nil && t.cmd.Process != nil && t.cmd.ProcessState == nil {
			continue // 进程仍活 ⇒ 不是死信
		}
		hb := t.HeartbeatAt
		if hb == nil {
			hb = t.ClaimedAt
		}
		if hb == nil {
			continue // 无心跳面（旧件）—— 不据此判死
		}
		if now.Sub(*hb) > timeout {
			dead = append(dead, t)
		}
	}
	if len(dead) == 0 {
		s.mu.Unlock()
		return nil
	}
	ids := make([]string, 0, len(dead))
	for _, t := range dead {
		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill() // 进程句柄还在（已退出未回收）—— 补一刀，防僵尸
		}
		delete(s.running, t.ID)
		t.ClaimedAt = nil
		t.ClaimedBy = ""
		t.HeartbeatAt = nil
		t.cmd = nil
		t.RetryCount++
		if t.RetryCount > 3 {
			t.FailReason = fmt.Sprintf("死信：心跳超时 %s 无刷新且进程已丢（已重派 %d 次）—— 标 failed 待查", timeout, t.RetryCount)
			t.applyStatus(StatusFailed, now)
			s.history[t.ID] = t
			log.Printf("☠️ scheduler: task %s dead-letter → failed (%s no heartbeat, process gone, retries %d)", t.ID, timeout, t.RetryCount)
		} else {
			t.applyStatus(StatusQueued, now)
			t.FailReason = fmt.Sprintf("死信回收：心跳超时 %s 且进程已丢 —— 回队重派（attempt %d/3）", timeout, t.RetryCount)
			heap.Push(&s.queue, t)
			log.Printf("♻️ scheduler: task %s dead-letter reclaim → re-queued (%s no heartbeat, attempt %d/3)", t.ID, timeout, t.RetryCount)
		}
		ids = append(ids, t.ID)
	}
	if err := saveTasksLocked(s.queue, s.running, s.history, s.waiting); err != nil {
		log.Printf("⚠️ 任务落盘失败: %v\n", err)
	}
	s.dispatchLocked()
	s.mu.Unlock()
	return ids
}

// ---- HTTP 处理器（本件与 handlers.go 同包；R7 排他锁不许碰 handlers.go，
//      故 sched 面处理器定义在此；路由登记由父代理串行落 core/cmd/zerg-core/main.go）----

// SchedStateHandler GET /api/sched —— 调度器状态（只读）。
func (h *Handlers) SchedStateHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	writeJSON(w, http.StatusOK, h.Scheduler.SchedState())
}

// SchedPauseHandler POST /api/sched/pause —— 暂停派活。
func (h *Handlers) SchedPauseHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.PauseDispatch(); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "SCHED_PAUSE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.Scheduler.SchedState())
}

// SchedResumeHandler POST /api/sched/resume —— 恢复派活。
func (h *Handlers) SchedResumeHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.ResumeDispatch(); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "SCHED_RESUME_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.Scheduler.SchedState())
}

// SchedSetHandler POST /api/sched/set?max=<N> —— 设并发上限。
func (h *Handlers) SchedSetHandler(w http.ResponseWriter, r *http.Request) {
	maxStr := strings.TrimSpace(r.URL.Query().Get("max"))
	n, err := strconv.Atoi(maxStr)
	if maxStr == "" || err != nil || n < 0 {
		writeErrorCode(w, http.StatusBadRequest, "BAD_MAX", "max 要非负整数（query: ?max=<N>）")
		return
	}
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.SetMaxConcurrent(n); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "SCHED_SET_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.Scheduler.SchedState())
}

// PauseTask 暂停/继续排队任务（queued→paused / paused→queued——2026-08-21 Mr2109右键功能）
func (s *MasterScheduler) PauseTask(taskID string, pause bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.queue {
		if t.ID == taskID {
			if pause && t.Status == "queued" {
				t.Status = "paused"
				log.Printf("⏸️ scheduler: task %s paused", taskID)
				return nil
			}
			if !pause && t.Status == "paused" {
				t.Status = "queued"
				log.Printf("▶️ scheduler: task %s resumed", taskID)
				// 2026-09-09: 恢复即派发(同 RetryTask 修复——防恢复后空等)
				s.dispatchLocked()
				return nil
			}
			return fmt.Errorf("task %s status %s cannot be %s", taskID, t.Status, map[bool]string{true: "paused", false: "resumed"}[pause])
		}
	}
	return fmt.Errorf("task %s is not in the queue", taskID)
}

// RetryTask 重跑任务（failed→queued——同 ID——RetryCount 重置——2026-08-21 Mr2109右键功能）
func (s *MasterScheduler) RetryTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.history {
		if t.ID == taskID && t.Status == "failed" {
			t.applyStatus(StatusQueued, time.Now())
			t.RetryCount = 0
			t.FailReason = ""
			heap.Push(&s.queue, t)
			log.Printf("🔁 scheduler: task %s manually re-run (context menu — same ID)", taskID)
			// 2026-09-09 修复（write v1.0.1 验收实证——卡 queued 10 分钟根因）:
			// 只推堆不派发 → 无后续事件则永不执行——补派发
			s.dispatchLocked()
			return nil
		}
	}
	return fmt.Errorf("task %s not found or not re-runnable", taskID)
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
		return fmt.Errorf("task %s is not queued (only queued tasks can be reordered)", taskID)
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
		return fmt.Errorf("unknown action: %s", action)
	}
	log.Printf("↕️ scheduler: task %s reordered (%s)", taskID, action)
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
			log.Printf("🗑️ scheduler: task %s deleted (context menu)", taskID)
			return nil
		}
	}
	return fmt.Errorf("task %s is not queued (only queued tasks can be deleted)", taskID)
}

// copyReportToTaskDir 复制 worktree 报告到任务目录（merge 后报告保留——2026-08-21 Mr2109发现修复）
// worktree 删除后报告丢——详情读不到——复制 internal-task-report.md/review-report.md 到任务目录
func copyReportToTaskDir(taskID, wtDir string) {
	if wtDir == "" {
		return
	}
	taskDir := filepath.Join(statepath.TaskRoot(), sanitizeID(taskID))
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

// IsTerminal 派生谓词 **is_terminal = done|failed|cancelled**（2026-10-03 GAP-20261003-04 刀1 · T-03）。
// ★ 纯派生——不落盘、不作字段、不新增合并态（设计 §4.1「要退场总览用派生谓词，不新增合并态」）。
// ★ 设计 §O-9 曾提「可升级为 … || closed_at != nil」——本仓不采纳此升级: 结案标记只对终态件
//
//	（done/failed/cancelled）打（CancelTask 守卫），故 closed_at 非空 ⟹ status 已终态，
//	升级式恒等价于本条，加了反多一处可漂的分支。
func (t *Task) IsTerminal() bool {
	switch t.Status {
	case StatusDone, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// applyStatus 改态**唯一入口**（2026-10-21 任务状态机修）——只保证一件事: 「状态」与「时间字段」永远自洽。
//
// 不变式（落盘面另由 normalizeTaskTime 兜底）:
//   - 终态 done/failed ⇒ CompletedAt = now、WaitingSince = nil；
//   - 终态 cancelled  ⇒ 不写 CompletedAt（已撤 ≠ 已完成 · 拍板 2026-10-21）、WaitingSince = nil；
//   - waiting_retry             ⇒ CompletedAt 清零、WaitingSince = now；
//   - 其余 queued/paused/running/reviewing ⇒ 两个时刻都清零。
//
// ★ 病根: runTask / finishTask 原先在「成功/失败分支**之前**」无条件写 CompletedAt ⇒
// 失败回队（queued）与挂起（waiting_retry）也带着完成时间落盘（存量 87 件 queued 带 completed_at）。
func (t *Task) applyStatus(status string, now time.Time) {
	t.Status = status
	if t.IsTerminal() {
		if status == StatusCancelled {
			// ★ 拍板（2026-10-21）: cancelled 是「退场」不是「完成」—— completed_at 一个字都不写；
			//   残留值一并清零（否则「cancelled 必无 completed_at」这条读不出来；T5 只对 done/failed 有意义）。
			t.CompletedAt = time.Time{}
		} else {
			t.CompletedAt = now
		}
		t.WaitingSince = nil
		return
	}
	t.CompletedAt = time.Time{}
	if status == StatusWaitingRetry {
		t.WaitingSince = &now
		return
	}
	t.WaitingSince = nil
}

// sinceWaiting 挂起起始时刻。旧件没有 WaitingSince ⇒ 回退用当时的 CompletedAt
// （旧实现正是把挂起时刻写在 completed_at 上）；再没有 ⇒ now（本次不判超时——宁可多等、不放件）。
func sinceWaiting(t *Task, now time.Time) time.Time {
	if t.WaitingSince != nil {
		return *t.WaitingSince
	}
	if !t.CompletedAt.IsZero() {
		return t.CompletedAt
	}
	return now
}

// trimHistoryLocked 历史上限（防无限增长——保留最近 100 个）。调用方持锁。
// 只在**终态件入档之后**调用——排序键 CompletedAt 现有不变式保证「终态必有时刻、非终态必无」。
func (s *MasterScheduler) trimHistoryLocked() {
	const maxHistory = 100
	if len(s.history) <= maxHistory {
		return
	}
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

// IsClosed 是否已结案（正交面）: closed_at 非空即已结案（T-06「非空即已结案」）。
func (t *Task) IsClosed() bool { return t.ClosedAt != nil }

// markClosed 打正交标记（**四条同生**——T-06）: 结案时间（UTC）/ 主体 / 理由 / 备注一并写入。
// 「覆盖式非 write-once」（§11.1）: 重复调用即覆盖为最新一次结案。
func (t *Task) markClosed(reason, note string) {
	now := time.Now().UTC()
	t.ClosedAt = &now
	t.ClosedBy = closedByLocal
	t.ClosedReason = reason
	t.ClosedNote = note
}

// clearClosed 清正交标记（**四条同灭**——T-06「清空即重开」）。
func (t *Task) clearClosed() {
	t.ClosedAt = nil
	t.ClosedBy = ""
	t.ClosedReason = ""
	t.ClosedNote = ""
}

// CleanupStaleWorktrees 启动清理残留任务 worktree（P1-5 治本）
// 扫描 git worktree list——所有 task-* 分支的 worktree（不在当前运行任务列表中的=残留）
// 有报告 → merge 回 main（保留产物）；无报告 → 强制清理（任务已死）
// 返回清理数量
func CleanupStaleWorktrees(repoDir string) int {
	cmd := exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "worktree", "list", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("  worktree list failed: %v\n", err)
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
		if err := mergeWorktree(wtDir); err != nil {
			log.Printf("  stale %s merge failed — forcing cleanup: %v\n", branch, err)
			exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "worktree", "remove", "--force", wtDir).Run()
			exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "branch", "-D", branch).Run()
			return true
		}
		log.Printf("  stale %s merged back to main (task artifacts kept)\n", branch)
		return true
	}
	exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "worktree", "remove", "--force", wtDir).Run()
	exec.Command("git", "-C", repoDir, "-c", "core.quotepath=false", "branch", "-D", branch).Run()
	log.Printf("  stale %s had no report — cleaned up (task is dead)\n", branch)
	return true
}
