// human_decision_test.go — T5.7 实证：人的五态 ⇒ 给模型的结果（**语义边界硬守 · 更正留痕 · 全程可审计**）。
//
// 九条用例（含两处**反例**；每条都带对侧断言 —— 只断「该拒的拒了」会漏掉「把一切都拒了」这种假绿，
// 与 T5.1/T5.3/T5.4/T5.8 用例同一纪律）：
//
//	① respond  ⇒ 成功型结果且**无任何拒绝/失败字样**（正文 = 人给的内容原样）；工具**不执行**
//	② reject   ⇒ **未执行** + **不要重试** + 为什么（落在有副作用的工具上 —— 这正是它的主用途）
//	③ correct  ⇒ 参数被替换、**原文指纹保留**、`EditedBy` 留痕、F10 告警在场；放行执行
//	④ escalate ⇒ 返回**专项错误**（`errors.Is(ErrEscalated)`）+ 上交标记，**不产出可喂模型的结果**
//	⑤ 反例：`respond` 用在**有副作用**的工具上 ⇒ 报错（`ErrRespondOnSideEffect`），且不产出任何东西
//	⑥ 反例：**未知决定** ⇒ 报错（`ErrUnknownDecisionKind`，fail-closed）
//	⑦ 反例：**副作用事实缺省** ⇒ 按「有」处理（respond 被拒；对侧：同样缺省下 reject 照样可用）
//	⑧ 反例：**替人签字 / 批错对象**（未答复 · 指纹不符 · 已超时结算 · 账本答复不同族）⇒ 逐条报错
//	⑨ 更正的结构化形态：序列化出来就是 F10 的 `{"type":"edit","edited_action":{…}}`，且能原样读回
//
// 纪律：挂单一律用**真账本**（`MemoryPendingStore` 的 Open/Answer）构造 —— 本层与 pending.go 的语义
// 是衔接的，用例里就得真走一遍，不许手搓一个「已答复」的假挂单。now 也一律注入（不用 time.Now）。
package policy

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// answeredFor — 造一张**真的答过**的挂单（Open + Answer 走真账本；失败即 Fatal）。
func answeredFor(t *testing.T, kind PendingKind, tool, digest string, ans Answer, expires time.Time) Pending {
	t.Helper()
	s := NewMemoryPendingStore()
	p := mustOpen(t, s, Pending{
		Kind: kind, Tool: tool, ArgsDigest: digest,
		SessionID: "sess-hd", RuleID: "never.rm",
		ExpiresAt: expires,
	}, pendingT0())
	got, err := s.Answer(p.ID, ans, pendingAt(time.Minute))
	if err != nil {
		t.Fatalf("Answer(%s/%s, %s) 不应失败：%v", kind, tool, ans, err)
	}
	if got.State != StatusAnswered || got.Answer != ans {
		t.Fatalf("答复后必须落成 answered 且记下答复：%+v", got)
	}
	return got
}

// askedPending — 造一张**还没人答**的挂单（真账本；用于「不许替人签字」的反例）。
func askedPending(t *testing.T, tool, digest string) Pending {
	t.Helper()
	s := NewMemoryPendingStore()
	return mustOpen(t, s, Pending{
		Kind: PendingToolApproval, Tool: tool, ArgsDigest: digest,
		ExpiresAt: pendingAt(5 * time.Minute),
	}, pendingT0())
}

// timedOutPending — 造一张**已按超时结算**的挂单（真账本 + 真到点）。
func timedOutPending(t *testing.T, tool, digest string) Pending {
	t.Helper()
	s := NewMemoryPendingStore()
	p := mustOpen(t, s, Pending{
		Kind: PendingToolApproval, Tool: tool, ArgsDigest: digest,
		ExpiresAt: pendingAt(time.Minute),
	}, pendingT0())
	if out := s.Expire(p.ExpiresAt); len(out) != 1 {
		t.Fatalf("到点必须结算恰好一条，得到 %d 条", len(out))
	}
	got, _ := s.Lookup(p.ID)
	if got.State != StatusTimedOut {
		t.Fatalf("结算后必须是 timed_out：%+v", got)
	}
	return got
}

