// cli_write_face_timeout_test.go —— 「写面独立超时」的**回归牙**（`GAP-20261005-03`）。
//
// 治的是什么（现读定的接口面）：
//
//	`family_task.go` 里 `sendJSON` 是命令面**唯一**的写面出口（`postJSON` / `putJSON` 都转发到它）。
//	它按 **HTTP 方法**分流客户端：`method != http.MethodGet` ⇒ 走 `writeFaceClient`
//	（`writeFaceTimeout = 60s`）；否则走读面 `c.hc`（`client.go` 里 10s）。
//	独装一个 21GB/35B 冷模型 >45s（`GAP-20260925-44`）⇒ 客户端若与读面同用 10s 会先超时退 12
//	而真果已装成。★ 本件让「分流」长出牙：把两面的超时做成**短替身**（✗ 不真等 60s），
//	用**交叉对拍**证明 POST 走写面、GET 走读面。
//
// 牙怎么咬（★ **行为牙**，✗ 不是「常量断言」）：
//
//	✗ **不改** `family_task.go` 的语义：`writeFaceClient` 是包级 `var`（同包 · `package main`），
//	本件在用例里临时**替换**它、跑完复原。一个「慢答」服务端用延时 D 把两面分开：
//
//	  交叉态①：写面代用 200ms · 读面代用 40ms · D=120ms ⇒ POST 该成、GET 该超时；
//	  交叉态②：写面代用 40ms · 读面代用 200ms · D=120ms ⇒ POST 该超时、GET 该成。
//
//	两面代用值一换、结果对调 ⇒ 只可能是「按方法分流」。**哑化**（把分流拆掉、一律走 `c.hc`）
//	时两态都错（态① POST 超时、态② POST 成功）⇒ 用例 **FAIL**。
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// wftSlowServer 起一个「慢答」服务端：收到请求后先等 d 再回 200（用 d 把两个超时档分开）。
func wftSlowServer(t *testing.T, d time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(d)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// wftRunState 跑一态：把**写面**客户端临时换成 writeTO、**读面** `c.hc` 换成 readTO，
// 服务端慢答 delay；对同一个慢服务端分别发 POST 与 GET，返回两者**是否成功**（err==nil）。
// ★ 只临时替换包级 `writeFaceClient`（`defer` 复原）—— ✗ 不改 `family_task.go` 的语义。
func wftRunState(t *testing.T, writeTO, readTO, delay time.Duration) (postOK, getOK bool) {
	t.Helper()
	orig := writeFaceClient
	defer func() { writeFaceClient = orig }()

	srv := wftSlowServer(t, delay)
	writeFaceClient = &http.Client{Timeout: writeTO}
	c := &client{base: srv.URL, hc: &http.Client{Timeout: readTO}}

	_, _, perr := sendJSON(c, http.MethodPost, "/api/tasks", `{}`)
	_, _, gerr := sendJSON(c, http.MethodGet, "/api/sched", "")
	return perr == nil, gerr == nil
}

// TestWriteFaceTimeoutRoutingByMethod —— 「按方法分流」的交叉对拍（行为牙）。
func TestWriteFaceTimeoutRoutingByMethod(t *testing.T) {
	delay := 120 * time.Millisecond

	// 态①：写面代用 200ms（够答）· 读面代用 40ms（不够）⇒ POST 该成、GET 该超时。
	postOK, getOK := wftRunState(t, 200*time.Millisecond, 40*time.Millisecond, delay)
	if !postOK {
		t.Fatalf("态①：POST 该走写面（代用 200ms ≥ 服务端 %v）却失败 ⇒ 写面分流没生效（哑化？）", delay)
	}
	if getOK {
		t.Fatalf("态①：GET 该走读面（代用 40ms < 服务端 %v）却成功 ⇒ GET 被误分流到写面", delay)
	}

	// 态②：写面代用 40ms（不够）· 读面代用 200ms（够）⇒ POST 该超时、GET 该成。
	postOK, getOK = wftRunState(t, 40*time.Millisecond, 200*time.Millisecond, delay)
	if postOK {
		t.Fatalf("态②：POST 该走写面（代用 40ms < 服务端 %v）却成功 ⇒ 写面分流按方法判的没生效（哑化？）", delay)
	}
	if !getOK {
		t.Fatalf("态②：GET 该走读面（代用 200ms ≥ 服务端 %v）却失败 ⇒ 读面被误分流到写面", delay)
	}
}

// TestWriteFaceTimeoutConstants —— 常量面守卫：写面 60s · 读面逐字 10s（✗ 不许被写面一起抬）。
func TestWriteFaceTimeoutConstants(t *testing.T) {
	if writeFaceTimeout != 60*time.Second {
		t.Fatalf("writeFaceTimeout = %v，要 60s（GAP-20261005-03：写面独立超时）", writeFaceTimeout)
	}
	if got := writeFaceClient.Timeout; got != writeFaceTimeout {
		t.Fatalf("writeFaceClient.Timeout = %v，要与 writeFaceTimeout(%v) 一致", got, writeFaceTimeout)
	}
	// 真源在 client.go 的 newClient：读面逐字保持 10s。
	if got := newClient().hc.Timeout; got != 10*time.Second {
		t.Fatalf("读面 c.hc.Timeout = %v，要逐字保持 10s（✗ 不许随写面一起抬）", got)
	}
}
