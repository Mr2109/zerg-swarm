// check_cli_ownership_test.go —— `check-cli-ownership`（CLI 归属门）的**三态 + 负控**定向单测。
//
// 口径：仓外夹具（`t.TempDir()` · ✗ 不脏真仓）。用例名公共前缀 `TestCliOwnership`，
// 定向：`go test ./cmd/zerg/ -count=1 -run 'CliOwnership|SelfTest'`。
//
// 三态（照设计稿 §四）：
//
//	甲 未接标准路由 —— 闭集件 · 键 ∉ inproc/名册/台账 ⇒ 判红 ＋ 逐字判词「未接 inproc 标准路由」；
//	乙 补键入表     —— 同一件补键入 inproc ⇒ 判绿（标准路由）；
//	丙 例外放行     —— 键 ∈ retire_exceptions ⇒ 放行 ＋ 逐条理由；
//	丁 销账放行     —— 键 ∈ retire_ledger ⇒ 放行；
//	＋ 排除前缀（vendor/ / tools/ocr/venv/）不计入候选 · 缺真源 ⇒ 不给结论（rc=2）。
//
// ★ 负控（**喂空表 = 哑化 inproc 读取支**，✗ 禁改真表造红）：`TestCliOwnershipNegativeControl` 对同一候选
// 「补键入表 ⇒ 绿」对「喂空表 ⇒ 红」**成对**断言 —— 若判据体不真读传入的 inproc 表，该用例必 FAIL（带行号）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ccoRunT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkCliOwnershipGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func ccoWriteT(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ccoBaseT —— 仓外夹具根：一件候选（thing.py）＋ 两件排除面（vendor / tools/ocr/venv）＋ 空真源。
func ccoBaseT(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ccoWriteT(t, root, "thing.py", "print(1)\n")
	ccoWriteT(t, root, "vendor/x.py", "print(9)\n")
	ccoWriteT(t, root, "tools/ocr/venv/y.py", "print(8)\n")
	ccoWriteT(t, root, scriptRetireTruthRel, `{"retire_ledger":[],"retire_exceptions":[]}`)
	return root
}

// TestCliOwnershipStates —— 三态 ＋ 排除面 ＋ 缺真源（成对：绿↔红不同码）。
func TestCliOwnershipStates(t *testing.T) {
	// ── 甲 · 未接标准路由：键 ∉ 表/名册/台账 ⇒ rc=1 + 逐字判词（且裁决走判面 ⇒ 与检索/写作无关）──
	rootA := ccoBaseT(t)
	rcA, outA, errA := ccoRunT(t, "--check", "--root", rootA)
	if rcA != 1 {
		t.Fatalf("甲·未接标准路由 应 rc=1，得 rc=%d out=%q err=%q", rcA, outA, errA)
	}
	if !strings.Contains(outA, "thing.py 未接 inproc 标准路由（键 thing ∉ 判据臂键表 ∧ ∉ 名册 ∧ ∉ 台账）") {
		t.Fatalf("甲·未接标准路由 判词不符（须写「未接 inproc 标准路由」· ✗ 禁「打不着」）：out=%q", outA)
	}
	if strings.Contains(outA, "打不着") || strings.Contains(outA, "不可达") {
		t.Fatalf("甲·判词出现被禁措辞（打不着/不可达）：out=%q", outA)
	}
	if !strings.Contains(outA, "候选 1 件") {
		t.Fatalf("甲·排除前缀应不计入候选（应 1 件）：out=%q", outA)
	}

	// ── 乙' · 例外放行（B）：键 ∈ 名册 ⇒ rc=0 + 逐条理由 ──
	rootB := ccoBaseT(t)
	ccoWriteT(t, rootB, scriptRetireTruthRel,
		`{"retire_ledger":[],"retire_exceptions":[{"object":"thing.py","category":"夹具例外类","reason":"夹具例外理由","keep_cost":"夹具代价","review_date":"2026-10-07"}]}`)
	rcB, outB, _ := ccoRunT(t, "--root", rootB)
	if rcB != 0 {
		t.Fatalf("乙'·名册放行 应 rc=0，得 rc=%d out=%q", rcB, outB)
	}
	if !strings.Contains(outB, "例外放行 thing.py（键 thing ∈ 名册 · 夹具例外类 · 夹具例外理由）") {
		t.Fatalf("乙'·名册放行 未逐条打印理由：out=%q", outB)
	}

	// ── 丙' · 销账放行（C）：键 ∈ 台账 ⇒ rc=0 ──
	rootC := ccoBaseT(t)
	ccoWriteT(t, rootC, scriptRetireTruthRel,
		`{"retire_ledger":[{"object":"thing.py","state":"已登记退场","evidence":"夹具"}],"retire_exceptions":[]}`)
	rcC, outC, _ := ccoRunT(t, "--root", rootC)
	if rcC != 0 {
		t.Fatalf("丙'·台账销账 应 rc=0，得 rc=%d out=%q", rcC, outC)
	}
	if strings.Contains(outC, "未接 inproc 标准路由") {
		t.Fatalf("丙'·台账销账 仍报未接标准路由：out=%q", outC)
	}

	// 成对（✗ 禁恒绿）：红(`甲`) ↔ 绿(`乙'`/`丙'`) 必须不同码。
	if rcA == rcB || rcA == rcC {
		t.Fatalf("成对断言失败：红 rc=%d 与绿 rc=%d/%d 同码 ⇒ 无鉴别力", rcA, rcB, rcC)
	}

	// ── 缺真源 ⇒ 不给结论（rc=2）──
	rootMiss := t.TempDir()
	ccoWriteT(t, rootMiss, "thing.py", "print(1)\n")
	rcMiss, _, errMiss := ccoRunT(t, "--root", rootMiss)
	if rcMiss != 2 || !strings.Contains(errMiss, "读不到退场真源") {
		t.Fatalf("缺真源 应 rc=2 且报「读不到退场真源」，得 rc=%d err=%q", rcMiss, errMiss)
	}
}

