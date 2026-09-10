// Package adapters 提供模型适配器插件实现。
package adapters

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// OrnithAdapter 实现 plugin.Plugin 接口
//
// 封装 example-35b-v2 模型的散落假设（设计-v2.5.4.2）:
//   - Temperature 默认 0.8（实测 0.3 更差）
//   - Responses API 格式（/v1/responses 网关路径，顶层 tools 格式）
//   - 零外部依赖；Init 可覆盖默认值；纯函数式检测（可独立测试）
// 状态：定稿——待测试验证

// OrnithAdapter 实现 plugin.Plugin 接口
type OrnithAdapter struct {
	name    string
	version string
	// 可配置参数（Init 后生效）
	Temperature float64
	TopP        float64
	TopK        int
	MinP        float64
	MaxTokens   int
	TimeoutSec  int
	Format      string // chat / responses——2026-08-27 提为可配置（原硬编码 chat）
	Thinking    bool   // 思考开关——example-35b-v2 思考占 max_tokens（1.5 尤其明显）
	AuthToken   string
	// 完成语义词表（example-35b-v2 散落的完成词检测）
	FinishWords []string
	// Responses API 配置
	GatewayURL      string
	UseResponsesAPI bool
	// 状态
	initialized bool
	started     bool
}

// NewOrnithAdapter 创建 OrnithAdapter（默认配置）
func NewOrnithAdapter() *OrnithAdapter {
	return &OrnithAdapter{
		name:            "example-35b-v2",
		version:         "0.1.0",
		Temperature:     0.8,
		TopP:            0.95,
		TopK:            40,
		MinP:            0.0,
		MaxTokens:       32768,
		TimeoutSec:      120,
		Format:          "chat",
		Thinking:        true,
		AuthToken:       "example-35b",
		FinishWords:     DefaultFinishWords(),
		GatewayURL:      statepath.GatewayBaseURL(),
		UseResponsesAPI: true,
	}
}

// Name 插件唯一名称
func (o *OrnithAdapter) Name() string { return o.name }

// Type 插件类型
func (o *OrnithAdapter) Type() plugin.PluginType { return plugin.PluginTypeModelAdapter }

// Version 插件版本（SemVer 格式）
func (o *OrnithAdapter) Version() string { return o.version }

// Capabilities 插件声明的能力列表
func (o *OrnithAdapter) Capabilities() []string {
	return []string{
		"example-35b-v2-temperature-0.8",
		"example-35b-v2-auth-token",
		"example-35b-v2-finish-words",
		"example-35b-v2-responses-api",
	}
}

