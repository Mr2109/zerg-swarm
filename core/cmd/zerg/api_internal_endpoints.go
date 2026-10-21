// api_internal_endpoints.go —— `api` 族的第二来源：② 保留为内部实现的 HTTP 端点（**只读投影**）。
//
// 为什么需要这一张表：`zerg api ls` / `zerg api openapi` 现读只投影主控的**对外发现面**
// （`/api/capabilities` · `/api/openapi.json`；真源 = `core/internal/api/routes.go` 的 routeTable）。
// 而仓里另有一批**已注册、但按设计不进对外发现面**的 HTTP 端点（归属 ② 档）⇒ 从命令面看它们
// **完全看不见**（既不在能力面、也不在路径面）。本表把这一批**列出来**（只列 · 不执行）。
//
// 归属真源（逐条照抄 · 一件不自编）：
//
//	· `Zerg-内部文档/项目文档/v2.5.10/设计-命令面与契约-v0.7.md` §5.1（HTTP 面逐条 · 每条给归属与理由）；
//	· `core/internal/contract/adopt-retire.json` 的 decisions：
//	  决议①「对话面 19 条 ⇒ 不开命令面」（verdict = 保留为内部实现（②）· landing = 只登记归属 · 不建命令）；
//	  ★ 2026-10-17 新令（`P-1` 乙案 · 决议①**收窄保留** · 旧行原样留档）：只禁「**裸 HTTP 转发命令面**」与通用 `api call`；
//	  19 条**仍不各自开命令**（本族一条不新增）⇒ **人类正门 = `zerg chat`**（独立新族 · 只经正门 · ✗ 不开通用转发）。
//	  决议③「宿主文件浏览器 4 条 ⇒ 随文件浏览器虫茧迁走」（归属写「迁往虫茧」＝ ② 的迁移态）；
//	  决议⑨「`api call` 写口 ⇒ 维持不开」。
//
// 形态（照同仓 ② 类先例 `core/cmd/zerg/family_eval.go`：① 收编／② 保留为内部实现／④ 待定）：
//
//	① **只做投影**：本表只把 ② 端点**列出来**（`api ls` 一行一件 · `api openapi` 一行一路径×方法）；
//	② **✗ 不开执行面**：`api call` 一类通用逃生门一条都不开（决议⑨逐字「维持不开」）；
//	③ **✗ 不复算底层逻辑**：本表只有「方法 · 路径 · 归属 · 出处」四样，壳里不算任何业务
//	   （「收编即两份真相」的反面）。
//
// ★ 三条硬边界：
//
//	① **底层件一字节不改**：本表只**投影**注册面；`core/internal/api/chat_handlers.go` ·
//	  `core/cmd/zerg-core/main.go` · `core/internal/gateway/gateway.go` 三个注册件一个字都不动。
//	  表与注册面的**双向可查**由 `api_internal_endpoints_test.go` 逐条现读核对给出
//	  （注册面少一条 ⇒ 该测红；表里多一条 ⇒ 同红）。
//	② **同一端点只出一次**：早经 `routeTable` 进了对外发现面的端点（如宿主文件浏览器三条）本表
//	  照样收（归属与出处要留痕），但**投影时按 (方法, 路径) 去重** ⇒ 一次输出里同一端点只出现一次。
//	③ **本表不新立族、不新立命令**：本表挂**既有** `api` 族的**既有**两条只读动作，命令树一个节点都不新增
//	  （★ 2026-10-17 新令 · `P-1` 乙案：**正门 = `zerg chat`** 是**另立的新族**、判据自持 ⇒ 与本表互不冒充；
//	   本表仍只出「方法 · 路径 · 归属 · 出处」四样 · ✗ 不开通用转发 —— 甲案的错处正是把这条防线一并删掉）。
package main

import "strings"

