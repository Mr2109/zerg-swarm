// family_fleet.go —— `zerg fleet render`（E1 · 缺失项 `GAP-20260927-172` 之正解）：
//
//	由**真源**（机器名册 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`）生成**投影**
//	（`gateway/fleet.yaml` 的 `fleet:` 段与 `updates.nodes`）—— 消「两处手抄 ⇒ 漂移」。
//
// 出处与地位（契约 §九 M13 `Z2` · 稿 E v2 §2 F1′ 逐字）：
//
//	· 真源 = `~/.zerg/contexts/<档>.yaml` 的 `nodes:`（本机物件 · 不入仓）；
//	· 投影 = `gateway/fleet.yaml` 的 `fleet:`（子端寻址：host/port/os/kind）与
//	  `updates.nodes`（换件 ssh 目标：ssh/root/prefix/api/components）。
//	权威字段映射 = 门④ `check-nodes-roster` 的判据 C / D（`check_nodes_roster.go:986`/`:1021`）——
//	本件**复用其解析原语**（`cnrReadTextLines` / `cnrSectionRange` / `cnrParseBlock` /
//	`cnrLoadRoster` / `cnrStripComment` / `cnrStripQuotes`）⇒ **零第二套解析口径**
//	（✗ 不改门、✗ 不造第二条真源）。
//
// 三档（旗标名逐字取稿 E v2 §2）：
//
//	· `--preview-only`（**缺省档**）：只打印**将要写的投影全文**（零写盘 · rc=0）；
//	· 全局 `--dry-run`：**等价于缺省档** `--preview-only`（判据逐字见 `preview := inv.hasFlag("--preview-only") || inv.dryRun`）
//	  —— 本件是「**干跑语义站点**」，故该字面在此显式登记（门⑩ 一套语义 · `dryrun.v1`）；
//	· `--emit`＋`--yes`：**真写**回投影件（D2 惯例 = 同目录临时件＋rename 原子写＋读回校＋
//	  任一步不过即回滚 · `writeFleetAtomic` · 先例 `zerg model add`）；
//	· `--expect-no-changes`：**只读断言** —— 现读投影 ⟷ 名册生成逐字段比，不一致 ⇒ 非 0
//	  （F5 漂移门用：`fleet render --preview-only --expect-no-changes`）。
//
// ★★ 写作面**只碰投影件**（`gateway/fleet.yaml` 的**两段数据行**）——
//
//	✗ 不改真源（`~/.zerg/contexts/…`）、✗ 不碰 `models:`/`auth:`/`aliases:`、
//	✗ 不碰两段里的**注释行**（逐字保留 —— 其中就有门④判据 B 要的「投影源声明」）。
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ── 权威字段映射（照门④ `check-nodes-roster` 判据 C / D 逐字 · 不自造）──────────────
var (
	// fleetRenderFleetFields —— `fleet:` 段（子端寻址）的字段：判据 C 核 host/port/os；
	// `kind` 是该段既有列（在册三台都有）⇒ 随投影一并带上（多一列不破判据，少一列会丢信息）。
	fleetRenderFleetFields = []string{"host", "port", "os", "kind"}
	// fleetRenderNodeFields —— `updates.nodes`（换件 ssh 目标）的字段：判据 D 逐字这五枚。
	fleetRenderNodeFields = []string{"ssh", "root", "prefix", "api", "components"}
)

// fleetRenderJSONFields —— `--json` 字段面（与命令树 `fields:` 那条**字面量**逐字同）。
var fleetRenderJSONFields = []string{"action", "roster", "fleet", "nodes", "drift", "changed", "result"}

// fleetRenderNode —— 一个名册节点的**投影面**（两段各自的字段值 · 均为去引号后的字面值）。
type fleetRenderNode struct {
	name  string
	fleet map[string]string // host/port/os/kind
	node  map[string]string // ssh/root/prefix/api/components
}

// ─────────────────────────── 取值 / 文本原语 ───────────────────────────

