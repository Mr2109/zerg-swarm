// cli_dev_edit_precheck_test.go —— `zerg dev edit` 的**预检档（第三态）**判据（`GAP-20260928-91` · 2026-10-19）。
//
// 病灶（缺口正文逐字）：「`dev edit` 的干跑档跳过批准闸 —— 没有『只验人签批准件、不写件』的预检面」：
// `--dry-run` 出计划件却不验批准链（验签与 scope 判定要到真写那一档才跑）。
//
// 本件钉住四态（全部在合成仓根 ＋ 合成状态目录里跑，不碰真仓的件、不碰真 `edit_audit.jsonl`）：
//
//	① **无批准件** ⇒ 预检档退 2 且**点名「无批准件」**，目标件一个字节不动、审计零新增行；
//	② **有件但 scope 不含本件** ⇒ 退 2 且**点名 scope 不含本件**，同上零副作用；
//	③ **合格** ⇒ 退 0 且打出**验签结论**（谁签的 · 时刻 · key_id）与**命中工具**（对上），仍零副作用；
//	④ **三态下 `edit_audit.jsonl` 零新增行** —— 干跑 / 预检（①②③ 各态）/ 真写拒执 都零新增行，
//	   只有真写成功才 +1（成对：证明「零新增行」不是「审计面根本不工作」）。
//
// 成对负控（✗ 禁恒绿）：⑤ 干跑档（无 `--confirm`）仍退 0 且**不打**预检结论（两档可区分）·
// ⑥ `--dry-run --confirm --yes` 同给 ⇒ 退 2（既有 `dry_run_yes_conflict`，先进）·
// ⑦ 预检档**不要求内容来源**（不给 `--from` 也能跑）· ⑧ 本档产不出的 `--json` 字段 ⇒ 验前拒。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countAuditLines —— 审计件行数（不存在 ⇒ 0；「零新增行」的判据面）。
func countAuditLines(t *testing.T, state string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("读不了审计件：%v", err)
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// TestDevEdit_VerifyApprovalPrecheck —— 预检档（第三态）四态 + 成对负控。
func TestDevEdit_VerifyApprovalPrecheck(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9101", target)
	sign := writeOperatorKey(t, state)
	host := hostnameOf(t)
	after := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, after, "package main\n\n// 真写才落的内容\n")

	// 预置两行审计：**非空基线**才能证明「零新增行」（空 → 空 那条会被「审计面根本不工作」骗过）。
	mustWrite(t, filepath.Join(state, "edit_audit.jsonl"),
		"{\"event\":\"edit\",\"proposal\":\"DEV-old1\",\"file\":\"x\"}\n{\"event\":\"edit\",\"proposal\":\"DEV-old2\",\"file\":\"y\"}\n")
	auditBase := countAuditLines(t, state)
	if auditBase != 2 {
		t.Fatalf("夹具自检：审计基线应为 2 行，实得 %d", auditBase)
	}
	targetBefore, _ := os.ReadFile(filepath.Join(repo, target))

	// ① 无批准件 ⇒ 退 2 且点名「无批准件」，零副作用。
	rc, out, errb := runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host)
	if rc != 2 {
		t.Fatalf("①无批准件 ⇒ 预检档应退 2，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "无批准件") {
		t.Errorf("①要点名「无批准件」：%q", errb)
	}
	if !strings.Contains(out, "预检件（预检档") {
		t.Errorf("①人面要能认出这是预检档（三态可判）：%q", out)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != string(targetBefore) {
		t.Errorf("①预检档零写入：目标件被改了 %q", got)
	}
	if n := countAuditLines(t, state); n != auditBase {
		t.Errorf("①预检档零审计：审计 %d → %d（要求零新增行）", auditBase, n)
	}

	// ② 有件但 scope 不含本件 ⇒ 退 2 且点名 scope。
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "core/别的目录/", "张三", "2026-10-19T00:00:00+08:00", "批的是别处")))
	rc, out, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host)
	if rc != 2 {
		t.Fatalf("②scope 不含本件 ⇒ 预检档应退 2，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "scope 不含本件") {
		t.Errorf("②要点名「scope 不含本件」：%q", errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != string(targetBefore) {
		t.Errorf("②预检档零写入：目标件被改了 %q", got)
	}
	if n := countAuditLines(t, state); n != auditBase {
		t.Errorf("②预检档零审计：审计 %d → %d", auditBase, n)
	}

	// ③ 合格 ⇒ 退 0 + 验签结论 + 命中工具，仍零副作用。
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-19T07:00:00+08:00", "放行这一件")))
	rc, out, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host)
	if rc != 0 {
		t.Fatalf("③合格 ⇒ 预检档应退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "验过") || !strings.Contains(out, "张三") || !strings.Contains(out, "key_id=") {
		t.Errorf("③要打出验签结论（谁签的 · 时刻 · key_id）：%q", out)
	}
	if !strings.Contains(out, "命中工具") || !strings.Contains(out, "对上") {
		t.Errorf("③要打出「命中工具」且判对上：%q", out)
	}
	if !strings.Contains(out, "预检结论") || !strings.Contains(out, "可落") {
		t.Errorf("③要给预检结论：%q", out)
	}
	if !strings.Contains(errb, "零新增行") {
		t.Errorf("③stderr 要明写「零写入零审计」的承诺：%q", errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != string(targetBefore) {
		t.Errorf("③预检档零写入（合格也不行）：目标件被改了 %q", got)
	}
	if n := countAuditLines(t, state); n != auditBase {
		t.Errorf("③预检档零审计：审计 %d → %d", auditBase, n)
	}
	// ③-b `--json` 只给本档产得出的名字（approval_scope_hit / approver 是干跑档那一族，预检同样产）。
	rc, out, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host, "--json", "approval_scope_hit,approver")
	if rc != 0 || !strings.Contains(out, "\"approval_scope_hit\":\"yes\"") || !strings.Contains(out, "\"approver\":\"张三\"") {
		t.Errorf("③-b `--json` 预检面：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}

	// ④ 干跑档（无 `--confirm`）仍是**另一档**：退 0（不需要批准件）且不打预检结论。
	rc, out, _ = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9101", "--file", target, "--from", after, "--dry-run")
	if rc != 0 {
		t.Fatalf("④干跑档 ⇒ 退 0，得到 %d", rc)
	}
	if strings.Contains(out, "预检结论") {
		t.Errorf("④干跑档不许打预检结论（两档要能区分）：%q", out)
	}

	// ⑤ 同给判定先进：`--dry-run --confirm --yes` ⇒ 退 2（既有 dry_run_yes_conflict）。
	rc, _, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host, "--yes")
	if rc != 2 || !strings.Contains(errb, "dry_run_yes_conflict") {
		t.Errorf("⑤`--dry-run --confirm --yes` ⇒ 退 2 且点名 dry_run_yes_conflict：rc=%d stderr=%q", rc, errb)
	}

	// ⑥ 确认值不匹配 ⇒ 预检档也退 2（与真写同一判据 `devEditHostAccept`），仍零副作用。
	rc, _, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm=不是本机-zz")
	if rc != 2 || !strings.Contains(errb, "本机名") {
		t.Errorf("⑥确认值不符 ⇒ 退 2 且点名本机名：rc=%d stderr=%q", rc, errb)
	}
	if n := countAuditLines(t, state); n != auditBase {
		t.Errorf("⑥零审计：审计 %d → %d", auditBase, n)
	}

	// ⑦ 本档产不出的 `--json` 字段 ⇒ 验前拒（不静默略去）。
	rc, _, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--dry-run", "--confirm="+host, "--json", "before_sha256")
	if rc != 2 || !strings.Contains(errb, "不产出字段") {
		t.Errorf("⑦预检档要不出真写/干跑档的字段名 ⇒ 退 2 且点名：rc=%d stderr=%q", rc, errb)
	}

	// ⑧ 成对：真写成功 ⇒ 审计 +1（证明上面那些「零新增行」不是审计面不工作）。
	rc, out, errb = runDevEdit(t, repo, prop, state,
		"--proposal", "DEV-9101", "--file", target, "--from", after, "--confirm="+host, "--yes")
	if rc != 0 {
		t.Fatalf("⑧真写 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if n := countAuditLines(t, state); n != auditBase+1 {
		t.Errorf("⑧真写要落审计（+1 行）：审计 %d → %d", auditBase, n)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); !strings.Contains(string(got), "真写才落的内容") {
		t.Errorf("⑧真写要真落盘：%q", got)
	}
}
