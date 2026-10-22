// family_gap_setwant_tiers_test.go —— 缺口 `GAP-20261008-07`（want 无 CLI 写面）的**第二组牙**（本单新增件）。
//
// 为什么另起一件：`family_gap_setwant_test.go` 已有〔甲〕两态逐字 / 〔乙〕缺 `--yes` fail-closed /
// 〔丙〕越界闭集 / 〔丁〕`--dry-run` 与形状纯度 / 〔戊〕闭集面 / 〔己〕取证件六组；本件只补**本单点名要
// 而前件没有**的三组，✗ 不动前件一字（前件在两态/闭集上有自己的取舍，改它会动别人的锚）：
//
//	〔庚〕旧值 → 新值**逐字**（族与动作同时变 · 只改动作时族报「逐字未动」· `--json` 回读同一份真值）
//	〔辛〕**只读借用档硬拒 rc=2**（`ZERG_GAP_LEDGER_BORROW`）—— 借源零新增文件（连锁件一枚不造）·
//	     借源与沙箱两份账**都逐字节不变** · 审计零行（写面没有落点，✗ 不许换落到沙箱那份）
//	〔壬〕**缺参数 rc=2**（`--want-family` / `--want-action` / `--evidence` / 两枚同缺）——点名缺哪枚、
//	     给用法串、账一字不动、审计不落盘
//
// ★ 写面落点口径（本单要求②）：真写一律走族内唯一收口 `gapRewriteOne` ⇒ 它里面的写前取锁走
// **写面落点函数 `gapLedgerWritePath`**（不是读面 `gapLedgerPath`）。〔辛〕正是这一条的**负控**：
// 借用档在册 ⇒ 写面无落点 ⇒ 取锁之前就退 2（借源旁边连 0 字节 `.lock` 都不许出现）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 〔庚〕旧值 → 新值逐字 ──
// 夹具（`gapSetWantTestState`）：GAP-20261008-07 = want_family `gap` · want_action `set-state`。
func TestGapSetWantOldToNewVerbatim(t *testing.T) {
	gapSetWantTestInstall(t)
	ledger := gapSetWantTestState(t)

	// ① 族与动作**同时**变（gap/set-state → gate/set-want）
	rc, out, errb := gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gate", "--want-action", "set-want",
		"--evidence", "拍板：want 换到门禁族", "--yes")
	if rc != 0 {
		t.Fatalf("改 want 应 rc=0（实测 rc=%d · stdout=%q · stderr=%q）", rc, out, errb)
	}
	// 回执：两格都要按「旧 → 新」逐字给出（这就是「两态回执」的写面那一半）
	if !strings.Contains(out, "want_family : gap → gate") {
		t.Errorf("回执要逐字给「want_family : gap → gate」（stdout=%q）", out)
	}
	if !strings.Contains(out, "want_action : set-state → set-want") {
		t.Errorf("回执要逐字给「want_action : set-state → set-want」（stdout=%q）", out)
	}
	if !strings.Contains(out, "state       : 仍缺（逐字未动）") {
		t.Errorf("回执要点名 `state` 逐字未动（stdout=%q）", out)
	}
	// 账内逐字
	row := gapSetWantTestRow(t, ledger, "GAP-20261008-07")
	if row["want_family"] != "gate" {
		t.Errorf("want_family 应逐字为 gate（got=%v）", row["want_family"])
	}
	if row["want_action"] != "set-want" {
		t.Errorf("want_action 应逐字为 set-want（got=%v）", row["want_action"])
	}
	if row["state"] != "仍缺" {
		t.Errorf("state 不许动（got=%v）", row["state"])
	}

	// ② 只改动作（族同值）⇒ 回执说族「逐字未动」，动作仍逐字给旧 → 新
	rc, out, errb = gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gate", "--want-action", "ls",
		"--evidence", "拍板：动作收窄到 ls", "--yes")
	if rc != 0 {
		t.Fatalf("只改动作应 rc=0（实测 rc=%d · stderr=%q）", rc, errb)
	}
	if !strings.Contains(out, "want_family : gate（逐字未动）") {
		t.Errorf("族同值时要逐字报「逐字未动」（stdout=%q）", out)
	}
	if !strings.Contains(out, "want_action : set-want → ls") {
		t.Errorf("动作要逐字给「set-want → ls」（stdout=%q）", out)
	}
	row = gapSetWantTestRow(t, ledger, "GAP-20261008-07")
	if row["want_family"] != "gate" || row["want_action"] != "ls" {
		t.Errorf("账内应为 gate/ls（got=%v/%v）", row["want_family"], row["want_action"])
	}
	// 留痕注：三条改口径各一条（1 原始 + 2 本件）
	notes, _ := row["notes"].([]any)
	if len(notes) != 3 {
		t.Fatalf("notes 应 1 → 3 条（got=%d）", len(notes))
	}
	if last, _ := notes[2].(string); !strings.Contains(last, "want_action: set-want → ls") ||
		!strings.Contains(last, "拍板：动作收窄到 ls") {
		t.Errorf("留痕注要含「旧 → 新」与证据原文（got=%v）", notes[2])
	}

	// ③ `--json` 回读同一份真值（写面字段面与真源不许两套说法）
	rc, out, errb = gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gate", "--want-action", "set-want",
		"--evidence", "拍板：动作回到 set-want", "--yes",
		"--json", "id,want_family,want_action,changed")
	if rc != 0 {
		t.Fatalf("--json 写面应 rc=0（实测 rc=%d · stderr=%q）", rc, errb)
	}
	// `--json` 走族内统一信封（`items[]` + `meta`）—— 取 `items[0]`，与真源逐格对拍。
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("--json 输出不是 JSON（stdout=%q · err=%v）", out, err)
	}
	if len(env.Items) != 1 {
		t.Fatalf("--json 应恰好 1 个 item（got=%d · stdout=%q）", len(env.Items), out)
	}
	got := env.Items[0]
	if got["id"] != "GAP-20261008-07" || got["want_family"] != "gate" ||
		got["want_action"] != "set-want" || got["changed"] != "true" {
		t.Errorf("--json 应回读真值（got=%v）", got)
	}
	row = gapSetWantTestRow(t, ledger, "GAP-20261008-07")
	if row["want_action"] != "set-want" {
		t.Errorf("--json 那一发要真落账（got=%v）", row["want_action"])
	}
}

