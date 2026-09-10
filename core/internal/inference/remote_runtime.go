package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// RemoteRuntime 远端推理运行时（X3/mini）。
// 进程由远端 agent 管理，Runtime 只做 HTTP 转发 + 快照读取。
type RemoteRuntime struct {
	Host    string // 机器名（x3, mini1...）
	URL     string // 转发 URL（http://{ip}:{port}/infer）
	Token   string // X-Auth-Token
	store   *store.Store
	client  *http.Client
	timeout time.Duration
}

// NewRemoteRuntime 创建 RemoteRuntime。
func NewRemoteRuntime(host, url, token string, st *store.Store) *RemoteRuntime {
	return &RemoteRuntime{
		Host:    host,
		URL:     url,
		Token:   token,
		store:   st,
		client:  &http.Client{Timeout: 10 * time.Minute},
		timeout: 10 * time.Minute,
	}
}

// Load 远端无加载概念（agent 按需加载）——no-op。
func (r *RemoteRuntime) Load(ctx context.Context, model, file string, memGB int) error {
	return nil
}

// Unload 远端无卸载概念——no-op。
func (r *RemoteRuntime) Unload() error {
	return nil
}

// IsReady 从 fleet 快照判断是否健康。
func (r *RemoteRuntime) IsReady() bool {
	s := r.Snapshot()
	return s != nil && s.Healthy
}

// State 从快照返回状态。
func (r *RemoteRuntime) State() string {
	s := r.Snapshot()
	if s == nil {
		return "unknown"
	}
	return s.BackendState
}

// Snapshot 从 store 的 FleetSnapshot 读取。
func (r *RemoteRuntime) Snapshot() *Snapshot {
	if r.store == nil {
		return nil
	}
	s := r.store.GetSnapshot(r.Host)
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
		ActiveReqs:   s.ActiveRequests,
		Uptime:       s.Uptime,
	}
}

// Infer 转发推理请求到远端 agent。
func (r *RemoteRuntime) Infer(ctx context.Context, model string, body []byte, path string) (*Response, error) {
	// 构造转发 body：加 _path 字段（agent 按端点转发）
	var reqMap map[string]interface{}
	if err := json.Unmarshal(body, &reqMap); err != nil {
		return nil, fmt.Errorf("解析请求体失败: %w", err)
	}
	reqMap["_path"] = path
	forwardBody, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 独立超时 context（客户端断开不中断后端推理）
	forwardCtx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(forwardCtx, "POST", r.URL, bytes.NewBuffer(forwardBody))
	if err != nil {
		return nil, fmt.Errorf("创建转发请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.Token != "" {
		req.Header.Set("X-Auth-Token", r.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("转发请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return &Response{
		Body:        respBody,
		ContentType: resp.Header.Get("Content-Type"),
		Headers:     map[string][]string(resp.Header),
	}, nil
}

// Ping 探测远端可达性（轻量，1s 超时）。
func (r *RemoteRuntime) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pingCtx, "GET", r.URL, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
