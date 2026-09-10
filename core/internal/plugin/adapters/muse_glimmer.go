package adapters

// MuseGlimmerAdapter — example-30b 模型适配器（2026-08-28 补全——Mr2109: 完成所有模型适配器）
// 特性: Meta 296亿稠密 多模态(文本+图像→文本)——131K+ 上下文——四档推理强度(low/medium/high/xhigh)
// 工具: 结构化函数调用强——agentic（工具失败恢复/编程/电脑操作/LLM-as-a-judge）
// 官方评测参数: temp 1.0 / top_p 0.95 / top_k 64 / reasoning strength high
// 注意: 训练覆盖100+语言但中文未全面评测（英文强）——长上下文 AA-LCR 第1

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

const (
	museGlimmerName        = "example-30b"
	museGlimmerCtxWindow   = 131072 // 131K——官方（fleet.yaml 131072）
	museGlimmerDefaultTemp = 0.8    // 生产默认（官方评测 1.0 是基准测试）
	museGlimmerMaxTokens   = 32768  // 默认 max_tokens（reasoning+content 都装）
	museGlimmerTimeout     = 120    // 首 token 超时（推理模型——留余量）
)

// MuseGlimmerConfig — 适配器配置
type MuseGlimmerConfig struct {
	Temperature      float64
	TopP             float64
	TopK             int
	MinP             float64
	MaxTokens        int
	CtxWindow        int
	TimeoutSec       int
	ReasoningEffort  string // low/medium/high/xhigh——官方四档推理强度
	Multimodal       bool   // 原生多模态（ViT-G/14 感知编码器）
	AuthToken        string
	BaseURL          string
}

// MuseGlimmerAdapter — 实现 plugin.Plugin 接口
type MuseGlimmerAdapter struct {
	config MuseGlimmerConfig
}

var _ plugin.Plugin = (*MuseGlimmerAdapter)(nil)

// NewMuseGlimmerAdapter — 创建适配器（默认配置）
func NewMuseGlimmerAdapter() *MuseGlimmerAdapter {
	return &MuseGlimmerAdapter{
		config: MuseGlimmerConfig{
			Temperature:      museGlimmerDefaultTemp,
			TopP:             0.95, // 官方评测
			TopK:             64,   // 官方评测
			MinP:             0,
			MaxTokens:        museGlimmerMaxTokens,
			CtxWindow:        museGlimmerCtxWindow,
			TimeoutSec:       museGlimmerTimeout,
			ReasoningEffort:  "medium", // 生产默认（官方建议复杂任务 high/xhigh）
			Multimodal:       true,     // 原生多模态
		},
	}
}

// Name — 插件名
func (a *MuseGlimmerAdapter) Name() string { return museGlimmerName }

// Type — 插件类型
func (a *MuseGlimmerAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *MuseGlimmerAdapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *MuseGlimmerAdapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "tool-calling", "multimodal", "judge"}
}

// Init — 初始化配置
func (a *MuseGlimmerAdapter) Init(cfg map[string]any) error {
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
	if v, ok := cfg["multimodal"]; ok {
		if b, ok := v.(bool); ok {
			a.config.Multimodal = b
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
func (a *MuseGlimmerAdapter) Start() error { return nil }

// Stop — 停止
func (a *MuseGlimmerAdapter) Stop() error { return nil }

// Close — 关闭
func (a *MuseGlimmerAdapter) Close() error { return nil }

// Execute — 执行（元数据回显——实际路由走网关）
func (a *MuseGlimmerAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("example-30b: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":             museGlimmerName,
			"max_tokens":        a.config.MaxTokens,
			"ctx_window":        a.config.CtxWindow,
			"temperature":       a.config.Temperature,
			"top_p":             a.config.TopP,
			"top_k":             a.config.TopK,
			"min_p":             a.config.MinP,
			"timeout_sec":       a.config.TimeoutSec,
			"reasoning_effort":  a.config.ReasoningEffort,
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"multimodal":        a.config.Multimodal, // 原生多模态（ViT-G/14）
			"tool_support":      true,                 // 结构化函数调用强
			"judge_capable":     true,                 // LLM-as-a-judge
		},
	}, nil
}

// GetDescriptions — 描述
func (a *MuseGlimmerAdapter) GetDescriptions() string {
	return "example-30b——Meta 296亿稠密 多模态——131K上下文——四档推理强度——agentic工具强——英文强"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (m *MuseGlimmerAdapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":     m.config.Temperature,
		"TopP":            m.config.TopP,
		"TopK":            m.config.TopK,
		"MinP":            m.config.MinP,
		"MaxTokens":       m.config.MaxTokens,
		"CtxWindow":       m.config.CtxWindow,
		"TimeoutSec":      m.config.TimeoutSec,
		"ReasoningEffort": m.config.ReasoningEffort,
		"Multimodal":      m.config.Multimodal,
		"AuthToken":       m.config.AuthToken,
		"BaseURL":         m.config.BaseURL,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (m *MuseGlimmerAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: m.config.Temperature, Desc: "采样温度（官方评测1.0）", Min: 0, Max: 2},
		{Key: "top_p", Type: "number", Value: m.config.TopP, Desc: "核采样（官方0.95）", Min: 0, Max: 1},
		{Key: "top_k", Type: "number", Value: m.config.TopK, Desc: "Top-K 采样（官方64）", Min: 0, Max: 100},
		{Key: "min_p", Type: "number", Value: m.config.MinP, Desc: "Min-P 采样", Min: 0, Max: 1},
		{Key: "max_tokens", Type: "number", Value: m.config.MaxTokens, Desc: "输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: m.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "timeout_sec", Type: "number", Value: m.config.TimeoutSec, Desc: "首 token 超时（秒）", Min: 1},
		{Key: "reasoning_effort", Type: "enum", Value: m.config.ReasoningEffort, Desc: "推理强度（官方四档）", Options: []string{"low", "medium", "high", "xhigh"}},
		{Key: "multimodal", Type: "bool", Value: m.config.Multimodal, Desc: "多模态（ViT-G/14 视觉）"},
		{Key: "base_url", Type: "string", Value: m.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: m.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (m *MuseGlimmerAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return m.Init(cfg)
}
