package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// pluginModelAdapter 桥：把 plugin.Plugin（模型适配器）适配成 agent.ModelAdapter。
// v2.5.4.7——主控零模型假设——loop 通过此桥调插件。
type pluginModelAdapter struct {
	plugin plugin.Plugin
}

// NewPluginModelAdapter 创建桥（注入模型适配器插件）。
func NewPluginModelAdapter(p plugin.Plugin) ModelAdapter {
	return &pluginModelAdapter{plugin: p}
}

// Call 实现 ModelAdapter——构造 PluginInput——调插件 Execute——解析 PluginOutput。
func (a *pluginModelAdapter) Call(ctx context.Context, sysPrompt string, messages []Message, tools []ToolDef) (*ModelResponse, error) {
	// 构造请求上下文
	msgs := make([]map[string]any, 0, len(messages)+1)
	msgs = append(msgs, map[string]any{"role": "system", "content": sysPrompt})
	for _, m := range messages {
		if m.Role == "tool" {
			msgs = append(msgs, map[string]any{"role": "tool", "content": m.Content})
			continue
		}
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if len(m.ToolCalls) > 0 {
			tcs := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":       tc.ID,
					"type":     "function",
					"function": map[string]any{"name": tc.Name, "arguments": tc.RawArgs},
				})
			}
			msg["tool_calls"] = tcs
		}
		msgs = append(msgs, msg)
	}
	// 工具定义
	toolDefs := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		toolDefs = append(toolDefs, map[string]any{"name": t.Function.Name, "description": t.Function.Description})
	}

	input := plugin.PluginInput{
		Data: map[string]any{
			"messages": msgs,
			"tools":    toolDefs,
		},
		Context: map[string]any{"prompt": sysPrompt},
	}
	out, err := a.plugin.Execute(input)
	if err != nil {
		return nil, fmt.Errorf("plugin adapter 执行失败: %w", err)
	}
	// 解析 PluginOutput.Result —— 期望 map 含 content/tool_calls
	result, ok := out.Result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("plugin adapter 返回类型错误: %T", out.Result)
	}
	resp := &ModelResponse{}
	if content, ok := result["content"].(string); ok {
		resp.Content = content
	}
	if tcsRaw, ok := result["tool_calls"].([]map[string]any); ok {
		for _, tc := range tcsRaw {
			name, _ := tc["name"].(string)
			argsStr := ""
			if args, ok := tc["arguments"].(string); ok {
				argsStr = args
			}
			tcItem := ToolCall{
				ID:      fmt.Sprintf("call_%d", len(resp.ToolCalls)),
				Name:    name,
				RawArgs: argsStr,
			}
			// v2.5.5 T5 截断容错: args JSON 解析失败（reasoning 长——max_tokens 小→截断）→ 报错触发重试
			// 不静默忽略——否则模型返回工具调用意图但参数空——任务异常（死循环/假完成）
			if argsStr != "" {
				if err := json.Unmarshal([]byte(argsStr), &tcItem.Args); err != nil {
					return nil, fmt.Errorf("工具参数 JSON 解析失败（可能截断）: %v——原始: %s", err, truncateStr(argsStr, 300))
				}
			}
			resp.ToolCalls = append(resp.ToolCalls, tcItem)
		}
	}
	// v2.5.5 T5 截断容错: 模型声明了 tool_calls 但解析后为空（JSON 截断——格式不符）→ 报错重试
	if len(resp.ToolCalls) == 0 {
		if raw, ok := result["tool_calls"]; ok && raw != nil {
			if rawStr, isStr := raw.(string); isStr && rawStr != "" {
				return nil, fmt.Errorf("工具调用 JSON 格式不符（可能是字符串——截断）: %s", truncateStr(rawStr, 300))
			}
		}
	}
	return resp, nil
}

// truncateStr 截断字符串（错误信息用——防超长）
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
