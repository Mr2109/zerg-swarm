// wall_mutate.go —— `zerg wall mutate <bridge|macos>`（茧壁**变异**验证 · 危险档 D2/写仓 · 本版**已开放**）。
//
// 真身（唯一真源）= `scripts/wall/{wall-bridge-mutate,wall-macos-mutate}.py`：各自**真改仓内件**
// （改坏 `wall/src/**.rs` 与迁移桥两侧）⇒ 断言门必须红 ⇒ 还原 ⇒ 核 sha；还原走公共件
// `scripts/docs/inplace-window.py`（跑前 `arm` 落前像 · 落状态目录 · 不落仓内）。命令面**只做入口 ＋ 转发**，
// ✗ 不重写脚本任何一段 —— 与同族 `wall evidence`（`cmdWallEvidence` · shell 转账）逐字同形（脚本退码原样转出）。
//
// 依据：`待拍清单-AlltoGo-20261008.md` P5（两个变异器真改仓内件 ⇒ 入危险档 D2/写仓 ＋ 还原台账纪律）。
//
// 三态（与 `agent load` / `core reload` 同一条口径 · §4.1 K7 · §九 M3 C1/C2/C4 · §十二 P-014）：
//
//	· `--dry-run` ⇒ 只出**计划件**、零副作用（★ 不设任何前置 —— 裸 `zerg wall mutate --dry-run` 也退 0）；
//	· 缺 `--yes`（D2）⇒ fail-closed 退 2（从不提问 · 无 TTY 也一样）；
//	· 确认档齐 ⇒ 转发脚本真跑。
//
// ★ 本命令**非透传型**（`passthrough` 未置）⇒ 未知旗标 / 不认的契约主号一律在**执行前**由全局解析面退 2
// （不进脚本）—— 与 `wall evidence`（透传型）**刻意不同**：变异器真改仓内件 ⇒ 旗标面收窄到命令面能判的闭集。
package main

import (
	"fmt"
	"io"
	"strings"
)

// wallMutateRoster —— 对象名 → 变异脚本（唯一真源 = 脚本本体 · ✗ 不搬实现）。
var wallMutateRoster = []struct{ Name, File string }{
	{"bridge", "scripts/wall/wall-bridge-mutate.py"},
	{"macos", "scripts/wall/wall-macos-mutate.py"},
}

// wallMutateScript —— 按**逐字名**取变异脚本（模糊匹配等于没匹配 ⇒ 不猜）。
func wallMutateScript(name string) (string, bool) {
	for _, p := range wallMutateRoster {
		if p.Name == name {
			return p.File, true
		}
	}
	return "", false
}

// wallMutateNames —— 闭集里可用的子动作名（逐字给出 · 报错时用）。
func wallMutateNames() []string {
	names := make([]string, 0, len(wallMutateRoster))
	for _, p := range wallMutateRoster {
		names = append(names, p.Name)
	}
	return names
}

// cmdWallMutate —— `zerg wall mutate <bridge|macos>` 的唯一执行门（D2 · 确认档齐 ⇒ 转发变异脚本真跑）。
func cmdWallMutate(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	target := ""
	if len(inv.args) > 0 {
		target = strings.TrimSpace(inv.args[0])
	}
	// ① 干跑：计划件（零副作用 · 一个脚本都不起）—— ★ 不设任何前置（裸 `--dry-run` 也退 0）。
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未起任何脚本、未改任何仓内件）")
		fmt.Fprintf(stdout, "  动作     : %s wall mutate\n", progName)
		fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 脚本内跑前落备份、跑后逐件核 sha 还原；要 --yes）\n", dangerD2)
		fmt.Fprintf(stdout, "  目标     : %s\n", orDash(target))
		fmt.Fprintf(stdout, "  它会动   : 真改仓内件做变异验证（改坏 wall/src/**.rs 与迁移桥两侧 ⇒ 门必须红 ⇒ 还原）；还原台账纪律在脚本内落地（`scripts/docs/inplace-window.py` arm）\n")
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n", dangerD2)
		fmt.Fprintf(stdout, "  底层     : scripts/wall/<bridge|macos>-mutate.py（**唯一真源** · 命令面只转发、不翻译）\n")
		fmt.Fprintf(stdout, "  来源     : §九 M10 · 待拍清单 P5（茧壁变异器入危险档 D2/写仓）· 同族先例 `wall evidence`（壳转账）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// ② 确认档（fail-closed · 与 `cmdGuarded` ③ 段逐字同一口径）：D2 缺 `--yes` ⇒ 拒执（零副作用 · 零脚本）。
	if rc := guardedControlConfirmGate(inv, stderr, target); rc != exitOK {
		return rc
	}
	// ③ 子动作闭集（真跑档才要求 · 逐字名）：不认的名 ⇒ 用法错 2、逐字给出可用名（同 `wall evidence`）。
	rel, ok := wallMutateScript(target)
	if !ok {
		inv.setErr("usage", "wall_mutate_unknown", "茧壁变异名不在册")
		fmt.Fprintf(stderr, "%s: `wall mutate` 认的子动作名：%s（给的是 %q）⇒ 不给结论（退码 2）\n",
			progName, strings.Join(wallMutateNames(), " | "), target)
		return exitUsage
	}
	// ④ 真跑：转发脚本（**唯一真源** · 只转发、不翻译 · 脚本退码原样转出）—— 与 `cmdWallEvidence` 同形。
	return wallRunScript(inv, stdout, stderr, "python3", rel, nil)
}
