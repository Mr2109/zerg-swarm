package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Codex OpenAI Responses API 适配器。
// 请求：responses 格式基本透传（llama-server 原生支持），合并 system/instructions、max_output_tokens、思考模型非流式。
// 响应：非流式 responses JSON → SSE 流式帧（Codex 强制流式；done 事件带完整 text——issue #2928）。
type Codex struct{}

func (a *Codex) Name() string { return "codex" }

func (a *Codex) Detect(r *http.Request, body []byte) bool {
	if r.URL.Path == "/v1/responses" {
		return true
	}
	// body 有 input 字段（responses 格式）
	if strings.Contains(string(body), "\"input\"") && !strings.Contains(string(body), "\"messages\"") {
		return true
	}
	return false
}

// TransformRequest 入站转换（responses 格式基本透传）。
func (a *Codex) TransformRequest(r *http.Request, body []byte) ([]byte, string, error) {
	// llama-server Jinja 要求 system 唯一且开头：合并 instructions + developer → 一条 system
	body = mergeResponsesSystem(body)
	// maxToken=上下文：补大 max_output_tokens 避免思考占满截断 message
	body = ensureMaxOutputTokens(body, 262144)
	// 思考模型流式 responses 只有 reasoning 无 message 块（Codex 解析失败）→
	// 强制非流式转发（非流式有完整 message），网关转 SSE 帧返回。
	body = forceNonStreamForThinkingModels(body)
	return body, "/v1/responses", nil
}

// TransformResponse 非流式 responses JSON → 客户端期望格式。
// 按请求 stream 参数分流：
//   - stream=true → 转 SSE 流式帧（Codex 强制流式）
//   - stream=false → 直接透传 JSON（Hermes 等客户端期待 JSON，转 SSE 会解析失败）
func (a *Codex) TransformResponse(w http.ResponseWriter, resp *http.Response, req *http.Request, body []byte) {
	if !IsStreamRequest(body) {
		// 客户端要 JSON：直接透传
		copyRawResponse(w, resp)
		return
	}
	copyResponseAsResponsesSSE(w, resp)
}

// copyRawResponse 原样透传响应（状态码+头+体）。
func copyRawResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// TransformError responses 错误格式（SSE 帧）。
func (a *Codex) TransformError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	flusher, _ := w.(http.Flusher)
	WriteSSE(w, flusher, "error", fmt.Sprintf(`{"type":"error","code":%q,"message":%q}`, code, msg))
}

// ========== Responses 转换（从 gateway.go 迁入） ==========