// Init 初始化——接收配置
// cfg 支持:
//   - temperature: float64（覆盖默认 0.8）
//   - auth_token: string（覆盖默认 "example-35b"）
//   - finish_words: []string（覆盖默认完成词表）
//   - gateway_url: string（覆盖默认网关）
//   - use_responses_api: bool（覆盖默认 true）
func (o *OrnithAdapter) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{} // 无配置用默认
	}

	if v, ok := cfg["temperature"]; ok {
		switch t := v.(type) {
		case float64:
			o.Temperature = t
		case float32:
			o.Temperature = float64(t)
		case int:
			o.Temperature = float64(t)
		default:
			return fmt.Errorf("example-35b-v2: temperature must be number, got %T", v)
		}
	}

	if v, ok := cfg["top_p"]; ok {
		switch t := v.(type) {
		case float64:
			o.TopP = t
		case float32:
			o.TopP = float64(t)
		case int:
			o.TopP = float64(t)
		default:
			return fmt.Errorf("example-35b-v2: top_p must be number, got %T", v)
		}
	}

	if v, ok := cfg["top_k"]; ok {
		switch t := v.(type) {
		case float64:
			o.TopK = int(t)
		case int:
			o.TopK = t
		default:
			return fmt.Errorf("example-35b-v2: top_k must be number, got %T", v)
		}
	}

	if v, ok := cfg["min_p"]; ok {
		switch t := v.(type) {
		case float64:
			o.MinP = t
		case float32:
			o.MinP = float64(t)
		case int:
			o.MinP = float64(t)
		default:
			return fmt.Errorf("example-35b-v2: min_p must be number, got %T", v)
		}
	}

	if v, ok := cfg["max_tokens"]; ok {
		switch t := v.(type) {
		case float64:
			o.MaxTokens = int(t)
		case int:
			o.MaxTokens = t
		default:
			return fmt.Errorf("example-35b-v2: max_tokens must be number, got %T", v)
		}
	}

	if v, ok := cfg["timeout_sec"]; ok {
		switch t := v.(type) {
		case float64:
			o.TimeoutSec = int(t)
		case int:
			o.TimeoutSec = t
		default:
			return fmt.Errorf("example-35b-v2: timeout_sec must be number, got %T", v)
		}
	}

	if v, ok := cfg["format"]; ok {
		if s, ok := v.(string); ok && s != "" {
			o.Format = s
		}
	}

	if v, ok := cfg["thinking"]; ok {
		if b, ok := v.(bool); ok {
			o.Thinking = b
		}
	}

	if v, ok := cfg["auth_token"]; ok {
		if s, ok := v.(string); ok {
			o.AuthToken = s
		} else {
			return fmt.Errorf("example-35b-v2: auth_token must be string")
		}
	}

	if v, ok := cfg["finish_words"]; ok {
		if words, ok := v.([]string); ok && len(words) > 0 {
			o.FinishWords = words
		}
	}

	if v, ok := cfg["gateway_url"]; ok {
		if s, ok := v.(string); ok {
			o.GatewayURL = s
		}
	}

	if v, ok := cfg["use_responses_api"]; ok {
		if b, ok := v.(bool); ok {
			o.UseResponsesAPI = b
		}
	}

	o.initialized = true
	return nil
}

// Start 启动插件（非阻塞，等待就绪）
func (o *OrnithAdapter) Start() error {
	if !o.initialized {
		return fmt.Errorf("example-35b-v2: not initialized")
	}
	o.started = true
	return nil
}

// Stop 停止插件（优雅停止——等待当前任务完成）
func (o *OrnithAdapter) Stop() error {
	o.started = false
	return nil
}

// Close 关闭插件——释放所有资源（不可逆）
func (o *OrnithAdapter) Close() error {
	o.initialized = false
	o.started = false
	return nil
}

// Execute 执行任务——传入输入，返回输出
// input.Data 期望是 map[string]interface{}，包含:
//   - prompt: string（提示词）
//   - temperature: float64（可选，覆盖默认）
//   - auth_token: string（可选，覆盖默认）
//
// 返回 PluginOutput:
//   - Result: map[string]interface{}（响应数据）
//   - Meta: map[string]interface{}（含 finish_words_detected, gateway_url 等）
func (o *OrnithAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !o.started {
		return plugin.PluginOutput{}, fmt.Errorf("example-35b-v2: not started")
	}

	prompt, _ := input.Context["prompt"].(string)
	if prompt == "" {
		return plugin.PluginOutput{}, fmt.Errorf("example-35b-v2: prompt is empty")
	}

	// 构建响应（模拟——实际应调网关）
	response := map[string]interface{}{
		"prompt_length":  len(prompt),
		"gateway_url":    o.GatewayURL,
		"temperature":    o.Temperature,
		"auth_token_set": o.AuthToken != "",
		"responses_api":  o.UseResponsesAPI,
		// v2.5.5 T5c 元数据风格（与 qwen38/nemotron 一致——B-3 路由参数覆盖用）
		"machine":     "",
		"format":      o.Format,
		"max_tokens":  o.MaxTokens,
		"timeout_sec": o.TimeoutSec,
		"top_p":       o.TopP,
		"top_k":       o.TopK,
		"min_p":       o.MinP,
		"thinking":    o.Thinking,
	}

	// 检测完成词（example-35b-v2 散落假设的核心）
	if detected, words := ContainsFinishWords(prompt, o.FinishWords); detected {
		response["finish_words_detected"] = true
		response["matched_words"] = words
	} else {
		response["finish_words_detected"] = false
	}

	return plugin.PluginOutput{Result: response}, nil
}

// 完成语义词表（example-35b-v2 散落的完成词检测——从 loop.go 搬入）

