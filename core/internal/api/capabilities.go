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
		// 2026-09-20 批 B ②：能力清单由**真源表**派生（routes.go 的 routeTable）——
		// 与 /api/openapi.json 的 paths **同一张表**，逐条对齐（含**本清单自己**）。
		"capabilities":     capabilityItems(),
		"capability_count": len(routeTable),
		"workflow":         "提交任务 → CA 执行（worktree git）→ 确定性验证（机器检查）→ 复查模型（跨家族）→ 通过 merge/打回重做",
		"notes":            "单槽铁律: 单设备串行——排队慢正常（等更多 X3 并行）",
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
		// 2026-09-20 批 B ②：paths 由**真源表**派生（routes.go）——键集与 capabilities 的端点集逐条相等。
		"paths": openapiPaths(),
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
