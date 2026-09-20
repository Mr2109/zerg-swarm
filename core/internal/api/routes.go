package api

// routes.go — 对外端点的**唯一真源**（2026-09-20 批 B · ②「能力清单列出自己且与路由表对齐」）
//
// 为什么需要这一张表：同一份「虫族能做什么」先前写在**两处**（`capabilities[]` 与 OpenAPI 的
// `paths`），两处各写各的 ⇒ 天然对不齐（2026-09-20 实测：**17 条能力 vs 8 条路径**），而且
// 能力快照**不列自己**（`/api/capabilities` 与 `/api/openapi.json` 在清单里没有条目）⇒
// 别的 agent 只 curl 这一份清单时，看不到自描述面本身。
//
// 处置：**一张表 = 真源**，两个 handler 都从它**派生** ⇒ 两面同源由构造保证，不再靠人眼对。
//   ★ 同一条纪律见契约 §十（`K13` / `U2` / `U4`）：命令清单 · `/api/capabilities` ·
//     `/api/openapi.json` 三面同源、字段差集为空。
//   ★ 与**主控真实路由表**（`core/cmd/zerg-core/main.go` 的 `r.Get/Post/Delete`）逐条对齐：
//     由 `routes_test.go` 扫真实注册行、逐条核对（表里每个端点都必须在真实路由表里注册过；
//     任一条查不到即红）。**表不许出现幻影端点** —— 这也是本表存在的第二重意义。

import "strings"

// routeEntry 一条对外端点：能力清单与 OpenAPI 路径表都由它派生。
type routeEntry struct {
	Name    string // 能力名（`capabilities[].name`）
	Desc    string // 一句话说明（两面的 summary 共用）
	Method  string // GET / POST / DELETE（逐字取自真实注册行）
	Path    string // 路径（不含方法；`{id}` 这类占位符照真实路由逐字写）
	Example string // 可选：curl 示例（只有需要点明调用形状的才给）
}

