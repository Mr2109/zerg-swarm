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
//	   写面（`egg run` 装载 / `egg stop` 卸载）**已于 2026-09-26 开出**（★ 由 Mr2109 令开）——
//	   内部只打**控制层既有**那两条 POST（`/api/control/load|unload` · 见本件「卵面写档」那一段），
//	   仍然**不新开一条路** ✗；`--dry-run` 先行、真跑要 `--yes`（三态与 `model add` 同一张表）。
//	   ★ 本件旧文那句「写面（`egg run`）本波不做 ✗（押后 —— 起算 = 动生产）」**已作废**，逐字留在这里
//	     是让下一个人看得到它**变过**（不是它还在）。
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- 卵面：三格 + `state` 闭集（值域与主控逐字同源）------------------------------------------

// eggStateLoaded / eggStateIdle / eggStateDeclared / eggStateUnknown —— 卵的 `state` **闭集四个词**。
//
// ★ 2026-09-26（缺口 `GAP-20260926-08` · P0）：此前闭集**只有两个词**（`已加载` / `未加载`），而
// 它们的取值是**逐字取主控 `GET /api/models/{name}` 的 `status`** —— 那一格的来源链是：
//
//	子端心跳（`agent/internal/heartbeat/heartbeat.go`）的 `models[] = backend.RegistryNames()`
//	⇒ 主控心跳快照（`store.GetSnapshot(host)`）⇒ `ModelDetailHandler` 的 `status`
//
// **那不是「在跑的引擎」，那是子端注册表的名册**（这台机**认识**哪些模型）⇒ 主控对**每一个**
// 注册过的模型都报 `已加载`。实测：本机 `Mr2109` 注册表 7 条 ⇒ `egg ls` 报 7 枚「已加载」，
// 而同一刻 `ps` 上只有 **1** 个 `llama-server`（`example-35b`）、子端 `/eggs` 也只有 1 枚。
// 即：**拿「声明」当真值**。卸载之后引擎进程已经没了，`egg ls` 照旧报「已加载」✗。
//
// 修法（在命令面拿得到的数据内对账 —— 不新开一条路 ✗）：把主控那一格（**声明**）与**子端
// `/eggs` 的真后端回据**交叉比，逐枚卵落进四态之一：
//
//	已加载            子端 `/eggs` 里有这一枚卵且 `port` > 0（**真后端在跑** —— 声明与回据一致）
//	已声明但后端不在  主控说「已加载」，而子端 `/eggs` 里**没有它**（或 `port` = 0／单元已死）
//	未加载            主控没声「已加载」，子端回据也没有它（两处一致：没在跑）
//	取不到            子端打不到／鉴权不过／回据不是合法 JSON ⇒ **不给结论**（不编一个值）
//
// ★ 为什么 `取不到` 必须是**独立一格**、不许并进「未加载」：子端引擎监听 127.0.0.1（远端机
// 无法直连 ⇒ 只有在子端向才拿得到真后端信息）。读不到就**只是读不到** —— 把它写成「未加载」
// 是**编一个数**（§九 M3 铁律）；把它写成「已加载」正是本缺口本身。
//
// ★ `已加载` / `未加载` 两个词**逐字未动**（与主控 `status` 同源那一条判据仍在）：本笔只
// **加**两个词，不换那两个字 —— 同一格多出来的两态是「声明与回据对不上」与「回据取不到」，
// 它们是**新的语义**，不是旧词的同义改写。
const (
	eggStateLoaded   = "已加载"
	eggStateIdle     = "未加载"
	eggStateDeclared = "已声明但后端不在"
	eggStateUnknown  = "取不到"
)

// eggStateClosedSet —— 四词闭集（机检用一处取值 · 判词与测试都引它，不各抄一份）。
var eggStateClosedSet = []string{eggStateLoaded, eggStateIdle, eggStateDeclared, eggStateUnknown}

// eggDisplay —— 人面 / 行式面的**三格**（判据①：每行三格齐；缺一格即红）。
var eggDisplay = []string{"host", "model", "state"}

