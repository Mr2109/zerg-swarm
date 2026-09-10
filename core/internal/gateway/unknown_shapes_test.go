package gateway

import (
	"path/filepath"
	"testing"
)

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

// 2026-09-11 一致性审计补齐：unknown_shapes 跨重启往返（此前只持久化计数=半个闭环）
func TestUnknownShapesPersistRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "prefix_cache.json")
	tr := newPrefixCache(50, 0)
	tr.enablePersistence(file) // 隔离：不用 newPrefixCachePersistent（那会读生产状态文件）
	tr.markUnknown([]string{"output", "usage"})
	tr.markUnknown([]string{"choices"})
	if got := len(tr.Snapshot("").UnknownShapes); got != 2 {
		t.Fatalf("写入后应有 2 个签名，实得 %d", got)
	}
	tr.Flush() // 强制落盘

	tr2 := newPrefixCache(50, 0)
	tr2.enablePersistence(file) // 触发 load
	snap := tr2.Snapshot("")
	if len(snap.UnknownShapes) != 2 {
		t.Fatalf("重启后应恢复 2 个形态签名，实得 %v", snap.UnknownShapes)
	}
	if snap.UnknownForms != 2 {
		t.Fatalf("重启后计数应为 2，实得 %d", snap.UnknownForms)
	}
}
