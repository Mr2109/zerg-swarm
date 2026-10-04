// family_sched.go —— 新族 `sched` 三格**写面**的唯一执行门（缺口 `GAP-20261005-10` · P1）。
//
// 病（缺口原文 · 前一票真跑坐实）：`sched pause|resume|set` 是改**在跑调度器派活态**的写面，
// 而 `main.go` 三格原无 `dangerSpec` ⇒ 既不进 `zerg help dangerous` 危险表（现读 61 条里一条
// sched 都没有），用法串也不收 `--dry-run`/`--confirm`/`--yes`；实测 `zerg sched pause`
// （**无任何旗标**）即把调度器 `running→paused` 并退 0 —— 一次真实控制面变更零授权、零登记、
// 零审计。对照同语义写面（`task cancel` 要 `--confirm`+`--yes` · `gateway breakers --reset` /
// `resource pin` 要 `--yes`）⇒ 同族口径不齐。
//
// 治法（与同语义写面口径**逐字一致** · 参照 `task cancel`〔D2 + --confirm + --yes〕与外族
// `guard.go` 的三态纪律 · ✗ 不自创第三档 · §九 M3 C1/C2/C4）：
//
//	· D2 档（有影响但可逆：pause↔resume / set 改回旧 N 即原状）；
//	· `--confirm=<本机名>` —— 值须与**本机名**逐字相同（判等走同族 `devEditHostAccept`，
//	  源头仍是 `planHost()`，✗ 不新开取源）；
//	· `--yes` —— fail-closed（缺就拒，无 TTY 也从不提问）；
//	· `--dry-run` —— 只出**计划件**、零副作用、退 0（档序：干跑先于确认档）；
//	· `--dry-run` 与 `--yes` 同给 ⇒ `dryRunYesConflict` 判用法错 2。
//
// 三格共用本门；确认档齐后**仍转原实现真写**（`cmdSchedPause`/`cmdSchedResume`/`cmdSchedSet`
// 落在 `family_task.go` —— 本票**不碰**任何 `family_*.go`，故这道门落本件、原实现一件不动）。
package main

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

// schedPlanHeader —— 计划件首行（与 `cmdHazardWrite` / `task cancel` 同一句读法）。
const schedPlanHeader = "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）"

// schedConfirmGate —— sched 写面的确认档判定（D2 · `--confirm=<本机名>` + `--yes` · fail-closed）。
//
// 返回 `exitOK` ⇒ 确认档已齐（调用方转原实现真写）；否则返回的就是本趟退码（用法错 2）。
// 口径逐字照同族写面（`task cancel` 的 `--confirm` 落点 + `guard.go` 的 `cmdGuarded` D2 支）：
// 缺 / 值不匹配 ⇒ fail-closed，从不静默当「没给」。
func schedConfirmGate(inv *invocation, stderr io.Writer, action string) int {
	if !devEditHostAccept(inv.confirm) {
		msg := fmt.Sprintf("`%s` 是 D2 档 —— **缺 --confirm=<本机名> ⇒ 不执行**", action)
		if inv.confirmGiven {
			msg = fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机名是 %q）⇒ 不执行", inv.confirm, planHost())
		}
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与本机名**逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `%s` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName, action)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, action)
		return exitUsage
	}
	return exitOK
}

// emitSchedPlan —— 计划件（人面 · stderr 一行收口）；三格共用（零副作用）。
func emitSchedPlan(stdout, stderr io.Writer, action, endpoint, effect string) {
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s %s\n", progName, action)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 · 要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, endpoint)
	fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
	fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	fmt.Fprintf(stdout, "  来源     : 缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdSchedPauseGuarded —— `zerg sched pause` 的唯一执行门（确认档齐 ⇒ 转 `cmdSchedPause` 真写）。
func cmdSchedPauseGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		emitSchedPlan(stdout, stderr, "sched pause", "/api/sched/pause",
			"暂停派活（不再起新的任务；**在跑的不动** —— 要停在跑的走 `task terminate`）—— 可逆：`sched resume` 即回原态")
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "sched pause"); rc != exitOK {
		return rc
	}
	return cmdSchedPause(inv, stdout, stderr)
}

// cmdSchedResumeGuarded —— `zerg sched resume` 的唯一执行门。
func cmdSchedResumeGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		emitSchedPlan(stdout, stderr, "sched resume", "/api/sched/resume",
			"恢复派活（回到设定并发并立即派发积压）—— 可逆：`sched pause` 即回原态")
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "sched resume"); rc != exitOK {
		return rc
	}
	return cmdSchedResume(inv, stdout, stderr)
}

// cmdSchedSetGuarded —— `zerg sched set <N>` 的唯一执行门（N 校验先于干跑/确认档，照 `task cancel` 档序）。
func cmdSchedSetGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// 目标：位置参数一个非负整数 N（与 `cmdSchedSet` 同一闭集口径 · 缺/非整数 ⇒ 用法错 2）。
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺并发上限 N")
		fmt.Fprintf(stderr, "%s: `sched set` 要一个非负整数 N（如 `%s sched set 2`）\n", progName, progName)
		return exitUsage
	}
	n := strings.TrimSpace(inv.args[0])
	if _, err := strconv.Atoi(n); err != nil {
		inv.setErr("usage", "bad_flag_value", "并发上限要非负整数")
		fmt.Fprintf(stderr, "%s: `sched set` 的 N 要非负整数（给的是 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	if inv.dryRun {
		emitSchedPlan(stdout, stderr, "sched set", "/api/sched/set?max="+url.QueryEscape(n),
			"把调度器并发上限设成 "+n+"（显式旋钮）—— 可逆：再 `sched set <旧 N>` 即回原态")
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "sched set"); rc != exitOK {
		return rc
	}
	return cmdSchedSet(inv, stdout, stderr)
}