// mustNotMaterialize — 断言：这次物化**必须报错**，且必须命中某个专项错误；
// 同时断言返回值是**零值 outcome**（报错时不许留下任何能被误当结果的东西）。
func mustNotMaterialize(t *testing.T, err error, want error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s ⇒ 必须报错，实际拿到 nil（fail-closed 被破坏）", what)
	}
	if !errors.Is(err, want) {
		t.Fatalf("%s ⇒ 必须命中 %v，实际错误：%v", what, want, err)
	}
}

// zeroOutcome — 零值判定（结构体含指针字段 ⇒ 用 DeepEqual，不用 ==）。
func zeroOutcome(o ToolCallOutcome) bool { return reflect.DeepEqual(o, ToolCallOutcome{}) }

// joinedResult — 模型**实际能看到**的两段文本（正文 + 硬话）拼起来 —— 断言一律打在这里，
// 因为「模型只看得见这两段」是本文件与模型之间的全部界面。
func joinedResult(o ToolCallOutcome) string {
	return o.Result.Content + "\n" + o.Result.ModelNote
}

// ── ① respond ⇒ 成功型且**无拒绝字样** ─────────────────────────────────────

func TestHumanDecision_RespondIsSuccessNotRejection(t *testing.T) {
	d := HumanDecision{
		Kind: DecisionRespond, Tool: "read", ArgsDigest: "sha256:aaaa1111",
		Response: "（人替工具给的答复）src/main.go 第 12 行：func main() {",
	}
	p := answeredFor(t, PendingToolApproval, "read", "sha256:aaaa1111", AnswerApprove, pendingAt(5*time.Minute))

	out, err := Materialize(d, p, ToolFacts{Tool: "read", HasSideEffect: false})
	if err != nil {
		t.Fatalf("无副作用的工具上 respond 必须成立（人替工具给结果）：%v", err)
	}
	if out.Kind != OutcomeResponded {
		t.Errorf("类别：got %q want %q", out.Kind, OutcomeResponded)
	}
	if !out.Result.Emitted || out.Result.Kind != ResultSuccess {
		t.Errorf("respond 的结果必须是**成功型**且真的发出去：%+v", out.Result)
	}
	if out.Result.Content != d.Response {
		t.Errorf("respond 的正文必须是**人给的内容原样**（不加前缀、不改写）：\ngot  %q\nwant %q",
			out.Result.Content, d.Response)
	}
	// ★ 本用例的核心：成功型结果里**不许出现任何拒绝/失败字样**。
	//   （出现即意味着「拿 respond 表达拒绝」—— 工具一步没跑，模型却以为做成了。）
	for _, bad := range []string{"拒绝", "未执行", "不要重试", "失败", "被拒"} {
		if strings.Contains(joinedResult(out), bad) {
			t.Errorf("respond 的结果里出现了「%s」⇒ 语义边界被破坏（F11：respond ≠ reject）", bad)
		}
	}
	if out.ShouldExecute() {
		t.Errorf("respond **不得放行执行**：人充当了工具，工具本体一步都不许跑（否则真副作用还会发生一次）")
	}
	if out.Escalated() {
		t.Errorf("respond 不是上交：%+v", out)
	}
	// 审计痕迹：决定原文 + 挂单回指 + **原始参数指纹** + 账本答复 + 低基数原因码。
	if out.Decision != DecisionRespond || out.PendingID != p.ID || out.SessionID != p.SessionID || out.RuleID != p.RuleID {
		t.Errorf("审计痕迹不全（决定/挂单/会话/规则）：%+v", out)
	}
	if out.ArgsDigest != p.ArgsDigest || out.ArgsDigest != "sha256:aaaa1111" {
		t.Errorf("必须留**原始**参数指纹：got %q want %q", out.ArgsDigest, p.ArgsDigest)
	}
	if out.LedgerState != StatusAnswered || out.LedgerAnswer != AnswerApprove {
		t.Errorf("账本侧事实必须原样留痕（供审计对照）：%+v", out)
	}
	if out.Reason != ReasonHumanRespond {
		t.Errorf("原因码：got %q want %q", out.Reason, ReasonHumanRespond)
	}
	if out.TargetTool() != "read" {
		t.Errorf("TargetTool：got %q want read", out.TargetTool())
	}
}

// ── ② reject ⇒ 未执行 + 不要重试 + 为什么（副作用工具上正是它该干活的地方）──

