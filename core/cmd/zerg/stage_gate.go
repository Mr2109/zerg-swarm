// stage_gate.go —— 三阶推进的**升阶闸门**（§20.4 三阶表 · §20.7 `OM11` · §十二 `P-S6-1` ·
// §20.2 原则①④ · 开工单 T-62）。
//
// `OM11` 登记的病（逐字）：「本节右列判据**全是人判**」⇒ 本节最弱的一处。本件的落法：
//
//	① 判据**可机检**：每条判据的「可机检那一面」落在真源 `core/internal/contract/stage-gate.json`
//	   的 `stages[].criteria[].machine` 里；`StageGate()` 解真源时**逐条查**那一格非空
//	   （空的 ⇒ 报错 ⇒ 不给结论）—— 「又回到人判」这件事本身就是判据。
//	② 升阶的四道闸（契约核验 → 跨家族复查 → 门禁全量 → 人拍板）**缺一道即不算过**：
//	   本件的 `judgeFourGates` 是唯一判定口，每道闸给 PASS / FAIL / BLOCKED 三值之一；
//	   只要有一道不是 PASS ⇒ 整体**不算过**（退码 2 = 不给结论 —— 「过了三道」不是「过了」，
//	   也不按「错」计：没结论 ≠ 错）。
//	③ **人不在场**时只走到「候选件 + 证据单」为止：`G4` 拿不到人拍板 ⇒ BLOCKED ⇒ 不算过，
//	   `release` 之后默认不开。
//	④ **不新增命令名**：闸门挂在 `zerg dev verify --gate`（同一族、同一判据面）——
//	   照 §17.4 第 4 条 `--candidate` 的同款口径（只加一枚旗标）。
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// stageGateVerdict —— 一道闸的判决（三值 + 依据 + 落点）。
type stageGateVerdict struct {
	ID      string
	Name    string
	Verdict string // PASS / FAIL / BLOCKED
	Detail  string
}

// devFamilyClosureForGate —— §17.4 全表那七条（`G2` 跨家族复查的对拍真值）。
// 与 `dev_proposal_test.go` 的 `devFamilyClosure` **同一份清单**的两处写法是**故意**的：
// 测试那份是「闭集不许悄悄长大」，这份是「闸门要知道该长成什么样」—— 两处不一致时 `G2` 红。
var devFamilyClosureForGate = []string{"dev proposal", "dev release", "dev rollback", "dev receipt",
	"dev build", "dev test", "dev verify"}

// judgeFourGates —— **唯一判定口**：四道闸逐条判，缺一道即不算过。
//
// `evidencePath` 给 `G3` 读（候选区的全量门禁结果表）；`human` 给 `G4` 读（人拍板的名字）。
func judgeFourGates(spec *contract.StageGateSpec, root, evidencePath, human string) []stageGateVerdict {
	out := []stageGateVerdict{}
	for _, g := range spec.FourGates {
		switch g.ID {
		case "G1":
			out = append(out, judgeG1(g))
		case "G2":
			out = append(out, judgeG2(g))
		case "G3":
			out = append(out, judgeG3(g, spec, evidencePath))
		case "G4":
			out = append(out, judgeG4(g, human))
		default:
			// 真源里出现一道**本判定口不认**的闸 ⇒ 不给结论（「新加的闸没人判」比少一道更坏）。
			out = append(out, stageGateVerdict{g.ID, g.Name, "BLOCKED",
				"判定口不认这道闸（真源加了新闸但没人实现它）⇒ 不给结论"})
		}
	}
	_ = root
	return out
}

// judgeG1 —— 契约核验：四个真源都解得出来。
func judgeG1(g contract.StageGateDef) stageGateVerdict {
	names := []string{}
	bad := []string{}
	if _, err := contract.DevCandidate(); err != nil {
		bad = append(bad, "dev-candidate.json: "+err.Error())
	} else {
		names = append(names, "dev-candidate.json")
	}
	if _, err := contract.AIBoundary(); err != nil {
		bad = append(bad, "ai-boundary.json: "+err.Error())
	} else {
		names = append(names, "ai-boundary.json")
	}
	if _, err := contract.IntentPlan(); err != nil {
		bad = append(bad, "intent-plan.json: "+err.Error())
	} else {
		names = append(names, "intent-plan.json")
	}
	if _, err := contract.Receipt(); err != nil {
		bad = append(bad, "receipt.json: "+err.Error())
	} else {
		names = append(names, "receipt.json")
	}
	if len(bad) > 0 {
		return stageGateVerdict{g.ID, g.Name, "FAIL", "真源解得出的：[" + strings.Join(names, ", ") + "] · 坏的：" + strings.Join(bad, " | ")}
	}
	return stageGateVerdict{g.ID, g.Name, "PASS", "四个真源都成形：[" + strings.Join(names, ", ") + "]"}
}

// judgeG2 —— 跨家族复查：`dev` 族闭集 == §17.4 七条 + `SD10-b` 规范入口说明在。
func judgeG2(g contract.StageGateDef) stageGateVerdict {
	got := []string{}
	for _, p := range commandPaths() {
		if p == "dev" || strings.HasPrefix(p, "dev ") {
			got = append(got, p)
		}
	}
	sort.Strings(got)
	want := append([]string{}, devFamilyClosureForGate...)
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		return stageGateVerdict{g.ID, g.Name, "FAIL",
			fmt.Sprintf("`dev` 族 = %v ≠ §17.4 七条 %v", got, want)}
	}
	dc, err := contract.DevCandidate()
	if err != nil {
		return stageGateVerdict{g.ID, g.Name, "BLOCKED", "候选区真源读不出来 ⇒ 不给结论"}
	}
	if strings.TrimSpace(dc.CanonicalEntry["rule"]) == "" || strings.TrimSpace(dc.CanonicalEntry["build"]) == "" {
		return stageGateVerdict{g.ID, g.Name, "FAIL", "`SD10-b`「谁是规范入口」的说明不在真源里"}
	}
	return stageGateVerdict{g.ID, g.Name, "PASS", fmt.Sprintf("`dev` 族 %d 条与 §17.4 逐条对上 · `SD10-b` 说明在", len(got))}
}

