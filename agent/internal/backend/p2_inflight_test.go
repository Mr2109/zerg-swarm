package backend

// p2_inflight_test.go —— P2 批 1：在飞引用计数为唯一真源 + 卸载锁内赢权。
//
// 设计依据：设计-子端沙箱化-20260914.md §7.7 修补 3 / §7.1 补记（在飞判据唯一真源）/ §13 Q6。
// 口径（Mr2109 2026-09-15 拍定）：
//   - 「在飞」的唯一真源 = 子端自己的在飞引用计数（infInflight）；/slots（parseSlotBusy）只作交叉校验，
//     不参与卸载/切换判定，且引擎不提供 /slots 时一切照常工作；
//   - 卸载必须「从状态里赢得权利」：锁内判（可卸条件 + 在飞==0）⇒ 锁内置 draining ⇒ 出锁后才停进程；
//   - 有在飞 ⇒ 卸载被拒（跳过），绝不硬杀在飞生成（§7.2 / §13 Q6）。

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// p2Manager 造一个带 TTL 总开关的管理器（P2 巡检判定的前提：idleTTL>0）。
func p2Manager(limit int, procs map[string]*subproc) *Manager {
	m := newEvictTestManager(limit, procs)
	m.SetIdleTTL(300 * time.Second)
	return m
}

// inflightEntry 造一个带空窗阈值的卵条目（测试用，cmd 指向不存在的二进制防真起进程）。
func inflightEntry() *registry.ModelEntry {
	return &registry.ModelEntry{
		Backend: "llama-server",
		File:    "/data/models/p2/inflight.gguf",
		MemGB:   4,
		Cmd:     registry.CmdString("/nonexistent/zerg-p2-noop-binary"),
	}
}

// 正例：在飞>0 ⇒ ReapIdle 拒卸（existing TestTTL_InflightAndLoadingNotReaped 已覆盖 reqCount 路径，
// 这里钉住新口径：**计时中**的到期项有在飞也不许卸——锁内赢权判定含「在飞==0」）。
func TestP2B1_InflightNotReapedWhileIdleArmedExpired(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"busy": {model: "busy", entry: inflightEntry(), port: 9411,
			state: StateIdleArmed, lastUsed: now.Add(-time.Hour), inflight: 1},
	})
	reaped := m.ReapIdle(now)
	if len(reaped) != 0 {
		t.Fatalf("在飞>0 时到期也不得卸载（§7.7 修补3），实得 %v", reaped)
	}
	if _, still := m.procs["busy"]; !still {
		t.Fatal("在飞>0 的驻留被卸载了——「不打断在飞生成」被破（§7.2/Q6）")
	}
	// 在飞归零后，同一时刻巡检应能赢权卸载（条件是「在飞==0」而非「/slots 说闲」）
	m.mu.Lock()
	m.procs["busy"].inflight = 0
	m.mu.Unlock()
	reaped = m.ReapIdle(now)
	if len(reaped) != 1 || reaped[0] != "busy" {
		t.Fatalf("在飞==0 且已到期 ⇒ 应赢权卸载，实得 %v", reaped)
	}
}

