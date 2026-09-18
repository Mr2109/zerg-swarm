// cost_table_test.go — B 项 **B10** 用例（其二）：**四指标同表可聚合**（+ 定层同表）。
//
// 覆盖（正反两侧都要）：
//
//	F. 四指标**同表**：显存 / 调用次数 / token / 墙钟（稿面字面四项，不增删）+ 视图顺序固定
//	G. **确定性 + 纯**：同输入两次聚合逐字节相同；聚合**不改入参**；空输入 ⇒ 四项「未标定」
//	H. **缺读数/非法读数 ⇒ 未标定**（台账没给显存、无 token 键、负显存/NaN）；「数出来的 0」照写
//	I. 读观测文件：键名逐字对上 obs.go；坏行**单列计数**不静默；端到端**可复算**（同输入同表）
package sliceobs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// pInt64 / pI / pF — 造读数的测试小工具（nil 语义 = 没读数）。
func pInt64(v int64) *int64 { return &v }
func pI(v int) *int         { return &v }
func pF(v float64) *float64 { return &v }

// ── F. 四指标同表 ───────────────────────────────────────────────────────

func TestB10_CostTable_FourMetricsSameTable(t *testing.T) {
	samples := []CostSample{
		{Kind: "turn", Session: "s1", TokensIn: pInt64(100), TokensOut: pInt64(50), WallMS: pInt64(1000)},
		{Kind: "turn", Session: "s1", TokensIn: pInt64(200), WallMS: pInt64(2000)},
		{Kind: "tool", Session: "s1"},
		{Kind: "tool", Session: "s1"},
		{Kind: "tool", Session: "s2"},
		{Kind: "compact", Session: "s2", TokensIn: pInt64(10), TokensOut: pInt64(5)},
		{Kind: "turn", Session: "s2", WallMS: pInt64(500)},
		{Kind: "weird_kind", Session: "s2"}, // 口径外的 kind ⇒ 单列计数
	}
	calib := BudgetCalib{Path: "fixture", Items: []BudgetValue{
		{Key: BudgetKeyStepsMax, Value: pF(3), Layer: LayerArchive},
	}}

	table := AggregateCost(samples, pF(12.5), calib)

	// ① 四个指标都在**同一张表**的同一行里（稿面字面四项，顺序 = 显存 / 调用次数 / token / 墙钟）
	wantCols := []string{"12.5", "3", "365", "3500"}
	if got := table.Total.ColumnTexts(); !reflect.DeepEqual(got, wantCols) {
		t.Fatalf("四指标同表一行应为 %v（显存/调用次数/token/墙钟），实际 %v", wantCols, got)
	}
	// ② 调用次数只数 kind=="tool"
	if table.Total.Calls == nil || *table.Total.Calls != 3 {
		t.Fatalf("调用次数应为 3，实际 %q", table.Total.CallsText())
	}
	// ③ 口径外的 kind 单列、**不计进**调用次数
	if table.OtherKind != 1 {
		t.Fatalf("口径外 kind 应计 1 条，实际 %d", table.OtherKind)
	}
	// ④ 按会话拆开：两行、session 升序、各带三项（显存恒 nil —— 机器级不按会话切）
	if len(table.BySession) != 2 {
		t.Fatalf("应拆成 2 行，实际 %d 行：%+v", len(table.BySession), table.BySession)
	}
	if table.BySession[0].Session != "s1" || table.BySession[1].Session != "s2" {
		t.Fatalf("按会话拆开必须升序，实际 %q / %q", table.BySession[0].Session, table.BySession[1].Session)
	}
	if got := table.BySession[0].Metrics.ColumnTexts(); !reflect.DeepEqual(got, []string{UncalibratedText, "2", "350", "3000"}) {
		t.Fatalf("s1 行应为 [未标定 2 350 3000]，实际 %v", got)
	}
	if got := table.BySession[1].Metrics.ColumnTexts(); !reflect.DeepEqual(got, []string{UncalibratedText, "1", "15", "500"}) {
		t.Fatalf("s2 行应为 [未标定 1 15 500]，实际 %v", got)
	}
	// ⑤ 定层与本表同表（§6.3 两句话同表）⇒ 表里就带着每项的 definition_layer
	if got := table.Calib.ItemLayerText(BudgetKeyStepsMax); got != "档案" {
		t.Fatalf("成本表应带上预算项的定层（该为「档案」），实际 %q", got)
	}
	t.Logf("四指标同表：显存=%s 调用次数=%s token=%s 墙钟=%s（按会话 %d 行）",
		table.Total.VRAMText(), table.Total.CallsText(), table.Total.TokensText(), table.Total.WallText(), len(table.BySession))
}

// ── G. 确定性 + 纯 ──────────────────────────────────────────────────────

