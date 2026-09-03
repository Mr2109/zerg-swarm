package agent

// net_tools.go — 虫族 v2.5 Agent 网络工具集（web_search / web_fetch / todo_write）
// 设计：网络搜索 + 网页抓取 + 任务跟踪，补充 tools.go 的本地文件工具
// 实现：searxng 搜索 + URL 抓取 + todo 计划（2026-08-13——我补全 Codex 空壳）

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SearchResult — searxng 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// SearxngResponse — searxng JSON 响应
type SearxngResponse struct {
	Results []SearchResult `json:"results"`
}

// WebSearch — 调 searxng 搜索（v2.5：直接调 Python 桥——不走 HTTP 服务）
// 复用 Hermes 同款 searx 引擎（searxng-src 库——内存调引擎）
func WebSearch(query string, limit int) (string, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	// 定位桥脚本（相对本文件——core/internal/agent/searx_search.py）
	_, file, _, _ := runtime.Caller(0)
	scriptDir := filepath.Dir(file)
	script := filepath.Join(scriptDir, "searx_search.py")
	venvPy := "<repo>/vendor/searxng/.venv/bin/python3"

	cmd := exec.Command(venvPy, script, query, fmt.Sprintf("%d", limit))
	cmd.Env = append(os.Environ(), "SEARXNG_SETTINGS_PATH=<repo>/vendor/searxng/searx/settings.yml")
	// 清除 PYTHONPATH（防 Hermes venv 污染——脚本内也处理）
	cmd.Env = removeEnv(cmd.Env, "PYTHONPATH")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("searx 桥执行失败: %v (stderr: %s)", err, truncate(stderr.String(), 200))
	}

	var sr struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &sr); err != nil {
		return "", fmt.Errorf("searx 桥解析失败: %w (输出: %s)", err, truncate(stdout.String(), 200))
	}
	if sr.Error != "" {
		return "", fmt.Errorf("searx 桥错误: %s", sr.Error)
	}

	if len(sr.Results) == 0 {
		return "（无搜索结果）", nil
	}

	var sb strings.Builder
	for i, r := range sr.Results {
		if i >= limit {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, truncate(r.Content, 300))
	}
	return sb.String(), nil
}

// removeEnv — 从环境变量列表删除指定 key
func removeEnv(env []string, key string) []string {
	out := env[:0]
	prefix := key + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

// WebFetch — 抓取 URL（限 5000 字符——去 HTML 标签）
func WebFetch(rawURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("URL 无效: %w", err)
	}
	req.Header.Set("User-Agent", "ZergAgent/2.5")
	req.Header.Set("Accept", "text/markdown,text/plain,text/html")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("抓取失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取失败: %w", err)
	}

	// 去 HTML 标签 + 压缩空白
	text := string(body)
	re := regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>|<style[^>]*>[\s\S]*?</style>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")

	// P4-50 截断加大（5000→10000——文档/长网页内容不足——模型反复抓取纠结）
	return truncate(text, 10000), nil
}

// TodoItem — 计划项
type TodoItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"` // pending/in_progress/completed
}

// TodoWrite — 更新任务计划（todo 列表——结构体传递）
func TodoWrite(todos []TodoItem) (string, error) {
	if len(todos) == 0 {
		return "（todo 列表为空）", nil
	}
	var sb strings.Builder
	for _, t := range todos {
		mark := "⬜"
		if t.Status == "completed" {
			mark = "✅"
		} else if t.Status == "in_progress" {
			mark = "🔄"
		}
		fmt.Fprintf(&sb, "%s %s %s\n", mark, t.ID, t.Content)
	}
	return sb.String(), nil
}

// ============ v1.0.1 升级（2026-09-02——ABC 全做——Tavily/Anthropic 调研） ============

// SearchParams — web_search v1.0.1 参数（模型可控）
type SearchParams struct {
	Query     string   // 搜索词
	Limit     int      // 结果数（默认 5——auto 深度: 长 query 自动 8）
	Lang      string   // 语言 all/zh/en...（ISO 639-1——searxng babel locale）
	TimeRange string   // 时间过滤 day/week/month/year（空=不限）
	Domains   []string // 限定域名（site: 操作符——信任站点）
	FetchTop  int      // 自动抓取前 N 条正文（0=不抓——默认 1 更实用）
	Rewrite   bool     // LLM query 重写（X3 9001——失败 fallback 规则）
}

