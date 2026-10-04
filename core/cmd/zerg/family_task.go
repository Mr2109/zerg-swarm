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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	args := append([]string{"-C", workdir, "-c", "core.quotepath=false"}, argv...)
	out, err := exec.Command("git", args...).CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// runGitRONoLock 与 `runGitRO` 同形，但走 `--no-optional-locks`（**一个可选锁都不取**）——
// 只读的**探读**面用它（照同族 `family_repo.go` 的 `gitRunNoLock`）：连 `git status` 的索引刷新
// 都不许发生（本命令是只读面，不许因为探读而在工作树里落任何痕迹）。
func runGitRONoLock(workdir string, argv ...string) (string, error) {
	args := append([]string{"-C", workdir, "-c", "core.quotepath=false", "--no-optional-locks"}, argv...)
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
	// laneEV：git 读不到**不许再谎报「干净」**。照同族已治件 `family_repo.go` 的治法定型
	// （照它，禁自创）：读不到就**自报 + 不给结论（退码 8）**，且成因**分开报** ——
	//   ① 仓根下没有 `.git` ⇒ 这不是一个 git 工作树（与「git 命令失败」**不是一回事**）；
	//   ② `git status` 跑不动 ⇒ git 面读不到；
	//   ③ HEAD 读不到：同一份 porcelain 带「未出生」标记 ⇒ 空仓（另档），否则 `rev-parse` 读不到。
	// 探读一律走 `--no-optional-locks`（一个可选锁都不取 ⇒ 连索引刷新都不发生）。
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		inv.setErr("blocked", "not_a_git_repo", "任务工作树里没有 .git")
		fmt.Fprintf(stderr, "%s: 任务 %q 的 workdir %s 下没有 .git ⇒ 这不是一个 git 工作树（不给结论 · 退码 8）\n",
			progName, id, workdir)
		return exitBlocked
	}
	porcelain, perr := runGitRONoLock(workdir, "status", "--porcelain=v1", "--branch")
	if perr != nil {
		inv.setErr("blocked", "git_status_failed", perr.Error())
		fmt.Fprintf(stderr, "%s: `git status` 在 %s 跑不动 ⇒ 读不到工作树 ⇒ 不给结论（退码 8）：%v\n",
			progName, workdir, perr)
		return exitBlocked
	}
	branch, _ := runGitRO(workdir, "branch", "--show-current")
	head, herr := runGitRO(workdir, "rev-parse", "HEAD")
	if herr != nil {
		if strings.HasPrefix(porcelain, repoUnbornHeadMark) {
			inv.setErr("blocked", "git_head_unborn", herr.Error())
			fmt.Fprintf(stderr, "%s: 任务 %q 的工作树**还没有任何提交**（HEAD 未出生）⇒ 读不到 HEAD ⇒ 不给结论（退码 8）\n",
				progName, id)
			return exitBlocked
		}
		inv.setErr("blocked", "git_rev_parse_failed", herr.Error())
		fmt.Fprintf(stderr, "%s: `rev-parse HEAD` 在 %s 跑不动 ⇒ 读不到 HEAD ⇒ 不给结论（退码 8）：%v\n",
			progName, workdir, herr)
		return exitBlocked
	}
	// `git diff --stat` 有输出 ⇒ 有未提交改动（只读；不写任何东西）。
	// ★ 本笔要治的**谎报**就在下面这一处：旧写法 `diff, _ := …` 把错丢掉、随后默认 `dirty="false"`
	//   ⇒ git 读不到时任务面报「干净」。现在 git 读不到 ⇒ **不给结论**（退码 8），绝不退化成「干净」。
	diff, derr := runGitRO(workdir, "diff", "--stat")
	if derr != nil {
		inv.setErr("blocked", "git_diff_failed", derr.Error())
		fmt.Fprintf(stderr, "%s: `git diff --stat` 在 %s 跑不动 ⇒ 读不到工作树改动 ⇒ 不给结论（退码 8）：%v\n",
			progName, workdir, derr)
		return exitBlocked
	}
	dirty := "false"
	if strings.TrimSpace(diff) != "" {
		dirty = "true"
	}
	// 与 main 的距离（`rev-list --count` · 只读）：判「产物有没有落回主分支」时要用。
	// ★ 这一格**不**跟着 fail-closed：默认分支不是 `main`（或仓里还没有 `main`）时它本来就跑不动，
	//   那时给空串是既有口径（判「有没有落回主分支」由调用方自己判）⇒ **不改**，免得把正常仓判死。
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
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
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
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
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
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		// 主控**回话了**（非 2xx）⇒ 不是「打不到」：走下面的「逐字透传」分支。
		status, respBody, err = he.status, he.body, nil
	}
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

