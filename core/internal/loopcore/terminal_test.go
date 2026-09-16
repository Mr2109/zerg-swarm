package loopcore

import "testing"

// ⑤ 四终局：预算/失败到尽头时，必须落四种之一（行业口径），且理由可行动。
func TestChooseTerminalFourStates(t *testing.T) {
	cases := []struct {
		name string
		in   TerminalInput
		want TerminalState
	}{
		{"已发生不可逆副作用 ⇒ 带补偿中止（优先，先保证可回滚）",
			TerminalInput{CompletedSteps: 3, HasIrreversible: true, HasRemainingWork: true}, TerminalAbortCompensated},
		{"有进展且仍有可完成部分 ⇒ 降级（做完能做的 + 标注未覆盖）",
			TerminalInput{CompletedSteps: 2, HasRemainingWork: true}, TerminalDegrade},
		{"有进展且可落检查点 ⇒ 保存状态续跑",
			TerminalInput{CompletedSteps: 1, Checkpointable: true}, TerminalResume},
		{"卡在需要人的判断 ⇒ 升级给人",
			TerminalInput{CompletedSteps: 1, NeedsHumanJudgment: true}, TerminalEscalate},
		{"零进展 ⇒ 也必须升级给人（绝不停在半路不吭声）",
			TerminalInput{}, TerminalEscalate},
	}
	for _, c := range cases {
		got, reason := ChooseTerminal(c.in)
		if got != c.want {
			t.Errorf("%s：得到 %s，期望 %s", c.name, got, c.want)
		}
		if reason == "" {
			t.Errorf("%s：必须给出理由（可行动）", c.name)
		}
	}
}

func TestTerminalNote(t *testing.T) {
	if got := TerminalNote(TerminalDegrade, "已升级 grep 的 G1/G3，G5 未做"); got == "" {
		t.Error("终局说明不得为空")
	}
	if got := TerminalNote(TerminalState("未知"), ""); got != "【已结束】" {
		t.Errorf("未知终局应有兜底说明，实际 %q", got)
	}
}
