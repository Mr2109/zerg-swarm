// check_approve_merge_test.go —— `check_approve_merge.go`（py `scripts/gates/check-approve-merge.py` 的 Go 真搬）
// 的**成对负控**定向单测。
//
// 面（✗ 禁恒绿）：
//
//	① 真仓夹具正控：`scripts/gates/fixtures/approve-merge/` 的四案 + 四反例逐案跑（经进程内复用入口），
//	   案数须 >0（防空转假绿）、不过须 ==0。
//	② A5 负控（有牙面）：产出面把来源机格填成 `本机` ⇒ `apmAuditView` **必须**判红；合法值 ⇒ 不红。
//	   ★ 若有人把他门「值里不许出现本机」那条判据删掉，本用例**立刻红**（见回执「负控有牙」实证）。
//	③ A6 成对：缺 `src_machine` ⇒ rc=2；补齐 ⇒ rc=0（两态不许同形）。
//	④ 用法错 vs 未取数两态不许同形；`-h` rc=0。
//
// ★ 本测件**不调用 python3**（纯 Go 进程内），故无需钉解释器路径。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func apmTRun(argv ...string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkApproveMergeGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func apmTRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// cwd = <repo>/core/cmd/zerg ⇒ 上溯三级
	return filepath.Dir(filepath.Dir(filepath.Dir(wd)))
}

func apmTFixtures() string {
	return filepath.Join(apmTRepoRoot(), "scripts", "gates", "fixtures", "approve-merge")
}

func apmTWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", path, err)
	}
}

// apmTMakeView 造一份 15 格齐的产出面（供 A5 正/负控用）。
func apmTMakeView(machine, srcPath, basis string) *apmOrderedMap {
	view := &apmOrderedMap{}
	view.Set("schema", apmViewSchema)
	view.Set(apmKeyKind, "ApproveMergeView")
	view.Set("headers", apmStrsToIface(apmHeaders))
	view.Set("strength_note", apmStrengthNote)
	view.Set("view_id", "approve-merge/v1")
	view.Set("sources", []interface{}{})
	view.Set("generated_at", "2026-10-01T12:00:00+08:00")
	view.Set("generated_at_note", "夹具")
	r := &apmOrderedMap{}
	r.Set("src_machine", machine)
	r.Set("src_machine_known", true)
	r.Set("src_basis", basis)
	r.Set("src_path", srcPath)
	r.Set("tool", "dev_edit")
	r.Set("approver", "A")
	r.Set("approved_at", "2026-01-02T03:04:05+08:00")
	r.Set("key_id", "0123456789abcdef")
	r.Set("sig_alg", "ed25519")
	r.Set("sig_state", "未验·本机无此钥")
	r.Set("clock_basis", "跨机·不判")
	r.Set("row_state", "能合")
	r.Set("missing_cells", []interface{}{})
	r.Set("conflict_with", []interface{}{})
	r.Set("notes", []interface{}{})
	view.Set("rows", []interface{}{r})
	return view
}

// ① 真仓夹具正控（案数 >0 · 不过 ==0）
func TestApproveMerge_InRepoFixtures(t *testing.T) {
	fix := apmTFixtures()
	if _, err := os.Stat(filepath.Join(fix, "cases.json")); err != nil {
		t.Skipf("夹具目录不在（%s）⇒ 本格跳过（不判绿也不判红）", fix)
	}
	var so bytes.Buffer
	bad, n := apmRunFixtures(&so, fix, true, nil)
	if n == 0 {
		t.Fatalf("夹具案数 = 0 ⇒ **空转假绿**（判据没跑）")
	}
	if len(bad) != 0 {
		t.Fatalf("夹具不过 %d 条（共 %d 案）: %v", len(bad), n, bad)
	}
}

// ② A5 负控（★ 有牙面）：填 `本机` 必红；依据走非法兜底必红；合法值不红。
func TestApproveMerge_A5NegativeControl(t *testing.T) {
	// 负控 1：来源机格填成 `本机`（名册**含** `本机` ⇒ 只有「值里不许出现本机」那条牙能抓住它）
	red, _ := apmAuditView(apmTMakeView("本机", "本机:/x:1", "audit_cell"), []string{"本机"})
	if len(red) == 0 {
		t.Fatal("A5 负控没牙：产出面把来源机格填成「本机」却判绿（应判红）")
	}
	// 负控 2：值恰在册、但依据走 `default_node` 兜底
	red2, _ := apmAuditView(apmTMakeView("Mr2109", "Mr2109:/x:1", "default_node"), []string{"Mr2109"})
	if len(red2) == 0 {
		t.Fatal("A5 负控没牙：`src_basis=default_node`（非法兜底）却判绿（应判红）")
	}
	// 负控 3：值与载体不自洽（写死 Mr2109，回指写别机）
	red3, _ := apmAuditView(apmTMakeView("Mr2109", "node-a:/x:1", "path_prefix"), []string{"Mr2109", "node-a"})
	if len(red3) == 0 {
		t.Fatal("A5 负控没牙：`src_basis=path_prefix` 而回指机名 node-a ≠ src_machine Mr2109 却判绿（应判红）")
	}
	// 正控：合法 ⇒ 不红
	redOK, notesOK := apmAuditView(apmTMakeView("Mr2109", "Mr2109:/x:1", "audit_cell"), []string{"Mr2109"})
	if len(redOK) != 0 {
		t.Fatalf("A5 正控假红：%v", redOK)
	}
	if len(notesOK) == 0 {
		t.Fatal("A5 正控：干净面向来应给「0 处」交代")
	}
}

