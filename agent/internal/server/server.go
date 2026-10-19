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
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"errors"
	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/agent/internal/startup"
	"github.com/Mr2109/zerg-swarm/agent/internal/version"
)

// Server 子端 Agent HTTP 服务器。
type Server struct {
	agent    *Agent
	listener net.Listener
	mux      *http.ServeMux
	// events SSE 事件广播（P7 批 2：只读观测面；由 eventLoop 巡检驱动）。
	events *eventHub
	// unitStateLog 单元活性核验的**变更去重**表（2026-09-19 ②：egg_id → "state/unit_state"）。
	// /eggs 被主控心跳定期轮询 ⇒ 死卵只在**状态变化**时留一行日志（不然刷屏，等于看不见）。
	unitStateLogMu sync.Mutex
	unitStateLog   map[string]string
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
	// reqWin 请求窗口记账（缺口 GAP-20261010-16 剩余面）：`/status` 的 recent_requests
	// 与 request_source 两格取它。★ ✗ 不接管 activeReqs —— 那是「正被 worker 处理」的
	// 执行期真值，两者在**同一处**增减但分账（改它一个字都不许）。
	reqWin    *requestWindow
	startedAt time.Time
}

// inferReq 推理请求项，包含 done channel 用于结果回传。
type inferReq struct {
	body        []byte
	model       string
	stream      bool
	forwardPath string
	resultCh    chan inferResult
	ctx         context.Context // v2.5.6 治本（2026-08-28——node-a 幽灵请求）: 客户端 context——断开自动取消后端请求——释放单槽
	// source 这条请求的来源（缺口 GAP-20261010-16 剩余面）：`handleInfer` 进门时从
	// `X-Forwarded-For` / `RemoteAddr` 取一次，执行期交给 `/status` 的 request_source 格。
	source string
	// startedCh 由 worker 在**出队开始执行**时非阻塞发一次；等待方据此把"排队期"与"执行期"分开：
	// 排队期有可配上限（排队中没有任何可推进的证据可看 ⇒ 时间只能兜底），执行期**不设时长上限**、
	// 活性一律交看门狗按证据判（输出 / 引擎自报进度 / 本单元 CPU 工时）。
	startedCh chan struct{}
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
		reqWin:     newRequestWindow(), // GAP-20261010-16：窗口计数 ＋ 在飞来源（/status 两格）
		startedAt:  time.Now(),
	}
}

// NewServer 创建 HTTP 服务器。
func NewServer(agent *Agent) *Server {
	return &Server{agent: agent, events: newEventHub()}
}

