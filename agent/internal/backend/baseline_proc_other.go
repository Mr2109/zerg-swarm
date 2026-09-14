//go:build !linux

// baseline_proc_other.go —— 非 Linux 平台（如主控所在的 macOS）的桩实现。
//
// 设计 §11 M14：借用与基线检查**仅 Linux 适用**（依赖 /proc、pidfd、llama-server /slots）。
// 非 Linux 上 findListenerProcess 一律返回"未定位"，于是 BaselineServices() 只报端口与身份，
// 不臆造 pid/托管方式——行为与 resident.go 的只读探测保持一致。
package backend

// procInfo 与 Linux 版同形（字段故意保留，避免上层出现平台分支）。
type procInfo struct {
	PID       int
	Argv      []string
	Cwd       string
	StartTime uint64
	Cgroup    string
	Ancestry  [][]string
	MemGB     float64 // 与 Linux 版同形；本平台实际取到的就是 RSS 口径
}

// findListenerProcess 非 Linux 平台不做进程定位（返回未命中）。
func findListenerProcess(port int) (procInfo, bool) {
	return procInfo{}, false
}

// readProcessMemGb 非 Linux 平台没有 drm fdinfo ⇒ 回落 RSS（RSS 本身就是该平台的可用口径）。
func readProcessMemGb(pid int) float64 { return readProcessRssGb(pid) }

// currentStartTime 非 Linux 平台取不到 start_time ⇒ 返回 0（守卫据此判定"未生效"而非误伤，
// 见 startTimeMismatch；借用机制本身也只在 Linux 落地，设计 §11 M14）。
func currentStartTime(pid int) uint64 { return 0 }
