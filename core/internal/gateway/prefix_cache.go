// 丙批（2026-09-10）——前缀命中率闭环（设计 §9.3 / §11 拍板 N4）。
//
// 目标（Mr2109拍板原话）："网关记录 cache_read/cache_creation 比例 + 版本级告警"。
//
// 本文件只做四件事，全部自包含于 gateway 包：
//  1. 解析上游返回的缓存计量——兼容多形态（实测本仓 llama-server 同时返回
//     OpenAI 式 usage.prompt_tokens_details.cached_tokens 与
//     llama.cpp 式 timings.cache_n/prompt_n；DeepSeek / Anthropic 形态一并兼容）。
//  2. 按 (model, prompt_version) 维护滑动窗口命中率 = cache_read / (read + miss)。
//  3. prompt_version 变更 → 重置基线，并与上一版本命中率对比，显著下降则告警。
//  4. GET /api/metrics/prefix_cache 查询端点（见 gateway.go 的 RegisterRoutes）。
//
// 并发安全：所有可变状态由 tracker.mu 保护。
// 不改动 autoCompact 逻辑（token 预算路径保持原样）。
package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/infergeom"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ---------------------------------------------------------------------------
// 常量（窗口 / 告警阈值）
// ---------------------------------------------------------------------------

const (
	// defaultPrefixWindowN 滑动窗口最大样本数（最近 N 次请求）。
	defaultPrefixWindowN = 50
	// defaultPrefixWindowDur 滑动窗口最大时间跨度（10 分钟——超龄样本剔除）。
	defaultPrefixWindowDur = 10 * time.Minute
	// defaultPrefixMinAlertSamples 触发版本级告警所需的最小样本数（防单次噪声误报）。
	defaultPrefixMinAlertSamples = 3
	// prefixAlertAbsDrop 告警阈值①：命中率绝对下降 ≥ 30 个百分点。
	prefixAlertAbsDrop = 0.30
	// prefixAlertRelFactor 告警阈值②：新版本命中率低于上一版本的 60%（×0.6）。
	prefixAlertRelFactor = 0.60
	// maxPrefixAlerts 告警列表保留上限（防内存膨胀）。
	maxPrefixAlerts = 100
	// maxPrefixModels 追踪的模型数上限（超过清理最久未更新的）。
	maxPrefixModels = 64

	// 丙批 C2：持久化节流参数（最多每 5 秒或每 10 个样本落盘一次，避免每请求一写）。
	defaultFlushEvery   = 5 * time.Second
	defaultFlushSamples = 10
	// persistedPrefixCacheVersion 落盘格式版本（未来不兼容变更时递增）。
	persistedPrefixCacheVersion = 1
	// maxUnknownLastKeys 最近未知样本的键名列表上限（只留最近 1 条样本，键名本身限长防膨胀）。
	maxUnknownLastKeys = 24
	// prefixCacheStateFile 落盘文件名（位于 statepath.Dir()）。
	prefixCacheStateFile = "prefix_cache.json"
)

// ---------------------------------------------------------------------------
// 数据结构
// ---------------------------------------------------------------------------

// cacheSample 单次响应的缓存计量样本。
type cacheSample struct {
	t         time.Time
	cacheRead int // 命中（复用前缀）的 token 数
	cacheMiss int // 未命中（需重新处理）的 token 数
}

// PrefixCacheAlert 版本级命中率下降告警。
type PrefixCacheAlert struct {
	Model         string  `json:"model"`
	PromptVersion string  `json:"prompt_version"` // 新版本
	PrevVersion   string  `json:"prev_version"`   // 上一版本（基线来源）
	Ratio         float64 `json:"ratio"`          // 新版本当前命中率
	BaselineRatio float64 `json:"baseline_ratio"` // 上一版本命中率
	Delta         float64 `json:"delta"`          // ratio - baseline（负=下降）
	DropPct       float64 `json:"drop_pct"`       // 相对下降百分比（0-100）
	Reason        string  `json:"reason"`         // 触发原因（阈值描述）
	Message       string  `json:"message"`        // 中文可读消息
	At            string  `json:"at"`             // 触发时间 RFC3339
}

// modelPrefixState 单个模型的当前版本窗口状态。
type modelPrefixState struct {
	version       string        // 当前 prompt 版本
	samples       []cacheSample // 当前版本的滑动窗口样本（时间升序）
	baselineRatio float64       // 上一版本命中率（版本切换时固化）
	hasBaseline   bool          // 是否已有可用基线（首版本无）
	prevVersion   string        // 上一版本号（告警展示用）
	alerted       bool          // 当前版本是否已告警（防重复刷屏）
	updatedAt     time.Time     // 最近一次采样时间（清理用）
}