func TestB10_CostTable_Deterministic_And_Pure(t *testing.T) {
	samples := []CostSample{
		{Kind: "tool", Session: "b"},
		{Kind: "turn", Session: "a", TokensIn: pInt64(5), TokensOut: pInt64(6), WallMS: pInt64(70)},
		{Kind: "tool", Session: ""}, // 没带会话 id ⇒ 空 id 组（排最前），不静默
	}
	before := make([]CostSample, len(samples))
	copy(before, samples)

	first := AggregateCost(samples, pF(1.5), BudgetCalib{})
	second := AggregateCost(samples, pF(1.5), BudgetCalib{})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("同输入两次聚合结果不同（不可复算）：\n%+v\n%+v", first, second)
	}
	if !reflect.DeepEqual(samples, before) {
		t.Fatalf("聚合改了入参（必须纯）：%+v vs %+v", samples, before)
	}
	if first.BySession[0].Session != "" {
		t.Fatalf("空 session 组应排最前（升序），实际首行 %q", first.BySession[0].Session)
	}

	// 空输入 ⇒ 四项**全未标定**（「空的输入」不是「实测到 0」）
	empty := AggregateCost(nil, nil, BudgetCalib{})
	for i, got := range empty.Total.ColumnTexts() {
		if got != UncalibratedText {
			t.Fatalf("空输入第 %d 列应为「%s」，实际 %q", i+1, UncalibratedText, got)
		}
	}
	if empty.Calibrated() {
		t.Fatalf("空输入 + 空档案 ⇒ 不得报「已标定」")
	}
	if len(empty.BySession) != 0 {
		t.Fatalf("空输入不得产出会话行，实际 %d 行", len(empty.BySession))
	}
	t.Logf("确定性：两次聚合 DeepEqual；入参未被改；空输入 ⇒ 四项「%s」", UncalibratedText)
}

// ── H. 缺读数 / 非法读数 ⇒ 未标定；数出来的 0 照写 ──────────────────────

func TestB10_CostTable_MissingOrIllegalReadingsAreUncalibrated(t *testing.T) {
	// ① 台账没给显存 ⇒ 该列未标定（其余三列照给）
	table := AggregateCost([]CostSample{{Kind: "tool", Session: "s"}}, nil, BudgetCalib{})
	if got := table.Total.VRAMText(); got != UncalibratedText {
		t.Fatalf("没给显存 ⇒ 应为「%s」，实际 %q", UncalibratedText, got)
	}
	// ② 非法显存（负数 / NaN）⇒ 同样未标定（不取绝对值、不当 0）
	for _, bad := range []float64{-1, -0.5} {
		if got := AggregateCost([]CostSample{{Kind: "tool"}}, pF(bad), BudgetCalib{}).Total.VRAMText(); got != UncalibratedText {
			t.Fatalf("非法显存 %v ⇒ 应为「%s」，实际 %q", bad, UncalibratedText, got)
		}
	}
	// ③ 该会话只有调用记录：token / 墙钟**没有读数** ⇒ 未标定；调用次数是**数出来的**（照写 0）
	only := AggregateCost([]CostSample{{Kind: "turn", Session: "s"}}, pF(1), BudgetCalib{})
	if got := only.Total.TokensText(); got != UncalibratedText {
		t.Fatalf("没有一条记录带 token 键 ⇒ 应为「%s」，实际 %q", UncalibratedText, got)
	}
	if got := only.Total.WallText(); got != UncalibratedText {
		t.Fatalf("没有一条记录带 total_ms ⇒ 应为「%s」，实际 %q", UncalibratedText, got)
	}
	if got := only.Total.CallsText(); got != "0" {
		t.Fatalf("调用次数是数出来的 0（定值）⇒ 应写 \"0\"，实际 %q", got)
	}
	// ④ 表级判定：四指标**都**要读到 + 预算项**都**要有层 ⇒ 才算已标定
	allItems := BudgetCalib{Path: "fixture", Items: []BudgetValue{
		{Key: BudgetKeyStepsMax, Value: pF(3), Layer: LayerArchive},
		{Key: BudgetKeyToolCallsMax, Value: pF(2), Layer: LayerArchive},
		{Key: BudgetKeyWallSMax, Value: pF(43.6), Layer: LayerArchive},
		{Key: BudgetKeyAssistantTokensMax, Value: pF(118), Layer: LayerArchive},
	}}
	full := AggregateCost([]CostSample{{Kind: "tool", Session: "s", TokensIn: pInt64(1), WallMS: pInt64(1)}}, pF(1), allItems)
	if !full.Calibrated() {
		t.Fatalf("四项读数齐 + 四个预算项都有定层 ⇒ 应报「已标定」")
	}
	noLayer := BudgetCalib{Path: "fixture", Items: []BudgetValue{{Key: BudgetKeyStepsMax, Value: pF(3)}}} // 只有值、没有层
	if partial := AggregateCost([]CostSample{{Kind: "tool", Session: "s", TokensIn: pInt64(1), WallMS: pInt64(1)}}, pF(1), noLayer); partial.Calibrated() {
		t.Fatalf("预算项缺定层 ⇒ 整表不得报「已标定」")
	}
	if only.Calibrated() {
		t.Fatalf("token / 墙钟没有读数 ⇒ 整表不得报「已标定」（缺一列就不算）")
	}
	t.Logf("缺读数/非法读数 ⇒ 「%s」；数出来的 0 照写；表级判定要求四项 + 定层都在", UncalibratedText)
}

