// cli_gate_find_regstate_test.go —— `zerg gate find` 的**登记态两格**（`state` / `not_collected`）
// 的判据（缺口 `GAP-20261015-08`）。
//
// 病（父代理 2026-10-19 现读 · 逐字）：`zerg gate find … --json state` ⇒「未知字段 \"state\"」退 2；
// 人面也只有 `wiring` 那一长串可自己推 ⇒ 「件在盘但没进步骤表」与「已进步骤表」在命令面上判不出来，
// 且「哪些没收集」一个字都没有。
//
// 本件钉**三态 + 负控 + 两面同源**（合成仓 · 进程内 `runZergRepo` 读**当前源码** —— `bin/zerg`
// 是按批重建制品，本条不依赖它）：
//
//	① **已登记**   ⇒ `state` = `已登记` · `not_collected` = **空**（步骤表里有它的声明行）；
//	② **未登记**   ⇒ `state` = `未登记` · `not_collected` = **点名**该件（件在盘、步骤表没点名）；
//	③ **不给结论** ⇒ 该步的登记字段**置空**（步名那格空）⇒ `state` 以 `不给结论` 起头且带理由，
//	              **✗ 不许静默读成「已登记」**（也不许把 `not_collected` 留空冒充「一件没漏」）；
//	④ 两面同源：人面（默认 TSV）表头与两只值都在；`--json state,not_collected` 点得出来（✗ 不再退 2）。
package main_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIGateFindRegStateThreeStates —— 三态各一格 + 负控（登记态置空 ⇒ 不给结论）。
func TestCLIGateFindRegStateThreeStates(t *testing.T) {
	root := showRepo(t) // 合成仓：两行 add_step（`gofmt -l core` / `合成步` —— 命令串含 `synthetic.py`）
	// ② 的靶子：件在盘、`add_step` 一个字都没点名它（真·未登记）。
	mustWrite(t, filepath.Join(root, "scripts", "gates", "uncollected-synth.sh"),
		"#!/usr/bin/env bash\nexit 0\n")
	// ③ 的靶子（**负控**）：一条 `add_step`，但**步名那一格置空** ⇒ 该步的登记字段缺失。
	//    这条声明仍进得了 `decls`（引号串允许空），但**不构成可引用的登记**（`gate show`/`gate explain`
	//    都拿步名当键）⇒ 判不了，必须显示「不给结论」。
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step gates \"\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/regstate-empty.py\"\n")

	// ① 已登记（既有「合成步」在步骤表里）—— `not_collected` 必须是**空串**。
	rc1, out1, err1 := runZergRepo(t, root, "gate", "find", "合成步", "--json", "name,state,not_collected")
	if rc1 != 0 {
		t.Fatalf("已登记态应 rc=0，实得 rc=%d（stderr=%s）", rc1, err1)
	}
	it1 := onlyItem(t, out1)
	if it1["state"] != "已登记" {
		t.Fatalf("已登记的步骤行 `state` 必须逐字「已登记」，实得 %q（items=%s）", it1["state"], out1)
	}
	if it1["not_collected"] != "" {
		t.Fatalf("已登记的行 `not_collected` 必须为空，实得 %q", it1["not_collected"])
	}

	// ② 未登记 —— `state` = 未登记，且 `not_collected` **点名**该件（基名）。
	rc2, out2, err2 := runZergRepo(t, root, "gate", "find", "uncollected-synth",
		"--json", "name,state,not_collected")
	if rc2 != 0 {
		t.Fatalf("未登记件应仍 rc=0（件在盘就是命中，不是错），实得 rc=%d（stderr=%s）", rc2, err2)
	}
	it2 := onlyItem(t, out2)
	if it2["state"] != "未登记" {
		t.Fatalf("步骤表未点名的门件行 `state` 必须逐字「未登记」，实得 %q（items=%s）", it2["state"], out2)
	}
	if !strings.Contains(it2["not_collected"], "uncollected-synth.sh") {
		t.Fatalf("未登记行的 `not_collected` 必须**点名**该件（uncollected-synth.sh），实得 %q", it2["not_collected"])
	}

	// ③ 负控：把该步的登记态**置空** ⇒ 必须显示「不给结论」（✗ 不许静默当已登记）。
	rc3, out3, err3 := runZergRepo(t, root, "gate", "find", "regstate-empty",
		"--json", "name,state,not_collected")
	if rc3 != 0 {
		t.Fatalf("负控夹具应仍 rc=0（命中该声明），实得 rc=%d（stderr=%s）", rc3, err3)
	}
	it3 := onlyItem(t, out3)
	if !strings.HasPrefix(it3["state"], "不给结论") {
		t.Fatalf("登记态置空时必须显示「不给结论」（✗ 不许静默当已登记），实得 state=%q（items=%s）",
			it3["state"], out3)
	}
	if strings.TrimSpace(it3["not_collected"]) == "" {
		t.Fatalf("「不给结论」那一行的 `not_collected` 不许留空（留空＝静默主张「一件都没漏」），实得空串")
	}

	// ④ 两面同源：人面表头必须带这两格（✗ 不许只在机器面有）。
	rc4, out4, err4 := runZergRepo(t, root, "gate", "find", "合成步")
	if rc4 != 0 {
		t.Fatalf("人面正控应 rc=0，实得 rc=%d（stderr=%s）", rc4, err4)
	}
	for _, col := range []string{"state", "not_collected"} {
		if !strings.Contains(out4, col) {
			t.Fatalf("人面（默认 TSV）表头必须带 %s 格（与机器面同源），实得 stdout=%q", col, out4)
		}
	}
	if !strings.Contains(out4, "已登记") {
		t.Fatalf("人面必须把「已登记」打出来（不是只给机器面），实得 stdout=%q", out4)
	}
}

// onlyItem —— `--json` 的 `items` 恰好一条时取回该行（多于一条 = 夹具跑偏，直接判红）。
//
// ★ 为什么钉条数：本件三态的靶子各自只该命中**一行**；多于一条会让「读数取自哪一行」变得不可知，
// 那时任何断言都可能是在别的行上碰巧成立（假绿）。
func onlyItem(t *testing.T, out string) map[string]string {
	t.Helper()
	items := envelopeItems(t, out)
	if len(items) != 1 {
		t.Fatalf("本夹具每态应恰好命中 1 行，实得 %d 行：%s", len(items), out)
	}
	return items[0]
}
