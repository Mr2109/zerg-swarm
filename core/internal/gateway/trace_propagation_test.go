// trace_propagation_test.go —— T1.6 跨进程实证：**主控 → 子端** 一次调用 ⇒ 两侧同一条 trace。
//
// 单测做不了真跨进程（两个进程），所以按任务约定用**假子端**（httptest）当"子端那一侧的事件"：
// 子端收到的 traceparent 就是它的证据。于是三样东西必须对得上：
//
//	① 假子端**收到**的 traceparent 里的 trace_id
//	② 主控 JSONL 里 dir=out 事件（我们实际写进请求头的原文）的 trace_id
//	③ 主控 JSONL 里 dir=in 事件（入站采纳的原文）的 trace_id
//
// 三个不同来源（网络头 / 出站记账 / 入站记账）都相等 ⇒ "拼得上"不是自称。
// 另加两条负控：本侧没上游头时必须自己生成（root=true）而不是空 trace_id；上游给了非法值必须拒。
package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// traceTestModel —— 用例专用模型名（不碰真实 fleet.yaml 里的任何模型）
const traceTestModel = "trace-echo-test"

// traceUpstreamSample —— 模拟上游（客户端/CA）给出的 traceparent（W3C 规范示例）
const traceUpstreamSample = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

// fakeSubEnd —— 假子端：记录每一跳收到的头（"子端侧事件"的证据），回一个最小 OpenAI 兼容响应。
type fakeSubEnd struct {
	srv *httptest.Server

	mu    sync.Mutex
	hdrs  []http.Header
	paths []string
}

func newFakeSubEnd(t *testing.T) *fakeSubEnd {
	t.Helper()
	f := &fakeSubEnd{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hdrs = append(f.hdrs, r.Header.Clone())
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_fake","object":"chat.completion","created":1,` +
			`"model":"trace-echo-test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},` +
			`"finish_reason":"stop"}],"usage":{"total_tokens":3}}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// hostPort —— 从 httptest 的 URL 取出 fleet 配置要的 (host, port)。
func hostPort(t *testing.T, raw string) (string, int) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("解析假子端地址失败：%v", err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("假子端端口不是数字：%v", err)
	}
	return u.Hostname(), p
}

// newTraceTestGateway —— 建一个把 x3 指向假子端的网关（不加载真实 fleet.yaml）。
func newTraceTestGateway(t *testing.T, srv *httptest.Server) *Gateway {
	t.Helper()
	host, port := hostPort(t, srv.URL)
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			traceTestModel: {{Host: "x3", File: "/data/trace-echo-test.gguf"}},
		},
		Fleet: map[string]config.FleetNode{
			"x3": {Host: host, Port: port},
		},
	}
	return NewGateway("tok", cfg, store.NewStore(), nil)
}

// traceReqBody —— 一次对话请求体（带会话 id —— 传播/粘性都靠它）
func traceReqBody(session string) string {
	return `{"model":"` + traceTestModel + `","session_id":"` + session + `",` +
		`"messages":[{"role":"user","content":"hi"}]}`
}

// obsTraceRow —— 读回 chat_obs.jsonl 里 kind=trace 的行
type obsTraceRow struct {
	Kind         string `json:"kind"`
	EventName    string `json:"event_name"`
	Dir          string `json:"dir"`
	Session      string `json:"session"`
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id"`
	ParentSource string `json:"parent_source"`
	Traceparent  string `json:"traceparent"`
	Baggage      string `json:"baggage"`
	Root         bool   `json:"root"`
	Upstream     string `json:"upstream"`
	RejectReason string `json:"reject_reason"`
	PeerHost     string `json:"peer_host"`
	PeerPath     string `json:"peer_path"`
}

// readObsTraceRows —— 逐行解析（解析不动即红：读侧宽容由它守住）
func readObsTraceRows(t *testing.T, stateDir string) []obsTraceRow {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateDir, "chat_obs.jsonl"))
	if err != nil {
		t.Fatalf("观测文件读不到（传播事件根本没落盘？）：%v", err)
	}
	var out []obsTraceRow
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r obsTraceRow
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON：%v\n%s", err, ln)
		}
		if r.Kind == "trace" {
			out = append(out, r)
		}
	}
	return out
}

// doChatRequest —— 走真实入站处理函数（handleRequest），可注入上游 traceparent
func doChatRequest(t *testing.T, g *Gateway, session, upstreamTraceparent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(traceReqBody(session)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "tok")
	if upstreamTraceparent != "" {
		req.Header.Set(tracectx.HeaderTraceparent, upstreamTraceparent)
	}
	rec := httptest.NewRecorder()
	g.handleRequest(rec, req)
	return rec
}

