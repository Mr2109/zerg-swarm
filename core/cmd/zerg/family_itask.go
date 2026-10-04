// family_itask.go —— 内部任务引擎族 `itask`（§7.1 `P8` · §5.1 `/api/internal-tasks*` 9 条端点）。
//
// 九条端点**逐字**（现跑 200 的四条只读 + 五条写面；出处 `core/internal/api/internal_*.go`）：
//
//	只读四条：GET `/api/internal-tasks` · `/api/internal-tasks/state` ·
//	          `/api/internal-tasks/modes` · `/api/internal-tasks/intervals`
//	写面五条：POST `/api/internal-tasks/{id}/mode` · `/{id}/interval` · `/{id}/run` ·
//	          `/api/internal-tasks/start` · `/api/internal-tasks/stop`
//
// 归属定案（开工单 T-49 ③ · §7.1 `P8` 的推荐口径）：**独立成族 `itask`，不并进 `core`**。
// 理由（照 §5.1 的分域）：`core` 族管的是**主控进程自身**（起停/日志/换件），而内部任务引擎
// 是**主控里的一个子系统**（16 类任务 + 周期 + 自动运行开关）；两者的生命周期、危险档、审计面
// 都不一样 —— 并进去会让 `core` 族同时回答「主控在不在」与「进化任务跑没跑」两件事。
//
// 与常驻 `health-check` / `proc-cleanup`（6h）的接缝（`P-046` 的落点）：它们是 `itask ls` 里的
// **两条具体任务**（`cooldown` 字段就是周期），本件不另立命令名 —— 周期与模式一律走
// `itask interval` / `itask mode`（一处接口，16 类共用）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// cmdItaskLs —— `zerg itask ls`（只读）：内部任务清单（16 类 · 含最近执行与冷却）。
func cmdItaskLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/internal-tasks", &resp, stderr); rc != exitOK {
		return rc
	}
	fields := []string{"id", "description", "cooldown", "default_hours", "auto_run", "last_run", "state"}
	rows := []map[string]string{}
	for _, it := range asList(resp["items"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, fields))
		}
	}
	if len(rows) == 0 {
		rows = append(rows, map[string]string{"id": "（清单为空）",
			"description": "载荷里 items 是空的", "cooldown": "", "default_hours": "", "auto_run": "",
			"last_run": "", "state": ""})
	}
	return listCmd(inv, stdout, stderr, []string{"id", "description", "cooldown", "auto_run"}, rows)
}

// cmdItaskState —— `zerg itask state`（只读）：引擎状态（enabled/running/stopped/ticks）。
func cmdItaskState(inv *invocation, stdout, stderr io.Writer) int {
	// 载荷真形状（现读）：`{note, state{enabled,running,stopped,since,last_tick,next_tick,reason,updated_at}}`
	// ⇒ 顶层键就是 `note`/`state`（**不猜键名**；`state` 里那八格由 `--json state` 整块给出）。
	return itaskOne(inv, stdout, stderr, "/api/internal-tasks/state", []string{"note", "state"})
}

// cmdItaskMode —— `zerg itask mode`（只读面）：16 类的自动运行开关现值（写面见 main.go 的 D2 登记）。
func cmdItaskMode(inv *invocation, stdout, stderr io.Writer) int {
	return itaskOne(inv, stdout, stderr, "/api/internal-tasks/modes", []string{"modes", "note", "state"})
}

// cmdItaskInterval —— `zerg itask interval`（只读面）：16 类的周期现值（小时）。
func cmdItaskInterval(inv *invocation, stdout, stderr io.Writer) int {
	return itaskOne(inv, stdout, stderr, "/api/internal-tasks/intervals",
		[]string{"intervals", "note", "state"})
}

// itaskOne 取一条「单个对象」的只读端点并按载荷键投影（字段名逐字取载荷，不另造词汇）。
func itaskOne(inv *invocation, stdout, stderr io.Writer, path string, want []string) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, path, &resp, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{}
	keys := []string{}
	for _, k := range want {
		if _, ok := resp[k]; ok {
			keys = append(keys, k)
			row[k] = cell(resp[k])
		}
	}
	if len(keys) == 0 {
		inv.setErr("blocked", "unexpected_payload", "载荷里没有期望的键（不给结论）")
		fmt.Fprintf(stderr, "%s: %s 的载荷里没有期望的键（%v）⇒ 不给结论（退码 8）\n", progName, path, want)
		return exitBlocked
	}
	return listCmd(inv, stdout, stderr, keys, []map[string]string{row})
}

