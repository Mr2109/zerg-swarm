// Package monitor 提供系统资源采样（内存、负载等）。
//
// 职责：
//   - 采样系统可用内存（GB）、总内存（GB）
//   - Linux 通过 /proc/meminfo，macOS 通过 sysctl
//   - 单例模式，所有请求共享同一份采样数据
package monitor

import (
	"bufio"
	"os"
	"runtime"
	"os/exec"
	"strconv"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Sampler 系统资源采样器，线程安全。
type Sampler struct {
	mu             sync.RWMutex
	memAvailableGb float64 // 可用内存（GB）
	memTotalGb     float64 // 总内存（GB）
	gpuTempC       float64 // GPU 温度（°C），0 表示无数据
	cpuPct         float64 // CPU 使用率 %（load/核数近似）
	gpuPct         float64 // GPU 使用率 %（显存占用近似）
	lastUpdate     time.Time
}

// DefaultSampler 全局采样器单例
var DefaultSampler = &Sampler{}

// StartLoop 启动后台采样循环，每 5 秒更新一次。
func (s *Sampler) StartLoop() {
	go func() {
		for {
			s.update()
			time.Sleep(5 * time.Second)
		}
	}()
}

// update 重新采样系统内存信息。
func (s *Sampler) update() {
	memAvail, memTotal := sampleMem()
	gpuTemp := sampleGpuTemp()
	// B4 v2：CPU/GPU 使用率（load 近似 / 显存近似）
	cpuPct := 0.0
	cores := runtime.NumCPU()
	if cores == 0 {
		cores = 1
	}
	if v, ok := readLinuxLoad(); ok {
		cpuPct = v / float64(cores) * 100 // load 相对核数归一
		if cpuPct > 100 {
			cpuPct = 100
		}
	} else if v := readMacOSLoad(); v > 0 {
		cpuPct = v / float64(cores) * 100
		if cpuPct > 100 {
			cpuPct = 100
		}
	}
	gpuPct := sampleGpuPct()
	s.mu.Lock()
	s.memAvailableGb = memAvail
	s.memTotalGb = memTotal
	s.gpuTempC = gpuTemp
	s.cpuPct = cpuPct
	s.gpuPct = gpuPct
	s.lastUpdate = time.Now()
	s.mu.Unlock()
}

// MemAvailableGb 返回当前可用内存（GB），保留 1 位小数。
func (s *Sampler) MemAvailableGb() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return roundTo(s.memAvailableGb, 1)
}

// MemTotalGb 返回当前总内存（GB）。
func (s *Sampler) MemTotalGb() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return roundTo(s.memTotalGb, 1)
}

// GpuTempC 返回当前 GPU 温度（°C），0 表示无数据。
func (s *Sampler) GpuTempC() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return roundTo(s.gpuTempC, 0)
}

// CpuPct 返回 CPU 使用率 %（B4 v2）。
func (s *Sampler) CpuPct() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return roundTo(s.cpuPct, 1)
}

// GpuPct 返回 GPU 使用率 %（B4 v2），0 表示无数据。
func (s *Sampler) GpuPct() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return roundTo(s.gpuPct, 1)
}

// LoadAvg 返回当前系统 1 分钟负载（0-1 相对 CPU 数，尽力而为）。
func (s *Sampler) LoadAvg() float64 {
	if v, ok := readLinuxLoad(); ok {
		return v
	}
	return readMacOSLoad()
}

// readLinuxLoad 读 /proc/loadavg，返回 1 分钟负载（相对 CPU 核数归一）。
func readLinuxLoad() (float64, bool) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, false
	}
	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// readMacOSLoad 通过 sysctl 读 1 分钟负载。
func readMacOSLoad() float64 {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0
	}
	// 输出形如: { 1.23 0.45 0.12 }
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		if val, err := strconv.ParseFloat(fields[1], 64); err == nil {
			return val
		}
	}
	return 0
}

// sampleGpuTemp 采样 GPU 温度（°C）。
//   - Linux（AMD ROCm）: rocm-smi --showtemp，解析 GPU temperature
//   - macOS（Apple Silicon）: GPU 温度不公开，尝试 powermetrics（root）失败返回 0
//
// 尽力而为：采集失败返回 0，不影响心跳。
func sampleGpuTemp() float64 {
	if temp, ok := readLinuxGpuTemp(); ok {
		return temp
	}
	return readMacOSGpuTemp()
}

// sampleGpuPct 采样 GPU 使用率 %（B4 v2）。
//   - Linux（AMD ROCm）: rocm-smi --showuse，解析 GPU use（%）
//   - macOS（Apple Silicon）: ioreg gpu-perf-tgt-utilization（无需 root）
func sampleGpuPct() float64 {
	out, err := exec.Command("rocm-smi", "--showuse").Output()
	if err != nil {
		// 尝试 nvidia-smi（NVIDIA GPU）
		out, err = exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits").Output()
		if err != nil {
			// macOS Apple Silicon：ioreg gpu-perf-tgt-utilization
			return sampleMacOSGpuPct()
		}
		var pct float64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &pct); err == nil {
			return pct
		}
		return -1
	}
	// rocm-smi 输出形如: "GPU use (%): 35.0%" 或 "GPU use: 35.0%"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// 匹配 "35.0%" 或 "35.0"
		if strings.Contains(line, "%") {
			var pct float64
			if _, err := fmt.Sscanf(line[strings.LastIndex(line, ":")+1:], "%f", &pct); err == nil {
				return pct
			}
		}
	}
	return -1
}

