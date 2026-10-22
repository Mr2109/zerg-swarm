// summarizer_http.go — P3 的**生产适配器**：把 SummarizerFunc 接到 OpenAI 兼容的
// `POST {endpoint}/v1/chat/completions` 上（网关 / llama-server / DS4 都是这条面）。
//
// 为什么在本文件里写一份 wire 结构，而不是 import chat/gateway：
//   - `compress` 的包契约（policy.go 包注释）是「只 import stdlib、不认识网关的任何类型」，
//     适配器必须待在契约内，否则压缩层被拖上 sqlite/onnx 一整套依赖；
//   - chat.CompactRequest 与 gateway 里都是**函数内匿名结构**（没有可复用的导出类型），
//     所以这里按**同一形状**落一份最小 wire：`{model, messages[{role,content}], max_tokens, temperature}`
//     → `{choices[{message{content}, finish_reason}], usage, error}`。字段名逐字对齐
//     chat.CompactRequest（model / messages / max_tokens / temperature；温度缺省 0.3 亦与它一致），
//     换端点不需要改这边。
//
// 四条硬约束（本文件的存在理由）：
//  1. **超时**：每次调用都套 context.WithTimeout（缺省 120s），与 Policy.Deadline 正交；
//  2. **单次输出预算**：缺省 1200 token（EstimateTokens 口径），超出即截断并标 Truncated
//     （文本尾部带截断标记，Stats() 里可查；finish_reason=="length" 也算截断）；
//  3. **失败不回传原文**：任何非 200 / JSON 坏 / 空 content / 上游 error 都返回 error 与 nil 片段，
//     **绝不用 prompt 兜底**（否则「摘要」变成把输入原样塞回去）；
//  4. **不回显密钥**：token 只出现在请求头里；错误串与截取的响应体一律过 redact()，
//     本文件不写任何日志（纯库，不碰 log 包 / 不碰 stdout）。
//
// 纯注入：本文件**不读环境变量**（端点 / 模型 / 令牌全部由调用方传进来）。
package compress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultSummarizerTimeout —— 单次摘要调用上限（与 DefaultPolicy 的 Deadline 同量级，
	// 但语义不同：Policy.Deadline 管整轮压缩，这里管一次 HTTP）。
	DefaultSummarizerTimeout = 120 * time.Second
	// DefaultSummarizerMaxTokens —— 单次输出预算：摘要块超过它就截断并标 Truncated。
	DefaultSummarizerMaxTokens = 1200
	// defaultSummarizerTemperature —— 与 chat.CompactRequest 的 0.3 对齐（摘要是复述，不是创作）。
	defaultSummarizerTemperature = 0.3
	// summarizerErrSnippetRunes —— 错误里最多带多少字的上游响应体（仍过 redact）。
	summarizerErrSnippetRunes = 200
	// summarizerMaxRespBytes —— 响应体读入上限，防上游吐巨物把内存打穿。
	summarizerMaxRespBytes = 4 << 20
	// summarizerTruncatedMark —— 截断标记：进文本（下游看得见），同时进 Stats。
	summarizerTruncatedMark = " …〔已按输出预算截断〕"
)

// summarizerSystemPrompt —— 摘要器的系统提示。要点：只输出正文、路径/命令/标识符逐字保留、
// 不新增事实。消费者若要换模板，请在外面包一层 SummarizerFunc（本适配器只管传输与预算）。
const summarizerSystemPrompt = `你是上下文压缩器。把给定的若干片段改写成一段更短的结构化中文摘要。
硬性要求：
1. 只输出摘要正文，不要解释、不要前后缀、不要代码块围栏；
2. 文件路径 / 命令 / 标识符 / 数字必须逐字保留，不得改写或翻译；
3. 不新增输入里没有的事实；不确定就省略，不要猜；
4. 篇幅控制在输入的 1/5 以内，优先保留结论、未完成的待办与失败原因。`

