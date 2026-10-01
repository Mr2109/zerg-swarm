// who_references_test.go —— Go 真搬的**成对负控**定向单测（M2 · 入口 whoReferencesGateMain）。
//
// ✗ 禁恒绿：同时断言「必然判绿（rc=0）」「必然判红（rc=1）」「不给结论（rc=2）」三态，
// 并含**新口径②的自灭对**（外部两树都在场 rc=1 ⟷ 两树都不在场 rc=2）—— 任一把判据写死都必失败。
// 用例名：TestWhoReferencesGatePort（定向：go test -run TestWhoReferencesGatePort）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// wrTestRepo —— 造一棵合成仓（root/scripts/gates 存在）+ 可选仓外两树。
// 返回 (root, docs, arch)；trees=false 时两棵外部树**都不造**（新口径②靶）。
func wrTestRepo(t *testing.T, trees bool) (string, string, string) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "Zerg")
	docs := filepath.Join(parent, "Zerg-内部文档")
	arch := filepath.Join(parent, "Zerg-归档")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mk := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join(root, "docs", "真件.md"), "真件\n")
	mk(filepath.Join(root, "readme.md"), "见 `docs/真件.md` 一件\n")
	mk(filepath.Join(root, "notes.md"), "旧参考 `docs/调研/旧稿.md` 一句\n")
	if trees {
		if err := os.MkdirAll(filepath.Join(docs, "调研"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(arch, "档案"), 0o755); err != nil {
			t.Fatal(err)
		}
		mk(filepath.Join(root, "old2.md"), "封装参考 `docs/档案/旧档.md` 一句\n")
		mk(filepath.Join(docs, "调研", "旧稿.md"), "旧稿\n")
		mk(filepath.Join(arch, "档案", "旧档.md"), "旧档\n")
	}
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	return root, docs, arch
}

func wrRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := whoReferencesGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestWhoReferencesGatePort(t *testing.T) {
	root, _, _ := wrTestRepo(t, true)

	// ── 正控（必然判绿）：真目标引用全部在仓 ⇒ rc=0 ─────────────────────
	rcGreen, outGreen, _ := wrRun(t, "--root", root, "docs/真件.md")
	if rcGreen != 0 || !strings.Contains(outGreen, "结论：引用全部指向仍存在的真目标") {
		t.Fatalf("正控失败：得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：引用指向 Zerg-内部文档 已归档件 ⇒ rc=1 ─────────────
	rcRed, outRed, _ := wrRun(t, "--root", root, "docs/调研/旧稿.md")
	if rcRed != 1 || !strings.Contains(outRed, "→Zerg-内部文档") {
		t.Fatalf("负控-已归档 失败：得 rc=%d out=%q", rcRed, outRed)
	}
	// ── 负控（必然判红）：引用指向 Zerg-归档 件 ⇒ rc=1 ───────────────────
	rcArch, _, _ := wrRun(t, "--root", root, "docs/档案/旧档.md")
	if rcArch != 1 {
		t.Fatalf("负控-归档树 失败：得 rc=%d", rcArch)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码。
	if rcGreen == rcRed || rcRed != rcArch {
		t.Fatalf("成对断言失败：绿=%d 红=%d 归档=%d ⇒ 无鉴别力", rcGreen, rcRed, rcArch)
	}

	// ── 不给结论：零引用 ⇒ rc=2 ──────────────────────────────────────────
	rcZero, outZero, _ := wrRun(t, "--root", root, "docs/不存在.md")
	if rcZero != 2 || !strings.Contains(outZero, "零引用 ⇒ rc=2") {
		t.Fatalf("零引用失败：得 rc=%d out=%q", rcZero, outZero)
	}

	// ── 不给结论：短名不存在（无 `/` 且非 .md）⇒ rc=2（stdout 提示） ──────
	rcShort, outShort, _ := wrRun(t, "--root", root, "nope")
	if rcShort != 2 || !strings.Contains(outShort, "在当前仓根不存在") {
		t.Fatalf("短名不存在失败：得 rc=%d out=%q", rcShort, outShort)
	}

	// ── 不给结论：空串（甲案：参数值无效 ⇒ 2，✗ 不当绿） ─────────────────
	rcEmpty, _, errEmpty := wrRun(t, "--root", root, "")
	if rcEmpty != 2 || !strings.Contains(errEmpty, "空串/纯空白") {
		t.Fatalf("空串失败：得 rc=%d stderr=%q", rcEmpty, errEmpty)
	}

	// ── 用法错：未知旗标 ⇒ rc=2（argparse 档） ───────────────────────────
	rcFlag, _, errFlag := wrRun(t, "--nope")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── `-h` ⇒ rc=0 + usage 正文 ────────────────────────────────────────
	rcH, outH, _ := wrRun(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "usage: who-references.py [-h]") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── --self-test 三档自身必须全过 ────────────────────────────────────
	rcST, outST, _ := wrRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检：通过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}

	// ★ 新口径②：显式旋钮指向不存在的目录 ⇒ rc=2（取不到外部件 · ✗ 不静默绿）。
	rcKnob, _, errKnob := wrRun(t, "--root", root, "--docs-root",
		filepath.Join(t.TempDir(), "no-such-docs"), "docs/调研/旧稿.md")
	if rcKnob != 2 || !strings.Contains(errKnob, "--docs-root 不是目录") {
		t.Fatalf("显式旋钮无效失败：得 rc=%d stderr=%q", rcKnob, errKnob)
	}
}

// ★ 新口径② 的**自灭对**：同一条有 pending 的查询 —— 外部两树在场 ⇒ rc=1；两树都不在场 ⇒ rc=2。
// 任一档把判据写死（恒 1 或恒 2）都必失败。
func TestWhoReferencesTreesFailClosed(t *testing.T) {
	rootT, _, _ := wrTestRepo(t, true)
	rcHas, _, _ := wrRun(t, "--root", rootT, "docs/调研/旧稿.md")
	if rcHas != 1 {
		t.Fatalf("两树在场档：期望 rc=1，得 rc=%d", rcHas)
	}
	rootN, _, _ := wrTestRepo(t, false)
	rcNone, _, errNone := wrRun(t, "--root", rootN, "docs/调研/旧稿.md")
	if rcNone != 2 || !strings.Contains(errNone, "外部树取不到") {
		t.Fatalf("两树缺席档：期望 rc=2 + 「外部树取不到」，得 rc=%d stderr=%q", rcNone, errNone)
	}
	if rcHas == rcNone {
		t.Fatalf("成对断言失败：两树在场 rc=%d 与缺席 rc=%d 同码 ⇒ 无鉴别力", rcHas, rcNone)
	}
}