// prefixCacheTracker 前缀命中率追踪器（按 model 分桶）。
type prefixCacheTracker struct {
	mu      sync.Mutex
	windowN int
	windowD time.Duration
	// minAlertSamples 触发告警所需最小样本数（可注入——测试用）。
	minAlertSamples int
	// now 时钟注入（默认 time.Now——测试可替换）。
	now func() time.Time

	models map[string]*modelPrefixState
	alerts []PrefixCacheAlert // 最新告警在末尾

	// 丙批 C2：跨重启持久化（persistPath 为空则禁用——单测构造默认禁用）
	persistPath  string        // 落盘路径（statepath.File("prefix_cache.json")）
	flushEvery   time.Duration // 节流：最短写盘间隔（默认 5s）
	flushSamples int           // 节流：累计样本数阈值（默认 10）
	lastSaveAt   time.Time     // 上次成功落盘时间
	dirtySamples int           // 距上次落盘累计的变更数
	dirty        bool          // 是否有未落盘变更

	// 丙批 C2：未知响应形态探测
	unknownForms    int             // 解析失败（无 timings 且无已知 usage 字段）的响应计数
	unknownLastKeys []string        // 最近一条未知样本的键名列表（不含值——防泄漏）
	unknownShapes   map[string]bool // 形态签名集合（键名排序后用 | 连接；上限 maxUnknownShapes）
}

// newPrefixCache 构造追踪器。windowN<=0 用默认 50；dur<=0 用默认 10 分钟。
func newPrefixCache(windowN int, dur time.Duration) *prefixCacheTracker {
	if windowN <= 0 {
		windowN = defaultPrefixWindowN
	}
	if dur <= 0 {
		dur = defaultPrefixWindowDur
	}
	return &prefixCacheTracker{
		windowN:         windowN,
		windowD:         dur,
		minAlertSamples: defaultPrefixMinAlertSamples,
		now:             time.Now,
		models:          map[string]*modelPrefixState{},
	}
}

// newPrefixCachePersistent 构造带跨重启持久化的追踪器（生产路径——见 NewGateway）。
// 立即尝试 load 已有文件；文件缺失/损坏 → 从空态开始并记日志（不阻断启动）。
func newPrefixCachePersistent(windowN int, dur time.Duration) *prefixCacheTracker {
	t := newPrefixCache(windowN, dur)
	t.enablePersistence(statepath.File(prefixCacheStateFile))
	return t
}

// enablePersistence 启用落盘并立即 load（path 为空则保持禁用——单测隔离友好）。
func (t *prefixCacheTracker) enablePersistence(path string) {
	if t == nil || path == "" {
		return
	}
	t.mu.Lock()
	t.persistPath = path
	t.flushEvery = defaultFlushEvery
	t.flushSamples = defaultFlushSamples
	t.lastSaveAt = t.now() // 节流基准 = 启用时刻
	t.mu.Unlock()
	t.load()
	// C2 补齐（2026-09-10）：后台定时刷盘——原先只在 Record 里按节流写，
	// 若末批样本之后不再有新请求，这批样本会一直留在内存、进程被杀即丢（实测踩到）。
	go t.autoFlushLoop()
}

// autoFlushLoop — 每 10s 把脏样本落盘（低流量场景的保底；不影响 Record 内的节流写）
func (t *prefixCacheTracker) autoFlushLoop() {
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()
	for range tk.C {
		t.Flush()
	}
}

// ---------------------------------------------------------------------------
// 解析：兼容多形态缓存计量
// ---------------------------------------------------------------------------

// parsePrefixCacheUsage 从响应体 JSON 解析缓存计量，兼容多形态。
//
// 支持的形态（按优先级）：
//
//	① DeepSeek 式：usage.prompt_cache_hit_tokens / usage.prompt_cache_miss_tokens
//	② OpenAI  式：usage.prompt_tokens_details.cached_tokens（miss = prompt_tokens - cached）
//	③ Anthropic 式：usage.cache_read_input_tokens / usage.cache_creation_input_tokens
//	④ llama.cpp 式：timings.cache_n（命中）/ timings.prompt_n（本次实际处理的未命中 token）
//
// T1.4（2026-09-17）：解析核心**收敛到 internal/infergeom**（叶子包，零内部依赖）——
// 因为同一份响应字段现在有两个消费方（此处的命中率统计 + chat 观测面的批量几何）。
// 本函数只留**形态适配**：签名与语义一字不改（既有用例就是这次收敛的守卫）。
// 几何字段（cache_n/prompt_n/cached_tokens/ubatch_n/slot_id/system_fingerprint）请用
// infergeom.Parse 直接取——那里每个字段都带 Has* 存在标志，缺席与 0 可分。
//
// 返回：cacheRead / cacheMiss / 形态名 / 原始片段（DEBUG 日志用）/ 是否解析成功。
// 缺失字段、类型不符、JSON 损坏一律返回 ok=false，绝不 panic。
func parsePrefixCacheUsage(respBody []byte) (cacheRead, cacheMiss int, form, raw string, ok bool) {
	g, ok := infergeom.Parse(respBody)
	if !ok {
		return 0, 0, "", "", false
	}
	return g.CacheRead, g.CacheMiss, g.Form, g.Raw, true
}

