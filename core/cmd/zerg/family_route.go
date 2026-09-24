// family_route.go —— `route` 族（**显式指定机器**）：`pin` / `ls` / `unpin` 三条（2026-09-24）。
//
// 设计稿（逐字底账）：`Zerg-内部文档/项目文档/v2.5.12/设计-指定机器路由-v1.0-20260924.md`。
// 它回答的那个现场问题（逐字）：子代理**没法把模型钉到指定机器** —— 网关择优默认挑本机，
// 请求落在 Mr2109 的 `llama-server`，而人要的是 x3；当时只有「手工 `POST /api/control/unload
// {"machine":"Mr2109"}` 绕路」这一条路。
//
// 三条命令的分工（「单独定制 > 路由默认规则」的人面）：
//
//	pin   钉一条：`route pin --model <模型> --machine <机器> --ttl <时长>`（**必须带时效**）
//	ls    查覆盖表：`route ls [--model <模型>]`（只读 · 不碰盘 · 不探主控）
//	unpin 撒（撤回）：`route unpin [--model <模型>]`（不给 `--model` ⇒ 撒全部 = 一行撤回）
//
// 四条写死（与设计稿的约束一一对应）：
//
//	① **唯一写口**：覆盖表（`routepin.Path()` · 状态目录下一件 · **不在任何仓里**）只有本族在写。
//	   选机那道门（`core/internal/gateway/route_pin.go`）**只读** ⇒ 「模型（请求面）写不到覆盖表」。
//	② **TTL 是必需品**：`pin` 不给 `--ttl` ⇒ 执行前判**拒**（`0` 也不是「不过期」）——
//	   「偶尔的要求」不许变成永久改变默认。
//	③ **一行撤回**：`route unpin`（幂等：没撒到东西 ⇒ 0 + `changed=false`，一个字节都不写）。
//	④ **三态照本仓写面先例**（`repo commit` / `doc meta fill` / `gap add` 同一条口径）：
//	   `--dry-run` ⇒ 计划件走 **stdout**、零副作用（一个字节不写）· 缺 `--yes` ⇒ 计划件走 **stderr**、退 `2` ·
//	   `--yes` 才真写。**从不提问、从不交互**。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：
//
//	pin   : 0 钉住（或幂等命中）· 2 用法错（缺模型/缺机器/缺时效/时长认不出/写旗标冲突）· 8 覆盖表读不到或写不进
//	ls    : 0 出表（**空表也是 0** —— 「没钉」是默认态，不是错）· 2 用法错 · 8 覆盖表读不到
//	unpin : 0 撒掉（或幂等命中）· 2 用法错 · 8 覆盖表读不到或写不进
//
// 两条**设计稿未钉死、本件按最小惊讶取定**的（回执里照实点名）：
//
//	· 时长口径**复用** `approve` 那一套（`approveTTLOf` + `30m`/`72h`/`1h30m`，另加 `7d`）——
//	  不另立第二份时长判据（「同一件事不许两份真源」）。
//	· 同一模型**至多一行**（钉两次 = 覆盖，不叠行）；`ls` 会把过期行也显出来（「过期的也要看得见」）。
package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/routepin"
)

// routeFields —— 三条命令**共用**的五格（顺序即人面列序）。
//
// ★ 必须写成 `[]string{…}` **字面量**：`scripts/gates/check-cli-contract.py` 的 `FIELDS_RE`
// 只认这一种形状（抽成变量 = 该条被读成「没有字段表」）—— 与 `net probe` / `publish tree has`
// / `ci green` 那三条**同一条纪律**。
var routeFields = []string{"model", "machine", "expires_at", "remaining_s", "state"}

// 状态闭集（人面三值；机器面同一个词 —— 同一件事只有一个写法）。
const (
	routeStateActive  = "生效"
	routeStateExpired = "已过期"
	routeStateRevoked = "已撤"
)

// routeModelOf —— 本族取 `--model` 的**唯一**口。
//
// ★ 为什么不是 `inv.flagVal("--model")`：`--model` 有一处**专用收口**（`setValueFlag` 的
// `--model` 分支进 `inv.modelWant`，**不进 `kv`**）—— 照 `family_config.go:290–292` 那条
// 现读教训写的（那里头同样点名了这个坑）。用错口 ⇒ 「给了 `--model` 却报没给」。
func routeModelOf(inv *invocation) string {
	return strings.TrimSpace(inv.modelWant)
}

