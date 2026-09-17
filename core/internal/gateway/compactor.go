package gateway

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ============================================================
// 逐字精简（快路径）——引擎无关，纯文本规则，毫秒级
// 理念：Observation Masking（JetBrains Junie 研究）——工具输出
// 用占位符替换，保留工具调用可见，删掉冗长输出。
// ============================================================

// 工具输出保留策略：只保留首尾 N 行（中间省略）
const (
	toolOutputHeadLines = 60  // 工具输出保留开头行数
	toolOutputTailLines = 40  // 工具输出保留结尾行数
	toolOutputMaxLen    = 800 // 单个工具输出最大保留字符
)

// 填充回复模式（assistant 的"收到/好的"类废话）
var fillerRe = regexp.MustCompile(`(?i)^(\s*(好的|收到|明白了|了解|ok|好的，记下来了|已处理|没问题)[。!！.\s]*)$`)

// TrimNoise 精简会话消息（快路径，转发前调用）。
// 策略（保守 10-20% 精简，保真优先）：
//  1. 工具输出（role=tool）→ observation masking：只保留首尾，中间省略
//  2. assistant 纯填充回复（"收到"等）→ 删除
//  3. 保留用户消息原文（用户说的重要，不动）
//
// 返回 (精简后的 messages, 是否发生精简)。
func TrimNoise(messages []map[string]interface{}) ([]map[string]interface{}, bool) {
	trimmed := false
	out := make([]map[string]interface{}, 0, len(messages))

	for _, m := range messages {
		role, _ := m["role"].(string)
		content := msgContent(m)

		switch role {
		case "tool":
			// 工具输出：observation masking（保留首尾）
			newContent := maskToolOutput(content)
			if newContent != content {
				trimmed = true
			}
			nm := cloneMsg(m)
			nm["content"] = newContent
			out = append(out, nm)

		case "assistant":
			// 纯填充回复 → 删除；否则保留
			if fillerRe.MatchString(strings.TrimSpace(content)) && !hasToolCalls(m) {
				trimmed = true
				continue // 跳过这条填充回复
			}
			out = append(out, m)

		default:
			// user / system：原文保留
			out = append(out, m)
		}
	}
	return out, trimmed
}

// maskToolOutput 工具输出精简：只保留首尾 N 行，中间省略。
// 判断：行数 > 首尾+1 就精简（不管字符数）；单行超长按字符截断。
//
// ⚠ 字符窗口 = 前 toolOutputHeadLines*40 = 2400 字节 + 后 toolOutputTailLines*40 = 1600 字节
// ⇒ **两个窗口装得下（len ≤ 4000）就不裁**（标记比省下的字还长，裁了也白裁）。
// 事故（2026-09-17 23:45–00:03，实测）：旧写法只查 len(content) > toolOutputMaxLen(800) 就切
// content[:2400] / content[len-1600:] ⇒ **801…3999 字节的单行输出必然越界 panic**
// （日志原文 `http: panic serving 127.0.0.1:65272: runtime error: slice bounds out of range [:0] with length 1210`），
// 而 net/http 在 handler panic 后**只记日志、不写任何响应、直接关连接** ⇒ 客户端只收到
// `Post "http://127.0.0.1:8082/v1/chat/completions": EOF`（对话层分类落到 other_error 兜底）。
// 边界值必须与窗口对齐：guard 必须是 len > headN+tailN，且切口要落在 UTF-8 字符边界上
// （中文工具输出按字节切会把一个汉字切成半个 ⇒ 提示词里出现非法序列）。
func maskToolOutput(content string) string {
	lines := strings.Split(content, "\n")

	// 行数多 → 按行精简（首尾保留）
	if len(lines) > toolOutputHeadLines+toolOutputTailLines+1 {
		head := strings.Join(lines[:toolOutputHeadLines], "\n")
		tail := strings.Join(lines[len(lines)-toolOutputTailLines:], "\n")
		return head + "\n...[此处省略 " + itoa(len(lines)-toolOutputHeadLines-toolOutputTailLines) + " 行：为省上下文被裁掉——如需完整内容请用 read offset/limit 分段重读]...\n" + tail
	}

	// 行数少但单行超长 → 按字符截断中间
	if len(content) > toolOutputMaxLen {
		headN, tailN := toolOutputHeadLines*40, toolOutputTailLines*40
		if len(content) <= headN+tailN {
			return content // 装不下首尾两个窗口 ⇒ 不裁（不是"短输出"，是"裁不动"）
		}
		h := runeStartBackward(content, headN)             // 头部切口退回字符起始
		t := runeStartForward(content, len(content)-tailN) // 尾部切口前进到字符起始
		if t <= h {
			return content
		}
		return content[:h] + "\n...[省略 " + itoa(t-h) + " 字符]...\n" + content[t:]
	}

	return content // 短输出不精简
}

