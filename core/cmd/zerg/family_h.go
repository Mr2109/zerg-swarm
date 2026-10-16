// family_h.go —— H 族四条：`gateway` / `api` / `resource` / `script`（§三 H 族 · §7.1 `P10`/`P7`）。
//
// 本件的四条口径（照定稿，不自造）：
//
//	① **`zerg api call` 暂不开**（§7.1 `P10` 定案）：通用逃生门绕过全部命令面校验 ⇒
//	   本族只做 `ls` / `openapi` / `help`；要开走提案 + 人批。
//	② **对话面 19 条不各自进 CLI**（§7.1 `P7`：HTTP 17 + UI 2）：等外部 agent 真在调 `/api/chat/*`
//	   再说（`U16` 未证实）⇒ **本族（`api` 族）**一条对话子命令都不登记。
//	   ★ 2026-10-17 新令（`P-1` 乙案）：全族口径已改 —— **人类正门 = `zerg chat`**（**另立新族** · 只经正门 ·
//	   ✗ 不开通用转发）⇒ 本族仍一条不登记；正门**不属本族**。
//	③ **写面两枚走同一个执行门**（`resource pin|unpin` · `gateway breakers --reset`）：
//	   `--dry-run` 出计划件（零副作用）· 缺 `--yes` ⇒ `2`（fail-closed）· 齐了才发请求；
//	   退码与错误码照 §九 M7/§十二 `P-013` 的表走（不透传主控的 5xx 当结论）。
//	④ 只读面一律**投影真端点**（`/api/resources/*` · `/api/gateway/breakers`），字段名逐字取载荷键。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---- 只读：资源面 -------------------------------------------------------------------------

// cmdResourceLs —— `zerg resource ls [<类型>]`（只读）。
// 给了类型 ⇒ 打 `/api/resources/{type}`；没给 ⇒ 打 `/api/resources/ledger`（账本全表）。
//
// ★ 2026-10-14（缺口 `GAP-20261014-47` · 落点 = 修 `resource ls 工具` 的 400 · 不开窗 · 本支**只增**）：
//
//	`工具` 是**唯一**一支**本地投影** —— 它读 CA 工具台账 `<仓根>/tools/versions.json`，**不走主控**。
//	为什么：主控 `GET /api/resources/{type}` 只认四个**英文**类型（`models|tools|skills|mcp`），
//	给中文标签 ⇒ `/api/resources/工具` 当场退 **400 `UNKNOWN_RESOURCE_TYPE`**（改前症状逐字 =
//	「主控 /api/resources/工具 返回 400」）；而「虫族有哪些工具」的真源是**仓内台账**
//	（`tools/versions.json` · 现读 141 键 · 出处 `AGENTS.md:56` ／ `tools/cli-tools.json:4`），
//	主控那份 `tools` 面是 agent 运行时工具（现读 142 条 · 与台账**不同集**）⇒ 本支直读台账。
//	其余类型（含英文 `tools`）与无参形态**一字不动** = 只增不改。
func cmdResourceLs(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 && inv.args[0] == "工具" {
		return cmdResourceToolsLedger(inv, stdout, stderr)
	}
	path := "/api/resources/ledger"
	if len(inv.args) > 0 {
		path = "/api/resources/" + inv.args[0]
	}
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, path, &resp, stderr); rc != exitOK {
		return rc
	}
	rows, display := flattenPayload(resp)
	return listCmd(inv, stdout, stderr, display, rows)
}

