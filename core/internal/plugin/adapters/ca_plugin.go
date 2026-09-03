// Package adapters 提供插件适配器实现。
// CaPlugin 包装 ca（虫族 agent）——Execute 派发 agent 任务。
package adapters

import (
	"fmt"

	"zerg/core/internal/plugin"
)

// AgentRunner 抽象 agent 执行（可 mock 测试）。
type AgentRunner interface {
	RunTask(task string, workdir string) (string, error)
}

// CaPlugin 实现 Plugin 接口——包装 ca agent 执行。
type CaPlugin struct {
	name    string
	version string
	runner  AgentRunner
	started bool
}

// NewCaPlugin 创建 CaPlugin。
func NewCaPlugin(runner AgentRunner) *CaPlugin {
	return &CaPlugin{
		name:    "ca",
		version: "0.1.0",
		runner:  runner,
	}
}

// Name 插件名。
func (p *CaPlugin) Name() string { return p.name }

// Type 插件类型。
func (p *CaPlugin) Type() plugin.PluginType { return plugin.PluginTypeCA }

// Version 版本。
func (p *CaPlugin) Version() string { return p.version }

// Capabilities 能力声明。
func (p *CaPlugin) Capabilities() []string {
	return []string{"agent-task", "subagent"}
}

// Init 初始化。
func (p *CaPlugin) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	if v, ok := cfg["name"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.name = s
		}
	}
	return nil
}

// Start 启动。
func (p *CaPlugin) Start() error {
	if p.runner == nil {
		return fmt.Errorf("ca: runner 未设置")
	}
	p.started = true
	return nil
}

// Stop 停止。
func (p *CaPlugin) Stop() error { p.started = false; return nil }

// Close 关闭。
func (p *CaPlugin) Close() error { p.started = false; return nil }

// Execute 派发 agent 任务。
func (p *CaPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !p.started {
		return plugin.PluginOutput{}, fmt.Errorf("ca: not started")
	}
	task, _ := input.Context["task"].(string)
	if task == "" {
		return plugin.PluginOutput{}, fmt.Errorf("ca: task 为空")
	}
	workdir, _ := input.Context["workdir"].(string)
	result, err := p.runner.RunTask(task, workdir)
	if err != nil {
		return plugin.PluginOutput{Error: err}, err
	}
	return plugin.PluginOutput{
		Result: map[string]interface{}{"output": result, "task_id": input.TaskID},
	}, nil
}