// ---- `itask stop` 写面（D2 · 写持久化用户意图 `stopped`）----
//
// 现读写路径（本票只开这一条）：`zerg itask stop` ⇒ POST `/api/internal-tasks/stop`
// （主控侧路由已存在 · `core/cmd/zerg-core/main.go` → `api.InternalTasksStopHandler`）
// ⇒ `SetEngineStopped(true, …)` ⇒ **持久化** `~/.zerg/state/internal_engine.json` 的 `stopped=true`
// （重启恢复用户意图）。核侧端点本票**一字未动** —— 只开放命令面的写入口。
//
// 口径逐字照同族写面先例（`sched pause`〔`family_sched.go` · 缺口 `GAP-20261005-10`〕＋
// `task cancel` 的三态纪律 · ✗ 不自创第三档 · §九 M3 C1/C2/C4）：
//
//	· D2 档（有影响但可逆：stopped=true ↔ 再起引擎即回 false）；
//	· `--dry-run` —— 只出**计划件**、零副作用、退 0（档序：干跑先于确认档）；
//	· `--confirm=<本机名>`（判等走同族 `devEditHostAccept` · 源头 `planHost()`）；`--yes` —— fail-closed；
//	· `--dry-run` 与 `--yes` 同给 ⇒ `dryRunYesConflict` 判用法错 2。
//
// ★ 本票**不开放** `itask start` / `itask run`（“启引擎/跑任务”与「暂停」无关，且风险面大 ⇒ 留待拍板）。

// cmdItaskStopGuarded —— `zerg itask stop` 的唯一执行门（D2 · 确认档齐 ⇒ 转真写）。
func cmdItaskStopGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		emitItaskStopPlan(stdout, stderr)
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "itask stop"); rc != exitOK {
		return rc
	}
	return cmdItaskStopWrite(inv, stdout, stderr)
}

// emitItaskStopPlan —— `itask stop --dry-run` 的计划件（人面 · stderr 一行收口 · 零副作用）。
func emitItaskStopPlan(stdout, stderr io.Writer) {
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s itask stop\n", progName)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再起引擎即回原态；要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  请求     : POST %s/api/internal-tasks/stop\n", newClient().base)
	fmt.Fprintf(stdout, "  它会动   : 把用户意图 `stopped` 置 true（**持久化**到状态件 `internal_engine.json`）—— 引擎按周期/空闲触发都**不再起新的**内部任务；**在跑的这一轮不动**\n")
	fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	fmt.Fprintf(stdout, "  来源     : §7.1 P8 · §5.1 POST /api/internal-tasks/stop · 开工单 T-49 · 同族写面先例 `sched pause`（D2 + --confirm + --yes）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdItaskStopWrite —— `itask stop` 的真写（确认档齐后）：POST `/api/internal-tasks/stop`，回带 state。
func cmdItaskStopWrite(inv *invocation, stdout, stderr io.Writer) int {
	return itaskPost(inv, stdout, stderr, "/api/internal-tasks/stop", "停止内部任务")
}

// itaskPost —— 打一发 itask 写面（POST）并把回带的 `state` 摊成人面/机器面（照 `schedPost` 同形；
// 失败面把 http 状态与主控原文一起带回来）。契约：变更即回状态（`core/internal/api/internal_engine.go`）。
func itaskPost(inv *invocation, stdout, stderr io.Writer, path, action string) int {
	status, body, err := postJSON(newClient(), path, "")
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		status, body, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次%s（http_%d）—— 看 `zerg help credentials`\n", progName, action, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(body)
		inv.setErr("usage", code, "主控拒绝"+action)
		fmt.Fprintf(stderr, "%s: 主控拒绝%s（http_%d）· code=%s\n", progName, action, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(body))
		if status == 404 {
			fmt.Fprintf(stderr, "⇒ 主控**没有这条路由**（`POST %s` 未接）—— 需重编 + 重启主控后才有\n", path)
		}
		return exitUsage
	}
	var resp jsonObj
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		inv.setErr("upstream_error", "bad_itask_payload", err.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	// 变更即回状态：投影 `message` / `state` 两块（字段名逐字取载荷 —— 不猜键名）。
	want := []string{"message", "state"}
	row := map[string]string{}
	keys := []string{}
	for _, k := range want {
		if _, ok := resp[k]; ok {
			keys = append(keys, k)
			row[k] = cell(resp[k])
		}
	}
	if len(keys) == 0 {
		inv.setErr("blocked", "unexpected_payload", "载荷里没有期望的键（不给结论）")
		fmt.Fprintf(stderr, "%s: %s 的载荷里没有期望的键（%v）⇒ 不给结论（退码 8）\n", progName, path, want)
		return exitBlocked
	}
	return listCmd(inv, stdout, stderr, keys, []map[string]string{row})
}
