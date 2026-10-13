package backend

// watchdog_test.go —— 批 A（T8）：活性看门狗的六条回归判据（设计稿 §5 / 任务单 §5）。
//
// 全部用**注入式依赖**：不真 sleep、不真读盘、不真起请求 ⇒ 毫秒级跑完，且判据精确到调用次数。
//   ① 有输出 ⇒ 立刻关看门狗（**零采样**，不得引入额外读盘）
//   ② 无输出 + 工时在涨 + 窗口未满 ⇒ 重置，Extensions 递增
//   ③ 无输出 + 工时不动 ⇒ 提前判死（远早于窗口用满）
//   ④ 无输出 + 工时一直涨 + 窗口用满 ⇒ 判死 stuck_no_progress（兜住"忙等不推进"）
//   ⑤ 读不到工时 ⇒ degraded（回落固定超时，绝不误杀）
//   ⑥ ZERG_WATCHDOG=0 ⇒ 纯等 done（一键回退）

import (
	"errors"
	"testing"
	"time"
)

// fakeWD 造一套可控依赖。
// cpuVals：依次返回的工时读数（不足则重复最后一个）；err：非 nil 时读数报错。
// closeDoneAtSleep：第 N 次 sleep 时关掉 done（模拟"这时候输出来了"）。
type fakeWD struct {
	cpuVals          []uint64
	err              error
	calls            int
	sleeps           int
	closeDoneAtSleep int
	done             chan struct{}
	slotBusy         func() (bool, string)
}

func (f *fakeWD) deps() watchdogDeps {
	return watchdogDeps{
		now:   func() time.Time { return time.Unix(0, 0) },
		after: func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch },
		sleep: func(time.Duration) {
			f.sleeps++
			if f.closeDoneAtSleep > 0 && f.sleeps == f.closeDoneAtSleep {
				close(f.done)
			}
		},
		cpuUsec: func() (uint64, error) {
			if f.err != nil {
				return 0, f.err
			}
			v := f.cpuVals[min(f.calls, len(f.cpuVals)-1)]
			f.calls++
			return v, nil
		},
		slotBusy: f.slotBusy,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func wdCfg(maxWindows int) WatchdogConfig {
	return WatchdogConfig{Enabled: true, Window: 90 * time.Second, MaxWindows: maxWindows, Sample: 5 * time.Second}
}

// ① 有输出 ⇒ 立刻 ok，且**一次读数都没发生**。
func TestWatchdog_OutputClosesImmediately(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{1, 2}}
	close(f.done) // 头已经到了
	w := newWatchdog(wdCfg(5), f.deps())
	v, _ := w.Wait(f.done)
	if v != WatchdogOK {
		t.Fatalf("有输出 ⇒ 应 ok，实得 %s", v)
	}
	if f.calls != 0 {
		t.Fatalf("有输出时不得采样（不得引入额外读盘），实得读数次数 %d", f.calls)
	}
}

// ② 无输出 + 工时在涨 + 窗口未满 ⇒ 重置（Extensions 递增），随后输出到达 ⇒ ok。
func TestWatchdog_CPUProgressExtendsWindow(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{100, 200, 300, 400}, closeDoneAtSleep: 2}
	w := newWatchdog(wdCfg(5), f.deps())
	v, _ := w.Wait(f.done)
	if v != WatchdogOK {
		t.Fatalf("第 3 次前输出到达 ⇒ 应 ok，实得 %s（reason=%s）", v, w.Observation().Reason)
	}
	obs := w.Observation()
	if obs.Extensions != 2 {
		t.Fatalf("工时有涨应重置两次，实得 Extensions=%d", obs.Extensions)
	}
	if obs.CPUUsecDelta == 0 {
		t.Fatalf("观测面应记录工时增量，实得 0")
	}
}

// ③ 无输出 + 工时不动 ⇒ **提前判死**（真停摆：两把钟双双冻结）。
//
// ★ 2026-10-14（`GAP-20261014-61` 两把时钟）：早杀口径从"一个窗口内 d2 == d1 就杀"
// 收紧成"连续 `DeadFlatSamples` 次采样两把钟都不动才杀"（默认 2 ⇒ 免一次采样抖动就误杀）。
// 因此读数是 **2 个窗口 / 4 次工时**（旧口径是 1 个窗口 / 2 次）—— 变的只是**多等一个采样**，
// 仍远早于任何预算（预算默认 45 min）。断言按新口径核，不是放宽。
func TestWatchdog_CPUStalledKillsEarly(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{777, 777, 777, 777}}
	cfg := wdCfg(5)
	cfg.DeadFlatSamples = 2
	w := newWatchdog(cfg, f.deps())
	v, reason := w.Wait(f.done)
	if v != WatchdogStuckCPUStalled {
		t.Fatalf("两把钟双双冻结 ⇒ 应 stuck_cpu_stalled，实得 %s", v)
	}
	if f.calls != 4 {
		t.Fatalf("提前判死该只读 2 个窗口（4 次工时），实得 %d", f.calls)
	}
	if w.Observation().WindowIdx != 2 {
		t.Fatalf("应停在第 2 个窗口（DeadFlatSamples=2），实得 %d", w.Observation().WindowIdx)
	}
	if reason == "" {
		t.Fatal("判死必须带人类可读理由（便于主控 failover 与复盘）")
	}
}

