// check_public_face_commands_test.go —— `check_public_face_commands.go`
// （= py `scripts/gates/check-public-face-commands.py` 598 行的真搬）的**成对负控**测试件。
//
// 口径：每条用例都钉「有牙」——哑化对应实现（仓外副本）⇒ 本件对应子用例 FAIL（仓外哑化实证见交付）。
// ★ 本件**不调任何 `python3`**（无 `subprocess` 面）⇒ `GAP-20261001-61`（测件里 python3 一律钉
//
//	`/usr/bin/python3`）在本件**不适用**（如实申报，不是漏钉）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func cpfcT_run(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkPublicFaceCommandsGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cpfcT_repoRoot：上溯找含 `core/cmd/zerg/main.go` 的仓根（= 生产入口 `cpfcRepoRootGo` 的口径）。
func cpfcT_repoRoot(t *testing.T) string {
	t.Helper()
	r := cpfcRepoRootGo()
	if r == "" || !cpfcIsFile(cpfcPyJoin(r, "core", "cmd", "zerg", "main.go")) {
		t.Skipf("上溯不到仓根（cwd 推导 = %q）⇒ 跳过需真仓的用例", r)
	}
	return r
}

func cpfcT_read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func cpfcT_write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cpfcT_fixture：造一棵最小判据树 —— 只含真页两件（`publish/docs/CLI.{zh-CN,en}.md` 的真副本，
// 与 py `make_fixture` 同口径）；真源二进制走 `--zerg` 显式指向 **仓内只读** `bin/zerg`（✗ 不写仓内）。
func cpfcT_fixture(t *testing.T, mutate func(root string)) string {
	t.Helper()
	repo := cpfcT_repoRoot(t)
	root := t.TempDir()
	for _, rel := range cpfcPages {
		dst := cpfcPyJoin(root, rel)
		if err := os.MkdirAll(cpfcPyDirname(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		cpfcT_write(t, dst, cpfcT_read(t, cpfcPyJoin(repo, rel)))
	}
	if mutate != nil {
		mutate(root)
	}
	return root
}

// cpfcT_zergArg：把仓内只读真源二进制喂给 `--zerg`（真仓 `bin/zerg` 不在 ⇒ 跳该用例）。
func cpfcT_zergArg(t *testing.T) string {
	t.Helper()
	z := cpfcPyJoin(cpfcT_repoRoot(t), "bin", "zerg")
	if !cpfcIsFile(z) {
		t.Skipf("仓内只读真源二进制不在（%s）⇒ 跳过需子进程的用例", z)
	}
	return z
}

// ── 1. 正控 + 成对负控（py `self_test` 的八格在 Go 侧真跑）──────────────────────
func TestCpfcS01_自检全过_成对负控八格(t *testing.T) {
	root := cpfcT_repoRoot(t)
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if !cpfcIsFile(cpfcPyJoin(root, "bin", "zerg")) {
		t.Skip("仓内 bin/zerg 不在 ⇒ 自检前置缺件")
	}
	rc, out, _ := cpfcT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "⇒ 自检全过（8 格）") {
		t.Fatalf("应打出「⇒ 自检全过（8 格）」\n%s", out)
	}
	if n := strings.Count(out, "   ✓ "); n != 8 {
		t.Fatalf("自检应实跑 8 格全 ✓（成对负控格缺失），得 %d\n%s", n, out)
	}
}

// ── 2. 方向 A 负控：页里删一格 ⇒ 红（`undocumented-command`）──────────────────
func TestCpfcS02_负控_页里删zerg_version一格(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		for _, rel := range cpfcPages {
			p := cpfcPyJoin(root, rel)
			var keep []string
			for _, l := range strings.Split(cpfcT_read(t, p), "\n") {
				if strings.Contains(l, "zerg version\x60") && strings.Contains(l, "| \x60zerg version") {
					continue
				}
				keep = append(keep, l)
			}
			cpfcT_write(t, p, strings.Join(keep, "\n"))
		}
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("删一格应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "undocumented-command") || !strings.Contains(out, "zerg version") {
		t.Fatalf("应点名 undocumented-command + zerg version\n%s", out)
	}
}

// ── 3. 方向 B 负控：页里塞不存在的命令 ⇒ 红 ─────────────────────────────────
func TestCpfcS03_负控_页里塞不存在的命令(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		p := cpfcPyJoin(root, cpfcPages[0])
		cpfcT_write(t, p, cpfcT_read(t, p)+"\n见 \x60zerg frobnicate --xyz\x60。\n")
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("塞假命令应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "nonexistent-command-documented") {
		t.Fatalf("应点名 nonexistent-command-documented\n%s", out)
	}
}

// ── 4. 方向 B 负控：真命令 + 假旗标 ⇒ 红 ────────────────────────────────────
func TestCpfcS04_负控_真命令假旗标(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		p := cpfcPyJoin(root, cpfcPages[0])
		cpfcT_write(t, p, cpfcT_read(t, p)+"\n见 \x60zerg version --frob\x60。\n")
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("假旗标应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "nonexistent-parameter-documented") || !strings.Contains(out, "--frob") {
		t.Fatalf("应点名 nonexistent-parameter-documented + --frob\n%s", out)
	}
}

// ── 5. 方向 B 负控：假帮助主题（真跑判）⇒ 红 ────────────────────────────────
func TestCpfcS05_负控_假帮助主题(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		p := cpfcPyJoin(root, cpfcPages[0])
		cpfcT_write(t, p, cpfcT_read(t, p)+"\n见 \x60zerg help frobnicate\x60。\n")
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("假主题应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "nonexistent-help-topic") {
		t.Fatalf("应点名 nonexistent-help-topic\n%s", out)
	}
}

// ── 6. 空转负控：公开面 0 串 ⇒ rc=2（空转 ≠ 绿）────────────────────────────
func TestCpfcS06_负控_公开面零串空转(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		for _, rel := range cpfcPages {
			cpfcT_write(t, cpfcPyJoin(root, rel),
				"<!-- ===== 生成区 BEGIN · 夹具 ===== -->\n夹具：无命令引用\n<!-- ===== 生成区 END ===== -->\n")
		}
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("空转应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "空转") {
		t.Fatalf("空转档应点名「空转」\n%s", out)
	}
}

// ── 7. 缺件负控：publish/docs 不在 ⇒ rc=2（缺件 ≠ 绿）──────────────────────
func TestCpfcS07_负控_公开面缺件(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, nil)
	if err := os.RemoveAll(cpfcPyJoin(fx, cpfcFaceDir)); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("缺件应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "缺件") {
		t.Fatalf("缺件档应点名「缺件」\n%s", out)
	}
}

