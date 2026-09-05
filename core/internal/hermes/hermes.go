// Package hermes — Hermes 工具调用协议（2026-09-05 从 chat 包抽出——CA Hermes 化）
// 模式: 不带 tools 字段 → 系统提示注入 <tools> JSON schema → 模型输出 <tool_call>JSON</tool_call>
// 治: llama-server tools 字段 + 自动 grammar 与 Qwen 系模板冲突 → 畸形 arguments（实测 CA 两任务失败根因）
// 与对话系统 P4-46 同源——chat 包保留转发兼容
package hermes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ToolSchema — 工具定义（hermes 自有——调用方适配）
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

// ToolCall — 解析出的工具调用（hermes 自有）
type ToolCall struct {
	ID      string
	Name    string
	Args    map[string]any
	RawArgs string
	RawCall string
}

var reCall = regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)
var reFn = regexp.MustCompile(`<function=([^>\s]+)>`)
var reParam = regexp.MustCompile(`(?s)<parameter=([^>]+)>(.*?)</parameter>`)

// BuildToolPrompt — 构建 Hermes 工具提示（<tools> 包 JSON schema + <tool_call> 格式说明）
// progressiveHint: 渐进式常驻提示（可空——对话模式用）
func BuildToolPrompt(tools []ToolSchema, progressiveHint string) string {
	schemas := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schemas = append(schemas, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters": map[string]any{
					"type":       "object",
					"properties": t.Parameters,
					"required":   t.Required,
				},
			},
		})
	}
	schemaJSON, _ := json.Marshal(schemas)
	callSchema := `{"properties": {"arguments": {"title": "Arguments", "type": "object"}, "name": {"title": "Name", "type": "string"}}, "required": ["arguments", "name"], "title": "FunctionCall", "type": "object"}`

	var b strings.Builder
	b.WriteString("\n\n# 工具调用（Hermes Function Calling 标准）\n")
	b.WriteString("你是函数调用 AI。以下是可用工具（<tools> 内 JSON schema 定义）:\n")
	b.WriteString("<tools>\n" + string(schemaJSON) + "\n</tools>\n\n")
	b.WriteString("每次函数调用输出一个 JSON 对象（函数名+参数）——包在 <tool_call></tool_call> XML 标签内:\n")
	b.WriteString("{" + callSchema + "}\n")
	b.WriteString("格式（name 与 arguments 必填——arguments 内参数按工具 schema 必填——不能为空）:\n")
	b.WriteString("<tool_call>\n{\"name\": \"工具名\", \"arguments\": {\"参数名\": \"参数值\"}}\n</tool_call>\n\n")
	b.WriteString("工具结果会以 <tool_response> 标签回传。\n")
	if progressiveHint != "" {
		b.WriteString(progressiveHint)
	}
	b.WriteString("调用工具时不要解释——直接输出 <tool_call>。\n")
	b.WriteString("【工具帮助】不确定工具的参数/用法时——给该工具加 help:true（如 {\"help\":true}）——返回该工具详细文档（参数示例/变更记录）——看完再调用。不要反复搜索同一关键词——工具已列出就直接调用。\n")
	return b.String()
}

// ParseXMLToolCalls — 从模型输出解析 <tool_call> 工具调用（三种格式兼容:
// ①Hermes JSON ②多对象逗号分隔 ③旧纯 XML <function=><parameter=>）
func ParseXMLToolCalls(content string) []ToolCall {
	var out []ToolCall
	for _, m := range reCall.FindAllStringSubmatch(content, -1) {
		inner := strings.TrimSpace(m[1])
		var obj struct {
			Name      string         `json:"name"`
			Function  string         `json:"function"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(inner), &obj); err == nil && (obj.Name != "" || obj.Function != "") {
			name := obj.Name
			if name == "" {
				name = obj.Function
			}
			rawArgs, _ := json.Marshal(obj.Arguments)
			out = append(out, ToolCall{Name: name, Args: obj.Arguments, RawArgs: string(rawArgs), RawCall: m[0]})
			continue
		}
		// 格式2: 多对象
		objs := extractJSONObjects(inner)
		if len(objs) > 0 {
			parsedAny := false
			for _, rawObj := range objs {
				var o2 struct {
					Name      string         `json:"name"`
					Function  string         `json:"function"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.Unmarshal([]byte(rawObj), &o2); err != nil {
					continue
				}
				name := o2.Name
				if name == "" {
					name = o2.Function
				}
				if name == "" {
					continue
				}
				rawArgs, _ := json.Marshal(o2.Arguments)
				out = append(out, ToolCall{Name: name, Args: o2.Arguments, RawArgs: string(rawArgs), RawCall: m[0]})
				parsedAny = true
			}
			if parsedAny {
				continue
			}
		}
		// 格式3: 纯 XML
		fm := reFn.FindStringSubmatch(inner)
		if len(fm) < 2 {
			continue
		}
		name := strings.TrimSpace(fm[1])
		args := map[string]any{}
		for _, p := range reParam.FindAllStringSubmatch(inner, -1) {
			args[strings.TrimSpace(p[1])] = strings.TrimSpace(p[2])
		}
		if len(args) == 0 {
			continue
		}
		rawArgs, _ := json.Marshal(args)
		out = append(out, ToolCall{Name: name, Args: args, RawArgs: string(rawArgs), RawCall: m[0]})
	}
	return out
}

// StripXMLToolCalls — 剥离 <tool_call> 块（正文保留）
func StripXMLToolCalls(content string) string {
	return reCall.ReplaceAllString(content, "")
}

// WrapToolResponse — 工具结果回传格式（[成功·N字] 状态标注 + <tool_response> 包装）
func WrapToolResponse(name, content string) string {
	status := fmt.Sprintf("[成功·%d字]", len([]rune(content)))
	if content == "" {
		status = "[成功·空]"
	}
	return wrapResp(name, content, status)
}

// WrapToolResponseErr — 失败标注
func WrapToolResponseErr(name, content string) string {
	return wrapResp(name, content, "[失败]")
}

func wrapResp(name, content, status string) string {
	b, _ := json.Marshal(content)
	return status + fmt.Sprintf("<tool_response>\n{\"name\": \"%s\", \"content\": %s}\n</tool_response>", name, string(b))
}

// extractJSONObjects — 括号平衡提取 JSON 对象
func extractJSONObjects(s string) []string {
	var out []string
	depth := 0
	start := -1
	var inStr bool
	var esc bool
	for i, r := range s {
		if esc {
			esc = false
			continue
		}
		switch r {
		case 92: // backslash
			if inStr {
				esc = true
			}
		case 34: // quote
			if depth > 0 {
				inStr = !inStr
			}
		case 123: // {
			if !inStr {
				if depth == 0 {
					start = i
				}
				depth++
			}
		case 125: // }
			if !inStr && depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					out = append(out, s[start:i+1])
					start = -1
				}
			}
		}
	}
	return out
}