// postJSON —— 写面出口（POST）；失败面把 http 状态与**原样响应体**一起带回来。
func postJSON(c *client, path, body string) (int, string, error) {
	return sendJSON(c, http.MethodPost, path, body)
}

// sendJSON —— **两处写面出口共用的一处**（POST / PUT；body 为空则不带体）。
// 失败面一律把 http 状态与**原样响应体**带回来（「逐字透传」要能拿到原文）。
func sendJSON(c *client, method, path, body string) (int, string, error) {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return 0, "", err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Auth-Token", c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		// 让调用方能按 http 状态分档（404 与 400 的处置不同）
		return resp.StatusCode, string(b), &httpError{path: path, status: resp.StatusCode, body: string(b)}
	}
	return resp.StatusCode, string(b), nil
}

// errorCodeOf 从主控的错误响应里取它的码 —— **两种真实现都认**（取不到 ⇒ 空串，不猜）：
//
//	① 顶层：`{"code":"SLICE_MISSING_ID"}`（`/api/tasks` POST 那一族）
//	② 嵌套：`{"error":{"message":"…","type":"MODEL_NO_ADAPTER"}}`（`writeErrorCode` 那一族）
//
// ★ 为什么两处都要认：命令面要的是「**逐字透传**主控的码」（§4.1 K7），只认一种写法
// 就会把另一种写成空 —— 那样的「透传」是假的。
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
	if e, ok := m["error"].(map[string]any); ok {
		for _, k := range []string{"type", "code"} {
			if v, ok := e[k].(string); ok && v != "" {
				return v
			}
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

// ---- 写面：`task cancel`（D2 ＋ `--confirm=<任务 id>` ＋ `--yes` · 本版**已开放**）----

// taskCancelReasons —— `--reason` 的闭集（5 值 · **逐字**对齐主控）。
// 出处：`core/internal/api/master_scheduler.go` 的 `closedReasonSet`（5 值 · 顺序逐字相同）；
// 设计-任务退场正门-v0.2 §11.5（小闭集必填 + `other` 兜底）。命令面**照它原样**列，不自造第二份闭集。
var taskCancelReasons = []string{"superseded", "obsolete", "duplicate", "abandoned", "other"}

// taskCancelReasonOK 判一个取值在不在闭集内（**大小写敏感 · 逐字相同**；不 trim、不猜）。
func taskCancelReasonOK(v string) bool {
	for _, r := range taskCancelReasons {
		if r == v {
			return true
		}
	}
	return false
}

// taskCancelNotFound —— 从主控的 **400** 响应里认「任务不存在」这一档（T-14：不存在 ⇒ 退码 2）。
// 主控这一档的码恒为 `TASK_CANCEL_FAILED`（与「在跑 / 等恢复 / 理由非法」同一个码，
// `handlers.go` 的 `writeErrorCode(…, "TASK_CANCEL_FAILED", err.Error())`），唯一区分是 message 里的
// `not found`（`master_scheduler.go` 的 `task %s not found`）⇒ **只认 error.message 那一个串**，
// 不拿整份原文兜底（否则会把路由缺失的 `404 page not found` 误认成「任务不存在」）。
func taskCancelNotFound(body string) bool {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return false
	}
	e, ok := m["error"].(map[string]any)
	if !ok {
		return false
	}
	msg, _ := e["message"].(string)
	return strings.Contains(strings.ToLower(msg), "not found")
}

// cmdTaskCancel —— `zerg task cancel <任务 id>`（D2 写面 · 本版**已开放**）。
//
// 三态：`--dry-run` 出计划件（零副作用 · rc=0）· 缺确认档 ⇒ 2（fail-closed）· 齐了才 POST。
// 幂等三成因（T-14 · 设计 §11.2）：id 不存在 ⇒ 2（detail=`not_found`）· 已 `cancelled` ⇒ 0
// （`already_cancelled`）· 已 `done`/`failed` ⇒ 0（`already_terminal`）· 可取消 ⇒ 0。
// ★ 铁律：**状态差异走 stdout 结构化字段、不走退码**；★ **单 id 绝不给 8**（本函数不返回 exitBlocked）。
// 与 `cmdTaskSubmit` 同形：互斥走**唯一**一处 `dryRunYesConflict`（本命令真判的互斥就这一条）；
// 请求走 `postJSON`；主控失败码**逐字透传**不翻译。
func cmdTaskCancel(inv *invocation, stdout, stderr io.Writer) int {
	// ① 两道确认档自相矛盾（`--dry-run` 与 `--yes` 同给）⇒ 用法错 2（本命令唯一的互斥真判）。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// ② 目标：位置参数一个任务 id。
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺任务 id")
		fmt.Fprintf(stderr, "%s: `task cancel` 要一个任务 id（先 `zerg task ls`）\n", progName)
		return exitUsage
	}
	id := inv.args[0]
	// ③ 结案理由：必填 · 闭集 5 值 · 无默认值（缺 / 出闭集都逐字印闭集）。
	reason := inv.flagVal("--reason")
	if reason == "" {
		inv.setErr("usage", "missing_reason", "缺 --reason（结案理由必填 · 闭集 5 值）")
		fmt.Fprintf(stderr, "%s: `task cancel` 缺 `--reason <结案理由>`（小闭集必填 · 不给默认值 · 设计 §11.5）\n", progName)
		fmt.Fprintf(stderr, "闭集（逐字）: %s\n", strings.Join(taskCancelReasons, " · "))
		return exitUsage
	}
	if !taskCancelReasonOK(reason) {
		inv.setErr("usage", "bad_reason", "结案理由不在闭集内")
		fmt.Fprintf(stderr, "%s: `--reason` 的值 %q **不在闭集内** ⇒ 用法错 2（不给结论）\n", progName, reason)
		fmt.Fprintf(stderr, "闭集（逐字）: %s\n", strings.Join(taskCancelReasons, " · "))
		return exitUsage
	}
	// ④ `--dry-run`：只出计划件 · 零副作用 · rc=0（照同族 `cmdGuarded` 的档序：干跑先于确认档）。
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s task cancel\n", progName)
		fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 · 要 --confirm=<任务 id> 与 --yes）\n", dangerD2)
		fmt.Fprintf(stdout, "  目标     : %s\n", id)
		fmt.Fprintf(stdout, "  结案理由 : %s\n", reason)
		if inv.hasFlag("--note") {
			fmt.Fprintf(stdout, "  备注     : %s\n", inv.flagVal("--note"))
		}
		fmt.Fprintf(stdout, "  请求     : POST %s/api/tasks/%s/cancel?reason=%s\n", newClient().base, id, reason)
		fmt.Fprintf(stdout, "  它会动   : 把该任务从 queue 摘出 → 置 status=cancelled → 落 history（可逆面：队列位次；在飞的件本版不给撤）\n")
		fmt.Fprintf(stdout, "  结局字段 : cancelled / already_cancelled / already_terminal（三值互斥 · 走 stdout 不走退码 · T-14）\n")
		fmt.Fprintf(stdout, "  来源     : 设计-任务退场正门-v0.2 §4.1/§11.2 · 开工单 T-11…T-14/T-17\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// ⑤ 危险档确认面（D2）：`--confirm=<任务 id>` 必带，且值必须与位置参数的任务 id **逐字相同**。
	// 口径逐字照同族四条（`task move/pause/resume/retry` → `guard.go`）：缺 / 值不匹配 ⇒ 用法错 2（fail-closed）。
	if !inv.confirmGiven || inv.confirm != id {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，目标是 %q）⇒ 不执行", inv.confirm, id)
		if !inv.confirmGiven {
			msg = fmt.Sprintf("`task cancel` 是 D2 档 —— **缺 --confirm=%s ⇒ 不执行**", id)
		}
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与位置参数的任务 id **逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	// ⑥ `--yes`（fail-closed：缺就拒 —— 与 `cmdTaskSubmit` 同一条）。
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `task cancel` 是 D2 档 —— **缺 --yes ⇒ 不执行**（§九 M3 C2 fail-closed）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s task cancel %s --reason %s --confirm=%s --dry-run\n", progName, id, reason, id)
		return exitUsage
	}
	// ⑦ 真写：POST /api/tasks/{id}/cancel?reason=…[&note=…]（handler 从 query 取参）。
	// ★ `--note` **无默认值**：未声明就不发这个键（「给过」与「没给」是两件事 —— 与 `--ttl` 同口径）。
	q := url.Values{}
	q.Set("reason", reason)
	if inv.hasFlag("--note") {
		q.Set("note", inv.flagVal("--note"))
	}
	reqPath := "/api/tasks/" + url.PathEscape(id) + "/cancel?" + q.Encode()
	status, respBody, err := postJSON(newClient(), reqPath, "")
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		// 主控**回话了**（非 2xx）⇒ 不是「打不到」：走下面的分档。
		status, respBody, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 撤下失败（打不到主控）：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次撤下（http_%d）—— 看 `zerg help credentials`\n", progName, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		detail := code
		// T-14：id 不存在 ⇒ 退码 2（detail=`not_found`）。**只在主控的 400 档**上认 ——
		// 404 是「这条路由没接」（本版主控未重编/未重启时就是这样），与本档**不是一回事**，不许混。
		if status == 400 && taskCancelNotFound(respBody) {
			detail = "not_found"
		}
		inv.setErr("usage", detail, "主控拒绝撤下")
		fmt.Fprintf(stderr, "%s: 主控拒绝撤下（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		if detail == "not_found" {
			fmt.Fprintf(stderr, "（任务 %q **不存在** ⇒ 退码 2 · detail=not_found · T-14）\n", id)
		}
		if status == 404 {
			fmt.Fprintf(stderr, "⇒ 主控**没有这条路由**（`POST /api/tasks/{id}/cancel` 未接）—— 需重编 + 重启主控后才有（开工单刀 1 / 刀 7）\n")
			fmt.Fprintf(stderr, "（此时**不给结论**：退码 2 · 不给 8 —— 单 id 绝不给 8 · T-14）\n")
		}
		return exitUsage
	}
	// ⑧ 成功（2xx）：三种结局都落这里，靠响应体的 outcome 派生三个互斥布尔（状态差异走 stdout）。
	var resp jsonObj
	_ = json.Unmarshal([]byte(respBody), &resp)
	outcome := cell(resp["outcome"])
	switch outcome {
	case "cancelled", "already_cancelled", "already_terminal":
	default:
		// 2xx 却给不出三值闭集里的结局 —— 主控契约面坏了：不给结论，但**不**用 8（单 id 绝不给 8）。
		inv.setErr("upstream_error", "bad_cancel_outcome", "主控回了 2xx 但 outcome 不在三值闭集")
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体里的 outcome=%q 不在三值闭集 ⇒ 不给结论（退码 1）\n", progName, outcome)
		return exitFail
	}
	tid := cell(resp["task_id"])
	if tid == "" {
		tid = cell(resp["id"])
	}
	if tid == "" {
		tid = id
	}
	row := map[string]string{
		"id":                tid,
		"outcome":           outcome,
		"cancelled":         fmt.Sprintf("%t", outcome == "cancelled"),
		"already_cancelled": fmt.Sprintf("%t", outcome == "already_cancelled"),
		"already_terminal":  fmt.Sprintf("%t", outcome == "already_terminal"),
	}
	// meta.changed（§九 M4）：真撤 ⇒ true；两种幂等结局（no-op）⇒ false。
	changed := outcome == "cancelled"
	inv.changed = &changed
	return listCmd(inv, stdout, stderr,
		[]string{"id", "outcome", "cancelled", "already_cancelled", "already_terminal"},
		[]map[string]string{row})
}

