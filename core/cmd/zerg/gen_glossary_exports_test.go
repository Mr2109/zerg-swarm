package main

// gen_glossary_exports_test.go —— `gen_glossary_exports.go` 的成对负控。
// 牙口：精确字节锁（四份机器词表）＋ rc 档（--check 一致/漂移 · 空输入 · 旗标错 · 只读性）。
// ✗ 无包级 init / TestMain（同包已有他件占用）。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ggexColsTSV = "zh\ten\tid\tdo_not_translate\tforbid_zh\tforbid_en\towner\tnote"

func ggexTinyMain() string {
	return ggexColsTSV + "\n" +
		"虫茧\tcocoon\tcocoon\tcocoon\t虫巢|集装箱\tpod\tMr2109\t只此一译\n" +
		"承接项\t\t{item}\t\t\t\tMr2109\t英文待拍\n"
}

func ggexWriteMainT(t *testing.T, root, main string, mtime int64) {
	t.Helper()
	p := filepath.Join(root, "docs", "术语表.tsv")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	if mtime > 0 {
		ts := time.Unix(mtime, 0)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
}

func ggexExportText(outs []ggexExport, rel string) string {
	for _, o := range outs {
		if strings.HasSuffix(o.path, "/"+rel) {
			return o.text
		}
	}
	return "\x00缺件"
}

// ggexProbeRoot —— 造一棵仓外夹具（含 cwd 上溯标记）并返回其根。
func ggexProbeRoot(t *testing.T, main string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "cmd", "zerg", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if main != "" {
		ggexWriteMainT(t, root, main, 1770000000)
	}
	return root
}

func TestGgexSplitMulti(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a|b|a", []string{"a", "b"}},
		{" a | b ", []string{"a", "b"}},
		{"", []string{}},
		{"|x||y|", []string{"x", "y"}},
	}
	for _, c := range cases {
		if got := ggexSplitMulti(c.in); !ggexStrEq(got, c.want) {
			t.Errorf("ggexSplitMulti(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestGgexReadMainErrors(t *testing.T) {
	root := t.TempDir()
	// 缺件
	if _, err := ggexReadMain(filepath.Join(root, "docs", "术语表.tsv")); err == nil ||
		!strings.Contains(err.Error(), "主表不存在") {
		t.Fatalf("缺件应报「主表不存在」，得 %v", err)
	}
	// 表头不符
	ggexWriteMainT(t, root, "a\tb\n", 0)
	_, err := ggexReadMain(filepath.Join(root, "docs", "术语表.tsv"))
	if err == nil || !strings.Contains(err.Error(), "表头不符") {
		t.Fatalf("表头不符应报「表头不符」，得 %v", err)
	}
	// 字段数不足
	ggexWriteMainT(t, root, ggexColsTSV+"\n虫茧\tcocoon\n", 0)
	_, err = ggexReadMain(filepath.Join(root, "docs", "术语表.tsv"))
	if err == nil || !strings.Contains(err.Error(), "字段数 = 2") {
		t.Fatalf("字段数不足应报字段数，得 %v", err)
	}
	// 空 zh
	ggexWriteMainT(t, root, ggexColsTSV+"\n\tcocoon\tc\tc\t\t\tM\t\n", 0)
	_, err = ggexReadMain(filepath.Join(root, "docs", "术语表.tsv"))
	if err == nil || !strings.Contains(err.Error(), "zh 为空") {
		t.Fatalf("空 zh 应报「zh 为空」，得 %v", err)
	}
	// 零数据行
	ggexWriteMainT(t, root, ggexColsTSV+"\n", 0)
	_, err = ggexReadMain(filepath.Join(root, "docs", "术语表.tsv"))
	if err == nil || !strings.Contains(err.Error(), "零数据行") {
		t.Fatalf("零数据行应报「零数据行」，得 %v", err)
	}
}

func TestGgexBuildExactByte(t *testing.T) {
	root := t.TempDir()
	ggexWriteMainT(t, root, ggexTinyMain(), 1770000000)
	outs, st, err := ggexBuild(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"docs/术语表.jieba.txt":                         "虫茧 10000 nz\n承接项 10000 nz\n",
		"docs/术语表.forbid.txt":                        "虫巢\t术语统一：只有「虫茧」（docs/术语表.tsv）\n集装箱\t术语统一：只有「虫茧」（docs/术语表.tsv）\npod\t术语统一：只有「虫茧」（docs/术语表.tsv）\n",
		"styles/config/vocabularies/Zerg/accept.txt": "虫茧\ncocoon\n承接项\n",
		"styles/config/vocabularies/Zerg/reject.txt": "虫巢\n集装箱\npod\n",
	}
	for rel, exp := range want {
		if got := ggexExportText(outs, rel); got != exp {
			t.Errorf("%s 逐字节不同：\n got=%q\nwant=%q", rel, got, exp)
		}
	}
	// md 结构面（updated_at 随 mtime，故不锁整 sha）
	md := ggexExportText(outs, "docs/术语表.md")
	for _, sub := range []string{
		"---\ntitle: 术语表（生成物 · 只读）",
		"source_of_truth: docs/术语表.tsv",
		"updated_at: ",
		"条数：主表 2 条 · jieba 词表 2 行 · 门禁词表 3 条 · Vale accept 3 条 / reject 3 条",
		"| 虫茧 | cocoon | cocoon | cocoon | 虫巢|集装箱 | pod | Mr2109 | 只此一译 |",
		"## `en` 列为空的词条（**不得进译稿**，共 1 条）",
		"- **承接项**（id: {item}）—— 英文待拍",
	} {
		if !strings.Contains(md, sub) {
			t.Errorf("md 缺片段 %q", sub)
		}
	}
	if st["rows"] != 2 || st["empty_en"] != 1 || st["jieba"] != 2 {
		t.Errorf("stats 不符：%v", st)
	}
}

// 负控：去重若失效 ⇒ 重复 zh 会多出词表行，本用例必红。
func TestGgexDedupNegativeControl(t *testing.T) {
	root := t.TempDir()
	main := ggexColsTSV + "\n" +
		"虫茧\tcocoon\tcocoon\tcocoon\t\t\tM\t\n" +
		"虫茧\tdup\tdup\tdup\t\t\tM\t重复行应被去重\n"
	ggexWriteMainT(t, root, main, 1770000000)
	outs, _, err := ggexBuild(root)
	if err != nil {
		t.Fatal(err)
	}
	jieba := ggexExportText(outs, "docs/术语表.jieba.txt")
	if strings.Count(jieba, "\n") != 1 {
		t.Fatalf("去重失效：jieba 行数 %d（应 1）\n%q", strings.Count(jieba, "\n"), jieba)
	}
}

// 负控：Vale 正则过滤若放宽 ⇒ 含元字符条目会进 accept，本用例必红。
func TestGgexValeFilterNegativeControl(t *testing.T) {
	root := t.TempDir()
	main := ggexColsTSV + "\n承接项\tcocoon\t{item}\t{: #slug }\t\t\tM\t\n"
	ggexWriteMainT(t, root, main, 1770000000)
	outs, _, err := ggexBuild(root)
	if err != nil {
		t.Fatal(err)
	}
	acc := ggexExportText(outs, "styles/config/vocabularies/Zerg/accept.txt")
	if acc != "承接项\ncocoon\n" {
		t.Fatalf("Vale 过滤失效：accept=%q（应「承接项\\ncocoon\\n」）", acc)
	}
}

func TestGgexSelftestDirect(t *testing.T) {
	var b bytes.Buffer
	if rc := ggexSelftest(&b); rc != 0 {
		t.Fatalf("selftest rc=%d\n%s", rc, b.String())
	}
	if !strings.Contains(b.String(), "自证：通过 ✓（10 条）") {
		t.Fatalf("selftest 输出不符：%q", b.String())
	}
}

func TestGgexRunFlagsAndCheck(t *testing.T) {
	root := ggexProbeRoot(t, ggexTinyMain())
	t.Chdir(root)
	run := func(argv ...string) (int, string, string) {
		var o, e bytes.Buffer
		rc := genGlossaryExportsGateMain(&o, &e, argv)
		return rc, o.String(), e.String()
	}
	// 未知旗标 rc=2
	if rc, _, e := run("--bogus"); rc != 2 || !strings.Contains(e, "✗ 未知参数: --bogus") {
		t.Fatalf("未知旗标：rc=%d err=%q", rc, e)
	}
	// --help rc=0 · 印 __doc__ · 不写盘
	rc, o, _ := run("--help")
	if rc != 0 || !strings.HasPrefix(o, "gen-glossary-exports.py —— ") || len(o) < 40 {
		t.Fatalf("--help：rc=%d out=%q", rc, o)
	}
	// 空初始生成 ⇒ rc=0 · 五件写完
	rc, o, _ = run()
	if rc != 0 || strings.Count(o, "   写 ") != 5 {
		t.Fatalf("生成：rc=%d out=%q", rc, o)
	}
	for _, rel := range []string{
		"docs/术语表.md", "docs/术语表.jieba.txt", "docs/术语表.forbid.txt",
		"styles/config/vocabularies/Zerg/accept.txt", "styles/config/vocabularies/Zerg/reject.txt",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("生成后缺件 %s：%v", rel, err)
		}
	}
	// 生成后 --check 一致 ⇒ rc=0
	if rc, _, _ = run("--check"); rc != 0 {
		t.Fatalf("一致 --check rc=%d（应 0）", rc)
	}
	// 只读性：--check 不改产出
	mdp := filepath.Join(root, "docs", "术语表.md")
	b1, _ := os.ReadFile(mdp)
	run("--check")
	b2, _ := os.ReadFile(mdp)
	if !bytes.Equal(b1, b2) {
		t.Fatal("--check 改了产出（违约只读）")
	}
	// 手改 ⇒ --check 漂移 rc=1 且 stderr 点名
	if err := os.WriteFile(mdp, append(b1, []byte("手改\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, e := run("--check")
	if rc != 1 || !strings.Contains(e, "导出物与主表不一致") {
		t.Fatalf("漂移：rc=%d err=%q", rc, e)
	}
}

func TestGgexRunEmptyMainRc2(t *testing.T) {
	root := ggexProbeRoot(t, ggexColsTSV+"\n") // 头在 · 零数据行
	t.Chdir(root)
	var o, e bytes.Buffer
	rc := genGlossaryExportsGateMain(&o, &e, []string{"--check"})
	if rc != 2 || !strings.Contains(e.String(), "术语表导出失败") || !strings.Contains(e.String(), "零数据行") {
		t.Fatalf("空输入：rc=%d err=%q", rc, e.String())
	}
}
