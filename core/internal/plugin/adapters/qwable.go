package adapters

// QwableAdapter — Qwable-v1 模型适配器（v2.5.4.9——2026-08-17 注册）
// 特性: 双角色（编码路由 q5_k_m + MoA 参考 IQ4_XS）——MoE——高吞吐
// 优化: CA 调研（other-adapter-analysis.md）——编码 temp 0.3-0.4（确定性）/ MoA temp 0.7（多样性）
// 注意: 同一模型两个量化变体——variant 参数区分（精度/速度权衡）

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

const (
	qwableName         = "Qwable-v1"
	qwableCtxWindow    = 32768 // 保守声明（量化 GGUF 通常 32K）
	qwableMaxTokens    = 16384 // 编码场景（代码生成块）
	qwableMoAMaxTokens = 2000  // MoA 参考（RefMaxTokens 一致）
)

// QwableConfig — 适配器配置
type QwableConfig struct {
	Temperature  float64 // 编码场景（默认 0.35——确定性）
	MoATemp      float64 // MoA 参考场景（默认 0.7——多样性）
	Variant      string  // 量化变体: iq4_xs / q5_k_m
	MaxTokens    int     // 编码场景
	MoAMaxTokens int     // MoA 参考
	CtxWindow    int
	AuthToken    string
	BaseURL      string
}

// QwableAdapter — 实现 plugin.Plugin 接口
type QwableAdapter struct {
	config QwableConfig
}

var _ plugin.Plugin = (*QwableAdapter)(nil)

// NewQwableAdapter — 创建适配器（默认配置——双角色）
func NewQwableAdapter() *QwableAdapter {
	return &QwableAdapter{
		config: QwableConfig{
			Temperature:  0.35, // 编码确定性（CA 建议 0.3-0.4）
			MoATemp:      0.7,  // MoA 多样性（保持现状）
			Variant:      "q5_k_m",
			MaxTokens:    qwableMaxTokens,
			MoAMaxTokens: qwableMoAMaxTokens,
			CtxWindow:    qwableCtxWindow,
		},
	}
}

// Name — 插件名
func (a *QwableAdapter) Name() string { return qwableName }

// Type — 插件类型
func (a *QwableAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *QwableAdapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *QwableAdapter) Capabilities() []string {
	return []string{"model", "high-throughput", "coding", "moa-reference"}
}

// Init — 初始化配置
func (a *QwableAdapter) Init(cfg map[string]any) error {
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
	if t, ok := cfg["moa_temp"]; ok {
		switch v := t.(type) {
		case float64:
			a.config.MoATemp = v
		case int:
			a.config.MoATemp = float64(v)
		}
	}
	if v, ok := cfg["variant"].(string); ok {
		a.config.Variant = v
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
	if v, ok := cfg["auth_token"].(string); ok {
		a.config.AuthToken = v
	}
	if v, ok := cfg["base_url"].(string); ok {
		a.config.BaseURL = v
	}
	return nil
}

// Start — 启动（无内部进程）
func (a *QwableAdapter) Start() error { return nil }

// Stop — 停止
func (a *QwableAdapter) Stop() error { return nil }

// Close — 关闭
func (a *QwableAdapter) Close() error { return nil }

// Execute — 执行（元数据——双角色标记）
func (a *QwableAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("Qwable: 输入为空")
	}
	// 角色判断（MoA 参考 or 编码——按输入标记）
	role := "coding"
	if data, ok := input.Data.(map[string]any); ok {
		if r, ok := data["role"].(string); ok && r == "moa" {
			role = "moa"
		}
	}
	temp := a.config.Temperature
	maxTok := a.config.MaxTokens
	if role == "moa" {
		temp = a.config.MoATemp
		maxTok = a.config.MoAMaxTokens
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":        qwableName,
			"variant":      a.config.Variant,
			"role":         role,
			"temperature":  temp,
			"max_tokens":   maxTok,
			"ctx_window":   a.config.CtxWindow,
			"tool_calling": role == "coding", // 编码支持工具——MoA 参考无工具
		},
	}, nil
}

// GetDescriptions — 描述
func (a *QwableAdapter) GetDescriptions() string {
	return "Qwable-v1——MoE高吞吐——双角色(编码路由q5_k_m/MoA参考IQ4_XS)——编码temp0.35/MoA temp0.7"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (a *QwableAdapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":  a.config.Temperature,
		"MoATemp":      a.config.MoATemp,
		"Variant":      a.config.Variant,
		"MaxTokens":    a.config.MaxTokens,
		"MoAMaxTokens": a.config.MoAMaxTokens,
		"CtxWindow":    a.config.CtxWindow,
		"AuthToken":    a.config.AuthToken,
		"BaseURL":      a.config.BaseURL,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (a *QwableAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: a.config.Temperature, Desc: "编码场景温度", Min: 0, Max: 2},
		{Key: "moa_temp", Type: "number", Value: a.config.MoATemp, Desc: "MoA 参考温度", Min: 0, Max: 2},
		{Key: "variant", Type: "enum", Value: a.config.Variant, Desc: "量化变体", Options: []string{"iq4_xs", "q5_k_m"}},
		{Key: "max_tokens", Type: "number", Value: a.config.MaxTokens, Desc: "编码输出上限", Min: 1},
		{Key: "moa_max_tokens", Type: "number", Value: a.config.MoAMaxTokens, Desc: "MoA 参考输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: a.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "base_url", Type: "string", Value: a.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: a.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (a *QwableAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return a.Init(cfg)
}