// ---- sched 族: 暂停派活 / 并发旋钮 / 调度器状态（缺口 GAP-20260927-210 · GAP-20260929-26）----
//
// 三条纪律：
//  ① `show` 是**只读**面（GET /api/sched）—— 调度器状态 + 在跑条目的领取/心跳面；
//  ② `pause`/`resume` 是「暂停**派活**」一等入口（≠ 终止在跑: 要停在跑的走 `task terminate`）；
//  ③ `set <N>` 给并发度一枚显式旋钮（N 走位置参数，免再往 main.go 的名字表加旗标）。
// ★ 命令面只做出口: 数值口径、持久化、派生有效并发全在服务端（master_scheduler.go）。

// schedStateRows 把一份调度器状态载荷摊成摘要行 + 领取行（单一表格 + 单一 --json）。
func schedStateRows(resp jsonObj) []map[string]string {
	paused := cell(resp["dispatch_paused"]) == "true"
	status := "running"
	if paused {
		status = "paused"
	}
	rows := []map[string]string{{
		"kind":          "scheduler",
		"id":            "（调度器）",
		"status":        status,
		"model":         "",
		"concurrency":   cell(resp["effective_max"]) + "/" + cell(resp["max_concurrent"]),
		"queued":        cell(resp["queued"]),
		"waiting_retry": cell(resp["waiting_retry"]),
		"claimed_by":    "",
		"claimed_at":    "",
		"heartbeat_at":  "",
	}}
	for _, it := range asList(resp["claimed"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		r := project(o, []string{"id", "status", "model", "claimed_by", "claimed_at", "heartbeat_at"})
		r["kind"] = "claimed"
		r["concurrency"] = ""
		r["queued"] = ""
		r["waiting_retry"] = ""
		rows = append(rows, r)
	}
	return rows
}

// renderSchedState 渲染一份调度器状态载荷（show / pause / resume / set 共用出口）。
func renderSchedState(inv *invocation, stdout, stderr io.Writer, resp jsonObj) int {
	return listCmd(inv, stdout, stderr,
		[]string{"kind", "id", "status", "model", "concurrency", "queued", "waiting_retry",
			"claimed_by", "claimed_at", "heartbeat_at"},
		schedStateRows(resp))
}

// schedGet 取调度器状态（GET /api/sched）。
func schedGet(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/sched", &resp, stderr); rc != exitOK {
		return rc
	}
	return renderSchedState(inv, stdout, stderr, resp)
}

