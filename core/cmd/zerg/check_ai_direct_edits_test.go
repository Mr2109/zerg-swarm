// check_ai_direct_edits_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckAiDirectEditsGatePort（定向：go test -run TestCheckAiDirectEditsGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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
		"R-20260920T000001.json": adeGood("R-20260920T000001",
			map[string]any{"created_at": "2026-09-20T10:00:00+08:00"}),
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
			map[string]any{"commits": []any{}, "evidence": []any{"只读 grep"},
				"created_at": "2026-09-20T10:00:00+08:00"}),
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

	// ── --self-test 三档 3 正控 + 13 负控 + 2 空转自身必须全过（防恒绿/恒红） ─
	rcST, outST, _ := adeRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "门⑬ 自检全过：3 正控 + 13 负控 + 2 空转（成对负控）") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}

// TestF6R5aFaces —— 门⑬ R5a 代拍口径（GAP-20261015-05）**四态** ＋ 无件清单旧口径回落的成对定向单测。
//
// 四态（每态真跑一遍入口 rc · 仓外夹具 · ✗ 不动真仓）：
//
//	① 源码面零编辑行（件在盘 · 件级账零行）＋ 轮窗空 ⇒ **仍红**
//	② 生成面（`docs/**`）⇒ **只报告（不红）**
//	③ 删除面（件不在盘 ⇒ 非 git 面退盘上存在性）⇒ **只报告（不红）**
//	④ 源码面真直写（该件 dev edit 只在其后＝补笔）＋ 轮窗空 ⇒ **仍红**
//	⑤ 页证据无可解析 `--file` 清单 ⇒ 按旧口径（轮窗）落红 ⇒ 旧断言不放宽
func TestF6R5aFaces(t *testing.T) {
	// adeF6Root —— 造仓外夹具根：命令树 ＋ 盘上件 ＋ 两页回执（P1 = 覆盖页 · P2 = 待判页）。
	adeF6Root := func(t *testing.T, p2ev string, disk []string) (string, string, string) {
		t.Helper()
		root := t.TempDir()
		td := filepath.Join(root, "core", "cmd", "zerg", "testdata")
		if err := os.MkdirAll(td, 0o755); err != nil {
			t.Fatal(err)
		}
		mb, _ := json.Marshal(map[string]any{"cases": []any{
			map[string]any{"command": "gate run --fast"}, map[string]any{"command": "repo commit"}}})
		if err := os.WriteFile(filepath.Join(td, "cli-matrix.json"), mb, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, p := range disk {
			fp := filepath.Join(root, p)
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fp, []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rd := filepath.Join(root, "receipts")
		if err := os.MkdirAll(rd, 0o755); err != nil {
			t.Fatal(err)
		}
		pg := func(name, ca, ev string) {
			b, _ := json.Marshal(adeGood(strings.TrimSuffix(name, ".json"),
				map[string]any{"created_at": ca, "evidence": []any{ev}}))
			if err := os.WriteFile(filepath.Join(rd, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		pg("R-20261002T000000.json", "2026-10-02T00:00:00+08:00",
			"zerg repo commit --file src/live.go --message m --yes")
		pg("R-20261002T010000.json", "2026-10-02T01:00:00+08:00", p2ev)
		return root, rd, filepath.Join(root, "ledger.jsonl")
	}
	// adeF6Run —— 进程内跑入口（与 CLI 同一入口 · 同一退码面）。
	adeF6Run := func(t *testing.T, root, rd, led string) (int, string) {
		t.Helper()
		var o, e bytes.Buffer
		rc := aidirecteditsGateMain(&o, &e, []string{"--root", root, "--dir", rd, "--ledger", led})
		return rc, o.String() + e.String()
	}
	led1 := `{"at":"2026-10-01T23:00:00+08:00","event":"edit","file":"src/live.go"}` + "\n"

	// ── 态① 源码面零编辑行 ⇒ 仍红 ──────────────────────────────────────────────
	root1, rd1, ledp1 := adeF6Root(t, "zerg repo commit --file src/zero.go --message m --yes",
		[]string{"src/zero.go", "src/live.go"})
	if err := os.WriteFile(ledp1, []byte(led1), 0o644); err != nil {
		t.Fatal(err)
	}
	rc1, out1 := adeF6Run(t, root1, rd1, ledp1)
	if rc1 != 1 || !strings.Contains(out1, "R5a") || !strings.Contains(out1, "src/zero.go") {
		t.Fatalf("态① 源码面零编辑行应 rc=1 且点名 src/zero.go，得 rc=%d out=%q", rc1, out1)
	}
	// ── 态② 生成面 ⇒ 只报告（不红）───────────────────────────────────────────
	root2, rd2, ledp2 := adeF6Root(t, "zerg repo commit --file docs/zh/参考-命令行.md --message m --yes",
		[]string{"src/live.go", "docs/zh/参考-命令行.md"})
	if err := os.WriteFile(ledp2, []byte(led1), 0o644); err != nil {
		t.Fatal(err)
	}
	rc2, out2 := adeF6Run(t, root2, rd2, ledp2)
	if rc2 != 0 || !strings.Contains(out2, "只报告") || !strings.Contains(out2, "docs/zh/参考-命令行.md") {
		t.Fatalf("态② 生成面应 rc=0（只报告 · 点名 docs/zh/参考-命令行.md），得 rc=%d out=%q", rc2, out2)
	}
	// ── 态③ 删除面 ⇒ 只报告（不红）───────────────────────────────────────────
	root3, rd3, ledp3 := adeF6Root(t, "zerg repo commit --file src/gone.go --message m --yes",
		[]string{"src/live.go"})
	if err := os.WriteFile(ledp3, []byte(led1), 0o644); err != nil {
		t.Fatal(err)
	}
	rc3, out3 := adeF6Run(t, root3, rd3, ledp3)
	if rc3 != 0 || !strings.Contains(out3, "只报告") || !strings.Contains(out3, "src/gone.go") {
		t.Fatalf("态③ 删除面应 rc=0（只报告 · 点名 src/gone.go），得 rc=%d out=%q", rc3, out3)
	}
	// ── 态④ 源码面真直写（补笔晚于页）⇒ 仍红 ──────────────────────────────────
	root4, rd4, ledp4 := adeF6Root(t, "zerg repo commit --file src/late.go --message m --yes",
		[]string{"src/late.go", "src/live.go"})
	if err := os.WriteFile(ledp4, []byte(led1+
		`{"at":"2026-10-02T03:00:00+08:00","event":"edit","file":"src/late.go"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 第三页（05:00）把「最新回执」顶到补笔之后 ⇒ rc 只能由 R5a 给（不让 R5b 混进来）
	b3, _ := json.Marshal(adeGood("R-20261002T050000", map[string]any{
		"created_at": "2026-10-02T05:00:00+08:00",
		"evidence":   []any{"zerg repo commit --file src/live.go --message m --yes"}}))
	if err := os.WriteFile(filepath.Join(rd4, "R-20261002T050000.json"), b3, 0o644); err != nil {
		t.Fatal(err)
	}
	rc4, out4 := adeF6Run(t, root4, rd4, ledp4)
	if rc4 != 1 || !strings.Contains(out4, "src/late.go") {
		t.Fatalf("态④ 真直写（补笔晚于页）应 rc=1 且点名 src/late.go，得 rc=%d out=%q", rc4, out4)
	}
	// ── 态⑤ 无件清单 ⇒ 旧口径（轮窗）落红 · 不放宽 ─────────────────────────────
	root5, rd5, ledp5 := adeF6Root(t, "zerg repo commit --message m --yes", []string{"src/live.go"})
	if err := os.WriteFile(ledp5, []byte(led1), 0o644); err != nil {
		t.Fatal(err)
	}
	rc5, out5 := adeF6Run(t, root5, rd5, ledp5)
	if rc5 != 1 || !strings.Contains(out5, "R5a") {
		t.Fatalf("态⑤ 无件清单应 rc=1（旧口径不放宽），得 rc=%d out=%q", rc5, out5)
	}
	// ── 成对（✗ 禁恒绿/恒红）：只报告面（②③=0）与源码面（①④⑤=1）必须异码 ────────
	if rc1 != 1 || rc2 != 0 || rc3 != 0 || rc4 != 1 || rc5 != 1 {
		t.Fatalf("成对断言失败：①%d ②%d ③%d ④%d ⑤%d", rc1, rc2, rc3, rc4, rc5)
	}
}

// TestF6R5aSignature —— 门⑬ R5a **署名面分流**（L1 · 用户已拍口径）四态 ＋ 成对负控的定向单测。
//
// want（用户已拍）=「**AI** 绕正门直写」—— 非 AI（人）的直改只报告、不进 rc；取不到署名 ⇒ fail-closed 判红。
// 判据面 = 该轮提交信息里的 `由:` trailer（`zerg repo commit --by` 落 · 真源 `family_repo_commit.go`）；
// git 的 `%an`/`%cn` **零鉴别力**（全仓一律 `Mr2109`）。
// 夹具 = **真 git 仓**（`git init` ＋ 一条带（或不带）`由:` trailer 的提交）；P2 页 `commits` = 该提交 sha。
//
//	① AI 署名（`由: Hermes (AI)`）＋ 源码面零编辑行 ⇒ **红**
//	② 非 AI 署名（`由: Mr2109`）＋ 源码面零编辑行 ⇒ **不红**（只报告）
//	③ 生成面 / 删除面 ⇒ **不红**（既有两态不回归）
//	④ 取不到署名（提交信息无 `由:` trailer）⇒ **红**（fail-closed）
func TestF6R5aSignature(t *testing.T) {
	gitRun := func(t *testing.T, root string, args ...string) string {
		t.Helper()
		a := append([]string{"-C", root, "-c", "user.email=x@y", "-c", "user.name=x",
			"-c", "commit.gpgsign=false"}, args...)
		out, err := exec.Command("git", a...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	// adeSigRoot —— 造**真 git** 夹具仓：命令树 ＋ 盘上件（已 `git add`）＋ 一条 trailerLine 提交 ＋ 两页回执。
	// 返回 (root, receiptsDir, ledgerPath, commitSha)。P2（待判页）的 `commits` = 该提交 sha。
	adeSigRoot := func(t *testing.T, trailerLine, p2ev string, disk []string) (string, string, string, string) {
		t.Helper()
		root := t.TempDir()
		td := filepath.Join(root, "core", "cmd", "zerg", "testdata")
		if err := os.MkdirAll(td, 0o755); err != nil {
			t.Fatal(err)
		}
		mb, _ := json.Marshal(map[string]any{"cases": []any{
			map[string]any{"command": "gate run --fast"}, map[string]any{"command": "repo commit"}}})
		if err := os.WriteFile(filepath.Join(td, "cli-matrix.json"), mb, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, p := range disk {
			fp := filepath.Join(root, p)
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fp, []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		gitRun(t, root, "init", "-q")
		gitRun(t, root, "add", "-A")
		msg := "题 M"
		if trailerLine != "" {
			msg += "\n\n" + trailerLine
		}
		gitRun(t, root, "commit", "-q", "--allow-empty", "-m", msg)
		sha := gitRun(t, root, "rev-parse", "HEAD")
		rd := filepath.Join(root, "receipts")
		if err := os.MkdirAll(rd, 0o755); err != nil {
			t.Fatal(err)
		}
		pg := func(name, ca, ev string, commits []any) {
			b, _ := json.Marshal(adeGood(strings.TrimSuffix(name, ".json"),
				map[string]any{"created_at": ca, "evidence": []any{ev}, "commits": commits}))
			if err := os.WriteFile(filepath.Join(rd, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		pg("R-20261002T000000.json", "2026-10-02T00:00:00+08:00",
			"zerg repo commit --file src/live.go --message m --yes", []any{"deadbee"})
		pg("R-20261002T010000.json", "2026-10-02T01:00:00+08:00", p2ev, []any{sha})
		return root, rd, filepath.Join(root, "ledger.jsonl"), sha
	}
	adeF6Run := func(t *testing.T, root, rd, led string) (int, string) {
		t.Helper()
		var o, e bytes.Buffer
		rc := aidirecteditsGateMain(&o, &e, []string{"--root", root, "--dir", rd, "--ledger", led})
		return rc, o.String() + e.String()
	}
	led1 := `{"at":"2026-10-01T23:00:00+08:00","event":"edit","file":"src/live.go"}` + "\n"
	writeLed := func(t *testing.T, led, body string) {
		t.Helper()
		if err := os.WriteFile(led, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// ── 态① AI 署名 ＋ 源码面零编辑行 ⇒ 仍红 ────────────────────────────────────
	r1, rd1, l1, _ := adeSigRoot(t, "由: Hermes (AI)", "zerg repo commit --file src/zero.go --message m --yes",
		[]string{"src/zero.go", "src/live.go"})
	writeLed(t, l1, led1)
	rc1, out1 := adeF6Run(t, r1, rd1, l1)
	if rc1 != 1 || !strings.Contains(out1, "R5a") || !strings.Contains(out1, "src/zero.go") || !strings.Contains(out1, "AI 面") {
		t.Fatalf("态① AI 署名应 rc=1 且点名 src/zero.go（AI 面），得 rc=%d out=%q", rc1, out1)
	}
	// ── 态② 非 AI 署名（Mr2109）⇒ 只报告（不红）────────────────────────────────
	r2, rd2, l2, _ := adeSigRoot(t, "由: Mr2109", "zerg repo commit --file src/zero.go --message m --yes",
		[]string{"src/zero.go", "src/live.go"})
	writeLed(t, l2, led1)
	rc2, out2 := adeF6Run(t, r2, rd2, l2)
	if rc2 != 0 || !strings.Contains(out2, "非 AI 署名") || !strings.Contains(out2, "Mr2109") {
		t.Fatalf("态② 非 AI 署名应 rc=0（只报告 · 点 Mr2109），得 rc=%d out=%q", rc2, out2)
	}
	// ── 态③-a 生成面 ⇒ 只报告（不红）───────────────────────────────────────────
	r3, rd3, l3, _ := adeSigRoot(t, "由: Hermes (AI)", "zerg repo commit --file docs/zh/参考-命令行.md --message m --yes",
		[]string{"src/live.go", "docs/zh/参考-命令行.md"})
	writeLed(t, l3, led1)
	rc3, out3 := adeF6Run(t, r3, rd3, l3)
	if rc3 != 0 || !strings.Contains(out3, "只报告") {
		t.Fatalf("态③-a 生成面应 rc=0（只报告），得 rc=%d out=%q", rc3, out3)
	}
	// ── 态③-b 删除面 ⇒ 只报告（不红）───────────────────────────────────────────
	r3b, rd3b, l3b, _ := adeSigRoot(t, "由: Hermes (AI)", "zerg repo commit --file src/gone.go --message m --yes",
		[]string{"src/live.go"})
	writeLed(t, l3b, led1)
	rc3b, out3b := adeF6Run(t, r3b, rd3b, l3b)
	if rc3b != 0 || !strings.Contains(out3b, "只报告") {
		t.Fatalf("态③-b 删除面应 rc=0（只报告），得 rc=%d out=%q", rc3b, out3b)
	}
	// ── 态④ 取不到署名（无 `由:` trailer）⇒ 仍红（fail-closed · ✗ 默认放行）────────
	r4, rd4, l4, _ := adeSigRoot(t, "", "zerg repo commit --file src/zero.go --message m --yes",
		[]string{"src/zero.go", "src/live.go"})
	writeLed(t, l4, led1)
	rc4, out4 := adeF6Run(t, r4, rd4, l4)
	if rc4 != 1 || !strings.Contains(out4, "取不到署名") {
		t.Fatalf("态④ 取不到署名应 rc=1（fail-closed），得 rc=%d out=%q", rc4, out4)
	}
	// ── 成对（✗ 禁恒绿/恒红）：AI 面（①④=1）与非 AI 面（②=0）必须异码 ────────────
	if !(rc1 == 1 && rc2 == 0 && rc3 == 0 && rc3b == 0 && rc4 == 1) {
		t.Fatalf("成对断言失败：①%d ②%d ③-a%d ③-b%d ④%d", rc1, rc2, rc3, rc3b, rc4)
	}
}

// TestF23R5bKeepsReport —— 门⑬ R5b（rc=2 · 判不了）**仍须**印出已算出的 R5a 只报告行（F23 真修）。
//
// 缺此一格时，本门现读只剩一行「不给结论」—— 谁都判不出「R5a 还剩几条红 / 几条只报告」
// （收口批实测：真仓现读 1 行 ⟷ 把件级账截到最新回执那一刻之后 55 行 ＋ rc=0）。
//
// 夹具（仓外 t.TempDir）：P1（22:00 · 说明动过仓、无件清单）· P2（23:00 · 生成面 docs/**）；
// 件级账两行（21:00 落在 P1 之前 · 09-20T09:00 晚于最新回执 ⇒ R5b 命中）。
//
//	① rc == 2 且含 "R5b"（判不了 · ✗ 不当绿）
//	② 同一次输出里**必须**含 P2 的 R5a「只报告」行（点名 生成面件）
func TestF23R5bKeepsReport(t *testing.T) {
	root, rd := adeFix(t, map[string]map[string]any{
		"R-20260919T220000.json": adeGood("R-20260919T220000", map[string]any{
			"created_at": "2026-09-19T22:00:00+08:00",
			"evidence":   []any{"zerg dev edit --proposal DEV-1"}}),
		"R-20260919T230000.json": adeGood("R-20260919T230000", map[string]any{
			"created_at": "2026-09-19T23:00:00+08:00",
			"evidence":   []any{"zerg dev edit --proposal DEV-1 --file docs/zh/参考-命令行.md"}}),
	})
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir("docs/zh/参考-命令行.md")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/zh/参考-命令行.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adeWriteLedger(t, root, []string{
		`{"at":"2026-09-19T21:00:00+08:00","event":"edit","file":"core/cmd/zerg/check_ai_direct_edits.go"}`,
		`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/cmd/zerg/check_ai_direct_edits.go"}`,
	})
	rc, out, _ := adeRun(t, "--root", root, "--dir", rd)
	if rc != 2 || !strings.Contains(out, "R5b") {
		t.Fatalf("R5b 判不了格失败：得 rc=%d out=%q（应 rc=2 且含 R5b）", rc, out)
	}
	if !strings.Contains(out, "R5a·只报告") || !strings.Contains(out, "docs/zh/参考-命令行.md") {
		t.Fatalf("rc=2 分支吞掉了 R5a 只报告行：得 out=%q（应含 R5a·只报告 且点名 生成面件）", out)
	}
}

// TestF35R5bNoStampFailsClosed —— 门⑬ R5b **无戳页**面（F35 · 用户已拍 A1）：回执页**无任何 `created_at`**
// ⇒ 没有时间基准可比 ⇒ **判 rc=2（不给结论）**；✗ 不得静默跳过该支报绿、✗ 不得折 rc=1。
//
// 失效面（改前实锤 · 仓外夹具真跑）：夹具「页面无 `created_at` ＋ 件级账有更晚主仓写轮」⇒ R5b 整支被跳过
// ⇒ **rc=0「零直改纪律全过」**，且印出一句**不实**的「R5 轮级账 ⟷ 件级账双向对账 ✓（… ≤ 最新回执）」
// （根本没有回执时间可比）。R2 五格**不要求** `created_at` ⇒ 该形态**合法可造**。
//
// 夹具（仓外 t.TempDir）：一页**无 `created_at`** ＋ 件级账一行（主仓写轮 · 时刻晚于任何回执）。
//
//	① rc == 2（✗ 不得折绿、✗ 不得折 rc=1）且**出声**（含 R5b 与「无任何」）
//	② 输出**不得**含「双向对账 ✓」（不实的对账通过行不许印）
func TestF35R5bNoStampFailsClosed(t *testing.T) {
	root, rd := adeFix(t, map[string]map[string]any{
		"R-20260920T000001.json": adeGood("R-20260920T000001", nil), // ✗ 无 created_at（无戳页）
	})
	adeWriteLedger(t, root, []string{
		`{"at":"2026-09-21T09:00:00+08:00","event":"edit","file":"core/x.go"}`,
	})
	rc, out, _ := adeRun(t, "--root", root, "--dir", rd)
	if rc != 2 || !strings.Contains(out, "R5b") || !strings.Contains(out, "无任何") {
		t.Fatalf("无戳页应 rc=2（判不了 · 不得折绿/折 rc=1）且出声：得 rc=%d out=%q", rc, out)
	}
	if strings.Contains(out, "双向对账 ✓") {
		t.Fatalf("无戳页不得印不实的对账通过行：得 out=%q", out)
	}
}

// TestF41NoStampPageFailsClosed —— 门⑬ **邻接两处 fail-open 面**（F41 · 扩 F35／拍定 A1）：任一回执页
// **无 `created_at`** ⇒ 无时间基准可比 ⇒ **判 rc=2（不给结论）**；✗ 不得静默放过、✗ 不得折 rc=1。
//
// 失效面（改前实锤 · 仓外夹具真跑）：① **混有无戳页**（≥1 页带 created_at）⇒ 无戳页在 R5a 被
// `pg.ca == ""` **整页跳过**、F35 支（`len(stamps) == 0`）又不触发 ⇒ **rc=0「零直改纪律全过」**；
// ② **件级账空 ＋ 无戳页** ⇒ `lg != nil && len(lg.ats) > 0` 两块整块不进 ⇒ 连 F35 支都够不着 ⇒ 仍 **rc=0**。
//
// 夹具（仓外 t.TempDir）：
//
//	① 一页带 created_at ＋ 一页无 ＋ 件级账一行（≤ 带戳页时刻 ⇒ R5b「晚于」支不命中）
//	② 一页无 created_at ＋ 空件级账
//	③ 成对：一页带 created_at ＋ 空件级账 ⇒ **仍 rc=0**（既有口径不误伤 · 自检⑮）
func TestF41NoStampPageFailsClosed(t *testing.T) {
	// ① 混有无戳页 ⇒ rc=2 且出声
	rootM, rdM := adeFix(t, map[string]map[string]any{
		"R-20260920T100000.json": adeGood("R-20260920T100000", map[string]any{
			"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}),
		"R-20260920T110000.json": adeGood("R-20260920T110000", nil), // ✗ 无 created_at
	})
	adeWriteLedger(t, rootM, []string{`{"at":"2026-09-20T09:00:00+08:00","event":"edit","file":"core/x.go"}`})
	rcM, outM, _ := adeRun(t, "--root", rootM, "--dir", rdM)
	if rcM != 2 || !strings.Contains(outM, "F41") {
		t.Fatalf("混有无戳页应 rc=2（判不了 · 不得折绿/折 rc=1）且出声：得 rc=%d out=%q", rcM, outM)
	}
	if strings.Contains(outM, "双向对账 ✓") {
		t.Fatalf("混有无戳页不得印不实的对账通过行：得 out=%q", outM)
	}
	// ② 空件级账 ＋ 无戳页 ⇒ rc=2
	rootE, rdE := adeFix(t, map[string]map[string]any{
		"R-20260920T100000.json": adeGood("R-20260920T100000", nil), // ✗ 无 created_at
	})
	adeWriteLedger(t, rootE, nil)
	rcE, outE, _ := adeRun(t, "--root", rootE, "--dir", rdE)
	if rcE != 2 || !strings.Contains(outE, "F41") {
		t.Fatalf("空件级账 + 无戳页应 rc=2（判不了），得 rc=%d out=%q", rcE, outE)
	}
	// ③ 成对（✗ 禁恒绿）：空件级账 ＋ **全带戳**页 ⇒ 仍 rc=0（既有口径不误伤）
	rootS, rdS := adeFix(t, map[string]map[string]any{
		"R-20260920T100000.json": adeGood("R-20260920T100000", map[string]any{
			"created_at": "2026-09-20T10:00:00+08:00", "evidence": []any{"zerg dev edit --proposal DEV-1"}}),
	})
	adeWriteLedger(t, rootS, nil)
	rcS, outS, _ := adeRun(t, "--root", rootS, "--dir", rdS)
	if rcS != 0 {
		t.Fatalf("空件级账 + 全带戳页应仍 rc=0（既有口径不误伤），得 rc=%d out=%q", rcS, outS)
	}
}
