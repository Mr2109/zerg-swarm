package resources

// 本文件原为批 1 的"跑得动吗"估算实现（EstimateFit / FitQuery / FitEstimate / 三要素）。
//
// 批 3 起，实现上移到共享模块 github.com/Mr2109/zerg-swarm/shared/resources/fit.go ——
// 主控（core）与子端（agent）共用同一份估算与结论口径（§3.2 fail-closed）。
// 导出符号由同包的 ledger.go（转发层）统一转发；本文件保留以免既有文件清单/引用断裂。