// WebSearchV2 — v1.0.1 完整搜索（query 重写 → 缓存 → 桥 → 格式化 → 自动抓取）
func WebSearchV2(p SearchParams) (string, error) {
	if p.Limit <= 0 || p.Limit > 10 {
		p.Limit = 5
	}
	// auto 深度: 长 query → 更多结果（Tavily 思路——query 越长信息面越大）
	if len([]rune(p.Query)) > 40 && p.Limit < 8 {
		p.Limit = 8
	}
	// 1. query 规则重写（Elasticsearch 实证: 词法检索 query 重写 +8 NDCG@10）
	q := rewriteQuery(p.Query)
	// 2. domains → site: 操作符
	if len(p.Domains) > 0 {
		q += " " + buildSiteOp(p.Domains)
	}
	// 3. LLM 重写（可选——失败 fallback 规则结果）
	if p.Rewrite {
		if rq, err := llmRewriteQuery(q); err == nil && rq != "" && len([]rune(rq)) >= 2 {
			q = rq
		}
	}
	// 4. 缓存（TTL 5min——同 query 秒回——模型反复搜索不烧引擎）
	cacheKey := q + "|" + p.Lang + "|" + p.TimeRange
	if hit := searchCacheGet(cacheKey); hit != "" {
		return hit, nil
	}
	// 5. 调桥（失败重试一次——引擎抖动容错）
	result, err := webSearchBridge(q, p.Limit, p.Lang, p.TimeRange)
	if err != nil {
		result, err = webSearchBridge(q, p.Limit, p.Lang, p.TimeRange)
	}
	if err != nil {
		return "", err
	}
	// 6. 自动抓取 top N（可选——模型省一轮 web_fetch——Anthropic include_raw_content 思路）
	if p.FetchTop > 0 && result.Results != nil {
		fetchN := p.FetchTop
		if fetchN > 3 {
			fetchN = 3
		}
		var extra strings.Builder
		for i, r := range result.Results {
			if i >= fetchN {
				break
			}
			body, ferr := WebFetch(r.URL)
			if ferr == nil && body != "" {
				fmt.Fprintf(&extra, "\n【自动抓取 top%d: %s】\n%s\n", i+1, r.URL, truncate(body, 3000))
			}
		}
		result.Formatted += extra.String()
	}
	// 7. 存缓存
	searchCachePut(cacheKey, result.Formatted)
	return result.Formatted, nil
}

// searchBridgeResult — 桥返回（结构化）
type searchBridgeResult struct {
	Results   []SearchResult
	Formatted string
}

// webSearchBridge — 调 searx 桥（v1.0.1: lang/time_range 参数 + score）
func webSearchBridge(query string, limit int, lang string, timeRange string) (*searchBridgeResult, error) {
	if lang == "" {
		lang = "all"
	}
	_, file, _, _ := runtime.Caller(0)
	scriptDir := filepath.Dir(file)
	script := filepath.Join(scriptDir, "searx_search.py")
	venvPy := "<repo>/vendor/searxng/.venv/bin/python3"
	args := []string{script, query, fmt.Sprintf("%d", limit), lang}
	if timeRange != "" {
		args = append(args, timeRange)
	}
	cmd := exec.Command(venvPy, args...)
	cmd.Env = append(os.Environ(), "SEARXNG_SETTINGS_PATH=<repo>/vendor/searxng/searx/settings.yml")
	cmd.Env = removeEnv(cmd.Env, "PYTHONPATH")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("searx 桥执行失败: %v (stderr: %s)", err, truncate(stderr.String(), 200))
	}
	var sr struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &sr); err != nil {
		return nil, fmt.Errorf("searx 桥解析失败: %w (输出: %s)", err, truncate(stdout.String(), 200))
	}
	if sr.Error != "" {
		return nil, fmt.Errorf("searx 桥错误: %s", sr.Error)
	}
	if len(sr.Results) == 0 {
		return &searchBridgeResult{Formatted: "（无搜索结果）"}, nil
	}
	var sb strings.Builder
	var res []SearchResult
	for i, r := range sr.Results {
		// 质量过滤: score 太低的结果丢弃（Tavily 建议 score<0.5 过滤——searxng score 尺度不同——0.1 以下弃）
		if r.Score < 0.1 && r.Score != 0 {
			continue
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, truncate(r.Content, 300))
		res = append(res, SearchResult{Title: r.Title, URL: r.URL, Content: r.Content})
	}
	if len(res) == 0 {
		return &searchBridgeResult{Formatted: "（无搜索结果）"}, nil
	}
	return &searchBridgeResult{Results: res, Formatted: sb.String()}, nil
}

