// check_devdocs_root_single_source_test.go —— `checkDevdocsRootSingleSourceGateMain` 的成对负控
// （正控绿 / 默认根≠真源 ⇒ 真红点名 / 真源取不到 ⇒ 不给结论）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// cdrssTree —— 造一棵合成树：真源（可选）+ 一件判据件（开发文档默认根版号 = ver）。
func cdrssTree(t *testing.T, withTrueSource bool, ver string) string {
	t.Helper()
	root := t.TempDir()
	if withTrueSource {
		cdrssWriteFile(root, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	}
	cdrssWriteFile(root, cdrssScanDirRel+"/check_x.go",
		"package main\n\nfunc x() string { return join(sib, \"项目文档\", \""+ver+"\") }\n")
	return root
}

func cdrssInvoke(t *testing.T, root string, extra ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"--root", root}, extra...)
	rc := checkDevdocsRootSingleSourceGateMain(&out, &errb, argv)
	return rc, out.String() + errb.String()
}

// TestCdrssSingleSource —— 三态：相符 ⇒ 绿 / 不符 ⇒ 真红点名 / 真源缺 ⇒ 不给结论。
func TestCdrssSingleSource(t *testing.T) {
	// 正控：默认根 == 真源 ⇒ rc=0（绿 · 不点名）。
	root := cdrssTree(t, true, "v9.9.9")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCok || strings.Contains(out, "✗ 不符") {
		t.Fatalf("正控失败：期望 rc=0 且无不符，实得 rc=%d out=%q", rc, out)
	}
	// 负控①：默认根 ≠ 真源 ⇒ rc=1 且逐条点名（含 件:行 + 两侧读数）。
	root = cdrssTree(t, true, "v2.5.10")
	rc, out := cdrssInvoke(t, root)
	if rc != cdrssRCfail {
		t.Fatalf("负控①失败：期望 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "check_x.go:3") ||
		!strings.Contains(out, "取的路径=项目文档/v2.5.10") ||
		!strings.Contains(out, "真源=v9.9.9") {
		t.Fatalf("负控①失败：点名或两侧读数缺：%q", out)
	}
	// 负控②：真源取不到 ⇒ rc=2（不给结论 · ✗ 不静默回落）。
	root = cdrssTree(t, false, "v2.5.10")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCblock || !strings.Contains(out, "不给结论") {
		t.Fatalf("负控②失败：期望 rc=2 且「不给结论」，实得 rc=%d out=%q", rc, out)
	}
}

// TestCdrssSelfTestGreen —— 本件自检（合成树）应全绿。
func TestCdrssSelfTestGreen(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := checkDevdocsRootSingleSourceGateMain(&out, &errb, []string{"--self-test"}); rc != 0 {
		t.Fatalf("--self-test 期望 rc=0，实得 rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
}
