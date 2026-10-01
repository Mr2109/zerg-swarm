// bench_summary_test.go —— Go 真搬件的**成对负控**定向单测（M2 路 A 续2 · 4.2「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿（有表）」「必然判红/不给结论（各 violation + 用法错）」两类 ⇒
// 任一态被写死必失败。用例名：TestBenchSummaryGatePort。
// 定向跑：go test -mod=mod GOSUMDB=off ./cmd/zerg -run TestBenchSummaryGatePort -count=1
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bsRun —— 进程内直调入口取（rc, stdout, stderr）。
func bsRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := benchSummaryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func bsWriteJSON(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bench.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBenchSummaryGatePort(t *testing.T) {
	// ── 正控甲（必然判绿）：--json 两趟同值 ⇒ rc=0 且中位 24.763s ──────────────
	green := bsWriteJSON(t, `{"items":[{"real_ms":"24763"},{"real_ms":"24763"}]}`)
	rcGreen, outGreen, errGreen := bsRun(t, "--json", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "中位 real = 24.763s") {
		t.Fatalf("正控甲失败：好件应 rc=0/24.763s，得 rc=%d out=%q err=%q", rcGreen, outGreen, errGreen)
	}

	// ── 正控乙（必然判绿）：--tier 前缀逐字落在结论前 ────────────────────────
	rcTier, outTier, _ := bsRun(t, "--json", green, "--tier", "daily")
	if rcTier != 0 || !strings.HasPrefix(outTier, "档位=daily\n") {
		t.Fatalf("正控乙失败：--tier 应前置 `档位=daily`，得 rc=%d out=%q", rcTier, outTier)
	}

	// ── 正控丙（必然判绿）：--from-results 解析 results.tsv ⇒ rc=0/合计 18s ───
	rdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rdir, "results.tsv"),
		[]byte("PASS\ta-step\t0\t3s\t/x.log\nFAIL\tb-step\t1\t12s\t/y.log\n"+
			"PASS\tc-step\t0\t3s\t/z.log\nPASS\td-step\t0\t-\t/w.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcRes, outRes, _ := bsRun(t, "--from-results", rdir)
	if rcRes != 0 || !strings.Contains(outRes, "合计 18s（4 步）") {
		t.Fatalf("正控丙失败：results.tsv 应 rc=0/合计 18s，得 rc=%d out=%q", rcRes, outRes)
	}
	if !strings.Contains(outRes, "│ 12s  1      b-step") || !strings.Contains(outRes, "│ 3s   0      a-step") {
		t.Fatalf("正控丙失败：稳定排序/等宽填充不一致，out=%q", outRes)
	}

	// ── 负控①：闭集外字段 `secs` ⇒ rc=2（接口自证） ────────────────────────
	banned := bsWriteJSON(t, `{"items":[{"real_ms":"1","secs":"2"}]}`)
	rcBan, outBan, _ := bsRun(t, "--json", banned)
	if rcBan != 2 || !strings.Contains(outBan, "未知字段 `secs`（接口闭集 log,rc,real_ms,run,sys_ms,user_ms）") {
		t.Fatalf("负控①失败：闭集外字段应 rc=2 并点名，得 rc=%d out=%q", rcBan, outBan)
	}

	// ── 负控②：缺 real_ms ⇒ rc=2（C0 接口自证） ────────────────────────────
	miss := bsWriteJSON(t, `{"items":[{"run":"x"}]}`)
	rcMiss, outMiss, _ := bsRun(t, "--json", miss)
	if rcMiss != 2 || !strings.Contains(outMiss, "第 0 趟缺 real_ms") {
		t.Fatalf("负控②失败：缺 real_ms 应 rc=2，得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 负控③：real_ms 不是整数 ⇒ rc=2（repr 逐字） ───────────────────────
	nonint := bsWriteJSON(t, `{"items":[{"real_ms":"1.5"}]}`)
	rcNon, outNon, _ := bsRun(t, "--json", nonint)
	if rcNon != 2 || !strings.Contains(outNon, "不是整数：'1.5'") {
		t.Fatalf("负控③失败：非整数应 rc=2 且 repr 逐字，得 rc=%d out=%q", rcNon, outNon)
	}

	// ── 负控④：items 空（空转 ⇒ 不许当绿）⇒ rc=2 ──────────────────────────
	empty := bsWriteJSON(t, `{"items":[]}`)
	rcEmpty, outEmpty, _ := bsRun(t, "--json", empty)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "空转不许当绿") {
		t.Fatalf("负控④失败：空转应 rc=2，得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 负控⑤：取不到输入（--json 件不在盘）⇒ rc=2 且走 stderr ────────────
	rcNo, outNo, errNo := bsRun(t, "--json", filepath.Join(t.TempDir(), "nope.json"))
	if rcNo != 2 || outNo != "" || !strings.Contains(errNo, "✗ 缺件 ") {
		t.Fatalf("负控⑤失败：缺件应 rc=2/仅 stderr，得 rc=%d out=%q err=%q", rcNo, outNo, errNo)
	}

	// ── 用法错①：未知旗标 ⇒ rc=2 且 argparse 报话逐字 ──────────────────────
	rcUF, outUF, errUF := bsRun(t, "--no-such-flag")
	if rcUF != 2 || outUF != "" || !strings.Contains(errUF, "error: unrecognized arguments: --no-such-flag") {
		t.Fatalf("用法错①失败：得 rc=%d out=%q err=%q", rcUF, outUF, errUF)
	}

	// ── 用法错②：`--json` 缺值 ⇒ rc=2 ─────────────────────────────────────
	rcNV, _, errNV := bsRun(t, "--json")
	if rcNV != 2 || !strings.Contains(errNV, "argument --json: expected one argument") {
		t.Fatalf("用法错②失败：得 rc=%d err=%q", rcNV, errNV)
	}

	// ── 用法错③：无参 ⇒ rc=2 且印用法 ─────────────────────────────────────
	rcNA, _, errNA := bsRun(t)
	if rcNA != 2 || !strings.Contains(errNA, "用法：bench-summary.py --json") {
		t.Fatalf("用法错③失败：得 rc=%d err=%q", rcNA, errNA)
	}

	// ── 自检档：--self-test ⇒ rc=0 且文案逐字 ─────────────────────────────
	rcST, outST, _ := bsRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检通过：5 控（正控 2 + 负控 3）逐条对拍 ✓") {
		t.Fatalf("自检档失败：得 rc=%d out=%q", rcST, outST)
	}

	// ── 帮助档：-h ⇒ rc=0 且帮助正文逐字 ──────────────────────────────────
	rcH, outH, errH := bsRun(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "optional arguments:") || errH != "" {
		t.Fatalf("帮助档失败：得 rc=%d out=%q err=%q", rcH, outH, errH)
	}

	// ── 成对断言（✗ 禁恒绿）：绿档 rc 与各负控 rc 必须不同码 ─────────────────
	if rcGreen == rcBan || rcGreen == rcMiss || rcGreen == rcEmpty {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc 同码 ⇒ 无鉴别力", rcGreen)
	}
	if rcRes == rcGreen && rcRes != 0 {
		t.Fatalf("成对断言失败：results 档 rc 异常 =%d", rcRes)
	}
}
