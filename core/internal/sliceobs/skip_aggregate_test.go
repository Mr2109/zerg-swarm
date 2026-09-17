// skip_aggregate_test.go — B 项⑥ 用例：**越片统计的聚合口径**（纯函数）+ 标定读数「缺键 ⇒ 未标定」。
//
// 覆盖：
//
//	G. 聚合：0 条 / 多条 / 混入其它事件**不计**（含闭集外的名字）/ 按片拆分 / 缺 slice_id 单列 / 确定性
//	H. **缺阈值键 ⇒ 未标定**：缺键、只有一个键、值不合法（junk / 负数）、档案不存在 —— 一律「未标定」，
//	   且此时 `BreachesSkipMax == nil`（**判不了 ≠ 没超**，不猜 ✗）
//	I. 键在且值合法 ⇒ **如实回显** + 阈值判定（数字来自**用例夹具**，不是标定值 —— 真档案里没有这两个键）
//	J. 真档案：默认路径的读数必须**诚实**（键缺 ⇔ 未标定；键在 ⇒ 必须给出数字）+ 打印现读
package sliceobs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCalibFixture — 造一份**用例夹具**档案（不是标定：值由用例自定，只为钉住「读键」这条路径）。
func writeCalibFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "task-budget-calib.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具档案失败：%v", err)
	}
	return path
}

// ── G. 聚合：只数越片事件 ───────────────────────────────────────────────────

func TestSkipAggregate_CountsOnlySkippedByExecutorEvents(t *testing.T) {
	calib := SliceCalib{} // 未读档案 ⇒ 两处读数 nil（未标定）

	// ① 0 条
	st := AggregateSkips(nil, calib)
	if st.SkipActions != 0 || st.OtherEvents != 0 || len(st.BySlice) != 0 {
		t.Fatalf("空序列必须聚合成全 0，实际 %+v", st)
	}
	if st.BreachesSkipMax != nil {
		t.Fatal("阈值未标定 ⇒ 必须「判不了」（nil），不得默认 false")
	}

	// ② 多条 + ③ 混入其它事件（含闭集外的名字：人工造的输入里可能真有 —— 也不许计进来）
	events := []Event{
		{Event: EventSliceCreated, SliceID: "S1"},
		{Event: EventSliceSkippedByExecutor, SliceID: "S2"},
		{Event: EventSliceSkippedByExecutor, SliceID: "S2"},
		{Event: EventSliceRejected, SliceID: "S2", Code: "POST_R10"},
		{Event: EventSliceEscalated, SliceID: "S9", Signal: SignalSecondSendbackOfSameSlice},
		{Event: "slice_exploded", SliceID: "S3"}, // 闭集外的名字（本该落不了盘；聚合侧仍要显式判一次）
		{Event: EventSliceSkippedByExecutor, SliceID: "S1"},
		{Event: EventSliceSkippedByExecutor, SliceID: ""}, // 缺 slice_id ⇒ 计入越片数，但单列缺陷
	}
	st = AggregateSkips(events, calib)
	if st.SkipActions != 4 {
		t.Fatalf("越片动作数应为 4（只数 slice_skipped_by_executor），实际 %d（%+v）", st.SkipActions, st)
	}
	if st.OtherEvents != 4 {
		t.Fatalf("不计入的其它事件应为 4，实际 %d", st.OtherEvents)
	}
	if st.SkipsWithoutSliceID != 1 {
		t.Fatalf("缺 slice_id 的越片事件应为 1 条（单列不静默），实际 %d", st.SkipsWithoutSliceID)
	}
	wantBySlice := []SliceSkipCount{{SliceID: "", Skips: 1}, {SliceID: "S1", Skips: 1}, {SliceID: "S2", Skips: 2}}
	if len(st.BySlice) != len(wantBySlice) {
		t.Fatalf("按片拆分应 %d 片，实际 %d 片：%+v", len(wantBySlice), len(st.BySlice), st.BySlice)
	}
	for i, want := range wantBySlice {
		if st.BySlice[i] != want {
			t.Fatalf("按片拆分第 %d 项应为 %+v（SliceID 升序、空 id 最前），实际 %+v", i, want, st.BySlice[i])
		}
	}
	// ④ 确定性 + 纯：同输入必同输出；入参不被改
	again := AggregateSkips(events, calib)
	if again.SkipActions != st.SkipActions || again.OtherEvents != st.OtherEvents || len(again.BySlice) != len(st.BySlice) {
		t.Fatalf("同输入必须同输出（纯函数）：%+v vs %+v", again, st)
	}
	for i := range st.BySlice {
		if again.BySlice[i] != st.BySlice[i] {
			t.Fatalf("同输入必须同输出（按片拆分第 %d 项不一致）", i)
		}
	}
	if len(events) != 8 {
		t.Fatalf("聚合不得改入参，实际 len=%d", len(events))
	}
	if st.Calib.Path != "" || st.Calib.SkipMax != nil || st.Calib.RestateEveryN != nil {
		t.Fatalf("聚合不得凭空改标定读数：%+v", st.Calib)
	}
	// 未标定 ⇒ 两处读数都是「未标定」+ 判不了阈值
	if st.SkipMaxText() != UncalibratedText || st.RestateEveryNText() != UncalibratedText {
		t.Fatalf("未标定时的文本必须是 %q，实际 skip_max=%q restate_n=%q",
			UncalibratedText, st.SkipMaxText(), st.RestateEveryNText())
	}
	if UncalibratedText != "未标定" {
		t.Fatalf("「无来源」的显式取值必须是字面「未标定」，实际 %q", UncalibratedText)
	}
	t.Logf("越片动作数=%d 其它事件=%d 缺 id=%d 按片=%+v skip_max=%s restate_n=%s",
		st.SkipActions, st.OtherEvents, st.SkipsWithoutSliceID, st.BySlice, st.SkipMaxText(), st.RestateEveryNText())
}

