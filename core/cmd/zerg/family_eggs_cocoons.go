// family_eggs_cocoons.go —— 虫卵 / 虫茧两族（I / J 族 · §3.4 · §7.1 `P6` · §3.3 `N4` 硬占位表）。
//
// 口径（照定稿 + 本机现跑，**不编**）：
//
//	① **卵面 = 只读投影**（`Q-101` · 任务清单 `T4` · 波② · 2026-09-23）：一枚卵 = **（模型 id × 主机）**
//	   这一对。依据逐字：`Zerg-内部文档/项目文档/v2.5.11/调研-缺口-卵面孵化-20260923.md` §二.3 ——
//	   systemd 的「模板 + 实例」（`svc@arg.service`）与本仓「一枚卵 = 一个（模型 × 机）实例」同形，
//	   而 `Q-101` 的降级方案 `zerg egg adopt <模型 id> --host <机>` 正好就是这一对。
//	② 投影**只用现成端点**（**不新开一条路** ✗ —— `T4` 落点逐字）：
//	   `GET /api/fleet/models`（名册展开成候选 ⇒ `host` / `model` 两格）
//	   ＋ `GET /api/fleet/status`（各机心跳快照的 `models[]` ⇒ `state` 格 —— 这也是主控自己
//	   那份 `ModelDetailHandler` 读的同一份快照）＋ `GET /api/models/{name}`（主控的 `status` ⇒ **逐字对账**）。
//	   写面（`egg run`）**本波不做** ✗（押后 —— 起算 = 动生产）。
//	③ `state` **值域只有两个词**：`已加载` / `未加载` —— 与主控 `GET /api/models/{name}` 的
//	   `status` **逐字同源**（主控侧字面量的落点在 `core/internal/api/handlers.go` 的
//	   `ModelDetailHandler`）；命令面**不另造同义词** ✗（同一格不许两套词 ⇒ 判据②）。
//	④ **三格齐是机检的**（判据①）：任一枚卵少一格 ⇒ **判红**，绝不静默打一行空格
//	   （负控：把 `host` 那一格拉掉 ⇒ 必红）。
//	⑤ **茧面不在本波**：主控面今天仍**没有**茧的投影端点 ⇒ `cocoon ls` 照旧 fail-closed
//	   （`kind=blocked` · 退码 8 · 不给结论 ✓ —— 不是「读成了健康」）。`cocoon open` 会**起一个服务**
//	   （8610）⇒ 起进程档：只出计划件、不执行（真开要 Mr2109 拍）。
//	⑥ 两族的族名进 `N4` 硬占位表（§3.3）：定稿是冻结件，本件**不改它一个字节**；
//	   「把族名写进 `N4`」登记为**待拍**（下一版定稿时一次改）。
package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---- 卵面：三格 + `state` 闭集（值域与主控逐字同源）------------------------------------------

// eggStateLoaded / eggStateIdle —— 卵的 `state` **闭集只有这两个词**：它们就是主控
// `GET /api/models/{name}` 里 `status` 的字面量（`core/internal/api/handlers.go` 的
// `ModelDetailHandler`：`status = "已加载"` / `"未加载"`）。
// 命令面**只许引用、不许另造**（判据②「同一格不许两套词」）✗。
const (
	eggStateLoaded = "已加载"
	eggStateIdle   = "未加载"
)

// eggDisplay —— 人面 / 行式面的**三格**（判据①：每行三格齐；缺一格即红）。
var eggDisplay = []string{"host", "model", "state"}

// eggFields —— `--json` 的字段表：三格在前，另带 `egg_id` / `backend` / `mem_gb` 三格便于机器面
// 定位（除 `egg_id` 是**拼**出来的，其余字段名**逐字取自它投影的载荷键** —— 不另造词汇）。
var eggFields = []string{"egg_id", "host", "model", "state", "backend", "mem_gb"}

// eggRequired —— 「三格齐」机检用的字段表（少一格 ⇒ 红）。
var eggRequired = []string{"host", "model", "state"}

// egg_id 的分隔符：`<模型 id>@<主机>`（照 `svc@arg.service` 那一格的形状）。
const eggSep = "@"

// ---- 茧面 / 写面：仍缺的那一份判词 ------------------------------------------------------------

