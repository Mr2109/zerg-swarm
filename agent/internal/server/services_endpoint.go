// services_endpoint.go —— P4：只读可观测面（重定义载荷）。
//
// 设计依据：设计-子端沙箱化-20260914.md §10.1 services_endpoint.go 行、§5.4 观测面——
// 快照字段由旧的 {declared_ports, reclaim, baseline, leases} 重定义为
// **{slot, eggs[], external_occupancy[], gtt}**：
//   - slot：当前槽位状态（单槽现实下的卵位）；
//   - eggs[]：本端托管的卵（虫卵化后的上报对象——P1 孵化器接线后由卵注册表填）；
//   - external_occupancy[]：**只含非引擎 GPU 使用者**（§8.7 本轮收窄：
//     不再有"外部推理服务"这一类；数据来自逐进程 GTT 归因，接线点见 TODO）；
//   - gtt：全局 GTT 账（主口径，monitor VitalsRecorder 的 GttSample）。
//
// 本文件只读：GET /services 返回快照，**不含任何写动作**（红线②）。
// 旧字段（declared_ports / reclaim / baseline / leases）已随 P4 退场清理删除
// （附录 C·C8：env 真源与 ReclaimMode 等导出符号一并摘除）。
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

// servicesEgg /services 载荷的卵条目。
//
// ⚠ TODO(P1 接线点)：P1 孵化器落地后由卵注册表填
// {egg_id, unit, cgroup, engine_impl, model, port, gtt_gb, state}——
// 今天的托管面只有旧 subproc 口径，先如实给出可得字段，缺的字段缺席（不编造）。
type servicesEgg struct {
	EggID   string  `json:"egg_id,omitempty"`
	Unit    string  `json:"unit,omitempty"`
	Model   string  `json:"model,omitempty"`
	State   string  `json:"state,omitempty"`
	Port    int     `json:"port,omitempty"`
	GttGb   float64 `json:"gtt_gb,omitempty"`
	Managed bool    `json:"managed"` // 恒 true：eggs[] 只装本端托管项
}

// servicesSnapshot 组装只读快照（纯函数，便于测试）。
//
// ⚠ TODO(P3 接线点)：external_occupancy[] 只含**非引擎 GPU 使用者**（§8.7）——
// 数据来自 monitor 逐进程 GTT 归因（vitals.go AttribSample），并要排除本端托管卵的 pid；
// P4 暂以占位空数组如实呈现（没有数据就给空，不编造），接线时替换。
func servicesSnapshot(slot *servicesSlot, eggs []servicesEgg, gtt monitor.GttSample, now time.Time) map[string]interface{} {
	payload := map[string]interface{}{
		"slot":               slot,
		"eggs":               eggs,
		"external_occupancy": []map[string]interface{}{},
		"gtt": map[string]interface{}{
			"known":    gtt.Ok,
			"used_gb":  round1f(gtt.UsedGb()),
			"total_gb": round1f(gtt.TotalGb()),
		},
		"generated_at": now.UTC().Format(time.RFC3339),
	}
	return payload
}

// round1f 四舍五入到 1 位小数（与心跳上报的口径一致）。
func round1f(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

// handleServices GET /services —— 只读：槽位与卵的现状。
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

	// slot：单槽现实——取驻留面里最新的就绪/加载项（P2 状态机接线后由槽位状态填）。
	var slot *servicesSlot
	if name := mgr.CurrentModel(); name != "" {
		slot = &servicesSlot{
			Occupied: true,
			Model:    name,
			State:    mgr.State(),
			Port:     mgr.CurrentPort(),
			Backend:  mgr.CurrentBackend(),
		}
	} else if mgr.State() != backend.StateIdle {
		slot = &servicesSlot{Occupied: true, State: mgr.State()}
	}

	// eggs[]：托管卵明细（subproc 口径；P1 卵注册表落地后换 egg_id/unit/engine_impl）。
	eggs := make([]servicesEgg, 0)
	for _, d := range mgr.ResidentDetail() {
		eggs = append(eggs, servicesEgg{
			EggID:   d.Alias, // TODO(P1 接线点)：换成真 egg_id
			Model:   d.Alias,
			State:   d.State,
			GttGb:   d.RssGb, // TODO(P3 接线点)：换成 GTT 口径实测值
			Managed: true,
		})
	}

	// gtt：全局 GTT 账（monitor 主口径；该平台读不到时 known=false，不编造）。
	var gtt monitor.GttSample
	if s.agent.vitals != nil {
		if v := s.agent.vitals.Snapshot(); v.Gtt.Ok {
			gtt = v.Gtt
		}
	}

	writeJSON(w, http.StatusOK, servicesSnapshot(slot, eggs, gtt, time.Now()))
}
