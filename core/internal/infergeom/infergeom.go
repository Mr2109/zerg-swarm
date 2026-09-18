// infergeom.go — 批量几何（batch geometry）的**唯一解析口**（T1.4，2026-09-17）。
//
// 背景（docs/调研/实验-批量几何与可复现性-T7.1-T7.2.md 实测坐实）：
// llama.cpp 的批量几何——同批内同时收尾需算 logits 的序列数 M、前缀缓存命中 cache_n、
// 本次实际评估 prompt_n——**确实改变 logits**：dense 模型在固定几何下逐比特可复现（0 ulp），
// MoE 跨几何差 2.4e-02；串行独占 5/5 相同，而同批有他人 25/25 全不同（差异率 ≥94.7%）。
// ⇒ 事件流必须把几何当**一等证据**记下，否则回放/回归的差异无法归因
// （缺字段的回放比对会**静默通过**——字段全空 = 处处相等 = 最危险的假绿）。
//
// 本包的位置：**叶子包**（零内部依赖，不进任何环），两个调用方——
// internal/gateway/prefix_cache.go（前缀命中率统计；T1.4 起解析核心**收敛到本包**，
// 网关侧只留形态适配，返回签名与语义一字不改，既有用例是这次收敛的守卫）
// 与 internal/chat（对话观测面 chat_obs.jsonl：把几何写进**大模型调用**那一类记录）。
//
// ⚠ 铁律（与 chat/obs.go 三条铁律同源）：**上游不给 ⇒ 拿不到就是拿不到**。
// 每个计量都配一个 Has* 存在标志：`cache_n=0`（冷缓存）是**有效测量值**，
// 与「上游没给这个字段」必须可分——调用方**不许**把缺席写成 0，也不许编默认值。
package infergeom

import (
	"encoding/json"
	"strings"
)

// ── 形态名（与网关既有口径一致：改这里必须同步 gateway/prefix_cache.go 的用例）──
const (
	FormDeepSeek  = "deepseek"         // usage.prompt_cache_hit_tokens / prompt_cache_miss_tokens
	FormOpenAI    = "openai"           // usage.prompt_tokens_details.cached_tokens
	FormAnthropic = "anthropic"        // usage.cache_read_input_tokens / cache_creation_input_tokens
	FormResponses = "openai-responses" // usage.input_tokens_details.cached_tokens（+ 顶层 output）
	FormLlamaCpp  = "llamacpp"         // timings.cache_n / timings.prompt_n
)

// Geometry — 一次上游推理调用的几何证据。
// 拿不到的字段一律 Has* = false（数值字段保持 0）——读侧**只认 Has***，不许拿 0 当"没有"。
type Geometry struct {
	// Form：命中的计量形态（决定 CacheRead/CacheMiss 的聚合口径）。空 = 没认出任何形态。
	Form string
	// CacheRead / CacheMiss：形态口径下的「命中 / 未命中」token 数。
	// 判据与原网关解析器一字不差（见 Parse 的优先级顺序）——命中率统计依赖它。
	CacheRead int
	CacheMiss int
	// Raw：usage/timings 原始片段（DEBUG/留痕用；截断到 600 字符，不含正文——防泄漏）。
	Raw string

	// —— 批量几何字段（上游给才记）——
	CacheN            int    // llama.cpp timings.cache_n —— 复用 KV 的 token 数（前缀命中）
	HasCacheN         bool   //
	PromptN           int    // llama.cpp timings.prompt_n —— 本次**实际评估**的 token 数（未命中）
	HasPromptN        bool   //
	CachedTokens      int    // usage.prompt_tokens_details.cached_tokens（Responses 式 input_tokens_details 亦认）
	HasCachedTokens   bool   //
	UbatchN           int    // ubatch 划分规模（llama.cpp 响应通常**不给** ⇒ 缺席）
	HasUbatchN        bool   //
	SlotID            int    // 命中的槽位（-np 多槽下的几何要素；上游给才记）
	HasSlotID         bool   //
	SystemFingerprint string // 引擎白送（实测形如 b10470-34af94cd9）；空 = 上游不给
}

