package modeladapter

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// Generic 默认适配器：未匹配到专用适配器时的兜底行为。
// 标准 JSON 工具调用、无重提示、llama-server 默认参数（-ngl 999 GPU 全量）。
type Generic struct{}

func (a *Generic) Name() string { return "" }

func (a *Generic) BuildArgs(entry *registry.ModelEntry, port int) []string {
	args := []string{
		"-m", entry.File,
		"-c", "0", // 自动用模型训练上下文（qwen35moe 等 GQA 模型增量小）
		"-ngl", "999", // GPU 全量加载（不带 = 纯 CPU，慢 10 倍+）
		"-ctk", "q8_0", // KV cache K 量化（省显存，P0-2）
		"-ctv", "q8_0", // KV cache V 量化（省显存，P0-2）
		"-fa", "on", // Flash Attention（prompt 处理加速，P0-4）
		"--cache-prompt", // prompt 缓存（agent 循环免重复 prefill，P0-1）
		// v2.5.5 T6: 空闲槽保留 KV 缓存——同会话连续请求前缀复用（CA 轮次间 TTFT 大降）
		"--cache-idle-slots",
		"-np", "1", // v2.5.5 单槽铁律（设计决策——执行层面单槽——GPU全负荷）
		"-cb", // continuous batching（显式）
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}
	// D3 多模态：视觉投影（mmproj 字段非空时——llama-server -mm）
	if entry.MMProj != "" {
		args = append(args, "-mm", entry.MMProj)
	}
	return args
}

func (a *Generic) ToolCallStyle() string { return "json" }

func (a *Generic) NeedsToolReminder() bool { return false }

func (a *Generic) ReminderPrompt() string { return "" }

func (a *Generic) ThinkingFormat() string { return "none" }

func (a *Generic) ContextWindow() int { return 262144 }
