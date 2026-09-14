package modeladapter

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
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
	// ctx 透传（2026-09-14）：ds4-server 的上下文窗口由 --ctx 决定；
	// 实测依据：docs/01-设计/设计-ds4适配器接视觉-20260914.md
	if ctx := customInt(entry, "ctx"); ctx > 0 {
		args = append(args, "--ctx", fmt.Sprintf("%d", ctx))
	}
	// 视觉编码器（2026-09-14）：ds4 的 --vision 与 llama-server 的 mmproj 语义不同，故单列键。
	if vision, ok := entry.Custom["vision"].(string); ok && vision != "" {
		args = append(args, "--vision", vision)
	}
	return args
}

// customInt 读取 inline 字段里的整数（YAML 可能给 int/int64/float64/string，逐类型兜）。
func customInt(entry *registry.ModelEntry, key string) int {
	switch v := entry.Custom[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n := 0
		for _, r := range v {
			if r < '0' || r > '9' {
				return 0
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	return 0
}

func (a *DS4) ToolCallStyle() string { return "json" }

func (a *DS4) NeedsToolReminder() bool { return false }

func (a *DS4) ReminderPrompt() string { return "" }

func (a *DS4) ThinkingFormat() string { return "none" }

func (a *DS4) ContextWindow() int { return 524288 }
