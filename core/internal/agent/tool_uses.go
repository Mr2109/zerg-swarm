package agent

// tool_uses.go — v2.5.7 P4-49 工具调用统一计数（Mr2109 2026-09-02）
// 设计: 所有工具调用次数归零——重新计数——无论对话（ExecuteChatTool）还是 CA（ExecuteTool）调用都计入
// 存储: /tmp/zerg-tool-uses.json
//
// 2026-09-06 修复（多进程覆盖丢失）:
//   原实现=每进程独立内存 map + 整体覆盖写盘——多进程(主控/多个CA/测试)并发时后写者覆盖先写者——
//   历史计数被清空（Mr2109发现: 昨天几十次今天只剩几次）。
//   新实现=RecordToolUse 直接 flock 文件锁 读-改-写——多进程原子累加——进程内不再持写缓存。

import (
	"encoding/json"
	"os"
	"sync"
	"syscall"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// toolUsesFile — 工具计数持久化文件
// 甲批 T2（2026-09-10）：/tmp → ~/.zerg/state/tool_uses.json（首次启动自动搬旧文件）
var toolUsesFile = statepath.MigrateIfNeeded("/tmp/zerg-tool-uses.json", "tool_uses.json")

// toolUseCount — 读缓存（查询用——启动加载一次；写永远走文件锁，不更新此缓存）
// 查询低频（UI 拉取）——直接读盘也行；保留缓存减少 IO
var (
	toolUseMu    sync.Mutex
	toolUseCount = map[string]int{}
)

// loadToolUses 加载计数（启动时——查询缓存）
func loadToolUses() {
	toolUseMu.Lock()
	defer toolUseMu.Unlock()
	if b, err := os.ReadFile(toolUsesFile); err == nil {
		_ = json.Unmarshal(b, &toolUseCount)
	}
}

// flockFile — 打开计数文件并加独占锁（多进程原子——2026-09-06 修）
func flockFile() (*os.File, error) {
	f, err := os.OpenFile(toolUsesFile, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// recordToolUseLocked — 持锁写盘: 读盘 → 累加 → 截断写回
func recordToolUseLocked(f *os.File, name string) error {
	counts := map[string]int{}
	if b, err := os.ReadFile(toolUsesFile); err == nil {
		_ = json.Unmarshal(b, &counts)
	}
	counts[name]++
	b, _ := json.Marshal(counts)
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	_, err := f.Write(b)
	return err
}

// RecordToolUse 工具调用计数 +1（对话/CA 统一——成功执行才计）
// 2026-09-06: flock 读-改-写盘——多进程不互覆（原整体覆盖丢计数）
func RecordToolUse(name string) {
	if name == "" || name[0] == '_' {
		return // 跳过内部工具（__bad_format__ 等）
	}
	f, err := flockFile()
	if err != nil {
		return
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	defer f.Close()
	_ = recordToolUseLocked(f, name)
}

// ToolUses 查询工具调用次数（2026-09-06: 直读盘保证最新——多进程写后缓存会陈旧）
func ToolUses(name string) int {
	counts := ToolUsesAll()
	return counts[name]
}

// ToolUsesAll 全量计数（UI 拉取用——直接读盘保证最新）
func ToolUsesAll() map[string]int {
	out := map[string]int{}
	if b, err := os.ReadFile(toolUsesFile); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// ResetToolUses 归零所有工具计数（Mr2109 2026-09-02——重新计数）
func ResetToolUses() {
	f, err := flockFile()
	if err != nil {
		return
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	defer f.Close()
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.Write([]byte("{}"))
}

// InitToolUses 初始化计数（加载——core/agent 启动调用）
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
