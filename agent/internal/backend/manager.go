// Package backend 提供后端进程生命周期管理。
//
// 职责：
//   - 根据模型类型启动 llama-server 或 ds4-server
//   - 动态端口分配（9000-9999）
//   - 健康检查（交替探测 /health 和 /v1/models）
//   - 崩溃自愈（重试 3 次后熔断）
//   - 多模型驻留 + 五档裁决淘汰（§八 Q1 默认单槽 / Q2 五档；裁决逻辑在共享包 shared/resources，
//     本包只负责"把状态翻译成入参 + 执行裁决"——见 residency.go）
//   - 请求合并（同模型共享加载槽）
//   - 内存预算检查 + 只卸够腾空间（fail-closed：腾不出缺口就 507 拒装）
//   - 优雅停止（SIGTERM → 等 5s → SIGKILL）
package backend

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// 状态机常量
const (
	StateIdle     = "idle"
	StateLoading  = "loading"
	StateReady    = "ready"
	StateCrashed  = "crashed"
	StateSleeping = "sleeping"
	// P2（设计 §7.7 修补 1）：显式状态「空窗计时中」与「draining」——定义在 p2_lifecycle.go。
	// 状态串：ready → 空窗计时中 → draining → stopped。
)

// subproc 单个模型的后端进程状态。
type subproc struct {
	proc     *exec.Cmd
	port     int
	model    string
	entry    *registry.ModelEntry
	state    string    // ready / loading / crashed / sleeping / idle_armed(空窗计时中) / draining
	failCnt  int       // 连续健康检查失败次数
	lastUsed time.Time // 最近使用时间（LRU）；P2 起兼作「最后一次活动时间戳」（§7.7 修补 2）
	reqCount int       // 活跃请求数（旧口径：InferForward 延迟减计数的观测面）
	// inflight 在飞引用计数（P2，设计 §7.7 修补 3）——「在飞」的唯一真源：
	// 请求进入生成中 +1（acquireInflight）、完成/失败 −1（releaseInflight）。
	// 卸载/切换判据一律取它；引擎 /slots 只作交叉校验，不作为条件。
	inflight int
	// pinUntil Q5：显式 pin 的到期时刻（零值 = 未 pin）。无 TTL 的 pin 不允许——见 Manager.Pin。
	pinUntil time.Time
}

// loadWaiter 请求合并：同模型并发请求共享一个加载槽
type loadWaiter struct {
	done   chan struct{}
	result map[string]interface{}
	err    error
}

// Manager 后端管理器，线程安全。多模型驻留 + 五档裁决淘汰（Q1/Q2）。
type Manager struct {
	mu          sync.Mutex
	procs       map[string]*subproc    // model → subproc（多模型驻留）
	loading     map[string]*loadWaiter // model → 正在加载的等待组（请求合并）
	registry    *registry.Registry
	machine     string
	maxResident int // 驻留上限（<=0 视为默认单槽；见 EnvMaxResident / SetMaxResident）
	// idleTTL 空闲 TTL（《设计-资源管理器》§3.3(c) 触发①）：某驻留模型空闲（无在飞请求、
	// 非加载中、pin 未生效）持续超过它即卸载。缺省 = DefaultIdleTTL（300 秒，Mr2109 2026-09-13 拍板）；
	// ZERG_MODEL_TTL_S 覆盖，设 0 = 关闭；<=0 一律视为未启用。
	idleTTL time.Duration
	// reaperStop 后台 TTL 回收循环的停止信号（nil = 未启动）。读写都在 m.mu 下。
	reaperStop chan struct{}
	// stopHook（P2，测试注入）：stopSubproc 执行真正停进程动作前回调（nil = 无回调）。
	// 用途：验收「锁内置 draining ⇒ 出锁后才停进程」（§7.7 修补 3 的可观测面）。
	// 回调里如需读状态须自行加锁（回调发生在锁外）。
	stopHook func(sp *subproc)
	// waitQ 等待队列（P7：切换期到达的请求挂起在此，出队须重校验当前卵）。
	// ⚠ 锁序：任何持 m.mu 的路径都不得调用 WaitQ* 方法（见 waitqueue.go 文件头不变式）。
	waitQ *p2Queue
}

