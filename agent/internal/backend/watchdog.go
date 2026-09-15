package backend

// watchdog.go —— 活性看门狗（虫须判活）：把「卡死」从**数秒数**改成**看证据**。
//
// 设计真源：docs/01-设计/设计-活性看门狗-20260915.md（Mr2109 2026-09-15 提出并认可）
// 任务单：docs/项目文档/v2.5.9/任务单-活性看门狗-20260915.md（批 A = 本文件 + 读数 + 用例）
//
// 要解决的问题（两条都是当日生产实证）：
//   · 非流式：引擎要**等整篇生成完**才回响应头 ⇒ 固定的"90 s 首字节超时"把**正常的长文**误杀；
//   · 长 prefill：几十分钟的预填充期间**一个 token 都不出** ⇒ 同样被误杀。
//
// 判据（三级证据，卡点不是死刑）：
//
//	窗口（默认 90 s）到点 → 看有没有输出；
//	  ① 有输出                                    ⇒ 关看门狗（ok）
//	  ② 无输出，但**本单元 CPU 工时在涨**            ⇒ 重置窗口（最多 MaxWindows 次，默认 5）
//	  ③ 无输出，且**本单元 CPU 工时不动**            ⇒ **提前判死**（stuck_cpu_stalled）
//	  ④ 无输出，工时一直涨但窗口用满                ⇒ 判死（stuck_no_progress，兜住"忙等不推进"）
//	  读不到工时（非 Linux/无 systemd/权限）        ⇒ degraded ⇒ 调用方回落固定超时（不误杀）
//
// 为什么用"本单元 CPU 工时"而不是整机 CPU%/GPU%：整机指标全机共享，隔壁卵/别的进程一忙就会
// 给"这枚卵还在干活"作伪证（见 monitor.EggCPUUsecFromCgroup 注释）。

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// WatchdogVerdict 判词（进观测面与错误响应，供主控 failover 与人工复盘）。
type WatchdogVerdict string

const (
	WatchdogOK              WatchdogVerdict = "ok"
	WatchdogExtended        WatchdogVerdict = "extended"
	WatchdogStuckCPUStalled WatchdogVerdict = "stuck_cpu_stalled"
	WatchdogStuckNoProgress WatchdogVerdict = "stuck_no_progress"
	WatchdogDegraded        WatchdogVerdict = "degraded"
)

// WatchdogConfig 看门狗参数（环境变量可覆盖；沙箱压秒测试也用它们）。
type WatchdogConfig struct {
	Enabled    bool
	Window     time.Duration // 卡点窗口（默认 90 s）
	MaxWindows int           // 最多重置几次（默认 5 ⇒ 硬上界 450 s）
	Sample     time.Duration // 两次工时读数间隔（默认 5 s）
	// DeadFlatSamples 正文阶段（T14）：连续几次采样"无推进且 CPU 工时也不动"就**立刻判死**
	// （提前判死，不必等窗口用满；默认 2 ⇒ 10 s）。这是"彻底停摆"的证据，与"忙但不推进"区分开。
	DeadFlatSamples int
}

// watchdogConfigFromEnv 读环境变量。
//
// 开关口径（与 `ZERG_HATCH` 同一先例）：**默认关，显式开** —— `ZERG_WATCHDOG=1/true/yes/on`。
// 为什么默认关（2026-09-15 实测教训）：开着时若证据读不到（测试环境/非 Linux 没有 cgroup）
// 会走 degraded 回落固定超时，把"对不响应引擎"的既有用例拖到 90s/15min；且本机制属行为变化，
// 按"先沙箱验证、再生产 drop-in 显式开"的纪律更稳。关着时行为与旧口径**逐字一致**（一键回退）。
// 窗口/采样用**秒（可小数）**，便于沙箱压秒验证。
func watchdogConfigFromEnv(get func(string) string) WatchdogConfig {
	cfg := WatchdogConfig{Enabled: false, Window: 90 * time.Second, MaxWindows: 5, Sample: 5 * time.Second, DeadFlatSamples: 2}
	switch v := strings.ToLower(strings.TrimSpace(get("ZERG_WATCHDOG"))); v {
	case "1", "true", "yes", "on":
		cfg.Enabled = true
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_WINDOW_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Window = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_MAX_WINDOWS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxWindows = n
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_SAMPLE_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Sample = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_DEAD_FLAT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DeadFlatSamples = n
		}
	}
	return cfg
}

// WatchdogObservation 观测面（设计 §6；字段名逐字对应）。
type WatchdogObservation struct {
	WindowIdx      int             `json:"window_idx"`
	Extensions     int             `json:"extensions"`
	LastOutputAgoS float64         `json:"last_output_ago_s"`
	CPUUsecDelta   uint64          `json:"cpu_usage_usec_delta"`
	Verdict        WatchdogVerdict `json:"verdict"`
	Reason         string          `json:"reason"`
}

// watchdogDeps 注入式依赖：单测不真 sleep、不真读盘（时间与读数都可控）。
type watchdogDeps struct {
	now      func() time.Time
	after    func(time.Duration) <-chan time.Time
	sleep    func(time.Duration)
	cpuUsec  func() (uint64, error)
	slotBusy func() (bool, string) // 可 nil（引擎不提供 /slots ⇒ 只靠工时判据）
}

