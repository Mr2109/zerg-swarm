// n16_findings_test.go —— 借用成功后重算的口径回归（n16）。
//
// 真机场景（2026-09-14）：借用释放了 32.7GB，但采样仍报 61.0GB 旧值 ⇒ 重算后仍不达标
// ⇒ 回 507、租约空悬到 TTL 才回收。修法是与"借用前 + 本次腾出量"取较大者。
package backend

import "testing"

func TestRecomputeAfterBorrow(t *testing.T) {
	cases := []struct {
		name                string
		sampled, before, gb float64
		want                float64
	}{
		{
			name:    "采样滞后（真机 n16 原样）：释放 32.7 但采样仍是旧值 ⇒ 用推算托底",
			sampled: 61.0, before: 61.0, gb: 32.7, want: 93.7,
		},
		{
			name:    "采样已跟上：不得重复计数（推算=采样）",
			sampled: 93.7, before: 61.0, gb: 32.7, want: 93.7,
		},
		{
			name:    "采样部分跟上：取较大者",
			sampled: 80.0, before: 61.0, gb: 32.7, want: 93.7,
		},
		{
			name:    "别的进程也释放了内存（采样更高）：以采样为准，绝不低估",
			sampled: 100.0, before: 61.0, gb: 32.7, want: 100.0,
		},
		{
			name:    "没腾出任何东西：就用采样",
			sampled: 61.0, before: 61.0, gb: 0, want: 61.0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := recomputeAfterBorrow(c.sampled, c.before, c.gb); got != c.want {
				t.Fatalf("recomputeAfterBorrow(%.1f, %.1f, %.1f) = %.1f，期望 %.1f",
					c.sampled, c.before, c.gb, got, c.want)
			}
		})
	}
}

// 未借出任何端口时占用合计为 0（不能凭空造出"腾出量"）。
func TestBorrowedOccupiedGb_Empty(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{})
	m.mu.Lock()
	defer m.mu.Unlock()
	if got := m.borrowedOccupiedGbLocked(nil); got != 0 {
		t.Fatalf("未借出时应为 0，实得 %v", got)
	}
}