func TestHumanDecision_RejectMeansNotExecutedAndDoNotRetry(t *testing.T) {
	d := HumanDecision{
		Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:bbbb2222",
		Rejection: "这条 rm -rf 指向工作目录外，改成 /tmp/zerg-hd 下的路径再来",
		Reason:    ReasonHumanReject,
	}
	// ★ 对侧（本用例同时钉住的一件事）：**有副作用的工具上 reject 必须成立** ——
	//   它是闸轨的主用途；要是这里也报错，等于把闸拆了（「什么都拒」的假绿与「什么都批」一样糟）。
	p := answeredFor(t, PendingToolApproval, "bash", "sha256:bbbb2222", AnswerReject, pendingAt(5*time.Minute))

	out, err := Materialize(d, p, ToolFacts{Tool: "bash", HasSideEffect: true})
	if err != nil {
		t.Fatalf("有副作用的工具上 reject 必须成立（这是它的主用途）：%v", err)
	}
	if out.Kind != OutcomeRejected || !out.Result.Emitted || out.Result.Kind != ResultFailed {
		t.Fatalf("reject 必须产出**失败型**结果：%+v", out)
	}
	seen := joinedResult(out)
	for _, want := range []string{"未执行", "不要重试", string(ReasonHumanReject), "改成 /tmp/zerg-hd 下的路径"} {
		if !strings.Contains(seen, want) {
			t.Errorf("给模型的结果里必须出现「%s」（未执行 + 不要重试 + 为什么，三样缺一不可）：\n%s", want, seen)
		}
	}
	if strings.Contains(out.Result.Content, "【已执行】") {
		t.Errorf("reject 的正文不许出现「已执行」字样：%q", out.Result.Content)
	}
	if out.ShouldExecute() {
		t.Errorf("reject ⇒ **工具不执行**（有副作用的工具更不能执行）：%+v", out)
	}
	// 审计痕迹：原始参数指纹 + 决定原文 + 账本答复（账本记的也是 reject ⇒ 两侧同族）。
	if out.ArgsDigest != p.ArgsDigest || out.Decision != DecisionReject {
		t.Errorf("审计痕迹不全：%+v", out)
	}
	if out.LedgerState != StatusAnswered || out.LedgerAnswer != AnswerReject {
		t.Errorf("账本答复必须留痕且同族：%+v", out)
	}
	if out.Reason != ReasonHumanReject {
		t.Errorf("原因码：got %q want %q", out.Reason, ReasonHumanReject)
	}
	// 对侧：没给原因码时就取默认（human_reject）—— 但「为什么」仍必须落在给模型的话里。
	d2 := d
	d2.Reason = ""
	out2, err := Materialize(d2, p, ToolFacts{Tool: "bash", HasSideEffect: true})
	if err != nil {
		t.Fatalf("原因码留空应按 Kind 取默认，不该报错：%v", err)
	}
	if out2.Reason != ReasonHumanReject {
		t.Errorf("留空时原因码应取默认：got %q", out2.Reason)
	}
	// 反例：连「为什么」都不给（Rejection 与 Reason 都空）⇒ 报错（不解释的拒绝会让模型反复试同一条路）。
	d3 := HumanDecision{Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:bbbb2222"}
	if _, err := Materialize(d3, p, ToolFacts{Tool: "bash", HasSideEffect: true}); err == nil {
		t.Errorf("reject 不给任何「为什么」⇒ 必须报错")
	} else if !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("应命中 ErrInvalidDecision，实际：%v", err)
	}
}

// ── ③ correct ⇒ 参数被替换、原文保留、谁改的留痕、告警在场 ────────────────