// intField / rawSnippet —— T1.4 起已随解析核心一并收敛到 internal/infergeom
// （那边是唯一实现；网关侧不再保留副本，避免两套解析口径漂移）。

// ---------------------------------------------------------------------------
// prompt 版本 = 工具 schema 集合 + 系统提示模板 的哈希
// ---------------------------------------------------------------------------

// promptVersionOf 计算请求的 prompt 版本号。
//
// 粒度要求："工具/提示一变就变版本"。哈希输入 = tools 数组（规范化）+ 系统提示文本
// （顶层 system / instructions + messages 里 role==system 的内容）。
// 返回形如 "pv-0123456789ab"（sha256 前 12 hex）。body 为空/损坏时返回 ""。
func promptVersionOf(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var raw struct {
		Tools        json.RawMessage `json:"tools"`
		System       json.RawMessage `json:"system"`
		Instructions json.RawMessage `json:"instructions"`
		Messages     []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}

	h := sha256.New()
	// 工具 schema 集合（解码后重编码——Go 的 map key 排序保证规范化稳定）
	h.Write([]byte("tools:"))
	if len(raw.Tools) > 0 {
		var v interface{}
		if json.Unmarshal(raw.Tools, &v) == nil {
			if b, err := json.Marshal(v); err == nil {
				h.Write(b)
			}
		} else {
			// 无法解析时退化用原始字节（仍能反映"变了/没变"）
			h.Write(raw.Tools)
		}
	}
	// 系统提示模板
	h.Write([]byte("\nsystem:"))
	writeCanonical(h, raw.System)
	writeCanonical(h, raw.Instructions)
	for _, m := range raw.Messages {
		if m.Role == "system" {
			writeCanonical(h, m.Content)
		}
	}

	sum := h.Sum(nil)
	return "pv-" + hex.EncodeToString(sum[:])[:12]
}

// writeCanonical 把 raw JSON 规范化后写入 hash（无法解析则写原始字节）。
func writeCanonical(h interface{ Write([]byte) (int, error) }, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var v interface{}
	if json.Unmarshal(raw, &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			h.Write(b)
			return
		}
	}
	h.Write(raw)
}

// ---------------------------------------------------------------------------
// 记录 + 滑动窗口 + 版本级告警
// ---------------------------------------------------------------------------

// Record 记录一次采样。返回当前版本号、是否发生版本变更、以及（若触发）告警。
//
// 逻辑：
//  1. 计算 body 的 prompt 版本；与当前版本不同 → 固化上一版本命中率为基线、清空窗口、解除已告警标记。
//  2. 追加样本并按 (窗口时长 / 窗口样本数) 双向裁剪。
//  3. 若已有基线、样本数达标、且命中率显著下降 → 生成告警（每版本最多一次）。
func (t *prefixCacheTracker) Record(model string, body []byte, read, miss int) (version string, changed bool, alert *PrefixCacheAlert) {
	if t == nil {
		return "", false, nil
	}
	version = promptVersionOf(body)
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()
	// 丙批 C2：本次采样后按节流规则尝试落盘（在持锁状态下执行——LIFO defer 先于 Unlock）
	defer t.markDirtyLocked()

	st, ok := t.models[model]
	if !ok {
		st = &modelPrefixState{}
		t.models[model] = st
	}

	// 版本变更 → 重置基线（对比上一版本）
	if st.version != version {
		if st.version != "" {
			// 上一版本有样本才形成基线
			if h, m := windowTotals(st.samples); h+m > 0 {
				st.baselineRatio = ratioOf(h, m)
				st.hasBaseline = true
				st.prevVersion = st.version
			}
		}
		st.version = version
		st.samples = nil
		st.alerted = false
		changed = true
	}

	// 追样本 + 双向裁剪（时间跨度 + 样本数）
	st.samples = append(st.samples, cacheSample{t: now, cacheRead: read, cacheMiss: miss})
	t.trimLocked(st, now)
	st.updatedAt = now

	// 评估告警
	if st.hasBaseline && st.baselineRatio > 0 && !st.alerted {
		if len(st.samples) >= t.minAlertSamples {
			h, m := windowTotals(st.samples)
			cur := ratioOf(h, m)
			if reason, down := prefixAlertReason(cur, st.baselineRatio); down {
				a := PrefixCacheAlert{
					Model:         model,
					PromptVersion: version,
					PrevVersion:   st.prevVersion,
					Ratio:         round4(cur),
					BaselineRatio: round4(st.baselineRatio),
					Delta:         round4(cur - st.baselineRatio),
					DropPct:       round2(pctDrop(cur, st.baselineRatio)),
					Reason:        reason,
					Message: "前缀命中率版本级下降：" + reason +
						"（新版本 " + version + " 命中率 " + fmtPct(cur) +
						"，上一版本 " + st.prevVersion + " " + fmtPct(st.baselineRatio) + "）——" +
						"请核查是否在会话中途改动了工具定义或系统提示。",
					At: now.Format(time.RFC3339),
				}
				st.alerted = true
				t.appendAlertLocked(a)
				t.evictModelsLocked(now)
				return version, changed, &a
			}
		}
	}

	t.evictModelsLocked(now)
	return version, changed, nil
}

