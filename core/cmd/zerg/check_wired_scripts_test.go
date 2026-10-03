// check_wired_scripts_test.go —— 判据件 `scripts/gates/check-wired-scripts.py` 的 Go 真搬
// （`check_wired_scripts.go`）的**成对负控**单测件。
//
// 设计要点（✗ 禁恒绿）：
//
//	· 断言 A 的牙：只在注释行 / 纯清单项 / 文案里出现 ⇒ 必须仍算「未挂」（拿掉判据即红）。
//	· 断言 B 的牙：零引用且未登记 ⇒ 只报告档报出、`--strict-report` rc=1；登记后 rc=0。
//	· 记录面的牙：台账点名不充作引用 ⇒ 那只件仍是零引用。
//	· A 类真口子（tempdir 泄漏）的回归钉：自检跑前/跑后同前缀残留 **delta=0**。
//	· py 路径/原语语义差逐格钉（join 的 `..`/绝对后段/空段 · normpath 双斜杠 · fnmatch `*` 跨 `/` · strip 的 \x1c）。
//
// ★ 夹具一律显式传 `--whitelist`（同 py 自检的做法）：内置白名单的 26 件名在夹具里**必然悬空**
// ⇒ 不传就会 rc=2（那是判据在正常工作，不是缺陷）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cwsRunT(t *testing.T, argv ...string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	rc := cwsRunMain(&buf, &buf, argv)
	return buf.String(), rc
}

func cwsWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 基线夹具：一只闸 + 一只已接线门脚本 + 一只零引用 helper（仅被记录面点名）。
func cwsBaseFixture(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	cwsWrite(t, filepath.Join(d, "scripts/gates/precommit-gates.sh"),
		"#!/usr/bin/env bash\nset -u\npython3 scripts/check-alpha.py --scope x\n")
	cwsWrite(t, filepath.Join(d, "scripts/check-alpha.py"), "# ck\n")
	cwsWrite(t, filepath.Join(d, "scripts/helper.py"), "# zero-ref\n")
	cwsWrite(t, filepath.Join(d, "scripts/run-gates.sh"),
		"#!/usr/bin/env bash\nbash scripts/gates/precommit-gates.sh\n")
	cwsWrite(t, filepath.Join(d, "Makefile"), "check:\n	bash scripts/run-gates.sh\n")
	cwsWrite(t, filepath.Join(d, "docs/项目文档/v9.9/债务台账-20990101.md"), "helper.py 是零引用\n")
	return d
}

func cwsWL(t *testing.T, entries []map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "wl.json")
	b, _ := json.Marshal(entries)
	cwsWrite(t, p, string(b))
	return p
}

// helper.py 已登记的基线白名单。
func cwsHelperWL(t *testing.T) string {
	return cwsWL(t, []map[string]string{{"name": "helper.py", "reason": "夹具", "date": "2026-10-01"}})
}

func TestCheckWiredScriptsSelftestAllPass(t *testing.T) {
	out, rc := cwsRunT(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d 期望 0\n输出：%s", rc, out)
	}
	if !strings.Contains(out, "19/19 通过") {
		t.Fatalf("自检未 19/19 全过：\n%s", out)
	}
}

// ★ A 类真口子的牙：自检自回收 ⇒ 同前缀 tempdir 残留 delta=0（哑化 RemoveAll ⇒ FAIL）。
func TestCheckWiredScriptsSelftestTempdirDeltaZero(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	pre := cwsCountSelftestDirs(t)
	if _, rc := cwsRunT(t, "--self-test"); rc != 0 {
		t.Fatalf("自检 rc=%d", rc)
	}
	post := cwsCountSelftestDirs(t)
	if post-pre != 0 {
		t.Fatalf("自检 tempdir 残留 delta=%d（期望 0）", post-pre)
	}
}

func cwsCountSelftestDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "wired-scripts-selftest-") {
			n++
		}
	}
	return n
}

// 断言 B 的牙：零引用且未登记 ⇒ 报出；登记后 rc=0。
func TestCheckWiredScriptsAssertionBTeeth(t *testing.T) {
	d := cwsBaseFixture(t)
	cwsWrite(t, filepath.Join(d, "scripts/dead.py"), "# dead zero-ref\n")

	wlH := cwsHelperWL(t)
	out, rc := cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", wlH)
	if rc != 0 {
		t.Fatalf("只报告档 rc=%d 期望 0\n%s", rc, out)
	}
	if !strings.Contains(out, "未登记 1 只") || !strings.Contains(out, "dead.py") {
		t.Fatalf("零引用未登记件未被报出：\n%s", out)
	}

	out, rc = cwsRunT(t, "--root", d, "--no-self-test", "--strict-report", "--whitelist", wlH)
	if rc != 1 {
		t.Fatalf("strict-report 下 rc=%d 期望 1\n%s", rc, out)
	}

	wlBoth := cwsWL(t, []map[string]string{
		{"name": "helper.py", "reason": "夹具", "date": "2026-10-01"},
		{"name": "dead.py", "reason": "夹具", "date": "2026-10-01"},
	})
	out, rc = cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", wlBoth)
	if rc != 0 {
		t.Fatalf("登记后 rc=%d 期望 0\n%s", rc, out)
	}
	if !strings.Contains(out, "未登记 0 只") {
		t.Fatalf("登记后仍有未登记：\n%s", out)
	}
}

