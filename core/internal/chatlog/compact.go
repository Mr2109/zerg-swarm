package chatlog

import (
	"fmt"
)

// ─────────────────────────────────────────────────────────────────────────────
// 对话压缩（波4 · 压缩片 · 逻辑不接命令面）
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §二.7 B1/B2/B3/B5/B6/B9/B10/B13
// 调研拄点：scratch `2.5.15-细节调研-对话面.md` §2「上下文与压缩」
//
// 四步、成本递增（★ **先确定性、后 LLM**）：
//
//	① 阈值**按现读（可用）窗口比例** —— 可配，默认取 50–80% 区间的**下沿**（0.50）。
//	   ★ ✗ 写死百分比、✗ 用「原始窗口」（同一条 reserve 在不同窗口上的百分比是**结果**不是**设置**）。
//	② **先确定性清理**（无 LLM）：清**陈旧工具结果** / 旧**思考块**，替为占位文本
//	   （触发阈值可配，默认 30–40K 区间的**下沿** 30000；`keep` 最近 N 个；★ 思考块清理排第一）。
//	③ **后才 LLM 摘要**：只对**旧轮**（✗ 绝不碰近轮逐字）· 摘要器**可注入**（✗ 不硬绑某个模型）。
//	④ 近轮逐字保护：按窗口比例（可配）＋首 N 条＋尾 N 条**取并集**；尾里**至少 1 条真实用户消息**。
//
// ★ 确定性：**同输入 ⇒ 同输出**（本文件 ✗ 无时间、✗ 无随机、✗ 无 map 迭代 ⇒ 无不可复现源）。
// 摘要器由调用方注入；其确定性由调用方负责（本面只保证装配与切点确定）。
// ─────────────────────────────────────────────────────────────────────────────

// 默认参数（可配 · 逐条带出处口径）。
const (
	// DefaultWindowTokens —— 缺省可用窗口（现读窗口）。调用方应传真实现读窗口；✗ 用原始窗口。
	DefaultWindowTokens = 8192
	// DefaultTriggerRatio —— 压缩触发比例：按窗口取 50–80% 的**下沿**（0.50 · B1 保守口径）。
	DefaultTriggerRatio = 0.50
	// DefaultRecentRatio —— 近轮逐字保护比例：15–30% 区间的取值（0.20 · B3）。
	DefaultRecentRatio = 0.20
	// DefaultProtectHead —— 永远不动的最前 N 条（首 3 · B3）。
	DefaultProtectHead = 3
	// DefaultProtectTail —— 永远不动的最后 N 条（尾 20 · B3）。
	DefaultProtectTail = 20
	// DefaultPruneTokenThreshold —— 确定性清理触发阈值：30–40K 的**下沿**（30000 · B9）。
	DefaultPruneTokenThreshold = 30000
	// DefaultPruneKeepRecent —— 每种陈旧块保留最近 N 个（B9/B10）。
	DefaultPruneKeepRecent = 3
)

// 标记与占位文本（均为常量 ⇒ 确定性 · ✗ 不引时间/随机）。
const (
	// SummaryMarker —— 摘要段落的可辨标记（一眼看出「此段已摘要」· B3）。
	SummaryMarker = "【已摘要】"
	// PrunedToolResultPlaceholder —— 陈旧工具结果替换文本。
	PrunedToolResultPlaceholder = "[已清理:陈旧工具结果]"
	// PrunedThinkingPlaceholder —— 旧思考块替换文本。
	PrunedThinkingPlaceholder = "[已清理:旧思考块]"
)

// 消息类别（压缩面靠它识别陈旧块）。
const (
	KindMessage    = "message"
	KindToolResult = "tool_result"
	KindThinking   = "thinking"
	KindSummary    = "summary"
)

// 压缩动作分档（CompactResult.Action 的闭集）。
const (
	ActionNone         = "none"              // 未触发（未超阈值）
	ActionPruned       = "pruned"            // 只做了确定性清理
	ActionSummarized   = "summarized"        // 只做了 LLM 摘要
	ActionPrunedAndSum = "pruned+summarized" // 先清后摘
)

// Summarizer —— **可注入**的摘要器（✗ 不硬绑某个模型 · B6「摘要用独立引擎」）。
//
// 输入 = 待摘要的**旧轮**（近轮已剔除，✗ 不会传进来）；输出 = 摘要正文。
// 返回错误 ⇒ 压缩**中止 LLM 摘要并保原文**（B7）；确定性清理那一档已做的不回滚。
type Summarizer interface {
	Summarize(old []Message) (string, error)
}

// SummarizerFunc —— 把普通函数当 Summarizer（便于注入）。
type SummarizerFunc func(old []Message) (string, error)

// Summarize 实现 Summarizer。
func (f SummarizerFunc) Summarize(old []Message) (string, error) { return f(old) }

