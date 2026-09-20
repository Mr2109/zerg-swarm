// family_task.go —— `task` 族**全动作面**（§三 D 族 12 个动作名 · §十二 `P-088`）。
//
// 12 个动作名（§3.1 逐字，一个不差）：`diff` `git` `logs` `ls` `move` `pause` `resume` `retry`
// `rm` `show` `submit` `terminate`。其中只读四条（`ls`/`show`/`diff`/`git`）· 危险五条
// （`terminate`/`rm`/`pause`/`resume`/`retry`/`move` —— 见 main.go 的 danger 登记）·
// 写面一条（`submit`）· 端点缺一条（`logs`）。
//
// 三条纪律：
//
//	① **`submit` 的旗标逐条对上 API 的请求体**（`description`/`model`/`priority`/`slice_id`/
//	   `depends_on`/`acceptance`）：名字不另造，映射写在本文件里，别处不再写第二份。
//	② **片的 400 语义透传不翻译**（§4.1 K7 的「薄壳不翻译」同一条纪律）：主控回什么码
//	   （`SLICE_MISSING_ID` / `SLICE_MISSING_ACCEPTANCE` / `SLICE_DEPENDS_CYCLE` /
//	   `SLICE_DEPENDS_DANGLING`）就**逐字**打出来，命令面**不**把它换成自己的话、也不**吞**。
//	③ `diff` / `git` 只跑**本机只读 git 子命令**（`diff --stat` · `rev-parse` · `branch` ·
//	   `rev-list`）—— 每一处调用都写在 `cmdTaskDiff`/`cmdTaskGit` 里（**没有**通用执行面：
//	   命令面不提供「跑任意 git 命令」这种口子）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
)

// ---- 只读：单条任务的现状（`/api/tasks/{id}`）----

// taskDetail 取一条任务的详情（只读 · 一次 GET · 不打第二枪）。
func taskDetail(inv *invocation, id string, stderr io.Writer) (jsonObj, int) {
	c := newClient()
	var d jsonObj
	if rc := fetchInv(inv, c, "/api/tasks/"+id, &d, stderr); rc != exitOK {
		return nil, rc
	}
	if len(d) == 0 {
		fmt.Fprintf(stderr, "%s: 主控返回了空对象 ⇒ 拿不到任务 %q 的详情（不给结论）\n", progName, id)
		inv.setErr("blocked", "empty_payload", "任务详情是空对象")
		return nil, exitBlocked
	}
	return d, exitOK
}

// runGit RO 跑一条白名单内的只读 git 命令（cwd 用 `-C` 指定，不 chdir）。
func runGitRO(workdir string, argv ...string) (string, error) {
	args := append([]string{"-C", workdir}, argv...)
	out, err := exec.Command("git", args...).CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// cmdTaskDiff —— `zerg task diff <id>`：该任务工作树里的**只读** `git diff --stat`。
func cmdTaskDiff(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `task diff` 要一个任务 id（先 `zerg task ls`）\n", progName)
		inv.setErr("usage", "missing_target", "缺任务 id")
		return exitUsage
	}
	id := inv.args[0]
	d, rc := taskDetail(inv, id, stderr)
	if rc != exitOK {
		return rc
	}
	workdir := cell(d["workdir"])
	if workdir == "" {
		inv.setErr("blocked", "workdir_absent", "该任务没有 workdir（阶段未起或已回收）⇒ 没有可比的东西")
		fmt.Fprintf(stderr, "%s: 任务 %q 的载荷里**没有 workdir** ⇒ 不给结论（退码 8）\n", progName, id)
		fmt.Fprintf(stderr, "不做的：不拿别处的目录顶替、不猜一个路径（§九 M20 O2 不偷偷换源）\n")
		return exitBlocked
	}
	out, err := runGitRO(workdir, "diff", "--stat")
	if err != nil && strings.TrimSpace(out) == "" {
		inv.setErr("failed", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 在 %s 里跑 `git diff --stat` 失败：%v\n", progName, workdir, err)
		return exitFail
	}
	row := map[string]string{"id": id, "workdir": workdir, "diff_stat": strings.TrimSpace(out)}
	if row["diff_stat"] == "" {
		row["diff_stat"] = "（工作树无改动）"
	}
	return listCmd(inv, stdout, stderr, []string{"id", "workdir", "diff_stat"}, []map[string]string{row})
}

// cmdTaskGit —— `zerg task git <id>`：该任务工作树的 git 面（分支 / HEAD / 有无未提交改动）。
func cmdTaskGit(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `task git` 要一个任务 id（先 `zerg task ls`）\n", progName)
		inv.setErr("usage", "missing_target", "缺任务 id")
		return exitUsage
	}
	id := inv.args[0]
	d, rc := taskDetail(inv, id, stderr)
	if rc != exitOK {
		return rc
	}
	workdir := cell(d["workdir"])
	if workdir == "" {
		inv.setErr("blocked", "workdir_absent", "该任务没有 workdir ⇒ 没有 git 面可读")
		fmt.Fprintf(stderr, "%s: 任务 %q 的载荷里**没有 workdir** ⇒ 不给结论（退码 8）\n", progName, id)
		return exitBlocked
	}
	branch, _ := runGitRO(workdir, "branch", "--show-current")
	head, _ := runGitRO(workdir, "rev-parse", "HEAD")
	// `git diff --stat` 有输出 ⇒ 有未提交改动（只读；不写任何东西）
	diff, _ := runGitRO(workdir, "diff", "--stat")
	dirty := "false"
	if strings.TrimSpace(diff) != "" {
		dirty = "true"
	}
	// 与 main 的距离（`rev-list --count` · 只读）：判「产物有没有落回主分支」时要用
	ahead, _ := runGitRO(workdir, "rev-list", "--count", "HEAD", "--not", "main")
	row := map[string]string{"id": id, "workdir": workdir, "branch": branch,
		"head": head, "dirty": dirty, "ahead_of_main": strings.TrimSpace(ahead)}
	return listCmd(inv, stdout, stderr,
		[]string{"id", "workdir", "branch", "head", "dirty", "ahead_of_main"}, []map[string]string{row})
}

