// chat_handlers.go — v2.5.7 对话 API（/api/chat/*——借鉴 Hermes——存储 + 推理 + 搜索）
// C2: 会话 CRUD + 消息读写 + 非流式对话——C3 加 SSE 流式

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"

	"zerg/core/internal/agent"
	"zerg/core/internal/chat"
)

// P4-11 系统提示词（借鉴 Hermes 精华——行为规格而非特质列表——
// 身份/风格/知识库铁律/完成任务/工具并行——Mr2109: 旧版"弱爆了"）
const chatSystemPrompt = `你是虫族 AI（Zerg）的对话助手——运行在虫族本地模型集群上。

# 回答风格
- 回答用中文，简洁直接。回复长度匹配问题重量：一行问题给一行答案；查证/操作用工具后给结论式总结（做了什么/结果是什么）——不重述过程。
- 无填充客套（"好的""很高兴""当然可以"），不重复用户的话，不叙述用户能看到的工具调用。
- 不确定就直说，绝不编造。同意是因为正确，不是因为用户说了。

# 对话 vs 任务（重要——你是在对话——不是被派单的子端）
- 你是**对话助手**：用户问什么答什么——自然语言交流——不是执行任务的 CA（子端）。
- 需要查证/操作时**用工具**——但回答用对话口吻（结论+依据）——**禁止**用"最终报告/改了什么/验证了什么/剩什么"这种任务汇报格式（那是 /delegate 派单后子端的汇报——不是对话助手）。
- **自己完成不反问**：需要操作就直接做完给结果——不要问"要我继续吗/要我直接测吗/要不要我…"——只有真需要用户决策（选方案/权限/外部依赖）才问。
- 简单问题快速答——复杂问题**一次做完**（探测→定位→结论）——不要分轮慢慢试（用户说"继续"才推进=失败）。

# 知识库铁律（最高优先级）
- 遇到不清楚/不确定的问题：先调用 kb_search 查虫族知识库；库中无所需再说明。
- 用户是职业剪辑师与虫族系统管理者——涉及 FCPX/达芬奇/多机位/音频/虫族系统/模型部署的问题优先查库再答。
- 按需取用，不是每个问题都查。

# 完成任务（finishing the job）
- 交付物是真实工具输出支撑的工作产物，不是描述。不要停在计划、空壳或单条命令——持续做到真实执行并验证。
- 工具/安装/网络失败阻塞真实路径时：直说失败并尝试替代方案（换包管理器/换思路/问用户）。绝不编造输出（假数据/假文件/假 API 响应）——报告阻塞好过发明结果。

# 工具使用
- 多个独立查询/搜索/读取（不互相依赖）批量合并到同一次回复（运行时并行执行）——不要一个工具一轮。
- 仅当后一步依赖前一步结果时才串行（如先读文件再改文件）。
- 工作目录是虫族项目根（<repo>）——查项目文件用 glob（按名找）/grep（按内容搜）/read（读文件）——不要用 bash 的 find/搜索绕路。
- 工具失败或结果不满足时——换工具/换参数/换思路继续——不要停下来问用户"要不要继续"。
- 任务未完成不要自己停——持续调用工具推进直到给出完整答案。只有真的收到"（已经尽力尝试了多种方式…）"这样的收尾指令时才收尾。
- 【工具分层（重要）】主提示只带基础工具（bash/read/write/edit/glob/grep/ls/kb_search/web_search/web_fetch/skill_load）。**需要其他能力时用 tool_search 搜索发现**——如查系统状态搜"系统"（cpu_status/mem_status/port_check/port_services——**本机服务/端口清单用 port_services——不要 bash lsof（输出截断）**）、查影音搜"剪辑"（media_info/ffmpeg/footage/fcpx）、查效率搜"计算"（calc/json_format）、查知识库搜"知识库"（kb_read/kb_stats）。发现后直接调用。
- 【专业领域先搜工具（重要）】涉及剪辑/FCPX/达芬奇/多机位（搜"剪辑"）、音乐下载（搜"音乐"）、系统状态（搜"系统"）、外部项目素材（footage 数据库——搜"剪辑"）时——**第一步就用 tool_search 找专用工具**——不要用 glob/grep 在项目目录里找外部资源（外部项目不在工作区——找不到是正常的）。

# 身份
- 你就是虫族 AI（Zerg）——虫族本地模型集群的一员。
- 虫族理念：人类=碳基/AI=硅基——共生。你是Mr2109（用户）的对话助手，帮助他管理虫族系统。`

// P4-50 渐进式常驻开关（ZERG_PROGRESSIVE=1 启用——默认关——对照验证——验证达标转正默认开）
var progressiveEnabled = os.Getenv("ZERG_PROGRESSIVE") == "1"

// ChatHandlers — 对话 API 处理器
type ChatHandlers struct {
	store *chat.ChatStore
	infer *chat.ChatInfer
}

// NewChatHandlers — 创建对话处理器（主控启动时挂载）
func NewChatHandlers(store *chat.ChatStore, infer *chat.ChatInfer) *ChatHandlers {
	return &ChatHandlers{store: store, infer: infer}
}

// RegisterChatRoutes — 注册 /api/chat/* 路由
func (h *ChatHandlers) RegisterChatRoutes(r chiRouter) {
	// 会话
	r.Get("/api/chat/sessions", h.ListSessions)
	r.Get("/api/chat/session-resolve", h.ResolveSession) // P4-32 resume 预留（独立路径——避免 {id} 通配冲突）
	r.Post("/api/chat/sessions", h.CreateSession)
	r.Get("/api/chat/sessions/{id}", h.GetSession)
	r.Delete("/api/chat/sessions/{id}", h.DeleteSession)
	r.Post("/api/chat/sessions/{id}/title", h.UpdateTitle)
	r.Post("/api/chat/sessions/{id}/model", h.UpdateModel)
	r.Post("/api/chat/sessions/{id}/pinned", h.SetPinned)
	r.Post("/api/chat/sessions/{id}/archive", h.SetArchive) // P4-33 归档（archived=1 列表隐藏）
	// 消息 + 对话
	r.Get("/api/chat/sessions/{id}/messages", h.ListMessages)
	r.Post("/api/chat/sessions/{id}/send", h.SendMessage)
	r.Post("/api/chat/sessions/{id}/send-tool", h.SendMessageTool) // C4b 工具循环对话
	r.Patch("/api/chat/messages/{mid}", h.EditMessage)            // P0 消息编辑（点击编辑——Hermes user-edit 借鉴）
	// 搜索
	r.Get("/api/chat/search", h.Search)
}

