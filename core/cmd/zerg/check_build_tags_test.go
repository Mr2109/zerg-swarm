// check_build_tags_test.go —— 门禁件 `check_build_tags.go`（py `check-build-tags.py` 的 Go 真搬）的定向单测。
//
// 目标：① 钉住 CPython 原语同构（join/normpath/strip/splitlines）② 钉住判据面纯函数
// （implicit_platforms / parse_constraints / tag_arg_candidates / is_reserved）
// ③ **成对负控**（有牙）：明写「若换回 Go 标准库等价物则必红」的对照断言，防日后被哑化。
//
// 不写仓内文件（夹具一律落 t.TempDir()）；不联网。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── ① CPython 原语同构 ──────────────────────────────────────────────

func TestCbtPyJoinThreeSemantics(t *testing.T) {
	cases := []struct {
		a    string
		rest []string
		want string
	}{
		{"a", []string{"../b"}, "a/../b"},  // ① py 原样留 `..`（✗ 不归一）
		{"repo", []string{"/abs"}, "/abs"}, // ② 后段绝对路径 ⇒ 丢弃前面所有段
		{"a", []string{"", "b"}, "a/b"},    // ③ 空段
		{"a", []string{""}, "a/"},          // ③ 空段（尾斜杠保留）
		{"", []string{"a"}, "a"},           // ③ 空首段
		{"a/", []string{"b"}, "a/b"},
	}
	for _, c := range cases {
		if got := cbtPyJoin(c.a, c.rest...); got != c.want {
			t.Errorf("cbtPyJoin(%q,%q)=%q want %q", c.a, c.rest, got, c.want)
		}
	}
}

// 负控（有牙）：本件必须**不是** filepath.Join —— 三态里至少两态与 Go 标准库分叉。
func TestCbtPyJoinIsNotFilepathJoin(t *testing.T) {
	if cbtPyJoin("a", "../b") == filepath.Join("a", "../b") {
		t.Fatal("哑化：cbtPyJoin 的 `..` 面与 filepath.Join 同值 ⇒ 该断言已失去牙口")
	}
	if cbtPyJoin("repo", "/abs") == filepath.Join("repo", "/abs") {
		t.Fatal("哑化：绝对后段面与 filepath.Join 同值 ⇒ 该断言已失去牙口")
	}
	if got := cbtPyJoin("repo", "/abs"); got != "/abs" {
		t.Fatalf("绝对后段应丢弃前段：got %q", got)
	}
	if got := cbtPyJoin("a", "../b"); got != "a/../b" {
		t.Fatalf("`..` 应原样留串：got %q", got)
	}
}

func TestCbtPyNormpathKeepsDoubleSlash(t *testing.T) {
	if got := cbtPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("双前导斜杠须保留：got %q want %q", got, "//a/c")
	}
	if got := cbtPyNormpath("///a/b"); got != "/a/b" {
		t.Errorf("三斜杠折成一：got %q", got)
	}
	if got := cbtPyNormpath("a/./b/"); got != "a/b" {
		t.Errorf("got %q", got)
	}
	// 负控：filepath.Clean 会把 //a/c 折成 /a/c ⇒ 二者必不同
	if cbtPyNormpath("//a/b/../c") == filepath.Clean("//a/b/../c") {
		t.Fatal("哑化：normpath 双斜杠面与 filepath.Clean 同值 ⇒ 失去牙口")
	}
}

func TestCbtPyStripFoldsPyWhitespace(t *testing.T) {
	in := "\x1c x\x1d"
	if got := cbtPyStrip(in); got != "x" {
		t.Errorf("py strip 须剥 \\x1c/\\x1d：got %q want %q", got, "x")
	}
	// 负控：strings.TrimSpace **不**剥 \x1c/\x1d
	if cbtPyStrip(in) == strings.TrimSpace(in) {
		t.Fatal("哑化：cbtPyStrip 与 strings.TrimSpace 同值 ⇒ 失去牙口")
	}
}

func TestCbtPySplitLines(t *testing.T) {
	got := cbtPySplitLines("a\r\nb\vc\nd")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if n := len(cbtPySplitLines("a\n\n")); n != 2 {
		t.Fatalf("尾随边界不多产空段：n=%d", n)
	}
}

// ── ② 判据面纯函数 ─────────────────────────────────────────────────

func TestCbtImplicitPlatforms(t *testing.T) {
	goos := map[string]bool{"linux": true, "darwin": true}
	goarch := map[string]bool{"amd64": true, "arm64": true}
	cases := []struct {
		fn   string
		want []string
	}{
		{"zz_linux.go", []string{"linux"}},
		{"zz_amd64.go", []string{"amd64"}},
		{"zz_linux_amd64.go", []string{"linux", "amd64"}},
		{"zz_linux_test.go", []string{"linux"}},
		{"zz_bare.go", nil},
	}
	for _, c := range cases {
		got := cbtImplicitPlatforms(c.fn, goos, goarch)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v want %v", c.fn, got, c.want)
			continue
		}
		for _, w := range c.want {
			if !got[w] {
				t.Errorf("%s: missing %s in %v", c.fn, w, got)
			}
		}
	}
}

