// events_endpoint_test.go —— P7 批 2：SSE /events 的验收（鉴权 / 只读 / 帧语义 / 慢订阅者不拖发布方）。
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

func eventsServer() *Server {
	return NewServer(NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), ""))
}

// 无令牌 ⇒ 401（只读面不开口子）。
func TestEvents_AuthRequired(t *testing.T) {
	s := eventsServer()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	s.handleEvents(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401，实得 %d", rec.Code)
	}
}

// 查询参数令牌可用（浏览器 EventSource 不能设头），但**头优先**且两者都不对 ⇒ 401。
func TestEvents_QueryTokenAcceptedAndWrongRejected(t *testing.T) {
	// 错的 token（头 + 查询）⇒ 401
	s := eventsServer()
	req := httptest.NewRequest(http.MethodGet, "/events?token=nope", nil)
	req.Header.Set("X-Auth-Token", "nope")
	rec := httptest.NewRecorder()
	s.handleEvents(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("错令牌应 401，实得 %d", rec.Code)
	}

	// 对的 token 走查询参数 ⇒ 进入流（首帧必到）
	s2 := eventsServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req2 := httptest.NewRequest(http.MethodGet, "/events?token=tok", nil).WithContext(ctx)
	rec2 := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { s2.handleEvents(rec2, req2); close(done) }()
	// 首帧在进入流后立即写出；给一点时间后取消——**读缓冲必须在处理返回之后**
	// （httptest.ResponseRecorder 的 Buffer 不是并发安全的：一边写一边读会触发 race）。
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done
	body := rec2.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("应至少发出首帧快照，实得 %q", body)
	}
	if !strings.Contains(body, `"state"`) {
		t.Fatalf("首帧应带 state 字段，实得 %q", body)
	}
	if ct := rec2.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type 应为 text/event-stream，实得 %q", ct)
	}
}

// 只读端点：非 GET ⇒ 405。
func TestEvents_MethodNotAllowed(t *testing.T) {
	s := eventsServer()
	req := httptest.NewRequest(http.MethodPost, "/events", nil)
	req.Header.Set("X-Auth-Token", "tok")
	rec := httptest.NewRecorder()
	s.handleEvents(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 应 405，实得 %d", rec.Code)
	}
}

// 帧语义：状态/模型变化必发（state_changed=true）；仅队列深度变化也发（state_changed=false）；
// 完全无变化不发。
func TestEventTick_ChangeDetection(t *testing.T) {
	now := time.Now()

	// 无变化 ⇒ 不发
	if _, send, seq := eventTick(5, now, "ready", "qwen", 0, 0, 0, "ready", "qwen", 0, 0); send || seq != 5 {
		t.Fatalf("无变化不应发帧，send=%v seq=%d", send, seq)
	}
	// 状态变化 ⇒ 发 + state_changed=true + seq+1
	f, send, seq := eventTick(5, now, "loading", "qwen", 0, 0, 0, "ready", "qwen", 0, 0)
	if !send || !f.StateChanged || f.Seq != 6 || f.State != "loading" {
		t.Fatalf("状态变化应发帧且标 changed，实得 %+v send=%v seq=%d", f, send, seq)
	}
	// 模型变化 ⇒ 同样算 changed
	f2, send2, _ := eventTick(6, now, "ready", "k2", 0, 0, 0, "ready", "qwen", 0, 0)
	if !send2 || !f2.StateChanged || f2.Model != "k2" {
		t.Fatalf("模型变化应发帧且标 changed，实得 %+v", f2)
	}
	// 仅排队深度变化 ⇒ 发，但 state_changed=false
	f3, send3, _ := eventTick(7, now, "ready", "k2", 3, 12.5, 1, "ready", "k2", 0, 0)
	if !send3 || f3.StateChanged || f3.QueueLen != 3 || f3.HeadETAS != 12.5 || f3.Inflight != 1 {
		t.Fatalf("排队深度变化应发帧且不标 changed，实得 %+v", f3)
	}
	// ETA 只有小数位内抖动（round1f 后相同）⇒ 不发：防抖动刷屏
	if _, send4, _ := eventTick(8, now, "ready", "k2", 3, 12.5001, 1, "ready", "k2", 3, 12.5); send4 {
		t.Fatal("ETA 亚分位抖动不应发帧（防刷屏）")
	}
}

// 广播非阻塞：慢订阅者不拖住发布方（其缓冲满则丢帧，其他订阅者照常收到）。
func TestEventHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	h := newEventHub()
	_, fast := h.subscribe()
	_, slow := h.subscribe() // 故意不消费

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			h.publish([]byte("frame"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish 被慢订阅者阻塞了（应丢帧而非等待）")
	}
	// fast 也应拿到若干帧（非零）
	select {
	case <-fast:
	default:
		t.Fatal("活跃订阅者应至少收到一帧")
	}
	// slow 的缓冲不应超过其容量（丢帧而非堆积）
	if len(slow) > cap(slow) {
		t.Fatalf("慢订阅者缓冲不应超容，实得 len=%d cap=%d", len(slow), cap(slow))
	}
	if got := h.subscriberCount(); got != 2 {
		t.Fatalf("订阅者数应为 2，实得 %d", got)
	}
}

// 订阅/退订：退订后不再计数（SSE 断连路径）。
func TestEventHub_Unsubscribe(t *testing.T) {
	h := newEventHub()
	id, _ := h.subscribe()
	if h.subscriberCount() != 1 {
		t.Fatalf("订阅后应为 1，实得 %d", h.subscriberCount())
	}
	h.unsubscribe(id)
	if h.subscriberCount() != 0 {
		t.Fatalf("退订后应为 0，实得 %d", h.subscriberCount())
	}
	h.publish([]byte("nobody-listens")) // 不得 panic
}

// 帧 JSON 形状（客户端契约）：字段名固定，ETA 未知时如实为 0。
func TestEventFrame_JSONShape(t *testing.T) {
	b, err := json.Marshal(buildEventFrame(3, time.Now(), "idle", "", false, 0, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"seq", "at", "state", "state_changed", "queue_len", "head_eta_s", "inflight"} {
		if _, ok := m[k]; !ok {
			t.Errorf("帧缺少字段 %s：%s", k, b)
		}
	}
	if _, ok := m["model"]; ok {
		t.Errorf("空模型应省略 model 字段（omitempty），实得 %s", b)
	}
}
