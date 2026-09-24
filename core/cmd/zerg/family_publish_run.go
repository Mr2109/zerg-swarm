package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// family_publish_run.go —— `zerg publish preflight|run`（缺口 `Q-223`：把发布流水线接进 CLI）。
//
// 判据（2026-09-25 现读 CLI 自证）：`zerg publish` 家族**只有** `publish tree has`（只读查树）✗；
// 与发布沾边者只有 `build release`（计划面开放 · 真跑拒执退 2）与 `build all --only cli`（自举档）
// ⇒ **出公开镜像 + 预检只能手跑脚本** ⇒ 本日因此手搓一路（猜路径/猜参数/编错件）✗，
// Mr2109 判定「频繁犯错 = 没以 CLI 为主要工作命令」。
//
// 口径逐条照 `family_build.go` 的 `cmdBuildPassthrough`（计划件 → 拒执/确认 → **原样转发脚本退码**）：
//   · `publish preflight <产物目录>` —— **只读**（不改任何状态）⇒ 不设 D3 确认；
//   · `publish run`                 —— 真跑只写**本地** `dist/<版本>/release`（不出网、不推送）⇒ 设 D3 三态确认。
// **「推远端」不在命令面**：不可逆动作按规矩等 Mr2109 发话后单独执行（不另开执行路径）。

// cmdPublishPreflight —— `zerg publish preflight <产物目录>`：跑公开面预检（只读）。
func cmdPublishPreflight(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺产物目录")
		fmt.Fprintf(stderr, "%s: `publish preflight` 要一个产物目录（`publish run` 出镜像的那个目录）\n", progName)
		fmt.Fprintf(stderr, "用法：%s publish preflight <产物目录>\n", progName)
		return exitUsage
	}
	dir := inv.args[0]
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动预检（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := "scripts/build/publish-preflight.sh"
	if _, err := os.Stat(filepath.Join(root, script)); err != nil {
		inv.setErr("blocked", "script_absent", "预检脚本不在")
		fmt.Fprintf(stderr, "%s: 预检脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, script)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 公开面预检（**只读** · 不改任何状态）—— bash %s %s\n", progName, script, dir)
	return runPublishChild(inv, root, script, []string{dir}, stdout, stderr)
}

// cmdPublishRun —— `zerg publish run [--dry-run | --confirm=<主机名> --yes]`：本地出公开镜像树。
func cmdPublishRun(inv *invocation, stdout, stderr io.Writer) int {
	const script = "scripts/build/publish-public.sh"
	const effect = "在**本地** `dist/<版本>/release` 生成公开镜像树（白名单 + 排除项 + 脱敏替换）—— **不出网、不推送**"
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s publish run\n", progName)
		fmt.Fprintf(stdout, "  危险档   : D3（发布档：会写产物目录；命中私有面门禁 ⇒ 整批自中止，一个字节都不推）\n")
		fmt.Fprintf(stdout, "  它会调   : bash %s\n", script)
		fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
		fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到\n")
		fmt.Fprintf(stdout, "  本版状态 : **已开放**（Mr2109 2026-09-25「脱敏推」授权：出镜像属本地动作；**推远端另行发话**）\n")
		fmt.Fprintf(stdout, "  下一步   : %s publish preflight <产物目录>（只读复核）\n", progName)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动发布（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, script)); err != nil {
		inv.setErr("blocked", "script_absent", "发布脚本不在")
		fmt.Fprintf(stderr, "%s: 发布脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, script)
		return exitBlocked
	}
	// D3 三态：--confirm 的值必须与主机名**逐字相同** + --yes 同时到（§4.1 K7 / §6.3 S5）。
	host := planHost()
	if !inv.confirmGiven {
		msg := "`publish run` 会写产物目录 ⇒ 按 D3 档：**缺确认 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看计划件：%s publish run --dry-run\n", progName)
		return exitUsage
	}
	if inv.confirm != host {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同** —— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := "`publish run` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	fmt.Fprintf(stderr, "%s: 发布档执行 —— bash %s\n", progName, script)
	return runPublishChild(inv, root, script, nil, stdout, stderr)
}

// runPublishChild 调脚本并把退码**原样转发**（与 gate 族/构建族同一条口径 · §4.3 U1）。
func runPublishChild(inv *invocation, root, script string, args []string, stdout, stderr io.Writer) int {
	cmd := exec.Command("bash", append([]string{filepath.Join(root, script)}, args...)...)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "%s: %s 退 %d（原样转发，不翻译）\n", progName, filepath.Base(script), ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "publish_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动 %s：%v\n", progName, script, err)
		return exitFail
	}
	return exitOK
}
