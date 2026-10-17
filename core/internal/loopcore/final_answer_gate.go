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

// ForcedAnswerHint — 「停手交答案」那一轮注入的 user 消息（逐字固定，单测钉着它）。
// 与既有收尾提示（timeout / empty_args / bad_format）同一口吻：只许写事实与结论，不编造。
const ForcedAnswerHint = "（停手：本轮不要再调用工具。把**已经拿到的事实与结论**直接用中文写出来——" +
	"几行即可，每行说清「是哪一条 / 我跑了哪条命令 / 结果是什么 / 结论是什么」；" +
	"信息确实不足就直说没查到，绝不编造。✗ 不要写「下一步打算 / 让我再查 / 我先把…做起来」——那不是交付。）"

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
