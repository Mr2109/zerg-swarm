package resources

import "testing"

// TestEvict_RankTable —— 驱逐排序（Q2 五档）：在飞绝不驱逐、pin 带 TTL、档序与次键。
func TestEvict_RankTable(t *testing.T) {
	cases := []struct {
		name          string
		residents     []ResidentEntry
		wantEvictable []string
		wantProtected map[string]string // digest → reason
	}{
		{
			name: "在飞请求者绝不驱逐(即便crashed/未托管)",
			residents: []ResidentEntry{
				{Digest: "busy", State: StateReady, Managed: true, MemGb: 4, ReqCount: 1},
				{Digest: "busy-crashed", State: StateCrashed, Managed: false, MemGb: 4, ReqCount: 3},
				{Digest: "idle", State: StateIdle, Managed: true, MemGb: 4},
			},
			wantEvictable: []string{"idle"},
			wantProtected: map[string]string{"busy": "inflight", "busy-crashed": "inflight"},
		},
		{
			name: "pin未到期不驱逐-TTL到期/无TTL可驱逐",
			residents: []ResidentEntry{
				{Digest: "pin-live", State: StateReady, Managed: true, MemGb: 4, Pinned: true, PinRemainS: 30},
				{Digest: "pin-dead", State: StateReady, Managed: true, MemGb: 4, Pinned: true, PinRemainS: 0},
				{Digest: "pin-notll", State: StateReady, Managed: true, MemGb: 4, Pinned: true},
				{Digest: "plain", State: StateIdle, Managed: true, MemGb: 4},
			},
			wantEvictable: []string{"pin-dead", "pin-notll", "plain"},
			wantProtected: map[string]string{"pin-live": "pin_active"},
		},
		{
			name: "五档顺序-未托管僵尸崩溃→空闲LRU→粘性最后(同档权重更大先)",
			residents: []ResidentEntry{
				{Digest: "W", State: StateReady, Managed: true, MemGb: 8, WeightsBytes: 100 * gib, WaitBound: true},  // 档5
				{Digest: "I1", State: StateReady, Managed: true, MemGb: 8, WeightsBytes: 8 * gib, LastUsedAgoS: 10},  // 档3
				{Digest: "K", State: StateCrashed, Managed: true, MemGb: 1, WeightsBytes: 1 * gib, LastUsedAgoS: 5},  // 档1
				{Digest: "I3", State: StateReady, Managed: true, MemGb: 4, WeightsBytes: 4 * gib, LastUsedAgoS: 99},  // 档3
				{Digest: "J", State: StateIdle, Managed: false, MemGb: 2, WeightsBytes: 2 * gib, LastUsedAgoS: 50},   // 档1
				{Digest: "I2", State: StateReady, Managed: true, MemGb: 8, WeightsBytes: 8 * gib, LastUsedAgoS: 500}, // 档3
			},
			wantEvictable: []string{"J", "K", "I2", "I1", "I3", "W"},
			wantProtected: map[string]string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := RankEvictions(c.residents)
			got := make([]string, len(plan.Evictable))
			for i, e := range plan.Evictable {
				got[i] = e.Digest
				if e.Reason == "" {
					t.Fatalf("可驱逐项缺 reason: %v", e)
				}
			}
			if len(got) != len(c.wantEvictable) {
				t.Fatalf("可驱逐项 want %v got %v", c.wantEvictable, got)
			}
			for i := range c.wantEvictable {
				if got[i] != c.wantEvictable[i] {
					t.Fatalf("顺序 want %v got %v", c.wantEvictable, got)
				}
			}
			// 档号须单调不减（1 → 3 → 5）
			for i := 1; i < len(plan.Evictable); i++ {
				if plan.Evictable[i].Tier < plan.Evictable[i-1].Tier {
					t.Fatalf("档号应单调不减，实得 %v", plan.Evictable)
				}
			}
			// 受保护项集合与原因
			if len(plan.Protected) != len(c.wantProtected) {
				t.Fatalf("受保护项 want %v got %v", c.wantProtected, plan.Protected)
			}
			for _, p := range plan.Protected {
				if c.wantProtected[p.Digest] != p.Reason {
					t.Fatalf("受保护原因 want %v got %v", c.wantProtected, plan.Protected)
				}
			}
		})
	}
}

// TestEvict_ToFree_Table —— 只取"够用"的前缀（替代一把全清）；受保护项不计入可腾退。
func TestEvict_ToFree_Table(t *testing.T) {
	rs := []ResidentEntry{
		{Digest: "E1", State: StateReady, Managed: true, MemGb: 4, LastUsedAgoS: 10},
		{Digest: "E2", State: StateReady, Managed: true, MemGb: 4, LastUsedAgoS: 20},
		{Digest: "E3", State: StateReady, Managed: true, MemGb: 8, LastUsedAgoS: 30}, // LRU 最旧 → 最先
	}
	protected := []ResidentEntry{
		{Digest: "free-me", State: StateIdle, Managed: true, MemGb: 4},
		{Digest: "busy", State: StateReady, Managed: true, MemGb: 16, ReqCount: 1},
		{Digest: "pinned", State: StateReady, Managed: true, MemGb: 16, Pinned: true, PinRemainS: 60},
	}
	cases := []struct {
		name          string
		residents     []ResidentEntry
		needGb        float64
		wantPlan      []string
		wantTotal     float64
		wantEvictable int
	}{
		{"需求<=0取空", rs, 0, nil, 16, 3},
		{"腾5只需E3", rs, 5, []string{"E3"}, 16, 3},
		{"腾9需E3+E2", rs, 9, []string{"E3", "E2"}, 16, 3},
		{"受保护不计入可腾退", protected, 1, []string{"free-me"}, 4, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvictToFree(c.residents, c.needGb)
			if len(got) != len(c.wantPlan) {
				t.Fatalf("计划 want %v got %v", c.wantPlan, got)
			}
			for i := range c.wantPlan {
				if got[i] != c.wantPlan[i] {
					t.Fatalf("计划 want %v got %v", c.wantPlan, got)
				}
			}
			if total := TotalEvictableGb(c.residents); total != c.wantTotal {
				t.Fatalf("可腾退总额 want %v got %v", c.wantTotal, total)
			}
			if n := len(EvictableDigests(c.residents)); n != c.wantEvictable {
				t.Fatalf("可驱逐清单长度 want %d got %d", c.wantEvictable, n)
			}
		})
	}
}
