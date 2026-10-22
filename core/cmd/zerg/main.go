// Command zerg —— 虫族命令面（薄壳 · 唯一入口）。S1 骨架：`version` + `help` 两条。
//
// 契约（成文）：Zerg-内部文档/项目文档/v2.5.10/契约-命令面-v1.0-20260920.md
// 设计定稿：Zerg-内部文档/项目文档/v2.5.10/设计-命令面与契约-v0.7.md §6.1 / §6.3 S1 / §7.1 P2
//
// 两条进程语义（§6.1 的判据）：本二进制**跑完即退** —— 无常驻、不绑端口、不强制令牌；
// `zerg-core` 常驻绑 8580。两者**同 module**（复用 core/internal/*），不是两个仓两套契约。
//
// 流分工（§4.1 K4）：stdout 只出结果（含 --json 整段）；进度/日志/错误一律 stderr。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// progName —— 报头 / 帮助 / 命令名三处共用的名字（§3.3 N1：定名 `zerg`）。
const progName = "zerg"

// 退出码（契约 §三 主表；完整表与占号纪律由 `zerg help exit-codes` 自描述）。
// 批 B 启用三档（原「已挂号未启用」· §十二 P-013 ②③ 与 §九 M7/M8 的落地）：
// `10` 资源不足 · `11` 超时 · `12` 不可达 · `14` 冲突/被占（**不许**再用 `507` 表达）。
const (
	exitOK          = 0
	exitFail        = 1   // 一般失败
	exitUsage       = 2   // 用法错 / 不给结论
	exitPartial     = 3   // 部分趟（只跑子集且子集全绿 · 不覆盖全量）
	exitAuth        = 4   // 未认证（403 与它并码、kind 分家）
	exitBlocked     = 8   // 有 BLOCKED
	exitResource    = 10  // 资源不足
	exitTimeout     = 11  // 超时
	exitUnreachable = 12  // 不可达（打不到主控）
	exitConflict    = 14  // 冲突 / 被占
	exitInterrupted = 130 // 人打断（Ctrl-C）
)

func main() {
	// 人打断 / 被要求终止（§九 M8 · §十二 `P-033`）：**默认只退订、不取消** —— 退 `130`，
	// 被跟的任务/被包的脚本**继续跑**；要真取消得给显式旗标 `--cancel-on-interrupt`。
	// 为什么不在这里做别的清理：命令面**无常驻状态**（§6.1），「退订」= 本进程退出。
	// 2026-10-02（缺口 `GAP-20261002-11`）：触发面不只 Ctrl-C —— `SIGTERM`/`SIGHUP` 同路，
	//   **退码仍统一 `130`**（不退 143/129：主表无号，裸数字会被门⑪ 判「表外码」）。
	sigs := make(chan os.Signal, 1)
	// 2026-10-02（缺口 `GAP-20261002-11`）：信号面从**只有 SIGINT** 扩到 `SIGINT`/`SIGTERM`/`SIGHUP`。
	// 缺口的实测形态是「父被 TERM 掉 ⇒ 子进程组无主仍跑（PPID=1 · 占住并发门跑闸）」
	// —— 清理逻辑（`cancelRunningChild`）一直在，从没被触发过，因为处理器只听 Ctrl-C。
	// `SIGKILL` 捕不到（那一路由探针自灭 ＋ `gate reap` 兜），但 `SIGTERM`/`SIGHUP` 是
	// **有主可捕**的常见形态（脚本 timeout、CI 取消、登出）。
	// **语义面一字不改**：仍尊重 `--cancel-on-interrupt`（默认只退订、不取消）。
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		sig := <-sigs
		what := "人打断（Ctrl-C）"
		if sig != syscall.SIGINT {
			what = fmt.Sprintf("收到 %v", sig)
		}
		if wantsCancelOnInterrupt(os.Args[1:]) {
			cancelRunningChild()
			fmt.Fprintf(os.Stderr, "%s: %s —— 已按 `--cancel-on-interrupt` **取消**在跑的步骤\n", progName, what)
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s —— **只退订、不取消**（在跑的任务/脚本不受影响；要取消给 --cancel-on-interrupt）\n", progName, what)
		}
		removeActiveMaterialized()
		os.Exit(exitInterrupted)
	}()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// wantsCancelOnInterrupt 只看命令行里有没有那枚显式旗标（不看配置：它是**本次**的意图）。
func wantsCancelOnInterrupt(args []string) bool {
	for _, a := range args {
		if a == "--cancel-on-interrupt" || a == "--cancel-on-interrupt=true" {
			return true
		}
	}
	return false
}

// runningChild —— 当前在跑的子进程（透传型命令登记在这里，供「取消」用；nil = 没有）。
var runningChild *exec.Cmd

// childKillGrace —— 两段收尾的宽限：TERM 之后等这么久，幸存者再吃 KILL。
// 先例：GNU `timeout -k`（先 TERM、宽限后 KILL）· lefthook（`Cancel` = `kill(-pgid, SIGKILL)`）。
// 是变量：判据件要把它压短（否则每条超时用例都要真等满宽限）。
var childKillGrace = 2 * time.Second

// armChildGroup 在**起子进程之前**把它自成一组（组长 = 它自己），返回同一个 cmd 便于链式登记。
//
// 为什么必须成组：到点要收的是**一整棵**。命令面起的多是 `bash <脚本>`，脚本再 fork 孙进程
// （门步骤 / `go test` / 子脚本）⇒ 只打直接子进程的话，孙进程改嫁后照跑（`GAP-…`：实测
// 不处理信号的壳被 `kill -9` 后，其子件/孙件 `PPID=1` 继续跑）。
func armChildGroup(cmd *exec.Cmd) *exec.Cmd {
	if cmd == nil {
		return cmd
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return cmd
}

// cancelRunningChild 收**本趟亲手起的那一棵**：先 TERM 到进程组 → 宽限 → KILL 到进程组。
func cancelRunningChild() {
	terminateChildTree(runningChild)
}

// terminateChildTree 两段收尾（GNU `timeout -k` 的先例）：先给整树一枚 SIGTERM（给得出优雅退出
// 的进程一个窗口），宽限内没走干净 ⇒ 再对**重算过**的清单发 SIGKILL。
func terminateChildTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	pid := cmd.Process.Pid
	signalChildTree(childTreeTargets(pid), syscall.SIGTERM)
	if childTreeGone(pid) {
		return
	}
	// 宽限里可能又冒出新后代 ⇒ 收尾前重算一次靶清单。
	signalChildTree(childTreeTargets(pid), syscall.SIGKILL)
}

// childTreeTargets —— 这一趟收尾要打的靶：**成组**（组长 = 自己）⇒ 只给 `-pgid`（一条信号管全树，
// 且不碰调用者与别人的组）；**未成组** ⇒ 本 pid + 沿 `ppid` 树收全部后代（只收自己这棵）。
func childTreeTargets(pid int) []int {
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid {
		return []int{-pid}
	}
	return descendantPids(pid)
}

