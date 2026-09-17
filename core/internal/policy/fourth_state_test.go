// fourth_state_test.go — B 项④-B 实证：**第 4 态「追问/阻滞」**（信息不足 ⇒ 退回上游补规格）。
//
// 用例纪律（与 pending_test.go 同一套）：`now` 一律**注入**（不看系统时间）；
// 每条都带对侧断言（既断「阻滞时不放行」，也断「上游补了规格后必须放行」——只断前者会漏掉"把一切都卡住"的假红）。
//
//	① 映射：第 4 态挂在 `interaction` 类挂单上（并显式断它不是 ask_user / tool_approval）
//	② 缺件（片 id / 上游 / 原因码 / 期限）⇒ **不开单**（fail-closed：宁可不进态）
//	③ 未答 ⇒ awaiting，**不放行**；上游补规格（approve）⇒ supplied，**放行**
//	④ 上游拒答（reject）⇒ 升级给人，**不放行**；答复 cancel ⇒ 作废，**不放行**
//	⑤ ★ 超时 ⇒ 按**现有逐类语义**结算（interaction = cancel）且**不得静默放行**（反例核心）
//	⑥ 认不出的一律报错：拿 tool_approval 挂单来结算 / 挂单的码回解不出
package policy

import (
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// fourthCode — 用例用的原因码（PRE_ = 切片者/输入可见集责任：R5 = 「未决必须写成 NEEDS-CLARIFICATION」）。
func fourthCode(t *testing.T) SendBackCode {
	t.Helper()
	c, err := NewSendBackCode(RespPreSlice, 5, "把未决项写成 NEEDS-CLARIFICATION 并补上落点片段")
	if err != nil {
		t.Fatalf("构造用例原因码失败：%v", err)
	}
	return c
}

// fourthReq — 用例用的第 4 态开单请求（缺件由各用例自己改坏）。
func fourthReq(t *testing.T) ClarificationRequest {
	t.Helper()
	return ClarificationRequest{
		SliceID:  "S3-C1",
		Upstream: "lead（切片者）",
		Code:     fourthCode(t),
		RuleID:   "check-slice:R5",
		Deadline: pendingAt(10 * time.Minute),
	}
}

// ── ① 映射：挂在 interaction 上 ─────────────────────────────────────────────

func TestFourthState_MapsToInteractionPending(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	out, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("RaiseClarification 不应失败：%v", err)
	}
	if out.Kind != PendingInteraction {
		t.Fatalf("第 4 态必须挂在 interaction 上，得到 %s", string(out.Kind))
	}
	// 为什么不是另两类（写进断言：改映射是改语义，必须由用例逼着改）
	if out.Kind == PendingAskUser {
		t.Error("不得挂在 ask_user 上：它的超时语义是 **continue** ⇒ 对阻滞而言就是超时即静默放行")
	}
	if out.Kind == PendingToolApproval {
		t.Error("不得挂在 tool_approval 上：它批的是**工具执行**，超时给的是 Effect=deny ⇒ 会污染工具判定面")
	}
	if FourthStateKind != PendingInteraction {
		t.Errorf("FourthStateKind 常量必须是 interaction，得到 %s", string(FourthStateKind))
	}
	// 进态本身**不放行任何东西**
	if out.AllowedToProceed() || out.Disposition != FourthAwaiting {
		t.Errorf("进态必须是 awaiting 且不放行，得到 %s / allowed=%v", string(out.Disposition), out.Allowed)
	}
	// 挂单事实：open、有期限、码落在 RuleID 上（结算侧的唯一真源）
	p, ok := s.Lookup(out.PendingID)
	if !ok {
		t.Fatalf("挂单必须能按 id 查回：%s", out.PendingID)
	}
	if p.State != StatusOpen {
		t.Errorf("开单后必须 open，得到 %s", string(p.State))
	}
	if p.ExpiresAt.IsZero() {
		t.Error("第 4 态挂单**必须有期限**（§4.7 禁无限等待）")
	}
	if p.RuleID != "PRE_R5" {
		t.Errorf("挂单 RuleID 应承载原因码的码部分（PRE_R5），得到 %q", p.RuleID)
	}
	if !strings.Contains(strings.ToLower(p.Tool), "s3-c1") {
		t.Errorf("交互点名必须回指到片（clarify:<slice>），得到 %q", p.Tool)
	}
	// 挂单侧会把交互点名归一为小写（pending.go 的既有工具名纪律，本批不改它）⇒
	// **片 id 的原始大小写以 Note 标记为准**（否则 `S3-C1` 会变成 `s3-c1`，回指不到合同里的那个 id）
	if !strings.Contains(p.Note, "片 S3-C1；") {
		t.Errorf("挂单 Note 必须按写死的格式带上片 id 的原文：%q", p.Note)
	}
	if len(s.ListOpen()) != 1 {
		t.Errorf("未答项必须可枚举（F12），得到 %d 条", len(s.ListOpen()))
	}
	if out.Upstream == "" || !strings.Contains(out.ModelNote, "不要硬猜") {
		t.Errorf("出态必须点明退给谁并写明禁硬猜：upstream=%q note=%q", out.Upstream, out.ModelNote)
	}
}

