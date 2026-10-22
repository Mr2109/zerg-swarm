package eval

import (
	"math"
	"sort"
	"strings"
)

// 本文件是**纯函数**：归一化、needle 命中、百分位、删除比。
// 所有算式对应任务单 §4，改这里必须同步改 score_test.go 的手算用例。

// norm —— 归一化只做三件事（任务单 规6）：
//
//	① 去 BOM ② CRLF→LF ③ 去行尾空白
//
// ★ 刻意**不**折叠空白、**不**小写化、**不**去行首缩进：
// 折叠空白会让「看起来在」冒充命中（"want 3 rows" 与 "want  3  rows" 不是同一断言），
// 小写化会把 `ErrGuard` 和 `errguard` 混成一条。
func norm(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !strings.Contains(s, "\n") {
		return strings.TrimRight(s, " \t")
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// needleHit —— 逐字子串命中（规5：禁「同义改写才算命中」）。
func needleHit(hay, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(hay, norm(needle))
}

// deleted —— 删掉了多少（任务单 §4 M4）：1 − after/before；before<=0 记 0。
func deleted(before, after int) float64 {
	if before <= 0 {
		return 0
	}
	return 1 - float64(after)/float64(before)
}

// deletedFromChatRatio —— `chat_obs.compaction.compression_ratio` 是 **before/after**（>1 才压下去），
// 换算成删除比：1 − 1/ratio（任务单 §7 风险 1）。ratio<=0 记 0。
func deletedFromChatRatio(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	return 1 - 1/ratio
}

// pctNearestRank —— 最近秩百分位（任务单 §4 M3）：升序第 ⌈q·n⌉ 个（1-based）。
//
// 空集返回 0（调用方负责标 N/A，别把 0 当读数）。
func pctNearestRank(xs []float64, q float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	if q <= 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	rank := int(math.Ceil(q * float64(n))) // 1-based
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// median —— 中位数（偶数个取中间两数均值）。
func median(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// ratioOf —— after/before；before<=0 记 1（没动）。
func ratioOf(before, after int) float64 {
	if before <= 0 {
		return 1
	}
	return float64(after) / float64(before)
}
