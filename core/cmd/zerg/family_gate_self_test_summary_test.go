// family_gate_self_test_summary_test.go —— `zerg gate self-test-summary` 的集中解析与三态的回归牙
// （缺口 `GAP-20260926-183` · 路 B）。表里每一条都是**从真判据的现读末行**取来的（probe 现读 181 件
// 的末行），外加两条**负控**（无数字 / 真失败）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestGateSelfTestSummaryParseTable —— 末行解析表（真格式 ⇒ 读数；无数字 ⇒ 判不了）。
func TestGateSelfTestSummaryParseTable(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantGrids int
		wantFails int
		wantOK    bool
	}{
		// ---- 有并列读数的真格式（正控族）----
		{"gsts:规范格失败", "self-test：12 格 · 0 失败", 12, 0, true},
		{"格·失败", "自检 28 格 · 失败 0 格", 28, 0, true},
		{"断言条·失败条", "── 自检结论：断言 19 条 · 失败 0 条 ──", 19, 0, true},
		{"格过·格不过", "自检：16 格过 · 0 格不过（真目标只读 · 未改任何字节）", 16, 0, true},
		{"过/红", "自检：74 过 / 0 红（74 条用例 = 1 判据面 + 1 清洁）", 74, 0, true},
		{"格过·格红", "—— 自检：15 格过 · 0 格红（夹具 /tmp/x）——", 15, 0, true},
		{"全过（断言N条·失败0）", "自检结论: 全过（断言 69 条 · 失败 0 条）", 69, 0, true},
		{"条通过/M条失败", "self-test：45 条通过 / 0 条失败", 45, 0, true},
		{"项通过，M项失败", "自测：71 项通过，0 项失败", 71, 0, true},
		{"用例·败", "⇒ 自检 全过（rc=0）（5 用例 · 0 败）", 5, 0, true},
		{"条·通过条", "自证用例：33 条 · 通过 33 条（少跑即失效）", 33, 0, true},
		{"分式N/N", "self-test: 6/6", 6, 0, true},
		{"分式带通过", "结果: 通过（20/20 条断言通过）  rc=0", 20, 0, true},
		{"用例条，失败条", "自检结论: 全过（用例 5 条，失败 0 条）", 5, 0, true},
		{"全过（N组）", "✓ 自检全过（11 组：正控 1 + 负控 10 · 成对）", 11, 0, true},
		{"PASS（N格）", "自检 PASS（5 格）", 5, 0, true},
		{"通过✓（N格）", "自检：通过 ✓（15 格：必红×2 · 豁免×1）", 15, 0, true},
		{"N格全过", "── 自检 13 格全过（3 负控必命中）──", 13, 0, true},
		{"合计N格", "合计 6 格 · 全绿", 6, 0, true},
		{"N控", "自检通过：7 控（正控 3 ＋ 负控 4）逐条对拍 ✓", 7, 0, true},
		{"通过（N条）", "self-test 通过（6 条正/负用例）", 6, 0, true},
		{"全过（断言**N**格·失败M格）", "自检结论: 全过（断言 **16** 格 · 失败 0 格）", 16, 0, true},

		// ---- 真失败（读数非 0）----
		{"真失败·断言5失败2", "自检结论: 全过（断言 5 条 · 失败 2 条）", 5, 2, true},
		{"真失败·格过1不过1", "自检：1 格过 · 1 格不过（真目标只读）", 2, 1, true},

		// ---- 负控族：**无数字**的末行 ⇒ 判不了（✗ 不许静默记 0）----
		{"无数字·有断言失败", "自检结论: **有断言失败**", 0, 0, false},
		{"无数字·全过", "自检结论: 全过", 0, 0, false},
		{"无数字·全过带装饰", "── 自检：全过 ✓", 0, 0, false},
		{"无数字·全过（rc=0）", "⇒ 自检 全过（rc=0）", 0, 0, false},
		{"无数字·全绿", "自检 全绿", 0, 0, false},
		{"无数字·PASS", "自检 PASS", 0, 0, false},
		{"负控·百分比表不当分式", "自测全绿（已知答案：0/6=39.30%、0/20=13.91%、1/20=21.61%）", 0, 0, false},
		{"空行", "", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, f, ok := gateSelfTestSummaryParse(c.line)
			if ok != c.wantOK || g != c.wantGrids || f != c.wantFails {
				t.Fatalf("parse(%q) = (%d,%d,%v)，要 (%d,%d,%v)", c.line, g, f, ok, c.wantGrids, c.wantFails, c.wantOK)
			}
		})
	}
}

