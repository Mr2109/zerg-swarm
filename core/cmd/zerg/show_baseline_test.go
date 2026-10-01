// show_baseline_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestShowBaselineGatePort（定向：go test -run TestShowBaselineGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sbwWrite —— 造件（存在即覆盖）。
func sbwWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sbwFixture —— 造一棵含仓根标记 + 四件棘轮件（+ 可选台账）的夹具树（仓外）。
// vals 逐条件给现值；ledgerVals 给台账里各条 new 值（nil ⇒ 不落台账）。
func sbwFixture(t *testing.T, vals map[string]int, ledgerVals map[string]int) string {
	t.Helper()
	root := t.TempDir()
	sbwWrite(t, filepath.Join(root, "core", "go.mod"), "")
	sbwWrite(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), "")
	body := func(key string, v int) string { return fmt.Sprintf("{\n  \"id\": \"fx\",\n  \"%s\": %d\n}\n", key, v) }
	sbwWrite(t, filepath.Join(root, "scripts", "gates", "gate-coverage.config.json"), body("baseline", vals["gate-coverage.baseline"]))
	sbwWrite(t, filepath.Join(root, "scripts", "gates", "cli-contract-baseline.json"), body("blanks", vals["cli-contract.blanks"]))
	sbwWrite(t, filepath.Join(root, "scripts", "gates", "doc-cmds-baseline.json"), body("devdocs_report_hits", vals["doc-cmds.devdocs_report_hits"]))
	sbwWrite(t, filepath.Join(root, "ui", "scripts", "i18n-baseline.json"), body("allowed_count", vals["i18n.allowed_count"]))
	if ledgerVals != nil {
		var lines []string
		for i, c := range sbConditions {
			v := ledgerVals[c.Name]
			rec := map[string]any{
				"seq": i + 1, "ts": "2026-09-24T11:26:05+08:00", "kind": "write", "name": c.Name,
				"gate": c.Gate, "file": c.File, "old": 0, "new": v, "reason": "夹具", "by": "fx",
				"sha256_before": "a", "sha256_after": "b", "measure": "夹具",
			}
			b, _ := json.Marshal(rec)
			lines = append(lines, string(b))
		}
		sbwWrite(t, filepath.Join(root, sbLEDGERRel), strings.Join(lines, "\n")+"\n")
	}
	return root
}

func sbwRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := showBaselineGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestShowBaselineGatePort(t *testing.T) {
	green := map[string]int{"gate-coverage.baseline": 5, "cli-contract.blanks": 0, "doc-cmds.devdocs_report_hits": 3, "i18n.allowed_count": 7}

	// ── 正控（必然判绿）：四件现值 == 台账最后一条 new ─────────────────────
	rcGreen, outGreen, _ := sbwRun("--root", sbwFixture(t, green, green))
	if rcGreen != 0 || !strings.Contains(outGreen, "⇒ 绿（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：件在 · 台账不在 ⇒ 「无记录」 ────────────────────
	rcNoLed, outNoLed, _ := sbwRun("--root", sbwFixture(t, green, nil))
	if rcNoLed != 1 || !strings.Contains(outNoLed, "无记录") {
		t.Fatalf("负控-无台账失败：得 rc=%d out=%q", rcNoLed, outNoLed)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcNoLed {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcNoLed)
	}

	// ── 负控 · 现值 ≠ 台账最后一条（没记录的手改） ────────────────────────
	rcMis, outMis, _ := sbwRun("--root", sbwFixture(t, map[string]int{"gate-coverage.baseline": 9, "cli-contract.blanks": 0, "doc-cmds.devdocs_report_hits": 3, "i18n.allowed_count": 7}, green))
	if rcMis != 1 || !strings.Contains(outMis, "没记录的手改") {
		t.Fatalf("负控-现值≠台账 失败：得 rc=%d out=%q", rcMis, outMis)
	}

	// ── 不给结论（rc=2）：台账断号（append-only 被破） ────────────────────
	rootGap := sbwFixture(t, green, green)
	gap := fmt.Sprintf(`{"seq": 1, "ts": "t", "kind": "write", "name": "%s", "file": "%s", "old": 5, "new": 5}
{"seq": 3, "ts": "t", "kind": "write", "name": "%s", "file": "%s", "old": 0, "new": 0}
`, sbConditions[0].Name, sbConditions[0].File, sbConditions[1].Name, sbConditions[1].File)
	sbwWrite(t, filepath.Join(rootGap, sbLEDGERRel), gap)
	rcGap, outGap, _ := sbwRun("--root", rootGap)
	if rcGap != 2 || !strings.Contains(outGap, "台账硬错") {
		t.Fatalf("负控-台账断号失败：得 rc=%d out=%q", rcGap, outGap)
	}

	// ── 取不到输入：根路径不存在 ⇒ rc=2 ──────────────────────────────────
	rcNoRoot, outNoRoot, _ := sbwRun("--root", filepath.Join(t.TempDir(), "nope"))
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "找不到仓根") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── 未知旗标 ⇒ rc=2（stderr 明说） ───────────────────────────────────
	rcFlag, _, errFlag := sbwRun("--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── 写意图 ⇒ rc=2 且件一字不写 ───────────────────────────────────────
	rootW := sbwFixture(t, green, green)
	fx := filepath.Join(rootW, sbConditions[0].File)
	pre := sbShaFile(fx)
	rcW, _, _ := sbwRun("--write", "--root", rootW)
	if rcW != 2 || sbShaFile(fx) != pre {
		t.Fatalf("写意图失败：得 rc=%d（件 sha 变=%v）", rcW, sbShaFile(fx) != pre)
	}

	// ── -h 逐字（内嵌 argparse 正文） ────────────────────────────────────
	rcH, outH, _ := sbwRun("-h")
	if rcH != 0 || !strings.Contains(outH, "usage: show-baseline.py") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── --json 机器面（rc=0 · 带 ledger 键） ─────────────────────────────
	rcJ, outJ, _ := sbwRun("--json", "--root", rootW)
	if rcJ != 0 || !strings.Contains(outJ, "\"single_source\": \"scripts/gates/baseline-ratchet.py\"") || !strings.Contains(outJ, "\"ledger_entries\": 4") {
		t.Fatalf("--json 失败：得 rc=%d out=%q", rcJ, outJ)
	}

	// ── --self-test 三态自身必须全过（防恒绿/恒红） ───────────────────────
	rcST, outST, _ := sbwRun("--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
