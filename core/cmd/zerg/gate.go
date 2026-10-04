// gate.go —— 门禁直通四条（`zerg gate ls|run|show|self-test` · §6.3 S3）。
//
// **一条铁律：只转发，不翻译**（§6.3 S3 判据③ · §九 M11 `G1-a`）——
//
//	· 不另写一份步骤表（`--list` 就是步骤表的**唯一真源**，命令面只把它原样转出）；
//	· 不动脚本退码（脚本退多少、命令面就返多少 —— **正码**逐字透传）；
//	  **唯一例外 = 表外负码**：子进程被信号打死时 Go 的 `ExitCode()` 给 `-1`（它**不是退码**、
//	  是「没有退码」）⇒ 显式收回 `exitInterrupted`(130)，落点见本文件 `:198`（缺口 `GAP-20261002-18`）。
//
// 六条动作与脚本旗标的对应（一一对应，不加戏）；`check` 是第七条第**不走脚本**的（判据转发壳）：
//
//	zerg gate ls          → bash scripts/gates/precommit-gates.sh --list
//	zerg gate run  <原样>  → bash scripts/gates/precommit-gates.sh <原样>      （--scope/--fast/--outdir/… 逐字透传）
//	zerg gate show <步名>  → bash scripts/gates/precommit-gates.sh --emit-cmd <步名>
//	zerg gate find <片段>  → **不走脚本**：门件名 ⇒ 步名 的桥（缺口 `GAP-20260927-07` · 只读）
//	zerg gate self-test    → bash scripts/gates/precommit-gates.sh --self-test
//	zerg gate results      → **不走脚本**：读现成一趟的 results.tsv（缺口 `Q-111`/`B-8` · 只读）
//	zerg gate check <判据名> → **不走脚本**：判据名 ⇒ `scripts/gates/*.py|*.sh` 的**保守**解析 ＋ 逐字透传子件
//	                          （v2.5.14 · M1 首项 · 设计-判据 CLI 化-v1.7 §9.1 · 见 `family_gate_check.go`：
//	                          名录真源 = 现读 `scripts/gates/` 目录列举；不在册 ⇒ 拒并给可用名；件自身崩 ⇒ `8`
//	                          ＋ `detail=judge_broken` ＋ `raw_rc` —— **零新占号**）
//	                          ★ 写工具（件头 `#zerg-class: write D2|D3`）**经壳强制危险档**：exec 前闸 ·
//	                          缺 `--yes` ⇒ 2 `yes_required`（非交互 fail-closed）· `--dry-run` 只出计划件 ⇒ 0 ·
//	                          过闸后壳级确认旗标不落给子件（见 `family_gate_check.go` 的写工具闸）。
//
// 六条**例外**（都只在命令面自己的旗标上生效，且都**不改脚本退码**）：
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
//	· `gate run --step <步名> --show-log` —— 单步档**步内读数**的手敲正门（`family_gate_run_step.go`
//	  的 `gateRunStepShowLog` · 缺口 `GAP-20260928-28`）：把那一步自己的日志正文**原样**打到 stdout
//	  （真源仍是脚本的 `results.tsv` 第 5 列）；缺 `--step` 时同 `--json` 退 2。
//
//	· ★ 2026-10-04（T-PR-03·04 · 设计-门禁部分趟-v0.7 §三 · 落点 `gate_run_partial.go`）：
//	  `gate run --since [<全量趟目录>]` = **部分趟**的**唯一显式入口**（部分趟必须显式 · 默认全量）。
//	  选步/改动件集/判定链全部在该件；它是命令面自己的旗标，**不改任何脚本退码**。三处转全量
//	  （undeclared → 五真源 → `|R|/N>=0.60`）命中即**转全量**（此时去掉 `--since` 按原全量路径走）。
//	  **缺 `--since` 时本支一个字节都不动**（默认全量的既有形态照旧）。⚠ 退码 3 的**登记**与
//	  freshness 随件属待拍（表乙 T-PR-15/T-PR-20），不在本处。
//
// 为什么**除这四处**不做 `--json`：其余三条的输出**就是**脚本的输出（逐行相同）；再包一层 JSON
// 等于在命令面里另写一份步骤表 —— `G1-a` 明令禁止。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gateScriptPath —— 自举件的路径**是条款的一部分**（§九 M11 `G1-e`）：搬家/改名 = 破坏自举，
// 必须单独一次提交。故这里**只拼这一段常量**，别处不许再拼。
const gateScriptRel = "scripts/gates/precommit-gates.sh"

