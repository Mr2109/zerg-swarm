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
	"strings"
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

//go:embed intent-plan.json
var intentPlanRaw []byte

//go:embed receipt.json
var receiptRaw []byte

//go:embed dev-candidate.json
var devCandidateRaw []byte

//go:embed ai-boundary.json
var aiBoundaryRaw []byte

//go:embed stage-gate.json
var stageGateRaw []byte

//go:embed selfupdate.json
var selfUpdateRaw []byte

//go:embed reap.json
var reapRaw []byte

// ReceiptSpec —— 交接回执的真源（§20.3 H3 · §20.1 步 9 · §十二 `P-119`/`P-120` · 开工单 T-61）。
type ReceiptSpec struct {
	Schema           string   `json:"schema"`
	Note             string   `json:"note"`
	Fields           []string `json:"fields"`
	RequiredFields   []string `json:"required_fields"`
	Landing          string   `json:"landing"`
	LandingDeviation string   `json:"landing_deviation"`
	TraceIDRule      string   `json:"trace_id_rule"`
	ResumeEntry      string   `json:"resume_entry"`
	ResumeEntryNote  string   `json:"resume_entry_note"`
	H1Rule           string   `json:"h1_rule"`
	VerdictWords     []string `json:"verdict_words"`
}

// Receipt —— 解出回执真源（解不动 / 缺关键格 ⇒ 报错，不吞）。
func Receipt() (*ReceiptSpec, error) {
	var s ReceiptSpec
	if err := json.Unmarshal(receiptRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 回执真源解不动（receipt.json 坏了？）: %w", err)
	}
	if len(s.RequiredFields) == 0 || s.ResumeEntry == "" || len(s.Fields) == 0 {
		return nil, fmt.Errorf("contract: 回执真源缺关键格（required_fields / resume_entry / fields 有一个是空的）")
	}
	return &s, nil
}

// IntentPlanSpec —— 意图件（`Plan` 对象）的四闭集与必备字段真源（§十八.3-3 · 开工单 T-59）。
//
// 一句话：把「想做什么」变成**可校验 · 可干跑 · 可审计**的数据 —— 七语义件 `F1`–`F7`
// 加四附加件，四闭集（包封 `kind` / `target.kind` / `action` / `error.kind`）都在这里，
// **读侧不再另写一份**（另写一份就是第二份真源）。
type IntentPlanSpec struct {
	Schema              string            `json:"schema"`
	Note                string            `json:"note"`
	EnvelopeSchema      string            `json:"envelope_schema"`
	EnvelopeKinds       []string          `json:"envelope_kinds"`
	Fields              []string          `json:"fields"`
	FieldIDs            map[string]string `json:"field_ids"`
	ExtraFields         []string          `json:"extra_fields"`
	TargetKinds         []string          `json:"target_kinds"`
	Actions             []string          `json:"actions"`
	Statuses            []string          `json:"statuses"`
	ErrorKinds          []string          `json:"error_kinds"`
	Layers              []string          `json:"layers"`
	LayerRule           string            `json:"layer_rule"`
	CapabilityVocabSrc  string            `json:"capability_vocab_source"`
	CapabilityDecidable []string          `json:"capability_decidable"`
	CapabilityCounter   string            `json:"capability_counter_rule"`
}

// IntentPlan —— 解出意图件真源（解不动 / 任一把闭集为空 ⇒ 报错，不吞 —— 空闭集 = 消费侧无从判）。
func IntentPlan() (*IntentPlanSpec, error) {
	var s IntentPlanSpec
	if err := json.Unmarshal(intentPlanRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 意图件真源解不动（intent-plan.json 坏了？）: %w", err)
	}
	for name, set := range map[string][]string{
		"envelope_kinds": s.EnvelopeKinds, "target_kinds": s.TargetKinds,
		"actions": s.Actions, "statuses": s.Statuses, "error_kinds": s.ErrorKinds,
		"fields": s.Fields, "layers": s.Layers,
	} {
		if len(set) == 0 {
			return nil, fmt.Errorf("contract: 意图件真源的闭集 %s 是空的 —— 空闭集 = 消费侧无从判", name)
		}
	}
	return &s, nil
}

