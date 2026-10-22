package compress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// 摘要器适配器的自证：**全部**走 httptest.Server，不触外网、不接真模型。
//
// 四条主用例 = 四种现实：
//  1. 正常 200（形状对、片段对、计数对）；
//  2. 500（失败且不回显 prompt / 令牌、不拿原文兜底）；
//  3. 超时（deadline 到点、不挂整轮）；
//  4. 截断（输出预算硬闸 + finish_reason=length 也算截断）。

const (
	testSumToken  = "sk-test-SECRET-do-not-leak-4f2a"
	testSumNeedle = "NEEDLE-7f3a-机密片段-勿回显"
)

// sumFixture —— 一组普通片段：一条长轨迹（带锚点）+ 一条观察。
func sumFixture() []Segment {
	return []Segment{
		{Kind: KindTrajectory, Text: testSumNeedle + strings.Repeat("轨迹内容", 40)},
		{Kind: KindObservation, Text: strings.Repeat("观察输出", 60), Anchors: []string{"core/internal/compress/policy.go"}},
	}
}

func newTestSummarizer(t *testing.T, srvURL string, timeout time.Duration, maxTokens int) *HTTPSummarizer {
	t.Helper()
	h, err := NewHTTPSummarizer(HTTPSummarizerConfig{
		Endpoint:  srvURL,
		Model:     "gemma-4-12B",
		Token:     testSumToken,
		Timeout:   timeout,
		MaxTokens: maxTokens,
	})
	if err != nil {
		t.Fatalf("构造适配器失败：%v", err)
	}
	return h
}

// ① 正常路径：OpenAI 兼容形状（model / messages / max_tokens / temperature）+ 令牌在头里不在体里。
func TestHTTPSummarizerPostsOpenAICompatibleRequestAndReturnsSegment(t *testing.T) {
	var gotPath, gotAuth, gotXAauth string
	var gotBody map[string]any
	var gotMsgs []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotXAauth = r.Header.Get("X-Auth-Token")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		if msgs, ok := gotBody["messages"].([]any); ok {
			for _, m := range msgs {
				gotMsgs = append(gotMsgs, m.(map[string]any))
			}
		}
		// 响应体故意带首尾空白：适配器必须 TrimSpace 后再交出去。
		fmt.Fprint(w, `{"model":"gemma-4-12B","choices":[{"message":{"content":"  摘要：A→B→C，路径 core/internal/compress/policy.go 保留  "},"finish_reason":"stop"}],"usage":{"prompt_tokens":900,"completion_tokens":24}}`)
	}))
	defer srv.Close()

	h := newTestSummarizer(t, srv.URL, 2*time.Second, 0) // 0 ⇒ 默认 1200
	segs := sumFixture()

	got, err := h.Func()(context.Background(), segs) // 走 SummarizerFunc 形状（注入面）
	if err != nil {
		t.Fatalf("正常响应不该报错：%v", err)
	}

	// —— 请求面：路径归一化 + 与 chat.CompactRequest 同形
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("端点未归一化到 /v1/chat/completions，实得 %q", gotPath)
	}
	if gotBody["model"] != "gemma-4-12B" {
		t.Fatalf("model 字段不对：%v", gotBody["model"])
	}
	if mt, _ := gotBody["max_tokens"].(float64); int(mt) != DefaultSummarizerMaxTokens {
		t.Fatalf("max_tokens 缺省该是 %d，实得 %v", DefaultSummarizerMaxTokens, gotBody["max_tokens"])
	}
	if temp, _ := gotBody["temperature"].(float64); temp != defaultSummarizerTemperature {
		t.Fatalf("temperature 缺省该是 %.1f，实得 %v", defaultSummarizerTemperature, gotBody["temperature"])
	}
	if len(gotMsgs) != 2 || gotMsgs[0]["role"] != "system" || gotMsgs[1]["role"] != "user" {
		t.Fatalf("messages 形状不对：%+v", gotMsgs)
	}
	userBlock, _ := gotMsgs[1]["content"].(string)
	if !strings.Contains(userBlock, "NEEDLE-7f3a") || !strings.Contains(userBlock, "core/internal/compress/policy.go") {
		t.Fatalf("user 块没带正文/锚点：%q", userBlock[:80])
	}

	// —— 认证：只在头里，且 Authorization 与 X-Auth-Token 都带（OpenAI 面 / 网关面各认一种）
	if gotAuth != "Bearer "+testSumToken || gotXAauth != testSumToken {
		t.Fatalf("认证头不对：Authorization=%q X-Auth-Token=%q", gotAuth, gotXAauth)
	}
	if b, _ := json.Marshal(gotBody); strings.Contains(string(b), testSumToken) {
		t.Fatal("令牌被写进了请求体")
	}

	// —— 响应面：一个 representation 片段，文本已 trim，Kind 已定
	if len(got) != 1 {
		t.Fatalf("该返回 1 个摘要片段，实得 %d", len(got))
	}
	if got[0].Kind != KindRepresentation {
		t.Fatalf("摘要片段 Kind 应为 representation，实得 %q", got[0].Kind)
	}
	if got[0].Text != "摘要：A→B→C，路径 core/internal/compress/policy.go 保留" {
		t.Fatalf("摘要文本未按预期 trim/保留：%q", got[0].Text)
	}

	st := h.Stats()
	if st.Calls != 1 || st.FailedCalls != 0 || st.TruncatedCalls != 0 || st.LastTruncated {
		t.Fatalf("计数不对：%+v", st)
	}
	if st.LastFinishReason != "stop" || st.LastModel != "gemma-4-12B" {
		t.Fatalf("元数据未记录：%+v", st)
	}
}