func TestCbtParseConstraintsBothForms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	// 约束形态由夹具文件产生（本 Go 源码内不落该形态的前导行）
	body := "// +build !windows,foo\n\npackage a\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	pos, neg, ok := cbtParseConstraints(p)
	if !ok {
		t.Fatal("readable file must parse")
	}
	if !neg["windows"] || !pos["foo"] {
		t.Fatalf("legacy 解析错：pos=%v neg=%v", pos, neg)
	}
	// 负控（有牙）：读不了的件必须报不可判（ok=false），✗ 不许静默当空集
	if _, _, ok := cbtParseConstraints(filepath.Join(dir, "missing.go")); ok {
		t.Fatal("缺件应 ok=false")
	}
}

func TestCbtTagArgCandidates(t *testing.T) {
	// 只在「命令行形态」行上取实参
	got := cbtTagArgCandidates("go build -tags=linux ./...")
	if len(got) != 1 || got[0][0] != "linux" {
		t.Fatalf("got %v", got)
	}
	if got := cbtTagArgCandidates("这是散文，讲 -tags=something 的用法"); len(got) != 0 {
		t.Fatalf("散文行不许取实参：got %v", got)
	}
	// `=` 与空格两种形态 + 逗号列表
	if got := cbtTagArgCandidates("go vet --build-tags debug,x ./..."); len(got) != 2 {
		t.Fatalf("逗号列表应拆两名：got %v", got)
	}
}

func TestCbtIsReservedShapeAndSet(t *testing.T) {
	reserved := cbtReservedNames(map[string]bool{"linux": true}, map[string]bool{"amd64": true})
	if !cbtIsReserved("linux", reserved) || !cbtIsReserved("amd64", reserved) {
		t.Fatal("平台名须判保留")
	}
	if !cbtIsReserved("go1.21", reserved) {
		t.Fatal("go1.N 形态须按形态判保留")
	}
	if cbtIsReserved("go1.21x", reserved) {
		t.Fatal("go1.21x 不属 go1.N 形态")
	}
	if cbtIsReserved("debug", reserved) {
		t.Fatal("debug 不应判保留（否则 a2 恒红）")
	}
}

func TestCbtTailDropsBlankAndKeepsLastN(t *testing.T) {
	got := cbtTail("a\n\nb\nc\n", 2)
	if got != "b\nc" {
		t.Fatalf("got %q", got)
	}
}

// ── ③ 入口面（rc / 输出）＋ 负控 ────────────────────────────────────

func runGate(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkBuildTagsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCbtGateListFace(t *testing.T) {
	rc, out, _ := runGate("--list")
	if rc != 0 {
		t.Fatalf("--list rc=%d want 0", rc)
	}
	if !strings.Contains(out, "门禁契约") || !strings.Contains(out, "允许的自定义 tag : debug") {
		t.Fatalf("--list 输出缺契约正文：%q", out)
	}
}

// 负控（有牙）：--json 在本门**无机器读面** ⇒ 必须 rc=2 且 stderr 点名（✗ 不许沉默）。
func TestCbtGateJsonRefused(t *testing.T) {
	rc, out, errs := runGate("--json")
	if rc != 2 {
		t.Fatalf("--json rc=%d want 2", rc)
	}
	if !strings.Contains(errs, "本门无机器读面") || !strings.Contains(errs, "字段表：无") {
		t.Fatalf("--json 拒收文案缺失：stderr=%q", errs)
	}
	if strings.Contains(out, "门禁契约") {
		t.Fatal("拒收档不许照走主流程")
	}
}

func TestCbtGateUnknownFlagAndHelp(t *testing.T) {
	if rc, _, errs := runGate("--nope"); rc != 2 || !strings.Contains(errs, "未知参数") {
		t.Fatalf("未知参数应 rc=2 + 点名：rc=%d errs=%q", rc, errs)
	}
	if rc, out, _ := runGate("-h"); rc != 2 || !strings.Contains(out, "退出码：0 全绿") {
		t.Fatalf("-h 应打用法后 rc=2：rc=%d", rc)
	}
}

func TestCbtGateSelfNoSelfConflict(t *testing.T) {
	rc, _, errs := runGate("--self-test", "--no-self-test")
	if rc != 2 || !strings.Contains(errs, "不能同用") {
		t.Fatalf("rc=%d errs=%q", rc, errs)
	}
}

func TestCbtGateNoSuchRoot(t *testing.T) {
	rc, _, errs := runGate("--root", filepath.Join(t.TempDir(), "nope"), "--no-self-test")
	if rc != 2 || !strings.Contains(errs, "仓根不存在") {
		t.Fatalf("rc=%d errs=%q", rc, errs)
	}
}

// 负控（有牙）：cbtRun 必须合并 stderr 进 stdout 并取**真实退出码**（✗ 不许 rc 恒 0）。
func TestCbtRunMergesAndReturnsRealRC(t *testing.T) {
	rc, ok, out := cbtRun([]string{"/bin/sh", "-c", "echo out; echo err 1>&2; exit 3"}, "", os.Environ(), nil, cbtBuildTimeout)
	if !ok || rc != 3 {
		t.Fatalf("真实退出码应为 3：ok=%v rc=%d", ok, rc)
	}
	if !strings.Contains(out, "out") || !strings.Contains(out, "err") {
		t.Fatalf("stderr 应并入 stdout：%q", out)
	}
	if _, ok, _ := cbtRun([]string{"/no/such/binary-xyz"}, "", os.Environ(), nil, cbtBuildTimeout); ok {
		t.Fatal("起不来必须 ok=false（判不了），✗ 不许当 rc=0 绿")
	}
}
