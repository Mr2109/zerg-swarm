// remote.go —— 远端语义（§九 M13 · 调研-M13 §五 条文 `Z1`–`Z7` · §十二 `P-066`–`P-071`）。
//
// 两轴分开（M13 的硬规矩）：
//
//	**连接级作用域**（`--context <档>`）：换的是「打哪台主控 + 用哪份凭据」，一次一条链。
//	**对象级作用域**（`--node <机>`）：换的是「这次动的对象在哪台机上」，可与连接级不同。
//	`Z1` 硬规矩 **L1 ≠ L3**：档里存的子端地址是**连接级**的东西，**不许**把它当成「本次目标」。
//
// 其余六条（本文件是它们的执行点）：
//
//	`Z2` **名册真源唯一** = `~/.zerg/contexts/<档>.yaml` 的 `nodes:`（`gateway/fleet.yaml`
//	     的 `fleet:` / `updates.nodes` 是**投影源**，不是真源 —— 门④ 已在核它）。
//	`Z3` 未知名 ⇒ `exit 2` + stderr 列名册 + 给最相似名。
//	`Z4` 子端**自报**名只作核对，**不自动加入**名册。
//	`Z5` 机器名不许改（不存在「别名表」）。
//	`Z6` `context ls` 离线也出表（已落地 —— T-03）。
//	`Z7` 该机不支持 ⇒ `kind=unsupported_on_node`。
//
// 默认作用域（§十二 `P-066`）：**群级只读 = 名册全集；其余 = `exit 2`**。
// 撞车规则（`P-067`）：位置参数机器名与 `--node` **同机 ⇒ 去重放行；异机 ⇒ `exit 2`**。
// 执行面（`P-068`）：**HTTP 经主控为唯一对外路径**（ssh 只留三条显式命令，见 §十五.4 `F-1`–`F-3`）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 默认档名（`~/.zerg/contexts/<档>.yaml`）；`--context` 可覆盖。
const defaultContextName = "default"

// machineTargets —— `dangerSpec.Target` 里表示「对象是某台机」的那几种写法（X 轴口径 §7.1）。
var machineTargets = map[string]bool{"机器名": true, "主机名": true, "网关主机名": true}

// roster —— 名册（`Z2` 的真源读法：只读 `nodes:` 的键 + `default_node`）。
type roster struct {
	Profile     string
	Path        string
	DefaultNode string
	Nodes       []string
}

// loadRoster 读一份档（读不到 ⇒ ok=false；**不**提示、不建文件）。
func loadRoster(profile string) (roster, bool) {
	r := roster{Profile: profile}
	home, err := os.UserHomeDir()
	if err != nil {
		return r, false
	}
	r.Path = filepath.Join(home, ".zerg", "contexts", profile+".yaml")
	b, err := os.ReadFile(r.Path)
	if err != nil {
		return r, false
	}
	inNodes := false
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inNodes = trimmed == "nodes:"
			if k, v, ok := splitScalar(trimmed); ok && k == "default_node" {
				r.DefaultNode = v
			}
			continue
		}
		if !inNodes {
			continue
		}
		// `nodes:` 块里的条目：`  x3:` 或 `  x3:` 后跟缩进字段；只取**键**，值一律不看。
		key := strings.TrimSuffix(trimmed, ":")
		if key == trimmed {
			continue
		}
		if i := strings.IndexByte(key, ':'); i >= 0 {
			key = strings.TrimSpace(key[:i])
		}
		if key != "" && !seen[key] {
			seen[key] = true
			r.Nodes = append(r.Nodes, key)
		}
	}
	sort.Strings(r.Nodes)
	return r, true
}

