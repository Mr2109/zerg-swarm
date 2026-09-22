// export_test.go —— 把**只读面**开给外部测试包（`package main_test`）。
//
// 为什么需要这个文件（开工单 T-42 判据②：「契约测试落外部测试包 `package main_test`」）：
// 外部测试包只能看见导出标识符，而本包（`package main`）里命令树的真源
// （`run` / `commands` / `catalog`）全是未导出的 ⇒ 不搭这座桥，「外部包」这条判据
// 就只能靠 `os/exec` 起子进程间接实现（那是**第三层**的活，见 cli_exec_test.go）。
//
// 三条纪律：
//
//	① 本文件**只在测试构建里编**（`_test.go` 后缀 + `go test` 才会带上）⇒ 制品里没有这些符号，
//	   命令面的对外面（SDK 面）一个字节都没变；
//	② 桥只开**只读**东西：跑一条命令（`RunForTest`）· 读命令树（路径/字段/档）· 读退码表。
//	   不导出任何写面入口；
//	③ 桥上的每个符号都**直接指真源**（`= run` / `= commands`），不许在桥里另写一份副本
//	   （另写一份 = 测试测的是那份副本，不是真东西）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/selfupdate"
)

// RunForTest 驱动一次命令（与 `main()` 调的是同一个 `run`）。
var RunForTest = run

// CommandPathsForTest 命令树里**全部**命令路径（逐条按 `a b` 形态给出，排序后）。
func CommandPathsForTest() []string {
	out := make([]string, 0, len(commands))
	for _, c := range catalog() {
		out = append(out, joinPath(c.path))
	}
	return out
}

// CommandInfoForTest 一条命令的形状（测试用来判「这条命令该用哪一类 must-fail」）。
type CommandInfoForTest struct {
	Fields      []string // `--json` 字段表（空 = 说明面）
	DangerLevel string   // D2 / D3（空 = 不是危险动作）
	Passthrough bool
	OpenForRun  bool // 本版是否已开放执行
}

// CommandInfoOfForTest 按路径取形状；找不到 ⇒ ok=false（测试据此判「矩阵里有幽灵条目」）。
func CommandInfoOfForTest(path string) (CommandInfoForTest, bool) {
	for _, c := range commands {
		if joinPath(c.path) != path {
			continue
		}
		info := CommandInfoForTest{Fields: append([]string{}, c.fields...), Passthrough: c.passthrough, OpenForRun: c.opened}
		if c.danger != nil {
			info.DangerLevel = c.danger.Level
		}
		return info, true
	}
	return CommandInfoForTest{}, false
}

// ExitCodeRowForTest 退码表的一格（测试用的**具名类型** —— 匿名结构体在两个包里各写一遍就会漂）。
type ExitCodeRowForTest struct {
	Code    int
	Name    string
	Meaning string
}

// ExitCodeTableForTest 退码表（主表 + 已挂号）逐格给出来 —— T3「退码表唯一」的对拍真值。
func ExitCodeTableForTest() []ExitCodeRowForTest {
	out := []ExitCodeRowForTest{}
	for _, rows := range [][]exitcodeRow{exitcodeTable, exitcodeReserved} {
		for _, r := range rows {
			out = append(out, ExitCodeRowForTest{r.Code, r.Name, r.Meaning})
		}
	}
	return out
}

// EnvelopeKeysForTest 包封六键（T4 的形状守卫用它，不另抄一份键名）。
func EnvelopeKeysForTest() []string {
	return []string{"schema", "kind", "items", "meta", "warnings", "truncated"}
}

// ContractSchemaForTest 契约号（`zerg/v1`）。
func ContractSchemaForTest() string { return contractSchema }

// joinPath 把命令路径拼成 `a b` 形态（与帮助/导出物里的写法同一口径）。
func joinPath(p []string) string {
	s := ""
	for i, seg := range p {
		if i > 0 {
			s += " "
		}
		s += seg
	}
	return s
}

// ---- T-58：候选区（`core/internal/contract/dev-candidate.json`）的**只读桥** ----
// 口径同本文件头部第 ③ 条：桥上的每个符号都**直接指真源**，不在桥里另写副本。

// DevCandidateSpecForTest 候选区真源的一格（测试用的**具名类型** —— 匿名结构体在两包里各写一遍就会漂）。
type DevCandidateSpecForTest struct {
	CandidateRoot        string
	CandidateIDPattern   string
	ProductionRoots      []string
	ProductionExcludes   []string
	EvidenceVerdicts     []string
	EvidenceKeysRequired []string
	EvidenceKeysOptional []string
	RollbackExitCodes    []int
	StepOrder            []string
	CanonicalEntry       map[string]string
}

// DevCandidateSpecOfForTest 取候选区真源（读的就是 `contract.DevCandidate()` 那一份）。
func DevCandidateSpecOfForTest() (DevCandidateSpecForTest, error) {
	s, err := contract.DevCandidate()
	if err != nil {
		return DevCandidateSpecForTest{}, err
	}
	return DevCandidateSpecForTest{
		CandidateRoot:        s.CandidateRoot,
		CandidateIDPattern:   s.CandidateIDPattern,
		ProductionRoots:      s.ProductionRoots,
		ProductionExcludes:   s.ProductionExcludes,
		EvidenceVerdicts:     s.EvidenceVerdicts,
		EvidenceKeysRequired: s.EvidenceKeysRequired,
		EvidenceKeysOptional: s.EvidenceKeysOptional,
		RollbackExitCodes:    s.RollbackExitCodes,
		StepOrder:            s.StepOrder,
		CanonicalEntry:       s.CanonicalEntry,
	}, nil
}

// CandidateIDValidForTest 候选 id 闭集判定口（真源里的 pattern）。
func CandidateIDValidForTest(id string) bool {
	spec, err := contract.DevCandidate()
	if err != nil {
		return false
	}
	return candidateIDValid(spec, id)
}

// ProductionCandidateNameViolationsForTest 生产目录里「名字带候选 id」的件（判据⑤ 的唯一判定口）。
func ProductionCandidateNameViolationsForTest(repoRoot string) ([]string, error) {
	spec, err := contract.DevCandidate()
	if err != nil {
		return nil, err
	}
	return productionCandidateNameViolations(repoRoot, spec)
}

// EvidenceSheetPathForTest 证据单落点（`dev verify` 写、`dev build`/`dev test` 读）。
func EvidenceSheetPathForTest(root, id string) string { return evidenceSheetPath(root, id) }

