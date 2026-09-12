package gateway

// ledger_route.go —— 批 5：把资源账本接进候选打分（《设计-资源管理器》§三.5）。
//
// 分工（§3.5 关键边界）：资源管理器**只提供输入**，路由**做决定**——
// 它不选端点、不改路由表；它只回答"这台机器 + 这个模型，跑不跑得动 / 要不要加载"。
//
// 本文件把设计稿 §3.5 的判据接进既有打分（打分循环在 gateway.go，语义不动，只叠加增量）：
//
//	② 可用性   —— 该机"跑得动这个模型吗"：内存与显存**二者取严**（§3.3d），确定不够 → 降权；
//	③ 成本/延迟 —— 冷/温/热：该机报有驻留账本、但目标不在驻留清单 → "冷"（需加载）降权；
//	                目标正在 loading → 降权（正被别的请求等待加载）。
//
// 红线（本批硬约束，测试逐条钉住）：
//  1. **账本缺失/未知 → 增量 0，完全退回既有打分**：绝不因"不知道"就把候选降权/排掉。
//  2. **只降权、不摘除**：任何情况下候选仍可被选中（绝不因账本把所有候选排没）。
//  3. 显存未知（非统一内存）**不降权**——"不知道"既不是"够"也不是"不够"；
//     只有该机真报了显存、且真不够时才降权。
//  4. 本文件**不引入任何"为腾地方而驱逐/接管未托管进程"的路径**：
//     它只读快照算分；腾退仍只在既有 ensureX3RoomForFile/yieldX3To（批 3）里发生。

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// 账本降权分档（相对既有打分量级：已加载目标 +8 / 满负载 -8 / 单槽忙 -5）。
// 只降权、不摘除——候选始终留在池子里，只是不再优先。
const (
	ledgerNoFitPenalty   = -6 // ② 确定装不下（内存或显存任一不够，§3.3d 二者取严）
	ledgerLoadingPenalty = -3 // ③ 目标模型正在加载中（正被别的请求等待）
	ledgerColdPenalty    = -2 // ③ 目标不在该机驻留清单（需加载——冷/温态，比热态慢）
)

// ledgerPresent 报告该机是否真的提供了资源账本（驻留明细或显存形态）。
//
// 这是红线①的落地点：批 2 之前的子端只报内存/状态（没有 resident[]、没有 vram_*），
// 那种快照判不了"能不能装/要不要加载"——一律返回 0，路由行为与改动前逐字一致。
func ledgerPresent(snap *store.FleetSnapshot) bool {
	if snap == nil {
		return false
	}
	return len(snap.Resident) > 0 || snap.VramKnown || snap.VramUnified
}

// ledgerAdjust 返回资源账本给该候选的**打分增量**（0 = 账本无可用输入 → 退回既有行为）。
// 只降权，绝不移除候选；账本缺席/判不了时返回 0。
func (g *Gateway) ledgerAdjust(snap *store.FleetSnapshot, cand config.ModelCandidate) (int, string) {
	if !ledgerPresent(snap) {
		return 0, ""
	}
	// 目标已驻留在**该机**（缓存/显存里）：既有"已加载 +8"已表达优先，这里只在异常档降权——
	//   · loading：正被别的请求等待加载 → 降权；
	//   · crashed/zombie：账本如实说它不可用（要重载）→ 按"冷"降权，不让假热态占优。
	if st := residentStateOf(snap, cand.File); st != "" || modelFileLoaded(snap, cand.File) {
		switch st {
		case resources.StateLoading:
			return ledgerLoadingPenalty, "ledger:target-loading"
		case resources.StateCrashed, resources.StateZombie:
			return ledgerColdPenalty, "ledger:target-unusable"
		default:
			return 0, ""
		}
	}
	// ② 可用性：只有**确定**不够（内存或显存任一不够）才降权；判不了（机器没报内存/显存未知）不降权。
	if short, why := g.ledgerShort(snap, cand); short {
		return ledgerNoFitPenalty, why
	}
	// ③ 成本/延迟：该机报有驻留账本、目标不在其中 → 需加载（冷/温），比热态慢。
	if len(snap.Resident) > 0 {
		return ledgerColdPenalty, "ledger:cold(needs-load)"
	}
	return 0, ""
}

// ledgerShort 报告该机是否**确定**装不下候选（§3.3d：内存与显存二者取严）。
//
// 只判"确定"：机器没报内存总量、候选没报内存需求 → 判不了（false，红线①）；
// 显存只在**该机真报了显存**时才参与判定（显存未知 ≠ 显存无限，也 ≠ 显存不足，红线③）。
func (g *Gateway) ledgerShort(snap *store.FleetSnapshot, cand config.ModelCandidate) (bool, string) {
	if snap.MemTotalGb <= 0 || cand.MemGb <= 0 {
		return false, ""
	}
	need := float64(cand.MemGb)
	// 内存：可用 + **我们有权腾退**的驻留占用（未托管进程的占用不算我们的，Q6）。
	if memFree := snap.MemAvailableGb + actionableOccupiedGb(snap.Resident); need > memFree {
		return true, fmt.Sprintf("ledger:no_fit(mem need=%.0f>free=%.0f)", need, memFree)
	}
	// 显存：仅当该机报了真显存；取严（含可腾退占用——单槽机器腾一个就够）。
	if snap.VramKnown {
		if vramFree := snap.VramFreeGb + actionableOccupiedGb(snap.Resident); need > vramFree {
			return true, fmt.Sprintf("ledger:no_fit(vram need=%.0f>free=%.0f)", need, snap.VramFreeGb)
		}
	}
	return false, ""
}

// actionableOccupiedGb 汇总"我们有权动作"的驻留项占用（GiB）——与动作侧同口径：
// 在飞 / 未托管 / pin 未到期 / 加载中 / 无法寻址者不计入（§八 Q2/Q5/Q6）。
func actionableOccupiedGb(residents []resources.ResidentEntry) float64 {
	var sum float64
	for _, r := range residents {
		if _, allowed := resources.ActionBlockReason(r); allowed {
			sum += resources.OccupiedGb(r)
		}
	}
	return sum
}

// residentStateOf 返回候选权重在该机驻留清单里的状态；不在清单里返回 ""（不知道/未驻留）。
// 匹配口径与 modelFileLoaded 一致（basename 去 .gguf，再按逻辑名去量化后缀比对），
// 因为子端上报的 File 可能是完整路径、Alias 是逻辑名。
func residentStateOf(snap *store.FleetSnapshot, file string) string {
	if snap == nil || file == "" {
		return ""
	}
	for _, r := range snap.Resident {
		if residentMatchesFile(r, file) {
			return r.State
		}
	}
	return ""
}

// residentMatchesFile 报告一条驻留项是否就是候选权重（同 modelFileLoaded 的宽松口径）。
func residentMatchesFile(r resources.ResidentEntry, file string) bool {
	target := strings.TrimSuffix(filepath.Base(file), ".gguf")
	if target == "" {
		return false
	}
	stem := stripQuantSuffix(target)
	for _, c := range []string{r.File, r.Alias} {
		c = strings.TrimSuffix(c, ".gguf")
		c = filepath.Base(c)
		if c == "" || c == "." || c == "/" {
			continue
		}
		if c == target || (stem != "" && c == stem) {
			return true
		}
	}
	return false
}

// logLedgerAdjust 记录一次账本降权（只读观测；账本缺席时什么都不打）。
func logLedgerAdjust(host, model string, delta int, why string) {
	if delta != 0 {
		log.Printf("🧮 ledger adjust: host=%s model=%s delta=%+d (%s)", host, model, delta, why)
	}
}
