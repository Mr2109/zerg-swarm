package agent

// context_window.go — v2.5.4.9 滚动窗口压缩（Mr2109思路——保留最近 N 轮 + 滚动压最早）
// 设计: docs/设计-滚动窗口压缩.md（2026-08-15 定稿——交叉调研验证——NiteAgent 5-8 轮）
// 核心: 最早轮压缩成摘要（摘要区）——保留最近轮完整
//
// GAP-20261018-02（2026-10-18）：触发判据从「只数轮」改成**双条件**
//
//	轮数 > ContextWindow  OR  估算 token > 上下文预算 × 阈值（默认 0.5）
//
// 病象（真机逐字）：子代理跑 10–11 轮就被网关拒发「输出预算：剩余输出额度不足
// （剩余≈306 < 512；ctx=32768）」，而 prompt≈31118/34594 —— **轮数没到窗口就一次都不压**，
// 于是一条条 3K 级的回执把 prompt 顶到 ctx 边上。旧判据只数 assistant 消息（本轮次），
// 全程不看 token ⇒ 「轮数少但每条很长」这一形态永远不触发。
//
// 保留轮数条件（不是替换）：轮数多是另一种形态（几十条短消息堆满窗口），拆掉它会漏一半。

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// summaryPrefix — 摘要区标记（history 开头——模型可参考早期上下文）
const summaryPrefix = "【历史摘要】"

// GAP-20261018-03（2026-10-18）：阈值扣掉**实际固定开销**（✗ 不硬编码 16K）
//
// 残留实测（上一笔 GAP-20261018-02 之后）：估算只数 history，**不含 system prompt 与工具定义**
// （真机固定开销约 16K）⇒ 阈值按整个 ctx 算 ⇒ 触发偏晚（一条条 3K 回执先把 prompt 顶到 ctx 边上）。
// 本笔把阈值改成「先扣固定开销，再乘比例」：
//
//	固定开销 = estimateTextTokens(sysPrompt) + estimateTextTokens(toolsJSON)
//	有效预算 = ctx − 固定开销（非负；触底回落 compactFixedOverheadFloor 并记 warning）
//	阈值     = 有效预算 × ratio
//
// 两个来源一律由**调用方**给：agent 层只能从 history 首条 system 拿 sysPrompt（工具定义在
// loopState 里，Agent 拿不到 ⇒ 只扣 sysPrompt）；chat 层由 chat_handlers 传 sysPrompt + 工具定义 JSON。
// 两个参数都为空 ⇒ 逐字回落旧口径（零回归）。
const (
	// compactFixedOverheadFloor — 有效预算下限（扣完固定开销若不足此值 ⇒ 回落到此值并记 warning）。
	// 为什么是 2048：再小的话「压到只剩最近几轮」也压不回阈值以下（一轮工具回执就 1–2K），
	// 那样判据会退化成「每轮都压、压了也没用」——留一个能装下最近一轮的底。
	compactFixedOverheadFloor = 2048
)

// 压缩触发参数（GAP-20261018-02）
const (
	// defaultCompactCtxTokens — 上下文预算默认值。
	// 与网关 `internal/gateway/outbudget.go:defaultCtxWindow = 32768` 同一个保守默认
	// （agent 层拿不到卵声明的 ctx：那是网关侧 fleet 档的事实；默认值取同一数字 ⇒ 两层不各写一套量纲）。
	defaultCompactCtxTokens = 32768
	// defaultCompactTriggerRatio — 触发阈值比例（默认 0.5）。
	// 与 chat 层 `CompactThresholdRatio = 0.5` 同量级 ⇒ 两层「什么时候开始压」口径一致。
	defaultCompactTriggerRatio = 0.5
)

