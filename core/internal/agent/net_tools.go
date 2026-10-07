package agent

// net_tools.go — 虫族 v2.5 Agent 网络工具集（web_search / web_fetch / todo_write）
// 设计：网络搜索 + 网页抓取 + 任务跟踪，补充 tools.go 的本地文件工具
// 实现：searxng 搜索 + URL 抓取 + todo 计划（2026-08-13——我补全 Codex 空壳）

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ─── A-06（2026-10-07）：searx 查询的超时闸 ＋ 空结果 fail-closed ───
//
// searxBridgeTimeout — searx 查询执行上限（撤桥后服务路 `searxQueryService` 用；旧 Python 桥已退役）。
// 取值：与**同件** WebFetch 的网络抓取口径一致（`&http.Client{Timeout: 10 * time.Second}`）——
// 同为网络检索，故取同件先例 10s（✗ 不自创大数字）。
// 形态：包级变量（非编译期常量，故不入 knobs.go 常量表；同 `searchCacheFile` 一族）——
// 测试可注入小值做超时负控/隔离，生产默认取先例值。
var searxBridgeTimeout = 10 * time.Second

// ─── 撤桥（2026-10-07）：恒走 Go HTTP 客户端直连 searxng 服务 ───
//
// 接线：`webSearchBridge` / `WebSearch` 恒走 `searx_client.go` 的 `SearxClient`（HTTP 直连
// `ZERG_SEARXNG_URL`，默认 http://127.0.0.1:8888），不再起 Python 子进程。★ 老桥 `searx_search.py`
// 与两处 `exec.Command`、回退旋钮 `ZERG_SEARX_LEGACY_BRIDGE` 本批已退役（撤桥批 A/B 组）。
// 两态可分：服务在跑 ⇒ 正常返回；服务不在（连不上 / 非 200 / 坏 JSON）⇒ 带具名原因 error（✗ 不静默空）。

// searxBridgeClientOverride — 测试注入的服务端客户端（非空 ⇒ 直接用；生产默认 nil ⇒ NewSearxClient()）。
// 形态：包内变量注入（✗ 不 export ZERG_*），测试隔离、不改生产行为（对齐本包测试注入惯例）。
var searxBridgeClientOverride *SearxClient

// searxClientForBridge — 取本路径用的客户端：测试注入优先，否则生产默认（env 取址 · 10s 超时 · 重试 1）。
func searxClientForBridge() *SearxClient {
	if c := searxBridgeClientOverride; c != nil {
		return c
	}
	return NewSearxClient()
}

// searxQueryService — 走 Go HTTP 客户端直连 searxng 服务（A2 步3 默认路）。
//
// 超时（保留 A-06 语义）：`searxBridgeTimeout`（10s）到点即 fail-closed —— 服务挂起不答也**不永久阻塞**。
// 错误面（保留 A-06 fail-closed 语义 ＋ 两态可分）：连不上 / 非 200 / 坏 JSON ⇒「服务不可达」error；
// 服务可达但空结果 ⇒「空结果」error（两者文案可分，✗ 不静默回「（无搜索结果）」）。
func searxQueryService(query string, limit int, lang string, timeRange string) ([]SearxResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), searxBridgeTimeout)
	defer cancel()
	res, err := searxClientForBridge().Search(ctx, SearxQuery{
		Query: query, Limit: limit, Lang: lang, TimeRange: timeRange,
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("searxng 服务超时（%s 内未应答；fail-closed）: %w", searxBridgeTimeout, err)
		}
		return nil, fmt.Errorf("searxng 服务不可达/查询失败（服务不在 · ✗ 不静默空）: %w", err)
	}
	if len(res) == 0 {
		// fail-closed（保留 A-06 语义）：服务可达但空结果 ⇒ 带具名原因上报，✗ 不静默回「（无搜索结果）」。
		return nil, fmt.Errorf("searxng 空结果（服务可达但无结果/引擎全失败；fail-closed 拒绝静默「无搜索结果」）")
	}
	return res, nil
}

// searxSearchViaService — WebSearch（v1 文本形态）走服务直连：查询 ⇒ 文本格式化。
func searxSearchViaService(query string, limit int, lang string, timeRange string) (string, error) {
	res, err := searxQueryService(query, limit, lang, timeRange)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, r := range res {
		if i >= limit {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, truncate(r.Content, 300))
	}
	return sb.String(), nil
}

