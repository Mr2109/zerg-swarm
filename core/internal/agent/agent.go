package agent

// agent.go - 虫族 v2.5 Agent 核心结构（2026-08-13）
// 对齐：Claude Code query() 循环 + Loop Engineering
// 依赖：agentstate.HarnessState（Todo/Evidence/Quota）

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"zerg/core/internal/agentstate"
)

// TerminateReason - 循环终止状态（与 Claude Code 对齐）
type TerminateReason string

const (
	ReasonComplete     TerminateReason = "complete"     // 正常完成（checker 通过）
	ReasonUserAbort    TerminateReason = "user_abort"   // 用户中止
	ReasonTokenBudget  TerminateReason = "token_budget" // token 预算耗尽
	ReasonMaxTurns     TerminateReason = "max_turns"    // 超过最大轮数
	ReasonStopHook     TerminateReason = "stop_hook"    // 外部 stop hook
	ReasonMaxRetries   TerminateReason = "max_retries"  // 最大重试次数
	ReasonModelError   TerminateReason = "model_error"  // 模型调用失败
	ReasonToolError    TerminateReason = "tool_error"   // 工具不可恢复错误
	ReasonEscalate     TerminateReason = "escalate"     // 升级给人
	ReasonBlocked      TerminateReason = "blocked"      // 被门控拦截
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
	Model      string        `json:"model"`       // 模型
	GatewayURL string        `json:"gateway_url"` // 网关地址
	AuthToken  string        `json:"auth_token"` // X-Auth-Token
	MaxTurns   int           `json:"max_turns"` // 最大轮数
	Budget     int64         `json:"budget"`    // token 预算
	Timeout    time.Duration `json:"timeout"`    // 网关超时
	ContextWindow int        `json:"context_window"` // v2.5.4.9 滚动窗口压缩——保留最近 N 轮完整（默认 5——Mr2109）
	WorkDir    string        `json:"work_dir"`    // 工作区
	StateDir   string        `json:"state_dir"` // 状态目录
	Temperature float64      `json:"temperature"` // 采样温度（v2.5：默认 0.3——低温稳定工具调用）
}

// Terminator - 外部终止回调（gate 或用户中止循环）
type Terminator func(reason TerminateReason)

