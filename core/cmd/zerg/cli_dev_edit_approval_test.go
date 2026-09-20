// cli_dev_edit_approval_test.go —— `zerg dev edit` 的**人签批准件**闸（主体③-a · 2026-09-21）。
//
// 本件判据四条（**全部在合成仓根 + 合成状态目录里跑**，不碰真仓的件）：
//
//	① 真写**没有批准件** ⇒ 退 2，且**件一个字节都没动**（fail-closed 的那一格）；
//	② **手写件**（字段齐、没签名）⇒ 退 2（「无签名（不算批准）」逐字给出来）；
//	③ 真签名件但**范围不含**本次改件 ⇒ 退 2（范围只认 `*` / 逐字同路径 / `目录/` 前缀）；
//	④ 真签名件 + 范围对上 ⇒ 退 0、件按 `--from` 写成、审计里能回读到**批准人与批准件路径**。
//
// 判据的四枚钥匙都由本件自己造（Ed25519 + `control.ApprovalPayload`/`KeyID`/`VerifyApprovalSignature`
// 是**消费者同一条路**）：私钥不留盘、公钥落 `operator.pub` —— 与 `approve keygen` 的产物同格式。
package main_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// hostnameOf 本机名（`dev edit` 的 `--confirm` 值必须与它逐字相同 —— 与 `planHost()` 同源）。
func hostnameOf(t *testing.T) string {
	t.Helper()
	h, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// synthRepoForEdit 建一个合成仓根（判据件 + 一件待改的件），返回 (仓根, 目标件相对路径)。
func synthRepoForEdit(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	target := filepath.Join("core", "cmd", "zerg", "target.go")
	mustWrite(t, filepath.Join(root, target), "package main\n\n// 原件\n")
	return root, target
}

// 合成提案件（声明它要改的那一件）——`dev edit` 的作用域真源。
func writeEditProposal(t *testing.T, dir, id, target string) {
	t.Helper()
	rec := map[string]any{
		"id": id, "title": "合成提案（人签批准件的判据夹具）", "target": "D3b",
		"goal": "g", "evidence": []string{"e"}, "rollback_ref": "r",
		"criterion": "zerg gate run --fast", "files": []string{target},
		"by": "夹具", "subject": "", "subject_kind": "human", "state": "未决",
		"created_at": "2026-09-21T00:00:00+08:00",
	}
	b, _ := json.MarshalIndent(rec, "", " ")
	mustWrite(t, filepath.Join(dir, id+".json"), string(b)+"\n")
}

// writeOperatorKey 造一枚操作员密钥对（私钥只在本函数里活着）；返回签发函数。
func writeOperatorKey(t *testing.T, state string) func(tool, scope, approver, at, note string) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	pf := map[string]string{"alg": "ed25519", "pub": pubB64, "key_id": control.KeyID(pubB64), "at": "2026-09-21T00:00:00+08:00"}
	b, _ := json.MarshalIndent(pf, "", " ")
	mustWrite(t, filepath.Join(state, "approvals", "operator.pub"), string(b)+"\n")
	return func(tool, scope, approver, at, note string) []byte {
		tk := map[string]string{
			"tool": tool, "approver": approver, "approved_at": at, "scope": scope, "note": note,
			"sig_alg": "ed25519", "key_id": control.KeyID(pubB64),
			"sig": base64.StdEncoding.EncodeToString(
				ed25519.Sign(priv, control.ApprovalPayload(tool, scope, approver, at, note))),
		}
		out, _ := json.MarshalIndent(tk, "", " ")
		return append(out, '\n')
	}
}

// runDevEdit 进程内跑一条 `dev edit`（env 全部隔离到合成根/合成状态目录）。
func runDevEdit(t *testing.T, repo, prop, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_PROPOSAL_DIR", prop)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(append([]string{"dev", "edit"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

func TestDevEdit_RealWriteNeedsHumanApproval(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9001", target)
	sign := writeOperatorKey(t, state) // 钥匙在，但**没签任何件**
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 被改后的内容\n")

	// ① 无件 ⇒ 拒
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 {
		t.Fatalf("真写没有批准件 ⇒ 退 2（fail-closed），得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "真写要一枚人签批准件") || !strings.Contains(errb, "approve new --tool dev_edit") {
		t.Errorf("要给拒因 + 下一步（怎么签那枚件）：%q", errb)
	}
	got, _ := os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "// 原件") {
		t.Errorf("被拒的那一格里件**一个字节都不许动**：%q", got)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("被拒 ⇒ 连审计都不该落（没写就不记账）")
	}

	// ② 手写件（字段齐、无签名）⇒ 拒
	hand := map[string]string{"tool": "dev_edit", "approver": "手写", "approved_at": "2026-09-21T00:00:00+08:00",
		"scope": "*", "note": "手写的"}
	hb, _ := json.MarshalIndent(hand, "", " ")
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"), string(hb)+"\n")
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 || !strings.Contains(errb, "无签名（不算批准）") {
		t.Errorf("手写件 ⇒ 拒且逐字给出「无签名（不算批准）」：rc=%d stderr=%q", rc, errb)
	}

	// ③ 真签名件但范围不含本次改件 ⇒ 拒
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "core/别的目录/", "张三", "2026-09-21T07:00:00+08:00", "批的是别处"))+"\n")
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 || !strings.Contains(errb, "不含本次改的件") {
		t.Errorf("范围不符 ⇒ 拒并点名范围：rc=%d stderr=%q", rc, errb)
	}

	// ④ 真签名件 + 范围对上 ⇒ 真写；审计里能回读到批准人与批准件路径
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-09-21T07:00:00+08:00", "放行这一件"))+"\n")
	rc, out, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("人签齐了 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	got, _ = os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "被改后的内容") {
		t.Errorf("人签齐了就要真写：%q", got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("审计件不在：%v", err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ab))), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v · %q", err, ab)
	}
	if line["approver"] != "张三" || !strings.HasSuffix(line["approval"].(string), "approvals/dev_edit.json") {
		t.Errorf("审计要能回读「谁让它改的」：%+v", line)
	}
}

// 干跑**不需要**批准件（零副作用那一档），但计划面要照实打出批准件的判决。
func TestDevEdit_DryRunNeedsNoApprovalButShowsIt(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9002", target)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 干跑不该落地\n")

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9002", "--file", target,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ 退 0（不需要批准件），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "批准件") || !strings.Contains(out, "无件（真写要一枚人签批准件）") {
		t.Errorf("计划面要打出批准件的判决（先看后签）：%q", out)
	}
	got, _ := os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "// 原件") {
		t.Errorf("干跑零副作用：%q", got)
	}
}
