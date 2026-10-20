// broker_http.go — 茧内 AI 面：broker 的 **HTTP 回环监听**（`/health` ＋ `POST /v1/chat/completions` 透传）。
//
// ★ 为什么要有这一面（T4b 的起因，T4 实测）：`Broker` 原来只有 unix socket 监听，而茧内拿到的
// `ZERG_AI_BASE` 必须是 http(s) 回环 URL（`CheckAIUpstream` 的硬校验）⇒ 茧内
// `curl $ZERG_AI_BASE/health` 到不了 broker（实测 `dial 127.0.0.1:8082` ⇒ ECONNREFUSED）。
// 本件补的就是那一段，形状是「**茧内只用 HTTP，真令牌只在 broker 进程内**」：
//
//	茧内（占位符 zerg-broker） → 127.0.0.1:<broker 端口> → broker（换真令牌） → 真上游
//
// 四条不变量（测试逐条打，改这里先读它们）：
//
//	① **只绑回环**：`ListenHTTP` 拒一切非回环 host（`0.0.0.0` / `::` / 空 host / 具体网卡地址），
//	   并在 bind 之后**现读内核实际绑的地址**再判一次（不信任参数校验）；请求处理时按
//	   `RemoteAddr` 第三条复核。⇒ 「broker 绑到 0.0.0.0」这件事在三个位置都做不成。
//	② **令牌单向、只出站**：茧内只有占位符（`AITokenPlaceholder`），真值由 broker 的
//	   `SecretResolver` 取出、只放进**发往上游**的请求头；响应体再走 `ScrubResponse`
//	   （上游回显也擦得掉）。broker → 茧内这一跳永远不带真值。
//	③ **不发明凭据**：没有 `SecretResolver` ⇒ 不加认证头（本机上游常见免认证）；
//	   有解析器但解析失败 ⇒ 502（**不裸发**，fail-closed）。
//	④ **错误面说实话**：上游不可达 ⇒ **502** ＋ 一行 JSON 说清「broker 到不了上游 <url>」，
//	   ✗ 不把连接失败伪装成 200，✗ 也不回 5xx 却让人以为是茧自己的错。
//
// ★ 与 unix 面的关系：两条面共用 `Broker` 的配置（`Tokens` / `Ops` / `Secrets` / `ScrubSecrets`
// / `Audit`）与**同一个生命周期**（`Serve` 起、`Close` 收）。没调 `ListenHTTP` 时 unix 面行为
// 逐字不变 —— 老调用方零改动。
//
// ★ 未接的面（照实登记）：回环 TCP 拿不到对端 uid（Linux 的 unix 面才有 peer 判据）⇒ 本面
// 的对端身份强度按 `none` 记，「谁在用 broker」只能靠审计里的 inst 头（客户端自报，可伪造）。
// 真要「每茧独立凭据」得把这面改成 per-cocoon 监听（或 unix socket ＋ 每茧独立 uid），
// 那是契约面变更，本版未做。
package cocoon

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBrokerHTTPAddr — 茧内 AI 面的缺省监听地址。★ 与 `cocoonDefaultAIUpstream`
	// （命令面注入给茧内的 `ZERG_AI_BASE`）必须同一个值：两处各写一份就等于让茧去敲一扇
	// 没人开的门（T4 的 ECONNREFUSED 就是这么来的）。
	DefaultBrokerHTTPAddr = "127.0.0.1:8082"
	// EnvBrokerHTTPAddr — 覆盖监听地址（宿主机 env；茧内不需要也不该看到它）。
	EnvBrokerHTTPAddr = "ZERG_BROKER_HTTP_ADDR"
	// EnvAIUpstream — 真上游地址（宿主机 env；茧内看到的是 `ZERG_AI_BASE` = broker 自己）。
	EnvAIUpstream = "ZERG_AI_UPSTREAM"
	// DefaultAIUpstream — 缺省真上游（本机模型服务）。
	DefaultAIUpstream = "http://127.0.0.1:9400"
	// DefaultAIUpstreamSecret — 从 `SecretResolver` 取真令牌用的键（`secret://` 同一套名字）。
	DefaultAIUpstreamSecret = "ai.upstream"
	// DefaultAIProxyTimeout — 单次透传的上限（推理可能慢，但必须有上限）。
	DefaultAIProxyTimeout = 300 * time.Second
	// maxAIBodyBytes — 透传请求体的上限（8 MiB：够放长上下文，又不至于让一条请求吃光内存）。
	maxAIBodyBytes = 8 << 20

	// aiHealthPath / aiChatPath — 本面暴露的两条路径（其余一律 404，不做「通配透传」：
	// 通配等于把茧内能敲的上游面交给上游自己决定）。
	aiHealthPath = "/health"
	aiChatPath   = "/v1/chat/completions"

	// aiInstHeader — 审计用的茧 id（客户端自报，先收窄再落盘；✗ 不当身份凭据）。
	aiInstHeader = "X-Zerg-Inst"
	// aiAuditOp — 本面在审计里的 op 名（与 unix 面的 `ai.request` 同一格，别造第二个名字）。
	aiAuditOp = "ai.request"
	// aiDefaultBackend — `/health` 里报的后端名（空 ⇒ "cocoon"）。
	aiDefaultBackend = "cocoon"
)