// eggsProjectionAbsent —— 主控面没有**茧**投影面时的那一份判词（`cocoon ls` · `cocoon build` 共用）。
//
// 为什么把它抽出来：几条**同因同果**，各写一遍判词就会漂（一处改了、三处没改）——
// 它们是同一个缺口的几个视角。★ 2026-09-23（波② `T4`）：**卵面已开**（只读投影 = 现成端点包装）
// ⇒ 本判词**只剩茧面**在用（卵的清单/单枚不再走它 —— `egg ls` / `egg show` 已能出矩阵 ✓）。
func eggsProjectionAbsent(inv *invocation, what string, stderr io.Writer) int {
	inv.setErr("blocked", "eggs_projection_absent", "主控面没有茧的投影端点（子端才有 /eggs）")
	fmt.Fprintf(stderr, "%s: 主控面**没有**茧的投影端点（%s ⇒ 现跑 404）\n", progName, what)
	fmt.Fprintf(stderr, "  子端 11 条端点里确有 `/eggs` · `/pin` · `/unpin` · `/services`（§三 I/J 族）\n")
	fmt.Fprintf(stderr, "⇒ %s **拿不到**，不给结论（退码 8 · kind=blocked · retryable=true）\n", what)
	fmt.Fprintf(stderr, "不做的：不偷偷直连子端、不拿本机文件顶替（§九 M20 O2「不偷偷」四条）\n")
	fmt.Fprintf(stderr, "虫卵面已开只读投影：`%s egg ls` / `%s egg show`（照现成端点包装 · 不新开一条路）\n", progName, progName)
	return exitBlocked
}

// ---- 卵面：唯一投影函数（`egg ls` / `egg show` 共用一份真源）----------------------------------

// eggSourceFail —— 三条源端点读不到时的**唯一**失败面：kind 分类与 `fetchInv` **同一套**
// （§九 M7 · 不打第二枪 `O2`/`O6`），不换端点、不读缓存当结果。
func eggSourceFail(inv *invocation, what string, err error, stderr io.Writer) int {
	kind, detail := "failed", ""
	var ae *authError
	var ne *netError
	var he *httpError
	switch {
	case errors.As(err, &ae):
		kind = "unauthenticated"
		if ae.status == 403 {
			kind = "forbidden"
		}
		detail = fmt.Sprintf("http_%d", ae.status)
	case errors.As(err, &ne):
		kind, detail = "unreachable", "dial_failed"
	case errors.As(err, &he):
		kind, detail = "upstream_error", fmt.Sprintf("http_%d", he.status)
	}
	inv.setErr(kind, detail, what+" 读不到")
	fmt.Fprintf(stderr, "%s: 卵面的源端点 %s 读不到：%v\n", progName, what, err)
	fmt.Fprintf(stderr, "error.kind=%s · retryable=%t · remedy=%s（§九 M7：重试判定只读 kind）\n",
		kind, retryableOf(kind), remedyOf(kind))
	return codeOfKind(kind)
}

