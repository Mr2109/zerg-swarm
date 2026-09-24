package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// 会话粘性的「键」（设计 v1.7 §二 · 缺口 Q-215 的另一半）
//
// 背景（现读证据 ✓）：粘性机制**早已存在**（`pickRoute` 内的 T4 段 + `boundSession` ✓，
// 自带健康把关与能力硬门 ✓），但 `sessionID != ""` **恒假** —— 因为 **Hermes 从不发会话号**
// ⇒ 那段代码一次都没进过 ⇒ 换机 ≈29%。**缺的是键，不是机制。**
//
// 三级键（优先序 ✓）：
//
//	① 显式头 `X-Zerg-Session`（客户端/将来 Hermes 注入时用它 —— 最准）
//	② 请求体会话号（`session_id` / `session` / `llm_request_id` —— 既有行为，逐字不变 ✓）
//	③ **隐式**：`hash(model + 规范化 system 提示)` —— 同一会话的 system 稳定 ⇒ 同键 ✓
//
// 都取不到 ⇒ 返回 `""` ⇒ **不启用粘性**（逐字保持今天的行为 ✓ 不臆造会话 ✓）。
const sessionHeaderName = "X-Zerg-Session"

// implicitKeyPrefix 隐式键前缀（与人发的会话号区分开 ✓ 不撞名 ✓）。
const implicitKeyPrefix = "imp-"

// deriveSessionKey 三级键推导。`model` 由请求体内部取（避免调用处再传参）。
func deriveSessionKey(h http.Header, body []byte) string {
	// ① 显式头
	if h != nil {
		if v := strings.TrimSpace(h.Get(sessionHeaderName)); v != "" {
			return v
		}
	}
	// ② 请求体会话号（既有口径 ✓）
	if v := extractSessionID(body); v != "" {
		return v
	}
	// ③ 隐式：model + 规范化 system
	return implicitSessionKey(body)
}

// implicitSessionKey 隐式键：hash(model + "\n" + 规范化 system 提示)。
// 取不到 system（或 system 为空）⇒ ""（**不臆造** —— 无 system 的请求按今天的行为逐字不变 ✓）。
func implicitSessionKey(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	model, _ := obj["model"].(string)
	sys := extractSystemText(obj)
	if sys == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(model + "\n" + sys))
	return implicitKeyPrefix + hex.EncodeToString(sum[:])[:16]
}

// extractSystemText 取 system 提示并**规范化**（去首尾空白 + 折叠连续空白为单空格）。
// 覆盖两种形状：OpenAI `messages[0].role == "system"` 与顶层 `system` 字段。
func extractSystemText(obj map[string]interface{}) string {
	var texts []string
	if msgs, ok := obj["messages"].([]interface{}); ok {
		for _, m := range msgs {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if role, _ := mm["role"].(string); role != "system" {
				continue
			}
			texts = append(texts, contentText(mm["content"]))
		}
	}
	// Responses 形状：顶层 system（字符串或 [{type:input_text,text:…}]）
	if texts == nil {
		if s, ok := obj["system"].(string); ok && s != "" {
			texts = append(texts, s)
		} else if s := contentText(obj["system"]); s != "" {
			texts = append(texts, s)
		}
	}
	joined := strings.Join(texts, "\n")
	return normalizeWhitespace(joined)
}

// contentText 把 content 的两种形状统一成文本：字符串 / [{type,text}…]（取全部 text 拼接）。
func contentText(v interface{}) string {
	switch c := v.(type) {
	case string:
		return c
	case []interface{}:
		var sb strings.Builder
		for _, part := range c {
			if pm, ok := part.(map[string]interface{}); ok {
				if t, ok := pm["text"].(string); ok {
					sb.WriteString(t)
					sb.WriteString("\n")
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

// normalizeWhitespace 去首尾空白 + 把连续空白（含换行）折叠成单空格。
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
