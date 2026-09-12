// package gateway — 熔断卡表只读快照 + 手动复位（原因可见）。
//
// 背景：网关熔断后客户端只收到一句「无可用候选（全部熔断或排除: x3,local）」，
// 看不到「为什么熔断 / 什么时候能恢复 / 怎么手动复位」。本文件把 chat 包
// ResetCompactState/CompactStateSnapshot 的「只读快照 + 手动重置」范式复制到网关卡表：
//   - BreakerSnapshot()  只读状态快照（GET /api/gateway/breakers）
//   - ResetBreakers(host) 手动复位（POST /api/gateway/breakers/reset）
//
// 状态判定只读，不触发任何副作用（不调用 isTripped——它有副作用：冷却到期会清零计数并清 failSince）。
package gateway

import (
	"log"
	"sort"
	"strings"
	"time"
)

// BreakerInfo 单台机器的熔断状态快照（JSON 可序列化——给 UI/运维看：为什么熔断？还剩多久？）。
//
// 字段语义（与 gateway.go 的 isTripped/circuitCooldown/tripMachine 对齐）：
//   - State: 三态。本仓熔断语义里确实存在「半开」（gateway.go isTripped 注释：
//     「超过冷却期自动恢复（半开状态：放行一次试错，成功则清零，失败则重新熔断）」），因此三态都真实存在：
//     "closed"（未熔断，路由正常）、"open"（熔断中，路由跳过）、
//     "half-open"（冷却期已过，下一次路由会放行一次试错）。
//   - FailCount: 连续失败次数（failCounts）——熔断判定的输入（>= circuitFailThreshold=8 才熔断，
//     且 healthy 机器在 < healthyTripLimit=10 时被保护——只降权不摘除）。
//   - TripCount: tripMachine 触发的熔断次数（tripCounts——外部 failover 调用链的计数）。
//   - LastError / LastErrorAt / LastErrorRecorded / LastErrorSource: 最后一次失败的原因文本、
//     时间、是否真的记录过原因、以及记录它的计数路径。**有熔断状态（fail_count/trip_count/open_since
//     任一非零）却没有任何原因记录时，LastError 不给空串**——空串让人以为「没失败过」，与计数非零
//     自相矛盾（活系统上出现过 fail_count=11 + state=half-open + last_error=="" 的迷惑现场）。
//     此时 LastError = breakerNoReasonText 且 LastErrorRecorded=false，消费方据此可判定「未记录」。
//   - OpenSince: 首次熔断时间（failSince），RFC3339；从未记录熔断时间时为空串。
//   - CooldownRemainingS: 剩余冷却秒数（closed/half-open 为 0）。
type BreakerInfo struct {
	Host               string  `json:"host"`
	State              string  `json:"state"`                       // closed / open / half-open
	FailCount          int     `json:"fail_count"`                  // failCounts——连续失败次数（熔断阈值输入）
	TripCount          int     `json:"trip_count"`                  // tripCounts——tripMachine 触发次数
	FailThreshold      int     `json:"fail_threshold"`              // circuitFailThreshold（8）——UI 可显示 3/8
	HealthyTripLimit   int     `json:"healthy_trip_limit"`          // healthyTripLimit（10）——healthy 保护上限
	Healthy            bool    `json:"healthy"`                     // 心跳快照是否 healthy（无快照=false）
	LastError          string  `json:"last_error"`                  // 最后一次失败原因文本；有熔断状态却无原因记录时= breakerNoReasonText（绝不空串）
	LastErrorRecorded  bool    `json:"last_error_recorded"`         // true=last_error 是真实记录的原因；false=没记录过（值是占位说明）
	LastErrorSource    string  `json:"last_error_source,omitempty"` // 记下该原因的计数路径（markFailure/tripMachine/pickRouteExcluding）；空=从未记录
	LastErrorAt        string  `json:"last_error_at,omitempty"`
	OpenSince          string  `json:"open_since,omitempty"` // RFC3339；为空=尚未记录首次熔断时间
	CooldownRemainingS float64 `json:"cooldown_remaining_s"` // 剩余冷却秒数；closed=0
}

