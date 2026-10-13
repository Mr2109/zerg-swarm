package backend

// watchdog_twoclock_test.go —— 两把正交时钟（liveness ＋ progress）的**两态自证**。
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §七.3 J4/J5 · §十二 Q4 参数块
// 缺口真源：`GAP-20261014-61`（旧口径 `Window 90 s × MaxWindows 5 = 450 s` 当硬上界，
//           而 132K 的一次合法 prefill ≈1380 s ⇒ 比值 3.07× ⇒ 结构性误杀）
//
// ★ 两态（**仓外夹具**驱动：本文件只做读数，夹具是 `~/.hermes/cache/scratch/2.5.15-波3/` 下的脚本）：
//
//	态 A「长时间无 progress 但有心跳」 ⇒ **不得被杀**（旧 450 s 硬上界下必死）
//	态 B「真卡」                      ⇒ **必被杀**（且必须死在 progress 上，不是死在"无输出时长"上）
//
// ★ 全部用**注入式虚拟时钟**：不真 sleep、不真读盘、不真起请求 ⇒ 毫秒级跑完，且可精确驱动到 45 min 档。

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// vclock 虚拟时钟：`after` 每被调一次就把表往前推 d，`sleep` 同理。
// cpu 每次读数按 cpuStep 递增（>0 即"工时有涨"= liveness 在续）；chunksPerCall 模拟真实 chunk 计数源。
type vclock struct {
	t             time.Time
	cpu           uint64
	cpuStep       uint64
	chunks        uint64
	chunksPerCall uint64
	progressErr   error
	progressCalls int
	// onTick 每次 `after` 推进后回调（夹具用它决定"到点就关 done"）。
	onTick func(v *vclock)
	// 读数出错开关（degraded 档）。
	cpuErr error
}

func newVClock() *vclock { return &vclock{t: time.Unix(1700000000, 0)} }

func (v *vclock) deps() watchdogDeps {
	return watchdogDeps{
		now: func() time.Time { return v.t },
		after: func(d time.Duration) <-chan time.Time {
			v.t = v.t.Add(d)
			if v.onTick != nil {
				v.onTick(v)
			}
			ch := make(chan time.Time, 1)
			ch <- v.t
			return ch
		},
		sleep: func(d time.Duration) { v.t = v.t.Add(d) },
		cpuUsec: func() (uint64, error) {
			if v.cpuErr != nil {
				return 0, v.cpuErr
			}
			v.cpu += v.cpuStep
			return v.cpu, nil
		},
		progress: func() (uint64, error) {
			v.progressCalls++
			if v.progressErr != nil {
				return 0, v.progressErr
			}
			v.chunks += v.chunksPerCall
			return v.chunks, nil
		},
	}
}

// twoClockCfg 一个"秒级压缩"的两把时钟配置（预算用秒档，便于虚拟时钟驱到 45 min 档的同一形态）。
func twoClockCfg(noProgress time.Duration) WatchdogConfig {
	return WatchdogConfig{
		Enabled: true, Window: 90 * time.Second, MaxWindows: 5, Sample: 5 * time.Second,
		DeadFlatSamples: 2, Heartbeat: DefaultHeartbeat, HeartbeatTimeout: DefaultHeartbeatTimeout,
		NoProgress: noProgress, Grace: 30 * time.Second, MaxStaleKills: DefaultMaxStaleKills,
	}
}

// runUntil 跑 Wait 到虚拟时间超过 limit 就关 done，返回判词与观测面。
func runUntil(t *testing.T, v *vclock, w *Watchdog, limit time.Duration) (WatchdogVerdict, WatchdogObservation) {
	t.Helper()
	start := v.t
	done := make(chan struct{})
	var closed bool
	v.onTick = func(c *vclock) {
		if !closed && c.t.Sub(start) >= limit {
			closed = true
			close(done)
		}
	}
	type res struct {
		v WatchdogVerdict
		r string
	}
	ch := make(chan res, 1)
	go func() {
		verdict, reason := w.Wait(done)
		ch <- res{verdict, reason}
	}()
	select {
	case got := <-ch:
		return got.v, w.Observation()
	case <-time.After(10 * time.Second):
		t.Fatalf("Wait 没在 10 s 内收敛（虚拟时钟推进异常）")
		return "", WatchdogObservation{}
	}
}

