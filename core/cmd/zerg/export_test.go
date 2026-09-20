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
	"os"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
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
