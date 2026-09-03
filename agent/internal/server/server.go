// Package server 提供子端 Agent 的 HTTP API 服务器。
//
// 职责：
//   - 监听 :8100（可配 --host）
//   - POST /status → 系统状态快照
//   - POST /load → 按需加载模型
//   - POST /infer → 转发推理请求到后端（支持透传 stream）
//   - POST /unload → 停止当前后端
//   - 认证：所有端点检查 X-Auth-Token
//   - 推理请求队列（本地排队，串行转发）
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"zerg/agent/internal/backend"
	"zerg/agent/internal/modeladapter"
	"zerg/agent/internal/monitor"
	"zerg/agent/internal/registry"
)

// Server 子端 Agent HTTP 服务器。
type Server struct {
	agent    *Agent
	listener net.Listener
	mux      *http.ServeMux
}

// Agent 应用核心，持有所有状态。
type Agent struct {
	machine    string
	token      string
	registry   *registry.Registry
	backends   *backend.Manager
	sampler    *monitor.Sampler
	controller string

	mu       sync.Mutex
	activeReqs int
	inferCh  chan inferReq
	startedAt time.Time
}

// inferReq 推理请求项，包含 done channel 用于结果回传。
type inferReq struct {
	body        []byte
	model       string
	stream      bool
	forwardPath string
	resultCh    chan inferResult
	ctx         context.Context // v2.5.6 治本（2026-08-28——x3 幽灵请求）: 客户端 context——断开自动取消后端请求——释放单槽
}

// inferResult 推理结果。
type inferResult struct {
	status  int
	headers http.Header
	body    []byte
	err     error
}

// NewAgent 创建应用核心。
func NewAgent(machine, token string, reg *registry.Registry, backends *backend.Manager, ctrl string) *Agent {
	return &Agent{
		machine:    machine,
		token:      token,
		registry:   reg,
		backends:   backends,
		sampler:    monitor.DefaultSampler,
		controller: ctrl,
		startedAt:  time.Now(),
		inferCh:    make(chan inferReq, 20),
	}
}

// NewServer 创建 HTTP 服务器。
func NewServer(agent *Agent) *Server {
	return &Server{agent: agent}
}

// Start 启动推理队列 worker 和 HTTP 服务器。
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}
	s.listener = listener
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/load", s.handleLoad)
	s.mux.HandleFunc("/infer", s.handleInfer)
	s.mux.HandleFunc("/unload", s.handleUnload)
	s.mux.HandleFunc("/infer/reload", s.handleReload)

	// 启动推理队列 worker
	go s.inferLoop()

	log.Printf("[server] Agent 启动: machine=%s, listener=%s", s.agent.machine, addr)
	if err := http.Serve(listener, s.mux); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP 服务错误: %w", err)
	}
	return nil
}

// Stop 停止服务器。
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	log.Printf("[server] Agent 停止: machine=%s", s.agent.machine)
}