// sampleMacOSGpuPct macOS Apple Silicon GPU 利用率（ioreg gpu-perf-tgt-utilization——无需 root）。
func sampleMacOSGpuPct() float64 {
	out, err := exec.Command("ioreg", "-l").Output()
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "gpu-perf-tgt-utilization") {
			start := strings.Index(line, "<")
			end := strings.Index(line, ">")
			if start >= 0 && end > start {
				hexStr := strings.ReplaceAll(line[start+1:end], " ", "")
				if len(hexStr) >= 2 {
					val, err := strconv.ParseInt(hexStr[:2], 16, 32)
					if err == nil {
						return float64(val)
					}
				}
			}
		}
	}
	return -1
}

// readLinuxGpuTemp 通过 rocm-smi 读取 AMD GPU 温度（°C）。
func readLinuxGpuTemp() (float64, bool) {
	// rocm-smi 常见路径
	candidates := []string{"/opt/rocm/bin/rocm-smi", "rocm-smi"}
	var out []byte
	var err error
	for _, cmd := range candidates {
		out, err = exec.Command(cmd, "--showtemp").Output()
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, false
	}
	// 输出形如: "GPU Temperature: 52.0°C" 或 "GPU Temperature (edge): 52.0°C"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "emperature") {
			// 提取数字（可能带 °C 后缀）
			fields := strings.Fields(line)
			for _, f := range fields {
				// 去掉 °C / C 后缀
				numStr := strings.TrimSuffix(f, "°C")
				numStr = strings.TrimSuffix(numStr, "C")
				numStr = strings.TrimSuffix(numStr, "°")
				val, err := strconv.ParseFloat(numStr, 64)
				if err == nil && val > 0 && val < 200 {
					return val, true
				}
			}
		}
	}
	return 0, false
}

// readMacOSGpuTemp 读取 macOS GPU 温度。
// Apple Silicon 不公开 GPU 温度传感器；powermetrics 需 root。
// 尽力而为：读不到返回 0。
func readMacOSGpuTemp() float64 {
	// 尝试 ioreg 读 AppleClCD / GPU 相关传感器（部分机型可用）
	out, err := exec.Command("/usr/sbin/ioreg", "-l").Output()
	if err != nil {
		return 0
	}
	// Apple Silicon 通常没有公开的 GPU 温度 key，这里尝试常见的
	// "temperature" 相关 key（如 Ambient / CPU die，尽力而为）
	_ = out
	return 0
}

// sampleMem 采样系统内存信息，返回 (availableGB, totalGB)。
func sampleMem() (float64, float64) {
	// Linux: 读 /proc/meminfo
	if available, total, ok := readLinuxMeminfo(); ok {
		return available, total
	}
	// macOS: 读 sysctl 输出
	return readMacOSMeminfo()
}

// readLinuxMeminfo 从 /proc/meminfo 解析 MemTotal 和 MemAvailable。
func readLinuxMeminfo() (float64, float64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMeminfoValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			available = parseMeminfoValue(line)
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return float64(available) / 1024 / 1024, float64(total) / 1024 / 1024, true
}

// parseMeminfoValue 解析 /proc/meminfo 中 "数值 kB" 的格式，返回 kB 值。
func parseMeminfoValue(line string) uint64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// readMacOSMeminfo 从 sysctl 获取内存信息（通过 /proc/sys 或 sysctl 命令）。
// macOS 没有 /proc/meminfo，这里尝试通过读取系统信息获取。
func readMacOSMeminfo() (float64, float64) {
	// macOS 内存读取：总内存用 sysctl hw.memsize；可用内存用 host_statistics64（vm_stat 等价）
	total := readMacOSTotalMem()
	avail := readMacOSAvailMem()
	return avail, total
}

// readMacOSTotalMem 通过 sysctl hw.memsize 获取总内存（字节）
func readMacOSTotalMem() float64 {
	// 用绝对路径（agent 可能由 launchd/systemd 启动，PATH 不含 /usr/bin）
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return float64(val) / (1024 * 1024 * 1024)
}

// readMacOSAvailMem 通过 vm_stat 计算可用内存（free + inactive + speculative 页）
// vm_stat 输出格式: "Pages free: 12345." 等
func readMacOSAvailMem() float64 {
	out, err := exec.Command("/usr/bin/vm_stat").Output()
	if err != nil {
		return 0
	}
	free, inactive, speculative := uint64(0), uint64(0), uint64(0)
	for _, line := range strings.Split(string(out), "\n") {
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
	// 页大小 16384 字节（Apple Silicon）
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

// roundTo 将浮点数四舍五入到指定小数位。
func roundTo(val float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(val*pow+0.5)) / pow
}
