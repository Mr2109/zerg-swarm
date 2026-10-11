// family_model_pull_guard_decision_test.go —— `model pull` 写面拒执（缺 --yes）的**控制层判决格**（`meta.decision`）机检。
//
// 病：`cmdModelPull` 的 D2 缺 `--yes` 拒执（`error.detail=yes_required`）退码 2，而拒执包封的
// `meta.decision` 若缺席 ⇒ 与 `block` 档（值对不上 / `--dry-run` 与 `--yes` 同给）**同为 rc=2**、
// 只能靠 `detail` 暗指 —— 控制层一格缺口（同 `family_config.go` 第一批 · `guard_decision_test.go`）。
//
// 治法（落点 `family_model_pull.go` 第 189 行 yes_required 支）：紧跟 `inv.setErr(...)` 后补
// `guardMarkDecision(inv, guardDecisionApproval)` ⇒ 包封 `meta.decision=require_approval`。
//
// 负控：哑化该行（overlay 回无该行版本）⇒ 本件必红（`meta.decision` 缺席）。
package main

import (
	"bytes"
	"testing"
)

// TestModelPullGuardDecision_YesRequiredApproval —— `model pull` 缺 `--yes` 拒执要带控制层格
// `meta.decision=require_approval`（用法面走完名册判定、落到缺 --yes 那一档；不需真机 / 真模型目录）。
func TestModelPullGuardDecision_YesRequiredApproval(t *testing.T) {
	root := mpullFixture(t, "TestPullG", "msrc-guard", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)

	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"model", "pull"}}
	inv.args = []string{"TestPullG@msrc-guard"}
	inv.to = t.TempDir() + "/downloaded.gguf"
	rc := cmdModelPull(inv, &out, &errb)
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
