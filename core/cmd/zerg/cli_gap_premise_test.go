// cli_gap_premise_test.go —— `gap set-state` 的「**前提不成立**」销案**可现读锚**闸的成对机检。
//
// 病（本单靶心 · 上一单负控实测）：`--state 已解 --solved-kind 前提不成立` 是**不受「未落提交面禁销」
// 硬线约束的例外通道**，而此前**谁都能填** —— 存量账零门槛（无两态闸）、新账也只机械卡「两态读数」
// ⇒ 一条**本该真做**的账条，只要填个 `前提不成立` 就能销掉（`--dry-run` 都放行）⇒ 硬线被这条旁路架空。
//
// 治：填 `前提不成立` 时，证据必须点名一个**可现读的物**（两态读数段 / `件:行`（件现读可得）/
// 命令树里解析得到的 `zerg …`）—— ✗ 不收纯自然语言。本件钉住下列格（缺一即失败）：
//
//	① 负控（病）：纯自然语言 ⇒ `--dry-run` **rc=2**、stdout 0 字节、真源 sha 不变、审计不增
//	② 负控（真写）：同上带 `--yes` ⇒ rc=2 且**真源一个字节不改**（闸在「审计先落盘」之前）
//	③ 负控（存在性真判）：`件:行` 指向**不存在的件** ⇒ rc=2
//	④ 正控甲（两态读数段 `cmd=`）⇒ rc=0
//	⑤ 正控乙（`件:行` 且件现读可得）⇒ rc=0
//	⑥ 不受影响：`--solved-kind 销案` + 纯散文 ⇒ 仍 rc=0（闸**只**对 `前提不成立`）
//	⑦ 不受影响：`--solved-kind 实现` + 两态齐 + `--yes` ⇒ rc=0 且真写落位
//
// 落点与同族一致：`package main_test` + `zerg.RunForTest`（跑**当前源码**行为）· 全部合成盘面
// （`gapEnv` / `gapRun` / `gapAddArgv` / `gapIDOf` / `gapRecOf` / `gapSHAOf` 复用同族尺）⇒ 不碰真账。
package main_test

import (
	"strings"
	"testing"
)

func TestGapPremiseInvalidAnchorGate(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 靶子：走同族写面 `add` 造一条（真源只由命令写）—— id 号内日期 = 今天 ⇒ **新账**。
	rc, out, errb := gapRun(t, append(gapAddArgv("手搓：前提不成立销案没有防护", "误销靶子"), "--yes")...)
	if rc != 0 {
		t.Fatalf("造靶子失败：rc=%d · stdout=%q stderr=%q", rc, out, errb)
	}
	id := gapIDOf(t, out)

	sha0 := gapSHAOf(t, ledger)
	na0 := len(gapAuditLines(t, audit))

	// ① 负控（病 · 存量豁免版）：纯自然语言 ⇒ **dry-run 也拒**（rc=2）。
	rc, out, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "这条该真做，但填个前提不成立就销了", "--dry-run")
	if rc != 2 {
		t.Errorf("① 病条 dry-run 要 2（拒收），得到 %d · stderr=%q", rc, errb)
	}
	if out != "" {
		t.Errorf("① 拒收那一态 stdout 要 0 字节，得到 %q", out)
	}
	if !strings.Contains(errb, "可现读的物") {
		t.Errorf("① 拒收判词要逐字点名「可现读的物」，得到 %q", errb)
	}

	// ② 负控（真写）：带 `--yes` 也拒，且真源一个字节不改。
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "纯自然语言", "--yes")
	if rc != 2 {
		t.Errorf("② 病条真写要 2，得到 %d · stderr=%q", rc, errb)
	}

	// ③ 负控（存在性真判）：`件:行` 指向不存在的件 ⇒ 拒。
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立：见 no/such/file-zzz.go:42", "--dry-run")
	if rc != 2 {
		t.Errorf("③ 指向不存在的件:行 要 2（存在性真判），得到 %d · stderr=%q", rc, errb)
	}

	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("①②③ 拒收动了真源或写了审计（sha 或审计行数变了）")
	}

	// ④ 正控甲：两态读数段（`cmd=`）⇒ rc=0（复落实账 GAP-20260927-97 形的合法销案）。
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立（父代理裁定）",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解", "--dry-run")
	if rc != 0 {
		t.Fatalf("④ 正控甲（两态读数段）要 0，得到 %d · stderr=%q", rc, errb)
	}

	// ⑤ 正控乙：`件:行` 且件现读可得（仓根相对路径）⇒ rc=0。
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立：见 core/cmd/zerg/family_code.go:42", "--dry-run")
	if rc != 0 {
		t.Fatalf("⑤ 正控乙（件:行 现读可得）要 0，得到 %d · stderr=%q", rc, errb)
	}

	// ⑥ 不受影响：`销案` + 纯散文 ⇒ 仍 0（闸**只**对 `前提不成立`）。
	rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "销案",
		"--evidence", "纯自然语言销案（不受本闸约束）", "--dry-run")
	if rc != 0 {
		t.Fatalf("⑥ `销案`+散文 要 0（闸只对前提不成立），得到 %d · stderr=%q", rc, errb)
	}

	// ⑦ 不受影响：`实现` + 两态齐 + `--yes` ⇒ 真写 rc=0 且落位。
	if rc, _, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "实现",
		"--evidence", "真修已落 待提交面",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解", "--yes"); rc != 0 {
		t.Fatalf("⑦ `实现`+两态+--yes 要 0，得到 %d · stderr=%q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id); r["state"] != "已解" || r["solved_kind"] != "实现" {
		t.Errorf("⑦ 真写落位不对：state=%v solved_kind=%v", r["state"], r["solved_kind"])
	}
}