// projectEggs —— 卵 × 设备矩阵的**唯一投影函数**（`egg ls` 与 `egg show` 共用，不各算一份）。
//
// 三条**现成**端点（一个都不新开 ✗）：
//
//	① `GET /api/fleet/models` —— 名册（每个 host 候选 = 一枚卵）⇒ `host` / `model`（＋ `backend` / `mem_gb`）
//	② `GET /api/fleet/status` —— 各机心跳快照的 `models[]` ⇒ 非主控点名的那些机的 `state` 格
//	③ `GET /api/models/{name}` —— 主控自己的 `host` + `status`：**主控点名那台机**上这一格
//	   **逐字取它**（不经过我们的推导 ⇒ 判据②「逐字对得上」在**构造上**成立，不是靠对拍）
//
// ★ 为什么主控点名那一格**必须逐字取主控的 `status`**（2026-09-23 现跑撞出来的）：先前那版
// 让每一格都「从快照自己算」再与主控 `status` 对拍，结果**重启主控后的心跳暖机窗口**里两处读数
// 会差一拍（快照刚重建、心跳还没补齐）⇒ 只读投影**当场判红**。那是把「两次读数的时序差」当成了
// 语义分歧 —— 只读投影要的是**可重复**：改成本位取主控原话 + 只对**闭集**判红（见下）。
//
// `only` 非空时只对**那个模型 id** 做第 ③ 步（`egg show` 用 —— 单枚卵不必把名册全拉一遍）。
//
// 返回：行（按 `host` 再按 `model` 排序 —— 矩阵要能逐次对拍/判红，不许每次换个次序）·
// 问题（非空 ⇒ 调用方判红）· 退码（非 0 时行不可用）。
func projectEggs(c *client, inv *invocation, stderr io.Writer, only string) ([]map[string]string, []string, int) {
	var fleet jsonObj
	if err := c.getJSON("/api/fleet/models", &fleet); err != nil {
		return nil, nil, eggSourceFail(inv, "卵名册 `/api/fleet/models`", err, stderr)
	}
	var snap jsonObj
	if err := c.getJSON("/api/fleet/status", &snap); err != nil {
		return nil, nil, eggSourceFail(inv, "各机驻留 `/api/fleet/status`", err, stderr)
	}
	// ② 各机在驻的模型集合（同一份快照真源 —— 主控自己也是读它）。
	resident := map[string]map[string]bool{}
	for host, mv := range asObj(snap["machines"]) {
		set := map[string]bool{}
		for _, m := range asList(asObj(mv)["models"]) {
			if s := cell(m); s != "" {
				set[s] = true
			}
		}
		resident[host] = set
	}
	// ① 名册展开成候选（每个候选 = 一枚卵；同一个模型的多台候选 = 多枚卵）。
	type cand struct{ host, backend, mem string }
	byModel := map[string][]cand{}
	order := []string{}
	for _, it := range asList(fleet["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		id := cell(o["id"])
		if id == "" {
			continue
		}
		if _, seen := byModel[id]; !seen {
			order = append(order, id)
		}
		byModel[id] = append(byModel[id], cand{cell(o["host"]), cell(o["backend"]), cell(o["mem_gb"])})
	}
	// ③ 主控自己的说法：`host`（它点名的机）+ `status`（它给那一格的原话）。
	problems := []string{}
	truth := map[string][2]string{} // 模型 id ⇒ (主控点名的机, 主控的 status 原话)
	for _, id := range order {
		if only != "" && id != only {
			continue
		}
		var detail jsonObj
		if err := c.getJSON("/api/models/"+id, &detail); err != nil {
			return nil, nil, eggSourceFail(inv, "主控 `GET /api/models/"+id+"`", err, stderr)
		}
		host, status := cell(detail["host"]), cell(detail["status"])
		truth[id] = [2]string{host, status}
		// 值域机检（判据②：**同一格不许两套词**）：主控给的 `status` 必须还在那两个词里。
		// 它一旦扩词 ⇒ 卵面**不许跟着漂**，也不许把它原样打出去假装没事 ⇒ 判红，交人定夺。
		if status != eggStateLoaded && status != eggStateIdle {
			problems = append(problems,
				fmt.Sprintf("%s：主控 `GET /api/models/%s` 的 `status`=%q **不在卵面 `state` 闭集**（%s / %s）",
					id, id, status, eggStateLoaded, eggStateIdle))
		}
		known := false
		for _, cd := range byModel[id] {
			if cd.host == host {
				known = true
			}
		}
		if host != "" && !known {
			problems = append(problems,
				fmt.Sprintf("%s：主控点名的机 %q 不在名册候选里（两处名册读数对不上）", id, host))
		}
	}
	rows := []map[string]string{}
	for _, id := range order {
		for _, cd := range byModel[id] {
			state := eggStateIdle
			if t, ok := truth[id]; ok && t[0] == cd.host && t[1] != "" {
				state = t[1] // 主控点名的机 ⇒ **逐字取主控原话**（判据②的构造性保证）
			} else if resident[cd.host][id] {
				state = eggStateLoaded
			}
			rows = append(rows, map[string]string{
				"egg_id":  id + eggSep + cd.host,
				"host":    cd.host,
				"model":   id,
				"state":   state,
				"backend": cd.backend,
				"mem_gb":  cd.mem,
			})
		}
	}
	// 排序：矩阵是拿来对拍的（`state` 逐字对账 + 负控），次序**不许每次变**。
	sort.Slice(rows, func(i, j int) bool {
		if rows[i]["host"] != rows[j]["host"] {
			return rows[i]["host"] < rows[j]["host"]
		}
		return rows[i]["model"] < rows[j]["model"]
	})
	return rows, problems, exitOK
}

// eggRowBroken —— 判据①「三格齐」的**机检**：返回第一个空格的字段名（齐 ⇒ 空串）。
//
// 为什么要有它（不是「顺手加的」）：一枚卵 = **（模型 × 机）**这一对 —— 少了 `host` 那一格，
// 这一行**就不再是一枚卵**（`egg_id` 都拼不出来）⇒ 必须判红、绝不静默打一行空格。
func eggRowBroken(row map[string]string) string {
	for _, f := range eggRequired {
		if strings.TrimSpace(row[f]) == "" {
			return f
		}
	}
	return ""
}

// eggRowsGuard —— 行面共用的两道守卫（① 三格齐 · ② `state` 逐字对账）。过了 ⇒ exitOK。
func eggRowsGuard(inv *invocation, rows []map[string]string, problems []string, stderr io.Writer) int {
	for i, r := range rows {
		if f := eggRowBroken(r); f != "" {
			inv.setErr("failed", "egg_cell_absent", fmt.Sprintf("第 %d 枚卵的 %s 格是空的", i+1, f))
			fmt.Fprintf(stderr, "%s: 第 %d 枚卵的 `%s` 格是空的 ⇒ 卵面**三格不许缺**（判据①）⇒ 判红\n",
				progName, i+1, f)
			fmt.Fprintf(stderr, "（一枚卵 = 模型 id × 主机这一对；少一格就不是一枚卵 —— 命令面不静默打空格）\n")
			return exitFail
		}
	}
	if len(problems) > 0 {
		inv.setErr("failed", "egg_state_verbatim_mismatch", "卵的 state 与主控 status 逐字对不上")
		fmt.Fprintf(stderr, "%s: 卵的 `state` 与主控 `GET /api/models/{name}` 的 `status` **逐字对不上** ⇒ 判红\n", progName)
		for _, p := range problems {
			fmt.Fprintf(stderr, "  · %s\n", p)
		}
		fmt.Fprintf(stderr, "（判据②：同一格不许两套词 —— 宁可判红，也不挑一个当答案）\n")
		return exitFail
	}
	return exitOK
}

// cmdEggLs —— `zerg egg ls`：卵 × 设备矩阵（**只读投影** · 波② `T4`），每枚卵三格 `host`/`model`/`state`。
func cmdEggLs(inv *invocation, stdout, stderr io.Writer) int {
	rows, problems, rc := projectEggs(newClient(), inv, stderr, "")
	if rc != exitOK {
		return rc
	}
	if len(rows) == 0 {
		inv.setErr("blocked", "egg_roster_empty", "名册展开后一枚卵都没有")
		fmt.Fprintf(stderr, "%s: 卵 × 设备矩阵**空**（名册展开后一枚卵都没有）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	if rc := eggRowsGuard(inv, rows, problems, stderr); rc != exitOK {
		return rc
	}
	return listCmd(inv, stdout, stderr, eggDisplay, rows)
}

// cmdEggShow —— `zerg egg show <卵 id>`：单枚卵的现状（**同一份投影真源**，只挑那一枚）。
//
// 卵 id 的形状 = `<模型 id>@<主机>`（照 `egg ls` 里 `egg_id` 那一格逐字给）。
// 只给模型 id 时：该模型**只有一台候选机**才代为定位；**多台 ⇒ 判用法错**并列出合法的卵 id
// （歧义不替人挑 ✗ —— 同一个模型在两台机上可以是状态不同的**两枚卵**）。
func cmdEggShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `egg show` 要一个卵 id（`zerg egg ls` 先看清单）\n", progName)
		fmt.Fprintf(stderr, "卵 id 的形状 = `<模型 id>%s<主机>`（例：Ternary-Bonsai-2-27B%sMr2109）\n", eggSep, eggSep)
		inv.setErr("usage", "missing_target", "缺卵 id")
		return exitUsage
	}
	model, host, hasHost := strings.Cut(inv.args[0], eggSep)
	if strings.TrimSpace(model) == "" {
		inv.setErr("usage", "bad_target", "卵 id 里没有模型 id")
		fmt.Fprintf(stderr, "%s: 卵 id 的模型 id 是空的（给的是 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	rows, problems, rc := projectEggs(newClient(), inv, stderr, model)
	if rc != exitOK {
		return rc
	}
	legal := []string{}
	for _, r := range rows {
		if r["model"] == model && r["host"] != "" {
			legal = append(legal, r["host"])
		}
	}
	sort.Strings(legal)
	if !hasHost || strings.TrimSpace(host) == "" {
		switch len(legal) {
		case 0:
			// 落到下面「名册里没有这枚卵」
		case 1:
			host = legal[0]
		default:
			inv.setErr("usage", "ambiguous_target", "该模型有多台候选机 ⇒ 卵 id 要带 @主机")
			fmt.Fprintf(stderr, "%s: 模型 %q 有 %d 台候选机 ⇒ 一枚卵要指明机（%s）\n",
				progName, model, len(legal), strings.Join(eggIDsOf(legal, model), " · "))
			fmt.Fprintf(stderr, "（同一模型在两台机上可以是**两枚状态不同的卵** ⇒ 歧义不替人挑）\n")
			return exitUsage
		}
	}
	var picked map[string]string
	for _, r := range rows {
		if r["model"] == model && r["host"] == host {
			picked = r
			break
		}
	}
	if picked == nil {
		ids := []string{}
		for _, r := range rows {
			ids = append(ids, r["egg_id"])
		}
		sort.Strings(ids)
		fmt.Fprintf(stderr, "%s: 名册里没有这枚卵：%q\n", progName, inv.args[0])
		if s := nearestName(inv.args[0], ids); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		inv.setErr("unsupported_on_node", "unknown_egg", "名册里没有这枚卵")
		return exitUsage
	}
	if rc := eggRowsGuard(inv, []map[string]string{picked}, problems, stderr); rc != exitOK {
		return rc
	}
	return listCmd(inv, stdout, stderr, eggDisplay, []map[string]string{picked})
}

// eggIDsOf 把一组主机名拼成合法的卵 id（`egg show` 的歧义面用：列出可选的卵 id）。
func eggIDsOf(hosts []string, model string) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, model+eggSep+h)
	}
	return out
}