// trimLocked 裁剪窗口：剔除超龄（> windowD）样本，再保留最近 windowN 个。
func (t *prefixCacheTracker) trimLocked(st *modelPrefixState, now time.Time) {
	// 剔除超龄（至少保留 1 个——刚追加的当前样本）
	i := 0
	for i < len(st.samples)-1 && now.Sub(st.samples[i].t) > t.windowD {
		i++
	}
	if i > 0 {
		st.samples = st.samples[i:]
	}
	// 保留最近 windowN 个
	if len(st.samples) > t.windowN {
		st.samples = st.samples[len(st.samples)-t.windowN:]
	}
}

// appendAlertLocked 追加告警并保留最近 maxPrefixAlerts 条。
func (t *prefixCacheTracker) appendAlertLocked(a PrefixCacheAlert) {
	t.alerts = append(t.alerts, a)
	if len(t.alerts) > maxPrefixAlerts {
		t.alerts = t.alerts[len(t.alerts)-maxPrefixAlerts:]
	}
}

// evictModelsLocked 模型数超限时清理最久未更新的（保留有基线的优先不动）。
func (t *prefixCacheTracker) evictModelsLocked(now time.Time) {
	if len(t.models) <= maxPrefixModels {
		return
	}
	var oldestKey string
	var oldest time.Time
	for k, st := range t.models {
		if oldestKey == "" || st.updatedAt.Before(oldest) {
			oldestKey, oldest = k, st.updatedAt
		}
	}
	if oldestKey != "" && now.Sub(oldest) > t.windowD {
		delete(t.models, oldestKey)
	}
}

// windowTotals 汇总窗口样本的 (hits, misses)。
func windowTotals(samples []cacheSample) (hits, misses int) {
	for _, s := range samples {
		hits += s.cacheRead
		misses += s.cacheMiss
	}
	return hits, misses
}

