// Package backend 提供后端进程生命周期管理。
//
// 职责：
//   - 根据模型类型启动 llama-server 或 ds4-server
//   - 动态端口分配（9000-9999）
//   - 健康检查（交替探测 /health 和 /v1/models）
//   - 崩溃自愈（重试 3 次后熔断）
//   - 多模型驻留 + LRU 淘汰（借鉴 llama.cpp router mode）
//   - 请求合并（同模型共享加载槽）
//   - 内存预算检查 + 卸载腾空间
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

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// 状态机常量
const (
	StateIdle      = "idle"
	StateLoading   = "loading"
	StateReady     = "ready"
	StateCrashed   = "crashed"
	StateSleeping  = "sleeping"

	// maxResident 模型驻留上限（LRU 淘汰阈值，借鉴 llama.cpp router mode）
	maxResident = 3
)

// subproc 单个模型的后端进程状态。
type subproc struct {
	proc     *exec.Cmd
	port     int
	model    string
	entry    *registry.ModelEntry
	state    string      // ready / loading / crashed / sleeping
	failCnt  int         // 连续健康检查失败次数
	lastUsed time.Time   // 最近使用时间（LRU）
	reqCount int         // 活跃请求数
}

// loadWaiter 请求合并：同模型并发请求共享一个加载槽
type loadWaiter struct {
	done   chan struct{}
	result map[string]interface{}
	err    error
}

// Manager 后端管理器，线程安全。多模型驻留 + LRU 淘汰。
type Manager struct {
	mu       sync.Mutex
	procs    map[string]*subproc   // model → subproc（多模型驻留）
	loading  map[string]*loadWaiter // model → 正在加载的等待组（请求合并）
	registry *registry.Registry
	machine  string
}

// NewManager 创建后端管理器。
func NewManager(reg *registry.Registry, machine string) *Manager {
	return &Manager{
		procs:    make(map[string]*subproc),
		loading:  make(map[string]*loadWaiter),
		registry: reg,
		machine:  machine,
	}
}

// Start 启动/复用模型对应的后端进程（多模型驻留）。
// - 模型已在驻留列表（ready）→ 直接复用（更新 lastUsed）
// - 模型正在加载（其他请求已触发）→ 等待同一加载槽（请求合并）
// - 不在驻留列表 → 加载新进程；驻留数超上限 → LRU 卸载
func (m *Manager) Start(modelName string) (map[string]interface{}, error) {
	m.mu.Lock()

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

	// 驻留超限 → LRU 淘汰（T1）
	m.evictIfNeededLocked()

	// 内存预算检查（T3 预检，借鉴 llama_cpp_router willModelFit）
	memRequired := entry.MemGB
	if memRequired > 0 {
		memAvail := monitor.DefaultSampler.MemAvailableGb()
		// 统计已驻留模型占用
		var residentGB float64
		for _, sp := range m.procs {
			if sp.entry != nil {
				residentGB += sp.entry.MemGB
			}
		}
		availForNew := memAvail + residentGB
		if availForNew < memRequired*1.1 {
			// 尝试卸载旧模型腾空间（LRU 优先）直到能装下（T3）
			freed := m.evictForMemoryLocked(memRequired * 1.1)
			if availForNew+freed < memRequired*1.1 {
				return errResponse(507, "insufficient memory", fmt.Sprintf(
					"available: %.1f GB, required: %.1f GB", availForNew, memRequired*1.1)), nil
			}
		}
		log.Printf("[backend] 内存预算检查通过: avail=%.1fGB, required=%.1fGB",
			availForNew, memRequired*1.1)
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
	adp := modeladapter.Dispatch(modelName)
	cmdArgs := adp.BuildArgs(entry, port)
	cmdPath := ""
	if entry.Backend == "ds4-server" {
		cmdPath = "ds4-server"
	} else {
		cmdPath = detectLlamaServerPath()
	}
	// 替换 {file}/{port} 占位符（适配器可返回占位符）
	for i, arg := range cmdArgs {
		cur := strings.ReplaceAll(arg, "{port}", fmt.Sprintf("%d", port))
		cur = strings.ReplaceAll(cur, "{file}", entry.File)
		cmdArgs[i] = cur
	}

	// 如果有自定义 cmd（字符串，空格分隔），优先使用（兼容旧配置覆盖）
	if entry.Cmd != "" {
		cmdArgs = strings.Fields(string(entry.Cmd))
		for i, arg := range cmdArgs {
			cur := strings.ReplaceAll(arg, "{port}", fmt.Sprintf("%d", port))
			cur = strings.ReplaceAll(cur, "{file}", entry.File)
			cmdArgs[i] = cur
		}
		if len(cmdArgs) > 0 {
			cmdPath = cmdArgs[0]
		}
	}

	var execArgs []string
	if len(cmdArgs) > 0 && cmdArgs[0] == cmdPath {
		execArgs = cmdArgs[1:]
	} else {
		execArgs = cmdArgs
	}

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
	// 延迟减计数
	go func() {
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		sp.reqCount--
		m.mu.Unlock()
	}()
	m.mu.Unlock()
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

// evictIfNeededLocked 驻留超限时 LRU 淘汰（调用方需持锁）。
func (m *Manager) evictIfNeededLocked() {
	if len(m.procs) < maxResident {
		return
	}
	// 找 last_used 最旧且 reqCount==0 的模型
	var victim *subproc
	var victimName string
	for name, sp := range m.procs {
		if sp.reqCount > 0 || sp.state != StateReady {
			continue
		}
		if victim == nil || sp.lastUsed.Before(victim.lastUsed) {
			victim = sp
			victimName = name
		}
	}
	if victim != nil {
		log.Printf("[backend] LRU 淘汰: 卸载 %s (last_used=%s)", victimName, victim.lastUsed.Format("15:04:05"))
		m.stopSubproc(victim)
		delete(m.procs, victimName)
	}
}

// evictForMemoryLocked 卸载模型腾内存（LRU 优先），返回释放的内存 GB。
func (m *Manager) evictForMemoryLocked(needGB float64) float64 {
	var freed float64
	for freed < needGB {
		var victim *subproc
		var victimName string
		for name, sp := range m.procs {
			if sp.reqCount > 0 || sp.state != StateReady {
				continue
			}
			if victim == nil || sp.lastUsed.Before(victim.lastUsed) {
				victim = sp
				victimName = name
			}
		}
		if victim == nil {
			break
		}
		gb := 0.0
		if victim.entry != nil {
			gb = victim.entry.MemGB
		}
		log.Printf("[backend] 内存腾退: 卸载 %s (释放 %.1fGB)", victimName, gb)
		m.stopSubproc(victim)
		delete(m.procs, victimName)
		freed += gb
	}
	return freed
}

// 内部：停止进程（SIGTERM → 等 5s → SIGKILL）
func (m *Manager) stopSubproc(sp *subproc) {
	if sp == nil || sp.proc == nil || sp.proc.Process == nil {
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

// 内部：分配空闲端口（9000-9999）
func (m *Manager) findFreePort() int {
	for port := 9000; port <= 9999; port++ {
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
