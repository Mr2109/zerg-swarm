// gap_verify_batch_isolation_test.go —— `zerg gap verify --all` 批量档**逐条隔离**的回归牙。
//
// 病（改前）：批中某条的判据落在**写面**（危险档且没带 `--dry-run`）⇒ 整批**中止**（退码 8），
// 一条判据都不跑 ⇒ 批量复核不可用（前面可跑的那些读数也拿不到）。
// 治（本笔）：批量档逐条隔离 —— 写面那条记「跳过·需人签」并**继续跑其余**；写面判据**仍不真跑**
// （闸不松）；末尾逐条点名 + 计数。单条面语义/退码**一字不动**（仍 8）。
//
// 负控（哑化隔离 = 改前态）：本件在同一条命令上必 FAIL（rc=8 ≠ 0）。
package main_test

import (
	"os"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// gapIsolationLine 一条合法真源行（state=仍缺 · 判据按参数给）。
func gapIsolationLine(id, fp, verify string) string {
	return `{"id":"` + id + `","fp":"` + fp + `","symptom":"s","handmade":"h","impact":"命令面","prio":"P1",` +
		`"state":"仍缺","want_family":"gap","want_action":"ls","repro_cmd":"x",` +
		`"verify_cmd":"` + verify + `","found_at":"2026-10-10T00:00:00+08:00"}`
}

func TestGapVerifyBatchIsolatesWriteFace(t *testing.T) {
	_, ledger, _ := gapEnv(t)
	lines := []string{
		// 只读判据（可跑 · 会解析但判红 rc≠0 ⇒ 不销案，避开两态闸干扰）
		gapIsolationLine("GAP-FX-RO", "x", "zerg version --json nosuchfield-zz"),
		// 写面判据（危险档 `help export` · 批中那条 · 绝不许真跑）
		gapIsolationLine("GAP-FX-WF", "y", "help export"),
	}
	if err := os.WriteFile(ledger, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("夹具写真源失败：%v", err)
	}

	// ① 批量档（改前 ⇒ rc=8 整批中止；改后 ⇒ rc=0 且继续跑完）
	rc, out, errb := runGapIsolation(t, "gap", "verify", "--all", "--dry-run")
	if rc != 0 {
		t.Errorf("批量档遇写面判据**不许整批中止**（改前 rc=8）⇒ 要 0，得到 %d · out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "GAP-FX-RO") {
		t.Errorf("可跑的只读判据要**照跑**并逐条点名 GAP-FX-RO，得到 %q", out)
	}
	if !strings.Contains(out, "GAP-FX-WF") || !strings.Contains(out, "跳过·需人签") {
		t.Errorf("写面判据要记「跳过·需人签」并点名 GAP-FX-WF，得到 %q", out)
	}
	if !strings.Contains(out, "1 条写面判据") {
		t.Errorf("末尾汇总要报计数，得到 %q", out)
	}

	// ② 真跑（--yes）批量：写面那条**不被改态**、可跑那条照写；rc 不再被写面拖成 8
	rc, out, errb = runGapIsolation(t, "gap", "verify", "--all", "--yes")
	if rc != 0 {
		t.Errorf("--yes 批量遇写面判据不许退 8 ⇒ 要 0，得到 %d · out=%q err=%q", rc, out, errb)
	}
	if rec := gapRecordOf(t, ledger, "GAP-FX-WF"); rec["state"] != "仍缺" {
		t.Errorf("写面判据那条**不许**被改态，得到 %v", rec["state"])
	}
	if rec := gapRecordOf(t, ledger, "GAP-FX-RO"); rec["state"] != "仍缺" {
		t.Errorf("判红（rc≠0）那条不该销案，得到 %v", rec["state"])
	}

	// ③ 单条面语义/退码**一字不动**：点名一条写面判据 ⇒ 仍「一条都不跑、不给结论」退 8
	if rc, _, _ := runGapIsolation(t, "gap", "verify", "GAP-FX-WF", "--dry-run"); rc != 8 {
		t.Errorf("单条面（点名写面判据）⇒ 仍要 8（一字不动），得到 %d", rc)
	}
}

// runGapIsolation 走与 `cli_gap_test.go` 同一入口（`zerg.RunForTest` · 跑当前源码）。
func runGapIsolation(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}
