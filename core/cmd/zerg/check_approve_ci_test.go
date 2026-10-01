// check_approve_ci_test.go —— `check_approve_ci.go` 的成对测试件（✗ 禁恒绿）。
//
// 面：自检绿 · help/错误档逐字 · `--json` 拒收 · py 词法原语（normpath/join/strip/splitlines）·
// ★ nil 不得兼表「取不到」与「空集合」（成对两条牙）· ★ 空口径件（判红 rc=1）⟷ 缺口径件（不给结论 rc=2）两态 ·
// ★ A 类止血牙（沙箱 delta=0）· ★ 真状态目录被写 ⇒ 判红。
//
// ★ 测试件里的 python3：**不适用** —— 本测试件零 python 依赖（纯 Go 调用 `checkApproveCIGateMain` + 壳脚本夹具）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func caciTWrite(t *testing.T, p, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// caciTRun 以进程内入口（已落仓件本体）跑一格，返 (rc, stdout, stderr)。
func caciTRun(t *testing.T, argv []string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkApproveCIGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// caciTFixture 造仓外夹具：`bin/zerg` = 失败面壳脚本（rc=2 · stdout 空）；
// fixture/bin 里放 `ssh-keygen` / `security` 壳脚本并前置进 PATH（✗ 不真起器件面 / 不依赖本机工具）。
func caciTFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	caciTWrite(t, filepath.Join(root, "bin", "zerg"), "#!/bin/sh\nexit 2\n", 0o755)
	bin := filepath.Join(root, "fakebin")
	caciTWrite(t, filepath.Join(bin, "ssh-keygen"), "#!/bin/sh\nexit 255\n", 0o755)
	caciTWrite(t, filepath.Join(bin, "security"), "#!/bin/sh\necho \"No smartcards found.\"\nexit 44\n", 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", root)
	t.Setenv("TMPDIR", t.TempDir())
	return root
}

func caciTCountPrefix(t *testing.T, dir, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

func TestCaciSelfTestGreen(t *testing.T) {
	rc, out, errb := caciTRun(t, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检应绿 rc=0，得 %d\nstderr=%s", rc, errb)
	}
	if !strings.HasPrefix(out, "check-approve-ci.py v1.0.0\n") {
		t.Fatalf("首行应为 VERSION，得：%q", out[:caciMin(len(out), 40)])
	}
	if !strings.Contains(out, "   自检：21 格过 · 0 格不过（真目标只读 · 未改任何字节 · 未签任何件）\n") {
		t.Fatalf("自检统计行不符：\n%s", out)
	}
	if !strings.Contains(out, "   ✓ ⑬-4 负控：CI 面出现 `operator.key` ⇒ 判红（红线）\n") {
		t.Fatalf("⑬-4 行不符：\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Fatalf("自检不该有 ✗：\n%s", out)
	}
}

func TestCaciHelpAndArgErrors(t *testing.T) {
	rc, out, _ := caciTRun(t, []string{"-h"})
	if rc != 0 {
		t.Fatalf("-h 应 rc=0，得 %d", rc)
	}
	for _, want := range []string{
		"usage: check-approve-ci.py [-h] [--root ROOT] [--state STATE]\n",
		"optional arguments:\n",
		"  --devdocs DEVDOCS  开发文档面根（默认 = `ZERG_DEVDOCS_ROOT` > 仓同级 `Zerg-\n",
		"                     docs/项目文档/v2.5.11`）\n",
		"  --no-self-test     跳过自检（只有已自证过的调用方才该用）\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help 缺片段 %q：\n%s", want, out)
		}
	}

	rc, out, errb := caciTRun(t, []string{"--bogus"})
	if rc != 2 || out != "" {
		t.Fatalf("未知旗标应 rc=2 且 stdout 空，得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errb, "check-approve-ci.py: error: unrecognized arguments: --bogus\n") {
		t.Fatalf("未知旗标 stderr 不符：%q", errb)
	}

	rc, _, errb = caciTRun(t, []string{"--timeout", "abc"})
	if rc != 2 || !strings.Contains(errb, "argument --timeout: invalid int value: 'abc'\n") {
		t.Fatalf("非法 int stderr 不符 rc=%d：%q", rc, errb)
	}

	rc, _, errb = caciTRun(t, []string{"--root"})
	if rc != 2 || !strings.Contains(errb, "argument --root: expected one argument\n") {
		t.Fatalf("缺值 stderr 不符 rc=%d：%q", rc, errb)
	}

	rc, _, errb = caciTRun(t, []string{"extra"})
	if rc != 2 || !strings.Contains(errb, "unrecognized arguments: extra\n") {
		t.Fatalf("位置参数 stderr 不符 rc=%d：%q", rc, errb)
	}

	// ★ `--` 本身也计入 unrecognized（py argparse 实测：['--','x'] ⇒ "-- x"）。
	rc, _, errb = caciTRun(t, []string{"x", "--", "y"})
	if rc != 2 || !strings.Contains(errb, "unrecognized arguments: x -- y\n") {
		t.Fatalf("`--` stderr 不符 rc=%d：%q", rc, errb)
	}
	rc, _, errb = caciTRun(t, []string{"--"})
	if rc != 2 || !strings.Contains(errb, "unrecognized arguments: --\n") {
		t.Fatalf("裸 `--` stderr 不符 rc=%d：%q", rc, errb)
	}
}

func TestCaciRefuseJSON(t *testing.T) {
	rc, out, errb := caciTRun(t, []string{"--json"})
	if rc != 2 || out != "" {
		t.Fatalf("--json 应 rc=2 且 stdout 空，得 rc=%d out=%q", rc, out)
	}
	for _, want := range []string{
		"⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）\n",
		"   字段表：无（本门只出人读面）\n",
		"⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n",
	} {
		if !strings.Contains(errb, want) {
			t.Fatalf("--json 拒收缺片段 %q：%q", want, errb)
		}
	}
	rc, _, errb = caciTRun(t, []string{"--json=1"})
	if rc != 2 || !strings.Contains(errb, "`--json=1`") {
		t.Fatalf("--json=1 应逐字点名，rc=%d：%q", rc, errb)
	}
	// 先于 argparse 拦截：即便同时给 -h 也走拒收（py 同序）。
	rc, out, _ = caciTRun(t, []string{"-h", "--json"})
	if rc != 2 || out != "" {
		t.Fatalf("--json 应先于 -h 拦截，得 rc=%d out=%q", rc, out)
	}
}

func TestCaciPyPrimitives(t *testing.T) {
	cases := []struct {
		got, want, why string
	}{
		{caciPyNormpath("//a/b"), "//a/b", "normpath 保留恰两个前导斜杠"},
		{caciPyNormpath("///a/b"), "/a/b", "三个以上折成一个"},
		{caciPyNormpath("/"), "/", "根"},
		{caciPyNormpath(""), ".", "空串"},
		{caciPyNormpath("a/../b"), "b", "相对上跳"},
		{caciPyJoin("a", ""), "a/", "空段保留尾斜杠"},
		{caciPyJoin("", "a"), "a", "空首段"},
		{caciPyJoin("a", "/abs"), "/abs", "后段绝对 ⇒ 丢弃前段"},
		{caciPyJoin("a", "..", "b"), "a/../b", "join ✗ 不归一"},
		{caciPyStrip("\x1c a \x1d"), "a", "strip 剥 \\x1c-\\x1d"},
		{caciExpandUser("~"), os.Getenv("HOME"), "expanduser"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("%s：got %q want %q", c.why, c.got, c.want)
		}
	}
	if got := strings.Join(caciPySplitLines("a\r\nb\rc\nd"), "|"); got != "a|b|c|d" {
		t.Fatalf("splitlines：%q", got)
	}
	if got := caciFirstLine("  x  \n y \n"); got != "x  " {
		t.Fatalf("firstLine：%q（py = 整串 strip 后取首行 ⇒ 行内尾空白保留）", got)
	}
	if got := caciFirstLine("  x\n y  "); got != "x" {
		t.Fatalf("firstLine：%q", got)
	}
	if got := caciFirstLine("\n\n"); got != "（空）" {
		t.Fatalf("firstLine 空档：%q", got)
	}
}

// ★ nil 不得兼表「取不到」与「空集合」：两态成对钉死两条牙。
func TestCaciNilNotBoth(t *testing.T) {
	// 空集合（有器件 ⇒ 不判）：必须是非 nil 的空切片 + 明写「不判」。
	p, w := caciJudgeDevicePath(0, false, false, false, true)
	if p == nil {
		t.Fatal("nil slice：空集合被当成取不到")
	}
	if len(p) != 0 {
		t.Fatalf("有器件应零问题，得 %v", p)
	}
	if !strings.Contains(w, "不判") {
		t.Fatalf("有器件应交代「不判」，得 %q", w)
	}
	// 另一面：零器件 + rc=255 且没落件 ⇒ 也是非 nil 空切片。
	p2, _ := caciJudgeDevicePath(255, false, false, false, false)
	if p2 == nil || len(p2) != 0 {
		t.Fatalf("零器件合格档应非 nil 空切片，得 %#v", p2)
	}
	// 判红那一档必须真出问题。
	p3, _ := caciJudgeDevicePath(0, false, false, false, false)
	if len(p3) == 0 {
		t.Fatal("无器件 rc=0 应判红")
	}
}

// ★ 「空口径件（在盘 ⇒ 判红 rc=1）」⟷「口径件不在（⇒ 不给结论 rc=2）」：两态各成一格。
func TestCaciEmptyDocRed_Vs_MissingDocNocall(t *testing.T) {
	root := caciTFixture(t)
	devdocs := filepath.Join(root, "zerg-docs")
	if err := os.MkdirAll(devdocs, 0o755); err != nil {
		t.Fatal(err)
	}
	argv := []string{"--root", root, "--devdocs", devdocs, "--state", filepath.Join(root, "state")}

	// 档一：件不在 ⇒ nocalls ⇒ rc=2（缺件 ≠ 绿）。
	rc, out, _ := caciTRun(t, argv)
	if rc != 2 {
		t.Fatalf("缺口径件应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "C3 **未取数**：口径件不在") {
		t.Fatalf("缺件档正文不符：\n%s", out)
	}
	if strings.Contains(out, "C3 破：") {
		t.Fatalf("缺件档不该出 C3 破（那是「空/缺句」才走的路）：\n%s", out)
	}

	// 档二：件在盘但零字节 ⇒ judge 判红 ⇒ rc=1。
	caciTWrite(t, filepath.Join(devdocs, caciDocName), "", 0o644)
	rc, out, _ = caciTRun(t, argv)
	if rc != 1 {
		t.Fatalf("空口径件应判红 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "C3 破：口径件缺句") {
		t.Fatalf("空件档正文不符：\n%s", out)
	}
}

// ★ A 类止血牙：沙箱（mkdtemp）跑前跑后同前缀计数必须 delta=0（删掉 defer ⇒ 本用例红）。
func TestCaciSandDeltaZero(t *testing.T) {
	root := caciTFixture(t)
	tmp := os.Getenv("TMPDIR")
	devdocs := filepath.Join(root, "zerg-docs")
	caciTWrite(t, filepath.Join(devdocs, caciDocName), strings.Join(caciMust, ""), 0o644)

	before := caciTCountPrefix(t, tmp, "approve-ci-")
	rc, out, _ := caciTRun(t, []string{"--root", root, "--devdocs", devdocs, "--state", filepath.Join(root, "state")})
	after := caciTCountPrefix(t, tmp, "approve-ci-")
	if after != before {
		t.Fatalf("A 类泄漏：沙箱未回收 before=%d after=%d\n%s", before, after, out)
	}
	if rc != 0 {
		t.Fatalf("全绿档应 rc=0，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("全绿正文不符：\n%s", out)
	}
}

// ★ 真状态目录被写 ⇒ 判红（夹具的 zerg 桩会往 $ZERG_STATE_DIR/approvals 落一枚件）。
func TestCaciStateWriteRed(t *testing.T) {
	root := caciTFixture(t)
	tmp := os.Getenv("TMPDIR")
	devdocs := filepath.Join(root, "zerg-docs")
	caciTWrite(t, filepath.Join(devdocs, caciDocName), strings.Join(caciMust, ""), 0o644)
	state := filepath.Join(root, "state")
	caciTWrite(t, filepath.Join(root, "bin", "zerg"),
		"#!/bin/sh\nmkdir -p \"$ZERG_STATE_DIR/approvals\"\n: > \"$ZERG_STATE_DIR/approvals/leak\"\nexit 2\n", 0o755)
	before := caciTCountPrefix(t, tmp, "approve-ci-")
	rc, out, _ := caciTRun(t, []string{"--root", root, "--devdocs", devdocs, "--state", state})
	after := caciTCountPrefix(t, tmp, "approve-ci-")
	if after != before {
		t.Fatalf("沙箱未回收 before=%d after=%d", before, after)
	}
	if rc != 1 {
		t.Fatalf("真状态目录被写应判红 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "C1 破：**真状态目录被写了**") {
		t.Fatalf("被写档正文不符：\n%s", out)
	}
}

func TestCaciMissingBinZerg(t *testing.T) {
	root := caciTFixture(t)
	if err := os.Remove(filepath.Join(root, "bin", "zerg")); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := caciTRun(t, []string{"--root", root, "--state", filepath.Join(root, "state")})
	if rc != 2 {
		t.Fatalf("缺 bin/zerg 应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "⛔ 未取数：`bin/zerg` 不在") {
		t.Fatalf("缺制品正文不符：\n%s", out)
	}
}

func caciMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
