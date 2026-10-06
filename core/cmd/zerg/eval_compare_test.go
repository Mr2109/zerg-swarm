// eval_compare_test.go —— 件 eval_compare.go 的成对负控（✗ 禁恒绿 · 前缀 TestEcv）。
//
//	正控：与 py scripts/evals/compare.py 的口径同构（含 Unicode 空白/行界、末 token 剥离、排序、百分比）。
//	负控：把每条关键语义各钉一枚「反例」—— 日后谁把实现退化（截断不再剥末 token / 用 Go TrimSpace /
//	用 strings.SplitLines 近似 / 用 filepath.Join / 按 map 迭代序输出），对应用例立刻红。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ---- 原语同构 ----

func TestEcvPyJoinSemantics(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"", "results/agent/runs", "results/agent/runs"},
		{"dir", "", "dir/"},
		{"a", "", "a/"},
		{"a", "b", "a/b"},
		{"a/", "b", "a/b"},
		{"a", "../b", "a/../b"},
		{"repo", "/abs", "/abs"},
	}
	for _, c := range cases {
		if got := ecvPyJoin(c.a, c.b); got != c.want {
			t.Fatalf("ecvPyJoin(%q,%q)=%q 期望 %q", c.a, c.b, got, c.want)
		}
	}
	if got := ecvPyJoin("/root", "results/agent/runs", "r1"); got != "/root/results/agent/runs/r1" {
		t.Fatalf("多段 join 错: %q", got)
	}
}

func TestEcvPyStripUnicode(t *testing.T) {
	// \x1c-\x1f、\x85、\xa0 都是 py 的空白 ⇒ 必剥（Go unicode.IsSpace 不认 \x1c-\x1f ⇒ 反例在此）。
	for _, s := range []string{
		"\x1c\x1d\x1e\x1f x \x1c", "\u0085x\u0085", "\u00a0x\u00a0", "\v\fx\f\v", " 	 x 	 ",
	} {
		if got := ecvPyStrip(s); got != "x" {
			t.Fatalf("ecvPyStrip(%q)=%q 期望 %q", s, got, "x")
		}
	}
	// 负例：零宽空格 U+200B 不是 py 空白 ⇒ 不剥。
	if got := ecvPyStrip("\u200bx\u200b"); got != "\u200bx\u200b" {
		t.Fatalf("U+200B 被误剥: %q", got)
	}
}

func TestEcvPySplitLines(t *testing.T) {
	if got := ecvPySplitLines("a\nb\n"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("尾界产空尾项: %#v", got)
	}
	if got := ecvPySplitLines("a\n\nb"); len(got) != 3 || got[1] != "" {
		t.Fatalf("空行丢失: %#v", got)
	}
	if got := ecvPySplitLines(""); len(got) != 0 {
		t.Fatalf("空串应为空切片: %#v", got)
	}
	if got := ecvPySplitLines("a\x0bb\x1cc"); len(got) != 3 {
		t.Fatalf("py 行界（\\v/\\x1c）未复刻: %#v", got)
	}
}

// ---- 正则口径（末 token 剥离）—— 成对正/负控 ----

func TestEcvParseRunRegexStripsLastToken(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "layer2-basic.txt"), "✅ alpha\n❌ beta extra\n乱码行不应命中\n")
	got, err := ecvParseRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 正控：单 token 名保留、多 token 名剥掉末 token、非 ✅❌ 行不进面。
	if it, ok := got["layer2-basic:alpha"]; !ok || !it.ok {
		t.Fatalf("alpha 未按 ✅ 记入: %#v", got)
	}
	if it, ok := got["layer2-basic:beta"]; !ok || it.ok {
		t.Fatalf("beta 未按 ❌ 记入: %#v", got)
	}
	// 负控：绝不产生带末 token 的名（若退化成正则截断/不剥 ⇒ 此断言红）。
	if _, ok := got["layer2-basic:beta extra"]; ok {
		t.Fatalf("末 token 未被剥离（实现退化）")
	}
	if len(got) != 2 {
		t.Fatalf("命中条数应为 2，得 %d: %#v", len(got), got)
	}
}

