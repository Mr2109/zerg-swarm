package adapters

// NemotronAdapter — NVIDIA Nemotron-3.5-Lightning-30B-A3B 模型适配器
// 特性: LatentMoE(30B总/3B激活——极快~400tok/s) / 1M上下文 / 英文强中文弱
// 思考: 默认开——content空时reasoning_content兜底(CA工具调用场景)
// v2.5.4.9 注册(2026-08-16——Mr2109:上下文最大+maxtoken=模型最大上下文)

import (
	"fmt"

	"zerg/core/internal/plugin"
)

const (
	nemotronName        = "Nemotron-3.5-Lightning"
	nemotronCtxWindow   = 1048576 // 1M——模型最大上下文
	nemotronDefaultTemp = 0.7     // 英文任务温度
	nemotronMaxTokens   = 32768   // 默认 max_tokens（reasoning+content 都装——可按需调）
)

// NemotronConfig — 适配器配置
type NemotronConfig struct {
	Temperature float64
	AuthToken   string
	BaseURL     string
	MaxTokens   int // 默认 32768（模型最大 1M——按任务设）
	CtxWindow   int // 默认 1M（模型最大上下文）
}

// NemotronAdapter — 实现 plugin.Plugin 接口（模型适配器）
type NemotronAdapter struct {
	config NemotronConfig
}

var _ plugin.Plugin = (*NemotronAdapter)(nil)

// NewNemotronAdapter — 创建适配器（默认配置）
func NewNemotronAdapter() *NemotronAdapter {
	return &NemotronAdapter{
		config: NemotronConfig{
			Temperature: nemotronDefaultTemp,
			MaxTokens:   nemotronMaxTokens,
			CtxWindow:   nemotronCtxWindow,
		},
	}
}

// Name — 插件名（模型名）
func (a *NemotronAdapter) Name() string { return nemotronName }

// Type — 插件类型（模型适配器）
func (a *NemotronAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *NemotronAdapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *NemotronAdapter) Capabilities() []string {
	return []string{"model", "reasoning", "agentic", "high-throughput"}
}

// Init — 初始化配置
func (a *NemotronAdapter) Init(cfg map[string]any) error {
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
	if m, ok := cfg["max_tokens"]; ok {
		switch v := m.(type) {
		case float64:
			a.config.MaxTokens = int(v)
		case int:
			a.config.MaxTokens = v
		}
	}
	if c, ok := cfg["ctx_window"]; ok {
		switch v := c.(type) {
		case float64:
			a.config.CtxWindow = int(v)
		case int:
			a.config.CtxWindow = v
		}
	}
	if t, ok := cfg["auth_token"].(string); ok {
		a.config.AuthToken = t
	}
	if b, ok := cfg["base_url"].(string); ok {
		a.config.BaseURL = b
	}
	return nil
}

// Start — 启动（模型适配器无内部进程——网关统一路由）
func (a *NemotronAdapter) Start() error { return nil }

// Stop — 停止
func (a *NemotronAdapter) Stop() error { return nil }

// Close — 关闭
func (a *NemotronAdapter) Close() error { return nil }

// Execute — 执行请求（适配器接口——构造请求/解析响应）
func (a *NemotronAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	// 模型适配器由网关统一路由——这里做参数校验/转发元数据
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("Nemotron: 输入为空")
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":      nemotronName,
			"max_tokens": a.config.MaxTokens,
			"ctx_window": a.config.CtxWindow,
			"temperature": a.config.Temperature,
			// 特性标记——网关路由/适配层用
			"reasoning_fallback": true, // content空时用reasoning_content兜底
			"lang":               "en", // 英文强——中文弱
		},
	}, nil
}

// GetDescriptions — 描述（插件注册用）
func (a *NemotronAdapter) GetDescriptions() string {
	return "Nemotron 3.5 Lightning——30B-A3B LatentMoE——1M上下文——英文高速(~400tok/s)——Agent/长文档/批量任务"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (a *NemotronAdapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature": a.config.Temperature,
		"AuthToken":   a.config.AuthToken,
		"BaseURL":     a.config.BaseURL,
		"MaxTokens":   a.config.MaxTokens,
		"CtxWindow":   a.config.CtxWindow,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (a *NemotronAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: a.config.Temperature, Desc: "采样温度", Min: 0, Max: 2},
		{Key: "max_tokens", Type: "number", Value: a.config.MaxTokens, Desc: "输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: a.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "base_url", Type: "string", Value: a.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: a.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (a *NemotronAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return a.Init(cfg)
}
