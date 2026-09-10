// Package adapters 提供模型适配器插件实现。
package adapters

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// Ds4Adapter 实现 plugin.Plugin 接口
//
// 封装 ds4 模型（deepseek-v4-flash）的散落假设（设计-v2.5.4.3）:
//   - Temperature 默认 0.6（deepseek 推荐值）
//   - OpenAI 兼容 API 格式（/v1/chat/completions）
//   - 零外部依赖；Init 可覆盖默认值；纯函数式检测（可独立测试）
// 状态：定稿——待测试验证

// Ds4Adapter 实现 plugin.Plugin 接口
type Ds4Adapter struct {
	name    string
	version string
	// 可配置参数（Init 后生效）
	Temperature float64
	AuthToken   string
	FinishWords []string
	GatewayURL  string
	APIFormat   string // "openai" 或 "responses"
	// 状态
	initialized bool
	started     bool
}

// NewDs4Adapter 创建 Ds4Adapter（默认配置）
func NewDs4Adapter() *Ds4Adapter {
	return &Ds4Adapter{
		name:        "ds4",
		version:     "0.1.0",
		Temperature: 0.6,
		AuthToken:   "ds4-gateway-token",
		FinishWords: []string{"任务完成", "已完成", "成功完成", "完成", "done", "completed"},
		GatewayURL:  statepath.GatewayBaseURL(),
		APIFormat:   "openai",
	}
}

// Name 插件唯一名称
func (d *Ds4Adapter) Name() string { return d.name }

// Type 插件类型
func (d *Ds4Adapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version 插件版本（SemVer 格式）
func (d *Ds4Adapter) Version() string { return d.version }

// Capabilities 插件声明的能力列表
func (d *Ds4Adapter) Capabilities() []string {
	return []string{
		"ds4-temperature-0.6",
		"ds4-auth-token-gateway",
		"ds4-finish-words",
		"ds4-openai-api",
		"deepseek-v4-flash",
	}
}

// Init 初始化——接收配置
// cfg 支持:
//   - temperature: float64（覆盖默认 0.6）
//   - auth_token: string（覆盖默认网关 token）
//   - finish_words: []string（覆盖默认完成词表）
//   - gateway_url: string（覆盖默认网关）
//   - api_format: string（覆盖默认 "openai"）
func (d *Ds4Adapter) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{} // 无配置用默认
	}

	if v, ok := cfg["temperature"]; ok {
		switch t := v.(type) {
		case float64:
			d.Temperature = t
		case float32:
			d.Temperature = float64(t)
		case int:
			d.Temperature = float64(t)
		default:
			return fmt.Errorf("ds4: temperature must be number, got %T", v)
		}
	}

	if v, ok := cfg["auth_token"]; ok {
		if s, ok := v.(string); ok {
			d.AuthToken = s
		} else {
			return fmt.Errorf("ds4: auth_token must be string")
		}
	}

	if v, ok := cfg["finish_words"]; ok {
		if words, ok := v.([]string); ok && len(words) > 0 {
			d.FinishWords = words
		}
	}

	if v, ok := cfg["gateway_url"]; ok {
		if s, ok := v.(string); ok {
			d.GatewayURL = s
		}
	}

	if v, ok := cfg["api_format"]; ok {
		if s, ok := v.(string); ok {
			d.APIFormat = s
		}
	}

	d.initialized = true
	return nil
}

// Start 启动插件（非阻塞，等待就绪）
func (d *Ds4Adapter) Start() error {
	if !d.initialized {
		return fmt.Errorf("ds4: not initialized")
	}
	d.started = true
	return nil
}

// Stop 停止插件（优雅停止——等待当前任务完成）
func (d *Ds4Adapter) Stop() error {
	d.started = false
	return nil
}

// Close 关闭插件——释放所有资源（不可逆）
func (d *Ds4Adapter) Close() error {
	d.initialized = false
	d.started = false
	return nil
}

// Execute 执行任务——传入输入，返回输出。
func (d *Ds4Adapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !d.initialized {
		return plugin.PluginOutput{}, fmt.Errorf("ds4: not initialized")
	}

	prompt := ""
	if data, ok := input.Data.(string); ok {
		prompt = data
	}

	response := map[string]interface{}{
		"model":       "deepseek-v4-flash",
		"temperature": d.Temperature,
		"auth_token":  d.AuthToken,
		"api_format":  d.APIFormat,
		"gateway_url": d.GatewayURL,
		// v2.5.5 T5c 元数据风格（与 qwen38/nemotron 一致——B-3 路由参数覆盖用）
		"machine":     "",
		"format":      "chat",
		"load_mode":   "full-memory", // 2026-08-24 Mr2109稿: 全驻留512K（实测8字/秒——比SSD 1M快4-5倍——配合让位清场DS4独占——余量10G够）
		"timeout_sec": 300,           // 全驻留: 加载22秒+推理余量——300s；SSD才120s
	}

	// 检测完成词
	if detected, words := ContainsFinishWords(prompt, d.FinishWords); detected {
		response["finish_words_detected"] = true
		response["matched_words"] = words
	} else {
		response["finish_words_detected"] = false
	}

	return plugin.PluginOutput{Result: response}, nil
}

// 验证 Ds4Adapter 实现 Plugin 接口（编译期检查）
// 如果 Ds4Adapter 没有完全实现 Plugin 接口，编译会报错
var _ plugin.Plugin = (*Ds4Adapter)(nil)

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同）
func (d *Ds4Adapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: d.Temperature, Desc: "采样温度", Min: 0, Max: 2},
		{Key: "api_format", Type: "enum", Value: d.APIFormat, Desc: "接口格式", Options: []string{"openai", "responses"}},
		{Key: "gateway_url", Type: "string", Value: d.GatewayURL, Desc: "网关地址"},
		{Key: "finish_words", Type: "array", Value: d.FinishWords, Desc: "完成语义词表（逗号分隔）"},
		{Key: "auth_token", Type: "string", Value: d.AuthToken, Desc: "认证 token（掩码显示）"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (d *Ds4Adapter) UpdateOptions(cfg map[string]interface{}) error {
	return d.Init(cfg)
}
