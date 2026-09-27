// readonly.go —— 批 A · S2「只读面」8 条的实现（§6.2 最小可验证集）。
//
// 四条纪律：
//
//	① **全程零写操作**：这里只 GET，不 POST/PUT/DELETE，不写任何文件、不改任何状态（§6.2 ✗）。
//	② **字段名逐字取自它投影的端点载荷**：不另造第二套词汇（§4.3 U2「`--json` 字段名与
//	   `/api/capabilities` 的字段同源」）⇒ 载荷里的键名就是 `--json` 的字段名。
//	③ **不打第二枪**：任何失败只报一次、不换端点、不重试（§九 M20 `O2`/`O6`）。
//	④ **非 TTY 零提示词**：永不提问（§4.1 K6）。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// ---- 载荷的通用取值（JSON → 字符串），字段名一个不改 ----

type jsonObj = map[string]any

func asObj(v any) jsonObj {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

// cell 把载荷里的一个值摊成一个单元格文本（数组用 `;` 连接；不做任何单位换算）。
func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			parts = append(parts, cell(x))
		}
		return strings.Join(parts, ";")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// project 把一条载荷对象投影成「字段名 → 值文本」；字段名逐字取自载荷的键。
func project(o jsonObj, fields []string) map[string]string {
	row := make(map[string]string, len(fields))
	for _, f := range fields {
		row[f] = cell(o[f])
	}
	return row
}

// fetch 取一个端点；失败按 §九 M7 分类成 kind 并给对应退码（不打第二枪 · O2/O6）。
func fetch(c *client, path string, out any, stderr io.Writer) int {
	return fetchInv(nil, c, path, out, stderr)
}

// fetchInv 同上，但把 kind 记进 invocation（`--json` 失败时它会进 `error` 块）。
func fetchInv(inv *invocation, c *client, path string, out any, stderr io.Writer) int {
	err := c.getJSON(path, out)
	if err == nil {
		return exitOK
	}
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
		kind = "upstream_error"
		detail = fmt.Sprintf("http_%d", he.status)
	}
	if inv != nil {
		inv.setErr(kind, detail, err.Error())
	}
	fmt.Fprintf(stderr, "%s: %v\n", progName, err)
	fmt.Fprintf(stderr, "error.kind=%s · retryable=%t · remedy=%s（§九 M7：重试判定只读 kind）\n",
		kind, retryableOf(kind), remedyOf(kind))
	return codeOfKind(kind)
}

// listCmd 是四条「清单型」只读命令的公共骨架（api ls · agent ls · task ls · model ls）。
// render 负责把载荷摊成「字段名 → 值文本」的若干行；本函数只管三态渲染与退码。
func listCmd(inv *invocation, stdout, stderr io.Writer, display []string, rows []map[string]string) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc // K2：给了 --json 但不给字段 ⇒ 退码取自退码表（归一后 = 用法错 2）+ stdout 0 字节
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(display))
		for _, f := range display {
			line = append(line, r[f])
		}
		table = append(table, line)
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, display, table)
	return exitOK
}

// ---- api 族（只读三面：ls · openapi · help）----

func cmdAPILs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/capabilities", &resp, stderr); rc != exitOK {
		return rc
	}
	rows := []map[string]string{}
	for _, it := range asList(resp["capabilities"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, []string{"name", "endpoint", "desc", "example"}))
		}
	}
	return listCmd(inv, stdout, stderr, []string{"name", "endpoint", "desc"}, rows)
}

func cmdAPIOpenAPI(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/openapi.json", &resp, stderr); rc != exitOK {
		return rc
	}
	paths := asObj(resp["paths"])
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	rows := []map[string]string{}
	for _, p := range names {
		for _, m := range []string{"get", "post", "put", "patch", "delete"} {
			op := asObj(asObj(paths[p])[m])
			if op == nil {
				continue
			}
			rows = append(rows, map[string]string{
				"path":    p,
				"method":  strings.ToUpper(m),
				"summary": cell(op["summary"]),
			})
		}
	}
	return listCmd(inv, stdout, stderr, []string{"path", "method", "summary"}, rows)
}

