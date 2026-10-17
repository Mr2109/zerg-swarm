// final_answer_gate.go — GAP-20261017-12（P0）：「最终轮不交答案」的判据与提示语
//
// 症状（实测 · 会话 chat_20261017_152630_13f8d2）：模型的取证方向**全对**
// （`zerg gap show` 取到 want → `zerg code find recentRequests` → `zerg code show family_agent.go:62`，
// 真读到零命中），但 `exit_kind=natural` 时的正文是「让我看源码上下文」这类**下一步打算**，
// 而不是任务要求的结论 ⇒ 用户拿到的是过程，不是交付物。
//
// 为什么不在提示词里一句话了事：提示词是**软约束**（同一段提示词下模型照样以打算收尾）；
// 而循环内核是**唯一能看见「本轮正文长什么样」的地方** ⇒ 判据只能落在这里，提示词只做先手引导。
//
// 判据只判**三件本层可知的事实**（不猜模型意图）：
//   - 正文为空 ⇒ 显然不是交付；
//   - 正文**开头**是「打算」口吻（让我/我打算/下一步…）⇒ 过程，不是结论；
//   - **不判长度**：实测既有 3 条用例的合法终答就是「续跑完成」「系统正常」这类极短句
//     ⇒ 把「短」当未交付会误伤正常收尾、还白多跑一轮（本闸第一版就是这么红掉的）。
//
// 只判「开头 80 符文」的原因：结论先写、末尾带一句「下一步建议」是**正常交付**，
// 全文搜关键词会把它误判成未完成、多跑一轮（那正是本闸要避免的副作用）。
package loopcore

import (
	"strings"
	"unicode/utf8"
)

// ForcedAnswerHintSourceMark — 注入消息的**来源标注**（GAP-20261018-06）。
//
// 为什么要这一句：这条提示是**内核注入**的，不是用户说的。旧文案以「（停手：「开头并以
// role=user 注入 ⇒ 实测模型把它读成**用户下达的命令**（同会话下一条任务里自述「用户第一条
// 消息即要求停手、不再调用工具」，整轮零命令零事实）。来源归属因此必须写在**正文第一句**，
// 且不得留「停手」这类会被读成用户命令的词。
const ForcedAnswerHintSourceMark = "【系统提示 · 不是用户的新要求】"

// ForcedAnswerHintRole — 注入消息的角色（保留 "user"）。
//
// 为什么**不**改成 "system"（本单评估过、按现读事实否决）：
//   - chat 侧每次请求都把系统提示钉在**首条** system（`core/internal/chat/chat_infer.go:114`
//     与 `:223`：msgsAll = [system(sysPrompt)] + msgs）；
//   - 本仓已有逐字结论「llama-server system 须唯一在开头」（`core/internal/chat/chat_message.go:246`
//     InsertSummaryMid 的注释），即上游对**中途再插一条 system** 没有兼容保证。
//
// ⇒ 中途插 system 属「换真源换风险」：可能被上游拒或被模板丢掉，那时这条提示**彻底消失**
// （比误导更坏）。故沿用 user 角色，靠正文首句声明来源 —— 判据一行未动。
const ForcedAnswerHintRole = "user"

// ForcedAnswerHint — 「只交答案」那一轮注入的消息（逐字固定，单测钉着它）。
// 与既有收尾提示（timeout / empty_args / bad_format）同一口吻：只许写事实与结论，不编造。
// 文案四条含义（不调工具 / 给结论 / 报白不编 / 禁「下一步打算」）一字不减，只换掉会误导的
// 表述与来源归属（GAP-20261018-06）。
const ForcedAnswerHint = "（" + ForcedAnswerHintSourceMark + "本轮请不要再调工具。" +
	"请把**已经拿到的事实与结论**直接用中文写出来——几行即可，" +
	"每行说清「是哪一条 / 我跑了哪条命令 / 结果是什么 / 结论是什么」；" +
	"信息确实不足就直说没查到，绝不编造。✗ 不要写「下一步打算 / 让我再查 / 我先把…做起来」——那不是交付。）"

// forcedAnswerHintMsg — 构造注入消息（角色与正文的唯一出处 ⇒ 单测只需钉这一处）。
func forcedAnswerHintMsg() map[string]any {
	return map[string]any{"role": ForcedAnswerHintRole, "content": ForcedAnswerHint}
}

// IsForcedAnswerHint — 一条消息是不是本闸注入的那条提示。
//
// 认法按「来源标注 + user 角色」，**不按整串逐字比**：历史里的那条可能来自旧版文案
// （旧版没有来源标注，但以「（停手：」开头）⇒ 两代都要认得出、都要能剥掉。
func IsForcedAnswerHint(m map[string]any) bool {
	role, _ := m["role"].(string)
	if role != ForcedAnswerHintRole {
		return false
	}
	c, _ := m["content"].(string)
	if strings.Contains(c, ForcedAnswerHintSourceMark) {
		return true
	}
	// 旧版（GAP-20261017-12 起至本单前）逐字前缀 —— 已落库/已进快照的历史仍要认。
	return strings.HasPrefix(c, "（停手：本轮不要再调用工具。")
}

