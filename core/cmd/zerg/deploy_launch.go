// deploy_launch.go —— 发布「意图件 + 回读」的 **CLI 正门**（`zerg deploy launch`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 4 · A13
//
//	（`deploy/scripts/deploy-launch.sh` · 125 行 · 现属面＝部署面/发布意图件+回读 · 归族 = 新族 `deploy`）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `deploy/scripts/deploy-launch.sh`
// （真跑 `ssh` 到节点跑发布器 · OS 原生/联网面 ⇒ 无法干净真 Go 化）；命令面**只做入口 ＋ 门禁 ＋ 转发**，
// ✗ 不重写脚本里的任何一段（否则就长出第二个真源 —— 与 `build_wall.go` 同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `deploy launch`、✗ 不改任何 `family_*.go`）。
// 登记那一条（`path: []string{"deploy","launch"}; run: cmdDeployLaunch`）要改 `main.go` ＝ **另一路独占**
// ⇒ 本件先落实现、接线延后；在此之前 `cmdDeployLaunch` 无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg deploy launch` 之后的**一切**（`--host`/`--file`/`--dry-run`/`--help`/
//	   未知参数）原样交给脚本，连退码也不改写（脚本的 0 / 2 / 3 / 4 / 5 逐档原样转出）。
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

// deployLaunchScriptRel —— 发布「意图件 + 回读」的**唯一实现**（真源）。本件一个字都不重写它：
// 旗标面（`--host`/`--file`/`--expect-sha256`/`--agent`/`--unit`/`--target`/`--expect-version`/
// `--probe-url`/`--launch-dir`/`--dry-run`/`--help`）、读数（意图件 / 回读摘要）、
// 退码（0 / 2 / 3 / 4 / 5）全在那边。
const deployLaunchScriptRel = "deploy/scripts/deploy-launch.sh"

// cmdDeployLaunch —— `zerg deploy launch [--host … --file … --expect-sha256 … | --help]`（A13 的 Go 正门 ·
// 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的退码原样转出 ——
//
//	0 意图件与回读件齐 · 2 用法错 · 3 意图件写失败 · 4 节点发布器非 0 · 5 回读失败。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论）。
func cmdDeployLaunch(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到发布封装脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, deployLaunchScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "deploy_launch_script_absent", "发布封装脚本不在")
		fmt.Fprintf(stderr, "%s: 发布封装脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, deployLaunchScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg deploy launch` 之后的**一切**原样交给脚本。
	// `deploy` 族占两个路径段（族 `deploy` ＋ 动作 `launch`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go`（`build` 族亦两段）同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, deployLaunchScriptRel)
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
			fmt.Fprintf(stderr, "%s: 发布封装脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "deploy_launch_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动发布封装脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// deployLaunchGateMain —— `cmdDeployLaunch` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg deploy launch` 之后的实参，转交 `cmdDeployLaunch`
// （唯一实现仍是 `cmdDeployLaunch` ＋ 底层脚本 `deploy/scripts/deploy-launch.sh` —— 本件 ✗ 不重写
// 任何一段、✗ 不改退码口径、✗ 不新增机器面；两态三面与 `cmdDeployLaunch` 逐字同）。
// ★ M2 纪律：本入口**暂不接线**（✗ 未在 `core/cmd/zerg/gate_inproc_table.go` 登记键 —— 表键另路一并接）。
func deployLaunchGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"deploy", "launch"},
		orig: append([]string{"deploy", "launch"}, argv...),
	}
	return cmdDeployLaunch(inv, stdout, stderr)
}
