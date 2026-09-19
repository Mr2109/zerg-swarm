// eggs_endpoint.go —— P7 批 1：GET /eggs 只读端点 + /services 真数据接线。
//
// 设计真源：设计-子端沙箱化.md
//
//	§5.4 观测面（端点名定案：只读端点名为 /eggs，字段一律 eggs[] / egg_id；
//	/services 亦可、载荷同上）· §4.3 第八项（schema_version 如实标出）· §8.7
//	（external_occupancy[] 只含非引擎 GPU 使用者）。
//
// 本文件只读：不含任何写动作（红线②）。缺的字段缺席（零值/known=false），不编造。
package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/enclosure"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// servicesSlot /services 载荷的槽位块（单槽现实下至多一枚在位卵）。
type servicesSlot struct {
	Occupied bool   `json:"occupied"`          // 槽位上是否有在位卵
	Model    string `json:"model,omitempty"`   // 当前模型名（有才出现）
	State    string `json:"state,omitempty"`   // 该卵当前状态（ready/loading/…）
	Port     int    `json:"port,omitempty"`    // 引擎监听端口（有才出现）
	Backend  string `json:"backend,omitempty"` // 后端类型（有才出现）
}

// servicesEgg /services 与 /eggs 载荷的卵条目（字段对齐设计 §5.4 定案：
// eggs[] / egg_id / engine_impl / schema_version；缺的缺席，不编造）。
type servicesEgg struct {
	EggID      string `json:"egg_id"`                // 卵名（注册表键，真 egg_id）
	Unit       string `json:"unit,omitempty"`        // 孵化器单元归属（孵化路径才有；裸 exec 如实缺席）
	EngineImpl string `json:"engine_impl,omitempty"` // 实际会执行的引擎实现名
	Model      string `json:"model,omitempty"`
	State      string `json:"state,omitempty"`
	// UnitState 单元活性核验结论（2026-09-19 ②）：active/activating/reloading/deactivating/
	// inactive/failed/missing。**只有孵化路径**（有 unit）才有这一格；裸 exec 路径如实缺席
	// （没有单元可核，不编造一个 "active"）。
	//
	// 为什么要它单独一格：`state` 是 backend 状态机自报的（卵「以为」自己 ready），
	// 而 unit_state 是**现场实读**的单元态 —— 真机上出现过 state=ready 而机器上进程为零
	// （主控据此派活儿 ⇒ 客户端 503/0 token）。两者放在同一张卡片上，读的人一眼看得出矛盾。
	UnitState string `json:"unit_state,omitempty"`
	Port      int    `json:"port,omitempty"`
	Inflight  int    `json:"inflight"` // P2 在飞引用计数（唯一真源）
	// Watchdog 活性看门狗判词/理由/窗口/工时增量（设计-活性看门狗 §6）；缺席 = 本卵没被看过。
	Watchdog      *backend.WatchdogObservation `json:"watchdog,omitempty"`
	IdleArmedS    *float64                     `json:"idle_armed_remaining_s"`   // null = 不在空窗计时中
	SchemaVersion int                          `json:"schema_version,omitempty"` // 0 = 未声明（如实缺席）
	HasProfile    bool                         `json:"has_profile"`              // 实测档案是否可用（§8.4）
	GttGb         float64                      `json:"gtt_gb,omitempty"`         // 逐进程 GTT 归因；无归因缺席
	Managed       bool                         `json:"managed"`                  // 恒 true：eggs[] 只装本端托管项
	// EnclosureVerified 封闭性是否**实读核验通过**（§6.9：静默失效不得当凭据）。
	// false 与 EnclosureNote 合起来读：「未核验/读不到」还是「不符」；note 空 = 从未声称过隔离。
	//
	// Deprecated（茧壁批 1）：旧口径的布尔摘要 —— 它把「期望什么等级」与「实测什么等级」压成了一格。
	// 保留只为不一次性打断既有消费侧；新消费侧一律读下方 Enclosure（expected/observed 分开）。
	EnclosureVerified bool   `json:"enclosure_verified"`
	EnclosureNote     string `json:"enclosure_note,omitempty"`
	// Enclosure 茧壁的等级声明（§4.2 四级 / §六 判据 8）：expected 与 observed **两个字段都出现**、
	// 可不等（未申报期望时 expected 为空串）；形状与卵档案同一份（`enclosure.Verdict`）。
	//
	// nil = 本端从未声称过隔离（裸 exec 路径，孵化开关关）—— 与「声称了但读不到」不是一回事。
	Enclosure *enclosure.Verdict `json:"enclosure,omitempty"`
}

