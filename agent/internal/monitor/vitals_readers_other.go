//go:build !linux

// vitals_readers_other.go —— 虫须的非 Linux 桩层（开发机 darwin / 其它平台）。
//
// 学 vram.go / baseline_proc_other.go 的诚实纪律：拿不到就 ok=false，绝不编数冒充。
// 本设计是 Linux-only（§12.1 第 4 条：依赖 /proc/<pid>/fdinfo、/sys/class/drm）；
// mac 主控机上虫须可编译、可跑、可测（环/钩子/分层逻辑全在平台无关层），只是
// 五类读数一律"未知"。
package monitor

import "syscall"

// readGlobalGtt 非 Linux 无 /sys/class/drm ⇒ 未知。
func readGlobalGtt() GttSample { return GttSample{} }

// readProcStatCpu 非 Linux 无 /proc/stat ⇒ 未知。
func readProcStatCpu() (uint64, uint64, bool) { return 0, 0, false }

// readGpuBusy 非 Linux 无 gpu_busy_percent ⇒ 未知。
func readGpuBusy() GpuBusySample { return GpuBusySample{} }

// readDataDisk 非 Linux 也用 statfs（macOS 上对任意目录都能拿到真实盘空间）。
func readDataDisk(dir string) DiskSample {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return DiskSample{}
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	free := float64(st.Bavail) * float64(st.Bsize)
	return DiskSample{FreeGb: free / (1024 * 1024 * 1024), TotalGb: total / (1024 * 1024 * 1024), Ok: true}
}

// readProcAttribution 非 Linux 无 /proc ⇒ 空归因（Ok=false）。
func readProcAttribution() AttribSample { return AttribSample{} }

// parseDrmGttKib 非 Linux 平台保留解析纯函式（用 X3 实测样例驱动单测，保证
// Linux 实读层同款格式的解析逻辑在 darwin 上也能被验证）。
func parseDrmGttKib(text string) float64 { return parseDrmGttKibCommon(text) }

// readProcArgv0 非 Linux 无 /proc ⇒ 空。
func readProcArgv0(pid int) string { return "" }
