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
	"fmt"
	"io"
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