// handleInfer 处理 /infer 请求。
func (s *Server) handleInfer(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var reqMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &reqMap); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// 提取模型名
	modelRaw, ok := reqMap["model"]
	if !ok {
		http.Error(w, `{"error":"missing model"}`, http.StatusBadRequest)
		return
	}
	var model string
	json.Unmarshal(modelRaw, &model)
	if model == "" {
		http.Error(w, `{"error":"missing model"}`, http.StatusBadRequest)
		return
	}

	// 提取 _path 决定转发端点
	forwardPath := "/v1/chat/completions"
	if pathRaw, ok := reqMap["_path"]; ok {
		json.Unmarshal(pathRaw, &forwardPath)
	}
	if forwardPath == "" {
		forwardPath = "/v1/chat/completions"
	}

	// 提取 stream
	var stream bool
	if streamRaw, ok := reqMap["stream"]; ok {
		json.Unmarshal(streamRaw, &stream)
	}

	// 创建结果 channel（带超时）
	resultCh := make(chan inferResult, 1)
	timeout := time.After(600 * time.Second) // 10 分钟超时

	// 放入推理队列
	req := inferReq{
		body:        body,
		model:       model,
		stream:      stream,
		forwardPath: forwardPath,
		resultCh:    resultCh,
		ctx:         r.Context(), // v2.5.6 治本: 客户端 context——断开取消后端请求——释放单槽
	}
	select {
	case s.agent.inferCh <- req:
		// 等待结果
		select {
		case res := <-resultCh:
			if res.err != nil {
				writeInferError(w, 500, "inference failed", res.err)
				return
			}
			// 写响应头
			for k, vals := range res.headers {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			// 治本（2026-08-12）：显式 Content-Length——避免 Go 自动 chunked 传输长响应
			// 中断（网关读 IncompleteRead——chunked 无长度歧义，显式长度让客户端明确读完）
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(res.body)))
			// 排查日志：记录响应体实际长度（网关读到 3902 截断——确认 agent 发了多少）
			log.Printf("[server] infer 响应: status=%d body_len=%d", res.status, len(res.body))
			w.WriteHeader(res.status)
			w.Write(res.body)
			// 治本（2026-08-12）：显式 Flush——确保完整写出（网关读截断 3902/4082——写缓冲未完整发出）
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-timeout:
			http.Error(w, `{"error":"inference timeout"}`, http.StatusGatewayTimeout)
		}
	default:
		http.Error(w, `{"error":"queue full"}`, http.StatusTooManyRequests)
	}
}

// inferLoop 推理队列 worker，串行处理推理请求。
func (s *Server) inferLoop() {
	for req := range s.agent.inferCh {
		s.handleInferRequest(req)
	}
}

// handleInferRequest 处理单个推理请求。
func (s *Server) handleInferRequest(req inferReq) {
	s.agent.mu.Lock()
	s.agent.activeReqs++
	s.agent.mu.Unlock()
	defer func() {
		s.agent.mu.Lock()
		s.agent.activeReqs--
		s.agent.mu.Unlock()
	}()

	defer func() {
		// 确保结果 always 回传，避免调用方死等
		select {
		case req.resultCh <- inferResult{status: 500, body: []byte(`{"error":"result not sent"}`)}:
		default:
		}
	}()

	// 多模型驻留：请求模型未驻留/不健康时自动加载（Start 幂等：已驻留直接复用）
	if !s.agent.backends.IsModelHealthy(req.model) {
		log.Printf("[server] 模型 %s 未驻留或健康失败，自动加载", req.model)
		result, loadErr := s.agent.backends.Start(req.model)
		if loadErr != nil || result == nil || result["ok"] == false {
			status := 500
			if result != nil {
				if st, ok := result["status"].(float64); ok {
					status = int(st)
				}
			}
			req.resultCh <- inferResult{
				status: status,
				body:   []byte(fmt.Sprintf(`{"error":"%s"}`, "load failed")),
			}
			return
		}
	}

	if !s.agent.backends.IsModelHealthy(req.model) {
		req.resultCh <- inferResult{
			status: 503,
			body:   []byte(`{"error":"backend not ready"}`),
		}
		return
	}

	// 转发到后端（按请求模型选进程）——v2.5.6 带客户端 context（断开取消——释放单槽）
	resp, err := s.agent.backends.InferForward(req.ctx, req.model, req.forwardPath, req.body)
	if err != nil {
		req.resultCh <- inferResult{
			status: 502,
			err:    err,
		}
		return
	}
	defer resp.Body.Close()

	if !req.stream {
		// 非流式：读取响应并原样返回
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			// 治本排查（2026-08-12）：暴露真实读取错误——长响应断根因定位
			log.Printf("[server] ⚠️ 读后端响应失败 (model=%s): %v (已读 %d 字节)", req.model, readErr, len(respBody))
		}

		// 重提示机制（unsloth PR #4769）：模型"该调不调"时注入 reminder 重试一次
		if remindedBody, should := s.maybeRemind(req.model, req.body, respBody); should {
			log.Printf("[server] ⚠️ 检测到模型未调工具（短响应+意图），注入重提示重试")
			resp2, err2 := s.agent.backends.InferForward(req.ctx, req.model, req.forwardPath, remindedBody)
			if err2 == nil {
				body2, _ := io.ReadAll(resp2.Body)
				resp2.Body.Close()
				respBody = body2
				// 用重试的响应状态（若后端返回了有效响应）
				if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
					req.resultCh <- inferResult{
						status:  resp2.StatusCode,
						headers: resp2.Header,
						body:    respBody,
					}
					return
				}
			}
		}

		req.resultCh <- inferResult{
			status:  resp.StatusCode,
			headers: resp.Header,
			body:    respBody,
		}
	} else {
		// 流式：透传 SSE
		respBody, _ := io.ReadAll(resp.Body)
		req.resultCh <- inferResult{
			status:  resp.StatusCode,
			headers: http.Header{"Content-Type": []string{"text/event-stream"}, "Transfer-Encoding": []string{"chunked"}},
			body:    respBody,
		}
	}
}

