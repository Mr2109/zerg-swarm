// cli_gap_state_test.go —— `gap` 族两条**状态写面**（`set-state` / `note` · 2026-09-26 ·
// 缺口账 `Q-239`）的机检。
//
// 落点与同族一致：`package main_test` + `zerg.RunForTest` —— 跑的是**当前源码**的行为；
// 全部合成盘面（`t.TempDir()` 当状态目录 · 复用 `cli_gap_test.go` 的 `gapEnv` / `gapRun` /
// `gapAddArgv` / `gapAuditLines` / `gapIDOf` 五把尺）⇒ **不碰真 `~/.zerg/state`**。
//
// 钉住的九格（逐格对设计口径）：
//
//	① 闭集：`--state` 闭集外 ⇒ 2 且 stderr 逐字印闭集（`仍缺 / 已解`）
//	② 缺 `--yes` ⇒ 2 且 **stdout 0 字节**（计划件走 stderr · H-5）· 真源一个字节未动
//	③ `--dry-run` ⇒ 0 且**零副作用**（真源 sha256 逐字不变 + 审计行数不变）
//	④ 真跑：`state` 转 `已解` + `solved_at` / `solved_evidence` 落位 · 真源行数不变 · 审计尾行 `gap_cmd=set-state`
//	⑤ 幂等：同 id 同态同证据 ⇒ 0「无变化」且**真源 sha 与审计行数都不动**
//	⑥ 账外 id ⇒ 2（本动作取定 · 与 `verify` 的 8 **不同码**）
//	⑦ `note` 真跑：`notes` +1 而 **`state` 逐字不动** · 审计尾行 `gap_cmd=note`；`note` 收 `--state` ⇒ 2
//	⑧ `note` 幂等：同 by 同 text ⇒ 0「无变化」且不写
//	⑨ 审计写不进 ⇒ 8 且**真源一个字节不改**（fail-closed）
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapRecOf 从真源里按 id 取那一行（解析成 map —— 只读，不改）。
func gapRecOf(t *testing.T, ledger, id string) map[string]any {
	t.Helper()
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源某行不是 JSON：%v", err)
		}
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("真源里找不到 %s", id)
	return nil
}