// ── 态 A：长时间无 progress、但**有心跳**，且时长在预算内 ⇒ **不得被杀** ────────────────
//
// 这是 `GAP-20261014-61` 的复现态：132K 的一次合法 prefill ≈1380 s。
// 旧口径的硬上界是 450 s ⇒ 旧看门狗**必杀**；新口径的有效预算
// = max(no_progress 45 min, prefill 132000÷96×2 ≈ 2750 s) ⇒ 1380 s 稳稳在预算内 ⇒ 不杀。
func TestTwoClock_LongPrefill_NoProgressWithHeartbeat_NotKilled(t *testing.T) {
	v := newVClock()
	v.cpuStep = 1000    // 工时有涨 ⇒ **liveness 在续**（心跳）
	v.chunksPerCall = 0 // progress **冻结**（没有任何真实 chunk 增量）
	cfg := twoClockCfg(DefaultNoProgress)
	cfg.EstCtxTokens = 132000
	cfg.TokSPerSec = 96
	budget, why := cfg.ProgressBudget()
	if budget < 1380*time.Second {
		t.Fatalf("有效预算必须盖得住 132K 的一次合法 prefill（≈1380 s）：预算=%s（%s）", budget, why)
	}

	w := newWatchdog(cfg, v.deps())
	verdict, obs := runUntil(t, v, w, 1380*time.Second) // 跑满一次合法 prefill 的时长
	if verdict != WatchdogOK {
		t.Fatalf("★ 态 A 误杀：1380 s 无 progress 但有心跳（旧 450 s 硬上界必死）⇒ 必须不被杀，实得 %s（%s）",
			verdict, obs.Reason)
	}
	if obs.HeartbeatAgoS >= obs.ProgressAgoS {
		t.Fatalf("liveness 必须比 progress **新得多**（态 A 的全部要点就是「有心跳、无推进」）："+
			"heartbeat_ago_s=%.1f progress_ago_s=%.1f", obs.HeartbeatAgoS, obs.ProgressAgoS)
	}
	if obs.ProgressAgoS < 1380 {
		t.Fatalf("progress 确实一直没动（这才是「无 progress」）：progress_ago_s=%.1f", obs.ProgressAgoS)
	}
	if obs.ProgressBudgetS <= 450 {
		t.Fatalf("预算必须已经**不再是**旧的 450 s 硬上界：progress_budget_s=%.0f", obs.ProgressBudgetS)
	}
}

// ── 态 B：真卡（progress 冻结超过预算） ⇒ **必被杀**，且死因必须是 progress ─────────────
//
// 关键：本态里**心跳一直在续**（CPU 在涨）—— 它证明「判卡只看 progress，liveness 再密也不救」。
func TestTwoClock_RealStall_MustBeKilled_OnProgressNotOnOutput(t *testing.T) {
	v := newVClock()
	v.cpuStep = 1000                     // ★ 心跳一直在续（liveness 活着）—— 仍必须被杀
	v.chunksPerCall = 0                  // progress 冻结
	cfg := twoClockCfg(60 * time.Second) // 夹具用 60 s 预算（同一形态，压缩到秒档）

	w := newWatchdog(cfg, v.deps())
	verdict, obs := runUntil(t, v, w, 10*time.Minute)
	if verdict != WatchdogStuckNoProgress {
		t.Fatalf("★ 态 B 漏杀：progress 冻结超过预算 ⇒ 必须判卡，实得 %s（%s）", verdict, obs.Reason)
	}
	if !strings.Contains(obs.Reason, "progress 冻结") || !strings.Contains(obs.Reason, "预算") {
		t.Fatalf("判词必须**死在 progress 上**（点名冻结时长与预算），实得：%s", obs.Reason)
	}
	if obs.ProgressBudgetS != 60 {
		t.Fatalf("预算读数不对：want 60 got %.0f", obs.ProgressBudgetS)
	}
	if obs.HeartbeatAgoS >= obs.ProgressAgoS {
		t.Fatalf("本态心跳是活的（正因如此才证明 liveness 不救 progress）：heartbeat_ago_s=%.1f progress_ago_s=%.1f",
			obs.HeartbeatAgoS, obs.ProgressAgoS)
	}
}

