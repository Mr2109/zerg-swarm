// family_sched_gate_guard_decision_test.go —— `sched` 写面闸（`schedConfirmGate`）缺 `--yes` 拒执的
// **控制层判决格**（`meta.decision`）机检。
//
// 病：`schedConfirmGate` 的 D2 缺 `--yes` 拒执（`error.detail=yes_required`）退码 2，而拒执包封的
// `meta.decision` 若缺席 ⇒ 与 `block` 档（值对不上 / `--dry-run` 与 `--yes` 同给）**同为 rc=2**、
// 只能靠 `detail` 暗指 —— 控制层一格缺口（同 `family_model_pull_guard_decision_test.go` · `guard_decision_test.go`）。
//
// 治法（落点 `family_sched.go` 的 `schedConfirmGate` yes_required 支）：紧跟 `inv.setErr(...)` 后补
// `guardMarkDecision(inv, guardDecisionApproval)` ⇒ 包封 `meta.decision=require_approval`。
//
// ★ 同函数 `confirm_mismatch` 支**不在此单**：那一条 detail **兼报两义**（缺 `--confirm` ⇒ require_approval /
// 值不匹配 ⇒ block），单值一格表达不了 ⇒ 保持原状（同族 `task cancel` 同形）。
//
// 负控：哑化该行（overlay 回无该行版本）⇒ 本件必红（`meta.decision` 缺席）。
package main

import (
	"bytes"
	"testing"
)

// TestSchedGateGuardDecision_YesRequiredApproval —— `sched` 写面闸缺 `--yes` 拒执要带控制层格
// `meta.decision=require_approval`（本机名由 `planHost()` 取；把它喂进 `--confirm` ⇒ 只在缺 --yes 那一档落地）。
func TestSchedGateGuardDecision_YesRequiredApproval(t *testing.T) {
	t.Setenv("ZERG_HOST", "schedgate-guard-dec.example.com")

	var errb bytes.Buffer
	inv := &invocation{path: []string{"sched", "pause"}}
	inv.confirm = planHost() // 本机名原值 —— `devEditHostAccept` 收
	inv.confirmGiven = true
	inv.yes = false

	rc := schedConfirmGate(inv, &errb, "sched pause")
	if rc != exitUsage {
		t.Fatalf("缺 --yes rc=%d（要 2 · fail-closed）· stderr=%q", rc, errb.String())
	}
	if inv.err == nil {
		t.Fatalf("缺 --yes 应落一枚错（setErr），却 inv.err=nil")
	}
	if inv.err.Detail != "yes_required" {
		t.Fatalf("error.detail=%q（要 yes_required —— 本单 ✗ 不改它）", inv.err.Detail)
	}
	got := ""
	if inv.err.Meta != nil {
		got = inv.err.Meta["decision"]
	}
	if got != guardDecisionApproval {
		t.Fatalf("缺 --yes 拒执 meta.decision=%q（要 %q）—— 控制层格缺席（本件要钉的牙）", got, guardDecisionApproval)
	}
}
