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
	"os/exec"
	"path/filepath"
	"runtime"
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
	seen := map[string]bool{}
	for _, it := range asList(resp["capabilities"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, []string{"name", "endpoint", "desc", "example"}))
			if k := liveEndpointKey(cell(o["endpoint"])); k != "" {
				seen[k] = true
			}
		}
	}
	// ★ 2026-10-14（2.5.15 波1 · 批 2「HTTP 端点投影」）：② 保留为内部实现的端点，命令面**只做投影**
	//   ⇒ 现读面之后追加本表里**尚未出现过**的那些（按 (方法, 路径) 去重 ⇒ 同一端点只出一次）。
	//   底层注册面（`chat_handlers.go` / `zerg-core/main.go` / `gateway.go`）**一个字节不改**；表与注册面
	//   的一致性（含出处锚 `件:行`）由 `api_internal_endpoints_test.go` 逐条现读核对。✗ 不开 `api call`。
	rows = append(rows, apiInternalLsRows(seen)...)
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
	seen := map[string]bool{}
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
			seen[apiEndpointKey(m, p)] = true
		}
	}
	// ★ 2026-10-14（2.5.15 波1 · 批 2「HTTP 端点投影」）：② 端点同批进路径面（口径与 `api ls` 同一条）——
	//   现读 `paths` 之后追加本表里尚未出现过的 (方法, 路径)，去重键相同 ⇒ 两面同一端点只出一次。
	rows = append(rows, apiInternalOpenAPIRows(seen)...)
	return listCmd(inv, stdout, stderr, []string{"path", "method", "summary"}, rows)
}