// chiRouter — chi 路由接口（避免 import chi 循环）
type chiRouter interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// ─────────── 会话 ───────────

// ResolveSession — resume 预留（P4-32: ref 解析——本期直通——v2.6 深链/通知用）
func (h *ChatHandlers) ResolveSession(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		writeChatError(w, http.StatusBadRequest, fmt.Errorf("chat: ref 参数必填"))
		return
	}
	// 兼容新旧 ID 格式（chat_ 前缀直通）——后续支持 resume id/title 解析
	writeChatJSON(w, http.StatusOK, map[string]any{"session_id": ref})
}

// ListSessions — 会话列表
func (h *ChatHandlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.ListSessions(100)
	if err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// CreateSession — 新建会话（P4-32: 新格式 ID + source/parent_session_id/title 参数）
func (h *ChatHandlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model           string `json:"model"`
		Source          string `json:"source"`
		ParentSessionID string `json:"parent_session_id"`
		Title           string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Model == "" {
		req.Model = "example-35b-v2" // 默认（Mr2109）
	}
	if req.Source == "" {
		req.Source = "desktop"
	}
	se, err := h.store.CreateSession(req.Model, req.Source, req.ParentSessionID, req.Title)
	if err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, se)
}

// GetSession — 会话详情（含消息）
func (h *ChatHandlers) GetSession(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	se, err := h.store.GetSession(id)
	if err != nil {
		writeChatError(w, http.StatusNotFound, err)
		return
	}
	msgs, _ := h.store.ListMessages(id)
	writeChatJSON(w, http.StatusOK, map[string]any{"session": se, "messages": msgs})
}

// DeleteSession — 删会话（硬删）
func (h *ChatHandlers) DeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	if err := h.store.DeleteSession(id); err != nil {
		writeChatError(w, http.StatusInternalServerError, fmt.Errorf("chat: 删会话失败: %w", err))
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// EditMessage — 编辑用户消息（P0 点击编辑——Hermes user-edit 借鉴——更新 content + FTS 同步）
func (h *ChatHandlers) EditMessage(w http.ResponseWriter, r *http.Request) {
	midStr := chiURLParam(r, "mid")
	mid, err := strconv.ParseInt(midStr, 10, 64)
	if err != nil {
		writeChatError(w, http.StatusBadRequest, fmt.Errorf("chat: 消息 id 非法: %s", midStr))
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeChatError(w, http.StatusBadRequest, fmt.Errorf("chat: 编辑内容为空"))
		return
	}
	// 从消息反查 session_id
	msgs, err := h.store.ListMessagesByID(mid)
	if err != nil || len(msgs) == 0 {
		writeChatError(w, http.StatusNotFound, fmt.Errorf("chat: 消息不存在"))
		return
	}
	sid := msgs[0].SessionID
	if err := h.store.UpdateMessageContent(sid, mid, strings.TrimSpace(req.Content)); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"edited": true, "id": mid, "session_id": sid, "content": strings.TrimSpace(req.Content)})
}

