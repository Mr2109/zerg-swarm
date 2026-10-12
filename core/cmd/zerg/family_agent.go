// family_agent.go —— `agent` 族（§三 B 族 · §十二 `P-038` 的族名闭合 · §九 M13 远端语义）。
//
// 为什么单独一件（§十二 `P-038` 逐字：设计稿内部**自相矛盾**，必须先闭合）：
// 稿里同时出现「`zerg agent …`」与「`zerg node <动作>`」两种族名（后者点名 reap / doctor 两条）。
// 本件按 §十二 `P-038` 的推荐口径落判：**族名只有一个 = `agent`**；
// `--node` 是**对象级旗标**（本次目标选谁，§九 M13 的「对象级作用域」），**不是族名**。
// ⇒ 全仓不许再出现 `zerg node <具体动作>` 这类写法（门⑪ 的 R7 盯着这条；写规矩时用
// `zerg node <动作>` 这种**占位**写法，别把被禁的具体口令抄进文档 —— 抄了 R7 就红，是设计如此）。
//
// 只读纪律：本件里除 `bootstrap`（危险档 · 会真的去装/起子端）之外全是**只读**命令；
// 「不许绕过主控」（§九 M20 `O3`）也是本族的硬规矩 —— 直达面只留 `agent ping --direct`
// 这一条**显式**例外（§十五.4 `F-2`）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- 族名闭合（P-038）----

// agentFamilyAliases —— `agent` 族的**唯一名字**（命令树里第 2 段是它的那些）。
// 判据（开工单 T-43 ①）：命令树里**没有** `node` 族；全仓 `zerg node <动作>` 写法归零。
func agentFamilyName() string { return "agent" }

// ---- 只读：单机投影（同一份载荷，不另打一枪）----

// fleetMachine 取一台机的条目：**同一份** `/api/fleet/status` 载荷（与 `agent ls` 同源）。
// 找不到 ⇒ 退码 2 + 名册 + 最相似名（§九 M13 `Z3`）。
func fleetMachine(inv *invocation, machine string, stderr io.Writer) (jsonObj, int) {
	c := newClient()
	var fleet jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/status", &fleet, stderr); rc != exitOK {
		return nil, rc
	}
	ms := asObj(fleet["machines"])
	o := asObj(ms[machine])
	if o == nil {
		names := make([]string, 0, len(ms))
		for n := range ms {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(stderr, "%s: 名册里没有这台机 %q（§九 M13 Z3）\n", progName, machine)
		fmt.Fprintf(stderr, "名册: %s\n", strings.Join(names, " · "))
		if s := nearestName(machine, names); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		inv.setErr("unsupported_on_node", "unknown_machine", "名册里没有这台机")
		return nil, exitUsage
	}
	return o, exitOK
}

// agentMarkUnknownGPU —— 把「平台线不可得」的 GPU 两格如实示 `n/a`（缺口 GAP-20260925-34 剩面①/②）。
//
// ★ 单一真源：`agent ls` 与 `agent show` 投影的是**同一份** `/api/fleet/status` 载荷 ⇒ 两处
//
//	必须同一口径（✗ 不许一套示 `n/a`、另一套把「未知」印成假 0 或留空串）。
//	· gpu_pct：macOS 机在主控投影层被剔除该键（ioreg 无可用占用读数）⇒ 载荷**缺键** ⇒ 示 `n/a`。
//	· gpu_used_gb：macOS 显存不可知（载荷 `vram_known=false`，源出心跳层「拿不到即 false」）⇒
//	  载荷里的 0 是「未知」哨兵、**不是真值** ⇒ 示 `n/a`。
//	· 其余平台（如 <机> ubuntu：`vram_known=true` 且有 `gpu_pct` 键）**逐字不动** —— 见有值不碰，
//	  只有「缺席／哨兵」才改写。口径沿用同族现成写法 `n/a`（family_net.go：「`n/a` 是一条答案，
//	  不是取不到」）—— ✗ 不自造词。
func agentMarkUnknownGPU(o jsonObj, row map[string]string) {
	if _, ok := o["gpu_pct"]; !ok {
		row["gpu_pct"] = "n/a"
	}
	if known, _ := o["vram_known"].(bool); !known {
		row["gpu_used_gb"] = "n/a"
	}
}