// SheetExitCodeForTest 证据单的四档判决口（`0` 全绿 · `1` 有 FAIL · `8` 有 BLOCKED）。
func SheetExitCodeForTest(totals map[string]int) (int, error) {
	spec, err := contract.DevCandidate()
	if err != nil {
		return 0, err
	}
	return sheetExitCode(spec, totals), nil
}

// EvidenceEntryForTest 证据单的一条。
type EvidenceEntryForTest struct {
	Criterion string
	Verdict   string
	Evidence  map[string]string
}

// EvidenceSheetForTest 证据单的形状（读回时用）。
type EvidenceSheetForTest struct {
	Schema    string
	Candidate string
	Contract  string
	Results   string
	Entries   []EvidenceEntryForTest
	Totals    map[string]int
}

// ReadEvidenceSheetForTest 读回一份证据单（解不动 ⇒ 报错，不吞）。
func ReadEvidenceSheetForTest(path string) (*EvidenceSheetForTest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s evidenceSheet
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	out := &EvidenceSheetForTest{Schema: s.Schema, Candidate: s.Candidate, Contract: s.Contract,
		Results: s.Results, Totals: s.Totals}
	for _, e := range s.Entries {
		out.Entries = append(out.Entries, EvidenceEntryForTest{Criterion: e.Criterion, Verdict: e.Verdict, Evidence: e.Evidence})
	}
	return out, nil
}

// ---- T-60：AI 边界（`core/internal/contract/ai-boundary.json`）的**只读桥** ----
// 口径同本文件头部 ③：桥上的符号直接指真源，不另写副本。

// AIBoundarySpecForTest AI 边界真源的一格。
type AIBoundarySpecForTest struct {
	SubjectKinds      []string
	Ports             []string
	EnvPrefix         string
	ScanPaths         []string
	ScanExcludeSuffix []string
	DoorWriteKeywords []string
	FileDropDoors     []string
	SudoDetail        string
}

// AIBoundarySpecOfForTest 取 AI 边界真源。
func AIBoundarySpecOfForTest() (AIBoundarySpecForTest, error) {
	s, err := contract.AIBoundary()
	if err != nil {
		return AIBoundarySpecForTest{}, err
	}
	doors := []string{}
	for _, d := range s.FileDropDoors {
		doors = append(doors, d.What+"|"+d.Writer)
	}
	return AIBoundarySpecForTest{
		SubjectKinds: s.SubjectKinds, Ports: s.ModelSideForbidden.Ports,
		EnvPrefix: s.ModelSideForbidden.EnvPrefix, ScanPaths: s.ModelSideForbidden.ScanPaths,
		ScanExcludeSuffix: s.ModelSideForbidden.ScanExcludeSuffixes,
		DoorWriteKeywords: s.DoorWriteKeywords, FileDropDoors: doors, SudoDetail: s.Sudo.Detail,
	}, nil
}

// SudoRefusalForTest 命令面的 `sudo` 拒收口（真源判据、调度前判）。
func SudoRefusalForTest(args []string) (string, bool) { return sudoRefusal(args) }

// ModelSideScanForTest 扫真源声明的模型侧面 ⇒ (命中行, 扫到的文件数)。
func ModelSideScanForTest(repoRoot string) ([]string, int, error) {
	spec, err := contract.AIBoundary()
	if err != nil {
		return nil, 0, err
	}
	vs, scanned, err := modelSideViolations(repoRoot, &spec.ModelSideForbidden)
	if err != nil {
		return nil, scanned, err
	}
	out := []string{}
	for _, v := range vs {
		out = append(out, fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Text))
	}
	return out, scanned, nil
}

// ModelSideScanRawForTest 对**给定**目录与令牌扫一遍（负控用合成夹具）。
func ModelSideScanRawForTest(root string, paths []string, ports []string, prefix string,
	exclude []string) ([]string, int, error) {
	spec := contract.ModelSideScan{Ports: ports, EnvPrefix: prefix, ScanPaths: paths,
		ScanExcludeSuffixes: exclude}
	vs, scanned, err := modelSideViolations(root, &spec)
	if err != nil {
		return nil, scanned, err
	}
	out := []string{}
	for _, v := range vs {
		out = append(out, fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Text))
	}
	return out, scanned, nil
}

// DoorWriteViolationsForTest 真命令树上的口子写面命中（判据③① 的机检面）。
func DoorWriteViolationsForTest() ([]string, error) {
	spec, err := contract.AIBoundary()
	if err != nil {
		return nil, err
	}
	return doorWriteViolations(spec), nil
}

// DoorWriteViolationsRawForTest 喂合成的命令进来（负控：证明这一格能红）。
func DoorWriteViolationsRawForTest(path, summary, usage string, args []string) ([]string, error) {
	spec, err := contract.AIBoundary()
	if err != nil {
		return nil, err
	}
	return doorWriteViolationsFor(spec, []doorCmd{{Path: path, Summary: summary, Usage: usage, Args: args}}), nil
}

// DoctorDoorItemNamesForTest doctor 的两段口子清单的名字（`P-101` ②：默认不静默）。
func DoctorDoorItemNamesForTest() []string {
	out := []string{}
	for _, it := range doctorSkillItems() {
		out = append(out, it["name"])
	}
	for _, it := range doctorMCPItems() {
		out = append(out, it["name"])
	}
	return out
}

// RepoRootForTest 仓根解析口（模型面扫描要用同一份推导，不另写一份）。
func RepoRootForTest() string { return repoRoot() }

// ---- `A2` 六层取数（变更影响面）的**只读桥** ------------------------------------------------

// ImpactSemanticGateForTest 第 ⑤ 层的**在位判据**判定口（三条子句 · 纯函数）。
// 桥它而不在测试里另写一份：另写一份 = 测的是那份副本，不是真东西（本文件第 ③ 条纪律）。
func ImpactSemanticGateForTest(tagsJSON []byte) (bool, string, string) {
	return impactSemanticGate(tagsJSON)
}

// ---- `A3` 波纹卡片的**只读桥** ---------------------------------------------------------------

// ImpactCardForTest 一张卡片的只读快照（具名类型 —— 匿名结构体在两个包里各写一遍就会漂）。
type ImpactCardForTest struct {
	Items      []map[string]string
	Raws       int
	NoWhy      int
	UnknownWhy int
	Oversize   int
	SemRed     int
	CutTier    [5]int
	Truncated  bool
	Warnings   []string
	HowRestore string

	FixedTokens, BodyTokens, TotalTokens, BytesTokens int
	RedN, Dist1N, LexMax, SimN, PubN                  int
}

