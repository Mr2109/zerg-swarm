package resources

// 本文件原为批 1 的驱逐排序实现（RankEvictions / EvictToFree / ProtectedReason / 五档常量）。
//
// 批 3 起，实现上移到共享模块 github.com/Mr2109/zerg-swarm/shared/resources/evict.go ——
// 主控（core）与子端（agent）两个 Go 模块必须共用同一份裁决，否则两侧规则会各自漂移；
// 而 Go 的 internal 规则不允许子端模块导入 core/internal/...，故实现只能住在共享模块里。
//
// 导出符号由同包的 ledger.go（转发层）统一转发；本文件保留以免既有文件清单/引用断裂。
