// dev_proposal_retract_test.go —— ★ GAP-20260928-95：`dev proposal --retract/--void <提案 id>` 作废档
// （打作废标记 · 号不复用 · 落审计一行）的机检。
//
// 落点：层①进程内 `zerg.RunForTest`（读的是**当前源码**）；夹具提案目录（`ZERG_PROPOSAL_DIR`）+
// 夹具审计落点（`ZERG_EDIT_AUDIT`）⇒ 不碰本机状态（同族 dev_proposal_test.go / cli_gap_state_test.go 同一落点口径）。
//
// 三条判据（正控 + 两条负控，真红）：
//
//	正控  作废一件 ⇒ rc=0；件里 `void=true`（**号仍在盘** —— 件不删 · `nextProposalID` 不复用）；
//	      `edit_audit.jsonl` 多一行（`event=proposal_retract` · 逐字相符）；`show` 读回面顶出作废标记；
//	      幂等重跑 ⇒ 0 且不写审计。
//	负控① 作废后再建新提案 ⇒ 号**不复用**（新件拿新号 ≠ 作废号）；作废件仍在盘（既有 O_EXCL 档：同路径被拒）。
//	负控② 缺 `--yes` / 缺理由 / 裸给旗标 / 提案不存在 / 两别名同给 ⇒ 按既有档拒（rc=2 · **零写**：
//	      件逐字节不变 · 无审计行）。
//
// ★ GAP-20261014-66（2026-10-14 追加 · 同件第二枚牙）：`TestDevProposalDryRunYesConflict` —— `dev proposal` 的四条
// 写面命令（`new` / `new --batch` / `--void` / `revise`）在 `--dry-run` 与 `--yes` **同给**时必须退 2
// （`detail=dry_run_yes_conflict`）；成对负控 = 单独 `--dry-run` 仍退 0 且零落件。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// retractRun 起一次作废档（**只走进程内 run** —— 层①的落点）。
func retractRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(append([]string{"dev", "proposal"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// readProposalJSON 读一件提案的 JSON（作废标记的取值面）。
func readProposalJSON(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件 %s 失败：%v", id, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	return rec
}

// retractAuditRows 读夹具审计落点里的全部行（每行解成 map；文件不存在 ⇒ 空）。
func retractAuditRows(t *testing.T, auditPath string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读审计落点失败：%v", err)
	}
	rows := []map[string]any{}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("审计行解不开：%v · %q", err, ln)
		}
		rows = append(rows, m)
	}
	return rows
}

