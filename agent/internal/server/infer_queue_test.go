// infer_queue_test.go —— P7 批 3：/infer 准入切到等待队列的验收（429 / 503+Retry-After / ETA）。
//
// 设计依据：设计-子端沙箱化-20260914.md §5.3（错误码表：队列满 ⇒ 立即 429；
// 排队超时上限到达 ⇒ 503 + Retry-After）· §13 Q5（排队 + ETA）· §8.4（ETA 只能来自实测档案）。
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

func inferServer() *Server {
	return NewServer(NewAgent("x3", "tok", nil, backend.NewManager(nil, "x3"), ""))
}

// postInfer 直接调 handler，返回状态码 / Retry-After / 解析后的响应。
func postInfer(t *testing.T, s *Server, model string) (int, string, map[string]interface{}) {
	t.Helper()
	body := `{"model":"` + model + `","stream":false,"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/infer", strings.NewReader(body))
	req.Header.Set("X-Auth-Token", "tok")
	rec := httptest.NewRecorder()
	s.handleInfer(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, rec.Header().Get("Retry-After"), out
}

// 队列满 ⇒ **立即** 429 + Retry-After（Q5：不挂起、不排队）。
func TestInfer_QueueFullIs429WithRetryAfter(t *testing.T) {
	s := inferServer()
	// 填满队列（p2Queue 默认限长 64），不启动 worker ⇒ 队列不会被消费。
	for i := 0; i < backend.WaitQueueCapacityForTest(); i++ {
		if !s.agent.backends.WaitQPush("filler", nil, 0) {
			t.Fatalf("第 %d 项应能入队（上限 %d）", i+1, backend.WaitQueueCapacityForTest())
		}
	}
	code, ra, out := postInfer(t, s, "qwen")
	if code != http.StatusTooManyRequests {
		t.Fatalf("队列满应 429，实得 %d (%v)", code, out)
	}
	if ra == "" {
		t.Fatal("429 必须带 Retry-After（客户端要读得懂还要等多久）")
	}
	if ra != "30" {
		t.Fatalf("无实测档案时 Retry-After 应为保守缺省 30，实得 %q", ra)
	}
}

// 排队+执行超过上限 ⇒ 503 + Retry-After（可等的失败，不是服务故障——不得回 5xx 静默）。
func TestInfer_WaitTimeoutIs503WithRetryAfter(t *testing.T) {
	t.Setenv("ZERG_INFER_WAIT_TIMEOUT_S", "1")
	s := inferServer()
	// 不启动 worker：请求入队后无人消费 ⇒ 必然走到超时分支。
	start := time.Now()
	code, ra, out := postInfer(t, s, "qwen")
	elapsed := time.Since(start)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("排队超时上限到达应 503，实得 %d (%v)", code, out)
	}
	if ra == "" {
		t.Fatal("503 必须带 Retry-After")
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("应在超时上限（1s）后才返回，实得 %v", elapsed)
	}
	// 请求确实入了队（未被丢弃）
	if got := s.agent.backends.WaitQLen(); got != 1 {
		t.Fatalf("超时后请求仍应在队中等待 worker（不静默丢），实得队列长度 %d", got)
	}
}

// ETA 只能来自实测档案：无档案 ⇒ 0（不编造）；retryAfterSeconds 对已知 ETA 如实取整。
func TestInfer_ETAComesFromProfileOnly(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 空档案目录 ⇒ 必然无档案
	if eta := estimateWaitETA("不存在的模型"); eta != 0 {
		t.Fatalf("无实测档案时 ETA 必须为 0（标定铁律：不许猜），实得 %v", eta)
	}
	if got := retryAfterSeconds(90 * time.Second); got != 90 {
		t.Fatalf("已知 ETA 应如实取整为 90，实得 %d", got)
	}
	if got := retryAfterSeconds(0); got != 30 {
		t.Fatalf("未知 ETA 应给保守缺省 30，实得 %d", got)
	}
	if got := retryAfterSeconds(500 * time.Millisecond); got != 1 {
		t.Fatalf("亚秒 ETA 至少给 1 秒（不得为 0），实得 %d", got)
	}
}

// 执行期**不受排队上限约束**（2026-09-16 治本：秒数只管排队）。
// 变异可验：把 handler 里的 `case <-startedCh:` 分支去掉 ⇒ 本用例必红（1s 后回 503）。
func TestInfer_ExecutionPhaseNotCutByQueueTimeout(t *testing.T) {
	t.Setenv("ZERG_INFER_WAIT_TIMEOUT_S", "1") // 排队上限压到 1 秒
	s := inferServer()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", strings.NewReader(`{"model":"qwen","stream":false,"messages":[]}`))
	req.Header.Set("X-Auth-Token", "tok")
	done := make(chan struct{})
	go func() { defer close(done); s.handleInfer(rec, req) }()

	// 等它入队（不启动真 worker：由测试精确模拟一次出队）
	deadline := time.Now().Add(2 * time.Second)
	for s.agent.backends.WaitQLen() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("请求没能入队")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// currentModel 返回我们刚入队那个模型 ⇒ 走向"匹配"分支（不触发换卵重校验）
	_, payload, ok, _ := s.agent.backends.WaitQPop(func() string { return "qwen" }, false)
	if !ok {
		t.Fatal("取不到队头")
	}
	markStarted(payload) // 与产品路径同一个函数：模拟"worker 已取走、进入执行期"

	// 执行期：等 2.5×排队上限。**不得**因为排队到点而返回。
	time.Sleep(2500 * time.Millisecond)
	select {
	case <-done:
		t.Fatalf("执行期被排队上限砍断：提前返回 code=%d body=%s", rec.Code, rec.Body.String())
	default:
	}

	// 交付结果让 handler 正常收口（不泄漏 goroutine）
	if r, isReq := payload.(inferReq); isReq {
		r.resultCh <- inferResult{status: 200, body: []byte(`{"ok":true}`)}
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("交付结果后 handler 仍未收口")
	}
	if rec.Code == http.StatusServiceUnavailable && strings.Contains(rec.Body.String(), "queued timeout") {
		t.Fatal("执行期不得回 queued timeout")
	}
}