// compactCtxBudgetTokens — 压缩判据用的上下文预算（token）。
//
// 真源优先级：`ZERG_AGENT_CTX_TOKENS`（正整数）> defaultCompactCtxTokens。
// ✗ 不拿 cfg.Budget 当本判据的预算：那是「整轮累计花费预算」（loop.go 的 token_budget 终止条件，
// 本地模型默认 0 = 不设限）—— 语义不同，拿 0 当 ctx 会让本判据静默全关。
func compactCtxBudgetTokens() int {
	if v := strings.TrimSpace(os.Getenv("ZERG_AGENT_CTX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultCompactCtxTokens
}

// compactTriggerRatio — 触发阈值比例。
//
// 真源优先级：`ZERG_AGENT_COMPACT_RATIO`（0 < r ≤ 1 的浮点）> defaultCompactTriggerRatio（0.5）。
// 取值不合法（非数字 / ≤0 / >1）⇒ 回落默认 0.5：宁可晚压一点，也不许把判据变成永真或永假。
func compactTriggerRatio() float64 {
	if v := strings.TrimSpace(os.Getenv("ZERG_AGENT_COMPACT_RATIO")); v != "" {
		if r, err := strconv.ParseFloat(v, 64); err == nil && r > 0 && r <= 1 {
			return r
		}
	}
	return defaultCompactTriggerRatio
}

// compactTokenThreshold — 本轮压缩的 token 触发线（= 预算 × 比例；✗ 不扣固定开销的老口径）。
func compactTokenThreshold() int {
	return int(float64(compactCtxBudgetTokens()) * compactTriggerRatio())
}

// compactBudget — 一次预算计算的四个读数（观测面用——ctx / 固定开销 / 有效预算 / 是否触底）。
type compactBudget struct {
	Ctx     int  // 上下文预算（token）
	Fixed   int  // 固定开销 = sysPrompt + 工具定义（token 估算）
	Budget  int  // 有效预算 = ctx − Fixed（非负；触底回落到 compactFixedOverheadFloor）
	Floored bool // 是否触底（Budget 被抬到下限）
}

// compactBudgetWithOverhead — **纯函数**：按固定开销扣减上下文预算。
//
//	Fixed  = estimateTextTokens(sysPrompt) + estimateTextTokens(toolsJSON)
//	Budget = max(ctx − Fixed, compactFixedOverheadFloor)   ← ✗ 永不为负
//
// 纯函数（只依赖入参）⇒ 单测直接钉公式，不受环境与模型影响；warning 由调用方记（见下方包装）。
func compactBudgetWithOverhead(ctx int, sysPrompt, toolsJSON string) compactBudget {
	b := compactBudget{Ctx: ctx}
	b.Fixed = estimateTextTokens(sysPrompt) + estimateTextTokens(toolsJSON)
	b.Budget = ctx - b.Fixed
	if b.Budget < compactFixedOverheadFloor {
		b.Budget = compactFixedOverheadFloor
		b.Floored = true
	}
	return b
}

// effectiveCompactBudget — **纯函数**：扣掉固定开销后的有效上下文预算（非负；触底回落下限）。
//
// 验收读法（ctx=32768 / ratio=0.5）：
//
//	固定开销 0      ⇒ 有效预算 32768 ⇒ 阈值 16384（与不扣开销的老口径逐字同值）
//	固定开销 16000  ⇒ 有效预算 16768 ⇒ 阈值 8384
//	固定开销 32000  ⇒ 有效预算 768 被抬到 2048（Floored=true，调用方记 warning）
func effectiveCompactBudget(ctx int, sysPrompt string, toolsJSON string) int {
	return compactBudgetWithOverhead(ctx, sysPrompt, toolsJSON).Budget
}

// compactTokenThresholdWithOverhead — 扣固定开销后的 token 触发线（= 有效预算 × 比例）。
//
// 兼容口径：两个参数**都为空** ⇒ 直接回落 compactTokenThreshold()，
// 即「调用方没给固定开销」时与旧口径**逐字一致**（零回归）。
// 触底（固定开销 ≥ ctx）⇒ 记一笔 warning：这时判据仍在工作（阈值 = 下限 × 比例），
// ✗ 不会算出负数/零（那样会变成「每轮都压、压不动」的永真判据）。
func compactTokenThresholdWithOverhead(sysPrompt, toolsJSON string) int {
	if sysPrompt == "" && toolsJSON == "" {
		return compactTokenThreshold()
	}
	b := compactBudgetWithOverhead(compactCtxBudgetTokens(), sysPrompt, toolsJSON)
	if b.Floored {
		log.Printf("⚠️ zerg agent ctx: fixed overhead %d ≥ ctx %d — effective compact budget floored to %d (compaction will be frequent)",
			b.Fixed, b.Ctx, b.Budget)
	}
	return int(float64(b.Budget) * compactTriggerRatio())
}

// estimateTextTokens — 纯函数：按**内容字节数**估算 token（bytes/3）。
//
// 为什么是 bytes/3，而不是 runes/2.5（两条都试算过）：
//   - runes/2.5 = 0.4 token/rune —— 对 CJK 严重低估：llama 系中文 ≈ 0.7 字/token（≈1.43 token/字），
//     差 3.5 倍 ⇒ 本仓这批提示（中文注释 + 中文工具结果占大头）会晚触发到压不动的地步；
//   - bytes/3 = 1 token/3 字节 —— UTF-8 下英文/代码 ≈ 4 字节/token（高估约 33%，宁早勿晚），
//     CJK 3 字节/字 ⇒ 1 token/字（比真值 ≈1.43 低估约 30%）。两向误差都在 1.5 倍以内，
//     且与网关 `outbudget.bytesPerToken = 4` 同族口径（同一量纲、不引 tokenizer 依赖）。
//
// 本函数是**纯函数**（只依赖入参）⇒ 单测直接钉公式，不受环境与模型影响。
func estimateTextTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 2) / 3 // 向上取整：1 字节也算 1 token，避免极小片段估成 0
}