// routeCheckFields —— 字段面**先判**（未知字段 ⇒ 2 + stderr 列合法字段）。
//
// ★ 为什么必须**先判**、不能等渲染到行：件为 0 条时 `listCmd` 那条路会直接出「共 0 条」并退 0，
// 于是「未知字段」在空表下**悄悄变成合法**（`family_approve.go:199–203` 现读抓到的同一条病：
// 同一个 case 在空目录退 0、在有件的目录退 2 ⇒ 判据飘）。本族判据件就钉这一格。
func routeCheckFields(inv *invocation, stderr io.Writer) int {
	if !inv.jsonGiven || len(inv.fields) == 0 {
		return exitOK
	}
	legal := map[string]bool{}
	for _, f := range fieldListOf(inv.path) {
		legal[f] = true
	}
	for _, f := range inv.fields {
		if !legal[f] {
			return reportBadField(stderr, inv.path, f)
		}
	}
	return exitOK
}

// routeRow —— 一行 → 五格投影（`remaining_s` 一律是**秒**的十进制字符串；过期 ⇒ `0`）。
func routeRow(p routepin.Pin, now time.Time, state string) map[string]string {
	rem := int64(0)
	if state == routeStateActive {
		if d := p.Remaining(now); d > 0 {
			rem = int64(d / time.Second)
		}
	}
	return map[string]string{
		"model":       p.Model,
		"machine":     p.Machine,
		"expires_at":  p.ExpiresAt,
		"remaining_s": fmt.Sprintf("%d", rem),
		"state":       state,
	}
}

// routeHumanRow —— 人面一行（`pin`/`unpin` 的回吐用它；`ls` 走 `listCmd` 的表）。
func routeHumanRow(stdout io.Writer, p routepin.Pin, now time.Time, state string) {
	rem := "0s"
	if state == routeStateActive {
		if d := p.Remaining(now); d > 0 {
			rem = d.Truncate(time.Second).String()
		}
	}
	fmt.Fprintf(stdout, "  模型 %s → 机器 %s ｜ 到期 %s（剩 %s）｜ %s\n",
		p.Model, p.Machine, p.ExpiresAt, rem, state)
}

// routeLoadTable —— 读覆盖表（**唯一**读法）。失败 ⇒ `8`（不给结论）+ 计划件/口径都照实点名落点。
func routeLoadTable(inv *invocation, stderr io.Writer) (*routepin.Table, string, int) {
	path := routepin.Path()
	t, err := routepin.Load(path)
	if err != nil {
		inv.setErr("blocked", "route_pins_unreadable", "覆盖表读不到 / 解读不了")
		fmt.Fprintf(stderr, "%s: 覆盖表**读不到**（%s）：%v\n", progName, path, err)
		fmt.Fprintf(stderr, "%s: 照「读不到 ≠ 没有覆盖」那条口径 ⇒ 退码 8（不给结论），不猜、不静默降级\n", progName)
		return nil, path, exitBlocked
	}
	return t, path, exitOK
}

// routeWriteGate —— **唯一**的写门（三态）。
//
//	return (rc, 放行?)：`--dry-run` ⇒ (0, false) 计划件走 stdout；缺 `--yes` ⇒ (2, false) 计划件走 stderr；
//	`--yes` ⇒ (0, true) 真写。
//
// 为什么计划件在两态走两条流（逐字照 `family_gap.go:24` 先例）：`--dry-run` 那一态的计划件**就是
// 它的结果**（走 stdout）；缺 `--yes` 那一态是**没执行**（走 stderr）—— 这也是本族矩阵格
// `want_stdout_bytes = 0` 能成立的前提。
func routeWriteGate(inv *invocation, stdout, stderr io.Writer, act, path, effect string, plan []string) (int, bool) {
	if inv.dryRun {
		fmt.Fprintf(stdout, "计划件（--dry-run · 零副作用 —— 未执行、覆盖表一个字节没动）\n")
		fmt.Fprintf(stdout, "  动作     : %s %s\n", progName, act)
		fmt.Fprintf(stdout, "  覆盖表   : %s\n", path)
		for _, ln := range plan {
			fmt.Fprintf(stdout, "  %s\n", ln)
		}
		fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n")
		return exitOK, false
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `%s %s` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName, progName, act)
		fmt.Fprintf(stderr, "  覆盖表   : %s\n", path)
		for _, ln := range plan {
			fmt.Fprintf(stderr, "  %s\n", ln)
		}
		fmt.Fprintf(stderr, "  它会动   : %s\n", effect)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, act)
		return exitUsage, false
	}
	return exitOK, true
}