// routeTable —— 对外端点真源（**顺序即人面顺序**，加条目请一并想清「它属于哪个族」）。
//
// 三条边界（不许越）：
//
//	① 只收**对外发现面**：`/api/internal-tasks/*`、`/api/archive`、`/api/logs/*` 这类主控内部面
//	   不进表（别的 agent 不需要靠它发现虫族能做什么）；
//	② 每条路径必须是**真实注册过的**（见 routes_test.go 的逐条核对）；
//	③ 设计稿承诺过而代码没有的端点**不许写成幻影**（原先 `resource_fit` / `resource_residency`
//	   就是这么处理的：如实给出映射到 ledger 的那条真实路由，并在 Desc 里写明「原设计承诺的
//	   /api/resources/fit 未实现」）。
var routeTable = []routeEntry{
	// ── task 族（提交与队列管理）─────────────────────────────────────────
	{Name: "submit_task", Desc: "提交任务（CA 执行——模型干活）", Method: "POST", Path: "/api/tasks",
		Example: `curl -X POST http://127.0.0.1:8580/api/tasks -H "X-Auth-Token: <令牌>" -H "Content-Type: application/json" -d '{"description":"任务描述","model":"example-35b-v2","priority":3}'`},
	{Name: "list_tasks", Desc: "查看任务队列（所有状态）", Method: "GET", Path: "/api/tasks"},
	{Name: "task_detail", Desc: "任务详情（状态/报告/轮次）", Method: "GET", Path: "/api/tasks/{id}"},
	{Name: "task_retry", Desc: "重跑任务（failed→queued）", Method: "POST", Path: "/api/tasks/{id}/retry"},
	{Name: "task_pause", Desc: "暂停/继续排队任务", Method: "POST", Path: "/api/tasks/{id}/pause"},
	{Name: "task_move", Desc: "重排任务（置顶/上移/下移）", Method: "POST", Path: "/api/tasks/{id}/move"},
	{Name: "task_delete", Desc: "删除排队任务", Method: "DELETE", Path: "/api/tasks/{id}"},
	// ── fleet 族（机群与模型）───────────────────────────────────────────
	{Name: "list_models", Desc: "可用模型清单（含候选机器）", Method: "GET", Path: "/api/fleet/models"},
	{Name: "fleet_status", Desc: "集群机器状态（健康/负载）", Method: "GET", Path: "/api/fleet/status"},
	// ── resource / doc / git 族 ────────────────────────────────────────
	{Name: "list_resources", Desc: "资源库（模型/工具/skill/mcp——信任度）", Method: "GET", Path: "/api/resources/{type}"},
	{Name: "list_docs", Desc: "项目文档（设计/计划）；带 root=/path= 查询参数可参数化读白名单内的文件", Method: "GET", Path: "/api/docs"},
	{Name: "git_status", Desc: "任务 git 状态（分支/diff）", Method: "GET", Path: "/api/git/status"},
	// ── 文件/目录浏览器 阶段 1（2026-09-13《设计-文件浏览器虫茧-20260913》§4.2）──
	{Name: "list_fileroots", Desc: "文件浏览器：五根白名单（docs/repo/models/tasks/weights）+ 可配置项；每根含 exists（根不存在也返回该项，exists=false）", Method: "GET", Path: "/api/fileroots"},
	{Name: "open_file", Desc: "用默认应用打开根内目录 / text_exts 内文本文件（白名单校验 + 审计留痕）", Method: "POST", Path: "/api/fileroots/open"},
	{Name: "reveal_file", Desc: "在访达中显示根内任意类型文件或目录（白名单校验 + 审计留痕；不执行不解析）", Method: "POST", Path: "/api/fileroots/reveal"},
	// ⚠ 2026-09-13（Mr2109「按建议」）：设计稿曾承诺 `/api/resources/fit` 与 `/api/resources/residency`，
	//   代码未实现（请求返回 400）。按「代码为准、回填文档」的规矩不另设端点——数据统一在 ledger，
	//   这里如实给出映射，免得外部 agent 按设计稿去找两个不存在的地址。
	{Name: "resource_fit", Desc: "资源是否装得下（原设计承诺的 /api/resources/fit 未实现——数据见 ledger；设计稿已回填）", Method: "GET", Path: "/api/resources/{type}"},
	{Name: "resource_residency", Desc: "驻留与未托管模型（原设计承诺的 /api/resources/residency 未实现——数据见 ledger；设计稿已回填）", Method: "GET", Path: "/api/resources/{type}"},
	// ── 自描述族（2026-09-20 批 B ②：**清单必须列出自己**）────────────────
	//   为什么必须列自己：外部 agent 只 curl 这一份清单时，自描述三面（能力/规范/示例）正是它
	//   接着要打的三条路；不列 = 清单自己把「续路」断了（实测差集 2 条正来自这里）。
	{Name: "discover_capabilities", Desc: "能力清单（本端点自身——虫族能做什么）", Method: "GET", Path: "/api/capabilities"},
	{Name: "discover_openapi", Desc: "OpenAPI 3.0 规范（端点全描述，供自动发现）", Method: "GET", Path: "/api/openapi.json"},
	{Name: "discover_help", Desc: "调用示例（curl 就能看——人类/agent 快速上手）", Method: "GET", Path: "/api/help"},
}

// objKV 是 composite literal 的小助手：objKV("a", 1, "b", 2) ⇒ map{"a":1,"b":2}。
// 为什么用它：OpenAPI 的请求体 schema 是深嵌套 map，逐个写 `map[string]interface{}{…}`
// 会把花括号数到看错（本文件第一版就写坏过一次）——键值成对写，一眼数得清。
func objKV(kv ...interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// capabilityItems 由真源表派生能力清单条目（`capabilities[]`）。
// 字段名沿用既有四面（`name`/`desc`/`endpoint`[/`example`]）——§4.3 `U2`：`--json` 的字段名
// 与本清单同源，**不在这里另造第二套词汇**。
func capabilityItems() []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(routeTable))
	for _, e := range routeTable {
		it := objKV(
			"name", e.Name,
			"desc", e.Desc,
			"endpoint", e.Method+" "+e.Path,
		)
		if e.Example != "" {
			it["example"] = e.Example
		}
		items = append(items, it)
	}
	return items
}

// routePaths 真源表的**去重路径集**（保序）。
func routePaths() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(routeTable))
	for _, e := range routeTable {
		if seen[e.Path] {
			continue
		}
		seen[e.Path] = true
		out = append(out, e.Path)
	}
	return out
}

