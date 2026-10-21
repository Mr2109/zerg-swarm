// broker_http_test.go — T4b：Broker 的 **HTTP 回环面**（/health ＋ /v1/chat/completions 透传）。
//
// 判据六条（每条都带负控；都是**行为面**：从 127.0.0.1 发一条真 HTTP 请求看结果）：
//
//	① 只绑回环：`0.0.0.0:0` / `:0` / 具体网卡地址一律拒（负控：`127.0.0.1:0` 放行）；
//	② `/health` 200 ＋ 一行 JSON（后端名 / 模型名 / 上游是否接着 / 令牌面形态），且**不需要令牌**；
//	③ 透传：茧内带占位符 `zerg-broker` ⇒ 上游收到的是**真令牌**（判据：上游那一侧看到的
//	   Authorization 是真值、且**没见过**占位符）；
//	④ 占位符缺/错 ⇒ 401 且上游一次都没被碰到（fail-closed）；
//	⑤ 上游不可达 ⇒ 502 ＋ body 说清「broker 到不了上游」（✗ 不许伪装成 200）；
//	⑥ 回显擦除 ＋ 审计无真值：上游把 Authorization 原样回显也擦得掉，审计行里没有真值。
package cocoon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// constTestRealToken — 测试里的「真令牌」。★ 它只许出现在**上游那一侧**与 SecretResolver 里；
// 茧内那一侧（请求头 / 响应体 / 审计）一处都不许有它 —— 下面几条断言就是查这个。
const constTestRealToken = "sk-t4b-real-TOKEN-9f3a"

// upstreamRecorder — 假上游：记下每一条收到的请求（头 ＋ body），并可按需回显。
type upstreamRecorder struct {
	mu       sync.Mutex
	srv      *httptest.Server
	hits     int
	lastAuth string
	lastBody string
	echoAuth bool
	respBody string
	status   int
}

func newUpstreamRecorder(t *testing.T) *upstreamRecorder {
	t.Helper()
	rec := &upstreamRecorder{respBody: `{"id":"chatcmpl-t4b","choices":[{"message":{"role":"assistant","content":"pong"}}]}`, status: http.StatusOK}
	rec.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.hits++
		rec.lastAuth = r.Header.Get("Authorization")
		rec.lastBody = string(body)
		echo, resp, status := rec.echoAuth, rec.respBody, rec.status
		rec.mu.Unlock()
		if echo {
			// 上游把认证头原样回显（真实上游的报错页 / 调试页会这么干）⇒ broker 必须擦掉。
			resp = `{"echo":"` + strings.ReplaceAll(rec.authOf(), `"`, `\"`) + `"}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, resp)
	}))
	t.Cleanup(rec.srv.Close)
	return rec
}

func (u *upstreamRecorder) authOf() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastAuth
}

func (u *upstreamRecorder) hitsOf() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits
}

func (u *upstreamRecorder) bodyOf() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastBody
}

// httpRig — 跑起来的 broker（unix 面 ＋ HTTP 面一起 Serve）＋ 审计缓冲。
type httpRig struct {
	t     *testing.T
	b     *Broker
	addr  string // 监听地址（127.0.0.1:<port>）
	audit *lockedBuf
}

// newHTTPRig — 建 broker → 开两条面 → 起 Serve → 等 /health 真的能用。
//
// ★ ListenHTTP 必须在 Serve 之前：HTTP 面挂在 Serve 的同一个生命周期上（见 broker_http.go
// 的 startHTTP）。绑定本身在 ListenHTTP 就完成了，所以这里只需等服务 goroutine 起来。
func newHTTPRig(t *testing.T, opts BrokerHTTPOptions, secrets SecretResolver) *httpRig {
	t.Helper()
	if strings.TrimSpace(opts.Addr) == "" {
		opts.Addr = "127.0.0.1:0"
	}
	dir := shortDir(t)
	sock := filepath.Join(dir, "broker.sock")
	b := NewBroker(NewTokenManager())
	b.PeerProbe = strongProbe
	if secrets != nil {
		b.Secrets = secrets
	}
	audit := &lockedBuf{}
	b.Audit = audit
	if err := b.Listen(sock); err != nil {
		t.Fatalf("Listen(%q) 失败: %v", sock, err)
	}
	addr, err := b.ListenHTTP(opts)
	if err != nil {
		t.Fatalf("ListenHTTP(%+v) 失败: %v", opts, err)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		// ★ 防「悄悄绑到别处」：内核实际绑的地址必须是回环。
		t.Fatalf("HTTP 面监听地址 %q 不是回环", addr)
	}
	serveAsync(t, b)
	r := &httpRig{t: t, b: b, addr: addr, audit: audit}
	r.waitHealthy()
	return r
}

// serveAsync — 起 Serve 并在 cleanup 时收工（与 newRig 同口径：5s 内没退就判红）。
func serveAsync(t *testing.T, b *Broker) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- b.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errc:
			if err != nil {
				t.Errorf("Serve 收工时返回错误: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Serve 在 5s 内没有退出")
		}
	})
}

// waitHealthy — 等服务真起来（绑定在 ListenHTTP，但响应要等 Serve 的 goroutine）。
func (r *httpRig) waitHealthy() {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := r.get("/health", nil)
		if err == nil {
			resp.Body.Close()
			return
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	r.t.Fatalf("HTTP 面 3s 内没起来（最后一次: %v）", lastErr)
}

// url — `http://<addr><path>`。
func (r *httpRig) url(path string) string { return "http://" + r.addr + path }

// client — 不透代理的测试客户端（★ 父进程环境里的 HTTP_PROXY 不该影响判据）。
func httpTestClient() *http.Client {
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: nil},
	}
}

