// services_endpoint_test.go —— P4 只读快照的回归（载荷重定义：{slot, eggs[], external_occupancy[], gtt}）。
package server

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

func TestServicesSnapshot_Shape(t *testing.T) {
	now := time.Now()
	slot := &servicesSlot{Occupied: true, Model: "m1", State: "ready", Port: 58100, Backend: "llama-server"}
	eggs := []servicesEgg{{EggID: "m1", Model: "m1", State: "ready", Port: 58100, Managed: true}}

	got := servicesSnapshot(slot, eggs, monitor.GttSample{UsedBytes: 32 << 30, TotalBytes: 64 << 30, Ok: true}, now)

	if s, ok := got["slot"].(*servicesSlot); !ok || s.Model != "m1" || s.Port != 58100 {
		t.Errorf("slot 形状不对，实得 %v", got["slot"])
	}
	eg, ok := got["eggs"].([]servicesEgg)
	if !ok || len(eg) != 1 || eg[0].Model != "m1" || !eg[0].Managed {
		t.Errorf("eggs 应为 1 条托管卵，实得 %v", got["eggs"])
	}
	// external_occupancy[]：只含非引擎 GPU 使用者——P4 占位为空数组（不得编造条目）。
	eo, ok := got["external_occupancy"].([]map[string]interface{})
	if !ok || len(eo) != 0 {
		t.Errorf("external_occupancy 应为空数组（P3 接线点），实得 %v", got["external_occupancy"])
	}
	g, ok := got["gtt"].(map[string]interface{})
	if !ok || g["known"] != true {
		t.Errorf("gtt 应带 known/used_gb/total_gb，实得 %v", got["gtt"])
	}
	if _, ok := got["generated_at"].(string); !ok {
		t.Error("应带 generated_at（可观测面的时间基准）")
	}
	// 旧字段必须已退场（附录 C·C8）。
	for _, k := range []string{"declared_ports", "reclaim", "baseline", "leases"} {
		if _, exists := got[k]; exists {
			t.Errorf("旧字段 %s 已随 P4 退场，不得再出现在 /services 载荷里", k)
		}
	}
}

func TestServicesSnapshot_GttUnknownNoFake(t *testing.T) {
	// GTT 读不到 ⇒ known=false，used/total 如实为 0（绝不编数）。
	got := servicesSnapshot(nil, nil, monitor.GttSample{}, time.Now())
	g, ok := got["gtt"].(map[string]interface{})
	if !ok {
		t.Fatalf("gtt 块缺失: %v", got["gtt"])
	}
	if g["known"] != false || g["used_gb"] != 0.0 || g["total_gb"] != 0.0 {
		t.Fatalf("GTT 未知时必须 known=false 且数值缺席/为 0，实得 %v", g)
	}
	if s, ok := got["slot"].(*servicesSlot); !ok || s != nil {
		t.Fatalf("无驻留时 slot 应为 nil（空槽），实得 %v", got["slot"])
	}
	if eg, ok := got["eggs"].([]servicesEgg); !ok || len(eg) != 0 {
		t.Fatalf("无卵应为长度 0 的数组，实得 %v", got["eggs"])
	}
}
