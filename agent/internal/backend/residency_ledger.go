// residency_ledger.go —— 卵**驻留账本**与「进程没了」巡检（缺口 `GAP-20261018-13`）。
//
// # 缺口（真机铁证）
//
// edge-node 上 `zerg-agentd` 活着（pid 1752149 · 已跑 2h36m · 8100 在听 · 心跳每 5 秒一条成功），
// 而同一刻 `llama-server` 与 `bwrap` **一个不剩**、94xx **0 个在听**、`zerg egg ls` 里 edge-node 全成
// 「已声明但后端不在」；agent 日志尾部**只有心跳**，`tail -400` 搜 exit/stop/kill/oom/died/
// terminat/unload 零命中 ⇒ **何时、被谁停的，事后无从判断**。
//
// 病灶不是一个 bug，是**两处结构性缺失**：
//
//	① 子端没有任何「这枚卵曾经真装上过」的**持久**记录 —— 卵表项与驻留表只活在进程内存里
//	   （`Manager.procs`）。agentd 一重启 / 进程被谁收走，证据**当场蒸发**：`/eggs` 变成空表，
//	   谁也说不清「它丢之前是什么态、丢在哪一刻」。
//	② 活性判据只看**状态机字段**（`state=ready` + `port>0`），**不看进程在不在**
//	   （`unit_liveness.go` 的件头已写过同一课：`state=ready · gtt_gb=78.2` 而机器上进程为 0）。
//
// # 本件的处置（只加证据与判词，不改既有字段语义、**不删记录**、✗ 不自动重拉）
//
//	① **驻留账本**（落盘 `egg-residency.json`）：每枚**真观测到进程在活**的卵记一条 ——
//	   `egg/model · pid · port · unit · state · loaded_at · last_seen · port_live`。
//	   ★ 落盘条件写死：**创建记录时 pid > 0 且 pid 真活着**（拿不到进程证据就不落账 ——
//	   账本里的每一条都对应过一次真实的活进程）。
//	② **存在性巡检**（`PatrolResidency`）：对账本里 `state=loaded` 的每一条核两样 ——
//	   进程（`kill(pid,0)`）与端口（`dial 127.0.0.1:port`）。进程没了 ⇒ 记一条
//	   `state=vanished · reason=process_vanished · last_seen=…`，并**追加一行结构化事件**到
//	   `egg-residency-events.jsonl`（`time · egg · model · pid · last_seen · reason`）——
//	   ★ 不只沉日志：日志会被心跳冲掉，事件件是**可机读、可事后追**的证据面。
//	③ 观测面透出：`ResidencyRecords()`（`/eggs` 的 `residency[]`）⇒ 命令面据此把「声明在但
//	   进程没了」与「已声明但后端不在（从未装上）」**分成两态**（此前混为一谈正是「看不出」的根）。
//
// 巡检**接在既有 1 秒只读巡检上**（`server/events_endpoint.go` 的 `eventLoop` → 每秒
// `EggObservations()`；主控也会定期拉 `/eggs`）⇒ 无需新起 goroutine；`m.residencyQuietUntil`
// 把实际开销封在「最短 2 秒一次」（N 枚卵 = N 次 kill + N 次 200ms 拨号）。
//
// # 边界（照实写死）
//
//	· **不自动重拉**（惊群 / 循环重启风险，Mr2109 单里明令本单不做）：本件只**留证**。
//	· 显式卸载（`evictSubprocLocked`：定向卸 / TTL 到期 / 服务类型互斥）⇒ 账本记 `released`，
//	  ✗ 不许记成 vanished（那是把「人按的停」谎报成「掉线」）。
//	· pid 拿不到（<=0）⇒ **不判** vanish（「不知道」不当「死了」；同 `unit_liveness.go` 口径）。
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 驻留账本的状态闭集（三词 · 只这三态：装上了 / 进程没了 / 显式卸了）。
const (
	// ResidencyStateLoaded 进程此刻在活（本端至少观测到过一次）。
	ResidencyStateLoaded = "loaded"
	// ResidencyStateVanished **曾经装上、现在进程没了**（缺口 `GAP-20261018-13` 的正题）。
	ResidencyStateVanished = "vanished"
	// ResidencyStateReleased 显式卸载（定向卸 / TTL / 服务类型互斥）⇒ 不是掉线。
	ResidencyStateReleased = "released"
)