// Start 启动推理队列 worker 和 HTTP 服务器。
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	// GAP-20260925-28：跨协议族端口冲突检测（**只加检测＋响亮报错**，✗ 不动绑定行为）。
	// 同一 host:port 若已被某一协议族（IPv4/IPv6）的**别的**监听者占用，本端再以
	// net.Listen("tcp", …) 去绑，内核会**静默**把本端降级到另一协议族（实测 darwin：
	// 占用方 hold IPv4 时本端 [::] 退化为 IPv6-only）⇒ 两边各占一族、互不知情，
	// 探活连上「别人」却报 reachable=true（GAP-20260925-28 的假阳性）。✗ 不自动退让。
	if msg := detectCrossFamilyPortConflict(host, port); msg != "" {
		return fmt.Errorf("监听 %s 被拒：%s", addr, msg)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}
	s.listener = listener
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/ready", s.handleReady)
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
	// 乙-2（排队可见，2026-09-17 Mr2109 拍「都做」）：把「我接到 → 我真正开始干活」的耗时回给上游。
	// 为什么放在响应头：对**流式与非流式都成立**（头在首字节前发出），上游一拿到就能算出排队时长。
	// 语义：这是**子端侧的等待**（含解析、等槽位/在途请求）；排队 ≠ 推理 ⇒ 只用于观测，不进任何时限。
	inferT0 := time.Now() // 乙-2：排队计时起点（进门即记）
	// ★ 缺口 `GAP-20261010-16`（剩余面）：窗口计数的记账点在**进门处**（认证已过、还没读体）——
	//   「最近 N 分钟请求数」的口径是**本端收到几次 `/infer`**，✗ 不是 active_requests
	//   （后者只数正被 worker 处理的那一个；排队中的请求也算「最近有人打」）。
	s.agent.reqWin.note()

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
	startedCh := make(chan struct{}, 1)

	// 放入等待队列（P7 批 3：p2Queue 取代原 20 槽 channel——限长 + ETA + 出队重校验）
	req := inferReq{
		body:        body,
		model:       model,
		stream:      stream,
		forwardPath: forwardPath,
		resultCh:    resultCh,
		startedCh:   startedCh,
		ctx:         r.Context(), // v2.5.6 治本: 客户端 context——断开取消后端请求——释放单槽
		// GAP-20261010-16：在飞来源（`/status` 的 request_source 取它，执行期才有）。
		source: requestSourceOf(r),
	}
	eta := estimateWaitETA(model)
	if !s.agent.backends.WaitQPush(model, req, eta) {
		// §5.3 / Q5：队列满 ⇒ **立即** 429（不挂起、不排队），并给出建议重试间隔。
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(eta)))
		http.Error(w, `{"error":"queue full"}`, http.StatusTooManyRequests)
		return
	}

	// 等待结果：**秒数只用于排队期**（排队中没有可观察的推进 ⇒ 时间兜底）；
	// 一旦 worker 出队开始执行（startedCh 收到信号），就不再计时，执行期活性交看门狗按证据判。
	var res inferResult
	select {
	case res = <-resultCh:
	case <-startedCh:
		// 乙-2：出队那一刻 = 排队结束 ⇒ 在此设排队时长（幂等；放"结果到手后"✗ 会把推理也算进排队）
		w.Header().Set("X-Zerg-Queued-Ms", strconv.FormatInt(time.Since(inferT0).Milliseconds(), 10))
		res = <-resultCh // 执行期：无时长上限（真卡死由看门狗判死；客户端断开由 ctx 取消）
	case <-time.After(s.inferWaitTimeout()):
		// 排队上限到达：明确告知"可稍后重试"，并让客户端知道等多久合理（不静默、不硬截断）。
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(eta)))
		http.Error(w, `{"error":"queued timeout","hint":"retry later"}`, http.StatusServiceUnavailable)
		return
	}
	{
		if res.err != nil {
			writeInferError(w, 500, "inference failed", res.err)
			return
		}
		// 乙-2：出队/开始执行之后、写响应之前，把排队时长回给上游
		// （响应头写了就冻结 ⇒ 必须在这里设 ✗ 不能放 defer）。排队 ≠ 推理，是两笔账。
		w.Header().Set("X-Zerg-Queued-Ms", strconv.FormatInt(time.Since(inferT0).Milliseconds(), 10))
		// ★ 缺口 GAP-20261019-01（可分辨面）：「谁在应答」的五格。
		//   取值一律来自 `Manager.ServedInfo` = 本端**实际驻留的那一枚**（不由请求的 model 推断、
		//   不由路由表推断）。✗ 不放正文、✗ 不改 stdout 的「正文即结果」契约。
		//   必须在 WriteHeader 之前（头写了就冻结 —— 与上一行 X-Zerg-Queued-Ms 同一课）。
		if si, ok := s.agent.backends.ServedInfo(req.model); ok {
			w.Header().Set("X-Zerg-Served-Model", si.Model)
			w.Header().Set("X-Zerg-Served-Host", si.Host)
			w.Header().Set("X-Zerg-Served-Port", strconv.Itoa(si.Port))
			w.Header().Set("X-Zerg-Served-Pid", strconv.Itoa(si.Pid))
			w.Header().Set("X-Zerg-Served-Pinned", si.PinnedHeaderValue())
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
	}
}

