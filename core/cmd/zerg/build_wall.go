// build_wall.go —— 茧壁构建口的 **CLI 正门**（`zerg build wall`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A4
//
//	（`scripts/build/build-wall.sh` · 102 行 · 现属面＝构建面/换件面 · 归族 = 现役 `build` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/build/build-wall.sh`（真跑 `cargo` 子进程 ·
// OS 原生面 ⇒ 无法干净真 Go 化）；命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段
// （否则就长出第二个真源 —— 与 `build-wall.sh:25` 「签名只一处实现」同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `build wall`、✗ 不改 `family_build*.go`、
//
//	✗ 不改任何 `add_step`）。登记那一条（`path: []string{"build","wall"}; passthrough: true;
//	run: cmdBuildWall`）要改 `main.go` ＝ **另一路独占**（派单 ④）⇒ 本件先落实现、接线延后；
//	在此之前 `cmdBuildWall` 无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `family_isolate_ro.go` / `gate` 族，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg build wall` 之后的**一切**（`--debug` / `--help` / 未知参数）原样交给
//	   脚本，连退码也不改写（0 / 2 逐档原样转出）。底层逻辑只有一处 ⇒ 本件不许重写任何一段 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 ·
//	   与 `build` 族同一档）；**不拿脚本的 2 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// buildWallScriptRel —— 茧壁构建的**唯一实现**（真源）。本件一个字都不重写它：
// 旗标面（`--debug` / `--help`）、读数（制品 path / size / sha256）、退码（0 / 2）全在那边。
const buildWallScriptRel = "scripts/build/build-wall.sh"

// cmdBuildWall —— `zerg build wall [--debug|--help]`（A4 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `2` 原样转出 ——
//
//	0 成功 · 2 硬失败（仓不完整 / 无 cargo / 编不出来 / 编完没有制品 / 制品起不来 / 用法错）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `build` 族同档）。
func cmdBuildWall(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到茧壁构建脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, buildWallScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "build_wall_script_absent", "茧壁构建脚本不在")
		fmt.Fprintf(stderr, "%s: 茧壁构建脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, buildWallScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg build wall` 之后的**一切**原样交给脚本。
	// `build` 族占两个路径段（族 `build` ＋ 动作 `wall`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `family_gate_reap.go` 同一取法（`gate` 族亦两段 ⇒ `inv.orig[2:]`）。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, buildWallScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `gate`/`build` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 茧壁构建脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "build_wall_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动茧壁构建脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
