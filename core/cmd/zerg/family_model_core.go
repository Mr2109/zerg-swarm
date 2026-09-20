// family_model_core.go —— `model` 与 `core` 两族（§三 E / C 族 · §7.1 `P11` · §十二 `P-029`）。
//
// 口径三条（照 §三 与 §十二，不自造）：
//
//	① **`model` 与 `machine` 不是一回事**（§十二 `P-029` 点名的重名坑）：`model` 族的对象是
//	   **模型 id**（`/api/fleet/models` 里的 id），`agent` 族的对象是**机器名**。两族的
//	   `--json` 字段名也分家（这里不出现 `machine` 之外的混用）。
//	② `core` 族 = **主控自身**（§三 C 族）：`status` 只读 · `daemon ls` 读**本机件**（scripts/svc/）·
//	   `start`/`restart`/`stop`/`reload`/`update` 是危险档（`stop` 要 `--confirm=<主机名>`）。
//	③ `model opts` 的两条端点**逐字取真源**（`core/cmd/zerg-core/main.go:176-177`）：
//	   `GET /api/models/{name}/adapter-opts` · `PUT /api/models/{name}/adapter-opts`。
//	   响应键 `model`/`schema`/`note` 就是 `--json` 的字段名（不另造词汇）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---- model 族：单模型 / 适配器选项 --------------------------------------------------------

// cmdModelShow —— `zerg model show <模型 id>`（只读）：从 `/api/fleet/models` 里挑一条。
func cmdModelShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `model show` 要一个模型 id（先 `zerg model ls`）\n", progName)
		inv.setErr("usage", "missing_target", "缺模型 id")
		return exitUsage
	}
	want := inv.args[0]
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &resp, stderr); rc != exitOK {
		return rc
	}
	fields := []string{"id", "host", "backend", "modality", "mem_gb", "file"}
	var row map[string]string
	ids := []string{}
	for _, it := range asList(resp["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		ids = append(ids, cell(o["id"]))
		if cell(o["id"]) == want {
			row = project(o, fields)
		}
	}
	if row == nil {
		sort.Strings(ids)
		fmt.Fprintf(stderr, "%s: 模型面里没有 %q\n", progName, want)
		if s := nearestName(want, ids); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		inv.setErr("unsupported_on_node", "unknown_model", "模型面里没有这个 id")
		return exitUsage
	}
	return listCmd(inv, stdout, stderr, fields, []map[string]string{row})
}

