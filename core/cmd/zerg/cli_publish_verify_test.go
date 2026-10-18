// cli_publish_verify_test.go —— `zerg publish verify`（拟名 · 待命令树登记）Go 真身的**定向单测**。
//
// 本件只测**确定性面**（纯函数：缩进 / 尾行 / grep-缩进 / 计数 / 行锚定 / 自指消解）；
// 两器实跑面（真起 bash / python / git）另走**两态对拍**（`publishVerifyRun` 直调），不进常规单测
// —— 免得把「跑两器全史」这种分钟级、依赖仓态的活塞进 `go test` 常态。
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPublishVerify_Indent(t *testing.T) {
	cases := []struct{ in, pad, want string }{
		{"", "  ", ""},
		{"a\nb\n", "  ", "  a\n  b\n"},
		{"a\nb", "  ", "  a\n  b"},
		{"x\n", "      ", "      x\n"},
		{"\n", "  ", "  \n"},
	}
	for _, c := range cases {
		if got := ppvIndent(c.in, c.pad); got != c.want {
			t.Fatalf("ppvIndent(%q,%q)=%q want %q", c.in, c.pad, got, c.want)
		}
	}
}

func TestPublishVerify_Tail(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"", 12, ""},
		{"a\nb\nc\n", 2, "b\nc\n"},
		{"a\nb", 5, "a\nb"},
		{"a\nb\nc", 12, "a\nb\nc"},
	}
	for _, c := range cases {
		if got := ppvTail(c.in, c.n); got != c.want {
			t.Fatalf("ppvTail(%q,%d)=%q want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestPublishVerify_GrepIndent(t *testing.T) {
	body := "aaa\n只有旧器有: x\n契约外差异 1\n契约外差异 2\n"
	got := ppvGrepIndent(body, ppvReOnlyOld.String(), "      ", 3)
	want := "      只有旧器有: x\n      契约外差异 1\n      契约外差异 2\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if g := ppvGrepIndent(body, "zzz-nomatch", "  ", 3); g != "" {
		t.Fatalf("无命中应出空串，实得 %q", g)
	}
	// head 生效（旧壳 `| head -2`）
	if g := ppvGrepIndent("契约外差异\n契约外差异\n契约外差异\n", ppvReEmptySym.String(), "  ", 2); g != "  契约外差异\n  契约外差异\n" {
		t.Fatalf("head 未生效：%q", g)
	}
}

func TestPublishVerify_CountLines(t *testing.T) {
	if ppvCountLines("") != 0 {
		t.Fatal("空串应 0")
	}
	if ppvCountLines("a\nb\n") != 2 {
		t.Fatal("a\\nb\\n 应 2")
	}
	if ppvCountLines("a\nb") != 1 {
		t.Fatal("a\\nb 应 1")
	}
}

func TestPublishVerify_LineMatch(t *testing.T) {
	re := regexp.MustCompile(`^已推送|已推送（非 force）`)
	if !lineMatch(re, "xx\n已推送: main → origin\n") {
		t.Fatal("行首 已推送 应命中")
	}
	if !lineMatch(re, "x已推送（非 force）y\n") {
		t.Fatal("未锚定的 已推送（非 force） 应命中")
	}
	if lineMatch(re, "  已推送: 缩进行首不锚定\n") {
		t.Fatal("缩进行首的裸 已推送 不该命中（^ 只锚第一条分支）")
	}
	if lineMatch(re, "  git push 命令提示（文本非动作）\n") {
		t.Fatal("推送命令提示不该命中")
	}
}

// TestPublishVerify_SelfNoContiguousPrivateName —— 自指消解声明成立吗（✗ 不写连续原串）。
//
// 本件落在公开面（`core/**`）内，而发布链的私有面扫描正盯着私有称谓 ⇒ 真身件里必须**拼串**。
// 这条测试是**回归牙**：若有人把连续原串写回真身件，本用例当场 FAIL（带文件名行号）。
func TestPublishVerify_SelfNoContiguousPrivateName(t *testing.T) {
	b, err := os.ReadFile("family_publish_verify_native.go")
	if err != nil {
		t.Skipf("从 cwd 读不到真身件（非包目录 cwd？）：%v", err)
	}
	src := string(b)
	banned := []string{"老" + "叨", "脑" + "（Hermes）", "脑" + "(Hermes)"}
	for _, lit := range banned {
		if strings.Contains(src, lit) {
			t.Fatalf("真身件里出现**连续**私有称谓原串（应拼串）：%q", lit)
		}
	}
	// 且探针值本身须与旧壳 `-e` 三条**逐字同**（此处用拼串造期望值，防手滑改坏）。
	if len(ppvLeakProbes) != 3 ||
		ppvLeakProbes[0] != "老"+"叨" ||
		ppvLeakProbes[1] != "脑"+"（Hermes）" ||
		ppvLeakProbes[2] != "脑"+"(Hermes)" {
		t.Fatalf("ppvLeakProbes 三条探针与旧壳 -e 值不符：%#v", ppvLeakProbes)
	}
}

// TestPublishVerify_WorkspaceUnderTMPDIR —— 缺口 `GAP-20261018-18` 的**回归牙**（默认档工作区落点）。
//
// 病灶：原实现把对拍工作区写死在系统临时目录（`/tmp/zerg-parity-<戳>`）、且该命令 `danger==nil`
// ⇒ 危险档面与写盘点面**都看不见它会写盘**，跑一次就在盘上留一棵树（实测已积 26 棵）。
//
// 本用例钉三格（静态面负控在 `TestPublishVerify_NoHardcodedTempPath`）：
//
//	① 默认档（不给 `PARITY_BASE`）⇒ 自建工作区，落点在 `TMPDIR` 之内（✗ 不是写死路径）；
//	② 收口**真回收**（走 `removeActiveMaterialized`，与 `Ctrl-C` 信号面同一条）；
//	③ 收到 `PARITY_BASE` ⇒ 用它、且**不**自建（两态对拍要同一只 BASE）。
func TestPublishVerify_WorkspaceUnderTMPDIR(t *testing.T) {
	td := t.TempDir()
	t.Setenv("TMPDIR", td)
	t.Setenv("PARITY_BASE", "")
	base, selfMade, err := ppvWorkspaceBase()
	if err != nil {
		t.Fatalf("默认档建工作区失败：%v", err)
	}
	if !selfMade {
		t.Fatalf("默认档（无 PARITY_BASE）必须自建工作区（收口才会回收）")
	}
	if want := td + string(os.PathSeparator); !strings.HasPrefix(base, want) {
		t.Fatalf("工作区不在 TMPDIR 内：base=%q 期望前缀 %q（写死临时目录的病复发）", base, want)
	}
	if st, err := os.Stat(base); err != nil || !st.IsDir() {
		t.Fatalf("工作区该真建出来：%v", err)
	}
	// 收口：与生产档同一条（`defer removeActiveMaterialized()`）。
	setActiveMatRoot(base)
	removeActiveMaterialized()
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("收口没回收工作区（盘上还留着 %s）：%v", base, err)
	}
}

