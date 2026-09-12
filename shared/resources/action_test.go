package resources

import "testing"

// ── 批 3 红线 ①：有在飞请求（req_count>0）的驻留绝不驱逐 ─────────────────────
//
// 断言口径：无论它多旧（last_used_ago_s 巨大）、多大（weights/mem 巨大）、
// 甚至是 crashed 或未托管，都必须既不在计划里，也不计入可腾退量。
func TestAction_InflightNeverEvicted(t *testing.T) {
	residents := []ResidentEntry{
		// 在飞 + 最旧 + 最大 + crashed + 未托管：干扰条件全满，仍必须动不了
		{Digest: "busy", Alias: "busy-model", State: StateCrashed, Managed: false, MemGb: 86, WeightsBytes: 100 * gib, LastUsedAgoS: 99999, ReqCount: 3},
		{Digest: "idle", Alias: "idle-model", State: StateReady, Managed: true, MemGb: 4, LastUsedAgoS: 5},
	}
	plan := EvictPlanForAction(residents, 4)
	if len(plan) != 1 || plan[0] != "idle-model" {
		t.Fatalf("计划只应含空闲项，实得 %v", plan)
	}
	for _, target := range plan {
		if target == "busy-model" {
			t.Fatal("红线①被破：在飞请求项出现在驱逐计划里")
		}
	}
	if _, ok := ActionBlockReason(residents[0]); ok {
		t.Fatal("红线①被破：在飞请求项被判为可动作")
	}
	if reason, _ := ActionBlockReason(residents[0]); reason != "inflight" {
		t.Fatalf("在飞项阻断原因应为 inflight，实得 %q", reason)
	}
	// 在飞项的 86G 不许被算成"可腾退"——否则会出现假腾退（计划看着够、卸完不够）
	if got := PlannedFreeGb(residents, []string{"busy-model"}); got != 0 {
		t.Fatalf("红线①被破：在飞项的占用被计入可腾退量 %.1f GiB", got)
	}
}

// ── 批 3 红线 ②：未托管进程绝不被接管、绝不被杀（Q6） ────────────────────────
//
// 断言口径：手工起的服务（managed=false）永不出现在动作计划里；
// 哪怕它就是五档里最该回收的档 1（未托管/僵尸/崩溃），也一律只报告不动手。
func TestAction_UnmanagedNeverTouched(t *testing.T) {
	residents := []ResidentEntry{
		// 实测 E2 形态：手工 screen 起、只听 127.0.0.1 的服务——最旧、最大、无在飞请求
		{Digest: "", Alias: "unmanaged@127.0.0.1:9001", State: StateReady, Managed: false, RssGb: 60, LastUsedAgoS: 5000},
		{Digest: "d-managed", Alias: "managed-a", State: StateReady, Managed: true, MemGb: 4, LastUsedAgoS: 10},
	}
	// 需要 60G：只有未托管项"看起来"够——但它是别人的进程，计划只能是"我们的 4G"
	plan := EvictPlanForAction(residents, 60)
	for _, target := range plan {
		if target == "unmanaged@127.0.0.1:9001" {
			t.Fatalf("红线②被破：未托管项进入动作计划 %v —— 等于接管别人的进程", plan)
		}
	}
	if len(plan) != 1 || plan[0] != "managed-a" {
		t.Fatalf("计划应只含托管项，实得 %v", plan)
	}
	if got := PlannedFreeGb(residents, []string{"unmanaged@127.0.0.1:9001"}); got != 0 {
		t.Fatalf("红线②被破：未托管进程的占用被当成可腾退量 %.1f GiB", got)
	}
	if reason, ok := ActionBlockReason(residents[0]); ok || reason != "unmanaged" {
		t.Fatalf("未托管项应被 unmanaged 阻断，实得 ok=%v reason=%q", ok, reason)
	}
	// 档号本身仍按设计把未托管排在最先（排序可见），但"可排序"≠"可动作"
	if got := RankEvictions(residents).Evictable[0].Managed; got {
		t.Fatal("未托管项在排序里应如实标 managed=false（只报告口径）")
	}
}

