package api

// capabilities.go — 接口可发现性（2026-08-22 Mr2109）
// 其他智能体调用虫族：/api/capabilities（能力清单）+ /api/openapi.json（端点规范）+ /api/help（调用示例）
// 目的：别的 agent 不用读代码——curl 端点就知道虫族能做什么、怎么调用

import (
	"encoding/json"
	"net/http"
)

// CapabilitiesHandler — 能力清单（虫族能做什么——其他 agent 发现用）
// GET /api/capabilities
func (h *Handlers) CapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"name":        "虫族 Zerg",
		"version":     "v2.5.5",
		"description": "去中心化 AI 任务网络——主控调度 + CA 执行 + 复查验证",
		"auth":        "X-Auth-Token 请求头（网关/主控配置的 token）",
		"base_url":    "http://<主控地址>:8580",
		"capabilities": []map[string]interface{}{
			{"name": "submit_task", "desc": "提交任务（CA 执行——模型干活）", "endpoint": "POST /api/tasks", "example": `curl -X POST http://127.0.0.1:8580/api/tasks -H "X-Auth-Token: <token>" -H "Content-Type: application/json" -d '{"description":"任务描述","model":"example-35b-v2","priority":3}'`},
			{"name": "list_tasks", "desc": "查看任务队列（所有状态）", "endpoint": "GET /api/tasks"},
			{"name": "task_detail", "desc": "任务详情（状态/报告/轮次）", "endpoint": "GET /api/tasks/{id}"},
			{"name": "task_retry", "desc": "重跑任务（failed→queued）", "endpoint": "POST /api/tasks/{id}/retry"},
			{"name": "task_pause", "desc": "暂停/继续排队任务", "endpoint": "POST /api/tasks/{id}/pause?pause=true"},
			{"name": "task_move", "desc": "重排任务（置顶/上移/下移）", "endpoint": "POST /api/tasks/{id}/move?action=top"},
			{"name": "task_delete", "desc": "删除排队任务", "endpoint": "DELETE /api/tasks/{id}"},
			{"name": "list_models", "desc": "可用模型清单（含候选机器）", "endpoint": "GET /api/fleet/models"},
			{"name": "fleet_status", "desc": "集群机器状态（健康/负载）", "endpoint": "GET /api/fleet/status"},
			{"name": "list_resources", "desc": "资源库（模型/工具/skill/mcp——信任度）", "endpoint": "GET /api/resources/{type}"},
			{"name": "list_docs", "desc": "项目文档（设计/计划）", "endpoint": "GET /api/docs"},
			{"name": "git_status", "desc": "任务 git 状态（分支/diff）", "endpoint": "GET /api/git/status"},
		},
		"workflow": "提交任务 → CA 执行（worktree git）→ 确定性验证（机器检查）→ 复查模型（跨家族）→ 通过 merge/打回重做",
		"notes": "单槽铁律: 单设备串行——排队慢正常（等更多 X3 并行）",
	}
	writeJSON(w, http.StatusOK, resp)
}

// OpenAPIHandler — OpenAPI 规范（端点全描述——其他 agent 自动发现）
// GET /api/openapi.json
func (h *Handlers) OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "虫族 Zerg API",
			"version":     "2.5.5",
			"description": "去中心化 AI 任务网络主控 API——提交/查询/管理任务",
		},
		"paths": map[string]interface{}{
			"/api/tasks": map[string]interface{}{
				"post": map[string]interface{}{"summary": "提交任务", "description": "提交任务——CA 执行", "requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{
					"description": map[string]interface{}{"type": "string", "description": "任务描述（自包含）"},
					"model":       map[string]interface{}{"type": "string", "description": "模型名（不传=默认调度）"},
					"priority":    map[string]interface{}{"type": "integer", "description": "优先级（默认 3——高优先小）"},
				}}}}}},
				"get": map[string]interface{}{"summary": "任务列表", "description": "所有任务（running/queued/done/failed）"},
			},
			"/api/tasks/{id}": map[string]interface{}{
				"get": map[string]interface{}{"summary": "任务详情", "description": "状态/执行报告/复查报告/轮次"},
			},
			"/api/fleet/models": map[string]interface{}{"get": map[string]interface{}{"summary": "模型清单"}},
			"/api/fleet/status": map[string]interface{}{"get": map[string]interface{}{"summary": "集群状态"}},
			"/api/resources/{type}": map[string]interface{}{"get": map[string]interface{}{"summary": "资源库（models/tools/skills/mcp）"}},
		},
		"security": []map[string]interface{}{
			{"X-Auth-Token": []string{}},
		},
	}
	writeJSON(w, http.StatusOK, spec)
}

// HelpHandler — 调用示例（curl 就能看——人类/agent 快速上手）
// GET /api/help
func (h *Handlers) HelpHandler(w http.ResponseWriter, r *http.Request) {
	help := `虫族 Zerg API 快速上手
========================

【认证】
所有请求带请求头: X-Auth-Token: <token>

【提交任务——CA 执行】
curl -X POST http://127.0.0.1:8580/api/tasks \
  -H "X-Auth-Token: <token>" -H "Content-Type: application/json" \
  -d '{"description":"你的任务描述","model":"example-35b-v2","priority":3}'
→ 返回 task_id——用 task_id 查进度

【查任务状态】
curl http://127.0.0.1:8580/api/tasks/<task_id> -H "X-Auth-Token: <token>"
→ status: queued(排队)/running(执行中)/done(完成)/failed(失败)
→ exec_report: 执行报告全文 / review_report: 复查报告全文

【可用模型】
curl http://127.0.0.1:8580/api/fleet/models -H "X-Auth-Token: <token>"

【任务管理】
- 重跑: curl -X POST http://127.0.0.1:8580/api/tasks/<id>/retry -H "X-Auth-Token: <token>"
- 暂停: curl -X POST "http://127.0.0.1:8580/api/tasks/<id>/pause?pause=true" -H "X-Auth-Token: <token>"
- 置顶: curl -X POST "http://127.0.0.1:8580/api/tasks/<id>/move?action=top" -H "X-Auth-Token: <token>"
- 删除: curl -X DELETE http://127.0.0.1:8580/api/tasks/<id> -H "X-Auth-Token: <token>"

【模型调用（不走任务——直接对话）】
curl http://127.0.0.1:8082/v1/responses -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"model":"example-35b-v2","input":[{"role":"user","content":"你好"}],"stream":true}'

【更多】/api/capabilities（能力清单）+ /api/openapi.json（OpenAPI 规范）
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"help": help})
}