func TestDevProposalRetractVoid(t *testing.T) {
	proposalEvidenceDir(t) // 出处闸读的合成缺口账（ZERG_STATE_DIR = 夹具）
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	auditPath := filepath.Join(t.TempDir(), "edit_audit.jsonl")
	t.Setenv("ZERG_EDIT_AUDIT", auditPath)

	newProposal := func() string {
		t.Helper()
		rc, out, errb := runProposalNew(t, "--title", "作废档夹具", "--target", "待办:"+ids[0], "--goal", "g",
			"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion)
		if rc != 0 {
			t.Fatalf("提不出来：rc=%d · stderr=%s", rc, errb)
		}
		id := strings.TrimSpace(out)
		if id == "" {
			t.Fatalf("成功时 stdout 要给提案 id")
		}
		return id
	}

	id := newProposal()
	if id != "DEV-0001" {
		t.Fatalf("夹具首件号 = %q（期望 DEV-0001）", id)
	}
	beforeBytes, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	reason := "探针误建，作废（GAP-20260928-95 自检）"
	unchanged := func(tag string) {
		t.Helper()
		if got, _ := os.ReadFile(filepath.Join(dir, id+".json")); string(got) != string(beforeBytes) {
			t.Errorf("%s：件被改了（拒执必须零写）", tag)
		}
	}

	// ---- 负控②：缺 `--yes` ⇒ 2 · 零写 ----
	rc, out, errb := retractRun(t, "--retract", id, "--reason", reason)
	if rc != 2 {
		t.Errorf("缺 --yes ⇒ rc=%d（要 2 · fail-closed）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, "--yes") {
		t.Errorf("缺 --yes 要点名 --yes：%q", errb)
	}
	unchanged("缺 --yes")
	if _, serr := os.Stat(auditPath); serr == nil {
		t.Errorf("缺 --yes 却写了审计行（拒执零写）")
	}

	// ---- 负控②：缺理由 ⇒ 2 ----
	rc, out, errb = retractRun(t, "--retract", id, "--yes")
	if rc != 2 || out != "" {
		t.Errorf("缺 --reason ⇒ rc=%d（要 2）· stdout=%q", rc, out)
	}
	if !strings.Contains(errb, "--reason") {
		t.Errorf("缺理由要点名 --reason：%q", errb)
	}
	unchanged("缺理由")

	// ---- 负控②：裸给旗标（无值）⇒ 2（不静默降级）----
	rc, out, errb = retractRun(t, "--retract", "--reason", reason, "--yes")
	if rc != 2 {
		t.Errorf("裸 --retract（无值）⇒ rc=%d（要 2 · 不降级）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("裸给时 stdout 必须 0 字节：%q", out)
	}
	unchanged("裸给")

	// ---- 负控②：提案不存在 ⇒ 2 ----
	rc, _, errb = retractRun(t, "--retract", "DEV-9999-nope", "--reason", reason, "--yes")
	if rc != 2 || !strings.Contains(errb, "没有") {
		t.Errorf("提案不存在 ⇒ rc=%d（要 2）、点名「没有」：%q", rc, errb)
	}

	// ---- 负控②：两别名同给 ⇒ 2 ----
	rc, _, errb = retractRun(t, "--retract", id, "--void", id, "--reason", reason, "--yes")
	if rc != 2 {
		t.Errorf("--retract 与 --void 同给 ⇒ rc=%d（要 2）· stderr=%s", rc, errb)
	}
	unchanged("负控一路")

	// ---- 正控：真作废 ⇒ rc=0 ----
	rc, out, errb = retractRun(t, "--retract", id, "--reason", reason, "--yes")
	if rc != 0 {
		t.Fatalf("真作废 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("成功时 stdout 要给作废回执")
	}
	rec := readProposalJSON(t, dir, id)
	if rec["void"] != true {
		t.Errorf("作废后可机读到标记要求 void=true，实得 %v", rec["void"])
	}
	if rec["void_reason"] != reason {
		t.Errorf("void_reason 要逐字 = %q，实得 %v", reason, rec["void_reason"])
	}
	if s, _ := rec["void_at"].(string); s == "" {
		t.Errorf("void_at（作废时刻）没落")
	}
	// 号仍在（件不删 —— 号不复用的机制在这一条）
	if _, serr := os.Stat(filepath.Join(dir, id+".json")); serr != nil {
		t.Fatalf("作废后件竟不在了（号必须仍在盘 ⇒ 不复用）：%v", serr)
	}
	// 审计行逐字相符
	rows := retractAuditRows(t, auditPath)
	var hit map[string]any
	for _, r := range rows {
		if r["event"] == "proposal_retract" {
			hit = r
		}
	}
	if hit == nil {
		t.Fatalf("审计里没有 event=proposal_retract 的行：%v", rows)
	}
	for k, want := range map[string]string{"proposal": id, "reason": reason, "proposal_dir": dir} {
		if got, _ := hit[k].(string); got != want {
			t.Errorf("审计行 %q 要求逐字 %q，实得 %q", k, want, got)
		}
	}
	if s, _ := hit["at"].(string); s == "" {
		t.Errorf("审计行 at（时刻）没落")
	}

	// ---- 正控：幂等重跑 ⇒ 0 且不写审计 ----
	nBefore := len(retractAuditRows(t, auditPath))
	rc, _, errb = retractRun(t, "--retract", id, "--reason", reason, "--yes")
	if rc != 0 {
		t.Errorf("幂等重跑 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if got := len(retractAuditRows(t, auditPath)); got != nBefore {
		t.Errorf("幂等重跑却多写了审计行（%d → %d）", nBefore, got)
	}

	// ---- 正控：show 读回面顶出作废标记（号仍在 · 只标作废）----
	var so, se strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id}, &so, &se); rc != 0 {
		t.Errorf("show ⇒ rc=%d（要 0）· stderr=%s", rc, se.String())
	}
	if !strings.Contains(so.String(), "作废") {
		t.Errorf("show 读回面要顶出作废标记：%s", so.String())
	}

	// ---- 负控①：同号再建新提案 ⇒ 号不复用（作废件仍占号）----
	id2 := newProposal()
	if id2 == id {
		t.Errorf("作废号被复用（新提案拿到 %q = 作废号）—— ✗ 静默复用", id2)
	}
	if id2 != "DEV-0002" {
		t.Errorf("作废后新件号 = %q（要 DEV-0002 · 作废件仍占号）", id2)
	}
	if _, serr := os.Stat(filepath.Join(dir, id+".json")); serr != nil {
		t.Errorf("作废件被删了（号会复用）：%v", serr)
	}
	// 既有 O_EXCL 档：同号路径**不可**再建（`new` 写路径的同名即拒）
	f, oerr := os.OpenFile(filepath.Join(dir, id+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if oerr == nil {
		f.Close()
		t.Errorf("同号路径竟可再建（既有 O_EXCL 档失效）")
	}
}

// TestDevProposalDryRunYesConflict —— ★ GAP-20261014-66（本单 · 运行时牙）：`dev proposal` 的四条
// 写面命令（`new` / `new --batch` / `--void` / `revise`）在 `--dry-run` 与 `--yes` **同给**时
// 必须退 2 且点 `detail=dry_run_yes_conflict` —— 让「件粒度 token 判」与**运行时**同尺。
//
//	病灶（立牙前的现读）：四条命令各自先跑 `--dry-run` 支并 `return exitOK`（早退），
//	`writeGuardYes` 里那句 `dryRunYesConflict` **永远走不到** ⇒ 同给时静默吞 `--yes`（rc=0）。
//	成对负控：单独 `--dry-run` 仍退 0（不误伤）· 且**一个提案件都不落**（零副作用）。
func TestDevProposalDryRunYesConflict(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(t.TempDir(), "edit_audit.jsonl"))

	// 夹具：先真建一枚提案（`--void` / `revise` 要有可审查物才走得到同给判定）。
	rc, out, errb := runProposalNew(t, "--title", "同给档夹具", "--target", "待办:"+ids[0],
		"--goal", "g", "--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Fatalf("夹具提案提不出来：rc=%d · stderr=%s", rc, errb)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		t.Fatalf("夹具提案没拿到 id（stdout=%q）", out)
	}
	countProps := func() int {
		ents, derr := os.ReadDir(dir)
		if derr != nil {
			t.Fatal(derr)
		}
		n := 0
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".json") {
				n++
			}
		}
		return n
	}

	// batch 件：一格合法（键 = 单片档旗标名去 `--`）。
	batch := filepath.Join(t.TempDir(), "batch.json")
	if werr := os.WriteFile(batch, []byte(`[{"title":"t","target":"待办:`+ids[0]+`","goal":"g","evidence":["`+
		proposalEvidenceInLedger+`"],"rollback":"r","criterion":"`+testCriterion+`",`+
		`"file":["core/cmd/zerg/family_dev.go"]}]`), 0o644); werr != nil {
		t.Fatal(werr)
	}

	same := []struct {
		name string
		argv []string
	}{
		{"new", []string{"new", "--dry-run", "--yes", "--title", "t", "--target", "待办:" + ids[0], "--goal", "g",
			"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion}},
		{"new --batch", []string{"new", "--batch", batch, "--dry-run", "--yes"}},
		{"--void", []string{"--void", id, "--reason", "同给档探针", "--dry-run", "--yes"}},
		{"revise", []string{"revise", id, "--criterion", testCriterion, "--dry-run", "--yes"}},
	}
	before := countProps()
	for _, c := range same {
		rc, out, errb := retractRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s · `--dry-run` 与 `--yes` 同给 ⇒ rc=%d（要 2 = 用法错）· stdout=%q · stderr=%q", c.name, rc, out, errb)
			continue
		}
		if !strings.Contains(errb, "dry_run_yes_conflict") {
			t.Errorf("%s · 退因不是本判定（stderr 里没有 detail=dry_run_yes_conflict）：%q", c.name, errb)
		}
	}
	if after := countProps(); after != before {
		t.Errorf("四条同给命令落了件（同给必须**零副作用**）：提案件数 %d → %d", before, after)
	}

	// 负控（成对）：单独 `--dry-run` 仍退 0 —— 不误伤 · 且零落件。
	for _, c := range []struct {
		name string
		argv []string
	}{
		{"new", []string{"new", "--dry-run", "--title", "t", "--target", "待办:" + ids[0], "--goal", "g",
			"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion}},
		{"--void", []string{"--void", id, "--reason", "同给档探针", "--dry-run"}},
		{"revise", []string{"revise", id, "--criterion", testCriterion, "--dry-run"}},
	} {
		rc, out, errb := retractRun(t, c.argv...)
		if rc != 0 {
			t.Errorf("负控 · 单独 `--dry-run`（%s）⇒ rc=%d（要 0 · 不许误伤）· stdout=%q · stderr=%q", c.name, rc, out, errb)
		}
	}
	if after := countProps(); after != before {
		t.Errorf("负控档落了件：提案件数 %d → %d", before, after)
	}
}
