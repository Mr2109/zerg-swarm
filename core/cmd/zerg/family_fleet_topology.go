// family_fleet_topology.go —— `zerg fleet topology`：**部署拓扑只读可查面**（缺口 `GAP-20260926-92` 之正解）。
//
// 为什么有这件（账内 symptom 逐字）：
//
//	「无正门可查部署拓扑真源：判『远端自持与否』只能靠 zerg agent ls 观测面 + 静态读
//	 fleet.yaml/setup-zerg.sh，脚本与真源无对拍面」
//
// 本笔之前的现读三条路，逐条都有洞：
//
//	① `zerg agent ls` = 打主控 `GET /api/fleet/status` 的**观测面** —— 只报「此刻在线」的机器
//	  （本笔现读：名册 3 台、只出 Mr2109/edge-node 两台，mini-node-2 整台消失），且只给 code_sha，
//	  **不答**「它是不是自己构建的」（自持与否）与「换件走的哪条通道」；
//	② `gateway/fleet.yaml` 的注释段（「每台机器各自跑源码式更新…不再推预编译制品」）= 人读；
//	③ `scripts/svc/setup-zerg.sh` / `deploy/scripts/zerg-deploy-agent.sh` = 人读**脚本意图**，
//	  与名册/投影**无机械对拍**。
//
// 本件读**四处盘上真源**（零网络 · 零 ssh · 零写盘 · 零子进程）：
//
//	① 名册（真源）：`~/.zerg/contexts/<档>.yaml` 的 `nodes:` —— 复用 `fleetRenderRoster` 的解析原语
//	  （`cnrLoadRoster` 一族 ⇒ 零第二套解析口径）；
//	② 投影：`gateway/fleet.yaml` 的 `fleet:` / `updates.nodes` —— 与①逐字段对拍（复用 `fleetRenderDrift`）；
//	③ 换件回执：`~/.zerg/update_receipts/*.json` 里 `kind=fleet` 的那些（`mode=per-machine-source-update`）
//	  —— 逐机的**源码式自更新**证据（`converged` / `source_remote` / `code_sha_after`）；
//	④ 推送台账：`$ZERG_STATE_DIR/agent_deploy_audit.jsonl`（`zerg agent deploy` 写的）
//	  —— 逐机的**推送换件**证据（`event=verified`）。
//
// 判决口径（「远端自持」= 该机自己从源码更新，不必由主控推预编译制品）：
//
//	· 自持**声明** = 名册该机 `components` 含 `core`（`gateway/fleet.yaml` 段首注释逐字：
//	  「core 让该机将来能自更新，agentd 是它真正在跑的守护进程」）；
//	· 自持**证据** = 该机在最近一份 fleet 回执里 `converged=true`；
//	· **矛盾** = 声明自持、回执也收敛，却在**该次回执之后**又被 `agent deploy` 推送换件
//	  （`verified` 事件更晚）—— 这正是「不再推预编译制品」这句口径与台账对不上的地方，本件把它顶到台面上。
//
// 边界（照实，✗ 不声称办不到的事）：
//
//	· **不 ssh、不读远端盘** —— 本件是「声明 + 回执 + 台账」三面对拍，**不是**远端现况探测。
//	  远端此刻跑的二进制是不是它自己构建的，本件**不判**（那要 `agent ls` 的 code_sha 或 ssh 现读）。
//	· 回执目录 / 推送台账不在盘上 ⇒ 对应列记「未核」，✗ **不当**「从没推过 / 从没自更新过」
//	  （本仓铁律：读不到不许当绿）；名册或投影读不到 ⇒ 退码 `8`（不给结论），与同族 `fleet render` 同一口径。
//	· ✗ 不碰 `gateway/fleet.yaml`、✗ 不碰名册、✗ 不碰台账（本件全程只读：`os.ReadFile` / `os.ReadDir`）。
//
// 本件**尚未登记进 `core/cmd/zerg/main.go` 的命令树**（另路正在改该件）—— 待插入的那一行见件尾注释。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// fleetTopoUsage —— 命令面用法串（登记时逐字搬进 `commands` 那一行的 `usage:`）。
const fleetTopoUsage = "zerg fleet topology [--roster <名册件>] [--path <投影件>] [--root <仓根>] [--context <档名>] [--expect-no-changes] [--json <字段>]"

