// build_wall.go —— 茧壁构建口的 **CLI 正门**（`zerg build wall`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A4
//
//	（`scripts/build/build-wall.sh` · 102 行 · 现属面＝构建面/换件面 · 归族 = 现役 `build` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = **Go 真身** `scripts/build/cmd/zerg-build/main.go`
// （`build-wall.sh` 的 Go 化对等体 · 批6 试点 · 真跑 `cargo` 子进程 · OS 原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写真身里的任何一段（否则就长出第二个真源 —— 与 `build-wall.sh:25` 同纪律）。
//
// ★ 接线现状（2026-10-09 现读）：**已接线** —— `main.go:3455-3463` 命令表登记 `build wall`（`path`:
//
//	`[]string{"build","wall"}` · `passthrough: true` · `run: cmdBuildWallFamily`）· `ui_start.go:676` 的
//	`cmdBuildWallFamily ⇒ cmdBuildWall` ⇒ `cmdBuildWall` **有调用点**；旧「M2 纪律：本件不接线 /
//	无调用点」的陈述自登记起已不成立。本件现读委派 **Go 真身**（见下 `buildWallGoCmd`），✗ 不再委派壳脚本。
//
// 三条口径（照仓里既有的透传族写 —— `family_isolate_ro.go` / `gate` 族，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg build wall` 之后的**一切**（`--debug` / `--help` / 未知参数）原样交给
//	   真身，连退码也不改写（0 / 2 逐档原样转出）。底层逻辑只有一处 ⇒ 本件不许重写任何一段 ✗。
//	② **命令面自己只在「连真身都递不到」那一档退码**：解析不到仓根 / 真身（go 工具链 ＋ 真身源码）不在
//	   ⇒ `8`（不给结论 · 与 `build` 族同一档）；**不拿真身的 2 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：真身没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// 茧壁构建的**唯一实现（Go 真身）**这几格：本件一个字都不重写它 —— 旗标面（`--debug` / `--help`）、
// 读数（制品 path / size / sha256）、退码（0 / 2）全在那边。调用形态 = `(cd scripts/build && go run
// ./cmd/zerg-build)`（现编现跑 · 源码在仓 ⇒ 与刚产出的 `bin/*` 解耦）。
const (
	buildWallGoDir  = "scripts/build"                        // 真身模块目录（`go run` 的工作目录）
	buildWallGoPkg  = "./cmd/zerg-build"                     // 真身包路径（相对模块目录）
	buildWallGoMain = "scripts/build/cmd/zerg-build/main.go" // 真身的源码件（递达性判据）
	buildWallGoCmd  = "cd " + buildWallGoDir + " && go run " + buildWallGoPkg
)

// cmdBuildWall —— `zerg build wall [--debug|--help]`（A4 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照真身**）：真身的 `0` / `2` 原样转出 ——
//
//	0 成功 · 2 硬失败（仓不完整 / 无 cargo / 编不出来 / 编完没有制品 / 制品起不来 / 用法错）。
//
// 命令面自己只在「递不到真身」那一档退码：解析不到仓根 / 真身（go 工具链 ＋ 真身源码）不在 ⇒ `8`
// （不给结论 · 与 `build` 族同档）。
//
// ★ 真身前置闸按「**能力可用**」判、✗ 不按「**壳脚本在盘**」判（2026-10-09 现读 · 同族缺口 `GAP-20261008-12`）：
// 执行面早已是 Go 真身 `cd scripts/build && go run ./cmd/zerg-build` ⇒ 闸若仍盯已退役的壳脚本
// `build-wall.sh`，该壳一退、命令照跑而此闸却先假红（rc=8）。故闸看「go 工具链可用 ＋ 真身源码在盘」
// （与 `family_build.go` 的 `buildExecPrereq` 同一口径）。
func cmdBuildWall(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到茧壁构建真身（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := exec.LookPath("go"); err != nil {
		inv.setErr("blocked", "build_wall_go_absent", "Go 工具链不可用")
		fmt.Fprintf(stderr, "%s: Go 工具链不可用（go 不在 PATH ⇒ 跑不动 `go run ./cmd/zerg-build`）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, buildWallGoMain)); err != nil {
		inv.setErr("blocked", "build_wall_realbody_absent", "茧壁构建真身不在")
		fmt.Fprintf(stderr, "%s: 茧壁构建真身不在（%s）⇒ 不给结论（退码 8）\n", progName, buildWallGoMain)
		return exitBlocked
	}
	// 透传面：`zerg build wall` 之后的**一切**原样交给真身。
	// `build` 族占两个路径段（族 `build` ＋ 动作 `wall`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `family_gate_reap.go` 同一取法（`gate` 族亦两段 ⇒ `inv.orig[2:]`）。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, buildWallGoCmd)
	cmd := armChildGroup(exec.Command("go", append([]string{"run", buildWallGoPkg}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = filepath.Join(root, buildWallGoDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：真身的码原样返回（与 `gate`/`build` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 茧壁构建真身退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "build_wall_realbody_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动茧壁构建真身：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
