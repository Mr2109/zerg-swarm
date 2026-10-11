// family_gate_check_selfcases_test.go —— 缺口 `GAP-20260927-240`：`zerg gate check` 的壳级布尔旗标
// `--list-selfcases`（**只读列举**该判据件的自测格清单 / 断言表）的**成对判据**（正控 + 负控），
// 走**真 `gateCheck` 入口**（✗ 不另抄一份逻辑）。
//
// 三面：
//
//	① 正控：`gate check <判据名> --list-selfcases` ⇒ rc=0 ＋ 出格名（圈号起头 · 去重保序）；
//	        且**只**收串字面量里的圈号 —— 注释里的圈号不收（取词窗口 = 串）。
//	② 机器面：`--json` ⇒ 键集与全表干跑同源（count/resolved/dry_run/entries · ✗ 不自造字段表）。
//	③ 消费面（负控靶）：`--list-selfcases` 由**壳**消费 ⇒ **不 exec** 判据件（marker 不落盘）。
//
// ★ 负控：把 `family_gate_check.go` 换回**改前态**（无 `--list-selfcases` 分支 · `-overlay`）⇒
// ①②③ 全红（壳不认该档 ⇒ 既不出格名、也把旗标透传给子件真跑）。
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gcsDemo —— 合成判据件正文：三类形态的**串字面量**格名 ＋ 一个**注释里的圈号**（不该被收）。
// 件自身跑起来恒退 0（负控靶要的是「有没有被 exec」，不是它的退码）。
const gcsDemo = "#!/usr/bin/env python3\n" +
	"# 注释里的圈号 ⑨ 与 ⑩ 不该被收（不在串里）\n" +
	"import os\n" +
	"open(os.environ.get('GCS_RAN', '/dev/null'), 'w').write('1')\n" +
	"SELF_TESTS = [\n" +
	"    (\"① 正控：解析器形态\", \"x\"),\n" +
	"    ('② 负控：同一形态落真命令位', \"y\"),\n" +
	"]\n" +
	"def main():\n" +
	"    print(\"③ 空转：扫描面 0 件 ⇒ rc=2\")\n" +
	"    return 0\n"

// gcsMakeRoot —— 造一个最小合成仓根：`<root>/scripts/gates/gcs-demo.py` 在盘（名录按目录派生）。
func gcsMakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gcs-demo.py"), []byte(gcsDemo), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// gcsRun —— 跑一次真 gateCheck 入口，回（rc, stdout, stderr）。
func gcsRun(t *testing.T, root string, argv []string, js bool) (int, string, string) {
	t.Helper()
	inv := &invocation{orig: argv, jsonGiven: js}
	var ob, eb bytes.Buffer
	rc := gateCheck(inv, &ob, &eb, root)
	return rc, ob.String(), eb.String()
}

// ① 正控：给 `--list-selfcases` ⇒ 出格名（圈号起头 · 去重保序）；注释里的圈号**不**收。
func TestGateCheckSelfcasesList(t *testing.T) {
	root := gcsMakeRoot(t)
	marker := filepath.Join(t.TempDir(), "ran.txt")
	t.Setenv("GCS_RAN", marker)

	rc, out, errs := gcsRun(t, root, []string{"gate", "check", "gcs-demo", "--list-selfcases"}, false)
	if rc != exitOK {
		t.Fatalf("`gate check gcs-demo --list-selfcases` rc=%d want %d（stdout=%q stderr=%q）",
			rc, exitOK, out, errs)
	}
	if !strings.Contains(out, "自测格清单") {
		t.Fatalf("缺只读列举表头；stdout=%q", out)
	}
	for _, want := range []string{"① 正控：解析器形态", "② 负控：同一形态落真命令位", "③ 空转：扫描面 0 件 ⇒ rc=2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("格名 %q 未列出；stdout=%q", want, out)
		}
	}
	if strings.Contains(out, "注释里的圈号") {
		t.Fatalf("注释（非串字面量）里的圈号被误收；stdout=%q", out)
	}
	// ③ 消费面：壳消费该旗标 ⇒ 判据件**不该**被 exec。
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatalf("`--list-selfcases` 档**不该 exec**判据件，但 marker 在盘（旗标被透传给子件真跑）")
	}
}

// ② 机器面：`--json` ⇒ 键集与全表干跑同源（count/resolved/dry_run/entries）。
func TestGateCheckSelfcasesJSON(t *testing.T) {
	root := gcsMakeRoot(t)
	t.Setenv("GCS_RAN", filepath.Join(t.TempDir(), "ran.txt"))

	rc, out, errs := gcsRun(t, root, []string{"gate", "check", "gcs-demo", "--list-selfcases", "--json"}, true)
	if rc != exitOK {
		t.Fatalf("`--json` 档 rc=%d want %d（stderr=%q）", rc, exitOK, errs)
	}
	var got struct {
		Count    int      `json:"count"`
		Resolved int      `json:"resolved"`
		DryRun   bool     `json:"dry_run"`
		Entries  []string `json:"entries"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("机器面不是合法 JSON：%v；stdout=%q", err, out)
	}
	if got.Count != 3 || got.Resolved != 3 || len(got.Entries) != 3 {
		t.Fatalf("格数 count=%d resolved=%d len(entries)=%d want 3/3/3", got.Count, got.Resolved, len(got.Entries))
	}
	// 键集逐字与命令树登记字段同集（✗ 不自造第二套字段表）。
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		t.Fatalf("机器面解析失败：%v", err)
	}
	want := map[string]bool{"count": true, "resolved": true, "dry_run": true, "entries": true}
	if len(raw) != len(want) {
		t.Fatalf("键集 = %v want %v", raw, want)
	}
	for k := range raw {
		if !want[k] {
			t.Fatalf("多出字段 %q（✗ 不许自造第二套字段表）", k)
		}
	}
}

// ③ 取词窗口：注释与文档串外的圈号不收；只收串字面量内的。
func TestGateCheckSelfcaseLabelsWindow(t *testing.T) {
	src := "// ① 注释里的不是格名\n" +
		"const usage = \"zerg foo ① 用法串里的是格名\"\n" +
		"var x = `② 反引号原串里的也是格名`\n" +
		"newline := \"③ 串尾带转义\\n结束\"\n"
	got := gateCheckSelfcaseLabels(src)
	want := []string{"① 用法串里的是格名", "② 反引号原串里的也是格名", "③ 串尾带转义"}
	if len(got) != len(want) {
		t.Fatalf("抽出 %d 条 %q want %d 条 %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 = %q want %q（全量 %q）", i, got[i], want[i], got)
		}
	}
}

var _ io.Writer = (*bytes.Buffer)(nil)
