// internal_engine.go — 内部任务引擎状态（2026-09-10 Mr2109拍板：治本方案①-③）
//
// 背景（APP-A07 治本）：启停状态原有三个真相源——UI 本地布尔、后端 stopped 标志（无人读取）、
// 环境门控 ZERG_INTERNAL_TASKS（UI 不可见）；且 stopped 不持久化 → 重启必漂移。
//
// 本文件建立**唯一真相源**：
//
//	GET /api/internal-tasks/state → {enabled, running, stopped, since, last_tick, next_tick, reason, updated_at}
//	变更接口（start/stop/mode/interval）响应体统一回带 state（契约：变更即回状态，UI 无需二次往返）
//	持久化 ~/.zerg/state/internal_engine.json（重启恢复用户意图；路径遵循 statepath 规则）
//
// 语义澄清：
//
//	enabled   = 环境门控是否放行（ZERG_INTERNAL_TASKS=1）——环境事实，不可由 UI 改变
//	stopped   = 用户意图（按钮）——持久化，重启保留
//	running   = 引擎此刻真的在跑 = enabled && !stopped
//	last_tick = 心跳。running=true 但心跳停滞 → UI 显示"异常"，不再谎报"运行中"
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"zerg/core/internal/statepath"
)

// EngineStateView 引擎状态快照（JSON 契约——UI 与外部 agent 共用）
type EngineStateView struct {
	Enabled   bool   `json:"enabled"`   // 环境门控放行（ZERG_INTERNAL_TASKS=1）
	Running   bool   `json:"running"`   // 引擎真的在跑（enabled && !stopped）
	Stopped   bool   `json:"stopped"`   // 用户意图：已停止
	Since     string `json:"since"`     // 当前状态起始时间（RFC3339）
	LastTick  string `json:"last_tick"` // 心跳（RFC3339——空=从未跑过）
	NextTick  string `json:"next_tick"` // 预计下次心跳
	Reason    string `json:"reason"`    // 人类可读解释（未启用/已停止/心跳停滞等）
	UpdatedAt string `json:"updated_at"`
}

type engineState struct {
	Enabled   bool      `json:"enabled"`
	Stopped   bool      `json:"stopped"`
	Since     time.Time `json:"since"`
	LastTick  time.Time `json:"last_tick"`
	NextTick  time.Time `json:"next_tick"`
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	engineMu   sync.RWMutex
	engine     = engineState{}
	engineFile string
)

func engineStatePath() string {
	if engineFile == "" {
		engineFile = statepath.File("internal_engine.json")
	}
	return engineFile
}

// InitInternalEngine 启动恢复（main.go 调用——早于 SetEngineEnabled）
// 恢复用户意图（stopped/since）；文件不存在=默认未停止（兼容现状）
func InitInternalEngine() {
	engineMu.Lock()
	defer engineMu.Unlock()
	data, err := os.ReadFile(engineStatePath())
	if err == nil {
		var raw engineState
		if json.Unmarshal(data, &raw) == nil {
			engine.Stopped = raw.Stopped
			engine.Since = raw.Since
		}
	}
	if engine.Since.IsZero() {
		engine.Since = time.Now()
	}
	engine.UpdatedAt = time.Now()
}

// saveEngineLocked 持久化（调用方持锁——目录不存在则创建；失败仅告警不阻断）
func saveEngineLocked() {
	data, err := json.MarshalIndent(engine, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(statepath.Dir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(engineStatePath(), data, 0o644)
}

// SetEngineEnabled 设置环境门控（main.go 启动时按 ZERG_INTERNAL_TASKS 调用）
func SetEngineEnabled(enabled bool, reason string) {
	engineMu.Lock()
	defer engineMu.Unlock()
	engine.Enabled = enabled
	engine.Reason = reason
	engine.UpdatedAt = time.Now()
	saveEngineLocked()
}

// SetEngineStopped 设置用户意图（启停按钮）
// 返回设置后的状态快照（供 handler 直接回带）
func SetEngineStopped(stopped bool, reason string) EngineStateView {
	engineMu.Lock()
	defer engineMu.Unlock()
	if engine.Stopped != stopped {
		engine.Since = time.Now()
	}
	engine.Stopped = stopped
	engine.Reason = reason
	engine.UpdatedAt = time.Now()
	if stopped {
		engine.NextTick = time.Time{}
	}
	saveEngineLocked()
	return engineViewLocked()
}

// EngineTick 心跳打点（主控周期循环每跑一圈调用；同时给出预计下次）
func EngineTick(interval time.Duration) {
	engineMu.Lock()
	defer engineMu.Unlock()
	now := time.Now()
	engine.LastTick = now
	if interval > 0 {
		engine.NextTick = now.Add(interval)
	}
	engine.UpdatedAt = now
}

// EngineSkippedTick 引擎被停止时循环仍然活着（只记"还活着"，不改心跳——保持"已停止"语义）
func EngineSkippedTick() {
	engineMu.Lock()
	defer engineMu.Unlock()
	engine.UpdatedAt = time.Now()
}

// InternalEngineStopped 引擎是否应停止触发（周期循环/其它触发路径拦阻用——真实生效）
func InternalEngineStopped() bool {
	engineMu.RLock()
	defer engineMu.RUnlock()
	return engine.Stopped
}

func engineViewLocked() EngineStateView {
	now := time.Now()
	v := EngineStateView{
		Enabled:   engine.Enabled,
		Running:   engine.Enabled && !engine.Stopped,
		Stopped:   engine.Stopped,
		Reason:    engine.Reason,
		UpdatedAt: engine.UpdatedAt.Format(time.RFC3339),
	}
	if !engine.Since.IsZero() {
		v.Since = engine.Since.Format(time.RFC3339)
	}
	if !engine.LastTick.IsZero() {
		v.LastTick = engine.LastTick.Format(time.RFC3339)
	}
	if !engine.NextTick.IsZero() {
		v.NextTick = engine.NextTick.Format(time.RFC3339)
	}
	// 心跳停滞检测（周期 60s——超过 3 分钟无心跳 = 异常，不谎报运行中）
	if v.Running && !engine.LastTick.IsZero() && now.Sub(engine.LastTick) > 3*time.Minute {
		v.Running = false
		v.Reason = "引擎开关为运行，但心跳已停滞（疑似异常）——最后一次心跳 " + engine.LastTick.Format("15:04:05")
	}
	if !engine.Enabled {
		v.Running = false
	}
	return v
}

// InternalEngineStateView 当前状态快照（供其它 handler 回带）
func InternalEngineStateView() EngineStateView {
	engineMu.RLock()
	defer engineMu.RUnlock()
	return engineViewLocked()
}

// InternalEngineStateMap 当前状态（map 形态——嵌入其它响应体用）
func InternalEngineStateMap() map[string]interface{} {
	v := InternalEngineStateView()
	return map[string]interface{}{
		"enabled":    v.Enabled,
		"running":    v.Running,
		"stopped":    v.Stopped,
		"since":      v.Since,
		"last_tick":  v.LastTick,
		"next_tick":  v.NextTick,
		"reason":     v.Reason,
		"updated_at": v.UpdatedAt,
	}
}

// InternalEngineHandler 引擎状态查询（UI 轮询 + 运维 curl）
// GET /api/internal-tasks/state
func (h *Handlers) InternalEngineHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"state": InternalEngineStateMap(),
		"note":  "enabled=环境门控（ZERG_INTERNAL_TASKS=1）；stopped=用户意图（持久化）；running=enabled&&!stopped 且心跳未停滞",
	})
}
