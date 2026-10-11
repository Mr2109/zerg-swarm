// check_mkdir_temp_cleanup_test.go —— 与 `check_mkdir_temp_cleanup.go` 成对（M2 · 搬运粒 21）。
//
// 定位：**成对负控**（✗ 禁恒绿）—— 每一格都钉住 py 的一条可观测行为，实现回退即红（哑化 ⇒ FAIL 带行号）。
// 用例前缀 `TestCmk`（现读 `code find 'func TestCmk'` 零命中 ⇒ 不与在仓测试件撞名）。
//
// ★ 自指消解（本件同样落在本门的扫描面内）：夹具里的「站点/回收」原串一律用包内 `cmkKTok`/`cmkRTok`
// 拼装（这两个常量在正文里是拆开的），✗ 不在本件里写连续可被本门正则命中的原串 ⇒ 本件不凭空造站点。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmkRunMain —— 进程内跑入口，收 rc/stdout/stderr 三面。
func cmkRunMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkMkdirTempCleanupGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// cmkWrite —— 造夹具件（自动建父目录）。
func cmkWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cmkFxBounds —— 「MkdirTemp 所在的 f 自己无 RemoveAll · 同件别函数 g（在 f **之后**）有 RemoveAll」⇒ 必须判 T2。
// g 放 f 之后是有意的：若 func_bounds 的收窗过宽（越过 f 的 `}`），会把 g 的 RemoveAll 也吞进来 ⇒ 误判 T1（牙所在）。
var cmkFxBounds = "package x\n\nfunc f() {\n    d, _ := " + cmkKTok + "\"\", \"z\")\n    _ = d\n}\n\nfunc g() {\n    _ = " + cmkRTok + "\"x\")\n}\n"

const cmkTierSummary = "站点 4 = T1 同函数干净 1 + T2 同件有 RemoveAll（helper/调用方清 · 只报告）1 + T3 同件无 RemoveAll 2\n"

// TestCmkHelpAndArgparse —— argparse 面：帮助 / 未知旗标 / 缺值 / 显式值拒收 / 唯一前缀缩写。
func TestCmkHelpAndArgparse(t *testing.T) {
	rc, out, errb := cmkRunMain(t, "-h")
	if rc != 0 || errb != "" || out != cmkHelpText {
		t.Fatalf("help rc=%d out=%q err=%q", rc, out, errb)
	}

	rc, out, errb = cmkRunMain(t, "--bogus")
	if rc != 2 || out != "" || errb != cmkUsage+"check-mkdir-temp-cleanup.py: error: unrecognized arguments: --bogus\n" {
		t.Fatalf("bogus rc=%d out=%q err=%q", rc, out, errb)
	}

	rc, _, errb = cmkRunMain(t, "--root")
	if rc != 2 || !strings.Contains(errb, "check-mkdir-temp-cleanup.py: error: argument --root: expected one argument\n") {
		t.Fatalf("noval rc=%d err=%q", rc, errb)
	}

	rc, _, errb = cmkRunMain(t, "--self-test=true")
	if rc != 2 || !strings.Contains(errb, "argument --self-test: ignored explicit argument 'true'\n") {
		t.Fatalf("selftest=eq rc=%d err=%q", rc, errb)
	}

	// 唯一前缀 `--ro` ⇒ 命中 `--root`（夹具空跑应 rc=2 空转，证前缀真解析成 root）。
	dir := t.TempDir()
	rc, _, errb = cmkRunMain(t, "--ro", dir, "--dirs", "nope")
	if rc != 2 || !strings.Contains(errb, "不给结论（rc=2）") {
		t.Fatalf("prefix --ro rc=%d err=%q", rc, errb)
	}
}

