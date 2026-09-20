// gate.go —— 门禁直通四条（`zerg gate ls|run|show|self-test` · §6.3 S3）。
//
// **一条铁律：只转发，不翻译**（§6.3 S3 判据③ · §九 M11 `G1-a`）——
//
//	· 不另写一份步骤表（`--list` 就是步骤表的**唯一真源**，命令面只把它原样转出）；
//	· 不动脚本退码（脚本退多少，命令面就返多少：`return cmd.ProcessState.ExitCode()`；
//	  全文件**零分支**改写退码 —— 这就是「不翻译」的证明面）。
//
// 四条动作与脚本旗标的对应（一一对应，不加戏）：
//
//	zerg gate ls          → bash scripts/gates/precommit-gates.sh --list
//	zerg gate run  <原样>  → bash scripts/gates/precommit-gates.sh <原样>      （--scope/--fast/--outdir/… 逐字透传）
//	zerg gate show <步名>  → bash scripts/gates/precommit-gates.sh --emit-cmd <步名>
//	zerg gate self-test    → bash scripts/gates/precommit-gates.sh --self-test
//
// 为什么不做 `--json`：这一族的输出**就是**脚本的输出（逐行相同）；再包一层 JSON 等于在命令面里
// 另写一份步骤表 —— `G1-a` 明令禁止。要机器面就读脚本自己的 `--list` / `--emit-cmd`。
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
		args = append(args, tail...)
	case "show":
		if len(tail) == 0 {
			fmt.Fprintf(stderr, "%s: `gate show` 要给步名（例：zerg gate show 'gofmt -l core'）\n", progName)
			return exitUsage
		}
		args = append([]string{"--emit-cmd"}, tail...)
	case "self-test":
		args = []string{"--self-test"}
	default:
		fmt.Fprintf(stderr, "%s: 未知 `gate` 动作 %q\n", progName, action)
		fmt.Fprintf(stderr, "可用：ls · run · show · self-test\n")
		return exitUsage
	}

	cmd := exec.Command("bash", append([]string{script}, args...)...)
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