// Any — 是否至少拿到一项几何证据（含 system_fingerprint；不含 form/raw 这类派生信息）。
// 调用方判据：false ⇒ 事件流里几何字段**全部缺席**（只留 geometry_recorded=false 的结论）。
func (g Geometry) Any() bool {
	return g.HasCacheN || g.HasPromptN || g.HasCachedTokens ||
		g.HasUbatchN || g.HasSlotID || g.SystemFingerprint != ""
}

// ubatchKeys / slotKeys —— 候选键名（挨个试，都没给 ⇒ 缺席）。
// llama.cpp 的 chat/completions 响应**通常不给**这两项（批次划分与槽位分配在服务端进程内部），
// 只有部分构建/接口会带 ⇒ 这里只做"上游给了就抄下来"，**绝不**按启动参数推断（那是编造）。
var (
	ubatchKeys = [][]string{
		{"timings", "ubatch_n"},
		{"timings", "n_ubatch"},
		{"timings", "ubatch_size"},
		{"ubatch_n"},
		{"n_ubatch"},
	}
	slotKeys = [][]string{
		{"slot_id"},
		{"id_slot"},
		{"timings", "slot_id"},
	}
)

// firstInt — 按候选路径逐条试，返回首个命中的 (值, true)；都没命中 ⇒ (0, false)。
func firstInt(obj map[string]interface{}, paths [][]string) (int, bool) {
	for _, p := range paths {
		if v, ok := intField(obj, p...); ok {
			return v, true
		}
	}
	return 0, false
}

// Parse — 从响应体 JSON 解析几何证据（不 panic、不改判：空/损坏/类型不符一律 ok=false）。
//
// ok 的语义**与既有网关解析器完全一致**：发现了**可识别的缓存计量**
// （决定 CacheRead/CacheMiss 两个聚合值，命中率统计依赖它）。
// 而 g.Any() 的语义更宽：只要拿到**任一**几何证据（含 system_fingerprint）即 true
// —— 例如"只给 system_fingerprint 不给 timings"的响应：ok=false 但 Any()=true。
//
// 形态优先级（一字不改沿用原解析器）：deepseek → openai → anthropic → openai-responses → llamacpp。
// 几何字段的抄录**与形态判定分开**：实测本仓 llama-server 同时给 openai 与 llamacpp 两形态；
// 形态优先级只决定聚合口径，而 cache_n/prompt_n 该记的都要记下来（它们才是决定 logits 的那一栏）。
func Parse(body []byte) (Geometry, bool) {
	if len(body) == 0 {
		return Geometry{}, false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return Geometry{}, false
	}

	usage, _ := obj["usage"].(map[string]interface{})
	timings, _ := obj["timings"].(map[string]interface{})

	g := Geometry{Raw: rawSnippet(usage, timings)}

	// ① 几何字段抄录（存在才记；逐字段独立于形态判定）
	if v, ok := intField(timings, "cache_n"); ok {
		g.CacheN, g.HasCacheN = v, true
	}
	if v, ok := intField(timings, "prompt_n"); ok {
		g.PromptN, g.HasPromptN = v, true
	}
	if v, ok := intField(usage, "prompt_tokens_details", "cached_tokens"); ok {
		g.CachedTokens, g.HasCachedTokens = v, true
	} else if v, ok := intField(usage, "input_tokens_details", "cached_tokens"); ok {
		g.CachedTokens, g.HasCachedTokens = v, true
	}
	if v, ok := firstInt(obj, ubatchKeys); ok {
		g.UbatchN, g.HasUbatchN = v, true
	}
	if v, ok := firstInt(obj, slotKeys); ok {
		g.SlotID, g.HasSlotID = v, true
	}
	if fp, ok := obj["system_fingerprint"].(string); ok && fp != "" {
		g.SystemFingerprint = fp
	}

	// ② 形态判定 + 聚合口径（与原解析器同序、同判据）
	read, miss, form, ok := aggregate(obj, usage, timings)
	if !ok {
		return g, false
	}
	g.Form, g.CacheRead, g.CacheMiss = form, read, miss
	return g, true
}

