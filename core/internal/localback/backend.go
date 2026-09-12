// Package localback 提供本机子端内置（LocalBackend）实现。
//
// 职责：管理本机 llama-server 进程的完整生命周期：
//   - 启动/停止 llama-server（os/exec）
//   - 健康检查（交替探测 /health 和 /v1/models）
//   - 加载状态机：idle → loading → ready → idle
//   - 单模型约束：同时只跑一个后端
//   - 内存预算：记录模型 mem_gb，加载时标记占用
//   - 崩溃自愈：健康检查失败 → 重试 3 次 → 熔断
//   - 推理转发：Infer() 方法转发请求到本机 llama-server
package localback

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// llamaServerPath 本机 llama-server 可执行文件路径
const llamaServerPath = "/opt/homebrew/bin/llama-server"

// 加载状态机枚举
const (
	stateIdle    = "idle"    // 空闲，未加载模型
	stateLoading = "loading" // 正在加载/启动中
	stateReady   = "ready"   // 就绪，可接受推理请求
	stateBroken  = "broken"  // 熔断，健康检查连续失败
)

// LocalBackend 本机子端内置管理器
//
// 线程安全：所有方法通过 mu 互斥锁保护共享状态。
type LocalBackend struct {
	mu      sync.Mutex
	process *exec.Cmd     // 当前 llama-server 进程
	port    int           // 当前端口（动态分配）
	file    string        // 当前加载的模型文件路径
	lock    *InstanceLock // B12: 单实例 flock 锁（持锁=服务在运行）
	memGB   int           // 内存预算（GB）
	state   string        // 当前状态（状态机）
	failCnt int           // 连续健康检查失败次数
	circuit int           // 熔断阈值（默认 3）
	logPath string        // 日志文件路径
}

// NewLocalBackend 创建并初始化本机后端实例
func NewLocalBackend(logPath string) *LocalBackend {
	return &LocalBackend{
		state:   stateIdle,
		circuit: 3, // 健康检查连续失败 3 次触发熔断
		logPath: logPath,
	}
}

// AdoptExisting — v2.5.4.9 探测本机已运行的 llama-server（孤儿/外部启动——如 9000 端口 ornith）
//
//	按候选模型文件匹配（fleet.yaml local 候选）——识别已加载模型 → 设置 ready
//	candidates: 本机候选模型文件列表（匹配其中一个才接管——避免接错残留进程）
//	返回值: 是否成功接管（本机有配置内的模型在跑）
func (lb *LocalBackend) AdoptExisting(candidates []string) bool {
	modelFile, port := findMatchingRunningModel(candidates)
	if modelFile == "" || port == 0 {
		return false
	}
	lb.file = modelFile
	lb.port = port
	lb.setState(stateReady)
	log.Printf("[localback] detected already-loaded local model: %s (port=%d) — adopting as ready", modelFile, port)
	return true
}

// findMatchingRunningModel — 扫 9000-9999 llama-server——匹配候选模型文件（返回模型文件+端口）
func findMatchingRunningModel(candidates []string) (string, int) {
	cmd := exec.Command("lsof", "-iTCP:9000-9999", "-sTCP:LISTEN", "-P", "-n")
	out, err := cmd.Output()
	if err != nil {
		return "", 0
	}
	seen := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || seen[pid] {
			continue
		}
		seen[pid] = true
		psOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			continue
		}
		cmdLine := string(psOut)
		if !strings.Contains(cmdLine, "llama-server") {
			continue
		}
		// 命令行匹配任一候选模型文件
		for _, cand := range candidates {
			if strings.Contains(cmdLine, cand) {
				// 提取 --port 参数
				parts := strings.Fields(cmdLine)
				for i, p := range parts {
					if p == "--port" && i+1 < len(parts) {
						if port, err := strconv.Atoi(parts[i+1]); err == nil {
							return cand, port
						}
					}
				}
				// lsof 行里也有端口（NAME 列）
				for _, f := range fields {
					if strings.Contains(f, ":") && len(f) > 3 {
						if port, err := strconv.Atoi(strings.Split(f, ":")[1]); err == nil && port >= 9000 {
							return cand, port
						}
					}
				}
				return cand, 0
			}
		}
	}
	return "", 0
}

