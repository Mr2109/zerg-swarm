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
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/gitpaths"
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
	removed := trimAll(inv.flagVals("--removed"))
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
	// ★ 缺口 `GAP-20261009-17`（「只撤跟踪」第三模式）：`--removed <路径…>` 与另两种模式互斥。
	if len(removed) > 0 && (len(files) > 0 || len(only) > 0) {
		inv.setErr("usage", "mode_conflict", "`--removed` 与 `--file`/`--only` 是三种模式，不许同时给")
		fmt.Fprintf(stderr, "%s: `--removed`（只撤跟踪：临时索引 = HEAD ＋ 点名件的删除态）与 `--file`/`--only` 是**三种模式** ⇒ 同时给退 2\n", progName)
		fmt.Fprintf(stderr, "选一条：`--file <件>…` / `--only <路径>…` / `--removed <路径>…`\n")
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
	if len(removed) > 0 {
		// `--removed` 模式（缺口 `GAP-20261009-17`）：**只撤跟踪** —— 临时索引 = HEAD ＋ 点名件的删除态。
		return repoCommitRemoved(inv, stdout, stderr, removed, message)
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
	// ★ `logDir` 不在这里拼：真跑那一支（④）用 `os.MkdirTemp` **原子新建**（理由见那里）——
	//   拼在这里建会让上面这一支（`--dry-run`：**零副作用**）也落一个临时目录。

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
	// ★ 件名**只**经 A1 出口（`core/internal/gitpaths` · C-1）：`FaceDiffCached` = 索引 vs HEAD，
	// argv 逐字 `git -C <仓> diff --cached --name-only -z` ⇒ `-z` 全免疫（真名逐字节）。
	// 旧写法那枚 `-c core.quotepath=false` 只管非 ASCII 一类，且本件 `gitRun` 已自带一枚（重复）；
	// 这不是放宽判据，是把两处读的**同一个东西**（件名）读成同一种形态。
	preEntries, err := gitpaths.List(root, gitpaths.FaceDiffCached)
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	if len(preEntries) > 0 {
		inv.setErr("usage", "index_not_empty", "暂存面不是空的")
		fmt.Fprintf(stderr, "%s: 暂存面**已经不是空的**（已暂存：%s）⇒ 拒执（退码 2）：\n",
			progName, strings.Join(entryNames(preEntries), " · "))
		fmt.Fprintf(stderr, "本命令只认「按文件名逐件暂存」这一条路 —— 先把别人的暂存清掉（`git reset`），别让它们混进这次提交\n")
		return exitUsage
	}

	// ③′ **门新鲜度闸**（缺口 `GAP-20260928-194` · 设计稿《流程规则程序化 v1.0》§2.2 的 `A2`/`A5` · §4 第 2 件）：
	//   **提交前**先算当前工作树指纹，再去**现有**门跑结果里找「全链 + 含单测步且通过 + 指纹逐字相符」的一趟；
	//   差任何一条 ⇒ 拒（退码 2）并逐条点名 + 给出该跑的完整真命令。**留痕跳道** = `--allow-stale-gates <理由>`
	//   （缺理由 ⇒ 拒 2；理由非空 ⇒ 真写一笔审计行，落不下痕也不许跳）。
	//   ★ 位置在**快速档之前**：闸判的是**别处那一趟**的产物，与本次要不要再跑快速档无关（快速档照旧跑）。
	if rc := gateFreshnessGate(root, "files", inv, stderr); rc != exitOK {
		return rc
	}

	// ④ **逐件暂存 + 复核暂存清单**（缺口 `GAP-20260927-79`：闸取**索引面** ⇒ 物化索引前索引必须
	//   先**含点名件** ⇒ 调序为「先 `git add` 再物化再跑闸」；被拦下时逐件 `git reset` 把索引面复原）。
	// ★ 与旧序（闸在暂存之前）相反的理由：旧序跑闸时索引为空 ⇒ 物化索引只能见 HEAD ⇒ 看不见本次改动。
	addArgs := append([]string{"add", "--"}, files...)
	if _, err := gitRun(root, addArgs...); err != nil {
		inv.setErr("failed", "git_add_failed", err.Error())
		fmt.Fprintf(stderr, "%s: `git add` 失败（逐件暂存）：%v\n", progName, err)
		return exitFail
	}
	postEntries, err := gitpaths.List(root, gitpaths.FaceDiffCached)
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	staged := entryNames(postEntries)
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

	// ④′ **物化索引 + 过快速档**（闸取**索引面**：物化树 = 本次要提交的内容）
	// ★ 2026-09-27（缺口 `GAP-20260927-434` · 与 `gate run` 那一支同形）：原来目录名按**秒级**时间戳拼 +
	//   `MkdirAll` 复用同名目录 ⇒ 同一秒内两次提交**共用同一处**日志目录；门禁脚本按 `--outdir` 读回
	//   `results.tsv` 数失败数（`report`）⇒ 后一次会把前一次落的表算成自己的（**陈旧判决回读**：
	//   这一轮没落表时「不许当绿」被旧表绕开）。
	//   改成 `MkdirTemp`：名字前缀带**纳秒**时间戳（给人看的时序）、后缀由它加一段随机串，且**原子新建**
	//   —— 目录必是**本次调用新建的空目录** ⇒ 这一轮没落表就一定读不回（不靠 mtime 一类脆判据，
	//   也不与任何旧目录重名）。`logDirPattern` 不动：它是计划件里的**模式**（§九 M11 同一判据跑两遍必须一致）。
	prefix := "zerg-repo-commit-" + time.Now().Format("20060102-150405.000000000") + "-"
	logDir, mkErr := os.MkdirTemp(osTempRoot(), prefix+"*")
	if mkErr != nil {
		inv.setErr("failed", "logdir_failed", mkErr.Error())
		fmt.Fprintf(stderr, "%s: 建不了门禁日志目录 %s：%v\n", progName, filepath.Join(os.TempDir(), prefix), mkErr)
		return exitFail
	}
	pruneRepoCommitLogDirs(osTempRoot(), logDir) // ★ 有界回收（缺口 `GAP-20261001-58`：清掉过窗口的旧日志目录；本轮目录 keep 不动 · 见件内 helper）
	matRoot, mErr := materializeIndex(root)
	if mErr != nil {
		inv.setErr("blocked", "index_materialize_failed", mErr.Error())
		fmt.Fprintf(stderr, "%s: **物化索引失败：%v ⇒ 不给结论（退码 8 · 绝不静默回落到工作树面）**\n", progName, mErr)
		if rc := resetStagedPaths(root, files, stderr); rc != exitOK {
			fmt.Fprintf(stderr, "%s: ⚠ 索引面复原失败 ⇒ 逐件暂存仍在：%s\n", progName, strings.Join(staged, " · "))
		}
		return exitBlocked
	}
	setActiveMatRoot(matRoot)
	gtRC, err := runFastGate(root, matRoot, logDir, stderr)
	defer removeActiveMaterialized()
	if err != nil {
		inv.setErr("failed", "gate_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	if gtRC != exitOK {
		// **不给结论 ≠ 绿**：门禁 1（有失败项）/ 2（不给结论）都拦 ⇒ 退码直通；**被拦下 ⇒ 索引面复原**。
		restored := resetStagedPaths(root, files, stderr) == exitOK
		if gtRC == exitFail {
			inv.setErr("failed", "gate_failed", "快速档有失败项")
		} else {
			inv.setErr("usage", "gate_blocked", "快速档不给结论")
		}
		if !restored {
			inv.setErr("failed", "unstage_failed", "被拦下但索引面复原失败")
			fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d · 索引面）⇒ 不提交**（⚠ 索引面复原失败 ⇒ 逐件暂存仍在 · 日志 %s）\n", progName, gtRC, logDir)
			return gtRC
		}
		fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d · 索引面）⇒ 不提交**（索引面与工作树逐字未动 · 日志 %s）\n", progName, gtRC, logDir)
		fmt.Fprintf(stderr, "复核：%s gate run --fast --outdir %s\n", progName, logDir)
		return gtRC
	}
	fmt.Fprintf(stderr, "%s: 快速档 rc=0 ✓（**索引面** · 日志 %s）⇒ 放行本次提交\n", progName, logDir)

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
	// ★ 「轮 = 一笔提交」：一笔提交落一页回执（缺口 `GAP-20261005-16`）—— 只出声，不改判决。
	writeCommitReceipt(message, head, "file", files, by, stderr)
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

// osTempRoot —— 临时根的**注入缝**（件⑤ · 2026-09-29 提速第一批：测试夹具不得污染真临时根）。
// 优先 ZERG_TMP_ROOT（测试/夹具置它 ⇒ t.TempDir()），否则退回 os.TempDir()（生产行为逐字不变）。
func osTempRoot() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_TMP_ROOT")); d != "" {
		return d
	}
	return os.TempDir()
}

// ── 日志目录的**有界回收**（缺口 `GAP-20261001-58` · 提交面）──────────────────────────────
//
// 病灶：本命令每次真跑都 `os.MkdirTemp` 建一枚 `zerg-repo-commit-*` 日志目录（`/private/var/tmp`
// 泄漏的主源之一 —— 该缺口记 2797 枚），从来没人删。
//
// 为什么**不能**照同族「建完就 `defer os.RemoveAll`」（`family_gate_bench.go` / `family_gate_run_step.go`
// 那一形态）：本目录是**输出契约的一部分** —— `--json` 的 `log_dir` / `message_path` 两格逐字指向它，
// 失败面那句「复核：… --outdir <logDir>」也要人在**函数返回之后**照它复跑 ⇒ 当场删＝契约报废
// （`gate run --step` / `gate bench` 的自建目录不进任何契约，故它们能当场删；本命令不能）。
// 故改为**有界保留**：每建一枚新目录，顺手清掉**过窗口**的旧目录（best-effort：删不掉只忽略，
// 绝不影响本轮判决，也绝不碰 `keep`＝本轮新目录）。窗口取值照回仓内同类先例
// （`family_gap.go` 的 `gapPoolTTLDays` 体例：TTL 常量 + `time.Now().Add(-TTL)` 截止）。
const repoCommitLogTTL = 24 * time.Hour

// pruneRepoCommitLogDirs —— 回收**过窗口**的 `zerg-repo-commit-*` 日志目录。
//
// 只动四处能确定的东西：① `root` 下、② 名字前缀 `zerg-repo-commit-`、③ `mtime` 早于截止、
// ④ 且**不是** `keep`（本轮新目录）；**符号链接天然跳过**（`DirEntry.IsDir()` 是 lstat 语义）。
// 任一步读不到/删不掉一律忽略 —— 本函数**不改判决、不改退码、不改任何输出**。
func pruneRepoCommitLogDirs(root, keep string) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cut := time.Now().Add(-repoCommitLogTTL)
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "zerg-repo-commit-") {
			continue
		}
		p := filepath.Join(root, e.Name())
		if p == keep {
			continue
		}
		fi, err := e.Info()
		if err != nil || fi.ModTime().After(cut) {
			continue
		}
		_ = os.RemoveAll(p)
	}
}

