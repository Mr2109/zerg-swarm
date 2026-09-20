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
	"fmt"
	"io"
	"sort"
	"strings"
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
		"mem_available_gb", "mem_total_gb", "models", "last_seen"}
	row := project(o, fields)
	row["machine"] = machine
	// 「命令有什么能力」不另造词汇：字段名逐字取自载荷（§4.3 U2）。
	return listCmd(inv, stdout, stderr, []string{"machine", "healthy", "code_version", "code_sha",
		"cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "last_seen"},
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
	fields := []string{"id", "host", "backend", "modality", "mem_gb", "file"}
	rows := []map[string]string{}
	for _, it := range asList(resp["models"]) {
		o := asObj(it)
		if o == nil || cell(o["host"]) != machine {
			continue
		}
		rows = append(rows, project(o, fields))
	}
	return listCmd(inv, stdout, stderr, []string{"id", "host", "backend", "modality", "mem_gb"}, rows)
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
