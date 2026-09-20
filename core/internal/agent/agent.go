package agent

// agent.go - 虫族 v2.5 Agent 核心结构（2026-08-13）
// 对齐：Claude Code query() 循环 + Loop Engineering
// 依赖：agentstate.HarnessState（Todo/Evidence/Quota）

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agentstate"
	"github.com/Mr2109/zerg-swarm/core/internal/hermes"
)

// TerminateReason - 循环终止状态（与 Claude Code 对齐）
type TerminateReason string

const (
	ReasonComplete    TerminateReason = "complete"     // 正常完成（checker 通过）
	ReasonUserAbort   TerminateReason = "user_abort"   // 用户中止
	ReasonTokenBudget TerminateReason = "token_budget" // token 预算耗尽
	ReasonMaxTurns    TerminateReason = "max_turns"    // 超过最大轮数
	ReasonStopHook    TerminateReason = "stop_hook"    // 外部 stop hook
	ReasonMaxRetries  TerminateReason = "max_retries"  // 最大重试次数
	ReasonModelError  TerminateReason = "model_error"  // 模型调用失败
	ReasonToolError   TerminateReason = "tool_error"   // 工具不可恢复错误
	ReasonEscalate    TerminateReason = "escalate"     // 升级给人
	ReasonBlocked     TerminateReason = "blocked"      // 被门控拦截
)

// ToolCall - 模型输出的工具调用
type ToolCall struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Args    map[string]any `json:"args"`
	RawArgs string         `json:"raw_args"`
	RawCall string         `json:"raw_call,omitempty"` // P4-47 模型输出原文（Hermes <tool_call> 原文——回传上下文用）
}

// Message - 对话消息（不可变，每轮新数组）
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// Config - Agent 配置
type Config struct {
	Model         string        `json:"model"`          // 模型
	GatewayURL    string        `json:"gateway_url"`    // 网关地址
	AuthToken     string        `json:"auth_token"`     // X-Auth-Token
	MaxTurns      int           `json:"max_turns"`      // 最大轮数
	Budget        int64         `json:"budget"`         // token 预算
	Timeout       time.Duration `json:"timeout"`        // 网关超时
	ContextWindow int           `json:"context_window"` // v2.5.4.9 滚动窗口压缩——保留最近 N 轮完整（默认 5——Mr2109）
	WorkDir       string        `json:"work_dir"`       // 工作区
	StateDir      string        `json:"state_dir"`      // 状态目录
	Temperature   float64       `json:"temperature"`    // 采样温度（v2.5：默认 0.3——低温稳定工具调用）
}

// Terminator - 外部终止回调（gate 或用户中止循环）
type Terminator func(reason TerminateReason)

// Agent - 虫族 Agent（替代 Codex 的手）
type Agent struct {
	cfg            Config
	history        []Message
	sessionLog     *SessionLog // v2.5.5 P1: 会话日志（日志=上下文真相——2026-08-21 Mr2109）
	state          *agentstate.HarnessState
	gate           ToolGater
	checker        Checker
	client         *http.Client
	terminator     Terminator
	resultHandler  func([]ToolCallResult)
	execContext    *ExecContext   // 工具执行上下文（工作区/超时/输出上限）
	mcpMgr         *MCPManager    // v2.5.1 MCP 管理器（codegraph 等——动态工具）
	skillMgr       *SkillManager  // v2.5.1 skill 管理器（SKILL.md 技能）
	adapter        ModelAdapter   // v2.5.4.7 模型适配器插件（可选——nil=老逻辑）
	logger         *Logger        // v2.5.4.9 结构化日志（可选——nil 时用 stderr）
	turnCount      int            // v2.5.5 冷启动检测：callModel 调用计数（首轮=冷启动）
	rollingSummary rollingSummary // v2.5.4.9 滚动窗口压缩——累积摘要（压缩轮次保留）
}

// SetLogger 设置结构化日志（v2.5.4.9——callModel 事件写入）。
func (a *Agent) SetLogger(l *Logger) {
	a.logger = l
}

// ModelAdapter 模型适配器接口（v2.5.4——主控零模型假设——适配器封装模型脾气）。
type ModelAdapter interface {
	Call(ctx context.Context, sysPrompt string, messages []Message, tools []ToolDef) (*ModelResponse, error)
}

