// check_keep_marks_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · 负控成对）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckKeepMarksGatePort（定向：go test -run TestCheckKeepMarksGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kmpFxRoot —— 在 t.TempDir() 下造一棵含 `ui/src/api.rs`（锚命中 anchors 次）的夹具根。
func kmpFxRoot(t *testing.T, anchors int) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ui", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < anchors; i++ {
		b.WriteString("#[allow(dead_code)]\nfn f() {}\n")
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "src", "api.rs"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func kmpFxDossier(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dossier.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func kmpRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := keepMarksGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

const (
	kmpDGood = `{"seq_max":1,"history_removed":[],"entries":[{"seq":1,"keep":"yes",` +
		`"file":"ui/src/api.rs","anchor":"#[allow(dead_code)]","reason":"合成理由"}]}`
	kmpDRemoved = `{"seq_max":1,"history_removed":[{"seq":1,"file":"ui/src/api.rs"}],` +
		`"entries":[{"seq":1,"keep":"yes","file":"ui/src/api.rs","anchor":"#[allow(dead_code)]","reason":"r"}]}`
	kmpDNoReason = `{"seq_max":1,"history_removed":[],"entries":[{"seq":1,"keep":"yes",` +
		`"file":"ui/src/api.rs","anchor":"#[allow(dead_code)]"}]}`
)

func TestCheckKeepMarksGatePort(t *testing.T) {
	// ── 正控（必然判绿）：锚恰 1 次 + 带理由 ⇒ rc=0 ─────────────────────────
	root := kmpFxRoot(t, 1)
	rcGreen, outGreen, _ := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDGood), "--root", root)
	if rcGreen != 0 || !strings.Contains(outGreen, "案卷件自身无红") {
		t.Fatalf("正控失败：好案卷应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}
	if !strings.Contains(outGreen, "带理由 1 条") {
		t.Fatalf("正控失败：应报「带理由 1 条」，得 out=%q", outGreen)
	}

	// ── 负控①（必然判红）：history_removed 非空 ⇒ rc=1（只增不减被破）──────
	rcRm, outRm, _ := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDRemoved), "--root", root)
	if rcRm != 1 || !strings.Contains(outRm, "只增不减**被破**") {
		t.Fatalf("负控-删条 失败：得 rc=%d out=%q", rcRm, outRm)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRm {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRm)
	}

	// ── 负控②：登记行缺 `reason` 字段 ⇒ rc=1 ───────────────────────────────
	rcNR, outNR, _ := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDNoReason), "--root", root)
	if rcNR != 1 || !strings.Contains(outNR, "没有 `reason` 字段") {
		t.Fatalf("负控-缺理由字段 失败：得 rc=%d out=%q", rcNR, outNR)
	}

	// ── 负控③：锚命中 2 次（锚不唯一）⇒ rc=1 ───────────────────────────────
	rcDup, outDup, _ := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDGood), "--root", kmpFxRoot(t, 2))
	if rcDup != 1 || !strings.Contains(outDup, "恰好命中 1 次") {
		t.Fatalf("负控-锚不唯一 失败：得 rc=%d out=%q", rcDup, outDup)
	}

	// ── 负控④：`seq` 非严格递增 ⇒ rc=1 ─────────────────────────────────────
	badSeq := `{"seq_max":2,"history_removed":[],"entries":[{"seq":2,"file":"ui/src/api.rs",` +
		`"anchor":"#[allow(dead_code)]","reason":"r"},{"seq":1,"file":"ui/src/api.rs",` +
		`"anchor":"#[allow(dead_code)]","reason":"r"}]}`
	rcSeq, outSeq, _ := kmpRun(t, "--dossier", kmpFxDossier(t, badSeq), "--root", root)
	if rcSeq != 1 || !strings.Contains(outSeq, "不是严格递增") {
		t.Fatalf("负控-序号乱 失败：得 rc=%d out=%q", rcSeq, outSeq)
	}

	// ── 不给结论（≠ 绿）：空转（ui/ 与 core/ 都没有）⇒ rc=2 ────────────────
	rcVac, outVac, _ := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDGood), "--root", t.TempDir())
	if rcVac != 2 || !strings.Contains(outVac, "判据域不存在") {
		t.Fatalf("空转失败：得 rc=%d out=%q", rcVac, outVac)
	}

	// ── 取不到输入：案卷件缺 ⇒ rc=2（缺件不许静默放绿）─────────────────────
	rcMiss, outMiss, _ := kmpRun(t, "--dossier", filepath.Join(t.TempDir(), "nope.json"), "--root", root)
	if rcMiss != 2 || !strings.Contains(outMiss, "缺件") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 想接闸：--block ⇒ rc=2 被拒 ─────────────────────────────────────────
	rcBlock, outBlock, _ := kmpRun(t, "--block")
	if rcBlock != 2 || !strings.Contains(outBlock, "不接闸") {
		t.Fatalf("接闸拒收失败：得 rc=%d out=%q", rcBlock, outBlock)
	}

	// ── Q-176：`--json` 表外字段 ⇒ rc=2（stderr 明说）─────────────────────
	rcJSON, _, errJSON := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDGood), "--root", root,
		"--json", "rc,nope")
	if rcJSON != 2 || !strings.Contains(errJSON, "不认的字段名") {
		t.Fatalf("--json 表外字段失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── 读失败不再静默的回归牙：`ui/**` 里一份读不动的 `.rs` ⇒ 登记进 kmpReadFails
	// ⇒ 入口非空即 stderr BLOCKED 头 ＋ 逐件点名 ＋ rc=2（B 类静默面已闭）。
	t.Run("读失败件_登记并拦停_rc2", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无视 0o000 权限位：本格无鉴别力")
		}
		root := kmpFxRoot(t, 1)
		p := filepath.Join(root, "ui", "src", "teeth-unreadable.rs")
		if err := os.WriteFile(p, []byte("fn g() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(p, 0o644) }()
		kmpReadFails = nil
		rc, out, errb := kmpRun(t, "--dossier", kmpFxDossier(t, kmpDGood), "--root", root)
		if rc != 2 {
			t.Fatalf("读不动的件应拦停 rc=2（✗ 不许静默跳过），得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(errb, "⛔ BLOCKED：判据件读失败") {
			t.Fatalf("stderr 缺 BLOCKED 头：%q", errb)
		}
		if !strings.Contains(errb, p) {
			t.Fatalf("stderr 未逐件点名读不动的件 %s：%q", p, errb)
		}
	})

	// ── --self-test 三档自身必须全过（防恒绿/恒红）──────────────────────────
	rcST, outST, _ := kmpRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检：全过") && !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
