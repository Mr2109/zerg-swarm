// package store 提供内存中的 fleet 状态存储和任务管理。
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// HeartbeatRequest 子端发送的心跳请求体。
// 字段参考 v1 主控协议。
type HeartbeatRequest struct {
	Machine        string   `json:"machine"`          // 机器标识（如 x3, local）
	Model          *string  `json:"model"`            // 当前加载的模型名（可为 null）
	Backend        *string  `json:"backend"`          // 后端名称（如 llama-server, ds4-server）
	Port           *int     `json:"port"`             // 后端端口
	MemAvailableGb float64  `json:"mem_available_gb"` // 可用内存（GB）
	MemTotalGb     float64  `json:"mem_total_gb"`     // 总内存（GB）
	Load           float64  `json:"load"`             // 系统负载（0-1）
	Models         []string `json:"models"`           // 当前加载的模型列表
	Uptime         float64  `json:"uptime"`           // 运行时间（秒）
	GpuUsedGb      float64  `json:"gpu_used_gb"`      // 真实显存占用（GB）；vram_known=false 时应视为未知，不再用 RSS 冒充
	GpuTempC       float64  `json:"gpu_temp_c"`       // GPU 温度（°C），0 表示无数据
	BackendRssGb   float64  `json:"backend_rss_gb"`   // 后端进程 RSS 内存（GB，实测）
	ActiveRequests int      `json:"active_requests"`  // 当前真实在飞请求数（子端 activeReqs）
	Healthy        bool     `json:"healthy"`          // 是否健康
	BackendState   string   `json:"backend_state"`    // 后端状态（如 ready, loading, error）
	Error          *string  `json:"error"`            // 错误信息（可为 null）
	CpuPct         float64  `json:"cpu_pct"`          // B4 v2：CPU 使用率 %
	GpuPct         float64  `json:"gpu_pct"`          // B4 v2：GPU 使用率 %
	// 代码身份（自动升级 L3）：子端自报版本+提交——"混版机群=不健康"必须看得见（设计稿 §3）
	CodeVersion string `json:"code_version"` // 子端版本号（如 2.5.9）
	CodeSHA     string `json:"code_sha"`     // 子端二进制提交（-ldflags 注入）

	// ── 资源账本新增（《设计-资源管理器》§3.1/§3.4；全部可选，缺省=该机器未提供，旧读者忽略）──
	VramKnown   bool    `json:"vram_known"`              // 子端能否拿到真实显存；false 时 gpu_used_gb 视为未知
	VramUnified bool    `json:"vram_unified,omitempty"`  // 统一内存平台（显存即内存，§3.1）——与显存未知不同：不 fail-closed
	VramTotalGb float64 `json:"vram_total_gb,omitempty"` // 真实显存总量（GB）
	VramUsedGb  float64 `json:"vram_used_gb,omitempty"`  // 真实显存占用（GB）
	VramFreeGb  float64 `json:"vram_free_gb,omitempty"`  // 真实显存空闲（GB）
	// 驻留明细：每台机器上驻留的模型（托管 managed=true / 未托管 managed=false 都如实上报）
	Resident []resources.ResidentEntry `json:"resident,omitempty"`
	// 未托管但占着端口的进程（覆盖实测 E2；Q6：只标注，不接管不杀）
	Unmanaged []resources.UnmanagedProcess `json:"unmanaged,omitempty"`
}

