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
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"

	"zerg/core/internal/agent"
	"zerg/core/internal/chat"
	"zerg/core/internal/loopcore"
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
	r.Patch("/api/chat/messages/{mid}", h.EditMessage)             // P0 消息编辑（点击编辑——Hermes user-edit 借鉴）
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
	// 2026-09-05 内核第三步A: 原走 chat.RunToolLoop（独立非流式循环——防循环裸奔已弃）
	// 改装 loopcore.Run（非流式形态——Events=nil）——与流式 /send 同一内核
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
	userMsg := &chat.Message{
		SessionID: id, Role: "user", Content: normalizeChatContent(req.Content),
		Active: true, Timestamp: chatNow(),
	}
	if _, err := h.store.AddMessage(userMsg); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	history, _ := h.store.GetActiveMessages(id)
	_, _ = h.store.CompressHistory(r.Context(), h.infer, id, se.Model, history)
	history, _ = h.store.GetActiveMessages(id)
	msgs := make([]map[string]any, 0, len(history))
	for _, m := range history {
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}
	var progRT *chat.ToolRuntime
	if progressiveEnabled {
		progRT = h.store.GetToolRuntime(id)
	}
	sysPrompt := chatSystemPrompt + fmt.Sprintf("\n\n# 你的身份\n- 你当前运行在模型 %s（虫族本地模型集群）——Mr2109的对话助手——不要调查或质疑自己的身份。", se.Model) + chat.BuildHermesToolPrompt(progRT)
	gate := &chat.ChatGate{}

	inferAdapter := func(ctx context.Context, model, sysP string, m []map[string]any,
		onDelta func(deltaType, text string), toolsParam []map[string]any) (*loopcore.Response, error) {
		ir, ierr := h.infer.Infer(ctx, model, sysP, m) // 非流式——Hermes 模式不带 tools 字段
		if ierr != nil {
			return nil, ierr
		}
		kr := &loopcore.Response{Content: ir.Content, Reasoning: ir.Reasoning, Finish: "stop",
			TotalTokens: int64(ir.InputTokens + ir.OutputTokens + ir.ReasoningTokens)}
		for _, tc := range ir.ToolCalls {
			kr.ToolCalls = append(kr.ToolCalls, loopcore.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args, RawArgs: tc.RawArgs})
		}
		return kr, nil
	}
	ec := agent.NewExecContext(chat.ChatToolsWorkDir)
	ec.AgentName = "chat"
	execFn := func(ctx context.Context, name string, targs map[string]any) (string, string, error) {
		if name == "kb_search" {
			query, _ := targs["query"].(string)
			limit := 10
			if l, ok := targs["limit"].(float64); ok {
				limit = int(l)
			}
			c, e := chat.KbSearchExecute(query, limit)
			return c, "", e
		}
		tc := agent.ToolCall{ID: "kernel", Name: name, Args: targs}
		result := ec.ExecuteTool(ctx, name, tc.Args, gate)
		if result.Error != "" {
			return result.Content, result.Duration, fmt.Errorf("%s", result.Error)
		}
		return result.Content, result.Duration, nil
	}
	kres := loopcore.Run(r.Context(), loopcore.Config{
		MaxRounds: chat.MaxToolRounds, WallClock: 600 * time.Second,
		RoundTimeout: 120 * time.Second, KeepRecent: 3,
	}, se.Model, sysPrompt, msgs, loopcore.Deps{Infer: inferAdapter, Exec: execFn})
	if kres.Err != "" {
		_ = h.store.DeleteMessage(id, userMsg.ID)
		writeChatError(w, http.StatusBadGateway, fmt.Errorf("%s", kres.Err))
		return
	}
	var traces []chat.ToolTrace
	for _, tr := range kres.Traces {
		traces = append(traces, chat.ToolTrace(tr))
	}
	var toolCallsStr string
	if len(traces) > 0 {
		tracesJSON, _ := json.Marshal(traces)
		toolCallsStr = string(tracesJSON)
	}
	msgContent := normalizeChatContent(kres.Content)
	msgReasoning := kres.Reasoning
	assistantMsg := &chat.Message{
		SessionID: id, Role: "assistant", Content: msgContent,
		Reasoning: msgReasoning, Model: se.Model,
		TokenCount: int(kres.Usage.TotalTokens), Active: true, Timestamp: chatNow(),
		ToolCalls: toolCallsStr,
	}
	if _, err := h.store.AddMessage(assistantMsg); err != nil {
		writeChatError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.TouchSession(id, int(kres.Usage.TotalTokens), 0, 0)
	if se.Title == "" {
		title := autoTitle(req.Content)
		_ = h.store.UpdateSessionTitle(id, title)
		se.Title = title
	}
	writeChatJSON(w, http.StatusOK, map[string]any{
		"content": msgContent, "reasoning": msgReasoning,
		"tool_calls": traces, "title": se.Title,
	})
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
		Image   string   `json:"image"`  // D3 多模态: data URL base64（单图——兼容）
		Images  []string `json:"images"` // P2 多图: data URL base64 数组（优先）
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
	// P4-46 Hermes 模式: 不带 tools 字段（内核 Deps.Tools=nil——模板 XML 分支不渲染——模型输出 <tool_call>JSON</tool_call>）

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

	ec := agent.NewExecContext(chat.ChatToolsWorkDir)
	ec.AgentName = "chat"
	var traces []chat.ToolTrace
	var result *chat.InferResult
	cur := msgs
	emptyArgsStreak := 0
	badFormatStreak := 0
	searchStreak := map[string]int{}
	searchNoUseStreak := 0
	guard := chat.NewLoopGuard(chat.DefaultLoopConfig())
	loopStart := time.Now()

	// 工具循环——2026-09-05 换装 loopcore 内核（对话循环两份合一第一步——
	// 五重防护/心跳/引导收尾全部沉淀进内核——此处只做装配）
	inferAdapter := func(ctx context.Context, model, sysPrompt string, m []map[string]any,
		onDelta func(deltaType, text string), toolsParam []map[string]any) (*loopcore.Response, error) {
		ir, ierr := h.infer.InferStream(ctx, model, sysPrompt, m, onDelta, toolsParam)
		if ierr != nil {
			return nil, ierr
		}
		kr := &loopcore.Response{
			Content: ir.Content, Reasoning: ir.Reasoning, Finish: "stop",
			TotalTokens: int64(ir.InputTokens + ir.OutputTokens + ir.ReasoningTokens),
		}
		for _, tc := range ir.ToolCalls {
			kr.ToolCalls = append(kr.ToolCalls, loopcore.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args, RawArgs: tc.RawArgs})
		}
		return kr, nil
	}
	var tracesMu sync.Mutex
	kres := loopcore.Run(r.Context(), loopcore.Config{
		MaxRounds:    chat.MaxToolRounds,
		WallClock:    600 * time.Second,
		RoundTimeout: 120 * time.Second,
		KeepRecent:   3,
	}, se.Model, sysPrompt, msgs, loopcore.Deps{
		Infer: inferAdapter,
		Exec: func(ctx context.Context, name string, targs map[string]any) (string, string, error) {
			// P4-36 异步+心跳（工具执行期间 SSE 不断流——UI 实时"执行中 N 秒"）
			type execRes struct {
				content string
				dur     string
				err     error
			}
			startT := time.Now()
			resCh := make(chan execRes, 1)
			go func() {
				var content string
				var dur string
				var execErr error
				if name == "kb_search" {
					query, _ := targs["query"].(string)
					limit := 10
					if l, ok := targs["limit"].(float64); ok {
						limit = int(l)
					}
					content, execErr = chat.KbSearchExecute(query, limit)
				} else {
					tc := agent.ToolCall{ID: "kernel", Name: name, Args: targs}
					result := ec.ExecuteTool(ctx, name, tc.Args, gate)
					content = result.Content
					dur = result.Duration
					if result.Error != "" {
						execErr = fmt.Errorf("%s", result.Error)
					}
				}
				resCh <- execRes{content, dur, execErr}
			}()
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			var res execRes
		waitLoop:
			for {
				select {
				case r := <-resCh:
					res = r
					break waitLoop
				case <-ticker.C:
					pingPayload, _ := json.Marshal(map[string]any{"name": name, "elapsed": int(time.Since(startT).Seconds())})
					writeSSE("tool_ping", string(pingPayload))
				case <-ctx.Done():
					res = execRes{err: fmt.Errorf("已取消")}
					break waitLoop
				}
			}
			ticker.Stop()
			tracesMu.Lock()
			traces = append(traces, chat.ToolTrace{Round: 0, CallID: "kernel", Name: name, Args: "", Result: res.content, Error: func() string {
				if res.err != nil {
					return res.err.Error()
				}
				return ""
			}(), Duration: res.dur})
			tracesMu.Unlock()
			return res.content, res.dur, res.err
		},
		Events: func(event, payload string) {
			if event == "delta" {
				writeSSE("delta", payload)
				return
			}
			writeSSE(event, payload)
		},
	})

	// 内核结果 → 原有落库/收尾形态（result/traces 适配）
	if kres.Err != "" {
		// 推理失败（重试后仍失败）——回滚 user 消息 + SSE 发 error
		_ = h.store.DeleteMessage(id, userMsgID)
		payload, _ := json.Marshal(map[string]any{"error": kres.Err})
		writeSSE("error", string(payload))
		return
	}
	for i := range kres.Traces {
		kres.Traces[i].Round = i + 1
	}
	if len(kres.Traces) > 0 {
		tb, _ := json.Marshal(kres.Traces)
		tracesJSON := string(tb)
		_ = tracesJSON
	}
	result = &chat.InferResult{
		Content: kres.Content, Reasoning: kres.Reasoning,
		InputTokens: int(kres.Usage.TotalTokens), // 内核只回总量——落库按 input 计（output 在 done 事件单算）
	}
	traces = traces[:0]
	for _, tr := range kres.Traces {
		traces = append(traces, chat.ToolTrace(tr))
	}
	cur = msgs
	_ = cur
	_ = emptyArgsStreak
	_ = badFormatStreak
	_ = searchStreak
	_ = searchNoUseStreak
	_ = guard
	_ = loopStart
	_ = ec

	// P4-48 最终回答质量检测: content 是推理文本（英文推理开头/无中文——ornith 通病）→ 重试一次"请用中文直接回答"
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
	// P4-50 Hermes 模式: ornith 推理写进 content（reasoning_content 空——无 tools 字段时训练格式如此）
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
		"message_id":       astID,
		"title":            se.Title,
		"input_tokens":     result.InputTokens,
		"output_tokens":    result.OutputTokens,
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
