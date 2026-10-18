// check_write_inventory_test.go —— `check-write-inventory` 判据的牙（正控 ＋ 负控成对）。
//
// 判「一条判据有没有牙」只有一种证法：**造一个真缺陷，看它是否变红并点名**。
// 本件逐个判据点各造一枚夹具，正控（守规矩）与负控（破承诺）**成对**跑，并且**断言退码**（不是只断言
// 某个内部函数的读数）—— 变异点落在「调用方拿到的那个 rc」上，才证明牙长在门上。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// wviTestEntry —— 造一行真源声明（默认档零写盘 · 不做裸跑探针）。
func wviTestEntry(path string, mutate ...func(*wviEntry)) wviEntry {
	e := wviEntry{
		Path:         path,
		DefaultWrite: json.RawMessage(`false`),
		Probe:        "exempt",
		GatedBy:      []string{},
		WritesWhere:  []string{},
	}
	for _, m := range mutate {
		m(&e)
	}
	return e
}

func wviTestCmd(path, usage string) wviCmd {
	return wviCmd{Path: path, Usage: usage, Run: "wviFixtureRun"}
}

// wviRunFixture —— 夹具的 src facts：默认「真读 --dry-run、有 --yes 闸」。
func wviFacts(dry, yes bool) map[string]wviSrcFact {
	return map[string]wviSrcFact{"fixture cmd": {HasDryRun: dry, HasYesGuard: yes, Resolved: true}}
}

func wviEval(doc *wviDoc, cmds []wviCmd, facts map[string]wviSrcFact, bin string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := wviEvaluate(doc, cmds, facts, wviOpts{Bin: bin}, &out, &errb)
	return rc, out.String(), errb.String()
}

// ── ① 实践面：声明「默认档零写盘」而裸跑真写 ⇒ 红且点名（本判据的主牙）───────────────
func TestWriteInventoryDeclaredReadOnlyButWrites(t *testing.T) {
	doc := &wviDoc{
		Schema:  "zerg/write-inventory/v1",
		Entries: []wviEntry{wviTestEntry("fixture cmd")},
		Curated: []wviCurated{{
			Path:   "fixture cmd",
			Argv:   []string{"-c", "mkdir -p \"$SB/state\" && printf x > \"$SB/state/leak.json\""},
			Expect: "nowrite",
			Note:   "负控：声明零写盘却落了一件",
		}},
	}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCBreach {
		t.Fatalf("负控：声明零写盘而裸跑写盘 ⇒ 要 rc=1，得 %d（stderr=%s）", rc, errs)
	}
	if !strings.Contains(errs, "fixture cmd") {
		t.Fatalf("负控：红要**点名命令**，实得 stderr=%s", errs)
	}
	if !strings.Contains(errs, "state/leak.json") {
		t.Fatalf("负控：红要**点名写到哪一件**，实得 stderr=%s", errs)
	}
}

// 正控：同一夹具改成不写 ⇒ 绿（证明上一条的红来自「写了」，不是来自夹具形态）。
func TestWriteInventoryDeclaredReadOnlyNoWrite(t *testing.T) {
	doc := &wviDoc{
		Schema:  "zerg/write-inventory/v1",
		Entries: []wviEntry{wviTestEntry("fixture cmd")},
		Curated: []wviCurated{{Path: "fixture cmd", Argv: []string{"-c", "exit 0"}, Expect: "nowrite"}},
	}
	rc, out, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCLand {
		t.Fatalf("正控：零写盘 ⇒ 要 rc=0，得 %d（stdout=%s stderr=%s）", rc, out, errs)
	}
}

// ── ② 只读旗标名：usage 里没宣告 ⇒ 红（用户按名字敲会撞未知旗标）───────────────────
func TestWriteInventoryReadonlyFlagNotInUsage(t *testing.T) {
	e := wviTestEntry("fixture cmd", func(e *wviEntry) { e.ReadonlyFlag = "--dry-run" })
	doc := &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{e}}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCBreach || !strings.Contains(errs, "--dry-run") {
		t.Fatalf("负控：只读旗标不在 usage ⇒ 要 rc=1 且点名该旗标，得 rc=%d stderr=%s", rc, errs)
	}
	// 正控：把它写进 usage ⇒ 绿。
	rc2, _, errs2 := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd [--dry-run]")}, wviFacts(true, false), "/bin/sh")
	if rc2 != wviRCLand {
		t.Fatalf("正控：旗标在 usage + 实现面真读它 ⇒ 要 rc=0，得 %d（%s）", rc2, errs2)
	}
}

// ── ③ 只读旗标是摆设：usage 宣告了，实现面从不读干跑字段 ⇒ 红 ───────────────────────
func TestWriteInventoryDryRunAdvertisedButIgnored(t *testing.T) {
	e := wviTestEntry("fixture cmd", func(e *wviEntry) { e.ReadonlyFlag = "--dry-run" })
	doc := &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{e}}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd [--dry-run]")}, wviFacts(false, false), "/bin/sh")
	if rc != wviRCBreach || !strings.Contains(errs, "从不读") {
		t.Fatalf("负控：--dry-run 无消费者 ⇒ 要 rc=1 且点名，得 rc=%d stderr=%s", rc, errs)
	}
}

// ── ④ `--yes` 闸：登记了 gated_by 而实现面没有 writeGuardYes ⇒ 红 ──────────────────
func TestWriteInventoryYesGuardMissing(t *testing.T) {
	e := wviTestEntry("fixture cmd", func(e *wviEntry) { e.GatedBy = []string{"--yes"} })
	doc := &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{e}}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCBreach || !strings.Contains(errs, "writeGuardYes") {
		t.Fatalf("负控：声明有 --yes 闸而代码没闸 ⇒ 要 rc=1，得 rc=%d stderr=%s", rc, errs)
	}
	rc2, _, _ := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, true), "/bin/sh")
	if rc2 != wviRCLand {
		t.Fatalf("正控：闸在 ⇒ 要 rc=0，得 %d", rc2)
	}
}