// ── ② 缺件 ⇒ 不开单 ─────────────────────────────────────────────────────────

func TestFourthState_RequiresSliceUpstreamCodeAndDeadline(t *testing.T) {
	now := pendingT0()
	cases := []struct {
		name   string
		mutate func(*ClarificationRequest)
	}{
		{"缺 SliceID", func(r *ClarificationRequest) { r.SliceID = " " }},
		{"缺 Upstream", func(r *ClarificationRequest) { r.Upstream = "" }},
		{"缺原因码", func(r *ClarificationRequest) { r.Code = SendBackCode{} }},
		{"原因码缺建议", func(r *ClarificationRequest) {
			r.Code = SendBackCode{Resp: RespPreSlice, R: 5} // 解析得出、但发不出去（§4.7 缺最小修复提示）
		}},
		{"缺 Deadline（禁无限等待）", func(r *ClarificationRequest) { r.Deadline = time.Time{} }},
	}
	for _, c := range cases {
		s := NewMemoryPendingStore()
		req := fourthReq(t)
		c.mutate(&req)
		out, err := RaiseClarification(s, req, now)
		if err == nil {
			t.Errorf("%s：必须报错（fail-closed），却成功了：%+v", c.name, out)
			continue
		}
		if out != (FourthStateOutcome{}) {
			t.Errorf("%s：报错时必须返回**零值** outcome（调用方拿不到可误当裁决的东西），得到 %+v", c.name, out)
		}
		if len(s.ListOpen()) != 0 {
			t.Errorf("%s：报错时**不得留下挂单**，却有 %d 条未答", c.name, len(s.ListOpen()))
		}
	}
	// 对侧：账本为空 ⇒ 报错（不静默跳过 = 不假装进了第 4 态）
	if _, err := RaiseClarification(nil, fourthReq(t), now); err == nil {
		t.Error("账本为空时必须报错")
	}
}

// ── ③ 未答 / 上游补了规格 ───────────────────────────────────────────────────