// Has 判一个值在不在闭集里（**精确相等**，不做大小写折叠、不做前缀匹配）。
func Has(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

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

// DevCandidateSpec —— 候选区闭集 + 验收证据单形状 + 回滚既有四值 + SD10-b 规范入口（§17.4 · 开工单 T-58）。
//
// 一句话：`zerg dev` 这一族**能写到哪**（闭集）、**判据长什么样**（证据单）、**回滚用哪几个数**
// （沿用既有四值、不新增码）、以及 `build`/`release` 撞名时**谁是规范入口**（SD10-b）——
// 四件事一次落成真源，生产侧与消费侧**读同一份**（另写一份就是第二份真源）。
type DevCandidateSpec struct {
	Schema               string            `json:"schema"`
	Note                 string            `json:"note"`
	CandidateRoot        string            `json:"candidate_root"`
	CandidateIDPattern   string            `json:"candidate_id_pattern"`
	CandidateIDRule      string            `json:"candidate_id_rule"`
	ProductionRoots      []string          `json:"production_roots"`
	ProductionExcludes   []string          `json:"production_excludes"`
	ProductionNameRule   string            `json:"production_name_rule"`
	EvidenceVerdicts     []string          `json:"evidence_verdicts"`
	EvidenceEntryFields  []string          `json:"evidence_entry_fields"`
	EvidenceKeysRequired []string          `json:"evidence_keys_required"`
	EvidenceKeysOptional []string          `json:"evidence_keys_optional"`
	EvidenceEmptyRule    string            `json:"evidence_empty_rule"`
	RollbackExitCodes    []int             `json:"rollback_exit_codes"`
	RollbackNote         string            `json:"rollback_note"`
	StepOrder            []string          `json:"step_order"`
	StepOrderRule        string            `json:"step_order_rule"`
	CanonicalEntry       map[string]string `json:"canonical_entry"`
	NotOpened            string            `json:"not_opened"`
	Boundary             string            `json:"boundary"`
}

// DevCandidate —— 解出候选区真源（解不动 / 任一闭集为空 ⇒ 报错，不吞 —— 空闭集 = 消费侧无从判）。
func DevCandidate() (*DevCandidateSpec, error) {
	var s DevCandidateSpec
	if err := json.Unmarshal(devCandidateRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 候选区真源解不动（dev-candidate.json 坏了？）: %w", err)
	}
	for name, set := range map[string][]string{
		"evidence_verdicts":      s.EvidenceVerdicts,
		"evidence_entry_fields":  s.EvidenceEntryFields,
		"evidence_keys_required": s.EvidenceKeysRequired,
		"production_roots":       s.ProductionRoots,
		"step_order":             s.StepOrder,
	} {
		if len(set) == 0 {
			return nil, fmt.Errorf("contract: 候选区真源的闭集 %s 是空的 —— 空闭集 = 消费侧无从判", name)
		}
	}
	if s.CandidateRoot == "" || s.CandidateIDPattern == "" || len(s.RollbackExitCodes) == 0 {
		return nil, fmt.Errorf("contract: 候选区真源缺关键格（candidate_root / candidate_id_pattern / rollback_exit_codes 有一个是空的）")
	}
	return &s, nil
}

// StepIndex 求一个动作在 $17.7 次序里的下标（未知动作 ⇒ -1）。
func (s *DevCandidateSpec) StepIndex(action string) int {
	for i, a := range s.StepOrder {
		if a == action {
			return i
		}
	}
	return -1
}

// AIBoundarySpec —— AI 的边界与授权真源（§九 M18 · §十二 `P-098`–`P-103` · 开工单 T-60）。
//
// 一句话：把四件事落成**可机检的闭集** —— ① 「提 ≠ 批」两个字段 + 谁不许批；
// ② 模型侧 argv/环境里零控制面端口与令牌；③ 「放文件即生效」两处口子的收口；④ `sudo` 的处置。
type AIBoundarySpec struct {
	Schema             string         `json:"schema"`
	Note               string         `json:"note"`
	SubjectKinds       []string       `json:"subject_kinds"`
	SubjectKindRule    string         `json:"subject_kind_rule"`
	ProposeJudgeRule   string         `json:"propose_judge_rule"`
	ModelSideForbidden ModelSideScan  `json:"model_side_forbidden"`
	FileDropDoors      []FileDropDoor `json:"file_drop_doors"`
	DoorWriteKeywords  []string       `json:"door_write_keywords"`
	DoorWriteRule      string         `json:"door_write_rule"`
	Sudo               SudoVerdict    `json:"sudo"`
	Boundary           string         `json:"boundary"`
}

// ModelSideScan —— 模型侧禁令牌的扫描面（paths 是**相对仓根**的目录）。
type ModelSideScan struct {
	Ports               []string `json:"ports"`
	EnvPrefix           string   `json:"env_prefix"`
	ScanPaths           []string `json:"scan_paths"`
	ScanExcludeSuffixes []string `json:"scan_exclude_suffixes"`
	Rule                string   `json:"rule"`
	MatchMode           string   `json:"match_mode"`
}

// FileDropDoor —— 一处「放文件即生效」的口子与它的收口。
type FileDropDoor struct {
	What       string `json:"what"`
	PathSource string `json:"path_source"`
	Writer     string `json:"writer"`
	Rule       string `json:"rule"`
	ListFace   string `json:"list_face"`
}

// SudoVerdict —— `sudo` 的处置定案（`P-103`）。
type SudoVerdict struct {
	Verdict       string `json:"verdict"`
	Detail        string `json:"detail"`
	MatchedRule   string `json:"matched_unless"`
	RulesYAMLNote string `json:"rules_yaml_note"`
}

// AIBoundary —— 解出 AI 边界真源（解不动 / 任一关键格空 ⇒ 报错，不吞）。
func AIBoundary() (*AIBoundarySpec, error) {
	var s AIBoundarySpec
	if err := json.Unmarshal(aiBoundaryRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: AI 边界真源解不动（ai-boundary.json 坏了？）: %w", err)
	}
	if len(s.SubjectKinds) == 0 || len(s.ModelSideForbidden.Ports) == 0 ||
		len(s.ModelSideForbidden.ScanPaths) == 0 || len(s.FileDropDoors) == 0 || s.Sudo.Detail == "" {
		return nil, fmt.Errorf("contract: AI 边界真源缺关键格（subject_kinds / model_side_forbidden / file_drop_doors / sudo 有一个是空的）")
	}
	return &s, nil
}

// StageGateSpec —— 三阶推进的升阶闸门真源（§20.4 · §20.7 `OM11` · 开工单 T-62）。
//
// 一句话：把三阶表右列的**人判**判据落成**可机检**的闭集（`stages[].criteria[].machine`），
// 并把「升阶」本身也落成四道闸 —— 缺一道即不算过。
type StageGateSpec struct {
	Schema               string              `json:"schema"`
	Note                 string              `json:"note"`
	Stages               []StageDef          `json:"stages"`
	StageAdvancePreconds map[string][]string `json:"stage_advance_preconditions"`
	FourGates            []StageGateDef      `json:"four_gates"`
	FullSuiteSteps       int                 `json:"full_suite_steps"`
	FullSuiteStepsNote   string              `json:"full_suite_steps_note"`
	HumanAbsentRule      string              `json:"human_absent_rule"`
	MissingOneRule       string              `json:"missing_one_rule"`
	NoNewCommand         string              `json:"no_new_command"`
}

// StageDef —— 一阶（id/名/判据表）。
type StageDef struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Criteria []StageCriterion `json:"criteria"`
}