// cmdResourceToolsLedger —— `zerg resource ls 工具`（只读 · **本地**投影 CA 工具台账）。
//
// 真源 = `<仓根>/tools/versions.json`（`{"<工具名>":"<版本>"}` · 现读 141 键 · 出处 `AGENTS.md:56`
// 与 `tools/cli-tools.json:4` 逐字点名它，且后者明文「**不是** CLI 依赖账」）。本命令**只列台账**、
// 不给任何执行口（缺口 `GAP-20261014-47` 的「执行不归 CLI」明文）。
//
// 三态（照本族既有口径 · 与 `cmdScriptLs` 同一条）：
//
//	① 台账在盘且非空 ⇒ 逐条列出（人类面 TSV `name`/`version`；`--json` 走 `listCmd` 同一出口）；
//	② 台账在盘但**为空**（`{}`）⇒ **正确空输出**（表头 + 0 行 · rc=0）—— 「一个工具都没有」是**合法结果**，
//	   ✗ 不是错（缺口 `GAP-20261014-47` 的反例要求）；
//	③ 台账**不在盘** / 不是合法 JSON ⇒ **不给结论**（rc=8 fail-closed）—— 拿「读不到」当「没有」= 假绿，
//	   本族一贯不许（同 `cmdScriptLs` 的 `scripts_absent`）。
func cmdResourceToolsLedger(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 列不出工具台账（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	ledger := filepath.Join(root, "tools", "versions.json")
	b, err := os.ReadFile(ledger)
	if err != nil {
		inv.setErr("blocked", "tool_ledger_absent", "读不到 CA 工具台账")
		fmt.Fprintf(stderr, "%s: 读不到 CA 工具台账 %s（%v）⇒ 不给结论（退码 8）\n", progName, ledger, err)
		return exitBlocked
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		inv.setErr("blocked", "tool_ledger_unparsable", "CA 工具台账不是合法 JSON")
		fmt.Fprintf(stderr, "%s: CA 工具台账 %s 不是合法 JSON（%v）⇒ 不给结论（退码 8）\n", progName, ledger, err)
		return exitBlocked
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names) // 行序稳定（字典序 · 便于人眼与 `--json` 逐条对拍）
	rows := []map[string]string{}
	for _, n := range names {
		rows = append(rows, map[string]string{"name": n, "version": cell(m[n])})
	}
	// 读到几条就说几条（现读读数 · ✗ 不写死）：人眼要的是「台账到底几键」。
	fmt.Fprintf(stderr, "%s: 工具台账 = %s（%d 键 · 现读 · 真源 tools/versions.json）\n", progName, ledger, len(rows))
	return listCmd(inv, stdout, stderr, []string{"name", "version"}, rows)
}

// cmdResourceLedger —— `zerg resource ledger`（只读 · 与 `resource ls` 同源，只是名字固定）。
func cmdResourceLedger(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/resources/ledger", &resp, stderr); rc != exitOK {
		return rc
	}
	rows, display := flattenPayload(resp)
	return listCmd(inv, stdout, stderr, display, rows)
}

// flattenPayload 把一份载荷摊成行：顶层数组 ⇒ 逐条；对象 ⇒ 逐个键值；
// 再不行就当单条对象（**字段名逐字取载荷键**，不另造词汇）。
func flattenPayload(resp jsonObj) ([]map[string]string, []string) {
	for _, k := range []string{"items", "resources", "ledger", "entries", "list", "machines", "registered_models"} {
		if l := asList(resp[k]); len(l) > 0 {
			return rowsOf(l), keysOfRow(asObj(l[0]))
		}
	}
	if len(resp) > 0 {
		// 全是标量 ⇒ 一行一对；有嵌套 ⇒ 当一个对象
		nested := false
		for _, v := range resp {
			if asObj(v) != nil || asList(v) != nil {
				nested = true
			}
		}
		if nested {
			row := map[string]string{}
			keys := []string{}
			for k, v := range resp {
				keys = append(keys, k)
				row[k] = cell(v)
			}
			sort.Strings(keys)
			if len(keys) > 8 {
				keys = keys[:8]
			}
			return []map[string]string{row}, keys
		}
		row := map[string]string{}
		keys := []string{}
		for k, v := range resp {
			keys = append(keys, k)
			row[k] = cell(v)
		}
		sort.Strings(keys)
		return []map[string]string{row}, keys
	}
	return nil, []string{"（空）"}
}

func rowsOf(l []any) []map[string]string {
	out := []map[string]string{}
	for _, it := range l {
		o := asObj(it)
		if o == nil {
			// 标量数组（`registered_models: ["a","b"]` 一类）：**不丢**，包成一格
			out = append(out, map[string]string{"value": cell(it)})
			continue
		}
		row := map[string]string{}
		for k, v := range o {
			row[k] = cell(v)
		}
		out = append(out, row)
	}
	return out
}

