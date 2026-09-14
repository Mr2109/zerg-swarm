// services_endpoint.go —— P7：只读可观测面（基线服务与借用租约）。
//
// 设计依据：《设计-子端服务切换与基线服务声明》§11 M9 ——
//
//	「增只读接口（谁在借 / 剩余时间 / 状态）+ UI 显示借用徽标；发现结果同时可用于
//	  回填 baseline（解决声明漂移）」。
//
// 本文件只读：GET /services 返回基线与租约的快照，**不含任何写动作**（红线②）。
package server

import (
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

// servicesSnapshot 组装只读快照（纯函数，便于测试）。
//
// 字段：
//   - declared_ports：baseline 声明的端口（真源：ZERG_BASELINE_PORTS）
//   - reclaim：借还档位（borrow | refuse）
//   - baseline：每个声明端口的只读检查结果（端口→身份→进程→托管方式）
//   - leases：借用租约（谁在借 / 状态 / 期限 / 是否该归还）
func servicesSnapshot(svcs []backend.BaselineService, leases []backend.ServiceLease,
	reclaim string, declared []int, now time.Time) map[string]interface{} {

	leaseViews := make([]map[string]interface{}, 0, len(leases))
	for _, l := range leases {
		should, why := backend.LeaseActionable(l, now)
		leaseViews = append(leaseViews, map[string]interface{}{
			"port":          l.Port,
			"kind":          l.Kind,
			"class":         l.Class,
			"identity":      l.Identity,
			"state":         l.State,
			"acquired_at":   l.AcquiredAt,
			"last_activity": l.LastActive,
			"ttl_s":         l.TTLS,
			"max_hold_s":    l.MaxHoldS,
			"approx_gb":     l.ApproxGB,
			"should_return": should,
			"return_reason": why,
			"pipeline_pids": l.Pipeline,
		})
	}

	return map[string]interface{}{
		"declared_ports": declared,
		"reclaim":        reclaim,
		"baseline":       svcs,
		"leases":         leaseViews,
		"generated_at":   now.UTC().Format(time.RFC3339),
	}
}

// handleServices GET /services —— 只读：基线与租约现状。
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
	writeJSON(w, http.StatusOK, servicesSnapshot(
		mgr.BaselineServices(),
		backend.ListLeases(),
		backend.ReclaimMode(),
		backend.DeclaredBaselinePorts(),
		time.Now(),
	))
}