// ── 8. 生成区标记找不到 ⇒ rc=2（判不了 ≠ 绿）───────────────────────────────
func TestCpfcS08_负控_生成区标记找不到(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		for _, rel := range cpfcPages {
			p := cpfcPyJoin(root, rel)
			cpfcT_write(t, p, strings.ReplaceAll(cpfcT_read(t, p), "生成区 BEGIN", "生成区 BROKEN"))
		}
	})
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("标记找不到应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "生成区标记找不到") {
		t.Fatalf("应点名「生成区标记找不到」\n%s", out)
	}
}

// ── 9. `--json` 字段名打错 ⇒ rc=2 ＋ 印可选字段表（不静默、不当零命中）────────
func TestCpfcS09_负控_json字段名打错不静默(t *testing.T) {
	rc, out, errb := cpfcT_run([]string{"--json=nope", "--no-self-test"})
	if rc != 2 {
		t.Fatalf("字段名打错应 rc=2，得 %d", rc)
	}
	if out != "" {
		t.Fatalf("用法错档 stdout 应为空，得 %q", out)
	}
	for _, want := range []string{"字段名打错 ≠ 零命中", "可选字段表", "\x60counts\x60", "\x60hits\x60", "\x60topics\x60"} {
		if !strings.Contains(errb, want) {
			t.Fatalf("stderr 应含 %q\n%s", want, errb)
		}
	}
}

// ── 10. 未知旗标 ⇒ rc=2（argparse 面）──────────────────────────────────────
func TestCpfcS10_负控_未知旗标(t *testing.T) {
	rc, out, errb := cpfcT_run([]string{"--bogus"})
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
	if out != "" {
		t.Fatalf("用法错档 stdout 应为空，得 %q", out)
	}
	if !strings.Contains(errb, "unrecognized arguments: --bogus") {
		t.Fatalf("stderr 应逐字报 unrecognized arguments\n%s", errb)
	}
}

// ── 11. 机器面键表牙：`counts` 的键名必须逐字（`dangerous` 而非 `danger`）────
//
//	★ 这条是「我的首版实现错」的回归牙：首版把键写成 `"danger"` ⇒ 5 格对拍失配 ⇒ 此用例立红。
func TestCpfcS11_机器面_counts键表逐字(t *testing.T) {
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, nil)
	rc, out, _ := cpfcT_run([]string{"--root", fx, "--zerg", z, "--json", "counts", "--no-self-test"})
	if rc != 0 {
		t.Fatalf("真页夹具应 rc=0，得 %d\n%s", rc, out)
	}
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "{\"counts\":") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("应打出 {\"counts\": …} 机器面行\n%s", out)
	}
	for _, want := range []string{
		"\"commands\":", "\"dangerous\":", "\"generic\":", "\"group_ref\":", "\"spans\":", "\"tree_total\":",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("counts 键表应含 %s（键名逐字 · ✗ 不许写成 danger）\n%s", want, line)
		}
	}
	if strings.Contains(line, "\"danger\":") {
		t.Fatalf("键名错写成 danger（真源逐字是 dangerous）\n%s", line)
	}
}

// ── 12. 自检夹具自回收（A 类止血牙）：`--self-test` 前后同前缀临时目录计数 delta=0 ──
//
//	★ 删掉主件 `defer os.RemoveAll(base)` ⇒ 本用例立红。
func TestCpfcS12_自检夹具自回收_delta0(t *testing.T) {
	root := cpfcT_repoRoot(t)
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	if !cpfcIsFile(cpfcPyJoin(root, "bin", "zerg")) {
		t.Skip("仓内 bin/zerg 不在 ⇒ 自检前置缺件")
	}
	before := cpfcT_countTempPrefix(t, "pfcmds-")
	rc, _, _ := cpfcT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d", rc)
	}
	after := cpfcT_countTempPrefix(t, "pfcmds-")
	if after != before {
		t.Fatalf("自检档应自回收（同前缀临时目录 delta 应 0）· before=%d after=%d", before, after)
	}
}

func cpfcT_countTempPrefix(t *testing.T, prefix string) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Skipf("临时目录读不了：%v", err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}