// ═══════════ 提交闸取「索引面」：物化索引（缺口 `GAP-20260927-79` · 2026-10-08）═══════════
//
// 病根（现读坐实）：提交闸对**工作树**取证而非**索引面** —— 31/31 快档步内容面取自工作树、0/31
// 取自索引 ⇒ **未跟踪件 / 未暂存件会替提交者变绿**（假绿）。
//
// 修法（逐字照 want）：跑闸前 `git checkout-index -a -f --prefix=<仓外同级临时目录>/` **物化索引**，
// 再以 `ZERG_GATE_REPO_ROOT=<该目录>` 跑闸（`scripts/gates/precommit-gates.sh:196` 的**专用旋钮** ——
// 本件零改闸脚本）。★ 明确**排除 `git archive HEAD`**：HEAD ≠ 索引 ⇒ 对 HEAD 取证同样会给假绿。
//
// ★ 物化树里补一枚 `.git` 符号链接（指向真仓 `.git`）：门82（hooksPath 挂载链）等**依赖 `.git`**
// 的步在物化树里跑 —— 没有 `.git` 时 `git config --local core.hooksPath` 读不到 ⇒ 判红（实测）。
// 门禁步全体只读 ⇒ 只读共享真 `.git`，不写仓。移除时**先摘链接再删目录**（`os.RemoveAll` 不跟链，
// 但显式摘更稳）。物化失败 ⇒ 调用方按**不给结论**（退 8）处置，绝不静默回落到工作树面。
//
//	★ 另补一枚 `bin` 符号链接（→ 真仓 `bin/`）：`bin/` **未跟踪** ⇒ 物化树里没有它，而门跑面有
//	**依赖已建制品**的步（矩阵两档档位判据 / `check-gap-ledger-view` 导出器守卫）⇒ 缺它假红；
//	同 `.git` 那条（依赖非索引件就补链）。
func materializeIndex(root string) (string, error) {
	dir, err := os.MkdirTemp(filepath.Dir(root), ".zerg-gate-idx-*")
	if err != nil {
		return "", err
	}
	prefix := dir + string(os.PathSeparator)
	if _, err := gitRun(root, "checkout-index", "-a", "-f", "--prefix="+prefix); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(dir, ".git")); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	// ★ 再补一枚 `bin` 链接（→ 真仓 `bin/`）—— 与 `.git` 同一条理由，且**必须**补：
	//   `bin/` **未被 git 跟踪** ⇒ 物化树（只含索引里的件）里**没有它** ⇒ 门跑面里**依赖已建制品**的步
	//   会假红：`core: 受影响包 go test` 的两档矩阵判据（`cli_matrix_test.go` 以「仓根有没有 `bin/`」
	//   定 local/public 档 —— 物化树缺 `bin/` ⇒ 被判成公开面档 ⇒ 7 条两档格全判不过）与
	//   `check-gap-ledger-view --self-test`（导出器在盘守卫拿 `<root>/bin/zerg` ⇒ 缺它整片 rc=2）。
	//   实测：物化树给 `bin` 链接后，两层 go 测试面全绿；`bin/` 本就没建（真仓没建过）= 缺件，照旧由门跑
	//   按缺件处置 ⇒ 不建链接。
	if st, err := os.Stat(filepath.Join(root, "bin")); err == nil && st.IsDir() {
		if err := os.Symlink(filepath.Join(root, "bin"), filepath.Join(dir, "bin")); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
}

// removeMaterialized —— 自回收物化树（先摘 `.git` / `bin` 两枚链接再删目录）。
func removeMaterialized(dir string) {
	if dir == "" {
		return
	}
	_ = os.Remove(filepath.Join(dir, ".git"))
	_ = os.Remove(filepath.Join(dir, "bin"))
	_ = os.RemoveAll(dir)
}