// descendantPids —— 沿 `ps -Ao pid=,ppid=` 的 `ppid` 树从 root 收全部后代（含 root 自己）。
// 读不到进程表 ⇒ 只给 root（宁可弱一拍，也不误伤别人）。
// 只用于**没走 `armChildGroup`** 的少数落点：这种收法认的是当下的父子关系，已改嫁的漏网。
func descendantPids(root int) []int {
	out := []int{root}
	raw, err := exec.Command("ps", "-Ao", "pid=,ppid=").Output()
	if err != nil {
		return out
	}
	kids := map[int][]int{}
	for _, ln := range strings.Split(string(raw), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil {
			continue
		}
		kids[pp] = append(kids[pp], p)
	}
	seen := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, c := range kids[p] {
			if seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}

// signalChildTree 逐靶发一枚信号（负靶 = 整组；`ESRCH` 当已不在，不当失败）。
func signalChildTree(targets []int, sig syscall.Signal) {
	for _, tgt := range targets {
		_ = syscall.Kill(tgt, sig)
	}
}

// childTreeGone 宽限内轮询「整棵收干净了吗」；到点还没干净 ⇒ 假（交给第二段 KILL）。
func childTreeGone(pid int) bool {
	deadline := time.Now().Add(childKillGrace)
	for {
		if !childTreeAlive(pid) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// childTreeAlive —— 成组看整组还有没有活的；未成组看本树逐个。
func childTreeAlive(pid int) bool {
	for _, tgt := range childTreeTargets(pid) {
		if syscall.Kill(tgt, 0) == nil {
			return true
		}
	}
	return false
}

// run 是唯一入口的实现面：解析旗标 → 查命令树 → 执行 → 把码原样返回。
// 拆出 run 是为了让契约测试能用 `package main_test`（黑盒）驱动，不必起进程。
func run(args []string, stdout, stderr io.Writer) int {
	// ── §十二 `P-103` 定案（批 E · T-60）：命令面**一律不接受 `sudo`** ───────────────────
	// 位置：**调度之前**（也在命令树解析之前）—— 命令面是「人与 AI 只敲命令」的唯一入口，
	// 入口都不收，才谈得上「一律」。子端侧的第二道在 `rules.yaml` 的 `terminal.args_deny`。
	if hit, ok := sudoRefusal(args); ok {
		return writeSudoRefusal(&invocation{orig: append([]string{}, args...)}, hit, stdout, stderr)
	}
	inv, err := parseInvocation(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	// 缺口 `GAP-20260928-151`（值旗标吞旗标 · P1）：那枚「看起来像旗标」的词**不**收下当值（裸形语义
	// 照旧：给过、值空 ⇒ `hasFlag` 与 `--acceptance` 三态、`publish run --out` / `script inventory
	// sync --set` 两个守卫逐字不变），但**当场报出来** ⇒ 不再「rc=0 且无信号」的假绿那一半。
	for _, w := range inv.valueFlagLooksLikeFlag {
		fmt.Fprintf(stderr, "%s: ⚠ %s\n", progName, w)
	}

	// `zerg --version` / `zerg version`：同一条命令的两个写法（§九 M15 T4）。
	if inv.wantVersion && len(inv.path) == 0 {
		inv.path = []string{"version"}
	}
	if inv.wantHelp && len(inv.path) == 0 {
		return cmdHelp(inv, stdout, stderr)
	}
	if len(inv.path) == 0 {
		if len(inv.unknown) > 0 {
			fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, inv.unknown[0])
			fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
			return exitUsage
		}
		return cmdHelp(inv, stdout, stderr)
	}

	cmd, rest := resolve(inv.path)
	if cmd == nil {
		// 插件（§6.3 S6 的 `zerg-<名>` 约定）：命令树里没有这条 ⇒ 先问插件，再报未知命令。
		if rc, done := tryPlugin(inv, stdout, stderr); done {
			return rc
		}
		// §4.1 K14 四件套：自报是谁 · 下一步 · 最像的合法输入 · 上下文定位。
		s := nearest(inv.path[0])
		nodeName := strings.Join(inv.path, " ")
		// ★ 缺口 `GAP-20260925-27` / `GAP-20260927-400` / `GAP-20260928-144`：`<族名|中间节点> --help`
		//   要的是「首行判词分家」（族名不是「真的未知命令」）。
		//   ★ 2026-10-04（Mr2109 授权 · 本枚）：**已落** —— 首词是真族名/中间节点（`isNode`）⇒ 首行换一句
		//   区分判词（**不再说「未知命令」**）；**真未知命令**（首词非族名）照旧逐字 `未知命令`。
		//   **rc 仍 = 2**（两态 rc 语义一字不动）；同一节点 `X` 与 `X --help` 首行仍**逐字相同**（同一句）。
		//   落点纪律：`cli_help_twostate_test.go` 的「真未知命令」格（`frobnicate`）一字不动；族名/节点那一格
		//   改成断言分家后的首行 + 两态首行相同（同批测试件）。
		// ★ 修：判据从「拿 nearest 的回串反推」改成**直查命令树**（`isFamilyNode`）——
		//   nearest 按登记序取**第一个**前缀命中，族名与另一条共享前缀的命令并存时会先撞上
		//   **别的**名字（`doc` 撞 `doctor`、`gateway` 撞 `gate`）⇒ 原来的回串判据把它们判成
		//   假 ⇒ 两族首行错成「未知命令」。直查命令树：存在一条命令首段 == 首词 ⇒ 就是族名/中间节点。
		isNode := len(inv.path) > 0 && isFamilyNode(inv.path[0])
		if isNode {
			fmt.Fprintf(stderr, "%s: %q 是族名/中间节点 —— 不是一个可执行动作（这一态不吃 `--help` · 族级/该节点帮助见下行）\n",
				progName, nodeName)
		} else {
			fmt.Fprintf(stderr, "%s: 未知命令 %q\n", progName, nodeName)
		}
		if s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s %s\n", progName, s)
		}
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		// 2026-09-27（缺口 `-23` · Mr2109 拍板选项②）：族名不吃 `--help`（`zerg gate --help` 退 2）⇒
		//   在块内补一行**族级帮助**正门指引（**只进 stderr** · 不动既有两态语义）。
		// 2026-09-27（缺口 `GAP-20260927-40` · 修正）：**只有首词确实是真族名时才打** ——
		//   判据 = `nearest` 给的最像命令首词与它相同（敲错字时 nearest 指向的是**别的族** ⇒ 不打，
		//   免得把用户指到一条不存在的正门：`zerg eggs` 曾打出 `zerg help eggs`）。
		if isNode {
			fmt.Fprintf(stderr, "族级帮助: `%s help %s`\n", progName, inv.path[0])
			// ★ 缺口 `GAP-20260926-138`（中间节点缺 help 面）：`zerg script inventory --help` 此前
			//   只回一句 `未知命令 "script inventory"` ⇒ 这一层有哪些动作读不出来（只有全路径 `sync` 有）。
			//   现读命令树**现算**该节点下的**直接动作**逐条列出（同源 · 命令树长一条这条跟着长）
			//   （只进 stderr · rc 不变 —— 仍退 2）。★ 只在 `--help` 那一态加（无 `--help` 的面一字不动）。
			if inv.wantHelp {
				if kids := childActions(inv.path); len(kids) > 0 {
					fmt.Fprintf(stderr, "该节点下的动作: %s\n", strings.Join(kids, " · "))
				}
			}
		}
		return exitUsage
	}
	// ★ 2026-09-24（缺口 `Q-137` / `Q-141` · 组A）：`--help` **分两态**的第一态 —— 走到这里说明
	//   命令**在册** ⇒ 出该条**用法串**（不再打全局树）· **不起命令**（零副作用）· 退 `0`。
	//   第二态（命令不在册）在上面那条「未知命令」分支：stderr 首行逐字 `未知命令 …` · 退 `2`。
	//   两态**同族同码**：`zerg frobnicate --help` 与 `zerg frobnicate` 都退 `2`、首行同为 `未知命令`。
	if inv.wantHelp && !passthroughChildHelp(cmd, inv, rest) {
		return cmdUsageHelp(cmd, stdout)
	}
	if inv.wantHelp {
		// ★ 本笔：走到这里 = 透传型命令 + `-h` + 已点名子件目标 ⇒ **`-h` 交给子件**。
		//   清掉 wantHelp（下面不再有读者，但别留一个「要帮助」的假信号）；`-h` 本身仍在
		//   `inv.orig` 里 —— 透传壳（如 `gate check` 的 `inv.orig[2:]`）逐字转发，子件自己印 usage。
		inv.wantHelp = false
	}
	inv.path = cmd.path
	inv.args = append(rest, inv.args...)
	inv.tty = ttyOf(stdout)
	// §17.3 铁律④③（D3b 第三步）：**命令面一律不接受 `--no-verify`** —— 它是「绕过提交闸」的唯一写法，
	// 而提交闸（`.githooks/pre-commit` 出口②）正是「AI 不许自评自批」那一层。对所有命令在 dispatch 里拒。
	for _, u := range inv.unknown {
		if u == "--no-verify" || u == "--no-verify=true" {
			inv.setErr("usage", "no_verify_forbidden", "命令面一律不接受 --no-verify")
			fmt.Fprintf(stderr, "%s: `--no-verify` **一律不接受**（§17.3 铁律④③「AI 不许 --no-verify」）\n", progName)
			fmt.Fprintf(stderr, "要绕行只有人能**显式**做（`git commit --no-verify`，会被 git 记在命令历史里）—— 命令面不开这个口子 ⇒ 退码 2\n")
			return exitUsage
		}
	}
	// ★ 批1 第四片（本枚）：`--top <N>` 是 `gap status` 排行面的**专用旗标** —— 它在 `valueFlagName`
	//   上刚上户口，而那张名字表是**全族共用的一张** ⇒ 若不在这里收口，别的命令（`gap ls` 等）真给
	//   `--top` 会从「未知旗标 2」**退化成静默吞**（硬约束⑤：既有命令的退码与输出**一字不动**）。
	//   治法与形状**逐字照拄**同一处 dispatch 面的既有拒收路：把不归本命令的那一枚**补回 `inv.unknown`**
	//   ⇒ 由下面那条**同一条**判词 + 同一个退码 2 收口（不另写一句判词 · 不另取一个码）。
	// ★ 缺口 `GAP-20260928-185`（2026-10-04 · 本枚）：判据从**逐命令硬枚举**（`cmd.path` 写死
	//   `gap status`/`gap plan` 两条）改成**数据驱动** —— 「本命令的 usage 串里写没写 `--top`」
	//   （`usageDeclaresFlag`；usage 是命令树里**同源**的声明面）⇒ 后续任何用 `--top` 的命令
	//   只要在 usage 里写上它即归它，不必再改这一处。既有两条命令 usage 里本就有 `--top`
	//   ⇒ 行为与退码**一字不动**（`gap status --top 3` 仍 rc=0 · `gap ls --top 3` 仍 rc=2）。
	//   `--by` 不在此列：它**早已**在名字表上（与 `dev proposal` / `gap add` 共用）⇒ 既有行为不变。
	if inv.hasFlag("--top") && !usageDeclaresFlag(cmd, "--top") {
		inv.unknown = append(inv.unknown, "--top")
	}
	// ★ 同一条数据驱动收口（缺口账 `GAP-20261005-60` 剩面 · 2026-10-05）：`--include-open` 在解析面
	//   刚上户口（`parseInvocation` 专用 case）⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）
	//   真给 `--include-open` 会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--top` 那条：把不归本命令的那一枚**补回 `inv.unknown`** ⇒ 由下面**同一条**
	//   判词 + 同一个退码 2 收口（不另写判词 · 不另取码）。`gap reread` 的 usage 已列它 ⇒ 行为逐字不变。
	if inv.hasFlag("--include-open") && !usageDeclaresFlag(cmd, "--include-open") {
		inv.unknown = append(inv.unknown, "--include-open")
	}
	// ★ 缺口账 `GAP-20260929-41`（2026-10-23）：`--bucket` 只对 `gap status` 有意义（按渲染桶名收窄）；
	//   既有的 `gap ls` / `gap show` **不许静默吞** —— 没在 usage 里声明 ⇒ 一律按未知旗标退 2
	//   （硬约束⑤：既有命令的退码与输出一字不动）。
	if inv.hasFlag("--bucket") && !usageDeclaresFlag(cmd, "--bucket") {
		inv.unknown = append(inv.unknown, "--bucket")
	}
	// ★ 同一条数据驱动收口（P0 #232 · 2026-10-08）：`--hours` 在解析面刚上户口（`valueFlagName`）⇒ 全族共用；
	//   若不在此收口，别族（usage 未列它的命令）真给 `--hours` 会从「未知旗标 2」退化成**静默吞**
	//   （硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 `--top` 那条。
	if inv.hasFlag("--hours") && !usageDeclaresFlag(cmd, "--hours") {
		inv.unknown = append(inv.unknown, "--hours")
	}
	// ★ 同一条数据驱动收口（P0 #232 · 2026-10-08）：`--max` / `--poll-interval` / `--max-retries` /
	//   `--cooldown` 在解析面刚上户口（`valueFlagName`）⇒ 全族共用；若不在此收口，别族（usage 未列它的
	//   命令）真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--hours` 那条。
	for _, f := range []string{"--max", "--poll-interval", "--max-retries", "--cooldown"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（缺口 `GAP-20260928-196` · 2026-10-10）：`--before-cmd` / `--after-cmd`
	//   在解析面刚上户口（`valueFlagName`）⇒ 全族共用；若不在此收口，别族（usage 未列它的命令）真给它们
	//   会从「未知旗标 2」退化成**静默吞**（硬约束⑤）。形状逐字照拄上面 `--max` 那条。
	for _, f := range []string{"--before-cmd", "--after-cmd"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 本枚（main.go 开窗第 4 条）：`--judge-cmd` 在解析面刚上户口（`valueFlagName`）⇒ 同族共用；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤）。
	//   形状逐字照拄上面 `--before-cmd` 那条（把不归本命令的那一枚**补回 `inv.unknown`**）。
	if inv.hasFlag("--judge-cmd") && !usageDeclaresFlag(cmd, "--judge-cmd") {
		inv.unknown = append(inv.unknown, "--judge-cmd")
	}
	// ★ 同一条数据驱动收口（缺口 `GAP-20261010-16` · want③）：`--window-min` 在解析面刚上户口
	//   （`valueFlagName`）⇒ 全族共用；若不在此收口，别族（usage 未列它的命令）真给它
	//   会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--judge-cmd` 那条（把不归本命令的那一枚**补回 `inv.unknown`**）。
	if inv.hasFlag("--window-min") && !usageDeclaresFlag(cmd, "--window-min") {
		inv.unknown = append(inv.unknown, "--window-min")
	}
	// ★ 数据驱动收口（缺口 `GAP-20260928-13` · 2026-10-19）：`--confirm` 早在全族共用的
	//   解析面上（`dev edit` / `agent deploy` 用）⇒ 不归本命令时会被**静默吞**。
	//   反例（改前实测）：`zerg repo commit --confirm=x` 被吞后一路走到「要一枚 --message」。
	//   ★ 只对 `repo commit` 收窄（不铺成全族）：别的命令有**自己的** `--confirm` 语义
	//   （如「确认值不匹配目标」那条 block 见证），铺开会把它们的判词顶成「未知旗标」——
	//   本轮实测踩过（TestAuthorityMapWitness 第 6 行当场红），故按路径精确生效。
	if len(cmd.path) == 2 && cmd.path[0] == "repo" && cmd.path[1] == "commit" && inv.confirmGiven {
		inv.unknown = append(inv.unknown, "--confirm")
	}
	// ★ 同一条数据驱动收口（缺口账 `GAP-20260928-172` · 2026-10-10 · 甲案）：`--force-recompute`
	//   在解析面刚上户口（`parseInvocation` 专用 case）⇒ **全族共用**；若不在此收口，别族（usage
	//   未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出
	//   一字不动）。形状逐字照拄上面 `--include-open` 那条：把不归本命令的那一枚**补回
	//   `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	if inv.hasFlag("--force-recompute") && !usageDeclaresFlag(cmd, "--force-recompute") {
		inv.unknown = append(inv.unknown, "--force-recompute")
	}
	// ★ 同一条数据驱动收口（缺口 `GAP-20260927-205` · 本枚）：`--on`/`--off` 在解析面刚上户口
	//   （`parseInvocation` 专用 case）⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）
	//   真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--force-recompute` 那条。
	for _, f := range []string{"--on", "--off"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（缺口 `GAP-20261010-20` · 2026-10-17）：`--fail-on-hit` 在解析面刚上户口
	//   （`parseInvocation` 专用 case）⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）真给它
	//   会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--on`/`--off` 那条：把不归本命令的那一枚**补回 `inv.unknown`**。
	if inv.hasFlag("--fail-on-hit") && !usageDeclaresFlag(cmd, "--fail-on-hit") {
		inv.unknown = append(inv.unknown, "--fail-on-hit")
	}
	if inv.hasFlag("--legacy") && !usageDeclaresFlag(cmd, "--legacy") {
		inv.unknown = append(inv.unknown, "--legacy")
	}
	// ★ 同一条数据驱动收口（缺口账 `GAP-20260927-259` · 本枚）：`--notes` 在解析面刚上户口
	//   （`valueFlagName`）⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）真给它
	//   会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字
	//   照拄上面 `--force-recompute` 那条：把不归本命令的那一枚**补回 `inv.unknown`** ⇒ 由下面
	//   **同一条**判词 + 同一个退码 2 收口。`gap add` / `gap idea promote` 的 usage 已列它 ⇒ 归它们。
	if inv.hasFlag("--notes") && !usageDeclaresFlag(cmd, "--notes") {
		inv.unknown = append(inv.unknown, "--notes")
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 GAP-20260927-234）：`--literal-text` 在解析面刚上户口（布尔 case）⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--notes` 那条：把不归本命令的那一枚补回 `inv.unknown` ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。`dev edit` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--literal-text") && !usageDeclaresFlag(cmd, "--literal-text") {
		inv.unknown = append(inv.unknown, "--literal-text")
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 GAP-20260927-355）：`--dist` 在解析面刚上户口（布尔 case）
	//   ⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」
	//   退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面
	//   `--literal-text` 那条；`dev build` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--dist") && !usageDeclaresFlag(cmd, "--dist") {
		inv.unknown = append(inv.unknown, "--dist")
	}
	// ★ 本枚（main.go 开窗第 6 条 · 缺口 GAP-20261012-27）：`--check-excludes-parity` 在解析面刚上户口
	//   （布尔 case）⇒ 全族共用；不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」
	//   退化成**静默吞**（硬约束⑤）。形状逐字照拄上面 `--dist` 那条。
	if inv.hasFlag("--check-excludes-parity") && !usageDeclaresFlag(cmd, "--check-excludes-parity") {
		inv.unknown = append(inv.unknown, "--check-excludes-parity")
	}
	// ★ Q-129：`--check`（裸形布尔）在解析面刚上户口 ⇒ **全族共用**；不在此收口，别族（usage
	//   未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**。★ 用**精确**判
	//   （`usageDeclaresExactFlag`）而不是 `usageDeclaresFlag`：`--check` 是
	//   `--check-excludes-parity` 的**前缀** ⇒ 子串判会让 `publish run --check` 被「证明」已登记、
	//   随后落进实现被静默吞。归 `update`（只读检查）。
	if inv.hasFlag("--check") && !usageDeclaresExactFlag(cmd, "--check") {
		inv.unknown = append(inv.unknown, "--check")
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 `GAP-20260929-28` 剩面）：`--uncertain` 在解析面刚上户口
	//   （布尔 case）⇒ 全族共用；若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」
	//   退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面
	//   `--check-excludes-parity` 那条；`gate find` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--uncertain") && !usageDeclaresFlag(cmd, "--uncertain") {
		inv.unknown = append(inv.unknown, "--uncertain")
	}
	// ★ 同一条数据驱动收口（批 B2 · 2026-10-20 · `zerg sched run` 归入命令面）：五枚旗标
	//   （`--workdir` / `--container` / `--agent-bin` / `--dev` / `--repo`）在解析面刚上户口
	//   （三枚 `valueFlagName` ＋ 两枚布尔 case）⇒ **全族共用**；若不在此收口，别族（usage 未列它的
	//   命令）真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--uncertain` 那条：把不归本命令的那一枚**补回 `inv.unknown`** ⇒ 由下面
	//   **同一条**判词 + 同一个退码 2 收口。`sched run` 的 usage 已列五枚 ⇒ 归它。
	for _, f := range []string{"--workdir", "--container", "--agent-bin", "--dev", "--repo"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（批 B4 · 2026-10-20 · `zerg api serve` 归入命令面）：两枚旗标
	//   （`--port` 取值 · 已在 `valueFlagName`；`--insecure` 布尔 · 上面刚上户口）⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它的命令）真给它们会从「未知旗标 2」退化成**静默吞**
	//   （硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 B2 那条：把不归本命令的
	//   那一枚**补回 `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	//   `api serve` 与 `context set-port` 的 usage 已列 `--port`；`api serve` 的 usage 已列
	//   `--insecure` ⇒ 归它们。
	for _, f := range []string{"--port", "--insecure"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（批 B5 · 2026-10-20 · `zerg agent run` 归入命令面）：十一枚旗标
	//   （九枚取值 —— `--task`/`--issue`/`--tools`/`--mcp`/`--max-turns`/`--retry`/`--gateway`/
	//   `--x3-status`/`--x3-token` 已在 `valueFlagName`；两枚布尔 —— `--auto-issue`/`--subtask`
	//   上面刚上户口）⇒ **全族共用**；若不在此收口，别族（usage 未列它们的命令）真给它们会从
	//   「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄
	//   上面 B4 那条：把不归本命令的那一枚**补回 `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个
	//   退码 2 收口。`agent run` 的 usage 已列这十一枚 ⇒ 归它。
	for _, f := range []string{"--task", "--issue", "--tools", "--mcp", "--max-turns", "--retry",
		"--gateway", "--x3-status", "--x3-token", "--auto-issue", "--subtask"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（批 B6 · 2026-10-20 · `zerg model record verify|probe|ls` 归入命令面）：
	//   十枚旗标（七枚取值 —— `--base-dir`/`--store-root`/`--endpoint`/`--engine`/`--id`/`--parent`/
	//   `--manifests` 已在 `valueFlagName`；三枚布尔 —— `--integrity`/`--in-place`/`--store` 上面刚上户口）
	//   ⇒ **全族共用**；若不在此收口，别族（usage 未列它们的命令）真给它们会从「未知旗标 2」退化成
	//   **静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 B5 那条：把不归本命令的
	//   那一枚**补回 `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	//   ★ 收口用的是 `usageDeclaresFlag`（**子串**判）⇒ 同族里互为前缀的两枚（`--base` / `--base-dir`）
	//   会互相「证明」对方已登记；`--strict` 又不在 `inv.kv` 里（它是 `inv.strict` 这个专用字段）⇒
	//   这两处由实现件入口的**精确**点名兜住（`family_model_record.go:modelRecordRejectForeign`）。
	for _, f := range []string{"--base-dir", "--store-root", "--endpoint", "--engine", "--id", "--parent",
		"--manifests", "--integrity", "--in-place", "--store"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（批 B7 · 2026-10-20 · `zerg compat list|check|migrate|manifest` 归入命令面）：
	//   两枚取值旗标（`--state-dir` / `--receipts-dir`）在 `valueFlagName` 上刚上户口 ⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它们的命令）真给它们会从「未知旗标 2」退化成**静默吞**
	//   （硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 B6 那条：把不归本命令的
	//   那一枚**补回 `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	//   ★ 收口用的是 `usageDeclaresFlag`（**子串**判）⇒ `--state` 会替 `--state-dir` 作证 ⇒ 该面由
	//   实现件入口的**精确**点名兜住（`family_compat.go:compatFlagAllowed`）。
	for _, f := range []string{"--state-dir", "--receipts-dir"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 GAP-20260928-139）：`--build-info` 在解析面刚上户口（布尔 case）⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--literal-text` 那条：把不归本命令的那一枚补回 `inv.unknown` ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。`version` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--build-info") && !usageDeclaresFlag(cmd, "--build-info") {
		inv.unknown = append(inv.unknown, "--build-info")
	}
	// ★ 本单（C1 · 虫茧生命周期 · 2026-10-20）：`--purge` 在解析面刚上户口（布尔 case）⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**
	//   （硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 `--build-info` 那条：
	//   把不归本命令的那一枚补回 `inv.unknown` ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	//   `cocoon close` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--purge") && !usageDeclaresFlag(cmd, "--purge") {
		inv.unknown = append(inv.unknown, "--purge")
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 GAP-20260927-240）：`--list-selfcases` 在解析面刚上户口（布尔 case）⇒ **全族共用**；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。
	//   形状逐字照拄上面 `--build-info` 那条：把不归本命令的那一枚补回 `inv.unknown` ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。`gate check` 的 usage 已列它 ⇒ 归它。
	if inv.hasFlag("--list-selfcases") && !usageDeclaresFlag(cmd, "--list-selfcases") {
		inv.unknown = append(inv.unknown, "--list-selfcases")
	}
	// ★ 同一条数据驱动收口（本单 · 缺口 GAP-20260928-65 的权威判据半）：`--from-file` 在解析面刚上户口
	//   （`valueFlagName`）⇒ **全族共用**；若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」
	//   退化成**静默吞**（硬约束⑤：既有命令的退码与输出一字不动）。形状逐字照拄上面 `--list-selfcases`
	//   那条：把不归本命令的那一枚补回 `inv.unknown` ⇒ 由下面**同一条**判词 + 同一个退码 2 收口。
	//   `dev proposal new` 的 usage 已列它 ⇒ 归它（既有命令一字不动）。
	if inv.hasFlag("--from-file") && !usageDeclaresFlag(cmd, "--from-file") {
		inv.unknown = append(inv.unknown, "--from-file")
	}
	// ★ 本枚（main.go 开窗第 5 条）：`--add-file` 在解析面刚上户口（`valueFlagName`）⇒ 同族共用；
	//   若不在此收口，别族（usage 未列它的命令）真给它会从「未知旗标 2」退化成**静默吞**（硬约束⑤）。
	//   形状逐字照拄上面 `--from-file` 那条：把不归本命令的那一枚补回 `inv.unknown`。
	if inv.hasFlag("--add-file") && !usageDeclaresFlag(cmd, "--add-file") {
		inv.unknown = append(inv.unknown, "--add-file")
	}
	// ★ 本单（缺失项 `GAP-20260927-172` 之正解 · E1 `fleet render`）：`--preview-only`/`--emit`/
	//   `--expect-no-changes`（布尔）与 `--roster`（取值）在解析面刚上户口 ⇒ **全族共用**；不在此收口，
	//   别族（usage 未列它的命令）真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤）。形状逐字照拄
	//   上面 `--add-file` 那条：把不归本命令的那一枚**补回 `inv.unknown`**。`fleet render` 的 usage 已列它们。
	for _, f := range []string{"--preview-only", "--emit", "--expect-no-changes", "--roster"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 本单（T3 · `zerg chat` 族 · 2026-10-17）：本族八枚旗标在解析面刚上户口（三枚取值进
	//   `valueFlagName` · 五枚布尔各一条专用 `case`）⇒ **全族共用**；若不在此收口，别族（usage
	//   未列它们的命令）真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码
	//   与输出**一字不动**）。形状逐字照拄上面 `--roster` 那条：把不归本命令的那一枚**补回
	//   `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口（不另写判词 · 不另取码）。
	//   `chat` 族 7 条命令的 usage 已按需列出它们 ⇒ 归它们（既有命令一字不动）。
	for _, f := range []string{"--session", "--images", "--window", "--new", "--archived",
		"--no-stream", "--markdown", "--last-in-cwd",
		// ★ 2026-10-17（本枚）：`chat ask` / `chat send` 机器面 —— 两枚取值（`--prompt-file` / `--as`）
		//   与一枚布尔（`--stream`）同样「全族共用」⇒ 不在此收口，别族真给它们会从「未知旗标 2」
		//   退化成**静默吞**（硬约束⑤）。归 `chat send` 与 `chat ask` 两条（usage 已列它们）。
		"--prompt-file", "--as", "--stream"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	// ★ 本单（缺口 `GAP-20261017-10` · `fs` 族三条正门）：五枚旗标在解析面刚上户口（三枚取值进
	//   `valueFlagName` · 两枚布尔各一条专用 `case`）⇒ **全族共用**；若不在此收口，别族（usage
	//   未列它们的命令）真给它们会从「未知旗标 2」退化成**静默吞**（硬约束⑤：既有命令的退码
	//   与输出**一字不动**）。形状逐字照拄上面 `--roster` 那条：把不归本命令的那一枚**补回
	//   `inv.unknown`** ⇒ 由下面**同一条**判词 + 同一个退码 2 收口（不另写判词 · 不另取码）。
	//   `fs read` / `fs write` / `fs run` 三条的 usage 已按需列出它们 ⇒ 归它们。
	for _, f := range []string{"--offset", "--content", "--content-stdin", "--append", "--cwd"} {
		if inv.hasFlag(f) && !usageDeclaresFlag(cmd, f) {
			inv.unknown = append(inv.unknown, f)
		}
	}
	if len(inv.unknown) > 0 && !cmd.passthrough {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, inv.unknown[0])
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	// ★ 2026-09-24（缺口 `Q-154` · 组A 同族）：**多余位置参数 ⇒ 用法错 2**。
	//   病灶（修前现读实据）：`zerg version extraarg` 退 `0` —— 多余的那一枚被**静默吞**掉，
	//   「给错了」被读成「给对了」。与 `Q-141`（`--help` 两态）同族：命令面不许把两种不同的
	//   真值并成同一个形状（§4.1 K14 四件套 · `exitcodes.go` 的用法错 = 2）。
	//   判据面（三条边界写死，逐条可核 —— 不是「所有命令一把抓」）：
	//     · `cmd.passthrough`（`gate` 族透传型）**不判**：位置参数逐字交给被包的脚本（§6.3 S3）；
	//     · `cmd.danger != nil`（危险档）**不判**：它们的目标语义在 `guard.go` 一处
	//       （`inv.args[0]` 可当目标用），且确认档缺失时本就 fail-closed 退 `2` ⇒ 本批不动（照实登记）；
	//     · 其余：`declaresNoPositional` 为真（`args` 为空 / 逐格都是**注记**：全角括号开头）⇒ 该条声明不收位置参数。
	if !cmd.passthrough && cmd.danger == nil && len(inv.args) > 0 && declaresNoPositional(cmd) {
		fmt.Fprintf(stderr, "%s: 多余位置参数 %q（`%s %s` 不收位置参数）\n",
			progName, inv.args[0], progName, strings.Join(cmd.path, " "))
		fmt.Fprintf(stderr, "See '%s %s --help'。\n", progName, strings.Join(cmd.path, " "))
		return exitUsage
	}
	// 没有机器面字段表的**说明面** ⇒ 不认 --json（用法错 2，不是 1）：1 是「给了 --json 但没给字段」
	// 那一档的码（§4.1 K2），两者不许混。
	// 例外：危险动作**没有结果面**（只有 `--dry-run` 的计划件），但它们的**错误面**必须机器可读
	// （§九 M7：AI 自愈靠 kind）⇒ 危险动作收 `--json`，只当错误面用。
	if inv.jsonGiven && len(cmd.fields) == 0 && cmd.danger == nil {
		fmt.Fprintf(stderr, "%s: `%s %s` 没有 `--json` 字段表（它是说明面）\n", progName, progName, strings.Join(cmd.path, " "))
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	// 契约主号校验（§十二 P-026：消费侧拒绝**不认的 schema 主号**，不许静默降级）。
	if inv.schemaGiven {
		major, _, ok := parseContractID(inv.schemaWant)
		known := false
		for _, m := range recognizedMajors() {
			if ok && m == major {
				known = true
			}
		}
		if !known {
			msg := fmt.Sprintf("认不得的契约主号 %q —— 本版只认 %s（§九 M6 · §十二 P-026）", inv.schemaWant, schemaMajorSet())
			inv.setErr("usage", "bad_schema_major", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "不认的主号**不许静默降级**成 v%d（§九 M15「不兼容即明确报错」）\n", contractMajor)
			fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
			emitErrIfJSON(inv, stdout, cmd, exitUsage)
			return exitUsage
		}
	}
	// 对象级作用域（§九 M13）：`Z1`/`Z3`/`P-066`/`P-067` 三条在**执行之前**判。
	if rc, done := enforceNodeRules(inv, cmd, stderr); done {
		emitErrIfJSON(inv, stdout, cmd, rc)
		return rc
	}
	// 非交互凭据（`C2`）：本轮的 `--token-stdin` 交给**唯一入口**去读（只读一次）。
	if inv.tokenStdin {
		stdinTokenWanted = true
	}
	cw := &countingWriter{w: stdout}
	// ★ 2026-09-24（序138 · 组4 §二.4 `W-57`（`研-禁:216` §六 栗④）· 上级裁定 **B**）：
	//   `--timeout <时长>` 从「收得下、不生效」接上**消费者**（`wallclock.go`）。
	//   两条判据栏要点落在这里：① **非法时长 ⇒ 先于任何动作退 `2`**（在 `cmd.run` 之前判 ——
	//   「先于任何动作」就是这一行的位置）；② 合法 ⇒ 该命令**整趟**带上墙钟上界，
	//   到点 ⇒ **先复原再报** ⇒ 退 `11`（「等不起」与「等到了坏结果」异码）。
	//   旗标是**全局**的（谁用谁读 · 不用的命令不给就不生效）—— 与 `--all` / `--fast` 同一形态。
	// ★ 缺口 `GAP-20260927-75`：gap 族入口三档环境旋钮处理（夹具口**当场**清）+ 状态目录可视输出。
	//   ★ 申报行**攒着、命令跑完后追加到 stderr 尾**（✗ 不顶掉 gap 族 stderr 的**首行判词** ——
	//   `cli_gap_reason_test` / `cli_gap_test` 逐字钉住首行；见 `family_gap_env.go` 件头两条纪律）。
	var gapEnvTail []byte
	if len(cmd.path) > 0 && cmd.path[0] == "gap" {
		gapEnvTail = prepGapFamilyEnv()
	} else if !cmd.passthrough {
		// ★ b1 全局推广（2026-10-10）：**非 gap 族**也在咽喉点做 ④「非默认根来源点名」——
		//   只申报 · **默认态零输出**（三项都走默认根 ⇒ 返回 nil）· 不动退码/判定。
		//   与 gap 族共用**同一取值口**（`gapNonDefaultRootLine`）⇒ ✗ 不另造第二套。
		//   ★ 透传型（`cmd.passthrough` · 如 `gate` 族）**不掺**：它们的 stderr 逐字是
		//   被包脚本的输出（`gate show` 默认面 = `--emit-cmd` 直取口 · 契约钉「一字不改」）。
		gapEnvTail = gapNonDefaultRootLine()
	}
	// ★ 本枚（P1 打样 · 件 `Q-150` · 2026-10-22）：`--json <字段>` 的**字段名**在**唯一咽喉点**核 ——
	//   位置写死：`cmd.run` **之前**（用法错必须**零副作用**：`GAP-20261020-01` 那条纪律 =
	//   「先核字段再动手」；`gate run --step … --json nope` 修前要**跑完整步**才在出口被拒）。
	//
	//   病（修前现读实据 · 两类，各自可复现）：
	//     ① **绕开出口的自出包封路**根本不过 `validateSelected` ⇒ 坏字段被**静默吞**：
	//        `zerg gap ls --grep zzzz --json nope` 修前 rc=1 + 314 B 包封（字段名一个字节不提）；
	//        `zerg apply --json nope` 修前 rc=0 · 535 B；`zerg core logs --json nope` 修前 rc=8 · 450 B；
	//        `zerg gap assign ls --json nope` 修前 rc=1 · 486 B；`zerg publish accept --json nope`
	//        修前干脆**挂住**（30 s 不返回）。自出包封的点位：`family_gap.go` 六处零命中/超时快速路
	//        · `family_gap_diff.go` · `guard.go` 的幂等重跑路（`family_code_constdup.go` 自拼对象但
	//        **先核**字段 ⇒ 不在病灶清单里）。
	//     ② **迟到**：走 `emitSelected` 的那些（`gate run` 单步档就是：**跑完整步**、建日志目录、
	//        读回 results.tsv 之后才在出口核）⇒ 退码对但代价白付、状态可能已改。
	//   一处（本格）修好 ⇒ 全部有字段表的命令同受益（修前实测：290 条里 223 条有字段表；
	//   无表的 67 条照旧走下面「说明面」那条判词，行为一字不动）。
	//
	//   口径（三格写死，✗ 不许放宽）：
	//     ① 只在 `cmd.fields` 非空（该条**有机器面字段表**）时判 —— 没表的说明面/危险档错误面照旧；
	//     ② 收受集 = `fields ∪ optFields`（与 `validateSelected` **同一条白名单**：可选字段层只增）⇒
	//        **合法字段一个不误伤**（合法面的输出字节由下面「两态对拍」逐字节钉住）；
	//     ③ 坏名 ⇒ `reportBadField`（**同一句判词 + 同一张表**，与出口那条逐字同形）+ `exitUsage`，
	//        stdout 0 字节（K2 的口径：用法错不是结果面）。
	if inv.jsonGiven && len(inv.fields) > 0 && len(cmd.fields) > 0 {
		if rc := rejectUnknownJSONFields(inv, cmd, stderr); rc != exitOK {
			// ★ 尾行（`gap` 族的 ①②③④ 申报 / 非 gap 族的④点名）照旧补上 —— 既有命令在
			//   非默认 env 下的 stderr 因此**逐字不动**（默认态本来一字不加 ⇒ 两态全同）。
			if len(gapEnvTail) > 0 {
				_, _ = stderr.Write(gapEnvTail)
			}
			return rc
		}
	}
	var rc int
	if inv.hasFlag(wallclockFlag) {
		d, werr := parseWallclock(inv.flagVal(wallclockFlag))
		if werr != nil {
			inv.setErr("usage", "bad_wallclock", werr.Error())
			fmt.Fprintf(stderr, "%s: %v\n", progName, werr)
			fmt.Fprintf(stderr, "See '%s %s --help'。\n", progName, strings.Join(cmd.path, " "))
			emitErrIfJSON(inv, stdout, cmd, exitUsage)
			return exitUsage
		}
		wallclockJournalReset()
		rc = wallclockGuard(d, func() int { return cmd.run(inv, cw, stderr) }, stderr)
		if rc == exitTimeout && inv.err == nil {
			inv.err = wallclockTimeoutError(d)
		}
	} else {
		rc = cmd.run(inv, cw, stderr)
	}
	// `--json <字段>` 的失败路径：把**机器可读**的 `error` 块挂进包封（§九 M7）——
	// 只在命令自己没往 stdout 写结果时补（写了结果就不改它，避免两个面打架）。
	// ★ 缺口 `GAP-20260929-41`：`kind=usage` 的失败**不补 stdout 包封** —— 用法错不是
	//   结果面（与 K2 的 0 字节档同口径：人读判词 + 字段表已全在 stderr，退码 2 为准），
	//   `stdout` 留零字节；其余 kind（失败侧 error 包封，§九 M7）照旧补。
	if rc != exitOK && inv.jsonGiven && cw.n == 0 && (inv.err == nil || inv.err.Kind != "usage") &&
		(inv.err != nil || len(inv.fields) > 0 || cmd.danger != nil) {
		emitErrIfJSON(inv, stdout, cmd, rc)
	}
	// ★ 缺口 `GAP-20260927-75`：gap 族环境旋钮申报行 —— **追加到 stderr 尾**（✗ 不顶掉首行判词）。
	if len(gapEnvTail) > 0 {
		_, _ = stderr.Write(gapEnvTail)
	}
	return rc
}

// countingWriter 记下命令往 stdout 写了多少字节（失败时决定能不能补错误包封）。
type countingWriter struct {
	w io.Writer
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += n
	return n, err
}

// emitErrIfJSON 在 `--json <字段>` 的失败路径上补错误包封（没 fields 且不是危险动作就不补 ——
// 那是 K2 的 0 字节档；危险动作没有结果面，`--json` 在它上面只作错误面）。
//
// `rc` = **本次进程真退码**（`run` 的一条路径各自把它传进来）。它只用在「没人报过 kind」那一格：
// 包封里的 `error.exit_code` 是 `kind` 的投影（`errors.go` 的 `codeOfKind` = 唯一真源，`E1`），
// 而兜底的 kind 必须**从本次真退码派生** —— 见下面 `Q-155` 那一段。
func emitErrIfJSON(inv *invocation, stdout io.Writer, cmd *command, rc int) {
	if !inv.jsonGiven {
		return
	}
	if len(inv.fields) == 0 && inv.err == nil && (cmd == nil || cmd.danger == nil) {
		return
	}
	e := inv.err
	if e == nil {
		// ★ 2026-09-24（缺口 `Q-155`）：兜底 kind **不许写死 `exitFail`**。
		//   病灶（修前现读实据）：`./bin/zerg version --json bogusfield` 退 `2`（用法错），
		//   而包封内 `error.exit_code` 写 `1` —— **两个码面打架**（调用方按包封读会以为是一般失败）。
		//   治法：兜底 kind 由**本次真退码**派生（`kindForExitCode(rc)`）⇒ `codeOfKind` 反出来
		//   与 `rc` 同源（现读覆盖 `1/2/4/8/10/11/12/14/130` 全部单值码）。
		//   **不新造机制**：不新增顶层键、不新增 kind、不改 `codeOfKind` 的「kind → 退码」单向口径；
		//   报出过 kind 的那条路一字不动（谁先报谁为准 —— 与「不打第二枪」同一条纪律）。
		//   同族先例：`family_script_inventory.go:452–461` 那一格的注释说的就是这个病（它靠**不报 kind**
		//   绕开 ⇒ 兜底路那一格的退码是 `1`，与本改法同值、行为不变）。
		e = &cliError{Kind: kindForExitCode(rc), Message: "（命令未报出 kind，按退码兜底）"}
	}
	emitErrEnvelope(stdout, cmd, e, inv.envMeta)
}

// declaresNoPositional 判「该条命令**声明不收位置参数**」（`Q-154` 那条纪律的判据口）。
//
// ★ 2026-09-24（块B `R-7`）：判据口从**散文启发式**改成读**声明式**的一格 `arity`
// —— 原来是「`args` 逐格以**全角括号** `（` 开头」。两格的关系一行写死：
//
//	`arity == "any"` ⇒ 该条声明**收**位置参数；其余（`"none"` / 空串）⇒ 声明**不收**。
//
// 为什么换（三条，逐条可核）：
//
//	① 启发式是**猜**：`args` 那几格语义是「位置参数的说明」（人读的散文），今天凑巧
//	   「不收」的写法恰好全是全角括号开头；换个人写半角 `(无位置参数)` 就当场判反。
//	② 交叉核对（`cobra` `Args PositionalArgs` / `NoArgs` / `ExactArgs` / `MinimumNArgs` /
//	   `MaximumNArgs`）：arity 是**声明式验证器**，不从帮助文本反推。
//	③ **订正一句**（原注释引 §九 M6 `R7`「字段只增不改」当「不动 `command` 字段面」的理由，
//	   **是过度解读** ✗）：`R7` 逐字是「只增不改」⇒ **增是允许的**；且 `R7` 管的是 `zerg/v1`
//	   JSON 面，`command` 是**内部结构体** ⇒ 两个面不同。
//
// 档位闭集 = `none` / `exact(n)` / `min(n)` / `max(n)` / `any`（照 `cobra` 那五档）。
// **本版只用 `none` / `any` 两档**（逐条声明在命令树字面量里）；`exact(n)` / `min(n)` /
// `max(n)` 要**逐个命令数真实基数**，而现读 `args` 那几格是散文、数不出来 ⇒ **照实登记为
// 未做**，不拿散文猜一个 n 填进去 ✗。
//
// 判错的代价仍是**单向**的（只有真声明了位置参数的命令被误判成「不收」才会误退 2）：
// 现读反查过全部命令的「读 `inv.args` 的 handler ↔ 是否声明位置参数」，只剩 5 条不一致，
// 其中 4 条**自己就拒**（`api help` / `script inventory sync` / `itask start` /
// `core reload`），第 5 条（`gateway breakers` 的 target）在危险档 ⇒ 本函数**不覆盖它们**
// （`cmd.danger != nil` 一律不判 · 见 `run` 里那三条边界）。
// ★ 2026-10-05：`itask stop` 已开放写面（`cmdItaskStopGuarded` · 不再读 `inv.args`）⇒ 从上表移出。
func declaresNoPositional(cmd *command) bool {
	if cmd == nil {
		return false
	}
	return cmd.arity != arityAny
}

// ---- 命令树（真源：帮助文本、markdown 导出、别名解析都从这里出，不许旁写一份）----

type command struct {
	path    []string
	summary string
	usage   string
	fields  []string // --json 可取的全部字段（不给字段时 stderr 列的就是它）
	// optFields —— `--json` **可选**字段层（登在出口白名单里、点得出来，但**不进** `fields`
	// 那一份文档/帮助导出面字段表 —— 与 `approveOptionalFields` 同口径：冻结表不动、可选只增）。
	optFields []string
	args      []string // 位置参数的说明（帮助里逐条列出 · **只给人读** · 不再当判据口）
	arity     string   // 位置参数元数（声明式 · `R-7`）：闭集 none/exact(n)/min(n)/max(n)/any；本版只用 none/any（空串视作 none）
	endpoint  string   // 它投影的远端端点（本机命令为空）；T-08 的三面同源对账用
	kind      string   // 包封里的 kind（§九 M6 I2：与命令一一对应 · 单数 CamelCase）
	danger    *dangerSpec
	idem      *idemSpec // M4 幂等四字段（没显式写的在 seedIdem 里按规则派生）
	// 群级只读（§十二 P-066）：没有目标时「读全群」是允许的；**其余命令无目标 ⇒ exit 2**。
	groupReadOnly bool

	// machine —— 「机器目标名」（缺口 GAP-20260927-424 · 机读闭集见 remote.go 的 machineTargets）：
	// 本命令的对象是**某台机**时填那台的机器目标名（machineNodeName / machineHostName /
	// machineGatewayHost）；空串 ⇒ 对象不是机器。
	// ★ 与 `dangerSpec.Target`（人面**显示串**）**分字段**：显示串可自由措辞（含「本机名」
	// 「（落点目录）」一类显示修饰），机器判定只看本字段。此前二者同源，改显示串会连带翻转
	// needsMachine（`dev edit` 被迫用第三值「本机名」绕开的根因）。
	machine string
	// 茧壁层级标记（§九 M10 `X1`：闭集 host/node/space；没显式写的按族派生）
	layer  string
	opened bool // 本版是否可执行（危险动作**逐条标**：有真实现的为 true ⇒ 帮助面不许一律写「未开放」）
	// refuses —— **真跑一律拒执**（非危险档专用，缺口 序33）：`danger == nil` 的命令默认开放，
	// 但少数几条**今天真跑就是不给结论**（`egg run` 退 8 · `cocoon open` / `build release` / `apply` 退 2）
	// ⇒ 它们在帮助面/导出面**不算「已开放」**（口径 = `openedForRun`，见该函数）。
	refuses     bool
	passthrough bool // 原样透传型（gate 族）：旗标与位置参数逐字交给被包的脚本
	// childHelp：透传型里的**子件帮助透传档** —— 给了 `-h`（短形）且已点名子件目标时，
	// `-h` 交给子件印它自己的 usage；`--help`（长形）**永远**出该条命令的族帮助。
	// 缺省 `false` = 现行行为（`-h` 与 `--help` 一样出族帮助）。★ 只给「`-h` 本身就是纯帮助」
	// 的转发壳开（`gate check`）—— 跑真门/真动作的命令（`gate run` 等）不许开：
	// `-h` 漏进子件 = 把「问帮助」变成「真干一次」。
	childHelp bool
	run       func(*invocation, io.Writer, io.Writer) int
}

// openedForRun —— 这条命令**今天真跑能不能执行**（帮助面 / 导出面「已开放」计数的**唯一**口径）。
//
// 口径（2026-09-24 · 缺口 序33 · Mr2109 拍）：`opened && !拒执` —— 逐条分两支：
//
//	① 危险档（`danger != nil`）：看**逐条**的 `opened` 标记（有真实现的为 true；没实现的真跑拒执）；
//	② 非危险档（`danger == nil`）：默认开放，**但声明了拒执**的（`refuses`）不算。
//
// 修前病根（照实现读出来的）：计数只写 `danger == nil` ⇒ 把「非危险档」当成「已开放」的**代理**，
// 于是一条真跑拒执的命令**两头占**：既进了「已开放」的条数、又不进任何危险档 ⇒ **双计**。
// 现读实据（修前）：`zerg help egg` 报「动作 6 条 · 已开放 3 · 危险档 3」—— 6 = 3 + 3 看着自洽，
// 而 `egg run` 真跑退 8（`cmdEggRun`：卵写面本波不做）⇒ 它本不该算「已开放」。
// ⇒ 计数必须**三档各归各的**：已开放 / 拒执 / 危险档，三者之和 = 动作数（`renderFamilyHelp` 逐数报）。
func openedForRun(c *command) bool {
	if c.danger != nil {
		return c.opened
	}
	return !c.refuses
}

// `arity` 的闭集（照 `cobra` `Args` 验证器的五档）：本版只用下面**两档**。
// `exact(n)` / `min(n)` / `max(n)` 三档要逐个命令数真实基数 ⇒ 照实登记为未做（见 `declaresNoPositional`）。
const (
	arityNone = "none" // 声明**不收**位置参数（多余位置参数 ⇒ 用法错 2）
	arityAny  = "any"  // 声明**收**位置参数（本版不数基数，照旧放行）
)

// commands —— 批 A（S1 起）登记的只读面；后续各票在此续行。
// 用 init() 而不是包级字面量：命令树的 handler 又回头读 `commands`（帮助渲染），
// 写成包级字面量会被编译器判成初始化环。
var commands []*command

func init() {
	// 命令表**按族拆件**（缺口 `GAP-20261011-03`）：每族一件 `cmdtable_<族>.go`，本函数只做装配。
	// 装配顺序 = 各族在旧字面量里**首次出现**的顺序（与拆分前逐条顺序的可见行为一致）。
	commands = []*command{}
	commands = append(commands, cmdtable_version()...)
	commands = append(commands, cmdtable_root()...)
	commands = append(commands, cmdtable_help()...)
	commands = append(commands, cmdtable_doctor()...)
	commands = append(commands, cmdtable_context()...)
	commands = append(commands, cmdtable_api()...)
	commands = append(commands, cmdtable_agent()...)
	commands = append(commands, cmdtable_task()...)
	commands = append(commands, cmdtable_model()...)
	commands = append(commands, cmdtable_gate()...)
	commands = append(commands, cmdtable_watch()...)
	commands = append(commands, cmdtable_core()...)
	commands = append(commands, cmdtable_resource()...)
	commands = append(commands, cmdtable_gateway()...)
	commands = append(commands, cmdtable_script()...)
	commands = append(commands, cmdtable_find()...)
	commands = append(commands, cmdtable_ls_face()...)
	commands = append(commands, cmdtable_code()...)
	commands = append(commands, cmdtable_registry()...)
	commands = append(commands, cmdtable_repo()...)
	commands = append(commands, cmdtable_audit()...)
	commands = append(commands, cmdtable_ui()...)
	commands = append(commands, cmdtable_doc()...)
	commands = append(commands, cmdtable_port()...)
	commands = append(commands, cmdtable_net()...)
	commands = append(commands, cmdtable_publish()...)
	commands = append(commands, cmdtable_ci()...)
	commands = append(commands, cmdtable_route()...)
	commands = append(commands, cmdtable_egg()...)
	commands = append(commands, cmdtable_cocoon()...)
	commands = append(commands, cmdtable_itask()...)
	commands = append(commands, cmdtable_build()...)
	commands = append(commands, cmdtable_sched()...)
	commands = append(commands, cmdtable_dev()...)
	commands = append(commands, cmdtable_update()...)
	commands = append(commands, cmdtable_approve()...)
	commands = append(commands, cmdtable_propose()...)
	commands = append(commands, cmdtable_plugin()...)
	commands = append(commands, cmdtable_chat()...)
	commands = append(commands, cmdtable_ask()...)
	commands = append(commands, cmdtable_plan()...)
	commands = append(commands, cmdtable_apply()...)
	commands = append(commands, cmdtable_isolate_ro()...)
	commands = append(commands, cmdtable_swap()...)
	commands = append(commands, cmdtable_deploy()...)
	commands = append(commands, cmdtable_calib()...)
	commands = append(commands, cmdtable_eval()...)
	commands = append(commands, cmdtable_impact()...)
	commands = append(commands, cmdtable_archive()...)
	commands = append(commands, cmdtable_compat()...)
	commands = append(commands, cmdtable_config()...)
	commands = append(commands, cmdtable_gap()...)
	commands = append(commands, cmdtable_metrics()...)
	commands = append(commands, cmdtable_trace()...)
	commands = append(commands, cmdtable_review()...)
	commands = append(commands, cmdtable_sandbox()...)
	commands = append(commands, cmdtable_wall()...)
	commands = append(commands, cmdtable_fleet()...)
	commands = append(commands, cmdtable_mcp()...)
	commands = append(commands, cmdtable_fs()...)
	commands = append(commands, cmdtable_skill()...)
	// 群级只读（§十二 `P-066`）：这些命令「无目标 = 读全群」是**定义**，不是遗漏。
	for _, c := range commands {
		switch strings.Join(c.path, " ") {
		case "version", "help", "help export", "doctor", "context ls", "api ls", "api openapi",
			"api help", "agent ls", "task ls", "model ls", "gate ls", "gate self-test":
			c.groupReadOnly = true
		}
	}
	// 茧壁层级标记（§九 M10 `X1`）：与四字段同一处补齐。
	seedLayer()
	// M4 四字段：命令树建完立刻补齐（每条命令都有四格 · 一条不漏）。
	seedIdem()
}

// resolve 按「最多 3 段」贪心匹配（`zerg help` 第 1 行的「深度 ≤ 3 层」）：先试 [p0,p1,p2]，
// 再试 [p0,p1]，最后试 [p0]；剩下的是位置参数。
func resolve(pos []string) (*command, []string) {
	// ★ 2026-09-23 `A3` 落 `script inventory sync`（3 段）时现读到的**命令面缺口**：
	//   本函数原来只试 `pos[:2]` 与 `pos[:1]` —— 而 `zerg help` 第 1 行**逐字承诺**
	//   「形态 `zerg <对象> <动作> [参数] [旗标]` · **深度 ≤ 3 层**」⇒ 3 段命令**根本派发不到**
	//   （现读全树 0 条 3 段命令 ⇒ 这条缺口此前没人撞到）。本处**只加一段更长的前缀试探**：
	//   既有 1/2 段命令的解析结果**一字未动**（3 段没命中时照旧落到下面两支）。
	if len(pos) >= 3 {
		if c := find(pos[:3]); c != nil {
			return c, pos[3:]
		}
	}
	if len(pos) >= 2 {
		if c := find(pos[:2]); c != nil {
			return c, pos[2:]
		}
	}
	if len(pos) >= 1 {
		if c := find(pos[:1]); c != nil {
			return c, pos[1:]
		}
	}
	return nil, nil
}

func find(path []string) *command {
	for _, c := range commands {
		if len(c.path) != len(path) {
			continue
		}
		ok := true
		for i := range path {
			if c.path[i] != path[i] {
				ok = false
				break
			}
		}
		if ok {
			return c
		}
	}
	return nil
}

// nearest 给一个「最像的合法输入」（K14 第三件）。前缀命中优先，其次同首字母。
func nearest(word string) string {
	var same []string
	for _, c := range commands {
		head := c.path[0]
		if strings.HasPrefix(head, word) || strings.HasPrefix(word, head) {
			return strings.Join(c.path, " ")
		}
		same = append(same, strings.Join(c.path, " "))
	}
	sort.Strings(same)
	if len(same) > 0 {
		return same[0]
	}
	return ""
}

// isFamilyNode —— 首词是不是命令树里的**真族名/中间节点**（直查命令树，✗ 不依赖 `nearest` 的回串）。
//
// 为什么单开一条（缺口 `GAP-20260927-400` 的真 bug）：`nearest` 是「最像的合法输入」——它按
// 命令树**登记序**取第一个前缀命中（`HasPrefix(head,word) || HasPrefix(word,head)`），而族名与
// 另一条共享前缀的命令并存时，先命中的可能是**别的**名字（`doc` 撞早登记的 `doctor`、`gateway`
// 撞早登记的 `gate`）。若拿它的回串反推「首词是不是族名」（旧判据
// `s == inv.path[0] || HasPrefix(s, inv.path[0]+" ")`），两族会被判成**假** ⇒ 首行错成「未知命令」。
// 判据直查命令树：**存在一条命令，其首段 == word** ⇒ word 就是族名/中间节点（命令树下有它的孩子）。
func isFamilyNode(word string) bool {
	for _, c := range commands {
		if len(c.path) > 0 && c.path[0] == word {
			return true
		}
	}
	return false
}

// childActions —— 某节点（族名或中间节点）下的**直接**动作名（命令树现算 · 只读）。
// 供「族名/中间节点不吃 --help」那一块的 stderr 指路用（缺口 `GAP-20260926-138`）。
func childActions(prefix []string) []string {
	if len(prefix) == 0 {
		return nil
	}
	head := strings.Join(prefix, " ")
	var out []string
	for _, c := range catalog() {
		if len(c.path) != len(prefix)+1 {
			continue
		}
		if strings.Join(c.path[:len(prefix)], " ") == head {
			out = append(out, c.path[len(prefix)])
		}
	}
	return out
}

// usageDeclaresFlag —— 该命令的 usage 串里写没写这枚旗标（数据驱动旗标的**同源**声明面）。
// 供 `--top` 收口用（缺口 `GAP-20260928-185`：把逐命令硬枚举改成按 usage 声明统一收 —— usage 是
// 命令树里「这条命令认哪些旗标」的**同一个**声明面，与 `cmdUsageHelp` 渲染的面同源）。
func usageDeclaresFlag(c *command, flag string) bool {
	if c == nil {
		return false
	}
	return strings.Contains(c.usage, flag)
}

// emitHelpFamilies —— `help --json` 的机器面：逐族一行（`family` = 族名 · `actions` = 该族动作数）。
// 现算自命令树（`catalog()` · 名字序稳定 ⇒ 一个数都不写死）；总族数进包封 `meta.count`
// （缺口 `GAP-20260927-290` 的「条数可机读」）。字段投影走 `inv.fields`（`requireFields` 已确保非空）。
func emitHelpFamilies(inv *invocation, stdout, stderr io.Writer) int {
	var order []string
	cnt := map[string]int{}
	for _, c := range catalog() {
		if len(c.path) == 0 {
			continue
		}
		if _, ok := cnt[c.path[0]]; !ok {
			order = append(order, c.path[0])
		}
		cnt[c.path[0]]++
	}
	rows := make([]map[string]string, 0, len(order))
	for _, f := range order {
		rows = append(rows, map[string]string{"family": f, "actions": strconv.Itoa(cnt[f])})
	}
	return selectJSONList(stdout, stderr, inv, []string{"help"}, inv.fields, rows)
}

// emitHelpFamilyActions —— `zerg help <族> --json` 的机器面（缺口 `GAP-20260927-335` · 2026-10-10 本枚）：
// 逐**动作**一行 —— 答「某动作是不是危险档 / 档位几 / 开没开」，此前这两格只能肉眼从
// `zerg help <族>` 人面数 `[危险档 D2]`（本条病灶「`gap export` 真写盘却判安全档」就是这么才被发现的）。
//
// 口径（只增机器面 · ✗ 不改任何判定语义 · ✗ 不动任何冻结帮助面）：
//
//	· 行来源 = `familyMembers(族)`（命令树 `catalog()` 是唯一真源 · 名字序稳定）；
//	· `command` = `zerg <路径>`（全行都有；用来指认是哪一条）；
//	· `danger`  = `dangerSpec.Level`（`D2`/`D3`）—— **只有危险档那一行才出这一格**；安全档本行**缺该键**
//	  ⇒ 机器面「无该字段 = 非危险档」（与 `emitSelected` 的「在册缺值 ⇒ 整键略去」同一条铁律，✗ 不落空串）；
//	· `opened`  = `openedForRun(c)`（真跑开没开 —— 与族级人面报数同一取法）；
//	· 族名认不出 ⇒ `(exitOK,false)`，交回主题/族面分派（✗ 不新增第三种错误形状）。
//
// ★ 为什么走 `selectJSONList`（同一出口）：字段投影 / 白名单 / 六键包封三件事**只有一处实现**
// （`emitSelected`）—— 本条只把「可选字段层」`command`/`danger`/`opened` 登进 `help` 的 `optFields`，出口照旧收口。
func emitHelpFamilyActions(inv *invocation, family string, stdout, stderr io.Writer) (int, bool) {
	members := familyMembers(family)
	if len(members) == 0 {
		return exitOK, false
	}
	if rc := requireFields(inv, stderr); rc != exitOK {
		return rc, true
	}
	rows := make([]map[string]string, 0, len(members))
	for _, c := range members {
		row := map[string]string{
			"command": "zerg " + strings.Join(c.path, " "),
			"opened":  strconv.FormatBool(openedForRun(c)),
		}
		if c.danger != nil {
			row["danger"] = strings.TrimSpace(c.danger.Level)
		}
		rows = append(rows, row)
	}
	return selectJSONList(stdout, stderr, inv, []string{"help"}, inv.fields, rows), true
}

func catalog() []*command {
	out := make([]*command, len(commands))
	copy(out, commands)
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i].path, " ") < strings.Join(out[j].path, " ")
	})
	return out
}

// ---- invocation：旗标解析（不引第三方库：批 A 的旗标面很小，自己解析更好审）----

type invocation struct {
	path []string
	args []string

	jsonGiven bool
	fields    []string

	plain    bool
	wantHelp bool
	// ★ 本笔：`-h` 与 `--help` **分开记**（修「`zerg gate check <判据名> -h` 被命令树截成族帮助」）。
	//   解析面此前把两枚并成同一个 `wantHelp` ⇒ 透传型命令（`passthrough: true`）的**子件**永远
	//   收不到 `-h`：`gate check` 壳逐字转发的是 `inv.orig[2:]`，可派发层在 `run` 里先被
	//   `if inv.wantHelp { return cmdUsageHelp(...) }` 截住，子件 usage 在 CLI 上一律不可达。
	//   两枚**语义分开**（判据在 `passthroughChildHelp`）：`--help` 仍是该条命令的族帮助面
	//   （既有行为一字不动），`-h` 在「透传型 + 已点名子件目标」时交给子件。
	wantHelpShort bool // 只由 `-h` 置位
	wantHelpLong  bool // 只由 `--help` 置位
	wantVersion   bool
	tty           bool // stdout 是不是终端（行式面/人面的分档依据 · §九 M14）

	// 原样透传用：命令行原样（含未知旗标）+ 这一轮见过的未知旗标（非透传命令要据此报错）
	orig    []string
	unknown []string
	// 缺口 `GAP-20260928-151`：值旗标后面紧跟的词**以连字符开头**的逐条点名（解析面只**记**、不判 ——
	// 由 `run` 原样报到 stderr；裸形语义与退码一个字节不动）。见 `noteValueFlagLooksLikeFlag`。
	valueFlagLooksLikeFlag []string

	// 危险动作三态（§4.1 K7 · §九 M3 C1/C2/C4）
	dryRun bool
	all    bool // `build show --all`
	fast   bool // `gate bench --fast`
	// `--no-fast`（缺口 `GAP-20260927-41` ③ · 2026-10-12）：`repo commit` 的**全量档出口** —— 与
	// `--fast` **相反**（提交面默认只跑快档）：给它 = **强制走全量档**（同一条门禁命令不带 `--fast`）。
	// 与上面几枚同一形态：全局布尔、谁用谁读 —— 不用的命令静默忽略。
	noFast bool
	// `--full`（缺口 `GAP-20260927-09` · 2026-09-27 父代理现场补）：`code find`/`code show` 的
	// **不截断**档 —— 此前 `truncateDisplay(…, 200)` 把长行尾巴在**人面与机器面一起**切掉。
	full bool
	// `A3` 波纹卡片的两档（§4.1 · `R9` 已拍「分两档」；`R38` 拍定：与 `--json` **不是同一条** ——
	// 前者只决定**内容与裁剪**（条数 / 全文），后者只做**字段投影**；两者可叠加，都不改六键包封）。
	forModel     bool
	forHuman     bool
	wide         bool // `C3`：文档面口径 A 档（裸词 · `R23`）
	strict       bool // `C3`：文档面口径 C 档（与「件:行」同行 · 最严档）
	deep         bool // `C3`/`C4`/`C1`：**贵面现跑**（重复面报数 + 六档标定曲线 · 删面两器）
	confirm      string
	confirmGiven bool
	// `模型面` 两枚布尔（波① `T2`/`T1a`）：`--declared`（`core daemon ls` 并给声明面两列 +
	// 幽灵计数）与 `--verified`（`model add` 写 `verified: true`）。与 `--all`/`--fast` 同一形态：
	// 全局布尔、谁用谁读 —— 不用的命令静默忽略。
	declared bool
	verified bool
	// `--last`（缺口 `Q-111` · `gate results`）：读**最近一趟**现成的门禁产物（与 `--dir` 互斥 ——
	// 两个来源不许混）。与 `--all`/`--fast` 同一形态：全局布尔、谁用谁读。
	last bool
	// `--list`（`agent registry` 的只读面 · 2026-09-26）：与 `--all`/`--fast` 同一形态 ——
	// 全局布尔、谁用谁读（别的命令静默忽略，不各开一个分叉）。
	list bool
	// `--manifest-only`（缺口 `Q-226` · 2026-09-26）：`build release` 的**清单档** —— 只重算
	// `dist/<版本>/release/` 的清单与目录内逐件 sha256（**不编译 · 不重打包 · 不拷进 `bin/` ·
	// 不重签 · 不重启** ⇒ 碰不到正在跑的件）。与 `--list`/`--all`/`--fast` 同一形态：
	// 全局布尔、谁用谁读 —— 不用的命令静默忽略（不各开一个分叉）。
	manifestOnly bool
	// `--ttl <时长>`（`E4` · 人签批准件的**有效期**面）：与 `--confirm` 同一种形态 —— 「给了旗标」与
	// 「给了值」是两件事（`--ttl` 裸给 ⇒ `ttlGiven` 真、值为空 ⇒ 由 `approve new` 判成用法错 2，
	// **不许**静默当「没给」）。缺省（不给这一枚）⇒ 不过期，件与今天逐字节同形态。
	ttl      string
	ttlGiven bool
	yes      bool

	// 契约主号（§十二 P-026：消费侧拒绝不认的 schema 主号 —— `--schema zerg/v2` ⇒ 2）
	schemaWant  string
	schemaGiven bool

	// 幂等键（§九 M4 · 调研-M5 §3.9 的 `--idempotency-key`；不给则派生）
	idemKey string

	// 远端语义（§九 M13）：连接级 --context / 对象级 --node（可重复）
	contextWant string
	nodes       []string

	// M4 §5.3 的三档旗标（本版只解析 + 判词，语义属写面）
	reload bool
	force  bool

	// 非交互凭据（§九 M2 C2）：令牌从 stdin 读，**不进 argv**
	tokenStdin bool

	// 包封三真值（**只在有真事时**才写；它们**不是** JSON 键 —— 顶层仍是那六键）：
	//   · envWarn → `warnings[]` 逐条（空 ⇒ 逐字 `[]`；**拿不到真值就不许写**）
	//   · envCut  → `truncated`（只有**真裁了条目**才置 true；没判定过 ⇒ false）
	//   · envMeta → `meta` 的**按需子键**（序即给定序；`count`/`source`/`changed`/`node`/
	//     `idempotency_key` 五个旧子键不许从这里写 —— 旧子键语义不变 ✗）
	envWarn []string
	envCut  bool
	envMeta []envMetaKV

	// 内容协商（§九 M1 W12）：`--accept <媒体类型>`
	acceptWant string

	// 茧壁直连（§十五.4 例外清单 F-2 / 丙案：默认经主控，直连要显式）
	direct bool

	// 动作面旗标（批 D · S5：值原样收下，语义校验在各自命令里 —— 这里只做「收下来」）
	// 映射写在 family_task.go 一处（`--desc` → 请求体的 `description` 一类）。
	desc               string
	modelWant          string
	priority           string
	sliceID            string
	dependsOn          []string
	acceptance         []string
	acceptanceDeclared bool
	moveTo             string
	to                 string
	setPairs           []string // `--set k=v`（可重复；`model opts set` 用）
	reset              bool     // `--reset`（`gateway breakers` 的重置开关）

	// `--quick`：贵项跳过并记 SKIP（§十二 P-040）
	quick bool

	// `--self-test`：走**合成夹具**的成对负控（正控 + 负控），不碰真目标（D3b 第四步起）。
	selfTest bool

	// `--gate`：`zerg dev verify` 的**升阶闸门**档（§20.4 · §20.7 OM11 · 批 E · T-62）。
	// 与 `--quick` 同形：一枚布尔旗标，**不新增命令名**。
	stageGate bool

	// `--resume`：续做面（§十二 `P-120` 定案 ① —— 「上次做到哪」的入口 = `zerg context ls --resume`）
	resume bool

	// M8 长任务三档（§十二 P-020/P-033–P-037）
	wait              bool
	noWait            bool
	follow            bool
	cancelOnInterrupt bool

	// 本次调用被报出来的那个错（§九 M7：`--json` 失败时挂进包封的 `error` 块）。
	err *cliError

	// 本次调用是否改变了状态（§九 M4 的 `changed`；nil ⇒ 按命令的幂等档派生）。
	changed *bool

	// 自开发面旗标（批 E · T-57 起）：**原样收下**、语义校验在各自命令里。
	// 与上面那排具名旗标的分工：具名的是「一条命令一件」（`--desc` 只给 `task submit` 用）；
	// 这里是「一族共用一张名字表」——`--title`/`--target`/`--evidence`/`--candidate` 一类。
	// ★ 为什么仍要走 valueFlagName：不认的旗标在 dispatch 里一律 2 ⇒ 不收下来就等于命令不可用
	//   （`--out` 今天的教训：用法串里写着、解析器不认 ⇒ 真给就退 2 · 见表二「所见非本批」）。
	kv map[string][]string

	// ★ GAP-20260928-196（甲 · 2026-10-10）：两态读数 resolver 的**一发一份**缓存 —— 裁决
	//   （`gapTwoStateJudge`）与落账（`gapTwoStateLanded`）同源：同一发里多次调用只**跑一趟**
	//   （`--before-cmd`/`--after-cmd` 至多各跑一次）· 且逐字同值。nil ⇒ 还没解过。
	twoStateCache *gapTwoStateResolution
}

// kvSet 收一枚自开发面旗标的值（可重复的名字就逐次 append —— 先给先留）。
func (inv *invocation) kvSet(name, val string) {
	if inv.kv == nil {
		inv.kv = map[string][]string{}
	}
	inv.kv[name] = append(inv.kv[name], val)
}

// flagVal 取一枚旗标的**最后一个值**（不给/只给旗标不给值 ⇒ 空串）。
func (inv *invocation) flagVal(name string) string {
	vs := inv.kv[name]
	if len(vs) == 0 {
		return ""
	}
	return vs[len(vs)-1]
}

// flagVals 取一枚（可重复）旗标的**全部值**（按给值次序）。
func (inv *invocation) flagVals(name string) []string {
	return inv.kv[name]
}

// hasFlag 判「这枚旗标**给过没有**」（与「值非空」是两件事 —— 与 `ttlGiven` 同一条口径）。
// 序138 的墙钟上界就靠它决定要不要给这一趟挂守卫（不给 ⇒ 一个字都不加）。
func (inv *invocation) hasFlag(name string) bool {
	_, ok := inv.kv[name]
	return ok
}

func parseInvocation(args []string) (*invocation, error) {
	inv := &invocation{orig: append([]string{}, args...)}
	// 缺口 `GAP-20260928-151`（值旗标吞旗标 · P1）：解析**之前**扫一遍 —— 只**记**信号，不判码。
	inv.valueFlagLooksLikeFlag = noteValueFlagLooksLikeFlag(args)
	var pos []string
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case a == "--json" || a == "--json=":
			// 给了旗标但没给字段：留给命令去判（契约 §4.1 K2：退码**取自退码表** `usage` = 2 + stderr 列字段）。
			inv.jsonGiven = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.fields = splitFields(args[i])
			}
		case strings.HasPrefix(a, "--json="):
			inv.jsonGiven = true
			inv.fields = splitFields(strings.TrimPrefix(a, "--json="))
		case a == "--plain":
			inv.plain = true
		case a == "--idempotency-key" || a == "--idempotency-key=":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.idemKey = args[i]
			}
		case strings.HasPrefix(a, "--idempotency-key="):
			inv.idemKey = strings.TrimPrefix(a, "--idempotency-key=")
		case a == "--schema" || a == "--schema=":
			inv.schemaGiven = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.schemaWant = args[i]
			}
		case strings.HasPrefix(a, "--schema="):
			inv.schemaGiven = true
			inv.schemaWant = strings.TrimPrefix(a, "--schema=")
		case a == "--node" || a == "--node=":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.nodes = append(inv.nodes, args[i])
			}
		case strings.HasPrefix(a, "--node="):
			inv.nodes = append(inv.nodes, strings.TrimPrefix(a, "--node="))
		case a == "--context" || a == "--context=":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.contextWant = args[i]
			}
		case strings.HasPrefix(a, "--context="):
			inv.contextWant = strings.TrimPrefix(a, "--context=")
		case a == "--token-stdin":
			inv.tokenStdin = true
		case a == "--reset":
			inv.reset = true
		case valueFlagName(a) != "":
			// 动作面旗标（批 D · S5）：`--desc` / `--model` / `--priority` / `--slice-id` /
			// `--depends-on` / `--acceptance` / `--to` / `--set`。值**原样**收下（语义校验在各自命令里）。
			// ★ 裸形（后面跟的是另一枚旗标、或已到 argv 尾）与 `--k=v` 走**同一条路**：都
			// `setValueFlag(a, "")` —— 即「**给过**、值空」，键**一定**存在 ⇒ `hasFlag`/`inv.kv`
			// 这两条「给过没有」的守卫才真生效（此前只有 `--acceptance` 有 else ⇒ 其余 90 枚裸形
			// 静默无痕 ⇒ `publish run --out`、`script inventory sync --out/--set` 两个守卫**永不触发**）。
			// `--acceptance` 的**三态语义不变**：`setValueFlag("--acceptance", "")` 照旧置
			// `acceptanceDeclared=true` 而不 append ⇒ 仍发 `null`（「没写」与「写了空」是两件事，
			// 不许在这里合并 —— 与 `--acceptance=` 分派（本 switch 下一 case）的行为逐字一致）。
			// ★ 单枚 `-` 是**真值**（`-` = stdin 的 CLI 惯例）：本判与上面
			// `noteValueFlagLooksLikeFlag` 的口径（`nxt == "-"` ⇒ 真值）原来不一致 ——
			// 结果是 `--from -` 被当「给过、值空」，`-` 这一档**永远够不到**。
			// 本枚只补这一格（判定面），`--k=v` 与裸形语义其余一字不动。
			if i+1 < len(args) && (!strings.HasPrefix(args[i+1], "-") || args[i+1] == "-") {
				i++
				inv.setValueFlag(a, args[i])
			} else {
				inv.setValueFlag(a, "")
			}
		case strings.HasPrefix(a, "--") && strings.Contains(a, "=") && valueFlagName(strings.SplitN(a, "=", 2)[0]) != "":
			k, v, _ := strings.Cut(a, "=")
			inv.setValueFlag(k, v)
		case a == "--accept" || a == "--accept=":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.acceptWant = args[i]
			}
		case strings.HasPrefix(a, "--accept="):
			inv.acceptWant = strings.TrimPrefix(a, "--accept=")
		case a == "--direct":
			inv.direct = true
		case a == "--quick":
			inv.quick = true
		case a == "--gate":
			inv.stageGate = true
		case a == "--resume":
			inv.resume = true
		case a == "--wait":
			inv.wait = true
		case a == "--no-wait":
			inv.noWait = true
		case a == "--follow":
			inv.follow = true
		case a == "--cancel-on-interrupt":
			inv.cancelOnInterrupt = true
		case a == "--reload":
			inv.reload = true
		case a == "--force":
			inv.force = true
		case a == "--self-test":
			// `--self-test`（D3b 第四步）：新门/新命令的**成对负控**入口（合成夹具 · 不碰真目标）。
			inv.selfTest = true
		case a == "--list":
			// `agent registry --list`：那台子端注册表的**只读面**（不改一个字节）。
			inv.list = true
		// `--manifest-only`（缺口 `Q-226` · 2026-09-26）：`build release` 的清单档。
		// 与上面几枚同一种形态：全局布尔、谁用谁读 —— 不用的命令静默忽略。
		case a == "--manifest-only":
			inv.manifestOnly = true
		case a == "--dry-run":
			inv.dryRun = true
		// `--purge`（C1 · 虫茧生命周期 · 2026-10-20）：`cocoon close` 的**删树**档。
		// 布尔旗标**不进** `valueFlagName`（门73 判据丙：布尔按取值收会判形态不一致）⇒ 这里单列一条。
		case a == "--purge":
			inv.kvSet(a, "true")
		case a == "--yes":
			inv.yes = true
		// 缺口面 P0 两枚布尔旗标（2026-09-21）：`--all`（`build show` 列全部件）与
		// `--fast`（`gate bench` 的档位）。与 `--force`/`--dry-run` 同一种形态：
		// 全局布尔、谁用谁读 —— 不用的命令静默忽略（解析器不许给两条命令各开一个分叉）。
		case a == "--all":
			inv.all = true
		case a == "--fast":
			inv.fast = true
		// `--no-fast`（缺口 `GAP-20260927-41` ③ · 本枚）：`repo commit` 的**全量档出口** —— 与
		// `--fast` **相反**。与上面几枚同一形态：全局布尔、谁用谁读 —— 不用的命令静默忽略。
		// ★ **✗ 不进 `valueFlagName`**（那张表是**取值**旗标名表）：布尔 case 即登记（门73 面③）。
		case a == "--no-fast":
			inv.noFast = true
		// `--full`（缺口 `GAP-20260927-09`）：长行**不截断**。与上面几枚同一种形态 —— 全局布尔、
		// 谁用谁读，不用的命令静默忽略；★ 它必须在本表「上户口」，否则派单里写着 `--full` 会
		// 一律退 2「未知旗标」= 命令等于不可用（`:2508` 那条教训的原话）。
		case a == "--full":
			inv.full = true
		// `--last`（2026-09-23 · 缺口 `Q-111`/`B-8`）：`gate results` 读**最近一趟**现成的门禁产物。
		// 与上面两枚同一种形态：全局布尔、谁用谁读 —— 不用的命令静默忽略。
		// 为什么不做成取值旗标：它指的是**目录来源**（最近一趟），不是一条路径 —— 路径由 `--dir` 给。
		case a == "--last":
			inv.last = true
		// `A3` 两档（§4.1）：模型档 / 人面档。可叠加（§4.1 的模型档是人面档的子集 ⇒
		// 同时给以**人面档**为准，由命令自己明说；R38：「可叠加」指的是与 `--json`）。
		case a == "--for-model":
			inv.forModel = true
		case a == "--for-human":
			inv.forHuman = true
		// `C3` 文档面口径三档（`R23` 已拍）：`--wide` = A 档（裸词）· `--strict` = C 档（与「件:行」同行）。
		// 与 `--for-model`/`--for-human` 同一种形态：全局布尔、谁用谁读 —— 不用的命令静默忽略。
		case a == "--wide":
			inv.wide = true
		case a == "--strict":
			inv.strict = true
		// `C` 批新加的两个**贵面**（重复面 = 一次 `jscpd` + 六档曲线；删面 = `deadcode -test` +
		// `staticcheck U1000`）：默认档与 `--all` 都**不跑**（只打口径与照实状态），
		// 要真跑就显式 `--deep` —— 跑一次约 40s，不给日常路径与门禁套件加这份账。
		case a == "--deep":
			inv.deep = true
		case a == "--declared":
			inv.declared = true
		case a == "--verified":
			inv.verified = true
		case a == "--confirm" || a == "--confirm=":
			// 给了旗标但没给值 ⇒ confirmGiven 为真、值为空（由 guard 判成「值不匹配目标」）
			inv.confirmGiven = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.confirm = args[i]
			}
		case strings.HasPrefix(a, "--confirm="):
			inv.confirmGiven = true
			inv.confirm = strings.TrimPrefix(a, "--confirm=")
		// `--ttl <时长>`（`E4`）：与 `--confirm` 逐字同一种形态（**给了旗标 ≠ 给了值**）——
		// 裸给 ⇒ `ttlGiven` 真、值为空 ⇒ `approve new` 判「没给时长」用法错 2（不静默当没给）。
		case a == "--ttl" || a == "--ttl=":
			inv.ttlGiven = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.ttl = args[i]
			}
		case strings.HasPrefix(a, "--ttl="):
			inv.ttlGiven = true
			inv.ttl = strings.TrimPrefix(a, "--ttl=")
		// `zerg ls-face` 的六枚旗标（A2 · 2026-09-27 · 缺口账 `GAP-20260927-216` 的 ②）：
		// 四个**面**旗标 + `--utf8` 是**布尔**（给过即真，语义与互斥由命令自己判），
		// `--prefix` 是**取值**旗标。与 `--full`/`--last` 同一种形态：全局布尔/取值、谁用谁读 ——
		// 不用的命令静默忽略；★ 必须在下面那条「未知旗标」兜底**之前**上户口，否则派单里
		// 写着它们会一律退 2「未知旗标」= 新命令等于不可用。
		case a == "--tracked" || a == "--others" || a == "--staged" || a == "--diff" || a == "--utf8":
			inv.kvSet(a, "true")
		// 门进度可查（设计-门进度可查-v0.3 §3.1/§十.4/§十.6）：两枚**布尔**旗标。
		// `--pick` = 多趟活时挑一趟（**非 TTY 下拒执**，由 family_gate_progress.go 判）；
		// `--exit-status` = `gate wait` 把门禁的红透成退码。写进 inv.kv ⇒ foreignFlag 看得见。
		case a == "--pick" || a == "--exit-status":
			inv.kvSet(a, "true")
		case a == "--prefix" || a == "--prefix=":
			// 裸形（后面跟的是另一枚旗标、或已到 argv 尾）与 `--prefix=` 走同一条路：
			// 键**一定**存在、值空 ⇒ 命令面靠 `hasFlag("--prefix")` 判「给过没有」、再判空值
			// （「没给」与「给了空」是两件事 —— 与 `--ttl`/`--acceptance` 同一条口径）。
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.kvSet("--prefix", args[i])
			} else {
				inv.kvSet("--prefix", "")
			}
		case strings.HasPrefix(a, "--prefix="):
			inv.kvSet("--prefix", strings.TrimPrefix(a, "--prefix="))
		// `code find` 的**按件聚合**两档（缺口 `GAP-20260928-53` 的 ② · 2026-09-28）：
		// `--count`（件名 + 命中数）与 `--files-only`（只列件名）。两枚都是**布尔**
		// —— 与上一块 ls-face 的面旗标同一种形态：写进 `inv.kv`（`kvSet`）而不是
		// 单开一个字段 ⇒ 好处是 `foreignFlag` 能**看得见**它们（它只读 `inv.kv` + `--all`/`--full`）
		// ⇒ 不归本命令用的那些面（`code show` / `find`）仍然 rc=2 点名，
		// 不会从「未知旗标 2」悄悄退化成「静默吞」（缺口 `GAP-20260927-16` 那一类）。
		// ★ 必须在下面那条「未知旗标」兜底**之前**上户口，否则派单里写着它们会一律退 2。
		case a == "--count" || a == "--files-only":
			inv.kvSet(a, "true")
		// `code find` 的两枚**放行**旗标（缺口 `GAP-20260927-66` 的 `--hidden` · `GAP-20260927-72` 的
		// `--include-runtime` · 本枚）：`--hidden`（**点项放行** —— 默认跳过 `.` 开头的件/目录）与
		// `--include-runtime`（**运行期目录整档放行**）。与上一块 `--count`/`--files-only` 逐字同一种
		// 形态：写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见（`cmdCodeFind` 已在读 `inv.hasFlag`）；
		// ★ 此前**解析面没上户口** ⇒ 用法串/实现件都当它俩存在、解析器却一律退 2「未知旗标」= 两枚
		//   等于不可用（`GAP-20260929-13`）。照 `--count` 那条：必须在下面「未知旗标」兜底**之前**。
		//   ★ 两枚**各写一条** `case`（行为逐字不变）：门73 的取数面按 `case a == "--x"` 逐条现读，
		//   写成 `--hidden || --include-runtime` 时它只读得到前者（同 `--aging`/`--stale` 那条教训）。
		case a == "--hidden":
			inv.kvSet(a, "true")
		case a == "--include-runtime":
			inv.kvSet(a, "true")

		// ★ 本枚（缺口 `GAP-20261001-69` 的前置工具 `code const-dup`）：`--fail-on-hit` 是**布尔**旗标
		//   —— 命中 >0 ⇒ 退 1（供 CI 用）；不给它 ⇒ 命中再多也退 0（**默认不红**）。写进 `inv.kv`
		//   （`kvSet`）而不是单开字段 ⇒ `foreignFlag` 看得见它（不归本命令用 ⇒ 仍 rc=2 点名）。
		//   ★ 布尔旗标**不**进 `valueFlagName` 名字表（门73 判据丙会判红），且**单独一条 case**
		//   （与 `--hidden` 同一条教训：合并写只读得到前者）。
		case a == "--fail-on-hit":
			inv.kvSet(a, "true")
		// ★ 2026-10-05（缺口 `GAP-20261005-68`）：`ask` 的 `--no-fallback` **已摘牌**（从命令树移除）——
		//   它此前在命令树登记（用法串 + 本处解析 case）却在**全库零消费者**（`cmdAsk` 从不读
		//   `inv.kv("--no-fallback")`，也无全局消费面）⇒ 真跑静默 rc=0、毫无作用，属「有登记无实现」。
		//   （`GAP-20260928-127` ⑥ 此前只治了它的**形态**「取值 ⇒ 布尔」，没治「登记面 ⟷ 实现面」不对齐。）
		//   摘牌后真给该旗标 ⇒ 落下面「未知旗标」兜底 ⇒ 退 2（负控 = `zerg ask ping --no-fallback` 应 rc=2）。
		// ★ 批3 第三片（2026-09-28）：`gap assign ls` 的两枚**布尔**面旗标 —— `--aging`（按超时限程度
		//   降序 · 时限算不出的单列「时限不明」）/ `--stale`（只列超时限未收的派单）。与上一块
		//   `--count`/`--files-only` 逐字同一种形态：写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见；
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则派单里写着它们会一律退 2「未知旗标」。
		//   ★ `--stale` 单列一条 `case`（行为逐字不变 —— 两枚仍各自 `kvSet(a, "true")`）：门73
		//   的取数面按 `case a == "--x"` 逐条现读，写成 `--aging || --stale` 时它只读得到前者
		//   ⇒ `gate reap --stale […]`（用法串是取值形态）被判成「解析面不认」= 假红。
		case a == "--aging":
			inv.kvSet(a, "true")
		case a == "--stale":
			inv.kvSet(a, "true")
		// ★ R1 登记（2026-10-04 · 缺口 `GAP-20260929-20`/`-12`）：`gap plan` 的 `--allow-drifted`
		//   —— 「现读状态」列对**漂移行**默认拒处理，`--allow-drifted` 才放行并留痕。语义是
		//   布尔档 ⇒ 与 `--aging`/`--stale` 同一种形态（写进 `inv.kv` ⇒ `foreignFlag` 看得见）；
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则派单里写着它会一律退 2「未知旗标」。
		case a == "--allow-drifted":
			inv.kvSet(a, "true")
		// ★ 2026-10-09（缺口账 `GAP-20261008-06`/`-07`）：`gap set-unit` 的 `--allow-missing-unit`
		//   —— `--unit` 指向**仓内不存在**路径时的**显式放行闸**（默认拒 2 · ✗ 不许默默放行）。
		//   语义是布尔档 ⇒ 与 `--aging`/`--stale`/`--allow-drifted` 同一种形态（写进 `inv.kv`）。
		case a == "--allow-missing-unit":
			inv.kvSet(a, "true")
		// ★ 本枚（缺口 `GAP-20261010-20` · 2026-10-17）：`audit hardcoded-addr` 的**布尔**旗标
		//   `--fail-on-hit` —— 默认清单档（有命中仍 0）；给了它才把「命中 > 0」透成 rc=1（供门/CI）。
		//   与 `--aging`/`--stale`/`--allow-drifted` 同一种形态（写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见）；
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则派单里写着它会一律退 2「未知旗标」= 旗标等于不可用。
		//   ★ **✗ 不进 `valueFlagName`**（那张表是**取值**旗标名表 · 门73 判据丙）。
		case a == "--fail-on-hit":
			inv.kvSet(a, "true")
		// ★ 本枚（缺口 `GAP-20260928-54`）：`gap ls --untriaged` 的**布尔**面旗标（只列未打
		//   `triage_batch` 标记的条）。与 `--aging`/`--stale`/`--allow-drifted` 同一种形态（写进
		//   `inv.kv` ⇒ `foreignFlag` 看得见）；★ 必须在下面「未知旗标」兜底**之前**上户口。
		case a == "--untriaged":
			inv.kvSet(a, "true")
		// ★ R1 登记（2026-10-04 · 缺口 `GAP-20260927-377` · R3）：`dev check --syntax` 的
		//   两枚**布尔**旗标 —— `--syntax`（本命令的语义档）与 `--vet`（跑 go vet 而非 go build）。
		//   与 `--aging`/`--stale`/`--allow-drifted` 同一种形态（写进 `inv.kv` ⇒ `foreignFlag` 看得见）；
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则派单里写着它们会一律退 2「未知旗标」。
		case a == "--syntax":
			inv.kvSet(a, "true")
		case a == "--vet":
			inv.kvSet(a, "true")
		// `zerg chat` 族的五枚**布尔**旗标（T3 · 2026-10-17 · 本枚）：与 `--aging`/`--stale`/
		// `--allow-drifted` 同一种形态（写进 `inv.kv`（`kvSet`）⇒ 全族共用 · `foreignFlag` 看得见）；
		// ★ 必须在下面「未知旗标」兜底**之前**上户口，否则族里写着它们会一律退 2「未知旗标」。
		// ★ **✗ 不进 `valueFlagName`**（那张表是**取值**旗标名表 · 门73 判据丙）。语义面：
		//   `--new` = `O-2`（一次性发话 · ✗ 不加 `--once`）· `--archived` = `chat ls` 含归档 ·
		//   `--no-stream` = §7.5 非流式档 · `--markdown` = `O-9` 渲染 opt-in ·
		//   `--last-in-cwd` = `O-16`（把 `--last` 收窄到本工作区）。
		//   ★ 五枚**各写一条** `case`（✗ 不许合并成一串 `||`：门73 的取数面按 `case a == "--x"`
		//   逐条现读，合并时它只读得到前者 ⇒ 后者被判「解析面不认」= 假红）。
		case a == "--new":
			inv.kvSet(a, "true")
		case a == "--archived":
			inv.kvSet(a, "true")
		case a == "--no-stream":
			inv.kvSet(a, "true")
		case a == "--markdown":
			inv.kvSet(a, "true")
		case a == "--last-in-cwd":
			inv.kvSet(a, "true")
		// ★ 本枚（2026-10-17 · `chat ask` 机器正门）：`--stream` 是**布尔**档 —— 默认**不**流式（只出成稿），
		//   显式 `--stream` 才逐帧打。与上面五枚同一种形态（写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见）；
		//   ★ **✗ 不进 `valueFlagName`**（那张表是取值旗标名表 · 门73 判据丙）；★ 单列一条 `case`
		//   （✗ 不许与兄弟合并成一串 `||`：门73 取数面按 `case a == "--x"` 逐条现读）。
		case a == "--stream":
			inv.kvSet(a, "true")
		// ★ 批 B1（2026-10-20 · `zerg trace` 归入命令面）：三枚**布尔**旗标 ——
		//   `--live`（实时尾随）/ `--tokens`（token 分析视图）/ `--errors`（错误清单视图）。
		//   与 `--stream`/`--include-open` 同一种形态（写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见）；
		//   ★ **✗ 不进 `valueFlagName`**（那张表是取值旗标名表 · 门73 判据丙：布尔名登进去 ⇒ 形态判红）；
		//   ★ 三枚**各写一条** `case`（✗ 不许合并成一串 `||`：门73 的取数面按 `case a == "--x"` 逐条现读，
		//   合并时只读得到前者 ⇒ 后两枚被判「解析面不认」= 假红）。用法串与本处**必须同改**（门73）。
		case a == "--live":
			inv.kvSet(a, "true")
		case a == "--tokens":
			inv.kvSet(a, "true")
		case a == "--errors":
			inv.kvSet(a, "true")
		// ★ 批 B2（2026-10-20 · `zerg sched run` 归入命令面）：两枚**布尔**旗标 ——
		//   `--container`（容器模式 · CA 脱离沙箱）/ `--dev`（开发模式 · 容器内分支→commit→PR）。
		//   与 `--live`/`--tokens` 同一种形态（写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见）；
		//   ★ **✗ 不进 `valueFlagName`**（那张表是取值旗标名表 · 门73 判据丙：布尔名登进去 ⇒ 形态判红）；
		//   ★ 两枚**各写一条** `case`（✗ 不许合并成一串 `||`：门73 的取数面按 `case a == "--x"` 逐条现读）。
		//   用法串与本处**必须同改**（门73）＋ dispatch 收口（见下 `--container`/`--dev` 那一排）。
		case a == "--container":
			inv.kvSet(a, "true")
		case a == "--dev":
			inv.kvSet(a, "true")
		// ★ 批 B4（2026-10-20 · `zerg api serve` 归入命令面）：一枚**布尔**旗标 ——
		//   `--insecure`（允许无令牌运行 · **会打横幅**；源件 `-insecure` 的逐字保留）。
		//   与 `--container`/`--dev` 同一种形态（写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见）；
		//   ★ **✗ 不进 `valueFlagName`**（那张表是取值旗标名表 · 门73 判据丙：布尔名登进去 ⇒ 形态判红）；
		//   ★ 单列一条 `case`（✗ 不许合并：门73 的取数面按 `case a == \"--x\"` 逐条现读）。
		//   用法串与本处**必须同改**（门73）＋ dispatch 收口（见下 `--insecure` 那一排）。
		case a == "--insecure":
			inv.kvSet(a, "true")
		// ★ 批 B5（2026-10-20 · `zerg agent run` 归入命令面）：两枚**布尔**旗标 ——
		//   `--auto-issue`（重试耗尽仍失败 ⇒ 写 `docs/issues/` 挂单 · 源件 `-auto-issue` 逐字保留）·
		//   `--subtask`（子任务结晶模式 · 源件 `-subtask` 逐字保留）。
		//   与 `--container`/`--dev`/`--insecure` 同一种形态（写进 `inv.kv`（`kvSet`）⇒
		//   `foreignFlag` 看得见）；★ **✗ 不进 `valueFlagName`**（门73 判据丙：布尔名登进取值表 ⇒
		//   形态判红）；★ 两枚**各写一条** `case`（✗ 不许合并成一串 `||`：门73 取数面按
		//   `case a == "--x"` 逐条现读，合并只读得到前者）。用法串与本处**必须同改**（门73）
		//   ＋ dispatch 收口（见下 `--auto-issue` 那一排）。
		case a == "--auto-issue":
			inv.kvSet(a, "true")
		case a == "--subtask":
			inv.kvSet(a, "true")
		// ★ 批 B6（2026-10-20 · `zerg model record verify|probe|ls` 归入命令面）：三枚**布尔**旗标 ——
		//   `--integrity`（verify 额外核 files[] 与磁盘件 · 源件 `--integrity` 逐字保留）·
		//   `--in-place`（verify 核「记录位置与身份自洽」· 源件 `--in-place`）·
		//   `--store`（probe 写进模型目录 manifests/ · 源件 `--store`）。
		//   与 `--container`/`--dev`/`--insecure`/`--auto-issue` 同一种形态（写进 `inv.kv`（`kvSet`）
		//   ⇒ `foreignFlag` 看得见）；★ **✗ 不进 `valueFlagName`**（门73 判据丙：布尔名登进取值表 ⇒
		//   形态判红）；★ 三枚**各写一条** `case`（✗ 不许合并成一串 `||`：门73 取数面按
		//   `case a == "--x"` 逐条现读，合并只读得到前者）。用法串与本处**必须同改**（门73）
		//   ＋ dispatch 收口（见下 `--integrity` 那一排）。
		case a == "--integrity":
			inv.kvSet(a, "true")
		case a == "--in-place":
			inv.kvSet(a, "true")
		case a == "--store":
			inv.kvSet(a, "true")
		// `code find` 的**短形**上下文/归一旗标（缺口 `GAP-20260927-250/252` · 本枚）：
		//   `-i`（大小写归一 · 布尔）· `-A<N>`/`--after <N>`（后文）· `-B<N>`/`--before <N>`（前文）·
		//   `-C<N>`（前后各 N，POSIX grep 惯用 · 与缺口症状逐字 `-A2`/`-C1` 同形）。
		// 长形（`--after`/`--before`）走 `valueFlagName` 那一支；短形**不经过**本函数 ⇒ 在此另起 case。
		// 写进 `inv.kv`（与 `--hidden`/`--count` 同一种形态）⇒ `foreignFlag` 看得见。`-C` 同时落
		// `--after` 与 `--before` 两键（一处语义、同一份取值）。判在 `--help`/未知短形支**之前**。
		case a == "-i" || a == "--ignore-case":
			inv.kvSet("--ignore-case", "true")
		case a == "-A" || a == "-B" || a == "-C":
			v := ""
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				v = args[i]
			}
			switch a {
			case "-A":
				inv.kvSet("--after", v)
			case "-B":
				inv.kvSet("--before", v)
			case "-C":
				inv.kvSet("--after", v)
				inv.kvSet("--before", v)
			}
		case len(a) > 2 && a[0] == '-' && a[1] == 'A' || len(a) > 2 && a[0] == '-' && a[1] == 'B' || len(a) > 2 && a[0] == '-' && a[1] == 'C':
			// 紧贴值形态：`-A2` / `-B3` / `-C1`（含等号 `-A=2`）。
			v := strings.TrimPrefix(a[2:], "=")
			switch a[1] {
			case 'A':
				inv.kvSet("--after", v)
			case 'B':
				inv.kvSet("--before", v)
			case 'C':
				inv.kvSet("--after", v)
				inv.kvSet("--before", v)
			}
		// ★ 本笔：两枚**合并成一条 case 行是故意的**（门73 的取数面按 `case a == "--x"` 逐条现读，
		//   拆成两支会多出一枚未在用法串登记的短形）—— 语义在块内按字面分：长形 = 族帮助面，
		//   短形 = 可交给子件（判据在 `passthroughChildHelp`）。`zerg -h` / `zerg gate -h` 的
		//   裸形语义**不变**（两枚都置 `wantHelp`）。
		case a == "--help" || a == "-h":
			inv.wantHelp = true
			if a == "-h" {
				inv.wantHelpShort = true
			} else {
				inv.wantHelpLong = true
			}
		case a == "--version":
			inv.wantVersion = true
		// ★ 户口登记（2026-10-05 · 缺口账 `GAP-20261005-60` 剩面）：`gap reread --include-open`
		//   —— 把**未闭**（`state == 仍缺`）条目的 `verify_cmd` 纳入同一套只读判活读数。语义是**布尔档**
		//   ⇒ 与 `--count`/`--hidden` 逐字同一种形态：写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见；
		//   `cmdGapReread` 已用 `inv.hasFlag("--include-open")` 读它。
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则 `gap reread --include-open` 一律退 2
		//   （用法串 / 实现件都当它存在、CLI 解析面却恒拒 = 与 GAP-20260929-13 同形）。
		case a == "--include-open":
			inv.kvSet(a, "true")
		// ★ 2026-10-10（缺口账 `GAP-20260928-172` · 甲案）：`gap backfill-unit --force-recompute`
		//   —— 存量回填面的**重算档**（全账逐条现算三键 · 只写与现抽值不同的行）。语义是**布尔档**
		//   ⇒ 与 `--include-open`/`--allow-missing-unit` 同一种形态：写进 `inv.kv`（`kvSet`）⇒
		//   dispatch 的数据驱动收口看得见（usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则 `--force-recompute` 一律退 2。
		case a == "--force-recompute":
			inv.kvSet(a, "true")
		// ★ 2026-10-10（缺口 GAP-20260927-205 · 本枚）：`itask mode set --on|--off` 的两枚**布尔**旗标——
		//   与 `--force-recompute` 同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动收口看得见
		//   （usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。用法串与本表**必须同改**（门73）。
		//   ★ 两枚**各写一条** case（门73 取数面按 `case a == "--x"` 逐条现读 —— 写成两枚合并只读得到前者）。
		case a == "--on":
			inv.kvSet(a, "true")
		case a == "--off":
			inv.kvSet(a, "true")
		// ★ 本单（Mr2109「甲案」仓内半 · 缺口 GAP-20260927-234「守卫 literal-text」）：`dev edit --literal-text`
		//   —— **声明口**（布尔）：声明「本次命令行含**字面文本**（写文档 / 写夹具时引用的命令字面），**不是要执行的命令**」。
		//   与钩子侧免检口**同口径**（命令串里出现 `literal-text` ⇒ 该段是字面文本 ⇒ 放行）；旗标名本身含该子串 ⇒ 它就是那枚免检口的 **CLI 正门形态**。
		//   ★ **只声明、✗ 不放宽任何闸 / 审批 / 判据**（读者见 `cmdDevEdit`：只出一行回执、不改任何变量与退码）。
		//   形态 = **布尔**（用法串写裸形 `[--literal-text]`）⇒ 走本布尔 case（写进 `inv.kv` ⇒ `foreignFlag` 看得见）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙「形态」判红（名字表按取值收、用法串写裸形 ⇒ 值/无值两态不一致）。归属靠下面 dispatch 收口。
		case a == "--literal-text":
			inv.kvSet(a, "true")
		// ★ 本单（缺口账 `GAP-20260928-139` · 本枚）：`version --build-info`
		//   —— 制品身份面的**布尔**档（人面默认仍只印版本行，给此旗标才出构件身份块）。与
		//   `--literal-text`/`--force-recompute` 同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动
		//   收口看得见（usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。用法串与本 case**必须同改**（门73）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙「形态」
		//   判红（名字表按取值收、用法串写裸形 ⇒ 值/无值两态不一致）。用法串写**裸形** `[--build-info]`。
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则 `version --build-info` 一律退 2。
		case a == "--build-info":
			inv.kvSet(a, "true")
		// ★ 本单（缺口账 `GAP-20260927-240` · 本枚）：`gate check --list-selfcases`
		//   —— 判据转发壳 `zerg gate check <判据名>` 的**只读列举档**（列该判据件的自测格清单 /
		//   断言表 · ✗ 不跑判据 · 零写盘）。语义是**布尔档** ⇒ 与 `--build-info`/`--literal-text`
		//   同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动收口看得见（usage 未列它的
		//   命令 ⇒ 真给即退 2「未知旗标」）。用法串写**裸形** `[--list-selfcases]`。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙
		//   「形态」判红（名字表按取值收、用法串写裸形 ⇒ 值/无值两态不一致）。归属靠下面 dispatch 收口。
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则 `gate check … --list-selfcases` 一律退 2。
		case a == "--list-selfcases":
			inv.kvSet(a, "true")
		// ★ 本单（缺失项 `GAP-20260927-172` 之正解 · E1 `fleet render`）：三枚**布尔**旗标 ——
		//   `--preview-only`（缺省·只打印）/ `--emit`（真写·D2）/ `--expect-no-changes`（只读断言）。
		//   与 `--build-info`/`--list-selfcases` 同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动
		//   收口看得见（usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。用法串写**裸形**（用法串与本 case 同批）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙「形态」判红。
		case a == "--preview-only":
			inv.kvSet(a, "true")
		case a == "--emit":
			inv.kvSet(a, "true")
		case a == "--expect-no-changes":
			inv.kvSet(a, "true")
		// ★ 本单（缺口账 `GAP-20260927-355` · 本枚）：`dev build --dist` —— 用法串**逐字承诺**
		//   （`main.go:2737` `… [--scope go|rust|ui|all] [--dist] [--yes] [--dry-run]`）而解析面**没上户口**
		//   ⇒ 真给该旗标即落下面「未知旗标」兜底退 2 = **「承诺即拒收」**
		//   （本函数上段 `--out` 那条注释逐字：「用法串里写着的旗标必须真能被解析，
		//   否则命令等于不可用」）。同一条 usage 行的兄弟 `--scope` 早已在
		//   `valueFlagName` 登记 ⇒ 本枚是按同一口径补齐。
		//   语义是**布尔档**（用法串写裸形 `[--dist]`）⇒ 与 `--build-info`/`--list-selfcases` 同一种形态：
		//   写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动收口看得见（usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙「形态」判红。
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则 `dev build --dist` 一律退 2。
		case a == "--dist":
			inv.kvSet(a, "true")
		// ★ 本枚（main.go 开窗第 6 条 · 缺口 GAP-20261012-27）：`--check-excludes-parity` —— `publish run`
		//   的**对拍闸只读档**（零副作用：只跑排除名册对拍、不走 D3 确认 · 语义在 `family_publish_run.go`
		//   的 `inv.hasFlag("--check-excludes-parity")` 支）。语义是**布尔档**（用法串写裸形）⇒ 与
		//   `--dist`/`--build-info` 同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 收口看得见（usage 未列
		//   它的命令 ⇒ 真给即退 2「未知旗标」）。★ **✗ 不进 `valueFlagName`**（那是**取值**名表）：布尔名
		//   登进去会让门73 判据丙「形态」判红。必须在下面「未知旗标」兜底**之前**上户口。
		case a == "--check-excludes-parity":
			inv.kvSet(a, "true")
		// ★ Q-129：`--check` —— `zerg update` 的**只读**「有没有新版本」检查（真身
		//   `core/internal/selfupdate.Check`；与 `zerg-core update --check` 同一个函数、同一份缓存）。
		//   布尔档 ⇒ 写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动收口看得见；✗ 不进 `valueFlagName`。
		case a == "--check":
			inv.kvSet(a, "true")
		// ★ 本笔（缺口 `GAP-20260926-22`）：`--legacy` —— `publish run` 的**应急通道档**（直调已弃用的
		//   压平器 `scripts/build/publish-public.sh` · 语义在 `family_publish_run.go` 的 `legacy` 分支）。
		//   与上面 `--check-excludes-parity` 同一种形态：布尔档 ⇒ `kvSet` 上户口；dispatch 的
		//   数据驱动收口（`run` 里那条 `usageDeclaresFlag` 判）看得见 —— 别族 usage 未列它 ⇒ 真给即退 2。
		//   ★ **✗ 不进 `valueFlagName`**（那是取值名表；布尔名登进去会让门73 判据丙「形态」判红）。
		case a == "--legacy":
			inv.kvSet(a, "true")
		// ★ 本单（缺口 `GAP-20260929-28` 剩面 · 本枚）：`gate find --uncertain` —— 把「解析不确定」
		//   清单从 stderr 侧信道提到**正门** stdout / 机器面（`family_gate_find.go` 的
		//   `cmdGateFindUncertain`）。语义是**布尔档**（用法串写裸形 `[--uncertain]`）⇒ 与
		//   `--list-selfcases` / `--check-excludes-parity` 同一种形态：写进 `inv.kv`（`kvSet`）⇒
		//   dispatch 的数据驱动收口看得见（usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙
		//   「形态」判红。必须在下面「未知旗标」兜底**之前**上户口，否则 `gate find --uncertain` 一律退 2。
		case a == "--uncertain":
			inv.kvSet(a, "true")
		// ★ 本单（缺口 `GAP-20261017-10` · `fs` 族三条正门）：两枚**布尔**旗标 ——
		//   `--content-stdin`（`fs write` 从 stdin 读内容 · 与 `--content` 二选一）与
		//   `--append`（`fs write` 追加而非覆盖）。
		//   与 `--uncertain`/`--dist` 同一种形态：写进 `inv.kv`（`kvSet`）⇒ dispatch 的数据驱动收口看得见
		//   （usage 未列它的命令 ⇒ 真给即退 2「未知旗标」）。
		//   ★ 两枚**各写一条** case（门73 取数面按 `case a == "--x"` 逐条现读 —— 写成两枚合并只读得到前者）。
		//   ★ **✗ 不进 `valueFlagName`**：那张表是**取值**旗标名表，布尔名登进去会让门73 判据丙「形态」判红。
		case a == "--content-stdin":
			inv.kvSet(a, "true")
		case a == "--append":
			inv.kvSet(a, "true")
		case strings.HasPrefix(a, "-") && a != "-":
			// 未知旗标：**先记下**（透传型命令要把它逐字交给脚本）；非透传命令在 dispatch 里硬判。
			inv.unknown = append(inv.unknown, a)
		default:
			pos = append(pos, a)
		}
	}
	// ★ 2026-09-24（缺口 `Q-137` / `Q-141` · 组A）：`--help` 在这里**不再抹掉 `pos`**。
	//
	// 原写法 `if inv.wantHelp && len(pos) > 0 { pos = nil }` 把「问某条命令的用法」与
	// 「压根没给命令」并成**同一个形状**，于是 `zerg frobnicate --help`（命令**不存在**）
	// 和 `zerg frobnicate` 走了**两条互不相干的路**：前者落进 `run` 的「无 `path` ⇒ 打全局树」
	// 那一支 ⇒ `rc=0` + 全局头行 = **假绿**（探针把「命令不存在」读成「命令存在」）。
	//
	// 现在 `pos` **原样**交给 `run`：由它 `resolve` 之后**分两态** ——
	//   · 命令**在册** ⇒ 该条**用法串**（`run` 在派发之前返回）· 退 `0`；
	//   · 命令**不在册** ⇒ 走 `run` 的「未知命令」分支（stderr 首行逐字 `未知命令 …`）· 退 `2`。
	// 纪律不变：`--help` 仍然**不起命令**（两态都在 `cmd.run` 之前返回）⇒ 零副作用。
	inv.path = pos
	if inv.plain && inv.jsonGiven {
		return nil, fmt.Errorf("--plain 与 --json 互斥（--json 是机器面 · --plain 是行式面）")
	}
	// 长任务三档**互斥**（§九 M8 行 262–267：三档不许同时给，也不许自造第四档）。
	if n := boolCount(inv.wait, inv.noWait, inv.follow); n > 1 {
		return nil, fmt.Errorf("--wait / --no-wait / --follow 三档互斥（给了 %d 个；§九 M8 行 262–267）", n)
	}
	return inv, nil
}