// defaultReapInterval 是后台 TTL 回收循环的扫描间隔（只决定"多久查一次"，不是 TTL 本身）。
const defaultReapInterval = 30 * time.Second

// NewManager 创建后端管理器。驻留上限取自 ZERG_MAX_RESIDENT（默认单槽，Q1）；
// 空闲 TTL 取自 ZERG_MODEL_TTL_S（缺省 300 秒，见 DefaultIdleTTL；设 0 关闭）。TTL 启用时同时启动后台回收循环。
func NewManager(reg *registry.Registry, machine string) *Manager {
	m := &Manager{
		procs:       make(map[string]*subproc),
		loading:     make(map[string]*loadWaiter),
		registry:    reg,
		machine:     machine,
		maxResident: resolveMaxResident(),
		idleTTL:     resolveIdleTTL(),
	}
	// TTL 未启用（idleTTL<=0，例如显式设成 0）→ StartIdleReaper 直接返回，不产生后台 goroutine。
	m.StartIdleReaper(defaultReapInterval)
	return m
}

// Start 启动/复用模型对应的后端进程（多模型驻留）。
// - 模型已在驻留列表（ready）→ 直接复用（更新 lastUsed）
// - 模型正在加载（其他请求已触发）→ 等待同一加载槽（请求合并）
// - 不在驻留列表 → 加载新进程；驻留数超上限 → LRU 卸载
func (m *Manager) Start(modelName string) (map[string]interface{}, error) {
	m.mu.Lock()

	if needUnload, blocked := m.RequestModel(modelName); needUnload || blocked {
		// 异模型请求与当前卵冲突（§7.7 末条）：
		//   - needUnload=true：已锁内赢权置 draining——先停掉旧卵，再走正常孵化装新模型；
		//   - blocked=true：旧卵有在飞/pin/外部——不打断在飞，请求进等待队列（挂起，Q5）。
		if needUnload {
			m.mu.Lock()
			var victimName string
			var victimSP *subproc
			for name, sp := range m.procs {
				if sp.state == StateDraining {
					victimName, victimSP = name, sp
					break
				}
			}
			var victimProc *exec.Cmd
			if victimSP != nil {
				victimProc = victimSP.proc
			}
			m.mu.Unlock()
			if victimSP != nil {
				m.stopVictimProcess(victimProc, victimSP)
				m.mu.Lock()
				if cur, ok := m.procs[victimName]; ok && cur == victimSP {
					delete(m.procs, victimName)
				}
				m.mu.Unlock()
				log.Printf("[backend] 提前收卵完成: %s（异模型请求 %s 可孵化）", victimName, modelName)
			}
			// 落到下方正常 Start（孵化）路径
		} else {
			// blocked：等待当前卵在飞清零（低频轮询重校验——禁 sleep 定时器直接动手；
			// 这里只等「可孵化」信号，收卵由巡检/后续请求完成）。上限 = 上行超时。
			deadline := time.Now().Add(120 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(500 * time.Millisecond)
				if needU, blk := m.RequestModel(modelName); !blk {
					if needU {
						// 赢权成功——跳出去走孵化（递归一次，收卵段同上）
						m.mu.Unlock()
						return m.Start(modelName)
					}
					break
				}
			}
			// 等不到就继续往下走正常 Start（内部按红线拒装/拒孵，明确报错不硬来）
		}
	}

	// 查找模型配置
	entry, ok := m.registry.Get(modelName)
	if !ok {
		m.mu.Unlock()
		return errResponse(404, "unknown model", modelName), nil
	}

	// 已驻留且就绪 → 直接复用
	if sp, ok := m.procs[modelName]; ok && sp.state == StateReady {
		sp.lastUsed = time.Now()
		sp.reqCount++
		sp.inflight++ // P2（§7.7 修补 3）：在飞引用计数 +1（唯一真源）；对应 release 在 doStart 失败路径与调用方
		m.mu.Unlock()
		log.Printf("[backend] 模型 %s 已驻留，直接复用 (port=%d)", modelName, sp.port)
		return okResponse(modelName, entry.Backend, sp.port), nil
	}

	// 请求合并：该模型正在加载 → 等待同一加载槽
	if w, ok := m.loading[modelName]; ok {
		m.mu.Unlock()
		log.Printf("[backend] 模型 %s 正在加载，等待共享加载槽 (请求合并)", modelName)
		<-w.done
		if w.err != nil {
			return nil, w.err
		}
		return w.result, nil
	}

	// 无驻留且无加载中 → 自己触发加载
	w := &loadWaiter{done: make(chan struct{})}
	m.loading[modelName] = w
	m.mu.Unlock()

	result, err := m.doStart(modelName, entry)

	m.mu.Lock()
	delete(m.loading, modelName)
	close(w.done)
	w.result = result
	w.err = err
	m.mu.Unlock()
	return result, err
}