// ── ⑤ 登记面完备：catalog 有而真源没有 / 真源有而 catalog 没有 ⇒ 两头都红 ────────────
func TestWriteInventoryCoverageBothWays(t *testing.T) {
	doc := &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{wviTestEntry("ghost cmd")}}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCBreach {
		t.Fatalf("负控：未登记命令 ⇒ 要 rc=1，得 %d", rc)
	}
	if !strings.Contains(errs, "catalog 里有、写盘点里没有") || !strings.Contains(errs, "写盘点里有、catalog 里没有") {
		t.Fatalf("负控：两头都要点名（未登记 ＋ 幽灵条目），实得 stderr=%s", errs)
	}
}

// ── ⑥ 默认档真写盘却不在危险档 ⇒ 红；锁件类已登记 waiver ⇒ 只报注不判红 ──────────────
func TestWriteInventoryWritesOutsideDangerTier(t *testing.T) {
	e := wviTestEntry("fixture cmd", func(e *wviEntry) {
		e.DefaultWrite = json.RawMessage(`true`)
		e.WritesWhere = []string{"state"}
	})
	doc := &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{e}}
	rc, _, errs := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc != wviRCBreach || !strings.Contains(errs, "不在任何危险档") {
		t.Fatalf("负控：默认档写盘且 danger 空 ⇒ 要 rc=1，得 rc=%d stderr=%s", rc, errs)
	}
	// 正控：把同一行标成锁件例外（write_kind=lock ＋ waiver）⇒ 只报注、不判红。
	lock := wviTestEntry("fixture cmd", func(x *wviEntry) {
		x.DefaultWrite = json.RawMessage(`true`)
		x.WritesWhere = []string{"state"}
		x.WriteKind = "lock"
		x.Waiver = "锁件不是耐久产物（夹具）"
	})
	doc = &wviDoc{Schema: "zerg/write-inventory/v1", Entries: []wviEntry{lock}}
	rc2, out2, errs2 := wviEval(doc, []wviCmd{wviTestCmd("fixture cmd", "zerg fixture cmd")}, wviFacts(true, false), "/bin/sh")
	if rc2 != wviRCLand {
		t.Fatalf("正控：锁件例外已登记 ⇒ 要 rc=0，得 %d（%s %s）", rc2, out2, errs2)
	}
	if !strings.Contains(out2, "write_kind=lock") {
		t.Fatalf("正控：例外必须在人面**显式打出来**（不许静默），实得 stdout=%s", out2)
	}
}

// ── ⑦ 判不了：真源缺 / 坏 / schema 不对 / 空表 ⇒ rc=8（✗ 不当绿）────────────────────
func TestWriteInventoryUnjudgeableInputs(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"garbage.json": "not json at all",
		"schema.json":  `{"schema":"other/v9","entries":[{"path":"x"}]}`,
		"empty.json":   `{"schema":"zerg/write-inventory/v1","entries":[]}`,
	} {
		p := dir + "/" + name
		if err := writeFileForTest(p, body); err != nil {
			t.Fatalf("夹具写不进：%v", err)
		}
		doc, rc, why := wviLoadInventory(p)
		if doc != nil || rc != wviRCUnjudge {
			t.Fatalf("%s ⇒ 要 rc=8 且不给 doc，得 rc=%d why=%s", name, rc, why)
		}
	}
	if _, rc, _ := wviLoadInventory(dir + "/absent.json"); rc != wviRCUnjudge {
		t.Fatalf("真源不在盘 ⇒ 要 rc=8，得 %d", rc)
	}
}

// ── ⑧ 三态解析：default_writes 只认 true/false/"unprobed" ──────────────────────────
func TestWriteInventoryTriState(t *testing.T) {
	cases := []struct {
		raw   string
		want  bool
		state string
		ok    bool
	}{
		{`true`, true, "true", true},
		{`false`, false, "false", true},
		{`"unprobed"`, false, "unprobed", true},
		{`"nope"`, false, `bad:"nope"`, false},
		{``, false, "missing", false},
	}
	for _, c := range cases {
		got, state, ok := wviTri(json.RawMessage(c.raw))
		if got != c.want || state != c.state || ok != c.ok {
			t.Fatalf("wviTri(%s) = (%v,%s,%v)，要 (%v,%s,%v)", c.raw, got, state, ok, c.want, c.state, c.ok)
		}
	}
}

// ── ⑨ 真源件自检：现读仓内真源必须 schema 对、行数非空、每条都有 probe 闭集值 ─────────
func TestWriteInventoryRepoFileShape(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("仓根解析不到（合成树里跑测试）⇒ 这一格不判")
	}
	doc, rc, why := wviLoadInventory(wviInventoryPath(root))
	if doc == nil {
		t.Fatalf("仓内真源读不到：%s（rc=%d）", why, rc)
	}
	for _, e := range doc.Entries {
		if e.Probe != "live" && e.Probe != "exempt" {
			t.Fatalf("%s：probe=%q 不在闭集 {live,exempt}", e.Path, e.Probe)
		}
		if e.Probe == "exempt" && strings.TrimSpace(e.Reason) == "" {
			t.Fatalf("%s：豁免必须写 reason（✗ 不静默豁免）", e.Path)
		}
	}
}

// writeFileForTest —— 夹具落一件（测试专用，不进生产路径）。
func writeFileForTest(p, body string) error {
	return os.WriteFile(p, []byte(body), 0o644)
}
