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
//	`node-a` **argv 硬判据**：`zerg`（及门禁脚本、`scripts/` 任何一件）**永不**出现在茧壁的 argv 里；
//	     空间内可执行件只有该卵声明的引擎及其依赖。
//
// 另加两条：`X5` **空间内自报不可信**（证据取自空间之外）；`X7` **逃生门默认不存在**
// （`--in-space` / `api call` 一类不给就是不给，不给「以后加个开关」这种隐含承诺）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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

// helpWall —— `zerg help enclosure`（§九 M10 的自描述面）。
// ★ 2026-10-08：本主题原名为 `wall`；因新立**族** `zerg wall` 与之撞名 ⇒ 主题改名 `enclosure`
// 腾出 `wall` 给新族（`待拍清单-AlltoGo-20261008.md` P3 · 方案甲）。函数名 `helpWall` 保留不改
// （别名与机器面无涉，仅人读名；改名只动主题表那一处 + 本注释）。
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
	b.WriteString("\n`X" + "3` 可验硬判据（都能红）：\n")
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
			// ★ 缺口 GAP-20260925-20：`answered_by` —— **谁答的探活**。本档答话的是**主控**：
			//   取自主控在 `/api/fleet/status` 响应里**自报的自身身份**
			//   （`master_code_version` / `master_origin_sha` —— 由主控自己算、自己随响应报出），
			//   ✗ 不是调用方写死的常量（改前正是那句常量 `"core（经主控 /api/fleet/status）"`）。
			row["answered_by"] = coreAnsweredBy(fleet)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
		}
		fmt.Fprintf(stdout, "%s\thealthy=%s\tcode_version=%s\tvia=%s\n",
			machine, cell(o["healthy"]), cell(o["code_version"]), via)
		return exitOK
	}
	// 显式直连（`F-2` 例外）：**端口探测 + 一次身份自报读取**（只连只读面，不送业务），输出必带 `via=direct`。
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
		fmt.Fprintf(stderr, "%s: via=%s（直连，未经主控 · F-2 例外）—— 打不到 %s: %v%s\n", progName, via, hostPort, err, dialErrHint(hostPort, err.Error()))
		inv.setErr("unreachable", "dial_failed", err.Error())
		return exitUnreachable
	}
	_ = conn.Close()
	// ★ 缺口 GAP-20260925-20（**治本**）：连上**不是终点** —— 向对端**只读面**读一次
	//   **自报身份**（`GET /ready`：无令牌、不送业务、不落盘）。收方（本命令）**只校验并透传**：
	//   ① 校验信封是不是虫族子端 `/ready` 形状、自报 `machine` 是否非空；
	//   ② 再与本方**声称**的机器名对照（`--direct` 前的第一个位置参数）——
	//      不符 ⇒ 当场标出「声称 A · 实答 B」（`identity=mismatch`）。
	//   ✗ 铁律：任何一档都**不**拿入参（地址 / 声称名）顶替自报值（改前正是「回显随输入变、不随对端变」）。
	claimed := ""
	if len(inv.args) > 1 {
		claimed = strings.TrimSpace(inv.args[0]) // 位置 1 = 声称的机器名；位置 2 = 直连地址
	}
	reported, selfReported, selfDetail := peerSelfReport(hostPort, 2*time.Second)
	answeredBy, identity := answeredByOf(claimed, reported, selfReported)
	machineCell := reported
	if !selfReported {
		machineCell = "（未自报）"
	}
	fmt.Fprintf(stderr, "%s: %s\n", progName, selfDetail)
	if inv.jsonGiven {
		row := map[string]string{
			"machine":     machineCell,
			"healthy":     "true",
			"via":         via,
			"answered_by": answeredBy + " · identity=" + identity,
		}
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s\treachable=true\tvia=%s（直连，未经主控 · F-2 例外）\tanswered_by=%s · identity=%s\n",
		hostPort, via, answeredBy, identity)
	return exitOK
}

