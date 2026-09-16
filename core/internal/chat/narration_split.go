// narration_split.go — 交付轮：把「以过程叙述开头的那一段」切出去（Mr2109 定的语义）
//
// 语义（Mr2109 2026-09-17 定）：一轮对话 = 思考 → 工具调用 → 思考 → 工具调用 → … → **最后输出正文**。
// 现象：该模型在本系统长提示 + 多轮上下文下，会把「过程叙述」写进 content 通道（取证见
// DumpInferResult 的三条实测：content 以「用户问的是…让我回顾一下…」开头）。
// 处置：交付轮只做**窄判据**切分 —— 仅当 content 的**第一段**（到首个空行为止）以过程叙述开头，
// 且其后还有正文时才切；否则原样返回（宁可不动，也不误伤正文）。
//
// 参考 Hermes（agent/stream_delivery.py:110-115）：它不做事后缠斗，而是**把"过程相"隐藏**
// （analysis 隐藏；commentary 留 reasoning 通道）——本函数即同一思路在"单通道模型"上的落点。
package chat

import "strings"

// narrationMarkers — 过程叙述的开头标记（中英双语；**只在首段**匹配 ⇒ 避免误伤正文中的正当提及）
var narrationMarkers = []string{
	"用户问", "用户让我", "用户要求", "用户似乎", "用户想", "用户要",
	"我需要", "让我",   // 裸「让我」覆盖「让我从…/让我把…」等实测形态
	"让我先", "让我总结", "让我回顾", "让我分析", "让我看看",
	"我已经读到", "我已经读完", "我读完了", "文件已经读出来", "我找到了",
	"现在我有", "从内容看", "从文件内容来看", "从文件内容看", "从代码来看", "从注释看", "从结构看",
	"The user", "Now I", "Let me", "I've now", "I have a clear", "First, let me",
}

// SplitLeadingNarration — 把「以过程叙述开头的首段」切出来。
// 返回（过程叙述, 正文）。不满足窄判据时返回（""，原样 content）。
func SplitLeadingNarration(content string) (string, string) {
	trimmed := strings.TrimLeft(content, " \t\r\n")
	if trimmed == "" {
		return "", content
	}
	// 首段边界：优先空行；没有空行时以第一个换行为界（实测该模型常用「过程句：\n1. …」形态）
	idx := strings.Index(trimmed, "\n\n")
	if idx < 0 {
		idx = strings.Index(trimmed, "\n")
	}
	first := trimmed
	rest := ""
	if idx >= 0 {
		first = trimmed[:idx]
		rest = strings.TrimLeft(trimmed[idx:], " \t\r\n")
	}
	// 窄判据 ①：首段必须以过程叙述标记开头
	hit := false
	for _, mk := range narrationMarkers {
		if strings.HasPrefix(first, mk) {
			hit = true
			break
		}
	}
	if !hit {
		return "", content
	}
	// 窄判据 ②：其后必须还有正文（否则切了就空了 ⇒ 宁可保留）
	if strings.TrimSpace(rest) == "" {
		return "", content
	}
	return strings.TrimSpace(first), rest
}