// SetAdapter 设置模型适配器（v2.5.4.7——注入插件——nil 保持老逻辑）。
func (a *Agent) SetAdapter(ad ModelAdapter) {
	a.adapter = ad
}

// SetMCPManager — 设置 MCP 管理器（main 连接后注入）
func (a *Agent) SetMCPManager(m *MCPManager) {
	a.mcpMgr = m
}

// SetSkillManager — 设置 skill 管理器（main 初始化后注入）
func (a *Agent) SetSkillManager(sm *SkillManager) {
	a.skillMgr = sm
}

// NewAgent - 创建 Agent，填充默认值
func NewAgent(cfg Config) *Agent {
	if cfg.MaxTurns == 0 {
		// 唯一真源见 max_turns.go（§二十一 已红第 12 条 · 批 C 的 T-34）——这里原来是字面量 30，
		// 与命令行默认（100）和子任务配额（20）各写一个数 ⇒ 同一条上限三个真源。
		cfg.MaxTurns = DefaultMaxTurns
	}
	// v2.5.4.9 滚动窗口压缩——保留最近 N 轮完整（默认 5——Mr2109——业界 5-8 轮）
	if cfg.ContextWindow == 0 {
		cfg.ContextWindow = 5
	}
	// v2.5.4.9 本地模型 token 预算无限制（Mr2109——本地模型不需预算——CA 可无限跑）
	//   Budget=0 → loop 不触发预算检查（loop.go:310 budget>0 才检查）——本地模型无限制
	if cfg.Timeout == 0 {
		cfg.Timeout = 300 * time.Second // v2.5 调优：120s→300s（bash 任务 Qwable 响应慢）
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "/tmp/zerg-agent"
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.8 // v2.5：实测 0.3 更差（example-35b-v2 保守不调工具）——恢复 0.8（波动靠提示/重试缓解）
	}
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = statepath.GatewayBaseURL()
	}
	if cfg.AuthToken == "" {
		cfg.AuthToken = "example-35b"
	}
	a := &Agent{
		cfg:         cfg,
		client:      &http.Client{Timeout: cfg.Timeout},
		execContext: NewExecContext(cfg.WorkDir),
	}
	// v2.5.5 P2: 注入父 agent 引用（spawn_agent 子 agent 委托——2026-08-21 Mr2109）
	a.execContext.Parent = a
	return a
}

// SetGate - 注入 M3 gate（工具拦截）
func (a *Agent) SetGate(g ToolGater) {
	a.gate = g
}

// SetChecker - 注入验证器（maker/checker 分离）
func (a *Agent) SetChecker(c Checker) {
	a.checker = c
}

// SetTerminator - 注入终止回调
func (a *Agent) SetTerminator(t Terminator) {
	a.terminator = t
}

// SetResultHandler - 注入工具结果处理器
func (a *Agent) SetResultHandler(f func([]ToolCallResult)) {
	a.resultHandler = f
}

// SetState - 注入 HarnessState
func (a *Agent) SetState(s *agentstate.HarnessState) {
	a.state = s
}

// State - 获取当前 HarnessState
func (a *Agent) State() *agentstate.HarnessState {
	return a.state
}

// History - 获取对话历史（只读）
func (a *Agent) History() []Message {
	return a.history
}

// AppendMessage - 追加消息到历史（任务指令/用户输入）
// v2.5.5 P1: 同时写会话日志（日志=上下文真相——模型可见即已记录——2026-08-21 Mr2109）
func (a *Agent) AppendMessage(m Message) {
	a.history = append(a.history, m)
	if a.sessionLog != nil {
		a.sessionLog.Append(m)
	}
}

// SetSessionLog - 注入会话日志（main 创建 Agent 后设置）
func (a *Agent) SetSessionLog(sl *SessionLog) {
	a.sessionLog = sl
}

// RestoreFromLog - 从会话日志恢复历史（CA 崩溃/续跑——日志=权威）
// 返回恢复的消息数（0=无日志/新会话）
func (a *Agent) RestoreFromLog() int {
	if a.sessionLog == nil {
		return 0
	}
	msgs, err := a.sessionLog.Load()
	if err != nil || len(msgs) == 0 {
		return 0
	}
	// 恢复（日志=真相——覆盖内存）
	a.history = msgs
	return len(msgs)
}