// ── 态 B′：旧 450 s 硬上界**已拆**（`no_progress: 0 ＝ 关` 那一档不是"零秒就杀"）─────────
//
// 同样"无 progress + 有心跳"，但**关掉**进度判卡（no_progress=0 且无 est_ctx/tok s）
// ⇒ 跑过 450 s 的 10 倍也不得判卡（只留"两把钟双双冻结"那条早杀）。
func TestTwoClock_ZeroNoProgressMeansOff_NotAHardBound(t *testing.T) {
	v := newVClock()
	v.cpuStep = 1000 // 有心跳（liveness 在续）
	cfg := twoClockCfg(0)

	w := newWatchdog(cfg, v.deps())
	verdict, obs := runUntil(t, v, w, 4500*time.Second) // 10 × 旧的 450 s 硬上界
	if verdict != WatchdogOK {
		t.Fatalf("no_progress=0 ⇒ 判卡**关闭**（✗ 不是「零秒就杀」）⇒ 不得判卡，实得 %s（%s）", verdict, obs.Reason)
	}
	if obs.ProgressBudgetS != 0 {
		t.Fatalf("预算应读 0（= 关）：实得 %.0f", obs.ProgressBudgetS)
	}
	if !strings.Contains(obs.BudgetWhy, "关闭") {
		t.Fatalf("预算理由必须逐字说「关闭」：%s", obs.BudgetWhy)
	}
}

// ── 态 C：**真停摆**（两把钟双双冻结） ⇒ 提前判死（不必等预算）────────────────────────
func TestTwoClock_BothClocksFrozen_KilledEarly(t *testing.T) {
	v := newVClock()
	v.cpuStep = 0 // 工时不动 ⇒ liveness 也冻结
	cfg := twoClockCfg(DefaultNoProgress)

	w := newWatchdog(cfg, v.deps())
	verdict, obs := runUntil(t, v, w, 10*time.Minute)
	if verdict != WatchdogStuckCPUStalled {
		t.Fatalf("两把钟双双冻结 ⇒ 必须提前判死（stuck_cpu_stalled），实得 %s（%s）", verdict, obs.Reason)
	}
	if !strings.Contains(obs.Reason, "双双冻结") {
		t.Fatalf("判词要点名两把钟双双冻结：%s", obs.Reason)
	}
	if obs.WindowIdx > 2 {
		t.Fatalf("应是**提前**判死（DeadFlatSamples=2），实得 window_idx=%d", obs.WindowIdx)
	}
}

// ── ★ 牙：判卡只看 progress —— **心跳绝不续 progress** ────────────────────────────────
// 连打 100 次心跳（并让虚拟时钟前进），progress 时钟**一格都不许动**；
// 而一次真实 chunk 增量必须续它。
func TestTwoClock_HeartbeatNeverResetsProgress(t *testing.T) {
	v := newVClock()
	w := newWatchdog(twoClockCfg(time.Minute), v.deps())

	base := w.LastProgressAt()
	for i := 0; i < 100; i++ {
		v.t = v.t.Add(30 * time.Second)
		w.MarkHeartbeat()
	}
	if !w.LastProgressAt().Equal(base) {
		t.Fatalf("★ 心跳把 progress 续了 —— 两把钟不再正交（progress 被永久清零的死循环就是这么来的）：base=%s now=%s",
			base, w.LastProgressAt())
	}
	if !w.LastHeartbeatAt().After(base) {
		t.Fatalf("心跳必须续 liveness：base=%s now=%s", base, w.LastHeartbeatAt())
	}

	// 真实 chunk 计数：单调增量才续；回退/持平**不算**推进。
	if !w.MarkProgressChunks(10) {
		t.Fatal("首次真实 chunk 增量必须续 progress")
	}
	after := w.LastProgressAt()
	if after.Equal(base) {
		t.Fatal("真实 chunk 增量必须推进 progress 时钟")
	}
	if w.MarkProgressChunks(9) || w.MarkProgressChunks(10) {
		t.Fatal("回退/持平的计数**不算**推进（单调，✗ 时间戳式重置）")
	}
	if !w.LastProgressAt().Equal(after) {
		t.Fatalf("回退计数不得改 progress 时钟：%s → %s", after, w.LastProgressAt())
	}
}

