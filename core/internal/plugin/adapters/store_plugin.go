// Package adapters 提供插件适配器实现。
// StorePlugin 简单 KV 存储插件——Execute Get/Set。
package adapters

import (
	"fmt"
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// StorePlugin 实现 Plugin 接口——KV 存储。
type StorePlugin struct {
	name    string
	version string
	mu      sync.RWMutex
	data    map[string]string
	started bool
}

// NewStorePlugin 创建 StorePlugin。
func NewStorePlugin() *StorePlugin {
	return &StorePlugin{
		name:    "store",
		version: "0.1.0",
		data:    make(map[string]string),
	}
}

// Name 插件名。
func (p *StorePlugin) Name() string { return p.name }

// Type 插件类型。
func (p *StorePlugin) Type() plugin.PluginType { return plugin.PluginTypeStore }

// Version 版本。
func (p *StorePlugin) Version() string { return p.version }

// Capabilities 能力声明。
func (p *StorePlugin) Capabilities() []string {
	return []string{"kv-store", "persistence"}
}

// Init 初始化。
func (p *StorePlugin) Init(cfg map[string]interface{}) error { return nil }

// Start 启动。
func (p *StorePlugin) Start() error { p.started = true; return nil }

// Stop 停止。
func (p *StorePlugin) Stop() error { p.started = false; return nil }

// Close 关闭。
func (p *StorePlugin) Close() error { p.started = false; return nil }

// Execute 读写存储——Data 传 map: {"action":"get"/"set", "key":..., "value":...}。
func (p *StorePlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if !p.started {
		return plugin.PluginOutput{}, fmt.Errorf("store: not started")
	}
	args, ok := input.Data.(map[string]interface{})
	if !ok {
		return plugin.PluginOutput{}, fmt.Errorf("store: Data 需为 map[string]interface{}")
	}
	action, _ := args["action"].(string)
	key, _ := args["key"].(string)
	switch action {
	case "set":
		val, _ := args["value"].(string)
		p.mu.Lock()
		p.data[key] = val
		p.mu.Unlock()
		return plugin.PluginOutput{Result: map[string]interface{}{"ok": true}}, nil
	case "get":
		p.mu.RLock()
		val, exists := p.data[key]
		p.mu.RUnlock()
		return plugin.PluginOutput{Result: map[string]interface{}{"key": key, "value": val, "exists": exists}}, nil
	default:
		return plugin.PluginOutput{}, fmt.Errorf("store: 未知 action %s", action)
	}
}