// HTTPSummarizerConfig —— 适配器的全部输入（纯值，调用方装配；本包不读 env）。
type HTTPSummarizerConfig struct {
	// Endpoint 端点基址或完整 URL：`http://127.0.0.1:8100` 会自动补 `/v1/chat/completions`；
	// 以 `/v1` 结尾则补 `/chat/completions`；已带完整路径则原样使用。
	Endpoint string
	Model    string // 必填（空 ⇒ 构造失败：宁可启动时炸，也不要每次调用烧一次 400）
	// Token 认证令牌。只写进请求头（Authorization: Bearer ＋ X-Auth-Token，两者都带，
	// 兼容 OpenAI 面与网关面），**绝不**进日志 / 错误串 / Stats。
	Token string
	// Timeout 单次调用超时；≤0 ⇒ DefaultSummarizerTimeout(120s)。
	Timeout time.Duration
	// MaxTokens 单次输出预算；≤0 ⇒ DefaultSummarizerMaxTokens(1200)。
	// ★ 装配时须保证它**明显小于**一批输入的 token 估算，否则 SummaryEngine 的
	// 「必须真的变小」硬闸会判失败并退回 free（白烧一次调用）。适配器不替调用方猜比例。
	MaxTokens int
	// Temperature 采样温度；nil ⇒ 0.3（显式给 0 表示确定性解码，不会被默认值吃掉）。
	Temperature *float64
	// HTTPClient 可注入（测试用 httptest 的 client）；nil ⇒ 新建带 Timeout 的默认 client。
	// 注入了也仍受 Timeout 约束（ctx 上另有一道 deadline）。
	HTTPClient *http.Client
}

// SummarizeStats —— 适配器的只读运行计数（审计 / 断言用；不含任何 prompt 或令牌）。
type SummarizeStats struct {
	Calls            int    // 成功返回片段的调用数
	TruncatedCalls   int    // 其中因输出预算被截断的
	TimeoutCalls     int    // 因超时/取消失败的
	FailedCalls      int    // 其余失败（非 200 / 坏 JSON / 空内容）
	LastTruncated    bool   // 最近一次是否截断
	LastOutputTokens int    // 最近一次返回文本的 token 估算（截断后）
	LastFinishReason string // 最近一次 finish_reason（"length" 常见于截断）
	LastModel        string // 最近一次响应自报的模型（可能为空）
}

// 请求 / 响应 wire —— 与 chat.CompactRequest、gateway 的 /v1/chat/completions 同形（见文件头）。
type summarizerMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type summarizerRequest struct {
	Model       string              `json:"model"`
	Messages    []summarizerMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
}

type summarizerResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// HTTPSummarizer —— SummarizerFunc 的生产实现（可并发复用：client 与 Stats 都安全）。
type HTTPSummarizer struct {
	endpoint    string // 已归一化的完整 URL（仅供建请求；不进错误串）
	model       string
	token       string
	timeout     time.Duration
	maxTokens   int
	temperature float64
	client      *http.Client

	mu    sync.Mutex
	stats SummarizeStats
}

// NewHTTPSummarizer —— 构造并**校验**配置；配置错在装配期暴露，不拖到每次调用。
func NewHTTPSummarizer(cfg HTTPSummarizerConfig) (*HTTPSummarizer, error) {
	ep, err := normalizeSummarizerEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("compress: 摘要端点缺少 model")
	}
	h := &HTTPSummarizer{
		endpoint:    ep,
		model:       cfg.Model,
		token:       cfg.Token,
		timeout:     cfg.Timeout,
		maxTokens:   cfg.MaxTokens,
		temperature: defaultSummarizerTemperature,
		client:      cfg.HTTPClient,
	}
	if h.timeout <= 0 {
		h.timeout = DefaultSummarizerTimeout
	}
	if h.maxTokens <= 0 {
		h.maxTokens = DefaultSummarizerMaxTokens
	}
	if cfg.Temperature != nil {
		h.temperature = *cfg.Temperature
	}
	if h.client == nil {
		h.client = &http.Client{Timeout: h.timeout}
	}
	return h, nil
}

// Func —— 取注入用的函数值：`p.Summarize = h.Func()`（Policy.Summarize 的唯一入口）。
func (h *HTTPSummarizer) Func() SummarizerFunc { return h.Summarize }

// Stats —— 快照（并发安全）。
func (h *HTTPSummarizer) Stats() SummarizeStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

