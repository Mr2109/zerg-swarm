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
		if s := nearest(inv.path[0]); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s %s\n", progName, s)
		}
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
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
	if rc != exitOK && inv.jsonGiven && cw.n == 0 && (len(inv.fields) > 0 || cmd.danger != nil) {
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
	if len(inv.fields) == 0 && (cmd == nil || cmd.danger == nil) {
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
			fields:   []string{"name", "verdict", "detail", "advice"},
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
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "last_seen"},
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
		// ---- 批 A · S3 门禁直通四条（**只转发、不翻译** · §6.3 S3 判据③）----
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
			usage:       "zerg gate run [--scope <s> | --fast] [--outdir <目录>] … | zerg gate run --step <步名> [--self-test] [--json <字段>]",
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
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "last_seen"},
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
		{
			path:    []string{"agent", "bootstrap"},
			summary: "子端引导（危险 D3 · 本版未开放）",
			usage:   "zerg agent bootstrap <机器名> --confirm=<机器名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"在目标机上装/起子端（**F-1 子端引导例外**：不经主控的显式命令；会改目标机状态）",
				"§十五.4 F-1 · §九 M20 · 开工单 T-43"},
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
			danger:  &dangerSpec{dangerD2, "任务 id", "改这条任务在队列里的次序（可能插到别人前面）", "§三 D 族 · 开工单 T-44"},
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
			danger:  &dangerSpec{dangerD3, "主机名", "起主控进程（会绑端口 8580；已在跑时是**换件**前置）", "§三 C 族 · §7.1 P11 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "restart"},
			summary: "重启主控（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`）· 审计留痕 + 就绪判据绑自己的 pid",
			usage:   "zerg core restart --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停 + 起主控（**整个虫群的控制面会断一会儿**）", "§三 C 族 · 开工单 T-45 · 缺口 Q-103"},
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
			danger:   &dangerSpec{dangerD2, "台账件", "把仓外台账的行面与两个 yes/no 列按现跑重算（可逆：写前备份 + 写后读回，复查不过逐字节写回）；**一条命令写**、**不许第二条写路径**", "设计-仓外台账写面-A3-b案-v1.0-20260923.md §二 · `A3` = `G-13`/`Q-002`"},
			opened:   true,
			endpoint: "",
			run:      cmdScriptInventorySync,
		},
		// ---- 缺口面 P0（缺口-命令面-20260921 §十一 · 2026-09-21）：今天手搓最多的一类先补上 ----
		{
			path:     []string{"code", "find"},
			kind:     "CodeFind",
			summary:  "在码里找一处东西在哪（只读取证 · 手搓 grep/git grep 的替身）",
			usage:    "zerg code find <正则> [--path <子目录>] [--glob <模式>] [--json <字段>]",
			arity:    "any",
			args:     []string{"正则（POSIX 语法）"},
			fields:   []string{"path", "line", "text"},
			endpoint: "",
			run:      cmdCodeFind,
		},
		{
			path:     []string{"code", "show"},
			kind:     "CodeShow",
			summary:  "看源码里**某一行**长什么样（带 `件:行` · 只读取证 · 手搓 `sed -n` / `awk` 的替身）",
			usage:    "zerg code show <件:行> [--ctx <N>] [--json <字段>]",
			arity:    "any",
			args:     []string{"件:行（件 = 仓相对路径 · 行 = 正整数）"},
			fields:   []string{"path", "line", "text", "target"},
			endpoint: "",
			run:      cmdCodeShow,
		},
		{
			path:     []string{"repo", "status"},
			kind:     "RepoStatus",
			summary:  "看仓脏没脏 / HEAD 在哪 / 有没有别人在写它（手敲 git status 的替身）· `--root` 给 ≥2 次 ⇒ **多仓汇总**（一条只读命令出两仓 HEAD + 脏件数；写旗标一律拒 2）",
			usage:    "zerg repo status [--root <仓根>] [--json <字段>]",
			fields:   []string{"head", "branch", "path", "status", "untracked", "mtime", "sha256"},
			endpoint: "",
			run:      cmdRepoStatus,
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
			danger:  &dangerSpec{dangerD2, "提交主题", "把点名的件提交（可逆：`git reset --soft HEAD~1`）；**先跑快速档**，rc≠0 不提交（要带账放行得 `--waive <步名> --reason <…>`）", "缺口-命令面 §九 I4 · §九 M3 C5 · D3b 第三步 · 缺口 Q-104"},
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
		{
			path:     []string{"gate", "matrix"},
			kind:     "GateMatrix",
			summary:  "命令面自己的 must-fail 矩阵（逐格可读可导 —— 新增命令照着它补格）",
			usage:    "zerg gate matrix [--out <件>] [--json <字段>]",
			fields:   []string{"command", "case", "want_rc", "why"},
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
				"缺口-命令面-20260921 §八 H2 · 开工记录 D3 §三 新增 1 条 · D3③-a 的等价命令"},
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
				"设计-指定机器路由-v1.0-20260924.md §4 · 单独定制 > 路由默认规则（个别要求一次性 + TTL）"},
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
				"设计-指定机器路由-v1.0-20260924.md §4 · 「一行撤回」那条约束的落点"},
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
			path:    []string{"egg", "run"},
			summary: "把卵跑起来（**写面本版未开放**：连干跑一道押后、真跑退码 8；只读投影见 `egg ls` / `egg show`）",
			usage:   "zerg egg run <卵 id> [--yes | --dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			refuses: true, // 真跑拒执（`cmdEggRun` 无条件退 8 · kind=blocked —— 连干跑一道押后）
			run:     cmdEggRun,
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
			danger:  &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎开始按周期跑（会自动占机器与模型槽）", "§7.1 P8 · §5.1 POST /api/internal-tasks/start · 开工单 T-49"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"itask", "stop"},
			summary: "停内部任务引擎（危险 D2 · 本版未开放）",
			usage:   "zerg itask stop --yes [--dry-run]",
			danger:  &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎停下（在跑的进化任务会跑到当前一轮为止）", "§7.1 P8 · §5.1 POST /api/internal-tasks/stop · 开工单 T-49"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"itask", "run"},
			summary: "手动跑一条内部任务（危险 D2 · 本版未开放）",
			usage:   "zerg itask run <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "立刻跑一次该内部任务（绕过它的冷却 · 会占机器）", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/run · 开工单 T-49"},
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
			summary: "打包发布件（**计划面已开放**：`--dry-run` 出计划件；换件档真跑本版未开放 · 拒执退码 2）",
			usage:   "zerg build release [--dry-run | --confirm=<主机名> --yes]",
			refuses: true, // 真跑拒执（`cmdBuildPassthrough` 的 `openForExec` 只对 `build all --only cli` 为真 ⇒ 发布档非 dry-run 一律 `not_opened` 退 2）
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
				"§三 D 族 · §4.1 K7 · §九 M3 C1/C4 · 开工单 T-44"},
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
				"§三 D 族 · §4.1 K7 · 开工单 T-44"},
			run: cmdGuarded,
		},
		{
			path:    []string{"task", "pause"},
			summary: "暂停任务（危险 D2 · 本版未开放）",
			usage:   "zerg task pause <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把排队中的任务置为暂停态（可 resume 回来）", "§三 D 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "resume"},
			summary: "继续任务（危险 D2 · 本版未开放）",
			usage:   "zerg task resume <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把暂停的任务放回排队（可能立刻占机器）", "§三 D 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "retry"},
			summary: "重跑任务（危险 D2 · 本版未开放）",
			usage:   "zerg task retry <任务 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把 failed 任务置回 queued（会再占一次机器与模型槽）", "§三 D 族 · §6.3 S5"},
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
				"§三 B 族 · §九 M4 · 开工单 T-43"},
			run: cmdGuarded,
		},
		{
			path:    []string{"agent", "load"},
			summary: "加载模型到子端（危险 D2 · 本版未开放）",
			usage:   "zerg agent load <机器名> <模型> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名", "模型"},
			danger:  &dangerSpec{dangerD2, "机器名", "把模型装进该子端（占内存/显存槽 · 单槽机是串行的）", "§三 B 族 · §九 M5 · 开工单 T-43"},
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
				"§九 M9 · §十五.2 三档回收权 · RC11 红线 · 开工单 T-54"},
			run: cmdAgentReap,
		},
		{
			path:    []string{"model", "stop"},
			summary: "停模型（危险 D3 · 本版未开放）",
			usage:   "zerg model stop <模型 id> --confirm=<模型 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD3, "模型 id", "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500", "§三 E 族 · §九 M4 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"model", "start"},
			summary: "起模型（危险 D2 · 本版未开放）",
			usage:   "zerg model start <模型 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD2, "模型 id", "把模型装载起来（占槽位 · 单槽机要排队）", "§三 E 族 · §九 M5 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "stop"},
			summary: "停主控（危险 D3 · 本版未开放）",
			usage:   "zerg core stop --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停掉主控进程（**整个虫群的控制面会断**）", "§三 C 族 · §7.1 P11 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "reload"},
			summary: "重载主控配置（危险 D2 · 本版未开放）",
			usage:   "zerg core reload --yes [--confirm=<主机名>] [--dry-run]",
			danger:  &dangerSpec{dangerD2, "主机名", "让主控重读配置（规则表/名册）—— 生效面即时", "§三 C 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "update"},
			summary: "主控自身换件（危险 D3 · 本版未开放）",
			usage:   "zerg core update --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执）", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "pin"},
			summary: "钉住一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg pin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的）", "§3.4 I 族 · 开工单 T-47"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "unpin"},
			summary: "解钉一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg unpin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "取消在孵标记（原本占有单槽的卵会被换下）", "§3.4 I 族 · 开工单 T-47"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "retire"},
			summary: "退役一枚卵（危险 D3 · 本版未开放）",
			usage:   "zerg egg retire <卵 id> --confirm=<卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD3, "卵 id", "把该卵从名册与盘上退掉（**不可逆**；档 ③ 件永不自动）", "§十五.2 档③ · 开工单 T-54"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "release"},
			summary: "发布候选件（危险 D3 · 本版未开放）",
			usage:   "zerg dev release --candidate <候选 id> --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把候选件推上生产面（**只能由人拍板开**；AI 不许自升）", "§17.4 · §九 M18 C4 · 开工单 T-58"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "rollback"},
			summary: "回滚（危险 D3 · 本版未开放）",
			usage:   "zerg dev rollback [--to <目标>] --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把生产面退回某个已知状态（回滚件到期前**永不自动**）", "§17.2 ⑦ · §十五.2 档③ · 开工单 T-58"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"update"},
			summary: "源码式自更新（危险 D3 · 本版未开放）",
			usage:   "zerg update --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单）", "§7.1 P12 · §九 M20 F-3 · 开工单 T-52"},
			run:     cmdGuarded,
		},
		{
			path:     []string{"dev", "proposal"},
			kind:     "Proposal",
			summary:  "提案件通道：只产可审查物（new|list|show|check）· 目标必须回指既有编号 · **判据必须可机检** · 「提 ≠ 批」两对字段（subject/approver）",
			usage:    "zerg dev proposal new --title <题> --target <待办编号> --goal <目标> --evidence <出处> --rollback <退点> --criterion <可跑的判据> [--file <要改的件>]… [--subject <提出者>] [--subject-kind human|ai|egg|ci] [--egg-id <卵 id>] [--approver <批准者>] [--approver-kind human]",
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
			usage:   "zerg approve new --tool <工具名> --by <人名> --note <理由> [--scope <范围>] [--self-test]",
			arity:   "any",
			args:    []string{"工具名（--tool）", "人名（--by）"},
			fields:  approveNewFields,
			danger:  &dangerSpec{dangerD3, "工具名", "签一枚批准件（逃生门）—— 只作 require_approval 的放行凭据；人不在场时等于没签", "§九 M18 C4② · §17.6 SD7 · D3b 第四步"},
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
			usage:   "zerg dev edit --proposal <提案 id> --file <仓内相对路径> (--from <件> | --replace <件>) [--by <谁>] [--dry-run | --confirm=<主机名> --yes]",
			arity:   "any",
			args:    []string{"提案 id（--proposal）", "要改的件（--file · 必须在提案的 files[] 里）"},
			fields:  devEditFields,
			danger:  &dangerSpec{dangerD3, "提案 id", "改仓内件（写工作树）—— 作用域 = 提案声明的件；审计一行一事件；回滚 = 提案退点 + git", "§17.3 铁律③ · §九 M3 C4/C5 · §4.1 K7 · D3b 第二步"},
			// `opened`: 真跑已开放（`--confirm=<主机名> --yes` 齐 + 人签批准件 ⇒ 写工作树）。
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
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区编出成套制品（底层就是 scripts/build/build-all.sh —— 不新造第二条构建路）", "§17.4 第 2 条 · §17.6 SD10-b · 开工单 T-58"},
			run:     cmdDevBuild,
		},
		{
			path:    []string{"dev", "test"},
			summary: "跑候选件的测试集（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev test [--candidate <候选 id> | --pkg <包> [--run <正则>]] [--scope go|rust|ui|all] [--outdir D] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区跑测试集（底层 = make test + 门禁既有步，不新立判据）", "§17.4 第 3 条 · §17.7 次序 · 开工单 T-58"},
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
				"§6.3 S6 · §九 M18 C5 · 开工单 T-50"},
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
			usage:    "zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--out <件>] [--json <字段>]",
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
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 标定线 3 件"},
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
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 评测线 22 件"},
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
		{
			path:     []string{"archive", "manifest"},
			kind:     "ArchiveManifest",
			summary:  "出归档**三件套**（RFC 8493 BagIt：载荷 `data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）· `--dry-run` 先出逐件清单 · 真写要 `--yes` · 失败回滚",
			usage:    "zerg archive manifest <载荷目录> --out <袋目录> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"载荷目录", "袋落点（--out）"},
			fields:   []string{"bag", "entry", "sha256"},
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
		{
			path:     []string{"model", "add"},
			kind:     "ModelAdd",
			summary:  "往名册件（`gateway/fleet.yaml`）**先校验后写**加一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚 · 不覆盖别人的条）——**块按 `--model`（模型名）定位/新建**（列表形与裸映射形都认）· `--host` 只作该条的 `host:` 字段值",
			usage:    "zerg model add --model <模型名> --host <主机> --file <GGUF 路径> [--backend …] [--mem-gb …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键 / 块名 ⇒ 按它定位或新建那一块）", "主机（--host：该条 `host:` 的字段值 ＝ 这台模型跑在哪台机器）", "GGUF 路径（--file）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "added"},
			endpoint: "",
			run:      cmdModelAdd,
		},
		{
			path:     []string{"config", "reload"},
			kind:     "ConfigReload",
			summary:  "热加载主控配置（照 `nginx -s reload`：**先校验、失败回滚**）—— 名册件本地解析不过 ⇒ **不发请求**（旧配置继续跑）",
			usage:    "zerg config reload [--dry-run] --yes [--json <字段>]",
			arity:    "none",
			args:     []string{"（无名册件参数：走 `--root` / `--path` 或仓根）"},
			fields:   []string{"status", "models", "added", "fleet_nodes", "fingerprint"},
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
			usage:    "zerg gap ls [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--json <字段>]",
			fields:   gapListFields,
			endpoint: "",
			run:      cmdGapLs,
		},
		{
			path:     []string{"gap", "add"},
			kind:     "GapAdd",
			summary:  "记一条缺口（写面 · 留证据）：**手搓记录 + 验证命令是两件必填**（防呆⑤）· 同 fp 同内容 ⇒ 幂等命中 0 · 同 fp 内容不同 ⇒ `14` · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap add --symptom <一句> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] --repro-cmd <命令> --verify-cmd <命令> [--depends-on <fp>…] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapAddFields,
			danger:   &dangerSpec{dangerD2, "缺口 fp", "往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删）；审计写不进 ⇒ 真源一行不写", "设计-命令面-gap族-v1.0-20260923.md §二.2 · §四 · `H-10`（不要人签：`--yes` 是命令行确认档，不是批准件）"},
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
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_evidence` / `last_verified_at` + 审计一行（可逆：照审计那一格回写）；判红（`回归`）只在真跑那一态可达", "设计-命令面-gap族-v1.0-20260923.md §二.3 · §三 · `H-10`（判据是机器给的：跑命令看 rc）"},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerify,
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

	// 危险动作三态（§4.1 K7 · §九 M3 C1/C2/C4）
	dryRun bool
	all    bool // `build show --all`
	fast   bool // `gate bench --fast`
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
			// `--depends-on` / `--acceptance` / `--to`。值**原样**收下（语义校验在各自命令里）。
			// `--acceptance` 单独给（后面没跟值）也算「**声明了**」—— 「没写」与「写了空」
			// 在 API 那边是两件事（`nil` vs `[]`），不许在这里合并。
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				inv.setValueFlag(a, args[i])
			} else if a == "--acceptance" {
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

// valueFlagName 认「动作面旗标」的名字（**唯一真源**：解析与 `--k=v` 分派都读它）。
func valueFlagName(a string) string {
	switch a {
	case "--desc", "--model", "--priority", "--slice-id", "--depends-on", "--acceptance", "--to", "--set":
		return a
	}
	// 自开发面旗标（批 E · T-57 起）：一族共用一张名字表 —— 值照收，语义在各自命令里判。
	switch a {
	case "--title", "--target", "--goal", "--evidence", "--rollback", "--by",
		"--criterion", "--criteria", "--candidate", "--state", "--round",
		"--dir", "--producer", "--expect":
		return a
	}
	// 受控写面与提交面旗标（D3b 第二/三步 · 2026-09-21）：受控写入的件名、提交信息、
	// 审批的件名与理由 —— 与上面同一张名字表的口径（值照收，语义在各自命令里判）。
	switch a {
	case "--file", "--message", "--proposal", "--tool", "--note", "--from", "--replace", "--root":
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
	case "--capability", "--prefer", "--min-ctx", "--min-mem-gb", "--no-fallback",
		"--timeout", "--out", "--target-ref":
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
	// 缺口族旗标（`gap` 族 · 设计稿 `设计-命令面-gap族-v1.0-20260923.md` §二）：形状面 7 枚 + 判据面 1 枚。
	//   `--want-argv` 可重复（append）；`--state` / `--depends-on` / `--by` / `--dry-run` / `--yes` / `--json`
	//   已在上面各排（本族**不重开**同名旗标 —— 一族共用一张名字表）。
	switch a {
	case "--symptom", "--handmade", "--impact", "--want-family", "--want-action", "--want-argv",
		"--prio", "--repro-cmd", "--verify-cmd":
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
//	① 只**加出口**：命令树计数（119）不动 —— 这不是一条新命令，是 `help` 这条既有命令在
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
	return exitOK
}
