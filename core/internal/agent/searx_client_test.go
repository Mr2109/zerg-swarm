package agent

// searx_client_test.go — A2 步2 `searx_client.go` 的定向单测（httptest 打桩 · 不联网）。
//
// 覆盖：正控（200+JSON ⇒ 解析 + 请求面逐字）· ★ 空结果非 error（vs 服务不可达 error，两态可分）·
// 5xx / 坏 JSON / 连不上 ⇒ error · 重试 1 次 · env 取址。
//
// 负控（哑化⇒FAIL 带行号）：把 `searx_client.go` 的「空结果 ⇒ (空, nil)」改成「空结果也返 error」
// ⇒ `TestSearxClientEmptyIsNotError` 必红（FAIL 行号落该用例的 `t.Fatalf`）。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestSearxClientSearchOK —— 正控：200 + 合法 JSON ⇒ 解析出结果，且请求面逐字
// （q / format=json / language / time_range / safesearch=0 / pageno=1 + domains→site:）。
func TestSearxClientSearchOK(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[` +
			`{"title":"T1","url":"https://a/","content":"C1","score":1.5},` +
			`{"title":"T2","url":"https://b/","content":"C2","score":0.9}]}`))
	}))
	defer srv.Close()

	c := &SearxClient{BaseURL: srv.URL, HTTP: srv.Client(), Retries: 0}
	got, err := c.Search(context.Background(), SearxQuery{
		Query: "虫族 分布式 节点", Limit: 5, Lang: "zh", TimeRange: "week",
		Domains: []string{"a.com", "b.com"},
	})
	if err != nil {
		t.Fatalf("正控：Search 应成功，实得 err=%v", err)
	}
	if len(got) != 2 || got[0].Title != "T1" || got[1].URL != "https://b/" || got[0].Score != 1.5 {
		t.Fatalf("结果解析不符: %#v", got)
	}
	for _, kv := range []string{"format=json", "language=zh", "time_range=week", "safesearch=0", "pageno=1"} {
		if !strings.Contains(gotURL, kv) {
			t.Errorf("请求面缺 %q（实得 %s）", kv, gotURL)
		}
	}
	if !strings.Contains(gotURL, "site%3Aa.com") || !strings.Contains(gotURL, "site%3Ab.com") {
		t.Errorf("domains 未折成 site: 操作符（实得 %s）", gotURL)
	}
}

// TestSearxClientEmptyIsNotError —— ★ 本件核心：200 + {"results":[]} ⇒ 空结果 **且 err=nil**
// （真无结果 ≠ 服务不可达）。哑化：空结果也判 error ⇒ 本用例在 t.Fatalf 那行红。
func TestSearxClientEmptyIsNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()}
	got, err := c.Search(context.Background(), SearxQuery{Query: "zzz 无结果"})
	if err != nil {
		t.Fatalf("空结果不该是 error（真无结果 ≠ 不可达）: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("空结果集应 len=0，实得 %d", len(got))
	}
}

// TestSearxClientUnreachableIsError —— 对偶：连不上 ⇒ 必须 error（与上一条合成「两态可分」对）。
func TestSearxClientUnreachableIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 立即关：端口必拒连
	c := &SearxClient{BaseURL: url, HTTP: &http.Client{Timeout: time.Second}}
	if _, err := c.Search(context.Background(), SearxQuery{Query: "x"}); err == nil {
		t.Fatal("服务不可达必须返 error，实得 nil")
	}
}

// TestSearxClientHTTP500IsError —— 非 200 ⇒ error。
func TestSearxClientHTTP500IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Search(context.Background(), SearxQuery{Query: "x"}); err == nil {
		t.Fatal("HTTP 500 必须返 error，实得 nil")
	}
}

// TestSearxClientBadJSONIsError —— JSON 坏 ⇒ error。
func TestSearxClientBadJSONIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()
	c := &SearxClient{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Search(context.Background(), SearxQuery{Query: "x"}); err == nil {
		t.Fatal("坏 JSON 必须返 error，实得 nil")
	}
}

// TestSearxClientRetryOnce —— 首次 500、再次 200 ⇒ 恰好请求 2 次（1 次重试）并成功。
func TestSearxClientRetryOnce(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"title":"ok"}]}`))
	}))
	defer srv.Close()
	c := &SearxClient{BaseURL: srv.URL, HTTP: srv.Client(), Retries: 1}
	got, err := c.Search(context.Background(), SearxQuery{Query: "x"})
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if len(got) != 1 || got[0].Title != "ok" {
		t.Fatalf("重试结果不符: %#v", got)
	}
	if atomic.LoadInt32(&n) != 2 {
		t.Fatalf("应恰好请求 2 次（1 次重试），实得 %d", n)
	}
}

// TestSearxClientEnvBaseURL —— NewSearxClient 取址走 env，缺省回落默认 8888。
func TestSearxClientEnvBaseURL(t *testing.T) {
	t.Setenv(searxEnvBaseURL, "http://127.0.0.1:9999")
	if got := NewSearxClient().BaseURL; got != "http://127.0.0.1:9999" {
		t.Fatalf("env 取址不符: %q", got)
	}
	t.Setenv(searxEnvBaseURL, "")
	if got := NewSearxClient().BaseURL; got != searxDefaultBaseURL {
		t.Fatalf("默认取址不符: %q want %q", got, searxDefaultBaseURL)
	}
	if got := NewSearxClient().Retries; got != searxDefaultRetries {
		t.Fatalf("默认重试数不符: %d want %d", got, searxDefaultRetries)
	}
}