// CompactOptions —— 压缩可配面（逐条对应设计稿 B 组）。
type CompactOptions struct {
	WindowTokens        int        // 现读（可用）窗口；<=0 ⇒ DefaultWindowTokens
	TriggerRatio        float64    // 触发比例；<=0 ⇒ DefaultTriggerRatio
	TriggerMinTokens    int        // 小窗口抬底：阈值 = max(窗口×比例, 本值)；<=0 ⇒ 不抬底
	RecentRatio         float64    // 近轮保护比例；<=0 ⇒ DefaultRecentRatio
	ProtectHead         int        // 首 N 条（0 ⇒ 默认；<0 ⇒ 关闭）
	ProtectTail         int        // 尾 N 条（0 ⇒ 默认；<0 ⇒ 关闭）
	PruneTokenThreshold int        // 确定性清理阈值；<=0 ⇒ DefaultPruneTokenThreshold
	PruneKeepRecent     int        // 保留最近 N 个；<=0 ⇒ DefaultPruneKeepRecent
	Summarizer          Summarizer // 可注入；nil ⇒ 只做确定性清理（✗ 不默默调模型）
}

// CompactResult —— 一次压缩的结果与读数（全部可现算 · ✗ 无时间戳）。
type CompactResult struct {
	Prompt            Prompt // 压缩后的提示（未触发时原样返回）
	Action            string // ActionNone / ActionPruned / ActionSummarized / ActionPrunedAndSum
	RemovedCount      int    // 确定性清理替换掉的陈旧块数
	SummarizedCount   int    // 被摘要（折叠进摘要段）的旧轮条数
	TotalBeforeTokens int    // 压缩前 messages 总 token（估算）
	TotalAfterTokens  int    // 压缩后 messages 总 token（估算）
	Reason            string // 人可读说明（✗ 空也不影响确定性）
}

// normalized —— 填默认值（0 ⇒ 默认；ProtectHead/Tail 负数 ⇒ 关闭）。
func (o CompactOptions) normalized() CompactOptions {
	if o.WindowTokens <= 0 {
		o.WindowTokens = DefaultWindowTokens
	}
	if o.TriggerRatio <= 0 {
		o.TriggerRatio = DefaultTriggerRatio
	}
	if o.RecentRatio <= 0 {
		o.RecentRatio = DefaultRecentRatio
	}
	if o.ProtectHead == 0 {
		o.ProtectHead = DefaultProtectHead
	}
	if o.ProtectTail == 0 {
		o.ProtectTail = DefaultProtectTail
	}
	if o.ProtectHead < 0 {
		o.ProtectHead = 0
	}
	if o.ProtectTail < 0 {
		o.ProtectTail = 0
	}
	if o.PruneTokenThreshold <= 0 {
		o.PruneTokenThreshold = DefaultPruneTokenThreshold
	}
	if o.PruneKeepRecent <= 0 {
		o.PruneKeepRecent = DefaultPruneKeepRecent
	}
	return o
}

// EstimateTokens —— 确定性 token 估算（保守取向 · ✗ 无模型调用 · ✗ 无时间/随机）。
//
// 口径：ASCII 按 4 字符 / token（上取整），非 ASCII（CJK 等）按 1 字符 / token。
// ★ 估算保守、可复现；真值应以 provider 回报校准（B18）。
func EstimateTokens(s string) int {
	ascii, wide := 0, 0
	for _, r := range s {
		if r < 0x80 {
			ascii++
		} else {
			wide++
		}
	}
	return wide + (ascii+3)/4
}

// messagesTokens —— 一组消息的估算总 token。
func messagesTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Content)
	}
	return total
}

// triggerTokens —— 触发阈值 = max(窗口×比例, 抬底)（B1/B2）。
func triggerTokens(o CompactOptions) int {
	t := int(float64(o.WindowTokens) * o.TriggerRatio)
	if o.TriggerMinTokens > t {
		t = o.TriggerMinTokens
	}
	if t <= 0 {
		t = 1
	}
	return t
}

// pruneStale —— 确定性清理（无 LLM · B9/B10）：把**陈旧**工具结果 / 思考块替为占位文本。
//
// ★ 思考块清理**排第一**（B9 顺序）；每种块保留**最近 PruneKeepRecent 个**（近的 ✗ 不动）。
// 就地改 msgs（调用方已持有副本）；返回替换掉的块数。
func pruneStale(msgs []Message, keepRecent int) int {
	replaced := 0
	replaced += pruneKind(msgs, KindThinking, PrunedThinkingPlaceholder, keepRecent)
	replaced += pruneKind(msgs, KindToolResult, PrunedToolResultPlaceholder, keepRecent)
	return replaced
}

// pruneKind —— 某一类陈旧块的清理（保留最近 keepRecent 个 · 其余替为 placeholder）。
func pruneKind(msgs []Message, kind, placeholder string, keepRecent int) int {
	// 从尾往前数：最近的 keepRecent 个保留；更旧的（若还不是占位文本）替换为 placeholder。
	kept := 0
	replaced := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind != kind {
			continue
		}
		if kept < keepRecent {
			kept++
			continue
		}
		if msgs[i].Content != placeholder { // 幂等：已是占位文本则不重复计数
			msgs[i].Content = placeholder
			replaced++
		}
	}
	return replaced
}

