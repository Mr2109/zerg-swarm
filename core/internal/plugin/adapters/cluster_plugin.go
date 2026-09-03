// Package adapters 提供插件适配器实现。
// ClusterPlugin 包装集群状态——Execute 返回机器列表/健康。
package adapters

import (
	"fmt"

	"zerg/core/internal/plugin"
)

// MachineInfo 集群机器信息。
type MachineInfo struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Healthy bool   `json:"healthy"`
	Model   string `json:"model"`
}

// ClusterPlugin 实现 Plugin 接口——集群状态。
type ClusterPlugin struct {
	name     string
	version  string
	machines []MachineInfo
	started  bool
}

// NewClusterPlugin 创建 ClusterPlugin（默认机器列表）。
func NewClusterPlugin(machines []MachineInfo) *ClusterPlugin {
	if machines == nil {
		machines = []MachineInfo{
			{Name: "x3", Host: "<worker-ip>", Healthy: true, Model: "example-35b"},
			{Name: "local", Host: "127.0.0.1", Healthy: true, Model: "example-35b-Q4_K_M"},
		}
	}
	return &ClusterPlugin{
		name:     "cluster",
		version:  "0.1.0",
		machines: machines,
	}
}

// Name 插件名。
func (p *ClusterPlugin) Name() string { return p.name }

// Type 插件类型。
func (p *ClusterPlugin) Type() plugin.PluginType { return plugin.PluginTypeCluster }

// Version 版本。
func (p *ClusterPlugin) Version() string { return p.version }

// Capabilities 能力声明。
func (p *ClusterPlugin) Capabilities() []string {
	return []string{"fleet-status", "machine-health"}
}

// Init 初始化。
func (p *ClusterPlugin) Init(cfg map[string]interface{}) error { return nil }

// Start 启动。
func (p *ClusterPlugin) Start() error { p.started = true; return nil }

// Stop 停止。
func (p *ClusterPlugin) Stop() error { p.started = false; return nil }

// Close 关闭。
func (p *ClusterPlugin) Close() error { p.started = false; return nil }

// Execute 返回集群状态（Data 传 action: "status"/"list"——默认 status）。
func (p *ClusterPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !p.started {
		return plugin.PluginOutput{}, fmt.Errorf("cluster: not started")
	}
	return plugin.PluginOutput{
		Result: map[string]interface{}{
			"machines": p.machines,
			"count":    len(p.machines),
		},
	}, nil
}
