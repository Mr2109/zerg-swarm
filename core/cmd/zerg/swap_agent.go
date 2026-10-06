// swap_agent.go —— 子端换件口（`zerg-swap-agent.sh`）的 **CLI 正门**（`zerg swap agent`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A2
//
//	（`scripts/build/zerg-swap-agent.sh` · 355 行 · 现属面＝构建面/换件面 · 归族 = 新族 `swap`）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/zerg-swap-agent.sh`
// （真跑 `git status` / `codesign` / `launchctl` / `curl` 探针 · OS 原生/联网面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段
// （否则就长出第二个真源 —— 与 `build_wall.go` / `deploy_launch.go` 同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `swap agent`、✗ 不改任何 `family_*.go`）。
// 登记那一条（`path: []string{"swap","agent"}; run: cmdSwapAgent`）要改 `main.go` ＝ **另一路独占**
// ⇒ 本件先落实现、接线延后；在此之前 `cmdSwapAgent` 无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `deploy_launch.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg swap agent` 之后的**一切**（`--check` / `--help` / `--self-test` /
//	   `--machine` / `--label` / 未知参数）原样交给脚本，连退码也不改写（脚本的 0 / 1 / 2 逐档原样转出）。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论）；
//	   **不拿脚本的 2 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// swapAgentScriptRel —— 子端换件受控流程的**唯一实现**（真源）。本件一个字都不重写它：
// 旗标面（`--check` / `--help` / `--self-test` / `--swap` / `--machine` / `--label`）、
// 六闸判决（闸A/闸B/闸①②③④）、读数、退码（0 / 1 / 2）全在那边。
const swapAgentScriptRel = "scripts/build/zerg-swap-agent.sh"

// cmdSwapAgent —— `zerg swap agent [--check|--help|--self-test|--machine …|--label …]`（A2 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` / `2` 原样转出 ——
//
//	0 六闸全通过（只读判定）· 1 有闸判 FAIL · 2 用法错 / 不给结论（有 BLOCKED）/ 真换件档未开放。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `build` 族同档）。
func cmdSwapAgent(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到子端换件脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, swapAgentScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "swap_agent_script_absent", "子端换件脚本不在")
		fmt.Fprintf(stderr, "%s: 子端换件脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, swapAgentScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg swap agent` 之后的**一切**原样交给脚本。
	// `swap` 族占两个路径段（族 `swap` ＋ 动作 `agent`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` / `deploy_launch.go`（两段族）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, swapAgentScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build_wall.go` 同一条口径 · 不吞不翻）。
			fmt.Fprintf(stderr, "%s: 子端换件脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "swap_agent_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动子端换件脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