// openapiPathDetail —— 少数端点需要**写实的 schema/描述**（请求体字段、错误码），单独放详表。
//
// 与真源表的关系：详表只提供「这条路径上某个方法怎么写」，**路径集由真源表决定** ——
// 详表里出现真源表没有的路径会被 `routes_test.go` 判红（防详表长出幻影端点）。
var openapiPathDetail = map[string]map[string]interface{}{
	"/api/tasks": {
		"post": objKV(
			"summary", "提交任务",
			"description", "提交任务——CA 执行",
			"requestBody", objKV("content", objKV("application/json", objKV("schema", objKV(
				"type", "object",
				"properties", objKV(
					"description", objKV("type", "string", "description", "任务描述（自包含）"),
					"model", objKV("type", "string", "description", "模型名（不传=默认调度）"),
					"priority", objKV("type", "integer", "description", "优先级（默认 3——高优先小）"),
					// B 项③ 片（单子）最小 schema（2026-09-18）: 声明了其中任一字段 ⇒ 该任务按「片」过挂板校验
					// （缺 slice_id / 缺 acceptance 声明 / depends_on 环 / 悬空依赖 ⇒ 400，拒绝入队）。
					// 未声明片字段的任务不受影响（现有调用方行为不变）。
					"slice_id", objKV("type", "string", "description", "片（单子）真源 id；声明了任一 slice 字段就必须给出，否则 400 SLICE_MISSING_ID"),
					"depends_on", objKV("type", "array", "items", objKV("type", "string"), "description", "依赖的其它片 id 列表；成环 ⇒ 400 SLICE_DEPENDS_CYCLE；指向板上不存在的片 ⇒ 400 SLICE_DEPENDS_DANGLING"),
					"owner", objKV("type", "string", "description", "片归属者（可选——本轮不参与判定，只随片记录）"),
					"acceptance", objKV("type", "array", "items", objKV("type", "string"), "description", "验收判据；**必须显式声明**（缺失 ⇒ 400 SLICE_MISSING_ACCEPTANCE；显式空数组 [] 允许——「没写」与「写了空」是两件事）"),
				),
			)))),
		),
		"get": objKV("summary", "任务列表", "description", "所有任务（running/queued/done/failed）"),
	},
	"/api/tasks/{id}": {
		"get": objKV("summary", "任务详情", "description", "状态/执行报告/复查报告/轮次"),
	},
	// 2026-09-13 文件/目录浏览器 阶段 1：两括号端点如实补录（字段/错误码按 fileroots.go 实现，不编造）
	"/api/fileroots/open": {
		"post": objKV(
			"summary", "用默认应用打开",
			"description", "目录一律放行；文件须在 text_exts 内（ZERG_FILEBROWSER_ALLOW_ALL_TYPES=1 放开任意类型）。成功返回 {\"ok\":true,\"abs\":\"<后端解析出的绝对路径>\"}；错误码 INVALID_BODY/INVALID_ROOT/INVALID_PATH/NOT_ALLOWED（400）、NOT_FOUND（404）、OPEN_FAILED（500）",
			"requestBody", objKV("content", objKV("application/json", objKV("schema", objKV(
				"type", "object",
				"properties", objKV(
					"root", objKV("type", "string", "description", "白名单根 id（docs/repo/models/tasks/weights）"),
					"path", objKV("type", "string", "description", "根内相对路径（空=根本身；不接受绝对路径与 .. 段）"),
					"mode", objKV("type", "string", "description", "file|dir（契约字段；后端以实际 stat 为准，不信前端声明）"),
				),
			)))),
		),
	},
	"/api/fileroots/reveal": {
		"post": objKV(
			"summary", "在访达中显示",
			"description", "对任意类型放行（只打开文件管理器并高亮，不执行不解析）。成功与错误码同 /api/fileroots/open（无类型闸门，故不会出现 NOT_ALLOWED）",
			"requestBody", objKV("content", objKV("application/json", objKV("schema", objKV(
				"type", "object",
				"properties", objKV(
					"root", objKV("type", "string", "description", "白名单根 id"),
					"path", objKV("type", "string", "description", "根内相对路径（空=根本身）"),
				),
			)))),
		),
	},
}

// openapiPaths 由真源表 + 详表派生 OpenAPI 的 `paths`。
// 判据（routes_test.go 钉住）：`keys(paths) == routePaths()` —— 两面路径**逐条相等**，差集为空。
func openapiPaths() map[string]interface{} {
	out := map[string]interface{}{}
	for p, methods := range openapiPathDetail {
		m := map[string]interface{}{}
		for k, v := range methods {
			m[strings.ToLower(k)] = v
		}
		out[p] = m
	}
	for _, e := range routeTable {
		m, _ := out[e.Path].(map[string]interface{})
		if m == nil {
			m = map[string]interface{}{}
			out[e.Path] = m
		}
		k := strings.ToLower(e.Method)
		if _, ok := m[k]; !ok {
			m[k] = objKV("summary", e.Desc)
		}
	}
	return out
}
