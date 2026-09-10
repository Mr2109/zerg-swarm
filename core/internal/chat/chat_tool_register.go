package chat

// chat_tool_register.go — v2.5.7 P4-49 统一工具库（对话工具注册到 agent——CA/对话通用）
// 设计: 对话 126 个 deferred 工具执行器注册进 agent 扩展注册表——agent.ExecuteTool 路由执行
// 原则: 一套注册中心——CA/对话都能调——只是入口不同（Mr2109 2026-09-02）

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// RegisterChatExtraTools — 注册对话 deferred 工具到 agent 统一工具库（core 启动时调用一次）
// 每个工具: name/desc（ChatExtraToolDefs 提取）+ 执行器（ExecuteChatTool 包装）
func RegisterChatExtraTools() {
	defs := ChatExtraToolDefs()
	for name, def := range defs {
		desc := ""
		if fn, ok := def["function"].(map[string]any); ok {
			if d, ok2 := fn["description"].(string); ok2 {
				desc = d
			}
		}
		nm := name // 闭包捕获
		agent.RegisterExtraTool(agent.ExtraTool{
			Name: nm,
			Desc: desc,
			Fn: func(args map[string]any, workDir string) (string, error) {
				res := ExecuteChatTool(nm, args, workDir)
				if res.Error != "" {
					return "", fmt.Errorf("%s", res.Error)
				}
				return res.Content, nil
			},
		})
	}
}
