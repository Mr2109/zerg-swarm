// package gateway 负责 Zerg v2 网关：三种 AI 标准（OpenAI Chat / Claude Messages / OpenAI Responses）的统一入口。
// 阶段 2：纯透传——认证 + 路由 + 转发，不做任何格式转换。
//
// 架构：
//
//	POST :8682/v1/*
//	  → AuthMiddleware（三兼容认证）
//	  → 提取 body.model
//	  → pickRoute（已加载→空闲→负载低）
//	  → 转发到子端 :8100/infer（body + "_path" 字段）
//	  → 响应透传（非流式完整 / 流式逐 chunk）
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compressor"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/control"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway/adapter"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway/orchestrator"
	"github.com/Mr2109/zerg-swarm/core/internal/localback"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

// Gateway 网关主结构，持有配置和 HTTP 客户端。
// 阶段 3：内置 LocalBackend 管理本机子端。
// 阶段 B：内置动作级路由表（actionRouter）。
type Gateway struct {
	authToken string // 从 fleet.yaml auth.token 读取
	config    *config.FleetConfig
	client    *http.Client            // 转发用 HTTP 客户端
	localBack *localback.LocalBackend // 本机子端（阶段 3）
	store     *store.Store            // fleet 快照（路由决策用：已加载模型/活跃请求/健康）
	comp      *compressor.Compressor  // LLMLingua-2 压缩器（V22，Go 一体化）
	compMu    sync.RWMutex
	gate      *control.Gate // M3 集中控制层（v2.4——工具调用拦截；nil=不启用）

	// 会话粘性（T4，借鉴 vLLM consistent_hash / llama.cpp conv_model_tracker）
	// sessionID → 绑定的机器。同一会话固定同一台机器，复用 KV cache。
	sessionMu sync.RWMutex
	sessions  map[string]sessionBinding

	// cache-aware 路由（T5，借鉴 vLLM cache_aware）
	// 每机器记录最近请求的 prompt 前缀签名（字符级，避免分词开销），
	// 路由时前缀匹配度高的机器加分（复用 KV cache / prefix cache）。
	prefixMu sync.RWMutex
	prefixes map[string]map[string]int // host → prompt签名 → 出现次数

	// 熔断/降权（T7，借鉴 vLLM/open-llm-router）
	// 每机器连续转发失败计数，超阈值后短期降权（不再选为最优），
	// 心跳恢复健康后自动回池。
	failMu     sync.Mutex
	failCounts map[string]int // host → 连续失败次数
	// modelFailCounts host → 模型级失败次数（引擎自己报的 5xx、子端 507 拒装等）。
	// **不计入熔断**（设计-子端服务切换 §11 M4）：上游不可达才降级整机；
	// 模型级错误只记原因 + 有限重试。与 failCounts 同一把锁（failMu）保护。
	modelFailCounts map[string]int
	failSince       map[string]time.Time // host → 首次熔断时间（自动恢复用）

	// v2.5.5 重启窗口期治本: 主控启动时间——宽限期内（startupGrace 秒）忽略 unhealthy 快照
	// 根因: 主控重启瞬间 X3 心跳失败（主控 API 未就绪窗口期）→ 快照 unhealthy → 路由拒绝
	// 治本: 启动后宽限期——机器还没机会心跳——不应因启动窗口失败被拒
	startupTime time.Time

	// 阶段 B：动作级路由表
	actionRouter *ActionModelRouter // 动作→模型映射（代码常量，后续迁移到 fleet.yaml）

	// B13: 负载均衡轮询（压力均分——本机/X3 交替接活）
	roundRobinMu sync.Mutex
	roundRobin   map[string]int // model → 轮询计数器（0=本机先, 1=X3先, 交替）
	tripMu       sync.Mutex
	tripCounts   map[string]int // v2.5.4.9 C failover——机器失败计数（熔断用）

	// 熔断原因可见（只读快照 + 手动复位入口）：
	// failCounts 只记次数——熔断后用户看不到「为什么熔断」。这里额外记最后一次失败原因文本与时间，
	// 由 BreakerSnapshot() 只读暴露给 /api/gateway/breakers。
	// 与 failCounts/failSince 同一把锁（failMu）保护——写入点仅 markFailure/tripMachine/
	// pickRouteExcluding（后两者是历史上漏写原因的计数路径）/markSuccess/ClearFailures/ResetBreakers。
	lastErr   map[string]string    // host → 最后一次失败原因文本（人类可读——转发错误原文）
	lastErrAt map[string]time.Time // host → 最后一次失败时间（快照按 RFC3339 输出）
	// host → 记录该原因的计数路径名（markFailure / tripMachine / pickRouteExcluding）——
	// 「兜底原因」不在这里区分（那是 last_error 文本的 unspecified 前缀的事），本字段恒为该次计数的来源。
	// 为什么单独存：活系统上出现过「fail_count=11 但 last_error 为空」——只存自由文本无法判定
	// 是哪条计数路径涨的计数，本字段让快照能直接指向路径（每一处 ++/赋值都必须写它）。
	lastErrSrc map[string]string

	// 阶段 C：脑手编排器（复合模型 zerg-baiyan（白眼——多视角参考+聚合提炼））
	orchestrator *orchestrator.Orchestrator

	// B4 v2：排除本机模式（用户工作时——路由跳过 local，任务全走远程）
	excludeLocalMu sync.RWMutex
	excludeLocal   bool

	// v2.5.4.10 模型适配器注册表（方案 B——适配器完整路由）
	// 模型名 → 适配器插件（Execute 返回路由字段——machine/format/params）
	// 无适配器的模型 → 回退旧路由（fleet.yaml + 打分）——兼容
	adapterRegistry map[string]plugin.Plugin

	// v2.5.4.10 模型级超时覆盖（适配器声明 timeout_sec——转发时用）
	timeoutOverride map[string]int

	// 丙批 N4（2026-09-10）：前缀命中率闭环——按 (model, prompt_version) 滑动窗口
	// 统计 cache_read/cache_miss，并做版本级下降告警；GET /api/metrics/prefix_cache 查询。
	prefixCache *prefixCacheTracker

	// 能力硬门槛（待修补 #11）：按**目标引擎**判定请求必需能力（如 vision）。
	// capSource 可注入（测试用假实现，绝不写 ~/.zerg）；modelsRootPath 可注入
	// （测试用 t.TempDir），为空时用 modelreg.DefaultModelsDir()。
	capSource      CapabilitySnapshotSource
	modelsRootPath string
}

// setRequestTimeout — 模型级超时覆盖（适配器声明——Qwen3.8 120s/Nemotron 60s）
func (g *Gateway) setRequestTimeout(model string, sec int) {
	if g.timeoutOverride == nil {
		g.timeoutOverride = map[string]int{}
	}
	g.timeoutOverride[model] = sec
	log.Printf("⏱️ adapter %s: timeout override %ds", model, sec)
}

// getRequestTimeout — 查询模型超时覆盖（无则 0——用默认）
func (g *Gateway) getRequestTimeout(model string) int {
	if g.timeoutOverride == nil {
		return 0
	}
	return g.timeoutOverride[model]
}

// SetExcludeLocal 切换排除本机模式（true=路由跳过 local 候选）。
// 持久化到 state 文件——主控重启后保持开关状态（UI 同步不丢）。
func (g *Gateway) SetExcludeLocal(exclude bool) {
	g.excludeLocalMu.Lock()
	g.excludeLocal = exclude
	g.excludeLocalMu.Unlock()
	log.Printf("🚫 local-exclude mode: %v", exclude)
	_ = g.persistExcludeLocal(exclude)
}

// LoadExcludeLocal 启动时从 state 文件读回开关状态（主控重启后保持）。
func (g *Gateway) LoadExcludeLocal() {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return // 无状态文件——默认关
	}
	var st struct {
		ExcludeLocal bool `json:"exclude_local"`
	}
	if json.Unmarshal(data, &st) == nil && st.ExcludeLocal {
		g.excludeLocalMu.Lock()
		g.excludeLocal = true
		g.excludeLocalMu.Unlock()
		log.Printf("🚫 local-exclude mode (restored after controller restart): true")
	}
}

func (g *Gateway) persistExcludeLocal(exclude bool) error {
	st := struct {
		ExcludeLocal bool   `json:"exclude_local"`
		UpdatedAt    string `json:"updated_at"`
	}{ExcludeLocal: exclude, UpdatedAt: time.Now().Format(time.RFC3339)}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath(), data, 0644)
}

// stateFilePath 主控状态文件路径（/tmp——重启后保留，机器重启才清）。
func stateFilePath() string {
	return "/tmp/zerg-state.json"
}

// ExcludeLocal 查询排除本机模式状态。
func (g *Gateway) ExcludeLocal() bool {
	g.excludeLocalMu.RLock()
	defer g.excludeLocalMu.RUnlock()
	return g.excludeLocal
}

// CompositeModelName 复合模型名（客户端指定走脑手编排）
const CompositeModelName = "zerg-baiyan"

// circuitCooldown 熔断自动恢复时间（熔断后过冷却期自动重新尝试）。
// v2.5.5 #9: 60s→30s（瞬时失败快速恢复——X3 healthy 但超时误熔断后能快速半开试错）
const circuitCooldown = 30 * time.Second

// circuitFailThreshold 熔断阈值（连续失败次数——达到才熔断）。
// v2.5.5 #9: 3→5→8（X3 偶发转发失败（连接层——infer 实际 200）——阈值提高——心跳清零兜底）
const circuitFailThreshold = 8

// healthyTripLimit healthy 机器的熔断上限（失败数 >= 此值即使 healthy 也熔断）。
// v2.5.5 #9 补充3: X3 偶发转发失败——healthy 保护（失败<10 不熔断——只降权）
const healthyTripLimit = 10

// breakerReasonUnspecified 兜底原因的前缀：计数点没能给出原因时用它开头，而不是留空串。
// 禁用 unset/pending/tbd/todo/n/a/placeholder/unknown/待定/未定/none/null 等占位词
// （标准-模型接入与目录贡献 §五「禁占位符」——命中即 error）。
const breakerReasonUnspecified = "unspecified"

// breakerNoReasonText 快照对外文本：有熔断状态却一条原因都没记录时用它，绝不给空串。
// 空串会让人以为「没失败过」，而计数非零又说明失败过——两者矛盾就是这次要堵死的缺口。
const breakerNoReasonText = "not recorded by this failure path"

// startupGrace 主控启动宽限期（秒）——宽限期内忽略 unhealthy 快照（防重启窗口期误拒）
// v2.5.5 #9 补充2 治本: 主控重启瞬间 X3 心跳失败（API 未就绪窗口）→ 快照 unhealthy → 路由拒绝
const startupGrace = 30 * time.Second

// sessionBinding 会话绑定记录。
type sessionBinding struct {
	host     string    // 绑定的机器（如 "x3"）
	model    string    // 绑定时请求的模型
	lastUsed time.Time // 最近使用时间（过期清理）
	tokens   int       // 会话累计 token（V22 上下文预算）
}

// sessionTTL 会话绑定过期时间（超过后允许重新路由）。
const sessionTTL = 30 * time.Minute

// maxSessionTokens 会话 token 预算上限（超过触发 compaction 提示，V22-1）。
// 128K 覆盖典型 agent 会话；超预算由客户端或编排层 compaction。
const maxSessionTokens = 131072