// ── 预算公式：`est_ctx ÷ 实测 tok/s × 2`（132K 档 ≈46 min，与设计参数表逐字对得上）──────
func TestPrefillBudget_Formula(t *testing.T) {
	got := PrefillBudget(132000, 96)
	want := time.Duration(132000.0 / 96.0 * 2 * float64(time.Second))
	if got != want {
		t.Fatalf("公式不对：want %s got %s", want, got)
	}
	if got < 45*time.Minute || got > 47*time.Minute {
		t.Fatalf("132K 档应算到 ≈46 min（设计 §十二 Q4 的例）：实得 %s", got)
	}
	if PrefillBudget(0, 96) != 0 || PrefillBudget(132000, 0) != 0 {
		t.Fatal("缺输入 ⇒ 0（「算不出来」，✗ 不许当「预算为零」去杀）")
	}
	// 有效预算取**宽**的那把。
	cfg := WatchdogConfig{NoProgress: 45 * time.Minute, EstCtxTokens: 132000, TokSPerSec: 96}
	b, why := cfg.ProgressBudget()
	if b != got {
		t.Fatalf("max(no_progress, prefill 预算) 应取 prefill 预算 %s，实得 %s（%s）", got, b, why)
	}
	if !strings.Contains(why, "取宽的那把") {
		t.Fatalf("理由要点明取宽的那把：%s", why)
	}
}

// ── 两级：先请求优雅退 ⇒ grace 到期才硬断 ─────────────────────────────────────────────
func TestEscalateStall_TwoStages(t *testing.T) {
	// ① grace 内自己退了 ⇒ 第一级够了，**不许**硬断。
	v := newVClock()
	w := newWatchdog(twoClockCfg(time.Minute), v.deps())
	graceful, hard := 0, 0
	exited := make(chan struct{})
	close(exited) // 立刻"它自己退了"
	stage, reason := w.EscalateStall(StallPlan{
		Graceful: func() { graceful++ },
		Hard:     func() { hard++ },
		Grace:    time.Minute,
		Exited:   exited,
	})
	if graceful != 1 || hard != 0 {
		t.Fatalf("第一级应发且不该硬断：graceful=%d hard=%d", graceful, hard)
	}
	if stage != WatchdogStalledGrace {
		t.Fatalf("判词应为 stalled_grace，实得 %s（%s）", stage, reason)
	}

	// ② grace 到期还没退 ⇒ 第二级硬断。
	v2 := newVClock()
	w2 := newWatchdog(twoClockCfg(time.Minute), v2.deps())
	graceful2, hard2 := 0, 0
	stage2, reason2 := w2.EscalateStall(StallPlan{
		Graceful: func() { graceful2++ },
		Hard:     func() { hard2++ },
		Grace:    30 * time.Second,
		Exited:   make(chan struct{}), // 永不关 ⇒ 只能等 grace
	})
	if graceful2 != 1 || hard2 != 1 {
		t.Fatalf("两级都要发：graceful=%d hard=%d", graceful2, hard2)
	}
	if stage2 != WatchdogStalledHard {
		t.Fatalf("判词应为 stalled_hard，实得 %s（%s）", stage2, reason2)
	}
	if w2.Observation().Stage != 2 {
		t.Fatalf("stage 应记 2（第二级）：%+v", w2.Observation())
	}
}

// ── 有界：重连封顶 `max_stale_kills`（超了 ⇒ abandoned，✗ 不无上限重置 ⇒ 死循环）────────
func TestEscalateStall_StaleKillsCap(t *testing.T) {
	v := newVClock()
	cfg := twoClockCfg(time.Minute)
	cfg.MaxStaleKills = 3
	w := newWatchdog(cfg, v.deps())
	hard := 0
	for i := 1; i <= 3; i++ {
		stage, _ := w.EscalateStall(StallPlan{Graceful: func() {}, Hard: func() { hard++ }, Grace: time.Second, Exited: make(chan struct{})})
		if stage != WatchdogStalledHard {
			t.Fatalf("第 %d 次应硬断，实得 %s", i, stage)
		}
	}
	stage, reason := w.EscalateStall(StallPlan{Graceful: func() {}, Hard: func() { hard++ }, Grace: time.Second, Exited: make(chan struct{})})
	if stage != WatchdogAbandoned {
		t.Fatalf("第 4 次应封顶 abandoned（max_stale_kills=3），实得 %s（%s）", stage, reason)
	}
	if hard != 3 {
		t.Fatalf("封顶之后**不许**再硬断：hard 调用次数=%d", hard)
	}
	if w.Observation().StaleKills != 4 {
		t.Fatalf("stale_kills 应计 4：%+v", w.Observation())
	}
}

