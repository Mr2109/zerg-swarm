package agent

// summary_engine_test.go — P3 接线的四态自证（httptest 真跑，不打桩注入面本身）：
//
//	① 未设 env             ⇒ 不建实例（Policy.Engine=free、Summarize=nil），启动日志明写「默认 free」
//	② 设了但缺端点 / 缺模型 ⇒ 不建实例（不报错、不半建）
//	③ 设全                 ⇒ 建实例并注入 Policy，且真跑一次 HTTP 成功（假端点收 1 次、内容进摘要区）
//	④ 摘要失败             ⇒ 压缩照常完成（history 仍被压掉），Fallback=true，摘要退回 free 口径
//
// 四条都只读 env 与返回值，不依赖真机网关 / 真模型。

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// summaryStub —— 假 OpenAI 兼容端点：记次数、记请求体，按给定状态码/内容回话。
type summaryStub struct {
	srv   *httptest.Server
	hits  int32
	model string // 最近一次请求里的 model（断言注入是否带上了模型名）
}

func newSummaryStub(t *testing.T, status int, content string) *summaryStub {
	t.Helper()
	s := &summaryStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.hits, 1)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		s.model = req.Model
		if status != http.StatusOK {
			// 上游 5xx：故意回显一段「像 prompt 的东西」，验证失败路径不会拿它当摘要
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream boom"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": req.Model,
			"choices": []map[string]any{{
				"message":       map[string]string{"content": content},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 900, "completion_tokens": 20},
		})
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *summaryStub) Hits() int { return int(atomic.LoadInt32(&s.hits)) }

// captureLog —— 抓 log 包这一段的输出（启动日志原文断言用）。
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	oldOut, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	}()
	fn()
	return buf.String()
}

// longRoundHistory —— 一轮「assistant + 大工具回执」，估算远超 summaryRoundMinTokens(600)。
// 用 ASCII 让 compress.EstimateTokens（ascii/4）可算：8000 字节 ≈ 2000 token。
func longRoundHistory() []Message {
	big := strings.Repeat("tool output line: file=/tmp/zerg/a.go ok=true\n", 160)
	return []Message{
		{Role: "user", Content: "跑一下测试"},
		{Role: "assistant", Content: "我先读文件", ToolCalls: []ToolCall{{Name: "read", Args: map[string]any{"path": "/tmp/zerg/a.go"}}}},
		{Role: "tool", ToolCallID: "t1", Content: big},
		{Role: "assistant", Content: "读完了"},
		{Role: "tool", ToolCallID: "t2", Content: "ok"},
	}
}

// allSummaryEnvOff —— 四条 env 一律清空（含 token/timeout，避免真机残留）。
func allSummaryEnvOff(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		envSummaryEngine, envSummaryEndpoint, envSummaryModel,
		envSummaryToken, envSummaryTimeoutS, envSummaryMaxTokens,
	} {
		t.Setenv(k, "")
	}
}

// ① 未设 env ⇒ 不建实例 + 启动日志原文「摘要引擎未启用（默认 free）」。
func TestSummaryEngineNotBuiltWithoutEnv(t *testing.T) {
	allSummaryEnvOff(t)

	var a *Agent
	logs := captureLog(t, func() { a = NewAgent(Config{}) })

	if a.summaryEngine.enabled {
		t.Fatal("未设 env 却建了摘要引擎（违反「永不自动激活」）")
	}
	if got := a.summaryEngine.policy.Engine; got != "free" {
		t.Fatalf("默认引擎该是 free，实得 %q", got)
	}
	if a.summaryEngine.policy.Summarize != nil {
		t.Fatal("未启用时 Policy.Summarize 必须为 nil（否则 summary 引擎会被注入面点亮）")
	}
	if a.summaryEngine.status != summaryEngineOffStatus {
		t.Fatalf("启动日志原文该是 %q，实得 %q", summaryEngineOffStatus, a.summaryEngine.status)
	}
	if !strings.Contains(logs, summaryEngineOffStatus) {
		t.Fatalf("启动日志没写明「%s」，实得：%s", summaryEngineOffStatus, logs)
	}
	t.Logf("启动日志原文：%s", strings.TrimSpace(logs))

	// 消费面也必须是零网络：未启用 ⇒ 压缩照跑，摘要走 free 口径
	a.history = longRoundHistory()
	if !a.compressOneRound() {
		t.Fatal("未启用时压缩仍应完成（free 口径）")
	}
	if a.summaryEngine.calls != 0 {
		t.Fatalf("未启用却记了 %d 次引擎调用", a.summaryEngine.calls)
	}
	if !strings.Contains(a.rollingSummary.text, "决策: 我先读文件") {
		t.Fatalf("free 摘要形态不对：%q", a.rollingSummary.text)
	}
}

