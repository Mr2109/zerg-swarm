// check_git_path_outlet_test.go —— 判据件 `check-git-path-outlet.py` 的 Go 搬运配套单测。
//
// 成对负控：夹具一律用**行内绝对拼串**钉死（✗ 不拿被测常量当夹具来源 —— 那样哑化常量后用例会跟着自洽，
// 等于无牙）。「合规必须不报」「裸调必须报 GPO-F1」「摘掉判据必须不报」三条互为负控。
//
// 自指面：本测试件同落 core/cmd/zerg/ ⇒ 在**本门自己**的扫描面内（SCAN_ROOTS 含 core）。
// 全部夹具串一律**分段拼串**（"ls" + "-" + "files" 一类），不落可匹配原串。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGpoPyJoinSemantics(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "../b"}, "a/../b"},
		{[]string{"repo", "/abs"}, "/abs"},
		{[]string{"a", "", "b"}, "a/b"},
		{[]string{"a", ""}, "a/"},
		{[]string{"", "a"}, "a"},
		{[]string{"root", "scripts"}, "root/scripts"},
	}
	for _, c := range cases {
		got := gpoPyJoin(c.parts[0], c.parts[1:]...)
		if got != c.want {
			t.Fatalf("gpoPyJoin(%q)=%q 期望 %q", c.parts, got, c.want)
		}
	}
}

func TestGpoPyNormpathLeadingSlashes(t *testing.T) {
	if got := gpoPyNormpath("//a/b"); got != "//a/b" {
		t.Fatalf("normpath(//a/b)=%q 期望 //a/b", got)
	}
	if got := gpoPyNormpath("///a/b"); got != "/a/b" {
		t.Fatalf("normpath(///a/b)=%q 期望 /a/b", got)
	}
	if got := gpoPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Fatalf("normpath(//a/b/../c)=%q 期望 //a/c", got)
	}
	if got := gpoPyNormpath(""); got != "." {
		t.Fatalf("normpath()=%q 期望 .", got)
	}
}

func TestGpoPyRelpathAncestor(t *testing.T) {
	if got := gpoPyRelpath("/r/scripts/gates/x.py", "/r"); got != "scripts/gates/x.py" {
		t.Fatalf("relpath=%q", got)
	}
	if got := gpoPyRelpath("/a/b", "/a/b"); got != "." {
		t.Fatalf("relpath 同路径=%q 期望 .", got)
	}
	if got := gpoPyRelpath("/a/b", "/a/c"); got != "../b" {
		t.Fatalf("relpath 兄弟=%q 期望 ../b", got)
	}
}

func TestGpoPyStripUnicodeWhitespace(t *testing.T) {
	// py str.strip() 剥的 Unicode 空白含 \x1c-\x1f / \x85 / \xa0 —— 与 unicode.IsSpace 的差集。
	if got := gpoPyStrip("\x1cabc\x1f"); got != "abc" {
		t.Fatalf("PyStrip(\\x1cabc\\x1f)=%q 期望 abc", got)
	}
	if got := gpoPyStrip("\u00a0x\u00a0"); got != "x" {
		t.Fatalf("PyStrip(NBSP x NBSP)=%q 期望 x", got)
	}
}

func TestGpoPySplitLinesCR(t *testing.T) {
	got := gpoPySplitLines("a\r\nb\rc\nd")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("splitlines 段数=%d 期望 %d（%q）", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitlines[%d]=%q 期望 %q", i, got[i], want[i])
		}
	}
}

func TestGpoMaskLineOnlyLineStartComments(t *testing.T) {
	if got := gpoMaskLine("  # x"); got != "" {
		t.Fatalf("行首注释未掩蔽：%q", got)
	}
	if got := gpoMaskLine("// y"); got != "" {
		t.Fatalf("// 注释未掩蔽：%q", got)
	}
	if got := gpoMaskLine("*x"); got != "*x" {
		t.Fatalf("块注释星号后非空白被误掩：%q", got)
	}
	if got := gpoMaskLine("* x"); got != "" {
		t.Fatalf("* 后空白未掩蔽：%q", got)
	}
}

