// eval_run_all.go —— 评测批跑口的 **CLI 正门**（`zerg eval run-all`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A7
//
//	（`scripts/build/run_all.sh` · 49 行 · 现属面＝构建面/评测批跑 · 归族 = 现役 `eval` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/run_all.sh`（真起 `go test` 子进程、
// 真跑 `scripts/evals/*.py` 任务集、真写 `zerg-evals/` 存档 · 外部/进程原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段（否则就长出第二个真源 ——
// 与 `family_eval.go:21`「不搬实现 · 22 个脚本一个字节不改」同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `eval run-all`、✗ 不改 `family_eval*.go`、
//
//	✗ 不改任何 `add_step`）。登记那一条（`path: []string{"eval","run-all"}; passthrough: true;
//	run: cmdEvalRunAll`）要改 `main.go` ＝ **另一路独占**（派单 ④）⇒ 本件先落实现、接线延后；
//	在此之前 `cmdEvalRunAll` 无调用点（与同批 M2 落件同态 —— 见 `build_wall.go` / `archive_mount.go`）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `archive_mount.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg eval run-all` 之后的**一切**（`<模型>` / `[版本标签]` / `--help` / 未知参数）
//	   原样交给脚本，连退码也不改写（脚本退多少就返多少）。底层逻辑只有一处 ⇒ 本件不许重写任何一段 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 ·
//	   与 `eval` 族同一档）；**不拿脚本的某个码冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// evalRunAllScriptRel —— 评测批跑的**唯一实现**（真源）。本件一个字都不重写它：
// 用法（`<模型> [版本标签]`）、三层任务集（层1 Go 单测 / 层2 基础 / 层3 复合）、
// 存档落点（`zerg-evals/results/agent/runs/<模型>-<版本>/`）、退码全在那边。
const evalRunAllScriptRel = "scripts/build/run_all.sh"

// cmdEvalRunAll —— `zerg eval run-all [<模型> [版本标签]]`（A7 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的码原样转出 ——
//
//	0 成功（三层跑完并落档）· 非 0 脚本自己判定的失败（层1 `go test` 红等 · `set -e` 下如实退）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `eval` 族同档）。
func cmdEvalRunAll(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到评测批跑脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, evalRunAllScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "eval_run_all_script_absent", "评测批跑脚本不在")
		fmt.Fprintf(stderr, "%s: 评测批跑脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, evalRunAllScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg eval run-all` 之后的**一切**原样交给脚本。
	// `eval` 族占两个路径段（族 `eval` ＋ 动作 `run-all`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` / `archive_mount.go` 同一取法（两族亦两段）。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, evalRunAllScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build`/`archive`/`eval` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 评测批跑脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "eval_run_all_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动评测批跑脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