// externalOccupant external_occupancy[] 条目：非引擎 GPU 使用者（§8.7 收窄口径）。
type externalOccupant struct {
	PID   int     `json:"pid"`
	Argv0 string  `json:"argv0,omitempty"`
	GttGb float64 `json:"gtt_gb"`
}

// eggEntries 把 backend 只读快照装配成载荷条目。
// gtt 归因按 pid 匹配（本轮归因采样里该 pid 的 drm-memory-gtt 读数）；
// has_profile 按实测档案目录是否存在该卵的档案（§8.4——取不到就 false，不编造）。
func eggEntries(obs []backend.EggObservation, attrib map[int]monitor.ProcAttrib, profileOf func(eggID string) bool) []servicesEgg {
	out := make([]servicesEgg, 0, len(obs))
	for _, o := range obs {
		e := servicesEgg{
			EggID: o.EggID,
			// Unit 必须在这里透出去（缺陷 14 真机补漏）：backend 侧的 o.Unit 已填，
			// 但装配器原先没赋这一格 ⇒ /eggs 里 unit 恒缺席。孵化路径才有单元名；
			// 裸 exec 路径如实空（那是本端进程，不属任何单元）。
			Unit:              o.Unit,
			EngineImpl:        o.EngineImpl,
			Model:             o.Model,
			State:             o.State,
			Port:              o.Port,
			Inflight:          o.Inflight,
			Watchdog:          o.Watchdog,
			IdleArmedS:        o.IdleArmedRemainS,
			SchemaVersion:     o.SchemaVersion,
			HasProfile:        profileOf != nil && profileOf(o.EggID),
			Managed:           true,
			EnclosureVerified: o.EnclosureVerified,
			EnclosureNote:     o.EnclosureNote,
			Enclosure:         o.Enclosure,
		}
		if a, ok := attrib[o.PID]; ok {
			e.GttGb = round1f(a.GttGb)
		}
		out = append(out, e)
	}
	return out
}

// ── 单元活性核验（2026-09-19 ②）──────────────────────────────────────────────
//
// 真机现象：`GET /eggs` 报 `state=ready · gtt_gb=78.2`，同一时刻机器上 `GTT 用 0.0 GiB ·
// ds4 进程 0`（`watchdog.verdict=ok`）⇒ 主控按「有活儿能派」路由过去 ⇒ 客户端 503/0 token。
// 病灶：卵表项直接来自 backend 自报的状态机字段，**没有单位活体核验**。
// 处置（保守）：输出前核 unit 活性 ⇒ 单位不在活动就把 `state` 落 dead/missing、补 `unit_state`
// 字段与看门狗 `dead_unit` 判词。**不删记录**（记录是证据；清理归卸载/GC 路径）。

// applyUnitLiveness 卵表输出前的活性核验（纯逻辑，便于单测：liveFor 注入假核验）。
//
//	obs 与 eggs 必须**同序同长**（eggEntries 保证：逐条 append）；不等长 ⇒ 一条都不动（不猜对齐）。
//	返回被判死的条数。note 可 nil。
//
// 语义要点：
//   - 有 unit 才核（裸 exec 路径没有单元 ⇒ unit_state 缺席，不编造）；
//   - 核不到（missing）也**不报 ready**（那是「不知道」，绝不当「活着」）；
//   - 记录一律保留：本函数只改 State/UnitState/Watchdog 三格，绝不删条目。
func applyUnitLiveness(obs []backend.EggObservation, eggs []servicesEgg, liveFor func(string) backend.UnitLiveness,
	note func(eggID, unitState, state, reason string)) int {
	if liveFor == nil || len(obs) != len(eggs) {
		return 0
	}
	dead := 0
	for i, o := range obs {
		unit := strings.TrimSpace(o.Unit)
		if unit == "" {
			continue // 裸 exec：没有单元可核（如实缺席）
		}
		lv := liveFor(unit)
		eggs[i].UnitState = lv.State
		if lv.Alive {
			continue
		}
		dead++
		if lv.State == "missing" {
			eggs[i].State = "missing"
		} else {
			eggs[i].State = "dead"
		}
		eggs[i].Watchdog = deadUnitObservation(eggs[i].Watchdog, lv)
		if note != nil {
			note(o.EggID, lv.State, eggs[i].State, lv.Detail)
		}
	}
	return dead
}