// ---- 动作面旗标（批 D · S5）：一张名字表 + 一个收值口 ----------------------------------------

// ── 缺口 `GAP-20260928-151`（P1 · 「值旗标吞旗标」）───────────────────────────────────────────
//
// 病（2026-09-28 现读实测）：值旗标**紧跟的那枚 token 以连字符开头**时，上面的 switch 一律按
// 既有的「裸形」一路收下 —— 旗标**给过了**、值是**空串**。裸形本身是**承重**的（「键在、值空」
// 正是 `hasFlag` 与 `--acceptance` 三态的靠山：`publish run --out`、
// `script inventory sync --out/--set` 两个守卫的真身就是它）。于是一次「值漏给了」被读成
// 「给对了」：
//
//	`zerg gate results --dir --last` ⇒ `--dir` 的值被静默收空、命令改用**另一个来源**（`--last`）
//	⇒ stdout 5 行 · rc=0 · stderr 0 字节（现读实测）—— 假绿，且**无信号**。
//
// 治法（**只加信号、不动裸形语义**）：解析**之前**扫一遍 argv，把「值旗标 + 看起来像旗标的下一个词」
// 逐条点名列下，由 `run` 原样报到 stderr。口径**只此一处**（本函数 + 它那两句文案），别处不许再判：
//
//	· 退码**一个字节不动**（不改任何既有函数行为、不引入新退码 —— 裸形照旧生效）；
//	· **不**把那个「看起来像旗标的词」收下当值：那会改掉 `--acceptance --yes` 这类**合法**三态写法
//	  的语义，也会改掉 `code find --glob -x` 的整个取值面（现读后判为**风险更大**的一边，弃）；
//	· 与 `--dir/--last` 同形的专用字段旗标（`--json`/`--ttl`/`--confirm`/…）一并覆盖 —— 同一张名单。
func noteValueFlagLooksLikeFlag(args []string) []string {
	var hits []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break // `--` 之后一律位置参数（既有语义）⇒ 不为它下结论
		}
		if !valueFlagTakesNext(a) {
			continue
		}
		if i+1 >= len(args) {
			continue // 已到 argv 尾：裸形（给过、值空）—— 既有语义，不报
		}
		nxt := args[i+1]
		if !strings.HasPrefix(nxt, "-") || nxt == "-" {
			continue // 真值（含单枚 `-`）
		}
		hits = append(hits, fmt.Sprintf(
			"值旗标 %s 后面跟的是 %s —— 它**看起来像旗标**，故这一发里 %s 的值按**空**收（裸形语义：给过、值空）⇒ 命令可能改用**别的来源**：值漏给了就补上（`%s <值>`），确实要给空值就写 `%s=`",
			a, nxt, a, a, a))
	}
	return hits
}

