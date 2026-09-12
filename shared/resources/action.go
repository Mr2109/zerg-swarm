package resources

// action.go —— "只卸够"的动作计划（批 3）：把 §八 Q2 五档排序变成**可执行的目标清单**。
//
// 与 RankEvictions / EvictToFree 的分工：
//   - RankEvictions / EvictToFree 只表达"排序 + 够用前缀"（批 1 的纯排序口径）；
//   - 本文件在其上再叠一层**动作准入**：只有"我们有权动作"的驻留项才能进计划。
//
// 动作准入（红线，逐条对应 §八 拍板）：
//  1. 有在飞请求（req_count>0）的驻留绝不驱逐——无论多旧、多大（Q2②）。
//  2. 未托管（managed=false）的进程绝不被接管、绝不被杀（Q6）——只报告，不进计划，
//     并且它占的内存**不许**算进"可腾退量"（不许把别人的内存当自己的）。
//  3. pin 且 TTL 未到期者不驱逐；pin 无 TTL 视为已到期（Q5：无 TTL 的 pin 等同内存泄漏）。
//  4. 无法寻址（别名与摘要都为空）者不进计划——不猜、不误伤。
//  5. 加载中（loading）者不驱逐——正被请求等待（比在飞更早的阶段，同样不许杀）。
//
// 本文件仍**只出计划、不动手**（IO/杀进程在调用方）。

// TargetID 返回一个驻留项在"动作接口"上的寻址标识（§3.1：摘要即身份、名字作别名）。
// 动作接口（子端 POST /unload）按模型名寻址，故别名优先；无别名时回退摘要。
// 两者皆空 → 空串（无法寻址，调用方必须跳过，不得猜测目标）。
func TargetID(r ResidentEntry) string {
	if r.Alias != "" {
		return r.Alias
	}
	return r.Digest
}

// ActionBlockReason 报告该驻留项**不能**被动作的原因；ok=true 表示允许动作。
// 顺序即优先级：在飞 > 未托管 > pin 未到期 > 加载中 > 无法寻址。
// 未托管项的"未托管"理由排在 pin 之前——因为"这不是我们的进程"比"我们别动它"更强。
// "加载中"（loading）不驱逐：它正被某个请求等待，杀掉它等于让那个请求失败（与在飞同理，只是阶段更早）。
func ActionBlockReason(r ResidentEntry) (string, bool) {
	if r.ReqCount > 0 {
		return "inflight", false
	}
	if !r.Managed {
		return "unmanaged", false
	}
	if r.PinActive() {
		return "pin_active", false
	}
	if r.State == StateLoading {
		return "loading", false
	}
	if TargetID(r) == "" {
		return "unaddressable", false
	}
	return "", true
}

// EvictPlanForAction 返回"只卸够"的目标序列：
// 按 §八 Q2 五档排序（RankEvictions），只取能安全动作的项，累计占用 >= needFreeGb 即停（规则④）。
//
// 不能被动作的项（在飞 / 未托管 / pin 未到期 / 无法寻址）既不出现在计划里，
// 也不计入累计可腾退量——否则会出现"计划看着够、卸完还是不够"的假腾退。
// needFreeGb<=0 返回 nil（无事可做）。需求超出全部可动作项时返回全部可动作项（尽力而为，
// 但仍只限"我们的"）。本函式不执行任何卸载。
func EvictPlanForAction(residents []ResidentEntry, needFreeGb float64) []string {
	if needFreeGb <= 0 {
		return nil
	}
	byKey := make(map[string]ResidentEntry, len(residents))
	for _, r := range residents {
		k := r.Digest + "\x00" + r.Alias
		if _, dup := byKey[k]; !dup {
			byKey[k] = r
		}
	}
	var freed float64
	var out []string
	for _, e := range RankEvictions(residents).Evictable {
		r, ok := byKey[e.Digest+"\x00"+e.Alias]
		if !ok {
			continue
		}
		if _, allowed := ActionBlockReason(r); !allowed {
			continue
		}
		out = append(out, TargetID(r))
		freed += occupiedGb(r)
		if freed >= needFreeGb {
			break
		}
	}
	return out
}

// PlannedFreeGb 汇总一份动作计划能腾出的内存（GiB）。
// 只计计划内、且确实可动作的目标（与 EvictPlanForAction 同口径），供调用方判断"够不够"。
func PlannedFreeGb(residents []ResidentEntry, targets []string) float64 {
	if len(targets) == 0 {
		return 0
	}
	want := make(map[string]bool, len(targets))
	for _, t := range targets {
		if t != "" {
			want[t] = true
		}
	}
	var sum float64
	for _, r := range residents {
		if _, allowed := ActionBlockReason(r); !allowed {
			continue
		}
		if want[TargetID(r)] {
			sum += occupiedGb(r)
		}
	}
	return sum
}