// cmdAgentShow —— `zerg agent show <机>`：一台机的逐字段现状（投影 `/api/fleet/status`）。
func cmdAgentShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent show` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := inv.args[0]
	o, rc := fleetMachine(inv, machine, stderr)
	if rc != exitOK {
		return rc
	}
	fields := []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin", "cpu_pct", "gpu_pct",
		"mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen", "recent_requests"}
	row := project(o, fields)
	row["machine"] = machine
	// GPU 两格的「不可得示 n/a」= 单一真源助手（`agent ls` / `agent show` 同一口径）——
	// ✗ 不把 macOS 缺席的 `gpu_pct` 印成空串、✗ 不把 `vram_known=false` 的哨兵 0 当显存真值
	// （GAP-20260925-34 剩面①②）。
	agentMarkUnknownGPU(o, row)
	// 「命令有什么能力」不另造词汇：字段名逐字取自载荷（§4.3 U2）。
	return listCmd(inv, stdout, stderr, []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin",
		"cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen", "recent_requests"},
		[]map[string]string{row})
}

// cmdAgentModels —— `zerg agent models <机>`：该机上可用的模型（投影 `/api/fleet/models` 里 host 命中的那些）。
func cmdAgentModels(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent models` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	// ★ 名册闸（缺口 GAP-20261005-11）：机器名不在名册里 ⇒ 不是错、是没有 ——
	// 退 2 ＋ 逐字复用同族现成报话（agentChildAddr；与 agent engines/slots/reload/registry/bench 同一句串），
	// ✗ 不自创文案。此闸把「没有这台机」（退 2）与「这台机存在但 0 个模型」（仍走下面 ⇒ 空表退 0）分开：
	// 后者是空结论，不许并成一格。
	if _, rc := agentChildAddr(machine, stderr); rc != exitOK {
		return rc
	}
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &resp, stderr); rc != exitOK {
		return rc
	}
	// ★ 对账证据面（照 `family_eggs_cocoons.go:57-63` 已有的四态对账形态办 —— **不新开一条路**）：
	// 主控名册（`/api/fleet/models` = **声明**）⟷ **子端注册表回据**（各机心跳快照 `models[]`，
	// 来源 = 子端 `backend.Manager.RegistryNames()`）。逐条落进四态之一：
	//
	//	declared=名册有 + subend=子端有   两处一致：名册与子端都有
	//	declared=名册有 + subend=子端无   **名册有、子端无**（本账 `GAP-20260926-07` 的症状）
	//	declared=名册无 + subend=子端有   子端有、名册无
	//	subend=取不到                     快照里没有这台机／回据不是合法列表 ⇒ **不给结论**
	//
	// ★ `取不到` 必须是独立一格、不许并进「子端无」：读不到只是读不到，把它写成「子端无」
	// 是**编一个数**（§九 M3），把它写成「子端有」正是本缺口本身。
	const (
		agentModelsDeclaredYes = "名册有"
		agentModelsDeclaredNo  = "名册无"
		agentModelsSubendYes   = "子端有"
		agentModelsSubendNo    = "子端无"
		agentModelsUnknown     = "取不到"
	)
	subend := map[string]bool{}
	subendKnown := false
	var snap jsonObj
	if err := c.getJSON("/api/fleet/status", &snap); err == nil {
		if so := asObj(asObj(snap["machines"])[machine]); so != nil && asList(so["models"]) != nil {
			subendKnown = true
			for _, it := range asList(so["models"]) {
				subend[cell(it)] = true
			}
		}
	}
	declaredMissing := 0
	fields := []string{"id", "host", "backend", "modality", "mem_gb", "file"}
	rows := []map[string]string{}
	inRoster := map[string]bool{}
	for _, it := range asList(resp["models"]) {
		o := asObj(it)
		if o == nil || cell(o["host"]) != machine {
			continue
		}
		row := project(o, fields)
		inRoster[cell(o["id"])] = true
		row["declared"] = agentModelsDeclaredYes
		switch {
		case !subendKnown:
			row["subend"] = agentModelsUnknown
		case subend[cell(o["id"])]:
			row["subend"] = agentModelsSubendYes
		default:
			row["subend"] = agentModelsSubendNo
			declaredMissing++
		}
		rows = append(rows, row)
	}
	if subendKnown {
		// 反向那一半（子端有、名册无）也要显形，否则「对账」只做了一半。
		extra := make([]string, 0, len(subend))
		for n := range subend {
			if !inRoster[n] {
				extra = append(extra, n)
			}
		}
		sort.Strings(extra)
		for _, n := range extra {
			rows = append(rows, map[string]string{"id": n, "host": machine, "backend": "", "modality": "",
				"mem_gb": "", "declared": agentModelsDeclaredNo, "subend": agentModelsSubendYes})
		}
		fmt.Fprintf(stderr, "%s: 对账（名册 ⟷ 子端注册表回据）—— 名册 %d 条 · 子端 %d 条 · 名册有子端无 %d 条 · 子端有名册无 %d 条\n",
			progName, len(inRoster), len(subend), declaredMissing, len(extra))
	}
	return listCmd(inv, stdout, stderr, []string{"id", "host", "backend", "modality", "mem_gb", "declared", "subend"}, rows)
}

