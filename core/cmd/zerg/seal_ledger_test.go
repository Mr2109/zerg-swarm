// seal_ledger_test.go —— `core/cmd/zerg/seal_ledger.go`（M2 真搬件）的成对负控。
//
// 目的：给 `sealLedgerGateMain`（及内部 `slgDoSeal` / `slgDoVerify` / `slgSelfTest`）一道**带牙**的
// 回归 —— 哑化任一条判据（核章三态闭集 · 链式摘要 · 缺格 · 精确排除章账自身 · tempdir 回收）⇒
// 必有一格 FAIL。合成件全在 `t.TempDir()` / 临时前缀目录（仓外 · 不碰真目标 · ✗ 不写真实章账）。
//
// ★ GAP-20261001-61：本测试件**零 python 依赖**（不 spawn `python3`）—— 一切走进程内入口 ⇒
// 该条「python3 一律钉 /usr/bin/python3」在本件**不适用**（如实声明，见未覆盖面）。
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// slgTEntry —— 进程内取 (rc, stdout, stderr)（走正门入口）。
func slgTEntry(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := sealLedgerGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// slgTGit —— 静默跑一条 git（仅夹具用）。
func slgTGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out)
	}
}

// slgTFix —— 造一个「合成仓根 + 合成一趟」夹具（照 py `do_check` 的夹具形态）。
func slgTFix(t *testing.T) (root, run, ledger string) {
	t.Helper()
	dir := t.TempDir()
	root = filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		[]byte("#!/bin/bash\necho '   1     tri    步甲'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initCmd := exec.Command("git", "init", "-q", root)
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init 失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "judge.py"), []byte("print('judge')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slgTGit(t, root, "add", "-A")
	run = filepath.Join(dir, "zerg-gates-fake")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "results.tsv"),
		[]byte("PASS\t步甲\t0\t1s\t/nowhere/a.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fb := `{"head": "abc1234", "worktree_fp": "` + strings.Repeat("f", 64) + `"}`
	if err := os.WriteFile(filepath.Join(run, "freshness.json"), []byte(fb), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger = filepath.Join(dir, "seal.jsonl")
	return root, run, ledger
}

func slgSealT(t *testing.T, root, run, ledger string, write bool, reason string) (int, string, string) {
	var so, se strings.Builder
	rc := slgDoSeal(&so, &se, root, run, ledger, write, reason, "2026-01-01T00:00:00Z")
	return rc, so.String(), se.String()
}

func slgVerifyT(t *testing.T, root, ledger string, jsonOn bool) (int, string, string) {
	var so, se strings.Builder
	rc := slgDoVerify(&so, &se, root, ledger, jsonOn)
	return rc, so.String(), se.String()
}

func TestSealLedgerGatePort(t *testing.T) {
	t.Run("正控1 盖章落盘且九格齐", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		rc, out, errs := slgSealT(t, root, run, ledger, true, "自检")
		if rc != 0 || !slgIsFile(ledger) {
			t.Fatalf("正控1 失败：rc=%d err=%q", rc, errs)
		}
		body, _ := slgReadText(ledger)
		for _, key := range []string{`"chapter_no": 1`, `"item_set_digest"`, `"judge_ver"`,
			`"gate_ver"`, `"executed": true`, `"kind": "cover"`, `"chain"`} {
			if !strings.Contains(body, key) {
				t.Fatalf("正控1 缺格 %s：%s", key, body)
			}
		}
		if !strings.Contains(out, "已盖 1 枚章") {
			t.Fatalf("正控1 stdout 不符：%q", out)
		}
	})

	t.Run("负控2 缺 --reason ⇒ rc=2", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		rc, _, errs := slgSealT(t, root, run, ledger, true, "")
		if rc != 2 || !strings.Contains(errs, "--reason") {
			t.Fatalf("负控2 失败：rc=%d err=%q", rc, errs)
		}
	})

	t.Run("正控3 核章带未见跑且逐字不含 PASS", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		if rc, _, errs := slgSealT(t, root, run, ledger, true, "自检"); rc != 0 {
			t.Fatalf("夹具盖章失败：%s", errs)
		}
		rc, out, _ := slgVerifyT(t, root, ledger, false)
		if rc != 0 {
			t.Fatalf("正控3 期望 rc=0，得 %d：%s", rc, out)
		}
		if !strings.Contains(out, "未真跑 · 结论由章支持") {
			t.Fatalf("正控3 缺「未真跑 · 结论由章支持」：%s", out)
		}
		if strings.Contains(out, "PASS") || strings.Contains(out, "通过") {
			t.Fatalf("守门失败：核章输出出现 PASS/通过：%s", out)
		}
		if !strings.Contains(out, "executed=true 计数 0") {
			t.Fatalf("正控3 缺 executed 断言：%s", out)
		}
		if !slgHasLineContaining(out, "未真跑（核章）") {
			t.Fatalf("正控3 步状态不在三态闭集：%s", out)
		}
	})

	t.Run("负控4 改动面变 ⇒ stale rc=1", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		if rc, _, errs := slgSealT(t, root, run, ledger, true, "自检"); rc != 0 {
			t.Fatalf("夹具盖章失败：%s", errs)
		}
		if err := os.WriteFile(filepath.Join(root, "judge.py"), []byte("print('judgeCHANGED')\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := slgVerifyT(t, root, ledger, false)
		if rc != 1 || !strings.Contains(out, "stale") {
			t.Fatalf("负控4 失败：rc=%d out=%s", rc, out)
		}
	})

	t.Run("负控5 手工改章 ⇒ 链摘要对不上 rc=1", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		if rc, _, errs := slgSealT(t, root, run, ledger, true, "自检"); rc != 0 {
			t.Fatalf("夹具盖章失败：%s", errs)
		}
		recs := slgLoadRawLines(ledger)
		if len(recs) == 0 {
			t.Fatal("夹具章账为空")
		}
		recs[0]["status"] = "FORGED"
		slgWriteRawLines(ledger, recs)
		rc, out, _ := slgVerifyT(t, root, ledger, false)
		if rc != 1 || !strings.Contains(out, "链摘要对不上") {
			t.Fatalf("负控5 失败：rc=%d out=%s", rc, out)
		}
	})

	t.Run("负控6 缺格 ⇒ 判无效 rc=1", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		if rc, _, errs := slgSealT(t, root, run, ledger, true, "自检"); rc != 0 {
			t.Fatalf("夹具盖章失败：%s", errs)
		}
		recs := slgLoadRawLines(ledger)
		c := map[string]interface{}{}
		for k, v := range recs[0] {
			c[k] = v
		}
		c["item_set_digest"] = ""
		led2 := filepath.Join(filepath.Dir(ledger), "seal2.jsonl")
		slgWriteRawLines(led2, []map[string]interface{}{c})
		rc, out, _ := slgVerifyT(t, root, led2, false)
		if rc != 1 || !strings.Contains(out, "缺格") {
			t.Fatalf("负控6 失败：rc=%d out=%s", rc, out)
		}
	})

	t.Run("负控7 章账空 ⇒ rc=2 不给结论", func(t *testing.T) {
		root, _, _ := slgTFix(t)
		rc, _, errs := slgVerifyT(t, root, filepath.Join(t.TempDir(), "none.jsonl"), false)
		if rc != 2 || !strings.Contains(errs, "不给结论") {
			t.Fatalf("负控7 失败：rc=%d err=%q", rc, errs)
		}
	})

	t.Run("正控8 核章 --json 全表含 executed_true 0", func(t *testing.T) {
		root, run, ledger := slgTFix(t)
		if rc, _, errs := slgSealT(t, root, run, ledger, true, "自检"); rc != 0 {
			t.Fatalf("夹具盖章失败：%s", errs)
		}
		_, out, _ := slgVerifyT(t, root, ledger, true)
		for _, key := range []string{`"kind": "SealVerify"`, `"executed_true": 0`,
			`"verdicts"`, `"item_set_digest"`, `"missing"`, `"extra"`} {
			if !strings.Contains(out, key) {
				t.Fatalf("正控8 缺键 %s：%s", key, out)
			}
		}
	})

	t.Run("负控9 未知旗标/无动作 ⇒ rc=2", func(t *testing.T) {
		if rc, _, _ := slgTEntry("--nope"); rc != 2 {
			t.Fatalf("负控9a 期望 rc=2 得 %d", rc)
		}
		if rc, _, _ := slgTEntry(); rc != 2 {
			t.Fatalf("负控9b 期望 rc=2 得 %d", rc)
		}
		if rc, _, _ := slgTEntry("--seal"); rc != 2 {
			t.Fatalf("负控9c 期望 rc=2 得 %d", rc)
		}
	})

	t.Run("正控10 自检 --check ⇒ rc=0 且全过", func(t *testing.T) {
		rc, out, errs := slgTEntry("--check")
		if rc != 0 || !strings.Contains(out, "自检结论: 全过") {
			t.Fatalf("正控10 失败：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("正控11 --now 注入旋钮确定化时刻", func(t *testing.T) {
		_, run, ledger := slgTFix(t)
		rc, _, errs := slgTEntry("--seal", run, "--ledger", ledger, "--write",
			"--reason", "自检", "--now", "2020-02-02T02:02:02Z")
		if rc != 0 {
			t.Fatalf("正控11 失败：rc=%d err=%q", rc, errs)
		}
		body, _ := slgReadText(ledger)
		if !strings.Contains(body, `"at": "2020-02-02T02:02:02Z"`) {
			t.Fatalf("正控11 未注入时刻：%s", body)
		}
	})

	// ★ 牙：章账自身必须被**精确排除**出件集内容摘要（否则固定点不可达 · 章恒 stale）。
	t.Run("负控12 件集摘要精确排除章账自身", func(t *testing.T) {
		root, _, _ := slgTFix(t)
		sealDir := filepath.Join(root, "scripts", "gates", "seal")
		if err := os.MkdirAll(sealDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// 让 seal/ 进索引（有在册件）⇒ 其下新件在 porcelain 里**单列**（否则整目录被折叠成一条）。
		if err := os.WriteFile(filepath.Join(sealDir, ".keep"), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		slgTGit(t, root, "add", "-A")
		isd1, _ := slgItemSetDigest(root, "scripts/gates/seal/ledger.jsonl")
		if err := os.WriteFile(filepath.Join(sealDir, "ledger.jsonl"),
			[]byte("{\"chapter_no\": 1}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		isd2, names2 := slgItemSetDigest(root, "scripts/gates/seal/ledger.jsonl")
		if isd1 != isd2 {
			t.Fatalf("负控12 失败：章账未被排除 isd %s→%s names=%v", isd1, isd2, names2)
		}
		// 对照：别的一件改动**必须**改摘要（证牙没被哑化成「恒不变」）。
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		isd3, names3 := slgItemSetDigest(root, "scripts/gates/seal/ledger.jsonl")
		if isd3 == isd2 || len(names3) == 0 {
			t.Fatalf("负控12 牙失效：别的件未改摘要 isd=%s names=%v", isd3, names3)
		}
	})

	// ★ 牙：自检夹具必须回收（A 类 tempdir 泄漏的回归牙 · 前后同前缀增量 = 0）。
	t.Run("负控13 自检夹具回收 delta=0", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
		const prefix = "seal-ledger-selftest."
		before := slgCountPrefix(os.TempDir(), prefix)
		if rc, _, _ := slgTEntry("--check"); rc != 0 {
			t.Fatalf("自检失败")
		}
		after := slgCountPrefix(os.TempDir(), prefix)
		if after != before {
			t.Fatalf("负控13 失败：自检夹具未回收 before=%d after=%d", before, after)
		}
	})
}

func slgCountPrefix(dir, prefix string) int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// TestSlgPyPathSemantics —— 钉 py 路径原语的三类语义差（✗ 不用 filepath.Join 的理由）。
func TestSlgPyPathSemantics(t *testing.T) {
	if got := slgPyJoin("a", "../b"); got != "a/../b" {
		t.Fatalf("pyJoin .. 段：得 %q", got)
	}
	if got := slgPyJoin("repo", "/abs"); got != "/abs" {
		t.Fatalf("pyJoin 后段绝对：得 %q", got)
	}
	if got := slgPyJoin("a", ""); got != "a/" {
		t.Fatalf("pyJoin 空段：得 %q", got)
	}
	if got := slgPyJoin("a", "", "b"); got != "a/b" {
		t.Fatalf("pyJoin 空段中缀：得 %q", got)
	}
	if got := slgPyNormpath("//a/b"); got != "//a/b" {
		t.Fatalf("pyNormpath 双前导斜杠：得 %q", got)
	}
	if got := slgPyNormpath("/a/b/../c"); got != "/a/c" {
		t.Fatalf("pyNormpath 归一：得 %q", got)
	}
}

// TestSealLedgerGateUsageFlags —— 件自己声明的每个旗标各跑一格（D 类死旗标守门）。
func TestSealLedgerGateUsageFlags(t *testing.T) {
	// 取值型旗标：缺参 ⇒ rc=2。
	for _, a := range []string{"--seal", "--ledger", "--reason", "--json"} {
		rc, _, _ := slgTEntry(a)
		if rc != 2 {
			t.Fatalf("旗标 %s 缺参应 rc=2，得 %d", a, rc)
		}
	}
	// 开关型旗标：不带值也解析得动 ⇒ 落到各自动作面（不许报「未知旗标」）。
	if rc, _, errs := slgTEntry("--check"); rc != 0 {
		t.Fatalf("--check 期望 rc=0，得 %d err=%q", rc, errs)
	}
	if rc, _, errs := slgTEntry("--write"); rc != 2 || strings.Contains(errs, "未知旗标") {
		t.Fatalf("--write 单独应 rc=2 用法档，得 rc=%d err=%q", rc, errs)
	}
	if rc, out, _ := slgTEntry("-h"); rc != 0 || !strings.Contains(out, "usage:") {
		t.Fatalf("-h 期望 rc=0 + usage，得 rc=%d out=%q", rc, out)
	}
	// ★ 回归牙（战线① 对齐）：未知参数 ⇒ **argparse 口径正文**（短用法 + `seal-ledger.py: error:`），
	//   逐字对拍 py 现件；把 `slgUsageErr` 改回中文「用法错」文案即红（带行号）。
	{
		rc, _, errs := slgTEntry("--self-test")
		wantPrefix := "usage: seal-ledger.py [-h] [--seal SEAL] [--verify] [--check]\n" +
			"                      [--ledger LEDGER] [--write] [--reason REASON]\n" +
			"                      [--json JSON]\n"
		if rc != 2 || !strings.HasPrefix(errs, wantPrefix) ||
			!strings.HasSuffix(errs, "seal-ledger.py: error: unrecognized arguments: --self-test\n") {
			t.Fatalf("负控14 未知参数应出 argparse 口径正文：rc=%d err=%q", rc, errs)
		}
	}
	// --verify 有真动作 ⇒ 章账空时 rc=2（不许塌陷成 0）。
	if rc, _, _ := slgTEntry("--verify", "--ledger", filepath.Join(t.TempDir(), "none.jsonl")); rc != 2 {
		t.Fatalf("--verify 空账期望 rc=2，得 %d", rc)
	}
}