// eggFields —— `--json` 的字段表：三格在前，另带 `egg_id` / `backend` / `mem_gb` 三格便于机器面
// 定位（除 `egg_id` 是**拼**出来的，其余字段名**逐字取自它投影的载荷键** —— 不另造词汇）。
//
// ★ 2026-09-26（`GAP-20260926-08`）：`declared` 与 `backend_port` 两格是**对账的证据面** ——
// 机器面要能自己看出「这一格为什么是 `已声明但后端不在` / `取不到`」：
//
//	declared      主控那一格的原话（`GET /api/models/{name}` 的 `status`；非主控点名的机 ⇒
//	              心跳快照 `models[]` 里的成员关系折成的同一个词）—— 即**声明**
//	backend_port  子端 `/eggs` 报的 `port`（**逐字取自载荷键 `port`**）；没在跑 / 取不到 ⇒ 空
//
// 两格都是**纯追加**（旧字段一个没动、顺序没动）⇒ 闭集外沿只增不改。
var eggFields = []string{"egg_id", "host", "model", "state", "declared", "backend_port", "backend", "mem_gb"}

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
				// ★ `declared` = 主控那一格的**原话**（即「声明」）。`state` 初值 = 它，
				// 但**真正的 `state` 由 `eggReconcileBackends` 与子端真后端回据对账后落定**
				// （`GAP-20260926-08`：不许把声明当真值）。
				"declared":     state,
				"backend_port": "",
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

// ---- 卵面：与子端「真后端」对账（`GAP-20260926-08` 的判口 · 三态在这里落定）------------------
//
// 为什么落点在这里（而不是又去问主控）：主控面那份 `status` 的**来源就是子端的声明**
// （心跳的 `RegistryNames()`）—— 再问主控一次 = 把同一份声明再读一遍，对不出任何东西。
// 真后端的回据**只在子端向**（引擎监听 127.0.0.1 ⇒ 远端机无法直连。
// 引擎端口按 `agent/internal/server/eggs_endpoint.go` 的 `/eggs` 走。
//
// 对账**只用现成端点**（`GET /eggs` · 同一个 `X-Auth-Token` 面 · 不新开一条路 ✗）：
// `core/cmd/zerg/family_agent_ops.go` 的 `agentChildDo` / `agentFleetAddrs` 已是本仓既有的
// 「打子端」通路（`agent bench` 等已用它）—— 本笔**复用**，不另起一个 HTTP 客户端。

// eggBackendFact —— 一台机的子端对「真后端」的**回据**（只读快照）。
//
//	Reachable  子端 `/eggs` 回了 200 且解得动（**这一格 = false ⇒ 该机所有卵都是 `取不到`**）
//	Running    模型 id ⇒ 在跑的引擎端口（`/eggs` 里 `port` > 0 且单元没落 dead/missing 的那些）
//	Why        取不到时的一句原因（人面说白 · 不编）
type eggBackendFact struct {
	Reachable bool
	Running   map[string]int
	Why       string
}

// eggBackendProbe —— 子端真后端探测的**唯一出口**（测试可换成合成子端 ⇒ 判据件不连真网）。
// 生产路径 = `eggBackendProbeLive`；换口只在测试构建里做（`export_test.go` 的桥）。
var eggBackendProbe = eggBackendProbeLive

// ★ 为什么探测口收的是**机器名 + 地址**两件（而不是只收 `agentAddr`）：机器名是名册
// （`gateway/fleet.yaml` 的 `fleet:` 段）的**键**，而 `agentAddr.Host` 是**网络地址**
// （`Mr2109` ⇒ `127.0.0.1`）—— 两件不是一回事。对账按机器名归并（行面上那一格就是机器名），
// 探测按地址打；合成子端的替身也按机器名查表（判据件里写不出真地址）。

// eggBackendProbeLive 真探测：`GET /eggs`（只读）⇒ 真后端回据。
//
// 判「这一枚在跑」的口径（**保守**：宁可少认，不可多认）：
//
//	① 载荷里 `port` > 0（子端自报的引擎端口 —— 0 就是没在跑）；
//	② `managed` 没被明确写成 false（不是本端托管的卵不算）；
//	③ `state` 没落 `dead` / `missing`（`applyEggUnitLiveness` 落的那两个词 = 单元已死 ⇒
//	   「进程为零而 `state` 仍 ready」那条老缺口不许再从本格漏进「已加载」）。
func eggBackendProbeLive(machine string, a agentAddr) eggBackendFact {
	status, body, err := agentChildDo(a, http.MethodGet, "/eggs", "", eggBackendTimeout)
	if err != nil {
		return eggBackendFact{Why: fmt.Sprintf("打不到子端 %s/eggs（%v）", agentChildBase(a), err)}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return eggBackendFact{Why: fmt.Sprintf("子端 %s/eggs 回 HTTP %d（令牌不对/权限不够）", agentChildBase(a), status)}
	}
	if status != http.StatusOK {
		return eggBackendFact{Why: fmt.Sprintf("子端 %s/eggs 回 HTTP %d", agentChildBase(a), status)}
	}
	var doc jsonObj
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return eggBackendFact{Why: fmt.Sprintf("子端 %s/eggs 的响应不是合法 JSON（%v）", agentChildBase(a), err)}
	}
	running := map[string]int{}
	for _, it := range asList(doc["eggs"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		port, _ := strconv.Atoi(cell(o["port"]))
		if port <= 0 {
			continue
		}
		if v, ok := o["managed"]; ok && v != nil && cell(v) == "false" {
			continue
		}
		if s := cell(o["state"]); s == "dead" || s == "missing" {
			continue
		}
		id := cell(o["model"])
		if id == "" {
			id = cell(o["egg_id"])
		}
		if id != "" {
			running[id] = port
		}
	}
	return eggBackendFact{Reachable: true, Running: running}
}