// deadUnitObservation 把「单元已不在活动」落到观测面的看门狗格。
//
// 保留原有计数（窗口/次数/工时/上次输出间隔），只换判词与理由 —— 原来那条判词不丢（附在后面），
// 因为「它曾经为什么卡过」也是复盘要的证据（§6.9：现场证据不可被一句结论替换掉）。
//
// ⚠ 必须**新建**对象：prev 指向 backend 账本里的共享观测（改它会污染下一轮采样看到的事实）。
func deadUnitObservation(prev *backend.WatchdogObservation, lv backend.UnitLiveness) *backend.WatchdogObservation {
	obs := backend.WatchdogObservation{}
	if prev != nil {
		obs = *prev
	}
	reason := fmt.Sprintf("单元已不在活动 ⇒ 判 dead_unit：%s；本卵记录**保留**（不静默删记录——清理归卸载/GC 路径）", lv.Detail)
	if prev != nil && prev.Verdict != "" && prev.Verdict != backend.WatchdogDeadUnit {
		reason = fmt.Sprintf("%s（覆盖原判词 %s：%s）", reason, prev.Verdict, prev.Reason)
	}
	obs.Verdict = backend.WatchdogDeadUnit
	obs.Reason = reason
	return &obs
}

// applyEggUnitLiveness 生产接线：核验器 = backend.Manager（只读探针），留痕 = 变更去重日志。
func (s *Server) applyEggUnitLiveness(obs []backend.EggObservation, eggs []servicesEgg) int {
	if s == nil || s.agent == nil || s.agent.backends == nil {
		return 0
	}
	return applyUnitLiveness(obs, eggs, s.agent.backends.UnitLiveness, s.noteUnitStateChange)
}

// noteUnitStateChange 单元活性的留痕（**只在变化时**打日志）。
//
// 为什么要去重：/eggs 会被主控心跳定期轮询，死卵如果每次轮询都打一行，日志会被冲掉
// （本项目对「静默失效」的硬要求是「看得见」，不是「刷屏」）。同一 (egg_id, unit_state) 只记一次；
// 状态变回去再变死会重新记一条（那是新事实）。
func (s *Server) noteUnitStateChange(eggID, unitState, state, reason string) {
	cur := state + "/" + unitState
	s.unitStateLogMu.Lock()
	if s.unitStateLog == nil {
		s.unitStateLog = map[string]string{}
	}
	prev, seen := s.unitStateLog[eggID]
	s.unitStateLog[eggID] = cur
	s.unitStateLogMu.Unlock()
	if seen && prev == cur {
		return
	}
	log.Printf("[server] ⚠ 卵单元已不在活动: egg=%s state=%s unit_state=%s —— %s", eggID, state, unitState, reason)
}

// eggProfileExists 判一枚卵有没有可用实测档案（§8.4：档案=每卵一份实测事实卡）。
// 档案目录不可用/该卵无档案 → false（如实，绝不编造 has_profile=true）。
//
// ①（2026-09-19）：生产路径改走 `Manager.EggProfileAvailable`（读档案缓存；缓存没有才现读一次），
// 免得主控每次心跳轮询都逐卵 os.ReadFile。本函数保留为**无 Manager 时的回落**（与既有用例的注入点）。
func eggProfileExists(eggID string) bool {
	if eggID == "" {
		return false
	}
	_, err := monitor.LoadEggProfile(monitor.EggProfilePath(eggID))
	return err == nil
}

// eggProfileOf 观测面的 has_profile 判据（生产：backend 的档案缓存读，见 ① 的 egg_profile_cache.go）。
func (s *Server) eggProfileOf() func(string) bool {
	if s != nil && s.agent != nil && s.agent.backends != nil {
		return s.agent.backends.EggProfileAvailable
	}
	return eggProfileExists
}

