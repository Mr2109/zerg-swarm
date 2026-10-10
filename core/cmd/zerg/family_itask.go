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
	"net/url"
	"strconv"
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

// ---- `itask interval set` 写面（D2 · 设/取内部任务的循环周期 · P0 #232 第三面 · 2026-10-08）----
//
// 现读写路径（本票只开这一条）：`zerg itask interval set <任务|all> --hours N`
//   ⇒ POST `/api/internal-tasks/{id}/interval` `{"hours": N}`
//   （主控侧路由已存在 · `core/cmd/zerg-core/main.go` → `api.InternalIntervalHandler`）
//   ⇒ `SetInternalInterval(id, hours)` ⇒ 持久化 `<任务目录根>/intervals.json`（重启恢复）。
//   后端范围（`core/internal/api/internal_interval.go` 的 `req.Hours < 0 || > 24*30` 同一档）：`0（取消）~ 720`。
//
// 口径逐字照同族写面先例（`itask stop`〔本件〕＋ `sched set`〔`family_sched.go`〕· §九 M3 C1/C2/C4）：
//
//   - D2 档（有影响但可逆：再设回旧周期即回原态）；走自家确认档门 `cmdItaskIntervalSetGuarded`；
//   - `--dry-run`：只出**计划件**、零副作用、退 0（档序：干跑先于确认档）；
//   - `--confirm=<本机名>` + `--yes`：fail-closed（缺一即退 2，从不提问）；
//   - `--dry-run` 与 `--yes` 同给 ⇒ `dryRunYesConflict` 判用法错 2。
//
// `<任务|all>`：给任务 id ⇒ 设那一条；给 `all` ⇒ 现读 `GET /api/internal-tasks` 取 16 类 id 逐条设。
// ★ 本票**不开放** `itask mode set`（另面待拍）—— 命令树、帮助面、危险档三处一律不登记。

// itaskIntervalMaxHours —— 周期上界（与后端 `internal_interval.go` 的 `24*30` 同一档；0 = 取消）。
const itaskIntervalMaxHours = 24 * 30

// cmdItaskIntervalSetGuarded —— `zerg itask interval set` 的唯一执行门（D2 · 确认档齐 ⇒ 转真写）。
func cmdItaskIntervalSetGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺任务 id（或 all）")
		fmt.Fprintf(stderr, "%s: `itask interval set` 要一个任务 id 或 `all`（如 `%s itask interval set health-check --hours 6`）\n", progName, progName)
		return exitUsage
	}
	target := strings.TrimSpace(inv.args[0])
	// `--hours N` 必填值旗标：缺 / 坏值 / 越界 ⇒ 用法错 2 · **零写**（判在 dry-run 与写之前）。
	if !inv.hasFlag("--hours") {
		inv.setErr("usage", "missing_flag", "缺 --hours N")
		fmt.Fprintf(stderr, "%s: `itask interval set` 要 --hours N（0 = 取消 · 范围 0~%d 小时）\n", progName, itaskIntervalMaxHours)
		return exitUsage
	}
	raw := strings.TrimSpace(inv.flagVal("--hours"))
	hours, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		inv.setErr("usage", "bad_flag_value", "周期要一个数（小时）")
		fmt.Fprintf(stderr, "%s: `--hours` 要一个数（小时）—— 给的是 %q（0 = 取消 · 范围 0~%d）\n", progName, raw, itaskIntervalMaxHours)
		return exitUsage
	}
	if hours < 0 || hours > float64(itaskIntervalMaxHours) {
		inv.setErr("usage", "bad_flag_value", "周期范围 0~720 小时")
		fmt.Fprintf(stderr, "%s: 周期范围 0（取消）~ %d 小时 —— 给的是 %g\n", progName, itaskIntervalMaxHours, hours)
		return exitUsage
	}
	if inv.dryRun {
		emitItaskIntervalSetPlan(stdout, stderr, target, hours)
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "itask interval set"); rc != exitOK {
		return rc
	}
	return cmdItaskIntervalSetWrite(inv, stdout, stderr, target, hours)
}