// eggBackendTimeout 单机探测的超时（子端是**近端**服务；一条 `egg ls` 对每台机只问一次）。
const eggBackendTimeout = 8 * time.Second

// eggBackends —— 本轮要用的各机真后端回据：每台机**只问一次**（同一批卵共用一份回据 ——
// 免得一枚卵打一次子端：既慢，又让同一台机的两枚卵可能读到相隔几秒的两份快照）。
//
// 机器不在名册里（`gateway/fleet.yaml` 的 `fleet:` 段）⇒ `Reachable=false` + 说白的原因。
func eggBackends(hosts []string) map[string]eggBackendFact {
	out := map[string]eggBackendFact{}
	uniq := map[string]bool{}
	order := []string{}
	for _, h := range hosts {
		if h != "" && !uniq[h] {
			uniq[h] = true
			order = append(order, h)
		}
	}
	addrs, _, rc := agentFleetAddrs(io.Discard)
	if rc != exitOK {
		for _, h := range order {
			out[h] = eggBackendFact{Why: "名册（" + agentFleetAddrFileRel + " 的 `fleet:` 段）读不到 ⇒ 子端地址问不出来"}
		}
		return out
	}
	for _, h := range order {
		a, ok := addrs[h]
		if !ok {
			out[h] = eggBackendFact{Why: fmt.Sprintf("名册里没有 %q 这台机 ⇒ 子端地址问不出来", h)}
			continue
		}
		out[h] = eggBackendProbe(h, a)
	}
	return out
}

