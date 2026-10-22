package eval

import (
	"math"
	"testing"
)

// 指标算式的手算用例（任务单 §6 笔1 第 4 条）：
// 这些数字是人先算出来的，代码只是复现；改算式先改这里。
func TestScore_Formulas(t *testing.T) {
	// M4 删除比：before=100, after=40 ⇒ 0.60
	if got := deleted(100, 40); math.Abs(got-0.60) > 1e-9 {
		t.Errorf("deleted(100,40) = %v，手算 0.60", got)
	}
	// 真机口径：compression_ratio = before/after；ratio=2.5 ⇒ deleted = 1 − 1/2.5 = 0.60
	if got := deletedFromChatRatio(2.5); math.Abs(got-0.60) > 1e-9 {
		t.Errorf("deletedFromChatRatio(2.5) = %v，手算 0.60", got)
	}
	// 反向：ratio<1 表示「压完更大」⇒ 删除比为负（如实报，不夹到 0）
	if got := deletedFromChatRatio(0.8); math.Abs(got-(-0.25)) > 1e-9 {
		t.Errorf("deletedFromChatRatio(0.8) = %v，手算 -0.25", got)
	}
	if got := deleted(0, 40); got != 0 {
		t.Errorf("deleted(0,40) = %v，before<=0 应记 0", got)
	}
	if got := deletedFromChatRatio(0); got != 0 {
		t.Errorf("deletedFromChatRatio(0) = %v，应记 0", got)
	}

	// M3 p95 最近秩：1..20 升序第 ⌈0.95×20⌉=19 个 ⇒ 19
	var xs []float64
	for i := 1; i <= 20; i++ {
		xs = append(xs, float64(i))
	}
	if got := pctNearestRank(xs, 0.95); got != 19 {
		t.Errorf("pctNearestRank(1..20, 0.95) = %v，手算 19", got)
	}
	// 1..10 ⇒ ⌈9.5⌉=10 ⇒ 10
	xs10 := xs[:10]
	if got := pctNearestRank(xs10, 0.95); got != 10 {
		t.Errorf("pctNearestRank(1..10, 0.95) = %v，手算 10", got)
	}
	// 空集不许 panic，返回 0 由调用方标 N/A
	if got := pctNearestRank(nil, 0.95); got != 0 {
		t.Errorf("空集 p95 = %v，应 0", got)
	}
	// 不修改入参顺序（调用方后面还要算中位数）
	shuffled := []float64{9, 1, 5, 3}
	_ = pctNearestRank(shuffled, 0.95)
	if shuffled[0] != 9 {
		t.Errorf("pctNearestRank 改动了入参：%v", shuffled)
	}

	// 中位数
	if got := median([]float64{3, 1, 2}); got != 2 {
		t.Errorf("median([3,1,2]) = %v", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median([4,1,3,2]) = %v，手算 2.5", got)
	}

	// ratio
	if got := ratioOf(100, 25); got != 0.25 {
		t.Errorf("ratioOf(100,25) = %v", got)
	}
	if got := ratioOf(0, 25); got != 1 {
		t.Errorf("ratioOf(0,25) = %v，无原文应记 1", got)
	}
}

// norm 只做三件事：去 BOM、CRLF→LF、去行尾空白（规6）。
// 这几条是「命中」与「冒充命中」的分界线，必须钉死。
func TestScore_Norm(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\ufeffa\r\nb\r\n", "a\nb\n"},   // BOM + CRLF
		{"a  \nb\t\n", "a\nb\n"},         // 行尾空白
		{"a  b", "a  b"},                 // 中间空白**不**折叠
		{"  a", "  a"},                   // 行首缩进**不**去
		{"ErrGuard", "ErrGuard"},         // **不**小写化
		{"single line  ", "single line"}, // 单行也要去尾空白
	}
	for _, c := range cases {
		if got := norm(c.in); got != c.want {
			t.Errorf("norm(%q) = %q，want %q", c.in, got, c.want)
		}
	}

	// 命中判定逐字：多一个空格就不算命中（防「看起来在」）
	if needleHit(norm("want 3 rows, got 0"), "want  3 rows") {
		t.Errorf("折叠宽度不同的字符串被判成命中")
	}
	if !needleHit(norm("store_test.go:118: want 3 rows, got 0  "), "want 3 rows, got 0") {
		t.Errorf("行尾空白差异不该影响命中")
	}
	if needleHit("anything", "") {
		t.Errorf("空 needle 不该命中")
	}
}

// 速率与四舍五入：分母 0 记 0（由调用方标 N/A），不许 NaN 混进 JSON。
func TestScore_Rate(t *testing.T) {
	if got := rate(0, 0); got != 0 {
		t.Errorf("rate(0,0) = %v，应 0（N/A 由调用方标）", got)
	}
	if got := rate(1, 3); math.Abs(got-0.3333) > 1e-4 {
		t.Errorf("rate(1,3) = %v", got)
	}
	if math.IsNaN(rate(0, 0)) {
		t.Errorf("rate 产出了 NaN（会污染 JSON）")
	}
}
