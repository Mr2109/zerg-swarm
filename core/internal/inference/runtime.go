package inference

import "context"

// Runtime 推理运行时接口，管理进程生命周期。
type Runtime interface {
	Load(ctx context.Context, model, file string, memGB int) error
	Unload() error
	IsReady() bool
	State() string
	Snapshot() *Snapshot
}

// Transport 推理执行接口，薄层发送请求。
type Transport interface {
	Infer(ctx context.Context, model string, body []byte, path string) (*Response, error)
	Ping(ctx context.Context) error
}

// Response 推理响应。
type Response struct {
	Body        []byte
	ContentType string
	Usage       *Usage
	Headers     map[string][]string
}

// Usage token 用量统计。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Snapshot 运行时快照。
type Snapshot struct {
	Host         string  `json:"host"`
	Model        string  `json:"model,omitempty"`
	BackendState string  `json:"backend_state"`
	Healthy      bool    `json:"healthy"`
	MemAvailable float64 `json:"mem_available_gb"`
	MemTotal     float64 `json:"mem_total_gb"`
	Load         float64 `json:"load"`
	GpuUsed      float64 `json:"gpu_used_gb"`
	GpuTemp      float64 `json:"gpu_temp_c"`
	BackendRss   float64 `json:"backend_rss_gb"`
	ActiveReqs   int     `json:"active_requests"`
	Uptime       float64 `json:"uptime"`
}