// ② 500 与空内容：一律 error + nil 片段；错误串里**不得**出现令牌或 prompt 正文
// （服务端故意把请求体回显进错误体——真实世界里 infer 服务就这么干）。
func TestHTTPSummarizer500FailsWithoutEchoingPromptOrToken(t *testing.T) {
	var mode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		switch mode {
		case "empty":
			fmt.Fprint(w, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`)
		case "tokenonly":
			// 不回显 prompt、只把密钥写进错误体 —— 这类要过 redact 变成 ***，而不是整段隐去
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `{"message":"upstream rejected key %s for model gemma-4-12B"}`, testSumToken)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			// 回显：把整个 request body（含 prompt）和令牌都吐进 error body
			fmt.Fprintf(w, `{"error":{"message":"boom"},"echo":%s,"token":%q}`, raw, testSumToken)
		}
	}))
	defer srv.Close()

	h := newTestSummarizer(t, srv.URL, 2*time.Second, 0)
	segs := sumFixture()

	check := func(tag string, err error, got []Segment) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s：该报错却成功了", tag)
		}
		if got != nil {
			t.Fatalf("%s：失败时必须返回 nil 片段（不许拿原文兜底），实得 %d 条", tag, len(got))
		}
		msg := err.Error()
		if strings.Contains(msg, testSumToken) {
			t.Fatalf("%s：错误串泄漏令牌：%q", tag, msg)
		}
		if strings.Contains(msg, "NEEDLE-7f3a") || strings.Contains(msg, "轨迹内容") {
			t.Fatalf("%s：错误串回显了 prompt 正文：%q", tag, msg)
		}
		if strings.Contains(strings.ToLower(msg), "authorization") || strings.Contains(msg, "bearer") {
			t.Fatalf("%s：错误串提到认证头：%q", tag, msg)
		}
		t.Logf("%s 的错误串（实测原文，已可安全落日志）：%s", tag, msg)
	}

	mode = "500"
	got, err := h.Summarize(context.Background(), segs)
	check("500", err, got)
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("错误串该带状态码（否则排障没抓手）：%q", err.Error())
	}

	mode = "empty"
	got, err = h.Summarize(context.Background(), segs)
	check("空内容", err, got)

	mode = "tokenonly"
	got, err = h.Summarize(context.Background(), segs)
	check("密钥回显", err, got)
	if !strings.Contains(err.Error(), "***") {
		t.Fatalf("密钥该被抹成 ***（而不是整段隐去后什么都看不到）：%q", err.Error())
	}

	st := h.Stats()
	if st.FailedCalls != 3 || st.Calls != 0 {
		t.Fatalf("失败计数不对：%+v", st)
	}
}

// ③ 超时：适配器的 Timeout 硬闸生效，错误可被 errors.Is 识别，且不拖死调用方。
func TestHTTPSummarizerTimeoutReturnsDeadlineExceeded(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer func() { close(release); srv.Close() }()

	h := newTestSummarizer(t, srv.URL, 60*time.Millisecond, 0)

	start := time.Now()
	got, err := h.Func()(context.Background(), sumFixture())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("端点不返回却当成功")
	}
	if got != nil {
		t.Fatalf("超时必须返回 nil 片段，实得 %d 条", len(got))
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("错误未携带 context.DeadlineExceeded（上层没法分流超时）：%v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("超时闸没生效：耗时 %s（配置 60ms）", elapsed)
	}
	if strings.Contains(err.Error(), testSumToken) {
		t.Fatalf("超时错误串泄漏令牌：%q", err.Error())
	}
	if st := h.Stats(); st.TimeoutCalls != 1 || st.Calls != 0 {
		t.Fatalf("超时计数不对：%+v", st)
	}
	t.Logf("超时实测：%s 内返回 %v", elapsed.Round(time.Millisecond), err)
}

// ④ 输出预算：超出即截断并标 Truncated（文本带可见标记、Stats 可查）；finish_reason=length 同样算截断。
func TestHTTPSummarizerTruncatesToOutputBudget(t *testing.T) {
	const budget = 200
	var mode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, finish := strings.Repeat("细节", 3000), "stop"
		if mode == "length" {
			content, finish = "短摘要，但上游说它被截了", "length"
		}
		fmt.Fprintf(w, `{"model":"m","choices":[{"message":{"content":%q},"finish_reason":%q}]}`, content, finish)
	}))
	defer srv.Close()

	h := newTestSummarizer(t, srv.URL, 2*time.Second, budget)

	mode = "long"
	got, err := h.Summarize(context.Background(), sumFixture())
	if err != nil {
		t.Fatalf("截断路径不该报错：%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("该返回 1 个摘要片段，实得 %d", len(got))
	}
	text := got[0].Text
	if n := EstimateTokens(text); n > budget {
		t.Fatalf("截断后仍超预算：%d > %d token（%d 字）", n, budget, len([]rune(text)))
	}
	if !strings.HasSuffix(text, summarizerTruncatedMark) {
		t.Fatalf("截断标记没进文本：…%q", text[len(text)-20:])
	}
	if !strings.HasPrefix(text, "细节") {
		t.Fatalf("截断破坏了前缀（应保留模型输出开头）：%q", text[:8])
	}
	st := h.Stats()
	if !st.LastTruncated || st.TruncatedCalls != 1 || st.Calls != 1 {
		t.Fatalf("截断未记账：%+v", st)
	}
	if st.LastOutputTokens > budget {
		t.Fatalf("Stats 报的输出 token 超预算：%d > %d", st.LastOutputTokens, budget)
	}

	// 上游自报 length：即使文本短也要标截断（否则「被截了」这件事丢了）
	mode = "length"
	got, err = h.Summarize(context.Background(), sumFixture())
	if err != nil {
		t.Fatalf("length 路径不该报错：%v", err)
	}
	if st := h.Stats(); !st.LastTruncated || st.TruncatedCalls != 2 || st.LastFinishReason != "length" {
		t.Fatalf("finish_reason=length 未标截断：%+v", st)
	}
	if got[0].Text != "短摘要，但上游说它被截了" {
		t.Fatalf("未超预算的文本不该被改写：%q", got[0].Text)
	}

	// 小预算（连标记都放不下）仍不许超：只截正文
	h2 := newTestSummarizer(t, srv.URL, 2*time.Second, 3)
	mode = "long"
	got, err = h2.Summarize(context.Background(), sumFixture())
	if err != nil {
		t.Fatalf("极小预算不该报错：%v", err)
	}
	if n := EstimateTokens(got[0].Text); n > 3 {
		t.Fatalf("极小预算下仍超：%d > 3（%q）", n, got[0].Text)
	}
	if len([]rune(got[0].Text)) == 0 {
		t.Fatal("极小预算不该截成空串（SummaryEngine 会把空文本判为失败）")
	}
}

// ⑤ 纯注入守卫：本包不得直接读环境变量（④ 的机读证据）。
func TestSummarizerHTTPReadsNoEnv(t *testing.T) {
	src, err := os.ReadFile("summarizer_http.go")
	if err != nil {
		t.Fatalf("读源文件失败：%v", err)
	}
	for _, bad := range []string{"os.Getenv", "os.LookupEnv", "os.Environ", "log.Printf", "log.Println"} {
		if strings.Contains(string(src), bad) {
			t.Fatalf("适配器出现了不该有的调用 %q（env 只能由调用方读；本文件也不该打日志——令牌会顺手进日志）", bad)
		}
	}
}