// cmdTaskLogs —— `zerg task logs <id>`：**任务维度**的日志（`/api/logs/task/{id}`）。
//
// 已红的现况（照实说，不粉饰）：处理器在 `core/internal/api/handlers.go:966` 三条
// （`/api/logs` · `/api/logs/task/{id}` · `/api/logs/agent/{machine}`），但**路由没接**
// ⇒ 现跑 `/api/logs*` 一律 `404`。本命令照实报这一档（`kind=blocked` · 退码 `8`），
// **不**去读本机文件顶替、**不**偷偷直连子端；接上路由（= 改主控源码 + 重启）是拍板项。
func cmdTaskLogs(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `task logs` 要一个任务 id（先 `zerg task ls`）\n", progName)
		inv.setErr("usage", "missing_target", "缺任务 id")
		return exitUsage
	}
	id := inv.args[0]
	c := newClient()
	var resp jsonObj
	err := c.getJSON("/api/logs/task/"+id, &resp)
	if err == nil {
		logs := cell(resp["logs"])
		if logs == "" {
			logs = cell(resp["log"])
		}
		row := map[string]string{"id": id, "available": "true", "detail": logs}
		return listCmd(inv, stdout, stderr, []string{"id", "available", "detail"}, []map[string]string{row})
	}
	var he *httpError
	if asHTTP(err, &he) && he.status == 404 {
		inv.setErr("blocked", "logs_route_absent", "主控面 /api/logs/task/{id} 未注册（handler 在、路由没接）")
		fmt.Fprintf(stderr, "%s: `/api/logs/task/%s` ⇒ **404**（处理器在 core/internal/api/handlers.go:966，路由没接）\n", progName, id)
		fmt.Fprintf(stderr, "⇒ 任务维度的日志**拿不到**，不给结论（退码 8 · kind=blocked · retryable=true）\n")
		fmt.Fprintf(stderr, "不做的：不读本机文件顶替、不偷偷直连子端（§九 M20 O2）\n")
		return exitBlocked
	}
	inv.setErr("failed", "logs_unreadable", err.Error())
	fmt.Fprintf(stderr, "%s: 取任务日志失败：%v\n", progName, err)
	return exitFail
}

// asHTTP 把错误认成 HTTP 错误（本文件里只用于把 404 与别的错分开）。
func asHTTP(err error, out **httpError) bool {
	if he, ok := err.(*httpError); ok {
		*out = he
		return true
	}
	return false
}

// ---- 写面：`task submit`（D2 · 全旗标对得上 API 请求体）----

// taskSubmitCodes —— 片的 400 语义**逐字**列表（**透传不翻译**：主控回什么码就打什么码）。
// 出处：`core/internal/api/routes.go:128-135` 的 `/api/tasks` POST 声明。
var taskSubmitCodes = []string{"SLICE_MISSING_ID", "SLICE_MISSING_ACCEPTANCE",
	"SLICE_DEPENDS_CYCLE", "SLICE_DEPENDS_DANGLING"}

