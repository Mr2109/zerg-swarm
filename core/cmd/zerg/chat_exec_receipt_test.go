package main

import (
	"strings"
	"testing"
)

// TestAnnotateUnbackedClaims —— GAP-20261018-25 第二半的四态。
//
// 病灶（实测）：本地卵编造过 rc=0 / after_sha256 / hunk 数的完整假回执。
// 四态都要钉住，尤其 ③ —— 首版把「我不知道 after_sha256」也标成未取证（假阳性）。
func TestAnnotateUnbackedClaims(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantMark bool
	}{
		{"①报数字无凭据 ⇒ 标", "跑完了，rc=0，after_sha256=67b35f9d9b03184a，hunk 4 处改动", true},
		{"②带凭据 ⇒ 不标", "跑完了 [EXEC x-1a2b3c4d]，rc=0，after_sha256=67b35f9d9b03184a", false},
		{"③只提词不报值 ⇒ 不标（假阳性回归）", "我没有执行过，不知道 after_sha256，也不知道 rc", false},
		{"④纯文本无数字 ⇒ 不标", "这是对话的第一条消息，没有工具调用记录。", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := annotateUnbackedClaims(c.in)
			marked := strings.Contains(got, "未取证")
			if marked != c.wantMark {
				t.Fatalf("标不标不符：want=%v got=%v\n  输入：%s\n  输出：%s", c.wantMark, marked, c.in, got)
			}
			if !c.wantMark && got != c.in {
				t.Fatalf("不该改动正文，却被改了：%q", got)
			}
		})
	}
}
