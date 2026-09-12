// 显存（VRAM）采样——与内存分开，单独一条口径。
//
// 诚实纪律（对齐《设计-资源管理器》§3.1 与 §八 Q4"宁可标估的，不许冒充实测"）：
//   - 能拿到真实显存就报真实值；
//   - 拿不到就返回 ok=false（"未知"），**绝不用进程 RSS 冒充显存**。
//
// 平台口径：
//   - Linux + AMD ROCm：rocm-smi --showmeminfo vram（X3 的情形）
//   - Linux + NVIDIA：  nvidia-smi --query-gpu=memory.used,memory.total
//   - macOS Apple Silicon：统一内存、无独立显存额度 → 一律 ok=false（不编造）
package monitor

import (
	"os/exec"
	"strconv"
	"strings"
)

// VramUsedGb 返回真实显存占用（GB）；ok=false 表示该平台/该机器拿不到显存（未知）。
func (s *Sampler) VramUsedGb() (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.vramKnown {
		return 0, false
	}
	return roundTo(s.vramUsedGb, 1), true
}

// VramTotalGb 返回真实显存总量（GB）；ok=false 表示未知。
func (s *Sampler) VramTotalGb() (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.vramKnown {
		return 0, false
	}
	return roundTo(s.vramTotalGb, 1), true
}

// VramKnown 报告该机器能否拿到显存（统一内存平台返回 false）。
func (s *Sampler) VramKnown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vramKnown
}

// sampleVram 采样显存，返回 (usedGb, totalGb, ok)。拿不到一律 ok=false。
func sampleVram() (float64, float64, bool) {
	if used, total, ok := readRocmVram(); ok {
		return used, total, true
	}
	if used, total, ok := readNvidiaVram(); ok {
		return used, total, true
	}
	// macOS（Apple Silicon）统一内存：无独立显存额度，如实返回"未知"。
	return 0, 0, false
}

// readRocmVram 通过 rocm-smi 读 AMD 显存（X3 的 ROCm 情形）。
// 输出形如：
//
//	GPU[0] : VRAM Total Memory (B): 25753026560
//	GPU[0] : VRAM Total Used Memory (B): 1234567890
func readRocmVram() (float64, float64, bool) {
	var out []byte
	var err error
	for _, cmd := range []string{"/opt/rocm/bin/rocm-smi", "rocm-smi"} {
		out, err = exec.Command(cmd, "--showmeminfo", "vram").Output()
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, 0, false
	}
	usedB, totalB := parseRocmVram(string(out))
	if totalB <= 0 {
		return 0, 0, false
	}
	const gib = 1 << 30
	return float64(usedB) / gib, float64(totalB) / gib, true
}

// parseRocmVram 解析 rocm-smi 显存输出，返回 (usedBytes, totalBytes)。
// 先匹配 "VRAM Total Used Memory (B)"，再匹配 "VRAM Total Memory (B)"
// （后者是前者子串，故必须先长后短匹配，避免把 used 当 total）。
func parseRocmVram(out string) (uint64, uint64) {
	var used, total uint64
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		// rocm-smi 行形如 "GPU[0]  : VRAM Total Memory (B): 25753026560"——
		// key 里本身带冒号，故按**最后一个**冒号切分取数值。
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		valStr := strings.TrimSpace(line[idx+1:])
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(key, "VRAM Total Used Memory"):
			used = val
		case strings.Contains(key, "VRAM Total Memory"):
			total = val
		}
	}
	return used, total
}

// readNvidiaVram 通过 nvidia-smi 读 NVIDIA 显存（MiB → GB）。
// 输出形如："1234, 24576"（memory.used, memory.total）。
func readNvidiaVram() (float64, float64, bool) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.used,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return 0, 0, false
	}
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0])
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return 0, 0, false
	}
	used, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	total, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || total <= 0 {
		return 0, 0, false
	}
	const mib = 1024.0
	return used / mib, total / mib, true
}
