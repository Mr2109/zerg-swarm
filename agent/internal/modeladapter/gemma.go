package modeladapter

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// Gemma Gemma 4 系列适配器。
// 特性：非思考模型（ThinkingFormat: none）、标准 JSON 工具调用、免重提示。
type Gemma struct{}

func (a *Gemma) Name() string { return "gemma" }

func (a *Gemma) BuildArgs(entry *registry.ModelEntry, port int) []string {
	return []string{
		"-m", entry.File,
		"-c", "32768",
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		"-np", "1", // v2.5.5 单槽铁律（设计决策——执行层面单槽——GPU全负荷）
		"-cb",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}
}

func (a *Gemma) ToolCallStyle() string { return "json" }

func (a *Gemma) NeedsToolReminder() bool { return false }

func (a *Gemma) ReminderPrompt() string { return "" }

func (a *Gemma) ThinkingFormat() string { return "none" }

func (a *Gemma) ContextWindow() int { return 131072 } // GGUF 元数据实测 gemma4.context_length=131072