// gateLSRejectFlags —— 缺口 `GAP-20260927-53`（2026-10-04 · P1 仍缺 · 边界件本片收）：
// `gate ls` 的**用法面**逐字是 `zerg gate ls`（`main.go` 登记 · **无旗标**），而脚本侧
// `--list` 出口**不读任何筛选旗标** ⇒ 若命令面把尾参原样丢掉，`zerg gate ls --scope go`
// 这类**筛选意图会被静默吞**（用户拿到全表还以为筛过）。
//
// 本文件开头那条铁律「**只转发、不翻译**」明令命令面**不许**自己实现筛选（= 另写一份
// 步骤表）⇒ 唯一 fail-closed 的处置 = **有尾参即拒并给用法**（退用法码 `exitUsage`、
// **不 exec 脚本**），与 `gate show` 缺步名那一支同规。无尾参 ⇒ 放行（原路径一字不动）。
func gateLSRejectFlags(tail []string, stderr io.Writer) int {
	if len(tail) == 0 {
		return exitOK
	}
	fmt.Fprintf(stderr, "%s: `gate ls` 不接受旗标（用法逐字：`zerg gate ls`）—— 收到 %q\n", progName, tail)
	fmt.Fprintf(stderr, "要筛步骤请用：`zerg gate find <片段>`（门件名 ⇒ 步名）或 `zerg gate run --step <步名>`。\n")
	return exitUsage
}

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
	// ★ 门新鲜度闸（缺口 `GAP-20260928-194` · 设计稿 §2.2 A2/A5）：`gate run` 那一趟要把**当时**的
	//   工作树指纹与 HEAD 记进那一趟的产物（`freshness.json`）。指纹在 **exec 之前**取（= 门看到的那棵树），
	//   落笔在跑完之后（写不下去**不改**本命令的任何退码 —— 只转发不翻译不下岗）。
	freshFP, freshHead := "", ""
	runOutdirArg := ""
	var runSince time.Time
	// ★ 缺口 `GAP-20261005-46`：默认面零命中的**追加指路**开关（命中 / 非 show 档 ⇒ 空 ⇒ 零副作用）。
	showZeroHit := false
	showZeroName := ""
	var showZeroDecls []gateStepDecl
	switch action {
	case "ls":
		if inv.jsonGiven {
			return gateLsJSON(inv, stdout, stderr, root, script)
		}
		// ★ 缺口 `GAP-20260927-53`（2026-10-04）：`gate ls` 的用法面无旗标，而 `--list` 出口不读筛选旗标
		//   ⇒ 有尾参即 **fail-closed 拒执**（不 exec）。无尾参 ⇒ 原路径一字不动。判据与理由见 `gateLSRejectFlags`。
		if rc := gateLSRejectFlags(tail, stderr); rc != exitOK {
			return rc
		}
		args = []string{"--list"}
	case "run":
		// ★ 2026-10-04（T-PR-03·04 · 设计-门禁部分趟-v0.7 §三）：`--since [<全量趟目录>]` = **部分趟**的
		//   唯一显式入口（`gate_run_partial.go`）。判在**一切下游拦截之前**（叠并拒绝的 rc=2 归它）。
		//   handled=false ⇒「转全量」：去掉 `--since` 后按**原全量路径**继续（下面那些拦截与透传一字不改）；
		//   handled=true  ⇒ 已就地处置（部分趟台账 + 退码，或用法/基线/对齐的 rc=2）。
		//   缺 `--since` 时本支一字不动 —— 默认全量的既有形态逐字照旧。
		if gatePartialSinceGiven(tail) {
			if rc, handled := gateRunPartial(inv, stdout, stderr, root, script, tail); handled {
				return rc
			}
			tail = gatePartialStripSince(tail)
			// ★ D8（T-PR-13）：转全量那一路也要把 `partial.json`（含五真源 `triggers`）落在
			//   **同一趟目录** —— `gateRunPartial` 已把该目录定好（`gatePartialTakeFullOutdir`）；
			//   此处**仅在未给 `--outdir`** 时补进 tail（给了就以用户那个为准，不重复注入）。
			if d := gatePartialTakeFullOutdir(); d != "" && gateRunOutdirArg(tail) == "" {
				tail = append(tail, "--outdir", d)
			}
		}
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
		// ★ 2026-09-28（缺口 `GAP-20260928-28`）：`--show-log` = 单步档**步内读数**的可手敲正门。
		//   病：`--step <步名> --only-step` 的透传面只回「状态计数 + 软门禁末段」（脚本的 `report()`
		//   只对 FAIL/BLOCKED/REPORT 三档打日志尾巴，**PASS 档只打一行状态**），那一步自己的逐格读数
		//   **只在** `<日志目录>/NN-<步名>.log` ⇒ 「拿门跑当活值对拍器」这条正门是断的。
		//   口径与 `--json` 同：仍复用脚本自己落的 `results.tsv`（第 5 列 = 日志件路径）读回，
		//   **不另抄步骤表、不改任何脚本退码**；只把读回的那一步正文**原样**打到 stdout。
		//   与 `--verify-live`/`--json` 同规：**缺 `--step` 时一个字节都不动**（仍原样走自举件）；
		//   同给 `--json` 时以机器面为先（上一条支已截获）。
		if gateRunWantsShowLog(tail) {
			return gateRunStepShowLog(inv, stdout, stderr, root, script, tail)
		}
		// ★ 2026-09-28（缺口 `GAP-20260928-146`）：旗标校验**前移** —— 自举件的 arg 循环在
		//   `main()` 很后面 ⇒ 未知参数要等「合成自检 + 软门禁」跑完才判到（实测 stdout 先落 22 行）。
		//   这里在 exec **之前**逐枚核一遍，未知 ⇒ 照自举件的逐字判词退 2、**不再 exec**。
		//   退码语义不变（未知参数改前 2、改后 2）；合法形态原样透传（本函数只读、不翻译）。
		if rc := gateRunPrecheckFlags(tail, stderr); rc != exitOK {
			return rc
		}
		// ★ 2026-09-28（缺口 `GAP-20260928-157`）：`--candidate` 的**值域**（闭集 = 真源
		//   `core/internal/contract/dev-candidate.json` 的 `candidate_id_pattern`）也前移 —— 过去是自举件
		//   `main()` 判的，且它走 `return 2` ⇒ 脚本还会往下跑完**尾部软门禁**（实测 stdout 22 行先落地）。
		//   这里在 exec **之前**按真源判（真源只读自 JSON、复用命令面既有解析，不复制第二份）；退码语义不变
		//   （坏值改前 2、改后 2）。偏序照自举件：这一条在自举件里判在 `--scope` 的 `build_steps` 之前。
		//   取不到真源 ⇒ 放行（行为与改前逐字相同）。
		if rc := gateRunPrecheckCandidateValue(tail, stderr); rc != exitOK {
			return rc
		}
		// ★ 2026-09-28（缺口 `GAP-20260928-153`）：**值面**也前移 —— `--scope <坏值>` 的闭集
		//   过去是自举件 `build_steps()` 判的（在合成自检＋软门禁**之后**）⇒ 实测 stdout 175 行先落地。
		//   这里在 exec **之前**按自举件自己的闭集判（闭集只读自它、不复制第二份）；退码语义不变
		//   （坏值改前 2、改后 2），只不再白跑。取不到闭集 ⇒ 放行（行为与改前逐字相同）。
		if rc := gateRunPrecheckScopeValues(script, tail, stderr); rc != exitOK {
			return rc
		}
		args = append(args, tail...)
		// 指纹**在执行前**取：它就是这一趟门跑**看到**的那棵树。
		runOutdirArg, runSince = gateRunOutdirArg(tail), time.Now()
		freshFP, freshHead, _ = worktreeFingerprint(root)
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
		// ★ 缺口 `GAP-20261005-46`（2026-10-05）：默认面**零命中不再黑箱**。脚本 `--emit-cmd`
		//   零命中时只印一行 `✗ 步骤名里没有匹配 … 的`（无候选、无指路），而**同一条命令**的
		//   `--json` 面与 `gate explain` 都给了「最像的合法输入 + 候选」⇒ 默认面是这条命令唯一的
		//   黑箱出口（`GAP-20260927-280` 只治了「歧义时列候选」那一半）。
		//   处置（照 `gate explain` 方言 · 不另写一份口径）：命令面**执行前先判**这一步名会不会
		//   零命中（逐字与子串都不中 —— 与脚本 `--emit-cmd` 的匹配口径同源）；会 ⇒ 脚本那一行
		//   仍照跑照印（默认面**一字不改**），跑完**追加**候选与指路。命中（逐字或子串）⇒ 这一路
		//   一个字节都不动，仍是 `--emit-cmd` 的直取口（`bash -c "$(…)"` 的既有用法不破）。
		if len(tail) > 0 && strings.TrimSpace(tail[0]) != "" {
			if decls, ok := gateShowReadDecls(script); ok && gateShowZeroHit(decls, tail[0]) {
				showZeroHit, showZeroName, showZeroDecls = true, tail[0], decls
			}
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
	case "check":
		// ★ v2.5.14 · M1 首项（设计-判据 CLI 化-v1.7 §9.1）：**判据转发壳**。
		// 判据名 ⇒ 件 的**保守**解析（名录真源 = 现读 `scripts/gates/` 目录列举）＋ 逐字透传；
		// 件自身崩 ⇒ `8` ＋ `detail=judge_broken` ＋ `raw_rc`（**不占新号** · 见 `family_gate_check.go`）。
		// 与既有六条同规：只转发、不翻译；除本动作**不改任何既有动作**的判定与退码。
		return gateCheck(inv, stdout, stderr, root)
	default:
		fmt.Fprintf(stderr, "%s: 未知 `gate` 动作 %q\n", progName, action)
		fmt.Fprintf(stderr, "可用：ls · run · show · find · self-test · results · check\n")
		return exitUsage
	}

	cmd := armChildGroup(exec.Command("bash", append([]string{script}, args...)...))
	// 登记在跑的这一步：人打断时 `--cancel-on-interrupt` 才有东西可取消（§九 M8 · `P-033`）。
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// 门新鲜度闸：跑完了（哪怕红/不给结论）也照样把**当时**的指纹落进那一趟的产物。
			recordRunFreshness(root, runOutdirArg, runSince, freshFP, freshHead, stderr)
			// ★ 缺口 `GAP-20261005-46`：零命中 ⇒ 脚本那一行已印，这里**追加**候选与指路。
			if showZeroHit {
				gateShowZeroHitHint(stderr, showZeroDecls, showZeroName)
			}
			// ★ 只转发、不翻译：脚本的**正码**原样返回（0/1/2/8… 逐字透传，不改写）。
			code := ee.ExitCode()
			// ★★ 缺口 `GAP-20261002-18`（2026-10-02）：**被信号打死**的子进程在 Go 里
			//   `ExitCode()` 返回 **-1**（`os/exec_posix.go` 的语义：ProcessState 只带信号号、
			//   没有退出码）⇒ 原样 `os.Exit(-1)` 会被掩码成**表外码 255**（主表根本没有 255）。
			//   故**只对负码**显式收回 `exitInterrupted`(130)：`-1` 本就**不是退码**、是
			//   「没有退码」⇒ 这一格不是把码翻译成别的码，是把表外码收回表内（与 `main.go:74`
			//   的 `os.Exit(exitInterrupted)` 同号同义 —— SIGINT/SIGTERM/SIGHUP 同一路）。
			if code < 0 {
				return exitInterrupted
			}
			return code
		}
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	recordRunFreshness(root, runOutdirArg, runSince, freshFP, freshHead, stderr)
	if showZeroHit {
		gateShowZeroHitHint(stderr, showZeroDecls, showZeroName)
	}
	return exitOK
}

