// team_events_test.go — B 项 B9 用例：**派活/小队层事件族**（§6.2 逐字三名 + 同一套机制）。
//
// 覆盖（正反两侧都要）：
//
//	A. 三个记录名**逐字**对齐设计稿 §6.2 第 376 行（字面写死，不用常量自证）+ §6.1 四名**一字未动**
//	   + 合并闭集 = 7 名、无重复、且与升级信号闭集交集为空
//	B. 三个名字**各至少一条**用例：都能落盘、原始 JSON 字段齐（事件名 / 结局 / agent_name / id / target）
//	C. 判据版本无来源 ⇒ 「未标定」；调用方给真来源 ⇒ 如实带上（**不覆盖**）
//	D. 交接对象：**只有一个**出处取值 `user`；闭集外（近义 / 大小写 / 尾随空格 / 空）⇒ **不落行**（认不出不猜）
//	E. 低基数结局这一位必须有值：派活侧三名**没有**稿面来源的固定结局 ⇒ 结局不给/认不出 ⇒ **不落行**；
//	   同一作用域内**反侧**：片侧四名的兜底行为**一字未变**（零回归钉死）
//	F. best-effort：落点写不进去时三个出口都不 panic、不阻断、不造文件
//	G. 归因字段缺失 ⇒ 落行但 detail 点明「观测缺陷」（不静默）
package sliceobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── A. 三个记录名逐字对齐设计稿 §6.2；§6.1 四名未被撑动 ──────────────────────

func TestTeamEvents_NamesVerbatimFromDesignDoc(t *testing.T) {
	// 期望值**写成字面**（不引用常量）：常量的值本身就是要被钉住的东西 ——
	// 拿常量断言常量等于自证。设计稿 v2.1 §6.2 第 376 行逐字三名。
	want := []string{"squad", "work_order", "handoff"}
	got := TeamEventNames()
	if len(got) != len(want) {
		t.Fatalf("派活/小队层记录名应 %d 个（设计稿 §6.2 第 376 行），实际 %d 个：%v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个与设计稿 §6.2 不符：期望 %q，实际 %q", i+1, want[i], got[i])
		}
	}

	// ① §6.1 四名**一字未动**（B9 只加名，不改本节：EventNames 仍是那四条的逐字清单）
	sliceNames := EventNames()
	if len(sliceNames) != 4 {
		t.Fatalf("EventNames() 必须仍是 §6.1 的四名（本件不动它），实际 %d 名：%v", len(sliceNames), sliceNames)
	}
	if !knownEvent(EventSliceCreated) || !knownEvent(EventSliceSkippedByExecutor) {
		t.Fatal("§6.1 四名必须仍在闭集内（零回归）")
	}

	// ② 合并闭集 = 4 + 3 = 7 名；三个新名必须真的在闭集内（否则落了也会被 Emit 丢掉）
	all := AllEventNames()
	if len(all) != 7 {
		t.Fatalf("合法 event 值应 = §6.1 四名 ∪ §6.2 三名 = 7，实际 %d：%v", len(all), all)
	}
	for _, name := range want {
		if !knownEvent(name) {
			t.Fatalf("§6.2 的记录名 %q 必须被闭集认下（否则永远落不了盘）", name)
		}
	}

	// ③ 两节互不重叠、无空串无重复（防「七个」被凑数）
	seen := map[string]bool{}
	for i, n := range all {
		if strings.TrimSpace(n) == "" {
			t.Fatalf("第 %d 个是空串（闭集里不许有空条目）", i+1)
		}
		if seen[n] {
			t.Fatalf("闭集里有重复条目：%q", n)
		}
		seen[n] = true
	}

	// ④ 与升级信号闭集**交集为空**（三个闭集互不派生：事件名 / 信号 / 交接对象）
	for _, n := range TeamEventNames() {
		if IsEscalationSignal(n) {
			t.Fatalf("派活层记录名 %q 不得同时是升级信号（两个闭集互不派生）", n)
		}
		if IsSourcedHandoffTarget(n) {
			t.Fatalf("派活层记录名 %q 不得同时是交接对象（两个闭集互不派生）", n)
		}
	}
	// ⑤ 交接对象闭集：**只有**稿面逐字的那一个取值（§4.6-7 `handoffs=["user"]`）
	if tg := SourcedHandoffTargets(); len(tg) != 1 || tg[0] != "user" {
		t.Fatalf("有稿面出处的交接对象应恰好一个 %q，实际 %v", HandoffTargetUser, tg)
	}
	t.Logf("§6.1 四名 = %v ｜ §6.2 三名 = %v ｜ 合并闭集 = %v", sliceNames, got, all)
}

