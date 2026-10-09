// cli_publish_verify_test.go —— `zerg publish verify`（拟名 · 待命令树登记）Go 真身的**定向单测**。
//
// 本件只测**确定性面**（纯函数：缩进 / 尾行 / grep-缩进 / 计数 / 行锚定 / 自指消解）；
// 两器实跑面（真起 bash / python / git）另走**两态对拍**（`publishVerifyRun` 直调），不进常规单测
// —— 免得把「跑两器全史」这种分钟级、依赖仓态的活塞进 `go test` 常态。
package main

import (
	"os"
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
