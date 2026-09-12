package resources

import "testing"

// TestLedger_RoomForNeed_Table —— 账本"还能装下 X 吗"：装得下/装不下/刚好边界；
// 单槽默认 vs 显式多槽；显存独立生效 vs 统一内存口径。
func TestLedger_RoomForNeed_Table(t *testing.T) {
	m := func(memAvail float64, resident ...ResidentEntry) MachineLedger {
		return MachineLedger{Machine: "local", MemTotalGb: 64, MemAvailGb: memAvail, Resident: resident}
	}
	cases := []struct {
		name        string
		machine     MachineLedger
		need        float64
		maxResident int
		wantFit     bool
	}{
		{"装得下", m(10), 5, 0, true},
		{"刚好边界", m(10), 10, 0, true},
		{"装不下", m(10), 10.5, 0, false},
		{"单槽默认-已有驻留即拒", m(40, ResidentEntry{Digest: "d1", MemGb: 2, Managed: true, State: StateReady}), 2, 0, false},
		{"显式多槽-内存够即过", m(40, ResidentEntry{Digest: "d1", MemGb: 2, Managed: true, State: StateReady}), 2, 3, true},
		{"显存不足-拒(内存够)", MachineLedger{Machine: "x3", MemTotalGb: 128, MemAvailGb: 100, VramTotalGb: 24, VramFreeGb: 1}, 2, 0, false},
		{"统一内存-只按内存口径", MachineLedger{Machine: "mac", MemTotalGb: 64, MemAvailGb: 40}, 20, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason := c.machine.RoomForNeedGb(c.need, c.maxResident)
			if ok != c.wantFit {
				t.Fatalf("want fit=%v got %v（原因=%s）", c.wantFit, ok, reason)
			}
			if reason == "" {
				t.Fatal("原因不得为空")
			}
		})
	}
}

// TestLedger_Aggregations —— 账本聚合：驻留数/总占用/权重合计/新增可用/在飞/未托管/查询。
func TestLedger_Aggregations(t *testing.T) {
	m := MachineLedger{
		Machine: "local", MemTotalGb: 32, MemAvailGb: 10,
		Resident: []ResidentEntry{
			{Digest: "d1", Alias: "alpha", State: StateReady, Managed: true, MemGb: 4, WeightsBytes: 4 * gib, ReqCount: 1},
			{Digest: "d2", State: StateIdle, Managed: false, RssGb: 3, WeightsBytes: 2 * gib}, // MemGb 缺失 → 占用回退 Rss
		},
	}
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"驻留数", m.ResidentCount(), 2},
		{"总占用GiB(声明优先,缺失回退RSS)", m.ResidentOccupiedGb(), 7.0},
		{"权重合计", m.ResidentWeightsBytes(), 6 * gib},
		{"新增可用(avail+resident)", m.FreeMemGbForNew(), 17.0},
		{"在飞数", len(m.InflightResidents()), 1},
		{"在飞首项", m.InflightResidents()[0].Digest, "d1"},
		{"有在飞", m.HasInflight(), true},
		{"未托管驻留数", len(m.UnmanagedResidents()), 1},
		{"未托管首项", m.UnmanagedResidents()[0].Digest, "d2"},
		{"托管驻留数", len(m.ManagedResidents()), 1},
		{"按别名命中", mustFind(m, "alpha"), true},
		{"不存在的摘要不命中", mustFind(m, "nope"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Fatalf("want %v got %v", c.want, c.got)
			}
		})
	}
}

// mustFind 返回按 id 是否命中驻留项。
func mustFind(m MachineLedger, id string) bool { _, ok := m.FindResident(id); return ok }

// TestLedger_VramKnownAndDefaults —— 显存是否提供 的判定 + Q1 默认单槽常量。
func TestLedger_VramKnownAndDefaults(t *testing.T) {
	cases := []struct {
		name  string
		vram  float64
		known bool
	}{
		{"无独立显存(统一内存)", 0, false},
		{"有独立显存", 24, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (MachineLedger{VramTotalGb: c.vram}).VramKnown(); got != c.known {
				t.Fatalf("VramKnown want %v got %v", c.known, got)
			}
		})
	}
	if DefaultMaxResident != 1 {
		t.Fatalf("Q1 默认单槽应为 1，实得 %d", DefaultMaxResident)
	}
}

// TestLedger_Q6_UnmanagedVisible —— 手工服务如实标 managed=false 且可见；不被自动接管。
func TestLedger_Q6_UnmanagedVisible(t *testing.T) {
	m := MachineLedger{
		Machine: "x3", MemTotalGb: 128, MemAvailGb: 50,
		Resident:  []ResidentEntry{{Digest: "sha256-manual", State: StateReady, Managed: false, MemGb: 8}},
		Unmanaged: []UnmanagedProcess{{PID: 4242, Port: 8100, Command: "llama-server", RssGb: 8, Note: "手工 screen 起，只绑 127.0.0.1"}},
	}
	if got := m.UnmanagedResidents(); len(got) != 1 || got[0].Managed {
		t.Fatalf("未托管驻留应如实可见且 managed=false，实得 %v", got)
	}
	if len(m.Unmanaged) != 1 || m.Unmanaged[0].Port != 8100 {
		t.Fatalf("未托管进程清单应可见，实得 %v", m.Unmanaged)
	}
}