// ratioOf 命中率 = read / (read + miss)；分母为 0 时返回 0。
func ratioOf(hits, misses int) float64 {
	total := hits + misses
	if total <= 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// prefixAlertReason 判断是否显著下降，返回中文原因与是否告警。
// 阈值：绝对下降 ≥ 30 个百分点，或低于上一版本的 60%。
func prefixAlertReason(cur, baseline float64) (string, bool) {
	if baseline <= 0 || cur >= baseline {
		return "", false
	}
	if baseline-cur >= prefixAlertAbsDrop {
		return "绝对下降 ≥ 30 个百分点", true
	}
	if cur < baseline*prefixAlertRelFactor {
		return "低于上一版本的 60%", true
	}
	return "", false
}

// pctDrop 相对下降百分比（基于基线；clamp 到 [0,100]）。
func pctDrop(cur, baseline float64) float64 {
	if baseline <= 0 {
		return 0
	}
	d := (baseline - cur) / baseline * 100
	if d < 0 {
		return 0
	}
	if d > 100 {
		return 100
	}
	return d
}

// ---------------------------------------------------------------------------
// 丙批 C2：跨重启持久化（statepath + 节流写）
// ---------------------------------------------------------------------------

// persistedSample 单个窗口样本的落盘形态（时间用 Unix 毫秒——跨重启稳定）。
type persistedSample struct {
	TMs  int64 `json:"t_ms"`
	Read int   `json:"read"` // 命中（复用前缀）的 token 数
	Miss int   `json:"miss"` // 未命中（需重新处理）的 token 数
}

// persistedModel 单模型（当前 prompt_version 窗口 + 基线）的落盘形态。
type persistedModel struct {
	PromptVersion string            `json:"prompt_version"`
	PrevVersion   string            `json:"prev_version,omitempty"`
	BaselineRatio float64           `json:"baseline_ratio"`
	HasBaseline   bool              `json:"has_baseline"`
	Alerted       bool              `json:"alerted"`
	UpdatedAtMs   int64             `json:"updated_at_ms"`
	Hits          int               `json:"hits"`   // 窗口内命中合计（冗余——便于人读/校对）
	Misses        int               `json:"misses"` // 窗口内未命中合计（冗余——便于人读/校对）
	Samples       []persistedSample `json:"samples"`
}

// persistedPrefixCache 落盘文件根结构（prefix_cache.json）。
type persistedPrefixCache struct {
	Version         int                       `json:"version"`
	SavedAt         string                    `json:"saved_at"`
	Models          map[string]persistedModel `json:"models"`
	Alerts          []PrefixCacheAlert        `json:"alerts"`
	UnknownForms    int                       `json:"unknown_forms"`
	UnknownLastKeys []string                  `json:"unknown_last_keys,omitempty"`
	UnknownShapes   []string                  `json:"unknown_shapes,omitempty"` // 形态签名集（2026-09-11 补齐：计数持久化而形态集不持久化=半个闭环）
}

// load 从磁盘恢复窗口/基线/告警/未知形态统计。
// 文件缺失 → 静默空态（首次启动）；读取失败或 JSON 损坏 → 记日志后忽略（不阻断启动、不 panic）。
func (t *prefixCacheTracker) load() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.persistPath == "" {
		return
	}
	b, err := os.ReadFile(t.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️ [prefix_cache] failed to read persistence file (ignored, starting empty): %v", err)
		}
		return
	}
	var pf persistedPrefixCache
	if err := json.Unmarshal(b, &pf); err != nil {
		log.Printf("⚠️ [prefix_cache] persistence file corrupted (ignored, starting empty): %v", err)
		return
	}

	models := map[string]*modelPrefixState{}
	for name, pm := range pf.Models {
		st := &modelPrefixState{
			version:       pm.PromptVersion,
			baselineRatio: pm.BaselineRatio,
			hasBaseline:   pm.HasBaseline,
			prevVersion:   pm.PrevVersion,
			alerted:       pm.Alerted,
			updatedAt:     time.UnixMilli(pm.UpdatedAtMs),
		}
		for _, ps := range pm.Samples {
			st.samples = append(st.samples, cacheSample{
				t:         time.UnixMilli(ps.TMs),
				cacheRead: ps.Read,
				cacheMiss: ps.Miss,
			})
		}
		// 兜底：样本为空但有合计值时合成一条聚合样本，保证 ratio 可复现。
		if len(st.samples) == 0 && (pm.Hits > 0 || pm.Misses > 0) {
			st.samples = append(st.samples, cacheSample{
				t: st.updatedAt, cacheRead: pm.Hits, cacheMiss: pm.Misses,
			})
		}
		models[name] = st
	}
	t.models = models
	if pf.Alerts != nil {
		t.alerts = pf.Alerts
	}
	t.unknownForms = pf.UnknownForms
	t.unknownLastKeys = append([]string(nil), pf.UnknownLastKeys...)
	if len(pf.UnknownShapes) > 0 {
		t.unknownShapes = make(map[string]bool, len(pf.UnknownShapes))
		for _, sig := range pf.UnknownShapes {
			if len(t.unknownShapes) >= maxUnknownShapes {
				break
			}
			t.unknownShapes[sig] = true
		}
	}
	log.Printf("✅ [prefix_cache] restored from %s: %d models / %d warnings / %d unknown shapes",
		t.persistPath, len(models), len(t.alerts), t.unknownForms)
}

// saveLocked 原子写盘（临时文件 + rename）——调用方须持 t.mu。
func (t *prefixCacheTracker) saveLocked() {
	if t.persistPath == "" {
		return
	}
	pf := persistedPrefixCache{
		Version:         persistedPrefixCacheVersion,
		SavedAt:         t.now().Format(time.RFC3339),
		Models:          make(map[string]persistedModel, len(t.models)),
		Alerts:          t.alerts,
		UnknownForms:    t.unknownForms,
		UnknownLastKeys: t.unknownLastKeys,
		UnknownShapes:   t.unknownShapeList(),
	}
	for name, st := range t.models {
		h, m := windowTotals(st.samples)
		pm := persistedModel{
			PromptVersion: st.version,
			PrevVersion:   st.prevVersion,
			BaselineRatio: st.baselineRatio,
			HasBaseline:   st.hasBaseline,
			Alerted:       st.alerted,
			UpdatedAtMs:   st.updatedAt.UnixMilli(),
			Hits:          h,
			Misses:        m,
		}
		for _, s := range st.samples {
			pm.Samples = append(pm.Samples, persistedSample{
				TMs: s.t.UnixMilli(), Read: s.cacheRead, Miss: s.cacheMiss,
			})
		}
		pf.Models[name] = pm
	}
	b, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		log.Printf("⚠️ [prefix_cache] persistence serialization failed (skipping this save): %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(t.persistPath), 0o755); err != nil {
		log.Printf("⚠️ [prefix_cache] failed to create persistence directory (skipping this save): %v", err)
		return
	}
	tmp := t.persistPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("⚠️ [prefix_cache] failed to write persistence temp file (skipping this save): %v", err)
		return
	}
	if err := os.Rename(tmp, t.persistPath); err != nil {
		log.Printf("⚠️ [prefix_cache] persistence atomic replace failed: %v", err)
		return
	}
	t.lastSaveAt = t.now()
	t.dirtySamples = 0
	t.dirty = false
}