// 断言 A 的牙：注释 / 纯清单项 / 文案都不算接线（哑化 bare_item / call 判定 ⇒ FAIL）。
func TestCheckWiredScriptsAssertionACommentNotWiring(t *testing.T) {
	d := cwsBaseFixture(t)
	cwsWrite(t, filepath.Join(d, "scripts/check-gamma.py"), "# ck\n")
	cwsWrite(t, filepath.Join(d, "scripts/gates/precommit-gates.sh"),
		"#!/usr/bin/env bash\nset -u\npython3 scripts/check-alpha.py --scope x\n"+
			"# python3 scripts/check-gamma.py\n"+"EXCLUDES=(\n  \"scripts/check-gamma.py\"\n)\n"+
			"echo \"按名调用 check-gamma.py\"\n")
	out, rc := cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", cwsHelperWL(t))
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0\n%s", rc, out)
	}
	if !strings.Contains(out, "A 命中 1") || !strings.Contains(out, "check-gamma.py") {
		t.Fatalf("注释/清单项/文案被误判成接线：\n%s", out)
	}
}

// 断言 A 的正面：命令位调用（带引号 + 变量前缀）必须算接线。
func TestCheckWiredScriptsAssertionACallIsWiring(t *testing.T) {
	d := cwsBaseFixture(t)
	cwsWrite(t, filepath.Join(d, "scripts/gates/check-epsilon.py"), "# ck\n")
	cwsWrite(t, filepath.Join(d, "scripts/gates/precommit-gates.sh"),
		"#!/usr/bin/env bash\nset -u\npython3 scripts/check-alpha.py --scope x\n"+
			"python3 \"$Z/scripts/gates/check-epsilon.py\" \"$D\" > \"$D/x.log\" 2>&1\n")
	out, rc := cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", cwsHelperWL(t))
	if rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "A 命中 0") || strings.Contains(out, "check-epsilon.py") {
		t.Fatalf("命令位调用未被认作接线：\n%s", out)
	}
}

// 记录面的牙：台账点名不充作引用。
func TestCheckWiredScriptsRecordSurfaceNotReference(t *testing.T) {
	d := cwsBaseFixture(t)
	out, _ := cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", cwsHelperWL(t))
	if !strings.Contains(out, "零引用 1 只") {
		t.Fatalf("记录面被当成了引用（helper 应仍零引用）：\n%s", out)
	}
}

// 白名单悬空 ⇒ rc=2（配置错）。
func TestCheckWiredScriptsDanglingWhitelistRc2(t *testing.T) {
	d := cwsBaseFixture(t)
	wl := cwsWL(t, []map[string]string{{"name": "ghost.py", "reason": "夹具", "date": "2026-10-01"}})
	out, rc := cwsRunT(t, "--root", d, "--no-self-test", "--whitelist", wl)
	if rc != 2 || !strings.Contains(out, "悬空") {
		t.Fatalf("悬空白名单 rc=%d 期望 2 且含「悬空」：\n%s", rc, out)
	}
}

func TestCheckWiredScriptsListRules(t *testing.T) {
	o := cwsListRules()
	for _, m := range []string{"断言 A", "断言 B", "precommit-gates.sh", "publish-preflight.sh", "升级路径", "白名单"} {
		if !strings.Contains(o, m) {
			t.Fatalf("--list-rules 缺 %q", m)
		}
	}
}

// py 路径 / 原语语义差逐格钉（✗ 用 filepath.Join / Match 会在此判红）。
func TestCwsPyPathAndPatternSemantics(t *testing.T) {
	if got := cwsPyJoin("a", "../b"); got != "a/../b" {
		t.Fatalf("pyJoin .. 段：got %q", got)
	}
	if got := cwsPyJoin("repo", "/abs"); got != "/abs" {
		t.Fatalf("pyJoin 绝对后段：got %q", got)
	}
	if got := cwsPyJoin("a", ""); got != "a/" {
		t.Fatalf("pyJoin 尾空段：got %q", got)
	}
	if got := cwsPyJoin("", "a"); got != "a" {
		t.Fatalf("pyJoin 首空段：got %q", got)
	}
	if got := cwsPyJoin("a", "", "b"); got != "a/b" {
		t.Fatalf("pyJoin 中空段：got %q", got)
	}
	if got := cwsPyNormpath("//a/b"); got != "//a/b" {
		t.Fatalf("normpath 双前导斜杠：got %q", got)
	}
	if got := cwsPyNormpath("///a/b"); got != "/a/b" {
		t.Fatalf("normpath 三前导斜杠：got %q", got)
	}
	if got := cwsPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Fatalf("normpath 双斜杠+..：got %q", got)
	}
	if !cwsFnmatchCase("docs/项目文档/v9.9/债务台账-x", "docs/项目文档/*/债务台账-*") {
		t.Fatalf("fnmatch 整串 `*` 应跨 /")
	}
	if !cwsFnmatchCase("a/b/c", "a*b*c") {
		t.Fatalf("fnmatch `*` 应跨 /")
	}
	if cwsPyStrip("\x1c x \x1f") != "x" {
		t.Fatalf("pyStrip 应剥 \\x1c-\\x1f")
	}
	if cwsDW("门") != 2 || cwsDW("a") != 1 {
		t.Fatalf("dw 宽度算错")
	}
	if got := cwsPyRelpath("/r", "/r/scripts/x.py", "/r"); got != "scripts/x.py" {
		t.Fatalf("pyRelpath：got %q", got)
	}
}