// StageCriterion —— 一条判据（`machine` 是**可机检**的那一面）。
type StageCriterion struct {
	ID      string `json:"id"`
	What    string `json:"what"`
	Machine string `json:"machine"`
}

// StageGateDef —— 一道闸。
type StageGateDef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Machine string `json:"machine"`
}

// StageGate —— 解出升阶闸门真源（解不动 / 缺关键格 ⇒ 报错，不吞）。
func StageGate() (*StageGateSpec, error) {
	var s StageGateSpec
	if err := json.Unmarshal(stageGateRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 升阶闸门真源解不动（stage-gate.json 坏了？）: %w", err)
	}
	if len(s.Stages) != 3 || len(s.FourGates) != 4 || s.FullSuiteSteps <= 0 {
		return nil, fmt.Errorf("contract: 升阶闸门真源缺关键格（三阶 / 四道闸 / 全量步数 有一个不齐）")
	}
	for _, st := range s.Stages {
		if len(st.Criteria) == 0 {
			return nil, fmt.Errorf("contract: 升阶闸门真源的 %s 一条判据都没有（空判据 = 又回到人判）", st.ID)
		}
		for _, c := range st.Criteria {
			if strings.TrimSpace(c.Machine) == "" {
				return nil, fmt.Errorf("contract: 升阶闸门真源 %s/%s 的**可机检**那一格是空的（OM11 要治的正是这个）", st.ID, c.ID)
			}
		}
	}
	return &s, nil
}