// routeSaveAndVerify —— 写 + **写后读回对拍**（「写了但读回来不是它」不许当成功）。
func routeSaveAndVerify(inv *invocation, stderr io.Writer, t *routepin.Table, path, model, wantMachine string) int {
	if err := t.Save(path); err != nil {
		inv.setErr("blocked", "route_pins_unwritable", "覆盖表写不进")
		fmt.Fprintf(stderr, "%s: 覆盖表**写不进**（%s）：%v\n", progName, path, err)
		fmt.Fprintf(stderr, "%s: 照「写失败即拒」那条口径 ⇒ 退码 8（不给结论）—— 真源一个字节都没改成\n", progName)
		return exitBlocked
	}
	back, err := routepin.Load(path)
	if err != nil {
		inv.setErr("blocked", "route_pins_verify_failed", "覆盖表写后读不回")
		fmt.Fprintf(stderr, "%s: 写后**读回对拍**失败（%s）：%v ⇒ 退码 8\n", progName, path, err)
		return exitBlocked
	}
	if wantMachine == "" {
		return exitOK
	}
	if p, ok := back.HostFor(model, time.Now()); !ok || p.Machine != wantMachine {
		inv.setErr("blocked", "route_pins_verify_failed", "写后读回的那一行不是刚写下的那一行")
		fmt.Fprintf(stderr, "%s: 写后**读回对拍**失败：模型 %s 读回来不是 %s ⇒ 退码 8（不许「写了但读不回来」）\n",
			progName, model, wantMachine)
		return exitBlocked
	}
	return exitOK
}

// cmdRoutePin —— `zerg route pin --model <模型> --machine <机器> --ttl <时长>`（钉 · D2 写面）。
func cmdRoutePin(inv *invocation, stdout, stderr io.Writer) int {
	if rc := routeCheckFields(inv, stderr); rc != exitOK {
		return rc
	}
	model := routeModelOf(inv)
	machine := strings.TrimSpace(inv.flagVal("--machine"))
	if model == "" {
		inv.setErr("usage", "model_required", "`route pin` 要 `--model <模型>`")
		fmt.Fprintf(stderr, "%s: `route pin` **要 `--model <模型>`**（钉的是「哪个模型走哪台机器」这一对）\n", progName)
		fmt.Fprintf(stderr, "例：%s route pin --model example-35b-v2 --machine x3 --ttl 2h --yes\n", progName)
		return exitUsage
	}
	if machine == "" {
		inv.setErr("usage", "machine_required", "`route pin` 要 `--machine <机器>`")
		fmt.Fprintf(stderr, "%s: `route pin` **要 `--machine <机器>`**（机器名与 `fleet.yaml` 的 `fleet:` 段同名）\n", progName)
		fmt.Fprintf(stderr, "例：%s route pin --model %s --machine x3 --ttl 2h --yes\n", progName, model)
		return exitUsage
	}
	// `--ttl` 复用 `approve` 那一套（同一枚旗标、同一套时长口径 —— 不另立第二份判据）。
	ttl, ttlGiven, ttlErr := approveTTLOf(inv)
	if ttlErr != nil {
		inv.setErr("usage", "bad_ttl", ttlErr.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 不钉（退码 2）\n", progName, ttlErr)
		return exitUsage
	}
	if !ttlGiven {
		inv.setErr("usage", "ttl_required", "`route pin` 必须带 `--ttl <时长>`")
		fmt.Fprintf(stderr, "%s: `route pin` **必须带 `--ttl <时长>`** —— 不带时效的钉 = **永久改变默认** ⇒ 本命令不收\n", progName)
		fmt.Fprintf(stderr, "例：%s route pin --model %s --machine %s --ttl 2h --yes\n", progName, model, machine)
		return exitUsage
	}
	tbl, path, rc := routeLoadTable(inv, stderr)
	if rc != exitOK {
		return rc
	}
	now := time.Now()
	dropped := tbl.Prune(now) // 写的时候顺手清过期行（读者永不删行）
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "local"
	}
	p := routepin.Pin{
		Model:      model,
		Machine:    machine,
		CreatedAt:  now.Format(time.RFC3339),
		ExpiresAt:  now.Add(ttl).Format(time.RFC3339),
		TTLSeconds: int64(ttl / time.Second),
		By:         by,
		Note:       strings.TrimSpace(inv.flagVal("--note")),
	}
	prev, prevActive := tbl.HostFor(model, now)
	same := prevActive && prev.Machine == machine && prev.ExpiresAt == p.ExpiresAt
	plan := []string{
		fmt.Sprintf("钉住     : 模型 %s → 机器 %s", model, machine),
		fmt.Sprintf("时效     : %s（到期 %s）", ttl.Truncate(time.Second), p.ExpiresAt),
		fmt.Sprintf("表里现状 : %s", routeExistingLine(prev, prevActive, len(tbl.Pins), len(dropped))),
		"写什么   : 覆盖表整件原子重写（临时件 + fsync + rename）；同一模型至多一行（钉两次 = 覆盖）",
		fmt.Sprintf("撤回一行 : %s route unpin --model %s --yes", progName, model),
	}
	rc, goOn := routeWriteGate(inv, stdout, stderr, "route pin", path, "把这一条写进覆盖表（可逆：`route unpin` 或等它自己到期）", plan)
	if !goOn {
		return rc
	}
	if same {
		changed := false
		inv.changed = &changed
		fmt.Fprintf(stderr, "%s: 已在该状态（模型 %s 已钉在 %s、到期时刻逐字相同）⇒ **幂等：覆盖表一个字节都没写**\n", progName, model, machine)
		return routeEmit(inv, stdout, stderr, routeRow(prev, now, routeStateActive), routeStateActive, path, model)
	}
	tbl.Upsert(p)
	if rc := routeSaveAndVerify(inv, stderr, tbl, path, model, machine); rc != exitOK {
		return rc
	}
	changed := true
	inv.changed = &changed
	return routeEmit(inv, stdout, stderr, routeRow(p, now, routeStateActive), routeStateActive, path, model)
}

