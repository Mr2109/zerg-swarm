// family_gap_reread_stale_test.go —— 「已修未销」单列（缺口 `GAP-20260929-12`）的**机检**。
//
// 口径（只读面 · 与 `family_gap_reread_test.go` 同一套夹具 helpers · 不复写）：
//
//	① 未闭条（state 仍缺）的 `verify_cmd` 现跑 rc==0 ⇒ **已修未销**（人面单列 + `--json stale=true`）
//	② 未闭条 `verify_cmd` 现跑 rc≠0 ⇒ **症状仍可复现**（✗ 不进已修未销 · `stale=false`）
//	③ 只读：本面一字节都不写真源（前后 sha256 逐字不变）
//	④ `gap regress` 同判：未闭条判据现跑通过 ⇒ 单列 + `meta.fixed_not_closed`
//
// 两态 / 负控（哑化）：不带 overlay 应**绿**（本件过）；带 overlay（把 `family_gap_reread.go` 映回
// 改前 / 把 `rcRun == 0` 判「已修未销」那一段删掉）应**红** —— 证这颗牙长在「改前 ⟷ 改后」的真实差上。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestGapRereadFixedNotClosed —— 点名档逐条现读未闭条：rc==0 ⇒ 已修未销；rc≠0 ⇒ 症状仍可复现。
func TestGapRereadFixedNotClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gapRereadOpenFixtureRec("GAP-20260929-99-A", "zerg gap ls"),            // 本账非空 ⇒ rc=0 ⇒ 已修未销
		gapRereadOpenFixtureRec("GAP-20260929-99-B", "zerg gap ls --state 不做"), // 本账零「不做」⇒ rc=1 ⇒ 仍可复现
	}
	ledger := gapRereadWriteLedger(t, dir, recs)
	sha0 := gapRereadFileSHA(t, ledger)

	// ① rc==0 ⇒ 已修未销（单列 · 点名到条）
	rc, out, errb := gapRereadRun(t, "GAP-20260929-99-A")
	if rc != 0 {
		t.Fatalf("rc==0 档要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "已修未销") {
		t.Errorf("rc==0 的未闭条要出「已修未销」单列：%q", out)
	}
	if !strings.Contains(out, "GAP-20260929-99-A") {
		t.Errorf("「已修未销」单列要点名到条：%q", out)
	}

	// ② rc!=0 ⇒ 症状仍可复现（**不许**进已修未销）
	rc, out, _ = gapRereadRun(t, "GAP-20260929-99-B")
	if rc != 0 {
		t.Errorf("rc!=0 档要 rc=0，得到 %d", rc)
	}
	if strings.Contains(out, "已修未销（单列") {
		t.Errorf("rc!=0 的未闭条**不许**进「已修未销」：%q", out)
	}

	// ③ `--json stale` 两态（A=true · B=false）
	rc, js, jerr := gapRereadRun(t, "GAP-20260929-99-A", "--json", "id,stale")
	if rc != 0 {
		t.Fatalf("`--json stale` 要 rc=0（改前该字段不存在 ⇒ rc=2），得到 %d · stderr=%q", rc, jerr)
	}
	if !strings.Contains(js, `"stale":"true"`) {
		t.Errorf("A（rc==0）的 `stale` 要为 true：%q", js)
	}
	rc, js, _ = gapRereadRun(t, "GAP-20260929-99-B", "--json", "id,stale")
	if rc != 0 || !strings.Contains(js, `"stale":"false"`) {
		t.Errorf("B（rc!=0）的 `stale` 要为 false：rc=%d %q", rc, js)
	}

	// ④ 只读：本面一字节都不写真源
	if got := gapRereadFileSHA(t, ledger); got != sha0 {
		t.Errorf("本面必须**只读**（真源 sha 变了）：%s → %s", sha0, got)
	}
}

// gap12RegressRec —— 造一条 regress 夹具账行（可指定 state/unit/verify_cmd）。
// ★ verify 用 `zerg gap ls --state 已解`（4 token ⇒ 非样板值 ⇒ 进「真判据」档；本账有 已解 条 ⇒ rc=0）。
func gap12RegressRec(id, state, unit, verify string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（只读对拍）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": state,
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": verify,
		"found_at": "2026-10-01T00:00:00+08:00", "unit": unit,
	}
}

// TestGapRegressFixedNotClosed —— `gap regress` 同判：未闭条判据现跑通过 ⇒ 单列 + `meta.fixed_not_closed`。
func TestGapRegressFixedNotClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		// 一条 已解 条（unit 空 ⇒ 不被点名件匹配）—— 让 `zerg gap ls --state 已解` 真跑 rc=0。
		gap12RegressRec("GAP-20260929-98-X", "已解", "", "zerg gap ls"),
		// 目标：仍缺 + 点名件 = family_gap.go + 判据 rc=0 ⇒ 已修未销。
		gap12RegressRec("GAP-20260929-98-A", "仍缺", gapRegressMetaUnit, "zerg gap ls --state 已解"),
	}
	ledger := gapRegressMetaWriteLedger(t, dir, recs)
	sha0 := gapRegressMetaFileSHA(t, ledger)

	var out, errb bytes.Buffer
	if rc := RunForTest([]string{"gap", "regress", gapRegressMetaUnit}, &out, &errb); rc != 0 {
		t.Fatalf("regress（真跑）要 rc=0，得到 %d · stderr=%q", rc, errb.String())
	}
	if !strings.Contains(out.String(), "已修未销") {
		t.Errorf("regress 要把「已修未销」单列：%q", out.String())
	}

	var jout, jerr bytes.Buffer
	if rc := RunForTest([]string{"gap", "regress", gapRegressMetaUnit, "--json", "file"}, &jout, &jerr); rc != 0 {
		t.Fatalf("regress --json 要 rc=0，得到 %d · stderr=%q", rc, jerr.String())
	}
	meta := gapRegressMetaOf(t, jout.String())
	if n, ok := meta["fixed_not_closed"].(float64); !ok || int(n) != 1 {
		t.Errorf("meta.fixed_not_closed 要为 1，得到 %v", meta["fixed_not_closed"])
	}

	if got := gapRegressMetaFileSHA(t, ledger); got != sha0 {
		t.Errorf("regress 必须**只读**（真源 sha 变了）：%s → %s", sha0, got)
	}
}
