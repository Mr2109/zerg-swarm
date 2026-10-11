// check_structured_count_assert_test.go —— `check_structured_count_assert.go`（门128）的回归牙。
//
// 覆盖五面：
//
//	① `--self-test` 真跑 rc=0（报格数）；
//	② 真仓普查：rc ∈ {0,1} · 输出含权威读数行（**有锚声明行** 现读复算）与参考基线 251；
//	③ 扫描面缺件 ⇒ rc=2 且明说「不给结论」（fail-closed · 属性 ④）；
//	④ 未知旗标 ⇒ rc=2（用法错）；
//	⑤ 权威读数下界（棘轮）冻结常量 = 251（本批加本门步前现读复算值 · 只许降）。
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func cscaInvoke(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkStructuredCountAssertGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// ① 自检真跑 rc=0（报格数）。
func TestCscaSelfTestRC0(t *testing.T) {
	rc, so, se := cscaInvoke("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test rc=%d want 0; out=%q err=%q", rc, so, se)
	}
	if !strings.Contains(so, "自检 PASS（9 格）") {
		t.Fatalf("自检未报格数「9 格」/未过：%q", so)
	}
}

// ② 真仓普查：rc ∈ {0,1} · 权威读数行在 · 参考基线 251 在。
func TestCscaRepoCensus(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	rc, so, _ := cscaInvoke("--root", root)
	if rc != 0 && rc != 1 {
		t.Fatalf("真仓普查 rc=%d want 0/1（只报告档）", rc)
	}
	if !strings.Contains(so, "权威读数（有锚声明行 `add_step`）") {
		t.Fatalf("输出缺「权威读数」行：%q", so)
	}
	if !strings.Contains(so, "＝ **251**") {
		t.Fatalf("权威读数应现读复算为 251（有锚声明行 · 自排除本门那一步）：%q", so)
	}
	if !strings.Contains(so, "对照读法：naive") {
		t.Fatalf("输出缺三读法对照行：%q", so)
	}
}

// ③ 扫描面缺件 ⇒ rc=2（fail-closed）。
func TestCscaMissingRootBlocked(t *testing.T) {
	rc, _, se := cscaInvoke("--root", filepath.Join(t.TempDir(), "no-such-root"))
	if rc != 2 || !strings.Contains(se, "不给结论") {
		t.Fatalf("缺扫描面应 rc=2 且明说「不给结论」：rc=%d err=%q", rc, se)
	}
}

// ④ 未知旗标 ⇒ rc=2。
func TestCscaUnknownFlagBlocked(t *testing.T) {
	rc, _, _ := cscaInvoke("--bogus")
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
}

// ⑤ 权威读数下界（棘轮）冻结常量。
func TestCscaBaselineFrozen(t *testing.T) {
	if cscaAuthoritativeBaseline != 251 {
		t.Fatalf("权威读数下界冻结值应 251（有锚声明行读法 · 本批加本门步前现读复算），得 %d", cscaAuthoritativeBaseline)
	}
}