// ── B. 三个名字各落一行 + 原始 JSON 字段齐 ──────────────────────────────────

func TestTeamEvents_EachNameWritesOneRow(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	r := 4

	EmitSquad("member-a", "t-squad-1", OutcomeCreated, "", nil)
	EmitWorkOrder("S-wo-1", "t-wo-1", "member-b", OutcomeRejected, "test-ruleset-v9", &r)
	EmitHandoff("member-a", HandoffTargetUser, "S-wo-1", "t-wo-1", OutcomeRaised, "", nil)

	raw := readRawEvents(t)
	if len(raw) != 3 {
		t.Fatalf("三个 §6.2 记录名各落 1 行，应 3 行，实际 %d 行：%v", len(raw), raw)
	}
	evs := readEvents(t)

	// ① 事件名必须逐字是 §6.2 那三个（查**原始 JSON**，不看内存结构）
	for i, want := range []string{"squad", "work_order", "handoff"} {
		if evs[i].Event != want {
			t.Fatalf("第 %d 行事件名应为 %q，实际 %q", i+1, want, evs[i].Event)
		}
		if !strings.Contains(raw[i], `"event":"`+want+`"`) {
			t.Fatalf("第 %d 行原始 JSON 里事件名不对：%s", i+1, raw[i])
		}
	}

	// ② squad 行
	if evs[0].Outcome != OutcomeCreated || !evs[0].OK {
		t.Fatalf("squad 行结局应为 %q 且 ok=true（结局=created ⇒ ok），实际 outcome=%q ok=%v",
			OutcomeCreated, evs[0].Outcome, evs[0].OK)
	}
	if evs[0].AgentName != "member-a" || evs[0].TaskID != "t-squad-1" {
		t.Fatalf("squad 行归因不符（agent_name / task_id）：%+v", evs[0])
	}
	// 片侧字段不得凭空出现（squad 行没有片 id）—— 查原始 JSON 的字段缺席
	if strings.Contains(raw[0], `"slice_id":`) || strings.Contains(raw[0], `"target":`) {
		t.Fatalf("squad 行不该带 slice_id / target：%s", raw[0])
	}
	// 没给 R 编号 ⇒ `r` 字段整个不出现（不写 0 冒充 R0）
	if strings.Contains(raw[0], `"r":`) || evs[0].R != nil {
		t.Fatalf("squad 行没给 R 编号，不得写 r 字段：%s", raw[0])
	}

	// ③ work_order 行（带 R 编号 + 真判据版本）
	if evs[1].Outcome != OutcomeRejected || evs[1].OK {
		t.Fatalf("work_order 行结局应为 %q 且 ok=false，实际 outcome=%q ok=%v",
			OutcomeRejected, evs[1].Outcome, evs[1].OK)
	}
	if evs[1].SliceID != "S-wo-1" || evs[1].TaskID != "t-wo-1" || evs[1].AgentName != "member-b" {
		t.Fatalf("work_order 行归因不符：%+v", evs[1])
	}
	if evs[1].R == nil || *evs[1].R != 4 || evs[1].CriteriaVersion != "test-ruleset-v9" {
		t.Fatalf("work_order 行的 R 编号与判据版本应如实带上：r=%v criteria_version=%q",
			evs[1].R, evs[1].CriteriaVersion)
	}

	// ④ handoff 行（交接对象 = 出处闭集里那一个取值）
	if evs[2].Target != HandoffTargetUser || !strings.Contains(raw[2], `"target":"user"`) {
		t.Fatalf("handoff 行的交接对象应为 %q 且真落在原始 JSON 里：%s", HandoffTargetUser, raw[2])
	}
	if evs[2].Outcome != OutcomeRaised || evs[2].OK {
		t.Fatalf("handoff 行结局应为 %q 且 ok=false，实际 outcome=%q ok=%v",
			OutcomeRaised, evs[2].Outcome, evs[2].OK)
	}
	if evs[2].AgentName != "member-a" {
		t.Fatalf("handoff 行应带发起交接的成员名，实际 %q", evs[2].AgentName)
	}

	// ⑤ 硬规则 ④：detail 不得回显自由文本（本件三个出口都不接受自由文本 ⇒ 只能是固定句式）
	for i, ev := range evs {
		if !strings.Contains(ev.Detail, "§6.2") {
			t.Fatalf("第 %d 行 detail 必须点明来源（§6.2），实际 %q", i+1, ev.Detail)
		}
		if strings.Contains(ev.Detail, "请求体") && !strings.Contains(ev.Detail, "不回显") {
			t.Fatalf("第 %d 行 detail 不得回显请求体原文：%q", i+1, ev.Detail)
		}
	}
	for _, line := range raw {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("不是合法 JSON 行：%s", line)
		}
		for _, k := range []string{"event", "time", "ok", "outcome", "criteria_version", "detail"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("原始行缺必需位 %q：%s", k, line)
			}
		}
	}
	for _, line := range raw {
		t.Logf("原始行 = %s", line)
	}
}

