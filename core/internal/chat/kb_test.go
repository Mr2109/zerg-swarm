package chat

import (
	"os"
	"strings"
	"testing"
)

// TestKBSearch — kb_search 工具执行（C6——知识库直查）
func TestKBSearch(t *testing.T) {
	// 2026-09-11 B 批：知识库路径改为可配置（ZERG_KB_PATH / <工作区>/data/knowledge.db）——
	// 未配置或文件不存在时跳过，而不是硬失败（外人的机器上通常没有这个库）
	if _, err := os.Stat(KBPath); err != nil {
		t.Skipf("知识库不可用（%s 不存在）——跳过", KBPath)
	}
	out, err := KbSearchExecute("虫族 经济", 5)
	if err != nil {
		t.Fatalf("KbSearchExecute 失败: %v", err)
	}
	t.Logf("结果: %s", out)
	if out == "" {
		t.Fatal("搜索结果为空")
	}
	if !strings.Contains(out, "[") {
		t.Fatal("结果格式异常（应含域标记）")
	}
}
