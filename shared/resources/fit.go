package resources

import (
	"fmt"
	"strings"
)

// 结论三态（§3.2 可解释输出 / §四.2 FitEstimate.verdict）。
const (
	VerdictFit   = "fit"    // 现在（不驱逐）就装得下
	VerdictEvict = "evict"  // 驱逐可腾退的驻留后装得下，已列出计划
	VerdictNoFit = "no_fit" // 装不下 / 不可判定（fail-closed）
)

// FitQuery 是"跑得动吗"的模型侧输入（§3.2 三要素所需）。
// 缺关键输入时不得默许放行——由 EstimateFit fail-closed。
type FitQuery struct {
	Model        string  // 别名/摘要，仅回填到输出
	Ctx          int     // 目标上下文长度（估算输入，不能只报权重体积）
	WeightsBytes int64   // 权重（量化后）真实文件字节数；<=0 表示未知 → fail-closed
	NLayer       int     // n_layers（block_count）；<=0 表示未知 → fail-closed
	NKvHeads     int     // n_kv_heads；<=0 表示未知 → 按架构族回退（estimated=true）
	HeadDim      int     // head_dim；<=0 表示未知 → 按架构族回退（estimated=true）
	BytesPerElem float64 // KV 每元素字节（dtype）；<=0 表示未知 → 回退常量（estimated=true）
	ArchFamily   string  // 架构族（llama/qwen2/...）；KV 参数缺失时用于回退
}

// FitEstimate 是"跑得动吗"的可解释估算（§四.2 形状）。
type FitEstimate struct {
	Model         string   `json:"model"`
	Ctx           int      `json:"ctx"`
	WeightsBytes  int64    `json:"weights_bytes"`
	KvCacheBytes  int64    `json:"kv_cache_bytes"`
	OverheadBytes int64    `json:"overhead_bytes"`
	NeedBytes     int64    `json:"need_bytes"`
	FreeBytes     int64    `json:"free_bytes"`     // 可腾退后的可用上限（avail + 可驱逐占用）
	FreeNowBytes  int64    `json:"free_now_bytes"` // 现在就能用（不驱逐任何驻留）
	VramNeedBytes int64    `json:"vram_need_bytes,omitempty"`
	VramFreeBytes int64    `json:"vram_free_bytes,omitempty"`
	Verdict       string   `json:"verdict"` // fit | evict | no_fit
	EvictPlan     []string `json:"evict_plan,omitempty"`
	Estimated     bool     `json:"estimated"` // true=含经验常量/回退假设，非全实测
	Basis         string   `json:"basis"`     // 人类可读的来源说明（恒非空）
}

// Fits 报告结论是否为"装得下"。
func (e FitEstimate) Fits() bool { return e.Verdict == VerdictFit }

