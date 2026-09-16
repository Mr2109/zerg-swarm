// reasoning_split.go — 思考与正文分离（Mr2109 定：**思考不能关，只能分开**）
//
// 事故背景（2026-09-17 实测）：
//
//	回灌历史时 assistant 消息只带 `content`（把思考也塞在里面），从不带 `reasoning_content`。
//	Qwen3 官方模板对 assistant 段的逻辑是：若 `reasoning_content` 不是字符串 ⇒ 去找 `</think>` 标签剥离思考。
//	我们两条都没满足（不传字段 ✗、无标签 ✗）⇒ **模板只能把整段当正文** ⇒ 下一轮模型照抄该口吻 ⇒ 正文污染、复读。
//	证据：实际请求体转储里历史 assistant 的键恒为 ['content','role']（27/27 条）。
//
// 借鉴 Hermes（`ui-tui/src/lib/reasoning.ts`）两条纪律：
//
//	① 成对标签 `<tag>…</tag>` ⇒ 抽出思考、从正文移除；
//	② **未闭合标签只从"消息开头"锚定** —— 正文中间出现的字面 `<think>`（模型引用该词、代码块含标签）
//	   不得吃掉其后段落；真实的未闭合思考块总是位于消息开头（推理模型就是这么流的）。
package chat

import "strings"

// reasoningTags — 与 Hermes 保持一致的标签表（少即是多：只认业界在用的这几个）
var reasoningTags = []string{"think", "reasoning", "thinking", "thought", "REASONING_SCRATCHPAD"}

// splitReasoning — 把一段文本切成（思考, 正文）。
// 先按成对标签切，再按"开头的未闭合标签"切；都没有则思考为空、正文原样返回。
func splitReasoning(content string) (string, string) {
	text := content
	var parts []string
	for _, tag := range reasoningTags {
		open, close := "<"+tag+">", "</"+tag+">"
		for {
			i := strings.Index(text, open)
			if i < 0 {
				break
			}
			j := strings.Index(text[i:], close)
			if j < 0 {
				break // 未闭合交给下面"只认开头"那条规则处理
			}
			inner := strings.TrimSpace(text[i+len(open) : i+j])
			if inner != "" {
				parts = append(parts, inner)
			}
			text = text[:i] + text[i+j+len(close):]
		}
		// 未闭合：**只从开头锚定**（防误伤正文中段的字面标签）
		t := strings.TrimLeft(text, " \t\r\n")
		if strings.HasPrefix(t, open) {
			inner := strings.TrimSpace(t[len(open):])
			if inner != "" {
				parts = append(parts, inner)
			}
			text = ""
		}
	}
	return strings.Join(parts, "\n\n"), strings.TrimSpace(text)
}

// SplitReasoningForHistory — 组装**历史**时的分离入口：
//   - 已存有独立 reasoning 字段 ⇒ 直接采用（源头分开，最好的一条路）
//   - 正文里带标签 ⇒ 按标签切分（兜底，覆盖"思考内联在正文里"的模型）
//   - 都没有 ⇒ 正文原样（不臆造思考 ✗）
//
// 返回（reasoning_content 值, 干净的 content 值）。
func SplitReasoningForHistory(content, storedReasoning string) (string, string) {
	tagReasoning, clean := splitReasoning(content)
	// 正文里切出了标签思考 ⇒ 以标签为准（它就长在正文里）
	if tagReasoning != "" {
		if storedReasoning != "" && !strings.Contains(storedReasoning, tagReasoning) {
			return storedReasoning + "\n\n" + tagReasoning, clean
		}
		return tagReasoning, clean
	}
	// 正文干净：若库里另有思考 ⇒ 思考归思考、正文归正文
	if strings.TrimSpace(storedReasoning) != "" {
		// ⚠ 兼容历史脏数据：早期版本可能把思考**同时**写进 content（正文以思考开头）。
		// 此时以"库里的 thinking 字段"为准，并把正文里重复的那段前缀去掉。
		if r := strings.TrimSpace(storedReasoning); strings.HasPrefix(clean, r) {
			clean = strings.TrimSpace(clean[len(r):])
		}
		return strings.TrimSpace(storedReasoning), clean
	}
	return "", clean
}
