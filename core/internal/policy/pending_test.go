// pending_test.go — T5.8 实证：**长暂停**（持久化开放交互对象）与**逐类超时语义**。
//
// 五条用例（每条都带反例或对侧断言 —— 只断"该拒的拒了"会漏掉"把一切都拒了"这种假绿，
// 与 T5.1/T5.2/T5.3/T5.4 用例同一纪律）：
//
//	① 工具批准超时 ⇒ **判拒绝**（不是同意、不是继续）
//	② ask_user 超时 ⇒ 告知"无人应答"后继续（不涉及工具判定）
//	③ 交互点超时 ⇒ 按取消处理（`correct` 对非工具批准不成立）
//	④ List 可枚举未答项、Answer 后不可重答、反例不改任何东西
//	⑤ 序列化重启后仍能枚举（重启期间到期的挂单**不复活**）
//
// 纪律：now 一律**注入**（不用 time.Now）—— 时间由调用方给，判定才是纯函数、才可回放。
// 本包不碰文件系统、不 import 任何业务包。
package policy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// pendingT0 — 用例的固定时点（全部判定都以它为基准）。
func pendingT0() time.Time { return time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC) }

// pendingAt — 相对固定时点的偏移。
func pendingAt(d time.Duration) time.Time { return pendingT0().Add(d) }

// mustOpen — 开单（失败即 Fatal：这些前置开单不该失败）。
func mustOpen(t *testing.T, s PendingStore, p Pending, now time.Time) Pending {
	t.Helper()
	got, err := s.Open(p, now)
	if err != nil {
		t.Fatalf("Open(%s/%s) 不应失败：%v", p.Kind, p.Tool, err)
	}
	if got.ID == "" || got.Seq == 0 {
		t.Fatalf("开单后没有 id/序号（可枚举与可答复的前提）：%+v", got)
	}
	if got.State != StatusOpen {
		t.Fatalf("开单后状态必须是 open：%+v", got)
	}
	return got
}

// openIDs — 当前"未答项"枚举里的 id（保序）。
func openIDs(s PendingStore) []string {
	var out []string
	for _, p := range s.ListOpen() {
		out = append(out, p.ID)
	}
	return out
}

// ── ① 工具批准超时 ⇒ 判拒绝（**不是同意**）──────────────────────────────────