// fleetTopoJSONFields —— `--json` 字段面（机器面闭集 · 登记时逐字搬进那一行的 `fields:`）。
var fleetTopoJSONFields = []string{
	"machine", "components", "root", "ssh", "self_host", "channel",
	"last_self_update", "self_update_converged", "last_push", "last_push_file",
	"verdict", "drift",
}

// 判决闭集（机器面取值 · 人面另有中文映射，见 `fleetTopoVerdictText`）。
const (
	// fleetTopoSelfHosted —— 声明含 core ＋ 最近回执 converged ＋ 回执后无推送。
	fleetTopoSelfHosted = "self-hosted"
	// fleetTopoSelfHostedUnverified —— 声明含 core，但没有 converged 回执（或回执没跑成）。
	fleetTopoSelfHostedUnverified = "self-hosted-unverified"
	// fleetTopoPushAfterSelfUpdate —— 声明自持且回执 converged，之后又被推送预编译件（口径矛盾）。
	fleetTopoPushAfterSelfUpdate = "push-after-self-update"
	// fleetTopoPushOnly —— 名册未声明 core ⇒ 该机只能靠推送换件（非自持）。
	fleetTopoPushOnly = "push-only"
	// fleetTopoUnknown —— 名册该机没有 `components` 字段（判不了，✗ 不猜）。
	fleetTopoUnknown = "unknown"
)

// fleetTopoChannel 取值（换件通道轴 · 由**声明**给出，证据另列两列）。
const (
	fleetTopoChannelSource = "source-update" // 源码自更新（声明含 core）
	fleetTopoChannelPush   = "artifact-push" // 推送预编译件（未声明 core）
	fleetTopoChannelUnseen = "unobserved"    // 名册没声明 components ⇒ 判不了
)

// ftopoPush —— 推送台账里一台机的一条「已核推送」证据。
type ftopoPush struct {
	At   string
	File string
}

// ftopoReceipt —— fleet 回执里一台机的一条「源码式自更新」证据。
type ftopoReceipt struct {
	At         string
	Converged  bool
	Status     string
	Source     string
	CodeShaNew string
}

// ─────────────────────────── 路径解析（本机物件） ───────────────────────────

// ftopoStateDir —— 状态目录（`ZERG_STATE_DIR` > `~/.zerg/state`）—— 与各门/各面同一口径。
func ftopoStateDir() string {
	if s := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); s != "" {
		return s
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "state")
}

// ftopoReceiptsDir —— 换件回执目录（`~/.zerg/update_receipts` · 与 `zerg-deploy-agent.sh` 的
// `--receipt-dir` 缺省、`fleet` 回执落点同源）。
func ftopoReceiptsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "update_receipts")
}

// ftopoPushLedgerPath —— 推送台账件（`zerg agent deploy` 的审计面 · 同目录名逐字）。
func ftopoPushLedgerPath() string {
	d := ftopoStateDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "agent_deploy_audit.jsonl")
}

// ftopoAtKey —— 时间串归一（`2026-10-12T16:50:08Z` 与 `20261012T165008Z` 可比）：只留字母数字。
func ftopoAtKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ftopoAtLess —— a 早于 b（归一后逐字比 · ✗ 不解析时区：两种形态归一后同构）。
func ftopoAtLess(a, b string) bool {
	ka, kb := ftopoAtKey(a), ftopoAtKey(b)
	if ka == "" || kb == "" {
		return false // 取不到时间 ⇒ 不声称先后（✗ 拿空串当"更早"）
	}
	return ka < kb
}

// ftopoHasCore —— 名册 `components` 里有没有 `core`（逗号分隔 · 去空白 · 大小写不敏感）。
func ftopoHasCore(components string) bool {
	for _, p := range strings.Split(components, ",") {
		if strings.EqualFold(strings.TrimSpace(p), "core") {
			return true
		}
	}
	return false
}

// ─────────────────────────── 读③换件回执 ───────────────────────────