// eggReconcileBackends —— 把所有行与子端真后端回据对账，**落定 `state`** 那两格。
//
// 返回：`exitOK`（每一枚卵都有结论）或 `exitBlocked`（有卵读不到 ⇒ 前置拿不到 ⇒ 退 8）。
// 有卵读不到时：`state` 落 `取不到`（**照实说** · 绝不并进「未加载」）、`inv` 里留下
// `error.kind=blocked` + `detail=backend_unreadable`、stderr 逐台把原因说白。
func eggReconcileBackends(inv *invocation, rows []map[string]string, stderr io.Writer) int {
	hosts := make([]string, 0, len(rows))
	for _, r := range rows {
		hosts = append(hosts, r["host"])
	}
	facts := eggBackends(hosts)
	unreadable := map[string]bool{} // 读不到的机（去重 · 只在 stderr 说一次）
	for _, r := range rows {
		f := facts[r["host"]]
		declared := r["declared"]
		// ① 取不到：子端没回据 ⇒ **不给结论**（这一格是「读不到」，不是「没在跑」）。
		if !f.Reachable {
			r["state"] = eggStateUnknown
			r["backend_port"] = ""
			unreadable[r["host"]] = true
			continue
		}
		// ② 真后端在跑（子端 `/eggs` 里 `port` > 0）⇒ `已加载` —— **回据说了算**，与声明无关。
		//    为什么此时若声明说「未加载」也落 `已加载`：本格量的是**真后端在不在**（缺口原话
		//    「不再拿声明当真值」）—— 声明那一格原样留在 `declared` 里，机器面自己看得见两者不一致。
		if port, ok := f.Running[r["model"]]; ok {
			r["state"] = eggStateLoaded
			r["backend_port"] = strconv.Itoa(port)
			continue
		}
		// ③ 真后端不在：主控声「已加载」⇒ **已声明但后端不在**（本缺口的正题）；
		//    主控也没声 ⇒ `未加载`（两处一致）。
		r["backend_port"] = ""
		if declared == eggStateLoaded {
			r["state"] = eggStateDeclared
			continue
		}
		r["state"] = eggStateIdle
	}
	if len(unreadable) == 0 {
		return exitOK
	}
	names := make([]string, 0, len(unreadable))
	for h := range unreadable {
		names = append(names, h)
	}
	sort.Strings(names)
	inv.setErr("blocked", "backend_unreadable",
		"子端 "+strings.Join(names, " · ")+" 的真后端回据（`/eggs`）读不到")
	fmt.Fprintf(stderr, "%s: 子端 **%s** 的真后端回据（`GET /eggs`）读不到 ⇒ 这些机上的卵 `state` 落 `%s`\n",
		progName, strings.Join(names, " · "), eggStateUnknown)
	for _, h := range names {
		fmt.Fprintf(stderr, "  · %s：%s\n", h, orDash(facts[h].Why))
	}
	fmt.Fprintf(stderr, "  为什么这必须是独立一格（不许并进 `%s`）：子端引擎监听 127.0.0.1 ⇒ **只有在子端向**拿得到真后端；\n", eggStateIdle)
	fmt.Fprintf(stderr, "  读不到就只是读不到 —— 写成 `%s` 是**编一个数**（§九 M3 铁律）\n", eggStateIdle)
	fmt.Fprintf(stderr, "error.kind=blocked · detail=backend_unreadable · retryable=true · remedy=fix_precondition\n")
	return exitBlocked
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
	// ★ `GAP-20260926-08`：三格齐 / 值域两道守卫过了之后，才拿**子端真后端回据**落定 `state`。
	//   顺序不许反：取不到那一档要能**在打网络之前**被判掉的坏行（缺 host / 出闭集）先挡掉。
	//
	// ★ 为什么**照旧把矩阵打出来**（哪怕整张表都 `取不到`）：`取不到` 是**逐枚卵的结论**，它必须
	//   在机器面上看得见（判据②「三态不许塌成一格」）—— 把 stdout 清零等于**让机器看不到那一态**
	//   （机器面只会看到「退 8 + 空表」，与「名册空」那一档分不开）。所以：表格照出、`state` 逐格
	//   说白、退码 8 表示「有前置拿不到」。
	rcBackend := eggReconcileBackends(inv, rows, stderr)
	if rc := listCmd(inv, stdout, stderr, eggDisplay, rows); rc != exitOK {
		return rc
	}
	return rcBackend
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
	// ★ 同 `egg ls`：`state` 由**子端真后端回据**落定（单枚卵 ⇒ 取不到就是退 8 + stdout 0 字节）。
	if rc := eggReconcileBackends(inv, []map[string]string{picked}, stderr); rc != exitOK {
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

// ---- 卵面写档（`egg run` 装载 / `egg stop` 卸载）：两族里**唯一会动生产**的两条 ---------------
//
// ★ 2026-09-26：写面**开出来**（本件此前那条「写面本波不做 ⇒ 押后」的判词**已作废**，逐字记在这里
// 免得下一个人以为它还在）。此前装载 / 卸载**只能手搓控制 API**：
//
//	POST http://127.0.0.1:8580/api/control/load    体 {"machine":"<机>","model":"<模型 id>"}
//	POST http://127.0.0.1:8580/api/control/unload  体 {"machine":"<机>"}
//
// 鉴权走请求头 `X-Auth-Token`（令牌**绝不进 argv** —— 读法照 `client.go`：`resolveCredentials()`
// 一处取令牌，除了 header 之外哪儿都不落）。端点是控制层**既有**路由（`core/cmd/zerg-core/main.go:196-197`）
// ⇒ 命令面**不新开一条路**，只把「既有端点」包成命令（同 `egg ls` / `egg show` 的只读包装口径）。
//
// 三条口径（与族内其它写面逐条同形）：
//
//	① **一枚卵 = 模型 id × 主机这一对**（`egg ls` 的 `egg_id` 那一格）：`egg run` 装的就是这一对 ——
//	   模型从卵 id 取，机器从 `--machine` 取；`--machine` 缺省时用卵档案（名册里该条）的 `host`。
//	② **三态**（`repo commit` / `doc meta fill` / `model add` / `route pin` 同一条口径）：
//	   `--dry-run` ⇒ 计划件走 stdout、零副作用（**一个请求都不发**）· 缺 `--yes` ⇒ 计划件走 stderr、
//	   退 `2`（stdout **0 字节**）· `--yes` 才真发那一条 POST。**从不提问、从不交互**。
//	   ★ 为什么**不给**危险档（`dangerSpec`）：本次纪律逐字要「`--json` 不给字段 ⇒ stdout 0 字节」，
//	   而危险档的 K2 失败面走**错误包封**（stdout 非 0 字节 · `script inventory sync` 那一档的先例）
//	   ⇒ 与纪律冲突。同族的 D2 写面先例 `model add`（`--dry-run` 先行 + 缺 `--yes` ⇒ 2 + 0 字节）
//	   走的正是**非危险档 + 自判三态**这一档。
//	③ **目标核对在册**（写之前，读名册一次 · 只读）：模型不在册 / 机器不在该模型的候选里
//	   ⇒ **不是错、是没有** —— 退 `2` + `detail` 点名（`no_such_model` / `no_such_machine` /
//	   `model_not_deployed`）+ **列可选值**（照 `family_fusion.go` 的 `cmdAsk` 那一套：
//	   `fleetHasModel` / `fleetHasHost` + `no_such_model`）；名册**读不到**或**为空** ⇒ 退 `8`
//	   （不给结论 —— 「读不到」不许当「没有」）。
//
// 退码（一律引现有表 `exitcodes.go` · 本档**不取新号**）：
//
//	0 装/卸成功（`changed=true`）· 2 用法错（缺卵 id / 多余位置参数 / 未知名 / `--json` 字段面 ·
//	缺 `--yes` · 目标不在册）· 8 不给结论（名册读不到或为空）· 4/12/1 主控面（未认证 / 打不到 / 上游错）。

// eggActFields —— `egg run` / `egg stop` 两档**共用**的 `--json` 字段表（闭集 · 顺序即人面列序）。
// `action` 取 `load` / `unload`（与它投影的控制端点逐字同源：`/api/control/load|unload`）。
var eggActFields = []string{"egg_id", "model", "machine", "action", "state", "result"}

// eggActDisplay —— 人面那几格（`result` 那格是主控原样响应，太长 ⇒ 只进机器面）。
var eggActDisplay = []string{"egg_id", "model", "machine", "action", "state"}

const (
	eggActLoad   = "load"
	eggActUnload = "unload"
)

// eggActEndpoint —— 动作 → 控制层端点（**逐字**取自控制层路由表，不另造路径）。
func eggActEndpoint(action string) string {
	if action == eggActUnload {
		return "/api/control/unload"
	}
	return "/api/control/load"
}

// eggActVerb —— 动作 → 命令动词（帮助面 / 报头共用一处）。
func eggActVerb(action string) string {
	if action == eggActUnload {
		return "stop"
	}
	return "run"
}

// eggCand —— 名册里的一条候选（模型在**某台机**上的一条部署）。
type eggCand struct{ host, backend, mem string }

// eggRosterOf —— `/api/fleet/models` 载荷 → （模型 id → 候选 · 模型名册序 · 机器名册序）。
// 只读一次、两处共用（目标核对与计划件），不各算一份。
func eggRosterOf(fleet jsonObj) (map[string][]eggCand, []string, []string) {
	byModel := map[string][]eggCand{}
	models, hosts := []string{}, []string{}
	seenModel, seenHost := map[string]bool{}, map[string]bool{}
	for _, it := range asList(fleet["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		id := cell(o["id"])
		if id == "" {
			continue
		}
		if !seenModel[id] {
			seenModel[id] = true
			models = append(models, id)
		}
		h := cell(o["host"])
		byModel[id] = append(byModel[id], eggCand{host: h, backend: cell(o["backend"]), mem: cell(o["mem_gb"])})
		if h != "" && !seenHost[h] {
			seenHost[h] = true
			hosts = append(hosts, h)
		}
	}
	sort.Strings(models)
	sort.Strings(hosts)
	return byModel, models, hosts
}

// eggPlanLines —— 计划件的**共用**几行（干跑与缺 `--yes` 两态印同一份 ⇒ 两档判据不许分叉）。
// `cands` 为空时（授权闸在**读名册之前**判）只印得出目标两行 —— 照实少印，不编。
func eggPlanLines(action, eggID, model, machine string, cands []eggCand) []string {
	verb := "装载"
	if action == eggActUnload {
		verb = "卸载"
	}
	lines := []string{
		fmt.Sprintf("卵       : %s（模型 %s × 机器 %s）", eggID, model, orDash(machine)),
		fmt.Sprintf("动作     : %s（%s 到这台机 / 从这台机卸回来）", verb, model),
	}
	if len(cands) > 0 {
		hosts := []string{}
		for _, c := range cands {
			hosts = append(hosts, c.host+fmt.Sprintf("（backend %s · mem_gb %s）", orDash(c.backend), orDash(c.mem)))
		}
		lines = append(lines, fmt.Sprintf("该模型候选: %s", strings.Join(hosts, " · ")))
	}
	if action == eggActUnload {
		lines = append(lines,
			fmt.Sprintf("请求体   : {\"machine\":%s}（控制面那条端点**只吃 machine** —— 逐字照它的体形状）", jstr(machine)))
	} else {
		lines = append(lines,
			fmt.Sprintf("请求体   : {\"machine\":%s,\"model\":%s}", jstr(machine), jstr(model)))
	}
	return lines
}

// eggWriteGate —— 写档三态的唯一判口（照 `routeWriteGate` 的同一张表 · 措辞按卵面）。
//
// `--json` 面：人读的计划件走 **stderr**（机器面 stdout 只许剩包封 —— `A3-b` 案的先例）。
func eggWriteGate(inv *invocation, stdout, stderr io.Writer, act, eggID, machine, endpoint, effect string, plan []string) (int, bool) {
	planW := stdout
	if inv.jsonGiven {
		planW = stderr
	}
	if inv.dryRun {
		fmt.Fprintf(planW, "计划件（--dry-run · 零副作用 —— 未执行、未打主控写面）\n")
		fmt.Fprintf(planW, "  动作     : %s %s\n", progName, act)
		fmt.Fprintf(planW, "  命令     : %s %s %s\n", progName, act, eggID)
		fmt.Fprintf(planW, "  请求     : POST %s%s\n", newClient().base, endpoint)
		for _, ln := range plan {
			fmt.Fprintf(planW, "  %s\n", ln)
		}
		fmt.Fprintf(planW, "  它会动   : %s\n", effect)
		// `Q-021`（波① `T3`）**两档同一张授权表**：干跑要把真跑要什么写明（否则两档判据分叉）。
		fmt.Fprintf(planW, "  授权判据 : 真跑要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n")
		fmt.Fprintf(planW, "  来源     : §3.4 I 族 · §7.1 P6 · 控制面 POST %s\n", endpoint)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n")
		return exitOK, false
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `%s %s` 是 D2 档（**会在真机上装/卸**）—— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n",
			progName, progName, act)
		fmt.Fprintf(stderr, "  命令     : %s %s %s\n", progName, act, eggID)
		fmt.Fprintf(stderr, "  请求     : POST %s%s\n", newClient().base, endpoint)
		for _, ln := range plan {
			fmt.Fprintf(stderr, "  %s\n", ln)
		}
		fmt.Fprintf(stderr, "  它会动   : %s\n", effect)
		fmt.Fprintf(stderr, "加 --yes 才执行；先看计划件：%s %s %s --dry-run\n", progName, act, eggID)
		return exitUsage, false
	}
	return exitOK, true
}

// cmdEggRun —— `zerg egg run <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]`：
// 把该卵的模型**装到**那台机器上（内部 = 控制面 `POST /api/control/load`）。
func cmdEggRun(inv *invocation, stdout, stderr io.Writer) int {
	return eggAct(inv, stdout, stderr, eggActLoad)
}

// cmdEggStop —— `zerg egg stop <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]`：
// 卸载 —— 把该卵的模型从那台机器上**卸掉**（内部 = 控制面 `POST /api/control/unload`）。
func cmdEggStop(inv *invocation, stdout, stderr io.Writer) int {
	return eggAct(inv, stdout, stderr, eggActUnload)
}

// eggAct —— 两档**共用**的实现（装载与卸载的差别只在端点、请求体、几个词 —— 不各写一份）。
func eggAct(inv *invocation, stdout, stderr io.Writer, action string) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	verb := eggActVerb(action)
	act := "egg " + verb
	endpoint := eggActEndpoint(action)
	effect := "把该模型的引擎装到那台机上（**占内存/槽位** · 单槽机是串行的）—— 反动作 = `" + progName + " egg stop`"
	if action == eggActUnload {
		effect = "把那台机上的该模型卸掉（**在跑的任务会被打断**）—— 反动作 = `" + progName + " egg run`"
	}

	// ① 用法面（**执行前判** · 零网络 · 零副作用）：`--json` 字段面先判。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		legal := map[string]bool{}
		for _, f := range eggActFields {
			legal[f] = true
		}
		for _, f := range inv.fields {
			if !legal[f] {
				return reportBadField(stderr, inv.path, f)
			}
		}
	}
	// ② 位置参数：卵 id **一枚**（危险档不判「多余位置参数」⇒ 本档自己判 —— 不许静默吞掉多给的那枚）。
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺卵 id")
		fmt.Fprintf(stderr, "%s: `%s` 要一个卵 id（先 `%s egg ls` 看清单）\n", progName, act, progName)
		fmt.Fprintf(stderr, "卵 id 的形状 = `<模型 id>%s<主机>`（例：Ternary-Bonsai-2-27B%sMr2109）；\n", eggSep, eggSep)
		fmt.Fprintf(stderr, "`--machine <机>` 可覆盖卵档案里那台机（缺省 = 用名册里该条的 `host`）\n")
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一枚卵 id）")
		fmt.Fprintf(stderr, "%s: `%s` 只吃**一枚**卵 id（多给的是 %q）—— 一台机一次\n",
			progName, act, inv.args[1])
		fmt.Fprintf(stderr, "机器走 `--machine <机>`，不许当第二个位置参数（§4.1 K14 四件套）\n")
		return exitUsage
	}
	eggArg := strings.TrimSpace(inv.args[0])
	model, host, hasHost := strings.Cut(eggArg, eggSep)
	if strings.TrimSpace(model) == "" {
		inv.setErr("usage", "bad_target", "卵 id 里没有模型 id")
		fmt.Fprintf(stderr, "%s: 卵 id 的模型 id 是空的（给的是 %q）\n", progName, eggArg)
		return exitUsage
	}
	machine := strings.TrimSpace(inv.flagVal("--machine"))
	if machine == "" && hasHost {
		machine = strings.TrimSpace(host)
	}

	// ③ 授权闸（**先于任何网络** —— 缺 `--yes` 的判词不许靠「主控打得到了」才给得出）。
	// 干跑**不走这一支**：它的计划件要等目标核对过之后才印（这样计划件里带得出候选那几行）。
	// 这一步的计划件只印得出目标两行（名册还没读）⇒ 照实少印，不编候选。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if !inv.dryRun {
		if rc, goOn := eggWriteGate(inv, stdout, stderr, act, eggArg, machine, endpoint, effect,
			eggPlanLines(action, eggArg, model, machine, nil)); !goOn {
			return rc
		}
	}

	// ④ 名册（只读一次）：目标核对在册 —— 「不在册」是「没有」，不是「错」（退 2 + 列可选值）；
	//    「读不到 / 空名册」是「不给结论」（退 8）。
	c := newClient()
	var fleet jsonObj
	if err := c.getJSON("/api/fleet/models", &fleet); err != nil {
		return eggSourceFail(inv, "卵名册 `/api/fleet/models`", err, stderr)
	}
	byModel, models, hosts := eggRosterOf(fleet)
	if len(models) == 0 {
		inv.setErr("blocked", "egg_roster_empty", "名册展开后一枚卵都没有")
		fmt.Fprintf(stderr, "%s: 名册读得到但**一枚卵都没有**（`/api/fleet/models` 的 models 为空）\n", progName)
		fmt.Fprintf(stderr, "⇒ 目标核不了、也不给结论（退码 8 · kind=blocked）—— 不猜「大概能装」\n")
		return exitBlocked
	}
	if _, ok := byModel[model]; !ok {
		inv.setErr("usage", "no_such_model", fmt.Sprintf("没有叫 %q 的模型（名册里查不到这个 id）", model))
		fmt.Fprintf(stderr, "%s: %s 里没有模型 %q ⇒ 退码 2（**不是错、是没有** —— 不静默退回别的模型）\n",
			progName, act, model)
		if s := nearestName(model, models); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		fmt.Fprintf(stderr, "可选模型（名册）：%s\n", strings.Join(models, " · "))
		return exitUsage
	}
	cands := byModel[model]
	candHosts := []string{}
	for _, cd := range cands {
		if cd.host != "" {
			candHosts = append(candHosts, cd.host)
		}
	}
	sort.Strings(candHosts)
	switch {
	case machine == "" && len(candHosts) > 1:
		inv.setErr("usage", "ambiguous_target", "该模型有多台候选机 ⇒ 卵 id 要带 @主机（或用 --machine）")
		fmt.Fprintf(stderr, "%s: 模型 %q 有 %d 台候选机 ⇒ 一枚卵要指明机：%s\n",
			progName, model, len(candHosts), strings.Join(eggIDsOf(candHosts, model), " · "))
		fmt.Fprintf(stderr, "（同一模型在两台机上可以是**两枚状态不同的卵** ⇒ 歧义不替人挑）\n")
		return exitUsage
	case machine == "":
		machine = candHosts[0]
	case !contains(candHosts, machine):
		// 「机器不在列」：先分清是**这台机名册里根本没有**，还是**有这台机但这个模型没部署到它上面**。
		if !contains(hosts, machine) {
			inv.setErr("usage", "no_such_machine", fmt.Sprintf("名册里没有 %q 这台机", machine))
			fmt.Fprintf(stderr, "%s: %s 名册里没有这台机 %q ⇒ 退码 2（**不是错、是没有**）\n", progName, act, machine)
			if s := nearestName(machine, hosts); s != "" {
				fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
			}
		} else {
			// 这台机在册、但该模型没部署到它上面 ⇒ 换机名（上面那条）才是有用的下一步，别给「最像的机名」。
			inv.setErr("usage", "model_not_deployed",
				fmt.Sprintf("模型 %q 在机器 %q 上没有部署（控制面会回 MODEL_NOT_DEPLOYED）", model, machine))
			fmt.Fprintf(stderr, "%s: 模型 %q 在 %q 上**没有部署** ⇒ 退码 2（**不是错、是没有**；控制面那条端点也回 `MODEL_NOT_DEPLOYED`）\n",
				progName, model, machine)
		}
		fmt.Fprintf(stderr, "该模型的候选机: %s\n", orDash(strings.Join(candHosts, " · ")))
		fmt.Fprintf(stderr, "名册里的机器（fleet）：%s\n", orDash(strings.Join(hosts, " · ")))
		return exitUsage
	}
	eggID := model + eggSep + machine
	plan := eggPlanLines(action, eggID, model, machine, cands)

	// ⑤ 干跑：计划件已在校验之后 ⇒ 把**核对过的**目标与候选一并印出来（零请求）。
	// `--json` 面：人读计划件走 stderr、机器面 stdout 只出包封（不混）。
	if inv.dryRun {
		eggWriteGate(inv, stdout, stderr, act, eggID, machine, endpoint, effect, plan)
		changed := false
		inv.changed = &changed
		if !inv.jsonGiven {
			// 人面：计划件本身就是结果面（不再多印一张单行表）。
			return exitOK
		}
		row := map[string]string{
			"egg_id": eggID, "model": model, "machine": machine,
			"action": action, "state": "", "result": "",
		}
		return listCmd(inv, stdout, stderr, eggActDisplay, []map[string]string{row})
	}

	// ⑥ 真跑：一条 POST（**唯一**写面出口 —— `postJSON` **不新开路径**）。
	body := fmt.Sprintf("{\"machine\":%s,\"model\":%s}", jstr(machine), jstr(model))
	if action == eggActUnload {
		body = fmt.Sprintf("{\"machine\":%s}", jstr(machine))
	}
	status, respBody, err := postJSON(c, endpoint, body)
	if err != nil {
		kind, detail := "unreachable", "dial_failed"
		var he *httpError
		if errors.As(err, &he) {
			// 主控回了非 2xx：码**逐字透传**（命令面不翻译）。
			kind, detail = "upstream_error", fmt.Sprintf("http_%d", status)
			if code := errorCodeOf(respBody); code != "" {
				detail = code
				switch code {
				case "MODEL_NOT_FOUND", "MODEL_NOT_DEPLOYED", "MACHINE_NOT_FOUND":
					// 「不是错、是没有」那一族（前置不在 ⇒ 用法错 2，不是 1）。
					kind = "usage"
				}
			}
		}
		inv.setErr(kind, detail, fmt.Sprintf("控制面 %s 拒了这次请求", endpoint))
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ %v\n", progName, newClient().base, endpoint, err)
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s\n",
			kind, orDash(detail), retryableOf(kind), remedyOf(kind))
		fmt.Fprintf(stderr, "（码**逐字透传、命令面不翻译** —— 上面那串是主控自己的说法）\n")
		return codeOfKind(kind)
	}
	changed := true
	inv.changed = &changed
	// 后置读数：**只取主控自己的原话**，取不到就不给值（不编、也不装读到过）。
	state := eggStateAfter(inv, c, model, machine, stderr)
	row := map[string]string{
		"egg_id": eggID, "model": model, "machine": machine,
		"action": action, "state": state, "result": strings.TrimSpace(respBody),
	}
	fmt.Fprintf(stderr, "%s: %s 已发（%s ⇒ %s · 机器 %s）—— 复核：`%s egg ls` 或 `%s agent show %s`\n",
		progName, act, endpoint, orDash(state), machine, progName, progName, machine)
	return listCmd(inv, stdout, stderr, eggActDisplay, []map[string]string{row})
}