// SelfUpdateSpec —— 自更新/换件的**唯一真源**（§7.1 `P12` · §3.3 `N2` · §十七 `SD12` · 开工单 T-52）。
//
// 一句话：六阶段内核是**一支脚本**（`scripts/build/zerg-upgrade.sh`），Go 与 Rust 两处都是**调用方**；
// 退码表也只有这一份 —— 两边各写一遍正是「同数字两义」的病根（实测：`4` 在两边一个意思都没有对上）。
type SelfUpdateSpec struct {
	Schema               string            `json:"schema"`
	Note                 string            `json:"note"`
	TruthSource          map[string]string `json:"truth_source"`
	Stages               []string          `json:"stages"`
	ExitCodes            []SelfUpdateCode  `json:"exit_codes"`
	Callers              []map[string]any  `json:"callers"`
	GoConstsRule         string            `json:"go_consts_rule"`
	SemanticsSplit       map[string]any    `json:"semantics_split"`
	NearSynonymForbidden []string          `json:"near_synonym_forbidden"`
	NearSynonymRule      string            `json:"near_synonym_rule"`
	OneWordRule          string            `json:"one_word_rule"`
	CollisionFixed       string            `json:"collision_fixed"`
	Boundary             string            `json:"boundary"`
}

// SelfUpdateCode —— 真源里的一格退码。
type SelfUpdateCode struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

// SelfUpdate —— 解出自更新真源（解不动 / 缺关键格 ⇒ 报错，不吞）。
func SelfUpdate() (*SelfUpdateSpec, error) {
	var s SelfUpdateSpec
	if err := json.Unmarshal(selfUpdateRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 自更新真源解不动（selfupdate.json 坏了？）: %w", err)
	}
	if len(s.ExitCodes) == 0 || len(s.Stages) != 6 || strings.TrimSpace(s.TruthSource["kernel"]) == "" {
		return nil, fmt.Errorf("contract: 自更新真源缺关键格（exit_codes / 六阶段 / truth_source.kernel 有一个不齐）")
	}
	return &s, nil
}

// ExitCodeNamed 按**名**取一格（名字不在真源里 ⇒ ok=false —— 缺就缺，不猜）。
func (s *SelfUpdateSpec) ExitCodeNamed(name string) (int, bool) {
	for _, e := range s.ExitCodes {
		if e.Name == name {
			return e.Code, true
		}
	}
	return 0, false
}