// LoadModel 加载模型并启动 llama-server。
// 线程安全：单模型约束，先停旧进程再启新进程。
//
// 流程：
//  1. 检查是否已有模型在运行 → 停止（SIGTERM → 等 5s → SIGKILL）
//  2. 检查模型文件是否存在
//  3. 记录内存预算
//  4. 设置状态 loading
//  5. 动态分配端口，exec llama-server
//  6. 等待健康检查通过（最多 60 秒）
//  7. 设置状态 ready
//
// 失败时返回 error 并保持状态 broken。
func (lb *LocalBackend) LoadModel(modelFile string, memGB int) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// ── 整机单槽（2026-09-10 修复，Mr2109拍板）────────────────────────────────
	// 背景：本机实例常是"接管"来的（AdoptExisting → lb.process == nil），
	// 旧逻辑"有进程才停"会漏 → 实测同时驻留 Ornith-1.5 + gemma-4-26B（PhysMem 56G used / 21G compressor）。
	// 规则：① 目标模型已在跑 → 接管复用（不新起）② 其它模型实例 → 一律停掉（整机只驻留一个）。
	if procs := listRunningLlamaModels(); len(procs) > 0 {
		for _, pr := range procs {
			if pr.ModelFile == modelFile && pr.Port > 0 {
				lb.file = modelFile
				lb.port = pr.Port
				lb.process = nil // 外部实例（接管态）
				lb.memGB = memGB
				_ = lb.ensureMachineLock()
				lb.failCnt = 0
				lb.setState(stateReady)
				log.Printf("[localback] single-slot: reusing running instance of same model pid=%d port=%d model=%s",
					pr.PID, pr.Port, filepath.Base(modelFile))
				return nil
			}
		}
		if killed := sweepOtherModels(modelFile); len(killed) > 0 {
			log.Printf("[localback] single-slot cleanup: stopped %d other local instance(s) %v (one model resident machine-wide)", len(killed), killed)
		}
	}
	// 进程内残留（本进程自己起的旧实例）也停掉
	if lb.process != nil && lb.process.ProcessState == nil {
		log.Printf("[localback] another model is running (%s), stopping it first...", lb.file)
		lb.stopProcess()
	}
	// 整机单例锁（持锁 = 本机唯一后端；锁文件在状态目录，不用 /tmp）
	if err := lb.ensureMachineLock(); err != nil {
		lb.setState(stateBroken)
		return err
	}

	// 检查模型文件是否存在
	if _, err := os.Stat(modelFile); err != nil {
		lb.setState(stateBroken)
		return fmt.Errorf("model file does not exist: %s: %w", modelFile, err)
	}

	// 内存预算：记录模型 mem_gb，加载时标记占用
	lb.memGB = memGB
	if memGB > 0 {
		log.Printf("[localback] memory budget: %.1f GB", float64(memGB))
	}

	// 状态机：loading
	lb.setState(stateLoading)
	lb.file = modelFile

	// B12：启动前清理同模型旧进程（孤儿——主控 kill -9 后遗留）
	// 旧 llama-server 进程占着端口/显存——不杀会导致重复实例（Text file busy 类问题）
	oldPids := findExistingProcess(modelFile)
	if len(oldPids) > 0 {
		log.Printf("[localback] found %d stale process(es) for the same model %v — cleaning up", len(oldPids), oldPids)
		for _, pid := range oldPids {
			// 先 SIGTERM，1 秒后没死再 SIGKILL
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Signal(syscall.SIGTERM)
			}
		}
		time.Sleep(1 * time.Second)
		for _, pid := range oldPids {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	}

	// 动态端口分配：在 9000-9999 范围内找可用端口
	port := lb.findFreePort()
	if port == 0 {
		lb.setState(stateBroken)
		return fmt.Errorf("no free port available (9000-9999)")
	}

	// 构造 llama-server 启动参数
	logFile, err := os.OpenFile(lb.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[localback] cannot open log file %s: %v", lb.logPath, err)
		lb.setState(stateBroken)
		return fmt.Errorf("failed to open log file: %w", err)
	}

	args := []string{
		"-m", modelFile, // 模型文件
		"-c", "262144", // 上下文窗口 256K（ornith 支持最大，maxToken 同值）
		"--host", "127.0.0.1", // 绑定本机
		"--port", fmt.Sprintf("%d", port), // 动态端口
		"--log-disable", // 禁用 llama-server 内部日志（我们用文件）
	}

	// 启动 llama-server 进程
	lb.process = exec.Command(llamaServerPath, args...)
	lb.process.Stdout = logFile
	lb.process.Stderr = logFile
	if err := lb.process.Start(); err != nil {
		logFile.Close()
		lb.setState(stateBroken)
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	lb.port = port
	pid := lb.process.Process.Pid
	log.Printf("[localback] llama-server started: pid=%d, port=%d, model=%s", pid, port, modelFile)

	// 等待健康检查通过（最多 60 秒）
	if err := lb.waitForReady(); err != nil {
		log.Printf("[localback] health check failed, stopping process: %v", err)
		lb.stopProcess()
		lb.setState(stateBroken)
		return fmt.Errorf("health check failed: %w", err)
	}

	// 状态机：ready，重置熔断计数
	lb.failCnt = 0
	lb.setState(stateReady)
	log.Printf("[localback] backend ready: port=%d, state=ready", port)
	return nil
}

