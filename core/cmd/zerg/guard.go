// guard.go —— 不可逆动作的**防呆形状**（三态：`--dry-run` / `--confirm=<目标>` / `--yes`）。
//
// 出处（不新造语义）：§4.1 `K7`（`--dry-run` + `--confirm=<id/名>`；**值必须匹配目标**）·
// §九 M3 `C1`（危险档 → 确认档**一一映射**，不许自创第三档）· `C2`（非交互 **fail-closed**，
// 无 TTY **不提示**）· `C3`（不可逆动作必须有**旧值匹配** `--expect=<旧 sha>`）·
// `C4`（`--dry-run` 三态 + **计划件**）· `C5`（留痕：一行一事件 · 追加只写 · 写失败即拒）·
// §十二 `P-014`（D2 用 `--yes`；**D3 必须 `--confirm` 与 `--yes` 同时到**）。
//
// **本批（S2 · 批 A）零写操作**（§6.2 逐字：「批 1 期间**不许有任何写操作命令**」）⇒ 这里登记的是
// **危险动作的形状与清单**，不是它们的实现：真跑一律**拒执**（退码 2 = 不给结论），只回**计划件**。
package main

import (
	"fmt"
	"io"
	"strings"
)

// 危险档两档（§九 M3 `C1`：危险档→确认档**一一映射**，不许自创第三档）。
const (
	dangerD2 = "D2" // 有影响但可逆：`--yes` 即可
	dangerD3 = "D3" // 不可逆：`--confirm=<目标>` 与 `--yes` **同时到**才执行
)

type dangerSpec struct {
	Level  string // D2 / D3
	Target string // `--confirm=<目标>` 里的「目标」是什么（任务 id / 机器名 / 主机名 / 候选 id）
	Effect string // 它会动什么（计划件逐条写出来）
	Source string // 定稿出处（可追溯到 §节）
}

// cmdGuarded —— 所有登记为「危险」的动作的唯一执行门（本批只到「拒执 + 计划件」）。
func cmdGuarded(inv *invocation, stdout, stderr io.Writer) int {
	cmd := find(inv.path)
	spec := cmd.danger
	path := strings.Join(cmd.path, " ")
	target := ""
	if len(target) == 0 && len(inv.args) > 0 {
		target = inv.args[0]
	}

	// ① 状态面先判（§九 M5 `C4` 判据顺序「先锁 → 再判闸门 → 再收卵」· §十二 `P-031` 幂等优先）——
	//    只在**能只读判出来**的那几条动作上做（本版：`agent load` 的「已装载」）。
	if rc, done := probeStateConflict(inv, cmd, stdout, stderr); done {
		return rc
	}

	// ② `--dry-run`：只出**计划件**，零副作用（§九 M3 C4）—— 这是本批**唯一**会返回 0 的那一态。
	if inv.dryRun {
		fmt.Fprint(stdout, planFor(cmd, spec, target))
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}

	// ③ 确认档（fail-closed：缺就拒，**从不提问** —— 无 TTY 也一样）
	//    D3：`--confirm` **必须给**（§十二 P-014「D3 必须两者都到」）；
	//    D2：`--confirm` 给不给都行，**给了就校验**（值必须与目标逐字相同 —— §4.1 K7 一个口径）。
	if spec.Level == dangerD3 && !inv.confirmGiven {
		msg := fmt.Sprintf("`%s` 是 %s 档（不可逆）——**缺确认 ⇒ 不执行**", path, spec.Level)
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=<%s> --yes\n", spec.Target)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, path)
		fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.confirmGiven && (target == "" || inv.confirm != target) {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，目标是 %q）⇒ 不执行", inv.confirm, target)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := fmt.Sprintf("`%s` 是 %s 档 —— **缺 --yes ⇒ 不执行**", path, spec.Level)
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, path)
		return exitUsage
	}

	// ④ 确认档齐了 —— 但**本批写面未开放**（§6.2）：不给结论，退码 2。
	msg := fmt.Sprintf("确认档已到 ✓（%s）；但 `%s` 在**本版未开放** —— 批 A 全程零写操作（§6.2），不给结论", spec.Level, path)
	inv.setErr("usage", "not_opened", msg)
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(stderr, "排期见 §6.3 S5 与它的承载任务（开工单 §3.1 的命令映射表）\n")
	return exitUsage
}

