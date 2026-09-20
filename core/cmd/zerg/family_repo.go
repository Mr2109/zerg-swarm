// family_repo.go —— 仓面只读一条（§九 I3 · 缺口-命令面-20260921 §十一 P0-2）。
//
// 为什么它排 P0：「开工底 / 收工对拍」今天都是手敲 `git log + git status`（**11 行**），
// 而**一仓一写入者**这条纪律机器判不了。命令化之后：开工前基线可回读、多代并行时
// 「谁在写这个仓」有据、`dev receipt new --commit` 有真源、提交闸拿得到干净的输入。
//
// 口径（照 §十一 P0-2 的形态，不自造）：
//
//	形态 `zerg repo status [--json <字段>]`
//	输出 `HEAD/分支/脏件清单/未跟踪件/是否被别的会话占`
//	退码 `0` 干净 / `1` 有脏件（**不是错** —— 「脏」是状态不是故障）/ `2` 用法错 / `8` 读不到
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// repoStatusRow —— 一行（`--json` 的字段就是这些键；**head/branch 逐行重复**是刻意的：
// 包封六键不许加，而「哪一版 / 哪一支」必须能随任一行带出去）。
type repoStatusSummary struct {
	head      string
	branch    string
	dirty     int
	untracked int
	held      string // 「另一个 git 进程正在写这个仓」的真源：`.git/index.lock` 在不在
}

func cmdRepoStatus(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到仓的状态（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		inv.setErr("blocked", "not_a_git_repo", "仓根下没有 .git")
		fmt.Fprintf(stderr, "%s: %s 下没有 .git ⇒ 这不是一个 git 工作树（不给结论 · 退码 8）\n", progName, root)
		return exitBlocked
	}

	// 脏件清单：**真源就是 git 自己的 porcelain 面**（不自己比 mtime/sha —— 那会造出第二套口径）。
	porcelain, err := gitRun(root, "status", "--porcelain=v1", "--branch", "--untracked-files=normal")
	if err != nil {
		inv.setErr("blocked", "git_status_failed", err.Error())
		fmt.Fprintf(stderr, "%s: `git status` 跑不动 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	head, _ := gitRun(root, "rev-parse", "--short", "HEAD")
	head = strings.TrimSpace(head)

	sum := repoStatusSummary{head: head, held: "否"}
	if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); err == nil {
		sum.held = "是（.git/index.lock 在 —— 另一个 git 进程正在写这个仓）"
	}
	rows := []map[string]string{}
	for _, ln := range strings.Split(porcelain, "\n") {
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "## ") {
			sum.branch = parsePorcelainBranch(strings.TrimPrefix(ln, "## "))
			continue
		}
		if len(ln) < 4 {
			continue
		}
		code := strings.TrimSpace(ln[:2])
		path := strings.TrimSpace(ln[3:])
		untracked := "否"
		if strings.HasPrefix(code, "?") {
			untracked, sum.untracked = "是", sum.untracked+1
		} else {
			sum.dirty++
		}
		rows = append(rows, map[string]string{
			"head":      sum.head,
			"branch":    sum.branch,
			"path":      path,
			"status":    code,
			"untracked": untracked,
		})
	}
	if sum.branch == "" {
		sum.branch = "（detached / 读不到）"
	}
	fmt.Fprintf(stderr, "%s: 仓 %s\n", progName, root)
	fmt.Fprintf(stderr, "%s: HEAD %s · 分支 %s · 脏件 %d · 未跟踪 %d · 被占 %s\n",
		progName, sum.head, sum.branch, sum.dirty, sum.untracked, sum.held)
	if len(rows) == 0 {
		// 干净仓：**仍然出一行**（否则 `--json` 里 item 为空 ⇒ head/branch 就带不出去了）。
		rows = append(rows, map[string]string{
			"head": sum.head, "branch": sum.branch, "path": "（无脏件）",
			"status": "clean", "untracked": "否",
		})
		fmt.Fprintf(stderr, "%s: 干净（退码 0）—— 逐字对拍请用 `zerg repo status --json` 两跑 diff\n", progName)
		if rc := listCmd(inv, stdout, stderr, []string{"head", "branch", "path", "status"}, rows); rc != exitOK {
			return rc
		}
		return exitOK
	}
	// 有脏件：退 1（**不是错** —— 「没提交」是状态；要看它是不是错由调用方判）。
	fmt.Fprintf(stderr, "%s: 有 %d 处未提交（退码 1 · 不是错，是状态）\n", progName, len(rows))
	if rc := listCmd(inv, stdout, stderr, []string{"head", "branch", "path", "status", "untracked"}, rows); rc != exitOK {
		return rc
	}
	return exitFail
}

// parsePorcelainBranch —— 解析 `--branch` 的第一行：`main...origin/main [ahead 1]` ⇒ `main`。
func parsePorcelainBranch(s string) string {
	if i := strings.Index(s, "..."); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, " "); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// gitRun 在仓根跑一条只读 git 命令（**只读**：本命令不许写仓 —— 连 `git status` 的索引刷新
// 都由 git 自己决定，命令面不加任何旗标去改这一行为）。
func gitRun(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s：%v（%s）", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}
