// gate_ls_optfields_parity_test.go —— `gate ls` 的**可选字段层**两张手抄表逐字对拍
// （缺口 `GAP-20261010-13` · 本片）：`gate.go` 的 `gateLsOptFields` ⟷ `main.go` 命令树里
// `gate ls` 登记格的 `optFields`。两表**手抄** ⇒ 可**静默漂移**（`gate ls` 不走
// `emitSelected`/`--help`/合法字段面 ⇒ 漂了也**无观测差** = 语义债）⇒ 本判据把漂移
// **变可见**：缺一即红（两向：任一表多/少一格都红）。
//
// 形制照同族先例 `core/internal/api/routes_test.go` 的 `TestRouteTableMatchesRealRoutes`
// （「表 ⟷ 真源逐条命中」）—— 但本判据与两张表**同包** ⇒ 直接读真源（`find` 与
// `gateLsOptFields`），✗ 不扫文本、✗ 不另抄一份。
package main

import "testing"

// gateLsOptFieldsEqual —— 两张可选字段表**逐字**比对（序 + 值逐格同 ⇒ true）。
// 提成纯函数 ⇒ 成对负控能直接喂合成对（哑化本比对 ⇒ 负控对红）。
func gateLsOptFieldsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestGateLsOptFieldsParity —— `gate.go` 的 `gateLsOptFields` ⟷ `main.go` 里 `gate ls` 的
// `optFields` 两表逐字一致。缺一即红。
func TestGateLsOptFieldsParity(t *testing.T) {
	cmd := find([]string{"gate", "ls"})
	if cmd == nil {
		t.Fatal("命令树里找不到 `gate ls`（登记格缺失 ⇒ 判据不成立）")
	}
	if !gateLsOptFieldsEqual(gateLsOptFields, cmd.optFields) {
		t.Fatalf("两表漂移（`gate.go` 的 gateLsOptFields ⟷ `main.go` 里 `gate ls` 的 optFields 逐字不一致）：\n"+
			"  gate.go:366  gateLsOptFields      = %q\n"+
			"  main.go:788  gate ls 登记 optFields = %q",
			gateLsOptFields, cmd.optFields)
	}
	t.Logf("两表逐字一致：gateLsOptFields = optFields = %q", gateLsOptFields)
}

// TestGateLsOptFieldsEqualPaired —— 成对负控：一致 ⇒ true；任一表多/少一格 ⇒ false。
// 哑化 `gateLsOptFieldsEqual`（恒 true / 恒 false）⇒ 本用例必红（证明主判据的比对有牙）。
func TestGateLsOptFieldsEqualPaired(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"两表逐字同 ⇒ 一致", []string{"name", "gates_total"}, []string{"name", "gates_total"}, true},
		{"右表缺一格 ⇒ 不一致", []string{"name", "gates_total"}, []string{"name"}, false},
		{"左表缺一格 ⇒ 不一致", []string{"name"}, []string{"name", "gates_total"}, false},
		{"值同序不同 ⇒ 不一致", []string{"name", "gates_total"}, []string{"gates_total", "name"}, false},
	}
	for _, c := range cases {
		if got := gateLsOptFieldsEqual(c.a, c.b); got != c.want {
			t.Errorf("%s：gateLsOptFieldsEqual(%q, %q) = %v，要 %v", c.name, c.a, c.b, got, c.want)
		}
	}
}