// valueFlagTakesNext —— 这枚 token 是不是「**值为下一个 token**」的旗标。
// 口径与上面 switch 的两处**同表**：`valueFlagName` 那一族 + 九枚专用字段旗标。
// ★ 只认**裸形**名字：`--k=v` 形态自带值 ⇒ 不在面内（`--json=fields` 也不会命中）。
func valueFlagTakesNext(a string) bool {
	if valueFlagName(a) != "" {
		return true
	}
	switch a {
	case "--json", "--json=", "--idempotency-key", "--idempotency-key=",
		"--schema", "--schema=", "--node", "--node=", "--context", "--context=",
		"--accept", "--accept=", "--confirm", "--confirm=", "--ttl", "--ttl=",
		"--prefix", "--prefix=":
		return true
	}
	return false
}

// valueFlagName 认「动作面旗标」的名字（**唯一真源**：解析与 `--k=v` 分派都读它）。
func valueFlagName(a string) string {
	switch a {
	case "--desc", "--model", "--priority", "--slice-id", "--depends-on", "--acceptance", "--to", "--set",
		"--port":
		return a
	}
	// 自开发面旗标（批 E · T-57 起）：一族共用一张名字表 —— 值照收，语义在各自命令里判。
	// ★ 2026-09-28（缺口 `GAP-20260928-127` 的 ②③④ · 本枚）：`--criteria` / `--round` /
	//   `--producer` 三枚**幽灵死条目**从本表除去 —— 全包零引用 + 全部用法串零提及
	//   = 给不存在的旗标发户口（门73 判据乙「多一处」）。同日门73 存量豁免表同步除名。
	switch a {
	case "--title", "--target", "--goal", "--evidence", "--rollback", "--by",
		"--criterion", "--criterion-new", "--candidate", "--state",
		"--dir", "--expect",
		// ★ 本枚（缺口账 `GAP-20260927-249`）：`--solved-kind` —— `gap set-state` 的**销案分栏**
		//   （`实现` / `销案` / `前提不成立` / `部分` · 第四值由缺口 `GAP-20260927-244` 方案乙加）。
		//   与上面各排同一口径：值照收，语义在 `cmdGapSetState` / `cmdGapLs` 里判（★ 本排是名字表
		//   「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用）。
		// ★ 缺口 `GAP-20260927-244` 方案乙：续 `--remainder <残面描述>`（`set-state` 的**残面另记** ——
		//   写法同上）。
		// ★ 本枚（main.go 开窗第 4 条）：续 `--judge-cmd <命令串>` —— `gap set-state` 的
		//   **「前提不成立」可跑判据**（正门**真跑**它、用 rc/读数当证据；语义在 `cmdGapSetState` 里判）。
		//   本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
		"--solved-kind", "--remainder", "--judge-cmd":
		return a
	}
	// ★ 缺口 `GAP-20260926-70`（2026-10-23 · 本枚）：续两枚 —— `--qid <Q-NNN>` / `--qid-source <出处锚>`。
	//   `--qid` 是**缺口 ↔ 版账 Q 号**的对应格：`gap add` 写新行那一格、`gap ls` 拿它做反查轴；
	//   `--qid-source` 只配 `--qid`（无号无处落锚）。语义在两条命令里判（本排只管上户口）。
	//   ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--qid", "--qid-source":
		return a
	}
	// ★ 2026-10-23（缺口 `GAP-20260926-78` · `repo uncommitted`）：三枚取值旗标上户口 ——
	//   `--since <时长|RFC3339>` / `--me <提案号|人>` / `--audit <审计账>`。语义在 `cmdRepoUncommitted`
	//   里判（本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用）。
	switch a {
	case "--since", "--me", "--audit":
		return a
	}
	// 受控写面与提交面旗标（D3b 第二/三步 · 2026-09-21）：受控写入的件名、提交信息、
	// 审批的件名与理由 —— 与上面同一张名字表的口径（值照收，语义在各自命令里判）。
	// ★ 2026-09-28（缺口 `GAP-20260928-79` · 本枚）：续 `--allow-cross-root <理由>` 一枚 ——
	//   `dev edit` **异源拒写档的显式放行闸**（理由进审计行 · 语义在 `cmdDevEdit` 里判；
	//   本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用）。
	// ★ 本枚（main.go 开窗第 5 条）：`--add-file <路径>` —— `dev proposal revise` 的**补件**旗标
	//   （把仓内相对路径**加进**既有提案的 `files[]` · **可重复** · 幂等）；与 `--file`/`--replace` 同族 ⇒ 就近
	//   登记在本排。★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--file", "--message", "--proposal", "--tool", "--note", "--from", "--replace", "--replace-inline", "--replace-inline-b64", "--root",
		"--allow-cross-root", "--allow-sibling-repo", "--pre", "--removed", "--add-file":
		return a
	}
	// ★ 缺口 `GAP-20261010-16`（want③）：`--window-min <分钟>` —— `agent show` 两列的**窗口宽度**
	//   （默认 5 · 只收 1..1440；语义在 `cmdAgentShow` / `agentReqWindow` 里判）。与上面各排同一
	//   口径：★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--window-min":
		return a
	}
	// 产出树面旗标（`W-50` · 任务单序132 · 2026-09-24 波17）：`--tree <树>`（**可重复** ——
	// 一树一行、两代树两行）。与上面几排同一口径：值照收，语义在各自命令里判
	// （`publish tree has` 判「点没点名」「树在不在盘」「树身份读得到吗」）。
	switch a {
	case "--tree":
		return a
	}
	// T09/T11 正门两条的旗标（本枚 · **只增**）：与上面各排同一口径 —— 值照收，语义在各自命令里判。
	//   `gate selfreport diff`：`--source`（自报件）/ `--against`（独立对拍件）；
	//   `gate four-states`   ：`--current`（现役件命令串）/ `--patched`（改件命令串）/ `--yes-flag`（确认旗标名，缺省 `--yes`）。
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（照 `--allow-cross-root` 那一条的现读口径）。
	switch a {
	case "--source", "--against", "--current", "--patched", "--yes-flag":
		return a
	}
	// T20 正门一条的旗标（本枚 · **只增**）：`repo diffstat --report <件>` 的报告件。
	// 与上面各排同一口径 —— 值照收，语义在各自命令里判；★ 本排是名字表「唯一真源」⇒
	// 不登记就一律退 2「未知旗标」= 新旗标等于不可用（已现读实测：只加命令树格不加本表
	// ⇒ `zerg: 未知旗标 "--report"` rc=2）。
	switch a {
	case "--report":
		return a
	}
	// 门进度可查（设计-门进度可查-v0.3 §三/§十.6）：三枚**取值**旗标（值照收，语义在 family_gate_progress.go 里判）。
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--interval", "--silent-threshold", "--limit":
		return a
	}
	// ★ 本单（缺口 `GAP-20261017-10` · `fs` 族三条正门）：三枚**取值**旗标 ——
	//   `--offset N`（`fs read` 的起始行 · 1 起）/ `--content <文本>`（`fs write` 的内联内容）/
	//   `--cwd <目录>`（`fs run` 的工作目录 · 必须在仓内）。
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；
	//   用法串与本表**必须同改**（门73 判据乙/丙）。`--limit`/`--timeout`/`--by`/`--json` 已在既有各排
	//   （本排**不重开同名旗标** —— 一族共用一张名字表）。
	switch a {
	case "--offset", "--content", "--cwd":
		return a
	}
	// 公开面 CI 归绿面旗标（`W-51` · 任务单序133 · 2026-09-24 波18）：`--run <记录件>`（check-run
	// 记录件 = **人贴屏那张图**的机器替身）与 `--decl <声明件>`（必需集合的声明面 · 可省 ⇒ 走默认落点
	// `publish/ci/required-checks.json`）。与上面各排同一口径：值照收，语义在各自命令里判。
	// ★ `--run` 已在下面「测试作用域旗标」那一排（`dev test --run` 的包名过滤）—— **一行两用、
	//   不重开同名旗标**（一族共用一张名字表）；本排只续 `--decl` 这一枚新名字。
	switch a {
	case "--decl":
		return a
	}
	// 「显式指定机器」族旗标（2026-09-24 · 单独定制 > 路由默认规则）：`--machine <机器>`。
	// 与上面各排同一口径：值照收，语义在各自命令里判（`route pin` 判「给没给」，选机那道门判
	// 「这台是不是该模型的候选」）。★ 为什么**不**复用 `--node`：`--node` 是**对象级**旗标
	// （§九 M13 的远端语义 —— 它把整条命令发到那台机的主控去），而这一枚指的是**这次推理落哪台**，
	// 两件事混用会让「命令发到哪」与「请求落在哪」变成一个词。
	switch a {
	case "--machine":
		return a
	}
	// 虫族库面旗标（T2④ / T4① · 2026-10-14 · 本单）：`--db <库路径>` —— `core db` 的**库落点**
	// （不给 ⇒ `zdb.DefaultPath()`）。与上面各排同一口径：值照收，语义在 `family_core_db.go` 里判；
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--db":
		return a
	}
	// dev 族「基线回放」旗标（GAP-20261009-10 修法① · 本枚）：`dev edit --base <件路径|HEAD|sha>` ——
	// 用给的**基线内容**当 `--replace` 的匹配基（而不是读现盘件），并把该基线作为审计的 `before`
	// （真前态指纹）⇒ 已手改过盘的件（改前文本已不在盘上）也能把同一笔改动**回放**出真 before→after。
	// 与上面各排同一口径：值照收，语义在 `cmdDevEdit` 里判。
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--base":
		return a
	}
	// 名册面旗标（波① `T1a` · `Q-099`：`model add` 往 `gateway/fleet.yaml` 写一条）。
	// 与上面几排同一口径：值照收，语义在各自命令里判（`model add` 逐个校验）。
	switch a {
	case "--host", "--backend", "--ctx", "--mem-gb", "--size-bytes", "--arch", "--mmproj", "--added", "--cmd":
		return a
	}
	// ★ 本单（缺失项 `GAP-20260927-172` 之正解 · E1 `fleet render`）：`--roster <名册件>` —— 真源名册
	//   件路径（缺省 `~/.zerg/contexts/<档>.yaml`）。与上面各排同一口径：值照收，语义在 `cmdFleetRender` 里判。
	//   ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--roster":
		return a
	}
	// 融合面旗标（§十八.3 四件 · 批 E · T-59）：`ask` 的能力路由与 `plan`/`apply` 的落点。
	// ★ 加这一排的直接理由：`--out` 此前**只写在用法串里、解析器不认** ⇒ 真给就退 2
	//   （见表二「所见非本批」）—— 用法串里写着的旗标必须真能被解析，否则命令等于不可用。
	switch a {
	case "--capability", "--prefer", "--min-ctx", "--min-mem-gb",
		"--timeout", "--out", "--target-ref":
		return a
	}
	// `zerg chat` 族旗标（T3 · 2026-10-17 · 本枚 · **只增**）：三枚**取值**旗标 ——
	// `--session <id>`（续接既有会话 · `O-12`：只认**完整** id）· `--images <路径>`（可重复 ⇒ 多图）·
	// `--window <mid>`（`chat show` 的召回窗口锚）。
	// ★ 与上面各排同一口径：值照收，语义在 `family_chat.go` 里判；★ 本排是名字表「唯一真源」⇒
	//   不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例：`--allow-cross-root` 那一排）。
	// ★ 本族的**布尔**旗标（`--new`/`--archived`/`--no-stream`/`--markdown`/`--last-in-cwd`）
	//   **✗ 不进本表**：它们各写一条 `parseInvocation` 专用 `case`（门73 判据丙：布尔进名字表 =
	//   值/无值两态不一致；同族先例 `--no-fallback` 的那次形态纠正）。
	switch a {
	case "--session", "--images", "--window", "--prompt-file", "--as":
		return a
	}
	// ★ 2026-10-17（本枚 · `chat ask` / `chat send` 机器面）：`--prompt-file <路径>`（提示整件从盘上读）
	//   与 `--as <名>`（会话别名）都是**取值**旗标 ⇒ 落在上一排（与 `--session` 同表）。
	//   `--stream` 是**布尔**档 ⇒ **✗ 不进本表**，各写一条 `parseInvocation` 专用 `case`
	//   （门73 判据丙：布尔进名字表 = 值/无值两态不一致）。
	// ★ 2026-09-28（缺口 `GAP-20260928-127` 的 ⑥ · 本枚）：`--no-fallback` **从本表移出** ——
	//   它的语义本就是**布尔档**（用法串只写裸形 `[--no-fallback]`），按「取值」收进本表
	//   是**形态不统一**（门73 判据丙）。现在与 `--count`/`--files-only` 同一种形态
	//   （解析面布尔 case + 写进 `inv.kv`）⇒ 两处登记面**同名同形态**。
	//   ★ 2026-10-05（缺口 `GAP-20261005-68`）：`--no-fallback` 已**整体摘牌**（命令树用法串 + 解析面 case 双处移除）—— 本段仅记历史形态；真给该旗标今退 2「未知旗标」。
	// ★ 2026-10-11（本枚）：`--exit-on` 已**整体摘牌**（命令树用法串 + 解析面 case 双处移除）
	//   —— 该旗标全库**零消费者**（`cmdWatch` 从不读它），此前只「认得」不生效。摘牌依据＝
	//   明文口径「零消费者 ⇒ 摘牌」的先例（同族 `ask --no-fallback`）。本段仅记历史形态；
	//   真给该旗标今退 2「未知旗标」。
	// 度量与排序面旗标（组1 序12 · `zerg metrics`）：读数档的落点。
	// 与上面几排同一口径：值照收，语义在命令里判（本命令只把它当一个**只读**的件路径）。
	switch a {
	case "--input":
		return a
	}
	// 取证面旗标（缺口 `GAP-20260927-233` · 2026-10-04）：`code show --batch <件:行,…>` 批量取件:行。
	// 与上面各排同一口径：值照收，语义在 `cmdCodeShowBatch` 里判（值与位置参数二选一、空项不收）。
	// ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	// ★（本单 · 缺口 `GAP-20260928-65` 的**权威判据**半）：续 `--from-file <件>` —— `dev proposal new` 的
	//   **单片档可机读声明件**（读一枚 JSON **对象** = 一片提案的字段）。与 `--batch` 同族（都从**盘上**读
	//   一枚 JSON 件），故**就近登记**在本排（✗ 不新开段）。值照收，语义在 `cmdDevProposalNewFromFile` 里判；
	//   本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。用法串与本表**必须同改**
	//   （门73 判据乙/丙：名字只在名字表出现 = 乙「多一处」；用法串只写裸形 = 丙「形态」）。
	switch a {
	case "--batch", "--from-file":
		return a
	}
	// 文档面旗标（缺口 `G-19` 同根：`--docs-ver <X.Y.Z>` 显式钉一版 = 兼容「钉死在某一版」的旧行为；
	// 默认档才是版本无关。`help export` 与 `doc meta fill` 共用同一枚 —— 取源只有一处 `devdocs.go`）。
	// ★ `A3` 的 `b` 案在这里续一枚 `--docs-root <Zerg-内部文档 根>`（设计稿 §二「落点白名单」第 2 条：
	//   `--docs-root` **只换仓根**、不许换件名 —— 形状先例 = 门㉑ 与 `zerg repo status --root <仓根>`）。
	switch a {
	case "--docs-ver", "--docs-root":
		return a
	}
	// 回执面旗标（§20.3 `H3` · 批 E · T-61）：一轮一页回执的五格 + 证据/提交。
	switch a {
	case "--what", "--next", "--blockers", "--trace", "--commit":
		return a
	}
	// 候选区旗标（§17.4 第 2/3/5 条 · 批 E · T-58）：作用域 / 结果表 / 证据单的三格可选键。
	// ★ 同上面 `--out` 的教训：用法串里写着的旗标必须真能被解析，否则命令等于不可用。
	switch a {
	case "--scope", "--results", "--code-sha", "--layer", "--outdir":
		return a
	}
	// 授权面旗标（§九 M18 `C4` · 批 E · T-60）：提出者 / 批准者**成对**出现（「提 ≠ 批」两个字段）。
	switch a {
	case "--subject", "--subject-kind", "--egg-id", "--approver", "--approver-kind":
		return a
	}
	// 升阶闸门旗标（§20.4 · 批 E · T-62）：`--human-approval` 是**人拍板**那一格的入口。
	switch a {
	case "--human-approval":
		return a
	}
	// 回收面旗标（§十五.2 · 批 D · T-54）：`--min-age-days` 是 `RC6` 的**显式**龄阈值（不许魔数）。
	switch a {
	case "--min-age-days", "--only":
		return a
	}
	// 提交面例外旗标（`Q-104` · 2026-09-23 已拍：开「带审计的显式例外旗标」）：
	//   `--waive <步名>`（可重复 —— 一次可点多个要豁免的步名）与 `--reason <理由>`（缺它 ⇒ 拒执 2）。
	// ★ 它与 `--no-verify` 是**两件事**：命令面**不提供**绕行；这个旗标只把「红」变成**可回读**的记账。
	switch a {
	case "--waive", "--reason",
		// ★ 门新鲜度闸的**留痕跳道**（缺口 `GAP-20260928-194` · 设计稿《流程规则程序化 v1.0》§2.2 A2/A5）：
		//   `--allow-stale-gates <一句话理由>` —— **不传理由的裸跳不许存在**（语义在 `gateFreshnessGate`
		//   里判：理由空 ⇒ 拒 2；理由非空 ⇒ 真留一笔痕，落不下痕也不许跳）。名字表是本仓「唯一真源」
		//   ⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例逐字同上）。
		"--allow-stale-gates":
		return a
	}
	// 影响面实测回填（`B3` · 2026-09-22）：`--gate-results` 是**那次门禁的结果表**（或它的日志目录）——
	// ★ 回填的输入**必须**是那次门禁自己的账（三数要能按同一份表复算 ⇒ 输入得能指名到件）。
	// 与上面各排同一口径：值照收，语义在 `zerg impact` 里判；不用的命令静默忽略。
	switch a {
	case "--gate-results":
		return a
	}
	// 测试作用域旗标（缺口面 P0-3 · 2026-09-21）：`dev test` 只跑相关那几个测的两枚旗标。
	switch a {
	case "--pkg", "--run":
		return a
	}
	// 门禁解释/标定旗标（缺口面 P0-5/P0-8 · 2026-09-21）：矩阵导出落点与重复趟数。
	switch a {
	case "--repeat", "--step":
		return a
	}
	// 码面只读旗标（缺口面 P0-1 · 2026-09-21）：`code find` 的两枚收窄旗标。
	// ★ 与上面 `--out` 同一条教训：用法串里写着的旗标必须真能被解析，否则命令等于不可用。
	// ★ 缺口 `GAP-20260927-250/251`（本枚 · **只增**）：续 `--after`/`--before`（命中行上下文半径 ·
	//   值 = 非负整数）与 `--kind <代码|注释>` 分流面。与上面各排同一口径 —— 值照收，语义在
	//   `cmdCodeFind` 里判；★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	//   短形 `-A`/`-B`/`-C` 与 `-i` 在 `parseInvocation` 里另起 case（短形不经过本函数）。
	switch a {
	case "--path", "--glob", "--after", "--before", "--kind":
		return a
	}
	// 码面 R5 第一枚新旗标（缺口 `GAP-20260927-237`）：`code callers --func <函数名>` 的反查入口。
	// 同上面各排同一口径 —— 值照收，语义在 `cmdCodeCallers` 里判（与位置参数两收、后者优先）。
	// ★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
	switch a {
	case "--func":
		return a
	}
	// 按名找件旗标（`zerg find` · 2026-09-25 父代理补登）：实现件早已会读这三枚（family_find.go 走 flagVal），
	// 只差在名字表上户口 —— 此前用法串写着 `--limit/--type/--root` 却一律退 2「未知旗标」= 命令等于不可用。
	switch a {
	case "--root", "--type", "--limit":
		return a
	}
	// `agent` 族第二组旗标（缺口 `GAP-20260925-50` 的 ②③④ · 2026-09-26）：`--add <模型名>`
	//   （`agent registry` 的写面：往子端注册表加一条）与 `--n-predict <N>`（`agent bench` 的生成上限）。
	//   ★ `--list` 是**布尔**旗标（在下面布尔那一排）；`--file`/`--ctx`/`--mem-gb`/`--model` 已在上面的
	//   名册面那一排 —— 本族**不重开**同名旗标（一族共用一张名字表）。
	switch a {
	case "--add", "--n-predict":
		return a
	}
	// 缺口族旗标（`gap` 族 · 设计稿 `设计-命令面-gap族-v1.0-20260923.md` §二）：形状面 7 枚 + 判据面 1 枚。
	//   `--want-argv` 可重复（append）；`--state` / `--depends-on` / `--by` / `--dry-run` / `--yes` / `--json`
	//   已在上面各排（本族**不重开**同名旗标 —— 一族共用一张名字表）。
	//   ★ 2026-09-26（缺口账 `Q-239` 两条状态写面）：`--evidence` 已在「自开发面」那一排（值照收，
	//     语义在命令里判）；本排只续 `--text` 这一枚新名字（`gap note` 的那句话）。
	//   ★ 2026-09-28（缺口账 `GAP-20260926-233`）：再续 `--retract <n|指纹>` 一枚（作废一注 · 不删原文）。
	switch a {
	case "--symptom", "--handmade", "--impact", "--want-family", "--want-action",
		"--want-argv", "--prio", "--repro-cmd", "--verify-cmd", "--text", "--retract",
		// ★ GAP-20260928-95（本枚）：`--void` —— `dev proposal --retract/--void <提案 id>` 作废档的**别名**
		//   （与 `--retract` 同表同口径；名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 该别名等于不可用）。
		"--void",
		// ★ 批1 第三片：`gap ls` 两轴收窄 —— `--unit <件路径|目录前缀>` / `--module <模块前缀>`。
		//   名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例：
		//   `--allow-cross-root` 那一排的注释逐字同病）。用法串与本表**必须同改**（门判据乙）。
		"--unit", "--module", "--grep",
		// ★ 批1 第四片：`gap status` 排行面 —— `--top <N>`（一页桶数 · 缺省 20 · 上限同 `gapLsRowCap`）。
		//   `--by <unit|module>` **已在上面「自开发面」那一排**（与 `dev proposal` 等共用一张名字表）⇒
		//   本排只续 `--top` 这一枚新名字（一族共用一张名字表 · **不重开同名旗标**）。
		//   ★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
		"--top",
		"--bucket",
		// ★ 批2 第一片（2-2）：`gap idea add|ls` 的 `--source <guard|gate|egg|human>`（四值闭集 · 逐字照
		//   设计稿 §9.1 第 2 条）。名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用。
		// ★ 批3 第一片（2026-09-28）：`gap assign` 的作业单六栏五枚旗标 + 回执路径一枚。
		//   名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例同上）。
		"--egg", "--allow", "--forbid", "--occupies", "--criterion", "--deadline", "--receipt",
		// ★ 缺口账 `GAP-20260928-184` 第三面（2026-10-10）：`gap receipt new` 的三枚值旗标 ——
		//   `--reading <缺口号=结果>`（逐条读数 · 可重复）· `--conclusion <结论>` · `--closed-id <GAP id>`（可重复）。
		//   名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；用法串与本表**必须同改**（门73）。
		"--reading", "--conclusion", "--closed-id",
		// ★ 批4 第四片（2026-09-28）：`gap set-state` 销案**两态对拍**的两枚读数旗标
		//   （改前 / 改后成对 · 值 = 同族 `k=v · k=v` 逐格串）。名字表是本仓「唯一真源」
		//   ⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例逐字同上）。
		"--before-read", "--after-read",
		// ★ GAP-20260928-196（甲 · 2026-10-10）：`gap set-state` 销案两态读数的**自动跑**两枚 ——
		//   `--before-cmd <命令串>` / `--after-cmd <命令串>`（CLI 自己跑、把 `cmd=/rc=/reading=` 三格
		//   按**真退码**填好 · 唯一 resolver 见 `family_gap_state.go:gapTwoStateResolve`）。名字表是本仓
		//   「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；用法串与本表**必须同改**（门73）。
		"--before-cmd", "--after-cmd",
		"--source",
		// ★ R1 登记（2026-10-04 · 各路停摆等 main.go 的登记面）：三枚值旗标 —
		//   `--gate-step`/`--log-path`/`--rc`（`gap idea add` 的候选池**门来源三格** · 缺口
		//   `GAP-20260928-178`：门来源的「触发子」要能按门聚合 ⇒ 池行带结构化三格）；
		//   `--supersede <n>`（`gap note` 的**替代注** · 缺口 `GAP-20260928-58`：现只 `--retract`
		//   作废、不能给替代 ⇒ 救场要两步）。同上面各排同一口径 —— 值照收，语义在各自
		//   `family_gap*.go` 里判（读者由 R2 同批落）。★ 名字表是本仓「唯一真源」⇒ 不登记就
		//   一律退 2「未知旗标」= 新旗标等于不可用（同族先例逐字同上）。用法串与本表**必须同改**（门73）。
		"--gate-step", "--log-path", "--rc", "--supersede",
		// ★ 2026-10-09（缺口账 `GAP-20261008-06`/`-07`）：`--unit-source <auto|hand>` ——
		//   `gap set-unit` 的件面三键之一（`unit_source`）。名字表是本仓「唯一真源」⇒
		//   不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例逐字同上）。
		// ★ 缺口 `GAP-20260927-259`（本枚）：续 `--notes` —— `gap add` / `gap idea promote`
		//   **建条时顺带落注**的第二种写法（与既有 `--note` **同义**；`--evidence` 亦同义 ⇒
		//   三枚都落 `notes` 那一格）。名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」=
		//   新旗标等于不可用；用法串与本表**必须同改**（门73）。语义在 `gapAddNotesFromFlags` 里判。
		"--notes", "--unit-source":
		return a
	}
	// 内部任务周期写面旗标（P0 #232 · 2026-10-08 · 本枚 · **只增**）：`itask interval set <任务|all> --hours N`。
	// 与上面各排同一口径 —— 值照收，语义在 `cmdItaskIntervalSetGuarded` 里判。
	// ★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；配套 dispatch 收口
	//   （不归本命令的那一枚补回 `inv.unknown`，既有命令一字不动）。
	switch a {
	case "--hours":
		return a
	}
	// `agent` 族调度旋钮写面旗标（P0 #232 · 2026-10-08 · 本枚 · **只增**）：
	//   `agent workers set <机> --max N` 与 `agent scheduler set --poll-interval/--max-retries/--cooldown`。
	// 与上面各排同一口径 —— 值照收，语义在 `cmdAgentWorkersSetGuarded` / `cmdAgentSchedulerSetGuarded` 里判。
	// ★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；配套 dispatch 收口
	//   （不归本命令的那一枚补回 `inv.unknown`，既有命令一字不动）。
	switch a {
	case "--max", "--poll-interval", "--max-retries", "--cooldown":
		return a
	}
	// ★ 批 B2（2026-10-20 · `zerg sched run` 归入命令面）：三枚**取值**旗标 ——
	//   `--workdir <目录>`（工作目录 · 源件 `-workdir` 的逐字保留）· `--agent-bin <路径>`
	//   （容器模式挂载的 Linux zerg-agent）· `--repo <路径>`（DevMode 的开发仓库）。
	// 与上面各排同一口径：值照收，语义在 `family_sched_run.go:schedRunParse` 里判。
	// ★ 名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；
	//   用法串与本表**必须同改**（门73）；配套 dispatch 收口（不归本命令的那一枚补回 `inv.unknown`）。
	switch a {
	case "--workdir", "--agent-bin", "--repo":
		return a
	}
	// ★ 批 B5（2026-10-20 · `zerg agent run` 归入命令面）：**九枚取值旗标** ——
	//   `--task <任务描述>`（与 `--issue` 二选一）· `--issue <问题单>` · `--tools <子集>` ·
	//   `--mcp <spec>` · `--max-turns <N>` · `--retry <N>` · `--gateway <URL>` ·
	//   `--x3-status <URL>` · `--x3-token <令牌>`（后两枚 = 机器级采样的环境变量兜底）。
	//   与上面各排同一口径：值照收，语义在 `family_agent_run.go:agentRunParse` 里判。
	//   ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；
	//   用法串与本表**必须同改**（门73 判据乙/丙）；配套 dispatch 收口（不归本命令的那一枚
	//   补回 `inv.unknown`）。★ `--model` / `--workdir` / `--state` / `--json` 已在既有各排
	//   （一族共用一张名字表 ⇒ 本排**不重开同名旗标**）。
	switch a {
	case "--task", "--issue", "--tools", "--mcp", "--max-turns", "--retry",
		"--gateway", "--x3-status", "--x3-token":
		return a
	}
	// ★ 批 B6（2026-10-20 · `zerg model record verify|probe|ls` 归入命令面）：**七枚取值旗标** ——
	//   `--base-dir <目录>`（verify 的 files[] 基准目录 · 源件 `--base-dir`）· `--store-root <目录>`
	//   （probe 的模型目录根 · 源件 `--store-root`）· `--endpoint <URL>` / `--engine <引擎>` /
	//   `--id <id>` / `--parent <上一版>`（probe 的探测面与血缘面 · 源件逐字）· `--manifests <目录>`
	//   （ls 直接指 manifests 目录 · 源件 `--manifests`）。
	//   与上面各排同一口径：值照收，语义在 `family_model_record.go` 里判。
	//   ★ 本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用；
	//   用法串与本表**必须同改**（门73 判据乙/丙）；配套 dispatch 收口（不归本命令的那一枚
	//   补回 `inv.unknown`）。★ `--root` / `--base` / `--out` / `--timeout` / `--json` 已在既有各排
	//   （一族共用一张名字表 ⇒ 本排**不重开同名旗标**）；三枚**布尔**（`--integrity` / `--in-place` /
	//   `--store`）**✗ 不进本表**（门73 判据丙：布尔进名字表 = 值/无值两态不一致）。
	switch a {
	case "--base-dir", "--store-root", "--endpoint", "--engine", "--id", "--parent", "--manifests":
		return a
	}
	// ★ 批 B7（2026-10-20 · `zerg compat list|check|migrate|manifest` 归入命令面）：**两枚取值旗标** ——
	//   `--state-dir <目录>`（状态目录 · 源件 `--state-dir`）· `--receipts-dir <目录>`（升级回执目录 ·
	//   源件 `--receipts-dir`）。源件另两枚 `--dry-run` / `--json` 是**解析层专用字段**
	//   （`inv.dryRun` / `inv.jsonGiven`）⇒ ✗ 不进本表（门73 判据丙：布尔进名字表 = 值/无值两态不一致）。
	//   与上面各排同一口径：值照收，语义在 `family_compat.go` 里判（`--state` 是另一枚既有旗标，
	//   ✗ 不许拿它的子串去「证明」`--state-dir` 已登记 —— 实现在入口处按**精确名**点名）。
	switch a {
	case "--state-dir", "--receipts-dir":
		return a
	}
	return ""
}

