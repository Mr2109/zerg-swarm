package main

// family_cocoon_egg_test.go —— A12 归名（决-13 · `zerg cocoon pack|info|verify|xfer`）＋
// 换件三 action 补 D3（`回执-A档批1-换件面-A2A3-20261007.md` §四「写面 · 待 D3」）的**成对负控测试件**。
//
// 三组牙（哑化任一 ⇒ 本件必 FAIL 带行号）：
//  ① `catalog()` 里 `cocoon pack|info|verify|xfer` 四条在册且挂 `cmdCocoonEgg`（哑化＝删登记 ⇒ 红）；
//  ② `cmdCocoonEgg` 真转发到 `zergEggGateMain`（哑化＝不转发 / 吞退码 ⇒ 红）；
//  ③ 换件三 action（`swap core`/`swap agent`/`swap ui`）挂 `dangerSpec D3` 且 `opened: true`
//     （哑化＝不挂 danger ⇒ 红；且 `opened` 只能为真 —— runner 是真 exec 壳、不在 `refusingRunners` 自拒闭集里）。

import (
	"strings"
	"testing"
)

// ① A12 归名：四条在册 + run 非空 + passthrough（只转发不翻译）。
func TestCocoonEggRegistered(t *testing.T) {
	subs := []string{"pack", "info", "verify", "xfer"}
	got := map[string]bool{}
	for _, c := range catalog() {
		if len(c.path) != 2 || c.path[0] != "cocoon" {
			continue
		}
		for _, s := range subs {
			if c.path[1] != s {
				continue
			}
			got[s] = true
			if c.run == nil {
				t.Fatalf("`zerg cocoon %s` 在册但 run 为 nil", s)
			}
			if !c.passthrough {
				t.Errorf("`zerg cocoon %s` 应是 passthrough（只转发、不翻译）", s)
			}
		}
	}
	for _, s := range subs {
		if !got[s] {
			t.Fatalf("`zerg cocoon %s` 不在命令树里 ⇒ A12 归名未落（决-13）", s)
		}
	}
}

// ② 转发面：`cmdCocoonEgg` 把族动作名 ＋ 其后的一切原样交给 `zergEggGateMain`（连退码也不改写）。
func TestCocoonEggForwardsToEggGateMain(t *testing.T) {
	// 未知旗标：转发给真搬体 ⇒ argparse 式 rc=2（exitUsage）+ stderr `unrecognized arguments`。
	inv := &invocation{path: []string{"cocoon", "pack"}, orig: []string{"cocoon", "pack", "--bogus-zz"}}
	var o, e strings.Builder
	if rc := cmdCocoonEgg(inv, &o, &e); rc != exitUsage {
		t.Fatalf("cocoon pack --bogus-zz rc=%d 期望 %d（转发面）", rc, exitUsage)
	}
	if !strings.Contains(e.String(), "unrecognized arguments: --bogus-zz") {
		t.Fatalf("转发的 stderr 不含 argparse 判词：%q", e.String())
	}
	// 缺实参：转发 ⇒ 真搬体的 `pack` 用法错（zegExitToolUsage=64 · EX_USAGE）原样转出。
	inv2 := &invocation{path: []string{"cocoon", "pack"}, orig: []string{"cocoon", "pack"}}
	var o2, e2 strings.Builder
	if rc := cmdCocoonEgg(inv2, &o2, &e2); rc != zegExitToolUsage {
		t.Fatalf("cocoon pack 缺参 rc=%d 期望 %d（真搬体 EX_USAGE · 原样转发）", rc, zegExitToolUsage)
	}
	if !strings.Contains(e2.String(), "用法：pack") {
		t.Fatalf("缺参 stderr 不含真搬体用法：%q", e2.String())
	}
	// 子命令名逐字取自 `inv.path[1]`（info 面）：读不到件 ⇒ 退 1（exitFail）。
	inv3 := &invocation{path: []string{"cocoon", "info"}, orig: []string{"cocoon", "info"}}
	var o3, e3 strings.Builder
	if rc := cmdCocoonEgg(inv3, &o3, &e3); rc != exitFail {
		t.Fatalf("cocoon info 无参 rc=%d 期望 %d（读不到件 ⇒ 退 1）", rc, exitFail)
	}
}

// ③ 换件三 action 补 D3 danger（opened 只能为真 —— 见 cli_danger_opened_test.go 的闭集判据）。
func TestSwapActionsCarryD3Danger(t *testing.T) {
	want := map[string]bool{"swap core": false, "swap agent": false, "swap ui": false}
	for _, c := range catalog() {
		j := strings.Join(c.path, " ")
		if _, ok := want[j]; !ok {
			continue
		}
		want[j] = true
		if c.danger == nil {
			t.Fatalf("%s 未挂 dangerSpec（A2/A3 回执 §四「写面 · 待 D3」）", j)
		}
		if c.danger.Level != dangerD3 {
			t.Errorf("%s 档位 = %s 期望 %s", j, c.danger.Level, dangerD3)
		}
		if !c.opened {
			t.Errorf("%s 未标 opened: true（runner 是真 exec 壳、不在自拒闭集 ⇒ 必须 opened:true）", j)
		}
	}
	for j, seen := range want {
		if !seen {
			t.Fatalf("%s 不在命令树里", j)
		}
	}
	// 负控：补 D3 后，「`opened` ⟷ 实现」对账闸仍须空（换件 runner 不在 refusingRunners ⇒ 只能 opened:true）。
	if probs := dangerOpenedProblems(catalog()); len(probs) > 0 {
		t.Errorf("dangerOpenedProblems 非空 ⇒ 换件 D3 补法与 opened 闸打架：%v", probs)
	}
}
