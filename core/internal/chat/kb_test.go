package chat

import (
	"strings"
	"testing"
)

// TestKBSearch — kb_search 工具执行（C6——知识库直查）
func TestKBSearch(t *testing.T) {
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
