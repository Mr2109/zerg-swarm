package chat

import "testing"

// 事故背景：历史 assistant 只带 content（思考混在里面）、不带 reasoning_content
// ⇒ Qwen3 官方模板整段当正文 ⇒ 模型照抄口吻 ⇒ 正文污染。以下用例守住"分开"这条铁律。
func TestSplitReasoningPairsAndUnclosed(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantReason  string
		wantContent string
	}{
		{"成对标签", "<think>我在想</think>这是正文", "我在想", "这是正文"},
		{"未闭合在开头", "<think>还在想…这是正文", "还在想…这是正文", ""},
		{"中段字面标签不吃后文", "正文说 <think> 是个标签，后面还有字", "", "正文说 <think> 是个标签，后面还有字"},
		{"无标签", "只有正文", "", "只有正文"},
	}
	for _, c := range cases {
		gotR, gotC := splitReasoning(c.content)
		if gotR != c.wantReason || gotC != c.wantContent {
			t.Errorf("[%s] 得到（%q, %q）期望（%q, %q）", c.name, gotR, gotC, c.wantReason, c.wantContent)
		}
	}
}

func TestSplitReasoningForHistoryKeepsThemSeparate(t *testing.T) {
	// 库里已分栏 ⇒ 原样分开（正文不得含思考）
	r, c := SplitReasoningForHistory("这是正文", "我在想")
	if r != "我在想" || c != "这是正文" {
		t.Fatalf("分栏历史被破坏：（%q, %q）", r, c)
	}
	// 历史脏数据：思考被同时写进 content 前缀 ⇒ 必须剥掉，正文干净
	r2, c2 := SplitReasoningForHistory("我在想这是正文", "我在想")
	if r2 != "我在想" || c2 != "这是正文" {
		t.Fatalf("脏数据前缀未剥净：（%q, %q）", r2, c2)
	}
}
