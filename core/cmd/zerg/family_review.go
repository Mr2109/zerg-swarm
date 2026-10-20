// family_review.go —— `zerg review`（脑确认复查 · **归入命令面** · 批 B3）。
//
// 源件 = 独立入口 `core/cmd/zerg-review/main.go`（178 行 · 位置参数两枚 + 一枚动作 + 一枚旗标
// `-repo`）。本件是那一件的**逐面搬**：读 `docs/issues/<id>.md` → 判状态必须是 `waiting_review`
// → `approve`（分支 merge 回 main ＋ issue → done）/ `reject`（丢分支 ＋ issue → retry）/
// `list`（列在等脑确认的单）。
//
// 形态（位置参数与旗标**逐字**保留源件 · 只把 Go `flag` 的单横线写成命令面的双横线）：
//
//	zerg review <workdir> <issue-id|list> [approve|reject] [--repo <路径>]
//
//	<workdir>        工作目录（读 issue = `<workdir>/docs/issues/<id>.md`）
//	<issue-id|list>  issue id；`list` = 列全部在等脑确认的单（不收动作）
//	approve | reject 动作（给 issue-id 时必填 —— 源件 `approve or reject expected`）
//	--repo <路径>    开发仓库路径（DevMode 架构分离：merge 用 repoDir、读 issue 用 workDir；
//	                 不给 ⇒ 同 `<workdir>`，与源件缺省一致）
//
// 口径（与源件逐条一致，✗ 不改一处语义）：
//
//	· 状态门：`- **状态**:` 不是 `waiting_review` ⇒ 拒做（源件同档：`⚠️ issue status=%s`）。
//	· `approve`：issue 里没有 `- **分支**:` ⇒ 拒做（源件同档：`cannot merge`）；
//	  否则 `git checkout main` → `git merge --no-ff <分支> -m 'merge: <分支> → main (PR <id>)'`
//	  → `git branch -d <分支>`，三步全过才 `waiting_review → done` 并写回 issue。
//	· `reject`：有分支才丢（`git checkout main` + `git branch -D <分支>`），然后
//	  `waiting_review → retry` 并写回 issue。
//	· 状态流转一律走 `agent.TransitionIssue`（唯一真源；本件不自造状态机）。
//	· `list`：逐件读 `docs/issues/*.md`，只列正文含 `- **状态**: waiting_review` 的，
//	  格式 `  <id>: <任务>`；一件没有 ⇒ `  (no tasks awaiting approval)`。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0  approve/reject 做成 / `list` 出表
//	2  用法错（缺位置参数 · 缺动作 · 动作不在 `approve|reject` · 多余位置参数 · `--repo` 裸给）
//	1  issue 读不到 / 状态不是 `waiting_review` / merge 失败 / 状态流转失败 / issue 写不回
//
// 与源件的**照实差异**（六处 · 逐条写清，别处不猜）：
//
//	① 用法类退码 **1 → 2**：源件缺参 / 缺动作 / 动作认不得一律 `os.Exit(1)`；命令面统一走
//	   `exitcodes.go` 的 usage 档（同批 B1 `trace`、B2 `sched run` 的同一条归一）。
//	② **多余位置参数 / `list` 带动词 ⇒ 2**：源件 `os.Args[3]` 之后的多余参数**静默忽略**、
//	   `list` 后面的动词也照样只列表；命令面按 §4.1 `K14`（`Q-154`）不许静默吞 ⇒ 退 2 说明。
//	③ **执行面不再拼 shell**：源件把仓库路径与分支名拼进 `sh -c "cd <repo> && git …"`；本件按
//	   逐参 `git -C <repo> …` 传（**同一组 git 子命令、同顺序、同判词**）。行为差异只出现在
//	   「路径/分支名里含 shell 元字符」时 —— 源件会被 shell 展开/注入，本件按字面执行。
//	④ **写盘错误不再静默**：源件 `os.WriteFile(issuePath, …, 0o644)` 不看返回值（写失败也打
//	   `✅ merge confirmed`）；本件写失败 ⇒ stderr 判词 + 退 1（✗ 不许假绿）。
//	⑤ **丢分支的失败不再静默**：源件 `discardBranch` 把 `runShell` 的返回值丢掉；本件仍按源件
//	   继续标 `retry`（**退码与状态不变**），但失败那一行逐字进 stderr（`⚠️` 前缀）。
//	⑥ **stdout / stderr 分工**（§4.1 `K4`：人面进度不进 stdout）：结果面
//	   （`✅ merge confirmed…` / `❌ rejected…` / `📋` 表）走 **stdout**；进度面
//	   （`🔀 …` / `🗑 …`）与全部错误判词走 **stderr**。源件一律打 stdout。
//
// ★ 双轨期：`core/cmd/zerg-review` 二进制**一字不动**（本件只搬逻辑、不删入口）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// reviewIssuesDir —— issue 单的相对目录（源件 `filepath.Join(workDir, "docs", "issues", …)` 的同源）。
const reviewIssuesDir = "docs/issues"