func cmdAPIHelp(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		fmt.Fprintf(stderr, "%s: `api help` 不吃位置参数（给的是 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	fmt.Fprint(stdout, `api 族 —— HTTP 面两条腿：**客户端只读投影** ＋ **本机服务端**

  zerg api ls        能力清单（投影主控 /api/capabilities；字段名与它逐字同源）
  zerg api openapi   路径表（投影主控 /api/openapi.json 的 paths）
  zerg api help      本说明
  zerg api serve     在本机**起**外部接入 REST API（常驻档 · 批 B4 归入：源件 zerg-api）
                     POST /task 提交任务 · GET /task/{id} 查 · GET /capabilities 清单 ·
                     GET /status 统计 · GET /panel 面板。鉴权：环境变量 ZERG_API_TOKEN
                     （缺则**拒启**；显式 --insecure 才放行并打横幅）

分工：ls / openapi / help 打的是**主控**（客户端面，零写操作）；serve 是**服务端**（在本机起
进程、由别的程序来打你 —— 写面，但只写你自己的任务单目录）。

不做的：`+"`api call`"+`（通用逃生门）**暂不开** —— 它绕过全部命令面校验，风险最高，
等命令面稳定再放（§7.1 P10 的定案）。
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
	// ★ 2026-10-23（缺口 `GAP-20261021-08`）：观察面 = **心跳快照**（载荷只由 `ReceiveHeartbeat` 建行 ·
	//   `core/internal/store/store.go:138/153-163`；主控 `StatusHandler` 不与名册并集 ·
	//   `core/internal/api/handlers.go:1224`）⇒ 名册里在、但此刻没有心跳的机器**整行消失**
	//   （改前现读：名册 3 台，本命令只出 2 台 —— mini-node-2 整台不见，而它就在名册里）。
	//   名册是**声明面**（§九 M13 `Z2` 真源 · `Z6` 离线也出表）⇒ 这里按名册补行：
	//   名册有 + 心跳无 ⇒ 出一行，`backend_state=无心跳`、`healthy=unknown`、其余格**留空不猜**。
	//   ✗ 不改主控载荷（`/api/fleet/status` 的 `machines` 含义一个字不动）；✗ 不把缺读数印成 0/idle。
	rosterNames, rosterPath, rosterWhy := agentRosterNames(inv)
	names := make([]string, 0, len(ms)+len(rosterNames))
	seen := make(map[string]bool, len(ms)+len(rosterNames))
	for n := range ms {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range rosterNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	// ★ 2026-10-05（缺口 `GAP-20261005-45`）：投影面 = **登记面**（`fieldListOf` 单一真源）。
	//   改前这里硬编 10 格，而命令树登记 16 格（`model`/`backend_state`/`active_requests`/
	//   `gpu_used_gb`/`backend_rss_gb`/`gpu_temp_c` 六格）⇒ `--json <那六格>`「公告却不产出」
	//   （更早的制品里还有「退 2 且列在同一条合法字段里」的自相矛盾）。真源 = `/api/fleet/status`
	//   的机器载荷（`store.FleetSnapshot` 逐键如实透出这 16 格）⇒ 投影补到登记面。
	//   只落**载荷真有的**键：无值整键不出现（与出口 `marshalObjectRegistered` 同一条纪律）。
	fields := fieldListOf(inv.path)
	rows := []map[string]string{}
	missing := []string{}
	for _, n := range names {
		o := asObj(ms[n])
		if o == nil {
			// 名册有、心跳无 ⇒ 补一行（只有能证实的格；其余整键缺席 —— `--json` 侧自动略键）。
			rows = append(rows, agentNoHeartbeatRow(n))
			missing = append(missing, n)
			continue
		}
		row := make(map[string]string, len(fields))
		for _, f := range fields {
			if v, ok := o[f]; ok {
				row[f] = cell(v)
			}
		}
		// ★ 与 `agent show` 同一口径（单一真源助手 `agentMarkUnknownGPU`）：macOS 上 `gpu_pct`
		//   缺键、`gpu_used_gb` 是「未知」哨兵 0 ⇒ 两格示 `n/a`。改前 ls 面此处**没做**这一步
		//   ⇒ 同一份载荷在 ls/show 两处**两套口径**（ls 留空串 / 印假 0，show 示 `n/a`）
		//   —— 正是缺口 GAP-20260925-34 的「不反映真实值」在 ls 面的残格。
		agentMarkUnknownGPU(o, row)
		rows = append(rows, row)
	}
	// 对齐结论**必须说出口**（✗ 静默补行 —— 否则读者分不清「名册 3 台都在」与「只读到 2 台」）：
	//   名册读不到 ⇒ 对齐**判不了**（✗ 当 0 台）；名册读了但有台没心跳 ⇒ 点名是哪几台。
	if rosterWhy != "" {
		fmt.Fprintf(stderr, "%s: 名册读不到（%s）· %s ⇒ 与名册的**逐台对齐判不了**（本面只有心跳 %d 台）\n",
			progName, rosterPath, rosterWhy, len(ms))
	} else if len(missing) > 0 {
		fmt.Fprintf(stderr, "%s: 名册 %d 台 · 心跳 %d 台 · 名册有但无心跳 %d 台（%s）⇒ 已按名册补行（backend_state=%s · 其余格留空「不猜」）\n",
			progName, len(rosterNames), len(ms), len(missing), strings.Join(missing, " · "), agentNoHeartbeat)
	}
	// ★ 2026-10-10（缺口 `GAP-20261010-29` / `GAP-20261010-06`）：人面列 = **字段面同源**
	//   （`fieldListOf` 单一真源）。改前这里另写一份硬编 8 格清单 ⇒ 人面看不到机读面（`--json`）
	//   与登记面已有的 `active_requests` 等 8 格 ⇒ 「默认 ls 即判某台在不在干活」做不到。
	//   ⇒ 人面与机读面同源，✗ 不再造第三套口径。
	return listCmd(inv, stdout, stderr, fields, rows)
}

// ---- task 族（只读：ls）----

func cmdTaskLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/tasks", &resp, stderr); rc != exitOK {
		return rc
	}
	// ★ 缺口 `GAP-20260926-18` 第一步：补两格（issue_status = 台账真源映射到状态机 10 值；
	//   artifact_present = 方案 A 产物面）。拒「queued 但产物已在盘」这种不可信态（见 status_conflict）。
	fields := []string{"id", "status", "issue_status", "artifact_present", "status_conflict", "conflict_reason", "model", "machine", "priority", "created_at", "description", "slice_id"}
	rows := []map[string]string{}
	for _, it := range asList(resp["tasks"]) {
		if o := asObj(it); o != nil {
			rows = append(rows, project(o, fields))
		}
	}
	return listCmd(inv, stdout, stderr, []string{"id", "status", "issue_status", "artifact_present", "model", "machine", "priority", "created_at", "description"}, rows)
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
// doctorCredentialConsistency —— 缺口 GAP-20260927-200（批⑤ · 2026-10-13）：**凭据一致性**判词。
//
// 只对拍本单**已有的两行判决**（「凭据」⟷「主控可达」）—— ✗ 不另起一条探活、✗ 不读令牌值
// （令牌永不进 argv / 日志 / 回显）。「主控接受本机凭据」= 拿着本机凭据打得通主控面那一趟。
func doctorCredentialConsistency(items []map[string]string) string {
	cred, core := "", ""
	for _, it := range items {
		switch it["name"] {
		case "凭据":
			cred = it["verdict"]
		case "主控可达":
			core = it["verdict"]
		}
	}
	switch {
	case cred == "":
		return "（未取数：本单没有「凭据」项）"
	case cred != "PASS":
		return "（无本机凭据 ⇒ 无从对拍）"
	case core == "PASS":
		return "一致（磁盘凭据被主控接受 · 主控判决 PASS）"
	default:
		c := core
		if c == "" {
			c = "（无「主控可达」项）"
		}
		return "（未取数：主控" + c + " ⇒ 拿不到主控对凭据的判决）"
	}
}

func doctorItems(inv *invocation) []map[string]string {
	items := doctorItemsRaw(inv)
	// ★ 缺口 GAP-20260927-200（批⑤ · 2026-10-13）：`credential_consistency` —— 只对拍下面两枚既有行的
	//   判决（凭据 ⟷ 主控可达），✗ 不另起探活；真值落「凭据」那一行，其余行补齐空串（同
	//   `artifact_commit`/`source_head` 的补键体例）。
	consistency := doctorCredentialConsistency(items)
	for _, it := range items {
		if _, ok := it["artifact_commit"]; !ok {
			it["artifact_commit"] = ""
		}
		if _, ok := it["source_head"]; !ok {
			it["source_head"] = ""
		}
		if it["name"] == "凭据" {
			it["credential_consistency"] = consistency
		} else if _, ok := it["credential_consistency"]; !ok {
			it["credential_consistency"] = ""
		}
	}
	return items
}

// doctorMemCells —— 缺口 `GAP-20260925-47`：doctor 的「内存余量 / 内核 OOM 归因」两格。
//
// 为什么两格：node-a 的 llama-server 被内核 OOM 杀掉时，主控侧只表现为 503 + 超时，而机器本体健康
// （子端活、heartbeat 绿）⇒ 运维看不出是 OOM。数据在子端（单元结局）与心跳（mem_available）里
// 都有，缺的是**诊断面把余量与 OOM 事件露出来**（§九 M9 `RC9`：读不到不当健康）。
//
//	① 内核 OOM 归因：读 `machines.<机>.exit_reason`（子端上抛的单元结局词表）；
//	   命中 `oom-killed` ⇒ REPORT（只报告，不影响 rc）· 名册读得到却**从无该键** ⇒ SKIP（子端未上抛）
//	   · 有名册且无命中 ⇒ PASS。★ 只认 `oom-killed` 一个词（`signal+9` 一类不误报 —— 误报比漏报更坏）。
//	② 引擎内存余量：`machines.<机>.mem_available_gb` 对 `/api/fleet/models` 该机**最大**模型
//	   `mem_gb`（阈值口径 = 缺口 `want_action`③「始终留出 >=1 个最大模型的 mem_gb」）。
//	   ★ **不二次扣减已驻留**（`monitor/dual_gate.go:7`：MemAvailable 已含 GTT 占用）。
//	   余量 < 阈值 ⇒ REPORT · 某台读不到 ⇒ SKIP（整格不给结论）。
//
// `known=false`（拿不到 `/api/fleet/status` 载荷）⇒ 两格都 SKIP。
func doctorMemCells(fleet jsonObj, known bool) []map[string]string {
	oom := map[string]string{"name": "内核 OOM 归因", "verdict": "SKIP",
		"detail": "读不到机群载荷（/api/fleet/status）", "advice": "先从主控读机群状态（§九 M9 RC9：读不到不当健康）"}
	mem := map[string]string{"name": "引擎内存余量", "verdict": "SKIP",
		"detail": "读不到机群载荷（/api/fleet/status）", "advice": "先从主控读机群状态（§九 M9 RC9）"}
	if !known {
		return []map[string]string{oom, mem}
	}
	ms := asObj(fleet["machines"])

	// ① 内核 OOM 归因（只认 `oom-killed` 一个词）。
	seenKey := false
	oomHit := []string{}
	for name, v := range ms {
		o := asObj(v)
		if o == nil {
			continue
		}
		if _, ok := o["exit_reason"]; ok {
			seenKey = true
		}
		if strings.EqualFold(cell(o["exit_reason"]), "oom-killed") {
			oomHit = append(oomHit, name)
		}
	}
	sort.Strings(oomHit)
	switch {
	case !seenKey:
		oom = map[string]string{"name": "内核 OOM 归因", "verdict": "SKIP",
			"detail": "主控名册里没有 `exit_reason` 这一格（子端未上抛单元结局 ⇒ 读不到，不当健康）",
			"advice": "上抛端在子端（另路）：子端把 systemd Result/ExecMainStatus 归类的 `oom-killed` 打进心跳后本格自动生效"}
	case len(oomHit) > 0:
		oom = map[string]string{"name": "内核 OOM 归因", "verdict": "REPORT",
			"detail": "最近有单元被内核 OOM 杀掉：" + strings.Join(oomHit, ", ") + "（心跳绿 ≠ 没被杀过）",
			"advice": "查该机 dmesg 与常驻模型数；按「mem_available 始终留出 >=1 个最大模型的 mem_gb」控常驻数"}
	default:
		oom = map[string]string{"name": "内核 OOM 归因", "verdict": "PASS",
			"detail": "名册内无 `oom-killed` 结局", "advice": ""}
	}

	// ② 引擎内存余量。
	var models jsonObj
	if err := newClient().getJSON("/api/fleet/models", &models); err != nil {
		mem = map[string]string{"name": "引擎内存余量", "verdict": "SKIP",
			"detail": "读不到 /api/fleet/models：" + err.Error(), "advice": "先从主控读模型清单（§九 M9 RC9）"}
		return []map[string]string{oom, mem}
	}
	maxMem := map[string]float64{}
	for _, it := range asList(models["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		h := cell(o["host"])
		if f, ok := numOf(o["mem_gb"]); ok && f > maxMem[h] {
			maxMem[h] = f
		}
	}
	low := []string{}
	unknownHosts := 0
	for name, v := range ms {
		o := asObj(v)
		if o == nil {
			continue
		}
		avail, ok := numOf(o["mem_available_gb"])
		if !ok {
			unknownHosts++
			continue
		}
		thr := maxMem[name]
		if thr <= 0 {
			continue // 该机无名册模型 ⇒ 无从判（不编一个数）
		}
		if avail < thr {
			low = append(low, fmt.Sprintf("%s（余 %.1f < 最大模型 %.1f GB）", name, avail, thr))
		}
	}
	sort.Strings(low)
	switch {
	case len(low) > 0:
		mem = map[string]string{"name": "引擎内存余量", "verdict": "REPORT",
			"detail": "余量低于该机最大模型：" + strings.Join(low, " · "),
			"advice": "按 want_action③：让 mem_available 始终留出 >=1 个最大模型的 mem_gb，再决定是否起新引擎"}
	case unknownHosts > 0:
		mem = map[string]string{"name": "引擎内存余量", "verdict": "SKIP",
			"detail": fmt.Sprintf("%d 台读不到 mem_available_gb（读不到不当健康）", unknownHosts), "advice": "先从主控读机群状态"}
	default:
		mem = map[string]string{"name": "引擎内存余量", "verdict": "PASS",
			"detail": "各机余量均不低于其最大模型 mem_gb", "advice": ""}
	}
	return []map[string]string{oom, mem}
}

// doctorHostLoadItem —— 本机过载体检（只读 · 缺口 `GAP-20260926-202` 候选 c1 的只读落点）。
//
// 回答「本机现在**过载了吗**」：此前**无一面**可答 —— 负载 vs 核数、本机并发上限（卵占槽/上限）都只能事后手查
// （真因＝并发总量无上限：卵数 × 每卵 `-np` 槽）。本格把三样摊成一格：
//
//	① `loadavg` 三均值 vs 核数（`hw.ncpu`）⇒ **明确「过载/不过载」判定**（✗ 不只印数字）；
//	② 本机并发上限 = `config/workers.yaml` 本机条的 `max_workers`（复用 `agentDeviceMaxFor` 单一真源，✗ 不造第二份）；
//	③ 本机在跑（`active_requests`）—— 只有机群载荷读得到才报（✗ 取不到不编）。
//
// 判定词取 **REPORT**（只报告）—— 照 `doctorMemCells` 资源类项先例（过载 = 机器忙 ≠ 配置坏，不混进退码 ·
// REPORT **不影响** `doctor` 退码）；`loadavg` / 核数读不到 ⇒ SKIP（§九 M9：读不到不当健康）。
// ★ 只读：读 `/proc/loadavg` 或 `sysctl -n vm.loadavg` 与 `config/workers.yaml`，✗ 不写件、✗ 不改 workers.yaml、
// ✗ 不触发调度、✗ 不启停服务。
func doctorHostLoadItem(fleet jsonObj, fleetKnown bool) map[string]string {
	l1, l5, l15, ok := hostLoadAvg()
	if !ok {
		return map[string]string{
			"name": "本机过载体检", "verdict": "SKIP",
			"detail": "读不到 loadavg（/proc/loadavg 与 `sysctl -n vm.loadavg` 都取不到）—— 读不到不当健康",
			"advice": "确认 sysctl 可用（macOS: `/usr/sbin/sysctl -n vm.loadavg`）；本格不编一个数"}
	}
	ncpu := hostNCPU()
	if ncpu <= 0 {
		return map[string]string{
			"name": "本机过载体检", "verdict": "SKIP",
			"detail": fmt.Sprintf("读到 loadavg=%.2f/%.2f/%.2f 但拿不到核数（`hw.ncpu`）⇒ 无从判过载（不编一个分母）", l1, l5, l15),
			"advice": "确认 `sysctl -n hw.ncpu` 可用；核数是「过载」判据的分母"}
	}
	overload := l1 > float64(ncpu)
	word, cmp := "不过载", "≤"
	if overload {
		word, cmp = "过载", ">"
	}
	verdict := "PASS"
	advice := ""
	if overload {
		verdict = "REPORT"
		advice = "核数忙满：先看谁在跑（`zerg agent ls` 的 active_requests）· 本机可 `zerg agent workers set <本机> --max N` 降并发上限（0 = 暂停派活）；本格只读、不动调度"
	}
	detail := fmt.Sprintf("%s：load1=%.2f %s ncpu=%d（load5=%.2f · load15=%.2f）", word, l1, cmp, ncpu, l5, l15)
	// ② 本机并发上限（卵占槽/上限）：读仓内 `config/workers.yaml` 本机条的 max_workers（复用同族只读解析，单一真源）。
	if p := agentWorkersConfigPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if v, found := agentDeviceMaxFor(string(b), planHost()); found {
				if strings.TrimSpace(v) == "0" {
					detail += " · 本机并发上限 max_workers=0（显式暂停派活）"
				} else {
					detail += " · 本机并发上限 max_workers=" + v
				}
			} else {
				detail += " · 本机不在 devices 段（回退内置默认并发）"
			}
		}
	}
	// ③ 本机在跑（占槽）：机群载荷读得到才报（机名取本机名，逐字小写比；取不到 ⇒ 不编）。
	if fleetKnown {
		if ms := asObj(fleet["machines"]); ms != nil {
			want := strings.ToLower(strings.TrimSpace(planHost()))
			for name, v := range ms {
				if strings.ToLower(strings.TrimSpace(name)) != want {
					continue
				}
				if ar, aok := numOf(asObj(v)["active_requests"]); aok {
					detail += fmt.Sprintf(" · 在跑 active_requests=%d", int(ar))
				}
				break
			}
		}
	}
	return map[string]string{"name": "本机过载体检", "verdict": verdict, "detail": detail, "advice": advice}
}

