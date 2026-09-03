package adapters

// GemmaAdapter — gemma-4 家族模型适配器（v2.5.4.9——2026-08-17 注册）
// 特性: 双角色（MoA 参考 26B + LLMJudge 12B）——MoE——无工具纯文本分析
// 优化: CA 调研——MoA temp 0.7（多样性）/ Judge temp 0.2（评判稳定）——size 参数区分
// 注意: 家族 12B/26B 两档——size 参数（12B 做 Judge 快——26B 做参考准）

import (
	"fmt"

	"zerg/core/internal/plugin"
)

const (
	gemmaName      = "gemma-4"
	gemmaCtxWindow = 32768
)

// GemmaConfig — 适配器配置
type GemmaConfig struct {
	Temperature  float64 // MoA 参考（默认 0.7）
	JudgeTemp    float64 // Judge 场景（默认 0.2——评判稳定）
	Size         string  // 家族档位: 12B / 26B
	MaxTokens    int     // MoA 参考（2000）
	JudgeTokens  int     // Judge（4096）
	CtxWindow    int
	AuthToken    string
	BaseURL      string
}

// GemmaAdapter — 实现 plugin.Plugin 接口
type GemmaAdapter struct {
	config GemmaConfig
}

var _ plugin.Plugin = (*GemmaAdapter)(nil)

// NewGemmaAdapter — 创建适配器（默认配置——双角色）
func NewGemmaAdapter() *GemmaAdapter {
	return &GemmaAdapter{
		config: GemmaConfig{
			Temperature: 0.7,  // MoA 多样性
			JudgeTemp:   0.2,  // Judge 稳定
			Size:        "26B", // 默认 MoA 参考
			MaxTokens:   2000, // MoA RefMaxTokens 一致
			JudgeTokens: 4096, // Judge 输出短但完整
			CtxWindow:   gemmaCtxWindow,
		},
	}
}

// Name — 插件名
func (a *GemmaAdapter) Name() string { return gemmaName }

// Type — 插件类型
func (a *GemmaAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version — 版本
func (a *GemmaAdapter) Version() string { return "1.0.0" }

// Capabilities — 能力清单
func (a *GemmaAdapter) Capabilities() []string {
	return []string{"model", "analysis", "judge", "moa-reference"}
}

// Init — 初始化配置
func (a *GemmaAdapter) Init(cfg map[string]any) error {
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
	if t, ok := cfg["judge_temp"]; ok {
		switch v := t.(type) {
		case float64:
			a.config.JudgeTemp = v
		case int:
			a.config.JudgeTemp = float64(v)
		}
	}
	if v, ok := cfg["size"].(string); ok {
		a.config.Size = v
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
func (a *GemmaAdapter) Start() error { return nil }

// Stop — 停止
func (a *GemmaAdapter) Stop() error { return nil }

// Close — 关闭
func (a *GemmaAdapter) Close() error { return nil }

// Execute — 执行（元数据——双角色标记）
func (a *GemmaAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if input.Data == nil {
		return plugin.PluginOutput{}, fmt.Errorf("gemma: 输入为空")
	}
	role := "moa"
	if data, ok := input.Data.(map[string]any); ok {
		if r, ok := data["role"].(string); ok && r == "judge" {
			role = "judge"
		}
	}
	temp := a.config.Temperature
	maxTok := a.config.MaxTokens
	if role == "judge" {
		temp = a.config.JudgeTemp
		maxTok = a.config.JudgeTokens
	}
	return plugin.PluginOutput{
		Result: map[string]any{
			"model":       gemmaName,
			"size":        a.config.Size,
			"role":        role,
			"temperature": temp,
			"max_tokens":  maxTok,
			"ctx_window":  a.config.CtxWindow,
			"tool_calling": false, // 无工具纯文本分析
			"judge_capable": true, // Judge 能力
		},
	}, nil
}

// GetDescriptions — 描述
func (a *GemmaAdapter) GetDescriptions() string {
	return "gemma-4——MoE——双角色(MoA参考26B/Judge12B)——temp0.7/0.2——无工具纯文本"
}

// Options 适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
func (g *GemmaAdapter) Options() map[string]interface{} {
	return map[string]interface{}{
		"Temperature": g.config.Temperature,
		"JudgeTemp":   g.config.JudgeTemp,
		"Size":        g.config.Size,
		"MaxTokens":   g.config.MaxTokens,
		"JudgeTokens": g.config.JudgeTokens,
		"CtxWindow":   g.config.CtxWindow,
		"AuthToken":   g.config.AuthToken,
		"BaseURL":     g.config.BaseURL,
	}
}

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (g *GemmaAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: g.config.Temperature, Desc: "MoA 参考温度", Min: 0, Max: 2},
		{Key: "judge_temp", Type: "number", Value: g.config.JudgeTemp, Desc: "Judge 评判温度", Min: 0, Max: 2},
		{Key: "size", Type: "enum", Value: g.config.Size, Desc: "家族档位", Options: []string{"12B", "26B"}},
		{Key: "max_tokens", Type: "number", Value: g.config.MaxTokens, Desc: "MoA 参考输出上限", Min: 1},
		{Key: "judge_tokens", Type: "number", Value: g.config.JudgeTokens, Desc: "Judge 输出上限", Min: 1},
		{Key: "ctx_window", Type: "number", Value: g.config.CtxWindow, Desc: "上下文窗口", Min: 1024},
		{Key: "base_url", Type: "string", Value: g.config.BaseURL, Desc: "模型服务地址"},
		{Key: "auth_token", Type: "string", Value: g.config.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (g *GemmaAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return g.Init(cfg)
}