// runeStartBackward — 把切口 i 向前退到最近的 UTF-8 字符起始（≤ i）。
// 用于"取前缀"：传进来的是窗口右端，退回边界即可保证 content[:h] 是合法 UTF-8。
func runeStartBackward(s string, i int) int {
	if i > len(s) {
		i = len(s)
	}
	for i > 0 && s[i]&0xC0 == 0x80 { // 0x80==续字节 ⇒ 正落在字符中间
		i--
	}
	return i
}

// runeStartForward — 把切口 i 向后推到最近的 UTF-8 字符起始（≥ i）。
// 用于"取后缀"：传进来的是窗口左端，推到边界即可保证 content[t:] 是合法 UTF-8。
func runeStartForward(s string, i int) int {
	if i < 0 {
		i = 0
	}
	for i < len(s) && s[i]&0xC0 == 0x80 {
		i++
	}
	return i
}

// hasToolCalls 判断 assistant 消息是否带工具调用（带工具调用不能删）。
func hasToolCalls(m map[string]interface{}) bool {
	_, ok := m["tool_calls"]
	return ok
}

// msgContent 提取消息文本内容（兼容 string 和数组格式）。
func msgContent(m map[string]interface{}) string {
	switch c := m["content"].(type) {
	case string:
		return c
	case []interface{}:
		// OpenAI 多段格式（text/image）
		var sb strings.Builder
		for _, part := range c {
			if pm, ok := part.(map[string]interface{}); ok {
				if t, ok := pm["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	return ""
}

// cloneMsg 浅拷贝消息（避免修改原 map）。
func cloneMsg(m map[string]interface{}) map[string]interface{} {
	nm := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		nm[k] = v
	}
	return nm
}

// itoa 简易整数转字符串（避免 import strconv 冲突）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// ============================================================
// 结构化摘要（慢路径，50% 临界点）——引擎无关，调虫族模型
// 固定类别输出（借鉴 Claude Code 结构化摘要 7000-12000 字符思路）
// ============================================================

// BuildStructuredSummaryPrompt 构造结构化摘要 prompt。
// 输出固定类别：目标/进展/决策/关键事实/待办/文件引用。
func BuildStructuredSummaryPrompt(msgsJSON string) string {
	return `将以下对话压缩成结构化摘要，严格按以下类别输出，每类用「」包裹：

「目标」当前会话的主要目标
「进展」已完成的事
「决策」确定的关键决策
「关键事实」必须记住的精确信息（数字/人名/路径/参数，原样保留不要改写）
「待办」未完成的事项
「文件引用」提到的文件/路径（精确路径原样保留）

要求：
1. 精确值（数字、路径、人名、错误码）必须逐字保留，不得改写或省略
2. 只输出摘要本体，不要解释
3. 如果某类无内容，写"无"

待压缩对话：
` + msgsJSON
}

// ParseStructuredSummary 从模型输出提取结构化摘要（去重包裹，保留原文结构）。
// 兼容模型输出带/不带「」包裹。
func ParseStructuredSummary(raw string) string {
	if raw == "" {
		return ""
	}
	// 去掉可能的思考前缀（<think> 或 Thinking Process）
	if idx := strings.Index(raw, "「目标」"); idx >= 0 {
		raw = raw[idx:]
	}
	return strings.TrimSpace(raw)
}

// ensureMap 确保 messages 解析为 []map[string]interface{}（供 TrimNoise 使用）。
func ensureMap(messages []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(messages))
	for _, m := range messages {
		if mm, ok := m.(map[string]interface{}); ok {
			out = append(out, mm)
		}
	}
	return out
}

// trimRequestMessages 快路径精简请求 body 的 messages（工具输出 masking + 填充删除）。
// 返回是否发生精简。body 是 *[]byte，精简后回写。
func trimRequestMessages(body *[]byte) bool {
	if body == nil || len(*body) == 0 {
		return false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(*body, &obj); err != nil {
		return false
	}
	msgs, ok := obj["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return false
	}
	trimmed, changed := TrimNoise(ensureMap(msgs))
	if !changed {
		return false
	}
	// 回写精简后的 messages
	trimmedAny := make([]interface{}, 0, len(trimmed))
	for _, m := range trimmed {
		trimmedAny = append(trimmedAny, m)
	}
	obj["messages"] = trimmedAny
	newBody, err := json.Marshal(obj)
	if err != nil {
		return false
	}
	*body = newBody
	return true
}
