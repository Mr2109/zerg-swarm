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

// engineLiveSource 引擎实时态来源（由 backend.Manager 实现）。
//
// 与 ActiveCounter/vramSource 同纪律：**没有来源就不出现**该字段，绝不写死空数组。
// GAP-20260925-45：主控此前只能看到子端自报的 healthy/backend_state，看不到
// 「此刻在不在算 / 有没有输出 / 卡没卡」；本来源把每枚引擎的实时事实带进心跳。
type engineLiveSource interface {
	EngineLiveStates() []backend.EngineLive
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
	engineLive engineLiveSource
	startedAt  time.Time
	interval   time.Duration
	stopCh     chan struct{}
	httpClient *http.Client
}

// NewRunner 创建心跳上报器。
//   - active：提供真实在飞请求计数（nil 则该字段不出现，绝不写死 0）。
func NewRunner(controller, token, machine string, mgr *backend.Manager, smp *monitor.Sampler, active ActiveCounter) *Runner {
	r := &Runner{
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
	// 引擎实时态来源：backend.Manager 实现该接口 ⇒ 接线；否则该块缺席（不写死）。
	// （NewRunner 签名不动 —— 调用方 agentd 属主 agent/cmd/zerg-agentd/main.go。）
	if ls, ok := any(mgr).(engineLiveSource); ok {
		r.engineLive = ls
	}
	return r
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
//   - origin_sha：本机二进制**源提交**（构建期注入的 `GitOrigin-RevId` · GAP-20261013-09）——
//     与 code_sha（镜像 tip）分属两套 git 命名空间，本格是主控「同源判定」唯一的可比键；
//     未注入 ⇒ 空串 = 如实报「判不了」（✗ 不拿 code_sha 顶上冒充）。
//   - active_requests：真实在飞计数（有来源才出现）。
//   - gpu_used_gb：真实显存（拿得到才出现）；配套 vram_known/vram_used_gb/vram_total_gb/vram_free_gb。
//   - resident[]：驻留明细（托管项 managed=true；探测到的未托管项 managed=false）。
//   - unmanaged[]：未托管但占着端口的进程（只读上报，不接管不杀）。
//   - engines[]：引擎实时态（在算否/输出停滞/看门狗判词；有来源才出现）。
func (r *Runner) buildBody() map[string]interface{} {
	// 身份三格的真源（缺口 GAP-20261018-20）：本进程自己的二进制身份 —— 只读、进程内缓存一次。
	self := version.ReadSelf()
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
		// L3 自动升级：子端自报代码身份（版本矩阵的数据来源）。
		// ★ 缺口 `GAP-20261018-20`（身份同步半）：三格改从**正在跑的这一件二进制自己**读
		//   （`version.ReadSelf`：嵌入 tags > VCS 戳 > ldflags 回落）—— 旧写法直接报
		//   `version.Commit`（只在构建期 `-ldflags -X` 注入）⇒ 手搓构建上报 `unknown`，
		//   而脏树两次构建报**同一个** `<sha>+dirty` ⇒ 换件前后 `code_sha` 逐字相同、
		//   运维面判不出新件跑没跑。改后 `code_sha` 至少随「这一件」走。
		"code_version":    self.Version,
		"code_sha":        self.Commit,
		"code_build_time": self.BuildTime,
		// code_digest：自身可执行件 sha256 前 16 位（**内容身份**）—— 同提交的两份不同
		// 二进制只有它能分开。★ 主控今天的 `HeartbeatRequest` **不收这一格**（`core/internal/api`
		// 的结构体只取已知键）⇒ 它现在只对**直读子端 `/status`** 的换件核对有用，不进 `agent ls`；
		// ✗ 不把「报了」当「主控存了」。
		"code_digest": self.Digest,
		// 同源判定（GAP-20261013-09 · 候选 c）：本机二进制**源提交** —— 构建期注入的
		// `GitOrigin-RevId` trailer（镜像树）或 HEAD（私有树）。它与 code_sha 分属两套 git
		// 命名空间（私有提交 ⟷ 镜像 tip），本格是主控拼「同源/混版/判不了」唯一的可比键。
		// 未注入 ⇒ 空串 = 判不了（✗ 不回落成 code_sha —— 两套命名空间混成一句就是假判决）。
		"origin_sha": version.Origin,
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

	// engines[]：引擎实时态（GAP-20260925-45）—— 每枚托管引擎的在飞计数 + 看门狗判词/
	// 输出停滞/工时增量。主控据此区分「挂住（stuck_*）」与「只是慢（有输出/工时在涨）」。
	// 无来源、或本端无托管引擎 ⇒ 该块缺席（与 active_requests 同纪律：**绝不写死空数组**）。
	if r.engineLive != nil {
		if live := r.engineLive.EngineLiveStates(); len(live) > 0 {
			body["engines"] = live
		}
	}

	return body
}

// round1 四舍五入到 1 位小数（与 monitor 采样口径一致）。
func round1(v float64) float64 { return math.Round(v*10) / 10 }
