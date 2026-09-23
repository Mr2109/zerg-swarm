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
	"encoding/json"
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
//
// 两种模式（`Q-104` · 2026-09-23 已拍 = 甲：落 `repo commit --only <路径…>`，**不开顶层 `commit`**）：
//
//	默认模式（今天那一条，**语义一个字不改**）：逐个 `--file <件>` 暂存 ⇒ 索引面必须为空 ⇒ 过快速档 ⇒ 提交；
//	`--only` 模式（新增）：**点名单路径** ⇒ **不 add**（`git commit --only -F <件> -- <路径…>`）⇒
//	  索引面非空是**常态**（别人的暂存件**只许多不许少**）⇒ 被排除件**逐条打印** ⇒ 对拍 + 审计。
func cmdRepoCommit(inv *invocation, stdout, stderr io.Writer) int {
	files := trimAll(inv.flagVals("--file"))
	only := trimAll(inv.flagVals("--only"))
	message := strings.TrimSpace(inv.flagVal("--message"))

	// ① 用法面（在任何盘面动作之前）
	if message == "" {
		inv.setErr("usage", "missing_message", "缺提交信息")
		fmt.Fprintf(stderr, "%s: `repo commit` 要一枚 `--message <题>`（提交信息模板的第一行 = 题）\n", progName)
		return exitUsage
	}
	if len(only) > 0 && len(files) > 0 {
		inv.setErr("usage", "mode_conflict", "`--file` 与 `--only` 是两种模式，不许同时给")
		fmt.Fprintf(stderr, "%s: `--file`（逐件暂存，索引面必须为空）与 `--only`（点名单路径，索引面非空是常态）是**两种模式** ⇒ 同时给退 2\n", progName)
		fmt.Fprintf(stderr, "选一条：`--file <件>…`（默认模式）或 `--only <路径>…`（Q-104 点名单路径模式）\n")
		return exitUsage
	}
	// `--waive <步名>` —— **带审计的显式例外旗标**（`Q-104` 已拍：开）。缺 `--reason` ⇒ 拒（不许「无痕放行」）。
	waive := trimAll(inv.flagVals("--waive"))
	reason := strings.TrimSpace(inv.flagVal("--reason"))
	if len(waive) > 0 && reason == "" {
		inv.setErr("usage", "waive_requires_reason", "`--waive` 必须同时给 `--reason <理由>`")
		fmt.Fprintf(stderr, "%s: `--waive %s` 没带 `--reason <理由>` ⇒ **拒执**（退码 2）\n", progName, strings.Join(waive, " · "))
		fmt.Fprintf(stderr, "口径（`Q-104` 已拍）：例外旗标**带审计**才有意义 —— 不写理由的豁免就是洗白（`T5` 那句的同一口径）\n")
		fmt.Fprintf(stderr, "形态：--waive <步名> --reason <为什么这次可以带账放行>\n")
		return exitUsage
	}
	if len(only) > 0 {
		// `--only` 模式：**不新增对外面**（命令树 +0）—— 就长在 `repo commit` 这一条上（`Q-104` 甲案）。
		return repoCommitOnly(inv, stdout, stderr, only, message, waive, reason)
	}
	if len(files) == 0 {
		inv.setErr("usage", "missing_file", "缺件名清单")
		fmt.Fprintf(stderr, "%s: `repo commit` 要逐个 `--file <件>` —— **禁 `git add -A`**（§九 M3 纪律：按文件名暂存）\n", progName)
		fmt.Fprintf(stderr, "或走点名单路径模式：`--only <路径…>`（`Q-104`；索引面非空是常态）\n")
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
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
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
	// ★ `-c core.quotepath=false`：件名含非 ASCII（本项目大量中文件名）时，git 默认会把路径**转义成**
	// `\345\217\202…` 形态 ⇒ 与「人给的件名」逐字对不上，「复核暂存清单」这条判据会当场假红。
	// 这不是放宽判据，是把两处读的**同一个东西**（件名）读成同一种形态。
	pre, err := gitRun(root, "-c", "core.quotepath=false", "diff", "--cached", "--name-only")
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
	post, err := gitRun(root, "-c", "core.quotepath=false", "diff", "--cached", "--name-only")
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
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
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

// ══════════════════════════ `Q-104`：点名单路径模式（`repo commit --only`） ══════════════════════
//
// 病根（设计稿 §1.2 逐字）：默认模式的判据是 **`index_not_empty` ⇒ 拒执**，而多写者并发时索引面
// **天然非空** ⇒ 这条路根本走不到（判据与场景不相容）；它给的替代动作 `git reset` 是**危险建议**
// （会清掉别人的暂存）。于是人自然去找另一条路（手捣 `git commit -- <路径>`，并因门禁红而顺手加
// `--no-verify`）。本模式把那条对的路（git 原生 pathspec）做成命令面的一等形态。
//
// 四件必须做出来的机制（设计稿 §四）：
//
//	① pathspec **走 argv、不经 shell**（空格/中文/`$`/引号天然不被解释）；
//	② **条数对拍三格**：点名的件真进了这次提交 · 别人的暂存件一枚都没少 · 两枚计数；
//	③ **被排除的他人件显式打印**（不是「我们跳过了」；`N=0` 也要写口径，不许沉默）；
//	④ 快速档红 ⇒ 打印门禁原样输出 + 日志目录 + 复核命令（**退码直通不翻译**）——
//	   本命令**不提供 `--no-verify`**；确需人签例外走已拍的 `--waive <步名> --reason`（**进审计**）。

// commitAuditLine —— `commit_audit.jsonl` 的一行（一行一事件 · 追加只写 · **失败即拒**；形状照 `editAuditLine`）。
type commitAuditLine struct {
	At        string   `json:"at"`
	Event     string   `json:"event"`
	By        string   `json:"by"`
	Mode      string   `json:"mode"`
	Only      []string `json:"only"`
	Excluded  []string `json:"excluded"`
	GateRC    int      `json:"gate_rc"`
	Head      string   `json:"head,omitempty"`
	Waive     []string `json:"waive,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Commit    string   `json:"commit,omitempty"`
	AuditPath string   `json:"audit_path"`
	Note      string   `json:"note,omitempty"`
}

// commitAuditPath —— 审计落点（与 `edit_audit.jsonl` 同目录：`<状态目录>/commit_audit.jsonl`）。
func commitAuditPath() string { return filepath.Join(stateDirOf(), "commit_audit.jsonl") }

// appendCommitAudit 追加一行（O_APPEND · 一行一事件）。**失败即拒**（调用方据此不改仓）。
func appendCommitAudit(path string, line commitAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// onlyMatchPaths 点名件逐条判「在不在脏面/未跟踪面里」（`git status --porcelain` 是唯一真源）。
// ⇒ (匹配上的, 一件都没匹配上的)。
func onlyMatchPaths(root string, paths []string) (matched, unmatched []string) {
	for _, p := range paths {
		out, err := gitRun(root, "-c", "core.quotepath=false", "status", "--porcelain=v1",
			"--untracked-files=all", "--", p)
		if err != nil || strings.TrimSpace(out) == "" {
			unmatched = append(unmatched, p)
			continue
		}
		matched = append(matched, p)
	}
	return matched, unmatched
}

// setMinus a−b（保留 a 的次序与重复项 ⇒ 计数是**条数**不是集合势）。
func setMinus(a, b []string) []string {
	bset := map[string]bool{}
	for _, x := range b {
		bset[strings.TrimSpace(x)] = true
	}
	out := []string{}
	for _, x := range a {
		if !bset[strings.TrimSpace(x)] {
			out = append(out, x)
		}
	}
	return out
}

// containsLine 逐字找一行（对拍用；不做子串匹配 —— 件名要逐字比）。
func containsLine(lines []string, want string) bool {
	for _, ln := range lines {
		if ln == want {
			return true
		}
	}
	return false
}

// repoCommitOnly —— `repo commit --only <路径…> --message <题> [--waive <步名> --reason <…>] [--dry-run] [--yes]`。
func repoCommitOnly(inv *invocation, stdout, stderr io.Writer, paths []string, message string, waive []string, reason string) int {
	// ⓪ 多余位置参数 ⇒ 拒执（**fail-closed**）。现读缺口：`--only a.go b.go c.go` 里 `--only` 只吃**一枚**值
	//    （`main.go` 的值旗标一次消费一个 argv），`b.go`/`c.go` 会掉进位置参数被**静默丢掉** ——
	//    那正是「只提交我点名的」这一格最危险的形态（点三件、交一件、还挺绿）。⇒ 多件必须逐件 `--only`。
	if len(inv.args) > 0 {
		inv.setErr("usage", "only_extra_positional", "`--only` 一次只吃一枚值：多件要逐件 `--only`")
		fmt.Fprintf(stderr, "%s: `--only` **一次只吃一枚值** —— 多件要逐件写（`--only <件1> --only <件2> …`）\n", progName)
		fmt.Fprintf(stderr, "多余的 %d 个位置参数：%s\n", len(inv.args), strings.Join(inv.args, " · "))
		fmt.Fprintf(stderr, "（不用「多余位置参数当路径用」那种宽容解析：点三件交一件还挺绿 = 静默丢件 ⇒ 一律退 2）\n")
		return exitUsage
	}
	// ① 禁词 / 通配：**照默认模式那张表逐字** —— `--only` **不是**批量暂存禁令的后门（设计稿 §3.3 写死那一句）。
	for _, f := range paths {
		for _, bad := range gitForbiddenStaging {
			if f == bad {
				inv.setErr("usage", "bulk_staging_forbidden", "禁批量暂存")
				fmt.Fprintf(stderr, "%s: `--only %s` 是**批量暂存**写法 ⇒ 拒执（退码 2）—— `--only` 后面仍只许**逐件点名**\n", progName, f)
				return exitUsage
			}
		}
		if strings.ContainsAny(f, "*?[") || strings.HasPrefix(f, ":") {
			inv.setErr("usage", "glob_staging_forbidden", "禁通配")
			fmt.Fprintf(stderr, "%s: `--only %s` 含通配/路径规格 ⇒ 拒执（退码 2 · 一次只点名一件件）\n", progName, f)
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
	if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); err == nil {
		inv.setErr("conflict", "repo_busy", "另一个 git 进程正在写这个仓")
		fmt.Fprintf(stderr, "%s: `.git/index.lock` 在 ⇒ **被占**（另有 git 进程在写这个仓）—— 退码 14（可等可重来）\n", progName)
		return exitConflict
	}

	// ② 提交前读数：别人的暂存面（**允许非空** —— 那是本模式存在的理由）+ 点名件匹配面
	preRaw, err := gitRun(root, "-c", "core.quotepath=false", "diff", "--cached", "--name-only")
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	preStaged := splitLines(preRaw)
	matched, unmatched := onlyMatchPaths(root, paths)
	if len(matched) == 0 {
		inv.setErr("usage", "only_not_matched", "点名的路径一件都没匹配上")
		fmt.Fprintf(stderr, "%s: 点名的 %d 件**一件都没匹配上**（工作树里没有这些改动）⇒ 拒执（退码 2）\n", progName, len(paths))
		for _, p := range unmatched {
			fmt.Fprintf(stderr, "  没匹配上：%s\n", p)
		}
		fmt.Fprintf(stderr, "口径（设计稿 §2.3「**不学** `jj` 的 warning」）：点名的件一件都没命中 ⇒ 那一定是我敲错了\n")
		return exitUsage
	}
	excluded := setMinus(preStaged, paths)
	// ②′ 新件（未跟踪）：`git commit --only -- <未跟踪件>` 会 `did not match any file(s) known to git`
	// ⇒ 先做**意图登记**（`git add -N`：只把路径记进索引、不写内容、不动别人的暂存 —— 这是 git 的硬要求，
	// 不是「暂存」）。现读实测：不带这一步，新件一提交就 `rc=1`（`pathspec did not match`）。
	untracked := []string{}
	uargs := append([]string{"ls-files", "--others", "--exclude-standard", "--"}, paths...)
	if out, uerr := gitRun(root, uargs...); uerr == nil {
		untracked = splitLines(out)
	}
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未声明）"
	}
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	trace := strings.TrimSpace(inv.flagVal("--trace"))
	criterion := strings.TrimSpace(inv.flagVal("--criterion"))
	msg := commitMessageTemplate(message, proposalID, by, trace, criterion, paths)
	logDirPattern := filepath.Join(os.TempDir(), "zerg-repo-commit-<YYYYMMDD-HHMMSS>")
	logDir := filepath.Join(os.TempDir(), "zerg-repo-commit-"+time.Now().Format("20060102-150405"))
	auditPath := commitAuditPath()
	onlyRow := map[string]string{
		"head": "", "files": strings.Join(paths, ","), "staged": strings.Join(preStaged, ","),
		"gate_rc": "", "commit": "", "message_path": "", "log_dir": logDirPattern,
	}

	// ③ 三态：`--dry-run` ⇒ 计划件 rc=0；缺 `--yes` ⇒ 计划件 + rc=2；确认档齐 ⇒ 真跑。
	if inv.dryRun || !inv.yes {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未暂存、未提交、未跑门禁）")
		fmt.Fprintf(stdout, "  动作     : %s repo commit **--only**（点名单路径模式；索引面**允许非空** —— 那是常态）\n", progName)
		fmt.Fprintf(stdout, "  仓       : %s\n", root)
		fmt.Fprintf(stdout, "  点名件   : %d 件 —— %s\n", len(paths), strings.Join(paths, " · "))
		fmt.Fprintf(stdout, "  匹配面   : 匹配上 %d 件 · 一件都没匹配上 %d 件（预算值；真跑时**一件都没匹配上就判红**）\n",
			len(matched), len(unmatched))
		if len(unmatched) > 0 {
			fmt.Fprintf(stdout, "  未匹配   : %s\n", strings.Join(unmatched, " · "))
		}
		fmt.Fprintf(stdout, "  进提交的路: git commit --only -F <临时件> -- <点名件…>（**不 add**；pathspec 走 argv、不经 shell）\n")
		if len(untracked) > 0 {
			fmt.Fprintf(stdout, "  新件意图登记: %d 件 —— %s（`git add -N`：只记路径、不写内容、不动别人暂存；git 硬要求）\n",
				len(untracked), strings.Join(untracked, " · "))
		}
		fmt.Fprintf(stdout, "  别人的暂存: %s\n", commitExcludedNote(excluded))
		fmt.Fprintf(stdout, "  门禁     : bash scripts/gates/precommit-gates.sh --fast --outdir %s（**退码直通**）\n", logDirPattern)
		if len(waive) > 0 {
			fmt.Fprintf(stdout, "  例外旗标 : --waive %s --reason %s（**进审计**；需要 --yes）\n", strings.Join(waive, " · "), reason)
		}
		fmt.Fprintf(stdout, "  提交信息 :\n")
		for _, ln := range strings.Split(strings.TrimRight(msg, "\n"), "\n") {
			fmt.Fprintf(stdout, "    | %s\n", ln)
		}
		fmt.Fprintf(stdout, "  出口     : git commit --only -F <临时件> -- <点名件…>（**不加 `--no-verify`** —— 命令面一律不接受它）\n")
		if !inv.yes {
			fmt.Fprintf(stdout, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		}
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			_ = selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, onlyRow)
		}
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未跑门禁、未暂存、未提交）")
		if !inv.yes && !inv.dryRun {
			fmt.Fprintf(stderr, "%s: `repo commit --only` 要 `--yes`（D2 档：提交可逆 —— `git reset --soft HEAD~1`）⇒ 缺它不执行\n", progName)
			return exitUsage
		}
		return exitOK
	}

	// ④ 快速档（**在提交之前跑** ⇒ 被拦下时索引面与工作树逐字未动）；红时给归因三格 + 例外旗标判定。
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		inv.setErr("failed", "logdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了门禁日志目录 %s：%v\n", progName, logDir, err)
		return exitFail
	}
	gtRC, gtOut, err := runFastGateCapture(root, logDir)
	if err != nil {
		inv.setErr("failed", "gate_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	fmt.Fprint(stderr, gtOut)
	if gtRC != exitOK {
		waivedOK := len(waive) > 0 && reason != ""
		missing := []string{}
		for _, w := range waive {
			if !strings.Contains(gtOut, w) {
				missing = append(missing, w)
			}
		}
		if len(missing) > 0 {
			waivedOK = false
		}
		if waivedOK {
			// **带审计的显式例外**（`Q-104` 已拍）：照实记账、照实打印，**不换出口**（仍然不是 `--no-verify`）。
			fmt.Fprintf(stderr, "%s: ⚠ **例外旗标生效**：`--waive %s`（理由：%s）—— 快速档 rc=%d 的这几步**带账放行**\n",
				progName, strings.Join(waive, " · "), reason, gtRC)
			fmt.Fprintf(stderr, "%s: 这次例外会落进 `%s`（谁 · 什么时候 · 哪一步 · 什么理由 —— 「红」从此可回读）\n", progName, auditPath)
			fmt.Fprintf(stderr, "%s: ★ 绕行仍然**只有**人能显式做（`git commit --no-verify`）；命令面**不提供** `--no-verify`，本旗标只是把它换成可查的那一条\n", progName)
		} else {
			// fail-closed：rc=1 有失败项、rc=2 不给结论都拦 ⇒ **退码直通不翻译**
			line := commitAuditLine{At: time.Now().Format(time.RFC3339), Event: "gate_blocked", By: by,
				Mode: "only", Only: paths, Excluded: excluded, GateRC: gtRC, AuditPath: auditPath}
			if len(waive) > 0 {
				line.Waive, line.Reason = waive, reason
				if len(missing) > 0 {
					line.Note = "点名要豁免的步名不在这一次的报面上：" + strings.Join(missing, " · ")
				}
			}
			if aerr := appendCommitAudit(auditPath, line); aerr != nil {
				fmt.Fprintf(stderr, "%s: ⚠ 撞红那一行审计落不下：%v\n", progName, aerr)
			}
			if gtRC == exitFail {
				inv.setErr("failed", "gate_failed", "快速档有失败项")
			} else {
				inv.setErr("usage", "gate_blocked", "快速档不给结论")
			}
			fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d）⇒ 不提交**（索引面与工作树逐字未动 · 日志 %s）\n", progName, gtRC, logDir)
			if len(missing) > 0 {
				fmt.Fprintf(stderr, "  `--waive` 点名的 %s **不在**这一次的报面上 ⇒ 例外旗标不生效（豁免不许挂在空气上）\n", strings.Join(missing, " · "))
			}
			fmt.Fprintf(stderr, "  归因：步名逐字见上面门禁原样输出 · 日志目录 %s · 复核命令 `bash scripts/gates/precommit-gates.sh --fast --outdir %s`\n", logDir, logDir)
			fmt.Fprintf(stderr, "  合法出口三个：① 等并行写者收工后重跑 ② 按日志逐条复核 ③ 确需人签例外 ⇒ `--waive <步名> --reason <…> --yes`（**进审计**）\n")
			return gtRC
		}
	} else {
		fmt.Fprintf(stderr, "%s: 快速档 rc=0 ✓（日志 %s）⇒ 放行本次提交\n", progName, logDir)
	}

	// ⑤ 提交：**不 add**（只对**未跟踪的新件**做 `git add -N` 意图登记 —— git 硬要求，见 ②′），
	// pathspec 走 argv（`exec.Command` 经 fork/exec、不经 `sh -c`）。
	if len(untracked) > 0 {
		if _, err := gitRunWrite(root, append([]string{"add", "--intent-to-add", "--"}, untracked...)...); err != nil {
			inv.setErr("failed", "intent_add_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 新件意图登记失败（`git add -N`）：%v ⇒ 不提交\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(stderr, "%s: 新件意图登记 %d 件（`git add -N`：只记路径、不写内容、不动别人暂存）：%s\n",
			progName, len(untracked), strings.Join(untracked, " · "))
	}
	msgPath := filepath.Join(logDir, "commit-message.txt")
	if err := os.WriteFile(msgPath, []byte(msg), 0o600); err != nil {
		inv.setErr("failed", "message_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 提交信息临时件写不了：%v\n", progName, err)
		return exitFail
	}
	preHead, _ := gitRun(root, "rev-parse", "--short", "HEAD")
	preHead = strings.TrimSpace(preHead)
	rc := runGitCommitOnly(root, msgPath, paths, stdout, stderr)
	head := preHead
	if rc == exitOK {
		if h, err := gitRun(root, "rev-parse", "--short", "HEAD"); err == nil {
			head = strings.TrimSpace(h)
		}
	}

	// ⑥ 三格对拍（缺一格不给结论）：① 点名的件真进了这次提交 ② 别人的暂存件一枚都没少 ③ 条数
	postRaw, perr := gitRun(root, "-c", "core.quotepath=false", "diff", "--cached", "--name-only")
	postStaged := []string{}
	if perr == nil {
		postStaged = splitLines(postRaw)
	}
	committed := []string{}
	if rc == exitOK {
		if s, err := gitRun(root, "-c", "core.quotepath=false", "show", "--name-only", "--pretty=format:", "HEAD"); err == nil {
			committed = splitLines(s)
		}
	}
	keptBack := setMinus(preStaged, paths)
	short := []string{}
	for _, k := range keptBack {
		if !containsLine(postStaged, k) {
			short = append(short, k)
		}
	}
	same, extra, missingF := compareFileSets(paths, committed)

	// ⑦ 审计（先落盘再报判决；落不下 ⇒ 报面照实点名，不改判决）
	line := commitAuditLine{At: time.Now().Format(time.RFC3339), Event: "commit", By: by, Mode: "only",
		Only: paths, Excluded: excluded, GateRC: gtRC, Head: head, Commit: head, AuditPath: auditPath}
	if len(waive) > 0 {
		line.Waive, line.Reason = waive, reason
	}
	if rc != exitOK {
		line.Event, line.Note = "commit_rejected", "提交被拦（rc="+fmt.Sprintf("%d", rc)+"）"
	}
	if aerr := appendCommitAudit(auditPath, line); aerr != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 审计落不下（%s）：%v —— 判决照旧（本行只记账）\n", progName, auditPath, aerr)
	}

	// ⑧ 被排除件那一片：**显式逐条**（不是「我们跳过了」；`N=0` 也写口径，不许沉默）
	fmt.Fprintf(stderr, "%s: %s\n", progName, commitExcludedNote(excluded))
	for _, e := range excluded {
		fmt.Fprintf(stderr, "  · %s\n", e)
	}
	if rc != exitOK {
		fmt.Fprintf(stderr, "%s: **提交未落地（rc=%d）** ⇒ 索引面与工作树均未按本命令改动（`--only` 模式不 `add` ⇒ 失败时索引面天然未动）\n", progName, rc)
		return rc
	}
	if !same {
		// 对拍不合 ⇒ 判红（退码 2）：这是「只提交我点名的」那一格崩了
		inv.setErr("usage", "committed_mismatch", "提交的件集与点名清单不一致")
		fmt.Fprintf(stderr, "%s: **提交的件集与点名清单不一致 ⇒ 判红**（退码 2）\n", progName)
		if len(extra) > 0 {
			fmt.Fprintf(stderr, "  多出来的：%s\n", strings.Join(extra, " · "))
		}
		if len(missingF) > 0 {
			fmt.Fprintf(stderr, "  少了的  ：%s\n", strings.Join(missingF, " · "))
		}
		fmt.Fprintf(stderr, "  回滚：`git reset --soft HEAD~1`（D2 可逆 · 提交内容面）\n")
		return exitUsage
	}
	if len(short) > 0 {
		inv.setErr("failed", "others_staged_lost", "别人的暂存件少了")
		fmt.Fprintf(stderr, "%s: **别人的暂存件少了一枚**（本该只许多不许少）⇒ 判红：%s\n", progName, strings.Join(short, " · "))
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 对拍 ✓ ① 提交的 %d 件与点名清单逐件相同 ② 别人的暂存 %d 件一件没少、点名件已不在暂存面 ③ 头 %s · 门禁 rc=%d\n",
		progName, len(committed), len(keptBack), head, gtRC)
	for _, f := range committed {
		fmt.Fprintln(stdout, f)
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		onlyRow["head"], onlyRow["staged"] = head, strings.Join(postStaged, ",")
		onlyRow["gate_rc"], onlyRow["commit"] = fmt.Sprintf("%d", gtRC), head
		onlyRow["message_path"], onlyRow["log_dir"] = msgPath, logDir
		return selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, onlyRow)
	}
	return exitOK
}

// commitExcludedNote 被排除件那一片的**口径行**（`N=0` 必写面内未见，不许静默）。
func commitExcludedNote(excluded []string) string {
	if len(excluded) == 0 {
		return "经手未提交（他人暂存，原样保留）：0 件 —— 暂存面此前为空（无他人件）"
	}
	return fmt.Sprintf("经手未提交（他人暂存，原样保留）：%d 件", len(excluded))
}

// runFastGateCapture 跑快速档并**捕获输出**（照实打回；退码直通不翻译）。
// 与 `runFastGate` 同一份口径（同一条命令、同一个 `logDir`），差别只在把 stdout/stderr 收回来判归因。
func runFastGateCapture(root, logDir string) (int, string, error) {
	script := filepath.Join(root, "scripts/gates/precommit-gates.sh")
	if _, err := os.Stat(script); err != nil {
		return exitBlocked, "", fmt.Errorf("门禁最小入口不在（%s）", script)
	}
	cmd := exec.Command("bash", script, "--fast", "--outdir", logDir)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), string(out), nil
		}
		return exitFail, string(out), err
	}
	return exitOK, string(out), nil
}

// gitRunWrite —— 跑一条**会写索引**的 git 命令（唯一用途：新件的意图登记 `git add -N`）。
// ★ 与 `gitRun`（只读契约）分开命名：写面**一处一命名**，免得「只读」这个词被悄悄放宽。
func gitRunWrite(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s：%v（%s）", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// runGitCommitOnly —— `git commit --only -F <件> -- <路径…>`（**不加 `--no-verify`**）。
// 路径走 argv 数组直传（`fork/exec`、**不经 `sh -c`**）⇒ 空格 / 中文 / `$` / 引号天然不被解释。
func runGitCommitOnly(root, msgPath string, paths []string, stdout, stderr io.Writer) int {
	args := append([]string{"-C", root, "commit", "--only", "-F", msgPath, "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "%s: `git commit --only` 起不来：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
