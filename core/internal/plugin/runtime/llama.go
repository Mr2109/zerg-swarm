// Package runtime 提供运行时插件——手机壳模式封装外部引擎。
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"zerg/core/internal/plugin"
)

// LlamaRuntime 实现 plugin.Plugin 接口
//
// 封装 llama-server 作为黑盒手机壳（v2.5.4.4 设计）:
//   - 不修改 llama 内部逻辑——只负责调用（OpenAI 兼容 API）
//   - 支持自动启动进程或连接已有 8081 端口
//   - 升级 llama-server 不影响虫族（手机壳模式）

const (
	// 默认 llama-server 地址/端口
	defaultHost = "127.0.0.1"
	defaultPort = "8081"
	// llama-server 启动等待超时
	defaultStartupTimeout = 30 * time.Second
	// HTTP 请求超时
	defaultRequestTimeout = 120 * time.Second
	// 健康检查间隔
	defaultHealthCheckInterval = 200 * time.Millisecond
	// 最大重试次数
	defaultMaxRetries = 3
)

// Llama API 请求/响应结构

// ChatMessage llama OpenAI 兼容 API 的消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest llama OpenAI 兼容 API 的请求体
type ChatCompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream    bool          `json:"stream,omitempty"`
}

// ChatCompletionResponse llama OpenAI 兼容 API 的响应体
type ChatCompletionResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []Choice          `json:"choices"`
	Usage   Usage             `json:"usage"`
}