func (c ImpactCardForTest) toCard() impactCard {
	return impactCard{
		Items: c.Items, Raws: c.Raws, NoWhy: c.NoWhy, UnknownWhy: c.UnknownWhy, Oversize: c.Oversize,
		SemRed: c.SemRed, CutTier: c.CutTier, Truncated: c.Truncated, Warnings: c.Warnings,
		HowRestore: c.HowRestore, FixedTokens: c.FixedTokens, BodyTokens: c.BodyTokens,
		TotalTokens: c.TotalTokens, BytesTokens: c.BytesTokens,
		RedN: c.RedN, Dist1N: c.Dist1N, LexMax: c.LexMax, SimN: c.SimN, PubN: c.PubN,
	}
}

// ImpactCardOfForTest 造卡片（纯函数 · 负控能直接喂坏条目）。
func ImpactCardOfForTest(rows []map[string]string, fixed, tgt string) ImpactCardForTest {
	c := impactCardOf(rows, fixed, tgt)
	return ImpactCardForTest{
		Items: c.Items, Raws: c.Raws, NoWhy: c.NoWhy, UnknownWhy: c.UnknownWhy, Oversize: c.Oversize,
		SemRed: c.SemRed, CutTier: c.CutTier, Truncated: c.Truncated, Warnings: c.Warnings,
		HowRestore: c.HowRestore, FixedTokens: c.FixedTokens, BodyTokens: c.BodyTokens,
		TotalTokens: c.TotalTokens, BytesTokens: c.BytesTokens,
		RedN: c.RedN, Dist1N: c.Dist1N, LexMax: c.LexMax, SimN: c.SimN, PubN: c.PubN,
	}
}

// ImpactCardJudgeForTest 判据①②③的唯一判定口（喂坏卡片 ⇒ 必须报错）。
func ImpactCardJudgeForTest(c ImpactCardForTest) error { return impactJudgeCard(c.toCard()) }

// ImpactCardWhyJudgeForTest 判据②的判定口（`why` 闭集六选一 · 硬门槛）。
func ImpactCardWhyJudgeForTest(items []map[string]string) error { return impactJudgeCardWhy(items) }

// ImpactCardRedLineJudgeForTest 判据「语义级永不进会红行」（§3.5 铁律）的判定口。
func ImpactCardRedLineJudgeForTest(items []map[string]string) error {
	return impactJudgeCardRedLine(items)
}

// ImpactWhySixForTest `why` 闭集（六选一）真源 —— 测试不另抄一份。
func ImpactWhySixForTest() []string { return append([]string{}, impactWhySix...) }

// ImpactCardCapsForTest 卡片四条硬上限（条数 / 总量 token / 单条行数 / 单条 token）。
func ImpactCardCapsForTest() (items, tokens, lines, itemTokens int) {
	return impactItemMax, impactTokenMax, impactItemLineMax, impactItemTokenMax
}

// ImpactTokenEstimateForTest token 估算（返回 保守口径, 设计现读口径=字节÷4）。
func ImpactTokenEstimateForTest(s string) (int, int) { return impactTokenEstimate(s) }

// ImpactCardItemTokensForTest 单条卡片的保守 token（判据①「单条 ≤ 60」的同一口径）。
func ImpactCardItemTokensForTest(it map[string]string) int { return impactCardItemTokens(it) }

// ImpactReversibilityForTest 退法的一格（具名类型）。
type ImpactReversibilityForTest struct {
	Tier    int
	Cmd     string
	Resolve []string
	SHA     string
	Line    string
}

// ImpactReversibilityDecideForTest 判档（纯函数）：三档逐档可喂。
func ImpactReversibilityDecideForTest(isBinProduct bool, rollback, gitSHA, rel string) ImpactReversibilityForTest {
	r := impactReversibilityDecide(isBinProduct, rollback, gitSHA, rel)
	return ImpactReversibilityForTest{r.Tier, r.Cmd, r.Resolve, r.SHA, r.Line}
}

// ImpactReversibilityOfForTest 现算退法档（只读 · 盘上找现成回滚件 + `git log -1`）。
func ImpactReversibilityOfForTest(root, rel string) ImpactReversibilityForTest {
	r := impactReversibilityOf(root, &impactTarget{Kind: impactKindFile, Raw: rel, Rel: rel})
	return ImpactReversibilityForTest{r.Tier, r.Cmd, r.Resolve, r.SHA, r.Line}
}

// ImpactJudgeReversibilityForTest 判据④（§九 判据⑨ 退法可执行性）的判定口。
func ImpactJudgeReversibilityForTest(root string, rev ImpactReversibilityForTest) error {
	return impactJudgeReversibility(root, impactReversibility{rev.Tier, rev.Cmd, rev.Resolve, rev.SHA, rev.Line})
}

// ---- T-62：升阶闸门（`core/internal/contract/stage-gate.json`）的**只读桥** ----

// StageGateSpecForTest 升阶闸门真源的一格。
type StageGateSpecForTest struct {
	Stages          int
	Criteria        int
	MachineCells    int // 可机检那一格**非空**的判据条数
	GateIDs         []string
	FullSuiteSteps  int
	HumanAbsentRule string
	MissingOneRule  string
}

// StageGateSpecOfForTest 取升阶闸门真源。
func StageGateSpecOfForTest() (StageGateSpecForTest, error) {
	s, err := contract.StageGate()
	if err != nil {
		return StageGateSpecForTest{}, err
	}
	out := StageGateSpecForTest{Stages: len(s.Stages), FullSuiteSteps: s.FullSuiteSteps,
		HumanAbsentRule: s.HumanAbsentRule, MissingOneRule: s.MissingOneRule}
	for _, st := range s.Stages {
		out.Criteria += len(st.Criteria)
		for _, c := range st.Criteria {
			if strings.TrimSpace(c.Machine) != "" {
				out.MachineCells++
			}
		}
	}
	for _, g := range s.FourGates {
		out.GateIDs = append(out.GateIDs, g.ID+"="+g.Name)
	}
	return out, nil
}

// StageGateVerdictForTest 一道闸的判决。
type StageGateVerdictForTest struct {
	ID      string
	Name    string
	Verdict string
	Detail  string
}

// JudgeFourGatesForTest 跑四道闸（判据的**唯一判定口**）。
func JudgeFourGatesForTest(results, human string) ([]StageGateVerdictForTest, bool, error) {
	spec, err := contract.StageGate()
	if err != nil {
		return nil, false, err
	}
	vs := judgeFourGates(spec, "", results, human)
	out := []StageGateVerdictForTest{}
	for _, v := range vs {
		out = append(out, StageGateVerdictForTest{v.ID, v.Name, v.Verdict, v.Detail})
	}
	return out, gatesPassed(vs, spec), nil
}

