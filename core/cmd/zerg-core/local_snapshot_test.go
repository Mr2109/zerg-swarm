package main

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/localback"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestStartLocalSnapshotRefresh_WritesBeforeFirstTick —— 待修补 #31 反例（反例优先）。
//
// 背景：store 的 local 行原本只由 30s 周期任务写、首次触发在注册后 30s——启动窗口内
// 只读 store 的消费方（路由打分/调度探活/模型聚合/pin 校验）看不到 local 行。
//
// 本用例构造「ticker 尚未触发」的初始状态：周期设成 1 小时，测试期间绝不首拍，
// 因此能写进 store 的 local 行**只能**来自注册时的同步首写。断言注册动作本身就写好了 local 行。
//
// 变异验证：删掉 startLocalSnapshotRefresh 里的 refreshLocalSnapshot(...) 同步首写
// （只留 goroutine），本用例必红——证它不是永远通过。
func TestStartLocalSnapshotRefresh_WritesBeforeFirstTick(t *testing.T) {
	lb := localback.NewLocalBackend("")
	st := store.NewStore()

	// 前置：注册前 store 里没有任何 local 行——这正是启动窗口的真实初始态
	if snap := st.GetSnapshot("local"); snap != nil {
		t.Fatalf("前置断言失败：注册前 store 不应有 local 行，实得 %+v", snap)
	}

	// 周期 1 小时——ticker 不可能在测试内触发，唯一写入来源就是同步首写
	startLocalSnapshotRefresh(lb, st, time.Hour)

	snap := st.GetSnapshot("local")
	if snap == nil {
		t.Fatal("启动窗口内 store 无 local 行：ticker 首拍之前消费方读不到 local（#31 未修）")
	}
	if snap.Machine != "local" {
		t.Fatalf("local 行机器名 = %q，应 local", snap.Machine)
	}
	if snap.LastSeen.IsZero() {
		t.Fatal("local 行 LastSeen 为零值——未走 SetLocalSnapshot 写入")
	}
}

// TestStartLocalSnapshotRefresh_PeriodicStillRefreshes —— 周期刷新未退化。
//
// 短周期下应观察到 store 的 local 行被反复替换（SetLocalSnapshot 每次写新指针）；
// 保证修法没有把「周期刷新」这一半丢掉。
//
// 2026-09-16 第十四轮：**上限从 5s 抬到 30s**（判据一字未改：仍要求"观察到新一次写入"）。
// 依据（实测）：全量 `-race` 套件（18 个包并行）里本用例红过一次、用例自身耗时 8.93s（= 5s 上限被打满）；
// 同命令单跑（`-run TestStartLocalSnapshotRefresh -count=1 -race`）连跑 3 次全绿 ⇒ **负载相关的墙太近**，
// 不是"周期刷新退化"。机制解释（非实测）：每次刷新要跑 `localBack.Snapshot()` 与 `collectGpuPct()`，
// 后者是 `exec.Command("ioreg", "-l")` —— 真机重命令。这里**没有**改周期、也**没有**放宽任何断言，
// 只把「等一个可观察事件」的上限放到"一次重命令 + 调度迟延"的量级。
func TestStartLocalSnapshotRefresh_PeriodicStillRefreshes(t *testing.T) {
	lb := localback.NewLocalBackend("")
	st := store.NewStore()
	startLocalSnapshotRefresh(lb, st, 50*time.Millisecond)

	first := st.GetSnapshot("local")
	if first == nil {
		t.Fatal("同步首写缺失：local 行不存在")
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if cur := st.GetSnapshot("local"); cur != nil && cur != first {
			return // 观察到新一次写入——周期刷新在跑
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("周期刷新未发生（30s 内 local 行未被再次写入）——周期行为退化")
}