// TestPublishVerify_ParityBaseKept —— 显式 `PARITY_BASE` ⇒ 点名路径生效、**不**自建。
// 这一格是上一条的**反向**：别把「默认档零残留」做成「把调用方点名的现场也删了」。
func TestPublishVerify_ParityBaseKept(t *testing.T) {
	td := t.TempDir()
	want := filepath.Join(td, "named", "base")
	t.Setenv("PARITY_BASE", want)
	base, selfMade, err := ppvWorkspaceBase()
	if err != nil {
		t.Fatalf("PARITY_BASE 档建工作区失败：%v", err)
	}
	if selfMade {
		t.Fatalf("给了 PARITY_BASE 就不许自建临时工作区（两态对拍要同一只 BASE）")
	}
	if base != want {
		t.Fatalf("工作区应逐字等于 PARITY_BASE：got %q want %q", base, want)
	}
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatalf("PARITY_BASE 目录该建出来：%v", err)
	}
}

// TestPublishVerify_NoHardcodedTempPath —— 静态面负控：真身件里✗ 不许再出现**写死的**临时路径字面。
//
// 牙在哪：把真身件里那行改回写死形（`"/tmp/…"`）⇒ 本用例当场 FAIL 并点名；同时正控确认落点
// 仍是 `os.MkdirTemp("", ppvTempPrefix)`（⇒ 读 `TMPDIR`，不是写死路径）。
func TestPublishVerify_NoHardcodedTempPath(t *testing.T) {
	b, err := os.ReadFile("family_publish_verify_native.go")
	if err != nil {
		t.Skipf("从 cwd 读不到真身件（非包目录 cwd？）：%v", err)
	}
	src := string(b)
	for _, lit := range []string{`"/tmp/`, `"/private/tmp/`} {
		if strings.Contains(src, lit) {
			t.Fatalf("真身件里仍有写死的临时路径字面 %q（GAP-20261018-18：应走 os.MkdirTemp / PARITY_BASE）", lit)
		}
	}
	if !strings.Contains(src, `os.MkdirTemp("", ppvTempPrefix)`) {
		t.Fatalf("真身件里找不到 TMPDIR 工作区落点 `os.MkdirTemp(\"\", ppvTempPrefix)`")
	}
}
