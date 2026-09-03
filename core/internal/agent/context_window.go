package agent

// context_window.go — v2.5.4.9 滚动窗口压缩（Mr2109思路——保留最近 N 轮 + 滚动压最早）
// 设计: docs/设计-滚动窗口压缩.md（2026-08-15 定稿——交叉调研验证——NiteAgent 5-8 轮）
// 核心: history 轮数 > ContextWindow 时——最早轮压缩成摘要（摘要区）——保留最近 N 轮完整

import (
	"fmt"
	"strings"
	"time"
)

// summaryPrefix — 摘要区标记（history 开头——模型可参考早期上下文）
const summaryPrefix = "【历史摘要】"

// rollingSummary — 滚动摘要（增量——每压一轮追加）
type rollingSummary struct {
	text      string // 累积摘要
	compacted int    // 已压缩轮数
}

// compressOldest — 滚动窗口压缩：超过 ContextWindow 轮——压缩最早一轮
// 返回: 是否压缩了（有轮次被压缩）
func (a *Agent) compressOldest() bool {
	if a.cfg.ContextWindow <= 0 {
		return false
	}
	// 统计"轮"（assistant 消息=轮开始——工具结果跟在后面算同一轮）
	// 简化：数 assistant 消息（含 tool_calls 的）——超过窗口即压最早
	rounds := 0
	for _, m := range a.history {
		if m.Role == "assistant" {
			rounds++
		}
	}
	if rounds <= a.cfg.ContextWindow {
		return false
	}
	// 找最早轮的 assistant 消息位置——压缩它和它的后续（直到下个 assistant 前）
	// 即: 最早的 [assistant + 跟随的 tool 消息] 压成摘要
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
		// 压缩了——日志
		if a.logger != nil {
			a.logger.LogEvent("context", "warn", "rolling_compress",
				"", fmt.Sprintf("滚动压缩: 窗口=%d, 已压=%d 轮, 耗时=%s",
					a.cfg.ContextWindow, a.rollingSummary.compacted, time.Since(start).Round(time.Millisecond)),
				nil, "", "", "")
		}
	}
}