var (
	// ErrHTTPNotLoopback — 监听地址不是回环（含空 host：`:8082` 等于绑全部网卡）。
	ErrHTTPNotLoopback = errors.New("cocoon: broker HTTP 面只许绑回环地址（127.0.0.0/8 · ::1 · localhost）")
	// ErrHTTPBadAddr — 监听地址形态非法（不是 host:port）。
	ErrHTTPBadAddr = errors.New("cocoon: broker HTTP 面监听地址非法（要 host:port）")
	// ErrHTTPBadUpstream — 上游 base 形态非法（非 http(s) / 带 userinfo / 带 query）。
	ErrHTTPBadUpstream = errors.New("cocoon: AI 上游 base 形态非法（要 http(s)://host[:port][/path]，不带 userinfo/query）")
	// ErrHTTPNilBroker — 零值 Broker 上开会话面。
	ErrHTTPNilBroker = errors.New("cocoon: broker 为空（HTTP 面无从挂载）")
)

// BrokerHTTPOptions — 茧内 AI 回环面的配置。零值 ⇒ 全部取缺省（env ＞ 内置常量）。
type BrokerHTTPOptions struct {
	// Addr — 监听地址。空 ⇒ `ZERG_BROKER_HTTP_ADDR` ⇒ `DefaultBrokerHTTPAddr`。
	Addr string
	// Upstream — 真上游 base。空 ⇒ `ZERG_AI_UPSTREAM` ⇒ `DefaultAIUpstream`。
	Upstream string
	// Backend — `/health` 里报的后端名（空 ⇒ "cocoon"）。
	Backend string
	// Model — `/health` 里报的模型名（空 ⇒ `DefaultCocoonAIModel`）。
	Model string
	// SecretName — `SecretResolver` 的键（空 ⇒ `DefaultAIUpstreamSecret`）。
	SecretName string
	// AuthHeader / AuthScheme — 出站认证头（空 ⇒ `Authorization` / `Bearer`）。
	AuthHeader string
	AuthScheme string
	// Timeout — 单次透传上限（0 ⇒ `DefaultAIProxyTimeout`）。
	Timeout time.Duration
}

// brokerHTTPState — 已解析的会话面（`ListenHTTP` 一次算好，之后只读）。
type brokerHTTPState struct {
	addr       string
	upstream   string
	backend    string
	model      string
	secretName string
	authHeader string
	authScheme string
	client     *http.Client
}

// resolved — 把零值选项补成一份可执行的配置（env 与内置常量在这里收口）。
func (o BrokerHTTPOptions) resolved() (brokerHTTPState, error) {
	addr, err := resolveBrokerHTTPAddr(o.Addr)
	if err != nil {
		return brokerHTTPState{}, err
	}
	upstream, err := resolveAIUpstream(o.Upstream)
	if err != nil {
		return brokerHTTPState{}, err
	}
	st := brokerHTTPState{
		addr:       addr,
		upstream:   upstream,
		backend:    strings.TrimSpace(o.Backend),
		model:      strings.TrimSpace(o.Model),
		secretName: strings.TrimSpace(o.SecretName),
		authHeader: strings.TrimSpace(o.AuthHeader),
		authScheme: strings.TrimSpace(o.AuthScheme),
	}
	if st.backend == "" {
		st.backend = aiDefaultBackend
	}
	if st.model == "" {
		st.model = DefaultCocoonAIModel
	}
	if st.secretName == "" {
		st.secretName = DefaultAIUpstreamSecret
	}
	if st.authHeader == "" {
		st.authHeader = "Authorization"
	}
	if st.authScheme == "" {
		st.authScheme = "Bearer"
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = DefaultAIProxyTimeout
	}
	st.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// ★ Proxy=nil：继承 `HTTP_PROXY` 会把**带真令牌**的请求交给环境里那台代理
			// —— 那是本件最不该出的事（令牌只在 broker 进程内，不许再出门一次）。
			Proxy:               nil,
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			MaxIdleConns:        8,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	return st, nil
}

