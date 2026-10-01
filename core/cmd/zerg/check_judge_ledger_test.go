// check_judge_ledger_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckJudgeLedgerGatePort（定向：go test -run TestCheckJudgeLedgerGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// jlgRowLine —— 造一条合法台账行（六字段齐 · 可覆写 raw_rc / 抹字段）。
func jlgRowLine(over map[string]string, drop ...string) string {
	m := map[string]string{
		"raw_rc": "0", "detail": "ok", "sha256": strings.Repeat("a", 64),
		"head": "6aa1120e", "judge_resolved": "true", "dry_run": "false",
	}
	for k, v := range over {
		m[k] = v
	}
	for _, d := range drop {
		delete(m, d)
	}
	// 固定键序（人读稳定 · 与 py `_row` 同键集）。
	order := []string{"raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"}
	seen := map[string]bool{}
	emit := func(k, v string) string {
		seen[k] = true
		if k == "raw_rc" || v == "true" || v == "false" {
			return `"` + k + `": ` + v
		}
		return `"` + k + `": "` + v + `"`
	}
	var parts []string
	for _, k := range order {
		if v, ok := m[k]; ok {
			parts = append(parts, emit(k, v))
		}
	}
	var extra []string
	for k := range m {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		parts = append(parts, emit(k, m[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func jlgWriteLedger(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "judge-run-ledger.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func jlgRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := judgeLedgerGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckJudgeLedgerGatePort(t *testing.T) {
	d := t.TempDir()

	// ── 正控（必然判绿）：两行合法（六字段齐）⇒ rc=0 ─────────────────────
	green := jlgWriteLedger(t, d, jlgRowLine(nil)+"\n"+jlgRowLine(map[string]string{"raw_rc": "1", "detail": "judge_red"})+"\n")
	rcGreen, outGreen, _ := jlgRun(t, "--check", "--ledger", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "判据运行台账对拍绿") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：缺 `detail` ⇒ rc=1 ────────────────────────────
	miss := jlgWriteLedger(t, d, jlgRowLine(nil, "detail")+"\n")
	rcRed1, _, errRed1 := jlgRun(t, "--check", "--ledger", miss)
	if rcRed1 != 1 || !strings.Contains(errRed1, "缺必填字段 `detail`") {
		t.Fatalf("负控-缺 detail 失败：得 rc=%d stderr=%q", rcRed1, errRed1)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRed1 {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRed1)
	}

	// ── 负控 · `raw_rc` 被 `rc` 顶替（硬边）⇒ rc=1 ──────────────────────
	rep := jlgWriteLedger(t, d, jlgRowLine(map[string]string{"rc": "0"}, "raw_rc")+"\n")
	rcRep, _, errRep := jlgRun(t, "--check", "--ledger", rep)
	if rcRep != 1 || !strings.Contains(errRep, "**`raw_rc` 被顶替**") || !strings.Contains(errRep, "用 rc 顶格") {
		t.Fatalf("负控-顶替 失败：得 rc=%d stderr=%q", rcRep, errRep)
	}

	// ── 负控 · `raw_rc` 不是整数（1.5）⇒ rc=1 ───────────────────────────
	frac := jlgWriteLedger(t, d, jlgRowLine(map[string]string{"raw_rc": "1.5"})+"\n")
	rcFrac, _, errFrac := jlgRun(t, "--check", "--ledger", frac)
	if rcFrac != 1 || !strings.Contains(errFrac, "`raw_rc` 不是整数：1.5") {
		t.Fatalf("负控-非整数 失败：得 rc=%d stderr=%q", rcFrac, errFrac)
	}

	// ── 负控 · `judge_resolved` 不是布尔 ⇒ rc=1 ─────────────────────────
	jr := jlgWriteLedger(t, d, jlgRowLine(map[string]string{"judge_resolved": "1"})+"\n")
	rcJR, _, errJR := jlgRun(t, "--check", "--ledger", jr)
	if rcJR != 1 || !strings.Contains(errJR, "`judge_resolved` 不是布尔：'1'") {
		t.Fatalf("负控-judge_resolved 失败：得 rc=%d stderr=%q", rcJR, errJR)
	}

	// ── 负控 · 缺两字段（sha256 / head）⇒ 各报一条 ──────────────────────
	two := jlgWriteLedger(t, d, jlgRowLine(nil, "sha256", "head")+"\n")
	rcTwo, _, errTwo := jlgRun(t, "--check", "--ledger", two)
	if rcTwo != 1 || !strings.Contains(errTwo, "缺必填字段 `sha256`") || !strings.Contains(errTwo, "缺必填字段 `head`") {
		t.Fatalf("负控-缺两字段 失败：得 rc=%d stderr=%q", rcTwo, errTwo)
	}

	// ── 不给结论（≠ 绿）：空台账（空件/无有效行）⇒ rc=2 ─────────────────
	empty := jlgWriteLedger(t, d, "")
	rcEmpty, _, errEmpty := jlgRun(t, "--check", "--ledger", empty)
	if rcEmpty != 2 || !strings.Contains(errEmpty, "空件/无有效行") {
		t.Fatalf("空台账失败：得 rc=%d stderr=%q", rcEmpty, errEmpty)
	}

	// ── 不给结论：台账不存在 ⇒ rc=2 ─────────────────────────────────────
	rcAbsent, _, errAbsent := jlgRun(t, "--check", "--ledger", filepath.Join(d, "absent.jsonl"))
	if rcAbsent != 2 || !strings.Contains(errAbsent, "台账不存在") {
		t.Fatalf("台账不存在失败：得 rc=%d stderr=%q", rcAbsent, errAbsent)
	}

	// ── append-only：基线旧行被改 ⇒ rc=1 ──────────────────────────────
	db := filepath.Join(d, "bl1")
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}
	base := jlgWriteLedger(t, db, jlgRowLine(map[string]string{"sha256": strings.Repeat("b", 64)})+"\n")
	led2 := jlgWriteLedger(t, d, jlgRowLine(map[string]string{"sha256": strings.Repeat("c", 64)})+"\n")
	rcAnn, _, errAnn := jlgRun(t, "--check", "--ledger", led2, "--baseline", base)
	if rcAnn != 1 || !strings.Contains(errAnn, "append-only 破") {
		t.Fatalf("append-only 破失败：得 rc=%d stderr=%q", rcAnn, errAnn)
	}

	// ── append-only：基线旧行仍在（含新行）⇒ rc=0（防恒红） ─────────────
	db2 := filepath.Join(d, "bl2")
	if err := os.MkdirAll(db2, 0o755); err != nil {
		t.Fatal(err)
	}
	baseKeep := jlgWriteLedger(t, db2, jlgRowLine(nil)+"\n")
	ledKeep := jlgWriteLedger(t, d, jlgRowLine(nil)+"\n"+jlgRowLine(map[string]string{"raw_rc": "1"})+"\n")
	rcKeep, outKeep, _ := jlgRun(t, "--check", "--ledger", ledKeep, "--baseline", baseKeep)
	if rcKeep != 0 || !strings.Contains(outKeep, "append-only 成立（基线") {
		t.Fatalf("append-only 成立失败：得 rc=%d out=%q", rcKeep, outKeep)
	}

	// ── 用法错（不给结论）：--check / --self-test 二选一都不给 ⇒ rc=2 ───
	rcUse, _, errUse := jlgRun(t)
	if rcUse != 2 || !strings.Contains(errUse, "二选一必给一个") {
		t.Fatalf("用法错失败：得 rc=%d stderr=%q", rcUse, errUse)
	}

	// ── --self-test 成对负控自身必须全过（防恒绿/恒红） ───────────────────
	rcST, outST, _ := jlgRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "负控①（缺 `detail` 字段）：现 rc=1") ||
		!strings.Contains(outST, "负控③（空台账）：现 rc=2") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