// hostLoadAvg —— 本机 loadavg 三均值（1/5/15 分钟）。只读：Linux 读 `/proc/loadavg`；macOS/BSD 走
// `sysctl -n vm.loadavg`（照同族 `agent/internal/monitor` 口径，✗ 不另造）。两条通路都取不到 ⇒ ok=false。
// ★ 首参一律**带引号的字面量**（✗ 不用变量当首个实参）—— 门103 `check-exec-two-condition` 的 P3 档专报
// `exec.Command(<变量>` 形态；用字面量即不命中该报告档。
func hostLoadAvg() (l1, l5, l15 float64, ok bool) {
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if a, c, d, aok := parseLoadAvgFields(strings.Fields(string(b))); aok {
			return a, c, d, true
		}
	}
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		out, err = exec.Command("sysctl", "-n", "vm.loadavg").Output()
	}
	if err == nil {
		// 形如 `{ 3.19 4.12 4.90 }`（花括号两枚去掉后按字段取三枚）。
		f := strings.Fields(strings.NewReplacer("{", " ", "}", " ").Replace(string(out)))
		if a, c, d, aok := parseLoadAvgFields(f); aok {
			return a, c, d, true
		}
	}
	return 0, 0, 0, false
}

// parseLoadAvgFields —— 从字段序列取前三枚浮点（1/5/15 分钟均值）；不足 / 坏值 ⇒ ok=false（不编）。
func parseLoadAvgFields(f []string) (l1, l5, l15 float64, ok bool) {
	if len(f) < 3 {
		return 0, 0, 0, false
	}
	a, e1 := strconv.ParseFloat(f[0], 64)
	b, e2 := strconv.ParseFloat(f[1], 64)
	c, e3 := strconv.ParseFloat(f[2], 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, 0, 0, false
	}
	return a, b, c, true
}

