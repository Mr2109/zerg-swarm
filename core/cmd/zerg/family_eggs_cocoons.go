// family_eggs_cocoons.go —— 虫卵 / 虫茧两族（I / J 族 · §3.4 · §7.1 `P6` · §3.3 `N4` 硬占位表）。
//
// 口径（照定稿 + 本机现跑，**不编**）：
//
//	① 卵与茧的**真面在子端**：子端 11 条端点里有 `/eggs` · `/pin` · `/unpin` · `/services`（§三 I/J 族）。
//	   而**主控面今天没有**卵/茧的投影端点 —— 现跑 `/api/eggs` 与 `/api/fleet/eggs` 都是 **404**
//	   （只有 `/api/archive` 200）。⇒ 本族走 T-17 `watch` 同一档：**声明面 + `kind=blocked` + 退码 8**，
//	   **不**偷偷直连子端（§九 M20 `O2`）。
//	② `cocoon open <名>` 会**起一个服务**（8610）⇒ 那是「起进程」档，属不可逆动作的邻居：
//	   本版**只登记形状、不执行**，真开要 Mr2109 拍（起服务 = 动生产）。
//	③ 族名进 `N4` 硬占位表：`N4` 在**定稿里**（`设计-命令面与契约-v0.7.md` §3.3）——
//	   定稿是冻结件，本件**不改它一个字节**；"把这两族的族名写进 `N4`" 登记为**待拍**
//	   （下一版定稿时一次改），本件只把命令面这一侧落齐。
package main

import (
	"fmt"
	"io"
	"strings"
)

// eggsProjectionAbsent —— 主控面没有卵/茧投影面时的那一份判词（一处文本，六条命令共用）。
//
// 为什么把它抽出来：`egg ls`/`egg show`/`cocoon ls`/`cocoon build` 四条**同因同果**，
// 各写一遍判词就会漂（一处改了、三处没改）—— 它们是同一个缺口的四个视角。
func eggsProjectionAbsent(inv *invocation, what string, stderr io.Writer) int {
	inv.setErr("blocked", "eggs_projection_absent", "主控面没有卵/茧投影端点（子端才有 /eggs）")
	fmt.Fprintf(stderr, "%s: 主控面**没有**卵/茧的投影端点（%s ⇒ 现跑 404）\n", progName, what)
	fmt.Fprintf(stderr, "  子端 11 条端点里确有 `/eggs` · `/pin` · `/unpin` · `/services`（§三 I/J 族）\n")
	fmt.Fprintf(stderr, "⇒ %s **拿不到**，不给结论（退码 8 · kind=blocked · retryable=true）\n", what)
	fmt.Fprintf(stderr, "不做的：不偷偷直连子端、不拿本机文件顶替（§九 M20 O2「不偷偷」四条）\n")
	fmt.Fprintf(stderr, "要开这一面 = 主控加投影端点（源码 + 重启）⇒ 待 Mr2109 拍\n")
	return exitBlocked
}

// cmdEggLs —— `zerg egg ls`：卵 × 设备矩阵（**主控面没有投影端点 ⇒ 不给结论**）。
func cmdEggLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	err := c.getJSON("/api/eggs", &resp)
	if err == nil {
		rows, display := flattenPayload(resp)
		return listCmd(inv, stdout, stderr, display, rows)
	}
	return eggsProjectionAbsent(inv, "卵的清单（卵 × 设备矩阵）", stderr)
}

// cmdEggShow —— `zerg egg show <卵 id>`：单枚卵的现状。
func cmdEggShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `egg show` 要一个卵 id（`zerg egg ls` 先看清单）\n", progName)
		inv.setErr("usage", "missing_target", "缺卵 id")
		return exitUsage
	}
	c := newClient()
	var resp jsonObj
	if err := c.getJSON("/api/eggs/"+inv.args[0], &resp); err == nil {
		row := map[string]string{}
		keys := []string{}
		for k, v := range resp {
			keys = append(keys, k)
			row[k] = cell(v)
		}
		if len(keys) == 0 {
			return eggsProjectionAbsent(inv, "这枚卵的现状", stderr)
		}
		return listCmd(inv, stdout, stderr, keys, []map[string]string{row})
	}
	return eggsProjectionAbsent(inv, "这枚卵的现状", stderr)
}

// cmdEggRun —— `zerg egg run <卵 id>`：把卵跑起来（D2 写面 · **本版未开放**：它会在真机上起算）。
func cmdEggRun(inv *invocation, stdout, stderr io.Writer) int {
	return eggsProjectionAbsent(inv, "把一枚卵跑起来（写面未开）", stderr)
}

// cmdCocoonLs —— `zerg cocoon ls`：虫茧清单。
func cmdCocoonLs(inv *invocation, stdout, stderr io.Writer) int {
	c := newClient()
	var resp jsonObj
	if err := c.getJSON("/api/cocoons", &resp); err == nil {
		rows, display := flattenPayload(resp)
		return listCmd(inv, stdout, stderr, display, rows)
	}
	return eggsProjectionAbsent(inv, "虫茧清单", stderr)
}

// cmdCocoonOpen —— `zerg cocoon open <名>`：起 8610 服务 ⇒ **只登记形状，不执行**。
//
// 为什么不做成"直接跑一下"：起服务 = 起进程 = **动生产**（不可逆动作的邻居）。
// 判据（开工单 T-47 ③）要求的是"它能起 8610 服务"这件事**有落点**，不是"本版替你起一次"。
func cmdCocoonOpen(inv *invocation, stdout, stderr io.Writer) int {
	name := "(未给名)"
	if len(inv.args) > 0 {
		name = inv.args[0]
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s cocoon open\n", progName)
		fmt.Fprintf(stdout, "  危险档   : D3（起一个常驻服务 ⇒ 按不可逆动作办）\n")
		fmt.Fprintf(stdout, "  目标     : %s\n", name)
		fmt.Fprintf(stdout, "  它会动   : 起虫茧的文档服务（监听 **8610**）—— 这会占端口、进程常驻\n")
		fmt.Fprintf(stdout, "  执行要   : --confirm=<茧名> 与 --yes 同时到\n")
		fmt.Fprintf(stdout, "  本版状态 : **未开放** —— 起服务 = 动生产；要开请 Mr2109 拍（§6.3 S5）\n")
		fmt.Fprintf(stdout, "  来源     : §3.4 J 族 · §7.1 P6 · 开工单 T-47\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	inv.setErr("usage", "not_opened", "起服务属本版未开放档")
	fmt.Fprintf(stderr, "%s: `cocoon open` 要在目标机上**起一个常驻服务（8610）** ⇒ 本版**未开放**（不给结论 · 退码 2）\n", progName)
	fmt.Fprintf(stderr, "先看计划件：%s cocoon open %s --dry-run\n", progName, name)
	return exitUsage
}

// eggCocoonNames —— 两族的族名（`N4` 硬占位表的落点面）。
func eggCocoonNames() []string {
	names := []string{}
	for _, c := range catalog() {
		if len(c.path) == 2 && (c.path[0] == "egg" || c.path[0] == "cocoon") {
			names = append(names, strings.Join(c.path, " "))
		}
	}
	return names
}