// NewGateway 创建网关实例，附带本机后端。
// localBack 可为 nil（此时 host=local 的模型仍返回 503）。
// adapters v2.5.4.10：模型适配器注册表（模型名→插件）——可为 nil（回退旧路由）。
func NewGateway(authToken string, cfg *config.FleetConfig, localBack *localback.LocalBackend, st *store.Store, adapters map[string]plugin.Plugin) *Gateway {
	// v2.5.4.10 适配器注册表（nil→空 map——回退旧路由）
	if adapters == nil {
		adapters = map[string]plugin.Plugin{}
	}
	// 阶段 B：创建动作级路由表（默认配置）
	actionRouter := newActionModelRouter()
	// 阶段 C：脑手编排器（走网关统一路由——executor 带认证）
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig(),
		newGatewayExecutor(authToken))

	// M3 集中控制层（v2.4）：加载规则——nil 不启用；观察模式（记录不拦截——Mr2109确认策略后改拦截）
	var ctrlGate *control.Gate
	if gate, err := control.NewGateFromFile(filepath.Join(statepath.WorkspaceRoot(), "core", "internal", "control", "rules.yaml")); err == nil {
		ctrlGate = gate
		log.Printf("🔒 M3 central control layer loaded (observe mode — record only, no blocking)")
	} else {
		log.Printf("⚠️ M3 control layer load failed (disabled): %v", err)
	}

	g := &Gateway{
		authToken:       authToken,
		config:          cfg,
		actionRouter:    actionRouter,
		roundRobin:      make(map[string]int), // B13: 轮询计数器初始化
		orchestrator:    orch,
		gate:            ctrlGate, // M3 集中控制层（观察模式）
		adapterRegistry: adapters, // v2.5.4.10 模型适配器注册表
		client: &http.Client{
			// v2.5.6 故障自愈（Mr2109 2026-08-28）: 5min→45min——长生成（思考模型 13万token）
			// 之前 5min 总超时 + override 120s 覆盖整个请求——长生成必被杀——熔断风暴根因
			// 现在: 有 override 的模型走 overrideClient（首 token 超时=override——总超时 45min）
			//       无 override 的模型走这里（首 token 90s——总超时 45min）
			Timeout: 45 * time.Minute,
			// 内部转发永远直连：禁用代理（Go 默认 Transport 会读 macOS 系统代理，
			// 导致局域网 10.0.x 请求被发给代理 → no route to host）
			Transport: &http.Transport{
				Proxy: nil,
				// 治本（2026-08-12）：禁 keep-alive——X3 agent Keep-Alive: timeout=5
				// 连接 5s 空闲被 agent 关——网关复用断连接 → IncompleteRead（EOF 根因）
				DisableKeepAlives: true,
				// v2.5.4.9 C failover：首 token 超时（等响应头 90s——卡死检测）
				// 知识库经验: Codex 请求体大(64-79KB含tools)→example-35b-v2推理38-60s(正常——35B思考模型+工具调用)
				// 60s 误杀大请求（60.02s 触发——实际快好了）——调 90s 给余量
				// 真卡死: 完全无响应头 90s——判定挂起——failover 换机器
				ResponseHeaderTimeout: 90 * time.Second,
			},
		},
		localBack:       localBack,
		store:           st,
		sessions:        make(map[string]sessionBinding),
		prefixes:        make(map[string]map[string]int),
		failCounts:      make(map[string]int),
		modelFailCounts: make(map[string]int),
		failSince:       make(map[string]time.Time),
		lastErr:         make(map[string]string),
		lastErrAt:       make(map[string]time.Time),
		lastErrSrc:      make(map[string]string),
		startupTime:     time.Now(), // v2.5.5 重启窗口期治本: 启动宽限期起点
		// 丙批 N4 / C2：前缀命中率闭环（窗口 50 次 / 10 分钟——见 prefix_cache.go）
		// C2：带跨重启持久化（statepath ~/.zerg/state/prefix_cache.json + 节流写）
		prefixCache: newPrefixCachePersistent(defaultPrefixWindowN, defaultPrefixWindowDur),
	}
	// v2.5.6 适配器覆盖配置恢复（启动重放——2026-08-27 Mr2109）
	g.loadAdapterOverrides()
	return g
}

// SetCompressor 设置 LLMLingua-2 压缩器（V22，主控启动时调用，加载 ONNX 模型）。
func (g *Gateway) SetCompressor(c *compressor.Compressor) {
	g.compMu.Lock()
	g.comp = c
	g.compMu.Unlock()
	if c != nil {
		// 加载 ONNX 模型（异步，不阻塞启动）
		go func() {
			if err := c.Load(); err != nil {
				log.Printf("⚠️ compressor load failed (continuing with gemma fallback): %v", err)
			}
		}()
	}
}

// compressWithLLMLingua2 用 LLMLingua-2 压缩文本（引擎无关，Go 进程内 ONNX 推理）。
// 未加载或失败时返回 error，调用方 fallback 到 gemma 摘要。
func (g *Gateway) compressWithLLMLingua2(text string) (string, error) {
	g.compMu.RLock()
	c := g.comp
	g.compMu.RUnlock()
	if c == nil {
		return "", fmt.Errorf("compressor not configured")
	}
	compressed, _, _, err := c.Compress(text)
	if err != nil {
		return "", fmt.Errorf("LLMLingua-2 compression failed: %w", err)
	}
	if compressed == "" || len(compressed) >= len(text)*90/100 {
		// 压缩无效或压缩率过低（<10%），视为失败（gemma 兜底）
		return "", fmt.Errorf("LLMLingua-2 compression ratio too low")
	}
	return compressed, nil
}

// RegisterRoutes 注册路由到 chi 路由器。
func (g *Gateway) RegisterRoutes(r *chi.Mux) {
	// 认证中间件应用到 /v1/*
	r.Group(func(r chi.Router) {
		r.Use(g.authMiddleware)

		// POST /v1/chat/completions（OpenAI Chat）
		r.Post("/v1/chat/completions", g.handleRequest)

		// POST /v1/messages（Claude Messages）
		r.Post("/v1/messages", g.handleRequest)

		// POST /v1/responses（OpenAI Responses）
		r.Post("/v1/responses", g.handleRequest)

		// GET /v1/models（模型列表，Codex/客户端需要）
		r.Get("/v1/models", g.handleModels)

		// POST /v1/context/compact（V22-2 上下文压缩：调虫族模型压缩长会话为摘要）
		r.Post("/v1/context/compact", g.handleCompact)

		// GET /v1/context/{session_id}（V22 客户端主动查询会话 token 状态）
		r.Get("/v1/context/{session_id}", g.handleContextQuery)

		// 丙批 N4：GET /api/metrics/prefix_cache（前缀命中率闭环查询——网关自有 mux）
		// 与项目约定一致：/api/* 需认证（X-Auth-Token / Bearer / x-api-key）
		r.Get("/api/metrics/prefix_cache", g.handlePrefixCacheMetrics)
	})
}

// handleContextQuery 处理 GET /v1/context/{session_id}，返回会话 token 状态（客户端主动查询）。
func (g *Gateway) handleContextQuery(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		http.Error(w, `{"error":"session_id 不能为空"}`, http.StatusBadRequest)
		return
	}
	// 用会话绑定模型的动态阈值（ctx_window/4槽/50%）
	boundModel := ""
	g.sessionMu.RLock()
	if b, ok := g.sessions[sessionID]; ok {
		boundModel = b.model
	}
	g.sessionMu.RUnlock()

	threshold := g.compactThreshold(boundModel)
	out, _ := json.Marshal(map[string]interface{}{
		"session_id":  sessionID,
		"tokens":      g.sessionTokens(sessionID),
		"max_tokens":  threshold,
		"over_budget": g.sessionTokens(sessionID) > threshold,
		"status":      "ok",
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// handleModels 处理 GET /v1/models，返回路由表全部模型。
func (g *Gateway) handleModels(w http.ResponseWriter, r *http.Request) {
	var data []map[string]interface{}
	for name, candidates := range g.config.Models {
		for _, c := range candidates {
			data = append(data, map[string]interface{}{
				"id":       name,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": c.Host,
				"backend":  c.Backend,
			})
		}
	}
	resp, _ := json.Marshal(map[string]interface{}{"data": data})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// handleCompact 处理 POST /v1/context/compact（V22-2 上下文压缩）。
// 接收 {session_id, messages}，调虫族压缩模型（gemma 轻量）生成摘要，
// 返回 {summary}——引擎无关（走网关内部调用，不依赖 llama 特性）。
func (g *Gateway) handleCompact(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var req struct {
		SessionID string        `json:"session_id"`
		Messages  []interface{} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"解析请求失败"}`, http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		http.Error(w, `{"error":"messages 不能为空"}`, http.StatusBadRequest)
		return
	}

	// 压缩模型：选 gemma（轻量快）或 example-35b-v2（更懂上下文）。有 gemma 用 gemma，否则 example-35b-v2。
	compactModel := "gemma-4-12B"
	if _, ok := g.config.Models[compactModel]; !ok {
		compactModel = "example-35b"
	}

	// 构造压缩 prompt：结构化摘要（固定类别：目标/进展/决策/关键事实/待办/文件引用）
	msgsJSON, _ := json.Marshal(req.Messages)
	compactPrompt := BuildStructuredSummaryPrompt(string(msgsJSON))

	// 调虫族网关内部（复用路由/转发）
	compactBody := map[string]interface{}{
		"model":      compactModel,
		"messages":   []interface{}{map[string]interface{}{"role": "user", "content": compactPrompt}},
		"max_tokens": 2000,
	}
	compactBodyJSON, _ := json.Marshal(compactBody)

	// 内部转发（不走外部 HTTP，直接路由到机器）
	model := compactModel
	// 压缩模型优先走 local（本机）——X3 agent 响应截断 bug（issue-3889字节）导致压缩不可靠
	// local 不可用（本机没加载 gemma）时 fallback 到 X3
	route, err := g.pickRouteLocal(model)
	if err != nil {
		route, err = g.pickRoute(model, "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"routing failed: %v"}`, err), http.StatusServiceUnavailable)
			return
		}
	}
	log.Printf("📎 compaction route: %s (%s)", route.Host, route.URL)

	var resp *http.Response
	var respBody []byte
	// 压缩模型可能忙（gemma 单槽）+ X3 agent 响应偶发截断（issue-3889字节）——重试 5 次
	// 注意：截断是"成功返回但 body 不完整"（err=nil），必须解析失败也重试
	for attempt := 0; attempt < 5; attempt++ {
		if route.Host == "local" {
			if g.localBack == nil {
				http.Error(w, `{"error":"本机后端不可用"}`, http.StatusServiceUnavailable)
				return
			}
			// 加载压缩模型（如果未加载或模型不同）
			if err := g.loadModel(compactModel, route); err != nil {
				log.Printf("⚠️ compaction local model load failed: %v", err)
				http.Error(w, fmt.Sprintf(`{"error":"compaction model load failed: %v"}`, err), http.StatusServiceUnavailable)
				return
			}
			resp, err = g.localBack.Infer("/v1/chat/completions", compactBodyJSON)
		} else {
			resp, err = g.forwardToBackend(r.Context(), route, "/v1/chat/completions", compactBodyJSON, r.Header, nil)
		}
		if err != nil {
			log.Printf("⚠️ compaction attempt %d failed: %v (retrying)", attempt+1, err)
			if attempt < 2 {
				time.Sleep(2 * time.Second)
			}
			continue
		}
		// 读 body 并校验 JSON 完整性（截断 → 重试）
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		var chk map[string]interface{}
		if json.Unmarshal(respBody, &chk) == nil {
			break // 完整响应
		}
		log.Printf("⚠️ compaction attempt %d got incomplete response (%d bytes, truncated), retrying", attempt+1, len(respBody))
		if attempt < 2 {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"compaction failed: %v"}`, err), http.StatusBadGateway)
		return
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(respBody, &obj); err != nil {
		log.Printf("⚠️ handleCompact parse failed: %v, first 200 bytes: %s", err, string(respBody)[:min(len(respBody), 200)])
		http.Error(w, `{"error":"压缩响应解析失败"}`, http.StatusBadGateway)
		return
	}
	choices, _ := obj["choices"].([]interface{})
	if len(choices) == 0 {
		http.Error(w, `{"error":"压缩无结果"}`, http.StatusBadGateway)
		return
	}
	msg, _ := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	summary, _ := msg["content"].(string)
	if summary == "" {
		summary, _ = msg["reasoning_content"].(string)
	}

	// 返回摘要；会话 token 计数重置（压缩后上下文变小）
	if req.SessionID != "" {
		g.resetSessionTokens(req.SessionID)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"summary":    summary,
		"model":      compactModel,
		"session_id": req.SessionID,
		"status":     "ok",
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// 支持三种认证头：
//   - X-Auth-Token: <token>（Zerg 内部标准）
//   - Authorization: Bearer <token>（OpenAI 标准）
//   - x-api-key: <token>（Claude 标准）
//
// v1 血泪教训：urllib 会自动规范化 X-Api-Key → X-Api-Key（小写首字母+大写其余），
// 导致匹配失败。Go 的 http.Header.Get 天然大小写不敏感，直接用即可。
func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 提取认证 token（三种认证方式）

		// 方式 1：X-Auth-Token header（Zerg 内部标准）
		// Go 的 http.Header.Get 自动忽略大小写，无需手动处理
		token := r.Header.Get("X-Auth-Token")

		// 方式 2：Authorization: Bearer <token>（OpenAI 标准）
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token = strings.TrimSpace(authHeader[len("bearer "):])
			}
		}

		// 方式 3：x-api-key header（Claude 标准）
		// v1 被 urllib 的 X-Api-Key 规范化坑过，Go 天然不敏感
		if token == "" {
			token = r.Header.Get("x-api-key")
		}

		// 验证 token 是否匹配
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized: missing authentication"}`))
			return
		}

		if token != g.authToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized: invalid token"}`))
			return
		}

		// 认证通过，继续处理请求
		next.ServeHTTP(w, r)
	})
}

