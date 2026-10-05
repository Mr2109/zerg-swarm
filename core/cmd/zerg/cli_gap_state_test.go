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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	// ⑤ 批4 第五片：销案证据形态钉死 ⇒ `solved_evidence` 落的是写死形态串（两态对拍 + 散文）。
	if r["state"] != "已解" || !strings.Contains(fmt.Sprint(r["solved_evidence"]), "散文=e1") || r["solved_at"] == nil || r["solved_at"] == "" {
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
	// ★ 2026-09-29 随动（批4-5 销案证据形态钉死）：新账销案的 `solved_evidence` 落的是**写死形态串**
	//   （两态对拍 + 散文，见 `family_gap_state.go` 的 `gapTwoStateLanded`）⇒ 幂等比对的那一格
	//   = `销案两态对拍 · 改前{…} · 改后{…} · 散文=e1` ⇒ 这一发必须带**与真跑那一发逐字相同**的一对读数，
	//   否则证据格不同 ⇒ 不走幂等、直撞销案两态闸（退 2）。
	rc, out, _ = gapRun(t, "gap", "set-state", id, "--state", "已解", "--evidence", "e1",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前该号列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后该号列于已解", "--yes")
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

// TestGapSolvedKindPartition —— 缺口账 `GAP-20260927-249`（「销案 ≠ 修」的机制化）的成对机检。
//
// 钉住的格（逐格缺一即失败）：
//
//	① 真写：`--state 已解 --solved-kind 销案` ⇒ rc=0 且账行 `solved_kind=销案`
//	② 缺省：`--state 已解` 不带 `--solved-kind` ⇒ 账行 `solved_kind=未分类`（老账/未分类）
//	③ 闭集：`--solved-kind 乱` ⇒ 2
//	④ 裸给：`--solved-kind`（缺值）⇒ 2（不许静默降级）
//	⑤ 只对已解：`--state 仍缺 --solved-kind 销案` ⇒ 2
//	⑥ 报数分栏：`gap ls` 人面印「已解分栏（solved_kind）：…」且 `--json` 带 `solved_kind` 格
//	⑦ 幂等：同 id 同态同证据同 kind ⇒ 0「无变化」且不写盘
func TestGapSolvedKindPartition(t *testing.T) {
	_, ledger, audit := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("手搓：销案与实现压一个态", "销案分栏靶子"), "--yes")...); rc != 0 {
		t.Fatalf("造靶子失败：rc=%d stderr=%q", rc, errb)
	}
	// 造第二条（拿它的 GAP id；add 的 stdout 回执行）
	rc, out, errb := gapRun(t, append(gapAddArgv("手搓：第二条靶子", "缺省分栏靶子"), "--yes")...)
	if rc != 0 {
		t.Fatalf("造第二条失败：rc=%d stderr=%q", rc, errb)
	}
	id2 := gapIDOf(t, out)

	// 取两条 id：从真源逐行解析（与同族 `gapRecOf` 同源口径）。
	ids := []string{}
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源某行不是 JSON：%v", err)
		}
		if s, _ := m["id"].(string); s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("靶子条数不对：%v", ids)
	}
	id1 := ids[0]

	brA := "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺"
	brB := "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解"

	// ③ 闭集外 ⇒ 2
	if rc, _, e := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--evidence", "e", "--solved-kind", "乱", "--yes"); rc != 2 {
		t.Errorf("`--solved-kind 乱` 要 2，得到 %d · %s", rc, e)
	}
	// ④ 裸给 ⇒ 2
	if rc, _, _ := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--evidence", "e", "--solved-kind", "--yes"); rc != 2 {
		t.Errorf("裸给 --solved-kind 要 2，得到 %d", rc)
	}
	// ⑤ 只对已解 ⇒ 2
	if rc, _, _ := gapRun(t, "gap", "set-state", id1, "--state", "仍缺", "--evidence", "e", "--solved-kind", "销案", "--yes"); rc != 2 {
		t.Errorf("`--state 仍缺 --solved-kind` 要 2，得到 %d", rc)
	}

	// ① 真写：销案
	if rc, _, e := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--evidence", "这一条是销案（前提不成立）", "--solved-kind", "销案",
		"--before-read", brA, "--after-read", brB, "--yes"); rc != 0 {
		t.Fatalf("销案真跑要 0，得到 %d · %s", rc, e)
	}
	if r := gapRecOf(t, ledger, id1); r["solved_kind"] != "销案" {
		t.Errorf("① solved_kind 要 销案，得到 %v", r["solved_kind"])
	}

	// ② 缺省 ⇒ 未分类
	if rc, _, e := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--evidence", "这一条真修了（没给分栏）",
		"--before-read", brA, "--after-read", brB, "--yes"); rc != 0 {
		t.Fatalf("缺省真跑要 0，得到 %d · %s", rc, e)
	}
	if r := gapRecOf(t, ledger, id2); r["solved_kind"] != "未分类" {
		t.Errorf("② 缺省 solved_kind 要 未分类，得到 %v", r["solved_kind"])
	}

	// ⑥ 报数分栏（人面）
	rc, out, errb = gapRun(t, "gap", "ls", "--state", "已解")
	if rc != 0 {
		t.Fatalf("gap ls --state 已解 要 0，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(out, "已解分栏（solved_kind）：") || !strings.Contains(out, "销案 1") || !strings.Contains(out, "未分类 1") {
		t.Errorf("⑥ gap ls 人面没印分栏：%q", out)
	}
	// ⑥ 机器面
	rc, out, _ = gapRun(t, "gap", "ls", "--state", "已解", "--json", "id,solved_kind")
	if rc != 0 || !strings.Contains(out, "\"solved_kind\":\"销案\"") || !strings.Contains(out, "\"solved_kind\":\"未分类\"") {
		t.Errorf("⑥ gap ls --json 缺 solved_kind：rc=%d out=%q", rc, out)
	}

	// ⑦ 幂等：同 id 同态同证据同 kind ⇒ 0 且不写盘
	sha1, na1 := sha256Of(t, ledger), len(gapAuditLines(t, audit))
	rc, out, _ = gapRun(t, "gap", "set-state", id1, "--state", "已解", "--evidence", "这一条是销案（前提不成立）", "--solved-kind", "销案",
		"--before-read", brA, "--after-read", brB, "--yes")
	if rc != 0 || !strings.Contains(out, "无变化") {
		t.Errorf("⑦ 同态同证据同 kind 要 0「无变化」，得到 rc=%d out=%q", rc, out)
	}
	if sha256Of(t, ledger) != sha1 || len(gapAuditLines(t, audit)) != na1 {
		t.Errorf("⑦ 幂等命中却写了盘")
	}
}

