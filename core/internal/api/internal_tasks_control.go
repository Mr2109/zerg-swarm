// Package api - v2.5.6 内部任务启停（Mr2109 2026-08-27——UI 停止/启动内部任务）
// 2026-09-10 治本（APP-A07）：启停不再只改标志位——
//
//	① 状态入单一真相源（internal_engine.go，持久化、可查询）
//	② 停止对**所有**触发路径生效（周期调度 + 空闲检测——原先周期调度根本不看该标志）
//	③ 环境门控未放行时如实拒绝（原实现空桩应答 ok——UI 以为启动成功）
package api

import (
	"net/http"
	"sync"
)

// internalTasksControl 内部任务启停控制（全局——main.go 注入 idleDetector 操作函数）
var internalTasksControl = struct {
	sync.RWMutex
	// 控制函数（main.go 注入——操作 idleDetector）
	onStop  func()
	onStart func()
}{}

// SetInternalTasksControl 注入控制函数（main.go 调用——操作 idleDetector）
// onStop: 停止内部任务（idle 检测不再触发）——onStart: 启动（恢复触发）
func SetInternalTasksControl(onStop, onStart func()) {
	internalTasksControl.Lock()
	defer internalTasksControl.Unlock()
	internalTasksControl.onStop = onStop
	internalTasksControl.onStart = onStart
}

// InternalTasksStopped 用户是否已停止内部任务（周期循环每圈调用——2026-09-10：原先无人读取）
func InternalTasksStopped() bool { return InternalEngineStopped() }

// InternalTasksStopHandler 停止内部任务（UI 按钮）
// POST /api/internal-tasks/stop —— 响应体回带完整 state（契约：变更即回状态）
func InternalTasksStopHandler(w http.ResponseWriter, r *http.Request) {
	st := SetEngineStopped(true, "已按用户请求停止（UI 按钮）")
	internalTasksControl.RLock()
	onStop := internalTasksControl.onStop
	internalTasksControl.RUnlock()
	if onStop != nil {
		onStop()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"success": true,
		"state":   InternalEngineStateMap(),
		"message": "内部任务已停止" + stoppedSuffix(st),
	})
}

// InternalTasksStartHandler 启动内部任务（UI 按钮）
// POST /api/internal-tasks/start —— 门控未放行时如实拒绝（不再假成功）
func InternalTasksStartHandler(w http.ResponseWriter, r *http.Request) {
	if !InternalEngineStateView().Enabled {
		// 环境门控未放行：明确 403 + 回带状态（UI 显示"未启用"及原因，而不是假装启动成功）
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"ok":      false,
			"success": false,
			"state":   InternalEngineStateMap(),
			"error":   "内部任务引擎未启用（2026-09-06 事故后默认关闭）——需以 ZERG_INTERNAL_TASKS=1 启动主控",
		})
		return
	}
	st := SetEngineStopped(false, "已按用户请求启动（UI 按钮）")
	internalTasksControl.RLock()
	onStart := internalTasksControl.onStart
	internalTasksControl.RUnlock()
	if onStart != nil {
		onStart()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"success": true,
		"state":   InternalEngineStateMap(),
		"message": "内部任务已启动" + startedSuffix(st),
	})
}

func stoppedSuffix(st EngineStateView) string {
	if !st.Enabled {
		return "（注意：引擎本就未启用——本操作只记录意图）"
	}
	return ""
}

func startedSuffix(st EngineStateView) string {
	if !st.Enabled {
		return "（注意：引擎未启用）"
	}
	return ""
}