// handleRequest 统一请求处理：提取 model → 路由 → 转发 → 透传响应。
//
// 处理流程：
//  1. 读取请求体（大 body 支持：按 Content-Length 循环读）
//  2. 提取 body.model 字段
//  3. pickRoute 选择目标机器
//  4. host=local → 通过 LocalBackend 转发（阶段 3 实现）
//  5. 转发到子端 :8100/infer（body + " _path" 字段）
//  6. 响应透传（非流式完整 / 流式逐 chunk）
func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 1. 读取请求体（大 body 支持：按 Content-Length 循环读，不截断）
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request body: %v", err)
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 1.5 客户端适配器识别（路径/UA/body 格式 → codex/chat/claude/generic）
	adp := adapter.Dispatch(r, body)
	log.Printf("🔌 client adapter: %s (%s)", adp.Name(), r.URL.Path)

	// 2. 提取 model 字段（三种标准都在 body.model）
	// 阶段 B：如果未显式指定 model，走动作路由解析
	model, usedAction, err := g.extractModelWithAction(body, r.URL.Path)
	if err != nil {
		log.Printf("failed to extract model: %v", err)
		adp.TransformError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("failed to extract model: %v", err))
		return
	}
	if usedAction {
		log.Printf("🎯 request model (action routing): %s", model)
	} else {
		log.Printf("🎯 request model: %s", model)
	}

	// 2.5 入站转换（客户端格式 → 后端格式；Claude 全量转 chat，Codex 基本透传）
	forwardBody, backendPath, err := adp.TransformRequest(r, body)
	if err != nil {
		log.Printf("inbound transform failed: %v", err)
		adp.TransformError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	// 2.6 模型别名解析：客户端可能用别名请求（Hermes 发 zerg-example-35b-v2 等）。
	// 不仅路由要用标准名，转发给后端的 body 里的 model 字段也要替换（否则 agent 找不到模型）。
	if canonical, ok := g.config.Aliases[model]; ok {
		log.Printf("🎭 model alias: %s → %s", model, canonical)
		model = canonical
		forwardBody = adapter.JsonSetField(forwardBody, "model", canonical)
	}

	// 2.7 能力硬门槛（待修补 #11 / #38 ①）：请求里出现结构化图像部件 → 必需 vision。
	// 只有"必要性升档"（不升就做不了）走硬门槛；判定在路由选择处按**目标引擎**进行。
	// 无必需能力时为零开销（required 为空）。
	//
	// 待修补 #38 ①：本求解必须放在复合模型分支**之前**。历史上它在 2.8（复合分支之后），
	// 而 zerg-baiyan 分支在算必需能力之前就 return——带图请求于是整条绕过门槛。
	required := RequiredCapabilitiesFromRequest(body)
	if len(required) > 0 {
		log.Printf("🧱 request requires capabilities: %v (hard gate, per target engine)", required)
	}

	// 2.65 复合模型：model=zerg-baiyan → 脑手编排器（decompose→dispatch→synthesize）
	// 不走常规路由——编排器内部按脑/手子任务调不同模型
	if model == CompositeModelName && g.orchestrator != nil {
		// 能力硬门槛（待修补 #38 ①）：复合模型路径只取**纯文本** prompt 交给编排器——
		// extractLastUserPrompt/extractPrompt 把 content 反序列化进 string，而带图请求的
		// content 是数组（[{"type":"image_url",...}]）→ 反序列化失败 → prompt 为空。
		// 即：带图请求会**静默丢图**并降级成一条空 prompt 文本请求；且复合模型不在 fleet 表里，
		// 无目标引擎、无能力证据可判。⇒ 有必需能力时一律 fail-closed（capability_unavailable/400，
		// 与 #11 同一套原因码），明确报因，绝不静默降级。纯文本请求（required 为空）行为一字不变。
		if len(required) > 0 {
			ge := &CapabilityGateError{
				Model:      model,
				Engine:     "orchestrator", // 复合模型不是 fleet 引擎——无目标引擎可解析
				Capability: strings.Join(required, ","),
				Reason:     modelreg.CapReasonNoSnapshot, // 复合模型无能力快照 → 不可判定
				Trace:      fmt.Sprintf("capability gate: composite model %q runs an orchestrator (not a fleet engine) — no capability snapshot / engine evidence for required capabilities %v; structured image parts would be silently dropped here (fail-closed)", model, required),
			}
			log.Printf("🚧 capability gate BLOCK model=%s engine=%s capability=%s reason=%s | %s", ge.Model, ge.Engine, ge.Capability, ge.Reason, ge.Trace)
			code, status := classifyRouteError(ge)
			adp.TransformError(w, status, code, ge.Error())
			return
		}
		log.Printf("🧠 composite model: %s → brain-hand orchestrator", model)
		// 提取用户消息（最后一条 user 内容）
		prompt := extractLastUserPrompt(body)
		if prompt == "" {
			prompt = extractPrompt(body)
		}
		result, err := g.orchestrator.Execute(ctxFromReq(r), prompt, 4000)
		if err != nil {
			log.Printf("orchestrator execution failed: %v", err)
			adp.TransformError(w, http.StatusInternalServerError, "orchestration_error", fmt.Sprintf("orchestration failed: %v", err))
			return
		}
		// 返回 OpenAI 兼容响应（chat 格式——客户端用 /v1/chat/completions 请求）
		// MoA（白眼）：CombinedResult = example-35b-v2 聚合最终回答；Summary 仅元信息
		finalContent := result.CombinedResult
		if strings.TrimSpace(finalContent) == "" {
			finalContent = result.Summary
		}
		respJSON := fmt.Sprintf(`{"id":"chatcmpl_%s","object":"chat.completion","created":%d,"model":"zerg-baiyan","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}],"usage":{"total_tokens":%d}}`,
			"orchestrator", time.Now().Unix(), quoteJSON(finalContent), result.TotalTokens)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(respJSON))
		log.Printf("🧠 orchestration done: %d subtasks, %v", len(result.TaskResults), result.TotalTime)
		return
	}

	// 3. 路由选择：pickRoute（会话粘性 → 已加载→空闲→负载低 → cache-aware）
	sessionID := extractSessionID(body)
	prompt := extractPrompt(body)

	// v2.5.4.10 方案 B：适配器参数覆盖（有适配器——按适配器特征覆盖请求体）
	// 无适配器 → 原样（fleet.yaml 默认——兼容旧模型）
	if ada, ok := g.adapterRegistry[model]; ok {
		out, aerr := ada.Execute(plugin.PluginInput{Data: map[string]any{"model": model}})
		if aerr != nil {
			// v2.5.6 错误码设计（2026-08-29）: 适配器执行失败不能静默——记日志（覆盖失败用默认参数——不阻塞请求）
			log.Printf("⚠️ adapter %s execution failed (using defaults): %v", model, aerr)
		}
		if aerr == nil && out.Result != nil {
			if res, ok := out.Result.(map[string]any); ok {
				// 温度/采样参数覆盖
				if t, ok := res["temperature"].(float64); ok {
					forwardBody = adapter.JsonSetField(forwardBody, "temperature", t)
					log.Printf("🎛️ adapter %s: temperature override %.2f", model, t)
				}
				// v2.5.6 修复（Mr2109 2026-08-28）: max_tokens 同时覆盖两种格式——
				// 之前只写 max_tokens（chat 格式）——调度器用 max_output_tokens（responses 格式）
				// 字段不匹配 → 适配器 32768 从未覆盖调度器请求 → 适配器形同虚设
				// 适配器 = 参数唯一来源（程序调用参数由适配器决定——每步都生效）
				if mt, ok := res["max_tokens"].(int); ok && mt > 0 {
					forwardBody = adapter.JsonSetField(forwardBody, "max_tokens", mt)
					forwardBody = adapter.JsonSetField(forwardBody, "max_output_tokens", mt)
					log.Printf("🎛️ adapter %s: max_tokens override %d (chat+responses both)", model, mt)
				}
				// reasoning_effort 覆盖（思考深度——适配器声明——Mr2109 low）
				if re, ok := res["reasoning_effort"].(string); ok && re != "" {
					forwardBody = adapter.JsonSetField(forwardBody, "reasoning_effort", re)
				}
				// 超时覆盖（按模型——Qwen3.8 120s / Nemotron 60s——适配器声明）
				if ts, ok := res["timeout_sec"].(int); ok && ts > 0 {
					g.setRequestTimeout(model, ts)
				}
			}
		}
	}

	// 2.7 快路径逐字精简（渐进式压缩：50%前每次请求前轻量删噪）
	// 引擎无关：纯规则（工具输出 observation masking + 填充回复删除），毫秒级
	if trimmed := trimRequestMessages(&forwardBody); trimmed {
		log.Printf("✂️ fast-path trim: tool output / filler replies de-noised (%s)", sessionID)
	}

	// 2.8 必需能力 required 已在 2.7 求解（待修补 #38 ①：前移到复合模型分支之前，
	// 否则复合分支在求解前就 return，带图请求整条绕过门槛）。此处直接用于按**目标引擎**的路由硬门槛。
	route, err := g.pickRoute(model, sessionID, prompt, required...)
	if err != nil {
		log.Printf("route selection failed: %v", err)
		// v2.5.6 故障自愈（Mr2109 2026-08-28）: 错误码语义化——调度器按 code 分类处理
		// 熔断/无候选/被排除 = 环境故障（503 circuit_open——可等——waiting_retry）
		// 模型不在路由表 = 配置问题（404 model_not_found——不可重试）
		msg := fmt.Sprintf("route selection failed: %v", err)
		code, status := classifyRouteError(err)
		adp.TransformError(w, status, code, msg)
		return
	}

	// 4. host=local 的模型：通过 LocalBackend 转发到本机 llama-server
	if route.Host == "local" {
		if g.localBack == nil {
			handleLocalModel(w)
			return
		}
		// 加载模型（如果未加载或模型不同）
		if err := g.loadModel(model, route); err != nil {
			log.Printf("failed to load local model: %v", err)
			// v2.5.6 故障自愈: 加载失败=资源/环境故障（可等——OOM/冷加载——waiting_retry）
			adp.TransformError(w, http.StatusServiceUnavailable, "circuit_open", fmt.Sprintf("failed to load local model: %v", err))
			return
		}
		// 通过 LocalBackend 转发
		g.forwardToLocal(w, r, route, forwardBody, body, adp)
		return
	}

	log.Printf("📍 routed to: %s:%d", route.Host, route.Port)

	// 会话粘性：记录绑定（仅非 local 机器）
	if sessionID != "" && route.Host != "local" {
		g.bindSession(sessionID, route.Host, model)
	}
	// cache-aware：记录该机器处理过的 prompt 前缀签名
	if prompt != "" && route.Host != "local" {
		g.recordPrefix(route.Host, prompt)
	}

	// 5. 转发到子端（适配器已做入站转换；含 tools 强制非流式由 chat 适配器在出站处理）
	hasTools := adapter.ContainsTools(forwardBody)
	log.Printf("🔍 request check: tools=%v stream=%v bodyLen=%d", hasTools, adapter.IsStreamRequest(forwardBody), len(forwardBody))

	// M3 集中控制层（v2.4）：工具调用拦截检查——观察模式（记录不拦截——等Mr2109确认策略后启用）
	// gate 检查请求里的工具调用——block 记录告警；require_approval 记录待审（暂不拦截——默认放行保持现有行为）
	if hasTools && g.gate != nil {
		for _, toolName := range adapter.ExtractToolNames(forwardBody) {
			decision := g.gate.Check(toolName, "", "agent")
			if decision.Action != control.ActionAllow {
				log.Printf("🔒 M3 control[observe]: tool %s → %s (%s) — not blocking (observe mode)",
					toolName, decision.Action, decision.Message)
			}
		}
	}

	// 转发 + 失败重试（T7）：失败记录熔断计数，重路由到次优机器
	// v2.5.5 T1 连接层治本: maxRetries 2→3——连接层失败重试不立即熔断（重试成功就不算失败）
	const maxRetries = 3
	var resp *http.Response
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 重试：重新路由（熔断机器已被跳过）
			log.Printf("🔄 retry %d: re-routing (%s)", attempt, model)
			route, err = g.pickRoute(model, sessionID, prompt, required...)
			if err != nil {
				break
			}
			log.Printf("📍 retry routed to: %s:%d", route.Host, route.Port)
		}

		resp, err = g.forwardToBackend(r.Context(), route, backendPath, forwardBody, r.Header, required)
		if err == nil && resp != nil && resp.StatusCode < 500 {
			break // 转发成功（非 5xx）
		}
		// v2.5.5 T5b: 5xx（后端/模型错误）也触发重试换机器（另一台可能正常）
		if err == nil && resp != nil && resp.StatusCode >= 500 {
			log.Printf("⚠️ backend %s returned %d — trying another machine (T5b 5xx failover)", route.Host, resp.StatusCode)
			// 5xx 是**模型级**错误（引擎自报，如 ds4 的 rocm prefill failed）——
			// 记入 modelFailCounts 而**不涨熔断**（设计 §11 M4）：不该因"这个模型跑不了"就 demote 整机。
			if route.Host != "local" {
				g.markModelFailure(route.Host, fmt.Sprintf("backend %s returned %d", route.Host, resp.StatusCode))
			}
			// v2.5.6 错误码设计（2026-08-29）: 读取子端错误体——透传真实错误消息（调试关键）
			// 子端错误如 {"error":"queue full"}(429) / {"error":"inference timeout"}(504)——
			// 不读 body 则客户端只见"后端 x3 返回 500"——根因丢失
			bodyMsg := ""
			if b, rerr := io.ReadAll(resp.Body); rerr == nil && len(b) > 0 {
				bodyMsg = strings.TrimSpace(string(b))
				if len(bodyMsg) > 200 {
					bodyMsg = bodyMsg[:200]
				}
			}
			resp.Body.Close()
			if bodyMsg != "" {
				err = fmt.Errorf("backend %s returned %d: %s", route.Host, resp.StatusCode, bodyMsg)
			} else {
				err = fmt.Errorf("backend %s returned %d", route.Host, resp.StatusCode)
			}
		}

		log.Printf("forward request failed: %v", err)
		// v2.5.5 T1 连接层治本: 连接层失败（网络/超时）——重试不立即熔断（给足机会）
		// 模型错误（HTTP 响应——4xx/5xx）才立即 markFailure（真失败）
		if route.Host != "local" && isModelError(err) {
			reason := "model error (unspecified)"
			if err != nil {
				reason = err.Error()
			}
			// 模型级（后端 4xx/5xx —— 引擎/子端自报的语义错误）⇒ **不计熔断**，只记原因（§11 M4）
			g.markModelFailure(route.Host, reason)
		}
		// 连接层失败——不 markFailure（重试成功就没事——偶发网络不熔断）
	}
	if err != nil {
		// 待修补 #38 ②：换机（或重试重路由）被能力硬门槛拦下时，透出统一原因码
		// capability_unavailable/400（与 #11 同一套），而不是让它被下面默认的 502 upstream_fail 盖掉。
		// 该分支只对 *CapabilityGateError 生效——其余错误路径行为一字不变。
		var gateErr *CapabilityGateError
		if errors.As(err, &gateErr) {
			adp.TransformError(w, http.StatusBadRequest, "capability_unavailable", gateErr.Error())
			return
		}
		// v2.5.6 故障自愈（Mr2109 2026-08-28）: 错误码语义化——按错误类型区分
		//   circuit_open    (503) 熔断/无候选——可等（waiting_retry）
		//   upstream_fail   (502) 转发失败——可重试（换机/换模型）
		//   bad_request     (400) 参数错——不可重试
		code := "upstream_fail"
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "熔断") || strings.Contains(err.Error(), "无可用候选") {
			code = "circuit_open"
			status = http.StatusServiceUnavailable
		} else if strings.Contains(err.Error(), "bad request") || strings.Contains(err.Error(), "参数") {
			code = "bad_request"
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "queue full") || strings.Contains(err.Error(), "429") {
			// v2.5.6 错误码设计（2026-08-29）: 子端 queue full(429)=单槽被占——可等（busy）
			code = "busy"
			status = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "model not found") || strings.Contains(err.Error(), "missing model") || strings.Contains(err.Error(), "404") {
			// v2.5.6 错误码设计: 子端模型不存在/缺失——不可重试（model_not_found）
			code = "model_not_found"
			status = http.StatusNotFound
		}
		adp.TransformError(w, status, code, fmt.Sprintf("forward request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	// 转发成功：恢复机器权重
	if route.Host != "local" {
		g.markSuccess(route.Host)
	}

	// 6. 出站转换（后端响应 → 客户端格式，由适配器完成）
	// V22 token 预算：内部累计 + 按模型窗口 50% 自动 compaction（不通知客户端）
	// （autoCompact 逻辑不变；respBody 提到外层以便复用——丙批前缀命中率也读它）
	var respBody []byte
	if sessionID != "" {
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		if tok := extractUsageTokensFromBytes(respBody); tok > 0 {
			g.addSessionTokens(sessionID, tok)
			// 丙批 §4.2：网关侧是**安全网**（85% 兜底，且 history ≥ 4 条才动）——不与对话侧主压缩抢跑
			if threshold := g.compactThreshold(model); threshold > 0 && g.sessionTokens(sessionID) > threshold && requestMessageCount(body) >= 4 {
				slog.Info("session exceeded compaction threshold, auto-compacting", "session", sessionID, "tokens", g.sessionTokens(sessionID), "threshold", threshold)
				log.Printf("♻️ session %s exceeded compaction threshold (%d/%d), auto-compacting", sessionID, g.sessionTokens(sessionID), threshold)
				g.autoCompact(sessionID, model, body)
			}
		}
	}
	// 丙批 N4：前缀命中率闭环——解析上游缓存计量（非流式响应才缓冲；流式交给适配器直通，不破坏 SSE）
	if g.prefixCache != nil && respBody == nil && !adapter.IsStreamResponse(resp) {
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	if g.prefixCache != nil && len(respBody) > 0 {
		g.recordPrefixCache(model, forwardBody, respBody)
	}
	// 丙批 N4 补齐（2026-09-10）：流式响应也采样——tee 只留尾部窗口，响应结束后取末块 timings/usage
	var usageTee *sseUsageTee
	if g.prefixCache != nil && adapter.IsStreamResponse(resp) {
		usageTee = newSSEUsageTee(resp.Body)
		resp.Body = usageTee
	}
	adp.TransformResponse(w, resp, r, body)
	if g.prefixCache != nil && usageTee != nil {
		if ub := usageTee.UsageJSON(); len(ub) > 0 {
			g.recordPrefixCache(model, forwardBody, ub)
		}
	}
}

// extractModel 从请求体提取 model 字段。
// 三种标准（OpenAI Chat / Claude Messages / OpenAI Responses）都在 body.model。
func extractModel(body []byte) (string, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", fmt.Errorf("failed to parse request body JSON: %w", err)
	}

	model, ok := req["model"].(string)
	if !ok || model == "" {
		return "", fmt.Errorf("request body missing model field")
	}

	return model, nil
}

// extractModelWithAction 尝试提取 model，如果缺失则走动作路由解析。
// 返回模型名和是否使用了动作路由（usedAction 为 true 表示走了动作路由）。
// 设计：只有当客户端未显式指定 model 时，才使用动作路由；已指定的尊重客户端选择。
func (g *Gateway) extractModelWithAction(body []byte, path string) (model string, usedAction bool, err error) {
	// 先尝试标准提取
	model, err = extractModel(body)
	if err == nil {
		// 标准提取成功，直接使用（尊重客户端选择）
		return model, false, nil
	}

	// 标准提取失败（缺少 model 字段），尝试动作路由
	log.Printf("🔍 no model specified, trying action routing: %v", err)

	action := detectAction(body, path)
	log.Printf("🎯 action type recognized: %s", action.String())

	// 通过动作路由解析目标模型
	resolved := g.actionRouter.resolveModel(action)
	if resolved == "" {
		return "", false, fmt.Errorf("action routing found no target model: %s", action.String())
	}

	log.Printf("✅ action routing resolved: %s → %s", action.String(), resolved)
	return resolved, true, nil
}

// pickRoute 路由选择：已加载→空闲→负载低。
//
// 路由策略：
//  1. 在 fleet.yaml 的 models 表中查找模型
//  2. 如果有多个候选 host，按优先级选择：
//     - 已加载该模型的 host 优先
//     - 同级别按负载（mem_gb）选择负载最低的
//  3. host=local 的模型：阶段 3 实现，先返回 503
//
// 阶段 1 已有 FleetConfig.Models 结构，这里复用。
// markFailure 记录机器转发失败（熔断计数）。
// reason 必填（不是可选参数）：本函数会涨 failCounts，而快照要把「为什么熔断」展示给人看——
// 让「只计数不记原因」在编译期就不可能（历史上 reason 是可变参数，漏传就成了静默计数）。
// 若调用点只能给出空串（动态拼接失败），breakerCountReason 会自动补一条带调用点的明确文本，
// 绝不写空原因。src 记录计数路径名（供快照 last_error_source 取证）。
func (g *Gateway) markFailure(host string, reason string) {
	g.failMu.Lock()
	defer g.failMu.Unlock()
	g.failCounts[host]++
	g.recordFailureReasonLocked(host, "markFailure", reason, 2)
	slog.Warn("machine forward failed", "host", host, "fail_count", g.failCounts[host], "threshold", circuitFailThreshold)
	log.Printf("%s", breakerFailLogLine(host, g.failCounts[host]))
}

// markModelFailure 记录一次**模型级失败**（引擎自己报的 5xx、子端 507 拒装等）。
//
// 与 markFailure 的关键区别：**不涨熔断计数**，因此不会把整机 demote 掉。
// 设计依据：《设计-子端服务切换与基线服务声明》§11 M4 ——
//
//	「上游不可达/连接失败 ⇒ 计入熔断；引擎 5xx（模型级：rocm prefill failed、
//	  insufficient memory）⇒ 记错 + 有限重试，不计熔断。」
//
// 2026-09-14 实测教训：DS4@1M 遭遇 rocm prefill failed，被记成 machine x3 forward failed (8/8)
// 并 demote（随后靠心跳复位）——而问题只在"该模型 + 该上下文"，不该牵连整机。
func (g *Gateway) markModelFailure(host string, reason string) {
	g.failMu.Lock()
	defer g.failMu.Unlock()
	g.modelFailCounts[host]++
	g.recordFailureReasonLocked(host, "markModelFailure", reason, 2)
	slog.Warn("model-level failure (not counted toward breaker)",
		"host", host, "model_fail_count", g.modelFailCounts[host], "reason", reason)
	log.Printf("⚠️ 模型级失败（不计熔断）: %s — %s", host, reason)
}

// breakerCountReason 保证「涨计数」的路径一定带上可读原因。
//
// 为什么需要它：失败/熔断计数有几条互相独立的增长路径（markFailure / tripMachine /
// pickRouteExcluding 的强制熔断），历史上 tripMachine 与 pickRouteExcluding 不写 lastErr，
// 于是活系统上出现过 fail_count=11（= healthyTripLimit+1，正是 pickRouteExcluding 的强制熔断值）
// 而 last_error 为空串的迷惑状态。规矩：计数点要么给出原因，要么被自动补一条带调用点的明确文本。
// 禁用 unset/pending/tbd/todo/n/a/placeholder/unknown/待定/未定/none/null 这类占位词
// （标准-模型接入与目录贡献 §五「禁占位符」——命中即 error），本函数用 unspecified。
//
// skip：runtime.Caller 的帧数（0=本函数，1=调用本函数的计数点，2=计数点的调用方）。
func breakerCountReason(reason string, skip int) string {
	if s := strings.TrimSpace(reason); s != "" {
		return s
	}
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return breakerReasonUnspecified + " (count path gave no reason; caller site unknown)"
	}
	return fmt.Sprintf("%s at %s:%d (count path gave no reason)", breakerReasonUnspecified, filepath.Base(file), line)
}

