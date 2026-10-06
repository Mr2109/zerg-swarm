// swap_ui.go —— 桌面 UI 换件口（`zerg-swap-ui.sh`）的 **CLI 正门**（`zerg swap ui`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A3
//
//	（`scripts/build/zerg-swap-ui.sh` · 204 行 · 现属面＝构建面/换件面（只报告档）· 归族 = 新族 `swap`）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/zerg-swap-ui.sh`
// （真跑 `git status` / `codesign` / 进程 inode 对拍 · OS 原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段
// （否则就长出第二个真源 —— 与 `build_wall.go` / `swap_agent.go` 同一条纪律）。
//
// ★ 安全线（换件面 · 与 A1/A2 同）：本件底层是**只报告档**（真换件档未开放 ⇒ `--swap` 一律 rc=2 且零副作用），
// 但归面仍是换件面 ⇒ 验收两态一律只用**不产生副作用**的档（`--help` / 未知参数）—— ✗ 不真跑换件。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `swap_agent.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg swap ui` 之后的**一切**（`--check` / `--help` / `--swap` / 未知参数）
//	   原样交给脚本，连退码也不改写（脚本的 0 / 1 / 2 逐档原样转出）。
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

// swapUIScriptRel —— 桌面 UI 换件受控流程的**唯一实现**（真源）。本件一个字都不重写它：
// 旗标面（`--check` / `--help` / `--swap`）、六闸判决（闸A/闸B/闸①②③④ · 含 UI 特有的
// 「进程活着 ∧ 跑的件 == 盘上件」自证）、读数、退码（0 / 1 / 2）全在那边。
const swapUIScriptRel = "scripts/build/zerg-swap-ui.sh"

// cmdSwapUI —— `zerg swap ui [--check|--help]`（A3 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` / `2` 原样转出 ——
//
//	0 六闸全通过（只读判定）· 1 有闸判 FAIL · 2 用法错 / 不给结论（有 BLOCKED）/ 真换件档未开放。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `swap` 族同档）。
func cmdSwapUI(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到 UI 换件脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, swapUIScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "swap_ui_script_absent", "UI 换件脚本不在")
		fmt.Fprintf(stderr, "%s: UI 换件脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, swapUIScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg swap ui` 之后的**一切**原样交给脚本。
	// `swap` 族占两个路径段（族 `swap` ＋ 动作 `ui`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` / `swap_agent.go`（两段族）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, swapUIScriptRel)
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
			fmt.Fprintf(stderr, "%s: UI 换件脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "swap_ui_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动 UI 换件脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