// setValueFlag 把一枚动作面旗标的值收进 invocation（可重复的两枚用 append）。
func (inv *invocation) setValueFlag(name, val string) {
	switch name {
	case "--desc":
		inv.desc = val
	case "--model":
		inv.modelWant = val
	case "--priority":
		inv.priority = val
	case "--slice-id":
		inv.sliceID = val
	case "--depends-on":
		inv.dependsOn = append(inv.dependsOn, val)
	case "--acceptance":
		inv.acceptanceDeclared = true
		if val != "" {
			inv.acceptance = append(inv.acceptance, val)
		}
	case "--to":
		inv.to = val
	case "--set":
		inv.setPairs = append(inv.setPairs, val)
	default:
		// 自开发面旗标：进 kv（`flagVal`/`flagVals` 是唯一读法）。
		inv.kvSet(name, val)
	}
}

// boolCount 数一数有几个 true（三档互斥判据用）。
func boolCount(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

func splitFields(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// requireFields 统一处理「--json 不给字段」的**提示面**（契约 §4.1 K2）：stdout 0 字节、字段清单走 stderr。
//
// ★ `K2` 甲档归一（2026-09-24 · 批四 §二 #7 · 块B `R-1`/`R-2` · 一笔成片）：**码由本函数回**
// （原先是 `bool`，码散在 36 个调用点各自 `return exitFail` ⇒ 表说「用法错 = 2」、实现写 1，
// 同一错误全族不同码也没人会报）。现在：
//
//	满足   ⇒ `exitOK`
//	不满足 ⇒ `exitUsage`（**只此一处**决定 —— 调用点写不出第二种码，编译期就能看见）
//
// 调用点一律 `if rc := requireFields(inv, stderr); rc != exitOK { return rc }`。
// 归一方向**写死**：退码表（`exitcodes.go`）**不动**、本函数与自描述面**取自表**（不是表跟实现走）。
func requireFields(inv *invocation, stderr io.Writer) int {
	if len(inv.fields) > 0 {
		return exitOK
	}
	fmt.Fprintf(stderr, "%s: --json 需要逗号分隔的字段列表\n", progName)
	fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(fieldListOf(inv.path), ","))
	// ★ 2026-09-24（缺口 `Q-070` · 组3 §一 序52 · 波10 序83）：§4.1 K14 四件套的「下一步」那一件。
	//   光列字段表 = 指出**哪里错**；补一句**可照抄**的例（取该条字段表第一格）⇒ 推到「知道怎么改」。
	//   为什么取第一格而不写死某个字段名：字段表是命令树的真源，写死会漂（`Q-155` 同族那条教训）。
	if fs := fieldListOf(inv.path); len(fs) > 0 {
		fmt.Fprintf(stderr, "下一步：从上面「可选字段」里挑逗号分隔的名字，例：`%s %s --json %s`\n",
			progName, strings.Join(inv.path, " "), fs[0])
	}
	// ★ 2026-09-27（缺口 `GAP-20260927-360`）：错误面**机器可读**（§九 M7）。本函数原只写 stderr、不调
	//   `setErr` ⇒ 危险档走到这里时包封里没有 `error.detail`，退化成「（命令未报出 kind，按退码兜底）」。
	//   与 gap 族 6 处（`family_gap.go:558/726/990` · `family_gap_export.go:50` · `family_gap_state.go:223/356`）
	//   逐字同 kind 同 detail。人面一字不动（stderr 文案仍是上面那几句）。
	inv.setErr("usage", "json_fields_required", "--json 不给字段")
	return exitCodeOf("usage")
}

// fieldListOf 取某条命令的可选字段（供 K2 / I5 的自描述面用）。
func fieldListOf(path []string) []string {
	if c := find(path); c != nil {
		return c.fields
	}
	return nil
}

// marshalObject 按「用户给的字段序」拼一个 JSON 对象；点到不存在的字段 ⇒ bad = 那个字段名。
func marshalObject(fields []string, row map[string]string) (obj string, bad string) {
	var b strings.Builder
	b.WriteString("{")
	for i, f := range fields {
		v, ok := row[f]
		if !ok {
			return "", f
		}
		if i > 0 {
			b.WriteString(",")
		}
		k, _ := json.Marshal(f)
		val, _ := json.Marshal(v)
		b.Write(k)
		b.WriteString(":")
		b.Write(val)
	}
	b.WriteString("}")
	return b.String(), ""
}

// marshalObjectRegistered 按「用户给的字段序」拼一个 JSON 对象；字段**已在登记面**但**本行缺值** ⇒
// 该键**整键略去**（✗ 不落 `0`/空串）。与 `marshalObject` 的分工：后者是 `dev edit` 干跑口的
// 「本档必不产出 ⇒ 写前拒」探针（`family_dev_edit.go:676`），**缺键即 bad**（写面前置不许放宽）——
// 两者不同语义，故不得合并或改后者；本函数只服务 `emitSelected` 这条 `--json` 出口
// （「在册缺值」是**合法结果**，只有**不在册**的音由 `emitSelected` 的白名单校验拦下）。
func marshalObjectRegistered(fields []string, row map[string]string) string {
	var b strings.Builder
	b.WriteString("{")
	n := 0
	for _, f := range fields {
		v, ok := row[f]
		if !ok {
			continue // 在册但本行缺值 ⇒ 整键略去（✗ 不落 0）
		}
		if n > 0 {
			b.WriteString(",")
		}
		k, _ := json.Marshal(f)
		val, _ := json.Marshal(v)
		b.Write(k)
		b.WriteString(":")
		b.Write(val)
		n++
	}
	b.WriteString("}")
	return b.String()
}

func reportBadField(stderr io.Writer, path []string, bad string) int {
	if bad == "*" {
		// §4.1 K1：必须给**逗号分隔的字段名**；通配不在契约里（这是「点名字段」的机器面）。
		fmt.Fprintf(stderr, "%s: `--json *` **不存在** —— 本契约要的是**逗号分隔的字段名**（§4.1 K1）\n", progName)
	} else {
		fmt.Fprintf(stderr, "%s: 未知字段 %q\n", progName, bad)
	}
	fmt.Fprintf(stderr, "合法字段: %s\n", strings.Join(fieldListOf(path), ","))
	fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
	return exitUsage
}

// contractSchema —— 包封第一键的取值（§九 M6 / §十五.7 定案：只写 `zerg/v1`，**不写** `schema_version`）。
const contractSchema = "zerg/v1"

// envelopeKeys —— 包封**顶层六键**的唯一真源（§九 M6）。
//
// 为什么要抽出来：形状守卫（`TestCLIContractJSONShape`）与只读桥（`EnvelopeKeysForTest`）今天
// 各写一遍键名 —— 两份清单就会漂（一处加了第七键、另一处照旧绿）。本表是**唯一**一份。
var envelopeKeys = []string{"schema", "kind", "items", "meta", "warnings", "truncated"}

// envMetaKV —— `meta` 的**按需子键**一格：`Val` 是**已序列化的 JSON 值**（字符串/布尔/数组都行）。
//
// 口径（本批新立 · 三条）：
//  1. **旧子键语义不变**：`count`/`source`/`changed`/`node`/`idempotency_key` 的名字、类型、取法
//     一个字不动 ⇒ 同名子键**不许从这个口子写**（`envMetaReserved` 拒收，见 `emitEnvelopeWith`）。
//  2. **只让已有键有值**：本结构只装 `meta` 里的东西，**不新增第七个顶层键** ✗。
//  3. **不编造**：没有真值就**不写这一格**（缺席 ≠ 空数组/假值；旧子键照旧恒在）。
type envMetaKV struct {
	Key string
	Val string // 已序列化的 JSON 值（调用方负责用 jstr / json.Marshal 出合法 JSON）
}

// envMetaReserved —— `meta` 里的**旧子键**（不许从 `envMeta` 覆盖写；语义铁律的机械落点）。
var envMetaReserved = []string{"count", "source", "changed", "node", "idempotency_key"}

// warnf —— 记一条 `warnings[]` 真值（**只在真有事时调**；拿不到真值就不许调 —— 缺口留白，不装）。
func (inv *invocation) warnf(format string, a ...any) {
	if inv == nil {
		return
	}
	msg := strings.TrimSpace(fmt.Sprintf(format, a...))
	if msg == "" {
		return // 空串不是信号（不编造）
	}
	inv.envWarn = append(inv.envWarn, msg)
}

// markTruncated —— `truncated=true` 的**唯一**置位口（只在**真裁了条目**时调；没裁一律 `false`）。
func (inv *invocation) markTruncated() {
	if inv == nil {
		return
	}
	inv.envCut = true
}

// metaAddJSON —— 写一格 `meta` 的按需子键（值自带 JSON 形态）；旧子键名一律拒收（不静默覆盖）。
func (inv *invocation) metaAddJSON(key, valJSON string) {
	if inv == nil {
		return
	}
	key = strings.TrimSpace(key)
	valJSON = strings.TrimSpace(valJSON)
	if key == "" || valJSON == "" {
		return
	}
	for _, r := range envMetaReserved {
		if key == r {
			return
		}
	}
	inv.envMeta = append(inv.envMeta, envMetaKV{Key: key, Val: valJSON})
}

// metaAddStr / metaAddStrings —— 字符串与字符串数组两枚便捷口（值一律走 `jstr` 转义）。
func (inv *invocation) metaAddStr(key, val string) {
	if strings.TrimSpace(val) == "" {
		return
	}
	inv.metaAddJSON(key, jstr(val))
}

func (inv *invocation) metaAddStrings(key string, vals []string) {
	if len(vals) == 0 {
		return
	}
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) == "" {
			continue
		}
		parts = append(parts, jstr(v))
	}
	if len(parts) == 0 {
		return
	}
	inv.metaAddJSON(key, "["+strings.Join(parts, ",")+"]")
}

