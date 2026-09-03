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

// parseModelResponse — 解析模型响应（Chat API 格式——choices[].message）
func parseModelResponse(raw []byte) (*ModelResponse, error) {
	// Responses API 格式：output items（message/reasoning/function_call）
	var parsed struct {
		Output []struct {
			Type      string `json:"type"`
			Content   []struct {
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
