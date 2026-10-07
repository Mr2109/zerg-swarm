// check_lang_freeze_test.go —— `check-lang-freeze`（语言冻结门）的**三态 + 负控**定向单测。
//
// 口径：仓外夹具（`t.TempDir()` · ✗ 不脏真仓）。用例名公共前缀 `TestCheckLangFreeze`，
// 定向：`go test ./cmd/zerg/ -run TestCheckLangFreeze -count=1`。
//
// 三态（对应设计稿 §三）：
//
//	甲 A 新增  —— 现读多一个自研面 .py ⇒ 判红 ＋ 点名该件；
//	乙 B 未销账 —— 快照有·盘上无·台账无 ⇒ 判红 ＋ 点名；同件进台账 ⇒ 放行（销账口 = zerg script retire）；
//	丙 C 例外  —— 命中 retire_exceptions ⇒ 放行 ＋ 逐条打印理由；
//	＋ 正控正常 ⇒ 绿 · 缺快照 ⇒ 不给结论（rc=2）· 排除面（vendor/ / tools/ocr/venv/）不进自研面。
//
// 哑化负控：抽掉 A 判定支（rc 条件里去掉 `len(added)>0`）⇒ `TestCheckLangFreezeStates` 必红（带行号）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clfRunT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkLangFreezeGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func clfWriteT(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func clfSnapT(t *testing.T, root string, entries []clfEntry) {
	t.Helper()
	s := clfSnapshot{Schema: clfSchema, Generated: "2026-10-07T00:00:00Z", Entries: entries}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	clfWriteT(t, root, clfSnapshotRel, string(b)+"\n")
}

func clfTruthT(t *testing.T, root, raw string) {
	t.Helper()
	clfWriteT(t, root, scriptRetireTruthRel, raw)
}

// clfBaseT —— 仓外夹具根：三件自研面（a.py / b.sh / sub/c.py）＋ 两件排除面（vendor / tools/ocr/venv）。
func clfBaseT(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	clfWriteT(t, root, "a.py", "print(1)\n")
	clfWriteT(t, root, "b.sh", "echo b\n")
	clfWriteT(t, root, "sub/c.py", "print(2)\n")
	clfWriteT(t, root, "vendor/x.py", "print(9)\n")
	clfWriteT(t, root, "tools/ocr/venv/y.py", "print(8)\n")
	clfSnapT(t, root, []clfEntry{
		{Path: "a.py", Lang: "py"}, {Path: "b.sh", Lang: "sh"}, {Path: "sub/c.py", Lang: "py"},
	})
	clfTruthT(t, root, `{"retire_ledger":[],"retire_exceptions":[]}`)
	return root
}

