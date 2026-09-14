// n16b_lease_test.go —— 钉死 n16 端到端复验抓到的真 bug。
//
// 真机日志原文：`[backend] 借用后重算：采样 61.4 GB、本次腾出 0.0 GB ⇒ 取 61.4 GB`
// 原因：腾出量原从 baselineServicesLocked() 里按端口求和，而服务一旦被停、
// 身份探测（/v1/models）就失败 ⇒ 它不再出现在基线列表里 ⇒ 求和**恒为 0**。
// 正确来源是租约存档：ApproxGB 是**停机前**记下的。
package backend

import "testing"

func TestBorrowedOccupiedGb_ComesFromLeaseArchive(t *testing.T) {
	t.Setenv(LeaseDirEnv, t.TempDir())
	if err := SaveLease(ServiceLease{
		LeaseID: "59995-n16b", Port: 59995, Kind: "llama", Class: "bare",
		Identity: "/data/models/x.gguf", Argv: []string{"/bin/true"},
		State: LeaseBorrowed, ApproxGB: 32.7,
	}); err != nil {
		t.Fatalf("写租约失败: %v", err)
	}

	m := newEvictTestManager(1, map[string]*subproc{})
	m.mu.Lock()
	defer m.mu.Unlock()

	// 此刻服务已被停 ⇒ 不在基线列表里。旧实现会返回 0（正是真机上的症状）。
	got := m.borrowedOccupiedGbLocked([]int{59995})
	if got != 32.7 {
		t.Fatalf("必须从租约存档取到 32.7，实得 %v —— 若为 0 说明又变回了「从基线列表求和」，"+
			"服务一停那里就查不到，会白借一次", got)
	}
}

// 端口没有对应租约时不得凭空造数（缺席就是 0，不编造）。
func TestBorrowedOccupiedGb_MissingLeaseIsZero(t *testing.T) {
	t.Setenv(LeaseDirEnv, t.TempDir())
	m := newEvictTestManager(1, map[string]*subproc{})
	m.mu.Lock()
	defer m.mu.Unlock()
	if got := m.borrowedOccupiedGbLocked([]int{59994}); got != 0 {
		t.Fatalf("无租约应为 0，实得 %v", got)
	}
}