// taskSubmitBody 把旗标映射成 API 请求体（`description`/`model`/`priority`/`slice_id`/
// `depends_on`/`acceptance`）。**只发用户真给了的键**：`acceptance` 的三态
// （没声明 / 显式空 / 有内容）在 API 那边是三件事（`nil` / `[]` / `[…]`）⇒ 这里不许合并。
func taskSubmitBody(inv *invocation) (string, error) {
	body := map[string]any{}
	if inv.desc != "" {
		body["description"] = inv.desc
	}
	if inv.modelWant != "" {
		body["model"] = inv.modelWant
	}
	if inv.priority != "" {
		p := 0
		if _, err := fmt.Sscanf(inv.priority, "%d", &p); err != nil {
			return "", fmt.Errorf("--priority 要是整数（给的是 %q）", inv.priority)
		}
		body["priority"] = p
	}
	if inv.sliceID != "" {
		body["slice_id"] = inv.sliceID
	}
	if len(inv.dependsOn) > 0 {
		body["depends_on"] = inv.dependsOn
	}
	if inv.acceptanceDeclared {
		body["acceptance"] = inv.acceptance // 显式空数组也发（与「没写」是两件事）
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// cmdTaskSubmit —— `zerg task submit`（D2 写面 · 本版**开放**：这是 S5 的落地）。
//
// 三态：`--dry-run` 出计划件（零副作用 · rc=0）· 缺 `--yes` ⇒ 2（fail-closed）· 齐了才 POST。
// 失败面：主控的 **400 码逐字透传**（`taskSubmitCodes`），命令面不替换、不吞（§4.1 K7 的口径）。
// rc 只从契约退码表取：`400` 这类「你给的东西不合法」= 用法错 `2`。
func cmdTaskSubmit(inv *invocation, stdout, stderr io.Writer) int {
	body, err := taskSubmitBody(inv)
	if err != nil {
		inv.setErr("usage", "bad_flag_value", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return exitUsage
	}
	if inv.desc == "" {
		inv.setErr("usage", "missing_desc", "缺 --desc（任务描述是自包含的，必须有）")
		fmt.Fprintf(stderr, "%s: `task submit` 缺 `--desc <任务描述>`（§4.1 K7：位置参数不够，要显式旗标）\n", progName)
		return exitUsage
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s task submit\n", progName)
		fmt.Fprintf(stdout, "  危险档   : D2（有影响但可逆 · 要 --yes）\n")
		fmt.Fprintf(stdout, "  请求     : POST %s/api/tasks\n", newClient().base)
		fmt.Fprintf(stdout, "  请求体   : %s\n", body)
		fmt.Fprintf(stdout, "  片字段   : 声明了任一片字段 ⇒ 该任务按「片」过挂板校验；缺失由主控判\n")
		fmt.Fprintf(stdout, "  400 码   : %s（**逐字透传、不翻译** —— 出处 core/internal/api/routes.go）\n",
			strings.Join(taskSubmitCodes, " · "))
		fmt.Fprintf(stdout, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
		fmt.Fprintf(stdout, "  来源     : §三 D 族 · §十二 P-088 · 开工单 T-44\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `task submit` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s task submit --desc '…' --dry-run\n", progName)
		return exitUsage
	}
	status, respBody, err := postJSON(newClient(), "/api/tasks", body)
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 提交失败（打不到主控）：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次提交（http_%d）—— 看 `zerg help credentials`\n", progName, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		inv.setErr("usage", code, "主控拒绝入队")
		// ★ 透传不翻译：主控的码**逐字**打出来；命令面只补一句「它出自谁」。
		fmt.Fprintf(stderr, "%s: 主控拒绝入队（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		if code != "" {
			fmt.Fprintf(stderr, "（码**逐字透传、命令面不翻译** —— 片的 400 语义见 §十二 P-088）\n")
		}
		return exitUsage
	}
	id := idOf(respBody)
	row := map[string]string{"id": id, "status": "submitted", "request": body}
	return listCmd(inv, stdout, stderr, []string{"id", "status", "request"}, []map[string]string{row})
}

// postJSON —— 一处 POST（写面唯一出口；失败面把 http 状态与**原样响应体**一起带回来）。
func postJSON(c *client, path, body string) (int, string, error) {
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Auth-Token", c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

// errorCodeOf 从主控的错误响应里取它的码（`code` / `error` 两种写法都认；取不到 ⇒ 空）。
func errorCodeOf(body string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return ""
	}
	for _, k := range []string{"code", "error", "kind"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// idOf 从创建成功的响应里取任务 id（`id` 优先，其次 `task_id`）。
func idOf(body string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return ""
	}
	for _, k := range []string{"id", "task_id"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// taskFamilyNames —— 本族的 12 个动作名（**判据面**：门⑪ 的矩阵与帮助都从命令树出，
// 这里只是把「12 条齐不齐」写出来给复核用）。
func taskFamilyNames() []string {
	names := []string{}
	for _, c := range catalog() {
		if len(c.path) == 2 && c.path[0] == "task" {
			names = append(names, c.path[1])
		}
	}
	sort.Strings(names)
	return names
}
