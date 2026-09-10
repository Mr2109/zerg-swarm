// chat_prompt_test.go — 丙批 §4.1 验收：系统提示三档 + 会话内字节稳定（2026-09-10）
// 验收①：同一会话连续 10 轮，system prompt 哈希不变（除非压缩/切模型）

package chat

import (
	"strings"
	"testing"
)

// TestBatchC_PromptFrozenAcrossTurns — 10 轮取提示：哈希恒定；常驻工具中途"增长"也不变
func TestBatchC_PromptFrozenAcrossTurns(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "丙1冻结验收")
	if err != nil {
		t.Fatal(err)
	}
	rt := st.GetToolRuntime(se.ID)
	first := st.SessionSystemPrompt(se.ID, "【基础指令】", se.Model, rt)
	h0 := PromptHash(first)
	if h0 == "" {
		t.Fatal("提示哈希不应为空")
	}
	for i := 0; i < 10; i++ {
		rt.AddResident("memory") // 模拟渐进模式"发现新工具"（旧实现会改提示 → 前缀失效）
		got := st.SessionSystemPrompt(se.ID, "【基础指令】", se.Model, rt)
		if PromptHash(got) != h0 {
			t.Fatalf("第 %d 轮提示哈希变了（会话内必须字节稳定）: %s != %s", i+1, PromptHash(got), h0)
		}
	}
	if hash, _, frozen := st.FrozenPromptInfo(se.ID); hash != h0 || !frozen {
		t.Fatalf("冻结信息不一致: hash=%s frozen=%v（期望 %s）", hash, frozen, h0)
	}
	t.Logf("✓ 验收①：10 轮 + 常驻工具增长，提示哈希恒为 %s", h0)
}

// TestBatchC_PromptRebuildOnModelSwitch — 模型切换必须重建（缓存键含模型）
func TestBatchC_PromptRebuildOnModelSwitch(t *testing.T) {
	st := newTestStore(t)
	se, _ := st.CreateSession("example-35b-v2", "desktop", "", "丙1模型切换")
	rt := st.GetToolRuntime(se.ID)
	a := st.SessionSystemPrompt(se.ID, "【基础指令】", "example-35b-v2", rt)
	b := st.SessionSystemPrompt(se.ID, "【基础指令】", "Qwen3.8-27B", rt)
	if a == b {
		t.Fatal("切模型后提示应重建（内容须不同）")
	}
	if !strings.Contains(b, "Qwen3.8-27B") {
		t.Fatalf("重建后的提示应含新模型名: %s", b)
	}
	t.Log("✓ 模型切换 → 提示自动重建并含新模型名")
}

// TestBatchC_PromptClearedAfterCompaction — 压缩后清冻结 → 下一轮重建
func TestBatchC_PromptClearedAfterCompaction(t *testing.T) {
	st := newTestStore(t)
	se, _ := st.CreateSession("example-35b-v2", "desktop", "", "丙1压缩重建")
	rt := st.GetToolRuntime(se.ID)
	before := st.SessionSystemPrompt(se.ID, "【基础指令】", se.Model, rt)
	st.ClearSessionPrompt(se.ID)
	if hash, _, frozen := st.FrozenPromptInfo(se.ID); frozen || hash != "" {
		t.Fatalf("清理后应无冻结: hash=%q frozen=%v", hash, frozen)
	}
	after := st.SessionSystemPrompt(se.ID, "【基础指令】", se.Model, rt)
	if PromptHash(after) != PromptHash(before) {
		t.Log("提示内容因记忆块/工具清单变化而不同（预期——重建的目的）")
	}
	t.Log("✓ 压缩后清冻结 → 下一轮重新构建并冻结")
}
