package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Claude Anthropic Messages API 适配器（Claude Code）。
// 入站：Anthropic 格式（system 块 + content blocks + tool_use/tool_result）→ 后端 chat 格式。
// 出站：后端 chat SSE → Anthropic SSE 状态机（message_start → content_block_start/delta/stop → message_delta → message_stop）。
type Claude struct{}

func (a *Claude) Name() string { return "claude" }

func (a *Claude) Detect(r *http.Request, body []byte) bool {
	if r.URL.Path == "/v1/messages" {
		return true
	}
	// anthropic-version 头（Claude Code 必带）
	if r.Header.Get("anthropic-version") != "" {
		return true
	}
	return false
}

// TransformRequest 入站转换：Anthropic → chat。
func (a *Claude) TransformRequest(r *http.Request, body []byte) ([]byte, string, error) {
	var req struct {
		Model       string          `json:"model"`
		System      string          `json:"system"`
		Messages    []claudeMessage `json:"messages"`
		MaxTokens   int             `json:"max_tokens"`
		Stream      bool            `json:"stream"`
		Tools       []claudeTool    `json:"tools"`
		Temperature *float64        `json:"temperature,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, "", fmt.Errorf("解析 Anthropic 请求失败: %w", err)
	}

	// 构建 chat messages：system → 第一条 system message
	var chatMsgs []map[string]interface{}
	if req.System != "" {
		chatMsgs = append(chatMsgs, map[string]interface{}{
			"role":    "system",
			"content": req.System,
		})
	}
	for _, m := range req.Messages {
		chatMsgs = append(chatMsgs, m.toChatMessage())
	}

	// tools：Anthropic input_schema → OpenAI parameters
	var chatTools []map[string]interface{}
	for _, t := range req.Tools {
		chatTools = append(chatTools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}

	chatBody := map[string]interface{}{
		"model":    req.Model, // 保留原 model（核心层按原始 body 路由）
		"messages": chatMsgs,
		"stream":   req.Stream,
	}
	if req.MaxTokens > 0 {
		chatBody["max_tokens"] = req.MaxTokens
	}
	if len(chatTools) > 0 {
		chatBody["tools"] = chatTools
	}
	if req.Temperature != nil {
		chatBody["temperature"] = *req.Temperature
	}

	newBody, err := json.Marshal(chatBody)
	if err != nil {
		return nil, "", err
	}
	return newBody, "/v1/chat/completions", nil
}

// TransformResponse 出站转换：后端 chat SSE/JSON → Anthropic SSE 状态机。
func (a *Claude) TransformResponse(w http.ResponseWriter, resp *http.Response, req *http.Request, body []byte) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)

	// 心跳保活（Anthropic 惯例：后台推理长时发 ping）
	keepAliveDone := StartKeepAlive(w, flusher)
	defer close(keepAliveDone)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		WriteSSE(w, flusher, "error", `{"type":"error","error":{"type":"api_error","message":"读取响应失败"}}`)
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		w.Write(raw)
		return
	}

	// 从 chat 响应提取 content / reasoning / tool_calls
	choices, _ := obj["choices"].([]interface{})
	if len(choices) == 0 {
		w.Write(raw)
		return
	}
	choice, _ := choices[0].(map[string]interface{})
	msg, _ := choice["message"].(map[string]interface{})
	finish, _ := choice["finish_reason"].(string)

	// message_start
	msgID := "msg_" + uuidHex()
	WriteSSE(w, flusher, "message_start", fmt.Sprintf(
		`{"type":"message_start","message":{"id":%q,"type":"message","role":"assistant","model":"","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}}`,
		msgID))

	// reasoning（Anthropic thinking blocks）
	if msg != nil {
		if reasoning, _ := msg["reasoning_content"].(string); reasoning != "" {
			blockIdx := 0
			WriteSSE(w, flusher, "content_block_start", fmt.Sprintf(
				`{"type":"content_block_start","index":%d,"content_block":{"type":"thinking","thinking":""}}`, blockIdx))
			WriteSSE(w, flusher, "content_block_delta", fmt.Sprintf(
				`{"type":"content_block_delta","index":%d,"delta":{"type":"thinking_delta","thinking":%q}}`, blockIdx, escapeJSON(reasoning)))
			WriteSSE(w, flusher, "content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, blockIdx))
		}

		// text
		blockIdx := 0
		if text, _ := msg["content"].(string); text != "" {
			WriteSSE(w, flusher, "content_block_start", fmt.Sprintf(
				`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}`, blockIdx))
			WriteSSE(w, flusher, "content_block_delta", fmt.Sprintf(
				`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":%q}}`, blockIdx, escapeJSON(text)))
			WriteSSE(w, flusher, "content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, blockIdx))
		}

		// tool_use（chat tool_calls → Anthropic tool_use 块）
		if tcs, ok := msg["tool_calls"].([]interface{}); ok {
			for i, tc := range tcs {
				tcMap, _ := tc.(map[string]interface{})
				fn, _ := tcMap["function"].(map[string]interface{})
				name, _ := fn["name"].(string)
				argsRaw, _ := fn["arguments"].(string)
				// 规范化参数 + 转成 JSON 对象（Anthropic input 是对象不是字符串）
				argsRaw = normalizeFunctionArgs(argsRaw, name)
				var argsJSON interface{}
				json.Unmarshal([]byte(argsRaw), &argsJSON)
				if argsJSON == nil {
					argsJSON = map[string]interface{}{}
				}
				argsObj, _ := json.Marshal(argsJSON)
				callID := tcMap["id"].(string)
				if callID == "" {
					callID = "toolu_" + uuidHex()
				}
				blockIdx := i + 1 // 文本块之后
				WriteSSE(w, flusher, "content_block_start", fmt.Sprintf(
					`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":%q,"name":%q,"input":{}}}`, blockIdx, callID, escapeJSON(name)))
				WriteSSE(w, flusher, "content_block_delta", fmt.Sprintf(
					`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%q}}`, blockIdx, escapeJSON(string(argsObj))))
				WriteSSE(w, flusher, "content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, blockIdx))
			}
		}
	}

	// message_delta（usage + stop_reason）
	stopReason := "end_turn"
	if finish == "tool_calls" {
		stopReason = "tool_use"
	} else if finish == "length" {
		stopReason = "max_tokens"
	}
	WriteSSE(w, flusher, "message_delta", fmt.Sprintf(
		`{"type":"message_delta","delta":{"stop_reason":%q,"stop_sequence":null},"usage":{"output_tokens":1}}`, stopReason))
	WriteSSE(w, flusher, "message_stop", `{"type":"message_stop"}`)
}

// TransformError Anthropic 错误格式。
func (a *Claude) TransformError(w http.ResponseWriter, status int, code, msg string) {
	anthropicType := map[string]string{
		"invalid_request_error": "invalid_request_error",
		"not_found_error":       "not_found_error",
		"rate_limit_error":      "rate_limit_error",
		"timeout_error":         "timeout_error",
		"overloaded_error":      "overloaded_error",
		"api_error":             "api_error",
	}[code]
	if anthropicType == "" {
		anthropicType = "api_error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":%q,"message":%q}}`, anthropicType, msg)
}

// ========== Anthropic 请求结构 ==========

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // Anthropic 支持 string 或 []claudeContent
}

type claudeContent struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// parseContent 解析 content 字段：兼容 string（纯文本）或 []claudeContent（结构体）。
func (m claudeMessage) parseContent() []claudeContent {
	raw := m.Content
	if len(raw) == 0 {
		return nil
	}
	// 先试数组
	var arr []claudeContent
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	// 再试 string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []claudeContent{{Type: "text", Text: s}}
	}
	return nil
}

// toChatMessage 把 Anthropic 消息转成 chat 消息。
func (m claudeMessage) toChatMessage() map[string]interface{} {
	// 解析 content：Anthropic 支持 string 或 []claudeContent
	contents := m.parseContent()
	switch m.Role {
	case "assistant":
		// 提取 tool_use → tool_calls；text → content
		text := ""
		var toolCalls []map[string]interface{}
		for _, c := range contents {
			switch c.Type {
			case "text":
				text += c.Text
			case "tool_use":
				args := string(c.Input)
				if args == "" {
					args = "{}"
				}
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   c.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      c.Name,
						"arguments": args,
					},
				})
			}
		}
		return map[string]interface{}{
			"role":       "assistant",
			"content":    text,
			"tool_calls": toolCalls,
		}
	case "user":
		// 提取 text + tool_result → tool 角色消息
		text := ""
		var toolResults []map[string]interface{}
		for _, c := range contents {
			switch c.Type {
			case "text":
				text += c.Text
			case "tool_result":
				toolResults = append(toolResults, map[string]interface{}{
					"tool_call_id": c.ToolUseID,
					"role":         "tool",
					"content":      string(c.Content),
				})
			}
		}
		if len(toolResults) > 0 {
			return map[string]interface{}{
				"role":    "user",
				"content": text,
			}
		}
		return map[string]interface{}{
			"role":    "user",
			"content": text,
		}
	default:
		return map[string]interface{}{
			"role":    "user",
			"content": "",
		}
	}
}

// v2.5.4.9 清理: 删除 var _ = strings 侧链（strings import 同步删——未实际使用）
