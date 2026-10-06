package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本测试件是 `gen_cli_reference.go`（gen-cli-reference.py 的 Go 真搬）的**成对负控**。
// 每条用例都钉住一处「把实现哑化 / 换成标准库近似就会红」的语义点（py 与 Go 的判准差异）。
// 哑化验证：把 `gcrPyStrip` 换成 `strings.TrimSpace` ⇒ `TestGcrPyStripUnicodeSpace`
// 在下方对应 `t.Fatalf` 行（本文件行号）报错 —— 即「哑化 ⇒ FAIL 带行号」。

func TestGcrPyJoinSemantics(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "", "b"}, "a/b"},    // 空段忽略
		{[]string{"a", ""}, "a/"},          // 尾空段保留尾斜杠
		{[]string{"", "a"}, "a"},           // 前空段
		{[]string{"repo", "/abs"}, "/abs"}, // 后段绝对 ⇒ 丢弃前段（filepath.Join 会拼成 repo/abs）
		{[]string{"a", "../b"}, "a/../b"},  // `..` 原样留串（filepath.Join 会归一成 b）
		{[]string{"root", "core/cmd/zerg"}, "root/core/cmd/zerg"},
	}
	for _, c := range cases {
		if got := gcrPyJoin(c.parts[0], c.parts[1:]...); got != c.want {
			t.Fatalf("gcrPyJoin(%q) = %q, want %q", c.parts, got, c.want)
		}
	}
}

func TestGcrPyStripUnicodeSpace(t *testing.T) {
	// py str.strip() 剥 \x1c-\x1f 与 U+00A0；strings.TrimSpace 不剥 \x1c-\x1f ⇒ 哑化即红。
	if got := gcrPyStrip(" \x1c\x1d\x1e\x1f x \u00a0 "); got != "x" {
		t.Fatalf("gcrPyStrip unicode space = %q, want %q", got, "x")
	}
	if got := gcrPyStrip("\u3000y\u3000"); got != "y" {
		t.Fatalf("gcrPyStrip ideographic space = %q, want %q", got, "y")
	}
}

func TestGcrPySplitlines(t *testing.T) {
	got := gcrPySplitlines("a\r\nb\vc\fd\x1ce\n")
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("gcrPySplitlines len = %d (%q), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gcrPySplitlines[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := gcrPySplitlines("a\n\n"); len(got) != 2 || got[1] != "" {
		t.Fatalf("gcrPySplitlines trailing blank = %q, want [a ]", got)
	}
}

func TestGcrPyDirnameAndNormpath(t *testing.T) {
	if got := gcrPyDirname("/a/b/"); got != "/a/b" {
		t.Fatalf("gcrPyDirname trailing slash = %q, want /a/b", got)
	}
	if got := gcrPyDirname("/"); got != "/" {
		t.Fatalf("gcrPyDirname root = %q, want /", got)
	}
	if got := gcrPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Fatalf("gcrPyNormpath double leading slash = %q, want //a/c", got)
	}
	if got := gcrPyNormpath("///a/b"); got != "/a/b" {
		t.Fatalf("gcrPyNormpath triple slash = %q, want /a/b", got)
	}
}

// gcrStripCitations 循环到不动点 ⇒ 嵌套 `（见（§x））` 只剥内层 `（§x）`（外层无 § 不入面）。
func TestGcrStripCitations(t *testing.T) {
	in := "命令说明（§5.6②）续（见（§x））尾"
	if got := gcrStripCitations(in); got != "命令说明续（见）尾" {
		t.Fatalf("gcrStripCitations = %q, want %q", got, "命令说明续（见）尾")
	}
}

func TestGcrIdentFields(t *testing.T) {
	f, ok := gcrIdentFields("zerg 2.5.13 41357be4 2026-10-06T18:10:30Z")
	if !ok || f.component != "zerg" || f.commit != "41357be4" || f.buildTime != "2026-10-06T18:10:30Z" {
		t.Fatalf("gcrIdentFields ok=%v f=%+v", ok, f)
	}
	if _, ok := gcrIdentFields("zerg 2.5.13"); ok {
		t.Fatalf("gcrIdentFields 三段应判形状不认")
	}
}

func TestGcrMapUsageLongestFirst(t *testing.T) {
	paths := []string{"dev", "dev proposal new"}
	lines := []string{"zerg dev proposal new --x", "zerg dev --y"}
	m := gcrMapUsage(paths, lines)
	if m["dev proposal new"] != "zerg dev proposal new --x" {
		t.Fatalf("gcrMapUsage longest not matched: %+v", m)
	}
	if m["dev"] != "zerg dev --y" {
		t.Fatalf("gcrMapUsage dev = %q", m["dev"])
	}
}

func TestGcrAlignRowAnchor(t *testing.T) {
	header := []string{"命令", "说明", "茧壁层级"}
	// 说明列里的 `|` 被切开 ⇒ 靠锚列（命令/茧壁层级）拼回
	got, err := gcrAlignRow([]string{"`zerg x`", "a", "b", "`host`"}, header)
	if err != nil {
		t.Fatalf("gcrAlignRow err=%v", err)
	}
	if got[1] != "a | b" {
		t.Fatalf("gcrAlignRow free col = %q, want %q", got[1], "a | b")
	}
}

