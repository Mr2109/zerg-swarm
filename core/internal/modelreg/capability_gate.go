package modelreg

import (
	"fmt"
)

// ── 能力硬门槛：按**目标引擎**判定（待修补 #11）──────────────────────────
//
// 背景（评审 ornith 观点 1）：清单/快照声明 `vision: true`（是在 llama.cpp 上探得的），
// 但路由把图发给 vLLM（其配方未挂 mmproj）→ 端点报错 → 用户只看到"这个模型不行"。
// 这条**静默失败**正是四条底线之一「绝不静默降级」所禁止的。
//
// 根因：能力断言缺引擎维度，硬门槛只能按「模型 + 能力」判 → 被引擎层打穿。
//
// 本判定器的规则（宁可保守，绝不误放）：
//   - 放行的唯一条件：**目标引擎**确实被证过支持该能力（该能力条的 engines[] 含目标引擎）；
//   - 目标引擎无该能力证据 / 能力条无引擎维度（旧快照）/ 快照缺失 / 目标引擎未知
//     → 一律 unverifiable + fail-closed（Allowed=false）+ 留痕；
//   - 能力被明确判 false（端点明确拒绝）→ 确定不支持，同样不放行
//     （这是"不支持"，不是"不能判定"——两者在 Reason 里严格分开）。
//
// 绝不读记录正文的能力、绝不把全局 capability 放行到未验证引擎。

// 能力硬门槛的原因码（机器可读；Trace 里另有人类可读的留痕）。
const (
	// CapReasonAllowed 目标引擎被证过支持该能力——唯一放行原因。
	CapReasonAllowed = "evidence_for_target_engine"
	// CapReasonNoSnapshot 读不到能力快照（文件缺失/坏 JSON/读失败）——不能判定。
	CapReasonNoSnapshot = "no_snapshot"
	// CapReasonEngineUnknown 目标引擎未知（空/无法标识）——不能判定。
	CapReasonEngineUnknown = "target_engine_unknown"
	// CapReasonNoEvidence 快照里根本没有这条能力的任何证据——不能判定。
	CapReasonNoEvidence = "no_capability_evidence"
	// CapReasonUnverifiable 该能力在快照的 unverifiable[] 里（预算不足/超时等）——不能判定。
	CapReasonUnverifiable = "unverifiable_for_target_engine"
	// CapReasonUnsupported 该能力被明确判 false（端点明确拒绝）——确定不支持。
	CapReasonUnsupported = "proven_unsupported_on_target_engine"
	// CapReasonEngineDimensionMissing 能力条没有引擎维度（旧快照，兼容代价）——不能判定。
	CapReasonEngineDimensionMissing = "capability_engine_dimension_missing"
	// CapReasonOtherEngineOnly 能力只在**别的引擎**上被证过，目标引擎没有——不能判定。
	CapReasonOtherEngineOnly = "proven_on_other_engine_only"
)

// CapabilityDecision 是"目标引擎是否被证过支持某能力"的判定结果。
//
// Allowed=false 时 Reason/Trace 必须写清「哪个引擎缺哪条能力证据」——这是留痕，
// 也是上层把拒绝原因原样转给用户的依据（绝不静默降级）。
type CapabilityDecision struct {
	Allowed bool   `json:"allowed"`
	Name    string `json:"name"`
	Engine  string `json:"engine"` // 规范化后的目标引擎（CanonicalEngine 的产出）
	Reason  string `json:"reason"`
	Trace   string `json:"trace"` // 人类可读：哪个引擎缺哪条能力证据
}

