// services_endpoint.go —— P4→P7：只读可观测面。
//
// 设计依据：设计-子端沙箱化-20260914.md §5.4 观测面——
// 载荷 **{slot, eggs[], external_occupancy[], gtt}**。
//
// P7 批 1 起装配逻辑真数据接线完成，全部搬进 eggs_endpoint.go（本文件只留历史注脚，
// 兼容引用旧注释的读者——实现统一在 eggs_endpoint.go，避免两处各说一份）。
//
// 本文件历史（保留供追溯）：P4 把载荷由旧的 {declared_ports, reclaim, baseline, leases}
// 重定义为上述四块；旧字段随 P4 退场清理删除（附录 C·C8）。
package server
