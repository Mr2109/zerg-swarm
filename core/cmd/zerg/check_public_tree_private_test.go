// check_public_tree_private_test.go —— 成对负控（✗ 禁恒绿）。
//
// 覆盖：黑名单命中（rc=1 且点名）＋ **抽掉规则后退回漏报**（配对负控，有牙）· docs/ 正向断言
// （非法 rc=1 逐条点名 · 空转 rc=2 · 合法树 rc=0）· `--json` 拒收 rc=2 · 未知旗标 argparse 文案 ·
// py 路径/空白语义三态 · `--self-test` 全绿。
package main

import (
	"os"
	"strings"
	"testing"
)

// TestMain —— 仓外跑测时用 ZERG_REPO 钉根（仓内跑测则由 cptpResolveRoot 从 cwd 上溯）。
func TestMain(m *testing.M) {
	if v := os.Getenv("ZERG_REPO"); v != "" && cptpExists(cptpPyJoin(v, "publish", "private-paths.txt")) {
		cptpRepoOverride = v
	}
	os.Exit(m.Run())
}

func cptpTRepo(t *testing.T) string {
	t.Helper()
	r := cptpResolveRoot()
	if r == "" {
		t.Fatal("无法定位仓根")
	}
	if !cptpExists(cptpPyJoin(r, "publish", "private-paths.txt")) {
		t.Skipf("仓根无黑名单件：%s", r)
	}
	return r
}

func cptpTRun(argv ...string) (int, string, string) {
	var ob, eb strings.Builder
	rc := checkPublicTreePrivateGateMain(&ob, &eb, argv)
	return rc, ob.String(), eb.String()
}