// resolveBrokerHTTPAddr — 监听地址的三级真源 ＋ **回环硬闸**。
//
// ★ 空 host（`:8082`）也拒：它的语义是「绑全部网卡」，而 Go 的 `net.Listen("tcp", ":8082")`
// 会**成功** —— 不判这一条，「只绑回环」就只是注释。
func resolveBrokerHTTPAddr(addr string) (string, error) {
	s := strings.TrimSpace(addr)
	if s == "" {
		s = strings.TrimSpace(os.Getenv(EnvBrokerHTTPAddr))
	}
	if s == "" {
		s = DefaultBrokerHTTPAddr
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrHTTPBadAddr, s)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("%w: %q（缺端口）", ErrHTTPBadAddr, s)
	}
	if host == "" || !isLoopbackHost(host) {
		return "", fmt.Errorf("%w: host=%q（地址 %q）", ErrHTTPNotLoopback, host, s)
	}
	return net.JoinHostPort(host, port), nil
}

// resolveAIUpstream — 真上游的三级真源（参数 ＞ env ＞ 缺省）＋ 形态校验。
//
// ★ 与 `CheckAIUpstream` 的分工：那个口管「**茧内**拿到的 base」（必须回环，茧内不许把流量
// 放出宿主）；本函数管「broker 自己拨的上游」（可以是非回环的真模型服务）。两者共用
// 「不带 userinfo / 不带 query」这条 —— 凭据与令牌都不许藏在 URL 里。
func resolveAIUpstream(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = strings.TrimSpace(os.Getenv(EnvAIUpstream))
	}
	if s == "" {
		s = DefaultAIUpstream
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%w（不回显原值）", ErrHTTPBadUpstream)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w（scheme=%q）", ErrHTTPBadUpstream, u.Scheme)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w（不带 userinfo：凭据走 SecretResolver）", ErrHTTPBadUpstream)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w（不带 query/fragment）", ErrHTTPBadUpstream)
	}
	return u.Scheme + "://" + u.Host + strings.TrimSuffix(u.Path, "/"), nil
}

// ListenHTTP — 开茧内 AI 的 HTTP 回环面，返回**实际**监听地址（`127.0.0.1:0` 时是内核挑的端口）。
//
// ★ 三步都 fail-closed：配置算不出来 ⇒ 不 bind；bind 之后现读内核实际绑的 IP 再判一次；
// 端口被占（例如与 zerg-core 网关同抢 8082）⇒ 原样报出 `bind:` 那句，✗ 不静默换端口
// （换了端口，茧内的 `ZERG_AI_BASE` 就指向虚空 —— 那正是 T4 的病）。
func (b *Broker) ListenHTTP(opts BrokerHTTPOptions) (string, error) {
	if b == nil {
		return "", ErrHTTPNilBroker
	}
	st, err := opts.resolved()
	if err != nil {
		return "", err
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return "", ErrNotListening
	}
	if b.http != nil {
		b.mu.Unlock()
		return "", ErrBrokerAlreadyListening
	}
	b.mu.Unlock()

	ln, err := net.Listen("tcp", st.addr)
	if err != nil {
		return "", fmt.Errorf("cocoon: 绑定 broker HTTP 面 %q 失败: %w", st.addr, err)
	}
	// ★ 自检读的是**内核实际绑的**地址（不信任参数校验：`localhost` 的解析、双栈行为都在这之后）。
	if err := verifyLoopbackListener(ln); err != nil {
		_ = ln.Close()
		return "", err
	}

	srv := &http.Server{
		Handler:           b.aiHandler(st),
		ReadHeaderTimeout: 10 * time.Second,
		// 读体上限在本层自己判（要报 413 而不是让 http 库把连接直接掐了）。
		MaxHeaderBytes: 64 << 10,
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		_ = srv.Close()
		_ = ln.Close()
		return "", ErrNotListening
	}
	b.http = &httpFace{state: st, ln: ln, srv: srv}
	b.HTTPOptions = opts
	actual := ln.Addr().String()
	b.mu.Unlock()
	return actual, nil
}