// 事件原因的取值（`reason` 一格 · 闭集写死，机读侧按它分类）。
const (
	// ResidencyReasonProcessVanished 进程没了 —— 本件的头号判词。
	ResidencyReasonProcessVanished = "process_vanished"
	// ResidencyReasonPortClosed 进程还在，但它该在听的端口不听了（半死：比整条掉了更隐蔽）。
	ResidencyReasonPortClosed = "port_closed"
	// ResidencyReasonReleased 显式卸载（人按的 / 到期收的）—— 记录用，不是告警。
	ResidencyReasonReleased = "released"
)

// EnvResidencyLedger 账本件路径覆盖（测试 / 多实例隔离用）。
// 给 "off" ⇒ 本机制**整体关掉**（不读不写不巡检）：唯一逃生门。
const EnvResidencyLedger = "ZERG_EGG_RESIDENCY_LEDGER"

// EnvResidencyPatrolMinS 两次巡检之间的最短间隔（秒 · 防 `/eggs` 每次轮询都拨一遍端口）。
const EnvResidencyPatrolMinS = "ZERG_EGG_RESIDENCY_PATROL_MIN_S"

// residencySchema 账本件的 schema 主号（读端只认它；别的版本 ⇒ 当读不到，不猜）。
const residencySchema = "zerg-egg-residency/1"

