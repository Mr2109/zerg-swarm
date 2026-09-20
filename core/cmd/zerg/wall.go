// wall.go —— 茧壁边界（§九 M10 · 调研-M10 条文 `X1`–`X10`）。
//
// 一句话结论（M10）：**命令面是茧壁外的控制面** —— `zerg` 永不作茧壁绑定，**空间内零命令面**。
//
// 本文件是三条可判定的东西的落点：
//
//	`X1` **每条命令带「层级标记」**（`layer`，闭集三值：`host` / `node` / `space`）——
//	     「哪一层跑」是机器可读形状，取值不在闭集 ⇒ 门禁红（`scripts/gates/check-wall-layer.py`）。
//	`X2` **分区表**：A 类必须宿主 / B 类允许空间内 / C 类禁止空间内 —— 现状 **B 类为空集**，
//	     本版 `space` 档的命令集**恒为空**（判据：一条都没有）。
//	`X3` **argv 硬判据**：`zerg`（及门禁脚本、`scripts/` 任何一件）**永不**出现在茧壁的 argv 里；
//	     空间内可执行件只有该卵声明的引擎及其依赖。
//
// 另加两条：`X5` **空间内自报不可信**（证据取自空间之外）；`X7` **逃生门默认不存在**
// （`--in-space` / `api call` 一类不给就是不给，不给「以后加个开关」这种隐含承诺）。
package main

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"
)

// layer 闭集（`X1`）：**三值**，不许自造第四档。
const (
	layerHost  = "host"  // T0 主控机
	layerNode  = "node"  // T1 目标机宿主侧
	layerSpace = "space" // T2 茧壁之内（**本版空集**）
)

// layerClosedSet 闭集（门禁与帮助共用一处）。
func layerClosedSet() []string { return []string{layerHost, layerNode, layerSpace} }

// layerOf 按族派生层级（照 `X2` 分区表的推荐默认值；逐条要改就在命令上显式写 `layer`）。
func layerOf(c *command) string {
	if c.layer != "" {
		return c.layer
	}
	head := c.path[0]
	switch head {
	case "agent":
		// 子端族的动作落到**目标机宿主侧**；`agent ping` 默认经主控（§十五.4 丙案）⇒ host。
		if len(c.path) > 1 && c.path[1] == "ping" {
			return layerHost
		}
		return layerNode
	case "egg":
		// 孵一枚卵 = **建**边界 ⇒ 目标机宿主侧（`X2` I 族）。
		return layerNode
	case "build", "cocoon", "core", "doc", "gate", "gateway", "api", "resource", "script",
		"task", "model", "context", "help", "version", "doctor", "update", "watch", "plugin", "svc":
		return layerHost
	default:
		return layerHost
	}
}

// seedLayer 给命令树补齐层级（没显式写的按族派生 ⇒ 一条不漏）。
func seedLayer() {
	for _, c := range commands {
		if c.layer == "" {
			c.layer = layerOf(c)
		}
	}
}

// layerCounts 出「三档各多少条」的机器可读摘要（进导出物的 `--json`）。
func layerCounts() string {
	n := map[string]int{}
	for _, c := range commands {
		n[c.layer]++
	}
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, n[k]))
	}
	return strings.Join(parts, ",")
}

// helpWall —— `zerg help wall`（§九 M10 的自描述面）。
func helpWall() string {
	var b strings.Builder
	b.WriteString("茧壁边界（§九 M10 · 调研-M10 条文 `X1`–`X10`）\n\n")
	b.WriteString("一句话：**命令面是茧壁外的控制面** —— `zerg` 永不作茧壁绑定，**空间内零命令面**。\n\n")
	b.WriteString("`X1` 层级标记（**闭集三值**，每条命令都有）：\n")
	b.WriteString("  host   T0 主控机（控制面、门禁、构建、任务队列…）\n")
	b.WriteString("  node   T1 目标机宿主侧（孵卵/收卵、装载/卸载）\n")
	b.WriteString("  space  T2 茧壁之内 —— **本版恒为空集**（`P-047`/`P-048`：不进、也不暴露子集）\n")
	fmt.Fprintf(&b, "  本机现读计数：%s（machine-readable 面见 `zerg help export --json layers`）\n", layerCounts())
	b.WriteString("\n`X2` 分区表（三类 · 判据只有一条）：\n")
	b.WriteString("  A 类 必须宿主：碰能力（建/收边界、归属、凭据、准入、观测边界）⇒ 绝大多数命令\n")
	b.WriteString("  B 类 允许空间内：**现状空集**（要开按 `X6` 三条先决 + 白名单）\n")
	b.WriteString("  C 类 禁止空间内：`egg` 孵化/收卵 · `agent load/unload` · `core stop/start` ·\n")
	b.WriteString("        `model start/stop` · `gate run` · `script run` · `api call` · `context use` · `update`\n")
	b.WriteString("\n`X3` 可验硬判据（都能红）：\n")
	b.WriteString("  · 任意 `plan` 产物的 argv 里，**空间内落点** ⊆ 该卵声明的挂载集合；\n")
	b.WriteString("  · 全部 `plan` 产物里 **`zerg` 零命中**；\n")
	b.WriteString("  · 既有 `EnclosureReport` 的「宿主权重树/数据目录在空间内看不见」那一档**不被放宽**。\n")
	b.WriteString("  接线：`scripts/gates/check-wall-layer.py`（gates scope · 阻断档）。\n\n")
	b.WriteString("`X5` **空间内自报不可信**：巡检/审计的证据一律取自空间**之外**（空间内说什么都只当线索）。\n")
	b.WriteString("`X7` **逃生门默认不存在**：`--in-space` / `api call` 这类不给就是不给 ——\n")
	b.WriteString("     不许留「以后加个开关」的隐含承诺（要开就走提案 + 人批）。\n")
	b.WriteString("`X8` **不新增退码**：越界一律用既有码（用法错 `2` / 不给结论）。\n\n")
	b.WriteString("`agent ping` 与「命令面必须经主控」的冲突 —— **已闭合**（§十二 `P-052` · §十五.4 丙案）：\n")
	b.WriteString("  · 默认 **经主控**（`zerg agent ping <机>` ⇒ HTTP 打主控，`meta.via=\"core\"`）；\n")
	b.WriteString("  · 直连降为**显式** `--direct`（属 §十五.4 例外清单 `F-2` 探活/诊断），输出**必带 `via`**；\n")
	b.WriteString("  · 「不许绕过主控」的理由是**一份凭据 + 一处审计**，**不是**茧壁边界（M20 §5.3 的措辞更正）。\n")
	return b.String()
}