// reviewCfg —— 解析后的调用（`action == ""` = `list` 档；`repoDir` 已回落到 `workDir`）。
type reviewCfg struct {
	workDir string
	issueID string
	action  string
	repoDir string
}

// reviewUsageLine —— 用法串（与命令树 `usage` 字段**逐字同源**：一处写错两处就分家）。
const reviewUsageLine = "zerg review <workdir> <issue-id|list> [approve|reject] [--repo <路径>]"

// reviewParse —— 位置参数 + `--repo` → 配置（语义校验只在本处）。
//
// 退码：`0` 配置齐；`2` 用法错（缺参 / 缺动作 / 动作认不得 / 多余位置参数 / `--repo` 裸给）。
// 判序照源件：先看 `list`（它不要动作），再看动作齐不齐。
func reviewParse(inv *invocation, stderr io.Writer) (reviewCfg, int) {
	var cfg reviewCfg
	if len(inv.args) < 2 || strings.TrimSpace(inv.args[0]) == "" || strings.TrimSpace(inv.args[1]) == "" {
		inv.setErr("usage", "review_missing_args", "缺位置参数 <workdir> <issue-id|list>")
		fmt.Fprintf(stderr, "%s: 缺位置参数 —— 本命令要 `<workdir> <issue-id|list> [approve|reject]`\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
		return cfg, exitUsage
	}
	cfg.workDir = inv.args[0]
	cfg.issueID = inv.args[1]
	cfg.repoDir = cfg.workDir // 源件缺省：非 DevMode ⇒ merge 也用工作目录

	// `--repo` 是**取值**旗标（源件 `-repo`）：裸给/空值 ⇒ 用法错 2（值旗标纪律，`GAP-20260928-151`）。
	if inv.hasFlag("--repo") {
		repo := strings.TrimSpace(inv.flagVal("--repo"))
		if repo == "" {
			inv.setErr("usage", "review_repo_empty", "`--repo` 裸给或空值")
			fmt.Fprintf(stderr, "%s: `--repo` 要一个**仓库目录**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = 同 <workdir>）\n", progName)
			fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
			return cfg, exitUsage
		}
		cfg.repoDir = repo
	}

	if cfg.issueID == "list" {
		// 源件：`list` 之后的东西一律不看（差异 ②：这里不再静默吞）。
		if len(inv.args) > 2 {
			inv.setErr("usage", "review_list_takes_no_action", "`list` 不收动作")
			fmt.Fprintf(stderr, "%s: `list` 不收动作（现得 %q）—— 列在等脑确认的单就写 `%s <workdir> list`\n",
				progName, inv.args[2], progName)
			fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
			return cfg, exitUsage
		}
		return cfg, exitOK
	}

	if len(inv.args) < 3 {
		inv.setErr("usage", "review_missing_action", "缺动作 approve|reject")
		fmt.Fprintf(stderr, "%s: approve or reject expected（给 issue id 时动作必填）\n", progName)
		fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
		return cfg, exitUsage
	}
	if len(inv.args) > 3 {
		inv.setErr("usage", "review_extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: 多余位置参数 %q（`%s review` 只收 <workdir> <issue-id> [approve|reject]）\n",
			progName, inv.args[3], progName)
		fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
		return cfg, exitUsage
	}
	cfg.action = inv.args[2]
	if cfg.action != "approve" && cfg.action != "reject" {
		inv.setErr("usage", "review_bad_action", "动作不在 approve|reject 闭集")
		fmt.Fprintf(stderr, "%s: unknown action (approve/reject)（现得 %q）\n", progName, cfg.action)
		fmt.Fprintf(stderr, "用法: %s\n", reviewUsageLine)
		return cfg, exitUsage
	}
	return cfg, exitOK
}

// cmdReview —— `zerg review <workdir> <issue-id|list> [approve|reject] [--repo <路径>]`。
func cmdReview(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := reviewParse(inv, stderr)
	if rc != exitOK {
		return rc
	}
	return reviewRun(inv, cfg, stdout, stderr)
}

// reviewRun —— 本体（`approve` / `reject` / `list` 三档）。
//
// `inv` 必须是 **dispatch 那一个**（`error.detail` 记在它身上，包封才带得出去 ——
// 另开一枚 `&invocation{}` = 错记了、包封里读不到）。
func reviewRun(inv *invocation, cfg reviewCfg, stdout, stderr io.Writer) int {
	if cfg.action == "" {
		return reviewListWaiting(inv, cfg.workDir, stdout, stderr)
	}

	issuePath := filepath.Join(cfg.workDir, reviewIssuesDir, cfg.issueID+".md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		inv.setErr("failed", "review_issue_unreadable", "issue 单读不到")
		fmt.Fprintf(stderr, "%s: ❌ failed to read issue: %v\n", progName, err)
		return exitFail
	}
	text := string(content)

	branch := reviewExtractField(text, "- **分支**:")
	status := reviewExtractField(text, "- **状态**:")
	if status != agent.StatusWaitReview {
		inv.setErr("failed", "review_not_waiting_review", "issue 状态不是 waiting_review")
		fmt.Fprintf(stderr, "%s: ⚠️ issue status=%s (waiting_review required)\n", progName, status)
		return exitFail
	}

	switch cfg.action {
	case "approve":
		if branch == "" {
			inv.setErr("failed", "review_no_branch", "issue 里没有分支字段（不能 merge）")
			fmt.Fprintf(stderr, "%s: ❌ issue has no branch field (cannot merge) — check whether the task is in dev mode\n", progName)
			return exitFail
		}
		if err := reviewMergeBranch(cfg.repoDir, branch, cfg.issueID, stderr); err != nil {
			inv.setErr("failed", "review_merge_failed", "git merge 失败")
			fmt.Fprintf(stderr, "%s: ❌ merge failed: %v (check whether branch %s exists)\n", progName, err, branch)
			return exitFail
		}
		newContent, err := agent.TransitionIssue(text, agent.StatusWaitReview, agent.StatusDone)
		if err != nil {
			inv.setErr("failed", "review_transition_failed", "状态流转 wait_review→done 失败")
			fmt.Fprintf(stderr, "%s: ❌ failed to mark done: %v\n", progName, err)
			return exitFail
		}
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			inv.setErr("failed", "review_issue_unwritable", "issue 单写不回")
			fmt.Fprintf(stderr, "%s: ❌ failed to write issue (merge 已做，issue 仍是 waiting_review): %v\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(stdout, "✅ merge confirmed: %s → main (issue done)\n", cfg.issueID)
		return exitOK

	case "reject":
		if branch != "" {
			reviewDiscardBranch(cfg.repoDir, branch, stderr)
		}
		newContent, err := agent.TransitionIssue(text, agent.StatusWaitReview, agent.StatusRetry)
		if err != nil {
			inv.setErr("failed", "review_transition_failed", "状态流转 wait_review→retry 失败")
			fmt.Fprintf(stderr, "%s: ❌ failed to mark retry: %v\n", progName, err)
			return exitFail
		}
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			inv.setErr("failed", "review_issue_unwritable", "issue 单写不回")
			fmt.Fprintf(stderr, "%s: ❌ failed to write issue (状态仍是 waiting_review): %v\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(stdout, "❌ rejected: %s (branch discarded — issue retry)\n", cfg.issueID)
		return exitOK
	}
	// `reviewParse` 已收口动作闭集 ⇒ 到不了这里；留着只为「新增动作漏改解析面」时 fail-closed。
	fmt.Fprintf(stderr, "%s: unknown action (approve/reject)\n", progName)
	return exitUsage
}

// reviewListWaiting —— 源件 `listWaiting` 的逐字搬（表头 / 行格式 / 空档判词全同）。
//
// 唯一差别：`docs/issues/` **不存在** ⇒ 照源件出「no tasks awaiting approval」并退 0
// （没有任务目录 = 没有在等的单，这是真话）；其余读错（权限等）⇒ 判词 + 退 1（读不到不许当健康）。
func reviewListWaiting(inv *invocation, workDir string, stdout, stderr io.Writer) int {
	dir := filepath.Join(workDir, reviewIssuesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			inv.setErr("failed", "review_issues_dir_unreadable", "任务目录读不到")
			fmt.Fprintf(stderr, "%s: ❌ failed to read issues dir %s: %v\n", progName, dir, err)
			return exitFail
		}
		entries = nil
	}
	fmt.Fprintln(stdout, "📋 awaiting brain approval (waiting_review):")
	found := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // 源件同档：单件读不到只跳过（整表照出）
		}
		if !strings.Contains(string(data), "- **状态**: waiting_review") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		task := reviewExtractField(string(data), "- **任务**:")
		fmt.Fprintf(stdout, "  %s: %s\n", id, task)
		found = true
	}
	if !found {
		fmt.Fprintln(stdout, "  (no tasks awaiting approval)")
	}
	return exitOK
}