// TestGateSelfTestSummaryClassifyThreeStates —— 三态：通过 / 失败 / 判不了。
func TestGateSelfTestSummaryClassifyThreeStates(t *testing.T) {
	if st, g, f, _ := gateSelfTestSummaryClassify("  \nself-test：12 格 · 0 失败\n\n"); st != gstsPass || g != 12 || f != 0 {
		t.Fatalf("通过态失配：%s %d %d", st, g, f)
	}
	if st, g, f, _ := gateSelfTestSummaryClassify("自检结论: 全过（断言 5 条 · 失败 2 条）\n"); st != gstsFail || g != 5 || f != 2 {
		t.Fatalf("失败态失配：%s %d %d", st, g, f)
	}
	// 负控：无数字末行 ⇒ 判不了（不是通过、也不是失败）。
	if st, g, f, _ := gateSelfTestSummaryClassify("自检结论: **有断言失败**\n"); st != gstsUnknown || g != 0 || f != 0 {
		t.Fatalf("判不了态失配（无数字末行必须判不了）：%s %d %d", st, g, f)
	}
	// 末行判定：只取最后一条非空行。
	if _, _, _, s := gateSelfTestSummaryClassify("中间行\n\n最后一行\n  \n"); s != "最后一行" {
		t.Fatalf("末行判定失配：%q", s)
	}
}

// TestGateSelfTestSummarySelfTestControlsGreen —— 命令自带自检（正控 2 + 负控 3）必须全过。
func TestGateSelfTestSummarySelfTestControlsGreen(t *testing.T) {
	var out, errb bytes.Buffer
	rc := gateSelfTestSummarySelfTest(&out, &errb)
	if rc != exitOK {
		t.Fatalf("自检应 rc=0，得 rc=%d\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "self-test：") || !strings.Contains(out.String(), "0 失败") {
		t.Fatalf("自检末行缺「0 失败」：\n%s", out.String())
	}
}

// TestGateSelfTestSummaryRealJudgePositive —— 正控：真判据 check-doc-consumers 的自检能被机器读到
// 格数>0 且态=通过（端到端：跑真自检 + 解析真末行）。
func TestGateSelfTestSummaryRealJudgePositive(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("仓根解析不到（非仓内跑）")
	}
	fn := gateCheckGoEntry["check-doc-consumers"]
	if fn == nil {
		t.Fatal("check-doc-consumers 不在进程内判据臂键表")
	}
	out, _, timedOut, crashed := gateSelfTestSummaryRunOne(root, "check-doc-consumers", fn)
	if timedOut || crashed {
		t.Fatalf("真判据自检超时/崩：timedOut=%v crashed=%v", timedOut, crashed)
	}
	st, g, f, _ := gateSelfTestSummaryClassify(out)
	if st != gstsPass || g <= 0 || f != 0 {
		t.Fatalf("真判据自检读数失配：态=%s 格数=%d 失败=%d（要 pass/正数/0）\n末行上下文：%s", st, g, f, gateSelfTestSummaryLastLine(out))
	}
}

// TestGateSelfTestSummaryNegativeControlRed — 负控**真红**：把解析器退回「无数字 ⇒ 记 0」的假绿形态时，
// 自检必须变红。这里直接断言负控输入落到「判不了」，即那两格的牙口真实存在。
func TestGateSelfTestSummaryNegativeControlRed(t *testing.T) {
	// 若解析器把「有断言失败」误判成可解析（记 0），这两条会红。
	if _, _, ok := gateSelfTestSummaryParse("自检结论: **有断言失败**"); ok {
		t.Fatal("负控①失效：无数字的失败末行被判成可解析（＝假绿）")
	}
	if _, g, f, _ := gateSelfTestSummaryClassify("自检结论: 全过（断言 5 条 · 失败 2 条）\n"); g != 5 || f != 2 {
		t.Fatalf("负控②失效：真失败末行的读数被读歪（%d,%d）", g, f)
	}
}