// ── 夹具一律行内绝对拼串（哑化常量必须能杀掉下列用例）──────────────────────────

const (
	tG   = "git"
	tLSF = "ls" + "-" + "files"
	tZ   = "-" + "z"
	tC   = "-" + "c"
	tKU  = "core." + "quote" + "Path"
	tKL  = "core." + "quote" + "path"
	tPR  = "--" + "porcelain"
	tLST = "ls" + "-" + "tree"
	tNM  = "--" + "name-only"
)

func TestGpoConstantsPinned(t *testing.T) {
	if gpoGitTok != tG {
		t.Fatalf("gpoGitTok=%q 期望 %q", gpoGitTok, tG)
	}
	if gpoNLsFiles != tLSF {
		t.Fatalf("gpoNLsFiles=%q 期望 %q", gpoNLsFiles, tLSF)
	}
	if gpoFZ != tZ {
		t.Fatalf("gpoFZ=%q 期望 %q", gpoFZ, tZ)
	}
	if gpoKeyUpper != tKU {
		t.Fatalf("gpoKeyUpper=%q 期望 %q", gpoKeyUpper, tKU)
	}
	if gpoKeyLower != tKL {
		t.Fatalf("gpoKeyLower=%q 期望 %q", gpoKeyLower, tKL)
	}
}

func TestGpoJudgeCompliantClean(t *testing.T) {
	line := tG + " " + tLSF + " " + tZ
	if hits := gpoJudgeWindow(line, nil, 3, line); len(hits) != 0 {
		t.Fatalf("合规写法（带 -z）命中 %d 条（期望 0）：%+v", len(hits), hits)
	}
}

func TestGpoJudgeBareIsFlagged(t *testing.T) {
	line := tG + " " + tLSF
	hits := gpoJudgeWindow(line, nil, 2, line)
	if len(hits) != 1 || hits[0].Rule != "GPO-F1" {
		t.Fatalf("裸调应恰报 1 条 GPO-F1，实得：%+v", hits)
	}
}

func TestGpoJudgeLateC(t *testing.T) {
	line := tG + " " + "status" + " " + tPR + " " + tC + " " + tKL + "=false"
	hits := gpoJudgeWindow(line, nil, 5, line)
	seenF2 := false
	for _, h := range hits {
		if h.Rule == "GPO-F2" {
			seenF2 = true
		}
		if h.Rule == "GPO-F1" || h.Rule == "GPO-F4" {
			t.Fatalf("带 `-c <拼法键>` 的调用被误报为无免疫开关：%+v", h)
		}
	}
	if !seenF2 {
		t.Fatalf("`-c` 后置未报 GPO-F2：%+v", hits)
	}
}

func TestGpoJudgeUpperKeyFlagged(t *testing.T) {
	line := tC + " " + tKU + "=false"
	hits := gpoJudgeWindow(line, nil, 2, line)
	if len(hits) == 0 || hits[0].Rule != "GPO-F3" {
		t.Fatalf("大写拼法应报 GPO-F3，实得：%+v", hits)
	}
}

func TestGpoJudgeF5Consumer(t *testing.T) {
	line := tG + " " + tLSF + " | " + "grep" + " -q " + ".startswith("
	hits := gpoJudgeWindow(line, nil, 6, line)
	seenF5 := false
	for _, h := range hits {
		if h.Rule == "GPO-F5" {
			seenF5 = true
		}
	}
	if !seenF5 {
		t.Fatalf("未经出口直入消费动作应报 GPO-F5：%+v", hits)
	}
	// 合规写法（带 -z）:取件名已是真名 ⇒ 不报 F5（负控）。
	ok := tG + " " + tLSF + " " + tZ + " | " + "grep" + " -q " + ".startswith("
	for _, h := range gpoJudgeWindow(ok, nil, 7, ok) {
		if h.Rule == "GPO-F5" {
			t.Fatalf("合规调用被误报 GPO-F5：%+v", h)
		}
	}
}

