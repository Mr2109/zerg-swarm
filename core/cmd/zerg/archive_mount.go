// archive_mount.go —— 归档区挂/卸口的 **CLI 正门**（`zerg archive mount`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A11
//
//	（`scripts/svc/archive-mount.sh` · 208 行 · 现属面＝服务面/归档挂卸 · 归族 = 现役 `archive` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `scripts/svc/archive-mount.sh`（真跑 `hdiutil attach/detach`
// ＋现读挂载表 · OS 原生面 ⇒ 无法干净真 Go 化）；命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里任何一段
// （否则就长出第二个真源 —— 与脚本里「挂/摘只走这一次动作」同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `archive mount`、✗ 不改 `family_archive*.go`、
//
//	✗ 不改任何 `add_step`）。登记那一条（`path: []string{"archive","mount"}; passthrough: true;
//	run: cmdArchiveMount`）要改 `main.go` ＝ **另一路独占**（派单 ④）⇒ 本件先落实现、接线延后；
//	在此之前 `cmdArchiveMount` 无调用点（与同批 M2 落件同态）。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg archive mount` 之后的**一切**（`on` / `off` / `status` / `--help` / 未知参数）
//	   原样交给脚本，连退码也不改写（0 / 1 / 2 / 64 逐档原样转出）。底层逻辑只有一处 ⇒ 本件不许重写任何一段 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 ·
//	   与 `archive` 族同一档）；**不拿脚本的某个码冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// archiveMountScriptRel —— 归档挂/卸的**唯一实现**（真源）。本件一个字都不重写它：
// 三个动作（`on` / `off` / `status`）、幂等口径、两处读数交叉核对（挂载表 ⟷ `hdiutil info`）、
// 退码表（0 成功 · 1 失败 · 2 不给结论 · 64 用法错）全在那边。
const archiveMountScriptRel = "scripts/svc/archive-mount.sh"

// cmdArchiveMount —— `zerg archive mount [on|off|status]`（A11 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` / `2` / `64` 原样转出 ——
//
//	0 成功（`on`/`off` 到位 · `status` 报「已挂」）· 1 失败（动作没成 / 回读不等 · `status` 报「未挂」）
//	· 2 不给结论（前提不可判：dmg 不在 / 原路径不是符号链接 / 两处读数打架 / 挂载表读不到）· 64 用法错。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `archive` 族同档）。
func cmdArchiveMount(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到归档挂载脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, archiveMountScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "archive_mount_script_absent", "归档挂载脚本不在")
		fmt.Fprintf(stderr, "%s: 归档挂载脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, archiveMountScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg archive mount` 之后的**一切**原样交给脚本。
	// `archive` 族占两个路径段（族 `archive` ＋ 动作 `mount`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` 同一取法（`build` 族亦两段 ⇒ `inv.orig[len(inv.path):]`）。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, archiveMountScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `build`/`archive` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 归档挂载脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "archive_mount_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动归档挂载脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
