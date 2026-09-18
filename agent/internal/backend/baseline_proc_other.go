//go:build !linux

// baseline_proc_other.go —— 非 Linux 平台（如主控所在的 macOS）的桩实现。
//
// 设计依据：设计-子端沙箱化.md §10.1 baseline_proc_linux.go 行——
// findListenerProcess / readProcessMemGb 的非 Linux 桩；存档类字段已随 P4 退场（附录 C·C2）。
package backend

// procInfo 与 Linux 版同形（字段故意保留，避免上层出现平台分支）。
type procInfo struct {
	PID    int
	Argv   []string
	Cgroup string
	MemGB  float64 // 与 Linux 版同形；本平台实际取到的就是 RSS 口径
}

// findListenerProcess 非 Linux 平台不做进程定位（返回未命中）。
func findListenerProcess(port int) (procInfo, bool) {
	return procInfo{}, false
}

// readProcessMemGb 非 Linux 平台没有 drm fdinfo ⇒ 回落 RSS（RSS 本身就是该平台的可用口径）。
func readProcessMemGb(pid int) float64 { return readProcessRssGb(pid) }