// breakerStateLocked 计算机器当前熔断状态（只读）。
//
// 调用方必须已持有 g.failMu（读 failCounts/failSince；内部可能经 snapshotFor 读 store 快照——
// isTripped 亦在同一把锁内读快照，行为一致）。
//
// 判定复刻 isTripped（gateway.go）的语义，但不产生副作用：
//  1. 失败数 < circuitFailThreshold(8) → closed（路由仍参与，只是降权）；
//  2. 启动宽限期内（startupGrace）且失败数 < healthyTripLimit → closed（重启窗口期不误判熔断）；
//  3. 失败数 < healthyTripLimit(10) 且快照 healthy → closed（healthy 保护——只降权不摘除）；
//  4. 已记首个熔断时间且已过 circuitCooldown(30s) → half-open（下次 isTripped 放行一次试错）；
//  5. 其余 → open。
//
// 返回值：state、openSince（可能为零值——计数已超阈值但 isTripped 尚未写入首次熔断时间）、
// 剩余冷却秒数（closed/half-open 为 0）。
func (g *Gateway) breakerStateLocked(host string) (state string, openSince time.Time, cooldownLeftS float64) {
	cnt := g.failCounts[host]
	since, hasSince := g.failSince[host]

	if cnt < circuitFailThreshold {
		return "closed", time.Time{}, 0
	}
	// 复刻 isTripped 的两处「不熔断」保护（启动宽限期 + healthy 机器保护）
	if cnt < healthyTripLimit {
		if time.Since(g.startupTime) < startupGrace {
			return "closed", time.Time{}, 0
		}
		if snap := g.snapshotFor(host); snap != nil && snap.Healthy {
			return "closed", time.Time{}, 0
		}
	}
	if hasSince {
		left := circuitCooldown - time.Since(since)
		if left <= 0 {
			// 冷却到期 = 半开（下一次 isTripped 会清零计数放行一次试错）
			return "half-open", since, 0
		}
		return "open", since, left.Seconds()
	}
	// 计数已超阈值但还没记录首次熔断时间（isTripped 首次返回 true 时才写 failSince）
	// ——按满额冷却展示，避免 UI 看到「熔断中但剩余 0 秒」的误导值。
	return "open", time.Time{}, circuitCooldown.Seconds()
}