// callModel - 调网关（2026-09-05 协议统一: /v1/chat/completions——对齐 2026-08-30 铁律——
// 原走 /v1/responses 与对话系统协议分叉——chat 格式字符串 content 历史完全兼容（实测））
func (a *Agent) callModel(ctx context.Context, sysPrompt string, tools []ToolDef) (*ModelResponse, error) {
	a.turnCount++ // v2.5.5 冷启动计数（首轮=1）
	// v2.5.4.7：如果设置了适配器插件——走适配器（主控零模型假设）
	if a.adapter != nil {
		return a.adapter.Call(ctx, sysPrompt, a.history, tools)
	}
	// 2026-09-05 CA Hermes 化: ZERG_HERMES_TOOLS=1 走 Hermes 协议（不带 tools 字段——
	// 提示注入 <tools> schema——模型输出 <tool_call>XML——治 X3 长 prompt 下畸形 arguments）
	if os.Getenv("ZERG_HERMES_TOOLS") == "1" {
		return a.callModelHermes(ctx, sysPrompt, tools)
	}
	// 老逻辑（未设适配器——兼容）
	msgs := make([]map[string]any, 0, len(a.history)+2)
	msgs = append(msgs, map[string]any{"role": "system", "content": sysPrompt})
	// v2.5.4.9 滚动窗口压缩：摘要区（最早轮压缩的累积——模型可参考早期上下文）
	if sum := a.buildSummaryBlock(); sum != nil {
		msgs = append(msgs, map[string]any{"role": "system", "content": sum.Content})
	}
	for _, m := range a.history {
		if m.Role == "tool" {
			// chat 格式工具结果: role=tool + tool_call_id（2026-09-05 协议统一）
			msgs = append(msgs, map[string]any{
				"role":         "tool",
				"tool_call_id": m.ToolCallID,
				"content":      m.Content,
			})
			continue
		}
		if len(m.ToolCalls) > 0 {
			// chat 格式 assistant 工具调用: message.tool_calls[]（历史 assistant 完全兼容）
			tcs := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.RawArgs,
					},
				})
			}
			msgs = append(msgs, map[string]any{
				"role":       "assistant",
				"content":    m.Content,
				"tool_calls": tcs,
			})
			continue
		}
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	body := map[string]any{
		"model":       a.cfg.Model,
		"messages":    msgs,
		"stream":      true, // v2.5.5 P1: 流式（首 token 秒级反馈——聚合后返回完整响应）
		"temperature": a.cfg.Temperature,
		"reasoning":   map[string]any{"effort": "low"}, // 思考不能关（Mr2109铁律——与对话系统一致）
	}
	// Hermes 模式不带 tools 字段（P4-46——模板 XML 分支不渲染——模型输出 <tool_call>）
	// 工具定义已在 callModelHermes 注入 sysPrompt
	if len(tools) > 0 && os.Getenv("ZERG_HERMES_TOOLS") != "1" {
		// chat 格式工具: tools[].function
		chatTools := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			chatTools = append(chatTools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
				},
			})
		}
		body["tools"] = chatTools
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request body: %w", err)
	}
	// v2.5.4.8 日志完善：FULL BODY 加开关（ZERG_DEBUG=1 才打印——平时不打——不刷屏）
	if os.Getenv("ZERG_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[callModel] FULL BODY: %s\n", string(data))
		fmt.Fprintf(os.Stderr, "[callModel] messages=%d tools=%d body=%d bytes tools_names=%v\n",
			len(msgs), len(tools), len(data), toolNames(tools))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.GatewayURL+"/v1/chat/completions", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.cfg.AuthToken != "" {
		req.Header.Set("X-Auth-Token", a.cfg.AuthToken)
	}

	start := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway call failed (%s): %w", time.Since(start).Round(time.Millisecond), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		errStr := strings.TrimSpace(string(b))
		// v2.5.4.8 日志完善：错误详情完整打印（含响应体——找错必需）
		fmt.Fprintf(os.Stderr, "[callModel] ❌ HTTP %d (%s) url=%s\n  响应体: %s\n  请求体摘要: %d bytes\n",
			resp.StatusCode, time.Since(start).Round(time.Millisecond),
			a.cfg.GatewayURL+"/v1/chat/completions",
			errStr, len(data))
		// v2.5.4.9 结构化日志：model_call 失败事件
		if a.logger != nil {
			a.logger.LogEvent(string(EventModelCall), "error", "model_call_failed", "",
				sysPrompt, map[string]any{"status": resp.StatusCode, "body_bytes": len(data)},
				"", errStr, time.Since(start).String())
		}
		return nil, fmt.Errorf("网关返回 %d: %s", resp.StatusCode, errStr)
	}

	// v2.5.5 P1: 流式响应解析（2026-08-21 Mr2109——SSE 逐 chunk——聚合完整响应）
	// 流式: 每行 "data: {...}"——聚合 response.delta 文本——结束聚合完整 JSON
	raw, err := streamReadChat(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stream read failed: %w", err)
	}
	// Hermes 模式: 从正文解析 <tool_call>（chat 解析器只认 message.tool_calls）
	if os.Getenv("ZERG_HERMES_TOOLS") == "1" {
		return parseHermesChatResult(raw)
	}
	// v2.5.4.9 结构化日志：model_call 成功事件（含耗时/token 数——跟踪程序用）
	// 先解析响应（一次——日志用 tokens + 返回值用同一结果）
	parsed, perr := parseChatModelResponse(raw)
	if a.logger != nil {
		usage := 0
		if perr == nil {
			usage = int(parsed.Usage.TotalTokens)
		}
		// v2.5.5 冷启动标记：首轮（turnCount==1）请求 = 冷启动（加载+首推理）——后续轮 = 热
		coldStart := a.turnCount == 1
		a.logger.LogEvent(string(EventModelCall), "info", "model_call_ok", "",
			sysPrompt, map[string]any{"body_bytes": len(raw), "status": resp.StatusCode, "tokens": usage, "cold_start": coldStart},
			"", "", time.Since(start).String())
	}
	if perr != nil {
		return nil, perr
	}
	return parsed, nil
}