// inferWaitTimeout **排队**上限（ZERG_INFER_WAIT_TIMEOUT_S 可覆盖；缺省 600s）。
// 语义已于 2026-09-16 收窄：**只管排队**（等槽位/等孵化）——执行期不设时长上限，活性由看门狗按证据判。
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

// markStarted 通知等待方"该请求已出队、进入执行期"（非阻塞、幂等：startedCh 缓冲 1，等待方只读一次）。
// 产品路径由 inferLoop 在出队后调用；测试用它精确模拟"worker 已取走"这一时刻（无需真引擎）。
func markStarted(payload any) {
	req, isReq := payload.(inferReq)
	if !isReq || req.startedCh == nil {
		return
	}
	select {
	case req.startedCh <- struct{}{}:
	default:
	}
}

// inferLoop 推理队列 worker：从 p2Queue 取队头，**出队须重校验当前卵**（§7.7 修补 4）。
// （P7 批 3 起，原 20 槽 channel 队列已被 backend 等待队列取代；Mr2109 2026-09-15 拍：
// 停用的旧队列字段直接删除，需要时由 git 还原。）
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
		markStarted(payload)
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
	// ★ 缺口 `GAP-20261010-16`（剩余面）：在飞来源与 activeReqs **同处**增减（同一个 defer）⇒
	//   任一时刻「request_source 为空 ⟺ active_requests == 0」。✗ 不动上面那次计数（语义一字不改）。
	reqWinID := s.agent.reqWin.enter(req.source)
	defer func() {
		s.agent.reqWin.leave(reqWinID)
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
				status = responseStatus(result, status)
			}
			// ★ 缺口 GAP-20261019-01：自动装载被拒时**如实转达拒因**（code/message/pinned）。
			//   此前这里一律写死 `{"error":"load failed"}` ⇒ 「被钉住的默认模型挡住」与
			//   「内存不够」「名册没有这枚」在调用方看来一模一样（拒因被吞掉，等于没法分辨）。
			payload := map[string]interface{}{"error": "load failed"}
			if result != nil {
				for _, k := range []string{"code", "message", "error", "pinned", "pin_remain_s", "requester"} {
					if v, ok := result[k]; ok && v != nil && v != "" {
						payload[k] = v
					}
				}
			}
			body, _ := json.Marshal(payload)
			req.resultCh <- inferResult{status: status, body: body}
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
		// 后端忙（单槽被占 / 响应头超时 / 被引擎断开）⇒ 语义是「等会儿再试」，
		// **不是**故障：回 503 + Retry-After，别让上游把它当故障去换机
		// （真机缺陷 17 附带，2026-09-15：重叠请求时子端曾直接回 500）。
		if backend.IsBackendBusy(err) {
			payload := map[string]interface{}{
				"error":         "backend busy",
				"status":        503,
				"message":       err.Error(),
				"retry_after_s": 5,
			}
			// 活性看门狗的判词/理由/窗口/工时增量：主控据此 failover，人据此复盘
			var bbe *backend.BackendBusyError
			if errors.As(err, &bbe) {
				payload["watchdog"] = bbe.Obs
			}
			b, _ := json.Marshal(payload)
			req.resultCh <- inferResult{
				status:  503,
				headers: http.Header{"Retry-After": []string{"5"}},
				body:    b,
			}
			return
		}
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

		// 重提示机制（unsloth PR #4769）：模型"该调不调"时注入 reminder 重试一次。
		// v2.5.12 批三 H1：**默认关**（见 maybeRemind 闸①）——默认路径不改写请求体、不重跑、
		// 不把第二次的回答回给客户端。
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
//
// v2.5.12 批三（H1 修法）：本机制**默认关**，且**只在客户端确实声明了工具时**才可能触发。
// 依据 `Zerg-内部文档/项目文档/v2.5.12/排查-卵子代理回声与会话串话-v2.5.12-20260924.md` §③H1 / §④F1。
// 旧行为的代价（本单实测）：追加一句英文 reminder 进请求体并对同一请求**再跑一次推理**、
// 把**第二次**的回答回给客户端 ⇒ ① 客户端拿不到自己那一问的答案 ② 引擎侧 `prompt_tokens`
// 比干净直连多 ≈24（45 vs 21）③ 正文与 think 两个通道被同一段注入污染。
//
// 触发条件（两道闸都过 + unsloth PR #4769 原四条）：
//  0. 闸①（默认关）该模型被 `ZERG_TOOL_REMINDER` 显式点名（见 toolReminderEnabled）
//  1. 闸② 请求体里客户端**声明了工具**（无工具可调时注入只会改写请求体，见 bodyDefinesTools）
//  2. 该模型配置了需要重提示（NeedsToolReminder）
//  3. 响应 < 500 字符
//  4. 响应含前瞻意图（"我将/我用/首先/第一步/let me/i'll 等）
//  5. 响应没有工具调用（无 tool_calls / finish_reason != tool_calls）
//
// 满足则往请求体追加 reminder 用户消息，返回 true。
func (s *Server) maybeRemind(model string, body, respBody []byte) ([]byte, bool) {
	// 闸①（默认关）：未显式开启 ⇒ 不改写发给模型的请求体、不重跑、不把第二次回答回给客户端。
	if !toolReminderEnabled(model) {
		return nil, false
	}
	// 闸②：客户端没定义任何工具 ⇒ 一律不注入（没有工具可调，注入只会污染请求体）。
	if !bodyDefinesTools(body) {
		return nil, false
	}
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

// toolReminderEnabled 判断「该模型的『该调不调』重提示」是否被**显式**开启（v2.5.12 批三 · H1 修法）。
//
// 默认**关**——旧行为会改写发给模型的请求体、重跑一次、并把第二次的回答回给客户端，
// 还会让引擎侧的 `prompt_tokens` 偏离干净直连基准（实测 45 vs 21）。要恢复旧行为必须显式开：
//
//	ZERG_TOOL_REMINDER 未设 / 空串        ⇒ 全关（**默认**）
//	ZERG_TOOL_REMINDER=1 / true / all / * ⇒ 全开（逐字恢复旧行为）
//	ZERG_TOOL_REMINDER=example-35b-v2,qwen3.6     ⇒ 仅「模型名前缀」命中时开（与 modeladapter.Dispatch 同口径）
//
// 逐枚开（而不是一个全局布尔）的理由：今天有 4 枚适配器声明 NeedsToolReminder()==true
// （example-35b-v2 / qwen36 / k2horizon / qwen38flash），产线里究竟哪几枚真需要这层兜底还未按模型拆清
// （见任务清单序 35）⇒ 只允许点名开，不允许默默全开。
func toolReminderEnabled(model string) bool {
	raw := strings.TrimSpace(os.Getenv("ZERG_TOOL_REMINDER"))
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "1", "true", "all", "*":
		return true
	}
	m := strings.ToLower(model)
	for _, p := range strings.Split(lower, ",") {
		if p = strings.TrimSpace(p); p != "" && strings.HasPrefix(m, p) {
			return true
		}
	}
	return false
}