// ① 上游给了 traceparent ⇒ 主控入站沿用、出站透传、子端收到 —— 三处同一条 trace_id
func TestTracePropagationMainToSubEndSameTrace(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)

	fake := newFakeSubEnd(t)
	g := newTraceTestGateway(t, fake.srv)
	session := t.Name() + "-sess"

	if rec := doChatRequest(t, g, session, traceUpstreamSample); rec.Code != http.StatusOK {
		t.Fatalf("入站请求未成功：%d %s", rec.Code, rec.Body.String())
	}

	// ①-1 假子端（子端那一侧）：收到的 traceparent 必须能解析，且 trace_id 就是上游那条
	if len(fake.hdrs) != 1 {
		t.Fatalf("假子端应收到 1 跳，实际 %d（paths=%v）", len(fake.hdrs), fake.paths)
	}
	gotTP := tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderTraceparent)
	gotTC, err := tracectx.ParseTraceparent(gotTP)
	if err != nil {
		t.Fatalf("子端收到的 traceparent 不是合法 W3C 格式：%q（%v）", gotTP, err)
	}
	if gotTC.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("子端收到的 trace_id 与上游不一致（链路在转发处断了）：%q", gotTC.TraceID)
	}
	if gotTC.SpanID == "00f067aa0ba902b7" {
		t.Errorf("出站必须新开本跳 span（不能照抄上游 span）：%q", gotTC.SpanID)
	}
	if bg := tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderBaggage); bg != "zerg.session="+session {
		t.Errorf("子端收到的 baggage 应带会话标记：%q", bg)
	}

	// ①-2 主控这一侧：入站/出站各一条传播事件，trace_id 与子端收到的一致
	rows := readObsTraceRows(t, stateDir)
	in, out := traceRowOf(t, rows, "in"), traceRowOf(t, rows, "out")
	if in.TraceID == "" || out.TraceID == "" {
		t.Fatalf("主控侧传播事件缺席：in=%+v out=%+v（全部行=%+v）", in, out, rows)
	}
	if in.TraceID != gotTC.TraceID || out.TraceID != gotTC.TraceID {
		t.Errorf("两侧不同 trace：子端=%q 主控入站=%q 主控出站=%q", gotTC.TraceID, in.TraceID, out.TraceID)
	}
	if in.Upstream != tracectx.UpstreamPresent || in.Root {
		t.Errorf("入站应沿用上游（present + root=false）：%+v", in)
	}
	if in.ParentSpanID != "00f067aa0ba902b7" || in.ParentSource != tracectx.ParentFromHeader {
		t.Errorf("入站事件的父 span 应指上游那一跳：%+v", in)
	}
	if out.PeerHost != "x3" || out.PeerPath != "/infer" {
		t.Errorf("出站事件应写明对端：%+v", out)
	}
	if out.Session != session {
		t.Errorf("出站事件应带会话（子端据此认领）：%+v", out)
	}
	// 事件里记的 traceparent 原文必须与子端实际收到的**逐字节**一致（记账 ≠ 真实是更糟的失败）
	if out.Traceparent != gotTP {
		t.Errorf("出站事件记的原文与实际发出的不一致：事件=%q 实收=%q", out.Traceparent, gotTP)
	}
	// 证据（-v 时打印）：两侧三处的原文，供人工复核
	t.Logf("子端收到 traceparent = %s", gotTP)
	t.Logf("子端收到 baggage     = %s", tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderBaggage))
	t.Logf("主控入站事件 = %s", traceRawLine(t, stateDir, "in"))
	t.Logf("主控出站事件 = %s", traceRawLine(t, stateDir, "out"))
}

// ② 上游没给 ⇒ 本侧生成（root=true，绝不留空 trace_id）；入站/出站/子端仍是同一条
func TestTracePropagationGeneratesRootWhenAbsent(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)

	fake := newFakeSubEnd(t)
	g := newTraceTestGateway(t, fake.srv)
	session := t.Name() + "-sess"

	if rec := doChatRequest(t, g, session, ""); rec.Code != http.StatusOK {
		t.Fatalf("入站请求未成功：%d %s", rec.Code, rec.Body.String())
	}

	gotTP := tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderTraceparent)
	gotTC, err := tracectx.ParseTraceparent(gotTP)
	if err != nil {
		t.Fatalf("没给上游头时也必须写出合法 traceparent：%q（%v）", gotTP, err)
	}
	rows := readObsTraceRows(t, stateDir)
	in, out := traceRowOf(t, rows, "in"), traceRowOf(t, rows, "out")
	if in.Upstream != tracectx.UpstreamAbsent || !in.Root {
		t.Errorf("上游没给 ⇒ 应 absent + root=true：%+v", in)
	}
	if in.TraceID == "" || in.SpanID == "" {
		t.Errorf("本侧生成必须落到事件里（不许静默拿空串当 trace_id）：%+v", in)
	}
	if in.TraceID != gotTC.TraceID || out.TraceID != gotTC.TraceID {
		t.Errorf("本侧生成的链路没贯穿到底：子端=%q in=%q out=%q", gotTC.TraceID, in.TraceID, out.TraceID)
	}
	if in.ParentSpanID != "" || in.ParentSource != "" {
		t.Errorf("root 没有父（不许编一个）：%+v", in)
	}
	if in.TraceID == out.SpanID || len(in.TraceID) != tracectx.TraceIDHexLen {
		t.Errorf("trace_id 形态不对：%q", in.TraceID)
	}
}