// Choice 响应中的一个选择
type Choice struct {
	Index        int           `json:"index"`
	Message      ChatMessage   `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// Usage token 用量统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LlamaServerProcess 管理的 llama-server 进程

// LlamaServerProcess 管理一个 llama-server 子进程
type LlamaServerProcess struct {
	cmd    *exec.Cmd
	proc   *os.Process
	ready  bool
	stopCh chan struct{}
}

// IsRunning 检查进程是否存活
func (p *LlamaServerProcess) IsRunning() bool {
	if p.proc == nil {
		return false
	}
	err := p.proc.Signal(nil) // 发送信号 0 检查存活
	return err == nil
}

// Stop 停止进程
func (p *LlamaServerProcess) Stop() error {
	if p.proc == nil {
		return nil
	}
	close(p.stopCh)
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// LlamaRuntime 实现 plugin.Plugin 接口

// LlamaRuntime 实现 plugin.Plugin 接口
// 封装 llama-server 黑盒——只做调用——不改内部
type LlamaRuntime struct {
	name    string
	version string

	// 可配置参数（Init 后生效）
	Host              string
	Port              string
	ModelFile         string
	NGPULayers        int
	StartupTimeout    time.Duration
	RequestTimeout    time.Duration
	MaxRetries        int
	AutoStart         bool // 是否自动启动 llama-server
	ExecutablePath    string // llama-server 可执行文件路径

	// 内部状态
	baseURL    string
	httpClient *http.Client
	process    *LlamaServerProcess
	initialized bool
	started     bool

	// 验证用的公开字段（不导出——仅测试用）
	lastRequestURL string // 最后构造的完整请求 URL（测试验证用）
}

// NewLlamaRuntime 创建 LlamaRuntime（默认配置）
func NewLlamaRuntime() *LlamaRuntime {
	return &LlamaRuntime{
		name:             "llama",
		version:          "0.1.0",
		Host:             defaultHost,
		Port:             defaultPort,
		NGPULayers:       999,
		StartupTimeout:   defaultStartupTimeout,
		RequestTimeout:   defaultRequestTimeout,
		MaxRetries:       defaultMaxRetries,
		AutoStart:        false, // 默认不自动启动——可手动连接已有服务
		baseURL:          fmt.Sprintf("http://%s:%s", defaultHost, defaultPort),
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
		},
		process: &LlamaServerProcess{
			stopCh: make(chan struct{}),
		},
	}
}

// Name 插件唯一名称
func (l *LlamaRuntime) Name() string { return l.name }

// Type 插件类型
func (l *LlamaRuntime) Type() plugin.PluginType { return plugin.PluginTypeLlama }

// Version 插件版本（SemVer 格式）
func (l *LlamaRuntime) Version() string { return l.version }

// Capabilities 插件声明的能力列表
func (l *LlamaRuntime) Capabilities() []string {
	return []string{
		"llama-server-openai-compat",
		"llama-auto-start",
		"llama-gpu-ngl-999",
		"llama-black-box",
	}
}

// Init 初始化——接收配置
// cfg 支持:
//   - host: string（llama-server 地址，默认 "127.0.0.1"）
//   - port: string（llama-server 端口，默认 "8081"）
//   - model_file: string（模型文件路径）
//   - ngl: int（GPU 层数，默认 999——全放 GPU）
//   - auto_start: bool（是否自动启动 llama-server）
//   - executable: string（llama-server 可执行文件路径）
//   - startup_timeout: int（启动超时秒数）
//   - request_timeout: int（请求超时秒数）
func (l *LlamaRuntime) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	if v, ok := cfg["host"]; ok {
		if s, ok := v.(string); ok && s != "" {
			l.Host = s
		}
	}

	if v, ok := cfg["port"]; ok {
		switch p := v.(type) {
		case string:
			if p != "" {
				l.Port = p
			}
		case int:
			l.Port = fmt.Sprintf("%d", p)
		case int64:
			l.Port = fmt.Sprintf("%d", p)
		case float64:
			l.Port = fmt.Sprintf("%d", int(p))
		}
	}

	if v, ok := cfg["model_file"]; ok {
		if s, ok := v.(string); ok && s != "" {
			l.ModelFile = s
		}
	}

	if v, ok := cfg["ngl"]; ok {
		switch n := v.(type) {
		case int:
			l.NGPULayers = n
		case float64:
			l.NGPULayers = int(n)
		case int64:
			l.NGPULayers = int(n)
		default:
			return fmt.Errorf("llama: ngl must be number, got %T", v)
		}
	}

	if v, ok := cfg["auto_start"]; ok {
		if b, ok := v.(bool); ok {
			l.AutoStart = b
		}
	}

	if v, ok := cfg["executable"]; ok {
		if s, ok := v.(string); ok && s != "" {
			l.ExecutablePath = s
		}
	}

	if v, ok := cfg["startup_timeout"]; ok {
		switch t := v.(type) {
		case int:
			l.StartupTimeout = time.Duration(t) * time.Second
		case float64:
			l.StartupTimeout = time.Duration(t) * time.Second
		}
	}

	if v, ok := cfg["request_timeout"]; ok {
		switch t := v.(type) {
		case int:
			l.RequestTimeout = time.Duration(t) * time.Second
		case float64:
			l.RequestTimeout = time.Duration(t) * time.Second
		}
	}

	// 重建 baseURL
	l.baseURL = fmt.Sprintf("http://%s:%s", l.Host, l.Port)

	// 重建 httpClient
	l.httpClient = &http.Client{
		Timeout: l.RequestTimeout,
	}

	l.initialized = true
	return nil
}

// Start 启动插件
// 如果 AutoStart=true 且 llama-server 未运行——自动启动进程
// 否则——连接已有服务（检测 8081 端口是否可用）
func (l *LlamaRuntime) Start() error {
	if !l.initialized {
		return fmt.Errorf("llama: not initialized")
	}

	// 如果设置了可执行文件且未运行——启动
	if l.AutoStart && l.ExecutablePath != "" {
		if err := l.startServerProcess(); err != nil {
			return fmt.Errorf("llama: failed to start server: %w", err)
		}
	}

	// 健康检查——确认服务可用
	if err := l.waitForReady(); err != nil {
		return fmt.Errorf("llama: service not ready: %w", err)
	}

	l.started = true
	return nil
}

// startServerProcess 启动 llama-server 子进程
func (l *LlamaRuntime) startServerProcess() error {
	if l.ExecutablePath == "" {
		// 尝试从 PATH 查找
		l.ExecutablePath = "llama-server"
	}

	args := []string{
		"-m", l.ModelFile,
		"--port", l.Port,
		"--host", l.Host,
		"--ngl", fmt.Sprintf("%d", l.NGPULayers),
		"--ctx-size", "4096",
		"--threads", fmt.Sprintf("%d", 4),
		"--batch-size", "512",
	}

	l.process.cmd = exec.Command(l.ExecutablePath, args...)
	l.process.cmd.Stdout = os.Stdout
	l.process.cmd.Stderr = os.Stderr

	if err := l.process.cmd.Start(); err != nil {
		return fmt.Errorf("llama: exec start: %w", err)
	}

	l.process.proc = l.process.cmd.Process
	return nil
}

// waitForReady 等待服务就绪（HTTP 健康检查）
func (l *LlamaRuntime) waitForReady() error {
	timeout := time.After(l.StartupTimeout)
	ticker := time.NewTicker(defaultHealthCheckInterval)
	defer ticker.Stop()

	// 构造 /health 或 /v1/models 端点
	healthURL := fmt.Sprintf("%s/v1/models", l.baseURL)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("llama: startup timeout (%v)", l.StartupTimeout)
		case <-ticker.C:
			if l.process.IsRunning() {
				resp, err := l.httpClient.Get(healthURL)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return nil
					}
				}
			}
		}
	}
}

// Stop 停止插件（优雅停止）
func (l *LlamaRuntime) Stop() error {
	l.started = false
	if l.process.IsRunning() {
		return l.process.Stop()
	}
	return nil
}

// Close 关闭插件——释放所有资源（不可逆）
func (l *LlamaRuntime) Close() error {
	l.Stop()
	l.initialized = false
	l.process = nil
	return nil
}

// Execute 执行任务——转发请求到 llama-server OpenAI 兼容 API
// input.Data 期望是 map[string]interface{}，包含:
//   - prompt: string（提示词）
//   - model: string（可选，覆盖默认模型）
//   - temperature: float64（可选）
//   - max_tokens: int（可选）
//
// 返回 PluginOutput:
//   - Result: map[string]interface{}（含 response, usage 等）
//   - Meta: map[string]interface{}（含 request_url 等）
func (l *LlamaRuntime) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !l.started {
		return plugin.PluginOutput{}, fmt.Errorf("llama: not started")
	}

	// 从 input.Context 提取参数
	prompt, _ := input.Context["prompt"].(string)
	if prompt == "" {
		return plugin.PluginOutput{}, fmt.Errorf("llama: prompt is empty")
	}

	model := "llama"
	if m, ok := input.Context["model"].(string); ok && m != "" {
		model = m
	}

	temperature := 0.7
	if t, ok := input.Context["temperature"].(float64); ok {
		temperature = t
	}

	maxTokens := 2048
	if mt, ok := input.Context["max_tokens"].(int); ok {
		maxTokens = mt
	}

	// 构造请求体
	reqBody := ChatCompletionRequest{
		Model:       model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	// 构造请求 URL——/v1/chat/completions
	requestURL := fmt.Sprintf("%s/v1/chat/completions", l.baseURL)
	l.lastRequestURL = requestURL // 保存用于测试验证

	// 序列化请求体
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("llama: marshal request: %w", err)
	}

	// 发送 HTTP 请求
	ctx, cancel := context.WithTimeout(context.Background(), l.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("llama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 带重试的发送
	var respBody []byte
	for attempt := 1; attempt <= l.MaxRetries; attempt++ {
		resp, err := l.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				respBody, err = io.ReadAll(resp.Body)
				if err != nil {
					return plugin.PluginOutput{}, fmt.Errorf("llama: read response: %w", err)
				}
				break
			}
			// 非 200 状态码
			bodyBytes, _ := io.ReadAll(resp.Body)
			return plugin.PluginOutput{}, fmt.Errorf(
				"llama: server returned %d: %s", resp.StatusCode, string(bodyBytes))
		}
		if attempt < l.MaxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if respBody == nil {
		return plugin.PluginOutput{}, fmt.Errorf("llama: request failed after %d retries", l.MaxRetries)
	}

	// 解析响应
	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("llama: unmarshal response: %w", err)
	}

	// 构建输出
	var reply string
	if len(apiResp.Choices) > 0 {
		reply = apiResp.Choices[0].Message.Content
	}

	result := map[string]interface{}{
		"response":     reply,
		"model":        apiResp.Model,
		"finish_reason": func() string {
			if len(apiResp.Choices) > 0 {
				return apiResp.Choices[0].FinishReason
			}
			return ""
		}(),
		"usage": map[string]int{
			"prompt_tokens":     apiResp.Usage.PromptTokens,
			"completion_tokens": apiResp.Usage.CompletionTokens,
			"total_tokens":      apiResp.Usage.TotalTokens,
		},
	}

	return plugin.PluginOutput{
		Result: result,
		Meta: map[string]interface{}{
			"request_url":    requestURL,
			"model_file":     l.ModelFile,
			"ngl":            l.NGPULayers,
			"auto_start":     l.AutoStart,
			"request_method": "POST",
			"api_format":     "openai-chat-completions",
			"black_box":      true, // 黑盒模式标记
		},
	}, nil
}

// BuildRequestURL 返回 Execute 会构造的请求 URL（不发送）
// 用于测试验证 URL 构造正确性
func (l *LlamaRuntime) BuildRequestURL() string {
	return fmt.Sprintf("%s/v1/chat/completions", l.baseURL)
}

// BuildRequestBody 构造请求体 JSON（不发送）
// 用于测试验证请求体构造正确性
func (l *LlamaRuntime) BuildRequestBody(prompt string, model string, temperature float64, maxTokens int) ([]byte, error) {
	reqBody := ChatCompletionRequest{
		Model:       model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
	return json.Marshal(reqBody)
}

// BaseURL 返回当前 base URL（测试验证用）
func (l *LlamaRuntime) BaseURL() string {
	return l.baseURL
}

// IsStarted 返回是否已启动（测试验证用）
func (l *LlamaRuntime) IsStarted() bool {
	return l.started
}

// IsInitialized 返回是否已初始化（测试验证用）
func (l *LlamaRuntime) IsInitialized() bool {
	return l.initialized
}

// BuildBaseURL 返回当前 base URL（公开方法）
func (l *LlamaRuntime) BuildBaseURL() string {
	return l.baseURL
}

// 工具函数

// ValidateModelFile 验证模型文件是否存在
func ValidateModelFile(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// FormatBaseURL 格式化 base URL（去除尾部斜杠）
func FormatBaseURL(host, port string) string {
	base := fmt.Sprintf("http://%s:%s", host, port)
	return strings.TrimRight(base, "/")
}

// 验证 LlamaRuntime 实现 Plugin 接口（编译期检查）
var _ plugin.Plugin = (*LlamaRuntime)(nil)