// envelopeWarningsJSON —— `warnings[]` 的**真值渲染**（纯函数 · 判定口正/负控都调它）。
// 空（含只有空串/空白的那些）⇒ 逐字 `[]`：**无事就是空**，不许填充 ✗。
func envelopeWarningsJSON(warns []string) string {
	out := []string{}
	for _, w := range warns {
		if strings.TrimSpace(w) == "" {
			continue
		}
		out = append(out, jstr(w))
	}
	return "[" + strings.Join(out, ",") + "]"
}

// envelopeTruncatedJSON —— `truncated` 的**真值渲染**：只有**真裁了条目**才是 `true`。
func envelopeTruncatedJSON(cut bool) string {
	if cut {
		return "true"
	}
	return "false"
}

// truncatedDetailCutFromSet —— `meta.truncated_detail.cut_from` 的**三值闭集**（块D `K-1` ·
// `O-10` 逐字「只给『从哪砍』的三值枚举、**不给下标**」—— 与「`items[]` 顺序不保证」相容）。
var truncatedDetailCutFromSet = []string{"head", "middle", "tail"}

// truncatedDetailKeys —— `meta.truncated_detail` 的**四键真源**（判定口与自检都读这一份，不另抄）。
var truncatedDetailKeys = []string{"cut_from", "kept_items", "dropped_items", "total_items"}