// latestAttrib 最新一轮逐进程归因（pid → 读数）。该平台读不到归因 ⇒ 空表（不编造）。
func (s *Server) latestAttrib() map[int]monitor.ProcAttrib {
	out := map[int]monitor.ProcAttrib{}
	if s.agent.vitals == nil {
		return out
	}
	samples := s.agent.vitals.RecentAttrib(1)
	if len(samples) == 0 {
		return out
	}
	for _, p := range samples[len(samples)-1].Procs {
		out[p.PID] = p
	}
	return out
}

// buildSlot 单槽块：取驻留面里最新的就绪/加载项。
func buildSlot(mgr *backend.Manager) *servicesSlot {
	if name := mgr.CurrentModel(); name != "" {
		return &servicesSlot{
			Occupied: true,
			Model:    name,
			State:    mgr.State(),
			Port:     mgr.CurrentPort(),
			Backend:  mgr.CurrentBackend(),
		}
	}
	if mgr.State() != backend.StateIdle {
		return &servicesSlot{Occupied: true, State: mgr.State()}
	}
	return nil
}

// servicesSnapshot 组装只读快照（纯函数，便于测试）。
func servicesSnapshot(slot *servicesSlot, eggs []servicesEgg, external []externalOccupant, gtt monitor.GttSample, now time.Time) map[string]interface{} {
	if external == nil {
		external = []externalOccupant{}
	}
	return map[string]interface{}{
		"slot":               slot,
		"eggs":               eggs,
		"external_occupancy": external,
		"gtt": map[string]interface{}{
			"known":    gtt.Ok,
			"used_gb":  round1f(gtt.UsedGb()),
			"total_gb": round1f(gtt.TotalGb()),
		},
		"generated_at": now.UTC().Format(time.RFC3339),
	}
}

// round1f 四舍五入到 1 位小数（与心跳上报的口径一致）。
func round1f(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

// externalOccupants 非引擎 GPU 使用者清单（§8.7）：逐进程 GTT 归因里，
// 排除本端托管卵自己的 pid（PIDSet 只读快照），剩下的如实列出；读不到 ⇒ 空数组。
func (s *Server) externalOccupants() []externalOccupant {
	managed := s.agent.backends.PIDSet()
	attrib := s.latestAttrib()
	out := make([]externalOccupant, 0)
	for _, p := range attrib {
		if _, isManaged := managed[p.PID]; isManaged {
			continue
		}
		out = append(out, externalOccupant{PID: p.PID, Argv0: p.Argv0, GttGb: round1f(p.GttGb)})
	}
	return out
}

// handleEggs GET /eggs —— 只读：卵清单视角（设计 §5.4 端点名定案 (a)）。
func (s *Server) handleEggs(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed（只读端点，仅支持 GET）",
		})
		return
	}
	obs := s.agent.backends.EggObservations()
	eggs := eggEntries(obs, s.latestAttrib(), s.eggProfileOf())
	// ②（2026-09-19）：输出前核 unit 活性 —— 单位不在活动 ⇒ state 落 dead/missing +
	// unit_state 字段 + 看门狗 dead_unit 判词（记录保留，绝不静默删）。
	s.applyEggUnitLiveness(obs, eggs)
	if eggs == nil {
		eggs = []servicesEgg{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"eggs":         eggs,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleServices GET /services —— 只读：槽位与卵的现状（载荷同 §5.4，真数据接线完成）。
func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed（只读端点，仅支持 GET）",
		})
		return
	}
	mgr := s.agent.backends

	obs := mgr.EggObservations()
	eggs := eggEntries(obs, s.latestAttrib(), s.eggProfileOf())
	// ② 同 /eggs：/services 也是观测面，同样不许把「单元已死」的卵报成 ready。
	s.applyEggUnitLiveness(obs, eggs)

	var gtt monitor.GttSample
	if s.agent.vitals != nil {
		if v := s.agent.vitals.Snapshot(); v.Gtt.Ok {
			gtt = v.Gtt
		}
	}

	writeJSON(w, http.StatusOK, servicesSnapshot(buildSlot(mgr), eggs, s.externalOccupants(), gtt, time.Now()))
}
