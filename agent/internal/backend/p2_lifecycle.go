// p2_lifecycle.go —— P2：状态机与竞态修补（《设计-子端沙箱化-20260914》§7 / §7.7 / §7.2 / §13 Q5/Q6）。
//
// 本文件承载三件事（分三批落地，口径全部已拍定，Mr2109 2026-09-15）：
//
//  1. 在飞引用计数（唯一真源）+ 卸载锁内赢权（§7.7 修补 3 / §7.1 补记）：
//     「在飞」只有一个真源——子端自己的引用计数 subproc.inflight；引擎 /slots（parseSlotBusy）
//     降为交叉校验，不参与任何卸载/切换判定（ds4 类引擎未必提供 /slots——正确性不挂在引擎特性上）。
//     卸载必须「从状态里赢得权利」：锁内判（状态==空窗计时中 且 到期 且 在飞==0）⇒ 锁内置 draining
//     ⇒ 出锁后才停进程。请求侧在同一把锁上计数 +1/唤醒——谁先拿锁谁赢，输方看到的一定是新状态。
//
//  2. 显式状态「空窗计时中」（idle armed）+ 时间戳低频巡检（§7.7 修补 1/2）：
//     状态串 ready → 空窗计时中 → draining → stopped。只记「最后一次活动时间戳」（lastUsed），
//     低频巡检判 now-lastUsed > 每卵阈值（idle_unload_s，P1 已加字段；缺省 600s）。
//     **代码里不存在「sleep N 后直接杀」路径**——「重置空窗」退化为更新一个时间戳。
//
//  3. 队列出队重校验 + 不打断在飞（§7.7 修补 4 / §7.2）：见本文件下半部。
package backend

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/shared/resources"
)

// ── 状态机：显式状态「空窗计时中」（idle armed）与 draining（§7.1 补记 / §7.7 修补 1）─────

const (
	// StateIdleArmed 显式状态「空窗计时中」：在飞为零、空窗计时已启动、尚未到期的「待卸」档。
	// 缺它则「待卸」与「在跑」在实现上长得一样（两者都是"进程活着、在飞为零"）——竞态必然发生（§7.7 修补 1）。
	StateIdleArmed = "idle_armed"
	// StateDraining 排空中：已赢得收卵权利（锁内置位）、在飞归零待停/正在停。
	// 进入本状态后不再接新请求（acquireInflight 拒绝）。
	StateDraining = "draining"
)

// p2Draining 是 evictSubprocLocked 内部使用的「排空中」标记（与 StateDraining 同值，
// 独立常量仅为注释可读：卸载动作在锁内置位它、出锁后才真正停进程——§7.7 修补 3）。
const p2Draining = StateDraining

// eggIdleUnloadThreshold 每卵「空窗收走阈值」（§4.3 第九项 / §13 Q21）。
// P1 已在卵声明加 idle_unload_s 字段；未声明（<=0）取 DefaultIdleUnloadSeconds（600s）。
func eggIdleUnloadThreshold(entry *registry.ModelEntry) time.Duration {
	if entry == nil {
		return time.Duration(registry.DefaultIdleUnloadSeconds) * time.Second
	}
	return time.Duration(entry.IdleUnloadSeconds()) * time.Second
}

// p2ReapResult 单次巡检判定的内部结果（仅供 ReapIdle 组装）。
type p2ReapResult struct {
	name  string
	why   string
	entry *registry.ModelEntry
}

// p2pickExpiredLocked 锁内判定：哪些驻留卵「空窗计时中且到期且在飞==0」⇒ 赢得收卵权利。
// 这是 §7.7 修补 3 的「卸载侧」判据——判据只有三个输入，全部来自子端自身状态：
//   - 状态 == 空窗计时中（不是 ready——ready 下没在计时，谈不上到期）；
//   - now - lastActivity > 每卵阈值（时间戳口径，§7.7 修补 2；lastUsed 即最后一次活动时间戳）；
//   - 在飞引用计数 == 0（唯一真源——**不是** /slots 的看法）。
//
// 命中即在锁内置 draining（赢权），返回需出锁停进程的清单。pin 未到期、加载中、外部项照旧被挡
// （红线①③与 M10 铁律不因 P2 放宽）。调用方需持锁。
func (m *Manager) p2pickExpiredLocked(now time.Time) []p2ReapResult {
	ttl := m.idleTTL
	if ttl <= 0 {
		return nil
	}
	var out []p2ReapResult
	for name, sp := range m.procs {
		if sp == nil || name == "" {
			continue
		}
		if sp.state != StateIdleArmed {
			continue // 不是「空窗计时中」：ready（在跑）/loading（被等待）/draining（已赢权）都不在此列
		}
		if sp.inflight != 0 {
			continue // 在飞引用计数 != 0 ⇒ 不卸（§7.7 修补 3；唯一真源，/slots 不参与）
		}
		if pinned, _ := pinState(sp, now); pinned {
			continue // 红线③：pin 未到期不卸
		}
		if sp.external {
			continue // M10 铁律：外部复用项绝不进收卵路径
		}
		threshold := eggIdleUnloadThreshold(sp.entry)
		if threshold <= 0 {
			threshold = time.Duration(registry.DefaultIdleUnloadSeconds) * time.Second
		}
		// 巡检触发条件：now − lastActivity > 阈值。注意管理器级 TTL 是总开关（<=0 = 整个机制关闭），
		// 每卵阈值决定各自到期时刻（Q21：小模型 120s、缺省 600s）。
		_ = ttl
		if sp.lastUsed.IsZero() || now.Sub(sp.lastUsed) <= threshold {
			continue // 未到期
		}
		// 赢权：锁内置 draining（出锁后才停进程——§7.7 修补 3）
		sp.state = p2Draining
		out = append(out, p2ReapResult{name: name, why: fmt.Sprintf("空窗到期(空闲>%s)", threshold), entry: sp.entry})
	}
	return out
}

