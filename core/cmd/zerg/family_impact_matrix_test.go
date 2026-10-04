// family_impact_matrix_test.go —— 反查腿（`GAP-20260927-366` 残留）的**成对正负控**。
//
// 落点：**内部测试包 `package main`**（要直接调未导出的 `impactMatrixCellsOf` 一族）。
// 用 `t.TempDir()` 造一棵**仓外**最简夹具树（只放 `core/cmd/zerg/testdata/cli-matrix.json`），
// 不碰真仓、不跑子进程。负控逐条钉死「读不到 ≠ 没有格」「空表 ≠ 没有格」「幽灵不误报」。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// impactMatrixFixture 在 root 下造 `core/cmd/zerg/testdata/cli-matrix.json`。
func impactMatrixFixture(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "core", "cmd", "zerg", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建夹具目录：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cli-matrix.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具矩阵：%v", err)
	}
}

const impactMatrixFixtureBody = `{
 "id": "cli-contract.v1",
 "exemptions": [],
 "cases": [
  {"id":"gap export#只钉rc","command":"gap export","argv":["gap","export","--dry-run"],"want_rc":2,"want_stdout_bytes":0,"why":"w1"},
  {"id":"gap export#未知名",  "command":"gap export","argv":["gap","export","--zz"],       "want_rc":2,"want_stdout_bytes":0,"why":"w2"},
  {"id":"version#未知旗标",   "command":"version",   "argv":["version","--zz"],           "want_rc":2,"want_stdout_bytes":0,"why":"w3"}
 ]
}`

// TestImpactMatrixCellsIndex —— 正控：两张反查表（command / argv token）键值与去重定序。
func TestImpactMatrixCellsIndex(t *testing.T) {
	root := t.TempDir()
	impactMatrixFixture(t, root, impactMatrixFixtureBody)
	cat := impactMatrixCellsOf(root)
	if cat.Status != "取值" {
		t.Fatalf("本应是取值态，得 status=%q reason=%q", cat.Status, cat.Reason)
	}
	if len(cat.Cells) != 3 {
		t.Fatalf("格数 = %d（要 3）", len(cat.Cells))
	}
	if got := strings.Join(impactMatrixCellsForCommand(cat, "gap export"), ","); got != "gap export#只钉rc,gap export#未知名" {
		t.Errorf("by_command[gap export] = %q", got)
	}
	if got := strings.Join(impactMatrixCellsForArgvToken(cat, "version"), ","); got != "version#未知旗标" {
		t.Errorf("by_token[version] = %q", got)
	}
	// `--zz` 跨两条命令（gap export 与 version）⇒ 逐 token 反查必须**收拢多处命中**。
	if got := strings.Join(impactMatrixCellsForArgvToken(cat, "--zz"), ","); got != "gap export#未知名,version#未知旗标" {
		t.Errorf("by_token[--zz] = %q（要跨命令收拢两格）", got)
	}
	if got := impactMatrixCellsForCommand(cat, "nope"); len(got) != 0 || got == nil {
		t.Errorf("未知命令行应回空表（非 nil），得 %#v", got)
	}
	if got := impactMatrixCellsForArgvToken(cat, "gap"); strings.Join(got, ",") != "gap export#只钉rc,gap export#未知名" {
		t.Errorf("by_token[gap] = %q", got)
	}
	if c, ok := impactMatrixCellOf(cat, "version#未知旗标"); !ok || c.Command != "version" {
		t.Errorf("按 id 取回那一格失败：%#v ok=%v", c, ok)
	}
	if _, ok := impactMatrixCellOf(cat, "幽灵"); ok {
		t.Errorf("幽灵 id 不该被取回")
	}
}

// TestImpactMatrixCellsUnreadable —— 负控①：件不在盘 ⇒ `读不到`（**不许**读成「没有格」）。
func TestImpactMatrixCellsUnreadable(t *testing.T) {
	root := t.TempDir() // 一棵空树：没有 testdata/cli-matrix.json
	cat := impactMatrixCellsOf(root)
	if cat.Status != "读不到" {
		t.Fatalf("缺件应读不到，得 status=%q", cat.Status)
	}
	if len(cat.Cells) != 0 {
		t.Fatalf("读不到时不该有格，得 %d", len(cat.Cells))
	}
	if got := impactMatrixCellsForCommand(cat, "gap export"); len(got) != 0 {
		t.Errorf("读不到态反查应回空表，得 %#v", got)
	}
	if !strings.Contains(cat.Reason, "读不到") {
		t.Errorf("理由应含「读不到」：%q", cat.Reason)
	}
}

// TestImpactMatrixCellsEmptyCases —— 负控②：`cases[]` 空表 ⇒ `读不到`（空表不许当没有格）。
func TestImpactMatrixCellsEmptyCases(t *testing.T) {
	root := t.TempDir()
	impactMatrixFixture(t, root, `{"id":"cli-contract.v1","exemptions":[],"cases":[]}`)
	cat := impactMatrixCellsOf(root)
	if cat.Status != "读不到" {
		t.Fatalf("空表应读不到，得 status=%q", cat.Status)
	}
	if !strings.Contains(cat.Reason, "空表") {
		t.Errorf("理由应含「空表」：%q", cat.Reason)
	}
}

// TestImpactMatrixCellsSignalDeterministic —— 机读行**两跑逐字同**（定序 · 无时钟无耗时）。
func TestImpactMatrixCellsSignalDeterministic(t *testing.T) {
	root := t.TempDir()
	impactMatrixFixture(t, root, impactMatrixFixtureBody)
	a := impactMatrixCellsSignal(impactMatrixCellsOf(root))
	b := impactMatrixCellsSignal(impactMatrixCellsOf(root))
	if a != b {
		t.Errorf("机读行两跑不同（定序被打破）：\n a=%s\n b=%s", a, b)
	}
	for _, want := range []string{`"signal":"impact.matrix"`, `"status":"取值"`, `"cells":3`, `"by_command"`, `"by_token"`} {
		if !strings.Contains(a, want) {
			t.Errorf("机读行缺 %s：%s", want, a)
		}
	}
}

// TestImpactMatrixCellCard —— 卡片四字段齐（`what`/`why`/`how`/`red`）且 `why` 落在六选一里。
func TestImpactMatrixCellCard(t *testing.T) {
	c := cliMatrixCase{ID: "gap export#只钉rc", Command: "gap export",
		Argv: []string{"gap", "export", "--dry-run"}, WantRC: 2, WantStdoutBytes: 0}
	row := impactMatrixCellCard(c)
	for _, k := range []string{"what", "why", "how", "red"} {
		if strings.TrimSpace(row[k]) == "" {
			t.Errorf("卡片四字段缺 %q：%#v", k, row)
		}
	}
	ok := false
	for _, w := range impactWhySix {
		if row["why"] == w {
			ok = true
			break
		}
	}
	if !ok {
		t.Errorf("why = %q 不在六选一里（%v）", row["why"], impactWhySix)
	}
	if !strings.Contains(row["what"], c.ID) {
		t.Errorf("what 应带格 id：%q", row["what"])
	}
	if !strings.Contains(impactMatrixCellName(c), c.ID) {
		t.Errorf("点名行应带格 id：%q", impactMatrixCellName(c))
	}
}
