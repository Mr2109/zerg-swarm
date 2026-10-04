// check_lineref_limit_test.go —— Go 真搬的**成对负控**定向单测（M2 新口径 · 外部树旋钮族）。
//
// ✗ 禁恒绿：本件同时钉住「必然判绿」「必然出红明细 rc=1」「不给结论 rc=2（三档）」
// 与**新口径退 2 边界**（只判「都取不到」：根缺席 ⇒ rc=2；只剩根一棵 ⇒ 照判 rc=0）四态，
// 并钉住 B 类读失败（读不了的件）已**翻正**为「登记 + 判红 rc=2」（✗ 不再静默吞件）。
// 用例名：TestCheckLinerefLimitGatePort（定向：go test -run TestCheckLinerefLimitGatePort ./cmd/zerg）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clrlW —— 造件（自动建父目录）。
func clrlW(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// clrlFx —— 造一棵合成夹具（仓外）：主仓 root + 第二棵树 docs。
func clrlFx(t *testing.T) (root, docs string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "Zerg")
	docs = filepath.Join(base, "Zerg-内部文档")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, docs
}

func clrlRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkLinerefLimitGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckLinerefLimitGatePort(t *testing.T) {
	// ── 正控（必然判绿）：可判且不越界 ⇒ rc=0 ──────────────────────────────────
	t.Run("正控_可判不越界_rc0", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "ok.md"), "见 wall/Cargo.lock:7\n")
		rc, out, errs := clrlRun(t, "--root", root, "--docs-root", docs)
		if rc != 0 || errs != "" {
			t.Fatalf("正控失败：应 rc=0/stderr 空，得 rc=%d err=%q out=%s", rc, errs, out)
		}
		if !strings.Contains(out, "可判 1 · 越界 0") || !strings.Contains(out, "分母守恒 ✓") {
			t.Fatalf("正控失败：正文不符 out=%s", out)
		}
	})

	// ── 负控①（有红明细）：行号止 > 件行数 ⇒ rc=1 且逐处点名 ────────────────────
	t.Run("负控_越界_rc1_逐处点名", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "bad.md"), "见 wall/Cargo.lock:8\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs)
		if rc != 1 {
			t.Fatalf("负控①失败：越界应 rc=1，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "越界 scripts/gates/bad.md:1  被引件 wall/Cargo.lock  行号止 8 > 件行数 7") {
			t.Fatalf("负控①失败：缺逐处点名 out=%s", out)
		}
	})

	// ── 负控②：样张档（夹具构造行里的引用）⇒ 只登记、不许判越界 ────────────────
	t.Run("负控_样张档不算越界", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "fx.md"),
			"_mk(root, docs, \"x.md\", \"见 wall/Cargo.lock:99\\\\n\")\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 {
			t.Fatalf("负控②失败：样张档不许越界 ⇒ rc=0，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "判不了 scripts/gates/fx.md:1  wall/Cargo.lock（sample）") {
			t.Fatalf("负控②失败：应落 sample 且不判 out=%s", out)
		}
	})

	// ── 分母守恒（判不了 ≠ 消失）：改名的落点 ⇒ missing，总数不减 ───────────────
	t.Run("负控_改名落判不了不消失", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "scripts", "gates", "gone.md"), "见 wall/nope.lock:3\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 || !strings.Contains(out, "（missing）") || !strings.Contains(out, "引用总 1 处") {
			t.Fatalf("守恒失败：应落 missing 且总数守恒，得 rc=%d out=%s", rc, out)
		}
	})

	// ── 孪生反向格：同名多解行数不同 ⇒ 不许判（ambiguous）；行数全同 ⇒ 判 ──────
	t.Run("负控_孪生多解不许判", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "a", "gem.md"), "l1\nl2\nl3\n")
		clrlW(t, filepath.Join(root, "b", "gem.md"), "l1\nl2\nl3\nl4\nl5\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "twin.md"), "见 gem.md:3\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 || !strings.Contains(out, "（ambiguous）") {
			t.Fatalf("孪生反向格失败：应 ambiguous，得 rc=%d out=%s", rc, out)
		}
		clrlW(t, filepath.Join(root, "b", "gem.md"), "l1\nl2\nl3\n")
		rc, out, _ = clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 || !strings.Contains(out, "可判 1 · 越界 0") {
			t.Fatalf("孪生同长失败：应判且不越界，得 rc=%d out=%s", rc, out)
		}
	})

	// ── 不给结论三档：面 0 件 / 引用 0 处 / 根不在盘 ⇒ 一律 rc=2 ────────────────
	t.Run("不给结论_扫描面0件_rc2", func(t *testing.T) {
		root, docs := clrlFx(t)
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs)
		if rc != 2 || !strings.Contains(out, "扫描面 scripts/gates 下 0 件（rc=2）") {
			t.Fatalf("面0件失败：得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("不给结论_引用0处_rc2", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "scripts", "gates", "empty.md"), "无引用的普通一行\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs)
		if rc != 2 || !strings.Contains(out, "引用 0 处（rc=2）") {
			t.Fatalf("引用0处失败：得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("不给结论_根不在盘_rc2", func(t *testing.T) {
		root, docs := clrlFx(t)
		rc, out, _ := clrlRun(t, "--root", filepath.Join(root, "nope"), "--docs-root", docs)
		if rc != 2 || !strings.Contains(out, "0 件（rc=2）") {
			t.Fatalf("根缺席失败：得 rc=%d out=%s", rc, out)
		}
	})

	// ── ★ 新口径退 2 边界（成对牙）：只剩根一棵（外部树缺席）⇒ 照判 rc=0 ────────
	t.Run("退2边界_只剩根一棵_照判rc0", func(t *testing.T) {
		root, _ := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "ok.md"), "见 wall/Cargo.lock:3\n")
		missing := filepath.Join(t.TempDir(), "no-docs")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", missing)
		if rc != 0 {
			t.Fatalf("退2边界失败：只剩根一棵应 rc=0（≠ 恒红），得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "可判 1") {
			t.Fatalf("退2边界失败：应照判根那棵 out=%s", out)
		}
	})

	// ── ★ 外部树旋钮 --docs-root 有牙：裸名只在该树里找得到 ⇒ 只有传了才对 ─────
	t.Run("旋钮_docs_root生效", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "scripts", "gates", "r.md"), "见 uniq.md:3\n")
		other := filepath.Join(t.TempDir(), "Zerg-内部文档")
		clrlW(t, filepath.Join(other, "q", "uniq.md"), "l1\nl2\nl3\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 || !strings.Contains(out, "（missing）") {
			t.Fatalf("旋钮缺省失败：缺省树里没有该件 ⇒ 应 missing，得 rc=%d out=%s", rc, out)
		}
		rc, out, _ = clrlRun(t, "--root", root, "--docs-root", other, "--verbose")
		if rc != 0 || !strings.Contains(out, "可判 1 · 越界 0") {
			t.Fatalf("旋钮生效失败：传对树后应 found，得 rc=%d out=%s", rc, out)
		}
	})

	// ── ★ B 类读失败（读不了的件）**已翻正**：登记 + 判红 rc=2（✗ 不再静默吞件）────
	//    该件里有一条**本可判越界**的引用（:99 引 3 行件）；新口径不再把它整件跳过，
	//    而是登记相对路径 + BLOCKED rc=2（体例照 check_token_argv.go）。本用例钉住新态：
	//    谁把它改回「静默吞件 ⇒ rc=0」，本用例当场红。
	t.Run("读不了的件_登记判红_rc2", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "ok.md"), "见 wall/Cargo.lock:3\n")
		locked := filepath.Join(root, "scripts", "gates", "locked.md")
		clrlW(t, locked, "见 wall/Cargo.lock:99\n")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(locked, 0o644)
		if _, err := os.ReadFile(locked); err == nil {
			t.Skip("本机该件仍可读（例如以 root 跑）⇒ 读失败面不可复现")
		}
		rc, _, errs := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 2 {
			t.Fatalf("读不了的件应 rc=2（不给结论），得 rc=%d err=%s", rc, errs)
		}
		if !strings.Contains(errs, "locked.md") {
			t.Fatalf("读不了的件应被逐件点名，stderr=%q", errs)
		}
	})

	// ── 遍历/排序点1 有牙：顶层件序 = py 明写 sorted（名称升序）⇒ 明细行序同 ────
	t.Run("遍历序_顶层件名升序", func(t *testing.T) {
		root, docs := clrlFx(t)
		for _, n := range []string{"c.md", "a.md", "b.md"} {
			clrlW(t, filepath.Join(root, "scripts", "gates", n), "见 nope.lock:1\n")
		}
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--verbose")
		if rc != 0 {
			t.Fatalf("遍历序失败：rc=%d out=%s", rc, out)
		}
		ia := strings.Index(out, "a.md:1")
		ib := strings.Index(out, "b.md:1")
		ic := strings.Index(out, "c.md:1")
		if ia < 0 || ib < 0 || ic < 0 || !(ia < ib && ib < ic) {
			t.Fatalf("遍历序失败：应 a<b<c 的升序明细，得 out=%s", out)
		}
	})

	// ── 机器面三形态：[] / null / 键恒在场（✗ 不许 omitempty 吞键）──────────────
	t.Run("机器面_json三形态", func(t *testing.T) {
		root, docs := clrlFx(t)
		clrlW(t, filepath.Join(root, "wall", "Cargo.lock"), "l1\nl2\nl3\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "ok.md"), "见 wall/Cargo.lock:3\n")
		clrlW(t, filepath.Join(root, "scripts", "gates", "gone.md"), "见 nope.lock:9\n")
		rc, out, _ := clrlRun(t, "--root", root, "--docs-root", docs, "--json")
		if rc != 0 {
			t.Fatalf("机器面失败：rc=%d out=%s", rc, out)
		}
		for _, want := range []string{
			`"face": 2,`, `"total": 2,`, `"judged": 1,`, `"unjudged": 1,`,
			`"over": 0,`, `"conserved": true`, `"over": []`,
			`"target": null`, `"n_lines": null`, `"unjudged": [`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("机器面失败：缺 %q\nout=%s", want, out)
			}
		}
		// unjudged 为空时也必须显式 `[]`（✗ 不许吞键 / ✗ 不许 null）
		root2, docs2 := clrlFx(t)
		clrlW(t, filepath.Join(root2, "wall", "Cargo.lock"), "l1\nl2\nl3\n")
		clrlW(t, filepath.Join(root2, "scripts", "gates", "only.md"), "见 wall/Cargo.lock:3\n")
		_, out2, _ := clrlRun(t, "--root", root2, "--docs-root", docs2, "--json")
		if !strings.Contains(out2, `"unjudged": []`) {
			t.Fatalf("机器面空集合失败：应见 \"unjudged\": []，得 out=%s", out2)
		}
	})

	// ── 自检 / 帮助 / 用法错 ────────────────────────────────────────────────
	t.Run("自检8格", func(t *testing.T) {
		rc, out, _ := clrlRun(t, "--self-test")
		if rc != 0 || !strings.Contains(out, "自检 8 格 · 失败 0 格") {
			t.Fatalf("自检失败：rc=%d out=%s", rc, out)
		}
	})
	t.Run("帮助面逐字", func(t *testing.T) {
		rc, out, _ := clrlRun(t, "-h")
		if rc != 0 || !strings.HasPrefix(out, "usage: check-lineref-limit.py [-h] [--json] [--verbose] [--root ROOT]\n"+
			"                              [--self-test]\n") {
			t.Fatalf("-h 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "  --root ROOT  仓根（缺省自动回溯；供入仓前试跑）") {
			t.Fatalf("-h 失败：缺 --root 帮助行 out=%q", out)
		}
	})
	t.Run("未知旗标_rc2", func(t *testing.T) {
		rc, out, errs := clrlRun(t, "--zzz")
		if rc != 2 || out != "" || !strings.Contains(errs, "unrecognized arguments: --zzz") {
			t.Fatalf("未知旗标失败：rc=%d stdout=%q stderr=%q", rc, out, errs)
		}
	})
	t.Run("缺值_rc2", func(t *testing.T) {
		rc, _, errs := clrlRun(t, "--root")
		if rc != 2 || !strings.Contains(errs, "argument --root: expected one argument") {
			t.Fatalf("缺值失败：rc=%d stderr=%q", rc, errs)
		}
	})
}
