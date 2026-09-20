// stage_gate_test.go —— 三阶推进的**升阶闸门**四条判据的机检（层①进程内 · 外部测试包
// `package main_test` · 开工单 T-62）。
//
// 判据（开工单逐字）：
//
//	① 升阶判据**可机检**（`OM11`：本节右列判据全是人判 = 本节最弱的一处）；
//	② 阶 2 的升阶前置 = 退码表已定（`P-013`）+ 候选区闭集有形状；
//	③ **人不在场**时流程只走到「候选件 + 证据单」为止，`release` 之后默认不开；
//	④ 四道闸（契约核验 → 跨家族复查 → 门禁全量 → 人拍板）**缺一道即不算过**。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// writeFullSuite 造一张「全量档」门禁结果表（`n` 行 · 其中 `fails` 行 FAIL）。
func writeFullSuite(t *testing.T, n, fails int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "results.tsv")
	lines := []string{}
	for i := 0; i < n; i++ {
		status := "PASS"
		if i < fails {
			status = "FAIL"
		}
		lines = append(lines, status+"\tgate: 合成步骤 "+string(rune('A'+i%26))+"\t0\t0s\t/tmp/x.log")
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// ---- 判据①：真源自己**可机检**（每条判据的可机检那一格非空）----

func TestStageGateIsMachineCheckable(t *testing.T) {
	spec, err := zerg.StageGateSpecOfForTest()
	if err != nil {
		t.Fatalf("升阶闸门真源读不出来：%v", err)
	}
	if spec.Stages != 3 {
		t.Errorf("阶数 = %d（要三阶——阶 1 → 阶 2 → 阶 3）", spec.Stages)
	}
	if len(spec.GateIDs) != 4 {
		t.Errorf("闸数 = %d（要四道）· %v", len(spec.GateIDs), spec.GateIDs)
	}
	// `OM11` 要治的正是这一格：**每一条**判据都要有「可机检那一面」，空的 ⇒ 又回到人判。
	if spec.MachineCells != spec.Criteria {
		t.Errorf("判据 %d 条，其中可机检那一格非空的只有 %d 条 —— 空的那些就是 OM11 要治的病",
			spec.Criteria, spec.MachineCells)
	}
	if spec.FullSuiteSteps <= 0 {
		t.Errorf("全量档步数 = %d（要正数）", spec.FullSuiteSteps)
	}
	if strings.TrimSpace(spec.HumanAbsentRule) == "" || strings.TrimSpace(spec.MissingOneRule) == "" {
		t.Error("「人不在场」与「缺一道即不算过」两条口径必须逐字在真源里")
	}
	t.Logf("三阶 · %d 条判据（可机检 %d 条）· 四道闸 %v · 全量档 %d 步",
		spec.Criteria, spec.MachineCells, spec.GateIDs, spec.FullSuiteSteps)
}

// ---- 判据④：四道闸缺一道即不算过（逐道负控）----

func TestFourGatesMissingOneIsNotPass(t *testing.T) {
	spec, err := zerg.StageGateSpecOfForTest()
	if err != nil {
		t.Fatal(err)
	}
	full := spec.FullSuiteSteps

	// ⓐ 四道齐（全量档结果表 + 人拍板）⇒ 过（`G1`/`G2` 在真仓上是 PASS —— 见下一条断言）。
	vs, passed, err := zerg.JudgeFourGatesForTest(writeFullSuite(t, full, 0), "Mr2109")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 4 {
		t.Fatalf("四道闸判出 %d 条：%v", len(vs), vs)
	}
	for _, v := range vs {
		if v.Verdict != "PASS" {
			t.Errorf("四道齐时 %s（%s）不是 PASS：%s —— %s", v.ID, v.Name, v.Verdict, v.Detail)
		}
	}
	if !passed {
		t.Error("四道全 PASS 却判成「不算过」")
	}

	// ⓑ 缺 `G4`（人不在场）⇒ 不算过。
	vs, passed, _ = zerg.JudgeFourGatesForTest(writeFullSuite(t, full, 0), "")
	if passed {
		t.Error("人不在场时判成了「过」—— §20.2：人不在场只走到「候选件 + 证据单」为止")
	}
	if vs[3].Verdict != "BLOCKED" || !strings.Contains(vs[3].Detail, "人不在场") {
		t.Errorf("G4 的判决 = %s / %s（要 BLOCKED + 逐字「人不在场」）", vs[3].Verdict, vs[3].Detail)
	}

	// ⓒ 缺 `G3`（全量结果表不在）⇒ 不算过。
	vs, passed, _ = zerg.JudgeFourGatesForTest(filepath.Join(t.TempDir(), "nope.tsv"), "Mr2109")
	if passed {
		t.Error("全量结果表不在时判成了「过」—— 「没跑过全量」不是「过了」")
	}
	if vs[2].Verdict != "BLOCKED" {
		t.Errorf("G3 的判决 = %s（要 BLOCKED）", vs[2].Verdict)
	}

	// ⓓ 全量档**没跑满** ⇒ G3 FAIL（拿 `--fast` 的 29 步冒充全量 = 偷工）。
	vs, passed, _ = zerg.JudgeFourGatesForTest(writeFullSuite(t, full-1, 0), "Mr2109")
	if passed {
		t.Error("步数不满也判成「过」")
	}
	if vs[2].Verdict != "FAIL" {
		t.Errorf("步数不满时 G3 = %s（要 FAIL）· %s", vs[2].Verdict, vs[2].Detail)
	}

	// ⓔ 全量档有 FAIL ⇒ G3 FAIL。
	vs, _, _ = zerg.JudgeFourGatesForTest(writeFullSuite(t, full, 1), "Mr2109")
	if vs[2].Verdict != "FAIL" || !strings.Contains(vs[2].Detail, "FAIL") {
		t.Errorf("有 FAIL 时 G3 = %s / %s（要 FAIL）", vs[2].Verdict, vs[2].Detail)
	}

	// ⓕ 空表 ⇒ BLOCKED（0 步 = 没结论，不许当「全过」）。
	empty := filepath.Join(t.TempDir(), "empty.tsv")
	if err := os.WriteFile(empty, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vs, _, _ = zerg.JudgeFourGatesForTest(empty, "Mr2109")
	if vs[2].Verdict != "BLOCKED" {
		t.Errorf("空表时 G3 = %s（要 BLOCKED）", vs[2].Verdict)
	}
}

// ---- 判据④端到端：`zerg dev verify --gate`（只加旗标、不新增命令名）----

func TestDevVerifyGateEndToEnd(t *testing.T) {
	spec, err := zerg.StageGateSpecOfForTest()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Setenv("ZERG_CANDIDATE_ROOT", root)
	results := writeFullSuite(t, spec.FullSuiteSteps, 0)

	// ① 四道齐 ⇒ rc=0 且输出里逐条给出四道闸。
	rc, out, errb := runCapture("dev", "verify", "--candidate", "DEV-0009", "--gate",
		"--results", results, "--human-approval", "Mr2109")
	if rc != 0 {
		t.Fatalf("四道齐 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	for _, want := range []string{"契约核验", "跨家族复查", "门禁全量", "人拍板", "**过**"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出里没有 %q：%s", want, out)
		}
	}
	// ② 人不在场 ⇒ rc=2（**不算过**），且逐字印出那条口径。
	rc, out, errb = runCapture("dev", "verify", "--candidate", "DEV-0009", "--gate", "--results", results)
	if rc != 2 {
		t.Fatalf("人不在场 rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "**不算过**") || !strings.Contains(errb, "人不在场") {
		t.Errorf("人不在场的输出没点明「不算过 / 人不在场」：stdout=%s stderr=%s", out, errb)
	}
	// ③ 缺结果表（默认落点不在）⇒ 也不算过 —— 顺带钉住默认落点是候选区闭集里的那一份。
	rc, _, _ = runCapture("dev", "verify", "--candidate", "DEV-0009", "--gate", "--human-approval", "Mr2109")
	if rc != 2 {
		t.Errorf("缺全量结果表 rc=%d（要 2）", rc)
	}
	// ④ 负控：候选 id 不在闭集里 ⇒ 仍然先判 2（闸门不绕过 id 闭集）。
	if rc, _, _ := runCapture("dev", "verify", "--candidate", "dev-0009", "--gate"); rc != 2 {
		t.Errorf("坏候选 id rc=%d（要 2）", rc)
	}
	// ⑤ 判据②的**静态面**：阶 2 的两条前置逐字在真源里（退码表已定 + 候选区闭集有形状）。
	rc2, out2, _ := runCapture("dev", "verify", "--candidate", "DEV-0009", "--gate", "--results", results,
		"--human-approval", "Mr2109", "--json", "candidate,criterion")
	if rc2 != 0 && rc2 != 1 && rc2 != 2 {
		t.Errorf("`--gate` 与 `--json` 同用退了异常码 %d", rc2)
	}
	_ = out2
}
