// Package resources 是 core 侧对共享裁决库（shared/resources）的转发层。
//
// 为什么是转发层（而不是把实现留在 core/internal）：
// 《设计-资源管理器》批 3 要求子端（agent/internal/backend）与主控（core）用**同一份**
// 驻留/驱逐裁决（§八 Q1/Q2/Q5）。Go 的 internal 规则只允许 core 模块内的路径导入
// core/internal/...，子端模块（github.com/Mr2109/zerg-swarm/agent）导不到这里；
// 因此实现上移到第三方共享模块 github.com/Mr2109/zerg-swarm/shared/resources，
// 本包只做类型/常量/函式转发，使既有导入路径（store 等）与既有测试**一字不改**继续有效。
//
// 边界与纪律（与实现同源，详见 shared/resources 的包注释）：
//   - 纯计算：无 IO、无系统命令、不联网、不读时钟；
//   - 诚实优先：用经验常量回退必标 Estimated=true，估不出即 fail-closed；
//   - 只出计划不动手：本包不卸载任何进程。
package resources

import (
	shared "github.com/Mr2109/zerg-swarm/shared/resources"
)

// ── 类型转发（类型别名：与共享实现是**同一个类型**，JSON 形状与类型身份不变）──

type (
	// ResidentEntry 一台机器上的一个驻留模型。
	ResidentEntry = shared.ResidentEntry
	// UnmanagedProcess 未托管但占着资源的进程/端口（只报告）。
	UnmanagedProcess = shared.UnmanagedProcess
	// MachineLedger 一台机器的资源账本。
	MachineLedger = shared.MachineLedger
	// RankedEviction 可驱逐项（含排序键）。
	RankedEviction = shared.RankedEviction
	// ProtectedEntry 绝不驱逐的驻留项及原因。
	ProtectedEntry = shared.ProtectedEntry
	// EvictionPlan 驱逐排序结果。
	EvictionPlan = shared.EvictionPlan
	// FitQuery "跑得动吗"的模型侧输入。
	FitQuery = shared.FitQuery
	// FitEstimate "跑得动吗"的可解释估算。
	FitEstimate = shared.FitEstimate
	// ArchKVFallback 架构族的 KV 回退参数。
	ArchKVFallback = shared.ArchKVFallback
)

// ── 常量转发 ────────────────────────────────────────────────────────────────

const (
	// DefaultMaxResident 默认驻留上限（§八 Q1：默认单槽）。
	DefaultMaxResident = shared.DefaultMaxResident

	StateLoading = shared.StateLoading
	StateReady   = shared.StateReady
	StateCrashed = shared.StateCrashed
	StateIdle    = shared.StateIdle
	StateZombie  = shared.StateZombie

	TierReclaim = shared.TierReclaim
	TierIdle    = shared.TierIdle
	TierWait    = shared.TierWait

	VerdictFit   = shared.VerdictFit
	VerdictEvict = shared.VerdictEvict
	VerdictNoFit = shared.VerdictNoFit

	DefaultBytesPerElem = shared.DefaultBytesPerElem
	DefaultOverheadGb   = shared.DefaultOverheadGb
)

// ── 函式转发 ────────────────────────────────────────────────────────────────

// ProtectedReason 应用两条硬规则（在飞绝不驱逐 / pin 未到期不驱逐）。
func ProtectedReason(r ResidentEntry) (string, bool) { return shared.ProtectedReason(r) }

// RankEvictions 把驻留项排成可驱逐顺序（§八 Q2 五档，只排序不动作）。
func RankEvictions(residents []ResidentEntry) EvictionPlan { return shared.RankEvictions(residents) }

// EvictableDigests 返回可驱逐项的有序摘要列表（含受保护项的排除）。
func EvictableDigests(residents []ResidentEntry) []string { return shared.EvictableDigests(residents) }

// EvictToFree 按驱逐顺序取"够用的最小前缀"（纯排序口径，含未托管项）。
func EvictToFree(residents []ResidentEntry, needFreeGb float64) []string {
	return shared.EvictToFree(residents, needFreeGb)
}

// TotalEvictableGb 汇总全部可驱逐项的占用（GiB）。
func TotalEvictableGb(residents []ResidentEntry) float64 { return shared.TotalEvictableGb(residents) }

// OccupiedGb 单个驻留项的占用估计（GiB）：声明内存优先，缺失回退实测 RSS。
func OccupiedGb(r ResidentEntry) float64 { return shared.OccupiedGb(r) }

// TargetID 动作接口上的寻址标识（别名优先，回退摘要）。
func TargetID(r ResidentEntry) string { return shared.TargetID(r) }

// ActionBlockReason 报告该驻留项不能被动做的原因（在飞/未托管/pin 未到期/无法寻址）。
func ActionBlockReason(r ResidentEntry) (string, bool) { return shared.ActionBlockReason(r) }

// EvictPlanForAction 返回"只卸够"的目标序列（只含我们有权动作的项）。
func EvictPlanForAction(residents []ResidentEntry, needFreeGb float64) []string {
	return shared.EvictPlanForAction(residents, needFreeGb)
}

// PlannedFreeGb 汇总一份动作计划能腾出的内存（GiB）。
func PlannedFreeGb(residents []ResidentEntry, targets []string) float64 {
	return shared.PlannedFreeGb(residents, targets)
}

// EstimateFit 对 MachineLedger 做"跑得动吗"估算（三要素、可解释、fail-closed）。
func EstimateFit(q FitQuery, m MachineLedger) FitEstimate { return shared.EstimateFit(q, m) }

// ArchFallback 查架构族的 KV 回退常量；ok=false 表示该族未知（调用方须 fail-closed）。
func ArchFallback(family string) (ArchKVFallback, bool) { return shared.ArchFallback(family) }

// SupportedArchFamilies 返回已登记的架构族名。
func SupportedArchFamilies() []string { return shared.SupportedArchFamilies() }
