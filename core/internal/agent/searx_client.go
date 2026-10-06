package agent

// searx_client.go — A2 步2：searxng **Go 客户端**（把「进程内导入 searxng 的 Python 桥」换成
// 对**外部 searxng 服务**的一次 HTTP 查询）。设计真源：
// `~/.zerg/state/设计-A2-searxng外置-20261006.md` §二 2.2（件名 / 结构 / 查询面 / 错误面逐字）。
//
// 为什么是 HTTP 客户端（不是又一份桥）
// ------------------------------------
//   · 老桥 `searx_search.py` 用 `sys.path.insert` 把 `vendor/searxng` 当库**进程内导入** ⇒ 依赖第三方
//     组件进仓、子进程无超时（A-06 另路治）。外置后 searxng 是本机 launchd 常驻服务（决-04），仓内
//     只留这一份 Go 客户端。老铁律「不走端口」已由 Mr2109 明示作废（决-05 · 缺口 `GAP-20261007-02`）。
//   · ★ **与桥的语义差异＝本件的核心价值**（正对 A2 缺陷① / `X-26` 的 fail-open）：老桥把「服务不可达」
//     与「真无结果」都回 `{"results": []}`（Python 侧全失败也 `print` 空集）⇒ 两者不可分，用户看到的是
//     「（无搜索结果）」而非「服务不可达」。HTTP 客户端天然可分：
//       - 连不上 / 非 200 / JSON 坏  ⇒ 返回 `error`（服务不可达 · 读不到）；
//       - 200 且 `{"results": []}`   ⇒ 返回空结果 **且 `err == nil`**（真无结果）。
//     判据：**空结果不是 error，服务不可达才是 error**（`X-26` 的 Q4-B 档升为验收负控）。
//
// 本件**只加件**
// --------------
// 非 searxng 的纯 Go 逻辑（`rewriteQuery` / `buildSiteOp` / `llmRewriteQuery` / 搜索缓存 / `FetchTop`）
// 仍留 `net_tools.go`，本件**不动它一个字节**（A2 步3 才切 `net_tools.go`）。故本件落盘后
// `net_tools.go` 的 sha16 必须与落盘前逐字相同（「`net_tools` 行为一字不动」的机检落点）。
//
// 取址：env `ZERG_SEARXNG_URL`（默认 `http://127.0.0.1:8888`，对齐仓外现例 `~/zerg-searxng`，见设计 §二 2.1）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// searxDefaultBaseURL —— searxng 服务默认地址（端口 8888 对齐仓外现例 `~/zerg-searxng`，设计 §二 2.1）。
	searxDefaultBaseURL = "http://127.0.0.1:8888"
	// searxDefaultTimeout —— 单次 HTTP 查询上限（对齐同件 `net_tools.go` 的 WebFetch 10s 网络口径，✗ 不自创大数字）。
	searxDefaultTimeout = 10 * time.Second
	// searxEnvBaseURL —— 服务地址的环境变量名（**替代**桥时代的 `ZERG_SEARXNG_PY` / `SETTINGS` / `SRC`，设计 §二 2.2）。
	searxEnvBaseURL = "ZERG_SEARXNG_URL"
	// searxDefaultRetries —— 失败重试次数（对齐 `net_tools.go` 桥调用「失败重试一次」的引擎抖动容错）。
	searxDefaultRetries = 1
)

// SearxQuery —— 一次搜索的入参（对齐设计 §二 2.2 的 `SearxQuery`）。
type SearxQuery struct {
	Query     string   // 搜索词
	Limit     int      // 结果数上限（<=0 ⇒ 不截断，由调用方定）
	Lang      string   // 语言 all/zh/en…（空 ⇒ all）
	TimeRange string   // 时间过滤 day/week/month/year（空 ⇒ 不限）
	Domains   []string // 限定域名（`site:` 操作符，复用同件 `buildSiteOp`）
}

// SearxResult —— searxng 单条结果（把 `net_tools.go` 桥内的匿名 struct 升为具名；多一个 `Score`）。
type SearxResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// SearxClient —— searxng HTTP 客户端（对齐设计 §二 2.2）。
//
// 零值语义（明写，防「未给」与「给 0」同形被误读）：
//   - `HTTP == nil`  ⇒ 用默认 `&http.Client{Timeout: searxDefaultTimeout}`；
//   - `Retries == 0` ⇒ **不重试**（重试次数 = 本字段；生产默认 1 由 `NewSearxClient()` 给）。
type SearxClient struct {
	BaseURL string       // 服务地址（空 ⇒ 读 env ZERG_SEARXNG_URL ⇒ 默认 127.0.0.1:8888）
	HTTP    *http.Client // 零值 ⇒ 默认 &http.Client{Timeout: 10s}
	Retries int          // 失败后重试次数（<0 ⇒ 0；生产默认 1 见 NewSearxClient）
}