// fleetRenderSplitLines —— 与 `cnrReadTextLines` 同折行口径（`\r\n` / 裸 `\r` → `\n`），
// 但**保留原始字节拼回**用于回写（`strings.Split(raw, "\n")` 再 `Join` 即逐字还原）。
func fleetRenderSplitLines(raw string) []string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// fleetRenderStrip —— 逐行去注释（复用门④的 `cnrStripComment`）⇒ 段范围判定用。
func fleetRenderStrip(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = cnrStripComment(l)
	}
	return out
}

// fleetRenderSortedNames —— 排好序的名字切片（唯一进输出的序来源 · ✗ 不用 Go map 迭代序）。
func fleetRenderSortedNames(names map[string]bool) []string {
	ks := make([]string, 0, len(names))
	for k := range names {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// fleetRenderVal —— flow 行里的一个值：只在**需要**时加引号（空串、含 `: , { } [ ] # " \`
// 或含空白）。⇒ 地址/端口/`os`/`kind` 一类裸值保持裸形（与现册 `fleet:` 段逐字同形）。
func fleetRenderVal(v string) string {
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, ":#,{}[]\"\\") || strings.ContainsAny(v, " \t") {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
	}
	return v
}

// fleetRenderFleetLine —— 出 `fleet:` 段的一条（与现册同形：两格缩进 + flow 映射）。
func fleetRenderFleetLine(n fleetRenderNode) string {
	parts := make([]string, 0, len(fleetRenderFleetFields))
	for _, f := range fleetRenderFleetFields {
		parts = append(parts, f+": "+fleetRenderVal(n.fleet[f]))
	}
	return "  " + n.name + ": { " + strings.Join(parts, ", ") + " }"
}

// fleetRenderNodeLine —— 出 `updates.nodes` 的一条（四格缩进 + flow 列表项 · `name:` 领衔）。
func fleetRenderNodeLine(n fleetRenderNode) string {
	parts := make([]string, 0, len(fleetRenderNodeFields)+1)
	parts = append(parts, "name: "+fleetRenderVal(n.name))
	for _, f := range fleetRenderNodeFields {
		parts = append(parts, f+": "+fleetRenderVal(n.node[f]))
	}
	return "    - { " + strings.Join(parts, ", ") + " }"
}

// ─────────────────────────── 读真源 / 读现值 ───────────────────────────

// fleetRenderRosterPath —— 真源件路径：`--roster` 优先 → 档位件 `~/.zerg/contexts/<档>.yaml`
// （`--context` 缺省 `default` · 复用 `contextYAMLPath`）。
func fleetRenderRosterPath(inv *invocation) (string, string) {
	if p := strings.TrimSpace(inv.flagVal("--roster")); p != "" {
		return p, ""
	}
	return contextYAMLPath(inv)
}

// fleetRenderRoster —— 读真源 `nodes:` → 排好序的节点切片（复用门④ `cnrLoadRoster`）。
func fleetRenderRoster(rosterPath string) ([]fleetRenderNode, string) {
	nodes, msg := cnrLoadRoster(rosterPath)
	if msg != "" {
		return nil, msg
	}
	out := make([]fleetRenderNode, 0, len(nodes))
	for _, name := range cnrSortedKeys(nodes) {
		m, _ := nodes[name].(map[string]cnrValue)
		n := fleetRenderNode{
			name:  cnrStripQuotes(name),
			fleet: map[string]string{},
			node:  map[string]string{},
		}
		for _, f := range fleetRenderFleetFields {
			n.fleet[f] = cnrStripQuotes(cnrToString(m[f]))
		}
		for _, f := range fleetRenderNodeFields {
			n.node[f] = cnrStripQuotes(cnrToString(m[f]))
		}
		out = append(out, n)
	}
	return out, ""
}

// fleetRenderCurrentFleet —— 解析现值 `fleet:` 段 → name → {host,port,os,kind}。
func fleetRenderCurrentFleet(lines []string) (map[string]map[string]string, bool) {
	stripped := fleetRenderStrip(lines)
	s, e, ok := cnrSectionRange(stripped, "fleet")
	if !ok {
		return nil, false
	}
	blk := cnrParseBlock(stripped[s:e])
	out := map[string]map[string]string{}
	for k, v := range blk {
		m, ok := v.(map[string]cnrValue)
		if !ok {
			continue
		}
		row := map[string]string{}
		for _, f := range fleetRenderFleetFields {
			row[f] = cnrStripQuotes(cnrToString(m[f]))
		}
		out[cnrStripQuotes(k)] = row
	}
	return out, true
}

// fleetRenderCurrentNodes —— 解析现值 `updates.nodes` → 条目切片（每条含 name + 五字段）。
func fleetRenderCurrentNodes(lines []string) ([]map[string]string, bool) {
	stripped := fleetRenderStrip(lines)
	s, e, ok := cnrSectionRange(stripped, "updates")
	if !ok {
		return nil, false
	}
	blk := cnrParseBlock(stripped[s:e])
	v, ok := blk["nodes"]
	if !ok {
		return nil, false
	}
	lst, ok := v.([]cnrValue)
	if !ok {
		return nil, false
	}
	out := make([]map[string]string, 0, len(lst))
	for _, it := range lst {
		m, ok := it.(map[string]cnrValue)
		if !ok {
			continue
		}
		row := map[string]string{"name": cnrStripQuotes(cnrToString(m["name"]))}
		for _, f := range fleetRenderNodeFields {
			row[f] = cnrStripQuotes(cnrToString(m[f]))
		}
		out = append(out, row)
	}
	return out, true
}

// ─────────────────────────── 断言：现值 ⟷ 生成 ───────────────────────────

// fleetRenderDrift —— 「两处手抄」的**逐项漂移**判：判据 C（fleet: 两向键集 + 逐字段）与
// 判据 D 的**加强版**（updates.nodes 也两向：名册里每台都必须在册 —— 这正是 C1「漏项」判据）。
// 返回漂移条目（空 ⇒ 一致）。
func fleetRenderDrift(roster []fleetRenderNode, curFleet map[string]map[string]string, curNodes []map[string]string) []string {
	drift := []string{}

	// ── fleet: 段 ──
	wantF := map[string]bool{}
	for _, n := range roster {
		wantF[n.name] = true
	}
	haveF := map[string]bool{}
	for k := range curFleet {
		haveF[k] = true
	}
	for _, k := range fleetRenderSortedNames(haveF) {
		if !wantF[k] {
			drift = append(drift, fmt.Sprintf("fleet: 段的 `%s` 不在名册（子端名册没登记它）", k))
		}
	}
	for _, n := range roster {
		cur, ok := curFleet[n.name]
		if !ok {
			drift = append(drift, fmt.Sprintf("fleet: 段缺名册里的 `%s`（名册有、投影没有）", n.name))
			continue
		}
		for _, f := range fleetRenderFleetFields {
			if cur[f] != n.fleet[f] {
				drift = append(drift, fmt.Sprintf("fleet: `%s` 的 %s 漂了：现读=%q 名册=%q", n.name, f, cur[f], n.fleet[f]))
			}
		}
	}

	// ── updates.nodes ──
	curByName := map[string]map[string]string{}
	for _, r := range curNodes {
		curByName[r["name"]] = r
	}
	wantN := map[string]bool{}
	for _, n := range roster {
		wantN[n.name] = true
	}
	haveN := map[string]bool{}
	for k := range curByName {
		haveN[k] = true
	}
	for _, k := range fleetRenderSortedNames(haveN) {
		if !wantN[k] {
			drift = append(drift, fmt.Sprintf("updates.nodes 里的 `%s` 不在名册", k))
		}
	}
	for _, n := range roster {
		cur, ok := curByName[n.name]
		if !ok {
			drift = append(drift, fmt.Sprintf("updates.nodes 缺名册里的 `%s`（漏项 ⇒ C1 型漂移 · 名册有、投影没有）", n.name))
			continue
		}
		for _, f := range fleetRenderNodeFields {
			if cur[f] != n.node[f] {
				drift = append(drift, fmt.Sprintf("updates.nodes `%s` 的 %s 漂了：现读=%q 名册=%q", n.name, f, cur[f], n.node[f]))
			}
		}
	}
	return drift
}

// ─────────────────────────── 生成：整件投影全文 ───────────────────────────

// frSpliceBody —— 段体内把**条目行**换成本段生成的行：段首注释/空行**原位保留**，
// 生成的条目**全部**落在本段**第一条条目**的位置，其后的注释/空行（含文件末尾那个
// 换行占位空串）**原位保留** ⇒ 注释零丢失、末尾换行字节不变（✗ 不吞 `models:`/注释）。
func frSpliceBody(body []string, gen []string) []string {
	first := -1
	for i, l := range body {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") {
			first = i
			break
		}
	}
	if first < 0 {
		b := body
		eofNL := false
		if n := len(b); n > 0 && b[n-1] == "" {
			eofNL = true
			b = b[:n-1]
		}
		out := append([]string{}, b...)
		out = append(out, gen...)
		if eofNL {
			out = append(out, "")
		}
		return out
	}
	out := make([]string, 0, len(body)+len(gen))
	out = append(out, body[:first]...)
	out = append(out, gen...)
	for _, l := range body[first:] {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			out = append(out, l)
		}
	}
	return out
}