// copyResponseAsResponsesSSE 把非流式 responses JSON 转成 responses SSE 流式帧。
// Codex 发 stream:true，网关强制非流式后后端返回完整 JSON——
// 这里包装成 SSE 帧（response.created → output_item.added → delta → done → completed）。
func copyResponseAsResponsesSSE(w http.ResponseWriter, resp *http.Response) {
	// 先发 response.created（立即），再读完整响应。
	// Codex 等首帧判断连接存活；后端推理可能 30-60s，必须先发首帧避免客户端超时。
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)

	// 1. response.created
	respID := "resp_" + uuidHex()
	WriteSSE(w, flusher, "response.created", fmt.Sprintf(
		`{"type":"response.created","response":{"id":%q,"object":"response","status":"in_progress"}}`, respID))

	// 1.5 SSE 心跳保活（Codex 超时修复）：后端推理 30-60s，io.ReadAll 阻塞期间
	// 定时发 ": ping" 注释帧（SSE 标准：客户端必须忽略注释行，但连接保持活跃）。
	keepAliveDone := StartKeepAlive(w, flusher)

	body, err := io.ReadAll(resp.Body)
	close(keepAliveDone)
	if err != nil {
		WriteSSE(w, flusher, "error", `{"type":"error","message":"读取响应失败"}`)
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		// 非 JSON 直接透传 body（首帧已发）
		w.Write(body)
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	// 2. 遍历 output，把 message/function_call 转成流式事件
	output, _ := obj["output"].([]interface{})
	for idx, item := range output {
		m, _ := item.(map[string]interface{})
		switch m["type"] {
		case "message":
			itemID := "msg_" + uuidHex()
			// 提取完整文本（Codex 从 done 事件提取最终消息——issue #2928 教训：done 必须带 text）
			content, _ := m["content"].([]interface{})
			text := ""
			if len(content) > 0 {
				if cm, ok := content[0].(map[string]interface{}); ok {
					text, _ = cm["text"].(string)
				}
			}
			// output_item.added (message)
			WriteSSE(w, flusher, "response.output_item.added", fmt.Sprintf(
				`{"type":"response.output_item.added","output_index":%d,"item":{"id":%q,"type":"message","role":"assistant","status":"in_progress","content":[]}}`,
				idx, itemID))
			// content_part.added
			WriteSSE(w, flusher, "response.content_part.added", fmt.Sprintf(
				`{"type":"response.content_part.added","item_id":%q,"output_index":%d,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`,
				itemID, idx))
			// output_text.delta（完整文本作为一个 delta）
			WriteSSE(w, flusher, "response.output_text.delta", fmt.Sprintf(
				`{"type":"response.output_text.delta","item_id":%q,"output_index":%d,"content_index":0,"delta":"%s"}`,
				itemID, idx, escapeJSON(text)))
			// content_part.done（必须带完整 text——Codex 从这提取最终消息）
			WriteSSE(w, flusher, "response.content_part.done", fmt.Sprintf(
				`{"type":"response.content_part.done","item_id":%q,"output_index":%d,"content_index":0,"part":{"type":"output_text","text":"%s","annotations":[]}}`,
				itemID, idx, escapeJSON(text)))
			// output_item.done（content 必须带完整 text——issue #2928 教训）
			WriteSSE(w, flusher, "response.output_item.done", fmt.Sprintf(
				`{"type":"response.output_item.done","output_index":%d,"item":{"id":%q,"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"%s","annotations":[]}]}}`,
				idx, itemID, escapeJSON(text)))
		case "reasoning":
			// 思考内容：作为 reasoning item 输出
			itemID := "rs_" + uuidHex()
			content, _ := m["content"].([]interface{})
			text := ""
			if len(content) > 0 {
				if cm, ok := content[0].(map[string]interface{}); ok {
					text, _ = cm["text"].(string)
				}
			}
			WriteSSE(w, flusher, "response.output_item.added", fmt.Sprintf(
				`{"type":"response.output_item.added","output_index":%d,"item":{"id":%q,"type":"reasoning","summary":[],"content":[]}}`,
				idx, itemID))
			if text != "" {
				WriteSSE(w, flusher, "response.reasoning_text.delta", fmt.Sprintf(
					`{"type":"response.reasoning_text.delta","item_id":%q,"output_index":%d,"content_index":0,"delta":"%s"}`,
					itemID, idx, escapeJSON(text)))
			}
			WriteSSE(w, flusher, "response.output_item.done", fmt.Sprintf(
				`{"type":"response.output_item.done","output_index":%d,"item":{"id":%q,"type":"reasoning","content":[{"type":"reasoning_text","text":"%s","annotations":[]}]}}`,
				idx, itemID, escapeJSON(text)))
		case "function_call":
			// itemID 必须用模型返回的真实 call_id（Codex 用它关联 function_call ↔ function_call_output）。
			itemID, _ := m["call_id"].(string)
			if itemID == "" {
				itemID, _ = m["id"].(string)
			}
			if itemID == "" {
				itemID = "call_" + uuidHex()
			}
			name, _ := m["name"].(string)
			args, _ := m["arguments"].(string)
			// 规范化参数（x.0 → x，goal 枚举映射，Codex 兼容）
			args = normalizeFunctionArgs(args, name)
			WriteSSE(w, flusher, "response.output_item.added", fmt.Sprintf(
				`{"type":"response.output_item.added","output_index":%d,"item":{"id":%q,"type":"function_call","name":"%s","arguments":"","status":"in_progress","call_id":%q}}`,
				idx, itemID, escapeJSON(name), itemID))
			WriteSSE(w, flusher, "response.function_call_arguments.delta", fmt.Sprintf(
				`{"type":"response.function_call_arguments.delta","item_id":%q,"output_index":%d,"delta":"%s"}`,
				itemID, idx, escapeJSON(args)))
			WriteSSE(w, flusher, "response.function_call_arguments.done", fmt.Sprintf(
				`{"type":"response.function_call_arguments.done","item_id":%q,"output_index":%d,"arguments":"%s"}`,
				itemID, idx, escapeJSON(args)))
			WriteSSE(w, flusher, "response.output_item.done", fmt.Sprintf(
				`{"type":"response.output_item.done","output_index":%d,"item":{"id":%q,"type":"function_call","name":"%s","arguments":"%s","status":"completed","call_id":%q}}`,
				idx, itemID, escapeJSON(name), escapeJSON(args), itemID))
		}
	}

	// 3. response.completed（id 与 response.created 一致，Codex 解析必需；output 带完整内容——双保险）
	// 治本（2026-08-13）：思考模型（ornith）可能只返回 reasoning 无 message——最终答案在 reasoning 里
	// Codex 需要 message 输出——fallback：用 reasoning 最后内容生成 message（记忆铁律：思考模型 content 空读 reasoning_content）
	hasMessage := false
	for _, item := range output {
		if m, ok := item.(map[string]interface{}); ok && m["type"] == "message" {
			hasMessage = true
			break
		}
	}
	if !hasMessage {
		// 收集所有 reasoning text
		reasoningText := ""
		for _, item := range output {
			if m, ok := item.(map[string]interface{}); ok && m["type"] == "reasoning" {
				if c, ok := m["content"].([]interface{}); ok && len(c) > 0 {
					if cm, ok := c[0].(map[string]interface{}); ok {
						if t, ok := cm["text"].(string); ok {
							reasoningText += t
						}
					}
				}
			}
		}
		if reasoningText != "" {
			// 发一个 message item（答案 = reasoning 内容）
			itemID := "msg_" + uuidHex()
			msgIdx := len(output)
			WriteSSE(w, flusher, "response.output_item.added", fmt.Sprintf(
				`{"type":"response.output_item.added","output_index":%d,"item":{"id":%q,"type":"message","role":"assistant","status":"in_progress","content":[]}}`,
				msgIdx, itemID))
			WriteSSE(w, flusher, "response.content_part.added", fmt.Sprintf(
				`{"type":"response.content_part.added","item_id":%q,"output_index":%d,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`,
				itemID, msgIdx))
			WriteSSE(w, flusher, "response.output_text.delta", fmt.Sprintf(
				`{"type":"response.output_text.delta","item_id":%q,"output_index":%d,"content_index":0,"delta":"%s"}`,
				itemID, msgIdx, escapeJSON(reasoningText)))
			WriteSSE(w, flusher, "response.content_part.done", fmt.Sprintf(
				`{"type":"response.content_part.done","item_id":%q,"output_index":%d,"content_index":0,"part":{"type":"output_text","text":"%s","annotations":[]}}`,
				itemID, msgIdx, escapeJSON(reasoningText)))
			WriteSSE(w, flusher, "response.output_item.done", fmt.Sprintf(
				`{"type":"response.output_item.done","output_index":%d,"item":{"id":%q,"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"%s","annotations":[]}]}}`,
				msgIdx, itemID, escapeJSON(reasoningText)))
			// completed output 带 fallback message
			fbOutput := []interface{}{map[string]interface{}{
				"id":      itemID,
				"type":    "message",
				"role":    "assistant",
				"status":  "completed",
				"content": []interface{}{map[string]interface{}{"type": "output_text", "text": reasoningText, "annotations": []interface{}{}}},
			}}
			fbOutJSON, _ := json.Marshal(fbOutput)
			WriteSSE(w, flusher, "response.completed", fmt.Sprintf(
				`{"type":"response.completed","response":{"id":%q,"object":"response","status":"completed","output":%s}}`,
				respID, string(fbOutJSON)))
			return
		}
	}
	completedOutput := obj["output"]
	if completedOutput == nil {
		completedOutput = []interface{}{}
	}
	completedOutJSON, _ := json.Marshal(completedOutput)
	WriteSSE(w, flusher, "response.completed", fmt.Sprintf(
		`{"type":"response.completed","response":{"id":%q,"object":"response","status":"completed","output":%s}}`,
		respID, string(completedOutJSON)))
}