// hostNCPU —— 本机逻辑核数（`hw.ncpu`）。macOS 走 `sysctl -n hw.ncpu`；取不到 ⇒ 回落 `runtime.NumCPU()`（Linux：
// `hw.ncpu` 不存在 ⇒ 用逻辑 CPU 数当分母，语义同「核数」）。两者都 0 ⇒ 0（调用方 SKIP，不猜一个分母）。
func hostNCPU() int {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.ncpu").Output()
	if err != nil {
		out, err = exec.Command("sysctl", "-n", "hw.ncpu").Output()
	}
	if err == nil {
		if n, aerr := strconv.Atoi(strings.TrimSpace(string(out))); aerr == nil && n > 0 {
			return n
		}
	}
	if n := runtime.NumCPU(); n > 0 {
		return n
	}
	return 0
}

// numOf —— 把一个载荷值取成 float64（JSON 数是 float64；字符串形态也认，取不到 ⇒ ok=false）。
func numOf(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
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
		// 缺口 `GAP-20260925-47`：`--quick` 下两格**仍在表里**（==SKIP），不许整格消失（E 型「绿着错」）。
		items = append(items, doctorMemCells(nil, false)...)
		// 本机过载体检（本地读 · 快档也出；机群载荷未读 ⇒ 占槽那格缺席，不编）。
		items = append(items, doctorHostLoadItem(nil, false))
		return items
	}
	var fleet jsonObj
	fleetKnown := false
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
		fleetKnown = true
	}
	// 缺口 `GAP-20260925-47`：内存余量 / 内核 OOM 归因两格（读得到才判；读不到 ⇒ SKIP · 不当健康）。
	items = append(items, doctorMemCells(fleet, fleetKnown)...)
	// 本机过载体检（GAP-20260926-202 候选 c1 的只读落点）：本地负载 vs 核数 ＋ 本机并发上限（占槽）。
	items = append(items, doctorHostLoadItem(fleet, fleetKnown))

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

	// ⑨ 制品件权限 / 签名核验面 / 摘要方言（缺口 `GAP-20260926-89`/`-90`/`-133`）
	items = append(items, doctorBinPermsItem())
	items = append(items, doctorSignatureItem())
	items = append(items, doctorDigestDialectItem())

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
// 制品身份真源 = `version.Commit`（`-ldflags -X` 注入 · 见 Go 真身 `scripts/build/cmd/zerg-build-all/main.go`）—— 不另造第二份口径。
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
		// ★ 两种过期成因要分开报（缺口 GAP-20260927-411）：id 相同**不等于**「未过期」——
		//   制品编的那笔（= 源码 HEAD）之后，工作树可能还有**未提交改动**；那批改动没进制品 ⇒
		//   制品相对当前工作树仍是旧的。此前这里一律 PASS「未过期」，而另一种写法（把真差异印成
		//   `制品 X ≠ 源码 HEAD X`）在两侧 id 相同时与判词自相矛盾 ⇒ 两种成因在出口同形、只能靠人读。
		//   现在：id 相同且工作树干净 ⇒ PASS；id 相同但工作树有未提交改动 ⇒ REPORT 明说「id 相同、
		//   只是源码之后改过 N 件」—— 与下面 id **不等** 的「≠」分支措辞不再混淆。
		if files := driftChangedFiles(root, art); len(files) > 0 {
			const maxShow = 5
			shown := files
			suffix := ""
			if len(shown) > maxShow {
				shown = shown[:maxShow]
				suffix = fmt.Sprintf(" 等 %d 件", len(files))
			}
			return driftIDs(map[string]string{
				"name": "源码/制品对拍", "verdict": "REPORT",
				"detail": "**过期**：制品 id **相同**（" + driftShort(art) + " == 源码 HEAD " + driftShort(head) +
					"），但源码之后工作树改过 " + strconv.Itoa(len(files)) + " 件（未提交改动 · 制品不含）⇒ 制品过期：" +
					strings.Join(shown, " · ") + suffix,
				"advice": "重编走正门：`zerg build all --only cli`（不重编则 `./bin/zerg` 不含工作树里的未提交改动）"}, art, head)
		}
		return driftIDs(map[string]string{
			"name": "源码/制品对拍", "verdict": "PASS",
			"detail": "未过期：制品 " + driftShort(art) + " == 源码 HEAD " + driftShort(head),
			"advice": ""}, art, head)
	}
	pa, ph := driftPair(art, head)
	// id **不等** 分支：措辞点明「id 不等」，与上面「id 相同、但有未提交改动」那档分开（GAP-20260927-411）。
	detail := "**过期**：制品 id 与源码 HEAD **不等**（制品 " + pa + " ≠ 源码 HEAD " + ph + "）"
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
	// ★ 2026-10-04（缺口 `GAP-20260927-77`）：drift 是只读面 —— `git status` 不带 `--no-optional-locks`
	//   会刷新并写回 `.git/index`（stat 过期件在场时）⇒ 走同族 `runGitRONoLock`，连索引刷新都不做。
	if out, err := runGitRONoLock(root, "status", "--porcelain"); err == nil {
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

// doctorBinPermsItem —— 「制品件权限」体检项（缺口 `GAP-20260926-89`）。
//
// 病：`bin/` 各件的权限位没有机读面 ⇒ 「内核件有没有 x 位」「有没有组可写」只能靠人 `ls -l` 看；
// 组可写让「只读面自证可执行」这类完整性判据只抗误操作、不抗有意篡改。
//
// 只读：只 `os.ReadDir` + `Info()` 读权限位（不写盘、不改权限、不起子进程）。
// 机读键 `bin_perms` = 现读计数一行（真值，不截断）。
func doctorBinPermsItem() map[string]string {
	root := repoRoot()
	if root == "" {
		return map[string]string{
			"name": "制品件权限", "verdict": "SKIP",
			"detail": "取不到仓根 ⇒ `bin/` 权限读不到",
			"advice": "读不到就不给结论（SKIP ⇒ 退码 8）", "bin_perms": ""}
	}
	ents, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil {
		return map[string]string{
			"name": "制品件权限", "verdict": "SKIP",
			"detail": "`bin/` 读不到（" + err.Error() + "）",
			"advice": "读不到就不给结论（SKIP ⇒ 退码 8）", "bin_perms": ""}
	}
	files, execOK, noExec, gW, oW := 0, 0, 0, 0, 0
	var off []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		fi, ierr := e.Info()
		if ierr != nil {
			continue
		}
		files++
		perm := fi.Mode().Perm()
		if perm&0o111 != 0 {
			execOK++
		} else {
			noExec++
			off = append(off, e.Name()+"(无 x 位)")
		}
		if perm&0o020 != 0 {
			gW++
			off = append(off, e.Name()+"(组可写)")
		}
		if perm&0o002 != 0 {
			oW++
		}
	}
	read := fmt.Sprintf("件 %d · 可执行 %d · 无 x 位 %d · 组可写 %d · 世界可写 %d", files, execOK, noExec, gW, oW)
	verdict, advice := "PASS", ""
	if gW > 0 || oW > 0 {
		verdict = "REPORT"
		advice = "组/世界可写 ⇒ 完整性判据只抗误操作、不抗有意篡改；收回写位：`chmod go-w bin/<件>`"
	} else if noExec > 0 {
		verdict = "REPORT"
		advice = "有件无 x 位：非可执行件（如 JSON）属正常；若该件本应可跑 ⇒ `chmod +x bin/<件>`"
	}
	detail := "`bin/` 权限现读（真值 · 只 `stat` 不写盘）：" + read
	if len(off) > 0 {
		detail += "；离群件：" + strings.Join(off, " · ")
	}
	return map[string]string{
		"name": "制品件权限", "verdict": verdict, "detail": detail, "advice": advice, "bin_perms": read}
}

