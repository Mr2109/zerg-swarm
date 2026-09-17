// escalation_signal_test.go — B 项⑥ 用例：**升级信号闭集**（逐字）+「不在闭集内必须给理由码」+ 升级事件出口。
//
// 覆盖（正反两侧都要）：
//
//	A. 六条信号**逐字**对齐设计稿 §4.7 第 255 行（字面写死，不用常量自证）+ 无重复 + 恰好六条
//	B. 两个闭集**互不派生**：升级信号闭集 ∩ 事件名闭集 = ∅，且事件名仍是 §6.1 的四名（没被本件撑成五名）
//	C. **进闭集** ⇒ 通过（**不要求**理由码）；带/不带理由码的**差异**落在原始 JSON 上（有无 `reason_code` 字段）
//	D. **不在闭集内且无理由码** ⇒ 报错（ErrMissingEscalationReasonCode）+ 观测面**不落行**
//	E. **不在闭集内但带理由码** ⇒ 通过（裁决面）；观测面**不落升级行**（本仓口径：升级必须带闭集内信号）
//	F. 升级事件出口：事件名 / 结局 / 片归因 / R 编号 / 未标定 / detail 可行动 + 不回显自由文本
package sliceobs

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// ── A. 六条信号逐字对齐设计稿 §4.7 ──────────────────────────────────────────

func TestEscalationSignals_ClosedSetVerbatimFromDesignDoc(t *testing.T) {
	// 期望值**写成字面**（不引用常量）：常量的值本身就是要被钉住的东西 ——
	// 拿常量断言常量等于自证。来源：v2.1 §4.7 第 255 行（「① 不可逆/删除/越权 … ⑥ 同一片二次打回」）。
	want := []string{
		"不可逆/删除/越权",
		"两轮无进展或预算耗尽",
		"自报低置信或自相矛盾",
		"输入含可疑指令（注入）",
		"触网/外发",
		"同一片二次打回", // 稿面行文末带句号「。」—— 句读不属于条目本身（登记在 escalation_signal.go 头）
	}
	got := EscalationSignals()
	if len(got) != 6 {
		t.Fatalf("升级信号闭集应**恰好 6 条**（§4.7 ①②③④⑤⑥），实际 %d 条：%v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条与设计稿 §4.7 不符：期望 %q，实际 %q", i+1, want[i], got[i])
		}
	}
	// 逐条自造一条「多一个字符/少一个字符」的近似串 ⇒ 必须**不**算命中（逐字比较，不归一化）
	for _, sig := range want {
		if !IsEscalationSignal(sig) {
			t.Fatalf("闭集内信号 %q 必须被认成闭集内", sig)
		}
		for _, near := range []string{sig + " ", " " + sig, sig + "。", sig + "/越权"} {
			if IsEscalationSignal(near) {
				t.Fatalf("近似串 %q 不得算命中（逐字比较、不归一化：认不出时不猜）", near)
			}
		}
	}
	// 闭集内无重复、无空串（防「六条」被凑数）
	seen := map[string]bool{}
	for i, s := range got {
		if strings.TrimSpace(s) == "" {
			t.Fatalf("第 %d 条是空串（闭集里不许有空条目）", i+1)
		}
		if seen[s] {
			t.Fatalf("闭集里有重复条目：%q", s)
		}
		seen[s] = true
	}
	t.Logf("升级信号闭集（§4.7 逐字，%d 条）= %v", len(got), got)
}

// ── B. 两个闭集互不派生（信号 ≠ 事件名）──────────────────────────────────

func TestEscalationSignals_DisjointFromEventNameClosedSet(t *testing.T) {
	// 事件名闭集仍是 §6.1 的四名 —— 本件**没**把信号塞成第五个事件名
	if got := len(EventNames()); got != 4 {
		t.Fatalf("事件名闭集必须仍是 §6.1 的四名（本件不动它），实际 %d 名：%v", got, EventNames())
	}
	for _, sig := range EscalationSignals() {
		if knownEvent(sig) {
			t.Fatalf("升级信号 %q 不得同时是事件名（两个闭集互不派生）", sig)
		}
	}
	for _, name := range EventNames() {
		if IsEscalationSignal(name) {
			t.Fatalf("事件名 %q 不得同时是升级信号（两个闭集互不派生）", name)
		}
	}
	if IsEscalationSignal(AliasSliceMount) {
		t.Fatalf("归并别名 %q 也不得混进信号闭集", AliasSliceMount)
	}
	t.Logf("事件名闭集 = %v ｜ 升级信号闭集 = %v ｜ 交集 = ∅", EventNames(), EscalationSignals())
}

// ── C. 进闭集 ⇒ 通过（不要求理由码）；带/不带理由码的差异 ────────────────────

