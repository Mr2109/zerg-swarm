package modeladapter

import (
	"fmt"
	"os"

	"zerg/agent/internal/registry"
)

// Ornith example-35b 适配器（基于 Qwen 3.5）。
// 特性（调研结论 docs/调研-example-35b-v2工具调用根因.md）：
//   - 工具调用是 Qwen 风格 XML（<tool_call>），需 --jinja + 外部 chat template 解析
//   - GGUF 内嵌模板是旧版（raise_exception bug，system 非首条 400）→ 必须外部模板覆盖
//   - 需 --chat-template-kwargs：enable_thinking / preserve_thinking / auto_disable_thinking_with_tools:false
//   - "该调不调"重提示（unsloth PR #4769）
type Ornith struct{}

func (a *Ornith) Name() string { return "example-35b-v2" }

func (a *Ornith) BuildArgs(entry *registry.ModelEntry, port int) []string {
	tmpl := a.templatePath()
	args := []string{
		"-m", entry.File,
		"-c", "0", // 自动用模型训练上下文（qwen35moe GQA，262144 增量小 ~4.8GB）
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		"-np", "1", // v2.5.5 单槽铁律（Mr2109——执行层面单槽——GPU全负荷）
		"-cb",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"--jinja", // 启用 GGUF 内嵌模板（外部模板覆盖前的基础）
	}
	// D3 多模态：视觉投影（example-35b-v2-1.5 是 qwen35moe-vlm——agent_models.yaml mmproj 字段）
	if entry.MMProj != "" {
		args = append(args, "-mm", entry.MMProj)
	}
	if tmpl != "" {
		args = append(args,
			"--chat-template-file", tmpl,
			"--chat-template-kwargs", `{"enable_thinking":true,"preserve_thinking":true,"auto_disable_thinking_with_tools":false}`,
		)
	}
	return args
}

// templatePath 返回 example-35b-v2 外部 chat template 路径（若存在）。
func (a *Ornith) templatePath() string {
	candidates := []string{
		"/home/g01/agent/example-35b-v2_chat_template.jinja", // X3
		"~/agent/example-35b-v2_chat_template.jinja", // 本机
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func (a *Ornith) ToolCallStyle() string { return "xml" }

func (a *Ornith) NeedsToolReminder() bool { return true }

func (a *Ornith) ReminderPrompt() string {
	return "Always call tools directly. Never describe what you plan to do — just call the tool immediately."
}

func (a *Ornith) ThinkingFormat() string { return "deepseek" }

func (a *Ornith) ContextWindow() int { return 262144 }