func TestEcvParseRunStripUnicodeWS(t *testing.T) {
	dir := t.TempDir()
	// 行首带 NBSP（py 空白）⇒ strip 后仍命正则；若用 Go TrimSpace 则 NBSP 不剥 ⇒ 不命中 ⇒ 此用例红。
	mustWrite(t, filepath.Join(dir, "layer3-complex.txt"), "\u00a0✅ 名称 one\n")
	got, err := ecvParseRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if it, ok := got["layer3-complex:名称"]; !ok || !it.ok {
		t.Fatalf("NBSP 未被剥 ⇒ 行未命中: %#v", got)
	}
}

// ---- 入口三面 ----

func TestEvalCompareGateMainUsageEmpty(t *testing.T) {
	t.Setenv("ZERG_EVALS_DIR", t.TempDir()) // base 不存在 ⇒ runs=[]
	var out, errb bytes.Buffer
	rc := evalCompareGateMain(&out, &errb, nil)
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0", rc)
	}
	want := "用法: compare.py <run1> <run2>\n可用运行: []\n"
	if out.String() != want {
		t.Fatalf("stdout=%q 期望 %q", out.String(), want)
	}
}

func TestEvalCompareGateMainUsageLists(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZERG_EVALS_DIR", root)
	base := filepath.Join(root, "results", "agent", "runs")
	for _, n := range []string{"zeta", "alpha", "mid"} {
		if err := os.MkdirAll(filepath.Join(base, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	rc := evalCompareGateMain(&out, &errb, []string{"only"}) // len<2 ⇒ 用法档
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	// 负控：必须是排序后、py list-repr 形（若按 readdir 原序输出 ⇒ 此断言红）。
	want := "用法: compare.py <run1> <run2>\n可用运行: ['alpha', 'mid', 'zeta']\n"
	if out.String() != want {
		t.Fatalf("stdout=%q 期望 %q", out.String(), want)
	}
}

func TestEvalCompareGateMainCompare(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZERG_EVALS_DIR", root)
	base := filepath.Join(root, "results", "agent", "runs")
	mustWrite(t, filepath.Join(base, "r1", "layer2-basic.txt"), "✅ alpha\n❌ beta extra\n乱码行\n")
	mustWrite(t, filepath.Join(base, "r2", "layer2-basic.txt"), "✅ alpha\n✅ beta\n✅ gamma extra-col\n")
	var out, errb bytes.Buffer
	rc := evalCompareGateMain(&out, &errb, []string{"r1", "r2"})
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errb.String())
	}
	want := "📊 对比: r1 vs r2\n" +
		"==================================================\n" +
		"⬆️ 提升项:\n" +
		"   beta\n" +
		"   gamma（新增）\n" +
		"= 持平项:\n" +
		"   alpha\n" +
		"==================================================\n" +
		"r1: 1/2 通过 (50%)\n" +
		"r2: 3/3 通过 (100%)\n" +
		"✅ 无回归——能力持平或提升\n"
	if out.String() != want {
		t.Fatalf("stdout 逐字节不同:\n--- got ---\n%s--- want ---\n%s", out.String(), want)
	}
}

func TestEvalCompareGateMainRegressionRed(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZERG_EVALS_DIR", root)
	base := filepath.Join(root, "results", "agent", "runs")
	mustWrite(t, filepath.Join(base, "a", "layer2-basic.txt"), "✅ x\n")
	mustWrite(t, filepath.Join(base, "b", "layer2-basic.txt"), "❌ x\n")
	var out, errb bytes.Buffer
	if rc := evalCompareGateMain(&out, &errb, []string{"a", "b"}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !bytes.Contains(out.Bytes(), []byte("⬇️ 退化项（回归！）:\n   x\n")) ||
		!bytes.Contains(out.Bytes(), []byte("⚠️ 有回归——检查退化项！\n")) {
		t.Fatalf("退化档未被点红: %q", out.String())
	}
}

func TestEvalCompareGateMainMissingRuns(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZERG_EVALS_DIR", root)
	var out, errb bytes.Buffer
	rc := evalCompareGateMain(&out, &errb, []string{"nope1", "nope2"})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "📊 对比: nope1 vs nope2\n" +
		"==================================================\n" +
		"==================================================\n" +
		"nope1: 0/0 通过 (0%)\n" +
		"nope2: 0/0 通过 (0%)\n" +
		"✅ 无回归——能力持平或提升\n"
	if out.String() != want {
		t.Fatalf("空运行档 stdout=%q 期望 %q", out.String(), want)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