func TestPending_ToolApprovalTimeoutIsRejectNotConsent(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	p := mustOpen(t, s, Pending{
		Kind: PendingToolApproval, Tool: "bash", ArgsDigest: "sha256:2656ff0e00",
		SessionID: "sess-1", RuleID: "never.rm",
		ExpiresAt: pendingAt(5 * time.Minute),
	}, now)

	// 事实由账本写：CreatedAt 取注入的 now（不许调用方回填）、开单后进"未答项"枚举。
	if !p.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt 必须由开单时点定死：got %s want %s", p.CreatedAt, now)
	}
	if ids := openIDs(s); len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("刚开的挂单必须出现在「未答项」枚举里（F12）：%v", ids)
	}

	// 对侧：**没到点不许提前结算**（早一个纳秒也不行 —— 提前判定拒绝等于把闸多关了一道）。
	if out := s.Expire(p.ExpiresAt.Add(-time.Nanosecond)); len(out) != 0 {
		t.Fatalf("未到点不该结算：%+v", out)
	}
	if got, _ := s.Lookup(p.ID); got.State != StatusOpen {
		t.Fatalf("未到点状态不该变：%+v", got)
	}

	// 恰等于到期时刻 ⇒ 到点（边界取严，与 grants 的 TTL 同一口径）。
	out := s.Expire(p.ExpiresAt)
	if len(out) != 1 {
		t.Fatalf("到点必须结算恰好一条，得到 %d 条", len(out))
	}
	o := out[0]
	if o.Result != TimeoutReject {
		t.Errorf("工具批准超时的结局 = %q，必须是 %q（拒绝）", o.Result, TimeoutReject)
	}
	if o.Effect != EffectDeny {
		t.Errorf("工具批准超时对工具的判定 = %q，必须是 %q（deny）", o.Effect, EffectDeny)
	}
	// ★ 本批最重要的一条断言：超时**绝不被读作同意**（F12 明令）。
	if o.Effect == EffectAllow {
		t.Error("工具批准超时被读成了 allow ⇒ 把沉默当签字（「等了半天没人反对就做吧」）")
	}
	if o.Result == TimeoutContinue {
		t.Error("工具批准超时被读成了「继续」⇒ 没有批准也在执行")
	}
	if o.Answer != AnswerReject {
		t.Errorf("超时等价于的答复 = %q，必须是 reject", o.Answer)
	}
	if o.Reason != ReasonApprovalTimeout {
		t.Errorf("超时原因码 = %q，必须是 %q（低基数、可聚合）", o.Reason, ReasonApprovalTimeout)
	}
	if !strings.Contains(o.ModelNote, "拒绝") || !strings.Contains(o.Note, "绝不被读作同意") {
		t.Errorf("结算单必须把「拒绝」写明白：note=%q model_note=%q", o.Note, o.ModelNote)
	}

	// 落库事实：状态、结算时刻、依据都留痕。
	got, ok := s.Lookup(p.ID)
	if !ok {
		t.Fatal("结算后挂单不该从账本里消失（事实不消失）")
	}
	if got.State != StatusTimedOut || !got.SettledAt.Equal(p.ExpiresAt) {
		t.Errorf("超时结算没落库：state=%s settled=%s", got.State, got.SettledAt)
	}
	if got.RuleID != "never.rm" {
		t.Errorf("依据（rule_id）必须能回查：%q", got.RuleID)
	}
	// 幂等：重复结算不产生第二份结局，也不再出现在"未答项"里。
	if again := s.Expire(pendingAt(time.Hour)); len(again) != 0 {
		t.Errorf("重复 Expire 不该再结算：%+v", again)
	}
	if ids := openIDs(s); len(ids) != 0 {
		t.Errorf("已超时的挂单不该还在「未答项」里：%v", ids)
	}

	// ★ 迟到的批准不改变结局（超时那一刻生效的事实不可回溯）。
	if _, err := s.Answer(p.ID, AnswerApprove, pendingAt(6*time.Minute)); err == nil {
		t.Fatal("超时后迟到的 approve 竟被接受 ⇒ 「批准超时=拒绝」被翻案")
	} else if !strings.Contains(err.Error(), "超时") {
		t.Errorf("报错必须点明是超时（而不是含糊的「失败」）：%v", err)
	}
	after, _ := s.Lookup(p.ID)
	if after.State != StatusTimedOut || after.Answer != "" {
		t.Errorf("迟到的答复改动了记录：%+v", after)
	}
}

// ── ② ask_user 超时 ⇒ 告知"无人应答"后继续 ──────────────────────────────────

