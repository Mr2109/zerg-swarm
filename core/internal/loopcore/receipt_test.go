package loopcore

import "testing"

// 第①步契约：主张必须落到**真实回执原文**上；编造落 send_back（永久错误 ⇒ 不重试）。
func TestVerifyClaimsFourVerdicts(t *testing.T) {
	receipts := []Receipt{
		{ID: "17", Tool: "read", Output: "1098\t\tmatches = append(matches, fmt.Sprintf(\"%s:%d: %s\", rel, i+1, strings.TrimSpace(line)))"},
	}
	claims := []Claim{
		// ① 有据：片段确实来自回执原文（含排版差异也应命中）
		{Text: "我改了 TrimSpace 那一行", ReceiptID: "17",
			Quote: "matches = append(matches,\n fmt.Sprintf(\"%s:%d: %s\", rel, i+1, strings.TrimSpace(line)))"},
		// ② 编造：引用了回执 17，但片段根本不在原文里
		{Text: "我已把 Walk 改成 WalkDir", ReceiptID: "17", Quote: "filepath.WalkDir(root, func(path string, d fs.DirEntry"},
		// ③ 假回执：引用了不存在的回执
		{Text: "我读过设计稿", ReceiptID: "99", Quote: "G1 Walk 只跳 . 开头"},
		// ④ 缺引用：干脆没给依据
		{Text: "我已经更新了 tools/versions.json"},
		// ⑤ 有回执但没片段 ⇒ revise（不算无据，避免误杀）
		{Text: "我读过了", ReceiptID: "17"},
	}
	got := VerifyClaims(claims, receipts)
	want := []ClaimVerdict{VerdictKeep, VerdictSendBack, VerdictRetract, VerdictRevise, VerdictRevise}
	for i, w := range want {
		if got[i].Verdict != w {
			t.Errorf("第 %d 条：得到 %s，期望 %s（理由：%s）", i+1, got[i].Verdict, w, got[i].Reason)
		}
	}
	if got[1].Verdict != VerdictSendBack {
		t.Error("『编造片段』必须落 send_back —— 这是我们治『自己编』的核心判据")
	}
	if got[0].Reason == "" {
		t.Error("裁决必须带理由（可行动）")
	}
}

func TestCountByVerdict(t *testing.T) {
	got := VerifyClaims([]Claim{{Text: "a"}, {Text: "b"}}, nil)
	m := CountByVerdict(got)
	if m["revise"] != 2 {
		t.Errorf("汇总错误：%v", m)
	}
}
