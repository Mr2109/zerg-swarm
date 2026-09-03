// Package modeladapter 模型适配层。
//
// 与客户端适配层（gateway/adapter）对称：按模型模块化不同模型的行为策略。
// 每个模型适配器管理：启动参数 / 工具调用风格 / 重提示策略 / 思考格式 / 上下文窗口。
//
// 参考设计：docs/设计-模型适配层.md
package modeladapter

import (
	"strings"
	"sync"

	"zerg/agent/internal/registry"
)

// ModelAdapter 模型适配器接口。
type ModelAdapter interface {
	// Name 适配的模型名前缀（如 "ornith"、"qwen3.6"），Dispatch 按前缀匹配。
	Name() string
	// BuildArgs 启动参数（替代散落的 cmd）。可含 {file}/{port}/{dir} 占位符（由 manager 替换）。
	BuildArgs(entry *registry.ModelEntry, port int) []string
	// ToolCallStyle 工具调用风格：xml（Qwen 风格 <tool_call>）/ json（标准 OpenAI tool_calls）。
	ToolCallStyle() string
	// NeedsToolReminder 该模型是否需要"该调不调"重提示。
	NeedsToolReminder() bool
	// ReminderPrompt 重提示注入的系统提示（unsloth 方案）。
	ReminderPrompt() string
	// ThinkingFormat 思考格式：deepseek / plain / none。
	ThinkingFormat() string
	// ContextWindow 上下文窗口建议（max_tokens 等）。
	ContextWindow() int
}

// Registry 适配器注册表。
var (
	mu      sync.RWMutex
	adapters []ModelAdapter
)

// Register 注册适配器（包 init 调用）。
func Register(a ModelAdapter) {
	mu.Lock()
	defer mu.Unlock()
	adapters = append(adapters, a)
}

// Dispatch 按模型名匹配适配器（前缀匹配，最长优先，不区分大小写）。
// 找不到匹配时返回 generic（默认行为）。
func Dispatch(modelName string) ModelAdapter {
	mu.RLock()
	defer mu.RUnlock()
	var best ModelAdapter
	bestLen := 0
	lower := strings.ToLower(modelName)
	for _, a := range adapters {
		name := strings.ToLower(a.Name())
		if name == "" {
			continue
		}
		if strings.HasPrefix(lower, name) && len(name) > bestLen {
			best = a
			bestLen = len(name)
		}
	}
	if best != nil {
		return best
	}
	return &Generic{}
}

// init 注册内置适配器。
func init() {
	Register(&Generic{})
	Register(&Ornith{})
	Register(&Qwen36{})
	Register(&Qwen38Flash{}) // 2026-08-29 Qwen3.8-Flash-Next（qwen4 架构——前缀优先于 qwen3）
	Register(&DS4{})
	Register(&Gemma{})
}
