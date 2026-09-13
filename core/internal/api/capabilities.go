package api

// capabilities.go — 接口可发现性（2026-08-22 Mr2109）
// 其他智能体调用虫族：/api/capabilities（能力清单）+ /api/openapi.json（端点规范）+ /api/help（调用示例）
// 目的：别的 agent 不用读代码——curl 端点就知道虫族能做什么、怎么调用

import (
	"encoding/json"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
	"net/http"
)

// CapabilitiesHandler — 能力清单（虫族能做什么——其他 agent 发现用）
// GET /api/capabilities
func (h *Handlers) CapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"name":    "虫族 Zerg",
		"version": version.Tag,
		// 代码身份（自动升级模块：verify 阶段比对"活进程 vs 目标制品"——混版必须可见）
		"code_sha":    version.Commit,
		"build_time":  version.BuildTime,
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
			// 2026-09-13 文件/目录浏览器 阶段 1（《设计-文件浏览器集装箱-20260913》§4.2）：
			// 三条端点已在 main.go 注册，能力清单也必须能发现它们——别的 agent 只 curl 这一份清单。
			{"name": "list_fileroots", "desc": "文件浏览器：五根白名单（docs/repo/models/tasks/weights）+ 可配置项；每根含 exists（根不存在也返回该项，exists=false）", "endpoint": "GET /api/fileroots"},
			{"name": "open_file", "desc": "用默认应用打开根内目录 / text_exts 内文本文件（白名单校验 + 审计留痕）", "endpoint": "POST /api/fileroots/open"},
			{"name": "reveal_file", "desc": "在访达中显示根内任意类型文件或目录（白名单校验 + 审计留痕；不执行不解析）", "endpoint": "POST /api/fileroots/reveal"},
		},
		"workflow": "提交任务 → CA 执行（worktree git）→ 确定性验证（机器检查）→ 复查模型（跨家族）→ 通过 merge/打回重做",
		"notes":    "单槽铁律: 单设备串行——排队慢正常（等更多 X3 并行）",
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
			"version":     version.Version,
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
			"/api/fleet/models":     map[string]interface{}{"get": map[string]interface{}{"summary": "模型清单"}},
			"/api/fleet/status":     map[string]interface{}{"get": map[string]interface{}{"summary": "集群状态"}},
			"/api/resources/{type}": map[string]interface{}{"get": map[string]interface{}{"summary": "资源库（models/tools/skills/mcp）"}},
			// 2026-09-13 文件/目录浏览器 阶段 1：三端点如实补录（字段/错误码按 fileroots.go 实现，不编造）
			"/api/fileroots": map[string]interface{}{"get": map[string]interface{}{"summary": "文件根白名单", "description": "五项白名单根（docs/repo/models/tasks/weights）+ config（display_max/allow_all_types/text_exts）；每根含 id/label/path/default/writable/exists——根不存在也照常返回该项，exists=false"}},
			"/api/fileroots/open": map[string]interface{}{"post": map[string]interface{}{"summary": "用默认应用打开", "description": "目录一律放行；文件须在 text_exts 内（ZERG_FILEBROWSER_ALLOW_ALL_TYPES=1 放开任意类型）。成功返回 {\"ok\":true,\"abs\":\"<后端解析出的绝对路径>\"}；错误码 INVALID_BODY/INVALID_ROOT/INVALID_PATH/NOT_ALLOWED（400）、NOT_FOUND（404）、OPEN_FAILED（500）", "requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"root": map[string]interface{}{"type": "string", "description": "白名单根 id（docs/repo/models/tasks/weights）"},
				"path": map[string]interface{}{"type": "string", "description": "根内相对路径（空=根本身；不接受绝对路径与 .. 段）"},
				"mode": map[string]interface{}{"type": "string", "description": "file|dir（契约字段；后端以实际 stat 为准，不信前端声明）"},
			}}}}}}},
			"/api/fileroots/reveal": map[string]interface{}{"post": map[string]interface{}{"summary": "在访达中显示", "description": "对任意类型放行（只打开文件管理器并高亮，不执行不解析）。成功与错误码同 /api/fileroots/open（无类型闸门，故不会出现 NOT_ALLOWED）", "requestBody": map[string]interface{}{"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"root": map[string]interface{}{"type": "string", "description": "白名单根 id"},
				"path": map[string]interface{}{"type": "string", "description": "根内相对路径（空=根本身）"},
			}}}}}}},
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

【文件/目录浏览器（阶段 1，2026-09-13）】
- 根白名单（只读；每根含 exists，根不存在也返回该项且 exists=false）:
curl http://127.0.0.1:8580/api/fileroots -H "X-Auth-Token: ***"
→ roots: docs(可写)/repo/models/tasks/weights；config: display_max / allow_all_types / text_exts
- 用默认应用打开（目录、或 text_exts 内的文本文件）:
curl -X POST http://127.0.0.1:8580/api/fileroots/open -H "X-Auth-Token: ***" -H "Content-Type: application/json" \
  -d '{"root":"docs","path":"INDEX.md"}'
→ {"ok":true,"abs":"<后端解析出的绝对路径>"}；错误码 INVALID_ROOT/INVALID_PATH/NOT_ALLOWED/NOT_FOUND/OPEN_FAILED
- 在访达中显示（任意类型，只高亮不执行）:
curl -X POST http://127.0.0.1:8580/api/fileroots/reveal -H "X-Auth-Token: ***" -H "Content-Type: application/json" \
  -d '{"root":"weights","path":"m.gguf"}'
- 读文件（参数化；不带查询参数时仍返回既有 docs 行为）:
curl "http://127.0.0.1:8580/api/docs?root=weights&path=m.gguf" -H "X-Auth-Token: ***"
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"help": help})
}
