// terminal.go — 第⑤步：四终局选择（设计稿 v1.2 第③步 / §一②）
//
// 依据（行业调研，hidekazu-konishi 生产指南）：
//
//	「生产级 agent 最常见的**结构性缺口**：大多数实现只有两种终局 —— 返回答案，或抛异常。
//	 让它可运维的是这四种：降级（degrade）／升级给人（escalate）／带补偿地中止（abort with compensation）／保存状态后续跑。」
//
// ⇒ 本文件把"预算/失败到了尽头"这件事**变成一个显式决定**，而不是"循环结束/报错" ✗。
//
// 判据（可判定，不靠感觉）：
//
//	· 已完成的步 > 0 且任务还剩"可独立完成的部分" ⇒ **degrade**（降级：把能做的做完，如实标注未覆盖部分）
//	· 已完成的步 > 0 且无法继续但有检查点价值      ⇒ **resume**（保存状态续跑）
//	· 已发生**不可逆副作用**（改了文件/建了仓库/发了消息）⇒ **abort_compensated**（带补偿中止：登记已发生的改动，可回滚）
//	· 其余（零进展 / 需要人的判断 / 死胡同）        ⇒ **escalate**（升级给人，带状态摘要与建议下一步）
package loopcore

// TerminalState — 四终局（对齐行业口径）
type TerminalState string

const (
	TerminalDegrade          TerminalState = "degrade"           // 降级：做完可完成的部分，如实标注未覆盖
	TerminalResume           TerminalState = "resume"            // 保存状态续跑：落检查点，下次继续
	TerminalAbortCompensated TerminalState = "abort_compensated" // 带补偿中止：登记不可逆改动，可回滚
	TerminalEscalate         TerminalState = "escalate"          // 升级给人：带摘要与建议
)

// TerminalInput — 决定四终局所需的事实（全部来自观测/回执，不猜）
type TerminalInput struct {
	CompletedSteps     int  // 已完成的步数（有回执的步）
	HasIrreversible    bool // 本轮是否已发生不可逆副作用（写文件/发布/删除等）
	HasRemainingWork   bool // 是否还有"可独立完成"的余下部分
	Checkpointable     bool // 当前状态是否可落检查点（有进展且有明确待办）
	NeedsHumanJudgment bool // 是否卡在"需要人的判断"（如决策/凭据/取舍）
}

// ChooseTerminal — 四终局选择（顺序即优先级；每条都能给出可行动理由）
func ChooseTerminal(in TerminalInput) (TerminalState, string) {
	switch {
	case in.HasIrreversible:
		// 已动过不可逆的东西 ⇒ 先补偿（登记改动、可回滚），再交人
		return TerminalAbortCompensated, "已发生不可逆副作用 ⇒ 带补偿中止：登记已改动项，确保可回滚"
	case in.CompletedSteps > 0 && in.HasRemainingWork:
		return TerminalDegrade, "已完成的步 > 0 且仍有可独立完成的部分 ⇒ 降级：做完能做的，如实标注未覆盖部分"
	case in.CompletedSteps > 0 && in.Checkpointable:
		return TerminalResume, "有进展且有明确待办 ⇒ 落检查点后保存状态续跑"
	case in.NeedsHumanJudgment:
		return TerminalEscalate, "卡在需要人的判断 ⇒ 升级给人：附状态摘要与建议下一步"
	default:
		return TerminalEscalate, "零进展或无可继续路径 ⇒ 升级给人（绝不停在半路不吭声）"
	}
}

// TerminalNote — 终局的**可行动**说明（写入会话/观测；不改任何安全语义）
func TerminalNote(st TerminalState, detail string) string {
	prefix := map[TerminalState]string{
		TerminalDegrade:          "【降级完成】",
		TerminalResume:           "【已保存，可续跑】",
		TerminalAbortCompensated: "【带补偿中止】",
		TerminalEscalate:         "【已升级给人】",
	}[st]
	if prefix == "" {
		prefix = "【已结束】"
	}
	if detail == "" {
		return prefix
	}
	return prefix + " " + detail
}