// Infer 转发推理请求到本机 llama-server。
// 每个请求先做健康检查，失败则重试（崩溃自愈）。
// 连续失败超过熔断阈值则返回 broken。
//
// path: 透传路径（如 /v1/chat/completions, /v1/responses, /v1/messages）
// body: 请求体 JSON
//
// 返回 http.Response（调用方负责 Close）和 error。
func (lb *LocalBackend) Infer(path string, body []byte) (*http.Response, error) {
	lb.mu.Lock()
	// v2.5.4.9 接管模式兼容：AdoptExisting 后 process=nil（外部进程）——但 state=ready + port>0 允许推理
	adopted := lb.state == stateReady && lb.process == nil && lb.port > 0
	if lb.state != stateReady || (lb.process == nil && !adopted) {
		state := lb.state
		lb.mu.Unlock()
		return nil, fmt.Errorf("backend not ready (state=%s)", state)
	}
	port := lb.port
	lb.mu.Unlock()

	// 每次请求前健康检查
	if !lb.healthCheck() {
		lb.mu.Lock()
		lb.failCnt++
		if lb.failCnt >= lb.circuit {
			// 熔断：停止进程，重置状态
			log.Printf("[localback] circuit breaker tripped (%d consecutive failures)", lb.failCnt)
			lb.stopProcess()
			lb.setState(stateBroken)
			lb.mu.Unlock()
			return nil, fmt.Errorf("backend circuit-broken (state=broken, %d consecutive failures)", lb.failCnt)
		}
		log.Printf("[localback] health check failed (%d/%d), retrying...", lb.failCnt, lb.circuit)
		lb.mu.Unlock()
		// 重试一次
		time.Sleep(500 * time.Millisecond)
		if !lb.healthCheck() {
			return nil, fmt.Errorf("health check still failing after retry (%d/%d)", lb.failCnt, lb.circuit)
		}
		lb.mu.Lock()
		lb.failCnt = 0
		lb.mu.Unlock()
	}

	// 构造转发请求
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 转发到本机 llama-server
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		// 连接失败视为崩溃，触发健康检查
		if !lb.healthCheck() {
			lb.mu.Lock()
			lb.failCnt++
			if lb.failCnt >= lb.circuit {
				log.Printf("[localback] circuit opened after connection failure")
				lb.stopProcess()
				lb.setState(stateBroken)
			}
			lb.mu.Unlock()
		}
		return nil, fmt.Errorf("failed to forward to local backend: %w", err)
	}

	// 请求成功，重置失败计数
	lb.mu.Lock()
	lb.failCnt = 0
	lb.mu.Unlock()
	return resp, nil
}

// State 返回当前状态字符串（供状态 API 展示）
func (lb *LocalBackend) State() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.state
}

// IsReady 是否已就绪可接受推理请求
func (lb *LocalBackend) IsReady() bool {
	return lb.State() == stateReady
}

// ModelFile 返回当前加载的模型文件路径
func (lb *LocalBackend) ModelFile() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.file
}

