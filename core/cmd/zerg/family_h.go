// family_h.go —— H 族四条：`gateway` / `api` / `resource` / `script`（§三 H 族 · §7.1 `P10`/`P7`）。
//
// 本件的四条口径（照定稿，不自造）：
//
//	① **`zerg api call` 暂不开**（§7.1 `P10` 定案）：通用逃生门绕过全部命令面校验 ⇒
//	   本族只做 `ls` / `openapi` / `help`；要开走提案 + 人批。
//	② **对话面 19 条不进 CLI**（§7.1 `P7`：HTTP 17 + UI 2）：等外部 agent 真在调 `/api/chat/*`
//	   再说（`U16` 未证实）⇒ 本族一条对话命令都不登记。
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
func cmdResourceLs(inv *invocation, stdout, stderr io.Writer) int {
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

// ---- 只读：脚本面（最小面；125 件的「现状标注」在 T-50 落）---------------------------------

// cmdScriptLs —— `zerg script ls`（只读本机面）：`scripts/` 下的脚本逐件列出。
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
	for _, sub := range scanRoots(filepath.Join(root, "scripts")) {
		rows = append(rows, sub...)
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
	return listCmd(inv, stdout, stderr, []string{"path", "public"}, rows)
}

// scanRoots 递归列 `scripts/` 下的可执行件（顶层 + 一层子目录，与门禁脚本的扫描面同口径）。
func scanRoots(root string) [][]map[string]string {
	out := [][]map[string]string{}
	ents, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	one := []map[string]string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sh") && !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		one = append(one, map[string]string{"path": filepath.Join("scripts", e.Name()), "public": ""})
	}
	sort.Slice(one, func(i, j int) bool { return one[i]["path"] < one[j]["path"] })
	out = append(out, one)
	subs := []string{}
	for _, e := range ents {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			subs = append(subs, e.Name())
		}
	}
	sort.Strings(subs)
	for _, s := range subs {
		dir := filepath.Join(root, s)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		rows := []map[string]string{}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !strings.HasSuffix(f.Name(), ".sh") && !strings.HasSuffix(f.Name(), ".py") {
				continue
			}
			rows = append(rows, map[string]string{"path": filepath.Join("scripts", s, f.Name()), "public": ""})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["path"] < rows[j]["path"] })
		out = append(out, rows)
	}
	return out
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
