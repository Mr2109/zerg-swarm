// sliceobs_test.go — B 项⑤ 机制面用例（事件名闭集 / 未标定 / best-effort / 越片出口）。
//
// 覆盖：
//
//	A. 事件名闭集**逐字**对齐设计稿 §6.1（防事件名漂移——名字漂了统计脚本就瞎）
//	B. 认不出的 event 名 ⇒ **不落行**（不猜、不产生垃圾基数）
//	C. 判据版本无来源 ⇒ 显式「未标定」；调用方给了真来源 ⇒ 如实带上（**不覆盖**）
//	D. `r` 编号缺席时字段整个不出现（不与「R0 是合法编号」混淆）—— 查**原始 JSON**，不是查结构体
//	E. best-effort：落点写不进去时 Emit 不 panic、不阻断
package sliceobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readRawEvents — 真读回落盘的**原始行**（观测面取证：不看返回值、不看内存结构）。
func readRawEvents(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(EventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读片事件文件失败: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// readEvents — 解析成结构体（顺带断言每行都是合法单行 JSON）。
func readEvents(t *testing.T) []Event {
	t.Helper()
	var out []Event
	for _, line := range readRawEvents(t) {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("片事件不是合法 JSON 行: %q (%v)", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// ── A. 事件名闭集逐字对齐设计稿 §6.1 ────────────────────────────────────────

func TestSliceEvents_NameSetMatchesDesignDoc(t *testing.T) {
	// 期望值**写成字面**（不引用常量）：常量的值本身就是要被钉住的东西 ——
	// 拿常量断言常量等于自证。设计稿 v2.1 §6.1 / v1.1 §4 逐字四名。
	want := []string{"slice_created", "slice_rejected", "slice_escalated", "slice_skipped_by_executor"}
	got := EventNames()
	if len(got) != len(want) {
		t.Fatalf("事件名闭集应 %d 个（设计稿 §6.1），实际 %d 个：%v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("事件名闭集第 %d 个与设计稿 §6.1 不符：期望 %q，实际 %q", i, want[i], got[i])
		}
	}
	// 旧名归并成 alias（不再是 event 值）
	if AliasSliceMount != "slice_mount" {
		t.Fatalf("旧名别名应为字面 slice_mount，实际 %q", AliasSliceMount)
	}
	if knownEvent(AliasSliceMount) {
		t.Fatal("旧名 slice_mount 已归并 ⇒ 不得再在事件名闭集内（否则等于一套机制两个名字空间）")
	}
	t.Logf("事件名闭集 = %v（+ 归并别名 %q）", got, AliasSliceMount)
}

// ── B. 认不出的 event 名 ⇒ 不落行 ───────────────────────────────────────────

func TestSliceEvents_UnknownEventNameIsNotWritten(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	Emit(Event{Event: "slice_mount"})    // 归并前的旧名：不再是 event 值 ⇒ 不得落
	Emit(Event{Event: "slice_exploded"}) // 闭集外的自造名 ⇒ 不得落
	Emit(Event{Event: ""})               // 空名 ⇒ 不得落
	if got := readRawEvents(t); len(got) != 0 {
		t.Fatalf("闭集外的事件名一律不落，实际落了 %d 行：%v", len(got), got)
	}
	// 反侧：闭集内的名字必须落（否则上面那条「不落」可被「什么都不落」伪证）
	Emit(Event{Event: EventSliceCreated, SliceID: "S1"})
	if got := readRawEvents(t); len(got) != 1 {
		t.Fatalf("闭集内的 slice_created 必须落 1 行，实际 %d 行", len(got))
	}
}

// ── C. 判据版本：无来源 ⇒ 「未标定」；有来源 ⇒ 如实带上 ─────────────────────

func TestSliceEvents_CriteriaVersionUncalibratedWhenNoSource(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	// ① 无来源（空串）⇒ 显式「未标定」，**不编造**
	Emit(Event{Event: EventSliceCreated, SliceID: "S1"})
	// ② 越片出口同样无来源 ⇒ 未标定
	EmitSliceSkippedByExecutor("S2", "t-2", "", nil)
	// ③ 调用方给了真来源（将来检查器落版本串）⇒ 如实带上，Emit **不覆盖**
	EmitSliceSkippedByExecutor("S3", "t-3", "test-ruleset-v9", nil)

	evs := readEvents(t)
	if len(evs) != 3 {
		t.Fatalf("应有 3 行，实际 %d 行", len(evs))
	}
	for i, ev := range evs {
		if ev.Time == "" {
			t.Fatalf("第 %d 行缺时间（时间必须由 Emit 兜底填上）", i)
		}
	}
	if evs[0].CriteriaVersion != CriteriaVersionUncalibrated {
		t.Fatalf("无来源时判据版本必须写「%s」，实际 %q", CriteriaVersionUncalibrated, evs[0].CriteriaVersion)
	}
	if CriteriaVersionUncalibrated != "未标定" {
		t.Fatalf("「无来源」的显式取值必须是字面「未标定」，实际 %q", CriteriaVersionUncalibrated)
	}
	if evs[1].CriteriaVersion != "未标定" {
		t.Fatalf("越片出口无来源时也必须写「未标定」，实际 %q", evs[1].CriteriaVersion)
	}
	if evs[2].CriteriaVersion != "test-ruleset-v9" {
		t.Fatalf("调用方给了真来源时必须如实带上（不覆盖）：实际 %q", evs[2].CriteriaVersion)
	}
	t.Logf("① %q ② %q ③ %q", evs[0].CriteriaVersion, evs[1].CriteriaVersion, evs[2].CriteriaVersion)
}

// ── D. 越片出口：字段齐 + r 缺席时字段整个不出现（查原始 JSON）──────────────

func TestSliceEvents_SkippedByExecutorEntryPoint(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	r := 7
	EmitSliceSkippedByExecutor("S4-C2", "t-skip-1", "", &r)  // 有 R 编号
	EmitSliceSkippedByExecutor("S4-C3", "t-skip-2", "", nil) // 无 R 编号

	raw := readRawEvents(t)
	if len(raw) != 2 {
		t.Fatalf("越片出口应落 2 行，实际 %d 行：%v", len(raw), raw)
	}
	evs := readEvents(t)
	ev := evs[0]
	if ev.Event != "slice_skipped_by_executor" {
		t.Fatalf("事件名应为 slice_skipped_by_executor，实际 %q", ev.Event)
	}
	if ev.Outcome != OutcomeSkipped || ev.OK {
		t.Fatalf("越片事件的结局应为 %q 且 ok=false，实际 outcome=%q ok=%v", OutcomeSkipped, ev.Outcome, ev.OK)
	}
	if ev.SliceID != "S4-C2" || ev.TaskID != "t-skip-1" {
		t.Fatalf("片/任务归因不符：%+v", ev)
	}
	if ev.R == nil || *ev.R != 7 {
		t.Fatalf("R 编号应如实带 7，实际 %v", ev.R)
	}
	if ev.CriteriaVersion != "未标定" {
		t.Fatalf("判据版本应「未标定」，实际 %q", ev.CriteriaVersion)
	}
	if !strings.Contains(ev.Detail, "越片动作") || !strings.Contains(ev.Detail, "POST_") {
		t.Fatalf("detail 必须可行动（点明越片动作 + 归因层）：%q", ev.Detail)
	}
	// 无 R 编号时：`r` 字段**整个不出现**（用指针而非 int ⇒ R0 与「没有」不混淆）
	if strings.Contains(raw[1], `"r":`) {
		t.Fatalf("没有 R 编号时不得写 r 字段（不写 0 冒充 R0）：%s", raw[1])
	}
	if evs[1].R != nil {
		t.Fatalf("没有 R 编号时 R 应为 nil，实际 %v", *evs[1].R)
	}
	// 硬规则 ④：detail 里不得出现调用方自由文本（这里调用方没给 ⇒ 只能是固定句式）
	if strings.Contains(ev.Detail, "S4-C2 的请求体") {
		t.Fatal("detail 不得回显请求体原文")
	}
	t.Logf("原始行①=%s", raw[0])
	t.Logf("原始行②=%s", raw[1])
}

// ── E. best-effort：写不进去也不 panic / 不阻断 ─────────────────────────────

func TestSliceEvents_EmitIsBestEffortAndNeverPanics(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("造堵塞文件失败：%v", err)
	}
	t.Setenv("ZERG_STATE_DIR", filepath.Join(blocker, "state")) // 父是普通文件 ⇒ MkdirAll 必失败

	// 四种事件名各打一次（写全失败）—— 只要不 panic、不阻断，本条即通过
	Emit(Event{Event: EventSliceCreated, SliceID: "S1"})
	Emit(Event{Event: EventSliceRejected, SliceID: "S1", Code: "SLICE_MISSING_ID"})
	Emit(Event{Event: EventSliceEscalated, SliceID: "S1"})
	EmitSliceSkippedByExecutor("S1", "t1", "", nil)

	// 父路径是普通文件 ⇒ stat 报 ENOTDIR（不是 ENOENT）——「stat 不成功」即「没造出来」。
	if _, err := os.Stat(filepath.Join(blocker, "state", "slice-mount-events.jsonl")); err == nil {
		t.Fatal("落点写不进去时不得凭空造出文件")
	}
	if strings.Contains(EventsFile(), "slice-mount-events.jsonl") == false {
		t.Fatalf("落点文件名不该变（归并口径：仍是 B 项③ 的同一个文件），实际 %q", EventsFile())
	}
	t.Logf("落点（写不进去的那个）=%s", EventsFile())
}
