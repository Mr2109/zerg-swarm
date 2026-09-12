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
	"sort"
	"strings"
	"time"
)

// ── 在线探测器（text / vision / tools / embedding / rerank + 端点 meta）──────
//
// 判据遵循开工方案 §八 风险1：只认结构化信号（HTTP 码 + 能否解析出字段 +
// 引擎明确的能力声明），不拿文案字符串当通过条件——**没有任何例外**
// （视觉失败的归因见 ProbeVision；待修补 #23 已删掉"响应体出现 mmproj 就升级"的文案匹配）。

// Endpoint 是一个 OpenAI 兼容引擎端点。
type Endpoint struct {
	BaseURL string // 形如 http://127.0.0.1:9000 或 http://host:port/v1
	Model   string // 请求体里的 model 字段（引擎多会忽略）
	Timeout time.Duration
	Client  *http.Client

	// ModelDir/WeightsName 是本地权重文件所在目录与文件名（可空）。它们只为一条结构化信号
	// 服务（待修补 #23）：同目录是否放着**本模型**的视觉投影器建材（mmproj*.gguf，按模型名
	// token 配对），用于把"引擎声明无视觉"细分为 mmproj_missing 还是 no_vision。
	// 空（如只给 URL 探测）→ 该项无从判断（宁判 no_vision，不误报 mmproj_missing）。
	ModelDir    string
	WeightsName string
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
//
// 归因（待修补 #23）：失败时**只认结构化信号**，绝不匹配响应体文案。
//   - 引擎能力声明（/props.modalities.vision，退一步 /v1/models 的 capabilities 含
//     "multimodal"）说明**当前有没有可用的视觉**；
//   - 本地投影器建材（同目录 mmproj*.gguf，见 projectorPresent）说明**投影器在不在本地**。
//
// 两条信号齐全才能判 mmproj_missing（引擎声明无视觉 + 本地确有投影器建材却没被装载）；
// 引擎声明无视觉但本地没有投影器建材 → no_vision；引擎完全没有结构化声明 → unverifiable
// （缺=未知，绝不按文案升级、也绝不降级成 value=false）。
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
		// 非 200：归因**只认结构化信号**（待修补 #23）——先问引擎的结构化能力声明，
		// 再看本地有没有投影器建材；响应体文案一律不看。
		sig := probeVisionCapSignal(e)
		base := tr.Summary
		if sig.Known {
			tr.Summary = truncate(fmt.Sprintf("%s | 引擎声明 %s=%t", base, sig.Anchor, sig.Active), 200)
			if sig.Active {
				// 引擎声明视觉可用，图像请求却失败 → 不是投影器缺失。
				tr.FailureClass = FailNoVision
				return runResult{false, FailNoVision, tr, false}
			}
			if name, ok := projectorPresent(e.ModelDir, e.WeightsName); ok {
				tr.FailureClass = FailMmprojMiss
				tr.Summary = truncate(fmt.Sprintf("%s | 引擎声明 %s=false；本地投影器建材 %s 未被装载", base, sig.Anchor, name), 200)
				return runResult{false, FailMmprojMiss, tr, false}
			}
			// 引擎声明无视觉，且本地没有投影器建材可归因 → no_vision。
			tr.FailureClass = FailNoVision
			return runResult{false, FailNoVision, tr, false}
		}
		// 引擎没给结构化声明：4xx 是引擎对请求的明确拒绝（结构化 HTTP 码，照旧判 false，
		// 守住 #27 的区分点）；其余（5xx 等）无法归因 → 不可判定，绝不按文案升级（#23 反例①）。
		if st >= 400 && st < 500 {
			tr.FailureClass = FailNoVision
			return runResult{false, FailNoVision, tr, false}
		}
		tr.FailureClass = FailNoStructuredSignal
		tr.Summary = truncate(fmt.Sprintf("%s | 引擎 HTTP %d 且未提供结构化信号（/props.modalities.vision 与 /v1/models capabilities 均无）", base, st), 200)
		return runResult{FailureClass: FailNoStructuredSignal, Trace: tr, Undetermined: true}
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

// ── 待修补 #23：视觉失败归因用的结构化信号 ────────────────────────────────
//
// 旧实现把"响应体里出现 mmproj 字样"当作 mmproj_missing 的判据，违反开工方案 §八
// 风险1「只认结构化信号」（引擎文案一变就误判）。本机实测（llama.cpp build 10470，
// 型号 example-35b-v2 + mmproj-example-35b-v2-BF16 与纯文本 gemma-4-26B）可用的信号：
//
//	信号                                    挂 mmproj     VLM 未挂 mmproj   纯文本模型
//	GET /props → modalities.vision           true          false             false
//	GET /v1/models → models[0].capabilities  [completion,  [completion]      [completion]
//	                                         multimodal]
//	视觉请求 HTTP 码                          200           500               500
//	视觉请求响应体                            正常回答      同一条文案         同一条文案
//
// 结论：引擎侧信号只回答"当前有没有可用的视觉"，**区分不了** VLM 未挂 mmproj 与纯文本
// 模型（两者逐字节相同）。要判 mmproj_missing 必须再加一条模型侧信号：本地同目录有没有
// mmproj*.gguf（projectorPresent）。两条都不成立 → 不可判定，不猜。

// visionCapSignal 是引擎给出的结构化"视觉能力声明"（替代旧的响应体文案匹配）。
type visionCapSignal struct {
	Known  bool   // 引擎是否给出了结构化声明
	Active bool   // 声明里当前视觉是否可用
	Anchor string // 证据锚（具体字段名），如 /props.modalities.vision
}

// probeVisionCapSignal 读引擎的只读结构化能力声明。顺序：
//  1. GET /props        → JSON 对象字段 modalities.vision（布尔）
//  2. GET /v1/props     → 同上（个别实现把属性接口挂在 /v1 下）
//  3. GET /v1/models    → models[0].capabilities 数组是否含 "multimodal"
//
// 三步都不成立 → Known=false（缺=未知，绝不猜、绝不匹配文案）。
func probeVisionCapSignal(e Endpoint) visionCapSignal {
	if sig, ok := propsVisionSignal(e, "/props", e.rootURL("/props")); ok {
		return sig
	}
	if sig, ok := propsVisionSignal(e, "/v1/props", e.url("/props")); ok {
		return sig
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	st, data, _, err := e.do(ctx, http.MethodGet, "/models", nil)
	if err == nil && st == http.StatusOK {
		if sig, ok := modelsVisionSignal(data); ok {
			return sig
		}
	}
	return visionCapSignal{}
}

// propsVisionSignal 从只读"属性"接口取 modalities.vision（结构化布尔字段）。
// 判据是**结构化字段类型**（JSON 对象 → modalities 对象 → vision 布尔），不做文案匹配。
func propsVisionSignal(e Endpoint, displayPath, rawURL string) (visionCapSignal, bool) {
	st, data, _, err := e.getAbs(rawURL)
	if err != nil || st != http.StatusOK {
		return visionCapSignal{}, false
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return visionCapSignal{}, false
	}
	mods, ok := m["modalities"].(map[string]any)
	if !ok {
		return visionCapSignal{}, false
	}
	v, ok := mods["vision"].(bool)
	if !ok {
		return visionCapSignal{}, false
	}
	return visionCapSignal{Known: true, Active: v, Anchor: displayPath + ".modalities.vision"}, true
}

// modelsVisionSignal 从 /v1/models 的 models[].capabilities 数组判定"是否声明多模态"。
// 数组存在即视为一次结构化声明：含 "multimodal" → 有视觉；不含 → 没有视觉。
// 数组缺失（老式 OpenAI 响应只有 data[]）→ Known=false（缺=未知）。
func modelsVisionSignal(data []byte) (visionCapSignal, bool) {
	var resp struct {
		Models []struct {
			Capabilities []string `json:"capabilities"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &resp) != nil || len(resp.Models) == 0 || resp.Models[0].Capabilities == nil {
		return visionCapSignal{}, false
	}
	for _, c := range resp.Models[0].Capabilities {
		if strings.EqualFold(strings.TrimSpace(c), "multimodal") {
			return visionCapSignal{Known: true, Active: true, Anchor: "/v1/models.models[0].capabilities"}, true
		}
	}
	return visionCapSignal{Known: true, Active: false, Anchor: "/v1/models.models[0].capabilities"}, true
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

// ── probe.template.v1 的 from_tokenizer 路径：从端点只读元信息取 chat_template ──────
//
// 待修补 #22：模板来源必须"真探测"。llama.cpp 的服务器把 chat_template 挂在只读 GET /props
// 上（根路径，不在 /v1 下）；部分 OpenAI 兼容服务把它附在 /v1/models 的模型对象里。
// 只发 GET、不改端点状态、不加载权重；拿不到就如实说不知道（缺=未知，绝不猜）。

// endpointTemplate 是端点只读元信息里读到的 chat_template 及其证据锚。
type endpointTemplate struct {
	OK       bool
	Anchor   string // 证据锚（具体端点字段名），如 /props.chat_template
	Template string // 读到的模板原文（仅用于断言/留痕，不落记录正文）
}

// rootURL 拼端点根路径（llama.cpp 的 /props 挂在根，不在 /v1 下）。
func (e Endpoint) rootURL(path string) string {
	base := strings.TrimRight(e.BaseURL, "/")
	base = strings.TrimSuffix(base, "/v1")
	return base + path
}

// getAbs 对绝对 URL 发 GET（用于根路径 /props 这类不长在 /v1 下的只读接口）。
func (e Endpoint) getAbs(rawURL string) (int, []byte, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, 0, err
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

// ProbeChatTemplateFromEndpoint 从端点的只读元信息接口尝试取 chat_template（probe.template.v1
// 的 from_tokenizer 路径，待修补 #22）。顺序：/props → /v1/props → /v1/models 里键名含
// "template" 的非空字符串字段。全拿不到 → OK=false（缺=未知，绝不猜）。
func ProbeChatTemplateFromEndpoint(e Endpoint) endpointTemplate {
	// ① llama.cpp 服务器属性（根路径）/props
	if et := probePropsTemplate(e, "/props", e.rootURL("/props")); et.OK {
		return et
	}
	// ② 个别实现把属性接口挂在 /v1 下
	if et := probePropsTemplate(e, "/v1/props", e.url("/props")); et.OK {
		return et
	}
	// ③ 退一步：/v1/models 的模型对象里带模板字段
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()
	st, data, _, err := e.do(ctx, http.MethodGet, "/models", nil)
	if err == nil && st == http.StatusOK {
		if anchor, tmpl := templateFieldFromModels(data); tmpl != "" {
			return endpointTemplate{OK: true, Anchor: anchor, Template: tmpl}
		}
	}
	return endpointTemplate{}
}

// probePropsTemplate 向一个只读"属性"接口发 GET，取其中的 chat_template 字段。
// displayPath 仅用于生成证据锚（如 /props → /props.chat_template）。
// 判据是**结构化字段**（JSON 里 chat_template 为非空字符串），不做文案匹配。
func probePropsTemplate(e Endpoint, displayPath, rawURL string) endpointTemplate {
	st, data, _, err := e.getAbs(rawURL)
	if err != nil || st != http.StatusOK {
		return endpointTemplate{}
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return endpointTemplate{}
	}
	s, _ := m["chat_template"].(string)
	if strings.TrimSpace(s) == "" {
		return endpointTemplate{}
	}
	return endpointTemplate{OK: true, Anchor: displayPath + ".chat_template", Template: s}
}

// templateFieldFromModels 在 /v1/models 响应里找键名含 "template" 的非空字符串字段。
// 键名排序后取第一个，保证同一响应得到相同的证据锚（可复现）。
func templateFieldFromModels(data []byte) (anchor, template string) {
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return "", ""
	}
	for i, m := range resp.Data {
		keys := make([]string, 0, len(m))
		for k := range m {
			if strings.Contains(strings.ToLower(k), "template") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			s, ok := m[k].(string)
			if !ok || strings.TrimSpace(s) == "" {
				continue
			}
			return fmt.Sprintf("/v1/models.data[%d].%s", i, k), s
		}
	}
	return "", ""
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