// ── 物化树在飞登记（缺口 `GAP-20261012-07`）──────────────────────────
// 信号打断走 `os.Exit`（**不跑 defer**）⇒ 物化树会漏；主 goroutine 与信号处理器 goroutine
// 并发读写同一枚登记位 ⇒ 用互斥守住。正常收口与信号面两路共用 `removeActiveMaterialized`。
var activeMatRootMu sync.Mutex

// activeMatRoot —— 当前**在飞**的物化索引树（空串 = 无）。
var activeMatRoot string

// setActiveMatRoot —— 登记在飞的物化树（摘完由 `removeActiveMaterialized` 置空）。
func setActiveMatRoot(dir string) {
	activeMatRootMu.Lock()
	activeMatRoot = dir
	activeMatRootMu.Unlock()
}

// removeActiveMaterialized —— 回收当前在飞的物化树（正常收口与信号处理器两路共用 · 幂等）。
func removeActiveMaterialized() {
	activeMatRootMu.Lock()
	dir := activeMatRoot
	activeMatRoot = ""
	activeMatRootMu.Unlock()
	removeMaterialized(dir)
}

// gateEnv —— 在现环境上叠加 `ZERG_GATE_REPO_ROOT`（物化树）：闸脚本据此**自动改面**（REPO_ROOT）。
//
// ★ 2026-10-09（出口② 血证 · 门①/门82 只在提交钩子面假红）：`git commit --only <路径…>` 会把
//
//	`GIT_INDEX_FILE` 指到它自己的**临时索引**（`<仓>/.git/next-index-<pid>.lock`）并**泄进 pre-commit
//	钩子**；钩子物化树 + 跑闸时那段 git 索引面若被继承，门步里凡读 `git ls-files` 的（门① 覆盖 /
//	门82 hooksPath）就取到**非暂存面**（更糟：那段临时索引还会被本趟 `go test` 的夹具改写）⇒ 假红。
//	⇒ 闸子进程一律**剥掉** `GIT_INDEX_FILE` / `GIT_DIR` / `GIT_WORK_TREE`（只留本仓默认索引面）。
//	★ 与 `.githooks/pre-commit` 的同一处剥离**同规同批**（两条出口同规）。
//
// ★ 2026-10-10（缺口 `GAP-20260927-70`）：再**钉**门的状态目录（见 `pinGateStateDir`）—— 外部继承的
//
//	`ZERG_STATE_DIR` 不再被采信（指沙箱 ⇒ 状态目录门 rc=2 假红）。与 `scripts/gates/precommit-gates.sh`
//	的封口（`gate_seal_env`）**同规同批**。
func gateEnv(matRoot string) []string {
	env := os.Environ()
	if matRoot != "" {
		env = append(env, "ZERG_GATE_REPO_ROOT="+matRoot)
	}
	return pinGateStateDir(stripGitIndexFace(env))
}

// stripGitIndexFace —— 从环境里逐条剔掉泄漏的 git 索引面三键（`GIT_INDEX_FILE` / `GIT_DIR` /
// `GIT_WORK_TREE`）。名字一律**全等**匹配（✗ 不做前缀/模糊），免得连坐别的面。
func stripGitIndexFace(env []string) []string {
	drop := map[string]bool{"GIT_INDEX_FILE": true, "GIT_DIR": true, "GIT_WORK_TREE": true}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if drop[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// ── 门的状态目录口径（缺口 `GAP-20260927-70` · 2026-10-10）────────────────────────────────────
//
// 病灶：门**盲信外部继承的** `ZERG_STATE_DIR` —— 指沙箱/不存在目录时，读它的子门把「取不到数」记成
// rc=2 假红（头号假红源）。修法：门内一律以门专用旋钮 `ZERG_GATE_STATE_DIR`（缺省 `~/.zerg/state`）
// 为准（照同族 `ZERG_GATE_REPO_ROOT` 的「本件专用旋钮」体例）—— 外部继承值一律不再采信。
const gateStateDirKnob = "ZERG_GATE_STATE_DIR"

// gateStateDirKnobValue —— 门专用旋钮的现读值（**空串 = 没给**，与「给了空串」同档）。
func gateStateDirKnobValue() string { return strings.TrimSpace(os.Getenv(gateStateDirKnob)) }

// pinGateStateDir —— 把 env 里的 `ZERG_STATE_DIR` 按门内口径**重定**（缺口 `GAP-20260927-70`）：
//
//	· 显式给了门旋钮 `ZERG_GATE_STATE_DIR` ⇒ 把 `ZERG_STATE_DIR` **钉**成它（合法隔离跑法）；
//	· 没给 ⇒ 逐条**剔掉**继承来的 `ZERG_STATE_DIR`（✗ 不再采信外部继承值 —— 子门落自己的缺省
//	  `~/.zerg/state`）。
//
// 照 `stripGitIndexFace` 同规：名字一律**全等**匹配（✗ 不做前缀/模糊），只动这一枚、其余一字不改。
// 「没给就剔」而不是「没给就钉成缺省值」：后者会让子门恒看到该变量非空 ⇒ 门⑰/门㉜/门⑱ 的「沙箱
// 读数」警告在正常跑里误报（而值恰恰就是真缺省）⇒ 取「剔」这一档，正常跑与改前逐字同。
func pinGateStateDir(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if kv == "ZERG_STATE_DIR" || strings.HasPrefix(kv, "ZERG_STATE_DIR=") {
			continue
		}
		out = append(out, kv)
	}
	if knob := gateStateDirKnobValue(); knob != "" {
		out = append(out, "ZERG_STATE_DIR="+knob)
	}
	return out
}

// resetStagedPaths —— **被拦下时复原索引面**（逐件 `git reset -q -- <路径…>`）：调序后索引面在跑闸前
// 已含点名件 ⇒ 拦住/不给结论时必须把它**复原**，否则违背「被拦下时索引面与工作树逐字未动」。
// 复原失败 ⇒ 照实出声并退非 0（判决照旧，但**不许谎称「索引面未动」**）。
func resetStagedPaths(root string, paths []string, stderr io.Writer) int {
	if len(paths) == 0 {
		return exitOK
	}
	if _, err := gitRun(root, append([]string{"reset", "-q", "--"}, paths...)...); err != nil {
		fmt.Fprintf(stderr, "  ⚠ 撤销暂存失败（索引面留在暂存态）：%v\n", err)
		return exitFail
	}
	return exitOK
}

// runFastGate —— 跑快速档（**退码直通**：不翻译、不吞输出）。
// `matRoot` = 物化索引树（缺口 `GAP-20260927-79`）：闸取**索引面**而非工作树。
func runFastGate(root, matRoot, logDir string, stderr io.Writer) (int, error) {
	script := filepath.Join(root, "scripts/gates/precommit-gates.sh")
	if _, err := os.Stat(script); err != nil {
		return exitBlocked, fmt.Errorf("门禁最小入口不在（%s）", script)
	}
	cmd := armChildGroup(exec.Command("bash", script, "--fast", "--outdir", logDir))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Env = gateEnv(matRoot)
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
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "commit", "-F", msgPath)
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

// entryNames —— A1 出口（`gitpaths.Entry`）→ 逐件名文本：**只渲染、不开第二个出口**。
// 出口的字节面在这里派生一次（C-10「字符串只在末端派生」），供本件的对拍与打印用。
func entryNames(entries []gitpaths.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.String())
	}
	return out
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