// verifyLoopbackListener — 「只绑回环」的机器判据：看 Listener **实际**绑到的 IP。
//
// ★ 为什么不能只靠参数校验：`127.0.0.1` 与 `localhost` 都对，但 `localhost` 在有的机器上
// 先解析成 `::1`、在另一些上解析成 `127.0.0.1`；真出错时我们要的是「内核绑的是不是回环」，
// 而不是「我们当初想绑什么」。
func verifyLoopbackListener(ln net.Listener) error {
	if ta, ok := ln.Addr().(*net.TCPAddr); ok && ta.IP != nil && !ta.IP.IsLoopback() {
		return fmt.Errorf("%w: 内核实际绑定 %s", ErrHTTPNotLoopback, ta.String())
	}
	return nil
}

// HTTPAddr — 当前 HTTP 面的监听地址（未监听 ⇒ 空串）。
func (b *Broker) HTTPAddr() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.http == nil {
		return ""
	}
	return b.http.ln.Addr().String()
}

// aiHandler — 本面唯一的路由表：两条路径，别的一张都不开。
func (b *Broker) aiHandler(st brokerHTTPState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(aiHealthPath, func(w http.ResponseWriter, r *http.Request) {
		if !aiLoopbackPeer(r) {
			writeAIJSON(w, http.StatusForbidden, aiErrBody("forbidden"))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeAIJSON(w, http.StatusMethodNotAllowed, aiErrBody("method_not_allowed"))
			return
		}
		// ★ 一行 JSON，**没有真值**：后端名 / 模型名 / 上游是否接着 / 令牌面在哪。
		//   「是否接上游」= 配置里有没有可拨的上游（不是探测结果）：/health 不许拨号
		//   —— 一个健康探针顺手把上游打一顿，会在上游慢的时候把 broker 一起拖死。
		writeAIJSON(w, http.StatusOK, struct {
			OK                 bool   `json:"ok"`
			Backend            string `json:"backend"`
			Model              string `json:"model"`
			Upstream           string `json:"upstream"`
			UpstreamConfigured bool   `json:"upstream_configured"`
			TokenMode          string `json:"token_mode"`
		}{
			OK:                 true,
			Backend:            st.backend,
			Model:              st.model,
			Upstream:           st.upstream,
			UpstreamConfigured: st.upstream != "",
			TokenMode:          b.aiTokenMode(st),
		})
	})
	mux.HandleFunc(aiChatPath, b.aiChatHandler(st))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeAIJSON(w, http.StatusNotFound, aiErrBody("not_found"))
	})
	return mux
}

// aiTokenMode — `/health` 里「令牌面在哪」那一格：没有真值，只有形态。
func (b *Broker) aiTokenMode(st brokerHTTPState) string {
	if b.Secrets == nil {
		return "none"
	}
	return "broker:" + st.secretName
}