// ---- T-52：`update`/`upgrade` 不分指两事（`core/internal/contract/selfupdate.json`）的**只读桥** ----

// SelfUpdateSpecForTest 自更新真源的一格。
type SelfUpdateSpecForTest struct {
	Kernel        string
	Stages        []string
	ExitCodes     map[string]int // name → code
	ExitMeanings  map[int]string // code → meaning
	Callers       []string
	NearSynonym   []string
	CollisionNote string
}

// SelfUpdateSpecOfForTest 取自更新真源。
func SelfUpdateSpecOfForTest() (SelfUpdateSpecForTest, error) {
	s, err := contract.SelfUpdate()
	if err != nil {
		return SelfUpdateSpecForTest{}, err
	}
	out := SelfUpdateSpecForTest{Kernel: s.TruthSource["kernel"], Stages: s.Stages,
		ExitCodes: map[string]int{}, ExitMeanings: map[int]string{},
		NearSynonym: s.NearSynonymForbidden, CollisionNote: s.CollisionFixed}
	for _, e := range s.ExitCodes {
		out.ExitCodes[e.Name] = e.Code
		out.ExitMeanings[e.Code] = e.Meaning
	}
	for _, c := range s.Callers {
		if f, ok := c["file"].(string); ok {
			out.Callers = append(out.Callers, f)
		}
	}
	return out, nil
}

// UpdateSplitViolationsForTest 真命令树上的「近义动词分指两事」命中（判据②③ 的机检面）。
func UpdateSplitViolationsForTest() ([]string, error) {
	spec, err := contract.SelfUpdate()
	if err != nil {
		return nil, err
	}
	return updateSplitViolations(spec), nil
}

// UpdateSplitViolationsRawForTest 在**真命令树**上只改坏一条（负控：证明这一格能红，
// 且判词干净 —— 不掺「命令树里找不到命令」这类无关命中）。
func UpdateSplitViolationsRawForTest(path, summary, usage, dangerSource string) ([]string, error) {
	spec, err := contract.SelfUpdate()
	if err != nil {
		return nil, err
	}
	return updateSplitViolationsWith(spec, []updateCmdShape{{Path: path, Summary: summary, Usage: usage, DangerSource: dangerSource}}), nil
}

// SelfUpdateExitCodesMatchForTest 逐格对拍判定口的**投影**（真源 ⟷ 本包常量在 selfupdate 包内已自测）。
func SelfUpdateExitCodesMatchForTest() error { return selfupdate.ExitCodesMatchTruthSource() }

// ---- T-54：回收与残留（`core/internal/contract/reap.json`）的**只读桥** ----

// ReapSpecForTest 回收真源的一格。
type ReapSpecForTest struct {
	Tiers       []string
	DiffJudges  map[string]string
	TierOfDiff  map[string]string
	RCRules     []string
	RedLine     string
	PlanIDRule  string
	ObjectClass []string
	RetiredPath string
}

// ReapSpecOfForTest 取回收真源。
func ReapSpecOfForTest() (ReapSpecForTest, error) {
	s, err := contract.Reap()
	if err != nil {
		return ReapSpecForTest{}, err
	}
	out := ReapSpecForTest{DiffJudges: map[string]string{}, TierOfDiff: map[string]string{},
		RedLine: s.RedLine.Rule, PlanIDRule: s.PlanIDRule}
	for _, t := range s.Tiers {
		out.Tiers = append(out.Tiers, t.Name)
	}
	for _, d := range s.Differences {
		out.DiffJudges[d.ID] = d.Judge
		out.TierOfDiff[d.ID] = d.Tier
	}
	for _, r := range s.RCRules {
		out.RCRules = append(out.RCRules, r.ID)
	}
	for _, c := range s.ObjectClasses {
		out.ObjectClass = append(out.ObjectClass, c.ID+" "+c.Class+" tier="+c.Tier)
	}
	out.RetiredPath = s.RetiredMarker["verdict"]
	return out, nil
}

// ReapItemForTest 干跑单里的一件。
type ReapItemForTest struct {
	Rel, Tier, Class, Diff, Judge string
	Bytes                         int64
	Evidence                      map[string]string
}

// ReapPlanForTest 一份干跑单的形状。
type ReapPlanForTest struct {
	PlanID  string
	Items   []ReapItemForTest
	Skipped []ReapItemForTest
	Tracked []string
	Bytes   int64
}

// BuildReapPlanForTest 造干跑单（注入「算不算入库件」的集合 —— 让 `RC11` 红线**能负控**）。
func BuildReapPlanForTest(root string, minAgeDays int, tracked []string) (ReapPlanForTest, error) {
	spec, err := contract.Reap()
	if err != nil {
		return ReapPlanForTest{}, err
	}
	set := map[string]bool{}
	for _, t := range tracked {
		set[t] = true
	}
	p := buildReapPlan(root, spec, minAgeDays, func(rel string) bool { return set[rel] }, time.Now())
	out := ReapPlanForTest{PlanID: p.PlanID, Tracked: p.Tracked, Bytes: p.TotalBytes}
	conv := func(in []reapItem) []ReapItemForTest {
		res := []ReapItemForTest{}
		for _, it := range in {
			res = append(res, ReapItemForTest{Rel: it.Rel, Tier: it.Tier, Class: it.Class,
				Diff: it.Diff, Judge: it.Judge, Bytes: it.Bytes, Evidence: it.Evidence})
		}
		return res
	}
	out.Items = conv(p.Items)
	out.Skipped = conv(p.Skipped)
	return out, nil
}

// ReapPlanIDOfSameSetForTest 同一集两次取指纹（证明它稳定）；再喂一个改过的集（证明它能变）。
func ReapPlanIDOfSameSetForTest(root string, minAgeDays int) (string, string, error) {
	spec, err := contract.Reap()
	if err != nil {
		return "", "", err
	}
	none := func(string) bool { return false }
	a := buildReapPlan(root, spec, minAgeDays, none, time.Now()).PlanID
	b := buildReapPlan(root, spec, minAgeDays, none, time.Now()).PlanID
	return a, b, nil
}

// CommitMessageTemplateForTest / CompareFileSetsForTest —— 提交面（`repo commit`）的两枚**纯函数**（测试用）。
// 桥的纪律同本文件顶部三条：只读形状、直接指真源、不另写副本。
func CommitMessageTemplateForTest(title, proposal, by, trace, criterion string, files []string) string {
	return commitMessageTemplate(title, proposal, by, trace, criterion, files)
}

func CompareFileSetsForTest(want, got []string) (bool, []string, []string) {
	return compareFileSets(want, got)
}