// recordFailureReasonLocked 写入一次失败原因（调用方必须已持有 g.failMu）。
// src 是计数路径名（markFailure/tripMachine/pickRouteExcluding）——快照用它回答「哪个计数点涨的」。
func (g *Gateway) recordFailureReasonLocked(host, src, reason string, skip int) {
	if g.lastErr == nil {
		g.lastErr = map[string]string{}
	}
	if g.lastErrAt == nil {
		g.lastErrAt = map[string]time.Time{}
	}
	if g.lastErrSrc == nil {
		g.lastErrSrc = map[string]string{}
	}
	g.lastErr[host] = breakerCountReason(reason, skip+1)
	g.lastErrAt[host] = time.Now()
	g.lastErrSrc[host] = src
}

// breakerFailLogLine 组装熔断失败日志行，阈值一律取自真实常量 circuitFailThreshold。
//
// 历史上这里写死过 3，而真实阈值是 circuitFailThreshold=8——日志与判定不符会误导运维。
// 把格式化收敛到本函数，测试即可断言「输出与常量一致」，杜绝再次漂移。
func breakerFailLogLine(host string, failCount int) string {
	return fmt.Sprintf("🚨 machine %s forward failed (%d/%d); exceeding threshold will demote", host, failCount, circuitFailThreshold)
}

// markSuccess 清零机器失败计数（转发成功后调用，恢复权重）。
func (g *Gateway) markSuccess(host string) {
	g.failMu.Lock()
	defer g.failMu.Unlock()
	if g.failCounts[host] != 0 {
		log.Printf("✅ machine %s recovered, failure count reset", host)
		g.failCounts[host] = 0
	}
	delete(g.failSince, host)
	// 失败已恢复——清原因（快照不显示陈旧原因）
	delete(g.lastErr, host)
	delete(g.lastErrAt, host)
	delete(g.lastErrSrc, host)
}

