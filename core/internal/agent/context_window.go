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
	"os"
	"strconv"
	"strings"
	"time"
)

// summaryPrefix — 摘要区标记（history 开头——模型可参考早期上下文）
const summaryPrefix = "【历史摘要】"

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

// compactTokenThreshold — 本轮压缩的 token 触发线（= 预算 × 比例）。
func compactTokenThreshold() int {
	return int(float64(compactCtxBudgetTokens()) * compactTriggerRatio())
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

// compactionTokens — 本次判据看的 token 量：history + 已累积摘要区。
//
// 摘要区算进来：它每轮都会随 prompt 一起发出去（buildSummaryBlock），
// 不计数会让「压了半天总量没降」这种情况在读数里看不见。
func (a *Agent) compactionTokens() int {
	n := estimateHistoryTokens(a.history)
	if a.rollingSummary.text != "" {
		n += estimateTextTokens(summaryPrefix) + estimateTextTokens(a.rollingSummary.text)
	}
	return n
}

// needCompress — 双条件判据（每压一轮前重判一次——压完 token 可能已回落）。
//
//	轮数 > ContextWindow          —— 旧条件（保留；几十条短消息堆满窗口是另一种形态）
//	估算 token > 预算 × 阈值      —— 新条件（GAP-20261018-02：轮数没到窗口也可能是满的）
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
	return a.compactionTokens() > compactTokenThreshold()
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
		if a.logger != nil {
			a.logger.LogEvent("context", "warn", "rolling_compress",
				"", fmt.Sprintf("滚动压缩: 窗口=%d, 轮数=%d, 估算 token=%d/%d（阈值）, 已压=%d 轮, 耗时=%s",
					a.cfg.ContextWindow, countRounds(a.history), a.compactionTokens(),
					compactTokenThreshold(), a.rollingSummary.compacted,
					time.Since(start).Round(time.Millisecond)),
				nil, "", "", "")
		}
	}
}
