// fleet_health.go — 「混版机群必须被判为不健康」这一条判据的**唯一实现处**（§二十一 已红第 1 条 · T-23）。
//
// 病灶（《设计-命令面与契约-v0.7.md》§十一.1 逐字）：
//
//	命令面与主控 = 2.5.10 · 本机子端 = 2.5.9 ⇒ 同一命令调用里三件并存，而系统报「全员健康」
//	（healthy_count: 2 · unhealthy_count: 0）⇒ **矩阵数据在、判决没做**。
//
// 依据（照设计稿，不另立口径）：
//
//	· `设计-自动升级模块.md:22` 逐字「混版必须被判为不健康」；
//	· 同稿 `:78` Verify 阶段「可证明过期/不匹配 → exit 1（混版不许当健康）」；
//	· 同稿 `:249` 又补了口径：升级编排**只用 `code_sha`/`code_version` 判混版**，不要用 `healthy`。
//	  ⇒ 这两句合起来的意思就是本条要落的形状：**`healthy` 字段照旧反映子端自报的运行健康**（旧字段不动、
//	  旧消费者零改动），而**计数面**（`healthy_count` / `unhealthy_count`）必须把「混版」判成不健康。
//
// 口径三条（缺一即错，逐条可机检）：
//
//	① **混 = 版本号（`code_version`）与主控自身不同** —— 判据只取版本号，不取 `code_sha`：
//	   `code_sha` 会因本地 dirty 而人人不同（本仓的常态），拿它判混版会把整机群判红，那不叫严格。
//	   版本号不同 = 有一件不是当前发布版 = 混版。
//	② **「没报」≠「一致」** —— 子端没带 `code_version`（空串）时记 `version_unknown`，既不进
//	   `healthy_count` 也不进 `unhealthy_count`：缺 = 未知（同 `vram_known` 那条既有口径），
//	   **绝不把「没报」判成混版、也绝不判成一致**。
//	③ **判决只此一处** —— `StatusHandler` 与任何别的消费方都调本文件的 `CountFleetHealth`，
//	   不许各自再写一份 `snap.Healthy` 的加减（两份实现就会漂，漂的形态就是本条这次的事故）。
package api

import (
	"sort"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// 版本裁决的三个取值（闭集 · 机器可读）。
const (
	VersionSame    = "same"    // 与主控同版本号
	VersionMixed   = "mixed"   // 版本号与主控不同 ⇒ 混版（按设计稿 = 不健康）
	VersionUnknown = "unknown" // 子端没自报版本号 ⇒ 不判（缺 = 未知）
)

// MasterCodeVersion 返回主控自身的版本号（唯一真源：`core/internal/version`）。
// 为什么不在本文件写常量：`core/internal/version/version.go` 是版本号的**唯一**真源
// （`-ldflags` 注入的只是 Commit/BuildTime），再写一份就是又一次硬编码漂移。
func MasterCodeVersion() string {
	return version.Version
}

// MachineVersionVerdict 单台机器的版本裁决（纯函数 · 无副作用）。
//   - machineVersion 为空 ⇒ `unknown`（没报 ≠ 一致 ≠ 混版）
//   - 与 masterVersion 逐字相同 ⇒ `same`
//   - 其余 ⇒ `mixed`
func MachineVersionVerdict(masterVersion, machineVersion string) string {
	switch {
	case machineVersion == "":
		return VersionUnknown
	case machineVersion == masterVersion:
		return VersionSame
	default:
		return VersionMixed
	}
}

// FleetHealth 是一次计数的全部结果（**计数面唯一出口**）。
type FleetHealth struct {
	MasterCodeVersion string   // 主控自身版本号（随响应报出，便于消费方复核判决）
	Healthy           int      // 健康 = 自报 healthy 且版本不混
	Unhealthy         int      // 不健康 = 自报不健康 **或** 混版
	VersionUnknown    int      // 没自报版本号的机器数（不进上面两个计数）
	MixedMachines     []string // 混版机器名（升序；空数组 = 无混版）
}

// Mixed 报告本次计数里是否存在混版机器（顶部 `mixed_version` 字段就是它）。
func (f FleetHealth) Mixed() bool { return len(f.MixedMachines) > 0 }

// CountFleetHealth 遍历快照，按上面三条口径算计数。
//
// 为什么把「混版」算进 unhealthy 而不是只登一个 WARN：设计稿 `:22` 的口径是**判为不健康**，
// 且 `:78` 明写「混版不许当健康」——把混版留在 healthy_count 里正是本条事故的形态。
func CountFleetHealth(masterVersion string, snaps map[string]*store.FleetSnapshot) FleetHealth {
	res := FleetHealth{MasterCodeVersion: masterVersion, MixedMachines: []string{}}
	for name, snap := range snaps {
		if snap == nil {
			continue
		}
		verdict := MachineVersionVerdict(masterVersion, snap.CodeVersion)
		switch verdict {
		case VersionMixed:
			res.MixedMachines = append(res.MixedMachines, name)
		case VersionUnknown:
			res.VersionUnknown++
		}
		if snap.Healthy && verdict != VersionMixed {
			res.Healthy++
		} else {
			res.Unhealthy++
		}
	}
	sort.Strings(res.MixedMachines)
	return res
}

// AnnotateVersionMismatch 把每台的版本裁决写进返回给调用方的**副本**上（`version_mismatch` 字段）。
//
// 为什么走副本：`store.GetAllSnapshots` 返回的是**共享指针**（同一台机器被两个请求同时读），
// 就地改字段 = 数据竞争。副本只有 map 与结构体两层的浅拷贝，字段少、代价可忽略。
func AnnotateVersionMismatch(masterVersion string, snaps map[string]*store.FleetSnapshot) map[string]*store.FleetSnapshot {
	out := make(map[string]*store.FleetSnapshot, len(snaps))
	for name, snap := range snaps {
		if snap == nil {
			continue
		}
		cp := *snap
		cp.VersionMismatch = MachineVersionVerdict(masterVersion, cp.CodeVersion) == VersionMixed
		out[name] = &cp
	}
	return out
}
