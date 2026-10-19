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
//	· `gate ls --fingerprint` —— **门禁指纹行**的只读呈现面（缺口 `GAP-20260927-306` · 2026-10-13）：
//	  单印「当前工作树指纹 · HEAD」（`gate_freshness.go` 的 `worktreeFingerprint` —— 与门跑那一趟
//	  记进 `freshness.json` 的口径**同源**，✗ 不另算一份）；只读、不 exec 脚本、不改任何退码。
//
// 为什么**除这四处**不做 `--json`：其余三条的输出**就是**脚本的输出（逐行相同）；再包一层 JSON
// 等于在命令面里另写一份步骤表 —— `G1-a` 明令禁止。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gateScriptPath —— 自举件的路径**是条款的一部分**（§九 M11 `G1-e`）：搬家/改名 = 破坏自举，
// 必须单独一次提交。故这里**只拼这一段常量**，别处不许再拼。
const gateScriptRel = pathPrecommitGates

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

// gateActionNeedsScript —— 本动作是否**要 exec** `scripts/gates/precommit-gates.sh`（= 是否受「最小入口」硬错约束）。
//
// 为什么要有这一枚：`gate` 族里两类动作**不 exec 脚本** —— 它们在 `switch action` 里就 `return` 了，
// 根本走不到下面那条 `exec.Command("bash", script, …)`：
//
//	· `check`   —— 判据转发壳（Go 半自足；判据名 ⇒ 件的解析见 `family_gate_check.go`）
//	· `results` —— 读现成产物出四数（**只读** · 不跑步骤 · 不写日志）
//
// 这两类在**自举件缺席**的树里（如公开面：`scripts/gates/**` 被有意排除）也必须能跑，故不过
// 「最小入口」前置。其余动作（`ls`/`run`/`show`/`find`/`self-test` 等真要脚本的）行为**一字不改**。
func gateActionNeedsScript(action string) bool {
	switch action {
	case "check", "results":
		return false
	}
	return true
}

// gateRootOverride —— 从 `gate run` 的 tail 里取 `--root <目录>` 的值（缺口 `GAP-20260928-124`）。
//
// 病灶：`--root` 在取值名字表里 ⇒ `gate run --root X` 逐字收下却**静默忽略**（rc=0、仍跑本仓脚本），
// 正门没有「对仓外副本跑某门」的出口。✗ 只看 `--` **之前**的形态 —— `--` 之后的 `--root` 是**单步只读
// 透传**旗标（归 `gateRunStepPassthroughFlags`），不在这里当仓根。取不到 ⇒ 空串（行为逐字不变）。
func gateRootOverride(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if a == "--root" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(a, "--root=") {
			return strings.TrimPrefix(a, "--root=")
		}
	}
	return ""
}