// HeartbeatResponse 心跳响应。
type HeartbeatResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// FleetSnapshot 单个子端的心跳快照。
// 这是集群主控维护的每个节点的运行时状态。
type FleetSnapshot struct {
	Machine        string   `json:"machine"`
	Model          *string  `json:"model,omitempty"`
	Backend        *string  `json:"backend,omitempty"`
	Port           *int     `json:"port,omitempty"`
	MemAvailableGb float64  `json:"mem_available_gb"`
	MemTotalGb     float64  `json:"mem_total_gb"`
	Load           float64  `json:"load"`
	Models         []string `json:"models"`
	Uptime         float64  `json:"uptime"`
	GpuUsedGb      float64  `json:"gpu_used_gb"`
	GpuTempC       float64  `json:"gpu_temp_c"`
	BackendRssGb   float64  `json:"backend_rss_gb"`
	ActiveRequests int      `json:"active_requests"`
	Healthy        bool     `json:"healthy"`
	BackendState   string   `json:"backend_state"`
	// B4 v2：CPU/GPU 使用率百分比（本机采集 / 子端上报）
	CpuPct float64 `json:"cpu_pct"`
	GpuPct float64 `json:"gpu_pct"`
	// 代码身份（自动升级 L3）：机群版本矩阵——混版机群必须看得见
	CodeVersion string    `json:"code_version,omitempty"`
	CodeSHA     string    `json:"code_sha,omitempty"`
	Error       *string   `json:"error,omitempty"`
	LastSeen    time.Time `json:"last_seen"` // 最后心跳时间

	// ── 资源账本新增（《设计-资源管理器》§3.1/§3.4）──
	// ⚠ 两个布尔**绝不能带 omitempty**（2026-09-16 修，实测踩过）：false 是**有意义的值** ——
	//   `vram_known=false` 表示"该机器拿不到显存"（口径见 #29：拿不到就 false，绝不用内存/RSS 冒充），
	//   `vram_unified=false` 表示"独显平台"；被 omitempty 吞掉后，消费方**分不清"没有"与"没报"** ✗。
	//   （实测形态：x3 的 vram_known=true 看得见 ✓，Mr2109 的 false 整个消失 ✗。）
	VramKnown   bool    `json:"vram_known"`              // 能否拿到真实显存；false 时 gpu_used_gb 视为未知
	VramUnified bool    `json:"vram_unified"`            // 统一内存平台（显存即内存，§3.1）；false = 独显平台
	VramTotalGb float64 `json:"vram_total_gb,omitempty"` // 真实显存总量（GB）
	VramUsedGb  float64 `json:"vram_used_gb,omitempty"`  // 真实显存占用（GB）
	VramFreeGb  float64 `json:"vram_free_gb,omitempty"`  // 真实显存空闲（GB）
	// 驻留明细（托管 managed=true；未托管 managed=false，均如实呈现）
	Resident []resources.ResidentEntry `json:"resident,omitempty"`
	// 未托管但占着端口的进程（E2；只读上报，不接管不杀）
	Unmanaged []resources.UnmanagedProcess `json:"unmanaged,omitempty"`
}

// TaskRequest 任务请求。
type TaskRequest struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Machine   string    `json:"machine"`
	Status    string    `json:"status"` // pending, running, completed, failed
	CreatedAt time.Time `json:"created_at"`
}

// Store 内存存储，管理 fleet 快照和任务列表。
type Store struct {
	mu         sync.RWMutex
	snapshots  map[string]*FleetSnapshot // machine -> snapshot
	tasks      []TaskRequest
	nextTaskID int
}

// NewStore 创建新的 Store。
func NewStore() *Store {
	return &Store{
		snapshots:  make(map[string]*FleetSnapshot),
		tasks:      []TaskRequest{},
		nextTaskID: 1,
	}
}

// ReceiveHeartbeat 处理子端心跳，更新或创建对应的快照。
func (s *Store) ReceiveHeartbeat(req HeartbeatRequest) *HeartbeatResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果机器不存在，创建新快照
	if _, exists := s.snapshots[req.Machine]; !exists {
		s.snapshots[req.Machine] = &FleetSnapshot{
			Machine:  req.Machine,
			LastSeen: time.Now(),
		}
	}

	// 更新快照
	snap := s.snapshots[req.Machine]
	snap.Model = req.Model
	snap.Backend = req.Backend
	snap.Port = req.Port
	snap.MemAvailableGb = req.MemAvailableGb
	snap.MemTotalGb = req.MemTotalGb
	snap.Load = req.Load
	snap.Models = req.Models
	snap.Uptime = req.Uptime
	snap.GpuUsedGb = req.GpuUsedGb
	snap.GpuTempC = req.GpuTempC
	snap.BackendRssGb = req.BackendRssGb
	snap.ActiveRequests = req.ActiveRequests
	snap.Healthy = req.Healthy
	snap.BackendState = req.BackendState
	snap.Error = req.Error
	snap.CpuPct = req.CpuPct // B4 v2：CPU/GPU 使用率
	snap.GpuPct = req.GpuPct
	snap.CodeVersion = req.CodeVersion // L3：记住子端自报身份
	snap.CodeSHA = req.CodeSHA
	// 资源账本新增字段（§3.1）：逐项透传，旧字段不动
	snap.VramKnown = req.VramKnown
	snap.VramUnified = req.VramUnified
	snap.VramTotalGb = req.VramTotalGb
	snap.VramUsedGb = req.VramUsedGb
	snap.VramFreeGb = req.VramFreeGb
	snap.Resident = req.Resident
	snap.Unmanaged = req.Unmanaged
	snap.LastSeen = time.Now()

	return &HeartbeatResponse{
		OK:      true,
		Message: fmt.Sprintf("收到 %s 的心跳", req.Machine),
	}
}

