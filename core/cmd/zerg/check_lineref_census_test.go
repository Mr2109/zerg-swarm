// check_lineref_census_test.go —— `checkLinerefCensusGateMain` 的成对负控
// （正控绿 / 行号指向不是那个东西 ⇒ 真红点名 件:行:目标 / 扫描根取不到 ⇒ 不给结论）。
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// clrcTree —— 合成树：被引件 b.go（第 2 行 = func hello）＋ 引用侧 a.md（内容由调用侧给）。
func clrcTree(t *testing.T, aContent string) string {
	t.Helper()
	root := t.TempDir()
	clrcWriteFile(root, "b.go", "package main\nfunc hello() {}\nfunc bye() {}\n")
	clrcWriteFile(root, "a.md", aContent)
	return root
}

func clrcInvoke(t *testing.T, root string, extra ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"--root", root}, extra...)
	rc := checkLinerefCensusGateMain(&out, &errb, argv)
	return rc, out.String() + errb.String()
}

// TestClrcCensus —— 三态：锚点在被引行 ⇒ 绿 / 行号指向不是那个东西 ⇒ 真红点名 / 扫描根缺 ⇒ 不给结论。
func TestClrcCensus(t *testing.T) {
	// 正控：`b.go:2 hello` 而第 2 行正是 func hello ⇒ rc=0（绿 · 不点名）。
	root := clrcTree(t, "见 `b.go:2 hello` 处。\n")
	if rc, out := clrcInvoke(t, root); rc != clrcRCok || strings.Contains(out, "✗") {
		t.Fatalf("正控失败：期望 rc=0 且无 ✗，实得 rc=%d out=%q", rc, out)
	}
	// 负控①：`b.go:3 hello` 而第 3 行不是那个东西 ⇒ rc=1 且点名 引用件:行:目标。
	root = clrcTree(t, "见 `b.go:3 hello` 处。\n")
	rc, out := clrcInvoke(t, root)
	if rc != clrcRCfail {
		t.Fatalf("负控①失败：期望 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "a.md:1") || !strings.Contains(out, "目标=b.go:3") ||
		!strings.Contains(out, "应见符号=hello") {
		t.Fatalf("负控①失败：点名或应见符号缺：%q", out)
	}
	// 负控②：扫描根取不到 ⇒ rc=2（不给结论 · ✗ 不静默回落）。
	if rc, out := clrcInvoke(t, filepath.Join(t.TempDir(), "does-not-exist")); rc != clrcRCblock ||
		!strings.Contains(out, "不给结论") {
		t.Fatalf("负控②失败：期望 rc=2 且「不给结论」，实得 rc=%d out=%q", rc, out)
	}
}

// TestClrcSelfTestGreen —— 本件自检（合成树）应全绿。
func TestClrcSelfTestGreen(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := checkLinerefCensusGateMain(&out, &errb, []string{"--self-test"}); rc != 0 {
		t.Fatalf("--self-test 期望 rc=0，实得 rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
}