// UpdateTitle — 改标题
func (h *ChatHandlers) UpdateTitle(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.store.UpdateSessionTitle(id, req.Title); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// UpdateModel — 切模型
func (h *ChatHandlers) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.store.UpdateSessionModel(id, req.Model); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SetArchive — 归档/取消归档（P4-33——archived=1 列表隐藏）
func (h *ChatHandlers) SetArchive(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Archived bool `json:"archived"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.store.SetArchived(id, req.Archived); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"archived": req.Archived})
}

// SetPinned — 固定/取消
func (h *ChatHandlers) SetPinned(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Pinned bool `json:"pinned"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.store.SetSessionPinned(id, req.Pinned); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─────────── 消息 + 对话 ───────────

// ListMessages — 会话消息
func (h *ChatHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	msgs, err := h.store.ListMessages(id)
	if err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	// P4-20 存量数据规范化（字面 \n → 真实换行——旧消息兼容）
	for _, m := range msgs {
		m.Content = normalizeChatContent(m.Content)
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// isValidAnswerBody — 剥离后正文有效性检查（2026-09-05: 防「中文前英文推理剥离」误吞正文——
// 正文须 ≥8 字且含非标点实词字符——纯标点/省略号视为无效）
func isValidAnswerBody(s string) bool {
	if len([]rune(s)) < 8 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// stripReasoningFromContent — P4-50 Hermes 模式推理分离（工具轮 <tool_call> 前剥离 + 收尾轮中文前英文剥离）
// 2026-09-05 从 SendMessageTool/SendMessage 两处内联抽出（消重复）+ isValidAnswerBody 加固
// 返回: (正文, 思考)
func stripReasoningFromContent(result *chat.InferResult) (string, string) {
	msgContent := normalizeChatContent(result.Content)
	msgReasoning := result.Reasoning
	if msgReasoning != "" {
		return msgContent, msgReasoning
	}
	if strings.Contains(msgContent, "<tool_call>") {
		if idx := strings.Index(msgContent, "<tool_call>"); idx > 0 {
			return "", strings.TrimSpace(msgContent[:idx])
		}
		return msgContent, ""
	}
	// 收尾轮启发式: 第一个中文字符前的英文推理剥离（仅当后面有中文回答——全英文不剥防误伤）
	// 加固: 剥离后正文必须有效（isValidAnswerBody）——否则整段保留不当 reasoning（防误吞正文）
	idx := -1
	for i, r := range msgContent {
		if r >= 0x4e00 && r <= 0x9fff {
			idx = i
			break
		}
	}
	if idx > 0 {
		pre := strings.TrimSpace(msgContent[:idx])
		rest := strings.TrimSpace(msgContent[idx:])
		if len([]rune(pre)) > 20 && isValidAnswerBody(rest) {
			return rest, pre
		}
	}
	return msgContent, ""
}

// SendMessageTool — 发消息（C4b 工具循环——非流式 Infer 带 tools——tool_calls 流转）
// 决策: llama-server 流式不推 tool_calls（GitHub #5769）——工具对话走非流式循环——
// SSE 转发最终结果（delta 一次性 + tool 事件）——纯对话仍走 SendMessage（C3 流式）
func (h *ChatHandlers) SendMessageTool(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeChatError(w, http.StatusBadRequest, errOrMsg(err, "消息内容为空"))
		return
	}
	se, err := h.store.GetSession(id)
	if err != nil {
		writeChatError(w, http.StatusNotFound, err)
		return
	}
	// 1. 存用户消息
	userMsg := &chat.Message{
		SessionID: id, Role: "user", Content: normalizeChatContent(req.Content),
		Active: true, Timestamp: chatNow(),
	}
	if _, err := h.store.AddMessage(userMsg); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	// 2. 历史
	history, _ := h.store.GetActiveMessages(id)
	_, _ = h.store.CompressHistory(r.Context(), h.infer, id, se.Model, history)
	history, _ = h.store.GetActiveMessages(id)
	msgs := make([]map[string]any, 0, len(history))
	for _, m := range history {
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}
	// 3. 工具循环（非流式——全工具——C7 危险命令黑名单 gate）
	// P4-41 身份治本: 系统提示注入真实模型名——模型不用"调查自己"（编造身份根因）
	// P4-46 Hermes 工具指令（治本: 不带 tools 字段——模板 XML 分支不渲染——模型输出 JSON 工具调用）
	// P4-50 渐进式常驻（ZERG_PROGRESSIVE=1 启用）
	var progRT *chat.ToolRuntime
	if progressiveEnabled {
		progRT = h.store.GetToolRuntime(id)
	}
	sysPrompt := chatSystemPrompt + fmt.Sprintf("\n\n# 你的身份\n- 你当前运行在模型 %s（虫族本地模型集群）——Mr2109的对话助手——不要调查或质疑自己的身份。", se.Model) + chat.BuildHermesToolPrompt(progRT)
	gate := &chat.ChatGate{} // C7 危险命令黑名单（rm 根目录/mkfs/shutdown 拦截+引导）
	result, traces, err := chat.RunToolLoop(r.Context(), h.infer, se.Model, sysPrompt, msgs, gate, progRT)
	if err != nil {
		_ = h.store.DeleteMessage(id, userMsg.ID)
		writeChatError(w, http.StatusBadGateway, err)
		return
	}
	// 4. 存 assistant 消息（含工具轨迹）
	// P4-31 无工具时存空串（json.Marshal(nil) = "null" 字符串——UI 误判显示"工具调用…"）
	var toolCallsStr string
	if len(traces) > 0 {
		tracesJSON, _ := json.Marshal(traces)
		toolCallsStr = string(tracesJSON)
	}
	// P4-50 Hermes 模式: example-35b-v2 推理写进 content（reasoning_content 空——无 tools 字段时训练格式如此）
	// 2026-09-05 抽公共 stripReasoningFromContent（两路径同构消重复）+ isValidAnswerBody 防误吞正文
	msgContent, msgReasoning := stripReasoningFromContent(result)
	assistantMsg := &chat.Message{
		SessionID: id, Role: "assistant", Content: msgContent,
		Reasoning: msgReasoning, Model: se.Model,
		TokenCount: result.OutputTokens, Active: true, Timestamp: chatNow(),
		ToolCalls: toolCallsStr,
	}
	if _, err := h.store.AddMessage(assistantMsg); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.TouchSession(id, result.InputTokens, result.OutputTokens, result.ReasoningTokens)
	if se.Title == "" {
		title := autoTitle(req.Content)
		_ = h.store.UpdateSessionTitle(id, title)
		se.Title = title
	}
	// 5. SSE 转发（工具事件 + 最终结果）
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	writeSSE := func(evType, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evType, data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	for _, tr := range traces {
		payload, _ := json.Marshal(map[string]any{"name": tr.Name, "args": tr.Args, "result": truncateStr(tr.Result, 300), "error": tr.Error})
		writeSSE("tool", string(payload))
	}
	if result.Content != "" {
		payload, _ := json.Marshal(map[string]any{"type": "output", "text": result.Content})
		writeSSE("delta", string(payload))
	}
	payload, _ := json.Marshal(map[string]any{"done": true, "title": se.Title})
	writeSSE("done", string(payload))
}

// toolNames — 当前 tools 列表工具名（tool_search 无结果时展示）
func toolNames(tools []map[string]any) string {
	return strings.Join(toolNameList(tools), ", ")
}

// toolNameList — 工具名列表（LoopGuard 引导用）
func toolNameList(tools []map[string]any) []string {
	var names []string
	for _, t := range tools {
		if fn, ok := t["function"].(map[string]any); ok {
			if nm, ok := fn["name"].(string); ok {
				names = append(names, nm)
			}
		}
	}
	return names
}

// truncateStr — 截断（工具结果展示）
// （复用 api 包 zerg_controlled_loop.go 的 truncateStr——此处删除防重名）

// chatImageDir — 对话图片存储目录（90 天销毁同区——/tmp/zerg-chat）
const chatImageDir = "/tmp/zerg-chat/images"

// saveChatImage — 保存对话图片（data URL base64 → 文件）——返回文件路径
func saveChatImage(dataURL string) string {
	// 格式: data:image/png;base64,xxx
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return ""
	}
	header := dataURL[:comma]
	ext := ".png"
	if strings.Contains(header, "jpeg") || strings.Contains(header, "jpg") {
		ext = ".jpg"
	} else if strings.Contains(header, "gif") {
		ext = ".gif"
	} else if strings.Contains(header, "webp") {
		ext = ".webp"
	}
	b64 := dataURL[comma+1:]
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(chatImageDir, 0o755); err != nil {
		return ""
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	path := filepath.Join(chatImageDir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return ""
	}
	return path
}

// P4-20 规范化对话内容（模型长文本输出字面 \n（0x5C 0x6E）而非真实换行——
// 统一转真实换行/引号/tab——存库和读取都过一遍——存量数据兼容）
func normalizeChatContent(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\\\t", "\t")
	// P4-24 表格修复：模型常在表头与分隔行之间多打空行（`|..|\n\n|--|`）——
	// markdown 规范要求紧邻——空行导致整个表解析成普通段落
	s = tableSepFix.ReplaceAllString(s, "$1\n$2\n")
	return s
}

// 表头行 + 空行 + 分隔行 → 表头行 + 分隔行（分隔行特征：| - | : | 组合）
var tableSepFix = regexp.MustCompile(`(\|[^\n]*\|)\n\n(\|[-| :]+\|)\n`)

// P4-14 修复: 历史图片只保留最新一条（keepImage）——旧图片转纯文本并标注——
// 根因①Scan缺ImagePath(图从未传出——纯上下文延续) ②Mr2109实测: 发"你好"回复"红色"
func chatMessageToReq(m *chat.Message, keepImage bool) map[string]any {
	m.Content = normalizeChatContent(m.Content)
	if keepImage && m.ImagePath != "" {
		parts := []map[string]any{}
		seen := 0
		for _, p := range strings.Split(m.ImagePath, ",") {
			p = strings.TrimSpace(p)
			if p == "" || seen >= 8 {
				continue
			}
			if raw, err := os.ReadFile(p); err == nil {
				ext := strings.TrimPrefix(filepath.Ext(p), ".")
				if ext == "" {
					ext = "png"
				}
				parts = append(parts, map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": "data:image/" + ext + ";base64," + base64.StdEncoding.EncodeToString(raw),
					},
				})
				seen++
			}
		}
		if len(parts) > 0 {
			parts = append([]map[string]any{{"type": "text", "text": m.Content}}, parts...)
			return map[string]any{"role": m.Role, "content": parts}
		}
	}
	// 历史图片消息（或图片读失败）——纯文本 + 省略标注（模型不再延续图片话题）
	if m.ImagePath != "" {
		return map[string]any{"role": m.Role, "content": m.Content + " [用户在此消息附带了图片——当前仅文字可见，图片已省略]"}
	}
	return map[string]any{"role": m.Role, "content": m.Content}
}

// SendMessage — 发消息（C3 流式 + D2 流式工具循环——SSE）
// 流程: 存 user → 流式调网关(带 tools) → delta 转发 → 若 tool_calls: 执行→tool 事件→再流式 → 完成后存 assistant
func (h *ChatHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Content string   `json:"content"`
		Image   string   `json:"image"`   // D3 多模态: data URL base64（单图——兼容）
		Images  []string `json:"images"`  // P2 多图: data URL base64 数组（优先）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeChatError(w, http.StatusBadRequest, errOrMsg(err, "消息内容为空"))
		return
	}
	se, err := h.store.GetSession(id)
	if err != nil {
		writeChatError(w, http.StatusNotFound, err)
		return
	}
	// 1. 存用户消息（D3/P2 图片: base64 → 文件 → image_path——多图逗号分隔）
	imagePath := ""
	imgs := req.Images
	if len(imgs) == 0 && req.Image != "" {
		imgs = []string{req.Image}
	}
	if len(imgs) > 0 {
		var paths []string
		for _, d := range imgs {
			if p := saveChatImage(d); p != "" {
				paths = append(paths, p)
			}
		}
		imagePath = strings.Join(paths, ",")
	}
	userMsg := &chat.Message{
		SessionID: id, Role: "user", Content: normalizeChatContent(req.Content),
		Active: true, Timestamp: chatNow(), ImagePath: imagePath,
	}
	userMsgID, err := h.store.AddMessage(userMsg)
	if err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	// 2. 历史（active 窗口内——压缩后旧消息不重发）
	history, _ := h.store.GetActiveMessages(id)
	msgs := make([]map[string]any, 0, len(history))
	for i, m := range history {
		// P4-12 只有最新一条（当前消息）保留图片——历史图片转纯文本
		keepImage := i == len(history)-1
		msgs = append(msgs, chatMessageToReq(m, keepImage))
	}
	// 3. 流式工具循环（D2——全工具——C7 黑名单 gate）
	// P4-41 身份治本: 系统提示注入真实模型名——模型不用"调查自己"（编造身份根因）
	// P4-46 Hermes 工具指令（治本: 不带 tools 字段——模板 XML 分支不渲染——模型输出 JSON 工具调用）
	// P4-50 渐进式常驻（ZERG_PROGRESSIVE=1 启用——流式主路径同开关）
	var progRT *chat.ToolRuntime
	if progressiveEnabled {
		progRT = h.store.GetToolRuntime(id)
	}
	sysPrompt := chatSystemPrompt + fmt.Sprintf("\n\n# 你的身份\n- 你当前运行在模型 %s（虫族本地模型集群）——Mr2109的对话助手——不要调查或质疑自己的身份。", se.Model) + chat.BuildHermesToolPrompt(progRT)
	gate := &chat.ChatGate{}
		// P4-46 Hermes 模式: 不带 tools 字段（模板 XML 分支不渲染——模型输出 <tool_call>JSON</tool_call>——parseXMLToolCalls 解析）
	tools := []map[string]any(nil)

	// SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeChatError(w, http.StatusInternalServerError, fmt.Errorf("SSE 不支持"))
		return
	}
	// 流式回调：写 SSE 事件
	writeSSE := func(evType, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evType, data)
		flusher.Flush()
	}

	// C4a 上下文压缩（P4-39 T5: SSE 先行——压缩过程可见——Hermes TurnActivityIndicator "compacting"）
	// 压缩耗时 10-30s——用户在 UI 看到"正在压缩历史…"而非静默卡住
	writeSSE("compacting", `{}`)
	compacted, _ := h.store.CompressHistory(r.Context(), h.infer, id, se.Model, history)
	if compacted {
		writeSSE("compact_done", `{"done":true}`)
		history, _ = h.store.GetActiveMessages(id)
		msgs = msgs[:0]
		for _, m := range history {
			msgs = append(msgs, chatMessageToReq(m, false))
		}
	}

	// 工具循环（最多 10 轮——每轮流式——tool_calls 执行后追加结果再流式）
	// P4-40 对话工具工作目录=项目根（模型查项目用 glob/grep/read 相对路径——之前 /tmp/zerg-chat/tools 导致路径全被 validatePath 拒——"工具已用尽"假象）
	// 2026-09-05 统一常量 ChatToolsWorkDir（与非流式路径共用——消除双循环目录分叉）
	ec := agent.NewExecContext(chat.ChatToolsWorkDir)
	ec.AgentName = "chat"
	var traces []chat.ToolTrace
	var result *chat.InferResult
	cur := msgs
	// P4-38 LoopGuard（SHA-256 指纹滑动窗口——检测重复/交替——引导换招——非失败停止）
	guard := chat.NewLoopGuard(chat.DefaultLoopConfig())
	loopStart := time.Now()
	// P4-44 空参数计数（模型输出 {} 高频——连续 3 次立即收尾——不空转）
	emptyArgsStreak := 0
	// P4-48 坏格式计数（content 含 <tool_call> 但解析失败——连续 3 次收尾）
	badFormatStreak := 0
	// P4-50 重复搜索计数（模型反复 tool_search 同关键词不执行——第 2 次强制提示）
	searchStreak := map[string]int{}
	// P4-50 搜索无进展计数（模型换词反复 tool_search 不执行——2 轮轻推/3 轮强推新工具——不收尾——Mr2109）
	searchNoUseStreak := 0
	for round := 1; round <= chat.MaxToolRounds; round++ {
		// P4-38 保护检查（墙钟 600s——防失控）
		if time.Since(loopStart).Seconds() > 600 {
			break
		}
		// P4-38 T5 上下文轻量化（loop 中旧工具结果压缩——小模型上下文金贵——防膨胀到放弃工具）
		if len(cur) > 30 {
			cur = chat.CompactToolResults(cur, 3)
		}
		// 流式调网关（带工具）
		var streamErr error
		// 2026-09-05 修复: 重试重复输出——deltaOnce 标记首轮是否已流出 token——
		// 已流出后再重试会从头重放（用户看到断片+重说）——只在收到 delta 前允许重试
		var deltaOnce bool
		deltaSink := func(deltaType, text string) {
			deltaOnce = true
			payload, _ := json.Marshal(map[string]any{"type": deltaType, "text": text})
			writeSSE("delta", string(payload))
		}
		// P4-50 每轮推理限时 120s（模型单轮生成不收敛——无限生成——墙钟在轮间查不到——轮内也限）
		roundCtx, roundCancel := context.WithTimeout(r.Context(), 120*time.Second)
		result, streamErr = h.infer.InferStream(roundCtx, se.Model, sysPrompt, cur, deltaSink, tools)
		roundCancel()
		if streamErr != nil && (strings.Contains(streamErr.Error(), "context deadline") || strings.Contains(streamErr.Error(), "超时")) {
			// 单轮超时——模型生成不收敛——直接收尾（提示用已有信息回答——不再等）
			writeSSE("loop_hint", `{"kind":"round_timeout"}`)
			cur = append(cur, map[string]any{"role": "user", "content": "（时间到——请立即把已获得的信息整理成最终回答——不要继续调用工具或推理——直接给出结论——信息不足就说明没找到——绝不编造。）"})
			streamErr = nil
			result = &chat.InferResult{} // 空结果——940 收尾轮会处理
			break
		}
		if streamErr != nil && !deltaOnce {
			// P4-48 故障自愈: X3 单槽排队超时（502/500——基础设施瞬时故障）——重试 2 次（间隔 2s）——3 次失败才回滚
			// 2026-09-05: 只在未流出任何 delta 时重试（deltaOnce=true 时重试必重复输出——直接报错回滚）
			retried := false
			for attempt := 1; attempt <= 2; attempt++ {
				if !strings.Contains(streamErr.Error(), "502") && !strings.Contains(streamErr.Error(), "500") {
					break
				}
				time.Sleep(2 * time.Second)
				writeSSE("retry", fmt.Sprintf("{\"attempt\":%d,\"reason\":\"X3 瞬时故障\"}", attempt))
				result, streamErr = h.infer.InferStream(r.Context(), se.Model, sysPrompt, cur, deltaSink, tools)
				if streamErr == nil {
					retried = true
					break
				}
			}
			if streamErr != nil {
				// 推理失败（重试后仍失败）——回滚 user 消息 + SSE 发 error
				_ = h.store.DeleteMessage(id, userMsgID)
				payload, _ := json.Marshal(map[string]any{"error": streamErr.Error()})
				writeSSE("error", string(payload))
				return
			}
			if retried {
				writeSSE("retry_done", "{}")
			}
		} else if streamErr != nil && deltaOnce {
			// 已流出部分 token 后才失败——无法安全重试（会重复）——发中断提示+按现有内容收尾
			writeSSE("loop_hint", `{"kind":"stream_broken_midway"}`)
			result = &chat.InferResult{}
			break
		}
		if len(result.ToolCalls) == 0 {
			// P4-47/48 坏工具调用检测: content 含 <tool_call> 但解析失败——不当正文——提示模型修正格式重试
			// P4-48 连续 3 次坏格式 → 收尾（模型学不会——不再空转）
			if strings.Contains(result.Content, "<tool_call>") {
				badFormatStreak++
				var badCall string
				if badFormatStreak >= 3 {
					badCall = "你的工具调用格式一直无效（已 " + fmt.Sprint(badFormatStreak) + " 次）。请停止调用工具——用中文把已知信息整理成最终回答。工具调用示例（严格照抄——一个 <tool_call> 只放一个 JSON 对象）:\n<tool_call>\n{\"name\": \"task_list\", \"arguments\": {}}\n</tool_call>"
					cur = append(cur, map[string]any{"role": "user", "content": badCall})
					writeSSE("tool", `{"name":"__bad_format__","args":"{}","result":"`+badCall+`"}`)
					break
				}
				badCall = "工具调用格式无效（<tool_call> 内必须是一个 JSON 对象 {\"name\": \"工具名\", \"arguments\": {...}}——无参数工具 arguments 写 {}——多个工具就输出多个 <tool_call> 块——严格照抄示例:\n<tool_call>\n{\"name\": \"task_list\", \"arguments\": {}}\n</tool_call>）——请重新输出格式正确的工具调用"
				cur = append(cur, map[string]any{"role": "user", "content": badCall})
				writeSSE("tool", `{"name":"__bad_format__","args":"{}","result":"`+badCall+`"}`)
				continue
			}
			badFormatStreak = 0
			break // 无工具调用——最终回复
		}
		// 执行工具（P4-36 异步 + 心跳——工具执行期间 SSE 不断流——UI 实时反馈"执行中 N 秒"）
		for _, tc := range result.ToolCalls {
			// P4-50 参数统一解包（模型 Hermes 风格嵌套——bash arguments 双层——见 NormalizeToolArgs）
			chat.NormalizeToolArgs(&tc)
			argsJSON, _ := json.Marshal(tc.Args)
			// P4-35 工具执行前发 tool_start 事件（UI 显示"🔧 bash 执行中…"——解决等待无反馈）
			startPayload, _ := json.Marshal(map[string]any{"name": tc.Name, "args": string(argsJSON)})
			writeSSE("tool_start", string(startPayload))
			startT := time.Now()
			// 异步执行（goroutine——handler 不被阻塞——心跳 ticker 持续推事件）
			type execRes struct {
				content string
				dur     string
				err     error
			}
			resCh := make(chan execRes, 1)
			go func(tc agent.ToolCall) {
				var content, dur string
				var execErr error
				// P4-50 隐藏工具拦截（3 次 exec 失败——本对话不再执行——tool_search 查询器除外）
				if progRT != nil && progRT.IsHidden(tc.Name) && tc.Name != "tool_search" {
					resCh <- execRes{content: fmt.Sprintf("【系统】工具 %s 本对话已隐藏（连续 3 次执行失败）。请换其他工具或 tool_search 搜索替代。", tc.Name)}
					return
				}
				if tc.Name == "kb_search" {
					query, _ := tc.Args["query"].(string)
					limit := 10
					if l, ok := tc.Args["limit"].(float64); ok {
						limit = int(l)
					}
					content, execErr = chat.KbSearchExecute(query, limit)
				} else if tc.Name == "tool_search" {
					// P4-46 Hermes 模式: tool_search 发现工具——文本描述注入 cur（无 tools 字段——模型后续轮按描述调用）
					query, _ := tc.Args["query"].(string)
					// P4-50 重复搜索检测（模型反复搜同关键词不执行——第 2 次强制提示立即调用）
					if query != "" {
						searchStreak[query]++
					}
					have := map[string]bool{}
					for _, t := range tools {
						if fn, ok := t["function"].(map[string]any); ok {
							if nm, ok := fn["name"].(string); ok {
								have[nm] = true
							}
						}
					}
					found := chat.ChatToolSearch(query, have, 8)
					// P4-50 隐藏工具过滤（本对话 3 次 exec 失败的工具不推荐——除非全部隐藏则保留第一个——"无其他可选不隐藏"）
					if progRT != nil {
						var visible []string
						for _, nm := range found {
							if !progRT.IsHidden(nm) {
								visible = append(visible, nm)
							}
						}
						if len(visible) > 0 {
							found = visible
						}
					}
					if len(found) == 0 {
						content = "（未发现匹配工具——当前可用: " + toolNames(tools) + "——可换个词再搜）"
					} else {
						extraDefs := chat.ChatExtraToolDefs()
						var descs []string
						for _, nm := range found {
							desc := nm
							if def, ok := extraDefs[nm]; ok {
								if fn, ok2 := def["function"].(map[string]any); ok2 {
									if d, ok3 := fn["description"].(string); ok3 {
										desc = nm + ": " + d
									}
								}
							}
							descs = append(descs, desc)
						}
						if searchStreak[query] >= 3 && len(found) > 0 {
							// P4-50 第 3 次重复搜索——系统代执行第一个匹配工具（模型搜了不调——系统弥补——虫族哲学: 小模型生成削弱由系统补）
							sysName := found[0]
							if chat.IsExtraTool(sysName) {
								ctr := chat.ExecuteChatTool(sysName, map[string]any{}, "<repo>")
								content = ctr.Content
								if ctr.Error != "" {
									execErr = fmt.Errorf("%s", ctr.Error)
								}
							} else {
								tres := ec.ExecuteTool(r.Context(), sysName, map[string]any{}, gate)
								content = tres.Content
								if tres.Error != "" {
									execErr = fmt.Errorf("%s", tres.Error)
								}
								dur = tres.Duration
							}
							content = fmt.Sprintf("【你已连续 3 次搜索 \"%s\" 未执行——系统替你执行了 %s 工具】\n%s", query, sysName, content)
							searchStreak[query] = 0 // 重置（防止下轮再触发）
						} else if searchStreak[query] >= 2 {
							// 重复搜索——强制提示（模型搜了不调——LoopGuard 引导不够）
							content = fmt.Sprintf("【你已经搜索过 \"%s\"——工具已在上方列出——不要重复搜索——立即调用其中一个（如 %s）——用 <tool_call> 格式直接调用】发现 %d 个工具:\n%s",
								query, found[0], len(found), strings.Join(descs, "\n"))
						} else {
							content = fmt.Sprintf("✅ 发现 %d 个工具——【已加入你的可用工具列表——与 <tools> 内工具同等地位——直接用工具名调用——参数写进 arguments——JSON 格式】:\n%s",
								len(found), strings.Join(descs, "\n"))
						}
					}
					execErr = nil
				} else if chat.IsExtraTool(tc.Name) {
					// P4-42 deferred 新工具（chat 层实现）
					ctr := chat.ExecuteChatTool(tc.Name, tc.Args, "<repo>")
					content = ctr.Content
					if ctr.Error != "" {
						execErr = fmt.Errorf("%s", ctr.Error)
					}
				} else {
					tres := ec.ExecuteTool(r.Context(), tc.Name, tc.Args, gate)
					content = tres.Content
					if tres.Error != "" {
						execErr = fmt.Errorf("%s", tres.Error)
					}
					dur = tres.Duration
				}
				resCh <- execRes{content: content, dur: dur, err: execErr}
			}(tc)
			// 心跳：每 2s 推 tool_ping（UI 显示执行中计时——Hermes 工具行等效）
			ticker := time.NewTicker(2 * time.Second)
			var res execRes
		waitLoop:
			for {
				select {
				case r := <-resCh:
					res = r
					break waitLoop
				case <-ticker.C:
					pingPayload, _ := json.Marshal(map[string]any{"name": tc.Name, "elapsed": int(time.Since(startT).Seconds())})
					writeSSE("tool_ping", string(pingPayload))
				case <-r.Context().Done():
					// 客户端断开——取消工具执行
					res = execRes{err: fmt.Errorf("已取消")}
					break waitLoop
				}
			}
			ticker.Stop()
			var content, dur string
			var execErr error
			content, dur, execErr = res.content, res.dur, res.err
			if execErr != nil {
				content = fmt.Sprintf("工具执行失败: %s（%s）", execErr.Error(), truncateStr(content, 500))
				// P4-43 空参数强化（小模型高频输出 {}——给示例照抄——治本）
				if tc.Name == "bash" && strings.Contains(content, "命令参数为空") {
					content = "bash 命令参数为空——必须提供 command 字段。正确示例: {\"command\":\"ls -la\"} 或 {\"command\":\"python3 test.py\"}——请照抄这个格式重新调用"
				}
				if tc.Name == "tool_search" && strings.Contains(content, "参数为空") {
					content = "tool_search 参数为空——必须提供 query 字段。正确示例: {\"query\":\"剪辑\"} 或 {\"query\":\"系统\"}——请照抄格式"
				}
			}
			// P4-50 渐进式常驻钩子（SSE 路径——成败判定只看执行层）
			if progRT != nil && tc.Name != "tool_search" {
				if execErr != nil {
					typ := chat.ErrTypeOf(execErr.Error())
					hint := progRT.RecordOutcome(tc.Name, typ, execErr.Error())
					chat.RecordToolError(tc.Name, execErr.Error(), tc.Args)
					if hint != "" {
						content = hint + "\n" + content
					}
				} else if !strings.HasPrefix(content, "【bash") && !strings.HasPrefix(content, "【系统】") {
					progRT.RecordOutcome(tc.Name, "", "")
				}
			}
			// P4-47 contentJSON（Hermes <tool_response>——content 转 JSON 字符串——避免引号破坏）
			contentJSON, _ := json.Marshal(content)
			traces = append(traces, chat.ToolTrace{
				Round: round, CallID: tc.ID, Name: tc.Name,
				Args: string(argsJSON), Result: content,
				Error:  func() string { if execErr != nil { return execErr.Error() }; return "" }(),
				Duration: dur,
			})
			// tool 事件 SSE
			payload, _ := json.Marshal(map[string]any{"name": tc.Name, "args": string(argsJSON), "result": truncateStr(content, 300)})
			writeSSE("tool", string(payload))
			// P4-47 contentJSON（Hermes <tool_response> 用——JSON 字符串化——避免引号破坏 XML）
			// P4-47 Hermes 标准回传: assistant 保留模型 <tool_call> 原文 + tool 消息用 <tool_response>（模型训练见过的格式）
			assistantContent := tc.RawCall
			if assistantContent == "" {
				assistantContent = fmt.Sprintf("<tool_call>\n{\"name\": \"%s\", \"arguments\": %s}\n</tool_call>", tc.Name, argsJSON)
			}
			toolResp := fmt.Sprintf("<tool_response>\n{\"name\": \"%s\", \"content\": %s}\n</tool_response>", tc.Name, contentJSON)
			cur = append(cur,
				map[string]any{"role": "assistant", "content": assistantContent},
				map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": toolResp},
			)
			// P4-44 空参数计数（{} 或空——模型无效调用——连续 3 次升级）
			argsJSON2 := string(argsJSON)
			if len(argsJSON2) <= 2 || argsJSON2 == "{}" || argsJSON2 == "null" {
				emptyArgsStreak++
			} else {
				emptyArgsStreak = 0
			}
			// P4-38 指纹记录（工具名+参数哈希——检测重复/交替）
			guard.Record(tc.Name, tc.Args)
			// P4-38 T3: 工具结果标记 [成功]/[失败]（小模型可读状态——futureagi: 无法读反馈=循环根因）
			// P4-50 加长度标注: [成功·N字]——模型明确知道"这就是完整返回"——防"被省略"误判反复核验
			status := "[失败]"
			if execErr != nil {
				status = "[失败]"
			} else {
				runeLen := len([]rune(content))
				if runeLen > 0 {
					status = fmt.Sprintf("[成功·%d字]", runeLen)
				} else {
					status = "[成功·空]"
				}
			}
			toolResp2 := fmt.Sprintf("<tool_response>\n{\"name\": \"%s\", \"content\": %s}\n</tool_response>", tc.Name, contentJSON)
			cur = append(cur,
				map[string]any{"role": "assistant", "content": assistantContent},
				map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": status + " " + toolResp2},
			)
		}
		// P4-50 搜索无进展检测（本轮全 tool_search 无执行 → 推新工具/换思路——不是收尾——Mr2109）
		searchedOnly := len(result.ToolCalls) > 0
		for _, tcr := range result.ToolCalls {
			if tcr.Name != "tool_search" {
				searchedOnly = false
				break
			}
		}
		if searchedOnly {
			searchNoUseStreak++
		} else {
			searchNoUseStreak = 0
		}
		if searchNoUseStreak >= 3 {
			// 第 3 轮仍只搜不用——强推：停止搜索——从已发现工具执行或换思路
			cur = append(cur, map[string]any{"role": "user", "content": "（你已连续 3 轮只搜索工具未执行任何工具。停止搜索——从已发现的工具里选一个直接执行（<tool_call> 格式）——若都不合适就换思路（help 看用法/直接 bash/read 查）——不要继续搜索。）"})
			writeSSE("loop_hint", `{"kind":"no_search_progress"}`)
			searchNoUseStreak = 0 // 引导已给——下轮再犯重新计
		} else if searchNoUseStreak == 2 {
			// 第 2 轮轻推
			cur = append(cur, map[string]any{"role": "user", "content": "（提示：你已连续 2 轮只搜索未执行工具——从搜索结果里选一个执行——或换思路——不要继续搜索同类问题。）"})
			writeSSE("loop_hint", `{"kind":"no_search_progress"}`)
		}
		// P4-44 空参数连续 3 次 → 立即收尾（模型无效输出——空转无意义）
		if emptyArgsStreak >= 3 {
			cur = append(cur, map[string]any{
				"role":    "user",
				"content": "（你的工具调用连续输出空参数（{}）——工具一直无法执行。请停止调用工具，把已知信息整理成最终回答；如果确实需要执行命令，请先说明要执行什么。绝不编造。）",
			})
			break
		}
		// P4-38 循环守卫（Detect + 分级引导——换策略→列工具→升级——不是失败就停）
		if ok, reason := guard.Detect(); ok {
			guide, upgrade := guard.BuildGuide(reason, toolNameList(tools))
			cur = append(cur, map[string]any{"role": "user", "content": guide})
			if upgrade {
				// 引导 3 次仍重复——升级收尾（保留已执行工作——不丢弃）
				break
			}
			writeSSE("loop_hint", `{"kind":"guide"}`)
		}
	}
	// P4-37 轮数用尽强制收尾（模型还在调工具——追加提示——最后带工具调一轮——必须给最终答案）
	if len(result.ToolCalls) > 0 {
		cur = append(cur, map[string]any{
			"role":    "user",
			"content": "（已经尽力尝试了多种方式，请把到目前为止获得的信息整理成最终回答。如果信息不足或没找到答案，就直接说明没找到——绝不编造。）",
		})
		var finalErr error
		// P4-50 收尾轮限时（墙钟 bug——模型收尾不收敛无限生成——60s 截断——有流式内容已发出——超时按已收内容返回）
		finCtx, finCancel := context.WithTimeout(r.Context(), 60*time.Second)
		result, finalErr = h.infer.InferStream(finCtx, se.Model, sysPrompt, cur, func(deltaType, text string) {
			payload, _ := json.Marshal(map[string]any{"type": deltaType, "text": text})
			writeSSE("delta", string(payload))
		}, []map[string]any{{"__temp__": 0.3}})
		finCancel()
		if finalErr != nil {
			// 收尾轮失败/超时——用已收集内容尽力收尾（不再发 error——有历史工具结果可整理）
			if strings.Contains(finalErr.Error(), "context deadline") || strings.Contains(finalErr.Error(), "超时") {
				writeSSE("loop_hint", `{"kind":"final_timeout"}`)
				finalErr = nil // 超时不算致命——有部分/历史内容
			}
		}
		if finalErr != nil {
			// 收尾轮失败（非超时）——用最后一轮结果（可能空——但尽力）
			_ = h.store.DeleteMessage(id, userMsgID)
			payload, _ := json.Marshal(map[string]any{"error": finalErr.Error()})
			writeSSE("error", string(payload))
			return
		}
	}
	// P4-48 最终回答质量检测: content 是推理文本（英文推理开头/无中文——example-35b-v2 通病）→ 重试一次"请用中文直接回答"
	if !isChineseAnswer(result.Content) && len(traces) > 0 {
		cur = append(cur, map[string]any{"role": "user", "content": "（你的上一条输出是思考过程——不是回答。请用中文直接回答用户的问题——基于已获取的工具结果——简洁总结。）"})
		var retryRes *chat.InferResult
		// P4-50 重试轮也限时（模型可能继续绕——不答中文——60s 截断）
		retryCtx, retryCancel := context.WithTimeout(r.Context(), 60*time.Second)
		retryRes, _ = h.infer.InferStream(retryCtx, se.Model, sysPrompt, cur, func(deltaType, text string) {
			payload, _ := json.Marshal(map[string]any{"type": deltaType, "text": text})
			writeSSE("delta", string(payload))
		}, []map[string]any{{"__temp__": 0.3}})
		retryCancel()
		if retryRes != nil && retryRes.Content != "" {
			result = retryRes
		}
	}
	// 4. 存 assistant 消息（含工具轨迹——思考分离）
	// P4-31 无工具时存空串（json.Marshal(nil) = "null" 字符串——UI 误判显示"工具调用…"）
	var toolCallsStr string
	if len(traces) > 0 {
		tracesJSON, _ := json.Marshal(traces)
		toolCallsStr = string(tracesJSON)
	}
	// P4-50 Hermes 模式: example-35b-v2 推理写进 content（reasoning_content 空——无 tools 字段时训练格式如此）
	// 2026-09-05 抽公共 stripReasoningFromContent（两路径同构消重复）+ isValidAnswerBody 防误吞正文
	msgContent, msgReasoning := stripReasoningFromContent(result)
	assistantMsg := &chat.Message{
		SessionID: id, Role: "assistant", Content: msgContent,
		Reasoning: msgReasoning, Model: se.Model,
		TokenCount: result.OutputTokens, Active: true, Timestamp: chatNow(),
		ToolCalls: toolCallsStr,
	}
	astID, err := h.store.AddMessage(assistantMsg)
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"error": err.Error()})
		writeSSE("error", string(payload))
		return
	}
	_ = h.store.TouchSession(id, result.InputTokens, result.OutputTokens, result.ReasoningTokens)
	// 5. 标题自动生成（首轮后——无标题时）
	if se.Title == "" {
		title := autoTitle(req.Content)
		_ = h.store.UpdateSessionTitle(id, title)
		se.Title = title
	}
	// 6. SSE done（含消息 id + 标题）
	payload, _ := json.Marshal(map[string]any{
		"message_id": astID,
		"title":      se.Title,
		"input_tokens": result.InputTokens,
		"output_tokens": result.OutputTokens,
		"reasoning_tokens": result.ReasoningTokens,
	})
	writeSSE("done", string(payload))
}