// fallbackAuditPath —— **备用**审计落点（与主落点同目录：`<状态目录>/audit_fallback.jsonl`）。
//
// ★ 它**不是**主落点的替身：**只有**「主落点写不进」这件事本身落到这里（一行一事件 · 追加只写）。
// 本命令的判决与退码**一字不变**（提交/签名不可逆 ⇒ 升 fail-closed 会把「审计盘满」变成「提交做不了」）。
func fallbackAuditPath() string { return filepath.Join(stateDirOf(), "audit_fallback.jsonl") }

// auditCompensationLine —— **补偿审计行**：「主落点写不进」这件事本身的一行（落 `audit_fallback.jsonl`）。
//
// ★ 硬口径：这一行白纸黑字带**主落点路径**（`audit_path`）+ **失败原因**（`reason` 归类 · `note` 原文）；
// 字段名照同件体例（`snake_case` · 与 `commitAuditLine` 同族）。
type auditCompensationLine struct {
	At        string `json:"at"`
	Event     string `json:"event"`          // 恒 `audit_compensated`
	Reason    string `json:"reason"`         // 恒 `audit_unwritable`
	By        string `json:"by,omitempty"`   // 谁（commit=`--by` · approve=批准人）
	AuditPath string `json:"audit_path"`     // **主落点**（写不进的那一处）
	Note      string `json:"note,omitempty"` // 失败原因原文（`error.Error()`）
}

