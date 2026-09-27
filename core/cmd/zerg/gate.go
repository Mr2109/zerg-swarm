// gate.go —— 门禁直通四条（`zerg gate ls|run|show|self-test` · §6.3 S3）。
//
// **一条铁律：只转发，不翻译**（§6.3 S3 判据③ · §九 M11 `G1-a`）——
//
//	· 不另写一份步骤表（`--list` 就是步骤表的**唯一真源**，命令面只把它原样转出）；
//	· 不动脚本退码（脚本退多少，命令面就返多少：`return cmd.ProcessState.ExitCode()`；
//	  全文件**零分支**改写退码 —— 这就是「不翻译」的证明面）。
//
// 六条动作与脚本旗标的对应（一一对应，不加戏）：
//
//	zerg gate ls          → bash scripts/gates/precommit-gates.sh --list
//	zerg gate run  <原样>  → bash scripts/gates/precommit-gates.sh <原样>      （--scope/--fast/--outdir/… 逐字透传）
//	zerg gate show <步名>  → bash scripts/gates/precommit-gates.sh --emit-cmd <步名>
//	zerg gate find <片段>  → **不走脚本**：门件名 ⇒ 步名 的桥（缺口 `GAP-20260927-07` · 只读）
//	zerg gate self-test    → bash scripts/gates/precommit-gates.sh --self-test
//	zerg gate results      → **不走脚本**：读现成一趟的 results.tsv（缺口 `Q-111`/`B-8` · 只读）
//
// 五条**例外**（都只在命令面自己的旗标上生效，且都**不改脚本退码**）：
//
//	· `gate run --step <步名> --json <字段>` —— 单步档的机器面（`family_gate_run_step.go`）；
//	· `gate show <步名> --json [<字段>]`   —— 四格机器面（`family_gate_show.go` · 缺口 `Q-061`/
//	  `B-3`：默认面**一个字节不动**，`--emit-cmd` 仍是命令串直取口 ⇒ `bash -c "$(…)"` 的既有用法不破）；
//	· `gate find <片段> [--json <字段>]`   —— 门件名 ⇒ 步名 的桥（`family_gate_find.go` · 缺口
//	  `GAP-20260927-07`：门族两条既有入口的键都是**步骤名**，而手上的名字是**门件名** ⇒ 没这一格
//	  就只能手搓 `grep add_step`；它**只读步骤声明**，不跑任何步骤、不改任何退码）；
//	· `gate results [--last|--dir <目录>] [--json <字段>]` —— 上一趟的四数（`family_gate_results.go`）；
//	· `gate run --step <步名> --verify-live` —— 单步**只读复核**的正门（`family_gate_run_step.go` 的
//	  `gateRunStepLiveVerify` · 2026-09-27）：`--verify-live` 是**步骤脚本自己的**旗标（门⑪ 的
//	  `check-cli-contract.py --verify-live` 逐格回放），交给自举件必被拒（现读 `✗ 未知参数` rc=2），
//	  直跑脚本又是 CLI 守卫拦下的手搓形态 ⇒ 命令面把它**逐字透传给那一步的命令串**（步名真源仍是
//	  `add_step` 行）。**缺 `--step` 时一个字节都不动**（仍原样走自举件、退码照旧）。
//
// 为什么**除这四处**不做 `--json`：其余三条的输出**就是**脚本的输出（逐行相同）；再包一层 JSON
// 等于在命令面里另写一份步骤表 —— `G1-a` 明令禁止。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// gateScriptPath —— 自举件的路径**是条款的一部分**（§九 M11 `G1-e`）：搬家/改名 = 破坏自举，
// 必须单独一次提交。故这里**只拼这一段常量**，别处不许再拼。
const gateScriptRel = "scripts/gates/precommit-gates.sh"