// ④ ★ 判卡**只看 progress**：liveness 一直续（工时在涨）、progress 可读但**冻结**超过预算 ⇒ 判卡。
//
// 换形前后对照（`GAP-20261014-61`）：
//
//	旧口径：「工时一直涨 + 窗口用满（`MaxWindows×Window` = 450 s 硬上界）」⇒ 判死；
//	新口径：「progress 冻结 ≥ 有效预算（默认 45 min / 按现读窗口算的 prefill 预算）」⇒ 判卡；
//	        而**工时一直涨只续 liveness，救不回 progress**（这正是本用例在证的那一牙）。
//
// 用**虚拟时钟**驱动（真时钟要跑 45 min），判据精确到窗口数。
func TestWatchdog_NoProgressAfterMaxWindows(t *testing.T) {
	v := newVClock()
	v.cpuStep = 1000                // 工时一直涨 ⇒ liveness 在续（旧的"忙等"那一形）
	v.chunksPerCall = 0             // progress 冻结（真实 chunk 计数一动不动）
	cfg := twoClockCfg(time.Minute) // 压缩到秒档（同一形态）

	w := newWatchdog(cfg, v.deps())
	verdict, obs := runUntil(t, v, w, 10*time.Minute)
	if verdict != WatchdogStuckNoProgress {
		t.Fatalf("progress 冻结超过预算 ⇒ 应判卡 stuck_no_progress，实得 %s（%s）", verdict, obs.Reason)
	}
	if obs.ProgressBudgetS != 60 {
		t.Fatalf("预算应读出自配置（60 s），实得 %.0f", obs.ProgressBudgetS)
	}
	if obs.Extensions == 0 {
		t.Fatal("liveness 应在续（Extensions 递增）—— 正因如此才证明它不救 progress")
	}
}

// ⑤ 读不到工时 ⇒ degraded（调用方回落固定超时；**绝不**因缺读数误杀）。
func TestWatchdog_DegradedWhenUnreadable(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), err: errors.New("本机没有 cgroup（非 Linux）")}
	w := newWatchdog(wdCfg(5), f.deps())
	v, reason := w.Wait(f.done)
	if v != WatchdogDegraded {
		t.Fatalf("读数缺失 ⇒ 应 degraded，实得 %s", v)
	}
	if reason == "" {
		t.Fatal("降级也要给理由")
	}
}

// ⑥ ZERG_WATCHDOG=0 ⇒ 纯等 done（一键回退），不采样。
func TestWatchdog_DisabledFallsBackToFixedTimeout(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{1, 2}}
	close(f.done)
	cfg := wdCfg(5)
	cfg.Enabled = false
	w := newWatchdog(cfg, f.deps())
	v, _ := w.Wait(f.done)
	if v != WatchdogOK {
		t.Fatalf("关掉时只等 done ⇒ ok，实得 %s", v)
	}
	if f.calls != 0 {
		t.Fatalf("关掉时不得采样，实得 %d", f.calls)
	}
}

// 配置口径：默认与覆盖（含沙箱压秒用的小数秒）。
func TestWatchdogConfigFromEnv(t *testing.T) {
	def := watchdogConfigFromEnv(func(string) string { return "" })
	if def.Enabled || def.Window != 90*time.Second || def.MaxWindows != 5 || def.Sample != 5*time.Second {
		t.Fatalf("默认口径应为 **关**/90s/5/5s（显式开同孵化先例），实得 %+v", def)
	}
	// ★ 两把时钟的默认档（设计 §十二 Q4 参数表）：no_progress 45 min · grace 30 s · 封顶 3。
	if def.NoProgress != 45*time.Minute || def.Grace != 30*time.Second || def.MaxStaleKills != 3 ||
		def.Heartbeat != 30*time.Second || def.HeartbeatTimeout != 90*time.Second {
		t.Fatalf("两把时钟默认档不对（§十二 Q4）：%+v", def)
	}
	on := watchdogConfigFromEnv(func(k string) string {
		if k == "ZERG_WATCHDOG" {
			return "1"
		}
		return ""
	})
	if !on.Enabled {
		t.Fatal("ZERG_WATCHDOG=1 应开")
	}
	env := map[string]string{
		"ZERG_WATCHDOG":             "0",
		"ZERG_WATCHDOG_WINDOW_S":    "0.3",
		"ZERG_WATCHDOG_MAX_WINDOWS": "3",
		"ZERG_WATCHDOG_SAMPLE_S":    "0.1",
	}
	got := watchdogConfigFromEnv(func(k string) string { return env[k] })
	if got.Enabled {
		t.Fatal("ZERG_WATCHDOG=0 应关")
	}
	if got.Window != 300*time.Millisecond || got.MaxWindows != 3 || got.Sample != 100*time.Millisecond {
		t.Fatalf("覆盖口径不对：%+v", got)
	}
}
