// sendback_code.go — B 项④-C：**四态裁决的「打回」出口接上原因码**（一个打回 = 一个主原因码）。
//
// 要治的缺口（设计稿 v2.1 §4.7 / §4.10）：
//
//	receipt.go 的四态裁决里 `send_back` 是「无据 ⇒ 永久错误 ⇒ 不重试，改走拆小步」——
//	但**打回本身只带一段自由文本**（terminal.go 的 `SendBackNote(reason)`）。自由文本的后果：
//
//		· 归因不可判定：同一份产出，三个人能写出三种责任归属（模型不行 / 提示没写好 / 环境不对）；
//		· 不可聚合：打回率里分不出「片的错」与「执行者的错」⇒ §4.9 要求成对给的指标根本算不出来；
//		· 会自欺：格式问题（判据不全）被记成模型问题，于是**去改模型** ✗（§4.7 逐字点名这条）。
//
//	⇒ 本文件把出口收紧成：**打回必须带一个主原因码**（`PRE_`/`POST_`/`INV_` + R 编号 + 最小修复提示，
//	  类型与校验在 policy/sendback_reason.go）。缺码 ⇒ **拒绝发回**（不静默放行、也不拿默认值兜底）。
//
// ── 口径（写死在这里，由用例钉住）──
//
//	① **一个打回只能带一个主原因码**：`SendBackRequest.Code` 是**单值**（不是切片）⇒ 类型上就带不了两个。
//	   多个问题 ⇒ 多条打回（各自的码分开记），而不是一条打回塞两个责任（那样归因又变成主观的了）。
//	② **缺码 / 码不过 Validate ⇒ 报错并且不产出打回指令**：返回零值记录 + error（调用方拿不到任何
//	   可以误当"打回成功"的东西）。
//	③ **不替调用方猜码**：本文件**不**从裁决结果反推原因码（谁判的谁给码）。若在这里拿「默认档」兜底，
//	   就等于把 §4.7 明文禁止的「默认成某一类」写进代码 ✗。
//	④ **重复打回的口径**：§4.7「同原因码连续 2 次打回同一片 ⇒ 先复审判据」是**硬规则**，但它的**计数器**
//	   属后续批次（本批不落，登记不实现）；本文件只保证每一次打回都**带码**（计数器要的输入由此而来）。
//
// ── 与既有 `SendBackNote(reason string) string`（terminal.go）的关系：**不重叠、不改它** ──
//
//	那一条是 **`ZERG_SENDBACK` 实验开关**的产物（默认关）：轮数/预算用尽 ⇒ 产出「打回重做」指令，
//	用于 A/B 实测「打回是帮助还是伤害」。**它当时的现场没有判据事实**（只是"轮数用尽"），
//	给它配一个码就是**编一个责任归属** ⇒ 故本批**不改它、不动 run.go 一行**（零回归），
//	并把「实验路径仍是无码路径」登记为未接线项（见回报）。判据侧（谁判红、判的是哪条）走本文件的出口。
//
// ── 本批边界 ──
//
//	· 只新增：`VerifyClaims`（四态裁决的计算）**一行不动**、终局与挂单的行为一行不动。
//	· 不落盘、不打观测：本文件只产出**落账形态**（`SendBackRecord`，可序列化）与打回指令文本；
//	  观测出口/HTTP 面属接线批（与 api 侧的片 schema 同一分工）。
package loopcore

import (
	"fmt"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/policy"
)

// SendBackRequest — 一次打回的完整入参。**原因码是必填项**（`Code` 为单值 ⇒ 天然「一个打回一个主原因码」）。
type SendBackRequest struct {
	// SliceID — 打回的是哪一片（空 ⇒ 报错：没有 id 的委派一律拒绝，沿用铁律）。
	SliceID string
	// Code — ★ **主原因码**（责任前缀 + R 编号 + 最小修复提示）。必须过 `Validate`。
	Code policy.SendBackCode
	// Detail — 打回说明（自由文本，**不进任何判定**；只进给人看的 Note 与落账）。
	Detail string
	// Verdicts — 判定者给出的四态裁决结果（`VerifyClaims` 的产物）。
	// 用途只有一个：落账「本批裁决里判成 send_back 的有几条」（可观测），**不参与**选码。
	// 空 ⇒ 报错（连一份裁决都没有就发回 = 判不了 ⇒ 不打回）。
	Verdicts []ClaimVerdictResult
}

// SendBackRecord — 一次打回的**落账形态**（低基数 + 可序列化；观测/审计/UI 复用同一形状）。
type SendBackRecord struct {
	SliceID string `json:"slice_id,omitempty"`
	// Code — 主原因码的规范形态（如 `PRE_R3`）。**单个值**。
	Code string `json:"code"`
	// Responsibility / ResponsibilityLabel — 责任前缀与其中文标签（§4.7 的三分归因：可聚合）。
	Responsibility      string `json:"responsibility"`
	ResponsibilityLabel string `json:"responsibility_label"`
	// Judge — R 编号（设计稿 v2.1 §3.4 的机器判据闭集 R0–R13 里的那一个）。
	Judge int `json:"judge"`
	// Advice — 最小修复提示（§4.7：没有它就不是有效打回）。
	Advice string `json:"advice"`
	// Detail — 打回说明（自由文本）。
	Detail string `json:"detail,omitempty"`
	// SendBackCount / VerdictCount — 本批裁决里判成 send_back 的条数 / 裁决总条数。
	// 两者都要（只看前者会漏掉"总共没几条"这种同样重要的信息 —— §4.9 的成对口径）。
	SendBackCount int `json:"send_back_count"`
	VerdictCount  int `json:"verdict_count"`
}