// fleetRenderRewriteFleet —— 换 `fleet:` 段的**数据行**（注释/空行逐字保留 · 生成条目落在首条位置）。
func fleetRenderRewriteFleet(lines []string, entries []string) ([]string, bool) {
	stripped := fleetRenderStrip(lines)
	s, e, ok := cnrSectionRange(stripped, "fleet")
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(lines)+len(entries))
	out = append(out, lines[:s]...)
	out = append(out, frSpliceBody(lines[s:e], entries)...)
	out = append(out, lines[e:]...)
	return out, true
}

// fleetRenderRewriteNodes —— 换 `updates:` 段其下 `nodes:` 的**列表项行**（`updates:` 头、
// 其注释、`check:` 一类、`nodes:` 行本身**全部逐字保留**；`nodes:` 之后的注释/空行保留、列表项换）。
func fleetRenderRewriteNodes(lines []string, items []string) ([]string, bool) {
	stripped := fleetRenderStrip(lines)
	s, e, ok := cnrSectionRange(stripped, "updates")
	if !ok {
		return nil, false
	}
	nodesAt := -1
	for i := s; i < e; i++ {
		if strings.TrimSpace(lines[i]) == "nodes:" {
			nodesAt = i
			break
		}
	}
	if nodesAt < 0 {
		return nil, false
	}
	out := make([]string, 0, len(lines)+len(items))
	out = append(out, lines[:nodesAt+1]...)
	out = append(out, frSpliceBody(lines[nodesAt+1:e], items)...)
	out = append(out, lines[e:]...)
	return out, true
}