// ---- A4：影响面钩子分档（`family_impact_hook.go`）的**只读桥** ----
// 桥的纪律同本文件顶部三条：只读形状、**直接指真源**（`= impactHookDecide` 那一份）、不另写副本。

// ImpactHookFactsForTest 判档要的**全部事实**（纯数据 ⇒ 成对负控能直接喂「可找回 / 不可找回」）。
type ImpactHookFactsForTest struct {
	Class     string
	Rel       string
	BeforeSHA string
	Tracked   bool
	GitSHA    string
	Archive   string
	Outside   bool
}

// ImpactHookVerdictForTest 判档结果（三档 + 依据 + 三条判据现读 + 可找回证据两件）。
type ImpactHookVerdictForTest struct {
	Class    string
	Tier     string
	Why      string
	Recall   [3]string
	Evidence []string
}

// ImpactHookDecideForTest 判档的**唯一判定口**（与实现同一份纯函数）。
func ImpactHookDecideForTest(f ImpactHookFactsForTest) ImpactHookVerdictForTest {
	h := impactHookDecide(impactHookFacts{
		Class: f.Class, Rel: f.Rel, BeforeSHA: f.BeforeSHA,
		Tracked: f.Tracked, GitSHA: f.GitSHA, Archive: f.Archive, Outside: f.Outside,
	})
	return ImpactHookVerdictForTest{Class: h.Class, Tier: h.Tier, Why: h.Why, Recall: h.Recall, Evidence: h.Evidence}
}

// ImpactHookClassForTest 分类器（与实现同一份 `impactHookClassOf` —— 判「改 / 符号级删 / 件级 / 判不了」）。
func ImpactHookClassForTest(rel, before, after string) (string, string) {
	return impactHookClassOf(rel, []byte(before), []byte(after))
}

// ImpactDigestForTest 摘要指纹（与实现同一份 `impactDigestOf` —— 第三者可复算：同一段正文 ⇒ 同一枚）。
var ImpactDigestForTest = impactDigestOf

// ImpactTierNamesForTest 三档取值（**与实现同一份常量** —— 测试不另造第二套名字）。
func ImpactTierNamesForTest() []string {
	return []string{impactHookTierRecord, impactHookTierReport, impactHookTierBlock}
}

// ImpactClassNamesForTest 三类动作的取值（同上）。
func ImpactClassNamesForTest() []string {
	return []string{impactHookClassEdit, impactHookClassSymbol, impactHookClassFile, impactHookClassUnknown}
}

// ---- `A5` 落盘缓存与毫秒档（`family_impact_cache.go`）的**只读桥** ---------------------------------
// 桥的纪律同本文件顶部三条：只读形状、**直接指真源**、不另写副本。

// ImpactStateLayoutForTest 落点契约件的一格（具名类型）。
type ImpactStateLayoutForTest struct {
	Schema   string
	IndexDir string
	CacheDir string
	Key      []string
	NoAuto   []string
}

// ImpactStateLayoutOfForTest 读仓内落点契约件（与实现同一份 `impactStateLayoutOf`）。
func ImpactStateLayoutOfForTest(root string) (ImpactStateLayoutForTest, error) {
	l, err := impactStateLayoutOf(root)
	if err != nil {
		return ImpactStateLayoutForTest{}, err
	}
	return ImpactStateLayoutForTest{l.Schema, l.IndexDir, l.CacheDir, l.Key, l.NoAuto}, nil
}

// ImpactStateLayoutParseForTest 解一份**给定的**契约正文（负控：半份契约必须报错）。
func ImpactStateLayoutParseForTest(body []byte) (ImpactStateLayoutForTest, error) {
	l, err := impactStateLayoutParse(body)
	if err != nil {
		return ImpactStateLayoutForTest{}, err
	}
	return ImpactStateLayoutForTest{l.Schema, l.IndexDir, l.CacheDir, l.Key, l.NoAuto}, nil
}

// ImpactStateKeysEqualForTest 键三件的逐字对拍口（负控：换掉任一件必须判 false）。
func ImpactStateKeysEqualForTest(key []string) bool { return impactStateKeysEqual(key) }

// ImpactCacheFingerprintForTest 源指纹现算（`R42` 口径 · 本批加严形态）。
func ImpactCacheFingerprintForTest(root, scope string) (string, int, error) {
	return impactCacheFingerprint(root, scope)
}

// ImpactCachePathForTest 落盘件路径（目录名来自契约件 · 状态目录来自 `stateDirOf()`）。
func ImpactCachePathForTest(root, head, seq, caliber, scopeKey string) (string, error) {
	lay, err := impactStateLayoutOf(root)
	if err != nil {
		return "", err
	}
	return impactCacheFileFor(lay, head, seq, caliber, scopeKey), nil
}

// ImpactCacheIndexDirForTest 索引落点（同一份契约件）。
func ImpactCacheIndexDirForTest(root string) (string, error) {
	lay, err := impactStateLayoutOf(root)
	if err != nil {
		return "", err
	}
	return impactCacheIndexDir(lay), nil
}

// ImpactCacheLoadForTest 读一件落盘件并逐条判失效条件（**与命令同一份判定口**）。
func ImpactCacheLoadForTest(path, head, seq, caliber, fp string) (bool, string, float64) {
	_, hit, why, cost := impactCacheLoad(path, impactLayer{Seq: seq, HeadSHA: head, Caliber: caliber}, fp)
	return hit, why, float64(cost.Microseconds()) / 1000.0
}

// ImpactCacheProbeMSForTest 毫秒档现跑标定口（读落盘产物 n 次取最慢一次）。
func ImpactCacheProbeMSForTest(path string, n int) (float64, int, error) {
	return impactCacheProbeMS(path, n)
}

// ImpactCacheFilesUnderForTest 列出某目录下的落盘件（判「只增不删」用）。
func ImpactCacheFilesUnderForTest(dir string) ([]string, error) { return impactCacheFilesUnder(dir) }

