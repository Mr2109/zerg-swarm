package api

// internal_interval.go — 内部任务周期调度（2026-08-27 Mr2109）
// 任务循环周期: 设置后主控按周期自动触发该内部任务（如 health-check 每 6 小时跑一次）
// POST /api/internal-tasks/{id}/interval  {"hours": N}  设置周期（N<=0 = 取消）
// GET  /api/internal-tasks/intervals                    查询当前周期
// 持久化 /tmp/zerg-tasks/intervals.json——重启恢复——主控 goroutine 每 60s 检查到点

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

var (
	intervalMu   sync.Mutex
	intervals    = map[string]float64{}   // defID → 周期（小时）
	intervalLast = map[string]time.Time{} // defID → 上次触发时间
	intervalFile = "/tmp/zerg-tasks/intervals.json"
)

// loadIntervals 启动恢复（文件不存在忽略）
func loadIntervals() {
	data, err := os.ReadFile(intervalFile)
	if err != nil {
		return
	}
	var raw map[string]struct {
		Hours   float64   `json:"hours"`
		LastRun time.Time `json:"last_run"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	intervalMu.Lock()
	defer intervalMu.Unlock()
	for id, v := range raw {
		if v.Hours > 0 {
			intervals[id] = v.Hours
			intervalLast[id] = v.LastRun
		}
	}
}

// saveIntervals 持久化（调用方持锁）
func saveIntervals() {
	raw := map[string]struct {
		Hours   float64   `json:"hours"`
		LastRun time.Time `json:"last_run"`
	}{}
	for id, h := range intervals {
		raw[id] = struct {
			Hours   float64   `json:"hours"`
			LastRun time.Time `json:"last_run"`
		}{Hours: h, LastRun: intervalLast[id]}
	}
	data, _ := json.MarshalIndent(raw, "", "  ")
	_ = os.MkdirAll(filepath.Dir(intervalFile), 0o755)
	_ = os.WriteFile(intervalFile, data, 0o644)
}

// SetInternalInterval 设置/取消周期（hours<=0 = 取消）
func SetInternalInterval(defID string, hours float64) {
	intervalMu.Lock()
	defer intervalMu.Unlock()
	if hours <= 0 {
		delete(intervals, defID)
		delete(intervalLast, defID)
	} else {
		intervals[defID] = hours
		// 从头算起（设置后等一个完整周期才触发）
		if _, ok := intervalLast[defID]; !ok {
			intervalLast[defID] = time.Now()
		}
	}
	saveIntervals()
}

// InternalIntervals 当前周期配置（defID → 小时）
func InternalIntervals() map[string]float64 {
	intervalMu.Lock()
	defer intervalMu.Unlock()
	out := map[string]float64{}
	for id, h := range intervals {
		out[id] = h
	}
	return out
}

// RunIntervalTick 检查到点的周期任务（主控 goroutine 每 60s 调一次）
// 返回: 本次到点需要触发的 defID 列表（触发后更新 last——避免重复）
func RunIntervalTick() []string {
	intervalMu.Lock()
	defer intervalMu.Unlock()
	var due []string
	now := time.Now()
	for id, h := range intervals {
		last, ok := intervalLast[id]
		if !ok || now.Sub(last) >= time.Duration(h*float64(time.Hour)) {
			due = append(due, id)
			intervalLast[id] = now
		}
	}
	if len(due) > 0 {
		saveIntervals()
	}
	return due
}

// InternalIntervalHandler 设置周期
// POST /api/internal-tasks/{id}/interval  {"hours": N}
func (h *Handlers) InternalIntervalHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	var req struct {
		Hours float64 `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_PARAMS", "参数解析失败: "+err.Error())
		return
	}
	if req.Hours < 0 || req.Hours > 24*30 {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_INTERVAL", "周期范围: 0（取消）~ 720 小时")
		return
	}
	SetInternalInterval(taskID, req.Hours)
	if req.Hours <= 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "id": taskID, "hours": 0, "message": "周期已取消", "state": InternalEngineStateMap()})
	} else {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "id": taskID, "hours": req.Hours, "message": "周期已设置: 每 " + formatHours(req.Hours) + " 触发", "state": InternalEngineStateMap()})
	}
}

// InternalIntervalsHandler 查询周期
// GET /api/internal-tasks/intervals
func (h *Handlers) InternalIntervalsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"intervals": InternalIntervals(),
		"note":      "内部任务循环周期（小时）——0/不存在=未设置",
		"state":     InternalEngineStateMap(),
	})
}

// formatHours 小时人性化（24→"24 小时"——0.5→"30 分钟"）
func formatHours(h float64) string {
	if h < 1 {
		return fmtDuration(h * 60)
	}
	if h == float64(int(h)) {
		return fmt.Sprintf("%d 小时", int(h))
	}
	return fmt.Sprintf("%.1f 小时", h)
}

func fmtDuration(min float64) string {
	if min == float64(int(min)) {
		return fmt.Sprintf("%d 分钟", int(min))
	}
	return fmt.Sprintf("%.0f 分钟", min)
}