// rewriteQuery — 规则层 query 重写（去噪音词 + 中英数字边界空格 + 长 query 压缩）
func rewriteQuery(q string) string {
	q = strings.TrimSpace(q)
	for _, nw := range []string{"请问", "帮我查一下", "帮我查", "帮我", "查一下", "我想要", "麻烦你", "麻烦", "一下", "呢？", "吗？", "呢", "吗"} {
		q = strings.ReplaceAll(q, nw, " ")
	}
	// 中文与英文/数字之间加空格（searxng 词法匹配——中英混排分词——提升命中）
	runes := []rune(q)
	var sb strings.Builder
	for i, r := range runes {
		sb.WriteRune(r)
		if i+1 < len(runes) {
			cur, next := runes[i], runes[i+1]
			curCJK := cur >= 0x4e00 && cur <= 0x9fff
			nextCJK := next >= 0x4e00 && next <= 0x9fff
			curAlnum := (cur >= 'a' && cur <= 'z') || (cur >= 'A' && cur <= 'Z') || (cur >= '0' && cur <= '9')
			nextAlnum := (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9')
			if curCJK && nextAlnum || curAlnum && nextCJK {
				sb.WriteByte(' ')
			}
		}
	}
	q = sb.String()
	// 长 query 压缩（>120 字符截断——searxng 长 query 命中差）
	if rl := len([]rune(q)); rl > 120 {
		q = string([]rune(q)[:120])
	}
	return strings.Join(strings.Fields(q), " ")
}

// buildSiteOp — domains → site: 操作符（Tavily include_domains 思路——searxng 支持 site:）
func buildSiteOp(domains []string) string {
	parts := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d != "" {
			parts = append(parts, "site:"+d)
		}
	}
	return strings.Join(parts, " OR ")
}

// llmRewriteQuery — LLM query 重写（X3 9001 ornith——失败 fallback 规则——可选）
// 通道: ssh g01@<worker-ip> → curl 127.0.0.1:9001/v1/chat/completions（llama-server 绑定 127.0.0.1——需 X3 本机调）
func llmRewriteQuery(q string) (string, error) {
	// 2026-09-03 实测修: ornith 思考模型——原提示词弱(max_tokens 60 全被 reasoning 吃——content 漏出分析文本"user wants me to..."——当改写词用了)
	// 强约束提示词 + max_tokens 200(reasoning 留空间) + 结果校验(>60字=分析漏出——error→fallback 规则层)
	prompt := "改写搜索词。规则: 1.直接输出改写后的搜索词(一行,40字内) 2.禁止任何分析/解释/翻译/思考过程 3.保留核心术语 4.中文词间加空格分词。原词: " + q + "\n\n改写结果:"
	payload := fmt.Sprintf(`{"model":"example-35b-v2","messages":[{"role":"user","content":%q}],"max_tokens":200,"stream":false}`, prompt)
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", "-o", "StrictHostKeyChecking=no", "g01@<worker-ip>",
		"curl -s -m 10 http://127.0.0.1:9001/v1/chat/completions -H 'Content-Type: application/json' -d '"+payload+"'")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("LLM 重写不可用: %w", err)
	}
	var d struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		return "", err
	}
	if len(d.Choices) == 0 {
		return "", fmt.Errorf("LLM 重写无结果")
	}
	out := strings.TrimSpace(d.Choices[0].Message.Content)
	if out == "" || len([]rune(out)) > 60 { // 分析漏出(长文本)=不合格——fallback 规则层
		return "", fmt.Errorf("LLM 重写不合格(分析漏出/空): %q", truncate(out, 40))
	}
	return out, nil
}

// ============ 缓存（TTL 5min——/tmp/zerg-search-cache.json 持久化） ============
var searchCacheMu sync.Mutex
var searchCacheMap = map[string]searchCacheEntry{}

type searchCacheEntry struct {
	Result string `json:"result"`
	Expire int64  `json:"expire"` // unix 秒
}

const searchCacheFile = "/tmp/zerg-search-cache.json"
const searchCacheTTL = 5 * 60 // 5 分钟

func searchCacheGet(key string) string {
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	e, ok := searchCacheMap[key]
	if !ok {
		return ""
	}
	if time.Now().Unix() > e.Expire {
		delete(searchCacheMap, key)
		return ""
	}
	return e.Result
}

func searchCachePut(key, result string) {
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	if len(searchCacheMap) > 200 {
		searchCacheMap = map[string]searchCacheEntry{}
	}
	searchCacheMap[key] = searchCacheEntry{Result: result, Expire: time.Now().Unix() + searchCacheTTL}
	// 异步持久化（失败静默——缓存非关键）
	go func() {
		searchCacheMu.Lock()
		b, _ := json.Marshal(searchCacheMap)
		searchCacheMu.Unlock()
		_ = os.WriteFile(searchCacheFile, b, 0o644)
	}()
}

// LoadSearchCache — 启动时加载缓存（main 调用）
func LoadSearchCache() {
	b, err := os.ReadFile(searchCacheFile)
	if err != nil {
		return
	}
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	_ = json.Unmarshal(b, &searchCacheMap)
	// 清过期
	now := time.Now().Unix()
	for k, e := range searchCacheMap {
		if now > e.Expire {
			delete(searchCacheMap, k)
		}
	}
}