func (r *httpRig) get(path string, hdr map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, r.url(path), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return httpTestClient().Do(req)
}

// chat — 发一条 chat completions（hdr 为 nil 时不带认证头）。
func (r *httpRig) chat(hdr map[string]string, body string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, r.url("/v1/chat/completions"), bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := httpTestClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	out, rerr := io.ReadAll(resp.Body)
	return resp, out, rerr
}

// pointerSecret — 固定值的 SecretResolver。
func pointerSecret(v string) SecretResolver {
	return FuncSecretResolver(func(string) (string, error) { return v, nil })
}

// ── ① 只绑回环 ────────────────────────────────────────────────────────

func TestBrokerHTTPListenLoopbackOnly(t *testing.T) {
	// 负控：非回环一律拒 —— 且**不 bind**（拒在 bind 之前，不会占住端口）。
	for _, addr := range []string{"0.0.0.0:0", ":0", "127.0.0.1:0", "localhost:0", "192.0.2.7:0", "[::]:0", "::1:0"} {
		b := NewBroker(NewTokenManager())
		got, err := b.ListenHTTP(BrokerHTTPOptions{Addr: addr, Upstream: "http://127.0.0.1:9400"})
		wantOK := addr == "127.0.0.1:0" || addr == "localhost:0"
		if wantOK {
			if err != nil {
				t.Fatalf("%q 本该放行，却拒了: %v", addr, err)
			}
			if !strings.HasPrefix(got, "127.0.0.1:") && !strings.HasPrefix(got, "[::1]:") {
				t.Fatalf("%q 绑到了非回环地址 %q", addr, got)
			}
			_ = b.Close()
			continue
		}
		if err == nil {
			_ = b.Close()
			t.Fatalf("%q 本该被拒（只许绑回环），却放行了", addr)
		}
		if !strings.Contains(err.Error(), "回环") && !strings.Contains(err.Error(), "非法") {
			t.Fatalf("%q 的拒执理由该点名「回环/非法」，实得: %v", addr, err)
		}
	}
}

