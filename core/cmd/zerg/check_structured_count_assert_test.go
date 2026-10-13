// check_structured_count_assert_test.go —— `check_structured_count_assert.go`（门128）的回归牙。
//
// 覆盖五面：
//
//	① `--self-test` 真跑 rc=0（报格数）；
//	② 真仓普查：rc ∈ {0,1} · 输出含权威读数行（**有锚声明行** 现读复算）· 期望值**从真源现读派生**；
//	③ 扫描面缺件 ⇒ rc=2 且明说「不给结论」（fail-closed · 属性 ④）；
//	④ 未知旗标 ⇒ rc=2（用法错）；
//	⑤ 权威读数下界（棘轮）冻结常量 = 254（**随源重钉值** · 见下「重钉记录」）。
//
// ── 重钉记录（冻结下界 `cscaAuthoritativeBaseline`）─────────────────────────────
//
//	2026-10-14 · 253 → 254（本单＝治存量红 `TestCscaRepoCensus`）：
//	· 症状：他路在 `scripts/gates/precommit-gates.sh` 加了一步（`add_step` 声明行），而本常量与
//	  ② 里手写的期望值（253）未同批刷新 ⇒ 真源现读 254 ≠ 期望 253 ⇒ 该用例存量红。
//	· 真源现读（仓根跑）：
//	  ① `grep -cE '^[[:space:]]*add_step[[:space:]]' scripts/gates/precommit-gates.sh` ⇒ 255（含本门那一步）
//	  ② 同式接 `| grep -vc check-structured-count-assert` ⇒ 254（**权威读数** = 自排除本门那一步）
//	  ③ `grep -cE 'add_step [a-z]* "' scripts/gates/precommit-gates.sh` ⇒ 261（naive · 门内自排除后 260）
//	  ④ `grep -o 'add_step' scripts/gates/precommit-gates.sh | wc -l` ⇒ 319（全 token · 门内自排除后 318）
//	· 判定：**真源真变了**（+1 步），不是常量凭空漂 ⇒ 常量随源重钉（Go `const` 读不了盘 ⇒ 无法派生）。
//	· 治本：② 的期望值改成**现读派生**（`cscaCountReadings` 读真源 · 与门内同函数 · 同自排除口径）
//	  ＋ 下界不变式（真源现读 ≥ 冻结下界 ⇒ 真源真变小必红）⇒ 以后他路再加步**不再把牙顶红**。
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// cscaRepoRoot —— 真仓根（本测试件的运行目录 = 包目录 ⇒ 仓根在上三级）。
func cscaRepoRoot() string { return filepath.Join("..", "..", "..") }

// cscaRepoAnchorReading —— ★ 从**真源现读派生**权威读数：与门内 `cscaCensus` 同函数（`cscaCountReadings`）、
// 同自排除口径 ⇒ 期望值不手写，真源一变 / 读数实现一变它就跟着变。真源取不到 ⇒ 直接判红（✗ 不静默回落）。
func cscaRepoAnchorReading(t *testing.T) int {
	t.Helper()
	text, ok := cscaReadRel(cscaRepoRoot(), zergPrecommitGatesRel)
	if !ok {
		t.Fatalf("真源取不到 ⇒ 派生不成立（✗ 不给结论）：%s", zergPrecommitGatesRel)
	}
	return cscaCountReadings(text).anchor
}

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

// ② 真仓普查：rc ∈ {0,1} · 权威读数行在 · ★ 期望值现读派生 ＋ 下界不变式（真源真变小 ⇒ 红）。
func TestCscaRepoCensus(t *testing.T) {
	rc, so, _ := cscaInvoke("--root", cscaRepoRoot())
	if rc != 0 && rc != 1 {
		t.Fatalf("真仓普查 rc=%d want 0/1（只报告档）", rc)
	}
	if !strings.Contains(so, "权威读数（有锚声明行 `add_step`）") {
		t.Fatalf("输出缺「权威读数」行：%q", so)
	}
	// ★ 期望值**现读派生**（✗ 不手写数字 · 手写必随他路加步而漂）：门内印的数 ⟷ 真源现读派生的数，
	//   二者不符即红（读数实现坏掉 / 被写死都躲不过）。
	want := cscaRepoAnchorReading(t)
	if !strings.Contains(so, fmt.Sprintf("＝ **%d**", want)) {
		t.Fatalf("权威读数应现读复算为 %d（真源 %s · 有锚声明行 · 自排除本门那一步）：%q",
			want, zergPrecommitGatesRel, so)
	}
	// 棘轮不变式：真源现读**不得低于**冻结下界（真源真变小 ⇒ 本条红 · 与门内属性 ② 同口径）。
	if want < cscaAuthoritativeBaseline {
		t.Fatalf("真源现读 %d < 冻结下界 %d ⇒ 结构静默变小（真变 · 门内同步 rc=1）：%s",
			want, cscaAuthoritativeBaseline, zergPrecommitGatesRel)
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

// ⑤ 权威读数下界（棘轮）冻结常量：**随源重钉值**（依据与命令见文件头「重钉记录」）。
func TestCscaBaselineFrozen(t *testing.T) {
	// ★ 本格钉住的是「重钉后的值」——✗ 不许静默调松（调松 = 棘轮无牙：结构少一步也不红）。
	//   派生不成立（Go `const` 读不了盘）⇒ 只能显式重钉：重钉时同批改「常量 ＋ 本格 ＋ 文件头记录」。
	if cscaAuthoritativeBaseline != 254 {
		t.Fatalf("权威读数下界冻结值应 254（真源现读重钉 · 见文件头「重钉记录」），得 %d", cscaAuthoritativeBaseline)
	}
	// ★ 且它必须是**真下界**：真源现读 ≥ 本值（否则真仓必红 · 与门内属性 ② 同口径）。
	if live := cscaRepoAnchorReading(t); live < cscaAuthoritativeBaseline {
		t.Fatalf("真源现读 %d < 冻结下界 %d ⇒ 该值已不是下界", live, cscaAuthoritativeBaseline)
	}
}
