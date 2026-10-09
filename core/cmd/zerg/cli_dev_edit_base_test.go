// cli_dev_edit_base_test.go —— `zerg dev edit --base <件路径|HEAD|sha>`（`GAP-20261009-10` 修法①）。
//
// 病灶（缺口逐字）：`--replace` 的每格 old 必须在**现盘**件里恰好命中 1 次 ⇒ 已手改过盘的件
// （改前文本已不在盘上）**回放不了**（第 1 格命中 0 次 · rc=2）。修法①：`--base` 给**基线内容**
// 当匹配基（而非读现盘件），并把该基线作为审计的 `before`（真前态指纹）⇒ 已改盘的件也能把
// 同一笔改动**回放**出真 before→after 的审计链。
//
// 成对负控（✗ 禁恒绿 —— 每一条都有相反的那一格）：
//
//	① 基线 + 补丁 ⇒ 回放结果 == 现盘 ⇒ rc=0（干跑零副作用）；真写 rc=0 且审计
//	   `before_sha256` = **基线**指纹、`after_sha256` = **现盘**指纹（真前态，不再是空动作）；
//	② **默认行为一字不改**（负控）：同一件**不加 `--base`** ⇒ 现盘无 old ⇒ rc=2（消歧「回放不了」）；
//	③ **安全闸**（只能更严）：基线算出的结果 **≠ 现盘**（要覆盖现盘）⇒ rc=2 · 逐字「不逐字节相同」，
//	   且带 `--yes` 也照样拒（件一个字节不动）；
//	④ 形态错（`--base` 与 `--from` 同给 ⇒ rc=2）· 基线读不到 / git 取不到 ⇒ rc=2（**不猜基线**）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevEdit_BaseReplay(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9200", target)
	sign := writeOperatorKey(t, state)
	approve := func() {
		mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
			string(sign("dev_edit", "*", "张三", "2026-10-09T00:00:00+08:00", "放行本件"))+"\n")
	}

	baseline := "package main\n\n// 原件\n"
	after := "package main\n\n// 改后\n"
	// 造「已改盘件」：现盘 = after（改前文本已不在盘上）。
	mustWrite(t, filepath.Join(repo, target), after)
	// 真前态（基线）落一份**仓外整件**（回放 demo 的 `--base` 值）。
	basePath := filepath.Join(t.TempDir(), "base.go")
	mustWrite(t, basePath, baseline)
	pb, _ := json.Marshal([]map[string]string{{"old": "// 原件\n", "new": "// 改后\n"}})
	patch := filepath.Join(t.TempDir(), "p.json")
	mustWrite(t, patch, string(pb))

	diskSHA := shaOf([]byte(after))
	baseSHA := shaOf([]byte(baseline))

	// ② 默认行为负控：不加 --base ⇒ 已改盘件现盘无 old ⇒ rc=2（消歧）。
	rc, _, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--replace", patch, "--dry-run")
	if rc != 2 || !strings.Contains(errb, "回放不了") {
		t.Fatalf("不加 --base ⇒ 已改盘件 rc=2 消歧：rc=%d stderr=%q", rc, errb)
	}

	// ① 基线回放（干跑）：rc=0（回放结果 == 现盘）· 零副作用。
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--replace", patch, "--base", basePath, "--dry-run")
	if rc != 0 {
		t.Fatalf("基线回放干跑 ⇒ rc=0：rc=%d stdout=%s stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "基线回放") {
		t.Errorf("干跑要照实点明「基线回放」：%q", errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != after {
		t.Errorf("干跑零副作用：%q", got)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("干跑不许落审计（没写就不记账）")
	}

	// ① 基线回放（真写 · 带 --yes）：rc=0 且审计 before=基线、after=现盘（真前态指纹）。
	approve()
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--replace", patch, "--base", basePath, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("基线回放真写 ⇒ rc=0：rc=%d stderr=%s", rc, errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != after {
		t.Errorf("零内容变更的回放：件仍应是 %q，实得 %q", after, got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	all := strings.Split(strings.TrimSpace(string(ab)), "\n")
	var line map[string]any
	if err := json.Unmarshal([]byte(all[len(all)-1]), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if b, _ := line["before_sha256"].(string); !strings.HasPrefix(baseSHA, b) {
		t.Errorf("审计 before 要是**基线**（真前态）的指纹：got=%v want[:16]=%s", line["before_sha256"], baseSHA[:16])
	}
	if a, _ := line["after_sha256"].(string); !strings.HasPrefix(diskSHA, a) {
		t.Errorf("审计 after 要是**现盘**的指纹：got=%v want[:16]=%s", line["after_sha256"], diskSHA[:16])
	}
	if line["before_sha256"] == line["after_sha256"] {
		t.Errorf("审计要留**真** before→after（不是空动作）：%+v", line)
	}

	// ③ 安全闸（只能更严）：基线结果 ≠ 现盘 ⇒ rc=2 · 逐字「不逐字节相同」；带 --yes 也照样拒。
	wrongBase := filepath.Join(t.TempDir(), "wrong.go")
	mustWrite(t, wrongBase, "package main\n\n// 原件\n\n// 附注\n") // 含 old ⇒ 补丁能落，但结果 ≠ 现盘
	for _, extra := range [][]string{{"--dry-run"}, {"--confirm=" + hostnameOf(t), "--yes"}} {
		argv := append([]string{"--proposal", "DEV-9200", "--file", target, "--replace", patch, "--base", wrongBase}, extra...)
		rc, _, errb = runDevEdit(t, repo, prop, state, argv...)
		if rc != 2 || !strings.Contains(errb, "不逐字节相同") {
			t.Errorf("基线结果 ≠ 现盘 ⇒ rc=2（安全闸 %v）：rc=%d stderr=%q", extra, rc, errb)
		}
		if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != after {
			t.Errorf("安全闸拒执 ⇒ 件一个字节不动：%q", got)
		}
	}

	// ④ 形态错：--base 与 --from 同给 ⇒ rc=2。
	from := filepath.Join(t.TempDir(), "x.go")
	mustWrite(t, from, after)
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--from", from, "--base", basePath, "--dry-run")
	if rc != 2 || !strings.Contains(errb, "只与") {
		t.Errorf("--base 与 --from 同给 ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	// ④ 基线（路径形态）读不到 ⇒ rc=2（不猜）。
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--replace", patch, "--base", filepath.Join(t.TempDir(), "nope.go"), "--dry-run")
	if rc != 2 || !strings.Contains(errb, "读不了") {
		t.Errorf("基线读不到 ⇒ rc=2（不猜）：rc=%d stderr=%q", rc, errb)
	}
	// ④ 基线（sha 形态）git 取不到 ⇒ rc=2（不猜 · 走 git show 那一支）。
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9200", "--file", target,
		"--replace", patch, "--base", "deadbeef", "--dry-run")
	if rc != 2 || !strings.Contains(errb, "git show") {
		t.Errorf("--base <sha> git 取不到 ⇒ rc=2（不猜）：rc=%d stderr=%q", rc, errb)
	}
}