func keysOfRow(o jsonObj) []string {
	keys := []string{}
	for k := range o {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = keys[:8]
	}
	return keys
}

// ---- 只读：网关面 -------------------------------------------------------------------------

// cmdGatewayModels —— `zerg gateway models`（只读）：网关侧看得见的模型面。
//
// 口径（如实说）：主控面把「模型」放在 `/api/fleet/models`，网关侧没有独立的模型端点
// ⇒ 本命令投影**同一份**载荷并按 `backend` 排序，且在 stderr 写明「它与 `model ls` 同源」，
// **不**假装网关有第二份模型表。
func cmdGatewayModels(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &resp, stderr); rc != exitOK {
		return rc
	}
	fmt.Fprintf(stderr, "%s: 与 `model ls` **同源**（`/api/fleet/models`）—— 网关侧没有第二份模型表\n", progName)
	fields := []string{"id", "host", "backend", "modality", "mem_gb", "file"}
	rows := []map[string]string{}
	for _, it := range asList(resp["models"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, fields))
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "host", "backend", "modality", "mem_gb"}, rows)
}

// ---- 只读：脚本面（最小面；件数以 `zerg script ls --json path` 现读为准 · 「现状标注」在 T-50 落）----

// cmdScriptLs —— `zerg script ls`（只读本机面）：`scripts/` 下的脚本逐件列出。
//
// ★ 2026-09-24（缺口 `Q-160` · 口径与判据件不同源）：收件面换成 `scriptInvScan`
// （见 `family_script_inventory.go:103`）**同一处**，本命令不再自带第二套扫描。
// 旧写法（`scanRoots`）只收 `.sh` / `.py` 且只下钻一层 ⇒ 比统一口径的 `scriptInvScan`
// **少 3 件无后缀可执行件** ⇒ 拿本命令的数去核台账会得假绿 ✗（`Q-160` 后收件口径已统一）。
// ★ **件数一律现读 · 禁手写**（写死的时点读数会烂）：本命令侧 = `zerg script ls --json path`
//
//	的 `items` 长度；判据件侧（`TestScriptInventoryMatchesLiveScan`）走同一处 `scriptInvScan`；
//	**仓外台账**侧 = `<Zerg-内部文档>/项目文档/v2.5.10/清单-脚本现状-20260920.tsv` 的数据行数 ——
//	三处现读已**同集**（旧文「三者同数不同集」的时点读数**已不成立** ✓）。
//
// 只换**收件口径**：字段面（`path` / `public`）与行序**都不动** ✓。
func cmdScriptLs(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 列不出脚本面（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	public := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv")); err == nil {
		for i, line := range strings.Split(string(b), "\n") {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue
			}
			f := strings.Split(line, "\t")
			if len(f) >= 2 {
				public[strings.TrimSpace(f[0])] = strings.TrimSpace(f[1])
			}
		}
	}
	rows := []map[string]string{}
	// 收件口径 = 判据件那一条（`scriptInvScan`）：`.sh` / `.py` / 无后缀且可执行且首行 `#!`，
	// 排除面同 `scriptInvExcludeDirs`（扫不动 ⇒ 走下面那条「没扫到东西」的 BLOCKED，不猜）。
	if scanned, err := scriptInvScan(root); err == nil {
		sort.Slice(scanned, func(i, j int) bool {
			return scriptLsOrderKey(scanned[i].Path) < scriptLsOrderKey(scanned[j].Path)
		})
		for _, r := range scanned {
			rows = append(rows, map[string]string{
				"path":        r.Path,
				"public":      "",
				"kind":        scriptLsKind(r.Kind),
				"count_basis": scriptLsCountBasis(),
				"refs":        r.Refs,
			})
		}
	}
	if len(rows) == 0 {
		inv.setErr("blocked", "scripts_absent", "scripts/ 下没扫到可列的东西")
		fmt.Fprintf(stderr, "%s: `scripts/` 没扫到东西 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	for _, r := range rows {
		if p, ok := public[r["path"]]; ok {
			r["public"] = p
		} else {
			r["public"] = "（未登记）"
		}
	}
	return listCmd(inv, stdout, stderr, []string{"path", "public", "kind", "count_basis", "refs"}, rows)
}

