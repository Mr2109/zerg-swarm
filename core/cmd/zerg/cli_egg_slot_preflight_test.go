// cli_egg_slot_preflight_test.go —— 干跑「槽位预言」的契约测试（缺口 `GAP-20260926-09`）。
//
// 症状（账上原文）：`zerg egg run <卵> --machine <机> --dry-run` 只印一句泛泛的
// 「占内存/槽位 · 单槽机是串行的」，**不点名这会挤掉那台机上正在跑的那台引擎** ⇒
// 人的下一步只能靠事后 `ps` 看。
//
// 三条判据（每条都成对给出「该绿」与「该红」两态，判据自己不许恒绿）：
//
//	① 账本上那台机**驻留着别的引擎** ⇒ 计划件必须**点名**它（逐字出现在
//	   `本次装载会挤掉` 那一行里）；
//	② 账本上那台机**没有别的引擎驻留** ⇒ 必须明写 `无（…）`（空也要说清，
//	   ✗ 不许留一句歧义 —— 这正是本账要求的口径）；
//	③ 那台机驻留的**就是本次要装的同一枚** ⇒ 必须写明「复用，不挤掉别人」，
//	   ✗ 不许把它自己当被挤者点名。
//
// 数据源只认 `/api/resources/ledger`（与 `zerg resource ledger` 同一份口径）：
// 本文件的三态全部由合成主控给出，**不连真网**、不读真账本。
package main_test

import (
	"strings"
	"testing"
)

// eggSlotFleetJSON —— 合成名册：`Slot-C` 只在 `Mr2109` 上（干跑目标），`Slot-A` 同样在册
// （它只是「本机驻留」那一格的候选名字，不需要在名册里出现也能进预言 —— 这里给全是为了
// 让夹具更接近现场）。
const eggSlotFleetJSON = `{"count":2,"models":[
 {"id":"Slot-C","host":"Mr2109","backend":"llama-server","mem_gb":11},
 {"id":"Slot-A","host":"Mr2109","backend":"llama-server","mem_gb":22}]}`

// eggSlotLedger —— 合成账本：`Mr2109` 上驻留着 `Slot-A`（托管 · pinned · 有剩余 TTL）。
const eggSlotLedger = `{"count":1,"machines":[
 {"machine":"Mr2109","mem_known":true,"mem_available_gb":20,"backend_state":"idle",
  "resident":[{"alias":"Slot-A","state":"idle_armed","managed":true,"pinned":true,
               "pin_ttl_s":1309.9,"mem_gb":22}]}]}`

// eggSlotLedgerEmpty —— 同一台机、**账本上有这台机但没有驻留明细**（空也要说清的那一态）。
const eggSlotLedgerEmpty = `{"count":1,"machines":[
 {"machine":"Mr2109","mem_known":true,"mem_available_gb":42,"backend_state":"idle"}]}`

// eggSlotLedgerSelf —— 驻留的正是本次要装的那一枚（复用档）。
const eggSlotLedgerSelf = `{"count":1,"machines":[
 {"machine":"Mr2109","mem_known":true,"mem_available_gb":20,
  "resident":[{"alias":"Slot-C","state":"idle_armed","managed":true}]}]}`

// eggSlotRun —— 跑一发干跑，返回（rc · 人面合流）。
func eggSlotRun(t *testing.T, ledger string) (int, string) {
	t.Helper()
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models":     eggSlotFleetJSON,
		"/api/resources/ledger": ledger,
	})
	defer srv.Close()
	rc, stdout, stderr := runEggCmd(t, "egg", "run", "Slot-C", "--machine", "Mr2109", "--dry-run")
	return rc, stdout + stderr
}

// 判据①：驻留着别的引擎 ⇒ 计划件**点名**它（正例）。
func TestEggRunDryRunNamesEvicted(t *testing.T) {
	rc, out := eggSlotRun(t, eggSlotLedger)
	if rc != 0 {
		t.Fatalf("干跑退码 = %d（要 0）· 输出=%s", rc, out)
	}
	if !strings.Contains(out, "本次装载会挤掉 : Slot-A") {
		t.Fatalf("计划件没点名会被挤掉的 `Slot-A`（这是本缺口的核心症状）· 输出=%s", out)
	}
	if !strings.Contains(out, "本机驻留 : Mr2109 ⇒ Slot-A") {
		t.Fatalf("计划件没印出该机当前驻留 · 输出=%s", out)
	}
	// pinned 档要把「谁钉着」也带出来（人要知道顶掉它有没有代价）。
	if !strings.Contains(out, "pinned") {
		t.Fatalf("被挤者 pinned 的读数没带出来 · 输出=%s", out)
	}
}

// 判据②：账本上那台机空着 ⇒ 明写「无」（负控：这一态**不许**点名任何模型）。
func TestEggRunDryRunSaysNoEviction(t *testing.T) {
	rc, out := eggSlotRun(t, eggSlotLedgerEmpty)
	if rc != 0 {
		t.Fatalf("干跑退码 = %d（要 0）· 输出=%s", rc, out)
	}
	if !strings.Contains(out, "本次装载会挤掉 : 无（") {
		t.Fatalf("驻留为空时必须明写「无（…）」· 输出=%s", out)
	}
	if strings.Contains(out, "本次装载会挤掉 : Slot-") {
		t.Fatalf("空账本却点名了模型（假点）· 输出=%s", out)
	}
	if !strings.Contains(out, "⇒ 无（") {
		t.Fatalf("「本机驻留」那一格也要明写「无」· 输出=%s", out)
	}
}

// 判据③：驻留的就是本次这一枚 ⇒ 「复用，不挤掉别人」（✗ 不许把自己点名成被挤者）。
func TestEggRunDryRunSelfResidentIsReuse(t *testing.T) {
	rc, out := eggSlotRun(t, eggSlotLedgerSelf)
	if rc != 0 {
		t.Fatalf("干跑退码 = %d（要 0）· 输出=%s", rc, out)
	}
	if !strings.Contains(out, "本次装载会挤掉 : 无（该模型已驻留在该机") {
		t.Fatalf("自驻留要报「复用」，输出=%s", out)
	}
	if strings.Contains(out, "本次装载会挤掉 : Slot-C") {
		t.Fatalf("把本次目标自己点名成被挤者（自指）· 输出=%s", out)
	}
}

// 判据④（负控 · 读不到）：账本面 404 ⇒ 明写「不给结论」，✗ 不许降级成「谁也不会被挤掉」。
func TestEggRunDryRunLedgerUnreachableDoesNotClaimSafe(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/models": eggSlotFleetJSON})
	defer srv.Close()
	rc, stdout, stderr := runEggCmd(t, "egg", "run", "Slot-C", "--machine", "Mr2109", "--dry-run")
	out := stdout + stderr
	if rc != 0 {
		t.Fatalf("干跑退码 = %d（要 0 —— 读不到驻留面不该把干跑打死）· 输出=%s", rc, out)
	}
	if !strings.Contains(out, "本次装载会挤掉 : 不给结论") {
		t.Fatalf("驻留面读不到时要印「不给结论」· 输出=%s", out)
	}
	if strings.Contains(out, "无（该机账本上没有别的引擎驻留）") {
		t.Fatalf("读不到却报「不会挤掉」= 拿未知冒充安全 · 输出=%s", out)
	}
}
