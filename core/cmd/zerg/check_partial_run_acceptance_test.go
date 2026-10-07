// check_partial_run_acceptance_test.go —— 判据件 `check-partial-run-acceptance` Go 真搬的**成对负控**测试件。
//
// 纪律：合成夹具全在 t.TempDir()（仓外）· ✗ 不碰仓内件 · ✗ 不碰 bin/zerg · ✗ 不定义 TestMain（避免与同包他件撞名）。
//
//	★ 每对「正/负」成对：正控断言语义成立，负控断言其反例**不**被误放行（有牙）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// praTestRunSelfTest —— 跑本件自检档，取 rc ＋ 正文。
func praTestRunSelfTest() (int, string) {
	var b bytes.Buffer
	rc := praSelfTest(&b)
	return rc, b.String()
}

// TestPartialRunAcceptanceSelfTestGreen —— 自检档 13 控 ＋ 13 变异全过（参考模型绿）。
func TestPartialRunAcceptanceSelfTestGreen(t *testing.T) {
	rc, out := praTestRunSelfTest()
	if rc != 0 {
		t.Fatalf("自检 rc=%d want 0\n%s", rc, out)
	}
	for _, want := range []string{"参考模型：13/13 全过", "断言 27 条 · 失败 0 条"} {
		if !strings.Contains(out, want) {
			t.Fatalf("自检输出缺 %q\n%s", want, out)
		}
	}
}

// TestPartialRunAcceptancePartialNeverRC0 —— 成对：部分趟全绿=3（正）／有 FAIL=1（负）／BLOCKED=2（负）；
//
//	并断言**任何**结局都不得退 0（D2 语义 · 「部分趟绝不退 0」）。
func TestPartialRunAcceptancePartialNeverRC0(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	if ok, d := praCtlA1(ctx, ""); !ok {
		t.Fatalf("A1 正控应过 detail=%s", d)
	}
	if ok, d := praCtlA2(ctx, ""); !ok {
		t.Fatalf("A2 负控应过 detail=%s", d)
	}
	if ok, d := praCtlA3(ctx, ""); !ok {
		t.Fatalf("A3 负控应过 detail=%s", d)
	}
	fx := ctx["A1"]
	for _, stt := range []string{"PASS", "FAIL", "BLOCKED", "REPORT"} {
		f := fx
		f.rStatuses = map[string]string{"step-00": stt}
		if r := praDecide(f, ""); r.rc == 0 {
			t.Fatalf("部分趟退化为 rc=0（stt=%s）⇒ 绝不退 0 被破", stt)
		}
	}
}

// TestPartialRunAcceptanceThresholdBoundary —— 阀边界：恰 60.0% ⇒ 全量 ／ 50.0% ⇒ 部分趟。
func TestPartialRunAcceptanceThresholdBoundary(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	if ok, d := praCtlB3(ctx, ""); !ok {
		t.Fatalf("B3 边界控应过 detail=%s", d)
	}
}

// TestPartialRunAcceptanceFailClosedPaths —— C 组 fail-closed：步面对齐/缺 freshness/歧义/部分基线 皆 rc=2。
func TestPartialRunAcceptanceFailClosedPaths(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	for _, id := range []string{"C1", "C2", "C3", "C4"} {
		c, ok := praCtlByID(id)
		if !ok {
			t.Fatalf("缺控 %s", id)
		}
		if passed, d := c.fn(ctx, ""); !passed {
			t.Fatalf("控 %s 应过 detail=%s", id, d)
		}
	}
}

// TestPartialRunAcceptanceTransitionsFull —— B 组转全量：undeclared ／ 五真源 ／ 无 --since 皆 mode=full。
func TestPartialRunAcceptanceTransitionsFull(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	for _, id := range []string{"A4", "B1", "B2"} {
		c, ok := praCtlByID(id)
		if !ok {
			t.Fatalf("缺控 %s", id)
		}
		if passed, d := c.fn(ctx, ""); !passed {
			t.Fatalf("控 %s 应过 detail=%s", id, d)
		}
	}
}

// TestPartialRunAcceptanceLastPointerFull —— D1：并存时 last 只指全量目录。
func TestPartialRunAcceptanceLastPointerFull(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	if ok, d := praCtlD1(ctx, ""); !ok {
		t.Fatalf("D1 应过 detail=%s", d)
	}
}

// TestPartialRunAcceptanceArgHelpAndUnknown —— argparse 面：-h 出帮助 rc=0 ／ 未知旗标 rc=2。
func TestPartialRunAcceptanceArgHelpAndUnknown(t *testing.T) {
	var o, e bytes.Buffer
	rc := checkPartialRunAcceptanceGateMain(&o, &e, []string{"-h"})
	if rc != 0 || !strings.Contains(o.String(), "optional arguments:") {
		t.Fatalf("help rc=%d out=%s", rc, o.String())
	}
	o.Reset()
	e.Reset()
	rc = checkPartialRunAcceptanceGateMain(&o, &e, []string{"--nope"})
	if rc != 2 || !strings.Contains(e.String(), "unrecognized arguments: --nope") {
		t.Fatalf("unknown rc=%d err=%s", rc, e.String())
	}
	// 未知旗标负控：合法旗标不得被判未识别
	o.Reset()
	e.Reset()
	rc = checkPartialRunAcceptanceGateMain(&o, &e, []string{"--list-rules"})
	if rc != 0 || strings.Contains(e.String(), "unrecognized") {
		t.Fatalf("list-rules rc=%d err=%s", rc, e.String())
	}
}

// TestPartialRunAcceptanceListRulesStable —— 规则表含 13 条控全名与三值退码表。
func TestPartialRunAcceptanceListRulesStable(t *testing.T) {
	var o bytes.Buffer
	if rc := checkPartialRunAcceptanceGateMain(&o, &bytes.Buffer{}, []string{"--list-rules"}); rc != 0 {
		t.Fatalf("list-rules rc=%d", rc)
	}
	out := o.String()
	for _, id := range []string{"A1", "A2", "A3", "A4", "B1", "B2", "B3", "C1", "C2", "C3", "C4", "D1", "D2"} {
		if !strings.Contains(out, "  "+id+"（") {
			t.Fatalf("规则表缺控 %s", id)
		}
	}
	if !strings.Contains(out, "  2  BLOCKED（不给结论）") {
		t.Fatalf("规则表缺退码表")
	}
}

// TestPartialRunAcceptanceMutationsCaught —— 负控自证的有牙版：每条控的变异必须使其翻红。
func TestPartialRunAcceptanceMutationsCaught(t *testing.T) {
	ctx := praBuildCtx(t.TempDir())
	for _, c := range praControls {
		mut := praMutations[c.id]
		if ok, _ := praSafeCtl(c, ctx, mut); ok {
			t.Fatalf("控 %s 在变异 %s 下仍绿 ⇒ 该控无牙", c.id, mut)
		}
	}
}

// praCtlByID —— 按 id 取控（测试辅助）。
func praCtlByID(id string) (praCtl, bool) {
	for _, c := range praControls {
		if c.id == id {
			return c, true
		}
	}
	return praCtl{}, false
}