// EvaluateCapabilityForEngine 判定目标引擎是否被证过支持某能力。
//
// 参数：
//   - snap：目标模型当前的能力快照（nil = 读不到，按不能判定处理）；
//   - engine：目标引擎（路由侧给的 fleet backend，或探测侧给的引擎名，内部会规范化）；
//   - name：能力标签（标准 §四 取值表）。
//
// 返回值恒非零：任何不确定都落在 Allowed=false（fail-closed）。
func EvaluateCapabilityForEngine(snap *CapabilitySnapshotArtifact, engine, name string) CapabilityDecision {
	eng := CanonicalEngine(engine)
	if snap == nil {
		return CapabilityDecision{
			Allowed: false, Name: name, Engine: eng, Reason: CapReasonNoSnapshot,
			Trace: fmt.Sprintf("capability gate: no capability snapshot — cannot verify %q for engine %q (fail-closed)", name, eng),
		}
	}
	if eng == "" {
		return CapabilityDecision{
			Allowed: false, Name: name, Engine: "", Reason: CapReasonEngineUnknown,
			Trace: fmt.Sprintf("capability gate: target engine unknown — cannot verify %q (fail-closed)", name),
		}
	}

	// ① 能力断言（capabilities[]）优先：它有确定取值与证据。
	// 同一 name 可能有多条（每个引擎一条，取值可真假不同），故先把同名条目收齐，
	// 再**按目标引擎**取那一条——这正是"同一条能力在不同引擎上真假不同"的落点。
	var matches []Capability
	for i := range snap.Capabilities {
		if snap.Capabilities[i].Name == name {
			matches = append(matches, snap.Capabilities[i])
		}
	}

	// ①.1 目标引擎有明确条目 → 取它的取值（true=通过 / false=确定不支持）。
	for i := range matches {
		c := matches[i]
		if !containsEngine(c.Engines, eng) {
			continue
		}
		if c.Value {
			return CapabilityDecision{
				Allowed: true, Name: name, Engine: eng, Reason: CapReasonAllowed,
				Trace: fmt.Sprintf("capability gate: %q proven on engine %q (evidence: %s)", name, eng, c.Evidence),
			}
		}
		// 确定不支持——它与"不能判定"严格分开（Reason 不同），但同样不放行。
		return CapabilityDecision{
			Allowed: false, Name: name, Engine: eng, Reason: CapReasonUnsupported,
			Trace: fmt.Sprintf("capability gate: %q is proven UNSUPPORTED (value=false) on engine %q — not routed (evidence: %s)", name, eng, c.Evidence),
		}
	}

	// ①.2 目标引擎**没有**条目 → 不能判定（fail-closed）。原因按证据形态细分，便于留痕。
	if len(matches) > 0 {
		hasEngineDim := false
		var otherEngines []string
		for i := range matches {
			if len(matches[i].Engines) > 0 {
				hasEngineDim = true
				otherEngines = append(otherEngines, matches[i].Engines...)
			}
		}
		if !hasEngineDim {
			// 旧快照：能力条没有引擎维度。这不是"可用"，是"不可判定"。
			// 兼容代价：宁可 fail-closed 让用户看到原因，也不用全局 capability 放行到未验证引擎。
			return CapabilityDecision{
				Allowed: false, Name: name, Engine: eng, Reason: CapReasonEngineDimensionMissing,
				Trace: fmt.Sprintf("capability gate: %q has no engine dimension (legacy snapshot) — cannot confirm engine %q (compat cost: fail-closed)", name, eng),
			}
		}
		return CapabilityDecision{
			Allowed: false, Name: name, Engine: eng, Reason: CapReasonOtherEngineOnly,
			Trace: fmt.Sprintf("capability gate: %q proven on engines %v — target engine %q has NO evidence (fail-closed)", name, otherEngines, eng),
		}
	}

	// ② 能力在 unverifiable[] 里（本轮没探出结论：预算不足/超时）——不能判定 ≠ 不支持。
	for i := range snap.Unverifiable {
		u := snap.Unverifiable[i]
		if u.Name != name {
			continue
		}
		return CapabilityDecision{
			Allowed: false, Name: name, Engine: eng, Reason: CapReasonUnverifiable,
			Trace: fmt.Sprintf("capability gate: %q is UNVERIFIABLE on engine %q (reason: %s; evidence: %s) (fail-closed)", name, eng, u.Reason, u.Evidence),
		}
	}

	// ③ 一点证据都没有——缺 = 未知，绝不等于可用。
	return CapabilityDecision{
		Allowed: false, Name: name, Engine: eng, Reason: CapReasonNoEvidence,
		Trace: fmt.Sprintf("capability gate: no capability evidence for %q on engine %q (fail-closed)", name, eng),
	}
}

// containsEngine 判断规范化后的目标引擎是否在断言的引擎列表里（两侧都规范化）。
func containsEngine(engines []string, target string) bool {
	for _, e := range engines {
		if CanonicalEngine(e) == target {
			return true
		}
	}
	return false
}

// EnginesOf 返回某能力条声明的引擎列表（规范化、去重、保持出现顺序）。
// 找不到该能力条返回 nil（缺 = 未知）。
func EnginesOf(snap *CapabilitySnapshotArtifact, name string) []string {
	if snap == nil {
		return nil
	}
	for i := range snap.Capabilities {
		if snap.Capabilities[i].Name != name {
			continue
		}
		seen := map[string]bool{}
		out := make([]string, 0, len(snap.Capabilities[i].Engines))
		for _, e := range snap.Capabilities[i].Engines {
			ce := CanonicalEngine(e)
			if ce == "" || seen[ce] {
				continue
			}
			seen[ce] = true
			out = append(out, ce)
		}
		return out
	}
	return nil
}

// CapabilitySnapshotHasEngineDimension 报告快照里是否至少有一条能力断言带引擎维度。
// 供上层区分"旧快照（无引擎维度）"与"新快照"——兼容代价的判定口径，别用字符串猜。
func CapabilitySnapshotHasEngineDimension(snap *CapabilitySnapshotArtifact) bool {
	if snap == nil {
		return false
	}
	for i := range snap.Capabilities {
		if len(snap.Capabilities[i].Engines) > 0 {
			return true
		}
	}
	return false
}

// trimEngines 供写入侧收敛：去空白、规范化、去重、保持顺序。空串丢弃。
func trimEngines(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, e := range in {
		ce := CanonicalEngine(e)
		if ce == "" || seen[ce] {
			continue
		}
		seen[ce] = true
		out = append(out, ce)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
