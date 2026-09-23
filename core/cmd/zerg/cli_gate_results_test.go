// cli_gate_results_test.go —— `zerg gate results --last [--json]` 的判据（缺口 `Q-111` / `B-8`）。
//
// 为什么落在这里（不是真二进制那一层）：本批**不重建 `bin/`**（换件要另走七步）⇒ 新面的棘轮
// 必须落进程内 `RunForTest`（读的是**当前源码**）。
//
// 钉住六件（全走**合成趟** —— 不碰真门禁产物、不跑任何步骤）：
//
//	① 人面四数 + 步数 + 总退码（口径逐字对齐脚本 `_exit_rc`）· 非 PASS 逐条带日志路径；
//	② `--json` 六键包封 + `items` 逐条含步名/状态/退码/耗时/日志路径 —— 且**逐条值不同**
//	   （负控：专治 `help export --json` 那种「一坨同一个值」）；
//	③ 总退码三档：有 FAIL ⇒ 1 · 只有 BLOCKED ⇒ 2 · 全 PASS ⇒ 0（与脚本 `_exit_rc` 同一条）；
//	④ 三态齐：能（0）· 不给结论（8：读不到/空表）· 错（2：两个来源 / 字段表外 / 多余位置参数）；
//	⑤ 结果表里有**认不出状态**的行 ⇒ 退 1 + `warnings` 非空（判不出不许当绿）；
//	⑥ **只读**：跑前/跑后整个趟目录逐件 `sha256` + 清单逐字相同（不删不改任何日志）。
package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gateResultsTSVName —— 结果表文件名（逐字取自脚本 `results=${outdir}/results.tsv`）。
const gateResultsTSVName = "results.tsv"

// writeGateRun —— 造一趟合成产物：`results.tsv` 逐行照脚本 `run_step` 的落盘式
// （`状态 \t 步名 \t rc \t 耗时 \t 日志路径`）+ 每行一个真日志文件。
func writeGateRun(t *testing.T, rows [][5]string) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	for i, r := range rows {
		log := filepath.Join(dir, fmt.Sprintf("%02d-step%d.log", i, i))
		if err := os.WriteFile(log, []byte("合成日志 "+r[0]+" "+r[1]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3], log)
	}
	if err := os.WriteFile(filepath.Join(dir, gateResultsTSVName), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// resultsRepo —— `gate` 族任何一条动作都要的最小仓（族规：门禁最小入口缺席即硬错 ——
// 只读动作也不放松这一条：`gate results` 照样要看到 `scripts/gates/precommit-gates.sh`）。
func resultsRepo(t *testing.T) string {
	t.Helper()
	return syntheticRepo(t, showGateScript)
}

var gateRunFourStatus = [][5]string{
	{"PASS", "gofmt -l core", "0", "0s", ""},
	{"PASS", "core: go vet ./...", "0", "3s", ""},
	{"FAIL", "core: go test ./... -count=1", "1", "42s", ""},
	{"BLOCKED", "docs: meta --scope formal", "2", "0s", ""},
	{"REPORT", "docs: 悬空引用", "1", "1s", ""},
}

// ① 人面：四数 + 步数 + 总退码 + 非 PASS 逐条（日志路径可点）。
func TestGateResultsHumanFourNumbers(t *testing.T) {
	dir := writeGateRun(t, gateRunFourStatus)
	rc, out, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir)
	if rc != 0 {
		t.Fatalf("读通那一趟 ⇒ 退 0（那一趟自身的红绿在数据里，不在本命令退码里），得到 %d · stderr=%s", rc, errb)
	}
	want := []string{"状态计数：PASS 2", "FAIL 1", "BLOCKED(不给结论) 1", "REPORT(只报告) 1", "步数 5", "总退码 1"}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("人面缺 %q：\n%s", w, out)
		}
	}
	if !strings.Contains(out, "core: go test ./... -count=1") || !strings.Contains(out, dir) {
		t.Errorf("非 PASS 逐条要给步名与日志路径：\n%s", out)
	}
}

