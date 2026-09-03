package modeladapter

import (
	"fmt"

	"zerg/agent/internal/registry"
)

// Qwen36 Qwen3.6-35B-A3B 适配器。
// 社区实测（HF #22）：ornith 35B 长任务能力衰减，Qwen3.6-35B-A3B 是"stable, reliable workhorse"。
// 原生标准 JSON 工具调用（无需 XML 转换），免重提示。
type Qwen36 struct{}

func (a *Qwen36) Name() string { return "qwen3" } // v2.5.5: qwen3.5/3.6/3.8 全系用此适配器（GQA 优化）

func (a *Qwen36) BuildArgs(entry *registry.ModelEntry, port int) []string {
	return []string{
		"-m", entry.File,
		"-c", "0", // 自动用模型训练上下文（qwen 系 GQA）
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		// v2.5.5 T6: 空闲槽保留 KV 缓存——同会话连续请求前缀复用（CA 轮次间 TTFT 大降）
		"--cache-idle-slots",
		// v2.5.5 单槽铁律（Mr2109——X3/本机都单槽执行——GPU 全负荷——active>=1 即忙）
		"-np", "1",
		"-cb",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}
}

func (a *Qwen36) ToolCallStyle() string { return "json" }

func (a *Qwen36) NeedsToolReminder() bool { return true }

func (a *Qwen36) ReminderPrompt() string {
	return "Always call tools directly. Never describe what you plan to do — just call the tool immediately."
}

func (a *Qwen36) ThinkingFormat() string { return "deepseek" }

func (a *Qwen36) ContextWindow() int { return 262144 }
