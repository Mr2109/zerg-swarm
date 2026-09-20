// family_repo_commit.go —— 提交面（`zerg repo commit` · D3b 第三步 · 缺口-命令面 §九 I4）。
//
// 病征（D3b 逐字）：判据与取证已全在命令面，而**「提交」还走不了命令面** ⇒ 今天的提交靠人/父代理
// 手工敲 `git`，两条纪律（**按文件名暂存** 禁 `git add -A` · **一仓一写入者**）机器判不了。
//
// 本件的口径（照 §九 I4 的形态，不自造）：
//
//	① **按文件名暂存**：逐个 `--file <件>` ⇒ `git add -- <件>…`；出现 `-A` / `--all` / `-a` / `.` / 通配
//	   ⇒ **拒执**（退码 2）。落暂存后**复核暂存清单**：`git diff --cached --name-only` 必须与给的清单
//	   **逐件相同**（多一件/少一件都拒 —— 「顺手带进去」正是这条纪律要防的）。
//	② **过快速档才放行**：真跑先跑 `scripts/gates/precommit-gates.sh --fast`，退码**直通不翻译**
//	   （0 才继续；1 有失败项；2 不给结论）。门禁在**暂存之前**跑 ⇒ 拦下来时索引面**逐字未动**。
//	③ **禁 `--no-verify`**：命令面**一律不接受**（§17.3 铁律④③「AI 不许 --no-verify」）——
//	   在 dispatch 里对**所有**命令拒（不给任何命令开这个口子）。
//	④ **提交信息模板**：题 + 提案/提出者/trace/判据/逐件清单（机器可回读的固定形态，不写自由散文）。
//	⑤ **撞车**：`.git/index.lock` 在 ⇒ 退 `14`（冲突/被占 · §十五.7 ① 与资源不足分开）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// repoCommitFields —— `--json` 面的全部字段（K1：机器面先定）。
var repoCommitFields = []string{"head", "files", "staged", "gate_rc", "commit", "message_path", "log_dir"}

// gitForbiddenStaging —— 暂存面的禁词（**一个都不许出现在 `--file` 里**）：
// 它们都是「一次把**不止一件**东西带进去」的写法，而本命令的纪律就是**逐件点名**。
var gitForbiddenStaging = []string{"-A", "--all", "-a", ".", "./", ":/", "*"}

