package api

import (
	"strings"
	"testing"
)

// TestAnnotateDegenerateAnswer —— 长任务末段退化标注的三态（GAP-20260926-214）。
func TestAnnotateDegenerateAnswer(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		toolCalls int
		wantAnnot bool
	}{
		{"退化：跑过 3 次工具却只回 pong", "pong! 🏓", 5, true},
		{"退化：跑过 9 次工具只回『好的』", "好的", 9, true},
		{"不标：没有工具足迹（本来就短）", "pong! 🏓", 0, false},
		{"不标：工具少（2 次）", "好的", 2, false},
		{"不标：答复够长（像结论）", "全部三条命令 rc=0，门禁 42 PASS / 0 FAIL，已提交 b752771b8。", 7, false},
		{"不标：空答复（另一条路处理）", "   ", 6, false},
	}
	for _, c := range cases {
		got := annotateDegenerateAnswer(c.content, c.toolCalls)
		annotated := got != c.content
		if annotated != c.wantAnnot {
			t.Fatalf("%s：期望标注=%v 实得=%v（%q）", c.name, c.wantAnnot, annotated, got)
		}
		if c.wantAnnot {
			if !strings.Contains(got, "疑似退化输出") || !strings.Contains(got, "GAP-20260926-214") {
				t.Fatalf("%s：标注缺少关键要素：%q", c.name, got)
			}
		}
	}
}

// TestAnnotateDegenerateKeepsOriginal —— 标注是**追加**，原有正文逐字保留（只标不改）。
func TestAnnotateDegenerateKeepsOriginal(t *testing.T) {
	in := "pong! 🏓"
	got := annotateDegenerateAnswer(in, 4)
	if !strings.HasPrefix(got, in) {
		t.Fatalf("原正文被改动：%q", got)
	}
}
