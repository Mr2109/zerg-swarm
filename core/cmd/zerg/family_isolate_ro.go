// family_isolate_ro.go —— 只读隔离的**顶层命令**（`zerg isolate-ro new|verify` · 根因 `GAP-20260926-20` P0 · 2026-09-26）。
//
// 病灶（事故形态 · 逐字留档）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，
// `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata` 之后 `rm -f <农场>/…/cli-matrix.json`
// —— 删掉的是**真仓的件**（链接把删除动作透传回去了）。真因不是手滑，是**没有「只读隔离」的正门**。
//
// 工具面已经有了（`scripts/dev/isolate-ro.sh`：硬拷贝 + manifest + 逐件 sha256 验回 + `--self-test`
// 17/17），但**只有脚本面、没有 CLI 正门** ⇒ 用法靠口口相传，「在别处试」就还有第二条路。
// 本件补的就是这条正门；`GAP-20260926-20` 的「无只读隔离命令面」这一格由此闭合。
//
// ★ 落**顶层**（父代理一拍 · 本件头一版曾落在 `zerg dev isolate`，已改）：两条理由 ——
//
//	· **与已登记名一致**：技能参考 `references/safe-isolation.md` 与缺口账写的就是门名
//	  `zerg isolate-ro new|verify`（一处改，另一处跟着对齐 —— 别两处都留）。
//	· **不撞 standing 判据**：`dev` 族被钉成「**只增改环、读环一条都不新增**」
//	  （`dev_proposal_test.go` 的 `TestDevFamilyIsACHangedOnlyRingNoNewReadCommands`
//	  + `stage_gate_test.go` 的「四道闸 · 跨家族复查」`G2` 的七条闭集）—— 这条属性是**有意的**
//	  （读事实走既有族），而只读隔离是**工具类**正门、不是 `dev` 族的读环 ⇒ 放顶层，两边都不动。
//
// 三条口径（**照仓里既有的透传族写，不自己创式样**）：
//
//	① **只转发、不翻译**（与 `gate` 族逐字同一口径 · §6.3 S3）：`zerg isolate-ro` 之后的一切
//	   ——动作、位置参数、旗标与它们的值——**原样**交给 `scripts/dev/isolate-ro.sh`，
//	   连退码也不改写（0 / 1 / 2 逐档原样转出）。
//	   ⇒ **底层逻辑只有一处**：本件**不许**重写拷贝/对拍/自检中的任何一段 ✗（否则就长出第二个真源）。
//	② **写面分家，名字自己说清**：`new` = **造副本**，一切写动作只落在 `${TMPDIR}` 或 `--dst` 那棵树里
//	   （**真仓只读**：脚本只 `rsync`/`tar` 读它，从不往真仓写、建链接、删）；`verify` = **只读**
//	   逐件 sha256 对拍副本与真仓 ⇒ 用来证明「这一轮验证一个字节都没改到真仓」。
//	   ⇒ 本条**不是危险档**（不碰主控、不碰 `bin/`、不 sudo、不联网、不起服务；与 `dev proposal new`
//	   同一档：写只落在仓外的临时/状态目录）。
//	③ **没有 `--json` 面**：底层脚本没有机器面（它给的是人读的读数与 rc）⇒ 本条不许自己拼一份包封
//	   （契约在命令面只有一处，每个被包的脚本各写一份就是把契约抄成 N 份）✗。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// isolateScriptRel —— 只读隔离的**唯一实现**（真源）。
// 本件一个字都不重写它：旗标面（`--dst` / `--include` / `--exclude` / `--all` / `--reject-symlinks` /
// `--no-manifest` / `--quiet` / `--max-examples` / `--self-test`）、读数、退码全在那边。
const isolateScriptRel = "scripts/dev/isolate-ro.sh"

// cmdIsolateRO —— `zerg isolate-ro <动作> [参数] [旗标]`（动作：new | verify；`--self-test` 自检）。
//
// 退码口径（**逐档照底层**）：脚本的 0 / 1 / 2 原样转出 ——
//
//	0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）
//
// 命令面自己只在「连脚本都递不到」那一档退码：解析不到仓根 / 脚本不在 / 起不动 `bash` ⇒ `8`
// （不给结论 · 与 `build` 族同一档；**不拿脚本的 2 冒充**，两件事不许并成一个形状）。
func cmdIsolateRO(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到隔离脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, isolateScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "isolate_script_absent", "隔离脚本不在")
		fmt.Fprintf(stderr, "%s: 隔离脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, isolateScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg isolate-ro` 之后的**一切**原样交给脚本（含动作与旗标及它们的值）。
	// 与 `cmdGate` 同一取法，**差一段**：`gate` 族占两个路径段 ⇒ 那边是 `inv.orig[2:]`；
	// 本条是**顶层命令**、只占一个路径段 ⇒ `inv.orig[1:]`（多切一段就把动作 `new`/`verify` 吃掉）。
	// 位置参数的次序、`--include a --include b` 的重复、`--dst <目录>` 的值都**不进解析器**，逐字过去。
	tail := []string{}
	if len(inv.orig) > 1 {
		tail = inv.orig[1:]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, isolateScriptRel)
	if len(tail) == 0 {
		fmt.Fprintf(stderr, "%s: 动作两枚 —— new <真仓> [--dst <目录>] 造副本（写只落在副本里）· "+
			"verify <副本> <真仓> 逐件 sha256 验回；另有 `--self-test` 自检\n", progName)
	}
	cmd := exec.Command("bash", append([]string{script}, tail...)...)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `gate`/`build` 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 隔离脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "isolate_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动隔离脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
