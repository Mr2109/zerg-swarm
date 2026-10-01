// check_glossary_test.go —— Go 真搬的**成对负控**定向单测（M2 · 新口径靶：外部树走显式入参旋钮）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckGlossaryGatePort（定向：go test -run TestCheckGlossaryGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chglFixture —— 在 t.TempDir() 下造一棵仓外夹具（docs/术语表.tsv + 中/英各一好件）。
func chglFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "zh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	cols := strings.Join(chglCols, "\t") + "\n"
	rows := "虫茧\tcocoon\tcocoon\tcocoon\t虫巢|集装箱\tpod\tMr2109\t只此一译\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "术语表.tsv"), []byte(cols+rows), 0o644); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntitle: T\ntranslation_of: docs/zh/good.md\nai_assisted: ai\ntranslation_status: machine_draft\n---\n"
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/zh/good.md", "本文只写正名：这里叫虫茧。\n")
	write("docs/en/good.md", fm+"只跑 `doc_search`。\n<!-- 跑 `doc_search` -->\n")
	return root
}

// chglRunIn —— 把 cwd 切到 root 跑入口（Go 侧仓根起于 cwd 上溯 · 见件头 (a)）。
func chglRunIn(t *testing.T, root string, argv ...string) (int, string, string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := checkGlossaryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckGlossaryGatePort(t *testing.T) {
	// ── ① 正控（好件 ⇒ 0）：中文页只写正名 · 词表 1 条 · 白名单 7 篇口径同时核 ──────
	fx := chglFixture(t)
	rcOK, outOK, _ := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/good.md")
	if rcOK != 0 || !strings.Contains(outOK, "合计：T1 0 · T2 0 · T3 0 · 单条豁免 0") ||
		!strings.Contains(outOK, "白名单 7 篇") {
		t.Fatalf("正控失败：好件应 rc=0 且全零，得 rc=%d out=%q", rcOK, outOK)
	}

	// ── ② 负控（T2 命中 ⇒ 1）：中文页出现 forbid_zh 词（虫巢），判词点名正名 ─────────
	if err := os.WriteFile(filepath.Join(fx, "docs", "zh", "bad.md"), []byte("旧称：这里叫虫巢。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcT2, outT2, _ := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/bad.md")
	if rcT2 != 1 || !strings.Contains(outT2, "T2 禁用中文写法") || !strings.Contains(outT2, "← 正名「虫茧」") {
		t.Fatalf("T2 负控失败：应 rc=1 且点名正名，得 rc=%d out=%q", rcT2, outT2)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcOK == rcT2 {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcOK, rcT2)
	}

	// ── ③ 负控（T1 阻塞 ⇒ 2）：英文译页缺三必填 frontmatter ────────────────────────
	if err := os.WriteFile(filepath.Join(fx, "docs", "en", "b4.md"), []byte("---\ntitle: T\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcT1, outT1, _ := chglRunIn(t, fx, "--lang", "en", "--paths", "docs/en/b4.md")
	if rcT1 != 2 || !strings.Contains(outT1, "T1 B4 译页三必填缺 translation_of/ai_assisted/translation_status") {
		t.Fatalf("T1 负控失败：应 rc=2 且点名三条必填，得 rc=%d out=%q", rcT1, outT1)
	}
	if rcT1 == rcT2 || rcT1 == rcOK {
		t.Fatalf("成对断言失败：T1 rc=%d 未与 T2/正控分档", rcT1)
	}

	// ── ④ 成对负控（--enforce 把 T2 升档 ⇒ 2）────────────────────────────────────
	rcEnf, _, _ := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/bad.md", "--enforce")
	if rcEnf != 2 {
		t.Fatalf("--enforce 升档失败：T2 命中应 rc=2，得 rc=%d", rcEnf)
	}

	// ── ⑤ 不给结论（≠ 绿）：主表取不到（null 形态）⇒ rc=2 ──────────────────────────
	os.Setenv("ZERG_GLOSSARY_TSV", filepath.Join(fx, "docs", "没有这份.tsv"))
	rcG, _, errG := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/good.md")
	os.Unsetenv("ZERG_GLOSSARY_TSV")
	if rcG != 2 || !strings.Contains(errG, "术语表主表不存在") {
		t.Fatalf("主表缺失败：应 rc=2，得 rc=%d stderr=%q", rcG, errG)
	}

	// ── ⑥ 不给结论（≠ 绿）：空表（[] 零数据行）与缺键（表头不符）⇒ 均 rc=2 ──────────
	emptyTSV := filepath.Join(fx, "empty.tsv")
	if err := os.WriteFile(emptyTSV, []byte(strings.Join(chglCols, "\t")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("ZERG_GLOSSARY_TSV", emptyTSV)
	rcE, _, errE := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/good.md")
	os.Unsetenv("ZERG_GLOSSARY_TSV")
	if rcE != 2 || !strings.Contains(errE, "主表零数据行") {
		t.Fatalf("空表失败：应 rc=2，得 rc=%d stderr=%q", rcE, errE)
	}
	keyTSV := filepath.Join(fx, "keys.tsv")
	if err := os.WriteFile(keyTSV, []byte("zh\ten\n虫茧\tcocoon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("ZERG_GLOSSARY_TSV", keyTSV)
	rcK, _, errK := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/good.md")
	os.Unsetenv("ZERG_GLOSSARY_TSV")
	if rcK != 2 || !strings.Contains(errK, "表头不符") {
		t.Fatalf("缺键失败：应 rc=2，得 rc=%d stderr=%q", rcK, errK)
	}

	// ── ⑦ 不给结论（≠ 绿）：扫到 0 个 .md（空转）⇒ rc=2 ──────────────────────────
	if err := os.MkdirAll(filepath.Join(fx, "docs", "zh", "空"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc0, _, err0 := chglRunIn(t, fx, "--lang", "zh", "--paths", "docs/zh/空")
	if rc0 != 2 || !strings.Contains(err0, "扫到 0 个 .md") {
		t.Fatalf("空转失败：应 rc=2，得 rc=%d stderr=%q", rc0, err0)
	}

	// ── ⑧ 读不了的件 ⇒ rc=2（fail-closed · ✗ 不静默绿）────────────────────────────
	noRead := filepath.Join(fx, "noread.md")
	if err := os.WriteFile(noRead, []byte("旧称：这里叫虫巢。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chmod(noRead, 0)
	rcNR, _, errNR := chglRunIn(t, fx, "--lang", "zh", "--paths", "noread.md")
	if rcNR != 2 || !strings.Contains(errNR, "读不了") {
		t.Fatalf("读不了失败：应 rc=2，得 rc=%d stderr=%q", rcNR, errNR)
	}

	// ── ⑨ 不给结论（≠ 绿）：devdocs 根整棵不在盘 ⇒ rc=2 ──────────────────────────
	os.Setenv("ZERG_DEVDOCS_ROOT", filepath.Join(fx, "no-such-Zerg-内部文档"))
	rcD, _, errD := chglRunIn(t, fx, "--scope", "devdocs")
	os.Unsetenv("ZERG_DEVDOCS_ROOT")
	if rcD != 2 || !strings.Contains(errD, "开发文档面根不在") {
		t.Fatalf("devdocs 缺根失败：应 rc=2，得 rc=%d stderr=%q", rcD, errD)
	}

	// ── ⑩ Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说）──────────────────
	rcJSON, _, errJSON := chglRunIn(t, fx, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") || !strings.Contains(errJSON, "字段表：无") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── ⑪ 未知旗标 ⇒ rc=2（逐字点名 · 不静默吞）· 正控 rc=0 已成对 ─────────────────
	rcBog, _, errBog := chglRunIn(t, fx, "--bogus")
	if rcBog != 2 || !strings.Contains(errBog, "✗ 未知参数: --bogus") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcBog, errBog)
	}

	// ── ⑫ -h 帮助正文逐字（py docstring「用法」段）⇒ rc=0 ─────────────────────────
	rcH, outH, _ := chglRunIn(t, fx, "-h")
	if rcH != 0 || !strings.HasPrefix(outH, "用法：\n  python3 scripts/gates/check-glossary.py [--lang zh,en]") ||
		!strings.Contains(outH, "--root <目录>    换扫描根") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── ⑬ --self-test 自身必须全过（防恒绿/恒红）⇒ rc=0 + 末行自印 ──────────────────
	rcST, outST, _ := chglRunIn(t, fx, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自证：通过 ✓") || !strings.Contains(outST, "自检：全过 ✓") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