// schedPost 打一发 sched 写面（pause/resume/set 共用）—— 失败面把 http 状态与主控原文一起带回来。
func schedPost(inv *invocation, stdout, stderr io.Writer, path string) int {
	status, body, err := postJSON(newClient(), path, "")
	var he *httpError
	if err != nil && asHTTP(err, &he) {
		status, body, err = he.status, he.body, nil
	}
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v\n", progName, err)
		return exitUnreachable
	}
	if status == 401 || status == 403 {
		inv.setErr("unauthenticated", fmt.Sprintf("http_%d", status), "未认证")
		fmt.Fprintf(stderr, "%s: 主控拒了这次调度变更（http_%d）—— 看 `zerg help credentials`\n", progName, status)
		return exitAuth
	}
	if status >= 400 {
		code := errorCodeOf(body)
		inv.setErr("usage", code, "主控拒绝调度变更")
		fmt.Fprintf(stderr, "%s: 主控拒绝调度变更（http_%d）· code=%s\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(body))
		if status == 404 {
			fmt.Fprintf(stderr, "⇒ 主控**没有这条路由**（`POST %s` 未接）—— 需重编 + 重启主控后才有\n", path)
		}
		return exitUsage
	}
	var resp jsonObj
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		inv.setErr("upstream_error", "bad_sched_payload", err.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	return renderSchedState(inv, stdout, stderr, resp)
}

// cmdSchedShow —— `zerg sched show`: 只读调度器状态（缺口 GAP-20260927-210 ② / GAP-20260929-26 ①）。
func cmdSchedShow(inv *invocation, stdout, stderr io.Writer) int {
	return schedGet(inv, stdout, stderr)
}

// cmdSchedPause —— `zerg sched pause`: 暂停派活（不再起新的；在跑的不动）。
func cmdSchedPause(inv *invocation, stdout, stderr io.Writer) int {
	return schedPost(inv, stdout, stderr, "/api/sched/pause")
}

// cmdSchedResume —— `zerg sched resume`: 恢复派活（回到设定并发并立即派发积压）。
func cmdSchedResume(inv *invocation, stdout, stderr io.Writer) int {
	return schedPost(inv, stdout, stderr, "/api/sched/resume")
}

// cmdSchedSet —— `zerg sched set <N>`: 设并发上限（显式旋钮 —— 缺口 GAP-20260929-26 ②）。
func cmdSchedSet(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺并发上限 N")
		fmt.Fprintf(stderr, "%s: `sched set` 要一个非负整数 N（如 `%s sched set 2`）\n", progName, progName)
		return exitUsage
	}
	n := strings.TrimSpace(inv.args[0])
	if _, err := strconv.Atoi(n); err != nil {
		inv.setErr("usage", "bad_flag_value", "并发上限要非负整数")
		fmt.Fprintf(stderr, "%s: `sched set` 的 N 要非负整数（给的是 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	return schedPost(inv, stdout, stderr, "/api/sched/set?max="+url.QueryEscape(n))
}
