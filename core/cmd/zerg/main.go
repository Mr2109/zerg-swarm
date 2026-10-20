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
	if inv.wantHelp {
		return cmdUsageHelp(cmd, stdout)
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
	run         func(*invocation, io.Writer, io.Writer) int
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
	commands = []*command{
		{
			path:    []string{"version"},
			kind:    "Version",
			summary: "单行身份（组件 版本 代码 sha 构建时间）· --json 报三层版本",
			usage:   "zerg version [--json <字段>] [--build-info]",
			fields:  []string{"name", "version", "commit", "build_time", "contract", "object_schema", "core_version", "core_code_sha", "window"},
			run:     cmdVersion,
		},
		{
			path:    []string{"root"},
			kind:    "RepoRoot",
			summary: "印仓根（人面一行）· `--json` 给 {root, via}（via = env / 可执行件上溯 / cwd 上溯）",
			usage:   "zerg root [--json <字段>]",
			fields:  []string{"root", "via"},
			run:     cmdRoot,
		},
		{
			path:    []string{"help"},
			summary: "帮助（主题见 `zerg help <主题>`；表在 topics.go）· `--json` 给族名清单机器面（族名 + 每族动作数）",
			usage:   "zerg help [<主题>] [--all] [--json <字段>]",
			arity:   "any",
			args:    []string{"主题（可省）"},
			// ★ 缺口 `GAP-20260927-290` / `GAP-20260927-403`：`help` 此前**无 `--json` 面**
			//   （`zerg help --all --json` ⇒ rc=2「没有 --json 字段表（它是说明面）」）⇒ 族名清单只能人读。
			//   本枚给它一档机器面：字段表两格（`family` / `actions`），产出面在 `cmdHelp` 的 `--json` 分支
			//   （`emitHelpFamilies` · 现算自命令树）。★ 必须写成 `[]string{…}` **字面量**（契约脚本的
			//   `FIELDS_RE` 只认字面量；抽成变量 = 该条被判「没有字段表」）。
			//   ★ 缺口 `GAP-20260927-335`（2026-10-10 · 本枚）：`zerg help <族> --json` 的**逐动作机器面**
			//   （某动作「是不是危险档 / 档位几 / 开没开」）—— 字段走**可选字段层**（`optFields`，与
			//   `code find` 的 `count` / `gate ls` 的 `name` 同一体例：登在出口白名单、点得出来，但**不进**
			//   上面那条 `fields:` ⇒ 人面 / `help export` 文档面一行不动、`clicFieldsRE` 照旧只认 `fields:`）。
			//   逐格：`command` = 全路径 · `danger` = 危险档位（D2/D3，**仅危险档出格**；安全档本行缺该键
			//   ⇒ 机器面「无该字段 = 非危险档」）· `opened` = 真跑开没开（`openedForRun`）。产出面见
			//   `cmdHelp` 的族名 `--json` 分支（`emitHelpFamilyActions`）。
			fields:    []string{"family", "actions"},
			optFields: []string{"command", "danger", "opened"},
			run:       cmdHelp,
		},
		// ---- 批 A · S2 只读面（§6.2 最小可验证集）：全程零写操作 ----
		{
			path:    []string{"doctor"},
			kind:    "DoctorCheck",
			summary: "环境自检（本机项 + 主控可达）· 逐项判定词",
			usage:   "zerg doctor [--json <字段>]",
			fields:  []string{"name", "verdict", "detail", "advice", "artifact_commit", "source_head", "bin_perms", "signature", "digest_dialect"},
			// ★ 缺口 GAP-20260927-200（批⑤ · 2026-10-13）：`credential_consistency`（凭据一致性判词）走**可选字段层**。
			optFields: []string{"credential_consistency"},
			endpoint:  "",
			run:       cmdDoctor,
		},
		{
			path:    []string{"context", "ls"},
			kind:    "Context",
			summary: "档位名册（离线也出表）",
			// ★ 2026-09-24（缺口 `Q-156`）：用法串补上**真旗标** `--resume`。
			//   病灶（修前现读实据）：`--resume` 解析器认（`parseInvocation` 的 `--resume` 那一格）
			//   且该命令**真读**（`readonly.go` 的 `cmdContextLs` → `cmdContextResume`），
			//   用法串却只写 `[--json <字段>]` ⇒ 公开文档面照用法串引 `zerg context ls --resume`
			//   时被判「旗标不在用法串里」（闸⑧ 的 `nonexistent-parameter-documented` 两处逐条在案）。
			//   同一枚旗标的先例写法：`readonly.go` 的续做面登记（§十二 `P-120` 定案 ①）。flag 名逐字不改。
			usage:    "zerg context ls [--resume] [--json <字段>]",
			fields:   []string{"name", "core", "gateway", "default_node", "token_source"},
			endpoint: "",
			run:      cmdContextLs,
		},
		{
			path:     []string{"api", "ls"},
			kind:     "Capability",
			summary:  "HTTP 能力面（投影 /api/capabilities）",
			usage:    "zerg api ls [--json <字段>]",
			fields:   []string{"name", "endpoint", "desc", "example"},
			endpoint: "GET /api/capabilities",
			run:      cmdAPILs,
		},
		{
			path:     []string{"api", "openapi"},
			kind:     "ApiPath",
			summary:  "HTTP 路径表（投影 /api/openapi.json 的 paths）",
			usage:    "zerg api openapi [--json <字段>]",
			fields:   []string{"path", "method", "summary"},
			endpoint: "GET /api/openapi.json",
			run:      cmdAPIOpenAPI,
		},
		{
			path:     []string{"api", "help"},
			summary:  "api 族说明（只读面 · 逃生门 `api call` 暂不开）",
			usage:    "zerg api help",
			endpoint: "",
			run:      cmdAPIHelp,
		},
		// ---- 批 B4（2026-10-20 · `zerg api serve`）：独立入口 `core/cmd/zerg-api` 归入命令面 ----
		// 源件 227 行 · 旗标三枚**逐字**保留（`-port 8083` / `-workdir .` / `-insecure`），只把 Go
		// `flag` 的单横线写成双横线。★ 并入**既有** `api` 族（`ls`/`openapi`/`help` 三条只读投影 =
		// 打主控 `/api/*` 的**客户端**面），✗ 不新开顶层族；动作名分家：`serve` = **在本机起服务端**。
		// **双轨期**：`core/cmd/zerg-api` 二进制一字不动。实现在 `family_api_serve.go`
		// （件头逐条写清与源件的五处差异 —— 用法退码 1→2、`--port` 值域前置判、工作目录启动时定死、
		// 不切进程 cwd、优雅停）。
		// 写面（`POST /task` 真派单 · 改任务单目录）但**不进危险表**，理由与 `sched run` 那一格同：
		// 它是操作者当场在本机起的那个常驻进程（本来由 launchd 拉起），Ctrl-C/SIGTERM 即停；
		// 远端可达面**由鉴权闸守着**（缺 `ZERG_API_TOKEN` 即拒启，无令牌必须显式 `--insecure` 并打横幅）
		// ⇒ 本机可逆、无远端副作用；与 `trace`/`review` 同一种「不进危险表」的判法。
		{
			path:     []string{"api", "serve"},
			kind:     "ApiServe",
			summary:  "在本机起外部接入 REST API（**常驻档**：`POST /task` 提交任务进 git 自循环 · `GET /task/{id}` 查 · `GET /capabilities` 机器可读清单 · `GET /status` 统计 · `GET /panel` 面板；缺 `ZERG_API_TOKEN` 即拒启、`--insecure` 放行并打横幅）",
			usage:    "zerg api serve [--port <端口>] [--workdir <目录>] [--insecure]",
			arity:    "none",
			args:     []string{"（不收位置参数：端口走 `--port`、工作目录走 `--workdir`）"},
			endpoint: "",
			opened:   true,
			run:      cmdAPIServe,
		},
		{
			path:     []string{"agent", "ls"},
			kind:     "Machine",
			summary:  "子端（机器）清单（投影 /api/fleet/status）",
			usage:    "zerg agent ls [--json <字段>]",
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
			endpoint: "GET /api/fleet/status",
			run:      cmdAgentLs,
		},
		{
			path:     []string{"task", "ls"},
			kind:     "Task",
			summary:  "任务队列（投影 /api/tasks）",
			usage:    "zerg task ls [--json <字段>]",
			fields:   []string{"id", "status", "model", "machine", "priority", "created_at", "description"},
			endpoint: "GET /api/tasks",
			run:      cmdTaskLs,
		},
		{
			path:     []string{"model", "ls"},
			kind:     "Model",
			summary:  "可用模型（投影 /api/fleet/models）",
			usage:    "zerg model ls [--json <字段>]",
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdModelLs,
		},
		{
			path:    []string{"help", "export"},
			kind:    "HelpExport",
			summary: "把命令树导出 markdown 进版本档案目录（产物 · 勿手改）；`--dry-run` 出逐条清单、真写要 `--yes`（默认档 fail-closed）",
			usage:   "zerg help export [--out <目录> | --docs-ver <X.Y.Z>] [--dry-run | --yes] [--json <字段>]",
			fields:  helpExportFields,
			// 授权侧（缺口 `GAP-20260927-140` ① · Mr2109 2026-10-08 拍「默认档要 fail-closed」）：
			// 本条**真写盘**（往版本档案目录落一件导出物）而修前无 `danger` ⇒ 默认档零授权写。
			danger:   &dangerSpec{dangerD2, "（落点目录）", "往版本档案目录落一件导出物（命令面 markdown · 同日幂等覆盖 · 仓外落点）", "GAP-20260927-140 ① · GAP-20260927-135 · 同族先例 doc meta fill（D2）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdHelpExport,
		},
		// ---- 批 A · S3 门禁直通六条（**只转发、不翻译** · §6.3 S3 判据③）----
		// ★ 缺口 `GAP-20260927-226` want①：`name` 走**可选字段层**（`optFields`，与 `code find` 的
		// `count` 同一体例）—— 步名的第二个键（同族 `gate find`/`gate explain` 的 `name` 逐字同名）；
		// **不进**上面这份帮助/导出面的冻结表 ⇒ 帮助面与命令页一行不动。
		{
			path:    []string{"gate", "ls"},
			summary: "门禁步骤表（逐行等于脚本 --list；薄壳不另写一份）",
			usage:   "zerg gate ls [--json <字段>] [--fingerprint]",
			fields:  []string{"total", "scope", "mode", "step"},
			// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：`reds`（最近一趟非 PASS 的步名清单）同走**可选字段层**。
			// ★ 缺口 `GAP-20260928-68`（本片 · 2026-10-14）：`id`（每步的**稳定短 id** = 步名派生的内容指纹 ·
			//  ✗ 不含行号/序号 ⇒ 步骤表插/删一行后其余步的 id 逐字不变）同走**可选字段层** ⇒
			//  `zerg gate ls --json id` 点得出来（件内那一侧是 `gate.go` 的 `gateLsOptFields`，两表逐字一致
			//  由 `TestGateLsOptFieldsParity` 钉住 · ✗ 不进冻结 `fields:` ⇒ 帮助面与命令页一行不动）。
			// ★ 缺口 `GAP-20260929-22`（本片 · 2026-10-14）：`cmd`（每步**可直接粘贴的整条命令**，含正确
			//  引号；单引号逐字 / 内嵌单引号按 `'\''` 闭合 · 见 `gate.go` 的 `gateLsPasteCmd`）同走**可选字段层**
			//  ⇒ `zerg gate ls --json cmd` 点得出来；✗ 不进冻结 `fields:`（帮助面与命令页一行不动 · 同 `name`/`id`）。
			optFields:   []string{"name", "gates_total", "fast", "reds", "id", "cmd"},
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:        []string{"gate", "run"},
			summary:     "跑门禁（旗标逐字透传；退码原样转出，不翻译；`--step` 只跑一道门）",
			usage:       "zerg gate run [--scope <s> | --fast] [--outdir <目录>] [--dry-run] … | zerg gate run --step <步名> [--dry-run] [--self-test] [--only-step] [--json <字段>] [--verify-live] [--show-log] [-- <只读旗标>…（如 --root <路径>）]",
			arity:       "any",
			args:        []string{"脚本旗标（原样透传）"},
			fields:      gateRunStepFields,
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:    []string{"gate", "show"},
			kind:    "GateShow",
			summary: "看某一步要跑的命令串（脚本 --emit-cmd）；`--json` 另给四格（scope/mode/判据/日志路径）",
			usage:   "zerg gate show <步名> [--json <字段>]",
			arity:   "any",
			args:    []string{"步名（与 --list 里逐字相同；`--json` 面是**精确匹配**，子串不给结论）"},
			// 四格 = `Q-061`/`B-3` 的可核条件逐字（`scope` / `mode` / 判据 / 日志路径）。
			// 默认面（不给 `--json`）仍是 `--emit-cmd` 直取口 —— 见 `family_gate_show.go` 的文件头。
			fields:      gateShowFields,
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:    []string{"gate", "find"},
			kind:    "GateFind",
			summary: "按**门件名/命令串片段**找步骤名（门件名 ⇒ 步名 的桥）· 只读",
			usage:   "zerg gate find <片段> [--json <字段>] | zerg gate find --uncertain [--json <字段>]",
			arity:   "any",
			args:    []string{"片段（门件名或命令串里的一段，例：check-gate-coverage）"},
			// 九格 = 与 `gate explain` 同义字段逐字同名（判据/日志/出处在本仓只有一处口径）+ 第八格
			// `next`（照抄即走的下一步）+ 第九格 `wiring`（三处接线面 precommit / all.sh / real-gates.sh
			// 的现读结论 —— 「零命中」与「不存在」的分别就靠它）。★ 本命令**不走脚本**：只读脚本里的
			// `add_step` 声明 ⇒ 与 `gate results` 同规（命令面分支），不改任何脚本退码。
			fields: gateFindFields,
			// ★ 本单（缺口 `GAP-20260929-28` 剩面）：可选字段层两批 —— ① `command_kind` /
			//   `command_resolved`（第五格形态 + 真身的人面两格 · 与 `gate explain` 同款）；
			//   ② `line` / `kind` / `why` / `raw`（`gate find --uncertain` 那张正门表的四格）。
			//   都不进冻结 `fields:` ⇒ 不重生命令页 / 帮助面；点得出即用。
			optFields: []string{"command_kind", "command_resolved", "line", "kind", "why", "raw"},
			endpoint:  "",
			run:       cmdGate,
		},
		{
			path:    []string{"gate", "results"},
			kind:    "GateResults",
			summary: "读**现成**一趟门禁产物的四数（通过/失败/不给结论/只报告 + 步数与总退码）· 只读",
			usage:   "zerg gate results [--last] [--dir <目录>] [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数：那一趟由 `--last`（缺省即最近一趟）或 `--dir <目录>` 指）"},
			// 逐条五格 = 步名/状态/退码/耗时/日志路径（缺口 `Q-111`/`B-8` 的机器面）。
			fields: gateResultsFields,
			// ★ 缺口 GAP-20260927-227（批⑤ · 2026-10-13）：`source` 走**可选字段层**（`optFields`）——
			//   「本页读的是**哪一趟**产物」的机读直读口（= 那一趟目录，与 `meta.run_dir`/`meta.results_tsv`
			//   同源），登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表 ⇒ 不重生命令页。
			optFields: []string{"source"},
			endpoint:  "",
			run:       cmdGate,
		},
		{
			path:        []string{"gate", "self-test"},
			summary:     "门禁自检（合成步骤 · 不碰真目标）",
			usage:       "zerg gate self-test",
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:     []string{"gate", "self-test-summary"},
			kind:     "GateSelfTestSummary",
			summary:  "**判据自检的统一机器面**（缺口 `GAP-20260926-183` · 路 B）：一个入口跑齐进程内判据的自检、**集中解析末行** ⇒ 三态（通过/失败/判不了）＋ 两读数（格数/失败数）",
			usage:    "zerg gate self-test-summary [--json <字段>] [--self-test]",
			arity:    "none",
			args:     []string{"（不收位置参数：名录 = 进程内判据臂 `gateCheckGoEntry` · 逐件跑 `--self-test` 并解析末行；解析不出 ⇒ 判不了）"},
			fields:   []string{"count", "pass", "fail", "unknown", "grids", "fails", "entries"},
			endpoint: "",
			run:      cmdGateSelfTestSummary,
		},
		// ---- v2.5.14 · M1 首项：判据转发壳 `zerg gate check <判据名>`（设计-判据 CLI 化-v1.7 §9.1）----
		// 形状：**动词做命令、名词做参数** ⇒ 判据名是**参数**，不是 189 条平级命令（§5.3）。
		// 走 `cmdGate` 的新动作分支（`gate.go` 的 `case "check"` → `family_gate_check.go`）：
		// 判据名 ⇒ 件 的**保守**解析（名录真源 = 现读 `scripts/gates/` 目录列举）＋ 逐字透传退码与两路输出；
		// 不在册 ⇒ 拒并给可用名；件自身崩 ⇒ `8` ＋ `detail=judge_broken` ＋ `raw_rc`（**零新占号**）。
		// 透传型（同 `gate run`/`show`）：子件的旗标与位置参数逐字交给子件 ⇒ 命令面不另判旗标面。
		{
			path:     []string{"gate", "check"},
			kind:     "GateCheck",
			summary:  "跑**一件判据件**（判据名 ⇒ scripts/gates/*.py|*.sh 的保守解析 · 逐字透传退码与输出；不在册 ⇒ 拒并给可用名；件自身崩 ⇒ 8 不给结论；**写工具**件强制危险档 · 缺 --yes ⇒ 2）",
			usage:    "zerg gate check <判据名> [--list-selfcases] [--dry-run | --yes] [子件参数…]",
			arity:    "any",
			args:     []string{"判据名（＝ scripts/gates/ 下件名去扩展名 · 逐字相同；✗ 不模糊匹配）"},
			endpoint: "",
			// ★ `--json` 字段表（M1 尾 · 全局字段表闸）：与 `family_gate_check.go` 的
			//   `gateCheckFullTable` 机器面**同源** —— 现读那一处 `enc.Encode(map[string]any{…})`
			//   的四个键（`count` / `resolved` / `dry_run` / `entries`），一字不多一字不少。
			//   没有它，`zerg gate check --json` 会被命令面当「说明面」拒成 2
			//   （逐字：``zerg gate check` 没有 --json 字段表（它是说明面）`）。
			// ★ 必须写成 `[]string{…}` **字面量**（契约脚本的 `FIELDS_RE` 只认字面量；
			//   抽成变量 = 该条被读成「没有字段表」）—— 与 `net probe` / `publish tree has` 同一条纪律。
			fields: []string{"count", "resolved", "dry_run", "entries"},
			// ★ 写工具危险档（M1 批B）：件头 `#zerg-class: write D2|D3` ⇒ 壳侧 exec 前强制确认档。
			// 取**最坏档 D3** 登记（写工具里最坏是不可逆；命令树一格只能写一档 ⇒ 取最坏）；
			// `ChecksConfirm: true` = 实现真读 `--confirm`（family_gate_check.go 的写工具闸 · 真读才印）。
			danger:      &dangerSpec{dangerD3, "判据名", "跑一件**写工具**（会真改盘：换件/落临时副本/落锁）；只读件不受影响", "本单 · 同族先例 §九 M3 C1 · family_gate_matrix.go:12-15", true, dangerScopeUnknown},
			opened:      true,
			passthrough: true,
			run:         cmdGate,
		},
		// ---- T09/T11 正门两条（本枚 · **只增**）----
		// ★ 两条都**不走** `cmdGate`：`cmdGate` 的动作分派长在 `gate.go`（本枚不许改的第三件之外），
		//   而 `resolve()` 的 3 段前缀试探（见其函数头）已能把 `gate selfreport diff` 精确命中 ⇒
		//   直接把 `run` 指向本枚新增的两个处理器即可（零改既有分派）。
		{
			path:     []string{"gate", "selfreport", "diff"},
			kind:     "GateSelfreportDiff",
			summary:  "自报件 × 独立对拍件**六格逐格**对拍（query_ts/computed_at/source/digest/verifier/verdict）· 只读 · 判不了不给结论",
			usage:    "zerg gate selfreport diff --source <件> --against <件> [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：自报方与对拍方分别由 `--source` / `--against` 指 —— 两枚同名即用法错，禁自报面自打分）"},
			fields:   gateSelfreportFields,
			endpoint: "",
			run:      cmdGateSelfreportDiff,
		},
		{
			path:     []string{"gate", "four-states"},
			kind:     "GateFourStates",
			summary:  "四态亲测：现役件／改件 × 不带／带 `--yes` 各跑一次，四格逐格比对 · 只读面必须逐字节相同、写面不许恒同",
			usage:    "zerg gate four-states --current <命令串> --patched <命令串> [--yes-flag <旗标>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：两条命令串由 `--current` / `--patched` 指；命令串按**空白切词**，不做 shell 展开）"},
			fields:   gateFourStatesFields,
			endpoint: "",
			run:      cmdGateFourStates,
		},
		{
			path:     []string{"gate", "reap"},
			danger:   &dangerSpec{dangerD3, "（陈旧/卡死门跑根）", "逐枚 pid 发 SIGTERM→SIGKILL 终止陈旧/卡死门跑根**及其门谱系** ＋ 追加审计 reap-audit.jsonl（真在飞的门跑绝不碰）", "GAP-20261005-35 · 同族先例 agent reap（D3）· gate 族写工具件走 gate check D3", false, dangerScopeSystem},
			opened:   true,
			kind:     "GateReap",
			summary:  "回收**陈旧/卡死门跑根**（只收祖链已断或死趟残儿的根**及其门谱系**；真在飞的门跑绝不碰）· 每个收回动作入审计",
			usage:    "zerg gate reap --stale [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：`--dry-run` 只列清单、`--yes` 才真收）"},
			fields:   gateReapFields,
			endpoint: "",
			run:      cmdGateReap,
		},
		{
			path:     []string{"gate", "ps"},
			kind:     "GatePS",
			summary:  "只读：门进程 + 门锁的现读快照（进程段复用 gateReapScan · 锁段读 <锁目录>/gate-run.lock）· 立即返回",
			usage:    "zerg gate ps [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   gatePSFields,
			endpoint: "",
			run:      cmdGatePS,
		},
		{
			path:     []string{"gate", "progress"},
			kind:     "GateProgress",
			summary:  "**只读快照**：看正在跑那一趟门禁的活进度（八格：趟号步数/正跑哪步/已耗时/静默秒/四数/是否在跑/趟归属/advancing）· 纯只读 · 立即返回 · 绝不阻塞（设计-门进度可查-v0.3 §三/§十） · 判活**只判第 3 层**（当前步日志 mtime 推进 ＋ 谱系 pcpu>0）：快照不做长采样，**完整三层取或**只在有时间窗口的 `gate wait` 落（缺口 `GAP-20261002-16`）",
			usage:    "zerg gate progress [--dir <目录> | --last] [--list] [--limit <n>] [--silent-threshold <秒>] [--json <字段>]（`--last` **按最新写入挑**：`results.tsv` 的 mtime 最新 · 与 `zerg gate results --last` 同口径；点名那一趟用 `--dir`）",
			arity:    "none",
			args:     []string{"（不收位置参数：那一趟由 `--dir` / `--last` 指 —— 非 TTY 下多趟活 ⇒ 列清单 + 退 2，绝不弹提示）"},
			fields:   []string{"run_dir", "finished", "alive", "root_pid", "origin_kind", "origin", "steps_done", "steps_total", "steps_source_sha", "current_step", "current_step_name", "current_secs", "elapsed_secs", "last_activity_ts", "silent_secs", "silent_threshold", "pass", "fail", "blocked", "report", "exit_code", "slowest_steps", "advancing", "deepest_pid", "deepest_cmd", "deepest_secs", "depth_limit_reached"},
			endpoint: "",
			run:      cmdGateProgress,
		},
		{
			path:     []string{"gate", "wait"},
			kind:     "GateWait",
			summary:  "**有界阻塞**等门跑收工（与 progress 共享同一引擎）；`--timeout` **默认有限**（不学 kubectl 的 0=无穷）；`--exit-status` 把门禁的红透成退码（设计-门进度可查-v0.3 §十.6） · 判活**自第二帧起判完整三层取或**（活动计数 / 最深层 PID 换过 / 当前步日志 mtime 推进 · §十.1）：与 `gate progress` 的快照口径（只判第 3 层 ＋ 谱系 pcpu>0）不同（缺口 `GAP-20261002-16`）",
			usage:    "zerg gate wait [--dir <目录> | --last] [--interval <秒>] [--timeout <秒>] [--exit-status] [--json <字段>]（`--last` **按最新写入挑**：`results.tsv` 的 mtime 最新 · 与 `zerg gate results --last` 同口径；点名那一趟用 `--dir`）",
			arity:    "none",
			args:     []string{"（不收位置参数：那一趟由 `--dir` / `--last` 指）"},
			fields:   []string{"run_dir", "finished", "alive", "root_pid", "origin_kind", "origin", "steps_done", "steps_total", "steps_source_sha", "current_step", "current_step_name", "current_secs", "elapsed_secs", "last_activity_ts", "silent_secs", "silent_threshold", "pass", "fail", "blocked", "report", "exit_code", "slowest_steps", "advancing", "deepest_pid", "deepest_cmd", "deepest_secs", "depth_limit_reached", "waited_secs", "timed_out", "interval", "timeout"},
			endpoint: "",
			run:      cmdGateWait,
		},
		{
			path:     []string{"gate", "seal", "verify"},
			kind:     "GateSealVerify",
			summary:  "核章档（第四档）章账**只读**核验：九格齐备 + 断言 executed 计数 0 + 失效规则（head/worktree_fp 与现读不符 ⇒ stale；步不在现读步集 ⇒ gap）· 三态逐字输出（未真跑（核章）/stale/gap）",
			usage:    "zerg gate seal verify [<章账>] [--json <字段>]",
			arity:    "any",
			args:     []string{"章账（append-only JSONL · 可省：缺省 scripts/gates/seal/ledger.jsonl）"},
			fields:   gateSealVerifyFields,
			endpoint: "",
			run:      cmdGateSealVerify,
		},
		// ---- 批 B · T-19 茧壁：`agent ping` 默认经主控、`--direct` 才直连（§十五.4 丙案）----
		{
			path:    []string{"agent", "ping"},
			kind:    "AgentPing",
			summary: "探活一台机（默认经主控；`--direct` 直连且回显 via）",
			usage:   "zerg agent ping <机器名> [--direct <host:port>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via"},
			// ★ 缺口 GAP-20260925-20（批⑤ · 2026-10-13）：`answered_by`（**谁答的探活**）走**可选字段层**。
			optFields: []string{"answered_by"},
			endpoint:  "GET /api/fleet/status（默认档）",
			run:       cmdAgentPing,
		},
		// ---- 批 B · T-17 事件面（§九 M1）：**唯一名字** `zerg watch` ----
		{
			path:          []string{"watch"},
			kind:          "Watch",
			summary:       "订阅事件流（单端点 + Accept 协商 · 唯一名字）",
			usage:         "zerg watch [<id>] [--accept <媒体类型>] [--follow]",
			arity:         "any",
			args:          []string{"对象 id（可省：跟全群）"},
			endpoint:      "GET /api/events（**主控面今天没有** ⇒ 本版不给结论）",
			groupReadOnly: true,
			run:           cmdWatch,
		},
		{
			path:     []string{"task", "show"},
			kind:     "TaskShow",
			summary:  "看单个任务（`--follow` 转发到 `zerg watch`，不另起一条流）",
			usage:    "zerg task show <任务 id> [--follow] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"id", "status", "model", "machine", "priority", "created_at", "description"},
			endpoint: "GET /api/tasks",
			run:      cmdTaskShow,
		},
		// ---- 批 D · T-43 `agent` 族收口（§三 B 族 · §十二 `P-038` 族名闭合 · 只读四条）----
		{
			path:     []string{"agent", "show"},
			kind:     "Machine",
			summary:  "看一台子端（投影 /api/fleet/status 的单机条目 —— 与 `agent ls` 同一份载荷）",
			usage:    "zerg agent show <机器名> [--window-min <分钟>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
			endpoint: "GET /api/fleet/status",
			run:      cmdAgentShow,
			// ★ 缺口 `GAP-20261010-16`（want③）：`recent_requests`（最近 N 分钟请求数）与
			//   `request_source`（当前请求来源）走**可选字段层**（`optFields`，与 `code find` 的
			//   `count` 同一体例）—— 登在出口白名单里、`--json` 点得出来，却**不进** `fields` 那份
			//   文档/帮助导出面的冻结表 ⇒ 随动面为零（✗ 不重生成命令页）。
			//   ★ 两格都**只增不改**：载荷那 18 格的字段名/顺序一个字未动（旧消费侧零改动）。
			optFields: []string{"recent_requests", "request_source"},
		},
		{
			path:     []string{"agent", "models"},
			kind:     "Model",
			summary:  "该机上可用的模型（投影 /api/fleet/models 里 host 命中的那些）",
			usage:    "zerg agent models <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file", "declared", "subend"},
			endpoint: "GET /api/fleet/models",
			run:      cmdAgentModels,
		},
		{
			path:     []string{"agent", "logs"},
			kind:     "AgentLogs",
			summary:  "该机的日志（走主控查询面 GET /api/fleet/logs/query?machine=）",
			usage:    "zerg agent logs <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"ts", "machine", "level", "component", "model", "request_id", "message"},
			endpoint: "GET /api/fleet/logs/query?machine=<机>",
			run:      cmdAgentLogs,
		},
		// ---- 缺口 `GAP-20261010-21`（B 单 · CLI 命令面）：`agent forget`（D2 写面）----
		// 打 A 单已落的主控端点 `POST /api/fleet/forget`（复用内核 `store.Forget`）；口径照端点：
		// 干跑 `dry_run:true` / 真删 `confirm:true`。命令面三态照同族写面先例 `model pull`（D2）：
		// `--dry-run` = **本地**计划件（一个请求都不发）· 缺 `--yes` fail-closed 2 · `--yes` 才真打端点。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门73 两半。
		{
			path:     []string{"agent", "forget"},
			kind:     "AgentForget",
			summary:  "把一台机从**主控运行态名册**里摘掉（`POST /api/fleet/forget` · D2 写面 · `--dry-run` 零副作用出计划件 · 缺 `--yes` fail-closed 2 · 幂等：不在册 ⇒ changed=false 仍退 0）",
			usage:    "zerg agent forget <机器名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（要摘掉的那台机 · 例 ms02）"},
			fields:   []string{"dry_run", "machine", "present", "changed", "would_forget"},
			danger:   &dangerSpec{dangerD2, "机器名", "把该机从**主控运行态名册**里摘掉（`store.Forget` —— 该机再来一次心跳即回名册；✗ 不碰子端 / ✗ 不重启主控）", "缺口 GAP-20261010-21 · A 单端点 POST /api/fleet/forget · 同族写面先例 model pull（D2）", false, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/fleet/forget",
			run:      cmdAgentForget,
		},
		// ---- 缺口 `GAP-20261018-20`：`agent deploy`（子端换件的**命令面正门** · D3）----
		// 病（逐字）：换件此前只能手搓 —— `scp` 直覆盖在跑的件 ⇒ `ETXTBSY`；
		// `systemctl stop` ⇒ 要交互认证（AI 不敲提权命令）；`scp` 到 `.new` ＋ `mv -f` ＋ `restart`
		// 才成，但换完 `agent ls` 的 `code_sha` 仍是旧值 ⇒ 运维面判不了远端跑哪版。
		// 语义 = 落地 `.new` → 远端 sha256 对拍 → `mv -f` 换名 → **只 restart**（✗ stop · ✗ 提权）
		// → 直读子端 `/status` 核对新件自报的 `code_sha` / `code_digest`（对不上 ⇒ 退 1）。
		// ★ 新命令的随动面（cli-matrix must-fail 格 / cli-contract-baseline / help 面冻结读数 /
		//   newcmd-paths-baseline / 换件这条 ssh 面的 §十五.4 例外格）见本单报告的「待随动」一节。
		{
			path:    []string{"agent", "deploy"},
			machine: machineNodeName,
			kind:    "AgentDeploy",
			summary: "子端换件正门（D3 · 先落 `.new` → 远端 sha256 对拍 → `mv -f` 换 inode → 只重启 unit；✗ scp 直覆盖（ETXTBSY）· ✗ systemctl stop（要交互认证）· ✗ 提权）＋ 换完直读子端自报身份，`code_sha`/`code_digest` 对不上 ⇒ 退 1",
			usage:   "zerg agent deploy <机器名> --file <本地件> [--dry-run | --confirm=<机器名> --yes] [--by <谁>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名", "--file <本地件>"},
			fields:  agentDeployFields,
			danger: &dangerSpec{dangerD3, "机器名",
				"换该子端的守护件并重启它（在跑任务被打断；旧件被 rename 覆盖、本命令不留回退件）",
				"缺口 GAP-20261018-20 · 同族先例 agent restart（D3 · 远端 ssh 落 F-5 那一格）", true, dangerScopeRemote},
			opened: true,
			run:    cmdAgentDeploy,
		},
		// ---- 批 B5（2026-10-20 · `zerg agent run`）：独立入口 `core/cmd/zerg-agent` 归入命令面 ----
		// 源件 462 行 · 十六枚 Go `flag` **逐字**保留（只把单横线写成双横线；`-version` 除外 ——
		// `zerg version`/全局 `--version` 是身份面唯一真源）。★ 并入**既有** `agent` 族
		// （`ls`/`show`/`ping`/`deploy`/`load`/`unload`/… = 打子端/主控的投影与控制面），✗ 不新开
		// 顶层族；动作名分家：`run` = **在本机跑一个 Agent 任务**（调网关、执行工具、写自己的任务单）。
		// 与 `sched run`／`api serve` 同一条判法（族已在册 ⇒ 只增一格）。
		// **双轨期**：`core/cmd/zerg-agent` 二进制**一字不动** —— 主控调度器
		// （`core/internal/agent/scheduler.go:477` 与 `worker.go:157`）仍 `exec zerg-agent`，
		// 且**靠它的进程内语义码** 0/2/3/4 判重试与死信；本命令走命令面退码表（3/4 已另有所指）
		// ⇒ 两条轨的退码空间**不同**，不许互换（差异 ③ 在实现件件头逐字写着）。
		// 写面（真调模型、真执行工具、写工作目录与任务单）但**不进危险表**，理由与 `sched run`
		// 同：它是操作者当场在本机起的那个任务运行器，跑的是自己给的那段任务、写的是自己的工作区；
		// 工具门在**内层**（`control.NewAgentGateFromRepo` 的规则表 · 缺省 `block` · 装不上则恒拦）
		// ⇒ 本机可逆、无远端副作用。
		{
			path:    []string{"agent", "run"},
			kind:    "AgentRun",
			summary: "在本机**跑一个 Agent 任务**（调网关 → 执行工具 → 写工作区；`--issue` 接单档：读问题单→查因→回填结论→标 resolved/escalated；`--auto-issue` 失败自动挂单 · `--subtask` 子任务结晶 · `--mcp` 挂 MCP 工具 · `--state` 断连恢复；缺令牌即拒跑、工具门缺省恒拦）",
			usage:   agentRunUsageLine,
			arity:   "none",
			args:    []string{"（不收位置参数：任务走 `--task`、问题单走 `--issue`）"},
			fields:  agentRunFields,
			opened:  true,
			run:     cmdAgentRun,
		},
		{
			path:    []string{"agent", "probe"},
			kind:    "AgentProbe",
			summary: "探活诊断（默认经主控；直达属 F-2 例外、要显式 `--direct`）",
			usage:   "zerg agent probe <机器名> [--direct] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via", "direct_gate"},
			// ★ 缺口 GAP-20260925-23（批⑤ · 2026-10-13）：`dial_addr`（本面若持 `--direct` 会拨的地址）走**可选字段层**。
			optFields: []string{"dial_addr"},
			endpoint:  "GET /api/fleet/status（默认档）",
			run:       cmdAgentProbe,
		},
		// ---- 缺口 `GAP-20260925-50` ②③④ · `agent` 族第二组（2026-09-26）----
		{
			path:     []string{"agent", "reload"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD2, "机器名", "让子端热重载（`POST http://<host>:<port>/infer/reload` · 改子端在跑引擎态 · 必带 `X-Auth-Token`）", "GAP-20261005-38 · 同族先例 core reload（D2）· itask stop（D2）", false, dangerScopeRemote},
			opened:   true,
			kind:     "AgentReload",
			summary:  "子端热重载（`POST http://<host>:<port>/infer/reload` · **必带 `X-Auth-Token`**（不带就回 unauthorized）；子端那三格原样透传）",
			usage:    "zerg agent reload <机器名> --yes [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentReloadFields,
			endpoint: "POST http://<host>:<port>/infer/reload（子端 HTTP 面 · 令牌只从凭据链取、走 X-Auth-Token）",
			run:      cmdAgentReload,
		},
		{
			path:     []string{"agent", "registry"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD3, "机器名", "改那台子端盘上的注册表件（`--add`：先校验 → `--dry-run` → `--yes` 才写 → 留 `.bak-registry-add-<日期>` → 写完读回再校，不过即回滚）；`--list` 只读", "GAP-20261005-37 · 同族先例 agent restart（D3）· agent load/unload 已挂危险档", false, dangerScopeRemote},
			opened:   true,
			kind:     "AgentRegistry",
			summary:  "改/看那台子端的注册表（`--list` 只读；`--add` 先校验 → `--dry-run` 先行 → `--yes` 才写 → 留 `.bak-registry-add-<日期>` → 写完读回再校，不过即回滚）",
			usage:    "zerg agent registry <机器名> [--list] [--add <模型名> --file <GGUF> --ctx <N> --mem-gb <N>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名", "（`--add` 的模型名 = 名册模型 id，**逐字相同**）"},
			fields:   agentRegistryFields,
			endpoint: "本地件面：子端注册表（`zerg-agentd --registry <件>`；落点现读 `deploy/com.zerg.agent-<机>.plist` 的 `--registry` 实参）；远端机 ⇒ kind=blocked",
			run:      cmdAgentRegistry,
		},
		{
			path:     []string{"agent", "bench"},
			kind:     "AgentBench",
			summary:  "推理测速：**按名字/端口从子端回据里认准**在跑的引擎（`--model` 点名；多枚在跑而没点名 ⇒ 退 2，**不许抓碰巧第一个**），对 `/v1/chat/completions` 发一次定形请求（出 predicted_per_second / prompt_per_second / predicted_n）+ **三态判据** `verdict`：**可读正文 / 只有思考（推理预算不够，未到正文）/ 真乱码** —— 三态各自成立、**都退 0**（引擎输出差 ≠ 命令错，写进 warnings[]）",
			usage:    "zerg agent bench <机器名> [--model <名>] [--n-predict <N>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentBenchFields,
			endpoint: "POST http://<host>:<engine_port>/v1/chat/completions（引擎由子端 `/eggs` 按**名字+端口**认准、`/status` 兜底；取不到 / 说不清是哪一枚 ⇒ blocked 或退 2）",
			run:      cmdAgentBench,
		},
		// ---- R6 · 缺口 `GAP-20260927-04` / `GAP-20260927-202`② · `agent` 族：`engines` / `slots` ----
		// 实现件已落（`family_agent.go`：`cmdAgentEngines`/`cmdAgentSlots` + 两个字段表）⇒ 本处只做命令树登记。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半。
		{
			path:     []string{"agent", "engines"},
			kind:     "AgentEngines",
			summary:  "一步看该机在跑哪些引擎（投影子端 /eggs：model · state · pid · port · managed · argv；`/status` 兜底）—— 含**引擎真命令行 argv**（GAP-20260926-01）；多机时不必来回 `egg ls` 或手搓 curl",
			usage:    "zerg agent engines <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentEnginesFields,
			endpoint: "GET http://<host>:<port>/eggs（子端回据 · /status 兜底；取不到 ⇒ blocked 退 8，不编）",
			// ★ `GAP-20260925-45`（2026-10-10）：引擎实时态三格（`inflight` / `last_output_ago_s` /
			//   `watchdog_verdict`）走**可选字段层**（`optFields`）—— 登在出口白名单里、`--json` 点得出来，
			//   但**不进** `agentEnginesFields` 那一份**冻结**的帮助 / 文档导出面字段表（与 `eggOptionalFields` 同一体例）。
			optFields: agentEnginesOptionalFields,
			run:       cmdAgentEngines,
		},
		{
			path:     []string{"agent", "slots"},
			kind:     "AgentSlots",
			summary:  "该机在跑引擎的槽位占用（直连引擎 /slots 摆槽位、/props 取 total_slots；引擎由子端 /eggs 认准）—— 「谁占了几号槽」的命令面读数",
			usage:    "zerg agent slots <机器名> [--model <名>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentSlotsFields,
			endpoint: "GET http://<host>:<engine_port>/slots（+ /props 的 total_slots；非 JSON 数组 ⇒ blocked 退 8）",
			run:      cmdAgentSlots,
		},
		// ---- P0 #232 第四/第五面（2026-10-08 · 本枚）：`agent workers set` / `agent scheduler set` 两条 D2 写面 ----
		// 落点 = **本地** `<仓根>/config/workers.yaml`（✗ 不碰主控 / ✗ 不碰远端）；口径逐字照同族写面先例
		// `itask interval set`（D2 + `--confirm=<本机名>` + `--yes` + `--dry-run` 计划件 + 落地值回读）。
		{
			path:     []string{"agent", "workers", "set"},
			kind:     "AgentWorkersSet",
			summary:  "设某机并发上限（D2 写面 · **只写本机** `config/workers.yaml` · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 远机 ⇒ 拒执）",
			usage:    "zerg agent workers set <机> --max N [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（**必须是本机**；远机需子端控制端点 · 本版未开 ⇒ 拒执退 2）"},
			fields:   []string{"machine", "max", "landed", "changed", "path"},
			danger:   &dangerSpec{dangerD2, "本机名", "把本机 devices 条目的 max_workers 设成 --max N（**持久化**到 config/workers.yaml —— scheduler 下次 NewScheduler 读到生效；0 = 暂停派活）—— 可逆：再设回旧值即回原态", "P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdAgentWorkersSetGuarded,
		},
		{
			path:     []string{"agent", "workers", "show"},
			kind:     "AgentWorkersShow",
			summary:  "读子端并发上限（投影本机 `<仓根>/config/workers.yaml` 的 devices 段：每机 hostname · max_workers · state · 是否本机；只读 · 一个字节不写）",
			usage:    "zerg agent workers show [--json <字段>]",
			arity:    "none",
			fields:   []string{"hostname", "max_workers", "state", "local"},
			endpoint: "",
			run:      cmdAgentWorkersShow,
		},
		{
			path:     []string{"agent", "scheduler", "set"},
			kind:     "AgentSchedulerSet",
			summary:  "设调度器旋钮（D2 写面 · 写 `config/workers.yaml` 顶 `scheduler:` 段 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2）",
			usage:    "zerg agent scheduler set [--poll-interval <N>] [--max-retries <N>] [--cooldown <N>] [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			fields:   []string{"path", "poll_interval", "max_retries", "cooldown", "changed"},
			danger:   &dangerSpec{dangerD2, "本机名", "改本地 `config/workers.yaml` 的 `scheduler:` 段（轮询间隔 / 死信阈值 / 失败冷却 —— scheduler.go 的 Start()/handleFailure() 读侧接线；未给值时用编译期常量兜底）—— 可逆：再设回旧值即回原态", "P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdAgentSchedulerSetGuarded,
		},
		// ---- 缺口 `GAP-20260927-195` · `agent` 族：`agent restart`（D3 · 已开放 · 照同族 `core restart` 形状）----
		{
			path:    []string{"agent", "restart"},
			machine: machineNodeName,
			kind:    "AgentRestart",
			summary: "重启子端进程（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`；**改令牌文件对运行中的子端无效，必须重启进程才重读令牌**；远端机走 ssh 属例外清单 ⇒ 退 8）",
			usage:   "zerg agent restart <机器名> --confirm=<机器名> --yes [--dry-run] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "host", "port", "branch", "readiness", "changed", "audit"},
			danger: &dangerSpec{dangerD3, "机器名",
				"重启该子端进程（**在跑的任务会被打断** · 起时/中断的连接都回不来）；治的是「改令牌文件对运行中的子端无效」",
				"缺口 GAP-20260927-195 · 照同族已开放的 `core restart`（family_core_restart.go）形状", true, dangerScopeRemote},
			opened: true,
			run:    cmdAgentRestart,
		},
		{
			path:    []string{"agent", "bootstrap"},
			machine: machineNodeName,
			summary: "子端引导的**域选择判定 + 可打印计划**（只读 · D3 真装未开放）—— 用户域 LaunchAgent vs 系统域 LaunchDaemon；取不到夹具 ⇒ 不给结论",
			usage:   "zerg agent bootstrap <机器名> --confirm=<机器名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"在目标机上装/起子端（**F-1 子端引导例外**：不经主控的显式命令；会改目标机状态）",
				"§十五.4 F-1 · §九 M20 · 开工单 T-43", true, dangerScopeRemote},
			run: cmdAgentBootstrap,
		},
		// ---- 批 D · T-44 `task` 族全动作面（§三 D 族 · 12 个动作名一个不差）----
		{
			path:     []string{"task", "submit"},
			danger:   &dangerSpec{dangerD2, "（提交主题）", "把任务入队（`POST /api/tasks` · 改控制面队列态 · 占机器与模型槽）", "GAP-20261005-40 · 同族先例 task cancel（D2）· §三 D 族", false, dangerScopeRemote},
			opened:   true,
			kind:     "TaskSubmit",
			summary:  "提交任务（D2 写面 · 全旗标 ⇒ API 请求体逐条对上）",
			usage:    "zerg task submit --desc <描述> [--model <模型>] [--priority <n>] [--slice-id <片>] [--depends-on <片>]… [--acceptance <判据>]… [--timeout <时长>] [--dry-run | --yes]",
			arity:    "none",
			args:     []string{"（旗标：--desc/--model/--priority/--slice-id/--depends-on/--acceptance）"},
			endpoint: "POST /api/tasks",
			run:      cmdTaskSubmit,
		},
		// ---- 批 D · T-11…T-14/T-17：`task cancel`（D2 写面 · 任务退场正门 · **本版已开放**）----
		// 与同族四条（`move/pause/resume/retry`）同形：D2 + `--confirm=<任务 id>` + `--yes`；
		// 本版真跑：确认档齐就 POST `/api/tasks/{id}/cancel`（幂等三成因走 stdout 字段 · 单 id 绝不给 8）。
		{
			path:     []string{"task", "cancel"},
			kind:     "TaskCancel",
			summary:  "撤下排队/暂停任务（D2 写面 · `--reason` 闭集 5 值必填 · `--confirm=<任务 id>` 与 `--yes` **同时到**才执行 · **本版已开放**）",
			usage:    "zerg task cancel <任务 id> --reason <superseded|obsolete|duplicate|abandoned|other> [--note <一句>] --confirm=<任务 id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"id", "outcome", "cancelled", "already_cancelled", "already_terminal"},
			danger:   &dangerSpec{dangerD2, "任务 id", "把该任务从 queue 摘出 → 置 status=cancelled → 落 history（可逆面：队列位次；**在飞的件本版不给撤** —— 先 terminate 另立账）；幂等三成因（cancelled / already_cancelled / already_terminal）走 stdout 字段、不走退码", "设计-任务退场正门-v0.2 §4.1/§11.2/§11.5 · 开工单 T-12/T-13/T-14/T-17", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/tasks/{id}/cancel?reason=<闭集 5 值 必填>&note=<一句 可选>",
			run:      cmdTaskCancel,
		},
		{
			path:     []string{"task", "diff"},
			kind:     "TaskDiff",
			summary:  "该任务工作树的**只读** `git diff --stat`",
			usage:    "zerg task diff <任务 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"id", "workdir", "diff_stat"},
			endpoint: "GET /api/tasks/{id}（取 workdir）+ 本机只读 git",
			run:      cmdTaskDiff,
		},
		{
			path:     []string{"task", "git"},
			kind:     "TaskGit",
			summary:  "该任务工作树的 git 面（分支 / HEAD / 未提交 / 与 main 的距离）",
			usage:    "zerg task git <任务 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"id", "workdir", "branch", "head", "dirty", "ahead_of_main"},
			endpoint: "GET /api/tasks/{id}（取 workdir）+ 本机只读 git",
			run:      cmdTaskGit,
		},
		{
			path:     []string{"task", "logs"},
			kind:     "TaskLogs",
			summary:  "该任务的日志（`/api/logs/task/{id}` **路由没接** ⇒ 不给结论，退码 8）",
			usage:    "zerg task logs <任务 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"id", "available", "detail"},
			endpoint: "GET /api/logs/task/{id}（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404）",
			run:      cmdTaskLogs,
		},
		{
			path:     []string{"task", "move"},
			kind:     "TaskMove",
			summary:  "挪动任务在队列里的位置（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）",
			usage:    "zerg task move <任务 id> --to <top|bottom|up|down> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"task_id", "action", "ok"},
			danger:   &dangerSpec{dangerD2, "任务 id", "改这条任务在队列里的次序（可能插到别人前面）", "§三 D 族 · 开工单 T-44 · 同族写面先例 `sched set`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/tasks/{id}/move?action=top|bottom|up|down",
			run:      cmdTaskMoveGuarded,
		},
		// ---- 批 D · T-45 `model` + `core` 两族（§三 E/C 族 · §7.1 `P11` · §十二 `P-029`）----
		{
			path:     []string{"model", "show"},
			kind:     "Model",
			summary:  "看一个模型（投影 /api/fleet/models 的单条；对象是**模型 id**，不是机器名）",
			usage:    "zerg model show <模型 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "peak_mem_gb", "file", "registry_id"},
			endpoint: "GET /api/fleet/models",
			run:      cmdModelShow,
		},
		{
			path:     []string{"model", "opts"},
			danger:   &dangerSpec{dangerD2, "模型 id", "改适配器参数（`set` 实时生效并**持久化**：`PUT /api/models/{name}/adapter-opts`；`get` 只读）", "GAP-20261005-40 · 同族先例 model add / model set（D2）· §三 E 族", false, dangerScopeRemote},
			opened:   true,
			kind:     "ModelOpts",
			summary:  "适配器参数（get 只读 / set 实时生效要 --yes）",
			usage:    "zerg model opts get <模型 id> [--json <字段>] | zerg model opts set <模型 id> --set k=v… [--dry-run | --yes]",
			arity:    "any",
			args:     []string{"动作（get|set）", "模型 id"},
			fields:   []string{"model", "schema", "note", "updated", "result"},
			endpoint: "GET|PUT /api/models/{name}/adapter-opts",
			run:      cmdModelOpts,
		},
		{
			path:     []string{"model", "verify"},
			kind:     "ModelVerify",
			summary:  "四方一次对账（只读）：(1)主控下发 (2)子端本地注册表 (3)模型档案 fleet 意图 + egg-profile 事实 (4)运行期实占（本版无通路）—— 人面 5 列 + 恒打 verdict（一致/不一致/缺席）",
			usage:    "zerg model verify 模型@机 [--json 字段]",
			arity:    "any",
			args:     []string{"对象（模型@机 · 例 example-35b-v2@Mr2109）"},
			fields:   []string{"egg_id", "side", "source", "value", "authority", "verdict", "key", "state", "note"},
			endpoint: "",
			run:      cmdModelVerify,
		},
		// ---- 批 B6（2026-10-20 · `zerg model record verify|probe|ls`）：独立入口
		// `core/cmd/zerg-model` 归入命令面 ----
		// 源件 668 行 · 三个子命令（`verify` / `probe` / `list`）· 十六枚旗标**逐字**保留。
		// ★ 并入**既有** `model` 族，✗ 不新开顶层族；但动作名**不能照抄** —— 源件的 `verify` 与 `list`
		//   在 `model` 族里都已被占了、且**语义不同**：
		//     `zerg model verify 模型@机` = 四方对账（`cmdModelVerify`，只读，对象是模型@机）
		//     `zerg model ls`            = 主控 `/api/fleet/models` 的机型投影（`cmdModelLs`）
		//   本件搬的是**另一件事**（标准 §三 的记录件面 · `core/internal/modelreg`）⇒ 挂一层 `record`
		//   中间段把这一面整块收纳（3 段路径，`resolve()` 已支持）；撞名的两条**一字不动**。
		// **双轨期**：`core/cmd/zerg-model` 二进制一字不动（它自己的 `0/1/2/3/4` 是**组件自有码空间**，
		//   `exitcode-namespaces.json` 里挂着号）。实现在 `family_model_record.go`，件头逐条写清与源件的
		//   六处差异（动作名 · 退码重取号 0/1/2/12/14 · 互斥旗标明拒 · `--json` 按 K2 口径 · `--strict`
		//   的精确点名 · 位置参数归用法错 2）。
		// 只读面（verify / ls）与写面（probe：`--out` 落件 / `--store` 进模型目录）混在同一条族段里：
		//   写面**不进危险表**，理由与 `model put/pull`（D2）不同 —— 这一面写的是**本机的记录件/模型目录**，
		//   真跑前有 `modelreg.Verify` 的门禁（不合标准即拒入目录、不写占位值凑绿），且 `--out` 的落点由
		//   调用者显式给；它与 `persona` 那类「改主控运行态」的写面不是一回事 ⇒ 无 `--yes`/`--dry-run` 档，
		//   与源件同形态（`--store` 幂等：同内容不重写、冲突拒覆盖）。
		{
			path:     []string{"model", "record", "verify"},
			kind:     "ModelRecordVerify",
			summary:  "校验一条模型登记记录（标准 §三）是否符合标准（只读）：`--integrity` 额外核对 files[] 与磁盘件（流式 sha256 + 大小）· `--in-place` 核「记录位置与身份是否自洽」（阶段 1 廉价篡改检出 · 不做签名 · 不代表权重可信）",
			usage:    modelRecordVerifyUsage,
			arity:    "any",
			args:     []string{"记录件路径（`--in-place` 档可给记录目录）"},
			fields:   []string{"path", "schema", "verdict", "level", "field", "detail", "id", "format_errors", "format_warns", "integrity_status", "integrity_name", "integrity_note"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordVerify,
		},
		{
			path:     []string{"model", "record", "probe"},
			kind:     "ModelRecordProbe",
			summary:  "探测一个模型（本地路径或端点 URL）并生成登记记录：默认只打印不写盘 · `--out <件>` 原子写（兄弟文件 `<版>.trace.json` / `<版>.capabilities.json` 另写）· `--store` 进模型目录 manifests/（同内容不重写 · 不合标准**拒入**）· 血缘 `--parent`/`--base` 只照抄显式入参、**不推断**",
			usage:    modelRecordProbeUsage,
			arity:    "any",
			args:     []string{"目标（本地模型路径或端点 URL）"},
			fields:   []string{"id", "version", "digest", "files", "capabilities", "errors", "warnings", "target", "endpoint", "engine", "out", "changed", "verdict"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordProbe,
		},
		{
			path:     []string{"model", "record", "ls"},
			kind:     "ModelRecordLs",
			summary:  "列出模型目录（manifests/）里的登记记录（只读）：逐条给 ID / 版本 / commercial / files / 能力 / error·warn / 落点，并标出「commercial != yes 不得作默认项」（标准 §五 红线）；目录里有不合标准的记录 ⇒ 退 1",
			usage:    modelRecordLsUsage,
			arity:    "none",
			args:     []string{"（不收位置参数：模型目录根走 `--root`、manifests 目录走 `--manifests`）"},
			fields:   []string{"id", "version", "commercial", "files", "capabilities", "errors", "warns", "path", "default_eligible", "detail"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordLs,
		},
		{
			path:     []string{"model", "put"},
			kind:     "ModelPut",
			summary:  "把模型的本地件搬到目标机的**显式落点**（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）：传输复用 `dplSSH` ＋ rsync · 落点必须显式给 `--to <路径>`（✗ 不猜默认）· 发前本机 sha256 ⟷ 对端回读**双侧对拍**，不一致 ⇒ 判红（退 1）",
			usage:    "zerg model put <模型@机> --to <路径> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"对象（模型@机 · 例 example-35b-v2@Mr2109）", "落点（--to：目标机上的**显式**路径）"},
			fields:   []string{"egg_id", "model", "machine", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict", "note"},
			danger:   &dangerSpec{dangerD2, "对象（模型@机）", "把该模型的本地件经 ssh/rsync 搬到目标机的**显式落点** `--to`（✗ 不猜默认路径）；发前本机 sha256 ⟷ 对端回读双侧对拍，不一致 ⇒ 判红；不碰主控运行态、不改名册件；可逆：删掉对端落点件即原状", "缺口账 GAP-20260925-25 · 同族写面先例 model add（D2）· §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			endpoint: "",
			run:      cmdModelPut,
		},
		{
			path:     []string{"model", "pull"},
			kind:     "ModelPull",
			summary:  "把模型件从**源机**拉到**本机的显式落点**（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）：`@机` = **对端机（= 源机）**（方向由动词给：put 本机→对端、pull 对端→本机）· 传输复用 `dplSSH` ＋ rsync · 落点必须显式给 `--to <本地路径>`（✗ 不猜默认）· 源机件回读 sha256 ⟷ 本机落点 sha256**双侧对拍**，不一致 ⇒ 判红（退 1）· `@本机` ⇒ 拒执（退 2）",
			usage:    "zerg model pull <模型@机> --to <本地路径> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"对象（模型@机 · @机 = **源机** · 例 example-35b-v2@ms02）", "落点（--to：**本机**上的**显式**路径）"},
			fields:   []string{"egg_id", "model", "machine", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict", "note"},
			danger:   &dangerSpec{dangerD2, "对象（模型@机）", "把该模型件从**源机**（`@机`）经 ssh/rsync 拉到**本机的显式落点** `--to`（✗ 不猜默认路径）；源机件回读 sha256 ⟷ 本机落点 sha256 双侧对拍，不一致 ⇒ 判红；不碰主控运行态、不改名册件；可逆：删掉本机落点件即原状", "缺口账 GAP-20261010-14 · 同族写面先例 model put（D2）· 设计稿 §2.4 甲案", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelPull,
		},
		{
			path:     []string{"core", "status"},
			kind:     "CoreStatus",
			summary:  "主控现状（投影 /api/core/status）",
			usage:    "zerg core status [--json <字段>]",
			fields:   []string{"ok", "pid", "started_at", "version"},
			endpoint: "GET /api/core/status",
			run:      cmdCoreStatus,
		},
		{
			path:     []string{"core", "logs"},
			kind:     "CoreLogs",
			summary:  "主控日志（`/api/logs` **路由没接** ⇒ 不给结论，退码 8）",
			usage:    "zerg core logs [--json <字段>]",
			fields:   []string{"available", "detail"},
			endpoint: "GET /api/logs（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404）",
			run:      cmdCoreLogs,
		},
		{
			path:     []string{"core", "daemon"},
			kind:     "CoreDaemon",
			summary:  "`daemon ls`：本机服务脚本逐件可查（scripts/svc/ 5 件）+ `--declared` 并给声明面两列与幽灵段（与 `doctor` 同源同值）",
			usage:    "zerg core daemon ls [--declared] [--json <字段>]",
			arity:    "any",
			args:     []string{"动作（ls）"},
			fields:   []string{"name", "script", "declared", "note"},
			endpoint: "",
			run:      cmdCoreDaemonLs,
		},
		// ---- 波① · T2 `Q-056`：现值面给人读三格（pid / 起时 / 命令行 · 2026-09-23）----
		// 今天**没有**这条命令（现跑「未知命令 "core ps"」）—— 幽灵/声明差要靠人肉 `ps | grep`
		// 加 `plutil -p` 才看得出（`Q-057` 的病根）。
		{
			path:     []string{"core", "ps"},
			kind:     "CorePs",
			summary:  "现值面逐条读（**三格**：pid / 起时 / 命令行）—— 行面 = 声明件点名的进程特征命中的进程（launchd 声明的 + ghost 幽灵都列）· 只读",
			usage:    "zerg core ps [--root <仓根> | --path <声明件>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无：现值面是「读全机」不是「要目标」）"},
			fields:   []string{"pid", "start", "command", "kind", "name", "owner"},
			endpoint: "",
			run:      cmdCorePs,
		},
		// ---- T2④ / T4①（2026-10-14 · 本单）：`core db <check|backup|restore|prune>` —— 虫族库
		// 备份 / 恢复 / 体检 / GFS 轮转的**唯一命令入口**。设计 §五 Q12 逐字：「✗ 不新开 `zerg db` 族」
		// ⇒ 挂**既有 `core` 族**下；登记形制照同族 `core daemon`（一条命令 · 子动作在 `inv.args[0]` 里派发）。
		// 实现件 = `core/cmd/zerg/family_core_db.go`（薄壳 · 逐字转发 `zdb` 已落接口 —— ✗ 命令层重写逻辑）；
		// ★ 与归档**共用同一枚 plist**（`com.zerg.dbbackup` · 先备份后归档）。
		{
			path:     []string{"core", "db"},
			kind:     "CoreDb",
			summary:  "虫族库五动作（`stamp`：**只打标不迁移** —— 只补 `PRAGMA application_id` 身份标（三态：==本标 空动作 / ==0 写一次＋回读自证 / 其它非 0 拒执），✗ 不跑迁移 ⇒ `user_version` 一字不动）（薄壳 · 转发 `core/internal/store/zdb`）：`check` **只读**体检（integrity_check ＋ foreign_key_check ＋ application_id ＋ user_version/page_count/journal_mode —— 真库上跑也一字节不写）· `backup` GFS 快照（checkpoint(TRUNCATE) → VACUUM INTO → 侧车八格 → prune 日14/周8/月12 · 保底 ≥3）· `restore` 从快照就位（VACUUM INTO → 体检 → 逐表行数对拍 → rename · 自证以侧车为准）· `prune` 逐代轮转（**只删匹配命名件**）",
			usage:    "zerg core db <check|backup|prune|stamp> [--db <库路径>] [--dry-run | --yes] [--json <字段>] · zerg core db restore --from <快照> --to <库路径> --confirm=<库路径> --yes [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"动作：check | backup | restore | prune | stamp"},
			fields:   coreDbFields,
			danger:   &dangerSpec{dangerD3, "库路径", "改虫族群库面：`restore` **覆盖 `--to` 指向的库**（自证 / 体检不过 ⇒ `zdb` 里 fail-closed，rename 之前就退）· `backup` / `prune` 写 / 删 `~/.zerg/backup/` 里的快照（GFS **只删匹配命名件** · 保底 ≥3）；`check` **只读**零写", "设计-虫族数据库-20261013 §四.2/§四.3 · §五 Q12/Q23/Q24 · 任务书 T2④/T4①", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdCoreDb,
		},
		{
			path:    []string{"core", "start"},
			machine: machineHostName,
			summary: "起主控（危险 D3 · 本版未开放）",
			usage:   "zerg core start [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "起主控进程（会绑端口 8580；已在跑时是**换件**前置）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "restart"},
			machine: machineHostName,
			summary: "重启主控（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`）· 审计留痕 + 就绪判据绑自己的 pid",
			usage:   "zerg core restart [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停 + 起主控（**整个虫群的控制面会断一会儿**）", "§三 C 族 · 开工单 T-45 · 缺口 Q-103", true, dangerScopeSystem},
			// `opened`: 真跑已开放（`--confirm=<主机名>` 与 `--yes` **同时到**才执行；`--dry-run` ⇒ 计划件 rc=0 ·
			// 缺确认档 ⇒ fail-closed rc=2）。动作只有一条：`launchctl kickstart -k gui/<uid>/com.zerg.core`
			// （定义不一致 / 未装载那两支不走 —— 归换件入口 `scripts/build/zerg-swap-core.sh`）。
			opened: true,
			run:    cmdCoreRestart,
		},
		// ---- 批 D · T-46 H 族四条（§三 H 族 · §7.1 `P10`/`P7`）----
		{
			path:     []string{"resource", "ls"},
			kind:     "Resource",
			summary:  "资源面（投影 /api/resources/ledger 或 /api/resources/{类型}）",
			usage:    "zerg resource ls [<类型>] [--json <字段>]",
			arity:    "any",
			args:     []string{"类型（可省）"},
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "vram_total_gb", "vram_used_gb", "vram_free_gb", "gpu_pct", "backend_state", "fit"},
			endpoint: "GET /api/resources/ledger | /api/resources/{type}",
			run:      cmdResourceLs,
		},
		{
			path:     []string{"resource", "ledger"},
			kind:     "Resource",
			summary:  "资源账本（投影 /api/resources/ledger）",
			usage:    "zerg resource ledger [--json <字段>]",
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "vram_total_gb", "vram_used_gb", "vram_free_gb", "gpu_pct", "backend_state", "fit"},
			endpoint: "GET /api/resources/ledger",
			run:      cmdResourceLedger,
		},
		{
			path:     []string{"gateway", "models"},
			kind:     "Model",
			summary:  "网关侧模型面（与 `model ls` **同源** —— 网关没有第二份模型表）",
			usage:    "zerg gateway models [--json <字段>]",
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdGatewayModels,
		},
		{
			path:     []string{"script", "ls"},
			kind:     "Script",
			summary:  "本机脚本清单（scripts/ · deploy/ · gateway/ 下的脚本件逐件 + 公开标记表）",
			usage:    "zerg script ls [--json <字段>]",
			arity:    "none",
			fields:   []string{"path", "public", "kind", "count_basis", "refs"},
			endpoint: "",
			run:      cmdScriptLs,
		},
		// ---- `A3`（`G-13`/`Q-002`）取「b」案：仓外台账（脚本现状清单）的**唯一写命令** + 写面进审计 ----
		// 已拍口径逐字（`设计-仓外台账写面-A3-b案-v1.0-20260923.md` §二）：台账**留仓外** + 一条唯一写命令
		// （三态照 `repo commit`：`--dry-run` 是**唯一**会返回 0 的那一态 · 缺 `--yes` ⇒ fail-closed 2）+ 审计。
		// ★ 与只读面**分家**：上面那条 `script ls`（`family_h.go cmdScriptLs`）**一字不动** ——
		//   只读面加一枚写旗标就变成第二条写路径（设计稿 §三）。
		{
			path:    []string{"script", "inventory", "sync"},
			kind:    "ScriptInventorySync",
			summary: "把**仓外**台账（脚本现状清单）按现跑重算（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计）",
			usage:   "zerg script inventory sync [--docs-root <Zerg-内部文档 根>] [--dry-run | --yes] [--by <谁>] [--json <字段>]",
			fields: []string{"result", "target", "rows_live", "rows_added", "rows_removed", "rows_unchanged",
				"declared_line", "inventory_before_sha256", "inventory_after_sha256", "audit_path"},
			danger:   &dangerSpec{dangerD2, "台账件", "把仓外台账的行面与两个 yes/no 列按现跑重算（可逆：写前备份 + 写后读回，复查不过逐字节写回）；**一条命令写**、**不许第二条写路径**", "设计-仓外台账写面-A3-b案-v1.0-20260923.md §二 · `A3` = `G-13`/`Q-002`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdScriptInventorySync,
		},
		// ---- 退场正门（设计稿 `Zerg-内部文档/01-设计/设计-退场正门-retire-20261006.md` ＋ 拍板 决-01/02/03/09 · 2026-10-07）----
		// 定位：**只管登记面退场**，✗ **不代管物理删除**（`git rm`）——真删交 git 历史＋提案审计（决-01）。
		// 真源**沿用** `core/internal/contract/adopt-retire.json` 的 `retire_ledger`（✗ 不另起第二真源 · 决-02）；
		// 目标件闭集 = **自研面全体 py/sh**（口径**单点真源** = `scriptRetireClosedSetExcludedPrefixes` · ✗ 不在此重抄前缀表以免长歪 · 决-12 · ★ 描述串与 `scriptRetireClosedSetDesc` 同源）；
		// 例外类 = **逐条登记名册 ＋ 理由 ＋ 保留代价栏**（同真源 `retire_exceptions` · 决-09）。
		// 危险档 D2（与 `inventory sync` 同档）：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计
		//   （审计先落盘 → `cp -p` 备份真源 → 原子写 → 写后读回，复查不过逐字节写回）。
		// 退码：0 成功 · 1 拒绝（例外类 / 目标不在闭集）· 2 用法错／不给结论（缺 `--yes`／缺 `--evidence`／
		//   目标不在盘／真源取不到）· 14 冲突（已有同 `object` 条目）。实现在 `family_script_retire.go`。
		{
			path: []string{"script", "retire"},
			kind: "ScriptRetire",
			summary: "退场正门（**只管登记面** · ✗ 不代管 `git rm`）：把一件自研面 py/sh 登记进 `retire_ledger`" +
				"（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计）；例外类件 / 目标不在闭集 ⇒ 1；" +
				"目标不在盘 / 真源取不到 ⇒ 2；已有同 `object` 条目 ⇒ 14",
			usage: "zerg script retire <件> [--dry-run | --yes] [--evidence <出处>]… [--by <谁>] [--json <字段>]",
			arity: "any",
			args:  []string{"件（仓内相对路径 · " + scriptRetireClosedSetDesc + "）"},
			fields: []string{"result", "object", "state", "closed_set", "in_exceptions",
				"ledger_entries_before", "ledger_entries_after", "ledger_before_sha256", "ledger_after_sha256", "audit_path"},
			danger: &dangerSpec{dangerD2, "登记面（`retire_ledger`）",
				"把一件自研面件登记为「已登记退场」（可逆：写前 `cp -p` 备份 + 写后读回，复查不过逐字节写回）；**只登记 · ✗ 不代管 `git rm`**（真删交 git 历史＋提案审计）",
				"Zerg-内部文档/01-设计/设计-退场正门-retire-20261006.md §一/§二/§三 · 拍板落定-所有稿-20261007.md 决-01/02/03/09", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdScriptRetire,
		},
		// ---- 缺口面 P0（缺口-命令面-20260921 §十一 · 2026-09-21）：今天手搓最多的一类先补上 ----
		// ── 缺口面 · `zerg find`（按名找件 · 2026-09-25 · 手搓 find 的替身）────────────
		{
			path:     []string{"find"},
			kind:     "Find",
			summary:  "按名字片段在仓内找件（受控遍历 · 输出相对路径/大小/修改时间）",
			usage:    "zerg find <名字片段> [--type file|dir] [--root <根>] [--limit N] [--json <字段>]",
			arity:    "any",
			args:     []string{"名字片段"},
			fields:   []string{"relpath", "size", "mtime"},
			endpoint: "",
			run:      cmdFind,
			// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修 ①）：`total` 走**可选字段层**（`optFields`，
			//  与 `code find` 的 `count` 同一体例：登在出口白名单、`--json total` 点得出来，但**不进**
			//  上面那份 `fields:` 冻结表 ⇒ 帮助面 / 命令页一行不动、`合法字段:` 那条错误面也一字不变）。
			//  逐行落一格 `total` = 本跑**真命中件数**（跑级真值 · 与包封 `meta.total` 同源同值）。
			optFields: []string{"total"},
		},
		// ── A2（2026-09-27）· 件名统一出口的**命令面**（《设计-件名统一出口与调用点判据-v0.1.md》
		//   §4.1 · P-7/P-10 · 任务清单 A2 · 缺口账 `GAP-20260927-216` 的 ②）─────────────
		//   本命令 = A1 出口（`core/internal/gitpaths`）的**唯一**命令面入口：取件名只经
		//   `gitpaths.List`，本命令**零 git argv**；四个面旗标互斥、缺省不猜；`--json` 带
		//   **字节面字段** `path_bytes`（全量小写十六进制，照 git2-rs `path_bytes` 形状）。
		//   实现件：`core/cmd/zerg/family_lsface.go`（★ 2026-09-27 当读：A1 已补出 `FaceOthers`，该件四面全接上出口、末段**无**遗留缺口）。
		{
			path:     []string{"ls-face"},
			kind:     "LsFace",
			summary:  "件名统一出口（A1 `core/internal/gitpaths`）的命令面：**逐字节真名**（转义形态在出口处硬失败 C-5）· 四个面旗标互斥、缺省不猜",
			usage:    lsFaceUsage,
			arity:    "none",
			args:     []string{"（不收位置参数 —— 面由旗标给、路径由 `--prefix <路径>` 给）"},
			fields:   lsFaceFields,
			endpoint: "",
			run:      cmdLsFace,
		},
		{
			path:    []string{"code", "find"},
			kind:    "CodeFind",
			summary: "在码里找一处东西在哪（只读取证 · 手搓 grep/git grep 的替身 · **扫工作树**：含未跟踪件与被忽略目录，比 `git grep` 的索引面多一片（两个面各扫多少件，命令每跑一次自己报一行「扫了 N 件」—— **不在这里写死**，写了就会烂；差值由运行时的数说话）；跳过 >2MB 的件）",
			usage:   "zerg code find <正则> [--path <目录或单件·仓外根用绝对路径显式指（只读）>] [--glob <模式>] [--limit <N>] [-i|--ignore-case] [--after <N>|-A<N>] [--before <N>|-B<N>|-C<N>] [--kind <代码|注释>] [--timeout <秒>] [--count | --files-only] [--full] [--hidden] [--include-runtime] [--tracked] [--json <字段>]",
			arity:   "any",
			args:    []string{"正则（**Go/RE2 语法** —— 不是 POSIX BRE；**交替**用**裸** `|`（例：a|b）；写 `\\|` 会被**拒**：本命令退 2 并给替身，因 RE2 里 `\\|` 是**字面竖线**不是交替）"},
			// ★ `GAP-20260928-118`（2026-09-28 · 本枚）：字段表补上 `truncated` —— 截断位此前
			// 只能从顶层包封读，字段面没有直读口（给 `truncated` 即报未知字段）。三档面（默认 /
			// `--count` / `--files-only`）逐行都产出它（**跑级真值**：本跑真裁了 = true，没裁 = false）
			// ⇒ 表与产出集同源；不给这一格时输出逐字节不变（`marshalObject` 只投影点名的格）。
			// ★ `GAP-20260929-29`（2026-10-04）：字段表再补上 `face`（工作树面 / 索引面）—— 与
			// `meta.face` 同源（`codeFaceWorktree`）⇒ 条目面与包封面各有一个直读口；同样只在
			// `--json` 那一态落键、不给这一格时输出逐字节不变。
			fields: []string{"path", "line", "text", "kind", "match", "truncated", "face"},
			// ★ `GAP-20260928-142`（2026-10-10 · 本枚）：`count` 走**可选字段层**（`optFields`）
			// —— 登在出口白名单里、`--json count` 点得出来（默认面 = 本跑命中条数 · `--count` 面
			// = 按件命中数既有语义），但**不进**上面那份文档/帮助导出面的**冻结表** ⇒ 不造
			// 「表列着、面不产出」的反例，也无需重生成命令页（与 `approveOptionalFields` 同一体例）。
			optFields: []string{"count", "path_kind"},
			endpoint:  "",
			run:       cmdCodeFind,
		},
		{
			path:     []string{"code", "show"},
			kind:     "CodeShow",
			summary:  "看源码里**某一行**长什么样（带 `件:行` · 只读取证 · 手搓 `sed -n` / `awk` 的替身）· 纯件名 = **整件读**（缺口 `GAP-20261001-59` · 手搓 `read_file` 的正门）",
			usage:    "zerg code show <件:行 | 件:起-止 | 件> [--ctx <N>] [--batch <件:行,…>] [--full] [--json <字段>]",
			arity:    "any",
			args:     []string{"件:行（件 = 仓相对路径 · 行 = 正整数）· 纯件名 = 整件读"},
			fields:   []string{"path", "line", "text", "target"},
			endpoint: "",
			run:      cmdCodeShow,
		},
		{
			path:     []string{"code", "diff"},
			kind:     "CodeDiff",
			summary:  "按件与 HEAD 的差（只读 · 手搓 `git diff` 的替身；`--stat` 另附整段差异统计）",
			usage:    "zerg code diff <件> [--stat] [--json <字段>]",
			arity:    "any",
			args:     []string{"件（仓相对路径）"},
			fields:   codeDiffFields,
			endpoint: "",
			run:      cmdCodeDiff,
		},
		// ---- 码面 R5 四枚（缺口 `GAP-20260927-235/236/237` · `GAP-20260928-30` · 2026-10-04）----
		// 实现件已落（`family_code.go` / `family_h.go`）⇒ 本处只做**命令树登记**（同一批 code 族）。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半（详见收口单）。
		{
			path:     []string{"code", "callers"},
			kind:     "CodeCallers",
			summary:  "反查一个函数的**全部调用点**（只读 · 手搓全仓搜的替身 · 缺口 GAP-20260927-237）· role=def 标定义行",
			usage:    "zerg code callers <函数名> [--func <函数名>] [--path <目录或单件>] [--glob <模式>] [--limit <N>] [--full] [--json <字段>]",
			arity:    "any",
			args:     []string{"函数名（与 --func 两收 · 后者优先）"},
			fields:   []string{"path", "line", "role", "text"},
			endpoint: "",
			run:      cmdCodeCallers,
		},
		{
			path:     []string{"code", "denom-recount"},
			kind:     "CodeDenomRecount",
			summary:  "两份件清单的**对称差**（只读 · 「计数差 1」时定点名那 1 件 · 缺口 GAP-20260927-236）",
			usage:    "zerg code denom-recount <左件> <右件> [--json <字段>]",
			arity:    "any",
			args:     []string{"左清单件（一行一件）", "右清单件"},
			fields:   []string{"path", "side"},
			endpoint: "",
			run:      cmdCodeDenomRecount,
		},
		{
			path:     []string{"code", "git-two-way"},
			kind:     "CodeGitTwoWay",
			summary:  "同一 git 命令**带/不带**某全局选项两版对比（只读诊断 · 缺口 GAP-20260927-235）",
			usage:    "zerg code git-two-way <k=v 或 -开关> -- <git 参数…> [--json <字段>]",
			arity:    "any",
			args:     []string{"全局选项", "git 参数（写在 -- 之后，防被当 zerg 旗标）"},
			fields:   []string{"side", "text"},
			endpoint: "",
			run:      cmdCodeGitTwoWay,
		},
		{
			path:     []string{"code", "audit-readings"},
			kind:     "CodeAuditReadings",
			summary:  "列出注释里的**绝对读数** + 真源取法（只读 · **只列候选不下判断** · 缺口 GAP-20260928-30）",
			usage:    "zerg code audit-readings --path <件或目录> [--glob <模式>] [--limit <N>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：件/目录由 --path 给）"},
			fields:   []string{"path", "line", "reading", "hint", "text"},
			endpoint: "",
			run:      cmdCodeAuditReadings,
		},

		// ---- 同值异名常量的只读面（缺口 `GAP-20261001-69` 的前置工具 · 本枚）----
		// ★ 只读（L0）：零写盘；**不改**任何重复常量、**不改**任何调用点 —— 本单只造工具。
		{
			path:     []string{"code", "const-dup"},
			kind:     "CodeConstDup",
			summary:  "扫**同值异名常量**（只读 · 同一个字符串值被多个常量各抄一份 ⇒ 逐条列 值 / 保留名 / 被废弃名·件:行 / 归属类）· 缺口 `GAP-20261001-69` 的前置工具",
			usage:    "zerg code const-dup --path <件或目录> [--glob <模式>] [--root <仓根>] [--json <字段>] [--fail-on-hit]",
			arity:    "none",
			args:     []string{"（不收位置参数：件/目录由 --path 给）"},
			fields:   ccdFields,
			endpoint: "",
			run:      cmdCodeConstDup,
		},
		{
			path:     []string{"repo", "status"},
			kind:     "RepoStatus",
			summary:  "看仓脏没脏 / HEAD 在哪 / 有没有别人在写它（手敲 git status 的替身）· `--root` 给 ≥2 次 ⇒ **多仓汇总**（一条只读命令出两仓 HEAD + 脏件数；写旗标一律拒 2）· **三态面**：行面七栏 `head/branch/path/status/untracked/mtime/sha256`，其中 `untracked` 栏只答「**未被忽略的真未跟踪**」（**件数随仓变 · 不在这里写死任何数** —— 写了就会烂；要数就现跑一条，按行面 `path` 前缀数「`untracked`=是」的行）；**被忽略**（`.gitignore` 命中）态现读**无栏** ✗",
			usage:    "zerg repo status [--root <仓根>] [--json <字段>]",
			fields:   []string{"head", "branch", "path", "status", "untracked", "mtime", "sha256", "head_files"},
			endpoint: "",
			run:      cmdRepoStatus,
			// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修 ①）：`total` 走**可选字段层**（`optFields`，
			//  与 `code find` 的 `count` 同一体例：登在出口白名单、`--json total` 点得出来，但**不进**
			//  上面那份 `fields:` 冻结表 ⇒ 帮助面 / 命令页一行不动、`合法字段:` 那条错误面也一字不变）。
			//  逐行落一格 `total` = 本跑**真合计处数**（与包封 `meta.total` 同源同值）。
			optFields: []string{"total"},
		},
		// ---- T20 正门一条（本枚 · **只增**）----
		// 「报行数三格」的机检面：判一份 diffstat 报没报齐 `原始 diff / 增 / 减`。
		// ★ 不退 `cmdRepo` 分派：本族 `repo status` 已是**直挂处理器**（`run: cmdRepoStatus`）⇒
		//   照它同规把 `run` 指向本枚新增的 `cmdRepoDiffstat`（零改既有分派 · 零改 `repo.go`）。
		// ★ 写旗标一律拒（`repoWriteWords`）—— 与 `repo status` 同一道只读牙（同族同守）。
		{
			path:     []string{"repo", "diffstat"},
			kind:     "RepoDiffstat",
			summary:  "报数三格机检（`原始 diff` / `增` / `减`）：三格齐 ⇒ 退 0；**只给净数（或缺任一格）⇒ 未报 ⇒ 判不了（退 2）** · 件读不到 ⇒ 判不了（退 8）· 只读",
			usage:    "zerg repo diffstat --report <报告件> [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：报告件由 `--report <件>` 指）"},
			fields:   repoDiffstatFields,
			endpoint: "",
			run:      cmdRepoDiffstat,
		},
		// ---- `GAP-20260928-98` 族（2026-09-28 · laneGS · 用户拍「按推荐」= laneGJ 设计稿 §二 乙档）----
		// 「工作树脏件 ↔ 审计行」的**对拍面**：分母 = `git status --porcelain=v1` 的**已跟踪 `M`/`A` 行**
		// （**不由审计定** —— 审计定的分母会让「零审计行的件」永远无对拍 = 最硬的假绿面）；
		// 对拍 = 工作树 `sha256` ↔ 审计里该件**最新**一条 `edit` 行的 `after_sha256`。
		// ★ **纯只读 · 只报告档（起手档）**：有未覆盖件仍退 0（「无记录」是状态不是错）· 不进任何必跑路径。
		// ★ 与 `repo status` 的口径**不相抵**：判「哪些件脏」的真源**仍是 git porcelain**（family_repo.go 那句
		//   「不自己比 mtime/sha」）；本命令比的是**审计行 vs 工作树身份**（两枚既有 sha 面的对拍），
		//   不是自己造第二套判脏口径 —— 候选实现面 = 只读命令 `core/cmd/zerg/family_audit.go`。
		{
			path:    []string{"audit", "cover"},
			kind:    "AuditCover",
			summary: "对拍「工作树脏件 ↔ 审计行」：分母 = `git status --porcelain=v1` 的**已跟踪 `M`/`A` 行**（真源是 git 自己的面 · **不由审计定**）· 逐件比「工作树 `sha256` ↔ 审计里该件最新一条 `edit` 行的 `after_sha256`」· 四判词 `有记录/无记录/记录过期/读不到` · 逐条点名未覆盖件（件·状态·工作树 sha16·审计有无行）+ 计数 · **只报告档**：有未覆盖仍退 0（不进任何必跑路径）· `8` = 读不到/**分母 0 判不了**（不当绿）· 与 `GAP-20260928-79` 族对偶（79 = 有行但回指不到授权物 · 本格 = 压根没有行）",
			usage:   auditCoverUsage,
			arity:   "none",
			args:    []string{"（不收位置参数 —— 面由 `--file <件>` 收窄、仓根由 `--root <仓根>` 给）"},
			fields:  auditCoverFields,
			// ★ 缺口 GAP-20260926-56（批⑤ · 2026-10-13）：`verified` 走**可选字段层**（`optFields`，与
			//   `code find` 的 `count` 同一体例）—— 「该件有没有改动记录」的机读直读口，登在出口白名单、
			//   `--json` 点得出来，但不进 `fields` 那份冻结帮助 / 文档导出面字段表 ⇒ 不重生命令页。
			optFields: []string{"verified"},
			endpoint:  "",
			run:       cmdAuditCover,
		},
		// ---- `GAP-20261010-20`（2026-10-17）：**仓内硬编码机器地址**的扇盘清单（只读 · L0）----
		// 拍定「做新命令、**不**扩既有断言」：既有 `check-hardcoded-private-paths` 是**门**（判红用），
		// 本命令是**扇盘工具**（给人看清单：件:行:内容 + 总数 + 按目录分组计），默认 rc=0；
		// 只有显式 `--fail-on-hit` 才把命中 > 0 透成 1。★ 规则表/白名单**复用**判据门那两张（不立第二份）。
		{
			path:     []string{"audit", "hardcoded-addr"},
			kind:     "AuditHardcodedAddr",
			summary:  "仓内「硬编码机器地址」**扇盘清单**（只读 · 零写盘 · L0）：扫 `core/` `agent/` `shared/` `scripts/`（不含 `ui/`/依赖树/点目录），逐处点名 `件:行:内容` + 命中总数 + **按目录分组计（处·件）**；规则与值白名单**直接复用**判据门 `check-hardcoded-private-paths` 的三族规则（`192.168.x` / 集群 `10.0.x` / 机器主机名 `*.local`；回环与文档段是声明过的合法写法 ⇒ 不报；裸端口数字不判）· 排除 `*_test.go` 夹具 / 注释行 / Rust 夹具段 / 二进制件 · 豁免件 `scripts/audit-addr-allow.txt`（一行 `路径:子串` · 写坏的行 ⇒ 2，不许静默失效）· **默认清单档：有命中仍退 0**；`--fail-on-hit` ⇒ 命中 > 0 退 1（供门/CI）· `8` = 仓根解析不到 / 扫描面 0 件（空转 ≠ 干净）",
			usage:    auditAddrUsage,
			arity:    "none",
			args:     []string{"（不收位置参数 —— 仓根由 `--root <仓根>` 给）"},
			fields:   auditAddrFields,
			endpoint: "",
			run:      cmdAuditHardcodedAddr,
		},
		// ---- D3b 第三步（2026-09-21）：**提交面**（缺口-命令面 §九 I4）----
		// 按文件名暂存（禁 `git add -A`）· 过快速档才放行 · 禁 `--no-verify` · 提交信息模板。
		{
			path:    []string{"repo", "commit"},
			kind:    "RepoCommit",
			summary: "提交：**按文件名逐件暂存**（禁 `git add -A`）· 或**点名单路径**（`--only <路径…>`：索引面允许非空、别人的暂存只许多不许少）· **过快速档才放行** · 禁 `--no-verify`（例外走 `--waive <步名> --reason <…>` 并进审计）",
			usage:   "zerg repo commit --message <题> (--file <件>… | --only <路径>[ --only <路径>]… | --removed <路径>…) [--proposal <提案 id>] [--by <谁>] [--trace <id>] [--criterion <判据>] [--waive <步名> --reason <理由>] [--allow-stale-gates <一句话理由>] [--no-fast] [--json <字段>] [--dry-run] [--yes]",
			arity:   "any",
			args:    []string{"提交主题（--message）", "逐件点名（--file · 可重复）或点名单路径（--only · 可重复 · 可 `--only=<路径>`）"},
			fields:  repoCommitFields,
			danger:  &dangerSpec{dangerD2, "提交主题", "把点名的件提交（可逆：`git reset --soft HEAD~1`）；**先跑快速档**，rc≠0 不提交（要带账放行得 `--waive <步名> --reason <…>`）", "缺口-命令面 §九 I4 · §九 M3 C5 · D3b 第三步 · 缺口 Q-104", false, dangerScopeRepo},
			// `opened`: 真跑已开放（`--yes` 就执行 —— 默认模式逐件暂存 / `--only` 模式点名单路径；D2 可逆）。
			opened:   true,
			endpoint: "",
			run:      cmdRepoCommit,
		},
		{
			path:    []string{"gate", "explain"},
			kind:    "GateExplain",
			summary: "读懂某一步到底在判什么（scope/模式/判据/退码口径/日志路径/出处文件:行 —— **精确匹配**步名）",
			usage:   "zerg gate explain <步名> [--json <字段>]",
			arity:   "any",
			args:    []string{"步名（与 `zerg gate ls` 逐字相同）"},
			fields:  []string{"scope", "mode", "criterion", "verdict", "exit", "log", "source", "command", "script_say"},
			// ★ 本单（缺口 `GAP-20260929-28` 剩面 · 2026-10-14）：`command_kind`（第五格形态：
			//   字面命令 / 变量间接 / 命令替换）与 `command_resolved`（真身：变量定义行 + 可能值集 /
			//   命令替换点名）走**可选字段层**（`optFields`）—— 它们由 `family_gate_explain.go` 一处算、
			//   与 `gate find` 同款 ⇒ `--json command_kind` 点得出来，但**不进**冻结 `fields:`
			//   ⇒ 帮助面与命令页一行不动（与 `gate results` 的 `source` 同体例）。
			optFields: []string{"command_kind", "command_resolved"},
			endpoint:  "",
			run:       cmdGateExplain,
		},
		// ── `danger`（**D2**）：本表项**真写盘**（修前无 `danger` ⇒ `zerg help` 把它判成安全档 ✗）
		//   定档理由（逐条读 `family_gate_matrix.go:cmdGateMatrix` 得）：`--out <件>` 那一支
		//   `os.WriteFile` 写一份逐格 TSV —— 只**新建/覆盖产出件**；不删件、不改矩阵真源
		//   `testdata/cli-matrix.json`、不改任何仓内真值 ⇒ 可逆（删掉/换回落点件即回原状）
		//   ⇒ **D2**（`--yes` 即可），不是 D3。
		//   ★ 三态面**已接**（2026-09-27 · `family_gate_matrix.go` 的 `gateMatrixOutPlan` + 三道判，
		//   全在 `os.WriteFile` **之前**）⇒ `usage` 串的互斥形态 `[--dry-run | --yes]` = **全树 19 条真判「两枚同给 ⇒ 2」命令的准形**
		//   （**2026-09-27 反向对齐**：`0b832b3f` 曾按「同给 ⇒ `--dry-run` 优先」把另 18 处收成 `[--dry-run] [--yes]`；`48cfef8a` 后实现改判 2 ⇒ 那 18 处已回改到本形态）：
		//   `--dry-run` ⇒ 计划件走 stdout + rc=0 且**一个字节都不落**（不建件、不建目录）；
		//   缺 `--yes` ⇒ 计划件走 stderr + rc=2（fail-closed）；`--dry-run` 与 `--yes` 同给 ⇒ rc=2。
		//   先看计划件：`zerg gate matrix --out <件> --dry-run`（同族先例 `zerg gap export`）。
		{
			path:    []string{"gate", "matrix"},
			kind:    "GateMatrix",
			summary: "命令面自己的 must-fail 矩阵（逐格可读可导 —— 新增命令照着它补格）",
			usage:   "zerg gate matrix [--out <件>] [--dry-run | --yes] [--json <字段>]",
			fields:  []string{"command", "case", "want_rc", "why"},
			danger: &dangerSpec{dangerD2, "落点件（`--out`）",
				"往 `--out <件>` 写一份逐格 TSV（command/case/want_rc/want_stdout_bytes/argv/why）；矩阵真源 `testdata/cli-matrix.json` 与仓内真值一个字不动（可逆：删掉落点件即回原状）",
				"缺口-命令面-20260921 §十一 P0-5 · 同族写面先例 `zerg gap export`（`--out` 真写盘）· §九 M3 C1", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGateMatrix,
		},
		{
			path:     []string{"gate", "bench"},
			kind:     "GateBench",
			summary:  "量门禁耗时（逐趟 real/user/sys + 中位/最差 + 门禁身份 sha256）",
			usage:    "zerg gate bench [--fast | --scope <s>…] [--repeat n] [--json <字段>]",
			fields:   []string{"run", "real_ms", "user_ms", "sys_ms", "rc", "log"},
			endpoint: "",
			run:      cmdGateBench,
		},
		// ── 门禁面 · `zerg ui i18n`（i18n 门进命令面 · 2026-09-25 · 缺口 C-10）──────────
		//   病灶：i18n 门在命令面**没有入口** ⇒ 人要手搓 `python3 ui/scripts/check-i18n.py`，
		//   退码与 stdout/stderr 全靠人盯。本条目 = 该脚本的命令面等价物：
		//   **脚本退多少，命令面退多少**（薄壳，不翻译门禁失败）。
		{
			path:     []string{"ui", "i18n"},
			kind:     "I18n",
			summary:  "i18n 门禁（四道门 G1..G4 · 脚本退多少命令面退多少）",
			usage:    "zerg ui i18n [--json <字段>]",
			fields:   []string{"rc", "verdict"},
			endpoint: "",
			run:      cmdI18n,
		},
		// ── S7 · 5 键归口（ui 族扩容）：`check-i18n`/`i18n-audit` 两件独立脚本收成命令面 ────────────
		// 归口件：`~/.zerg/state/②类-5键-CLI工具子命令归口-20261007.md`（② 类接线 · 5 键 · CLI 工具子命令）。
		// 底层唯一实现 = `i18n_audit.go`（`ui/scripts/i18n_audit.py` 的 Go 真搬体）；命令面**只做入口 ＋ 转发**
		// （连退码也不改写）—— ✗ 不重写真搬体里任何一段。纯分析工具 · 无自动调用者 ⇒ 排末。
		{
			path:        []string{"ui", "i18n-audit"},
			summary:     "i18n 中文字面量审计（纯分析工具 · 落点走 `ZERG_I18N_AUDIT_OUT` · 底层 = `i18n_audit.go` 真搬体 · 命令面只转发不翻译）",
			usage:       "zerg ui i18n-audit [classify|keys|all]",
			arity:       "any",
			args:        []string{"模式（classify/keys/all · 缺省 all · 旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdI18nAudit,
		},
		// ── 缺口面 P0 之外 · `zerg doc meta fill`（批量回填文件头 · 2026-09-21）──────────────
		//   病灶原样（开工记录 D3 §三）：③ 那 63 篇的抬头是**一次性 `/tmp` 脚本**回填的 ——
		//   命令面**没有「批量改字段」这条路**。本条目就是那次脚本的命令面等价物。
		{
			path:    []string{"doc", "meta"},
			kind:    "DocMeta",
			summary: "文档元数据面（动作 `fill` = 批量回填文件头：只填机械可判的日期 + 不开源标注 · 默认干跑 · 写审计）",
			usage:   "zerg doc meta fill [--scope devdocs | <目录>] [--docs-ver <X.Y.Z>] [--dry-run] [--by <谁>] [--json <字段>] [--yes]",
			arity:   "any",
			args:    []string{"动作（本版只有 fill）"},
			fields:  docMetaFillFields,
			danger: &dangerSpec{dangerD2, "（被扫根）",
				"回填文件头（日期 + 不开源标注）—— 只加机械可判的抬头行、不碰正文语义；可回滚 = git",
				"缺口-命令面-20260921 §八 H2 · 开工记录 D3 §三 新增 1 条 · D3③-a 的等价命令", false, dangerScopeRepo},
			// `opened`: 真跑已开放（`--yes` 就回填文件头；审计先落盘 + 回读对拍 sha256）。
			opened: true,
			run:    cmdDocMeta,
		},
		// ── 缺口面 `GAP-20260927-129` ④「把『定格』做成可现算读数」：只读命令 `zerg doc consumers` ──
		//   逐消费者现读其**解析出的版本目录**（zerg_overview / help export / doc meta fill / doc_search）·
		//   缺件不静默（版本档案取源根判不了 ⇒ 退 8 并点名候选）· 判定口径与判据 `check-doc-consumers`
		//   共用同一份现读（✗ 不两处各写一套）。
		{
			path:    []string{"doc", "consumers"},
			kind:    "DocConsumers",
			summary: "四个消费判据各自解析出的「定格版本目录」现读读数（**只读**：不写盘、不起子进程、不联网 · 逐消费者可比出定格是否分叉 · 判据 `check-doc-consumers` 共用同一读数）",
			usage:   "zerg doc consumers [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数）"},
			fields:  docConsumersFields,
			run:     cmdDocConsumers,
		},
		// ── S7 · 5 键归口（doc 族扩容）：3 件独立工具收成命令面（`kb-docs-sync`=A18 · `gen-tools-md` · `doc-index-gen`）──
		// 归口件：`~/.zerg/state/②类-5键-CLI工具子命令归口-20261007.md`。三件底层唯一实现 = 各自 Go 真搬体
		// （`kb_docs_sync.go` / `gen_tools_md.go` / `doc_index_gen.go`）；命令面**只做入口 ＋ 转发**（连退码也不改写）
		// —— ✗ 不重写真搬体里任何一段。`gen-tools-md`/`index-gen` 被门 `check-doc-freshness` 真跑。
		{
			path:        []string{"doc", "kb-sync"},
			summary:     "KB 文档同步（核验入库后把候选标记进 kb registry · 底层 = `kb_docs_sync.go` 真搬体 · 命令面只转发不翻译 · = A18）",
			usage:       "zerg doc kb-sync [--mark <file|全部>] [--help]",
			arity:       "any",
			args:        []string{"旗标逐字来自 tools/kb_docs_sync.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocKbSync,
		},
		{
			path:        []string{"doc", "gen-tools-md"},
			summary:     "生成 TOOLS 文档（publish/docs/TOOLS.*.md · 被门 `check-doc-freshness` 真跑 · 底层 = `gen_tools_md.go` 真搬体）",
			usage:       "zerg doc gen-tools-md [--help]",
			arity:       "any",
			args:        []string{"旗标逐字来自 tools/gen_tools_md.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocGenToolsMd,
		},
		{
			path:        []string{"doc", "index-gen"},
			summary:     "生成某版本目录的 `INDEX.md`（被门 `check-doc-freshness` 真跑 · 底层 = `doc_index_gen.go` 真搬体）",
			usage:       "zerg doc index-gen <版本目录> [--out <INDEX.md>] [--help]",
			arity:       "any",
			args:        []string{"版本目录", "旗标逐字来自 tools/doc_index_gen.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocIndexGen,
		},
		{
			path:     []string{"port", "ls"},
			kind:     "PortLs",
			summary:  "看某个端口被谁占着（含 pid/ppid/inode/在跑件路径 · 手敲 lsof 的替身）",
			usage:    "zerg port ls [<端口>] [--json <字段>]",
			arity:    "any",
			args:     []string{"端口（可省：缺省列声明面三个端口）"},
			fields:   []string{"port", "pid", "ppid", "process", "sock", "path"},
			endpoint: "",
			run:      cmdPortLs,
		},
		{
			path: []string{"net", "probe"},
			kind: "NetProbe",
			// ★ 字段表必须写成 `[]string{…}` **字面量**（不许抽成 `xxxFields` 变量）：
			//   `scripts/gates/check-cli-contract.py` 的 `FIELDS_RE` 只认这一种形状
			//   （`fields:\s*\[\]string\{…\}`）—— 抽成变量会让它读成「这条没有字段表」，
			//   于是重冻时派生的候选格变成「说明面不认 --json」，那句话与本命令**当场相反**。
			//   先例：`port ls` 也是字面量（同一条纪律，只是此前没人写下理由）。
			summary:  "环境面**只读**投影（代理在哪 / 走不走得通 / 直连还是代理 · 取不到 ⇒ 8）",
			usage:    "zerg net probe [--json <字段>]",
			fields:   []string{"proxy", "reachable", "mode"},
			endpoint: "",
			run:      cmdNetProbe,
		},
		{
			// 组4 §二.4 `W-50`（`研-禁:129` `R-16`）· 任务单序132 · 与组3 `Q-026` 同条：
			// 凡「在 / 不在」的答案都要带「口径 + 树 `head_sha`」；两代产出树答案不同 ⇒
			// 判「口径不同」· **不许并成一个数**（一树一行 ⇒ 两行）。
			path: []string{"publish", "tree", "has"},
			kind: "PublishTreeHas",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` 那条**同一条纪律**
			//   （契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 那一条被读成「没有字段表」）。
			summary:  "产出树「在 / 不在」**只读**读数（一树一行 · 每行带**口径 + 树 `head_sha`** · 树身份取不成 ⇒ 8 · **不给结论**）",
			usage:    "zerg publish tree has <件> --tree <树>… [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（相对树根的相对路径 —— 形状不合口径 ⇒ 2）"},
			fields:   []string{"tree", "caliber", "head_sha", "present"},
			endpoint: "",
			run:      cmdPublishTreeHas,
		},
		{
			// 病 · 已入账 `GAP-20260927-407`（缺面）：「这件**会不会**发」在命令面上**没有免树正门**
			// （`publish tree has` 必须有现成成品树）⇒ 只能绕、只能代码坐实，拿不到一个可手敲的口。
			// 本条目 = 发布判定链的**只读**机器面（免树）：逐层现读真源件复算 ——
			//   ① publish/whitelist.txt ② EXCLUDES（scripts/build/publish-public.sh）
			//   ③④ publish/mirror-public-lib.py 的 DROP_EXACT/DROP_PREFIX ⑤ 同件 map_path 映射丢弃。
			// 链真源读不到 ⇒ 8 · **不给结论**（空表会读成比真值更宽的答案）。
			path: []string{"publish", "set"},
			kind: "PublishSet",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` / `publish tree has`
			//   两条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 该条被读成「没有字段表」）。
			summary:  "「这件会不会发 + 哪一层拦的」**只读**读数（**免树** · 逐层现读判定链真源件复算 · 链真源读不到 ⇒ 8 · **不给结论**）· ★ **身份格 `caliber`（口径）不是可选投影格**：`--json` 里**必须点名**它，缺 ⇒ 2 `identity_cells_required`（缺口 `GAP-20260927-415`）",
			usage:    "zerg publish set <仓内路径>… [--json <字段>]",
			arity:    "any",
			args:     []string{"仓内路径（相对仓根 —— 形状不合口径 ⇒ 2）"},
			fields:   []string{"path", "caliber", "will_publish", "layer", "public_path", "private_face", "non_blob_basis"},
			endpoint: "",
			run:      cmdPublishSet,
		},
		{
			path:    []string{"publish", "preflight"},
			kind:    "PublishPreflight",
			summary: "公开面预检（**只读** · 跑 scripts/build/publish-preflight.sh <产物目录> · 不改任何状态）",
			usage:   "zerg publish preflight <产物目录>",
			arity:   "any",
			args:    []string{"产物目录（`publish run` 出的镜像树）"},
			run:     cmdPublishPreflight,
		},
		{
			path:    []string{"publish", "run"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "出公开镜像树（本地 dist/<版本>/release · 白名单+排除项+脱敏 · 不出网、不推送 · 底层 scripts/build/publish-public.sh）", "GAP-20261005-39 · family_publish_run.go:275 自印「危险档：D3」· §17.2", true, dangerScopeRepo},
			opened:  true,
			kind:    "PublishRun",
			summary: "出公开镜像树（**本地** dist/<版本>/release · 白名单+排除项+脱敏 · 不出网、不推送）",
			usage:   "zerg publish run [--node <机>] [--check-excludes-parity] [--dry-run | --confirm=<主机名> --yes]",
			// 本版已开放（Mr2109 2026-09-25「脱敏推」授权）：真跑只写**本地**产物目录。
			// 「推远端」不在命令面 —— 不可逆动作按规矩等 Mr2109 发话后单独执行（不另开执行路径）。
			run: cmdPublishRun,
		},
		{
			// ── 命令树登记批（2026-10-09）：`zerg publish verify` 入口 ──────────────────
			// 真身 = `family_publish_verify_native.go` 的 `cmdPublishVerify` → `publishVerifyRun`
			// （两器产出一致性验收：`zerg publish verify` ⟷ `bash scripts/build/test-publish-parity.sh`
			// · **本地** · 不出网、不推送 · DEV-2136）。
			// ★ run 指向**本族承载入口** `cmdPublishVerifyFamily`（宿主 `ui_start.go` · 门78
			// `check-newcmd-followup` 面④ 要求「新增 path 的实现件已在 `scripts/公开标记.tsv`」；
			// 真身件未跟踪、不在该表内 ⇒ 照 `mcp serve` 形状经**已登记宿主**转发）。入口只转发、
			// ✗ 不改退码口径（真跑退码由 `publishVerifyRun` 定）。
			path:    []string{"publish", "verify"},
			kind:    "PublishVerify",
			summary: "两器产出一致性验收（**本地** · 不出网、不推送 · 真身 `family_publish_verify_native.go:publishVerifyRun`；入口只转发）",
			usage:   "zerg publish verify",
			arity:   "none",
			run:     cmdPublishVerifyFamily,
		},
		{
			// ── 收口编排件（本单 · main.go 开窗）：`zerg publish accept` ───────────────────
			// 真身 = `family_publish_accept_native.go` 的 `publishAcceptRun`：把 8 条收口验收
			// **逐条 rc AND 成单一结论**（任一红 ⇒ 不收口）。★ 判据单一真源 = 任务单 ㈤v2；
			// 门已判的**只调不重判**（验-1 调 `publishVerifyRun` · 验-6/7/8 按名进程内调门 ·
			// 验-5 取值 `gate ls --json`）；仅验-2/3/4 无既有门 ⇒ 编排件内薄适配器。
			// ★ 同形先例 = `publish preflight`（Go 侧声明式步表 + 进程内按名调门 + 三档出口）。
			// ★ 出口三档（只读主档）：全 rc=0 ⇒ 0（收口）/ 任一 rc=1 ⇒ 1（不收口）/ 缺件跑不动 ⇒ 2
			//   （fail-closed · ✗ 不把 2 折成 0）。★ `--out <件>` = D2 报告件（照 `gate matrix --out`
			//   三态门 + 原子落件；落**调用方给的路径** · 命令面不预设入仓）。
			path:     []string{"publish", "accept"},
			kind:     "PublishAccept",
			summary:  "8 条收口验收逐条 rc AND（任一红 ⇒ 不收口 · 缺件/跑不动 ⇒ 2 不给结论）；真身 `family_publish_accept_native.go`（判据单一真源 = 任务单 ㈤v2）",
			usage:    "zerg publish accept [--out <件>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			fields:   []string{"id", "title", "rc", "criteria"},
			endpoint: "",
			run:      cmdPublishAccept,
		},
		{
			// ── 片3（本单 · C12 接线）：`zerg publish mirror` —— 逐提交镜像器入命令面 ───────────
			// 真身 = `family_mirror_public_native.go:mirrorPublicNative`（旧壳 `publish/mirror-public.sh`
			// 的 Go 真身 · 编排五段 · 引擎 `publish/mirror-public-lib.py` 逐字节内嵌）。原状
			// 「真身在盘、命令树**无**入口」⇒ `zerg publish mirror` 一律 rc=2（B2/R1 病根）。
			// ★ **B 式宿主桥**（C12 §模板B · **零新表行**）：真身件**不在** `scripts/公开标记.tsv`
			//   （门78 `check-newcmd-followup` 面④ 要求「新增 path 的实现件已在标记表」）⇒ `run:`
			//   指向**已登记宿主** `ui_start.go`（该表 `:158` `public=yes`）的桥 `cmdPublishMirror`
			//   —— 与同族 `publish verify`（`run: cmdPublishVerifyFamily`）**同一条形状**。
			// ★ **命令面只缴 `--dry-run`** ⇒ 桥映射到真身 `--list-only`（真·零写盘/零联网/零推送的
			//   只读预演）；真跑（出**本地**镜像树）按 **D3** 三态（照 `publish run`：缺
			//   `--confirm=<主机名>` 或 `--yes` ⇒ 2 fail-closed）。
			// ★ **`--push` 不上命令面**（族纪律「推远端不在命令面」· C12 §5.3②）—— 桥里**拒**该旗标。
			// ★ `passthrough: true`：除上面两枚外旗标逐字交真身自己的 argv 解析面（与 `doc kb-sync`
			//   同一种「透传型」形态：真身自带解析，命令面不复刻第二套旗标表）。
			path:        []string{"publish", "mirror"},
			kind:        "PublishMirror",
			summary:     "逐提交镜像（**本地** · 出镜像树 · 不出网、不推送 · 真身 `family_mirror_public_native.go:mirrorPublicNative`）；命令面只缴 `--dry-run` ⇒ 真身 `--list-only` 只读预演（零写盘）· `--push` 不上命令面",
			usage:       "zerg publish mirror [--dry-run | --confirm=<主机名> --yes]",
			arity:       "any",
			args:        []string{"（无位置参数：`--dry-run`（= 真身 `--list-only`）为只读预演；其余旗标逐字透传真身）"},
			endpoint:    "",
			danger:      &dangerSpec{dangerD3, "主机名", "出**本地**逐提交镜像树（真跑按 D3 三态：缺 `--confirm=<主机名>`/`--yes` ⇒ 2）—— **不出网、不推送**（`--push` 不上命令面）", "C12 §5.3② · 同族先例 publish run（D3）· 族纪律「推远端不在命令面」", true, dangerScopeRepo},
			opened:      true,
			passthrough: true,
			run:         cmdPublishMirror,
		},
		{
			// ── 命令树登记（本单）：`zerg publish snapshot-self-test` —— 发布链 Go 半余件接入命令面 ──
			// 真身 = `family_publish_snapshot_native.go:publishSnapshotSelfTestNative`（旧 py
			// `scripts/build/publish-snapshot-self-test.py` 205 行的 Go 真身）：把 `publish/ci/*.yml`
			// 里的 `run:` 步骤在**公开快照目录**上原样跑一遍（判据真源就是那两件 yml · 本件不另写判据）。
			// 原状「真身在盘、命令树无入口」⇒ 一律 rc=2 —— 缺口 `GAP-20261012-16`（门 `check-cli-ownership`
			// 类 B「未接 inproc 标准路由」余 1 件的正解）。
			// ★ run 指向**已登记宿主** `ui_start.go` 的桥 `cmdPublishSnapshotSelfTestFamily`（门78
			// `check-newcmd-followup` 面④：真身件不在 `scripts/公开标记.tsv` ⇒ 经已登记宿主转发）。
			// ★ `passthrough: true`（同 `publish mirror`）：`--job`/`--json` 逐字交真身自己的 argv 解析面。
			// ★ 字段表 = 真身 `--json` 的五格顶层键，必须写成 `[]string{…}` **字面量**（契约脚本 `FIELDS_RE`
			// 只认字面量 · 同 `net probe`/`publish tree has` 那条纪律）。
			path:        []string{"publish", "snapshot-self-test"},
			kind:        "PublishSnapshotSelfTest",
			summary:     "公开快照自测（**本地** · 把 `publish/ci/*.yml` 的 `run:` 步骤在公开快照目录上原样跑一遍 · 判据真源 = 那两件 yml · 真身 `family_publish_snapshot_native.go`；入口只转发）",
			usage:       "zerg publish snapshot-self-test <快照目录> [--job <作业>] [--json <字段>]",
			arity:       "any",
			args:        []string{"快照目录（公开快照的根 —— 不是目录/不存在 ⇒ 2）"},
			fields:      []string{"pass", "fail", "skip", "snapshot_has_git", "wall_secs"},
			endpoint:    "",
			passthrough: true,
			run:         cmdPublishSnapshotSelfTestFamily,
		},
		{
			// 组4 §二.4 `W-51`（`研-禁:134` `R-21` 归绿核心）· 任务单序133 · 与组3 `Q-025` 同条：
			// 归绿判据逐字「指定 commit 的必需检查集合全部 `success`、且没有一个 `skipped`」，
			// 读数带 `head_sha` —— 「**人贴屏不算判据**」⇒ 本命令是那张图的**只读**机器面
			// （读本机盘上两件：check-run 记录件 + 必需集合声明件 · 无网络面、无凭据面）。
			// ★ 「必需集合」的来处是**声明件**（`source` 闭集 `github_settings`/`unverified`）；
			//   `unverified` ⇒ 读数照出，但**口径行**明写「与 GitHub 侧 settings 的对齐未核」
			//   （源件逐字「未核（要 GitHub 侧 settings 才看得到）」）—— 两件事不许并成一个词。
			path: []string{"ci", "green"},
			kind: "CiGreen",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` / `publish tree has`
			//   两条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 该条被读成
			//   「没有字段表」）。
			summary:  "公开面 CI「归绿」**只读**读数（必需集合逐名 `success` 且集合内 `skipped`=0 · 带 `head_sha` · 声明集合的「必需」那格未核 ⇒ 口径行里印 · 有一条判不出 ⇒ 8 · **不给结论**）",
			usage:    "zerg ci green --run <记录件> [--decl <声明件>] [--json <字段>]",
			fields:   []string{"head_sha", "caliber", "decl_source", "required", "skipped", "verdict"},
			endpoint: "",
			run:      cmdCiGreen,
		},
		// ── 「显式指定机器」族（`route` · 2026-09-24 · 单独定制 > 路由默认规则）──────────────
		//   现场（逐字）：子代理**没法把模型钉到指定机器** —— 网关择优默认挑本机（请求落在 Mr2109 的
		//   `llama-server`），而人要的是该机型；当时只有「手工 `POST /api/control/unload {"machine":"Mr2109"}`
		//   绕路」这一条路。设计稿：`Zerg-内部文档/项目文档/v2.5.12/设计-指定机器路由-v1.0-20260924.md`。
		//   覆盖表 = `<状态目录>/route_pins.json`（**不在任何仓里** · 三件套里最要紧的「不永久改变默认」
		//   就落在那张表的 TTL 上）；选机那道门（`core/internal/gateway/route_pin.go`）**只读**它。
		//   ★ 三条命令都**不新增顶层命令**语义面之外的东西：命令树 +3，旗标面只加 `--machine` 一枚。
		{
			path: []string{"route", "pin"},
			kind: "RoutePin",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与 `net probe` / `publish tree has`
			//   / `ci green` 三条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量）。
			summary: "把**某个模型**钉到**某台机器**上（带 TTL 的覆盖表 · 命中即用 · 撤回一行 `route unpin`）",
			usage:   "zerg route pin --model <模型> --machine <机器> --ttl <时长> [--by <谁>] [--note <…>] [--dry-run | --yes] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：模型走 `--model`、机器走 `--machine`）"},
			fields:  []string{"model", "machine", "expires_at", "remaining_s", "state"},
			danger: &dangerSpec{dangerD2, "模型",
				"把这一条写进覆盖表（该模型的选机**命中即用**该机器）—— 可逆：`route unpin` 或等它自己到期",
				"设计-指定机器路由-v1.0-20260924.md §4 · 单独定制 > 路由默认规则（个别要求一次性 + TTL）", false, dangerScopeOutRepo},
			// `opened`: 真跑已开放（`--yes` 就写；`--dry-run` 零副作用）。
			opened:   true,
			endpoint: "",
			run:      cmdRoutePin,
		},
		{
			path:    []string{"route", "ls"},
			kind:    "RouteLs",
			summary: "看覆盖表现在钉着什么（**只读**：不碰盘、不探主控、零网络 · 过期行也显出来）",
			usage:   "zerg route ls [--model <模型>] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：过滤走 `--model`）"},
			// 与上面 `route pin` 那一条**同一份五格**（同一个写法只有一个来源）。
			fields: []string{"model", "machine", "expires_at", "remaining_s", "state"},
			// `opened`: 只读面直接开放（不写、不探网络 ⇒ 无危险档，与 `route pin/unpin` 的 D2 成对）。
			opened:   true,
			endpoint: "",
			run:      cmdRouteLs,
		},
		{
			path: []string{"route", "unpin"},
			kind: "RouteUnpin",
			// ★ 字段表同样必须是字面量（同上）。
			summary: "撒掉一条钉（不给 `--model` ⇒ **撒全部** · 一行撤回 · 幂等：没撒到 ⇒ 0 + `changed=false`）",
			usage:   "zerg route unpin [--model <模型>] [--dry-run | --yes] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：点名走 `--model`）"},
			fields:  []string{"model", "machine", "expires_at", "remaining_s", "state"},
			danger: &dangerSpec{dangerD2, "模型",
				"把点名的行从覆盖表里去掉（该模型的选机**立刻回默认择优**）—— 可逆：再 `route pin` 一次",
				"设计-指定机器路由-v1.0-20260924.md §4 · 「一行撤回」那条约束的落点", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdRouteUnpin,
		},
		// 写面两枚：**同一个执行门**（三态：--dry-run 计划件 / 缺 --yes ⇒ 2 / 齐了才发）
		{
			path:    []string{"resource", "pin"},
			danger:  &dangerSpec{dangerD2, "资源 id", "钉住资源（改资源不可回收标记 · 控制面 `POST /api/resources/pin`）—— 可逆：`resource unpin` 即回原态", "GAP-20261005-38 · 同族先例 task cancel（D2）· §三 H 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "ResourcePin",
			summary: "钉住资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource pin <资源 id> --ttl <时长> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"resource", "unpin"},
			danger:  &dangerSpec{dangerD2, "资源 id", "解钉资源（去掉不可回收标记 · `POST /api/resources/unpin`）—— 可逆：再 `resource pin` 即回原态", "GAP-20261005-38 · 同族先例 task cancel（D2）· §三 H 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "ResourceUnpin",
			summary: "解钉资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource unpin <资源 id> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"gateway", "breakers"},
			danger:  &dangerSpec{dangerD2, "（网关开关）", "网关断路开关（`--reset` 重置熔断 ⇒ 重新放流量 · `POST /api/gateway/breakers/reset`）", "GAP-20261005-38 · 同族先例 core reload（D2）· §三 C 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "GatewayBreakers",
			summary: "网关断路开关（D2 写面 · `--reset` 要 `--yes`）",
			usage:   "zerg gateway breakers [--reset] [--dry-run | --yes]",
			run:     cmdHazardWrite,
		},
		// ---- 批 D · T-47 虫卵 / 虫茧两族（I / J 族 · §3.4 · §7.1 P6 · N4 硬占位表）----
		{
			path:     []string{"egg", "ls"},
			kind:     "Egg",
			summary:  "卵 × 设备矩阵（只读投影：host / model / state 三格 · 照现成端点包装）",
			usage:    "zerg egg ls [--json <字段>]",
			fields:   eggFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路）",
			run:      cmdEggLs,
			// ★ `GAP-20260925-15`（Face A · 2026-10-10）：看门狗读数两格（`last_output_ago_s` / `watchdog_verdict`）
			//   走**可选字段层**（`optFields`）—— 登在出口白名单里、`--json` 点得出来，但**不进** `eggFields`
			//   那一份**冻结**的帮助 / 文档导出面字段表 ⇒ 无需重生成命令页（与 `code find` 的 `count` 同一体例）。
			optFields: eggOptionalFields,
		},
		{
			path:     []string{"egg", "show"},
			kind:     "Egg",
			summary:  "单枚卵的现状（只读投影：host / model / state 三格 · 同一份真源）",
			usage:    "zerg egg show <卵 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（<模型 id>@<主机>，照 `egg ls` 的 egg_id 那一格）"},
			fields:   eggFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路）",
			run:      cmdEggShow,
			// ★ `GAP-20260925-15`（Face A · 2026-10-10）：同 `egg ls` —— 看门狗读数两格走**可选字段层**
			//   （`optFields`：登在出口白名单、`--json` 点得出来，但不进冻结帮助 / 文档导出面字段表）。
			optFields: eggOptionalFields,
		},
		// ★ 缺口 `GAP-20261018-13`（第二层：体检正门）：卵的**三面对账**一条命令看全 ——
		//   声明态（主控 `/api/models/{name}`）⟷ 进程态/端口态（子端 `/eggs`，含驻留账本的
		//   `residency[]`）⟷ GTT 占用 ＋ 上次装载时刻，并**逐项点名**不一致。
		// 退码 = 0 全一致 / 1 有不一致 / 8 判不了（件头 `family_egg_doctor.go` 的「退码约定」）；
		// 只用既有端点、只 GET ⇒ 非危险档（`danger == nil`）、无干跑档（本命令零写面）。
		{
			path:    []string{"egg", "doctor"},
			kind:    "EggDoctor",
			summary: "卵体检（只读三面对账：声明态 / 进程态 / 端口态 / GTT / 上次装载时刻 ＋ 不一致项逐条点名 · 退码 0 一致 / 1 不一致 / 8 判不了）",
			usage:   "zerg egg doctor [--machine <机>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器（`--machine`，可省：缺省体检全机群）"},
			fields:  eggDoctorFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name} ＋ 子端 GET /eggs" +
				"（现成端点包装 · 不新开一条路 · 只读）",
			run: cmdEggDoctor,
		},
		{
			path:     []string{"cocoon", "ls"},
			kind:     "Cocoon",
			summary:  "虫茧清单（主控面无投影端点 ⇒ 不给结论）",
			usage:    "zerg cocoon ls [--json <字段>]",
			fields:   []string{"name", "state", "port", "path"},
			endpoint: "GET /api/cocoons（主控面没有 ⇒ 现跑 404）",
			run:      cmdCocoonLs,
		},
		{
			path:    []string{"cocoon", "open"},
			summary: "起虫茧的文档服务（8610 · D3 起服务档 · **计划面已开放**：`--dry-run` 出计划件；真跑本版未开放 · 拒执退码 2）",
			usage:   "zerg cocoon open <茧名> [--confirm=<茧名> --yes | --dry-run]",
			arity:   "any",
			args:    []string{"茧名"},
			refuses: true, // 真跑拒执（`cmdCocoonOpen` 非 dry-run 一律 `not_opened` 退 2 —— 起常驻服务本版未开放）
			run:     cmdCocoonOpen,
		},
		// ---- A 档 批 1（换件面）· A12：卵的**单文件封装**（`*.egg`）工具 —— 从「虫卵」归名到「虫茧」（决-13）----
		// 依据：`~/.zerg/state/拍板落定-所有稿-20261007.md` 决-13（Mr2109「拍 按你推荐」）。
		// 底层唯一实现 = `core/cmd/zerg/zerg_egg.go`（`scripts/<机型>/zerg-egg.py` 的 Go 真搬体 · DEV-1798）——
		// 命令面**只做入口 ＋ 转发**（连退码也不改写；✗ 不重写 `zerg_egg.go` 里任何一段）。
		// ★ 族名口径：`egg` 已被【虫卵（沙箱/容器）投影面】占用（`egg ls/show/run/stop`）⇒ 本工具归「虫茧」名下（✗ 不叫 `egg pack`）。
		{
			path:        []string{"cocoon", "pack"},
			summary:     "把一棵源目录打成单文件卵包（`*.egg` · 底层 = `zerg_egg.go` 真搬体 · 命令面只转发不翻译）",
			usage:       "zerg cocoon pack <源目录> <输出.egg> [--align <n>] [--chunk-size <n>] [--engine-dir <目录>] [--libs-dir <目录>] [--self-test]",
			arity:       "any",
			args:        []string{"源目录", "输出 .egg 件（旗标逐字来自 scripts/x" + "3/zerg-egg.py 的真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "info"},
			summary:     "读一枚卵包的索引（magic/align/段数/逐段 sha256 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon info <卵包.egg>",
			arity:       "any",
			args:        []string{"卵包 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "verify"},
			summary:     "校验一枚卵包（头块/对齐/逐段 sha256 · 可选重建到目录 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon verify <卵包.egg> [--rebuild-into <目录>]",
			arity:       "any",
			args:        []string{"卵包 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "xfer"},
			summary:     "按块可续传地把一枚卵包拷到目标（中断可续 · 坏块自愈 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon xfer <源.egg> <目标.egg> [--chunk-size <n>]",
			arity:       "any",
			args:        []string{"源 .egg 件", "目标 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:     []string{"egg", "run"},
			danger:   &dangerSpec{dangerD2, "卵 id", "把该卵的模型**装到**那台机上（控制面 `POST /api/control/load` · 占内存/显存槽 · 单槽机串行）—— 可逆：`egg stop` 即回原态", "GAP-20261005-38 · 同族先例 model start / agent load（D2）", false, dangerScopeRemote},
			opened:   true,
			kind:     "EggRun",
			summary:  "把卵的模型**装到**那台机上（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/load`）",
			usage:    "zerg egg run <卵 id> [--machine <机>] [--timeout <时长>] [--wait | --no-wait] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/load（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggRun,
		},
		{
			path:     []string{"egg", "stop"},
			danger:   &dangerSpec{dangerD2, "卵 id", "把那台机上的该模型**卸掉**（控制面 `POST /api/control/unload` · **在跑任务受影响**）", "GAP-20261005-38 · 同族先例 model stop / agent unload", false, dangerScopeRemote},
			opened:   true,
			kind:     "EggStop",
			summary:  "卸载一枚卵 —— 把那台机上的该模型**卸掉**（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/unload`）",
			usage:    "zerg egg stop <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/unload（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggStop,
		},
		// ---- 卵开工 · 卵7①（2026-10-15）：卵族**具名入口** —— `zerg egg <info|verify|gc|expand|release>` ----
		// 落件 = `core/cmd/zerg/family_egg.go`（薄壳 · 逐字转发已落能力 —— ✗ 命令层不重写逻辑）：
		//   · `verify` → `zerg_egg_verify.go` 的 `zegvVerifyEntry`（逐段并行 sha256 ＋ 内容寻址校验缓存 ·
		//     卵3① 已落 —— 此前只有 passthrough 的 `cocoon verify` 打得到）；
		//   · `info` / `gc` / `expand` / `release` → `core/internal/store/eggstore`（名册 / 引用图 / GC /
		//     CoW 展开 / 解引用 · 卵2 已落 —— 该包件头自注逐字「本包纯库件、零 CLI 接线」⇒ 命令面此前打不着）。
		// ★ 卵库根 = `--root <目录>`（**已在旗标名字表上** ⇒ ✗ 不新开旗标；缺省 `~/.zerg/eggs`）。
		// ★ 危险档照同族先例 `core db`（`family_core_db.go`）：`gc` = D3（真删不可逆 ⇒ `--confirm=<卵库根>`
		//   与 `--yes` **同时到**）· `expand` / `release` = D2（`--yes`）· 三条各有 `--dry-run` 计划件（零副作用）。
		// ★ 族名口径（延续 main.go 里既有那条自注）：`egg` **既有**的 `ls`/`show`/`run`/`stop`/`pin`/`unpin`
		//   是【虫卵（沙箱 / 容器）投影面】（远端只读投影 ＋ 控制面写面），与本次这五条**不同对象** ——
		//   本批只**增**五条（✗ 不动既有任何一条的用法串与退码）。`ls` 那一格**已被占用**
		//   ⇒ 卵库名册清单落 `zerg egg info` 的无参档（名册态），✗ 不新造同路径第二条。
		{
			path:    []string{"egg", "info"},
			kind:    "EggInfo",
			summary: "卵库档案 / 名册态（只读：给卵名 ⇒ 名册行 ＋ 引用格；不给 ⇒ 全册清单 —— 转发 `eggstore` 的名册与引用图）",
			usage:   "zerg egg info [<卵名>] [--root <卵库根>] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名（可省 ⇒ 报卵库名册态）"},
			fields:  []string{"action", "root", "root_source", "egg", "sha256", "state", "path", "bytes", "refs", "complete", "blocks", "added_at", "updated_at", "count", "verdict"},
			run:     cmdEggInfo,
		},
		{
			path:    []string{"egg", "verify"},
			kind:    "EggVerify",
			summary: "校验一枚卵包（逐段并行 sha256 ＋ 内容寻址校验缓存 —— 转发 `zerg_egg_verify.go` 的 `zegvVerifyEntry` · ✗ 命令层不重写判据）",
			usage:   "zerg egg verify <卵包.egg> [--json <字段>]",
			arity:   "any",
			args:    []string{"卵包 .egg 件"},
			fields:  []string{"action", "path", "size_bytes", "segments", "data_bytes", "align", "cache_hit", "problem_count", "problems", "verdict"},
			run:     cmdEggVerify,
		},
		{
			path:    []string{"egg", "gc"},
			danger:  &dangerSpec{dangerD3, "卵库根", "真删卵件（**只在引用计数归零且态可回收时**）—— 删前 `--dry-run` 出计划件；件被删即不可逆（须重取）", "设计 §16.2 · §13.1 · §14.4 M1 · 任务单 卵2② / 卵7① · 同族先例 `core db`（D3 ＋ `--confirm`）", true, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggGc",
			summary: "卵库回收（引用计数归零才真删 · 报「已释放」只认 df；D3：`--dry-run` 零副作用 · 真跑要 `--confirm=<卵库根>` ＋ `--yes`）",
			usage:   "zerg egg gc [--root <卵库根>] [--dry-run | --confirm=<卵库根> --yes] [--json <字段>]",
			arity:   "any",
			fields:  []string{"action", "root", "root_source", "dry_run", "deleted", "deleted_count", "pinned", "pinned_count", "df_before", "df_after", "freed_bytes_df", "freed_source", "nominal_bytes_not_freed", "nominal_note", "note", "verdict"},
			run:     cmdEggGC,
		},
		{
			path:    []string{"egg", "expand"},
			danger:  &dangerSpec{dangerD2, "卵名", "展开一枚卵到落点（CoW 整文件 clone ＋ 登记同一条引用 ⇒ 卵的引用计数 ＋1）—— 可逆：`egg release` 删件并解引用即回原态", "设计 §13.1 · §16.2 · 任务单 卵2② · 同族先例 `core db backup`（D2 ＋ `--yes`）", false, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggExpand",
			summary: "展开一枚卵（先登记引用 → 再 CoW 整文件落件 · 展开物纳入**同一**引用计数；D2 写面：`--dry-run` 出计划件 · 真跑要 `--yes`）",
			usage:   "zerg egg expand <卵名> <引用者 id> <落点> [--kind expansion|clone] [--root <卵库根>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名", "引用者 id", "落点（展开物 / 副本的落盘路径）"},
			fields:  []string{"action", "root", "root_source", "egg", "ref", "kind", "dst", "tier", "cow", "bytes", "refs_after", "refs_now", "dur_ms", "fell_back", "dry_run", "dst_exists", "egg_known", "egg_state", "egg_complete", "egg_src", "note", "verdict"},
			run:     cmdEggExpand,
		},
		{
			path:    []string{"egg", "release"},
			danger:  &dangerSpec{dangerD2, "卵名", "删掉展开物 / 副本件并解掉那一条引用（`eggstore.ReleaseExpansion` · 先删件 → 再解引用）—— 可逆：`egg expand` 重新展开即回原态", "设计 §16.2 · §14.4 M1 · 任务单 卵2② · 同族先例 `core db prune`（D2 ＋ `--yes`）", false, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggRelease",
			summary: "删前解引用（删副本件 ＋ 解引用 · **只有归零后才允许 `egg gc` 真删**；D2 写面：`--dry-run` 出计划件 · 真跑要 `--yes`）",
			usage:   "zerg egg release <卵名> <引用者 id> [<副本路径>] [--root <卵库根>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名", "引用者 id", "副本路径（可省 ⇒ 只解引用、不删件）"},
			fields:  []string{"action", "root", "root_source", "egg", "ref", "path", "released", "refs_left", "refs_now", "zero", "df_before", "df_after", "freed_bytes_df", "freed_source", "nominal_note", "path_exists", "dry_run", "note", "verdict"},
			run:     cmdEggRelease,
		},
		// ---- F52（2026-10-15）：`egg sign` **具名入口** —— 把卵5 已落的签名能力接进命令树 ----
		// 账面出处：`GAP-20261015-27` 的 `want_family=egg` / `want_action=sign`（族名与动作名照账面**逐字**）。
		// 真修件 = `core/cmd/zerg/egg_sign.go`（卵5 · 提案 `DEV-2958` 已落提交面）；命令面壳 = 该件 `cmdEggSign`。
		// ★ 判据一字不落：argv 组装 / 前置探针 / 自检面全在 `egg_sign.go`（本笔**只接线** —— ✗ 不改那件任何判据语义）。
		// ★ 退码闭集（照卵5 口径 · ✗ 不新增裸数字）：0 真跑签成 / 1 真跑了但未成（判决只看退码）/
		//   2 不给结论（工具缺失 / 版本不符 / 子进程起不来 / 用法错 —— 本库无 cosign ⇒ 恒走这一档）。
		// ★ ✗ 不新增旗标（usage 只列既有的 `--self-test`（全局布尔 · 谁用谁读）与 `--json` ⇒ 门73 旗标名表零改动 ·
		//   `usageDeclaresFlag` 无新增落点）。位置参数 1–3 枚（照同族 `egg expand` / `egg release` 的多位置参数先例）。
		{
			path:    []string{"egg", "sign"},
			kind:    "EggSign",
			summary: "为卵签名（cosign keyed · **调用形态真跑**：工具面前置探针 → argv 组装 → 转发 cosign）· 工具缺失 / 版本不符 ⇒ 退 2 不给结论（本库未装 cosign ⇒ 恒这一档）· 退码 0 签成 / 1 未成 / 2 不给结论 · `--self-test` 跑卵5 四态＋负控（9 项）",
			usage:   "zerg egg sign <卵包.egg> [<子钥件> [<bundle 件>]] [--self-test] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵包 .egg 件（必给）", "子钥件（cosign 的 key 取值）", "bundle 落件（cosign 的 bundle 取值）"},
			fields:  []string{"egg", "key", "bundle", "argv", "rc", "verdict"},
			run:     cmdEggSign,
		},
		// ---- 批 D · T-49 内部任务引擎族 `itask`（§7.1 P8 · §5.1 九条端点）----
		{
			path:     []string{"itask", "ls"},
			kind:     "InternalTask",
			summary:  "内部任务清单（16 类 · 含冷却与最近执行 · 投影 /api/internal-tasks）",
			usage:    "zerg itask ls [--json <字段>]",
			fields:   []string{"id", "description", "cooldown", "default_hours", "auto_run", "last_run", "state"},
			endpoint: "GET /api/internal-tasks",
			run:      cmdItaskLs,
		},
		{
			path:     []string{"itask", "state"},
			kind:     "InternalTaskState",
			summary:  "引擎状态（投影 /api/internal-tasks/state）",
			usage:    "zerg itask state [--json <字段>]",
			fields:   []string{"note", "state"},
			endpoint: "GET /api/internal-tasks/state",
			run:      cmdItaskState,
		},
		{
			path:     []string{"itask", "mode"},
			kind:     "InternalTaskMode",
			summary:  "自动运行开关现值（只读面；写面 `itask mode set` 已开放 · 见下条三格）",
			usage:    "zerg itask mode [--json <字段>]",
			fields:   []string{"modes", "note", "state"},
			endpoint: "GET /api/internal-tasks/modes",
			run:      cmdItaskMode,
		},
		{
			path:     []string{"itask", "interval"},
			kind:     "InternalTaskInterval",
			summary:  "周期现值（只读面；写面 `itask interval set` 已开放 · 见下条三格）",
			usage:    "zerg itask interval [--json <字段>]",
			fields:   []string{"intervals", "note", "state"},
			endpoint: "GET /api/internal-tasks/intervals",
			run:      cmdItaskInterval,
		},
		{
			path:     []string{"itask", "interval", "set"},
			kind:     "InternalTaskIntervalSet",
			summary:  "设内部任务周期（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 给 `all` 则逐条设）",
			usage:    "zerg itask interval set <任务|all> --hours N [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id（照 `itask ls` 的 id 那一格，或 `all` 逐条设）"},
			fields:   []string{"id", "hours", "landed", "message"},
			danger:   &dangerSpec{dangerD2, "本机名", "把该内部任务的循环周期设成 --hours N 小时（**持久化**到 intervals.json —— 0 = 取消；主控 goroutine 每 60s 检查到点）—— 可逆：再设回旧周期即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/interval · 同族写面先例 `itask stop`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/interval",
			run:      cmdItaskIntervalSetGuarded,
		},
		{
			path:     []string{"itask", "mode", "set"},
			kind:     "InternalTaskModeSet",
			summary:  "设内部任务的自动运行开关（D2 写面 · `--on`/`--off` 二选一 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2）",
			usage:    "zerg itask mode set <任务 id> [--on | --off] [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id（照 `itask ls` 的 id 那一格）"},
			fields:   []string{"id", "auto_run", "landed", "message"},
			danger:   &dangerSpec{dangerD2, "本机名", "把该内部任务的自动运行开关设成 `--on`（自动运行）/`--off`（手动运行）（**持久化**到 internal_modes.json —— 重启恢复）—— 可逆：再设回原开关即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/mode · 缺口 GAP-20260927-205 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/mode",
			run:      cmdItaskModeSetGuarded,
		},
		{
			path:     []string{"itask", "start"},
			kind:     "InternalTaskStart",
			summary:  "起内部任务引擎（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）",
			usage:    "zerg itask start [--dry-run | --yes] [--json <字段>]",
			fields:   []string{"message", "state"},
			danger:   &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎开始按周期跑（会自动占机器与模型槽）", "§7.1 P8 · §5.1 POST /api/internal-tasks/start · 开工单 T-49 · 同族写面先例 `itask stop`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/start",
			run:      cmdItaskStartGuarded,
		},
		{
			path:     []string{"itask", "stop"},
			kind:     "InternalTaskStop",
			summary:  "停内部任务引擎（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 写**持久化**的用户意图 `stopped=true`）",
			usage:    "zerg itask stop [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			fields:   []string{"message", "state"},
			danger:   &dangerSpec{dangerD2, "本机名", "让内部任务引擎停下（把**持久化**的用户意图 `stopped` 置 true —— 周期/空闲触发都**不再起新的**内部任务；**在跑的这一轮不动**）—— 可逆：再起引擎即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/stop · 开工单 T-49 · 同族写面先例 `sched pause`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/stop",
			run:      cmdItaskStopGuarded,
		},
		{
			path:     []string{"itask", "run"},
			kind:     "InternalTaskRun",
			summary:  "手动跑一条内部任务（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 绕过它的冷却）",
			usage:    "zerg itask run <任务 id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"task_id", "type", "message"},
			danger:   &dangerSpec{dangerD2, "任务 id", "立刻跑一次该内部任务（绕过它的冷却 · 会占机器）", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/run · 开工单 T-49 · 同族写面先例 `itask stop`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/run",
			run:      cmdItaskRunGuarded,
		},
		// ---- 批 D · T-48 构建族（K 族 · §3.5 · §7.1 P11 · §17.2 ③ 建环）----
		{
			path:    []string{"build", "ls"},
			kind:    "BuildArtifact",
			summary: "制品现状（现读 bin/ 逐件 sha256 + 身份件）—— 清单真源仍是构建脚本",
			usage:   "zerg build ls [--json <字段>]",
			fields:  []string{"name", "sha256", "bytes", "mtime", "version", "code", "built"},
			// ★ 缺口 GAP-20260927-311（批⑤ · 2026-10-13）：`identity`（逐件**身份串**）走**可选字段层**——
			//   登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表 ⇒ 不重生命令页。
			optFields: []string{"identity"},
			endpoint:  "",
			run:       cmdBuildLs,
		},
		{
			path:     []string{"build", "show"},
			kind:     "BuildArtifactShow",
			summary:  "一件的**身份**（sha256/mtime/inode/type/arch/签名态 —— 换件后验「在跑的件 == 盘上件」要它）",
			usage:    "zerg build show <件> | --all [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（bin/ 下的名字，或一个路径）"},
			fields:   []string{"name", "sha256", "bytes", "mtime", "inode", "type", "arch", "signed"},
			endpoint: "",
			run:      cmdBuildShow,
		},
		// ---- 缺口 `GAP-20261003-12`：构建签名**身份体检**（只读面 · 三格）----
		// 病征：构建脚本的身份解析（`build-all.sh:173-205` resolve_sign_identity）没有 CLI 出口。
		// 治法：只读复现该解析（真源 = scripts/build/internal/sign/sign.go 的 Resolve），出三格
		//（解析身份 · 两态标记 · 最终 SIGN_MODE）⇒ 判据可从 CLI 打得着 · 零写、不重编、不碰 bin/。
		{
			path:     []string{"build", "doctor"},
			kind:     "BuildDoctor",
			summary:  "构建签名**身份体检**（只读 · 复现构建脚本 `resolve_sign_identity` ⇒ 三格：解析身份 · 两态标记 · 最终 SIGN_MODE）",
			usage:    "zerg build doctor [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数）"},
			fields:   []string{"identity", "marker", "sign_mode"},
			endpoint: "",
			run:      cmdBuildDoctor,
		},
		{
			path:    []string{"build", "all"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "重编制品（自举档 `--only cli` 只写 bin/zerg 一件；**换件档会覆盖 bin/ 里的在跑制品** · 底层 Go 真身 scripts/build/cmd/zerg-build-all/main.go）", "GAP-20261005-39 · family_build.go:279 自印「危险档：D3」· §17.2 ③ 建环", true, dangerScopeSystem},
			opened:  true,
			summary: "重编制品（**自举档已开放**：`--only cli` 只写 bin/zerg 一件；换件档/发布档仍未开放）",
			usage:   "zerg build all [--node <机>] [--only cli] [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		{
			path:    []string{"build", "release"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "打包发布件（清单档 `--manifest-only` 只写 dist/<版本>/release/；**换件档真跑未开放** · 底层 Go 真身 scripts/build/cmd/zerg-build-all/main.go）", "GAP-20261005-39 · family_build.go:279 自印「危险档：D3」· §17.2 ③ 建环", true, dangerScopeRepo},
			opened:  true,
			summary: "打包发布件（**计划面已开放**：`--dry-run` 出计划件；**清单档 `--manifest-only` 已开放** —— 只重写 `dist/<版本>/release/` 的清单与逐件 sha256，不编译、不重打包、不碰在跑件；**换件档真跑已开放（2026-10-10 · Mr2109 拍）** —— 内含全量构建、会覆盖 `bin/` 里的在跑制品）",
			usage:   "zerg build release [--node <机>] [--manifest-only] [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		// ---- 危险动作：**未开放的那批只登记形状，不开放执行**（§6.2 批 1 零写操作）；
		// ★ 例外（本刀）：`task terminate` 本版**已开放**（D3 写面 —— 确认档齐就真执行，见下条）。----
		// 未开放的那批每条都过 cmdGuarded：`--dry-run` 出计划件（退码 0）；真跑一律拒执（退码 2 = 不给结论）。
		{
			path:    []string{"task", "terminate"},
			kind:    "TaskTerminate",
			summary: "强杀在跑的任务（D3 写面 · `--reason` 闭集 5 值必填 · `--confirm=<任务 id>` 与 `--yes` **同时到**才执行 · **本版已开放**）",
			usage:   "zerg task terminate <任务 id> --reason <superseded|obsolete|duplicate|abandoned|other> [--note <一句>] --confirm=<任务 id> [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"任务 id"},
			fields:  []string{"id", "outcome", "terminated", "not_running", "already_terminal"},
			danger: &dangerSpec{dangerD3, "任务 id",
				"终止该任务的执行（CA 侧停 + 任务状态置 terminated）· 已产出的工作树不自动回收",
				"§三 D 族 · §4.1 K7 · §九 M3 C1/C4 · 开工单 T-44", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/tasks/{id}/terminate?reason=<闭集 5 值 必填>&note=<一句 可选>",
			run:      cmdTaskTerminate,
		},
		{
			path:    []string{"task", "rm"},
			summary: "删除任务（危险 D3 · 本版未开放）",
			usage:   "zerg task rm <任务 id> --confirm=<任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger: &dangerSpec{dangerD3, "任务 id",
				"删除该任务的记录与它指派的工作树/分支（**不可逆**）",
				"§三 D 族 · §4.1 K7 · 开工单 T-44", true, dangerScopeRemote},
			run: cmdGuarded,
		},
		{
			path:     []string{"task", "pause"},
			danger:   &dangerSpec{dangerD2, "任务 id", "把排队中的任务置为暂停态（可 resume 回来）· 内部 = 控制面 `POST /api/tasks/{id}/pause?pause=true`", "§三 D 族 · §6.3 S5 · 同族写面先例 `task move`（D2）", true, dangerScopeRemote},
			opened:   true,
			summary:  "暂停任务（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/tasks/{id}/pause?pause=true`）",
			usage:    "zerg task pause <任务 id> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"任务 id"},
			endpoint: "POST /api/tasks/{id}/pause?pause=true（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdTaskPauseGuarded,
		},
		{
			path:     []string{"task", "resume"},
			danger:   &dangerSpec{dangerD2, "任务 id", "把暂停的任务放回排队（可能立刻占机器）· 内部 = 控制面 `POST /api/tasks/{id}/pause?pause=false`", "§三 D 族 · §6.3 S5 · 同族写面先例 `task move`（D2）", true, dangerScopeRemote},
			opened:   true,
			summary:  "继续任务（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/tasks/{id}/pause?pause=false`）",
			usage:    "zerg task resume <任务 id> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"任务 id"},
			endpoint: "POST /api/tasks/{id}/pause?pause=false（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdTaskResumeGuarded,
		},
		{
			path:     []string{"task", "retry"},
			danger:   &dangerSpec{dangerD2, "任务 id", "把 failed 任务置回 queued（会再占一次机器与模型槽）· 内部 = 控制面 `POST /api/tasks/{id}/retry`", "§三 D 族 · §6.3 S5 · 同族写面先例 `task pause`（D2）", true, dangerScopeRemote},
			opened:   true,
			summary:  "重跑任务（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/tasks/{id}/retry`）",
			usage:    "zerg task retry <任务 id> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"任务 id"},
			endpoint: "POST /api/tasks/{id}/retry（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdTaskRetryGuarded,
		},
		// ---- 面 G · R7 · 新族 `sched`（暂停派活 / 并发旋钮 / 调度器状态）----
		// 缺口 GAP-20260927-210（P0）+ GAP-20260929-26（P1）。show 只读；pause/resume/set 改调度器
		// 状态（可逆面：再 resume/set 回去即可）。实现件已落：family_task.go（命令面）+
		// core/internal/api/master_scheduler.go（服务端 /api/sched* 四处理器）。
		// ★ 新顶层族 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半。
		{
			path:     []string{"sched", "show"},
			kind:     "SchedShow",
			summary:  "调度器状态（只读 · 并发 x/y · 队列 n · 在跑领取/心跳面）",
			usage:    "zerg sched show [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "model", "concurrency", "queued", "waiting_retry", "claimed_by", "claimed_at", "heartbeat_at"},
			endpoint: "GET /api/sched",
			run:      cmdSchedShow,
		},
		{
			path:     []string{"sched", "pause"},
			kind:     "SchedPause",
			summary:  "暂停派活（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · **在跑的不动** —— 要停在跑的走 task terminate）",
			usage:    "zerg sched pause [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "暂停派活（不再起新的任务；**在跑的不动** —— 要停在跑的走 `task terminate`）—— 可逆：`sched resume` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/pause",
			run:      cmdSchedPauseGuarded,
		},
		{
			path:     []string{"sched", "resume"},
			kind:     "SchedResume",
			summary:  "恢复派活（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 回到设定并发并立即派发积压）",
			usage:    "zerg sched resume [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "恢复派活（回到设定并发并立即派发积压）—— 可逆：`sched pause` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/resume",
			run:      cmdSchedResumeGuarded,
		},
		{
			path:     []string{"sched", "set"},
			kind:     "SchedSet",
			summary:  "设并发上限（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 非负整数 N 走位置参数）",
			usage:    "zerg sched set <N> [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"并发上限 N（非负整数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "把调度器并发上限设成 N（显式旋钮）—— 可逆：再 `sched set <旧 N>` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/set?max=<N>",
			run:      cmdSchedSetGuarded,
		},
		// ---- 批 B2（2026-10-20 · `zerg sched run`）：独立入口 `core/cmd/zerg-scheduler` 归入命令面 ----
		// 源件 104 行 · 旗标五枚**逐字**保留（`--workdir` / `--container` / `--agent-bin` / `--dev` /
		// `--repo`），只把 Go `flag` 的单横线写成双横线。★ 与上面四条的**分家**：`show|pause|resume|set`
		// = 打控制面 `/api/sched*`（管**在跑的那个**）；本格 `run` = **在本机起进程**（源件语义）。
		// **双轨期**：`core/cmd/zerg-scheduler` 二进制一字不动。实现在 `family_sched_run.go`
		// （件头逐条写清与源件的四处差异 —— ✗ 不是 `family_sched.go`：那个名字已被 sched 写面确认门占着）。
		// 写面（真派单：起 worker · 改任务单状态）但**不进危险档**，理由照源件口径写清：
		// 它是「操作者当场在本机起的那个进程」（本来由 launchd/systemd 拉起），Ctrl-C/SIGTERM 即停
		// ⇒ 本机可逆、无远端效应；与 `trace` 那格同一种「不进危险表」的判法。
		{
			path:     []string{"sched", "run"},
			kind:     "SchedRun",
			summary:  "在本机起一个调度器（**常驻档**：轮询任务单目录 → 按优先级派单 → 每 10s 报状态 → SIGINT/SIGTERM 优雅停；容器/开发两档照源件 `zerg-scheduler`）",
			usage:    "zerg sched run [--workdir <目录>] [--container] [--agent-bin <路径>] [--dev] [--repo <路径>]",
			arity:    "none",
			args:     []string{"（不收位置参数：工作目录走 `--workdir`）"},
			endpoint: "",
			opened:   true,
			run:      cmdSchedRun,
		},
		{
			path:    []string{"agent", "unload"},
			machine: machineNodeName,
			danger: &dangerSpec{dangerD3, "机器名",
				"卸掉该子端上的模型（**在跑的任务会被打断**）· 幂等：已在未装载态按「已在该状态」报 · 内部 = 控制面 `POST /api/control/unload`",
				"§三 B 族 · §九 M4 · 开工单 T-43 · 同族已开放先例 `egg stop`（同为控制面卸载）", true, dangerScopeRemote},
			opened:   true,
			summary:  "卸载子端上的模型（D3 写面：`--dry-run` 先行出计划件 · 真跑要 `--confirm=<机器名>` + `--yes` · 内部 = 控制面 `POST /api/control/unload`）",
			usage:    "zerg agent unload <机器名> <模型> --confirm=<机器名> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"机器名", "模型"},
			endpoint: "POST /api/control/unload（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdAgentUnloadGuarded,
		},
		{
			path:     []string{"agent", "load"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD2, "机器名", "把模型装进该子端（占内存/显存槽 · 单槽机是串行的）· 内部 = 控制面 `POST /api/control/load`", "§三 B 族 · §九 M5 · 开工单 T-43 · 同族已开放先例 `egg run`（同为控制面装载）", true, dangerScopeRemote},
			opened:   true,
			summary:  "加载模型到子端（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/load`）",
			usage:    "zerg agent load <机器名> <模型> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"机器名", "模型"},
			endpoint: "POST /api/control/load（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdAgentLoadGuarded,
		},
		{
			path:    []string{"agent", "reap"},
			machine: machineNodeName,
			summary: "回收残留干跑单（真回收危险 D3 · 本版未开放；**入库件永不进候选 = 红线**）",
			usage:   "zerg agent reap [--node <机>] --dry-run --min-age-days <n>   # 干跑单（带 plan_id + 逐件证据）\n       zerg agent reap [--node <机>] --confirm=<机器名> --yes [--dry-run]   # 真回收（本版未开放）",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"按「声明树 ↔ 现值树」差集回收闲置资源（**默认干跑**；入库件永不进候选 = 红线）",
				"§九 M9 · §十五.2 三档回收权 · RC11 红线 · 开工单 T-54", true, dangerScopeRemote},
			run: cmdAgentReap,
		},
		{
			path:     []string{"model", "stop"},
			kind:     "ModelStop",
			summary:  "停模型（D3 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<模型 id>`/`--yes` ⇒ 2 · 幂等优先：已停不报 500）",
			usage:    "zerg model stop <模型 id> [--dry-run | --confirm=<模型 id> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"name", "host", "status", "changed"},
			danger:   &dangerSpec{dangerD3, "模型 id", "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500", "§三 E 族 · §九 M4 · 开工单 T-45 · 同族写面先例 `agent unload`（D3）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/models/{name}/stop",
			run:      cmdModelStopGuarded,
		},
		{
			path:     []string{"model", "start"},
			kind:     "ModelStart",
			summary:  "起模型（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）",
			usage:    "zerg model start <模型 id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"name", "host", "status", "changed"},
			danger:   &dangerSpec{dangerD2, "模型 id", "把模型装载起来（占槽位 · 单槽机要排队）", "§三 E 族 · §九 M5 · 开工单 T-45 · 同族写面先例 `agent load`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/models/{name}/start",
			run:      cmdModelStartGuarded,
		},
		{
			path:    []string{"core", "stop"},
			machine: machineHostName,
			summary: "停主控（危险 D3 · 本版未开放）",
			usage:   "zerg core stop [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停掉主控进程（**整个虫群的控制面会断**）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:     []string{"core", "reload"},
			danger:   &dangerSpec{dangerD2, "（群级：不点名主机）", "让主控重读配置（规则表/名册 · `POST /api/config/reload`）—— 生效面即时；不重启、不停任何进程、已加载模型不受影响 · 名册件解析不过 ⇒ 一个请求都不发；可逆：再热加载一次即回旧档", "§三 C 族 · §6.3 S5 · 同族已开放先例 `config reload`（同端点 · D2）· §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			summary:  "重载主控配置（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/config/reload`；先校验名册件，解析不过不发请求）",
			usage:    "zerg core reload [--dry-run] --yes",
			endpoint: "POST /api/config/reload（路由已在跑的主控上 ⇒ 主控零改动）",
			run:      cmdCoreReloadGuarded,
		},
		{
			path:    []string{"core", "update"},
			machine: machineHostName,
			summary: "主控自身换件（危险 D3 · 本版未开放）",
			usage:   "zerg core update [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执）", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "pin"},
			summary: "钉住一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg pin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的）", "§3.4 I 族 · 开工单 T-47", true, dangerScopeRemote},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "unpin"},
			summary: "解钉一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg unpin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "取消在孵标记（原本占有单槽的卵会被换下）", "§3.4 I 族 · 开工单 T-47", true, dangerScopeRemote},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "release"},
			summary: "发布候选件（危险 D3 · 本版未开放）",
			usage:   "zerg dev release --candidate <候选 id> --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把候选件推上生产面（**只能由人拍板开**；AI 不许自升）", "§17.4 · §九 M18 C4 · 开工单 T-58", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "rollback"},
			summary: "回滚（危险 D3 · 本版未开放）",
			usage:   "zerg dev rollback [--to <目标>] --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把生产面退回某个已知状态（回滚件到期前**永不自动**）", "§17.2 ⑦ · §十五.2 档③ · 开工单 T-58", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"update"},
			machine: machineHostName,
			summary: "源码式自更新（危险 D3 · 本版未开放）",
			usage:   "zerg update [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单）", "§7.1 P12 · §九 M20 F-3 · 开工单 T-52", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "proposal"},
			kind:    "Proposal",
			summary: "提案件通道：只产可审查物（new|list|show|check|revise）· 目标必须回指既有编号 · **判据必须可机检** · `check --deep` 真跑每件判据（报 rc 与命中数）· 「提 ≠ 批」两对字段（subject/approver）· 作废档 `--retract/--void <提案 id>`（打作废标记 · 号不复用 · 落审计一行）",
			usage:   "zerg dev proposal new --title <题（必填）> --target <待办编号（必填 · 只收 D/E/F/G 族编号 · 逐条闭集真源 core/internal/contract/dev-targets.json）> --goal <目标（必填）> --evidence <出处（必填 · 至少一条）—— 两类：① **id 形状**（首部 `GAP-…` / `DEV-…`，如 `GAP-YYYYMMDD-NN`）须**在册**（缺口账 / 在册提案件里查得到）—— 查不到 ⇒ 退码 2「出处 … 在缺口账 / 在册件里查不到 ⇒ 不给结论」；② **描述性证据**（无前缀自由文本）直接收，不要求在册> --rollback <退点件（必填 · 仓外整件路径 —— 退建议照它核 sha256 取改前态；路径形的退点（绝对 / 带目录分隔符）在提案这一步就要求**现读可得**（读不到 / 是目录 ⇒ 当场退 2），非路径形的「方式」文本才留到退建议那一步判）> --criterion <判据（必填 · 一条首词是 zerg、本版真跑得动的命令）> [--criterion-new <前瞻判据（可选 · 一条首词是 zerg、本版**还跑不动**的命令形态 ⇒ 显式标前瞻；本版已能真跑的命令写 --criterion）>] [--by <提出者>] [--file <要改的件（仓内相对路径 —— 与 --rollback 收的**仓外**整件路径是两套口径 · 别混）>]… [--subject <提出者>] [--subject-kind human|ai|egg|ci] [--egg-id <卵 id>] [--approver <批准者>] [--approver-kind human] [--batch <件.json（可机读批量档 · 一次建多枚 · 逐格走单片档同一条闸 · 与单片旗标互斥）>] [--from-file <件.json（单片档 · 读一枚 JSON 对象 = 一片提案的字段 · 与单片旗标互斥）>] [--add-file <路径（仓内相对 · 可重复 · 补进既有提案的 files[]）>…（revise 补件） [--retract <提案 id> | --void <提案 id> --reason <理由>（作废档：打作废标记 · 号不复用 · 落审计一行）] [--json <字段>] [--dry-run | --yes] —— ★ 自由文本字段（题/目标/出处/判据/退点）含反引号或 `$` 时：请用**单引号**包裹、或改走 --from-file <件>；**双引号会被 shell 先展开**（丢字符/误执行）",
			arity:   "any",
			args:    []string{"动作：new | list | show | check | revise", "提案 id（show/check/revise 才要 · `--retract`/`--void` 也点名它）"},
			fields:  proposalFields,
			// 授权侧（`GAP-20260927-140` ①）：`new` 真写提案件（状态目录）而修前无 `danger`。
			// 声明面照 `agent registry` 先例：**整条命令**登记危险档，`list`/`show`/`check` 只读
			// （fail-closed 只落在 `new` 的写盘点上 —— 见 `cmdDevProposalNew` 的 `--yes` 门）。
			danger:   &dangerSpec{dangerD2, "（提案 id）", "落一件提案件（状态目录）—— `new` 写、`list`/`show`/`check` 只读", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 agent registry（声明面 · `--list` 只读）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevProposal,
		},
		// ---- D3b 第四步（2026-09-21）：**人批通道**（`zerg approve` 一族 · §九 M18 `C4`②）----
		// 人签批准件 = `require_approval` 的逃生门；模型写得出件的字节，**写不出那枚签名**（口令只在人手里）。
		{
			path:      []string{"approve", "ls"},
			kind:      "Approval",
			summary:   "列人签批准件（逐件带上**验签判决**：验过 / 无签名 / 签名坏 —— 后两者不算批准）",
			usage:     "zerg approve ls [--json <字段>]",
			fields:    approveFields,
			optFields: approveOptionalFields,
			endpoint:  "",
			run:       cmdApprove,
		},
		{
			path:      []string{"approve", "show"},
			kind:      "ApprovalShow",
			summary:   "看一枚批准件的全貌 + 验签判决（消费者只认「验过」那一档）",
			usage:     "zerg approve show <工具名> [--json <字段>]",
			arity:     "any",
			args:      []string{"工具名"},
			fields:    approveFields,
			optFields: approveOptionalFields,
			endpoint:  "",
			run:       cmdApprove,
		},
		{
			path:    []string{"approve", "new"},
			kind:    "ApprovalSign",
			summary: "**人签**一枚批准件（要人在终端上敲口令；非交互会话一律拒）· 写不进即拒 · 同名不覆盖",
			usage:   "zerg approve new --tool <工具名> --by <人名> --note <理由> [--scope <范围>] [--self-test] [--json <字段>] [--dry-run | --confirm=<工具名> --yes]",
			arity:   "any",
			args:    []string{"工具名（--tool）", "人名（--by）"},
			fields:  approveNewFields,
			danger:  &dangerSpec{dangerD3, "工具名", "签一枚批准件（逃生门）—— 只作 require_approval 的放行凭据；人不在场时等于没签", "§九 M18 C4② · §17.6 SD7 · D3b 第四步", true, dangerScopeOutRepo},
			// `opened`: 真跑已开放（**人在终端上** + 操作员口令 ⇒ 签出批准件；非交互一律拒）。
			opened:   true,
			endpoint: "",
			run:      cmdApproveNew,
		},
		{
			path:     []string{"approve", "keygen"},
			danger:   &dangerSpec{dangerD3, "旧 key_id", "生成操作员密钥对：私钥（口令加密）＋ 公钥落盘；**换钥路**归档并 os.Remove 旧钥（旧件全废 · 不可逆）", "GAP-20261005-36 · 同族先例 approve new（D3）· §九 M18 C4②", false, dangerScopeOutRepo},
			opened:   true,
			kind:     "ApprovalKeygen",
			summary:  "生成**操作员密钥**（人在终端上设口令；私钥口令加密落盘，公钥给消费者验签）",
			usage:    "zerg approve keygen --by <人名> --yes [--dry-run] [--confirm=<旧 key_id>]",
			arity:    "any",
			args:     []string{"人名（--by）"},
			endpoint: "",
			run:      cmdApproveKeygen,
		},
		// ---- GAP-20261001-76：**撤销/覆盖正门**（`zerg approve rotate` · 同族先例 approve new（D3））----
		// 病征：批准件没有撤销/覆盖/轮换正门，`new` 同名不覆盖 ⇒ 换 scope 只能手改家目录件。
		// 治法：一条正门 —— 给 `--scope` = 覆盖/轮换（旧件退位 mv + 人签新件）；不给 = 撤销（只退位）。
		{
			path:    []string{"approve", "rotate"},
			kind:    "ApprovalRotate",
			summary: "**撤销/覆盖**一枚批准件（给 `--scope` = 换范围（旧件**退位**（mv 不删）+ 人签新件）；不给 = 撤销（只退位））· 要人在终端上敲 · D3",
			usage:   "zerg approve rotate <工具名> --by <人名> [--scope <新范围>] [--note <理由>] [--dry-run | --confirm=<工具名> --yes]",
			arity:   "any",
			args:    []string{"工具名（位置参数）", "人名（--by）"},
			danger:  &dangerSpec{dangerD3, "工具名", "撤销/覆盖一枚批准件（逃生门）—— 旧件**退位**（mv 进 retired/ · ✗ 不删）+ 覆盖档人签新件；不可逆，人不在场等于没动", "GAP-20261001-76 · 同族先例 approve new（D3）· §九 M18 C4②", true, dangerScopeOutRepo},
			// `opened`: 真跑已开放（**人在终端上** + 操作员口令 ⇒ 覆盖签新件；非交互一律拒）。
			opened:   true,
			endpoint: "",
			run:      cmdApproveRotate,
		},
		// ---- D3b 第二步（2026-09-21）：**受控写面**（`zerg dev edit` · §17.3 铁律③）----
		// 默认干跑 · 只改提案声明过的件（越界写 ⇒ 2）· 逐条审计（写不进审计就不改件）· D3 档确认。
		{
			path:    []string{"dev", "edit"},
			kind:    "DevEdit",
			summary: "受控写入：只改**提案声明过**的件（越界写 ⇒ 2）· 默认干跑 · 一行一事件的审计（写不进审计就不改件）",
			usage:   "zerg dev edit --proposal <提案 id> --file <仓内相对路径> (--from <件> | --replace <件：补丁 JSON 数组 [{\"old\":…,\"new\":…}] —— 逐格精确替换 · 每格 old 必须在目标件里**恰好命中 1 次**（0 次或多次 ⇒ 拒执 2）> | --replace-inline <同一形态的内联 JSON 串 —— **直给，不必先落件**（缺口 GAP-20261018-22） | --replace-inline-b64 <同一 JSON 的 base64 形态 —— 绕开对话层字符扫描，缺口 GAP-20261018-24>) [--base <件路径|HEAD|sha>] [--by <谁>] [--allow-cross-root <理由>] [--allow-sibling-repo <名>] [--literal-text] [--json <字段>] [--dry-run | --confirm=<本机名> --yes] （真写前置：一枚**人签批准件** <状态目录>/approvals/dev_edit.json —— 由 `zerg approve new --tool dev_edit` 签出 · `zerg approve ls` 可查现状；无件 / 手写件 / 它的 scope 不含本件 ⇒ 一律拒（退 2）；--dry-run 那一态不需要它；`--base` 只与 `--replace` 合用：用**基线内容**当匹配基（`HEAD`=该件的已提交版 · `sha`=某提交版 · 其它=当作仓外前态件路径读），并把基线作为审计的 before ⇒ 已改盘的件可**回放**；回放结果必须与现盘件逐字节相同、否则拒执 2）" + devEditDryRunOnlyNote,
			arity:   "any",
			args:    []string{"提案 id（--proposal）", "要改的件（--file · 必须在提案的 files[] 里）"},
			fields:  devEditFields,
			danger:  &dangerSpec{dangerD3, "本机名", "改仓内件（写工作树）—— 作用域 = 提案声明的件；审计一行一事件；回滚 = 提案退点件（回到 before_sha256；`git revert` 不算退点）", "§17.3 铁律③ · §九 M3 C4/C5 · §4.1 K7 · D3b 第二步", true, dangerScopeRepo},
			// `opened`: 真跑已开放（`--confirm=<本机名> --yes` 齐 + 人签批准件 ⇒ 写工作树）。
			opened:   true,
			endpoint: "",
			run:      cmdDevEdit,
		},
		// ---- GAP-20261010-30：**直写补正门**（`zerg dev reclaim`）----
		// 口径：真前态 = 该件审计末行 `after` → 回盘造前态（`--pre` / `git HEAD`；无审计行 ⇒ 空前态）
		// → 调**既有** `dev edit --from` 净零重落 · 核 sha 逐字同 ＋ 断言审计新增一行。✗ 不放宽任何闸。
		{
			path:     []string{"dev", "reclaim"},
			kind:     "DevReclaim",
			summary:  "**直写补正门**：给被非正门直写的件补审计链（真前态 = 审计末行 after 回盘 → 调既有 `dev edit --from` 净零重落 · 核 sha 逐字同 · 盘上内容逐字节不变）",
			usage:    "zerg dev reclaim <仓内相对件>… --proposal <提案 id> [--pre <件>=<仓外前态件>]… [--by <谁>] [--json <字段>] [--dry-run | --confirm=<本机名> --yes]",
			arity:    "any",
			args:     []string{"要补正的件（仓内相对路径 · 可多给）"},
			fields:   reclaimFields,
			danger:   &dangerSpec{dangerD3, "本机名", "补正审计链：回盘真前态后调 `dev edit --from` 净零重落（盘上内容逐字节不变 · 只补审计链）；真前态取不到 ⇒ 拒", "GAP-20261010-30 · 同族先例 dev edit（D3）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevReclaim,
		},
		// ---- R3 · 缺口 `GAP-20260927-377`（P0）· `dev check --syntax`（类型面只读正门）----
		// 实现件已落（`family_dev_check.go`：`cmdDevCheck` + `devCheckFields`）⇒ 本处只做命令树登记。
		// 只读（零制品 · 制品丢 /dev/null · 不进危险档清单）；无 `--dry-run` ⇒ 干跑语义那一面不适用。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半 ＋ 旗标名表。
		{
			path:    []string{"dev", "check"},
			kind:    "DevCheck",
			summary: "类型面只读正门：真跑一次编译/go vet（制品丢 /dev/null · 零制品）—— 施工卵禁跑 go build|vet 时的编译器背书（GAP-20260927-377）",
			usage:   "zerg dev check --syntax [--file <仓内相对 .go 件>]… | [--pkg <包>]… [--vet] [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数 —— 目标一律走 `--file` / `--pkg`）"},
			fields:  devCheckFields,
			run:     cmdDevCheck,
		},
		// ---- §17.4 第 2/3/5 条：候选区（批 E · T-58 · 真源 core/internal/contract/dev-candidate.json）----
		// 次序（§17.7 逐字「先有判据、再有自动化」）落成机检：build / test 找不到该候选的验收证据单 ⇒ 退码 2。
		{
			path:    []string{"dev", "build"},
			summary: "在**候选区**构建全套件（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev build --candidate <候选 id> [--scope go|rust|ui|all] [--dist] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区编出成套制品（底层就是 Go 真身 scripts/build/cmd/zerg-build-all/main.go —— 不新造第二条构建路）", "§17.4 第 2 条 · §17.6 SD10-b · 开工单 T-58", true, dangerScopeRepo},
			run:     cmdDevBuild,
		},
		{
			path:    []string{"dev", "test"},
			summary: "跑候选件的测试集（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev test [--candidate <候选 id> | --pkg <包> [--run <正则>]] [--scope go|rust|ui|all] [--outdir D] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区跑测试集（底层 = make test + 门禁既有步，不新立判据）", "§17.4 第 3 条 · §17.7 次序 · 开工单 T-58", true, dangerScopeRepo},
			run:     cmdDevTest,
		},
		{
			path:    []string{"dev", "verify"},
			kind:    "DevEvidence",
			summary: "合成一份验收证据单（只收证据、**不给「通过」的结论**）· 证据为空 ⇒ 2",
			usage:   "zerg dev verify --candidate <候选 id> [--results <结果表>] [--code-sha <sha>] [--node <名>] [--layer <档>] [--gate [--human-approval <名>]] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"候选 id"},
			fields:  devVerifyFields,
			// 授权侧（`GAP-20260927-140` ①）：真写 `dist/candidates/<id>/evidence.json`（判据物 ·
			// **仓内工作树**）而修前无 `danger` ⇒ 默认档零授权写。fail-closed 落在写盘点（见 `cmdDevVerify`）。
			danger:   &dangerSpec{dangerD2, "候选 id", "写 `dist/candidates/<id>/evidence.json`（判据物 · 仓内候选区）", "GAP-20260927-140 ① · GAP-20260927-90 · dev verify 是零授权写里唯一落仓内工作树者", false, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevVerify,
		},
		{
			path:     []string{"propose", "ls"},
			kind:     "Proposal",
			summary:  "提案清单（§3.2 propose 那条 · 与 dev proposal list 同一份清单的第二个入口）",
			usage:    "zerg propose ls [--state 未决|已批准|已否决] [--json <字段>]",
			fields:   proposalFields,
			endpoint: "",
			run:      cmdProposeLs,
		},
		{
			path:     []string{"plugin", "ls"},
			kind:     "Plugin",
			summary:  "插件清单（`zerg-<名>` 约定 · 零注册表 · 影子告警 + 信任声明）",
			usage:    "zerg plugin ls [--json <字段>]",
			fields:   pluginFields,
			endpoint: "",
			run:      cmdPluginLs,
		},
		// ---- `zerg chat` 族（**人类正门** · `P-1` 乙案 · 设计稿 v0.3 §5）----
		// 首版 **7 条**子命令（`F-1` 拍定 = `new`/`ls`/`send`/`show`/`abort`/`steer`/`rm`）。
		// 形态 = **薄壳**（打请求 + 渲染）：语义全在服务端（`core/internal/api/chat_handlers.go` ·
		// `core/internal/chat/**`），壳里**一个字都不重写**（§0 第一原则 · §9.1 第 3 条）；
		// 实现件 = `core/cmd/zerg/family_chat.go`（本族的 HTTP 出口 / 事件读法 / 错误分档全在它里）。
		// ★ ✗ **不登记**（`O-5`/`O-6` 已给理由）：`send-tool` / `regenerate` / `message_edit` / `search`
		//   —— 真敲它们是「未知命令」（退码 2），✗ 不是「有登记无实现」。
		// ★ 裸 `zerg chat`（交互 REPL 入口 · §5.5 ①）—— **2026-10-17 Mr2109 拍定：登记裸条 · 按 TTY 分流**
		//   （本条**订正**同批早先那句「裸条归 T9 ⇒ 不登记 ⇒ 否则 +8 ≠ 255」）：
		//   **有 TTY ⇒ 进 REPL**（骨架归 T9）· **无 TTY ⇒ 退 2 ＋ 提示走 stderr** · **`--help` ⇒ rc=0**；
		//   先例 = 现读全树唯一的 **`help`**（裸条 ＋ 同族动作**并存**）。⇒ **命令树 path 数 248 → 256**：
		//   **+8** 条 = 7 条子命令 ＋ 裸条 `chat`。★ 早先写的 **255**（= 只算 7 条）是 T9 勘定给的旧值、
		//   **已被本拍定作废** —— 随动面（门⑪ `commands` / 门78 `count` / cli-matrix / 命令页）一律按 **256** 刷。
		{
			path:    []string{"chat"},
			kind:    "Chat",
			summary: "对话正门：**裸跑按 TTY 分流**（有 TTY ⇒ 进交互 REPL · 无 TTY ⇒ 退 2 ＋ 提示）· 7 条子命令见下行（§5.5 ①）",
			usage:   "zerg chat [--session <会话id> | --new] [<子命令>]",
			arity:   "any",
			args:    []string{"子命令（裸跑 = 交互 REPL · 无 TTY ⇒ 退 2）"},
			run:     cmdChatRoot,
		},
		{
			path:     []string{"chat", "new"},
			kind:     "ChatNew",
			summary:  "新建一个对话会话（人类正门的起手式 · `POST /api/chat/sessions` · §5.1 第 1 行）",
			usage:    "zerg chat new [--model 名] [--title 题] [--source 名] [--json <字段>]",
			arity:    "none",
			fields:   chatNewFields,
			endpoint: "POST /api/chat/sessions",
			run:      cmdChatNew,
		},
		{
			path:     []string{"chat", "ls"},
			kind:     "ChatLs",
			summary:  "列会话（`GET /api/chat/sessions`；归档默认隐藏 · `--archived` 含 · §5.1 第 2 行）",
			usage:    "zerg chat ls [--archived] [--source 名] [--limit N] [--json <字段>]",
			arity:    "none",
			fields:   chatLsFields,
			endpoint: "GET /api/chat/sessions",
			run:      cmdChatLs,
		},
		{
			path:     []string{"chat", "send"},
			kind:     "ChatSend",
			summary:  "对话主写面（SSE 流式 · `POST /api/chat/sessions/{id}/send`；`--json`/非 TTY/`--no-stream` ⇒ 非流式 · §7.5）",
			usage:    "zerg chat send [--prompt-file <路径> | <提示> | -] [--session <id>] [--new] [--last] [--last-in-cwd] [--model 名] [--images <路径>…] [--no-stream] [--markdown] [--wait] [--json <字段>] [--timeout <时长>]",
			arity:    "any",
			args:     []string{"提示（`-` = 从 stdin 读 · 与 `--prompt-file` 三选一 · 零 shell 解释 · §5.5 ③）"},
			fields:   chatSendFields,
			endpoint: "POST /api/chat/sessions/{id}/send",
			run:      cmdChatSend,
		},
		{
			path:     []string{"chat", "show"},
			kind:     "ChatShow",
			summary:  "读回会话与消息（`GET /{id}` · `--window <mid>` 走 `GET /{id}/window` 召回窗口 · §5.1 第 4 行）",
			usage:    "zerg chat show [<id>] [--session <id>] [--last] [--last-in-cwd] [--limit N] [--window <mid>] [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id · 可省 —— 与 `--session`/`--last` 三选一）"},
			fields:   chatShowFields,
			endpoint: "GET /api/chat/sessions/{id}",
			run:      cmdChatShow,
		},
		{
			path:     []string{"chat", "abort"},
			kind:     "ChatAbort",
			summary:  "中断运行中的轮次（`POST /{id}/abort`；已流出内容由服务端落库附「已中断」· §7.4）",
			usage:    "zerg chat abort <id> [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）"},
			fields:   chatAbortFields,
			endpoint: "POST /api/chat/sessions/{id}/abort",
			run:      cmdChatAbort,
		},
		{
			path:     []string{"chat", "steer"},
			kind:     "ChatSteer",
			summary:  "生成中插话（`POST /{id}/steer` · 挂**下一次工具结果**；`turn_running=false` ⇒ 服务端不入库 · `O-4`）",
			usage:    "zerg chat steer <id> <文字> [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）", "插话文字"},
			fields:   chatSteerFields,
			endpoint: "POST /api/chat/sessions/{id}/steer",
			run:      cmdChatSteer,
		},
		{
			path:     []string{"chat", "rm"},
			kind:     "ChatRm",
			summary:  "删会话（**写面 fail-closed**：`--dry-run` 出计划件 · 缺 `--yes` ⇒ 2 · `DELETE /{id}` · §5.1 第 7 行）",
			usage:    "zerg chat rm <id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）"},
			fields:   chatRmFields,
			danger:   &dangerSpec{dangerD2, "会话 id", "**硬删**一个会话与其消息（服务端 `DeleteSession` ⇒ `deleted:true`）—— **不可逆**：删掉的消息取不回来", "设计稿 §5.1 第 7 行 · §5.2（写面 fail-closed）· 同族先例 `gateway breakers`（D2）", false, dangerScopeRemote},
			opened:   true,
			endpoint: "DELETE /api/chat/sessions/{id}",
			run:      cmdChatRm,
		},
		// ---- `zerg chat` 机器正门 ＋ 会话别名（2026-10-17 本枚 · 命令树 264 → 268 · **+4** 条 path）----
		// ★ 为什么加这四条（用户原话「怎么更方便长期对话和多任务对话」）：
		//   ① **长期对话的真痛点 = 记不住 `chat_xxx` 那串 id** ⇒ 会话别名（`alias set|ls|rm`），
		//      ✗ 不是再加一层 `--session`；
		//   ② **多任务的真痛点 = 一个任务一条线、互不串** ⇒ 同一套别名天然解决（`ask --as <名>`）；
		//   ③ **机器调用的真痛点 = 一次进一次出、正文即结果** ⇒ `chat ask`（✗ 不靠流式 ✗ 不靠二次 `show`）。
		// ★ 随动面已同批刷（门⑪ 计数 / 门69 冻结常量 / cli-matrix / newcmd-paths / error-detail / 命令页四件）。
		{
			path:     []string{"chat", "ask"},
			kind:     "ChatAsk",
			summary:  "机器正门：**一次进一次出**（`--as <名>` 有别名就续上、没有就新建**并登记**；默认只把成稿正文打上 stdout · ✗ 不靠流式 · ✗ 不靠二次 `show`）",
			usage:    "zerg chat ask --as <名> [--prompt-file <路径> | <题> | -] [--stream] [--json <字段>] [--timeout <时长>]",
			arity:    "any",
			args:     []string{"提示（与 `--prompt-file` 三选一 · `-` = 从 stdin 读）"},
			fields:   chatAskFields,
			endpoint: "POST /api/chat/sessions（无别名时新建）· POST /api/chat/sessions/{id}/send",
			run:      cmdChatAsk,
		},
		{
			path:    []string{"chat", "alias", "set"},
			kind:    "ChatAliasSet",
			summary: "登记会话别名（名字 → 完整会话 id · 落 `<状态目录>/chat-aliases.json` · 写走**独占文件锁**）",
			usage:   "zerg chat alias set <名> --session <会话id> [--json <字段>]",
			arity:   "any",
			args:    []string{"别名（非空 · ≤64 字符 · ✗ 不含空白/控制字符）"},
			fields:  chatAliasSetFields,
			run:     cmdChatAliasSet,
		},
		{
			path:    []string{"chat", "alias", "ls"},
			kind:    "ChatAliasLs",
			summary: "列会话别名（默认表格式 · `--json` 挑字段 · 读走共享锁快照 · 按名字排序）",
			usage:   "zerg chat alias ls [--json <字段>]",
			arity:   "none",
			fields:  chatAliasLsFields,
			run:     cmdChatAliasLs,
		},
		{
			path:    []string{"chat", "alias", "rm"},
			kind:    "ChatAliasRm",
			summary: "删一条会话别名（名字不在表里 ⇒ **2** · 会话本身不动 · 写走独占文件锁）",
			usage:   "zerg chat alias rm <名> [--json <字段>]",
			arity:   "any",
			args:    []string{"别名"},
			fields:  chatAliasRmFields,
			run:     cmdChatAliasRm,
		},
		// ---- §十八.3 融合四件（批 E · T-59）----
		// 依赖次序（**不许倒**）：能力路由（T-41 的 id 规范化真源）→ `ask` → 意图 schema → `plan`/`apply`。
		{
			path:     []string{"ask"},
			kind:     "Ask",
			summary:  "问一次推理、**不落任务队列**（`task submit` 的对偶）· 能力筛是硬筛",
			usage:    "zerg ask <提示> [--capability 名]… [--prefer 名]… [--model 名] [--node 名]… [--min-ctx n] [--min-mem-gb n] [--dry-run] [--timeout 时长] [--json <字段>]",
			arity:    "any",
			args:     []string{"提示（一句话）"},
			fields:   askFields,
			endpoint: "GET /api/fleet/models · GET /api/models/registry",
			run:      cmdAsk,
		},
		{
			path:    []string{"plan"},
			kind:    "Plan",
			summary: "**算**：产出一份意图件（M6 包封 · F1–F7 + 四附加件）· 写意图件要 `--yes`（默认档 fail-closed）",
			usage:   "zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--target-ref <编号>] [--out <件>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"族（= target.kind）", "动作", "对象名"},
			fields:  planFields,
			// 授权侧（`GAP-20260927-140` ①）：真写意图件（`--out <件>` 或 `$ZERG_PLAN_DIR`）而修前无
			// `danger` ⇒ 默认档零授权写。fail-closed 落在写盘点（见 `cmdPlan`）。
			danger:   &dangerSpec{dangerD2, "（意图件落点）", "写一份意图件（`--out <件>` 或 `$ZERG_PLAN_DIR` 缺省 ~/.zerg/plans/<plan_id>.json）", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 doc meta fill（D2）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdPlan,
		},
		{
			path:     []string{"apply"},
			kind:     "Apply",
			summary:  "**做**：只吃那一份意图件（L1 schema → L2 引用 → L3 干跑 → L4 人在环）· **校验四层已开放**；写面（真做）本版未开放 · 拒执退码 2",
			usage:    "zerg apply <件> [--confirm=<目标>] [--json <字段>]",
			arity:    "any",
			args:     []string{"意图件路径"},
			fields:   applyFields,
			endpoint: "",
			refuses:  true, // 真跑拒执（`cmdApply` 校验四层已开放、**写面（真做）本版未开放** ⇒ 真做退 2）
			run:      cmdApply,
		},
		// ---- §20.3 H3 交接回执（批 E · T-61）----
		// 轮级续做件（`P-119` 的取舍：变更账是版本级史书、回执是轮级续做件，不许混）。
		{
			path:    []string{"dev", "receipt"},
			kind:    "Receipt",
			summary: "交接回执（一轮一页 · 带 trace_id）：new | ls | show；读法 = `zerg context ls --resume`（`new` 写要 `--yes`）",
			usage:   "zerg dev receipt new --what <做了什么> --next <下一步> [--evidence <证据>]… [--blockers <阻碍>]… [--commit <sha>]… [--trace <trace_id>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"动作：new | ls | show", "轮次 id（只 show 要）"},
			fields:  receiptFields,
			// 授权侧（`GAP-20260927-140` ①）：`new` 真写回执件（状态目录 `receipts/`）而修前无 `danger`。
			// 声明面照 `agent registry` 先例：**整条命令**登记危险档，`ls`/`show` 只读
			// （fail-closed 只落在 `new` 的写盘点上 —— 见 `cmdReceiptNew` 的 `--yes` 门）。
			danger:   &dangerSpec{dangerD2, "（轮次 id）", "落一件回执（状态目录 `receipts/<R-…>.json`）—— `new` 写、`ls`/`show` 只读", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 agent registry（声明面）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevReceipt,
		},
		// ---- 只读隔离（`GAP-20260926-20` P0 · 2026-09-26）：脚本面的**CLI 正门** · **顶层命令** ----
		// 底层原件 = `scripts/dev/isolate-ro.sh`（只转发、不翻译；退码 0/1/2 原样转出）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 落**顶层**（父代理一拍）：① 与已登记名一致 —— 技能参考 `references/safe-isolation.md`
		//   与缺口账写的就是 `zerg isolate-ro new|verify`；② 不撞 standing 判据 —— `dev` 族被钉成
		//   「只增改环 · 读环一条都不新增」（`TestDevFamilyIsACHangedOnlyRingNoNewReadCommands`
		//   + 四道闸 `G2` 的七条闭集），那条属性是**有意的**；隔离是工具类正门、不是 `dev` 族的读环。
		// 名字分家：`new` = 造副本（写只落在副本/`--dst`）· `verify` = 只读逐件 sha256 验回。
		{
			path:        []string{"isolate-ro"},
			summary:     "只读隔离：「在别处试」的**硬拷贝**副本（new）/ 逐件 sha256 验回没碰真仓（verify）· 真搬 · 行为落 Go —— 不再 exec scripts/dev/isolate-ro.sh",
			usage:       "zerg isolate-ro new <真仓> [--dst <目录>] [--include <glob>]… [--exclude <路径>]… [--all] [--reject-symlinks] [--no-manifest] [--quiet] | zerg isolate-ro verify <副本> <真仓> [--max-examples N] [--quiet] | zerg isolate-ro --self-test",
			arity:       "any",
			args:        []string{"动作：new（造副本 · 真仓只读）| verify（只读对拍）", "旗标与退码逐档照 scripts/dev/isolate-ro.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdIsolateRO,
		},
		// ---- A 档 批 1（换件面）· A1：主控换件的 **CLI 正门**（`zerg swap core` · 新族 `swap` · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A1（★事故面：把最险的手工换件口收进受控正门）。
		// 底层原件 = `scripts/build/zerg-swap-core.sh`（2026-09-16 事故后固化：三道门禁 ＋ 两道前置闸；
		// 真换生产 `bin/zerg-core` 并重启 launchd ⇒ **动生产 `bin/` ⇒ 冻结面**）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作真跑会动生产 `bin/` ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数/`--self-test-rollback`）。
		{
			path:        []string{"swap", "core"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "主控换件（★事故面 · 真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-core.sh）",
			usage:       "zerg swap core [--help | --self-test-rollback] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-core.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapCore,
		},
		// ---- A 档 批 1（换件面）· A2：子端换件的 **CLI 正门**（`zerg swap agent` · 族 `swap` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A2。
		// 底层原件 = `scripts/build/zerg-swap-agent.sh`（只报告档：真跑 `git status`/`codesign`/`launchctl`/`curl`
		// 探针；真换件档未开放 ⇒ `--swap` 一律 rc=2 零副作用）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作归换件面 ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数）。
		{
			path:        []string{"swap", "agent"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "子端换件（真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-agent.sh；只报告档）",
			usage:       "zerg swap agent [--check | --help | --self-test | --machine <名> [--label <作业名>]] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-agent.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapAgent,
		},
		// ---- A 档 批 1（换件面）· A3：桌面 UI 换件的 **CLI 正门**（`zerg swap ui` · 族 `swap` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A3。
		// 底层原件 = `scripts/build/zerg-swap-ui.sh`（只报告档：真跑 `git status`/`codesign`/进程 inode 对拍；
		// 真换件档未开放 ⇒ `--swap` 一律 rc=2 零副作用）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作归换件面 ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数）。
		{
			path:        []string{"swap", "ui"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "桌面 UI 换件（真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-ui.sh；只报告档）",
			usage:       "zerg swap ui [--check | --help] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-ui.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapUI,
		},
		// ---- ② 类 CLI 工具子命令（命令树登记 · 2026-10-08）────────────────────────────────
		// 三条 CLI 工具子命令（✗ 不是判据 ⇒ ✗ 不进 `gate_inproc_table.go`，那会伪造判据名）；
		// 形状照同族先例（`passthrough: true` + `run` 直调实现件）。
		// ★ 三条均已**真搬**（行为落 Go · ✗ 不再 exec 底层脚本）：`deploy_launch.go` / `doc_site_build.go`
		//   / `ui_start.go` —— 底层脚本退役（移出仓）后两态三面逐字同（✗ 不再退 `8`）。
		//   ★ `deploy launch` 含**意图件落盘** ⇒ 是**写面** ⇒ 登记 `danger` + `opened`（fail-closed）。
		// ---- A 档 批 4（机群/发布面）· A13：机群发布的 **CLI 正门**（`zerg deploy launch` · 新族 `deploy` · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 4 · A13。
		// 底层原件 = `deploy/scripts/deploy-launch.sh`（真跑 `ssh` 到节点跑发布器 · OS 原生/联网面）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// 写面（意图件落盘 `~/.zerg/deploy_launch/`）⇒ 默认档 fail-closed（缺 `--yes` 退 2 · 零写）。
		{
			path:        []string{"deploy", "launch"},
			danger:      &dangerSpec{dangerD3, "发布目标主机", "真发布到节点（`ssh` 到节点跑发布器 · 会写主控侧意图件并触发节点侧发布 —— 不可逆动作的邻居）", "GAP-20260927-140 ① · A档批4 A13（真搬后成写面 · 门100 写面登记）", false, dangerScopeRemote},
			opened:      true,
			summary:     "机群发布「意图件 + 回读」（真搬 · 行为落 Go —— 默认档 fail-closed：缺 --yes 退 2 零写）",
			usage:       "zerg deploy launch [--host <主机> --file <意图件> --expect-sha256 <sha> | --help] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 deploy/scripts/deploy-launch.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDeployLaunch,
		},
		// ---- A 档 批 2（文档面）· A14：文档站「导出 + 构建」的 **CLI 正门**（`zerg doc site build` · 族 `doc` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 2 · A14。
		// ★ 真搬（2026-10-08）：`docs/site/export-and-build.sh` 的四段编排落进 `doc_site_build.go`
		//   （✗ 不再 exec 那个脚本）；只 `mkdocs`/`venv`/`pip` 那四处外部工具仍以子进程直调。
		{
			path:        []string{"doc", "site", "build"},
			summary:     "文档站「导出 + 构建」（真搬 · 行为落 Go —— 不再 exec docs/site/export-and-build.sh）",
			usage:       "zerg doc site build [--strict | --serve | --help]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 docs/site/export-and-build.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocSiteBuild,
		},
		// ---- A 档 批 3（运行面）· A6：UI「脱离会话」启动口的 **CLI 正门**（`zerg ui start` · 族 `ui` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A6（原底层件 = `scripts/build/start-zerg-ui.sh`）。
		// ★ 真搬（2026-10-08）：底层行为收进 `ui_start.go`（`setsid`/逐 PID 点名/日志重定向 ⇒ 逐档等价复现 ·
		//   ✗ 不再 exec 那个脚本；只 `ps`/`tail` 这类 OS 工具仍直调）。
		{
			path:        []string{"ui", "start"},
			summary:     "UI 脱离会话启动（真搬 · 行为落 Go —— 不再 exec scripts/build/start-zerg-ui.sh）",
			usage:       "zerg ui start",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/start-zerg-ui.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdUIStart,
		},
		// ---- T-56 余项 · 标定族 `calib`（`scripts/calib/` 3 件 ⇒ ① 收编 · DEV-0010 · 2026-09-21）----
		{
			path:     []string{"calib", "ls"},
			kind:     "Calib",
			summary:  "标定线 3 件的声明面（名字 · 件 · 角色 · 归属 —— 逐件现读，不另抄一份）",
			usage:    "zerg calib ls [--json <字段>]",
			fields:   calibLsFields,
			endpoint: "",
			run:      cmdCalibLs,
		},
		{
			path:     []string{"calib", "show"},
			kind:     "Calib",
			summary:  "单件标定脚本的现状（归属 · 角色 · 执行面 · 退码口径）",
			usage:    "zerg calib show <名> [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（与 `zerg calib ls` 逐字相同）"},
			fields:   calibShowFields,
			endpoint: "",
			run:      cmdCalibShow,
		},
		{
			path:    []string{"calib", "run"},
			kind:    "Calib",
			summary: "跑一支标定脚本（D2 三态 · --dry-run 零副作用 · 缺 --yes ⇒ 2 · 真调旧脚本、退码原样转出）",
			usage:   "zerg calib run <名> [位置参数…] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg calib ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  calibRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件标定脚本（会占机器/模型槽 · 出的是实测档案）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 标定线 3 件", false, dangerScopeUnknown},
			// `opened`: 真跑已开放（`--yes` 就真跑脚本；退码原样透传）。
			opened: true,
			run:    cmdCalibRun,
		},
		// ---- T-56 余项 · 评测族 `eval`（`scripts/evals/` 22 件 ⇒ ①5 收编 / ②4 内部 / ④13 待拍 · DEV-0010）----
		{
			path:     []string{"eval", "ls"},
			kind:     "Eval",
			summary:  "评测线 22 件的逐件归属（① 收编 5 · ② 保留内部 4 · ④ 维持待拍 13 —— 现算不手写）",
			usage:    "zerg eval ls [--json <字段>]",
			fields:   evalLsFields,
			endpoint: "",
			run:      cmdEvalLs,
		},
		{
			path:     []string{"eval", "show"},
			kind:     "Eval",
			summary:  "单件评测脚本的现状（归属 · 一句理由 · 执行面）",
			usage:    "zerg eval show <名> [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（与 `zerg eval ls` 逐字相同）"},
			fields:   evalShowFields,
			endpoint: "",
			run:      cmdEvalShow,
		},
		{
			path:    []string{"eval", "run"},
			kind:    "Eval",
			summary: "跑一支评测脚本（D2 三态 · **只对 ① 收编的件开放** · 缺 --yes ⇒ 2 · 真调旧脚本、退码原样转出）",
			usage:   "zerg eval run <名> [位置参数…] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg eval ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  evalRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件评测脚本（可能要跑着的生产面 · 占机器与模型槽）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 评测线 22 件", false, dangerScopeUnknown},
			// `opened`: 真跑已开放（`--yes` 就真跑脚本；退码原样透传）。
			opened: true,
			run:    cmdEvalRun,
		},
		// ---- 变更影响面（设计-变更影响面-v1.6 §7.1/§7.3/§7.4 · 任务单-影响面实施-20260922 §二 `A1`–`A3` · 2026-09-22）----
		// `A1` = 骨架（人面三行 + 六键包封）；`A2` = 六层取数；`A3` = 波纹卡片（四字段 · why 闭集六选一 ·
		// 四级排序 · 按档裁 · §3.7 公开面行 · §3.8 可逆性行 · 两档 `--for-model`/`--for-human`）。
		// 挂干跑与缓存分别属 `A4`/`A5`。不改 `emitEnvelope`、不落审计。
		// ── 2026-09-29 订正：本条**不是**只读 —— 默认档真写 impact 缓存件（现读写点
		//   `family_impact_cache.go:291/:305/:308`，账 `GAP-20260927-140` 现读坐实）；修前那句
		//   「只读 ⇒ 不写 danger / 不写缓存」是**自证式反证断言**（件里写着不写、实现却写）。
		//   定档与执行门**待拍板**（账 `GAP-20260929-31`：登记 ≠ 拦 —— 实测不给 `--yes` 也 rc=0 且真写），
		//   ★ 2026-10-08：`danger` 已挂（D2 · opened）—— Mr2109 拍「默认档要 fail-closed」（账
		//   `GAP-20260927-140` ①）。fail-closed 落在**缓存写盘点**（见 `cmdImpact`）。
		{
			path:    []string{"impact"},
			kind:    "Impact",
			summary: "改一处会牵动谁（判定只读；默认档写 impact 缓存件要 `--yes` ⇒人面三行 + 波纹卡片 ≤12 条/≤1.2k token + 六键包封）",
			usage:   impactUsageLine,
			arity:   "any",
			args:    []string{"目标（仓内件路径 · 或在册契约 id，如 S-g）"},
			fields:  impactFields,
			// 授权侧（`GAP-20260927-140` ①）：默认档 `impactCacheOn` 真写 impact 缓存件（写点现读 =
			// `impactCacheWritePoints` · owner = `impactCacheStore`）而修前无 `danger`。
			danger:   &dangerSpec{dangerD2, "（目标）", "默认档写 impact 缓存件（状态目录 `impact-cache/<sha>/`）", "GAP-20260927-140 ① · GAP-20260927-90 · family_impact_cache.go 写点登记表", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdImpact,
		},
		// ---- 波① · T1 `G-07`：归档 / 仓外仓命令面（**全账唯一 P0** · 2026-09-23）----
		// 为什么是它：归档六步全靠手写 shell（源件实测 1072 次 `shasum` + 1072 条临时索引）；
		// 形状照业界现成接口（`apt-ftparchive` 的逐件摘要清单 · coreutils 的一进程吃 N 件 ·
		// RFC 8493 BagIt 的「载荷 + 逐件清单 + 标签清单」）。三条都是只读或**只写指定落点**。
		{
			path:     []string{"archive", "hash"},
			kind:     "ArchiveHash",
			summary:  "一批件算 sha256（**一进程吃 N 件**）—— 逐行 `sha256␣␣路径`，与 `shasum -a 256` 逐字相同（手搓 1072 次 shasum 的替身）",
			usage:    "zerg archive hash <件|目录>… [--json <字段>]",
			arity:    "any",
			args:     []string{"件或目录（目录 ⇒ 顶层逐件 · 可给多个）"},
			fields:   []string{"path", "sha256", "bytes"},
			endpoint: "",
			run:      cmdArchiveHash,
		},
		// ── `danger`（**D2**）：本表项**真写盘**（修前无 `danger` ⇒ `zerg help` 把它判成安全档 ✗）
		//   定档理由（逐条读 `family_archive.go:cmdArchiveManifest` 得）：真跑按 `--out` **新建**
		//   一只 BagIt 袋（`data/` + 两份清单）；落点非空**即拒**（不覆盖别人的件）；写失败 ⇒
		//   `os.RemoveAll(out)` **回滚**（盘上不留半个袋）；归档区/载荷**一个字不碰**（不 chmod /
		//   不搬件 / 不删件）⇒「可逆的写」不是「不可逆的破坏」⇒ **D2**（`--yes` 即可），不是 D3。
		//   `usage` 串**2026-09-27 反向对齐**：本条真判「两枚同给 ⇒ 2」（`family_archive.go:263` 调 `dryRunYesConflict`）
		//   ⇒ 逐字回改成互斥形态 `[--dry-run | --yes]`（与准形 `family_gate_matrix.go:980` 同一形态）。
		{
			path:    []string{"archive", "manifest"},
			kind:    "ArchiveManifest",
			summary: "出归档**三件套**（RFC 8493 BagIt：载荷 `data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）· `--dry-run` 先出逐件清单 · 真写要 `--yes` · 失败回滚",
			usage:   "zerg archive manifest <载荷目录> --out <袋目录> [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"载荷目录", "袋落点（--out）"},
			fields:  []string{"bag", "entry", "sha256"},
			danger: &dangerSpec{dangerD2, "袋落点（`--out`）",
				"按 `--out` 新建一只 BagIt 袋（`data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）；落点非空即拒 · 写失败即回滚（删掉本次建的袋）；归档区不碰（可逆：删掉新建的袋目录即回原状）",
				"缺口-命令面-20260921 §一 G-07 · RFC 8493 BagIt · 同族写面先例 `zerg gap export`（同样真写盘）· §九 M3 C1", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdArchiveManifest,
		},
		{
			path:     []string{"archive", "verify"},
			kind:     "ArchiveVerify",
			summary:  "校验一只袋（**重算载荷** ↔ 清单逐件对拍：清单被抹一条 / 载荷改一字节 / 多出未登记件 ⇒ 判红）",
			usage:    "zerg archive verify <袋目录> [--json <字段>]",
			arity:    "any",
			args:     []string{"袋目录"},
			fields:   []string{"entry", "want", "got", "verdict"},
			endpoint: "",
			run:      cmdArchiveVerify,
		},
		// ---- 波① · T1a `Q-099`/`Q-100`：加模型 + 热加载（**纯 CLI 活 · 主控零改动** · 2026-09-23）----
		// 为什么是它们（§自排补遗 `S1` 逐字）：`Q-099` 是 P0、`Q-100` 是 P1，调研结论「主控零改动」——
		// 热加载路由**已经在跑的主控上**（`core/cmd/zerg-core/main.go`：`r.Post("/api/config/reload", …)`），
		// 且处理器**自己先解析**（解析不过即回 `CONFIG_LOAD_FAILED`）⇒ 命令面这边只补「先校验后写 /
		// 先校验再请求」两道。今天这两件事只能手改 YAML（手搓插坏名册两次）或重启主控 ✗。
		// ★ `danger` 登记（**D2** · 2026-09-27）：本表项**真写盘**，修前无 `danger` ⇒ `zerg help` 把它当安全档 ✗。
		//   定档理由（逐条读 `family_config.go:cmdModelAdd` 得）：唯一的写盘点是 `writeFleetAtomic`
		//   （`family_config.go:513` 临时件 `os.WriteFile` + `:518` `os.Rename` 原子换入），只给
		//   `gateway/fleet.yaml` 这一件的 `models:` 段**加一条**；不删件、不改别人的条、不碰主控运行态
		//   ⇒ 可逆（删掉那一行即回原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   ★ 三态面的两枚真判**都已接**（本单只补登记，不动实现件）：`--dry-run`（`family_config.go:460`）
		//   ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--yes`（`:489`）⇒ 计划件走 stderr + rc=2
		//   （fail-closed）—— 两条都判在 `writeFleetAtomic`（`:495`）**之前**。
		//   ★ 同族「`--dry-run` 与 `--yes` 同给 ⇒ 2」那条判本表项**未接**（全仓只有 `family_gate_matrix.go:99`
		//   一处）⇒ 本单**禁改既有函数行为** ⇒ 不擅自补（入缺口账，交父代理拍板）。
		//   ★ `opened: true` = 真跑已开放（`--yes` 就写；`--dry-run` 零副作用）—— 漏它会让三处显示面
		//   齐声报「本版未开放」（`cli_danger_opened_test.go` 的对账闸正是钉这个）。
		{
			path:     []string{"model", "add"},
			kind:     "ModelAdd",
			summary:  "往名册件（`gateway/fleet.yaml`）**先校验后写**加一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚 · 不覆盖别人的条）——**块按 `--model`（模型名）定位/新建**（列表形与裸映射形都认）· `--host` 只作该条的 `host:` 字段值",
			usage:    "zerg model add --model <模型名> --host <主机> --file <GGUF 路径> --confirm <模型名> [--backend …] [--mem-gb …] [--size-bytes …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--cmd <JSON 数组>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键 / 块名 ⇒ 按它定位或新建那一块）", "主机（--host：该条 `host:` 的字段值 ＝ 这台模型跑在哪台机器）", "GGUF 路径（--file）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "added"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "往名册件 `gateway/fleet.yaml` 的 `models:` 段加一条（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不删件、不改别人的条、不碰主控运行态）；可逆：删掉那一行即回原状", "缺口账 `GAP-20260927-342` · 同族写面先例 `zerg gap export`（真写盘）· `zerg gate matrix` · §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelAdd,
		},

		// ☆ 波3-A 命令面真修（缺口 `GAP-20260925-31`）：名册「删一条 / 改一条」两动作面。
		//   定档理由（逐条读 `family_config.go:cmdModelRm` 得）：唯一写盘点是 `writeFleetAtomic`
		//   （同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚），只动 `models:` 段里
		//   点准的那一条 ⇒ 可逆（把删掉那行加回即原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   三态面：`--dry-run` ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--yes` ⇒ 计划件走 stderr
		//   + rc=2（fail-closed）；`--dry-run` 与 `--yes` 同给 ⇒ 2（`guard.go:dryRunYesConflict`）。
		{
			path:     []string{"model", "rm"},
			kind:     "ModelRm",
			summary:  "从名册件（`gateway/fleet.yaml`）**先校验后写**删一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）——按 `--model`（块名）+ `--host`（该条 `host:`）点准那一条；删空则连块首行一起删",
			usage:    "zerg model rm --model <模型名> --host <主机> [--file <GGUF 路径>] --confirm <模型名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键/块名）", "主机（--host：要删那条的 `host:`）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "block_removed", "remaining"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "从名册件 `gateway/fleet.yaml` 的 `models:` 段删一条（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把删掉那一行加回即原状", "缺口账 GAP-20260925-31 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelRm,
		},
		{
			path:     []string{"model", "set"},
			kind:     "ModelSet",
			summary:  "改名册件（`gateway/fleet.yaml`）里**点准的那一条**的字段（`--set k=v` · 保序原位改 · 其余字段逐字保留 · `--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）——字段名闭集，无名键 / 列表值 / 值域不合 ⇒ 退 2（不猜）",
			usage:    "zerg model set --model <模型名> --host <主机> [--file <GGUF 路径>] --set k=v [--set k=v …] --confirm <模型名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键/块名）", "主机（--host：要改那条的 `host:`）"},
			fields:   []string{"model", "host", "fleet", "line", "changed", "entry"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "改名册件 `gateway/fleet.yaml` 里点准的那一条的字段（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把字段改回即原状", "缺口账 GAP-20260925-31 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelSet,
		},

		// ☆ 波3-A 命令面真修（缺口 `GAP-20260925-32`）：机器名册 `nodes:` 的第一条改面。
		//   定档理由（`family_config.go:cmdContextSetPort` 现读）：只改 `~/.zerg/contexts/<档>.yaml`
		//   里**现成的** `port:` 一行（临时件 + rename 原子写 · 写完读回逐字节再校 · 任一步不过 ⇒ 回滚）；
		//   可逆（把端口改回即原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   三态面：`--dry-run` ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--confirm`/`--yes` ⇒ 2（fail-closed）。
		{
			path:     []string{"context", "set-port"},
			machine:  machineNodeName,
			kind:     "ContextSetPort",
			summary:  "改机器名册（`~/.zerg/contexts/<档>.yaml` 的 `nodes:`）里某台机**现成的** `port:`（先校验后写 · `--dry-run` 先行 · 真写要 `--confirm=<机器名>` + `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）—— host/port/os/kind 的唯一写入口",
			usage:    "zerg context set-port --node <机器名> --port <端口> [--context <档名>] --confirm=<机器名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（--node：`nodes:` 段的一级键）", "端口（--port：1..65535）"},
			fields:   []string{"node", "port", "context", "line", "changed"},
			danger:   &dangerSpec{dangerD2, "机器名", "改机器名册 `~/.zerg/contexts/<档>.yaml` 里该节点现成的 `port:`（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把端口改回即原状", "缺口账 GAP-20260925-32 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdContextSetPort,
		},
		// ---- 批 B7（2026-10-20 · `zerg compat list|check|migrate|manifest`）：独立入口
		// `core/cmd/zerg-compat` 归入命令面 ----
		// 源件 243 行 · 四个子命令（`list` / `check` / `migrate` / `manifest`）· 四枚旗标逐字保留
		// （`--state-dir` / `--receipts-dir` / `--dry-run` / `--json`）。
		// ★ 派单原话是「并入**既有** compat 族」—— 而**现读没有这个族**（`zerg help --all` 零命中 ·
		//   本文件 `compat` 零命中）⇒ 无处可并，只能**新开顶层族 `compat`**（族名 = 源件名的主体）。
		//   为什么不挂 `build`（`build --only compat` 是构建档名，语义完全不同）/ `doctor` / `gate`：
		//   理由逐条写在 `family_compat.go` 的件头（照实记一笔，不粉饰派单与现读的差）。
		// 判定与落盘一行不改（仍是 `core/internal/compat` 的 Entries/StatusAll/MigrateAll/
		//   ManifestJSONForDump）；退码按 `exitcodes.go` **重取号**（源件 2/3 ⇒ 命令面 8/8，以
		//   `error.detail` 分家；源件 4「用法错」与命令面 4「未认证」撞号 ⇒ 一律回表取 2）。
		// ★ 写面只有 `migrate`（原地重写状态文件、先备份）：不进危险表 —— 它写的是**本机状态目录**、
		//   落点由 `--state-dir` 或环境变量显式给、每次覆盖前自动备份 `<file>.bak-<ts>`、`--dry-run`
		//   可预演；与 `model record probe`（批 B6）同一条「本机可逆 · 无远端副作用」口径。
		// **双轨期**：`core/cmd/zerg-compat` 二进制一字不动（它自己的 0/1/2/3/4 是**组件自有码空间**，
		//   `exitcode-namespaces.json` 里挂着号）。
		{
			path:     []string{"compat", "list"},
			kind:     "CompatList",
			summary:  "列出跨版本状态文件清单（只读）：逐条给 逻辑名 / 文件 / 当前 schema / 最低可读 / 信封（inband·sidecar）/ 谁盖版本号 / 归属，并打出它将要操作的两个目录",
			usage:    compatListUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "file", "dir", "current_schema", "min_readable", "envelope", "stamps_schema", "owner"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatList,
		},
		{
			path:     []string{"compat", "check"},
			kind:     "CompatCheck",
			summary:  "只读体检：逐文件报现状（不迁移、不备份、不写盘）—— 全部就绪 ⇒ 0 · 有读不动的状态文件 ⇒ 1 · 有待迁移文件或高于本机认知的 schema ⇒ 8（`error.detail` 分家两档）",
			usage:    compatCheckUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "path", "outcome", "on_disk_schema", "current_schema", "min_readable", "envelope", "owner", "error"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatCheck,
		},
		{
			path:     []string{"compat", "migrate"},
			kind:     "CompatMigrate",
			summary:  "执行一次性迁移（幂等 · 可反复跑）：旧版/无 schema 先备份 `<file>.bak-<ts>` 再原地写回并回读校验，校验不过回滚；`--dry-run` 只报将要迁移的文件、不落盘；失败 ⇒ 1",
			usage:    compatMigrateUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "path", "outcome", "error", "on_disk_schema", "current_schema", "min_readable", "envelope", "owner"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatMigrate,
		},
		{
			path:     []string{"compat", "manifest"},
			kind:     "CompatManifest",
			summary:  "打印内嵌清单原文（CI 门禁 / 排障的对照面）—— 直接写原始字节，不再序列化一遍（`<` 不许被转义成 `\\u003c`）；不收任何旗标",
			usage:    compatManifestUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数、也不收旗标：这一面就是清单原文）"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatManifest,
		},
		// ★ `danger` 登记（**D2** · 2026-09-27）：本表项**真跑有副作用**，修前无 `danger` ⇒ `zerg help` 把它当安全档 ✗。
		//   定档理由（逐条读 `family_config.go:cmdConfigReload` 得）：本件**零 os 落盘调用**（无 `os.WriteFile` /
		//   `os.MkdirAll` / `os.Create` / `os.Rename` / `os.Remove`）—— 它的效果面是 **HTTP POST**
		//   （`family_config.go:599` `POST /api/config/reload`：让在跑的主控重读名册、路由表即时更新）；
		//   不重启、不停任何进程、已加载模型不受影响；名册件本地解析不过 ⇒ 一个请求都不发（旧配置继续跑）
		//   ⇒ 可逆（再热加载一次即回旧档）⇒ **D2**（`--yes` 即可），不是 D3 —— 同族先例 = `zerg core reload`
		//   表项（`{\"core\", \"reload\"}`：同为「让主控重读配置」的 D2）。
		//   ★ 三态面的两枚真判**都已接**（本单只补登记，不动实现件）：`--dry-run`（`family_config.go:580`）
		//   ⇒ 计划件走 stdout + rc=0 且**一个请求都不发**；缺 `--yes`（`:593`）⇒ 计划件走 stderr + rc=2
		//   （fail-closed）—— 两条都判在 `sendJSON`（`:599`，本件唯一副作用点）**之前** —— 与
		//   `family_gate_matrix.go:99` 那处只差「两枚同给 ⇒ 2」一条（本单不补，见缺口账）。
		//   ★ `opened: true` = 真跑已开放（`--yes` 就发那一次请求；`--dry-run` 一个请求都不发）。
		{
			path:     []string{"config", "reload"},
			kind:     "ConfigReload",
			summary:  "热加载主控配置（照 `nginx -s reload`：**先校验、失败回滚**）—— 名册件本地解析不过 ⇒ **不发请求**（旧配置继续跑）",
			usage:    "zerg config reload [--dry-run] --yes [--json <字段>]",
			arity:    "none",
			args:     []string{"（无名册件参数：走 `--root` / `--path` 或仓根）"},
			fields:   []string{"status", "models", "added", "fleet_nodes", "fingerprint"},
			danger:   &dangerSpec{dangerD2, "（群级：不点名主机）", "让在跑的主控重读名册件（`POST /api/config/reload`）—— 生效面即时（路由表更新）；不重启、不停任何进程、已加载模型不受影响 · 名册件解析不过 ⇒ 一个请求都不发（旧配置继续跑）；可逆：再热加载一次即回旧档", "缺口账 `GAP-20260927-342` · 同族先例 `zerg core reload`（同为 D2）· §三 C 族 · §6.3 S5 · §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/config/reload（路由已在跑的主控上 ⇒ 主控零改动）",
			run:      cmdConfigReload,
		},
		// ── `gap` 族（缺口 · 设计稿 `设计-命令面-gap族-v1.0-20260923.md` §二/§五）────────────
		//   三条：`ls` 只读 · `add` 写（留证据）· `verify` 写（改状态）。**不新增顶层命令**；
		//   真源 = `<状态目录>/zerg-cli-gaps.jsonl`（不在任何仓里）；审计进 `dev edit` 同一件。
		//   ★ 矩阵 13 格与基线计数**同批**（`H-2`：分批 ⇒ 门⑬ `R4` 判幽灵调用）。
		{
			path:     []string{"gap", "ls"},
			kind:     "GapList",
			summary:  "缺口账（只读面：不写真源、不写审计）· 可按状态/优先级/影响面收窄 · 账内闭集外的值 ⇒ 判红并**点名到行**",
			usage:    "zerg gap ls [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--unit <件路径|目录前缀>] [--module <模块前缀>] [--grep <关键词>] [--untriaged] [--solved-kind <实现|销案|前提不成立|部分|未分类>] [--json <字段>]",
			fields:   gapListFields,
			endpoint: "",
			run:      cmdGapLs,
			// ★ `GAP-20260928-163`（2026-10-10 · 本枚）：`truncated`/`hits`/`total` 走**可选字段层**
			//   （`optFields`）—— 跑级截断自报的直读口，让**全量档与收窄档同尺**（两档都点得出这
			//   三格，同源 = 与 `meta.total`/`meta.hits`/`inv.markTruncated()` 同一份
			//   `total`/`hits`/`cut`）；登在出口白名单里、`--json` 点得出来，但**不进** `fields`
			//   那份冻结帮助 / 文档导出面字段表（与 `code find` 的 `count` 同一体例）⇒ 不造
			//   「表列着、面不产出」的反例，也无需重生成命令页。
			//   ★ 本枚（缺口 `GAP-20260928-54`）：`triage_batch` 同样走**可选字段层**（`optFields`）—— 分诊批次
			//   标记的直读口，登在出口白名单、`--json` 点得出来，但**不进** `fields` 那份冻结帮助 /
			//   文档导出面字段表（与 `truncated`/`hits`/`total`、`code find` 的 `count` 同一体例）
			//   ⇒ 无需重生成命令页，也不造「表列着、面不产出」的反例。
			// ★ 缺口 GAP-20260927-322（批⑤ · 2026-10-13）：`frozen`（账真源**读面冻结锚**：sha16 · 行数 ·
			//   读时点）同走**可选字段层** —— 登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表。
			optFields: []string{"truncated", "hits", "total", "triage_batch", "file", "frozen"},
		},
		// ★ `gap status` —— **排行面**（批1 第四片 · 只读面 · 2026-09-28）：按件（`--by unit` · 缺省）
		//   或按模块（`--by module`）分桶排行；**批4 第二片**再加 `--by tier`（按判据分级分桶 ——
		//   真判据 / 占位 / 无 三桶，列 = 条数 + 其中仍缺 + 其中已解，表尾固定两行：守恒式自校
		//   「三桶之和 == 总账」（不等 ⇒ 判红 1 并打印两数）+ 占位桶只报告（不退码））。
		//   实现件 = `family_gap.go:cmdGapStatus`。
		//   排序两键（都写进表头）：① 未闭降序 ② 近邻量化 P0/P1/P2 降序；对账等式自校
		//   （Σ(有件桶未闭) + 无件桶 == 账内仍缺）；带 `query_ts` / `ledger_sha16`（两个时刻的排行才可比）。
		{
			path:     []string{"gap", "status"},
			kind:     "GapStatus",
			summary:  "缺口账**排行面**（只读）：按件（`--by unit` · 缺省）/ 按模块（`--by module`）/ 按**判据分级**（`--by tier`）／按**入账日**（`--by day` · 新增走 found_at 轴、销案走 solved_at 轴、同给净）分桶。按**命令族**（`--by family` · 走账自带 `want_family` 原值成桶）分桶。件轴／模块轴：逐桶给 **未闭 / 已解 / 净**（可为负）+ 近邻量化（P0/P1/P2 计数）· 排序键 = ① 未闭降序 ② 近邻量化降序（都写进表头）· 对账等式自校（Σ(有件桶未闭) + 无件桶 == 账内仍缺 · 不成立 ⇒ `warnings` 点名差数）。**tier 轴**：行 = 三桶闭集（真判据 / 占位 / 无 —— `verify_tier` 已落库则用它，未落库则现读 `verify_cmd` 现算：空串/无键 ⇒ 无 · 含「占位」字样 ⇒ 占位 · 其余且能被命令树解析 ⇒ 真判据）· 列 = **条数 / 其中仍缺 / 其中已解** · 表尾固定两行：① 守恒式自校「三桶之和 == 总账」（两数各自现算 · 不等 ⇒ **判红 1** 并打印两数）② 占位桶**只报告不阻断**（不退码 · 先量分布）。全轴带 `query_ts` / `ledger_sha16`（不同时刻两个排行才可比）· 截断自报（`--top` 一页硬顶 = `gapLsRowCap`）· 读不到真源 ⇒ 8",
			usage:    "zerg gap status [--by <unit|module|tier|day|family>] [--top <N>] [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--json <字段>]",
			fields:   gapStatusFields,
			endpoint: "",
			run:      cmdGapStatus,
		},
		// ★ `gap plan` —— **作业单六件**（批3 第二片 · 只读面 · 2026-09-28 · 设计稿 §3.C `C1` + §11.3）。
		//   选择器 = 第一个位置参数（件路径或模块目录前缀 · 与 `gap ls --unit/--module` 同口径）；
		//   `--module` 声明按模块面算（前导匹配）；`--top N` 一页硬顶（缺省 20 · 上限同 `gapLsRowCap`）。
		//   输出六件（设计稿 §11.3 逐字）：① 该桶全部未闭（按 prio 排序）② 出口判据 = 该桶未闭归零
		//   （回显现读未闭数）③ 建议允许面（件清单）④ 禁碰面（建议值 + 理由）⑤ 占用状态（桶内 `已派`
		//   条逐条点名卵号 · 判据 2 `M-32`）⑥ 派单模板骨架（目标/允许面/禁碰面/出口判据/时限 · 无占位符）。
		//   ★ **零未闭 ≠ 空件**（桶内有条目而 `仍缺==0` ⇒ 照出六件、真报未闭 0、退码 0）；选择器零命中 ⇒ 1。
		//   实现件 = `family_gap.go:cmdGapPlan`。
		{
			path:     []string{"gap", "plan"},
			kind:     "GapPlan",
			summary:  "作业单**六件**（只读面 · 一字不写真源、不写审计）：把一件（或一模块）的未闭账变成可直接丢给子代理的派单骨架 —— ① 该桶全部未闭（按 prio 排序 P0→P1→P2 · 同 prio 按缺口号 · `--top` 一页硬顶）② 出口判据 = 该桶未闭归零（回显现读未闭数）③ 建议允许面（该桶涉及的件清单）④ 禁碰面（建议值 + 理由）⑤ 占用状态（桶内 `已派` 条逐条点名卵号 · 判据 2 `M-32`）⑥ 派单模板骨架（目标/允许面/禁碰面/出口判据/时限 · 无占位符）· 选择器 = 第一个位置参数（件路径或模块目录前缀）或 `--unit`/`--module`；`--module` 声明按模块面算（前导匹配 `gapLsModuleHit`）· **零未闭 ≠ 空件**（桶内有条目而 `仍缺==0` ⇒ 真报未闭 0、退码 0）；选择器零命中 ⇒ 1；缺选择器/`--top` 非正整数 ⇒ 2；读不到真源 ⇒ 8",
			usage:    "zerg gap plan <件路径|模块目录前缀> [--module <模块前缀>] [--unit <件路径|目录前缀>] [--top <N>] [--allow-drifted] [--json <字段>]",
			arity:    "any",
			args:     []string{"选择器（件路径或模块目录前缀 · **恰好一个** —— 与 `gap ls --unit/--module` 同口径，含「无件」闭集取值）"},
			fields:   gapPlanFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapPlan,
		},
		// ★ 批3 第四片（2026-09-28）：`gap receipt check` / `gap receipt apply` —— **回执收件面**。
		//   设计出处：§三十四 `O-12`（卵回执结构规范化：`closed_ids[]` 必填 · 取代散文）+
		//   施工清单 3-4（回执缺栏即拒收 · 闭案与账一一对应）。实现件 = `family_gap.go`。
		//   ★ 回执形状本片取定（设计稿只钉死 `closed_ids` 一名）：`egg` / `readings[]`（每条 `gap_id`+`result`）/
		//   `conclusion` / `closed_ids[]`（键必须在 · 可为空数组）—— 缺栏 ⇒ 2 并点名缺哪栏。
		{
			path:     []string{"gap", "receipt", "check"},
			kind:     "GapReceiptCheck",
			summary:  "回执**收件体检**（只读面 · 一字不写真源、不写审计）：按设计稿 `O-12` 逐条验 —— 四必填栏（卵号 `egg` · 逐条读数 `readings[]`（每条含 `gap_id`+`result`）· 结论 `conclusion` · 闭案号 `closed_ids[]`（**可为空数组，但键必须在**））缺栏 ⇒ 2 并点名缺哪栏 · 闭案与账一一对应（`closed_ids` 每个号必须在账里且当前 `state=已派`，否则点名冲突 ⇒ 2）· 反面（账里 `已派` 但回执没提）⇒ 列为「未结派单」告警（**不阻断退码**，只报告）· 读不到回执件或真源 ⇒ 8",
			usage:    "zerg gap receipt check <回执件路径> [--json <字段>]",
			arity:    "any",
			args:     []string{"回执件路径（**恰好一条** JSON 件）"},
			fields:   gapReceiptCheckFields,
			endpoint: "",
			run:      cmdGapReceiptCheck,
		},
		// ★ 缺口账 `GAP-20260928-184` 第三面（2026-10-10）：`gap receipt new` —— **卵闭案回执的生产者**。
		//   病：`gap receipt check` 要四栏卵闭案回执形状，而全仓只有 `check`/`apply`、**零产生口**
		//   （存量 receipts 全是 `dev receipt` 轮级交接回执族）。治：立合法产生口（同族 `dev receipt new` 体例）·
		//   落件沿用 receipts 目录但文件名前缀 `GR-`、件内四栏自证 ⇒ 与 `<R-…>` 族可自我区分；
		//   `check` 亦按签名键（`round_id`）判族（错族显式点名，不当「缺 4 栏」）。实现件 = `family_gap.go`。
		{
			path:     []string{"gap", "receipt", "new"},
			kind:     "GapReceiptNew",
			summary:  "卵闭案回执**生产者**（写面 · D2 档 · 缺 `--yes` fail-closed 2）：落一件卵闭案回执（`{egg, readings[], conclusion, closed_ids[]}`）到 receipts 目录（文件名前缀 `GR-`）—— 四栏与 `gap receipt check` 的收件判据**同一把尺**（生产者不许产出 check 必拒的件）· `--dry-run` 出计划件 · `--json` 走同族信封",
			usage:    "zerg gap receipt new --egg <卵号> --reading <缺口号=结果>… --conclusion <结论> [--closed-id <GAP id>]… [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"（不收位置参数 —— 四栏走旗标）"},
			fields:   gapReceiptNewFields,
			danger:   &dangerSpec{dangerD2, "卵闭案回执件", "落一件卵闭案回执（状态目录 `receipts/GR-<卵号>-<时刻>.json`）—— 逐条读数 + 结论 + 闭案号（可逆：删该件即回原态）", "缺口账 GAP-20260928-184 第三面（缺口账与自进化 · 2026-10-10）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapReceiptNew,
		},
		{
			path:     []string{"gap", "receipt", "apply"},
			kind:     "GapReceiptApply",
			summary:  "回执**闭案落账**（写面 · 缺 `--yes` fail-closed 2）：把 `closed_ids` 逐条改 `已解`（**走现有改态路径 `gapRewriteOne` · 不另开写路**）· 缺 `--evidence` ⇒ 2 · 有冲突号（账里非 `已派`）⇒ 2 且**不动账** · 成败后回显逐条改前→改后计数 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap receipt apply <回执件路径> --evidence <一句话> --yes [--by <谁>] [--json <字段>]",
			arity:    "any",
			args:     []string{"回执件路径（**恰好一条** JSON 件）"},
			fields:   gapReceiptApplyFields,
			danger:   &dangerSpec{dangerD2, "回执件（`closed_ids`）", "把回执 `closed_ids[]` 里的号在真源逐条改 `已解`（`state`/`solved_at`/`solved_evidence`）+ 审计逐条一行（可逆：照审计那一格回写）；缺栏 / 冲突号 ⇒ 2（一个字节都不写）", "批3 第四片（缺口账与自进化 · 2026-09-28）· 设计稿 §三十四 `O-12` + 施工清单 3-4 · 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapReceiptApply,
		},
		// ★ `gap show` —— **单条取全文**（2026-09-28 · 缺口账 `GAP-20260926-15` + 同族 `GAP-20260927-243`）：
		//   只读面（不登记 `danger` ⇒ 按只读幂等档列）· 点名面复用同族 `gapTargetOne`（**账内没有这个 id ⇒ 2**，
		//   与 `set-state` / `note` 同一口径）· 读不到真源 ⇒ 8。实现件 = `family_gap.go:cmdGapShow`。
		{
			path:      []string{"gap", "show"},
			kind:      "GapShow",
			summary:   "取一条缺口的**完整正文**（只读面 · 一字不截）：正文 / 手搓记录 / 复现 / 判据 / 已解证据 / 口径注全量出 —— `gap ls` 那一行只印摘要前 40 显示宽、`--json` 全量直出又会被读方截断（缺口账 `GAP-20260927-243`）· 账内没有这个 id ⇒ 用法错 2（同族口径 · 不跨族抄 `dev proposal show` 的 1）· 读不到真源 ⇒ 8",
			usage:     "zerg gap show <GAP id|件路径> [--json <字段>]（位置参数给一条件路径 = 按件反查：该件在账里被点到的条目逐条列出 · 件不在仓里 ⇒ 2 · 仓根取不到 ⇒ 8）",
			arity:     "any",
			args:      []string{"缺口 id（**恰好一条** —— 本面只取一条）"},
			fields:    gapShowFields,
			optFields: []string{"hints"},
			endpoint:  "",
			run:       cmdGapShow,
		},
		// ★ `gap hints` —— **正门提示面**（只读 · 2026-10-10 · 缺口 `GAP-20261010-26` ＝ 父
		//   `GAP-20260929-09` 的第二形态「正门 gap/contract 子命令出口」）。逐条给「怎么核 /
		//   怎么复现 / 下一步」的落点提示；真源 = 账内既有 `verify_cmd`/`repro_cmd`（✗ 不新增
		//   真源键）。只读面（不登记 `danger`）· 读不到真源 ⇒ 8。实现件 = `family_gap.go:cmdGapHints`。
		{
			path:     []string{"gap", "hints"},
			kind:     "GapHints",
			summary:  "缺口**正门提示面**（只读面 · 一字不写真源、不写审计）：逐条给「怎么核 / 怎么复现 / 下一步」的落点提示（真源 = 账内既有 `verify_cmd`/`repro_cmd`，✗ 不新增真源键）· `gap show <id> --json hints` 给同一条的同一份提示（走可选字段层）· 读不到真源 ⇒ 8",
			usage:    "zerg gap hints [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = 整账 —— 要收窄走 `gap ls`）"},
			fields:   gapHintsFields,
			endpoint: "",
			run:      cmdGapHints,
		},
		{
			path:     []string{"gap", "add"},
			kind:     "GapAdd",
			summary:  "记一条缺口（写面 · 留证据）：**手搓记录 + 验证命令是两件必填**（防呆⑤）· 同 fp 同内容 ⇒ 幂等命中 0 · 同 fp 内容不同 ⇒ `14` · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap add --symptom <一句> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] [--state <仍缺|已立项|不做>] --repro-cmd <命令> --verify-cmd <命令> [--depends-on <fp>…] [--note <注>|--notes <注>|--evidence <注>…] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapAddFields,
			danger:   &dangerSpec{dangerD2, "缺口 fp", "往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删）；审计写不进 ⇒ 真源一行不写", "设计-命令面-gap族-v1.0-20260923.md §二.2 · §四 · `H-10`（不要人签：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapAdd,
		},
		{
			path:     []string{"gap", "verify"},
			kind:     "GapVerify",
			summary:  "跑判据（`verify_cmd`）改缺口状态（写面）：过 ⇒ `已解` + `solved_evidence`·**已解现缺** ⇒ 记 `回归` 并退 1 · `--dry-run` 只跑只印（恒 0）",
			usage:    "zerg gap verify [<GAP id>…] [--all] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（可重复；与 `--all` 不许同给）"},
			fields:   gapVerifyFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_evidence` / `last_verified_at` + 审计一行（可逆：照审计那一格回写）；判红（`回归`）只在真跑那一态可达", "设计-命令面-gap族-v1.0-20260923.md §二.3 · §三 · `H-10`（判据是机器给的：跑命令看 rc）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerify,
		},
		// ★ 批4 第一片（2026-09-28）：`gap verify-one <GAP id>` —— **跑判据即写**（一条一次）。
		//   病（设计稿 §11.4 批4 · §11.2 批2 D1/D2）：`gap verify` 是**批量面**（点名多条 / `--all`），
		//   且只有 `--yes` 那一态写 `last_verified_at` ⇒ 「跑判据即写」没落地（现读账里该格仅 1 条）。
		//   本面 = 一条一次：跑该条的 `verify_cmd`，把 **last_verified_at / last_verify_rc /
		//   verify_tier** 三格写回该条（走本族唯一收口 `gapRewriteOne`：审计先落盘 → 整件重写 →
		//   读回对拍；只重写目标那一行）。
		//   ⚠ 命令名取定 = `verify-one`：`gap verify` 这一格**已被判据批量面占用**（形状/退码一字不许动）
		//   ⇒ 不占用同一条命令树路径（回执 CLI 缺口栏如实点名）。
		//   四条硬纪律：① `--dry-run` 只出计划件（**不跑判据**、不写账 · 恒 0）② 缺 `--yes` 真写 ⇒ fail-closed 2
		//   ③ 该条**无 `verify_cmd`** ⇒ 2 并点名（不当绿）④ 带 shell 元字符（管道/重定向/分号/与或）的判据
		//   **一律拒跑并点名**（不当绿也不当红）。
		{
			path:     []string{"gap", "verify-one"},
			kind:     "GapVerifyOne",
			summary:  "跑一条缺口的判据（`verify_cmd`）**一条一次**并**把结果写回该条**（写面）：写 `last_verified_at`（现取时刻）· `last_verify_rc`（真退码）· `verify_tier`（真判据/占位/无）· 走本族唯一收口 `gapRewriteOne`（只重写目标那一行 · 审计进 `edit_audit.jsonl`）· `--dry-run` 只出计划件（不跑判据 · 恒 0）· 缺 `--yes` ⇒ 2 · 无 `verify_cmd` ⇒ 2 点名 · 判据带 shell 元字符 ⇒ 拒跑点名",
			usage:    "zerg gap verify-one <GAP id> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本面一条一次）"},
			fields:   gapVerifyOneFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `last_verified_at` / `last_verify_rc` / `verify_tier` + 审计一行（可逆：照审计那一格回写）；**本面不改 `state`**（改态是 `gap verify` / `set-state` 的事）", "设计-缺口账与自进化-v2.0-20260928.md §11.4 批4 · §11.2 批2 D1/D2 · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerifyOne,
		},
		// ★ 批4 第三片（2026-09-28）：`gap regress <件…>` —— **分层回归巡检**（只读面）。
		//   设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.4 批4 D3「回归巡检」+ §附录
		//   「巡检条款措辞限缩（35.2 源四）」。病（§11.2 批4 · §11.4）：`verify-one` 一条一次 ⇒
		//   改一件之后无法快速知道「我碰过的那件名下账里那些条还站不站得住」；全跑贵且慢（`M-34`）。
		//   本面 = **只跑被点名件名下**的账（`unit` **精确等值**该件 · 不含模块前缀面），逐条跑其
		//   `verify_cmd`（进程内走同一 `run` 入口 · 不拼 shell 串），逐件汇总六数：
		//   条数 / 实跑 / 通过 / 失败 / 拒跑 / 未跑（被 `--limit` 截掉的 · 缺省 20）。
		//   ★ **声称纪律（本片核心）**：表尾**逐字**声明「本次只覆盖 modification-traversing 级
		//   （只跑了与被改件直接相关的账）；这不等于 safe，也不覆盖非确定性/环境依赖场景」。
		//   某件零条 ⇒ 明写「零条（不是通过）」。
		//   ★ 安全边界：只跑账里那一格命令本身；含 shell 元字符（管道/重定向/分号/与或/反引号/`$`/
		//   括号）⇒ **一律拒跑并点名**（不当绿也不当红）。本面**全只读**（不写账、不写审计、不改态）。
		//   退码：0 全过（含零条 / 全是拒跑）· 1 有条判红（`verify_cmd` rc≠0）· 2 用法错（缺件 /
		//   件不在仓 / `--limit` 非非负整数 / `--json` 不给字段）· 8 真源读不到 / 仓根取不到。
		//   实现件 = `family_gap.go:cmdGapRegress`。
		{
			path:     []string{"gap", "regress"},
			kind:     "GapRegress",
			summary:  "缺口账**分层回归巡检**（只读面 · 一字不写真源、不写审计、不改任何 `state`）：只跑被点名件名下（`unit` **精确等值**该件 · 不含模块前缀面）账内各条的 `verify_cmd` —— ① 汇总六数：件 → 条数 / 实跑 / 通过 / 失败 / 拒跑 / 未跑（被 `--limit` 截掉的 · 缺省 20）② 状态分栏（仍缺/已派/已立项/已解/回归/不做 · 不分状态全列）③ **声称纪律**：表尾逐字声明「只覆盖 modification-traversing 级（只跑了与被改件直接相关的账）；这不等于 safe，也不覆盖非确定性/环境依赖场景」④ 某件零条 ⇒ 明写「零条（不是通过）」。安全边界：只跑账里那一格命令本身 —— 含 shell 元字符（管道/重定向/分号/与或/反引号/`$`/括号）⇒ **一律拒跑并点名**（不当绿也不当红）；写面/无判据/占位 ⇒ 拒跑。件不在仓 ⇒ 2 并点名；`--limit` 非非负整数 ⇒ 2；读不到真源 / 仓根取不到 ⇒ 8。",
			usage:    "zerg gap regress <件路径>… [--limit <N>] [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"件路径（仓内相对路径 · **一个或多个** —— 只这些件名下的账会被跑）"},
			fields:   gapRegressFields,
			endpoint: "",
			run:      cmdGapRegress,
		},
		// ★ 本枚（缺口账 `GAP-20260927-248`）：`gap reread <GAP id>…|--all` —— **判据复跑**（只读面）。
		//   病（`GAP-20260927-248` 逐字）：`solved_evidence` 是**一次性文本**（"现有 75 条非空"）而非
		//   **可重放结果** ⇒ 「已解」条目**是否已回归**机器判不出（只能靠 `gap verify` 现跑，而它自己
		//   又不设防 —— 见 `GAP-20260927-247`）。治法 = 重跑 `solved_evidence` 里的**带键标读数**
		//   （`cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`），与记下的 `rc` / `reading` 逐条对拍：
		//   rc 变 ⇒ `reading_stale`；rc 一致再看 `件:行` 引用的**行内容**（✗ 不判绝对行号 —— 行号会漂）。
		//   ★ 口径（只报告 · 不对拍不进退出码 —— 免得存量陈旧把它变成恒红）：四条读数闭集
		//   `ok` / `reading_stale` / `not_rereadable` / `skipped`；**安全铁律** = 取值命令落在**危险档**
		//   （写面）/ 含 shell 元字符 / 解析不出 / 是本命令自己（自递归）⇒ **一律不执行**（判不准就丢
		//   `not_rereadable`）。★ 只读动作 ⇒ **不挂危险档**（本面只报告 · 一字节都不写真源、不写审计）。
		//   退码：0（含有 `reading_stale`）· 2 用法错（缺目标 / `--all` 与 id 同给 / `--limit` 非非负整数 /
		//   `--json` 不给字段 / 账内没有该 id）· 8 真源读不到。实现件 = `family_gap_reread.go:cmdGapReread`。
		//   ★ 2026-10-05（缺口账 `GAP-20261005-60` 剩面 · 本枚）：新增 `--include-open` —— 把**未闭**
		//   （`state == 仍缺`）条目的 `verify_cmd` 纳入同一套判活读数（可跑 ⇒ ok；元字符 / 占位 / 自递归 /
		//   空 / 解析不出 ⇒ not_rereadable · **不执行**）；解析面在 `parseInvocation` 专用 case 上户口，
		//   dispatch 面按 usage 声明收口（与 `--top` 同形：别族给它会仍退 2，不静默吞）。
		{
			path:     []string{"gap", "reread"},
			kind:     "GapReread",
			summary:  "缺口账**判据复跑**（只读面 · 一字不写真源、不写审计、不改任何 `state`）：重跑 `solved_evidence` 里的**带键标读数**（`cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`）并与记下的 `rc` / `reading` 对拍 —— rc 变 ⇒ `reading_stale`；rc 一致再看 `件:行` 引用的**行内容**（✗ 不判绝对行号 —— 行号会漂）· 四条读数闭集 `ok` / `reading_stale` / `not_rereadable` / `skipped` · **安全铁律**：取值命令落在**危险档**（写面）/ 含 shell 元字符 / 解析不出 / 是本命令自己（自递归）⇒ **一律不执行**（判不准就记 `not_rereadable`）· 表尾声称「**只报告** —— `reading_stale` / `not_rereadable` **均不进退出码**」（免得存量陈旧读数把它变成恒红）· `--include-open` 把**未闭**条（`state == 仍缺`）的 `verify_cmd` 纳入同一套判活（可跑 ⇒ ok；元字符 / 占位 / 自递归 / 空 / 解析不出 ⇒ `not_rereadable` · **不执行**）· 缺目标 / `--all` 与 id 同给 / `--limit` 非非负整数 / `--json` 不给字段 / 账内没有该 id ⇒ 2；读不到真源 ⇒ 8",
			usage:    "zerg gap reread [<GAP id>…] [--all] [--include-open] [--limit <N>] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**一条或多条**；与 `--all` 不许同给）"},
			fields:   gapRereadFields,
			endpoint: "",
			run:      cmdGapReread,
		},
		{
			path:     []string{"gap", "export"},
			kind:     "GapExport",
			summary:  "缺口真源 → 「结转账 markdown 片段」导出面（**只打 stdout 时零写盘**：不写真源、不写审计；**`--out <目录>` 那一支会拆件落盘** ⇒ 本表项按**写面**登记 `D2`）· **只列未闭**（`已解`/`不做` 不进正文，只进件头计数）· 件头 = 仪表盘（条数/未闭/已闭/逐面计数/最老未闭天数）· 单行 ≤200 字符 · 读不到真源 ⇒ 退 8（fail-closed）",
			usage:    "zerg gap export [--out <目录>] [--json <字段>] [--dry-run | --yes]",
			fields:   gapExportFields,
			danger:   &dangerSpec{dangerD2, "落点目录（`--out`）", "往 `--out <目录>` 拆件落盘（索引件 1 + 页件 N：`缺口总账-<日期>.md` / `-<面>-pNN.md`）；真源 `zerg-cli-gaps.jsonl` 与审计 `edit_audit.jsonl` **一个字不动**（可逆：删掉新落的那些件即回到原状）", "缺口账（2026-09-27）· `ddf57491` 接上 `--out` 真写盘 ⇒ 旧表项无 `danger`，本族写命令一直被 `zerg help` 判成**安全档** ✗ · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapExport,
		},
		{
			// ★ 缺口 `GAP-20261013-20`（F30 接线 · 2026-10-15）：`gap diff` —— 缺口账**差集面**（只读对拍）。
			// 实现件 = `family_gap_diff.go:cmdGapDiff`。两份**外部件**（账 jsonl / `gap export` 人面片段）逐格对拍
			// ⇒ 三类差（仅左 · 仅右 · 同 id 不同体 · 逐条点名到 id ＋ 哪一格）。退码 **0/1/2**：
			// 0 = 无差异 · 1 = 有差异（逐条点名）· 2 = 输入取不到 / 格式不对 / 用法错（**不给结论**）。
			// ★ 与同族「真源读不到 ⇒ 8」**不同码**：本面是两份**外部件**的对拍，缺件 / 坏件属**前置/用法面**
			// （派单口径 = 2 ⇒ `exitUsage`）。用法串须写明 `--out`（本命令认它）。
			// ★ `--out <件路径>` 落**报告件**（D2 写面 · ✗ 不写真源、不写审计）：`--dry-run` ⇒ 计划件走 stdout
			// ＋ 一个字节不落（rc 仍照真跑 0/1）；缺 `--yes` ⇒ 2（fail-closed）。
			path:    []string{"gap", "diff"},
			kind:    "GapDiff",
			summary: "缺口账**差集面**（只读）：两份账 / 两份导出对拍 ⇒ 三类差（仅左 · 仅右 · 同 id 不同体），逐条点名到 id ＋ 哪一格",
			usage:   "zerg gap diff <左件> <右件> [--json <字段>] [--out <件路径> [--dry-run|--yes]]",
			arity:   "any",
			args:    []string{"左件 · 右件（各一份账 jsonl 或 `gap export` 人面片段）"},
			fields:  gapDiffFields,
			run:     cmdGapDiff,
		},
		// ★ 两条**状态写面**（2026-09-26 · 缺口账 `Q-239`：「`gap` 族只有 4 个动作 ⇒ 改态/写口径
		//   没有正门」）。照同族写面（`add` / `verify`）的式样：`--dry-run` 恒 0 / 缺 `--yes`
		//   fail-closed 2 / 审计先落盘（写不进就不写真源）/ `--by`-`--yes`-`--json` 习惯逐字沿用。
		//   实现件 = `family_gap_state.go`（本族**只这两条**用 `--evidence` / `--text` 两枚旗标）。
		{
			path:     []string{"gap", "set-state"},
			kind:     "GapSetState",
			summary:  "改一条缺口的状态（写面 · 留证据）：`--state` 只认**三值闭集**（`仍缺` / `已解` / `不做`）—— 闭集外 ⇒ 2 并逐字印闭集 · 转 `已解` 写 `solved_at` + `solved_evidence` · **销案两态对拍**（批4 第四片 · 设计稿 §11.5 · 表 `D6`）：改到 `已解` 时**新账**（缺口号内日期 ≥ `2026-09-28`）**必须**带改前/改后两条读数（`--before-read` / `--after-read` · 每条形如 `cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`）—— 缺哪一段就退 2 并**逐段点名**（改前读数 / 改后读数）· **存量账**（号内日期 < 阈值）豁免但**不静默**：表尾逐字印「本发为存量豁免（未带两态对拍）」· 改**非** `已解` 的状态 ⇒ 本闸不适用（行为逐字不变）· 同 id 同态同证据 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap set-state <GAP id> --state <仍缺|已解|不做> --evidence <一句话> [--solved-kind <实现|销案|前提不成立|部分>] [--remainder <残面描述>] [--before-cmd <命令串>] [--after-cmd <命令串>] [--judge-cmd <命令串>] [--before-read <cmd=… · rc=… · reading=…>] [--after-read <cmd=… · rc=… · reading=…>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量改态）"},
			fields:   gapSetStateFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_at` / `solved_evidence` + 审计一行（可逆：照审计那一格回写）；`--state` 闭集外 ⇒ 2（一个字节都不写）", "缺口账 `Q-239` · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetState,
		},
		// ★ 2026-10-09（缺口账 `GAP-20261008-06`/`-07` · 「改 unit / 改 want 均无正门」）：`gap set-unit`
		//   —— **改件面三键（`unit` / `module` / `unit_source`）的正门**。病源：退役后旧 `unit` 指向已退场件
		//   ⇒ 只能往门75 存量豁免表加行（**只增不删**）绕过；有本门后这类条可**归位**。实现件 =
		//   `family_gap_state.go:cmdGapSetUnit`。形制照同族五写面：走唯一收口 `gapRewriteOne`（只重写目标那一行）·
		//   `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2（D2）· `--evidence` 必填（落成 `notes` 一条注）·
		//   `--unit` 不在仓内 ⇒ 默认拒 2（须显式 `--allow-missing-unit`）。
		{
			path:     []string{"gap", "set-unit"},
			kind:     "GapSetUnit",
			summary:  "改一条缺口的**件面三键**（`unit` / `module` / `unit_source` · 写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：走族内唯一收口 `gapRewriteOne`（只重写目标那一行 · 其余行逐字节不变 · 行序不变 · 末行换行不变）· `--unit` 必填（仓内相对路径）· `--module` 缺省由 `unit` 目录前缀推（`gapModuleOfUnit`）· `--unit-source` 缺省由 `unit` 形态推（兜底档 `hand` / 仓内路径 `auto` · 闭集 `auto|hand`）· `--evidence` 必填（**改件留证** —— 落成该条 `notes` 末尾一条注）· ★ **`--unit` 不在仓内（`git ls-files` 现读对不上）⇒ 默认拒 2**（一个字节都不写），须显式 `--allow-missing-unit` 才放行（兜底档 `无件(命令面)`/`拟(x)` 不受此闸）· 同 id 同三键 ⇒ 「无变化」0 · 回执回显「改前 → 改后」三键 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap set-unit <GAP id> --unit <件路径> [--module <模块前缀>] [--unit-source <auto|hand>] [--allow-missing-unit] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapSetUnitFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源那一行的 `unit` / `module` / `unit_source` 三键 + 往 `notes` 追加一条改件留证注 + 审计一行（可逆：照审计那一格回写 · 留证注删掉即回原状；其余键名与值逐字节不变 · 行序不变 · 末行换行不变）；`--unit` 不在仓内且未带 `--allow-missing-unit` / 缺 `--unit` / 缺 `--evidence` / 收到 `--state` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261008-06`/`-07`（「改 unit 无正门」· 治法）· 同族写面先例 `gap set-state` / `gap note` / `gap assign` / `gap verify-one`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetUnit,
		},
		// ★ 2026-10-10（缺口账 `GAP-20261010-34`）：`gap judge set` —— 账内 ~99 条判据「无鉴别力 /
		//   与 want 无关」，全卡在缺这个「改判据」写面（现读只有 `set-state` 改态、`set-unit` 改件面三键，
		//   两键 `verify_cmd` / `repro_cmd` 无正门）。形制照同族 `set-unit`：走唯一收口 `gapRewriteOne` ·
		//   `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2（D2）· `--evidence` 必填；校验逐字复用 `gap add`
		//   的既有助手（样板值闸 `gapBoilerplateWhy` + 命令树解析面 `gapJudgeArgv`）。实现件 = `family_gap.go:cmdGapJudgeSet`。
		{
			path:     []string{"gap", "judge", "set"},
			kind:     "GapJudgeSet",
			summary:  "改一条缺口的**判据两键**（`verify_cmd` / `repro_cmd` · 写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：走族内唯一收口 `gapRewriteOne`（只重写目标那一行 · 其余行逐字节不变 · 行序不变 · 末行换行不变）· `--verify-cmd` 必填（命令树里真有的命令 —— 逐字复用 `gap add` 的样板值闸 `gapBoilerplateWhy` + 解析面 `gapJudgeArgv`：通用内省型 / 解析不到 / 写面 / 自递归 ⇒ 拒 2）· `--repro-cmd` 缺省保留旧值（同 `gap add` 口径只过样板值闸）· `--evidence` 必填（改判据留证 —— 落成该条 `notes` 末尾一条注）· 同 id 同两键 ⇒ 「无变化」0 · 回执回显「改前 → 改后」两键 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap judge set <GAP id> --verify-cmd <命令串> [--repro-cmd <命令串>] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapJudgeSetFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `verify_cmd` / `repro_cmd` 两键 + 往 `notes` 追加一条改判据留证注 + 审计一行（可逆：照审计那一格回写 · 留证注删掉即回原状；其余键名与值逐字节不变 · 行序不变 · 末行换行不变）；`--verify-cmd` 是样板值 / 解析不到命令树 / 缺 `--verify-cmd` / 缺 `--evidence` / 收到 `--state` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261010-34`（账内判据无鉴别力 · 治法的一半）· 同族写面先例 `gap set-state` / `gap set-unit`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapJudgeSet,
		},
		// ★ 批3 第五片（2026-09-28）：`gap bulk set-state` —— **族级批量改态**（一次改 N 条）。
		//   病源 = 缺口账 `GAP-20260926-97`（逐字：想要 `zerg gap-batch-state` · 「一次只能改一个 GAP id
		//   （结账/收版时逐条改态成本高）」）；本片 = 该条点名的治法，**不另开写路**：逐条仍走
		//   `family_gap_state.go:gapRewriteOne` 那一个收口（审计先落盘 → 整件重写 → 写后读回对拍）。
		//   四条硬纪律：① 全量预检先行（任一不合规 ⇒ 整批拒 2 + 逐条点名 + **一字不写**）
		//   ② 逐条改态、逐条写**自己那一行**的 evidence ③ 任一条写失败 ⇒ 当场停手 + 如实报「已改 m/未改 n」
		//   ④ `--dry-run` 恒 0 / 缺 `--yes` fail-closed 2（D2）。实现件 = `family_gap_state.go:cmdGapBulkSetState`。
		{
			path:     []string{"gap", "bulk", "set-state"},
			kind:     "GapBulkSetState",
			summary:  "族级**批量改态**（一次改 N 条 · 写面）：第一位置参 = 清单件（JSONL · 每行至少 `id` 与 `evidence` 两键）· `--state` 闭集同单条（`仍缺` / `已解` / `不做`）· **全量预检先行**：任一条缺 `id`/`evidence`、或 `id` 不在账、或该条当前 `state` 不可改（必须是闭集三值之一 —— `已派`/`已立项`/`回归` 不归本面）⇒ **整批拒**（退 2 · 逐条点名到行 · **一字不写**）· 全过后才真写：逐条改态、逐条写自己的 `evidence`、审计逐条一行（走现有单条改态路径 `gapRewriteOne`，不另开写路）· 任一条写失败 ⇒ **当场停手**并如实报「已改 m 条 / 未改 n 条」· 表尾回显总数 / 逐条改前 → 改后 / 失败条数 · `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2",
			usage:    "zerg gap bulk set-state <清单件.jsonl> --state <仍缺|已解|不做> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"清单件路径（**恰好一个** · JSONL：每行至少 `id` 与 `evidence` 两键）"},
			fields:   gapBulkSetStateFields,
			danger:   &dangerSpec{dangerD2, "整批缺口 id（清单件点名的那几条）", "按清单逐条改真源里的 `state` / `solved_at` / `solved_evidence`（逐条写自己的 evidence）+ 审计**逐条**一行（可逆：照审计那一格逐条回写）；任一条不合规 ⇒ 2 且**一个字节都不写**（预检在真写之前）", "批3 第五片（缺口账与自进化 · 2026-09-28）· 缺口账 `GAP-20260926-97`（本片治法逐字）· 同族写面先例 `gap set-state` / `gap assign`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapBulkSetState,
		},
		{
			path:     []string{"gap", "note"},
			kind:     "GapNote",
			summary:  "给一条缺口追加一条**口径/上下文注**（写面）：落真源那一行的 `notes` 数组（`<时刻> · <谁>：<文本>`）· **不改 `state`**（形状里根本没有 `--state` ⇒ 收到即拒 2）· 同 by 同 text 已在位 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）· `--retract <n|指纹>` 给已落的注打**作废标记**（`notes_void` 追加 · **禁真删**：原文逐字节留在 `notes` 里 · 读面默认不显示作废项 · `--json void_notes` 带原文出来）",
			usage:    "zerg gap note <GAP id> [--text <一句话> | --retract <n|指纹> | --supersede <n>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条**）"},
			fields:   gapNoteFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "往真源那一行的 `notes` 数组追加一条（可逆：删那一格的那一条）+ 审计一行；`state` 逐字不动（改前改后现算对拍）；`--retract <n|指纹>` 只**追加**一条作废标记（`notes_void` · 可逆：删标记那一条）—— **禁真删任何注**，读面按标记过滤", "缺口账 `Q-239` · 同族写面先例同 `gap set-state` · 「注不改态」是本条与 `set-state` 的分工线", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapNote,
		},
		// ★ 批3 第一片（2026-09-28）：`gap assign`（作业单 + 派单登记）—— 让状态机的 `已派` **首次真启用**
		//   （现读 0 条）。设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.3（派单登记 = 写 `已派` +
		//   `notes` 记卵号与回执绝对路径）+ §4 判据 2（自派：`已派` 条数 == 在飞卵数 · 作业单必须带占用状态）
		//   + §8 `M-32`（「同改文件只许一路」⇒ `assign` 前先查占用）。
		//   ★ 写面**只走现有改态路径**（`family_gap_state.go:gapRewriteOne` 那一个收口 · 不另开写路）：
		//   `state` → `已派` 与 `notes` 追加登记**同一次**重写、一条审计行。六栏缺一栏 ⇒ 2 并点名缺哪栏。
		//   实现件 = `family_gap_state.go:cmdGapAssign`（写面）/ `family_gap.go:cmdGapAssignLs`（只读面）。
		{
			path:     []string{"gap", "assign"},
			kind:     "GapAssign",
			summary:  "派单登记（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：把该条 `state` 改 **`已派`**（走现有改态路径 `gapRewriteOne` · 不另开写路）并在 `notes` 末尾追加**一行机读登记**（`派单登记 egg=… · receipt=… · allow=… · forbid=… · occupies=… · criterion=… · deadline=…`）· **作业单六栏缺栏拒发**（① `--allow` 允许面 ② `--forbid` 禁碰面 ③ `--occupies` 占用件 ④ `--criterion` 出口判据 ⑤ `--deadline` 时限 ⑥ 该条全文 —— 缺一栏 ⇒ 2 并点名缺哪栏）· 预检「可派状态」= `仍缺` / `已立项`（非此 ⇒ 2 并点名当前 state）· `M-32` 占用预检（同件已被另一 `已派` 占住 ⇒ 2）· 同卵号登记已在位 ⇒ 「无变化」0",
			usage:    "zerg gap assign <GAP id> --egg <卵号> --allow <件清单> --forbid <面> --occupies <件|无> --criterion <出口判据> --deadline <绝对日期> [--receipt <回执绝对路径>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 一次只派一条 · 本动作不做批量）"},
			fields:   gapAssignFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `已派`）+ 往 `notes` 追加一行派单登记 + 审计一行（可逆：照审计那一格回写 · 登记那一行删掉即回原状）；六栏缺一栏 / 非可派状态 ⇒ 2（一个字节都不写）", "批3 第一片（缺口账与自进化 · 2026-09-28）· 设计稿 §11.3 + §4 判据 2 + §8 `M-32` · 同族写面先例 `gap set-state` / `gap note`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapAssign,
		},
		{
			path:     []string{"gap", "assign", "ls"},
			kind:     "GapAssignLs",
			summary:  "派单**读面**（只读 · 一字不写真源、不写审计）：列 `已派` 条与它们的作业单登记（六栏 + 卵号 + 回执 + 登记时刻）· `--egg <卵号>` 收窄 · `--aging`（按登记时刻与时限算超时多少 · **降序**最久的在最前 · 时限算不出的单列「时限不明」**不当零**）· `--stale`（只列超时限未收的派单 · 无一条 ⇒ rc 0 且明写「无超时派单」）· 表尾**固定**回显对账口径行（`已派 N 条 —— 请与平台在飞清单逐条比对` · N 现读现算），有超时条再点名它们的缺口号 + 卵号 + 超时时长（**只报告不自动断言** —— 在飞不在本 CLI 真源内）· `--json` 走同族信封（`meta.total` / `meta.hits` / `query` / `query_ts` / `ledger_sha16` / `assign_total` / `overdue_count` / `reconcile` + 截断自报）· 零命中 ⇒ 1（`已派` 0 条不是绿）· 真源读不到 ⇒ 8。判据 2「自派」的计数落点",
			usage:    "zerg gap assign ls [--egg <卵号>] [--aging] [--stale] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：收窄走 `--egg`）"},
			fields:   gapAssignListFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapAssignLs,
		},
		// ★ 三段链中段（2026-10-05 · 缺口账 `GAP-20261005-61` · 设计稿 `~/.zerg/state/设计-gap立项正门-20261005.md` §2 乙案 / §3 / §7 U1）：
		//   `仍缺 ──project──▶ 已立项（立项待拍） ──assign──▶ 已派（派单）`。甲案「扩 `set-state` / `bulk`
		//   的 `gapStateSettable` 闭集」已否 ⇒ 本面**只增两条动作**，`set-state` / `bulk` 一字不动 ✗。
		//   实现件 = `family_gap_state.go:cmdGapProject` / `cmdGapUnproject`（共用 `gapProjectRun`）。
		{
			path:     []string{"gap", "project"},
			kind:     "GapProject",
			summary:  "立项待拍（写面 · 三段链中段）：源 {`仍缺`, `不做`} → 目标**恒** `已立项`（**不收 `--state`** —— 目标态由动作名唯一确定，甲案「扩 set-state 闭集」已否）· `--evidence` **必填**（改态留证）· 转入 `已立项` **不取 `solved_at`**（非终态）· 同 id 同态同证据 ⇒ 「无变化」0（不写）· `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap project <GAP id> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapProjectFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `已立项`）+ `solved_evidence`（**不碰** `solved_at` / `solved_kind`）+ 审计一行（可逆：照审计那一格回写）；源态不在源闭集 / 收到 `--state` / 缺 `--evidence` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261005-61` · 设计稿 `设计-gap立项正门-20261005.md` §2 乙案 / §3 三段链 / §7 U1 · 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapProject,
		},
		{
			path:     []string{"gap", "unproject"},
			kind:     "GapUnproject",
			summary:  "撤立项（写面 · 三段链回退面 §7 U1）：源 {`已立项`} → 目标**恒** `仍缺`（**不收 `--state`**）· `--evidence` **必填** · 非 `已立项` 条 ⇒ 按源闭集拒执 2（逐字印当前 state + 源闭集）· 同 id 同态同证据 ⇒ 「无变化」0 · `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap unproject <GAP id> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapProjectFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `仍缺`）+ `solved_evidence`（**不碰** `solved_at` / `solved_kind`）+ 审计一行（可逆：照审计那一格回写）；源态非 `已立项` / 收到 `--state` / 缺 `--evidence` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261005-61` · 设计稿 `设计-gap立项正门-20261005.md` §2 乙案 / §7 U1（三段链回退面）· 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapUnproject,
		},
		// ★ `gap backfill-unit` —— **存量回填三键**（2026-09-28 · 批1 第二片）：第一片只把 `unit` / `module` /
		//   `unit_source` 落进**新落行**（`gap add`），存量 891 条账里没有 ⇒ 件面 / 模块分类在存量面上**无数据**。
		//   本面：`--dry-run` 只印「可抽到 N + 落兜底 M」（**N + M == 账内总条数**）+ 前 5 条预览（只读 · 恒 0）；
		//   `--yes` 逐条**只加三键**（其余键名与值**逐字节不变** · 行序不变 · 末行换行不变 · 已带三键**跳过** ⇒ 幂等）。
		//   抽取值形态逐字复用第一片的 `gapUnitExtract`；退码照现有表（0 成事 / 2 缺 `--yes` / 8 读不到或写不进）。
		//   实现件 = `family_gap.go:cmdGapBackfillUnit`。
		{
			path:     []string{"gap", "backfill-unit"},
			kind:     "GapBackfillUnit",
			summary:  "给**存量**缺口账回填件面三键（`unit` / `module` / `unit_source`）：`--dry-run` 恒 0 · 只读 · 零副作用 —— 印「**可抽到** N + **落兜底** M」（**N + M == 账内总条数**）+ 前 5 条待改预览（id + unit/module）· `--yes` 逐条**只加三键**（其余键名与值**逐字节不变** · 行序不变 · 末行换行不变）· 已带三键**跳过** ⇒ **幂等**（再跑 ⇒ 「新增 0 行改动」）· 抽取值形态逐字复用 `gapUnitExtract`（甲档全等 / 乙档路径子串 / 兜底手写 `拟(x)` 或 `无件(命令面)` + module `无件` + `unit_source=hand`）· 审计进 `edit_audit.jsonl`（写不进 ⇒ 真源一个字节不改）",
			usage:    "zerg gap backfill-unit [--dry-run | --yes] [--force-recompute] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = **整本账** · 本片不收窄）"},
			fields:   gapBackfillFields,
			danger:   &dangerSpec{dangerD2, "整本缺口账（`<状态目录>/zerg-cli-gaps.jsonl`）", "逐条**只加** `unit` / `module` / `unit_source` 三键（其余键名与值逐字节不变 · 行序不变 · 末行换行不变）+ 审计一行（可逆：删掉新加的那三键即回原状；`--dry-run` 那一态零副作用）", "批1 第二片（存量 891 条回填 · 2026-09-28）· 同族写面先例 `gap add` / `gap verify` / `gap set-state` / `gap note` · `设计-命令面-gap族-v1.0-20260923.md §三 三态纪律` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapBackfillUnit,
		},
		// ★ 批2 第一片（2-1 / 2-2 / 2-4）：候选池独立件 —— `gap idea` 两条（**新件** `<状态目录>/zerg-cli-gap-candidates.jsonl`）。
		//   设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.3（独立文件 · 不进主账 · 七格字段 · ttl_days=7）
		//   + §9.1 第 1/2 条（立候选池 · `source` 四值 `guard|gate|egg|human`）+ 施工清单 §3 的 2-1/2-2/2-4。
		//   ★ **`gap add` 仍是唯一入账**：这两条一个字都不写 `zerg-cli-gaps.jsonl`（判据：`idea add` 后账行数不变）。
		//   实现件 = `family_gap.go:cmdGapIdeaLs` / `cmdGapIdeaAdd`。
		{
			path:     []string{"gap", "idea", "ls"},
			kind:     "GapIdeaLs",
			summary:  "候选池**最小读面**（只读 · 不写池、不写真源、不写审计）：列池内候选（`cid` / `source` / `status` / 落池时刻 / 症状）· `--source` 收窄（四值闭集 `guard|gate|egg|human`，闭集外 ⇒ 2 并点名闭集）· `--json` 走同族信封（`meta.total` / `meta.hits` / `query_ts` / `pool_sha16` + 截断自报）· 池件不在盘上 ⇒ 8（「读不到」不当绿）· 零命中 ⇒ 1",
			usage:    "zerg gap idea ls [--source <guard|gate|egg|human>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：收窄走 `--source`）"},
			fields:   gapIdeaListFields,
			endpoint: "",
			run:      cmdGapIdeaLs,
		},
		{
			path:     []string{"gap", "idea", "add"},
			kind:     "GapIdeaAdd",
			summary:  "人随手记 ⇒ **落候选池**（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：落一行 `<状态目录>/zerg-cli-gap-candidates.jsonl`（`cid`/`source`/`raw`/`created_at`/`ttl_days=7`/`status=pending`/`fp`）· **真源一字不动**（`gap add` 仍是唯一入账）· 同 source 同 fp ⇒ 幂等 0 · 顺带做**七日归档**（落池超七天的条目搬入同目录 `…archive-<日期>.jsonl` · 只搬超期条）· `--source` 缺省 `human`，闭集外 ⇒ 2 并点名闭集",
			usage:    "zerg gap idea add --symptom <一句> [--source <guard|gate|egg|human>] [--unit <件路径>] [--gate-step <步>] [--log-path <件>] [--rc <N>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapIdeaFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）", "往**候选池**追加一行（可逆：删那一行）+ 顺带把落池超七天的条目搬进同目录归档件；**真源 `zerg-cli-gaps.jsonl` 与审计一个字节都不动**（本面不碰真源 —— 入账仍走 `gap add`）", "设计-缺口账与自进化-v2.0-20260928.md §11.3 + §9.1 第 1/2 条 · 施工清单 `任务清单-缺口账自进化-施工-20260928.md` §3 的 2-1/2-4 · 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaAdd,
		},
		// ★ 批2 第二片（2-3 出口面 · 2026-09-28）：候选池三条出口 + 一条读数（设计稿 §11.3「出口」+ §4 判据 1）。
		//   ★ **`promote` 复用 `gap add` 的既有校验（不许绕）**（§11.3 黑体）⇒ 与 `cmdGapAdd` 共用
		//   `family_gap.go:gapAddApply`（六必填 / impact 闭集 / prio 缺省 / 人面禁写 state / 判据命令树解析 /
		//   同 fp 幂等与 `14` / 审计先落盘 / 读回对拍）—— 实现件 = `cmdGapIdeaPromote`。
		{
			path:     []string{"gap", "idea", "show"},
			kind:     "GapIdeaShow",
			summary:  "候选池**单条全文**（只读 · 一字不截）：cid / 来源 / 状态 / 指纹 / 落池时刻 / 存活 / 点名件 / 症状 / **提升后的 `gap_id`**（`pool` 与 `ledger` 两边点名）· 池内没有这个 cid ⇒ 2（同族 `gap show` 口径）· 池件不在盘 ⇒ 8",
			usage:    "zerg gap idea show <cid> [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条** —— 本面只取一条）"},
			fields:   gapIdeaShowFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaShow,
		},
		{
			path:     []string{"gap", "idea", "promote"},
			kind:     "GapIdeaPromote",
			summary:  "池内一条候选 ⇒ **正式账**（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：**复用 `gap add` 的既有校验与落账路径**（`gapAddApply` —— 六必填 / `impact` 六值闭集 / 人面禁写 `state` / 判据命令树解析 / 同 fp 幂等与 `14` / 审计先落盘 / 读回对拍，一条不绕 · 设计稿 §11.3 黑体）· 只有 `status=pending` 可提升（其余 ⇒ 2 并点名当前 status）· 提升后池行**不删**：`status=promoted` + 记 `gap_id`（先落账、后改池行）",
			usage:    "zerg gap idea promote <cid> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] [--state <仍缺|已立项|不做>] --repro-cmd <命令> --verify-cmd <命令> [--note <注>|--notes <注>|--evidence <注>…] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条**）；其余六必填与本族 `gap add` 逐字同款（症状取自池件那一格 `raw`）"},
			fields:   gapIdeaPromoteFields,
			danger:   &dangerSpec{dangerD2, "候选 id（池内一条）+ 缺口 fp", "走 `gap add` 那条路往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删），并把池件那一行置 `status=promoted` + 记 `gap_id`（池行**不删**）；池件写不进 ⇒ 退 8 并如实报「正式账已落」", "设计-缺口账与自进化-v2.0-20260928.md §11.3（`promote` **复用 `gap add` 的既有校验（不许绕）**）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaPromote,
		},
		{
			path:     []string{"gap", "idea", "discard"},
			kind:     "GapIdeaDiscard",
			summary:  "池内一条候选 ⇒ `status=expired`（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：**不是删件** —— 池行不删、其余行逐字节不动、真源一个字节不碰 · 只有 `status=pending` 可作废（其余 ⇒ 2 并点名当前 status）",
			usage:    "zerg gap idea discard <cid> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条**）"},
			fields:   gapIdeaDiscardFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）点名那一行", "把池内该行 `status` 置 `expired`（可逆：照 `gap idea show` 的读数改回 `pending`）；池行**不删**、真源 `zerg-cli-gaps.jsonl` 与审计**一个字节不动**", "设计-缺口账与自进化-v2.0-20260928.md §11.3（出口面 · `status` 四值 `pending|promoted|expired|merged`）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add` / `gap idea add`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaDiscard,
		},
		{
			path:     []string{"gap", "idea", "stats"},
			kind:     "GapIdeaStats",
			summary:  "（设计稿 §4 判据 1）读数面（**只报告、不进退码**）：池内各 `status` 计数 + **来源分布**（`source` 四值）+ **候选→入账转换率**（`promoted / (promoted+expired+pending)`，分母逐字写出）· 只读（不写池、不写真源、不写审计）· 池件不在盘 ⇒ 8",
			usage:    "zerg gap idea stats [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = 整池）"},
			fields:   gapIdeaStatsFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaStats,
		},
		// ★ 批2 第三片（2-3 守卫腿 · 2026-10-13）：守卫拦截**回收面**。口径已拍（缺口
		//   `GAP-20260928-174`）：**落点定 control 层**（`core/internal/control/guardlog.go` 记事件）
		//   · **回收面走 `gap idea sweep --source guard`**（拦截点不内联写池）。实现件 = `cmdGapIdeaSweep`。
		{
			path:     []string{"gap", "idea", "sweep"},
			kind:     "GapIdeaSweep",
			summary:  "把**守卫拦截事件**（控制层 `<状态目录>/zerg-cli-gap-guard-events.jsonl`）回收成候选池条目（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：逐事件造一行 `source=guard` / `status=pending`（幂等靠池内 `fp` —— 同 source 同 fp ⇒ 不新增）· `--source` 只认 `guard`（闭集外 ⇒ 2 并点名；gate 腿由门禁脚本直写入池 · egg/human 腿走 `gap idea add`）· 拦截点**不内联写池**（落点见 `core/internal/control/guardlog.go`）· **真源一字不动** · 顺带做七日归档 · 没事件 ⇒ 1（零命中）· 池件写不进 ⇒ 8",
			usage:    "zerg gap idea sweep [--source <guard>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapIdeaSweepFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）", "往**候选池**追加「池里还没有的」守卫拦截候选（可逆：删那些行）+ 顺带把落池超七天的条目搬进同目录归档件；**真源 `zerg-cli-gaps.jsonl` 与守卫事件件一个字节都不动**（本面只读事件件；入账仍走 `gap add`）", "`设计-缺口账与自进化-v2.0-20260928.md` §11.3「三类入口」之二（守卫拦）· 缺口 `GAP-20260928-174`（已拍：落点定 control 层 · 回收面走本命令）· 同族写面先例 `gap idea add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaSweep,
		},
		// ---- 度量与排序面（组1 序12 · `承接自-v2.5.11/承接-度量与排序面-20260921.md:40-42` · 2026-09-24）----
		// 与 `impact`（变更影响面）**配对用、不合并成一条**：前者回答「改这一处会牵动谁」（别改坏），
		// 本命令回答「**该改哪**」（把四类读数归一成一个可比排序）。只读 ⇒ 不写 `danger`
		// （走默认「只读」幂等档）、不改 `emitEnvelope`、不落审计、不写缓存。
		{
			path:     []string{"metrics"},
			kind:     "Metrics",
			summary:  "「该改哪」那把尺：把四类读数（重复 / 未用符号 / 覆盖率 / 门禁）**归一成一个可比排序**（只读 · 空输入 ⇒ 不给结论）",
			usage:    "zerg metrics [--input <读数档>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：读数档走 `--input`）"},
			fields:   metricsFields,
			endpoint: "",
			run:      cmdMetrics,
		},
		// ---- CA 全量追踪（批 B1 · 2026-10-20）：独立入口 `core/cmd/zerg-trace` 归入命令面 ----
		// 旗标**逐字**保留源件那五枚（`--live` / `--json` / `--step` / `--tokens` / `--errors`），
		// `<logdir>` 必填。**双轨期**：`core/cmd/zerg-trace` 二进制不动（本族只加一条入口）。
		// 只读（`os.Open` / `os.Stat` 两件）⇒ 不写 `danger`（走默认「只读」幂等档）。
		{
			path:     []string{"trace"},
			kind:     "Trace",
			summary:  "CA 全量追踪（`events.jsonl` 时间线 · 每轮明细 · token 分析 · 错误清单 · 机器指标联动；只读）",
			usage:    "zerg trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]",
			arity:    "any",
			args:     []string{"<logdir>（必填：含 `events.jsonl` 的日志目录；`sysmetrics.jsonl` 有则联动）"},
			fields:   traceFields,
			endpoint: "",
			run:      cmdTrace,
		},
		// ---- 脑确认复查（批 B3 · 2026-10-20）：独立入口 `core/cmd/zerg-review` 归入命令面 ----
		// 位置参数与旗标**逐字**保留源件（`<workdir> <issue-id|list> [approve|reject]` +
		// 一枚 `-repo` ⇒ 命令面写作 `--repo`），只把 Go `flag` 的单横线写成双横线。
		// **双轨期**：`core/cmd/zerg-review` 二进制一字不动（本族只加一条入口）。
		// 实现在 `family_review.go`（件头逐条写清与源件的六处差异）。
		// 写面（真 merge / 真丢分支 / 改 issue 状态）但**不进危险表**，理由与 `sched run` 那一格同：
		// 它是操作者对**自己指定的**工作目录/仓库当场做的确认动作（源件本来就这么用），宿主是本机、
		// 目标是命令行给的那个路径，✗ 无远端效应；再加 `--yes`/`--dry-run` 会改源件行为 ⇒ 双轨期不加。
		{
			path:     []string{"review"},
			kind:     "Review",
			summary:  "脑确认复查（C4 流程：`approve` 把分支 merge 回 main 并置 issue done · `reject` 丢分支并置 retry · `list` 列在等确认的单）",
			usage:    "zerg review <workdir> <issue-id|list> [approve|reject] [--repo <路径>]",
			arity:    "any",
			args:     []string{"<workdir>", "<issue-id|list>", "[approve|reject]"},
			endpoint: "",
			run:      cmdReview,
		},
		// ---- L1 骨架（2026-10-08）：新族 `sandbox` / `wall` / `fleet` / `mcp` —— 只立族、不填业务 ----
		// 依据：`~/.zerg/state/AlltoGo接线总图-20261008.md` §②「立族清单 = 新立 4 族」·
		// `待拍清单-AlltoGo-20261008.md` P2（方案 A 立新族）/ P3（`zerg wall` 撞名 ⇒ 帮助主题改名腾出 `wall`）
		// / P4（`setup-zerg.sh` 归 `zerg svc setup` —— 另一族 · 不在本单四族内）。
		// 口径（本单只立骨架）：每族 **≥1 条只报告/占位动作**，占位动作**一律 fail-closed** ——
		// 未实现 ⇒ 退码 `2` + 逐字报「未接」（✗ 绝不静默返回 0）；实现体 = `ui_start.go` 的
		// `cmdFamilyNotWired`（复用一件**已登记**件作宿主 ⇒ 门78 面④ 齐 · 见该函数头）。X1 层级标记
		// = `host`（命令面恒在茧壁外 · 门⑥ `check-wall-layer` 闭集）。各路接线时把 `run:` 指向各自
		// `family_<族>.go` 真实现、并把 `refuses` 随实情撤/留 —— 本条即该族的第一条动作。
		// ---- L1 批1（2026-10-08）：`zerg sandbox` 真动作分派（壳转账 · 只转发、不翻译）----
		// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批1（沙箱探针 ＋ 两态门禁）。
		// 底层唯一实现 = `scripts/sandbox-probes/*.py|*.sh`（真跑子进程 / 沙箱语义 ⇒ 非纯 Go 可化）；
		// 命令面只做入口 ＋ 转发（✗ 不重写脚本任何一段）。未接的夹具动作（`sandbox test`）仍 fail-closed
		// 退 2（批3）。实现体宿主 = `ui_start.go`（门78 面④ 要求实现件已在 `scripts/公开标记.tsv` ⇒ 复用
		// 已登记件作宿主 · 本单铁律禁碰该表 —— 同 `cmdFamilyNotWired` 那条理由）。
		{
			path:        []string{"sandbox", "probe"},
			layer:       "host",
			summary:     "虫卵探针（`conn_jit` / `jitprobe` / `lx2-linux` / `miniprobe` · 底层只调 scripts/sandbox-probes/<名>.py —— 命令面不重写逻辑）",
			usage:       "zerg sandbox probe <conn_jit|jitprobe|lx2-linux|miniprobe> [<透传旗标…>]",
			arity:       "any",
			args:        []string{"探针名（逐字 · 见用法串的闭集）", "旗标与退码逐字来自 scripts/sandbox-probes/<名>.py（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxProbe,
		},
		{
			path:        []string{"sandbox", "verify"},
			layer:       "host",
			summary:     "两态可验门禁（判据7 · 断言A空间内通信可用 ＋ 断言B出网被拦 · 底层只调 scripts/sandbox-probes/verify-two-states.py —— 命令面不重写逻辑）",
			usage:       "zerg sandbox verify [--wall <茧壁二进制> | --self-test]",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/sandbox-probes/verify-two-states.py（原样透传 · 含 --wall / --self-test）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxVerify,
		},
		{
			path:        []string{"sandbox", "negctl"},
			layer:       "host",
			summary:     "两态门禁的负例活体控制（四条必须 rc=2 · 底层只调 scripts/sandbox-probes/negctl-two-states.sh —— 命令面不重写逻辑）",
			usage:       "zerg sandbox negctl",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/sandbox-probes/negctl-two-states.sh（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxNegctl,
		},
		{
			path:        []string{"sandbox", "test"},
			layer:       "host",
			summary:     "虫卵验收夹具（子动作分派：`relay`/`fleet`/`setup`/`update` 四子动作**均已接线** —— 各自底层只调 scripts/sandbox/test-<名>-sandbox.sh，命令面只做入口＋转发、不重写逻辑）",
			usage:       "zerg sandbox test <relay|fleet|setup|update>",
			arity:       "any",
			passthrough: true,
			run:         cmdSandboxTest,
		},
		// ---- L1 批2（2026-10-08）：`zerg wall` 真动作分派（茧壁证据 2 · 壳转账）＋ 茧壁变异 2（危险档 · 未接）----
		// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批2（茧壁证据 ×2 ＋ 茧壁变异 ×2）·
		// `待拍清单-AlltoGo-20261008.md` P3（`wall` 撞名 ⇒ 帮助主题已改名 `enclosure` 腾出本族名）·
		// P5（两个变异器**真改仓内件** ⇒ 入危险档 D2/写仓 ＋ 还原台账纪律）。
		// 形态（两条路，都照仓里既有先例、不自己创式样）：
		//   · `wall evidence <two-states|macos>` = **壳转账**（底层唯一实现 = `scripts/wall/<名>.py` ·
		//     真跑 cargo+verify ⇒ 非纯 Go 可化）：命令面只做入口 ＋ 转发，✗ 不重写脚本任何一段。
		//     实现体宿主 = `ui_start.go`（门78 面④ 要求实现件已在 `scripts/公开标记.tsv` ⇒ 复用**已登记**件
		//     作宿主 · 本单铁律禁碰该表 —— 同 `cmdSandboxProbe` 那条理由）。
		//   · `wall mutate <bridge|macos>` = **危险档 D2（写仓）· 本版已开放**：两个变异器**真改仓内件**
		//     （改坏 Rust/Go ⇒ 门必须红 ⇒ 还原）⇒ 依 P5 入危险档，✗ **不得默认裸跑** —— `run: cmdWallMutate`
		//     （`wall_mutate.go` · **壳转账**：`--dry-run` 出计划件退 0 / 缺 --yes fail-closed 退 2 / 确认档齐
		//     转发 `scripts/wall/{wall-bridge,wall-macos}-mutate.py` 真跑 · 脚本退码原样转出）。★ **还原台账
		//     纪律**（跑前落备份 · 跑后逐件核 sha · 回执报「变异期间碰过哪些件、怎么复原」）随
		//     `dangerSpec.Effect` 逐字登记，由脚本内 `scripts/docs/inplace-window.py` 落地。
		{
			path:        []string{"wall", "evidence"},
			kind:        "WallEvidence",
			layer:       "host",
			summary:     "茧壁活体回执生成器（two-states / macos · 底层只调 scripts/wall/<名>.py —— 命令面不重写逻辑 · 真跑 cargo+verify）",
			usage:       "zerg wall evidence <two-states|macos> [<透传旗标…>]",
			arity:       "any",
			args:        []string{"子动作名（逐字 · 见用法串的闭集）", "旗标与退码逐字来自 scripts/wall/<名>.py（原样透传 · 0 全绿 / 1 断言红 / 2 硬失败）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdWallEvidence,
		},
		{
			path:    []string{"wall", "mutate"},
			kind:    "WallMutate",
			layer:   "host",
			danger:  &dangerSpec{dangerD2, "变异目标（bridge / macos）", "真改仓内件做变异验证（改坏 wall/src/**.rs 与迁移桥两侧 ⇒ 门必须红 ⇒ 还原）；**还原台账纪律**：跑前落备份（`scripts/docs/inplace-window.py` arm）、跑后逐件核 sha、回执报「变异期间碰过哪些件、怎么复原」", "待拍清单 P5（茧壁变异器入危险档 D2/写仓）· 同族先例 gate check（D3）· 两个变异器件头（inplace-window 复原口径）", false, dangerScopeRepo},
			opened:  true,
			summary: "茧壁变异验证（bridge / macos · **危险档 D2/写仓 · 本版已开放**：真改仓内件 ⇒ 缺 --yes fail-closed 退 2；`--dry-run` 只出计划件）",
			usage:   "zerg wall mutate <bridge|macos> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"子动作名（逐字 · bridge / macos）", "确认档：`--dry-run` 出计划件 / `--yes` 才执行（缺 ⇒ fail-closed 退 2）"},
			run:     cmdWallMutate,
		},
		// ── 命令树登记批（2026-10-09）：`zerg fleet logs` 入口（原占位只到 `fleet ls`）──────
		// 真身 = `fleet_logs.go` 的 `fleetLogsGateMain`（`gateway/scripts/fleet_logs.py` 的 Go 真搬 ·
		// 四子命令 query/trace/errors/stats · 打主控 :8580 只读端点 · 已 inproc 登记
		// `gate_inproc_table.go:166` 键 `fleet-logs`）。入口 `cmdFleetLogs`（宿主 `ui_start.go` ·
		// 门78 面④ 同 `mcp serve` 理由）**只转发**：✗ 不重写它任何一段 · 退码原样转出
		// （0 成功 / 1 调用或 HTTP 失败 / 2 用法错）。
		{
			path:        []string{"fleet", "logs"},
			kind:        "FleetLogs",
			layer:       "host",
			summary:     "机群日志查询（query/trace/errors/stats · 打主控 :8580 只读端点 · 真身 `fleet_logs.go:fleetLogsGateMain`；入口只转发）",
			usage:       "zerg fleet logs [--controller <地址>] [--token-file <件>] <query|trace|errors|stats> [旗标…]",
			arity:       "any",
			args:        []string{"子命令与旗标逐字来自 gateway/scripts/fleet_logs.py（原样透传 · 0 成功 / 1 调用或 HTTP 失败 / 2 用法错）"},
			passthrough: true,
			run:         cmdFleetLogs,
		},
		// ---- E1（缺失项 `GAP-20260927-172` 之正解 · 稿 E v2 §2 `F1′`）：`zerg fleet render` ----
		// 真源（名册 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`）⇒ 生成投影（`gateway/fleet.yaml` 的
		// `fleet:` 段与 `updates.nodes`）—— 消「两处手抄 ⇒ 漂移」。真身 = `family_fleet.go:cmdFleetRender`。
		// 三档：`--preview-only`（缺省·只打印·零写盘）/ `--emit`＋`--yes`（真写·D2 = 同目录临时件＋rename
		// 原子写＋读回校＋不过即回滚 · 先例 `zerg model add`）/ `--expect-no-changes`（只读断言·现读≠生成 ⇒ 非 0）。
		// ★ 只碰投影件的两段**数据行** ⇒ 不碰真源、不碰 `models:` 与注释行（门④判据 B 的投影源声明）。
		{
			path:     []string{"fleet", "render"},
			kind:     "FleetRender",
			layer:    "host",
			summary:  "由**真源**（名册 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`）生成**投影**（`gateway/fleet.yaml` 的 `fleet:`/`updates.nodes`）—— 消「两处手抄 ⇒ 漂移」（缺失项 `GAP-20260927-172`）；`--preview-only`（缺省·只打印·零写盘）/ `--emit`＋`--yes`（真写·D2 原子写+读回校+回滚，只碰两段数据行）/ `--expect-no-changes`（只读断言·现读≠生成 ⇒ 非 0）",
			usage:    "zerg fleet render [--preview-only] [--expect-no-changes] [--emit] [--roster <名册件>] [--path <投影件>] [--context <档名>] [--json <字段>] [--dry-run | --yes]",
			arity:    "any",
			args:     []string{"（无位置参数）"},
			fields:   []string{"action", "roster", "fleet", "nodes", "drift", "changed", "result"},
			danger:   &dangerSpec{dangerD2, "投影件（`gateway/fleet.yaml` 的 `fleet:`/`updates.nodes` 两段数据行）", "`--emit`＋`--yes` 时写投影件 `gateway/fleet.yaml` 的两段数据行（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；✗ 不改真源 `~/.zerg/contexts/…`、✗ 不碰 `models:`/注释行）；可逆：由名册重渲即回名册态", "缺失项 `GAP-20260927-172` · 稿 E v2 §2 `F1′` · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdFleetRender,
		},
		// ---- L1 批3（2026-10-08）：`zerg mcp` 真动作分派（★ 真 Go 化 · 非壳搬运 · 占位 `mcp ls` 下架）----
		// 依据：`mcp_zerg_server-真Go化-先期设计-20261008.md` §五 步3（命令树接线）· §三（真 Go 化路径）。
		// 真实现 = `core/cmd/zerg/mcp_serve.go`（8 工具真 Go 化 · 默认 stdio · 复用仓内
		// `github.com/mark3labs/mcp-go v0.58.0` 的 `server` 子包 —— ✗ 不引新第三方模块 · ✗ 手写 JSON-RPC）；
		// 承载宿主 = `ui_start.go`（门78 面④ 要求新增 path 的实现件**已在** `scripts/公开标记.tsv` ⇒
		// 复用**已登记**件作宿主 · 本单铁律禁碰该表 —— 同 `cmdSandboxProbe` / `cmdWallEvidence`）。
		// ★ 口径（照设计 §3.6）：stdio 下 stdout 是 **MCP 协议通道** ⇒ 本命令**没有** `--json` 信封面
		// （前置拒收 rc=2）；退码只表「server 本身能不能起」：0 干净退出 · 2 用法错 · 8 名册读不到 · 1 收尾失败。
		// ★ host 取值照 **GAP-73 拍定 B**：`fleet_topology` 的机器地址与 `machine` 合法集**从名册取**
		// （`gateway/fleet.yaml` 的 `fleet:` 段 —— 自陈是 `~/.zerg/contexts/default.yaml` 的 `nodes:` 之
		// **同形投影**）· ✗ **不碰** `core/internal/store`（主控心跳契约）。
		{
			path:    []string{"mcp", "serve"},
			kind:    "McpServe",
			layer:   "host",
			summary: "虫族控制面 MCP server（默认 stdio · 8 工具：zerg_help/fleet_status/list_models/get_model_info/fleet_topology/load_model/unload_model/switch_model）—— 真 Go 化（复用仓内 mark3labs/mcp-go · stdout 是 MCP 协议通道 ⇒ 无 --json）",
			usage:   "zerg mcp serve",
			run:     cmdMCPServeFamily,
		},
		// ---- 接线批（2026-10-08）：四件「已真搬/真壳 · 命令面未接线」接进命令树 ----
		// 依据：`~/.zerg/state/退役-其余簇-接线复核-20261008.tsv`（`archive-mount` 工具面「未进命令树」）·
		// `A档18件-收CLI设计稿-20261007.md` A11/A4/A7（archive/build/eval **现役族扩 action** · 在闭集 ✓）·
		// 缺口 `GAP-20260928-12`（`script inventory` 只读口）。真实现分别在 `archive_mount.go`
		// （`cmdArchiveMount` · **真搬**）· `eval_run_all.go`（`cmdEvalRunAll` · **真搬**）·
		// `build_wall.go`（`cmdBuildWall` · 壳转账）· `family_script_inventory.go`（`cmdScriptInventoryVerify` · 只读）。
		// 入口宿主 = `ui_start.go`（门78 面④ 与 `cmdSandboxProbe` 同规 · 本单禁碰 `公开标记.tsv`）。
		{
			path:     []string{"archive", "mount"},
			kind:     "ArchiveMount",
			summary:  "归档区**按需挂/卸**（on|off|status · **真搬**：除 `hdiutil`/`mount` 两道系统工具薄壳外零 exec · 脚本移出仓读数逐字同 ⇒ 不退 8）；`status` 未挂 ⇒ 退 1（悬空符号链接不许当正常）· 不给结论 2 · 用法错 64",
			usage:    "zerg archive mount <on|off|status>",
			arity:    "any",
			args:     []string{"动作（on 挂 / off 卸 / status 报挂载态）"},
			endpoint: "",
			run:      cmdArchiveMountFamily,
		},
		{
			path:     []string{"eval", "run-all"},
			kind:     "EvalRunAll",
			summary:  "评测批跑总入口（三层：层1 `go test ./internal/agent/` · 层2/3 已归档载荷真跑 · **真搬**：批跑逻辑/落档/退码在 Go · ✗ 不再 exec `scripts/build/run_all.sh`）",
			usage:    "zerg eval run-all [<模型> [版本标签]]",
			arity:    "any",
			args:     []string{"模型（缺省 example-35b）", "版本标签（缺省 = 当前时刻 yyyymmdd-HHMM）"},
			endpoint: "",
			run:      cmdEvalRunAllFamily,
		},
		{
			path:        []string{"build", "wall"},
			kind:        "BuildWall",
			summary:     "茧壁构建（真身 = `scripts/build/cmd/zerg-build/main.go`（330 行）· 只转发、不翻译 · 退码原样转出 0/2 · 递不到真身 ⇒ 8）",
			usage:       "zerg build wall [--debug|--help]",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/build/cmd/zerg-build/main.go（原样透传 · 含 --debug / --help）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdBuildWallFamily,
		},
		{
			path:     []string{"script", "inventory", "verify"},
			kind:     "ScriptInventoryVerify",
			summary:  "只读核对仓外台账与现跑（新增/消失/格值变化任一非空或声明行不等 ⇒ 1 · 全一致 ⇒ 0 · 读不到 ⇒ 8/2 · ✗ 不写件、不收 `--dry-run`/`--yes` —— 要写用 `sync --yes`）",
			usage:    "zerg script inventory verify [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数）"},
			fields:   []string{"result", "target", "rows_live", "rows_added", "rows_removed", "rows_unchanged", "rows_changed", "rows_changed_named", "rows_changed_cells", "declared_line", "declared_line_live", "in_sync"},
			endpoint: "",
			run:      cmdScriptInventoryVerifyFamily,
		},
		// ---- F37（2026-10-15）：`fs` 族（乙档 · 用户已拍 · 族名照账面 want 逐字）----------------
		// ★ 本笔**只接 `fs unprotected` 一条**（`hidden-faces` 半归 F44 ⇒ ✗ 不碰 `family_fs.go`）。
		// 族名与子动作名照账面三条 want 原文（`GAP-20260927-223`/`-224`/`-225` 的 ③ 长线逐字写
		// `zerg fs unprotected` / `zerg fs hidden-faces`）⇒ 零口径改名。
		// ★ 分工（用户 2026-10-15 拍定）：`zerg repo status` 管「本仓面现状」（七栏不动）·
		//   `zerg fs` 管「**盘 ⟷ 版本面的差**」（含被忽略面 ＋ 体量排行）。
		// ★ 判据一字不落本表项：判据核心 = `family_fs.go:fsUnprotectedCore`（F38 已落 · 本笔只接线 ·
		//   ✗ 不改判据语义 · ✗ 不落第二套判脏口径 —— 真源仍全在 git 自己那一面：
		//   `git ls-files --others`（未跟踪 ∩ 盘上在）＋ `git check-ignore`（未被忽略））。
		// ★ 体量口径 = 降序前 50（`fsUnprotectedDefaultTop`）· 覆盖树 = 全仓（显式排除首段
		//   `bin`/`dist`/`.git`）。退码原样转出核心三态：0 无件 / 1 有件（逐件点名）/ 2 不给结论。
		// ★ ✗ 不新增旗标（usage 只列既有的 `--json` ⇒ 门73 旗标名表零改动 · usageDeclaresFlag 无新增落点）。
		{
			path:    []string{"fs", "unprotected"},
			kind:    "FsUnprotected",
			summary: "列**未保护件**（盘上有 / 版本面无 / 未被忽略 —— 真源全走 git 自己那一面）· 只读 · 体量降序取前 50（覆盖全仓 · 排除 `bin`/`dist`/`.git`）· 退码 0 无件 / 1 有件（逐件点名）/ 2 不给结论",
			usage:   "zerg fs unprotected [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数 —— 仓根取现读工作树：「盘 ⟷ 版本面的差」是**本仓**的读数 · ✗ 不开 `--root`）"},
			fields:  []string{"path", "bytes", "kind"},
			run:     cmdFSUnprotected,
		},
		// ---- GAP-20261017-10（P0 · 2026-10-17）：`fs` 族三条**正门** `read`/`write`/`run` -----------
		// 动因（缺口正文逐字）：对话面要撤 `bash`/`read`/`write`/`edit`，而虫族 CLI **没有对应正门** ——
		// `zerg run|exec|shell|sh` 四条全是未知命令（rc=2），`fs` 族只有 `unprotected` 一个动作
		// （`fs read` / `fs write` 报「是族名/中间节点」）⇒ 先造三条正门，才撤得动对话面。
		// ★ 判据/实现**全部**落在 `core/cmd/zerg/family_fs_tools.go`（✗ 一个字都不落 `family_fs.go` ——
		//   那件是 `fs unprotected` 的判据核心，本笔不改它的语义）。本处只做命令树登记（八格 + 危险档）。
		// ★ 三档：`fs read` **危险档 0**（只读）· `fs write` **D2**（照 `script inventory sync` 那一套）·
		//   `fs run` **D3**（最高 · 首词硬白名单 fail-closed · argv 直投不经 shell）。
		{
			path:    []string{"fs", "read"},
			kind:    "FsRead",
			summary: "只读一件（仓根相对 · 人面出**带行号文本**）· `--json` 闭集五格（path/content/total_lines/offset/limit）· 零写盘 · 件不存在 ⇒ 2（不给结论 · ✗ 不冒充空件）· `--allow-sibling-repo <名>` 显式开闸读兄弟仓（闭集只认登记过的兄弟仓名 · 缺省仍 fail-closed）",
			usage:   "zerg fs read <件> [--offset N] [--limit N] [--allow-sibling-repo <名>] [--json <字段>]",
			arity:   "any",
			args:    []string{"件（仓根相对路径；绝对路径只收**仓内**的 —— 出仓 ⇒ 2；给 `--allow-sibling-repo <名>` 时改按**兄弟仓根**解析 · 闭集只认登记过的兄弟仓名）", "窗口：`--offset N` 起始行（1 起 · 缺省 1）· `--limit N` 取几行（缺省 0 = 到件尾）"},
			fields:  fsReadFields,
			run:     cmdFSRead,
		},
		{
			path:    []string{"fs", "write"},
			kind:    "FsWrite",
			summary: "写/追加本仓一件（**D2**：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面先落审计 `edit_audit.jsonl`）· 禁绝对路径出仓 · 禁符号链接 · 父目录不存在 ⇒ 2（✗ 不自建）· `--replace <补丁件>` 定点替换（每格 `old` 恰好命中 1 次才改 · **不必先读全文**）· `--allow-sibling-repo <名>` 显式开闸写兄弟仓（闭集只认登记过的兄弟仓名 · 审计行带 `sibling_repo`/`sibling_root` · 缺省仍 fail-closed）",
			usage:   "zerg fs write <件> [--content <文本> | --content-stdin | --replace <补丁件>] [--append] [--by <谁>] [--allow-sibling-repo <名>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件（仓根相对路径 · 父目录必须**已存在**；给 `--allow-sibling-repo <名>` 时改按**兄弟仓根**解析 · 闭集只认登记过的兄弟仓名）", "内容来源：`--content <文本>` / `--content-stdin` / `--replace <补丁件>`（**三选一**；补丁件 = `[{\"old\":…,\"new\":…}]` · 每格 `old` 恰好命中 1 次 · 与 `dev edit --replace` 同形 · 走它**不必先读全文**）"},
			fields:  fsWriteFields,
			danger:  &dangerSpec{dangerD2, "件路径（仓根相对）", "写/追加**本仓工作树内**的一件（同目录临时件 + rename 原子写 · 写后读回对拍 · 不等即按前态写回）；审计先落 `<状态目录>/edit_audit.jsonl`（事件 `fs_file_written`）—— 可逆：按该审计行的 `fs_before_sha256` 恢复（件原本不在 ⇒ 删件）", "缺口 `GAP-20261017-10`（P0）· 同族 D2 写面先例 `script inventory sync`（--dry-run 零副作用 · 缺 --yes fail-closed · 写面进审计）", false, dangerScopeRepo},
			opened:  true,
			run:     cmdFSWrite,
		},
		{
			path:    []string{"fs", "run"},
			kind:    "FsRun",
			layer:   "host",
			summary: "跑一条命令（**D3**：首词硬白名单 `zerg|go|git|make|npm|cargo` · 其余 **rc=2 拒执** · **argv 直投 ✗ 不经 shell** · `--cwd` 必须在仓内 · 超时缺省 120s · 输出超 64KB 截断并标截断）",
			usage:   "zerg fs run <命令…> [--cwd <目录>] [--timeout <秒>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"命令与实参（首词只许 zerg/go/git/make/npm/cargo；子命令另有闭集：go `build|test|vet|fmt` · git `status|log|diff|show|cat-file|ls-files`）", "子命令自己的旗标写在 `--` 之后（例：`zerg fs run git log -- --oneline -3 --yes`）—— CLI 解析器把 `-` 开头的词当本命令旗标，`--` 之后一律原样转给子命令"},
			fields:  fsRunFields,
			danger:  &dangerSpec{dangerD3, "整条 argv（命令串）", "以本仓为工作目录**真起子进程**（argv 直投 · 不起 shell · 输出合流回显）；子命令闭集当场拒掉 `go run` / `git push` / `git reset --hard` 一类 —— 本命令**自己不写仓件、不落审计**（写不写由被跑的那条命令自己的危险档负责）", "缺口 `GAP-20261017-10`（P0）· 危险档最高档（D3）· fail-closed 硬白名单", false, dangerScopeUnknown},
			opened:  true,
			run:     cmdFSRun,
		},
		// ---- 技能库族（设计稿 `zerg-技能库设计稿-20261017.md` §3 · M1 只读面）----
		// 根只认一处：`statepath.SkillsDir()`（`ZERG_SKILLS_DIR` → `<仓根>/core/internal/agent/skills`）。
		// ✗ 不开 `--root`、✗ 不新增第二个默认根；`--json` 字段表写字面量（`clicFieldsRE` 直读这一行）。
		{
			path:    []string{"skill", "ls"},
			kind:    "SkillLs",
			summary: "技能库清单（**只读**：名字 · 触发描述 · 状态 · 子件数 · 字节数 · 落点；状态把「agent 静默跳过」的件显式标出来）",
			usage:   "zerg skill ls [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数 —— 技能根取唯一解析 `statepath.SkillsDir()` · ✗ 不开 `--root`）"},
			fields:  []string{"name", "description", "state", "files", "bytes", "path"},
			run:     cmdSkillLs,
		},
		{
			path:    []string{"skill", "show"},
			kind:    "SkillShow",
			summary: "看一条技能的正文（**只读**：正文取自 `agent.SkillManager.Load` —— 与 CA 侧 `skill_load` 同一函数，构造上同字节）",
			usage:   "zerg skill show <名> [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（与 `zerg skill ls` 的 `name` 逐字相同）"},
			fields:  []string{"name", "description", "state", "path", "bytes", "lines", "body"},
			run:     cmdSkillShow,
		},
		// ---- 技能库族 · M2 三条 D2 写面（设计稿 `zerg-技能库设计稿-20261017.md` §3.3–3.5）----
		// 三态照 `doc meta fill` / `script inventory sync`：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 ·
		// 写面进 `<状态目录>/skill_audit.jsonl`（审计先落盘）· 写完回读对拍 sha256。
		// ✗ 零新增旗标名（`--desc` / `--from` 早就在 `valueFlagName` 表里）⇒ 门73 两处登记面零改动。
		{
			path:    []string{"skill", "new"},
			kind:    "SkillNew",
			summary: "新建一条技能（**D2 写面**：默认干跑 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 同名已存在 ⇒ 14 不覆盖 · 写面进审计）",
			usage:   "zerg skill new <名> (--desc <一句话触发器> | --from <件|->) [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（`^[a-z0-9][a-z0-9-]{0,63}$` · 同时是目录名）", "（`--desc` 或 `--from` 二选一：前者的那一行会进系统提示，是触发器；后者是仓外整件 · `-` = stdin）"},
			fields:  []string{"result", "name", "path", "before_sha256", "after_sha256", "bytes", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"在技能根建一个 `<名>/SKILL.md`（可逆：同名已存在即拒 14 · 写前算 sha256 · 写后回读对拍 · 审计先落盘）",
				"zerg-技能库设计稿-20261017.md §3.3 · 同族先例 doc meta fill（D2）· 写权限只给人的既有教条（ai_boundary.go:209）",
				false, dangerScopeRepo},
			opened: true,
			run:    cmdSkillNew,
		},
		{
			path:    []string{"skill", "edit"},
			kind:    "SkillEdit",
			summary: "改一条技能（**D2 写面**：`--desc` 只改描述行 / `--from <件|->` 整篇替换 · 互斥 · 默认干跑 · 缺 `--yes` ⇒ 2 · 写前备份 ＋ 审计）",
			usage:   "zerg skill edit <名> (--desc <新触发器> | --from <件|->) [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（与 `zerg skill ls` 的 `name` 逐字相同）", "改法：`--desc`（只改描述行 · 正文一字不动）或 `--from`（整篇 · `-` = stdin）"},
			fields:  []string{"result", "name", "path", "mode", "before_sha256", "after_sha256", "bytes", "backup_path", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"改 `<名>/SKILL.md`（可逆：写前备份到 `<状态目录>/skill-backup/<ts>/<名>/` + 写后回读对拍 + 审计先落盘）",
				"zerg-技能库设计稿-20261017.md §3.4 · 同族先例 doc meta fill（D2）",
				false, dangerScopeUnknown}, // 落点由 ZERG_SKILLS_DIR 决定 ⇒ 可能仓内、可能仓外（guard.go:108）
			opened: true,
			run:    cmdSkillEdit,
		},
		{
			path:    []string{"skill", "rm"},
			kind:    "SkillRm",
			summary: "下架一条技能（**D2 写面**：**不硬删** —— 整目录挪进 `<状态目录>/skill-trash/<ts>/` 可逆；默认干跑 · 缺 `--yes` ⇒ 2 · 写面进审计）",
			usage:   "zerg skill rm <名> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（= 目录名）"},
			fields:  []string{"result", "name", "path", "trash_path", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"把 `<名>/` 整目录挪进回收站（可逆：`os.Rename` 同卷即时生效；跨卷 ⇒ 照实报 1，不降级成删除）",
				"zerg-技能库设计稿-20261017.md §3.5 · 同族先例 doc meta fill（D2）",
				false, dangerScopeUnknown},
			opened: true,
			run:    cmdSkillRm,
		},
	}
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

	plain       bool
	wantHelp    bool
	wantVersion bool
	tty         bool // stdout 是不是终端（行式面/人面的分档依据 · §九 M14）

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
		case a == "--help" || a == "-h":
			inv.wantHelp = true
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
	registered := make(map[string]bool, len(cmd.fields)+len(cmd.optFields))
	for _, f := range cmd.fields {
		registered[f] = true
	}
	// ★ 可选字段层（`cmd.optFields`）：登在**出口白名单**里、点得出来；但**不进** `cmd.fields`
	//   （那一份是文档/帮助导出面的冻结字段表 —— 可选字段是只增的第二层，`approveOptionalFields` 即它）。
	for _, f := range cmd.optFields {
		registered[f] = true
	}
	for _, f := range fields {
		if !registered[f] {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, path, f)
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
