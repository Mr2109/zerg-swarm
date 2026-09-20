// contract.go —— 契约登记的**单一真源读取面**（§九 M16 · §十二 `P-084`/`P-085`）。
//
// 形状照仓内先例（`S-a` `core/internal/compat`）：清单用 `go:embed` 进二进制，
// 消费者读的是**同一份** —— 不复制、不转抄（转抄就是第二份真源）。
//
// 谁在用它（引用面**非 0** 是这一条的判据）：`core/cmd/zerg` 的 `zerg help registry`
// 直接读本包渲染成表 ⇒ 登记表不是「放着好看的文件」，它在命令面上真的被读。
package contract

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed registry.json
var raw []byte

//go:embed dev-targets.json
var devTargetsRaw []byte

//go:embed unresolved-entries.json
var unresolvedRaw []byte

//go:embed decision-records.json
var decisionRecordsRaw []byte

//go:embed model-id-map.json
var modelIDMapRaw []byte

// ModelIDMapEntry —— 一条 id 映射（能力快照 id ↔ 路由表 id）。
type ModelIDMapEntry struct {
	RegistryID   string   `json:"registry_id"`
	RegistryName string   `json:"registry_name"`
	FleetIDs     []string `json:"fleet_ids"`
	Weights      string   `json:"weights"`
	Evidence     string   `json:"evidence"`
}

// ModelIDMapSpec —— id 规范化真源（§十八.3 依赖次序第一块 · `调研-R2` `R2-P1`）。
//
// 一句话：能力快照（`/api/models/registry` 的 manifest id，形如 `example-35b-v2-1-5-35b-q4-k-m`）
// 与路由表（`/api/fleet/models` 的 id，形如 `example-35b-v2`）**两套命名**今天接不上；
// 本件把「谁等于谁」落成**一张显式表**，读侧按它做**精确**映射 —— 不许模糊匹配兜。
type ModelIDMapSpec struct {
	Schema       string            `json:"schema"`
	Note         string            `json:"note"`
	Rule         string            `json:"rule"`
	EvidenceRule string            `json:"evidence_rule"`
	Entries      []ModelIDMapEntry `json:"entries"`
}

// ModelIDMap —— 解出 id 规范化真源（解不动 / 空表 ⇒ 报错，不吞）。
func ModelIDMap() (*ModelIDMapSpec, error) {
	var m ModelIDMapSpec
	if err := json.Unmarshal(modelIDMapRaw, &m); err != nil {
		return nil, fmt.Errorf("contract: id 规范化真源解不动（model-id-map.json 坏了？）: %w", err)
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("contract: id 规范化真源是空的（entries 为空）—— 空表会让能力面永远接不到路由表")
	}
	for _, e := range m.Entries {
		if e.RegistryID == "" || len(e.FleetIDs) == 0 {
			return nil, fmt.Errorf("contract: id 规范化真源有一条不成形（registry_id 空或 fleet_ids 空）—— 每一条都必须两侧齐")
		}
	}
	return &m, nil
}

// FleetIDForRegistry —— 精确查表：快照 id ⇒ 路由表 id 列表（未知 id ⇒ ok=false，缺就缺，不猜）。
func (m *ModelIDMapSpec) FleetIDForRegistry(registryID string) ([]string, bool) {
	for _, e := range m.Entries {
		if e.RegistryID == registryID {
			return e.FleetIDs, true
		}
	}
	return nil, false
}

// RegistryIDForFleet —— 反向精确查表：路由表 id ⇒ 快照 id（未知 ⇒ ok=false）。
func (m *ModelIDMapSpec) RegistryIDForFleet(fleetID string) (string, bool) {
	for _, e := range m.Entries {
		for _, f := range e.FleetIDs {
			if f == fleetID {
				return e.RegistryID, true
			}
		}
	}
	return "", false
}

// DecisionRecordSpec —— 决策记录的真源（§20.1 步 9 · §十二 `P-134`–`P-138` · 开工单 T-63）。
//
// 一句话：本件把定稿**已给出的取值**（落点 / 三格 / 8 必填 + 4 收口 / status 闭集 /
// `who_decided` 只人 / 经验落点 4 类 / 一物两态）落成**可机检的形状**，不新造字段、不新立落点。
type DecisionRecordSpec struct {
	Schema             string            `json:"schema"`
	LandingDir         string            `json:"landing_dir"`
	MinFields          []string          `json:"min_fields"`
	RequiredFields     []string          `json:"required_fields"`
	ConditionalFields  []string          `json:"conditional_fields"`
	ConditionalRules   map[string]string `json:"conditional_rules"`
	StatusSet          []string          `json:"status_set"`
	WhoDecidedRule     string            `json:"who_decided_rule"`
	AIDeniedMarkers    []string          `json:"ai_denied_markers"`
	WhySegments        []string          `json:"why_must_have_segments"`
	ProposerNotDecider string            `json:"proposer_neq_decider"`
	ExperienceDirs     []string          `json:"experience_dirs"`
	OneThingTwoStates  string            `json:"one_thing_two_states"`
	BatchRule          string            `json:"batch_rule"`
	Index              string            `json:"index"`
}