// nearestName 给「最像的合法名」（K14 第三件 · `Z3`）。
func nearestName(want string, cands []string) string {
	best, bestScore := "", -1
	for _, c := range cands {
		score := 0
		if strings.HasPrefix(c, want) || strings.HasPrefix(want, c) {
			score = 100
		} else {
			// 公共前缀长度当粗略相似度（不引依赖）
			n := 0
			for n < len(c) && n < len(want) && c[n] == want[n] {
				n++
			}
			score = n
		}
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// enforceNodeRules —— 对象级作用域的校验（`Z1`/`Z3`/`P-066`/`P-067`）。
//
// 返回 (rc, done)：done=true ⇒ 调用方直接返回 rc（已经报过话了）。
func enforceNodeRules(inv *invocation, cmd *command, stderr io.Writer) (int, bool) {
	profile := defaultContextName
	if inv.contextWant != "" {
		profile = inv.contextWant
	}
	r, ok := loadRoster(profile)
	if !ok {
		if inv.contextWant != "" {
			// `--context <不存在的档>` = 目标错 ⇒ 2（不许静默回落默认档）
			names := contextRows()
			list := make([]string, 0, len(names))
			for _, row := range names {
				list = append(list, row["name"])
			}
			fmt.Fprintf(stderr, "%s: 没有这个档 %q（读不到 %s）\n", progName, profile, filepath.Join("~", ".zerg", "contexts", profile+".yaml"))
			fmt.Fprintf(stderr, "现有档: %s\n", strings.Join(list, "、"))
			return exitUsage, true
		}
		return 0, false // 没建名册 ⇒ 本机面命令照旧跑（`Z6`：离线也出表）
	}

	// `--node` 可重复：去重（同机写两遍不是错，异机同时给才算歧义 —— `P-067` 的同一条口径）。
	nodes := dedupe(inv.nodes)

	// `Z3` 未知名 ⇒ 2 + 列名册 + 最像的
	for _, n := range nodes {
		if !contains(r.Nodes, n) {
			fmt.Fprintf(stderr, "%s: 名册里没有这台机 %q（名册真源 = %s 的 nodes: · §九 M13 Z2/Z3）\n",
				progName, n, r.Path)
			fmt.Fprintf(stderr, "名册: %s\n", strings.Join(r.Nodes, "、"))
			if s := nearestName(n, r.Nodes); s != "" {
				fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
			}
			fmt.Fprintf(stderr, "机器名不许改、也不许自动加入（自报名只作核对 · Z4/Z5）\n")
			return exitUsage, true
		}
	}
	if len(nodes) > 1 && !cmd.groupReadOnly {
		fmt.Fprintf(stderr, "%s: `%s %s` 的对象是**一台**机，`--node` 给了 %d 台（%s）\n",
			progName, progName, strings.Join(cmd.path, " "), len(nodes), strings.Join(nodes, "、"))
		fmt.Fprintf(stderr, "多目标只有**群级只读**命令允许（§十二 P-066）；其余要一台一条命令\n")
		return exitUsage, true
	}

	// 位置参数机器名 × `--node` 的撞车（`P-067`）
	machineArg := ""
	if len(inv.args) > 0 {
		if contains(r.Nodes, inv.args[0]) || (cmd.danger != nil && machineTargets[cmd.danger.Target]) {
			machineArg = inv.args[0]
		}
	}
	if machineArg != "" {
		if len(nodes) == 0 {
			// 位置参数就是对象级作用域（`Z1`：它与档里的地址**不是**一回事）
			return 0, false
		}
		if nodes[0] == machineArg {
			fmt.Fprintf(stderr, "%s: 位置参数与 `--node` **同机**（%s）⇒ 去重放行（§十二 P-067）\n", progName, machineArg)
			return 0, false
		}
		fmt.Fprintf(stderr, "%s: 位置参数机器名 %q 与 `--node %s` **异机** ⇒ 拒绝（§十二 P-067）\n",
			progName, machineArg, nodes[0])
		fmt.Fprintf(stderr, "同一台机器不许用两种说法指向两处（歧义禁令）；要打另一台就改 `--node` 或改位置参数\n")
		return exitUsage, true
	}

	// 默认作用域（`P-066`）：群级只读 = 名册全集；**其余不许无目标**
	needsMachine := cmd.danger != nil && machineTargets[cmd.danger.Target]
	if needsMachine && len(nodes) == 0 && len(inv.args) == 0 && !inv.dryRun {
		fmt.Fprintf(stderr, "%s: `%s %s` 的对象是某台**机**，本轮**没有目标**（位置参数与 `--node` 都没给）\n",
			progName, progName, strings.Join(cmd.path, " "))
		fmt.Fprintf(stderr, "默认作用域（§十二 P-066）：**群级只读 = 名册全集；其余 = exit 2** ⇒ 不给结论\n")
		fmt.Fprintf(stderr, "名册: %s（默认机 default_node = %s）\n", strings.Join(r.Nodes, "、"), orDash(r.DefaultNode))
		return exitUsage, true
	}
	return 0, false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// helpRemote —— `zerg help remote`（§九 M13 的自描述面）。
func helpRemote() string {
	var b strings.Builder
	b.WriteString("远端语义（§九 M13 · 调研-M13 §五）\n\n")
	b.WriteString("两轴分开（不许混 · `Z1` 硬规矩 L1 ≠ L3）：\n")
	b.WriteString("  连接级 `--context <档>`  打哪台主控 + 用哪份凭据（一次一条链）\n")
	b.WriteString("  对象级 `--node <机>`     这次动的对象在哪台机上（可重复，仅群级只读命令）\n")
	b.WriteString("  ★ 档里存的子端地址是**连接级**的东西 —— **不许**把它当成「本次目标」。\n\n")
	b.WriteString("默认作用域（§十二 `P-066`）：**群级只读 = 名册全集；其余 = `exit 2`**（没有目标就不猜）。\n")
	b.WriteString("撞车规则（`P-067`）：位置参数机器名与 `--node` **同机 ⇒ 去重放行；异机 ⇒ `exit 2`**。\n\n")
	b.WriteString("名册（`Z2` 真源唯一）：`~/.zerg/contexts/<档>.yaml` 的 `nodes:`；\n")
	b.WriteString("  `gateway/fleet.yaml` 的 `fleet:` / `updates.nodes` 只是**投影源**（门④ 在核它们逐字一致）。\n")
	b.WriteString("  未知名 ⇒ `exit 2` + 列名册 + 最相似名（`Z3`）；子端**自报名只作核对**、不许自动入册（`Z4`）。\n")
	b.WriteString("  机器名**不许改**（`Z5`：没有别名表）；`context ls` 离线也出表（`Z6`）。\n\n")
	b.WriteString("执行面（§十二 `P-068`）：**HTTP 经主控是唯一对外路径**；`ssh` 只留三条显式命令\n")
	b.WriteString("（§十五.4 的例外清单 `F-1` 子端引导 · `F-2` 探活诊断 · `F-3` 主控自身换件/自更新）。\n")
	b.WriteString("  直连若开，一律显式 `--direct` 且输出回显 `via`（不许偷偷直连 —— §九 M20 `O2`）。\n")
	if r, ok := loadRoster(defaultContextName); ok {
		b.WriteString("\n本机现读名册（" + r.Path + "）：\n")
		fmt.Fprintf(&b, "  default_node = %s\n", orDash(r.DefaultNode))
		for _, n := range r.Nodes {
			b.WriteString("  · " + n + "\n")
		}
	} else {
		b.WriteString("\n本机现读名册：**没有**（`~/.zerg/contexts/` 下无档 ⇒ 只有内置默认档 · `Z6`）\n")
	}
	return b.String()
}
