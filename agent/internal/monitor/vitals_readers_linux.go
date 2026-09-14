//go:build linux

// vitals_readers_linux.go —— 体征器的 Linux 实读层（X3 的真机路径）。
//
// 真源（全部只读）：
//   - 全局 GTT：/sys/class/drm/card*/device/mem_info_gtt_used（配套 mem_info_gtt_total）。
//     X3 实测（2026-09-14）：card1 上 used=60880695296（空载）/ total=133143986176（÷1024³ ≈ 123.96 GiB）。
//     多卡取和（设计 §8.2：全局 GTT 是账的主口径）。
//   - CPU：/proc/stat 首行 cpu 聚合行，差分算忙闲。
//   - GPU 忙：/sys/class/drm/card*/device/gpu_busy_percent（amdgpu 标准接口，多卡取最大）。
//   - 盘空间：statfs(dataDir)（X3 权重与 KV 盘所在 /data）。
//   - 逐进程归因：/proc/<pid>/fdinfo/* 的 drm-memory-gtt（交叉校验口径，§8.2）。
package monitor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// readGlobalGtt 读全局 GTT：扫 /sys/class/drm/card*/device/mem_info_gtt_{used,total}，多卡取和。
// 一张卡都不成 ⇒ Ok=false（诚实返回"未知"，学 vram.go 纪律）。
func readGlobalGtt() GttSample {
	paths, _ := filepath.Glob("/sys/class/drm/card*/device/mem_info_gtt_used")
	var used, total uint64
	any := false
	for _, p := range paths {
		u, ok := readSysfsUint(p)
		if !ok {
			continue
		}
		t, ok := readSysfsUint(strings.TrimSuffix(p, "used") + "total")
		if !ok {
			continue
		}
		used += u
		total += t
		any = true
	}
	if !any {
		return GttSample{}
	}
	return GttSample{UsedBytes: used, TotalBytes: total, Ok: true}
}

// readSysfsUint 读一个 sysfs 数字文件（末尾换行容忍）。解析本体在 vitals_parse_test.go
// 旁的平台无关 parseSysfsUintLine（两平台共用同一段逻辑、同被单测覆盖）。
func readSysfsUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	return parseSysfsUintLine(string(b))
}

// readProcStatCpu 读 /proc/stat 的 cpu 聚合行，返回 (总节拍, 忙节拍, ok)。
// 忙 = user+nice+system+irq+softirq+steal（idle/iowait 不算忙——空闲时它们是"没在干活"）。
func readProcStatCpu() (uint64, uint64, bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 5 {
			return 0, 0, false
		}
		var vals []uint64
		for _, f := range fields {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			vals = append(vals, v)
		}
		var total, busy uint64
		for i, v := range vals {
			total += v
			switch i {
			case 3, 4: // idle, iowait
			default:
				busy += v
			}
		}
		return total, busy, true
	}
	return 0, 0, false
}

// readGpuBusy 读 /sys/class/drm/card*/device/gpu_busy_percent，多卡取最大值。
// 文件内容是一个百分数（如 "3"）。一张卡都读不到 ⇒ Ok=false。
func readGpuBusy() GpuBusySample {
	paths, _ := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	maxPct := -1.0
	any := false
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil || v < 0 || v > 100 {
			continue
		}
		if v > maxPct {
			maxPct = v
		}
		any = true
	}
	if !any {
		return GpuBusySample{}
	}
	return GpuBusySample{Pct: maxPct, Ok: true}
}

// readDataDisk statfs 数据盘（X3 = /data：权重与 KV 盘所在）。
func readDataDisk(dir string) DiskSample {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return DiskSample{}
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	free := float64(st.Bavail) * float64(st.Bsize) // Bavail = 非特权可用
	return DiskSample{FreeGb: free / (1024 * 1024 * 1024), TotalGb: total / (1024 * 1024 * 1024), Ok: true}
}

// readProcAttribution 扫全 /proc 的 fdinfo，逐进程归因 drm-memory-gtt（只读；交叉校验口径）。
// 读不到 argv 或 gtt=0 的进程跳过（非 GPU 进程不是归因对象）。
// 权限不足的 /proc/<pid>/fdinfo 读不了 ⇒ 跳过即可（§2.2：他人进程只读得到的才进账）。
func readProcAttribution() AttribSample {
	s := AttribSample{Ok: false}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return s
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fds, err := filepath.Glob(filepath.Join("/proc", e.Name(), "fdinfo", "*"))
		if err != nil || len(fds) == 0 {
			continue
		}
		var kib float64
		for _, fd := range fds {
			b, err := os.ReadFile(fd)
			if err != nil {
				continue
			}
			kib += parseDrmGttKib(string(b))
		}
		if kib <= 0 {
			continue
		}
		argv0 := readProcArgv0(pid)
		if argv0 == "" {
			continue
		}
		gb := kib / (1024 * 1024)
		s.Procs = append(s.Procs, ProcAttrib{PID: pid, Argv0: argv0, GttGb: gb})
		s.SumGb += gb
		s.Ok = true
	}
	return s
}

// parseDrmGttKib 从一段 fdinfo 文本解析 drm-memory-gtt 的 KiB 合计。
// X3 实测格式（冒号后空格/制表符混排）：
//
//	drm-memory-vram:	2208 KiB
//	drm-memory-gtt: 	34286572 KiB
//
// 与 backend.parseDrmMemoryGb 同源但不同名不同位：monitor 这里只要 gtt 单项，
// 且以 KiB 为中间单位（逐进程求和做交叉校验用），不与 backend 的 gtt+vram 合计混淆。
func parseDrmGttKib(text string) float64 {
	var kib float64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "drm-memory-gtt:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "drm-memory-gtt:"))
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(fields[1]) {
		case "kib":
			kib += v
		case "mib":
			kib += v * 1024
		case "gib":
			kib += v * 1024 * 1024
		}
	}
	return kib
}

// readProcArgv0 读 /proc/<pid>/cmdline 的第一个参数（归因展示用）。
func readProcArgv0(pid int) string {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	parts := strings.Split(string(b), "\x00")
	for _, p := range parts {
		if p != "" {
			return p
		}
	}
	return ""
}