// emitItaskIntervalSetPlan —— `itask interval set --dry-run` 的计划件（人面 · stderr 一行收口 · 零副作用）。
func emitItaskIntervalSetPlan(stdout, stderr io.Writer, target string, hours float64) {
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s itask interval set\n", progName)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再设回旧周期即回原态；要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  目标     : %s\n", target)
	fmt.Fprintf(stdout, "  请求     : POST %s/api/internal-tasks/%s/interval {\"hours\": %g}\n", newClient().base, target, hours)
	fmt.Fprintf(stdout, "  它会动   : 把该内部任务的循环周期设成 %g 小时（**持久化**到 intervals.json —— 0 = 取消；主控 goroutine 每 60s 检查到点）\n", hours)
	fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	fmt.Fprintf(stdout, "  来源     : §7.1 P8 · §5.1 POST /api/internal-tasks/{id}/interval · 同族写面先例 `itask stop`（D2 + --confirm + --yes）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdItaskIntervalSetWrite —— 真写（确认档齐后）：一条或 `all` ⇒ POST `/api/internal-tasks/{id}/interval`，
// 再把落点值**回读**（`GET /api/internal-tasks/intervals`）逐条核（落地值回读）。
func cmdItaskIntervalSetWrite(inv *invocation, stdout, stderr io.Writer, target string, hours float64) int {
	ids := []string{}
	if target == "all" {
		var resp jsonObj
		if rc := fetchInv(inv, newClient(), "/api/internal-tasks", &resp, stderr); rc != exitOK {
			return rc
		}
		for _, it := range asList(resp["items"]) {
			if o := asObj(it); o != nil {
				if id := strings.TrimSpace(cell(o["id"])); id != "" {
					ids = append(ids, id)
				}
			}
		}
		if len(ids) == 0 {
			inv.setErr("blocked", "empty_targets", "`all` 取不到任何任务 id（不给结论）")
			fmt.Fprintf(stderr, "%s: `all` 要从 GET /api/internal-tasks 取任务 id，载荷里 items 是空的 ⇒ 不给结论（退码 8）\n", progName)
			return exitBlocked
		}
	} else {
		ids = append(ids, target)
	}
	body := fmt.Sprintf("{\"hours\":%g}", hours)
	rows := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		status, respBody, err := postJSON(newClient(), "/api/internal-tasks/"+url.PathEscape(id)+"/interval", body)
		var he *httpError
		if err != nil && asHTTP(err, &he) {
			status, respBody, err = he.status, he.body, nil
		}
		if err != nil {
			inv.setErr("unreachable", "dial_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
			return exitUnreachable
		}
		if status == 401 || status == 403 {
			inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
			fmt.Fprintf(stderr, "%s: 主控拒了这次设周期（http_%d）—— 看 `zerg help credentials`\n", progName, status)
			return exitAuth
		}
		if status >= 400 {
			code := errorCodeOf(respBody)
			inv.setErr("usage", code, "主控拒绝设周期")
			fmt.Fprintf(stderr, "%s: 主控拒绝设周期（http_%d）· code=%s\n", progName, status, orDash(code))
			fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
			return exitUsage
		}
		var resp jsonObj
		if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
			inv.setErr("upstream_error", "bad_itask_payload", err.Error())
			fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
			return exitFail
		}
		rows = append(rows, map[string]string{"id": id, "hours": cell(resp["hours"]), "message": cell(resp["message"])})
	}
	// 落地值回读：把现读的 intervals 逐条核进结果面（`set` 报的 hours 与回读值同源对照）。
	if landed, ok := itaskIntervalReadBack(inv, stderr); ok {
		for _, r := range rows {
			r["landed"] = landed[r["id"]]
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "hours", "landed", "message"}, rows)
}

// itaskIntervalReadBack —— 回读现读的周期表（`GET /api/internal-tasks/intervals`）；取不到 ⇒ ok=false（不吞错）。
func itaskIntervalReadBack(inv *invocation, stderr io.Writer) (map[string]string, bool) {
	var resp jsonObj
	if rc := fetchInv(inv, newClient(), "/api/internal-tasks/intervals", &resp, stderr); rc != exitOK {
		return nil, false
	}
	out := map[string]string{}
	if m := asObj(resp["intervals"]); m != nil {
		for k, v := range m {
			out[k] = cell(v)
		}
	}
	return out, true
}

// ---- `itask mode set` 写面（D2 · 设内部任务的自动运行开关 · 缺口 GAP-20260927-205 · 2026-10-10）----
//
// 现读写路径（本票只开这一条）：`zerg itask mode set <任务 id> --on|--off`
//   ⇒ POST `/api/internal-tasks/{id}/mode` `{"auto_run": true|false}`
//   （主控侧路由已存在 · `core/cmd/zerg-core/main.go` → `api.InternalModeHandler`）
//   ⇒ `SetInternalMode(id, auto)` ⇒ 持久化 `<任务目录根>/internal_modes.json`（重启恢复）。
//   `--on`（auto_run=true · 自动运行）/`--off`（auto_run=false · 手动运行）**恰好给一枚**（同给/都不给 ⇒ 用法错 2）。
//
// 口径逐字照同族写面先例（`itask interval set`〔本件〕＋ `itask stop` · §九 M3 C1/C2/C4）：
//   - D2 档（有影响但可逆：再设回原开关即回原态）；走自家确认档门 `cmdItaskModeSetGuarded`；
//   - `--dry-run`：只出**计划件**、零副作用、退 0（档序：干跑先于确认档）；
//   - `--confirm=<本机名>` + `--yes`：fail-closed（缺一即退 2，从不提问）；
//   - `--dry-run` 与 `--yes` 同给 ⇒ `dryRunYesConflict` 判用法错 2。
//   ★ 本票**不做删除面**（`itask rm` 另面待拍）—— 关比删安全（可回滚、不丢清单）。