// Note — 给**执行者**的打回指令（可行动：带原因码 + 最小修复提示 + 重交要求）。
//
// 返回 error 的情形（**fail-closed：宁可不吭声，也不发一条说不清原因的打回**）：
//
//	· 记录是零值 / 缺原因码（`Code` 为空）⇒ 「无原因码 ⇒ 本条不得发回」
//	· 原因码回解不过（认不出 / 缺 R 编号 / 越界 / 缺建议）⇒ 原样带上 policy 的专项错误
func (r SendBackRecord) Note() (string, error) {
	if strings.TrimSpace(r.Code) == "" {
		return "", fmt.Errorf("打回被拒：这条记录**没有原因码**（打回必须带 `PRE_`/`POST_`/`INV_` + R 编号 + 最小修复提示）" +
			"⇒ 不发回（也绝不拿默认责任兜底：默认 = 替人定了该改哪一层，且事后无从发现）")
	}
	code, err := policy.ParseSendBackCode(r.Code)
	if err != nil {
		return "", fmt.Errorf("打回被拒：原因码「%s」回解不过 ⇒ 不发回：%w", strings.TrimSpace(r.Code), err)
	}
	if advice := strings.TrimSpace(r.Advice); advice != "" {
		code.Advice = advice
	}
	s := "【打回重做】原因码 " + code.String() + "（" + code.Resp.Label() + "）：" + strings.TrimSpace(code.Advice)
	if strings.TrimSpace(r.Detail) != "" {
		s += "\n打回说明：" + strings.TrimSpace(r.Detail)
	}
	s += fmt.Sprintf("\n判据编号：R%d（取自设计稿 v2.1 §3.4 的机器判据 R0–R13）", code.R)
	if r.VerdictCount > 0 {
		s += fmt.Sprintf("；本批裁决 %d 条，其中判成 send_back（无据 ⇒ 永久错误、不重试）**%d 条**",
			r.VerdictCount, r.SendBackCount)
	}
	s += "\n请带**工具回执**重交：每条主张必须附「回执ID + 原文片段」；无回执的主张不算完成。"
	return s, nil
}

// SendBack — **打回的唯一出口**：校验（含原因码）⇒ 落账记录。缺件一律**报错、不产出记录**。
//
// 这是「不静默放行」的落点：本函数**没有**任何一条返回 (record, nil) 的路径允许缺原因码 ——
// 缺码时的返回值是零值记录 + error（调用方拿不到任何可当"打回已发出"的东西）。
func SendBack(req SendBackRequest) (SendBackRecord, error) {
	if strings.TrimSpace(req.SliceID) == "" {
		return SendBackRecord{}, fmt.Errorf("打回被拒：缺 slice_id ⇒ 没有 id 的打回回指不到片" +
			"（沿用铁律「无 id 的委派一律拒绝」）")
	}
	// ① 原因码必填且必须过 Validate（含「格式类打回必须附最小修复提示」）
	if err := req.Code.Validate(); err != nil {
		return SendBackRecord{}, fmt.Errorf("打回被拒（片 %s）：%w", strings.TrimSpace(req.SliceID), err)
	}
	// ② 没有裁决就没有"判红"这回事 ⇒ 不打回（不静默放行 = 也不静默发回）
	if len(req.Verdicts) == 0 {
		return SendBackRecord{}, fmt.Errorf("打回被拒（片 %s）：没有给出任何四态裁决 ⇒ 判不了「凭什么打回」"+
			"（打回必须由判据事实支撑，不是由感觉支撑）", strings.TrimSpace(req.SliceID))
	}
	sendBacks := 0
	for _, v := range req.Verdicts {
		if v.Verdict == VerdictSendBack {
			sendBacks++
		}
	}
	if sendBacks == 0 {
		return SendBackRecord{}, fmt.Errorf("打回被拒（片 %s）：给出的 %d 条裁决里**没有**一条判成 send_back"+
			"（keep/revise/retract 都不是打回：revise 是补引用、retract 是撤回该主张）⇒ 无可发回项",
			strings.TrimSpace(req.SliceID), len(req.Verdicts))
	}
	return SendBackRecord{
		SliceID:             strings.TrimSpace(req.SliceID),
		Code:                req.Code.String(),
		Responsibility:      string(req.Code.Resp),
		ResponsibilityLabel: req.Code.Resp.Label(),
		Judge:               req.Code.R,
		Advice:              strings.TrimSpace(req.Code.Advice),
		Detail:              strings.TrimSpace(req.Detail),
		SendBackCount:       sendBacks,
		VerdictCount:        len(req.Verdicts),
	}, nil
}