// TestCliOwnershipNegativeControl —— ★ 负控（喂空表 = 哑化 inproc 读取支）：同一候选补表⇒绿 / 空表⇒红，成对。
//
// 若判据体不真读传入的 inproc 表（例如改回硬编码读真表、或恒绿），本用例必 FAIL（`t.Fatalf` 打 件:行）。
func TestCliOwnershipNegativeControl(t *testing.T) {
	root := ccoBaseT(t)

	// 补键入表 ⇒ 必绿（标准路由）
	resIn := ccoJudge(root, map[string]bool{"thing": true}, map[string]bool{}, map[string]scriptRetireExc{})
	if resIn.Rc != 0 || resIn.Green != 1 {
		t.Fatalf("补键入表 应 rc=0 且绿 1，得 rc=%d green=%d ⇒ inproc 表未生效", resIn.Rc, resIn.Green)
	}

	// 喂空表（哑化 inproc 读取支）⇒ 必红 + 逐字点名
	resEmpty := ccoJudge(root, map[string]bool{}, map[string]bool{}, map[string]scriptRetireExc{})
	if resEmpty.Rc != 1 {
		t.Fatalf("喂空表 应 rc=1（真读 inproc 表则候选必红），得 rc=%d red=%v", resEmpty.Rc, resEmpty.Red)
	}
	want := "thing.py 未接 inproc 标准路由（键 thing ∉ 判据臂键表 ∧ ∉ 名册 ∧ ∉ 台账）"
	if !ccoHas(resEmpty.Red, want) {
		t.Fatalf("喂空表 未逐字点名，want=%q got=%v", want, resEmpty.Red)
	}
	// 成对：同候选两态不同码（空表 ⇒ 红 ≠ 补表 ⇒ 绿）—— 正是「读不读那张表」的鉴别力。
	if resEmpty.Rc == resIn.Rc {
		t.Fatalf("成对断言失败：空表 rc=%d 与补表 rc=%d 同码 ⇒ 判据不读 inproc 表", resEmpty.Rc, resIn.Rc)
	}
}

// TestCliOwnershipSelfTest —— 合成自检（仓外临时树）⇒ rc=0。
func TestCliOwnershipSelfTest(t *testing.T) {
	rc, out, errs := ccoRunT(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 rc=%d out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "0 格不过") {
		t.Fatalf("自检应报「0 格不过」：out=%q", out)
	}
}

