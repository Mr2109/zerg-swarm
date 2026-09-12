package resources

import "sort"

// 驱逐优先级"档"（§八 Q2 / §3.3d）。可排序的档只有 1/3/5；
// 规则②（有在飞请求者绝不驱逐）与规则④（同档权重更大先）不是档，而是过滤/次序键，
// 因此分别由 ProtectedReason 与排序次级键实现。
const (
	TierReclaim = 1 // ① 未托管/僵尸/崩溃且无在飞请求——最先赶
	TierIdle    = 3 // ③ 空闲（无在飞请求）——按 LRU 最旧先
	TierWait    = 5 // ⑤ 被别处等待/粘性绑定——降为最后
)

// RankedEviction 是一个可驱逐项（含排序键，供观测面解释"为什么是它先"）。
type RankedEviction struct {
	Digest       string  `json:"digest"`          // 身份（内容摘要）；本端拿不到时为空串（不编造）
	Alias        string  `json:"alias,omitempty"` // 名字/别名——动作接口（如 /unload）按名字寻址时用
	Managed      bool    `json:"managed"`         // 是否在托管清单（Q6：未托管项只标注，不得被动作）
	Tier         int     `json:"tier"`            // 1|3|5
	WeightsBytes int64   `json:"weights_bytes"`   // 同档次级键：更大者先（腾得多）
	LastUsedAgoS float64 `json:"last_used_ago_s"`
	OccupiedGb   float64 `json:"occupied_gb"`
	Reason       string  `json:"reason"`
}

// ProtectedEntry 是一个"绝不驱逐"的驻留项及其原因。
type ProtectedEntry struct {
	Digest string `json:"digest"`
	Reason string `json:"reason"` // inflight（在飞请求）| pin_active（pin 未到期）
}

// EvictionPlan 是驱逐排序的结果：有序的可驱逐列表 + 受保护列表。
type EvictionPlan struct {
	Evictable []RankedEviction `json:"evictable"` // 先赶者在前
	Protected []ProtectedEntry `json:"protected"`
}

// ProtectedReason 应用两条硬规则：① 有在飞请求者绝不驱逐；② pin 且 TTL 未到期者不驱逐。
// 在飞规则优先级最高——即便该项同时是 crashed/未托管，也不得驱逐（不杀活跃推理）。
func ProtectedReason(r ResidentEntry) (string, bool) {
	if r.ReqCount > 0 {
		return "inflight", true
	}
	if r.PinActive() {
		return "pin_active", true
	}
	return "", false
}

// tierOf 计算驻留项所属的驱逐档。
func tierOf(r ResidentEntry) int {
	if !r.Managed || r.State == StateCrashed || r.State == StateZombie {
		return TierReclaim
	}
	if r.ChurnSensitive() {
		return TierWait
	}
	return TierIdle
}

// tierReason 给出该档的人类可读理由。
func tierReason(t int) string {
	switch t {
	case TierReclaim:
		return "未托管/僵尸/崩溃且无在飞请求"
	case TierWait:
		return "被别处等待/粘性绑定，降为最后"
	default:
		return "空闲（无在飞请求）"
	}
}

// RankEvictions 把一个机器上的驻留项排成可驱逐顺序（§八 Q2 五档）。
//
// 排序键（依次）：
//  1. 档：①(1) → ③(3) → ⑤(5)；
//  2. 同档：权重（WeightsBytes）更大者先（规则④，"卸一个就够"）；
//  3. 再同：LastUsedAgoS 更大者先（规则③，LRU 最旧先）；
//  4. 最后：Digest 升序（确定性收尾，保证结果可复现）。
//
// 在飞请求者与未到期 pin 一律不进 Evictable，只进 Protected。
// 本函式**只表达排序，不执行任何驱逐**（批 1 边界）。
func RankEvictions(residents []ResidentEntry) EvictionPlan {
	plan := EvictionPlan{Evictable: []RankedEviction{}, Protected: []ProtectedEntry{}}
	for _, r := range residents {
		if reason, prot := ProtectedReason(r); prot {
			plan.Protected = append(plan.Protected, ProtectedEntry{Digest: r.Digest, Reason: reason})
			continue
		}
		t := tierOf(r)
		plan.Evictable = append(plan.Evictable, RankedEviction{
			Digest:       r.Digest,
			Alias:        r.Alias,
			Managed:      r.Managed,
			Tier:         t,
			WeightsBytes: r.WeightsBytes,
			LastUsedAgoS: r.LastUsedAgoS,
			OccupiedGb:   occupiedGb(r),
			Reason:       tierReason(t),
		})
	}
	sort.SliceStable(plan.Evictable, func(i, j int) bool {
		a, b := plan.Evictable[i], plan.Evictable[j]
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		if a.WeightsBytes != b.WeightsBytes {
			return a.WeightsBytes > b.WeightsBytes
		}
		if a.LastUsedAgoS != b.LastUsedAgoS {
			return a.LastUsedAgoS > b.LastUsedAgoS
		}
		if a.Alias != b.Alias {
			return a.Alias < b.Alias
		}
		return a.Digest < b.Digest
	})
	return plan
}

// EvictableDigests 返回可驱逐项的有序摘要列表（含受保护项的排除）。
func EvictableDigests(residents []ResidentEntry) []string {
	plan := RankEvictions(residents)
	out := make([]string, 0, len(plan.Evictable))
	for _, e := range plan.Evictable {
		out = append(out, e.Digest)
	}
	return out
}

// evictableOccupiedGb 汇总"可驱逐"驻留项的占用（GiB）——受保护项不计入。
func evictableOccupiedGb(residents []ResidentEntry) float64 {
	var sum float64
	for _, r := range residents {
		if _, prot := ProtectedReason(r); prot {
			continue
		}
		sum += occupiedGb(r)
	}
	return sum
}

// EvictToFree 按驱逐顺序取"够用的最小前缀"：累计占用 >= needFreeGb 即停（规则④"卸一个就够"，
// 替代"一把全清"的粗粒度）。needFreeGb<=0 返回 nil。需求超出全部可驱逐项时返回全部可驱逐项。
//
// 返回的是**寻址标识**（TargetID：别名优先，回退摘要）——动作接口（如子端 /unload）按名字寻址；
// 无法寻址（别名与摘要皆空）者不进列表（不猜目标）。
func EvictToFree(residents []ResidentEntry, needFreeGb float64) []string {
	if needFreeGb <= 0 {
		return nil
	}
	plan := RankEvictions(residents)
	byKey := make(map[string]ResidentEntry, len(residents))
	for _, r := range residents {
		k := r.Digest + "\x00" + r.Alias
		if _, dup := byKey[k]; !dup {
			byKey[k] = r
		}
	}
	var freed float64
	var out []string
	for _, e := range plan.Evictable {
		id := ""
		if r, ok := byKey[e.Digest+"\x00"+e.Alias]; ok {
			id = TargetID(r)
		}
		if id == "" {
			continue // 无法寻址：不进计划，也不计入可腾退量
		}
		out = append(out, id)
		freed += e.OccupiedGb
		if freed >= needFreeGb {
			break
		}
	}
	return out
}

// TotalEvictableGb 汇总全部可驱逐项的占用（GiB）。
func TotalEvictableGb(residents []ResidentEntry) float64 { return evictableOccupiedGb(residents) }
