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

// ③ 无输出 + 工时不动 ⇒ **提前判死**（只花一个窗口）。
func TestWatchdog_CPUStalledKillsEarly(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{777, 777}}
	w := newWatchdog(wdCfg(5), f.deps())
	v, reason := w.Wait(f.done)
	if v != WatchdogStuckCPUStalled {
		t.Fatalf("工时不动 ⇒ 应 stuck_cpu_stalled，实得 %s", v)
	}
	if f.calls != 2 {
		t.Fatalf("提前判死只该读两次工时，实得 %d", f.calls)
	}
	if w.Observation().WindowIdx != 1 {
		t.Fatalf("应停在第 1 个窗口，实得 %d", w.Observation().WindowIdx)
	}
	if reason == "" {
		t.Fatal("判死必须带人类可读理由（便于主控 failover 与复盘）")
	}
}

// ④ 工时一直涨但窗口用满 ⇒ stuck_no_progress（兜住"瞎花力气不推进"）。
func TestWatchdog_NoProgressAfterMaxWindows(t *testing.T) {
	f := &fakeWD{done: make(chan struct{}), cpuVals: []uint64{1, 2, 3, 4, 5, 6, 7, 8}}
	w := newWatchdog(wdCfg(3), f.deps())
	v, _ := w.Wait(f.done)
	if v != WatchdogStuckNoProgress {
		t.Fatalf("3 个窗口都只有工时在涨 ⇒ 应 stuck_no_progress，实得 %s", v)
	}
	obs := w.Observation()
	if obs.Extensions != 3 || obs.WindowIdx != 3 {
		t.Fatalf("应重置满 3 次，实得 Extensions=%d WindowIdx=%d", obs.Extensions, obs.WindowIdx)
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
