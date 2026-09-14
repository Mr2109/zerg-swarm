// Package heartbeat 提供子端到主控的心跳上报。
//
// 职责：
//   - 每 5 秒 POST {controller}/api/fleet/heartbeat
//   - 携带后端状态快照
//   - 失败时重试 3 次，防止偶发网络错误导致主控判离线
//
// 两条诚实纪律（《设计-资源管理器》§3.1/§八 Q4/Q6）：
//   - gpu_used_gb 只放**真实显存**；拿不到就不出现该字段（vram_known=false），**绝不用进程 RSS 冒充**。
//   - active_requests 放**真实在飞计数**；没有计数来源就不出现，绝不写死 0。
package heartbeat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/version"
)

// backendSource 心跳需要的后端事实来源（接口便于测试注入假值）。
type backendSource interface {
	CurrentModel() string
	RegistryNames() []string
	State() string
	BackendRssGb() float64
	IsHealthy() bool
	ResidentDetail() []backend.ResidentDetail
}

// vramSource 真实显存来源；ok=false 表示该平台/该机器拿不到显存（绝不冒充）。
type vramSource interface {
	VramUsedGb() (float64, bool)
	VramTotalGb() (float64, bool)
}

// ActiveCounter 真实在飞请求计数来源（由 server.Agent 实现）。
type ActiveCounter interface {
	ActiveRequests() int
}

// Runner 心跳上报器。
type Runner struct {
	controller string
	token      string
	machine    string
	backend    backendSource
	sampler    *monitor.Sampler
	vram       vramSource
	active     ActiveCounter
	startedAt  time.Time
	interval   time.Duration
	stopCh     chan struct{}
	httpClient *http.Client
}

// NewRunner 创建心跳上报器。
//   - active：提供真实在飞请求计数（nil 则该字段不出现，绝不写死 0）。
func NewRunner(controller, token, machine string, mgr *backend.Manager, smp *monitor.Sampler, active ActiveCounter) *Runner {
	return &Runner{
		controller: controller,
		token:      token,
		machine:    machine,
		backend:    mgr,
		sampler:    smp,
		vram:       smp,
		active:     active,
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
	body := r.buildBody()

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

// buildBody 组装心跳体（无 IO 副作用，便于测试断言字段）。
//
// 字段口径：
//   - 旧字段语义不变：machine/backend_state/model/mem_*/load/models/uptime/
//     backend_rss_gb/healthy/error/cpu_pct/gpu_pct/code_version/code_sha。
//   - active_requests：真实在飞计数（有来源才出现）。
//   - gpu_used_gb：真实显存（拿得到才出现）；配套 vram_known/vram_used_gb/vram_total_gb/vram_free_gb。
//   - resident[]：驻留明细（托管项 managed=true；探测到的未托管项 managed=false）。
//   - unmanaged[]：未托管但占着端口的进程（只读上报，不接管不杀）。
func (r *Runner) buildBody() map[string]interface{} {
	body := map[string]interface{}{
		"machine":          r.machine,
		"backend_state":    r.backend.State(),
		"model":            r.backend.CurrentModel(),
		"mem_available_gb": r.sampler.MemAvailableGb(),
		"mem_total_gb":     r.sampler.MemTotalGb(),
		"gpu_temp_c":       r.sampler.GpuTempC(),
		"load":             r.sampler.LoadAvg(),
		"models":           r.backend.RegistryNames(),
		"uptime":           time.Since(r.startedAt).Seconds(),
		"backend_rss_gb":   r.backend.BackendRssGb(),
		"healthy":          r.backend.IsHealthy(),
		"error":            nil,
		// B4 v2：CPU/GPU 使用率（主控监看展示）
		"cpu_pct": r.sampler.CpuPct(),
		"gpu_pct": r.sampler.GpuPct(),
		// L3 自动升级：子端自报代码身份（版本矩阵的数据来源）
		"code_version": version.Version,
		"code_sha":     version.Commit,
	}

	// active_requests：真实在飞请求计数。没有来源就不出现——绝不写死 0（旧行为已修）。
	if r.active != nil {
		body["active_requests"] = r.active.ActiveRequests()
	}

	// gpu_used_gb：真实显存占用。拿不到显存 → 该字段**缺席** + vram_known=false；
	// 绝不再用进程 RSS 冒充（旧行为已修）。backend_rss_gb 仍是真 RSS，两者不再混同。
	if r.vram != nil {
		if usedGb, ok := r.vram.VramUsedGb(); ok {
			body["gpu_used_gb"] = round1(usedGb)
			body["vram_used_gb"] = round1(usedGb)
			body["vram_known"] = true
			if totalGb, okTotal := r.vram.VramTotalGb(); okTotal {
				body["vram_total_gb"] = round1(totalGb)
				if free := totalGb - usedGb; free >= 0 {
					body["vram_free_gb"] = round1(free)
				}
			}
		} else {
			// 拿不到显存：如实标不可用，不出假值。
			body["vram_known"] = false
		}
	}

	// resident[]：驻留明细，全部来自后端管理器（托管项 managed=true）。
	// （unmanaged[] 未托管探测已随 P4 退场清理删除——卵之外无引擎，附录 C·C7；
	//   心跳里不再有"外部服务"这一类上报对象。）
	resident := r.backend.ResidentDetail()
	if len(resident) > 0 {
		body["resident"] = resident
	}

	return body
}

// round1 四舍五入到 1 位小数（与 monitor 采样口径一致）。
func round1(v float64) float64 { return math.Round(v*10) / 10 }