// doStart 实际执行加载流程（调用方需已持有加载槽）。
func (m *Manager) doStart(modelName string, entry *registry.ModelEntry) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// P1 接线（最先做）：适配器声明「本架构必须由非主线引擎承载」⇒ 卵声明必须带 cmd:
	// （专用 fork + 包装脚本含 env 处理），缺失即拒孵（502 语义），绝不静默退回主线
	// llama-server。放在一切裁决之前：声明校验属于「孵化前校验」（设计 §6.8.4），
	// 不该被内存采样/端口等因素抢先返回别的错误码。
	// 依据：设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1（fleet.yaml 无 cmd: 时第二台
	// 设备孵 K2 会静默退回主线 llama-server ⇒ 起不来或误链且不报错）。
	adpEarly := modeladapter.Dispatch(modelName)
	if nonMainline, ok := adpEarly.(modeladapter.NonMainlineEngine); ok && nonMainline.RequiresNonMainlineEngine() && entry.Cmd == "" {
		log.Printf("[backend] ✗ 拒孵 %s：该架构必须由非主线引擎承载（专用 fork + 包装脚本），卵声明缺少 cmd: 字段", modelName)
		return errResponse(502, "engine implementation missing",
			fmt.Sprintf("model %s requires a non-mainline engine (cmd: field in the egg manifest); refusing to silently fall back to the mainline llama-server", modelName)), nil
	}

	// M10「身份一致直接复用基线服务」已随 P4 退场清理删除（设计 §10.2 F1 / 附录 C·C2）：
	// 卵之外无引擎（§1.3）——子端之外的推理进程不是后端，不再登记复用。

	// 驻留超限 → 五档裁决淘汰（Q1 默认单槽 / Q2 五档）
	m.evictIfNeededLocked()

	// P1：跨服务类型强制清场（llama xor ds4）——受管侧保证「同一时间只有一种服务在跑」
	// （设计 §7 S2）。异类且可动作 ⇒ 卸下；异类被红线挡住（在飞/pin/加载中）⇒ 拒装，
	// 绝不硬来。未托管的手工服务不在此列（走 baseline/借用机制，见设计 §9）。
	kind := serviceKind(entry)
	if ev, blocked := m.evictOtherKindsLocked(modelName, kind); len(ev) > 0 || len(blocked) > 0 {
		log.Printf("[backend] 服务类型互斥(%s): 卸下=%v 红线挡住=%v", kind, ev, blocked)
		if len(blocked) > 0 {
			return errResponse(507, "service-kind conflict",
				fmt.Sprintf("另一类服务(%s)仍有受保护驻留: %v；本机同一时间只允许一种推理服务在跑",
					kind, blocked)), nil
		}
	}

	// 内存预算检查（T3 预检，借鉴 llama_cpp_router willModelFit）——fail-closed
	if memRequired := entry.MemGB; memRequired > 0 {
		avail := monitor.DefaultSampler.MemAvailableGb()
		ok, have, need := m.ensureMemoryForLocked(memRequired, avail)
		if !ok {
			// 腾不出缺口 → 拒装（507 语义不变）：不许赌"应该能跑"。
			return m.rejectInsufficientMemory(have, need), nil
		}
		log.Printf("[backend] 内存预算检查通过: avail=%.1fGB, required=%.1fGB", have, need)
	}

	// 设置 loading 状态
	sp := &subproc{
		model:    modelName,
		entry:    entry,
		state:    StateLoading,
		lastUsed: time.Now(),
	}
	m.procs[modelName] = sp

	// 动态端口分配
	port := m.findFreePort()
	if port == 0 {
		sp.state = StateCrashed
		return errResponse(500, "no free port", ""), nil
	}
	sp.port = port

	// 构建启动命令（模型适配层：按模型名选适配器，管理启动参数/工具风格/重提示）
	// （P1 的「非主线引擎必须带 cmd:」守卫在 doStart 最前面，见函数开头。
	//  批 1：构造整段抽成 buildEngineArgv —— 孵化路径（hatch_spec.go）要用**同一份**参数，
	//  两处各写一遍必然漂移；此处逐字保持原逻辑，行为不变。）
	cmdPath, execArgs := buildEngineArgv(modelName, entry, port)

	cmd := exec.Command(cmdPath, execArgs...)
	sp.proc = cmd
	log.Printf("[backend] spawn 命令: %s %v", cmdPath, execArgs)

	stderrPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		sp.state = StateCrashed
		return errResponse(500, "failed to start backend", err.Error()), nil
	}
	if stderrPipe != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderrPipe.Read(buf)
				if n > 0 {
					log.Printf("[backend] 后端stderr: %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()
	}

	pid := cmd.Process.Pid
	log.Printf("[backend] 启动 %s: pid=%d, port=%d, model=%s", entry.Backend, pid, port, modelName)

	// 等待健康检查通过
	if err := m.waitForReady(sp); err != nil {
		log.Printf("[backend] 健康检查失败: %v", err)
		m.stopSubproc(sp)
		sp.state = StateCrashed
		return errResponse(500, "health check failed", err.Error()), nil
	}

	// M3：**功能预检** —— "加载成功 ≠ 可用"（今天 502 的直接成因）。
	// 引擎自己还有守卫：`rocm prefill failed` 正是在"装好了"之后才发生。
	// 判据必须是一次**真实生成**（max_tokens=1）：只看 /health 只能证明端口活。
	if probeErr := probeInference(port, modelName, probeTimeout()); probeErr != nil {
		log.Printf("[backend] 功能预检未通过: model=%s port=%d: %v", modelName, port, probeErr)
		// 设计 §11 M3 ③：明确报错 + **归还**（停掉本端起的进程），绝不对外声称已就绪。
		m.stopSubproc(sp)
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "model unusable after load (functional probe failed)", probeErr.Error()), nil
	}
	log.Printf("[backend] 功能预检通过: model=%s port=%d", modelName, port)

	// 就绪
	sp.failCnt = 0
	sp.state = StateReady
	log.Printf("[backend] 后端就绪: port=%d, model=%s", port, modelName)
	return okResponse(modelName, entry.Backend, port), nil
}