// maybeRemind 检测模型"该调不调"并返回注入 reminder 的请求体。
// 触发条件（unsloth PR #4769）：
//  1. 该模型配置了需要重提示（NeedsToolReminder）
//  2. 响应 < 500 字符
//  3. 响应含前瞻意图（"我将/我用/首先/第一步/let me/i'll 等）
//  4. 响应没有工具调用（无 tool_calls / finish_reason != tool_calls）
// 满足则往请求体追加 reminder 用户消息，返回 true。
func (s *Server) maybeRemind(model string, body, respBody []byte) ([]byte, bool) {
	adp := modeladapter.Dispatch(model)
	if !adp.NeedsToolReminder() {
		return nil, false
	}
	prompt := adp.ReminderPrompt()
	if prompt == "" {
		return nil, false
	}
	// 条件 4：已调工具则不提醒（只认 finish_reason=tool_calls；message 里 tool_calls:null 不算）
	bodyStr := string(respBody)
	if strings.Contains(bodyStr, `"finish_reason":"tool_calls"`) || strings.Contains(bodyStr, `"finish_reason": "tool_calls"`) {
		return nil, false
	}
	// 条件 2：响应体不过大（responses 格式 JSON 含 output 数组/reasoning，阈值放宽到 2000）
	if len(respBody) > 2000 {
		return nil, false
	}
	// 条件 3：模型"给方案不执行"信号——含代码块或命令特征（bash/curl/echo/python 等）
	// 或含 planning 意图（unsloth 原版：let me/i'll/我将 等）
	intents := []string{"我将", "我用", "首先", "第一步", "让我", "let me", "i'll", "i will", "first,", "step 1", "我来"}
	cmdSignals := []string{"```bash", "```shell", "```sh", "```python", "curl ", "wget ", "echo ", "python ", "cd /tmp", "```json"}
	hasIntent := false
	lower := strings.ToLower(bodyStr)
	for _, it := range intents {
		if strings.Contains(lower, strings.ToLower(it)) {
			hasIntent = true
			break
		}
	}
	// 代码块/命令信号（不区分大小写，命令本身小写化后匹配）
	for _, cs := range cmdSignals {
		if strings.Contains(lower, strings.ToLower(cs)) {
			hasIntent = true
			break
		}
	}
	if !hasIntent {
		return nil, false
	}

	// 注入 reminder：往请求体 messages（chat）或 input（responses）追加 user 消息
	reminded, err := appendReminder(body, prompt)
	if err != nil {
		return nil, false
	}
	return reminded, true
}

