package agent

// subagent.go — 虫族 v2.5 子 Agent 委托
// 设计：子 agent 隔离父上下文，返回精简摘要，避免 token 爆炸
// 内置三类型：Explore（探索）/ Plan（规划）/ General（通用）
// 对齐：设计文档 docs/设计-v2.5-替代Codex-v2.md 第十七章

import (
	"context"
	"fmt"
	"strings"
)

// SubagentType — 子 agent 类型标识
type SubagentType string

const (
	TypeExplore SubagentType = "explore" // 探索：只读工具，快速了解代码库
	TypePlan    SubagentType = "plan"    // 规划：只读工具，生成实施计划
	TypeGeneral SubagentType = "general" // 通用：全部工具，执行具体任务
)

// SubagentConfig — 子 agent 运行配置
type SubagentConfig struct {
	Type     SubagentType // 子 agent 类型
	Model    string       // 模型名称（explore 用轻量模型——nanbeige）
	MaxTurns int          // 每层配额——默认 20
	Tools    []string     // 工具白名单（allowlist）
}

// TaskInput — 子 agent 执行的任务描述
type TaskInput struct {
	Description string       // 任务简述（用于日志/摘要）
	Prompt      string       // 完整提示词（发给子 agent）
	Type        SubagentType // 子 agent 类型
	Model       string       // 覆盖默认模型
}

// SubagentResult — 子 agent 执行结果
type SubagentResult struct {
	TaskDesc   string          // 任务描述
	Summary    string          // 精简摘要（1-2K 字符，父上下文干净）
	Terminates TerminateReason // 终止原因
	ToolUse    string          // 工具调用摘要（如 "read×3, grep×1"）
}

// SpawnSubagent — 派生子 agent 执行小任务，返回精简摘要
//
// 设计要点：
//   - 子 Agent 有独立 history（不继承父上下文，避免 token 爆炸）
//   - 复用 Agent 的 Loop 循环（直接调用 Loop 函数）
//   - MaxTurns 限制配额，防止子任务失控
//   - 返回摘要仅 1-2K 字符，父上下文保持干净
//
// 参数：
//   - parent: 父 agent（提供默认配置：网关/超时/工作区）
//   - task: 任务输入（描述/提示词/类型/模型）
//
// 返回：
//   - SubagentResult：摘要（子 agent 的工作成果）
//   - error：子 agent 执行失败时返回
func SpawnSubagent(parent *Agent, task TaskInput) (SubagentResult, error) {
	// 1. 确定配置——继承父 agent 的网关/超时/工作区
	cfg := parent.cfg
	if task.Model != "" {
		cfg.Model = task.Model
	} else {
		cfg.Model = defaultModelFor(task.Type)
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 20
	}

	// 2. 创建子 Agent（独立 history——不继承父）
	child := &Agent{
		cfg:         cfg,
		history:     make([]Message, 0),
		execContext: parent.execContext, // 共享执行上下文（同一工作区）
	}

	// 3. 注入终止回调（用于检测超轮数）
	child.terminator = func(reason TerminateReason) {
		// 子 agent 终止时记录原因
	}

	// 4. 构建工具集（按类型过滤）
	tools := resolveTools(task.Type)

	// 5. 构建子 agent 系统提示（注入 history——Loop 无 sysPrompt 参数）
	sysPrompt := buildSubagentPrompt(task)
	child.history = append(child.history, Message{Role: "system", Content: sysPrompt})

	// 6. 执行循环（通过 Loop 复用 Run 逻辑）
	result := Loop(context.Background(), child, tools, nil, nil, cfg.MaxTurns, 0, 3)

	// 7. 提取摘要
	summary := extractSummary(child, result, task)

	return SubagentResult{
		TaskDesc:   task.Description,
		Summary:    summary,
		Terminates: result.Reason,
		ToolUse:    summarizeToolUse(child),
	}, nil
}

// toolSetFor — 按子 agent 类型返回工具白名单
// Explore：只读工具（read/grep/glob）—— 只看不改
// Plan：只读工具（read/grep/glob）—— 只分析不执行
// General：全部工具（read/write/edit/glob/grep/ls/bash）
func toolSetFor(t SubagentType) []string {
	switch t {
	case TypeExplore:
		return []string{"read", "grep", "glob"}
	case TypePlan:
		return []string{"read", "grep", "glob"}
	case TypeGeneral:
		return []string{"read", "write", "edit", "glob", "grep", "ls", "bash"}
	default:
		return []string{"read", "grep", "glob"}
	}
}

// defaultModelFor — 按子 agent 类型返回默认模型
func defaultModelFor(t SubagentType) string {
	switch t {
	case TypeExplore:
		return "nanbeige" // 探索用轻量模型
	case TypePlan:
		return "nanbeige" // 规划用轻量模型
	case TypeGeneral:
		return "" // 通用用父 agent 的模型
	default:
		return ""
	}
}

// buildSubagentPrompt — 构建子 agent 系统提示
func buildSubagentPrompt(task TaskInput) string {
	prompt := fmt.Sprintf("你是虫族子 Agent（%s 模式）。\n\n", task.Type)
	prompt += fmt.Sprintf("任务描述: %s\n\n", task.Description)
	prompt += "请直接执行任务，返回精炼的结果摘要。\n"
	prompt += "不要询问确认，不要解释过程，直接输出结果。\n"
	prompt += fmt.Sprintf("提示词:\n%s", task.Prompt)
	return prompt
}

// resolveTools — 解析工具集（根据类型选择 allowlist）
func resolveTools(t SubagentType) []ToolDef {
	allowlist := toolSetFor(t)

	// 构建工具定义列表——只包含 allowlist 中的工具
	var tools []ToolDef
	for _, name := range allowlist {
		tools = append(tools, toolDefFor(name))
	}
	return tools
}

// toolDefFor — 返回单个工具的定义
func toolDefFor(name string) ToolDef {
	switch name {
	case "read":
		return toolRead()
	case "write":
		return toolWrite()
	case "edit":
		return toolEdit()
	case "glob":
		return toolGlob()
	case "grep":
		return toolGrep()
	case "ls":
		return toolLs()
	case "bash":
		return toolBash()
	default:
		return ToolDef{}
	}
}

// extractSummary — 从子 agent 执行结果中提取摘要
func extractSummary(child *Agent, result LoopResult, task TaskInput) string {
	// 优先使用最后一条助手消息
	lastContent := ""
	if len(child.history) > 0 {
		last := child.history[len(child.history)-1]
		if last.Role == "assistant" {
			lastContent = last.Content
		}
	}

	// 截断到 2K 字符
	summary := truncate(lastContent, 2048)
	if summary == "" {
		summary = fmt.Sprintf("子 agent(%s) 终止: %s, 共 %d 轮", task.Type, result.Reason, result.Turns)
	}
	return summary
}

// summarizeToolUse — 汇总工具使用情况
func summarizeToolUse(child *Agent) string {
	toolCounts := make(map[string]int)
	for _, msg := range child.history {
		if msg.Role != "assistant" {
			continue
		}
		for _, tc := range msg.ToolCalls {
			toolCounts[tc.Name]++
		}
	}
	var parts []string
	for name, count := range toolCounts {
		parts = append(parts, fmt.Sprintf("%s×%d", name, count))
	}
	return strings.Join(parts, ", ")
}
