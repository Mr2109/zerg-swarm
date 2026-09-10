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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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
	log.Printf("[localback] 探测到本机已加载模型: %s (port=%d)——接管为 ready", modelFile, port)
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
				log.Printf("[localback] 单槽: 复用已在运行的同模型实例 pid=%d port=%d model=%s",
					pr.PID, pr.Port, filepath.Base(modelFile))
				return nil
			}
		}
		if killed := sweepOtherModels(modelFile); len(killed) > 0 {
			log.Printf("[localback] 单槽清场: 已停止 %d 个其它本机实例 %v（整机单模型驻留）", len(killed), killed)
		}
	}
	// 进程内残留（本进程自己起的旧实例）也停掉
	if lb.process != nil && lb.process.ProcessState == nil {
		log.Printf("[localback] 已有模型在运行 (%s)，先停止...", lb.file)
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
		return fmt.Errorf("模型文件不存在: %s: %w", modelFile, err)
	}

	// 内存预算：记录模型 mem_gb，加载时标记占用
	lb.memGB = memGB
	if memGB > 0 {
		log.Printf("[localback] 内存预算: %.1f GB", float64(memGB))
	}

	// 状态机：loading
	lb.setState(stateLoading)
	lb.file = modelFile

	// B12：启动前清理同模型旧进程（孤儿——主控 kill -9 后遗留）
	// 旧 llama-server 进程占着端口/显存——不杀会导致重复实例（Text file busy 类问题）
	oldPids := findExistingProcess(modelFile)
	if len(oldPids) > 0 {
		log.Printf("[localback] 发现 %d 个同模型旧进程 %v——清理", len(oldPids), oldPids)
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
		return fmt.Errorf("无法找到可用端口 (9000-9999)")
	}

	// 构造 llama-server 启动参数
	logFile, err := os.OpenFile(lb.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[localback] 无法打开日志文件 %s: %v", lb.logPath, err)
		lb.setState(stateBroken)
		return fmt.Errorf("打开日志文件失败: %w", err)
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
		return fmt.Errorf("启动 llama-server 失败: %w", err)
	}

	lb.port = port
	pid := lb.process.Process.Pid
	log.Printf("[localback] llama-server 已启动: pid=%d, port=%d, model=%s", pid, port, modelFile)

	// 等待健康检查通过（最多 60 秒）
	if err := lb.waitForReady(); err != nil {
		log.Printf("[localback] 健康检查失败，停止进程: %v", err)
		lb.stopProcess()
		lb.setState(stateBroken)
		return fmt.Errorf("健康检查失败: %w", err)
	}

	// 状态机：ready，重置熔断计数
	lb.failCnt = 0
	lb.setState(stateReady)
	log.Printf("[localback] 后端就绪: port=%d, state=ready", port)
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
		return nil, fmt.Errorf("后端未就绪 (state=%s)", state)
	}
	port := lb.port
	lb.mu.Unlock()

	// 每次请求前健康检查
	if !lb.healthCheck() {
		lb.mu.Lock()
		lb.failCnt++
		if lb.failCnt >= lb.circuit {
			// 熔断：停止进程，重置状态
			log.Printf("[localback] 熔断触发 (连续失败 %d 次)", lb.failCnt)
			lb.stopProcess()
			lb.setState(stateBroken)
			lb.mu.Unlock()
			return nil, fmt.Errorf("后端已熔断 (state=broken, 连续失败 %d 次)", lb.failCnt)
		}
		log.Printf("[localback] 健康检查失败 (%d/%d)，重试中...", lb.failCnt, lb.circuit)
		lb.mu.Unlock()
		// 重试一次
		time.Sleep(500 * time.Millisecond)
		if !lb.healthCheck() {
			return nil, fmt.Errorf("健康检查重试仍失败 (%d/%d)", lb.failCnt, lb.circuit)
		}
		lb.mu.Lock()
		lb.failCnt = 0
		lb.mu.Unlock()
	}

	// 构造转发请求
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
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
				log.Printf("[localback] 连接失败后熔断")
				lb.stopProcess()
				lb.setState(stateBroken)
			}
			lb.mu.Unlock()
		}
		return nil, fmt.Errorf("转发到本机后端失败: %w", err)
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

// LocalSnapshot 本机子端的资源快照（供主控 status 合并展示）。
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
}