// ReapIdle 卸载「空窗计时中且到期且在飞==0」的驻留卵，返回被收走的别名（已排序）。
//
// P2 语义（覆盖旧 reqCount/lastUsed 直判版；旧行为向后兼容性见下）：
//   - 判据 = 状态==空窗计时中 且 到期 且 在飞==0（唯一真源：inflight 引用计数）；
//   - 锁内赢权（置 draining）⇒ 出锁后才停进程（§7.7 修补 3）；
//   - TTL<=0（未启用）⇒ 空操作（逃生门不变）。
//
// 并发纪律（-race 下的硬约束）：出锁后**只停进程句柄**（纯动作，不碰 m.procs）；
// map 摘除与记账在**重新持锁**的 finalize 里做。两段式保证：
//   - 停进程期间任何请求/巡检读 map 都安全（m.mu 未被长时间占用）；
//   - 摘除前卵已是 draining ⇒ acquireInflight 拒绝（不可能给将死卵计数）；
//   - finalize 幂等（先查 draining 归属），防与其它卸载路径双停。
func (m *Manager) ReapIdle(now time.Time) []string {
	m.mu.Lock()
	winners := m.p2pickExpiredLocked(now)
	type victim struct {
		name string
		sp   *subproc
		why  string
		proc *exec.Cmd // 停进程所需句柄（接管——后续动作不再经 sp 读共享字段）
	}
	victims := make([]victim, 0, len(winners))
	for _, w := range winners {
		sp, ok := m.procs[w.name]
		if !ok || sp.state != p2Draining {
			continue
		}
		victims = append(victims, victim{name: w.name, sp: sp, why: w.why, proc: sp.proc})
	}
	m.mu.Unlock()

	var reaped []string
	for _, v := range victims {
		// 出锁：只做停进程这个「动作」（§7.7 修补 3 的本意——不占锁等引擎退出）
		m.stopVictimProcess(v.proc, v.sp)
		// 重新持锁：map 摘除 + 占用记账 + 事件日志（与所有读者互斥）
		m.mu.Lock()
		if cur, ok := m.procs[v.name]; ok && cur == v.sp {
			gb := resources.OccupiedGb(residentEntryOf(v.name, v.sp, time.Now()))
			log.Printf("[backend] %s: 卸载 %s (释放 ≈%.1fGB)", v.why, v.name, gb)
			delete(m.procs, v.name)
			reaped = append(reaped, v.name)
		}
		m.mu.Unlock()
	}
	if len(reaped) > 0 {
		sortStrings(reaped)
	}
	return reaped
}

// stopVictimProcess 停一个已赢权的进程句柄（SIGTERM → 等 5s → SIGKILL）。
// 与 stopSubproc 相同的动作序列，但**只操作句柄参数**、不读写任何共享状态——
// 这是「出锁后才动手」的落地：动作阶段与锁完全无关，-race 干净。
// 观测钩子：m.stopHook 非 nil 时先回调（此刻状态必为 draining——锁内已赢权）。
func (m *Manager) stopVictimProcess(cmd *exec.Cmd, sp *subproc) {
	if sp != nil && m.stopHook != nil {
		m.stopHook(sp)
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[backend] SIGTERM 发送失败: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			log.Printf("[backend] 进程退出: %v", err)
		} else {
			log.Printf("[backend] 进程已优雅退出")
		}
	case <-time.After(5 * time.Second):
		log.Printf("[backend] 等待退出超时 (5s)，发送 SIGKILL")
		cmd.Process.Kill()
		cmd.Wait()
	}
}

// sortStrings 小排序（避免为此引入 sort 到本文件——residency.go 已有）。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ── 在飞引用计数（唯一真源）───────────────────────────────────────────────────

