package gateway

import (
	"encoding/json"
	"strings"
)

// ActionType 动作类型：识别客户端请求的语义意图，用于动作级路由。
// 阶段 B MVP：只做识别+映射，完整融合留阶段 D。
type ActionType int

const (
	ActionDefault  ActionType = iota // 默认动作（无明确信号，走默认路由）
	ActionCoding                     // 编码动作（refactor/fix/test/commit/code）
	ActionWriting                    // 写作动作（write/draft/章/文/报告）
	ActionResearch                   // 研究动作（search/research/分析/调研/文献）
	ActionToolCall                   // 工具调用动作（body 含 tools 字段 = agent 任务）
)

// String 动作类型中文名（日志用）。
func (a ActionType) String() string {
	switch a {
	case ActionCoding:
		return "编码"
	case ActionWriting:
		return "写作"
	case ActionResearch:
		return "研究"
	case ActionToolCall:
		return "工具调用"
	default:
		return "默认"
	}
}

// codingKeywords 编码相关关键词（英文 + 中文）。
var codingKeywords = []string{
	"refactor", "fix", "test", "commit", "code", "bug", "debug",
	"lint", "build", "compile", "deploy", "ci", "cd",
	"重构", "修复", "测试", "提交", "代码", "调试",
	"编译", "部署", "构建", "代码审查",
}

// writingKeywords 写作相关关键词。
var writingKeywords = []string{
	"write", "draft", "article", "essay", "blog", "post", "content",
	"报告", "文章", "文档", "说明", "指南", "教程", "总结", "汇报",
	"文案", "写作", "创作", "撰写", "大纲", "摘要",
}

// researchKeywords 研究相关关键词。
var researchKeywords = []string{
	"search", "research", "analyze", "investigate", "explore",
	"分析", "调研", "文献", "综述", "对比", "评估", "方案", "选型",
	"搜索", "研究", "探索", "发现",
}

// detectAction 从请求体中识别动作类型。
//
// 信号优先级（从高到低）：
//  1. path=/v1/responses → 工具调用（agent 任务）
//  2. body.tools 字段存在 → 工具调用
//  3. 消息内容匹配编码关键词 → 编码
//  4. 消息内容匹配写作关键词 → 写作
//  5. 消息内容匹配研究关键词 → 研究
//  6. 其余 → 默认
//
// 设计说明：
//   - 不依赖客户端明确标注动作类型
//   - 纯本地信号提取，零开销（<1ms）
//   - 默认路由兜底，避免误判
func detectAction(body []byte, path string) ActionType {
	// 信号 1：路径识别（OpenAI Responses 端点 = agent 任务）
	if path == "/v1/responses" {
		return ActionToolCall
	}
	// v2.5.4.9 路径识别扩展（测试规范——path 优先于 body 关键词）
	switch {
	case strings.Contains(path, "/research"):
		return ActionResearch
	case strings.Contains(path, "/write") || strings.Contains(path, "/writing"):
		return ActionWriting
	case strings.Contains(path, "/code") || strings.Contains(path, "/coding"):
		return ActionCoding
	}

	// 信号 2：body.tools 字段 = 工具调用
	if hasToolsField(body) {
		return ActionToolCall
	}

	// 信号 3~5：关键词匹配
	text := extractAllText(body)
	if text != "" {
		if matchKeywords(text, codingKeywords) {
			return ActionCoding
		}
		if matchKeywords(text, writingKeywords) {
			return ActionWriting
		}
		if matchKeywords(text, researchKeywords) {
			return ActionResearch
		}
	}

	return ActionDefault
}

// hasToolsField 检查请求体是否包含 tools 字段（简单字符串搜索）。
// MVP 阶段用字符串匹配代替 JSON 解析，更快。
func hasToolsField(body []byte) bool {
	idx := strings.Index(string(body), `"tools"`)
	if idx < 0 {
		return false
	}
	rest := body[idx+len(`"tools"`):]
	for _, ch := range rest {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			continue
		}
		return ch == ':' || ch == '[' || ch == '{'
	}
	return false
}

// extractAllText 从请求体中提取所有消息的文本内容（用于关键词匹配）。
// 兼容 OpenAI Messages（messages[].content）、Responses（input[]）、Claude。
func extractAllText(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 方法 1：JSON 解析提取 content/text 字段
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		// v2.5.4.9 malformed JSON 兜底：原始字符串匹配（测试规范——坏 JSON 也识别关键词）
		return string(body)
	}

	var sb strings.Builder
	// v2.5.4.9 顺序修正：system prompt 先（测试规范——system 是上下文——优先）
	if sys, ok := obj["system"]; ok {
		sb.WriteString(extractContentString(sys))
		sb.WriteString(" ")
	}
	// messages[] 或 input[] 数组
	for _, key := range []string{"messages", "input"} {
		if arr, ok := obj[key].([]interface{}); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					if content, ok := m["content"]; ok {
						sb.WriteString(extractContentString(content))
						sb.WriteString(" ")
					}
					if text, ok := m["text"]; ok {
						sb.WriteString(extractContentString(text))
						sb.WriteString(" ")
					}
				}
			}
		}
	}

	// 直接 content 字段
	if c, ok := obj["content"]; ok {
		sb.WriteString(extractContentString(c))
		sb.WriteString(" ")
	}

	// user 消息
	if u, ok := obj["user"]; ok {
		sb.WriteString(extractContentString(u))
		sb.WriteString(" ")
	}

	return sb.String()
}

// extractContentString 从 interface{} 中提取字符串内容（兼容 string/array 格式）。
// content 可能是 string、或 [{"type":"text","text":"..."}] 数组。
func extractContentString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		var sb strings.Builder
		for _, item := range val {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"]; ok {
					sb.WriteString(extractContentString(text))
					sb.WriteString(" ")
				}
			}
		}
		return sb.String()
	case map[string]interface{}:
		if text, ok := val["text"]; ok {
			return extractContentString(text)
		}
	}
	return ""
}

// matchKeywords 检查文本是否包含任一关键词（不区分大小写）。
// 关键词匹配：子串匹配（"fix" 匹配 "fix bug"）。
func matchKeywords(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