// Stop 停止所有后端进程。
func (m *Manager) Stop() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, sp := range m.procs {
		if sp.proc != nil && sp.proc.ProcessState == nil {
			log.Printf("[backend] 停止进程: model=%s", name)
			m.stopSubproc(sp)
		}
		delete(m.procs, name)
	}
	return okResponse("", "", 0)
}

// State 返回主状态（有 ready 进程即 ready，否则 idle）。
func (m *Manager) State() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady {
			return StateReady
		}
		if sp.state == StateLoading {
			return StateLoading
		}
	}
	return StateIdle
}

// CurrentModel 返回当前加载的模型名（驻留列表中最新的）。
func (m *Manager) CurrentModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var newest string
	var newestTime time.Time
	for _, sp := range m.procs {
		if sp.state == StateReady {
			if newest == "" || sp.lastUsed.After(newestTime) {
				newest = sp.model
				newestTime = sp.lastUsed
			}
		}
	}
	return newest
}

// CurrentBackend 返回主后端的类型。
func (m *Manager) CurrentBackend() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady && sp.entry != nil {
			return sp.entry.Backend
		}
	}
	return ""
}

// CurrentPort 返回主后端的端口。
func (m *Manager) CurrentPort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady {
			return sp.port
		}
	}
	return 0
}

// IsHealthy 检查后端是否健康（任一驻留模型健康即可）。
func (m *Manager) IsHealthy() bool {
	m.mu.Lock()
	var candidates []*subproc
	for _, sp := range m.procs {
		if sp.state == StateReady && sp.port > 0 {
			candidates = append(candidates, sp)
		}
	}
	m.mu.Unlock()

	for _, sp := range candidates {
		if m.healthCheck(sp) {
			return true
		}
	}
	return false
}

