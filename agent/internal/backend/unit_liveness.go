// unit_liveness.go —— 卵**单元活性**核验（2026-09-19 ②）。
//
// 现象（真机）：`GET /eggs` 报 `state=ready · gtt_gb=78.2`，而同一时刻机器上 `GTT 用 0.0 GiB ·
// ds4 进程 0`；`watchdog.verdict = "ok"`。危害：主控按「有活儿能派」把请求路由过去 ⇒ 客户端拿到
// 503/0 token；人工排查被「看起来活着」误导（当日已发生）。
//
// 病灶（两条）：
//   - 卵表项直接来自 backend 自报的**状态机字段**，没有任何单位活体核验；
//   - 活性看门狗只看「输出/CPU 工时增量」，**不看进程是否还在**（进程没了 ⇒ 工时自然不动，
//     但那要等一个窗口 + 采样才判，且判词是 stuck_*，不是「单元死了」）。
//
// 本文件的处置（保守：只加核验与判词，不改既有字段语义、**不删记录**）：
//
//	① 卵表输出前核 unit 活性（`systemctl --user show <unit> -p ActiveState …`；
//	   只需「是不是还在活动」的等价物都行 —— 复用既有只读探针 unitStateProbe，不新造一套读法）；
//	② 单位死亡 ⇒ 由消费侧（server 组装 /eggs）把 state 落 `dead`（单元不存在/核不到 ⇒ `missing`）、
//	   并补 `unit_state` 字段与看门狗判词 `dead_unit`；
//	③ **绝不因「进程死了」把卵记录删掉**：记录要留着当证据，清理归既有卸载/GC 路径（egg_gc.go）。
//
// 口径（写死，与 unit_probe.go 的 dead() 同一精神）：
//   - 只有**明确读到** Terminal 态（inactive / failed）才算「死了」；
//   - 读不到（非 Linux / 没有 systemctl / 命令失败且没有属性行）⇒ `missing`（「不知道」也如实说，
//     不当「活着」也不当「死了」——但它是**要人看见**的状态，故照样上观测面）；
//   - 过渡态（activating / reloading / deactivating）一律算**活着**（不误杀慢启动的正确卵）。
package backend

import (
	"fmt"
	"strings"
)

// UnitLiveness 一枚卵单元此刻的活性结论（只读；供观测面 /eggs 的 `unit_state` 用）。
type UnitLiveness struct {
	// State 归一后的单元态：active / activating / reloading / deactivating / inactive / failed /
	// missing（核不到：非 Linux、没有 systemctl、命令失败且无属性行）。
	State string
	// Alive 是否**明确在活动**（过渡态算活动；inactive/failed/missing 为 false）。
	Alive bool
	// Detail 一句话留痕（含 systemd 原文各项；便于复盘「到底核到了什么」）。
	Detail string
}

// Dead 是否**明确已死**（读不到 = missing 也算「不能报 ready」，但语义上不是「读到反证」——
// 消费侧两者都落非 ready，判词却不同：dead_unit 的理由里会写明是哪一种）。
func (l UnitLiveness) Dead() bool { return !l.Alive && l.State != "" }

// unitLivenessOf 由单元状态快照得出活性结论（纯函数，便于单测逐条喂）。
func unitLivenessOf(unit string, st unitState, probeErr error) UnitLiveness {
	as := strings.ToLower(strings.TrimSpace(st.ActiveState))
	if as == "" {
		// 没有任何属性行 ⇒ 核不到。**不许**当「活着」（那正是本缺陷的形态：状态机说 ready）。
		detail := fmt.Sprintf("核不到单元 %s 的 ActiveState", unit)
		if probeErr != nil {
			detail = fmt.Sprintf("%s：%v", detail, probeErr)
		}
		return UnitLiveness{State: "missing", Alive: false, Detail: detail}
	}
	lv := UnitLiveness{State: as}
	switch as {
	case "active", "activating", "reloading", "deactivating":
		lv.Alive = true
	}
	why := fmt.Sprintf("ActiveState=%s SubState=%s Result=%s ExecMainStatus=%d",
		orDash(st.ActiveState), orDash(st.SubState), orDash(st.Result), st.ExecMainStatus)
	lv.Detail = fmt.Sprintf("单元 %s：%s", unit, why)
	return lv
}

// UnitLiveness 核一枚卵单元此刻的活性（生产入口；只读，不发任何动作）。
//
// 单元名为空（裸 exec 路径：引擎是本端进程，不属任何单元）⇒ State 留空、Alive=false、
// Detail 如实说明「没有单元」——调用方据此**不出** unit_state 字段（缺席，不编造）。
func (m *Manager) UnitLiveness(unit string) UnitLiveness {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return UnitLiveness{Detail: "裸 exec 路径没有单元（本端进程，不属任何单元）"}
	}
	probe := unitStateProbe
	if probe == nil {
		probe = probeUnitState
	}
	st, err := probe(unit)
	return unitLivenessOf(unit, st, err)
}