// WebSearch — 调 searxng 搜索（v2.5：直接调 Python 桥——不走 HTTP 服务）
// ★ 已作废（2026-10-07 Mr2109 拍：新口径＝走服务端口）—— 原行保留不删；订正来源：拍板落定-所有稿-20261007.md 决-05
// 复用 Hermes 同款 searx 引擎（searxng-src 库——内存调引擎）
func WebSearch(query string, limit int) (string, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	// 撤桥（2026-10-07）：恒走 Go 客户端直连服务（旧 Python 桥与回退旋钮已退役）。
	return searxSearchViaService(query, limit, "all", "")
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
	Rewrite   bool     // LLM query 重写（EDGE-NODE 9001——失败 fallback 规则）
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

// webSearchBridge — 搜索入口（v1.0.1: lang/time_range 参数 + score）。
// 撤桥（2026-10-07）：恒走 **Go HTTP 客户端直连 searxng 服务**（HTTP 8888）；旧 Python 桥已退役。
func webSearchBridge(query string, limit int, lang string, timeRange string) (*searchBridgeResult, error) {
	if lang == "" {
		lang = "all"
	}
	res, err := searxQueryService(query, limit, lang, timeRange)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	var out []SearchResult
	for i, r := range res {
		// 质量过滤: score 太低的结果丢弃（Tavily 建议 score<0.5 过滤——searxng score 尺度不同——0.1 以下弃）
		if r.Score < 0.1 && r.Score != 0 {
			continue
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, truncate(r.Content, 300))
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Content: r.Content})
	}
	if len(out) == 0 {
		// fail-closed（保留 A-06 语义）：服务可达但结果全被 score 质量过滤（<0.1）⇒ 空集，✗ 不静默。
		return nil, fmt.Errorf("searxng 结果全被质量过滤（score 全部 < 0.1，空集；fail-closed）")
	}
	return &searchBridgeResult{Results: out, Formatted: sb.String()}, nil
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

// llmRewriteQuery — LLM query 重写（EDGE-NODE 9001 example-35b-v2——失败 fallback 规则——可选）
// 通道: ssh g01@<EDGE-NODE IP> → curl 127.0.0.1:9001/v1/chat/completions（llama-server 绑定 127.0.0.1——需 EDGE-NODE 本机调）
// 取址: EDGE-NODE 的 IP 由 fleet.yaml 的 fleet 段解析（config.NodeAddr——GAP-20260926-71），不再写死。
// 端口: 9001 是 EDGE-NODE 本机 llama-server 端口（不是 fleet 段字段，故留字面值）。
const x3RewritePort = 9001

// sshTargetFor 把机器名编成 ssh 目标（g01@<IP>）——IP 来自 fleet 真源（config.NodeAddr）。
// 名册缺该机器时 NodeAddr 回退机器名本身（与 gateway.fleetNode 同口径）。
func sshTargetFor(host string, fleet map[string]config.FleetNode) string {
	ip, _ := config.NodeAddr(fleet, host)
	return "g01@" + ip
}

// x3SSHTarget 取 EDGE-NODE 的 ssh 目标（g01@<IP>）——IP 来自 fleet.yaml 真源（config.NodeAddr）。
func x3SSHTarget() (string, error) {
	fleetYAML := filepath.Join(statepath.WorkspaceRoot(), "gateway", "fleet.yaml")
	cfg, err := config.LoadFleetConfig(fleetYAML)
	if err != nil {
		return "", fmt.Errorf("读 fleet.yaml 失败(%s): %w", fleetYAML, err)
	}
	return sshTargetFor("edge-node", cfg.Fleet), nil
}