// cmdAgentLogs —— `zerg agent logs <机>`：该机的日志（走**主控查询面** `/api/fleet/logs/query?machine=`）。
//
// 口径（缺口 `GAP-20260927-202` ① · 段2）：日志查询面 = 主控 `GET /api/fleet/logs/query`（参数逐字照
// `gateway/protocol.md:74-78`）；子端按段2 的批量上报把本机日志喂进它 ⇒ 本命令**只打主控**、
// 字段名逐字取自回据（§4.3 `U2`）。✗ 不读 `/tmp` 文件当结果、✗ 不偷偷直连子端（§九 M20 `O2`）。
// 发现面取**①**：照 `/api/logs/*` 那类**主控内部读面**先例（`core/internal/api/routes.go:32` 边界 ——
// 内部面不入能力表），直接读该已知面，✗ 不登进 `capabilities`。
//
// 未知 machine ⇒ 查询面按 machine 过滤后回**空数组** ⇒ **空结论**退 0（不是错、不崩）。
func cmdAgentLogs(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent logs` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent logs` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	c := newClient()
	var payload []jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/logs/query?machine="+url.QueryEscape(machine), &payload, stderr); rc != exitOK {
		return rc
	}
	rows := make([]map[string]string, 0, len(payload))
	for _, o := range payload {
		rows = append(rows, project(o, agentLogsFields))
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 主控日志面里 %q **没有**符合条件的日志 —— 这是**空结论**（退码 0），不是取不到\n", progName, machine)
	} else {
		fmt.Fprintf(stderr, "%s: agent logs %s · 主控查询面 /api/fleet/logs/query 命中 %d 条\n", progName, machine, len(rows))
	}
	return listCmd(inv, stdout, stderr, agentLogsDisplay, rows)
}

// agentLogsFields —— `agent logs` 的字段闭集：逐字取自查询面回据（`protocol.md:82` 的列）。
var agentLogsFields = []string{"ts", "machine", "level", "component", "model", "request_id", "message"}

// agentLogsDisplay —— 人面列（机器面用 agentLogsFields 全集）。
var agentLogsDisplay = []string{"ts", "level", "machine", "component", "model", "request_id", "message"}