// ── 环境旋钮齐备：两把时钟的每一档都必须**可配**（✗ 不留不可配的硬 timeout）──────────
func TestWatchdogTwoClockEnvKnobs(t *testing.T) {
	env := map[string]string{
		"ZERG_WATCHDOG":                     "1",
		"ZERG_WATCHDOG_HEARTBEAT_S":         "12",
		"ZERG_WATCHDOG_HEARTBEAT_TIMEOUT_S": "36",
		"ZERG_WATCHDOG_NO_PROGRESS_S":       "0",
		"ZERG_WATCHDOG_PREFILL_BUDGET_S":    "0",
		"ZERG_WATCHDOG_EST_CTX":             "132000",
		"ZERG_WATCHDOG_TOK_S":               "96.5",
		"ZERG_WATCHDOG_GRACE_S":             "120",
		"ZERG_WATCHDOG_MAX_STALE_KILLS":     "7",
		"ZERG_WATCHDOG_WAIVE":               "本机 128K 档：预算按现读窗口算，不设硬上界",
	}
	t.Setenv("ZERG_STOP_GRACE_S", "9") // stopGraceFromEnv 读的是**真进程 env**（不走注入器）
	got := watchdogConfigFromEnv(func(k string) string { return env[k] })
	if got.Heartbeat != 12*time.Second || got.HeartbeatTimeout != 36*time.Second {
		t.Fatalf("心跳两档没读进来：%+v", got)
	}
	if got.NoProgress != 0 {
		t.Fatalf("★ `NO_PROGRESS_S=0` 必须**收**（0 ＝ 关 是一档真语义，✗ 不是「非法值静默忽略」）：%v", got.NoProgress)
	}
	if got.EstCtxTokens != 132000 || got.TokSPerSec != 96.5 {
		t.Fatalf("现读窗口/实测速率没读进来：%+v", got)
	}
	if got.Grace != 120*time.Second || got.MaxStaleKills != 7 {
		t.Fatalf("grace / 封顶没读进来：%+v", got)
	}
	if got.Waived == "" {
		t.Fatal("豁免必须能留痕（ZERG_WATCHDOG_WAIVE）")
	}
	if stopGraceFromEnv() != 9*time.Second {
		t.Fatalf("ZERG_STOP_GRACE_S 没读进来：%s", stopGraceFromEnv())
	}
	// 默认档：每一项都有默认值，且 no_progress 默认 45 min。
	def := watchdogConfigFromEnv(func(string) string { return "" })
	if def.NoProgress != 45*time.Minute || def.Grace != 30*time.Second ||
		def.MaxStaleKills != 3 || def.Heartbeat != 30*time.Second || def.HeartbeatTimeout != 90*time.Second {
		t.Fatalf("默认档不对（设计 §十二 Q4 参数表）：%+v", def)
	}
}

// ── 观测面：**两把锁都出**（判卡只看 progress，但报告里两把都要能看见）──────────────
func TestTwoClock_BothClocksInObservation(t *testing.T) {
	v := newVClock()
	v.cpuStep = 500
	cfg := twoClockCfg(30 * time.Second)
	w := newWatchdog(cfg, v.deps())
	_, obs := runUntil(t, v, w, 5*time.Minute)
	if obs.LastHeartbeatAt == "" || obs.LastProgressAt == "" {
		t.Fatalf("两把锁都要在观测面里（last_heartbeat_at / last_progress_at）：%+v", obs)
	}
	if obs.ProgressBudgetS != 30 {
		t.Fatalf("预算读数要出：%+v", obs)
	}
	if obs.BudgetWhy == "" {
		t.Fatalf("预算理由要出（这是「为什么是这把尺」的取证）：%+v", obs)
	}
	if fmt.Sprintf("%s", obs.Verdict) != string(WatchdogStuckNoProgress) {
		t.Fatalf("应判卡：%+v", obs)
	}
}
