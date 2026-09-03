package adapter

// common.go — 跨适配器/网关公共函数集中地（D1-D9 去重）。
// 原分散在 gateway.go / forward.go / chat.go / codex.go / adapter.go 的重复函数统一收口。

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ContainsTools 判断请求体是否含 tools（工具调用兼容触发条件）。
func ContainsTools(body []byte) bool {
	return strings.Contains(string(body), "\"tools\"")
}

// ExtractToolNames 从请求体提取工具名列表（M3 控制层用——gate 检查）。
func ExtractToolNames(body []byte) []string {
	var obj struct {
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}
	var names []string
	for _, t := range obj.Tools {
		n := t.Name
		if n == "" {
			n = t.Function.Name
		}
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

// IsStreamRequest 判断请求是否流式。
func IsStreamRequest(body []byte) bool {
	var obj struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return false
	}
	return obj.Stream
}

// IsStreamResponse 判断响应是否 SSE 流。
func IsStreamResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "application/x-ndjson")
}

// CopyResponse 原样复制响应体（状态码 + 头 + 体）。
func CopyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	written, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil {
		// 治本排查（2026-08-12）：暴露透传错误——长响应截断根因
		log.Printf("[adapter:copy] ⚠️ io.Copy 失败: %v (已写 %d 字节)", copyErr, written)
	}
	resp.Body.Close()
}

// CopySSE 流式响应逐 chunk 转发（SSE 专用，含思考模型改写）。
//
// 用 ReadBytes('\n') 而非 bufio.Scanner：Scanner.Text() 会剥掉行尾 \r，
// 导致上游 CRLF 被压成 LF，破坏 SSE 事件分隔（OpenClaw 等客户端解析失败）。
// ReadBytes 保留原始行尾字节（\r\n 或 \n 原样透传）。
func CopySSE(w http.ResponseWriter, resp *http.Response) {
	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			toSend := line
			// 思考模型兼容：DS4/example-35b-v2 流式只输出 reasoning（chat: reasoning_content / responses: reasoning_text.delta）
			if bytes.Contains(line, []byte("reasoning_content")) && !bytes.Contains(line, []byte(`"content":`)) {
				if rewritten := RewriteReasoningToContent(line); rewritten != nil {
					toSend = rewritten
				}
			}
			if bytes.Contains(line, []byte("reasoning_text.delta")) {
				if rewritten := RewriteReasoningDeltaToOutput(line); rewritten != nil {
					toSend = rewritten
				}
			}
			if bytes.Contains(line, []byte(`"type":"response.output_item.added"`)) && bytes.Contains(line, []byte(`"type":"reasoning"`)) {
				toSend = RewriteReasoningItemToMessage(line)
			}
			if _, werr := w.Write(toSend); werr != nil {
				break
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
	resp.Body.Close()
}

// UUIDHex 生成 32 位 hex id（无连字符）。
func UUIDHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return fmt.Sprintf("%x", b)
}

// ExtractAuthToken 统一提取认证 token（三种认证方式）。
func ExtractAuthToken(headers http.Header) string {
	if v := headers.Get("X-Auth-Token"); v != "" {
		return v
	}
	if v := headers.Get("Authorization"); strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return strings.TrimSpace(v[len("bearer "):])
	}
	if v := headers.Get("x-api-key"); v != "" {
		return v
	}
	return ""
}

// JsonSetField 通用 JSON 字段设置：反序列化→改字段→重序列化。
// 替代 chat.go 的 modifyStream 和 gateway.go 的 replaceModelField。
func JsonSetField(body []byte, key string, value any) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}
	obj[key] = value
	newBody, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return newBody
}

// RewriteReasoningToContent 把 SSE data 行中的 reasoning_content 改名为 content。
// 思考模型（DS4/example-35b-v2）流式只输出 reasoning_content，OpenClaw 等客户端只认 content。
// 返回改写后的行；无法解析时返回 nil（保持原样）。
func RewriteReasoningToContent(line []byte) []byte {
	s := string(line)
	if !strings.Contains(s, "reasoning_content") {
		return nil
	}
	// 仅改写 data 行（SSE 数据），保留 event 行
	if !strings.HasPrefix(s, "data:") {
		return nil
	}
	s = strings.Replace(s, "reasoning_content", "content", 1)
	return []byte(s)
}

// RewriteReasoningDeltaToOutput 把 Responses 流式的 reasoning_text.delta 事件
// 转成 output_text.delta（思考模型只推 reasoning，Codex 只认 output_text）。
func RewriteReasoningDeltaToOutput(line []byte) []byte {
	s := string(line)
	if strings.Contains(s, "response.reasoning_text.delta") {
		s = strings.ReplaceAll(s, "response.reasoning_text.delta", "response.output_text.delta")
	}
	if strings.Contains(s, `"type":"response.reasoning_text.delta"`) {
		s = strings.ReplaceAll(s, `"type":"response.reasoning_text.delta"`, `"type":"response.output_text.delta"`)
	}
	return []byte(s)
}

// RewriteReasoningItemToMessage 把 Responses 流式 output_item.added 的 reasoning item
// 改造成 message item（Codex 需要 message 类型接收 output_text.delta）。
func RewriteReasoningItemToMessage(line []byte) []byte {
	s := string(line)
	if !strings.Contains(s, `"type":"reasoning"`) {
		return line
	}
	s = strings.Replace(s, `"type":"reasoning"`, `"type":"message"`, 1)
	if !strings.Contains(s, `"role":"assistant"`) {
		s = strings.Replace(s, `"summary":[]`, `"role":"assistant","summary":[]`, 1)
	}
	return []byte(s)
}