// BreakerSnapshot 返回当前所有已知机器（候选）的熔断状态快照（只读）。
//
// 「已知机器」= 熔断相关 map 里出现过的 host（失败计数/熔断时间/失败原因/熔断次数）
// ∪ fleet 配置里的机器（Fleet 节点 + 各模型候选 host）——这样 UI 能看到全貌（含健康的 closed 机器）。
// 返回按 host 升序排序。无任何已知机器时返回空切片（nil 安全：map 未初始化也照常工作）。
//
// 锁：复用现有 failMu（failCounts/failSince/lastErr 的保护锁）+ tripMu（tripCounts 的保护锁），
// 不新造锁；两把锁不同时持有，避免锁序问题。
func (g *Gateway) BreakerSnapshot() []BreakerInfo {
	hosts := map[string]bool{}

	g.failMu.Lock()
	for h := range g.failCounts {
		hosts[h] = true
	}
	for h := range g.failSince {
		hosts[h] = true
	}
	for h := range g.lastErr {
		hosts[h] = true
	}
	for h := range g.lastErrSrc {
		hosts[h] = true
	}
	// fleet 配置里的候选机器（未失败也要出现——state=closed）
	if g.config != nil {
		for h := range g.config.Fleet {
			if h != "" {
				hosts[h] = true
			}
		}
		for _, cands := range g.config.Models {
			for _, c := range cands {
				if c.Host != "" {
					hosts[c.Host] = true
				}
			}
		}
	}
	// tripCounts 由 tripMu 保护——短暂加锁单独读取（不嵌套 failMu）
	g.tripMu.Lock()
	trips := make(map[string]int, len(g.tripCounts))
	for h, n := range g.tripCounts {
		trips[h] = n
		hosts[h] = true
	}
	g.tripMu.Unlock()

	out := make([]BreakerInfo, 0, len(hosts))
	for h := range hosts {
		state, since, left := g.breakerStateLocked(h)
		healthy := false
		if snap := g.snapshotFor(h); snap != nil {
			healthy = snap.Healthy
		}
		info := BreakerInfo{
			Host:               h,
			State:              state,
			FailCount:          g.failCounts[h],
			TripCount:          trips[h],
			FailThreshold:      circuitFailThreshold,
			HealthyTripLimit:   healthyTripLimit,
			Healthy:            healthy,
			CooldownRemainingS: left,
		}
		// 原因可见，且**有熔断状态就不可能显示空串**：计数非零说明失败过，
		// 空 last_error 会让读快照的人以为「没失败过」——两者矛盾正是本次要堵死的现场。
		reason, hasReasonKey := g.lastErr[h]
		recorded := hasReasonKey && strings.TrimSpace(reason) != ""
		switch {
		case recorded:
			info.LastError = reason
			info.LastErrorRecorded = true
			info.LastErrorSource = g.lastErrSrc[h]
		case info.FailCount > 0 || info.TripCount > 0 || state != "closed":
			info.LastError = breakerNoReasonText
			info.LastErrorRecorded = false
		default:
			// 没有任何熔断状态（closed + 计数 0）——没有「为什么」可讲，空串不构成误导
			info.LastError = ""
			info.LastErrorRecorded = false
		}
		if !since.IsZero() {
			info.OpenSince = since.UTC().Format(time.RFC3339)
		}
		if at, ok := g.lastErrAt[h]; ok && !at.IsZero() {
			info.LastErrorAt = at.UTC().Format(time.RFC3339)
		}
		out = append(out, info)
	}
	g.failMu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

// ResetBreakers 手动复位机器熔断状态（运维/UI 手动入口——「怎么恢复」不用等 30s 冷却）。
//
// host == ""：清空全部机器的熔断状态（failCounts/failSince/tripCounts/lastErr/lastErrAt/lastErrSrc）；
// host != "": 只清该 host（不存在则什么也不做）。
// 返回被清掉的机器条数（改动了至少一项熔断状态的机器数）；幂等——无状态时返回 0。
//
// lastErrSrc 不单独计入清理条数：它总是与 lastErr 同时写入（见 recordFailureReasonLocked），
// 是 lastErr 键集的子集，不会引入新的「有过状态的机器」。
//
// 锁：先 tripMu（清 tripCounts）后 failMu（清其余），两把锁不同时持有。
func (g *Gateway) ResetBreakers(host string) int {
	cleared := 0

	// 1. 清 tripCounts（tripMu）
	g.tripMu.Lock()
	if host == "" {
		cleared = len(g.tripCounts)
		g.tripCounts = map[string]int{}
	} else if _, ok := g.tripCounts[host]; ok {
		delete(g.tripCounts, host)
		cleared = 1
	}
	g.tripMu.Unlock()

	// 2. 清 failCounts/failSince/lastErr/lastErrAt（failMu）
	g.failMu.Lock()
	if host == "" {
		// 被清掉的机器数 = 有过熔断状态的 host 去重计数（与 tripCounts 的计数取较大者，
		// 因为 tripMachine 可能只记 tripCounts 而未记 failCounts——实际不会，但保守取并集）
		n := len(g.failCounts)
		for h := range g.failSince {
			if _, ok := g.failCounts[h]; !ok {
				n++
			}
		}
		for h := range g.lastErr {
			if _, ok := g.failCounts[h]; !ok {
				if _, ok2 := g.failSince[h]; !ok2 {
					n++
				}
			}
		}
		if n > cleared {
			cleared = n
		}
		g.failCounts = map[string]int{}
		g.failSince = map[string]time.Time{}
		g.lastErr = map[string]string{}
		g.lastErrAt = map[string]time.Time{}
		g.lastErrSrc = map[string]string{}
	} else {
		touched := false
		if _, ok := g.failCounts[host]; ok {
			delete(g.failCounts, host)
			touched = true
		}
		if _, ok := g.failSince[host]; ok {
			delete(g.failSince, host)
			touched = true
		}
		if _, ok := g.lastErr[host]; ok {
			delete(g.lastErr, host)
			touched = true
		}
		if _, ok := g.lastErrAt[host]; ok {
			delete(g.lastErrAt, host)
			touched = true
		}
		if _, ok := g.lastErrSrc[host]; ok {
			delete(g.lastErrSrc, host)
			touched = true
		}
		if touched && cleared < 1 {
			cleared = 1
		}
	}
	g.failMu.Unlock()

	scope := "single"
	if host == "" {
		scope = "all"
	}
	log.Printf("♻️ gateway breakers manually reset: scope=%s host=%q cleared=%d", scope, host, cleared)
	return cleared
}
