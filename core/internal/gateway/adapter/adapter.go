// Package adapter 客户端协议适配层。
//
// 职责：把虫族网关从"单一客户端协议"解耦为"多客户端适配"。
// 每个客户端（Codex/Hermes/龙虾/Claude Code）一个适配器，
// 负责入站请求转换（TransformRequest）和出站响应转换（TransformResponse）。
//
// 架构（参考 LLM-Rosetta hub-and-spoke 简化版）：
//
//	客户端侧：Responses（Codex 原生）/ Chat（Hermes/龙虾）/ Anthropic（Claude Code）
//	后端侧：  Chat / Responses（llama-server 原生支持）
//	网关 = 两个 hub 之间的翻译机
package adapter

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Adapter 客户端协议适配器接口（入站+出站双向）。
type Adapter interface {
	// Name 适配器名（codex/chat/claude/generic）
	Name() string
	// Detect 判断请求是否属于本客户端（路径/UA/body 格式）
	Detect(r *http.Request, body []byte) bool
	// TransformRequest 入站转换：客户端格式 → 后端格式。
	// 返回转换后的 body + 后端端点路径（"/v1/chat/completions" 或 "/v1/responses"）。
	TransformRequest(r *http.Request, body []byte) (newBody []byte, backendPath string, err error)
	// TransformResponse 出站转换：后端响应 → 客户端期望格式（SSE 或 JSON）。
	TransformResponse(w http.ResponseWriter, resp *http.Response, req *http.Request, body []byte)
	// TransformError 错误转换：后端错误 → 客户端错误格式（各协议不同）。
	TransformError(w http.ResponseWriter, status int, code, msg string)
}

// Registry 适配器注册表。
var registry = struct {
	sync.RWMutex
	adapters []Adapter
}{}

// init 注册内置适配器（顺序决定 Dispatch 优先级：codex/claude 路径识别优先，chat 中间，generic 兜底）。
func init() {
	Register(&Codex{})
	Register(&Claude{})
	Register(&Chat{})
}

// Register 注册适配器（包 init 调用）。
func Register(a Adapter) {
	registry.Lock()
	defer registry.Unlock()
	registry.adapters = append(registry.adapters, a)
}

// Dispatch 按请求识别客户端，返回对应适配器。
// 识别顺序：已注册适配器按 Detect 顺序（路径/UA/body 格式），最后 generic 兜底。
func Dispatch(r *http.Request, body []byte) Adapter {
	registry.RLock()
	defer registry.RUnlock()
	for _, a := range registry.adapters {
		if a.Detect(r, body) {
			return a
		}
	}
	return &Generic{}
}

// ========== 公共工具（codex/claude 共用） ==========

// WriteSSE 写 SSE 帧 + Flush。event 可空（data-only 帧）。
func WriteSSE(w io.Writer, flusher http.Flusher, event, data string) {
	if event != "" {
		io.WriteString(w, "event: "+event+"\n")
	}
	// 多行 data 按 SSE 规范拼接（data: 前缀每行）
	lines := strings.Split(data, "\n")
	for _, ln := range lines {
		io.WriteString(w, "data: "+ln+"\n")
	}
	io.WriteString(w, "\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// StartKeepAlive SSE 心跳保活：定时发 ": ping" 注释帧（SSE 规范客户端必须忽略，
// 但连接保持活跃）。用于长推理期间防止客户端判死。
func StartKeepAlive(w io.Writer, flusher http.Flusher) chan struct{} {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				io.WriteString(w, ": ping\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}()
	return done
}

// normalizeFunctionArgs 规范化模型生成的 function_call arguments：
// 35B 模型对 JSON schema 的类型/枚举理解不精确，常把整数填成 "0.0"、
// 枚举值填成变体（如 goal status 的 "completed" 应为 "complete"）。
func normalizeFunctionArgs(args string, toolName string) string {
	if args == "" {
		return args
	}
	// 1. 整数值 float → 整数（Codex exec timeout 等 i32 参数兼容）
	re := regexp.MustCompile(`(^|[,:{[])\s*(-?\d+)\.0\s*([,}\]])`)
	out := re.ReplaceAllString(args, "${1}${2}${3}")

	// 2. goal 工具 status 枚举规范化
	if toolName == "create_goal" || toolName == "update_goal" {
		statusRe := regexp.MustCompile(`"status"\s*:\s*"(completed|done|in_progress|inprogress)"`)
		if statusRe.MatchString(out) {
			mapRe := regexp.MustCompile(`"status"\s*:\s*"completed"|"status"\s*:\s*"done"`)
			out = mapRe.ReplaceAllString(out, `"status":"complete"`)
			inRe := regexp.MustCompile(`"status"\s*:\s*"in_progress"|"status"\s*:\s*"inprogress"`)
			out = inRe.ReplaceAllString(out, `"status":"active"`)
		}
	}
	return out
}

// escapeJSON JSON 字符串转义（SSE data 内嵌）。
func escapeJSON(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// uuidHex 生成本地 UUID（无连字符，小写 hex）。
func uuidHex() string {
	return "local" + time.Now().Format("150405.000000000")
}

// itoa 整数转字符串。
func itoa(i int) string {
	return string(rune('0' + i))
}
