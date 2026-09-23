// plugin.go —— 插件机制（`zerg-<名>` 约定 · §6.3 **S6** · §九 M18 `C5` · §7.1 **`P14`** · 开工单 T-50）。
//
// 三条照定稿（不新造机制）：
//
//	① **插件 = 一个可执行件 + 一个命名约定**：`zerg-<名>` ⇒ `zerg <名> …`。
//	   这是 git/kubectl 的同款约定（S6 行逐字「`zerg-<名>` 插件约定」），
//	   **零注册表**（S6 逐字「插件机制可整体移除」）—— 目录里有什么就是什么。
//	② **信任声明**（`P14`）：**以调用者权限运行 · 不验证不签名不背书**。
//	   插件不是「虫族的一部分」：命令面不替它背书、不做完整性校验、不沙箱它。
//	   这条声明三个地方都要有：`zerg help plugins` · `zerg plugin ls` · **每次调插件时的 stderr 回显**。
//	③ **影子告警**：插件名与**内置命令**同名 ⇒ 内置命令胜出、插件那份**永远调不到** ⇒
//	   `plugin ls` 必须点出来（存在一个永远不生效的文件 = 定时炸弹）。
//
// 本版边界（照实说）：默认插件目录只有**仓内**`scripts/plugins/`（另加 `ZERG_PLUGIN_PATH` 显式改写，
// 冒号分隔）—— **不默认扫 PATH**。理由：本机 `/usr/local/bin` 里装着 `zerg-agent` / `zerg-agentd`
// 这类**常驻件**，把它们当插件列出来是**错的**（它们不是命令面的插件）；要看 PATH 得显式给
// `ZERG_PLUGIN_PATH`。这条边界登记在《开工记录》T-50 节。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// pluginTrustNotice —— 信任声明的**唯一措辞**（`P14`；三处引用同一常量，不许各写一句）。
const pluginTrustNotice = "以调用者权限运行 · 不验证不签名不背书"

// pluginPrefix —— 命名约定（`zerg-<名>`）。搬家/改名 = 改约定 ⇒ 必须单独一次提交。
const pluginPrefix = "zerg-"

// pluginFields —— `zerg plugin ls --json` 的字段表（K1：机器面先定）。
var pluginFields = []string{"name", "command", "path", "shadowed", "shadow_of", "trust"}

// pluginEntry —— 一个被发现的插件。
type pluginEntry struct {
	Name     string // 去掉前缀的短名（`hello`）
	Path     string // 可执行件的绝对路径
	ShadowOf string // 命中的内置命令名（空 = 不是影子）
}