// ImpactCacheableLayersForTest 进缓存的层（与实现同一份集合 · 测试不另抄一份）。
func ImpactCacheableLayersForTest() []string {
	out := []string{}
	for k := range impactCacheableLayers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ImpactMillisecondTierMSForTest 毫秒档判据值（§4.4：命中一次 ≤ 4 ms）。
func ImpactMillisecondTierMSForTest() float64 { return impactMillisecondTierMS }

// ImpactCacheSchemaForTest 落盘件的形状号（读到不认的形状号不许命中）。
func ImpactCacheSchemaForTest() string { return impactCacheSchema }

// ImpactCacheSourceForTest 读实现件源码（自检「没有删除动作 / 没有自动动作入口」用）。
func ImpactCacheSourceForTest() (string, error) {
	b, err := os.ReadFile("family_impact_cache.go")
	return string(b), err
}

// ImpactHookSourceForTest 读干跑钩子件源码（钉「干跑那一档传的是 `impactCacheOff`」）。
func ImpactHookSourceForTest() (string, error) {
	b, err := os.ReadFile("family_impact_hook.go")
	return string(b), err
}

// ImpactPullLayersModeOffForTest **给定的目标**在**缓存挡位=关**下跑六层（判「干跑那一档不碰缓存」）。
func ImpactPullLayersModeOffForTest(root, rel string) ([]string, error) {
	tgt, why := impactResolve(root, rel)
	if tgt == nil {
		return nil, fmt.Errorf("目标解析不到：%s", why)
	}
	notes := []string{}
	layers, _ := impactPullLayers(root, tgt, true, impactCacheOff)
	for _, l := range layers {
		notes = append(notes, l.Seq+"="+l.CacheNote)
	}
	return notes, nil
}

// ---- `B1` 步名真源与探针（`family_impact_steps.go`）-------------------------------------------
//
// 桥只开**只读**面：拉真源（`--list` 现跑）· 跑一条探针（`--emit-cmd`）· 把契约 id 投影到步名。
// 每个符号都**直接指真源**（不另写一份副本 —— 另写一份 = 测试测的是那份副本）。

// ImpactStepTableForTest 步名真源的一张**现跑快照**（含逐名探针：本跑步数 / 解析条数 / 名字不唯一
// 条数 / 负控退码）。测试据此判「步数不写常量」与「判据① 恰好 1 条」。
type ImpactStepTableForTest struct {
	Total      int
	Listed     int
	Status     string
	Detail     string
	Names      []string
	AmbigNames []string // 名字不唯一的步名（判据① 不成立 ⇒ 不可机检）
	Resolved   int
	Ambiguous  int
	Unresolv   int
	HeadSHA    string
	Dirty      string
	NegRC      int
	ListMs     float64
	ProbeMs    float64
}

// ImpactStepTableOfForTest 拉一张现跑快照（`withProbe=false` 就只跑 `--list`，不跑逐名探针）。
func ImpactStepTableOfForTest(root string, withProbe bool) ImpactStepTableForTest {
	t := impactStepTablePull(root)
	if withProbe {
		impactStepTableProbe(root, &t)
	}
	out := ImpactStepTableForTest{
		Total: t.Total, Listed: t.Listed, Status: t.Status, Detail: t.Detail,
		Resolved: t.Resolved, Ambiguous: t.Ambiguous, Unresolv: t.Unresolved,
		HeadSHA: t.HeadSHA, Dirty: t.Dirty, NegRC: t.NegRC,
		ListMs:  float64(t.ListCost.Microseconds()) / 1000.0,
		ProbeMs: float64(t.ProbeCost.Microseconds()) / 1000.0,
	}
	for _, r := range t.Rows {
		out.Names = append(out.Names, r.Name)
		if r.Hits != 1 {
			out.AmbigNames = append(out.AmbigNames, r.Name)
		}
	}
	return out
}

// ImpactStepProbeForTest 跑一条探针（`--emit-cmd '<原样传进来的那一个>'`）⇒ (输出, 退码)。
// 判据①②都靠它：真步名 ⇒ 1 条；子串 ⇒ 4 条；不存在的名字 ⇒ rc=2。
func ImpactStepProbeForTest(root, arg string) (string, int) {
	out, rc, _ := impactStepProbeCmd(root, arg)
	return out, rc
}

// ImpactStepNegativeProbeForTest 判据② 用的那个负控名字（与实现**同一份**常量 —— 不另抄一个）。
func ImpactStepNegativeProbeForTest() string { return impactStepNegativeProbe }

// ImpactStepViewForTest 投影的只读视图（含**人面那一格的渲染文本** ⇒ 测试判措辞不用另拼一份）。
type ImpactStepViewForTest struct {
	Status   string
	Reason   string
	Line     string // 「会红」行里**门步**那一格（`impactStepLineText` 的原样输出）
	Total    int
	Steps    []string
	ByID     map[string][]string
	Unjoined map[string]string
	Uncheck  []string
	Cands    int
	Probes   int
	NegRC    int
}

// ImpactStepProjectionForTest 把给定的契约 id 投影到步名（与 `zerg impact` 走的是同一个口）；
// `pull=false` = 默认档（**不拉真源** ⇒ 未机检）· `pull=true` = 按需档 `--all`（真拉 + 逐名探针）。
func ImpactStepProjectionForTest(root string, pull bool, ids ...string) ImpactStepViewForTest {
	p := impactStepProjectionOf(root, ids, pull)
	return ImpactStepViewForTest{
		Status: p.Status, Reason: p.Reason, Line: impactStepLineText(p),
		Total: p.Total, Steps: p.Steps, ByID: p.ByID, Unjoined: p.Unjoined,
		Uncheck: p.Uncheck, Cands: p.Cands, Probes: p.Probes, NegRC: p.NegRC,
	}
}

// ImpactStepNotRunLineForTest 干跑那一档那一格的渲染文本（**未机检** —— 与 `dev edit` 干跑同一份取值）。
func ImpactStepNotRunLineForTest() string { return impactStepLineText(impactStepProjectionNotRun()) }

// ---- `B2` 时间面 · 预测侧（`family_impact_timeface.go`）---------------------------------------
//
// 桥只开**只读**面：算一次预测面（真命令同一条路）· 拉一次「尺的标定」（起不起子进程随档位）·
// 读在册旧值那份清单。每个符号都**直接指真源**（不另写副本 —— 另写一份 = 测试测的是那份副本）。

// ImpactTimefaceViewForTest 预测面的只读视图（含**机读那一行原文** ⇒ 判「定序 · 无时钟」不用另拼）。
type ImpactTimefaceViewForTest struct {
	Target   string
	Tier     string
	HeadSHA  string
	Items    []string // `[why] red ← how` 形态（实现里那一行的原样）
	NotRun   []string
	EstCalib []string
	JSONLine string
	Calib    ImpactCalibViewForTest
}

// ImpactCalibViewForTest 「尺的标定」那一格的只读视图（现跑读数 + 成对闸 + 在册旧值）。
type ImpactCalibViewForTest struct {
	Status   string
	Reason   string
	ScriptK  bool
	Live     []string // `label=value` 形态（现跑）
	Old      []string // `label=value｜出处=…｜标定时刻=…` 形态（在册）
	Paired   []string // `--selftest` 里那两条 `(e)/(f)` 行
	Runs     []string // `flag rc=… 行数=… 耗时=…ms`
	CostMS   float64
	ProbeRan bool // 起过子进程没有（默认档必须 false）
}

// ImpactTimefaceOfForTest 算一次预测面 + 尺的标定（与 `zerg impact` **同一条路**）。
// `all=false` = 默认档（**不拉真源 · 不起子进程**）· `all=true` = 按需档 `--all`。
// 缓存挡位取`关`（`impactCacheOff`）⇒ 判据件不落缓存、两跑可比。
func ImpactTimefaceOfForTest(root, raw string, all bool) (ImpactTimefaceViewForTest, error) {
	out := ImpactTimefaceViewForTest{}
	tgt, why := impactResolve(root, raw)
	if tgt == nil {
		return out, fmt.Errorf("目标解析不到：%s", why)
	}
	cheap := !all
	layers, _ := impactPullLayers(root, tgt, cheap, impactCacheOff)
	proj := impactStepProjectionOf(root, impactStepIDsToJoin(tgt, layers), all)
	f := impactPredictFaceOf(tgt, layers, proj, cheap)
	out.Target, out.Tier, out.HeadSHA = f.Target, f.Tier, f.HeadSHA
	for _, it := range f.Items {
		out.Items = append(out.Items, fmt.Sprintf("[%s] %s ← %s", it.Why, it.Red, it.How))
	}
	out.NotRun, out.EstCalib = f.NotRun, f.EstCalib
	out.JSONLine = impactPredictJSON(f)
	out.Calib = impactCalibViewForTest(impactCalibrationOf(root, all))
	return out, nil
}

// ImpactCalibrationOfForTest 只拉「尺的标定」那一格（合成尺夹具上判「不齐 ⇒ 取不到」用）。
func ImpactCalibrationOfForTest(root string, pull bool) ImpactCalibViewForTest {
	return impactCalibViewForTest(impactCalibrationOf(root, pull))
}

func impactCalibViewForTest(c impactCalib) ImpactCalibViewForTest {
	v := ImpactCalibViewForTest{
		Status: c.Status, Reason: c.Reason, ScriptK: c.ScriptK,
		CostMS:   float64(c.Cost.Microseconds()) / 1000.0,
		ProbeRan: len(c.Runs) > 0,
	}
	for _, rd := range c.Live {
		v.Live = append(v.Live, rd.Label+"="+rd.Value)
	}
	for _, o := range c.Old {
		v.Old = append(v.Old, o.Label+"="+o.Value+"｜出处="+o.Source+"｜标定时刻="+o.At)
	}
	for _, r := range c.Runs {
		v.Runs = append(v.Runs, fmt.Sprintf("%s rc=%d 行数=%d 耗时=%.1fms", r.Name, r.RC, r.Lines,
			float64(r.Cost.Microseconds())/1000.0))
		v.Paired = append(v.Paired, r.Paired...)
	}
	return v
}

// ImpactCalibScriptRelForTest 尺的件路径（与实现**同一份**常量 —— 不另抄一个）。
func ImpactCalibScriptRelForTest() string { return impactCalibScriptRel }

// ImpactTimefaceSourceForTest 读实现件源码（自检「不落盘 / 不设阈值 / 不抢 `B3`」用）。
func ImpactTimefaceSourceForTest() (string, error) {
	b, err := os.ReadFile("family_impact_timeface.go")
	return string(b), err
}

// ---- `B2` 契约面（判据①–③）的只读桥 ----------------------------------------------------------

// ImpactContractEntryForTest 一条**目录项**（判据① 的四字段）。
type ImpactContractEntryForTest struct{ ID, ChangeClass, Gate, ChangeNote string }

// ImpactContractViewForTest 契约面现读视图（判据① 的两半判定 + 判据③ 的两枚指纹 + 两份文本）。
//
// `Block` = stderr 块原样文本（判措辞不用另拼一份）· `JSONLine` = 机读行（同一份取值）。
type ImpactContractViewForTest struct {
	Status    string
	Reason    string
	Lines     int
	Rows      []ImpactContractEntryForTest
	Missing   []string
	FourOK    bool
	IDEqual   bool
	SelfN     int
	CatN      int
	OnlyInReg []string
	OnlyInCat []string
	Head      string
	Rev       string
	Parent    string
	FpReason  string
	Block     string
	JSONLine  string
}

// ImpactContractFaceForTest 现读契约面（走的全是实现里那几个口 —— 测试不另算一遍）。
func ImpactContractFaceForTest(root string) ImpactContractViewForTest {
	cat, audit, head, rev, parent, why := impactContractFace(root)
	v := ImpactContractViewForTest{
		Status: cat.Status, Reason: cat.Reason, Lines: cat.Lines,
		Missing:   append([]string{}, audit.Missing...),
		FourOK:    audit.FourOK,
		IDEqual:   audit.IDEqual,
		SelfN:     audit.SelfN,
		CatN:      audit.CatN,
		OnlyInReg: append([]string{}, audit.OnlyInReg...),
		OnlyInCat: append([]string{}, audit.OnlyInCat...),
		Head:      head, Rev: rev, Parent: parent, FpReason: why,
	}
	for _, r := range cat.Rows {
		v.Rows = append(v.Rows, ImpactContractEntryForTest{ID: r.ID, ChangeClass: r.ChangeClass, Gate: r.Gate, ChangeNote: r.ChangeNote})
	}
	var b strings.Builder
	emitImpactContractBlock(&b, root, cat, audit, head, rev, parent, why)
	v.Block = b.String()
	v.JSONLine = impactContractSignal(cat, audit, head, rev, parent)
	return v
}

// ImpactContractFourFieldsForTest 判据① 的四字段（与实现**同一份**常量 —— 不另抄一份）。
func ImpactContractFourFieldsForTest() []string {
	return append([]string{}, impactContractFourFields...)
}

// ImpactContractSixFieldsForTest 判据③「比什么」的每条 6 字段。
func ImpactContractSixFieldsForTest() []string { return append([]string{}, impactContractSixFields...) }

// ImpactContractCompatLevelForTest 兼容级别那一格的唯一取值（判据②）。
func ImpactContractCompatLevelForTest() string { return impactContractCompatLevel }

// ImpactContractRegistryGateRelForTest `registry.json` 自陈的门件路径（`R43` 的那一格）。
func ImpactContractRegistryGateRelForTest() string { return impactContractRegistryGateRel }

// ImpactRedLineForTest 「会红」那一行的渲染文本（判据② 的成对负控要按**同一份口径**判红绿：
// 用的就是 `cmdImpact` 里那一个 `impactRedLine` 口 · 缓存挡位取关 ⇒ 两跑可比）。
func ImpactRedLineForTest(root, raw string) (string, error) {
	tgt, why := impactResolve(root, raw)
	if tgt == nil {
		return "", fmt.Errorf("目标解析不到：%s", why)
	}
	layers, _ := impactPullLayers(root, tgt, true, impactCacheOff)
	proj := impactStepProjectionOf(root, impactStepIDsToJoin(tgt, layers), false)
	return impactRedLine(layers, true, proj), nil
}

// ImpactContractSourceForTest 读实现件源码（源件自检：不落实现 / 不落新快照件 / 只读）。
func ImpactContractSourceForTest() (string, error) {
	b, err := os.ReadFile("family_impact_contract.go")
	return string(b), err
}

// ---- B3：实测回填闭环（`family_impact_backfill.go`）的**只读桥** ------------------------------
//
// 桥的纪律同本文件顶部三条：只读形状、**直接指真源**（每个符号都指那一份实现）、不另写副本。
// 这里唯一「写」的一格是 `ImpactBackfillAppendForTest` —— 它指的**就是**实现里那个写口（判据④ 的唯一出口），
// 为的是让「缺任一 ⇒ 不许写」能在**外部测试包**里被判（测试只往 `t.TempDir()` 里写）。

// ImpactBackfillCountsOfForTest 三个数的**唯一判定口**（与实现同一份 `impactBackfillCountsOf`）。
func ImpactBackfillCountsOfForTest(predict, fail, blocked, report []string) (hit, miss, fa int, exclB, exclR []string) {
	c, ex := impactBackfillCountsOf(predict, fail, blocked, report)
	return c.Hit, c.Miss, c.FalseAlarm, ex.Blocked, ex.Report
}

// ImpactBackfillKeysForTest 十键（**从契约件现读** —— 不另抄一份键表）。
func ImpactBackfillKeysForTest(root string) ([]string, error) {
	c, why := impactBackfillContractOf(root)
	if why != "" {
		return nil, fmt.Errorf("%s", why)
	}
	return append([]string{}, c.RequiredKeys...), nil
}

// ImpactBackfillContractTextForTest 契约件的**逐字原文**（判据⑤ 的「不可机检」与「不自设阈值」要落在原文上判）。
func ImpactBackfillContractTextForTest(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactBackfillContractRel)))
	return string(b), err
}

