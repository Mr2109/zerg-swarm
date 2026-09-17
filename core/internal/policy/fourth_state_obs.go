// fourth_state_obs.go — B 项⑤：**第 4 态「追问/阻滞」的观测出口**（设计稿 v2.1 §6.1 的 `slice_escalated`）。
//
// 为什么要单独一个文件：`fourth_state.go` 是本态的**判定**实现（一行判定都不许被观测改动），
// 本文件只放**观测**（组装事件 + 调用唯一写入出口 `sliceobs.Emit`）。
// 判定侧调用点只有两处、各一行（见 fourth_state.go 的 `emitSliceEscalatedFromRaise` /
// `emitSliceEscalatedFromSettle` 调用注释）——「进态」「结算成升级」。
//
// ── 接线口径（写死在这里，由用例钉住）────────────────────────────────────
//
//	① **进态**（`RaiseClarification` 成功开单）⇒ 一行 `slice_escalated`，outcome = `raised`。
//	   语义：片因信息不足**升级**到上游/人（退回补规格），不放行任何东西。
//	② **结算成升级类**（`SettleClarification` 得到 `escalate` 或 `void`）⇒ 一行 `slice_escalated`：
//	     · `FourthEscalate`（上游拒答/无规格可补）⇒ outcome = `refused`
//	     · `FourthVoid` 且 `TimedOut`（到点未答 ⇒ 既有逐类语义作废）⇒ outcome = `timeout`
//	     · `FourthVoid` 且非超时（答复 cancel）⇒ outcome = `refused`（同样是升级给人）
//	   **`awaiting`（还在等）与 `supplied`（上游补了规格）不落事件** —— 两者都不是升级，
//	   落了只会把低基数结局稀释掉（设计稿 §6.1 要的是可统计的升级流）。
//	③ **失败路径不落**：`SettleClarification` 的所有 `return …, err` 分支**一条事件都不落**
//	   （没有裁决就没有可观测的结局 —— 与「认不出一律报错」同向）。
//
// ── 本批边界 ──
//
//	· **不改任何判定**：只在既有成功/成升级路径的**原地**加一次 best-effort 写
//	  （`sliceobs.Emit` 无返回值 ⇒ 判定侧无从被改变，写失败只记日志）。
//	· 事件里**只落低基数值**（片 id / 原因码 / R 编号 / 挂单 id / 期限）——
//	  不回显请求体原文、凭据或私有路径（硬规则）。
//	· **本包依赖面变化（如实登记）**：`policy` 原为「叶子包：只 import 标准库」，
//	  本批起多了一条内部依赖 `core/internal/sliceobs`（它自身只依赖 `statepath` 与标准库，
//	  且**不反向依赖** policy ⇒ 无环）。理由：观测出口只有一套，而它必须能被 policy 触到。
package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/sliceobs"
)

// emitSliceEscalatedFromRaise — 进第 4 态（追问/阻滞）落一行 `slice_escalated`。
//
//	event = slice_escalated · outcome = raised · code/r = 原因码与 R 编号
//	criteria_version = 无来源 ⇒ 显式「未标定」（不编造）
//	detail = 低基数值拼出的可行动一句（退给谁 / 期限 / 为什么不得硬猜）
func emitSliceEscalatedFromRaise(o FourthStateOutcome, upstream string, deadline time.Time) {
	r := o.Code.R
	sliceobs.Emit(sliceobs.Event{
		Event:           sliceobs.EventSliceEscalated,
		OK:              false,
		Outcome:         sliceobs.OutcomeRaised,
		SliceID:         o.SliceID,
		Code:            o.Code.String(),
		R:               &r,
		CriteriaVersion: sliceobs.CriteriaVersionUncalibrated,
		Detail: fmt.Sprintf("片 %s 进第 4 态「追问/阻滞」（挂单 %s）：退回上游 %q 补规格"+
			"（原因码 %s = 判据 R%d，期限 %s）；进态**不放行任何东西** —— "+
			"片停在 awaiting，等不到答复按超时结算（fail closed，禁硬猜 = 禁假绿）。",
			o.SliceID, o.PendingID, strings.TrimSpace(upstream), o.Code.String(), r,
			deadline.UTC().Format(time.RFC3339)),
	})
}

// emitSliceEscalatedFromSettle — 第 4 态结算成**升级类**结局时落一行 `slice_escalated`。
//
// 只认 `escalate` / `void` 两类（`awaiting` / `supplied` 不是升级 ⇒ 不落，见文件头 ②）。
func emitSliceEscalatedFromSettle(o FourthStateOutcome) {
	if o.Disposition != FourthEscalate && o.Disposition != FourthVoid {
		return
	}
	outcome := sliceobs.OutcomeRefused
	why := "上游拒答/无规格可补 ⇒ **升级给人**（缺信息硬猜 = 假绿，故片不许带着缺规格继续）"
	if o.Disposition == FourthVoid {
		why = "本次交互作废（既有逐类语义）⇒ **不得当作已确认**：要么重新发起追问，要么升级给人"
		if o.TimedOut {
			outcome = sliceobs.OutcomeTimeout
			why = "**到点未答 ⇒ 超时**，按既有逐类语义（interaction ⇒ 取消）结算 ⇒ **升级给人**；" +
				"片**不得自行解除阻滞**（既不按缺规格往下跑，也不把沉默当批准）"
		}
	}
	r := o.Code.R
	sliceobs.Emit(sliceobs.Event{
		Event:           sliceobs.EventSliceEscalated,
		OK:              false,
		Outcome:         outcome,
		SliceID:         o.SliceID,
		Code:            o.Code.String(),
		R:               &r,
		CriteriaVersion: sliceobs.CriteriaVersionUncalibrated,
		Detail: fmt.Sprintf("片 %s 的第 4 态结算为 %s（挂单 %s，原因码 %s = 判据 R%d）：%s。",
			o.SliceID, strings.ToUpper(string(o.Disposition)), o.PendingID, o.Code.String(), r, why),
	})
}
