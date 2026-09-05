package agent

// extra_tools.go — v2.5.7 P4-49 统一工具库（CA/对话通用——Mr2109 2026-09-02）
// 设计: 对话层 126 个 deferred 工具注册到 agent 层扩展注册表——agent.ExecuteTool 不认识时路由到注册执行器
// 原则: 一套注册中心——CA/对话都能调——只是入口不同

import (
	"sync"
)

// ExtraTool — 扩展工具（对话层注册——CA/对话通用）
type ExtraTool struct {
	Name string                                                    // 工具名
	Desc string                                                    // 描述（工具列表/搜索用）
	Fn   func(args map[string]any, workDir string) (string, error) // 执行器
}

// extraToolRegistry — 扩展工具注册表（线程安全）
var (
	extraToolMu    sync.RWMutex
	extraToolReg   = map[string]ExtraTool{}
	extraToolOrder []string // 注册顺序（AllTools 稳定排序）
)

// RegisterExtraTool — 注册扩展工具（对话层启动时调用——重复注册覆盖）
func RegisterExtraTool(t ExtraTool) {
	extraToolMu.Lock()
	defer extraToolMu.Unlock()
	if _, exists := extraToolReg[t.Name]; !exists {
		extraToolOrder = append(extraToolOrder, t.Name)
	}
	extraToolReg[t.Name] = t
}

// LookupExtraTool — 查询扩展工具（ExecuteTool 路由用）
func LookupExtraTool(name string) (ExtraTool, bool) {
	extraToolMu.RLock()
	defer extraToolMu.RUnlock()
	t, ok := extraToolReg[name]
	return t, ok
}

// ExtraTools — 全部扩展工具（按注册顺序）
func ExtraTools() []ExtraTool {
	extraToolMu.RLock()
	defer extraToolMu.RUnlock()
	out := make([]ExtraTool, 0, len(extraToolOrder))
	for _, name := range extraToolOrder {
		out = append(out, extraToolReg[name])
	}
	return out
}

// ExtraToolNames — 扩展工具名集合（去重/判断用）
func ExtraToolNames() map[string]bool {
	extraToolMu.RLock()
	defer extraToolMu.RUnlock()
	out := make(map[string]bool, len(extraToolReg))
	for name := range extraToolReg {
		out[name] = true
	}
	return out
}
