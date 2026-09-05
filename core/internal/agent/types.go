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
	ToolCalls []ToolCall
	Finish    string
	Usage     struct {
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
		return nil, fmt.Errorf("解析响应失败: %w", err)
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
				log.Printf("解析 function_call arguments 失败: %v (raw: %s)", err, item.Arguments)
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
	// 思考模型 fallback（content 空读 reasoning）
	if resp.Content == "" && resp.Reasoning != "" {
		resp.Content = resp.Reasoning
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
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	resp := &ModelResponse{}
	if len(parsed.Choices) > 0 {
		m := parsed.Choices[0].Message
		resp.Content = m.Content
		resp.Reasoning = m.ReasoningContent
		for _, tc := range m.ToolCalls {
			args := map[string]any{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("解析 tool_call arguments 失败: %v (raw: %s)", err, tc.Function.Arguments)
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
	// 思考模型 fallback（content 空读 reasoning）
	if resp.Content == "" && resp.Reasoning != "" {
		resp.Content = resp.Reasoning
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

// truncate — 输出截断（工具输出 > 2000 字符截断 + 提示）
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("\n...（输出过长已截断——共 %d 字符）", len(s))
}

// containsTool 已废弃（2026-08-13——已用导出版 ContainsTools——见 chat.go）