// cmdAgentProbe —— `zerg agent probe <机>`：探活诊断（§十五.4 `F-2` 例外的**显式**形态）。
//
// 与 `agent ping` 的分工（不许两套语义）：`ping` 判「活不活」；`probe` 出**诊断面**
// （经主控能读到什么 · 直达面为什么默认关 · 要直达得给 `--direct`）。默认档同样**经主控**。
func cmdAgentProbe(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent probe` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := inv.args[0]
	o, rc := fleetMachine(inv, machine, stderr)
	if rc != exitOK {
		return rc
	}
	via := "core"
	if inv.direct {
		via = "direct（F-2 例外 · 输出必带回显；直达地址用 `--direct <host:port>`）"
	}
	fmt.Fprintf(stderr, "%s: via=%s（默认**经主控** —— 一份凭据、一处审计 · §九 M20 O3）\n", progName, via)
	row := project(o, []string{"machine", "healthy", "code_version", "code_sha", "last_seen"})
	row["machine"] = machine
	row["via"] = via
	row["direct_gate"] = "F-2 探活/诊断例外（要直达给 `--direct`；不许默认绕过主控）"
	return listCmd(inv, stdout, stderr,
		[]string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via", "direct_gate"},
		[]map[string]string{row})
}

// ---- ⑥ `zerg agent engines <机>`（缺口 `GAP-20260927-04` · F 族 · 只读）----
//
// 病根（缺口逐字）：`zerg agent bench` 对同机多枚引擎要求 `--model` 点名（不点名 ⇒ 退 2），
// 但**没有任何一格先告诉你那台机上在跑什么** —— 只能先 `zerg egg ls` 或手搓
// `curl 子端/eggs` 才找得到模型名，多机时还要在几台之间来回拼。
//
// 本命令把「认准之前先看清有什么」这**一步**单独开出来：数据走**现成**子端通道
// （`/eggs` 优先、`/status` 兜底 —— 与 `agent bench` 同一份回据，不另开一条路），
// 把每枚**在跑**（`port>0`）引擎的 模型 id · 状态 · pid · 端口 · managed 列成一张表。
// 取不到 ⇒ `blocked` 退 8；子端答得到但**没有在跑的** ⇒ 空结论退 0（不编、也不误报前置不在）。
//
// ★ 已跑时长（`uptime`）：子端 `/eggs` 今天**不跟踪**引擎启动时刻 ⇒ 本格如实**缺席**（空串），
//
//	**不**拿别的时间戳凑一个假值；要填满它需子端端点补 `started_at`（归 `GAP-20260927-202` 端点面）。
var agentEnginesFields = []string{"machine", "model", "state", "unit_state", "pid", "port", "uptime", "managed", "via", "argv"}

var agentEnginesDisplay = []string{"machine", "model", "state", "pid", "port", "uptime", "managed", "argv"}

// agentEngineFieldInflight / agentEngineFieldWatchdogVerdict / agentEngineFieldLastOutputAgoS ——
// 「引擎实时态」三格的**字段名**（`--json` 面用 · 缺口 `GAP-20260925-45`）。
//
// ★ 子端 `/eggs` **已经透出**这三样读数（`inflight` 在飞计数 + `watchdog` 子对象 =
// `backend.WatchdogObservation`：`last_output_ago_s` / `verdict`；见
// `agent/internal/server/eggs_endpoint.go`），而机面此前**把它们丢掉**（只解析
// `model/state/port/pid/managed/argv`）⇒ 「这台机的引擎此刻在不在算、多久没出声、看门狗怎么判的」
// 在机面上**看不见**（区分不了「挂住」与「只是慢」）。
//
// ★ 两格字段名与卵面（`egg ls` 的 `GAP-20260925-15`）**逐字同源**（`last_output_ago_s` /
// `watchdog_verdict`），`inflight` 逐字取自 `/eggs` 的键 —— ✗ 不另造词汇。
const (
	agentEngineFieldInflight        = "inflight"
	agentEngineFieldWatchdogVerdict = "watchdog_verdict"
	agentEngineFieldLastOutputAgoS  = "last_output_ago_s"
)

// agentEnginesOptionalFields —— `agent engines --json` 的**可选字段层**（`optFields`）：登在出口
// 白名单里、点得出来，但**不进** `agentEnginesFields` 那一份**冻结**的帮助 / 文档导出面字段表 ——
// 与 `eggOptionalFields` / `code find` 的 `count` 同一体例（冻结表不动、可选只增）。
var agentEnginesOptionalFields = []string{
	agentEngineFieldInflight, agentEngineFieldWatchdogVerdict, agentEngineFieldLastOutputAgoS,
}

// agentEnginesJSONFields —— `--json` 的**合法字段全集** = 冻结 `agentEnginesFields` ∪ 可选
// `agentEnginesOptionalFields`（本命令的前置闭集校验用；`emitSelected` 的出口白名单同样取这两份之并）。
var agentEnginesJSONFields = append(append([]string{}, agentEnginesFields...), agentEnginesOptionalFields...)

// agentEngineFact 子端 `/eggs` 回据里的一枚**在跑**引擎（只读投影，不做判定）。
type agentEngineFact struct {
	Model     string
	State     string
	UnitState string
	Port      int
	PID       int
	Uptime    string // 子端未跟踪启动时刻 ⇒ 恒空（缺席不编）
	Managed   bool
	Via       string
	// Argv 引擎真命令行（可执行 + 参数 · GAP-20260926-01）。nil = 子端回据里没有这一格 ⇒ 如实缺席，
	// 由渲染层**点名**（不静默当空、不编一个空数组）。
	Argv []string
	// ── 引擎实时态（GAP-20260925-45）：区分「挂住」与「只是慢」的只读读数 ──
	// Inflight 子端 `/eggs` 的 `inflight` 在飞引用计数（>0 = 此刻在算）；nil = 回据没这一格
	//   （`/status` 兜底路径没有它）⇒ 如实缺席，✗ 不编一个 0（0 与「没读法」不是一回事）。
	Inflight *int
	// Watchdog 子端 `/eggs` 的 `watchdog` 子对象（看门狗判词 + 距最近输出秒数）；nil = 没这一格。
	Watchdog *agentEngineWatchdog
}

// agentEngineWatchdog —— 子端 `/eggs` 逐枚引擎的看门狗读数（**逐字**取自 `watchdog` 子对象 · GAP-20260925-45）。
//
// ★ 两格字段名与卵面（`egg ls` 的 `GAP-20260925-15`）**逐字同源**（`last_output_ago_s` / `verdict`）
// —— 同一格不许两套词。★ 诚实边界（照卵面）：`watchdog` 是**上一次请求的快照** ⇒ 看门狗没跑时子端报
// `verdict=ok` / `last_output_ago_s=0`；本面**逐字照报**（不加工成「实时静默 N 秒」、不把 `0` 说成「未卡」）。
// 缺席（nil）= 回据里没有这一格 ⇒ 两格留空（绝不编 `ok`/`0`）。
type agentEngineWatchdog struct {
	LastOutputAgoS string // 距最近一次输出的秒数（数值文本，逐字；`0` 也是真读数）
	Verdict        string // 判词：ok / extended / stuck_cpu_stalled / stuck_no_progress / degraded / dead_unit
}

// agentEngineRow 把一枚**在跑**引擎的只读事实折成一行（含「引擎实时态」三格 · GAP-20260925-45）：
//
//	inflight          在飞引用计数（>0 = 此刻在算 · 判「在不在算」）
//	last_output_ago_s 距最近一次输出的秒数（判「还有没有在出」—— 区分「挂住」与「只是慢」）
//	watchdog_verdict  看门狗判词（ok/extended/stuck_*/degraded/dead_unit · 判「卡没卡」）
//
// 缺席（回据没那一格）⇒ 对应键**不出现**（`--json` 面整键略去、人面留空），✗ 绝不编 `0`/`ok`。
func agentEngineRow(machine string, f agentEngineFact) map[string]string {
	model := f.Model
	if model == "" {
		model = "<子端未自报模型名>"
	}
	row := map[string]string{
		"machine": machine, "model": model, "state": f.State, "unit_state": f.UnitState,
		"pid": strconv.Itoa(f.PID), "port": strconv.Itoa(f.Port), "uptime": f.Uptime,
		"managed": strconv.FormatBool(f.Managed), "via": f.Via,
		"argv": agentRenderArgv(f.Argv),
	}
	if f.Inflight != nil {
		row[agentEngineFieldInflight] = strconv.Itoa(*f.Inflight)
	}
	if f.Watchdog != nil {
		row[agentEngineFieldWatchdogVerdict] = f.Watchdog.Verdict
		row[agentEngineFieldLastOutputAgoS] = f.Watchdog.LastOutputAgoS
	}
	return agentFillRow(row, agentEnginesFields)
}

func cmdAgentEngines(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentEnginesJSONFields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent engines` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent engines` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}
	facts, rc := agentEngineFacts(inv, a, stderr)
	if rc != exitOK {
		return rc
	}
	rows := make([]map[string]string, 0, len(facts))
	for _, f := range facts {
		rows = append(rows, agentEngineRow(machine, f))
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 子端 %s 报**没有在跑的引擎**（`/eggs` 里 `port>0` 的条目 0 条、`/status.port=0`）—— 这是**空结论**（退码 0），不是取不到\n",
			progName, machine)
		fmt.Fprintf(stderr, "要测某枚：`%s agent bench %s --model <名>`；注册表清单：`%s agent registry %s --list`\n",
			progName, machine, progName, machine)
	} else {
		fmt.Fprintf(stderr, "%s: agent engines %s · 子端回据（%s）里**在跑**的引擎 %d 枚\n", progName, machine, facts[0].Via, len(rows))
		fmt.Fprintf(stderr, "口径：pid/端口/状态取自子端回据；**已跑时长这一格子端今天不跟踪引擎启动时刻 ⇒ 如实缺席**（不拿别的时间戳凑）\n")
	}
	return listCmd(inv, stdout, stderr, agentEnginesDisplay, rows)
}

// agentEngineFacts 问子端要**在跑**（`port>0`）的每一枚引擎（`/eggs` 优先、`/status` 兜底）——
// 与 `agent bench` 的 `agentRunningEggs` 同一份回据，只多带 pid / unit_state / managed 几格
// （bench 只关心「测哪一枚」，本命令要「看清有什么」）。
//
// 两处都问不到 ⇒ `blocked` 退 8（取不到就说取不到，**不编**端口）；
// `/eggs` 已给回据却没有在跑的条目 ⇒ 兜底问 `/status`，仍没有 ⇒ `[]` + 0（**空结论**，不是 blocked）。
func agentEngineFacts(inv *invocation, a agentAddr, stderr io.Writer) ([]agentEngineFact, int) {
	var eggs jsonObj
	status, body, err := agentChildDo(a, http.MethodGet, "/eggs", "", 15*time.Second)
	if err != nil {
		return nil, agentChildUnreachable(inv, a, "/eggs", err, stderr)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, agentChildAuthFail(inv, a, "/eggs", status, stderr)
	}
	gotEggs := false
	if status == http.StatusOK {
		if err := json.Unmarshal([]byte(body), &eggs); err != nil {
			fmt.Fprintf(stderr, "%s: 子端 /eggs 的响应不是合法 JSON（%v）—— 退一步问 /status\n", progName, err)
			eggs = nil
		} else {
			gotEggs = true
		}
	}
	facts := []agentEngineFact{}
	if eggs != nil {
		for _, it := range asList(eggs["eggs"]) {
			o := asObj(it)
			if o == nil {
				continue
			}
			port, _ := strconv.Atoi(cell(o["port"]))
			if port <= 0 {
				continue // 只列**在跑**的（与 bench 同口径：`port>0`）
			}
			managed := o["managed"] == nil || cell(o["managed"]) == "true"
			pid, _ := strconv.Atoi(cell(o["pid"]))
			f := agentEngineFact{
				Model: cell(o["model"]), State: cell(o["state"]), UnitState: cell(o["unit_state"]),
				Port: port, PID: pid, Managed: managed, Via: "/eggs",
				Argv: agentParseArgv(o["argv"]),
			}
			// 引擎实时态（GAP-20260925-45）：`inflight` 与 `watchdog` 子对象逐字留下。
			// 缺 `inflight`（`/status` 兜底路径）⇒ Inflight 保持 nil（缺席，不编 0）；缺 `watchdog` ⇒ nil。
			if v, ok := o["inflight"]; ok && v != nil {
				if n, perr := strconv.Atoi(cell(v)); perr == nil {
					f.Inflight = &n
				}
			}
			if w := asObj(o["watchdog"]); w != nil {
				f.Watchdog = &agentEngineWatchdog{
					LastOutputAgoS: cell(w["last_output_ago_s"]),
					Verdict:        cell(w["verdict"]),
				}
			}
			facts = append(facts, f)
		}
	}
	if len(facts) > 0 {
		return facts, exitOK
	}
	// `/eggs` 没有在跑的条目 ⇒ `/status` 兜底（子端自报的当前引擎端口）。
	var st jsonObj
	status, body, err = agentChildDo(a, http.MethodGet, "/status", "", 15*time.Second)
	if err != nil {
		if gotEggs {
			return facts, exitOK // `/eggs` 已是回据（只是没有在跑的）⇒ 空结论成立，不因 /status 打不到翻成 blocked
		}
		return nil, agentChildUnreachable(inv, a, "/status", err, stderr)
	}
	if status != http.StatusOK {
		if gotEggs {
			return facts, exitOK
		}
		inv.setErr("upstream_error", fmt.Sprintf("http_%d", status), "子端 /status 没回 200")
		fmt.Fprintf(stderr, "%s: 子端 /status ⇒ HTTP %d ⇒ 端口问不出来（退码 8）\n", progName, status)
		return nil, exitBlocked
	}
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		if gotEggs {
			return facts, exitOK
		}
		inv.setErr("blocked", "status_unparseable", "子端 /status 的响应不是合法 JSON")
		fmt.Fprintf(stderr, "%s: 子端 /status 的响应不是合法 JSON（%v）⇒ 端口问不出来（退码 8）\n", progName, err)
		return nil, exitBlocked
	}
	if port, _ := strconv.Atoi(cell(st["port"])); port > 0 {
		pid, _ := strconv.Atoi(cell(st["pid"]))
		return []agentEngineFact{{
			Model: cell(st["model"]), State: cell(st["backend_state"]),
			Port: port, PID: pid, Managed: true, Via: "/status",
		}}, exitOK
	}
	return facts, exitOK // 两处一致：没有在跑的引擎
}

// agentArgvAbsent 渲染层对「子端回据里没有 argv 这一格」的**点名**（GAP-20260926-01 负控：
// 取不到 ⇒ 不给结论、✗ 不静默当空）。人面与机器面用同一句，免得两处口径分家。
const agentArgvAbsent = "<子端未提供 argv>"

// agentParseArgv 从 /eggs 条目里解析 argv（JSON 字符串数组）。非数组 / 有非字符串元素 / 空数组
// ⇒ 一律当**缺席**返回 nil：「形状认不出」与「没有这一格」同呈现（空数组 ≠ 有、是空的）。
func agentParseArgv(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			return nil
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// agentRenderArgv 把 argv 渲染成机读串（JSON 数组、逐字保序）；缺席 ⇒ **点名**占位（不静默当空）。
func agentRenderArgv(argv []string) string {
	if len(argv) == 0 {
		return agentArgvAbsent
	}
	b, err := json.Marshal(argv)
	if err != nil {
		return agentArgvAbsent
	}
	return string(b)
}

// ---- ⑦ `zerg agent slots <机>`（缺口 `GAP-20260927-202` ② · F 族 · 只读）----
//
// 病根（缺口逐字，互证主控任务队列 `[Q-195]`）：**「谁占了几号槽、跑什么」只能手搓
// `curl http://<机>:9400/props` 与 `/slots`** —— 主控 `active_requests>0` 却答不出谁在请求。
// 本命令把这一层读数收进正门：先认准该机**在跑的引擎**（子端 `/eggs` —— 与 `agent bench` 同通道），
// 再直连**引擎本人**的 `/slots`（`/props` 取 `total_slots`）把槽位摆出来。
//
// 只读纪律：不换装载、不写引擎、不绕过主控改状态。字段名**逐字取自引擎回据**（§4.3 U2）；
// 引擎不吃 `/slots` 或形状认不出（非 JSON 数组）⇒ `blocked` 退 8 + 原样贴回据，**不给结论**。
var agentSlotsFields = []string{"machine", "model", "engine_port", "slot_id", "slot_state", "n_ctx", "prompt_chars", "slots_total", "via"}

var agentSlotsDisplay = []string{"machine", "model", "engine_port", "slot_id", "slot_state", "n_ctx", "prompt_chars", "slots_total"}

func cmdAgentSlots(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentSlotsFields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent slots` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent slots` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}
	// 认准在跑的那一枚引擎（多枚在跑而没点名 ⇒ 退 2 要你点名 —— 与 `agent bench` 同一纪律）。
	engine, rc := agentEnginePick(inv, machine, a, strings.TrimSpace(inv.modelWant), stderr)
	if rc != exitOK {
		return rc
	}
	eng := agentAddr{Host: engine.Host, Port: engine.Port}
	model := engine.Model
	if model == "" {
		model = machine + "（子端未自报模型名）"
	}
	slotsStatus, slotsBody, err := agentChildDo(eng, http.MethodGet, "/slots", "", 15*time.Second)
	if err != nil {
		inv.setErr("blocked", "engine_unreachable", fmt.Sprintf("引擎 %d 的 /slots 打不到", engine.Port))
		fmt.Fprintf(stderr, "%s: 打不到引擎 %s/slots（地址取自子端回据）⇒ **不给结论**（退码 8）\n",
			progName, agentChildBase(eng))
		fmt.Fprintf(stderr, "原因: %v\n", err)
		fmt.Fprintf(stderr, "error.kind=blocked · detail=engine_unreachable · retryable=true · remedy=fix_precondition\n")
		return exitBlocked
	}
	if slotsStatus != http.StatusOK {
		inv.setErr("blocked", fmt.Sprintf("engine_slots_http_%d", slotsStatus), "引擎没回 200（/slots）")
		fmt.Fprintf(stderr, "%s: GET %s/slots ⇒ HTTP %d\n", progName, agentChildBase(eng), slotsStatus)
		fmt.Fprintf(stderr, "引擎原样响应: %s\n", strings.TrimSpace(slotsBody))
		fmt.Fprintf(stderr, "⇒ **不给结论**（退码 8）：本命令不猜槽位形状，引擎不吃 /slots 就如实说\n")
		fmt.Fprintf(stderr, "error.kind=blocked · detail=engine_slots_http_%d · retryable=true · remedy=fix_precondition\n", slotsStatus)
		return exitBlocked
	}
	// `/props` 的 total_slots（引擎自报；取不到 ⇒ 空，不编）。
	propsTotal := ""
	if ps, pb, err := agentChildDo(eng, http.MethodGet, "/props", "", 10*time.Second); err == nil && ps == http.StatusOK {
		var p jsonObj
		if json.Unmarshal([]byte(pb), &p) == nil {
			propsTotal = cell(p["total_slots"])
		}
	}
	rows, rc := agentSlotRowsFromPayload(machine, model, engine.Port, slotsBody, propsTotal, engine.Via)
	if rc != exitOK {
		inv.setErr("blocked", "slots_shape_undecided", "引擎 /slots 回据不是 JSON 数组（形状认不出）")
		fmt.Fprintf(stderr, "%s: 引擎 /slots 的回据形状认不出（顶层不是 JSON 数组）⇒ **不给结论**（退码 8）\n", progName)
		fmt.Fprintf(stderr, "引擎原样响应: %s\n", agentSnippet(strings.TrimSpace(slotsBody), 400))
		fmt.Fprintf(stderr, "error.kind=blocked · detail=slots_shape_undecided · retryable=false · remedy=inspect_receipt\n")
		return rc
	}
	fmt.Fprintf(stderr, "%s: agent slots %s · 引擎 %s（子端回据 %s 认准）· 槽位 %d 个 · total_slots=%s\n",
		progName, machine, agentChildBase(eng), engine.Via, len(rows), orDash(propsTotal))
	fmt.Fprintf(stderr, "口径：字段名**逐字取自引擎回据**（/slots 的 id/state/n_ctx/prompt）；缺的格缺席 —— 不同引擎实现形状可不同\n")
	return listCmd(inv, stdout, stderr, agentSlotsDisplay, rows)
}