// ClearFailures 清零机器失败计数（v2.5.5 #9 补充5: 心跳健康时调用——防残留熔断）。
func (g *Gateway) ClearFailures(host string) {
	g.failMu.Lock()
	defer g.failMu.Unlock()
	if g.failCounts[host] != 0 {
		log.Printf("✅ machine %s heartbeat healthy — failure count reset", host)
		g.failCounts[host] = 0
	}
	delete(g.failSince, host)
	// 心跳健康 = 机器活着——清失败原因（快照不显示陈旧原因）
	delete(g.lastErr, host)
	delete(g.lastErrAt, host)
	delete(g.lastErrSrc, host)
}

// isModelError 判断转发错误是否是"模型错误"（HTTP 响应错误——4xx/5xx）。
// v2.5.5 T1 连接层治本: 连接层失败（网络/超时/EOF）≠ 模型失败——不熔断（偶发网络重试就好）
func isModelError(err error) bool {
	if err == nil {
		return false
	}
	// HTTP 响应错误（模型返回 4xx/5xx）——真失败
	var httpErr interface{ StatusCode() int }
	if errors.As(err, &httpErr) {
		code := httpErr.StatusCode()
		return code >= 400 && code < 600
	}
	// 连接层错误（net.Error/超时/EOF——网络问题）——不是模型失败
	var netErr net.Error
	if errors.As(err, &netErr) {
		return false
	}
	// 其他（URL 错误等）——保守按模型错误处理（熔断——防持续失败）
	return true
}

// isTripped 判断机器是否处于熔断状态（连续失败 >= 阈值 且未过冷却期）。
// 超过冷却期自动恢复（半开状态：放行一次试错，成功则清零，失败则重新熔断）。
// v2.5.5 #9 补充3: healthy 机器不轻易熔断（快照 healthy + 失败数未超高 → 只降权不摘除）
// 但失败数 >= healthyTripLimit（10）仍熔断（持续失败必须摘除）
func (g *Gateway) isTripped(host string) bool {
	g.failMu.Lock()
	defer g.failMu.Unlock()
	// healthy 机器且失败数未超高 → 不熔断（瞬时失败保护）
	// v2.5.5 重启窗口期治本: 宽限期内也不熔断（机器还没机会心跳——启动窗口失败不算）
	if g.failCounts[host] < healthyTripLimit && time.Since(g.startupTime) < startupGrace {
		return false
	}
	if g.failCounts[host] < healthyTripLimit {
		if snap := g.snapshotFor(host); snap != nil && snap.Healthy {
			return false
		}
	}
	if g.failCounts[host] < circuitFailThreshold {
		return false
	}
	// 已过冷却期 → 自动恢复（尝试放行）
	if since, ok := g.failSince[host]; ok && time.Since(since) > circuitCooldown {
		log.Printf("♻️ machine %s cooldown elapsed, attempting auto-recovery", host)
		g.failCounts[host] = 0
		delete(g.failSince, host)
		return false
	}
	// 首次熔断时记录时间
	if _, ok := g.failSince[host]; !ok {
		g.failSince[host] = time.Now()
	}
	return true
}

// pickRoute 返回最优路由（失败重试时排除指定机器）。
//
// required（可选，待修补 #11）是本次请求的**必需能力**（必要性升档，硬门槛）。
// 非空时按**目标引擎**逐条判定：目标引擎没被证过支持的能力 → 该候选跳过（不静默降级），
// 所有候选都不合格 → fail-closed 返回 *CapabilityGateError（写明哪个引擎缺哪条能力证据）。
// 空则不判定，行为与既往一致。
func (g *Gateway) pickRoute(model string, sessionID string, prompt string, required ...string) (*RouteResult, error) {
	// 别名解析：客户端可能用别名请求（Hermes 发 zerg-example-35b-v2 等），映射到标准模型名
	if _, ok := g.config.Models[model]; !ok {
		if canonical, ok2 := g.config.Aliases[model]; ok2 {
			log.Printf("🎭 model alias: %s → %s", model, canonical)
			model = canonical
		}
	}

	// v2.5.6 DS4 让位机制（2026-08-23 Mr2109——Hermes 调 DS4 熔断——81G 超大）
	// 1. 目标模型 = DS4 → X3 清场（卸载其他只留 DS4）
	// 2. 其他模型请求 → 按 DS4 状态三分支（未加载→正常 / 闲置→卸 DS4 / 繁忙→local）
	if model == ds4ModelName {
		if !g.ensureDS4Room() {
			log.Printf("⚠️ DS4 quiesce failed — continuing to route (circuit-breaker fallback possible)")
		}
	} else {
		forceLocal, reason := g.ds4RouteDecision()
		if forceLocal {
			log.Printf("🎯 %s — other models %s go local", reason, model)
			// 强制 local（排除 X3）
			if r, err := g.pickRouteLocal(model); err == nil {
				// 能力硬门槛（待修补 #11）：强制 local 也要按**目标引擎**判必需能力——
				// 目标引擎没被证过支持就不放行，绝不因"让位"而静默降级。
				if gerr := g.gateRoute(model, r.Host, required); gerr != nil {
					log.Printf("🧱 capability gate blocked forced-local route: %v — falling through to gated routing", gerr)
				} else {
					return r, nil
				}
			}
			// local 不可用——回退正常路由（pickRouteExcluding x3）
			log.Printf("⚠️ local unavailable — falling back (excluding X3 — don't disturb DS4)")
			// 这条路径也会强制熔断（若选回 x3 → failCounts[x3]=11）——给原因，别让快照出现空原因
			if r, err := g.pickRouteExcluding(model, "x3", "DS4 quiesce: local unavailable, X3 temporarily excluded to avoid disturbing DS4", required...); err == nil {
				return r, nil
			}
		}
	}

	// 在 models 表中查找
	models, ok := g.config.Models[model]
	if !ok {
		return nil, fmt.Errorf("model %s not found in the routing table", model)
	}

	// 辅助函数：根据 host 名从 Fleet 获取节点（IP + 端口）
	getNode := g.fleetNode

	// 会话粘性（T4）：会话已绑定到某机器且该机器仍可用（健康 + 仍是该模型的候选）→ 直接复用
	if sessionID != "" {
		if bound := g.boundSession(sessionID, model); bound != "" {
			// 确认绑定机器仍是该模型的候选
		affinity:
			for _, c := range models {
				if c.Host == bound {
					// 机器健康才复用，否则放行重新路由
					if snap := g.snapshotFor(bound); snap != nil && snap.Healthy {
						// 能力硬门槛（待修补 #11）：粘性不能绕过按目标引擎的必需能力判定——
						// 绑定机器的引擎没被证过支持就重新路由（绝不静默降级）。
						if gerr := g.gateRoute(model, bound, required); gerr != nil {
							log.Printf("🧱 session affinity blocked by capability gate: %v — re-routing", gerr)
							break affinity
						}
						log.Printf("🎯 session affinity: session=%s bound to %s, routing directly", sessionID, bound)
						ip, port := getNode(bound)
						return &RouteResult{
							Host:  bound,
							Port:  port,
							URL:   fmt.Sprintf("http://%s:%d/infer", ip, port),
							File:  c.File,
							MemGB: int(c.MemGb),
						}, nil
					}
					log.Printf("🎯 session affinity: session=%s bound to %s but unavailable, re-routing", sessionID, bound)
				}
			}
		}
	}

	// 如果有多个候选 host，选择最优的（B13: 负载均衡轮询——本机/X3 压力均分）
	if len(models) > 1 {
		// B13 策略：所有候选（含 local）参与打分 + 轮询权重（压力均分）
		//   1. 轮询：roundRobin[model] 计数器 → 上次选的机器下次减权（轮流）
		//   2. 健康/已加载+10/空闲+1/负载低+1 打分
		//   3. 熔断跳过保留
		// 会话粘性优先（前面已处理：有绑定走绑定机器）
		var best *config.ModelCandidate
		var bestScore int = -1
		var bestMemGB int
		// blocked 记录被能力硬门槛拦下的候选（host(engine):reason），用于全被拦下时给出明确原因。
		var blocked []string
		var firstBlocked *modelreg.CapabilityDecision

		// 轮询计数器：偶数 → local 先，奇数 → 远程先（交替）
		g.roundRobinMu.Lock()
		rr := g.roundRobin[model]
		g.roundRobinMu.Unlock()

		for i, candidate := range models {
			// 熔断（T7）：连续失败 >= 阈值 的机器跳过（降权/摘除）
			if g.isTripped(candidate.Host) {
				log.Printf("⛔ machine %s is circuit-broken, skipping candidate", candidate.Host)
				continue
			}
			// B4 v2：排除本机模式——local 候选直接跳过（用户工作时任务全走远程）
			if candidate.Host == "local" && g.ExcludeLocal() {
				continue
			}
			// 能力硬门槛（待修补 #11）：按**目标引擎**判必需能力——目标引擎没被证过支持
			// 就跳过这个候选（继续找被证过的引擎），绝不把请求发到未验证引擎上（绝不静默降级）。
			if len(required) > 0 {
				if d := g.gateRequiredCapabilities(model, candidate.Backend, required); !d.Allowed {
					blocked = append(blocked, fmt.Sprintf("%s(engine=%s):%s", candidate.Host, d.Engine, d.Reason))
					if firstBlocked == nil {
						dd := d
						firstBlocked = &dd
					}
					continue
				}
			}
			score := 0
			// B13 轮询倾向：同分时轻微倾向另一台（压力均分——不强制，只打破平局）
			// 真实打分（健康/已加载/空闲/负载）优先，轮询只在小分时起作用
			if candidate.Host == "local" {
				if rr%2 == 0 {
					score += 2 // 偶数轮：local 微倾向
				}
			} else {
				if rr%2 == 1 {
					score += 2 // 奇数轮：远程微倾向
				}
			}
			if snap := g.snapshotFor(candidate.Host); snap != nil {
				// v2.5.5 重启窗口期治本: 宽限期内 unhealthy 快照不算降权（机器还没机会心跳）
				// 主控重启瞬间 X3 心跳失败（API 未就绪窗口）→ 快照 unhealthy → 之前路由拒绝
				isHealthy := snap.Healthy
				if !isHealthy && time.Since(g.startupTime) < startupGrace {
					isHealthy = true // 宽限期内视为健康（等心跳恢复）
				}
				if isHealthy {
					score++
				}
				// 已加载目标模型 → 高分（+8——从+10降——让轮询/负载能打破"永远选X3"）
				// v2.5.6 修复（Mr2109 2026-08-28）: 精确相等改为文件名匹配——
				// local 快照 Model="Qwen3.8-27B-Q4_K_M-vcruz305"（文件名形式）≠ 请求逻辑名 "Qwen3.8-27B"
				// → local 永远拿不到 +8 → x3 恒赢 → 任务永远走 x3 → x3 故障反复挂起死循环
				if modelFileLoaded(snap, candidate.File) {
					score += 8
				}
				// v2.5.4.9 分配改进A：active 降权（最少连接思想——按机器并行度）
				//   并行度: X3/本机都单槽执行（GPU 全负荷——active>=1 即忙——降权转走）
				//   单槽机器（本机）active>=1 忙——扣分转走
				parallel := g.parallelSlots(candidate.Host)
				if snap.ActiveRequests == 0 {
					score++ // 空闲 +1
				} else if snap.ActiveRequests >= parallel {
					score -= 8 // 满负载——明显降权（倾向另一台）
				} else if parallel <= 1 {
					score -= 5 // 单槽机器忙——降权转走
				}
				// 负载低（Load < 0.5）→ +1
				if snap.Load < 0.5 {
					score++
				}
				// §3.5（批 5）：资源账本输入——可用性/成本延迟。账本缺失或未知时增量为 0，
				// 完全退回上面的既有打分（红线①：绝不因"不知道"降权/排掉候选）。
				if delta, why := g.ledgerAdjust(snap, candidate); delta != 0 {
					score += delta
					logLedgerAdjust(candidate.Host, model, delta, why)
				}
			} else if candidate.Host == "local" && g.localBack != nil {
				// B13: local 无快照（本机不心跳）——用 LocalBackend 状态打分
				if g.localBack.IsReady() {
					score++ // 健康
				}
				if g.localBack.ModelFile() == candidate.File {
					score += 10 // 已加载目标模型
				}
			}
			// cache-aware（T5）：该机器处理过相似 prompt → 加分
			if prompt != "" {
				if ps := g.prefixScore(candidate.Host, prompt); ps > 0 {
					score += ps
				}
			}
			// 同分取列表靠前者（fleet.yaml 顺序即配置优先级）
			if score > bestScore {
				bestScore = score
				best = &candidate
				bestMemGB = int(candidate.MemGb)
			}
			_ = i
		}

		// v2.5.5 #9 修复：所有候选都被熔断/排除时——返回明确错误（不兜底空 URL）
		// 原逻辑: best==nil 时 fallback models[0]（可能 URL:"" 空——转发 Post "" 失败）
		if best == nil {
			// 能力硬门槛（待修补 #11）：候选全被"目标引擎未证过支持该必需能力"拦下时，
			// fail-closed 返回明确原因（哪个引擎缺哪条能力证据）——绝不静默降级、
			// 也绝不落进下面的 allLocal 兜底（那会绕过门槛）。
			if len(blocked) > 0 {
				ge := &CapabilityGateError{
					Model:      model,
					Capability: strings.Join(required, ","),
					Reason:     modelreg.CapReasonNoEvidence,
					Trace: fmt.Sprintf("capability gate: no candidate engine proven for required capabilities %v — blocked: %s",
						required, strings.Join(blocked, "; ")),
				}
				// 用**第一条真实判定**的原因/引擎/留痕（不同候选可能因不同原因被拦：
				// 旧快照缺引擎维度 / 引擎未证过 / 快照缺失）——别用一个写死的汇总原因盖掉它。
				if firstBlocked != nil {
					ge.Engine = firstBlocked.Engine
					ge.Capability = firstBlocked.Name
					ge.Reason = firstBlocked.Reason
					ge.Trace = firstBlocked.Trace + " — blocked candidates: " + strings.Join(blocked, "; ")
				}
				return nil, ge
			}
			// 真正"所有候选都是 local 且未排除"才本机兜底
			allLocal := true
			for _, c := range models {
				if c.Host != "local" {
					allLocal = false
					break
				}
			}
			if allLocal && !g.ExcludeLocal() {
				c := models[0]
				return &RouteResult{
					Host:  c.Host,
					Port:  0, // 本机端口由 LocalBackend 动态分配
					URL:   "",
					File:  c.File,
					MemGB: int(c.MemGb),
				}, nil
			}
			// 无可用候选（全熔断/全排除）——明确错误（不转发空 URL）
			hosts := make([]string, 0, len(models))
			for _, c := range models {
				hosts = append(hosts, c.Host)
			}
			return nil, fmt.Errorf("model %s has no available candidate (all broken or excluded: %s)", model, strings.Join(hosts, ","))
		}

		// 有可用候选 → 返回 + 轮询计数器 +1
		g.roundRobinMu.Lock()
		g.roundRobin[model]++
		g.roundRobinMu.Unlock()
		// 2026-09-09 通用让位: X3 内存不足时先清场(防多实例堆积挂死)
		if best.Host == "x3" {
			g.ensureX3RoomForFile(best.File, bestMemGB)
		}
		ip, port := getNode(best.Host)
		return &RouteResult{
			Host:  best.Host,
			Port:  port,
			URL:   fmt.Sprintf("http://%s:%d/infer", ip, port),
			File:  best.File,
			MemGB: bestMemGB,
		}, nil
	}

	// 单个候选，直接返回（v2.5.5 #9：也要检查熔断/排除——单候选也可能全不可用）
	c := models[0]
	if g.isTripped(c.Host) {
		return nil, fmt.Errorf("model %s only candidate %s is circuit-broken — no candidate available", model, c.Host)
	}
	if c.Host == "local" && g.ExcludeLocal() {
		return nil, fmt.Errorf("model %s only candidate local is excluded — no candidate available", model)
	}
	// 能力硬门槛（待修补 #11）：单候选同样按**目标引擎**判定——目标引擎没被证过支持必需
	// 能力就 fail-closed（明确原因），绝不静默降级。
	if gerr := g.gateRoute(model, c.Host, required); gerr != nil {
		return nil, gerr
	}
	if c.Host == "local" {
		return &RouteResult{
			Host:  c.Host,
			Port:  0,
			URL:   "",
			File:  c.File,
			MemGB: int(c.MemGb),
		}, nil
	}
	ip, port := getNode(c.Host)
	return &RouteResult{
		Host:  c.Host,
		Port:  port,
		URL:   fmt.Sprintf("http://%s:%d/infer", ip, port),
		File:  c.File,
		MemGB: int(c.MemGb),
	}, nil
}