// cmdItaskModeSetGuarded —— `zerg itask mode set` 的唯一执行门（D2 · 确认档齐 ⇒ 转真写）。
func cmdItaskModeSetGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺任务 id")
		fmt.Fprintf(stderr, "%s: `itask mode set` 要一个任务 id（先 `%s itask ls`）⇒ 退码 2\n", progName, progName)
		return exitUsage
	}
	target := strings.TrimSpace(inv.args[0])
	// `--on` / `--off` 恰好给一枚（同给/都不给 ⇒ 用法错 2 · **零写** —— 判在干跑与写之前）。
	on := inv.hasFlag("--on")
	off := inv.hasFlag("--off")
	if on == off {
		inv.setErr("usage", "bad_flag_value", "`--on`/`--off` 要恰好给一枚")
		fmt.Fprintf(stderr, "%s: `itask mode set` 要 `--on` 或 `--off` **恰好给一枚**（如 `%s itask mode set health-check --off`）\n", progName, progName)
		return exitUsage
	}
	autoRun := on
	if inv.dryRun {
		emitItaskModeSetPlan(stdout, stderr, target, autoRun)
		return exitOK
	}
	if rc := schedConfirmGate(inv, stderr, "itask mode set"); rc != exitOK {
		return rc
	}
	return cmdItaskModeSetWrite(inv, stdout, stderr, target, autoRun)
}

// emitItaskModeSetPlan —— `itask mode set --dry-run` 的计划件（人面 · stderr 一行收口 · 零副作用）。
func emitItaskModeSetPlan(stdout, stderr io.Writer, target string, autoRun bool) {
	mode := "手动运行（off）"
	if autoRun {
		mode = "自动运行（on）"
	}
	fmt.Fprintln(stdout, schedPlanHeader)
	fmt.Fprintf(stdout, "  动作     : %s itask mode set\n", progName)
	fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再设回原开关即回原态；要 --confirm=<本机名> 与 --yes）\n", dangerD2)
	fmt.Fprintf(stdout, "  目标     : %s\n", target)
	fmt.Fprintf(stdout, "  请求     : POST %s/api/internal-tasks/%s/mode {\"auto_run\": %t}\n", newClient().base, target, autoRun)
	fmt.Fprintf(stdout, "  它会动   : 把该内部任务的自动运行开关设成 %s（**持久化**到 internal_modes.json —— 重启恢复）\n", mode)
	fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	fmt.Fprintf(stdout, "  来源     : §7.1 P8 · §5.1 POST /api/internal-tasks/{id}/mode · 缺口 GAP-20260927-205 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes）\n")
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
}

// cmdItaskModeSetWrite —— 真写（确认档齐后）：POST `/api/internal-tasks/{id}/mode`，
// 再把落点值**回读**（`GET /api/internal-tasks/modes`）逐条核（落地值回读）。
func cmdItaskModeSetWrite(inv *invocation, stdout, stderr io.Writer, target string, autoRun bool) int {
	body := fmt.Sprintf("{\"auto_run\":%t}", autoRun)
	status, respBody, err := postJSON(newClient(), "/api/internal-tasks/"+url.PathEscape(target)+"/mode", body)
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		status, respBody, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次设模式（http_%d）—— 看 `zerg help credentials`\n", progName, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		inv.setErr("usage", code, "主控拒绝设模式")
		fmt.Fprintf(stderr, "%s: 主控拒绝设模式（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		if status == 404 {
			fmt.Fprintf(stderr, "⇒ 主控**没有这条路由**（`POST /api/internal-tasks/{id}/mode` 未接）—— 需重编 + 重启主控后才有\n")
		}
		return exitUsage
	}
	var resp jsonObj
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		inv.setErr("upstream_error", "bad_itask_payload", err.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	row := map[string]string{"id": target, "auto_run": cell(resp["auto_run"]), "message": cell(resp["message"])}
	// 落地值回读：把现读的 modes 核进结果面（`set` 报的 auto_run 与回读值同源对照）。
	if landed, ok := itaskModeReadBack(inv, stderr); ok {
		if v, hit := landed[target]; hit {
			row["landed"] = v
		} else {
			row["landed"] = "（未列出 ⇒ 默认自动运行）"
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "auto_run", "landed", "message"}, []map[string]string{row})
}

// itaskModeReadBack —— 回读现读的模式表（`GET /api/internal-tasks/modes`）；取不到 ⇒ ok=false（不吞错）。
func itaskModeReadBack(inv *invocation, stderr io.Writer) (map[string]string, bool) {
	var resp jsonObj
	if rc := fetchInv(inv, newClient(), "/api/internal-tasks/modes", &resp, stderr); rc != exitOK {
		return nil, false
	}
	out := map[string]string{}
	if m := asObj(resp["modes"]); m != nil {
		for k, v := range m {
			out[k] = cell(v)
		}
	}
	return out, true
}