// doctorSignatureItem —— 「签名核验面」体检项（缺口 `GAP-20260926-90`）。
//
// 病：签名核验面（钥匙在不在、能不能验）不在任何门或稿里，是空白 ⇒ 排障时无法回答「本机能不能验签名」。
//
// 只读：只 `exec.LookPath` + 对 `./bin/zerg` 真跑一次 `codesign -v`（只验签、不写盘、不联网）。
// 机读键 `signature` = 「在/缺」两份名单 + 本机件验签结论一行（真值）。
func doctorSignatureItem() map[string]string {
	all := []string{"codesign", "spctl", "openssl", "gpg", "minisign", "signify", "keyctl", "tpm2"}
	var have, miss []string
	for _, t := range all {
		if _, err := exec.LookPath(t); err == nil {
			have = append(have, t)
		} else {
			miss = append(miss, t)
		}
	}
	if len(have) == 0 {
		return map[string]string{
			"name": "签名核验面", "verdict": "SKIP",
			"detail": "本机没有可用的核验工具（找过 " + strings.Join(all, ",") + "）",
			"advice": "读不到就不给结论（SKIP ⇒ 退码 8）", "signature": "在：(空) · 缺：" + strings.Join(miss, ",")}
	}
	verify := "未验（本机无 codesign）"
	if _, err := exec.LookPath("codesign"); err == nil {
		if root := repoRoot(); root != "" {
			bin := filepath.Join(root, "bin", "zerg")
			if _, serr := os.Stat(bin); serr == nil {
				out, cerr := exec.Command("codesign", "-v", bin).CombinedOutput()
				txt := strings.ReplaceAll(strings.TrimSpace(string(out)), "\n", " / ")
				if len(txt) > 160 {
					txt = txt[:160]
				}
				if cerr == nil {
					verify = "codesign -v ./bin/zerg ⇒ 通过"
				} else {
					verify = "codesign -v ./bin/zerg ⇒ 未通过/未签（" + txt + "）"
				}
			}
		}
	}
	read := "在：" + strings.Join(have, ",") + " · 缺：" + strings.Join(miss, ",") + " · " + verify
	return map[string]string{
		"name": "签名核验面", "verdict": "PASS",
		"detail": "核验工具面（真读 `LookPath` + 真跑一次 `codesign -v`）：" + read,
		"advice": "", "signature": read}
}