// ResidencyRecord 一枚卵的驻留记录（**持久证据**：不是缓存、不因进程消失而删）。
//
// 字段口径：
//
//	EggID/Model  注册表卵名（本仓里两者同值 —— 与 `/eggs` 的 `egg_id`/`model` 同源）
//	PID          装载时观测到的引擎进程 pid（0 = 从未拿到 ⇒ 不落账）
//	Port         子端自报的引擎端口
//	Unit         孵化单元名（裸 exec 路径为空）
//	State        loaded / vanished / released（见上面三常量）
//	LoadedAt     **首次观测到 ready 的时刻**（观测值，不是内核事件 —— 照实标注）
//	LastSeen     **最后一次观测到「进程在」的时刻**（vanish 时它就是「最后见到活着的时刻」）
//	DetectedAt   判出 vanished / released 的时刻
//	PortLive     最近一次巡检拨号的结果（loaded 才更新）
//	Reason       最近一次事件的 reason（闭集：process_vanished / port_closed / released）
type ResidencyRecord struct {
	EggID      string `json:"egg_id"`
	Model      string `json:"model"`
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	Unit       string `json:"unit,omitempty"`
	State      string `json:"state"`
	LoadedAt   string `json:"loaded_at,omitempty"`
	LastSeen   string `json:"last_seen,omitempty"`
	DetectedAt string `json:"detected_at,omitempty"`
	PortLive   bool   `json:"port_live"`
	Reason     string `json:"reason,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// residencyEvent 事件件的一行（JSONL · **追加写**，一事一行，永不改写）。
//
// 形状 = 缺口单里点名的五格（`time · egg/model · pid · last_seen · reason`）＋ 复用的 port/unit/detail。
// 为什么不是日志行：日志按时间滚、会被心跳冲掉（真机 tail -400 零命中的正是它）；事件件是
// **可机读、可聚合、可事后定位「丢在哪一刻」**的证据面。
type residencyEvent struct {
	Time     string `json:"time"`
	Egg      string `json:"egg"`
	Model    string `json:"model"`
	PID      int    `json:"pid"`
	Port     int    `json:"port"`
	Unit     string `json:"unit,omitempty"`
	State    string `json:"state"`
	LastSeen string `json:"last_seen,omitempty"`
	Reason   string `json:"reason"`
	Detail   string `json:"detail,omitempty"`
}

// residencyLedgerDoc 落盘形状（件头带 schema 与生成时刻 —— 读端只认 schema）。
type residencyLedgerDoc struct {
	Schema    string            `json:"schema"`
	UpdatedAt string            `json:"updated_at"`
	Records   []ResidencyRecord `json:"records"`
}

// residencyDisabled 关闸判定（`ZERG_EGG_RESIDENCY_LEDGER=off`）。
func residencyDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvResidencyLedger)), "off")
}

// residencyStateDir 本端状态目录（与 CLI 同源：`ZERG_STATE_DIR` ⇒ 缺省 `~/.zerg/state`）。
func residencyStateDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".zerg", "state")
}

// residencyLedgerPath 账本件路径（覆盖 ⇒ 用覆盖值；否则落在状态目录下；状态目录都给不出 ⇒ 空 = 不落账）。
func residencyLedgerPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvResidencyLedger)); p != "" && !strings.EqualFold(p, "off") {
		return p
	}
	if d := residencyStateDir(); d != "" {
		return filepath.Join(d, "egg-residency.json")
	}
	return ""
}

// residencyEventsPath 事件件路径（与账本同目录；账本不可给 ⇒ 空）。
func residencyEventsPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvResidencyLedger)); p != "" && !strings.EqualFold(p, "off") {
		return p + ".events.jsonl"
	}
	if d := residencyStateDir(); d != "" {
		return filepath.Join(d, "egg-residency-events.jsonl")
	}
	return ""
}

// residencyPatrolMin 两次巡检的最短间隔（缺省 2s；非法值按缺省 —— 配错宁可慢，不许把巡检关掉）。
func residencyPatrolMin() time.Duration {
	if v := strings.TrimSpace(os.Getenv(EnvResidencyPatrolMinS)); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil && f > 0 {
			return time.Duration(f * float64(time.Second))
		}
	}
	return 2 * time.Second
}

// rfc3339 统一时间格式（UTC · 秒级；账本是给人看与事后对的，不追纳秒）。
func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// ResidencyRecords 账本当前的**只读快照**（按卵名排序 · 给 `/eggs` 的 `residency[]` 用）。
//
// ★ 本方法**不触发巡检**（读面要快、要可重复）；巡检由 `PatrolResidency` 在观测面入口驱动。
func (m *Manager) ResidencyRecords() []ResidencyRecord {
	if m == nil || residencyDisabled() {
		return nil
	}
	m.resMu.Lock()
	defer m.resMu.Unlock()
	return m.residencySnapshotLocked()
}

// residencySnapshotLocked 账本的定序副本（调用方持 `resMu`）。
func (m *Manager) residencySnapshotLocked() []ResidencyRecord {
	if len(m.residency) == 0 {
		return nil
	}
	out := make([]ResidencyRecord, 0, len(m.residency))
	for _, r := range m.residency {
		if r == nil {
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EggID < out[j].EggID })
	return out
}

// residencyLive 巡检用的「本端当前托管项」快照（摘 `m.procs` 的四格，**不持锁出函数**）。
type residencyLive struct {
	pid   int
	port  int
	state string
	unit  string
}

// ∎ 巡检（本件的判据面）

// PatrolResidency 对账本里**每一枚曾装上过的卵**核「进程还在不在」，并把结论落账 / 落事件。
//
// 判据（写死，保守方向）：
//
//	① 进程 pid 拿不到（<=0） ⇒ **不判**（「不知道」不当「死了」，同 `unit_liveness.go`）
//	② pid 不在活       ⇒ `state=vanished · reason=process_vanished` ＋ 追加一行事件
//	③ pid 在活而端口不听 ⇒ 记 `port_live=false` ＋ **一次**（转变时）`reason=port_closed` 事件
//	④ 显式卸载过的（released）⇒ 永不改判（人按的停不是掉线）
//
// 返回巡检后的账本快照（定序）。★ 本函数**不启停任何进程**、**不重拉**、不删记录。
func (m *Manager) PatrolResidency() []ResidencyRecord {
	if m == nil {
		return nil
	}
	now := time.Now()
	// ★ 缺口 GAP-20261019-01（TTL 续期）：默认模型的 pin 在这里续 —— 复用本函数既有的
	//   **2 秒级巡检**（`residencyPatrolMin` 的节流在下面），**不新起常驻 goroutine**。
	//   位置在 ledgerDisabled 判定之前：账本关掉不等于保护停掉（保护另有总闸 ZERG_DEFAULT_PIN）。
	m.renewDefaultPin(now)
	if residencyDisabled() {
		return nil
	}
	// ① 先在 `m.mu` 下摘「本端托管项」快照，**出锁后**再动账本：
	//    锁序永远是 `m.mu → resMu`（`markResidencyReleased` 也从 `m.mu` 里进来，同序 ⇒ 不死锁）。
	lives := m.residencyLiveSnapshot()

	m.resMu.Lock()
	defer m.resMu.Unlock()
	if m.residencyQuietUntil.After(now) {
		return m.residencySnapshotLocked()
	}
	m.residencyQuietUntil = now.Add(residencyPatrolMin())
	m.residencyLoadLocked()

	rt := rfc3339(now)
	dirty := false

	// ② 在托管项 ⇒ 记账（首次见 ready 才建条目；之后只刷 pid/port/unit 与 last_seen）。
	for name, lv := range lives {
		if lv.state != StateReady && lv.state != StateIdleArmed && lv.state != StateLoading {
			continue
		}
		rec, ok := m.residency[name]
		if !ok || rec == nil || rec.State != ResidencyStateLoaded {
			// ★ 落账条件：**必须有活进程证据**（pid>0 且真活着）。
			//   拿不到 ⇒ 不建条目（账本里每一条都对应过一次真实的活进程 —— 这也是
			//   单测里合成 subproc（pid=0）不会在开发机状态目录里留垃圾的原因）。
			if lv.pid <= 0 || !pidAlive(lv.pid) {
				continue
			}
			rec = &ResidencyRecord{
				EggID:    name,
				Model:    name,
				PID:      lv.pid,
				Port:     lv.port,
				Unit:     lv.unit,
				State:    ResidencyStateLoaded,
				LoadedAt: rt,
				LastSeen: rt,
				PortLive: portAlive(lv.port),
			}
			m.residency[name] = rec
			dirty = true
			log.Printf("[backend] 驻留账本：记一枚卵 egg=%s pid=%d port=%d unit=%s（首次观测到 %s）",
				name, lv.pid, lv.port, orDash(lv.unit), orDash(lv.state))
			continue
		}
		// 已有一条 loaded 记录：刷新实测三格（pid/port/unit 可能随重连变）。
		if rec.PID != lv.pid || rec.Port != lv.port || rec.Unit != lv.unit {
			rec.PID, rec.Port, rec.Unit = lv.pid, lv.port, lv.unit
			dirty = true
		}
		if lv.pid > 0 && pidAlive(lv.pid) {
			if rec.LastSeen != rt {
				rec.LastSeen = rt
				dirty = true
			}
			if live := portAlive(lv.port); live != rec.PortLive {
				rec.PortLive = live
				dirty = true
				if !live {
					m.residencyEmitLocked(*rec, ResidencyReasonPortClosed,
						fmt.Sprintf("进程在（pid=%d）但端口 %d 不在听", lv.pid, lv.port))
				}
			}
		}
	}

	// ③ 账本里有、托管项里没有 ⇒ 核进程；没了就是本缺口的正题。
	for name, rec := range m.residency {
		if rec == nil || rec.State != ResidencyStateLoaded {
			continue
		}
		if _, ok := lives[name]; ok {
			continue
		}
		if rec.PID <= 0 {
			rec.Detail = "pid 未知（装载时没拿到进程证据）⇒ 不判进程死活"
			dirty = true
			continue
		}
		if pidAlive(rec.PID) {
			// 进程还在活但本端已不在管它（句柄丢了）—— 不是「进程没了」，**不谎报**。
			rec.LastSeen = rt
			rec.Detail = "进程在活，但本端托管表里已没有这一枚（句柄丢失）"
			dirty = true
			continue
		}
		rec.State = ResidencyStateVanished
		rec.Reason = ResidencyReasonProcessVanished
		rec.DetectedAt = rt
		rec.PortLive = false
		rec.Detail = fmt.Sprintf("进程 pid=%d 已不在活；最后见到活着的时刻=%s", rec.PID, orDash(rec.LastSeen))
		dirty = true
		m.residencyEmitLocked(*rec, ResidencyReasonProcessVanished, rec.Detail)
		log.Printf("[backend] ⚠ 卵进程静默消失: egg=%s pid=%d port=%d last_seen=%s reason=%s（**只留证、不重拉**）",
			rec.EggID, rec.PID, rec.Port, orDash(rec.LastSeen), ResidencyReasonProcessVanished)
	}

	if dirty {
		m.residencySaveLocked()
	}
	return m.residencySnapshotLocked()
}

// residencyLiveSnapshot 摘 `m.procs` 的 (pid, port, state, unit) 四格（**不持 m.mu 出函数**）。
func (m *Manager) residencyLiveSnapshot() map[string]residencyLive {
	out := map[string]residencyLive{}
	m.mu.Lock()
	for name, sp := range m.procs {
		if sp == nil || name == "" {
			continue
		}
		out[name] = residencyLive{pid: enginePIDOf(sp), port: sp.port, state: sp.state, unit: sp.Unit}
	}
	m.mu.Unlock()
	return out
}

// markResidencyReleased 显式卸载 ⇒ 账本记 `released`（✗ 不记 vanished —— 人按的停不是掉线）。
//
// 调用点 = `evictSubprocLocked`（定向卸 / TTL 到期 / 服务类型互斥的**唯一**收口）。
// 锁序：调用方持 `m.mu` ⇒ 本函数再拿 `resMu`（与 `PatrolResidency` 的 `m.mu → resMu` 同序）。
func (m *Manager) markResidencyReleased(name, why string) {
	if m == nil || residencyDisabled() {
		return
	}
	m.resMu.Lock()
	defer m.resMu.Unlock()
	m.residencyLoadLocked()
	rec, ok := m.residency[name]
	if !ok || rec == nil || rec.State != ResidencyStateLoaded {
		return
	}
	now := time.Now()
	rec.State = ResidencyStateReleased
	rec.Reason = ResidencyReasonReleased
	rec.DetectedAt = rfc3339(now)
	rec.Detail = strings.TrimSpace(why)
	m.residencyEmitLocked(*rec, ResidencyReasonReleased, why)
	m.residencySaveLocked()
}

// ∎ 落盘 / 读盘（失败一律「降级但不静默改判」：读不到当空账，写不进只记一次日志）

// residencyLoadLocked 读账本（调用方持 `resMu`）。
//
// 读不到 / 坏件 ⇒ 当空账并**记一次**日志（`residencyLoadWarned`）：⚠ 这里**不许**因为读不到就
// 把「曾经装上过」的证据丢掉 —— 坏件原地留给人看（本函数不删、不覆盖，直到下一次有变更才写）。
func (m *Manager) residencyLoadLocked() {
	if m.residencyLoaded {
		return
	}
	m.residencyLoaded = true
	if m.residency == nil {
		m.residency = map[string]*ResidencyRecord{}
	}
	p := residencyLedgerPath()
	if p == "" {
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) && !m.residencyLoadWarned {
			m.residencyLoadWarned = true
			log.Printf("[backend] 驻留账本读不到（%s）：%v ⇒ 本轮当空账（不猜、不覆盖）", p, err)
		}
		return
	}
	var doc residencyLedgerDoc
	if err := json.Unmarshal(b, &doc); err != nil || doc.Schema != residencySchema {
		if !m.residencyLoadWarned {
			m.residencyLoadWarned = true
			log.Printf("[backend] 驻留账本不可解析或 schema 不符（%s · schema=%q 期望 %q）⇒ 本轮当空账（✗ 不覆盖原件的判断留给下一次写前）",
				p, doc.Schema, residencySchema)
		}
		return
	}
	for i := range doc.Records {
		r := doc.Records[i]
		if strings.TrimSpace(r.EggID) == "" {
			continue
		}
		cp := r
		m.residency[cp.EggID] = &cp
	}
}

// residencySaveLocked 原子落盘（tmp + rename；调用方持 `resMu`）。
func (m *Manager) residencySaveLocked() {
	p := residencyLedgerPath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		m.residencySaveWarn(err)
		return
	}
	doc := residencyLedgerDoc{
		Schema:    residencySchema,
		UpdatedAt: rfc3339(time.Now()),
		Records:   m.residencySnapshotLocked(),
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		m.residencySaveWarn(err)
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		m.residencySaveWarn(err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		m.residencySaveWarn(err)
	}
}

// residencySaveWarn 写失败只记一次（每轮都刷日志会把真信号冲掉）。
func (m *Manager) residencySaveWarn(err error) {
	if m.residencySaveWarned {
		return
	}
	m.residencySaveWarned = true
	log.Printf("[backend] ⚠ 驻留账本写不进（%s）：%v ⇒ 本轮证据只在内存里（进程一退就丢）", residencyLedgerPath(), err)
}

// residencyEmitLocked 追加一行结构化事件（调用方持 `resMu`）。
//
// ★ 这是本缺口要求的那一格：**不能只沉在日志里**。日志会滚、会被心跳冲掉（真机 `tail -400`
// 零命中的正是它）；事件件是 JSONL 追加面 —— 可 grep、可聚合、可事后定位「丢在哪一刻」。
func (m *Manager) residencyEmitLocked(rec ResidencyRecord, reason, detail string) {
	ev := residencyEvent{
		Time:     rfc3339(time.Now()),
		Egg:      rec.EggID,
		Model:    rec.Model,
		PID:      rec.PID,
		Port:     rec.Port,
		Unit:     rec.Unit,
		State:    rec.State,
		LastSeen: rec.LastSeen,
		Reason:   reason,
		Detail:   detail,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	p := residencyEventsPath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		m.residencySaveWarn(err)
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		m.residencySaveWarn(err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		m.residencySaveWarn(err)
	}
}

// ResidencyEventLine 事件行的渲染（供人面/测试做形状断言用 —— 与落盘那一行**同形**）。
func ResidencyEventLine(rec ResidencyRecord, reason, detail string, at time.Time) string {
	b, err := json.Marshal(residencyEvent{
		Time: rfc3339(at), Egg: rec.EggID, Model: rec.Model, PID: rec.PID, Port: rec.Port,
		Unit: rec.Unit, State: rec.State, LastSeen: rec.LastSeen, Reason: reason, Detail: detail,
	})
	if err != nil {
		return ""
	}
	return string(b)
}