// ② `--json`：六键 + `items` 逐条五格 + **逐条值不同**（负控）。
func TestGateResultsJSONItemsArePerStep(t *testing.T) {
	dir := writeGateRun(t, gateRunFourStatus)
	rc, out, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir, "--json", "step,status,rc,secs,log")
	if rc != 0 {
		t.Fatalf("退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("不是对象：%v · %q", err, out)
	}
	for _, k := range []string{"schema", "kind", "items", "meta", "warnings", "truncated"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("包封缺键 %q", k)
		}
	}
	var kind string
	_ = json.Unmarshal(doc["kind"], &kind)
	if kind != "GateResults" {
		t.Errorf("kind = %q（要 GateResults）", kind)
	}
	items := envelopeItems(t, out)
	if len(items) != 5 {
		t.Fatalf("items 要 5 条（一步一条），得到 %d：%s", len(items), out)
	}
	// 逐条五格齐 + 值取自各自那一行。
	seen := map[string]bool{}
	for i, it := range items {
		for _, k := range []string{"step", "status", "rc", "secs", "log"} {
			if _, ok := it[k]; !ok {
				t.Errorf("items[%d] 缺 %q 格（要逐条含步名/状态/退码/日志路径）：%+v", i, k, it)
			}
		}
		if !strings.HasPrefix(it["log"], dir) {
			t.Errorf("items[%d].log 要指向那一趟目录里的日志：%+v", i, it)
		}
		seen[it["step"]+"|"+it["status"]+"|"+it["rc"]] = true
	}
	if len(seen) < 4 {
		t.Errorf("items 出现「一坨同一个值」的迹象（只有 %d 种 step/status/rc 组合）—— 那是 `help export --json` 的病：%s", len(seen), out)
	}
	logs := map[string]bool{}
	for _, it := range items {
		logs[it["log"]] = true
	}
	if len(logs) != len(items) {
		t.Errorf("items 的日志路径要逐条不同（%d 条 / %d 个），得到 %v", len(items), len(logs), logs)
	}
	// meta 四数 —— 与 `_exit_rc` 同一条口径重算出来的那一份。
	var meta map[string]any
	if err := json.Unmarshal(doc["meta"], &meta); err != nil {
		t.Fatalf("meta 解不动：%v", err)
	}
	for k, want := range map[string]float64{"steps": 5, "pass": 2, "fail": 1, "blocked": 1, "report": 1, "exit_code": 1} {
		if meta[k] != want {
			t.Errorf("meta.%s = %v（要 %v）", k, meta[k], want)
		}
	}
}

// ② 子集投影：点几格给几格。
func TestGateResultsJSONSubsetProjects(t *testing.T) {
	dir := writeGateRun(t, gateRunFourStatus)
	_, out, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir, "--json", "status,log")
	items := envelopeItems(t, out)
	if len(items) == 0 {
		t.Fatalf("要非空 items · stderr=%s", errb)
	}
	for i, it := range items {
		if len(it) != 2 || it["status"] == "" || it["log"] == "" {
			t.Errorf("items[%d] = %+v（要只有 status/log 两格）", i, it)
		}
	}
	// 不给字段 ⇒ 闭集全量（与 K2 通例不同之处：文件头写了理由）。
	_, outAll, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir, "--json")
	all := envelopeItems(t, outAll)
	if len(all) == 0 || len(all[0]) != 5 {
		t.Fatalf("`--json` 不给字段 ⇒ 出全部五格，得到 %+v", all)
	}
}

// ③ 总退码三档：口径与脚本 `_exit_rc` 逐字对齐。
func TestGateResultsExitCodeMirrorsScriptRule(t *testing.T) {
	cases := []struct {
		name string
		rows [][5]string
		want int
	}{
		{"有 FAIL ⇒ 1", [][5]string{{"PASS", "a", "0", "0s", ""}, {"FAIL", "b", "1", "1s", ""}, {"BLOCKED", "c", "2", "0s", ""}}, 1},
		{"只有 BLOCKED ⇒ 2", [][5]string{{"PASS", "a", "0", "0s", ""}, {"BLOCKED", "c", "2", "0s", ""}, {"REPORT", "d", "1", "1s", ""}}, 2},
		{"全 PASS（REPORT 不算）⇒ 0", [][5]string{{"PASS", "a", "0", "0s", ""}, {"REPORT", "d", "1", "1s", ""}}, 0},
	}
	for _, c := range cases {
		dir := writeGateRun(t, c.rows)
		rc, out, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir)
		if rc != 0 {
			t.Fatalf("%s：读通就该退 0，得到 %d", c.name, rc)
		}
		if !strings.Contains(out, fmt.Sprintf("总退码 %d", c.want)) {
			t.Errorf("%s：人面要印「总退码 %d」：\n%s", c.name, c.want, out)
		}
	}
}