// TestBrokerHTTPAddrResolution — 三级真源（参数 ＞ env ＞ 内置）与缺省值的口径。
func TestBrokerHTTPAddrResolution(t *testing.T) {
	// 缺省：与茧内 base 同一真源（T4 的 ECONNREFUSED 就是这两处不一致来的）。
	// ★ 两个 env 名都要清：新名（`ZERG_BROKER_ADDR`）优先，宿主机上留着一个就会盖住这一格。
	t.Setenv(EnvBrokerAddr, "")
	t.Setenv(EnvBrokerHTTPAddr, "")
	if got, err := resolveBrokerHTTPAddr(""); err != nil || got != DefaultBrokerHTTPAddr {
		t.Fatalf("缺省地址 = %q (%v)，want %q", got, err, DefaultBrokerHTTPAddr)
	}
	// env 覆盖缺省
	t.Setenv(EnvBrokerHTTPAddr, "127.0.0.1:18082")
	if got, err := resolveBrokerHTTPAddr(""); err != nil || got != "127.0.0.1:18082" {
		t.Fatalf("env 覆盖 = %q (%v)", got, err)
	}
	// 参数盖过 env
	if got, err := resolveBrokerHTTPAddr("127.0.0.1:18083"); err != nil || got != "127.0.0.1:18083" {
		t.Fatalf("参数覆盖 = %q (%v)", got, err)
	}
	// 负控：env 里写非回环也拒（env 不是绕闸的后门）
	t.Setenv(EnvBrokerHTTPAddr, "0.0.0.0:18082")
	if _, err := resolveBrokerHTTPAddr(""); err == nil {
		t.Fatal("env 写 0.0.0.0 本该被拒")
	}
	// 上游三级真源：缺省 9400 / 参数优先 / 带 userinfo 拒
	t.Setenv(EnvAIUpstream, "")
	if got, err := resolveAIUpstream(""); err != nil || got != DefaultAIUpstream {
		t.Fatalf("上游缺省 = %q (%v)，want %q", got, err, DefaultAIUpstream)
	}
	t.Setenv(EnvAIUpstream, "http://127.0.0.1:9401")
	if got, err := resolveAIUpstream(""); err != nil || got != "http://127.0.0.1:9401" {
		t.Fatalf("上游 env 覆盖 = %q (%v)", got, err)
	}
	for _, bad := range []string{"http://tok:pw@127.0.0.1:9400", "ftp://127.0.0.1:9400", "http://127.0.0.1:9400?token=x", "127.0.0.1:9400"} {
		if _, err := resolveAIUpstream(bad); err == nil {
			t.Fatalf("上游 %q 本该被拒", bad)
		}
	}
}

// TestBrokerAddrSingleKnob — **一个旋钮同时改两处**（T4c 的核心判据）。
//
// 两条消费点各打一遍、逐条对齐：
//
//	① `ResolveBrokerAddr()` —— broker 的监听地址（`ListenHTTP` 用它 bind）；
//	② `BrokerAIBase()`      —— 茧内缺省 `ZERG_AI_BASE`（命令面用它注入）。
//
// 判据（缺一条这一单就等于没做）：
//
//	甲 缺省 = `DefaultBrokerHTTPAddr` = `127.0.0.1:8092`，且**不是** 8082（`zerg-core` 网关的正式端口）；
//	乙 新名 `ZERG_BROKER_ADDR` 一设，①②同时跟着动（同一份 host:port）；
//	丙 旧名 `ZERG_BROKER_HTTP_ADDR` 作为兼容输入，同样①②同时动（**不许**让两处漂）；
//	丁 两个都给且不同 ⇒ 新名赢，且①②仍一致（✗ 一处读新名、一处读旧名）；
//	戊 回环硬闸对新名同样有效（`0.0.0.0` / 空 host / 非 host:port 一律拒，①②一起拒）。
func TestBrokerAddrSingleKnob(t *testing.T) {
	// 甲：缺省值 + 「不是 8082」这条负控（写死字面量：8082 是**别人**的端口，本包不许再出现它）。
	t.Run("甲 缺省 8092 且两处一致", func(t *testing.T) {
		t.Setenv(EnvBrokerAddr, "")
		t.Setenv(EnvBrokerHTTPAddr, "")
		addr, src, err := ResolveBrokerAddr()
		if err != nil {
			t.Fatalf("ResolveBrokerAddr: %v", err)
		}
		// ★ 两个字面量分开判（`addr != A || addr != B` 在 A≠B 时是永假式 —— 写不出判力）。
		if DefaultBrokerHTTPAddr != "127.0.0.1:8092" {
			t.Fatalf("DefaultBrokerHTTPAddr = %q，want 127.0.0.1:8092（T4c 定的缺省）", DefaultBrokerHTTPAddr)
		}
		if addr != DefaultBrokerHTTPAddr {
			t.Fatalf("缺省监听 = %q，want %q", addr, DefaultBrokerHTTPAddr)
		}
		if addr == "127.0.0.1:8082" {
			t.Fatal("缺省还是 8082 —— 那是 zerg-core 网关的正式端口，开箱必 EADDRINUSE")
		}
		if src != BrokerAddrSourceDefault {
			t.Fatalf("来源 = %q，want %q", src, BrokerAddrSourceDefault)
		}
		base, bsrc, berr := BrokerAIBase()
		if berr != nil {
			t.Fatalf("BrokerAIBase: %v", berr)
		}
		if base != "http://"+addr {
			t.Fatalf("茧内 base = %q，want http://%s（与监听同一个真源）", base, addr)
		}
		if bsrc != src {
			t.Fatalf("两处来源不一致：监听 %q / base %q", src, bsrc)
		}
	})

	// 乙丙丁：新名 / 旧名 / 两者并存。
	cases := []struct {
		name    string
		newVal  string
		oldVal  string
		want    string
		wantSrc string
	}{
		{"乙 新名同时改两处", "127.0.0.1:18092", "", "127.0.0.1:18092", BrokerAddrSourceEnvNew},
		{"丙 旧名（兼容）同样同时改两处", "", "127.0.0.1:18093", "127.0.0.1:18093", BrokerAddrSourceEnvOld},
		{"丁 两个都给 ⇒ 新名赢且两处一致", "127.0.0.1:18094", "127.0.0.1:18095", "127.0.0.1:18094", BrokerAddrSourceEnvNew},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(EnvBrokerAddr, c.newVal)
			t.Setenv(EnvBrokerHTTPAddr, c.oldVal)
			addr, src, err := ResolveBrokerAddr()
			if err != nil {
				t.Fatalf("ResolveBrokerAddr: %v", err)
			}
			if addr != c.want {
				t.Fatalf("监听 = %q，want %q", addr, c.want)
			}
			if src != c.wantSrc {
				t.Fatalf("来源 = %q，want %q", src, c.wantSrc)
			}
			base, _, berr := BrokerAIBase()
			if berr != nil {
				t.Fatalf("BrokerAIBase: %v", berr)
			}
			// ★ 这条是「两处不漂」的机器判据：茧内 base 的 host:port 逐字等于监听地址。
			if got := strings.TrimPrefix(base, "http://"); got != addr {
				t.Fatalf("茧内 base 的 host:port = %q ≠ 监听 %q —— 两处漂了", got, addr)
			}
		})
	}

	// 戊：回环硬闸对新名同样生效；①②**一起**拒（不许一处拒一处放行）。
	t.Run("戊 回环硬闸对新名同样有效", func(t *testing.T) {
		for _, bad := range []string{"0.0.0.0:18092", ":18092", "::1", "127.0.0.1", "http://127.0.0.1:18092"} {
			t.Setenv(EnvBrokerAddr, bad)
			t.Setenv(EnvBrokerHTTPAddr, "")
			if _, _, err := ResolveBrokerAddr(); err == nil {
				t.Fatalf("新名写 %q 本该被拒", bad)
			}
			if _, _, err := BrokerAIBase(); err == nil {
				t.Fatalf("新名写 %q：监听拒了、base 却放行 ⇒ 两处不同判", bad)
			}
		}
	})
}