// IsModelHealthy 检查指定模型是否健康。
func (m *Manager) IsModelHealthy(model string) bool {
	m.mu.Lock()
	sp, ok := m.procs[model]
	m.mu.Unlock()
	if !ok || sp.state != StateReady {
		return false
	}
	return m.healthCheck(sp)
}

// InferForward 转发推理请求到指定模型的后端。
// 若后端处于 crashed 状态，自动重置熔断，由上层 Load 流程重新启动。
// v2.5.6 治本（2026-08-28——x3 幽灵请求）: ctx 参数——客户端断开时取消后端请求——释放单槽
func (m *Manager) InferForward(ctx context.Context, model string, path string, body []byte) (*http.Response, error) {
	m.mu.Lock()
	sp, ok := m.procs[model]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("模型 %s 未驻留", model)
	}
	if sp.state == StateCrashed {
		log.Printf("[backend] 检测到 crashed，自动重置熔断，准备重启后端")
		sp.failCnt = 0
		sp.state = StateLoading
	}
	if sp.state != StateReady || sp.port == 0 {
		state := sp.state
		m.mu.Unlock()
		return nil, fmt.Errorf("后端未就绪 (state=%s)", state)
	}
	port := sp.port
	m.mu.Unlock()

	// 请求前健康检查
	if !m.healthCheck(sp) {
		m.mu.Lock()
		sp.failCnt++
		if sp.failCnt >= 3 {
			log.Printf("[backend] 熔断触发 (连续失败 %d 次)", sp.failCnt)
			m.stopSubproc(sp)
			sp.state = StateCrashed
			m.mu.Unlock()
			return nil, fmt.Errorf("后端已熔断 (连续失败 %d 次)", sp.failCnt)
		}
		log.Printf("[backend] 健康检查失败 (%d/3)，重试中...", sp.failCnt)
		m.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
		if !m.healthCheck(sp) {
			return nil, fmt.Errorf("健康检查重试仍失败")
		}
		m.mu.Lock()
		sp.failCnt = 0
		m.mu.Unlock()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	// v2.5.6 治本: 用客户端 context 构造请求——断开自动取消（不再等 llama-server 生成完——释放单槽）
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Minute,
		// 治本（2026-08-12）：禁 keep-alive——llama-server 连接空闲被关，
		// agent 复用断连接 → 长响应读断（IncompleteRead——网关 EOF 根因链）
		Transport: &http.Transport{
			DisableKeepAlives: true,
			// v2.5.4.9 首 token 超时（90s——llama-server 卡死检测——active 释放）
			// 知识库经验: 大请求(含tools)→example-35b-v2 38-60s 推理(正常)——60s 误杀——调 90s
			// 真卡死: 90s 无响应头 → 返回错误 → active 释放 → 主控 failover
			ResponseHeaderTimeout: 90 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if !m.healthCheck(sp) {
			m.mu.Lock()
			sp.failCnt++
			if sp.failCnt >= 3 {
				m.stopSubproc(sp)
				sp.state = StateCrashed
			}
			m.mu.Unlock()
		}
		return nil, fmt.Errorf("转发到后端失败: %w", err)
	}

	// 请求成功，更新 lastUsed + 重置失败计数
	m.mu.Lock()
	sp.failCnt = 0
	sp.lastUsed = time.Now()
	sp.reqCount++
	m.mu.Unlock()
	// P2（§7.7 修补 3）：转发返回即本次推理完成 ⇒ 在飞 −1（完成/失败都算完成）。
	// 归零时由 releaseInflight 转「空窗计时中」（只记时间戳，不起定时器）。
	m.releaseInflight(model)
	return resp, nil
}

// RegistryNames 返回注册表模型名列表（心跳上报用）。
func (m *Manager) RegistryNames() []string {
	return m.registry.Names()
}

// BackendRssGb 返回后端进程总内存占用（GB，心跳上报用）。
func (m *Manager) BackendRssGb() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total float64
	for _, sp := range m.procs {
		if sp.proc != nil && sp.proc.Process != nil {
			total += readMacOSRss(sp.proc.Process.Pid)
		}
	}
	return total
}

