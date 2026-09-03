package adapters

import (
	"fmt"
	"time"

	"zerg/core/internal/plugin"
)

// MCPClient 接口（解耦——适配器不依赖具体 MCPManager）
//
// MCPManager 在 agent 包，适配器在 plugin 包。
// 通过接口解耦——测试时用 mock，生产时用真实 MCPManager。
type MCPClient interface {
	Connect(name string, command string, args ...string) error
	ConnectHTTP(name string, baseURL string, headers map[string]string) error
	Call(serverName, toolName string, args map[string]interface{}) (string, error)
	Close()
	ToolDefs(extended bool) []MCPToolDef
}

// MCPToolDef 简化的工具定义（解耦——不依赖 agent.ToolDef）
type MCPToolDef struct {
	Type       string                 `json:"type"`
	Function   map[string]interface{} `json:"function"`
}

// McpPlugin 实现 plugin.Plugin 接口
// 包装现有 MCPManager——Execute 调用 MCP 工具
//
// 设计要点：
//   - 零外部依赖（只用到 Go 标准库）
//   - 通过 MCPClient 接口解耦——不直接依赖 agent.MCPManager
//   - 可配置（Init 可覆盖默认值）
//
// 来自设计-v2.5.4.5 mcp 插件化
type McpPlugin struct {
	name    string
	version string
	// 内部 MCPManager（通过接口解耦）
	client MCPClient
	// 状态
	initialized bool
	started     bool
}

// NewMcpPlugin 创建 McpPlugin（包装 MCPManager）
func NewMcpPlugin(client MCPClient) *McpPlugin {
	return &McpPlugin{
		name:    "mcp",
		version: "0.1.0",
		client:  client,
	}
}

// Name 插件唯一名称
func (m *McpPlugin) Name() string { return m.name }

// Type 插件类型
func (m *McpPlugin) Type() plugin.PluginType { return plugin.PluginTypeMCP }

// Version 插件版本（SemVer 格式）
func (m *McpPlugin) Version() string { return m.version }

// Capabilities 插件声明的能力列表
func (m *McpPlugin) Capabilities() []string {
	return []string{
		"mcp-connect",
		"mcp-call",
		"mcp-tools-list",
	}
}

// Init 初始化——接收配置
// cfg 支持:
//   - client: MCPClient（必须，注入 MCPManager）
//   - extended: bool（是否默认使用扩展工具列表）
func (m *McpPlugin) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	// client 已在构造时注入，Init 只做标记
	m.initialized = true
	return nil
}

// Start 启动插件
func (m *McpPlugin) Start() error {
	if !m.initialized {
		return fmt.Errorf("mcp plugin: not initialized")
	}
	m.started = true
	return nil
}

// Stop 停止插件
func (m *McpPlugin) Stop() error {
	m.started = false
	return nil
}

// Close 关闭插件——释放所有资源
func (m *McpPlugin) Close() error {
	if m.client != nil {
		m.client.Close()
	}
	m.initialized = false
	m.started = false
	return nil
}

// McpExecuteInput Execute 的输入 Data 类型
type McpExecuteInput struct {
	ServerName string                 // MCP 服务器名
	ToolName   string                 // MCP 工具名
	Args       map[string]interface{} // 工具参数
}

// McpExecuteOutput Execute 的输出 Result 类型
type McpExecuteOutput struct {
	ServerName string
	ToolName   string
	Result     string
	ExecutedAt string
}

// Execute 执行任务——调用 MCP 工具
// input.Data 期望是 McpExecuteInput（含 ServerName, ToolName, Args）
// 返回 PluginOutput:
//   - Result: McpExecuteOutput（含调用结果）
func (m *McpPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !m.started {
		return plugin.PluginOutput{}, fmt.Errorf("mcp plugin: not started")
	}
	if m.client == nil {
		return plugin.PluginOutput{}, fmt.Errorf("mcp plugin: client is nil")
	}

	var input_data McpExecuteInput
	switch v := input.Data.(type) {
	case McpExecuteInput:
		input_data = v
	case map[string]interface{}:
		if sn, ok := v["server_name"].(string); ok {
			input_data.ServerName = sn
		}
		if tn, ok := v["tool_name"].(string); ok {
			input_data.ToolName = tn
		}
		if args, ok := v["args"].(map[string]interface{}); ok {
			input_data.Args = args
		}
	default:
		return plugin.PluginOutput{}, fmt.Errorf("mcp plugin: invalid input type %T", input.Data)
	}

	if input_data.ServerName == "" {
		return plugin.PluginOutput{}, fmt.Errorf("mcp plugin: server_name is empty")
	}
	if input_data.ToolName == "" {
		return plugin.PluginOutput{}, fmt.Errorf("mcp plugin: tool_name is empty")
	}

	// 调用 MCP 工具
	result, err := m.client.Call(input_data.ServerName, input_data.ToolName, input_data.Args)
	if err != nil {
		return plugin.PluginOutput{
			Meta: map[string]interface{}{
				"error_type": "mcp_call_failed",
			},
		}, err
	}

	return plugin.PluginOutput{
		Result: McpExecuteOutput{
			ServerName: input_data.ServerName,
			ToolName:   input_data.ToolName,
			Result:     result,
			ExecutedAt: time.Now().Format(time.RFC3339),
		},
		Meta: map[string]interface{}{
			"result_length": len(result),
		},
	}, nil
}

// 验证 McpPlugin 实现 Plugin 接口（编译期检查）
var _ plugin.Plugin = (*McpPlugin)(nil)

// ListTools 列出所有可用 MCP 工具（便捷方法）
func (m *McpPlugin) ListTools(extended bool) []MCPToolDef {
	if m.client == nil {
		return nil
	}
	return m.client.ToolDefs(extended)
}