// markDirtyLocked 记一笔待落盘变更，并按节流规则决定是否立即写盘：
// 累计变更 ≥ flushSamples 或距上次写盘 ≥ flushEvery 时写；否则留待下次或 Flush。
func (t *prefixCacheTracker) markDirtyLocked() {
	if t.persistPath == "" {
		return
	}
	t.dirtySamples++
	t.dirty = true
	if t.dirtySamples >= t.flushSamples || t.now().Sub(t.lastSaveAt) >= t.flushEvery {
		t.saveLocked()
	}
}

// Flush 强制立即落盘（进程优雅退出前调用；无待落盘变更则 no-op——保证最后一批不丢）。
func (t *prefixCacheTracker) Flush() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.persistPath == "" || !t.dirty {
		return
	}
	t.saveLocked()
}

// ---------------------------------------------------------------------------
// 丙批 C2：未知响应形态探测
// ---------------------------------------------------------------------------

// unknownFormKeys 提取响应体的键名列表（仅顶层 + usage/timings 的键名；不含任何值/原文——防泄漏）。
// 返回 (键名列表, 是否可解析为 JSON 对象)。空/损坏/非对象一律 ok=false（不计入未知形态）。
func unknownFormKeys(respBody []byte) ([]string, bool) {
	if len(respBody) == 0 {
		return nil, false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(respBody, &obj); err != nil {
		return nil, false
	}
	keys := make([]string, 0, 16)
	top := make([]string, 0, len(obj))
	for k := range obj {
		top = append(top, k)
	}
	sort.Strings(top) // 排序保证输出稳定（便于跨样本比对差异）
	keys = append(keys, top...)
	for _, sect := range []string{"usage", "timings"} {
		m, ok := obj[sect].(map[string]interface{})
		if !ok {
			continue
		}
		sub := make([]string, 0, len(m))
		for k := range m {
			sub = append(sub, k)
		}
		sort.Strings(sub)
		for _, k := range sub {
			keys = append(keys, sect+"."+k)
		}
	}
	return keys, true
}

// maxUnknownShapes — 形态签名集合上限（防无界增长）
const maxUnknownShapes = 16

// formSignature — 键名集 → 稳定签名（排序后拼接；空集给占位符）
func formSignature(keys []string) string {
	if len(keys) == 0 {
		return "(无键名)"
	}
	cp := append([]string(nil), keys...)
	sort.Strings(cp)
	return strings.Join(cp, "|")
}

// unknownShapeList — 形态签名列表（调用方持锁；排序输出，便于稳定展示/测试）
func (t *prefixCacheTracker) unknownShapeList() []string {
	if len(t.unknownShapes) == 0 {
		return nil
	}
	out := make([]string, 0, len(t.unknownShapes))
	for sig := range t.unknownShapes {
		out = append(out, sig)
	}
	sort.Strings(out)
	return out
}

// markUnknown 记录一次未知响应形态：计数 +1、更新最近样本键名列表（不含值），并按节流规则尝试落盘。
// 返回累计未知形态次数。keys 为空也表示"有未知但无键名"。
func (t *prefixCacheTracker) markUnknown(keys []string) int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.unknownForms++
	if len(keys) > maxUnknownLastKeys {
		keys = keys[:maxUnknownLastKeys]
	}
	t.unknownLastKeys = append([]string(nil), keys...)
	// 形态去重：同键名集合只记一次（探针价值在「发现了几种新形态」，不是「命中多少次」）
	if t.unknownShapes == nil {
		t.unknownShapes = make(map[string]bool)
	}
	if sig := formSignature(keys); len(t.unknownShapes) < maxUnknownShapes && !t.unknownShapes[sig] {
		t.unknownShapes[sig] = true
	}
	t.markDirtyLocked()
	return t.unknownForms
}

// ---------------------------------------------------------------------------
// 查询端点数据
// ---------------------------------------------------------------------------