// scriptLsKind —— 命令面那一格**量词面** `kind` 的取值（`O-16` · 缺口 `Q-162`）。
//
// 为什么要有这一格：`Q-160` 的**收件口径**已修（件数**现读 · 禁手写**，取法 = 现跑 `zerg
// script ls --json path` 数 `items`），但「那个数是**哪一个**数」
// （含不含无后缀可执行件）**机器面无一格可读** ⇒ 下一位仍会拿它去核台账（「同数不同集」的病根
// 形态）。补这一格 = 把**这一次的判据**变成**长期判据**。
//
// 取值 = 台账第 2 列那一个闭集（`scriptInvKindOK`：`sh` / `py` / `无后缀`）的**命令面短名**
// （与 `public` 列在命令面写 `yes`/`no` 同一惯例）；照实三档，**认不出 ⇒ 原串照给（不猜）**：
//
//	`sh` → `sh` · `py` → `py` · `无后缀`（无后缀 + 可执行 + 首行 `#!`）→ `none`
func scriptLsKind(k string) string {
	switch k {
	case "sh", "py":
		return k
	case "无后缀":
		return "none"
	}
	return k
}

// scriptLsCountBasis —— `script ls` 的**量词面**（2026-09-24 · `待拍清单终版-20260924.md` 条 11 ·
// `序 147` · 设计 `v1.2` §5 `O-16` · 缺口 `Q-162`）：「**本跑按哪个口径数 / 扫的哪些根 / 含不含无后缀
// 可执行件**」三格同出一串。
//
// 为什么要有这一格：`Q-160` 的**收件口径**已修（同一处 `scriptInvScan`）、`kind` 那一格也已补，但
// 「**件数**是哪一个件数」仍只有**数**没有**口径** ⇒ 下一位仍会拿它去核台账（「同数不同集」的病根形态）。
// 三格取值：① 口径 = 收件函数名（唯一那一处）；② 根 = `<仓根>/scripts/`（排除面 13 项）；③ 含无后缀 =
// **读 `scriptInvTakesSuffixless` 那一枚常量**（与收件同源 ⇒ 改口径这一格随动，不再静默）。
func scriptLsCountBasis() string {
	suffixless := "否"
	if scriptInvTakesSuffixless {
		suffixless = "是（可执行 + 首行 #!）"
	}
	return fmt.Sprintf("收件口径=scriptInvScan（.sh / .bash / .py / 无后缀且可执行且首行 #!）· 扫的根=scripts/ · deploy/ · gateway/（排除面 13 项）· 含无后缀=%s", suffixless)
}

// scriptLsOrderKey —— `script ls` 的**行序**（与旧输出逐字同序，换口径时不许顺手挪行）：
// 顶层件排在最前，之后按**一层子目录名**成组、组内按整路径排（更深层不另开组，只跟着它
// 那一层的一级目录走）。口径与行序是两件事：`Q-160` 只换口径 ✓。
func scriptLsOrderKey(rel string) string {
	sep := string(filepath.Separator)
	i := strings.Index(rel, sep)
	if i < 0 {
		return "\x00" + rel
	}
	rest := rel[i+len(sep):]
	j := strings.Index(rest, sep)
	if j < 0 {
		return "\x00" + rel // 顶层件（`scripts/<件名>`）
	}
	return rel[:i+len(sep)+j] + "\x00" + rel // 组名 = `scripts/<一级子目录>`
}

// ---- 写面两枚（同一个执行门）--------------------------------------------------------------

// hazardWrite —— 一枚「有影响但可逆」的写动作的**逐字形状**（不新造语义：D2 = `--yes`）。
type hazardWrite struct {
	Command string // `resource pin` 一类
	Method  string
	Path    string // 端点（路径里的 id 由位置参数拼）
	Effect  string // 它会动什么
	Source  string // 出处
}