// ftopoFleetReceiptFile —— fleet 回执件的**读入面**（只取本件用到的格 · ✗ 不引整份 schema）。
type ftopoFleetReceiptFile struct {
	Kind  string `json:"kind"`
	At    string `json:"at"`
	Mode  string `json:"mode"`
	Nodes []struct {
		Name         string `json:"name"`
		Status       string `json:"status"`
		Converged    bool   `json:"converged"`
		SourceRemote string `json:"source_remote"`
		CodeShaAfter string `json:"code_sha_after"`
	} `json:"nodes"`
}

// ftopoLoadReceipts —— 逐件读回执目录，取**每台机最近一次**的源码式更新证据。
//
// 返回值三件：证据表 · 读不到的说明（空 ⇒ 读到了）· 跳过的件数说明（只统计不猜）。
// `kind=component` 的回执（本机单机换件）与 `latest.json` 都不算 —— 它们不按机归并（`nodes` 为空）。
func ftopoLoadReceipts(dir string) (map[string]ftopoReceipt, string, int) {
	out := map[string]ftopoReceipt{}
	if strings.TrimSpace(dir) == "" {
		return out, "回执目录解析不了（取不到家目录）", 0
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return out, fmt.Sprintf("回执目录不在 / 读不了：%s", dir), 0
	}
	skipped, bad := 0, 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			bad++
			continue
		}
		var f ftopoFleetReceiptFile
		if jerr := json.Unmarshal(raw, &f); jerr != nil {
			bad++
			continue
		}
		if f.Kind != "fleet" || len(f.Nodes) == 0 {
			skipped++
			continue
		}
		for _, n := range f.Nodes {
			name := strings.TrimSpace(n.Name)
			if name == "" {
				continue
			}
			cur, ok := out[name]
			if ok && !ftopoAtLess(cur.At, f.At) {
				continue // 已有更新的一份
			}
			out[name] = ftopoReceipt{
				At: f.At, Converged: n.Converged, Status: n.Status,
				Source: n.SourceRemote, CodeShaNew: n.CodeShaAfter,
			}
		}
	}
	note := ""
	if bad > 0 {
		note = fmt.Sprintf("回执目录有 %d 件读不了 / 不是 JSON（已跳过 · 未当证据）", bad)
	}
	return out, note, skipped
}

// ─────────────────────────── 读④推送台账 ───────────────────────────

// ftopoLoadPushes —— 逐行读推送台账，取**每台机最近一条 `event=verified`**（= 已核推送换件）。
//
// 台账是追加式 jsonl；`executing`（换件前一行）与 `failed` **不算**证据 ⇒ 只认 `verified`。
// 坏行**计数上报**（✗ 静默跳过：读不到的行也是读不到）。
func ftopoLoadPushes(path string) (map[string]ftopoPush, string, int) {
	out := map[string]ftopoPush{}
	if strings.TrimSpace(path) == "" {
		return out, "推送台账路径解析不了（取不到状态目录）", 0
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Sprintf("推送台账不在 / 读不了：%s", path), 0
	}
	bad := 0
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			At      string `json:"at"`
			Event   string `json:"event"`
			Machine string `json:"machine"`
			File    string `json:"file"`
		}
		if jerr := json.Unmarshal([]byte(line), &row); jerr != nil {
			bad++
			continue
		}
		if row.Event != "verified" {
			continue
		}
		m := strings.TrimSpace(row.Machine)
		if m == "" {
			bad++
			continue
		}
		cur, ok := out[m]
		if ok && !ftopoAtLess(cur.At, row.At) {
			continue
		}
		out[m] = ftopoPush{At: row.At, File: row.File}
	}
	note := ""
	if bad > 0 {
		note = fmt.Sprintf("推送台账有 %d 行读不了 / 缺 machine（已跳过 · 未当证据）", bad)
	}
	return out, note, 0
}

// ─────────────────────────── 判决 ───────────────────────────

// ftopoVerdict —— 单机判决（闭集 · 口径见件头）。
//
// 三轴分开算、一次合成：声明（components）· 证据（回执 converged）· 矛盾（回执之后又被推送）。
func ftopoVerdict(components string, rec ftopoReceipt, haveRec bool, push ftopoPush, havePush bool) string {
	comp := strings.TrimSpace(components)
	if comp == "" {
		return fleetTopoUnknown
	}
	if !ftopoHasCore(comp) {
		return fleetTopoPushOnly
	}
	if !haveRec || !rec.Converged {
		return fleetTopoSelfHostedUnverified
	}
	if havePush && ftopoAtLess(rec.At, push.At) {
		return fleetTopoPushAfterSelfUpdate
	}
	return fleetTopoSelfHosted
}

