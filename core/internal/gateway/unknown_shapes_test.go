package gateway

import "testing"

// 丙批 C2 补（2026-09-10）：未知形态按「形态」去重——12 条同形态 ≠ 12 种形态
func TestUnknownFormShapesDedup(t *testing.T) {
	// 用非持久化构造器：本测试只验「形态去重」（内存语义）。
	// 注意 newPrefixCachePersistent 会 load 生产状态文件（~/.zerg/state/prefix_cache.json），测试禁用。
	tr := newPrefixCache(50, 0)
	// 同一形态 3 次
	for i := 0; i < 3; i++ {
		tr.markUnknown([]string{"output", "usage", "usage.input_tokens"})
	}
	snap := tr.Snapshot("")
	if snap.UnknownForms != 3 {
		t.Fatalf("unknown_forms = %d，期望 3", snap.UnknownForms)
	}
	if len(snap.UnknownShapes) != 1 {
		t.Fatalf("同形态 3 次应只有 1 个签名，实得 %v", snap.UnknownShapes)
	}
	// 第二种形态
	tr.markUnknown([]string{"choices", "usage.completion_tokens"})
	snap2 := tr.Snapshot("")
	if len(snap2.UnknownShapes) != 2 {
		t.Fatalf("不同形态应得 2 个签名，实得 %v", snap2.UnknownShapes)
	}
	// 键序不同但集合相同 → 同一签名
	tr.markUnknown([]string{"usage", "output", "usage.input_tokens"})
	if got := len(tr.Snapshot("").UnknownShapes); got != 2 {
		t.Fatalf("键序不同不应新增签名，实得 %d", got)
	}
	// 上限保护
	for i := 0; i < 40; i++ {
		tr.markUnknown([]string{"shape", string(rune('a' + i%26)), string(rune('0' + i%10))})
	}
	if got := len(tr.Snapshot("").UnknownShapes); got > maxUnknownShapes {
		t.Fatalf("签名数 %d 超过上限 %d", got, maxUnknownShapes)
	}
}
