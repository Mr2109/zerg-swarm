package agent

// types.go — 虫族 v2.5 Agent：接口与工具定义（2026-08-13）
// 依赖：agentstate.HarnessState、agent.go 中的 ToolCall/Message/Terminator/ToolGater/Checker

import (
	"encoding/json"
	"fmt"
	"log"
)

// 接口

// ToolGater — M3 gate 接口（工具拦截三态：allow/block/require_approval）
type ToolGater interface {
	Check(toolName, args string, agent string) (Decision, error)
}

// Decision — gate 决策结果
type Decision struct {
	Action  string // allow / block / require_approval
	Message string // 决策说明
}

// Checker — 验证器接口（maker/checker 分离：验证工具结果是否达标）
type Checker interface {
	Pass(ctx interface{}, contract string) (bool, string)
}

// 工具定义

// ToolDef — 工具定义（name + description + input_schema，工具自带说明）
type ToolDef struct {
	Type     string      `json:"type"` // function
	Function FunctionDef `json:"function"`
}

// FunctionDef — 工具函数定义（name + description + parameters）
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"` // "怎么用"（含示例/边界）——模型靠它学会
	Parameters  map[string]any `json:"parameters"`  // JSON Schema
}

// ToolCallResult — 工具执行结果
type ToolCallResult struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"` // 工具名（无进展检测用——v2.5）
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
	Duration   string `json:"duration,omitempty"` // v2.5.4.9 工具执行耗时（CA 追踪）
}

// 模型响应解析

// ModelResponse — 模型响应（原 agent.go 定义——2026-09-05 随协议统一移入 types.go）
type ModelResponse struct {
	Content   string
	Reasoning string
	// ReasoningOnly 标记「只有思考、无正文」（不把思考搬进 Content——见事故注释）
	ReasoningOnly bool
	ToolCalls     []ToolCall
	Finish        string
	Usage         struct {
		TotalTokens int64
	}
}

// parseModelResponse — 解析模型响应（Chat API 格式——choices[].message）
func parseModelResponse(raw []byte) (*ModelResponse, error) {
	// Responses API 格式：output items（message/reasoning/function_call）
	var parsed struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			CallID    string `json:"call_id"`
		} `json:"output"`
		Usage struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	resp := &ModelResponse{}
	for _, item := range parsed.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				// v2.5.4.9 兼容两种文本类型：output_text（新）+ text（旧/其他实现）
				if c.Type == "output_text" || c.Type == "text" {
					resp.Content += c.Text
				}
			}
		case "reasoning":
			for _, c := range item.Content {
				if c.Type == "reasoning_text" {
					resp.Reasoning += c.Text
				}
			}
		case "function_call":
			args := map[string]any{}
			if err := json.Unmarshal([]byte(item.Arguments), &args); err != nil {
				log.Printf("failed to parse function_call arguments: %v (raw: %s)", err, item.Arguments)
			}
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:      item.CallID,
				Name:    item.Name,
				Args:    args,
				RawArgs: item.Arguments,
			})
		}
	}
	resp.Usage.TotalTokens = parsed.Usage.TotalTokens
	// ⚠ 铁律（2026-09-17 实测事故后立，Mr2109 定）：**思考不得顶替正文**。
	// 旧实现（「思考模型 fallback：content 空读 reasoning」）把思考搬进 Content ⇒ reasoning 列空 ⇒
	// 下一轮回灌时它成了「上一轮助手说的话」⇒ 模型照抄该口吻 ⇒ 正文污染 + 复读（实测链条见
	// Zerg-内部文档/项目文档/v2.5.10/变更-v2.5.10.md）。此处**只记录事实、不做搬运**：（2026-09-19 二次分家）
	// Content 空就是空（交付轮没有正文），思考一律留在 Reasoning。
	if resp.Content == "" && resp.Reasoning != "" {
		resp.ReasoningOnly = true
	}
	return resp, nil
}

// parseChatModelResponse — 解析 chat completions 响应（choices[].message——2026-09-05 协议统一）
// 容错: usage 缺失=0（llama-server 截断 bug 兼容——与 chat 包 parseChatResultTolerant 同思想）
func parseChatModelResponse(raw []byte) (*ModelResponse, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
		// 顶层 reasoning_content（streamReadChat 聚合产物——部分实现思考不在 message 内）
		ReasoningContent string `json:"reasoning_content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	resp := &ModelResponse{}
	if len(parsed.Choices) > 0 {
		m := parsed.Choices[0].Message
		resp.Content = m.Content
		resp.Reasoning = m.ReasoningContent
		for _, tc := range m.ToolCalls {
			args := map[string]any{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("failed to parse tool_call arguments: %v (raw: %s)", err, tc.Function.Arguments)
			}
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:      tc.ID,
				Name:    tc.Function.Name,
				Args:    args,
				RawArgs: tc.Function.Arguments,
			})
		}
		resp.Finish = parsed.Choices[0].FinishReason
	}
	if resp.Reasoning == "" && parsed.ReasoningContent != "" {
		resp.Reasoning = parsed.ReasoningContent
	}
	resp.Usage.TotalTokens = parsed.Usage.TotalTokens
	// ⚠ 铁律（2026-09-17 实测事故后立，Mr2109 定）：**思考不得顶替正文**。
	// 旧实现（「思考模型 fallback：content 空读 reasoning」）把思考搬进 Content ⇒ reasoning 列空 ⇒
	// 下一轮回灌时它成了「上一轮助手说的话」⇒ 模型照抄该口吻 ⇒ 正文污染 + 复读（实测链条见
	// Zerg-内部文档/项目文档/v2.5.10/变更-v2.5.10.md）。此处**只记录事实、不做搬运**：（2026-09-19 二次分家）
	// Content 空就是空（交付轮没有正文），思考一律留在 Reasoning。
	if resp.Content == "" && resp.Reasoning != "" {
		resp.ReasoningOnly = true
	}
	return resp, nil
}

// toolNames — 提取工具名列表（调试用）
func toolNames(tools []ToolDef) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Function.Name)
	}
	return names
}

// truncate — 输出截断（工具输出超过 limit **字节**截断 + 提示）
//
// ★ 口径：预算按**字节**算（上下文预算是字节账，不换成 rune 上限——那会把同一份预算
// 在中文上放大 3 倍）。但切口必须**退回 UTF-8 字符起始**：直接 s[:limit] 会把中文/emoji/
// 生僻字切在字符中间，产出非法 UTF-8（实测 limit=20 的中文串尾出 `这是一个很长\xe7\x9a`）。
// 退边界的手法与 internal/gateway/compactor.go 的 runeStartBackward 同源——那里记过
// 「不 clamp 就 s[:i] 越界 panic」的事故，故此处 limit 先夹到 [0, len(s)]。
func truncate(s string, limit int) string {
	if limit < 0 { // s[:负数] 亦属越界 panic，同一条口径一并夹住
		limit = 0
	}
	if len(s) <= limit {
		return s
	}
	cut := limit
	if cut > len(s) { // 防御性 clamp：与 compactor.go 同一事故口径（此处早退已挡住，留作重构护栏）
		cut = len(s)
	}
	for cut > 0 && s[cut]&0xC0 == 0x80 { // 0x80==续字节 ⇒ 正落在字符中间，退到起始
		cut--
	}
	return s[:cut] + fmt.Sprintf("\n...(output truncated — %d bytes total)", len(s))
}

// containsTool 已废弃（2026-08-13——已用导出版 ContainsTools——见 chat.go）