// cmdAgentPing —— `zerg agent ping <机>`（§十五.4 丙案：默认经主控；`--direct` 才直连且回显 `via`）。
func cmdAgentPing(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `agent ping` 要一台机器名（`zerg agent ls` 看名册）\n", progName)
		inv.setErr("usage", "missing_target", "缺机器名")
		return exitUsage
	}
	machine := inv.args[0]
	if !inv.direct {
		// 默认档：经主控（与其余 agent 命令同一条路 —— 一份凭据、一处审计）
		c := newClient()
		var fleet jsonObj
		if rc := fetchInv(inv, c, "/api/fleet/status", &fleet, stderr); rc != exitOK {
			return rc
		}
		o := asObj(asObj(fleet["machines"])[machine])
		if o == nil {
			inv.setErr("unsupported_on_node", "unknown_machine", "名册里没有这台机")
			fmt.Fprintf(stderr, "%s: 名册里没有这台机 %q（§九 M13 Z3）\n", progName, machine)
			return exitFail
		}
		via := "core"
		fmt.Fprintf(stderr, "%s: via=%s（**经主控** —— 默认档 · §十五.4 丙案）\n", progName, via)
		if inv.jsonGiven {
			row := project(o, []string{"machine", "healthy", "code_version", "code_sha", "last_seen"})
			row["via"] = via
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintf(stdout, "%s\thealthy=%s\tcode_version=%s\tvia=%s\n",
			machine, cell(o["healthy"]), cell(o["code_version"]), via)
		return exitOK
	}
	// 显式直连（`F-2` 例外）：**端口探测**（只连不送业务），输出必带 `via=direct`。
	r, ok := loadRoster(defaultContextName)
	if !ok {
		inv.setErr("blocked", "roster_absent", "名册不在 ⇒ 拿不到直连地址（不给结论）")
		fmt.Fprintf(stderr, "%s: 名册不在 ⇒ `--direct` 拿不到地址（不给结论 · §九 M20 O3）\n", progName)
		return exitBlocked
	}
	_ = r
	hostPort := strings.TrimSpace(inv.args[0])
	if len(inv.args) > 1 {
		hostPort = inv.args[1]
	}
	// 直连要一个**地址**（host:port）—— 给不出端口就是用法错（2），不是「打不到」（12）：
	// 码要区分「你没给对地址」与「地址给了、打不通」（§九 M7 `E1`）。
	if !strings.Contains(hostPort, ":") {
		fmt.Fprintf(stderr, "%s: `--direct` 要 `host:port`（给的是 %q —— 少了端口）\n", progName, hostPort)
		fmt.Fprintf(stderr, "先看名册里的地址: %s context ls；或 `%s help remote`\n", progName, progName)
		inv.setErr("usage", "bad_address", "直连要 host:port 形态")
		return exitUsage
	}
	conn, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	via := "direct"
	if err != nil {
		fmt.Fprintf(stderr, "%s: via=%s（直连，未经主控 · F-2 例外）—— 打不到 %s: %v\n", progName, via, hostPort, err)
		inv.setErr("unreachable", "dial_failed", err.Error())
		return exitUnreachable
	}
	_ = conn.Close()
	fmt.Fprintf(stdout, "%s\treachable=true\tvia=%s（直连，未经主控 · F-2 例外）\n", hostPort, via)
	return exitOK
}
