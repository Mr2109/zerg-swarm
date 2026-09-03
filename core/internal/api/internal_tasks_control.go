// Package api - v2.5.6 内部任务启停（Mr2109 2026-08-27——UI 停止/启动内部任务）
package api

import (
	"net/http"
	"sync"
)

// internalTasksControl 内部任务启停控制（全局——main.go 设置 idleDetector 引用）
var internalTasksControl = struct {
	sync.RWMutex
	stopped bool
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

// InternalTasksStopped 内部任务是否已停止（UI 显示状态用）
func InternalTasksStopped() bool {
	internalTasksControl.RLock()
	defer internalTasksControl.RUnlock()
	return internalTasksControl.stopped
}

// InternalTasksStopHandler 停止内部任务（UI 按钮）
func InternalTasksStopHandler(w http.ResponseWriter, r *http.Request) {
	internalTasksControl.Lock()
	defer internalTasksControl.Unlock()
	internalTasksControl.stopped = true
	if internalTasksControl.onStop != nil {
		internalTasksControl.onStop()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true,"internal_stopped":true}`))
}

// InternalTasksStartHandler 启动内部任务（UI 按钮）
func InternalTasksStartHandler(w http.ResponseWriter, r *http.Request) {
	internalTasksControl.Lock()
	defer internalTasksControl.Unlock()
	internalTasksControl.stopped = false
	if internalTasksControl.onStart != nil {
		internalTasksControl.onStart()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true,"internal_stopped":false}`))
}