// ── C. 判据版本：无来源 ⇒ 「未标定」；有来源 ⇒ 如实带上（不覆盖）──────────────

func TestTeamEvents_CriteriaVersionUncalibratedWhenNoSource(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	EmitSquad("member-a", "t-1", OutcomeCreated, "", nil)                             // 无来源
	EmitWorkOrder("S-1", "t-1", "member-a", OutcomeCreated, "", nil)                  // 无来源
	EmitHandoff("member-a", HandoffTargetUser, "S-1", "t-1", OutcomeCreated, "", nil) // 无来源
	EmitWorkOrder("S-2", "t-2", "member-b", OutcomeCreated, "rule-v7", nil)           // 有来源

	evs := readEvents(t)
	if len(evs) != 4 {
		t.Fatalf("应 4 行，实际 %d 行", len(evs))
	}
	for i := 0; i < 3; i++ {
		if evs[i].CriteriaVersion != CriteriaVersionUncalibrated {
			t.Fatalf("第 %d 行无来源 ⇒ 判据版本必须显式「未标定」，实际 %q", i+1, evs[i].CriteriaVersion)
		}
	}
	if CriteriaVersionUncalibrated != "未标定" {
		t.Fatalf("「无来源」的显式取值必须是字面「未标定」，实际 %q", CriteriaVersionUncalibrated)
	}
	if evs[3].CriteriaVersion != "rule-v7" {
		t.Fatalf("调用方给了真来源时必须如实带上（Emit 不覆盖），实际 %q", evs[3].CriteriaVersion)
	}
	for i, ev := range evs {
		if ev.Time == "" {
			t.Fatalf("第 %d 行缺时间（必须由 Emit 兜底填上）", i+1)
		}
	}
	t.Logf("未标定 ×3 + 真来源 ×1 = %v", []string{evs[0].CriteriaVersion, evs[1].CriteriaVersion, evs[2].CriteriaVersion, evs[3].CriteriaVersion})
}

// ── D. 交接对象闭集外 ⇒ 不落行（含近义 / 大小写 / 尾随空格 / 空）──────────────