func cmdAPIHelp(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		fmt.Fprintf(stderr, "%s: `api help` 不吃位置参数（给的是 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	fmt.Fprint(stdout, `api 族 —— HTTP 能力面的只读投影（批 A）

  zerg api ls        能力清单（投影主控 /api/capabilities；字段名与它逐字同源）
  zerg api openapi   路径表（投影主控 /api/openapi.json 的 paths）
  zerg api help      本说明

不做的：`+"`api call`"+`（通用逃生门）**暂不开** —— 它绕过全部命令面校验，风险最高，
等命令面稳定再放（§7.1 P10 的定案）。批 1 期间本族**零写操作**。
`)
	return exitOK
}

// ---- agent 族（只读：ls）----

func cmdAgentLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/status", &resp, stderr); rc != exitOK {
		return rc
	}
	ms := asObj(resp["machines"])
	names := make([]string, 0, len(ms))
	for n := range ms {
		names = append(names, n)
	}
	sort.Strings(names)
	fields := []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct",
		"mem_available_gb", "mem_total_gb", "models", "last_seen"}
	rows := []map[string]string{}
	for _, n := range names {
		o := asObj(ms[n])
		if o == nil {
			continue
		}
		rows = append(rows, project(o, fields))
	}
	return listCmd(inv, stdout, stderr, []string{"machine", "healthy", "code_version", "code_sha",
		"cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb"}, rows)
}

// ---- task 族（只读：ls）----

func cmdTaskLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/tasks", &resp, stderr); rc != exitOK {
		return rc
	}
	fields := []string{"id", "status", "model", "machine", "priority", "created_at", "description"}
	rows := []map[string]string{}
	for _, it := range asList(resp["tasks"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, fields))
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "status", "model", "machine", "priority", "created_at"}, rows)
}

// ---- model 族（只读：ls）----

func cmdModelLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &resp, stderr); rc != exitOK {
		return rc
	}
	fields := []string{"id", "host", "backend", "modality", "mem_gb", "file"}
	rows := []map[string]string{}
	for _, it := range asList(resp["models"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, fields))
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "host", "backend", "modality", "mem_gb"}, rows)
}

// ---- context 族（只读：ls）· 离线也出表（§九 M13 Z6）----

func cmdContextLs(inv *invocation, stdout, stderr io.Writer) int {
	// 续做面（§十二 `P-120` 定案 ①）：「上次做到哪」的入口就是这里，**不新立族**。
	if inv.resume {
		return cmdContextResume(inv, stdout, stderr)
	}
	rows := contextRows()
	return listCmd(inv, stdout, stderr, []string{"name", "core", "gateway", "default_node", "token_source"}, rows)
}

// contextRows 出档位名册：内置默认档恒在第一位；`~/.zerg/contexts/*.yaml` 有几个出几个。
// 名册真源 = `~/.zerg/contexts/<档>.yaml`（§九 M13 Z2）；令牌一律**掩码**（§九 M2 C6）。
func contextRows() []map[string]string {
	// 令牌来源**经唯一入口**取（`C1`：这条命令不许自己读环境变量/文件）。
	tok := tokenFromNone
	if v, src := resolveCredentials(); v != "" {
		tok = src
	}
	rows := []map[string]string{{
		"name":         "builtin",
		"core":         statepath.CoreBaseURL(),
		"gateway":      statepath.GatewayBaseURL(),
		"default_node": "",
		"token_source": string(tok) + "（值一律掩码）",
	}}
	home, err := os.UserHomeDir()
	if err != nil {
		return rows
	}
	dir := filepath.Join(home, ".zerg", "contexts")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return rows // 目录不存在 ⇒ 只有内置档（离线出表，不报错、不提示）
	}
	names := []string{}
	for _, e := range ents {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		row := map[string]string{
			"name":         strings.TrimSuffix(strings.TrimSuffix(n, ".yaml"), ".yml"),
			"core":         "",
			"gateway":      "",
			"default_node": "",
			"token_source": "",
		}
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := splitScalar(line)
			if !ok {
				continue
			}
			switch k {
			case "core", "gateway", "default_node":
				row[k] = v
			case "token", "token_source":
				row["token_source"] = "（掩码）"
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// splitScalar 认 `键: 值` 这一行（只读标量；本版不做完整 YAML 解析 —— 档位件今天还不存在）。
func splitScalar(line string) (string, string, bool) {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "-") {
		return "", "", false
	}
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", false
	}
	k := strings.TrimSpace(s[:i])
	v := strings.TrimSpace(s[i+1:])
	v = strings.Trim(v, `"'`)
	if k == "" || v == "" {
		return "", "", false
	}
	return k, v, true
}

// ---- doctor（裸动词 · 环境自检 · 声明树 ↔ 现值树 的差集）----
//
// 形状照 §九 M9：**逐项名 + 判定词 + 建议动作**，有任何问题就非零退出。
// 判定词是**五值闭集**（§九 M9 · 批 B · T-16）：`PASS` / `FAIL` / `BLOCKED` / `SKIP` / `REPORT`
// —— **不许加第六个**。退码口径（本版写死并打印，防误读）：
//
//	FAIL ⇒ 退码 1；**BLOCKED 与 SKIP 都 ⇒ 退码 8**（「读不到」「跳过没判」都不许当健康 · `RC9`）；
//	REPORT ⇒ 只报告、不影响退码。
//
// ★ 口径差（照实记，不许粉饰）：`调研-M9` §4 的五值表原文是 `PASS/WARN/FAIL/SKIP/BLOCKED`，
// 而开工单 T-16 判据① 要的是 `PASS/FAIL/BLOCKED/SKIP/REPORT`（把四档出口用的 `REPORT` 算进来、
// 不要 `WARN`）。本件按**开工单**落（它是本次的判据面），并把这条差登记进「所见非本批」⇒ 待拍。
//
// `--quick`：贵项（要打网/逐进程归因的那几项）**跳过并一律记 SKIP**（§十二 `P-040`）。
func cmdDoctor(inv *invocation, stdout, stderr io.Writer) int {
	items := doctorItems(inv)
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, items)
	}
	table := make([][]string, 0, len(items))
	worst := exitOK
	raise := func(code int) {
		// 严重度：FAIL(1) > BLOCKED/SKIP(8) —— 红优先于「没结论」（门禁同款口径）。
		if worst == exitOK || (code == exitFail && worst != exitFail) {
			worst = code
		}
	}
	for _, it := range items {
		table = append(table, []string{it["name"], it["verdict"], it["detail"], it["advice"]})
		switch it["verdict"] {
		case "FAIL":
			raise(exitFail)
		case "BLOCKED", "SKIP":
			if worst != exitFail {
				worst = exitBlocked
			}
		}
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain,
		[]string{"name", "verdict", "detail", "advice"}, table)
	fmt.Fprintln(stderr, "口径（五值闭集 · §九 M9）：FAIL ⇒ 退码 1 · BLOCKED ⇒ 8 · **SKIP ⇒ 8**（跳过没判 ≠ 健康）· REPORT ⇒ 只报告")
	return worst
}

// doctorItems —— 全单**逐行补齐机读面用键**（缺口 `GAP-20260928-60` 的另一半：`--json` 给两串原文）。
//
// 为什么在**外层**一次补、不在各分支里补：本单的项来自**两个件**（本件 + `ai_boundary.go` 的技能/MCP
// 两段），而 `--json` 出口（`emitSelected` → `marshalObject`）是**逐行核键**的 —— 只要有一行缺键，
// `zerg doctor --json name,artifact_commit` 就整单退 2（报「未知字段」）⇒ 只让漂移那一项有键 = **不可用**。
// ⇒ 两键在此**一次补齐**（缺省空串），漂移那一项在自己的分支里写真值（见 `driftIDs`）。
//
// 人面一字不动：表渲染取的是 name/verdict/detail/advice 四格，多两个键不进表；退码路径也不碰。
func doctorItems(inv *invocation) []map[string]string {
	items := doctorItemsRaw(inv)
	for _, it := range items {
		if _, ok := it["artifact_commit"]; !ok {
			it["artifact_commit"] = ""
		}
		if _, ok := it["source_head"]; !ok {
			it["source_head"] = ""
		}
	}
	return items
}

// doctorItemsRaw —— 原来的组装体（逐项 + 判定词 + 建议动作），一字未改，只在最外层被 `doctorItems` 补两键。
func doctorItemsRaw(inv *invocation) []map[string]string {
	items := []map[string]string{}

	// ① 仓根
	root := repoRoot()
	gate := ""
	if root != "" {
		gate = filepath.Join(root, "scripts", "gates", "precommit-gates.sh")
	}
	if root == "" {
		items = append(items, map[string]string{
			"name": "仓库根", "verdict": "FAIL", "detail": "解析不到仓根（找 core/internal/version/version.go 失败）",
			"advice": "在仓内跑，或设 ZERG_REPO=<仓根>"})
	} else if !isRepoRoot(root) {
		// ★ 收窄形（缺口 `GAP-20260927-436`）：`ZERG_REPO` 是**无条件采信**的（`root.go` 第 16 行 · 那里不动）。
		// 指到非仓根时上游每一面都 fail-closed（`gate run`/`gate ls` 硬错 rc=2 · `repo status`/`repo commit` 硬错 rc=8），
		// 唯独本行会把毒根报成「仓库根 PASS」= 静默换源且落在**报绿**这一侧（最险）。
		// ⇒ 本行只治这一面：解析出的位置**不是仓根**（判据同 `isRepoRoot` ⇒ 找不到 `core/internal/version/version.go`）
		//   ⇒ 不报 PASS，报「拿不到结论」（BLOCKED ⇒ 退码 8 · §九 M9 `RC9`：读不到不许当健康），并点名那枚毒值。
		items = append(items, map[string]string{
			"name": "仓库根", "verdict": "BLOCKED",
			"detail": "拿不到结论：ZERG_REPO 指到的位置不是仓根（找不到 core/internal/version/version.go）· ZERG_REPO=" + root,
			"advice": "请 `env -u ZERG_REPO` 再跑，或把它改指真仓根；本栏不猜路径、也不回退到上溯推导"})
	} else {
		items = append(items, map[string]string{
			"name": "仓库根", "verdict": "PASS", "detail": root, "advice": ""})
	}

	// ② 门禁最小入口（§九 M11 G1-e：路径是条款的一部分）
	if gate != "" {
		if fi, err := os.Stat(gate); err == nil && !fi.IsDir() {
			items = append(items, map[string]string{
				"name": "门禁最小入口", "verdict": "PASS", "detail": "scripts/gates/precommit-gates.sh 在", "advice": ""})
		} else {
			items = append(items, map[string]string{
				"name": "门禁最小入口", "verdict": "FAIL", "detail": "scripts/gates/precommit-gates.sh 不在",
				"advice": "它不依赖命令面/hook/网络，是自举件，必须就地存在"})
		}
	}

	// ③ 凭据（令牌不进 argv）
	if resolveToken() == "" {
		items = append(items, map[string]string{
			"name": "凭据", "verdict": "FAIL", "detail": "没找到令牌（ZERG_TOKEN / ~/.zerg/token 都空）",
			"advice": "备好令牌再跑需要主控的命令；令牌永不进 argv"})
	} else {
		items = append(items, map[string]string{
			"name": "凭据", "verdict": "PASS", "detail": "来自 " + string(newClient().tokenOrigin()), "advice": ""})
	}

	// ④ 主控可达（fail-closed：打不到就报，不换端点、不读缓存）
	c := newClient()
	var caps jsonObj
	if err := c.getJSON("/api/capabilities", &caps); err != nil {
		inv.setErr("blocked", "core_unreadable", err.Error())
		items = append(items, map[string]string{
			"name": "主控可达", "verdict": "BLOCKED", "detail": err.Error(),
			"advice": "端点来源 " + baseFromBuiltin + "；命令面不偷偷直连子端（§九 M20 O1/O3）"})
	} else {
		items = append(items, map[string]string{
			"name": "主控可达", "verdict": "PASS",
			"detail": fmt.Sprintf("%s · 版本 %s · 能力 %d 条", c.base, cell(caps["version"]), len(asList(caps["capabilities"]))),
			"advice": ""})
	}

	// ⑤⑥ 「放文件即生效」两处口子的**收口②**（§十二 `P-101` ② · 批 E · T-60）：
	//   **默认不静默** —— 技能目录逐条列出；MCP 清单**明说不可见**（运行期清单在子端侧）。
	//   两段都是 REPORT/PASS 档（REPORT **不影响退码**）⇒ 加这两段不改 `doctor` 的退出码。
	items = append(items, doctorSkillItems()...)
	items = append(items, doctorMCPItems()...)

	// ⑤ 子端（经主控）· 顺带把「混版观测」照实报出来（判决权在批 C 的 T-23，本版只报事实）
	if inv.quick {
		items = append(items, map[string]string{
			"name": "子端名册", "verdict": "SKIP",
			"detail": "`--quick`：贵项（要打主控的那一项）本轮**跳过**",
			"advice": "SKIP 也会把整单退码拉到 `8`（「跳过没判」不许当健康 · §九 M9 `RC9` / §十二 P-040）"})
		items = append(items, ghostReapItem())
		return items
	}
	var fleet jsonObj
	if err := c.getJSON("/api/fleet/status", &fleet); err != nil {
		inv.setErr("blocked", "fleet_unreadable", err.Error())
		items = append(items, map[string]string{
			"name": "子端名册", "verdict": "BLOCKED", "detail": err.Error(), "advice": "先看主控可达那一项"})
	} else {
		ms := asObj(fleet["machines"])
		vers := map[string][]string{}
		for name, v := range ms {
			o := asObj(v)
			if o == nil {
				continue
			}
			vers[cell(o["code_version"])] = append(vers[cell(o["code_version"])], name)
		}
		keys := make([]string, 0, len(vers))
		for k := range vers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		detail := fmt.Sprintf("%d 台 · healthy_count=%s · unhealthy_count=%s",
			len(ms), cell(fleet["healthy_count"]), cell(fleet["unhealthy_count"]))
		verdict := "PASS"
		advice := ""
		// ★ 混版判决（§二十一 已红第 1 条 · T-23 已落）：**主控侧**是唯一判决处
		// （`core/internal/api/fleet_health.go`），响应里带 `mixed_version` 就直接引用它。
		// 三格分开，不许混：
		//   `mixed_version` = true  ⇒ FAIL（设计稿逐字「混版必须被判为不健康」）
		//   `mixed_version` = false ⇒ PASS（这一格只有新主控给得出）
		//   字段不在（旧主控的响应）⇒ REPORT（只报事实）+ 写明判决已落源码、运行期要等换件重启
		if mv, ok := fleet["mixed_version"].(bool); ok {
			if mv {
				verdict = "FAIL"
				advice = "主控判定混版：mixed_machines=" + cell(fleet["mixed_machines"]) +
					" · 主控版本 " + cell(fleet["master_code_version"]) + "（混版不许当健康 ⇒ 先对齐机群版本）"
			}
		} else if len(keys) > 1 {
			verdict = "REPORT"
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, k+"="+strings.Join(vers[k], ","))
			}
			detail += " · 观测到多版本：" + strings.Join(parts, " · ")
			advice = "本主控的响应里**没有** mixed_version 字段（旧件）⇒ 判决跑不到运行期；" +
				"判决实现已在源码（T-23 · core/internal/api/fleet_health.go），生效要换件重启主控 ⇒ 待 Mr2109 拍"
		}
		items = append(items, map[string]string{"name": "子端名册", "verdict": verdict, "detail": detail, "advice": advice})
	}

	// ⑥ 档位名册（离线也出表）
	rows := contextRows()
	items = append(items, map[string]string{
		"name": "档位名册", "verdict": "PASS",
		"detail": fmt.Sprintf("%d 档（含内置默认档）", len(rows)),
		"advice": "名册真源 = ~/.zerg/contexts/<档>.yaml（§九 M13 Z2）"})

	// ⑦ 命令面自身
	items = append(items, map[string]string{
		"name": "命令面", "verdict": "PASS",
		"detail": version.Line(progName), "advice": ""})

	// ⑦-b 源码 ↔ 制品两代对拍（缺口 `GAP-20260927-410`）：doctor 已报的制品代码 id（⑦）与源码 HEAD
	// 之间**补一条判据** —— 「当前 bin 是哪个提交编的 · 源码之后又改过哪些件」。不开第二份口径、不建新命令。
	items = append(items, doctorDriftItem())

	// ⑧ 回收候选（§十五.3 对象清册 · §九 M9：**幽灵服务不进自动候选**）
	items = append(items, ghostReapItem())

	return items
}

// doctorDriftItem —— 「源码 ↔ 制品」两代对拍的一条判据（缺口 `GAP-20260927-410`）。
//
// 病：`dev edit` 只写源码、**不触发重编** ⇒ 新行为在源码层成立而 `./bin/zerg` 还是旧代；今天只能
// 靠人记得重编。本项把「制品是哪个提交编的 · 源码之后又改过哪些件」摊成一格，一条 `zerg doctor` 可读。
//
// 口径（三态，逐字可读）：
//
//	① 干净：制品 `version.Commit` == 源码 `git rev-parse HEAD` ⇒ PASS「未过期」
//	② 过期：二者不等 ⇒ REPORT「过期 + 源码之后改过的件名」（REPORT = 只报告、**不改 doctor 退码**）
//	③ 拿不到结论：`version.Commit` 未注入（裸 `go build` / 进程内测试）或取不到仓根/HEAD ⇒ REPORT 明说拿不到
//
// 只读：只跑 `git rev-parse` / `git diff --name-only` / `git status --porcelain`（不写盘、不联网）。
// 制品身份真源 = `version.Commit`（`-ldflags -X` 注入 · 见 `scripts/build/build-all.sh`）—— 不另造第二份口径。
func doctorDriftItem() map[string]string {
	art := strings.TrimSpace(version.Commit)
	if art == "" || art == "unknown" {
		return driftIDs(map[string]string{
			"name": "源码/制品对拍", "verdict": "REPORT",
			"detail": "拿不到结论：本制品**没注入代码 id**（`version.Commit`=" + art + "）—— 裸 `go build` / 进程内测试的形态",
			"advice": "要能对拍就走正门重编：`zerg build all --only cli`（`-ldflags -X` 才会写进 `version.Commit`）"}, art, "")
	}
	root := repoRoot()
	if root == "" {
		return driftIDs(map[string]string{
			"name": "源码/制品对拍", "verdict": "REPORT",
			"detail": "拿不到结论：解析不到仓根 ⇒ 取不到源码 HEAD（不猜一个路径）",
			"advice": "在仓内跑，或设 `ZERG_REPO=<仓根>`"}, art, "")
	}
	head, err := runGitRO(root, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	if err != nil || head == "" {
		return driftIDs(map[string]string{
			"name": "源码/制品对拍", "verdict": "REPORT",
			"detail": "拿不到结论：`git rev-parse HEAD` 在仓根取不到（" + errText(err) + "）",
			"advice": "确认仓根是真的 git 工作树（`git -C <仓根> rev-parse HEAD`）"}, art, "")
	}
	if head == art || strings.HasPrefix(head, art) || strings.HasPrefix(art, head) {
		return driftIDs(map[string]string{
			"name": "源码/制品对拍", "verdict": "PASS",
			"detail": "未过期：制品 " + driftShort(art) + " == 源码 HEAD " + driftShort(head),
			"advice": ""}, art, head)
	}
	pa, ph := driftPair(art, head)
	detail := "**过期**：制品 " + pa + " ≠ 源码 HEAD " + ph
	if files := driftChangedFiles(root, art); len(files) == 0 {
		detail += "（git 面没列出改动件 —— 制品那笔可能不在本仓历史里）"
	} else {
		const maxShow = 5
		shown := files
		suffix := ""
		if len(shown) > maxShow {
			shown = shown[:maxShow]
			suffix = fmt.Sprintf(" 等 %d 件", len(files))
		}
		detail += "；源码之后改过 " + strconv.Itoa(len(files)) + " 件：" + strings.Join(shown, " · ") + suffix
	}
	return driftIDs(map[string]string{
		"name": "源码/制品对拍", "verdict": "REPORT", "detail": detail,
		"advice": "重编走正门：`zerg build all --only cli`（不重编则 `./bin/zerg` 还是旧代 · `-ldflags` 才写代码 id）"}, art, head)
}

// driftIDs —— 两串**完整指纹原文**进机读面（缺口 `GAP-20260928-60` 的「`--json` 给两串原文」那一半）。
//
// 只**添**两个新键，旧键（name/verdict/detail/advice）一个不删不改名；人面**一字不动**
// （展示层仍走 `driftShort` / `driftPair` 掐位与加长）⇒ 人面与退码不受本函数影响。
//
// 两键逐个落真值不截断：`artifact_commit` = 制品身份原文（`version.Commit` 全串，可能是 `+dirty` 形态）·
// `source_head` = 源码 `git rev-parse HEAD` 全串。三态里取不到的那一侧**留空串**（不猜、不填 `unknown`）。
func driftIDs(m map[string]string, art, head string) map[string]string {
	m["artifact_commit"] = art
	m["source_head"] = head
	return m
}

// driftShort —— 代码 id 取前 8 位（制品可能是短 sha、源码 HEAD 是 40 位；对拍只比 `==`/前缀，不截断取值）。
func driftShort(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// driftPair —— 出口展示层（缺口 `GAP-20260928-60`）：两侧**截短后看起来相同**时，把窗口按两串的
// 公共前缀往后加长（封顶为各串全长），让显示面不再印出 `X ≠ 源码 HEAD X` 这种与判词自相矛盾的
// 行 —— 真差异（制品身份带 `+dirty` 后缀 · 或差异落在第八位之后）在出口看得见。
//
// ★ 只改文案：**不参与判等**（判等仍是 `doctorDriftItem` 里那一次全串 `==`/前缀比）、不改退码。
// 调用点只在判「不等」的分支 ⇒ 两串全串必不相同、且**互不为前缀** ⇒ 加长后必然印出两串相异的值。
func driftPair(art, head string) (string, string) {
	a, h := driftShort(art), driftShort(head)
	if a != h {
		return a, h
	}
	n := 0
	for n < len(art) && n < len(head) && art[n] == head[n] {
		n++
	}
	n += 8
	if n > len(art) {
		n = len(art)
	}
	if n > len(head) {
		n = len(head)
	}
	return art[:n], head[:n]
}

// driftChangedFiles —— 制品那笔之后源码改过的件（**已提交**的 `art..HEAD` + **未提交**的工作树），
// 去重排序。取不到的部分按空处理（对拍只报能拿到的，不猜、不失败）。
func driftChangedFiles(root, art string) []string {
	seen := map[string]bool{}
	addPaths := func(out string) {
		for _, ln := range strings.Split(out, "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				seen[ln] = true
			}
		}
	}
	if out, err := runGitRO(root, "diff", "--name-only", art+"..HEAD"); err == nil {
		addPaths(out)
	}
	// porcelain 每行 `XY <路径>`（状态码两列 + 一个空格）⇒ 第 4 字节起是路径。
	if out, err := runGitRO(root, "status", "--porcelain"); err == nil {
		for _, ln := range strings.Split(out, "\n") {
			if len(ln) > 3 {
				seen[strings.TrimSpace(ln[3:])] = true
			}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

// errText —— 把 error 摊成短文本（nil ⇒ "无错误"），doctor 格里不塞长串。
func errText(err error) string {
	if err == nil {
		return "无错误"
	}
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// ghostReapItem —— 「现值有 · 声明无」的幽灵服务（§二十一 第 3 条 / §十五.3）。
//
// 口径（照 §九 M9 · `P-042`）：**只报 + 干跑单列「不属管辖」，不进自动候选**（自动回收只收
// 「能证明是自己且已到期」的件）。所以这里给的是 `REPORT` + 建议动作，不是 `FAIL`。
// 只读：读声明件 + 跑一次 `ps`（不碰任何进程、不杀不重启）。
//
// ★ `T2`（波①）**同源改造**：本项与 `zerg core daemon ls --declared` 的幽灵段**都调
// `ghostProcesses()`**（唯一一处读数）⇒ 两处条数**逐字同值**（`Q-057` 判据③）。
// 声明侧真源 = `deploy/服务声明.tsv`（`kind=ghost` 行的进程特征）；声明面读不到/没登记 ghost ⇒
// 退回内置特征，不发假绿。
func ghostReapItem() map[string]string {
	rows, perr := ghostProcesses()
	if perr != "" {
		return map[string]string{
			"name": "回收候选（幽灵服务）", "verdict": "SKIP",
			"detail": "`ps`/声明面读不到（" + perr + "）",
			"advice": "读不到就不给结论（SKIP ⇒ 退码 8）"}
	}
	ghosts := ghostBrief(rows)
	if len(ghosts) == 0 {
		return map[string]string{
			"name": "回收候选（幽灵服务）", "verdict": "PASS",
			"detail": "现值面没看到「声明外」的服务进程（本项只查 `cocoon-docs-service` 一类点名件）",
			"advice": ""}
	}
	return map[string]string{
		"name": "回收候选（幽灵服务）", "verdict": "REPORT",
		"detail": fmt.Sprintf("%d 条：%s", len(ghosts), strings.Join(ghosts, " | ")),
		"advice": "现值有 · 声明无 ⇒ 进 `reap` 干跑单并**列「不属管辖」**；幽灵服务**不进自动候选**（§九 M9）"}
}