// cmdEggRun —— `zerg egg run <卵 id>`：把卵跑起来（D2 写面 · **本波不做 ⇒ 押后** ✗）。
//
// 为什么本波不做（`T4` 逐字）：写面会在**真机上起算**（占内存、占端口 ⇒ 动生产）⇒ 波②只做
// **只读投影**（`egg ls` / `egg show`），写面**押后**。而且「先看代价、再决定动不动手」
// 正是本仓 `--dry-run` / 业界 `--estimate-only` 的同一个思想：投影面先立住，写面的**后置条件**
// （「它到底起没起来」）才第一次有判据。
func cmdEggRun(inv *invocation, stdout, stderr io.Writer) int {
	inv.setErr("blocked", "write_face_deferred", "卵的写面（起算）本波不做 —— 押后")
	fmt.Fprintf(stderr, "%s: `egg run` 是**写面**（会在真机上起算：占内存、占端口）⇒ 本波**不做**（押后）\n", progName)
	fmt.Fprintf(stderr, "波② 只开**只读投影**：`%s egg ls` / `%s egg show`（照现成端点包装 · 不新开一条路）\n", progName, progName)
	fmt.Fprintf(stderr, "⇒ 不给结论（退码 8 · kind=blocked · retryable=true）\n")
	return exitBlocked
}