// estimateMessageTokens — 纯函数：单条消息计入压缩判据的 token 估算。
//
// 计入面 = 正文 + 工具调用（名 + 原始参数 + 结构化参数）——
// 工具回执与本仓的「一次 read 十几 K」都在这两类里；不把 Reasoning 计入（Message 结构无此字段）。
func estimateMessageTokens(m Message) int {
	n := estimateTextTokens(m.Content)
	for _, tc := range m.ToolCalls {
		n += estimateTextTokens(tc.Name)
		n += estimateTextTokens(tc.RawArgs)
		n += estimateTextTokens(tc.RawCall)
		if len(tc.Args) > 0 {
			var sb strings.Builder
			for k, v := range tc.Args { // 只累长度，与 map 迭代顺序无关
				sb.WriteString(k)
				sb.WriteString(fmt.Sprintf("%v", v))
			}
			n += estimateTextTokens(sb.String())
		}
	}
	return n
}

// estimateHistoryTokens — 纯函数：整段历史的 token 估算（判据与观测共用同一把尺）。
func estimateHistoryTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateMessageTokens(m)
	}
	return total
}

// rollingSummary — 滚动摘要（增量——每压一轮追加）
type rollingSummary struct {
	text      string // 累积摘要
	compacted int    // 已压缩轮数
}

// countRounds — 统计"轮"（assistant 消息=轮开始——工具结果跟在后面算同一轮）。
func countRounds(msgs []Message) int {
	rounds := 0
	for _, m := range msgs {
		if m.Role == "assistant" {
			rounds++
		}
	}
	return rounds
}

// compactSysPromptIndex — history 里那条「固定 system prompt」的下标（-1 = 没有）。
//
// 为什么要单独认它：这条是**不可压的固定开销**（系统提示压不掉，每轮都得发），
// 本笔已从 ctx 里扣掉它 ⇒ 计数时必须把它排除，否则**两头都算**（预算扣一次、用量再算一次），
// 阈值会偏早约 fixed/2，把「还没满就压」变成常态。
//
// 认法：第一条 role=system 且不是摘要区（摘要区是【历史摘要】开头——它属于 history 侧、要计数）。
func (a *Agent) compactSysPromptIndex() int {
	for i, m := range a.history {
		if m.Role != "system" {
			continue
		}
		if strings.HasPrefix(m.Content, summaryPrefix) {
			continue // 摘要区不是固定开销（它随 history 一起被压/被算）
		}
		return i
	}
	return -1
}