func TestEscalationValidation_SignalsInClosedSetPassWithoutReasonCode(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const reason = "已人工确认" // 理由码**没有**稿面语法 ⇒ 用例只用一句普通文本（本件不发明语法）

	// ① 六条信号各一：不带理由码 ⇒ 校验通过，且裁决 = 升级
	for i, sig := range EscalationSignals() {
		if err := ValidateEscalationSignal(sig, ""); err != nil {
			t.Fatalf("闭集内信号 %q **不要求**理由码，却报错：%v", sig, err)
		}
		v, err := NewEscalationVerdict(sig, "")
		if err != nil {
			t.Fatalf("闭集内信号 %q 构造裁决失败：%v", sig, err)
		}
		if !v.Escalate {
			t.Fatalf("闭集内信号 %q 的裁决必须是「升级」（§4.12：升级信号 ⇒ ask/升级档）", sig)
		}
		if v.ReasonCode != "" {
			t.Fatalf("不带理由码时裁决里不该凭空冒出理由码，实际 %q", v.ReasonCode)
		}
		EmitSliceEscalatedSignal("S-"+strconv.Itoa(i+1), "t-esc", sig, "", "", nil)
	}
	// ② 差异侧：闭集内 + 理由码 ⇒ **仍**通过（理由是补充，不是必需），且理由码必须如实落到行里
	r := 7
	if err := ValidateEscalationSignal(SignalNetworkOrOutbound, reason); err != nil {
		t.Fatalf("闭集内信号带理由码也必须通过（理由码是补充理由）：%v", err)
	}
	EmitSliceEscalatedSignal("S-7", "t-esc", SignalNetworkOrOutbound, reason, "test-ruleset-v9", &r)

	raw := readRawEvents(t)
	if len(raw) != 7 {
		t.Fatalf("应落 7 行（6 条无理由码 + 1 条带理由码），实际 %d 行：%v", len(raw), raw)
	}
	evs := readEvents(t)
	for i, ev := range evs[:6] {
		if ev.Event != EventSliceEscalated {
			t.Fatalf("第 %d 行事件名应为 %s，实际 %q", i+1, EventSliceEscalated, ev.Event)
		}
		if ev.Signal != EscalationSignals()[i] {
			t.Fatalf("第 %d 行信号应逐字为 %q，实际 %q", i+1, EscalationSignals()[i], ev.Signal)
		}
		if ev.ReasonCode != "" {
			t.Fatalf("第 %d 行没给理由码却落了 reason_code=%q", i+1, ev.ReasonCode)
		}
		// **查原始 JSON**：没给理由码 ⇒ `reason_code` 字段整个不出现（不写空串冒充）
		if strings.Contains(raw[i], `"reason_code":`) {
			t.Fatalf("第 %d 行没给理由码，原始行里不得出现 reason_code 字段：%s", i+1, raw[i])
		}
	}
	last := evs[6]
	if last.Signal != SignalNetworkOrOutbound || last.ReasonCode != reason {
		t.Fatalf("带理由码那行应为 signal=%q reason_code=%q，实际 signal=%q reason_code=%q",
			SignalNetworkOrOutbound, reason, last.Signal, last.ReasonCode)
	}
	if last.R == nil || *last.R != 7 || last.CriteriaVersion != "test-ruleset-v9" {
		t.Fatalf("R 编号与判据版本应如实带上：r=%v criteria_version=%q", last.R, last.CriteriaVersion)
	}
	if !strings.Contains(raw[6], `"reason_code":"`+reason+`"`) {
		t.Fatalf("带理由码那行的原始 JSON 里必须真有 reason_code：%s", raw[6])
	}
	t.Logf("原始行①（无理由码）=%s", raw[0])
	t.Logf("原始行⑦（带理由码）=%s", raw[6])
}

// ── D. 不在闭集内且无理由码 ⇒ 报错 + 不落行 ─────────────────────────────────

func TestEscalationValidation_OutOfClosedSetWithoutReasonCodeFails(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	outOfSet := []struct {
		why    string
		signal string
	}{
		{"空串 = 一条都没命中（稿面说的「无需升级」）", ""},
		{"只有空白", "   "},
		{"截断的自造条目", "不可逆/删除"},
		{"自造近义说法", "越权操作"},
		{"闭集内条目但带尾随空格（逐字比较 ⇒ 不算命中）", SignalNetworkOrOutbound + " "},
		{"闭集内条目但带句号", SignalSecondSendbackOfSameSlice + "。"},
		{"压根不是信号", "要不要升级我拿不准"},
	}
	for _, c := range outOfSet {
		err := ValidateEscalationSignal(c.signal, "")
		if err == nil {
			t.Fatalf("不在闭集内却无理由码必须报错（§4.7-9g）：signal=%q（%s）", c.signal, c.why)
		}
		if !errors.Is(err, ErrMissingEscalationReasonCode) {
			t.Fatalf("错误必须可用 errors.Is 判成 ErrMissingEscalationReasonCode，实际 %v（signal=%q）", err, c.signal)
		}
		// 空白理由码 = 没给理由码（不认「一个空格」为理由）
		if err := ValidateEscalationSignal(c.signal, "   "); err == nil {
			t.Fatalf("只有空白的理由码必须与「没给」同判：signal=%q", c.signal)
		}
		// 构造侧同样的门（构造不出一个过不了校验的裁决）
		if _, err := NewEscalationVerdict(c.signal, ""); err == nil {
			t.Fatalf("裁决构造侧也必须拦住：signal=%q", c.signal)
		}
		// 观测侧：不在闭集内 ⇒ **不落行**（与「事件名认不出不落」同规；绝不猜成某一条信号）
		EmitSliceEscalatedSignal("S-x", "t-x", c.signal, "", "", nil)
	}
	if got := readRawEvents(t); len(got) != 0 {
		t.Fatalf("不在闭集内的信号一律不落行，实际落了 %d 行：%v", len(got), got)
	}
	t.Logf("闭集外 + 无理由码：%d 种写法全部报 ErrMissingEscalationReasonCode 且零落行", len(outOfSet))
}

