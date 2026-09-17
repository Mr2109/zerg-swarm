// ring.go — 环形缓冲 + probation + 字节/速率闸（C2/C3/C6）。
//
// 本地形态（C3：我们没有 OTel Collector 的缓冲器，tail 采样的"先缓冲后决策"必须自己做）：
//
//	到达 ──► Sampler.Decide（判定）
//	          ├─ 判留 ──► 闸（字节/速率）──► sink（落盘）   ← 闸只在这里拦，拦不住就"必留破例/非必留裁掉"
//	          └─ 判丢 ──► probation（宽限 discard_grace）──► 到期定案（账上墓碑的 probation_until 就是终局判据）
//	                          └─ 宽限期内 Overturn（晚到的判词）⇒ **翻案**：改判为留并落盘，账上追加 amendment
//
// 三条写死的口径：
//
//	① **绝不阻塞**（C2）：闸满、容量满、sink 报错 —— 一律**当次**给出结论（裁掉 / 破例 / 留碑），
//	   不 sleep、不排队等预算、不重试、不向调用方抛错。想异步就把 sink 换成 ChanSink(有界通道)。
//	② **不超容**（C3）：容量不够时按**淘汰序**提前终结 probation；淘汰不动（剩下全是必留档）时
//	   新条目**不进 probation**（直接终局留碑，degrade_level=ring）—— 宁可少一次翻案机会，也不让内存无界。
//	③ **必留永不淘汰**（C1 的延伸）：淘汰序里必留档最高；闸饱和时必留**破例放行**并计入
//	   MustKeepBypasses（破例是可量化的，不是看不见的）。
//
// 尺寸约束（C3）：ring ≥ 到达率 × 宽限期、max_inflight 同理。不满足**不报错**：宽限期会被提前终结
// （那些墓碑的 degrade_level=ring），调用方用 Config.SizingOK 自己算差值。
package sampling

import (
	"errors"
	"sync"
	"time"
)

// ErrQueueFull — ChanSink 非阻塞语义的返回值：写不进去就**立刻**报满（宁可丢，绝不等）。
var ErrQueueFull = errors.New("sampling: sink 队列已满（非阻塞投递：宁可丢也不等）")

// ChanSink — "绝不阻塞"的现成形态（C2）：投得进就投，投不进立刻返回 ErrQueueFull。
// 用法：调用方起一个写盘 goroutine，ch 给一个有界缓冲 ⇒ 写盘慢的时候丢样本而不是拖住 agent。
func ChanSink(ch chan<- Event) Sink {
	return func(ev Event) error {
		select {
		case ch <- ev:
			return nil
		default:
			return ErrQueueFull
		}
	}
}

// probationEntry — 一个"已判丢但仍在宽限期内"的条目（晚到的判词可能把它翻回来）。
type probationEntry struct {
	runID string
	key   string
	rank  int // 淘汰序位次（越大越不该被牺牲；必留档永不淘汰）
	bytes int
	seq   int64 // 同档按**最早优先**淘汰
	until time.Time
	ev    Event // 翻案时要真的把它交付出去 ⇒ 事件本身必须留着
}

func (e *probationEntry) mapKey() string { return probKey(e.runID, e.key) }

// probKey — probation 的复合键（run 内唯一）；用 0x00 分隔，避免 run/key 拼串歧义。
func probKey(runID, key string) string { return runID + "\x00" + key }

// evictRank — **淘汰序**（与判定序不是一回事，别混）：
//
//	默认拒绝(0) < 概率未中签(1) < drop 命中(2) < 必留档(3，永不淘汰)
//
// 为什么这么排：判定序回答"谁定案"，淘汰序回答"谁最不重要"。默认拒绝连判据都没命中 ⇒ 最可牺牲；
// 概率未中签至少被抽签看过一遍（信息量高一档）；drop 是**显式**判据命中的（有人写了规则要丢它，
// 这条规则本身就是要取证的事实）；必留档（含被显式否决的必留）永不淘汰 —— **容量不许把"必留"这个判词藏起来**。
func evictRank(k PolicyKind) int {
	switch k {
	case KindDefaultDeny:
		return 0
	case KindProbabilistic:
		return 1
	case KindDrop:
		return 2
	case KindMustKeep, KindOff:
		return 3
	}
	return 0
}