// ─────────── 搜索 ───────────

// Search — 会话搜索（FTS5）
func (h *ChatHandlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeChatJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}
	results, err := h.store.SearchMessages(q, 50)
	if err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatJSON(w, http.StatusOK, map[string]any{"results": results})
}

// ─────────── 工具 ───────────

// autoTitle — 标题自动生成（首条消息截断——C5 用模型生成）
func autoTitle(content string) string {
	s := strings.TrimSpace(content)
	runes := []rune(s)
	if len(runes) > 20 {
		return string(runes[:20]) + "…"
	}
	return s
}

// chatNow — 当前时间戳（秒）
func chatNow() float64 {
	return float64(time.Now().Unix())
}

// chiURLParam — 取路径参数（chi 官方）
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// writeChatJSON — JSON 响应
func writeChatJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeChatError — 错误响应
func writeChatError(w http.ResponseWriter, status int, err error) {
	msg := "未知错误"
	if err != nil {
		msg = err.Error()
	}
	writeChatJSON(w, status, map[string]any{"error": msg})
}

// errOrMsg — err 为 nil 时用默认消息
func errOrMsg(err error, def string) error {
	if err != nil {
		return err
	}
	return &chatErr{msg: def}
}

type chatErr struct{ msg string }

func (e *chatErr) Error() string { return e.msg }

// isChineseAnswer — 判断回答是否含中文（模型推理文本常为英文——回答应为中文）
func isChineseAnswer(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}