// ── ② /health ─────────────────────────────────────────────────────────

func TestBrokerHTTPHealth(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL, Backend: "nsjail", Model: "Nemotron-3.5"}, pointerSecret(constTestRealToken))

	// ★ 不带任何令牌就能读（茧内第一句 `curl $ZERG_AI_BASE/health` 就是这个形态）。
	resp, err := r.get("/health", nil)
	if err != nil {
		t.Fatalf("GET /health 失败: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health 状态 = %d，want 200（body=%s）", resp.StatusCode, raw)
	}
	if lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n"); len(lines) != 1 {
		t.Fatalf("/health 该是**一行** JSON，实得 %d 行: %q", len(lines), raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("/health 不是合法 JSON: %q (%v)", raw, err)
	}
	if ok, _ := got["ok"].(bool); !ok {
		t.Fatalf("/health ok 位 = %#v", got["ok"])
	}
	if got["backend"] != "nsjail" {
		t.Fatalf("backend = %#v，want nsjail", got["backend"])
	}
	if got["model"] != "Nemotron-3.5" {
		t.Fatalf("model = %#v", got["model"])
	}
	if got["upstream"] != rec.srv.URL {
		t.Fatalf("upstream = %#v，want %q", got["upstream"], rec.srv.URL)
	}
	if uc, _ := got["upstream_configured"].(bool); !uc {
		t.Fatalf("upstream_configured = %#v，want true", got["upstream_configured"])
	}
	if tm, _ := got["token_mode"].(string); !strings.HasPrefix(tm, "broker:") {
		t.Fatalf("token_mode = %#v，want broker:<secret>", got["token_mode"])
	}
	// ★ 真值一个字都不许出现在 /health 里。
	if strings.Contains(string(raw), constTestRealToken) {
		t.Fatalf("/health 泄漏了真令牌: %s", raw)
	}
	// ★ /health 不许顺手拨上游（探针拖死 broker 的那条路）。
	if n := rec.hitsOf(); n != 0 {
		t.Fatalf("/health 不该拨上游，却打了 %d 次", n)
	}
}