// DropForcedAnswerHints — 从消息里剥掉强制答题提示，返回**新切片**与剥掉的条数。
//
// GAP-20261018-06（本单最核心的修复）：这条提示的生命周期只有**一发请求**——
// 模型看完它就得走，绝不许躺在历史里跨轮/跨任务生效（实测后果：一次 forced_answer
// 之后，同会话下一条任务模型自述「用户第一条消息即要求停手」，零命令零事实）。
// 三个调用位点（run.go）：① 进 Run 先剥历史遗留（含旧版快照/调用方复用的消息表）；
// ② 补轮那一发推理返回后立刻剥（本发已送达模型）；③ 写快照前再剥一道（收口全部返回路径）。
func DropForcedAnswerHints(msgs []map[string]any) ([]map[string]any, int) {
	dropped := 0
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if IsForcedAnswerHint(m) {
			dropped++
			continue
		}
		out = append(out, m)
	}
	if dropped == 0 {
		return msgs, 0
	}
	return out, dropped
}

// withoutForcedAnswerHints — 只要剥干净后的消息（写快照那一类「只需结果」的位点用）。
func withoutForcedAnswerHints(msgs []map[string]any) []map[string]any {
	out, _ := DropForcedAnswerHints(msgs)
	return out
}

const (
	// forcedAnswerHeadRunes — 只判开头这么多个符文（结论先写、末尾带建议属正常交付）
	forcedAnswerHeadRunes = 80
)

// forcedAnswerPlanMarkers — 「打算 / 将来时」标记（不区分大小写；只在前 N 符文里判）
var forcedAnswerPlanMarkers = []string{
	"让我", "我先", "我打算", "我将", "我要再", "待我", "我再查", "我再看看",
	"let me", "i'll", "i will", "i am going to",
}

// forcedAnswerStartMarkers — **只在正文开头**出现的「还没开始交付」的过渡句。
// （放开头判而不是全文判：结论里出现「下一步」是建议，开头出现「下一步」是没交答案。）
var forcedAnswerStartMarkers = []string{
	"下一步", "接下来我", "接下来", "然后我", "接着我", "下面我", "next, i",
}

// ── GAP-20261017-13（P0）：末轮带工具调用 ⇒ 补的那一轮「只许文本」 ─────────────────
//
// 与上面 GAP-20261017-12 的分工（两条闸治的是**不同形状**，所以提示语也不同）：
//   · GAP-12：本轮**零工具调用**、却只写了「打算」⇒ 判正文形态（NeedsForcedAnswer）；
//   · GAP-13：本轮**带工具调用**、且它已是最后一轮 ⇒ 工具执行完模型就没轮次了
//     （实测：名册没写上、结论没交出来）⇒ 按「还有没有轮次」判，与正文形态无关。
// 两条共用 run.go 里同一个 forcedOnce 事实位 ⇒ **相加最多补一轮**（不各算各的，否则可能多跑两轮）。

// ForcedTextOnlyHint — 末轮工具调用执行完之后补的那一轮注入的 user 消息（逐字固定，单测钉着它）。
const ForcedTextOnlyHint = "（工具已执行完毕，现将结果附上。不要再调工具，直接给结论。" +
	"格式：用中文写清「是哪一条 / 我跑了哪条命令 / 结果是什么 / 结论是什么」，几行即可；" +
	"信息确实不足就直说没查到，绝不编造。）"

// ForcedTextOnlyMark — 「补轮仍请求工具调用 ⇒ 不执行、不再补」的机器可读标记（写进终局说明，单测钉着它）。
const ForcedTextOnlyMark = "forced_text_only"

// ForcedTextOnlyNote — 上述情形的终局说明：人面说清发生了什么，末尾带机器可读标记。
func ForcedTextOnlyNote() string {
	return "（补轮仍请求工具调用 ⇒ 不再执行、也不再补轮，按已拿到的内容收尾 · " + ForcedTextOnlyMark + "）"
}

// forcedTextOnlyContent — 补轮仍请求工具时交付正文怎么取：优先补轮的正文，其次本轮正文，
// 最后给一句**如实**的说明（不编造结论）。判据只有「有没有正文」这一件本层可知的事实。
func forcedTextOnlyContent(closing, carry string) string {
	if strings.TrimSpace(closing) != "" {
		return closing
	}
	if strings.TrimSpace(carry) != "" {
		return carry
	}
	return "（末轮只发出了工具调用、没有给出结论；工具结果已附在上一轮，详见工具轨迹。）"
}

// NeedsForcedAnswer — 本轮正文是否**不算交付**（"", why 为空 = 算交付）。
// 只认两件事实：空正文 · 「打算」口吻（过程被当成结果写了）。判据不收长度 —— 见文件头的理由。
func NeedsForcedAnswer(content string) (bool, string) {
	s := strings.TrimSpace(content)
	if s == "" {
		return true, "empty"
	}
	head := s
	if utf8.RuneCountInString(head) > forcedAnswerHeadRunes {
		head = string([]rune(s)[:forcedAnswerHeadRunes])
	}
	low := strings.ToLower(head)
	for _, m := range forcedAnswerPlanMarkers {
		if strings.Contains(low, m) {
			return true, "plan"
		}
	}
	for _, m := range forcedAnswerStartMarkers {
		if strings.HasPrefix(low, m) {
			return true, "plan_head"
		}
	}
	return false, ""
}