// TestCheckLangFreezeStates —— 三态 ＋ 正控 ＋ 缺快照（成对：绿↔红不同码）。
func TestCheckLangFreezeStates(t *testing.T) {
	// ── 丙 · C 例外：keep.py ∉ 快照 但 ∈ retire_exceptions ⇒ 放行 + 逐条理由 ──
	greenRoot := clfBaseT(t)
	clfWriteT(t, greenRoot, "keep.py", "print(3)\n")
	clfTruthT(t, greenRoot, `{"retire_ledger":[],"retire_exceptions":[{"object":"keep.py","category":"夹具例外类","reason":"夹具例外理由","keep_cost":"夹具代价","review_date":"2026-10-07"}]}`)
	rcGreen, outGreen, errGreen := clfRunT(t, "--check", "--root", greenRoot)
	if rcGreen != 0 {
		t.Fatalf("丙·例外 应 rc=0，得 rc=%d out=%q err=%q", rcGreen, outGreen, errGreen)
	}
	if !strings.Contains(outGreen, "例外放行 keep.py（夹具例外类 · 夹具例外理由）") {
		t.Fatalf("丙·例外 未逐条打印理由：out=%q", outGreen)
	}
	if strings.Contains(outGreen, "新增自研面 py:") {
		t.Fatalf("丙·例外 例外件被误判为新增：out=%q", outGreen)
	}
	if !strings.Contains(outGreen, "⇒ 全绿（rc=0）") {
		t.Fatalf("丙·例外 应报全绿：out=%q", outGreen)
	}

	// ── 甲 · A 新增：多一个自研面 .py ⇒ 判红 + 点名 ──
	rootA := clfBaseT(t)
	clfWriteT(t, rootA, "scripts/gates/new-thing.py", "print(4)\n")
	rcA, outA, _ := clfRunT(t, "--root", rootA)
	if rcA != 1 {
		t.Fatalf("甲·新增 应 rc=1，得 rc=%d out=%q", rcA, outA)
	}
	if !strings.Contains(outA, "新增自研面 py: scripts/gates/new-thing.py（新代码只用 Go；确需保留请入例外名册）") {
		t.Fatalf("甲·新增 点名不符：out=%q", outA)
	}

	// ── 乙 · B 未销账：快照有·盘上无·台账无 ⇒ 判红 + 点名 ──
	rootB := clfBaseT(t)
	clfSnapT(t, rootB, []clfEntry{
		{Path: "a.py", Lang: "py"}, {Path: "b.sh", Lang: "sh"}, {Path: "sub/c.py", Lang: "py"},
		{Path: "scripts/gates/gone.py", Lang: "py"},
	})
	rcB, outB, _ := clfRunT(t, "--root", rootB)
	if rcB != 1 {
		t.Fatalf("乙·未销账 应 rc=1，得 rc=%d out=%q", rcB, outB)
	}
	if !strings.Contains(outB, "scripts/gates/gone.py 已消失但未登记退场（唯一销账口：zerg script retire）") {
		t.Fatalf("乙·未销账 点名不符：out=%q", outB)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码。
	if rcGreen == rcA || rcGreen == rcB {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d/%d 同码 ⇒ 无鉴别力", rcGreen, rcA, rcB)
	}

	// ── 乙' · 同件进 retire_ledger ⇒ 放行（唯一销账口 = script retire）──
	rootB2 := clfBaseT(t)
	clfSnapT(t, rootB2, []clfEntry{
		{Path: "a.py", Lang: "py"}, {Path: "b.sh", Lang: "sh"}, {Path: "sub/c.py", Lang: "py"},
		{Path: "scripts/gates/gone.py", Lang: "py"},
	})
	clfTruthT(t, rootB2, `{"retire_ledger":[{"object":"scripts/gates/gone.py","state":"已登记退场","evidence":"夹具"}],"retire_exceptions":[]}`)
	rcB2, outB2, _ := clfRunT(t, "--root", rootB2)
	if rcB2 != 0 {
		t.Fatalf("乙'·已销账 应 rc=0，得 rc=%d out=%q", rcB2, outB2)
	}
	if strings.Contains(outB2, "已消失但未登记退场") {
		t.Fatalf("乙'·已销账 仍报未销账：out=%q", outB2)
	}

	// ── 正控 · 排除面不动色（vendor/ 与 tools/ocr/venv/ 下 .py ✗ 计入；快照只含 3 件 ⇒ 绿）──
	rcBase, outBase, _ := clfRunT(t, "--root", clfBaseT(t))
	if rcBase != 0 {
		t.Fatalf("正控·基础夹具 应 rc=0（排除面不计入），得 rc=%d out=%q", rcBase, outBase)
	}
	if !strings.Contains(outBase, "现读 3 件") {
		t.Fatalf("正控·基础夹具 现读件数应 3（排除 vendor/ 与 tools/ocr/venv/）：out=%q", outBase)
	}

	// ── 缺快照 ⇒ 不给结论（rc=2）──
	rootMiss := t.TempDir()
	clfWriteT(t, rootMiss, "a.py", "print(1)\n")
	rcMiss, _, errMiss := clfRunT(t, "--root", rootMiss)
	if rcMiss != 2 || !strings.Contains(errMiss, "读不到冻结快照") {
		t.Fatalf("缺快照 应 rc=2 且报「读不到冻结快照」，得 rc=%d err=%q", rcMiss, errMiss)
	}
}

// TestCheckLangFreezeEmit —— `--emit` 写快照：件数 / 集合 / 排除面逐条现读断言（✗ 不写死数字）。
func TestCheckLangFreezeEmit(t *testing.T) {
	root := t.TempDir()
	clfWriteT(t, root, "a.py", "print(1)\n")
	clfWriteT(t, root, "b.sh", "echo b\n")
	clfWriteT(t, root, "sub/c.py", "print(2)\n")
	clfWriteT(t, root, "sub/__pycache__/junk.py", "print(0)\n") // __pycache__ 跳过
	clfWriteT(t, root, "vendor/x.py", "print(9)\n")             // vendor/ 排除
	clfWriteT(t, root, "tools/ocr/venv/y.py", "print(8)\n")     // tools/ocr/venv/ 排除
	clfWriteT(t, root, "tools/ocr/z.py", "print(7)\n")          // tools/ocr/（非 venv）**计入**

	out := filepath.Join(root, filepath.FromSlash(clfSnapshotRel))
	rc, sout, serr := clfRunT(t, "--emit", "--root", root)
	if rc != 0 {
		t.Fatalf("--emit 应 rc=0，得 rc=%d err=%q", rc, serr)
	}
	if !strings.Contains(sout, "4 件（py 3 + sh 1）") {
		t.Fatalf("--emit 计数不符（应 4 件 py3+sh1）：out=%q", sout)
	}
	if !strings.Contains(sout, out) {
		t.Fatalf("--emit 应报默认落点 %s：out=%q", out, sout)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var snap clfSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatalf("快照解不动：%v", err)
	}
	if snap.Schema != clfSchema {
		t.Fatalf("schema 不符：%q", snap.Schema)
	}
	var got []string
	for _, e := range snap.Entries {
		got = append(got, e.Path)
	}
	want := []string{"a.py", "b.sh", "sub/c.py", "tools/ocr/z.py"} // 已排序
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("快照集合/序不符：\n got=%v\nwant=%v", got, want)
	}
	// 快照写完后再 `--check` 同一夹具 ⇒ 必绿（写面 ⟷ 判面自洽）。
	if rc2, out2, _ := clfRunT(t, "--check", "--root", root); rc2 != 0 {
		t.Fatalf("emit 后 check 应 rc=0，得 rc=%d out=%q", rc2, out2)
	}
}

// TestCheckLangFreezeSelfTest —— 合成自检（仓外临时树）⇒ rc=0。
func TestCheckLangFreezeSelfTest(t *testing.T) {
	rc, out, errs := clfRunT(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 rc=%d out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "0 格不过") {
		t.Fatalf("自检应报「0 格不过」：out=%q", out)
	}
}

// TestCheckLangFreezeUsage —— 用法档：未知旗标 / 三档互斥 / `-h` ⇒ 退码与文案。
func TestCheckLangFreezeUsage(t *testing.T) {
	if rc, _, _ := clfRunT(t, "--nope"); rc != 2 {
		t.Fatalf("未知旗标 应 rc=2，得 rc=%d", rc)
	}
	if rc, _, _ := clfRunT(t, "--check", "--emit"); rc != 2 {
		t.Fatalf("--check 与 --emit 同给 应 rc=2，得 rc=%d", rc)
	}
	if rc, out, _ := clfRunT(t, "-h"); rc != 0 || !strings.Contains(out, clfJudgeKey) {
		t.Fatalf("`-h` 应 rc=0 且带件名，得 rc=%d out=%q", rc, out)
	}
}