// judgeG3 —— 门禁全量：候选区的全量结果表步数 == 真源的全量步数 且 FAIL 0。
//
// 缺表 / 0 行 ⇒ **BLOCKED**（不给结论 —— 「没跑过全量」不是「过了」）。
func judgeG3(g contract.StageGateDef, spec *contract.StageGateSpec, results string) stageGateVerdict {
	body, err := os.ReadFile(results)
	if err != nil {
		return stageGateVerdict{g.ID, g.Name, "BLOCKED",
			fmt.Sprintf("读不到候选区的全量门禁结果表 %s（%v）⇒ 不给结论", results, err)}
	}
	steps, fails := 0, 0
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		steps++
		if strings.HasPrefix(line, "FAIL") {
			fails++
		}
	}
	if steps == 0 {
		return stageGateVerdict{g.ID, g.Name, "BLOCKED", "结果表是空的（0 步）⇒ 不给结论"}
	}
	if steps != spec.FullSuiteSteps {
		return stageGateVerdict{g.ID, g.Name, "FAIL",
			fmt.Sprintf("步数 %d ≠ 全量档 %d（走的是不是全量档？`--fast` 不算）", steps, spec.FullSuiteSteps)}
	}
	if fails != 0 {
		return stageGateVerdict{g.ID, g.Name, "FAIL", fmt.Sprintf("全量档有 %d 个 FAIL", fails)}
	}
	return stageGateVerdict{g.ID, g.Name, "PASS", fmt.Sprintf("全量档 %d 步 · FAIL 0", steps)}
}

// judgeG4 —— 人拍板：`--human-approval <名>` 非空。**人不在场 ⇒ 不给结论**（不算过）。
func judgeG4(g contract.StageGateDef, human string) stageGateVerdict {
	if strings.TrimSpace(human) == "" {
		return stageGateVerdict{g.ID, g.Name, "BLOCKED",
			"没有人的拍板（缺 `--human-approval <名>`）⇒ **人不在场**：流程只走到「候选件 + 证据单」为止，`release` 之后默认不开"}
	}
	return stageGateVerdict{g.ID, g.Name, "PASS", "人拍板：" + strings.TrimSpace(human)}
}

// gatesPassed —— 四道闸**全 PASS** 才算过（缺一道即不算过）。
func gatesPassed(vs []stageGateVerdict, spec *contract.StageGateSpec) bool {
	if len(vs) != len(spec.FourGates) {
		return false
	}
	for _, v := range vs {
		if v.Verdict != "PASS" {
			return false
		}
	}
	return true
}

// cmdDevVerifyGate —— `zerg dev verify --gate`：印四道闸的逐条判决并给结论。
//
// 退码：全过 ⇒ 0 · 有 FAIL ⇒ 1 · 缺闸（BLOCKED）或有人不在场 ⇒ 2（**不算过**，也不按错计）。
func cmdDevVerifyGate(inv *invocation, stdout, stderr io.Writer, id string) int {
	spec, err := contract.StageGate()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 升阶闸门真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked
	}
	root := candidateRootDir()
	results := strings.TrimSpace(inv.flagVal("--results"))
	if results == "" && root != "" {
		results = evidenceSheetPath(root, id) // 证据单的同胞：候选区的全量结果表
		results = strings.TrimSuffix(results, "evidence.json") + "gate/results.tsv"
	}
	vs := judgeFourGates(spec, root, results, strings.TrimSpace(inv.flagVal("--human-approval")))
	fmt.Fprintf(stdout, "升阶闸门（§20.4 · §20.7 OM11 · 开工单 T-62）· 候选 %s\n", id)
	fmt.Fprintf(stdout, "  三阶闭集：%s\n", stageNames(spec))
	for _, v := range vs {
		fmt.Fprintf(stdout, "  %-8s %s：%s\n", v.Verdict, v.Name, v.Detail)
	}
	passed := gatesPassed(vs, spec)
	fmt.Fprintf(stdout, "结论      : %s\n", boolText(passed, "**过**（四道闸全 PASS）", "**不算过**"))
	fmt.Fprintf(stderr, "%s\n", spec.MissingOneRule)
	fmt.Fprintf(stderr, "%s\n", spec.HumanAbsentRule)
	rc := exitOK
	for _, v := range vs {
		switch v.Verdict {
		case "FAIL":
			rc = exitFail
		case "BLOCKED":
			if rc != exitFail {
				rc = exitUsage
			}
		}
	}
	return rc
}

func stageNames(spec *contract.StageGateSpec) string {
	parts := []string{}
	for _, s := range spec.Stages {
		parts = append(parts, s.ID+"="+s.Name)
	}
	return strings.Join(parts, " · ")
}

func boolText(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// commandPaths —— 命令树全部路径（`G2` 用；不另抄命令树）。
func commandPaths() []string {
	out := make([]string, 0, len(commands))
	for _, c := range commands {
		out = append(out, strings.Join(c.path, " "))
	}
	return out
}
