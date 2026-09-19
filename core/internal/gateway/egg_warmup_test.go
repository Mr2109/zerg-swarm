package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// 缺口 ⑤（冷卵自动孵）路径用例。判据（正解版：只在 5xx「model not available」后孵 + 原地重发一次）：
//
//	① 冷卵：先 503 ⇒ 必须孵（/load 恰 1 次）⇒ **原地重发恰 1 次** ⇒ 第二次 200 返回给调用方
//	② 已经是 ready 却回 5xx ⇒ 不孵（不是"冷"的问题）⇒ 不重发 ⇒ 503 原样上抛（不掩盖真故障）
func newForwardTestGateway(t *testing.T, srv *httptest.Server) *Gateway {
	t.Helper()
	u := strings.TrimPrefix(srv.URL, "http://")
	i := strings.LastIndex(u, ":")
	port := 0
	for _, c := range u[i+1:] {
		port = port*10 + int(c-'0')
	}
	return &Gateway{
		config:    &config.FleetConfig{Fleet: map[string]config.FleetNode{"x3": {Host: u[:i], Port: port}}},
		authToken: "test-token",
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func TestForwardToBackend_ColdEggHatchedThenRetriedOnce(t *testing.T) {
	var inferCalls, loadCalls int32
	ready := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/eggs":
			state := "cold"
			if ready {
				state = "ready"
			}
			io.WriteString(w, `{"eggs":[{"model":"deepseek-v4-flash","state":"`+state+`"}]}`)
		case "/load":
			atomic.AddInt32(&loadCalls, 1)
			ready = true
			io.WriteString(w, `{"ok":true}`)
		case "/infer":
			n := atomic.AddInt32(&inferCalls, 1)
			if !ready {
				w.WriteHeader(http.StatusServiceUnavailable)
				io.WriteString(w, `{"error":"model not available"}`)
				return
			}
			if n == 1 {
				t.Errorf("第一次转发就该吃到 503（冷卵）")
			}
			io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	g := newForwardTestGateway(t, srv)
	route := &RouteResult{Host: "x3", URL: srv.URL + "/infer"}
	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)

	resp, err := g.forwardToBackend(context.Background(), route, "/v1/chat/completions", body, http.Header{}, nil)
	if err != nil {
		t.Fatalf("不该报错（应孵后重发成功）: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重发后应 200，实际 %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(got), "ok") {
		t.Fatalf("重发返回体不对: %s", got)
	}
	if n := atomic.LoadInt32(&loadCalls); n != 1 {
		t.Fatalf("/load 应恰 1 次，实际 %d", n)
	}
	if n := atomic.LoadInt32(&inferCalls); n != 2 {
		t.Fatalf("/infer 应恰 2 次（首发 + 重发一次），实际 %d", n)
	}
}

func TestForwardToBackend_ReadyEggFiveXXNotHatched(t *testing.T) {
	var inferCalls, loadCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/eggs":
			io.WriteString(w, `{"eggs":[{"model":"deepseek-v4-flash","state":"ready"}]}`)
		case "/load":
			atomic.AddInt32(&loadCalls, 1)
			io.WriteString(w, `{"ok":true}`)
		default:
			atomic.AddInt32(&inferCalls, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":"model not available"}`)
		}
	}))
	defer srv.Close()

	g := newForwardTestGateway(t, srv)
	route := &RouteResult{Host: "x3", URL: srv.URL + "/infer"}
	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)

	resp, err := g.forwardToBackend(context.Background(), route, "/v1/chat/completions", body, http.Header{}, nil)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("已是 ready 却回 5xx ⇒ 应原样上抛 503，实际 %d", resp.StatusCode)
	}
	if n := atomic.LoadInt32(&loadCalls); n != 0 {
		t.Fatalf("不是冷卵就不该孵，/load 实际 %d 次", n)
	}
	if n := atomic.LoadInt32(&inferCalls); n != 1 {
		t.Fatalf("不该重发，/infer 实际 %d 次", n)
	}
}

// 非 5xx（正常 200）时一个字节都不该多读、不该孵（热路径零侵入）
func TestForwardToBackend_HealthyNoEggTraffic(t *testing.T) {
	var eggCalls, loadCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eggs" {
			atomic.AddInt32(&eggCalls, 1)
			io.WriteString(w, `{"eggs":[]}`)
			return
		}
		if r.URL.Path == "/load" {
			atomic.AddInt32(&loadCalls, 1)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"hello"}}]}`)
	}))
	defer srv.Close()

	g := newForwardTestGateway(t, srv)
	route := &RouteResult{Host: "x3", URL: srv.URL + "/infer"}
	body := []byte(`{"model":"deepseek-v4-flash","messages":[]}`)

	resp, err := g.forwardToBackend(context.Background(), route, "/v1/chat/completions", body, http.Header{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("200 响应体被破坏: %v (%s)", err, b)
	}
	if atomic.LoadInt32(&eggCalls) != 0 || atomic.LoadInt32(&loadCalls) != 0 {
		t.Fatalf("健康响应不该产生任何蛋流量（/eggs=%d /load=%d）",
			atomic.LoadInt32(&eggCalls), atomic.LoadInt32(&loadCalls))
	}
}
