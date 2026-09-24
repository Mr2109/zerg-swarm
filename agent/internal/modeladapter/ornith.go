package modeladapter

import (
	"fmt"
	"os"
	"strings"

	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
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
	tmpl := a.templatePath(entry)
	args := []string{
		"-m", entry.File,
		"-c", "0", // 自动用模型训练上下文（qwen35moe GQA，262144 增量小 ~4.8GB）
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		// 注意：并行槽数（-np）**不在此处声明** —— 真源 = 各机卵清单的 `cmd:` 字段 ✓
		// （`hatch_spec.go:768 buildEngineArgv`：卵清单有 `cmd:` 时**整段覆盖**适配器参数 ⇒ 这里写了也不生效 ✗）
		// 实测事实：两机引擎都是 4 槽 × 262144（LLaMA 默认值），与本项目 2026-08-15 认知一致 ✓（缺口 Q-218）
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

// templatePath 解析 example-35b-v2 外部 chat template 路径（可选；都不存在时不加该参数）。
// 2026-09-11 登记表配置化：优先级如下（前两者命中即返回）
//  1. 登记表 `chat_template:` 字段 —— 任意机器/任意模型可配，推荐用法
//  2. 环境变量 ZERG_ORNITH_TEMPLATE
//  3. ~/.zerg/example-35b-v2_chat_template.jinja
//  4. 仓库内相对路径 agent/example-35b-v2_chat_template.jinja（及其上一级）
//
// 链路上任一处显式配置但文件不存在 → 记 WARN 并继续回退，不会把坏路径传给后端。
func (a *Ornith) templatePath(entry *registry.ModelEntry) string {
	if entry != nil && entry.ChatTemplate != "" {
		p := expandHome(entry.ChatTemplate)
		if fileExists(p) {
			return p
		}
		logx.Warnf("modeladapter", "登记表 chat_template 不存在，已忽略并回退", "path", entry.ChatTemplate)
	}
	candidates := []string{}
	if p := os.Getenv("ZERG_ORNITH_TEMPLATE"); p != "" {
		candidates = append(candidates, expandHome(p))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home+"/.zerg/example-35b-v2_chat_template.jinja")
	}
	candidates = append(candidates,
		"agent/example-35b-v2_chat_template.jinja",
		"../agent/example-35b-v2_chat_template.jinja",
	)
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

// expandHome 展开路径开头的 ~（仅 ~ 与 ~/ 形式，其余原样返回）。
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return home + p[1:]
		}
	}
	return p
}

// fileExists 路径存在且为普通文件。
func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func (a *Ornith) ToolCallStyle() string { return "xml" }

func (a *Ornith) NeedsToolReminder() bool { return true }

func (a *Ornith) ReminderPrompt() string {
	return "Always call tools directly. Never describe what you plan to do — just call the tool immediately."
}

func (a *Ornith) ThinkingFormat() string { return "deepseek" }

func (a *Ornith) ContextWindow() int { return 262144 }
