package api

// internal_tasks_api.go — 内部任务 API（2026-08-22 Mr2109补充）
// GET /api/internal-tasks —— 内部任务清单（16 类——ID/描述/冷却——含最近执行）
// POST /api/internal-tasks/{id}/run —— 手动执行内部任务（Mr2109——UI 按钮）

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/go-chi/chi/v5"
)

// InternalTasksHandler 内部任务清单（2026-08-22 Mr2109——UI 分类显示）
// GET /api/internal-tasks
// v2.5.6 增强（2026-08-27 Mr2109）: 每类带 skill 内容（该类最近任务的 SKILL.md——新流程任务沉淀）
func (h *Handlers) InternalTasksHandler(w http.ResponseWriter, r *http.Request) {
	defs := agent.ListInternalTasks()
	items := make([]map[string]interface{}, 0, len(defs))
	for _, d := range defs {
		// 该类最近任务的 skill 内容（glob internal-<id>-*/SKILL.md——最新优先）
		skill := ""
		matches, _ := filepath.Glob(filepath.Join(statepath.TaskRoot(), "internal-"+d.ID+"-*", "SKILL.md"))
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				fi, erri := os.Stat(matches[i])
				fj, errj := os.Stat(matches[j])
				if erri != nil || errj != nil {
					return false
				}
				return fi.ModTime().After(fj.ModTime())
			})
			if data, err := os.ReadFile(matches[0]); err == nil {
				skill = string(data)
			}
		}
		items = append(items, map[string]interface{}{
			"id":            d.ID,
			"description":   d.Description,
			"template":      d.Template,
			"cooldown":      d.Cooldown.String(),
			"default_hours": d.Cooldown.Hours(),   // v2.5.6: 默认执行周期（Mr2109——UI 周期下拉默认显示）
			"skill":         skill,                // 该类最近任务的 skill（无=空——模型自举中）
			"auto_run":      IsInternalAuto(d.ID), // v2.5.6: 运行模式（true=自动——编排触发——false=手动——只手动触发——Mr2109 2026-08-28）
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(items),
		"items": items,
		"note":  "内部任务=进化任务（为自己）——编排自动运行——也可手动执行（POST /api/internal-tasks/{id}/run）",
	})
}

// InternalTaskRunHandler 手动执行内部任务（2026-08-22 Mr2109——UI 按钮）
// POST /api/internal-tasks/{id}/run
func (h *Handlers) InternalTaskRunHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	taskID := chi.URLParam(r, "id")
	// 找内部任务定义
	defs := agent.ListInternalTasks()
	var found *agent.InternalTask
	for i := range defs {
		if defs[i].ID == taskID {
			found = &defs[i]
			break
		}
	}
	if found == nil {
		writeErrorCode(w, http.StatusNotFound, "INTERNAL_TASK_NOT_FOUND", "内部任务不存在: "+taskID)
		return
	}
	if os.Getenv("ZERG_INTERNAL_TASKS") != "1" {
		writeErrorCode(w, http.StatusForbidden, "INTERNAL_TASKS_DISABLED", "内部任务引擎已停用（2026-09-06 误删事故）——设 ZERG_INTERNAL_TASKS=1 并重启主控后可运行")
		return
	}
	// 提交任务（内部——优先级 5——模型=内部任务默认（新流程须模型名——网关路由））
	// v2.5.6 Mr2109: 内部任务走新机制（Flow=zerg——程序定量驱动）
	task := &Task{
		ID:          fmt.Sprintf("internal-%s-%d", found.ID, time.Now().UnixNano()),
		Description: found.Template,
		Type:        "internal",
		Priority:    5,
		Status:      "queued",
		Model:       "Qwen3.8-27B", // 内部任务模型池第一个（自动触发走轮换——手动执行固定测试）
		Flow:        "zerg",        // v2.5.6: 新流程（状态机——方案/选定/执行/封闭）
		SkillKey:    found.ID,      // v2.5.6: skill 归属=任务独属（def.ID）
		CreatedAt:   time.Now(),
	}
	h.Scheduler.Submit(task)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"task_id":   task.ID,
		"type":      taskID,
		"message":   fmt.Sprintf("内部任务 %s 已提交（排队执行）", taskID),
		"submitted": time.Now().Format(time.RFC3339),
	})
}