// DefaultFinishWords 返回 example-35b-v2 默认完成语义词表
// 来自 loop.go:584 containsDoneWords()——散落在主控的假设收进插件
func DefaultFinishWords() []string {
	return []string{
		"任务完成",
		"已完成",
		"完成成功",
		"已创建",
		"已写入",
		"已修改",
		"已修复",
		"成功完成",
		"done",
		"completed",
		"Task complete",
	}
}

// ContainsFinishWords 检测文本是否包含完成语义词
// 参数:
//   - text: 要检测的文本
//   - words: 完成语义词表（nil 时用默认）
//
// 返回:
//   - detected: 是否检测到完成词
//   - matched: 匹配到的词列表
//
// 来自 loop.go:584 containsDoneWords()——散落在主控的假设收进插件
func ContainsFinishWords(text string, words []string) (bool, []string) {
	if len(words) == 0 {
		words = DefaultFinishWords()
	}

	textLower := strings.ToLower(text)
	var matched []string
	for _, w := range words {
		if strings.Contains(textLower, strings.ToLower(w)) {
			matched = append(matched, w)
		}
	}
	return len(matched) > 0, matched
}

// 验证 OrnithAdapter 实现 Plugin 接口（编译期检查）
// 如果 OrnithAdapter 没有完全实现 Plugin 接口，编译会报错
var _ plugin.Plugin = (*OrnithAdapter)(nil)

// OptionSchema 声明本适配器可编辑参数（Mr2109 2026-08-27——每个模型各自不同——调用该模型的所有参数都列出+说明）
func (o *OrnithAdapter) OptionSchema() []plugin.OptionDef {
	return []plugin.OptionDef{
		{Key: "temperature", Type: "number", Value: o.Temperature, Desc: "采样温度——越高回答越随机（0.8 实测最佳，0.3 更差）", Min: 0, Max: 2},
		{Key: "top_p", Type: "number", Value: o.TopP, Desc: "核采样——只从累计概率 top_p 的 token 中采样（0.95 默认宽松）", Min: 0, Max: 1},
		{Key: "top_k", Type: "number", Value: o.TopK, Desc: "Top-K——只从概率最高的 K 个 token 中采样（40 默认）", Min: 0, Max: 100},
		{Key: "min_p", Type: "number", Value: o.MinP, Desc: "Min-P——过滤概率低于最高概率 ×min_p 的 token（0 关闭）", Min: 0, Max: 1},
		{Key: "max_tokens", Type: "number", Value: o.MaxTokens, Desc: "输出上限——example-35b-v2-1.5 思考占 token，小了 reasoning 挤掉内容/tool_calls（32768 装 reasoning+content）", Min: 256, Max: 131072},
		{Key: "timeout_sec", Type: "number", Value: o.TimeoutSec, Desc: "请求超时（秒）——example-35b-v2 推理慢（31tok/s）留 120s 余量", Min: 10, Max: 600},
		{Key: "format", Type: "enum", Value: o.Format, Desc: "请求格式——chat=聊天补全 / responses=新版 API", Options: []string{"chat", "responses"}},
		{Key: "thinking", Type: "bool", Value: o.Thinking, Desc: "思考开关——example-35b-v2 思考占 max_tokens（1.5 尤其明显——占满导致 content 空/tool_calls 丢）"},
		{Key: "use_responses_api", Type: "bool", Value: o.UseResponsesAPI, Desc: "走 responses API 格式（网关 /v1/responses 路径）"},
		{Key: "gateway_url", Type: "string", Value: o.GatewayURL, Desc: "网关地址——example-35b-v2 模型服务所在（http://127.0.0.1:8082）"},
		{Key: "finish_words", Type: "array", Value: o.FinishWords, Desc: "完成语义词表（逗号分隔）——检测模型回复是否含完成标记"},
		{Key: "auth_token", Type: "string", Value: o.AuthToken, Desc: "认证 token（掩码显示）——网关 Bearer"},
	}
}

// UpdateOptions 运行时更新配置（Init 覆盖式——实时生效不重启）
func (o *OrnithAdapter) UpdateOptions(cfg map[string]interface{}) error {
	return o.Init(cfg)
}