// compactSysPrompt — 本 Agent 可见的 system prompt（拿不到 ⇒ 空串 ⇒ 阈值回落旧口径）。
//
// agent 层的唯一来源就是 history（Loop 的 systemPrompt() 每轮现算、不入 history；
// 子代理把提示塞进 history[0]——见 subagent.go）。
// ✗ 工具定义在 agent 这侧拿不到（在 loopState.tools 里）⇒ agent 层只扣 sysPrompt，chat 层两样都扣。
func (a *Agent) compactSysPrompt() string {
	if i := a.compactSysPromptIndex(); i >= 0 {
		return a.history[i].Content
	}
	return ""
}

// compactionTokens — 本次判据看的 token 量：history + 已累积摘要区（跳过固定 system prompt）。
//
// 摘要区算进来：它每轮都会随 prompt 一起发出去（buildSummaryBlock），
// 不计数会让「压了半天总量没降」这种情况在读数里看不见。
// 固定 system prompt 排除在外：它是扣过一次的固定开销（见 compactSysPromptIndex），不参与「可压量」。
func (a *Agent) compactionTokens() int {
	skip := a.compactSysPromptIndex()
	n := estimateHistoryTokens(a.history) // 无 system prompt ⇒ 整段照旧（与旧口径逐字一致）
	if skip >= 0 {
		n -= estimateMessageTokens(a.history[skip]) // 固定开销已从预算里扣过 ⇒ 用量侧不再算它
	}
	if a.rollingSummary.text != "" {
		n += estimateTextTokens(summaryPrefix) + estimateTextTokens(a.rollingSummary.text)
	}
	return n
}

// compactThresholdTokens — 本 Agent 现用的 token 触发线（扣掉可见的固定开销）。
//
// 有 system prompt（子代理形态）⇒ 阈值 = (ctx − sysPrompt) × ratio；
// 没有 ⇒ 与旧口径 compactTokenThreshold() 逐字一致。
func (a *Agent) compactThresholdTokens() int {
	return compactTokenThresholdWithOverhead(a.compactSysPrompt(), "")
}

// needCompress — 双条件判据（每压一轮前重判一次——压完 token 可能已回落）。
//
//	轮数 > ContextWindow          —— 旧条件（保留；几十条短消息堆满窗口是另一种形态）
//	估算 token > 有效预算 × 阈值  —— 新条件（GAP-20261018-02 起；GAP-20261018-03 起扣固定开销）
//
// 下限：至少留 1 轮（全压成摘要 = 对话没有最近上下文了，模型接不上话）。
func (a *Agent) needCompress() bool {
	rounds := countRounds(a.history)
	if rounds <= 1 {
		return false
	}
	if a.cfg.ContextWindow > 0 && rounds > a.cfg.ContextWindow {
		return true
	}
	return a.compactionTokens() > a.compactThresholdTokens()
}

// compressOldest — 滚动窗口压缩：满足双条件即压最早一轮，直到条件不再成立。
// 返回: 是否压缩了（有轮次被压缩）
func (a *Agent) compressOldest() bool {
	compressed := false
	for a.needCompress() {
		if !a.compressOneRound() {
			break
		}
		compressed = true
	}
	return compressed
}

