package api

// chat_degenerate_guard.go —— 长任务末段退化标注（GAP-20260926-214）。
//
// 病（实测）：本地卵干反假绿审计那笔真干了 53 分钟（120 行轨迹、已拷 90 只门到 scratch），
// 最后一步 think 退化成字面「pong! 🏓」⇒ 产物 TSV 未落，而调用方看到的是「跑完了」。
//
// 为什么在服务端标而不是在 CLI：只有服务端手里同时有「最终答复」与「本轮跑了多少次工具」，
// CLI 侧拿不到 per-session 的工具计数（chat_obs.jsonl 的行里没有会话号）。
//
// 判据刻意保守：**跑过 ≥3 次工具，且最终答复短于 16 个字符** 才标。标记是**只标不拦** ——
// 与「未取证」同一条纪律：不改结果、不影响退码，只把「这可能是退化」写进正文，让调用方复核。

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// degenerateToolCalls — 触发标注所需的最少工具调用次数（低于它说明任务本来就短）。
const degenerateToolCalls = 3

// degenerateMaxRunes — 触发标注的答复长度上限（长得像结论的一律不标）。
const degenerateMaxRunes = 16

// annotateDegenerateAnswer —— 有工具足迹却只吐了一句短话 ⇒ 在正文尾追加退化标记。
//
// 返回新正文；不满足条件时**原样返回**（逐字不改，避免污染既有金标）。
func annotateDegenerateAnswer(content string, toolCalls int) string {
	trimmed := strings.TrimSpace(content)
	if toolCalls < degenerateToolCalls || trimmed == "" {
		return content
	}
	if utf8.RuneCountInString(trimmed) >= degenerateMaxRunes {
		return content
	}
	snippet := trimmed
	if utf8.RuneCountInString(snippet) > 16 {
		r := []rune(snippet)
		snippet = string(r[:16]) + "…"
	}
	return content + fmt.Sprintf(
		"\n\n⚠ **疑似退化输出**：本轮跑过 %d 次工具，最终答复却只有 %d 个字符（%q）。"+
			"长任务末段常这样退化成一句无关短句，**产物很可能没落盘** —— 请以盘上件 / 缺口账 / 提交记录为准，✗ 别把这一轮当交付（缺口 GAP-20260926-214）。\n",
		toolCalls, utf8.RuneCountInString(trimmed), snippet)
}
