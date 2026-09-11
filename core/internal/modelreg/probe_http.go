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
	BaseURL string        // 形如 http://127.0.0.1:9000 或 http://host:port/v1
	Model   string        // 请求体里的 model 字段（引擎多会忽略）
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

// chatResponse 是 OpenAI 兼容 chat 响应的最小可解析子集。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string            `json:"content"`
			ToolCalls []json.RawMessage `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ProbeText 文本探测器（probe.text.v1）。
// 通过判据：HTTP 200 且能解析出 choices[0].message 的 content 或 tool_calls。
func ProbeText(e Endpoint) runResult {
	body := map[string]any{
		"model": e.Model,
		"messages": []any{
			map[string]any{"role": "user", "content": "你好"},
		},
		"max_tokens": 16,
	}
	st, data, el, err := e.chatCall(body)
	tr := Trace{Probe: EvidenceText, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return runResult{false, tr.FailureClass, tr}
	}
	tr.Summary = summarize(data)
	var resp chatResponse
	if json.Unmarshal(data, &resp) != nil || st != 200 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	if len(resp.Choices) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	if strings.TrimSpace(resp.Choices[0].Message.Content) == "" && len(resp.Choices[0].Message.ToolCalls) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	tr.OK = true
	return runResult{true, "", tr}
}

// ProbeVision 视觉探测器（probe.vision.1x1.v1）——本仓最看重的一项。
// 通过判据：HTTP 200 且响应可解析出 message。失败分类 no_vision / mmproj_missing。
// 绝不因为模型"自称"多模态就写 true：一律实测。
func ProbeVision(e Endpoint) runResult {
	content := []any{
		map[string]any{"type": "text", "text": "这张图是什么颜色？一句话回答。"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": onePixelPNGDataURL()}},
	}
	body := map[string]any{
		"model":      e.Model,
		"messages":   []any{map[string]any{"role": "user", "content": content}},
		"max_tokens": 16,
	}
	st, data, el, err := e.chatCall(body)
	tr := Trace{Probe: EvidenceVision, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return runResult{false, tr.FailureClass, tr}
	}
	tr.Summary = summarize(data)
	if st != 200 {
		cls := FailNoVision
		// 结构化主判据是 HTTP 码；"mmproj" 只是把 no_vision 细分出来的提示。
		if strings.Contains(strings.ToLower(string(data)), "mmproj") {
			cls = FailMmprojMiss
		}
		tr.FailureClass = cls
		return runResult{false, cls, tr}
	}
	var resp chatResponse
	if json.Unmarshal(data, &resp) != nil || len(resp.Choices) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	tr.OK = true
	return runResult{true, "", tr}
}

// ProbeTools 工具调用探测器（probe.tools.v1）。
// 通过判据：返回结构化 tool_calls。返回文本（明确或默认不支持）记为 tools:false，
// 不算"失败"，失败分类 no_tools 只用于说明原因。
func ProbeTools(e Endpoint) runResult {
	body := map[string]any{
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
		"max_tokens": 64,
	}
	st, data, el, err := e.chatCall(body)
	tr := Trace{Probe: EvidenceTools, HTTPStatus: st, ElapsedMS: el.Milliseconds()}
	if err != nil {
		tr.FailureClass = classifyNetErr(err)
		tr.Summary = truncate(err.Error(), 200)
		return runResult{false, tr.FailureClass, tr}
	}
	tr.Summary = summarize(data)
	var resp chatResponse
	if jerr := json.Unmarshal(data, &resp); jerr != nil {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	if st != 200 {
		tr.FailureClass = FailNoTools
		return runResult{false, FailNoTools, tr}
	}
	if len(resp.Choices) > 0 && len(resp.Choices[0].Message.ToolCalls) > 0 {
		tr.OK = true
		return runResult{true, "", tr}
	}
	tr.FailureClass = FailNoTools
	return runResult{false, FailNoTools, tr}
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
		return runResult{false, tr.FailureClass, tr}
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
		return runResult{false, FailUnsupported, tr}
	}
	var resp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil || len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	tr.OK = true
	return runResult{true, "", tr}
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
		return runResult{false, tr.FailureClass, tr}
	}
	tr.Summary = summarize(data)
	if st != 200 {
		tr.FailureClass = FailUnsupported
		if st != 404 && st != 501 && st != 405 {
			tr.Summary = fmt.Sprintf("端点未提供可用的 /v1/rerank（HTTP %d）：%s", st, summarize(data))
		} else {
			tr.Summary = fmt.Sprintf("端点未提供 /v1/rerank（HTTP %d）", st)
		}
		return runResult{false, FailUnsupported, tr}
	}
	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	if json.Unmarshal(data, &resp) != nil || len(resp.Results) == 0 {
		tr.FailureClass = FailBadFormat
		return runResult{false, FailBadFormat, tr}
	}
	tr.OK = true
	return runResult{true, "", tr}
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
		return "", runResult{false, tr.FailureClass, tr}
	}
	tr.Summary = summarize(data)
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if st != 200 || json.Unmarshal(data, &resp) != nil || len(resp.Data) == 0 || resp.Data[0].ID == "" {
		tr.FailureClass = FailNoMeta
		return "", runResult{false, FailNoMeta, tr}
	}
	tr.OK = true
	return resp.Data[0].ID, runResult{true, "", tr}
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
