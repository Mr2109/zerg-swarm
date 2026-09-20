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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// progName —— 报头 / 帮助 / 命令名三处共用的名字（§3.3 N1：定名 `zerg`）。
const progName = "zerg"

// 退出码（契约 §三 主表；完整表与占号纪律由 `zerg help exit-codes` 自描述）。
const (
	exitOK      = 0
	exitFail    = 1 // 一般失败
	exitUsage   = 2 // 用法错 / 不给结论
	exitAuth    = 4 // 未认证
	exitBlocked = 8 // 有 BLOCKED
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
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
	// 没有人面/机器面的机器面表 ⇒ 不认 --json（用法错 2，不是 1）：1 是「给了 --json 但没给字段」
	// 那一档的码（§4.1 K2），两者不许混。
	if inv.jsonGiven && len(cmd.fields) == 0 {
		fmt.Fprintf(stderr, "%s: `%s %s` 没有 `--json` 字段表（它是说明面）\n", progName, progName, strings.Join(cmd.path, " "))
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	return cmd.run(inv, stdout, stderr)
}

// ---- 命令树（真源：帮助文本、markdown 导出、别名解析都从这里出，不许旁写一份）----

type command struct {
	path        []string
	summary     string
	usage       string
	fields      []string // --json 可取的全部字段（不给字段时 stderr 列的就是它）
	args        []string // 位置参数的说明（帮助里逐条列出）
	endpoint    string   // 它投影的远端端点（本机命令为空）；T-08 的三面同源对账用
	kind        string   // 包封里的 kind（§九 M6 I2：与命令一一对应 · 单数 CamelCase）
	danger      *dangerSpec
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
			summary: "单行身份（组件 版本 代码 sha 构建时间）",
			usage:   "zerg version [--json <字段>]",
			fields:  []string{"name", "version", "commit", "build_time"},
			run:     cmdVersion,
		},
		{
			path:    []string{"help"},
			summary: "帮助（主题: exit-codes · config）",
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
			path:    []string{"resource", "pin"},
			summary: "钉住资源（危险 D2 · 本版未开放）",
			usage:   "zerg resource pin <资源> --yes [--dry-run]",
			args:    []string{"资源 id"},
			danger:  &dangerSpec{dangerD2, "资源 id", "把资源标成不可回收（回收候选里会被排除）", "§7.1 P10 · §6.3 S5 · 开工单 T-46"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"resource", "unpin"},
			summary: "解钉资源（危险 D2 · 本版未开放）",
			usage:   "zerg resource unpin <资源> --yes [--dry-run]",
			args:    []string{"资源 id"},
			danger:  &dangerSpec{dangerD2, "资源 id", "取消不可回收标记（它会重新进入回收候选）", "§7.1 P10 · §6.3 S5 · 开工单 T-46"},
			run:     cmdGuarded,
		},
		{
			path:    []string{"gateway", "breakers"},
			summary: "网关断路开关（危险 D2 · 本版未开放）",
			usage:   "zerg gateway breakers [--reset] --yes [--confirm=<网关主机名>] [--dry-run]",
			danger:  &dangerSpec{dangerD2, "网关主机名", "重置网关熔断器（会立刻重新放流量进去）", "§7.1 P10 · §6.3 S5 · 开工单 T-46"},
			run:     cmdGuarded,
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
	return inv, nil
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
	src := cmd.endpoint
	if src == "" {
		src = "local（本机）"
	}
	fmt.Fprintf(stdout, "{\"schema\":%s,\"kind\":%s,\"items\":%s,\"meta\":{\"count\":%d,\"source\":%s},\"warnings\":[],\"truncated\":false}\n",
		jstr(contractSchema), jstr(cmd.kind), itemsJSON, count, jstr(src))
}

// emitSelected 是**全部** `--json` 出口的唯一实现：先按用户点名的字段（序即用户给的序）拼对象，
// 再套包封。字段点错 ⇒ 退码 2 + 列全部合法字段（§九 M6 I5）。
func emitSelected(stdout, stderr io.Writer, path []string, fields []string, rows []map[string]string) int {
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
	emitEnvelope(stdout, cmd, "["+strings.Join(objs, ",")+"]", len(rows))
	return exitOK
}

// selectJSON 单件命令用（仍然出数组：`items` 里一条 —— I3 恒数组）。
func selectJSON(stdout, stderr io.Writer, path []string, fields []string, row map[string]string) int {
	return emitSelected(stdout, stderr, path, fields, []map[string]string{row})
}

// selectJSONList 清单命令用。
func selectJSONList(stdout, stderr io.Writer, path []string, fields []string, rows []map[string]string) int {
	return emitSelected(stdout, stderr, path, fields, rows)
}

// ---- version ----

func cmdVersion(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail // K2：给了 --json 但不给字段 ⇒ 1（不是用法错）
		}
		return selectJSON(stdout, stderr, inv.path, inv.fields, map[string]string{
			"name":       progName,
			"version":    version.Version,
			"commit":     version.Commit,
			"build_time": version.BuildTime,
		})
	}
	fmt.Fprintln(stdout, version.Line(progName))
	return exitOK
}

// ---- help ----

func cmdHelp(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		switch inv.args[0] {
		case "exit-codes":
			fmt.Fprint(stdout, helpExitCodes())
			return exitOK
		case "config":
			fmt.Fprint(stdout, helpConfig())
			return exitOK
		case "dangerous":
			fmt.Fprint(stdout, helpDangerous())
			return exitOK
		default:
			fmt.Fprintf(stderr, "%s: 未知帮助主题 %q\n", progName, inv.args[0])
			fmt.Fprintf(stderr, "可用主题: exit-codes · config · dangerous\n")
			fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
			return exitUsage
		}
	}
	fmt.Fprint(stdout, helpText())
	return exitOK
}