func cmdGate(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 门禁直通需要仓根（找 core/internal/version/version.go 失败）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitUsage
	}
	// ★ GAP-20260928-124（本单）：`--root <仓外副本根>` —— 正门加「对仓外副本跑某门」的出口。
	//   病灶：`--root` 在取值名字表里 ⇒ `gate run --root X` 逐字收下却**静默忽略**（rc=0、仍跑本仓脚本）。
	//   新行为：`run` 的 tail 里 `--root <目录>`（`--` **之前**的形态）⇒ 用它当仓根；该根下没有自举件
	//   ⇒ 硬错退出（✗ 不静默回落本仓 —— 「读不到不当没有」）。缺它 ⇒ 仓根、行为逐字不变。
	if len(inv.path) > 1 && inv.path[1] == "run" {
		if alt := gateRootOverride(inv.orig[2:]); alt != "" {
			if st, aerr := os.Stat(filepath.Join(alt, gateScriptRel)); aerr != nil || !st.Mode().IsRegular() {
				fmt.Fprintf(stderr, "%s: `--root %s` 指的仓外副本根下没有自举件（%s）⇒ 不给结论（退码 2）\n", progName, alt, gateScriptRel)
				return exitUsage
			}
			root = alt
		}
	}
	script := filepath.Join(root, gateScriptRel)

	// ★ 动作解析**提前**到「最小入口」检查之前：`check` / `results` 一类**不 exec 脚本**的动作
	//   （Go 半自足 · 见下方 `case "check"` 的转发壳）在自举件缺席的树里也必须能跑 —— 公开面
	//   把 `scripts/gates/**` 有意排除（排除表逐条登记），而公开仓 CI 正是用这些动作跑公开面判据。
	//   旧行为（本函数初版）：无条件先判自举件在场 ⇒ 连不碰脚本的动作也一并硬错（rc=2）。
	action := ""
	if len(inv.path) > 1 {
		action = inv.path[1]
	}

	if gateActionNeedsScript(action) {
		if _, err := os.Stat(script); err != nil {
			fmt.Fprintf(stderr, "%s: 门禁最小入口不在（%s）—— 它是自举件，就地缺席即硬错\n", progName, gateScriptRel)
			return exitUsage
		}
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
	// ★ 缺口 `GAP-20260929-22`（本片）：只读档 `gate ls` 的**粘贴表**开关（非 ls 档 ⇒ false ⇒ 零副作用）；
	//   lsBuf 是 stdout 的**旁路副件**（`io.MultiWriter`）—— 用户拿到的 stdout 逐字节不变（只转发不翻译）。
	lsPaste := false
	var lsBuf bytes.Buffer
	switch action {
	case "ls":
		if inv.jsonGiven {
			return gateLsJSON(inv, stdout, stderr, root, script)
		}
		// ★ 缺口 `GAP-20260927-306`（本片 · 2026-10-13）：门禁指纹行的**只读呈现面**。
		//   病：门跑那一趟算出的「当时工作树指纹」（`worktreeFingerprint` —— 口径唯一一处实现）
		//   只在真跑的产物（`freshness.json`）与新鲜度闸的判词里看得见，三条只读档（`--list`/
		//   `--emit-cmd`/`--self-test`）全拿不到 ⇒ 想核「门看到的树」必须先真跑一趟。
		//   `--fingerprint` = 单印该行（只读、不 exec 脚本、不改任何脚本退码）；算不出 ⇒ 照
		//   新鲜度闸同规退 8「不给结论」（✗ 不把算不出当空串、✗ 不回落别的数）。
		if len(tail) == 1 && tail[0] == "--fingerprint" {
			fp, head, ferr := worktreeFingerprint(root)
			if ferr != nil {
				fmt.Fprintf(stderr, "%s: 算不出当前工作树指纹 ⇒ 不给结论（退码 8）：%v\n", progName, ferr)
				return exitBlocked
			}
			fmt.Fprintf(stdout, "── 门禁指纹 · 工作树 %s · HEAD %s ──\n", fp, head)
			return exitOK
		}
		// ★ 缺口 `GAP-20260927-53`（2026-10-04）：`gate ls` 的用法面无旗标，而 `--list` 出口不读筛选旗标
		//   ⇒ 有尾参即 **fail-closed 拒执**（不 exec）。无尾参 ⇒ 原路径一字不动。判据与理由见 `gateLSRejectFlags`。
		if rc := gateLSRejectFlags(tail, stderr); rc != exitOK {
			return rc
		}
		// ★ 缺口 `GAP-20260929-22`（本片）：**不新增旗标** —— 只读档在脚本 `--list` 那份 stdout
		//   **逐字节转出之后**（stdout 一字不改），往 stderr 追加「可直接粘贴的整条命令」表 ＋ 引号口径。
		lsPaste = true
		args = []string{"--list"}
	case "run":
		// ★ 缺口 `GAP-20260927-02`（P1 · 命令面）：门跑**干跑档** —— `--dry-run` ⇒ 只摆出
		//   「本趟将跑 vs 被跳过」的步表（步名/档位/形态），**不起任何子进程、立即返回**
		//   （真材在 `family_gate_run_dry.go`）。判在一切下游拦截之前：`--dry-run` 不在自举件的
		//   旗标表里 ⇒ 不早判就会被下面的「旗标前移校验」当未知旗标拒（退 2）。
		//   缺 `--dry-run` 时一个字节都不动（默认全量的既有形态逐字照旧）。
		if inv.dryRun && gateRunWantsDryRun(tail) {
			return gateRunDryRun(inv, stdout, stderr, root, script, tail)
		}
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
		// ★ 缺口 `GAP-20261001-35`（本片）：单步档**不带 `--only-step`** ⇒ 脚本会先跑整套
		//   『合成步骤·不碰真目标』自检，其输出与真那一步**混在同一条 stdout**（既有两次误读）。
		//   命令面在 exec **之前**补一行**显式声明**（= stdout 首行 · 逐字串见 `family_gate_run_step.go`
		//   的 `gateRunStepMixedNotice`）；本支是三处前移校验**都过了**才走到 ⇒ 被拒执的发（未知旗标 /
		//   坏值）不会多打这行。`--only-step`/`--self-test` 不产合成段 ⇒ 不打；`--json`/`--show-log`/
		//   `--verify-live`/`--` 四档已在上面更早返回。哑化负控：`ZERG_GATE_STEP_NONOTICE`。
		if gateRunWantsMixedNotice(tail) {
			gateRunStepNotice(stdout)
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
		// ★ 本单（缺口 `GAP-20260929-28` 剩面 · 2026-10-14）：`--uncertain` = 只列全部「解析不确定」行
		//   （把 `reportUncertainSteps` 的 stderr 侧信道提到**正门** stdout / 机器面 ——
		//   `family_gate_find.go` 的 `cmdGateFindUncertain`）。不计位置参数；只读、不走脚本、
		//   不改任何脚本退码（与 `find` 同规）。
		if inv.hasFlag("--uncertain") {
			return cmdGateFindUncertain(inv, stdout, stderr, root, script)
		}
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
	if lsPaste {
		// ★ 缺口 `GAP-20260929-22`：只读档把脚本 `--list` 的 stdout **旁路**一份（MultiWriter 先写 stdout
		//   再写副本 ⇒ 用户拿到的字节与改前**逐字节相同**），供跑完后的粘贴表按同一份清单取名。
		cmd.Stdout = io.MultiWriter(stdout, &lsBuf)
	} else {
		cmd.Stdout = stdout
	}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// 门新鲜度闸：跑完了（哪怕红/不给结论）也照样把**当时**的指纹落进那一趟的产物。
			recordRunFreshness(root, runOutdirArg, runSince, freshFP, freshHead, stderr)
			// ★ 缺口 `GAP-20261005-46`：零命中 ⇒ 脚本那一行已印，这里**追加**候选与指路。
			if showZeroHit {
				gateShowZeroHitHint(stderr, showZeroDecls, showZeroName)
			}
			// ★ 缺口 `GAP-20260929-22`：只读档 `gate ls` 的粘贴表（追加在脚本输出之后 · 同 show 零命中体例）。
			if lsPaste {
				gateLsPasteFooter(stderr, lsBuf.String())
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
	// ★ 缺口 `GAP-20260929-22`：只读档 `gate ls` 的粘贴表（追加在脚本那份清单之后 · stderr）。
	if lsPaste {
		gateLsPasteFooter(stderr, lsBuf.String())
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

// gateLsFields —— `gate ls --json` 的**闭集四格**（与 `main.go` 登记 `fields:` 逐字同源：
// `total`/`scope`/`mode`/`step`）。未知字段 ⇒ 2 + stderr 列这四格（同族出口 `reportBadField`）。
var gateLsFields = []string{"total", "scope", "mode", "step"}

// gateLsOptFields —— `gate ls --json` 的**可选字段层**（缺口 `GAP-20260927-226` want①）：步名在
// want 形状 `{steps:[{name,mode,scope}],total:N}` 里写作 `name`（与同族 `gate find`/`gate explain`
// 的 `name` 逐字同名），而 `step` 是本命令既有的**冻结键**。可选字段**不进** `main.go` 登记 `fields:`
// 那一份（= 帮助/导出面的冻结表）⇒ 帮助面与命令页一行不动，只多一个点得出来的键（与 `code find`
// 的 `count` 同一体例）。`name` 与 `step` **同值**（都指同一步名）；裸 `--json` 一字不改。
// ★ 缺口 `GAP-20261010-13`（本片）：`gates_total` 同走**可选字段层** —— 步表机读面
// （`precommit-gates.sh --list --json`）新增的顶层格 `gates_total`（前序 DEV-2400 落的），
// 命令面此前**未随动** ⇒ `zerg gate ls --json gates_total` 退 2「未知字段」（加格到机读面 ≠
// CLI 打得着）。本处把它登进出口白名单（顶层标量格，与 `total` 同形：值 = `total` = `count`，
// 已由 `pgsParseListJSON` 的 `gates_total==len(steps)` 自证钉住）—— ✗ 不进冻结 `fields:`
// （帮助/导出面与命令页一行不动，同 `name` 体例）。逐字：`zerg gate ls --json gates_total`
// ⇒ `{"gates_total":N}`。
// ★ 缺口 `GAP-20260927-21`（本片）：`fast` 同走**可选字段层** —— 步表机读面
// （`precommit-gates.sh --list --json`）每步新增一格 `fast`（该步**是否在快速档** = 步名 ∈
// `FAST_STEPS`；名单**唯一来源是门脚本自己**，命令面只转发、✗ 不自留第二份）⇒
// `zerg gate ls --json fast` 点得出来（快速档 = `--fast` 跑的那批步 ⇒ 「某门不进快速档」可机读对拍）。
// 它是本命令**唯一的布尔格**（其余四格都是串）—— 逐字：
// `zerg gate ls --json fast` ⇒ `{"steps":[{"fast":true},{"fast":false},…]}`。
// ✗ 不进冻结 `fields:`（帮助/导出面与命令页一行不动，同 `name`/`gates_total` 体例）。
// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：`reds`（最近一趟非 PASS 的步名清单）同走**可选字段层**
//
//	（顶层数组格 · 与 `gates_total` 同形：✗ 不进冻结 `fields:` ⇒ 帮助 / 命令页一行不动）。
//
// ★ 缺口 `GAP-20260928-68`（本片 · 2026-10-14）：`id` 同走**可选字段层** —— 每步一个**稳定短 id**
// （`gateStepShortID`：`s` ＋ sha256(归一后步名) 前 4 字节 hex ⇒ `s1a2b3c4d`）。它是**步名确定性派生**
// 的内容指纹：**不含行号 / 序号 / 表内位置** ⇒ 步骤表里插一行、删一行、重排，其余步的 id **逐字不变**
// （行号当 id 必漂 —— 那正是本条要治的）；「步名 ↦ 短 id」因此是一张**映射**，✗ 不是第二张步骤表。
// 映射另**落件** `core/cmd/zerg/testdata/gate-step-ids.json`（真步名的冻结向量 + 机检）。**同名步**
// ⇒ 响亮判冲突（口径照 `gate explain` 的「步骤名出现 N 次 ⇒ 歧义」），✗ 不静默取首。
// 逐字：`zerg gate ls --json id` ⇒ `{"steps":[{"id":"s…"},…]}`。✗ 不进冻结 `fields:`（同 `name` 体例）。
var gateLsOptFields = []string{"name", "gates_total", "fast", "reds", "id", "cmd"}

// gateLsFieldLegal —— 字段是否在闭集四格内（与 `gateShowFieldLegal` 同形 · 不另写第二份口径）。
func gateLsFieldLegal(f string) bool {
	for _, ok := range gateLsFields {
		if f == ok {
			return true
		}
	}
	for _, ok := range gateLsOptFields {
		if f == ok {
			return true
		}
	}
	return false
}

// gateStepShortID —— 步的**稳定短 id**（缺口 `GAP-20260928-68` · 2026-10-14）：由步名**确定性派生**的
// 内容指纹。**输入里没有任何行号 / 序号 / 表内位置。**
//
//	· 形状：`s` ＋ `sha256(normStepName(步名))` 前 4 字节的 16 进制 ⇒ **8** 个十六进制字符（`s1a2b3c4d`）；
//	· 为什么这样定：本仓步名是**人读串**（含空格 / 冒号 / CJK / 反引号），而 `gate run --step` / `--fast` /
//	  脚本的 `filter_step` 一律**按名逐字精确匹配**（脚本口径「命中 0 次（要恰好 1）」）⇒ 工具与脚本手上
//	  没有一个短而稳的句柄。id 由名派生 ⇒ 引用面变短、变稳，而步骤表真源仍**只有脚本那一处**；
//	· `normStepName` 是**权威归一**（与脚本 `filter_step` 去反斜杠同一串空间 · 见 `family_gate_run_step.go`）
//	  ⇒ `--list` 形态与 `add_step` 源文形态（带反斜杠）派生出**同一个** id；
//	· **插 / 删一行不变**：id 只看步名 ⇒ 表里插一行、删一行、重排，其余步的 id 逐字不变（本片判据
//	  `gate_step_ids_test.go` 的 `TestGateStepShortID_StableUnderRowInsertDelete` 钉这一条）。
func gateStepShortID(name string) string {
	sum := sha256.Sum256([]byte(normStepName(name)))
	return "s" + hex.EncodeToString(sum[:4])
}

// gateStepIDRow —— 判「短 id 撞车」要的那三格（步名 / scope / 模式）；id 现算，✗ 不落第二张表。
type gateStepIDRow struct {
	Scope string
	Mode  string
	Name  string
}

// gateStepIDConflictReports —— 短 id 撞车清单（缺口 `GAP-20260928-68`）：把「**一个短 id 落在不止一个步上**」
// 的每一组逐条报出来，✗ 不替调用者择一（同族口径：`gate explain` 的「步骤名出现 N 次 ⇒ 歧义 · 退码 2」）。
//
// 两形态**同一条判据**（都报、不分家）：
//
//	① **同名步**（`normStepName` 归一后同名）⇒ 短 id 必同，而按名匹配的那几档本来就**不可能有唯一答案**；
//	② **异名同 id**（sha256 前 4 字节撞了）⇒ 概率极低但**必须报**：静默取首会让工具引到一个**不是它要的**
//	   步上 —— 「静默取首」正是这类假绿最容易被读成成功的地方。
//
// 返回值：每组一行（`短 id ⇒ N 步撞车；scope / 模式 / 步名…`），无撞车 ⇒ 空切片。
func gateStepIDConflictReports(rows []gateStepIDRow) []string {
	byID := map[string][]int{}
	order := []string{}
	for i, r := range rows {
		id := gateStepShortID(r.Name)
		if _, seen := byID[id]; !seen {
			order = append(order, id)
		}
		byID[id] = append(byID[id], i)
	}
	out := []string{}
	sort.Strings(order)
	for _, id := range order {
		idx := byID[id]
		if len(idx) < 2 {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s ⇒ %d 步撞在同一个短 id 上（判据要一个答案，不是一坨 · 退码 2）", id, len(idx))
		for _, i := range idx {
			fmt.Fprintf(&b, "；%s / %s / %q", rows[i].Scope, rows[i].Mode, rows[i].Name)
		}
		out = append(out, b.String())
	}
	return out
}

// gateLsJSON —— `gate ls --json [<字段>]` 机器面：仍只跑脚本 `--list`（**不另写步骤表**），
// 把脚本自己那一段「步骤清单」三列（scope/mode/名称）重新序列化成 JSON（G1-a 只转发）。
//
// ★ 缺口 `GAP-20261005-65`（2026-10-05 · 本片）：改前 `--json <字段>` 的**实参被整枚吞掉** ——
// 既不按字段投影、也不校名（`--json zzz` 与裸 `--json` 逐字节相同、静默 rc=0 = fail-open，
// 同族 `gate show`/`gate find` 一律 rc=2 + 「合法字段:」）。本处补两截：
//
//	① **执行前校名**：按闭集 `{total,scope,mode,step}` 逐枚核，闭集外 ⇒ `2` ＋ **复用同族出口**
//	   `reportBadField`（打「未知字段 + 合法字段: …」· ✗ 不新造第五套判词/退码）；
//	② **投影**：只出点到的格子（序即用户给的序）。
//
// 投影口径（本轮只做【硬面】＋ 顶层/行级的直投影）：
//
//	· `total`              = 顶层那一格（脚本 `--list` 末行的「共 N 步」）；
//	· `scope`/`mode`/`step` = 每一行的三格 —— 点名即投影（`steps` 容器**保留**、行内只留点到的键）。
//
// ★ **未定（待人拍板 · 本轮 ✗ 不自行定）**：`steps` 容器是否拍平成顶层数组（如 `"step":[…]`）·
// `steps` 本身是否应登记为可选字段 · 容器是否要改包封形状。⇒ 现行只做「点名即投影」这一层。
// ★ 裸 `--json`（不给字段）= 闭集全量：与改前**逐字节相同**（默认面一字不动）。
func gateLsJSON(inv *invocation, stdout, stderr io.Writer, root, script string) int {
	// ① 字段面（**执行前判** —— 在 exec 脚本之前）：闭集外 ⇒ 2 + 同族出口。
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateLsFields
	}
	for _, f := range fields {
		if !gateLsFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}

	// ★ GAP-20261010-25（本片）：**步表机读面优先**（`--list --json` 的单对象
	//   `{"count":N,"steps":[{scope,mode,name},…]}`）—— 复用同族助手 `pgsParseListJSON`
	//   （✗ 不另造第二套解析）；机读面在场而形状坏/自证不过 ⇒ **响亮失败**（rc=2 不给结论），
	//   ✗ 不许静默退回人面把读数当绿。人面（`--list` 逐行抠「共 N 步」+ 三列）**只作退化路**。
	//   两态：改前只抠人面 ⇒ 现读改走机读面；机读面不在（脚本不认 `--json`）⇒ 落回人面（与改前逐字相同）。
	type row struct {
		Scope string `json:"scope"`
		Mode  string `json:"mode"`
		Step  string `json:"step"`
		// fast —— 该步是否在快速档（缺口 `GAP-20260927-21`）。**小写且无 tag** ⇒ `json` 包
		// 不序列化它 ⇒ 裸 `--json`（`enc.Encode(… {"steps": rows})`）的逐字节形态**一字不动**；
		// 只有点名档 `--json fast` 手工拼它。
		fast bool
	}
	rows := []row{}
	total := 0
	machineDone := false
	// ★ 家族可取消面（本片 · 同族最后报出的一处）：此处原先是**裸 `exec.Command`**（未成组、未登记
	//   `runningChild`）⇒ 全局 `--timeout` / 墙钟到点（`wallclockGuard` ⇒ `cancelRunningChild` ⇒
	//   `terminateChildTree`）**打不到它**（到点子进程改嫁后照跑）。照同族正确形态补「成组 + 登记」
	//   两件（`armChildGroup` · `main.go:103` ＋ `runningChild` · `main.go:91`）—— 参数/语义/输出
	//   一字不动（仍 `bash <script> --list --json` 取 `.Output()`）；✗ 不用 `CommandContext`（它只杀
	//   直接子）—— 照树收先例 `ffpRunProbe`（`check_fields_face_parity.go:691`）走 `terminateChildTree`。
	mcmd := armChildGroup(exec.Command("bash", script, "--list", "--json"))
	runningChild = mcmd
	defer func() { runningChild = nil }()
	if mout, merr := mcmd.Output(); merr == nil {
		if msteps, mtotal, isM, perr := pgsParseListJSON(string(mout)); isM {
			if perr != "" {
				inv.setErr("usage", "gate_ls_machine_face_broken", perr)
				fmt.Fprintf(stderr, "%s: 步表机读面（`%s --list --json`）形状坏：%s ⇒ **不给结论**（✗ 不退回人面）\n",
					progName, gateScriptRel, perr)
				return exitUsage
			}
			for _, s := range msteps {
				rows = append(rows, row{Scope: s.Scope, Mode: s.Mode, Step: s.Name, fast: s.fast})
			}
			total = mtotal
			machineDone = true
		}
	}
	if !machineDone {
		// ② 退化路：机读面不在 ⇒ 人面 `--list`（与改前逐字相同）。
		c := armChildGroup(exec.Command("bash", script, "--list"))
		runningChild = c
		defer func() { runningChild = nil }()
		c.Dir = root
		var buf bytes.Buffer
		c.Stdout = &buf
		c.Stderr = stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(stderr, "%s: 跑不动 `--list`：%v\n", progName, err)
			return exitFail
		}
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
			// ★ 2026-10-05（缺口 `GAP-20261005-49`）：步名**取原文、不折叠空白** —— 本仓有步名含连续两个
			//   ASCII 空格（`core: linux  build ./...`），而 `strings.Fields`＋`Join` 会把它折成单空格 ⇒
			//   机器面读出**另一个名字**（与默认面 `--list` 的逐字形态不符；折后形拿去调 `--emit-cmd`/`gate show`
			//   一律零命中）。行形与切法**复用** `impactStepListLineRe`（`--list` 清单列的唯一切法 · ✗ 不另造第二套）。
			if m := impactStepListLineRe.FindStringSubmatch(t); m != nil {
				rows = append(rows, row{Scope: m[1], Mode: m[2], Step: m[3]})
				continue
			}
			rows = append(rows, row{Step: t})
		}
	}
	// 裸 `--json`（不给字段）= 闭集全量：与改前**逐字节相同**（默认面一字不动）。
	if len(inv.fields) == 0 {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(map[string]any{"total": total, "steps": rows}); err != nil {
			fmt.Fprintf(stderr, "%s: 出 JSON 失败：%v\n", progName, err)
			return exitFail
		}
		return exitOK
	}

	// ② 点名档：只出点到的几格（顶层 `total` + 行级 `scope`/`mode`/`step` · 序即用户给的序）。
	wantTotal := false
	wantGatesTotal := false
	wantFast := false
	wantReds := false
	wantID := false
	rowKeys := []string{}
	for _, f := range fields {
		switch f {
		case "total":
			wantTotal = true
		case "gates_total":
			// ★ GAP-20261010-13：顶层标量格（值 = total = count，已被 pgsParseListJSON 自证钉住）。
			wantGatesTotal = true
		case "scope", "mode", "step", "name", "cmd":
			// ★ 缺口 `GAP-20260929-22`（本片）：`cmd` 同走**可选字段层** —— 每步一格的
			//   **可直接粘贴的整条命令**（含正确引号 · 见 `gateLsPasteCmd`）。✗ 不进冻结
			//   `fields:`（帮助/导出面与命令页一行不动，同 `name` 体例）。
			rowKeys = append(rowKeys, f)
		case "fast":
			// ★ 缺口 `GAP-20260927-21`（本片）：每步**是否在快速档**（名单真源 = 门脚本的 `FAST_STEPS`）。
			wantFast = true
			rowKeys = append(rowKeys, f)
		case "id":
			// ★ 缺口 `GAP-20260928-68`（本片）：每步的**稳定短 id**（步名派生 · 现算 · ✗ 不落第二张表）。
			wantID = true
			rowKeys = append(rowKeys, f)
		case "reds":
			// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：红项清单（顶层格 · 不进 rowKeys）。
			wantReds = true
		}
	}
	// ★ 缺口 `GAP-20260927-21`（本片）：`fast` 的取数只有**一条**路 —— 步表机读面。两态都**响亮失败**，
	//   ✗ 不拿「全 false」当读数（那会把「取不到」伪装成「都不在快速档」= 把这条约束变成不可证伪）：
	//   ① 机读面不在（脚本不认 `--json`）⇒ 人面 `--list` 根本没有这一格；
	//   ② 机读面在场而**一步都没有** `fast:true` ⇒ 只可能是旧机读面缺该格（`FAST_STEPS` 非空 ∧
	//      脚本自检 ⑬ 钉住「名单每一条都能在默认集步骤表里找到」⇒ `--list` 默认集里必有 ≥1 步在快速档）。
	if wantFast {
		if !machineDone {
			inv.setErr("usage", "gate_ls_fast_needs_machine_face", "快速档归属取不到")
			fmt.Fprintf(stderr, "%s: `--json fast` 要**步表机读面**（`%s --list --json` 的每步 `fast` 格）；人面 `--list` 不带这一格 ⇒ 不给结论\n",
				progName, gateScriptRel)
			return exitUsage
		}
		anyFast := false
		for _, r := range rows {
			if r.fast {
				anyFast = true
				break
			}
		}
		if !anyFast {
			inv.setErr("usage", "gate_ls_fast_field_absent", "步表机读面缺 fast 格")
			fmt.Fprintf(stderr, "%s: 步表机读面（`%s --list --json`）**一步都没有** `fast:true` ⇒ 该格缺失（旧机读面）⇒ 不给结论（✗ 不把「全 false」当读数）\n",
				progName, gateScriptRel)
			return exitUsage
		}
	}
	// ★ 缺口 `GAP-20260928-68`（本片 · 2026-10-14）：`--json id` 的取数面**两路都够得着**（id 是步名的
	//   纯函数 ⇒ 步表机读面在不在都算得出，与 `fast` 那条「必须机读面」不同）⇒ 没有取数失败态；
	//   要拦的只有一件：**撞车**（一个短 id 落在不止一个步上）⇒ 响亮判红、**不给结论**（退 2），
	//   ✗ 不静默取首（口径照 `gate explain` 的「步骤名出现 N 次 ⇒ 歧义」）。
	if wantID {
		idRows := make([]gateStepIDRow, 0, len(rows))
		for _, r := range rows {
			idRows = append(idRows, gateStepIDRow{Scope: r.Scope, Mode: r.Mode, Name: r.Step})
		}
		if conf := gateStepIDConflictReports(idRows); len(conf) > 0 {
			inv.setErr("usage", "gate_ls_step_id_ambiguous", "步的短 id 撞车")
			fmt.Fprintf(stderr, "%s: `--json id` 判不了 —— 短 id 撞车（一个 id 落在不止一个步上 · **不给结论** · 退码 2）：\n", progName)
			for _, c := range conf {
				fmt.Fprintf(stderr, "  %s\n", c)
			}
			return exitUsage
		}
	}
	// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：`--json reds` —— 最近一趟里**非 PASS** 的步名清单
	//   （= 「本版必做几红」的单一收口面）。取数只走**既有**这条路（`gateFindLiveRun` + `gateResultsReadTSV`
	//   —— 与 `gate results --last` 同一真源），✗ 不另跑一趟门。
	//   最近一趟取不到 ⇒ **不给结论**（退码 8 · ✗ 不把空当读数 —— 同族 `fast` 那条铁律）。
	redList := []string{}
	if wantReds {
		dir2, _, ok2, ferr2 := gateFindLiveRun()
		if ferr2 != nil || !ok2 {
			inv.setErr("blocked", "gate_ls_reds_no_run", "本机没有现成的门禁产物")
			fmt.Fprintf(stderr, "%s: `--json reds` 要**最近一趟**门禁产物（`zerg gate results --last` 同一真源）—— 现读没有 ⇒ 不给结论（退码 8 · ✗ 不把空当读数）\n", progName)
			return exitBlocked
		}
		rr2, _, rerr2 := gateResultsReadTSV(filepath.Join(dir2, gateResultsTSV))
		if rerr2 != nil {
			inv.setErr("blocked", "gate_ls_reds_unreadable", "最近一趟结果表读不到")
			fmt.Fprintf(stderr, "%s: 最近一趟 %s 读不到（%v）⇒ 不给结论（退码 8）\n", progName, dir2, rerr2)
			return exitBlocked
		}
		for _, r2 := range rr2 {
			if r2.Status != "PASS" {
				redList = append(redList, r2.Name)
			}
		}
	}
	var b strings.Builder
	b.WriteString("{")
	n := 0
	if wantTotal {
		fmt.Fprintf(&b, "\"total\":%d", total)
		n++
	}
	if wantGatesTotal {
		if n > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "\"gates_total\":%d", total)
		n++
	}
	// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：顶层红项清单（序 = 结果表序 · 同名同源）。
	if wantReds {
		if n > 0 {
			b.WriteString(",")
		}
		rl, _ := json.Marshal(redList)
		b.WriteString("\"reds\":")
		b.Write(rl)
		n++
	}
	if len(rowKeys) > 0 {
		if n > 0 {
			b.WriteString(",")
		}
		b.WriteString("\"steps\":[")
		for i, r := range rows {
			if i > 0 {
				b.WriteString(",")
			}
			// ★ 行级对象按**用户给的键序**拼 —— `fast` 是布尔格（本命令唯一非串格）
			//   ⇒ ✗ 不走 `marshalObject`（取值面是 `map[string]string`，bool 会被出成 `"true"` 串）。
			b.WriteString("{")
			for j, f := range rowKeys {
				if j > 0 {
					b.WriteString(",")
				}
				switch f {
				case "fast":
					b.WriteString("\"fast\":")
					if r.fast {
						b.WriteString("true")
					} else {
						b.WriteString("false")
					}
				default:
					v := r.Step
					if f == "scope" {
						v = r.Scope
					} else if f == "mode" {
						v = r.Mode
					} else if f == "id" {
						// ★ 缺口 `GAP-20260928-68`：短 id 由步名现算（✗ 不落第二张表 · ✗ 不看行号）。
						v = gateStepShortID(r.Step)
					} else if f == "cmd" {
						// ★ 缺口 `GAP-20260929-22`：可直接粘贴的整条命令（含正确引号 · 步名现算）。
						v = gateLsPasteCmd(r.Step)
					}
					kk, _ := json.Marshal(f)
					vv, _ := json.Marshal(v)
					b.Write(kk)
					b.WriteString(":")
					b.Write(vv)
				}
			}
			b.WriteString("}")
		}
		b.WriteString("]")
	}
	b.WriteString("}\n")
	if _, err := io.WriteString(stdout, b.String()); err != nil {
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
	gateStepQuoteHint(stderr, name)
	fmt.Fprintf(stderr, "要列全部步名 ⇒ `zerg gate ls`；已知门件名 ⇒ `zerg gate find <门件名>` 反查步名\n")
}

// ── 缺口 `GAP-20260929-22`：步名的**引号安全**（可直接粘贴的整条命令 ＋ 未命中提示）──────────────
//
// 病（现读 · 第三批调研血证）：步名是人读串（含空白 / CJK / 反引号 / `$`），而 `gate run --step` /
// `gate show` / `gate explain` 一律**按名逐字精确匹配** ⇒ 「怎么把这个串安全地交给 shell」全靠
// 调用者自己知道，三种粘法里**只有一种对**：
//
//	· 裸粘   ⇒ shell 按空白切词 ⇒ 步名被切散 ⇒ 退 2（word-split）；
//	· 双引号 ⇒ `$HOME` 一类被**展开**、反引号被**执行** ⇒ 串被改写 ⇒ 退 2（假红）；
//	· 单引号 ⇒ 逐字不动（唯一正确形态）。
//
// 本处**不新增旗标**（用户拍定形态），只做两件：① 只读档 `gate ls` 追加「可直接粘贴的整条命令」表；
// ② 步名未命中时提示「可能是引号/展开所致，试试单引号」。两者都**不改任何脚本退码**，也**不动 `gate ls`
// 的 stdout**（那句「只转发、不翻译」不下岗 —— 本命令 stdout 仍是脚本 `--list` 的逐字节转出）。

// gateLsShellQuote —— 步名的 **shell 单引号逐字**形态：整串用一对单引号包住，内嵌的单引号按
// 「闭引号 ＋ 反斜杠单引号 ＋ 重开引号」的写法闭合（POSIX 的**唯一**正确写法）。
//
// ★ 为什么必须走这一枚、✗ 不许就近拼：单引号内**没有**转义语法，直接把串塞进一对单引号里，
// 步名一带单引号就会把引号结构撑破、后半段重新落回 shell 解析面。
// ★ 自证（真喂 `bash -c` 转一圈比对逐字节）：`TestGateLsShellQuoteSelfProven`；
// 成对负控（裸粘 / 双引号 ⇒ 串被改写）：`TestGateLsShellQuotePaired`。
func gateLsShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// gateLsPasteCmd —— 一步的**可直接粘贴的整条命令**（含正确引号 · 正式档）。
func gateLsPasteCmd(step string) string {
	return progName + " gate run --step " + gateLsShellQuote(step) + " --only-step"
}

// gateLsPasteReadonlyCmd —— 同一条命令的**只读零副作用**形态（只想核「引号对不对」时用）。
func gateLsPasteReadonlyCmd(step string) string {
	return gateLsPasteCmd(step) + " " + gateRunDryFlag
}

// gateLsNameNeedsQuote —— 步名是否含空白或 shell 活动字符（裸粘会被切词、双引号会被展开/执行）。
// 非也是同样一条口径（一律单引号），只是这股在表里不加星标。
func gateLsNameNeedsQuote(s string) bool {
	return strings.ContainsAny(s, " \t`$'\"\\|&;<>()*?[]{}!#~")
}

// gateLsPasteRowsFromList —— 从脚本 `--list` 的**人面正文**里抠出 (scope, mode, 步名) 清单。
// ★ 切法与 `gateLsJSON` 的退化路**同一把**（`impactStepListLineRe` · ✗ 不另造第二套）：这样表里的
// 步名与用户刚看到的那一份清单**逐字同源**（机读面那份 JSON 是另一份文本，不走这里）。
func gateLsPasteRowsFromList(body string) []gateStepIDRow {
	rows := []gateStepIDRow{}
	for _, ln := range strings.Split(body, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "共") || strings.HasPrefix(t, "──") {
			continue
		}
		if m := impactStepListLineRe.FindStringSubmatch(t); m != nil {
			rows = append(rows, gateStepIDRow{Scope: m[1], Mode: m[2], Name: m[3]})
			continue
		}
		rows = append(rows, gateStepIDRow{Name: t})
	}
	return rows
}

// gateLsPasteFooter —— 只读档 `gate ls` 跑完之后**追加**的「可直接粘贴的整条命令」表（走 stderr）。
//
// 为什么走 stderr 而不是 stdout：本命令的 stdout 是脚本 `--list` 那一份的**逐字节转出**
// （`TestCLIExecPassthroughListIsVerbatim` 钉着这一条）⇒ 一个字节都不许动；故这里只**追加**到
// stderr，与 `gate show` 零命中指路（`gateShowZeroHitHint`）同一体例。
// 步名清单取自**用户刚看到的那份 stdout 旁路副本**（`cmdGate` 的 `lsBuf`）⇒ 表 ⟷ 清单逐字同源，
// ✗ 不是另跑一趟脚本、✗ 不是另抄一份步骤表。
func gateLsPasteFooter(stderr io.Writer, listBody string) {
	fmt.Fprintln(stderr, "── 可直接粘贴的整条命令（缺口 `GAP-20260929-22` · 逐字单引号）──")
	fmt.Fprintln(stderr, "引号口径：步名一律**单引号**逐字包住 —— ✗ 裸粘会被 shell 按空白切词（步名被切散 ⇒ 退 2）；")
	fmt.Fprintln(stderr, "          ✗ 双引号会被 shell 展开 `$` 或**执行**反引号（步名串被改写 ⇒ 退 2 假红）。")
	fmt.Fprintln(stderr, "          步名自带单引号时按 `'\\''` 闭合（下表已按此拼好，照抄即可）。")
	rows := gateLsPasteRowsFromList(listBody)
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 步骤清单正文里没抠到一条步名 ⇒ 不列粘贴表（✗ 不编步名 · 退码不变）\n", progName)
		return
	}
	for _, r := range rows {
		mark := "  "
		if gateLsNameNeedsQuote(r.Name) {
			mark = "★ " // ★ = 名字含空白或 shell 活动字符（裸粘 / 双引号会被改写）
		}
		fmt.Fprintf(stderr, "%s%s\n", mark, gateLsPasteCmd(r.Name))
	}
	fmt.Fprintln(stderr, "★ = 步名含空白或 shell 活动字符；每一条都**照抄即可**。")
	fmt.Fprintf(stderr, "只想核「引号对不对」（零副作用）：%s\n", gateLsPasteReadonlyCmd("<步名>"))
	fmt.Fprintf(stderr, "机读面同表（每步一格 `cmd`）：%s gate ls --json cmd\n", progName)
}

// gateStepQuoteHint —— 步名**未命中**时的引号指路（缺口 `GAP-20260929-22` ②）。
//
// 步名是人读串，「粘进 shell 时怎么引用」是这条路上**唯一**的失败源（裸粘 word-split / 双引号被展开）
// ⇒ 每条「步名找不到」的判词后都补这一段；四处（`gate run --step --json` / `--verify-live` 与
// `--show-log` 与 `--dry-run` 共用的 `resolveGateStep` / `gate show --json` / `gate show` 默认面
// 零命中指路）**共用这一处口径**（✗ 不各写一份）。
func gateStepQuoteHint(stderr io.Writer, given string) {
	fmt.Fprintf(stderr, "可能是引号/展开所致 —— 试试**单引号**逐字包住（✗ 裸粘会被空白切词；✗ 双引号会被 shell 展开 `$` 与反引号 ⇒ 串被改写、退 2 假红）：\n")
	fmt.Fprintf(stderr, "  %s gate run --step %s --only-step\n", progName, gateLsShellQuote(given))
	fmt.Fprintf(stderr, "  每一步可直接粘贴的整条命令 ⇒ `%s gate ls`（或 `%s gate ls --json cmd`）\n", progName, progName)
}