// truncatedDetailJudge —— `meta.truncated_detail` 的**唯一判定口**（喂整个包封文本 · 只判不写）：
//
//	① **与 `truncated` 成对**：`true` ⇒ 本格必须在（**只给布尔 ⇒ 红**）；`false` ⇒ 本格必须缺席
//	   （没裁却给计数 = 「余量不是裁了」那一类的假值 ✗）；
//	② **四键齐、一个不多一个不少**，`kept_items` / `dropped_items` / `total_items` 都是**整数**；
//	③ **三数自校**：`kept_items + dropped_items == total_items`，且真裁时 `dropped_items ≥ 1`；
//	④ `cut_from` 在**三值闭集**内（闭集外的值 ⇒ 红）。
//
// 抽出来为什么：负控要能直接喂「只给布尔」的**合成**包封（真跑只覆盖到「本机今天恰好会裁的件」）
// ⇒ 判定口与真跑两路读同一份判据（照 `impactJudgeCard` / `cli_envelope_truth_test.go` 的先例）。
func truncatedDetailJudge(envJSON string) error {
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		return fmt.Errorf("包封不是 JSON：%v", err)
	}
	cut := false
	if raw, ok := env["truncated"]; ok {
		if err := json.Unmarshal(raw, &cut); err != nil {
			return fmt.Errorf("`truncated` 不是布尔：%v", err)
		}
	}
	var meta map[string]json.RawMessage
	if raw, ok := env["meta"]; ok {
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Errorf("`meta` 不是对象：%v", err)
		}
	}
	raw, has := meta["truncated_detail"]
	if !cut {
		if has {
			return fmt.Errorf("没裁却写了 `meta.truncated_detail`（`truncated=false` 时本格必须缺席 —— 缺席 ≠ 假值）")
		}
		return nil
	}
	if !has {
		return fmt.Errorf("`truncated=true` 却只给布尔、没有 `meta.truncated_detail`（块D `K-1`：只给布尔 ⇒ 判红）")
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("`meta.truncated_detail` 不是对象：%v", err)
	}
	if len(d) != len(truncatedDetailKeys) {
		return fmt.Errorf("`meta.truncated_detail` 键数 = %d（要 %d · 四键一个不多一个不少）：%s",
			len(d), len(truncatedDetailKeys), string(raw))
	}
	for _, k := range truncatedDetailKeys {
		if _, ok := d[k]; !ok {
			return fmt.Errorf("`meta.truncated_detail` 缺键 `%s`", k)
		}
	}
	var cutFrom string
	if err := json.Unmarshal(d["cut_from"], &cutFrom); err != nil {
		return fmt.Errorf("`cut_from` 不是字符串：%v", err)
	}
	inSet := false
	for _, v := range truncatedDetailCutFromSet {
		if cutFrom == v {
			inSet = true
		}
	}
	if !inSet {
		return fmt.Errorf("`cut_from` = %q 不在三值闭集 %v 内", cutFrom, truncatedDetailCutFromSet)
	}
	nums := map[string]int{}
	for _, k := range []string{"kept_items", "dropped_items", "total_items"} {
		var n int
		if err := json.Unmarshal(d[k], &n); err != nil {
			return fmt.Errorf("`%s` 不是整数：%v（%s）", k, err, string(d[k]))
		}
		nums[k] = n
	}
	if nums["kept_items"]+nums["dropped_items"] != nums["total_items"] {
		return fmt.Errorf("三数不自校：kept_items %d + dropped_items %d ≠ total_items %d",
			nums["kept_items"], nums["dropped_items"], nums["total_items"])
	}
	if nums["dropped_items"] < 1 || nums["total_items"] <= 0 {
		return fmt.Errorf("`truncated=true` 却 dropped_items = %d / total_items = %d（裁了就得说裁了几条）",
			nums["dropped_items"], nums["total_items"])
	}
	return nil
}

func jstr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// emitEnvelope 出**统一外层包封**（§九 M6：六键 `schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）。
//
//	· `items` **恒数组**、空为 `[]`、**永不为 `null`**（I3）
//	· `kind` 与命令一一对应、单数 CamelCase（I2）
//	· `meta.source` 写清这份数据从哪来（远端端点 / 本机）；将来 `meta.node` 装多机目标（M13）
func emitEnvelope(stdout io.Writer, cmd *command, itemsJSON string, count int) {
	emitEnvelopeWith(stdout, cmd, itemsJSON, count, nil)
}

// emitEnvelopeWith 是 emitEnvelope 的完整形态：多一块 `meta.changed` 与 `meta.idempotency_key`
// （§九 M4：`--json` 必须给 `changed`；幂等键落在 meta —— **不**动外层六键，守住 T-06 的包封面）。
//
// ★ 本批（`G-08` · 解禁「不改 `emitEnvelope*`」）：**只让已有键有值** ✗ —— 顶层**仍是那六键**
// （一个不多一个不少 · 真源 = `envelopeKeys`），变的只是 `warnings[]` / `truncated` 两个既有键
// 的**真值** 与 `meta` 的**按需子键**：
//
//	· `warnings[]`   ← `inv.envWarn`（命令**自己判定出**的真事逐条）；空 ⇒ 逐字 `[]`
//	· `truncated`    ← `inv.envCut`（**真裁了条目**才 true；没判定过 ⇒ `false`）
//	· `meta`         ← 先写五个**旧子键**（取法一个字未动），再按需追加 `inv.envMeta`
//	                   （`metaAddJSON` 已拒收旧子键名 ⇒ 同名子键**不许**被覆盖）
//
// **不编造**（本批第二条铁律）：没有真值就**保持 `[]`/`false`/缺席** —— 不许用 `warnings[]` 装
// 「我以为」，也不许把「没跑 / 没取数」写成「没有」（那两件事分别由上一条命令自己点名）。
//
// ★ **包封可选扩展的裁定句**（2026-09-24 · 组1 序25 · 缺口 `G-110` · 照 `O-10` + `O-1`）：
//
//	**顶层六键冻结** —— 真源 `envelopeKeys`（`schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）
//	**一个不多一个不少**（`O-1`（批一）逐字：**冻结顶层 / 放开子键**）。
//	本面**不加第七键** ✗ —— `O-10`（批二）逐字：`truncated` 的计数**走 `meta.truncated_detail`**。
//	截断细节（`cut_from` / `kept_items` / `dropped_items` / `total_items`）**只从 `meta` 子键出**；
//	子键名由 `envMetaReserved` 挡住「覆盖旧子键」，真值由命令自己判定后经 `inv.envMeta` 追加。
//	⇒ 所以「**第七键前置**」这一条**不需要落**：它要解决的问题（`truncated = true` 之后看不见
//	「砍了多少」）已由 `meta.truncated_detail` 同一处承接（同向于块D `K-6`：**「升格」不是新造**）。
//	★ 本笔**只增注释、零行为变更** ✓（六键、子键拒收表、`emitEnvelope` / `emitEnvelopeWith` 的
//	取法**一个字未动** ✗）。
func emitEnvelopeWith(stdout io.Writer, cmd *command, itemsJSON string, count int, inv *invocation) {
	src := cmd.endpoint
	if src == "" {
		src = "local（本机）"
	}
	changed := "false"
	key := ""
	if inv != nil && inv.changed != nil {
		changed = fmt.Sprintf("%t", *inv.changed)
	} else if cmd.idem != nil {
		changed = fmt.Sprintf("%t", cmd.idem.Changed && cmd.danger != nil)
	}
	if inv != nil {
		key = idempotencyKey(inv)
	}
	meta := fmt.Sprintf("\"count\":%d,\"source\":%s,\"changed\":%s", count, jstr(src), changed)
	if inv != nil && len(inv.nodes) > 0 {
		meta += ",\"node\":" + jstr(strings.Join(dedupe(inv.nodes), ","))
	}
	if key != "" {
		meta += ",\"idempotency_key\":" + jstr(key)
	}
	// 按需子键：只追加**非旧子键名**（`metaAddJSON` 已在入口拒收；这里再挡一道 —— 双保险）。
	if inv != nil {
		for _, kv := range inv.envMeta {
			reserved := false
			for _, r := range envMetaReserved {
				if kv.Key == r {
					reserved = true
					break
				}
			}
			if kv.Key == "" || kv.Val == "" || reserved {
				continue
			}
			meta += "," + jstr(kv.Key) + ":" + kv.Val
		}
	}
	extra := ""
	if inv != nil && inv.err != nil {
		extra = ",\"error\":" + inv.err.errJSON()
	}
	warns, trunc := "[]", "false"
	if inv != nil {
		warns = envelopeWarningsJSON(inv.envWarn)
		trunc = envelopeTruncatedJSON(inv.envCut)
	}
	fmt.Fprintf(stdout, "{\"schema\":%s,\"kind\":%s,\"items\":%s,\"meta\":{%s},\"warnings\":%s,\"truncated\":%s%s}\n",
		jstr(contractSchema), jstr(cmd.kind), itemsJSON, meta, warns, trunc, extra)
}

// validateSelected —— `--json` 字段名的**纯校验**（不产出、无副作用），与 emitSelected 同一口径。
//
// 为什么要单独一支（GAP-20261020-01）：`chat ask --json <非法字段>` 原先在**建会话/登记别名之后**
// 才走到出口校验 ⇒ 退码虽是 2，状态目录却已多出 chat-aliases.json / chat-last.json / tool_events/。
// 用法错必须是**零副作用**的 ⇒ 在动手之前先调这一支。
func validateSelected(inv *invocation, stderr io.Writer, path []string, fields []string) int {
	cmd := find(path)
	if cmd == nil || len(cmd.fields) == 0 || len(fields) == 0 {
		return exitOK
	}
	for _, f := range fields {
		if !jsonFieldRegistered(cmd, f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, path, f)
		}
	}
	return exitOK
}

// jsonFieldRegistered —— 某个 `--json` 字段名**是否在册**（`fields` ∪ `optFields`）：
// `--json` 字段白名单的**唯一**判据口（`validateSelected` 出口那一侧与 `rejectUnknownJSONFields`
// 咽喉点那一侧共用它 ⇒ 两处收受集**不可能漂**）。`optFields` 是可选字段层（登在出口白名单、
// 点得出来，但不进文档/帮助导出的冻结字段表），对它一律只做**收受**、不做**公告**。
func jsonFieldRegistered(cmd *command, f string) bool {
	if cmd == nil {
		return false
	}
	for _, x := range cmd.fields {
		if x == f {
			return true
		}
	}
	for _, x := range cmd.optFields {
		if x == f {
			return true
		}
	}
	return false
}

// rejectUnknownJSONFields —— `--json <字段>` 字段名的**咽喉点**校验（件 `Q-150`：主流程在
// `cmd.run` 之前调一次 ⇒ 填错字段的命令**零副作用**地退 2 并点名可用字段）。
//
// 与出口那条（`validateSelected`）的分工写死：本支只服务 `run` 的前置面（`cmd` 已经取到 ⇒
// 不重新 `find`）；判据、判词、字段表、退码**四处逐字相同**（`jsonFieldRegistered` +
// `reportBadField` + `exitUsage`）⇒ 同一条命令无论是被前置面还是出口面拦下，调用方读到的
// stderr 与 rc 都一致（✗ 不造第二套文案）。
func rejectUnknownJSONFields(inv *invocation, cmd *command, stderr io.Writer) int {
	for _, f := range inv.fields {
		if !jsonFieldRegistered(cmd, f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, cmd.path, f)
		}
	}
	return exitOK
}

// emitSelected 是**全部** `--json` 出口的唯一实现：先按用户点名的字段（序即用户给的序）拼对象，
// 再套包封。字段点错 ⇒ 退码 2 + 列全部合法字段（§九 M6 I5）。
func emitSelected(stdout, stderr io.Writer, inv *invocation, path []string, fields []string, rows []map[string]string) int {
	cmd := find(path)
	if cmd == nil {
		return exitUsage
	}
	// ★ 2026-10-05（`GAP-20260926-11`）：**收受端按已登记白名单核 → 缺值整键略去**。
	//   改前收受端只认「逐行键」—— 请求的字段只要在**任一行**缺键即**整单 fail-closed 退 2**
	//   ⇒ 字段就算已登在命令表 `fields:` 里也照样报「未知字段」（实测 `resource ledger --json gpu_pct`
	//   因四台账本里有一台机的行没有它而退 2）。⇒ 收受口径改为「**登记即认**」：
	//   ① 真未知字段（不在 `cmd.fields` 里，如 `cpu_pct`）⇒ 仍退 2（`reportBadField` 逐字同）；
	//   ② 在册字段但**本行缺值** ⇒ 该键**整键略去**（✗ 绝不落 `0`/空串 —— 与 API 的 `omitempty` 同一条铁律）。
	//   与同族既有前置校验（`family_impact.go` 的 `fieldListOf` 逐名核）同一条口径；无字段表的命令（如 `help`）不受影响。
	if rc := validateSelected(inv, stderr, path, fields); rc != exitOK {
		return rc
	}
	objs := make([]string, 0, len(rows))
	for _, row := range rows {
		objs = append(objs, marshalObjectRegistered(fields, row))
	}
	emitEnvelopeWith(stdout, cmd, "["+strings.Join(objs, ",")+"]", len(rows), inv)
	return exitOK
}

// selectJSON 单件命令用（仍然出数组：`items` 里一条 —— I3 恒数组）。
func selectJSON(stdout, stderr io.Writer, inv *invocation, path []string, fields []string, row map[string]string) int {
	return emitSelected(stdout, stderr, inv, path, fields, []map[string]string{row})
}

// selectJSONList 清单命令用。
func selectJSONList(stdout, stderr io.Writer, inv *invocation, path []string, fields []string, rows []map[string]string) int {
	return emitSelected(stdout, stderr, inv, path, fields, rows)
}

// ---- M4 幂等四字段（§九 M4 · 调研-M4 §5.1：每条命令各自带四格）----

