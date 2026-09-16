package chat

import "testing"

// 交付轮窄判据：只切"以过程叙述开头的首段"，且其后必须还有正文；其余一律原样（防误伤）。
func TestSplitLeadingNarration(t *testing.T) {
	cases := []struct{ name, in, wantNarr, wantBody string }{
		{"过程开头+正文", "用户问的是时间字段有哪些。让我回顾一下：\n\n1. TS — 记录时刻\n2. Model — 模型名",
			"用户问的是时间字段有哪些。让我回顾一下：", "1. TS — 记录时刻\n2. Model — 模型名"},
		{"英文过程开头", "The user asked me to read the file. Let me summarize.\n\n它是观测面。",
			"The user asked me to read the file. Let me summarize.", "它是观测面。"},
		{"正文没有过程 ⇒ 原样", "它是对话链路的观测面，记录每轮事实。", "", "它是对话链路的观测面，记录每轮事实。"},
		{"只有过程没有正文 ⇒ 原样（不切空）", "用户问的是这个文件是干什么的。", "", "用户问的是这个文件是干什么的。"},
		{"真实第3轮（让我分析一下 + 列表）", "让我分析一下：\n\n1. `StartedAt` — 开始时间\n2. `FinishedAt` — 结束时间",
			"让我分析一下：", "1. `StartedAt` — 开始时间\n2. `FinishedAt` — 结束时间"},
		{"真实第1轮（从文件内容看： 后接续行）", "从文件内容看：\n1. 这是对话链路的观测面\n2. 定义观测结构",
			"从文件内容看：", "1. 这是对话链路的观测面\n2. 定义观测结构"},
		{"正文首行不是过程 则不动", "结论：该文件用于观测。\n补充说明见下。", "", "结论：该文件用于观测。\n补充说明见下。"},
		{"正文中段提到用户 ⇒ 不动", "结论：该文件用于观测。\n\n用户问问题时会被记录。", "", "结论：该文件用于观测。\n\n用户问问题时会被记录。"},
	}
	for _, c := range cases {
		n, b := SplitLeadingNarration(c.in)
		if n != c.wantNarr || b != c.wantBody {
			t.Errorf("[%s] 得到（%q, %q）期望（%q, %q）", c.name, n, b, c.wantNarr, c.wantBody)
		}
	}
}
