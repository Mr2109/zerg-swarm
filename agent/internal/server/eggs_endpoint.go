// eggs_endpoint.go —— P7 批 1：GET /eggs 只读端点 + /services 真数据接线。
//
// 设计真源：设计-子端沙箱化-20260914.md
//
//	§5.4 观测面（端点名定案：只读端点名为 /eggs，字段一律 eggs[] / egg_id；
//	/services 亦可、载荷同上）· §4.3 第八项（schema_version 如实标出）· §8.7
//	（external_occupancy[] 只含非引擎 GPU 使用者）。
//
// 本文件只读：不含任何写动作（红线②）。缺的字段缺席（零值/known=false），不编造。
package server

import (
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
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
	EggID         string   `json:"egg_id"`                // 卵名（注册表键，真 egg_id）
	Unit          string   `json:"unit,omitempty"`        // 孵化器单元归属（P1 单元化落地前缺席）
	EngineImpl    string   `json:"engine_impl,omitempty"` // 实际会执行的引擎实现名
	Model         string   `json:"model,omitempty"`
	State         string   `json:"state,omitempty"`
	Port          int      `json:"port,omitempty"`
	Inflight      int      `json:"inflight"`                 // P2 在飞引用计数（唯一真源）
	IdleArmedS    *float64 `json:"idle_armed_remaining_s"`   // null = 不在空窗计时中
	SchemaVersion int      `json:"schema_version,omitempty"` // 0 = 未声明（如实缺席）
	HasProfile    bool     `json:"has_profile"`              // 实测档案是否可用（§8.4）
	GttGb         float64  `json:"gtt_gb,omitempty"`         // 逐进程 GTT 归因；无归因缺席
	Managed       bool     `json:"managed"`                  // 恒 true：eggs[] 只装本端托管项
	// EnclosureVerified 封闭性是否**实读核验通过**（§6.9：静默失效不得当凭据）。
	// false 与 EnclosureNote 合起来读：「未核验/读不到」还是「不符」；note 空 = 从未声称过隔离。
	EnclosureVerified bool   `json:"enclosure_verified"`
	EnclosureNote     string `json:"enclosure_note,omitempty"`
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
			EggID:             o.EggID,
			EngineImpl:        o.EngineImpl,
			Model:             o.Model,
			State:             o.State,
			Port:              o.Port,
			Inflight:          o.Inflight,
			IdleArmedS:        o.IdleArmedRemainS,
			SchemaVersion:     o.SchemaVersion,
			HasProfile:        profileOf != nil && profileOf(o.EggID),
			Managed:           true,
			EnclosureVerified: o.EnclosureVerified,
			EnclosureNote:     o.EnclosureNote,
		}
		if a, ok := attrib[o.PID]; ok {
			e.GttGb = round1f(a.GttGb)
		}
		out = append(out, e)
	}
	return out
}

// eggProfileExists 判一枚卵有没有可用实测档案（§8.4：档案=每卵一份实测事实卡）。
// 档案目录不可用/该卵无档案 → false（如实，绝不编造 has_profile=true）。
func eggProfileExists(eggID string) bool {
	if eggID == "" {
		return false
	}
	_, err := monitor.LoadEggProfile(monitor.EggProfilePath(eggID))
	return err == nil
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
	eggs := eggEntries(obs, s.latestAttrib(), eggProfileExists)
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
	eggs := eggEntries(obs, s.latestAttrib(), eggProfileExists)

	var gtt monitor.GttSample
	if s.agent.vitals != nil {
		if v := s.agent.vitals.Snapshot(); v.Gtt.Ok {
			gtt = v.Gtt
		}
	}

	writeJSON(w, http.StatusOK, servicesSnapshot(buildSlot(mgr), eggs, s.externalOccupants(), gtt, time.Now()))
}
