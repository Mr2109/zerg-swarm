// sendback_code_test.go — B 项④-C 实证：**打回出口必须带原因码**（一个打回 = 一个主原因码）。
//
// 用例纪律（与 sendback_test.go / receipt_test.go 同一套）：裁决结果由**真实的**四态裁决算出来
// （`VerifyClaims`，一行没改），不手搓假结果 —— 否则「出口接上了原因码」这件事就没被真验过。
//
//	① ★ 缺原因码 ⇒ **拒绝发回**（不静默放行）：既不给记录，也不给打回指令（反例核心）
//	② 成功路径：4 条裁决里 1 条 send_back ⇒ 记录里**只有一个**主原因码，指令里也只出现一次
//	③ 没有 send_back / 没有裁决 ⇒ 拒绝发回（打回必须由判据事实支撑，不是由感觉支撑）
//	④ 码的三种坏形态（前缀认不出 / 缺 R 编号 / 缺建议）⇒ 一律拒绝
//	⑤ 零回归守卫：旧实验路径 `SendBackNote(reason)` 的行为**一字不变**（本批不改 run.go）
package loopcore

import (
	"errors"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/policy"
)

// codeFor — 用例用的原因码（POST_ = 执行者责任；R3 在 §3.4 是「criteria 三件套齐全」，
// 这里只用作**编号取值**的钉子，不代表本用例在判 R3）。
func codeFor(t *testing.T, resp policy.Responsibility, judge int, advice string) policy.SendBackCode {
	t.Helper()
	c, err := policy.NewSendBackCode(resp, judge, advice)
	if err != nil {
		t.Fatalf("构造用例原因码失败：%v", err)
	}
	return c
}

// realVerdicts — 用**真实的**四态裁决产出一批结果：keep 1 · retract 1 · send_back 1 · revise 1。
func realVerdicts() []ClaimVerdictResult {
	return VerifyClaims(
		[]Claim{
			{Text: "c1 有据", ReceiptID: "1", Quote: "命中的片段"},
			{Text: "c2 假回执", ReceiptID: "404", Quote: "什么都行"},
			{Text: "c3 无据", ReceiptID: "1", Quote: "对不上的片段"},
			{Text: "c4 缺引用"},
		},
		[]Receipt{{ID: "1", Tool: "read", Output: "这里含有命中的片段 原文"}},
	)
}

// ── ① ★ 缺原因码 ⇒ 拒绝发回（不得静默放行）──────────────────────────────────

func TestSendBack_RequiresReasonCode(t *testing.T) {
	req := SendBackRequest{SliceID: "S3-C1", Detail: "回执对不上", Verdicts: realVerdicts()}
	rec, err := SendBack(req) // Code 零值
	if err == nil {
		t.Fatal("★ 缺原因码必须**拒绝发回**，却成功了（静默放行）")
	}
	if rec != (SendBackRecord{}) {
		t.Errorf("拒绝时必须返回**零值**记录（调用方拿不到可误当打回成功的东西），得到 %+v", rec)
	}
	if !errors.Is(err, policy.ErrUnknownResponsibility) {
		t.Errorf("缺码应报前缀认不出（**不得默认成某一类**），得到 %v", err)
	}
	// 连零值记录的 Note() 也要拒绝：不能绕过出口去手搓一条打回指令
	if note, err := (SendBackRecord{}).Note(); err == nil || note != "" {
		t.Errorf("零值记录不得产出打回指令，得到 note=%q err=%v", note, err)
	}
	// 只有码、没有建议 ⇒ 同样发不出去（§4.7 格式类打回必须附最小修复提示）
	req.Code = policy.SendBackCode{Resp: policy.RespPostExec, R: 3}
	if _, err := SendBack(req); !errors.Is(err, policy.ErrMissingAdvice) {
		t.Errorf("缺最小修复提示必须拒绝发回，得到 %v", err)
	}
}

// ── ② 成功路径：只有一个主原因码 ─────────────────────────────────────────────

func TestSendBack_OnePrimaryCodeCarriedOnce(t *testing.T) {
	code := codeFor(t, policy.RespPostExec, 3, "把主张改写到有回执支撑：贴「回执ID + 原文片段」")
	rec, err := SendBack(SendBackRequest{
		SliceID:  "S3-C1",
		Code:     code,
		Detail:   "两条主张的回执对不上原文",
		Verdicts: realVerdicts(),
	})
	if err != nil {
		t.Fatalf("合法打回不应失败：%v", err)
	}
	if rec.Code != "POST_R3" || rec.Responsibility != "POST" || rec.Judge != 3 {
		t.Errorf("记录字段不对：%+v", rec)
	}
	if rec.SendBackCount != 1 || rec.VerdictCount != 4 {
		t.Errorf("打回条数/裁决条数必须成对落账（§4.9 成对口径），得到 %d/%d", rec.SendBackCount, rec.VerdictCount)
	}
	note, err := rec.Note()
	if err != nil {
		t.Fatalf("Note 不应失败：%v", err)
	}
	if !strings.Contains(note, "【打回重做】") || !strings.Contains(note, "POST_R3（执行者责任）") {
		t.Errorf("指令必须带可识别标记与责任前缀：%q", note)
	}
	if strings.Count(note, "POST_R3") != 1 {
		t.Errorf("★ 一个打回只能带**一个**主原因码（指令里应当只出现一次），得到 %q", note)
	}
	for _, other := range []string{"PRE_", "INV_"} {
		if strings.Contains(note, other) {
			t.Errorf("指令里不得混进第二个原因码（%s）：%q", other, note)
		}
	}
	if !strings.Contains(note, "回执") || !strings.Contains(note, "判据编号：R3") {
		t.Errorf("指令必须写明重交要求与判据编号：%q", note)
	}
}

