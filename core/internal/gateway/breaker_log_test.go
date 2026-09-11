package gateway

// breaker_log_test.go — 待修补 #20：熔断失败日志的阈值必须引用真实常量 circuitFailThreshold。
//
// 历史缺陷：markFailure 里写死过 "threshold 3" / "(%d/3)"，真实阈值是 circuitFailThreshold=8，
// 日志与判定不符会误导运维。本测试不锚定日志文案字面量，而是断言「格式化输出与常量一致」，
// 因此将来任何把阈值写死的改动都会在此暴露。

import (
	"fmt"
	"strings"
	"testing"
)

// 日志阈值位必须由 circuitFailThreshold 决定（不是任何硬编码数字）。
func TestBreakerFailLogLine_UsesCircuitFailThreshold(t *testing.T) {
	const failCount = 3 // 刻意取一个历史硬编码出现过的计数值
	line := breakerFailLogLine("x3", failCount)

	// 断言：输出里的阈值位 == 常量（用常量拼出期望，而非写死数字）。
	want := fmt.Sprintf("(%d/%d)", failCount, circuitFailThreshold)
	if !strings.Contains(line, want) {
		t.Fatalf("熔断日志阈值位必须取自 circuitFailThreshold：期望包含 %q，实际 %q", want, line)
	}

	// 回归防护：真实阈值不是 3 时，输出里不得再出现 "(3/3)" 这种历史漂移写法。
	if circuitFailThreshold != 3 && strings.Contains(line, fmt.Sprintf("(%d/%d)", failCount, 3)) {
		t.Fatalf("熔断日志阈值位出现硬编码 3（历史漂移，真实阈值 %d）：%q", circuitFailThreshold, line)
	}
}

// 改变计数值时，阈值位恒定等于 circuitFailThreshold（证明阈值来自常量而非随计数变化）。
func TestBreakerFailLogLine_ThresholdIndependentOfCount(t *testing.T) {
	suffix := fmt.Sprintf("/%d); exceeding threshold will demote", circuitFailThreshold)
	for _, cnt := range []int{1, 3, 8, 100} {
		line := breakerFailLogLine("x3", cnt)
		if !strings.HasSuffix(line, suffix) {
			t.Fatalf("count=%d 时日志必须以 /%d 收口：%q", cnt, circuitFailThreshold, line)
		}
	}
}