// ── H. 缺阈值键 ⇒ 未标定（且不猜）──────────────────────────────────────────

func TestSkipAggregate_UncalibratedWhenThresholdKeysMissing(t *testing.T) {
	skips := []Event{
		{Event: EventSliceSkippedByExecutor, SliceID: "S1"},
		{Event: EventSliceSkippedByExecutor, SliceID: "S1"},
		{Event: EventSliceSkippedByExecutor, SliceID: "S1"},
	}

	// ① 照真档案的键集，但**不含**本件要读的两个键（= 本机现状）
	path := writeCalibFixture(t, strings.Join([]string{
		"model: Qwen3.8-27B",
		"calib_runs: 3",
		"measured_at: 2026-09-16T19:42:19Z",
		"steps_max: 3",
		"tool_calls_max: 2",
		"wall_s_max: 43.6",
		"assistant_tokens_est_max: 118",
		"",
	}, "\n"))
	c := LoadSliceCalibFrom(path)
	if c.ReadErr != "" {
		t.Fatalf("夹具档案可读，不该有读失败分类：%q", c.ReadErr)
	}
	if c.SkipMax != nil || c.RestateEveryN != nil {
		t.Fatalf("档案里没有这两个键 ⇒ 读数必须为 nil（不猜），实际 %+v", c)
	}
	if c.Calibrated() {
		t.Fatalf("两处读数都缺 ⇒ 不得自称已标定：%+v", c)
	}
	st := AggregateSkips(skips, c)
	if st.SkipMaxText() != "未标定" || st.RestateEveryNText() != "未标定" {
		t.Fatalf("缺键 ⇒ 必须写「未标定」，实际 %q / %q", st.SkipMaxText(), st.RestateEveryNText())
	}
	if st.BreachesSkipMax != nil {
		t.Fatalf("缺键 ⇒ **判不了**（不得默认 false 说没超），实际 %v", *st.BreachesSkipMax)
	}

	// ② 只给一个键 ⇒ 另一个仍是「未标定」（一处缺就算未标定）
	c2 := LoadSliceCalibFrom(writeCalibFixture(t, "slice_skip_max: 1\n"))
	if c2.SkipMax == nil || *c2.SkipMax != 1 || c2.RestateEveryN != nil {
		t.Fatalf("只给 slice_skip_max 时：该键应回显 1、另一个应 nil，实际 %+v", c2)
	}
	if c2.RestateEveryNText() != "未标定" || c2.Calibrated() {
		t.Fatalf("只给一个键不得自称已标定：%+v", c2)
	}

	// ③ 键在但值认不出（junk / 负数 / 带小数）⇒ 一律「未标定」：**不截断、不四舍五入、不取默认值**
	c3 := LoadSliceCalibFrom(writeCalibFixture(t,
		"slice_skip_max: abc\nrestate_every_n_steps: -3\n"))
	if c3.SkipMax != nil || c3.RestateEveryN != nil {
		t.Fatalf("值认不出 ⇒ 必须 nil（不猜），实际 %+v", c3)
	}
	if c3.SkipMaxText() != "未标定" {
		t.Fatalf("值认不出 ⇒ 必须「未标定」，实际 %q", c3.SkipMaxText())
	}
	c3b := LoadSliceCalibFrom(writeCalibFixture(t,
		"slice_skip_max: 1.5\nrestate_every_n_steps: [5]\n"))
	if c3b.SkipMax != nil || c3b.RestateEveryN != nil {
		t.Fatalf("小数/列表值 ⇒ 必须 nil（不截断、不猜），实际 %+v", c3b)
	}

	// ④ 档案不存在 / 路径为空 ⇒ 未标定（且给低基数分类，不 panic）
	c4 := LoadSliceCalibFrom(filepath.Join(t.TempDir(), "not-there.yaml"))
	if c4.SkipMax != nil || c4.RestateEveryN != nil || c4.ReadErr != calibReadErrNotExist {
		t.Fatalf("档案不存在 ⇒ 读数 nil + 分类 %q，实际 %+v", calibReadErrNotExist, c4)
	}
	c5 := LoadSliceCalibFrom("")
	if c5.SkipMax != nil || c5.ReadErr != calibReadErrEmpty {
		t.Fatalf("空路径 ⇒ nil + 分类 %q，实际 %+v", calibReadErrEmpty, c5)
	}
	t.Logf("缺键/值认不出/档案不存在 ⇒ 一律「未标定」；ReadErr 分类：not_exist=%q empty=%q",
		c4.ReadErr, c5.ReadErr)
}