// doctorDigestDialectItem —— 「摘要方言」体检项（缺口 `GAP-20260926-133`）。
//
// 病：`sha256sum` 的方言（GNU / uutils / Darwin-BSD 各派）与 `--check`/`--status` 可用性
// 不在任何门或稿里 ⇒ 按字节冻结摘要输出的判据在不同机上会静默漂。
//
// 只读：只跑 `sha256sum --version` 与逐旗标探（`/dev/null` 作哑元 · 不写盘）。
// 机读键 `digest_dialect` = 方言名 + 旗标可用性一行（真值）。
func doctorDigestDialectItem() map[string]string {
	bin, err := exec.LookPath("sha256sum")
	if err != nil {
		return map[string]string{
			"name": "摘要方言", "verdict": "SKIP",
			"detail": "本机没有 `sha256sum`（找过 PATH）",
			"advice": "读不到就不给结论（SKIP ⇒ 退码 8）", "digest_dialect": ""}
	}
	dialect := "未知"
	if out, verr := exec.Command(bin, "--version").CombinedOutput(); verr == nil {
		s := string(out)
		switch {
		case strings.Contains(s, "uutils"):
			dialect = "uutils"
		case strings.Contains(s, "GNU"):
			dialect = "GNU"
		case strings.Contains(s, "Darwin") || strings.Contains(s, "BSD"):
			dialect = "Darwin/BSD"
		}
	}
	var okFlags, noFlags []string
	for _, fl := range []string{"--check", "--status", "-c", "--quiet"} {
		cerr := exec.Command(bin, fl, "/dev/null").Run()
		if cerr == nil {
			okFlags = append(okFlags, fl)
			continue
		}
		msg := strings.ToLower(cerr.Error())
		if strings.Contains(msg, "unrecognized") || strings.Contains(msg, "invalid option") || strings.Contains(msg, "illegal option") {
			noFlags = append(noFlags, fl)
		} else {
			okFlags = append(okFlags, fl)
		}
	}
	read := "方言=" + dialect + " · 认：" + strings.Join(okFlags, ",") + " · 不认：" + strings.Join(noFlags, ",")
	return map[string]string{
		"name": "摘要方言", "verdict": "PASS",
		"detail": "摘要工具方言面（真跑 `sha256sum --version` 与逐旗标探）：" + read,
		"advice": "", "digest_dialect": read}
}
