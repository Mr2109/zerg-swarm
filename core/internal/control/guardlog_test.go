// guardlog_test.go — 守卫拦截事件落点（缺口 `GAP-20260928-174` · 落 control 层）的回归牙。
//
// 钉住三格：
//
//	① **只有拦截才落**（`block` / `require_approval` 落 · `allow` 不落）· 事件逐格对 + `fp` 取法对
//	   （同 `gap idea add`：`sha256("guard\x00raw")[:12]`）；
//	② **合成 YAML 造的门不武装**（夹具/单测不许往真状态目录写事件）—— 这条是「测试安全」的机检；
//	③ 事件件**不在盘** ⇒ 空 + 无错（回收面据此判「零命中」），不是错误。
package control

import (
	"os"
	"path/filepath"
	"testing"
)

const guardLogTestRules = `version: 1
rules:
  default: block
  tools:
    - name: "read"
      action: allow
    - name: "edit_rules_yaml"
      action: block
    - name: "terminal"
      action: require_approval
`

// TestGuardLog_RecordsOnlyInterceptions — ①+③：拦截落、放行不落；读回逐格对；件不在 ⇒ 空且无错。
func TestGuardLog_RecordsOnlyInterceptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, GuardEventFile)

	// ③ 先证明「件不在 ⇒ 空 + 无错」。
	evs, err := ReadGuardEvents(path)
	if err != nil {
		t.Fatalf("件不在应无错：%v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("件不在应得 0 条，实得 %d：%+v", len(evs), evs)
	}

	g, err := NewGateFromYAML([]byte(guardLogTestRules))
	if err != nil {
		t.Fatal(err)
	}
	g.EnableGuardIntake(path)

	g.Check("read", "", "zerg")            // allow ⇒ **不落**
	g.Check("edit_rules_yaml", "", "zerg") // block ⇒ 落
	g.Check("terminal", "", "zerg")        // require_approval ⇒ 落
	g.Check("read", "", "zerg")            // 再放行一次 ⇒ 仍不落

	evs, err = ReadGuardEvents(path)
	if err != nil {
		t.Fatalf("读事件件：%v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("事件条数：期望 2（只有两条拦截），实得 %d：%+v", len(evs), evs)
	}
	// block 那条逐格对。
	wantFP := GuardFP(GuardSymptom("edit_rules_yaml", string(ActionBlock), "edit_rules_yaml"))
	if evs[0].Tool != "edit_rules_yaml" || evs[0].Action != string(ActionBlock) ||
		evs[0].Rule != "edit_rules_yaml" || evs[0].FP != wantFP {
		t.Fatalf("block 事件逐格不符（fp 期望 %s）：%+v", wantFP, evs[0])
	}
	// require_approval 那条：命中规则 `terminal`。
	if evs[1].Tool != "terminal" || evs[1].Action != string(ActionRequireApproval) || evs[1].Rule != "terminal" {
		t.Fatalf("require_approval 事件逐格不符：%+v", evs[1])
	}
	// 症状文本**不含时刻** ⇒ 同一条拦截重放必得同 fp（回收面幂等的前提）。
	if GuardSymptom("terminal", string(ActionRequireApproval), "terminal") != GuardSymptom("terminal", string(ActionRequireApproval), "terminal") {
		t.Fatal("症状文本不稳定 —— fp 去重会失效")
	}
}

// TestGuardLog_SyntheticGateIsNotArmed — ②`NewGateFromYAML` 造的门**不武装**
// （`NewGateFromFile` 才武装 · 见 `gate.go`）⇒ 合成夹具不会往真状态目录写事件。
func TestGuardLog_SyntheticGateIsNotArmed(t *testing.T) {
	g, err := NewGateFromYAML([]byte(guardLogTestRules))
	if err != nil {
		t.Fatal(err)
	}
	if g.guardSink != "" {
		t.Fatalf("合成 YAML 造的门不许武装守卫入池，实得 sink=%q", g.guardSink)
	}
	// 成对：不武装时问门**仍然该拦**（守卫判定不受记录面影响）· 且一个字节都不落。
	dir := t.TempDir()
	if d := g.Check("edit_rules_yaml", "", "zerg"); d.Action != ActionBlock {
		t.Fatalf("不武装时判定不该变：得到 %q，要 %q", d.Action, ActionBlock)
	}
	if _, err := os.Stat(filepath.Join(dir, GuardEventFile)); !os.IsNotExist(err) {
		t.Fatal("不武装却落了事件件")
	}
}

// TestGuardLog_GuardIntakeDoesNotChangeDecision — 记录面是 best-effort：武装前后**判定逐字同**
// （守卫的 semantics 不受入池影响）。
func TestGuardLog_GuardIntakeDoesNotChangeDecision(t *testing.T) {
	mk := func(sink string) *Gate {
		g, err := NewGateFromYAML([]byte(guardLogTestRules))
		if err != nil {
			t.Fatal(err)
		}
		if sink != "" {
			g.EnableGuardIntake(sink)
		}
		return g
	}
	bare := mk("")
	armedPath := filepath.Join(t.TempDir(), GuardEventFile)
	armed := mk(armedPath)
	for _, tool := range []string{"read", "edit_rules_yaml", "terminal", "never_listed"} {
		a := bare.Check(tool, "", "zerg")
		b := armed.Check(tool, "", "zerg")
		if a != b {
			t.Fatalf("武装改变了判定（tool=%s）：裸 %+v ⟷ 武装 %+v", tool, a, b)
		}
	}
	// 武装那一侧确实落了 3 条（read 放行不落；其余三条：block×1 + require_approval×1 + 默认档 block×1）。
	if evs, err := ReadGuardEvents(armedPath); err != nil || len(evs) != 3 {
		t.Fatalf("武装侧事件条数：期望 3，实得 %d（err=%v）", len(evs), err)
	}
}