// rateWindow — 固定 1s 窗口的字节/条数预算（C2）。用固定窗而非滑动窗：可判定、零内存、够用。
type rateWindow struct {
	start  time.Time
	events int
	bytes  int
}

// Ring — 采样器本体：判定器 + 缓冲 + 闸 + 判定账。**并发安全**（内部一把锁，临界区内不做 IO 之外的等待）。
type Ring struct {
	cfg     Config
	sampler *Sampler
	now     func() time.Time

	mu        sync.Mutex
	ledgers   map[string]*Ledger
	runOrder  []string // LRU（最久未出现的 run 先淘汰）
	prob      map[string]*probationEntry
	probList  []*probationEntry // 插入序（淘汰时同档取最早者）
	probBytes int
	probSeq   int64
	win       rateWindow

	admits         int
	emits          int
	evictions      int
	overturns      int
	sinkFails      int
	mustKeepBypass int
	probSkipped    int // 容量淘汰不动 ⇒ 新条目不进 probation 的次数
}

// New — 建采样器（cfg 零值会被 Normalize 补默认；policies 里空名/未登记类别会被丢弃）。
func New(cfg Config, policies ...Policy) *Ring {
	n := cfg.Normalize()
	return &Ring{
		cfg:     n,
		sampler: NewSampler(n, policies...),
		now:     n.Clock,
		ledgers: map[string]*Ledger{},
		prob:    map[string]*probationEntry{},
	}
}

// Config — 归一化后的生效配置。
func (r *Ring) Config() Config { return r.cfg }

// Sampler — 底层判定器（只读用途：核对策略表与写死序）。
func (r *Ring) Sampler() *Sampler { return r.sampler }

// ── 到达 ─────────────────────────────────────────────────────────────────

// Admit — 一条事件到达：判定 + 闸 + 缓冲 + 记账 + （按判词）落盘。**永不阻塞、永不抛错**。
//
// 返回的 Decision 是**落地后**的完整事实：Verdict/DecidingPolicy 是判词与定案者，
// Emitted ⇔ 真的交给了 sink 且成功，Tombstone ⇔ 真的缺席且已留碑。
func (r *Ring) Admit(ev Event) Decision {
	now := r.now()
	d := r.sampler.Decide(ev)
	led := r.ledgerFor(ev.RunID)

	r.mu.Lock()
	r.admits++
	r.expireLocked(now) // 顺手回收过期 probation（只清内存，不写盘、不阻塞）
	r.mu.Unlock()

	if d.DryRun {
		// C4：只计算并记录决策、**不真裁** ⇒ 事件一条不落地全交出去（落盘条数 = 全量）。
		// 判词（含 drop）照记 —— 这正是 dry-run 的用处：量 must-keep 召回，不做任何牺牲。
		emitted, failed := r.emit(ev)
		deg := DegradeNone
		if failed {
			deg = DegradeSink
		}
		d.Emitted = emitted
		d.DegradeLevel = deg
		led.recordDecision(now, ev, d, emitted, false, time.Time{}, deg, false)
		return d
	}

	if d.Verdict != VerdictKeep {
		return r.probate(now, led, ev, d)
	}

	// 判留：过闸（闸只拦这里）。
	allowed, deg := r.gateAllow(ev, d.DecidingKind == KindMustKeep)
	if !allowed {
		// 闸裁掉：判词被覆盖，但"谁想留它"照样留在 DecidingPolicy 里（两个事实都不丢）。
		d.Verdict = VerdictDrop
		d.GateEnforced = true
		d.DegradeLevel = deg
		d.Reason = d.Reason + "；被闸裁掉（" + deg + "）"
		d.Tombstone = true
		// 被闸裁的**不进 probation**：闸的意义就是"慢的时候立刻减负"，把裁掉的东西再抱在内存里等于没减负。
		// 因此 probation_until 为零 ⇒ 这条墓碑**终局**（没有翻案机会），degrade_level 说明原因。
		led.recordDecision(now, ev, d, false, true, time.Time{}, deg, true)
		return d
	}

	emitted, failed := r.emit(ev)
	if !emitted {
		// 落盘失败 = 事实缺席 ⇒ 必须留碑（否则"缺席"的歧义又会从另一条路回来）。
		d.Emitted = false
		d.Tombstone = true
		if failed {
			deg = DegradeSink
		}
		d.DegradeLevel = deg
		led.recordDecision(now, ev, d, false, true, time.Time{}, deg, false)
		return d
	}
	d.Emitted = true
	d.DegradeLevel = deg // 必留破例时这里是 rate/bytes（可量化：见 MustKeepBypasses）
	led.recordDecision(now, ev, d, true, false, time.Time{}, deg, deg != DegradeNone)
	return d
}