func TestHumanDecision_CorrectReplacesArgsKeepsOriginalAndWarns(t *testing.T) {
	newArgs := map[string]any{"path": "/tmp/zerg-hd/a.txt", "content": "hi"}
	d := HumanDecision{
		Kind: DecisionCorrect, Tool: "write", ArgsDigest: "sha256:cccc3333",
		EditedAction: &EditedAction{
			Type:         EditActionType,
			EditedAction: EditedToolCall{Name: "write", Args: newArgs},
		},
		EditedBy: "Mr2109@local", Reason: ReasonHumanCorrect,
	}
	p := answeredFor(t, PendingToolApproval, "write", "sha256:cccc3333", AnswerCorrect, pendingAt(5*time.Minute))

	out, err := Materialize(d, p, ToolFacts{Tool: "write", HasSideEffect: true})
	if err != nil {
		t.Fatalf("correct 必须成立：%v", err)
	}
	if out.Kind != OutcomeCorrected || !out.ShouldExecute() {
		t.Fatalf("correct ⇒ 按**新参数**放行执行：%+v", out)
	}
	// 参数确实被替换了（新参数带着走）。
	if out.EditedAction == nil {
		t.Fatalf("correct 必须把 EditedAction 原样带上：%+v", out)
	}
	if out.EditedAction.EditedAction.Name != "write" {
		t.Errorf("更正后的工具名：got %q", out.EditedAction.EditedAction.Name)
	}
	if got := out.EditedAction.EditedAction.Args["path"]; got != "/tmp/zerg-hd/a.txt" {
		t.Errorf("更正后的参数必须是人的那一份：got %v", got)
	}
	// 原文保留（供审计）：**原始**工具名与**原始**参数指纹一字不改。
	if out.Tool != "write" || out.ArgsDigest != "sha256:cccc3333" {
		t.Errorf("更正后仍必须留住原文（工具名 + 原始参数指纹）：%+v", out)
	}
	if out.ToolChanged {
		t.Errorf("本例没换工具名 ⇒ ToolChanged 应为 false")
	}
	if out.TargetTool() != "write" {
		t.Errorf("TargetTool：got %q", out.TargetTool())
	}
	// 谁改的必须留痕，且**进得了序列化面**（重启/取证都读得到）。
	if out.EditedBy != "Mr2109@local" {
		t.Errorf("EditedBy 必须留痕：got %q", out.EditedBy)
	}
	b, jerr := json.Marshal(out)
	if jerr != nil {
		t.Fatalf("outcome 必须可序列化（审计要落盘）：%v", jerr)
	}
	if !strings.Contains(string(b), `"edited_by":"Mr2109@local"`) {
		t.Errorf("「谁改的」必须进序列化面：%s", string(b))
	}
	if !strings.Contains(string(b), `"args_digest":"sha256:cccc3333"`) {
		t.Errorf("原始参数指纹必须进序列化面：%s", string(b))
	}
	// 新参数指纹在场，且与旧的**不同**（说明「改了什么」这件事可对照）。
	if out.NewArgsLocalDigest == "" {
		t.Errorf("新参数的规范化指纹必须算出来（对照用）")
	}
	if out.NewArgsLocalDigest == out.ArgsDigest {
		t.Errorf("新参数指纹与原始指纹相同 ⇒ 说明对照失效：%q", out.NewArgsLocalDigest)
	}
	// F10 的告警：机器可判 + 人可读，两处都要在。
	if out.Warning != CorrectReplanWarning {
		t.Errorf("Warning 必须是 CorrectReplanWarning：got %q", out.Warning)
	}
	if !strings.Contains(out.Note, CorrectReplanWarning) {
		t.Errorf("告警也必须进 Note（人可读）：%q", out.Note)
	}
	// 对侧：工具还没跑 ⇒ 本 outcome **不含**给模型的结果（真实结果稍后由工具产出）。
	if out.Result.Emitted {
		t.Errorf("correct 不产出给模型的结果（工具还没执行）：%+v", out.Result)
	}

	// 反例 A：没有新参数（EditedAction 为 nil）⇒ 报错。
	dNoEdit := d
	dNoEdit.EditedAction = nil
	if _, err := Materialize(dNoEdit, p, ToolFacts{Tool: "write", HasSideEffect: true}); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("correct 缺 EditedAction ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// 反例 B：说不出「谁改的」⇒ 报错（留痕缺一半）。
	dNoBy := d
	dNoBy.EditedBy = "  "
	if _, err := Materialize(dNoBy, p, ToolFacts{Tool: "write", HasSideEffect: true}); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("correct 缺 EditedBy ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// 反例 C：形态不是标准形态（type 认不出）⇒ 报错（不猜形态）。
	dBadType := d
	ea := *d.EditedAction
	ea.Type = "patch"
	dBadType.EditedAction = &ea
	if _, err := Materialize(dBadType, p, ToolFacts{Tool: "write", HasSideEffect: true}); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("EditedAction.type 认不出 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
}

// ── ④ escalate ⇒ 专项错误 + 上交标记（**不当成普通结果**）──────────────────

func TestHumanDecision_EscalateGoesToUpperLayerNotToModel(t *testing.T) {
	// 反例的对侧：上交**允许**挂在未答复的挂单上（它不冒充谁的签字、也不产出给模型的东西）。
	p := askedPending(t, "bash", "sha256:dddd4444")
	d := HumanDecision{
		Kind: DecisionEscalate, Tool: "bash", ArgsDigest: "sha256:dddd4444",
		Rejection: "这台机器我没权限碰，交给上层运维", Reason: ReasonHumanEscalate,
	}

	out, err := Materialize(d, p)
	if err == nil {
		t.Fatalf("escalate 必须返回**专项错误**（否则接线批会把「交回上层」当成一条普通结果 ⇒ 默默丢弃）")
	}
	if !errors.Is(err, ErrEscalated) {
		t.Fatalf("escalate 的错误必须可判（errors.Is(ErrEscalated)），实际：%v", err)
	}
	if !out.Escalated() || out.Kind != OutcomeEscalated {
		t.Errorf("必须留下**上交标记**：%+v", out)
	}
	// ★ 核心：上交**不产出任何能喂给模型的东西**（没有结果、没有正文、没有硬话）。
	if out.Result.Emitted || out.Result.Content != "" || out.Result.ModelNote != "" || out.Result.Kind != "" {
		t.Errorf("escalate 不得产出可喂模型的结果（交回上层 ≠ 给模型一个答复）：%+v", out.Result)
	}
	if out.ShouldExecute() {
		t.Errorf("escalate 不得放行执行")
	}
	// 人的话与理由留在**给人/审计**的字段里（不是留给模型的）。
	if !strings.Contains(out.Note, "交给上层运维") || !strings.Contains(out.Note, string(ReasonHumanEscalate)) {
		t.Errorf("上交的理由与人的话必须留痕：%q", out.Note)
	}
	if out.Decision != DecisionEscalate || out.PendingID != p.ID || out.ArgsDigest != "sha256:dddd4444" {
		t.Errorf("审计痕迹不全：%+v", out)
	}
	if out.LedgerState != StatusOpen {
		t.Errorf("本例挂在未答复的挂单上 ⇒ 账本状态应如实记为 open：%+v", out)
	}
	// 对侧：上交也可以发生在**已答复**的挂单上（人先答了再上交）。
	pAns := answeredFor(t, PendingToolApproval, "bash", "sha256:dddd4444", AnswerReject, pendingAt(5*time.Minute))
	if _, err := Materialize(d, pAns); !errors.Is(err, ErrEscalated) {
		t.Errorf("已答复的挂单上 escalate 也必须走同一条路（ErrEscalated），实际 %v", err)
	}
	// 反例：上交却不说是为什么（三样都空）⇒ 报错（上层接到的不能是一张白纸）。
	dEmpty := HumanDecision{Kind: DecisionEscalate, Tool: "bash", ArgsDigest: "sha256:dddd4444"}
	if _, err := Materialize(dEmpty, p); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("escalate 不给任何理由/人的话 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
}

// ── ⑤ 反例：respond 用在**有副作用**的工具上 ⇒ 必须报错 ────────────────────

func TestHumanDecision_RespondOnSideEffectIsRefused(t *testing.T) {
	d := HumanDecision{
		Kind: DecisionRespond, Tool: "write", ArgsDigest: "sha256:eeee5555",
		Response: "文件已经写好了（人说的）",
	}
	p := answeredFor(t, PendingToolApproval, "write", "sha256:eeee5555", AnswerApprove, pendingAt(5*time.Minute))

	out, err := Materialize(d, p, ToolFacts{Tool: "write", HasSideEffect: true})
	mustNotMaterialize(t, err, ErrRespondOnSideEffect,
		"用 respond 去回应一个有副作用的工具（那等于告诉模型「做成了」）")
	if !zeroOutcome(out) {
		t.Errorf("报错时必须返回**零值** outcome（不许留下半成品被误当结果）：%+v", out)
	}

	// ★ 对侧（防「什么都拒」的假绿）：**同一件事实下，reject 必须照常可用** ——
	//   闸要拦的是「宣布成功」，不是「拒绝执行」。
	pRej := answeredFor(t, PendingToolApproval, "write", "sha256:eeee5555", AnswerReject, pendingAt(5*time.Minute))
	outRej, err := Materialize(HumanDecision{
		Kind: DecisionReject, Tool: "write", ArgsDigest: "sha256:eeee5555", Rejection: "这个路径不许写",
	}, pRej, ToolFacts{Tool: "write", HasSideEffect: true})
	if err != nil {
		t.Fatalf("同条件下 reject 必须照常可用（否则是把闸拆了，不是修语义）：%v", err)
	}
	if outRej.Result.Kind != ResultFailed || outRej.ShouldExecute() {
		t.Errorf("reject 的结局必须是「失败 + 不执行」：%+v", outRej)
	}
	// 再一个对侧：无副作用的工具上 respond 必须成立（见用例①；这里用第二个工具确认不是「只放行 read」）。
	pRO := answeredFor(t, PendingToolApproval, "grep", "sha256:eeee5555", AnswerApprove, pendingAt(5*time.Minute))
	if _, err := Materialize(HumanDecision{
		Kind: DecisionRespond, Tool: "grep", ArgsDigest: "sha256:eeee5555", Response: "（人给的结果）命中 3 处",
	}, pRO, ToolFacts{HasSideEffect: false}); err != nil {
		t.Errorf("无副作用工具上 respond 必须成立，实际 %v", err)
	}
}

// ── ⑥ 反例：未知决定 ⇒ 必须报错（fail-closed）──────────────────────────────

func TestHumanDecision_UnknownKindIsRefused(t *testing.T) {
	p := answeredFor(t, PendingToolApproval, "bash", "sha256:ffff6666", AnswerApprove, pendingAt(5*time.Minute))
	for _, bad := range []string{"accept", "yes", "RESPOND!", ""} {
		out, err := Materialize(HumanDecision{Kind: DecisionKind(bad), Tool: "bash", ArgsDigest: "sha256:ffff6666"}, p)
		mustNotMaterialize(t, err, ErrUnknownDecisionKind, "未知决定「"+bad+"」")
		if !zeroOutcome(out) {
			t.Errorf("未知决定「%s」时不许产出任何结果：%+v", bad, out)
		}
	}
	// ParseDecisionKind：大小写不敏感 + 去空白；认不出报错。
	if k, err := ParseDecisionKind("  ReSpOnD "); err != nil || k != DecisionRespond {
		t.Errorf("ParseDecisionKind 应认大小写与空白：got (%q, %v)", k, err)
	}
	if _, err := ParseDecisionKind("nope"); !errors.Is(err, ErrUnknownDecisionKind) {
		t.Errorf("ParseDecisionKind 认不出必须命中 ErrUnknownDecisionKind，实际 %v", err)
	}
	// 对侧：五态一个都不许被这条闸误伤（「认不出就拒」不等于「什么都拒」）。
	for _, k := range []DecisionKind{
		DecisionRespond, DecisionReject, DecisionApprove, DecisionCorrect, DecisionEscalate,
	} {
		if !k.Valid() {
			t.Errorf("合法决定 %q 被判成非法 ⇒ 闸开过头了", k)
		}
	}
}

// ── ⑦ 反例：副作用事实缺省 ⇒ 按「有」处理（respond 被拒；reject 不受影响）──

func TestHumanDecision_MissingSideEffectFactsAreTreatedAsSideEffect(t *testing.T) {
	p := answeredFor(t, PendingToolApproval, "read", "sha256:7777aaaa", AnswerApprove, pendingAt(5*time.Minute))
	// 省略 facts = 不知道有没有副作用 ⇒ **按「有」处理**（本文件唯一一处「不知道 ⇒ 更严」的默认）。
	out, err := Materialize(HumanDecision{
		Kind: DecisionRespond, Tool: "read", ArgsDigest: "sha256:7777aaaa", Response: "内容如下…",
	}, p)
	mustNotMaterialize(t, err, ErrRespondOnSideEffect, "副作用事实缺省时的 respond（不知道就不许宣布成功）")
	if !zeroOutcome(out) {
		t.Errorf("报错时必须是零值 outcome：%+v", out)
	}
	// 对侧：缺省取严**只压「宣布成功」，不压「拒绝」**（否则缺省就变成了「什么都拒」）。
	pRej := answeredFor(t, PendingToolApproval, "bash", "sha256:7777bbbb", AnswerReject, pendingAt(5*time.Minute))
	outRej, err := Materialize(HumanDecision{
		Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:7777bbbb", Rejection: "先别动",
	}, pRej)
	if err != nil {
		t.Fatalf("缺省事实下 reject 必须照常可用：%v", err)
	}
	if outRej.Result.Kind != ResultFailed {
		t.Errorf("reject 仍是失败型：%+v", outRej.Result)
	}
	// 反例：给**两份**工具事实 ⇒ 报错（不猜哪一份是真的）。
	if _, err := Materialize(HumanDecision{
		Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:7777bbbb", Rejection: "先别动",
	}, pRej, ToolFacts{Tool: "bash", HasSideEffect: true}, ToolFacts{Tool: "bash", HasSideEffect: false}); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("给两份工具事实 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// 反例：工具事实说的是**另一把工具** ⇒ 对不上号 ⇒ 报错。
	if _, err := Materialize(HumanDecision{
		Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:7777bbbb", Rejection: "先别动",
	}, pRej, ToolFacts{Tool: "write", HasSideEffect: true}); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("工具事实与挂单对不上号 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
}

// ── ⑧ 反例：替人签字 / 批错对象 ⇒ 逐条报错 ─────────────────────────────────

func TestHumanDecision_FailClosedOnLedgerMismatch(t *testing.T) {
	facts := ToolFacts{Tool: "bash", HasSideEffect: true}

	// (a) 挂单**还没人答** ⇒ 不许替人签字（四种「要执行/要回话」的决定逐个钉）。
	open := askedPending(t, "bash", "sha256:8888bbbb")
	for _, d := range []HumanDecision{
		{Kind: DecisionApprove, Tool: "bash", ArgsDigest: "sha256:8888bbbb"},
		{Kind: DecisionReject, Tool: "bash", ArgsDigest: "sha256:8888bbbb", Rejection: "不许"},
		{Kind: DecisionCorrect, Tool: "bash", ArgsDigest: "sha256:8888bbbb", EditedBy: "me",
			EditedAction: &EditedAction{Type: EditActionType, EditedAction: EditedToolCall{Name: "bash"}}},
		{Kind: DecisionRespond, Tool: "bash", ArgsDigest: "sha256:8888bbbb", Response: "给结果"},
	} {
		if _, err := Materialize(d, open, facts); !errors.Is(err, ErrInvalidDecision) {
			t.Errorf("%s 挂在未答复的挂单上 ⇒ 应命中 ErrInvalidDecision（不许替人签字），实际 %v", d.Kind, err)
		}
	}

	// (b) 参数指纹不一致 ⇒ 批的是另一次调用 ⇒ 报错（与 grants 的逐字匹配同一纪律）。
	p := answeredFor(t, PendingToolApproval, "bash", "sha256:8888bbbb", AnswerApprove, pendingAt(5*time.Minute))
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "bash", ArgsDigest: "sha256:other"}, p, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("指纹不一致 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// (c) 挂单有指纹、决定没写 ⇒ 无法确认是同一次调用 ⇒ 报错。
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "bash"}, p, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("决定没写指纹 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// (d) 决定指向**另一把工具** ⇒ 批错对象 ⇒ 报错。
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "write", ArgsDigest: "sha256:8888bbbb"}, p, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("工具名不一致 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}

	// (e) **已按超时结算**的挂单 ⇒ 迟到的答复不改变结局（与 pending.go ② 同向）。
	timed := timedOutPending(t, "bash", "sha256:9999cccc")
	for _, d := range []HumanDecision{
		{Kind: DecisionApprove, Tool: "bash", ArgsDigest: "sha256:9999cccc"},
		{Kind: DecisionEscalate, Tool: "bash", ArgsDigest: "sha256:9999cccc", Reason: ReasonHumanEscalate},
	} {
		if _, err := Materialize(d, timed, facts); !errors.Is(err, ErrInvalidDecision) {
			t.Errorf("%s 挂在已超时结算的挂单上 ⇒ 应命中 ErrInvalidDecision，实际 %v", d.Kind, err)
		}
	}

	// (f) 决定与**账本里记的答复不同族**（账本记 reject、决定说 approve）⇒ 报错。
	pRej := answeredFor(t, PendingToolApproval, "bash", "sha256:8888bbbb", AnswerReject, pendingAt(5*time.Minute))
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "bash", ArgsDigest: "sha256:8888bbbb"}, pRej, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("账本答复与决定不同族 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// 对侧：同族的 approve 必须照常放行（别把同族闸做成「一律拒」）。
	pApp := answeredFor(t, PendingToolApproval, "bash", "sha256:8888bbbb", AnswerApprove, pendingAt(5*time.Minute))
	out, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "bash", ArgsDigest: "sha256:8888bbbb"}, pApp, facts)
	if err != nil {
		t.Fatalf("同族 approve 必须成立：%v", err)
	}
	if !out.ShouldExecute() || out.Result.Emitted {
		t.Errorf("approve ⇒ 放行执行且**不含**给模型的结果（真实结果稍后产出）：%+v", out)
	}
	if out.Reason != ReasonHumanApprove || out.LedgerAnswer != AnswerApprove {
		t.Errorf("audit 痕迹：%+v", out)
	}

	// (g) 空工具名的挂单（真账本拒开 ⇒ 这里只钉「归一化后为空就拒」这条守卫本身）。
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "  "}, Pending{State: StatusAnswered, Answer: AnswerApprove}, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("无名挂单 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
	// (h) 状态认不出 ⇒ 不猜、不物化（fail-closed）。
	if _, err := Materialize(HumanDecision{Kind: DecisionApprove, Tool: "bash"}, Pending{State: "weird", Tool: "bash"}, facts); !errors.Is(err, ErrInvalidDecision) {
		t.Errorf("状态认不出 ⇒ 应命中 ErrInvalidDecision，实际 %v", err)
	}
}

// ── ⑨ 更正的结构化形态 = F10 的标准形态（读写两端同源）─────────────────────

func TestHumanDecision_EditedActionJSONShapeIsTheStandardForm(t *testing.T) {
	ea := NewEdit("write", map[string]any{"path": "/tmp/a.txt", "content": "hi"})
	b, err := json.Marshal(ea)
	if err != nil {
		t.Fatalf("Marshal：%v", err)
	}
	// json.Marshal 对 map 键排序 ⇒ 字节形态确定（同内容必然同字节）。
	const want = `{"type":"edit","edited_action":{"name":"write","args":{"content":"hi","path":"/tmp/a.txt"}}}`
	if string(b) != want {
		t.Errorf("更正的标准形态不对：\ngot  %s\nwant %s", string(b), want)
	}
	// 反向：能原样读回来（读写两端同源 —— 形态只有一份真相）。
	var back EditedAction
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal：%v", err)
	}
	if !reflect.DeepEqual(back, ea) {
		t.Errorf("往返不一致：got %+v want %+v", back, ea)
	}
	if back.Type != EditActionType || back.EditedAction.Name != "write" ||
		back.EditedAction.Args["path"] != "/tmp/a.txt" {
		t.Errorf("往返后的字段不对：%+v", back)
	}
	// NewEdit 写死的 type 就是 F10 那一个（不许是空串或别的词）。
	if ea.Type != "edit" {
		t.Errorf("NewEdit 必须写死 type=edit：got %q", ea.Type)
	}
	// 本地规范化指纹：**同内容必然同值、不同内容必然不同值**（否则「改了什么」对照失效）。
	a := localArgsDigest(map[string]any{"path": "/tmp/a.txt", "content": "hi"})
	a2 := localArgsDigest(map[string]any{"content": "hi", "path": "/tmp/a.txt"}) // 键序不同 ⇒ 同值
	c := localArgsDigest(map[string]any{"path": "/tmp/b.txt", "content": "hi"})
	if a == "" || a != a2 {
		t.Errorf("同内容必须同指纹（键序无关）：%q vs %q", a, a2)
	}
	if a == c {
		t.Errorf("不同内容必须不同指纹：%q vs %q", a, c)
	}
	if !strings.HasPrefix(a, "sha256local:") {
		t.Errorf("本地指纹必须带显式前缀（别与审计侧权威指纹混淆）：%q", a)
	}
	if localArgsDigest(nil) != "" {
		t.Errorf("算不出时必须**显式空串**，不编假指纹：%q", localArgsDigest(nil))
	}
}