// cmdCocoonLs —— `zerg cocoon ls`：虫茧清单（**主控面仍没有茧投影端点 ⇒ 不给结论**）。
func cmdCocoonLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if err := c.getJSON("/api/cocoons", &resp); err == nil {
		rows, display := flattenPayload(resp)
		return listCmd(inv, stdout, stderr, display, rows)
	}
	return eggsProjectionAbsent(inv, "虫茧清单", stderr)
}

// cmdCocoonOpen —— `zerg cocoon open <名>`：起 8610 服务 ⇒ **只登记形状，不执行**。
//
// 为什么不做成"直接跑一下"：起服务 = 起进程 = **动生产**（不可逆动作的邻居）。
// 判据（开工单 T-47 ③）要求的是"它能起 8610 服务"这件事**有落点**，不是"本版替你起一次"。
func cmdCocoonOpen(inv *invocation, stdout, stderr io.Writer) int {
	name := "(未给名)"
	if len(inv.args) > 0 {
		name = inv.args[0]
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s cocoon open\n", progName)
		fmt.Fprintf(stdout, "  危险档   : D3（起一个常驻服务 ⇒ 按不可逆动作办）\n")
		fmt.Fprintf(stdout, "  目标     : %s\n", name)
		fmt.Fprintf(stdout, "  它会动   : 起虫茧的文档服务（监听 **8610**）—— 这会占端口、进程常驻\n")
		fmt.Fprintf(stdout, "  执行要   : --confirm=<茧名> 与 --yes 同时到\n")
		fmt.Fprintf(stdout, "  本版状态 : **未开放** —— 起服务 = 动生产；要开请 Mr2109 拍（§6.3 S5）\n")
		fmt.Fprintf(stdout, "  来源     : §3.4 J 族 · §7.1 P6 · 开工单 T-47\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	inv.setErr("usage", "not_opened", "起服务属本版未开放档")
	fmt.Fprintf(stderr, "%s: `cocoon open` 要在目标机上**起一个常驻服务（8610）** ⇒ 本版**未开放**（不给结论 · 退码 2）\n", progName)
	fmt.Fprintf(stderr, "先看计划件：%s cocoon open %s --dry-run\n", progName, name)
	return exitUsage
}

// eggCocoonNames —— 两族的族名（`N4` 硬占位表的落点面）。
func eggCocoonNames() []string {
	names := []string{}
	for _, c := range catalog() {
		if len(c.path) == 2 && (c.path[0] == "egg" || c.path[0] == "cocoon") {
			names = append(names, strings.Join(c.path, " "))
		}
	}
	return names
}