func realWatchdogDeps(cpuUsec func() (uint64, error), slotBusy func() (bool, string)) watchdogDeps {
	return watchdogDeps{now: time.Now, after: time.After, sleep: time.Sleep, cpuUsec: cpuUsec, slotBusy: slotBusy}
}

// Watchdog 单次请求的活动看门狗。零值不可用，用 newWatchdog 构造。
type Watchdog struct {
	cfg        WatchdogConfig
	deps       watchdogDeps
	obs        WatchdogObservation
	lastOutput time.Time
}

func newWatchdog(cfg WatchdogConfig, deps watchdogDeps) *Watchdog {
	return &Watchdog{cfg: cfg, deps: deps}
}

// newRealWatchdog 生产构造：工时读数由调用方给出（按单元的 cgroup 目录读）。
func newRealWatchdog(cfg WatchdogConfig, cpuUsec func() (uint64, error), slotBusy func() (bool, string)) *Watchdog {
	return newWatchdog(cfg, realWatchdogDeps(cpuUsec, slotBusy))
}

// Observation 取观测面（判死后由调用方写进错误响应/事件帧）。
func (w *Watchdog) Observation() WatchdogObservation { return w.obs }

// MarkOutput 记录"有输出"（响应头/分块到达）。一旦有输出，看门狗即刻退场。
func (w *Watchdog) MarkOutput() { w.lastOutput = w.deps.now() }

// Wait 等「响应头到达」（done 关闭即正常）。返回：
//
//	ok            —— 头到了（正常路径）
//	stuck_*       —— 判死（调用方应 cancel 请求并回 503 + reason）
//	degraded      —— 读不到证据（调用方回落固定超时，行为与旧口径一致）
func (w *Watchdog) Wait(done <-chan struct{}) (WatchdogVerdict, string) {
	w.lastOutput = w.deps.now()

	if !w.cfg.Enabled {
		<-done
		w.obs.Verdict = WatchdogOK
		w.obs.Reason = "看门狗未启用（ZERG_WATCHDOG=0）⇒ 纯固定超时路径"
		return WatchdogOK, ""
	}

	for window := 0; window < w.cfg.MaxWindows; window++ {
		// **输出优先**（先做一次非阻塞检查）：头若已经回来，就不再花这个窗口取证据 ——
		// 否则 select 在两个就绪通道间随机选，可能"抢在 done 前"把已经正常的请求判死。
		select {
		case <-done:
			w.obs.Verdict = WatchdogOK
			w.obs.Reason = ""
			w.obs.LastOutputAgoS = w.deps.now().Sub(w.lastOutput).Seconds()
			return WatchdogOK, ""
		default:
		}

		select {
		case <-done:
			w.obs.Verdict = WatchdogOK
			w.obs.Reason = ""
			w.obs.LastOutputAgoS = w.deps.now().Sub(w.lastOutput).Seconds()
			return WatchdogOK, ""
		case <-w.deps.after(w.cfg.Window):
		}

		// 卡点到（不是死刑）：取"有没有在花力气"的证据
		d1, err := w.deps.cpuUsec()
		if err != nil {
			w.obs.Verdict = WatchdogDegraded
			w.obs.Reason = fmt.Sprintf("读不到本单元 CPU 工时（%v）⇒ 降级为固定超时（不误杀）", err)
			return WatchdogDegraded, w.obs.Reason
		}
		w.deps.sleep(w.cfg.Sample)
		d2, err := w.deps.cpuUsec()
		if err != nil {
			w.obs.Verdict = WatchdogDegraded
			w.obs.Reason = fmt.Sprintf("第二次读工时失败（%v）⇒ 降级为固定超时（不误杀）", err)
			return WatchdogDegraded, w.obs.Reason
		}

		w.obs.WindowIdx = window + 1
		w.obs.LastOutputAgoS = w.deps.now().Sub(w.lastOutput).Seconds()

		if d2 > d1 {
			w.obs.CPUUsecDelta = d2 - d1
			w.obs.Extensions++
			w.obs.Verdict = WatchdogExtended
			note := ""
			if w.deps.slotBusy != nil {
				if busy, detail := w.deps.slotBusy(); !busy {
					note = fmt.Sprintf("；注：/slots 说没有在处理（%s）", detail)
				}
			}
			w.obs.Reason = fmt.Sprintf("第 %d 个窗口无输出，但本单元 CPU 工时在涨（+%d µs）⇒ 重置（最多 %d 次）%s",
				window+1, d2-d1, w.cfg.MaxWindows, note)
			continue
		}

		w.obs.CPUUsecDelta = 0
		w.obs.Verdict = WatchdogStuckCPUStalled
		w.obs.Reason = fmt.Sprintf("第 %d 个窗口无输出，且本单元 CPU 工时一点不动（%d µs）⇒ 判卡住（提前，不必等满）", window+1, d2)
		return WatchdogStuckCPUStalled, w.obs.Reason
	}

	w.obs.CPUUsecDelta = 0
	w.obs.Verdict = WatchdogStuckNoProgress
	w.obs.Reason = fmt.Sprintf("%d 个窗口（约 %s）内一无输出：CPU 虽在涨但无任何推进 ⇒ 判卡住",
		w.cfg.MaxWindows, time.Duration(w.cfg.MaxWindows)*w.cfg.Window)
	return WatchdogStuckNoProgress, w.obs.Reason
}

// ── 小工具：环境变量读取（与既有代码同一风格）────────────────────────────────

func watchdogEnv(key string) string { return os.Getenv(key) }