// probate — 判丢：进 probation（宽限期内可翻案）并**立刻留碑**（tombstone=true + probation_until）。
func (r *Ring) probate(now time.Time, led *Ledger, ev Event, d Decision) Decision {
	kb := ev.bytesOf()
	// 淘汰序位次：被**显式否决**的必留仍按必留档对待 —— 否决是"有人表态要留它"，
	// 容量不许把这条表态藏起来（evictRank 的注释就是这条口径）。
	rank := evictRank(d.DecidingKind)
	if d.OverriddenMustKeep {
		rank = evictRank(KindMustKeep)
	}
	r.mu.Lock()
	skip := r.evictForLocked(kb, rank, now)
	r.mu.Unlock()

	if skip {
		// 装不下且淘汰不动（剩下的全是必留档），或新条目比谁都低档 ⇒ 不给它翻案机会，直接终局。
		// 这是"按优先级淘汰"的另一半：低档不许挤掉高档（容量该留给更值钱的东西）。
		r.mu.Lock()
		r.probSkipped++
		r.mu.Unlock()
		d.Tombstone = true
		d.DegradeLevel = DegradeRing
		d.Reason = d.Reason + "；容量淘汰不动（" + DegradeRing + "）：不进 probation，墓碑终局"
		led.recordDecision(now, ev, d, false, true, time.Time{}, DegradeRing, false)
		return d
	}

	until := now.Add(r.cfg.DiscardGrace)
	e := &probationEntry{runID: ev.RunID, key: ev.Key, rank: rank, bytes: kb, until: until, ev: ev}
	r.mu.Lock()
	r.probSeq++
	e.seq = r.probSeq
	r.prob[e.mapKey()] = e
	r.probList = append(r.probList, e)
	r.probBytes += kb
	r.mu.Unlock()

	d.Tombstone = true
	led.recordDecision(now, ev, d, false, true, until, d.DegradeLevel, d.GateEnforced)
	return d
}

// Overturn — **翻案**（C3 的 probation）：宽限期内晚到的判词把一条"已判丢"改为"留下"。
//
// 三条纪律：
//   - 只对**仍在自己宽限期内**的条目有效；不在（不存在 / 已到期 / 已被容量提前终结）⇒ false。
//     翻案失败必须可判定 —— 不许"看起来翻了"（账上墓碑还在，那就是没翻）。
//   - 翻案不受闸裁：翻案的依据是晚到的**必留类**判词，与 MUSTKEEP 同待遇（闸只裁非必留）。
//   - **追加**一条 amendment 行（probation → overturned），绝不回头改旧行（append-only，C5）。
func (r *Ring) Overturn(runID, key, reason string) bool {
	now := r.now()
	r.mu.Lock()
	r.expireLocked(now) // 过期的先清掉 ⇒ 下面查不到的语义就是"不在宽限期内"
	e := r.prob[probKey(runID, key)]
	if e == nil {
		r.mu.Unlock()
		return false
	}
	delete(r.prob, e.mapKey())
	for i, p := range r.probList {
		if p == e {
			r.probList = append(r.probList[:i], r.probList[i+1:]...)
			break
		}
	}
	r.probBytes -= e.bytes
	r.overturns++
	r.mu.Unlock()

	emitted, failed := r.emit(e.ev)
	deg := DegradeNone
	if failed {
		deg = DegradeSink
	}
	// 翻案成立即改判为留；但"翻案成立"与"落盘成功"是两件事 ⇒ 两个事实都记账（emitted/degrade）。
	r.ledgerFor(runID).recordAmendment(now, key, StateProbation, StateOverturned, reason, deg, emitted)
	return true
}

// Sweep — 把已过宽限期的 probation 条目清出内存，返回清出的条数。
// 账**不改**：墓碑上的 probation_until 就是"到此为止再没有翻案机会"的判据（append-only）。
func (r *Ring) Sweep() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.expireLocked(r.now())
}

// ── 闸（C2）──────────────────────────────────────────────────────────────