func TestPending_AskUserTimeoutContinuesWithNoAnswerNote(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	p := mustOpen(t, s, Pending{
		Kind: PendingAskUser, Tool: "ask_user", SessionID: "sess-1",
		ExpiresAt: pendingAt(2 * time.Minute), Note: "问用户要不要删临时目录",
	}, now)

	out := s.Expire(p.ExpiresAt)
	if len(out) != 1 {
		t.Fatalf("到点必须结算一条：%+v", out)
	}
	o := out[0]
	if o.Result != TimeoutContinue {
		t.Errorf("提问超时的结局 = %q，必须是 %q（继续）", o.Result, TimeoutContinue)
	}
	// **不涉及工具判定**：Effect 为空串（空串不是 allow —— 它表示"没有工具判定这回事"）。
	if o.Effect != "" {
		t.Errorf("提问超时不该产生工具判定：Effect=%q（既不能是 allow 也不能是 deny）", o.Effect)
	}
	if !strings.Contains(o.ModelNote, "无人应答") {
		t.Errorf("必须明写「无人应答」（模型只看到这一句）：%q", o.ModelNote)
	}
	if strings.Contains(o.ModelNote, "同意") || strings.Contains(o.ModelNote, "批准") {
		t.Errorf("「无人应答」不许被写成「得到批准」：%q", o.ModelNote)
	}
	// 对侧：这类超时既不拒绝也不取消（否则模型会以为工具被拒了）。
	if o.Result == TimeoutReject || o.Result == TimeoutCancel {
		t.Errorf("提问超时被读成了拒绝/取消：%q", o.Result)
	}

	got, _ := s.Lookup(p.ID)
	if got.State != StatusTimedOut {
		t.Errorf("提问超时也要落状态（否则接线批会反复结算）：%+v", got)
	}
	if got.Answer != "" {
		t.Errorf("「无人应答」不是一种答复：answer=%q", got.Answer)
	}
	if ids := openIDs(s); len(ids) != 0 {
		t.Errorf("已结算的挂单不该还在未答项里：%v", ids)
	}

	// 对侧：**不设期限**（零值 ExpiresAt）= 永不到期（人工会话允许无限等）。
	q := mustOpen(t, s, Pending{Kind: PendingAskUser, Tool: "ask_user", SessionID: "sess-1"}, now)
	if out := s.Expire(pendingAt(72 * time.Hour)); len(out) != 0 {
		t.Errorf("不设期限的挂单不该超时：%+v", out)
	}
	if gotQ, _ := s.Lookup(q.ID); gotQ.State != StatusOpen {
		t.Errorf("不设期限的挂单必须还等着人：%+v", gotQ)
	}

	// 反例：**工具批准**不设期限 ⇒ 拒绝开单（没人答的批准挂单会永久挂着 ⇒「超时=拒绝」永不生效）。
	if _, err := s.Open(Pending{Kind: PendingToolApproval, Tool: "bash"}, now); err == nil {
		t.Error("tool_approval 不设 ExpiresAt 竟被接受 ⇒ 批准会永久挂着（超时语义失效）")
	}
}

// ── ③ 交互点超时 ⇒ 按取消处理 ───────────────────────────────────────────────

func TestPending_InteractionTimeoutIsCancel(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	p := mustOpen(t, s, Pending{
		Kind: PendingInteraction, Tool: "node:ConfirmPlan", SessionID: "sess-1",
		ExpiresAt: pendingAt(time.Hour), Note: "计划确认点",
	}, now)
	// 交互点名与工具名同一套归一化（否则枚举面里出现两个"同一个点"）。
	if p.Tool != "node:confirmplan" {
		t.Errorf("交互点名必须归一化（小写、去首尾空白）：%q", p.Tool)
	}

	if out := s.Expire(pendingAt(59 * time.Minute)); len(out) != 0 {
		t.Fatalf("未到点不该结算：%+v", out)
	}
	out := s.Expire(p.ExpiresAt)
	if len(out) != 1 {
		t.Fatalf("到点必须结算一条：%+v", out)
	}
	o := out[0]
	if o.Result != TimeoutCancel {
		t.Errorf("交互点超时的结局 = %q，必须是 %q（取消）", o.Result, TimeoutCancel)
	}
	if o.Answer != AnswerCancel {
		t.Errorf("交互点超时等价于的答复 = %q，必须是 cancel", o.Answer)
	}
	if o.Effect != "" {
		t.Errorf("交互点超时不该产生工具判定：Effect=%q", o.Effect)
	}
	if !strings.Contains(o.ModelNote, "取消") || !strings.Contains(o.ModelNote, "作废") {
		t.Errorf("必须明写「取消/作废、不得当作已确认」：%q", o.ModelNote)
	}

	// `correct`（更正参数，F10）只对**工具批准**有意义：
	if q := mustOpen(t, s, Pending{Kind: PendingInteraction, Tool: "node:second", ExpiresAt: pendingAt(2 * time.Hour)}, now); q.ID != "" {
		if _, err := s.Answer(q.ID, AnswerCorrect, now); err == nil {
			t.Error("`correct` 用在交互点上竟被接受 ⇒ 更正一个「交互点」没有定义（不许猜语义）")
		}
	}
	r := mustOpen(t, s, Pending{Kind: PendingToolApproval, Tool: "bash", RuleID: "never.rm", ExpiresAt: pendingAt(2 * time.Hour)}, now)
	got, err := s.Answer(r.ID, AnswerCorrect, now)
	if err != nil {
		t.Fatalf("`correct` 用在工具批准上必须成立（F10 更正是改参数后继续）：%v", err)
	}
	if got.State != StatusAnswered || got.Answer != AnswerCorrect {
		t.Errorf("更正没落库：%+v", got)
	}

	// 三类结局两两不同（逐类语义的意义就在这里：任何两类被合并 ⇒ 这里红）。
	seen := map[TimeoutResult]PendingKind{}
	for _, k := range []PendingKind{PendingToolApproval, PendingAskUser, PendingInteraction} {
		got := timeoutOutcome(Pending{ID: "x#1", Kind: k, Tool: "t"})
		if prev, dup := seen[got.Result]; dup {
			t.Errorf("类别 %s 与 %s 得到同一个超时结局 %q ⇒ 逐类语义被合并了", k, prev, got.Result)
		}
		seen[got.Result] = k
	}
}