// ── 〔辛〕只读借用档硬拒 rc=2 ──
// 判据三面：① rc=2 且点名「借用」；② **借源**逐字节不变 + 目录里零新增文件（连锁件都不造）；
// ③ **沙箱**那份账也一字不动（写面没有落点 ⇒ 不许「悄悄改落到沙箱」）+ 审计零行。
func TestGapSetWantBorrowTierRefused(t *testing.T) {
	gapSetWantTestInstall(t)
	// `gapSetWantTestState` 返回的**就是**账件路径（不是目录）。
	sandbox := gapSetWantTestState(t)
	sbxBefore, err := os.ReadFile(sandbox)
	if err != nil {
		t.Fatalf("读沙箱账失败：%v", err)
	}

	// 借源：另一目录里的一件真源快照（`ZERG_GAP_LEDGER_BORROW=<绝对件>`）
	borrowDir := t.TempDir()
	borrowed := filepath.Join(borrowDir, "zerg-cli-gaps.jsonl")
	if err := os.WriteFile(borrowed, sbxBefore, 0o644); err != nil {
		t.Fatalf("造借源失败：%v", err)
	}
	t.Setenv(gapBorrowEnv, borrowed)

	rc, out, errb := gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gate", "--want-action", "ls",
		"--evidence", "借用档下试写（应被拒）", "--yes")
	if rc != 2 {
		t.Fatalf("借用档下写面必须硬拒 rc=2（实测 rc=%d · stdout=%q · stderr=%q）", rc, out, errb)
	}
	if !strings.Contains(errb, "借用") {
		t.Errorf("拒词要点名「借用」（stderr=%q）", errb)
	}
	if !strings.Contains(errb, "退码 2") {
		t.Errorf("拒词要给出退码（stderr=%q）", errb)
	}
	t.Logf("借用档拒执逐字（stderr）：%q", errb)

	// ② 借源逐字节不变 + 零新增文件（`.lock` 一枚都不许出现）
	after, err := os.ReadFile(borrowed)
	if err != nil {
		t.Fatalf("借源读不回来（被动了？）：%v", err)
	}
	if !bytes.Equal(sbxBefore, after) {
		t.Errorf("借源必须逐字节不变（%d → %d 字节）", len(sbxBefore), len(after))
	}
	ents, err := os.ReadDir(borrowDir)
	if err != nil {
		t.Fatalf("列借源目录失败：%v", err)
	}
	if len(ents) != 1 {
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("借源目录必须零新增文件（写面取锁走 `gapLedgerWritePath` ⇒ 无落点 ⇒ 取锁之前就拒），实得 %d 枚：%v",
			len(ents), names)
	}

	// ③ 沙箱那份账与审计：写面没有落点 ⇒ 也不许改落到沙箱
	sbxAfter, err := os.ReadFile(sandbox)
	if err != nil {
		t.Fatalf("读沙箱账失败：%v", err)
	}
	if !bytes.Equal(sbxBefore, sbxAfter) {
		t.Errorf("借用档下沙箱账也必须一字不动（写面无落点，✗ 不许换落点）")
	}
	if _, err := os.Stat(os.Getenv("ZERG_EDIT_AUDIT")); err == nil {
		t.Errorf("借用档被拒时审计不许落盘")
	}
}

