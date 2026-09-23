// family_repo.go —— 仓面只读一条（§九 I3 · 缺口-命令面-20260921 §十一 P0-2）。
//
// 为什么它排 P0：「开工底 / 收工对拍」今天都是手敲 `git log + git status`（**11 行**），
// 而**一仓一写入者**这条纪律机器判不了。命令化之后：开工前基线可回读、多代并行时
// 「谁在写这个仓」有据、`dev receipt new --commit` 有真源、提交闸拿得到干净的输入。
//
// 口径（照 §十一 P0-2 的形态，不自造）：
//
//	形态 `zerg repo status [--root <仓根>]… [--json <字段>]`
//	输出 `HEAD/分支/脏件清单/未跟踪件/是否被别的会话占`
//	退码 `0` 干净 / `1` 有脏件（**不是错** —— 「脏」是状态不是故障）/ `2` 用法错（含**写旗标一律拒**）/ `8` 读不到
//
// `W-28` 加的两半（2026-09-24 · 波12 序111 · 组4 §二.2 · `Q-028`/`A7`/`R-10`）：
//
//	① **多仓汇总**：`--root` 给 **≥2 次** ⇒ 一条**只读**命令出两仓（或多仓）`HEAD + 脏件数`
//	   （组3 §一 序28 逐字：「跨两仓没有一处给两仓 `git status`」）。口径**与单仓同源** ——
//	   每仓仍走同一条 `git status --porcelain=v1`，**不另立第二份计数口径**。
//	② **写旗标一律拒**（`rc=2`）：见 `repoWriteWords`（形态无关 · 不看位置）——
//	   因为这条命令跑在**仓外仓**上（`Zerg-内部文档/` 是并排仓），且写词里有些是**真旗标名**
//	   （`--commit`），解析器会静默收下 ⇒ 必须自己判一道。
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

// repoWriteWords —— `W-28` 判据②的唯一真源：**写旗标闭集**（形态无关 · 不看位置）。
//
// 为什么这条命令要**自己**再判一道（`--nosuchflag-zz` 已经能退 2）：因为**写词里有些是真旗标名**
// （`--commit` 就是 `repo commit` 的旗标）—— 解析器认得它、于是**静默收下不报错**，
// 而这条命令跑在**仓外仓**上（`Zerg-内部文档/` 是并排仓、不在主仓仓根下）⇒ 误写的代价与 `G-06` 同族。
// ⇒ 一律先扫一道：命中任一写词 ⇒ **拒（`rc=2`）**，且**绝不代跑任何 git 写子命令**。
var repoWriteWords = []string{
	"add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
	"merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
	"mv", "apply", "stash", "update-index", "write-tree", "hash-object", "gc",
	"prune", "repack", "unlock", "commit-tree", "symbolic-ref-set", "config-set",
}

// repoWriteHits 扫原样 argv（`inv.orig`）⇒ 命中的写词（原样）。
func repoWriteHits(orig []string) []string {
	var hits []string
	for _, tok := range orig {
		t := strings.TrimSpace(tok)
		if t == "" || t == "--" {
			continue
		}
		name := strings.ToLower(strings.Split(strings.TrimLeft(t, "-"), "=")[0])
		if name == "" {
			continue
		}
		// 形态无关：带旗标（`--push` / `--push=origin`）与**纯词**（`push`）都认；
		// 值位（路径 / 字段表）里含 `/` `.` `,` 的一律不当写词（避免拿路径名误判）。
		if !strings.HasPrefix(t, "-") && strings.ContainsAny(t, "/.,") {
			continue
		}
		for _, w := range repoWriteWords {
			if name == w {
				hits = append(hits, t)
				break
			}
		}
	}
	return hits
}

func cmdRepoStatus(inv *invocation, stdout, stderr io.Writer) int {
	// `W-28`（`Q-028`）第一道牙：写旗标一律拒（见 `repoWriteWords` 上方的口径）。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读汇总 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `repo status` **只读**（它长在仓外仓上 —— 主仓 + 并排的 Zerg-内部文档/）· "+
			"要写请用 `%s repo commit`（那一支是危险档、有自己的确认档）\n", progName, progName)
		return exitUsage
	}
	// `W-28`（`Q-028`）：`--root` **给 0/1 次** ⇒ 单仓面（**逐字不变**）；
	// `--root` **给 ≥2 次** ⇒ 多仓汇总面（一条只读命令出两仓 HEAD + 脏件数）。
	roots := []string{}
	for _, r := range inv.flagVals("--root") {
		if s := strings.TrimSpace(r); s != "" {
			roots = append(roots, s)
		}
	}
	if len(roots) >= 2 {
		return repoStatusMulti(roots, inv, stdout, stderr)
	}
	root := ""
	if len(roots) == 1 {
		root = roots[0]
	}
	return repoStatusOne(root, inv, stdout, stderr)
}

