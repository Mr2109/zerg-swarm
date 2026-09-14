// dual_gate.go —— 双闸门纯函式（GTT 账 + 内存账，两账都过才装；fail-closed）。
//
// 设计真源：§8.4 准入闸门算法：
//   - 读账（只读，不产生副作用）→ gtt_free ≥ need_gtt 且 mem_free ≥ need_mem ⇒ 放行；
//   - 否则拒装，reason 要能列「谁挡着 / 差多少」（507 detail 的原料）；
//   - 两条腿都要（§8.3）：只用内存账会重演「子端说够、引擎说不够」；只用 GTT 账
//     漏掉 CPU 侧。统一内存上 MemAvailable 已含 GTT 占用 ⇒ **不要二次扣减**。
//
// 纯函式：不读文件、不碰采样器、不起 goroutine——账的全部输入由调用方传入，
// 时钟注入由参数携带（可测性优先）。backend 接线归 P4 后统一。
package monitor

import "fmt"

// DualGateResult 双闸门裁决结果。
type DualGateResult struct {
	// Ok 两账都过（任一不过即 false）。
	Ok bool
	// Reason 不过时的人读原因（「谁挡着 / 差多少」——507 detail 的原料）。
	Reason string
	// GttShortGb / MemShortGb 各账差额（>0 表示缺多少 GB；过闸时为 0）。
	GttShortGb float64
	MemShortGb float64
}

// GateAccount 双闸门的单侧账（GTT 或内存）。
type GateAccount struct {
	// AvailGb 当前可用（GTT 侧=预算−已用−reserve；内存侧=MemAvailable−reserve）。
	AvailGb float64
	// NeedGb 这枚卵要多少（实测档案口径，通常已乘安全系数）。
	NeedGb float64
}

// checkDualGateArgs 双闸门入参（结构化，防位置参数写反——gb 量纲同名易错）。
type checkDualGateArgs struct {
	Gtt GateAccount
	Mem GateAccount
}

// CheckDualGate 双闸门裁决：两账都过才装；任一不过 ⇒ Ok=false + 差额 + 原因。
//
// 纯函式；输入允许负值/NaN？——负可用按 0 处理照样判不够（fail-closed），
// 不在这里做"修正"，差额如实反映负数（宁可怕人，不许骗人）。
func CheckDualGate(gttAvailGb, memAvailGb, gttNeedGb, memNeedGb float64) DualGateResult {
	return checkDualGate(checkDualGateArgs{
		Gtt: GateAccount{AvailGb: gttAvailGb, NeedGb: gttNeedGb},
		Mem: GateAccount{AvailGb: memAvailGb, NeedGb: memNeedGb},
	})
}

// CheckDualGateWithReserve 带 reserve 的便利入口：先从两侧可用里各扣 reserve_gb
// 再裁决（§8.3：gtt_free = budget − used − reserve_gb；mem_free = MemAvailable − reserve_gb）。
// reserve < 0 视为 0（不许负预留变相放大可用）。
func CheckDualGateWithReserve(gttAvailGb, memAvailGb, gttNeedGb, memNeedGb, reserveGb float64) DualGateResult {
	if reserveGb < 0 {
		reserveGb = 0
	}
	return checkDualGate(checkDualGateArgs{
		Gtt: GateAccount{AvailGb: gttAvailGb - reserveGb, NeedGb: gttNeedGb},
		Mem: GateAccount{AvailGb: memAvailGb - reserveGb, NeedGb: memNeedGb},
	})
}

func checkDualGate(a checkDualGateArgs) DualGateResult {
	gttShort := a.Gtt.NeedGb - a.Gtt.AvailGb
	memShort := a.Mem.NeedGb - a.Mem.AvailGb
	res := DualGateResult{
		GttShortGb: max0(gttShort),
		MemShortGb: max0(memShort),
	}
	switch {
	case gttShort > 0 && memShort > 0:
		res.Reason = fmt.Sprintf("两账都不够：GTT 缺 %.1f GB（可用 %.1f < 需 %.1f），内存缺 %.1f GB（可用 %.1f < 需 %.1f）",
			gttShort, a.Gtt.AvailGb, a.Gtt.NeedGb, memShort, a.Mem.AvailGb, a.Mem.NeedGb)
	case gttShort > 0:
		res.Reason = fmt.Sprintf("GTT 账不够：可用 %.1f < 需 %.1f（缺 %.1f GB）", a.Gtt.AvailGb, a.Gtt.NeedGb, gttShort)
	case memShort > 0:
		res.Reason = fmt.Sprintf("内存账不够：可用 %.1f < 需 %.1f（缺 %.1f GB）", a.Mem.AvailGb, a.Mem.NeedGb, memShort)
	default:
		res.Ok = true
	}
	return res
}

// CanHatchWithProfile 判据函式：**没有实测档案（或档案校验不过）不许孵**（标定铁律）。
//
// 这是"档案缺失拒孵"的判据纯函式：profile 有效 + 双闸门过 ⇒ 放行；
// 档案无效 ⇒ Ok=false 且 reason 说明档案问题（与账无关）。
// profileOk=false 时传零值 EggProfile 即可（本函式只看 profileOk 与账）。
func CanHatchWithProfile(profileOk bool, gttAvailGb, memAvailGb float64, p EggProfile) DualGateResult {
	if !profileOk {
		return DualGateResult{
			Reason: "无有效实测档案，拒孵（标定铁律 §8.4：闸门与预算只读实测档案，卵声明里的估值不参与）",
		}
	}
	return CheckDualGate(gttAvailGb, memAvailGb, p.GttNeed(), p.MemNeed())
}

func max0(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
