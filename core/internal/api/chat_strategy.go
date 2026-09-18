// chat_strategy.go — T3.6 提示脚手架策略：**对话路径的调用点声明**（H7）
//
// 设计依据（逐字）：docs/01-设计/设计-内建调试版-v1.2.md 第〇节
//
//	「H7 **缺"策略本身"的版本化**——我们观察到的"大目标⇒只读不交差；小目标⇒真动手"属**提示脚手架差异**｜★★★｜
//	  把策略当 prompt 的 config 一起版本化：`strategy_id / step_index / 有无验收标准 / 是否要求先出 tool_call`」。
//
// 为什么策略在**这里**声明（而不是由观测面从提示文本猜）：策略是**装配处**的事实——只有本文件所在层
// 知道"这次请求跑的循环是什么配置"。观测面（internal/chat）不许从提示文本反推策略：
// 文本里没有机器可读的策略声明，靠形态猜出来的字段**看着有、实际不可信**（I1「探针本身要自证」）⇒ 宁可缺席。
//
// 事实依据（写死；改这里先核对，别顺手美化）：
//   - 循环配置：与 chat_handlers.go 里 `loopcore.Config{...}` 字面量**同一处来源**
//     （chat.MaxToolRounds / chat.WallClockFor / chat.RoundTimeoutFor）——两处漂了就是两套口径；
//   - no_terminator：对话路径**不挂**终止仲裁（loopcore.Deps.Terminator 未设）⇒ 无线内契约核验；
//   - hermes_xml_tools：本路径不带请求体 tools 字段（Deps.Tools 未设）⇒ 工具以 XML 形式进系统提示；
//   - 因此本路径的两个布尔量都是**显式 false（声明了"没有"）**，而不是"没声明"（nil）：
//     ① has_acceptance_criteria=false —— 没有验收判定（无 Terminator/契约）；对话路径的终答不按契约核验；
//     ② requires_tool_call_first=false —— 不要求先出工具调用（loopcore/run.go：模型无工具调用 ⇒ natural 终止）。
//     两者都随消息存疑时**宁可不声明**（传 nil）——本文件的选择是"我们知道，就如实写 false"。
package api

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

// chatScaffoldSpec — 对话路径的提示脚手架**配置事实**（strategy_id 的输入；规范化取指纹见 chat.StrategyIDOf）。
// 口径：**集合语义**（Features/Params 与书写顺序无关）⇒ 同一份配置两次请求必得同一 id，改一个参数 id 必变。
func chatScaffoldSpec(model string) chat.PromptScaffoldSpec {
	return chat.PromptScaffoldSpec{
		Strategy: "chat.loop",
		Features: []string{"no_terminator", "hermes_xml_tools"},
		Params: []string{
			fmt.Sprintf("max_rounds=%d", chat.MaxToolRounds),
			fmt.Sprintf("wall_clock=%s", chat.WallClockFor(model)),
			fmt.Sprintf("round_timeout=%s", chat.RoundTimeoutFor(model)),
		},
	}
}

// chatPromptStrategy — 本次请求的策略声明（H7 四个字段里除 step_index 外的三个 + step_index 交观测面数）。
//
// step_index 留 0 ⇒ 由观测面按**本会话的提示装配计数**填（"本会话第几步"，chat.PromptStepIndex）——
// 轮次号是"一次请求内"的序号，会话级步序才是 H7 要的东西（"给一步一验的小目标"是按会话推进的）。
func chatPromptStrategy(model string) *chat.PromptStrategyIn {
	return &chat.PromptStrategyIn{
		StrategyID:            chat.StrategyIDOf(chatScaffoldSpec(model)),
		HasAcceptanceCriteria: chat.PromptStrategyFlag(false), // 上面「事实依据①②」：显式声明"没有"
		RequiresToolCallFirst: chat.PromptStrategyFlag(false),
	}
}

// chatPromptName — 本次提示的**装配处名字**（H4 的 prompt_name）。对话路径只有一处三档装配
// （chat.BuildTieredSystemPrompt，见 chat_prompt.go）⇒ 名字恒定；日后多一处装配就给各自的名字，事件里即可分账。
func chatPromptName() string { return chat.PromptNameChatTieredSystem }