func TestFourthState_OpenThenApproveUnblocks(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	raised, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("开单失败：%v", err)
	}
	// 未到点、未答复 ⇒ awaiting 且不放行
	open, err := SettleClarification(s, raised.PendingID, pendingAt(time.Minute))
	if err != nil {
		t.Fatalf("未答结算不应报错：%v", err)
	}
	if open.Disposition != FourthAwaiting || open.AllowedToProceed() {
		t.Errorf("未答必须停在 awaiting 且不放行，得到 %s / allowed=%v", string(open.Disposition), open.Allowed)
	}
	// 上游真的补了规格（approve）⇒ 唯一的放行路径
	if _, err := s.Answer(raised.PendingID, AnswerApprove, pendingAt(2*time.Minute)); err != nil {
		t.Fatalf("答复 approve 不应失败：%v", err)
	}
	done, err := SettleClarification(s, raised.PendingID, pendingAt(3*time.Minute))
	if err != nil {
		t.Fatalf("已答结算不应报错：%v", err)
	}
	if done.Disposition != FourthSupplied || !done.AllowedToProceed() {
		t.Errorf("上游补规格后必须放行（supplied），得到 %s / allowed=%v", string(done.Disposition), done.Allowed)
	}
	if done.Code != fourthCode(t) {
		t.Errorf("结算必须回带上完整原因码（码来自 RuleID、建议来自 Note 标记 ⇒ 往返一致），得到 %+v", done.Code)
	}
	if done.SliceID != "S3-C1" {
		t.Errorf("结算必须回带上片 id 的原文（S3-C1），得到 %q", done.SliceID)
	}
	if !strings.Contains(done.ModelNote, "重判") {
		t.Errorf("放行的 ModelNote 必须写明「按新规格重判」：%q", done.ModelNote)
	}
}

// ── ④ 上游拒答 / 作废 ⇒ 都不放行 ────────────────────────────────────────────

func TestFourthState_RejectEscalatesAndCancelVoids(t *testing.T) {
	now := pendingT0()
	cases := []struct {
		answer Answer
		want   FourthDisposition
	}{
		{AnswerReject, FourthEscalate},
		{AnswerCancel, FourthVoid},
	}
	for _, c := range cases {
		s := NewMemoryPendingStore()
		raised, err := RaiseClarification(s, fourthReq(t), now)
		if err != nil {
			t.Fatalf("开单失败：%v", err)
		}
		if _, err := s.Answer(raised.PendingID, c.answer, pendingAt(time.Minute)); err != nil {
			t.Fatalf("答复 %s 不应失败：%v", string(c.answer), err)
		}
		out, err := SettleClarification(s, raised.PendingID, pendingAt(2*time.Minute))
		if err != nil {
			t.Fatalf("结算不应报错：%v", err)
		}
		if out.Disposition != c.want {
			t.Errorf("答复 %s 的处置应为 %s，得到 %s", string(c.answer), string(c.want), string(out.Disposition))
		}
		if out.AllowedToProceed() {
			t.Errorf("答复 %s **不得放行**（缺信息硬猜 = 假绿）", string(c.answer))
		}
	}
}

// ── ⑤ ★ 超时 ⇒ 按现有逐类语义结算 + **不得静默放行**（反例核心）──────────────

