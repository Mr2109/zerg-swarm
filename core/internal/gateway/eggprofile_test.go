package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

// 事实优先：档案里有 ctx_window 就用它；无档案/无字段/坏值 ⇒ 一律 (0,false) 交给上层退回意图/默认。
func TestProfileCtxWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	if _, ok := ProfileCtxWindow("Qwen3.8-27B"); ok {
		t.Fatal("无档案时应返回 false")
	}
	// 正常档案（含注释行，模拟真实档案形态）
	if err := os.WriteFile(filepath.Join(dir, "Qwen3.8-27B.yaml"),
		[]byte("# 实测档案\n# ctx_window: 999(注释应忽略)\nctx_window: 262144\nmachine: x3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, ok := ProfileCtxWindow("Qwen3.8-27B")
	if !ok || v != 262144 {
		t.Fatalf("档案值读取错：v=%d ok=%v", v, ok)
	}
	// 大小写不敏感文件名兜底
	if v2, ok2 := ProfileCtxWindow("qwen3.8-27b"); !ok2 || v2 != 262144 {
		t.Fatalf("大小写兜底失败：v=%d ok=%v", v2, ok2)
	}
	// 坏值 ⇒ 不采信
	if err := os.WriteFile(filepath.Join(dir, "Bad.yaml"), []byte("ctx_window: abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok3 := ProfileCtxWindow("Bad"); ok3 {
		t.Fatal("坏值不应被采信")
	}
}