// snapshotFor 安全获取机器快照（无 store 或机器未知时返回 nil）。
func (g *Gateway) snapshotFor(machine string) *store.FleetSnapshot {
	if g.store == nil {
		return nil
	}
	return g.store.GetSnapshot(machine)
}

// defaultAgentPort 是子端 agent 的默认 HTTP 端口（fleet.yaml 未给端口时的回退，与既有口径一致）。
const defaultAgentPort = 8100

// fleetNode 解析一个 fleet 节点：机器名 → (IP, 端口)。
// 地址一律来自 fleet.yaml 的 fleet 段（配置即事实源）；缺配置/端口时回退机器名 + 默认端口
// （与既有 pickRoute 内的同口径逻辑一致——批 3 把它提成方法，供让位路径复用，避免硬编码 IP）。
func (g *Gateway) fleetNode(host string) (string, int) {
	if g.config != nil {
		if node, ok := g.config.Fleet[host]; ok && node.Host != "" {
			port := node.Port
			if port <= 0 {
				port = defaultAgentPort
			}
			return node.Host, port
		}
	}
	return host, defaultAgentPort
}

// agentURLFor 拼子端 agent 的 HTTP 地址（如 /unload）——地址只从配置来，绝不写死 IP。
func (g *Gateway) agentURLFor(host, path string) string {
	ip, port := g.fleetNode(host)
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d%s", ip, port, path)
}

// modelFileLoaded 判断机器快照是否已加载候选模型文件（v2.5.6 修复——Mr2109 2026-08-28）
// 匹配规则（宽松匹配——兼容文件名形式与逻辑名）:
//   - 快照 Model 与候选 File 的 basename 匹配（去路径去 .gguf 后缀）
//     local 快照 Model="Qwen3.8-27B-Q4_K_M-vcruz305"（文件名形式）
//   - 快照 Model 是逻辑名（如 "Qwen3.8-27B"）——与候选 File basename 去量化后缀后匹配
//     （"Qwen3.8-27B-Q4_K_M-vcruz305" 去 "-Q4_K_M-vcruz305" → "Qwen3.8-27B"）
//   - 快照 Models 列表任一匹配（同规则）
//
// 注意: 不能用 filepath.Ext 剥扩展——模型名含点号（Qwen3.8 的 .8 会被误当扩展名）
func modelFileLoaded(snap *store.FleetSnapshot, file string) bool {
	if snap == nil || file == "" {
		return false
	}
	// 目标文件 basename（去路径 + 去 .gguf）——如 ".../Qwen3.8-27B-Q4_K_M-vcruz305.gguf" → "Qwen3.8-27B-Q4_K_M-vcruz305"
	target := filepath.Base(file)
	target = strings.TrimSuffix(target, ".gguf")
	// 目标去量化后缀（"Qwen3.8-27B-Q4_K_M-vcruz305" → "Qwen3.8-27B"）——逻辑名匹配用
	targetStem := stripQuantSuffix(target)
	// 当前模型（快照 Model——可能文件名形式或逻辑名）
	if snap.Model != nil {
		cur := *snap.Model
		cur = strings.TrimSuffix(cur, ".gguf")
		// 可能带路径（子端上报完整路径）
		cur = filepath.Base(cur)
		if cur == target {
			return true
		}
		if targetStem != "" && (cur == targetStem || *snap.Model == targetStem) {
			return true
		}
	}
	// Models 列表（已加载模型集合）
	for _, m := range snap.Models {
		cur := m
		cur = strings.TrimSuffix(cur, ".gguf")
		cur = filepath.Base(cur)
		if cur == target {
			return true
		}
		if targetStem != "" && (cur == targetStem || m == targetStem) {
			return true
		}
	}
	return false
}

// stripQuantSuffix 去掉量化/版本后缀（Q4_K_M/vcruz305 等）——逻辑名匹配用
// "Qwen3.8-27B-Q4_K_M-vcruz305" → "Qwen3.8-27B"；无后缀原样返回
func stripQuantSuffix(name string) string {
	// 常见量化/版本标记（出现在模型逻辑名之后）
	markers := []string{"-Q", "-q", "_Q", "_q", "-fp", "-Fp", "-bf16", "-v"}
	best := name
	for _, m := range markers {
		if idx := strings.Index(name, m); idx > 0 {
			cand := name[:idx]
			if len(cand) > 3 && len(cand) < len(best) {
				best = cand
			}
		}
	}
	return best
}

// classifyRouteError 路由错误分类（v2.5.6 故障自愈——Mr2109 2026-08-28）
// 返回 (错误码, HTTP 状态码)——调度器按 code 分类处理（可等/可重试/不可重试）
//
//	circuit_open    (503) 熔断/无候选/被排除——环境故障——可等（waiting_retry）
//	model_not_found (404) 模型不在路由表——配置问题——不可重试
func classifyRouteError(err error) (string, int) {
	if err == nil {
		return "api_error", http.StatusBadGateway
	}
	// 能力硬门槛 fail-closed（待修补 #11）：目标引擎没被证过支持该必需能力 →
	// 明确 400 + capability_unavailable（不可重试，用户需换模型/引擎或先补探测）——
	// 绝不静默降级成别的机器/降级模型。
	var gateErr *CapabilityGateError
	if errors.As(err, &gateErr) {
		return "capability_unavailable", http.StatusBadRequest
	}
	msg := err.Error()
	// 熔断/无候选/被排除 → 环境故障（可等——机器恢复后自动重派）
	if strings.Contains(msg, "熔断") || strings.Contains(msg, "无可用候选") ||
		strings.Contains(msg, "已被排除") {
		return "circuit_open", http.StatusServiceUnavailable
	}
	// 模型不在路由表 → 配置问题（不可重试——纠正模型名/配置）
	if strings.Contains(msg, "未在路由表中找到") || strings.Contains(msg, "未知模型") {
		return "model_not_found", http.StatusNotFound
	}
	// 默认——转发失败（可重试）
	return "upstream_fail", http.StatusBadGateway
}

// Snapshot 导出机器快照（v2.5.5 T3: 内部任务引擎空闲检测用）。
func (g *Gateway) Snapshot(machine string) *store.FleetSnapshot {
	return g.snapshotFor(machine)
}

// pickFallbackRoute — v2.5.4.9 C failover：换机器（跳过失败机器——选其他候选）
// 转发失败（超时/卡死）→ 熔断失败机器 + 重选候选（强制排除失败机器）
// reason 必填：本次失败的原文（调用方持有 err），会写进熔断快照的 last_error——
// 这条路径（tripMachine + pickRouteExcluding 强制熔断）历史上不写原因，是「计数涨了原因空」的元凶之一。
//
// required（必填，待修补 #38 ②）是本次请求的必需能力（必要性升档，硬门槛）。
// 换机是**重新选路**：新目标引擎必须重新过同一道按引擎的能力门槛——首跳过了不代表换机后也过。
// 该参数不是可选（不是 ...string）：它曾被漏传，导致换机目标引擎未经能力验证就被放行（静默降级）。
// 改成必填让「漏传」在编译期不可能；确实没有必需能力的路径显式传 nil。
func (g *Gateway) pickFallbackRoute(failed *RouteResult, model string, reason string, required []string) (*RouteResult, error) {
	if model == "" {
		return nil, fmt.Errorf("model is empty — cannot fail over")
	}
	// 熔断失败机器（连续失败计数——isTripped 后续跳过）
	g.tripMachine(failed.Host, reason)
	// 重选候选（pickRoute——但强制排除失败机器）；required 透传，保证每次换机都重新过门槛。
	route, err := g.pickRouteExcluding(model, failed.Host, reason, required...)
	if err != nil {
		return nil, fmt.Errorf("failover has no available candidate: %w", err)
	}
	return route, nil
}

// pickRouteExcluding — 选路但排除指定机器（failover 用——不选回失败机器）
// reason 必填：本函数在「选回被排除机器」时会强制把 failCounts 顶到 healthyTripLimit+1
// （11）——这是一次计数写入，必须留下原因与路径，否则快照会出现「fail_count=11 但 last_error 空」。
// required（可选，待修补 #11）是必需能力——透传给内部两次 pickRoute，硬门槛不得因"排除重选"被绕过。
func (g *Gateway) pickRouteExcluding(model, exclude string, reason string, required ...string) (*RouteResult, error) {
	// v2.5.5 #9 修复: 直接调用 pickRoute 的内部逻辑但跳过 exclude 机器（不走熔断 hack——防 healthy 保护冲突）
	// 先试正常 pickRoute——若选回 exclude——用"临时排除"重选（设置 failCounts 到超高——强制跳过）
	route, err := g.pickRoute(model, "", "", required...)
	if err != nil {
		return nil, err
	}
	if route.Host != exclude {
		return route, nil
	}
	// 选回失败机器——临时熔断它再重选（设到 healthyTripLimit+1——保证即使 healthy 保护也熔断）
	g.failMu.Lock()
	if g.failCounts == nil {
		g.failCounts = map[string]int{}
	}
	if g.failSince == nil {
		g.failSince = map[string]time.Time{}
	}
	g.recordFailureReasonLocked(exclude, "pickRouteExcluding", reason, 2)
	g.failCounts[exclude] = healthyTripLimit + 1 // 超高——强制跳过（v2.5.5 #9: 用 healthyTripLimit+1）
	g.failSince[exclude] = time.Now()
	g.failMu.Unlock()
	route2, err2 := g.pickRoute(model, "", "", required...)
	if err2 != nil {
		return nil, err2
	}
	if route2.Host == exclude {
		return nil, fmt.Errorf("no other candidate — only %s", exclude)
	}
	return route2, nil
}

