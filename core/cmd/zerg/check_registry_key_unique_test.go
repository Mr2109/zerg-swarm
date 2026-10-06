package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 测试助手（前缀 rkuTest…，✗ 不用 TestMain / init，避与同包他件撞名）────────────

func rkuTestFx(t *testing.T, files map[string]string) string {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, "core", "internal", "contract"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(d, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func rkuTestRun(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkRegistryKeyUniqueGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ── 正控 / 负控（成对：哑化主件后本组断言必带行号 FAIL）────────────────────────

func TestRkuDupStringKeyRed(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"entries":[{"id":"a"},{"id":"b"},{"id":"a"}]}`,
	})
	rc, out, errs := rkuTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("重复键应 rc=1，实得 rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(out, "  ✗ scripts/gates/reg.json[entries] 键 id=a 出现 2 次（要求恰好 1 次）") {
		t.Fatalf("应点名重复键 a，stdout=%q", out)
	}
}

func TestRkuUniqueKeyGreen(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"entries":[{"id":"a"},{"id":"b"}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("键唯一应 rc=0，实得 %d", rc)
	}
	if !strings.Contains(out, "重复键 = 0 条") {
		t.Fatalf("应报 0 条，stdout=%q", out)
	}
}

func TestRkuNoKeySkipsNotRed(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"criteria":[{"foo":1},{"bar":2}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("无键面不该判红，实得 rc=%d", rc)
	}
	if !strings.Contains(out, "跳过（无键面 · 非重复）：1 列") ||
		!strings.Contains(out, "    · scripts/gates/reg.json[criteria]") {
		t.Fatalf("应点名跳过列，stdout=%q", out)
	}
}

func TestRkuIntKeyValue(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"core/internal/contract/reg.json": `{"entries":[{"id":1},{"id":2},{"id":1}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("int 重复键应 rc=1，实得 %d", rc)
	}
	if !strings.Contains(out, "键 id=1 出现 2 次") {
		t.Fatalf("int 键值展示应为 1，stdout=%q", out)
	}
}

// ★ 本用例钉「顶层键序 = 件内原始序」（Go 若用 map[string]any 迭代必随机 ⇒ 本用例随机会红）。
func TestRkuTopKeyOrderPreserved(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"zz":[{"foo":1}],"aa":[{"bar":2}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("跳过档应 rc=0，实得 %d", rc)
	}
	iz := strings.Index(out, "reg.json[zz]")
	ia := strings.Index(out, "reg.json[aa]")
	if iz < 0 || ia < 0 || iz > ia {
		t.Fatalf("顶层键序应 zz 先于 aa（件内序），stdout=%q", out)
	}
}

// ★ 本用例钉「多目录扫描序 + 明细追加序 = SCAN_DIRS 序」。
func TestRkuScanDirOrder(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/a.json":          `{"zz":[{"id":"1"},{"id":"1"}]}`,
		"core/internal/contract/b.json": `{"k":[{"key":"m"},{"key":"m"}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("应 rc=1，实得 %d", rc)
	}
	ig := strings.Index(out, "scripts/gates/a.json[zz]")
	ic := strings.Index(out, "core/internal/contract/b.json[k]")
	if ig < 0 || ic < 0 || ig > ic {
		t.Fatalf("脚本目录明细应先于合约目录，stdout=%q", out)
	}
}

func TestRkuBadJSONBlocked(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"core/internal/contract/reg.json": "{ broken",
	})
	rc, _, errs := rkuTestRun([]string{"--root", d})
	if rc != 2 {
		t.Fatalf("非法 JSON 应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(errs, "BLOCKED —— 登记件读不出/非法 JSON") ||
		!strings.Contains(errs, "    core/internal/contract/reg.json ← ") {
		t.Fatalf("应逐件点名非法件，stderr=%q", errs)
	}
}

// ★ 本用例钉「非法 JSON 正文 = py `str(JSONDecodeError)` 逐字（含 line/column/char 三数）」。
// 哑化 `rkuPyJSONErrStr` / `rkuLoadJSON` 里的 py 口径扫描器（回落到 Go 编码器原文）⇒ 本用例必 FAIL 带行号。
func TestRkuBadJSONPythonMessage(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"core/internal/contract/z.json": "{ not json",
	})
	rc, _, errs := rkuTestRun([]string{"--root", d})
	if rc != 2 {
		t.Fatalf("非法 JSON 应 rc=2，实得 %d", rc)
	}
	want := "    core/internal/contract/z.json ← 不是合法 JSON：Expecting property name enclosed in double quotes: line 1 column 3 (char 2)\n"
	if !strings.Contains(errs, want) {
		t.Fatalf("非法 JSON 正文应与 py 逐字：\n want=%q\n got =%q", want, errs)
	}
}

func TestRkuEmptyDomainBlocked(t *testing.T) {
	d := rkuTestFx(t, nil)
	rc, _, errs := rkuTestRun([]string{"--root", d})
	if rc != 2 {
		t.Fatalf("空域应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(errs, "空域：扫面（scripts/gates、core/internal/contract）下 0 个 *.json 登记件") {
		t.Fatalf("应为空域守卫文案，stderr=%q", errs)
	}
}

func TestRkuMissingRootBlocked(t *testing.T) {
	d := filepath.Join(t.TempDir(), "nope")
	rc, _, errs := rkuTestRun([]string{"--root", d})
	if rc != 2 {
		t.Fatalf("扫面根不存在应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(errs, "BLOCKED —— 扫面根不存在（缺 scripts/gates 与 core/internal/contract）⇒ rc=2，不给结论") {
		t.Fatalf("应为根不存在文案，stderr=%q", errs)
	}
}

func TestRkuJSONOutputShape(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"entries":[{"id":"a"},{"id":"a"}],"more":[{"x":1}]}`,
	})
	rc, out, _ := rkuTestRun([]string{"--root", d, "--json"})
	if rc != 1 {
		t.Fatalf("应 rc=1，实得 %d", rc)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
		t.Fatalf("json 档应可解析：%v out=%q", err, out)
	}
	if m["rc"].(float64) != 1 || m["dup_count"].(float64) != 1 {
		t.Fatalf("json 字段应 rc=1/dup_count=1，实得 %v", m)
	}
	dups := m["dups"].([]any)
	d0 := dups[0].(map[string]any)
	if d0["file"] != "scripts/gates/reg.json" || d0["list"] != "entries" || d0["field"] != "id" || d0["value"] != "a" || d0["count"].(float64) != 2 {
		t.Fatalf("dups[0] 形状不符：%v", d0)
	}
	sk := m["skipped_no_key"].([]any)
	if len(sk) != 1 || sk[0].(map[string]any)["list"] != "more" {
		t.Fatalf("skipped_no_key 应含 more：%v", sk)
	}
}

// ★ 空键序列的空清单档：显式置空（py 打 "dups": []，✗ 不能是 null 或缺键）。
func TestRkuJSONEmptyListsNotNull(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"entries":[{"id":"a"}]}`,
	})
	_, out, _ := rkuTestRun([]string{"--root", d, "--json"})
	line := strings.TrimSpace(out)
	if !strings.Contains(line, `"dups": []`) || !strings.Contains(line, `"skipped_no_key": []`) {
		t.Fatalf("空清单必须显式 []，实得 %q", line)
	}
}

func TestRkuSelfTestPass(t *testing.T) {
	rc, out, _ := rkuTestRun([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检应 rc=0，实得 %d，out=%q", rc, out)
	}
	if !strings.Contains(out, "自检 PASS（5 格）") {
		t.Fatalf("自检应 PASS 5 格，out=%q", out)
	}
}

// ★ 空/缺键面默认根档（env ZERG_ROOT 生效；t.Setenv 会在用例后自动还原）。
func TestRkuRootFromEnv(t *testing.T) {
	d := rkuTestFx(t, map[string]string{
		"scripts/gates/reg.json": `{"entries":[{"id":"x"},{"id":"x"}]}`,
	})
	t.Setenv("ZERG_ROOT", d)
	rc, out, _ := rkuTestRun(nil)
	if rc != 1 {
		t.Fatalf("env 根下应 rc=1，实得 %d out=%q", rc, out)
	}
	if !strings.Contains(out, "键 id=x 出现 2 次") {
		t.Fatalf("env 根未生效？out=%q", out)
	}
}

func TestRkuUnknownFlag(t *testing.T) {
	rc, _, errs := rkuTestRun([]string{"--nope"})
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(errs, "error: unrecognized arguments: --nope") {
		t.Fatalf("应报未识别，stderr=%q", errs)
	}
}

func TestRkuHelpExitZero(t *testing.T) {
	rc, out, _ := rkuTestRun([]string{"-h"})
	if rc != 0 {
		t.Fatalf("-h 应 rc=0，实得 %d", rc)
	}
	if !strings.Contains(out, "usage: check-registry-key-unique.py [-h] [--root ROOT] [--json] [--self-test]") {
		t.Fatalf("-h 应印 usage，out=%q", out)
	}
}
