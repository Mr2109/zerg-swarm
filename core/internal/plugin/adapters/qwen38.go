package adapters

// Qwen38Adapter — Qwen3.8-27B 模型适配器（v2.5.4.9——注册 2026-08-16）
// 特性: 27B稠密 / Gated DeltaNet混合注意力(3:1) / 1M上下文(YaRN) / 多模态原生
// 思考: 默认开——reasoning_effort 控制——content空时reasoning兜底
// 优化: 交叉调研(我的追踪实证 + CA调研报告)——温度0.7/采样参数组/超时120s

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

const (
	qwen38Name        = "Qwen3.8-27B"
	qwen38CtxWindow   = 1048576 // 1M——模型最大上下文（YaRN 扩展）
	qwen38DefaultTemp = 0.7     // 官方推荐（思考模式 0.6-0.7——工具调用稳定）
	qwen38MaxTokens   = 32768   // 默认 max_tokens（reasoning+content 都装）
	qwen38Timeout     = 120     // 首 token 超时（秒——思考模式长——追踪实证 90s 不够）
)

// Qwen38Config — 适配器配置
type Qwen38Config struct {
	Temperature   float64
	TopP          float64 // 采样参数（官方 0.8）
	TopK          int     // 官方 20
	MinP          float64 // 官方 0
	AuthToken     string
	BaseURL       string
	MaxTokens     int
	CtxWindow     int
	TimeoutSec    int  // 首 token 超时（思考模式长）
	ReasoningEffort string // low/medium/high——思考深度控制
}

// Qwen38Adapter — 实现 plugin.Plugin 接口
type Qwen38Adapter struct {
	config Qwen38Config
}

var _ plugin.Plugin = (*Qwen38Adapter)(nil)

// NewQwen38Adapter — 创建适配器（默认配置——官方推荐）
func NewQwen38Adapter() *Qwen38Adapter {
	return &Qwen38Adapter{
		config: Qwen38Config{
			Temperature:     qwen38DefaultTemp,
			TopP:            0.8,
			TopK:            20,
			MinP:            0,
			MaxTokens:       qwen38MaxTokens,
			CtxWindow:       qwen38CtxWindow,
			TimeoutSec:      qwen38Timeout,
			ReasoningEffort: "low", // Mr2109 2026-08-20: Qwen3.8 思考模式 low（少思考——更快响应）
		},
	}
}

// Name — 插件名
func (a *Qwen38Adapter) Name() string { return qwen38Name }

// Type — 插件类型
func (a *Qwen38Adapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *Qwen38Adapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *Qwen38Adapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "multimodal", "tool-calling"}
}

// Init — 初始化配置
func (a *Qwen38Adapter) Init(cfg map[string]any) error {
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
	if v, ok := cfg["min_p"]; ok {
		switch t := v.(type) {
		case float64:
			a.config.MinP = t
		case float32:
			a.config.MinP = float64(t)
		case int:
			a.config.MinP = float64(t)
		default:
			return fmt.Errorf("qwen38: min_p must be number, got %T", v)
		}
	}

	if v, ok := cfg["top_k"]; ok {
		if i, ok := v.(int); ok {
			a.config.TopK = i
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
func (a *Qwen38Adapter) Start() error { return nil }

// Stop — 停止
func (a *Qwen38Adapter) Stop() error { return nil }

// Close — 关闭
func (a *Qwen38Adapter) Close() error { return nil }

// Execute — 执行（元数据回显——实际路由走网关——reasoning 兜底标记）
func (a *Qwen38Adapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("Qwen3.8: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":             qwen38Name,
			"max_tokens":        a.config.MaxTokens,
			"ctx_window":        a.config.CtxWindow,
			"temperature":       a.config.Temperature,
			"top_p":             a.config.TopP,
			"top_k":             a.config.TopK,
			"min_p":             a.config.MinP,
			"timeout_sec":       a.config.TimeoutSec,
			"reasoning_effort":  a.config.ReasoningEffort,
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"multimodal":        true,  // 原生多模态（mmproj 已配）
		},
	}, nil
}

// GetDescriptions — 描述
func (a *Qwen38Adapter) GetDescriptions() string {
	return "Qwen3.8-27B——27B稠密 Gated DeltaNet——1M上下文——中文通用——多模态——思考模型"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (q *Qwen38Adapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":     q.config.Temperature,
		"TopP":            q.config.TopP,
		"TopK":            q.config.TopK,
		"MinP":            q.config.MinP,
		"AuthToken":       q.config.AuthToken,
		"BaseURL":         q.config.BaseURL,
		"MaxTokens":       q.config.MaxTokens,
		"CtxWindow":       q.config.CtxWindow,
		"TimeoutSec":      q.config.TimeoutSec,
		"ReasoningEffort": q.config.ReasoningEffort,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (q *Qwen38Adapter) OptionSchema() []plugin.OptionDef {
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
func (q *Qwen38Adapter) UpdateOptions(cfg map[string]interface{}) error {
	return q.Init(cfg)
}