// bodyDefinesTools 判断请求体里客户端**确实**声明了工具（chat 的 tools / functions）。
//
// 一个工具都没有的场景注入 reminder 毫无意义（没有工具可调），注入只会改写请求体并让客户端
// 拿到第二次的回答（v2.5.12 批三 · F1 甲案；本单实测：无工具探针被注入 4/6 发）。
// 读不出请求体 ⇒ 保守判「没有工具」⇒ 不注入（fail-closed，宁可少兜底也不改体）。
func bodyDefinesTools(body []byte) bool {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return false
	}
	for _, key := range []string{"tools", "functions"} {
		if arr, ok := obj[key].([]interface{}); ok && len(arr) > 0 {
			return true
		}
	}
	return false
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
// handleReady 启动就绪信号（治本口径：外部判「到没到」读程序自己的话，不读秒、不读 systemd 的 activating）。
// 未就绪返回 503 + 当前阶段的 JSON（那个阶段名就是"卡在哪"的答案）。
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	phase, ready, elapsedMS, history := startup.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		// ★ 缺口 GAP-20260925-20：`/ready` 是**无令牌**的只读探活面 ⇒ 探活方（`zerg agent ping --direct`）
		//   连上后读它一次即可认定「这条链路答话的是谁」——**由回答方（本端）在响应里带自己的身份**。
		//   ✗ 不报身份，探活方就只能拿**它自己输入的地址/名册名**顶替（改前的病：回显随输入变、不随对端变，
		//   外部服务占了端口也照报 reachable）。`machine` 是**本进程自报**的机器名（`agent.machine`）。
		"machine":    s.agent.machine,
		"ready":      ready,
		"phase":      phase,
		"elapsed_ms": elapsedMS,
		"phases":     history,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	s.agent.mu.Lock()
	activeReqs := s.agent.activeReqs
	s.agent.mu.Unlock()

	// ★ 缺口 `GAP-20261010-16`（剩余面）：两格 —— 窗口内 `/infer` 计数与在飞来源。
	//   ★ 不带 `?window_s=` 时两格**也必须出现**（缺键 = 旧病复发：CLI 面又落「未知」）。
	//   `?window_s=` 给了但读不出 / ≤0 / 超上限 ⇒ 400 明报拒收（✗ 静默回落默认）。
	reqWin, winErr := requestWindowParam(r)
	if winErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "bad window_s",
			"message": winErr.Error(),
		})
		return
	}
	recentReqs, sources := s.agent.reqWin.snapshot(reqWin)
	requestSource := strings.Join(sources, ",")
	if requestSource == "" {
		// 无在飞 ⇒ **明写「无」**（✗ 空串：CLI 侧会把空串读成「子端没报」= 未知）。
		requestSource = requestSourceNone
	}

	// ★ 缺口 `GAP-20261018-20`（身份自报半）：子端在 `/status` 里报**自己这一件**的身份。
	//   换件正门 `zerg agent deploy` 靠它核对「盘上换上的就是正在跑的那件」（`code_digest` =
	//   自身可执行件 sha256 前 16 位）与「跑的是哪笔提交」（`code_sha`）。
	//   旧 `/status` 一格身份都没有 ⇒ 换完只能等下一次心跳、再由主控面间接看，运维面判不了。
	//   ★ 三格一律**如实**（取不到即空串/unknown，✗ 不编、✗ 不拿别格顶上）。
	self := version.ReadSelf()

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
		// 两格（GAP-20261010-16 剩余面）：**真值**，✗ 再也不是「未知」。
		//   recent_requests = 窗口内收到的 `/infer` 次数；request_source = 在飞来源（无 ⇒ 「无」）。
		"recent_requests": recentReqs,
		"request_source":  requestSource,
		"healthy":         s.agent.backends.IsHealthy(),
		"backend_state":   s.agent.backends.State(),
		// B4 v2：CPU/GPU 使用率（主控监看展示）
		"cpu_pct": s.agent.sampler.CpuPct(),
		"gpu_pct": s.agent.sampler.GpuPct(),
		// 身份四格（GAP-20261018-20）：code_sha / code_version 与心跳同源（`version.ReadSelf`），
		// code_digest 是**内容身份**（换件核对用它，心跳那一路主控今天还不收）。
		"code_version":    self.Version,
		"code_sha":        self.Commit,
		"code_build_time": self.BuildTime,
		"code_digest":     self.Digest,
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

	// 单卵时限（GAP-20260926-201 全通路）：CLI `--timeout` ⇒ /load 体 `egg_max_s` ⇒ 落成该模型
	// 的逐枚覆盖，逐 `InferForward` 生效（优先级 本值 > env `ZERG_EGG_MAX_S` > 现默认 0）。
	// 给了但读不出 ⇒ **明报拒收**（400 · ✗ 不静默回落默认）—— 与「没给」是两件事。
	if raw, ok := reqMap["egg_max_s"]; ok {
		var egs string
		_ = json.Unmarshal(raw, &egs)
		d, perr := backend.ParseEggMaxS(egs)
		if perr != nil {
			http.Error(w, `{"error":"bad egg_max_s: `+perr.Error()+`"}`, http.StatusBadRequest)
			return
		}
		s.agent.backends.SetEggMaxOverride(model, d)
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
		status = responseStatus(result, 500)
	}

	writeJSON(w, status, result)
}