// GetAllSnapshots 获取所有节点的快照（用于 /api/fleet/status）。
func (s *Store) GetAllSnapshots() map[string]*FleetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*FleetSnapshot, len(s.snapshots))
	for k, v := range s.snapshots {
		result[k] = v
	}
	return result
}

// GetSnapshot 获取指定机器的快照。
func (s *Store) GetSnapshot(machine string) *FleetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshots[machine]
}

// LocalSnapshotData 是本机（local）快照的写入载荷。
//
// 为什么用载荷结构：本机不跑独立子端、不经心跳（B13）——主控必须用**自己知道的**信息
// 把 local 这一行填成与远程**同一套口径**（§八 Q7）：基础状态 + 驻留明细（resident[]）+
// 显存（拿不到就 VramKnown=false，VramUnified 表示"统一内存：显存即内存"，绝不拿内存冒充显存）。
type LocalSnapshotData struct {
	Machine        string
	Model          *string
	Models         []string
	Healthy        bool
	State          string
	MemAvailableGb float64
	MemTotalGb     float64
	Load           float64
	ActiveRequests int
	CpuPct         float64
	GpuPct         float64

	// ── 资源账本（批 5 #30）：本机驻留明细 + 显存三态 ──
	// 本机驻留清单来自 LocalBackend 自己的状态（ModelFile/MemGB/State），不是猜的。
	Resident []resources.ResidentEntry
	// VramKnown=false 时显存三值一律不填（拿不到就不冒充，与批 2 同口径）。
	VramKnown   bool
	VramUnified bool
	VramTotalGb float64
	VramUsedGb  float64
	VramFreeGb  float64
}

// SetLocalSnapshot 写入本机（local）快照——B13 修复：本机不跑独立 agent 心跳，
// 由主控周期任务把 LocalBackend 状态写入 store（路由打分需要 local 的健康/加载状态）。
//
// 批 5（#30）：载荷补齐 resident[]/vram_*，使 /api/resources/ledger 的 local 一行
// 与远程子端同口径（§八 Q7）——不再"只有内存/状态、没有驻留谁"。
func (s *Store) SetLocalSnapshot(d LocalSnapshotData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[d.Machine] = &FleetSnapshot{
		Machine:        d.Machine,
		Model:          d.Model,
		Models:         d.Models,
		MemAvailableGb: d.MemAvailableGb,
		MemTotalGb:     d.MemTotalGb,
		Load:           d.Load,
		Healthy:        d.Healthy,
		BackendState:   d.State,
		ActiveRequests: d.ActiveRequests,
		CpuPct:         d.CpuPct,
		GpuPct:         d.GpuPct,
		Resident:       d.Resident,
		VramKnown:      d.VramKnown,
		VramUnified:    d.VramUnified,
		VramTotalGb:    d.VramTotalGb,
		VramUsedGb:     d.VramUsedGb,
		VramFreeGb:     d.VramFreeGb,
		LastSeen:       time.Now(),
	}
}

// GetAllModels 获取所有已知的模型列表（从快照中提取）。
func (s *Store) GetAllModels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	modelSet := make(map[string]bool)
	for _, snap := range s.snapshots {
		for _, m := range snap.Models {
			modelSet[m] = true
		}
	}

	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	return models
}

// CreateTask 创建新任务。
func (s *Store) CreateTask(model, machine string) TaskRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextTaskID++
	task := TaskRequest{
		ID:        fmt.Sprintf("task-%d", s.nextTaskID),
		Model:     model,
		Machine:   machine,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	s.tasks = append(s.tasks, task)
	return task
}

// GetTasks 获取所有任务。
func (s *Store) GetTasks() []TaskRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TaskRequest, len(s.tasks))
	copy(result, s.tasks)
	return result
}

// MachineCount 返回已报告的机器数量。
func (s *Store) MachineCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}