// agentSlotRowsFromPayload 把引擎 `/slots` 回据投影成表格行（纯函数：便于用回据原文钉判据，不真打引擎）。
//
// 顶层必须是 **JSON 数组**（llama.cpp 等引擎的 /slots 形状）；不是数组 ⇒ 退 8（形状未定案，**不编字段**）。
// 每槽取 id / state / n_ctx / prompt（prompt 只报**字符数**，不把整段提示词刷屏）。
func agentSlotRowsFromPayload(machine, model string, enginePort int, slotsBody, propsTotal, via string) ([]map[string]string, int) {
	var arr []any
	if err := json.Unmarshal([]byte(slotsBody), &arr); err != nil {
		return nil, exitBlocked
	}
	rows := make([]map[string]string, 0, len(arr))
	for i, it := range arr {
		o := asObj(it)
		if o == nil {
			continue
		}
		id := cell(o["id"])
		if id == "" {
			id = strconv.Itoa(i) // 引擎没自报 id ⇒ 用它在数组里的位置（**标出来**，不假装是引擎给的）
		}
		prompt := cell(o["prompt"])
		rows = append(rows, agentFillRow(map[string]string{
			"machine": machine, "model": model, "engine_port": strconv.Itoa(enginePort),
			"slot_id": id, "slot_state": cell(o["state"]), "n_ctx": cell(o["n_ctx"]),
			"prompt_chars": strconv.Itoa(len([]rune(prompt))), "slots_total": propsTotal, "via": via,
		}, agentSlotsFields))
	}
	if len(rows) == 0 && len(arr) > 0 {
		// 数组里全是非对象（形状认不出）⇒ 不给结论。
		return nil, exitBlocked
	}
	return rows, exitOK // 空数组是合法结论（引擎报了 0 个槽）⇒ 退 0、空表
}