// ── ③ 透传：茧内占位符 ⇒ 上游真令牌 ──────────────────────────────────

func TestBrokerHTTPProxyInjectsTokenOnlyUpstream(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))

	body := `{"model":"m","messages":[{"role":"user","content":"ping"}]}`
	// 茧内那一侧：只有协议占位符（与 ZERG_AI_TOKEN_FILE 的内容一致）。
	resp, out, err := r.chat(map[string]string{"Authorization": "Bearer " + AITokenPlaceholder}, body)
	if err != nil {
		t.Fatalf("POST /v1/chat/completions 失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态 = %d，want 200（body=%s）", resp.StatusCode, out)
	}
	if !strings.Contains(string(out), "chatcmpl-t4b") {
		t.Fatalf("上游回复没透传回来: %s", out)
	}
	// 上游看到的是**真令牌**，且**没**看到占位符。
	if auth := rec.authOf(); auth != "Bearer "+constTestRealToken {
		t.Fatalf("上游收到的 Authorization = %q，want 真令牌", auth)
	}
	if strings.Contains(rec.authOf(), AITokenPlaceholder) {
		t.Fatalf("占位符被发到了上游（%q）—— 换值这一跳没生效", rec.authOf())
	}
	// 请求体逐字透传。
	if rec.bodyOf() != body {
		t.Fatalf("上游收到的 body = %q，want %q", rec.bodyOf(), body)
	}
	// 回给茧内的 body 里没有真值（上游这次没回显，这里判的是常规路径）。
	if strings.Contains(string(out), constTestRealToken) {
		t.Fatalf("响应体泄漏真令牌: %s", out)
	}
	// 审计：一行 op=ai.request，无真值、无占位符。
	lines := r.audit.Lines()
	if len(lines) != 1 {
		t.Fatalf("审计该恰一行，实得 %d: %v", len(lines), lines)
	}
	for _, ln := range lines {
		if strings.Contains(ln, constTestRealToken) || strings.Contains(ln, AITokenPlaceholder) {
			t.Fatalf("审计行里出现了令牌材料: %s", ln)
		}
		var a AuditLine
		if err := json.Unmarshal([]byte(ln), &a); err != nil {
			t.Fatalf("审计行不是合法 JSON: %q (%v)", ln, err)
		}
		if a.Op != aiAuditOp || !a.OK {
			t.Fatalf("审计行 = %+v，want op=%s ok=true", a, aiAuditOp)
		}
	}
}

// TestBrokerHTTPProxyNoResolverForwardsUnauthenticated — 没有 SecretResolver 时**不发明凭据**
// （本机上游免认证档）：上游收不到任何认证头。
func TestBrokerHTTPProxyNoResolverForwardsUnauthenticated(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, nil)
	resp, out, err := r.chat(map[string]string{"Authorization": "Bearer " + AITokenPlaceholder}, `{"model":"m"}`)
	if err != nil {
		t.Fatalf("透传失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("透传失败: status=%d body=%s", resp.StatusCode, out)
	}
	if auth := rec.authOf(); auth != "" {
		t.Fatalf("没有解析器时不该给上游加认证头，实得 %q", auth)
	}
}

// ── ④ 占位符缺/错 ⇒ 401，且上游一次都没被碰 ──────────────────────────

func TestBrokerHTTPProxyRequiresPlaceholder(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))

	cases := map[string]map[string]string{
		"没带认证头":       nil,
		"带了个别的令牌":     {"Authorization": "Bearer not-the-placeholder"},
		"方案不是 Bearer": {"Authorization": AITokenPlaceholder},
	}
	for name, hdr := range cases {
		resp, out, err := r.chat(hdr, `{"model":"m"}`)
		if err != nil {
			t.Fatalf("%s: 请求失败: %v", name, err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s: 状态 = %d，want 401（body=%s）", name, resp.StatusCode, out)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("%s: 401 body 不是 JSON: %q", name, out)
		}
		if got["code"] != CodeUnauthorized {
			t.Fatalf("%s: code = %#v，want %s", name, got["code"], CodeUnauthorized)
		}
	}
	if n := rec.hitsOf(); n != 0 {
		t.Fatalf("占位符不对时上游一次都不该被碰，实得 %d 次", n)
	}
}

// ── ⑤ 上游不可达 ⇒ 502 ＋ 说清楚 ──────────────────────────────────────