// responseStatus 从 backend 的失败响应体里取它自称的 status 码（取不到 / 认不得 ⇒ fallback）。
//
// ⚠ 这里曾是一条**真机缺陷**（2026-09-15 第一枚卵实测 · 报告 §12 缺陷 7）：backend.errResponse
// 用 **int** 装 status（`map[string]interface{}{"status": 507}`），而本文件原先只断言
// `.(float64)`（那是 JSON 反序列化后的形态，不是内进程 map 的形态）⇒ 断言恒失败 ⇒
// **HTTP 状态码恒为 500**，而 body 里明明写着 `"status":507`/`502`（拒孵闸门码与 HTTP 行自相矛盾）。
//
// 兼容 int / float64 / int64 / 数字字符串四种形态：**本函数是唯一的映射点**，
// 任何调用方都不许再自己 `.(float64)` 断言。
func responseStatus(result map[string]interface{}, fallback int) int {
	if result == nil {
		return fallback
	}
	switch v := result["status"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

// handleReload 热加载 agent_models.yaml → 更新注册表 → 返回 {status: ok, models: N}
//
// ①（2026-09-19）：**顺带刷新卵实测档案缓存**。原来的 reload 只重载注册表，档案不重载 ⇒
// 盘上档案重标定后（`peak_gtt_gb` 等已更新）子端仍可能按旧副本放行/展示。顺序写死：
// **先重载注册表**（卵名映射是最新的）**再刷档案**（按最新的卵名集合逐枚现读）。
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
	// 卵档案缓存刷新（尽力而为 + 留痕：某一枚档案坏了不让整条 reload 失败，但问题必须回给调用方）
	profiles, problems := 0, []string(nil)
	if s.agent.backends != nil {
		profiles, problems = s.agent.backends.RefreshEggProfiles()
	}
	if len(problems) > 0 {
		log.Printf("[server] ⚠ 卵实测档案刷新：%d 枚成功，%d 枚有问题: %v", profiles, len(problems), problems)
	} else {
		log.Printf("[server] 卵实测档案刷新完成: %d 枚（缓存已按盘上档案重建）", profiles)
	}
	resp := map[string]interface{}{"status": "ok", "models": len(models), "egg_profiles": profiles}
	if len(problems) > 0 {
		// 如实呈现（不许把「有档案读不出来」说成刷新完成）——调用方据此决定要不要人工介入。
		resp["egg_profile_errors"] = problems
	}
	writeJSON(w, 200, resp)
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

// ── GAP-20260925-28：端口跨协议族冲突检测（只加检测＋响亮报错，✗ 不动绑定行为）────────
//
// 症状：同一 host:port 上，若已有**别的**监听者只占住某一个协议族（IPv4 或 IPv6），
// 本端再以 net.Listen("tcp", addr) 去绑，内核会**静默**把本端降级到另一个协议族
// （实测 darwin：外部进程 hold IPv4 时，本端 [::] 退化成 IPv6-only）⇒ 两边各占一族、
// 互不知情；`zerg agent ping --direct` 的探活连上的是「别人」却仍报 reachable=true（假阳性）。
//
// 本函数把这种「静默抢占」变成「响亮报错」：启动前分别探两个协议族，**恰有一个已被占**
// ⇒ 本端若继续绑只会与该进程各占一族 ⇒ 返回非空描述（含被占协议族 + 可得的占位进程）。
// 两族都空（本端可独占双栈）或两族都被占（本端根本起不来 —— 交 net.Listen 的既有错误路）
// ⇒ 返回 ""。✗ 不改绑定方式、✗ 不自动退让。
func detectCrossFamilyPortConflict(host string, port int) string {
	occupied := []string{}
	for _, pr := range familyProbeAddrs(host, port) {
		if familyOccupied(pr[0], pr[1]) {
			occupied = append(occupied, pr[0])
		}
	}
	if len(occupied) != 1 {
		// 0 ⇒ 本端可独占双栈，无冲突；≥2 ⇒ 端口整体被占，交 net.Listen 的既有错误路。
		return ""
	}
	owner := portListenersBrief(port)
	return fmt.Sprintf("端口 %d 的 %s 已被**别的**监听者占用%s —— 本端若继续绑定只会与它各占一个协议族"+
		"（探活会连上对方却报 reachable=true）⇒ 拒绝静默启动（✗ 不自动退让；先处理占位进程，"+
		"或用 `zerg port ls %d` 现读属主）", port, occupied[0], owner, port)
}

// familyProbeAddrs 把监听口径 (host, port) 归一成要探的「协议族 → 地址」对。
// 通配（空 / 0.0.0.0 / ::）⇒ 两族各探一个通配地址；具体地址 ⇒ 只探它自己那一族
// （具体地址上的绑定本就只落一族，不存在跨族抢占面）。
func familyProbeAddrs(host string, port int) [][2]string {
	h := strings.TrimSpace(strings.Trim(host, "[]"))
	switch h {
	case "", "0.0.0.0", "::":
		return [][2]string{
			{"tcp4", fmt.Sprintf("0.0.0.0:%d", port)},
			{"tcp6", fmt.Sprintf("[::]:%d", port)},
		}
	}
	if strings.Contains(h, ":") {
		return [][2]string{{"tcp6", fmt.Sprintf("[%s]:%d", h, port)}}
	}
	return [][2]string{{"tcp4", fmt.Sprintf("%s:%d", h, port)}}
}

// familyOccupied 探「该地址在该协议族上是否已被监听者占用」。
// 只读：绑一次、立即关；EADDRINUSE ⇒ 已占；别的错误（如本机没开 IPv6）⇒ 判「未占」，
// 不把它误报成冲突（fail-open 于**误报**方向 —— 宁可不报，也不制造假冲突）。
func familyOccupied(network, addr string) bool {
	ln, err := net.Listen(network, addr)
	if err != nil {
		return errors.Is(err, syscall.EADDRINUSE)
	}
	ln.Close()
	return false
}

// portListenersBrief 现读某端口上的监听者（best-effort · lsof；取不到 ⇒ 明写「未自报」，
// ✗ 不拿输入名顶替 —— 照 family_port.go 的现成口径）。
func portListenersBrief(port int) string {
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		return "（占位进程未自报：本机无 lsof）"
	}
	out, err := exec.Command(lsof, "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN").Output()
	if err != nil {
		return "（占位进程未自报：lsof 读不到）"
	}
	names := []string{}
	for i, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue // 表头 / 空行
		}
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		names = append(names, fmt.Sprintf("pid=%s(%s)", f[1], f[0]))
	}
	if len(names) == 0 {
		return "（占位进程未自报：lsof 无匹配）"
	}
	return "（占位进程 " + strings.Join(names, " · ") + "）"
}