// fleetRenderRender —— 由名册节点重渲整件投影全文：**只换**两段的「数据行」，
// 其余（`models:`/`auth:`/`aliases:` 与两段里的注释行）逐字保留。结构不认 ⇒ 第 2 值报理由。
func fleetRenderRender(lines []string, roster []fleetRenderNode) (string, string) {
	fleetEntries := make([]string, 0, len(roster))
	nodeItems := make([]string, 0, len(roster))
	for _, n := range roster {
		fleetEntries = append(fleetEntries, fleetRenderFleetLine(n))
		nodeItems = append(nodeItems, fleetRenderNodeLine(n))
	}
	out, ok := fleetRenderRewriteFleet(lines, fleetEntries)
	if !ok {
		return "", "投影件里没有顶层 `fleet:` 段（结构不认 ⇒ 不给结论）"
	}
	out, ok = fleetRenderRewriteNodes(out, nodeItems)
	if !ok {
		return "", "投影件里没有 `updates:` 段或其下的 `nodes:`（结构不认 ⇒ 不给结论）"
	}
	return strings.Join(out, "\n"), ""
}

// ─────────────────────────── 命令实现 ───────────────────────────

// cmdFleetRender —— `zerg fleet render`（见件头三档说明）。
func cmdFleetRender(inv *invocation, stdout, stderr io.Writer) int {
	preview := inv.hasFlag("--preview-only") || inv.dryRun
	emit := inv.hasFlag("--emit")
	expect := inv.hasFlag("--expect-no-changes")

	// ── 档位互斥（照同族 `dryRunYesConflict` 的口径：两道确认档自相矛盾 ⇒ 用法错 2）──
	if emit && expect {
		inv.setErr("usage", "emit_expect_conflict", "--emit（真写）与 --expect-no-changes（断言无变化）不许同给")
		fmt.Fprintf(stderr, "%s: `--emit`（真写投影）与 `--expect-no-changes`（断言现读 ⟷ 生成一致）**不许同给** ⇒ 用法错 2\n", progName)
		fmt.Fprintf(stderr, "error.kind=usage · detail=emit_expect_conflict · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if emit && preview {
		inv.setErr("usage", "emit_preview_conflict", "--emit（真写）与 --preview-only（只打印）不许同给")
		fmt.Fprintf(stderr, "%s: `--emit`（真写）与 `--preview-only`（只打印）**不许同给** ⇒ 用法错 2\n", progName)
		fmt.Fprintf(stderr, "error.kind=usage · detail=emit_preview_conflict · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
	}

	// ── 读真源（名册）── 读不到 ⇒ 8（不给结论）
	rosterPath, rerr := fleetRenderRosterPath(inv)
	if rerr != "" {
		inv.setErr("blocked", "context_path_bad", rerr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 读不到真源名册（不给结论 · 退码 8）\n", progName, rerr)
		return exitBlocked
	}
	roster, msg := fleetRenderRoster(rosterPath)
	if msg != "" {
		inv.setErr("blocked", "roster_unreadable", msg)
		fmt.Fprintf(stderr, "%s: 真源名册读不到 / 解析不过（%s）：%s ⇒ 不给结论（退码 8）\n", progName, rosterPath, msg)
		return exitBlocked
	}

	// ── 读投影件 ── 读不到 ⇒ 8（不给结论）
	fleetPath, perr := fleetYAMLPath(inv)
	if perr != "" {
		inv.setErr("blocked", "fleet_path_absent", perr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, perr)
		return exitBlocked
	}
	rawBytes, err := os.ReadFile(fleetPath)
	if err != nil {
		inv.setErr("blocked", "fleet_unreadable", "投影件读不到")
		fmt.Fprintf(stderr, "%s: 投影件读不到（%s）：%v ⇒ 不给结论（退码 8）\n", progName, fleetPath, err)
		return exitBlocked
	}
	raw := string(rawBytes)
	lines := fleetRenderSplitLines(raw)

	// ── 生成投影全文（结构不认 ⇒ 判红 1 · 不写）──
	rendered, oerr := fleetRenderRender(lines, roster)
	if oerr != "" {
		inv.setErr("failed", "projection_shape_unknown", oerr)
		fmt.Fprintf(stderr, "%s: %s（投影件 %s）⇒ 判红（退码 1 · **不写**）\n", progName, oerr, fleetPath)
		return exitFail
	}

	// ── 真写档（D2）：fail-closed —— 缺 `--yes` ⇒ 退 2 只出计划件 ──
	if emit {
		if !inv.yes {
			inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
			guardMarkDecision(inv, guardDecisionApproval)
			fmt.Fprintf(stderr, "%s: `fleet render --emit` 是 D2 档（真写投影件）—— 缺 --yes ⇒ 不执行\n", progName)
			fmt.Fprintf(stderr, "  它会写 : %s 的 `fleet:`/`updates.nodes` 两段数据行（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚）\n", fleetPath)
			fmt.Fprintf(stderr, "先看计划件：%s fleet render --preview-only\n", progName)
			fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
			return exitUsage
		}
		if rc := writeFleetAtomic(fleetPath, raw, rendered, stderr); rc != exitOK {
			inv.setErr("failed", "write_failed", "写投影件失败（已回滚）")
			return rc
		}
		fmt.Fprintf(stderr, "%s: 投影件已写（%s · 只换 `fleet:`/`updates.nodes` 两段数据行 · 注释与 `models:` 逐字保留）· 节点 %d 个\n",
			progName, fleetPath, len(roster))
		changed := true
		inv.changed = &changed // 包封 `meta.changed`：本档**真改了盘** ⇒ true
		if inv.jsonGiven {
			row := map[string]string{
				"action": "emit", "roster": rosterPath, "fleet": fleetPath,
				"nodes": fmt.Sprintf("%d", len(roster)), "drift": "0", "changed": "true",
				"result": "written",
			}
			return listCmd(inv, stdout, stderr, fleetRenderJSONFields, []map[string]string{row})
		}
		fmt.Fprintf(stdout, "已写投影件：%s（由真源 %s 生成 · 节点 %d 个）\n", fleetPath, rosterPath, len(roster))
		return exitOK
	}

	// ── 只读面：断言（可选）＋ 预览（缺省）──
	curFleet, okF := fleetRenderCurrentFleet(lines)
	curNodes, okN := fleetRenderCurrentNodes(lines)
	if !okF || !okN {
		inv.setErr("failed", "projection_shape_unknown", "投影件两段结构不认（fleet: / updates.nodes）")
		fmt.Fprintf(stderr, "%s: 投影件 %s 的两段结构不认（缺 `fleet:` 或 `updates:`/`nodes:`）⇒ 判红（退码 1）\n", progName, fleetPath)
		return exitFail
	}
	drift := fleetRenderDrift(roster, curFleet, curNodes)
	rc := exitOK
	if expect && len(drift) > 0 {
		rc = exitFail
	}
	// 只读两档（`--preview-only` / `--expect-no-changes`）**零写盘** ⇒ 包封 `meta.changed=false`
	// （不改它会被 danger!=nil 的派生规则误报成 true）。
	changed := false
	inv.changed = &changed

	if inv.jsonGiven {
		action := "preview"
		result := "preview"
		if expect {
			action = "check"
			if len(drift) > 0 {
				result = "drift"
			} else {
				result = "consistent"
			}
		}
		row := map[string]string{
			"action": action, "roster": rosterPath, "fleet": fleetPath,
			"nodes": fmt.Sprintf("%d", len(roster)), "drift": fmt.Sprintf("%d", len(drift)),
			"changed": "false", "result": result,
		}
		return listCmd(inv, stdout, stderr, fleetRenderJSONFields, []map[string]string{row})
	}

	// 人面：断言结论（给 `--expect-no-changes` 时）——逐条抄漂移
	if expect {
		if len(drift) > 0 {
			fmt.Fprintf(stderr, "%s: ✗ 现读投影 ⟷ 名册生成 **不一致**（漂移 %d 项 · 退码 1）—— 「两处手抄 ⇒ 漂移」正是本命令要守的那件事：\n", progName, len(drift))
			for _, d := range drift {
				fmt.Fprintf(stderr, "  · %s\n", d)
			}
			fmt.Fprintf(stderr, "  修法：`%s fleet render --preview-only` 对拍 ⇒ 认了再 `%s fleet render --emit --yes`\n", progName, progName)
		} else {
			fmt.Fprintf(stderr, "%s: ✅ 现读投影 ⟷ 名册生成 **一致**（fleet: 与 updates.nodes 逐字段相符 · 退码 0）\n", progName)
		}
	}

	// 预览：只打印**将要写的投影全文**（缺省档 · `--preview-only`）——
	// 与 `| diff - gateway/fleet.yaml` 对拍即「无输出 = 一致」。
	if preview || !expect {
		fmt.Fprintf(stderr, "%s: 投影全文（真源 = %s 的 nodes: · 节点 %d 个 · 零写盘 —— 未改任何状态）\n", progName, rosterPath, len(roster))
		fmt.Fprint(stdout, rendered)
	}
	return rc
}
