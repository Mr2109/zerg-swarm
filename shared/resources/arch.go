package resources

// 本文件承载 KV cache 估算的"经验回退常量"（§八 Q4）。
//
// 纪律：这些常量**不是实测值**。只要在估算中用到它们一次，结果必须标 Estimated=true，
// 且 Basis 里必须写明"按架构族常量回退"——宁可标"估的"，不许冒充实测。
// 已知缺口：现有 GGUF 元数据探测只读 .context_length / .block_count，未读 n_kv_heads / head_dim，
// 故这两个量缺失时只能按架构族回退（补读项另行安排，不在本批）。

// DefaultBytesPerElem 是 KV 每元素字节数的回退常量（fp16 = 2 字节）。
// KV dtype 未知时回退到它，并标 estimated=true。
const DefaultBytesPerElem = 2.0

// DefaultOverheadGb 是引擎运行时/临时缓冲固定开销的回退常量（GiB，经验值）。
// 机器未提供 EngineOverheadGb 时回退到它，并标 estimated=true。
const DefaultOverheadGb = 2.0

// ArchKVFallback 是某架构族的 KV 回退参数（每个 KV 头的头维度 + KV 头数）。
type ArchKVFallback struct {
	KVHeads int
	HeadDim int
}

// archFallbacks 是架构族 → KV 回退常量的表（经验值，非实测；用即 estimated=true）。
// 表内为常见族；未列出的族视为"不可判定"，估算 fail-closed（不擅自套 generic 常量冒充已知）。
var archFallbacks = map[string]ArchKVFallback{
	"llama":     {KVHeads: 8, HeadDim: 128},
	"qwen2":     {KVHeads: 4, HeadDim: 128},
	"qwen3":     {KVHeads: 4, HeadDim: 128},
	"mistral":   {KVHeads: 8, HeadDim: 128},
	"gemma2":    {KVHeads: 4, HeadDim: 256},
	"gemma3":    {KVHeads: 4, HeadDim: 256},
	"phi3":      {KVHeads: 8, HeadDim: 96},
	"command-r": {KVHeads: 8, HeadDim: 128},
}

// ArchFallback 查架构族的 KV 回退常量；ok=false 表示该族未知（调用方须 fail-closed）。
func ArchFallback(family string) (ArchKVFallback, bool) {
	f, ok := archFallbacks[family]
	return f, ok
}

// SupportedArchFamilies 返回已登记的架构族名（供观测面/测试枚举；顺序无关）。
func SupportedArchFamilies() []string {
	out := make([]string, 0, len(archFallbacks))
	for k := range archFallbacks {
		out = append(out, k)
	}
	return out
}