// ── I. 键在且值合法 ⇒ 如实回显 + 阈值判定（值为**用例夹具**，非标定值）──────────

func TestSkipAggregate_FixtureKeysAreReadVerbatim(t *testing.T) {
	// ⚠️ 这两个数是**用例夹具**（2 / 5），**不是**标定值：真档案里没有这两个键（见用例 H / J）。
	path := writeCalibFixture(t, "slice_skip_max: 2\nrestate_every_n_steps: 5\n")
	c := LoadSliceCalibFrom(path)
	if c.SkipMaxText() != "2" || c.RestateEveryNText() != "5" {
		t.Fatalf("键在 ⇒ 必须如实回显，实际 %q / %q", c.SkipMaxText(), c.RestateEveryNText())
	}
	if !c.Calibrated() {
		t.Fatalf("两处读数都有来源 ⇒ 已标定：%+v", c)
	}

	// 越片数 = 阈值 ⇒ 不算超（**严格大于**才算超）；0 条也不算超
	for _, tc := range []struct {
		n    int
		want bool
	}{
		{0, false}, {2, false}, {3, true}, {10, true},
	} {
		events := make([]Event, 0, tc.n)
		for i := 0; i < tc.n; i++ {
			events = append(events, Event{Event: EventSliceSkippedByExecutor, SliceID: "S1"})
		}
		st := AggregateSkips(events, c)
		if st.SkipActions != tc.n {
			t.Fatalf("越片数应为 %d，实际 %d", tc.n, st.SkipActions)
		}
		if st.BreachesSkipMax == nil {
			t.Fatalf("阈值已标定（夹具 2）⇒ 必须给判定，越片数=%d", tc.n)
		}
		if *st.BreachesSkipMax != tc.want {
			t.Fatalf("越片数 %d 对阈值 2 的判定应为 %v，实际 %v", tc.n, tc.want, *st.BreachesSkipMax)
		}
	}
	t.Logf("夹具档案（值 2 / 5，非标定值）⇒ 如实回显 + 阈值判定（>2 才算超）")
}

// ── J. 默认路径 + 真档案读数必须诚实（键缺 ⇔ 未标定）────────────────────────

func TestSkipAggregate_DefaultPathAndRealArchiveReadingsAreHonest(t *testing.T) {
	realHome := os.Getenv("HOME")

	// ① 默认路径的形状（不猜路径；拼出来必须指向 ~/.zerg/egg-profiles/task-budget-calib.yaml）
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	want := filepath.Join(fakeHome, ".zerg", "egg-profiles", "task-budget-calib.yaml")
	if got := DefaultSliceCalibPath(); got != want {
		t.Fatalf("默认档案路径应为 %q，实际 %q", want, got)
	}

	// ② 真档案（**只读**：不写、不迁移、不造目录）现读
	t.Setenv("HOME", realHome)
	realPath := DefaultSliceCalibPath()
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Skipf("本机没有标定档案（%s）⇒ 跳过真档案读数（不是失败：%v）", realPath, err)
	}
	hasSkipMax := strings.Contains(string(data), CalibKeySliceSkipMax)
	hasN := strings.Contains(string(data), CalibKeyRestateEveryNSteps)
	c := LoadSliceCalibFrom(realPath)
	if c.ReadErr != "" {
		t.Fatalf("真档案读失败了：%q", c.ReadErr)
	}
	// 诚实性：键在 ⇔ 读数非 nil（键在却报未标定 = 读漏；键缺却给出数字 = 编造）
	if hasSkipMax != (c.SkipMax != nil) {
		t.Fatalf("键 %q 存在=%v，但读数 nil=%v —— 不诚实", CalibKeySliceSkipMax, hasSkipMax, c.SkipMax == nil)
	}
	if hasN != (c.RestateEveryN != nil) {
		t.Fatalf("键 %q 存在=%v，但读数 nil=%v —— 不诚实", CalibKeyRestateEveryNSteps, hasN, c.RestateEveryN == nil)
	}
	if !hasSkipMax && c.SkipMaxText() != "未标定" {
		t.Fatalf("真档案缺键 ⇒ 必须「未标定」，实际 %q", c.SkipMaxText())
	}
	t.Logf("真档案：path=%s 键%s=%v 键%s=%v ⇒ skip_max=%s restate_every_n=%s",
		realPath, CalibKeySliceSkipMax, hasSkipMax, CalibKeyRestateEveryNSteps, hasN,
		c.SkipMaxText(), c.RestateEveryNText())
}
