// check_gap_closure_drift_test.go —— 已落件 `check_gap_closure_drift.go` 的成对负控（同批）。
//
// 覆盖面（每条都对到 py 原件的可观测行为）：
//
//	· 自检档 rc=0 且 stdout 含 `自检 PASS（4 格）`；
//	· 正控：真源含「仍缺」且点名件在盘 ⇒ rc=1 ＋ 逐条点名正文；
//	· 负控 A：`--allow <id>` 后 ⇒ rc=0（降噪生效）；
//	· 负控 B：真源缺 ⇒ rc=2 ＋ stderr `BLOCKED`；
//	· 空域（真源只有空行）⇒ rc=2；
//	· `--json` 档逐字节等于 py 口径的 JSON（缺省分隔符 `, `/`: `）；
//	· 未知旗标 ⇒ rc=2 ＋ stderr `unrecognized arguments`；
//	· `-h` ⇒ rc=0 ＋ stdout `optional arguments:`；
//	· 占位 unit / 仓外绝对路径 / `~` 路径 ⇒ 不命中；
//	· 哑化牙：`cgcdUnitsOf` 的占位与分号拆分行为单测（改坏必红）；
//	· 自回收牙：`--self-test` 后 TMPDIR 内 0 残留（删 `RemoveAll` 必红）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cgcdFixture —— 造仓外夹具（真源 ＋ 假仓根），返回 (root, ledger)。
func cgcdFixture(t *testing.T, lines ...string) (string, string) {
	t.Helper()
	td := t.TempDir()
	root := filepath.Join(td, "repo")
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatalf("建夹具根失败：%v", err)
	}
	ledger := filepath.Join(td, "gaps.jsonl")
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(ledger, []byte(body), 0o644); err != nil {
		t.Fatalf("写真源失败：%v", err)
	}
	return root, ledger
}

// cgcdTouch —— 在假仓根内落一枚件。
func cgcdTouch(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("落件失败：%v", err)
	}
}

func cgcdRunMain(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkGapClosureDriftGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckGapClosureDriftGateMain_SelfTest(t *testing.T) {
	rc, out, errb := cgcdRunMain(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d（期望 0）stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "自检 PASS（4 格）") {
		t.Fatalf("自检未 PASS：stdout=%q", out)
	}
}

// TestCheckGapClosureDriftGateMain_SelfTestReclaimsTemp 是「自回收」牙：
// 删掉 cgcdSelfTest 的 `defer os.RemoveAll(td)` ⇒ 本用例必红。
func TestCheckGapClosureDriftGateMain_SelfTestReclaimsTemp(t *testing.T) {
	td := t.TempDir()
	t.Setenv("TMPDIR", td)
	rc, _, errb := cgcdRunMain(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d stderr=%q", rc, errb)
	}
	ents, err := os.ReadDir(td)
	if err != nil {
		t.Fatalf("读 TMPDIR 失败：%v", err)
	}
	if len(ents) != 0 {
		t.Fatalf("自检临时目录未回收：残留 %d 个（行号：cgcdSelfTest 的 defer）", len(ents))
	}
}

func TestCheckGapClosureDriftGateMain_DriftHit(t *testing.T) {
	root, ledger := cgcdFixture(t,
		`{"id": "GAP-T-1", "state": "仍缺", "unit": "core/cmd/zerg/landed.go"}`,
		`{"id": "GAP-T-2", "state": "仍缺", "unit": "core/cmd/zerg/absent.go"}`,
	)
	cgcdTouch(t, root, "core/cmd/zerg/landed.go")
	rc, out, _ := cgcdRunMain(t, "--ledger", ledger, "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d（期望 1）stdout=%q", rc, out)
	}
	if !strings.Contains(out, "  ✗ GAP-T-1：点名件已在盘 ⇒ core/cmd/zerg/landed.go") {
		t.Fatalf("未逐条点名 GAP-T-1：stdout=%q", out)
	}
	if strings.Contains(out, "GAP-T-2") {
		t.Fatalf("absence 件误命中：stdout=%q", out)
	}
	if !strings.Contains(out, "真源 "+ledger+" · 读入 2 行 · 仍缺且点名件在盘 = 1 条") {
		t.Fatalf("表头不符：stdout=%q", out)
	}
}

func TestCheckGapClosureDriftGateMain_Allow(t *testing.T) {
	root, ledger := cgcdFixture(t, `{"id": "GAP-T-1", "state": "仍缺", "unit": "core/cmd/zerg/landed.go"}`)
	cgcdTouch(t, root, "core/cmd/zerg/landed.go")
	rc, out, _ := cgcdRunMain(t, "--ledger", ledger, "--root", root, "--allow", "GAP-T-1")
	if rc != 0 {
		t.Fatalf("--allow 未生效 rc=%d stdout=%q", rc, out)
	}
}

func TestCheckGapClosureDriftGateMain_MissingLedger(t *testing.T) {
	root, _ := cgcdFixture(t)
	rc, _, errb := cgcdRunMain(t, "--ledger", filepath.Join(root, "nope.jsonl"), "--root", root)
	if rc != 2 {
		t.Fatalf("缺语料 rc=%d（期望 2）", rc)
	}
	if !strings.Contains(errb, "BLOCKED —— 真源不存在：") {
		t.Fatalf("stderr 不符：%q", errb)
	}
}

func TestCheckGapClosureDriftGateMain_EmptyLedger(t *testing.T) {
	root, ledger := cgcdFixture(t, "", "   ")
	rc, _, errb := cgcdRunMain(t, "--ledger", ledger, "--root", root)
	if rc != 2 {
		t.Fatalf("空域 rc=%d（期望 2）", rc)
	}
	if !strings.Contains(errb, "真源解析出 0 行（空域）⇒ rc=2 不给结论") {
		t.Fatalf("stderr 不符：%q", errb)
	}
}

