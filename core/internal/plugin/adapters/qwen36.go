package adapters

// Qwen36Adapter — Qwen3.6-35B-A3B 模型适配器（2026-08-28 补全——Mr2109: 完成所有模型适配器）
// 特性: 阿里 35B总/3B激活 稀疏 MoE——262144 上下文——思考模型(qwen3 reasoning parser)
// 工具: qwen3_coder tool-call parser——agentic 编码强（Terminal-Bench 51.5/SWE-bench Pro 49.5）
// 优化: 编码/智能体任务主力——思考默认开——工具调用强

import (
	"fmt"

	"zerg/core/internal/plugin"
)

const (
	qwen36Name        = "Qwen3.6-35B-A3B"
	qwen36CtxWindow   = 262144 // 262K——官方（fleet.yaml 262144）
	qwen36DefaultTemp = 0.7    // 工具调用稳定（Qwen 官方推荐 0.6-0.7）
	qwen36MaxTokens   = 32768  // reasoning+content 都装——默认不掐思考
	qwen36Timeout     = 120    // 首 token 超时（思考模型——同 Qwen3.8 实测 16s——留余量）
)

// Qwen36Config — 适配器配置
type Qwen36Config struct {
	Temperature   float64
	TopP          float64
	TopK          int
	MinP          float64
	MaxTokens     int
	CtxWindow     int
	TimeoutSec    int
	ReasoningEffort string // low/medium/high——思考深度控制
	AuthToken     string
	BaseURL       string
}

// Qwen36Adapter — 实现 plugin.Plugin 接口
type Qwen36Adapter struct {
	config Qwen36Config
}

var _ plugin.Plugin = (*Qwen36Adapter)(nil)

// NewQwen36Adapter — 创建适配器（默认配置）
func NewQwen36Adapter() *Qwen36Adapter {
	return &Qwen36Adapter{
		config: Qwen36Config{
			Temperature:     qwen36DefaultTemp,
			TopP:            0.8,
			TopK:            20,
			MinP:            0,
			MaxTokens:       qwen36MaxTokens,
			CtxWindow:       qwen36CtxWindow,
			TimeoutSec:      qwen36Timeout,
			ReasoningEffort: "low", // 同 Qwen3.8——少思考更快
		},
	}
}

// Name — 插件名
func (a *Qwen36Adapter) Name() string { return qwen36Name }

// Type — 插件类型
func (a *Qwen36Adapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *Qwen36Adapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *Qwen36Adapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "tool-calling", "coding"}
}

// Init — 初始化配置
func (a *Qwen36Adapter) Init(cfg map[string]any) error {
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
	if v, ok := cfg["auth_token"].(string); ok {
		a.config.AuthToken = v
	}
	if v, ok := cfg["base_url"].(string); ok {
		a.config.BaseURL = v
	}
	return nil
}

// Start — 启动（无内部进程）
func (a *Qwen36Adapter) Start() error { return nil }

// Stop — 停止
func (a *Qwen36Adapter) Stop() error { return nil }

// Close — 关闭
func (a *Qwen36Adapter) Close() error { return nil }

// Execute — 执行（元数据回显——实际路由走网关）
func (a *Qwen36Adapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("Qwen3.6-35B-A3B: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":             qwen36Name,
			"max_tokens":        a.config.MaxTokens,
			"ctx_window":        a.config.CtxWindow,
			"temperature":       a.config.Temperature,
			"top_p":             a.config.TopP,
			"top_k":             a.config.TopK,
			"min_p":             a.config.MinP,
			"timeout_sec":       a.config.TimeoutSec,
			"reasoning_effort":  a.config.ReasoningEffort,
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"tool_support":      true,   // qwen3_coder 工具解析
			"coding":            true,   // agentic 编码强
		},
	}, nil
}

// GetDescriptions — 描述
func (a *Qwen36Adapter) GetDescriptions() string {
	return "Qwen3.6-35B-A3B——阿里35B/3B激活 MoE——262K上下文——思考模型——agentic编码强——工具调用强"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (q *Qwen36Adapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":     q.config.Temperature,
		"TopP":            q.config.TopP,
		"TopK":            q.config.TopK,
		"MinP":            q.config.MinP,
		"MaxTokens":       q.config.MaxTokens,
		"CtxWindow":       q.config.CtxWindow,
		"TimeoutSec":      q.config.TimeoutSec,
		"ReasoningEffort": q.config.ReasoningEffort,
		"AuthToken":       q.config.AuthToken,
		"BaseURL":         q.config.BaseURL,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (q *Qwen36Adapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: q.config.Temperature, Desc: "采样温度", Min: 0, Max: 2},
		{Key: "top_p", Type: "number", Value: q.config.TopP, Desc: "核采样", Min: 0, Max: 1},
		{Key: "top_k", Type: "number", Value: q.config.TopK, Desc: "Top-K 采样", Min: 0, Max: 100},
		{Key: "min_p", Type: "number", Value: q.config.MinP, Desc: "Min-P 采样", Min: 0, Max: 1},
		{Key: "max_tokens", Type: "number", Value: q.config.MaxTokens, Desc: "输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: q.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "timeout_sec", Type: "number", Value: q.config.TimeoutSec, Desc: "首 token 超时（秒）", Min: 1},
		{Key: "reasoning_effort", Type: "enum", Value: q.config.ReasoningEffort, Desc: "思考深度", Options: []string{"low", "medium", "high"}},
		{Key: "base_url", Type: "string", Value: q.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: q.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (q *Qwen36Adapter) UpdateOptions(cfg map[string]interface{}) error {
	return q.Init(cfg)
}
