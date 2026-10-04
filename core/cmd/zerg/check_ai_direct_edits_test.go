// check_ai_direct_edits_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckAiDirectEditsGatePort（定向：go test -run TestCheckAiDirectEditsGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adeFix —— 造一棵仓外夹具树：`<root>/core/cmd/zerg/testdata/cli-matrix.json`（命令树）
// ＋ `<root>/receipts/`（回执目录）。返回 (root, receiptsDir)。
func adeFix(t *testing.T, receipts map[string]map[string]any) (string, string) {
	t.Helper()
	root := t.TempDir()
	td := filepath.Join(root, "core", "cmd", "zerg", "testdata")
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	matrix := map[string]any{"cases": []any{
		map[string]any{"command": "gate run --fast"},
		map[string]any{"command": "gate ls"},
		map[string]any{"command": "dev edit"},
	}}
	mb, _ := json.Marshal(matrix)
	if err := os.WriteFile(filepath.Join(td, "cli-matrix.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	rd := filepath.Join(root, "receipts")
	if receipts != nil {
		if err := os.MkdirAll(rd, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, obj := range receipts {
			b, _ := json.Marshal(obj)
			if err := os.WriteFile(filepath.Join(rd, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 件级账夹具（R5 上了判 ⇒ 夹具**必须**自带一件，✗ 不许落到默认状态目录 —— 否则测试不 hermetic）。
	if err := os.WriteFile(filepath.Join(root, "ledger.jsonl"),
		[]byte("{\"at\":\"2026-09-19T00:00:00+08:00\",\"event\":\"edit\",\"file\":\"core/x.go\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rd
}

// adeWriteLedger —— 覆写夹具件级账（R5 各格用；一行一条 JSONL，空串跳过）。
func adeWriteLedger(t *testing.T, root string, rows []string) {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		if r == "" {
			continue
		}
		b.WriteString(r + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, "ledger.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// adeGood —— 合规回执模板（与 py `_good()` 同形）。
func adeGood(rid string, kw map[string]any) map[string]any {
	r := map[string]any{
		"round_id": rid, "trace_id": "TR-0123456789ab", "what": "做了事", "next": "下一步",
		"blockers": []any{}, "evidence": []any{"zerg gate run --fast"}, "commits": []any{"deadbee"},
	}
	for k, v := range kw {
		r[k] = v
	}
	return r
}

func adeRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	// R5 上了判 ⇒ 夹具一律自带件级账：没显式给 `--ledger` 时按 `--root` 补一件（✗ 不许落到默认状态目录 —— 否则测试不 hermetic）。
	hasLedger := false
	for _, a := range argv {
		if a == "--ledger" || strings.HasPrefix(a, "--ledger=") {
			hasLedger = true
		}
	}
	if !hasLedger {
		root := "."
		for i, a := range argv {
			if a == "--root" && i+1 < len(argv) {
				root = argv[i+1]
			}
		}
		argv = append(argv, "--ledger", filepath.Join(root, "ledger.jsonl"))
	}
	var out, errb bytes.Buffer
	rc := aidirecteditsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckAiDirectEditsGatePort(t *testing.T) {
	// ── 正控（必然判绿）：合规回执一页 ⇒ rc=0 ────────────────────────────
	rootG, rdG := adeFix(t, map[string]map[string]any{
		"R-20260920T000001.json": adeGood("R-20260920T000001", nil),
	})
	rcGreen, outGreen, _ := adeRun(t, "--root", rootG, "--dir", rdG)
	if rcGreen != 0 || !strings.Contains(outGreen, "⇒ 零直改纪律全过（rc=0）") {
		t.Fatalf("正控失败：合规应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控 R1（必然判红）：动过仓但证据里没有命令面调用 ⇒ rc=1 ──────────
	root1, rd1 := adeFix(t, map[string]map[string]any{
		"R-20260920T000002.json": adeGood("R-20260920T000002",
			map[string]any{"evidence": []any{"vim core/x.go"}}),
	})
	rcR1, outR1, _ := adeRun(t, "--root", root1, "--dir", rd1)
	if rcR1 != 1 || !strings.Contains(outR1, "AI 直改") {
		t.Fatalf("负控-R1 失败：得 rc=%d out=%q", rcR1, outR1)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcR1 {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 R1 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcR1)
	}

	// ── 负控 R2 缺格：缺 next ⇒ rc=1 ────────────────────────────────────
	root2, rd2 := adeFix(t, map[string]map[string]any{
		"R-20260920T000003.json": adeGood("R-20260920T000003", map[string]any{"next": ""}),
	})
	rcR2, outR2, _ := adeRun(t, "--root", root2, "--dir", rd2)
	if rcR2 != 1 || !strings.Contains(outR2, "R2 回执缺格 next") {
		t.Fatalf("负控-R2 缺格 失败：得 rc=%d out=%q", rcR2, outR2)
	}

	// ── 负控 R2 trace 形态：trace_id 坏 ⇒ rc=1（含 Python repr 引号） ─────
	root2b, rd2b := adeFix(t, map[string]map[string]any{
		"R-20260920T000004.json": adeGood("R-20260920T000004", map[string]any{"trace_id": "trace-bad"}),
	})
	rcR2b, outR2b, _ := adeRun(t, "--root", root2b, "--dir", rd2b)
	if rcR2b != 1 || !strings.Contains(outR2b, "trace_id 形态不对") || !strings.Contains(outR2b, "⇒ 'trace-bad'") {
		t.Fatalf("负控-R2 trace 失败：得 rc=%d out=%q", rcR2b, outR2b)
	}

	// ── 负控 R3：件名 ≠ round_id ⇒ rc=1 ─────────────────────────────────
	root3, rd3 := adeFix(t, map[string]map[string]any{
		"R-错的.json": adeGood("R-20260920T000005", nil),
	})
	rcR3, outR3, _ := adeRun(t, "--root", root3, "--dir", rd3)
	if rcR3 != 1 || !strings.Contains(outR3, "R3 件名与 round_id 对不上") {
		t.Fatalf("负控-R3 失败：得 rc=%d out=%q", rcR3, outR3)
	}

	// ── 负控 R4：点名的命令面调用在命令树里解析不到 ⇒ rc=1 ───────────────
	root4, rd4 := adeFix(t, map[string]map[string]any{
		"R-20260920T000006.json": adeGood("R-20260920T000006",
			map[string]any{"evidence": []any{"zerg zzznoSuchCmd"}}),
	})
	rcR4, outR4, _ := adeRun(t, "--root", root4, "--dir", rd4)
	if rcR4 != 1 || !strings.Contains(outR4, "幽灵调用") {
		t.Fatalf("负控-R4 失败：得 rc=%d out=%q", rcR4, outR4)
	}

	// ── 负控 R1 不误判：没动仓（commits 空）⇒ rc=0 ──────────────────────
	root5, rd5 := adeFix(t, map[string]map[string]any{
		"R-20260920T000007.json": adeGood("R-20260920T000007",
			map[string]any{"commits": []any{}, "evidence": []any{"只读 grep"}}),
	})
	rcNo1, _, _ := adeRun(t, "--root", root5, "--dir", rd5)
	if rcNo1 != 0 {
		t.Fatalf("负控-R1 不误判 失败：得 rc=%d（应为 0）", rcNo1)
	}

	// ── 不给结论（≠ 绿）：回执目录 0 页 ⇒ rc=2 ──────────────────────────
	root6, rd6 := adeFix(t, map[string]map[string]any{})
	rcEmpty, outEmpty, _ := adeRun(t, "--root", root6, "--dir", rd6)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "0 页") {
		t.Fatalf("空转失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}
	if rcEmpty == rcGreen {
		t.Fatalf("成对断言失败：空转 rc=%d 与正控 rc=%d 同码 ⇒ 空转被当绿", rcEmpty, rcGreen)
	}

	// ── 取不到输入：回执目录不存在 ⇒ rc=2 ───────────────────────────────
	root7, rd7 := adeFix(t, nil)
	rcMiss, outMiss, _ := adeRun(t, "--root", root7, "--dir", filepath.Join(rd7, "nope"))
	if rcMiss != 2 || !strings.Contains(outMiss, "回执目录不存在") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── R5a（假绿绕行 · GAP-20260927-81）：手改后补一条含任意 zerg 调用的回执 ⇒ rc=1 ──
	rootR5a, rdR5a := adeFix(t, map[string]map[string]any{
		"R-20260920T100000.json": adeGood("R-20260920T100000", map[string]any{
			"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}),
		"R-20260920T120000.json": adeGood("R-20260920T120000", map[string]any{
			"created_at": "2026-09-20T12:00:00+08:00", "evidence": []any{"zerg gate ls"}}),
	})
	adeWriteLedger(t, rootR5a, []string{`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/x.go"}`})
	rcR5a, outR5a, _ := adeRun(t, "--root", rootR5a, "--dir", rdR5a)
	if rcR5a != 1 || !strings.Contains(outR5a, "R5a") {
		t.Fatalf("负控-R5a 失败：得 rc=%d out=%q", rcR5a, outR5a)
	}
	if rcR5a == rcGreen {
		t.Fatalf("成对断言失败：R5a rc=%d 与正控 rc=%d 同码 ⇒ 假绿绕行未被拦", rcR5a, rcGreen)
	}

	// ── R5b（覆盖自证 · 漏）：件级账最新写轮晚于最新回执 ⇒ rc=2（不是绿）──────────
	rootR5b, rdR5b := adeFix(t, map[string]map[string]any{
		"R-20260920T100100.json": adeGood("R-20260920T100100", map[string]any{
			"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}),
	})
	adeWriteLedger(t, rootR5b, []string{
		`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/x.go"}`,
		`{"at":"2026-09-20T12:00:00+08:00","event":"edit","file":"core/y.go"}`,
	})
	rcR5b, outR5b, _ := adeRun(t, "--root", rootR5b, "--dir", rdR5b)
	if rcR5b != 2 || !strings.Contains(outR5b, "R5b") {
		t.Fatalf("负控-R5b 失败：得 rc=%d out=%q", rcR5b, outR5b)
	}

	// ── R5 正控：写轮在轮窗内 + sibling 行不上判 ⇒ rc=0 ────────────────────
	rootR5g, rdR5g := adeFix(t, map[string]map[string]any{
		"R-20260920T100200.json": adeGood("R-20260920T100200", map[string]any{
			"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}),
	})
	adeWriteLedger(t, rootR5g, []string{
		`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/x.go"}`,
		`{"at":"2026-09-21T00:00:00+08:00","event":"edit","file":"docs/a.md","sibling_repo":"Zerg-内部文档"}`,
	})
	rcR5g, outR5g, _ := adeRun(t, "--root", rootR5g, "--dir", rdR5g)
	if rcR5g != 0 || !strings.Contains(outR5g, "R5 轮级账 ⟷ 件级账双向对账 ✓") {
		t.Fatalf("R5 正控失败：得 rc=%d out=%q", rcR5g, outR5g)
	}

	// ── R5 不误判：回执早于件级账最早一行 ⇒ R5a 不上判，但 R5b 仍 rc=2（真仓首件即此态）──
	rootR5s, rdR5s := adeFix(t, map[string]map[string]any{
		"R-20260919T235900.json": adeGood("R-20260919T235900", map[string]any{
			"created_at": "2026-09-19T23:59:00+08:00"}),
	})
	adeWriteLedger(t, rootR5s, []string{`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/x.go"}`})
	rcR5s, outR5s, _ := adeRun(t, "--root", rootR5s, "--dir", rdR5s)
	if rcR5s != 2 || strings.Contains(outR5s, "R5a") {
		t.Fatalf("R5 不上判格失败：得 rc=%d out=%q（应 rc=2 且无 R5a）", rcR5s, outR5s)
	}

	// ── R5 不上判 + 空件级账 ⇒ rc=0（空账不算「更晚的写轮」）────────────────
	rootR5e, rdR5e := adeFix(t, map[string]map[string]any{
		"R-20260919T230000.json": adeGood("R-20260919T230000", map[string]any{
			"created_at": "2026-09-19T23:00:00+08:00"}),
	})
	adeWriteLedger(t, rootR5e, nil)
	rcR5e, outR5e, _ := adeRun(t, "--root", rootR5e, "--dir", rdR5e)
	if rcR5e != 0 {
		t.Fatalf("空件级账格失败：得 rc=%d out=%q（应 0）", rcR5e, outR5e)
	}

	// ── 多页定序：两页坏件 ⇒ 逐条按件名字典序（R-…009 先于 R-…010） ──────
	root8, rd8 := adeFix(t, map[string]map[string]any{
		"R-20260920T000010.json": adeGood("R-20260920T000010",
			map[string]any{"evidence": []any{"vim a"}}),
		"R-20260920T000009.json": adeGood("R-20260920T000009",
			map[string]any{"evidence": []any{"vim b"}}),
	})
	rcOrd, outOrd, _ := adeRun(t, "--root", root8, "--dir", rd8)
	i9 := strings.Index(outOrd, "R-20260920T000009.json")
	i10 := strings.Index(outOrd, "R-20260920T000010.json")
	if rcOrd != 1 || i9 < 0 || i10 < 0 || i9 > i10 {
		t.Fatalf("多页定序失败：得 rc=%d i9=%d i10=%d out=%q", rcOrd, i9, i10, outOrd)
	}

	// ── 未知旗标 ⇒ rc=2（stderr 明说 · 丢结论） ─────────────────────────
	rcFlag, _, errFlag := adeRun(t, "--root", rootG, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说） ──────────
	rcJSON, _, errJSON := adeRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}
	rcJSON2, _, _ := adeRun(t, "--json=stdout")
	if rcJSON2 != 2 {
		t.Fatalf("--json=stdout 拒收失败：得 rc=%d", rcJSON2)
	}

	// ── `-h/--help` ⇒ rc=0 且出 usage（与 py argparse 逐字档） ───────────
	rcH, outH, _ := adeRun(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "usage: check-ai-direct-edits.py [-h] [--root ROOT] [--dir DIR] [--ledger LEDGER] [--self-test]") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── --self-test 三档 3 正控 + 10 负控 + 2 空转自身必须全过（防恒绿/恒红） ─
	rcST, outST, _ := adeRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "门⑬ 自检全过：3 正控 + 10 负控 + 2 空转（成对负控）") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
