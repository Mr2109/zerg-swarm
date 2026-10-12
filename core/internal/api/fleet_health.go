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
//	④ **同源判定另立一格**（GAP-20261013-09 · 候选 c）—— `same_origin`：`same`/`mixed`/`unknown`。
//	   它比的**不是**版本号、也**不是** `code_sha`，而是**源提交**（`origin_sha` ＝ 镜像 tip 的
//	   `GitOrigin-RevId` trailer ＝ 私有图源提交）：主控那行报私有 sha、节点那行报镜像 tip，
//	   两套 git 命名空间不同名 ⇒ 矩阵里「全绿也可能是混版」。✗ 判不了（缺映射/trailer 缺/没报）
//	   一律 `unknown` —— **缺 ≠ 一致**，绝不折叠成 `same`。
package api

import (
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// 版本裁决的三个取值（闭集 · 机器可读）。
const (
	VersionSame    = "same"    // 与主控同版本号
	VersionMixed   = "mixed"   // 版本号与主控不同 ⇒ 混版（按设计稿 = 不健康）
	VersionUnknown = "unknown" // 子端没自报版本号 ⇒ 不判（缺 = 未知）
)

// 「同源判定」的三个取值（闭集 · 机器可读 · GAP-20261013-09 · 候选 c）。
//
// 病灶：主控那行报**私有提交 sha**、节点那行报**镜像 tip sha** —— 两个 git 命名空间里的字符串，
// 矩阵里没有可比键 ⇒ 三台其实同源也看不出（「全绿也可能是混版」）。
// 治法：把各行的身份折成**规范键**（`origin_sha` ＝ 镜像 tip 的 `GitOrigin-RevId` trailer），
// 逐行与主控基准比，把判决**印在行上**。
//
// ★ 三态里 **`unknown` 必须保留（缺 ≠ 一致）**：映射缺 / trailer 缺 / 该机没带该格 ⇒ 一律 `unknown`，
// ✗ 绝不折叠成 `same` —— 那正是「防假绿」这一条要挡的形态。
const (
	OriginSame    = "same"    // 与主控同源（同一笔源提交）
	OriginMixed   = "mixed"   // 源提交与主控不同 ⇒ 真混版
	OriginUnknown = "unknown" // 判不了（主控或该机没带可比键）—— 缺 ≠ 一致
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

// MasterOriginSHA 返回主控自身的**源提交**（同源判定的基准键 · GAP-20261013-09）。
//
// 真源与 `MasterCodeVersion` 同一条：只有 `core/internal/version.Origin`（构建期 -ldflags 注入）。
// ★ 未注入 ⇒ 空串 = **判不了** ⇒ 全表 `same_origin` 一律 `unknown`（fail-closed）：✗ 绝不在本函数
// 里回落成 `version.Commit` —— 主控 `Commit` 是私有提交、节点 `Commit` 是镜像 tip，拿它当基准会把
// 「同源」判成 `mixed`（假红）。宁可报判不了，也不给一句看着像结论的假判决。
func MasterOriginSHA() string {
	return version.Origin
}

// MachineOriginVerdict 单台机器的**同源裁决**（纯函数 · 无副作用 · GAP-20261013-09）。
//
//	· 主控基准不可用（空串 / "unknown"）⇒ `unknown`（**判不了**：没有基准就没有判决）
//	· 该机不可用（空串 / "unknown" ＝ 没带源提交 / trailer 缺）⇒ `unknown`（缺 ≠ 一致）
//	· 两者逐字相同 ⇒ `same`
//	· 其余 ⇒ `mixed`（真混版）
//
// ★ 顺序不可颠倒：**先判两侧缺、再判等** —— 否则两个空串会「相等」⇒ 把「都没报」判成「同源」，
// 那正是「缺 ≠ 一致」要挡的假绿。`unknown` 这个字面量也当缺（`Commit` 未注入时的旧哨兵同形）。
func MachineOriginVerdict(masterOrigin, machineOrigin string) string {
	if !originUsable(masterOrigin) || !originUsable(machineOrigin) {
		return OriginUnknown
	}
	if machineOrigin == masterOrigin {
		return OriginSame
	}
	return OriginMixed
}

// originUsable 判一个源提交值是否**可用于比对**：空串与 "unknown"（未注入的旧哨兵）都不算。
func originUsable(origin string) bool {
	o := strings.TrimSpace(origin)
	return o != "" && o != "unknown"
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

// AnnotateIdentityVerdicts 把每台的两个身份裁决**一次拷贝**写进返回给调用方的**副本**上
// （`version_mismatch` 版本裁决 ＋ `same_origin` 同源裁决三态 · GAP-20261013-09）。
//
// 为什么走副本：`store.GetAllSnapshots` 返回的是**共享指针**（同一台机器被两个请求同时读），
// 就地改字段 = 数据竞争。副本只有 map 与结构体两层的浅拷贝，字段少、代价可忽略。
//
// 为什么两个裁决走**同一趟拷贝**：分两趟会让后一趟把前一趟写的字段抹掉（同一条响应里只剩一个
// 判决）；本仓「判决只此一处」的口径下，拷贝这件事也只该有一处。
//
// ★ 同名旧函数 `AnnotateVersionMismatch` 已并入本函数（改名 ＋ 多一个入参）：全仓调用点只有
// `handlers.go` 的 `StatusHandler` 一处（现读逐条核过），✗ 不留两份拷贝实现。
func AnnotateIdentityVerdicts(masterVersion, masterOrigin string, snaps map[string]*store.FleetSnapshot) map[string]*store.FleetSnapshot {
	out := make(map[string]*store.FleetSnapshot, len(snaps))
	for name, snap := range snaps {
		if snap == nil {
			continue
		}
		cp := *snap
		cp.VersionMismatch = MachineVersionVerdict(masterVersion, cp.CodeVersion) == VersionMixed
		// 同源裁决：比 `OriginSHA`（规范键），✗ 不比 `CodeSHA`（两套 git 命名空间，比不出结论）。
		cp.SameOrigin = MachineOriginVerdict(masterOrigin, cp.OriginSHA)
		out[name] = &cp
	}
	return out
}
