// Package heartbeat 提供子端到主控的心跳上报。
//
// 职责：
//   - 每 5 秒 POST {controller}/api/fleet/heartbeat
//   - 携带后端状态快照
//   - 失败时重试 3 次，防止偶发网络错误导致主控判离线
package heartbeat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"zerg/agent/internal/backend"
	"zerg/agent/internal/logx"
	"zerg/agent/internal/monitor"
)

// Runner 心跳上报器。
type Runner struct {
	controller string
	token      string
	machine    string
	backend    *backend.Manager
	sampler    *monitor.Sampler
	startedAt  time.Time
	interval   time.Duration
	stopCh     chan struct{}
	httpClient *http.Client
}

// NewRunner 创建心跳上报器。
func NewRunner(controller, token, machine string, mgr *backend.Manager, smp *monitor.Sampler) *Runner {
	return &Runner{
		controller: controller,
		token:      token,
		machine:    machine,
		backend:    mgr,
		sampler:    smp,
		startedAt:  time.Now(),
		interval:   5 * time.Second,
		stopCh:     make(chan struct{}),
		// 禁用代理：子端到主控是局域网直连，系统代理（如 mihomo TUN）会劫持导致 no route
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy: nil,
			},
		},
	}
}

// Start 启动心跳循环。
func (r *Runner) Start() {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				r.send()
			case <-r.stopCh:
				return
			}
		}
	}()
}

// Stop 停止心跳。
func (r *Runner) Stop() {
	close(r.stopCh)
}

// send 发送一次心跳。
func (r *Runner) send() {
	// 构建心跳体（补全字段：内存/显存/温度/负载/模型列表/健康状态）
	curModel := r.backend.CurrentModel()
	models := r.backend.RegistryNames()
	body := map[string]interface{}{
		"machine":          r.machine,
		"backend_state":    r.backend.State(),
		"model":            curModel,
		"mem_available_gb": r.sampler.MemAvailableGb(),
		"mem_total_gb":     r.sampler.MemTotalGb(),
		"gpu_used_gb":      r.backend.BackendRssGb(),
		"gpu_temp_c":       r.sampler.GpuTempC(),
		"load":             r.sampler.LoadAvg(),
		"models":           models,
		"uptime":           time.Since(r.startedAt).Seconds(),
		"active_requests":  0,
		"backend_rss_gb":   r.backend.BackendRssGb(),
		"healthy":          r.backend.IsHealthy(),
		"error":            nil,
		// B4 v2：CPU/GPU 使用率（主控监看展示）
		"cpu_pct": r.sampler.CpuPct(),
		"gpu_pct": r.sampler.GpuPct(),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		logx.Errorf("heartbeat", "序列化心跳失败", "error", err)
		return
	}

	url := fmt.Sprintf("%s/api/fleet/heartbeat", r.controller)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		logx.Errorf("heartbeat", "构造请求失败", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", r.token)

	// 发送，失败重试 3 次
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := r.httpClient.Do(req)
		if err != nil {
			lastErr = err
			logx.Errorf("heartbeat", "心跳失败", "machine", r.machine, "url", url, "attempt", attempt+1, "error", err)
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == 200 {
			logx.Infof("heartbeat", "心跳上报成功", "machine", r.machine, "url", url, "attempt", attempt+1)
			return
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		logx.Warnf("heartbeat", "心跳失败", "machine", r.machine, "attempt", attempt+1, "status", resp.StatusCode)
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		logx.Errorf("heartbeat", "心跳上报最终失败", "machine", r.machine, "url", url, "error", lastErr)
	}
}