// TestCliOwnershipGoClass —— ★ 类 B（Go 真身件 · 缺口 `GAP-20261012-16` 的 Go 真身面）两态：
// 命令树 `run:` 链可达 ⇒ 绿；**无人调用**的真身件 ⇒ 红 ＋ 逐字点名；哑化命令树那行 ⇒ 两件都红
// （证判据真读命令树 · ✗ 不是恒绿）；且**类 A（闭集 py/sh）判面一字未动**（原候选面仍是 py/sh）。
func TestCliOwnershipGoClass(t *testing.T) {
	root := t.TempDir()
	ccoWriteT(t, root, "core/cmd/zerg/main.go",
		"package main\n\nvar commands = []*command{\n\t{path: []string{\"foo\"}, run: cmdFoo},\n}\n")
	ccoWriteT(t, root, "core/cmd/zerg/family_a_native.go",
		"package main\n\nfunc cmdFoo() int { return aNative() }\n\nfunc aNative() int { return 0 }\n")
	ccoWriteT(t, root, "core/cmd/zerg/family_b_native.go", "package main\n\nfunc bNative() int { return 0 }\n")

	files, natives, ok := ccoScanGo(root)
	if !ok || len(natives) != 2 {
		t.Fatalf("类 B 候选面应恰 2 件（`*_native.go`；✗ 不吃 main.go），得 ok=%v natives=%v", ok, natives)
	}
	wired, red := ccoJudgeGo(files, natives)
	want := "core/cmd/zerg/family_b_native.go 未接 inproc 标准路由（Go 真身件 · 从命令树 `run:` 符号沿同包顶层符号调用链**未引用**它）"
	if wired != 1 || len(red) != 1 || !ccoHas(red, want) {
		t.Fatalf("类 B 判决不符：已接=%d 未接=%v（want 1 / 点名 %q）", wired, red, want)
	}
	res := ccoJudge(root, map[string]bool{}, map[string]bool{}, map[string]scriptRetireExc{})
	if res.Rc != 1 || res.GoTotal != 2 || res.GoWired != 1 {
		t.Fatalf("经 ccoJudge 应 rc=1 且 GoTotal=2/GoWired=1，得 rc=%d %d/%d", res.Rc, res.GoTotal, res.GoWired)
	}
	// 类 A 判面一字未动：同根加一件闭集 py ⇒ 候选面仍只收 py/sh。
	ccoWriteT(t, root, "thing.py", "print(1)\n")
	if resA := ccoJudge(root, map[string]bool{}, map[string]bool{}, map[string]scriptRetireExc{}); resA.Total != 1 {
		t.Fatalf("类 A 候选面应仍只收 py/sh（1 件），得 %d ⇒ 原判面被改动了", resA.Total)
	}
	// 哑化（拿掉命令树那行 `run:`）⇒ 两件都红 ⇒ 与上态成对（✗ 禁恒绿）。
	ccoWriteT(t, root, "core/cmd/zerg/main.go", "package main\n\nvar commands = []*command{}\n")
	files2, natives2, _ := ccoScanGo(root)
	wired2, red2 := ccoJudgeGo(files2, natives2)
	if wired2 != 0 || len(red2) != 2 {
		t.Fatalf("哑化命令树应 已接 0 / 未接 2，得 %d / %d", wired2, len(red2))
	}
	if wired == wired2 || len(red) == len(red2) {
		t.Fatalf("成对断言失败：两态同读数（已接 %d/%d · 未接 %d/%d）⇒ 判据不真读命令树", wired, wired2, len(red), len(red2))
	}
}

// TestCliOwnershipUsage —— 用法档：未知旗标 / 两档互斥 / `-h` ⇒ 退码与文案。
func TestCliOwnershipUsage(t *testing.T) {
	if rc, _, _ := ccoRunT(t, "--nope"); rc != 2 {
		t.Fatalf("未知旗标 应 rc=2，得 rc=%d", rc)
	}
	if rc, _, _ := ccoRunT(t, "--check", "--self-test"); rc != 2 {
		t.Fatalf("`--check` 与 `--self-test` 同给 应 rc=2，得 rc=%d", rc)
	}
	if rc, out, _ := ccoRunT(t, "-h"); rc != 0 || !strings.Contains(out, ccoJudgeKey) {
		t.Fatalf("`-h` 应 rc=0 且带件名 %s，得 rc=%d out=%q", ccoJudgeKey, rc, out)
	}
}