// cmdModelOpts —— `zerg model opts get|set <模型 id>`。
//
//	get：只读，投影 `GET /api/models/{name}/adapter-opts`（键 `model`/`schema`/`note` 逐字）。
//	set：D2 写面（**实时生效** ⇒ 要 `--yes`）；参数用可重复的 `--set k=v` 给，
//	     PUT 体就是这些键值 —— **不替主控编键名**（认不认由主控判，错误码逐字透传）。
func cmdModelOpts(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 {
		fmt.Fprintf(stderr, "%s: 用法：`zerg model opts get <模型 id>` / `zerg model opts set <模型 id> --set k=v [--yes]`\n", progName)
		inv.setErr("usage", "missing_target", "缺动作（get/set）或模型 id")
		return exitUsage
	}
	action, name := inv.args[0], inv.args[1]
	switch action {
	case "get":
		// 只读：一次 GET。失败面**把主控的码逐字打出来**（透传不翻译）——
		// 例：这台上没适配器的模型主控回 `404 {"error":{"type":"MODEL_NO_ADAPTER"}}`。
		status, respBody, err := sendJSON(newClient(), "GET", "/api/models/"+name+"/adapter-opts", "")
		if err != nil {
			code := errorCodeOf(respBody)
			kind := "upstream_error"
			if status == 404 {
				kind = "unsupported_on_node"
			}
			inv.setErr(kind, code, "主控拒绝了这次读取")
			fmt.Fprintf(stderr, "%s: 主控 /api/models/%s/adapter-opts ⇒ http_%d · code=%s\n",
				progName, name, status, orDash(code))
			fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
			fmt.Fprintf(stderr, "（码**逐字透传、命令面不翻译**）\n")
			fmt.Fprintf(stderr, "error.kind=%s · retryable=%t · remedy=%s\n", kind, retryableOf(kind), remedyOf(kind))
			return codeOfKind(kind)
		}
		var resp jsonObj
		if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
			inv.setErr("failed", "bad_payload", "响应不是 JSON")
			fmt.Fprintf(stderr, "%s: 适配器面响应不是 JSON：%v\n", progName, err)
			return exitFail
		}
		row := project(resp, []string{"model", "schema", "note"})
		if row["model"] == "" {
			row["model"] = name
		}
		return listCmd(inv, stdout, stderr, []string{"model", "schema", "note"}, []map[string]string{row})
	case "set":
		body := map[string]any{}
		for _, kv := range inv.setPairs {
			k, v, ok := strings.Cut(kv, "=")
			if !ok || strings.TrimSpace(k) == "" {
				inv.setErr("usage", "bad_set_pair", "--set 要 `键=值` 形态")
				fmt.Fprintf(stderr, "%s: `--set` 要 `键=值`（给的是 %q）\n", progName, kv)
				return exitUsage
			}
			body[strings.TrimSpace(k)] = coerceScalar(strings.TrimSpace(v))
		}
		payloadBytes, _ := json.Marshal(body)
		payload := string(payloadBytes)
		if len(body) == 0 {
			inv.setErr("usage", "no_params", "一个 `--set` 都没给（主控会回 NO_PARAMS）")
			fmt.Fprintf(stderr, "%s: 至少给一个 `--set 键=值`（主控对空体回 `NO_PARAMS`）\n", progName)
			return exitUsage
		}
		if inv.dryRun {
			fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
			fmt.Fprintf(stdout, "  动作     : %s model opts set\n", progName)
			fmt.Fprintf(stdout, "  请求     : PUT %s/api/models/%s/adapter-opts\n", newClient().base, name)
			fmt.Fprintf(stdout, "  请求体   : %s\n", payload)
			fmt.Fprintf(stdout, "  它会动   : 该模型的适配器参数（**编辑后实时生效、不重启**；持久化重启恢复）\n")
			fmt.Fprintf(stdout, "  错误码   : NO_PARAMS · INVALID_PARAMS · ADAPTER_OPTIONS_FAILED · MODEL_NO_ADAPTER（**逐字透传**）\n")
			fmt.Fprintf(stdout, "  来源     : §三 E 族 · §7.1 P11 · 开工单 T-45\n")
			fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
			return exitOK
		}
		if !inv.yes {
			inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
			fmt.Fprintf(stderr, "%s: `model opts set` 是 D2 档（**实时生效**）—— 缺 --yes ⇒ 不执行\n", progName)
			fmt.Fprintf(stderr, "先看计划件：%s model opts set %s --set k=v --dry-run\n", progName, name)
			return exitUsage
		}
		status, respBody, err := putJSON(newClient(), "/api/models/"+name+"/adapter-opts", payload)
		if err != nil {
			inv.setErr("unreachable", "dial_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
			return exitUnreachable
		}
		if status >= 400 {
			code := errorCodeOf(respBody)
			inv.setErr("usage", code, "主控拒绝了这次参数更新")
			fmt.Fprintf(stderr, "%s: 主控拒绝（http_%d）· code=%s\n", progName, status, orDash(code))
			fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
			return exitUsage
		}
		row := map[string]string{"model": name, "updated": payload, "result": strings.TrimSpace(respBody)}
		return listCmd(inv, stdout, stderr, []string{"model", "updated", "result"}, []map[string]string{row})
	default:
		inv.setErr("usage", "bad_action", "`model opts` 的动作只有 get / set")
		fmt.Fprintf(stderr, "%s: `model opts` 的动作只有 `get` / `set`（给的是 %q）\n", progName, action)
		return exitUsage
	}
}

// putJSON —— 另一处写面出口（与 postJSON 同一形状；失败面把 http 状态与原样响应体带回来）。
func putJSON(c *client, path, body string) (int, string, error) {
	return sendJSON(c, "PUT", path, body)
}

// coerceScalar 把 `--set k=v` 的值转成 JSON 标量（数字/布尔认出来，其余当字符串）：
// 这样 `--set temperature=0.8` 送的是 0.8 而不是 "0.8"（主控的适配器参数是数值）。
func coerceScalar(v string) any {
	switch v {
	case "true":
		return true
	case "false":
		return false
	}
	var f float64
	if err := json.Unmarshal([]byte(v), &f); err == nil {
		return f
	}
	return v
}