// ftopoSelfHost —— 声明轴（`--json` 的 self_host 列）：自持能力只看名册声明，不掺证据。
func ftopoSelfHost(components string) string {
	comp := strings.TrimSpace(components)
	if comp == "" {
		return fleetTopoUnknown
	}
	if ftopoHasCore(comp) {
		return fleetTopoSelfHosted
	}
	return fleetTopoPushOnly
}

// ftopoChannel —— 通道轴（由声明给出 —— 实际走了哪条，看 last_self_update / last_push 两列）。
func ftopoChannel(components string) string {
	switch ftopoSelfHost(components) {
	case fleetTopoSelfHosted:
		return fleetTopoChannelSource
	case fleetTopoPushOnly:
		return fleetTopoChannelPush
	default:
		return fleetTopoChannelUnseen
	}
}

// fleetTopoVerdictText —— 判决的人面中文（机器面取值**逐字不译**：`--json` 出英文闭集）。
func fleetTopoVerdictText(v string) string {
	switch v {
	case fleetTopoSelfHosted:
		return "自持"
	case fleetTopoSelfHostedUnverified:
		return "自持（未核）"
	case fleetTopoPushAfterSelfUpdate:
		return "自持但另有推送"
	case fleetTopoPushOnly:
		return "非自持（推送）"
	case fleetTopoUnknown:
		return "未知"
	}
	return v
}

// ftopoLocalName —— 名册里「本机」的机器名（`ssh: "-"` 的那台 · 恰好一台时才算）。
//
// 为什么需要它：fleet 回执的节点名把本机记作 `local`（`zerg-upgrade.sh --fleet` 的 `order` 段），
// 名册里却按机器名记（`Mr2109`）⇒ 不归并就会把本机的自更新证据整条丢掉。
func ftopoLocalName(roster []fleetRenderNode) string {
	local := ""
	n := 0
	for _, r := range roster {
		if strings.TrimSpace(r.node["ssh"]) == "-" {
			local = r.name
			n++
		}
	}
	if n != 1 {
		return "" // 0 台或多台 ⇒ 不猜（✗ 拿第一台顶）
	}
	return local
}

// ftopoReceiptFor —— 取某机的回执证据：先按机器名，再按本机别名 `local` 回落。
func ftopoReceiptFor(name, local string, receipts map[string]ftopoReceipt) (ftopoReceipt, bool) {
	if r, ok := receipts[name]; ok {
		return r, true
	}
	if local != "" && name == local {
		if r, ok := receipts["local"]; ok {
			return r, true
		}
	}
	return ftopoReceipt{}, false
}

// ftopoDriftCount —— 某机在「名册 ⟷ 投影」上的**字段级漂移条数**（两段各自比 · ✗ 不串成一条）。
func ftopoDriftCount(name string, r fleetRenderNode, curFleet map[string]map[string]string, curNodes map[string]map[string]string) int {
	n := 0
	cf, ok := curFleet[name]
	if !ok {
		n++ // 投影 `fleet:` 段整条缺项
	} else {
		for _, f := range fleetRenderFleetFields {
			if cf[f] != r.fleet[f] {
				n++
			}
		}
	}
	cn, ok := curNodes[name]
	if !ok {
		n++ // 投影 `updates.nodes` 整条缺项
	} else {
		for _, f := range fleetRenderNodeFields {
			if cn[f] != r.node[f] {
				n++
			}
		}
	}
	return n
}

// ─────────────────────────── 行装配 ───────────────────────────