func llmRewriteQuery(q string) (string, error) {
	// 2026-09-03 实测修: example-35b-v2 思考模型——原提示词弱(max_tokens 60 全被 reasoning 吃——content 漏出分析文本"user wants me to..."——当改写词用了)
	// 强约束提示词 + max_tokens 200(reasoning 留空间) + 结果校验(>60字=分析漏出——error→fallback 规则层)
	prompt := "改写搜索词。规则: 1.直接输出改写后的搜索词(一行,40字内) 2.禁止任何分析/解释/翻译/思考过程 3.保留核心术语 4.中文词间加空格分词。原词: " + q + "\n\n改写结果:"
	payload := fmt.Sprintf(`{"model":"example-35b-v2","messages":[{"role":"user","content":%q}],"max_tokens":200,"stream":false}`, prompt)
	sshTarget, err := x3SSHTarget()
	if err != nil {
		return "", fmt.Errorf("LLM 重写不可用: %w", err)
	}
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", "-o", "StrictHostKeyChecking=no", sshTarget,
		fmt.Sprintf("curl -s -m 10 http://127.0.0.1:%d/v1/chat/completions -H 'Content-Type: application/json' -d '%s'", x3RewritePort, payload))
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

// 2026-09-18 修（/tmp 硬编码——口径同 api/tasks_persist.go）: 原写死 const searchCacheFile = "/tmp/zerg-search-cache.json"。
// 缓存落 /tmp = macOS 重启即清 + tmp_cleaner 3 天未访问即删；改为 statepath 统一状态目录派生
// （ZERG_STATE_DIR → ~/.zerg/state/zerg-search-cache.json）。
// 迁移兼容（首次）: 新路径不存在而旧 /tmp 存在 ⇒ **读旧一次**；**写只写新路径**；旧文件**不删、不改**。
// 新路径已存在 ⇒ 旧路径完全不看。显式覆盖（测试隔离）⇒ 不走旧回退。
const searchCacheFileName = "zerg-search-cache.json"

// searchCacheLegacyDefaultPath 旧硬编码落点字面量。只作**首次迁移读取**来源：读一次；不删、不改、永不写入。
const searchCacheLegacyDefaultPath = "/tmp/zerg-search-cache.json"

// legacySearchCacheFile 旧路径（包级变量 = 上面的字面量；迁移用例/测试进程隔离可切换）。
var legacySearchCacheFile = searchCacheLegacyDefaultPath

// searchCacheFile 缓存文件（包级变量——测试可切换隔离路径）。
// 语义: 非空 ⇒ 直接用该路径（测试隔离/显式覆盖）；空 ⇒ 走 statepath 统一状态目录派生（生产默认）。
var searchCacheFile = ""

const searchCacheTTL = 5 * 60 // 5 分钟

// searchCacheWritePath 写路径：永远是统一状态目录。永不写旧 /tmp 路径。
func searchCacheWritePath() string {
	if p := strings.TrimSpace(searchCacheFile); p != "" {
		return p
	}
	return statepath.File(searchCacheFileName)
}

// searchCacheReadPath 读路径：统一状态目录优先；新路径不存在且旧 /tmp 存在 ⇒ 读旧一次（迁移兼容）。
// 新路径存在 ⇒ 旧路径完全不看。显式覆盖 searchCacheFile（测试隔离）时不退旧路径。
func searchCacheReadPath() string {
	p := searchCacheWritePath()
	if _, err := os.Stat(p); err == nil {
		return p // 新路径已存在——旧路径完全不看
	}
	if strings.TrimSpace(searchCacheFile) != "" {
		return p
	}
	legacy := strings.TrimSpace(legacySearchCacheFile)
	if legacy == "" {
		return p
	}
	if _, err := os.Stat(legacy); err != nil {
		return p // 无旧文件——首次运行（空缓存）
	}
	log.Printf("📜 搜索缓存首次迁移: 读旧 %s（只读一次——旧文件保留不删；写入只落 %s）\n", legacy, p)
	return legacy
}

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
	// 异步持久化（失败静默——缓存非关键）；写只写新路径（统一状态目录——旧 /tmp 永不写）
	go func() {
		searchCacheMu.Lock()
		b, _ := json.Marshal(searchCacheMap)
		searchCacheMu.Unlock()
		dst := searchCacheWritePath()
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return
		}
		_ = os.WriteFile(dst, b, 0o644)
	}()
}

// LoadSearchCache — 启动时加载缓存（main 调用）
// 读路径：新（统一状态目录）优先；新缺失 + 旧 /tmp 在 ⇒ 读旧一次（2026-09-18 迁移兼容）
func LoadSearchCache() {
	b, err := os.ReadFile(searchCacheReadPath())
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