// repoStatusMulti —— `--root` 给了 **≥2** 次：**一条只读命令出两仓（或多仓）`HEAD + 脏件数`**
// （`W-28` · `Q-028` · 组3 §一 序28「跨两仓没有一处给两仓 `git status`」）。
//
// 口径（**与单仓同源 · 不另立第二份计数口径**）：每仓仍走
// `git status --porcelain=v1 --branch --untracked-files=normal` + `rev-parse --short HEAD`；
// 脏件数 = 提交面里非 `??` 的行数 · 未跟踪数 = `??` 的行数（与单仓面**同一个式子**）。
//
// 退码：全干净 `0` / 任一仓有未提交 `1`（**「脏」是状态不是错** · 照单仓口径）/
//
//	任一 `--root` 不是 git 工作树 ⇒ `8`（**读不到不许当绿** · 也不拿它当 0）。
//
// ★ 只读：不写仓、不跑任何 git 写子命令；`--json` 面仍是同一套六键包封 + 同一张字段面
//
//	（一行一仓：`path` = 仓根 · `status` = `clean` / `dirty <N>` · `untracked` = 是/否）。
func repoStatusMulti(roots []string, inv *invocation, stdout, stderr io.Writer) int {
	type repoRow struct {
		label, root, head, branch, held string
		dirty, untracked                int
	}
	rows := []repoRow{}
	for _, r := range roots {
		if _, err := os.Stat(filepath.Join(r, ".git")); err != nil {
			inv.setErr("blocked", "not_a_git_repo", "仓根下没有 .git")
			fmt.Fprintf(stderr, "%s: %s 下没有 .git ⇒ 这不是一个 git 工作树（不给结论 · 退码 8）\n", progName, r)
			return exitBlocked
		}
		porcelain, err := gitRun(r, "status", "--porcelain=v1", "--branch", "--untracked-files=normal")
		if err != nil {
			inv.setErr("blocked", "git_status_failed", err.Error())
			fmt.Fprintf(stderr, "%s: `git status` 在 %s 跑不动 ⇒ 不给结论（退码 8）：%v\n", progName, r, err)
			return exitBlocked
		}
		head, _ := gitRun(r, "rev-parse", "--short", "HEAD")
		it := repoRow{label: filepath.Base(strings.TrimRight(r, string(os.PathSeparator))),
			root: r, head: strings.TrimSpace(head), held: "否"}
		if _, err := os.Stat(filepath.Join(r, ".git", "index.lock")); err == nil {
			it.held = "是"
		}
		for _, ln := range strings.Split(porcelain, "\n") {
			if ln == "" {
				continue
			}
			if strings.HasPrefix(ln, "## ") {
				it.branch = parsePorcelainBranch(strings.TrimPrefix(ln, "## "))
				continue
			}
			if len(ln) < 4 {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(ln[:2]), "?") {
				it.untracked++
			} else {
				it.dirty++
			}
		}
		if it.branch == "" {
			it.branch = "（detached / 读不到）"
		}
		rows = append(rows, it)
	}

	totalDirty, totalUnt := 0, 0
	items := []map[string]string{}
	for _, it := range rows {
		totalDirty += it.dirty
		totalUnt += it.untracked
		st := "clean"
		if it.dirty+it.untracked > 0 {
			st = fmt.Sprintf("dirty %d", it.dirty+it.untracked)
		}
		un := "否"
		if it.untracked > 0 {
			un = "是"
		}
		items = append(items, map[string]string{
			"head": it.head, "branch": it.branch, "path": it.root, "status": st, "untracked": un,
		})
	}
	fmt.Fprintf(stderr, "%s: 多仓汇总（`--root` 给了 %d 次 · **只读** · 口径与单仓同源）\n", progName, len(rows))
	fmt.Fprintf(stderr, "%s: %-14s %-10s %-8s %6s %8s  %s\n", progName, "仓", "HEAD", "分支", "脏件", "未跟踪", "被占")
	for _, it := range rows {
		fmt.Fprintf(stderr, "%s: %-14s %-10s %-8s %6d %8d  %s\n",
			progName, it.label, it.head, it.branch, it.dirty, it.untracked, it.held)
	}
	fmt.Fprintf(stderr, "%s: 合计：未提交 %d 处（脏 %d · 未跟踪 %d）—— 「脏」是状态不是错（照单仓口径）\n",
		progName, totalDirty+totalUnt, totalDirty, totalUnt)
	if rc := listCmd(inv, stdout, stderr, []string{"head", "branch", "path", "status", "untracked"}, items); rc != exitOK {
		return rc
	}
	if totalDirty+totalUnt > 0 {
		return exitFail
	}
	return exitOK
}

func repoStatusOne(root string, inv *invocation, stdout, stderr io.Writer) int {
	// `--root <仓根>`（D3 缺口③）：指根 —— 不指就按 `ZERG_REPO` / 可执行件上溯 / 当前目录上溯解析。
	if root == "" {
		root = repoRoot()
	}
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