// compressOneRound — 压最早一轮（assistant + 跟随的 tool 消息 → 摘要），返回是否真压了。
func (a *Agent) compressOneRound() bool {
	// 最早的轮起点 = 第一条 assistant
	firstRoundStart := -1
	for i, m := range a.history {
		if m.Role == "assistant" {
			firstRoundStart = i
			break
		}
	}
	if firstRoundStart < 0 {
		return false
	}
	// 最早轮结束 = 下个 assistant 前（或 history 尾）
	firstRoundEnd := len(a.history)
	for i := firstRoundStart + 1; i < len(a.history); i++ {
		if a.history[i].Role == "assistant" {
			firstRoundEnd = i
			break
		}
	}
	// 提取最早轮内容 → 摘要
	earliest := a.history[firstRoundStart:firstRoundEnd]
	summary := summarizeRound(earliest)
	// 摘要追加到滚动摘要区
	a.rollingSummary.text = a.rollingSummary.text + summary
	a.rollingSummary.compacted++
	// 从 history 移除最早轮（保留摘要——在 history 开头）
	a.history = append(a.history[:firstRoundStart], a.history[firstRoundEnd:]...)
	return true
}

// summarizeRound — 将一轮（assistant+tool）压成摘要
func summarizeRound(msgs []Message) string {
	var sb strings.Builder
	sb.WriteString("\n")
	for _, m := range msgs {
		switch m.Role {
		case "assistant":
			content := m.Content
			if len(content) > 300 {
				content = content[:300] + "…"
			}
			if content != "" {
				sb.WriteString(fmt.Sprintf("决策: %s\n", content))
			}
			for _, tc := range m.ToolCalls {
				sb.WriteString(fmt.Sprintf("调用工具: %s(%v)\n", tc.Name, truncateArgs(tc.Args)))
			}
		case "tool":
			res := m.Content
			if len(res) > 150 {
				res = res[:150] + "…"
			}
			sb.WriteString(fmt.Sprintf("工具结果: %s\n", res))
		case "user":
			c := m.Content
			if len(c) > 150 {
				c = c[:150] + "…"
			}
			sb.WriteString(fmt.Sprintf("用户: %s\n", c))
		}
	}
	return sb.String()
}

// truncateArgs — 工具参数截断（摘要用——不完整保留）
func truncateArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var sb strings.Builder
	for k, v := range args {
		s := fmt.Sprintf("%v", v)
		if len(s) > 60 {
			s = s[:60] + "…"
		}
		sb.WriteString(fmt.Sprintf("%s=%s ", k, s))
	}
	return strings.TrimSpace(sb.String())
}

// buildSummaryBlock — 构造摘要区消息（history 开头）
func (a *Agent) buildSummaryBlock() *Message {
	if a.rollingSummary.text == "" {
		return nil
	}
	text := summaryPrefix + "（已压缩 " + fmt.Sprintf("%d", a.rollingSummary.compacted) + " 轮）:\n" + a.rollingSummary.text
	return &Message{Role: "system", Content: text}
}

// maybeCompact — 每轮开始调用（loop 里）——滚动窗口压缩
func (a *Agent) maybeCompact() {
	start := time.Now()
	if a.compressOldest() {
		// 压缩了——日志（把两个判据的现读读数一起打出来：轮数与估算 token）
		// GAP-20261018-03：只加不删——补「ctx / 固定开销 / 有效预算 / 阈值」四个数（阈值下降要有证据）
		if a.logger != nil {
			b := compactBudgetWithOverhead(compactCtxBudgetTokens(), a.compactSysPrompt(), "")
			a.logger.LogEvent("context", "warn", "rolling_compress",
				"", fmt.Sprintf("滚动压缩: 窗口=%d, 轮数=%d, 估算 token=%d/%d（阈值）, 已压=%d 轮, 耗时=%s, ctx=%d, 固定开销=%d, 有效预算=%d, 阈值=%d（未扣开销=%d）",
					a.cfg.ContextWindow, countRounds(a.history), a.compactionTokens(),
					compactTokenThreshold(), a.rollingSummary.compacted,
					time.Since(start).Round(time.Millisecond),
					b.Ctx, b.Fixed, b.Budget, int(float64(b.Budget)*compactTriggerRatio()), compactTokenThreshold()),
				nil, "", "", "")
		}
	}
}
