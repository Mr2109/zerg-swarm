// family_gate_baseline_test.go —— `zerg gate baseline show` 的判据（Q-125）。
//
// 为什么落进程内 `RunForTest`（不是真二进制那一层）：本批**不重建 `bin/`** ⇒ 新面的棘轮
// 必须读**当前源码**。
//
// 钉住八格（全走**仓外夹具** —— 不碰真基线件、不跑任何门）：
//
//	① 全齐档（件齐 + 台账对得上）⇒ rc=0 · 表格四行 · 差列 +0；
//	② 无台账档（件在、台账不在）⇒ rc=1 · 「无记录」· 「判红（rc=1）」；
//	③ 无件档（连基线件都不在）⇒ rc=2 · 「判不了」· 「不给结论（rc=2）」
//	   —— ★ 成对负控：棘轮 `--view` **同一档**是 rc=0（`baseline_ratchet_test.go:149` 钉住的
//	   「判不了 ≠ 绿 但 rc 不翻红」口子）⇒ 本命令**有意不继承**那个口子。两态在同一夹具上读，
//	   差只可能来自「本命令的 n/a 闸」；
//	④ 点名档：只出一条 · 点错名 ⇒ rc=2 + 可用清单（= 条件表现读，四条）；
//	⑤ `--json`：**单包封可解析**（判语落 stderr）· 八键闭集 · `--json name,value` 是投影 ·
//	   字段表外 ⇒ rc=2 + 合法字段行；
//	⑥ **只读**：跑前/跑后整个夹具逐件 sha256 相同（不写不删不改）：★ 与 `--anchor` 成对 ——
//	   本命令**没有**写侧；
//	⑦ `--root` 裸给（给过旗标、没给值）⇒ rc=2（✗ 不静默回落本仓）；
//	⑧ 「差」的语义：件值比台账高 3 ⇒ 差 `+3` + 「与台账不符」+ rc=1（不是只印判词）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// gbsCond —— 夹具里四个条件（与 `baseline_ratchet.go` 的条件表同名同键；本条只造文件与台账）。
type gbsCond struct{ name, file, key string }

var gbsConds = []gbsCond{
	{"gate-coverage.baseline", "scripts/gates/gate-coverage.config.json", "baseline"},
	{"cli-contract.blanks", "scripts/gates/cli-contract-baseline.json", "blanks"},
	{"doc-cmds.devdocs_report_hits", "scripts/gates/doc-cmds-baseline.json", "devdocs_report_hits"},
	{"i18n.allowed_count", "ui/scripts/i18n-baseline.json", "allowed_count"},
}

// gbsFixture —— 造一棵「有仓根标记 + 四件基线（可选）+ 可选台账」的夹具。
//
// files=false ⇒ 四件基线件一件都不造（无件档）；ledger=false ⇒ 不造台账件。
// bump≠0 ⇒ 第一个条件的件值比台账高 bump（造「差 ≠ 0」那一格）。
func gbsFixture(t *testing.T, files, ledger bool, bump int) string {
	t.Helper()
	root := syntheticRepo(t, "")
	mustWrite(t, filepath.Join(root, "core", "go.mod"), "")
	vals := map[string]int{"gate-coverage.baseline": 55, "cli-contract.blanks": 0,
		"doc-cmds.devdocs_report_hits": 79, "i18n.allowed_count": 7}
	lines := []string{}
	for i, c := range gbsConds {
		v := vals[c.name]
		if files {
			onDisk := v
			if i == 0 {
				onDisk = v + bump
			}
			mustWrite(t, filepath.Join(root, c.file),
				fmt.Sprintf("{\n  \"id\": \"fx\",\n  \"%s\": %d\n}\n", c.key, onDisk))
		}
		if ledger {
			lines = append(lines, fmt.Sprintf(
				`{"seq": %d, "ts": "2026-10-01T00:00:00+08:00", "kind": "write", "name": %q, "gate": "门①", "file": %q, "old": %d, "new": %d, "reason": "夹具", "by": "fx <fx@example.invalid>"}`,
				i+1, c.name, c.file, v, v))
		}
	}
	if ledger {
		mustWrite(t, filepath.Join(root, "scripts", "gates", "baseline-audit.jsonl"), strings.Join(lines, "\n")+"\n")
	}
	return root
}

