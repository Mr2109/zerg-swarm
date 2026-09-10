package gateway

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// ========== v2.5.6 路由打分修复测试（Mr2109 2026-08-28——反复挂起根因） ==========

// TestModelFileLoaded_LocalFileName 关键修复验证:
// local 快照 Model="Qwen3.8-27B-Q4_K_M-vcruz305"（文件名形式）
// 候选 File=".../Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
// → 应匹配（之前精确相等不匹配——local 拿不到 +8——x3 恒赢——反复挂起死循环）
func TestModelFileLoaded_LocalFileName(t *testing.T) {
	snap := &store.FleetSnapshot{
		Model: strPtr("Qwen3.8-27B-Q4_K_M-vcruz305"),
	}
	file := "~/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
	if !modelFileLoaded(snap, file) {
		t.Fatalf("文件名形式快照应匹配候选 File（修复核心——之前不匹配）")
	}
}

// TestModelFileLoaded_LogicalName x3 快照 Model="Qwen3.8-27B"（逻辑名）——与 File basename 不同——也应匹配
func TestModelFileLoaded_LogicalName(t *testing.T) {
	snap := &store.FleetSnapshot{
		Model: strPtr("Qwen3.8-27B"),
	}
	file := "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
	if !modelFileLoaded(snap, file) {
		t.Fatalf("逻辑名快照应匹配（basename 匹配）")
	}
}

// TestModelFileLoaded_ModelsList 快照 Models 列表含候选 → 匹配
func TestModelFileLoaded_ModelsList(t *testing.T) {
	snap := &store.FleetSnapshot{
		Models: []string{"example-35b-v2", "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"},
	}
	file := "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
	if !modelFileLoaded(snap, file) {
		t.Fatalf("Models 列表含候选文件应匹配")
	}
}

// TestModelFileLoaded_Negative 未加载该模型 → 不匹配
func TestModelFileLoaded_Negative(t *testing.T) {
	snap := &store.FleetSnapshot{
		Model: strPtr("example-35b"),
	}
	file := "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
	if modelFileLoaded(snap, file) {
		t.Fatalf("未加载该模型不应匹配")
	}
	// nil 快照
	if modelFileLoaded(nil, file) {
		t.Fatalf("nil 快照不应匹配")
	}
	// 空文件
	if modelFileLoaded(snap, "") {
		t.Fatalf("空候选文件不应匹配")
	}
}

func strPtr(s string) *string { return &s }