// ========== responses 请求转换辅助（从 gateway.go 迁入） ==========

// mergeResponsesSystem 合并 Responses 的 instructions + developer → 一条 system。
// llama-server Jinja 要求 system 唯一且在开头；Codex 有顶层 instructions + input 里 developer
// （也映射 system）→ 两个 system 冲突 → 500。
func mergeResponsesSystem(body []byte) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}

	// 收集 instructions + input 里的 developer/system 消息
	var systemParts []string
	if instr, ok := obj["instructions"].(string); ok && instr != "" {
		systemParts = append(systemParts, instr)
	}

	input, _ := obj["input"].([]interface{})
	var newInput []interface{}
	for _, item := range input {
		m, _ := item.(map[string]interface{})
		role, _ := m["role"].(string)
		if role == "developer" || role == "system" {
			if content, ok := m["content"].([]interface{}); ok {
				for _, c := range content {
					if cm, ok := c.(map[string]interface{}); ok {
						if t, ok := cm["text"].(string); ok && t != "" {
							systemParts = append(systemParts, t)
						}
					}
				}
			}
			continue // 移除原 developer/system
		}
		newInput = append(newInput, item)
	}

	if len(systemParts) > 0 {
		// 合并成一条 system 放最前
		systemMsg := map[string]interface{}{
			"type":    "message",
			"role":    "system",
			"content": []interface{}{map[string]interface{}{"type": "input_text", "text": strings.Join(systemParts, "\n\n")}},
		}
		newInput = append([]interface{}{systemMsg}, newInput...)
		obj["input"] = newInput
		// ★ 删除顶层 instructions（内容已并入 system 消息，保留会导致双 system 冲突 → 后端 500）
		delete(obj, "instructions")
	}

	newBody, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return newBody
}

// ensureMaxOutputTokens 补大 max_output_tokens（避免思考占满截断 message）。
func ensureMaxOutputTokens(body []byte, minTokens int) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}
	if _, ok := obj["max_output_tokens"]; !ok {
		obj["max_output_tokens"] = minTokens
	}
	newBody, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return newBody
}

// forceNonStreamForThinkingModels 思考模型强制非流式。
func forceNonStreamForThinkingModels(body []byte) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}
	obj["stream"] = false
	newBody, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return newBody
}

// v2.5.4.9 清理: 删除 var _ = regexp 侧链（regexp import 同步删——未实际使用）
