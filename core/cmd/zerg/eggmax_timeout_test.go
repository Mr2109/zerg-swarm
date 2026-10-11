// eggmax_timeout_test.go —— `GAP-20260926-201`（全通路 · 单卵时限）**CLI 侧**自检。
//
// 判据（本单口径 ④）：
//
//	正控 —— 给了 `--timeout` ⇒ 下发体里**带** `egg_max_s`（秒 · 与子端 env `ZERG_EGG_MAX_S` 同口径）；
//	负控 —— 给了但读不出 ⇒ **明报拒收**（退 2 · ✗ **不静默回落**默认）—— 这一条要能**真红**。
//
// 白盒 · `package main`（判据要直接驱动那一格，不是转引别的层）。零网络：dry-run 只出计划件。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestEggMaxSecondsFlag —— `eggMaxSeconds` 逐格：给了转秒 · 不给空串 · 非法明报拒收。
func TestEggMaxSecondsFlag(t *testing.T) {
	ok := []struct {
		val  string
		want string
	}{
		{"5m", "300"},
		{"90", "90"}, // 裸数字按秒
		{"1.5s", "1.5"},
	}
	for _, c := range ok {
		inv := &invocation{kv: map[string][]string{wallclockFlag: {c.val}}}
		var sb strings.Builder
		got, rc := eggMaxSeconds(inv, &sb)
		if rc != exitOK || got != c.want {
			t.Fatalf("eggMaxSeconds(%q) = %q rc=%d，要 %q rc=0（stderr=%q）", c.val, got, rc, c.want, sb.String())
		}
	}
	// 不给 ⇒ 空串（**不下发**该键 ⇒ 子端回落 env）
	invNone := &invocation{kv: map[string][]string{}}
	var sbN strings.Builder
	if got, rc := eggMaxSeconds(invNone, &sbN); rc != exitOK || got != "" {
		t.Fatalf("eggMaxSeconds(不给) = %q rc=%d，要 \"\" rc=0", got, rc)
	}
	// 负控：非法 ⇒ 明报拒收（退 2 且点名旗标）
	invBad := &invocation{kv: map[string][]string{wallclockFlag: {"5x"}}}
	var sbB strings.Builder
	got, rc := eggMaxSeconds(invBad, &sbB)
	if rc == exitOK || got != "" {
		t.Fatalf("eggMaxSeconds(5x) 该**拒收**（拿到 %q rc=%d）—— 不许静默回落默认", got, rc)
	}
	if !strings.Contains(sbB.String(), wallclockFlag) {
		t.Fatalf("负控要**明报**拒收并点名旗标，实得 stderr=%q", sbB.String())
	}
}

// TestTaskSubmitBodyCarriesEggMax —— 下发体逐格：带它 / 不带它 / 非法值报错。
func TestTaskSubmitBodyCarriesEggMax(t *testing.T) {
	// 正控：`--timeout 5s` ⇒ 体里 `"egg_max_s":"5"`
	with := &invocation{desc: "x", kv: map[string][]string{wallclockFlag: {"5s"}}}
	body, err := taskSubmitBody(with)
	if err != nil {
		t.Fatalf("taskSubmitBody(带 --timeout) 不该报错：%v", err)
	}
	if !strings.Contains(body, `"egg_max_s":"5"`) {
		t.Fatalf("下发体没带 egg_max_s：%s", body)
	}
	// 不给 ⇒ 体里**没有**该键（回落 env 兜底）
	without := &invocation{desc: "x", kv: map[string][]string{}}
	body2, err := taskSubmitBody(without)
	if err != nil {
		t.Fatalf("taskSubmitBody(不给) 不该报错：%v", err)
	}
	if strings.Contains(body2, "egg_max_s") {
		t.Fatalf("没给 --timeout 不该出现 egg_max_s：%s", body2)
	}
	// 负控：非法 ⇒ 构造体即**报错**（退 2 的来处 · ✗ 不静默）
	bad := &invocation{desc: "x", kv: map[string][]string{wallclockFlag: {"oops"}}}
	if _, err := taskSubmitBody(bad); err == nil {
		t.Fatalf("非法 --timeout 该报错（✗ 不许静默回落默认）")
	}
}

// TestTaskSubmitDryRunEndToEnd —— `cmdTaskSubmit --dry-run` 端到端（零网络）：正控体带值 · 负控退 2。
// TestTaskSubmitDeadTimeoutWarnsDeadField —— 缺口 `GAP-20261010-09`：`--timeout` 在**任务面**的死字段
// （`Task.EggMaxS` 无处消费）必须**明说**（响亮警告 · rc 不变 · ✗ 不静默空转）。
//
//	正控 —— 给了 `--timeout` ⇒ stderr 响亮点名「不生效」且点名那枚旗标；
//	负控 —— **不给** ⇒ 一字不加（默认态既有输出逐字不动 ⇒ 这条要能真红）。
func TestTaskSubmitDeadTimeoutWarnsDeadField(t *testing.T) {
	with := &invocation{dryRun: true, path: []string{"task", "submit"}, desc: "x",
		kv: map[string][]string{wallclockFlag: {"5s"}}}
	var out, errb bytes.Buffer
	if rc := cmdTaskSubmit(with, &out, &errb); rc != exitOK {
		t.Fatalf("dry-run 该退 0，得 %d：%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "不生效") || !strings.Contains(errb.String(), wallclockFlag) {
		t.Fatalf("`--timeout` 在任务面不生效必须**明说**（点名旗标 + 「不生效」），实得 stderr=%q", errb.String())
	}
	without := &invocation{dryRun: true, path: []string{"task", "submit"}, desc: "x",
		kv: map[string][]string{}}
	var out2, errb2 bytes.Buffer
	if rc := cmdTaskSubmit(without, &out2, &errb2); rc != exitOK {
		t.Fatalf("不给 --timeout 的 dry-run 该退 0，得 %d：%s", rc, errb2.String())
	}
	if strings.Contains(errb2.String(), "不生效") {
		t.Fatalf("没给 --timeout 不该出警告（默认态一字不加），实得 stderr=%q", errb2.String())
	}
}

func TestTaskSubmitDryRunEndToEnd(t *testing.T) {
	// 正控
	inv := &invocation{dryRun: true, path: []string{"task", "submit"}, desc: "x",
		kv: map[string][]string{wallclockFlag: {"5s"}}}
	var out, errb bytes.Buffer
	if rc := cmdTaskSubmit(inv, &out, &errb); rc != exitOK {
		t.Fatalf("dry-run 该退 0，得 %d：%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), `"egg_max_s":"5"`) {
		t.Fatalf("计划件的请求体没带 egg_max_s：\n%s", out.String())
	}
	// 负控：非法值 ⇒ 退 2（真红）
	inv2 := &invocation{dryRun: true, path: []string{"task", "submit"}, desc: "x",
		kv: map[string][]string{wallclockFlag: {"oops"}}}
	var out2, errb2 bytes.Buffer
	if rc := cmdTaskSubmit(inv2, &out2, &errb2); rc != exitUsage {
		t.Fatalf("非法 --timeout 该退 %d，得 %d：%s", exitUsage, rc, errb2.String())
	}
}