// tripMachine — 熔断一台机器（加 failCounts——isTripped 连续失败 >= circuitFailThreshold 熔断）
// reason 必填：本函数同时涨 tripCounts 与 failCounts（两条计数），历史上不写 lastErr——
// 于是计数涨了而快照原因空。现在与 markFailure 一样必须给出原因（空串会被自动补明确文本）。
// failSince 的盖章门限必须引用 circuitFailThreshold（历史缺陷 #32：此处曾写死 3，而真实阈值是 8，
// 使 30s 冷却计时从第 3 次失败就起跑——第 3→第 8 次失败跨越冷却期时，达阈值那一刻即被判「冷却已过」
// 而清零放行，熔断从未生效）。
func (g *Gateway) tripMachine(host string, reason string) {
	g.tripMu.Lock()
	if g.tripCounts == nil {
		g.tripCounts = map[string]int{}
	}
	g.tripCounts[host]++
	cnt := g.tripCounts[host]
	g.tripMu.Unlock()
	// 同步到 failCounts（isTripped 用——连续失败 >= circuitFailThreshold 熔断）
	g.failMu.Lock()
	if g.failCounts == nil {
		g.failCounts = map[string]int{}
	}
	if g.failSince == nil {
		g.failSince = map[string]time.Time{}
	}
	g.failCounts[host]++
	g.recordFailureReasonLocked(host, "tripMachine", reason, 2)
	// 阈值之下不盖章（计时起点=首次达阈值的时刻）；用 >= 保留「达阈值后再失败即刷新计时起点」的既有语义
	if g.failCounts[host] >= circuitFailThreshold {
		g.failSince[host] = time.Now()
	}
	fcnt := g.failCounts[host]
	g.failMu.Unlock()
	log.Printf("⛔ machine %s failure counts trip=%d fail=%d", host, cnt, fcnt)
}

// modelName — 从请求体提取模型名（failover 用）
func modelName(reqMap map[string]interface{}) string {
	if m, ok := reqMap["model"].(string); ok {
		return m
	}
	return ""
}

// setRespBody 替换响应体并**同步长度**。
//
// 2026-09-14 修复：此前只换 Body 不换 ContentLength/Content-Length 头，
// 于是转发层按旧长度抄写正文 ⇒ `io.Copy` 报 "wrote more than the declared
// Content-Length" ⇒ 客户端收到 200 但**空正文**（GLM 5.3 思考模型实测复现）。
func setRespBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	if resp.Header != nil {
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}
}

// applyReasoningFallback — v2.5.4.9 reasoning 兜底（思考模型 content 空时拼 reasoning）
// 场景: Nemotron/Qwen3.8 思考模式——生成全在 reasoning_content——content 空
// 处理: 读 body——chat.completions 响应里 message.content 空但有 reasoning_content
//
//	→ content 用 reasoning_content 填充（调用方不误判"无输出"）
func (g *Gateway) applyReasoningFallback(resp *http.Response) *http.Response {
	if resp == nil || resp.Body == nil {
		return resp
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp
	}
	defer resp.Body.Close()

	// 解析响应——chat.completions 格式
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		// 非 JSON——原样返回
		setRespBody(resp, body)
		return resp
	}
	choices, ok := obj["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		setRespBody(resp, body)
		return resp
	}
	changed := false
	for _, c := range choices {
		choice, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		msg, ok := choice["message"].(map[string]interface{})
		if !ok {
			continue
		}
		content, _ := msg["content"].(string)
		reasoning, _ := msg["reasoning_content"].(string)
		if strings.TrimSpace(content) == "" && strings.TrimSpace(reasoning) != "" {
			// content 空 + reasoning 有——用 reasoning 兜底
			msg["content"] = reasoning
			changed = true
		}
	}
	if !changed {
		setRespBody(resp, body)
		return resp
	}
	// 重新序列化
	newBody, err := json.Marshal(obj)
	if err != nil {
		setRespBody(resp, body)
		return resp
	}
	log.Printf("🔄 reasoning fallback: content empty → using reasoning_content (thinking model)")
	setRespBody(resp, newBody)
	return resp
}

// parallelSlots — 机器执行并行度（分配改进A——active 降权按执行能力）
//
//	Mr2109认知纠正（2026-08-15）：X3 -np 4 ≠ 能并行 4 任务
//	  = 显存大能同时加载 4 个模型（切换快——选择多）
//	  但执行 1 个模型 = GPU 全负荷（单任务推理）
//	→ 执行层面并行度都是 1——active>=1 即忙（GPU 满）——应降权转走
//	所有机器统一 1（保守——不低估忙）
func (g *Gateway) parallelSlots(machine string) int {
	return 1 // 执行层面单任务（GPU 全负荷——active>=1 即忙）
}

// extractSessionID 从请求体提取会话 ID（OpenAI 标准: body.session_id；Responses 标准: body.llm_request_id / body.session）。
// 没有则返回 ""（不启用会话粘性）。
func extractSessionID(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	if v, ok := obj["session_id"].(string); ok && v != "" {
		return v
	}
	if v, ok := obj["session"].(string); ok && v != "" {
		return v
	}
	if v, ok := obj["llm_request_id"].(string); ok && v != "" {
		return v
	}
	return ""
}

// boundSession 查询会话绑定的机器（未过期且模型匹配时返回，否则 ""）。
func (g *Gateway) boundSession(sessionID, model string) string {
	g.sessionMu.RLock()
	defer g.sessionMu.RUnlock()
	b, ok := g.sessions[sessionID]
	if !ok {
		return ""
	}
	if time.Since(b.lastUsed) > sessionTTL {
		delete(g.sessions, sessionID)
		return ""
	}
	if b.model != model {
		return "" // 换了模型，重新路由
	}
	b.lastUsed = time.Now()
	g.sessions[sessionID] = b
	return b.host
}

// pickRouteLocal 强制选择 local（本机）候选——用于压缩模型避开 X3 截断 bug。
// 若无 local 候选或本机后端不可用，返回错误（调用方 fallback 到 pickRoute）。
func (g *Gateway) pickRouteLocal(model string) (*RouteResult, error) {
	candidates, ok := g.config.Models[model]
	if !ok || len(candidates) == 0 {
		return nil, fmt.Errorf("model %s is not configured", model)
	}
	for _, c := range candidates {
		if c.Host == "local" {
			return &RouteResult{Host: "local", URL: "", File: c.File, MemGB: int(c.MemGb)}, nil
		}
	}
	return nil, fmt.Errorf("model %s has no local candidate", model)
}

// bindSession 记录会话 → 机器绑定。
func (g *Gateway) bindSession(sessionID, host, model string) {
	g.sessionMu.Lock()
	defer g.sessionMu.Unlock()
	g.sessions[sessionID] = sessionBinding{
		host:     host,
		model:    model,
		lastUsed: time.Now(),
	}
	// 简单清理：超过 64 个绑定且数量过多时删过期
	if len(g.sessions) > 64 {
		for k, v := range g.sessions {
			if time.Since(v.lastUsed) > sessionTTL {
				delete(g.sessions, k)
			}
		}
	}
}

// extractUsageTokensFromBytes 从响应体提取本次请求消耗的 token（usage.total_tokens）。
func extractUsageTokensFromBytes(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return 0
	}
	if usage, ok := obj["usage"].(map[string]interface{}); ok {
		if total, ok := usage["total_tokens"].(float64); ok {
			return int(total)
		}
	}
	return 0
}

// addSessionTokens 累加会话 token 用量（V22 上下文预算）。
func (g *Gateway) addSessionTokens(sessionID string, n int) {
	g.sessionMu.Lock()
	defer g.sessionMu.Unlock()
	b, ok := g.sessions[sessionID]
	if !ok {
		return
	}
	b.tokens += n
	g.sessions[sessionID] = b
}

// sessionTokens 返回会话累计 token。
func (g *Gateway) sessionTokens(sessionID string) int {
	g.sessionMu.RLock()
	defer g.sessionMu.RUnlock()
	if b, ok := g.sessions[sessionID]; ok {
		return b.tokens
	}
	return 0
}

// resetSessionTokens 重置会话 token 计数（compaction 后上下文变小）。
func (g *Gateway) resetSessionTokens(sessionID string) {
	g.sessionMu.Lock()
	defer g.sessionMu.Unlock()
	if b, ok := g.sessions[sessionID]; ok {
		b.tokens = 0
		g.sessions[sessionID] = b
	}
}

// compactThreshold 返回网关侧**安全网**压缩阈值（模型窗口 × 85%——单槽执行）。
// 丙批 §4.2（2026-09-10）：网关不再在 50% 独立触发（那会与对话侧主压缩抢跑）——
// 主压缩在对话侧（chat.MaybeCompact，阈值 max(min(ctx×0.5,8000),2000)）；
// 网关侧只在 **85%** 兜底（仅当 history ≥ 4 条时生效，见调用点）。
func (g *Gateway) compactThreshold(model string) int {
	if candidates, ok := g.config.Models[model]; ok && len(candidates) > 0 {
		ctx := candidates[0].CtxWindow
		if ctx > 0 {
			perSlot := ctx            // 单槽执行（v2.5.5: X3/本机都单槽——取消 4 槽假设）
			return perSlot * 85 / 100 // 85% 安全网兜底
		}
	}
	return maxSessionTokens * 85 / 100
}

// requestMessageCount — 请求体里的历史条数（网关安全网的 len(history)≥4 门槛用）
func requestMessageCount(body []byte) int {
	var req struct {
		Messages []interface{} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return 0
	}
	return len(req.Messages)
}

// autoCompact 自动压缩会话历史（超阈值时调用）。
// 优先 LLMLingua-2（Go 进程内 ONNX，删除式保真，0.1s）；失败/未加载 → gemma 摘要兜底。
// 引擎无关：不依赖 llama 特性。
func (g *Gateway) autoCompact(sessionID, model string, body []byte) {
	// 从请求 body 提取 messages（当前请求的上下文）
	var req struct {
		Messages []interface{} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Messages) == 0 {
		return
	}

	// 优先：LLMLingua-2 压缩（Go 进程内，删除式，保真）
	msgsJSON, _ := json.Marshal(req.Messages)
	if compressed, err := g.compressWithLLMLingua2(string(msgsJSON)); err == nil {
		log.Printf("🧠 session %s LLMLingua-2 compaction done (%d→%d chars), tokens reset", sessionID, len(msgsJSON), len(compressed))
		g.resetSessionTokens(sessionID)
		return
	}

	// 兜底：gemma 摘要（重写式，结构化 prompt 保针）
	compactModel := "gemma-4-12B"
	if _, ok := g.config.Models[compactModel]; !ok {
		compactModel = model
	}

	compactPrompt := BuildStructuredSummaryPrompt(string(msgsJSON))

	compactBody := map[string]interface{}{
		"model":      compactModel,
		"messages":   []interface{}{map[string]interface{}{"role": "user", "content": compactPrompt}},
		"max_tokens": 2000,
	}
	compactBodyJSON, _ := json.Marshal(compactBody)

	route, err := g.pickRoute(compactModel, "", "")
	if err != nil {
		log.Printf("⚠️ autoCompact routing failed: %v", err)
		return
	}

	var resp *http.Response
	if route.Host == "local" {
		if g.localBack == nil {
			return
		}
		resp, err = g.localBack.Infer("/v1/chat/completions", compactBodyJSON)
	} else {
		resp, err = g.forwardToBackend(context.Background(), route, "/v1/chat/completions", compactBodyJSON, nil, nil)
	}
	if err != nil {
		log.Printf("⚠️ autoCompact compaction failed: %v", err)
		return
	}
	defer resp.Body.Close()

	// 压缩成功 → token 重置（摘要替代历史，上下文变小）
	respBody, _ := io.ReadAll(resp.Body)
	var obj map[string]interface{}
	if err := json.Unmarshal(respBody, &obj); err == nil {
		if choices, ok := obj["choices"].([]interface{}); ok && len(choices) > 0 {
			if msg, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{}); ok {
				summary, _ := msg["content"].(string)
				if summary == "" {
					summary, _ = msg["reasoning_content"].(string)
				}
				if summary != "" {
					summary = ParseStructuredSummary(summary)
					log.Printf("♻️ session %s compaction done (summary %d chars), tokens reset", sessionID, len(summary))
					g.resetSessionTokens(sessionID)
					return
				}
			}
		}
	}
	log.Printf("⚠️ autoCompact produced no result")
}