// ── ③ 没有 send_back / 没有裁决 ⇒ 拒绝 ──────────────────────────────────────

func TestSendBack_RejectsWithoutSendBackVerdict(t *testing.T) {
	code := codeFor(t, policy.RespPostExec, 3, "补引用")
	// 全是 keep / retract / revise：没有任何一条构成打回
	keepOnly := []ClaimVerdictResult{
		{Claim: Claim{Text: "c1", ReceiptID: "1", Quote: "有据"}, Verdict: VerdictKeep},
		{Claim: Claim{Text: "c2"}, Verdict: VerdictRevise},
		{Claim: Claim{Text: "c3", ReceiptID: "1", Quote: "x"}, Verdict: VerdictRetract},
	}
	if _, err := SendBack(SendBackRequest{SliceID: "S3-C1", Code: code, Verdicts: keepOnly}); err == nil {
		t.Error("没有一条 send_back 时必须拒绝发回（无可发回项）")
	}
	if _, err := SendBack(SendBackRequest{SliceID: "S3-C1", Code: code}); err == nil {
		t.Error("没有裁决时必须拒绝发回（打回必须由判据事实支撑）")
	}
	if _, err := SendBack(SendBackRequest{Code: code, Verdicts: realVerdicts()}); err == nil {
		t.Error("缺 slice_id 时必须拒绝发回（无 id 的打回回指不到片）")
	}
	// 对侧：真的有一条 send_back 时必须成功（防「把一切都拒了」的假绿）
	if _, err := SendBack(SendBackRequest{SliceID: "S3-C1", Code: code, Verdicts: realVerdicts()}); err != nil {
		t.Errorf("有 send_back 时不应拒绝：%v", err)
	}
}

// ── ④ 码的三种坏形态一律拒（与 policy 侧的专项错误对齐）─────────────────────

func TestSendBack_RejectsBadCodes(t *testing.T) {
	base := SendBackRequest{SliceID: "S3-C1", Verdicts: realVerdicts(), Detail: "x"}
	cases := []struct {
		name string
		code policy.SendBackCode
		want error
	}{
		{"前缀认不出", policy.SendBackCode{Resp: "BOGUS", R: 3, Advice: "x"}, policy.ErrUnknownResponsibility},
		{"缺 R 编号", policy.SendBackCode{Resp: policy.RespInvEnv, R: -1, Advice: "x"}, policy.ErrMissingJudgeNumber},
		{"编号越界", policy.SendBackCode{Resp: policy.RespInvEnv, R: 14, Advice: "x"}, policy.ErrJudgeNumberOutOfRange},
		{"缺建议", policy.SendBackCode{Resp: policy.RespInvEnv, R: 3}, policy.ErrMissingAdvice},
	}
	for _, c := range cases {
		req := base
		req.Code = c.code
		rec, err := SendBack(req)
		if !errors.Is(err, c.want) {
			t.Errorf("%s：应报 %v，得到 %v", c.name, c.want, err)
		}
		if rec != (SendBackRecord{}) {
			t.Errorf("%s：拒绝时必须返回零值记录，得到 %+v", c.name, rec)
		}
	}
}

// ── ⑤ 零回归守卫：旧实验路径（terminal.go 的 SendBackNote）一字不变 ──────────

func TestSendBack_LegacyExperimentPathUnchanged(t *testing.T) {
	// 旧路径是 ZERG_SENDBACK 开关的产物（默认关；run.go 一行未动）——它**没有判据事实**，
	// 故本批**不给它配码**（配码 = 编一个责任归属 ✗），只保证它没被这次改动影响。
	t.Setenv("ZERG_SENDBACK", "")
	if SendBackEnabled() {
		t.Error("默认必须为关（不得改变线上行为）")
	}
	legacy := SendBackNote("仍有未完成的工作")
	if !strings.Contains(legacy, "【打回重做】") || !strings.Contains(legacy, "回执") {
		t.Errorf("旧路径行为必须一字不变：%q", legacy)
	}
	// 两条路径并存但不重叠：新出口带原因码，旧路径不带（登记为未接线项，见回报）
	if strings.Contains(legacy, "R3") || strings.Contains(legacy, "PRE_") {
		t.Errorf("旧路径不得凭空出现原因码：%q", legacy)
	}
}