// 归属闭集（与 `family_eval.go` 的 ①②④ 闭集同形 —— 本表只收 ②）。
// 每条取值逐字对应上面三处真源里那一档的判词，✗ 不另造词汇。
const (
	apiInternalClassChat     = "② 保留为内部实现（决议①收窄保留：✗ 不开裸 HTTP 转发命令面 · 人类正门 = `zerg chat` · 命令面只做投影）"
	apiInternalClassFleet    = "② 保留为内部实现（子端→主控内部协议 · 不是人/agent 的动作）"
	apiInternalClassFileroot = "② 保留为内部实现（决议③：宿主文件浏览器 ⇒ 随文件浏览器虫茧迁走）"
	apiInternalClassGateway  = "② 保留为内部实现（对外协议面 · 给外部客户端 · 不是 CLI 的投影对象）"
	apiInternalClassMetrics  = "② 保留为内部实现（指标面 · 供监控刮取 · 不是命令对象）"
)

// apiInternalEndpoint —— 一条 ② 端点（命令面上只作**投影对象**，✗ 不是可执行件）。
type apiInternalEndpoint struct {
	Name   string // 投影面对象名（`api ls` 的 name 列）
	Method string // GET / POST / DELETE / PATCH（逐字取自注册行）
	Path   string // 路径（`{id}` 一类占位符照注册行逐字写）
	Class  string // 归属（闭集取值见上）
	Why    string // 一句理由（设计稿 §5.1 该条理由的缩写）
	Origin string // 注册出处（件:行 —— 与注册面核对的锚）
}