// ── 〔壬〕缺参数 ⇒ rc=2（点名缺哪枚 + 给用法串 + 账一字不动 + 审计不落盘）──
func TestGapSetWantMissingRequiredFlags(t *testing.T) {
	gapSetWantTestInstall(t)
	ledger := gapSetWantTestState(t)
	before, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("读账失败：%v", err)
	}

	cases := []struct {
		name  string
		argv  []string
		want  string
		usage bool // 缺 want 两格那一支会一并打用法串（`--evidence` 那一支只点名+纪律，不打用法）
	}{
		{"缺 --want-family", []string{"gap", "set-want", "GAP-20261008-07",
			"--want-action", "ls", "--evidence", "e", "--yes"}, "--want-family", true},
		{"缺 --want-action", []string{"gap", "set-want", "GAP-20261008-07",
			"--want-family", "gap", "--evidence", "e", "--yes"}, "--want-action", true},
		{"缺 --evidence", []string{"gap", "set-want", "GAP-20261008-07",
			"--want-family", "gap", "--want-action", "ls", "--yes"}, "--evidence", false},
		{"两枚同缺", []string{"gap", "set-want", "GAP-20261008-07",
			"--evidence", "e", "--yes"}, "--want-family · --want-action", true},
	}
	for _, c := range cases {
		rc, _, errb := gapSetWantTestRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s：应 rc=2（实测 rc=%d · stderr=%q）", c.name, rc, errb)
			continue
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：拒词要**逐字点名** %q（stderr=%q）", c.name, c.want, errb)
		}
		if c.usage && !strings.Contains(errb, "zerg gap set-want <GAP id> --want-family") {
			t.Errorf("%s：要一并给用法串（stderr=%q）", c.name, errb)
		}
	}
	after, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("读账失败：%v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("缺参数被拒时账必须一字不动")
	}
	if _, err := os.Stat(os.Getenv("ZERG_EDIT_AUDIT")); err == nil {
		t.Errorf("缺参数被拒时审计不许落盘")
	}
}
