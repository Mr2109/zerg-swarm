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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// Server 子端 Agent HTTP 服务器。
type Server struct {
	agent    *Agent
	listener net.Listener
	mux      *http.ServeMux
	// events SSE 事件广播（P7 批 2：只读观测面；由 eventLoop 巡检驱动）。
	events *eventHub
}

// Agent 应用核心，持有所有状态。
type Agent struct {
	machine    string
	token      string
	registry   *registry.Registry
	backends   *backend.Manager
	sampler    *monitor.Sampler
	vitals     *monitor.VitalsRecorder
	controller string

	mu         sync.Mutex
	activeReqs int
	// inferCh 【已停用｜P7 批 3】原 20 槽推理 channel，已被 backend 等待队列（p2Queue）取代
	// （队列满立即 429 + Retry-After、排队含 ETA、出队重校验当前卵）。
	// 保留字段仅为兼容：**不再有任何读写**。是否删除待 Mr2109 点头（删代码需先问）。
	inferCh   chan inferReq
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
		vitals:     monitor.NewVitalsRecorder(), // P4：/services 的全局 GTT 账来源（只读快照）
		controller: ctrl,
		startedAt:  time.Now(),
		inferCh:    make(chan inferReq, 20),
	}
}

// NewServer 创建 HTTP 服务器。
func NewServer(agent *Agent) *Server {
	return &Server{agent: agent, events: newEventHub()}
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
	s.mux.HandleFunc("/pin", s.handlePin)
	s.mux.HandleFunc("/unpin", s.handleUnpin)
	s.mux.HandleFunc("/infer/reload", s.handleReload)
	// P7：只读可观测面（基线与借用租约）——设计 §11 M9
	s.mux.HandleFunc("/services", s.handleServices)
	// P7 批 1：卵清单只读端点（设计 §5.4 端点名定案 (a)）
	s.mux.HandleFunc("/eggs", s.handleEggs)
	// P7 批 2：SSE 只读事件流（状态迁移 + 排队深度 + ETA）
	s.mux.HandleFunc("/events", s.handleEvents)

	// 启动推理队列 worker
	go s.inferLoop()
	// P7 批 2：事件巡检（1s 低频、只读采样——不在热路径上）
	go s.eventLoop(time.Second)

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

	// 放入等待队列（P7 批 3：p2Queue 取代原 20 槽 channel——限长 + ETA + 出队重校验）
	req := inferReq{
		body:        body,
		model:       model,
		stream:      stream,
		forwardPath: forwardPath,
		resultCh:    resultCh,
		ctx:         r.Context(), // v2.5.6 治本: 客户端 context——断开取消后端请求——释放单槽
	}
	eta := estimateWaitETA(model)
	if !s.agent.backends.WaitQPush(model, req, eta) {
		// §5.3 / Q5：队列满 ⇒ **立即** 429（不挂起、不排队），并给出建议重试间隔。
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(eta)))
		http.Error(w, `{"error":"queue full"}`, http.StatusTooManyRequests)
		return
	}

	// 等待结果；排队+执行的总时长超过上限 ⇒ 503 + Retry-After（可等的失败，不是服务故障）。
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
	case <-time.After(s.inferWaitTimeout()):
		// 排队上限到达：明确告知"可稍后重试"，并让客户端知道等多久合理（不静默、不硬截断）。
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(eta)))
		http.Error(w, `{"error":"queued timeout","hint":"retry later"}`, http.StatusServiceUnavailable)
	}
}

