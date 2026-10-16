// guard_decision_test.go —— 写面闸拒执的**控制层判决格**机检（`block` ⟷ `require_approval`）。
//
// 病（本件要钉死的那一格）：写面闸拒执分**两档** —— `block`（硬拦：`--confirm` 值对不上 /
// `--dry-run` 与 `--yes` 同给）与 `require_approval`（等一次显式许可：缺 `--confirm` / 缺 `--yes`）——
// 而两档**退码同为 2**、既有 `error.detail` 各是**原因码**（`confirm_mismatch` / `dry_run_yes_conflict`
// vs `confirm_required` / `yes_required`）。
//
// ★★ 缺口 `GAP-20260929-41`（本批）把读面改了：`kind=usage` 的失败**不吐 stdout 包封**
//
//	（用法错 stdout 零字节 · 报错全在 stderr）⇒ 原先读包封 `meta.decision` 的那一格**不再是可观察面**
//	（`guardMarkDecision` 仍把 `decision` 挂在 `inv.err.Meta` 上，但那一块只在非 usage 的失败包封里印）。
//	本件随之把**观测面**搬到 stderr（`guard.go` 各拒执支逐字打印的那几行判词 / 机器面读法行）——
//	判据强度不降反升：
//	① 两档 rc 同为 2 而 stderr 的**判别词逐字不同**（`dry_run_yes_conflict` / `确认值不匹配`
//	   ⟷ `缺 --yes`）⇒ 可分（这正是本件的牙）；
//	② **stdout 必须零字节**（口径本身就是判据：usage 错的包封若又冒出来 ⇒ 立红）；
//	③ 交叉负控：block 档的判别词**不许**出现在 require_approval 档上，反之亦然
//	   （两档 stderr 分不开 ⇒ 判口退回「rc 一刀切」= 没牙）。
//	★ 与同族先例同口径：`cli_authority_map_test.go` 的判口逐字写着「`block` 与 `require_approval`
//	**同为 rc=2**，光看 rc 分不开 ⇒ 判口必须落到 stderr 的 `detail`/缺失旗标上」，本件用的就是同一批判别词。
//
// 负控（哑化那些拒执支 / 把两档的判词改成同一句）⇒ 本件必红（见本单「验牙」读数）。
package main_test

import (
	"strings"
	"testing"
)

// wfGuardDecision 跑一条写面拒执命令，取 退码 · stdout · stderr（三面都要）。
func wfGuardDecision(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	return runCapture(argv...)
}

// 该档的**判别词**（逐字取 `guard.go` 各拒执支打印的那一行，与 `cli_authority_map_test.go`
// 的见证词同一批）。
const (
	wfNeedleBlockConflict = "dry_run_yes_conflict" // block：两道确认档自相矛盾（机器面读法行）
	wfNeedleBlockMismatch = "不一致"                  // block：`--confirm` 值对不上目标
	wfNeedleApproval      = "缺 --yes"              // require_approval：等一次显式许可
)

// TestWriteFaceGuardDecision_BlockVsApproval —— 两档 rc 同为 2，而 stderr 判别词不同 ⇒ 可分。
func TestWriteFaceGuardDecision_BlockVsApproval(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))

	// ① block 档：`--dry-run` 与 `--yes` 同给（两道确认档自相矛盾）。
	rcB, outB, errB := wfGuardDecision(t, "model", "opts", "set", "m1", "--set", "temperature=0.8", "--dry-run", "--yes", "--json", "model")
	if rcB != 2 {
		t.Fatalf("block 档退码 rc=%d（要 2）· stderr=%q", rcB, errB)
	}
	if !strings.Contains(errB, wfNeedleBlockConflict) {
		t.Fatalf("block 档 stderr 没点名 %q（判口要看 stderr —— 两档 rc 同为 2 分不开）：%q", wfNeedleBlockConflict, errB)
	}
	if strings.Contains(errB, wfNeedleApproval) {
		t.Fatalf("block 档 stderr 也带 require_approval 的 %q ⇒ 两档 stderr 分不开：%q", wfNeedleApproval, errB)
	}
	if outB != "" {
		t.Fatalf("block 档（用法错）stdout 必须零字节（GAP-20260929-41），得到 %q", outB)
	}
	// ② require_approval 档：D2 缺 `--yes` ⇒ 等一次显式许可。
	rcA, outA, errA := wfGuardDecision(t, "model", "opts", "set", "m1", "--set", "temperature=0.8", "--json", "model")
	if rcA != 2 {
		t.Fatalf("require_approval 档退码 rc=%d（要 2）· stderr=%q", rcA, errA)
	}
	if !strings.Contains(errA, wfNeedleApproval) {
		t.Fatalf("require_approval 档 stderr 没点名 %q：%q", wfNeedleApproval, errA)
	}
	if strings.Contains(errA, wfNeedleBlockConflict) {
		t.Fatalf("require_approval 档 stderr 也带 block 的 %q ⇒ 两档 stderr 分不开：%q", wfNeedleBlockConflict, errA)
	}
	if outA != "" {
		t.Fatalf("require_approval 档（用法错）stdout 必须零字节（GAP-20260929-41），得到 %q", outA)
	}
	// ③ 两档 rc 同（皆为 2），而判别词不同 ⇒ 可分（本件的牙）。
	if rcB == rcA && !strings.Contains(errB, wfNeedleBlockConflict) && !strings.Contains(errA, wfNeedleApproval) {
		t.Fatalf("两档 rc 同为 %d 且 stderr 判别词都不在 ⇒ 不可分", rcB)
	}
}

// TestWriteFaceGuardDecision_MismatchVsMissingYes —— 成对第二组：值对不上（block）⟷ 缺许可（require_approval）。
func TestWriteFaceGuardDecision_MismatchVsMissingYes(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))

	rcM, outM, errM := wfGuardDecision(t, "model", "add", "--model", "m-x", "--host", "Mr2109", "--file", "/models/m-x.gguf", "--confirm", "wrong", "--yes", "--json", "model")
	if rcM != 2 || !strings.Contains(errM, wfNeedleBlockMismatch) {
		t.Fatalf("confirm_mismatch 档 rc=%d · stderr 没点名 %q（要 2 / block）：%q", rcM, wfNeedleBlockMismatch, errM)
	}
	if strings.Contains(errM, wfNeedleApproval) {
		t.Fatalf("confirm_mismatch 档 stderr 也带 %q ⇒ 两档 stderr 分不开：%q", wfNeedleApproval, errM)
	}
	if outM != "" {
		t.Fatalf("confirm_mismatch 档（用法错）stdout 必须零字节（GAP-20260929-41），得到 %q", outM)
	}
	rcY, outY, errY := wfGuardDecision(t, "model", "add", "--model", "m-x", "--host", "Mr2109", "--file", "/models/m-x.gguf", "--confirm", "m-x", "--json", "model")
	if rcY != 2 || !strings.Contains(errY, wfNeedleApproval) {
		t.Fatalf("yes_required 档 rc=%d · stderr 没点名 %q（要 2 / require_approval）：%q", rcY, wfNeedleApproval, errY)
	}
	if strings.Contains(errY, wfNeedleBlockMismatch) {
		t.Fatalf("yes_required 档 stderr 也带 block 的 %q ⇒ 两档 stderr 分不开：%q", wfNeedleBlockMismatch, errY)
	}
	if outY != "" {
		t.Fatalf("yes_required 档（用法错）stdout 必须零字节（GAP-20260929-41），得到 %q", outY)
	}
}
