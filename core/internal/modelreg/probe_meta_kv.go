package modelreg

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// probe_meta_kv.go —— 批 3：把 GGUF 真值接到"跑得动吗"估算（《设计-资源管理器》§八 Q4 / §3.2）。
//
// 背景（设计稿 §3.2 的诚实边界）：KV cache = 2 × n_layers × n_kv_heads × head_dim × bytes_per_elem × ctx。
// 其中 n_layers/context 早已读得到，但 n_kv_heads/head_dim 以前没读 → 只能按架构族经验常量回退，
// 结果必须标 estimated=true（不许把猜当实测）。本批补读 GGUF 真值：
//   - n_kv_heads  ← {arch}.attention.head_count_kv
//   - head_dim    ← {arch}.attention.key_length（新式 GGUF 显式给）
//                    否则由 {arch}.embedding_length ÷ {arch}.attention.head_count 推得（与 llama.cpp 同口径），
//                    再否则用 {arch}.rope.dimension_count 作代理——推得/代理一律**仍标 estimated**（见下）。
//
// KV cache 的每元素字节数**GGUF 里没有**（它是引擎运行参数，如 llama-server --cache-type-k/v fp16=2），
// 因此只能由调用方给（bytesPerElem）；不给就回退常量并标 estimated=true——不拿权重量化类型冒充。

// KVHeads 返回 KV 头数真值；0 表示该 GGUF 没有这个键（调用方按回退处理）。
func (m *GGUFMeta) KVHeads() int {
	if m == nil {
		return 0
	}
	return m.AttentionHeadKv
}

// HeadDim 返回 KV 头的头维度：优先 attention.key_length（真值），
// 否则 embedding_length/attention.head_count（推导，与 llama.cpp 同口径），
// 否则 rope.dimension_count（代理）。derived=true 表示后两种（不是 GGUF 直给的真值）。
func (m *GGUFMeta) HeadDim() (dim int, derived bool) {
	if m == nil {
		return 0, false
	}
	if m.KeyLength > 0 {
		return m.KeyLength, false
	}
	if m.EmbedLength > 0 && m.AttentionHeadN > 0 {
		if d := m.EmbedLength / m.AttentionHeadN; d > 0 {
			return d, true
		}
	}
	if m.RopeDimCount > 0 {
		return m.RopeDimCount, true
	}
	if m.ValueLength > 0 {
		return m.ValueLength, true
	}
	return 0, false
}

// FitInputFromMeta 把 GGUF 真值翻译成估算输入（§3.2 三要素的模型侧）。
//
// 参数：
//   - meta           GGUF 元数据（真值来源；nil 表示没有元数据）
//   - model          名字/摘要（仅回填输出）
//   - ctx            目标上下文长度（估算输入，不能只报权重体积）
//   - weightsBytes   权重字节数（量化后真实文件大小；<=0 → 估算会 fail-closed）
//   - bytesPerElem   KV cache 每元素字节（引擎运行参数；<=0 → 回退常量并标 estimated）
//   - archFamily     KV 参数缺失时的回退族（未登记族在估算里 fail-closed）
//
// 返回 (FitQuery, notes)：notes 逐条写明"哪些量是推得/代理来的"——调用方必须据此把结论标 estimated
// （见 EstimateFitFromMeta）。读不到的字段一律留 0，不编造。
func FitInputFromMeta(meta *GGUFMeta, model string, ctx int, weightsBytes int64, bytesPerElem float64, archFamily string) (resources.FitQuery, []string) {
	q := resources.FitQuery{
		Model:        model,
		Ctx:          ctx,
		WeightsBytes: weightsBytes,
		BytesPerElem: bytesPerElem,
		ArchFamily:   archFamily,
	}
	if meta == nil {
		return q, []string{"无 GGUF 元数据：n_layers/n_kv_heads/head_dim 全部缺席（估算将按回退/fail-closed 处理）"}
	}
	q.NLayer = meta.BlockCount
	q.NKvHeads = meta.KVHeads()
	dim, derived := meta.HeadDim()
	q.HeadDim = dim
	var notes []string
	if derived && dim > 0 {
		notes = append(notes, fmt.Sprintf("head_dim=%d 为推导/代理值（GGUF 无 attention.key_length）", dim))
	}
	return q, notes
}

// EstimateFitFromMeta 一步到位：GGUF 真值 → "跑得动吗"结论（含诚实标注）。
//
// 诚实规则（不许把猜当实测）：
//   - 只有"GGUF 直给的真值 + 调用方给的 KV dtype"才配 estimated=false；
//   - head_dim 是推导/代理值，或 KV dtype 缺失（走回退常量）时，即便 EstimateFit 本身算出了
//     "全真值"的结论，也必须把 estimated 拉回 true 并把原因写进 basis。
func EstimateFitFromMeta(meta *GGUFMeta, model string, ctx int, weightsBytes int64, bytesPerElem float64, archFamily string, machine resources.MachineLedger) resources.FitEstimate {
	q, notes := FitInputFromMeta(meta, model, ctx, weightsBytes, bytesPerElem, archFamily)
	if bytesPerElem <= 0 {
		notes = append(notes, "KV cache dtype 未知（GGUF 不记录，须由引擎参数给）——按回退常量估算")
	}
	est := resources.EstimateFit(q, machine)
	if len(notes) > 0 && !est.Estimated {
		est.Estimated = true
		est.Basis = est.Basis + "；★" + joinNotes(notes)
	}
	return est
}

// joinNotes 把备注拼成一行（原样保留，不加日期/人名）。
func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "；"
		}
		out += n
	}
	return out
}
