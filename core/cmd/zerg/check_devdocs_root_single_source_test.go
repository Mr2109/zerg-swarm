// check_devdocs_root_single_source_test.go —— `checkDevdocsRootSingleSourceGateMain` 的成对负控
// （口径：**派生默认根字面量 = 0** · 缺口 GAP-20260927-88 重锚）。
//
// ★ 两基线（已重锚 · 旧基线「字面 == 真源 ⇒ 绿」**已废**——那正是恒绿的病）：
//
//	① 绿基线 = 判据件里**无派生默认根字面量**（derive 现算）⇒ rc=0；
//	② 红基线 = **未登记的派生默认根字面量** ⇒ rc=1 且逐条点名（★ 字面**等于真源**也照样红）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdrssTree —— 造一棵合成树：真源（可选）+ 一件判据件（内容由 body 给）。
func cdrssTree(t *testing.T, withTrueSource bool, body string) string {
	t.Helper()
	root := t.TempDir()
	if withTrueSource {
		cdrssWriteFile(root, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	}
	cdrssWriteFile(root, cdrssScanDirRel+"/check_x.go", body)
	return root
}

func cdrssInvoke(t *testing.T, root string, extra ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"--root", root}, extra...)
	rc := checkDevdocsRootSingleSourceGateMain(&out, &errb, argv)
	return rc, out.String() + errb.String()
}

// TestCdrssSingleSource —— 四态：derive（无字面）⇒ 绿 / 未登记字面（含 == 真源）⇒ 真红点名 / 真源缺 ⇒ 不给结论。
func TestCdrssSingleSource(t *testing.T) {
	// 正控（新绿基线）：判据件里**无字面**（derive 现算）⇒ rc=0（绿 · 不点名）。
	root := cdrssTree(t, true, "package main\n\nfunc x() string { return join(sib, \"项目文档\", mapVer(Version)) }\n")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCok || strings.Contains(out, "✗ 未登记") {
		t.Fatalf("正控失败：期望 rc=0 且无未登记，实得 rc=%d out=%q", rc, out)
	}
	// 负控①：未登记的派生默认根字面量 ≠ 真源 ⇒ rc=1 且逐条点名（含 件:行 + 两侧读数）。
	root = cdrssTree(t, true, "package main\n\nfunc x() string { return join(sib, \"项目文档\", \"v2.5.10\") }\n")
	rc, out := cdrssInvoke(t, root)
	if rc != cdrssRCfail {
		t.Fatalf("负控①失败：期望 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "check_x.go:3") ||
		!strings.Contains(out, "字面=项目文档/v2.5.10") ||
		!strings.Contains(out, "真源=v9.9.9") {
		t.Fatalf("负控①失败：点名或两侧读数缺：%q", out)
	}
	// 负控①b（★ 恒绿反证）：字面**等于真源**（v9.9.9）⇒ **仍 rc=1**（「== 真源」不再是逃生口）。
	root = cdrssTree(t, true, "package main\n\nfunc x() string { return join(sib, \"项目文档\", \"v9.9.9\") }\n")
	rc, out = cdrssInvoke(t, root)
	if rc != cdrssRCfail || !strings.Contains(out, "check_x.go:3") {
		t.Fatalf("负控①b失败：等于真源的字面应仍判红并点名，实得 rc=%d out=%q", rc, out)
	}
	// 负控②：真源取不到 ⇒ rc=2（不给结论 · ✗ 不静默回落）。
	root = cdrssTree(t, false, "package main\n\nfunc x() string { return join(sib, \"项目文档\", \"v2.5.10\") }\n")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCblock || !strings.Contains(out, "不给结论") {
		t.Fatalf("负控②失败：期望 rc=2 且「不给结论」，实得 rc=%d out=%q", rc, out)
	}
}

// TestCdrssDatedLedgerAllowed —— ★ 口径未被弄成「凡旧版号就红」：带日期的历史台账/语料 ⇒ 不红。
func TestCdrssDatedLedgerAllowed(t *testing.T) {
	// ① 带日期的历史台账**文件引用**（串尾是件名 · 不是派生默认根）⇒ 不红。
	root := cdrssTree(t, true, "package main\n\nconst ledgerRel = \"项目文档/v2.5.10/清单-脚本现状-20260920.tsv\"\n")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCok {
		t.Fatalf("台账文件引用应不红，实得 rc=%d out=%q", rc, out)
	}
	// ② 形态 C（同级根串）· **已登记**的带日期历史台账位 ⇒ 不红。
	root = t.TempDir()
	cdrssWriteFile(root, cdrssVersionRel, "package version\n\nconst Version = \"9.9.9\"\n")
	cdrssWriteFile(root, cdrssScanDirRel+"/check_idspace_map.go",
		"package main\n\n// idmDefaultDoc —— 台账固定落点\nfunc idmDefaultDoc() string { return root + \"/../Zerg-内部文档/项目文档/v2.5.10/\" + leaf }\n")
	if rc, out := cdrssInvoke(t, root); rc != cdrssRCok {
		t.Fatalf("已登记台账位应不红，实得 rc=%d out=%q", rc, out)
	}
}

// TestCdrssSelfTestGreen —— 本件自检（合成树）应全绿。
func TestCdrssSelfTestGreen(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := checkDevdocsRootSingleSourceGateMain(&out, &errb, []string{"--self-test"}); rc != 0 {
		t.Fatalf("--self-test 期望 rc=0，实得 rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
}

// TestCdrssReadFailTeeth —— B 类读失败不再静默的回归牙：扫描面里一件 .go 锁成 chmod 000 ⇒
// 登记面（cdrssReadFails）必响：rc=2 + stderr BLOCKED 头 + 逐件点名。删回静默本用例必红。
func TestCdrssReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视权限位（chmod 000 仍可读）⇒ 跳过")
	}
	root := cdrssTree(t, true, "package main\n\nfunc x() string { return join(sib, \"项目文档\", mapVer(Version)) }\n")
	cdrssWriteFile(root, cdrssScanDirRel+"/check_y.go", "package main\n\nfunc y() string { return mapVer(Version) }\n")
	bad := filepath.Join(root, "core", "cmd", "zerg", "check_x.go")
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(bad, 0o644) }()
	rc, out := cdrssInvoke(t, root)
	if rc != cdrssRCblock || !strings.Contains(out, "BLOCKED") || !strings.Contains(out, "check_x.go") {
		t.Fatalf("读不到件应 rc=2 + BLOCKED 点名 check_x.go，得 rc=%d out=%q", rc, out)
	}
}
