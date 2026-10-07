package agent

// net_tools_search_service_test.go — A2 步3（2026-10-07）：`net_tools.go` 接 `searx_client.go`（HTTP 直连
// searxng 服务）的**定向两腿**回归牙。
//
// 两腿（httptest **真起一个 HTTP 服务** —— ✗ 不打桩内部函数、✗ 不 `export ZERG_*`）：
//   ① 服务在跑腿：200 + 合法 JSON ⇒ `webSearchBridge` 正常返回结果（✗ 空、✗ 错）。
//   ② 服务不在腿：端口拒连 ⇒ 带具名原因「不可达」的 error（✗ 静默空 —— **两态可分**）。
// 另加：空结果 fail-closed（保留 A-06 语义）· 挂起超时 fail-closed · 撤桥后默认路恒走服务。
//
// 负控（父代理 `go test -overlay` 哑化 ⇒ FAIL 带行号）：把 `searxQueryService` 的空结果 fail-closed
// 还原成静默（`return res, nil`）＋ `webSearchBridge` 的质量过滤空集支还原成「（无搜索结果）」⇒
// `TestSearxSearchServiceEmptyFailsClosed` 必红（行号落该用例 t.Fatalf）；把「不可达」具名文案抹平
// ⇒ `TestSearxSearchServiceUnreachableIsError` 必红（两态不可分）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// withSearxClientOverride 注入服务端客户端（包内变量 —— 测试隔离），测试结束自动复原。
func withSearxClientOverride(t *testing.T, c *SearxClient) {
	t.Helper()
	old := searxBridgeClientOverride
	searxBridgeClientOverride = c
	t.Cleanup(func() { searxBridgeClientOverride = old })
}

// withSearchServiceTimeout 临时改服务查询超时（包内变量 —— 测试隔离），测试结束自动复原。
func withSearchServiceTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := searxBridgeTimeout
	searxBridgeTimeout = d
	t.Cleanup(func() { searxBridgeTimeout = old })
}

// TestSearxSearchServiceResults — 腿①：真起 HTTP 服务回合法 JSON ⇒ webSearchBridge 正常返回结果。
func TestSearxSearchServiceResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "format=json") {
			t.Errorf("请求未带 format=json: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"results":[{"title":"T1","url":"http://example.com/1","content":"C1","score":1.0}]}`))
	}))
	defer srv.Close()
	withSearxClientOverride(t, &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()})
	withSearchServiceTimeout(t, 5*time.Second)

	r, err := webSearchBridge("服务在跑腿", 3, "all", "")
	if err != nil {
		t.Fatalf("腿①：服务在跑应正常返回，实得 err=%v", err)
	}
	if r == nil || len(r.Results) != 1 || !strings.Contains(r.Formatted, "http://example.com/1") {
		t.Fatalf("腿①：结果不符: %+v", r)
	}
}

// TestSearxSearchServiceUnreachableIsError — 腿②：端口拒连 ⇒ 带具名原因「不可达」的 error（✗ 静默空）。
// 「两态可分」：这一腿与上一条（服务在跑）合读 —— 服务不在 ⇒ 具名 error（✗ 不得回落「无搜索结果」）。
func TestSearxSearchServiceUnreachableIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 关端口 ⇒ 必拒连
	withSearxClientOverride(t, &SearxClient{BaseURL: url, HTTP: &http.Client{Timeout: time.Second}})
	withSearchServiceTimeout(t, time.Second)

	r, err := webSearchBridge("服务不在腿", 3, "all", "")
	if err == nil {
		t.Fatalf("腿②：服务不可达应 fail-closed 报 error，实得 nil（r=%+v）—— 静默空 = fail-open", r)
	}
	if !strings.Contains(err.Error(), "不可达") {
		t.Fatalf("腿②：错误未带具名原因「不可达」（两态不可分）: %v", err)
	}
	if strings.Contains(err.Error(), "无搜索结果") {
		t.Fatalf("腿②：✗ 不得回落静默「无搜索结果」: %v", err)
	}
}

// TestSearxSearchServiceEmptyFailsClosed — 保留 A-06 语义：服务可达但空结果 ⇒ 带具名原因「空结果」的 error。
func TestSearxSearchServiceEmptyFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	withSearxClientOverride(t, &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()})
	withSearchServiceTimeout(t, 5*time.Second)

	r, err := webSearchBridge("空结果负控", 3, "all", "")
	if err == nil {
		t.Fatalf("空结果应 fail-closed 报 error，实得 nil（r=%+v）—— 静默「无搜索结果」= fail-open", r)
	}
	if !strings.Contains(err.Error(), "空结果") {
		t.Fatalf("空结果错误未带具名原因「空结果」: %v", err)
	}
}

// TestSearxSearchServiceTimeoutFailsClosed — 服务挂起不答 ⇒ 在 searxBridgeTimeout 内 fail-closed（✗ 永久阻塞）。
func TestSearxSearchServiceTimeoutFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(800 * time.Millisecond)
	}))
	defer srv.Close()
	withSearxClientOverride(t, &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()})
	withSearchServiceTimeout(t, 150*time.Millisecond)

	done := make(chan error, 1)
	t0 := time.Now()
	go func() { _, err := webSearchBridge("超时腿", 3, "all", ""); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("服务挂起不答应报超时 error，实得 nil")
		}
		if !strings.Contains(err.Error(), "超时") {
			t.Fatalf("超时错误未带具名原因「超时」: %v", err)
		}
		if d := time.Since(t0); d > 2*time.Second {
			t.Fatalf("超时未在阈值附近生效：耗时 %s（阈值 150ms）", d)
		}
		t.Logf("超时腿：耗时 %s → %v", time.Since(t0).Round(time.Millisecond), err)
	case <-time.After(3 * time.Second):
		t.Fatal("服务挂起不答却未在 3s 看门狗内返回 —— 超时闸失效")
	}
}

// TestSearxSearchServiceDefaultPath — 撤桥（2026-10-07）：`webSearchBridge` 恒走服务直连
// （旧 Python 桥 searx_search.py 与回退旋钮 ZERG_SEARX_LEGACY_BRIDGE 已退役）。
// 原 TestSearxLegacyBridgeKnob 的「回退格」随旋钮删除退场；此保留其「服务态格」。
func TestSearxSearchServiceDefaultPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"title":"via-service","url":"http://svc/1","content":"c","score":1.0}]}`))
	}))
	defer srv.Close()
	withSearxClientOverride(t, &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()})

	r, err := webSearchBridge("服务默认路", 3, "all", "")
	if err != nil || r == nil || !strings.Contains(r.Formatted, "http://svc/1") {
		t.Fatalf("撤桥后应恒走服务直连: err=%v r=%+v", err, r)
	}
}
