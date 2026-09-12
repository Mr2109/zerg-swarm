package modelreg

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ── 在线探测器（text / vision / tools / embedding / rerank + 端点 meta）──────
//
// 判据遵循开工方案 §八 风险1：只认结构化信号（HTTP 码 + 能否解析出字段），
// 不拿文案字符串当通过条件；唯一例外是"缺 mmproj"这一条——它是把 no_vision
// 细分出来的提示，主判据仍是 HTTP 码（见 ProbeVision）。

// Endpoint 是一个 OpenAI 兼容引擎端点。
type Endpoint struct {
	BaseURL string // 形如 http://127.0.0.1:9000 或 http://host:port/v1
	Model   string // 请求体里的 model 字段（引擎多会忽略）
	Timeout time.Duration
	Client  *http.Client
}

func (e Endpoint) timeout() time.Duration {
	if e.Timeout > 0 {
		return e.Timeout
	}
	return DefaultProbeTimeout
}

func (e Endpoint) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return &http.Client{Timeout: e.timeout()}
}

// url 把相对路径拼到端点上；端点若已带 /v1 就不再重复。
func (e Endpoint) url(path string) string {
	base := strings.TrimRight(e.BaseURL, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + path
	}
	return base + "/v1" + path
}

func (e Endpoint) do(ctx context.Context, method, path string, body any) (int, []byte, time.Duration, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.url(path), rdr)
	if err != nil {
		return 0, nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	start := time.Now()
	resp, err := e.client().Do(req)
	el := time.Since(start)
	if err != nil {
		return 0, nil, el, err
	}
	defer resp.Body.Close()
	data, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if rerr != nil {
		return resp.StatusCode, data, el, rerr
	}
	return resp.StatusCode, data, el, nil
}

func (e Endpoint) chatCall(body map[string]any) (int, []byte, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	return e.do(ctx, http.MethodPost, "/chat/completions", body)
}

// classifyNetErr 把网络层错误归到标准失败分类（cant_start / timeout）。
func classifyNetErr(err error) string {
	if err == nil {
		return ""
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return FailTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailTimeout
	}
	return FailCantStart
}

// probeBudgetLadder 是"先小后大"的生成预算阶梯（待修补 #27）。
//
// 为什么要有阶梯：思考模型会把预算先花在思考（reasoning）上——预算太小就撞长度
// （finish_reason="length"）且 content 为空。此时把"没答案"判成能力 false 是误判：
// 模型没问题，是探测器预算给小了。于是从最小档起步，撞长度/空正文就抬档重试，
// 直到拿到可用回答或到达上限（**有上限**，不无限抬）。
var probeBudgetLadder = []int{16, 512, 2048}

// usableFn 判定一次 chat 响应是否已能给出结论（能则不再抬预算）。
type usableFn func(chatResponse) bool

// needsBiggerBudget 判定一次响应是否"可能只是预算不够"：撞长度，或没有可用内容。
// 只有这种情况才值得抬预算重试；其它（如 4xx、纯文本拒答）不重试。
func needsBiggerBudget(resp chatResponse) bool {
	if len(resp.Choices) == 0 {
		return false
	}
	ch := resp.Choices[0]
	if len(ch.Message.ToolCalls) > 0 {
		return false
	}
	if strings.EqualFold(ch.FinishReason, "length") {
		return true
	}
	return strings.TrimSpace(ch.Message.Content) == ""
}

// chatWithBudgetRetry 按"先小后大"的预算阶梯发 chat 请求，直到拿到可用回答或到达上限。
// 返回最后一次的 (状态码, 原始响应, 耗时, 网络错误, 实际用掉的预算)。
//
// 只对"HTTP 200 且能解析、但内容因预算不足而不可用"的响应重试；端点报错（4xx/5xx）
// 是"确定的态度"，原样返回给它自己的判据（真不支持照旧记 false）。
func chatWithBudgetRetry(e Endpoint, build func(maxTokens int) map[string]any, usable usableFn) (int, []byte, time.Duration, error, int) {
	var st int
	var data []byte
	var el time.Duration
	var err error
	budget := probeBudgetLadder[0]
	for i := 0; ; i++ {
		st, data, el, err = e.chatCall(build(budget))
		if err != nil {
			return st, data, el, err, budget
		}
		var resp chatResponse
		if json.Unmarshal(data, &resp) == nil && st == http.StatusOK {
			if usable(resp) {
				return st, data, el, nil, budget
			}
			if needsBiggerBudget(resp) && i+1 < len(probeBudgetLadder) {
				budget = probeBudgetLadder[i+1]
				continue
			}
		}
		return st, data, el, nil, budget
	}
}

