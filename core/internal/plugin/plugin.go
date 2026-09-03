// Package plugin 提供虫族插件体系的基础接口和类型定义。
// 严格依照《设计-v2.5.4-插件体系-定稿》第三节（插件接口）定义。
package plugin

// PluginType 插件类型标识。
type PluginType string

// 预定义的插件类型常量（来自设计定稿）。
const (
	PluginTypeSkill          PluginType = "skill"
	PluginTypeMCP            PluginType = "mcp"
	PluginTypeCA             PluginType = "ca"
	PluginTypeLlama          PluginType = "llama"
	PluginTypeModelAdapter   PluginType = "model-adapter"
	PluginTypeTool           PluginType = "tool"
	PluginTypeCluster        PluginType = "cluster"
	PluginTypeStore          PluginType = "store"
)

// Plugin 插件接口——所有插件必须实现。
// 来自设计定稿第三节。
type Plugin interface {
	// Name 插件唯一名称。
	Name() string
	// Type 插件类型。
	Type() PluginType
	// Version 插件版本（SemVer 格式）。
	Version() string
	// Capabilities 插件声明的能力列表。
	Capabilities() []string
	// Init 初始化——接收配置。
	Init(cfg map[string]interface{}) error
	// Start 启动插件（非阻塞，等待就绪）。
	Start() error
	// Stop 停止插件（优雅停止——等待当前任务完成）。
	Stop() error
	// Close 关闭插件——释放所有资源（不可逆）。
	Close() error
	// Execute 执行任务——传入输入，返回输出。
	Execute(input PluginInput) (PluginOutput, error)
}

// PluginInput 插件输入。
type PluginInput struct {
	// TaskID 任务唯一标识。
	TaskID string
	// Data 任务数据（任意类型）。
	Data interface{}
	// Context 任务上下文（任意键值对）。
	Context map[string]interface{}
}

// PluginOutput 插件输出。
type PluginOutput struct {
	// Result 执行结果（任意类型）。
	Result interface{}
	// Error 执行错误（nil 表示成功——与标准库 error 接口兼容）。
	Error error
	// Meta 附加元数据（任意键值对）。
	Meta map[string]interface{}
}
