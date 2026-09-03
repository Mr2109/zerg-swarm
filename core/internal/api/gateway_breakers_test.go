package api

// gateway_breakers_test.go — 熔断快照/复位 HTTP 接口测试（httptest + 真实 chi 路由 + AuthMiddleware）。
//
// 用真实 chi.NewRouter + AuthMiddleware("test-token") 组路由，验证：
//   - 带令牌 GET /api/gateway/breakers → 200 且 JSON 可解析（breakers 为数组）；
//   - 不带令牌 → 401（新接口不是免鉴权旁路）；
//   - POST /api/gateway/breakers/reset（无 body / {"host":"x3"}）→ 200 且 reset >= 0；
//   - 非法 body → 400；Gateway 未注入 → 503。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/gateway"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

// newBreakerTestRouter 建一个与 main.go 注册方式一致的测试路由（同一套 AuthMiddleware）。
func newBreakerTestRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware("test-token"))
	r.Get("/api/gateway/breakers", h.BreakersHandler)
	r.Post("/api/gateway/breakers/reset", h.BreakersResetHandler)
	return r
}

// newBreakerTestHandlers 带真实 Gateway 实例的 handlers（网关构造方式与 main.go 一致）。
func newBreakerTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	h := newTestHandlers()
	h.Gateway = gateway.NewGateway("test-token", h.Config, nil, store.NewStore(), nil)
	if h.Gateway == nil {
		t.Fatal("Gateway 构造失败")
	}
	return h
}

// doBreakerReq 发一次请求（token 为空则不带头）。
func doBreakerReq(t *testing.T, r *chi.Mux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestBreakerAPI_SnapshotOK GET 带令牌 → 200 + JSON 可解析 + breakers 为数组（初始全 closed）。
func TestBreakerAPI_SnapshotOK(t *testing.T) {
	h := newBreakerTestHandlers(t)
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodGet, "/api/gateway/breakers", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Breakers []gateway.BreakerInfo `json:"breakers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v body=%s", err, w.Body.String())
	}
	if resp.Breakers == nil {
		t.Fatalf("breakers 应为数组——实际 null body=%s", w.Body.String())
	}
	for _, b := range resp.Breakers {
		if b.State != "closed" {
			t.Errorf("初始 state 应为 closed——host=%s 实际 %s", b.Host, b.State)
		}
	}
	t.Logf("GET /api/gateway/breakers → 200, breakers=%d 条: %s", len(resp.Breakers), w.Body.String())
}

// TestBreakerAPI_SnapshotRequiresAuth 不带令牌 → 401（鉴权中间件确实生效）。
func TestBreakerAPI_SnapshotRequiresAuth(t *testing.T) {
	h := newBreakerTestHandlers(t)
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodGet, "/api/gateway/breakers", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401——实际 %d body=%s", w.Code, w.Body.String())
	}
	w2 := doBreakerReq(t, r, http.MethodPost, "/api/gateway/breakers/reset", "wrong-token", "")
	if w2.Code != http.StatusForbidden {
		t.Fatalf("错误令牌应 403——实际 %d body=%s", w2.Code, w2.Body.String())
	}
}

// TestBreakerAPI_ResetAllNoBody POST 空 body → 200 + reset >= 0（空 body = 全部复位）。
func TestBreakerAPI_ResetAllNoBody(t *testing.T) {
	h := newBreakerTestHandlers(t)
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodPost, "/api/gateway/breakers/reset", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Reset *int `json:"reset"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v body=%s", err, w.Body.String())
	}
	if resp.Reset == nil {
		t.Fatalf("响应应含 reset 字段——body=%s", w.Body.String())
	}
	if *resp.Reset < 0 {
		t.Errorf("reset 应 >= 0——实际 %d", *resp.Reset)
	}
	t.Logf("POST /api/gateway/breakers/reset（空 body）→ 200 %s", w.Body.String())
}

// TestBreakerAPI_ResetSingleHost POST {"host":"x3"} → 200 + reset >= 0。
func TestBreakerAPI_ResetSingleHost(t *testing.T) {
	h := newBreakerTestHandlers(t)
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodPost, "/api/gateway/breakers/reset", "test-token", `{"host":"x3"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Reset *int `json:"reset"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v body=%s", err, w.Body.String())
	}
	if resp.Reset == nil || *resp.Reset < 0 {
		t.Errorf("reset 应存在且 >= 0——body=%s", w.Body.String())
	}
	t.Logf("POST /api/gateway/breakers/reset {\"host\":\"x3\"} → 200 %s", w.Body.String())
}

// TestBreakerAPI_ResetInvalidBody 非法 JSON → 400。
func TestBreakerAPI_ResetInvalidBody(t *testing.T) {
	h := newBreakerTestHandlers(t)
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodPost, "/api/gateway/breakers/reset", "test-token", "not json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 body 应 400——实际 %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("INVALID_BODY")) {
		t.Errorf("错误码应为 INVALID_BODY——body=%s", w.Body.String())
	}
}

// TestBreakerAPI_GatewayNotReady Gateway 未注入 → 503（不 panic）。
func TestBreakerAPI_GatewayNotReady(t *testing.T) {
	h := newTestHandlers() // Gateway 为 nil
	r := newBreakerTestRouter(h)

	w := doBreakerReq(t, r, http.MethodGet, "/api/gateway/breakers", "test-token", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Gateway 未注入时 GET 应 503——实际 %d body=%s", w.Code, w.Body.String())
	}
	w2 := doBreakerReq(t, r, http.MethodPost, "/api/gateway/breakers/reset", "test-token", "")
	if w2.Code != http.StatusServiceUnavailable {
		t.Fatalf("Gateway 未注入时 POST 应 503——实际 %d body=%s", w2.Code, w2.Body.String())
	}
}
