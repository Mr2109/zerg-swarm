package adapters

// Glm47Adapter — GLM-4.7-Flash 模型适配器（2026-08-28 补全——Mr2109: 完成所有模型适配器）
// 特性: 智谱 30B-A3B MoE——128K+ 上下文(MLA)——思考默认开(Enable Thinking)——工具调用强
// 官方评测参数: temp 1.0 / top_p 0.95 / max 131072——思考模型——τ²-Bench 79.5/SWE-bench 59.2
// 优化: 编码/UI生成/工具调用强——Haiku 等效——适合执行类任务

import (
	"fmt"

	"zerg/core/internal/plugin"
)

const (
	glm47Name        = "GLM-4.7-Flash"
	glm47CtxWindow   = 131072 // 128K——官方（fleet.yaml 131072）
	glm47DefaultTemp = 0.7    // 工具调用稳定（官方评测 1.0 是基准测试——生产用 0.7 稳）
	glm47MaxTokens   = 32768  // reasoning+content 都装（官方 max 131072——默认不掐思考）
	glm47Timeout     = 120    // 首 token 超时（思考模型——同 Qwen3.8 实测 16s——留余量）
)

// Glm47Config — 适配器配置
type Glm47Config struct {
	Temperature   float64
	TopP          float64
	TopK          int
	MinP          float64
	MaxTokens     int
	CtxWindow     int
	TimeoutSec    int
	ReasoningEffort string // low/medium/high——思考深度控制
	Thinking      bool   // 思考开关（官方 Enable Thinking——默认 true——思考不能关）
	AuthToken     string
	BaseURL       string
}

// Glm47Adapter — 实现 plugin.Plugin 接口
type Glm47Adapter struct {
	config Glm47Config
}

var _ plugin.Plugin = (*Glm47Adapter)(nil)

// NewGlm47Adapter — 创建适配器（默认配置——思考开）
func NewGlm47Adapter() *Glm47Adapter {
	return &Glm47Adapter{
		config: Glm47Config{
			Temperature:     glm47DefaultTemp,
			TopP:            0.95, // 官方评测
			TopK:            50,
			MinP:            0,
			MaxTokens:       glm47MaxTokens,
			CtxWindow:       glm47CtxWindow,
			TimeoutSec:      glm47Timeout,
			ReasoningEffort: "low",   // 同 Qwen3.8——少思考更快
			Thinking:        true,    // 思考不能关（Mr2109原则）
		},
	}
}

// Name — 插件名
func (a *Glm47Adapter) Name() string { return glm47Name }

// Type — 插件类型
func (a *Glm47Adapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *Glm47Adapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *Glm47Adapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "tool-calling", "coding"}
}

// Init — 初始化配置
func (a *Glm47Adapter) Init(cfg map[string]any) error {
	if cfg == nil {
		return nil
	}
	if t, ok := cfg["temperature"]; ok {
		switch v := t.(type) {
		case float64:
			a.config.Temperature = v
		case int:
			a.config.Temperature = float64(v)
		}
	}
	if v, ok := cfg["top_p"]; ok {
		if f, ok := v.(float64); ok {
			a.config.TopP = f
		}
	}
	if v, ok := cfg["top_k"]; ok {
		if i, ok := v.(int); ok {
			a.config.TopK = i
		}
	}
	if v, ok := cfg["min_p"]; ok {
		switch t := v.(type) {
		case float64:
			a.config.MinP = t
		case int:
			a.config.MinP = float64(t)
		}
	}
	if v, ok := cfg["max_tokens"]; ok {
		switch v := v.(type) {
		case float64:
			a.config.MaxTokens = int(v)
		case int:
			a.config.MaxTokens = v
		}
	}
	if v, ok := cfg["ctx_window"]; ok {
		switch v := v.(type) {
		case float64:
			a.config.CtxWindow = int(v)
		case int:
			a.config.CtxWindow = v
		}
	}
	if v, ok := cfg["timeout_sec"]; ok {
		switch v := v.(type) {
		case float64:
			a.config.TimeoutSec = int(v)
		case int:
			a.config.TimeoutSec = v
		}
	}
	if v, ok := cfg["reasoning_effort"].(string); ok {
		a.config.ReasoningEffort = v
	}
	if v, ok := cfg["thinking"]; ok {
		if b, ok := v.(bool); ok {
			a.config.Thinking = b
		}
	}
	if v, ok := cfg["auth_token"].(string); ok {
		a.config.AuthToken = v
	}
	if v, ok := cfg["base_url"].(string); ok {
		a.config.BaseURL = v
	}
	return nil
}

// Start — 启动（无内部进程）
func (a *Glm47Adapter) Start() error { return nil }

// Stop — 停止
func (a *Glm47Adapter) Stop() error { return nil }

// Close — 关闭
func (a *Glm47Adapter) Close() error { return nil }

// Execute — 执行（元数据回显——实际路由走网关）
func (a *Glm47Adapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("GLM-4.7-Flash: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":             glm47Name,
			"max_tokens":        a.config.MaxTokens,
			"ctx_window":        a.config.CtxWindow,
			"temperature":       a.config.Temperature,
			"top_p":             a.config.TopP,
			"top_k":             a.config.TopK,
			"min_p":             a.config.MinP,
			"timeout_sec":       a.config.TimeoutSec,
			"reasoning_effort":  a.config.ReasoningEffort,
			"thinking":          a.config.Thinking,
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"tool_support":      true,
		},
	}, nil
}

// GetDescriptions — 描述
func (a *Glm47Adapter) GetDescriptions() string {
	return "GLM-4.7-Flash——智谱30B-A3B MoE——128K上下文(MLA)——思考模型——工具调用强——编码/UI生成"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (g *Glm47Adapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":     g.config.Temperature,
		"TopP":            g.config.TopP,
		"TopK":            g.config.TopK,
		"MinP":            g.config.MinP,
		"MaxTokens":       g.config.MaxTokens,
		"CtxWindow":       g.config.CtxWindow,
		"TimeoutSec":      g.config.TimeoutSec,
		"ReasoningEffort": g.config.ReasoningEffort,
		"Thinking":        g.config.Thinking,
		"AuthToken":       g.config.AuthToken,
		"BaseURL":         g.config.BaseURL,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (g *Glm47Adapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: g.config.Temperature, Desc: "采样温度", Min: 0, Max: 2},
		{Key: "top_p", Type: "number", Value: g.config.TopP, Desc: "核采样", Min: 0, Max: 1},
		{Key: "top_k", Type: "number", Value: g.config.TopK, Desc: "Top-K 采样", Min: 0, Max: 100},
		{Key: "min_p", Type: "number", Value: g.config.MinP, Desc: "Min-P 采样", Min: 0, Max: 1},
		{Key: "max_tokens", Type: "number", Value: g.config.MaxTokens, Desc: "输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: g.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "timeout_sec", Type: "number", Value: g.config.TimeoutSec, Desc: "首 token 超时（秒）", Min: 1},
		{Key: "reasoning_effort", Type: "enum", Value: g.config.ReasoningEffort, Desc: "思考深度", Options: []string{"low", "medium", "high"}},
		{Key: "thinking", Type: "bool", Value: g.config.Thinking, Desc: "思考开关（默认开——思考不能关）"},
		{Key: "base_url", Type: "string", Value: g.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: g.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (g *Glm47Adapter) UpdateOptions(cfg map[string]interface{}) error {
	return g.Init(cfg)
}
