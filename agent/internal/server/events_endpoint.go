// events_endpoint.go —— P7 批 2：SSE 只读事件流（GET /events）。
//
// 设计真源：设计-子端沙箱化-20260914.md §5.4（观测面：状态迁移 + 排队深度 + ETA）·
// §13 Q5（排队必须含 ETA，客户端要"读得懂还要等多久"）· §11 P4（只读，红线②）。
//
// 鉴权（默认口径，待 Mr2109 拍）：与既有端点同门——`X-Auth-Token` 头；
// **另支持 `?token=` 查询参数**，因为浏览器原生 `EventSource` 不能自定义请求头。
// ⚠ 安全权衡：查询参数会进访问日志/代理日志（头不会）。取舍与替代方案见回报，未拍前按此实现。
//
// 事件形状（每行一个 SSE data 帧，JSON）：
//
//	{"seq":12,"at":"…RFC3339…","state":"ready","model":"qwen","state_changed":true,
//	 "queue_len":3,"head_eta_s":42.5,"inflight":1}
//
// 如实原则：ETA 未知 ⇒ 该字段为 0（不编造）；GTT/卵细节走 /eggs，不在这里重复搬运。
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// eventHub 事件广播：只读、非阻塞——慢订阅者丢帧而不是拖住发布方。
type eventHub struct {
	mu   sync.Mutex
	next int
	subs map[int]chan []byte
}

func newEventHub() *eventHub {
	return &eventHub{subs: make(map[int]chan []byte)}
}

// subscribe 注册一个订阅者（带缓冲，容量满则丢帧——SSE 只读面不承担可靠性语义）。
func (h *eventHub) subscribe() (int, chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	ch := make(chan []byte, 8)
	h.subs[id] = ch
	return id, ch
}

func (h *eventHub) unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, id)
}

// publish 广播一帧；任何订阅者缓冲满 ⇒ 丢弃该订阅者的这一帧（不阻塞发布方）。
func (h *eventHub) publish(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- b:
		default:
		}
	}
}

// subscriberCount 当前订阅者数（测试与观测用）。
func (h *eventHub) subscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// eventFrame 事件帧的可序列化形状（纯数据，便于单测断言）。
type eventFrame struct {
	Seq          int64   `json:"seq"`
	At           string  `json:"at"`
	State        string  `json:"state"`
	Model        string  `json:"model,omitempty"`
	StateChanged bool    `json:"state_changed"`
	QueueLen     int     `json:"queue_len"`
	HeadETAS     float64 `json:"head_eta_s"` // 0 = 未知或队空（不编造）
	Inflight     int     `json:"inflight"`
}

// buildEventFrame 组装一帧（纯函数，便于测试；seq/at 由调用方给）。
// stateChanged 由调用方按「与上一帧的 state/model 是否变化」判定。
func buildEventFrame(seq int64, at time.Time, state, model string, stateChanged bool, queueLen int, headETA float64, inflight int) eventFrame {
	return eventFrame{
		Seq:          seq,
		At:           at.UTC().Format(time.RFC3339),
		State:        state,
		Model:        model,
		StateChanged: stateChanged,
		QueueLen:     queueLen,
		HeadETAS:     round1f(headETA),
		Inflight:     inflight,
	}
}

// eventTick 采样一次并决定要不要发帧：状态/模型变化必发；队列深度或 ETA 变化也发。
// 返回 (帧, 是否发) 与新的比较基线。
func eventTick(seq int64, now time.Time, state, model string, queueLen int, headETA float64, inflight int,
	lastState, lastModel string, lastLen int, lastETA float64) (eventFrame, bool, int64) {
	changed := state != lastState || model != lastModel
	if !changed && queueLen == lastLen && round1f(headETA) == round1f(lastETA) {
		return eventFrame{}, false, seq
	}
	seq++
	return buildEventFrame(seq, now, state, model, changed, queueLen, headETA, inflight), true, seq
}

// observeTick 一次只读巡检（P7 接线点 1）：**先驱动体征器**（频率分层由记录器自己管：
// 快采 2s 五类 / 慢采 15s 逐进程归因），再取状态机与队列快照。
//
// 为什么必须驱动：不驱动 ⇒ 逐进程归因永远为空 ⇒ `/services` 的 external_occupancy[] 与
// `/eggs` 的 gtt_gb 全是空（真机实测过这个缺口），"外部占用只读数"这条就等于没落地。
func (s *Server) observeTick(now time.Time) (state, model string, queueLen int, headETA float64, inflight int) {
	if s.agent.vitals != nil {
		s.agent.vitals.MaybeCollect(now)
	}
	mgr := s.agent.backends
	state = mgr.State()
	model = mgr.CurrentModel()
	queueLen = mgr.WaitQLen()
	headETA, _ = mgr.WaitQHeadETA()
	for _, o := range mgr.EggObservations() {
		inflight += o.Inflight
	}
	return state, model, queueLen, headETA, inflight
}

// eventLoop 低频巡检（1s）：状态机迁移 + 排队深度 + ETA ⇒ 广播。
// 这是**只读巡检**：只读快照、不改任何状态（与 §7.7 修补 2「时间戳 + 低频巡检」同风格）。
func (s *Server) eventLoop(interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	var seq int64
	var lastState, lastModel string
	var lastLen int
	var lastETA float64
	for range t.C {
		now := time.Now()
		state, model, qLen, headETA, inflight := s.observeTick(now)
		frame, send, newSeq := eventTick(seq, now, state, model, qLen, headETA, inflight,
			lastState, lastModel, lastLen, lastETA)
		seq = newSeq
		if !send {
			continue
		}
		lastState, lastModel, lastLen, lastETA = state, model, qLen, headETA
		if b, err := json.Marshal(frame); err == nil {
			s.events.publish(b)
		}
	}
}

// checkAuthSSE SSE 鉴权：优先 X-Auth-Token 头（与既有端点同门），退化到 ?token=
// （浏览器 EventSource 无法设头）。两处都不对 ⇒ 401。
func (s *Server) checkAuthSSE(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-Auth-Token") == s.agent.token {
		return true
	}
	if q := r.URL.Query().Get("token"); q != "" && q == s.agent.token {
		return true
	}
	log.Printf("[server] SSE 认证失败: token mismatch")
	writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "unauthorized"})
	return false
}

// handleEvents GET /events —— 只读事件流（SSE）。无写动作（红线②）。
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuthSSE(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed（只读端点，仅支持 GET）",
		})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	id, ch := s.events.subscribe()
	defer s.events.unsubscribe(id)

	// 首帧：当前快照（订阅即刻可用，客户端不必等下一次巡检）。
	mgr := s.agent.backends
	headETA, _ := mgr.WaitQHeadETA()
	inflight := 0
	for _, o := range mgr.EggObservations() {
		inflight += o.Inflight
	}
	first, _ := json.Marshal(buildEventFrame(0, time.Now(), mgr.State(), mgr.CurrentModel(), true,
		mgr.WaitQLen(), headETA, inflight))
	fmt.Fprintf(w, "data: %s\n\n", first)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-heartbeat.C:
			// 心跳注释帧：保持连接与代理通路，不产生事件语义。
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// parseIntOr 读整数查询参数（缺省回退）。当前用于 ETA/超时类参数的可覆盖化。
func parseIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