// Summarize —— 实现 SummarizerFunc：片段 → prompt → 端点 → 一个 representation 片段。
//
// 失败语义（约束 ③）：error ＋ nil，**永不**把 prompt 当结果返回；调用方（SummaryEngine）
// 据此退回 free 结果并标 Fallback。
func (h *HTTPSummarizer) Summarize(ctx context.Context, segs []Segment) ([]Segment, error) {
	if len(segs) == 0 {
		return nil, fmt.Errorf("compress: 摘要器收到空批次")
	}

	payload, err := json.Marshal(summarizerRequest{
		Model:       h.model,
		Messages:    summarizerMessages(segs),
		MaxTokens:   h.maxTokens,
		Temperature: h.temperature,
	})
	if err != nil {
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要请求编码失败：%v", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, h.endpoint, bytes.NewReader(payload))
	if err != nil {
		// url 错误可能带回完整 URL（含 query 里的凭据）⇒ 过 redact 再造串
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要请求构造失败：%s", h.redact(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
		req.Header.Set("X-Auth-Token", h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		// ★ 不 wrap 原始 err：*url.Error 自带 "Post \"http://…<token>…\"" 全文。
		//   只保留脱敏后的文本 + ctx 的哨兵错误（errors.Is(err, context.DeadlineExceeded) 仍可用）。
		timedOut := reqCtx.Err() == context.DeadlineExceeded
		if timedOut {
			h.countTimeout()
			return nil, fmt.Errorf("compress: 摘要请求超时（>%s）：%w", h.timeout, context.DeadlineExceeded)
		}
		if cerr := reqCtx.Err(); cerr != nil {
			h.countFail()
			return nil, fmt.Errorf("compress: 摘要请求取消：%w", cerr)
		}
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要请求失败：%s", h.redact(err.Error()))
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, summarizerMaxRespBytes))
	if readErr != nil {
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要响应读取失败：%s", h.redact(readErr.Error()))
	}

	if resp.StatusCode != http.StatusOK {
		// 上游 5xx 常把请求体（= prompt / 令牌）回显进错误体 ⇒ 截短 + 脱敏；若仍像请求回显则整段隐去。
		snippet := h.snippetOrMasked(string(body), segs)
		h.countFail()
		if snippet == "" {
			return nil, fmt.Errorf("compress: 摘要端点返回 HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("compress: 摘要端点返回 HTTP %d：%s", resp.StatusCode, snippet)
	}

	var out summarizerResponse
	if err := json.Unmarshal(body, &out); err != nil {
		// 不 dump 响应体：坏 JSON 经常是上游把请求原样吐回（含 prompt 与令牌）。
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要响应不是合法 JSON（%d 字节）", len(body))
	}
	if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要端点报错：%s",
			h.redact(trimRunes(strings.TrimSpace(out.Error.Message), summarizerErrSnippetRunes)))
	}
	if len(out.Choices) == 0 {
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要端点未返回 choices")
	}

	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		// ★ 约束 ③ 的正面表述：空结果按失败处理，绝不拿 prompt 顶上。
		h.countFail()
		return nil, fmt.Errorf("compress: 摘要端点返回空内容（不以上下文原文兜底）")
	}

	cut, truncated := markTruncated(text, h.maxTokens)
	if out.Choices[0].FinishReason == "length" {
		truncated = true
	}

	h.mu.Lock()
	h.stats.Calls++
	h.stats.LastTruncated = truncated
	h.stats.LastOutputTokens = EstimateTokens(cut)
	h.stats.LastFinishReason = out.Choices[0].FinishReason
	h.stats.LastModel = out.Model
	if truncated {
		h.stats.TruncatedCalls++
	}
	h.mu.Unlock()

	return []Segment{{
		Kind: KindRepresentation,
		Text: cut,
		// Tokens / Source 留给 SummaryEngine 统一补（它按 run[0].Source 归属并重算 token）。
	}}, nil
}

// ---- 内部件 ----

func (h *HTTPSummarizer) countFail() {
	h.mu.Lock()
	h.stats.FailedCalls++
	h.mu.Unlock()
}

func (h *HTTPSummarizer) countTimeout() {
	h.mu.Lock()
	h.stats.TimeoutCalls++
	h.stats.FailedCalls++
	h.mu.Unlock()
}

// redact —— 抹掉令牌（以及 Bearer 形状），错误串 / 响应片段出厂前必过这一道。
func (h *HTTPSummarizer) redact(s string) string {
	if h.token == "" || s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "Bearer "+h.token, "Bearer ***")
	return strings.ReplaceAll(s, h.token, "***")
}

