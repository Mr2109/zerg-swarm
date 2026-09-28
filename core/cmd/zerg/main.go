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
	"strings"
	"syscall"

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
	exitAuth        = 4   // 未认证（403 与它并码、kind 分家）
	exitBlocked     = 8   // 有 BLOCKED
	exitResource    = 10  // 资源不足
	exitTimeout     = 11  // 超时
	exitUnreachable = 12  // 不可达（打不到主控）
	exitConflict    = 14  // 冲突 / 被占
	exitInterrupted = 130 // 人打断（Ctrl-C）
)

func main() {
	// 人打断（§九 M8 · §十二 `P-033`）：**默认只退订、不取消** —— Ctrl-C 退 `130`，
	// 被跟的任务/被包的脚本**继续跑**；要真取消得给显式旗标 `--cancel-on-interrupt`。
	// 为什么不在这里做别的清理：命令面**无常驻状态**（§6.1），「退订」= 本进程退出。
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		if wantsCancelOnInterrupt(os.Args[1:]) {
			cancelRunningChild()
			fmt.Fprintf(os.Stderr, "%s: 人打断（Ctrl-C）—— 已按 `--cancel-on-interrupt` **取消**在跑的步骤\n", progName)
		} else {
			fmt.Fprintf(os.Stderr, "%s: 人打断（Ctrl-C）—— **只退订、不取消**（在跑的任务/脚本不受影响；要取消给 --cancel-on-interrupt）\n", progName)
		}
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

// cancelRunningChild 只杀我们**亲手起的那个子进程**（不杀进程组、不碰别人）。
func cancelRunningChild() {
	if runningChild != nil && runningChild.Process != nil {
		_ = runningChild.Process.Kill()
	}
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
		fmt.Fprintf(stderr, "%s: 未知命令 %q\n", progName, strings.Join(inv.path, " "))
		s := nearest(inv.path[0])
		if s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s %s\n", progName, s)
		}
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		// 2026-09-27（缺口 `-23` · Mr2109 拍板选项②）：族名不吃 `--help`（`zerg gate --help` 退 2）⇒
		//   在「未知命令」块**首行之后**补一行**族级帮助**正门指引（**只进 stderr** · 不动既有两态语义）。
		// 2026-09-27（缺口 `GAP-20260927-40` · 修正）：**只有 `inv.path[0]` 确实是真族名时才打** ——
		//   判据 = `nearest` 给的最像命令首词与它相同（敲错字时 nearest 指向的是**别的族** ⇒ 不打，
		//   免得把用户指到一条不存在的正门：`zerg eggs` 曾打出 `zerg help eggs`）。
		if len(inv.path) > 0 && (s == inv.path[0] || strings.HasPrefix(s, inv.path[0]+" ")) {
			fmt.Fprintf(stderr, "族级帮助: `%s help %s`\n", progName, inv.path[0])
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
	//   判据 = 「给过 `--top`」且「本命令不是 `gap status`」（`cmd.path` 是 `resolve` 出来的**规范路径**）。
	//   `--by` 不在此列：它**早已**在名字表上（与 `dev proposal` / `gap add` 共用）⇒ 既有行为不变。
	if inv.hasFlag("--top") && !(len(cmd.path) == 2 && cmd.path[0] == "gap" && (cmd.path[1] == "status" || cmd.path[1] == "plan")) {
		inv.unknown = append(inv.unknown, "--top")
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
	if rc != exitOK && inv.jsonGiven && cw.n == 0 && (inv.err != nil || len(inv.fields) > 0 || cmd.danger != nil) {
		emitErrIfJSON(inv, stdout, cmd, rc)
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
	emitErrEnvelope(stdout, cmd, e)
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
// 现读反查过全部命令的「读 `inv.args` 的 handler ↔ 是否声明位置参数」，只剩 6 条不一致，
// 其中 5 条**自己就拒**（`api help` / `script inventory sync` / `itask start` / `itask stop` /
// `core reload`），第 6 条（`gateway breakers` 的 target）在危险档 ⇒ 本函数**不覆盖它们**
// （`cmd.danger != nil` 一律不判 · 见 `run` 里那三条边界）。
func declaresNoPositional(cmd *command) bool {
	if cmd == nil {
		return false
	}
	return cmd.arity != arityAny
}

// ---- 命令树（真源：帮助文本、markdown 导出、别名解析都从这里出，不许旁写一份）----

type command struct {
	path     []string
	summary  string
	usage    string
	fields   []string // --json 可取的全部字段（不给字段时 stderr 列的就是它）
	args     []string // 位置参数的说明（帮助里逐条列出 · **只给人读** · 不再当判据口）
	arity    string   // 位置参数元数（声明式 · `R-7`）：闭集 none/exact(n)/min(n)/max(n)/any；本版只用 none/any（空串视作 none）
	endpoint string   // 它投影的远端端点（本机命令为空）；T-08 的三面同源对账用
	kind     string   // 包封里的 kind（§九 M6 I2：与命令一一对应 · 单数 CamelCase）
	danger   *dangerSpec
	idem     *idemSpec // M4 幂等四字段（没显式写的在 seedIdem 里按规则派生）
	// 群级只读（§十二 P-066）：没有目标时「读全群」是允许的；**其余命令无目标 ⇒ exit 2**。
	groupReadOnly bool
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
			usage:   "zerg version [--json <字段>]",
			fields:  []string{"name", "version", "commit", "build_time", "contract", "object_schema", "core_version", "core_code_sha", "window"},
			run:     cmdVersion,
		},
		{
			path:    []string{"help"},
			summary: "帮助（主题见 `zerg help <主题>`；表在 topics.go）",
			usage:   "zerg help [<主题>]",
			arity:   "any",
			args:    []string{"主题（可省）"},
			run:     cmdHelp,
		},
		// ---- 批 A · S2 只读面（§6.2 最小可验证集）：全程零写操作 ----
		{
			path:     []string{"doctor"},
			kind:     "DoctorCheck",
			summary:  "环境自检（本机项 + 主控可达）· 逐项判定词",
			usage:    "zerg doctor [--json <字段>]",
			fields:   []string{"name", "verdict", "detail", "advice", "artifact_commit", "source_head"},
			endpoint: "",
			run:      cmdDoctor,
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
		{
			path:     []string{"agent", "ls"},
			kind:     "Machine",
			summary:  "子端（机器）清单（投影 /api/fleet/status）",
			usage:    "zerg agent ls [--json <字段>]",
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
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
			path:     []string{"help", "export"},
			kind:     "HelpExport",
			summary:  "把命令树导出 markdown 进版本档案目录（产物 · 勿手改）；只读档 `--dry-run` 出逐条清单、一个字节不写",
			usage:    "zerg help export [--out <目录> | --docs-ver <X.Y.Z>] [--dry-run] [--json <字段>]",
			fields:   helpExportFields,
			endpoint: "",
			run:      cmdHelpExport,
		},
		// ---- 批 A · S3 门禁直通六条（**只转发、不翻译** · §6.3 S3 判据③）----
		{
			path:        []string{"gate", "ls"},
			summary:     "门禁步骤表（逐行等于脚本 --list；薄壳不另写一份）",
			usage:       "zerg gate ls",
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:        []string{"gate", "run"},
			summary:     "跑门禁（旗标逐字透传；退码原样转出，不翻译；`--step` 只跑一道门）",
			usage:       "zerg gate run [--scope <s> | --fast] [--outdir <目录>] … | zerg gate run --step <步名> [--self-test] [--only-step] [--json <字段>] [--verify-live] [--show-log]",
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
			usage:   "zerg gate find <片段> [--json <字段>]",
			arity:   "any",
			args:    []string{"片段（门件名或命令串里的一段，例：check-gate-coverage）"},
			// 九格 = 与 `gate explain` 同义字段逐字同名（判据/日志/出处在本仓只有一处口径）+ 第八格
			// `next`（照抄即走的下一步）+ 第九格 `wiring`（三处接线面 precommit / all.sh / real-gates.sh
			// 的现读结论 —— 「零命中」与「不存在」的分别就靠它）。★ 本命令**不走脚本**：只读脚本里的
			// `add_step` 声明 ⇒ 与 `gate results` 同规（命令面分支），不改任何脚本退码。
			fields:   gateFindFields,
			endpoint: "",
			run:      cmdGate,
		},
		{
			path:    []string{"gate", "results"},
			kind:    "GateResults",
			summary: "读**现成**一趟门禁产物的四数（通过/失败/不给结论/只报告 + 步数与总退码）· 只读",
			usage:   "zerg gate results [--last] [--dir <目录>] [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数：那一趟由 `--last`（缺省即最近一趟）或 `--dir <目录>` 指）"},
			// 逐条五格 = 步名/状态/退码/耗时/日志路径（缺口 `Q-111`/`B-8` 的机器面）。
			fields:   gateResultsFields,
			endpoint: "",
			run:      cmdGate,
		},
		{
			path:        []string{"gate", "self-test"},
			summary:     "门禁自检（合成步骤 · 不碰真目标）",
			usage:       "zerg gate self-test",
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		// ---- 批 B · T-19 茧壁：`agent ping` 默认经主控、`--direct` 才直连（§十五.4 丙案）----
		{
			path:     []string{"agent", "ping"},
			kind:     "AgentPing",
			summary:  "探活一台机（默认经主控；`--direct` 直连且回显 via）",
			usage:    "zerg agent ping <机器名> [--direct <host:port>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via"},
			endpoint: "GET /api/fleet/status（默认档）",
			run:      cmdAgentPing,
		},
		// ---- 批 B · T-17 事件面（§九 M1）：**唯一名字** `zerg watch` ----
		{
			path:          []string{"watch"},
			kind:          "Watch",
			summary:       "订阅事件流（单端点 + Accept 协商 · 唯一名字）",
			usage:         "zerg watch [<id>] [--accept <媒体类型>] [--exit-on <kind>] [--follow]",
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
			endpoint: "GET /api/tasks",
			run:      cmdTaskShow,
		},
		// ---- 批 D · T-43 `agent` 族收口（§三 B 族 · §十二 `P-038` 族名闭合 · 只读四条）----
		{
			path:     []string{"agent", "show"},
			kind:     "Machine",
			summary:  "看一台子端（投影 /api/fleet/status 的单机条目 —— 与 `agent ls` 同一份载荷）",
			usage:    "zerg agent show <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
			endpoint: "GET /api/fleet/status",
			run:      cmdAgentShow,
		},
		{
			path:     []string{"agent", "models"},
			kind:     "Model",
			summary:  "该机上可用的模型（投影 /api/fleet/models 里 host 命中的那些）",
			usage:    "zerg agent models <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdAgentModels,
		},
		{
			path:     []string{"agent", "logs"},
			kind:     "AgentLogs",
			summary:  "该机的日志（**主控面今天没有日志端点 ⇒ 不给结论**，退码 8）",
			usage:    "zerg agent logs <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "available", "detail"},
			endpoint: "（/api/logs/* 今天不在路由表里 ⇒ 本命令声明面 + kind=blocked）",
			run:      cmdAgentLogs,
		},
		{
			path:     []string{"agent", "probe"},
			kind:     "AgentProbe",
			summary:  "探活诊断（默认经主控；直达属 F-2 例外、要显式 `--direct`）",
			usage:    "zerg agent probe <机器名> [--direct] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via", "direct_gate"},
			endpoint: "GET /api/fleet/status（默认档）",
			run:      cmdAgentProbe,
		},
		// ---- 缺口 `GAP-20260925-50` ②③④ · `agent` 族第二组（2026-09-26）----
		{
			path:     []string{"agent", "reload"},
			kind:     "AgentReload",
			summary:  "子端热重载（`POST http://<host>:<port>/infer/reload` · **必带 `X-Auth-Token`**（不带就回 unauthorized）；子端那三格原样透传）",
			usage:    "zerg agent reload <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentReloadFields,
			endpoint: "POST http://<host>:<port>/infer/reload（子端 HTTP 面 · 令牌只从凭据链取、走 X-Auth-Token）",
			run:      cmdAgentReload,
		},
		{
			path:     []string{"agent", "registry"},
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
		{
			path:    []string{"agent", "bootstrap"},
			summary: "子端引导（危险 D3 · 本版未开放）",
			usage:   "zerg agent bootstrap <机器名> --confirm=<机器名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"在目标机上装/起子端（**F-1 子端引导例外**：不经主控的显式命令；会改目标机状态）",
				"§十五.4 F-1 · §九 M20 · 开工单 T-43", true},
			run: cmdGuarded,
		},
		// ---- 批 D · T-44 `task` 族全动作面（§三 D 族 · 12 个动作名一个不差）----
		{
			path:     []string{"task", "submit"},
			kind:     "TaskSubmit",
			summary:  "提交任务（D2 写面 · 全旗标 ⇒ API 请求体逐条对上）",
			usage:    "zerg task submit --desc <描述> [--model <模型>] [--priority <n>] [--slice-id <片>] [--depends-on <片>]… [--acceptance <判据>]… [--dry-run | --yes]",
			arity:    "none",
			args:     []string{"（旗标：--desc/--model/--priority/--slice-id/--depends-on/--acceptance）"},
			endpoint: "POST /api/tasks",
			run:      cmdTaskSubmit,
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
			path:    []string{"task", "move"},
			summary: "挪动任务在队列里的位置（危险 D2 · 本版未开放）",
			usage:   "zerg task move <任务 id> --to <位置> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "改这条任务在队列里的次序（可能插到别人前面）", "§三 D 族 · 开工单 T-44", true},
			run:     cmdGuarded,
		},
		// ---- 批 D · T-45 `model` + `core` 两族（§三 E/C 族 · §7.1 `P11` · §十二 `P-029`）----
		{
			path:     []string{"model", "show"},
			kind:     "Model",
			summary:  "看一个模型（投影 /api/fleet/models 的单条；对象是**模型 id**，不是机器名）",
			usage:    "zerg model show <模型 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdModelShow,
		},
		{
			path:     []string{"model", "opts"},
			kind:     "ModelOpts",
			summary:  "适配器参数（get 只读 / set 实时生效要 --yes）",
			usage:    "zerg model opts get <模型 id> [--json <字段>] | zerg model opts set <模型 id> --set k=v… [--dry-run | --yes]",
			arity:    "any",
			args:     []string{"动作（get|set）", "模型 id"},
			fields:   []string{"model", "schema", "note"},
			endpoint: "GET|PUT /api/models/{name}/adapter-opts",
			run:      cmdModelOpts,
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
		{
			path:    []string{"core", "start"},
			summary: "起主控（危险 D3 · 本版未开放）",
			usage:   "zerg core start --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "起主控进程（会绑端口 8580；已在跑时是**换件**前置）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "restart"},
			summary: "重启主控（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`）· 审计留痕 + 就绪判据绑自己的 pid",
			usage:   "zerg core restart --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停 + 起主控（**整个虫群的控制面会断一会儿**）", "§三 C 族 · 开工单 T-45 · 缺口 Q-103", true},
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
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "gpu_pct", "backend_state", "fit"},
			endpoint: "GET /api/resources/ledger | /api/resources/{type}",
			run:      cmdResourceLs,
		},
		{
			path:     []string{"resource", "ledger"},
			kind:     "Resource",
			summary:  "资源账本（投影 /api/resources/ledger）",
			usage:    "zerg resource ledger [--json <字段>]",
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "gpu_pct", "backend_state", "fit"},
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
			summary:  "本机脚本清单（scripts/ 逐件 + 公开标记表）",
			usage:    "zerg script ls [--json <字段>]",
			arity:    "none",
			fields:   []string{"path", "public", "kind", "count_basis"},
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
			danger:   &dangerSpec{dangerD2, "台账件", "把仓外台账的行面与两个 yes/no 列按现跑重算（可逆：写前备份 + 写后读回，复查不过逐字节写回）；**一条命令写**、**不许第二条写路径**", "设计-仓外台账写面-A3-b案-v1.0-20260923.md §二 · `A3` = `G-13`/`Q-002`", false},
			opened:   true,
			endpoint: "",
			run:      cmdScriptInventorySync,
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
			usage:   "zerg code find <正则> [--path <目录或单件>] [--glob <模式>] [--limit <N>] [--count | --files-only] [--full] [--json <字段>]",
			arity:   "any",
			args:    []string{"正则（POSIX 语法）"},
			// ★ `GAP-20260928-118`（2026-09-28 · 本枚）：字段表补上 `truncated` —— 截断位此前
			// 只能从顶层包封读，字段面没有直读口（给 `truncated` 即报未知字段）。三档面（默认 /
			// `--count` / `--files-only`）逐行都产出它（**跑级真值**：本跑真裁了 = true，没裁 = false）
			// ⇒ 表与产出集同源；不给这一格时输出逐字节不变（`marshalObject` 只投影点名的格）。
			fields:   []string{"path", "line", "text", "truncated"},
			endpoint: "",
			run:      cmdCodeFind,
		},
		{
			path:     []string{"code", "show"},
			kind:     "CodeShow",
			summary:  "看源码里**某一行**长什么样（带 `件:行` · 只读取证 · 手搓 `sed -n` / `awk` 的替身）",
			usage:    "zerg code show <件:行 | 件:起-止> [--ctx <N>] [--full] [--json <字段>]",
			arity:    "any",
			args:     []string{"件:行（件 = 仓相对路径 · 行 = 正整数）"},
			fields:   []string{"path", "line", "text", "target"},
			endpoint: "",
			run:      cmdCodeShow,
		},
		{
			path:     []string{"repo", "status"},
			kind:     "RepoStatus",
			summary:  "看仓脏没脏 / HEAD 在哪 / 有没有别人在写它（手敲 git status 的替身）· `--root` 给 ≥2 次 ⇒ **多仓汇总**（一条只读命令出两仓 HEAD + 脏件数；写旗标一律拒 2）· **三态面**：行面七栏 `head/branch/path/status/untracked/mtime/sha256`，其中 `untracked` 栏只答「**未被忽略的真未跟踪**」（**件数随仓变 · 不在这里写死任何数** —— 写了就会烂；要数就现跑一条，按行面 `path` 前缀数「`untracked`=是」的行）；**被忽略**（`.gitignore` 命中）态现读**无栏** ✗",
			usage:    "zerg repo status [--root <仓根>] [--json <字段>]",
			fields:   []string{"head", "branch", "path", "status", "untracked", "mtime", "sha256"},
			endpoint: "",
			run:      cmdRepoStatus,
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
			path:     []string{"audit", "cover"},
			kind:     "AuditCover",
			summary:  "对拍「工作树脏件 ↔ 审计行」：分母 = `git status --porcelain=v1` 的**已跟踪 `M`/`A` 行**（真源是 git 自己的面 · **不由审计定**）· 逐件比「工作树 `sha256` ↔ 审计里该件最新一条 `edit` 行的 `after_sha256`」· 四判词 `有记录/无记录/记录过期/读不到` · 逐条点名未覆盖件（件·状态·工作树 sha16·审计有无行）+ 计数 · **只报告档**：有未覆盖仍退 0（不进任何必跑路径）· `8` = 读不到/**分母 0 判不了**（不当绿）· 与 `GAP-20260928-79` 族对偶（79 = 有行但回指不到授权物 · 本格 = 压根没有行）",
			usage:    auditCoverUsage,
			arity:    "none",
			args:     []string{"（不收位置参数 —— 面由 `--file <件>` 收窄、仓根由 `--root <仓根>` 给）"},
			fields:   auditCoverFields,
			endpoint: "",
			run:      cmdAuditCover,
		},
		// ---- D3b 第三步（2026-09-21）：**提交面**（缺口-命令面 §九 I4）----
		// 按文件名暂存（禁 `git add -A`）· 过快速档才放行 · 禁 `--no-verify` · 提交信息模板。
		{
			path:    []string{"repo", "commit"},
			kind:    "RepoCommit",
			summary: "提交：**按文件名逐件暂存**（禁 `git add -A`）· 或**点名单路径**（`--only <路径…>`：索引面允许非空、别人的暂存只许多不许少）· **过快速档才放行** · 禁 `--no-verify`（例外走 `--waive <步名> --reason <…>` 并进审计）",
			usage:   "zerg repo commit --message <题> (--file <件>… | --only <路径>[ --only <路径>]…) [--proposal <提案 id>] [--by <谁>] [--trace <id>] [--criterion <判据>] [--waive <步名> --reason <理由>] [--dry-run] [--yes]",
			arity:   "any",
			args:    []string{"提交主题（--message）", "逐件点名（--file · 可重复）或点名单路径（--only · 可重复 · 可 `--only=<路径>`）"},
			fields:  repoCommitFields,
			danger:  &dangerSpec{dangerD2, "提交主题", "把点名的件提交（可逆：`git reset --soft HEAD~1`）；**先跑快速档**，rc≠0 不提交（要带账放行得 `--waive <步名> --reason <…>`）", "缺口-命令面 §九 I4 · §九 M3 C5 · D3b 第三步 · 缺口 Q-104", false},
			// `opened`: 真跑已开放（`--yes` 就执行 —— 默认模式逐件暂存 / `--only` 模式点名单路径；D2 可逆）。
			opened:   true,
			endpoint: "",
			run:      cmdRepoCommit,
		},
		{
			path:     []string{"gate", "explain"},
			kind:     "GateExplain",
			summary:  "读懂某一步到底在判什么（scope/模式/判据/退码口径/日志路径/出处文件:行 —— **精确匹配**步名）",
			usage:    "zerg gate explain <步名> [--json <字段>]",
			arity:    "any",
			args:     []string{"步名（与 `zerg gate ls` 逐字相同）"},
			fields:   []string{"scope", "mode", "criterion", "verdict", "exit", "log", "source", "command", "script_say"},
			endpoint: "",
			run:      cmdGateExplain,
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
				"缺口-命令面-20260921 §十一 P0-5 · 同族写面先例 `zerg gap export`（`--out` 真写盘）· §九 M3 C1", false},
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
				"缺口-命令面-20260921 §八 H2 · 开工记录 D3 §三 新增 1 条 · D3③-a 的等价命令", false},
			// `opened`: 真跑已开放（`--yes` 就回填文件头；审计先落盘 + 回读对拍 sha256）。
			opened: true,
			run:    cmdDocMeta,
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
			summary:  "「这件会不会发 + 哪一层拦的」**只读**读数（**免树** · 逐层现读判定链真源件复算 · 链真源读不到 ⇒ 8 · **不给结论**）",
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
			kind:    "PublishRun",
			summary: "出公开镜像树（**本地** dist/<版本>/release · 白名单+排除项+脱敏 · 不出网、不推送）",
			usage:   "zerg publish run [--dry-run | --confirm=<主机名> --yes]",
			// 本版已开放（Mr2109 2026-09-25「脱敏推」授权）：真跑只写**本地**产物目录。
			// 「推远端」不在命令面 —— 不可逆动作按规矩等 Mr2109 发话后单独执行（不另开执行路径）。
			run: cmdPublishRun,
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
		//   `llama-server`），而人要的是 x3；当时只有「手工 `POST /api/control/unload {"machine":"Mr2109"}`
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
				"设计-指定机器路由-v1.0-20260924.md §4 · 单独定制 > 路由默认规则（个别要求一次性 + TTL）", false},
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
				"设计-指定机器路由-v1.0-20260924.md §4 · 「一行撤回」那条约束的落点", false},
			opened:   true,
			endpoint: "",
			run:      cmdRouteUnpin,
		},
		// 写面两枚：**同一个执行门**（三态：--dry-run 计划件 / 缺 --yes ⇒ 2 / 齐了才发）
		{
			path:    []string{"resource", "pin"},
			kind:    "ResourcePin",
			summary: "钉住资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource pin <资源 id> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"resource", "unpin"},
			kind:    "ResourceUnpin",
			summary: "解钉资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource unpin <资源 id> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"gateway", "breakers"},
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
		{
			path:     []string{"egg", "run"},
			kind:     "EggRun",
			summary:  "把卵的模型**装到**那台机上（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/load`）",
			usage:    "zerg egg run <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/load（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggRun,
		},
		{
			path:     []string{"egg", "stop"},
			kind:     "EggStop",
			summary:  "卸载一枚卵 —— 把那台机上的该模型**卸掉**（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/unload`）",
			usage:    "zerg egg stop <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/unload（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggStop,
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
			summary:  "自动运行开关现值（只读面；写面 `itask mode set` 未开放）",
			usage:    "zerg itask mode [--json <字段>]",
			fields:   []string{"modes", "note", "state"},
			endpoint: "GET /api/internal-tasks/modes",
			run:      cmdItaskMode,
		},
		{
			path:     []string{"itask", "interval"},
			kind:     "InternalTaskInterval",
			summary:  "周期现值（只读面；写面 `itask interval set` 未开放）",
			usage:    "zerg itask interval [--json <字段>]",
			fields:   []string{"intervals", "note", "state"},
			endpoint: "GET /api/internal-tasks/intervals",
			run:      cmdItaskInterval,
		},
		{
			path:    []string{"itask", "start"},
			summary: "起内部任务引擎（危险 D2 · 本版未开放）",
			usage:   "zerg itask start --yes [--dry-run]",
			danger:  &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎开始按周期跑（会自动占机器与模型槽）", "§7.1 P8 · §5.1 POST /api/internal-tasks/start · 开工单 T-49", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"itask", "stop"},
			summary: "停内部任务引擎（危险 D2 · 本版未开放）",
			usage:   "zerg itask stop --yes [--dry-run]",
			danger:  &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎停下（在跑的进化任务会跑到当前一轮为止）", "§7.1 P8 · §5.1 POST /api/internal-tasks/stop · 开工单 T-49", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"itask", "run"},
			summary: "手动跑一条内部任务（危险 D2 · 本版未开放）",
			usage:   "zerg itask run <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "立刻跑一次该内部任务（绕过它的冷却 · 会占机器）", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/run · 开工单 T-49", true},
			run:     cmdGuarded,
		},
		// ---- 批 D · T-48 构建族（K 族 · §3.5 · §7.1 P11 · §17.2 ③ 建环）----
		{
			path:     []string{"build", "ls"},
			kind:     "BuildArtifact",
			summary:  "制品现状（现读 bin/ 逐件 sha256 + 身份件）—— 清单真源仍是构建脚本",
			usage:    "zerg build ls [--json <字段>]",
			fields:   []string{"name", "sha256", "bytes", "mtime", "version", "code", "built"},
			endpoint: "",
			run:      cmdBuildLs,
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
		{
			path:    []string{"build", "all"},
			summary: "重编制品（**自举档已开放**：`--only cli` 只写 bin/zerg 一件；换件档/发布档仍未开放）",
			usage:   "zerg build all [--only cli] [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		{
			path:    []string{"build", "release"},
			summary: "打包发布件（**计划面已开放**：`--dry-run` 出计划件；**清单档 `--manifest-only` 已开放** —— 只重写 `dist/<版本>/release/` 的清单与逐件 sha256，不编译、不重打包、不碰在跑件；换件档真跑本版未开放 · 拒执退码 2）",
			usage:   "zerg build release [--manifest-only] [--dry-run | --confirm=<主机名> --yes]",
			refuses: true, // 真跑拒执（**换件档**）：`cmdBuildPassthrough` 的 `openForExec` 只对 `build all --only cli` 与 `build release --manifest-only`（缺口 `Q-226` · 只写 `dist/<版本>/release/` 一个目录）为真 ⇒ 其余非 dry-run 一律 `not_opened` 退 2
			run:     cmdBuildPassthrough,
		},
		// ---- 危险动作：**只登记形状，不开放执行**（§6.2 批 1 零写操作）----
		// 每条都过 cmdGuarded：`--dry-run` 出计划件（退码 0）；真跑一律拒执（退码 2 = 不给结论）。
		{
			path:    []string{"task", "terminate"},
			summary: "终止任务（危险 D3 · 本版未开放）",
			usage:   "zerg task terminate <任务 id> --confirm=<任务 id> --yes [--expect=<旧值>] [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger: &dangerSpec{dangerD3, "任务 id",
				"终止该任务的执行（CA 侧停 + 任务状态置 terminated）· 已产出的工作树不自动回收",
				"§三 D 族 · §4.1 K7 · §九 M3 C1/C4 · 开工单 T-44", true},
			run: cmdGuarded,
		},
		{
			path:    []string{"task", "rm"},
			summary: "删除任务（危险 D3 · 本版未开放）",
			usage:   "zerg task rm <任务 id> --confirm=<任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger: &dangerSpec{dangerD3, "任务 id",
				"删除该任务的记录与它指派的工作树/分支（**不可逆**）",
				"§三 D 族 · §4.1 K7 · 开工单 T-44", true},
			run: cmdGuarded,
		},
		{
			path:    []string{"task", "pause"},
			summary: "暂停任务（危险 D2 · 本版未开放）",
			usage:   "zerg task pause <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把排队中的任务置为暂停态（可 resume 回来）", "§三 D 族 · §6.3 S5", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "resume"},
			summary: "继续任务（危险 D2 · 本版未开放）",
			usage:   "zerg task resume <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把暂停的任务放回排队（可能立刻占机器）", "§三 D 族 · §6.3 S5", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "retry"},
			summary: "重跑任务（危险 D2 · 本版未开放）",
			usage:   "zerg task retry <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把 failed 任务置回 queued（会再占一次机器与模型槽）", "§三 D 族 · §6.3 S5", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"agent", "unload"},
			summary: "卸载子端上的模型（危险 D3 · 本版未开放）",
			usage:   "zerg agent unload <机器名> <模型> --confirm=<机器名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名", "模型"},
			danger: &dangerSpec{dangerD3, "机器名",
				"卸掉该子端上的模型（**在跑的任务会被打断**）· 幂等：已在未装载态按「已在该状态」报",
				"§三 B 族 · §九 M4 · 开工单 T-43", true},
			run: cmdGuarded,
		},
		{
			path:    []string{"agent", "load"},
			summary: "加载模型到子端（危险 D2 · 本版未开放）",
			usage:   "zerg agent load <机器名> <模型> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名", "模型"},
			danger:  &dangerSpec{dangerD2, "机器名", "把模型装进该子端（占内存/显存槽 · 单槽机是串行的）", "§三 B 族 · §九 M5 · 开工单 T-43", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"agent", "reap"},
			summary: "回收残留干跑单（真回收危险 D3 · 本版未开放；**入库件永不进候选 = 红线**）",
			usage:   "zerg agent reap --dry-run --min-age-days <n>   # 干跑单（带 plan_id + 逐件证据）\n       zerg agent reap --confirm=<机器名> --yes [--dry-run]   # 真回收（本版未开放）",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"按「声明树 ↔ 现值树」差集回收闲置资源（**默认干跑**；入库件永不进候选 = 红线）",
				"§九 M9 · §十五.2 三档回收权 · RC11 红线 · 开工单 T-54", true},
			run: cmdAgentReap,
		},
		{
			path:    []string{"model", "stop"},
			summary: "停模型（危险 D3 · 本版未开放）",
			usage:   "zerg model stop <模型 id> --confirm=<模型 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD3, "模型 id", "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500", "§三 E 族 · §九 M4 · 开工单 T-45", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"model", "start"},
			summary: "起模型（危险 D2 · 本版未开放）",
			usage:   "zerg model start <模型 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD2, "模型 id", "把模型装载起来（占槽位 · 单槽机要排队）", "§三 E 族 · §九 M5 · 开工单 T-45", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "stop"},
			summary: "停主控（危险 D3 · 本版未开放）",
			usage:   "zerg core stop --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停掉主控进程（**整个虫群的控制面会断**）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "reload"},
			summary: "重载主控配置（危险 D2 · 本版未开放）",
			usage:   "zerg core reload --yes [--confirm=<主机名>] [--dry-run]",
			danger:  &dangerSpec{dangerD2, "主机名", "让主控重读配置（规则表/名册）—— 生效面即时", "§三 C 族 · §6.3 S5", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "update"},
			summary: "主控自身换件（危险 D3 · 本版未开放）",
			usage:   "zerg core update --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执）", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "pin"},
			summary: "钉住一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg pin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的）", "§3.4 I 族 · 开工单 T-47", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "unpin"},
			summary: "解钉一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg unpin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "取消在孵标记（原本占有单槽的卵会被换下）", "§3.4 I 族 · 开工单 T-47", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "retire"},
			summary: "退役一枚卵（危险 D3 · 本版未开放）",
			usage:   "zerg egg retire <卵 id> --confirm=<卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD3, "卵 id", "把该卵从名册与盘上退掉（**不可逆**；档 ③ 件永不自动）", "§十五.2 档③ · 开工单 T-54", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "release"},
			summary: "发布候选件（危险 D3 · 本版未开放）",
			usage:   "zerg dev release --candidate <候选 id> --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把候选件推上生产面（**只能由人拍板开**；AI 不许自升）", "§17.4 · §九 M18 C4 · 开工单 T-58", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "rollback"},
			summary: "回滚（危险 D3 · 本版未开放）",
			usage:   "zerg dev rollback [--to <目标>] --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把生产面退回某个已知状态（回滚件到期前**永不自动**）", "§17.2 ⑦ · §十五.2 档③ · 开工单 T-58", true},
			run:     cmdGuarded,
		},
		{
			path:    []string{"update"},
			summary: "源码式自更新（危险 D3 · 本版未开放）",
			usage:   "zerg update --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单）", "§7.1 P12 · §九 M20 F-3 · 开工单 T-52", true},
			run:     cmdGuarded,
		},
		{
			path:     []string{"dev", "proposal"},
			kind:     "Proposal",
			summary:  "提案件通道：只产可审查物（new|list|show|check）· 目标必须回指既有编号 · **判据必须可机检** · 「提 ≠ 批」两对字段（subject/approver）",
			usage:    "zerg dev proposal new --title <题（必填）> --target <待办编号（必填 · 只收 D/E/F/G 族编号 · 逐条闭集真源 core/internal/contract/dev-targets.json）> --goal <目标（必填）> --evidence <出处（必填 · 至少一条）> --rollback <退点件（必填 · 仓外整件路径 —— 退建议照它核 sha256 取改前态；不是这一种形态的在提案这一步不拒，到退建议那一步才判不可退）> --criterion <判据（必填 · 一条首词是 zerg、本版真跑得动的命令）> [--by <提出者>] [--file <要改的件>]… [--subject <提出者>] [--subject-kind human|ai|egg|ci] [--egg-id <卵 id>] [--approver <批准者>] [--approver-kind human]",
			arity:    "any",
			args:     []string{"动作：new | list | show | check", "提案 id（show/check 才要）"},
			fields:   proposalFields,
			endpoint: "",
			run:      cmdDevProposal,
		},
		// ---- D3b 第四步（2026-09-21）：**人批通道**（`zerg approve` 一族 · §九 M18 `C4`②）----
		// 人签批准件 = `require_approval` 的逃生门；模型写得出件的字节，**写不出那枚签名**（口令只在人手里）。
		{
			path:     []string{"approve", "ls"},
			kind:     "Approval",
			summary:  "列人签批准件（逐件带上**验签判决**：验过 / 无签名 / 签名坏 —— 后两者不算批准）",
			usage:    "zerg approve ls [--json <字段>]",
			fields:   approveFields,
			endpoint: "",
			run:      cmdApprove,
		},
		{
			path:     []string{"approve", "show"},
			kind:     "ApprovalShow",
			summary:  "看一枚批准件的全貌 + 验签判决（消费者只认「验过」那一档）",
			usage:    "zerg approve show <工具名> [--json <字段>]",
			arity:    "any",
			args:     []string{"工具名"},
			fields:   approveFields,
			endpoint: "",
			run:      cmdApprove,
		},
		{
			path:    []string{"approve", "new"},
			kind:    "ApprovalSign",
			summary: "**人签**一枚批准件（要人在终端上敲口令；非交互会话一律拒）· 写不进即拒 · 同名不覆盖",
			usage:   "zerg approve new --tool <工具名> --by <人名> --note <理由> [--scope <范围>] [--self-test] [--dry-run | --confirm=<工具名> --yes]",
			arity:   "any",
			args:    []string{"工具名（--tool）", "人名（--by）"},
			fields:  approveNewFields,
			danger:  &dangerSpec{dangerD3, "工具名", "签一枚批准件（逃生门）—— 只作 require_approval 的放行凭据；人不在场时等于没签", "§九 M18 C4② · §17.6 SD7 · D3b 第四步", true},
			// `opened`: 真跑已开放（**人在终端上** + 操作员口令 ⇒ 签出批准件；非交互一律拒）。
			opened:   true,
			endpoint: "",
			run:      cmdApproveNew,
		},
		{
			path:     []string{"approve", "keygen"},
			kind:     "ApprovalKeygen",
			summary:  "生成**操作员密钥**（人在终端上设口令；私钥口令加密落盘，公钥给消费者验签）",
			usage:    "zerg approve keygen --by <人名>",
			arity:    "any",
			args:     []string{"人名（--by）"},
			endpoint: "",
			run:      cmdApproveKeygen,
		},
		// ---- D3b 第二步（2026-09-21）：**受控写面**（`zerg dev edit` · §17.3 铁律③）----
		// 默认干跑 · 只改提案声明过的件（越界写 ⇒ 2）· 逐条审计（写不进审计就不改件）· D3 档确认。
		{
			path:    []string{"dev", "edit"},
			kind:    "DevEdit",
			summary: "受控写入：只改**提案声明过**的件（越界写 ⇒ 2）· 默认干跑 · 一行一事件的审计（写不进审计就不改件）",
			usage:   "zerg dev edit --proposal <提案 id> --file <仓内相对路径> (--from <件> | --replace <件>) [--by <谁>] [--allow-cross-root <理由>] [--dry-run | --confirm=<本机名> --yes] （真写前置：一枚**人签批准件** <状态目录>/approvals/dev_edit.json —— 无件 / 手写件 / 它的 scope 不含本件 ⇒ 一律拒（退 2）；--dry-run 那一态不需要它）" + devEditDryRunOnlyNote,
			arity:   "any",
			args:    []string{"提案 id（--proposal）", "要改的件（--file · 必须在提案的 files[] 里）"},
			fields:  devEditFields,
			danger:  &dangerSpec{dangerD3, "本机名", "改仓内件（写工作树）—— 作用域 = 提案声明的件；审计一行一事件；回滚 = 提案退点 + git", "§17.3 铁律③ · §九 M3 C4/C5 · §4.1 K7 · D3b 第二步", true},
			// `opened`: 真跑已开放（`--confirm=<本机名> --yes` 齐 + 人签批准件 ⇒ 写工作树）。
			opened:   true,
			endpoint: "",
			run:      cmdDevEdit,
		},
		// ---- §17.4 第 2/3/5 条：候选区（批 E · T-58 · 真源 core/internal/contract/dev-candidate.json）----
		// 次序（§17.7 逐字「先有判据、再有自动化」）落成机检：build / test 找不到该候选的验收证据单 ⇒ 退码 2。
		{
			path:    []string{"dev", "build"},
			summary: "在**候选区**构建全套件（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev build --candidate <候选 id> [--scope go|rust|ui|all] [--dist] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区编出成套制品（底层就是 scripts/build/build-all.sh —— 不新造第二条构建路）", "§17.4 第 2 条 · §17.6 SD10-b · 开工单 T-58", true},
			run:     cmdDevBuild,
		},
		{
			path:    []string{"dev", "test"},
			summary: "跑候选件的测试集（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev test [--candidate <候选 id> | --pkg <包> [--run <正则>]] [--scope go|rust|ui|all] [--outdir D] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区跑测试集（底层 = make test + 门禁既有步，不新立判据）", "§17.4 第 3 条 · §17.7 次序 · 开工单 T-58", true},
			run:     cmdDevTest,
		},
		{
			path:     []string{"dev", "verify"},
			kind:     "DevEvidence",
			summary:  "合成一份验收证据单（只收证据、**不给「通过」的结论**）· 证据为空 ⇒ 2",
			usage:    "zerg dev verify --candidate <候选 id> [--results <结果表>] [--code-sha <sha>] [--node <名>] [--layer <档>] [--gate [--human-approval <名>]] [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id"},
			fields:   devVerifyFields,
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
			path:    []string{"script", "run"},
			summary: "跑一件脚本（危险 D3 · 本版未开放）",
			usage:   "zerg script run <脚本> --confirm=<脚本> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"脚本"},
			danger: &dangerSpec{dangerD3, "脚本",
				"按调用者权限执行脚本（**退码原样透传**；核心名硬占位）",
				"§6.3 S6 · §九 M18 C5 · 开工单 T-50", true},
			run: cmdGuarded,
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
		// ---- §十八.3 融合四件（批 E · T-59）----
		// 依赖次序（**不许倒**）：能力路由（T-41 的 id 规范化真源）→ `ask` → 意图 schema → `plan`/`apply`。
		{
			path:     []string{"ask"},
			kind:     "Ask",
			summary:  "问一次推理、**不落任务队列**（`task submit` 的对偶）· 能力筛是硬筛",
			usage:    "zerg ask <提示> [--capability 名]… [--prefer 名]… [--model 名] [--node 名]… [--min-ctx n] [--min-mem-gb n] [--no-fallback] [--dry-run] [--timeout 时长] [--json <字段>]",
			arity:    "any",
			args:     []string{"提示（一句话）"},
			fields:   askFields,
			endpoint: "GET /api/fleet/models · GET /api/models/registry",
			run:      cmdAsk,
		},
		{
			path:     []string{"plan"},
			kind:     "Plan",
			summary:  "**算**：产出一份意图件（M6 包封 · F1–F7 + 四附加件）· **零副作用**",
			usage:    "zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--target-ref <编号>] [--out <件>] [--json <字段>]",
			arity:    "any",
			args:     []string{"族（= target.kind）", "动作", "对象名"},
			fields:   planFields,
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
			path:     []string{"dev", "receipt"},
			kind:     "Receipt",
			summary:  "交接回执（一轮一页 · 带 trace_id）：new | ls | show；读法 = `zerg context ls --resume`",
			usage:    "zerg dev receipt new --what <做了什么> --next <下一步> [--evidence <证据>]… [--blockers <阻碍>]… [--commit <sha>]… [--trace <trace_id>]",
			arity:    "any",
			args:     []string{"动作：new | ls | show", "轮次 id（只 show 要）"},
			fields:   receiptFields,
			endpoint: "",
			run:      cmdDevReceipt,
		},
		// ---- 只读隔离（`GAP-20260926-20` P0 · 2026-09-26）：脚本面的**CLI 正门** · **顶层命令** ----
		// 底层唯一实现 = `scripts/dev/isolate-ro.sh`（只转发、不翻译；退码 0/1/2 原样转出）。
		// ★ 落**顶层**（父代理一拍）：① 与已登记名一致 —— 技能参考 `references/safe-isolation.md`
		//   与缺口账写的就是 `zerg isolate-ro new|verify`；② 不撞 standing 判据 —— `dev` 族被钉成
		//   「只增改环 · 读环一条都不新增」（`TestDevFamilyIsACHangedOnlyRingNoNewReadCommands`
		//   + 四道闸 `G2` 的七条闭集），那条属性是**有意的**；隔离是工具类正门、不是 `dev` 族的读环。
		// 名字分家：`new` = 造副本（写只落在副本/`--dst`）· `verify` = 只读逐件 sha256 验回。
		{
			path:        []string{"isolate-ro"},
			summary:     "只读隔离：「在别处试」的**硬拷贝**副本（new）/ 逐件 sha256 验回没碰真仓（verify）· 底层只调 scripts/dev/isolate-ro.sh（命令面不重写逻辑）",
			usage:       "zerg isolate-ro new <真仓> [--dst <目录>] [--include <glob>]… [--exclude <路径>]… [--all] [--reject-symlinks] [--no-manifest] [--quiet] | zerg isolate-ro verify <副本> <真仓> [--max-examples N] [--quiet] | zerg isolate-ro --self-test",
			arity:       "any",
			args:        []string{"动作：new（造副本 · 真仓只读）| verify（只读对拍）", "旗标与退码逐字来自 scripts/dev/isolate-ro.sh（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdIsolateRO,
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
			usage:   "zerg calib run <名> [位置参数…] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg calib ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  calibRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件标定脚本（会占机器/模型槽 · 出的是实测档案）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 标定线 3 件", false},
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
			usage:   "zerg eval run <名> [位置参数…] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg eval ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  evalRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件评测脚本（可能要跑着的生产面 · 占机器与模型槽）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 评测线 22 件", false},
			// `opened`: 真跑已开放（`--yes` 就真跑脚本；退码原样透传）。
			opened: true,
			run:    cmdEvalRun,
		},
		// ---- 变更影响面（设计-变更影响面-v1.6 §7.1/§7.3/§7.4 · 任务单-影响面实施-20260922 §二 `A1`–`A3` · 2026-09-22）----
		// `A1` = 骨架（人面三行 + 六键包封）；`A2` = 六层取数；`A3` = 波纹卡片（四字段 · why 闭集六选一 ·
		// 四级排序 · 按档裁 · §3.7 公开面行 · §3.8 可逆性行 · 两档 `--for-model`/`--for-human`）。
		// 挂干跑与缓存分别属 `A4`/`A5`。只读 ⇒ 不写 `danger`（走默认「只读」幂等档）、不改 `emitEnvelope`、
		// 不写缓存、不落审计。
		{
			path:     []string{"impact"},
			kind:     "Impact",
			summary:  "改一处会牵动谁（只读：人面三行 + 波纹卡片 ≤12 条/≤1.2k token + 六键包封；挂干跑属 `A4`）",
			usage:    impactUsageLine,
			arity:    "any",
			args:     []string{"目标（仓内件路径 · 或在册契约 id，如 S-g）"},
			fields:   impactFields,
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
				"缺口-命令面-20260921 §一 G-07 · RFC 8493 BagIt · 同族写面先例 `zerg gap export`（同样真写盘）· §九 M3 C1", false},
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
			usage:    "zerg model add --model <模型名> --host <主机> --file <GGUF 路径> [--backend …] [--mem-gb …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键 / 块名 ⇒ 按它定位或新建那一块）", "主机（--host：该条 `host:` 的字段值 ＝ 这台模型跑在哪台机器）", "GGUF 路径（--file）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "added"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "往名册件 `gateway/fleet.yaml` 的 `models:` 段加一条（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不删件、不改别人的条、不碰主控运行态）；可逆：删掉那一行即回原状", "缺口账 `GAP-20260927-342` · 同族写面先例 `zerg gap export`（真写盘）· `zerg gate matrix` · §九 M3 C1", true},
			opened:   true,
			endpoint: "",
			run:      cmdModelAdd,
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
			danger:   &dangerSpec{dangerD2, "（群级：不点名主机）", "让在跑的主控重读名册件（`POST /api/config/reload`）—— 生效面即时（路由表更新）；不重启、不停任何进程、已加载模型不受影响 · 名册件解析不过 ⇒ 一个请求都不发（旧配置继续跑）；可逆：再热加载一次即回旧档", "缺口账 `GAP-20260927-342` · 同族先例 `zerg core reload`（同为 D2）· §三 C 族 · §6.3 S5 · §九 M3 C1", false},
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
			usage:    "zerg gap ls [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--unit <件路径|目录前缀>] [--module <模块前缀>] [--json <字段>]",
			fields:   gapListFields,
			endpoint: "",
			run:      cmdGapLs,
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
			summary:  "缺口账**排行面**（只读）：按件（`--by unit` · 缺省）/ 按模块（`--by module`）/ 按**判据分级**（`--by tier`）分桶。件轴／模块轴：逐桶给 **未闭 / 已解 / 净**（可为负）+ 近邻量化（P0/P1/P2 计数）· 排序键 = ① 未闭降序 ② 近邻量化降序（都写进表头）· 对账等式自校（Σ(有件桶未闭) + 无件桶 == 账内仍缺 · 不成立 ⇒ `warnings` 点名差数）。**tier 轴**：行 = 三桶闭集（真判据 / 占位 / 无 —— `verify_tier` 已落库则用它，未落库则现读 `verify_cmd` 现算：空串/无键 ⇒ 无 · 含「占位」字样 ⇒ 占位 · 其余且能被命令树解析 ⇒ 真判据）· 列 = **条数 / 其中仍缺 / 其中已解** · 表尾固定两行：① 守恒式自校「三桶之和 == 总账」（两数各自现算 · 不等 ⇒ **判红 1** 并打印两数）② 占位桶**只报告不阻断**（不退码 · 先量分布）。全轴带 `query_ts` / `ledger_sha16`（不同时刻两个排行才可比）· 截断自报（`--top` 一页硬顶 = `gapLsRowCap`）· 读不到真源 ⇒ 8",
			usage:    "zerg gap status [--by <unit|module|tier>] [--top <N>] [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--json <字段>]",
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
			usage:    "zerg gap plan <件路径|模块目录前缀> [--module <模块前缀>] [--unit <件路径|目录前缀>] [--top <N>] [--json <字段>]",
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
		{
			path:     []string{"gap", "receipt", "apply"},
			kind:     "GapReceiptApply",
			summary:  "回执**闭案落账**（写面 · 缺 `--yes` fail-closed 2）：把 `closed_ids` 逐条改 `已解`（**走现有改态路径 `gapRewriteOne` · 不另开写路**）· 缺 `--evidence` ⇒ 2 · 有冲突号（账里非 `已派`）⇒ 2 且**不动账** · 成败后回显逐条改前→改后计数 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap receipt apply <回执件路径> --evidence <一句话> --yes [--by <谁>] [--json <字段>]",
			arity:    "any",
			args:     []string{"回执件路径（**恰好一条** JSON 件）"},
			fields:   gapReceiptApplyFields,
			danger:   &dangerSpec{dangerD2, "回执件（`closed_ids`）", "把回执 `closed_ids[]` 里的号在真源逐条改 `已解`（`state`/`solved_at`/`solved_evidence`）+ 审计逐条一行（可逆：照审计那一格回写）；缺栏 / 冲突号 ⇒ 2（一个字节都不写）", "批3 第四片（缺口账与自进化 · 2026-09-28）· 设计稿 §三十四 `O-12` + 施工清单 3-4 · 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapReceiptApply,
		},
		// ★ `gap show` —— **单条取全文**（2026-09-28 · 缺口账 `GAP-20260926-15` + 同族 `GAP-20260928-243`）：
		//   只读面（不登记 `danger` ⇒ 按只读幂等档列）· 点名面复用同族 `gapTargetOne`（**账内没有这个 id ⇒ 2**，
		//   与 `set-state` / `note` 同一口径）· 读不到真源 ⇒ 8。实现件 = `family_gap.go:cmdGapShow`。
		{
			path:     []string{"gap", "show"},
			kind:     "GapShow",
			summary:  "取一条缺口的**完整正文**（只读面 · 一字不截）：正文 / 手搓记录 / 复现 / 判据 / 已解证据 / 口径注全量出 —— `gap ls` 那一行只印摘要前 40 显示宽、`--json` 全量直出又会被读方截断（缺口账 `GAP-20260928-243`）· 账内没有这个 id ⇒ 用法错 2（同族口径 · 不跨族抄 `dev proposal show` 的 1）· 读不到真源 ⇒ 8",
			usage:    "zerg gap show <GAP id|件路径> [--json <字段>]（位置参数给一条件路径 = 按件反查：该件在账里被点到的条目逐条列出 · 件不在仓里 ⇒ 2 · 仓根取不到 ⇒ 8）",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本面只取一条）"},
			fields:   gapShowFields,
			endpoint: "",
			run:      cmdGapShow,
		},
		{
			path:     []string{"gap", "add"},
			kind:     "GapAdd",
			summary:  "记一条缺口（写面 · 留证据）：**手搓记录 + 验证命令是两件必填**（防呆⑤）· 同 fp 同内容 ⇒ 幂等命中 0 · 同 fp 内容不同 ⇒ `14` · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap add --symptom <一句> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] --repro-cmd <命令> --verify-cmd <命令> [--depends-on <fp>…] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapAddFields,
			danger:   &dangerSpec{dangerD2, "缺口 fp", "往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删）；审计写不进 ⇒ 真源一行不写", "设计-命令面-gap族-v1.0-20260923.md §二.2 · §四 · `H-10`（不要人签：`--yes` 是命令行确认档，不是批准件）", false},
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
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_evidence` / `last_verified_at` + 审计一行（可逆：照审计那一格回写）；判红（`回归`）只在真跑那一态可达", "设计-命令面-gap族-v1.0-20260923.md §二.3 · §三 · `H-10`（判据是机器给的：跑命令看 rc）", false},
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
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `last_verified_at` / `last_verify_rc` / `verify_tier` + 审计一行（可逆：照审计那一格回写）；**本面不改 `state`**（改态是 `gap verify` / `set-state` 的事）", "设计-缺口账与自进化-v2.0-20260928.md §11.4 批4 · §11.2 批2 D1/D2 · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四`", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerifyOne,
		},
		{
			path:     []string{"gap", "export"},
			kind:     "GapExport",
			summary:  "缺口真源 → 「结转账 markdown 片段」导出面（**只打 stdout 时零写盘**：不写真源、不写审计；**`--out <目录>` 那一支会拆件落盘** ⇒ 本表项按**写面**登记 `D2`）· **只列未闭**（`已解`/`不做` 不进正文，只进件头计数）· 件头 = 仪表盘（条数/未闭/已闭/逐面计数/最老未闭天数）· 单行 ≤200 字符 · 读不到真源 ⇒ 退 8（fail-closed）",
			usage:    "zerg gap export [--out <目录>] [--json <字段>] [--dry-run | --yes]",
			fields:   gapExportFields,
			danger:   &dangerSpec{dangerD2, "落点目录（`--out`）", "往 `--out <目录>` 拆件落盘（索引件 1 + 页件 N：`缺口总账-<日期>.md` / `-<面>-pNN.md`）；真源 `zerg-cli-gaps.jsonl` 与审计 `edit_audit.jsonl` **一个字不动**（可逆：删掉新落的那些件即回到原状）", "缺口账（2026-09-27）· `ddf57491` 接上 `--out` 真写盘 ⇒ 旧表项无 `danger`，本族写命令一直被 `zerg help` 判成**安全档** ✗ · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapExport,
		},
		// ★ 两条**状态写面**（2026-09-26 · 缺口账 `Q-239`：「`gap` 族只有 4 个动作 ⇒ 改态/写口径
		//   没有正门」）。照同族写面（`add` / `verify`）的式样：`--dry-run` 恒 0 / 缺 `--yes`
		//   fail-closed 2 / 审计先落盘（写不进就不写真源）/ `--by`-`--yes`-`--json` 习惯逐字沿用。
		//   实现件 = `family_gap_state.go`（本族**只这两条**用 `--evidence` / `--text` 两枚旗标）。
		{
			path:     []string{"gap", "set-state"},
			kind:     "GapSetState",
			summary:  "改一条缺口的状态（写面 · 留证据）：`--state` 只认**三值闭集**（`仍缺` / `已解` / `不做`）—— 闭集外 ⇒ 2 并逐字印闭集 · 转 `已解` 写 `solved_at` + `solved_evidence` · 同 id 同态同证据 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap set-state <GAP id> --state <仍缺|已解|不做> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量改态）"},
			fields:   gapSetStateFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_at` / `solved_evidence` + 审计一行（可逆：照审计那一格回写）；`--state` 闭集外 ⇒ 2（一个字节都不写）", "缺口账 `Q-239` · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetState,
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
			danger:   &dangerSpec{dangerD2, "整批缺口 id（清单件点名的那几条）", "按清单逐条改真源里的 `state` / `solved_at` / `solved_evidence`（逐条写自己的 evidence）+ 审计**逐条**一行（可逆：照审计那一格逐条回写）；任一条不合规 ⇒ 2 且**一个字节都不写**（预检在真写之前）", "批3 第五片（缺口账与自进化 · 2026-09-28）· 缺口账 `GAP-20260926-97`（本片治法逐字）· 同族写面先例 `gap set-state` / `gap assign`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapBulkSetState,
		},
		{
			path:     []string{"gap", "note"},
			kind:     "GapNote",
			summary:  "给一条缺口追加一条**口径/上下文注**（写面）：落真源那一行的 `notes` 数组（`<时刻> · <谁>：<文本>`）· **不改 `state`**（形状里根本没有 `--state` ⇒ 收到即拒 2）· 同 by 同 text 已在位 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）· `--retract <n|指纹>` 给已落的注打**作废标记**（`notes_void` 追加 · **禁真删**：原文逐字节留在 `notes` 里 · 读面默认不显示作废项 · `--json void_notes` 带原文出来）",
			usage:    "zerg gap note <GAP id> [--text <一句话> | --retract <n|指纹>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条**）"},
			fields:   gapNoteFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "往真源那一行的 `notes` 数组追加一条（可逆：删那一格的那一条）+ 审计一行；`state` 逐字不动（改前改后现算对拍）；`--retract <n|指纹>` 只**追加**一条作废标记（`notes_void` · 可逆：删标记那一条）—— **禁真删任何注**，读面按标记过滤", "缺口账 `Q-239` · 同族写面先例同 `gap set-state` · 「注不改态」是本条与 `set-state` 的分工线", false},
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
			summary:  "派单登记（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：把该条 `state` 改 **`已派`**（走现有改态路径 `gapRewriteOne` · 不另开写路）并在 `notes` 末尾追加**一行机读登记**（`派单登记 egg=… · receipt=… · allow=… · forbid=… · occupies=… · criterion=… · deadline=…`）· **作业单六栏缺栏拒发**（① `--allow` 允许面 ② `--forbid` 禁碰面 ③ `--occupies` 占用件 ④ `--criterion` 出口判据 ⑤ `--deadline` 时限 ⑥ 该条全文 —— 缺一栏 ⇒ 2 并点名缺哪栏）· 预检「可派状态」= 默认只 `仍缺`（非此 ⇒ 2 并点名当前 state）· `M-32` 占用预检（同件已被另一 `已派` 占住 ⇒ 2）· 同卵号登记已在位 ⇒ 「无变化」0",
			usage:    "zerg gap assign <GAP id> --egg <卵号> --allow <件清单> --forbid <面> --occupies <件|无> --criterion <出口判据> --deadline <绝对日期> [--receipt <回执绝对路径>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 一次只派一条 · 本动作不做批量）"},
			fields:   gapAssignFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `已派`）+ 往 `notes` 追加一行派单登记 + 审计一行（可逆：照审计那一格回写 · 登记那一行删掉即回原状）；六栏缺一栏 / 非可派状态 ⇒ 2（一个字节都不写）", "批3 第一片（缺口账与自进化 · 2026-09-28）· 设计稿 §11.3 + §4 判据 2 + §8 `M-32` · 同族写面先例 `gap set-state` / `gap note`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false},
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
			usage:    "zerg gap backfill-unit [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = **整本账** · 本片不收窄）"},
			fields:   gapBackfillFields,
			danger:   &dangerSpec{dangerD2, "整本缺口账（`<状态目录>/zerg-cli-gaps.jsonl`）", "逐条**只加** `unit` / `module` / `unit_source` 三键（其余键名与值逐字节不变 · 行序不变 · 末行换行不变）+ 审计一行（可逆：删掉新加的那三键即回原状；`--dry-run` 那一态零副作用）", "批1 第二片（存量 891 条回填 · 2026-09-28）· 同族写面先例 `gap add` / `gap verify` / `gap set-state` / `gap note` · `设计-命令面-gap族-v1.0-20260923.md §三 三态纪律` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false},
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
			usage:    "zerg gap idea add --symptom <一句> [--source <guard|gate|egg|human>] [--unit <件路径>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapIdeaFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）", "往**候选池**追加一行（可逆：删那一行）+ 顺带把落池超七天的条目搬进同目录归档件；**真源 `zerg-cli-gaps.jsonl` 与审计一个字节都不动**（本面不碰真源 —— 入账仍走 `gap add`）", "设计-缺口账与自进化-v2.0-20260928.md §11.3 + §9.1 第 1/2 条 · 施工清单 `任务清单-缺口账自进化-施工-20260928.md` §3 的 2-1/2-4 · 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false},
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
			usage:    "zerg gap idea promote <cid> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] --repro-cmd <命令> --verify-cmd <命令> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条**）；其余六必填与本族 `gap add` 逐字同款（症状取自池件那一格 `raw`）"},
			fields:   gapIdeaPromoteFields,
			danger:   &dangerSpec{dangerD2, "候选 id（池内一条）+ 缺口 fp", "走 `gap add` 那条路往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删），并把池件那一行置 `status=promoted` + 记 `gap_id`（池行**不删**）；池件写不进 ⇒ 退 8 并如实报「正式账已落」", "设计-缺口账与自进化-v2.0-20260928.md §11.3（`promote` **复用 `gap add` 的既有校验（不许绕）**）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false},
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
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）点名那一行", "把池内该行 `status` 置 `expired`（可逆：照 `gap idea show` 的读数改回 `pending`）；池行**不删**、真源 `zerg-cli-gaps.jsonl` 与审计**一个字节不动**", "设计-缺口账与自进化-v2.0-20260928.md §11.3（出口面 · `status` 四值 `pending|promoted|expired|merged`）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add` / `gap idea add`", false},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaDiscard,
		},
		{
			path:     []string{"gap", "idea", "stats"},
			kind:     "GapIdeaStats",
			summary:  "设计稿 §4 判据 1 的读数面（**只报告、不进退码**）：池内各 `status` 计数 + **来源分布**（`source` 四值）+ **候选→入账转换率**（`promoted / (promoted+expired+pending)`，分母逐字写出）· 只读（不写池、不写真源、不写审计）· 池件不在盘 ⇒ 8",
			usage:    "zerg gap idea stats [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = 整池）"},
			fields:   gapIdeaStatsFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaStats,
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
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
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
		// `ask` 的 `--no-fallback`（缺口 `GAP-20260928-127` 的 ⑥ · 本枚）：用法串写**裸形**、语义本就是
		// **布尔档** ⇒ 从「按取值收」的名字表移到布尔这一面（与上一块逐字同一种形态：写进 `inv.kv`、
		// `foreignFlag` 看得见）。裸形与 `--k=v` 两态都收 ⇒ 行为逐字不变。
		case a == "--no-fallback":
			inv.kvSet(a, "true")
		case strings.HasPrefix(a, "--no-fallback="):
			inv.kvSet("--no-fallback", strings.TrimPrefix(a, "--no-fallback="))
		// ★ 批3 第三片（2026-09-28）：`gap assign ls` 的两枚**布尔**面旗标 —— `--aging`（按超时限程度
		//   降序 · 时限算不出的单列「时限不明」）/ `--stale`（只列超时限未收的派单）。与上一块
		//   `--count`/`--files-only` 逐字同一种形态：写进 `inv.kv`（`kvSet`）⇒ `foreignFlag` 看得见；
		//   ★ 必须在下面「未知旗标」兜底**之前**上户口，否则派单里写着它们会一律退 2「未知旗标」。
		case a == "--aging" || a == "--stale":
			inv.kvSet(a, "true")
		case a == "--help" || a == "-h":
			inv.wantHelp = true
		case a == "--version":
			inv.wantVersion = true
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
	case "--desc", "--model", "--priority", "--slice-id", "--depends-on", "--acceptance", "--to", "--set":
		return a
	}
	// 自开发面旗标（批 E · T-57 起）：一族共用一张名字表 —— 值照收，语义在各自命令里判。
	// ★ 2026-09-28（缺口 `GAP-20260928-127` 的 ②③④ · 本枚）：`--criteria` / `--round` /
	//   `--producer` 三枚**幽灵死条目**从本表除去 —— 全包零引用 + 全部用法串零提及
	//   = 给不存在的旗标发户口（门73 判据乙「多一处」）。同日门73 存量豁免表同步除名。
	switch a {
	case "--title", "--target", "--goal", "--evidence", "--rollback", "--by",
		"--criterion", "--candidate", "--state",
		"--dir", "--expect":
		return a
	}
	// 受控写面与提交面旗标（D3b 第二/三步 · 2026-09-21）：受控写入的件名、提交信息、
	// 审批的件名与理由 —— 与上面同一张名字表的口径（值照收，语义在各自命令里判）。
	// ★ 2026-09-28（缺口 `GAP-20260928-79` · 本枚）：续 `--allow-cross-root <理由>` 一枚 ——
	//   `dev edit` **异源拒写档的显式放行闸**（理由进审计行 · 语义在 `cmdDevEdit` 里判；
	//   本排是名字表「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用）。
	switch a {
	case "--file", "--message", "--proposal", "--tool", "--note", "--from", "--replace", "--root",
		"--allow-cross-root":
		return a
	}
	// 产出树面旗标（`W-50` · 任务单序132 · 2026-09-24 波17）：`--tree <树>`（**可重复** ——
	// 一树一行、两代树两行）。与上面几排同一口径：值照收，语义在各自命令里判
	// （`publish tree has` 判「点没点名」「树在不在盘」「树身份读得到吗」）。
	switch a {
	case "--tree":
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
	// 名册面旗标（波① `T1a` · `Q-099`：`model add` 往 `gateway/fleet.yaml` 写一条）。
	// 与上面几排同一口径：值照收，语义在各自命令里判（`model add` 逐个校验）。
	switch a {
	case "--host", "--backend", "--ctx", "--mem-gb", "--arch", "--mmproj", "--added":
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
	// ★ 2026-09-28（缺口 `GAP-20260928-127` 的 ⑥ · 本枚）：`--no-fallback` **从本表移出** ——
	//   它的语义本就是**布尔档**（用法串只写裸形 `[--no-fallback]`），按「取值」收进本表
	//   是**形态不统一**（门73 判据丙）。现在与 `--count`/`--files-only` 同一种形态
	//   （解析面布尔 case + 写进 `inv.kv`）⇒ 两处登记面**同名同形态**。
	// 事件面旗标（§九 M1 · `W11` · 缺口 `GAP-20260928-128` · 本枚）：`zerg watch` 的用法串写着
	//   `[--exit-on <kind>]`（跟到某个事件就退），而本表此前**没有**它 ⇒ 照用法串真给就退 2
	//   「未知旗标」= **承诺与解析面不符**。本排只补「认得」这一件（§九 M1 `W11` 的语义仍归
	//   事件面：主控面今天没有 `/api/events` ⇒ `zerg watch` 照旧出声明面 + 不给结论，退码 8）。
	switch a {
	case "--exit-on":
		return a
	}
	// 度量与排序面旗标（组1 序12 · `zerg metrics`）：读数档的落点。
	// 与上面几排同一口径：值照收，语义在命令里判（本命令只把它当一个**只读**的件路径）。
	switch a {
	case "--input":
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
	case "--waive", "--reason":
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
	switch a {
	case "--path", "--glob":
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
		// ★ 批1 第三片：`gap ls` 两轴收窄 —— `--unit <件路径|目录前缀>` / `--module <模块前缀>`。
		//   名字表是本仓「唯一真源」⇒ 不登记就一律退 2「未知旗标」= 新旗标等于不可用（同族先例：
		//   `--allow-cross-root` 那一排的注释逐字同病）。用法串与本表**必须同改**（门判据乙）。
		"--unit", "--module",
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
		"--source":
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

// emitSelected 是**全部** `--json` 出口的唯一实现：先按用户点名的字段（序即用户给的序）拼对象，
// 再套包封。字段点错 ⇒ 退码 2 + 列全部合法字段（§九 M6 I5）。
func emitSelected(stdout, stderr io.Writer, inv *invocation, path []string, fields []string, rows []map[string]string) int {
	cmd := find(path)
	if cmd == nil {
		return exitUsage
	}
	objs := make([]string, 0, len(rows))
	for _, row := range rows {
		obj, bad := marshalObject(fields, row)
		if bad != "" {
			// ★ 2026-09-28：`--json <未知字段>` 的失败面 `error.detail` 原**恒缺**。
			//   病灶：`reportBadField` 只写 stderr、**不调 `setErr`** ⇒ 走到 `emitErrIfJSON` 时
			//   `inv.err == nil`，包封退化成兜底那一格（`main.go:286`：kind 由真退码派生、
			//   message 写「（命令未报出 kind，按退码兜底）」）—— 机器面拿不到**真因**，
			//   AI 自愈只剩退码（与 `requireFields` 的 `GAP-20260927-360` 逐字同病）。
			//   治法与 `main.go:2740` 那处**同一条口子、同 kind**（`errors.go:138` 的 `setErr`：
			//   谁先报谁为准 · 不打第二枪），只补上 detail 的赋值 —— 点名那个未知字段。
			//   人面一字不动：下面 `reportBadField` 照旧写 stderr，退码仍由它回（`exitUsage` = 2）。
			inv.setErr("usage", "json_field_unknown:"+bad, fmt.Sprintf("未知字段 %q", bad))
			return reportBadField(stderr, path, bad)
		}
		objs = append(objs, obj)
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
//	   主题面赢（既有面一字不动），族面只兜主题表没有的名字（`model` / `egg` / `impact` …）。
//	③ 族名**认不出** ⇒ 返回 `done=false`，交回主题面报「未知帮助主题」+ 列可用主题（K14 第三件），
//	   退码照旧 `2` —— 不新增第三种错误形状。
//
// 内容真源 = 命令树（`catalog()`，名字序稳定）：逐条**用法行逐字**来自 `command.usage`，
// 危险档成员照实带档位标记（不隐藏、也不与 `help dangerous` 打架 —— 那一条判的是三态，这里只标档）。
func renderFamilyHelp(family string, stdout io.Writer) (int, bool) {
	var members []*command
	for _, c := range catalog() {
		if len(c.path) > 0 && c.path[0] == family {
			members = append(members, c)
		}
	}
	if len(members) == 0 {
		return exitOK, false
	}
	open, danger, refused := 0, 0, 0
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
	// 计数口径（缺口 序33 · 2026-09-24 已拍）：**三档各归各的** —— 已开放 / 拒执 / 危险档，
	// 三者之和恒等于 `len(members)`（修前拿 `danger == nil` 当「已开放」的代理 ⇒ 真跑拒执的那条
	// 既算「已开放」、又不进危险档 ⇒ **双计**，而 6 = 3 + 3 看着还是自洽的 ⇒ 肉眼看不出病）。
	// 「已开放」那一档的逐条真源 = `openedForRun`（本函数**不再自己判一次**）。
	fmt.Fprintf(stdout, "%s %s —— 族级用法（动作 %d 条 · 已开放 %d · 拒执 %d · 危险档 %d）\n",
		progName, family, len(members), open, refused, danger)
	fmt.Fprintf(stdout, "（本页只从这个族名出帮助；逐条 `%s %s <动作> --help` 出该条用法串。名字与用法逐字来自命令树。）\n\n",
		progName, family)
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
	fmt.Fprintf(stdout, "\n见 '%s help' 看命令树 · '%s help dangerous' 看危险档三态 · '%s help <主题>' 看专题。\n",
		progName, progName, progName)
	return exitOK, true
}

func cmdHelp(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		// 主题表在 `topics.go`（**唯一真源**：分派与「可用主题」列清单同一处）。
		// ★ 2026-09-24（缺口 `Q-149`）：主题表**先判**（表里有的名字一字不动）；表里没有 ⇒ 再看
		//   「**族名**」（命令树里某条命令路径的第一段）—— 族级帮助此前没有出口，见 `renderFamilyHelp`。
		if !isHelpTopic(inv.args[0]) {
			if rc, done := renderFamilyHelp(inv.args[0], stdout); done {
				return rc
			}
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