// ③ 上游给了**非法**值 ⇒ 拒绝（不沿用）+ 本侧新生成 + 原因落盘；链路照样贯穿到子端
func TestTracePropagationRejectsInvalidUpstream(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)

	fake := newFakeSubEnd(t)
	g := newTraceTestGateway(t, fake.srv)

	const bogus = "00-ABC-def-01" // 大写 + 长度不符：两条都该被拒
	if rec := doChatRequest(t, g, t.Name()+"-sess", bogus); rec.Code != http.StatusOK {
		t.Fatalf("非法上游头不得影响业务（拒绝了也要照常服务）：%d %s", rec.Code, rec.Body.String())
	}

	gotTP := tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderTraceparent)
	gotTC, err := tracectx.ParseTraceparent(gotTP)
	if err != nil {
		t.Fatalf("转发出去的头必须合法：%q（%v）", gotTP, err)
	}
	rows := readObsTraceRows(t, stateDir)
	in, out := traceRowOf(t, rows, "in"), traceRowOf(t, rows, "out")
	if in.Upstream != tracectx.UpstreamInvalid {
		t.Errorf("非法上游必须记 invalid（不是 absent —— 对端实现有问题要留痕）：%+v", in)
	}
	if in.RejectReason == "" {
		t.Errorf("拒绝必须留原因：%+v", in)
	}
	if in.TraceID == "" || strings.Contains(strings.ToUpper(in.TraceID), "ABC") {
		t.Errorf("拒绝的含义就是不沿用：%+v", in)
	}
	if in.TraceID != gotTC.TraceID || out.TraceID != gotTC.TraceID {
		t.Errorf("拒绝后新生成的链路也要贯穿到底：子端=%q in=%q out=%q", gotTC.TraceID, in.TraceID, out.TraceID)
	}
}

// ④ 回放标记：ZERG_REPLAY=1 ⇒ baggage 带 zerg.replay=1（子端看得见"这是回放"）
func TestTracePropagationReplayMarker(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv(tracectx.ReplayEnv, "1")

	fake := newFakeSubEnd(t)
	g := newTraceTestGateway(t, fake.srv)
	session := t.Name() + "-sess"
	if rec := doChatRequest(t, g, session, ""); rec.Code != http.StatusOK {
		t.Fatalf("入站请求未成功：%d", rec.Code)
	}
	bg, err := tracectx.ParseBaggage(tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderBaggage))
	if err != nil {
		t.Fatalf("baggage 不是合法格式：%v", err)
	}
	if !bg.Replay() || bg.Session() != session {
		t.Errorf("回放标记/会话标记没传到子端：%q", tracectx.HeaderValue(fake.hdrs[0], tracectx.HeaderBaggage))
	}
}

// traceRowOf —— 取某方向的第一条传播事件（缺席即 Fatal：缺证据不能当通过）
func traceRowOf(t *testing.T, rows []obsTraceRow, dir string) obsTraceRow {
	t.Helper()
	for _, r := range rows {
		if r.Dir == dir {
			return r
		}
	}
	t.Fatalf("缺少 dir=%s 的传播事件（全部 %d 行：%+v）", dir, len(rows), rows)
	return obsTraceRow{}
}

// traceRawLine —— 取某方向的传播事件**原文行**（证据用：贴给报告的是落盘原文，不是结构体）
func traceRawLine(t *testing.T, stateDir, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateDir, "chat_obs.jsonl"))
	if err != nil {
		return "<读不到：" + err.Error() + ">"
	}
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var r obsTraceRow
		if json.Unmarshal([]byte(ln), &r) == nil && r.Kind == "trace" && r.Dir == dir {
			return ln
		}
	}
	return "<没有 dir=" + dir + " 的行>"
}
