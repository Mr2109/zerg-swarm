// engine_live.go —— 引擎实时态（子端心跳上报 · 主控内部消费面）· GAP-20260925-45。
//
// 病灶：心跳此前不带引擎实时态 ⇒ 主控看得到「子端自报 healthy」，看不到
// 「此刻在不在算 / 有没有输出 / 卡没卡」（WatchdogObservation 只在子端 /eggs，主控未消费）。
//
// 本结构透传子端上报的引擎实时事实，经 FleetSnapshot 出现在 /api/fleet/status 的
// machines[] 条目里，供消费方区分「挂住」与「只是慢」。
//
// 形状与 agent 侧 backend.EngineLive **逐字段同形**（两侧独立模块、不能互相 import，
// 故各留一份镜像；字段名与 JSON 键必须逐字一致）。
//
// 字段缺席 = 子端没报（零值缺席），不编造；旧读者忽略新增字段。
package store

// EngineLive 一枚引擎的实时态（与 agent 侧 backend.EngineLive 同形）。
type EngineLive struct {
	EggID    string `json:"egg_id"`           // 注册表卵名
	State    string `json:"state"`            // ready/loading/crashed/idle_armed/draining/sleeping
	PID      int    `json:"pid,omitempty"`    // 引擎进程 pid；0 ⇒ 缺席
	Inflight int    `json:"inflight"`         // 在飞引用计数（>0 = 此刻在算）
	FailN    int    `json:"fail_n,omitempty"` // 连续失败计数（非窗口计数）
	// ── 最近一次看门狗观测（子端侧 nil = 还没被看过 ⇒ 四格缺席）──
	Verdict        string  `json:"verdict,omitempty"`              // 判词：ok/extended/stuck_*/degraded/dead_unit
	Reason         string  `json:"reason,omitempty"`               // 判词理由
	LastOutputAgoS float64 `json:"last_output_ago_s,omitempty"`    // 距最近一次输出的秒数
	CPUUsecDelta   uint64  `json:"cpu_usage_usec_delta,omitempty"` // 窗口内 CPU 工时增量
}