// routeExistingLine —— 计划件里的「表里现状」那一行（三种形态分开说，不许混成一句）。
func routeExistingLine(prev routepin.Pin, active bool, total, dropped int) string {
	tail := fmt.Sprintf("表内共 %d 条", total)
	if dropped > 0 {
		tail += fmt.Sprintf("（顺手清掉已过期 %d 条）", dropped)
	}
	if !active {
		return "这个模型没有生效的行 ⇒ 一切走默认择优；" + tail
	}
	return fmt.Sprintf("模型 %s 现钉在 %s（到期 %s）⇒ 这一条会被**覆盖**；%s", prev.Model, prev.Machine, prev.ExpiresAt, tail)
}

// cmdRouteLs —— `zerg route ls [--model <模型>]`（查 · **只读**：不碰盘、不探主控、零网络）。
func cmdRouteLs(inv *invocation, stdout, stderr io.Writer) int {
	if rc := routeCheckFields(inv, stderr); rc != exitOK {
		return rc
	}
	tbl, path, rc := routeLoadTable(inv, stderr)
	if rc != exitOK {
		return rc
	}
	model := routeModelOf(inv)
	now := time.Now()
	rows := []map[string]string{}
	active, expired := 0, 0
	for _, p := range tbl.RowsOf(model) {
		state := routeStateExpired
		if p.Active(now) {
			state = routeStateActive
			active++
		} else {
			expired++
		}
		rows = append(rows, routeRow(p, now, state))
	}
	fmt.Fprintf(stderr, "%s: 覆盖表 = %s · 条数 = %d（生效 %d · 已过期 %d）· 只读（本命令一个字节都不写）\n",
		progName, path, len(rows), active, expired)
	fmt.Fprintf(stderr, "%s: 生效语义 = 选机那道门按 (mtime,size) 热读覆盖表 ⇒ 钉/撒**不必重启主控**"+
		"（前提：主控跑的是**带这道门**的版本；不带门的版本对它一个字都不看）\n", progName)
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 零条 —— 「没钉任何模型」是**默认态**（不是错）⇒ 一切走默认择优（退码 0）\n", progName)
	}
	return listCmd(inv, stdout, stderr, routeFields, rows)
}