// gbsRun —— 跑一条 argv，取 (rc, stdout, stderr)。
func gbsRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gbsTreeSHA —— 夹具里每个件 → sha256（只读负控用）。
func gbsTreeSHA(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, p)
		m[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func gbsSameTree(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestGateBaselineShow_四态与只读(t *testing.T) {
	// ① 全齐档
	root := gbsFixture(t, true, true, 0)
	before := gbsTreeSHA(t, root)
	rc, out, _ := gbsRun(t, "gate", "baseline", "show", "--root", root)
	if rc != 0 {
		t.Fatalf("① 全齐档该 rc=0，得 %d\n%s", rc, out)
	}
	for _, want := range []string{"+0", "合计 4 条", "值齐 4", "⇒ 绿（rc=0）"} {
		if !strings.Contains(out, want) {
			t.Fatalf("① 人面缺 %q：\n%s", want, out)
		}
	}
	// ⑥ 只读负控：一个字节都不许动
	if after := gbsTreeSHA(t, root); !gbsSameTree(before, after) {
		t.Fatal("⑥ 只读档改了夹具（本命令没有写侧）")
	}

	// ② 无台账档
	root2 := gbsFixture(t, true, false, 0)
	rc, out, _ = gbsRun(t, "gate", "baseline", "show", "--root", root2)
	if rc != 1 {
		t.Fatalf("② 无台账档该 rc=1，得 %d\n%s", rc, out)
	}
	for _, want := range []string{"无记录", "判红（rc=1）", "**件还不存在**"} {
		if !strings.Contains(out, want) {
			t.Fatalf("② 人面缺 %q：\n%s", want, out)
		}
	}

	// ③ 无件档 + 与棘轮 --view 的成对负控
	root3 := gbsFixture(t, false, false, 0)
	rc, out, _ = gbsRun(t, "gate", "baseline", "show", "--root", root3)
	if rc != 2 {
		t.Fatalf("③ 无件档该 rc=2（判不了 ⇒ 不给结论），得 %d\n%s", rc, out)
	}
	for _, want := range []string{"判不了", "不给结论（rc=2）", "「读不到」不是绿"} {
		if !strings.Contains(out, want) {
			t.Fatalf("③ 人面缺 %q：\n%s", want, out)
		}
	}
	// ★ 成对负控：**同一个夹具**上走棘轮自己的 `--view` ⇒ rc 回落 0（那个口子还在，本命令不继承）
	if rc2, _, _ := gbsRun(t, "gate", "check", "baseline-ratchet", "--view", "--root", root3); rc2 != 0 {
		t.Fatalf("③ 成对负控：棘轮 --view 现状该是 rc=0（口子未改），得 %d", rc2)
	}

	// ⑧ 差 ≠ 0：件值比台账高 3
	root4 := gbsFixture(t, true, true, 3)
	rc, out, _ = gbsRun(t, "gate", "baseline", "show", "--root", root4)
	if rc != 1 {
		t.Fatalf("⑧ 差≠0 该 rc=1，得 %d\n%s", rc, out)
	}
	for _, want := range []string{"+3", "与台账不符", "1 条与台账不符"} {
		if !strings.Contains(out, want) {
			t.Fatalf("⑧ 人面缺 %q：\n%s", want, out)
		}
	}
}

func TestGateBaselineShow_点名与JSON(t *testing.T) {
	root := gbsFixture(t, true, true, 0)
	// ④ 点名档：只出一条
	rc, out, _ := gbsRun(t, "gate", "baseline", "show", "i18n.allowed_count", "--root", root)
	if rc != 0 || !strings.Contains(out, "合计 1 条") || strings.Contains(out, "gate-coverage.baseline") {
		t.Fatalf("④ 点名档该只出一条 rc=0，得 rc=%d\n%s", rc, out)
	}
	// ④ 点错名 ⇒ 2 + 可用清单（四条全列）
	rc, _, errb := gbsRun(t, "gate", "baseline", "show", "no.such.cond", "--root", root)
	if rc != 2 {
		t.Fatalf("④ 点错名该 rc=2，得 %d", rc)
	}
	for _, c := range gbsConds {
		if !strings.Contains(errb, c.name) {
			t.Fatalf("④ 可用清单缺 %q：\n%s", c.name, errb)
		}
	}
	// ⑤ --json：单包封（stdout 只有 JSON，判语在 stderr）
	rc, out, errb = gbsRun(t, "gate", "baseline", "show", "--json", "--root", root)
	if rc != 0 {
		t.Fatalf("⑤ --json 该 rc=0，得 %d", rc)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("⑤ --json 的 stdout 不是**单个**可解析包封（判语漏进了 stdout）：%v\n%s", err, out)
	}
	if !strings.Contains(errb, "⇒ 绿（rc=0）") {
		t.Fatalf("⑤ 判语该落 stderr：\n%s", errb)
	}
	for _, k := range []string{"mode", "root", "ledger", "ledger_entries", "rc", "counts", "rows", "ledger_hard_errors", "ledger_soft_notes"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("⑤ 包封缺键 %q：%s", k, out)
		}
	}
	rows, _ := doc["rows"].([]any)
	if len(rows) != 4 {
		t.Fatalf("⑤ rows 该 4 条，得 %d", len(rows))
	}
	got := []string{}
	for k := range rows[0].(map[string]any) {
		got = append(got, k)
	}
	sort.Strings(got)
	want := []string{"delta", "file", "gate", "ledger_value", "name", "value", "verdict", "verdict_text"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("⑤ rows 键集该是八格闭集：得 %v", got)
	}
	// ⑤ 投影
	rc, out, _ = gbsRun(t, "gate", "baseline", "show", "--json", "name,value", "--root", root)
	if rc != 0 {
		t.Fatalf("⑤ 投影该 rc=0，得 %d", rc)
	}
	var proj map[string]any
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatal(err)
	}
	pr := proj["rows"].([]any)[0].(map[string]any)
	if len(pr) != 2 || pr["name"] == nil || pr["value"] == nil {
		t.Fatalf("⑤ 投影该只剩两格：%v", pr)
	}
	// ⑤ 字段表外 ⇒ 2 + 合法字段行
	rc, _, errb = gbsRun(t, "gate", "baseline", "show", "--json", "nosuch", "--root", root)
	if rc != 2 || !strings.Contains(errb, "name,gate,file,value,ledger_value,delta,verdict,verdict_text") {
		t.Fatalf("⑤ 字段表外该 rc=2 + 列合法字段，得 rc=%d\n%s", rc, errb)
	}
}

func TestGateBaselineShow_用法面(t *testing.T) {
	root := gbsFixture(t, true, true, 0)
	// ⑦ --root 裸给 ⇒ 2（✗ 不静默回落本仓）
	if rc, _, errb := gbsRun(t, "gate", "baseline", "show", "--root"); rc != 2 || !strings.Contains(errb, "没给值") {
		t.Fatalf("⑦ --root 裸给该 rc=2 + 点名缺值，得 rc=%d\n%s", rc, errb)
	}
	// ⑦ 两个位置参数 ⇒ 2
	if rc, _, errb := gbsRun(t, "gate", "baseline", "show", "a", "b", "--root", root); rc != 2 || !strings.Contains(errb, "只收**一个**条件名") {
		t.Fatalf("⑦ 两个位置参数该 rc=2，得 rc=%d\n%s", rc, errb)
	}
	// ⑦ 中间节点本身仍是「不是可执行动作」（rc=2 · 与登记前同判词）
	if rc, _, errb := gbsRun(t, "gate", "baseline"); rc != 2 || !strings.Contains(errb, "族名/中间节点") {
		t.Fatalf("⑦ 中间节点裸敲该 rc=2 + 节点判词，得 rc=%d\n%s", rc, errb)
	}
}
