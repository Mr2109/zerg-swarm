package backend

// defect17_inflight_wiring_test.go —— 真机缺陷 17（2026-09-15 切生产后抓到）的回归判据。
//
// 真机现象（子端日志与引擎日志时间戳咬合）：
//
//	18:20:44 引擎 n_gen=137 tg=12.17 t/s        ← 正在出字
//	18:20:48 子端「异模型请求 GLM-4.7-Flash：提前收卵 Qwen3.8-27B（无在飞，锁内置 draining）」
//	          ⇒ 收卵砍断在飞生成，客户端 500 EOF
//
// 根因：inflight 计数两头错位——`Start` 的「已驻留复用」分支 `sp.inflight++`（把**加载**记成在飞），
// 而唯一的 `releaseInflight` 在 `InferForward` 尾部（**推理**完成）⇒ 生成进行中 inflight==0
// ⇒ `RequestModel` 的「异模型请求：有在飞就先等生成跑完」（§7.2 / Q6 不打断在飞）永不成立。
//
// 本文件钉住三条（改回旧写法即红）：
//
//	① 转发进行中（引擎还在吐 body）⇒ inflight == 1；
//	② 此刻异模型请求 ⇒ blocked=true / needUnload=false（**不许**收卵）；
//	③ 客户端把 body 读干并 Close 后 ⇒ inflight 归零，异模型请求才能赢权收卵（needUnload=true）。

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func waitCond17(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return f()
}

// fakeSlowEngine 假引擎：GET 立刻 200（healthCheck 用）；POST 先回响应头再卡住（=正在出字）。
func fakeSlowEngine(gate <-chan struct{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush() // 头先回去：这正是真机形态（转发"返回"了，生成还在继续）
		}
		<-gate
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
}

func portOf17(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefect17_InferForwardMaintainsInflight(t *testing.T) {
	gate := make(chan struct{})
	srv := fakeSlowEngine(gate)
	defer srv.Close()
	port := portOf17(t, srv.URL)

	m := newEvictTestManager(1, map[string]*subproc{
		"m": {model: "m", entry: inflightEntry(), port: port, state: StateReady, lastUsed: time.Now()},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := m.InferForward(context.Background(), "m", "/v1/chat/completions", []byte(`{"model":"m"}`))
		if err != nil {
			return
		}
		_, _ = io.ReadAll(resp.Body) // 真机路径：读干再 Close（流式则边读边写客户端）
		_ = resp.Body.Close()
	}()

	// ① 生成进行中 ⇒ inflight == 1
	if !waitCond17(3*time.Second, func() bool { return m.inflightCount("m") == 1 }) {
		t.Fatalf("转发进行中 inflight 应为 1（缺陷 17 回归），实得 %d", m.inflightCount("m"))
	}
	// ② 此刻异模型请求 ⇒ 必须 blocked、不得收卵
	if needUnload, blocked := m.RequestModel("other"); needUnload || !blocked {
		t.Fatalf("生成进行中：异模型请求应 blocked=true / needUnload=false，实得 needUnload=%v blocked=%v"+
			"（真机那次就是这里判成「无在飞」⇒ 收卵 ⇒ 500 EOF）", needUnload, blocked)
	}

	close(gate)
	<-done

	// ③ 读完并 Close 后 ⇒ 归零；此时异模型请求可赢权
	if !waitCond17(3*time.Second, func() bool { return m.inflightCount("m") == 0 }) {
		t.Fatalf("响应体读完并 Close 后 inflight 应回 0，实得 %d（release 挂在 body.Close 上）", m.inflightCount("m"))
	}
	if needUnload, blocked := m.RequestModel("other"); !needUnload || blocked {
		t.Fatalf("在飞归零后异模型请求应可赢权（needUnload=true blocked=false），实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
}

// 判据补充：`/load` 复用**不得**动 inflight（旧写法在这里 +1 ⇒ 加载被记成在飞 ⇒ 计数漂移）。
func TestDefect17_ReuseOfResidentDoesNotInflateInflight(t *testing.T) {
	m := startLockTestManager(t, "egga")
	m.mu.Lock()
	m.procs["egga"] = &subproc{
		model: "egga", port: 9411, state: StateIdleArmed,
		lastUsed: time.Now().Add(-time.Minute), entry: inflightEntry(),
	}
	m.mu.Unlock()

	resp, err := m.Start("egga")
	if err != nil {
		t.Fatalf("复用路径不应报错: %v", err)
	}
	if resp == nil || resp["ok"] != true {
		t.Fatalf("已驻留应直接复用（ok=true），实得 %+v", resp)
	}
	if got := m.inflightCount("egga"); got != 0 {
		t.Fatalf("复用（加载）不得计入在飞：inflight 应为 0，实得 %d（缺陷 17 的错位就出在这里）", got)
	}
	if st := m.StateOf("egga"); st != StateReady {
		t.Fatalf("复用应取空窗、回 ready，实得 %s", st)
	}
}

func TestIsBackendBusy(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"未驻留", errors.New(`模型 m 未驻留`), false},
		{"响应头超时", errors.New(`转发到后端失败: Post "http://127.0.0.1:9400/v1/chat/completions": net/http: timeout awaiting response headers`), true},
		{"引擎断连EOF", errors.New(`转发到后端失败: Post "http://127.0.0.1:9400/v1/chat/completions": EOF`), true},
		{"对端重置", errors.New(`转发到后端失败: read tcp 127.0.0.1:1->127.0.0.1:9400: connection reset by peer`), true},
		{"客户端超时", context.DeadlineExceeded, true},
	}
	for _, c := range cases {
		if got := IsBackendBusy(c.err); got != c.want {
			t.Errorf("%s: IsBackendBusy=%v 期望 %v", c.name, got, c.want)
		}
	}
}

// 首字节超时按请求形态区分（2026-09-15 生产复验）：非流式长生成曾被 90s 看门狗误杀。
func TestBodyStreams(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{`{"model":"m","stream":true}`, true},
		{`{"model":"m","stream":false}`, false},
		{`{"model":"m"}`, false},
		{`{bad json`, false}, // 解析不了 ⇒ 按非流式（给宽超时，宁松不误杀）
	}
	for _, c := range cases {
		if got := bodyStreams([]byte(c.body)); got != c.want {
			t.Errorf("bodyStreams(%q)=%v 期望 %v", c.body, got, c.want)
		}
	}
}
