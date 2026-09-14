// service_borrow_test.go —— P3b-2 借用编排的可测部分（档位 / 受阻语义 / 空扫描）。
//
// 设计依据：设计-子端服务切换与基线服务声明-20260914.md §7 S5、§9.3、§11 M1/M6/M7。
// 真机停服已在 service_control_test.go 的 N7 用例覆盖；此处只测"决策与语义"。
package backend

import (
	"testing"
	"time"
)

func TestBaselineReclaimMode(t *testing.T) {
	cases := []struct{ env, want string }{
		{"", "borrow"}, // 未配置 ⇒ 默认 borrow（Mr2109 拍板）
		{"borrow", "borrow"},
		{"BORROW", "borrow"},
		{"refuse", "refuse"},
		{"REFUSE", "refuse"},
		{"乱七八糟", "borrow"}, // 非法值 ⇒ 回落默认
	}
	for _, c := range cases {
		t.Setenv(EnvServiceExclusive, c.env)
		if got := baselineReclaimMode(); got != c.want {
			t.Errorf("EnvServiceExclusive=%q ⇒ %q，期望 %q", c.env, got, c.want)
		}
	}
}

func TestTryBorrow_RefuseModeIsBlocked(t *testing.T) {
	t.Setenv(EnvServiceExclusive, "refuse")
	t.Setenv(EnvBaselinePorts, "59999")
	t.Setenv(LeaseDirEnv, t.TempDir())
	m := newEvictTestManager(1, map[string]*subproc{})

	m.mu.Lock()
	borrowed, err := m.tryBorrowForMemoryLocked(40)
	m.mu.Unlock()

	if len(borrowed) != 0 {
		t.Fatalf("refuse 档不该借出任何东西，实得 %v", borrowed)
	}
	if err == nil || !IsBorrowBlocked(err) {
		t.Fatalf("refuse 档应返回「可解释的受阻」错误，实得 %v", err)
	}
}

func TestTryBorrow_NoBaselineDeclaredIsBlocked(t *testing.T) {
	t.Setenv(EnvServiceExclusive, "borrow")
	t.Setenv(EnvBaselinePorts, "")
	t.Setenv(LeaseDirEnv, t.TempDir())
	m := newEvictTestManager(1, map[string]*subproc{})

	m.mu.Lock()
	borrowed, err := m.tryBorrowForMemoryLocked(40)
	m.mu.Unlock()

	if len(borrowed) != 0 {
		t.Fatalf("未声明基线时不该借出，实得 %v", borrowed)
	}
	if err == nil || !IsBorrowBlocked(err) {
		t.Fatalf("未声明基线应返回受阻错误，实得 %v", err)
	}
	if _, ok := LoadLease(59999); ok {
		t.Fatal("受阻路径不得留下任何租约（未停即未存档）")
	}
}

func TestTryBorrow_NoLeaseLeftWhenNothingBorrowable(t *testing.T) {
	// 不变式（与平台无关）：只要没能真借到，就**绝不许留下租约**（write-then-act 的反面：
	// 没有真正停止动作，就不该有借用存档）。覆盖两种情形：采样不可用（fail-safe 拒借）、
	// 端口无人监听（无候选）。59999 必然空闲 ⇒ 绝不会真去停任何东西。
	t.Setenv(EnvServiceExclusive, "borrow")
	t.Setenv(EnvBaselinePorts, "59999")
	t.Setenv(LeaseDirEnv, t.TempDir())
	m := newEvictTestManager(1, map[string]*subproc{})

	m.mu.Lock()
	borrowed, _ := m.tryBorrowForMemoryLocked(1)
	m.mu.Unlock()

	if len(borrowed) != 0 {
		t.Fatalf("没有可借服务时不该借出，实得 %v", borrowed)
	}
	if n := len(ListLeases()); n != 0 {
		t.Fatalf("未借到却留下 %d 条租约（不得无停止而存档）", n)
	}
}

func TestBorrowNeeded(t *testing.T) {
	cases := []struct {
		need, avail float64
		want        bool
		deficit     float64
	}{
		{40, 60, false, 0},
		{40, 40, false, 0},
		{40, 10, true, 30},
		{1, 0, true, 1},
		{0, 0, false, 0},
	}
	for _, c := range cases {
		gotN, gotD := borrowNeeded(c.need, c.avail)
		if gotN != c.want {
			t.Errorf("borrowNeeded(%.0f,%.0f) 的 needed=%v，期望 %v", c.need, c.avail, gotN, c.want)
			continue
		}
		if gotN && gotD != c.deficit {
			t.Errorf("borrowNeeded(%.0f,%.0f) 的 deficit=%.0f，期望 %.0f", c.need, c.avail, gotD, c.deficit)
		}
	}
}

func TestStartTimeMismatch(t *testing.T) {
	cases := []struct {
		archived, current uint64
		want              bool
	}{
		{1000, 1000, false}, // 同一进程
		{1000, 2000, true},  // pid 被复用 ⇒ 必须中止停止
		{0, 2000, false},    // 平台取不到（非 Linux）⇒ 不因"取不到"误伤
		{1000, 0, false},
		{0, 0, false},
	}
	for _, c := range cases {
		if got := startTimeMismatch(c.archived, c.current); got != c.want {
			t.Errorf("startTimeMismatch(%d,%d)=%v，期望 %v", c.archived, c.current, got, c.want)
		}
	}
}

func TestReturnExpiredLeases_EmptyDir(t *testing.T) {
	t.Setenv(LeaseDirEnv, t.TempDir())
	m := newEvictTestManager(1, map[string]*subproc{})
	if got := m.ReturnExpiredLeases(time.Now()); len(got) != 0 {
		t.Fatalf("空租约目录应无动作，实得 %v", got)
	}
}

func TestReturnExpiredLeases_NonActionableSkipped(t *testing.T) {
	// restored / 未到期的 borrowed 都不该被动作（避免误还正在用的服务）。
	t.Setenv(LeaseDirEnv, t.TempDir())
	now := time.Now()
	if err := SaveLease(ServiceLease{Port: 9500, State: LeaseRestored, Argv: []string{"x"},
		AcquiredAt: now, LastActive: now, TTLS: 1, MaxHoldS: 1}); err != nil {
		t.Fatal(err)
	}
	// 刚借出、远未到期
	if err := SaveLease(ServiceLease{Port: 9501, State: LeaseBorrowed, Argv: []string{"x"},
		AcquiredAt: now, LastActive: now, TTLS: 3600, MaxHoldS: 7200}); err != nil {
		t.Fatal(err)
	}
	m := newEvictTestManager(1, map[string]*subproc{})
	if got := m.ReturnExpiredLeases(now); len(got) != 0 {
		t.Fatalf("不该动作未到期/已归还的租约，实得 %v", got)
	}
	if n := len(ListLeases()); n != 2 {
		t.Fatalf("租约不该被删，实得 %d 条", n)
	}
}
