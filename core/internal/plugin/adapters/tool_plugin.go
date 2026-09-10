// Package adapters 提供插件适配器实现。
// ToolPlugin 包装工具执行（bash exec 包装——context 里传 command）。
package adapters

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// ToolPlugin 实现 Plugin 接口——Execute 执行工具命令（bash exec 包装）。
// 通过 context 传入 "command" 字段——如 "ls -la" 或 ["bash", "-c", "echo hello"]。
type ToolPlugin struct {
	name    string
	version string
	started bool
}

// NewToolPlugin 创建 ToolPlugin。
func NewToolPlugin() *ToolPlugin {
	return &ToolPlugin{
		name:    "tool",
		version: "0.1.0",
	}
}

// Name 插件名。
func (p *ToolPlugin) Name() string { return p.name }

// Type 插件类型。
func (p *ToolPlugin) Type() plugin.PluginType { return plugin.PluginTypeTool }

// Version 版本。
func (p *ToolPlugin) Version() string { return p.version }

// Capabilities 能力声明。
func (p *ToolPlugin) Capabilities() []string {
	return []string{"exec", "shell", "command"}
}

// Init 初始化。
func (p *ToolPlugin) Init(cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	return nil
}

// Start 启动。
func (p *ToolPlugin) Start() error {
	p.started = true
	return nil
}

// Stop 停止。
func (p *ToolPlugin) Stop() error { p.started = false; return nil }

// Close 关闭。
func (p *ToolPlugin) Close() error { p.started = false; return nil }

// Execute 执行工具命令。
// input.Context["command"] 支持 string（shell 解析）或 []interface{}（直接执行）。
// input.Context["timeout"] 可选超时（秒），默认 30。
func (p *ToolPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !p.started {
		return plugin.PluginOutput{}, fmt.Errorf("tool: not started")
	}
	cmdRaw, ok := input.Context["command"]
	if !ok {
		return plugin.PluginOutput{}, fmt.Errorf("tool: command 未设置")
	}
	var args []string
	switch v := cmdRaw.(type) {
	case string:
		// shell 风格——直接用 exec.Command 执行
		args = []string{"bash", "-c", v}
	case []interface{}:
		args = make([]string, len(v))
		for i, item := range v {
			args[i] = fmt.Sprintf("%v", item)
		}
	default:
		return plugin.PluginOutput{}, fmt.Errorf("tool: command 类型错误: %T", cmdRaw)
	}
	// 超时控制
	timeout := 30
	if t, ok := input.Context["timeout"]; ok {
		switch tv := t.(type) {
		case int:
			timeout = tv
		case float64:
			timeout = int(tv)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmdCtx := ctx
	// 如果有 cwd
	if cwd, ok := input.Context["cwd"]; ok {
		if s, ok := cwd.(string); ok && s != "" {
			cmdCtx = context.Background()
		}
	}
	cmd := exec.CommandContext(cmdCtx, args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if cwd, ok := input.Context["cwd"]; ok {
		if s, ok := cwd.(string); ok && s != "" {
			cmd.Dir = s
		}
	}
	err := cmd.Run()
	result := map[string]interface{}{
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
		"command": cmdRaw,
	}
	if err != nil {
		result["exit_error"] = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		}
		return plugin.PluginOutput{
			Result: result,
			Error:  fmt.Errorf("tool: exec failed: %w", err),
			Meta:   map[string]interface{}{"task_id": input.TaskID},
		}, err
	}
	result["exit_code"] = 0
	return plugin.PluginOutput{
		Result: result,
		Meta:   map[string]interface{}{"task_id": input.TaskID},
	}, nil
}

// durationSeconds 简单转换秒数为 time.Duration。
func durationSeconds(s int) (sec int64) { return int64(s) }