// Snapshot 返回本机子端状态快照（machine=local）。
// 本机不跑独立 agent，内存/负载直接从本机采样；模型信息来自 localback 状态。
func (lb *LocalBackend) Snapshot() *LocalSnapshot {
	lb.mu.Lock()
	state := lb.state
	modelFile := lb.file
	memGB := lb.memGB
	lb.mu.Unlock()

	snap := &LocalSnapshot{
		Machine:        "local",
		BackendState:   state,
		MemAvailableGb: sampleLocalMemAvailable(),
		MemTotalGb:     sampleLocalMemTotal(),
		Load:           sampleLocalLoad(),
		GpuTempC:       sampleLocalGpuTemp(),
		Healthy:        state == stateReady,
		Models:         []string{},
	}
	// 本机已加载模型：从模型文件名推断模型名（fleet.yaml 的 file 是绝对路径，取 basename 前缀）
	if state != stateIdle && modelFile != "" {
		name := modelFile
		if idx := lastIndexByte(name, '/'); idx >= 0 {
			name = name[idx+1:]
		}
		// 去掉常见后缀
		for _, suffix := range []string{".gguf", ".GGUF"} {
			if len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix {
				name = name[:len(name)-len(suffix)]
				break
			}
		}
		// 有模型时 healthy=true 且状态 ready
		if state == stateReady {
			snap.Healthy = true
		}
		snap.Models = []string{name}
		snap.GpuUsedGb = float64(memGB)
		snap.BackendRssGb = float64(memGB)
	}

	return snap
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

// 本机采样（尽力而为，失败返回 0）
func sampleLocalMemAvailable() float64 {
	return sampleMacMem("mem_available")
}
func sampleLocalMemTotal() float64 {
	return sampleMacMem("mem_total")
}

// sampleMacMem 从 sysctl/vm_stat 采样本机内存（主控跑在 macOS）。
func sampleMacMem(kind string) float64 {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	totalBytes, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || totalBytes <= 0 {
		return 0
	}
	if kind == "mem_total" {
		return totalBytes / (1024 * 1024 * 1024)
	}
	// 可用内存：vm_stat 计算 free + inactive + speculative
	vout, err := exec.Command("/usr/bin/vm_stat").Output()
	if err != nil {
		return 0
	}
	free, inactive, speculative := uint64(0), uint64(0), uint64(0)
	for _, line := range strings.Split(string(vout), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			free = parseVMPages(line)
		case strings.HasPrefix(line, "Pages inactive:"):
			inactive = parseVMPages(line)
		case strings.HasPrefix(line, "Pages speculative:"):
			speculative = parseVMPages(line)
		}
	}
	// 页大小 16384（Apple Silicon）
	const pageSize = 16384.0
	return float64(free+inactive+speculative) * pageSize / (1024 * 1024 * 1024)
}

// parseVMPages 从 "Pages free: 12345." 提取数字
func parseVMPages(line string) uint64 {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0
	}
	val := strings.Trim(strings.TrimSpace(parts[1]), ".")
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// sampleLocalLoad 读取本机 1 分钟负载。
func sampleLocalLoad() float64 {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		if val, err := strconv.ParseFloat(fields[1], 64); err == nil {
			return val
		}
	}
	return 0
}

// sampleLocalGpuTemp 本机 GPU 温度（Apple Silicon 不公开，尽力而为返回 0）。
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
		log.Printf("[localback] 状态变更: %s → %s", old, s)
	}
}

// 内部：停止进程（SIGTERM → 等 5s → SIGKILL）
func (lb *LocalBackend) stopProcess() {
	if lb.process == nil || lb.process.Process == nil {
		return
	}
	// 先发送 SIGTERM（优雅退出）
	if err := lb.process.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[localback] SIGTERM 发送失败: %v", err)
	}

	// 等待最多 5 秒
	done := make(chan error, 1)
	go func() {
		done <- lb.process.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[localback] 进程退出: %v", err)
		} else {
			log.Printf("[localback] 进程已优雅退出")
		}
	case <-time.After(5 * time.Second):
		log.Printf("[localback] 等待退出超时 (5s)，发送 SIGKILL")
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
			return fmt.Errorf("llama-server 进程已退出")
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("等待就绪超时 (%v)", timeout)
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
		log.Printf("[localback] 单槽清场: 停止其它实例 pid=%d model=%s", p.PID, filepath.Base(p.ModelFile))
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
			log.Printf("[localback] 单槽清场: pid=%d 未退出——SIGKILL", pid)
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
		log.Printf("[localback] 单槽: 整机锁已被其它进程持有（沿用现有实例，本进程不重复启动）")
	}
	lb.lock = lock
	return nil
}