// Agent - 虫族 Agent（替代 Codex 的手）
type Agent struct {
	cfg           Config
	history       []Message
	sessionLog    *SessionLog // v2.5.5 P1: 会话日志（日志=上下文真相——2026-08-21 Mr2109）
	state         *agentstate.HarnessState
	gate          ToolGater
	checker       Checker
	client        *http.Client
	terminator    Terminator
	resultHandler func([]ToolCallResult)
	execContext   *ExecContext // 工具执行上下文（工作区/超时/输出上限）
	mcpMgr        *MCPManager  // v2.5.1 MCP 管理器（codegraph 等——动态工具）
	skillMgr      *SkillManager // v2.5.1 skill 管理器（SKILL.md 技能）
	adapter       ModelAdapter // v2.5.4.7 模型适配器插件（可选——nil=老逻辑）
	logger        *Logger      // v2.5.4.9 结构化日志（可选——nil 时用 stderr）
	turnCount     int          // v2.5.5 冷启动检测：callModel 调用计数（首轮=冷启动）
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
		cfg.MaxTurns = 30
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
		cfg.GatewayURL = "http://127.0.0.1:8082"
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

// callModel - 调网关（Responses API——/v1/responses——Codex 同款协议）
func (a *Agent) callModel(ctx context.Context, sysPrompt string, tools []ToolDef) (*ModelResponse, error) {
	a.turnCount++ // v2.5.5 冷启动计数（首轮=1）
	// v2.5.4.7：如果设置了适配器插件——走适配器（主控零模型假设）
	if a.adapter != nil {
		return a.adapter.Call(ctx, sysPrompt, a.history, tools)
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
			// Responses API 工具结果回喂：function_call_output item
			// 注意：须在 function_call 之后（配对顺序——先 call 后 output）
			msgs = append(msgs, map[string]any{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  m.Content,
			})
			continue
		}
		if len(m.ToolCalls) > 0 {
			// Responses API：assistant 的工具调用拆成顶层 function_call item（须在 output 之前）
			// 注意：assistant 文本消息须转 type:message item（llama 转换器不认裸 role:assistant——上游确认）
			if m.Content != "" {
				msgs = append(msgs, map[string]any{
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]any{{"type": "output_text", "text": m.Content}},
				})
			}
			for _, tc := range m.ToolCalls {
				msgs = append(msgs, map[string]any{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      tc.Name,
					"arguments": tc.RawArgs,
				})
			}
			continue
		}
		// 其他 assistant 消息（纯文本——无工具调用）也转 type:message item
		if m.Role == "assistant" && m.Content != "" {
			msgs = append(msgs, map[string]any{
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{{"type": "output_text", "text": m.Content}},
			})
			continue
		}
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	body := map[string]any{
		"model":       a.cfg.Model,
		"input":       msgs,
		"stream":      true, // v2.5.5 P1: 流式（2026-08-21 Mr2109——首 token 秒级反馈——Codex 借鉴）
		"temperature": a.cfg.Temperature,
	}
	if len(tools) > 0 {
		// v2.5.4.8 方向A：Responses API 工具格式——顶层 name/description/parameters
		respTools := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			respTools = append(respTools, map[string]any{
				"type":        "function",
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			})
		}
		body["tools"] = respTools
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}
	// v2.5.4.8 日志完善：FULL BODY 加开关（ZERG_DEBUG=1 才打印——平时不打——不刷屏）
	if os.Getenv("ZERG_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[callModel] FULL BODY: %s\n", string(data))
		fmt.Fprintf(os.Stderr, "[callModel] messages=%d tools=%d body=%d bytes tools_names=%v\n",
			len(msgs), len(tools), len(data), toolNames(tools))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.GatewayURL+"/v1/responses", strings.NewReader(string(data)))
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
		return nil, fmt.Errorf("调用网关失败(%s): %w", time.Since(start).Round(time.Millisecond), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		errStr := strings.TrimSpace(string(b))
		// v2.5.4.8 日志完善：错误详情完整打印（含响应体——找错必需）
		fmt.Fprintf(os.Stderr, "[callModel] ❌ HTTP %d (%s) url=%s\n  响应体: %s\n  请求体摘要: %d bytes\n",
			resp.StatusCode, time.Since(start).Round(time.Millisecond),
			a.cfg.GatewayURL+"/v1/responses",
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
	raw, err := streamReadResponses(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("流式读取失败: %w", err)
	}
	// v2.5.4.9 结构化日志：model_call 成功事件（含耗时/token 数——跟踪程序用）
	// 先解析响应（一次——日志用 tokens + 返回值用同一结果）
	parsed, perr := parseModelResponse(raw)
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

// streamReadResponses 流式读取 Responses API（SSE——逐 chunk——聚合完整响应 JSON）
// v2.5.5 P1（2026-08-21 Mr2109——Codex 借鉴——首 token 秒级反馈）
// 流式 chunk: {"type":"response.output_text.delta","delta":"好"} / ...output_text.done 等
// 聚合: 收集 delta 文本 + 拼接完整响应（结束输出 response.completed 完整 JSON）
func streamReadResponses(body io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var fullText strings.Builder
	var lastJSON []byte
	textDone := false

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		evType, _ := ev["type"].(string)
		switch evType {
		case "response.output_text.delta":
			if d, ok := ev["delta"].(string); ok {
				fullText.WriteString(d)
			}
		case "response.output_text.done":
			textDone = true
		case "response.completed":
			// 完整响应（含 output/tools/usage）——保留作为最终 JSON
			if resp, ok := ev["response"].(map[string]any); ok {
				lastJSON, _ = json.Marshal(resp)
			}
		case "error":
			b, _ := json.Marshal(ev)
			return nil, fmt.Errorf("流式错误: %s", string(b))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 有完整 JSON（response.completed）→ 用它（工具调用/usage 完整）
	if len(lastJSON) > 0 {
		return lastJSON, nil
	}
	// 无 completed（截断/兼容）→ 构造最小响应（text + output）
	if textDone || fullText.Len() > 0 {
		msg := map[string]any{
			"id":      "stream",
			"object":  "response",
			"status":  "completed",
			"output": []map[string]any{
				{"type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": fullText.String()}}},
			},
			"usage": map[string]any{"total_tokens": 0},
		}
		return json.Marshal(msg)
	}
	return []byte(`{"id":"stream","object":"response","status":"completed","output":[],"usage":{"total_tokens":0}}`), nil
}

// parseModelResponse 解析模型响应
type ModelResponse struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Finish    string
	Usage     struct {
		TotalTokens int64
	}
}