// idemSpec —— 四字段（取值域照 调研-M4 §5.1，不许自造第五档）。
type idemSpec struct {
	Band    string // 幂等档：只读 / 状态幂等 / 覆盖式 / 追加式
	Rerun   string // 重跑语义：200 复用（changed:false）· 200 重设（changed:true）· 409 … · 400 …
	Effect  string // 生效语义：即时 / 重读声明（--reload）/ 重建（卸+装 / 新代次）
	Danger  string // 危险档：安全 / --dry-run 预演 / --confirm=<目标> / --yes
	Changed bool   // 本次调用是否改变了状态（进 `meta.changed`）
}

// defaultIdem 按「危险档 + 是否只读」派生四字段（显式覆盖见 commands 里各自的 idem 字段）。
func defaultIdem(c *command) *idemSpec {
	if c.danger == nil {
		return &idemSpec{
			Band:    "只读",
			Rerun:   "200 复用（changed:false）",
			Effect:  "即时（无副作用）",
			Danger:  "安全",
			Changed: false,
		}
	}
	d := "安全"
	if c.danger.Level == dangerD3 {
		d = "--confirm=<" + c.danger.Target + "> · --yes"
	} else {
		d = "--yes"
	}
	return &idemSpec{
		Band:    "状态幂等",
		Rerun:   "200 复用（changed:false）· 已在该状态按状态报（**不许**静默什么都不做）",
		Effect:  "重建（卸 + 装 / 新代次）",
		Danger:  d,
		Changed: true,
	}
}

// seedIdem 给命令树补齐四字段（没显式写的按规则派生 ⇒ 四字段**一条不漏**）。
func seedIdem() {
	for _, c := range commands {
		if c.idem == nil {
			c.idem = defaultIdem(c)
		}
		// 危险动作原来没有 kind（批 A 只做形状）⇒ 这里按路径**派生**一个（I2：与命令一一对应
		// 的单数 CamelCase；派生而非手写 23 份，避免两处写法漂）。
		if c.kind == "" {
			c.kind = pascalKind(c.path)
		}
	}
}

// pascalKind `task terminate` → `TaskTerminate`（I2 的字面口径）。
func pascalKind(path []string) string {
	var b strings.Builder
	for _, seg := range path {
		if seg == "" {
			continue
		}
		b.WriteString(strings.ToUpper(seg[:1]))
		b.WriteString(seg[1:])
	}
	return b.String()
}

// idempotencyKey —— 本次调用的幂等键：给了 `--idempotency-key` 就用它（同键 ⇒ 同结果），
// 没给则按「命令 + 位置参数」**派生**（派生键稳定：同参连跑两次同值）。出处：调研-M5 §3.9
// （`Idempotency-Key` 草案 · 本稿标注「推荐，非实证」）· `audit.go` 已有的 `idempotency_key` 效果键。
func idempotencyKey(inv *invocation) string {
	if inv.idemKey != "" {
		return inv.idemKey
	}
	h := sha256.Sum256([]byte(progName + " " + strings.Join(inv.path, " ") + "\x00" + strings.Join(inv.args, "\x00")))
	return fmt.Sprintf("zerg-%x", h[:8])
}

// helpIdempotency —— `zerg help idempotency`：逐命令四字段（§九 M4 · 调研-M4 §5.1）。
func helpIdempotency() string {
	var b strings.Builder
	b.WriteString("幂等语义（§九 M4 · 调研-M4 §5.1 的四字段 · 每条命令各自一格）\n\n")
	b.WriteString("先分清两件事（M4 的坑就在这里）：**幂等**保证「同参重跑，**状态**不变」；\n")
	b.WriteString("**生效**保证「声明改了，运行态跟上」。两者都不许含糊成「重复不报错」。\n\n")
	b.WriteString("硬规矩（调研-M4 §5.2）：\n")
	b.WriteString("  · 幂等重跑**必须退 0**；「已在该状态」**不许**退 1/2（退码 0 + `meta.changed=false`）。\n")
	b.WriteString("  · `--json` 必须给 `changed`（`true`=真改变了状态 / `false`=无变化，明说无变化）。\n")
	b.WriteString("  · 声明已改而运行态未同步 ⇒ `kind=declaration_stale`、退码 `1`、提示 `--reload`/`--force`。\n")
	b.WriteString("  · 任何命令都**不许**在「目标已存在」时静默什么都不做（要么 changed:false 说明，要么按档执行）。\n\n")
	b.WriteString("旗标分档（调研-M4 §5.3）：`--reload` 重读声明 · `--force` 重建承载者（换代次）· `--dry-run` 只算差。\n\n")
	b.WriteString("逐命令四字段（幂等档 / 重跑语义 / 生效语义 / 危险档）：\n")
	for _, c := range catalog() {
		name := "zerg " + strings.Join(c.path, " ")
		fmt.Fprintf(&b, "  %s\n", name)
		fmt.Fprintf(&b, "      幂等档   %s\n", c.idem.Band)
		fmt.Fprintf(&b, "      重跑语义 %s\n", c.idem.Rerun)
		fmt.Fprintf(&b, "      生效语义 %s\n", c.idem.Effect)
		fmt.Fprintf(&b, "      危险档   %s\n", c.idem.Danger)
	}
	b.WriteString("\n幂等键：`--idempotency-key <键>`（同键 ⇒ 同结果）；没给时按「命令 + 位置参数」派生，\n")
	b.WriteString("落在 `meta.idempotency_key`（出处：调研-M5 §3.9 的 `Idempotency-Key` 草案 —— 本稿标注\n")
	b.WriteString("「推荐，非实证」；效果侧已有的同名键在 `core/internal/audit` 的 Record 里）。\n")
	return b.String()
}

// ---- version ----

func cmdVersion(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 本单（缺口账 `GAP-20260928-139` · 本枚）：`--build-info` —— 制品身份面（构件身份块）。
	//   ★ 只用**已有真源** `version.Version`/`version.Commit`/`version.BuildTime`（与下面 `--json` 面同源 ·
	//   同一枚 `-ldflags -X` 注入）⇒ ✗ 不造第二份真源、✗ 不起子进程 / 不读 `go` 工具链 / 不读 `bin/build-info.json`。
	//   给该旗标才走这一支（默认面一字不动）；先于 `--json` 支 ⇒ 与 `--json` 同给时以构件身份块为准。
	if inv.hasFlag("--build-info") {
		fmt.Fprintf(stdout, "build-info: component=%s version=%s commit=%s build_time=%s\n",
			progName, version.Version, version.Commit, version.BuildTime)
		return exitOK
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			// 退码 **2**（用法错 · 退码表 `exitcodes.go` 的 `usage` / `errors.go` 的 `kind=usage`）。
			// ★ `Q-146`（v2.5.12）先归位这一处；★ `K2` 甲档归一（2026-09-24）把**全族 36 个调用点**
			// 搬回漏斗 —— 码由 `requireFields`（**取自退码表**）回，调用点不再自己决定。
			return rc
		}
		row := map[string]string{
			"name":          progName,
			"version":       version.Version,
			"commit":        version.Commit,
			"build_time":    version.BuildTime,
			"contract":      contractID,
			"object_schema": fmt.Sprintf("%d", objectSchemaID),
			"window":        windowSummary(),
		}
		// 主控版本与它的 code_sha 只从 `/api/capabilities` 三键取（§九 M15 T1）——
		// **只在被点名时**才去打主控：`zerg version` 在离线白名单里（§九 M20 O8），
		// 无脑取一次会让它变成一条要网络的命令。
		want := map[string]bool{}
		for _, f := range inv.fields {
			want[f] = true
		}
		if want["core_version"] || want["core_code_sha"] {
			var caps jsonObj
			if err := newClient().getJSON("/api/capabilities", &caps); err != nil {
				row["core_version"] = ""
				row["core_code_sha"] = ""
				fmt.Fprintf(stderr, "%s: 主控不可达 ⇒ core_version / core_code_sha 留空（不打第二枪 · §九 M20 O2）\n", progName)
			} else {
				row["core_version"] = cell(caps["version"])
				row["core_code_sha"] = cell(caps["code_sha"])
			}
		}
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintln(stdout, version.Line(progName))
	return exitOK
}

// ---- help ----

// cmdUsageHelp 回应 `zerg <在册命令> --help`：只出**该条**命令的用法串（缺口 `Q-137` / `Q-141` · 组A）。
//
// 两态口径（一行写死，别在第二处再判一次）：
//
//	① 命令**在册** ⇒ 本函数：只打该条用法（用法串 + 摘要 + 位置参数 + `--json` 字段表），
//	   **不起命令**，退 `0`；
//	② 命令**不在册** ⇒ 不走到这里 —— `run` 在 `resolve` 那一步就落到「未知命令」分支：
//	   stderr 首行逐字 `zerg: 未知命令 "<路径>"`，退 `2`。
//
// 为什么按**命令树真身**渲染、不复用 `zerg help <主题>`：主题表（`topics.go`）是人写的一小撮
// 专题，命令树里绝大多数命令不在其中；本函数读的就是 `command` 那几格（加一条命令只改一处），
// 且 `stdout` **不含**全局头行（`helpText()` 第 1 行）—— 这正是「`--help` 能不能当存在性判据」
// 的分水岭（设计稿 `设计-CLI机器读面-v1.0` §1.3 坑 1/2）。

// passthroughChildHelp —— `-h` 该不该**交给子件**（而不是被命令树截成族帮助）。
//
// 三格同时成立才透传：
//
//	① `cmd.childHelp`（**逐条点名**的透传档 —— 缺省 ✗ 不透传：`gate run` 这类「跑真门」的命令
//	   里 `-h` 一旦漏给子件就会**真跑一门**，那是拿帮助面换副作用，不许默认开）；
//	② 短形 `-h` 且**没有**长形 `--help`（`--help` 永远是该条命令的族帮助面 —— 既有行为一字不动）；
//	③ 命令路径之后**还剩位置参数**（= 已点名子件目标；`zerg gate check -h` 这种没点名的不算，
//	   仍出族帮助、退 0）。
//
// 为什么必须在这里分：`gate check` 壳转发的是 `inv.orig[2:]`（用户敲在 `gate check` 之后的**一切**），
// `-h` 本来就在里面 —— 唯一把子件 usage 挡在门外的就是派发层那一格 `if inv.wantHelp`。
func passthroughChildHelp(cmd *command, inv *invocation, rest []string) bool {
	if cmd == nil || !cmd.childHelp || !cmd.passthrough {
		return false
	}
	if !inv.wantHelpShort || inv.wantHelpLong {
		return false
	}
	return len(rest) > 0
}

func cmdUsageHelp(cmd *command, stdout io.Writer) int {
	usage := strings.TrimSpace(cmd.usage)
	if usage == "" {
		usage = progName + " " + strings.Join(cmd.path, " ")
	}
	fmt.Fprintln(stdout, usage)
	if s := strings.TrimSpace(cmd.summary); s != "" {
		fmt.Fprintf(stdout, "  %s\n", s)
	}
	for _, a := range cmd.args {
		fmt.Fprintf(stdout, "  参数: %s\n", a)
	}
	if len(cmd.fields) > 0 {
		fmt.Fprintf(stdout, "  --json <字段>: %s\n", strings.Join(cmd.fields, ","))
	}
	fmt.Fprintf(stdout, "见 '%s help' 看命令树。\n", progName)
	return exitOK
}

// isHelpTopic 判名字在不在主题表（`topics.go` 是**唯一真源** —— 这里只问它，不旁写第二份名单）。
func isHelpTopic(name string) bool {
	for _, t := range helpTopics() {
		if t.Name == name {
			return true
		}
	}
	return false
}

// renderFamilyHelp —— `zerg help <族>`：列该族下**全部动作** + 逐条用法行（缺口 `Q-149`）。
//
// 为什么要有这一格（修前现读实据）：**族级 `--help` 没有出口** ——
// `zerg model --help` 退 `2`（族名不是一条在册命令 ⇒ 落「未知命令」）、`zerg help model` 也退 `2`
// （族名不是主题 ⇒ 落「未知帮助主题」）；对照 `zerg model show --help` 退 `0`。于是「这条族里
// 有哪些动作」只能靠 `zerg help` 那棵大树里翻，机器读面（`--help`）在**族这一层是断的**。
//
// 三条边界（写死在这里，别在第二处再判一次）：
//
//	① 只**加出口**：命令树计数（141）不动 —— 这不是一条新命令，是 `help` 这条既有命令在
//	   「主题名」之外多认一种输入（族名）；`zerg <族> --help` 的两态**一字不改**（族名仍不在册 ⇒ 退 2）。
//	② **主题表先判**（`cmdHelp` 里那一步）：`version` / `watch` / `config` 既是族名又是主题名 ⇒
//	   主题面赢（专题正文为准 · ✗ 不改名、✗ 不改成族面赢）；2026-10-09 起于专题正文**之后追加**该族
//	   的动作清单（`appendFamilyActions`），族面只兜主题表没有的名字（`model` / `egg` / `impact` …）。
//	③ 族名**认不出** ⇒ 返回 `done=false`，交回主题面报「未知帮助主题」+ 列可用主题（K14 第三件），
//	   退码照旧 `2` —— 不新增第三种错误形状。
//
// 内容真源 = 命令树（`catalog()`，名字序稳定）：逐条**用法行逐字**来自 `command.usage`，
// 危险档成员照实带档位标记（不隐藏、也不与 `help dangerous` 打架 —— 那一条判的是三态，这里只标档）。
// familyMembers —— 现算某族的**全部动作**（命令树 `catalog()` 是唯一真源 · 名字序稳定）。
func familyMembers(family string) []*command {
	var members []*command
	for _, c := range catalog() {
		if len(c.path) > 0 && c.path[0] == family {
			members = append(members, c)
		}
	}
	return members
}

// familyTally —— 三档**各归各的**计数（已开放 / 拒执 / 危险档 · 危险档优先）：与 `renderFamilyHelp`
// 同一口径。「已开放」那一档的逐条真源 = `openedForRun`（别在第二处再判一次）。
func familyTally(members []*command) (open, refused, danger int) {
	for _, c := range members {
		switch {
		case c.danger != nil:
			danger++ // 危险档**逐条各算一条**（与「已开放」两码事）
		case openedForRun(c):
			open++
		default:
			refused++
		}
	}
	return open, refused, danger
}

// renderFamilyActionList —— 逐条动作的用法行（`renderFamilyHelp` 与「专题面后追加族动作」**共用同一渲染**）：
// 用法串逐字来自 `command.usage`（续行照缩进），危险档照实带档位标记，末尾接该条 summary。
func renderFamilyActionList(members []*command, stdout io.Writer) {
	for _, c := range members {
		usage := strings.TrimSpace(c.usage)
		if usage == "" {
			usage = progName + " " + strings.Join(c.path, " ")
		}
		// `agent reap` 一类用法串内含换行（两条形态并列）⇒ 续行照缩进，不把第二行拍到行首。
		for i, line := range strings.Split(usage, "\n") {
			if i == 0 {
				fmt.Fprintf(stdout, "  %s", line)
			} else {
				fmt.Fprintf(stdout, "\n  %s", strings.TrimSpace(line))
			}
		}
		if c.danger != nil {
			fmt.Fprintf(stdout, "  [危险档 %s]", strings.TrimSpace(c.danger.Level))
		}
		if s := strings.TrimSpace(c.summary); s != "" {
			fmt.Fprintf(stdout, "  %s", s)
		}
		fmt.Fprintln(stdout)
	}
}

func renderFamilyHelp(family string, stdout io.Writer) (int, bool) {
	members := familyMembers(family)
	if len(members) == 0 {
		return exitOK, false
	}
	// 计数口径（缺口 序33 · 2026-09-24 已拍）：**三档各归各的** —— 已开放 / 拒执 / 危险档，
	// 三者之和恒等于 `len(members)`（修前拿 `danger == nil` 当「已开放」的代理 ⇒ 真跑拒执的那条
	// 既算「已开放」、又不进危险档 ⇒ **双计**，而 6 = 3 + 3 看着还是自洽的 ⇒ 肉眼看不出病）。
	// 「已开放」那一档的逐条真源 = `openedForRun`（`familyTally` **不再自己判一次**）。
	open, refused, danger := familyTally(members)
	fmt.Fprintf(stdout, "%s %s —— 族级用法（动作 %d 条 · 已开放 %d · 拒执 %d · 危险档 %d）\n",
		progName, family, len(members), open, refused, danger)
	fmt.Fprintf(stdout, "（本页只从这个族名出帮助；逐条 `%s %s <动作> --help` 出该条用法串。名字与用法逐字来自命令树。）\n\n",
		progName, family)
	renderFamilyActionList(members, stdout)
	fmt.Fprintf(stdout, "\n见 '%s help' 看命令树 · '%s help dangerous' 看危险档三态 · '%s help <主题>' 看专题。\n",
		progName, progName, progName)
	return exitOK, true
}

// appendFamilyActions —— 名字**既是族名又是主题名**时（`version` / `watch` / `config`），于**专题正文之后
// 追加**该族的动作清单（保留「主题面赢」的拍板：专题正文在前、族动作清单在后；✗ 不改名、✗ 不改成族面赢）：
// 此前这族的动作在帮助面**双路全瞎** —— `zerg config` 与 `zerg config --help` 都退 2 指到
// `zerg help config`，而后者只出专题正文、一条用法行都没有 ⇒ 族动作 `config reload` 全不可见。
// 无该族 ⇒ **一个字节都不打**（名字只是主题、不是族名的那些，行为一字不动）。渲染与 `renderFamilyHelp` 同源。
func appendFamilyActions(family string, stdout io.Writer) {
	members := familyMembers(family)
	if len(members) == 0 {
		return
	}
	open, refused, danger := familyTally(members)
	fmt.Fprintf(stdout, "\n`%s %s` 同时是一个命令族（动作 %d 条 · 已开放 %d · 拒执 %d · 危险档 %d）—— 本族动作：\n\n",
		progName, family, len(members), open, refused, danger)
	renderFamilyActionList(members, stdout)
}

func cmdHelp(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 缺口 `GAP-20260927-290` / `GAP-20260927-403`：`help` 的 `--json` 机器面（族名清单 + 每族动作数）。
	//   此前 `zerg help --all --json` ⇒ rc=2「没有 --json 字段表（它是说明面）」⇒ 机器/UI 无法枚举族名清单。
	//   产出面**现算自命令树**（`catalog()` · 名字序稳定）⇒ 命令树长一族这条跟着长（一个数都不写死）。
	//   只在**不给主题**（族级清单）那一态开这一档 ⇒ 主题面（`help <主题>`）行为一字不动。
	if inv.jsonGiven && len(inv.args) == 0 {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return emitHelpFamilies(inv, stdout, stderr)
	}
	// ★ 缺口 `GAP-20260927-335`（2026-10-10 · 本枚）：`zerg help <族> --json <字段>` 的**逐动作机器面**
	//   —— 「某动作是不是危险档 / 档位几 / 开没开」此前只能肉眼数 `[危险档 D2]`。只在**点了非主题的
	//   族名**那一态开这一档（主题面赢是人面既有拍板 ⇒ 主题名不进本分支，行为一字不动）；族名认不出 ⇒
	//   `emitHelpFamilyActions` 交回 `(exitOK,false)`，落回下面原有的主题/族面分派（人面逐字不变是本条硬线）。
	if inv.jsonGiven && len(inv.args) > 0 && !isHelpTopic(inv.args[0]) {
		if rc, done := emitHelpFamilyActions(inv, inv.args[0], stdout, stderr); done {
			return rc
		}
	}
	if len(inv.args) > 0 {
		// 主题表在 `topics.go`（**唯一真源**：分派与「可用主题」列清单同一处）。
		// ★ 2026-09-24（缺口 `Q-149`）：主题表**先判**（表里有的名字：专题正文为准）；表里没有 ⇒ 再看
		//   「**族名**」（命令树里某条命令路径的第一段）—— 族级帮助此前没有出口，见 `renderFamilyHelp`。
		// ★ 2026-10-09：名字**既是族名又是主题名**（`version` / `watch` / `config`）⇒ **保留「主题面赢」**
		//   （✗ 不改名、✗ 不改成族面赢），于**专题正文之后追加**那一族的动作清单 —— 修前这族动作在帮助面
		//   **双路全瞎**（`zerg config` 与 `zerg config --help` 都退 2 指到 `zerg help config`，而后者只出
		//   专题正文、一条用法行都没有 ⇒ 族动作 `config reload` 全不可见）。追加面见 `appendFamilyActions`。
		if isHelpTopic(inv.args[0]) {
			rc := renderHelpTopic(inv.args[0], stdout, stderr)
			if rc == exitOK {
				appendFamilyActions(inv.args[0], stdout)
			}
			return rc
		}
		if rc, done := renderFamilyHelp(inv.args[0], stdout); done {
			return rc
		}
		return renderHelpTopic(inv.args[0], stdout, stderr)
	}
	// `inv.all`（`--all`）在这一条命令上**只加出口**（缺口 `Q-071` · 波11 序93）：危险动作那一段
	// 逐条列全。命令树**一条都不新增**（`--all` 是既有全局布尔，`parseInvocation` 那一格认它）。
	fmt.Fprint(stdout, helpText(inv.all))
	// 族级发现面（缺口 `-25` · 2026-09-27）：`help --all` 与主帮助此前都**不列族名清单** ——
	// 用户要找某族只能猜族名（正门指路此前只出现在「敲错族名」那条 stderr 里，见本文件 `:130`）。
	// 只在 `--all` 这一档末尾追加族名清单：族名与顺序**现算**（`catalog()` 名字序稳定 ⇒
	// 一个数字都不写死，命令树长一条这条跟着长）。三条纪律：
	//   ① 只加在 `--all` 上（裸 `help` / `--help` 逐字节不动；`check-public-face-commands.py`
	//      读的是裸 `help` 那棵树，不动它）；
	//   ② 新行**不以「两个空格 + `zerg `」开头** —— `cli_help_all_test.go` 的条数等式
	//      （`--all` 条数 = `help` 条数 + 危险档条数）靠这一条守住；
	//   ③ 两态 rc 语义一字不动（族名仍不吃 `--help`，见 `:130` 那段拍板口径）。
	if inv.all {
		seen := map[string]bool{}
		var fams []string
		for _, c := range catalog() {
			if len(c.path) == 0 || seen[c.path[0]] {
				continue
			}
			seen[c.path[0]] = true
			fams = append(fams, c.path[0])
		}
		fmt.Fprintf(stdout, "\n族级面（%d 族 · 不必猜族名 · 逐族 `%s help <族名>` 列该族全部动作）:\n", len(fams), progName)
		const perRow = 8
		for i := 0; i < len(fams); i += perRow {
			j := i + perRow
			if j > len(fams) {
				j = len(fams)
			}
			fmt.Fprintf(stdout, "  %s\n", strings.Join(fams[i:j], " · "))
		}
	}
	return exitOK
}

// cmdFSUnprotected —— `zerg fs unprotected` 的命令面**薄壳**（F37 接线 · 2026-10-15）。
//
// ★ 判据一行都不落本函数：判据核心 = `family_fs.go:fsUnprotectedCore`（F38 已落 · 提案 `DEV-2976`）。
// 本层只做三件事：① 用法面（`--json` 字段面）② 转发判据核心 ③ 人面 / 机读面（同一份结论）。
// ★ 退码原样转出核心三态（0 无件 / 1 有件 / 2 不给结论）—— ✗ 不在此另取一档、✗ 不放宽断言。
// ★ 本函数**不新报任何 `error.kind` / `error.detail`**（甲档棘轮：新增取值**或新增站点**即红 ⇒
//
//	门⑤ 的登记面归另一路）：用法面 / 不给结论面一律照 dispatch 既有口径 —— 出 stderr 判词 ＋ 退 2。
//
// 落点说明（照同族先例 `family_gap_diff.go:cmdGapDiff` 同形；本笔落 `main.go` 是任务单硬边界：
// `core/cmd/zerg/family_fs.go` 的 `hidden-faces` 半正由另单（F44）在改 ⇒ 本笔 ✗ 不碰那件）。
func cmdFSUnprotected(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面：给了 `--json` 而没给字段 ⇒ 用法错 2（不给结论 —— 与同族 `gap diff` / `repo status` 同口径）。
	if inv.jsonGiven && len(inv.fields) == 0 {
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本命令口径 = 用法错 2）\n", progName)
		if c := find(inv.path); c != nil {
			fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(c.fields, ","))
		}
		return exitUsage
	}
	// ② 判据核心（只读 · 零写盘）：入参零值 ⇒ 仓根走 `repoRoot()`、体量取默认前 50。
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{})
	if rc == exitUsage {
		// 2 = 不给结论（仓根解析不到 / 非 git 工作树 / git 真源取不到）⇒ ✗ 不假绿。
		fmt.Fprintf(stderr, "%s: 判据真源取不到（仓根 / git 工作树 / git 真源）⇒ **不给结论**（退码 2 · ✗ 不假绿）\n", progName)
		fmt.Fprintf(stderr, "（真源 = `git ls-files --others`（未跟踪 ∩ 盘上在）与 `git check-ignore`（未被忽略）—— 任一取不到就不下结论）\n")
		return exitUsage
	}
	// ③ 机读面 / 人面：同一份结论出两个面（✗ 不各写各的）。
	if inv.jsonGiven {
		inv.changed = boolPtr(false)
		if ec := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, fsUnprotectedRows(res)); ec != exitOK {
			return ec
		}
		return rc
	}
	inv.changed = boolPtr(false)
	fsUnprotectedHuman(stdout, res)
	return rc
}