// ④ 三态 + ⑤ 认不出状态。
func TestGateResultsStatesAndUnknownStatus(t *testing.T) {
	// 能（0）已在上面几件里；这里是「不给结论」与「错」两档。
	rc, out, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", "/nonexistent-zerg-zz")
	if rc != 8 || !strings.Contains(errb, "不给结论") {
		t.Errorf("读不到那一趟 ⇒ 退 8 + 明说「不给结论」，得到 %d · stderr=%q", rc, errb)
	}
	if out != "" {
		t.Errorf("人面那一档失败时 stdout 不许出半份（四数一个都没数出来）：%q", out)
	}
	// 同一档给 `--json`：包封里 `error.kind` 要是 blocked（`8` 的 kind 唯一）。
	_, jout, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", "/nonexistent-zerg-zz", "--json", "step")
	if !strings.Contains(jout, "\"kind\":\"blocked\"") || !strings.Contains(jout, "\"exit_code\":8") {
		t.Errorf("不给结论那一档的 error 块要是 kind=blocked · exit_code=8：%q", jout)
	}
	// 空表 ⇒ 8。
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, gateResultsTSVName), []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", empty); rc != 8 {
		t.Errorf("空表 ⇒ 退 8（空转不是绿），得到 %d", rc)
	}
	// 错（2）：两个来源 / 字段表外 / 多余位置参数。
	dir := writeGateRun(t, gateRunFourStatus)
	if rc, _, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--last", "--dir", dir); rc != 2 || !strings.Contains(errb, "两个来源") {
		t.Errorf("`--last` 与 `--dir` 同给 ⇒ 2 + 明说「两个来源」，得到 %d · stderr=%q", rc, errb)
	}
	if rc, _, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", dir, "--json", "nosuchfield-zz"); rc != 2 || !strings.Contains(errb, "step,status,rc,secs,log") {
		t.Errorf("字段表外 ⇒ 2 + stderr 列字段表，得到 %d · stderr=%q", rc, errb)
	}
	if rc, _, errb := runZergRepo(t, resultsRepo(t), "gate", "results", dir); rc != 2 || !strings.Contains(errb, "不收位置参数") {
		t.Errorf("多余位置参数 ⇒ 2，得到 %d · stderr=%q", rc, errb)
	}
	// ⑤ 认不出状态 ⇒ 退 1 + `warnings` 非空 + 那几行不进计数。
	bad := writeGateRun(t, [][5]string{{"PASS", "a", "0", "0s", ""}})
	f, err := os.OpenFile(filepath.Join(bad, gateResultsTSVName), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("WAT\t怪状态步\t7\t0s\t/tmp/nope.log\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	rc2, out2, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", bad)
	if rc2 != 1 {
		t.Errorf("认不出状态的行 ⇒ 退 1（判不出不许当绿），得到 %d", rc2)
	}
	if !strings.Contains(out2, "状态计数：PASS 1") {
		t.Errorf("认得的那一行仍要计数：%q", out2)
	}
	_, jout2, _ := runZergRepo(t, resultsRepo(t), "gate", "results", "--dir", bad, "--json", "step,status")
	var env struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(jout2), &env); err != nil {
		t.Fatalf("解不动：%v · %q", err, jout2)
	}
	if len(env.Warnings) == 0 {
		t.Errorf("认不出状态必须进 warnings（不许静默吞掉）：%q", jout2)
	}
}

// ⑥ `--last`：取 `<TMPDIR>` 下 `zerg-gates-*` 里 `results.tsv` 最新的一趟；没有结果表的不算候选。
func TestGateResultsLastPicksNewest(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TMPDIR", base)
	old := filepath.Join(base, "zerg-gates-20260101-000000")
	uns := filepath.Join(base, "zerg-gates-20260202-000000") // 没有 results.tsv ⇒ 不是「一趟」
	newer := filepath.Join(base, "zerg-gates-20260303-000000")
	other := filepath.Join(base, "not-a-run") // 前缀不对 ⇒ 不扫
	for _, d := range []string{old, uns, newer, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTSV := func(dir, status string) {
		if err := os.WriteFile(filepath.Join(dir, gateResultsTSVName),
			[]byte(status+"\t"+filepath.Base(dir)+"\t0\t0s\t/tmp/x.log\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTSV(old, "PASS")
	writeTSV(newer, "FAIL")
	writeTSV(other, "PASS")
	rc, out, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--last")
	if rc != 0 {
		t.Fatalf("`--last` ⇒ 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "zerg-gates-20260303-000000") {
		t.Errorf("`--last` 要取最新那一趟（20260303）：\n%s", out)
	}
	if !strings.Contains(out, "FAIL 1") {
		t.Errorf("取到的应当是那份 FAIL 的：\n%s", out)
	}
	// 显式不给旗标 = 默认也取最近一趟（`--last` 是显式化的默认档）。
	if _, out2, _ := runZergRepo(t, resultsRepo(t), "gate", "results"); !strings.Contains(out2, "zerg-gates-20260303-000000") {
		t.Errorf("不给旗标也要取最近一趟：\n%s", out2)
	}
	// 一枚候选都没有 ⇒ 8（没回执不是绿）。
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(newer); err != nil {
		t.Fatal(err)
	}
	if rc, _, errb := runZergRepo(t, resultsRepo(t), "gate", "results", "--last"); rc != 8 || !strings.Contains(errb, "读不到那一趟") {
		t.Errorf("本机没有现成产物 ⇒ 8 + 明说读不到，得到 %d · stderr=%q", rc, errb)
	}
}

// ⑦ 只读：跑前/跑后整个趟目录逐件 sha256 + 清单逐字相同。
func TestGateResultsIsReadOnly(t *testing.T) {
	dir := writeGateRun(t, gateRunFourStatus)
	before := treeFingerprint(t, dir)
	for _, argv := range [][]string{
		{"gate", "results", "--dir", dir},
		{"gate", "results", "--dir", dir, "--json"},
		{"gate", "results", "--dir", dir, "--json", "status"},
		{"gate", "results", "--dir", "/nonexistent-zerg-zz"},
	} {
		_, _, _ = runZergRepo(t, resultsRepo(t), argv...)
	}
	if after := treeFingerprint(t, dir); after != before {
		t.Errorf("只读面跑完那一趟变了（不删不改任何日志那一条破了）：\n前=%s\n后=%s", before, after)
	}
}