// appendAuditCompensation —— 把「主落点失败」写一条到**备用落点**（`O_APPEND` · 一行一事件）。
//
// ★ 备用落点也写不进 ⇒ 返回 err：调用方**只**出声，**不**改判决、**不**改退码（不假报失败、
// 不掩盖「提交/签名已成功」这个事实）。本函数**不**引入任何 env 开关。
func appendAuditCompensation(by, primaryPath string, cause error) error {
	line := auditCompensationLine{
		At: time.Now().Format(time.RFC3339), Event: "audit_compensated", Reason: "audit_unwritable",
		By: by, AuditPath: primaryPath, Note: cause.Error(),
	}
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	p := fallbackAuditPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

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

// readGateResultsStatuses 读门禁结果表 `<outdir>/results.tsv` 的**状态列**。
//
// 列序（真源 = `scripts/gates/precommit-gates.sh:run_step` 的落行格式）：`状态\t步名\trc\t秒\t日志`。
// ⇒ 回 map[步名]状态（PASS/FAIL/BLOCKED/REPORT）。读不到 / 0 行 ⇒ `(nil, err)` —— 调用方
// **fail-closed**（`GAP-20260927-132` ④：不许拿「读不到」当豁免依据）。
// ★ 它是替代「对整段门禁输出做子串包含」的那一处取数面：豁免判据只认**状态列**，✗ 不认自由文本。
func readGateResultsStatuses(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		f := strings.Split(ln, "\t")
		if len(f) < 2 {
			continue
		}
		out[f[1]] = f[0]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("结果表 0 行")
	}
	return out, nil
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
	// ★ ② 点名件匹配判定：**在撞车判定之前**（2026-09-30 修 · 矩阵偶发退码 14 的真因）——
	//   「点名的件一件都没命中 = 我敲错了」是**永久**用法错（退 2）；`.git/index.lock` 在是**瞬时**被占
	//   （退 14 · 「可等可重来」）。**永久判决不许被瞬时态挡住**：等下去也不会把敲错的件名等对，
	//   等完拿到的仍是同一个 2 ⇒ 先报 14 是把人支去等一件等不成的事。
	//   这一判只是只读 `git status`（`onlyMatchPaths`）—— **不写索引、不取锁** ⇒ 别的进程持锁时照样判得出，
	//   故「缺件优先于参数校验」那条理由（判不出才先报缺件）在这里**不适用**。
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
	// ⑤ 撞车：别人正在写这个仓（**点名件全部命中之后**才判被占 —— 理由见上）
	if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); err == nil {
		inv.setErr("conflict", "repo_busy", "另一个 git 进程正在写这个仓")
		fmt.Fprintf(stderr, "%s: `.git/index.lock` 在 ⇒ **被占**（另有 git 进程在写这个仓）—— 退码 14（可等可重来）\n", progName)
		return exitConflict
	}

	// ② 提交前读数：别人的暂存面（**允许非空** —— 那是本模式存在的理由）+ 点名件匹配面
	preEntries, err := gitpaths.List(root, gitpaths.FaceDiffCached)
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	preStaged := entryNames(preEntries)
	excluded := setMinus(preStaged, paths)
	// ②′ 新件（未跟踪）：`git commit --only -- <未跟踪件>` 会 `did not match any file(s) known to git`
	// ⇒ 先做**意图登记**（`git add -N`：只把路径记进索引、不写内容、不动别人的暂存 —— 这是 git 的硬要求，
	// 不是「暂存」）。现读实测：不带这一步，新件一提交就 `rc=1`（`pathspec did not match`）。
	untracked := []string{}
	uargs := append([]string{"-c", "core.quotepath=false", "ls-files", "--others", "--exclude-standard", "--"}, paths...)
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
	// ★ `logDir` 不在这里拼：真跑那一支（④）用 `os.MkdirTemp` **原子新建**（理由见那里）——
	//   拼在这里建会让上面这一支（`--dry-run`：**零副作用**）也落一个临时目录。
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

	// ③′ **门新鲜度闸**（缺口 `GAP-20260928-194`）：同默认模式那一处 —— 本闸只管「提交」这一个落地动作。
	if rc := gateFreshnessGate(root, "only", inv, stderr); rc != exitOK {
		return rc
	}

	// ④ **先暂存点名件**（缺口 `GAP-20260927-79`：闸取**索引面** ⇒ 物化索引前索引须含点名件）。
	//   ★ 旧序对未跟踪新件**只**做 `git add -N` 意图登记 ⇒ 索引里没有内容 ⇒ 物化索引看不见改动；
	//     调序为「先 `git add` 再物化再跑闸」；被拦下时逐件 `git reset` 复原（别人的暂存一枚不动）。
	if _, err := gitRun(root, append([]string{"add", "--"}, matched...)...); err != nil {
		inv.setErr("failed", "git_add_failed", err.Error())
		fmt.Fprintf(stderr, "%s: `git add` 失败（暂存点名件）：%v ⇒ 不提交\n", progName, err)
		return exitFail
	}

	// ④′ **物化索引 + 跑快速档**（闸取**索引面**：物化树 = 本次要落的内容）；红时给归因三格 + 例外旗标判定。
	// ★ 2026-09-27（缺口 `GAP-20260927-434` · 与 `gate run` 那一支同形）：原来目录名按**秒级**时间戳拼 +
	//   `MkdirAll` 复用同名目录 ⇒ 同一秒内两次提交**共用同一处**日志目录；门禁脚本按 `--outdir` 读回
	//   `results.tsv` 数失败数（`report`）⇒ 后一次会把前一次落的表算成自己的（**陈旧判决回读**）。
	//   改成 `MkdirTemp`：名字前缀带**纳秒**时间戳（给人看的时序）、后缀由它加一段随机串，且**原子新建**
	//   —— 目录必是**本次调用新建的空目录** ⇒ 这一轮没落表就一定读不回（不靠 mtime 一类脆判据，
	//   也不与任何旧目录重名）。`logDirPattern` 不动：它是计划件里的**模式**（§九 M11 同一判据跑两遍必须一致）。
	prefix := "zerg-repo-commit-" + time.Now().Format("20060102-150405.000000000") + "-"
	logDir, mkErr := os.MkdirTemp(osTempRoot(), prefix+"*")
	if mkErr != nil {
		inv.setErr("failed", "logdir_failed", mkErr.Error())
		fmt.Fprintf(stderr, "%s: 建不了门禁日志目录 %s：%v\n", progName, filepath.Join(os.TempDir(), prefix), mkErr)
		return exitFail
	}
	pruneRepoCommitLogDirs(osTempRoot(), logDir) // ★ 有界回收（缺口 `GAP-20261001-58`：清掉过窗口的旧日志目录；本轮目录 keep 不动 · 见件内 helper）
	matRoot, mErr := materializeIndex(root)
	if mErr != nil {
		inv.setErr("blocked", "index_materialize_failed", mErr.Error())
		fmt.Fprintf(stderr, "%s: **物化索引失败：%v ⇒ 不给结论（退码 8 · 绝不静默回落到工作树面）**\n", progName, mErr)
		if rc := resetStagedPaths(root, matched, stderr); rc != exitOK {
			fmt.Fprintf(stderr, "%s: ⚠ 索引面复原失败 ⇒ 逐件暂存仍在：%s\n", progName, strings.Join(matched, " · "))
		}
		return exitBlocked
	}
	setActiveMatRoot(matRoot)
	gtRC, gtOut, err := runFastGateCapture(root, matRoot, logDir)
	defer removeActiveMaterialized()
	if err != nil {
		inv.setErr("failed", "gate_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	fmt.Fprint(stderr, gtOut)
	if gtRC != exitOK {
		// ★ `GAP-20260927-132` ④（2026-10-06）：豁免的**成立判据**改读 `<logDir>/results.tsv` 的**状态列**
		//   —— 旧形态是对整段门禁输出做**子串包含**（点任一步名即可豁免整趟，且 FAIL / BLOCKED 一视同仁）。
		//   三条口径：① 点名的步必须在**结果表状态列**里出现（✗ 不认自由文本）；
		//     ② **只许豁免 FAIL**（点名到 PASS/REPORT ⇒ 不成立）；③ **禁豁免 BLOCKED** ——
		//     结果表里**只要还有任何 BLOCKED 步**，本次一律不豁免（「不给结论」≠ 红 ⇒ 不许一键洗掉）。
		//   结果表读不到 / 0 行 ⇒ **fail-closed**：不许拿「读不到」当豁免依据。
		waivedOK := len(waive) > 0 && reason != ""
		missing := []string{} // 点名了、结果表里没有（或那一行不是 FAIL）的步
		banned := []string{}  // 点名到 BLOCKED（禁豁免）的步
		hasBlocked := false   // 结果表里是否还有任何 BLOCKED 步
		statusPath := filepath.Join(logDir, "results.tsv")
		statuses, sErr := readGateResultsStatuses(statusPath)
		if sErr != nil || len(statuses) == 0 {
			why := "结果表 0 行"
			if sErr != nil {
				why = sErr.Error()
			}
			fmt.Fprintf(stderr, "%s: `--waive` **判不了**：读不到结果表 %s（%s）⇒ 不许豁免（fail-closed）\n",
				progName, statusPath, why)
			waivedOK = false
		} else {
			for _, st := range statuses {
				if st == "BLOCKED" {
					hasBlocked = true
				}
			}
			for _, w := range waive {
				st, ok := statuses[w]
				switch {
				case !ok:
					missing = append(missing, w)
				case st == "BLOCKED":
					banned = append(banned, w)
				case st != "FAIL":
					missing = append(missing, w+"（结果表状态="+st+"）")
				}
			}
			if len(missing) > 0 || len(banned) > 0 || hasBlocked {
				waivedOK = false
			}
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
				// ★ 补偿行：把「主落点失败」本身记账到备用落点（**不**改判决 · **不**改退码）。
				if cerr := appendAuditCompensation(by, auditPath, aerr); cerr == nil {
					fmt.Fprintf(stderr, "%s: 已把「主落点失败」记一条补偿行到备用落点 %s\n", progName, fallbackAuditPath())
				} else {
					fmt.Fprintf(stderr, "%s: ⚠ 补偿行也落不下（%s）：%v —— 只出声（判决/退码一字不变）\n",
						progName, fallbackAuditPath(), cerr)
				}
			}
			if gtRC == exitFail {
				inv.setErr("failed", "gate_failed", "快速档有失败项")
			} else {
				inv.setErr("usage", "gate_blocked", "快速档不给结论")
			}
			fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d）⇒ 不提交**（索引面与工作树逐字未动 · 日志 %s）\n", progName, gtRC, logDir)
			if len(missing) > 0 {
				fmt.Fprintf(stderr, "  `--waive` 点名的 %s **不在**这一次的**结果表状态列**上（或那一行不是 FAIL）⇒ 例外旗标不生效（豁免不许挂在空气上）\n", strings.Join(missing, " · "))
			}
			if len(banned) > 0 {
				fmt.Fprintf(stderr, "  `--waive` 点名的 %s 在结果表里是 **BLOCKED（不给结论）** ⇒ **禁豁免**（只许豁免 FAIL）\n", strings.Join(banned, " · "))
			}
			if hasBlocked {
				fmt.Fprintf(stderr, "  结果表里还有 **BLOCKED（不给结论）** 的步 ⇒ 本次一律**不豁免**（不给结论 ≠ 红 ⇒ 不许一键洗掉整趟）\n")
			}
			fmt.Fprintf(stderr, "  归因：步名逐字见上面门禁原样输出 · 日志目录 %s · 复核命令 `bash scripts/gates/precommit-gates.sh --fast --outdir %s`\n", logDir, logDir)
			fmt.Fprintf(stderr, "  合法出口三个：① 等并行写者收工后重跑 ② 按日志逐条复核 ③ 确需人签例外 ⇒ `--waive <步名> --reason <…> --yes`（**进审计**）\n")
			// 缺口 `GAP-20260927-79`：调序后索引面在跑闸前已含点名件 ⇒ **被拦下必须复原**（别人的暂存一枚不动）。
			if rrc := resetStagedPaths(root, matched, stderr); rrc != exitOK {
				fmt.Fprintf(stderr, "%s: ⚠ 索引面复原失败 ⇒ 逐件暂存仍在：%s\n", progName, strings.Join(matched, " · "))
			}
			return gtRC
		}
	} else {
		fmt.Fprintf(stderr, "%s: 快速档 rc=0 ✓（**索引面** · 日志 %s）⇒ 放行本次提交\n", progName, logDir)
	}

	// ⑤ 提交：点名件已在 ④ **暂存**（缺口 `GAP-20260927-79` 调序 —— 物化索引须先含内容；不再需要
	// `git add -N` 意图登记）；pathspec 走 argv（`exec.Command` 经 fork/exec、不经 `sh -c`）。
	// `git commit --only` 仍**只落点名件**（别人的暂存件原样保留）。
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
	postEntries, perr := gitpaths.List(root, gitpaths.FaceDiffCached)
	postStaged := []string{}
	if perr == nil {
		postStaged = entryNames(postEntries)
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
		// ★ 补偿行：把「主落点失败」本身记账到备用落点（**不**改判决 · **不**改退码 · **不**升 fail-closed）。
		if cerr := appendAuditCompensation(by, auditPath, aerr); cerr == nil {
			fmt.Fprintf(stderr, "%s: 已把「主落点失败」记一条补偿行到备用落点 %s\n", progName, fallbackAuditPath())
		} else {
			fmt.Fprintf(stderr, "%s: ⚠ 补偿行也落不下（%s）：%v —— 只出声（判决/退码一字不变）\n",
				progName, fallbackAuditPath(), cerr)
		}
	}

	// ★ 「轮 = 一笔提交」：一笔提交落一页回执（缺口 `GAP-20261005-16`）—— 只在本笔真落地时写。
	if rc == exitOK {
		writeCommitReceipt(message, head, "only", paths, by, stderr)
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

// ══════════════════════════ `GAP-20261009-17`：「只撤跟踪」模式（`repo commit --removed`）══════════════════════════
//
// 病根：`repo commit` 落不下**纯删除面**（把某件从跟踪面摘掉、**件留盘**）—— 两模式各有一处堵死：
//
//	· `--file` 模式要求索引面**先空**（删除面挂索引即拒执 rc=2）；
//	· `--only` 模式走 `git commit --only`，它取**工作树内容** ⇒ 结构上表达不了「索引删、工作树留」，
//	  反而把已撤索引的件**加回**。
//
// 修法（Mr2109 代拍 · 窄口径 · 缺口 `GAP-20261009-17`）：临时索引 = HEAD 全量 ＋ 点名件的**删除态**
// ⇒ 用该临时索引 commit ⇒ **真索引一字不动** ⇒ 别人的暂存**天然原样保留**（不靠「先记后还原」两趟）。
// ★ 全程**只动索引/临时索引**，✗ 绝不碰工作树件（「件留盘」是本模式的定义）。
func repoCommitRemoved(inv *invocation, stdout, stderr io.Writer, paths []string, message string) int {
	// ⓪ 多余位置参数 ⇒ 拒执（照 `--only` 同一口径：多件要逐件 `--removed`）。
	if len(inv.args) > 0 {
		inv.setErr("usage", "removed_extra_positional", "`--removed` 一次只吃一枚值：多件要逐件 `--removed`")
		fmt.Fprintf(stderr, "%s: `--removed` **一次只吃一枚值** —— 多件要逐件写（`--removed <件1> --removed <件2> …`）\n", progName)
		fmt.Fprintf(stderr, "多余的 %d 个位置参数：%s\n", len(inv.args), strings.Join(inv.args, " · "))
		return exitUsage
	}
	// ① 禁词 / 通配：**照默认模式那张表逐字**（`--removed` 不是批量暂存的后门）。
	for _, f := range paths {
		for _, bad := range gitForbiddenStaging {
			if f == bad {
				inv.setErr("usage", "bulk_staging_forbidden", "禁批量暂存")
				fmt.Fprintf(stderr, "%s: `--removed %s` 是**批量暂存**写法 ⇒ 拒执（退码 2）\n", progName, f)
				return exitUsage
			}
		}
		if strings.ContainsAny(f, "*?[") || strings.HasPrefix(f, ":") {
			inv.setErr("usage", "glob_staging_forbidden", "禁通配")
			fmt.Fprintf(stderr, "%s: `--removed %s` 含通配/路径规格 ⇒ 拒执（退码 2 · 一次只点名一件件）\n", progName, f)
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
	// ② 点名件逐条必须是**当前索引里的删除态**（照 want 的复现：先 `git rm --cached <件>`）——
	//    否则本模式的语义不成立（「只撤跟踪」的前提正是它已被撤出索引）。
	for _, p := range paths {
		if !removedIsDeletion(root, p) {
			inv.setErr("usage", "removed_not_staged_deletion", "点名件不在挂索引的删除面上")
			fmt.Fprintf(stderr, "%s: `--removed %s` 不在**挂索引的删除面**上 ⇒ 拒执（退码 2）\n", progName, p)
			fmt.Fprintf(stderr, "本模式只认「已 `git rm --cached <件>`（撤索引 · 件留盘）」那一态；先撤索引再来\n")
			return exitUsage
		}
	}
	// ③ 撞车：别人正在写这个仓
	if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); err == nil {
		inv.setErr("conflict", "repo_busy", "另一个 git 进程正在写这个仓")
		fmt.Fprintf(stderr, "%s: `.git/index.lock` 在 ⇒ **被占**（另有 git 进程在写这个仓）—— 退码 14（可等可重来）\n", progName)
		return exitConflict
	}
	// ④ 提交前读数：真索引面（= HEAD ＋ 删除态 ＋ 别人的暂存）—— 别人的暂存本模式**天然不动**。
	preEntries, err := gitpaths.List(root, gitpaths.FaceDiffCached)
	if err != nil {
		inv.setErr("blocked", "git_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了暂存面：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	preStaged := entryNames(preEntries)
	excluded := setMinus(preStaged, paths)
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未声明）"
	}
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	trace := strings.TrimSpace(inv.flagVal("--trace"))
	criterion := strings.TrimSpace(inv.flagVal("--criterion"))
	msg := commitMessageTemplate(message, proposalID, by, trace, criterion, paths)
	logDirPattern := filepath.Join(os.TempDir(), "zerg-repo-commit-<YYYYMMDD-HHMMSS>")
	auditPath := commitAuditPath()
	removedRow := map[string]string{
		"head": "", "files": strings.Join(paths, ","), "staged": strings.Join(preStaged, ","),
		"gate_rc": "", "commit": "", "message_path": "", "log_dir": logDirPattern,
	}
	// ⑤ 三态：`--dry-run` ⇒ 计划件 rc=0；缺 `--yes` ⇒ 计划件 + rc=2；确认档齐 ⇒ 真跑。
	if inv.dryRun || !inv.yes {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未暂存、未提交、未跑门禁）")
		fmt.Fprintf(stdout, "  动作     : %s repo commit **--removed**（只撤跟踪；临时索引 = HEAD ＋ 点名件的删除态）\n", progName)
		fmt.Fprintf(stdout, "  仓       : %s\n", root)
		fmt.Fprintf(stdout, "  撤跟踪件 : %d 件 —— %s（件**留盘**，只从跟踪面摘掉）\n", len(paths), strings.Join(paths, " · "))
		fmt.Fprintf(stdout, "  进提交的路: 临时索引 = HEAD ＋ 点名件删除态 ⇒ git commit -F <临时件>（**不动真索引** ⇒ 别人的暂存原样保留）\n")
		fmt.Fprintf(stdout, "  别人的暂存: %s\n", commitExcludedNote(excluded))
		fmt.Fprintf(stdout, "  门禁     : bash scripts/gates/precommit-gates.sh --fast --outdir %s（**退码直通**）\n", logDirPattern)
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
			_ = selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, removedRow)
		}
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未跑门禁、未暂存、未提交）")
		if !inv.yes && !inv.dryRun {
			fmt.Fprintf(stderr, "%s: `repo commit --removed` 要 `--yes`（D2 档：提交可逆 —— `git reset --soft HEAD~1`）⇒ 缺它不执行\n", progName)
			return exitUsage
		}
		return exitOK
	}

	// ⑤′ **门新鲜度闸**（缺口 `GAP-20260928-194`）：同另两模式同一处 —— 本闸只管「提交」这一个落地动作。
	if rc := gateFreshnessGate(root, "removed", inv, stderr); rc != exitOK {
		return rc
	}

	// ⑥ **造临时索引 = HEAD ＋ 点名件的删除态**（★ 只动临时索引 · 真索引一字不动）
	logDir, mkErr := os.MkdirTemp(osTempRoot(), "zerg-repo-commit-"+time.Now().Format("20060102-150405.000000000")+"-*")
	if mkErr != nil {
		inv.setErr("failed", "logdir_failed", mkErr.Error())
		fmt.Fprintf(stderr, "%s: 建不了门禁日志目录：%v\n", progName, mkErr)
		return exitFail
	}
	pruneRepoCommitLogDirs(osTempRoot(), logDir) // ★ 有界回收（缺口 `GAP-20261001-58`）
	tmpIndex := filepath.Join(root, ".git", fmt.Sprintf("zerg-removed-index-%d", os.Getpid()))
	defer os.Remove(tmpIndex)
	if _, err := gitRunIndex(root, tmpIndex, "read-tree", "HEAD"); err != nil {
		inv.setErr("failed", "tmp_index_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 造不了临时索引（read-tree HEAD）：%v ⇒ 不提交\n", progName, err)
		return exitFail
	}
	if _, err := gitRunIndex(root, tmpIndex, append([]string{"update-index", "--force-remove", "--"}, paths...)...); err != nil {
		inv.setErr("failed", "tmp_index_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 临时索引置删除态失败：%v ⇒ 不提交\n", progName, err)
		return exitFail
	}
	// ⑦ 物化**临时索引** + 跑快速档（闸取**本次要落的内容面** = 临时索引；与缺省/`--only` 两模式同源）
	matRoot, mErr := materializeTempIndex(root, tmpIndex)
	if mErr != nil {
		inv.setErr("blocked", "index_materialize_failed", mErr.Error())
		fmt.Fprintf(stderr, "%s: **物化临时索引失败：%v ⇒ 不给结论（退码 8 · 绝不静默回落到工作树面）**\n", progName, mErr)
		return exitBlocked
	}
	setActiveMatRoot(matRoot)
	gtRC, err := runFastGate(root, matRoot, logDir, stderr)
	defer removeActiveMaterialized()
	if err != nil {
		inv.setErr("failed", "gate_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	if gtRC != exitOK {
		// ★ 本模式**从未动过真索引**（删除面挂在临时索引上）⇒ 被拦下时索引面与工作树**天然逐字未动**。
		if gtRC == exitFail {
			inv.setErr("failed", "gate_failed", "快速档有失败项")
		} else {
			inv.setErr("usage", "gate_blocked", "快速档不给结论")
		}
		line := commitAuditLine{At: time.Now().Format(time.RFC3339), Event: "gate_blocked", By: by,
			Mode: "removed", Only: paths, Excluded: excluded, GateRC: gtRC, AuditPath: auditPath}
		if aerr := appendCommitAudit(auditPath, line); aerr != nil {
			fmt.Fprintf(stderr, "%s: ⚠ 撞红那一行审计落不下：%v\n", progName, aerr)
		}
		fmt.Fprintf(stderr, "%s: **快速档没过（rc=%d）⇒ 不提交**（真索引与工作树逐字未动 · 日志 %s）\n", progName, gtRC, logDir)
		return gtRC
	}
	fmt.Fprintf(stderr, "%s: 快速档 rc=0 ✓（临时索引面 · 日志 %s）⇒ 放行本次提交\n", progName, logDir)

	// ⑧ 提交：用**临时索引** commit（★ 真索引一字不动 ⇒ 别人的暂存原样保留）
	msgPath := filepath.Join(logDir, "commit-message.txt")
	if err := os.WriteFile(msgPath, []byte(msg), 0o600); err != nil {
		inv.setErr("failed", "message_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 提交信息临时件写不了：%v\n", progName, err)
		return exitFail
	}
	preHead, _ := gitRun(root, "rev-parse", "--short", "HEAD")
	preHead = strings.TrimSpace(preHead)
	rc := runGitCommitIndex(root, tmpIndex, msgPath, stdout, stderr)
	head := preHead
	if rc == exitOK {
		if h, err := gitRun(root, "rev-parse", "--short", "HEAD"); err == nil {
			head = strings.TrimSpace(h)
		}
	}
	// ⑨ 三格对拍：① 落库的件集 == 点名件 ② 别人的暂存一枚没少 ③ 工作树件仍在（本模式不碰工作树）
	committed := []string{}
	if rc == exitOK {
		if s, err := gitRun(root, "-c", "core.quotepath=false", "show", "--name-only", "--pretty=format:", "HEAD"); err == nil {
			committed = splitLines(s)
		}
	}
	postEntries, perr := gitpaths.List(root, gitpaths.FaceDiffCached)
	postStaged := []string{}
	if perr == nil {
		postStaged = entryNames(postEntries)
	}
	same, extra, missingF := compareFileSets(paths, committed)
	line := commitAuditLine{At: time.Now().Format(time.RFC3339), Event: "commit", By: by, Mode: "removed",
		Only: paths, Excluded: excluded, GateRC: gtRC, Head: head, Commit: head, AuditPath: auditPath}
	if rc != exitOK {
		line.Event, line.Note = "commit_rejected", "提交被拦（rc="+fmt.Sprintf("%d", rc)+"）"
	}
	if aerr := appendCommitAudit(auditPath, line); aerr != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 审计落不下（%s）：%v —— 判决照旧（本行只记账）\n", progName, auditPath, aerr)
	}
	if rc == exitOK {
		writeCommitReceipt(message, head, "removed", paths, by, stderr) // 「轮 = 一笔提交」：缺口 `GAP-20261005-16`
	}
	fmt.Fprintf(stderr, "%s: %s\n", progName, commitExcludedNote(excluded))
	for _, e := range excluded {
		fmt.Fprintf(stderr, "  · %s\n", e)
	}
	if rc != exitOK {
		fmt.Fprintf(stderr, "%s: **提交未落地（rc=%d）** ⇒ 真索引与工作树均未动（本模式只动临时索引）\n", progName, rc)
		return rc
	}
	if !same {
		inv.setErr("usage", "committed_mismatch", "落库的件集与点名清单不一致")
		fmt.Fprintf(stderr, "%s: **落库的件集与点名清单不一致 ⇒ 判红**（退码 2）\n", progName)
		if len(extra) > 0 {
			fmt.Fprintf(stderr, "  多出来的：%s\n", strings.Join(extra, " · "))
		}
		if len(missingF) > 0 {
			fmt.Fprintf(stderr, "  少了的  ：%s\n", strings.Join(missingF, " · "))
		}
		fmt.Fprintf(stderr, "  回滚：`git reset --soft HEAD~1`（D2 可逆 · 提交内容面）\n")
		return exitUsage
	}
	short := []string{}
	for _, k := range excluded {
		if !containsLine(postStaged, k) {
			short = append(short, k)
		}
	}
	if len(short) > 0 {
		inv.setErr("failed", "others_staged_lost", "别人的暂存件少了")
		fmt.Fprintf(stderr, "%s: **别人的暂存件少了一枚**（本该只许多不许少）⇒ 判红：%s\n", progName, strings.Join(short, " · "))
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 对拍 ✓ ① 撤跟踪的 %d 件与点名清单逐件相同 ② 别人的暂存 %d 件一件没少 ③ 头 %s · 门禁 rc=%d\n",
		progName, len(committed), len(excluded), head, gtRC)
	for _, f := range committed {
		fmt.Fprintln(stdout, f)
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		removedRow["head"], removedRow["staged"] = head, strings.Join(postStaged, ",")
		removedRow["gate_rc"], removedRow["commit"] = fmt.Sprintf("%d", gtRC), head
		removedRow["message_path"], removedRow["log_dir"] = msgPath, logDir
		return selectJSON(stdout, stderr, inv, inv.path, repoCommitFields, removedRow)
	}
	return exitOK
}

// removedIsDeletion —— path 在当前**真索引**里是不是「删除态」（`git diff --cached` 报 D）。
func removedIsDeletion(root, path string) bool {
	out, err := gitRun(root, "-c", "core.quotepath=false", "diff", "--cached", "--name-status", "--", path)
	if err != nil {
		return false
	}
	for _, ln := range splitLines(out) {
		f := strings.SplitN(ln, "\t", 2)
		if len(f) == 2 && strings.HasPrefix(f[0], "D") && f[1] == path {
			return true
		}
	}
	return false
}

// gitRunIndex —— 跑一条 git 命令，但**索引面指到 indexFile**（`GIT_INDEX_FILE`）⇒ 只动临时索引、不碰真索引。
func gitRunIndex(root, indexFile string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotepath=false"}, args...)...)
	cmd.Env = append(stripGitIndexFace(os.Environ()), "GIT_INDEX_FILE="+indexFile)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s：%v（%s）", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// materializeTempIndex —— 物化**临时索引**（`git checkout-index` 走 `GIT_INDEX_FILE`）⇒ 闸取本次要落的内容面。
// 与 `materializeIndex` 同形（补 `.git`/`bin` 两枚链接的理由逐字同上），差别只在索引面指到临时索引。
func materializeTempIndex(root, indexFile string) (string, error) {
	dir, err := os.MkdirTemp(filepath.Dir(root), ".zerg-gate-idx-*")
	if err != nil {
		return "", err
	}
	prefix := dir + string(os.PathSeparator)
	if _, err := gitRunIndex(root, indexFile, "checkout-index", "-a", "-f", "--prefix="+prefix); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(dir, ".git")); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if st, err := os.Stat(filepath.Join(root, "bin")); err == nil && st.IsDir() {
		if err := os.Symlink(filepath.Join(root, "bin"), filepath.Join(dir, "bin")); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
}

// runGitCommitIndex —— 用**临时索引**提交（`GIT_INDEX_FILE`）⇒ 真索引一字不动（**不加 `--no-verify`**）。
func runGitCommitIndex(root, indexFile, msgPath string, stdout, stderr io.Writer) int {
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "commit", "-F", msgPath)
	cmd.Env = append(stripGitIndexFace(os.Environ()), "GIT_INDEX_FILE="+indexFile)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "%s: `git commit`（临时索引）起不来：%v\n", progName, err)
		return exitFail
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
// `matRoot` = 物化索引树（缺口 `GAP-20260927-79`）：闸取**索引面**而非工作树。
func runFastGateCapture(root, matRoot, logDir string) (int, string, error) {
	script := filepath.Join(root, "scripts/gates/precommit-gates.sh")
	if _, err := os.Stat(script); err != nil {
		return exitBlocked, "", fmt.Errorf("门禁最小入口不在（%s）", script)
	}
	cmd := armChildGroup(exec.Command("bash", script, "--fast", "--outdir", logDir))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Env = gateEnv(matRoot)
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
	// ★ `-c core.quotepath=false` 必落**子命令之前**（`-C` 前后均可；放子命令之后：ls-files 的 `-c` 被当 `--cached` ⇒ rc=0 但 0 行）。
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotepath=false"}, args...)...)
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
	// ★ 同上：`-c core.quotepath=false` 补在 `args`（首项 `-C`）**之前** ⇒ 仍在子命令之前。
	cmd := exec.Command("git", append([]string{"-c", "core.quotepath=false"}, args...)...)
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

// ═══════════ 「轮 = 一笔提交」：提交链自动落一页回执（缺口 `GAP-20261005-16`）═══════════
//
// 病根：门⑬ 的 R5b 把「件级账最新主仓写轮 晚于 最新回执」判成 `rc=2`（不给结论 · BLOCKED），
// 而回执的一页此前**没有自动调用者**（`zerg dev receipt new` 只能手工敲）⇒ 轮级账一旦停更，
// 「最新写轮 ≤ 最新回执」这条不变式当场破，门⑬ 就锁在 BLOCKED。
//
// 本钩子把不变式补上：**一笔提交 ⇒ 自动落一页回执**（append-only · 一轮一页 · 不覆盖别人）。
// 页内容**全部取真源**：题取 `--message` 首行 · 头取提交后的 HEAD · 件清单取本命令的点名件；
// `evidence` 里落的是**本命令自己的真调用**（`zerg repo commit …`）—— 真命令面调用，不是伪串凑数
// （过门⑬ H1/R4：首词 `repo` 在命令树里可解析）。
//
// ★ 挡位（两选一 · 按证据定 = **不阻断**）：与同件 `commit_audit` 落不下时的口径逐字一致
// （「判决照旧 · 只出声」· 见 `auditCompensationLine` 那段），理由同：**升 fail-closed 会把
// 「状态盘写不进」变成「提交做不了」**，而门⑬（**不在快速档**）才是这一格的牙 —— 回执缺了，
// 门⑬ R5b 会在提交之后照实判 `rc=2`，不靠提交当时互相挡。
func writeCommitReceipt(title, head, mode string, files []string, by string, stderr io.Writer) {
	if strings.TrimSpace(head) == "" {
		return // 拿不到 HEAD ⇒ 这一笔没落地 ⇒ 不落页（不造无头的轮）
	}
	dir := receiptDir()
	if dir == "" {
		fmt.Fprintf(stderr, "%s: ⚠ 回执落点解析不出来 ⇒ 本笔未落回执（门⑬ R5b 将据此判不给结论）\n", progName)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 回执目录建不出来（%v）⇒ 本笔未落回执\n", progName, err)
		return
	}
	// 轮次 id / 落点：与 `zerg dev receipt new` 同形（`R-` + 时间戳）；同一秒内两笔 ⇒ 加后缀，
	// **绝不覆盖别人的页**（append-only · 一轮一页）。
	rid := "R-" + time.Now().Format("20060102T150405")
	path := filepath.Join(dir, rid+".json")
	for n := 2; ; n++ {
		if _, e := os.Stat(path); os.IsNotExist(e) {
			break
		}
		rid = "R-" + time.Now().Format("20060102T150405") + "-" + fmt.Sprintf("%d", n)
		path = filepath.Join(dir, rid+".json")
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "（无题）"
	}
	flag := "--file"
	switch mode {
	case "only":
		flag = "--only"
	case "removed":
		flag = "--removed"
	}
	rec := receiptRecord{
		RoundID:   rid,
		TraceID:   newTraceID(),
		What:      fmt.Sprintf("提交 %s（%s 模式）：%s", head, mode, t),
		Next:      "续做入口：zerg context ls --resume（本轮改过仓内件，下一轮读这一页接上）",
		Blockers:  []string{},
		Evidence:  []string{fmt.Sprintf("zerg repo commit %s %s --message %q --yes", flag, strings.Join(files, " "), t)},
		Commits:   []string{head},
		CreatedAt: time.Now().Format(time.RFC3339),
		Path:      path,
	}
	b, _ := json.MarshalIndent(rec, "", " ")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 回执落不下（%s）：%v —— 判决照旧（门⑬ R5b 将据此判不给结论）\n", progName, path, err)
		if cerr := appendAuditCompensation(by, path, err); cerr == nil {
			fmt.Fprintf(stderr, "%s: 已把「回执主落点失败」记一条补偿行到备用落点 %s\n", progName, fallbackAuditPath())
		}
		return
	}
	fmt.Fprintf(stderr, "%s: 已落一页回执 %s（轮级账 ⟷ 件级账 · 门⑬ R5b 覆盖自证）\n", progName, path)
}