// ftopoRows —— 名册逐机一行（序 = 名册名 sorted ⇒ 输出定序 · ✗ 用 map 迭代序）。
func ftopoRows(roster []fleetRenderNode, curFleet map[string]map[string]string, curNodes map[string]map[string]string, pushes map[string]ftopoPush, receipts map[string]ftopoReceipt) []map[string]string {
	localName := ftopoLocalName(roster)
	rows := make([]map[string]string, 0, len(roster))
	for _, r := range roster {
		comp := strings.TrimSpace(r.node["components"])
		rec, haveRec := ftopoReceiptFor(r.name, localName, receipts)
		push, havePush := pushes[r.name]
		verdict := ftopoVerdict(comp, rec, haveRec, push, havePush)

		selfUpdate := "-（无回执：未核）"
		converged := ""
		if haveRec {
			selfUpdate = rec.At
			converged = fmt.Sprintf("%t", rec.Converged)
		}
		lastPush, lastPushFile := "-（无记录：未核）", ""
		if havePush {
			lastPush, lastPushFile = push.At, push.File
		}
		rows = append(rows, map[string]string{
			"machine":               r.name,
			"components":            comp,
			"root":                  strings.TrimSpace(r.node["root"]),
			"ssh":                   strings.TrimSpace(r.node["ssh"]),
			"self_host":             ftopoSelfHost(comp),
			"channel":               ftopoChannel(comp),
			"last_self_update":      selfUpdate,
			"self_update_converged": converged,
			"last_push":             lastPush,
			"last_push_file":        lastPushFile,
			"verdict":               verdict,
			"drift":                 fmt.Sprintf("%d", ftopoDriftCount(r.name, r, curFleet, curNodes)),
		})
	}
	return rows
}

// ftopoCounts —— 人面汇总（判决闭集逐档计数 · 序固定 ⇒ 同一输入同一行字）。
func ftopoCounts(rows []map[string]string) map[string]int {
	c := map[string]int{}
	for _, r := range rows {
		c[r["verdict"]]++
	}
	return c
}

// ftopoFieldOK —— `--json` 字段是否在本条闭集里（未登记进命令树时 `fieldListOf` 为空 ⇒ 自校。
// 登记后两处同源：`fleetTopoJSONFields` 与那一行 `fields:` 逐字相同）。
func ftopoFieldOK(f string) bool {
	for _, x := range fleetTopoJSONFields {
		if x == f {
			return true
		}
	}
	return false
}

// ftopoEmitJSON —— 机器面：六键包封 + 逐机一条（`--json` 的字段投影走 `emitEnvelopeWith` 同一渲染器）。
//
// 为什么不用 `listCmd`：它走 `find(path)` 取命令行，而本件**尚未登记**进命令树 ⇒ `find` 为 nil
// 会让机器面直接退 2。这里**自造那一行的等价物**（path/kind/fields 三格逐字等于待登记的行）⇒
// 登记前后机器面输出逐字同（登记后 `listCmd` 与本函数二选一，输出一致）。
func ftopoEmitJSON(inv *invocation, stdout io.Writer, rows []map[string]string) int {
	cmd := &command{path: inv.path, kind: "FleetTopology", fields: fleetTopoJSONFields}
	objs := make([]string, 0, len(rows))
	for _, row := range rows {
		objs = append(objs, marshalObjectRegistered(inv.fields, row))
	}
	emitEnvelopeWith(stdout, cmd, "["+strings.Join(objs, ",")+"]", len(rows), inv)
	return exitOK
}

// ─────────────────────────── 入口 ───────────────────────────