// cmdRepoCommit —— `zerg repo commit`：按文件名暂存 + 过快速档 + 模板化提交信息。
func cmdRepoCommit(inv *invocation, stdout, stderr io.Writer) int {
	files := trimAll(inv.flagVals("--file"))
	message := strings.TrimSpace(inv.flagVal("--message"))

	// ① 用法面（在任何盘面动作之前）
	if message == "" {
		inv.setErr("usage", "missing_message", "缺提交信息")
		fmt.Fprintf(stderr, "%s: `repo commit` 要一枚 `--message <题>`（提交信息模板的第一行 = 题）\n", progName)
		return exitUsage
	}
	if len(files) == 0 {
		inv.setErr("usage", "missing_file", "缺件名清单")
		fmt.Fprintf(stderr, "%s: `repo commit` 要逐个 `--file <件>` —— **禁 `git add -A`**（§九 M3 纪律：按文件名暂存）\n", progName)
		fmt.Fprintf(stderr, "先看脏件：%s repo status\n", progName)
		return exitUsage
	}
	for _, f := range files {
		for _, bad := range gitForbiddenStaging {
			if f == bad {
				inv.setErr("usage", "bulk_staging_forbidden", "禁批量暂存")
				fmt.Fprintf(stderr, "%s: `--file %s` 是**批量暂存**写法 ⇒ 拒执（退码 2）—— 只有**按文件名逐件暂存**才留得下可复核的清单\n", progName, f)
				return exitUsage
			}
		}
		if strings.ContainsAny(f, "*?[") || strings.HasPrefix(f, ":") {
			inv.setErr("usage", "glob_staging_forbidden", "禁通配暂存")
			fmt.Fprintf(stderr, "%s: `--file %s` 含通配/路径规格 ⇒ 拒执（退码 2 · 一次只点名一件件）\n", progName, f)
			return exitUsage
		}
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		inv.setErr("blocked", "not_a_git_repo", "仓根下没有 .git")
		fmt.Fprintf(stderr, "%s: %s 下没有 .git ⇒ 不是 git 工作树（退码 8）\n", progName, root)
		return exitBlocked
	}
	// ⑤ 撞车：别人正在写这个仓
	if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); err == nil {
		inv.setErr("conflict", "repo_busy", "另一个 git 进程正在写这个仓")
		fmt.Fprintf(stderr, "%s: `.git/index.lock` 在 ⇒ **被占**（另有 git 进程在写这个仓）—— 退码 14（可等可重来）\n", progName)
		return exitConflict
	}
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	by := strings.TrimSpace(inv.flagVal("--by"))
	trace := strings.TrimSpace(inv.flagVal("--trace"))
	criterion := strings.TrimSpace(inv.flagVal("--criterion"))
	msg := commitMessageTemplate(message, proposalID, by, trace, criterion, files)
	// 计划件里的日志落点写**模式**、不写实值：实值带时间戳 ⇒ 同一判据跑两遍不会逐字一致
	// （§九 M11「同一判据跑两遍必须一致」）。真跑那一次才用实值（它落在 `--json` 的结果面里）。
	logDirPattern := filepath.Join(os.TempDir(), "zerg-repo-commit-<YYYYMMDD-HHMMSS>")
	logDir := filepath.Join(os.TempDir(), "zerg-repo-commit-"+time.Now().Format("20060102-150405"))

	// ② 干跑（默认按三态；`--dry-run` 或确认档不齐都只出计划件）
	if inv.dryRun || !inv.yes {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未暂存、未提交、未跑门禁）")
		fmt.Fprintf(stdout, "  动作     : %s repo commit（按文件名暂存 + **过快速档才放行**）\n", progName)
		fmt.Fprintf(stdout, "  仓       : %s\n", root)
		fmt.Fprintf(stdout, "  件清单   : %s\n", strings.Join(files, " · "))
		fmt.Fprintf(stdout, "  暂存方式 : git add -- <逐件>（**禁 `git add -A`**）· 暂存后复核 `git diff --cached --name-only` 与清单逐件相同\n")
		fmt.Fprintf(stdout, "  门禁     : bash scripts/gates/precommit-gates.sh --fast --outdir %s（**退码直通**：0 才继续）\n", logDirPattern)
		fmt.Fprintf(stdout, "  提交信息 :\n")
		for _, ln := range strings.Split(strings.TrimRight(msg, "\n"), "\n") {
			fmt.Fprintf(stdout, "    | %s\n", ln)
		}
		fmt.Fprintf(stdout, "  出口     : git commit -F <临时件>（**不加 `--no-verify`** —— 命令面一律不接受它）\n")
		if !inv.yes {
			fmt.Fprintf(stdout, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		}
		if inv.jsonGiven {
			if !requireFields(inv, stderr) {
				return exitFail
			}
			_ = selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, map[string]string{
				"head": "", "files": strings.Join(files, ","), "staged": "", "gate_rc": "",
				"commit": "", "message_path": "", "log_dir": logDirPattern,
			})
		}
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未跑门禁、未暂存、未提交）")
		// 三态：`--dry-run` 是**唯一会返回 0** 的那一态（§九 M3 C4）；没给 `--dry-run` 而只缺 `--yes`
		// ⇒ 计划件 + 退码 2（fail-closed：从不提问，也不偷偷提交）。
		if !inv.yes && !inv.dryRun {
			fmt.Fprintf(stderr, "%s: `repo commit` 要 `--yes`（D2 档：提交可逆 —— `git reset --soft HEAD~1`）⇒ 缺它不执行\n", progName)
			return exitUsage
		}
		return exitOK
	}

	// ③ 暂存前：索引面必须是空的（不许把「别人已暂存的」顺手提交）
	pre, err := gitRun(root, "diff", "--cached", "--name-only")
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	if strings.TrimSpace(pre) != "" {
		inv.setErr("usage", "index_not_empty", "暂存面不是空的")
		fmt.Fprintf(stderr, "%s: 暂存面**已经不是空的**（已暂存：%s）⇒ 拒执（退码 2）：\n",
			progName, strings.ReplaceAll(strings.TrimSpace(pre), "\n", " · "))
		fmt.Fprintf(stderr, "本命令只认「按文件名逐件暂存」这一条路 —— 先把别人的暂存清掉（`git reset`），别让它们混进这次提交\n")
		return exitUsage
	}

	// ④ **过快速档才放行**（在暂存之前跑 ⇒ 被拦下时索引面逐字未动）
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		inv.setErr("failed", "logdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了门禁日志目录 %s：%v\n", progName, logDir, err)
		return exitFail
	}
	gtRC, err := runFastGate(root, logDir, stderr)
	if err != nil {
		inv.setErr("failed", "gate_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	if gtRC != exitOK {
		// **不给结论 ≠ 绿**：门禁 1（有失败项）/ 2（不给结论）都拦 ⇒ 退码直通。
		if gtRC == exitFail {
			inv.setErr("failed", "gate_failed", "快速档有失败项")
		} else {
			inv.setErr("usage", "gate_blocked", "快速档不给结论")
		}
		fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d）⇒ 不提交**（索引面与工作树逐字未动 · 日志 %s）\n", progName, gtRC, logDir)
		fmt.Fprintf(stderr, "复核：%s gate run --fast --outdir %s\n", progName, logDir)
		return gtRC
	}
	fmt.Fprintf(stderr, "%s: 快速档 rc=0 ✓（日志 %s）⇒ 放行本次提交\n", progName, logDir)

	// ⑤ 逐件暂存 + **复核暂存清单**（多一件 / 少一件都拒）
	addArgs := append([]string{"add", "--"}, files...)
	if _, err := gitRun(root, addArgs...); err != nil {
		inv.setErr("failed", "git_add_failed", err.Error())
		fmt.Fprintf(stderr, "%s: `git add` 失败（逐件暂存）：%v\n", progName, err)
		return exitFail
	}
	post, err := gitRun(root, "diff", "--cached", "--name-only")
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	staged := splitLines(post)
	if same, extra, missing := compareFileSets(files, staged); !same {
		inv.setErr("usage", "staged_mismatch", "暂存清单与给的清单不一致")
		fmt.Fprintf(stderr, "%s: **暂存清单与给的清单不一致 ⇒ 拒执**（退码 2）\n", progName)
		if len(extra) > 0 {
			fmt.Fprintf(stderr, "  多出来的：%s\n", strings.Join(extra, " · "))
		}
		if len(missing) > 0 {
			fmt.Fprintf(stderr, "  少了的  ：%s\n", strings.Join(missing, " · "))
		}
		if _, err := gitRun(root, append([]string{"reset", "-q", "--"}, files...)...); err != nil {
			fmt.Fprintf(stderr, "  ⚠ 撤销暂存失败（索引面留在暂存态）：%v\n", err)
		}
		return exitUsage
	}

	// ⑥ 提交（信息走临时件 ⇒ 不经过 shell 转义；**不加 `--no-verify`**）
	msgPath := filepath.Join(logDir, "commit-message.txt")
	if err := os.WriteFile(msgPath, []byte(msg), 0o600); err != nil {
		inv.setErr("failed", "message_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 提交信息临时件写不了：%v\n", progName, err)
		return exitFail
	}
	rc := runGitCommit(root, msgPath, stdout, stderr)
	if rc != exitOK {
		inv.setErr("failed", "commit_rejected", "提交被拦")
		fmt.Fprintf(stderr, "%s: 提交未落地（rc=%d）—— 索引面留在暂存态，逐件已暂存：%s\n",
			progName, rc, strings.Join(staged, " · "))
		return rc
	}
	head, _ := gitRun(root, "rev-parse", "--short", "HEAD")
	head = strings.TrimSpace(head)
	row := map[string]string{
		"head": head, "files": strings.Join(files, ","), "staged": strings.Join(staged, ","),
		"gate_rc": "0", "commit": head, "message_path": msgPath, "log_dir": logDir,
	}
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		return selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, row)
	}
	for _, f := range staged {
		fmt.Fprintln(stdout, f)
	}
	fmt.Fprintf(stderr, "%s: 已提交 %s（%d 件 · 快速档 rc=0 · 提交信息 %s）\n", progName, head, len(staged), msgPath)
	return exitOK
}