// pluginDirs —— 插件目录（`ZERG_PLUGIN_PATH` > 仓内 `scripts/plugins/`）。
//
// 为什么默认不扫 PATH：见文件头「本版边界」。
func pluginDirs() []string {
	out := []string{}
	if v := strings.TrimSpace(os.Getenv("ZERG_PLUGIN_PATH")); v != "" {
		for _, p := range filepath.SplitList(v) {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	if root := repoRoot(); root != "" {
		out = append(out, filepath.Join(root, "scripts", "plugins"))
	}
	return out
}

// builtinNames —— 内置命令的第一段名（影子判定的真源：**命令树自己**，不另抄一份清单）。
func builtinNames() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range catalog() {
		if len(c.path) == 0 {
			continue
		}
		if n := c.path[0]; !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// discoverPlugins 扫插件目录：`zerg-*` 且可执行 ⇒ 一件插件。目录不存在 ⇒ 空（不是错）。
func discoverPlugins() []pluginEntry {
	out := []pluginEntry{}
	builtins := map[string]bool{}
	for _, n := range builtinNames() {
		builtins[n] = true
	}
	seen := map[string]bool{}
	for _, dir := range pluginDirs() {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue // 目录不在 = 没有插件（「没有」与「读不到」在这一面上不区分：目录是**可选**的）
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasPrefix(e.Name(), pluginPrefix) {
				continue
			}
			name := strings.TrimPrefix(e.Name(), pluginPrefix)
			if name == "" {
				continue
			}
			abs := filepath.Join(dir, e.Name())
			st, err := os.Stat(abs)
			if err != nil || st.Mode()&0o111 == 0 {
				continue // 不可执行 ⇒ 不是一个能跑的插件（不列，免得给人「它在那儿能跑」的错觉）
			}
			key := dir + "/" + e.Name()
			if seen[key] {
				continue
			}
			seen[key] = true
			ent := pluginEntry{Name: name, Path: abs}
			if builtins[name] {
				ent.ShadowOf = name
			}
			out = append(out, ent)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// pluginRow —— 一件插件 → 机器面的一行。
func pluginRow(p pluginEntry) map[string]string {
	shadowed := "false"
	if p.ShadowOf != "" {
		shadowed = "true"
	}
	return map[string]string{
		"name":      p.Name,
		"command":   progName + " " + p.Name,
		"path":      p.Path,
		"shadowed":  shadowed,
		"shadow_of": p.ShadowOf,
		"trust":     pluginTrustNotice,
	}
}

// cmdPluginLs —— `zerg plugin ls`（S6 的插件自描述面：列出 + 影子告警 + 信任声明）。
func cmdPluginLs(inv *invocation, stdout, stderr io.Writer) int {
	plugins := discoverPlugins()
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		rows := []map[string]string{}
		for _, p := range plugins {
			rows = append(rows, pluginRow(p))
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	fmt.Fprintf(stdout, "插件（`%s<名>` 约定 · **零注册表**：目录里有什么就是什么）\n", pluginPrefix)
	fmt.Fprintf(stdout, "信任声明：插件%s（§6.3 S6 · §7.1 P14）\n", pluginTrustNotice)
	dirs := pluginDirs()
	if len(dirs) == 0 {
		fmt.Fprintf(stdout, "插件目录：**解析不到**（仓根找不到，且没给 ZERG_PLUGIN_PATH）\n")
	} else {
		fmt.Fprintf(stdout, "插件目录：%s\n", strings.Join(dirs, " · "))
	}
	if len(plugins) == 0 {
		fmt.Fprintf(stdout, "共 0 件 —— 目录里没有可执行的 `%s*` 件（这不是错：插件是**可选**的）\n", pluginPrefix)
		return exitOK
	}
	w := 0
	for _, p := range plugins {
		if len(p.Name) > w {
			w = len(p.Name)
		}
	}
	shadows := 0
	for _, p := range plugins {
		mark := ""
		if p.ShadowOf != "" {
			mark = "  ⚠ 影子"
			shadows++
		}
		fmt.Fprintf(stdout, "  %-*s  %s  命令名 %s %s%s\n", w+len(pluginPrefix), pluginPrefix+p.Name, p.Path, progName, p.Name, mark)
	}
	fmt.Fprintf(stdout, "共 %d 件", len(plugins))
	if shadows > 0 {
		fmt.Fprintf(stdout, " · **影子 %d 件**（与内置命令同名 ⇒ 永远调不到）", shadows)
	}
	fmt.Fprintf(stdout, "\n")
	if shadows > 0 {
		for _, p := range plugins {
			if p.ShadowOf != "" {
				fmt.Fprintf(stderr, "⚠ 影子告警：`%s%s` 与内置命令 `%s` 同名 —— 内置命令**胜出**，"+
					"这份插件**永远不会被调到**（要么改名，要么删；留着一个永不生效的文件就是定时炸弹）\n",
					pluginPrefix, p.Name, p.ShadowOf)
			}
		}
	}
	fmt.Fprintf(stderr, "信任声明：插件%s —— 命令面不替它背书（每次调插件也会再回显一次）\n", pluginTrustNotice)
	return exitOK
}

// tryPlugin —— `zerg <名> …` 落到插件上（git 同款）。第二个返回值 = 是否已处理。
//
// 纪律：**只转发、不翻译** —— 参数原样给插件、退码原样返回（与 `zerg gate` 同一条铁律）。
// 只在「命令树里没有这条命令」时才会走到这里（`cmd == nil`）。
func tryPlugin(inv *invocation, stdout, stderr io.Writer) (int, bool) {
	if len(inv.path) == 0 {
		return 0, false
	}
	name := inv.path[0]
	// 只认「简单名」：带路径分隔符 / 大小写混杂的输入不当插件名（免得 `zerg ../../x` 这种形状被解释）
	if name == "" || strings.ContainsAny(name, "/\\") || name != strings.ToLower(name) {
		return 0, false
	}
	var hit pluginEntry
	found := false
	for _, p := range discoverPlugins() {
		if p.Name == name {
			if p.ShadowOf != "" {
				// 影子不派发（内置命令胜出）—— 但**要说清**为什么不动
				fmt.Fprintf(stderr, "%s: 命令 %q 有内置实现；同名的插件 %s 被**跳过**（内置命令胜出）\n",
					progName, name, p.Path)
				return 0, false
			}
			hit, found = p, true
			break
		}
	}
	if !found {
		return 0, false
	}
	argv := []string{}
	if len(inv.orig) > 1 {
		argv = inv.orig[1:] // 去掉插件短名那一格，其余**原样**交给插件
	}
	fmt.Fprintf(stderr, "插件 %s：%s（信任声明：%s）\n", pluginPrefix+hit.Name, hit.Path, pluginTrustNotice)
	cmd := exec.Command(hit.Path, argv...)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), true // 退码原样转出（不翻译）
		}
		inv.setErr("failed", "plugin_run_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不了插件 %s：%v\n", progName, hit.Path, err)
		return exitFail, true
	}
	return exitOK, true
}

// helpPlugins —— `zerg help plugins`（S6 的自描述面 · `P14` 的信任声明措辞落在这一处）。
func helpPlugins() string {
	var b strings.Builder
	b.WriteString("插件机制（§6.3 S6 · §7.1 P14 · §九 M18 C5）\n\n")
	fmt.Fprintf(&b, "**信任声明（唯一措辞 · P14）**：插件%s。\n", pluginTrustNotice)
	b.WriteString("  含义逐条：① 命令面**不以自己的权限**替它做事 —— 它就是**以调用者权限**跑的；\n")
	b.WriteString("  ② 命令面**不验证**它（没有完整性校验、没有哈希清单）；③ **不签名**（没有签名面）；\n")
	b.WriteString("  ④ **不背书**（插件的行为不是虫族的行为，出问题不指向命令面）。\n\n")
	b.WriteString("约定（**零注册表** · S6 逐字「插件机制可整体移除」）：\n")
	fmt.Fprintf(&b, "  · 件名 `%s<名>` 且**可执行** ⇒ `zerg <名> …` 会调到它（git / kubectl 同款约定）。\n", pluginPrefix)
	b.WriteString("  · 参数**原样**交给插件；**退码原样**返回（只转发、不翻译 —— 同 `zerg gate` 那条铁律）。\n")
	b.WriteString("  · 内置命令**永远胜出**：同名插件是「影子」⇒ `zerg plugin ls` 会告警，且**永不派发**。\n")
	b.WriteString("  · 没有注册表、没有配置文件、没有清单文件 —— 目录里有什么就是什么。\n\n")
	b.WriteString("插件目录（本版边界，照实说）：\n")
	b.WriteString("  · 默认只有**仓内** `scripts/plugins/`；`ZERG_PLUGIN_PATH=<冒号分隔>` 可显式改写。\n")
	b.WriteString("  · **不默认扫 PATH**：本机 /usr/local/bin 里装着 zerg-agent / zerg-agentd 这类**常驻件**，\n")
	b.WriteString("    把它们当插件列出来是错的（它们不是命令面的插件）。要看 PATH 得显式给 ZERG_PLUGIN_PATH。\n\n")
	b.WriteString("自检（现跑可复跑）：\n")
	b.WriteString("  · `zerg plugin ls` —— 列件 + 影子告警 + 信任声明；\n")
	b.WriteString("  · `zerg plugin ls --json name,shadowed` —— 机器面（字段表见 `zerg help contract`）；\n")
	b.WriteString("  · `zerg hello` —— 仓内演示插件（`scripts/plugins/zerg-hello`）跑通即 S6 判据①。\n")
	return b.String()
}