// ── E. 不在闭集内但带理由码 ⇒ 裁决面通过（观测面不落升级行：本仓口径）────────

func TestEscalationValidation_OutOfClosedSetWithReasonCodePasses(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const reason = "人已批准外发"

	if err := ValidateEscalationSignal("", reason); err != nil {
		t.Fatalf("不在闭集内**但带理由码**必须通过（§4.7-9g 只要求给理由码）：%v", err)
	}
	v, err := NewEscalationVerdict("要触网", reason)
	if err != nil {
		t.Fatalf("带理由码的闭集外信号应可构造裁决：%v", err)
	}
	if v.Escalate {
		t.Fatal("不在闭集内的信号不得被判成「升级」（不是六条之一）")
	}
	if v.ReasonCode != reason {
		t.Fatalf("理由码应如实保留，实际 %q", v.ReasonCode)
	}
	if v.Signal != "要触网" {
		t.Fatalf("信号应如实保留（不改写、不猜成闭集里的条目），实际 %q", v.Signal)
	}

	// 观测面口径（登记）：升级事件必须带**闭集内**信号 ⇒ 闭集外的裁决**不落升级行**
	// （§6.1 的四个事件名里没有「无需升级」这一名 ⇒ 这类裁决今天落不了行，登记为未实现项）
	EmitSliceEscalatedSignal("S-e", "t-e", "要触网", reason, "", nil)
	if got := readRawEvents(t); len(got) != 0 {
		t.Fatalf("闭集外的信号不得落成升级行（升级行无从解释），实际落了 %d 行：%v", len(got), got)
	}
	// 反侧：闭集内信号必须落（否则上面那条「不落」可被「什么都不落」伪证）
	EmitSliceEscalatedSignal("S-e", "t-e", SignalSuspiciousInstructionInInput, reason, "", nil)
	if got := readRawEvents(t); len(got) != 1 {
		t.Fatalf("闭集内信号必须落 1 行，实际 %d 行", len(got))
	}
	t.Logf("闭集外 + 理由码：裁决通过（escalate=false）· 观测面 0 行；闭集内信号同参数 ⇒ 1 行")
}

// ── F. 升级事件出口的形状（结局 / 归因 / 未标定 / detail）───────────────────

func TestEscalationEmit_ShapeAndUncalibratedVersion(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	EmitSliceEscalatedSignal("S-f1", "t-f1", SignalIrreversibleOrDeleteOrPrivilege, "", "", nil)

	raw := readRawEvents(t)
	if len(raw) != 1 {
		t.Fatalf("应落 1 行，实际 %d 行", len(raw))
	}
	ev := readEvents(t)[0]
	if ev.Event != "slice_escalated" {
		t.Fatalf("事件名应为 slice_escalated，实际 %q", ev.Event)
	}
	if ev.OK {
		t.Fatal("升级不是「通过」：ok 必须为 false（不静默通过）")
	}
	if ev.Outcome != OutcomeRaised {
		t.Fatalf("信号触发的升级沿用升级类结局 %q，实际 %q（不新增第七个结局值）", OutcomeRaised, ev.Outcome)
	}
	if ev.SliceID != "S-f1" || ev.TaskID != "t-f1" {
		t.Fatalf("片/任务归因不符：%+v", ev)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本无来源 ⇒ 必须写「未标定」（不编造），实际 %q", ev.CriteriaVersion)
	}
	// R 编号没给 ⇒ 字段整个不出现（不写 0 冒充 R0）
	if strings.Contains(raw[0], `"r":`) || ev.R != nil {
		t.Fatalf("没给 R 编号时不得写 r 字段：%s", raw[0])
	}
	// detail 可行动：点明信号 + 升级档 + 不放行；且**不得**回显调用方自由文本（本函数不接受自由文本）
	for _, want := range []string{"升级信号", SignalIrreversibleOrDeleteOrPrivilege, "ask/升级档", "不放行"} {
		if !strings.Contains(ev.Detail, want) {
			t.Fatalf("detail 必须可行动（缺 %q）：%q", want, ev.Detail)
		}
	}
	// 缺 slice_id 时 detail 必须显式点出这是观测缺陷
	EmitSliceEscalatedSignal("", "t-f2", SignalNetworkOrOutbound, "", "", nil)
	evs := readEvents(t)
	if !strings.Contains(evs[1].Detail, "观测缺陷") {
		t.Fatalf("缺 slice_id 时必须点明观测缺陷：%q", evs[1].Detail)
	}
	t.Logf("原始行 = %s", raw[0])
}