// recordRunFreshness —— 把这一趟的指纹随件落进那一趟的目录（**只写这一枚**；写不下去只出声、
// **一字不改**本命令的退码 —— `gate.go` 开头那条「只转发、不翻译」不下岗）。
//
// 认「这一趟」的办法两条：`--outdir` 显式给的 ⇒ 就用它；否则取 `<TMPDIR>` 下带 `results.tsv`
// 且 mtime 落在本次执行窗口内的最新一份（防把**更旧**的一趟当这一趟 —— 同 `gateResultsFindLast` 那条口径）。
func recordRunFreshness(root, outdirArg string, since time.Time, fp, head string, stderr io.Writer) {
	if fp == "" {
		return
	}
	dir := outdirArg
	if dir == "" {
		dir = gateRunNewestOutdirSince(since)
	}
	if dir == "" {
		if outdirArg == "" {
			return // `--list`/`--emit-cmd` 一类**不产生趟目录**的档：无事可做，静默
		}
		fmt.Fprintf(stderr, "%s: ⚠ 门新鲜度闸：`--outdir %s` 里没有 %s ⇒ 这一趟没记指纹（退码不变）\n",
			progName, dir, gateResultsTSV)
		return
	}
	// ★ D10（T-PR-15·已拍 α）：写手按「目录里有没有 `results-partial.tsv`」**分派** ——
	//   全量走原 `writeGateFreshness`（**一字不动**）；部分趟走新 `writeGateFreshnessPartial`
	//   （含 `based_on` 基线四格）。α 随件落趟目录、非仓内 ⇒ 天然不进 `worktreeFingerprint`。
	if _, perr := os.Stat(filepath.Join(dir, gateResultsPartialTSV)); perr == nil {
		if err := writeGateFreshnessPartial(dir, root, fp, head, gatePartialBasedOnSnapshot()); err != nil {
			fmt.Fprintf(stderr, "%s: ⚠ 门新鲜度闸：部分趟指纹随件落不下去（%v）⇒ 这一趟没记指纹（退码不变）\n", progName, err)
		}
		return
	}
	if err := writeGateFreshness(dir, root, fp, head); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 门新鲜度闸：指纹随件落不下去（%v）⇒ 这一趟没记指纹（退码不变）\n", progName, err)
	}
	// ★ D12（T-PR-17）：全量趟**全绿才**覆盖写 `last-green.json` —— **红不覆盖**；
	//   「全绿」取**本机制口径**（`FAIL==0 && BLOCKED==0 && REPORT==0` · 与 P2 一致）。
	if rows, _, rerr := gateResultsReadTSV(filepath.Join(dir, gateResultsTSV)); rerr == nil {
		counts := map[string]int{}
		for _, r := range rows {
			counts[r.Status]++
		}
		if gateRunAllGreen(counts) {
			if werr := writeGateLastGreen(root, dir, head, fp, len(rows), 0); werr != nil {
				fmt.Fprintf(stderr, "%s: ⚠ 最近全绿件 last-green.json 落不下去（%v）—— 退码不变\n", progName, werr)
			}
		}
	}
}