// commitMessageTemplate —— 提交信息的**模板**（机器可回读的固定形态；自由散文不进这个面）。
func commitMessageTemplate(title, proposal, by, trace, criterion string, files []string) string {
	var b strings.Builder
	b.WriteString(title + "\n\n")
	if proposal != "" {
		fmt.Fprintf(&b, "提案: %s\n", proposal)
	}
	if by != "" {
		fmt.Fprintf(&b, "由: %s\n", by)
	}
	if trace != "" {
		fmt.Fprintf(&b, "trace: %s\n", trace)
	}
	if criterion != "" {
		fmt.Fprintf(&b, "判据: %s\n", criterion)
	}
	fmt.Fprintf(&b, "件: %s\n", strings.Join(files, " · "))
	fmt.Fprintf(&b, "门禁: 快速档 rc=0（scripts/gates/precommit-gates.sh --fast）\n")
	return b.String()
}

// runFastGate —— 跑快速档（**退码直通**：不翻译、不吞输出）。
func runFastGate(root, logDir string, stderr io.Writer) (int, error) {
	script := filepath.Join(root, "scripts/gates/precommit-gates.sh")
	if _, err := os.Stat(script); err != nil {
		return exitBlocked, fmt.Errorf("门禁最小入口不在（%s）", script)
	}
	cmd := exec.Command("bash", script, "--fast", "--outdir", logDir)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = stderr, stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return exitFail, err
	}
	return exitOK, nil
}

// runGitCommit —— `git commit -F <件>`（**不加 `--no-verify`**）：输出原样打印、退码原样返回。
func runGitCommit(root, msgPath string, stdout, stderr io.Writer) int {
	cmd := exec.Command("git", "-C", root, "commit", "-F", msgPath)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "%s: `git commit` 起不来：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// compareFileSets —— 给的清单与暂存清单**逐件相同**才放行（多 / 少都点名）。
func compareFileSets(want, got []string) (bool, []string, []string) {
	w, g := map[string]bool{}, map[string]bool{}
	for _, f := range want {
		w[strings.TrimSpace(f)] = true
	}
	for _, f := range got {
		g[strings.TrimSpace(f)] = true
	}
	extra, missing := []string{}, []string{}
	for k := range g {
		if !w[k] {
			extra = append(extra, k)
		}
	}
	for k := range w {
		if !g[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	return len(extra) == 0 && len(missing) == 0, extra, missing
}

// splitLines —— 逐行（丢空行）。
func splitLines(s string) []string {
	out := []string{}
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}
