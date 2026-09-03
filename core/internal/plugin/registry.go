// Package plugin 提供虫族插件体系的基础接口和类型定义。
// 严格依照《设计-v2.5.4-插件体系-定稿》第三节（插件接口）定义。
package plugin

import (
	"fmt"
	"sync"
)

// Registry 插件注册表——管理已注册的插件（并发安全）。
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin // name -> plugin
	order   []string          // 注册顺序（List 稳定返回——v2.5.4.8 优化）
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

// Register 注册插件（重名报错）。
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p == nil {
		return fmt.Errorf("plugin 不能为 nil")
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin 名称不能为空")
	}
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("插件 %s 已注册", name)
	}
	r.plugins[name] = p
	r.order = append(r.order, name)
	return nil
}

// Unregister 注销插件（热插拔卸载用）。
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Get 按名称获取插件。
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List 列出全部插件（按注册顺序——稳定）。
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.order))
	for _, name := range r.order {
		if p, ok := r.plugins[name]; ok {
			out = append(out, p)
		}
	}
	return out
}

// ByType 按类型列出插件。
func (r *Registry) ByType(t PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Plugin
	for _, p := range r.plugins {
		if p.Type() == t {
			out = append(out, p)
		}
	}
	return out
}

// Count 插件数量。
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Names 全部插件名。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.plugins))
	for n := range r.plugins {
		out = append(out, n)
	}
	return out
}