// cmdFleetTopology —— `zerg fleet topology`（只读 · 零写 · 零网络 · 零子进程）。
//
// 退码：
//
//	0 —— 读到了名册与投影，判决已出；
//	1 —— 投影件结构不认（`fleet:` / `updates.nodes` 缺段）或给了 `--expect-no-changes` 而确有漂移；
//	2 —— 用法错（`--json` 不给字段 / 给了未知字段）；
//	8 —— 名册或投影读不到（**不给结论** · 与 `fleet render` 同口径：「读不到」不许当绿）。
func cmdFleetTopology(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
		for _, f := range inv.fields {
			if ftopoFieldOK(f) {
				continue
			}
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			fmt.Fprintf(stderr, "%s: 未知字段 %q\n", progName, f)
			fmt.Fprintf(stderr, "合法字段: %s\n", strings.Join(fleetTopoJSONFields, ","))
			return exitUsage
		}
	}

	// ── ① 名册（真源）── 读不到 ⇒ 8（不给结论）
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

	// ── ② 投影 ── 读不到 ⇒ 8；结构不认 ⇒ 1（与 `fleet render` 逐条同口径）
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
	lines := fleetRenderSplitLines(string(rawBytes))
	curFleet, okF := fleetRenderCurrentFleet(lines)
	curNodes, okN := fleetRenderCurrentNodes(lines)
	if !okF || !okN {
		inv.setErr("failed", "projection_shape_unknown", "投影件两段结构不认（fleet: / updates.nodes）")
		fmt.Fprintf(stderr, "%s: 投影件 %s 的两段结构不认（缺 `fleet:` 或 `updates:`/`nodes:`）⇒ 判红（退码 1）\n", progName, fleetPath)
		return exitFail
	}
	curNodesByName := map[string]map[string]string{}
	for _, r := range curNodes {
		curNodesByName[r["name"]] = r
	}

	// ── ③④ 证据面（软读数：读不到 ⇒ 记「未核」，✗ 不当「没有」）
	receiptsDir := ftopoReceiptsDir()
	receipts, recNote, skipped := ftopoLoadReceipts(receiptsDir)
	ledger := ftopoPushLedgerPath()
	pushes, pushNote, _ := ftopoLoadPushes(ledger)

	rows := ftopoRows(roster, curFleet, curNodesByName, pushes, receipts)
	drift := fleetRenderDrift(roster, curFleet, curNodes)
	expect := inv.hasFlag("--expect-no-changes")

	// 只读面 ⇒ 包封 `meta.changed=false`（✗ 不按幂等档派生）
	changed := false
	inv.changed = &changed

	// 机器面
	if inv.jsonGiven {
		return ftopoEmitJSON(inv, stdout, rows)
	}

	// 人面：先台账面（stderr），再逐机表（stdout），最后汇总 / 漂移 / 说明（stderr）
	fmt.Fprintf(stderr, "%s: 部署拓扑（只读 · 零网络 · 零写盘）· 名册 %s · 投影 %s\n", progName, rosterPath, fleetPath)
	fmt.Fprintf(stderr, "  回执 %s · 推送台账 %s\n", receiptsDir, ledger)
	header := []string{"machine", "components", "自持", "通道", "最近自更新", "最近推送", "判决", "漂移"}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{
			r["machine"], r["components"],
			ftopoDeclText(r["self_host"]), ftopoChannelText(r["channel"]),
			r["last_self_update"], r["last_push"],
			fleetTopoVerdictText(r["verdict"]), r["drift"],
		})
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, header, table)

	c := ftopoCounts(rows)
	fmt.Fprintf(stderr, "\n%s: 合计 %d 台 · 自持 %d · 自持（未核）%d · 非自持（推送）%d · 自持但另有推送 %d · 未知 %d · 名册⟷投影漂移 %d 项\n",
		progName, len(rows),
		c[fleetTopoSelfHosted], c[fleetTopoSelfHostedUnverified], c[fleetTopoPushOnly],
		c[fleetTopoPushAfterSelfUpdate], c[fleetTopoUnknown], len(drift))
	fmt.Fprintf(stderr, "%s: 口径：自持 = 名册 `components` 含 `core`（能自己从源码更新）＋ 最近一份 fleet 回执 converged；\n", progName)
	fmt.Fprintf(stderr, "  推送 = `agent deploy` 台账里的 `verified` 事件（推预编译制品换件）· 「未核」= 台账/回执不在盘上，✗ 不当「没有」\n")

	if recNote != "" {
		fmt.Fprintf(stderr, "%s: ⚠ %s\n", progName, recNote)
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "%s: 提示：回执目录另有 %d 件非 fleet 回执（单机 component 回执 / latest.json）不按机归并，未计\n", progName, skipped)
	}
	if pushNote != "" {
		fmt.Fprintf(stderr, "%s: ⚠ %s ⇒ `最近推送` 列按「未核」记，✗ **不**当「从没推过」\n", progName, pushNote)
	}

	rc := exitOK
	if expect {
		if len(drift) > 0 {
			rc = exitFail
			fmt.Fprintf(stderr, "%s: ✗ `--expect-no-changes`：名册 ⟷ 投影 **不一致**（%d 项 · 退码 1）：\n", progName, len(drift))
			for _, d := range drift {
				fmt.Fprintf(stderr, "  · %s\n", d)
			}
			fmt.Fprintf(stderr, "  修法：`%s fleet render --preview-only` 对拍 ⇒ 认了再 `%s fleet render --emit --yes`\n", progName, progName)
		} else {
			fmt.Fprintf(stderr, "%s: ✅ `--expect-no-changes`：名册 ⟷ 投影 逐字段相符（fleet: 与 updates.nodes 两段）\n", progName)
		}
	} else if len(drift) > 0 {
		fmt.Fprintf(stderr, "%s: ⚠ 名册 ⟷ 投影有 %d 项漂移（逐项：`%s fleet topology --expect-no-changes`）\n", progName, len(drift), progName)
	}
	return rc
}