// TestCmkScanTiersAndOrder —— 判据体：T1/T2/T3 三档计数 + `/vendor` `/testdata` 跳过 + 非 .go 跳过。
func TestCmkScanTiersAndOrder(t *testing.T) {
	dir := t.TempDir()
	cmkWrite(t, dir, "core/a.go", cmkT1Go)                  // T1
	cmkWrite(t, dir, "core/b.go", cmkT3Go)                  // T3
	cmkWrite(t, dir, "core/sub/c.go", cmkT2Go)              // T2（同件别函数有 RemoveAll）
	cmkWrite(t, dir, "agent/d.go", cmkT3Go)                 // T3
	cmkWrite(t, dir, "core/vendor/v.go", cmkT3Go)           // 跳过（/vendor）
	cmkWrite(t, dir, "core/testdata/t.go", cmkT3Go)         // 跳过（/testdata）
	cmkWrite(t, dir, "core/note.txt", "os.Mkdir"+"Temp(\n") // 非 .go ⇒ 跳过

	rc, out, errb := cmkRunMain(t, "--root", dir)
	if rc != 1 {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errb)
	}
	if out != cmkTierSummary {
		t.Fatalf("汇总行: %q", out)
	}
	if !strings.Contains(errb, "  core/b.go:4\n") || !strings.Contains(errb, "  agent/d.go:4\n") {
		t.Fatalf("T3 点名: %q", errb)
	}
	if strings.Contains(errb, "vendor/") || strings.Contains(errb, "testdata/") || strings.Contains(errb, "note.txt") {
		t.Fatalf("跳过面失灵: %q", errb)
	}
}