// cmdRouteUnpin —— `zerg route unpin [--model <模型>]`（撒 · D2 写面 · **一行撤回**）。
func cmdRouteUnpin(inv *invocation, stdout, stderr io.Writer) int {
	if rc := routeCheckFields(inv, stderr); rc != exitOK {
		return rc
	}
	tbl, path, rc := routeLoadTable(inv, stderr)
	if rc != exitOK {
		return rc
	}
	model := routeModelOf(inv)
	now := time.Now()
	dropped := tbl.Prune(now)
	removed := tbl.Remove(model)
	scope := "**全部**"
	if model != "" {
		scope = fmt.Sprintf("模型 %s", model)
	}
	plan := []string{
		fmt.Sprintf("撒掉     : %s（%d 条）", scope, len(removed)),
		"写什么   : 覆盖表整件原子重写；撒完这一条 ⇒ 该模型的选机**立刻回默认择优**",
	}
	if len(dropped) > 0 {
		plan = append(plan, fmt.Sprintf("顺手清掉 : 已过期 %d 条", len(dropped)))
	}
	rc, goOn := routeWriteGate(inv, stdout, stderr, "route unpin", path, "把点名的行从覆盖表里去掉（可逆：再 `route pin` 一次）", plan)
	if !goOn {
		return rc
	}
	if len(removed) == 0 && len(dropped) == 0 {
		changed := false
		inv.changed = &changed
		fmt.Fprintf(stderr, "%s: 覆盖表里没有可撒的行（%s）⇒ **幂等：一个字节都没写**；该模型本来就走默认择优\n", progName, scope)
		// ★ 幂等那一档**也要给读数**（§九 M4：「不许在目标已存在时静默什么都不做 —— 要么
		// `changed:false` 说明，要么按档执行」）。机器面：空 items + `meta.changed=false`；
		// 人面：一行说明走 stdout（与真撒了那一档同一条流，调用方不必按退码猜）。
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSONList(stdout, stderr, inv, inv.path, routeFields, []map[string]string{})
		}
		fmt.Fprintf(stdout, "%s route unpin：0 条可撒（%s）｜ changed=false ｜ 幂等：覆盖表一个字节没写（%s）\n",
			progName, scope, path)
		return exitOK
	}
	if rc := routeSaveAndVerify(inv, stderr, tbl, path, "", ""); rc != exitOK {
		return rc
	}
	changed := len(removed) > 0
	inv.changed = &changed
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		rows := []map[string]string{}
		for _, p := range removed {
			rows = append(rows, routeRow(p, now, routeStateRevoked))
		}
		return selectJSONList(stdout, stderr, inv, inv.path, routeFields, rows)
	}
	fmt.Fprintf(stdout, "%s route unpin：撒掉 %d 条（覆盖表 %s）\n", progName, len(removed), path)
	for _, p := range removed {
		routeHumanRow(stdout, p, now, routeStateRevoked)
	}
	if len(removed) == 0 {
		fmt.Fprintf(stdout, "  （只有已过期行被顺手清掉 %d 条 —— 该模型本来就不在覆盖表里）\n", len(dropped))
	}
	fmt.Fprintf(stdout, "  覆盖表   %s\n", path)
	fmt.Fprintf(stderr, "%s: 撒完 ⇒ 该模型的选机立刻回默认择优（钉住的那台不再优先）\n", progName)
	return exitOK
}

// routeEmit —— `pin` 的回吐（机器面 / 人面两条流，五格同一份投影）。
func routeEmit(inv *invocation, stdout, stderr io.Writer, row map[string]string, state, path, model string) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, routeFields, row)
	}
	fmt.Fprintf(stdout, "%s route pin：%s（覆盖表 %s）\n", progName, state, path)
	fmt.Fprintf(stdout, "  模型 %s → 机器 %s ｜ 到期 %s ｜ %s\n", row["model"], row["machine"], row["expires_at"], state)
	fmt.Fprintf(stdout, "  撤回一行 %s route unpin --model %s --yes\n", progName, model)
	fmt.Fprintf(stderr, "%s: 生效语义 = 即时（选机那道门按 (mtime,size) 热读覆盖表 ⇒ 钉/撒不必重启主控）\n", progName)
	return exitOK
}
