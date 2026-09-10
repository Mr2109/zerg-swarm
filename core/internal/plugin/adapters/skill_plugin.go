// Package adapters 提供 skill/mcp/ca 插件适配器实现。
// 每个适配器包装现有 manager，实现 plugin.Plugin 接口。
package adapters

import (
	"fmt"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// SkillLoader 接口（解耦——适配器不依赖具体 SkillManager）
//
// SkillManager 在 agent 包，适配器在 plugin 包。
// 通过接口解耦——测试时用 mock，生产时用真实 SkillManager。
type SkillLoader interface {
	Load(name string) (string, error)
	ListDescriptions() string
	SkillNames() []string
}

// SkillPlugin 实现 plugin.Plugin 接口
// 包装现有 SkillManager——Execute 执行技能加载
//
// 设计要点：
//   - 零外部依赖（只用到 Go 标准库）
//   - 通过 SkillLoader 接口解耦——不直接依赖 agent.SkillManager
//   - 可配置（Init 可覆盖默认值）
//
// 来自设计-v2.5.4.5 skill 插件化
type SkillPlugin struct {
	name    string
	version string
	// 内部 SkillManager（通过接口解耦）
	loader SkillLoader
	// 状态
	initialized bool
	started     bool
}

// NewSkillPlugin 创建 SkillPlugin（包装 SkillManager）
func NewSkillPlugin(loader SkillLoader) *SkillPlugin {
	return &SkillPlugin{
		name:    "skill",
		version: "0.1.0",
		loader:  loader,
	}
}

// Name 插件唯一名称
func (s *SkillPlugin) Name() string { return s.name }

// Type 插件类型
func (s *SkillPlugin) Type() plugin.PluginType { return plugin.PluginTypeSkill }

// Version 插件版本（SemVer 格式）
func (s *SkillPlugin) Version() string { return s.version }

// Capabilities 插件声明的能力列表
func (s *SkillPlugin) Capabilities() []string {
	return []string{
		"skill-load",
		"skill-list",
		"skill-descriptions",
	}
}

// Init 初始化——接收配置
// cfg 支持:
//   - loader: SkillLoader（必须，注入 SkillManager）
//   - 其他配置项忽略
func (s *SkillPlugin) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	// loader 已在构造时注入，Init 只做标记
	s.initialized = true
	return nil
}

// Start 启动插件
func (s *SkillPlugin) Start() error {
	if !s.initialized {
		return fmt.Errorf("skill plugin: not initialized")
	}
	s.started = true
	return nil
}

// Stop 停止插件
func (s *SkillPlugin) Stop() error {
	s.started = false
	return nil
}

// Close 关闭插件
func (s *SkillPlugin) Close() error {
	s.initialized = false
	s.started = false
	return nil
}

// SkillExecuteInput Execute 的输入 Data 类型
type SkillExecuteInput struct {
	SkillName string // 要加载的技能名称
}

// SkillExecuteOutput Execute 的输出 Result 类型
type SkillExecuteOutput struct {
	SkillName string // 技能名称
	Body      string // 技能正文
	LoadedAt  string // 加载时间
}

// Execute 执行任务——加载并返回技能正文
// input.Data 期望是 SkillExecuteInput（含 SkillName）
// 返回 PluginOutput:
//   - Result: SkillExecuteOutput（含 Body）
func (s *SkillPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !s.started {
		return plugin.PluginOutput{}, fmt.Errorf("skill plugin: not started")
	}
	if s.loader == nil {
		return plugin.PluginOutput{}, fmt.Errorf("skill plugin: loader is nil")
	}

	var skillName string
	switch v := input.Data.(type) {
	case SkillExecuteInput:
		skillName = v.SkillName
	case string:
		skillName = v
	case map[string]interface{}:
		if name, ok := v["skill_name"].(string); ok {
			skillName = name
		} else if name, ok := v["name"].(string); ok {
			skillName = name
		}
	default:
		return plugin.PluginOutput{}, fmt.Errorf("skill plugin: invalid input type %T", input.Data)
	}

	if skillName == "" {
		return plugin.PluginOutput{}, fmt.Errorf("skill plugin: skill_name is empty")
	}

	body, err := s.loader.Load(skillName)
	if err != nil {
		return plugin.PluginOutput{
			Meta: map[string]interface{}{
				"error_type": "skill_not_found",
			},
		}, err
	}

	return plugin.PluginOutput{
		Result: SkillExecuteOutput{
			SkillName: skillName,
			Body:      body,
			LoadedAt:  time.Now().Format(time.RFC3339),
		},
		Meta: map[string]interface{}{
			"body_length": len(body),
		},
	}, nil
}

// 验证 SkillPlugin 实现 Plugin 接口（编译期检查）
var _ plugin.Plugin = (*SkillPlugin)(nil)

// 便捷：从 SkillManager 创建 SkillPlugin
// 如果调用方有 agent.SkillManager 实例，可直接用此函数创建插件
//
// 示例:
//	ListAllSkills 列出所有可用技能（便捷方法）
func (s *SkillPlugin) ListAllSkills() []string {
	if s.loader == nil {
		return nil
	}
	return s.loader.SkillNames()
}

// GetDescriptions 获取所有技能描述（便捷方法）
func (s *SkillPlugin) GetDescriptions() string {
	if s.loader == nil {
		return ""
	}
	return s.loader.ListDescriptions()
}
