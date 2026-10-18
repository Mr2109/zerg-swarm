// expand_lock_test.go —— 「展开出临界区」的两态验收（GAP-20261018-15）。
//
// 为什么要有它：首次展开一枚 113 GB 的蛋在 edge-node 实测 471 s，旧代码把这段放在 `m.mu` 里 ⇒
// 那 471 s 内 `/eggs`、`/status`、心跳全拿不到锁，主控「探不到就按超时判红」，而模型其实起得来。
//
// 钉住的两态（任一条被改坏，本文件先红）：
//
//	① **展开窗口**：`hatchStartLocked` 调 `hatchSpecBuild` 那一段 `m.mu` **未持有**
//	   （`TryLock` 成功）—— 且此刻 `/eggs` 的读路（`EggObservations`）**不被阻塞**、
//	   能读到 `expand.state=expanding` ＋ 字节进度；
//	② **登记窗口**：重新取回锁之后（校验 / 起单元 / 登记）`m.mu` **持有**（`TryLock` 失败）。
package backend

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/eggpkg"
	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// TestHatchLockSplit_TwoStates —— 锁剖分两态。
func TestHatchLockSplit_TwoStates(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	const model = "GLM-5.3-Flash"

	t.Run("展开窗口_mu未持有_且进度面可读", func(t *testing.T) {
		entry := hatchTestEntry(model)
		m := newHatchTestManager(&fakeHatcher{})
		sp := &subproc{model: model, entry: entry, state: StateLoading, port: 45999}
		m.procs[model] = sp

		orig := hatchSpecBuild
		defer func() { hatchSpecBuild = orig }()
		entered := make(chan struct{})
		release := make(chan struct{})
		lockFree := false
		hatchSpecBuild = func(e *registry.ModelEntry, impl string, port int, prof monitor.EggProfile,
			on func(eggpkg.ExpandProgress)) (hatch.Spec, error) {
			// ① 展开窗口：锁必须是空的（展开在锁外做）
			if m.mu.TryLock() {
				lockFree = true
				m.mu.Unlock()
			}
			// 模拟展开器回报一帧进度（真路径里由 eggpkg 逐段回调）
			if on != nil {
				on(eggpkg.ExpandProgress{
					State: "expanding", BytesDone: 7, BytesTotal: 10,
					SegmentsDone: 1, SegmentsTotal: 2, PID: 1234,
				})
			}
			close(entered)
			<-release
			return hatch.Spec{}, errors.New("stub：本用例只验锁的形态，不真孵")
		}

		finished := make(chan struct{})
		go func() {
			m.mu.Lock() // 调用方（doStart）本来就持锁进来
			_, _ = m.hatchStartLocked(model, entry, sp, 45999, monitor.EggProfile{})
			m.mu.Unlock()
			close(finished)
		}()

		<-entered
		if !lockFree {
			t.Fatal("展开窗口里 m.mu 仍被持有（TryLock 失败）—— 展开没出临界区")
		}
		// 进度面两态之「展开中」：/eggs 的读路必须现在就能回，且带 expanding ＋ 字节
		type ro struct {
			obs  []EggObservation
			used time.Duration
		}
		ch := make(chan ro, 1)
		go func() {
			t0 := time.Now()
			ch <- ro{obs: m.EggObservations(), used: time.Since(t0)}
		}()
		select {
		case got := <-ch:
			if len(got.obs) != 1 {
				t.Fatalf("展开中 /eggs 应恰看到这枚卵，实得 %d 条", len(got.obs))
			}
			e := got.obs[0].Expand
			if e == nil {
				t.Fatal("展开中 /eggs 的 expand 子对象缺席（进度面等于没接线）")
			}
			if e.State != "expanding" || e.BytesDone != 7 || e.BytesTotal != 10 ||
				e.SegmentsDone != 1 || e.SegmentsTotal != 2 {
				t.Fatalf("展开中 /eggs 的进度读数不对：%+v", *e)
			}
			if got.obs[0].State != StateLoading {
				t.Fatalf("展开期状态机的词仍是 loading（不动词表），实得 %q", got.obs[0].State)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("展开窗口里 /eggs 的读路被 m.mu 锁死（>3s 没回）—— 正是本缺口要治的形态")
		}

		close(release)
		<-finished
		// 失败收场后：占位清掉、进度面回到「不在展开」（不把旧帧当现读）
		if _, ok := m.procs[model]; ok {
			t.Fatal("孵化声明映射失败后不应把占位条目留在账本里")
		}
		if sp.expandPhase != "" || sp.expandDone != 0 {
			t.Fatalf("失败后进度面应清掉，实得 phase=%q done=%d", sp.expandPhase, sp.expandDone)
		}
	})

	t.Run("登记窗口_mu持有", func(t *testing.T) {
		entry := hatchTestEntry(model)
		m := newHatchTestManager(&fakeHatcher{})
		sp := &subproc{model: model, entry: entry, state: StateLoading, port: 45998}
		m.procs[model] = sp

		orig := hatchSpecBuild
		defer func() { hatchSpecBuild = orig }()
		hatchSpecBuild = func(e *registry.ModelEntry, impl string, port int, prof monitor.EggProfile,
			on func(eggpkg.ExpandProgress)) (hatch.Spec, error) {
			return hatch.Spec{}, errors.New("stub")
		}
		held := false
		hatchRegLockedHook = func() {
			// ② 登记窗口：锁必须已被取回（TryLock 必须失败）
			if m.mu.TryLock() {
				held = false
				m.mu.Unlock()
				return
			}
			held = true
		}
		defer func() { hatchRegLockedHook = nil }()

		m.mu.Lock()
		_, _ = m.hatchStartLocked(model, entry, sp, 45998, monitor.EggProfile{})
		m.mu.Unlock()

		if !held {
			t.Fatal("登记/起单元那一段没有持锁（TryLock 成功）—— 临界区被剖得太碎，账本会与真单元错位")
		}
	})
}

// TestHatchLockSplit_StallProgressClearedOnSuccess 收尾清进度：成功路径上展开窗口结束后
// `expand` 必须回到缺席（否则 `/eggs` 会永远报一个不动的 expanding）。
func TestHatchLockSplit_ProgressClearedAfterWindow(t *testing.T) {
	t.Setenv(EnvHatch, "1")
	const model = "GLM-5.3-Flash"
	entry := hatchTestEntry(model)
	m := newHatchTestManager(&fakeHatcher{})
	sp := &subproc{model: model, entry: entry, state: StateLoading, port: 45997}
	m.procs[model] = sp

	orig := hatchSpecBuild
	defer func() { hatchSpecBuild = orig }()
	var emitted bool
	hatchSpecBuild = func(e *registry.ModelEntry, impl string, port int, prof monitor.EggProfile,
		on func(eggpkg.ExpandProgress)) (hatch.Spec, error) {
		if on != nil {
			on(eggpkg.ExpandProgress{State: "expanding", BytesDone: 3, BytesTotal: 9, SegmentsDone: 1, SegmentsTotal: 3})
			emitted = true
		}
		return hatch.Spec{}, errors.New("stub")
	}

	m.mu.Lock()
	_, _ = m.hatchStartLocked(model, entry, sp, 45997, monitor.EggProfile{})
	m.mu.Unlock()

	if !emitted {
		t.Fatal("测试前置失效：桩一次进度都没发（这条断言就没意义）")
	}
	if sp.expandPhase != "" {
		t.Fatalf("展开窗口结束后 expand 必须缺席，实得 phase=%q", sp.expandPhase)
	}
	if strings.Contains(sp.expandPhase, "expanding") {
		t.Fatal("残留 expanding 会让 /eggs 永远报「正在展开」")
	}
}
