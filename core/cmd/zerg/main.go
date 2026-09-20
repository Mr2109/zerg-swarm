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
		// §4.1 K14 四件套：自报是谁 · 下一步 · 最像的合法输入 · 上下文定位。
		fmt.Fprintf(stderr, "%s: 未知命令 %q\n", progName, strings.Join(inv.path, " "))
		if s := nearest(inv.path[0]); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s %s\n", progName, s)
		}
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	inv.path = cmd.path
	inv.args = append(rest, inv.args...)
	inv.tty = ttyOf(stdout)
	if len(inv.unknown) > 0 && !cmd.passthrough {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, inv.unknown[0])
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
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
			emitErrIfJSON(inv, stdout, cmd)
			return exitUsage
		}
	}
	// 对象级作用域（§九 M13）：`Z1`/`Z3`/`P-066`/`P-067` 三条在**执行之前**判。
	if rc, done := enforceNodeRules(inv, cmd, stderr); done {
		emitErrIfJSON(inv, stdout, cmd)
		return rc
	}
	// 非交互凭据（`C2`）：本轮的 `--token-stdin` 交给**唯一入口**去读（只读一次）。
	if inv.tokenStdin {
		stdinTokenWanted = true
	}
	cw := &countingWriter{w: stdout}
	rc := cmd.run(inv, cw, stderr)
	// `--json <字段>` 的失败路径：把**机器可读**的 `error` 块挂进包封（§九 M7）——
	// 只在命令自己没往 stdout 写结果时补（写了结果就不改它，避免两个面打架）。
	if rc != exitOK && inv.jsonGiven && cw.n == 0 && (len(inv.fields) > 0 || cmd.danger != nil) {
		emitErrIfJSON(inv, stdout, cmd)
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
func emitErrIfJSON(inv *invocation, stdout io.Writer, cmd *command) {
	if !inv.jsonGiven {
		return
	}
	if len(inv.fields) == 0 && (cmd == nil || cmd.danger == nil) {
		return
	}
	e := inv.err
	if e == nil {
		e = &cliError{Kind: kindForExitCode(exitFail), Message: "（命令未报出 kind，按退码兜底）"}
	}
	emitErrEnvelope(stdout, cmd, e)
}

// ---- 命令树（真源：帮助文本、markdown 导出、别名解析都从这里出，不许旁写一份）----

type command struct {
	path     []string
	summary  string
	usage    string
	fields   []string // --json 可取的全部字段（不给字段时 stderr 列的就是它）
	args     []string // 位置参数的说明（帮助里逐条列出）
	endpoint string   // 它投影的远端端点（本机命令为空）；T-08 的三面同源对账用
	kind     string   // 包封里的 kind（§九 M6 I2：与命令一一对应 · 单数 CamelCase）
	danger   *dangerSpec
	idem     *idemSpec // M4 幂等四字段（没显式写的在 seedIdem 里按规则派生）
	// 群级只读（§十二 P-066）：没有目标时「读全群」是允许的；**其余命令无目标 ⇒ exit 2**。
	groupReadOnly bool
	// 茧壁层级标记（§九 M10 `X1`：闭集 host/node/space；没显式写的按族派生）
	layer       string
	opened      bool // 本版是否可执行（危险动作在批 A 一律未开放 · §6.2 零写操作）
	passthrough bool // 原样透传型（gate 族）：旗标与位置参数逐字交给被包的脚本
	run         func(*invocation, io.Writer, io.Writer) int
}

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
			path:     []string{"context", "ls"},
			kind:     "Context",
			summary:  "档位名册（离线也出表）",
			usage:    "zerg context ls [--json <字段>]",
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
			summary:  "把命令树导出 markdown 进 Zerg-内部文档（产物 · 勿手改）",
			usage:    "zerg help export [--out <目录>] [--json <字段>]",
			fields:   []string{"path", "commands", "dangerous", "schema"},
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
			summary:     "跑门禁（旗标逐字透传；退码原样转出，不翻译）",
			usage:       "zerg gate run [--scope <s> | --fast] [--outdir <目录>] …",
			args:        []string{"脚本旗标（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:        []string{"gate", "show"},
			summary:     "看某一步要跑的命令串（脚本 --emit-cmd）",
			usage:       "zerg gate show <步名>",
			args:        []string{"步名（与 --list 里逐字相同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
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
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via", "direct_gate"},
			endpoint: "GET /api/fleet/status（默认档）",
			run:      cmdAgentProbe,
		},
		{
			path:    []string{"agent", "bootstrap"},
			summary: "子端引导（危险 D3 · 本版未开放）",
			usage:   "zerg agent bootstrap <机器名> --confirm=<机器名> --yes [--dry-run]",
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
			args:     []string{"（旗标：--desc/--model/--priority/--slice-id/--depends-on/--acceptance）"},
			endpoint: "POST /api/tasks",
			run:      cmdTaskSubmit,
		},
		{
			path:     []string{"task", "diff"},
			kind:     "TaskDiff",
			summary:  "该任务工作树的**只读** `git diff --stat`",
			usage:    "zerg task diff <任务 id> [--json <字段>]",
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
			args:     []string{"任务 id"},
			fields:   []string{"id", "available", "detail"},
			endpoint: "GET /api/logs/task/{id}（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404）",
			run:      cmdTaskLogs,
		},
		{
			path:    []string{"task", "move"},
			summary: "挪动任务在队列里的位置（危险 D2 · 本版未开放）",
			usage:   "zerg task move <任务 id> --to <位置> --yes [--dry-run]",
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
			summary:  "`daemon ls`：本机服务脚本逐件可查（scripts/svc/ 5 件）",
			usage:    "zerg core daemon ls [--json <字段>]",
			args:     []string{"动作（ls）"},
			fields:   []string{"name", "script", "declared", "note"},
			endpoint: "",
			run:      cmdCoreDaemonLs,
		},
		{
			path:    []string{"core", "start"},
			summary: "起主控（危险 D3 · 本版未开放）",
			usage:   "zerg core start --confirm=<主机名> --yes [--dry-run]",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "起主控进程（会绑端口 8580；已在跑时是**换件**前置）", "§三 C 族 · §7.1 P11 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "restart"},
			summary: "重启主控（危险 D3 · 本版未开放）",
			usage:   "zerg core restart --confirm=<主机名> --yes [--dry-run]",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停 + 起主控（**整个虫群的控制面会断一会儿**）", "§三 C 族 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		// ---- 批 D · T-46 H 族四条（§三 H 族 · §7.1 `P10`/`P7`）----
		{
			path:     []string{"resource", "ls"},
			kind:     "Resource",
			summary:  "资源面（投影 /api/resources/ledger 或 /api/resources/{类型}）",
			usage:    "zerg resource ls [<类型>] [--json <字段>]",
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
			fields:   []string{"path", "public"},
			endpoint: "",
			run:      cmdScriptLs,
		},
		// 写面两枚：**同一个执行门**（三态：--dry-run 计划件 / 缺 --yes ⇒ 2 / 齐了才发）
		{
			path:    []string{"resource", "pin"},
			kind:    "ResourcePin",
			summary: "钉住资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource pin <资源 id> [--dry-run | --yes]",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"resource", "unpin"},
			kind:    "ResourceUnpin",
			summary: "解钉资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource unpin <资源 id> [--dry-run | --yes]",
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
			summary:  "卵 × 设备矩阵（主控面无投影端点 ⇒ 现跑 404 ⇒ 不给结论）",
			usage:    "zerg egg ls [--json <字段>]",
			fields:   []string{"egg_id", "machine", "state", "engine_impl", "schema_version"},
			endpoint: "GET /api/eggs（主控面没有 ⇒ 现跑 404；子端才有 /eggs）",
			run:      cmdEggLs,
		},
		{
			path:     []string{"egg", "show"},
			kind:     "Egg",
			summary:  "单枚卵的现状（同上：主控面无投影 ⇒ 不给结论）",
			usage:    "zerg egg show <卵 id> [--json <字段>]",
			args:     []string{"卵 id"},
			fields:   []string{"egg_id", "machine", "state", "engine_impl", "schema_version"},
			endpoint: "GET /api/eggs/{id}（主控面没有 ⇒ 现跑 404）",
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
			summary: "起虫茧的文档服务（8610 · D3 起服务档 · 本版未开放）",
			usage:   "zerg cocoon open <茧名> [--confirm=<茧名> --yes | --dry-run]",
			args:    []string{"茧名"},
			run:     cmdCocoonOpen,
		},
		{
			path:    []string{"egg", "run"},
			summary: "把卵跑起来（写面 · 本版未开放）",
			usage:   "zerg egg run <卵 id> [--yes | --dry-run]",
			args:    []string{"卵 id"},
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
			path:    []string{"build", "all"},
			summary: "重编全部制品（换件档 · 本版未开放；计划件见 --dry-run）",
			usage:   "zerg build all [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		{
			path:    []string{"build", "release"},
			summary: "打包发布件（换件档 · 本版未开放；计划件见 --dry-run）",
			usage:   "zerg build release [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		// ---- 危险动作：**只登记形状，不开放执行**（§6.2 批 1 零写操作）----
		// 每条都过 cmdGuarded：`--dry-run` 出计划件（退码 0）；真跑一律拒执（退码 2 = 不给结论）。
		{
			path:    []string{"task", "terminate"},
			summary: "终止任务（危险 D3 · 本版未开放）",
			usage:   "zerg task terminate <任务 id> --confirm=<任务 id> --yes [--expect=<旧值>] [--dry-run]",
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
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把排队中的任务置为暂停态（可 resume 回来）", "§三 D 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "resume"},
			summary: "继续任务（危险 D2 · 本版未开放）",
			usage:   "zerg task resume <任务 id> --yes [--dry-run]",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把暂停的任务放回排队（可能立刻占机器）", "§三 D 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"task", "retry"},
			summary: "重跑任务（危险 D2 · 本版未开放）",
			usage:   "zerg task retry <任务 id> --yes [--dry-run]",
			args:    []string{"任务 id"},
			danger:  &dangerSpec{dangerD2, "任务 id", "把 failed 任务置回 queued（会再占一次机器与模型槽）", "§三 D 族 · §6.3 S5"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"agent", "unload"},
			summary: "卸载子端上的模型（危险 D3 · 本版未开放）",
			usage:   "zerg agent unload <机器名> <模型> --confirm=<机器名> --yes [--dry-run]",
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
			args:    []string{"机器名", "模型"},
			danger:  &dangerSpec{dangerD2, "机器名", "把模型装进该子端（占内存/显存槽 · 单槽机是串行的）", "§三 B 族 · §九 M5 · 开工单 T-43"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"agent", "reap"},
			summary: "回收残留（危险 D3 · 本版未开放）",
			usage:   "zerg agent reap --confirm=<机器名> --yes [--dry-run]",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"按「声明树 ↔ 现值树」差集回收闲置资源（**默认干跑**；入库件永不进候选 = 红线）",
				"§九 M9 · §十五.2 三档回收权 · 开工单 T-54"},
			run: cmdGuarded,
		},
		{
			path:    []string{"model", "stop"},
			summary: "停模型（危险 D3 · 本版未开放）",
			usage:   "zerg model stop <模型 id> --confirm=<模型 id> --yes [--dry-run]",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD3, "模型 id", "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500", "§三 E 族 · §九 M4 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"model", "start"},
			summary: "起模型（危险 D2 · 本版未开放）",
			usage:   "zerg model start <模型 id> --yes [--dry-run]",
			args:    []string{"模型 id"},
			danger:  &dangerSpec{dangerD2, "模型 id", "把模型装载起来（占槽位 · 单槽机要排队）", "§三 E 族 · §九 M5 · 开工单 T-45"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "stop"},
			summary: "停主控（危险 D3 · 本版未开放）",
			usage:   "zerg core stop --confirm=<主机名> --yes [--dry-run]",
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
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执）", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "pin"},
			summary: "钉住一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg pin <卵 id> --yes [--dry-run]",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的）", "§3.4 I 族 · 开工单 T-47"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "unpin"},
			summary: "解钉一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg unpin <卵 id> --yes [--dry-run]",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "取消在孵标记（原本占有单槽的卵会被换下）", "§3.4 I 族 · 开工单 T-47"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "retire"},
			summary: "退役一枚卵（危险 D3 · 本版未开放）",
			usage:   "zerg egg retire <卵 id> --confirm=<卵 id> --yes [--dry-run]",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD3, "卵 id", "把该卵从名册与盘上退掉（**不可逆**；档 ③ 件永不自动）", "§十五.2 档③ · 开工单 T-54"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "release"},
			summary: "发布候选件（危险 D3 · 本版未开放）",
			usage:   "zerg dev release --candidate <候选 id> --confirm=<候选 id> --yes [--dry-run]",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把候选件推上生产面（**只能由人拍板开**；AI 不许自升）", "§17.4 · §九 M18 C4 · 开工单 T-58"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "rollback"},
			summary: "回滚（危险 D3 · 本版未开放）",
			usage:   "zerg dev rollback [--to <目标>] --confirm=<候选 id> --yes [--dry-run]",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把生产面退回某个已知状态（回滚件到期前**永不自动**）", "§17.2 ⑦ · §十五.2 档③ · 开工单 T-58"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"update"},
			summary: "源码式自更新（危险 D3 · 本版未开放）",
			usage:   "zerg update --confirm=<主机名> --yes [--dry-run]",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单）", "§7.1 P12 · §九 M20 F-3 · 开工单 T-52"},
			run:     cmdGuarded,
		},
		{
			path:     []string{"dev", "proposal"},
			kind:     "Proposal",
			summary:  "提案件通道：只产可审查物（new|list|show）· 目标必须回指既有编号",
			usage:    "zerg dev proposal new --title <题> --target <待办编号> --goal <目标> --evidence <出处> --rollback <退点> [--criterion <判据>] [--by <提出者>]",
			args:     []string{"动作：new | list | show", "提案 id（只 show 要）"},
			fields:   proposalFields,
			endpoint: "",
			run:      cmdDevProposal,
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
			args:    []string{"脚本"},
			danger: &dangerSpec{dangerD3, "脚本",
				"按调用者权限执行脚本（**退码原样透传**；核心名硬占位）",
				"§6.3 S6 · §九 M18 C5 · 开工单 T-50"},
			run: cmdGuarded,
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

// resolve 按「最多 2 段」贪心匹配：先试 [p0,p1]，再试 [p0]；剩下的是位置参数。
func resolve(pos []string) (*command, []string) {
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
	dryRun       bool
	confirm      string
	confirmGiven bool
	yes          bool

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
			// 给了旗标但没给字段：留给命令去判（契约 §4.1 K2：exit 1 + stderr 列字段）。
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
		case a == "--dry-run":
			inv.dryRun = true
		case a == "--yes":
			inv.yes = true
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
	if inv.wantHelp && len(pos) > 0 {
		pos = nil // `zerg task ls --help`：先只回帮助，不起命令
	}
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

// requireFields 统一处理「--json 不给字段」（契约 §4.1 K2）：stdout 0 字节、字段清单走 stderr、退码 1。
func requireFields(inv *invocation, stderr io.Writer) bool {
	if len(inv.fields) > 0 {
		return true
	}
	fmt.Fprintf(stderr, "%s: --json 需要逗号分隔的字段列表\n", progName)
	fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(fieldListOf(inv.path), ","))
	return false
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
	extra := ""
	if inv != nil && inv.err != nil {
		extra = ",\"error\":" + inv.err.errJSON()
	}
	fmt.Fprintf(stdout, "{\"schema\":%s,\"kind\":%s,\"items\":%s,\"meta\":{%s},\"warnings\":[],\"truncated\":false%s}\n",
		jstr(contractSchema), jstr(cmd.kind), itemsJSON, meta, extra)
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
		if !requireFields(inv, stderr) {
			return exitFail // K2：给了 --json 但不给字段 ⇒ 1（不是用法错）
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

func cmdHelp(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		// 主题表在 `topics.go`（**唯一真源**：分派与「可用主题」列清单同一处）。
		return renderHelpTopic(inv.args[0], stdout, stderr)
	}
	fmt.Fprint(stdout, helpText())
	return exitOK
}
