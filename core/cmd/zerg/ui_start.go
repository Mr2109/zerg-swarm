// ui_start.go —— UI「脱离会话」启动口的 **CLI 正门**（`zerg ui start`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A6
//
//	（`scripts/build/start-zerg-ui.sh` · 81 行 · 现属面＝构建面/UI 启动 · 归族 = 现役 `ui` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/start-zerg-ui.sh`（真跑
// `os.setsid()` 脱离会话 ＋ 逐 PID 点名选进程 ＋ 日志/stdin 重定向 · OS 原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段（否则就长出第二个真源 ——
// 与脚本里「逐 PID 点名、禁子串/通配」同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `ui start`、✗ 不改任何 `family_*.go`、
//
//	✗ 不改任何 `add_step`）。登记那一条（`path: []string{"ui","start"}; run: cmdUIStart`）要改
//	`main.go` ＝ **另一路独占**（派单 ④）⇒ 本件先落实现、接线延后；在此之前 `cmdUIStart`
//	无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg ui start` 之后的**一切**原样交给脚本，连退码也不改写
//	   （0 / 1 逐档原样转出；脚本 `set -euo pipefail`，未成即 1）。底层逻辑只有一处 ⇒ 本件不许重写 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 ·
//	   与 `ui` 族同档）；**不拿脚本的 1 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// uiStartScriptRel —— UI 脱离会话启动的**唯一实现**（真源）。本件一个字都不重写它：
// 脱离会话（`os.setsid`）、逐 PID 点名（`zerg_ui_pids`）、令牌注入（`~/.zerg/token`）、
// 日志/stdin 重定向、退码（0 已成 / 1 未成）全在那边。
const uiStartScriptRel = "scripts/build/start-zerg-ui.sh"

// cmdUIStart —— `zerg ui start`（A6 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` 原样转出 ——
//
//	0 成功（UI 已起，或已在运行 ⇒ 幂等不重起）· 1 失败（UI 二进制不可执行 / 起来又退了）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `ui` 族同档）。
func cmdUIStart(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到 UI 启动脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, uiStartScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "ui_start_script_absent", "UI 启动脚本不在")
		fmt.Fprintf(stderr, "%s: UI 启动脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, uiStartScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg ui start` 之后的**一切**原样交给脚本。
	// `ui` 族占两个路径段（族 `ui` ＋ 动作 `start`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go`（`build` 族亦两段）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, uiStartScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build`/`ui` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: UI 启动脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "ui_start_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动 UI 启动脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// uiStartGateMain —— `cmdUIStart` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg ui start` 之后的实参，转交 `cmdUIStart`
// （唯一实现仍是 `cmdUIStart` ＋ 底层脚本 `scripts/build/start-zerg-ui.sh` —— 本件 ✗ 不重写
// 任何一段、✗ 不改退码口径、✗ 不新增机器面；两态三面与 `cmdUIStart` 逐字同）。
// ★ 单一真源：底层脚本恒为 A6 面 `scripts/build/start-zerg-ui.sh`（顶层 `start-zerg-ui.sh`＝A17
// 是**另一件**、待退 ⇒ ✗ 不在此双登）。
// ★ M2 纪律：本入口**暂不接线**（✗ 未在 `core/cmd/zerg/gate_inproc_table.go` 登记键 —— 表键另路一并接）。
func uiStartGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"ui", "start"},
		orig: append([]string{"ui", "start"}, argv...),
	}
	return cmdUIStart(inv, stdout, stderr)
}