// ReapSpec —— 回收与残留的真源（§十五.2/§十五.3 · `RC1`–`RC14` · 开工单 T-54）。
//
// 一句话：三档回收权 × 五类差集**逐条有判词**；`RC11` 红线（入库件永不进候选）单独一栏；
// 干跑集指纹 `plan_id`（`RC12`）与退役标记落点（`P-111`）都在这里 —— 消费侧不另写一份。
type ReapSpec struct {
	Schema           string            `json:"schema"`
	Note             string            `json:"note"`
	Tiers            []ReapTier        `json:"tiers"`
	Differences      []ReapDifference  `json:"differences"`
	Tier3Prefixes    []string          `json:"tier3_prefixes"`
	ObjectClasses    []ReapObjectClass `json:"object_classes"`
	LegendNote       string            `json:"legend_note"`
	AgeThresholdRule string            `json:"age_threshold_rule"`
	PlanIDRule       string            `json:"plan_id_rule"`
	RedLine          ReapRedLine       `json:"red_line"`
	RetiredMarker    map[string]string `json:"retired_marker"`
	RCRules          []ReapRCRule      `json:"rc_rules"`
	NotOpened        string            `json:"not_opened"`
	Boundary         string            `json:"boundary"`
}

// ReapTier —— 一档回收权。
type ReapTier struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Rule string `json:"rule"`
}

// ReapDifference —— 一类差集（M9 的 `D-A…D-E`）。
type ReapDifference struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	What  string `json:"what"`
	Tier  string `json:"tier"`
	Judge string `json:"judge"`
}

// ReapObjectClass —— 清册里的一类对象（§十五.3 的 15 类里本件落的那几类）。
type ReapObjectClass struct {
	ID    string   `json:"id"`
	Class string   `json:"class"`
	Globs []string `json:"globs"`
	Tier  string   `json:"tier"`
	Judge string   `json:"judge"`
}

// ReapRedLine —— `RC11` 红线。
type ReapRedLine struct {
	Rule        string `json:"rule"`
	Enforcement string `json:"enforcement"`
	Known       string `json:"known"`
}

// ReapRCRule —— `RC*` 一条。
type ReapRCRule struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Reap —— 解出回收真源（解不动 / 缺关键格 ⇒ 报错，不吞）。
func Reap() (*ReapSpec, error) {
	var s ReapSpec
	if err := json.Unmarshal(reapRaw, &s); err != nil {
		return nil, fmt.Errorf("contract: 回收真源解不动（reap.json 坏了？）: %w", err)
	}
	if len(s.Tiers) != 3 || len(s.Differences) != 5 || len(s.RCRules) != 14 ||
		strings.TrimSpace(s.RedLine.Rule) == "" || strings.TrimSpace(s.PlanIDRule) == "" {
		return nil, fmt.Errorf("contract: 回收真源缺关键格（三档 / 五类差集 / RC1–RC14 / 红线 / plan_id 有一处不齐）")
	}
	for _, d := range s.Differences {
		if strings.TrimSpace(d.Judge) == "" {
			return nil, fmt.Errorf("contract: 回收真源里 %s 没有判词 —— 判据④要的是「逐条有判词」", d.ID)
		}
	}
	for _, c := range s.ObjectClasses {
		if strings.TrimSpace(c.Judge) == "" || len(c.Globs) == 0 {
			return nil, fmt.Errorf("contract: 回收真源里清册类 %s 没有判词或没有 glob —— 判据④要的是「逐条有判词」", c.ID)
		}
	}
	return &s, nil
}

// JudgeOf 按差集 id 取判词（不在真源里 ⇒ ok=false —— 缺就缺，不猜）。
func (s *ReapSpec) JudgeOf(id string) (string, bool) {
	for _, d := range s.Differences {
		if d.ID == id {
			return d.Judge, true
		}
	}
	return "", false
}
