// package store 提供内存中的 fleet 状态存储和任务管理。
package store

import (
	"fmt"
	"sync"
	"time"
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
	GpuUsedGb      float64  `json:"gpu_used_gb"`      // GPU 已用内存（GB）
	GpuTempC       float64  `json:"gpu_temp_c"`       // GPU 温度（°C），0 表示无数据
	BackendRssGb   float64  `json:"backend_rss_gb"`   // 后端进程 RSS 内存（GB）
	ActiveRequests int      `json:"active_requests"`  // 当前活跃请求数
	Healthy        bool     `json:"healthy"`          // 是否健康
	BackendState   string   `json:"backend_state"`    // 后端状态（如 ready, loading, error）
	Error          *string  `json:"error"`            // 错误信息（可为 null）
	CpuPct         float64  `json:"cpu_pct"`          // B4 v2：CPU 使用率 %
	GpuPct         float64  `json:"gpu_pct"`          // B4 v2：GPU 使用率 %
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
	CpuPct   float64   `json:"cpu_pct"`
	GpuPct   float64   `json:"gpu_pct"`
	Error    *string   `json:"error,omitempty"`
	LastSeen time.Time `json:"last_seen"` // 最后心跳时间
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

// SetLocalSnapshot 写入本机（local）快照——B13 修复：本机不跑独立 agent 心跳，
// 由主控周期任务把 LocalBackend 状态写入 store（路由打分需要 local 的健康/加载状态）。
func (s *Store) SetLocalSnapshot(machine string, model *string, models []string, healthy bool, state string, memAvail, memTotal, load float64, active int, cpuPct, gpuPct float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[machine] = &FleetSnapshot{
		Machine:        machine,
		Model:          model,
		Models:         models,
		MemAvailableGb: memAvail,
		MemTotalGb:     memTotal,
		Load:           load,
		Healthy:        healthy,
		BackendState:   state,
		ActiveRequests: active,
		CpuPct:         cpuPct,
		GpuPct:         gpuPct,
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