// ftopoDeclText —— 声明轴人面（`--json` 出闭集英文，人面出中文）。
func ftopoDeclText(v string) string {
	switch v {
	case fleetTopoSelfHosted:
		return "自持"
	case fleetTopoPushOnly:
		return "非自持"
	case fleetTopoUnknown:
		return "未知"
	}
	return v
}

// ftopoChannelText —— 通道轴人面。
func ftopoChannelText(v string) string {
	switch v {
	case fleetTopoChannelSource:
		return "源码自更新"
	case fleetTopoChannelPush:
		return "推送换件"
	case fleetTopoChannelUnseen:
		return "未声明"
	}
	return v
}

// ══════════════════════════ 待注册行（✗ 本笔不写进 main.go） ══════════════════════════
//
// 插入点：`core/cmd/zerg/main.go` 里 `commands = []*command{ … }` 中，
// 紧接 `fleet render` 那一行（`path: []string{"fleet","render"}, … run: cmdFleetRender,`）的**闭合 `},` 之后**、
// `// ---- L1 批3（2026-10-08）：`zerg mcp` 真动作分派 …` 那段注释之前。
//
// 待插入的原文（逐字 —— `usage:` / `fields:` 用**字面量**：契约侧 `FIELDS_RE`/`USAGE_RE` 只认字面量，
// 抽成变量会把这一条判成「没有字段表」；两处与本件的 `fleetTopoUsage` / `fleetTopoJSONFields` 逐字相同，
// 由 `TestFleetTopologyRegisteredRowMatches` 在登记后自动对拍）：
//
//	{
//		path:     []string{"fleet", "topology"},
//		kind:     "FleetTopology",
//		layer:    "host",
//		summary:  "部署拓扑只读面（名册真源 ⟷ 投影 对拍 + 逐机判「远端自持与否」：自持=components 含 core ＋ fleet 回执 converged；推送=agent deploy 台账 verified 事件）—— 零网络/零写盘，✗ 不替代 `agent ls` 观测面",
//		usage:    "zerg fleet topology [--roster <名册件>] [--path <投影件>] [--context <档名>] [--expect-no-changes] [--json <字段>]",
//		arity:    "none",
//		args:     []string{"（无位置参数）"},
//		fields:   []string{"machine", "components", "root", "ssh", "self_host", "channel", "last_self_update", "self_update_converged", "last_push", "last_push_file", "verdict", "drift"},
//		endpoint: "",
//		run:      cmdFleetTopology,
//	},
//
// ★ 插入点现读（2026-10-23 · 该件正被另路改，行号会漂 ⇒ 以**锚串**为准）：
// 第 4893 行 `run:      cmdFleetRender,` 的闭合 `},`（现读 4894 行）之后、4895 行
// `// ---- L1 批3（2026-10-08）：`zerg mcp` 真动作分派 …` 之前。
//
// 登记后随之要动的三处（本笔**不碰**，交给注册那一路）：
//
//	· 门73 用法串⟷名字表：用法串已含 `fleet topology` 字面（判据 `zerg code find 'fleet topology' --path core/cmd/zerg` 现读即命中本件）；
//	· 门78 新命令随动总账：新 path `fleet topology` 须登记（十一个面）；
//	· `cmd/zerg/testdata/cli-matrix.json`：按矩阵口径补该条的候选口令（未登记前不补 —— 否则矩阵会跑出一条打不着的命令）。
