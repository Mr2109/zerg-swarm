// check_flag_registry_parity_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckFlagRegistryParityGatePort（定向：go test -run TestCheckFlagRegistryParityGatePort）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// frpFixtureTree —— 在 t.TempDir() 下造一棵含 `core/cmd/zerg/main.go` 的夹具（仓外）。
func frpFixtureTree(t *testing.T, mainBody, extraBody string, withExtra bool) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte(mainBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if withExtra {
		if err := os.WriteFile(filepath.Join(d, "family_probe.go"), []byte(extraBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func frpRunEntry(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := flagRegistryParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckFlagRegistryParityGatePort(t *testing.T) {
	// ── 正控（必然判绿）：命令树两面（除存量豁免）对齐 ─────────────────────
	green := frpFixtureTree(t, frpFixtureMain, frpFixtureExtra, true)
	rcGreen, outGreen, _ := frpRunEntry(t, "--root", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "新分家项 0 项") {
		t.Fatalf("正控失败：对齐件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控①（必然判红）：名字表删 --desc ⇒ 「缺一处」点名 ────────────────
	missingBody := strings.Replace(frpFixtureMain,
		"case \"--desc\", \"--slice-id\":", "case \"--slice-id\":", 1)
	rcMiss, outMiss, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, missingBody, frpFixtureExtra, true))
	if rcMiss != 1 || !strings.Contains(outMiss, "缺一处") || !strings.Contains(outMiss, "--desc") {
		t.Fatalf("负控-缺一处 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcMiss {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcMiss)
	}

	// ── 负控②（必然判红）：名字表加一枚无人写的名 ⇒ 「多一处」 ─────────────
	extraBody := strings.Replace(frpFixtureMain,
		"case \"--desc\", \"--slice-id\":", "case \"--desc\", \"--slice-id\", \"--zz-new\":", 1)
	rcGhost, outGhost, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, extraBody, frpFixtureExtra, true))
	if rcGhost != 1 || !strings.Contains(outGhost, "多一处") || !strings.Contains(outGhost, "--zz-new") {
		t.Fatalf("负控-多一处 失败：得 rc=%d out=%q", rcGhost, outGhost)
	}

	// ── 负控③（必然判红）：用法串取值改裸形 ⇒ 「形态」 ────────────────────
	formBody := strings.Replace(frpFixtureMain, "--slice-id <片>", "--slice-id", 1)
	rcForm, outForm, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, formBody, frpFixtureExtra, true))
	if rcForm != 1 || !strings.Contains(outForm, "形态") || !strings.Contains(outForm, "--slice-id") {
		t.Fatalf("负控-形态 失败：得 rc=%d out=%q", rcForm, outForm)
	}

	// ── 不给结论（≠ 绿）：命令树零条 ⇒ rc=2 ──────────────────────────────
	rcBail, outBail, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, frpEmptyMain, "", false))
	if rcBail != 2 || !strings.Contains(outBail, "判不了") {
		t.Fatalf("空转/零条失败：得 rc=%d out=%q", rcBail, outBail)
	}

	// ── 取不到输入：根路径不存在 ⇒ rc=2 ──────────────────────────────────
	rcNoRoot, outNoRoot, _ := frpRunEntry(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "包目录不在盘") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── 未知旗标（argparse 用法错）⇒ rc=2 · stderr 明说 ──────────────────
	rcFlag, _, errFlag := frpRunEntry(t, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── `--json`（真仓机器面一行）⇒ rc=0 且尾行是 JSON ────────────────────
	repo := filepath.Join(t.TempDir(), "repo")
	_ = os.MkdirAll(filepath.Join(repo, "core", "cmd", "zerg"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "core", "cmd", "zerg", "main.go"), []byte(frpFixtureMain), 0o644)
	rcJSON, outJSON, _ := frpRunEntry(t, "--root", repo, "--json")
	if rcJSON != 0 || !strings.Contains(outJSON, "\"gate\": \"flag-registry-parity\"") {
		t.Fatalf("--json 失败：得 rc=%d out=%q", rcJSON, outJSON)
	}

	// ── `-h`（argparse 帮助正文）⇒ rc=0 ──────────────────────────────────
	rcHelp, outHelp, _ := frpRunEntry(t, "-h")
	if rcHelp != 0 || !strings.Contains(outHelp, "usage: check-flag-registry-parity.py") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcHelp, outHelp)
	}

	// ── `--self-test` 十二格自身必须全过（防恒绿/恒红） ───────────────────
	rcST, outST, _ := frpRunEntry(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "失败 0 条") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
	// ── 面④ 读者面（按命令 · 缺口账 `GAP-20260927-325`）在报告里可见（报告档 · 不进退码） ──
	if !strings.Contains(outST, "④读者面") {
		t.Fatalf("--self-test 输出里没有读者面两格：%q", outST)
	}
}

// frpFindRepoRoot —— 从 cwd 上溯找仓根（判据：`scripts/gates/check-flag-registry-parity.py` 在盘）。
func frpFindRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "scripts", "gates", "check-flag-registry-parity.py")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	t.Fatalf("上溯找不到仓根（从 %s）", wd)
	return ""
}

// TestFlagRegistryParityGoPyDifferential —— **Go 半 ⟷ py 半 逐字节对拍**（缺口账 `GAP-20260927-325`
// 的「只改一边 ⇒ 对拍必须 FAIL」那道保险）。四态：`--self-test` / `--list-rules` / `-h` / 未知旗标。
// 解释器钉 `/usr/bin/python3`（argparse 帮助正文随解释器大版本变 —— 见 `GAP-20261002-07`）。
// ★ 本测试**不需要真仓**（`--self-test` 走合成夹具；`--list-rules`/`-h` 是常量面）⇒ 两半搬同一份输入。
func TestFlagRegistryParityGoPyDifferential(t *testing.T) {
	root := frpFindRepoRoot(t)
	py := filepath.Join(root, "scripts", "gates", "check-flag-registry-parity.py")
	python := "/usr/bin/python3"
	if _, err := os.Stat(python); err != nil {
		t.Fatalf("钉住的解释器不在盘：%s（对拍要有确定输入面，不许换解释器）", python)
	}
	// ★ 真跑面（带 `--root` 夹具）：**读者面（面④）只在这一路出现** ⇒ 不加它 ⇒ 对拍治不到新腿。
	fx := frpMkFixture(filepath.Join(t.TempDir(), "diffroot"))
	cases := [][]string{{"--self-test"}, {"--list-rules"}, {"-h"}, {"--no-such-flag"},
		{"--root", fx}, {"--root", fx, "--json"}}
	for _, argv := range cases {
		var gOut, gErr bytes.Buffer
		grc := flagRegistryParityGateMain(&gOut, &gErr, argv)

		cmd := exec.Command(python, append([]string{py}, argv...)...)
		var pOut, pErr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &pOut, &pErr
		err := cmd.Run()
		prc := 0
		if ee, ok := err.(*exec.ExitError); ok {
			prc = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("py 半起不来 argv=%v：%v", argv, err)
		}
		if grc != prc {
			t.Errorf("对拍 FAIL（rc）argv=%v：go=%d py=%d", argv, grc, prc)
		}
		if gOut.String() != pOut.String() {
			t.Errorf("对拍 FAIL（stdout）argv=%v：\n--go--\n%s\n--py--\n%s", argv, gOut.String(), pOut.String())
		}
		if gErr.String() != pErr.String() {
			t.Errorf("对拍 FAIL（stderr）argv=%v：\n--go--\n%s\n--py--\n%s", argv, gErr.String(), pErr.String())
		}
	}
}