// ---- 缺口 `GAP-20261010-21`（B 单 · CLI 命令面）：`zerg agent forget <机>`（D2 写面）----
//
// 病根（缺口逐字）：主控端点 `POST /api/fleet/forget`（A 单已落 · `core/internal/api/fleet_forget.go`）
// 此前**无命令面出口** ⇒ 运行态名册里的无效条目（probe / tuntest / …）只能等主控重启才消失
// （`GAP-20260925-29` 残面）。
//
// 口径（照 A 单端点 · ✗ 不另造第二套）：干跑 `dry_run:true` / 真删 `confirm:true`；审计由端点侧
// `HeartbeatLogger.Info("fleet-forget",…)` 落。命令面三态照同族写面先例 `model pull`（D2）：
// `--dry-run` = **本地**计划件（**一个请求都不发**）· 缺 `--yes` fail-closed 2（✗ 从不提问）·
// `--yes` 才真打端点（写面出口 `postJSON` · ✗ 不新开传输层）。✗ 不放宽任何闸 / ✗ 不碰子端 / ✗ 不重启主控。

// agentForgetDisplay —— 人面列（机器面字段表 = 命令树 `fields:` 那一份字面量）。
var agentForgetDisplay = []string{"machine", "present", "changed", "would_forget"}

// cmdAgentForget —— `zerg agent forget <机> [--dry-run | --yes] [--json <字段>]`：
// 把一台机从**主控运行态名册**里摘掉（`POST /api/fleet/forget` · 幂等：不在册 ⇒ changed=false 仍退 0）。
func cmdAgentForget(inv *invocation, stdout, stderr io.Writer) int {
	// ── 位置参数先判（零副作用 · 不碰任何目标）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent forget` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent forget` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	if machine == "" {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent forget` 要一台**非空**机器名\n", progName)
		return exitUsage
	}

	// ── `--dry-run` 与 `--yes` 同给 ⇒ 两道确认档自相矛盾 ⇒ 用法错 2（唯一共享判定 · 写盘点之前）──
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// ② 干跑：**本地**计划件，零副作用（✗ 一个请求都不发 · 同族先例 model pull）。
	if inv.dryRun {
		agentForgetPlan(stdout, machine)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未打任何请求、未改名册）")
		return exitOK
	}
	// ③ 确认档 fail-closed：缺 `--yes` ⇒ 退 2（✗ 从不提问 · 无 TTY 也一样）。
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `agent forget` 是 D2 档（真摘除名册条目）—— **缺 --yes ⇒ 不执行**\n", progName)
		agentForgetPlan(stderr, machine)
		fmt.Fprintf(stderr, "先看计划件：%s agent forget %s --dry-run\n", progName, machine)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}

	// ── 真跑：一条 POST（**唯一**写面出口 `postJSON` · ✗ 不新开传输层）──
	body := "{\"machine\":" + jstr(machine) + ",\"confirm\":true}"
	status, respBody, err := postJSON(newClient(), "/api/fleet/forget", body)
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		// 主控**回话了**（非 2xx）⇒ 不是「打不到」：走下面的「逐字透传」分支。
		status, respBody, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次摘除（http_%d）—— 看 `zerg help credentials`\n", progName, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		inv.setErr("usage", code, "主控拒绝摘除")
		// ★ 透传不翻译：主控的码（如 CONFIRM_REQUIRED / MISSING_MACHINE）逐字打出来。
		fmt.Fprintf(stderr, "%s: 主控拒绝摘除（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		return exitUsage
	}
	var resp jsonObj
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		inv.setErr("upstream_error", "bad_forget_payload", err.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	row := map[string]string{
		"dry_run":      cell(resp["dry_run"]),
		"machine":      machine,
		"present":      cell(resp["present"]),
		"changed":      cell(resp["changed"]),
		"would_forget": cell(resp["would_forget"]),
	}
	if cell(resp["changed"]) == "true" {
		fmt.Fprintf(stderr, "%s: ✅ 已把 %q 从主控运行态名册摘除（它再来一次心跳会重新出现）· 退码 0\n", progName, machine)
	} else {
		fmt.Fprintf(stderr, "%s: %q 本就不在名册里 —— **空结论**（幂等 · changed=false · 退码 0），不是错\n", progName, machine)
	}
	return listCmd(inv, stdout, stderr, agentForgetDisplay, []map[string]string{row})
}