// aggregate — 形态判定与 (read, miss) 计算（T1.4 从 gateway/prefix_cache.go 原样收敛而来）。
func aggregate(obj, usage, timings map[string]interface{}) (read, miss int, form string, ok bool) {
	// ① DeepSeek 式（最显式：直接给 hit/miss 两个字段）
	if usage != nil {
		hit, hasHit := intField(usage, "prompt_cache_hit_tokens")
		m, hasMiss := intField(usage, "prompt_cache_miss_tokens")
		if hasHit || hasMiss {
			return hit, m, FormDeepSeek, true
		}
	}
	// ② OpenAI 式（cached_tokens；miss = prompt_tokens - cached_tokens）
	if usage != nil {
		if cached, hasCached := intField(usage, "prompt_tokens_details", "cached_tokens"); hasCached {
			prompt, hasPrompt := intField(usage, "prompt_tokens")
			m := 0
			if hasPrompt && prompt > cached {
				m = prompt - cached
			}
			return cached, m, FormOpenAI, true
		}
	}
	// ③ Anthropic 式（cache_read / cache_creation）
	if usage != nil {
		r, hasRead := intField(usage, "cache_read_input_tokens")
		creation, hasCreation := intField(usage, "cache_creation_input_tokens")
		if hasRead || hasCreation {
			return r, creation, FormAnthropic, true
		}
	}
	// ④ OpenAI Responses 式（2026-09-10 实测发现：键名 output/status/usage.input_tokens…）
	if usage != nil {
		if _, isResponses := obj["output"]; isResponses {
			cached, hasCached := intField(usage, "input_tokens_details", "cached_tokens")
			input, hasInput := intField(usage, "input_tokens")
			if hasCached || hasInput {
				m := 0
				if hasInput && input > cached {
					m = input - cached
				}
				return cached, m, FormResponses, true
			}
		}
	}
	// ⑤ llama.cpp 式（timings.cache_n / timings.prompt_n）
	if timings != nil {
		cacheN, hasCacheN := intField(timings, "cache_n")
		promptN, hasPromptN := intField(timings, "prompt_n")
		if hasCacheN || hasPromptN {
			return cacheN, promptN, FormLlamaCpp, true
		}
	}
	return 0, 0, "", false
}

// CarriesGeometry — 这段载荷是否**可能**带几何（流式逐块筛选用：只看键名，不解析、不判断）。
// 用途：流式路径只留最后一条"像几何"的块，流结束后再 Parse 一次（llama.cpp 把 timings/usage/
// system_fingerprint 放在**末块**；但末块之前也可能有 usage ⇒ 取最后一条命中者）。
func CarriesGeometry(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	s := string(payload)
	return strings.Contains(s, `"timings"`) ||
		strings.Contains(s, `"usage"`) ||
		strings.Contains(s, `"system_fingerprint"`)
}

// intField 从嵌套 map 逐层取整数字段（兼容 JSON 数字解析为 float64）。
// 返回 (值, 是否存在)。任一中间层缺失或类型不符即返回 false。
func intField(m map[string]interface{}, path ...string) (int, bool) {
	cur := interface{}(m)
	for _, key := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return 0, false
		}
		cur, ok = mm[key]
		if !ok {
			return 0, false
		}
	}
	switch v := cur.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// rawSnippet 拼装原始 usage/timings 片段（DEBUG 日志展示——便于以后适配新形态）。
// 上限 600 字符，避免污染日志。
func rawSnippet(usage, timings map[string]interface{}) string {
	out := map[string]interface{}{}
	if usage != nil {
		out["usage"] = usage
	}
	if timings != nil {
		out["timings"] = timings
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 600 {
		s = s[:600] + "…"
	}
	return s
}