// ensureMemoryForLocked 内存预检（fail-closed，调用方需持锁）。
//
// 口径：新模型需要 memRequiredGb*1.1；实测可用 availGb 不够时，先按五档裁决**只卸够**
// 缺口（avail - need），仍不够就返回 ok=false（调用方回 507 —— 估不出/腾不出就不装）。
// 返回 (ok, 腾退后的可用量, 需求量)。
//
// availGb 由调用方传入（doStart 传 monitor 实测值）——这样"够不够"的判定可被测试注入，
// 不必去动真机内存。
func (m *Manager) ensureMemoryForLocked(memRequiredGb, availGb float64) (bool, float64, float64) {
	need := memRequiredGb * 1.1
	if availGb >= need {
		return true, availGb, need
	}
	// 只卸够：缺口 = need - avail（不是模型总需求——旧实现传总量，等于多卸）
	freed := m.evictForMemoryLocked(need - availGb)
	have := availGb + freed
	return have >= need, have, need
}

// rejectInsufficientMemory 内存不足的拒装响应（507 insufficient memory——语义与既有实现一致）。
func (m *Manager) rejectInsufficientMemory(have, need float64) map[string]interface{} {
	return errResponse(507, "insufficient memory", fmt.Sprintf("available: %.1f GB, required: %.1f GB", have, need))
}

// evictIfNeededLocked 驻留超限时按五档裁决淘汰（调用方需持锁）。
//
// 上限 = maxResident（默认 1，Q1；配置项 ZERG_MAX_RESIDENT）。淘汰顺序不再只是 LRU，
// 而是共享裁决的五档：① 崩溃/僵尸（本端 procs 里只有我们自己起的进程，未托管者根本不在其中）
// → ③ 空闲中 LRU 最旧、同档权重更大者先（腾得多）→ ⑤ 被别处等待/加载中者最后；
// 在飞请求者（红线①）与 pin 未到期者（红线③）绝不进入列表。
func (m *Manager) evictIfNeededLocked() {
	limit := m.residentLimitLocked()
	if len(m.procs) < limit {
		return
	}
	// 要腾出的槽位数：让新模型进来后不超过上限
	needSlots := len(m.procs) - limit + 1
	// 准入过滤后的可动作清单（在飞/未托管/pin 未到期/加载中已被拦住）
	plan := m.rankActionableLocked()
	if len(plan) == 0 {
		// 无可驱逐项（全在飞 / 全 pin 未到期 / 全加载中）：宁可超限也不杀正在用的模型——
		// 内存预检与 507 兜底（fail-closed），并如实记日志。
		log.Printf("[backend] 驻留超限但无可驱逐项（在飞/pin/加载中受保护）: resident=%d limit=%d", len(m.procs), limit)
		return
	}
	if len(plan) < needSlots {
		log.Printf("[backend] 可驱逐 %d 项 < 需腾 %d 槽：只腾能腾的（受保护项不动）", len(plan), needSlots)
		needSlots = len(plan)
	}
	for _, e := range plan[:needSlots] {
		sp, ok := m.procs[e.Alias]
		if !ok {
			continue
		}
		m.evictSubprocLocked(e.Alias, sp, fmt.Sprintf("五档淘汰(tier=%d,%s)", e.Tier, e.Reason))
	}
}

// evictForMemoryLocked 只卸够：按五档裁决腾出缺口 needGB，返回实际腾出的 GB。
//
// needGB 语义 = "还差多少内存"（不是模型总需求）——调用方传 need - 当前可用，
// 这样"卸一个就够"才成立（旧实现传总量，等于多卸）。
// 在飞请求者、pin 未到期者、加载中者、未托管进程一律不进计划（红线①②③）。
func (m *Manager) evictForMemoryLocked(needGB float64) float64 {
	if needGB <= 0 {
		return 0
	}
	victims := m.evictVictimsLocked(needGB)
	var freed float64
	for _, name := range victims {
		sp, ok := m.procs[name]
		if !ok {
			continue
		}
		freed += m.evictSubprocLocked(name, sp, "内存腾退(只卸够)")
	}
	return freed
}

