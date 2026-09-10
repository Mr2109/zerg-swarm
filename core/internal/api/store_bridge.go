package api

// store_bridge.go — v2.5.6 ping 三级漏斗: store 快照 → FleetSnapshotLite 桥接
// 主控的 store 包（internal/store）实现机器快照——api 包通过 StoreReader 接口读取
// 避免 api 依赖 store 具体类型（解耦——测试可 mock）

import (
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// StoreSnapshotReader store 快照读取器（实现 StoreReader 接口）
type StoreSnapshotReader struct {
	Store *store.Store
}

// MachineSnapshot 实现 StoreReader 接口——store.FleetSnapshot → FleetSnapshotLite
func (r *StoreSnapshotReader) MachineSnapshot(machine string) *FleetSnapshotLite {
	if r == nil || r.Store == nil {
		return nil
	}
	snap := r.Store.GetSnapshot(machine)
	if snap == nil {
		return nil
	}
	lite := &FleetSnapshotLite{
		Healthy: snap.Healthy,
		Models:  snap.Models,
	}
	if snap.Model != nil {
		lite.Model = *snap.Model
	}
	return lite
}