func TestTeamEvents_HandoffTargetOutOfSourcedSetIsNotWritten(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	outOfSet := []struct {
		why    string
		target string
	}{
		{"空串（没有交接对象）", ""},
		{"只有空白", "   "},
		{"自造的成员名（稿面没给成员名形态）", "agent-b"},
		{"近义说法", "human"},
		{"大小写不同（逐字比较 ⇒ 不算命中）", "User"},
		{"尾随空格（不修剪：修剪等于悄悄放宽闭集）", "user "},
		{"闭集内取值 + 句号", "user。"},
	}
	for _, c := range outOfSet {
		EmitHandoff("member-a", c.target, "S-1", "t-1", OutcomeCreated, "", nil)
	}
	if got := readRawEvents(t); len(got) != 0 {
		t.Fatalf("交接对象不在出处闭集内 ⇒ 一律不落行，实际落了 %d 行：%v", len(got), got)
	}
	// 反侧：出处取值必须落（否则上面那条「不落」可被「什么都不落」伪证）
	EmitHandoff("member-a", HandoffTargetUser, "S-1", "t-1", OutcomeCreated, "", nil)
	got := readRawEvents(t)
	if len(got) != 1 {
		t.Fatalf("出处取值 %q 必须落 1 行，实际 %d 行：%v", HandoffTargetUser, len(got), got)
	}
	if !strings.Contains(got[0], `"target":"user"`) {
		t.Fatalf("原始行里必须真有 target=user：%s", got[0])
	}
	t.Logf("闭集外 %d 种写法全部零落行；闭集内 %q ⇒ 1 行：%s", len(outOfSet), HandoffTargetUser, got[0])
}

// ── E. 结局这一位必须有值：派活侧无名来源默认结局 ⇒ 不落行；片侧兜底**零回归** ──

func TestTeamEvents_UnknownOutcomeIsNotWrittenForTeamRecords(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// ① 派活侧三名：结局不给 / 认不出 ⇒ **不落行**（不高批注一个结局出来、不写空结局）
	bad := []string{"", "   ", "done", "SUCCESS", OutcomeCreated + " "}
	for _, o := range bad {
		EmitSquad("member-a", "t-1", o, "", nil)
		EmitWorkOrder("S-1", "t-1", "member-a", o, "", nil)
		EmitHandoff("member-a", HandoffTargetUser, "S-1", "t-1", o, "", nil)
	}
	if got := readRawEvents(t); len(got) != 0 {
		t.Fatalf("结局认不出 ⇒ 派活侧一律不落行（低基数结局这一位必须有值，不猜），实际 %d 行：%v", len(got), got)
	}

	// ② 反侧：既有的低基数结局值必须落（否则上面那条可被「什么都不落」伪证）
	for _, o := range []string{OutcomeCreated, OutcomeRejected, OutcomeRaised, OutcomeRefused, OutcomeTimeout, OutcomeSkipped} {
		EmitSquad("member-a", "t-1", o, "", nil)
	}
	if got := readRawEvents(t); len(got) != 6 {
		t.Fatalf("既有六值结局各应落 1 行，实际 %d 行", len(got))
	}

	// ③ **零回归**：片侧四名的兜底行为一字未变 —— 结局漏填时仍退回固定结局，照旧落行
	base := len(readRawEvents(t))
	Emit(Event{Event: EventSliceCreated, SliceID: "S-1"})
	Emit(Event{Event: EventSliceRejected, SliceID: "S-1"})
	Emit(Event{Event: EventSliceEscalated, SliceID: "S-1"})
	Emit(Event{Event: EventSliceSkippedByExecutor, SliceID: "S-1"})
	got := readRawEvents(t)
	if len(got) != base+4 {
		t.Fatalf("片侧四名结局漏填时必须照旧落行（零回归），应 %d 行，实际 %d 行", base+4, len(got))
	}
	evs := readEvents(t)
	wantOutcome := []string{OutcomeCreated, OutcomeRejected, OutcomeRaised, OutcomeSkipped}
	for i, want := range wantOutcome {
		ev := evs[base+i]
		if ev.Outcome != want {
			t.Fatalf("片侧第 %d 名的兜底结局应仍为 %q，实际 %q（零回归被破坏）", i+1, want, ev.Outcome)
		}
		// 片侧事件不得带上派活侧的两个新字段
		if ev.AgentName != "" || ev.Target != "" {
			t.Fatalf("片侧第 %d 名不该带 agent_name / target：%+v", i+1, ev)
		}
	}
	t.Logf("派活侧：%d 种非法结局 ⇒ 0 行 ｜ 既有六值结局 ⇒ 6 行 ｜ 片侧兜底四行 = %v", len(bad), wantOutcome)
}