// ── I. 读观测文件（键名逐字对上 obs.go）+ 端到端可复算 ────────────────────

func TestB10_LoadCostSamplesFrom_ObsKeysAndBadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat_obs.jsonl")
	lines := []string{
		// 现网同形的 turn 行（token 在 gen_ai.usage.*：obs.go:249-250「计费口径」）
		`{"ts":"2026-09-16T20:33:55+08:00","kind":"turn","session":"s1","round":1,"turn":{"first_byte_ms":2516,"total_ms":2521,"chunks":167},"gen_ai.usage.input_tokens":100,"gen_ai.usage.output_tokens":20}`,
		// 现网同形的 tool 行（obs.go:142）
		`{"ts":"2026-09-16T22:10:42+08:00","kind":"tool","session":"s1","round":1,"result":"err","tool":"doc_search"}`,
		// 压缩行：token 在 tokens_in/out（obs.go:133-134）
		`{"ts":"2026-09-16T22:11:00+08:00","kind":"compact","session":"s2","tokens_in":7,"tokens_out":3,"dur_ms":900}`,
		`{"kind":"turn","session":"s2","turn":{"total_ms":100}}`,
		`{这不是 JSON`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("造夹具失败：%v", err)
	}

	samples, skipped, err := LoadCostSamplesFrom(path)
	if err != nil {
		t.Fatalf("读夹具失败：%v", err)
	}
	if skipped != 1 {
		t.Fatalf("坏行应单列计数为 1，实际 %d", skipped)
	}
	if len(samples) != 4 {
		t.Fatalf("应读出 4 条样本，实际 %d", len(samples))
	}
	if samples[0].TokensIn == nil || *samples[0].TokensIn != 100 || samples[0].WallMS == nil || *samples[0].WallMS != 2521 {
		t.Fatalf("turn 行应取 gen_ai.usage.input_tokens=100 + turn.total_ms=2521，实际 %+v", samples[0])
	}
	if samples[2].TokensIn == nil || *samples[2].TokensIn != 7 {
		t.Fatalf("压缩行应取 tokens_in=7（gen_ai 缺席），实际 %+v", samples[2])
	}

	// 默认路径：走既有解析点（ZERG_STATE_DIR 覆盖），文件名逐字 = chat_obs.jsonl
	t.Setenv("ZERG_STATE_DIR", dir)
	if got, want := DefaultCostObsPath(), filepath.Join(dir, "chat_obs.jsonl"); got != want {
		t.Fatalf("默认观测路径应为 %q，实际 %q", want, got)
	}
	_, _, err = LoadCostSamplesFrom(DefaultCostObsPath())
	if err != nil {
		t.Fatalf("默认路径应能读到夹具：%v", err)
	}

	// 端到端**可复算**：读文件 → 聚合 → 同表四指标（金额级断言）
	calibPath := filepath.Join(dir, "task-budget-calib.yaml")
	if werr := os.WriteFile(calibPath, []byte("steps_max: 3\ntool_calls_max: 2\nwall_s_max: 43.6\nassistant_tokens_est_max: 118\n"), 0o644); werr != nil {
		t.Fatalf("造档案夹具失败：%v", werr)
	}
	table := AggregateCost(samples, pF(24.0), LoadBudgetCalibFrom(calibPath))
	want := []string{"24", "1", "130", "2621"}
	if got := table.Total.ColumnTexts(); !reflect.DeepEqual(got, want) {
		t.Fatalf("端到端四指标应为 %v，实际 %v", want, got)
	}
	if !table.Calibrated() {
		t.Fatalf("四项读数齐 + 四个预算项都有定层 ⇒ 应报「已标定」")
	}
	// 复算：再读一次、再聚合一次，必须逐字节相同
	samples2, _, _ := LoadCostSamplesFrom(path)
	if again := AggregateCost(samples2, pF(24.0), LoadBudgetCalibFrom(calibPath)); !reflect.DeepEqual(table, again) {
		t.Fatalf("同文件两次聚合不得有差异（脚本可复算）：\n%+v\n%+v", table, again)
	}
	t.Logf("端到端可复算：%v（坏行 %d 条单列）", table.Total.ColumnTexts(), skipped)

	// 只读：文件不存在 ⇒ 报错且**不创建**
	missing := filepath.Join(dir, "nope.jsonl")
	if _, _, merr := LoadCostSamplesFrom(missing); merr == nil {
		t.Fatalf("不存在的观测文件应报错")
	}
	if _, serr := os.Stat(missing); !os.IsNotExist(serr) {
		t.Fatalf("只读入口不得创建文件，实际 %v", serr)
	}
}
