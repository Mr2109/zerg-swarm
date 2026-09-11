package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Chat OpenAI Chat Completions 适配器（Hermes / 龙虾 OpenClaw 共用）。
// 请求：原样透传（llama-server 原生支持 chat 格式）。
// 响应：非流式 JSON 透传；流式 SSE 透传（llama-server chat 流式已兼容 OpenAI 格式）。
type Chat struct{}

func (a *Chat) Name() string { return "chat" }

func (a *Chat) Detect(r *http.Request, body []byte) bool {
	if r.URL.Path == "/v1/chat/completions" {
		return true
	}
	// 非 responses/messages 路径 + body 有 messages 字段 → chat
	if r.URL.Path != "/v1/responses" && r.URL.Path != "/v1/messages" &&
		!strings.Contains(string(body), "\"input\"") {
		return true
	}
	return false
}

// TransformRequest chat 格式原样透传（后端 chat 端点）。
// 工具调用兼容（GitHub #5769）：含 tools 时强制非流式转发——
// 本地模型流式只推 content/reasoning 不推 tool_calls，流式响应会丢工具调用。
// 强制非流式拿到完整 JSON（含 tool_calls），响应时再转 SSE。
// schema 容错：清理不合规 pattern（龙虾等客户端生成的非标准 JSON Schema）。
func (a *Chat) TransformRequest(r *http.Request, body []byte) ([]byte, string, error) {
	if ContainsTools(body) {
		body = sanitizeToolsSchema(body)
		return JsonSetField(body, "stream", false), "/v1/chat/completions", nil
	}
	return body, "/v1/chat/completions", nil
}

// sanitizeToolsSchema 清理工具 schema 里的不合规 pattern 字段。
// 背景：龙虾（OpenClaw）生成的 tools 里 pattern 不带 ^$ 前缀/后缀，
// llama-server 报 "Pattern must start with '^' and end with '$'" 400 拒绝。
// 处理：递归遍历 parameters，删除不以 ^ 开头或以 $ 结尾的 pattern 字段
// （非标准 JSON Schema，删除后语义无损）。
func sanitizeToolsSchema(body []byte) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}
	tools, ok := obj["tools"].([]interface{})
	if !ok {
		return body
	}
	for _, t := range tools {
		tm, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		fn, ok := tm["function"].(map[string]interface{})
		if !ok {
			continue
		}
		if params, ok := fn["parameters"].(map[string]interface{}); ok {
			sanitizePattern(params)
		}
	}
	newBody, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return newBody
}

// sanitizePattern 递归清理 schema 对象里的非法 pattern。
func sanitizePattern(node map[string]interface{}) {
	// 清理当前层的 pattern
	if p, ok := node["pattern"].(string); ok {
		if !strings.HasPrefix(p, "^") || !strings.HasSuffix(p, "$") {
			delete(node, "pattern")
		}
	}
	// 递归 properties
	if props, ok := node["properties"].(map[string]interface{}); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]interface{}); ok {
				sanitizePattern(sub)
			}
		}
	}
	// 递归 items
	if items, ok := node["items"].(map[string]interface{}); ok {
		sanitizePattern(items)
	}
	// 递归 $defs / definitions
	for _, key := range []string{"$defs", "definitions"} {
		if defs, ok := node[key].(map[string]interface{}); ok {
			for _, v := range defs {
				if sub, ok := v.(map[string]interface{}); ok {
					sanitizePattern(sub)
				}
			}
		}
	}
}

// TransformResponse 响应透传：
//   - 含 tools（已被强制非流式转发）→ 按客户端 stream 参数分流：
//     客户端 stream=true → 完整 JSON 转 SSE（工具调用兼容）
//     客户端 stream=false → 直接透传 JSON（客户端期待 JSON，转 SSE 会解析失败）
//   - 流式 SSE（无工具）→ 透传
//   - 非流式 JSON（无工具）→ 透传
func (a *Chat) TransformResponse(w http.ResponseWriter, resp *http.Response, req *http.Request, body []byte) {
	hasTools := ContainsTools(body)
	isStream := IsStreamRequest(body)
	log.Printf("[adapter:chat] response status=%d ct=%s hasTools=%v isStream=%v", resp.StatusCode, resp.Header.Get("Content-Type"), hasTools, isStream)

	if resp.StatusCode >= 400 {
		// 后端错误：透传（龙虾等客户端按自己格式解析）
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[adapter:chat] backend error %d: %s", resp.StatusCode, string(errBody)[:min(400, len(errBody))])
		w.WriteHeader(resp.StatusCode)
		w.Write(errBody)
		return
	}

	if isStream && !hasTools && IsStreamResponse(resp) {
		// 纯流式透传（无工具，后端原生流式）
		copySSE(w, resp)
		return
	}
	if hasTools {
		if isStream {
			// 客户端要流式：强制非流式拿到的完整 JSON → 转 SSE（工具调用兼容）
			copyResponseAsSSE(w, resp)
		} else {
			// 客户端要 JSON：直接透传完整 JSON（含 tool_calls）
			copyResponse(w, resp)
		}
		return
	}
	// 非流式 JSON 透传
	copyResponse(w, resp)
}