// inferWaitTimeout 排队+执行的总等待上限（ZERG_INFER_WAIT_TIMEOUT_S 可覆盖；缺省 600s）。
func (s *Server) inferWaitTimeout() time.Duration {
	if v := os.Getenv("ZERG_INFER_WAIT_TIMEOUT_S"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 600 * time.Second
}

// estimateWaitETA 排队预计等待：有实测档案（装载耗时）就用它，否则 0（未知，不编造）。
// 依据 §8.4 标定铁律——ETA 只能来自实测档案，不许猜。
func estimateWaitETA(model string) time.Duration {
	p, err := monitor.LoadEggProfile(monitor.EggProfilePath(model))
	if err != nil || p.LoadSeconds <= 0 {
		return 0
	}
	return time.Duration(p.LoadSeconds * float64(time.Second))
}

// retryAfterSeconds Retry-After 建议值：ETA 已知用 ETA，未知给一个保守缺省（秒）。
func retryAfterSeconds(eta time.Duration) int {
	if eta > 0 {
		secs := int(eta.Seconds())
		if secs < 1 {
			return 1
		}
		return secs
	}
	return 30 // 未知 ETA 的保守建议（Q5：让客户端"读得懂还要等多久"）
}

// inferLoop 推理队列 worker：从 p2Queue 取队头，**出队须重校验当前卵**（§7.7 修补 4）。
// 原先的 20 槽 channel（agent.inferCh）已停用——保留字段仅为兼容，见 Agent 结构注释。
func (s *Server) inferLoop() {
	mismatchStreak := 0
	for {
		item, payload, ok, mismatch := s.agent.backends.WaitQPop(s.agent.backends.CurrentModel, true)
		if mismatch {
			// 卵已换（或还没孵）：**不得直接转发**，先重走孵化流程（Start 幂等）。
			if mismatchStreak == 0 {
				if item.Model != "" {
					if _, err := s.agent.backends.Start(item.Model); err != nil {
						log.Printf("[server] 出队重校验：重走孵化 %s 失败: %v", item.Model, err)
					}
				}
				mismatchStreak++
				continue
			}
			// 第二次仍不匹配 ⇒ 孵化没能把它变成当前卵：摘除队头并把失败回给该请求，
			// 避免热旋（不放回队列——放回会饿死后面的项，也不断重试这个装不起来的模型）。
			s.agent.backends.WaitQDropHead()
			if req, isReq := payload.(inferReq); isReq {
				req.resultCh <- inferResult{status: 503, body: []byte(`{"error":"model not available"}`)}
			}
			mismatchStreak = 0
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if !ok {
			continue
		}
		mismatchStreak = 0
		if req, isReq := payload.(inferReq); isReq {
			s.handleInferRequest(req)
		}
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
//
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

// ActiveRequests 返回当前在飞（正被推理 worker 处理）的请求数。
// 这是 server.go 里的 activeReqs 真值（handleInferRequest 里增减）——
// 心跳的 active_requests 字段取它，取代原先写死的 0。
func (a *Agent) ActiveRequests() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeReqs
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
//
// 两种用法（批 3 起）：
//   - 带 body {"models":["a","b"]} → **定向**卸载这几项（"只卸够"的主控让位用它）；
//     在飞请求中、pin 未到期的项会被跳过并在响应里给出原因（红线：绝不杀活跃推理）。
//   - 不带 body / 空清单 → 卸全部（沿用既有语义，运维显式动作那条路不变）。
func (s *Server) handleUnload(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	var req struct {
		Models []string `json:"models"`
	}
	if r.Body != nil {
		// 解码失败（含空体/老客户端不带体）一律按"全卸"处理——不因格式问题改变旧语义。
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	result := s.agent.backends.Unload(req.Models)
	writeJSON(w, 200, result)
}

// handlePin 处理 /pin 请求（批 4：主控观测面把"在 TTL 内不被自动驱逐"的锁定意图落到本端）。
//
// 铁律（逐条不得绕过）：
//   - 只锁定**已在本端驻留**的模型（backend.Manager.Pin 只认自己 procs 里的名字）；
//     非驻留项一律拒绝——**绝不**因此启动/接管任何进程（§八 Q6）。
//   - ttl_s<=0 一律拒绝：无 TTL 的 pin 等同内存泄漏（§八 Q5）。
func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	var req struct {
		Model string `json:"model"`
		TTLS  int    `json:"ttl_s"`
	}
	if r.Body == nil {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "missing_body"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "invalid_json"})
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "missing_model"})
		return
	}
	if req.TTLS <= 0 {
		writeJSON(w, 400, map[string]interface{}{
			"ok":      false,
			"error":   "pin_ttl_required",
			"message": "pin must carry a positive ttl_s (no-TTL pin equals a memory leak)",
		})
		return
	}
	// Manager.Pin 只认已驻留的模型；非驻留 → 返回错误（绝不启动/接管）
	if err := s.agent.backends.Pin(model, time.Duration(req.TTLS)*time.Second); err != nil {
		writeJSON(w, 409, map[string]interface{}{"ok": false, "error": "not_resident", "message": err.Error()})
		return
	}
	remain, _ := s.agent.backends.PinRemainS(model)
	writeJSON(w, 200, map[string]interface{}{
		"ok":           true,
		"model":        model,
		"pin_remain_s": remain,
	})
}

// handleUnpin 处理 /unpin 请求（立刻恢复可驱逐）。与 /pin 同理：只认已驻留的托管项。
func (s *Server) handleUnpin(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	var req struct {
		Model string `json:"model"`
	}
	if r.Body == nil {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "missing_body"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "invalid_json"})
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		writeJSON(w, 400, map[string]interface{}{"ok": false, "error": "missing_model"})
		return
	}
	if err := s.agent.backends.Unpin(model); err != nil {
		writeJSON(w, 409, map[string]interface{}{"ok": false, "error": "not_resident", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"ok": true, "model": model, "pinned": false})
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
		"error":  errMsg,
		"status": status,
	}
	if loadErr != nil {
		resp["message"] = loadErr.Error()
	}
	writeJSON(w, status, resp)
}
