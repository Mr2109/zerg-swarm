package resources

// 本文件原为批 1 的 KV 估算经验回退常量表（架构族 → KVHeads/HeadDim）。
//
// 批 3 起，实现上移到共享模块 github.com/Mr2109/zerg-swarm/shared/resources/arch.go；
// 纪律不变：这些常量**不是实测值**，用一次就必须标 Estimated=true（§八 Q4）。
// 批 3 已为真值补齐读键路径（core/internal/modelreg/probe_meta.go 读 attention.head_count_kv 等）。
// 导出符号由同包的 ledger.go（转发层）统一转发；本文件保留以免既有文件清单/引用断裂。