// PrefixCacheWindowInfo 窗口元信息。
type PrefixCacheWindowInfo struct {
	Size        int `json:"size"`         // 窗口最大样本数（默认 50）
	DurationSec int `json:"duration_sec"` // 窗口最大时间跨度（默认 600s）
	Samples     int `json:"samples"`      // 当前窗口内样本数
}

// ModelPrefixStats 单模型统计（端点 models 明细用）。
type ModelPrefixStats struct {
	PromptVersion string  `json:"prompt_version"`
	PrevVersion   string  `json:"prev_version,omitempty"`
	Hits          int     `json:"hits"`
	Misses        int     `json:"misses"`
	Ratio         float64 `json:"ratio"`
	BaselineRatio float64 `json:"baseline_ratio"`
	HasBaseline   bool    `json:"has_baseline"`
	Samples       int     `json:"samples"`
	Alerted       bool    `json:"alerted"`
}

// PrefixCacheSnapshot GET /api/metrics/prefix_cache 的返回体。
// 必含字段：prompt_version / window / hits / misses / ratio / baseline_ratio / alerts。
type PrefixCacheSnapshot struct {
	PromptVersion string                      `json:"prompt_version"`
	Window        PrefixCacheWindowInfo       `json:"window"`
	Hits          int                         `json:"hits"`
	Misses        int                         `json:"misses"`
	Ratio         float64                     `json:"ratio"`
	BaselineRatio float64                     `json:"baseline_ratio"`
	HasBaseline   bool                        `json:"has_baseline"`
	Alerts        []PrefixCacheAlert          `json:"alerts"`
	Models        map[string]ModelPrefixStats `json:"models,omitempty"`
	// 丙批 C2：未知响应形态探测
	UnknownForms    int      `json:"unknown_forms"`               // 解析失败的响应计数（无 timings 且无已知 usage 字段）
	UnknownLastKeys []string `json:"unknown_last_keys,omitempty"` // 最近一条未知样本的键名列表（不含值）
	UnknownShapes   []string `json:"unknown_shapes,omitempty"`    // 去重后的形态签名（键名集；同一形态只列一次——「12 条同形态」≠「12 种形态」）
	Status          string   `json:"status"`
}