// ── 批 3 红线 ③：pin 且 TTL 未到期者不驱逐；pin 必须带 TTL（Q5） ──────────────
func TestAction_PinTTL(t *testing.T) {
	residents := []ResidentEntry{
		{Digest: "pin-live", Alias: "pinned-review", State: StateReady, Managed: true, MemGb: 17, Pinned: true, PinRemainS: 600, LastUsedAgoS: 9999},
		{Digest: "pin-expired", Alias: "pin-done", State: StateReady, Managed: true, MemGb: 2, Pinned: true, PinRemainS: 0, LastUsedAgoS: 1},
		{Digest: "pin-no-ttl", Alias: "pin-no-ttl", State: StateReady, Managed: true, MemGb: 2, Pinned: true, LastUsedAgoS: 1},
	}
	plan := EvictPlanForAction(residents, 2)
	for _, target := range plan {
		if target == "pinned-review" {
			t.Fatalf("红线③被破：pin 未到期的驻留进入计划 %v", plan)
		}
	}
	if reason, ok := ActionBlockReason(residents[0]); ok || reason != "pin_active" {
		t.Fatalf("pin 未到期项应被 pin_active 阻断，实得 ok=%v reason=%q", ok, reason)
	}
	// TTL 已到期 / 根本没有 TTL 的 pin：都不是"有效的 pin"（无 TTL 的 pin 等同内存泄漏）
	for _, i := range []int{1, 2} {
		if _, ok := ActionBlockReason(residents[i]); !ok {
			t.Fatalf("pin 已到期/无 TTL 者应可动作，实得被阻断：%+v", residents[i])
		}
	}
	if got := PlannedFreeGb(residents, []string{"pinned-review"}); got != 0 {
		t.Fatalf("红线③被破：pin 未到期项的占用被计入可腾退量 %.1f GiB", got)
	}
}

// ── 批 3 目标：只卸够（不是一把全清） ────────────────────────────────────────
//
// 断言口径：LRU 最旧的先走；累计到刚够就停——不许多卸一个。
func TestAction_JustEnoughNotAll(t *testing.T) {
	residents := []ResidentEntry{
		{Digest: "oldest", Alias: "A", State: StateReady, Managed: true, MemGb: 8, LastUsedAgoS: 300},
		{Digest: "mid", Alias: "B", State: StateReady, Managed: true, MemGb: 8, LastUsedAgoS: 200},
		{Digest: "newest", Alias: "C", State: StateReady, Managed: true, MemGb: 8, LastUsedAgoS: 100},
	}
	cases := []struct {
		name    string
		needGb  float64
		want    []string
		wantAll bool
	}{
		{"腾1G只需最旧一个", 1, []string{"A"}, false},
		{"腾8G刚好一个", 8, []string{"A"}, false},
		{"腾9G需要两个（第二多卸1G也不多卸第三个）", 9, []string{"A", "B"}, false},
		{"腾24G=全部可腾退（尽力而为）", 24, []string{"A", "B", "C"}, true},
		{"腾100G超过全部→仍只列全部可动作项", 100, []string{"A", "B", "C"}, true},
		{"无需腾退→空计划", 0, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvictPlanForAction(residents, c.needGb)
			if len(got) != len(c.want) {
				t.Fatalf("计划 want %v got %v", c.want, got)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("计划顺序 want %v got %v", c.want, got)
				}
			}
			if !c.wantAll && c.needGb > 0 && c.needGb <= 8 && len(got) > 1 {
				t.Fatalf("要 %v G 却卸了 %d 个——退化成一把全清", c.needGb, len(got))
			}
			if freed := PlannedFreeGb(residents, got); freed < c.needGb && len(got) > 0 && c.needGb <= 24 {
				t.Fatalf("计划腾退量 %.1fG 不足需求量 %.1fG（计划本身不成立）", freed, c.needGb)
			}
		})
	}
}

// ── 批 3 反例：无法寻址者不进计划（不猜目标）────────────────────────────────
func TestAction_UnaddressableSkipped(t *testing.T) {
	residents := []ResidentEntry{
		{Digest: "", Alias: "", State: StateReady, Managed: true, MemGb: 40, LastUsedAgoS: 100},
		{Digest: "ok", Alias: "", State: StateReady, Managed: true, MemGb: 4, LastUsedAgoS: 10},
	}
	plan := EvictPlanForAction(residents, 4)
	if len(plan) != 1 || plan[0] != "ok" {
		t.Fatalf("无法寻址者不得进计划，实得 %v", plan)
	}
}