// DecisionRecords —— 解出决策记录真源（解不动 / 缺关键格 ⇒ 报错，不吞）。
func DecisionRecords() (*DecisionRecordSpec, error) {
	var s DecisionRecordSpec
	if err := json.Unmarshal(decisionRecordsRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 决策记录真源解不动（decision-records.json 坏了？）: %w", err)
	}
	if len(s.RequiredFields) == 0 || len(s.StatusSet) == 0 || s.LandingDir == "" {
		return nil, fmt.Errorf("contract: 决策记录真源缺关键格（required_fields/status_set/landing_dir 有一个是空的）—— 空真源 = 判据没有对象")
	}
	return &s, nil
}

// UnresolvedEntry —— 一个未定入口的取证一行（T-53 · §7.1 `P13` / §7.2 `U21`）。
//
// 本表**不拍归属**（那是人的活）：它只登记「谁在调我」的**逐行证据**，
// 且证据带 `file` + `line` + 这一行必须含的名字 ⇒ 判据机检能逐条复核，
// 证据不会随文件漂移而悄悄失效（挂在空气上的台账 = 第二份「规则写了没人接电」）。
// UnresolvedCaller —— 一条「谁在调我」的证据行（**可复核**：文件 + 行号 + 该行必须含的名字）。
type UnresolvedCaller struct {
	File string `json:"file"`
	Line int    `json:"line"`
	What string `json:"what"`
}

type UnresolvedEntry struct {
	ID       string             `json:"id"`
	Dir      string             `json:"dir"`
	Main     string             `json:"main"`
	Bin      string             `json:"bin"`
	WhoCalls string             `json:"who_calls"`
	Verdict  string             `json:"verdict"`
	Callers  []UnresolvedCaller `json:"callers"`
}

// UnresolvedLedger —— 七个未定入口的取证台账（原样读出，不裁剪）。
type UnresolvedLedger struct {
	Schema                  string            `json:"schema"`
	SearchCommand           string            `json:"search_command"`
	RuleKeep                string            `json:"rule_keep"`
	RuleNoCommand           string            `json:"rule_no_command"`
	ForbiddenCommandSegment []string          `json:"forbidden_command_segments"`
	Entries                 []UnresolvedEntry `json:"entries"`
}

// Unresolved —— 解出取出台账（解不动 / 空表 ⇒ 报错，不吞 —— 空台账会让「一个都不许删」无处可判）。
func Unresolved() (*UnresolvedLedger, error) {
	var l UnresolvedLedger
	if err := json.Unmarshal(unresolvedRaw, &l); err != nil {
		return nil, fmt.Errorf("contract: 未定入口台账解不动（unresolved-entries.json 坏了？）: %w", err)
	}
	if len(l.Entries) == 0 {
		return nil, fmt.Errorf("contract: 未定入口台账是空的（entries 为空）—— 空表 = 「拍板前一个都不许删」这条判据没有对象")
	}
	return &l, nil
}

// DevTargets —— 自开发面的**目标回指清单**（§17.6 `SD1` 的闭集真源）。
//
// 口径（照 `SD1` 逐字）：动机源必须是一份**人的清单** —— 目标只能**承接**既有编号
// （《待办-20260920.md》的 D/E/F/G 四族）；回指不上 ⇒ `exit 2`（**不新立码**）。
// 消费者：`core/cmd/zerg` 的 `dev proposal new`（现读同一份嵌入清单）+ 门⑫ 的静态断言。
func DevTargets() ([]string, error) {
	var d struct {
		Schema string   `json:"schema"`
		IDs    []string `json:"ids"`
	}
	if err := json.Unmarshal(devTargetsRaw, &d); err != nil {
		return nil, fmt.Errorf("contract: 目标回指清单解不动（dev-targets.json 坏了？）: %w", err)
	}
	if len(d.IDs) == 0 {
		return nil, fmt.Errorf("contract: 目标回指清单是空的（dev-targets.json 的 ids 为空）—— 空清单会让每一条提案都被拒")
	}
	return d.IDs, nil
}

// Registry —— 契约登记表（schema/authority/entries/change_flow）。
type Registry struct {
	Schema     string         `json:"schema"`
	Note       string         `json:"note"`
	Authority  map[string]any `json:"authority"`
	Entries    []Entry        `json:"entries"`
	ChangeFlow ChangeFlow     `json:"change_flow"`
}

// Entry —— 一条登记（**指向**真源，不复制真源内容）。
type Entry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Truth        string `json:"truth"`
	TruthShape   string `json:"truth_shape"`
	VersionField string `json:"version_field"`
	Gate         string `json:"gate"`
	ChangeClass  string `json:"change_class"`
	ChangeNote   string `json:"change_note"`
}

// ChangeFlow —— 变更流程（§九 M16 `V0`–`V7`）。
type ChangeFlow struct {
	Carrier                 string   `json:"carrier"`
	Steps                   []string `json:"steps"`
	AuthorNotApprover       bool     `json:"author_not_approver"`
	BreakingChangeDeprecate string   `json:"breaking_change_deprecation"`
}

// Load 解出登记表（解不动 ⇒ 报错，不吞）。
func Load() (*Registry, error) {
	var r Registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("contract: 登记表解不动（registry.json 坏了？）: %w", err)
	}
	return &r, nil
}

// Raw 原样返回嵌入的字节（导出物/对拍用）。
func Raw() []byte { return raw }