// ── F. best-effort：写不进去也不 panic / 不阻断 / 不造文件 ────────────────────

func TestTeamEvents_EmitBestEffortWriteFailureDoesNotAffectVerdict(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("造堵塞文件失败：%v", err)
	}
	t.Setenv("ZERG_STATE_DIR", filepath.Join(blocker, "state")) // 父是普通文件 ⇒ MkdirAll 必失败

	// 三个出口各打一次（写全失败）+ 一个刻意非法的（闭集外目标）—— 只要不 panic、不阻断，本条即通过
	EmitSquad("member-a", "t-1", OutcomeCreated, "", nil)
	EmitWorkOrder("S-1", "t-1", "member-a", OutcomeCreated, "", nil)
	EmitHandoff("member-a", HandoffTargetUser, "S-1", "t-1", OutcomeCreated, "", nil)
	EmitHandoff("member-a", "agent-b", "S-1", "t-1", OutcomeCreated, "", nil)
	Emit(Event{Event: EventSquad, Outcome: "nonsense"})

	// 父路径是普通文件 ⇒ stat 报 ENOTDIR（不是 ENOENT）——「stat 不成功」即「没造出来」。
	if _, err := os.Stat(filepath.Join(blocker, "state", "slice-mount-events.jsonl")); err == nil {
		t.Fatal("落点写不进去时不得凭空造出文件")
	}
	if !strings.Contains(EventsFile(), "slice-mount-events.jsonl") {
		t.Fatalf("落点文件名不该变（仍是同一个文件，不新建日志面），实际 %q", EventsFile())
	}
	t.Logf("落点（写不进去的那个）=%s —— 三个出口均无返回值 ⇒ 判定侧无从被观测失败改变", EventsFile())
}

// ── G. 归因字段缺失 ⇒ 落行 + detail 点明「观测缺陷」（不静默）─────────────────

func TestTeamEvents_MissingAttributionIsFlaggedAsObservationDefect(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	EmitSquad("", "", OutcomeCreated, "", nil)                          // 成员名 + 挂板 id 都缺
	EmitWorkOrder("", "", "", OutcomeCreated, "", nil)                  // 片 id + 挂板 id + 成员名都缺
	EmitHandoff("", HandoffTargetUser, "", "", OutcomeCreated, "", nil) // 发起成员 + 片 id 都缺

	evs := readEvents(t)
	if len(evs) != 3 {
		t.Fatalf("归因缺失仍应落行（观测缺陷不静默），应 3 行，实际 %d 行", len(evs))
	}
	for i, ev := range evs {
		if !strings.Contains(ev.Detail, "观测缺陷") {
			t.Fatalf("第 %d 行归因缺失时 detail 必须点明观测缺陷：%q", i+1, ev.Detail)
		}
	}
	if !strings.Contains(evs[1].Detail, "task_id") || !strings.Contains(evs[1].Detail, "§4.2-2") {
		t.Fatalf("work_order 缺 task_id 时必须逐字带上出处（§4.2-2「无 task_id 的委派一律拒绝」）：%q", evs[1].Detail)
	}
	if !strings.Contains(evs[0].Detail, "§2.3-14") {
		t.Fatalf("缺 agent_name 时必须指出出处 §2.3-14：%q", evs[0].Detail)
	}
	t.Logf("缺归因三行 detail 均点明观测缺陷；示例：%s", evs[1].Detail)
}