// EstimateFit 对 MachineLedger 做"跑得动吗"估算。
//
// 三要素：need = weights_bytes + kv_cache_bytes + overhead_bytes（§3.2）。
//   - 有真值 → 用真值，estimated=false；
//   - 缺 KV 参数（n_kv_heads/head_dim/dtype）→ 按架构族回退，estimated=true（Q4）；
//   - 缺关键输入（权重字节/上下文/层数；或 KV 缺失且架构族未知）→ fail-closed：verdict=no_fit。
//
// 比较口径（§3.2/§3.3d）：内存与显存**二者取严**（任一不够即装不下）。
// 显存：有独立显存 → 与内存一同取严；统一内存（显存即内存，§3.1）→ 按内存口径判；
// 显存未知**且非统一内存** → fail-closed（estimated=true，绝不把"未知"当"无限"放行）。
// verdict=fit 表示不驱逐即可；evict 表示驱逐一（够用的）批后即可，已给出有序计划（§3.3d）。
func EstimateFit(q FitQuery, m MachineLedger) FitEstimate {
	est := FitEstimate{Model: q.Model, Ctx: q.Ctx}
	var basis []string
	failClosed := func(reason string) FitEstimate {
		est.Verdict = VerdictNoFit
		est.Estimated = true
		basis = append(basis, reason)
		est.Basis = strings.Join(basis, "；")
		return est
	}

	// ---- 关键输入缺失 → fail-closed（不许默认放行）----
	if q.WeightsBytes <= 0 {
		return failClosed("权重字节数缺失：无法估算权重分量（fail-closed 判装不下）")
	}
	if q.Ctx <= 0 {
		return failClosed("上下文长度缺失：KV cache 随上下文线性增长，无法估算（fail-closed 判装不下）")
	}
	if q.NLayer <= 0 {
		return failClosed("层数（block_count）缺失：KV cache 无法估算（fail-closed 判装不下）")
	}
	if m.MemAvailGb < 0 || m.VramFreeGb < 0 {
		return failClosed("机器可用内存/显存为负：输入非法（fail-closed 判装不下）")
	}
	est.WeightsBytes = q.WeightsBytes
	basis = append(basis, fmt.Sprintf("权重=%d 字节（量化后文件真实大小）", q.WeightsBytes))

	// ---- KV cache 三参：有真值用真值，缺则按架构族回退并标 estimated ----
	kvHeads, headDim, bpe := q.NKvHeads, q.HeadDim, q.BytesPerElem
	fb, famOK := ArchFallback(q.ArchFamily)
	if kvHeads <= 0 || headDim <= 0 {
		if !famOK {
			return failClosed(fmt.Sprintf("KV 头参数缺失且架构族 %q 未登记：不可回退（fail-closed 判装不下）", q.ArchFamily))
		}
	}
	if kvHeads <= 0 {
		kvHeads = fb.KVHeads
		est.Estimated = true
		basis = append(basis, fmt.Sprintf("n_kv_heads 缺失，按架构族 %s 常量回退=%d", q.ArchFamily, kvHeads))
	}
	if headDim <= 0 {
		headDim = fb.HeadDim
		est.Estimated = true
		basis = append(basis, fmt.Sprintf("head_dim 缺失，按架构族 %s 常量回退=%d", q.ArchFamily, headDim))
	}
	if bpe <= 0 {
		bpe = DefaultBytesPerElem
		est.Estimated = true
		basis = append(basis, fmt.Sprintf("KV dtype 缺失，回退每元素 %.0f 字节", bpe))
	}
	// 2 = K 与 V 两份（§3.2 公式：2 × n_layers × n_kv_heads × head_dim × bytes_per_elem × ctx）
	est.KvCacheBytes = int64(2.0 * float64(q.NLayer) * float64(kvHeads) * float64(headDim) * bpe * float64(q.Ctx))
	basis = append(basis, fmt.Sprintf("KV=2×%d×%d×%d×%.0f×%d=%d 字节", q.NLayer, kvHeads, headDim, bpe, q.Ctx, est.KvCacheBytes))

	// ---- overhead：机器可配置；缺失回退常量并标 estimated ----
	overheadGb := m.EngineOverheadGb
	if overheadGb <= 0 {
		overheadGb = DefaultOverheadGb
		est.Estimated = true
		basis = append(basis, fmt.Sprintf("引擎开销未提供，回退常量 %.1f GiB", overheadGb))
	} else {
		basis = append(basis, fmt.Sprintf("引擎开销=%.2f GiB（机器提供）", overheadGb))
	}
	est.OverheadBytes = gbToBytes(overheadGb)

	est.NeedBytes = est.WeightsBytes + est.KvCacheBytes + est.OverheadBytes
	basis = append(basis, fmt.Sprintf("need=%d 字节", est.NeedBytes))

	// ---- 可用量：现在 vs 驱逐可腾退后（口径：avail + 可驱逐占用）----
	evictableGb := evictableOccupiedGb(m.Resident)
	est.FreeNowBytes = gbToBytes(m.MemAvailGb)
	est.FreeBytes = gbToBytes(m.MemAvailGb + evictableGb)
	basis = append(basis, fmt.Sprintf("free_now=%.2f GiB，可驱逐腾退=%.2f GiB，free=%.2f GiB",
		m.MemAvailGb, evictableGb, m.MemAvailGb+evictableGb))

	// ---- 显存约束（独立生效；§3.3d 内存与显存二者取严）----
	memNowOK := est.NeedBytes <= est.FreeNowBytes
	memEvictOK := est.NeedBytes <= est.FreeBytes
	vramNowOK, vramEvictOK := true, true
	switch {
	case m.VramKnown():
		est.VramNeedBytes = est.NeedBytes
		est.VramFreeBytes = gbToBytes(m.VramFreeGb)
		vramNowOK = est.NeedBytes <= est.VramFreeBytes
		vramEvictOK = est.NeedBytes <= gbToBytes(m.VramFreeGb+evictableGb)
		basis = append(basis, fmt.Sprintf("显存：空闲=%.2f GiB（二者取严）", m.VramFreeGb))
	case m.UnifiedMemory:
		// 统一内存（Apple Silicon）：显存即内存 —— 内存口径已表达该约束。
		// 这是"知道"（设计稿 §3.1），不是"猜"，故不因此置 estimated。
		basis = append(basis, "显存：统一内存（显存即内存），按内存口径判（§3.1）")
	default:
		// 显存拿不到又不是统一内存：不得当成"显存无限"默默放行 —— fail-closed（§3.2 诚实原则）
		return failClosed("显存未知且非统一内存：无法排除显存不足（fail-closed，不把未知当无限）")
	}

	// ---- 结论 ----
	switch {
	case memNowOK && vramNowOK:
		est.Verdict = VerdictFit
		basis = append(basis, "结论=fit（不驱逐即可）")
	case memEvictOK && vramEvictOK:
		est.Verdict = VerdictEvict
		// 缺口口径（§3.3d 二者取严）：内存缺口与显存缺口取大者——只看内存会给显存受限的机器
		// 算出负缺口/空计划（"说 evict 却不知道赶谁"）。
		deficit := est.NeedBytes - est.FreeNowBytes
		if m.VramKnown() && !vramNowOK {
			if vd := est.NeedBytes - est.VramFreeBytes; vd > deficit {
				deficit = vd
			}
		}
		if deficit < 0 {
			deficit = 0
		}
		deficitGb := bytesToGb(deficit)
		est.EvictPlan = EvictToFree(m.Resident, deficitGb)
		basis = append(basis, fmt.Sprintf("结论=evict（需腾退 %.2f GiB，计划=%v）", deficitGb, est.EvictPlan))
	default:
		est.Verdict = VerdictNoFit
		basis = append(basis, fmt.Sprintf("结论=no_fit（需 %.2f GiB，可腾退后仅 %.2f GiB）",
			bytesToGb(est.NeedBytes), bytesToGb(est.FreeBytes)))
	}
	if est.Estimated {
		basis = append(basis, "★estimated=true（含经验常量/回退，非全实测）")
	} else {
		basis = append(basis, "estimated=false（全部输入为真值）")
	}
	est.Basis = strings.Join(basis, "；")
	return est
}

// EstimateFit 是 MachineLedger 上的便捷方法（同 EstimateFit(q, m)）。
func (m MachineLedger) EstimateFit(q FitQuery) FitEstimate { return EstimateFit(q, m) }