func TestCheckGapClosureDriftGateMain_JSON(t *testing.T) {
	root, ledger := cgcdFixture(t, `{"id": "GAP-T-1", "state": "仍缺", "unit": "core/cmd/zerg/landed.go"}`)
	cgcdTouch(t, root, "core/cmd/zerg/landed.go")
	rc, out, _ := cgcdRunMain(t, "--ledger", ledger, "--root", root, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d（期望 1）", rc)
	}
	want := `{"rc": 1, "drift_count": 1, "drift": [{"id": "GAP-T-1", "unit": "core/cmd/zerg/landed.go"}]}` + "\n"
	if out != want {
		t.Fatalf("JSON 不逐字节：\n got=%q\nwant=%q", out, want)
	}
}

func TestCheckGapClosureDriftGateMain_JSONNoHit(t *testing.T) {
	root, ledger := cgcdFixture(t, `{"id": "GAP-T-3", "state": "已解", "unit": "core/cmd/zerg/landed.go"}`)
	rc, out, _ := cgcdRunMain(t, "--ledger", ledger, "--root", root, "--json")
	if rc != 0 {
		t.Fatalf("rc=%d（期望 0）", rc)
	}
	want := `{"rc": 0, "drift_count": 0, "drift": []}` + "\n"
	if out != want {
		t.Fatalf("空集 JSON 不逐字节：\n got=%q\nwant=%q", out, want)
	}
}

func TestCheckGapClosureDriftGateMain_DashDash(t *testing.T) {
	// argparse：无 positionals ⇒ `--` 自身也进 extras（实测 py 逐字 `unrecognized arguments: -- x`）。
	rc, _, errb := cgcdRunMain(t, "--", "x")
	if rc != 2 {
		t.Fatalf("rc=%d（期望 2）", rc)
	}
	if !strings.Contains(errb, "check-gap-closure-drift.py: error: unrecognized arguments: -- x") {
		t.Fatalf("`--` 未进 extras：stderr=%q", errb)
	}
}

func TestCheckGapClosureDriftGateMain_UnknownFlag(t *testing.T) {
	rc, _, errb := cgcdRunMain(t, "--bogus")
	if rc != 2 {
		t.Fatalf("未知旗标 rc=%d（期望 2）", rc)
	}
	if !strings.Contains(errb, "check-gap-closure-drift.py: error: unrecognized arguments: --bogus") {
		t.Fatalf("stderr 不符：%q", errb)
	}
}

func TestCheckGapClosureDriftGateMain_Help(t *testing.T) {
	rc, out, _ := cgcdRunMain(t, "-h")
	if rc != 0 {
		t.Fatalf("-h rc=%d（期望 0）", rc)
	}
	if !strings.Contains(out, "usage: check-gap-closure-drift.py [-h] [--ledger LEDGER] [--root ROOT]") {
		t.Fatalf("usage 不符：%q", out)
	}
	if !strings.Contains(out, "optional arguments:") {
		t.Fatalf("帮助正文不符：%q", out)
	}
}

func TestCheckGapClosureDriftGateMain_PlaceholderAndOutsideSkipped(t *testing.T) {
	root, ledger := cgcdFixture(t,
		`{"id": "GAP-T-4", "state": "仍缺", "unit": "无件(命令面)"}`,
		`{"id": "GAP-T-5", "state": "仍缺", "unit": "/etc/hosts"}`,
		`{"id": "GAP-T-6", "state": "仍缺", "unit": "~/anything"}`,
	)
	rc, out, _ := cgcdRunMain(t, "--ledger", ledger, "--root", root)
	if rc != 0 {
		t.Fatalf("占位/仓外路径误命中 rc=%d stdout=%q", rc, out)
	}
}

// TestCgcdUnitsOf 是「哑化牙」：改坏占位判定或分号拆分 ⇒ 本用例必红（带行号）。
func TestCgcdUnitsOf(t *testing.T) {
	cases := []struct {
		name string
		unit any
		want []string
	}{
		{"占位_无件", "无件", nil},
		{"占位_命令面", "无件(命令面)", nil},
		{"占位_空串", "", nil},
		{"占位_横杠", "-", nil},
		{"分号拆分", "a/b.go ; c/d.go", []string{"a/b.go", "c/d.go"}},
		{"分段含占位", "a ; 无件 ; b", []string{"a", "b"}},
		{"Unicode 空白剥离", "\u00a0a.go\u00a0", []string{"a.go"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cgcdUnitsOf(map[string]any{"unit": c.unit})
			if len(got) != len(c.want) {
				t.Fatalf("unit=%v got=%v want=%v（行号：cgcdUnitsOf）", c.unit, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("unit=%v got=%v want=%v（行号：cgcdUnitsOf）", c.unit, got, c.want)
				}
			}
		})
	}
}

// TestCgcdPyJoin —— posixpath.join 三态（`..` 原样留 / 后段绝对路径丢弃前缀 / 尾斜杠直连）。
func TestCgcdPyJoin(t *testing.T) {
	if got := cgcdPyJoin("/r", "core", "a.go"); got != "/r/core/a.go" {
		t.Fatalf("got=%q", got)
	}
	if got := cgcdPyJoin("/r", "/abs/x"); got != "/abs/x" {
		t.Fatalf("后段绝对路径未丢弃前缀：got=%q", got)
	}
	if got := cgcdPyJoin("/r/", "a.go"); got != "/r/a.go" {
		t.Fatalf("尾斜杠未直连：got=%q", got)
	}
	if got := cgcdPyJoin("/r", "a/../b"); got != "/r/a/../b" {
		t.Fatalf("`..` 被归一（应原样留）：got=%q", got)
	}
}
