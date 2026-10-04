// engine_live.go —— 引擎实时态只读投影（GAP-20260925-45）。
//
// 缺口：心跳体（agent/internal/heartbeat.buildBody）此前**没有引擎实时态** —— 主控
// 只能看到子端自报的 healthy/backend_state，看不到「此刻在不在算 / 有没有输出 / 卡没卡」；
// 而子端已有的 WatchdogObservation 只在 /eggs 端点里（主控侧未消费）。
//
// 本投影把每枚托管引擎的实时事实汇成一份只读快照，交心跳上报，使主控在不轮询 /eggs
// 的前提下也能区分「挂住（stuck_*）」与「只是慢（extended / 有输出）」。
//
// 红线（与 observe.go 同）：只读、不改 Manager 状态；字段取不到就缺席，不编造；
// 输出可复现（按 egg_id 排序）。
package backend

import "sort"

// EngineLive 一枚引擎的实时态（一次只读快照）。
//
// 字段缺席 = 本端真没有（零值缺席），不编造：
//   - PID 0 = 无引擎进程句柄（裸 exec 与孵化两条路径都给得上时才有值）；
//   - Verdict/Reason/LastOutputAgoS/CPUUsecDelta 四格来自该卵**最近一次**看门狗观测，
//     该卵还没被看过（watchdog == nil）时四格全缺席 —— 与「看过且判定为 ok」不是一回事。
type EngineLive struct {
	EggID    string `json:"egg_id"`           // 注册表卵名（=驻留键，真 egg_id）
	State    string `json:"state"`            // ready/loading/crashed/idle_armed/draining/sleeping
	PID      int    `json:"pid,omitempty"`    // 引擎进程 pid；0 ⇒ 缺席
	Inflight int    `json:"inflight"`         // 在飞引用计数（>0 = 此刻在算）
	FailN    int    `json:"fail_n,omitempty"` // 连续失败计数（健康检查失败或看门狗判死；成功即清零 —— 非窗口计数）
	// ── 以下来自该卵最近一次看门狗观测（设计-活性看门狗 §6；nil = 还没被看过）──
	Verdict        string  `json:"verdict,omitempty"`              // 判词：ok/extended/stuck_*/degraded/dead_unit
	Reason         string  `json:"reason,omitempty"`               // 判词理由（人工复盘）
	LastOutputAgoS float64 `json:"last_output_ago_s,omitempty"`    // 距最近一次输出的秒数（判「有没有在出」）
	CPUUsecDelta   uint64  `json:"cpu_usage_usec_delta,omitempty"` // 窗口内本单元 CPU 工时增量（判「有没有在花力气」）
}

// EngineLiveStates 当前托管引擎的实时态快照（按 egg_id 排序，可复现）。
//
// 只读：单次持锁出一份自洽快照。空表 = 本端没有托管引擎 —— 调用方据此**缺席**，
// 绝不写死空数组（与 active_requests 同纪律）。
func (m *Manager) EngineLiveStates() []EngineLive {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EngineLive, 0, len(m.procs))
	for name, sp := range m.procs {
		if sp == nil || name == "" {
			continue
		}
		e := EngineLive{
			EggID:    name,
			State:    sp.state,
			PID:      enginePIDOf(sp),
			Inflight: sp.inflight,
			FailN:    sp.failCnt,
		}
		if sp.watchdog != nil {
			w := *sp.watchdog // 拷一份：账本里的观测会被下一轮采样覆写，不共享指针
			e.Verdict = string(w.Verdict)
			e.Reason = w.Reason
			e.LastOutputAgoS = w.LastOutputAgoS
			e.CPUUsecDelta = w.CPUUsecDelta
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EggID < out[j].EggID })
	return out
}
