// family_gap_judgeset_test.go —— `zerg gap judge set`（**改判据写面** · 缺口 `GAP-20261010-34`）的回归牙。
//
// 面：本族此前只有 `set-state`（改 `state`）与 `set-unit`（改件面三键）两条改字段正门 ⇒
// **改 `verify_cmd` / `repro_cmd` 两键没有正门**（账内 ~99 条判据无鉴别力 / 与 want 无关，全卡在这一格）。
// 本动作照同族 `set-unit` 之型：走唯一收口 `gapRewriteOne`（锁 + 锁内现读 + 审计先落盘 + 整件原子换
// + 写后读回对拍）· `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2 · `--evidence` 必填。
//
// 钉住的格（逐格缺一即失败）：
//
//	① 用法面：缺 `--verify-cmd` / 缺 `--evidence` / 收 `--state` / `--repro-cmd` 裸给 ⇒ 各 2
//	② 校验（复用 `gap add` 的既有助手）：`--verify-cmd` 样板值 ⇒ 2 点名「样板值」· 不在命令树 ⇒ 2 点名
//	③ 三态：缺 `--yes` ⇒ 2（stdout 0 字节）· `--dry-run` ⇒ 0 零副作用 · `--yes` ⇒ 0
//	④ 真写：`verify_cmd` / `repro_cmd` 两键真变（回读相符）· `notes` +1（改判据留证）· `state` 逐字不动 ·
//	   审计尾行 `gap_cmd=judge set`
//	⑤ 幂等：同 id 同两键 ⇒ 0「无变化」且真源 sha 与审计行数都不动
//	⑥ 机器面：`--json` 回吐两键
//	⑦ 负控（fail-closed）：审计写不进 ⇒ 非 0 且真源一个字节不改
//
// 落点与同族一致：`package main_test` + `zerg.RunForTest`（跑当前源码行为）· 全部合成盘面（`t.TempDir()`
// 当状态目录 · 复用 `cli_gap_test.go` 的 `gapEnv` / `gapRun` / `gapAddArgv` / `gapIDOf` / `gapRecordOf` /
// `gapAuditLines` / `gapSHAOf` 七把尺）⇒ **不碰真 `~/.zerg/state`**、不碰任何仓内件。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGapJudgeSetWriteFace(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 靶子：走同族写面 `add` 造一条（**不用**手搓真源 —— 真源只由命令写）。
	// `add` 的 `--verify-cmd` 落 `zerg version --json name`（可解析、非样板），`--repro-cmd` 落一条复现串。
	rc, out, errb := gapRun(t, append(gapAddArgv("手搓：账内判据无鉴别力 / 与 want 无关", "改判据靶子"), "--yes")...)
	if rc != 0 {
		t.Fatalf("造靶子失败：rc=%d · stdout=%q stderr=%q", rc, out, errb)
	}
	id := gapIDOf(t, out)
	sha0 := gapSHAOf(t, ledger)

	// ① 用法面（在任何盘面动作之前 ⇒ 真源一个字节都不许动）
	if rc, _, _ := gapRun(t, "gap", "judge", "set", id, "--evidence", "e", "--yes"); rc != 2 {
		t.Errorf("① 缺 --verify-cmd ⇒ 2，得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg version", "--yes"); rc != 2 {
		t.Errorf("① 缺 --evidence ⇒ 2，得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg version", "--evidence", "e",
		"--state", "仍缺", "--yes"); rc != 2 {
		t.Errorf("① 收 --state ⇒ 2（本面只改判据两键），得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg version", "--repro-cmd",
		"--evidence", "e", "--yes"); rc != 2 {
		t.Errorf("① 裸给 --repro-cmd ⇒ 2（裸给不降级），得到 %d", rc)
	}

	// ② 校验（逐字复用 `gap add` 的既有助手）
	rc, _, errb = gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg gap ls", "--evidence", "e", "--yes")
	if rc != 2 {
		t.Errorf("② --verify-cmd 样板值 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "样板值") || !strings.Contains(errb, "--verify-cmd") {
		t.Errorf("② 拒收判词要点名「样板值」与 --verify-cmd，得到 %q", errb)
	}
	if rc, _, errb := gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg 无此命令-zz", "--evidence", "e", "--yes"); rc != 2 {
		t.Errorf("② --verify-cmd 不在命令树 ⇒ 2，得到 %d · %s", rc, errb)
	}
	// 负控：以上拒收都不许动真源
	if gapSHAOf(t, ledger) != sha0 {
		t.Errorf("② 拒收不许动真源")
	}

	// ③ 缺 `--yes` ⇒ 2 且 stdout 0 字节（计划件走 stderr · H-5）
	rc, out, _ = gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg version", "--evidence", "e")
	if rc != 2 || out != "" {
		t.Errorf("③ 缺 --yes 要 2 且 stdout 0 字节，得到 rc=%d stdout=%q", rc, out)
	}
	if gapSHAOf(t, ledger) != sha0 {
		t.Errorf("③ 缺 --yes 那一档动了真源")
	}
	// ③ `--dry-run` ⇒ 0 且零副作用
	na0 := len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "judge", "set", id, "--verify-cmd", "zerg version", "--evidence", "e", "--dry-run")
	if rc != 0 {
		t.Errorf("③ --dry-run 恒 0，得到 %d", rc)
	}
	if !strings.Contains(out, "计划件") {
		t.Errorf("③ --dry-run 的计划件没走 stdout：%q", out)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("③ --dry-run 有副作用（真源 sha 或审计行数变了）")
	}

	// ④ 真写：两键真变 + notes +1 + state 不动 + 审计尾行 gap_cmd=judge set
	rc, out, errb = gapRun(t, "gap", "judge", "set", id,
		"--verify-cmd", "zerg version --json commit",
		"--repro-cmd", "zerg gap show --json id,verify_cmd",
		"--evidence", "换一条有鉴别力的判据（现读对拍）", "--yes")
	if rc != 0 {
		t.Fatalf("④ 真写要 0，得到 %d · stdout=%q stderr=%q", rc, out, errb)
	}
	if len(gapReadLines(t, ledger)) != 1 {
		t.Errorf("④ 改判据**不许**增行（真源仍应 1 行）")
	}
	r := gapRecordOf(t, ledger, id)
	if r["verify_cmd"] != "zerg version --json commit" {
		t.Errorf("④ verify_cmd 未落位，得到 %v", r["verify_cmd"])
	}
	if r["repro_cmd"] != "zerg gap show --json id,verify_cmd" {
		t.Errorf("④ repro_cmd 未落位，得到 %v", r["repro_cmd"])
	}
	if r["state"] != "仍缺" {
		t.Errorf("④ 改判据不许动 state（要仍是 仍缺），得到 %v", r["state"])
	}
	notes, _ := r["notes"].([]any)
	if len(notes) != 1 || !strings.Contains(notes[0].(string), "改判据 verify_cmd:") {
		t.Errorf("④ 应落 1 条改判据留证注，得到 %v", r["notes"])
	}
	rows := gapAuditLines(t, audit)
	if got := rows[len(rows)-1]["gap_cmd"]; got != "judge set" {
		t.Errorf("④ 审计尾行 gap_cmd=%v（要 judge set）", got)
	}

	// ⑤ 幂等：同 id 同两键 ⇒ 0「无变化」· 真源 sha 与审计行数都不动（证据不同也不写）
	sha1, na1 := gapSHAOf(t, ledger), len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "judge", "set", id,
		"--verify-cmd", "zerg version --json commit",
		"--repro-cmd", "zerg gap show --json id,verify_cmd",
		"--evidence", "另一句证据", "--yes")
	if rc != 0 || !strings.Contains(out, "无变化") {
		t.Errorf("⑤ 同两键要 0「无变化」，得到 rc=%d stdout=%q", rc, out)
	}
	if gapSHAOf(t, ledger) != sha1 || len(gapAuditLines(t, audit)) != na1 {
		t.Errorf("⑤ 幂等命中却写了盘（真源 sha 或审计行数变了）")
	}

	// ⑥ 机器面：`--json` 回吐两键
	rc, out, _ = gapRun(t, "gap", "judge", "set", id,
		"--verify-cmd", "zerg version --json name",
		"--evidence", "机器面读数", "--yes", "--json", "id,verify_cmd,changed")
	if rc != 0 || !strings.Contains(out, `"verify_cmd":"zerg version --json name"`) {
		t.Errorf("⑥ --json 没回吐 verify_cmd：rc=%d out=%q", rc, out)
	}

	// ⑦ 负控（fail-closed）：审计写不进 ⇒ 非 0 且真源一个字节不改
	blocker := filepath.Join(filepath.Dir(ledger), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(blocker, "edit_audit.jsonl"))
	sha2 := gapSHAOf(t, ledger)
	rc, _, _ = gapRun(t, "gap", "judge", "set", id,
		"--verify-cmd", "zerg version --json commit",
		"--repro-cmd", "zerg gap show --json id", "--evidence", "写不进审计", "--yes")
	if rc == 0 {
		t.Errorf("⑦ 审计写不进却退 0（fail-closed 破了）")
	}
	if gapSHAOf(t, ledger) != sha2 {
		t.Errorf("⑦ 审计写不进却动了真源（审计先落盘那一条破了）")
	}
}
