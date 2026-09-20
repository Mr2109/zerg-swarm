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
	return cmd.run(inv, stdout, stderr)
}

// ---- 命令树（真源：帮助文本、markdown 导出、别名解析都从这里出，不许旁写一份）----

type command struct {
	path    []string
	summary string
	usage   string
	fields  []string // --json 可取的全部字段（不给字段时 stderr 列的就是它）
	args    []string // 位置参数的说明（帮助里逐条列出）
	run     func(*invocation, io.Writer, io.Writer) int
}

// commands —— 批 A（S1 起）登记的只读面；后续各票在此续行。
// 用 init() 而不是包级字面量：命令树的 handler 又回头读 `commands`（帮助渲染），
// 写成包级字面量会被编译器判成初始化环。
var commands []*command

func init() {
	commands = []*command{
		{
			path:    []string{"version"},
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
}

func parseInvocation(args []string) (*invocation, error) {
	inv := &invocation{}
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
		case a == "--help" || a == "-h":
			inv.wantHelp = true
		case a == "--version":
			inv.wantVersion = true
		case strings.HasPrefix(a, "-") && a != "-":
			return nil, fmt.Errorf("未知旗标 %q", a)
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

// selectJSON 出用户点名的那几个字段（值一律当字符串，形状约定由各命令自己保证）。
// 点到不存在的字段 ⇒ 退码 2 + 列全部合法字段（§九 M6 I5）。已登记字段里**没有值**的，
// 用空串占位，由调用方决定是否 `null`（本版不出现 null）。
func selectJSON(stdout, stderr io.Writer, path []string, fields []string, row map[string]string) int {
	legal := fieldListOf(path)
	var b strings.Builder
	b.WriteString("{")
	for i, f := range fields {
		v, ok := row[f]
		if !ok {
			fmt.Fprintf(stderr, "%s: 未知字段 %q\n", progName, f)
			fmt.Fprintf(stderr, "合法字段: %s\n", strings.Join(legal, ","))
			fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
			return exitUsage
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
	b.WriteString("}\n")
	io.WriteString(stdout, b.String())
	return exitOK
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
		fmt.Fprintf(stderr, "%s: 未知帮助主题 %q\n", progName, inv.args[0])
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	fmt.Fprint(stdout, helpText())
	return exitOK
}
