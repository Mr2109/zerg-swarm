// doc_index_gen_test.go —— Go 真搬的**成对负控**定向单测（M2 · 不接线）。
// ✗ 无 TestMain（包内他件已占）；helper 一律 `gdix` 前缀防撞名。
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func gdixRunT(args ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := docIndexGenGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

func gdixWriteT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gdixFixtureT —— <tmp>/v2.5.8 下三篇（分属 状态文档/使用指南/其他），同族仅此一版 ⇒ 当前版。
func gdixFixtureT(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	d := filepath.Join(base, "v2.5.8")
	gdixWriteT(t, filepath.Join(d, "00-架构.md"), "# 架构标题\n架构正文说明\n")
	gdixWriteT(t, filepath.Join(d, "使用-指南.md"), "使用说明第一行\n使用说明第二行\n")
	gdixWriteT(t, filepath.Join(d, "其他.md"), "杂项说明\n")
	return d
}

const gdixWantContent = "# v2.5.8 版本索引（INDEX——模型查本文档目录入口）\n" +
	"\n" +
	"> 当前版 = 最新档案（模型默认读这层）——历史版查 git/对应目录（勿读全文）\n" +
	"\n" +
	"## 状态文档\n\n| 文件 | 说明 |\n|---|---|\n| [00-架构.md](00-架构.md) | 架构正文说明 |\n" +
	"\n" +
	"## 使用指南\n\n| 文件 | 说明 |\n|---|---|\n| [使用-指南.md](使用-指南.md) | 使用说明第一行 |\n" +
	"\n" +
	"## 其他\n\n| 文件 | 说明 |\n|---|---|\n| [其他.md](其他.md) | 杂项说明 |\n"

func TestGdixGroupOf(t *testing.T) {
	cases := map[string]string{
		"00-架构.md": "状态文档", "07-模块-UI.md": "状态文档", "42-模块-x.md": "状态文档",
		"使用-指南.md": "使用指南", "变更-1.md": "版本变更", "CHANGELOG.md": "版本变更",
		"设计-x.md": "设计", "调研-y.md": "调研", "计划-z.md": "计划", "实施-完成.md": "实施/记录",
		"问题-1.md": "问题/经验", "杂项.md": "其他",
	}
	for in, want := range cases {
		if got := gdixGroupOf(in); got != want {
			t.Fatalf("group_of(%q)=%q want %q", in, got, want)
		}
	}
}

func TestGdixFirstLines(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.md")
	gdixWriteT(t, p, "# 标题行被跳\n## 二级也被跳\n正文 **加粗** [链接]\n")
	if got := gdixFirstLines(p, 1); got != "正文 加粗 链接" {
		t.Fatalf("first_lines=%q", got)
	}
	long := filepath.Join(d, "b.md")
	gdixWriteT(t, long, strings.Repeat("字", 80)+"\n")
	if got := gdixFirstLines(long, 1); len([]rune(got)) != 61 || !strings.HasSuffix(got, "…") {
		t.Fatalf("截断错：len=%d tail=%q", len([]rune(got)), got[len(got)-9:])
	}
}

func TestGdixPyNormpath(t *testing.T) {
	cases := map[string]string{
		"": ".", "a/b": "a/b", "a/./b": "a/b", "a//b": "a/b", "a/b/../c": "a/c",
		"../a": "../a", "/a/../b": "/b", "//a/b": "//a/b", "///a/b": "/a/b", "a/": "a",
	}
	for in, want := range cases {
		if got := gdixPyNormpath(in); got != want {
			t.Fatalf("normpath(%q)=%q want %q", in, got, want)
		}
	}
}

func TestGdixGenIndexExact(t *testing.T) {
	d := gdixFixtureT(t)
	got, err := gdixGenIndex(d)
	if err != nil {
		t.Fatal(err)
	}
	if got != gdixWantContent {
		t.Fatalf("gen_index mismatch\n got=%q\nwant=%q", got, gdixWantContent)
	}
}

func TestGdixMainWritesFile(t *testing.T) {
	d := gdixFixtureT(t)
	outdir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outdir, 0o755); err != nil {
		t.Fatal(err)
	}
	outp := filepath.Join(outdir, "INDEX.md")
	rc, so, se := gdixRunT(d, "--out", outp)
	if rc != 0 {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
	want := "INDEX 生成: " + outp + "（" + strconv.Itoa(len([]rune(gdixWantContent))) + " 字）\n"
	if so != want {
		t.Fatalf("stdout=%q want %q", so, want)
	}
	b, err := os.ReadFile(outp)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != gdixWantContent+"\n" {
		t.Fatalf("落盘内容不符")
	}
}

func TestGdixMainDefaultOut(t *testing.T) {
	d := gdixFixtureT(t)
	rc, so, _ := gdixRunT(d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "INDEX 生成: " + filepath.Join(d, "INDEX.md") + "（" + strconv.Itoa(len([]rune(gdixWantContent))) + " 字）\n"
	if so != want {
		t.Fatalf("stdout=%q want %q", so, want)
	}
	if _, err := os.Stat(filepath.Join(d, "INDEX.md")); err != nil {
		t.Fatalf("默认产物未落：%v", err)
	}
	// INDEX.md 自身必须被 SKIP（不进文件表）
	b, _ := os.ReadFile(filepath.Join(d, "INDEX.md"))
	if strings.Contains(string(b), "INDEX.md](") {
		t.Fatalf("INDEX.md 自指进表")
	}
}

func TestGdixMainHelp(t *testing.T) {
	rc, so, _ := gdixRunT("--help")
	if rc != 0 || so != gdixHelpText {
		t.Fatalf("rc=%d so=%q", rc, so)
	}
	rc2, so2, _ := gdixRunT("-h")
	if rc2 != 0 || so2 != gdixHelpText {
		t.Fatalf("-h rc=%d so=%q", rc2, so2)
	}
}

func TestGdixMainNoArgsRC2(t *testing.T) {
	rc, _, se := gdixRunT()
	want := gdixUsageLine + "doc_index_gen.py: error: the following arguments are required: dir\n"
	if rc != 2 || se != want {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
}

func TestGdixMainUnrecognizedRC2(t *testing.T) {
	rc, _, se := gdixRunT("--bogus", "XYZ")
	want := gdixUsageLine + "doc_index_gen.py: error: unrecognized arguments: --bogus\n"
	if rc != 2 || se != want {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
}

func TestGdixMainMissingOutValueRC2(t *testing.T) {
	rc, _, se := gdixRunT("dd", "--out")
	want := gdixUsageLine + "doc_index_gen.py: error: argument --out: expected one argument\n"
	if rc != 2 || se != want {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
}

func TestGdixMainDirMissingRC2(t *testing.T) {
	rc, _, se := gdixRunT("/no/such/dir/gdix-xyz")
	if rc != 2 || !strings.Contains(se, "输入目录读不了") {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
}

func TestGdixMainOutDirMissingRC2(t *testing.T) {
	d := gdixFixtureT(t)
	rc, _, se := gdixRunT(d, "--out", "/no/such/dir/gdix-out/INDEX.md")
	if rc != 2 || !strings.Contains(se, "输出写不进") {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
}

func TestGdixOutEqualsForm(t *testing.T) {
	d := gdixFixtureT(t)
	outdir := filepath.Join(t.TempDir(), "o")
	if err := os.MkdirAll(outdir, 0o755); err != nil {
		t.Fatal(err)
	}
	outp := filepath.Join(outdir, "INDEX.md")
	rc, so, _ := gdixRunT("--out="+outp, d)
	if rc != 0 || !strings.Contains(so, outp) {
		t.Fatalf("rc=%d so=%q", rc, so)
	}
}

func TestGdixVersionHistoricNotCur(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "v2.5.8")
	newer := filepath.Join(base, "v2.6.0")
	for _, d := range []string{old, newer} {
		gdixWriteT(t, filepath.Join(d, "00-架构.md"), "甲\n")
		gdixWriteT(t, filepath.Join(d, "01-主控.md"), "乙\n")
		gdixWriteT(t, filepath.Join(d, "02-网关.md"), "丙\n")
	}
	got, err := gdixGenIndex(old)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "> 历史档案") {
		t.Fatalf("旧版应判历史：%q", got)
	}
	got2, _ := gdixGenIndex(newer)
	if !strings.Contains(got2, "> 当前版") {
		t.Fatalf("新版应判当前：%q", got2)
	}
}

func TestGdixManyFilesGrouped(t *testing.T) {
	d := t.TempDir()
	for n := 0; n < 3; n++ {
		gdixWriteT(t, filepath.Join(d, "x"+strconv.Itoa(n)+".md"), "内容"+strconv.Itoa(n)+"\n")
	}
	outp := filepath.Join(t.TempDir(), "i.md")
	rc, so, _ := gdixRunT(d, "--out", outp)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	b, _ := os.ReadFile(outp)
	s := string(b)
	if !strings.Contains(s, "## 其他") || !strings.Contains(s, "| [x0.md](x0.md) | 内容0 |") ||
		!strings.Contains(s, "| [x2.md](x2.md) | 内容2 |") {
		t.Fatalf("分组/行不符:\n%s", s)
	}
	if !strings.Contains(so, outp) {
		t.Fatalf("so=%q", so)
	}
}