// aiChatHandler — `POST /v1/chat/completions`：验占位符 → 换真令牌 → 透传 → 擦除回写。
func (b *Broker) aiChatHandler(st brokerHTTPState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := b.now()
		inst := aiInst(r)
		if !aiLoopbackPeer(r) {
			writeAIJSON(w, http.StatusForbidden, aiErrBody("forbidden"))
			b.auditHTTP(start, inst, false)
			return
		}
		if r.Method != http.MethodPost {
			writeAIJSON(w, http.StatusMethodNotAllowed, aiErrBody("method_not_allowed"))
			b.auditHTTP(start, inst, false)
			return
		}
		// ① 茧内只能拿占位符出门（真令牌在茧内根本不存在）。
		if !aiPlaceholderPresent(r) {
			writeAIJSON(w, http.StatusUnauthorized, aiErrBody(CodeUnauthorized))
			b.auditHTTP(start, inst, false)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxAIBodyBytes+1))
		if err != nil {
			writeAIJSON(w, http.StatusBadRequest, aiErrBody(CodeBadRequest))
			b.auditHTTP(start, inst, false)
			return
		}
		if len(body) > maxAIBodyBytes {
			writeAIJSON(w, http.StatusRequestEntityTooLarge, aiErrBody("payload_too_large"))
			b.auditHTTP(start, inst, false)
			return
		}

		// ② 换真令牌（只在**出站**这一跳；茧内那一跳永远只有占位符）。
		token, terr := b.aiUpstreamToken(st)
		if terr != nil {
			// fail-closed：有解析器却取不到真值 ⇒ 不裸发（裸发等于把「未认证」当常态）。
			if b.ErrorHook != nil {
				b.ErrorHook(inst, aiAuditOp, terr)
			}
			writeAIJSON(w, http.StatusBadGateway, struct {
				OK     bool   `json:"ok"`
				Code   string `json:"code"`
				Error  string `json:"error"`
				Detail string `json:"detail"`
			}{
				OK: false, Code: "upstream_auth_unavailable",
				Error:  "broker 拿不到上游令牌（SecretResolver 解析失败）—— 请求未发出",
				Detail: "secret=" + st.secretName,
			})
			b.auditHTTP(start, inst, false)
			return
		}

		upstreamURL := st.upstream + aiChatPath
		req, rerr := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
		if rerr != nil {
			writeAIJSON(w, http.StatusInternalServerError, aiErrBody(CodeInternal))
			b.auditHTTP(start, inst, false)
			return
		}
		copyAIHeaders(req.Header, r.Header)
		// ★ 认证头由 broker **覆盖**（客户端带来的一律丢掉）：茧内自带的 Authorization
		//   只可能是占位符，转发它等于把「谁认证」交给茧。
		req.Header.Del("Authorization")
		req.Header.Del("Proxy-Authorization")
		req.Header.Del("X-Api-Key")
		req.Header.Del("Api-Key")
		req.Header.Del("X-Auth-Token")
		if token != "" {
			v := token
			if scheme := st.authScheme; scheme != "" {
				v = scheme + " " + token
			}
			req.Header.Set(st.authHeader, v)
		}

		// ③ 拨上游。不可达 ⇒ 502 ＋ 说清是「broker 到不了上游」。
		resp, derr := st.client.Do(req)
		if derr != nil {
			if b.ErrorHook != nil {
				b.ErrorHook(inst, aiAuditOp, derr)
			}
			writeAIJSON(w, http.StatusBadGateway, struct {
				OK       bool   `json:"ok"`
				Code     string `json:"code"`
				Error    string `json:"error"`
				Upstream string `json:"upstream"`
				Detail   string `json:"detail"`
			}{
				OK: false, Code: "upstream_unreachable",
				Error:    "broker 到不了 AI 上游（茧内到 broker 这一跳是通的；断在 broker → 上游）",
				Upstream: st.upstream,
				// ★ 只带拨号错误本身：它可能含 URL（不含令牌 —— URL 里不许有凭据，见 resolveAIUpstream）。
				Detail: derr.Error(),
			})
			b.auditHTTP(start, inst, false)
			return
		}
		defer resp.Body.Close()

		out, rerr := io.ReadAll(io.LimitReader(resp.Body, maxAIBodyBytes+1))
		if rerr != nil {
			writeAIJSON(w, http.StatusBadGateway, aiErrBody("upstream_read_failed"))
			b.auditHTTP(start, inst, false)
			return
		}
		// ④ 回显擦除：上游把令牌原样回显（报错页 / 调试页）也擦得掉。
		out = ScrubResponse(out, append(append([]string{}, b.ScrubSecrets...), token))

		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(out)
		b.auditHTTP(start, inst, resp.StatusCode < 400)
	}
}

// aiUpstreamToken — 取真令牌。没有解析器 ⇒ 空串（本机上游免认证档）；解析失败 ⇒ 错。
func (b *Broker) aiUpstreamToken(st brokerHTTPState) (string, error) {
	if b.Secrets == nil {
		return "", nil
	}
	return resolveSecret(b.Secrets, st.secretName)
}

// copyAIHeaders — 透传**非认证类**请求头（hop-by-hop 一律丢：它们描述的是那一跳的连接，
// 而不是这次请求）。
func copyAIHeaders(dst, src http.Header) {
	for k, vs := range src {
		if hopByHopHeader(k) {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	// Content-Length 由 http 库按 body 重算（客户端给的那个可能与我们的 body 不等）。
	dst.Del("Content-Length")
}

// hopByHopHeader — RFC 7230 的逐跳头 ＋ 明显与这一跳绑定的几个。
func hopByHopHeader(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "host", "content-length":
		return true
	}
	return false
}

// aiPlaceholderPresent — 请求有没有带**协议占位符**（`Authorization: Bearer zerg-broker`
// 或 `X-Auth-Token: zerg-broker`）。
//
// ★ 常量时间比较：占位符本身不是秘密，但这条路径将来可能承载真令牌 ⇒ 现在就用比较安全的写法。
func aiPlaceholderPresent(r *http.Request) bool {
	var got string
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		if parts := strings.SplitN(v, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			got = strings.TrimSpace(parts[1])
		}
	}
	if got == "" {
		got = strings.TrimSpace(r.Header.Get("X-Auth-Token"))
	}
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(AITokenPlaceholder)) == 1
}

