package backend

// p2_idlearmed_test.go —— P2 批 2：显式状态「空窗计时中」+ 时间戳低频巡检。
//
// 设计依据：设计-子端沙箱化-20260914.md §7.7 修补 1/2、§7.1 补记四行迁移表、§13 Q21。
// 验收点：
//   - 状态串 ready → 空窗计时中 → draining → stopped 只读可观测（StateOf）；
//   - 到期触发卸载（每卵阈值 idle_unload_s；缺省 600s、小模型 120s）；
//   - 请求到达重置计时戳（修补 2：「重置空窗」= 更新一个时间戳，无定时器可取消）；
//   - 同模型请求取消空窗回 ready（§7.1 补记第 2 行）；
//   - 异模型请求提前收卵：无在飞 ⇒ 赢权收卵；有在飞 ⇒ 等生成跑完（不硬杀，§7.2/Q6）；
//   - 禁「sleep N 后直接杀」路径：grep 自查零命中（见回报）。

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// smallEntry 小模型卵（idle_unload_s: 120——Q21 建议值），cmd 指向不存在的二进制防真起进程。
func smallEntry() *registry.ModelEntry {
	return &registry.ModelEntry{
		Backend:     "llama-server",
		File:        "/data/models/p2/small.gguf",
		MemGB:       2,
		IdleUnloadS: 120,
		Cmd:         registry.CmdString("/nonexistent/zerg-p2-noop-binary"),
	}
}

// defaultEntry 未声明阈值的卵 ⇒ 缺省 600s（§13 Q21 / DefaultIdleUnloadSeconds）。
func defaultEntry() *registry.ModelEntry {
	return &registry.ModelEntry{
		Backend: "llama-server",
		File:    "/data/models/p2/default.gguf",
		MemGB:   4,
		Cmd:     registry.CmdString("/nonexistent/zerg-p2-noop-binary"),
	}
}

// 每卵阈值：声明 120s 的小模型取 120；未声明取缺省 600（§4.3 第九项 / Q21）。
func TestP2B2_PerEggThreshold(t *testing.T) {
	if got := eggIdleUnloadThreshold(smallEntry()); got != 120*time.Second {
		t.Fatalf("声明 idle_unload_s:120 ⇒ 阈值 120s，实得 %v", got)
	}
	if got := eggIdleUnloadThreshold(defaultEntry()); got != 600*time.Second {
		t.Fatalf("未声明 ⇒ 缺省 600s，实得 %v", got)
	}
	if got := eggIdleUnloadThreshold(nil); got != 600*time.Second {
		t.Fatalf("nil entry ⇒ 缺省 600s，实得 %v", got)
	}
}

// 到期触发卸载：空窗计时中 + 越过**每卵**阈值 ⇒ 巡检收走；未越过（哪怕越过缺省 600）⇒ 留。
func TestP2B2_ExpiredReapPerEggThreshold(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		// 小模型（阈值 120s）：空窗 200s ⇒ 到期收走
		"small": {model: "small", entry: smallEntry(), port: 9421,
			state: StateIdleArmed, lastUsed: now.Add(-200 * time.Second)},
		// 未声明阈值（缺省 600s）：空窗 200s ⇒ 未到期，留着
		"def": {model: "def", entry: defaultEntry(), port: 9422,
			state: StateIdleArmed, lastUsed: now.Add(-200 * time.Second)},
	})
	reaped := m.ReapIdle(now)
	if len(reaped) != 1 || reaped[0] != "small" {
		t.Fatalf("只该收越过每卵阈值 120s 的 small，实得 %v", reaped)
	}
	if _, still := m.procs["def"]; !still {
		t.Fatal("未越过缺省 600s 的卵被误收（每卵阈值未生效）")
	}
}

// 请求到达重置：空窗计时中 ⇒ acquire（+1/−1 一轮）后计时戳刷新 ⇒ 原本将到期的不再到期。
func TestP2B2_RequestArrivalResetsTimestamp(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"m": {model: "m", entry: smallEntry(), port: 9423,
			state: StateIdleArmed, lastUsed: now.Add(-119 * time.Second)}, // 距 120s 只差 1s
	})
	// 请求到达又完成（一轮在飞）——时间戳被刷新为「现在」
	if !m.acquireInflight("m") {
		t.Fatal("空窗计时中的卵必须能接请求（取消空窗）")
	}
	m.releaseInflight("m")
	// 老时间戳视角下「早就该到期」，但时间戳已被重置 ⇒ 不卸（修补 2：重置=更新时间戳）
	reaped := m.ReapIdle(now)
	if len(reaped) != 0 {
		t.Fatalf("时间戳已重置 ⇒ 不得按旧时间戳到期收卵，实得 %v", reaped)
	}
	if got := m.StateOf("m"); got != StateIdleArmed {
		t.Fatalf("完成一轮后应重新进入空窗计时中，实得 %s", got)
	}
}

