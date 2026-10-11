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

// ── 1. 正控 + 成对负控（py `self_test` 的九格在 Go 侧真跑）──────────────────────
// ★ 2026-10-04：py 自检本会话新增「格 9 判不了负控（GAP-20260927-86）」⇒ 期望 8 格是除旧读数，
// 改 9（判据真源 = py `self_test()` 的 9 个 `check(...)` 调用 · Go 真搬件已逐格跟）。
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
	if !strings.Contains(out, "⇒ 自检全过（9 格）") {
		t.Fatalf("应打出「⇒ 自检全过（9 格）」\n%s", out)
	}
	if n := strings.Count(out, "   ✓ "); n != 9 {
		t.Fatalf("自检应实跑 9 格全 ✓（成对负控格缺失），得 %d\n%s", n, out)
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
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
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

// ── 13. 格 1 漂移免疫牙（`GAP-20260927-11`）：真页 +1 条新例外 ⇒ 自检仍 rc=0 ──────
//
//	★ 哑化回退旧形态（格 1 = 真页副本 + 写死例外串清洗）⇒ 新例外把格 1 带红 ⇒ 本用例立红。
func TestCpfcS13_格1漂移免疫_新例外不带红自检(t *testing.T) {
	repo := cpfcT_repoRoot(t)
	z := cpfcT_zergArg(t)
	drift := t.TempDir()
	for _, d := range []string{"core/cmd/zerg", "bin", cpfcFaceDir} {
		if err := os.MkdirAll(cpfcPyJoin(drift, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cpfcT_write(t, cpfcPyJoin(drift, "core", "cmd", "zerg", "main.go"), "package main\n")
	zb, err := os.ReadFile(z)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cpfcPyJoin(drift, "bin", "zerg"), zb, 0o755); err != nil {
		t.Fatal(err)
	}
	// 每页在**生成区内**注入一条新例外（真命令 + 假旗标）
	inj := "见 `zerg context ls --frobx`。\n"
	const endMark = "<!-- ===== 生成区 END"
	for _, rel := range cpfcPages {
		raw := cpfcT_read(t, cpfcPyJoin(repo, rel))
		if !strings.Contains(raw, endMark) {
			t.Fatalf("真页缺生成区 END 标记：%s", rel)
		}
		cpfcT_write(t, cpfcPyJoin(drift, rel), strings.Replace(raw, endMark, inj+endMark, 1))
	}
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(drift); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cpfcT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("漂移树（真页 +1 条新例外）⇒ 自检应漂移免疫 rc=0，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "格 1 正控（合成页：命令树") {
		t.Fatalf("格 1 应为合成页形态\n%s", out)
	}
}

// ── B 类（读失败静默 · 目录面）回归牙 ─────────────────────────────────────────
// 公开面内一个子目录锁成 chmod 000（**仓外夹具 · ✗ 不碰仓内件**）⇒ 读失败登记面（cpfcReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐处点名那个锁死目录。把登记/判红删回静默（原 py `os.walk(onerror=None)`），
// 本用例必红（改前该目录静默不入判 ⇒ rc=0）。成对先例：`check_bypass_idioms_test.go` 的
// `TestCheckBypassIdiomsReadFailTeeth` · `check_selftest_knobs_test.go` 的 `TestCskReadFailTeeth`。
func TestCpfcS14ReadFailTeethDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	z := cpfcT_zergArg(t)
	fx := cpfcT_fixture(t, func(root string) {
		locked := cpfcPyJoin(root, cpfcFaceDir, "zlocked")
		if err := os.MkdirAll(locked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	})
	rc, out, errb := cpfcT_run([]string{"--root", fx, "--zerg", z, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("读失败回归牙：锁死目录 ⇒ 应 rc=2（登记 + 判红），得 rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "zlocked") {
		t.Fatalf("读失败回归牙：stderr 须带 BLOCKED 头 + 点名 zlocked，得 err=%q", errb)
	}
	// 成对反控（✗ 禁恒红）：同构夹具但目录可读 ⇒ rc in {0,1} 且 stderr 无 BLOCKED。
	ok := cpfcT_fixture(t, nil)
	rcOK, _, errOK := cpfcT_run([]string{"--root", ok, "--zerg", z, "--no-self-test"})
	if rcOK == 2 || strings.Contains(errOK, "BLOCKED") {
		t.Fatalf("成对反控失败：可读夹具应 rc in {0,1} 且无 BLOCKED，得 rc=%d err=%q", rcOK, errOK)
	}
}
