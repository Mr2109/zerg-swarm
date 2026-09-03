package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMemory_WriteRead — 写事实 + 读热记忆
func TestMemory_WriteRead(t *testing.T) {
	m := NewMemoryStore("test-agent", t.TempDir())

	id, err := m.WriteFact("用户喜欢中文回复", "偏好,语言")
	if err != nil {
		t.Fatalf("WriteFact 失败: %v", err)
	}
	if id == "" {
		t.Fatal("记忆 id 为空")
	}

	// 追加热记忆
	if err := m.AddToHot("铁律: 测试必须走虫族网关"); err != nil {
		t.Fatalf("AddToHot 失败: %v", err)
	}
	hot := m.ReadHot()
	if !strings.Contains(hot, "虫族网关") {
		t.Fatalf("热记忆内容缺失: %s", hot)
	}

	// 列出事实
	facts := m.ListFacts()
	if len(facts) != 1 {
		t.Fatalf("期望 1 个事实得到 %d", len(facts))
	}
}

// TestMemory_Search — BM25 式搜索
func TestMemory_Search(t *testing.T) {
	m := NewMemoryStore("test-agent2", t.TempDir())
	m.WriteFact("llama-server 必须带 -ngl 999 才用 GPU", "llama,gpu")
	m.WriteFact("音频同步用相机 TC 不用 WAV 时戳", "音频,同步")
	m.WriteFact("M3 gate 三态拦截工具", "gate,安全")

	// 搜 llama
	results := m.Search("llama gpu", 3)
	if len(results) == 0 {
		t.Fatal("搜索 llama 无结果")
	}
	found := false
	for _, r := range results {
		if strings.Contains(r, "ngl") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("搜索结果未含 llama 记忆: %v", results)
	}
}

// TestMemory_HotLimit — 热记忆超限截断
func TestMemory_HotLimit(t *testing.T) {
	m := NewMemoryStore("test-agent3", t.TempDir())
	big := strings.Repeat("x", 6000)
	if err := m.AddToHot(big); err != nil {
		t.Fatalf("AddToHot 失败: %v", err)
	}
	hot := m.ReadHot()
	if len(hot) > hotLimit {
		t.Fatalf("热记忆超限: %d > %d", len(hot), hotLimit)
	}
	_ = filepath.Join // 避免 unused
}