// ImpactBackfillRecordJSONForTest 拼一枚回填件的行（JSON）；`predict`/`fail`/`blocked` 传 `nil` ⇒ 该格是 `null`
// （判据④ 的「不许缺」判的就是这个 —— `[]` 与「缺」是两件事）。
func ImpactBackfillRecordJSONForTest(head, target string, predict, fail, blocked, report []string,
	gateRunID, at string) (string, error) {
	rec := impactBackfillRecord{
		HeadSHA: head, Target: target, PredictRed: predict, ActualFail: fail, ActualBlocked: blocked,
		GateRunID: gateRunID, At: at, ActualReport: report,
	}
	b, err := json.Marshal(rec)
	return string(b), err
}

// ImpactBackfillAppendForTest 判据④ 的**唯一写口**（与实现同一份）：缺任一 ⇒ 只回缺失清单、**不写**。
func ImpactBackfillAppendForTest(path, recJSON string) ([]string, error) {
	var rec impactBackfillRecord
	if err := json.Unmarshal([]byte(recJSON), &rec); err != nil {
		return nil, err
	}
	return impactBackfillAppendIfComplete(path, rec)
}

// ImpactBackfillGateReadForTest 读一次结果表（只读）：返回四档步名与 `gate_run_id` + 取不到的原因。
func ImpactBackfillGateReadForTest(root, arg string) (status, reason, runID, tableSHA string,
	fail, blocked, report, pass []string) {
	r := impactGateResultsRead(root, arg)
	return r.Status, r.Reason, r.RunID, r.TableSHA, r.Fail, r.Blocked, r.Report, r.Pass
}

