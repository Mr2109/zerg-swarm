package api

// internal_mode.go — 内部任务运行模式开关（2026-08-28 Mr2109）
// 每个内部任务: 自动运行（auto——编排自动触发）/ 手动运行（manual——只手动触发）
// 默认 auto（兼容现状——16 类全自动）——切换后持久化——重启恢复
// POST /api/internal-tasks/{id}/mode  {"auto_run": true|false}  设置运行模式
// GET  /api/internal-tasks/modes                               查询当前模式
// 持久化 <任务目录根>/internal_modes.json（ZERG_TASK_ROOT 可覆盖，默认 /tmp/zerg-tasks）——重启恢复

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/go-chi/chi/v5"
)

var (
	modeMu sync.Mutex
	modes  = map[string]bool{} // defID → auto_run（默认 true——不存在=true）
	// 待修补 #36: 任务目录根不再写死——经 statepath.TaskRoot() 解析（ZERG_TASK_ROOT 可覆盖，默认 /tmp/zerg-tasks）
	modeFile = filepath.Join(statepath.TaskRoot(), "internal_modes.json")
)

// loadModes 启动恢复（文件不存在忽略——全默认 auto）
func loadModes() {
	data, err := os.ReadFile(modeFile)
	if err != nil {
		return
	}
	var raw map[string]bool
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	modeMu.Lock()
	defer modeMu.Unlock()
	for id, auto := range raw {
		modes[id] = auto
	}
}

// saveModes 持久化（调用方持锁）
func saveModes() {
	data, _ := json.MarshalIndent(modes, "", "  ")
	_ = os.MkdirAll(filepath.Dir(modeFile), 0o755)
	_ = os.WriteFile(modeFile, data, 0o644)
}

// SetInternalMode 设置运行模式（auto_run=false = 手动运行）
func SetInternalMode(defID string, autoRun bool) {
	modeMu.Lock()
	defer modeMu.Unlock()
	if autoRun {
		delete(modes, defID) // auto=默认——删除即可（IsInternalAuto 默认 true）
	} else {
		modes[defID] = false
	}
	saveModes()
}

// IsInternalAuto 该内部任务是否自动运行（默认 true——未设置=auto）
func IsInternalAuto(defID string) bool {
	modeMu.Lock()
	defer modeMu.Unlock()
	auto, ok := modes[defID]
	if !ok {
		return true
	}
	return auto
}

// InternalModes 当前模式配置（defID → auto_run）
func InternalModes() map[string]bool {
	modeMu.Lock()
	defer modeMu.Unlock()
	out := map[string]bool{}
	for id, auto := range modes {
		out[id] = auto
	}
	return out
}

// InternalModeHandler 设置运行模式
// POST /api/internal-tasks/{id}/mode  {"auto_run": true|false}
func (h *Handlers) InternalModeHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	var req struct {
		AutoRun bool `json:"auto_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_PARAMS", "参数解析失败: "+err.Error())
		return
	}
	SetInternalMode(taskID, req.AutoRun)
	mode := "自动运行"
	if !req.AutoRun {
		mode = "手动运行"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"id":       taskID,
		"auto_run": req.AutoRun,
		"message":  "内部任务 " + taskID + " 已切换为" + mode,
		"state":    InternalEngineStateMap(), // 2026-09-10：变更即回状态（契约统一）
	})
}

// InternalModesHandler 查询运行模式
// GET /api/internal-tasks/modes
func (h *Handlers) InternalModesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modes": InternalModes(),
		"note":  "内部任务运行模式——true=自动运行（编排触发）——false=手动运行（只手动触发）——未列出=默认自动",
	})
}

// InitInternalModes 启动时加载持久化模式（main.go 调用）
func InitInternalModes() {
	loadModes()
}
