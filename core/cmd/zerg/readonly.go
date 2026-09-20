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

// fetch 取一个端点；失败按 M20 的形状报一次并给码（未认证 ⇒ 4，其余 ⇒ 1）。
func fetch(c *client, path string, out any, stderr io.Writer) int {
	err := c.getJSON(path, out)
	if err == nil {
		return exitOK
	}
	fmt.Fprintf(stderr, "%s: %v\n", progName, err)
	var ae *authError
	if errors.As(err, &ae) {
		return exitAuth
	}
	return exitFail
}

// listCmd 是四条「清单型」只读命令的公共骨架（api ls · agent ls · task ls · model ls）。
// render 负责把载荷摊成「字段名 → 值文本」的若干行；本函数只管三态渲染与退码。
func listCmd(inv *invocation, stdout, stderr io.Writer, display []string, rows []map[string]string) int {
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail // K2：给了 --json 但不给字段 ⇒ 1 + stdout 0 字节
		}
		return selectJSONList(stdout, stderr, inv.path, inv.fields, rows)
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
	if rc := fetch(c, "/api/capabilities", &resp, stderr); rc != exitOK {
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
	if rc := fetch(c, "/api/openapi.json", &resp, stderr); rc != exitOK {
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
	if rc := fetch(c, "/api/fleet/status", &resp, stderr); rc != exitOK {
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
	if rc := fetch(c, "/api/tasks", &resp, stderr); rc != exitOK {
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
	if rc := fetch(c, "/api/fleet/models", &resp, stderr); rc != exitOK {
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
	rows := contextRows()
	return listCmd(inv, stdout, stderr, []string{"name", "core", "gateway", "default_node", "token_source"}, rows)
}

// contextRows 出档位名册：内置默认档恒在第一位；`~/.zerg/contexts/*.yaml` 有几个出几个。
// 名册真源 = `~/.zerg/contexts/<档>.yaml`（§九 M13 Z2）；令牌一律**掩码**（§九 M2 C6）。
func contextRows() []map[string]string {
	tok := tokenFromNone
	if resolveToken() != "" {
		tok = tokenFromFile
	}
	if strings.TrimSpace(os.Getenv("ZERG_TOKEN")) != "" {
		tok = tokenFromEnv
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

// ---- doctor（裸动词 · 环境自检）----
//
// 形状照 §九 M9：**逐项名 + 判定词 + 建议动作**，有任何问题就非零退出。
// 判定词取 M9 五值表的四个（本版不用 SKIP）：PASS / WARN / FAIL / BLOCKED。
//   口径（本版写死并打印，防误读）：FAIL ⇒ 退码 1；BLOCKED ⇒ 退码 8（「读不到」不许当健康）；
//   WARN ⇒ 只报告、不影响退码。

func cmdDoctor(inv *invocation, stdout, stderr io.Writer) int {
	items := doctorItems()
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		return selectJSONList(stdout, stderr, inv.path, inv.fields, items)
	}
	table := make([][]string, 0, len(items))
	worst := exitOK
	for _, it := range items {
		table = append(table, []string{it["name"], it["verdict"], it["detail"], it["advice"]})
		switch it["verdict"] {
		case "FAIL":
			worst = exitFail
		case "BLOCKED":
			if worst == exitOK {
				worst = exitBlocked
			}
		}
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain,
		[]string{"name", "verdict", "detail", "advice"}, table)
	fmt.Fprintln(stderr, "口径：FAIL ⇒ 退码 1 · BLOCKED ⇒ 退码 8（「读不到」不许当健康）· WARN ⇒ 只报告")
	return worst
}

func doctorItems() []map[string]string {
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
		items = append(items, map[string]string{
			"name": "主控可达", "verdict": "BLOCKED", "detail": err.Error(),
			"advice": "端点来源 " + baseFromBuiltin + "；命令面不偷偷直连子端（§九 M20 O1/O3）"})
	} else {
		items = append(items, map[string]string{
			"name": "主控可达", "verdict": "PASS",
			"detail": fmt.Sprintf("%s · 版本 %s · 能力 %d 条", c.base, cell(caps["version"]), len(asList(caps["capabilities"]))),
			"advice": ""})
	}

	// ⑤ 子端（经主控）· 顺带把「混版观测」当 WARN 报出来（判决权在批 C 的 T-23，本版只报事实）
	var fleet jsonObj
	if err := c.getJSON("/api/fleet/status", &fleet); err != nil {
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
		if len(keys) > 1 {
			verdict = "WARN"
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, k+"="+strings.Join(vers[k], ","))
			}
			detail += " · 观测到多版本：" + strings.Join(parts, " · ")
			advice = "混版判决（healthy_count 该不该算健康）属批 C 的 T-23，本版只报事实、不改判"
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

	return items
}