// chatResponse 是 OpenAI 兼容 chat 响应的最小可解析子集。
type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string            `json:"content"`
			ToolCalls []json.RawMessage `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// chatUsableText 文本探测的"可用回答"判据：有正文，或有工具调用。
func chatUsableText(resp chatResponse) bool {
	if len(resp.Choices) == 0 {
		return false
	}
	m := resp.Choices[0].Message
	return strings.TrimSpace(m.Content) != "" || len(m.ToolCalls) > 0
}

// ProbeText 文本探测器（probe.text.v1）。预算"先小后大"重试（待修补 #27）。
// 通过判据：拿到可用回答（HTTP 200 且 choices[0].message 有 content 或 tool_calls）。
// 到达预算上限仍拿不到可用回答 → 不判 false，改判"不可判定"（budget_exhausted）。
func ProbeText(e Endpoint) runResult {
	build := func(maxTokens int) map[string]any {
		return map[string]any{
			"model": e.Model,
			"messages": []any{
				map[string]any{"role": "user", "content": "你好"},
			},
			"max_tokens": maxTokens,
		}
	}
	st, data, el, err, budget := chatWithBudgetRetry(e, build, chatUsableText)
	tr := Trace{Probe: EvidenceText, HTTPStatus: st, ElapsedMS: el.Milliseconds(), Budget: budget}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		// 超时 = 没探出结论（系统/预算问题），不得写成 false（待修补 #27）。
		if tr.FailureClass == FailTimeout {
			return runResult{FailureClass: FailTimeout, Trace: tr, Undetermined: true}
		}
		return runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	var resp chatResponse
	if json.Unmarshal(data, &resp) != nil || st != 200 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	if len(resp.Choices) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	if chatUsableText(resp) {
		tr.OK = true
		return runResult{true, "", tr, false}
	}
	// 到达预算上限仍拿不到可用回答：这是"没探够"（撞长度/空正文），不是"确定没有"。
	tr.FailureClass = FailBudgetExhausted
	return runResult{FailureClass: FailBudgetExhausted, Trace: tr, Undetermined: true}
}

// ProbeVision 视觉探测器（probe.vision.1x1.v1）——本仓最看重的一项。
// 通过判据：HTTP 200 且响应可解析出正文。失败分类 no_vision / mmproj_missing。
// 绝不因为模型"自称"多模态就写 true：一律实测。预算"先小后大"重试（待修补 #27）。
// 端点明确不支持（非 200）照旧判 false；到达预算上限仍无正文 → 判"不可判定"。
func ProbeVision(e Endpoint) runResult {
	build := func(maxTokens int) map[string]any {
		content := []any{
			map[string]any{"type": "text", "text": "这张图是什么颜色？一句话回答。"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": onePixelPNGDataURL()}},
		}
		return map[string]any{
			"model":      e.Model,
			"messages":   []any{map[string]any{"role": "user", "content": content}},
			"max_tokens": maxTokens,
		}
	}
	usable := func(resp chatResponse) bool {
		if len(resp.Choices) == 0 {
			return false
		}
		m := resp.Choices[0].Message
		return strings.TrimSpace(m.Content) != "" || len(m.ToolCalls) > 0
	}
	st, data, el, err, budget := chatWithBudgetRetry(e, build, usable)
	tr := Trace{Probe: EvidenceVision, HTTPStatus: st, ElapsedMS: el.Milliseconds(), Budget: budget}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		if tr.FailureClass == FailTimeout {
			return runResult{FailureClass: FailTimeout, Trace: tr, Undetermined: true}
		}
		return runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	if st != 200 {
		cls := FailNoVision
		// 结构化主判据是 HTTP 码；"mmproj" 只是把 no_vision 细分出来的提示。
		if strings.Contains(strings.ToLower(string(data)), "mmproj") {
			cls = FailMmprojMiss
		}
		tr.FailureClass = cls
		return runResult{false, cls, tr, false}
	}
	var resp chatResponse
	if json.Unmarshal(data, &resp) != nil || len(resp.Choices) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	if usable(resp) {
		tr.OK = true
		return runResult{true, "", tr, false}
	}
	// 200 但无正文（撞长度/空正文），到达预算上限 → 没探够，不判 false。
	tr.FailureClass = FailBudgetExhausted
	return runResult{FailureClass: FailBudgetExhausted, Trace: tr, Undetermined: true}
}

// ProbeTools 工具调用探测器（probe.tools.v1）。预算"先小后大"重试（待修补 #27）。
// 通过判据：返回结构化 tool_calls。返回正文（明确/默认不支持）记为 tools:false，
// 不算"失败"，失败分类 no_tools 只用于说明原因。
// 到达预算上限仍撞长度/空正文 → 不判 false，改判"不可判定"（budget_exhausted）。
func ProbeTools(e Endpoint) runResult {
	build := func(maxTokens int) map[string]any {
		return map[string]any{
			"model": e.Model,
			"messages": []any{
				map[string]any{"role": "user", "content": "现在几点？请用 get_time 工具查询。"},
			},
			"tools": []any{
				map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":        "get_time",
						"description": "返回当前时间",
						"parameters": map[string]any{
							"type":       "object",
							"properties": map[string]any{"tz": map[string]any{"type": "string", "description": "时区"}},
						},
					},
				},
			},
			"max_tokens": maxTokens,
		}
	}
	usable := func(resp chatResponse) bool {
		if len(resp.Choices) == 0 {
			return false
		}
		m := resp.Choices[0].Message
		// 有 tool_calls = 支持；有正文 = 端点给了明确答复（不支持工具，如实记 false）。
		return len(m.ToolCalls) > 0 || strings.TrimSpace(m.Content) != ""
	}
	st, data, el, err, budget := chatWithBudgetRetry(e, build, usable)
	tr := Trace{Probe: EvidenceTools, HTTPStatus: st, ElapsedMS: el.Milliseconds(), Budget: budget}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		if tr.FailureClass == FailTimeout {
			return runResult{FailureClass: FailTimeout, Trace: tr, Undetermined: true}
		}
		return runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	var resp chatResponse
	if jerr := json.Unmarshal(data, &resp); jerr != nil {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	if st != 200 {
		// 端点在 HTTP 层明确拒绝工具请求 = 真不支持（与"预算不足"区分开）。
		tr.FailureClass = FailNoTools
		return runResult{false, FailNoTools, tr, false}
	}
	if len(resp.Choices) > 0 && len(resp.Choices[0].Message.ToolCalls) > 0 {
		tr.OK = true
		return runResult{true, "", tr, false}
	}
	if usable(resp) {
		// 有正文但没 tool_calls = 端点明确/默认不支持工具（真 false）。
		tr.FailureClass = FailNoTools
		return runResult{false, FailNoTools, tr, false}
	}
	// 到达预算上限仍撞长度/空正文 → 没探够，不判 false。
	tr.FailureClass = FailBudgetExhausted
	return runResult{FailureClass: FailBudgetExhausted, Trace: tr, Undetermined: true}
}

// ProbeEmbedding 嵌入探测器（probe.embedding.v1）。端点不支持 /v1/embeddings 时如实记 false。
func ProbeEmbedding(e Endpoint) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	st, data, el, err := e.do(ctx, http.MethodPost, "/embeddings", map[string]any{"model": e.Model, "input": "hello"})
	tr := Trace{Probe: EvidenceEmbedding, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	if st != 200 {
		tr.FailureClass = FailUnsupported
		if st != 404 && st != 501 && st != 405 {
			// 非"没有这个接口"的报错，也仍如实记 false
			tr.Summary = fmt.Sprintf("端点未提供可用的 /v1/embeddings（HTTP %d）：%s", st, summarize(data))
		} else {
			tr.Summary = fmt.Sprintf("端点未提供 /v1/embeddings（HTTP %d）", st)
		}
		return runResult{false, FailUnsupported, tr, false}
	}
	var resp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil || len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	tr.OK = true
	return runResult{true, "", tr, false}
}

// ProbeRerank 重排探测器（probe.rerank.v1）。端点不支持 /v1/rerank 时如实记 false。
func ProbeRerank(e Endpoint) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	st, data, el, err := e.do(ctx, http.MethodPost, "/rerank", map[string]any{
		"model":     e.Model,
		"query":     "hello",
		"documents": []string{"hello", "world"},
	})
	tr := Trace{Probe: EvidenceRerank, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	if st != 200 {
		tr.FailureClass = FailUnsupported
		if st != 404 && st != 501 && st != 405 {
			tr.Summary = fmt.Sprintf("端点未提供可用的 /v1/rerank（HTTP %d）：%s", st, summarize(data))
		} else {
			tr.Summary = fmt.Sprintf("端点未提供 /v1/rerank（HTTP %d）", st)
		}
		return runResult{false, FailUnsupported, tr, false}
	}
	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	if json.Unmarshal(data, &resp) != nil || len(resp.Results) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr, false}
	}
	tr.OK = true
	return runResult{true, "", tr, false}
}

// ProbeMetaModels 读引擎的 /v1/models（probe.meta.models.v1），返回首个模型 id。
// 端点通常不报上下文档位，故这里主要拿"身份 + 留痕"。
func ProbeMetaModels(e Endpoint) (string, runResult) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	st, data, el, err := e.do(ctx, http.MethodGet, "/models", nil)
	tr := Trace{Probe: EvidenceMetaModels, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return "", runResult{false, tr.FailureClass, tr, false}
	}
	tr.Summary = summarize(data)
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if st != 200 || json.Unmarshal(data, &resp) != nil || len(resp.Data) == 0 || resp.Data[0].ID == "" {
		tr.FailureClass = FailNoMeta
		return "", runResult{false, FailNoMeta, tr, false}
	}
	tr.OK = true
	return resp.Data[0].ID, runResult{true, "", tr, false}
}

// onePixelPNGDataURL 现场生成 1×1 PNG 的 data URL。
// 现场生成而不是写死一段 base64：避免"魔法字符串"说不清来源，也保证是合法 PNG
// （不读外网——开工方案 §四明确要求内嵌，不联网取图）。
func onePixelPNGDataURL() string {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// summarize 取响应前 200 个字符作为留痕（开工方案 §四）。
func summarize(b []byte) string {
	return truncate(strings.TrimSpace(string(b)), 200)
}

// truncate 按字符（非字节）截断，避免把多字节字符切坏。
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