// ── GAP-20260925-20：探活面的「谁答的」＝**对端自报且可核**（收方只校验并透传，✗ 不由调用方填）──────
//
// 病根（改前）：`--direct` 只做一次 TCP 连接即关闭 —— **零协议交互** ⇒ 结构上不可能知道对端是谁；
// 回显只能拿**调用方输入的地址 / 名册名**顶替 ⇒ 同一链路挂多台时 CLI 认不出「答话的是谁」，
// 乃至把**外部服务**报成 `reachable=true`（`GAP-20260925-28` 的假阳性同根）。
//
// 治本：直连档连上后**向对端只读面读一次身份**（`GET /ready` · 无令牌 · 不送业务、不落盘）——
// 由**回答方在响应里带自己的身份**（`machine` 一格）；本命令（收方）**只做两件事**：
//
//	① 校验：信封必须是虫族子端 `/ready` 形状（含 `ready`(bool) + `phase`(string)）且自报
//	   `machine` 非空 ⇒ 才算「已自报」；认不出（外部服务 / 401 / 非 JSON / 无 machine）⇒「未自报」。
//	② 透传 + 对账：把自报 `machine` 原样摊进 `machine` / `answered_by`，并与调用方**声称**的
//	   机器名对照 —— 不符 ⇒ 当场标出「声称 A · 实答 B」。
//
// ✗ 铁律：任何一档都不把入参写进「答话者」位（改前正是这一支：回显随输入变）。

// peerSelfReport 直连探活时**读一次对端自报**（只读 `GET /ready`：无令牌、不送业务、不落盘）。
// 返回 (自报 machine, 是否已自报, 依据文本)；依据文本只含对端回据，**不含任何入参**。
func peerSelfReport(hostPort string, timeout time.Duration) (machine string, ok bool, detail string) {
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Get("http://" + hostPort + "/ready")
	if err != nil {
		return "", false, "对端只读面 /ready 读不到（" + err.Error() + "）⇒ 未自报"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return parsePeerSelfReport(resp.StatusCode, body)
}

// parsePeerSelfReport 只做**结构与身份校验**（纯函数 ⇒ 成对负控可直调）。
// 四档判词都指向「为什么认不出来」，✗ 不掺入参、✗ 不猜身份。
func parsePeerSelfReport(status int, body []byte) (machine string, ok bool, detail string) {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return "", false, fmt.Sprintf("对端回 HTTP %d（401/403 —— 像**外部服务**占了该端口，非虫族子端）⇒ 未自报", status)
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		return "", false, fmt.Sprintf("对端回 HTTP %d 但**不是 JSON 对象**（像外部服务，非虫族子端）⇒ 未自报", status)
	}
	_, hasReady := env["ready"].(bool)
	_, hasPhase := env["phase"].(string)
	if !hasReady || !hasPhase {
		return "", false, fmt.Sprintf("对端 HTTP %d 的响应**不是子端 `/ready` 信封**（缺 ready/phase ⇒ 非本族）⇒ 未自报", status)
	}
	m := strings.TrimSpace(cell(env["machine"]))
	if m == "" {
		return "", false, "对端是子端 `/ready` 形状但**未自报 machine**（该端未起自报面）⇒ 未自报"
	}
	return m, true, "对端 `/ready` **自报** machine=" + m
}

// answeredByOf 收方**只校验并透传**：由对端**自报**值（reported / selfReported）与本方**声称**值
// （claimed）合成一格 `answered_by` 与机器可读判词 `identity=`（三值闭集，可 grep）：
//
//	verified   对端自报且与声称相符（调用方未声称也算相符 —— 无可对照的声称）
//	mismatch   对端自报**与声称不符** ⇒ 把「声称 A · 实答 B」当场摊开（本缺口的核心症状）
//	unverified 对端**未自报**（认不出信封 / 无 machine）
//
// ✗ 铁律：**claimed 只许出现在「与声称 X 不符」的对照里**，✗ 不得占据「答话者」位。
func answeredByOf(claimed, reported string, selfReported bool) (answeredBy, identity string) {
	if !selfReported {
		return "（未自报）", "unverified"
	}
	if claimed != "" && claimed != reported {
		return fmt.Sprintf("对端自报 machine=%s（与声称 %q 不符）", reported, claimed), "mismatch"
	}
	return "对端自报 machine=" + reported, "verified"
}

// coreAnsweredBy 非直连档答话的是**主控**：`answered_by` 取自主控在 `/api/fleet/status`
// 响应里**自报的自身身份**（`master_code_version` / `master_origin_sha` —— 都是主控自己算、
// 自己随响应报出的），✗ 不是调用方写死的常量（改前正是那句常量）。两枚都缺 ⇒ 未自报
// （fail-closed：✗ 不拿常量顶替）。
func coreAnsweredBy(fleet jsonObj) string {
	v := strings.TrimSpace(cell(fleet["master_code_version"]))
	sha := strings.TrimSpace(cell(fleet["master_origin_sha"]))
	if v == "" && sha == "" {
		return "（未自报） · identity=unverified · 主控响应未带自身身份字段"
	}
	if len(sha) > 16 {
		sha = sha[:16]
	}
	if sha == "" {
		sha = "（未注入）"
	}
	return fmt.Sprintf("core（主控自报 code_version=%s · origin_sha=%s） · identity=verified", v, sha)
}