// TestGapStateConcurrentWritesNoLostUpdate —— 缺口 `GAP-20261005-66` 的回归：
// 多路**不同 id** 的改态并发跑 ⇒ 「账中实落数 == 报 rc=0 的条数」（零静默丢更新），且 N 路应**全落**。
//
// 病（修前）：改态 = 读整件 → 改一行 → 乐观闸现读比对 → 审计先落盘（一次 fsync）→ 整件重写；
// 闸与重写**非原子**（中间恰夹着那次 fsync）⇒ 两写者都能过闸、后写整件覆盖先写 = **静默丢更新**。
// 治：账写排他锁（`flock(LOCK_EX)`）包住「锁内现读 + 按 id 重放」⇒ 每发都落到锁内新基线上。
// 本用例的判据就是那一句：**报 rc=0 的每一条都真在账里**（rc=0 却不在账 = 静默丢更新 ⇒ 判红）。
func TestGapStateConcurrentWritesNoLostUpdate(t *testing.T) {
	_, ledger, _ := gapEnv(t)
	const n = 8
	// 造 N 条**不同 id** 的账行（夹具合成 —— 与仓外 `race_repro.sh` 同形）。
	var b strings.Builder
	for k := 1; k <= n; k++ {
		rec := map[string]any{
			"id": fmt.Sprintf("GAP-20260101-%04d", k), "fp": fmt.Sprintf("%064d", k),
			"symptom": "并发夹具行", "handmade": "(夹具)", "impact": "命令面", "prio": "P1",
			"state": "仍缺", "want_family": "gap", "want_action": "ls",
			"repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls",
			"found_at": "2026-01-01T00:00:00+08:00", "unit": "无件(命令面)", "module": "无件", "unit_source": "hand",
		}
		line, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("夹具行序列化失败：%v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(ledger, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}

	// N 路并发改态（各改**不同** id ⇒ 修后应全落）。
	rcs := make([]int, n)
	var wg sync.WaitGroup
	for k := 0; k < n; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			id := fmt.Sprintf("GAP-20260101-%04d", k+1)
			rcs[k], _, _ = gapRun(t, "gap", "set-state", id, "--state", "不做", "--evidence", "并发夹具", "--yes")
		}(k)
	}
	wg.Wait()

	// 判据：报 rc=0 的条数 == 账中实落 `state=不做` 的条数（零丢更新）。
	rc0 := 0
	for _, rc := range rcs {
		if rc == 0 {
			rc0++
		}
	}
	present := 0
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源某行不是 JSON：%v", err)
		}
		if m["state"] == "不做" {
			present++
		}
	}
	if present != rc0 {
		t.Errorf("**静默丢更新**：报 rc=0 的 %d 条，账中实落 %d 条（差 %d 条 rc=0 却不在账）", rc0, present, rc0-present)
	}
	if rc0 != n {
		t.Errorf("N 路**不同 id** 应全落（修后口径）：rc=0 只 %d / %d（rc 表=%v）", rc0, n, rcs)
	}
}