// gateAllow — 字节/速率闸。返回（是否放行, 降级等级）。
//
// 规则（写死）：
//   - 预算够 ⇒ 放行，degrade=none；
//   - 预算不够 + 必留 ⇒ **破例放行**（degrade=rate/bytes，MustKeepBypasses 计数）—— 必留压一切（C1）在闸这里同样成立；
//   - 预算不够 + 非必留 ⇒ 裁掉（degrade=rate/bytes）；
//   - 两个闸同时超 ⇒ 记 rate（条数口径先看，字节口径的裁法相同）。
//
// **不 sleep、不排队等预算**：这就是 C2 要的"绝不阻塞 agent"。
func (r *Ring) gateAllow(ev Event, mustKeep bool) (bool, string) {
	kb := ev.bytesOf()
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if r.win.start.IsZero() || now.Sub(r.win.start) >= time.Second {
		r.win = rateWindow{start: now}
	}
	overEvents := r.win.events+1 > r.cfg.RateEventsPerSec
	overBytes := r.win.bytes+kb > r.cfg.RateBytesPerSec
	if !overEvents && !overBytes {
		r.win.events++
		r.win.bytes += kb
		return true, DegradeNone
	}
	deg := DegradeRate
	if !overEvents {
		deg = DegradeBytes
	}
	if mustKeep {
		r.mustKeepBypass++
		return true, deg
	}
	return false, deg
}

// ── 容量与淘汰 ───────────────────────────────────────────────────────────

// evictForLocked — 为新条目腾地方：按淘汰序（最低档、同档最早）**提前终结** probation 条目。
//
// 返回 skipNew=true ⇔ 新条目**不该进 probation**：
//   - 新条目档位**低于**当前最低档（低档不许挤掉高档 ⇒ 该被牺牲的是新条目自己）；
//   - 或淘汰后仍装不下（剩下的全是必留档，必留永不淘汰 ⇒ 宁可新条目少一次翻案机会，也不超容）。
//
// 提前终结会往**被淘汰者所属 run 的账**里追加 amendment（probation → evicted, degrade_level=ring）：
// "它本来还有翻案机会，是容量把它掐掉的"这件事必须留痕，否则宽限期就是个看不见的谎言。
func (r *Ring) evictForLocked(need, newRank int, now time.Time) (skipNew bool) {
	for {
		fits := r.probBytes+need <= r.cfg.RingBytes && len(r.probList)+1 <= r.cfg.MaxInflight
		if fits {
			break
		}
		victim := r.lowestLocked()
		if victim == nil || victim.rank > newRank {
			skipNew = true
			break
		}
		r.removeLocked(victim)
		r.evictions++
		if led := r.ledgers[victim.runID]; led != nil {
			led.recordAmendment(now, victim.key, StateProbation, StateEvicted,
				"容量淘汰（ring_bytes/max_inflight）：宽限期被提前终结", DegradeRing, false)
		}
	}
	if r.probBytes+need > r.cfg.RingBytes || len(r.probList)+1 > r.cfg.MaxInflight {
		skipNew = true
	}
	return skipNew
}

// lowestLocked — 淘汰序里最低档、同档最早的条目；必留档不参与（永不淘汰）。没有候选人 ⇒ nil。
func (r *Ring) lowestLocked() *probationEntry {
	var victim *probationEntry
	for _, e := range r.probList {
		if e.rank >= evictRank(KindMustKeep) {
			continue // 必留档永不淘汰
		}
		if victim == nil || e.rank < victim.rank || (e.rank == victim.rank && e.seq < victim.seq) {
			victim = e
		}
	}
	return victim
}

func (r *Ring) removeLocked(e *probationEntry) {
	delete(r.prob, e.mapKey())
	for i, p := range r.probList {
		if p == e {
			r.probList = append(r.probList[:i], r.probList[i+1:]...)
			break
		}
	}
	r.probBytes -= e.bytes
}

// expireLocked — 清出已过宽限期的条目。边界写死：**恰等于**到期时刻仍算在宽限内（压线的判词有效）。
func (r *Ring) expireLocked(now time.Time) int {
	n := 0
	for i := 0; i < len(r.probList); {
		e := r.probList[i]
		if now.After(e.until) {
			r.removeLocked(e)
			n++
			continue
		}
		i++
	}
	return n
}

// ── 落盘出口 ─────────────────────────────────────────────────────────────