func TestGapStateFacesSetStateAndNote(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 靶子：走同族写面 `add` 造一条（**不用**手搓真源 —— 真源只由命令写）。
	rc, out, errb := gapRun(t, append(gapAddArgv("手搓：改态与写口径没有正门", "状态写面靶子"), "--yes")...)
	if rc != 0 {
		t.Fatalf("造靶子失败：rc=%d · stdout=%q stderr=%q", rc, out, errb)
	}
	id := gapIDOf(t, out)

	// ① 闭集外 ⇒ 2 且逐字印闭集
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已派", "--evidence", "一句")
	if rc != 2 {
		t.Errorf("`--state 已派` 要 2（闭集外），得到 %d", rc)
	}
	if !strings.Contains(errb, "仍缺 / 已解") {
		t.Errorf("闭集外那一档没印闭集（要看到「仍缺 / 已解」）：%q", errb)
	}

	// ② 缺 `--yes` ⇒ 2 且 stdout 0 字节 + 真源不动
	sha0 := sha256Of(t, ledger)
	rc, out, _ = gapRun(t, "gap", "set-state", id, "--state", "已解", "--evidence", "一句")
	if rc != 2 || out != "" {
		t.Errorf("缺 --yes 要 2 且 stdout 0 字节（H-5），得到 rc=%d stdout=%q", rc, out)
	}
	if sha256Of(t, ledger) != sha0 {
		t.Errorf("缺 --yes 那一档动了真源")
	}

	// ③ `--dry-run` ⇒ 0 且零副作用
	na0 := len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "set-state", id, "--state", "已解", "--evidence", "e1", "--dry-run")
	if rc != 0 {
		t.Errorf("--dry-run 恒 0，得到 %d", rc)
	}
	if !strings.Contains(out, "计划件") {
		t.Errorf("--dry-run 的计划件没走 stdout：%q", out)
	}
	if sha256Of(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("--dry-run 有副作用（真源 sha 或审计行数变了）")
	}

	// ④ 真跑
	// ⑤ 批4 第四片：销案两态对拍闸（新账强制）—— 真跑那一发必须带成对的改前/改后读数。
	rc, out, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--evidence", "e1",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前该号列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后该号列于已解", "--yes")
	if rc != 0 {
		t.Fatalf("真跑要 0，得到 %d · stderr=%q", rc, errb)
	}
	if len(gapReadLines(t, ledger)) != 1 {
		t.Errorf("改态**不许**增行（真源仍应 1 行）")
	}
	r := gapRecOf(t, ledger, id)
	if r["state"] != "已解" || r["solved_evidence"] != "e1" || r["solved_at"] == nil || r["solved_at"] == "" {
		t.Errorf("改态落位不对：state=%v solved_at=%v solved_evidence=%v", r["state"], r["solved_at"], r["solved_evidence"])
	}
	rows := gapAuditLines(t, audit)
	if got := rows[len(rows)-1]["gap_cmd"]; got != "set-state" {
		t.Errorf("审计尾行 gap_cmd=%v（要 set-state）", got)
	}
	if rows[len(rows)-1]["gap_state_before"] != "仍缺" || rows[len(rows)-1]["gap_state_after"] != "已解" {
		t.Errorf("审计尾行的 before/after 不对：%v → %v",
			rows[len(rows)-1]["gap_state_before"], rows[len(rows)-1]["gap_state_after"])
	}

	// ⑤ 幂等：同 id 同态同证据 ⇒ 0「无变化」· 真源 sha 与审计行数都不动
	sha1, na1 := sha256Of(t, ledger), len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "set-state", id, "--state", "已解", "--evidence", "e1", "--yes")
	if rc != 0 || !strings.Contains(out, "无变化") {
		t.Errorf("同态同证据要 0「无变化」，得到 rc=%d stdout=%q", rc, out)
	}
	if sha256Of(t, ledger) != sha1 || len(gapAuditLines(t, audit)) != na1 {
		t.Errorf("幂等命中却写了盘（真源 sha 或审计行数变了）")
	}

	// ⑥ 账外 id ⇒ 2（本动作取定 · 与 `verify` 的 8 不同码）
	if rc, _, _ = gapRun(t, "gap", "set-state", "GAP-29990101-99", "--state", "已解", "--evidence", "x", "--yes"); rc != 2 {
		t.Errorf("账外 id 要 2，得到 %d", rc)
	}

	// ⑦ `note`：`notes` +1 而 `state` 逐字不动；`note` 收 `--state` ⇒ 2
	if rc, _, _ = gapRun(t, "gap", "note", id, "--text", "一句话", "--state", "仍缺", "--yes"); rc != 2 {
		t.Errorf("`gap note --state` 要 2（注不改态），得到 %d", rc)
	}
	rc, out, errb = gapRun(t, "gap", "note", id, "--text", "一句话", "--yes")
	if rc != 0 {
		t.Fatalf("note 真跑要 0，得到 %d · stderr=%q", rc, errb)
	}
	r = gapRecOf(t, ledger, id)
	notes, _ := r["notes"].([]any)
	if r["state"] != "已解" || len(notes) != 1 {
		t.Errorf("note 之后 state=%v（要逐字不动 已解）· notes=%v（要 1 条）", r["state"], r["notes"])
	}
	rows = gapAuditLines(t, audit)
	if got := rows[len(rows)-1]["gap_cmd"]; got != "note" {
		t.Errorf("审计尾行 gap_cmd=%v（要 note）", got)
	}

	// ⑧ `note` 幂等
	sha2, na2 := sha256Of(t, ledger), len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "note", id, "--text", "一句话", "--yes")
	if rc != 0 || !strings.Contains(out, "无变化") {
		t.Errorf("同 by 同 text 要 0「无变化」，得到 rc=%d stdout=%q", rc, out)
	}
	if sha256Of(t, ledger) != sha2 || len(gapAuditLines(t, audit)) != na2 {
		t.Errorf("note 幂等命中却写了盘")
	}

	// ⑨ 审计写不进 ⇒ 8 且真源一个字节不改（fail-closed）
	blocker := filepath.Join(filepath.Dir(ledger), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(blocker, "edit_audit.jsonl"))
	sha3 := sha256Of(t, ledger)
	rc, _, _ = gapRun(t, "gap", "set-state", id, "--state", "仍缺", "--evidence", "这一跑要写不进审计", "--yes")
	if rc == 0 {
		t.Errorf("审计写不进却退 0（fail-closed 破了）")
	}
	if sha256Of(t, ledger) != sha3 {
		t.Errorf("审计写不进却动了真源（审计先落盘那一条破了）")
	}
}