// MemGB 返回当前模型的内存预算
func (lb *LocalBackend) MemGB() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.memGB
}

// LocalSnapshot 本机子端的资源快照（供主控 status 合并展示，也供资源账本 #30 复用）。
//
// 口径（批 5 + #33）：
//   - GpuUsedGb 只放**真实**显存占用：只有平台真采到独立显存（Linux + nvidia-smi/rocm-smi）才填，
//     macOS（Apple Silicon 统一内存）拿不到 → 保持 0 且 VramKnown=false（旧行为 snap.GpuUsedGb = memGB
//     是拿内存冒充显存，已修，见 #29）。BackendRssGb 仍是本机后端的"占用"口径（与旧行为一致，未单独采样 RSS）。
//   - Vram*/显存三值：不变量 = 采到 → VramKnown=true 且三值到场；采不到 → VramKnown=false 且三值缺席
//     （JSON omitempty 整键不出现）。绝不存在"有值而 known=false"或"known=true 而无值"的半真状态（#33）。
//   - 内存总量/可用量：macOS 走 sysctl/vm_stat，Linux 走 /proc/meminfo，单位一律 GiB（1024^3），
//     与账本判定 mem_known（= MemTotalGb > 0）配套；采不到即 0 = 缺席。
//   - Resident[]：本机驻留明细——由 LocalBackend 自己的状态（ModelFile/MemGB/State）构造，
//     与远程子端**同一套**账本口径（§八 Q7）。
type LocalSnapshot struct {
	Machine        string   `json:"machine"`
	Model          *string  `json:"model,omitempty"`
	BackendState   string   `json:"backend_state"`
	MemAvailableGb float64  `json:"mem_available_gb"`
	MemTotalGb     float64  `json:"mem_total_gb"`
	Load           float64  `json:"load"`
	GpuUsedGb      float64  `json:"gpu_used_gb"`
	GpuTempC       float64  `json:"gpu_temp_c"`
	BackendRssGb   float64  `json:"backend_rss_gb"`
	Healthy        bool     `json:"healthy"`
	Models         []string `json:"models"`

	// ── 资源账本（批 5 #30/#29）──
	Resident    []resources.ResidentEntry `json:"resident,omitempty"`
	VramKnown   bool                      `json:"vram_known"`
	VramUnified bool                      `json:"vram_unified,omitempty"`
	VramTotalGb float64                   `json:"vram_total_gb,omitempty"`
	VramUsedGb  float64                   `json:"vram_used_gb,omitempty"`
	VramFreeGb  float64                   `json:"vram_free_gb,omitempty"`
}

// Snapshot 返回本机子端状态快照（machine=local）。
// 本机不跑独立 agent，内存/负载直接从本机采样（#33：macOS 走 sysctl/vm_stat，Linux 走 /proc）；
// 模型信息来自 localback 状态。
func (lb *LocalBackend) Snapshot() *LocalSnapshot {
	lb.mu.Lock()
	state := lb.state
	modelFile := lb.file
	memGB := lb.memGB
	lb.mu.Unlock()

	// 内存：一次采样取齐总量/可用量（口径一致）。采不到 → 0 = 缺席（"知不知道内存"由账本按
	// MemTotalGb > 0 判定，见 resources_ledger.go）。
	memTotal, memAvail := sampleLocalMem()

	// 显存（#29 + #33）：先问平台显存采样——只有 Linux 装了 nvidia-smi / rocm-smi 才可能拿到独立显存。
	// 拿到 → known=true + 三值到场；拿不到 → 保持 localVramShape() 的如实未知（不填假值）。
	// 本机（macOS Apple Silicon 统一内存）没有独立显存额度——如实标未知，绝不拿内存量冒充（旧行为已修）。
	pVramKnown, pVramTotal, pVramUsed, pVramFree := samplePlatformVram()
	known, unified := localVramShape()
	if pVramKnown {
		// 独立显存与统一内存互斥：统一内存平台没有"独立显存额度"这个概念。
		known, unified = true, false
	}

	snap := &LocalSnapshot{
		Machine:        "local",
		BackendState:   state,
		MemAvailableGb: memAvail,
		MemTotalGb:     memTotal,
		Load:           sampleLocalLoad(),
		GpuTempC:       sampleLocalGpuTemp(),
		Healthy:        state == stateReady,
		Models:         []string{},
		VramKnown:      known,
		VramUnified:    unified,
	}
	if pVramKnown {
		// 真采到独立显存才带值；拿不到时三键留 0 → JSON omitempty 整键不出现（缺席，不造值）。
		snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb = pVramTotal, pVramUsed, pVramFree
		// 真实显存占用（与 vram_used_gb 同源）——只在真拿到时填，绝不拿 RSS/内存量冒充（#29）。
		snap.GpuUsedGb = pVramUsed
	}
	// 本机已加载模型：从模型文件名推断模型名（fleet.yaml 的 file 是绝对路径，取 basename 前缀）
	if state != stateIdle && modelFile != "" {
		name := modelNameFromFile(modelFile)
		// 有模型时 healthy=true 且状态 ready
		if state == stateReady {
			snap.Healthy = true
		}
		snap.Models = []string{name}
		// 本机后端占用口径（未单独采样 RSS，与旧行为一致）
		snap.BackendRssGb = float64(memGB)
		// 驻留明细（#30）：用 LocalBackend 自己的状态构造，口径同远程（§八 Q7）。
		// 本机单槽：同时只有一个模型驻留，故 resident[] 最多一项。
		snap.Resident = []resources.ResidentEntry{{
			Alias:   name,
			File:    modelFile,
			State:   residentState(state),
			MemGb:   float64(memGB),
			Managed: true, // 在 LocalBackend 的托管清单里（含接管的外部实例——会被单槽规则替换）
			Source:  "localback",
		}}
	}

	return snap
}

