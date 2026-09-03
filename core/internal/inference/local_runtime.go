package inference

import (
	"context"
	"zerg/core/internal/localback"
)

// LocalRuntime 本机推理运行时，封装 LocalBackend。
type LocalRuntime struct {
	backend *localback.LocalBackend
	Host    string
}

// NewLocalRuntime 创建 LocalRuntime。
func NewLocalRuntime(logPath string) *LocalRuntime {
	return &LocalRuntime{
		backend: localback.NewLocalBackend(logPath),
		Host:    "local",
	}
}

// Load 加载模型到本机后端。
func (r *LocalRuntime) Load(ctx context.Context, model, file string, memGB int) error {
	// LocalBackend.LoadModel 阻塞等待 ready
	return r.backend.LoadModel(file, memGB)
}

// Unload 卸载本机模型。
func (r *LocalRuntime) Unload() error {
	r.backend.Stop()
	return nil
}

// IsReady 判断是否就绪。
func (r *LocalRuntime) IsReady() bool {
	return r.backend.IsReady()
}

// State 返回状态字符串。
func (r *LocalRuntime) State() string {
	return r.backend.State()
}

// Snapshot 返回快照。
func (r *LocalRuntime) Snapshot() *Snapshot {
	s := r.backend.Snapshot()
	if s == nil {
		return nil
	}
	model := ""
	if s.Model != nil {
		model = *s.Model
	}
	return &Snapshot{
		Host:         r.Host,
		Model:        model,
		BackendState: s.BackendState,
		Healthy:      s.Healthy,
		MemAvailable: s.MemAvailableGb,
		MemTotal:     s.MemTotalGb,
		Load:         s.Load,
		GpuUsed:      s.GpuUsedGb,
		GpuTemp:      s.GpuTempC,
		BackendRss:   s.BackendRssGb,
		ActiveReqs:   0, // 本机单进程，无并发计数
		Uptime:       0, // LocalSnapshot 无 uptime
	}
}