// streamReadChat 流式读取 chat completions SSE（逐 chunk 聚合完整 message JSON）
// chunk: choices[].delta {content/reasoning_content/tool_calls} + usage
// 聚合产物: {"choices":[{"message":{...}}],"usage":{...},"reasoning_content":...}——parseChatModelResponse 同构
func streamReadChat(body io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var content strings.Builder
	var reasoning strings.Builder
	tcMap := map[int]*map[string]any{} // index → {id,type,function:{name,arguments}}
	usage := map[string]any{}
	finish := ""
	has := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
					FinishReason *string `json:"finish_reason"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *map[string]any `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue // 容错跳过坏 chunk
		}
		if ev.Error != nil && ev.Error.Message != "" {
			return nil, fmt.Errorf("stream error: %s", ev.Error.Message)
		}
		has = true
		for _, ch := range ev.Choices {
			content.WriteString(ch.Delta.Content)
			reasoning.WriteString(ch.Delta.ReasoningContent)
			for _, dtc := range ch.Delta.ToolCalls {
				m, ok := tcMap[dtc.Index]
				if !ok {
					nm := map[string]any{
						"id":   dtc.ID,
						"type": "function",
						"function": map[string]any{
							"name":      dtc.Function.Name,
							"arguments": dtc.Function.Arguments,
						},
					}
					tcMap[dtc.Index] = &nm
					m = &nm
				} else {
					if dtc.ID != "" {
						(*m)["id"] = dtc.ID
					}
					fn, _ := (*m)["function"].(map[string]any)
					if fn == nil {
						fn = map[string]any{}
						(*m)["function"] = fn
					}
					if dtc.Function.Name != "" {
						fn["name"] = dtc.Function.Name
					}
					prev, _ := fn["arguments"].(string)
					fn["arguments"] = prev + dtc.Function.Arguments
				}
			}
			if ch.Delta.FinishReason != nil && *ch.Delta.FinishReason != "" {
				finish = *ch.Delta.FinishReason
			}
		}
		if ev.Usage != nil {
			usage = *ev.Usage
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !has {
		return []byte(`{"choices":[{"message":{"content":"","finish_reason":"stop"}}],"usage":{"total_tokens":0}}`), nil
	}
	msg := map[string]any{"role": "assistant", "content": content.String()}
	if len(tcMap) > 0 {
		idxs := make([]int, 0, len(tcMap))
		for i := range tcMap {
			idxs = append(idxs, i)
		}
		sort.Ints(idxs)
		tcs := make([]map[string]any, 0, len(idxs))
		for _, i := range idxs {
			tcs = append(tcs, *tcMap[i])
		}
		msg["tool_calls"] = tcs
	}
	out := map[string]any{
		"choices": []map[string]any{{"message": msg, "finish_reason": finish}},
		"usage":   usage,
	}
	if reasoning.Len() > 0 {
		out["reasoning_content"] = reasoning.String()
	}
	return json.Marshal(out)
}