// 批 1 核心竞态：请求计数 +1 与卸载同时抢锁——谁先拿到锁谁赢，输的一方一定看到新状态。
//   - 计数先赢 ⇒ 卸载侧读到 inflight==1 ⇒ 拒卸；
//   - 卸载先赢 ⇒ 项已被移除 ⇒ 计数侧不得对已死卵 +1。
func TestP2B1_Race_IncrementVsReap(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"m": {model: "m", entry: inflightEntry(), port: 9412,
			state: StateIdleArmed, lastUsed: now.Add(-2 * time.Hour)},
	})

	var wg sync.WaitGroup
	results := make(chan string, 2)

	// goroutine A：请求到达（acquireInflight——锁内 +1）
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Microsecond) // 制造交错
		ok := m.acquireInflight("m")
		if ok {
			results <- "inc"
		} else {
			results <- "inc-failed"
		}
	}()

	// goroutine B：巡检卸载（ReapIdle——锁内赢权）
	wg.Add(1)
	go func() {
		defer wg.Done()
		reaped := m.ReapIdle(now)
		if len(reaped) > 0 {
			results <- "reaped"
		} else {
			results <- "reap-passed"
		}
	}()

	wg.Wait()
	close(results)

	got := map[string]int{}
	for r := range results {
		got[r]++
	}
	m.mu.Lock()
	_, stillThere := m.procs["m"]
	inflight := 0
	if stillThere {
		inflight = m.procs["m"].inflight
	}
	m.mu.Unlock()

	// 不变量①：不存在「卵被卸了、但计数还对它 +1 成功」的组合
	// （要么 inc 成功且卵还在且未卸；要么卸成功且 inc 失败）
	if got["inc"] > 0 {
		if !stillThere {
			t.Fatalf("竞态不变量被破：计数 +1 成功但卵已消失（结果=%v）", got)
		}
		if got["reaped"] > 0 {
			t.Fatalf("竞态不变量被破：同一轮既 +1 成功又被卸载（结果=%v）", got)
		}
	}
	// 不变量②：卸载成功时不得留下在飞计数
	if got["reaped"] > 0 && stillThere {
		t.Fatalf("卸载成功但卵还在 procs 里（结果=%v）", got)
	}
	_ = inflight
}

// 竞态的负向锁死检查：反复并发「计数±1 × 到期卸载」，最终状态必须自洽
// （要么卸干净，要么剩下的项在飞>0 或未到期——绝不出现「在飞==0 且已到期却没被卸」的半死态）。
func TestP2B1_Race_Hammer(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"m": {model: "m", entry: inflightEntry(), port: 9413,
			state: StateIdleArmed, lastUsed: now.Add(-24 * time.Hour)},
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if m.acquireInflight("m") {
					m.releaseInflight("m")
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			m.ReapIdle(now)
		}
	}()
	wg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()
	if sp, ok := m.procs["m"]; ok {
		// 还在 ⇒ 必须处于自洽态。注意 lastUsed 是「最后一次活动时间戳」（P2 口径）：
		// 并发请求可能刚把它刷新（lastUsedAgo 为负或极小），此刻未到期属正常——
		// 自洽性的准确表述：在飞>0，或时间戳尚未越过阈值（now-lastUsed <= 阈值）。
		if sp.inflight == 0 && sp.state == StateIdleArmed {
			ago := now.Sub(sp.lastUsed)
			if ago > 300*time.Second {
				t.Fatalf("自洽性被破：卵在「空窗计时中」且在飞==0 且已越过阈值，却没被巡检收走（state=%s inflight=%d ago=%v）", sp.state, sp.inflight, ago)
			}
		}
	}
}

// 单一真源：引擎不提供 /slots（ds4 类）⇒ 卸载/切换判定照常工作（正确性不挂在引擎特性上）。
// 本用例全程不探 /slots（卵的 port 指向一个没有 /slots 的本地监听也不影响判定路径）。
func TestP2B1_JudgementWithoutSlots(t *testing.T) {
	now := time.Now()
	m := p2Manager(3, map[string]*subproc{
		"ds4egg": {model: "ds4egg", entry: inflightEntry(), port: 9499,
			state: StateIdleArmed, lastUsed: now.Add(-time.Hour)},
	})
	reaped := m.ReapIdle(now)
	if len(reaped) != 1 || reaped[0] != "ds4egg" {
		t.Fatalf("无 /slots 的引擎也必须能到期收卵（单一真源=在飞计数），实得 %v", reaped)
	}
}