// probeStateConflict —— 「已在该状态」的只读探测（§九 M4 幂等语义 · §十二 `P-031` 幂等优先）。
//
// 为什么在这一层：M4 要的是**先算差再动手**；`agent load <机> <模型>` 的目标机上该模型已在
// 装载态时，动手是无意义的（同参必败）⇒ 按契约**明确报「已在此状态」**（`kind=conflict` ·
// 退码 `14` · `retryable=false`），不报 500、也不假装要去装第二遍。
//
// 只读保证：只 GET `/api/fleet/status`，不写任何状态（§6.2）。
// 探测不到（主控不可达）⇒ **不吞**：返回 `false` 让主流程照旧走（fail-closed 由后面的确认档与
// 未开放档兜住），同时把 unreachable 记进 invocation。
func probeStateConflict(inv *invocation, cmd *command, stdout, stderr io.Writer) (int, bool) {
	if strings.Join(cmd.path, " ") != "agent load" || len(inv.args) < 2 {
		return 0, false
	}
	machine, model := inv.args[0], inv.args[1]
	c := newClient()
	var fleet jsonObj
	if err := c.getJSON("/api/fleet/status", &fleet); err != nil {
		inv.setErr("unreachable", "state_probe_failed", err.Error())
		return 0, false
	}
	o := asObj(asObj(fleet["machines"])[machine])
	if o == nil {
		msg := fmt.Sprintf("名册里没有这台机 %q（先 `zerg agent ls` 看名册 · §九 M13 Z3）", machine)
		inv.setErr("unsupported_on_node", "unknown_machine", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage, true
	}
	for _, m := range asList(o["models"]) {
		if cell(m) == model {
			msg := fmt.Sprintf("%q 上 %q **已在装载态**（幂等：按「已在此状态」报，不重装 · §九 M4 / §十二 P-031）",
				machine, model)
			inv.setErr("conflict", "already_loaded", msg)
			inv.err.Where = "node:" + machine
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "error.kind=conflict · retryable=false · remedy=wait_or_reload（要重装得显式 `--reload`，属批 D）\n")
			return exitConflict, true
		}
	}
	return 0, false
}

// planFor 生成计划件（零副作用）：要动什么 · 目标是谁 · 缺什么前置 · 怎么留痕 · 本版状态。
func planFor(cmd *command, spec *dangerSpec, target string) string {
	path := strings.Join(cmd.path, " ")
	need := "--yes"
	if spec.Level == dangerD3 {
		need = fmt.Sprintf("--confirm=<%s> 与 --yes 同时到", spec.Target)
	}
	var b strings.Builder
	b.WriteString("计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）\n")
	fmt.Fprintf(&b, "  动作     : %s %s\n", progName, path)
	fmt.Fprintf(&b, "  危险档   : %s（%s）\n", spec.Level, dangerLevelWord(spec.Level))
	fmt.Fprintf(&b, "  目标     : %s\n", orDash(target))
	fmt.Fprintf(&b, "  它会动   : %s\n", spec.Effect)
	fmt.Fprintf(&b, "  执行要   : %s\n", need)
	fmt.Fprintf(&b, "  旧值校验 : --expect=<旧值>（不可逆动作必须带，§九 M3 C3）\n")
	fmt.Fprintf(&b, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	fmt.Fprintf(&b, "  本版状态 : **未开放** —— 批 A 全程零写操作（§6.2）；排期见 §6.3 S5\n")
	fmt.Fprintf(&b, "  来源     : %s\n", spec.Source)
	return b.String()
}

func dangerLevelWord(l string) string {
	if l == dangerD3 {
		return "不可逆 · 必须确认到目标"
	}
	return "有影响但可逆"
}

func orDash(s string) string {
	if s == "" {
		return "（未给 —— 位置参数里要写明 id/名）"
	}
	return s
}

// helpDangerous 渲染「危险动作清单」：**§三 全族里标危险的动作逐条带 `--dry-run` 与 `--confirm`**。
func helpDangerous() string {
	var b strings.Builder
	b.WriteString("危险动作清单（§三 全族里标「危险」的动作 · 逐条给三态形状 · 契约 §七）\n\n")
	b.WriteString("三态（§4.1 K7 · §九 M3 C1/C2/C4 · §十二 P-014）：\n")
	b.WriteString("  --dry-run                 只出**计划件**，零副作用 —— 系统状态逐字不变\n")
	b.WriteString("  --confirm=<目标>          值必须与目标**逐字相同**；给错值 = 拒绝（不是「当没给」）· D3 **必须给**，D2 给了就校验\n")
	b.WriteString("  --yes                     D2 单独有效；**D3 必须 `--confirm` 与 `--yes` 同时到**（§十二 P-014 双档）\n")
	b.WriteString("  非交互：无 TTY **零提示词**、缺确认一律 **fail-closed**（不执行）\n\n")
	b.WriteString("清单（命令 · 档 · `--confirm` 的目标是什么 · 它会动什么 · 出处）：\n")
	w := 0
	for _, c := range catalog() {
		if c.danger == nil {
			continue
		}
		if n := displayWidth("zerg " + strings.Join(c.path, " ")); n > w {
			w = n
		}
	}
	for _, c := range catalog() {
		if c.danger == nil {
			continue
		}
		name := "zerg " + strings.Join(c.path, " ")
		line := "  " + pad(name, w) + "  " + c.danger.Level + "  --dry-run · --confirm=<" + c.danger.Target + "> · --yes"
		line += "  " + c.danger.Effect + "  [" + c.danger.Source + "]"
		if !c.opened {
			line += "  ← 本版未开放"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n本批（S2 · 批 A）**零写操作**（§6.2）：上表全部**拒执**（退码 2 = 不给结论），只有 `--dry-run` 出计划件（退码 0）。\n")
	return b.String()
}