// TransformError chat 错误格式：{"error":{"message":...}}
func (a *Chat) TransformError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"message":%q,"type":%q}}`, msg, code)
}

// ========== chat 响应透传辅助（从 gateway.go 迁入） ==========

// copyResponse 原样复制响应体（common.go 导出版）。
func copyResponse(w http.ResponseWriter, resp *http.Response) { CopyResponse(w, resp) }

// copySSE 透传 SSE 流（common.go 导出版，ReadBytes 保真 CRLF）。
func copySSE(w http.ResponseWriter, resp *http.Response) { CopySSE(w, resp) }

// copyResponseAsSSE 把非流式 chat JSON 转成 SSE 流（工具调用兼容）。
// 背景（GitHub #5769）：本地模型流式只推 content/reasoning 不推 tool_calls，
// 客户端（OpenClaw）流式请求 + tools 时工具调用丢失 → 强制非流式转发 → 这里转 SSE。
func copyResponseAsSSE(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		copyResponse(w, resp)
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		// 非 JSON 直接透传
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)

	// 从 choices[0].message 提取 content / reasoning / tool_calls
	choices, _ := obj["choices"].([]interface{})
	if len(choices) == 0 {
		// 无 choices：透传原始 JSON
		w.Write(body)
		return
	}
	msg, _ := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	finish, _ := choices[0].(map[string]interface{})["finish_reason"].(string)

	// 逐段输出 content（按行切分近似流式）
	if msg != nil {
		if content, _ := msg["content"].(string); content != "" {
			writeChatDelta(w, flusher, content, finish)
		}
		if reasoning, _ := msg["reasoning_content"].(string); reasoning != "" {
			writeChatReasoning(w, flusher, reasoning)
		}
		if tcs, ok := msg["tool_calls"].([]interface{}); ok {
			writeChatToolCalls(w, flusher, tcs, finish)
		}
	}

	// done 帧
	writeChatDone(w, flusher, obj)
}

func writeChatDelta(w http.ResponseWriter, flusher http.Flusher, text, finish string) {
	data := fmt.Sprintf(`{"id":"chatcmpl-local","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`,
		escapeJSON(text))
	WriteSSE(w, flusher, "", data)
}

func writeChatReasoning(w http.ResponseWriter, flusher http.Flusher, text string) {
	data := fmt.Sprintf(`{"id":"chatcmpl-local","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":%q},"finish_reason":null}]}`,
		escapeJSON(text))
	WriteSSE(w, flusher, "", data)
}

func writeChatToolCalls(w http.ResponseWriter, flusher http.Flusher, tcs []interface{}, finish string) {
	for i, tc := range tcs {
		m, _ := tc.(map[string]interface{})
		tcData := fmt.Sprintf(`{"id":"chatcmpl-local","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":%d,"id":%q,"type":"function","function":{"name":%q,"arguments":%q}}]},"finish_reason":null}]}`,
			i, strOrEmpty(m["id"]), strOrEmpty(m["function"].(map[string]interface{})["name"]), strOrEmpty(m["function"].(map[string]interface{})["arguments"]))
		WriteSSE(w, flusher, "", tcData)
	}
}

func writeChatDone(w http.ResponseWriter, flusher http.Flusher, obj map[string]interface{}) {
	usage, _ := obj["usage"].(map[string]interface{})
	usageJSON, _ := json.Marshal(usage)
	if usageJSON == nil {
		usageJSON = []byte("{}")
	}
	data := fmt.Sprintf(`{"id":"chatcmpl-local","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":%s}`, string(usageJSON))
	WriteSSE(w, flusher, "", data)
}

func strOrEmpty(v interface{}) string {
	if s, ok := v.(string); ok {
		return escapeJSON(s)
	}
	return ""
}

// containsTools / isStreamRequest / isStreamResponse → common.go 导出版（ContainsTools/IsStreamRequest/IsStreamResponse）
