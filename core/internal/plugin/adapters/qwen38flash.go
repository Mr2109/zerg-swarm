package adapters

// Qwen38FlashAdapter — Qwen3.8-Flash-Next 模型适配器（2026-08-29 注册）
// 特性: 125B MoE(6B激活) / qwen4架构(GDN+QSA+PLE 51B n-gram) / 262K上下文 / 多模态 / 思考模型
// 实测(X3 2026-08-29): UD-Q4_K_XL 111GB / 21.7 t/s / reasoning_content+content 正常
// 思考: 默认开——reasoning_effort 控制——content空时reasoning兜底（同 Qwen3.8-27B）
// 注意: 运行 ctx 受内存限制（X3 122G 装 111GB 模型后 KV 空间小）——实际 -c 32768（agent 适配器控制）

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

const (
	qwen38FlashName        = "Qwen3.8-Flash-Next"
	qwen38FlashCtxWindow   = 262144 // 训练上下文（内存受限时实际运行 32K）
	qwen38FlashDefaultTemp = 1.0    // 官方推荐：思考模式 1.0（非思考 0.7——2026-08-30 网络调研 HF 模型卡）
	qwen38FlashMaxTokens   = 32768
	qwen38FlashTimeout     = 120 // 首 token 超时（思考模式长）
)

// Qwen38FlashConfig — 适配器配置
type Qwen38FlashConfig struct {
	Temperature     float64
	TopP            float64
	TopK            int
	MinP            float64
	PresencePenalty float64 // 官方：思考 0.0 / 非思考 1.5（防重复）
	AuthToken       string
	BaseURL         string
	MaxTokens       int
	CtxWindow       int
	TimeoutSec      int
	ReasoningEffort string
}

// Qwen38FlashAdapter — 实现 plugin.Plugin 接口
type Qwen38FlashAdapter struct {
	config Qwen38FlashConfig
}

var _ plugin.Plugin = (*Qwen38FlashAdapter)(nil)

// NewQwen38FlashAdapter — 创建适配器
func NewQwen38FlashAdapter() *Qwen38FlashAdapter {
	return &Qwen38FlashAdapter{
		config: Qwen38FlashConfig{
			Temperature:     qwen38FlashDefaultTemp,
			TopP:            0.95, // 官方推荐思考模式（2026-08-30 网络调研）
			TopK:            20,
			MinP:            0,
			PresencePenalty: 0, // 官方思考模式 0.0（非思考推荐 1.5——客户端可按需传）
			MaxTokens:       qwen38FlashMaxTokens,
			CtxWindow:       qwen38FlashCtxWindow,
			TimeoutSec:      qwen38FlashTimeout,
			ReasoningEffort: "low", // Mr2109: 思考模式 low——少思考更快（官方默认 xhigh——选项已含）
		},
	}
}

// Name — 插件名
func (a *Qwen38FlashAdapter) Name() string { return qwen38FlashName }

// Type — 插件类型
func (a *Qwen38FlashAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *Qwen38FlashAdapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *Qwen38FlashAdapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "multimodal", "tool-calling"}
}

// Init — 初始化配置（覆盖式——实时生效）
func (a *Qwen38FlashAdapter) Init(cfg map[string]any) error {
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
			return fmt.Errorf("qwen38flash: min_p must be number, got %T", v)
		}
	}
	if v, ok := cfg["presence_penalty"]; ok {
		switch t := v.(type) {
		case float64:
			a.config.PresencePenalty = t
		case float32:
			a.config.PresencePenalty = float64(t)
		case int:
			a.config.PresencePenalty = float64(t)
		default:
			return fmt.Errorf("qwen38flash: presence_penalty must be number, got %T", v)
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
func (a *Qwen38FlashAdapter) Start() error { return nil }

// Stop — 停止
func (a *Qwen38FlashAdapter) Stop() error { return nil }

// Close — 关闭
func (a *Qwen38FlashAdapter) Close() error { return nil }

// Execute — 执行（元数据回显——实际路由走网关）
func (a *Qwen38FlashAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("Qwen3.8-Flash-Next: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":              qwen38FlashName,
			"max_tokens":         a.config.MaxTokens,
			"ctx_window":         a.config.CtxWindow,
			"temperature":        a.config.Temperature,
			"top_p":              a.config.TopP,
			"top_k":              a.config.TopK,
			"min_p":              a.config.MinP,
			"presence_penalty":   a.config.PresencePenalty,
			"timeout_sec":        a.config.TimeoutSec,
			"reasoning_effort":   a.config.ReasoningEffort,
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"multimodal":         true, // 原生多模态（mmproj 待配）
		},
	}, nil
}

// GetDescriptions — 描述
func (a *Qwen38FlashAdapter) GetDescriptions() string {
	return "Qwen3.8-Flash-Next——125B MoE(6B激活) qwen4架构——中文通用——思考模型——Qwen4 架构预览"
}

// Options 适配器配置项
func (q *Qwen38FlashAdapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":     q.config.Temperature,
		"TopP":            q.config.TopP,
		"TopK":            q.config.TopK,
		"MinP":            q.config.MinP,
		"PresencePenalty": q.config.PresencePenalty,
		"AuthToken":       q.config.AuthToken,
		"BaseURL":         q.config.BaseURL,
		"MaxTokens":       q.config.MaxTokens,
		"CtxWindow":       q.config.CtxWindow,
		"TimeoutSec":      q.config.TimeoutSec,
		"ReasoningEffort": q.config.ReasoningEffort,
	}
}

// OptionSchema 声明本适配器可编辑参数（各模型各自声明——不统一套用）
func (q *Qwen38FlashAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: q.config.Temperature, Desc: "采样温度（官方思考 1.0/非思考 0.7）", Min: 0, Max: 2},
		{Key: "top_p", Type: "number", Value: q.config.TopP, Desc: "核采样（官方思考 0.95）", Min: 0, Max: 1},
		{Key: "top_k", Type: "number", Value: q.config.TopK, Desc: "Top-K 采样（官方 20）", Min: 0, Max: 100},
		{Key: "min_p", Type: "number", Value: q.config.MinP, Desc: "Min-P 采样（官方 0）", Min: 0, Max: 1},
		{Key: "presence_penalty", Type: "number", Value: q.config.PresencePenalty, Desc: "存在惩罚（官方思考 0/非思考 1.5 防重复）", Min: 0, Max: 2},
		{Key: "max_tokens", Type: "number", Value: q.config.MaxTokens, Desc: "输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: q.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "timeout_sec", Type: "number", Value: q.config.TimeoutSec, Desc: "首 token 超时（秒）", Min: 1},
		{Key: "reasoning_effort", Type: "enum", Value: q.config.ReasoningEffort, Desc: "思考深度（官方档位）", Options: []string{"xhigh", "medium", "low", "none"}},
		{Key: "base_url", Type: "string", Value: q.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: q.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置
func (q *Qwen38FlashAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return q.Init(cfg)
}
