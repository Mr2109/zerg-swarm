package modeladapter

import (
	"fmt"

	"zerg/agent/internal/registry"
)

// DS4 DeepSeek-V4-Flash 适配器（ds4-server 后端，不走 llama-server）。
// 特性：ds4-server 原生支持 /v1/responses（RESPPROTO），非思考模型（ThinkingFormat: none）。
type DS4 struct{}

func (a *DS4) Name() string { return "deepseek" }

func (a *DS4) BuildArgs(entry *registry.ModelEntry, port int) []string {
	args := []string{
		"--model", entry.File,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
	}
	// ssd 流式加载
	if ssd, ok := entry.Custom["ssd"].(bool); ok && ssd {
		args = append(args, "--ssd-streaming")
	}
	if cache, ok := entry.Custom["ssd_streaming_cache_experts"].(string); ok && cache != "" {
		args = append(args, "--ssd-streaming-cache-experts", cache)
	}
	return args
}

func (a *DS4) ToolCallStyle() string { return "json" }

func (a *DS4) NeedsToolReminder() bool { return false }

func (a *DS4) ReminderPrompt() string { return "" }

func (a *DS4) ThinkingFormat() string { return "none" }

func (a *DS4) ContextWindow() int { return 524288 }