// ── ④ 可枚举未答项 + 不可重答 ───────────────────────────────────────────────

func TestPending_ListEnumeratesUnansweredAndAnswerIsOneShot(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	a := mustOpen(t, s, Pending{Kind: PendingToolApproval, Tool: "bash", ArgsDigest: "sha256:aa",
		RuleID: "never.rm", ExpiresAt: pendingAt(5 * time.Minute)}, now)
	b := mustOpen(t, s, Pending{Kind: PendingInteraction, Tool: "node:plan", SessionID: "sess-1",
		ExpiresAt: pendingAt(30 * time.Minute)}, now)

	if len(s.ListOpen()) != 2 || len(s.List()) != 2 {
		t.Fatalf("两个新挂单都该可枚举：open=%d all=%d", len(s.ListOpen()), len(s.List()))
	}
	if ids := openIDs(s); len(ids) != 2 || ids[0] != a.ID || ids[1] != b.ID {
		t.Errorf("未答项枚举必须保序（按开单顺序）：%v", ids)
	}

	got, err := s.Answer(a.ID, AnswerApprove, pendingAt(time.Minute))
	if err != nil {
		t.Fatalf("答复不应失败：%v", err)
	}
	if got.State != StatusAnswered || got.Answer != AnswerApprove || !got.AnsweredAt.Equal(pendingAt(time.Minute)) {
		t.Fatalf("答复没落库（含答复时刻）：%+v", got)
	}
	// 已答项从"未答"面消失，但**事实**仍在 List 里（用户与审计都要看得见）。
	if ids := openIDs(s); len(ids) != 1 || ids[0] != b.ID {
		t.Errorf("已答项不该还在未答项枚举里：%v", ids)
	}
	if len(s.List()) != 2 {
		t.Errorf("已答项必须留在 List（发生过的事实）：%d 条", len(s.List()))
	}

	// 不可重答（第二个人不许改第一个人的话）。
	if _, err := s.Answer(a.ID, AnswerReject, pendingAt(2*time.Minute)); err == nil {
		t.Fatal("已答的挂单竟可以被重答 ⇒ 第二个人能改第一个人的话")
	}
	after, _ := s.Lookup(a.ID)
	if after.State != StatusAnswered || after.Answer != AnswerApprove || !after.AnsweredAt.Equal(pendingAt(time.Minute)) {
		t.Errorf("重答被拒却改动了记录：%+v", after)
	}

	// 反例组：空 id / 未知 id / 非法答复 ⇒ 一律报错，且**不改任何东西**。
	for _, bad := range []struct {
		why string
		id  string
		dec Answer
	}{
		{"空 id", "   ", AnswerApprove},
		{"未知 id", "tool_approval:bash#999", AnswerApprove},
		{"非法答复", b.ID, Answer("maybe")},
	} {
		if _, err := s.Answer(bad.id, bad.dec, pendingAt(3*time.Minute)); err == nil {
			t.Errorf("%s：竟被接受（必须报错）", bad.why)
		}
	}
	if ids := openIDs(s); len(ids) != 1 || ids[0] != b.ID {
		t.Errorf("反例不该有副作用：%v", ids)
	}
	if gotB, _ := s.Lookup(b.ID); gotB.State != StatusOpen || gotB.Answer != "" {
		t.Errorf("反例改动了别的挂单：%+v", gotB)
	}

	// 返回的是副本：调用方改返回值不许污染账本。
	cp := s.List()[1]
	cp.Note = "被改了"
	if s.List()[1].Note == "被改了" {
		t.Error("List 返回的不是副本（调用方改返回值改到了账本内部）")
	}

	// 开单侧反例：无名挂单 / 已到点的挂单 / 调用方自填账本字段 / 类别非法 ⇒ 一律拒绝开单。
	for _, bad := range []struct {
		why string
		p   Pending
	}{
		{"工具名空白", Pending{Kind: PendingToolApproval, Tool: "  ", ExpiresAt: pendingAt(time.Minute)}},
		{"开单时点已到点", Pending{Kind: PendingInteraction, Tool: "n", ExpiresAt: now}},
		{"调用方自填 State", Pending{Kind: PendingInteraction, Tool: "n", State: StatusAnswered, ExpiresAt: pendingAt(time.Minute)}},
		{"调用方自填 ID", Pending{Kind: PendingInteraction, Tool: "n", ID: "伪造#1", ExpiresAt: pendingAt(time.Minute)}},
		{"类别认不出", Pending{Kind: PendingKind("nope"), Tool: "n"}},
	} {
		if _, err := s.Open(bad.p, now); err == nil {
			t.Errorf("%s：竟被接受（必须拒绝开单）", bad.why)
		}
	}
	if ids := openIDs(s); len(ids) != 1 {
		t.Errorf("开单侧反例不该留下挂单：%v", ids)
	}
}