// extractPrompt 从请求体提取 prompt 文本（拼接 messages 内容，用于前缀匹配）。
func extractPrompt(body []byte) string {
	var obj struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
		Input []struct {
			Content string `json:"content"`
		} `json:"input"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, m := range obj.Messages {
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	for _, m := range obj.Input {
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// prefixScore 返回该机器对给定 prompt 的缓存匹配加分（0-8）。
// 多级前缀匹配（V22-3，近似 radix trie）：前缀越长匹配 → 分越高。
// 引擎无关：不依赖任何引擎的 KV 实现，纯字符前缀记录。
func (g *Gateway) prefixScore(host, prompt string) int {
	if prompt == "" {
		return 0
	}
	// 多级签名：400/200/100/50 字符前缀（覆盖长/中/短 prompt 匹配）
	levels := []struct {
		len   int
		score int
	}{
		{400, 8}, {200, 6}, {100, 4}, {50, 2},
	}
	sigs := make([]string, 0, len(levels))
	for _, lv := range levels {
		s := prompt
		if len(s) > lv.len {
			s = s[:lv.len]
		}
		sigs = append(sigs, s)
	}

	g.prefixMu.RLock()
	defer g.prefixMu.RUnlock()

	hostMap, ok := g.prefixes[host]
	if !ok || len(hostMap) == 0 {
		return 0
	}
	// 从最长前缀开始匹配，命中即返回对应分
	for i, sig := range sigs {
		if _, ok := hostMap[sig]; ok {
			return levels[i].score
		}
	}
	return 0
}

// recordPrefix 记录该机器处理过的 prompt 前缀签名（V22-3 多级前缀缓存）。
func (g *Gateway) recordPrefix(host, prompt string) {
	if prompt == "" || host == "" || host == "local" {
		return
	}
	// 多级签名（400/200/100/50）
	lengths := []int{400, 200, 100, 50}
	sigs := make([]string, 0, len(lengths))
	for _, ln := range lengths {
		s := prompt
		if len(s) > ln {
			s = s[:ln]
		}
		sigs = append(sigs, s)
	}

	g.prefixMu.Lock()
	defer g.prefixMu.Unlock()
	if g.prefixes[host] == nil {
		g.prefixes[host] = make(map[string]int)
	}
	for _, sig := range sigs {
		g.prefixes[host][sig]++
	}
	// 简单限制每机器记录条数，防内存膨胀
	if len(g.prefixes[host]) > 500 {
		// 清空重建（最简策略：丢弃旧签名）
		g.prefixes[host] = make(map[string]int)
	}
}

// loadModel 加载本地模型到 LocalBackend（按需加载）。
// 如果本地后端已就绪且加载了相同模型，则跳过加载。
func (g *Gateway) loadModel(model string, route *RouteResult) error {
	if route.File == "" {
		return fmt.Errorf("local model is missing its file path")
	}

	// 检查是否已加载相同模型
	if g.localBack != nil && g.localBack.IsReady() && g.localBack.ModelFile() == route.File {
		log.Printf("[gateway] local model ready: %s (%s)", model, route.File)
		return nil
	}

	// 加载模型到 LocalBackend
	log.Printf("[gateway] loading local model: %s → %s (%d GB)", model, route.File, route.MemGB)
	if err := g.localBack.LoadModel(route.File, route.MemGB); err != nil {
		return fmt.Errorf("LocalBackend.LoadModel: %w", err)
	}
	log.Printf("[gateway] local model loaded: %s (state=%s)", model, g.localBack.State())
	return nil
}

// forwardToLocal 通过 LocalBackend 转发请求到本机 llama-server。
// 健康检查由 LocalBackend.Infer() 内部处理（崩溃自愈 + 熔断）。
func (g *Gateway) forwardToLocal(w http.ResponseWriter, r *http.Request, route *RouteResult, body []byte, origBody []byte, adp adapter.Adapter) {
	// 调用 LocalBackend.Infer() 转发到本机 llama-server
	// path 透传给 llama-server（如 /v1/chat/completions）
	resp, err := g.localBack.Infer(r.URL.Path, body)
	if err != nil {
		log.Printf("failed to forward to local backend: %v", err)
		// 后端未就绪或熔断，返回 503
		adp.TransformError(w, http.StatusServiceUnavailable, "api_error", fmt.Sprintf("local backend unavailable: %s", err.Error()))
		return
	}
	defer resp.Body.Close()

	// 丙批 N4：前缀命中率闭环——本机 llama-server 也返回缓存计量（timings/usage）。
	// 仅非流式响应读取缓冲（流式直通，不破坏 SSE）。
	if g.prefixCache != nil && !adapter.IsStreamResponse(resp) {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		if model, merr := extractModel(body); merr == nil {
			g.recordPrefixCache(model, body, respBody)
		}
	}

	// 丙批 N4 补齐（2026-09-10）：本机 llama-server 走这条路径——流式同样采样（tee 尾窗 → 末块 timings）
	var usageTeeLocal *sseUsageTee
	if g.prefixCache != nil && adapter.IsStreamResponse(resp) {
		usageTeeLocal = newSSEUsageTee(resp.Body)
		resp.Body = usageTeeLocal
	}
	// 出站转换（按客户端适配器）——用原始 body 判断流式（forwardBody 已被强制 stream:false）
	adp.TransformResponse(w, resp, r, origBody)
	if g.prefixCache != nil && usageTeeLocal != nil {
		if ub := usageTeeLocal.UsageJSON(); len(ub) > 0 {
			if model, merr := extractModel(body); merr == nil {
				g.recordPrefixCache(model, body, ub)
			}
		}
	}
}

// handleLocalModel host=local 且无 LocalBackend 时的回退处理。
func handleLocalModel(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{"error":"local backend not configured"}`))
	log.Println("⚠️  host=local model request but LocalBackend is not configured")
}

// RouteResult 路由选择结果。
type RouteResult struct {
	Host  string // 目标机器名（x3, mini1 等）
	Port  int    // 目标端口（默认 8100）
	URL   string // 完整转发 URL（http://{host}:{port}/infer）
	File  string // 模型文件路径（仅 host=local 时有效）
	MemGB int    // 内存预算 GB（仅 host=local 时有效）
}

// Start 启动网关服务器。
func (g *Gateway) Start(port int) error {
	r := chi.NewRouter()
	g.RegisterRoutes(r)

	// 用 0.0.0.0 显式监听 IPv4（Go 的 ":port" 默认 IPv6-only，子端 IPv4 连不上）
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("🚀 gateway started, listening on %s", addr)
	return http.ListenAndServe(addr, r)
}

// extractLastUserPrompt 提取请求体的最后一条 user 消息内容（复合模型用）。
func extractLastUserPrompt(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Input []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"input"`
	}
	if json.Unmarshal(body, &req) != nil {
		return ""
	}
	msgs := req.Messages
	if len(msgs) == 0 {
		msgs = make([]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}, 0)
		for _, m := range req.Input {
			msgs = append(msgs, struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{m.Role, m.Content})
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

// ctxFromReq 从请求提取上下文（编排器用——独立于请求生命周期）。
func ctxFromReq(r *http.Request) context.Context {
	return context.Background()
}

// quoteJSON 转义字符串为 JSON 字符串字面量。
func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// AdapterOptions 模型适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
// 优先适配器实现的 Options() 接口——没有则反射读取导出字段
func (g *Gateway) AdapterOptions(model string) map[string]interface{} {
	adp, ok := g.adapterRegistry[model]
	if !ok || adp == nil {
		return nil
	}
	// 1. 显式 Options() 接口（config 小写字段的适配器——qwen38/gemma/nemotron/qwable）
	if o, ok := adp.(interface{ Options() map[string]interface{} }); ok {
		return o.Options()
	}
	// 2. 反射兜底（example-35b-v2/ds4——导出字段）
	opts := map[string]interface{}{}
	v := reflect.ValueOf(adp)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	reflectPluginOptions(v, "", opts)
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// reflectPluginOptions 反射遍历导出字段（嵌套 struct 递归——config 子字段展开）
func reflectPluginOptions(v reflect.Value, prefix string, out map[string]interface{}) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := v.Field(i)
		key := f.Name
		if prefix != "" {
			key = prefix + "." + f.Name
		}
		switch fv.Kind() {
		case reflect.String:
			if fv.String() != "" {
				out[key] = fv.String()
			}
		case reflect.Float64:
			if fv.Float() != 0 || f.Name == "Temperature" || f.Name == "TopP" || f.Name == "TopK" || f.Name == "MinP" {
				out[key] = fv.Float()
			}
		case reflect.Bool:
			out[key] = fv.Bool()
		case reflect.Int, reflect.Int32, reflect.Int64:
			if fv.Int() != 0 {
				out[key] = fv.Int()
			}
		case reflect.Slice:
			if fv.Len() > 0 {
				items := make([]string, 0, fv.Len())
				for j := 0; j < fv.Len(); j++ {
					items = append(items, fmt.Sprint(fv.Index(j).Interface()))
				}
				out[key] = strings.Join(items, ", ")
			}
		case reflect.Struct:
			reflectPluginOptions(fv, key, out)
		}
	}
}

// 适配器选项编辑（2026-08-27 Mr2109——每个模型各自独立参数集——实时生效）

// adapterOverridesFile 适配器配置覆盖持久化（重启恢复）
var adapterOverridesFile = filepath.Join(statepath.TaskRoot(), "adapter_overrides.json")

// adapterOverrides 模型名 → 配置覆盖（map[model]map[key]value）
var adapterOverrides = map[string]map[string]interface{}{}

// loadAdapterOverrides 启动恢复（NewGateway 调用——Init 后重放）
func (g *Gateway) loadAdapterOverrides() {
	data, err := os.ReadFile(adapterOverridesFile)
	if err != nil {
		return
	}
	var raw map[string]map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	adapterOverrides = raw
	// 重放（每个适配器 UpdateOptions——覆盖默认值）
	for model, cfg := range raw {
		if adp, ok := g.adapterRegistry[model]; ok {
			if o, ok := adp.(plugin.OptionedAdapter); ok {
				if err := o.UpdateOptions(cfg); err != nil {
					log.Printf("⚠️ adapter %s override replay failed: %v", model, err)
				} else {
					log.Printf("🧩 adapter %s overrides restored (%d entries)", model, len(cfg))
				}
			}
		}
	}
}

// saveAdapterOverrides 持久化
func saveAdapterOverrides() {
	data, _ := json.MarshalIndent(adapterOverrides, "", "  ")
	_ = os.MkdirAll(filepathDir(adapterOverridesFile), 0o755)
	_ = os.WriteFile(adapterOverridesFile, data, 0o644)
}

func filepathDir(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "."
}

// AdapterSchema 模型适配器参数 schema（含当前值——编辑控件渲染用）
// 无适配器 → nil（走旧路由）
func (g *Gateway) AdapterSchema(model string) []plugin.OptionDef {
	adp, ok := g.adapterRegistry[model]
	if !ok || adp == nil {
		return nil
	}
	o, ok := adp.(plugin.OptionedAdapter)
	if !ok {
		return nil
	}
	return o.OptionSchema()
}

// UpdateAdapterOptions 更新适配器配置（实时生效——校验 key → UpdateOptions → 持久化）
func (g *Gateway) UpdateAdapterOptions(model string, cfg map[string]interface{}) error {
	adp, ok := g.adapterRegistry[model]
	if !ok || adp == nil {
		return fmt.Errorf("model %s has no adapter (legacy routing — not editable)", model)
	}
	o, ok := adp.(plugin.OptionedAdapter)
	if !ok {
		return fmt.Errorf("model %s adapter does not support editing", model)
	}
	// 校验 key（只允许 schema 内的参数——各模型各自参数集）
	schema := o.OptionSchema()
	valid := map[string]bool{}
	for _, d := range schema {
		valid[d.Key] = true
	}
	for k := range cfg {
		if !valid[k] {
			return fmt.Errorf("parameter %s is not in this model's adapter parameter set (differs per model — editable: %v)", k, keysOf(schema))
		}
	}
	if err := o.UpdateOptions(cfg); err != nil {
		return fmt.Errorf("adapter update failed: %w", err)
	}
	// 持久化（合并覆盖）
	if adapterOverrides[model] == nil {
		adapterOverrides[model] = map[string]interface{}{}
	}
	for k, v := range cfg {
		adapterOverrides[model][k] = v
	}
	saveAdapterOverrides()
	return nil
}

// keysOf schema 参数名列表（错误信息用）
func keysOf(schema []plugin.OptionDef) []string {
	out := make([]string, 0, len(schema))
	for _, d := range schema {
		out = append(out, d.Key)
	}
	return out
}