func TestGpoJudgeF4Face(t *testing.T) {
	// `ls-tree -r|--name-only` 属**五类面**（F1）；F4 是名单其余形态（show/log --name-only · diff --stat）。
	line := tG + " " + "show" + " " + tNM
	hits := gpoJudgeWindow(line, nil, 3, line)
	seenF4 := false
	for _, h := range hits {
		if h.Rule == "GPO-F4" {
			seenF4 = true
		}
	}
	if !seenF4 {
		t.Fatalf("非五类会转义面（show --name-only）无免疫开关应报 GPO-F4：%+v", hits)
	}
	// 同一形态下若子命令属五类面（ls-tree --name-only）⇒ 报 GPO-F1（面判准的负控）。
	line1 := tG + " " + tLST + " " + tNM
	hits1 := gpoJudgeWindow(line1, nil, 3, line1)
	seenF1 := false
	for _, h := range hits1 {
		if h.Rule == "GPO-F1" {
			seenF1 = true
		}
	}
	if !seenF1 {
		t.Fatalf("ls-tree --name-only 应报 GPO-F1（五类面）：%+v", hits1)
	}
}

func TestGpoRuleDropDisables(t *testing.T) {
	line := tG + " " + tLSF
	if hits := gpoJudgeWindow(line, []string{"GPO-F1", "GPO-F4"}, 2, line); len(hits) != 0 {
		t.Fatalf("摘掉判据后仍报：%+v", hits)
	}
}

func TestGpoJSONEmptyArraysNotNull(t *testing.T) {
	s := gpoJSONNormal(gpoScanResult{}, "report")
	if strings.Contains(s, "null") {
		t.Fatalf("空集合被写成 null：%s", s)
	}
	if !strings.Contains(s, `"hits_all": []`) {
		t.Fatalf("hits_all 空集合未显式置空：%s", s)
	}
	if !strings.Contains(s, `"top": []`) {
		t.Fatalf("top 空集合未显式置空：%s", s)
	}
}

// TestGpoReadFailTeeth —— 读失败不再静默的回归牙：扫面里一份读不动的 `.sh`
// ⇒ 登记进 gpoReadFails ⇒ 入口非空即 stderr BLOCKED 头 ＋ 逐件点名 ＋ rc=2。
func TestGpoReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视 0o000 权限位：本格无鉴别力")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "scripts", "teeth-unreadable.sh")
	if err := os.WriteFile(p, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()
	defer func() { gpoReadFails = nil }() // ★ 自清：本格登记面是包级变量，不许泄给后续 Test
	gpoReadFails = nil
	var out, errb bytes.Buffer
	rc := gitPathOutletGateMain(&out, &errb, []string{"--repo", root})
	if rc != 2 {
		t.Fatalf("读不动的件应拦停 rc=2（✗ 不许静默跳过），得 rc=%d stderr=%q", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "⛔ BLOCKED：判据件读失败") {
		t.Fatalf("stderr 缺 BLOCKED 头：%q", errb.String())
	}
	if !strings.Contains(errb.String(), "scripts/teeth-unreadable.sh") {
		t.Fatalf("stderr 未逐件点名读不动的件：%q", errb.String())
	}
}

func TestGpoSelftestAllPass(t *testing.T) {
	var b bytes.Buffer
	if !gpoSelftest(&b) {
		t.Fatalf("自检未全过：\n%s", b.String())
	}
	if !strings.Contains(b.String(), "自检 27 格 · 失败 0 格") {
		t.Fatalf("自检格数/失败数与预期不符：\n%s", b.String())
	}
}