// 卸载动作必须在「锁内置 draining 类状态、出锁后才停进程」——用 hook 验证停进程时
// 状态已是 draining（锁内赢权的可观测面）。
func TestP2B1_DrainingSetInsideLock(t *testing.T) {
	now := time.Now()
	var seenStateAtStop []string
	m := p2Manager(3, map[string]*subproc{
		"m": {model: "m", entry: inflightEntry(), port: 9414,
			state: StateIdleArmed, lastUsed: now.Add(-time.Hour)},
	})
	// 外部句柄接管路径也走 stopHook（P2 的两段式：出锁动作阶段回调观测面）。
	m.stopHook = func(sp *subproc) {
		m.mu.Lock()
		seenStateAtStop = append(seenStateAtStop, sp.state)
		m.mu.Unlock()
	}
	m.ReapIdle(now)
	if len(seenStateAtStop) != 1 {
		t.Fatalf("应恰好停一次，实得 %v", seenStateAtStop)
	}
	if seenStateAtStop[0] != StateDraining {
		t.Fatalf("停进程时状态应为 draining（锁内已赢权），实得 %s", seenStateAtStop[0])
	}
}

// acquireInflight 对未驻留/非可服务状态返回 false（不得给不存在或已死的卵计数）。
func TestP2B1_AcquireGuards(t *testing.T) {
	m := p2Manager(3, map[string]*subproc{
		"ok":     {model: "ok", entry: inflightEntry(), port: 9415, state: StateReady, lastUsed: time.Now()},
		"load":   {model: "load", entry: inflightEntry(), port: 9416, state: StateLoading},
		"crash":  {model: "crash", entry: inflightEntry(), port: 9417, state: StateCrashed},
		"drain":  {model: "drain", entry: inflightEntry(), port: 9418, state: StateDraining},
		"armed":  {model: "armed", entry: inflightEntry(), port: 9419, state: StateIdleArmed, lastUsed: time.Now()},
		"sleepy": {model: "sleepy", entry: inflightEntry(), port: 9420, state: StateSleeping, lastUsed: time.Now()},
	})
	if !m.acquireInflight("ok") || !m.acquireInflight("armed") {
		t.Fatal("ready/空窗计时中 的卵必须能计数 +1")
	}
	if m.acquireInflight("load") || m.acquireInflight("crash") || m.acquireInflight("drain") || m.acquireInflight("sleepy") {
		t.Fatal("loading/crashed/draining/sleeping 的卵不得计数（不在可服务态）")
	}
	if m.acquireInflight("ghost") {
		t.Fatal("未驻留模型不得计数成功")
	}
	m.mu.Lock()
	if m.procs["ok"].inflight != 1 || m.procs["armed"].inflight != 1 {
		t.Fatalf("计数应各为 1，实得 %d/%d", m.procs["ok"].inflight, m.procs["armed"].inflight)
	}
	m.mu.Unlock()
	m.releaseInflight("ok")
	m.mu.Lock()
	if m.procs["ok"].inflight != 0 {
		t.Fatalf("release 后应归零，实得 %d", m.procs["ok"].inflight)
	}
	m.mu.Unlock()
	m.releaseInflight("ghost") // 不存在：必须是无害空操作
	m.releaseInflight("ok")    // 双减防护：0 再减不得变负
	m.mu.Lock()
	if m.procs["ok"].inflight != 0 {
		t.Fatalf("重复 release 不得把计数减成负数，实得 %d", m.procs["ok"].inflight)
	}
	m.mu.Unlock()
}

// /slots 降级为交叉校验：parseSlotBusy 照旧存在且语义不变（探不到按忙），但它不再出现在
// 任何卸载/切换判据里——用 grep 式断言钉住：residency/manager 里不得调用 probeServiceIdle。
func TestP2B1_SlotsOnlyCrossCheck(t *testing.T) {
	// 编译期证明：residency.go / manager.go 的判定路径没有引用 probeServiceIdle / parseSlotBusy。
	// （Go 没有反射源码的 API，这里用「源文件读取」做一次结构性断言，防将来手滑接线。）
	for _, f := range []string{"residency.go", "manager.go"} {
		src, err := readFileForAssertion(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		if strings.Contains(src, "probeServiceIdle(") || strings.Contains(src, "parseSlotBusy(") {
			t.Fatalf("%s 中出现了 /slots 判据调用（§7.7 修补3：/slots 只作交叉校验、不得参与判定）", f)
		}
	}
}