func cmdGate(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 门禁直通需要仓根（找 core/internal/version/version.go 失败）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitUsage
	}
	script := filepath.Join(root, gateScriptRel)
	if _, err := os.Stat(script); err != nil {
		fmt.Fprintf(stderr, "%s: 门禁最小入口不在（%s）—— 它是自举件，就地缺席即硬错\n", progName, gateScriptRel)
		return exitUsage
	}

	action := ""
	if len(inv.path) > 1 {
		action = inv.path[1]
	}
	// 透传面：把用户敲在 `zerg gate <动作>` 之后的**一切**原样交给脚本（含旗标与它们的值）。
	tail := []string{}
	if len(inv.orig) > 2 {
		tail = inv.orig[2:]
	}

	var args []string
	switch action {
	case "ls":
		args = []string{"--list"}
	case "run":
		// 单步档的**机器面**（`--json <字段>`）走命令面（`family_gate_run_step.go`）：
		// 执行照样交给脚本（步骤表的唯一真源），命令面只做「执行前判 + 契约形状的包封」。
		// 其余一律原样透传 —— 本条分支是 `gate.go` 开头那句「只转发、不翻译」的**唯一例外**，
		// 且它**不改任何脚本退码**（包封读的是脚本自己落的结果表）。
		// ★ 2026-09-27：`--step <步名> --verify-live`（单步**只读复核**的正门）也走命令面
		//   （`family_gate_run_step.go` 的 `gateRunStepLiveVerify`）：`--verify-live` 是**步骤脚本
		//   自己的**旗标，透传给自举件会被它当未知参数拒掉（现读 rc=2）⇒ 命令面把它逐字透传给
		//   **那一步的命令串**。同给 `--json` 时以只读复核为先（`--json` 会落到那件脚本自己的
		//   「本门无机器读面」判词上，退码照旧 2）。**既有形态一个字节不改**：缺 `--step` 时
		//   `--verify-live` 仍原样走自举件。
		if gateRunWantsLiveVerify(tail) && gateStepNameFrom(tail) != "" {
			return gateRunStepLiveVerify(inv, stdout, stderr, root, script, tail)
		}
		if inv.jsonGiven {
			return gateRunStepJSON(inv, stdout, stderr, root, script, tail)
		}
		args = append(args, tail...)
	case "show":
		if len(tail) == 0 {
			fmt.Fprintf(stderr, "%s: `gate show` 要给步名（例：zerg gate show 'gofmt -l core'）\n", progName)
			return exitUsage
		}
		// `--json` 是**命令面**的旗标（缺口 `Q-061`/`B-3` 的四格机器面 · `family_gate_show.go`）：
		// 给了它 ⇒ 出四格；**不给 ⇒ 这一路一个字节都没动**（仍是 `--emit-cmd` 的直取口，
		// `bash -c "$(…)"` 的既有用法不破 —— `Q-061` 可核条件 ② 的负控）。
		if inv.jsonGiven {
			return gateShowJSON(inv, stdout, stderr, root, script)
		}
		args = append([]string{"--emit-cmd"}, tail...)
	case "find":
		// 缺口 `GAP-20260927-07`：**门件名 ⇒ 步骤名** 的桥（`family_gate_find.go`）。
		// 与 `results` 同规：**只读**、不走脚本、不改任何脚本退码 —— 它是第四条命令面分支。
		// 病根：`gate show` / `gate explain` 的键都是**步骤名**，而手上的名字是**门件名**
		// （`scripts/gates/check-*.py`）⇒ 没有这一格就只能手搓 `grep add_step` ✗。
		return cmdGateFind(inv, stdout, stderr, root, script)
	case "self-test":
		args = []string{"--self-test"}
	case "results":
		// 缺口 `Q-111`/`B-8`：读**现成的**一趟门禁产物出四数（**只读** —— 不跑任何步骤、
		// 不写不删任何日志）。本动作**不走脚本**（脚本没有这个动作）⇒ 是第四条命令面分支，
		// 与 `run --step --json` / `show --json` 同规：只在命令面自己的旗标上生效。
		return gateResults(inv, stdout, stderr, root)
	default:
		fmt.Fprintf(stderr, "%s: 未知 `gate` 动作 %q\n", progName, action)
		fmt.Fprintf(stderr, "可用：ls · run · show · find · self-test · results\n")
		return exitUsage
	}

	cmd := exec.Command("bash", append([]string{script}, args...)...)
	// 登记在跑的这一步：人打断时 `--cancel-on-interrupt` 才有东西可取消（§九 M8 · `P-033`）。
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（连异常码也不改写）。
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}
