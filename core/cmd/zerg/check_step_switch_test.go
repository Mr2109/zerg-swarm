// check_step_switch_test.go —— 与 `check_step_switch.go` 成对（M2 · 批-3 搬运粒 8）。
//
// 定位：**成对负控**（✗ 禁恒绿）—— 每一格都钉住 py 的一条可观测行为，实现回退即红。
// 用例前缀 `TestSsw`（现读 `code find 'func TestSsw'` 零命中 ⇒ 不与在仓测试件撞名）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sswRunMain —— 进程内跑入口，收 rc/stdout/stderr 三面。
func sswRunMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := stepSwitchGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// TestSswHelpAndArgparse —— argparse 面：帮助 / 未知旗标 / 缺值 / 歧义 / 显式值拒收。
func TestSswHelpAndArgparse(t *testing.T) {
	rc, out, errb := sswRunMain(t, "-h")
	if rc != 0 || errb != "" || !strings.HasPrefix(out, "usage: check-step-switch.py [-h] [--file FILE] [--self-test]\n") {
		t.Fatalf("help rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "optional arguments:") || !strings.Contains(out, "  --file FILE           被扫的表件") {
		t.Fatalf("help 正文与 py 3.9 不符: %q", out)
	}

	rc, _, errb = sswRunMain(t, "--bogus")
	if rc != 2 || errb != sswUsage+"check-step-switch.py: error: unrecognized arguments: --bogus\n" {
		t.Fatalf("bogus rc=%d err=%q", rc, errb)
	}

	rc, _, errb = sswRunMain(t, "--file")
	if rc != 2 || !strings.Contains(errb, "check-step-switch.py: error: argument --file: expected one argument\n") {
		t.Fatalf("noval rc=%d err=%q", rc, errb)
	}

	rc, _, errb = sswRunMain(t, "--s")
	if rc != 2 || !strings.Contains(errb, "error: ambiguous option: --s could match --self-test, --selftest\n") {
		t.Fatalf("ambiguous rc=%d err=%q", rc, errb)
	}

	rc, _, errb = sswRunMain(t, "--self")
	if rc != 2 || !strings.Contains(errb, "ambiguous option: --self could match --self-test, --selftest") {
		t.Fatalf("ambiguous2 rc=%d err=%q", rc, errb)
	}

	rc, _, errb = sswRunMain(t, "--help=x")
	if rc != 2 || !strings.Contains(errb, "argument -h/--help: ignored explicit argument 'x'") {
		t.Fatalf("help=x rc=%d err=%q", rc, errb)
	}

	rc, out, _ = sswRunMain(t, "--self-") // 唯一前缀 ⇒ 命中 --self-test
	if rc != 0 || !strings.HasPrefix(out, "  ✓ 正控：") {
		t.Fatalf("--self- rc=%d out=%.40q", rc, out)
	}
}

// TestSswScanGoodBadEmpty —— 判据体：正控 0 违例 · 三类负控各 1 违例 · 空转 0 步。
func TestSswScanGoodBadEmpty(t *testing.T) {
	steps, bad := sswScan(sswGood)
	if len(steps) != 2 || len(bad) != 0 {
		t.Fatalf("正控 steps=%d bad=%d", len(steps), len(bad))
	}
	if _, bad = sswScan(sswBadFlag); len(bad) != 1 || !strings.Contains(bad[0], "甲 缺严格旗标") {
		t.Fatalf("负控甲 bad=%v", bad)
	}
	if _, bad = sswScan(sswBadName); len(bad) != 1 || !strings.Contains(bad[0], "乙 名/档矛盾") {
		t.Fatalf("负控乙 bad=%v", bad)
	}
	if _, bad = sswScan(sswBadDown); len(bad) != 1 || !strings.Contains(bad[0], "tri-report") {
		t.Fatalf("负控乙·降档 bad=%v", bad)
	}
	if steps, _ = sswScan("# 只有注释，没有 add_step\n"); len(steps) != 0 {
		t.Fatalf("空转 steps=%d", len(steps))
	}
}

// TestSswEndToEnd —— 入口端到端：好表 rc=0 / 空表 rc=2 / 缺件 rc=2（夹具走 t.TempDir ⇒ 自回收）。
func TestSswEndToEnd(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sh")
	if err := os.WriteFile(good, []byte(sswGood), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := sswRunMain(t, "--file", good)
	if rc != 0 || errb != "" || !strings.Contains(out, "表自洽（add_step 2 条 · 默认集 tri 1 · tri-report 1 · 甲/乙 违例 0）") {
		t.Fatalf("good rc=%d out=%q err=%q", rc, out, errb)
	}

	empty := filepath.Join(dir, "empty.sh")
	if err := os.WriteFile(empty, []byte("# 只有注释\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb = sswRunMain(t, "--file", empty)
	if rc != 2 || out != "" || !strings.Contains(errb, "都读不到（"+empty+"）⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）") {
		t.Fatalf("empty rc=%d out=%q err=%q", rc, out, errb)
	}

	rc, out, errb = sswRunMain(t, "--file", filepath.Join(dir, "nope.sh"))
	if rc != 2 || out != "" || !strings.Contains(errb, "表件不在盘（") {
		t.Fatalf("missing rc=%d out=%q err=%q", rc, out, errb)
	}
}

// TestSswFixtureViolationSummary —— 违例汇总行（条数/步数/默认集计数）逐字对齐。
func TestSswFixtureViolationSummary(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.sh")
	two := sswBadFlag + "add_step rust \"门C（只报告）\" tri \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-doc-meta.py --missing=fail\"\n"
	if err := os.WriteFile(f, []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := sswRunMain(t, "--file", f)
	if rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.HasPrefix(out, "✗ 步名↔档位↔严格旗标：违例 2 条（扫到 add_step 2 条 · 默认集 tri 2 · tri-report 0）\n") {
		t.Fatalf("汇总行: %q", out)
	}
	if !strings.Contains(out, "   ✗ 乙 名/档矛盾：步「门C（只报告）」名写「只报告」却申报 `tri`（阻断）—— 两处对不上\n") {
		t.Fatalf("乙 明细: %q", out)
	}
}

// TestSswSelfTest —— 自检入口：`--self-test` 与别名 `--selftest` 同版；真仓表自洽 ⇒ rc=0。
func TestSswSelfTest(t *testing.T) {
	rc, out, errb := sswRunMain(t, "--self-test")
	rc2, out2, _ := sswRunMain(t, "--selftest")
	if out != out2 {
		t.Fatalf("别名输出不同:\n%q\n%q", out, out2)
	}
	if errb != "" {
		t.Fatalf("err=%q", errb)
	}
	// 真仓表自洽 ⇒ 全过；若门表被改成有违例，此断言会红（这正是自检的牙）。
	if rc != 0 || rc2 != 0 || !strings.Contains(out, "── 自检：全过 ✓") {
		t.Fatalf("rc=%d rc2=%d out=%q", rc, rc2, out)
	}
}

// TestSswPyJoinSemantics —— 自写 pyJoin 的三类语义差各钉一格（✗ 不用 filepath.Join）。
func TestSswPyJoinSemantics(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "..", "b"}, "a/../b"}, // ① `..` 不归一
		{[]string{"repo", "/abs"}, "/abs"},   // ② 后段绝对 ⇒ 丢弃前段
		{[]string{"a", "", "b"}, "a/b"},      // ③ 中间空段
		{[]string{"a", ""}, "a/"},            // ③ 尾空段留分隔符
		{[]string{"", "a"}, "a"},             // ③ 首空段
		{[]string{"/root", "scripts", "gates", "p.sh"}, "/root/scripts/gates/p.sh"},
	}
	for _, c := range cases {
		if got := sswPyJoin(c.parts...); got != c.want {
			t.Errorf("sswPyJoin(%q)=%q want %q", c.parts, got, c.want)
		}
	}
}

// TestSswPySplitLinesAndReadText —— py 行界（`\r\n` 一元组）与 universal newlines。
func TestSswPySplitLinesAndReadText(t *testing.T) {
	got := sswPySplitLines("a\r\nb\rc\nd")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("splitlines=%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitlines[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if l := sswPySplitLines("x\n"); len(l) != 1 || l[0] != "x" {
		t.Fatalf("尾空行应丢弃: %q", l)
	}
	if l := sswPySplitLines(""); len(l) != 0 {
		t.Fatalf("空串应 0 行: %q", l)
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "crlf.sh")
	if err := os.WriteFile(p, []byte("a\r\nb\rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := sswReadText(p)
	if err != nil || s != "a\nb\nc\n" {
		t.Fatalf("readtext=%q err=%v", s, err)
	}
}