// 同模型请求取消空窗：RequestModel(同模型) ⇒ 计时戳归零、转 ready、不换卵不停进程。
func TestP2B2_SameModelCancelsIdleArmed(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"m": {model: "m", entry: smallEntry(), port: 9424,
			state: StateIdleArmed, lastUsed: now.Add(-119 * time.Second)},
	})
	needUnload, blocked := m.RequestModel("m")
	if needUnload || blocked {
		t.Fatalf("同模型请求不得触发收卵/阻塞，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
	if got := m.StateOf("m"); got != StateReady {
		t.Fatalf("同模型请求后应回 ready（取消空窗），实得 %s", got)
	}
	if _, ok := m.procs["m"]; !ok {
		t.Fatal("同模型请求不得换卵/停进程")
	}
	// 取消后立刻巡检也不许收（计时戳已归零）
	if reaped := m.ReapIdle(now); len(reaped) != 0 {
		t.Fatalf("空窗已取消 ⇒ 巡检不得收卵，实得 %v", reaped)
	}
}

// 异模型请求提前收卵：当前卵无在飞 ⇒ 锁内赢权转 draining（needUnload=true）。
func TestP2B2_OtherModelPreemptiveUnload_NoInflight(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"old": {model: "old", entry: smallEntry(), port: 9425,
			state: StateIdleArmed, lastUsed: now.Add(-time.Second)},
	})
	needUnload, blocked := m.RequestModel("new")
	if !needUnload || blocked {
		t.Fatalf("异模型请求且无在飞 ⇒ 应赢权提前收卵，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
	if got := m.StateOf("old"); got != StateDraining {
		t.Fatalf("锁内赢权后状态应为 draining，实得 %s", got)
	}
	// 600s 还没走完就被收——「600s 只是没人用的优化，不是必须占满」（§7.7 末条）
}

// 异模型请求但当前卵有在飞 ⇒ 拒绝提前收卵（blocked=true，等生成跑完——不打断在飞，§7.2/Q6）。
func TestP2B2_OtherModelBlockedWhileInflight(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"old": {model: "old", entry: smallEntry(), port: 9426,
			state: StateReady, lastUsed: now.Add(-time.Second), inflight: 2},
	})
	needUnload, blocked := m.RequestModel("new")
	if needUnload || !blocked {
		t.Fatalf("有在飞 ⇒ 不得收卵、必须阻塞等待，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
	if got := m.StateOf("old"); got != StateReady {
		t.Fatalf("在飞中的卵状态不得被改动，实得 %s", got)
	}
	// 在飞清零后（释放归零 → 转空窗计时中），异模型请求再战 ⇒ 赢权
	m.releaseInflight("old")
	m.releaseInflight("old")
	if got := m.StateOf("old"); got != StateIdleArmed {
		t.Fatalf("在飞清零后应转空窗计时中，实得 %s", got)
	}
	if needUnload, blocked := m.RequestModel("new"); !needUnload || blocked {
		t.Fatalf("在飞清零后异模型请求应赢权收卵，实得 needUnload=%v blocked=%v", needUnload, blocked)
	}
}

// 观测面：IdleArmedRemainingS 如实报剩余秒数；ready/draining 报 ok=false。
func TestP2B2_Observability(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"armed":  {model: "armed", entry: smallEntry(), port: 9427, state: StateIdleArmed, lastUsed: now.Add(-30 * time.Second)},
		"ready1": {model: "ready1", entry: smallEntry(), port: 9428, state: StateReady, lastUsed: now},
	})
	if remain, ok := m.IdleArmedRemainingS("armed"); !ok || remain <= 89 || remain > 90 {
		t.Fatalf("空窗计时中剩余应 ≈90s，实得 remain=%v ok=%v", remain, ok)
	}
	if _, ok := m.IdleArmedRemainingS("ready1"); ok {
		t.Fatal("ready 态不在计时中，ok 必须为 false")
	}
	if _, ok := m.IdleArmedRemainingS("ghost"); ok {
		t.Fatal("未驻留模型 ok 必须为 false")
	}
}