// appendReminder 往请求体追加 reminder 消息（chat 的 messages / responses 的 input）。
func appendReminder(body []byte, reminder string) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	// chat 格式：messages 追加 user
	if msgs, ok := obj["messages"].([]interface{}); ok {
		msgs = append(msgs, map[string]interface{}{
			"role":    "user",
			"content": reminder,
		})
		obj["messages"] = msgs
	} else if input, ok := obj["input"].([]interface{}); ok {
		// responses 格式：input 追加 user message
		input = append(input, map[string]interface{}{
			"type":    "message",
			"role":    "user",
			"content": []interface{}{map[string]interface{}{"type": "input_text", "text": reminder}},
		})
		obj["input"] = input
	}
	return json.Marshal(obj)
}

// handleStatus 处理 /status 请求。
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	s.agent.mu.Lock()
	activeReqs := s.agent.activeReqs
	s.agent.mu.Unlock()

	// 构建快照
	snap := map[string]interface{}{
		"machine":          s.agent.machine,
		"model":            s.agent.backends.CurrentModel(),
		"backend":          s.agent.backends.CurrentBackend(),
		"port":             s.agent.backends.CurrentPort(),
		"mem_available_gb": s.agent.sampler.MemAvailableGb(),
		"models":           s.agent.registry.Names(),
		"uptime":           time.Since(s.agent.startedAt).Seconds(),
		"active_requests":  activeReqs,
		"healthy":          s.agent.backends.IsHealthy(),
		"backend_state":    s.agent.backends.State(),
		// B4 v2：CPU/GPU 使用率（主控监看展示）
		"cpu_pct": s.agent.sampler.CpuPct(),
		"gpu_pct": s.agent.sampler.GpuPct(),
	}

	writeJSON(w, http.StatusOK, snap)
}

// handleLoad 处理 /load 请求。
func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var reqMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &reqMap); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	modelRaw, ok := reqMap["model"]
	if !ok {
		http.Error(w, `{"error":"missing model"}`, http.StatusBadRequest)
		return
	}
	var model string
	json.Unmarshal(modelRaw, &model)
	if model == "" {
		http.Error(w, `{"error":"missing model"}`, http.StatusBadRequest)
		return
	}

	// 加载模型
	result, loadErr := s.agent.backends.Start(model)
	if loadErr != nil {
		log.Printf("[server] 加载模型失败: %v", loadErr)
		writeJSON(w, 500, map[string]interface{}{
			"ok":      false,
			"error":   "load failed",
			"message": loadErr.Error(),
		})
		return
	}

	status := 200
	if result["ok"] == false {
		if st, ok := result["status"].(float64); ok {
			status = int(st)
		} else {
			status = 500
		}
	}

	writeJSON(w, status, result)
}

// handleReload 热加载 agent_models.yaml → 更新注册表 → 返回 {status: ok, models: N}
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	// 重新加载注册表（线程安全）
	if err := s.agent.registry.Reload(); err != nil {
		log.Printf("[server] 注册表重载失败: %v", err)
		writeJSON(w, 500, map[string]interface{}{"status": "error", "message": fmt.Sprintf("reload failed: %v", err)})
		return
	}
	models := s.agent.registry.Names()
	log.Printf("[server] 注册表重载完成: %d 个模型", len(models))
	writeJSON(w, 200, map[string]interface{}{"status": "ok", "models": len(models)})
}

// handleUnload 处理 /unload 请求。
func (s *Server) handleUnload(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	result := s.agent.backends.Stop()
	writeJSON(w, 200, result)
}

// checkAuth 检查认证令牌。
func (s *Server) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	token := r.Header.Get("X-Auth-Token")
	if token != s.agent.token {
		log.Printf("[server] 认证失败: token mismatch")
		writeJSON(w, 401, map[string]interface{}{
			"error": "unauthorized",
		})
		return false
	}
	return true
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeInferError 写推理错误响应。
func writeInferError(w http.ResponseWriter, status int, errMsg string, loadErr error) {
	resp := map[string]interface{}{
		"error":   errMsg,
		"status":  status,
	}
	if loadErr != nil {
		resp["message"] = loadErr.Error()
	}
	writeJSON(w, status, resp)
}