// gateLsJSON —— [候选·未落地] `gate ls --json` 机器面：仍只跑脚本 `--list`（**不另写步骤表**），
// 把脚本自己那一段「步骤清单」三列（scope/mode/名称）重新序列化成 JSON（G1-a 只转发）。
func gateLsJSON(inv *invocation, stdout, stderr io.Writer, root, script string) int {
	c := exec.Command("bash", script, "--list")
	c.Dir = root
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		fmt.Fprintf(stderr, "%s: 跑不动 `--list`：%v\n", progName, err)
		return exitFail
	}
	type row struct {
		Scope string `json:"scope"`
		Mode  string `json:"mode"`
		Step  string `json:"step"`
	}
	rows := []row{}
	total := 0
	for _, ln := range strings.Split(buf.String(), "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "共") {
			fmt.Sscanf(t, "共 %d 步", &total)
			continue
		}
		if strings.HasPrefix(t, "──") {
			continue
		}
		f := strings.Fields(t)
		if len(f) < 3 {
			rows = append(rows, row{Step: t})
			continue
		}
		rows = append(rows, row{Scope: f[0], Mode: f[1], Step: strings.Join(f[2:], " ")})
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"total": total, "steps": rows}); err != nil {
		fmt.Fprintf(stderr, "%s: 出 JSON 失败：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// gateShowReadDecls —— 现读门禁脚本、解析出所有 `add_step` 声明（**不打印**「解析不确定」——
// 默认面是「只转发、不翻译」，脚本自己那一行才是它的输出；命令面只在零命中时**追加**指路）。
// 读不到 / 解析不出 ⇒ ok=false ⇒ 不拦，交原路（与命中一样一个字节都不动）。
func gateShowReadDecls(script string) ([]gateStepDecl, bool) {
	src, err := os.ReadFile(script)
	if err != nil {
		return nil, false
	}
	decls, _ := parseAddStepsStrict(string(src))
	return decls, true
}

// gateShowZeroHit —— 这一步名会不会在脚本 `--emit-cmd` 里**零命中**（逐字与子串都不中）。
//
// 与脚本 `--emit-cmd` 的匹配口径同源：先逐字（`STEP_NAME == 名`）、再子串（`STEP_NAME` 含名）；
// 两侧一律走 `normStepName`（与 `gate explain`/`gate show --json` **同一函数** —— 不另写第二份口径）。
// 只用来决定「要不要追加指路」；命中 ⇒ 返回 false，命令面**一个字节都不动**。
func gateShowZeroHit(decls []gateStepDecl, name string) bool {
	key := normStepName(name)
	for _, d := range decls {
		if normStepName(d.Name) == key {
			return false // 逐字命中
		}
	}
	for _, d := range decls {
		if key != "" && strings.Contains(normStepName(d.Name), key) {
			return false // 子串命中（脚本的 `*"${emit_cmd}"*` 回退）
		}
	}
	return true
}

// gateShowZeroHitHint —— 默认面零命中的**追加指路**（照 `gate explain`/`gate show --json` 方言：
// 「最像的合法输入 + 候选」来自**同一个** `nearestSteps`；再给一条去哪找全部步名的指路）。
// 只在脚本已印出它那一行**之后**调用 ⇒ 默认面原有输出**一字不改**，这里只是**补**（负控：把本
// 函数的调用去掉 ⇒ 输出即回到那个黑箱 —— 见 `TestGateShowZeroHitDefaultFaceGivesCandidates`）。
func gateShowZeroHitHint(stderr io.Writer, decls []gateStepDecl, name string) {
	fmt.Fprintf(stderr, "%s: ↑ 以上是脚本 `--emit-cmd` 自己的那一行；命令面照 `gate explain` 方言补指路（缺口 `GAP-20261005-46`）：\n", progName)
	if s := nearestSteps(decls, name, 1); len(s) > 0 {
		fmt.Fprintf(stderr, "最像的合法输入: %s\n", s[0])
	}
	for _, s := range nearestSteps(decls, name, 3) {
		fmt.Fprintf(stderr, "  候选：%s\n", s)
	}
	fmt.Fprintf(stderr, "要列全部步名 ⇒ `zerg gate ls`；已知门件名 ⇒ `zerg gate find <门件名>` 反查步名\n")
}