// emit — 交给 sink（best-effort）。返回（是否真的落盘成功, 是否失败）。
// Sink==nil ⇒ 只出账不落盘：返回 (false, false) —— 没有"该落未落"这回事，不算失败。
// 失败 ⇒ 计数器 +1 并**不重试、不外抛**（C2/C5：落盘失败也要在账上留下"它为什么缺席"）。
func (r *Ring) emit(ev Event) (ok bool, failed bool) {
	if r.cfg.Sink == nil {
		return false, false
	}
	if err := r.cfg.Sink(ev); err != nil {
		r.mu.Lock()
		r.sinkFails++
		r.mu.Unlock()
		return false, true
	}
	r.mu.Lock()
	r.emits++
	r.mu.Unlock()
	return true, false
}

// ── 判定账 ───────────────────────────────────────────────────────────────

// Ledger — 某个 run 的账（不存在则建；超过 MaxRuns 淘汰最久未出现的 run 的账）。
func (r *Ring) Ledger(runID string) *Ledger { return r.ledgerFor(runID) }

func (r *Ring) ledgerFor(runID string) *Ledger {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ledgerForLocked(runID)
}

func (r *Ring) ledgerForLocked(runID string) *Ledger {
	if l := r.ledgers[runID]; l != nil {
		r.touchLocked(runID)
		return l
	}
	for len(r.ledgers) >= r.cfg.MaxRuns && len(r.runOrder) > 0 {
		oldest := r.runOrder[0]
		r.runOrder = r.runOrder[1:]
		// run 的账被淘汰**不代表**它的墓碑消失了：字节已经交给调用方（JSONL），
		// 本包只为"长跑进程内存有界"负责（与 obs.go 的 obsTraceSessionsMax 同思路）。
		delete(r.ledgers, oldest)
	}
	l := newLedger(runID)
	r.ledgers[runID] = l
	r.runOrder = append(r.runOrder, runID)
	return l
}

func (r *Ring) touchLocked(runID string) {
	for i, id := range r.runOrder {
		if id == runID {
			r.runOrder = append(append(r.runOrder[:i], r.runOrder[i+1:]...), runID)
			return
		}
	}
	r.runOrder = append(r.runOrder, runID)
}

// RunIDs — 当前在账里的 run（插入序；用于消费侧枚举）。
func (r *Ring) RunIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.ledgers))
	for id := range r.ledgers {
		out = append(out, id)
	}
	return out
}

// ── 计数器（全部单调；给用例、也给运维看"采样到底干了什么"）─────────────────

// Admits — 到达数。
func (r *Ring) Admits() int { return r.count(&r.admits) }

// Emitted — 真的落盘成功的事件条数（dry-run 下 = 全量）。
func (r *Ring) Emitted() int { return r.count(&r.emits) }

// Evictions — 容量提前终结 probation 的条数（degrade_level=ring）。
func (r *Ring) Evictions() int { return r.count(&r.evictions) }

// Overturns — 翻案成功条数。
func (r *Ring) Overturns() int { return r.count(&r.overturns) }

// SinkFailures — 落盘失败条数（每个失败都对应一条墓碑）。
func (r *Ring) SinkFailures() int { return r.count(&r.sinkFails) }

// MustKeepBypasses — 闸饱和时**必留破例放行**的次数（破例可量化，不是看不见的）。
func (r *Ring) MustKeepBypasses() int { return r.count(&r.mustKeepBypass) }

// ProbationSkips — 因容量淘汰不动而**不进 probation**（墓碑终局）的次数。
func (r *Ring) ProbationSkips() int { return r.count(&r.probSkipped) }

// ProbationCount — 当前未决（等翻案）条数。
func (r *Ring) ProbationCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.probList)
}

// ProbationBytes — 当前未决占用的字节（与 Config.RingBytes 对照即知水位）。
func (r *Ring) ProbationBytes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.probBytes
}

// ProbationUntil — 某条未决条目的宽限截止时刻（不在 probation ⇒ ok=false）。
// 用途：用例/消费侧核对"它还有多久翻案窗口"。
func (r *Ring) ProbationUntil(runID, key string) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.prob[probKey(runID, key)]
	if e == nil {
		return time.Time{}, false
	}
	return e.until, true
}

// count — 读一个受锁保护的计数器（写成小工具是为了让上面 7 个 accessor 不会各自写错锁）。
func (r *Ring) count(p *int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return *p
}