// ImpactBackfillTruthForTest 真源现跑（两档）：步数 + 步名（不写常量 ⇒ 测试也读真源）。
func ImpactBackfillTruthForTest(root string) (fullTotal, fastTotal int, fullSteps, fastSteps []string, status, reason string) {
	t := impactBackfillTruthPull(root)
	return t.FullTotal, t.FastTotal, t.FullSteps, t.FastSteps, t.Status, t.Reason
}

// ImpactBackfillAlignForTest 齐不齐判定口（结果表 ⇔ 真源两档）。
func ImpactBackfillAlignForTest(root, arg string) (tier, why string) {
	r := impactGateResultsRead(root, arg)
	if r.Status != "取值" {
		return "", r.Reason
	}
	return impactBackfillAlign(r, impactBackfillTruthPull(root))
}

// ImpactBackfillHistoryForTest 回填件的**只读**汇总（累积那一格）。
func ImpactBackfillHistoryForTest(path string) (status, reason string, lines, broken, hit, miss, fa int) {
	h := impactBackfillHistoryOf(path)
	return h.Status, h.Reason, h.Lines, h.Broken, h.Hit, h.Miss, h.FalseAlarm
}

// ImpactBackfillScanForTest 静态自证那一半（回填件不作算法输入）。
func ImpactBackfillScanForTest(root, file string) (files int, hits, unread []string) {
	s := impactBackfillReadersScan(root, file)
	return s.Files, s.Hits, s.Unread
}

// ImpactBackfillFlagGivenForTest 「`--gate-results` 给没给」那一格（收尾那一格的用法错靠它判）。
func ImpactBackfillFlagGivenForTest(orig []string) bool { return impactBackfillFlagGiven(orig) }

// ImpactBackfillPathForTest 回填件的落点（读契约件里的 `file`）。
func ImpactBackfillPathForTest(root string) (string, error) {
	c, why := impactBackfillContractOf(root)
	if why != "" {
		return "", fmt.Errorf("%s", why)
	}
	return impactBackfillPath(c), nil
}

// ImpactBackfillSourceForTest 读实现件源码（源件自检用）。
func ImpactBackfillSourceForTest() (string, error) {
	b, err := os.ReadFile("family_impact_backfill.go")
	return string(b), err
}

// ImpactSlotLineForTest 现成混淆矩阵槽那一行（**逐字**）—— 判据⑤ 与「不自设阈值」两格都引它。
func ImpactSlotLineForTest(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "check-slice.py"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 7 {
		return "", fmt.Errorf("check-slice.py 不足 7 行")
	}
	return lines[6], nil
}
