// e2e_test.go —— T1.6 **端到端**：对话客户端 → 网关（真 HTTP）→ 子端（假子端），一条 trace 贯穿。
//
// 这是本任务最强的一条证据（比单测里的"假子端只看头"更进一步）：四个**互相独立**的来源必须给同一个 trace_id：
//
//	① 轮次事件（chat_obs.jsonl 里 kind=turn —— "本轮事件"）
//	② 主控入站事件（kind=trace dir=in —— 网关从请求头读到的）
//	③ 主控出站事件（kind=trace dir=out —— 主控**实际写进**转发请求头的）
//	④ 子端收到的头（假子端侧 —— 网络上的真实字节）
//
// 为什么放在 `package tracectx_test`（外部测试包）：它要同时 import chat 与 gateway，而这两个包都 import
// tracectx —— 在包内测试里会成环，外部测试包则天然合法（这就是它存在的理由）。
package tracectx_test

import (
	"context"
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

	"github.com/go-chi/chi/v5"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

const e2eModel = "trace-e2e-test"

// obsRowE2E —— 读回 chat_obs.jsonl 的行（宽解；键缺失 ⇒ 零值，这正是「缺席」的判据）
type obsRowE2E struct {
	Kind        string `json:"kind"`
	EventName   string `json:"event_name"`
	Dir         string `json:"dir"`
	Session     string `json:"session"`
	TraceID     string `json:"trace_id"`
	Root        bool   `json:"root"`
	Upstream    string `json:"upstream"`
	Traceparent string `json:"traceparent"`
	Baggage     string `json:"baggage"`
}

func readObsE2E(t *testing.T, stateDir string) []obsRowE2E {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateDir, "chat_obs.jsonl"))
	if err != nil {
		t.Fatalf("观测文件读不到：%v", err)
	}
	var out []obsRowE2E
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r obsRowE2E
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON：%v\n%s", err, ln)
		}
		out = append(out, r)
	}
	return out
}

// TestE2EOneTraceAcrossClientGatewaySubEnd —— 一次对话调用，四处证据同一个 trace_id。
func TestE2EOneTraceAcrossClientGatewaySubEnd(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)

	// ── 假子端（子端那一侧）──
	var (
		mu       sync.Mutex
		received []http.Header
	)
	subEnd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Header.Clone())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_e2e","object":"chat.completion","created":1,"model":"` + e2eModel + `",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],` +
			`"usage":{"total_tokens":3}}`))
	}))
	defer subEnd.Close()
	u, err := url.Parse(subEnd.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	// ── 真网关（真 HTTP 服务，走认证中间件 + 真实入口 handleRequest）──
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{e2eModel: {{Host: "x3", File: "/data/trace-e2e.gguf"}}},
		Fleet:  map[string]config.FleetNode{"x3": {Host: u.Hostname(), Port: port}},
	}
	gw := gateway.NewGateway("tok", cfg, store.NewStore(), nil)
	router := chi.NewRouter()
	gw.RegisterRoutes(router)
	gwSrv := httptest.NewServer(router)
	defer gwSrv.Close()

	// ── 对话客户端：本轮计时器先开（= 本轮事件诞生，含本条 trace）──
	session := t.Name() + "-sess"
	ctx := chat.WithSessionID(context.Background(), session)
	timer := chat.NewObsTimer(session, 1, e2eModel)

	ci := chat.NewChatInfer(gwSrv.URL, "tok")
	res, err := ci.Infer(ctx, e2eModel, "你是助手", []map[string]any{{"role": "user", "content": "hi"}})
	if err != nil {
		t.Fatalf("客户端调用失败：%v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("响应内容不符：%q", res.Content)
	}
	timer.MarkChunk()
	timer.Finish("finish")

	// ── 汇总证据 ──
	mu.Lock()
	gotTP := ""
	if len(received) > 0 {
		gotTP = tracectx.HeaderValue(received[0], tracectx.HeaderTraceparent)
	}
	gotBG := ""
	if len(received) > 0 {
		gotBG = tracectx.HeaderValue(received[0], tracectx.HeaderBaggage)
	}
	mu.Unlock()
	if gotTP == "" {
		t.Fatalf("子端没收到 traceparent（共收到 %d 跳）", len(received))
	}
	subTC, err := tracectx.ParseTraceparent(gotTP)
	if err != nil {
		t.Fatalf("子端收到的 traceparent 不合法：%q（%v）", gotTP, err)
	}

	rows := readObsE2E(t, stateDir)
	var turn, in, out obsRowE2E
	for _, r := range rows {
		switch {
		case r.Kind == "turn" && r.Session == session:
			turn = r
		case r.Kind == "trace" && r.Dir == "in":
			in = r
		case r.Kind == "trace" && r.Dir == "out":
			out = r
		}
	}
	if turn.TraceID == "" {
		t.Fatalf("本轮事件没有 trace_id（T1.1 骨架没生效？）：%+v", rows)
	}
	if in.TraceID == "" || out.TraceID == "" {
		t.Fatalf("主控侧传播事件缺席：in=%+v out=%+v", in, out)
	}

	// 四处必须同一个 trace_id（任意一处不同 ⇒ 这条链在那一跳断了）
	if turn.TraceID != in.TraceID {
		t.Errorf("轮次事件与网关入站不同 trace：turn=%q in=%q", turn.TraceID, in.TraceID)
	}
	if in.TraceID != out.TraceID {
		t.Errorf("网关入站与出站不同 trace：in=%q out=%q", in.TraceID, out.TraceID)
	}
	if out.TraceID != subTC.TraceID {
		t.Errorf("主控出站与子端实收不同 trace：out=%q 子端=%q", out.TraceID, subTC.TraceID)
	}
	if out.Traceparent != gotTP {
		t.Errorf("出站事件记的原文与子端实收不一致：事件=%q 实收=%q", out.Traceparent, gotTP)
	}
	// 会话标记也要真的到子端（baggage 载体不是装饰）
	if bg, err := tracectx.ParseBaggage(gotBG); err != nil || bg.Session() != session {
		t.Errorf("子端没收到会话标记：%q（%v）", gotBG, err)
	}
	// 上游链路的语义：客户端那一跳=present（带上本会话的 trace）⇒ 网关不需要另起 root
	if in.Upstream != tracectx.UpstreamPresent || in.Root {
		t.Errorf("网关应沿用客户端那条 trace（不是另起 root）：%+v", in)
	}
	if out.Root {
		t.Errorf("出站更不该另起 root：%+v", out)
	}

	// 证据原文（-v 打印，供人工复核 —— 报告里贴的是这几行）
	t.Logf("① 轮次事件      trace_id = %s", turn.TraceID)
	t.Logf("② 网关入站事件  trace_id = %s  traceparent = %s", in.TraceID, in.Traceparent)
	t.Logf("③ 网关出站事件  trace_id = %s  traceparent = %s", out.TraceID, out.Traceparent)
	t.Logf("④ 子端实收      trace_id = %s  traceparent = %s  baggage = %s", subTC.TraceID, gotTP, gotBG)
}