// localVramShape 报告本机显存的**形态**（§3.1；#33 后它只答"是不是统一内存"，不再是显存是否已知的唯一来源：
// Linux 上真采到独立显存时由 samplePlatformVram() 给 known=true，见 Snapshot）：
//   - macOS（Apple Silicon 统一内存）：无独立显存额度 → known=false, unified=true（显存即内存）
//   - Linux：多卡/独显不是统一内存 → known=false, unified=false；显存本身另由 nvidia-smi/rocm-smi 尽力采
//   - 其它平台：本机未实现显存采样 → known=false, unified=false（按"真未知"走估算法 fail-closed）
//
// 一律不返回"已知"的假值：拿不到就不冒充（#29 铁律）。
func localVramShape() (known, unified bool) {
	return false, runtime.GOOS == "darwin"
}

// residentState 把 localback 状态机的状态翻成账本（shared/resources）的状态取值。
// broken（熔断）对账本就是 crashed——如实反映"不可用"，不美化。
func residentState(state string) string {
	switch state {
	case stateReady:
		return resources.StateReady
	case stateLoading:
		return resources.StateLoading
	case stateBroken:
		return resources.StateCrashed
	default:
		return resources.StateIdle
	}
}

// modelNameFromFile 从模型文件路径推模型名（basename 去掉 .gguf/.GGUF 后缀）。
// 过渡期兼容：fleet.yaml 的 file 是绝对路径，账本/路由展示用名字作别名（身份仍是摘要）。
func modelNameFromFile(file string) string {
	name := file
	if idx := lastIndexByte(name, '/'); idx >= 0 {
		name = name[idx+1:]
	}
	for _, suffix := range []string{".gguf", ".GGUF"} {
		if len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

// lastIndexByte 返回最后一个指定字节的位置，无则 -1。
func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// sampleLocalMem 采样本机内存总量与可用量（GiB）——两个值取自同一次采样，口径一致。
//
// 平台差异在 sample_platform.go：macOS 走 sysctl hw.memsize + vm_stat，Linux 走 /proc/meminfo（#33）。
// 任一字段采不到 → 该项返回 0（缺席）；本机行"知不知道内存"由资源账本按 MemTotalGb > 0 判定
// （core/internal/api/resources_ledger.go），故这里绝不用 0 充数、也不用估算值。
func sampleLocalMem() (totalGB, availGB float64) {
	total, avail, totalOK, availOK := samplePlatformMem()
	if !totalOK {
		total = 0
	}
	if !availOK {
		avail = 0
	}
	return total, avail
}

// sampleLocalLoad 读取本机 1 分钟负载（平台差异见 sample_platform.go；采不到 → 0 = 缺席）。
func sampleLocalLoad() float64 {
	if v, ok := samplePlatformLoad(); ok {
		return v
	}
	return 0
}

// sampleLocalGpuTemp 本机 GPU 温度（Apple Silicon 不公开；Linux 侧未采样 → 尽力而为返回 0）。
func sampleLocalGpuTemp() float64 {
	return 0
}

// Stop 手动停止后端，释放资源
func (lb *LocalBackend) Stop() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.stopProcess()
	lb.setState(stateIdle)
}

// 内部：设置状态
func (lb *LocalBackend) setState(s string) {
	old := lb.state
	lb.state = s
	if old != s {
		log.Printf("[localback] state change: %s → %s", old, s)
	}
}

// 内部：停止进程（SIGTERM → 等 5s → SIGKILL）
func (lb *LocalBackend) stopProcess() {
	if lb.process == nil || lb.process.Process == nil {
		return
	}
	// 先发送 SIGTERM（优雅退出）
	if err := lb.process.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[localback] failed to send SIGTERM: %v", err)
	}

	// 等待最多 5 秒
	done := make(chan error, 1)
	go func() {
		done <- lb.process.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[localback] process exited: %v", err)
		} else {
			log.Printf("[localback] process exited gracefully")
		}
	case <-time.After(5 * time.Second):
		log.Printf("[localback] wait-for-exit timed out (5s), sending SIGKILL")
		_ = lb.process.Process.Kill()
		lb.process.Wait() // 等待 SIGKILL 生效
	}

	lb.process = nil
	lb.port = 0

	// B12: 释放单实例锁（进程停止 = 锁释放，其他程序可启动）
	if lb.lock != nil {
		lb.lock.Release()
		lb.lock = nil
	}
}

