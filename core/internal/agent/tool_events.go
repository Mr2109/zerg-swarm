package agent

// tool_events.go — v2.5.8 工具事件流水（2026-09-06 Mr2109——未来虫族区块链工作量账本雏形）
// 定位: 工具使用 = 未来开源/虫币经济的贡献度凭证——账本要求可审计(时间)/可汇总(节点)/可回溯
// 架构: 事件流水(源——按天 JSONL 追加) + 计数器(派生——flock JSON 顺带更新——读路径不变)
// 记录: {"ts":秒,"node":"local/x3","tool":"bash","dur_ms":120}——不记参数内容(隐私——够审计不泄活)
//
// 2026-09-06 升级（Mr2109: 计数器只是转正依据太窄——是所有人使用汇总数据的源）:
//   RecordToolUse → ①写事件流(按天轮转) ②flock 更新计数——两者皆原子追加

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// toolEventsDir — 事件流目录（按天文件——历史留档可回溯）
// 甲批 T2（2026-09-10）：/tmp → ~/.zerg/state/tool_events/（首次启动自动搬历史目录）
var toolEventsDir = func() string {
	d := statepath.MigrateIfNeeded("/tmp/zerg-tool-events", "tool_events")
	_ = os.MkdirAll(d, 0o755)
	return d
}()

// toolEventsMu — 事件写锁（进程内——跨进程靠 O_APPEND 原子追加）
var toolEventsMu sync.Mutex

// ToolEvent — 单条工具事件（账本单元——未来节点上报聚合）
type ToolEvent struct {
	Ts    int64  `json:"ts"`     // Unix 秒（时间维度——按天/月聚合）
	Node  string `json:"node"`   // 执行节点（local/x3/...——未来多节点汇总）
	Tool  string `json:"tool"`   // 工具名
	DurMs int64  `json:"dur_ms"` // 执行耗时（毫秒——工作量参考）
}

// nodeName — 当前节点标识（hostname 简写——未来 fleet 名）
var nodeName = func() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}()

// toolEventsFileFor — 当日事件文件名（按天轮转——历史自然归档）
func toolEventsFileFor(day string) string {
	return filepath.Join(toolEventsDir, "zerg-tool-events-"+day+".jsonl")
}

// todayStr — YYYY-MM-DD（本机时区）
func todayStr() string {
	return time.Now().Format("2006-01-02")
}

// AppendToolEvent — 导出的工具事件追加（对话侧 chat 包调用——CA 侧在 ExecuteTool 内联）
func AppendToolEvent(tool string, durMs int64) {
	appendToolEvent(ToolEvent{Ts: time.Now().Unix(), Node: nodeName, Tool: tool, DurMs: durMs})
}

// appendToolEvent — 追加一条工具事件（O_APPEND 原子——多进程安全）
// 目录懒创建; 失败静默(事件流是增强——不影响工具执行主流程)
func appendToolEvent(ev ToolEvent) {
	toolEventsMu.Lock()
	defer toolEventsMu.Unlock()
	if err := os.MkdirAll(toolEventsDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(toolEventsFileFor(todayStr()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if b, err := json.Marshal(ev); err == nil {
		_, _ = f.Write(append(b, '\n'))
	}
}

// ToolEventsToday — 当日事件明细（UI/审计查——按时间序）
func ToolEventsToday() []ToolEvent {
	return toolEventsOf(toolEventsFileFor(todayStr()))
}

// ToolEventsOfDay — 查指定日（YYYY-MM-DD——历史回溯）
func ToolEventsOfDay(day string) []ToolEvent {
	return toolEventsOf(toolEventsFileFor(day))
}

// toolEventsOf — 读事件文件全部行（坏行容错）
func toolEventsOf(path string) []ToolEvent {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []ToolEvent
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev ToolEvent
		if json.Unmarshal([]byte(line), &ev) == nil {
			out = append(out, ev)
		}
	}
	return out
}
