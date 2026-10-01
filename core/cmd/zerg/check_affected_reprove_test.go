// check_affected_reprove_test.go —— `check_affected_reprove.go` 的成对负控测试件
// （仓外夹具 · ✗ 不碰真目标 · 子件解释器钉 /usr/bin/python3）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// carvTRun —— 同进程调用入口（合并 stdout+stderr，对应 py `_run_fixture` 口径）。
func carvTRun(t *testing.T, argv ...string) (int, string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	var buf bytes.Buffer
	rc := checkAffectedReproveGateMain(&buf, &buf, argv)
	return rc, buf.String()
}

// carvTChdir —— chdir 到夹具（使 carvDefaultRoot 落在夹具根）。
func carvTChdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// ★ 恢复**必须注册在本 helper 内**（GAP-20261002-01）：本 helper 有 10 处调用点，
	// 若指望调用方自己的 cleanup，那时取到的 `old` 已是「chdir 之后的夹具目录」——
	// 而夹具是 `t.TempDir()`，测试一结束就被清理删除 ⇒ 恢复落进已删路径 ⇒ cwd 永久
	// 停在它上面 ⇒ 包内后续所有依赖 cwd 的测试**级联全红**。此处恢复失败也**不许静默**。
	// （cleanup 是 LIFO：先注册的先跑，故本项的恢复跑在调用方 cleanup 之后 ⇒ 最终值正确。）
	t.Cleanup(func() {
		if rerr := os.Chdir(old); rerr != nil {
			t.Errorf("carvTChdir 回收 cwd 失败：%v（原 cwd %s）", rerr, old)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

// carvTFixture —— 造一枚仓外夹具（健康态：四个子件全绿）。
func carvTFixture(t *testing.T) string {
	t.Helper()
	fix := carvMkFixture(t.TempDir(), "t")
	if fix == "" {
		t.Fatal("造夹具失败")
	}
	return fix
}

func TestCarvFixtureList(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	rc, out := carvTRun(t)
	if rc != 2 {
		t.Fatalf("无参应 rc=2（用法档），实 rc=%d\n%s", rc, out)
	}
	rc, out = carvTRun(t, "--list")
	if rc != 0 {
		t.Fatalf("--list 应 rc=0，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "scripts/gates/gate-a.py") {
		t.Fatalf("候选样本缺 gate-a.py\n%s", out)
	}
	if !strings.Contains(out, "候选样本 4 件") {
		t.Fatalf("候选数应为 4（自身基名被剔除）\n%s", out)
	}
	if strings.Contains(out, "check-affected-reprove.py") {
		t.Fatalf("候选样本混入自身基名（自指剔除失效）\n%s", out)
	}
}

func TestCarvPositiveDependency(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	rc, out := carvTRun(t, "--files", "scripts/gates/gate-a.py")
	if rc != 0 {
		t.Fatalf("正控应 rc=0，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "**真依赖** ✓") {
		t.Fatalf("正控应判「真依赖」\n%s", out)
	}
	if !strings.Contains(out, "· 真依赖 1 ·") {
		t.Fatalf("读数行应见「真依赖 1」\n%s", out)
	}
	if !strings.Contains(out, "复原逐字节一致") {
		t.Fatalf("应复原逐字节\n%s", out)
	}
}

func TestCarvOrphanNoStepRef(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	g := carvPyJoin(fix, "scripts", "gates")
	if err := os.WriteFile(carvPyJoin(g, "gate-z.py"),
		[]byte("#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out := carvTRun(t, "--files", "scripts/gates/gate-z.py")
	if rc != 2 {
		t.Fatalf("无步引用应 rc=2，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "无步引用") {
		t.Fatalf("应点名「无步引用」\n%s", out)
	}
}

// 负控（防空绿）：把 sha 弄哑 ⇒ 扰动生效断言瞎 ⇒ rc=2「扰动没生效」。
func TestCarvMutedSHANotGreen(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	realSHA := carvSHA
	carvSHA = func(string) string { return "MUTE" }
	defer func() { carvSHA = realSHA }()
	rc, out := carvTRun(t, "--files", "scripts/gates/gate-a.py")
	if rc != 2 {
		t.Fatalf("弄哑 sha 应 rc=2（不许静默当绿），实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "扰动没生效") {
		t.Fatalf("应点名「扰动没生效」\n%s", out)
	}
}

// 语义词族 · 只报档：rc=1 落 REPORT ⇒ 不影响门退出码 ⇒ 判「真无依赖」· rc=1。
func TestCarvSemanticReportOnly(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	rc, out := carvTRun(t, "--mutation", "semantic", "--files", "scripts/gates/gate-d.py")
	if rc != 1 {
		t.Fatalf("只报档 + semantic 应 rc=1，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "不影响门退出码") {
		t.Fatalf("应见「不影响门退出码」格\n%s", out)
	}
}

// 靶件零残留：正常跑完，夹具子件 sha 与开跑前同值（复原核验有牙）。
func TestCarvNoResidue(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	tgt := carvPyJoin(fix, "scripts", "gates", "gate-a.py")
	before := carvSHA(tgt)
	rc, out := carvTRun(t, "--files", "scripts/gates/gate-a.py")
	if rc != 0 {
		t.Fatalf("应 rc=0，实 rc=%d\n%s", rc, out)
	}
	if carvSHA(tgt) != before {
		t.Fatalf("靶件残留扰动（sha 变了）")
	}
}

// 未知旗标 ⇒ argparse 档 rc=2（不静默当绿）。
func TestCarvUnknownFlag(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	rc, _ := carvTRun(t, "--nope")
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，实 rc=%d", rc)
	}
}

// 自检：14 例全过 ⇒ rc=0（用例数由件内 WANT_CASES 钉住）。
func TestCarvSelfTest(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	rc, out := carvTRun(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检应 rc=0，实 rc=%d\n%s", rc, out)
	}
	n := strings.Count(out, "\n  ✓")
	if n != carvWantCases {
		t.Fatalf("自检用例数 = %d（应 %d）\n%s", n, carvWantCases, out)
	}
	if !strings.Contains(out, "自检全过") {
		t.Fatalf("缺「自检全过」\n%s", out)
	}
}

// ── 回归牙：空步表 ≠ 取不到件（钉死 py `parse_steps` 的两态口径）────────────────────────

// 空步表（件在盘 · 0 条 add_step）≠ 缺件：--list 走空表档 rc=0；--sample 走空表分析档 rc=2
// 且正文点名「样本集为空」——两处都**不许**出现「取不到步表」。修法把 nil 只留给缺件。
func TestCarvEmptyStepsTableNotMissing(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	if err := os.WriteFile(carvPyJoin(fix, carvPrecommitRel),
		[]byte("#!/usr/bin/env bash\n# 空步表夹具：0 条 add_step\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out := carvTRun(t, "--list")
	if rc != 0 {
		t.Fatalf("空步表 + --list 应 rc=0（空步表 ≠ 取不到件），实 rc=%d\n%s", rc, out)
	}
	if strings.Contains(out, "取不到步表") {
		t.Fatalf("空步表被误判成取不到步表\n%s", out)
	}
	if !strings.Contains(out, "候选样本 0 件") {
		t.Fatalf("应见「候选样本 0 件」\n%s", out)
	}
	rc, out = carvTRun(t, "--sample", "3")
	if rc != 2 {
		t.Fatalf("空步表 + --sample 应 rc=2（样本集为空档），实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "样本集为空") {
		t.Fatalf("应见「样本集为空」（非「取不到步表」）\n%s", out)
	}
	if strings.Contains(out, "取不到步表") {
		t.Fatalf("空步表被误判成取不到步表（--sample 档）\n%s", out)
	}
}

// 防修过头：步表件**不在盘** ⇒ nil 仍表「取不到件」⇒ rc=2 且正文点名「取不到步表」。
func TestCarvMissingStepsTableStillHard(t *testing.T) {
	fix := carvTFixture(t)
	carvTChdir(t, fix)
	if err := os.Remove(carvPyJoin(fix, carvPrecommitRel)); err != nil {
		t.Fatal(err)
	}
	rc, out := carvTRun(t, "--list")
	if rc != 2 {
		t.Fatalf("步表件不在盘应 rc=2（取不到步表），实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "取不到步表") {
		t.Fatalf("应点名「取不到步表」\n%s", out)
	}
}