// ── ⑤ 序列化重启后仍能枚举 ─────────────────────────────────────────────────

func TestPending_SurvivesRestartByEnumeration(t *testing.T) {
	now := pendingT0()
	s := NewMemoryPendingStore()
	a := mustOpen(t, s, Pending{Kind: PendingToolApproval, Tool: "bash", ArgsDigest: "sha256:bb",
		RuleID: "never.rm", SessionID: "sess-1", ExpiresAt: pendingAt(10 * time.Minute)}, now)
	b := mustOpen(t, s, Pending{Kind: PendingAskUser, Tool: "ask_user", SessionID: "sess-1"}, now) // 不设期限
	c := mustOpen(t, s, Pending{Kind: PendingInteraction, Tool: "node:plan", SessionID: "sess-1",
		ExpiresAt: pendingAt(time.Minute)}, now)
	if _, err := s.Answer(b.ID, AnswerApprove, pendingAt(30*time.Second)); err != nil {
		t.Fatalf("答复不应失败：%v", err)
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("账本序列化失败：%v", err)
	}

	// 重启：进程先退出（defer），稍后从持久化会话恢复（载入时点比 c 的到期时刻晚一分钟）。
	restarted, rep, err := LoadPending(data, pendingAt(2*time.Minute))
	if err != nil {
		t.Fatalf("挂单账本应能载入：%v", err)
	}
	if rep.Total != 3 || rep.Kept != 3 {
		t.Errorf("载入应保留全部事实（含已答/已超时）：%+v", rep)
	}
	if rep.SettledOnLoad != 1 {
		t.Errorf("重启期间到期的 open 项应当场结算成超时（不复活）：%+v", rep)
	}
	if rep.Open != 1 || rep.Answered != 1 || rep.TimedOut != 1 {
		t.Errorf("载入计数不对：%+v", rep)
	}

	// 可枚举未答项：重启后仍然问得清"还有哪些在等人"（F12 的一半证据）。
	ids := openIDs(restarted)
	if len(ids) != 1 || ids[0] != a.ID {
		t.Fatalf("重启后未答项枚举不对：%v（期望只剩 %s）", ids, a.ID)
	}
	if len(restarted.List()) != 3 {
		t.Errorf("重启后 List 应含全部三条：%d", len(restarted.List()))
	}
	// 已答的答复被完整继承（不是"当空账本重来"）。
	if got, ok := restarted.Lookup(b.ID); !ok || got.State != StatusAnswered || got.Answer != AnswerApprove {
		t.Errorf("重启后已答项没被继承：%+v ok=%v", got, ok)
	}
	if got, ok := restarted.Lookup(c.ID); !ok || got.State != StatusTimedOut || got.Answer != "" {
		t.Errorf("重启期间到期的项状态不对：%+v ok=%v", got, ok)
	}
	// ★ 不复活：重启期间到期的批准/交互点，迟到的批准照样被拒。
	if _, err := restarted.Answer(c.ID, AnswerApprove, pendingAt(3*time.Minute)); err == nil {
		t.Fatal("重启期间到期的挂单被复活 ⇒ 「超时」被一次重启抹掉")
	}
	// 未答项还能正常接着答（重启后挂单是可用的，不是只读的摆设）。
	if got, err := restarted.Answer(a.ID, AnswerApprove, pendingAt(3*time.Minute)); err != nil {
		t.Errorf("重启后未答项应能继续答复：%v", err)
	} else if got.State != StatusAnswered {
		t.Errorf("重启后答复没落库：%+v", got)
	}

	// 编号不撞车：接着开单的序号必须大于旧的最大序号，id 不与旧的重复。
	d := mustOpen(t, restarted, Pending{Kind: PendingInteraction, Tool: "node:next",
		ExpiresAt: pendingAt(time.Hour)}, pendingAt(4*time.Minute))
	if d.Seq <= c.Seq {
		t.Errorf("重启后序号回退（会与旧挂单撞 id）：新 %d 旧 %d", d.Seq, c.Seq)
	}
	for _, old := range []string{a.ID, b.ID, c.ID} {
		if d.ID == old {
			t.Errorf("重启后发出了与旧挂单相同的 id：%s", d.ID)
		}
	}

	// 坏输入一律**拒绝启用**（绝不按"空账本 / 坏账本"接着跑）。
	for _, bad := range []struct{ why, data string }{
		{"空串", ""},
		{"坏 JSON", "{"},
		{"版本不认识", `{"version":99,"next_seq":1,"pendings":[]}`},
		{"状态认不出", `{"version":1,"next_seq":2,"pendings":[{"id":"interaction:x#1","kind":"interaction","tool":"x","state":"maybe","seq":1}]}`},
		{"类别认不出", `{"version":1,"next_seq":2,"pendings":[{"id":"nope:x#1","kind":"nope","tool":"x","state":"open","seq":1}]}`},
		{"编号水位不自洽", `{"version":1,"next_seq":1,"pendings":[{"id":"interaction:x#1","kind":"interaction","tool":"x","state":"open","seq":1}]}`},
	} {
		if _, _, err := LoadPending([]byte(bad.data), pendingAt(2*time.Minute)); err == nil {
			t.Errorf("%s：载入竟成功（必须报错，绝不回退成空账本）", bad.why)
		}
	}
}