// Snapshot 生成查询快照。model 非空时只聚焦该模型；为空时聚合全部模型。
func (t *prefixCacheTracker) Snapshot(model string) PrefixCacheSnapshot {
	if t == nil {
		return PrefixCacheSnapshot{
			Window: PrefixCacheWindowInfo{Size: defaultPrefixWindowN, DurationSec: int(defaultPrefixWindowDur.Seconds())},
			Alerts: []PrefixCacheAlert{},
			Status: "disabled",
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	win := PrefixCacheWindowInfo{
		Size:        t.windowN,
		DurationSec: int(t.windowD.Seconds()),
	}
	models := map[string]ModelPrefixStats{}
	for name, st := range t.models {
		h, m := windowTotals(st.samples)
		models[name] = ModelPrefixStats{
			PromptVersion: st.version,
			PrevVersion:   st.prevVersion,
			Hits:          h,
			Misses:        m,
			Ratio:         round4(ratioOf(h, m)),
			BaselineRatio: round4(st.baselineRatio),
			HasBaseline:   st.hasBaseline,
			Samples:       len(st.samples),
			Alerted:       st.alerted,
		}
	}

	snap := PrefixCacheSnapshot{
		Alerts: []PrefixCacheAlert{},
		Status: "ok",
	}
	// 丙批 C2：未知响应形态统计（拷贝切片——解锁后调用方仍可安全读取）
	snap.UnknownForms = t.unknownForms
	if len(t.unknownLastKeys) > 0 {
		snap.UnknownLastKeys = append([]string(nil), t.unknownLastKeys...)
	}
	snap.UnknownShapes = t.unknownShapeList() // 形态去重（同键名集只列一次）

	// 过滤告警
	for _, a := range t.alerts {
		if model == "" || a.Model == model {
			snap.Alerts = append(snap.Alerts, a)
		}
	}
	// 最新在前，最多 20 条
	if len(snap.Alerts) > 20 {
		snap.Alerts = snap.Alerts[len(snap.Alerts)-20:]
	}
	reverseAlerts(snap.Alerts)

	if model != "" {
		st, ok := models[model]
		if !ok {
			snap.Window = win
			return snap
		}
		snap.PromptVersion = st.PromptVersion
		snap.Hits, snap.Misses, snap.Ratio = st.Hits, st.Misses, st.Ratio
		snap.BaselineRatio, snap.HasBaseline = st.BaselineRatio, st.HasBaseline
		win.Samples = st.Samples
		snap.Window = win
		return snap
	}

	// 聚合：合计 hits/misses，取样本数最多的模型作为版本代表
	totalH, totalM := 0, 0
	var leadKey string
	var lead ModelPrefixStats
	for name, st := range models {
		totalH += st.Hits
		totalM += st.Misses
		if st.Samples > lead.Samples {
			leadKey, lead = name, st
		}
	}
	snap.Hits, snap.Misses = totalH, totalM
	snap.Ratio = round4(ratioOf(totalH, totalM))
	snap.Window = win
	if leadKey != "" {
		snap.PromptVersion = lead.PromptVersion
		snap.BaselineRatio = lead.BaselineRatio
		snap.HasBaseline = lead.HasBaseline
		snap.Window.Samples = lead.Samples
	}
	// 无 ?model= 时给出全部模型明细
	if len(models) > 0 {
		snap.Models = models
	}
	return snap
}

// reverseAlerts 就地反转（最新的排前面）。
func reverseAlerts(a []PrefixCacheAlert) {
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
}

// round4 保留 4 位小数（命中率展示）。
func round4(f float64) float64 {
	return float64(int(f*10000+0.5)) / 10000
}

// round2 保留 2 位小数（百分比展示）。
func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// fmtPct 命中率 → 百分比字符串（整数）。
func fmtPct(f float64) string {
	return pcItoa(int(f*100+0.5)) + "%"
}

// pcItoa 极简整数转字符串（避免额外 import——注意与 compactor.go 的 itoa 区分，勿重名）。
func pcItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// Gateway 集成（记录 + 端点）
// ---------------------------------------------------------------------------

// recordPrefixCache 解析上游缓存计量并记录（丙批 N4）。
//
// 仅在响应体可用且能解析出缓存计量时生效；解析失败静默返回。
// 命中 DEBUG 日志展示真实响应字段，便于以后适配更多上游形态。
func (g *Gateway) recordPrefixCache(model string, forwardBody, respBody []byte) {
	if g.prefixCache == nil || len(respBody) == 0 {
		return
	}
	read, miss, form, raw, ok := parsePrefixCacheUsage(respBody)
	if !ok {
		// 丙批 C2：未知响应形态探测——记录键名（不含值）+ 计数 + WARN，便于以后发现新上游形态。
		// 仅当响应体可解析为 JSON 对象时计数；空/损坏/非对象不计（避免把解析错误误判为形态）。
		if keys, keyed := unknownFormKeys(respBody); keyed {
			n := g.prefixCache.markUnknown(keys)
			log.Printf("⚠️ [prefix_cache][WARN] unknown response shape (cumulative #%d) — sample keys: %v", n, keys)
			slog.Warn("prefix_cache unknown response shape", "count", n, "keys", keys)
		}
		return
	}
	version, changed, alert := g.prefixCache.Record(model, forwardBody, read, miss)

	// DEBUG：展示真实响应字段原文（需求①：便于以后适配）
	log.Printf("🔬 [prefix_cache][DEBUG] model=%s form=%s cache_read=%d cache_miss=%d version=%s raw=%s",
		model, form, read, miss, version, raw)
	slog.Debug("prefix_cache sampling",
		"model", model, "form", form, "cache_read", read, "cache_miss", miss, "prompt_version", version)

	if changed {
		log.Printf("🔄 [prefix_cache] version changed: model=%s new version=%s (window reset — comparing against baseline)", model, version)
	}
	if alert != nil {
		// 版本级告警：WARNING 日志 + 计入告警列表（列表在 tracker 内）
		slog.Warn("prefix hit rate dropped at version level",
			"model", alert.Model, "prompt_version", alert.PromptVersion,
			"ratio", alert.Ratio, "baseline_ratio", alert.BaselineRatio,
			"delta", alert.Delta, "reason", alert.Reason)
		log.Printf("⚠️ [prefix_cache][WARNING] %s", alert.Message)
	}
}

// handlePrefixCacheMetrics 处理 GET /api/metrics/prefix_cache（丙批 N4）。
// 可选 query 参数 ?model=xxx 聚焦单模型；不传则聚合全部模型并附 models 明细。
func (g *Gateway) handlePrefixCacheMetrics(w http.ResponseWriter, r *http.Request) {
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	snap := g.prefixCache.Snapshot(model) // nil 安全（Snapshot 内部处理）
	out, err := json.Marshal(snap)
	if err != nil {
		http.Error(w, `{"error":"marshal failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// FlushPrefixCache 强制将前缀命中率统计落盘（丙批 C2）。
// 供进程优雅退出钩子调用——保证节流窗口内最后一批样本不丢。
func (g *Gateway) FlushPrefixCache() {
	if g == nil || g.prefixCache == nil {
		return
	}
	g.prefixCache.Flush()
}