// acquireInflight 请求到达：同一把锁下 计数+1、更新活动时间戳、并按状态机迁移：
//   - 空窗计时中 ⇒ 转 ready（取消空窗——§7.1 补记第 2 行：请求到达取消空窗，不换卵不停进程）；
//   - ready ⇒ 保持（有在飞）。
//
// 返回 false 表示该卵当前不可服务（未驻留 / loading / crashed / draining / sleeping）——
// 调用方（server 层/Start 复用路径）应走加载流程而不是直接转发。
// 谁先拿到锁谁赢：本调用与巡检卸载在同一把锁上竞争，输方一定看到新状态（§7.7 修补 3）。
func (m *Manager) acquireInflight(model string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok {
		return false
	}
	switch sp.state {
	case StateReady, StateIdleArmed:
		sp.inflight++
		sp.lastUsed = time.Now() // 最后活动时间戳：请求到达即更新（§7.7 修补 2）
		if sp.state == StateIdleArmed {
			sp.state = StateReady // 取消空窗：计时戳已归零（fresh lastUsed），状态回 ready
			log.Printf("[backend] 空窗取消: model=%s（请求到达，回 ready）", model)
		}
		return true
	default:
		// loading / crashed / draining / sleeping：不可服务
		return false
	}
}

// releaseInflight 请求完成（成功或失败都算完成）：同一把锁下 计数−1。
// 最后一个在飞完成（归零）且卵仍 ready ⇒ 转「空窗计时中」（§7.1 补记第 1 行：
// 最后一个在飞请求完成 ⇒ 只记最后一次活动时间戳，不起定时器）。
func (m *Manager) releaseInflight(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok {
		return // 卵已被收走：无害空操作（竞态下的合法路径）
	}
	if sp.inflight > 0 {
		sp.inflight--
	}
	if sp.inflight != 0 {
		return
	}
	// 归零：最后一个在飞完成 ⇒ 转「空窗计时中」（只记时间戳，不起定时器——§7.7 修补 2）
	if sp.state == StateReady {
		sp.state = StateIdleArmed
		sp.lastUsed = time.Now() // 空窗计时起点 = 最后一个在飞完成时刻
		log.Printf("[backend] 空窗计时中: model=%s（在飞清零，阈值=%s）", model, eggIdleUnloadThreshold(sp.entry))
	}
}

// inflightCount 读某模型当前在飞数（观测面/交叉校验用；不持锁版本供测试与日志）。
func (m *Manager) inflightCount(model string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sp, ok := m.procs[model]; ok {
		return sp.inflight
	}
	return 0
}

// ── 队列出队重校验 + 不打断在飞（批 3，§7.7 修补 4 / §7.2）────────────────────────

// qReq 等待队列项：切换期到达的请求挂起在此，出队时必须重校验当前卵。
type qReq struct {
	model string // 它要的模型（入队时的快照——出队时可能已不是当前卵）
	req   interface{}
	// eta 预计等待（Q5：排队必须含 ETA；出队/入队时估算填充）
	eta time.Duration
}

// p2Queue 最小等待队列（§7.2：切换期到达的请求挂起；§13 Q5：限长 + ETA）。
// mu 保护全部字段；notify 用于唤醒出队方。
type p2Queue struct {
	mu      sync.Mutex
	items   []qReq
	maxLen  int
	notify  chan struct{}
	dropped int
}

// newP2Queue 造一个限长队列（maxLen<=0 用默认 64）。
func newP2Queue(maxLen int) *p2Queue {
	if maxLen <= 0 {
		maxLen = 64
	}
	return &p2Queue{maxLen: maxLen, notify: make(chan struct{}, 1)}
}

// push 入队。队列满 ⇒ 立即 false（上层回 429——Q5：不排队、不挂起）。
func (q *p2Queue) push(r qReq) bool {
	q.mu.Lock()
	if len(q.items) >= q.maxLen {
		q.mu.Unlock()
		return false
	}
	q.items = append(q.items, r)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return true
}

// popDequeue 出队 + 重校验（§7.7 修补 4 核心）：取队头后，先问当前卵是不是它要的模型——
//   - 一致 ⇒ 正常出队；
//   - 不一致（卵已换/已被收）⇒ **不得直接转发**：把该项重新送回队首并返回 mismatch，
//     让上层重走孵化流程（Start/doStart 那条）；重走时该项仍留在队首，避免饿死。
//
// 返回 (item, ok, mismatch)。ok=false 表示队列空（等待 notify 或直接返回）。
func (q *p2Queue) popDequeue(currentModel func() string, wait bool) (qReq, bool, bool) {
	for {
		q.mu.Lock()
		if len(q.items) == 0 {
			q.mu.Unlock()
			if !wait {
				return qReq{}, false, false
			}
			<-q.notify
			continue
		}
		head := q.items[0]
		q.mu.Unlock()
		cur := currentModel()
		if cur == head.model {
			// 重校验一致：真正出队（再锁一次取走队头——CAS 式两段，避免持锁做外部调用）
			q.mu.Lock()
			if len(q.items) > 0 && q.items[0].model == head.model {
				q.items = q.items[1:]
				q.mu.Unlock()
				return head, true, false
			}
			q.mu.Unlock()
			continue // 队头已变（并发出队），重来
		}
		// 重校验不一致：卵已换 ⇒ 不出队、上报 mismatch（上层重走孵化；队列保持队头）
		return head, false, true
	}
}

// lenOf 当前队列长度（观测面）。
func (q *p2Queue) lenOf() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// p2QueueCapacity 队列限长默认值（Q5：队列必须限长——一满立即 429，防雪崩）。
const p2QueueCapacity = 64