// modelName 从模型文件路径提取锁用的模型键（文件名去扩展名）。
func modelName(modelFile string) string {
	base := filepath.Base(modelFile)
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	// 锁文件名不能含路径分隔符，用 base 即可
	return base
}

// 内部：健康检查（交替探测 /health 和 /v1/models）
// llama-server 有 /health，ds4 没有——本机都是 llama-server，用 /health 为主，失败换 /v1/models
func (lb *LocalBackend) healthCheck() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", lb.port)
	resp, err := http.Get(url)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 && strings.Contains(string(body), "ok") {
			return true
		}
	}

	// 失败换 /v1/models
	url = fmt.Sprintf("http://127.0.0.1:%d/v1/models", lb.port)
	resp, err = http.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}
	return false
}

// 内部：等待后端就绪（最多 60 秒）
func (lb *LocalBackend) waitForReady() error {
	const timeout = 60 * time.Second
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		if lb.healthCheck() {
			return nil
		}
		// 检查进程是否还活着
		if lb.process != nil && lb.process.ProcessState != nil {
			return fmt.Errorf("llama-server process already exited")
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timed out waiting for readiness (%v)", timeout)
}

// 内部：在动态端口范围内找第一个可用端口
func (lb *LocalBackend) findFreePort() int {
	for port := 9000; port <= 9999; port++ {
		if isPortFree(port) {
			return port
		}
	}
	return 0
}

// 内部：检查端口是否可用
func isPortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// findAnyRunningModel — v2.5.4.9 扫 9000-9999 任一 llama-server 进程（不管哪个模型）
//
//	返回: PID 列表 + 模型文件（命令行里的 -m 参数）
func findAnyRunningModel() ([]int, string) {
	cmd := exec.Command("lsof", "-iTCP:9000-9999", "-sTCP:LISTEN", "-P", "-n")
	out, err := cmd.Output()
	if err != nil {
		return nil, ""
	}
	var pids []int
	seen := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || seen[pid] {
			continue
		}
		seen[pid] = true
		// 命令行含 -m 模型文件 → 识别模型
		psOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			continue
		}
		cmdLine := string(psOut)
		if !strings.Contains(cmdLine, "llama-server") {
			continue
		}
		// 提取 -m 后面的模型文件
		parts := strings.Fields(cmdLine)
		for i, p := range parts {
			if p == "-m" && i+1 < len(parts) {
				pids = append(pids, pid)
				return pids, parts[i+1]
			}
		}
	}
	return pids, ""
}

