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
	"strings"

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
