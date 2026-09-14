// services_endpoint_test.go —— P7 只读快照的回归（设计 §11 M9）。
package server

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

func TestServicesSnapshot_Shape(t *testing.T) {
	now := time.Now()
	svcs := []backend.BaselineService{{
		Port: 9001, PID: 1129430, Kind: "llama", Class: "screen",
		Identity: "/data/models/k2/k2horizon-q4_k_m.gguf", ApproxGB: 24, Listening: true,
	}}
	leases := []backend.ServiceLease{{
		Port: 9001, State: backend.LeaseBorrowed, Kind: "llama", Class: "screen",
		Identity:   "/data/models/k2/k2horizon-q4_k_m.gguf",
		AcquiredAt: now.Add(-time.Minute), LastActive: now.Add(-time.Minute),
		TTLS: 300, MaxHoldS: 1800,
	}}

	got := servicesSnapshot(svcs, leases, "borrow", []int{9001}, now)

	if got["reclaim"] != "borrow" {
		t.Errorf("reclaim 应为 borrow，实得 %v", got["reclaim"])
	}
	if dp, ok := got["declared_ports"].([]int); !ok || len(dp) != 1 || dp[0] != 9001 {
		t.Errorf("declared_ports 应为 [9001]，实得 %v", got["declared_ports"])
	}
	if b, ok := got["baseline"].([]backend.BaselineService); !ok || len(b) != 1 {
		t.Errorf("baseline 应回原样数组（1 条），实得 %v", got["baseline"])
	}
	lv, ok := got["leases"].([]map[string]interface{})
	if !ok || len(lv) != 1 {
		t.Fatalf("leases 应为 1 条视图，实得 %v", got["leases"])
	}
	if lv[0]["port"] != 9001 || lv[0]["state"] != backend.LeaseBorrowed {
		t.Errorf("租约视图字段不对：%v", lv[0])
	}
	if lv[0]["should_return"] != false {
		t.Errorf("刚借出、未到期的租约不该标记归还，实得 %v", lv[0]["should_return"])
	}
	if _, ok := got["generated_at"].(string); !ok {
		t.Error("应带 generated_at（可观测面的时间基准）")
	}
}

func TestServicesSnapshot_ExpiredLeaseFlagsReturn(t *testing.T) {
	// 可观测面要能回答"谁该还了"：超 max_hold 的租约必须标记 should_return=true 并给原因。
	now := time.Now()
	l := backend.ServiceLease{
		Port: 9000, State: backend.LeaseBorrowed,
		AcquiredAt: now.Add(-2 * time.Hour), LastActive: now.Add(-2 * time.Hour),
		TTLS: 300, MaxHoldS: 1800,
	}
	got := servicesSnapshot(nil, []backend.ServiceLease{l}, "borrow", nil, now)
	lv, ok := got["leases"].([]map[string]interface{})
	if !ok || len(lv) != 1 {
		t.Fatalf("应回 1 条租约视图，实得 %v", got["leases"])
	}
	if lv[0]["should_return"] != true {
		t.Fatalf("超 max_hold 的租约应标记为需归还，实得 %v", lv[0]["should_return"])
	}
	if s, _ := lv[0]["return_reason"].(string); s == "" {
		t.Fatal("应给出归还原因（谁该还、为什么）")
	}
}

func TestServicesSnapshot_EmptyIsValid(t *testing.T) {
	// 未声明基线、无租约 ⇒ 合法空快照（不是错误）。
	got := servicesSnapshot(nil, nil, "borrow", nil, time.Now())
	if got["reclaim"] != "borrow" {
		t.Errorf("空快照也应带档位，实得 %v", got["reclaim"])
	}
	if lv, ok := got["leases"].([]map[string]interface{}); !ok || len(lv) != 0 {
		t.Errorf("空租约应为长度 0 的数组，实得 %v", got["leases"])
	}
}
