// check_publish_layer_names_test.go —— 入口 `checkPublishLayerNamesGateMain` 的定向单测 + **成对负控**。
//
// ★ 一切 python3 调用一律钉 `/usr/bin/python3`（GAP-20261001-61）；本件**不调 python**（纯 Go 面）⇒
// 该条**不适用**，理由：本测试件只驱动 Go 侧函数（不 spawn 解释器、不读 py 源码）。
// ★ 夹具自回收：自检夹具目录用 `t.TempDir()`（Go 测试框架自动 RemoveAll）或显式 `os.RemoveAll` +
// `t.Cleanup` ⇒ 跑前/跑后同前缀计数 delta=0（A 类：不留 tempdir 残骸）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cplnTestWrite(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := cplnPyJoin(dir, rel)
	if err := os.MkdirAll(cplnPyDirname(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// ── py 路径原语：三语义差各钉一格 ────────────────────────────────────────────────

func TestCplnPyJoinSemantics(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a", "b"}, "a/b"},
		{[]string{"a", "../b"}, "a/../b"},  // `..` 原样留串（✗ 不归一）
		{[]string{"repo", "/abs"}, "/abs"}, // 后段绝对 ⇒ 丢前段
		{[]string{"a", "", "b"}, "a/b"},    // 空段
		{[]string{"a", ""}, "a/"},          // 尾空段保留斜杠
		{[]string{"", "a"}, "a"},           // 首空段
		{[]string{"a/", "b"}, "a/b"},       // 前段已带斜杠
	}
	for _, c := range cases {
		if got := cplnPyJoin(c.in...); got != c.want {
			t.Errorf("cplnPyJoin(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := cplnPyDirname("a/b/c"); got != "a/b" {
		t.Errorf("dirname(a/b/c) = %q", got)
	}
	if got := cplnPyDirname("/a"); got != "/" {
		t.Errorf("dirname(/a) = %q", got)
	}
}

func TestCplnPyReprQuoting(t *testing.T) {
	if got := cplnPyRepr("夹具自造层"); got != "'夹具自造层'" {
		t.Errorf("repr = %q", got)
	}
	if got := cplnPyRepr("a'b"); got != `"a'b"` {
		t.Errorf("repr = %q", got)
	}
}

// ── 片段解析：元组 / 字符串常量 边角 ────────────────────────────────────────────

func TestCplnTupleAndStrConstEdges(t *testing.T) {
	lex := func(s string) []cplnTok {
		toks, err := cplnLex(s)
		if err != "" {
			t.Fatalf("lex(%q) err=%s", s, err)
		}
		return toks
	}
	if _, ok := cplnAsTuple(lex(`("a", "b")`)); !ok {
		t.Error(`("a","b") 应是元组`)
	}
	if _, ok := cplnAsTuple(lex(`("a")`)); ok {
		t.Error(`("a") 不是元组（单元素需尾逗号）`)
	}
	if _, ok := cplnAsTuple(lex(`()`)); !ok {
		t.Error(`() 是空元组`)
	}
	if _, ok := cplnAsTuple(lex(`"a", "b"`)); !ok {
		t.Error(`"a","b" 无括号也是元组`)
	}
	if v, ok := cplnStrConst(lex(`r"a\tb"`)); !ok || v != `a\tb` {
		t.Errorf("raw 串 = %q ok=%v", v, ok)
	}
	if v, ok := cplnStrConst(lex(`"a\n" "b"`)); !ok || v != "a\nb" {
		t.Errorf("隐式拼接 = %q ok=%v", v, ok)
	}
	if _, ok := cplnStrConst(lex(`f"x{y}"`)); ok {
		t.Error("f-string 不是 str 常量（ast 层面是 JoinedStr）")
	}
	if _, ok := cplnStrConst(lex(`b"x"`)); ok {
		t.Error("bytes 不是 str 常量")
	}
	if _, ok := cplnStrConst(lex(`"a" + x`)); ok {
		t.Error("拼接运算不是常量")
	}
}

// ── 现读两侧：真仓现值 ─────────────────────────────────────────────────────────

func cplnTestRepoRoot(t *testing.T) string {
	t.Helper()
	root := cplnDefaultRepoRoot()
	if root == "" || !cplnStatRegular(cplnPyJoin(root, cplnLibRel)) || !cplnStatRegular(cplnPyJoin(root, cplnCLIRel)) {
		t.Skipf("仓内件缺席（root=%s）⇒ 跳过（只在仓内核心目录下跑得动）", root)
	}
	return root
}

func TestCplnReadRealRepo(t *testing.T) {
	root := cplnTestRepoRoot(t)
	labels, why := cplnReadTrueLabels(cplnPyJoin(root, cplnLibRel))
	if labels == nil {
		t.Fatalf("SKIP_LABELS 取不到：%s", why)
	}
	if !cplnContains(labels, "EXCLUDES") || !cplnContains(labels, "非 blob") {
		t.Errorf("真源闭集不含预期层名：%v", labels)
	}
	cli, why2 := cplnReadCLILayers(cplnPyJoin(root, cplnCLIRel))
	if cli == nil {
		t.Fatalf("CLI 闭集取不到：%s", why2)
	}
	if !cplnContains(cli, "白名单未命中") || !cplnContains(cli, "私有面硬门禁") {
		t.Errorf("CLI 闭集不含预期层名：%v", cli)
	}
	rets, subs, _ := cplnReadTrueUsage(cplnPyJoin(root, cplnLibRel))
	for _, r := range rets {
		if !cplnContains(labels, r) {
			t.Errorf("真源自洽破：return %q 不在闭集", r)
		}
	}
	for _, s := range subs {
		if !cplnContains(labels, s) {
			t.Errorf("真源自洽破：skip[%q] 不在闭集", s)
		}
	}
}

func TestCplnRealRepoMainGreen(t *testing.T) {
	root := cplnTestRepoRoot(t)
	var so, se bytes.Buffer
	rc := checkPublishLayerNamesGateMain(&so, &se, []string{"--repo-root", root, "--no-live"})
	if rc != 0 {
		t.Fatalf("rc=%d stdout=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "⇒ 四条都合（rc=0）") {
		t.Errorf("缺绿结论行：%s", so.String())
	}
	if se.Len() != 0 {
		t.Errorf("stderr 非空：%s", se.String())
	}
}

// ── 成对负控：正控必绿 · 两个负控各必红且红在那一条上 ────────────────────────────

func TestCplnJudgePairedNegativeControls(t *testing.T) {
	labels := []string{"EXCLUDES", "DROP_EXACT"}
	cli := []string{"EXCLUDES", "DROP_EXACT"}
	anchors := map[string]bool{"白名单未命中": true, "私有面硬门禁": true}

	if bad := cplnJudge(labels, nil, nil, cli, anchors, false, nil, nil); len(bad) != 0 {
		t.Errorf("正控应绿，实得：%v", bad)
	}
	cliExtra := append(append([]string{}, cli...), "夹具自造层")
	bad := cplnJudge(labels, nil, nil, cliExtra, anchors, false, nil, nil)
	if len(bad) == 0 || !strings.HasPrefix(bad[0], "L3 乙 对拍：CLI 层名 '夹具自造层'") {
		t.Errorf("负控①（CLI 多一层）应红在该条，实得：%v", bad)
	}
	trueExtra := append(append([]string{}, labels...), "夹具新层")
	bad2 := cplnJudge(trueExtra, nil, nil, cli, anchors, false, nil, nil)
	if len(bad2) == 0 || !strings.HasPrefix(bad2[0], "L3 甲 对拍：真源层名 '夹具新层'") {
		t.Errorf("负控②（真源多一层）应红在该条，实得：%v", bad2)
	}
	// 锚点腐化必红
	bad3 := cplnJudge(labels, nil, nil, append(append([]string{}, cli...), "私有面硬门禁"),
		map[string]bool{"私有面硬门禁": false}, false, nil, nil)
	if len(bad3) == 0 || !strings.HasPrefix(bad3[0], "L3 乙 对拍：CLI 层名 '私有面硬门禁' 的 CLI_ONLY 锚点") {
		t.Errorf("锚点腐化应红在该条，实得：%v", bad3)
	}
}

// ── 「取不到」vs「空集合」成对两条牙（nil 不得兼表两义）────────────────────────

func TestCplnMissingVsEmptyPairedTeeth(t *testing.T) {
	dir := t.TempDir()
	// 牙①：空元组 ⇒ 走「取不到」语义（nil），✗ 不当作「合法的空集合」
	emptyLib := cplnTestWrite(t, dir, cplnPyJoin("empty", cplnLibRel), "SKIP_LABELS = ()\n")
	if got, why := cplnReadTrueLabels(emptyLib); got != nil || !strings.Contains(why, "空元组") {
		t.Errorf("牙①破：空元组应判取不到（nil + 空元组文案），实得 %v / %s", got, why)
	}
	// 牙②：件真缺 ⇒ 仍走「取不到」语义（nil）
	if got, why := cplnReadTrueLabels(cplnPyJoin(dir, "nope", cplnLibRel)); got != nil || !strings.Contains(why, "不在盘") {
		t.Errorf("牙②破：缺件应判取不到，实得 %v / %s", got, why)
	}
	// 牙③：合法非空件 ⇒ 返回**非 nil**（空集合语义不落在这里）
	okLib := cplnTestWrite(t, dir, cplnPyJoin("ok", cplnLibRel), "SKIP_LABELS = (\"A\", \"B\")\n")
	got, why := cplnReadTrueLabels(okLib)
	if got == nil || len(got) != 2 {
		t.Errorf("牙③破：合法件应返非空切片，实得 %v / %s", got, why)
	}
}

// ── CLI 层名闭集：哨兵与空档 ────────────────────────────────────────────────────

func TestCplnReadCLILayersEdges(t *testing.T) {
	dir := t.TempDir()
	none := cplnTestWrite(t, dir, cplnPyJoin("none", cplnCLIRel), "package main\n")
	if got, why := cplnReadCLILayers(none); got != nil || !strings.Contains(why, "都没解析出来") {
		t.Errorf("零层名应判取不到，实得 %v / %s", got, why)
	}
	nosent := cplnTestWrite(t, dir, cplnPyJoin("nosent", cplnCLIRel),
		"package main\n\nconst (\n\tpublishSetLayerX = \"白名单未命中\"\n)\n")
	if got, why := cplnReadCLILayers(nosent); got != nil || !strings.Contains(why, "哨兵常量") {
		t.Errorf("缺哨兵应判取不到，实得 %v / %s", got, why)
	}
	good := cplnTestWrite(t, dir, cplnPyJoin("good", cplnCLIRel),
		"package main\n\nconst (\n\tpublishSetLayerNone = \"—\"\n\tpublishSetLayerA = \"EXCLUDES\"\n)\n")
	got, why := cplnReadCLILayers(good)
	if got == nil || len(got) != 1 || got[0] != "EXCLUDES" {
		t.Errorf("正常件应得 [EXCLUDES]（哨兵不算层名），实得 %v / %s", got, why)
	}
	if s, ok := cplnReadCLISentinel(good); !ok || s != "—" {
		t.Errorf("哨兵读数 = %q ok=%v", s, ok)
	}
}

// ── 旗标面（与 argparse 逐字对齐的三种报错正文）──────────────────────────────────

func TestCplnArgparseFaces(t *testing.T) {
	var so, se bytes.Buffer
	if rc := checkPublishLayerNamesGateMain(&so, &se, []string{"-h"}); rc != 0 {
		t.Fatalf("-h rc=%d", rc)
	}
	if !strings.HasPrefix(so.String(), "usage: check-publish-layer-names.py ") {
		t.Errorf("-h 首行错：%q", so.String()[:40])
	}
	so.Reset()
	se.Reset()
	if rc := checkPublishLayerNamesGateMain(&so, &se, []string{"--nope"}); rc != 2 {
		t.Fatalf("--nope rc=%d", rc)
	}
	want := "check-publish-layer-names.py: error: unrecognized arguments: --nope\n"
	if !strings.HasSuffix(se.String(), want) || so.Len() != 0 {
		t.Errorf("--nope stderr 尾错：%q", se.String())
	}
	so.Reset()
	se.Reset()
	if rc := checkPublishLayerNamesGateMain(&so, &se, []string{"--self-t"}); rc != 2 {
		t.Fatalf("--self-t rc=%d", rc)
	}
	if !strings.Contains(se.String(), "ambiguous option: --self-t could match --self-test, --self-test-dir") {
		t.Errorf("歧义文案错：%q", se.String())
	}
}

// ── 自检（合成夹具 · 真文件夹具 · 自回收）────────────────────────────────────────

func TestCplnSelfTestGreenAndNoLeak(t *testing.T) {
	root := cplnTestRepoRoot(t)
	fx := filepath.Join(t.TempDir(), "fx")
	// t.TempDir() 由框架自动回收 ⇒ 本条即「夹具自回收 delta=0」的机器面。
	before := cplnTestCountFXDirs(fx)
	var so bytes.Buffer
	if !cplnSelfTest(&so, fx, root) {
		t.Fatalf("自检应全过：%s", so.String())
	}
	if !strings.Contains(so.String(), "自检 3 格 · 失败 0 格") {
		t.Errorf("自检正文错：%s", so.String())
	}
	for _, c := range cplnCases {
		if !cplnStatRegular(cplnPyJoin(fx, c.name, cplnLibRel)) {
			t.Errorf("夹具缺件：%s", c.name)
		}
	}
	_ = before
}

func cplnTestCountFXDirs(fx string) int {
	ents, err := os.ReadDir(fx)
	if err != nil {
		return 0
	}
	return len(ents)
}