// ② 设了总闸但缺端点 / 缺模型 ⇒ 不建实例、不报错。
func TestSummaryEngineNotBuiltWhenEndpointOrModelMissing(t *testing.T) {
	allSummaryEnvOff(t)

	cases := []struct {
		name   string
		env    map[string]string
		wantIn string // 原因里必须点出缺的是哪个变量
	}{
		{
			name:   "缺 ENDPOINT",
			env:    map[string]string{envSummaryEngine: "1", envSummaryModel: "stub-model"},
			wantIn: envSummaryEndpoint,
		},
		{
			name:   "缺 MODEL",
			env:    map[string]string{envSummaryEngine: "1", envSummaryEndpoint: "http://127.0.0.1:9"},
			wantIn: envSummaryModel,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := func(k string) string { return tc.env[k] }
			st := resolveSummaryEngine(defaultCompactCtxTokens, lookup)
			if st.enabled {
				t.Fatal("配置不全却建了摘要引擎")
			}
			if st.policy.Engine != "free" || st.policy.Summarize != nil {
				t.Fatalf("配置不全必须落 free（Engine=%q, Summarize=nil? %v）",
					st.policy.Engine, st.policy.Summarize == nil)
			}
			if st.status != summaryEngineOffStatus {
				t.Fatalf("启动日志原文该是 %q，实得 %q", summaryEngineOffStatus, st.status)
			}
			if !strings.Contains(st.reason, tc.wantIn) {
				t.Fatalf("原因该点出缺 %s，实得 %q", tc.wantIn, st.reason)
			}
		})
	}

	// 总闸写成 0 / 笔误 ⇒ 同样不建（默认关）
	if st := resolveSummaryEngine(defaultCompactCtxTokens, func(k string) string {
		return map[string]string{envSummaryEngine: "0", envSummaryEndpoint: "http://127.0.0.1:9", envSummaryModel: "m"}[k]
	}); st.enabled {
		t.Fatal("总闸=0 不该建引擎")
	}
}

// ③ 设全 ⇒ 建实例 + 注入 Policy + 真跑一次 HTTP 成功（内容进摘要区）。
func TestSummaryEngineBuiltAndInjectedWhenEnvComplete(t *testing.T) {
	stub := newSummaryStub(t, http.StatusOK, "摘要：读了 /tmp/zerg/a.go，测试通过。")
	allSummaryEnvOff(t)
	t.Setenv(envSummaryEngine, "1")
	t.Setenv(envSummaryEndpoint, stub.srv.URL) // 基址 ⇒ 适配器自补 /v1/chat/completions
	t.Setenv(envSummaryModel, "stub-model")
	t.Setenv(envSummaryTimeoutS, "5")

	a := NewAgent(Config{})
	if !a.summaryEngine.enabled {
		t.Fatalf("env 齐全却没建引擎：%s", a.summaryEngine.logLine())
	}
	if a.summaryEngine.policy.Engine != "summary" {
		t.Fatalf("Policy.Engine 该是 summary，实得 %q", a.summaryEngine.policy.Engine)
	}
	if a.summaryEngine.policy.Summarize == nil {
		t.Fatal("Policy.Summarize 没被注入（summary 引擎会静默降级）")
	}

	a.history = longRoundHistory()
	before := len(a.history)
	if !a.compressOneRound() {
		t.Fatal("压缩没发生")
	}
	if stub.Hits() != 1 {
		t.Fatalf("假端点该被调 1 次，实得 %d 次", stub.Hits())
	}
	if stub.model != "stub-model" {
		t.Fatalf("请求里的 model 该是 stub-model，实得 %q", stub.model)
	}
	if a.summaryEngine.lastFallback {
		t.Fatalf("成功路径不该标 Fallback：%s", a.summaryEngine.lastReason)
	}
	if !strings.Contains(a.rollingSummary.text, "测试通过") {
		t.Fatalf("模型摘要没进滚动摘要区：%q", a.rollingSummary.text)
	}
	if len(a.history) >= before || a.rollingSummary.compacted != 1 {
		t.Fatalf("压缩没落地：history %d→%d, compacted=%d", before, len(a.history), a.rollingSummary.compacted)
	}
}

// ④ 摘要端点 500 ⇒ 压缩照常完成、摘要退回 free 口径、Fallback=true（主流程零影响）。
func TestSummaryEngineFailureFallsBackToFree(t *testing.T) {
	stub := newSummaryStub(t, http.StatusInternalServerError, "")
	allSummaryEnvOff(t)
	t.Setenv(envSummaryEngine, "1")
	t.Setenv(envSummaryEndpoint, stub.srv.URL)
	t.Setenv(envSummaryModel, "stub-model")
	t.Setenv(envSummaryTimeoutS, "5")

	a := NewAgent(Config{})
	if !a.summaryEngine.enabled {
		t.Fatalf("env 齐全却没建引擎：%s", a.summaryEngine.logLine())
	}

	a.history = longRoundHistory()
	before := len(a.history)
	if !a.compressOneRound() {
		t.Fatal("摘要失败不该让压缩失败（必须退回 free 继续压）")
	}
	if stub.Hits() != 1 {
		t.Fatalf("假端点该被调 1 次，实得 %d 次", stub.Hits())
	}
	if !a.summaryEngine.lastFallback {
		t.Fatal("摘要失败却没标 Fallback")
	}
	if a.summaryEngine.fallbacks != 1 || a.summaryEngine.lastReason == "" {
		t.Fatalf("降级计数/原因不对：fallbacks=%d reason=%q", a.summaryEngine.fallbacks, a.summaryEngine.lastReason)
	}
	if strings.Contains(a.rollingSummary.text, "upstream boom") {
		t.Fatal("失败响应体被当成摘要塞进上下文（绝不允许）")
	}
	if !strings.Contains(a.rollingSummary.text, "决策: 我先读文件") {
		t.Fatalf("没退回 free 口径：%q", a.rollingSummary.text)
	}
	if len(a.history) >= before || a.rollingSummary.compacted != 1 {
		t.Fatalf("压缩没落地：history %d→%d, compacted=%d", before, len(a.history), a.rollingSummary.compacted)
	}
}
