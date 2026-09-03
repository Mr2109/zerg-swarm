package modelreg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// ── 批 3：probe_meta 补读 KV 头数/头维度，供"跑得动吗"用真值（§八 Q4） ─────────

// 写一个带 KV 真值键的 GGUF（照真实 llama.cpp 键名）。
func writeGGUFWithKV(t *testing.T, name string, withKeyLength bool, withHeads bool) string {
	t.Helper()
	kvs := [][]byte{
		kv("general.architecture", ggufTypeString, gstr("llama")),
		kv("llama.context_length", ggufTypeUint32, le32(4096)),
		kv("llama.block_count", ggufTypeUint32, le32(32)),
		kv("llama.embedding_length", ggufTypeUint32, le32(4096)),
	}
	if withHeads {
		kvs = append(kvs,
			kv("llama.attention.head_count", ggufTypeUint32, le32(32)),
			kv("llama.attention.head_count_kv", ggufTypeUint32, le32(8)),
		)
	}
	if withKeyLength {
		kvs = append(kvs, kv("llama.attention.key_length", ggufTypeUint32, le32(128)))
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, makeGGUF(kvs...), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func fitMachine() resources.MachineLedger {
	return resources.MachineLedger{
		Machine:          "x3",
		MemTotalGb:       128,
		MemAvailGb:       120,
		EngineOverheadGb: 2,  // 机器提供开销 → 不因开销缺失而标 estimated
		VramTotalGb:      24, // 独立显存（GPU 机）——显存未知会 fail-closed，故测试给出真值
		VramFreeGb:       24,
	}
}

// 有键：KV 头数/头维度是真值 → estimated=false，且 KV 字节数等于公式值。
func TestProbeMeta_EstimateFitUsesKVTruth(t *testing.T) {
	p := writeGGUFWithKV(t, "kvtruth.gguf", true, true)
	meta, tr, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatalf("解析 GGUF 失败：%v", err)
	}
	if meta.KVHeads() != 8 {
		t.Fatalf("n_kv_heads 应为 8（attention.head_count_kv），实得 %d", meta.KVHeads())
	}
	if dim, derived := meta.HeadDim(); dim != 128 || derived {
		t.Fatalf("head_dim 应为真值 128（attention.key_length），实得 %d derived=%v", dim, derived)
	}
	if meta.AttentionHeadN != 32 {
		t.Fatalf("attention.head_count 应为 32，实得 %d", meta.AttentionHeadN)
	}
	if tr.Summary == "" || !contains(tr.Summary, "kv_heads=8") {
		t.Fatalf("探测留痕应带 kv_heads（证据链），实得 %q", tr.Summary)
	}

	const weights = int64(4) << 30
	est := EstimateFitFromMeta(meta, "TestModel", 4096, weights, 2.0, "llama", fitMachine())
	if est.Estimated {
		t.Fatalf("全部真值（KV 头数/头维度/dtype 齐备）时不得标 estimated；basis=%s", est.Basis)
	}
	wantKv := int64(2.0 * 32 * 8 * 128 * 2.0 * 4096)
	if est.KvCacheBytes != wantKv {
		t.Fatalf("KV 字节数应为 %d（2×32层×8头×128×2B×4096），实得 %d", wantKv, est.KvCacheBytes)
	}
	if est.Verdict != resources.VerdictFit {
		t.Fatalf("内存充裕应判 fit，实得 %s（%s）", est.Verdict, est.Basis)
	}
}

// 无键：KV 头数/头维度读不到 → 按架构族回退且 estimated=true（宁可标估，不许冒充实测）。
func TestProbeMeta_NoKVKeys_FallsBackAndMarksEstimated(t *testing.T) {
	p := writeTestGGUF(t, "", "nokv.gguf")
	meta, _, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatalf("解析 GGUF 失败：%v", err)
	}
	if meta.KVHeads() != 0 || meta.AttentionHeadN != 0 {
		t.Fatalf("无键时必须是 0（缺席），实得 kv=%d n=%d", meta.KVHeads(), meta.AttentionHeadN)
	}
	if dim, _ := meta.HeadDim(); dim != 0 {
		t.Fatalf("无键且无推导来源时 head_dim 必须为 0，实得 %d", dim)
	}
	est := EstimateFitFromMeta(meta, "TestModel", 4096, int64(4)<<30, 2.0, "llama", fitMachine())
	if !est.Estimated {
		t.Fatalf("回退常量算出的结论必须标 estimated=true；basis=%s", est.Basis)
	}
	if !contains(est.Basis, "回退") {
		t.Fatalf("basis 必须写明用了架构族回退，实得 %q", est.Basis)
	}

	// 反例：架构族未登记 → 连回退都没有 → fail-closed（估不出就不装）
	bad := EstimateFitFromMeta(meta, "TestModel", 4096, int64(4)<<30, 2.0, "", fitMachine())
	if bad.Verdict != resources.VerdictNoFit {
		t.Fatalf("未登记族 + 缺 KV 参数必须 fail-closed（no_fit），实得 %s", bad.Verdict)
	}
}

// 推导值：有 head_count/embedding_length 但无 key_length → 推得的 head_dim 仍必须标 estimated。
func TestProbeMeta_DerivedHeadDim_MarksEstimated(t *testing.T) {
	p := writeGGUFWithKV(t, "derived.gguf", false, true)
	meta, _, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatalf("解析 GGUF 失败：%v", err)
	}
	if dim, derived := meta.HeadDim(); dim != 128 || !derived {
		t.Fatalf("应由 embedding_length/head_count 推得 128 且 derived=true，实得 %d derived=%v", dim, derived)
	}
	est := EstimateFitFromMeta(meta, "TestModel", 4096, int64(4)<<30, 2.0, "llama", fitMachine())
	if !est.Estimated {
		t.Fatalf("推导值不得冒充实测——必须标 estimated；basis=%s", est.Basis)
	}
	if !contains(est.Basis, "推导/代理") {
		t.Fatalf("basis 必须写明 head_dim 是推导/代理值，实得 %q", est.Basis)
	}
}

// KV dtype 未知（GGUF 不记录，须由引擎参数给）→ 标 estimated（不许拿权重量化类型冒充）。
func TestProbeMeta_KVDtypeUnknown_MarksEstimated(t *testing.T) {
	p := writeGGUFWithKV(t, "nodtype.gguf", true, true)
	meta, _, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatal(err)
	}
	est := EstimateFitFromMeta(meta, "TestModel", 4096, int64(4)<<30, 0, "llama", fitMachine())
	if !est.Estimated {
		t.Fatalf("KV dtype 缺失必须标 estimated；basis=%s", est.Basis)
	}
	if !contains(est.Basis, "dtype") {
		t.Fatalf("basis 应写明 KV dtype 未知，实得 %q", est.Basis)
	}
}

// contains 是 strings.Contains 的小包装（本文件只做子串断言）。
func contains(s, sub string) bool { return strings.Contains(s, sub) }