func TestGcrRegionSpanNotes(t *testing.T) {
	page := "x\n" + gcrMarkBegin + " x -->\n\n> note1\n> note2\nbody\n" + gcrMarkEnd + " y -->\nz"
	bi, ei, ok := gcrRegionSpan(page)
	if !ok || bi >= ei {
		t.Fatalf("gcrRegionSpan ok=%v bi=%d ei=%d", ok, bi, ei)
	}
	body := gcrTestJoin(gcrPySplitlines(page)[bi+1 : ei])
	notes := gcrRegionNotes(body)
	if len(notes) != 2 || notes[0] != "> note1" {
		t.Fatalf("gcrRegionNotes = %q", notes)
	}
}

func gcrTestJoin(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += "\n"
		}
		out += x
	}
	return out
}

func TestGcrPadRight(t *testing.T) {
	got := gcrPadRight("参考", 6)
	if len([]rune(got)) != 6 {
		t.Fatalf("gcrPadRight rune width = %d, want 6 (%q)", len([]rune(got)), got)
	}
}

func TestGcrCodeSpansIgnoresComments(t *testing.T) {
	a := gcrCodeSpans("`keep` <!-- `drop` -->")
	b := gcrCodeSpans("`keep`")
	if len(a) != 1 || !a["keep"] || len(b) != 1 {
		t.Fatalf("gcrCodeSpans comment strip: %v vs %v", a, b)
	}
}

// ── argparse 面（唯一前缀缩写 + 互斥必填 + 擦除面）───────────────────────

func TestGcrParseArgsPrefixAbbrev(t *testing.T) {
	cases := []struct {
		argv []string
		rc   int
		emit bool
		chk  bool
	}{
		{[]string{"--emi"}, 0, true, false},
		{[]string{"--ch"}, 0, false, true},
		{[]string{"--e"}, 0, true, false},
		{[]string{"--emit"}, 0, true, false},
		{[]string{"--target", "x", "--ch"}, 0, false, true},
	}
	for _, c := range cases {
		var ob, eb bytes.Buffer
		o, rc, done := gcrParseArgs(&ob, &eb, c.argv)
		if done || rc != c.rc || o.emit != c.emit || o.check != c.chk {
			t.Fatalf("gcrParseArgs(%q) rc=%d done=%v emit=%v check=%v（成功档应 done=false）", c.argv, rc, done, o.emit, o.check)
		}
	}
}

func TestGcrParseArgsErrors(t *testing.T) {
	cases := []struct {
		argv    []string
		wantMsg string
	}{
		{[]string{}, "one of the arguments --emit --check is required"},
		{[]string{"--bogus"}, "one of the arguments --emit --check is required"},
		{[]string{"--emit", "--check"}, "argument --check: not allowed with argument --emit"},
		{[]string{"--check", "--emit"}, "argument --emit: not allowed with argument --check"},
		{[]string{"--target"}, "argument --target: expected one argument"},
		{[]string{"--emit", "--bogus"}, "unrecognized arguments: --bogus"},
	}
	for _, c := range cases {
		var ob, eb bytes.Buffer
		_, rc, done := gcrParseArgs(&ob, &eb, c.argv)
		if !done || rc != gcrRCblocked {
			t.Fatalf("gcrParseArgs(%q) done=%v rc=%d want rc=2", c.argv, done, rc)
		}
		if !bytes.Contains(eb.Bytes(), []byte(c.wantMsg)) {
			t.Fatalf("gcrParseArgs(%q) stderr=%q 不含 %q", c.argv, eb.String(), c.wantMsg)
		}
		if !bytes.Contains(eb.Bytes(), []byte(gcrUsageLine)) {
			t.Fatalf("gcrParseArgs(%q) 缺 usage 行", c.argv)
		}
	}
}

func TestGcrHelpPrintsNoErrors(t *testing.T) {
	var ob, eb bytes.Buffer
	_, rc, done := gcrParseArgs(&ob, &eb, []string{"--help"})
	if !done || rc != gcrRCok || eb.Len() != 0 {
		t.Fatalf("--help rc=%d done=%v stderr=%q", rc, done, eb.String())
	}
	if !bytes.Contains(ob.Bytes(), []byte("usage: gen-cli-reference.py")) {
		t.Fatalf("--help stdout 缺 usage 行")
	}
}

// ── 端到端：缺制品 ⇒ fail-closed（rc=2 口径）──────────────────────────
// ★ 不做 os.Chdir（go test 的 cwd 在真仓内 ⇒ 上溯会命中真仓、测不到缺件档）；
//
//	改直接测取数函数 `gcrLiveData(root)`（它收显式根）—— 同一 fail-closed 面。
func TestGcrLiveDataFailClosedNoBin(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "core", "cmd", "zerg", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := gcrLiveData(dir)
	if err == "" || !strings.Contains(err, "找不到 bin/zerg") {
		t.Fatalf("缺 bin/zerg 应 fail-closed（不给结论），实得 err=%q", err)
	}
}