// ③ A6 成对：缺 src_machine ⇒ rc=2；补齐 ⇒ rc=0。
func TestApproveMerge_A6RequiredCellPaired(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "anon.jsonl")
	apmTWrite(t, src, `{"event":"approve_signed","tool":"dev_edit","approver":"A","approved_at":"2026-01-02T03:04:05+08:00","key_id":"0123456789abcdef"}`+"\n")
	rc, out, _ := apmTRun("--no-self-test", "--source", src, "--roster-nodes", "Mr2109,node-a")
	if rc != 2 {
		t.Fatalf("缺 src_machine 应 rc=2 · 实读 %d", rc)
	}
	if !strings.Contains(out, "缺证据") || !strings.Contains(out, "src_machine") {
		t.Fatalf("缺 src_machine 应点名「缺证据」+ src_machine；实读正文段：%s", apmRuneSlice(out, 200))
	}
	// 成对：补齐 machine ⇒ rc=0
	apmTWrite(t, src, `{"event":"approve_signed","tool":"dev_edit","approver":"A","approved_at":"2026-01-02T03:04:05+08:00","key_id":"0123456789abcdef","machine":"Mr2109"}`+"\n")
	rc2, _, _ := apmTRun("--no-self-test", "--source", src, "--roster-nodes", "Mr2109,node-a")
	if rc2 != 0 {
		t.Fatalf("补齐来源机后应 rc=0 · 实读 %d", rc2)
	}
}

// ④ 两态不许同形：未知旗标（用法错）⟷ 来源件不在（未取数）；`-h` rc=0。
func TestApproveMerge_TwoStatesNotSameShape(t *testing.T) {
	rcBad, _, errBad := apmTRun("--no-self-test", "--not-a-flag")
	if rcBad != 2 || !strings.Contains(errBad, "用法错") {
		t.Fatalf("未知旗标应 rc=2 + 「用法错」· 实读 rc=%d err=%q", rcBad, errBad)
	}
	rcMiss, _, errMiss := apmTRun("--no-self-test", "--source", "/no/such/source.jsonl", "--roster-nodes", "Mr2109,node-a")
	if rcMiss != 2 || strings.Contains(errMiss, "用法错") {
		t.Fatalf("来源件不在应 rc=2 且 ≠「用法错」· 实读 rc=%d err=%q", rcMiss, errMiss)
	}
	rcH, outH, _ := apmTRun("-h")
	if rcH != 0 || !strings.Contains(outH, "usage: check-approve-merge.py") {
		t.Fatalf("-h 应 rc=0 + usage · 实读 rc=%d", rcH)
	}
}

// ⑤ `--root` 生效（py `GAP-20261001-63` · 本笔 Go 新行为 · 有牙）：显式 `--root <非目录>` ⇒
// run_cli 替代入口前置可用性判 ⇒ **rc=2 fail-closed**；改前 `a.root` 是死旗标 ⇒ 静默忽略 ⇒ 自检照过 rc=0。
func TestApproveMerge_RootFlagLive(t *testing.T) {
	// 基线：不给 --root ⇒ 自检照跑，rc=0（证明本条只咬「显式 --root」那一档）
	if rc, _, _ := apmTRun("--self-test"); rc != 0 {
		t.Fatalf("基线失败：不给 --root 应 rc=0，得 rc=%d", rc)
	}
	rc, out, _ := apmTRun("--root", "/nonexistent-twin-root", "--self-test")
	if rc != 2 {
		t.Fatalf("--root 生效失败：非目录 root 应 rc=2（fail-closed），得 rc=%d", rc)
	}
	if !strings.Contains(out, "不过") {
		t.Fatalf("--root 生效失败：自检应报「不过」（run_cli 前置判拦下），得 out=%.120q", out)
	}
}
