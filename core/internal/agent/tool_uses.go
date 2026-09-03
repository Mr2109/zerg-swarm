package agent

// tool_uses.go — v2.5.7 P4-49 工具调用统一计数（Mr2109 2026-09-02）
// 设计: 所有工具调用次数归零——重新计数——无论对话（ExecuteChatTool）还是 CA（ExecuteTool）调用都计入
// 存储: /tmp/zerg-tool-uses.json（每次变更写盘——进程重启不丢）

import (
	"encoding/json"
	"os"
	"sync"
)

// toolUsesFile — 工具计数持久化文件
var toolUsesFile = "/tmp/zerg-tool-uses.json"

// toolUseCounts — 工具名 → 调用次数（线程安全）
var (
	toolUseMu    sync.Mutex
	toolUseCount = map[string]int{}
)

// loadToolUses 加载计数（启动时）
func loadToolUses() {
	toolUseMu.Lock()
	defer toolUseMu.Unlock()
	if b, err := os.ReadFile(toolUsesFile); err == nil {
		_ = json.Unmarshal(b, &toolUseCount)
	}
}

// saveToolUses 保存计数
func saveToolUses() {
	toolUseMu.Lock()
	b, _ := json.Marshal(toolUseCount)
	toolUseMu.Unlock()
	_ = os.WriteFile(toolUsesFile, b, 0o644)
}

// RecordToolUse 工具调用计数 +1（对话/CA 统一——成功执行才计）
func RecordToolUse(name string) {
	if name == "" || name[0] == '_' {
		return // 跳过内部工具（__bad_format__ 等）
	}
	toolUseMu.Lock()
	toolUseCount[name]++
	toolUseMu.Unlock()
	saveToolUses()
}

// ToolUses 查询工具调用次数
func ToolUses(name string) int {
	toolUseMu.Lock()
	defer toolUseMu.Unlock()
	return toolUseCount[name]
}

// ResetToolUses 归零所有工具计数（Mr2109 2026-09-02——重新计数）
func ResetToolUses() {
	toolUseMu.Lock()
	toolUseCount = map[string]int{}
	toolUseMu.Unlock()
	saveToolUses()
}

// InitToolUses 初始化计数（加载）
func InitToolUses() {
	loadToolUses()
}

// ─── 工具版本（P4-49——Mr2109 2026-09-02: 每个工具版本号——进化可追溯）───

// toolVersionsFile — 工具版本表（项目根 tools/versions.json——工具资产集中）
var toolVersionsFile = "<repo>/tools/versions.json"

// toolVersions — 工具名 → 版本（加载缓存）
var toolVersions = map[string]string{}

// InitToolVersions 加载版本表（启动时）
func InitToolVersions() {
	toolUseMu.Lock()
	defer toolUseMu.Unlock()
	if b, err := os.ReadFile(toolVersionsFile); err == nil {
		_ = json.Unmarshal(b, &toolVersions)
	}
}

// ToolVersion 查询工具版本（未登记默认 v1.0.0）
func ToolVersion(name string) string {
	toolUseMu.Lock()
	defer toolUseMu.Unlock()
	if v, ok := toolVersions[name]; ok {
		return v
	}
	return "v1.0.0"
}