// TestCmkFuncBoundsWindow —— func_bounds **从函数定义行起算**：同件别函数有 RemoveAll 不得抬成 T1（牙：回退即红）。
func TestCmkFuncBoundsWindow(t *testing.T) {
	dir := t.TempDir()
	cmkWrite(t, dir, "core/x.go", cmkFxBounds)
	rc, out, _ := cmkRunMain(t, "--root", dir, "--dirs", "core")
	if rc != 0 {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	want := "站点 1 = T1 同函数干净 0 + T2 同件有 RemoveAll（helper/调用方清 · 只报告）1 + T3 同件无 RemoveAll 0\n" +
		"✓ T3 为 0（或已在白名单）\n"
	if out != want {
		t.Fatalf("T1/T2 分档错（func_bounds 窗口过宽）: %q", out)
	}
}

// TestCmkSymlinkNoDescend —— os.walk(followlinks=False)：**指向目录的符号链接不下钻**（牙：若下钻 ⇒ sites 3）。
func TestCmkSymlinkNoDescend(t *testing.T) {
	dir := t.TempDir()
	cmkWrite(t, dir, "real/r.go", cmkT3Go)
	cmkWrite(t, dir, "aa.go", cmkT3Go)
	if err := os.Symlink("real", filepath.Join(dir, "linkdir")); err != nil {
		t.Skipf("造符号链接失败（环境不支持）：%v", err)
	}
	rc, out, errb := cmkRunMain(t, "--root", dir, "--dirs", ".")
	if rc != 1 {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.HasPrefix(out, "站点 2 = ") {
		t.Fatalf("符号链接被下钻（sites 应 2）: %q", out)
	}
	if strings.Contains(errb, "linkdir/") {
		t.Fatalf("符号链接目录内容被扫（应跳过）: %q", errb)
	}
	if !strings.Contains(errb, "  aa.go:4\n") || !strings.Contains(errb, "  real/r.go:4\n") {
		t.Fatalf("真目录内容应被扫: %q", errb)
	}
}

// TestCmkWhitelistStopsT3 —— 白名单压 T3：件名进白名单 ⇒ rc=0 且打「T3 为 0」。
func TestCmkWhitelistStopsT3(t *testing.T) {
	dir := t.TempDir()
	cmkWrite(t, dir, "core/b.go", cmkT3Go)
	wl := filepath.Join(dir, "wl.txt")
	if err := os.WriteFile(wl, []byte("# 注释\n\ncore/b.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := cmkRunMain(t, "--root", dir, "--dirs", "core", "--whitelist", wl)
	if rc != 0 || errb != "" {
		t.Fatalf("rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "T3 同件无 RemoveAll 1\n") || !strings.HasSuffix(out, "✓ T3 为 0（或已在白名单）\n") {
		t.Fatalf("白名单出口: %q", out)
	}
}

// TestCmkEmptyRunRc2 —— 0 命中 = 空转 ⇒ 不给结论 rc=2（✗ 不静默绿）。
func TestCmkEmptyRunRc2(t *testing.T) {
	dir := t.TempDir()
	cmkWrite(t, dir, "core/x.txt", "no go here\n")
	rc, out, errb := cmkRunMain(t, "--root", dir, "--dirs", "core")
	if rc != 2 || out != "" || !strings.Contains(errb, "空转 = 假覆盖") {
		t.Fatalf("空转 rc=%d out=%q err=%q", rc, out, errb)
	}
}

// TestCmkSelfTest —— 自检入口：5 组 9 条断言全过（TMPDIR 收进私有域，✗ 不数全局临时目录）。
func TestCmkSelfTest(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	rc, out, errb := cmkRunMain(t, "--self-test")
	if rc != 0 || errb != "" {
		t.Fatalf("self-test rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "── 自检结论：断言 9 条 · 失败 0 条 ──\n") {
		t.Fatalf("自检结论: %q", out)
	}
}

// TestCmkPyJoinSemantics —— 自写 cmkPyJoin 的三类语义差各钉一格（✗ 不用 filepath.Join）。
func TestCmkPyJoinSemantics(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "..", "b"}, "a/../b"}, // ① `..` 不归一
		{[]string{"repo", "/abs"}, "/abs"},   // ② 后段绝对 ⇒ 丢弃前段
		{[]string{"a", "", "b"}, "a/b"},      // ③ 中间空段
		{[]string{"a", ""}, "a/"},            // ③ 尾空段留分隔符
		{[]string{"", "a"}, "a"},             // ③ 首空段
		{[]string{"/root", "core", "b.go"}, "/root/core/b.go"},
	}
	for _, c := range cases {
		if got := cmkPyJoin(c.parts...); got != c.want {
			t.Errorf("cmkPyJoin(%q)=%q want %q", c.parts, got, c.want)
		}
	}
}

// TestCmkNormpathAndSplitLines —— normpath 保留恰两个前导斜杠 + splitlines 的 py 行界。
func TestCmkNormpathAndSplitLines(t *testing.T) {
	if got := cmkPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("normpath //a/b/../c = %q want //a/c", got)
	}
	if got := cmkPyNormpath("/a/./b/"); got != "/a/b" {
		t.Errorf("normpath /a/./b/ = %q want /a/b", got)
	}
	got := cmkPySplitLines("a\r\nb\rc\nd")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("splitlines=%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitlines[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if l := cmkPySplitLines("x\n"); len(l) != 1 || l[0] != "x" {
		t.Fatalf("尾空行应丢弃: %q", l)
	}
	if l := cmkPySplitLines(""); len(l) != 0 {
		t.Fatalf("空串应 0 行: %q", l)
	}
}

// TestCmkReadFailTeeth —— B 类读失败不再静默的回归牙：扫面里一件 .go 锁成 chmod 000 ⇒
// 登记面（cmkReadFails）必响：rc=2 + stderr BLOCKED 头 + 逐件点名。删回静默（原 py 静默跳过）本用例必红。
func TestCmkReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视权限位（chmod 000 仍可读）⇒ 跳过")
	}
	dir := t.TempDir()
	cmkWrite(t, dir, "core/ok.go", cmkT3Go) // 可读 ⇒ sites>0（否则落「空转」档，测不到读失败面）
	cmkWrite(t, dir, "core/locked.go", cmkT3Go)
	locked := filepath.Join(dir, "core", "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o644) }()
	rc, _, errb := cmkRunMain(t, "--root", dir, "--dirs", "core")
	if rc != 2 || !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "core/locked.go") {
		t.Fatalf("读不到件应 rc=2 + BLOCKED 点名 core/locked.go，得 rc=%d err=%q", rc, errb)
	}
}