// hazardWrites —— 本族的两枚写面（**登记在这里 = 只有这两条能写**）。
func hazardWrites() []hazardWrite {
	return []hazardWrite{
		{"resource pin", http.MethodPost, "/api/resources/pin",
			"把该资源标成不可回收（回收候选里会被排除）", "§7.1 P10 · §三 H 族 · 开工单 T-46"},
		{"resource unpin", http.MethodPost, "/api/resources/unpin",
			"取消不可回收标记（它会重新进入回收候选）", "§7.1 P10 · §三 H 族 · 开工单 T-46"},
		{"gateway breakers", http.MethodPost, "/api/gateway/breakers/reset",
			"重置网关熔断器（会立刻重新放流量进去）", "§7.1 P10 · §三 H 族 · 开工单 T-46"},
	}
}

// cmdHazardWrite —— 两枚写面的**唯一执行门**（三态：计划件 / 缺 --yes 拒 / 齐了才发）。
func cmdHazardWrite(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	name := strings.Join(inv.path, " ")
	var spec *hazardWrite
	for i := range hazardWrites() {
		if hazardWrites()[i].Command == name {
			spec = &hazardWrites()[i]
			break
		}
	}
	if spec == nil {
		inv.setErr("usage", "unknown_hazard_write", "本门只服务登记过的写面")
		fmt.Fprintf(stderr, "%s: 本门不服务 %q（登记面见 family_h.go 的 hazardWrites）\n", progName, name)
		return exitUsage
	}
	target := ""
	if len(inv.args) > 0 {
		target = inv.args[0]
	}
	body := map[string]any{}
	if target != "" {
		body["id"] = target
	}
	if name == "gateway breakers" {
		body["reset"] = inv.reset
	}
	payloadBytes, _ := json.Marshal(body)
	payload := string(payloadBytes)

	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s %s\n", progName, name)
		fmt.Fprintf(stdout, "  危险档   : D2（有影响但可逆 · 要 --yes）\n")
		fmt.Fprintf(stdout, "  请求     : %s %s\n", spec.Method, newClient().base+spec.Path)
		fmt.Fprintf(stdout, "  请求体   : %s\n", payload)
		fmt.Fprintf(stdout, "  它会动   : %s\n", spec.Effect)
		fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
		fmt.Fprintf(stdout, "  来源     : %s\n", spec.Source)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `%s` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName, name)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, name)
		return exitUsage
	}
	status, respBody, err := sendJSON(newClient(), spec.Method, spec.Path, payload)
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		status, respBody, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		inv.setErr("usage", code, "主控拒绝了这次写动作")
		fmt.Fprintf(stderr, "%s: 主控拒绝（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		return exitUsage
	}
	row := map[string]string{"command": name, "status": "ok", "request": payload,
		"result": firstLine(strings.TrimSpace(respBody))}
	return listCmd(inv, stdout, stderr, []string{"command", "status", "request", "result"},
		[]map[string]string{row})
}

// familyHCodeReadingHints —— 本件（family_h.go）自身注释里的绝对读数 → 真源取法（缺口 `GAP-20260928-30`）。
//
// 为什么它在 family_h.go 而不并进 family_code.go：缺口逐字点名「同形病在 `family_h.go` …
// 已连烂 3 轮」—— 本件注释里的读数（「排除面 13 项」「写面…枚」一类）若烂了，门看不见注释 ⇒
// 静默腐。把**本件自己**这些读数的真源取法登记在**本件**（就近维护、改了读数就能顺手改取法），
// 由 `zerg code audit-readings` 读出去（与 family_code.go 的通用提示表合并 · 见 `codeAuditReadingHints`）。
func familyHCodeReadingHints() []codeReadingHint {
	return []codeReadingHint{
		{"排除面", "现读 `family_script_inventory.go` 的 `scriptInvExcludeDirs` 键数（本件『排除面 13 项』须与它相符）"},
		{"写面", "现读 `family_h.go` 的 `hazardWrites()` 条数"},
		{"无后缀", "现读 `family_script_inventory.go` 的 `scriptInvTakesSuffixless` 常量"},
	}
}