// findExistingProcess 按模型文件查找已运行的 llama-server 进程（含孤儿进程）。
// 返回进程 PID 列表（可能多个——主控被 kill -9 后遗留的孤儿）。
func findExistingProcess(modelFile string) []int {
	// 用 lsof 查监听 9000-9999 且命令行含模型文件的进程
	cmd := exec.Command("lsof", "-iTCP:9000-9999", "-sTCP:LISTEN", "-P", "-n")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pids []int
	seen := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		// lsof 格式: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || seen[pid] {
			continue
		}
		// 检查该进程命令行是否含模型文件（macOS/Linux 通用）
		psOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(psOut), modelFile) {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	return pids
}

// ─── 整机单槽（2026-09-10 Mr2109拍板）─────────────────────────────────────────

// llamaProc — 本机一个 llama-server 实例（pid/端口/模型文件）
type llamaProc struct {
	PID       int
	Port      int
	ModelFile string
}

// listRunningLlamaModels — 扫 9000-9999 上**所有** llama-server 实例（跨模型，不只匹配单个）
func listRunningLlamaModels() []llamaProc {
	out, err := exec.Command("lsof", "-iTCP:9000-9999", "-sTCP:LISTEN", "-P", "-n").Output()
	if err != nil {
		return nil
	}
	seen := map[int]bool{}
	var procs []llamaProc
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || seen[pid] {
			continue
		}
		psOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			continue
		}
		cmdLine := string(psOut)
		if !strings.Contains(cmdLine, "llama-server") {
			continue
		}
		seen[pid] = true
		procs = append(procs, llamaProc{
			PID:       pid,
			Port:      portFromCmdLine(cmdLine),
			ModelFile: modelFileFromCmdLine(cmdLine),
		})
	}
	return procs
}

// modelFileFromCmdLine — 从 llama-server 命令行提取 -m 的模型文件（无则 ""）
func modelFileFromCmdLine(cmdLine string) string {
	parts := strings.Fields(cmdLine)
	for i, p := range parts {
		if p == "-m" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// portFromCmdLine — 从 llama-server 命令行提取 --port（无则 0）
func portFromCmdLine(cmdLine string) int {
	parts := strings.Fields(cmdLine)
	for i, p := range parts {
		if p == "--port" && i+1 < len(parts) {
			if n, err := strconv.Atoi(parts[i+1]); err == nil {
				return n
			}
		}
	}
	return 0
}

// sweepOtherModels — 整机单槽清场：停掉除 target 之外的所有本机 llama-server 实例。
// SIGTERM → 最多等 8s（llama 释放显存需时间）→ 仍活则 SIGKILL。返回被处理的 pid 列表。
func sweepOtherModels(target string) []int {
	var killed []int
	for _, p := range listRunningLlamaModels() {
		if p.PID <= 1 || p.ModelFile == target {
			continue
		}
		log.Printf("[localback] single-slot cleanup: stopping other instance pid=%d model=%s", p.PID, filepath.Base(p.ModelFile))
		if proc, err := os.FindProcess(p.PID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		killed = append(killed, p.PID)
	}
	if len(killed) == 0 {
		return nil
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !anyAlive(killed) {
			return killed
		}
		time.Sleep(300 * time.Millisecond)
	}
	for _, pid := range killed {
		if pidAlive(pid) {
			log.Printf("[localback] single-slot cleanup: pid=%d did not exit — SIGKILL", pid)
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	}
	return killed
}

// pidAlive — 进程是否存活（signal 0）
func pidAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func anyAlive(pids []int) bool {
	for _, pid := range pids {
		if pidAlive(pid) {
			return true
		}
	}
	return false
}

// ensureMachineLock — 整机单例锁（整机只允许一个本机后端；持锁期间其它 zerg-core 不重复启动）
func (lb *LocalBackend) ensureMachineLock() error {
	if lb.lock != nil {
		return nil // 已持有
	}
	lock, exists, err := AcquireLock("local")
	if err != nil {
		return err
	}
	if exists {
		log.Printf("[localback] single-slot: machine-wide lock held by another process (reusing existing instance, not starting a new one)")
	}
	lb.lock = lock
	return nil
}