func cptpTWrite(t *testing.T, root, rel string) {
	t.Helper()
	p := cptpPyJoin(append([]string{root}, strings.Split(rel, "/")...)...)
	if err := os.MkdirAll(cptpPyDirname(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCPDBlacklistPaired —— 负向闸的成对负控（命中 vs 抽掉规则后退回漏报）。
func TestCPDBlacklistPaired(t *testing.T) {
	repo := cptpTRepo(t)
	rules := cptpPyJoin(repo, "publish", "private-paths.txt")
	tree := t.TempDir()
	cptpTWrite(t, tree, "bin/foo")

	rc, out, _ := cptpTRun(tree, "--rules", rules, "--no-docs-allowlist")
	if rc != 1 {
		t.Fatalf("黑名单命中应 rc=1，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "规则 bin/") {
		t.Fatalf("未点名规则 bin/\n%s", out)
	}

	r, err := cptpLoadRules(rules)
	if err != nil {
		t.Fatal(err)
	}
	var rest []string
	for _, d := range r.dirs {
		if d != "bin/" {
			rest = append(rest, d)
		}
	}
	rest = append(rest, r.files...)
	stripped := cptpPyJoin(t.TempDir(), "rules.txt")
	cptpWriteRules(stripped, rest)
	rc2, out2, _ := cptpTRun(tree, "--rules", stripped, "--no-docs-allowlist")
	if rc2 != 0 {
		t.Fatalf("抽掉规则后应退回漏报 rc=0，实得 %d\n%s", rc2, out2)
	}
}

// TestCPDDocsAllowlistGate —— 正向断言的成对探针（非法 rc=1 · 空转 rc=2 · 合法 rc=0）。
func TestCPDDocsAllowlistGate(t *testing.T) {
	repo := cptpTRepo(t)
	rules := cptpPyJoin(repo, "publish", "private-paths.txt")
	cwd, _ := os.Getwd()
	allow := cptpDocsAllowlist(cwd, repo)
	if allow.err != "" {
		t.Skipf("合法集读不出：%s", allow.err)
	}
	if len(allow.exact) == 0 {
		t.Skip("合法集为空")
	}

	tree := t.TempDir()
	cptpTWrite(t, tree, "docs/zh/x.md")
	bad := "docs/" + "临时非法" + "/a.md"
	cptpTWrite(t, tree, bad)
	rc, out, _ := cptpTRun(tree, "--rules", rules)
	if rc != 1 {
		t.Fatalf("docs 非法应 rc=1，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, bad) {
		t.Fatalf("未逐条点名 %s\n%s", bad, out)
	}

	empty := t.TempDir()
	cptpTWrite(t, empty, "README.md")
	rc2, out2, _ := cptpTRun(empty, "--rules", rules)
	if rc2 != 2 {
		t.Fatalf("docs 空转应 rc=2，实得 %d\n%s", rc2, out2)
	}

	legal := t.TempDir()
	cptpTWrite(t, legal, "docs/zh/x.md")
	cptpTWrite(t, legal, "docs/en/y.md")
	rc3, out3, _ := cptpTRun(legal, "--rules", rules)
	if rc3 != 0 {
		t.Fatalf("合法树应 rc=0，实得 %d\n%s", rc3, out3)
	}
}

// TestCPDJSONRefused —— 无机器读面 ⇒ rc=2 且 stdout 静默。
func TestCPDJSONRefused(t *testing.T) {
	rc, out, errs := cptpTRun("--json")
	if rc != 2 {
		t.Fatalf("--json 应 rc=2，实得 %d", rc)
	}
	if out != "" {
		t.Fatalf("--json 不应有 stdout：%q", out)
	}
	if !strings.Contains(errs, "无机器读面") {
		t.Fatalf("stderr 未声明无机器读面：%q", errs)
	}
}

// TestCPDUnknownFlagArgparse —— argparse 文案面逐字。
func TestCPDUnknownFlagArgparse(t *testing.T) {
	rc, out, errs := cptpTRun("--bogus")
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(out+errs, cptpProg+": error: unrecognized arguments: --bogus") {
		t.Fatalf("缺 argparse 文案：%q", out+errs)
	}
	if !strings.HasPrefix(out+errs, "usage: "+cptpProg) {
		t.Fatalf("缺 usage 前缀：%q", out+errs)
	}
}

// TestCPDPathAndStripSemantics —— 钉 py join/normpath/strip 三态。
func TestCPDPathAndStripSemantics(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "../b"}, "a/../b"},  // ① 中间 .. 原样留
		{[]string{"repo", "/abs"}, "/abs"}, // ② 后段绝对丢前段
		{[]string{"a", "", "b"}, "a/b"},    // ③ 空段
		{[]string{"a", ""}, "a/"},
		{[]string{"", "a"}, "a"},
	}
	for _, c := range cases {
		if got := cptpPyJoin(c.parts...); got != c.want {
			t.Errorf("cptpPyJoin(%q) = %q，期望 %q", c.parts, got, c.want)
		}
	}
	if got := cptpPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("cptpPyNormpath 前导双斜杠：got %q 期望 %q", got, "//a/c")
	}
	if got := cptpPyStrip("\x1c x \x1f"); got != "x" {
		t.Errorf("cptpPyStrip 未剥 U+001C-U+001F：%q", got)
	}
	if got := cptpPyStripSet(`"a"`, `"`); got != "a" {
		t.Errorf("cptpPyStripSet 未剥引号：%q", got)
	}
}

// TestCPDHistorySecretsRetiredTolerant —— 退役容忍（GAP-20261007-16）：历史扫描器 py 移开（已退役）
// 且其 Go 半在盘 ⇒ docs_allowlist 不再报「缺件」（与在盘一致）；Go 半也缺 ⇒ 仍报缺件（牙未放宽）。
func TestCPDHistorySecretsRetiredTolerant(t *testing.T) {
	repo := cptpTRepo(t)
	mk := func(hsecAbsent, goHalf bool) string {
		root := t.TempDir()
		for _, rel := range []string{
			"publish/mirror-public-lib.py", "publish/whitelist.txt",
			"scripts/build/publish-public.sh", "scripts/gates/check-history-secrets.py",
		} {
			b, err := os.ReadFile(cptpPyJoin(repo, rel))
			if err != nil {
				t.Fatalf("真仓缺 %s：%v", rel, err)
			}
			dst := cptpPyJoin(root, rel)
			if err := os.MkdirAll(cptpPyDirname(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if hsecAbsent {
			if err := os.Remove(cptpPyJoin(root, "scripts/gates/check-history-secrets.py")); err != nil {
				t.Fatal(err)
			}
		}
		if goHalf {
			g := cptpPyJoin(root, "core/cmd/zerg/check_history_secrets.go")
			if err := os.MkdirAll(cptpPyDirname(g), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(g, []byte("package main\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}

	// 正控：py 在盘 ⇒ 合法集读得出（不报 err）
	if a := cptpDocsAllowlistWith(cptpTRepo(t), mk(false, false), ""); a.err != "" {
		t.Fatalf("正控（py 在盘）不该报 err：%q", a.err)
	}

	// 退役：py 移开 + Go 半在盘 ⇒ 不再「缺件」，err 为空
	rootB := mk(true, true)
	if b := cptpDocsAllowlistWith(rootB, rootB, ""); b.err != "" {
		t.Fatalf("退役态仍报错（✗ 未退役容忍）：%q", b.err)
	}

	// 负控：py 移开 + 无 Go 半 ⇒ 仍是「缺件」（判据没放宽）
	rootC := mk(true, false)
	if c := cptpDocsAllowlistWith(rootC, rootC, ""); !strings.Contains(c.err, "缺件") {
		t.Fatalf("负控（无 Go 半）应仍报「缺件」，实得 %q", c.err)
	}
}

// TestCPDSelfTest —— 成对负控自检须全绿（真仓 publish/docs 非空）。
func TestCPDSelfTest(t *testing.T) {
	cptpTRepo(t)
	rc, out, _ := cptpTRun("--self-test")
	if rc != 0 {
		t.Fatalf("自检应 rc=0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检：") {
		t.Fatalf("自检缺摘要行\n%s", out)
	}
	if strings.Contains(out, "❌") {
		t.Fatalf("自检出现红格\n%s", out)
	}
}
