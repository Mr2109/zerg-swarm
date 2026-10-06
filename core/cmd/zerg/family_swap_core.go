// family_swap_core.go —— 主控换件口的 **CLI 正门**（`zerg swap core`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A1（★事故面）
//
//	（`scripts/build/zerg-swap-core.sh` · 417 行 · 现属面＝构建面/换件面 · 归族 = 新族 `swap`）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/zerg-swap-core.sh`
// （2026-09-16 事故后固化 —— 三道门禁：构建只走 build-all.sh / 换前验签（adhoc 判红）/ 起不来即回滚且回滚件同样验签；
// 两道前置闸：工作树无半成品 / launchd 定义可用；真换生产 `bin/zerg-core` 并重启 launchd ⇒ OS 原生面
// ⇒ 无法干净真 Go 化）；命令面**只做入口 ＋ 转发**，✗ 不重写脚本里的任何一段
// （否则就长出第二个真源 —— 与 `build_wall.go` / `deploy_launch.go` 同一条纪律）。
//
// ★ 安全线（本单最重要 · 事故面）：本族动作真跑会动生产 `bin/` 与在跑主控 ⇒ 本件只做「入口 ＋ 转发」，
// 命令面**不加任何自动执行 / 不替脚本挡**；验收两态一律只用**不产生副作用**的档
// （`--help` / 未知参数 / `--self-test-rollback`（抛靶 /tmp 目标））—— ✗ 不真跑换件。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `deploy_launch.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg swap core` 之后的**一切**（`--help` / `--self-test-rollback` / 未知参数）
//	   原样交给脚本，连退码也不改写（脚本的 0 / 1 / 2 / 3 逐档原样转出）。
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

// swapCoreScriptRel —— 主控换件的**唯一实现**（真源）。本件一个字都不重写它：
// 三道门禁（构建/验签/回滚）＋ 两道前置闸（工作树半成品 / launchd 定义）、读数（换前/换后端点与身份）与
// 退码（0 换件成功 · 1 不成功（未动生产 / 回滚成功，靠 error.kind 行分辨）· 2 用法错 · 3 回滚也失败）全在那边。
const swapCoreScriptRel = "scripts/build/zerg-swap-core.sh"

// cmdSwapCore —— `zerg swap core [--help | --self-test-rollback]`（A1 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的退码原样转出 ——
//
//	0 换件成功 · 1 不成功（未动生产 / 回滚成功两支，靠 error.kind=ROLLED_BACK 行分辨）· 2 用法错 · 3 回滚也失败（需人工）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论）。
func cmdSwapCore(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到主控换件脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, swapCoreScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "swap_core_script_absent", "主控换件脚本不在")
		fmt.Fprintf(stderr, "%s: 主控换件脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, swapCoreScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg swap core` 之后的**一切**原样交给脚本。
	// `swap` 族占两个路径段（族 `swap` ＋ 动作 `core`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` / `deploy_launch.go`（同为两段族）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, swapCoreScriptRel)
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
			fmt.Fprintf(stderr, "%s: 主控换件脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "swap_core_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动主控换件脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