// aiLoopbackPeer — 对端复核（第三条闸）：只收回环来源。
//
// ★ 为什么明明绑了回环还要查一遍：`ListenHTTP` 的地址可能被 env 改成 `localhost`，
// 而它在一个怪异环境里解析到非回环（以及将来可能有人加反代）⇒ 这一条让「不是本机来的」
// 无论如何都进不来。
func aiLoopbackPeer(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// aiInst — 审计用的茧 id（客户端自报，收窄后落盘；缺失时用固定标签，✗ 不冒充某个茧）。
func aiInst(r *http.Request) string {
	if r != nil {
		if v := auditField(strings.TrimSpace(r.Header.Get(aiInstHeader))); v != "" {
			return v
		}
	}
	return "cocoon-http"
}

// auditHTTP — HTTP 面的一行审计（字段表与 unix 面同一份：无令牌、无 args、无结果）。
//
// ★ 对端强度按 `none` 记：回环 TCP 拿不到 uid —— 记 `strong` 会是本件最糟的一句假话。
func (b *Broker) auditHTTP(start time.Time, inst string, ok bool) {
	b.audit(start, inst, aiAuditOp, ok, PeerIdentityRecord{Kind: "tcp-loopback", Confidence: PeerIdentityNone})
}

// aiErrBody — 错误体的统一形状（`{ok:false, code}`，与 unix 帧的码表同一套字串）。
func aiErrBody(code string) map[string]any {
	return map[string]any{"ok": false, "code": code}
}

// writeAIJSON — 一行 JSON ＋ 换行（茧内 `curl` 出来就是一行，便于对拍）。
func writeAIJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		body = []byte(`{"ok":false,"code":"internal"}`)
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// ── 与 Broker 生命周期的接缝 ──────────────────────────────────────────

// httpFace — HTTP 面的运行期句柄（listener ＋ server ＋ 解析好的配置）。
type httpFace struct {
	state brokerHTTPState
	ln    net.Listener
	srv   *http.Server
}

// startHTTP — `Serve` 起客前把 HTTP 面挂上（没 ListenHTTP ⇒ 什么都不做，老行为零变化）。
//
// ★ 收工形态：HTTP 面自己出错时**关掉 unix 监听**把 `Serve` 的 Accept 打醒 —— 于是
// 「HTTP 面死了」会从 `Serve` 的返回值里说出来，而不是让 broker 带着一只能听见半边的耳朵跑。
func (b *Broker) startHTTP(unixLn *net.UnixListener) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	face := b.http
	b.mu.Unlock()
	if face == nil {
		return nil
	}
	go func() {
		serr := face.srv.Serve(face.ln)
		if serr == nil || errors.Is(serr, http.ErrServerClosed) {
			return
		}
		b.setHTTPFailure(serr)
		if unixLn != nil {
			_ = unixLn.Close()
		}
	}()
	return nil
}

// setHTTPFailure — 记下 HTTP 面的致命错（供 `Serve` 的 Accept 失败分支读）。
func (b *Broker) setHTTPFailure(err error) {
	b.mu.Lock()
	b.httpErr = err
	b.mu.Unlock()
}

// httpFailure — 现读 HTTP 面的致命错（没有 ⇒ nil）。
func (b *Broker) httpFailure() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.httpErr
}

// closeHTTP — `Close` 收尾：停 HTTP 面（幂等，未开就什么都不做）。
func (b *Broker) closeHTTP() {
	b.mu.Lock()
	face := b.http
	b.http = nil
	b.mu.Unlock()
	if face == nil {
		return
	}
	// ★ 用 Close 而不是 Shutdown(ctx)：Shutdown 会等在手请求跑完，而这里可能在
	//   `Serve` 的 ctx 到期路径上 ⇒ 在飞的一条长推理能把收工卡到它的超时。
	_ = face.srv.Close()
	_ = face.ln.Close()
}
