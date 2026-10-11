// guard_decision_test.go —— 写面闸拒执的**控制层判决格**（`meta.decision`）机检。
//
// 病（本件要钉死的那一格）：写面闸拒执分**两档** —— `block`（硬拦：`--confirm` 值对不上 /
// `--dry-run` 与 `--yes` 同给）与 `require_approval`（等一次显式许可：缺 `--confirm` / 缺 `--yes`）——
// 而两档**退码同为 2**、既有 `error.detail` 各是**原因码**（`confirm_mismatch` / `dry_run_yes_conflict`
// vs `confirm_required` / `yes_required`）⇒ 光看退码 / 原因码分不出「硬拦」还是「需人签放行」。
//
// 治法（落点 `guard.go` 的 `guardMarkDecision`）：拒执包封的 `meta.decision` 逐字取
// `core/internal/contract/authority-map.json` 的 `控制层闭集`（`block` / `require_approval`）。
//
// 三条判据（可机检 · 成对）：
//
//	① block 档（`--dry-run` + `--yes` 同给）⇒ rc=2 且 `meta.decision=block`；
//	② require_approval 档（D2 缺 `--yes`）⇒ rc=2 且 `meta.decision=require_approval`；
//	③ 两档 rc 同（2）而 `meta.decision` **不同值** ⇒ 可分（这一格就是本件的牙）。
//	④ 成对第二组：`confirm_mismatch`（block · 值对不上）⟷ `yes_required`（require_approval · 缺许可）。
//
// 负控（哑化 `guardMarkDecision` ⇒ 两档 `meta.decision` 都缺席）⇒ 本件必红（见本单「验牙」读数）。
package main_test

import (
	"encoding/json"
	"testing"
)

// wfGuardDecision 跑一条 `--json` 命令，取退码 + `error.detail` + 包封 `meta.decision`。
func wfGuardDecision(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	rc, out, _ := runCapture(argv...)
	if out == "" {
		return rc, "", ""
	}
	var doc struct {
		Meta struct {
			Decision string `json:"decision"`
		} `json:"meta"`
		Err struct {
			Detail string `json:"detail"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("包封不是 JSON：%v · out=%q", err, out)
	}
	return rc, doc.Err.Detail, doc.Meta.Decision
}

// TestWriteFaceGuardDecision_BlockVsApproval —— 两档 rc 同为 2，而 `meta.decision` 取值不同 ⇒ 可分。
func TestWriteFaceGuardDecision_BlockVsApproval(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))

	// ① block 档：`--dry-run` 与 `--yes` 同给（两道确认档自相矛盾）。
	rcB, detB, decB := wfGuardDecision(t, "model", "opts", "set", "m1", "--set", "temperature=0.8", "--dry-run", "--yes", "--json", "model")
	if rcB != 2 {
		t.Fatalf("block 档退码 rc=%d（要 2）· detail=%q", rcB, detB)
	}
	if decB != "block" {
		t.Fatalf("block 档 meta.decision=%q（要 block）· detail=%q", decB, detB)
	}
	// ② require_approval 档：D2 缺 `--yes` ⇒ 等一次显式许可。
	rcA, detA, decA := wfGuardDecision(t, "model", "opts", "set", "m1", "--set", "temperature=0.8", "--json", "model")
	if rcA != 2 {
		t.Fatalf("require_approval 档退码 rc=%d（要 2）· detail=%q", rcA, detA)
	}
	if decA != "require_approval" {
		t.Fatalf("require_approval 档 meta.decision=%q（要 require_approval）· detail=%q", decA, detA)
	}
	// ③ 两档 rc 同（皆为 2），而 decision 不同 ⇒ 可分（本件的牙）。
	if rcB == rcA && decB == decA {
		t.Fatalf("两档 rc 同为 %d 且 meta.decision 同值（%q）⇒ 不可分", rcB, decB)
	}
}

// TestWriteFaceGuardDecision_MismatchVsMissingYes —— 成对第二组：值对不上（block）⟷ 缺许可（require_approval）。
func TestWriteFaceGuardDecision_MismatchVsMissingYes(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))

	rcM, detM, decM := wfGuardDecision(t, "model", "add", "--model", "m-x", "--host", "Mr2109", "--file", "/models/m-x.gguf", "--confirm", "wrong", "--yes", "--json", "model")
	if rcM != 2 || decM != "block" {
		t.Fatalf("confirm_mismatch 档 rc=%d meta.decision=%q detail=%q（要 2 / block）", rcM, decM, detM)
	}
	rcY, detY, decY := wfGuardDecision(t, "model", "add", "--model", "m-x", "--host", "Mr2109", "--file", "/models/m-x.gguf", "--confirm", "m-x", "--json", "model")
	if rcY != 2 || decY != "require_approval" {
		t.Fatalf("yes_required 档 rc=%d meta.decision=%q detail=%q（要 2 / require_approval）", rcY, decY, detY)
	}
	if decM == decY {
		t.Fatalf("成对两档 meta.decision 同值（%q）⇒ 不可分", decM)
	}
}