// snippetOrMasked —— 失败响应体的处理：**先在全文上判回显** → 截短 → 脱敏。
//
// 为什么在全文而不是截断后的片段上判：回显通常把整个 request body（含 messages）贴回来，
// 用户正文往往落在前 200 字之后 —— 只看片段会漏判，于是错误串里就带着 prompt 扬长而去。
func (h *HTTPSummarizer) snippetOrMasked(body string, segs []Segment) string {
	if echoesPrompt(body, segs) {
		return "（上游响应体疑似回显请求全文，已隐去）"
	}
	return h.redact(trimRunes(strings.TrimSpace(body), summarizerErrSnippetRunes))
}

// echoesPrompt —— 响应体里出现「system 提示前缀」或「任一片段正文的 ≥16 字前缀」⇒ 判为回显。
func echoesPrompt(body string, segs []Segment) bool {
	const probe = 16
	if rs := []rune(summarizerSystemPrompt); len(rs) >= probe && strings.Contains(body, string(rs[:probe])) {
		return true
	}
	for i := range segs {
		rs := []rune(strings.TrimSpace(segs[i].Text))
		if len(rs) < probe {
			continue
		}
		if strings.Contains(body, string(rs[:probe])) {
			return true
		}
	}
	return false
}

// normalizeSummarizerEndpoint —— 归一化端点：基址补 /v1/chat/completions，已带路径则原样。
func normalizeSummarizerEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("compress: 摘要端点为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("compress: 摘要端点不是合法 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("compress: 摘要端点必须是 http(s)")
	}
	if u.Host == "" {
		return "", fmt.Errorf("compress: 摘要端点缺少主机名")
	}
	p := strings.TrimRight(u.Path, "/")
	switch {
	case p == "":
		u.Path = "/v1/chat/completions"
	case strings.HasSuffix(p, "/v1/chat/completions"), strings.HasSuffix(p, "/chat/completions"):
		u.Path = p
	case strings.HasSuffix(p, "/v1"):
		u.Path = p + "/chat/completions"
	default:
		u.Path = p // 调用方给了完整（可能是自定义）路径，尊重它
	}
	return u.String(), nil
}

// summarizerMessages —— 片段 → chat messages：system 定型 + 一条 user 装全文。
//
// 为什么不用多轮 role 复刻原对话：摘要器只负责「压成一段」，粒度由一个 user 块控制，
// 模型不会误以为自己在续写对话（gateway / chat 的 6 段模板同理，都是拼成单块再送）。
func summarizerMessages(segs []Segment) []summarizerMessage {
	var sb strings.Builder
	sb.Grow(1024)
	for i := range segs {
		kind := string(segs[i].Kind)
		if kind == "" {
			kind = string(KindTrajectory)
		}
		fmt.Fprintf(&sb, "### 片段 %d（%s）\n", i+1, kind)
		sb.WriteString(segs[i].Text)
		sb.WriteString("\n")
		if len(segs[i].Anchors) > 0 {
			fmt.Fprintf(&sb, "（须逐字保留的锚点：%s）\n", strings.Join(segs[i].Anchors, " · "))
		}
	}
	return []summarizerMessage{
		{Role: "system", Content: summarizerSystemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

// markTruncated —— 按输出预算截断（EstimateTokens 口径）并挂可见标记。
//
// 预算含标记本身的体积；预算小到连标记都放不下时只截正文（仍报 Truncated）。
func markTruncated(s string, budget int) (string, bool) {
	if budget <= 0 {
		return s, false
	}
	if EstimateTokens(s) <= budget {
		return s, false
	}
	room := budget - EstimateTokens(summarizerTruncatedMark)
	if room < 1 {
		return cutToTokens(s, budget), true
	}
	return cutToTokens(s, room) + summarizerTruncatedMark, true
}

// cutToTokens —— 二分出最长的、EstimateTokens ≤ budget 的前缀（按 rune 切，不裂字）。
func cutToTokens(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	if EstimateTokens(s) <= budget {
		return s
	}
	rs := []rune(s)
	lo, hi := 0, len(rs)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if EstimateTokens(string(rs[:mid])) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(rs[:lo])
}

// trimRunes —— 按字符截断（错误片段用；不裂 multibyte）。
func trimRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
