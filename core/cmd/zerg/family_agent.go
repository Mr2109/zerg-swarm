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
	fields := []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct",
		"mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"}
	row := project(o, fields)
	row["machine"] = machine
	// 「命令有什么能力」不另造词汇：字段名逐字取自载荷（§4.3 U2）。
	return listCmd(inv, stdout, stderr, []string{"machine", "healthy", "code_version", "code_sha",
		"cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
		[]map[string]string{row})
}

// cmdAgentModels —— `zerg agent models <机>`：该机上可用的模型（投影 `/api/fleet/models` 里 host 命中的那些）。
func cmdAgentModels(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent models` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := inv.args[0]
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

// cmdAgentLogs —— `zerg agent logs <机>`：**端点缺 ⇒ 不给结论**（不许拿别的东西当日志流）。
//
// 口径（同 T-17 的 `watch`）：主控面今天**没有**日志端点（`/api/logs/*` 不在路由表里）⇒
// 本命令**声明面 + `kind=blocked` + 退码 8**；**不**去读 `/tmp` 下的文件、**不**偷偷直连子端
// （§九 M20 `O2`「不偷偷换源」）。日志端点落地后本命令即通（`retryable=true`）。
func cmdAgentLogs(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent logs` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := inv.args[0]
	c := newClient()
	var caps jsonObj
	haveLogs := false
	if err := c.getJSON("/api/capabilities", &caps); err == nil {
		for _, it := range asList(caps["capabilities"]) {
			o := asObj(it)
			if o == nil {
				continue
			}
			if strings.Contains(cell(o["endpoint"]), "/api/logs") {
				haveLogs = true
			}
		}
	}
	if haveLogs {
		// 端点有了 ⇒ 本版仍不猜它的载荷形状（形状未定案 ⇒ 不给结论，不许自己编字段）
		inv.setErr("blocked", "endpoint_shape_undecided", "日志端点已在能力面，但载荷形状未定案")
		fmt.Fprintf(stderr, "%s: 主控能力面出现了 `/api/logs` 类端点，但**载荷形状未定案** ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	inv.setErr("blocked", "logs_endpoint_absent", "主控面今天没有日志端点（/api/logs/*）")
	fmt.Fprintf(stderr, "%s: 主控面**没有**日志端点 ⇒ 拿不到 %q 的日志，**不给结论**（退码 8）\n", progName, machine)
	fmt.Fprintf(stderr, "error.kind=blocked · retryable=true · remedy=fix_precondition（端点落地后本命令即通）\n")
	fmt.Fprintf(stderr, "不做的：不读 /tmp 的日志文件当结果、不偷偷直连子端（§九 M20 O2）\n")
	return exitBlocked
}

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
var agentEnginesFields = []string{"machine", "model", "state", "unit_state", "pid", "port", "uptime", "managed", "via"}

var agentEnginesDisplay = []string{"machine", "model", "state", "pid", "port", "uptime", "managed"}

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
}

func cmdAgentEngines(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentEnginesFields); rc != exitOK {
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
		model := f.Model
		if model == "" {
			model = "<子端未自报模型名>"
		}
		rows = append(rows, agentFillRow(map[string]string{
			"machine": machine, "model": model, "state": f.State, "unit_state": f.UnitState,
			"pid": strconv.Itoa(f.PID), "port": strconv.Itoa(f.Port), "uptime": f.Uptime,
			"managed": strconv.FormatBool(f.Managed), "via": f.Via,
		}, agentEnginesFields))
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
			facts = append(facts, agentEngineFact{
				Model: cell(o["model"]), State: cell(o["state"]), UnitState: cell(o["unit_state"]),
				Port: port, PID: pid, Managed: managed, Via: "/eggs",
			})
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