// agentForgetPlan —— `--dry-run`（与「缺 `--yes`」那一档）的**本地**计划件：
// ✗ 不发任何请求（同族先例 `model pull` 的 plan）。真源口径见 cmdAgentForget 件头。
func agentForgetPlan(w io.Writer, machine string) {
	fmt.Fprintf(w, "🧹 agent forget 计划件（%s）\n", machine)
	fmt.Fprintf(w, "  动作     : %s agent forget %s\n", progName, machine)
	fmt.Fprintf(w, "  危险档   : %s（有影响但可逆 —— 该机再来一次心跳即回名册；要 --yes）\n", dangerD2)
	fmt.Fprintf(w, "  目标     : %s\n", machine)
	fmt.Fprintf(w, "  请求     : POST %s/api/fleet/forget {\"machine\":%q,\"confirm\":true}\n", newClient().base, machine)
	fmt.Fprintf(w, "  它会动   : 主控**运行态名册**（内核 `store.Forget`）里该机的条目 —— ✗ 不真摘除真机 · ✗ 不碰主控其它状态\n")
	fmt.Fprintf(w, "  授权判据 : 真跑要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n")
	fmt.Fprintf(w, "  来源     : 缺口账 GAP-20261010-21 · A 单端点 POST /api/fleet/forget · 同族写面先例 model pull（D2）\n")
}