// NewSearxClient —— 生产默认客户端：地址取 env（空则默认 URL），超时 10s，重试 1 次。
func NewSearxClient() *SearxClient {
	return &SearxClient{
		BaseURL: searxBaseURL(),
		HTTP:    &http.Client{Timeout: searxDefaultTimeout},
		Retries: searxDefaultRetries,
	}
}

// searxBaseURL —— 解析服务地址：env `ZERG_SEARXNG_URL`（去空白）优先，否则默认 `searxDefaultBaseURL`。
func searxBaseURL() string {
	if v := strings.TrimSpace(os.Getenv(searxEnvBaseURL)); v != "" {
		return v
	}
	return searxDefaultBaseURL
}

// normalizeSearxQuery —— 归一入参（`Lang` 空 ⇒ `all`，其余透传）。
func normalizeSearxQuery(q SearxQuery) SearxQuery {
	if strings.TrimSpace(q.Lang) == "" {
		q.Lang = "all"
	}
	return q
}

// searchRequestURL —— 拼查询 URL（对齐设计 §二 2.2 的查询面逐字）：
//
//	GET {BaseURL}/search?q=<urlencoded>&format=json&language=<lang>&time_range=<tr>&safesearch=0&pageno=1
//
// `domains` 走同件 `buildSiteOp` 拼 `site:…` 追加到 `q`；`TimeRange` 空则不出现 `time_range` 参数。
func (c *SearxClient) searchRequestURL(q SearxQuery) (string, error) {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		base = searxBaseURL()
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/search")
	if err != nil {
		return "", fmt.Errorf("searxng 服务地址无效(%q): %w", base, err)
	}
	text := q.Query
	if len(q.Domains) > 0 {
		if op := buildSiteOp(q.Domains); op != "" {
			text += " " + op
		}
	}
	vals := u.Query()
	vals.Set("q", text)
	vals.Set("format", "json")
	vals.Set("language", q.Lang)
	if tr := strings.TrimSpace(q.TimeRange); tr != "" {
		vals.Set("time_range", tr)
	}
	vals.Set("safesearch", "0")
	vals.Set("pageno", "1")
	u.RawQuery = vals.Encode()
	return u.String(), nil
}

// Search —— 查询 searxng 服务（失败按 `Retries` 重试）。
//
// 错误面（设计 §二 2.2）：连接失败 / 非 200 / JSON 解析失败 ⇒ 返回 `error`；
// **200 且空结果 ⇒ 返回空 + `nil` error**（真无结果，与「服务不可达」**可分** —— 本件治 fail-open 的落点）。
func (c *SearxClient) Search(ctx context.Context, q SearxQuery) ([]SearxResult, error) {
	q = normalizeSearxQuery(q)
	reqURL, err := c.searchRequestURL(q)
	if err != nil {
		return nil, err
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: searxDefaultTimeout}
	}
	retries := c.Retries
	if retries < 0 {
		retries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("searxng 查询取消: %w", ctxErr)
		}
		res, retryable, err := c.searchOnce(ctx, client, reqURL, q.Limit)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retryable {
			break // 语义错（坏地址 / 坏 JSON / 4xx）重试无意义
		}
	}
	return nil, fmt.Errorf("searxng 查询失败(%s): %w", reqURL, lastErr)
}

// searchOnce —— 单次查询。`retryable == false` 表示这是重试无意义的语义错。
// 5xx / 连接类 / 读身体失败 ⇒ retryable=true；4xx / 坏 JSON / 服务内 error 字段 ⇒ false。
func (c *SearxClient) searchOnce(ctx context.Context, client *http.Client, reqURL string, limit int) ([]SearxResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "ZergAgent/2.5")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("连接 searxng 服务失败: %w", err) // 连接类 ⇒ 可重试
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 5xx 可重试（服务瞬时），4xx 不重试（请求本身错）。
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, resp.StatusCode >= 500,
			fmt.Errorf("searxng 服务返回 HTTP %d: %s (body: %s)", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, fmt.Errorf("读 searxng 响应失败: %w", err)
	}
	var sr struct {
		Results []SearxResult `json:"results"`
		Error   string        `json:"error"`
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, false, fmt.Errorf("searxng 响应解析失败: %w", err)
	}
	if sr.Error != "" {
		return nil, false, fmt.Errorf("searxng 服务错误: %s", sr.Error)
	}
	if limit > 0 && len(sr.Results) > limit {
		sr.Results = sr.Results[:limit]
	}
	return sr.Results, false, nil // 空结果 ⇒ (空, nil)：真无结果，非 error
}