// protectedStart —— 尾部保护区的起始下标（[start, n) 逐字不动 · 不变量①的落地）。
//
// 保护 = 并集：① 角落首 N 条 ② 角尾 N 条 ③ 按窗口比例护最近 RecentRatio ④ 尾里至少 1 条真实用户消息。
// 返回 (start, ok)：ok=false ⇒ 无可摘要的旧轮（旧轮区间 [ProtectHead, start) 为空）。
func protectedStart(msgs []Message, o CompactOptions) (int, bool) {
	n := len(msgs)
	if n == 0 {
		return 0, false
	}
	// ② 尾 N 条（按条数）。
	start := n - o.ProtectTail
	// ③ 按窗口比例护最近 RecentRatio（按 token 从尾往前累加）。
	if o.RecentRatio > 0 {
		budget := int(float64(o.WindowTokens) * o.RecentRatio)
		if budget > 0 {
			acc := 0
			ratioStart := n
			for i := n - 1; i >= 0; i-- {
				acc += EstimateTokens(msgs[i].Content)
				ratioStart = i
				if acc >= budget {
					break
				}
			}
			if ratioStart < start {
				start = ratioStart
			}
		}
	}
	// ① 首 N 条必护（⇒ 保护区不得侵入首 N 条；也是「旧轮区间」的左界）。
	if start < o.ProtectHead {
		start = o.ProtectHead
	}
	if start > n {
		start = n
	}
	// ④ 尾里至少 1 条真实用户消息（防大纲式摘要吞掉诉求 · B5）。
	if !hasRealUser(msgs[start:]) {
		for i := start - 1; i >= o.ProtectHead; i-- {
			if msgs[i].Role == "user" && msgs[i].Kind != KindSummary {
				start = i
				break
			}
		}
	}
	if start < o.ProtectHead {
		start = o.ProtectHead
	}
	if start > n {
		start = n
	}
	return start, start > o.ProtectHead
}

// hasRealUser —— 这一段里是否有真实用户消息（✗ 摘要段不算）。
func hasRealUser(msgs []Message) bool {
	for _, m := range msgs {
		if m.Role == "user" && m.Kind != KindSummary {
			return true
		}
	}
	return false
}

// Compact —— 对一份提示做对话压缩（**先确定性清理后才 LLM 摘要** · 近轮逐字保护）。
//
// ★ 不动稳定层 / 上下文层（只改 Messages）⇒ 稳定前缀 sha256 不变（不变量②）。
// ★ 同输入 ⇒ 同输出（不变量④）。
func Compact(p Prompt, opt CompactOptions) (CompactResult, error) {
	o := opt.normalized()

	msgs := append([]Message(nil), p.Messages...)
	before := messagesTokens(msgs)
	res := CompactResult{
		Prompt:            p,
		Action:            ActionNone,
		TotalBeforeTokens: before,
		TotalAfterTokens:  before,
	}
	trigger := triggerTokens(o)

	// ② 先确定性清理（无 LLM）—— 触发阈值独立于窗口比例。
	pruned := 0
	if before >= o.PruneTokenThreshold {
		pruned = pruneStale(msgs, o.PruneKeepRecent)
	}
	mid := messagesTokens(msgs)

	// ③ 后才 LLM 摘要 —— 只对旧轮，且只在仍超触发阈值时。
	summarized := 0
	if mid >= trigger {
		start, ok := protectedStart(msgs, o)
		if ok {
			old := msgs[o.ProtectHead:start]
			if o.Summarizer == nil {
				res.Reason = "未注入摘要器：只做确定性清理（✗ 不硬绑某个模型）"
			} else {
				sum, err := o.Summarizer.Summarize(old)
				if err != nil {
					// B7：摘要失败**默认中止压缩并保原文**（旧轮原样保留）。
					res.Reason = fmt.Sprintf("摘要失败：中止压缩并保原文（%v）", err)
				} else {
					repl := Message{Role: "system", Kind: KindSummary, Content: SummaryMarker + sum}
					next := make([]Message, 0, o.ProtectHead+1+(len(msgs)-start))
					next = append(next, msgs[:o.ProtectHead]...)
					next = append(next, repl)
					next = append(next, msgs[start:]...)
					msgs = next
					summarized = len(old)
				}
			}
		}
	}

	// 组装动作标签。
	switch {
	case pruned > 0 && summarized > 0:
		res.Action = ActionPrunedAndSum
	case pruned > 0:
		res.Action = ActionPruned
	case summarized > 0:
		res.Action = ActionSummarized
	default:
		res.Action = ActionNone
	}

	res.Prompt = p
	res.Prompt.Messages = msgs
	res.RemovedCount = pruned
	res.SummarizedCount = summarized
	res.TotalAfterTokens = messagesTokens(msgs)
	// ★ 稳定层 / 上下文层一字不动（不变量②）：只替换了 Messages。
	return res, nil
}