func TestFourthState_TimeoutIsFailClosedViaExistingSemantics(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	raised, err := RaiseClarification(s, fourthReq(t), now)
	if err != nil {
		t.Fatalf("开单失败：%v", err)
	}
	// 恰等于到期时刻即算超时（沿用 pending.go 的边界取严：`!now.Before(ExpiresAt)`）
	out, err := SettleClarification(s, raised.PendingID, pendingAt(10*time.Minute))
	if err != nil {
		t.Fatalf("到点结算不应报错：%v", err)
	}
	if !out.TimedOut {
		t.Fatal("恰等于到期时刻必须算超时（边界取严）")
	}
	if out.Disposition != FourthVoid || out.AllowedToProceed() {
		t.Errorf("★ 超时必须作废且**不得静默放行**，得到 %s / allowed=%v", string(out.Disposition), out.Allowed)
	}
	if out.LedgerState != StatusTimedOut {
		t.Errorf("超时必须在账本上结算成 timed_out，得到 %s", string(out.LedgerState))
	}
	if !strings.Contains(out.ModelNote, "不得当作已确认") || !strings.Contains(out.ModelNote, "不得自行解除阻滞") {
		t.Errorf("超时的 ModelNote 必须写明「作废 + 不得自行解除阻滞」：%q", out.ModelNote)
	}
	// 超时语义**不是本文件自己的**：与「一张普通 interaction 挂单走 Expire」的结局逐字一致
	ref := NewMemoryPendingStore()
	plain := mustOpen(t, ref, Pending{
		Kind:      PendingInteraction,
		Tool:      "chat:wait#1",
		ExpiresAt: pendingAt(10 * time.Minute),
	}, now)
	refOut := ref.Expire(pendingAt(10 * time.Minute))
	if len(refOut) != 1 || refOut[0].PendingID != plain.ID {
		t.Fatalf("参照挂单应被结算一次，得到 %+v", refOut)
	}
	if out.TimeoutResult != refOut[0].Result {
		t.Errorf("第 4 态的超时结局必须与既有逐类语义一致：本态 %s vs 既有 %s",
			string(out.TimeoutResult), string(refOut[0].Result))
	}
	if out.TimeoutResult != TimeoutCancel {
		t.Errorf("interaction 的逐类结局应为 cancel，得到 %s", string(out.TimeoutResult))
	}
	if out.TimeoutEffect != "" {
		t.Errorf("interaction 不涉及工具判定 ⇒ Effect 必须为空串（空串不是 allow），得到 %q", string(out.TimeoutEffect))
	}
	// 幂等 + 不复活：再结算一次仍是 void，再扫一次不会出现第二份结局
	again, err := SettleClarification(s, raised.PendingID, pendingAt(20*time.Minute))
	if err != nil {
		t.Fatalf("重复结算不应报错（幂等）：%v", err)
	}
	if again.Disposition != FourthVoid || again.AllowedToProceed() {
		t.Errorf("重复结算必须仍是 void 且不放行，得到 %s / allowed=%v", string(again.Disposition), again.Allowed)
	}
	if got := s.Expire(pendingAt(30 * time.Minute)); len(got) != 0 {
		t.Errorf("已结算的挂单不得再产出结局（幂等），得到 %+v", got)
	}
	if n := len(s.ListOpen()); n != 0 {
		t.Errorf("超时后不得仍在「未答」枚举里（不复活成还能答），得到 %d 条", n)
	}
	// 迟到答复不改变结局（沿用 pending.go ②）：超时后答复一律被拒
	if _, err := s.Answer(raised.PendingID, AnswerApprove, pendingAt(31*time.Minute)); err == nil {
		t.Error("超时后的迟到答复必须被拒（迟到的答复不改变结局）")
	}
}

// ── ⑥ 认不出的一律报错（不猜语义）──────────────────────────────────────────

func TestFourthState_RejectsForeignOrUnparseablePendings(t *testing.T) {
	now := pendingT0()

	// (a) 拿一张 tool_approval 挂单来结算 ⇒ 报错（它的 approve 是「放行工具」，不是「补了规格」）
	s := NewMemoryPendingStore()
	appr := mustOpen(t, s, Pending{
		Kind: PendingToolApproval, Tool: "bash", ExpiresAt: pendingAt(10 * time.Minute),
	}, now)
	if _, err := SettleClarification(s, appr.ID, now); err == nil {
		t.Error("类别不是第 4 态的挂单必须报错（不猜语义）")
	}

	// (b) 挂单的 RuleID 回解不出原因码 ⇒ 报错（这张挂单不是本态开的）
	s2 := NewMemoryPendingStore()
	foreign := mustOpen(t, s2, Pending{
		Kind: PendingInteraction, Tool: "clarify:X", RuleID: "不是原因码", ExpiresAt: pendingAt(10 * time.Minute),
	}, now)
	if _, err := SettleClarification(s2, foreign.ID, now); err == nil {
		t.Error("RuleID 回解不出原因码必须报错")
	}

	// (c) 空 id / 没有这张挂单 / 空账本 ⇒ 报错（都不许被当成「已答复」）
	if _, err := SettleClarification(s2, "  ", now); err == nil {
		t.Error("空 id 必须报错")
	}
	if _, err := SettleClarification(s2, "interaction:clarify:X#99", now); err == nil {
		t.Error("不存在的挂单必须报错")
	}
	if _, err := SettleClarification(nil, "x", now); err == nil {
		t.Error("空账本必须报错")
	}
}