// ---- core 族：主控自身 ---------------------------------------------------------------------

// cmdCoreStatus —— `zerg core status`（只读 · 投影 `/api/core/status`）。
func cmdCoreStatus(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/core/status", &resp, stderr); rc != exitOK {
		return rc
	}
	keys := []string{}
	for k := range resp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	row := map[string]string{}
	for _, k := range keys {
		row[k] = cell(resp[k])
	}
	display := keys
	if len(display) > 8 {
		display = display[:8]
	}
	return listCmd(inv, stdout, stderr, display, []map[string]string{row})
}

// cmdCoreLogs —— `zerg core logs`：**路由没接 ⇒ 不给结论**（与 `task logs` 同一档）。
func cmdCoreLogs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	_, respBody, err := sendJSON(c, "GET", "/api/logs", "")
	var he *httpError
	if asHTTP(err, &he) && he.status == 404 {
		inv.setErr("blocked", "logs_route_absent", "主控面 /api/logs 未注册（handler 在、路由没接）")
		fmt.Fprintf(stderr, "%s: `/api/logs` ⇒ **404**（处理器在 core/internal/api/handlers.go:966，路由没接）\n", progName)
		fmt.Fprintf(stderr, "⇒ 主控日志**拿不到**，不给结论（退码 8 · kind=blocked · retryable=true）\n")
		fmt.Fprintf(stderr, "不做的：不读 /tmp 文件顶替（§九 M20 O2）\n")
		return exitBlocked
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	fmt.Fprintf(stderr, "%s: /api/logs 有响应（%s）—— 载荷形状未定案，本版只回原样\n", progName, firstLine(respBody))
	return exitOK
}

// coreDaemonDirs —— `core daemon ls` 的扫描面：**本机的服务脚本**（scripts/svc/）。
// 判据（开工单 T-45 ③）：`scripts/svc/` 5 件**逐件可查**。
func coreDaemonScripts(root string) []string {
	dir := filepath.Join(root, "scripts", "svc")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// cmdCoreDaemonLs —— `zerg core daemon ls`（只读 · 本机面）：`scripts/svc/` 逐件 + 声明面。
//
// 为什么并进 `core` 而不新立 `svc` 族（§7.1 `P11` 的推荐口径）：服务族与构建族共用
// 「制品/入口」这一套概念，再立一族就是把同一件事说两遍。
func cmdCoreDaemonLs(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 列不出本机服务脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	scripts := coreDaemonScripts(root)
	if len(scripts) == 0 {
		inv.setErr("blocked", "svc_dir_absent", "scripts/svc/ 读不到或为空")
		fmt.Fprintf(stderr, "%s: `scripts/svc/` 读不到（或空）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	// 声明面（deploy/服务声明.tsv 的第 1 列）——有就读，没有就如实写「未见声明面」
	declared := map[string]string{}
	declPath := filepath.Join(root, "deploy", "服务声明.tsv")
	if b, err := os.ReadFile(declPath); err == nil {
		for i, line := range strings.Split(string(b), "\n") {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue
			}
			f := strings.Split(line, "\t")
			if len(f) >= 2 {
				declared[strings.TrimSpace(f[1])] = strings.TrimSpace(f[0])
			}
		}
	}
	rows := []map[string]string{}
	for _, s := range scripts {
		rel := filepath.Join("scripts", "svc", s)
		row := map[string]string{"name": strings.TrimSuffix(s, ".sh"), "script": rel,
			"declared": "否", "note": ""}
		for path, id := range declared {
			if strings.HasSuffix(path, s) || strings.Contains(path, strings.TrimSuffix(s, ".sh")) {
				row["declared"] = "是"
				row["note"] = "声明件 id=" + id
				break
			}
		}
		if row["declared"] == "否" {
			row["note"] = "声明面没点名它 ⇒ 它是**手动脚本**（按 §十五.3 口径只报、不进自动候选）"
		}
		rows = append(rows, row)
	}
	return listCmd(inv, stdout, stderr, []string{"name", "script", "declared", "note"}, rows)
}

// firstLine 取一段文本的首行（错误回显用）。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