func TestBrokerHTTPProxyUpstreamUnreachable(t *testing.T) {
	// 拿一个**确定没人听**的端口：先 bind 再关（比硬编码端口稳）。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口失败: %v", err)
	}
	dead := "http://" + ln.Addr().String()
	_ = ln.Close()

	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: dead}, pointerSecret(constTestRealToken))
	resp, out, err := r.chat(map[string]string{"Authorization": "Bearer " + AITokenPlaceholder}, `{"model":"m"}`)
	if err != nil {
		t.Fatalf("请求失败（broker 本身该活着）: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("上游不可达该 502，实得 %d（body=%s）", resp.StatusCode, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("502 body 不是 JSON: %q", out)
	}
	if got["code"] != "upstream_unreachable" {
		t.Fatalf("code = %#v，want upstream_unreachable", got["code"])
	}
	// ★ 「说清楚」的机器判据：body 里点名上游地址，并说明断在 broker→上游那一段。
	if up, _ := got["upstream"].(string); up != dead {
		t.Fatalf("body 该点名上游 %q，实得 %#v", dead, got["upstream"])
	}
	if msg, _ := got["error"].(string); !strings.Contains(msg, "上游") {
		t.Fatalf("body 该说清是「到不了上游」，实得 %q", msg)
	}
	// ★ 负控的反面：它**不是** 200，也不是 500（500 会被读成「broker 自己坏了」）。
	if strings.Contains(string(out), constTestRealToken) {
		t.Fatalf("502 body 泄漏真令牌: %s", out)
	}
}

// ── ⑥ 回显擦除 ＋ 未知路径 ────────────────────────────────────────────

func TestBrokerHTTPScrubsUpstreamEcho(t *testing.T) {
	rec := newUpstreamRecorder(t)
	rec.echoAuth = true
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))
	resp, out, err := r.chat(map[string]string{"Authorization": "Bearer " + AITokenPlaceholder}, `{"model":"m"}`)
	if err != nil {
		t.Fatalf("透传失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("透传失败: status=%d body=%s", resp.StatusCode, out)
	}
	if strings.Contains(string(out), constTestRealToken) {
		t.Fatalf("上游回显的真令牌没被擦掉: %s", out)
	}
	if !strings.Contains(string(out), ScrubPlaceholder) {
		t.Fatalf("擦除后该出现 %s，实得: %s", ScrubPlaceholder, out)
	}
}

func TestBrokerHTTPUnknownPath(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))
	resp, err := r.get("/v1/models", nil)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("未接的路径该 404，实得 %d（通配透传等于把可敲的上游面交给上游自己决定）", resp.StatusCode)
	}
	if n := rec.hitsOf(); n != 0 {
		t.Fatalf("404 路径不该转上游，实得 %d 次", n)
	}
}

// TestBrokerHTTPCloseStopsFace — 负控（与集成验收的 ③ 同口径，只是放在进程内）：
// Close 之后同一个地址**连不上**（不是回 200/502）。
func TestBrokerHTTPCloseStopsFace(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))
	addr := r.addr
	if _, err := r.get("/health", nil); err != nil {
		t.Fatalf("收工前 /health 该通: %v", err)
	}
	if err := r.b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	c := httpTestClient()
	if resp, err := c.Get("http://" + addr + "/health"); err == nil {
		resp.Body.Close()
		t.Fatalf("Close 之后 /health 仍然可达（status=%d）—— 这就是「假 200」的那一形态", resp.StatusCode)
	}
	if got := r.b.HTTPAddr(); got != "" {
		t.Fatalf("Close 之后 HTTPAddr 该为空，实得 %q", got)
	}
	if _, err := r.b.ListenHTTP(BrokerHTTPOptions{Addr: "127.0.0.1:0", Upstream: rec.srv.URL}); err == nil {
		t.Fatal("Close 之后不该还能 ListenHTTP")
	}
}

// TestBrokerHTTPDoubleListenRefused — 重复 ListenHTTP 拒（两条面一进程一套，别静默覆盖）。
func TestBrokerHTTPDoubleListenRefused(t *testing.T) {
	rec := newUpstreamRecorder(t)
	r := newHTTPRig(t, BrokerHTTPOptions{Upstream: rec.srv.URL}, pointerSecret(constTestRealToken))
	if _, err := r.b.ListenHTTP(BrokerHTTPOptions{Addr: "127.0.0.1:0", Upstream: rec.srv.URL}); err == nil {
		t.Fatal("第二次 ListenHTTP 本该被拒")
	} else if !strings.Contains(fmt.Sprint(err), "监听") {
		t.Fatalf("拒执理由该点名「已在监听」，实得: %v", err)
	}
}