// callModelHermes — CA Hermes 模式（2026-09-05——治 X3 长 prompt 下 tools 字段畸形 arguments）
// 与对话系统 P4-46 同模式: 不带 tools 字段 → sysPrompt 注入 <tools> schema → 模型输出 <tool_call>XML
// 工具结果回传: assistant content=<tool_call> 原文 + user content=<tool_response>（模型训练见过的格式）
func (a *Agent) callModelHermes(ctx context.Context, sysPrompt string, tools []ToolDef) (*ModelResponse, error) {
	// 消息组装（chat 格式——工具结果用 <tool_response> 文本回传——非 role=tool）
	msgs := make([]map[string]any, 0, len(a.history)+2)
	// 工具 schema 注入 sysPrompt（P4-46——hermes 包构建）
	hermesTools := make([]hermes.ToolSchema, 0, len(tools))
	for _, t := range tools {
		hermesTools = append(hermesTools, hermes.ToolSchema{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	sysPrompt += hermes.BuildToolPrompt(hermesTools, "")
	msgs = append(msgs, map[string]any{"role": "system", "content": sysPrompt})
	if sum := a.buildSummaryBlock(); sum != nil {
		msgs = append(msgs, map[string]any{"role": "system", "content": sum.Content})
	}
	for _, m := range a.history {
		if m.Role == "tool" {
			// Hermes 模式: 工具结果作为 user 消息回传（<tool_response> 已含在 content 里——回传时直接用）
			msgs = append(msgs, map[string]any{"role": "user", "content": m.Content})
			continue
		}
		// assistant 的 <tool_call> 原文保留在 content（模型看过自己的调用历史）
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	body := map[string]any{
		"model":       a.cfg.Model,
		"messages":    msgs,
		"stream":      false, // Hermes 模式非流式（聚合简单——CA 不需要逐字）
		"temperature": a.cfg.Temperature,
		"reasoning":   map[string]any{"effort": "low"},
		// 注意: 不带 tools 字段（P4-46 核心）
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request body: %w", err)
	}
	if os.Getenv("ZERG_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[callModelHermes] body=%d bytes history=%d\n", len(data), len(a.history))
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.GatewayURL+"/v1/chat/completions", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.cfg.AuthToken != "" {
		req.Header.Set("X-Auth-Token", a.cfg.AuthToken)
	}
	start := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway call failed (%s): %w", time.Since(start).Round(time.Millisecond), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("网关返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseHermesChatResult(raw)
}

// parseHermesChatResult — 解析 Hermes 模式响应（content 含 <tool_call> → XML 解析）
func parseHermesChatResult(raw []byte) (*ModelResponse, error) {
	parsed, err := parseChatModelResponse(raw)
	if err != nil {
		return nil, err
	}
	// 从正文解析 <tool_call>
	xmlCalls := hermes.ParseXMLToolCalls(parsed.Content)
	if len(xmlCalls) > 0 {
		for _, xc := range xmlCalls {
			parsed.ToolCalls = append(parsed.ToolCalls, ToolCall{
				ID:      fmt.Sprintf("hermes_%d", time.Now().UnixNano()%100000),
				Name:    xc.Name,
				Args:    xc.Args,
				RawArgs: xc.RawArgs,
			})
		}
		parsed.Content = hermes.StripXMLToolCalls(parsed.Content)
	}
	return parsed, nil
}