// apiInternalEndpoints —— ② 端点 **30 条**（**28 条路径**：对话 17 ＋ 机群内部协议 2 ＋
// 宿主文件浏览器 3 ＋ 网关协议面 6）。
//
// ★ 条数与路径数对不上是**设计如此**：`GET`+`POST /api/chat/sessions` 与
// `GET`+`DELETE /api/chat/sessions/{id}` 各占两条（同一路径两个方法）⇒ 30 条 (方法,路径)
// 摊在 28 条路径上（与设计稿 §5.1 行 247 的「对话面 **17 条**（**19 次注册**）」同一口径）。
var apiInternalEndpoints = []apiInternalEndpoint{
	// ── 对话面（17 条路径 · 19 次注册 · 决议①）────────────────────────────────────
	{Name: "chat_sessions_list", Method: "GET", Path: "/api/chat/sessions", Class: apiInternalClassChat,
		Why: "会话列表（对话面主力只读面）", Origin: "core/internal/api/chat_handlers.go:178"},
	{Name: "chat_session_resolve", Method: "GET", Path: "/api/chat/session-resolve", Class: apiInternalClassChat,
		Why: "会话解析（resume 预留 · 独立路径避开 {id} 通配冲突）", Origin: "core/internal/api/chat_handlers.go:179"},
	{Name: "chat_session_create", Method: "POST", Path: "/api/chat/sessions", Class: apiInternalClassChat,
		Why: "建会话（写入口 · 走 HTTP 不在命令面）", Origin: "core/internal/api/chat_handlers.go:180"},
	{Name: "chat_session_show", Method: "GET", Path: "/api/chat/sessions/{id}", Class: apiInternalClassChat,
		Why: "会话详情", Origin: "core/internal/api/chat_handlers.go:181"},
	{Name: "chat_session_delete", Method: "DELETE", Path: "/api/chat/sessions/{id}", Class: apiInternalClassChat,
		Why: "删会话", Origin: "core/internal/api/chat_handlers.go:182"},
	{Name: "chat_session_title", Method: "POST", Path: "/api/chat/sessions/{id}/title", Class: apiInternalClassChat,
		Why: "改标题", Origin: "core/internal/api/chat_handlers.go:183"},
	{Name: "chat_session_model", Method: "POST", Path: "/api/chat/sessions/{id}/model", Class: apiInternalClassChat,
		Why: "换会话模型", Origin: "core/internal/api/chat_handlers.go:184"},
	{Name: "chat_session_pinned", Method: "POST", Path: "/api/chat/sessions/{id}/pinned", Class: apiInternalClassChat,
		Why: "置顶/取消置顶", Origin: "core/internal/api/chat_handlers.go:185"},
	{Name: "chat_session_archive", Method: "POST", Path: "/api/chat/sessions/{id}/archive", Class: apiInternalClassChat,
		Why: "归档（archived=1 列表隐藏）", Origin: "core/internal/api/chat_handlers.go:186"},
	{Name: "chat_messages_list", Method: "GET", Path: "/api/chat/sessions/{id}/messages", Class: apiInternalClassChat,
		Why: "消息列表", Origin: "core/internal/api/chat_handlers.go:188"},
	{Name: "chat_window", Method: "GET", Path: "/api/chat/sessions/{id}/window", Class: apiInternalClassChat,
		Why: "召回指针回到原文（窗口）", Origin: "core/internal/api/chat_handlers.go:189"},
	{Name: "chat_compact_reset", Method: "POST", Path: "/api/chat/sessions/{id}/compact-reset", Class: apiInternalClassChat,
		Why: "重置压缩冷却/硬熔断", Origin: "core/internal/api/chat_handlers.go:190"},
	{Name: "chat_send", Method: "POST", Path: "/api/chat/sessions/{id}/send", Class: apiInternalClassChat,
		Why: "发消息（对话主写面）", Origin: "core/internal/api/chat_handlers.go:191"},
	{Name: "chat_send_tool", Method: "POST", Path: "/api/chat/sessions/{id}/send-tool", Class: apiInternalClassChat,
		Why: "工具循环对话", Origin: "core/internal/api/chat_handlers.go:192"},
	{Name: "chat_abort", Method: "POST", Path: "/api/chat/sessions/{id}/abort", Class: apiInternalClassChat,
		Why: "服务端中断（部分结果落库）", Origin: "core/internal/api/chat_handlers.go:193"},
	{Name: "chat_regenerate", Method: "POST", Path: "/api/chat/sessions/{id}/regenerate", Class: apiInternalClassChat,
		Why: "重生成（软删其后消息 · 重跑末条 user）", Origin: "core/internal/api/chat_handlers.go:194"},
	{Name: "chat_steer", Method: "POST", Path: "/api/chat/sessions/{id}/steer", Class: apiInternalClassChat,
		Why: "生成中插话（挂下一次工具结果）", Origin: "core/internal/api/chat_handlers.go:195"},
	{Name: "chat_message_edit", Method: "PATCH", Path: "/api/chat/messages/{mid}", Class: apiInternalClassChat,
		Why: "消息编辑（user-edit 借鉴）", Origin: "core/internal/api/chat_handlers.go:196"},
	{Name: "chat_search", Method: "GET", Path: "/api/chat/search", Class: apiInternalClassChat,
		Why: "会话检索", Origin: "core/internal/api/chat_handlers.go:198"},
	// ── 机群内部协议（2 条 · 设计稿 §5.1 #32/#33）────────────────────────────────
	{Name: "fleet_heartbeat", Method: "POST", Path: "/api/fleet/heartbeat", Class: apiInternalClassFleet,
		Why: "守护进程间心跳（子端→主控）｜设计稿 §5.1 #32", Origin: "core/cmd/zerg-core/main.go:164"},
	{Name: "fleet_logs_report", Method: "POST", Path: "/api/fleet/logs", Class: apiInternalClassFleet,
		Why: "子端日志上报（写入口 · 与心跳同类内部协议）｜设计稿 §5.1 #33", Origin: "core/cmd/zerg-core/main.go:186"},
	// ── 宿主文件浏览器（3 条 · 决议③ 迁往虫茧）─────────────────────────────────────
	{Name: "list_fileroots", Method: "GET", Path: "/api/fileroots", Class: apiInternalClassFileroot,
		Why: "五根白名单＋可配置项（只读：不探目录不建目录）｜设计稿 §5.1 #28", Origin: "core/cmd/zerg-core/main.go:407"},
	{Name: "open_file", Method: "POST", Path: "/api/fileroots/open", Class: apiInternalClassFileroot,
		Why: "用默认应用打开（目录／text_exts 内类型）｜设计稿 §5.1 #29", Origin: "core/cmd/zerg-core/main.go:408"},
	{Name: "reveal_file", Method: "POST", Path: "/api/fileroots/reveal", Class: apiInternalClassFileroot,
		Why: "在访达中显示（只高亮不执行）｜设计稿 §5.1 #30", Origin: "core/cmd/zerg-core/main.go:409"},
	// ── 网关协议面（6 条 · 设计稿 §5.1 #51/#71–#74/#76）─────────────────────────────
	{Name: "gateway_chat_completions", Method: "POST", Path: "/v1/chat/completions", Class: apiInternalClassGateway,
		Why: "OpenAI 兼容面（对外客户端）｜设计稿 §5.1 #71", Origin: "core/internal/gateway/gateway.go:400"},
	{Name: "gateway_messages", Method: "POST", Path: "/v1/messages", Class: apiInternalClassGateway,
		Why: "Anthropic 兼容面｜设计稿 §5.1 #74", Origin: "core/internal/gateway/gateway.go:403"},
	{Name: "gateway_responses", Method: "POST", Path: "/v1/responses", Class: apiInternalClassGateway,
		Why: "OpenAI 兼容面（responses）｜设计稿 §5.1 #76", Origin: "core/internal/gateway/gateway.go:406"},
	{Name: "gateway_context_compact", Method: "POST", Path: "/v1/context/compact", Class: apiInternalClassGateway,
		Why: "上下文压缩面｜设计稿 §5.1 #72", Origin: "core/internal/gateway/gateway.go:412"},
	{Name: "gateway_context_query", Method: "GET", Path: "/v1/context/{session_id}", Class: apiInternalClassGateway,
		Why: "上下文查询面｜设计稿 §5.1 #73", Origin: "core/internal/gateway/gateway.go:415"},
	{Name: "prefix_cache_metrics", Method: "GET", Path: "/api/metrics/prefix_cache", Class: apiInternalClassMetrics,
		Why: "前缀命中率指标（供监控刮取）｜设计稿 §5.1 #51", Origin: "core/internal/gateway/gateway.go:419"},
}