// reviewExtractField —— 源件 `extractBranch` / `extractStatus` / `extractField` 的同一实现
// （源件把同一段循环写了三遍；三处的行为逐字相同 ⇒ 本件落一处，语义零差）。
func reviewExtractField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// reviewMergeBranch —— 源件 `mergeBranch` 的三步（同子命令、同顺序、同判词；差异 ③：不拼 shell）。
func reviewMergeBranch(repoDir, branch, issueID string, stderr io.Writer) error {
	fmt.Fprintf(stderr, "🔀 cd %s && git checkout main && git merge --no-ff %s -m 'merge: %s → main (PR %s)' && git branch -d %s\n",
		repoDir, branch, branch, issueID, branch)
	if out, err := reviewGit(repoDir, "checkout", "main"); err != nil {
		return fmt.Errorf("%v (output: %s)", err, out)
	}
	if out, err := reviewGit(repoDir, "merge", "--no-ff", branch,
		"-m", fmt.Sprintf("merge: %s → main (PR %s)", branch, issueID)); err != nil {
		return fmt.Errorf("%v (output: %s)", err, out)
	}
	if out, err := reviewGit(repoDir, "branch", "-d", branch); err != nil {
		return fmt.Errorf("%v (output: %s)", err, out)
	}
	return nil
}

// reviewDiscardBranch —— 源件 `discardBranch` 的两步（差异 ⑤：失败的那一行不再静默）。
func reviewDiscardBranch(repoDir, branch string, stderr io.Writer) {
	fmt.Fprintf(stderr, "🗑 cd %s && git checkout main && git branch -D %s\n", repoDir, branch)
	if out, err := reviewGit(repoDir, "checkout", "main"); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠️ git checkout main failed: %v (output: %s)\n", progName, err, out)
	}
	if out, err := reviewGit(repoDir, "branch", "-D", branch); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠️ git branch -D %s failed: %v (output: %s)\n", progName, branch, err, out)
	}
}

// reviewGit —— 在 `repoDir` 里跑一条 git 子命令（逐参传、无 shell；输出合流后回给调用方拼判词）。
func reviewGit(repoDir string, args ...string) (string, error) {
	full := append([]string{"-C", repoDir}, args...)
	c := exec.Command("git", full...)
	out, err := c.CombinedOutput()
	return string(out), err
}