// 内部：停止进程（SIGTERM → 等 5s → SIGKILL）
func (m *Manager) stopSubproc(sp *subproc) {
	if sp == nil {
		return
	}
	// P2（§7.7 修补 3）：真正动手前的观测钩子——此刻调用方应已在锁内置 draining 并出锁。
	if m.stopHook != nil {
		m.stopHook(sp)
	}
	if sp.proc == nil || sp.proc.Process == nil {
		return
	}

	if err := sp.proc.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[backend] SIGTERM 发送失败: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sp.proc.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[backend] 进程退出: %v", err)
		} else {
			log.Printf("[backend] 进程已优雅退出")
		}
	case <-time.After(5 * time.Second):
		log.Printf("[backend] 等待退出超时 (5s)，发送 SIGKILL")
		sp.proc.Process.Kill()
		sp.proc.Wait()
	}

	sp.proc = nil
	sp.port = 0
}

// 内部：健康检查（交替探测 /health 和 /v1/models）
func (m *Manager) healthCheck(sp *subproc) bool {
	if sp == nil || sp.port == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", sp.port)
	resp, err := http.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}

	url = fmt.Sprintf("http://127.0.0.1:%d/v1/models", sp.port)
	resp, err = http.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}
	return false
}

// 内部：等待后端就绪（最多 120 秒）
func (m *Manager) waitForReady(sp *subproc) error {
	const timeout = 120 * time.Second
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		if m.healthCheck(sp) {
			return nil
		}
		if sp.proc != nil && sp.proc.ProcessState != nil {
			return fmt.Errorf("后端进程已退出")
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("等待后端就绪超时 (%s)", timeout)
}

// EnvPortPool 子端专用端口区间（如 "9400-9999"）。未设置时用默认 9000-9999。
const EnvPortPool = "ZERG_PORT_POOL"

// findFreePort 分配空闲端口。
//
// 可用 ZERG_PORT_POOL="9400-9999" 把子端完全隔离到专用区间。
// （baseline 声明端口的那道排除已随 P4 退场清理删除——卵之外无引擎后不存在
// "手工服务的坑"，附录 C·C8。）
func (m *Manager) findFreePort() int {
	lo, hi := 9000, 9999
	if v := strings.TrimSpace(os.Getenv(EnvPortPool)); v != "" {
		if a, b, ok := strings.Cut(v, "-"); ok {
			if n1, err1 := strconv.Atoi(strings.TrimSpace(a)); err1 == nil {
				if n2, err2 := strconv.Atoi(strings.TrimSpace(b)); err2 == nil && n1 > 0 && n2 <= 65535 && n1 <= n2 {
					lo, hi = n1, n2
				}
			}
		}
	}
	for port := lo; port <= hi; port++ {
		// 跳过已被本管理器其他模型占用的端口
		taken := false
		for _, sp := range m.procs {
			if sp.port == port {
				taken = true
				break
			}
		}
		if taken {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port
	}
	return 0
}

// 内部：设置状态（旧接口保留，用于兼容）
func (m *Manager) setState(s string) {
	// no-op：状态现在由各 subproc 管理
}

// detectLlamaServerPath 探测 llama-server 可执行文件路径。
func detectLlamaServerPath() string {
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	candidates := []string{
		"/opt/homebrew/bin/llama-server", // macOS homebrew
		"/usr/local/bin/llama-server",    // Linux
		// X3——2026-08-30 统一单一库（build-hip-flash：支持 qwen4exp/PLE——旧 build-hip 已退役）
		"/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "llama-server"
}

// 内部：构造响应
func okResponse(model, backend string, port int) map[string]interface{} {
	return map[string]interface{}{
		"ok":      true,
		"model":   model,
		"backend": backend,
		"port":    port,
	}
}

func errResponse(status int, code, msg string) map[string]interface{} {
	return map[string]interface{}{
		"ok":     false,
		"status": status,
		"code":   code,
		"error":  msg,
	}
}

// 兼容旧接口：readMacOSRss 用 ps -o rss=<pid> 读 RSS（kB）转 GB。
func readMacOSRss(pid int) float64 {
	out, err := exec.Command("/bin/ps", "-o", "rss=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0
	}
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return kb / 1024 / 1024
}