// apiEndpointKey —— 去重键：方法与路径一起（同一路径允许多个方法，故不能只按路径）。
func apiEndpointKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + path
}

func (e apiInternalEndpoint) key() string { return apiEndpointKey(e.Method, e.Path) }

// endpointCell —— `api ls` 的 endpoint 列取值（形如 `GET /api/chat/sessions`）。
func (e apiInternalEndpoint) endpointCell() string { return strings.ToUpper(e.Method) + " " + e.Path }

// descCell —— `api ls` 的 desc 列取值：归属 ＋ 理由 ＋ 出处（三样都要能一眼看见）。
func (e apiInternalEndpoint) descCell() string {
	return e.Class + "；" + e.Why + "（出处 " + e.Origin + "）"
}

// liveEndpointKey —— 把现读载荷里的一个端点格子（形如 `GET /api/fileroots`）折成去重键。
// 取不到两段（形状变了）⇒ 返回空串：**不参与去重**（宁可能重复，也不静默吃掉一条现读行）。
func liveEndpointKey(cellText string) string {
	f := strings.Fields(cellText)
	if len(f) != 2 {
		return ""
	}
	return apiEndpointKey(f[0], f[1])
}

// apiInternalLsRows —— 把 ② 端点投影成 `api ls` 的行（字段名与现读面同源，✗ 不另造词汇）。
// seen = 现读面已出现过的 (方法,路径) 集 ⇒ 同一端点只出一次（边界 ②）。
func apiInternalLsRows(seen map[string]bool) []map[string]string {
	rows := []map[string]string{}
	for _, e := range apiInternalEndpoints {
		if seen[e.key()] {
			continue
		}
		rows = append(rows, map[string]string{
			"name":     e.Name,
			"endpoint": e.endpointCell(),
			"desc":     e.descCell(),
		})
	}
	return rows
}

// apiInternalOpenAPIRows —— 把 ② 端点投影成 `api openapi` 的行（path / method / summary）。
func apiInternalOpenAPIRows(seen map[string]bool) []map[string]string {
	rows := []map[string]string{}
	for _, e := range apiInternalEndpoints {
		if seen[e.key()] {
			continue
		}
		rows = append(rows, map[string]string{
			"path":    e.Path,
			"method":  strings.ToUpper(e.Method),
			"summary": e.descCell(),
		})
	}
	return rows
}