// eggStateAfter —— 写档的**后置读数**：`GET /api/models/{name}` 的 `status`（主控原话），
// 且**只在主控点名的那台机 == 本次目标机**时才给值。
//
// 为什么不一味从 `/api/fleet/status` 的快照推：心跳有**暖机窗口**，装完立刻读快照可能差一拍 ——
// 那一拍是**时序**不是**语义**（`projectEggs` 的上一条教训逐字讲过）⇒ 这里既不判红、也不装作
// 读到了：取不到就把 `state` 留空 + 一句 `warnings[]`，复核交给 `zerg egg ls`。
func eggStateAfter(inv *invocation, c *client, model, machine string, stderr io.Writer) string {
	var detail jsonObj
	if err := c.getJSON("/api/models/"+model, &detail); err != nil {
		inv.warnf("装/卸之后回读 `GET /api/models/%s` 读不到（%v）⇒ `state` 不给结论（留空，不编一个）", model, err)
		fmt.Fprintf(stderr, "%s: 后置读数读不到（`GET /api/models/%s`：%v）⇒ `state` 留空（**不编**）· 复核：`%s egg ls`\n",
			progName, model, err, progName)
		return ""
	}
	if h := cell(detail["host"]); h != "" && h != machine {
		inv.warnf("主控 `GET /api/models/%s` 点名的机是 %q（不是本次目标 %q）⇒ `state` 不给结论", model, h, machine)
		fmt.Fprintf(stderr, "%s: 主控点名的机是 %q（不是 %q）⇒ `state` 留空（**同一格不许两套词**）· 复核：`%s egg ls`\n",
			progName, h, machine, progName)
		return ""
	}
	return cell(detail["status"])
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
