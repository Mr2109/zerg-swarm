// guard.go —— 不可逆动作的**防呆形状**（三态：`--dry-run` / `--confirm=<目标>` / `--yes`）。
//
// 出处（不新造语义）：§4.1 `K7`（`--dry-run` + `--confirm=<id/名>`；**值必须匹配目标**）·
// §九 M3 `C1`（危险档 → 确认档**一一映射**，不许自创第三档）· `C2`（非交互 **fail-closed**，
// 无 TTY **不提示**）· `C3`（不可逆动作必须有**旧值匹配** `--expect=<旧 sha>`）·
// `C4`（`--dry-run` 三态 + **计划件**）· `C5`（留痕：一行一事件 · 追加只写 · 写失败即拒）·
// §十二 `P-014`（D2 用 `--yes`；**D3 必须 `--confirm` 与 `--yes` 同时到**）。
//
// **本批（S2 · 批 A）零写操作**（§6.2 逐字：「批 1 期间**不许有任何写操作命令**」）⇒ 这里登记的是
// **危险动作的形状与清单**，不是它们的实现：**未开放的那批**真跑一律**拒执**（退码 2 = 不给结论），只回**计划件**；**已开放**的走自家 runner、确认档齐就真执行（逐条以 `zerg help dangerous` 现读为准）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
	// ChecksConfirm —— 本命令的**实现**真读 `--confirm` 吗（直读 `inv.confirm`/`confirmGiven`，
	// 或真跑那一态转交 `cmdGuarded`）。真读 ⇒ 清单行才印 `--confirm=<目标>`；不读 ⇒ 不印。
	// 为什么要有这一格：清单行原来**无条件机械拼** `--confirm=<目标>`，而「前言」又无条件承诺
	// 「D3 必须给，D2 给了就校验」—— 命令树里一批**已开放**的实现根本不读 `--confirm` ⇒
	// 表在替实现许一个空承诺（§九 M3 C1：危险档 ↔ 确认档一一映射）。逐条登记见 main.go 每一处
	// `danger: &dangerSpec{…, ChecksConfirm}`；此格是**位置参数**，漏登记编译不过。
	ChecksConfirm bool
}

// cmdGuarded —— **登记到本门名下**（`run: cmdGuarded`）那批危险动作的执行门（未开放的那批只到「拒执 + 计划件」；**已开放**的各走自家 runner、不以本门为门 —— 逐条以 `zerg help dangerous` 现读为准）。
func cmdGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
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
		fmt.Fprint(stdout, planFor(cmd, spec, target, inv))
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

// writeGuardDryRun —— **已开放**写面命令的 `--dry-run` 计划件（缺口 `GAP-20260927-140` ① ·
// Mr2109 2026-10-08 拍「默认档要 fail-closed」）。给**没有自己的 `--dry-run` 支**的写面命令用；
// 计划件与 `cmdGuarded` **同源**（`planFor`）—— 不许两处各拼一份。
//
// 返回 (rc, done)：done=true ⇒ 调用方直接 `return rc`（已出计划件 · 零副作用）。
func writeGuardDryRun(inv *invocation, stdout, stderr io.Writer) (int, bool) {
	if !inv.dryRun {
		return 0, false
	}
	// `--dry-run` 与 `--yes` 同给 ⇒ 两道确认档自相矛盾 ⇒ 先把共享判定跑了（与 `cmdGuarded` 同口径）。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc, true
	}
	cmd := find(inv.path)
	if cmd == nil || cmd.danger == nil {
		return 0, false
	}
	target := ""
	if len(inv.args) > 0 {
		target = inv.args[0]
	}
	fmt.Fprint(stdout, planFor(cmd, cmd.danger, target, inv))
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
	return exitOK, true
}

// writeGuardYes —— **已开放**写面命令的 fail-closed **确认档**（唯一一处 · 三态里真跑那一档）。
// 在命令自己的 `--dry-run` 支**之后**、**唯一写盘点之前**调用（与 `cmdGuarded` 的 ③ 段同口径）：
//
//	· `--dry-run`（计划件那一档）⇒ 放行（本函数只管真跑档 —— 干跑不是「要 --yes」）；
//	· 缺 `--yes` ⇒ 用法错 2（**零副作用 · 不给结论** · 从不提问 · 无 TTY 也一样）；
//	· `--yes` 齐 ⇒ 放行（调用方照旧写）。
//
// 返回 exitOK ⇒ 往下写；否则 `return rc`。`--dry-run` 与 `--yes` 同给 ⇒ 共享判定 `dryRunYesConflict` 退 2。
func writeGuardYes(inv *invocation, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun || inv.yes {
		return exitOK
	}
	path := strings.Join(inv.path, " ")
	lvl := dangerD2
	if cmd := find(inv.path); cmd != nil && cmd.danger != nil {
		lvl = cmd.danger.Level
	}
	msg := fmt.Sprintf("`%s` 是 %s 档 —— **缺 --yes ⇒ 不执行**", path, lvl)
	inv.setErr("usage", "yes_required", msg)
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, path)
	fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
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
		if cell(m) != model {
			continue
		}
		// 已经在装载态 —— 分两档（§九 M4 §5.2 的硬规矩 + §十二 `P-031` 幂等优先）：
		//   ① **幂等重跑**（没给任何「重建」旗标）：退 **0**、`changed=false`、明说「已在该状态」
		//      —— **不许**退 1/2（「已存在」不是错）。
		//   ② **真冲突**（给了 `--reload`/`--force`：声明与运行态要求重建，而在飞/占位挡着）：
		//      `kind=conflict` + 退码 `14`（**`P-013` ②** 定稿：冲突 = `14`，不是 `507`）。
		if !inv.reload && !inv.force {
			// 「已在该状态」是**成功码内的语义**（调研-M4 §5.2）⇒ 不挂 `error` 块、
			// 只把 `state_kind` 与 `changed:false` 写进结果面（`meta.changed` 是硬要求）。
			no := false
			inv.changed = &no
			msg := fmt.Sprintf("%q 上 %q **已在该状态**（幂等重跑：changed=false，不重装 · §九 M4）", machine, model)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "退码 0（幂等重跑**必须退 0**：已存在不是错 · 调研-M4 §5.2）· meta.changed=false · 幂等键 %s\n",
				idempotencyKey(inv))
			if inv.jsonGiven {
				items := "[{\"state\":\"already_loaded\",\"state_kind\":\"already_in_state\",\"changed\":\"false\",\"machine\":" +
					jstr(machine) + ",\"model\":" + jstr(model) + "}]"
				emitEnvelopeWith(stdout, cmd, items, 1, inv)
			}
			return exitOK, true
		}
		msg := fmt.Sprintf("%q 上 %q 已在装载态，而本轮要求按 %s 重建 —— 这是**真冲突**（换成 `wait_or_reload` 档处理 · §十二 P-031/P-013 ②）",
			machine, model, reloadWord(inv))
		inv.setErr("conflict", "rebuild_blocked", msg)
		inv.err.Where = "node:" + machine
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "error.kind=conflict · retryable=false · remedy=wait_or_reload · 退码 14（`P-013` ②：冲突/被占走 14）\n")
		return exitConflict, true
	}
	return 0, false
}

// reloadWord 给判词里回显「是哪一枚旗标要求重建」（S6：回显解析后的真实目标/档）。
func reloadWord(inv *invocation) string {
	if inv.force {
		return "--force（重建承载者：卸 + 装 / 新代次）"
	}
	return "--reload（重读声明）"
}

// planFor 生成计划件（零副作用）：要动什么 · 目标是谁 · 缺什么前置 · 怎么留痕 · 本版状态。
func planFor(cmd *command, spec *dangerSpec, target string, inv *invocation) string {
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
	// ---- 长任务形态（§九 M8）：默认档由「读的人是谁」决定 + 句柄 ----
	mode := resolveWaitMode(inv)
	ttyStar := ""
	if !inv.wait && !inv.noWait && !inv.follow {
		ttyStar = "  ← **默认档**（" + ttyNote(inv) + "）"
	}
	fmt.Fprintf(&b, "  默认档   : %s%s\n", waitModeWord(mode, ttyStar), "")
	for _, l := range handleLines(newHandle(inv)) {
		b.WriteString(l + "\n")
	}
	// ---- 并发与锁（§九 M5 C4/P-022）：slot 块带版本号 / epoch ----
	for _, l := range slotBlockLines(target) {
		b.WriteString(l + "\n")
	}
	b.WriteString("  " + lockLine() + "\n")
	return b.String()
}

// ttyNote 说明默认档是按哪一侧判出来的（`P-020`：TTY ⇒ wait · 非 TTY ⇒ no-wait）。
func ttyNote(inv *invocation) string {
	if inv.tty {
		return "stdout 是 TTY ⇒ 默认 --wait"
	}
	return "stdout 不是 TTY ⇒ 默认 --no-wait"
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

// helpDangerous 渲染「危险动作清单」：**§三 全族里标危险的动作逐条带 `--dry-run` 与 `--yes`**；
// `--confirm=<目标>` **只在 `dangerSpec.ChecksConfirm` 为真的那几条上印** —— 印了就是实现真会校验它。
func helpDangerous() string {
	var b strings.Builder
	b.WriteString("危险动作清单（§三 全族里标「危险」的动作 · 逐条给三态形状 · 契约 §七）\n\n")
	b.WriteString("三态（§4.1 K7 · §九 M3 C1/C2/C4 · §十二 P-014）：\n")
	b.WriteString("  --dry-run                 只出**计划件**，零副作用 —— 系统状态逐字不变\n")
	b.WriteString("  --confirm=<目标>          值必须与目标**逐字相同**；给错值 = 拒绝（不是「当没给」）· D3 **必须给**；**下表逐条标出哪几条真读它** —— 没标 `--confirm` 的那几条本版实现**不读**它（给了也等于没给），D2 本就不需要它\n")
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
		// 三态形状**按登记现算**（`dangerSpec.ChecksConfirm`）：只有真读 `--confirm` 的命令才印它；
		// 不读的不印 —— 表与实现同源，不许替实现许空承诺。
		flags := "  --dry-run · --yes"
		if c.danger.ChecksConfirm {
			flags = "  --dry-run · --confirm=<" + c.danger.Target + "> · --yes"
		}
		line := "  " + pad(name, w) + "  " + c.danger.Level + flags
		line += "  " + c.danger.Effect + "  [" + c.danger.Source + "]"
		if c.opened {
			line += "  ← **已开放**（确认档齐就真执行）"
		} else {
			line += "  ← 本版未开放"
		}
		b.WriteString(line + "\n")
	}
	// 收口那句**按命令树现算**（不再一律写「全部拒执」：有真实现的已开放 ⇒ 逐条标）。
	open, closed := 0, 0
	for _, c := range catalog() {
		if c.danger == nil {
			continue
		}
		if c.opened {
			open++
		} else {
			closed++
		}
	}
	fmt.Fprintf(&b, "\n上表 %d 条：**已开放 %d 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 %d 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。\n",
		open+closed, open, closed)
	return b.String()
}

// dryRunYesConflict —— 「`--dry-run`（只出计划件）与 `--yes`（真写）**同给**」的**唯一**共享判定
// （`D3b` 反向对齐 · 2026-09-27）：两道确认档自相矛盾 ⇒ 用法错 2（不给结论），与写门
// `family_gate_matrix.go:99` 那处**逐字同一口径**（kind=`usage` · code=`dry_run_yes_conflict`）。
//
// 为什么单独抽一处：同给 = 自相矛盾，**不许**静默挑一枚（此前 18 条命令一律静默走 `--dry-run`
// 支、退 0 —— 人给了 `--yes` 以为写了，实际一个字节没落）。逐条命令在**自己的唯一写盘点之前**
// 调用（未过门前不建件、不建目录、不打任何请求）。
//
// 不成立（两枚未同给）⇒ 返回 `exitOK`，调用点照原样往下走（零行为改动）。
func dryRunYesConflict(inv *invocation, stderr io.Writer) int {
	if !inv.dryRun || !inv.yes {
		return exitOK
	}
	inv.setErr("usage", "dry_run_yes_conflict", "--dry-run 与 --yes 不许同给")
	fmt.Fprintf(stderr, "%s: `--dry-run`（只出计划件）与 `--yes`（真写）**不许同给** ⇒ 用法错 2（两道确认档自相矛盾 ⇒ 不给结论）\n", progName)
	// ★ 机器面读法（补的就是这一行）：默认档（不给 `--json`）下 stdout 恒 0 字节、stderr 这条判词
	//   又与别的「前置检查退 2」看着一样 ⇒ 机器分不出这一发是不是本判定给的。上句人面**一字不动**，
	//   只按本文件既有形状（`cmdGuarded` 的 `confirm_required` 那处）补一行 `error.*` 读法。
	fmt.Fprintf(stderr, "error.kind=usage · detail=dry_run_yes_conflict · retryable=false · remedy=fix_usage\n")
	return exitUsage
}

// ── 危险档「已开放」控制面写面 · 三条真 runner（2026-10-09 · 第一片）────────────────────────────
//
// 病（本单要治的）：`agent load` / `agent unload` / `core reload` 三条**控制面**危险动作原先的
// `run` 是未开放的 `cmdGuarded`（真跑一律拒执 · 退码 2 = 不给结论）⇒ 命令面**够不着**它们
// 已在跑的那三个既有端点（`POST /api/control/load|unload` · `POST /api/config/reload`）。
//
// 治法（照现役同形 · ✗ 不新造传输层）：三条各配一枚**真 runner** —— 干跑出计划件（零副作用）、
// 确认档齐就**真转发**现成端点、命令树登记同批改 `opened: true`。传输层一律复用现役出口：
// `postJSON`（`family_task.go` 全仓唯一写面出口）与 `cmdConfigReload`（`config reload` 的既有真实现，
// 同端点、同「先校验后请求」口径）。令牌只经 `newClient()` 的**唯一入口**取，只进 `X-Auth-Token`
// 头（✗ 不进 argv / URL / 日志）。三态（干跑 / 确认档 / 真跑）与 `sched` / `egg` 同一条口径。

// guardedControlConfirmGate —— D2/D3 确认档（与 `cmdGuarded` 的 ③ 段**逐字同一口径** ·
// `§九 M3 C1/C2` · `§十二 P-014`）：D3 必须给 `--confirm`（值须与目标逐字相同）；
// D2 给了就校验（值须与目标逐字相同 · `§4.1 K7`）；缺 `--yes` 一律 fail-closed（从不提问）。
func guardedControlConfirmGate(inv *invocation, stderr io.Writer, target string) int {
	path := strings.Join(inv.path, " ")
	lvl := dangerD2
	if cmd := find(inv.path); cmd != nil && cmd.danger != nil {
		lvl = cmd.danger.Level
	}
	if lvl == dangerD3 && !inv.confirmGiven {
		msg := fmt.Sprintf("`%s` 是 %s 档（不可逆）——**缺确认 ⇒ 不执行**", path, lvl)
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=<%s> --yes\n", target)
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
		msg := fmt.Sprintf("`%s` 是 %s 档 —— **缺 --yes ⇒ 不执行**", path, lvl)
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "先看计划件：%s %s --dry-run\n", progName, path)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	return exitOK
}

// agentActLevel / agentActNeed / agentActEffect —— 装载与卸载两档共用的几处（按档现算 · 与确认档同源）。
func agentActLevel(action string) string {
	if action == eggActUnload {
		return dangerD3
	}
	return dangerD2
}

func agentActNeed(action string) string {
	if action == eggActUnload {
		return "--confirm=<机器名> 与 --yes 同时到"
	}
	return "--yes"
}

func agentActEffect(action string) string {
	if action == eggActUnload {
		return "把该机器上的模型卸掉（**在跑的任务会被打断**）· 反动作 = `zerg agent load`"
	}
	return "把模型装到该机器上（**占内存/显存槽** · 单槽机是串行的）· 反动作 = `zerg agent unload`"
}

func agentActVerb(action string) string {
	if action == eggActUnload {
		return "unload"
	}
	return "load"
}

// agentActGuarded —— `agent load` / `agent unload` 两档**共用**的唯一执行门（差别只在端点、
// 请求体、几个词 —— 不各写一份）。确认档齐 ⇒ 真打控制层**既有**路由（✗ 不新开一条路）。
func agentActGuarded(inv *invocation, stdout, stderr io.Writer, action string) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	cmd := find(inv.path)
	act := "agent " + agentActVerb(action)
	// 位置参数：<机器名> <模型>（两枚都不许缺 / 不许空 —— 缺就 fail-closed，不猜）。
	if len(inv.args) < 2 || strings.TrimSpace(inv.args[0]) == "" || strings.TrimSpace(inv.args[1]) == "" {
		inv.setErr("usage", "missing_target", "要两个位置参数：<机器名> <模型>")
		fmt.Fprintf(stderr, "%s: `%s` 要两个位置参数：`<机器名> <模型>`（例：%s %s Mr2109 deepseek-v4-flash）⇒ 退码 2\n",
			progName, act, progName, act)
		return exitUsage
	}
	machine, model := strings.TrimSpace(inv.args[0]), strings.TrimSpace(inv.args[1])
	endpoint := eggActEndpoint(action)
	// ① 状态面先判（幂等 · 与未开放档同一条只读探测）：`agent load` 已在该机装载态 ⇒ 按「已在该状态」报。
	if rc, done := probeStateConflict(inv, cmd, stdout, stderr); done {
		return rc
	}
	// ② 干跑：计划件（零副作用 · 一个请求都不发）。
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未打任何请求）")
		fmt.Fprintf(stdout, "  动作     : %s\n", act)
		fmt.Fprintf(stdout, "  目标     : 机器 %s · 模型 %s\n", machine, model)
		fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, endpoint)
		fmt.Fprintf(stdout, "  它会动   : %s\n", agentActEffect(action))
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `%s`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n",
			agentActNeed(action), agentActLevel(action))
		fmt.Fprintf(stdout, "  令牌     : 只从凭据链取、走 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）\n")
		fmt.Fprintf(stdout, "  来源     : §三 B 族 · §九 M4/M5 · 控制面 POST %s（命令面不新开一条路）\n", endpoint)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// ③ 确认档（fail-closed）：D2 缺 `--yes` / D3 缺 `--confirm`+`--yes` ⇒ 拒执（零副作用 · 零请求）。
	if rc := guardedControlConfirmGate(inv, stderr, machine); rc != exitOK {
		return rc
	}
	// ④ 真转发：一条 POST（**唯一**写面出口 —— `postJSON` 不新开路径 · 不发明传输层）。
	body := fmt.Sprintf("{\"machine\":%s,\"model\":%s}", jstr(machine), jstr(model))
	if action == eggActUnload {
		body = fmt.Sprintf("{\"machine\":%s}", jstr(machine))
	}
	status, respBody, err := postJSON(newClient(), endpoint, body)
	if err != nil {
		kind, detail := "unreachable", "dial_failed"
		if _, ok := err.(*httpError); ok {
			// 主控回了非 2xx：码**逐字透传**（命令面不翻译）。
			kind, detail = "upstream_error", fmt.Sprintf("http_%d", status)
			if code := errorCodeOf(respBody); code != "" {
				detail = code
				switch code {
				case "MODEL_NOT_FOUND", "MODEL_NOT_DEPLOYED", "MACHINE_NOT_FOUND":
					kind = "usage" // 「不是错、是没有」那一族（前置不在 ⇒ 用法错 2）
				}
			}
		}
		inv.setErr(kind, detail, fmt.Sprintf("控制面 %s 拒了这次请求", endpoint))
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ %v\n", progName, newClient().base, endpoint, err)
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s\n",
			kind, orDash(detail), retryableOf(kind), remedyOf(kind))
		return codeOfKind(kind)
	}
	changed := true
	inv.changed = &changed
	fmt.Fprintf(stderr, "%s: 控制面已受理 `%s`（机器 %s · 模型 %s）· 主控原样响应: %s\n",
		progName, act, machine, model, strings.TrimSpace(respBody))
	return exitOK
}

// cmdAgentLoadGuarded —— `zerg agent load <机器名> <模型>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 控制面 `POST /api/control/load`）。
func cmdAgentLoadGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return agentActGuarded(inv, stdout, stderr, eggActLoad)
}

// cmdAgentUnloadGuarded —— `zerg agent unload <机器名> <模型>` 的唯一执行门（D3 · 确认档齐 ⇒ 真转发
// 控制面 `POST /api/control/unload`）。
func cmdAgentUnloadGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return agentActGuarded(inv, stdout, stderr, eggActUnload)
}

// cmdCoreReloadGuarded —— `zerg core reload` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发主控
// `POST /api/config/reload`）。底层真源 = `config reload` 的既有实现 `cmdConfigReload`
// （**同一端点、同一「先校验、失败不发请求」口径** —— 两条命令同源，✗ 不各写一份传输）。
func cmdCoreReloadGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未打任何请求）")
		fmt.Fprintf(stdout, "  动作     : core reload\n")
		fmt.Fprintf(stdout, "  请求     : POST %s/api/config/reload（路由**已在跑的主控上** ⇒ 主控零改动）\n", newClient().base)
		fmt.Fprintf(stdout, "  先校验   : 名册件 `gateway/fleet.yaml` 本地解析（走 `cmdConfigReload` 同一实现）—— 解析不过 ⇒ 一个请求都不发\n")
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不请求）\n", dangerD2)
		fmt.Fprintf(stdout, "  令牌     : 只从凭据链取、走 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）\n")
		fmt.Fprintf(stdout, "  来源     : §三 C 族 · §6.3 S5 · 控制面 POST /api/config/reload\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `core reload` 是 D2 档（生效面即时）—— **缺 --yes ⇒ 不执行**（先看计划件：加 `--dry-run`）\n", progName)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	// 真跑：交给 `config reload` 的既有实现（同端点 · 先校验后请求 · 打不到退 12 · 主控拒绝退 1）。
	return cmdConfigReload(inv, stdout, stderr)
}

// ── 危险档「已开放」可逆/查询面 · 五条真 runner（2026-10-09 · 第二片）────────────────────────
//
// 病 / 治法与第一片（`agent load`/`agent unload`/`core reload`）**逐字同源**：`model start|stop` ·
// `itask start|run` · `task move` 五条危险动作原先的 `run` 是未开放的 `cmdGuarded`（真跑一律拒执
// 退 2 = 不给结论）⇒ 命令面够不着它们已在跑的既有端点。本片给五条各配一枚真 runner ——
// 干跑出计划件（零副作用）、确认档齐就真转发现成端点、命令树登记同批改 `opened: true`。
// 传输层一律复用现役出口（`postJSON`；`itask` 两条复用本族既有 `itaskPost` —— 同族同源），
// 令牌只经 `newClient()` 唯一入口取、只进 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）。
// 三态与 `agent load` 同一条口径、确认档一律走 `guardedControlConfirmGate`（D2 需 `--yes`；
// D3 需 `--confirm=<目标>` 与 `--yes` 同时到）。

const (
	modelActStart = "start"
	modelActStop  = "stop"
)

// modelActPath —— 动作 + 模型名 → 端点路径（逐字取 `zerg-core/main.go:173/174` 的路由，不新开路径）。
func modelActPath(action, name string) string {
	return "/api/models/" + url.PathEscape(name) + "/" + action
}

// modelActLevel / modelActNeed / modelActEffect —— 两档共用的几处（按档现算 · 与确认档同源）。
func modelActLevel(action string) string {
	if action == modelActStop {
		return dangerD3
	}
	return dangerD2
}

func modelActNeed(action string) string {
	if action == modelActStop {
		return "--confirm=<模型 id> 与 --yes 同时到"
	}
	return "--yes"
}

func modelActEffect(action string) string {
	if action == modelActStop {
		return "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500 —— 反动作 = `zerg model start`"
	}
	return "把模型装载起来（占槽位 · 单槽机要排队）—— 反动作 = `zerg model stop`"
}

// modelActGuarded —— `model start` / `model stop` 两档**共用**的唯一执行门（差别只在端点、档、几个词）。
// 确认档齐 ⇒ 真打模型控制面**既有**路由（✗ 不新开一条路）。
func modelActGuarded(inv *invocation, stdout, stderr io.Writer, action string) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	act := "model " + action
	if len(inv.args) < 1 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺模型 id")
		fmt.Fprintf(stderr, "%s: `%s` 要一个模型 id（先 `%s model ls`）⇒ 退码 2\n", progName, act, progName)
		return exitUsage
	}
	name := strings.TrimSpace(inv.args[0])
	path := modelActPath(action, name)
	// ① 干跑：计划件（零副作用 · 一个请求都不发）。
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未打任何请求）")
		fmt.Fprintf(stdout, "  动作     : %s\n", act)
		fmt.Fprintf(stdout, "  目标     : 模型 %s\n", name)
		fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, path)
		fmt.Fprintf(stdout, "  它会动   : %s\n", modelActEffect(action))
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `%s`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n",
			modelActNeed(action), modelActLevel(action))
		fmt.Fprintf(stdout, "  令牌     : 只从凭据链取、走 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）\n")
		fmt.Fprintf(stdout, "  来源     : §三 E 族 · §九 M4/M5 · 控制面 POST %s（命令面不新开一条路）\n", path)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// ② 确认档（fail-closed）：D2 缺 `--yes` / D3 缺 `--confirm`+`--yes` ⇒ 拒执（零副作用 · 零请求）。
	if rc := guardedControlConfirmGate(inv, stderr, name); rc != exitOK {
		return rc
	}
	// ③ 真转发：一条 POST（**唯一**写面出口 —— `postJSON` 不新开路径 · 不发明传输层）。
	status, respBody, err := postJSON(newClient(), path, "")
	if err != nil {
		kind, detail := "unreachable", "dial_failed"
		if he, ok := err.(*httpError); ok {
			// 主控回了非 2xx：码**逐字透传**（命令面不翻译）。
			status, respBody = he.status, he.body
			kind, detail = "upstream_error", fmt.Sprintf("http_%d", status)
			if status == 401 || status == 403 {
				kind, detail = "unauthenticated", fmt.Sprintf("http_%d", status)
			} else if code := errorCodeOf(respBody); code != "" {
				detail = code
				switch code {
				case "MODEL_NOT_FOUND", "MODEL_NOT_DEPLOYED", "MACHINE_NOT_FOUND":
					kind = "usage" // 「不是错、是没有」那一族（前置不在 ⇒ 用法错 2）
				}
			}
		}
		inv.setErr(kind, detail, fmt.Sprintf("控制面 %s 拒了这次请求", path))
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ %v\n", progName, newClient().base, path, err)
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s\n",
			kind, orDash(detail), retryableOf(kind), remedyOf(kind))
		return codeOfKind(kind)
	}
	// ④ 成功：摊成人面/机器面（字段名逐字取载荷 —— 不猜键名）。
	var resp jsonObj
	if jerr := json.Unmarshal([]byte(respBody), &resp); jerr != nil {
		inv.setErr("upstream_error", "bad_model_payload", jerr.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	want := []string{"name", "host", "status", "changed"}
	row := map[string]string{}
	keys := []string{}
	for _, k := range want {
		if _, ok := resp[k]; ok {
			keys = append(keys, k)
			row[k] = cell(resp[k])
		}
	}
	if len(keys) == 0 {
		inv.setErr("blocked", "unexpected_payload", "载荷里没有期望的键（不给结论）")
		fmt.Fprintf(stderr, "%s: %s 的载荷里没有期望的键（%v）⇒ 不给结论（退码 8）\n", progName, path, want)
		return exitBlocked
	}
	ch := true
	if v, ok := resp["changed"]; ok {
		if b, ok2 := v.(bool); ok2 {
			ch = b
		}
	}
	inv.changed = &ch
	return listCmd(inv, stdout, stderr, keys, []map[string]string{row})
}

// cmdModelStartGuarded —— `zerg model start <模型 id>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 控制面 `POST /api/models/{name}/start`）。
func cmdModelStartGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return modelActGuarded(inv, stdout, stderr, modelActStart)
}

// cmdModelStopGuarded —— `zerg model stop <模型 id>` 的唯一执行门（D3 · 确认档齐 ⇒ 真转发
// 控制面 `POST /api/models/{name}/stop`）。
func cmdModelStopGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return modelActGuarded(inv, stdout, stderr, modelActStop)
}

// ── itask 引擎启停与手动执行（D2 · 复用本族既有 `itaskPost`）──────────────────────────────

// cmdItaskStartGuarded —— `zerg itask start` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 主控 `POST /api/internal-tasks/start`）。
func cmdItaskStartGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, schedPlanHeader)
		fmt.Fprintf(stdout, "  动作     : %s itask start\n", progName)
		fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再 `itask stop` 即回原态；要 --yes）\n", dangerD2)
		fmt.Fprintf(stdout, "  请求     : POST %s/api/internal-tasks/start\n", newClient().base)
		fmt.Fprintf(stdout, "  它会动   : 让内部任务引擎开始按周期跑（会自动占机器与模型槽）—— 反动作 = `%s itask stop`\n", progName)
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n", dangerD2)
		fmt.Fprintf(stdout, "  来源     : §7.1 P8 · §5.1 POST /api/internal-tasks/start · 开工单 T-49 · 同族写面先例 `itask stop`（D2）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// 群级开关：无点名目标 ⇒ 确认档只判 `--yes`（与 `core reload` 同口径）。
	if rc := guardedControlConfirmGate(inv, stderr, ""); rc != exitOK {
		return rc
	}
	return itaskPost(inv, stdout, stderr, "/api/internal-tasks/start", "启动内部任务")
}

// cmdItaskRunGuarded —— `zerg itask run <任务 id>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 主控 `POST /api/internal-tasks/{id}/run`）。
func cmdItaskRunGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) < 1 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺任务 id")
		fmt.Fprintf(stderr, "%s: `itask run` 要一个任务 id（先 `%s itask ls`）⇒ 退码 2\n", progName, progName)
		return exitUsage
	}
	id := strings.TrimSpace(inv.args[0])
	path := "/api/internal-tasks/" + url.PathEscape(id) + "/run"
	if inv.dryRun {
		fmt.Fprintln(stdout, schedPlanHeader)
		fmt.Fprintf(stdout, "  动作     : %s itask run\n", progName)
		fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 跑到的不改周期；要 --yes）\n", dangerD2)
		fmt.Fprintf(stdout, "  目标     : %s\n", id)
		fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, path)
		fmt.Fprintf(stdout, "  它会动   : 立刻跑一次该内部任务（绕过它的冷却 · 会占机器）\n")
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n", dangerD2)
		fmt.Fprintf(stdout, "  来源     : §7.1 P8 · §5.1 POST /api/internal-tasks/{id}/run · 开工单 T-49 · 同族写面先例 `itask stop`（D2）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if rc := guardedControlConfirmGate(inv, stderr, id); rc != exitOK {
		return rc
	}
	return itaskPost(inv, stdout, stderr, path, "运行内部任务")
}

// ── task move（D2 · 转发主控 `POST /api/tasks/{id}/move?action=…`）────────────────────────

// taskMoveActions —— `--to` 的闭集（4 值 · **逐字**对齐主控 `TaskMoveHandler` 的 `action=top|bottom|up|down`）。
var taskMoveActions = []string{"top", "bottom", "up", "down"}

// taskMoveActionOK —— 判一个取值在不在闭集内（**大小写敏感 · 逐字相同**；不 trim、不猜）。
func taskMoveActionOK(v string) bool {
	for _, a := range taskMoveActions {
		if a == v {
			return true
		}
	}
	return false
}

// cmdTaskMoveGuarded —— `zerg task move <任务 id> --to <位置>` 的唯一执行门（D2 · 确认档齐 ⇒
// 真转发主控 `POST /api/tasks/{id}/move?action=<位置>`）。
func cmdTaskMoveGuarded(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) < 1 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺任务 id")
		fmt.Fprintf(stderr, "%s: `task move` 要一个任务 id（先 `%s task ls`）⇒ 退码 2\n", progName, progName)
		return exitUsage
	}
	to := strings.TrimSpace(inv.to)
	if to == "" {
		inv.setErr("usage", "missing_flag", "缺 --to <位置>")
		fmt.Fprintf(stderr, "%s: `task move` 要 `--to <位置>`（闭集：%s）—— 位置必填（主控缺 action 回 MISSING_ACTION）\n",
			progName, strings.Join(taskMoveActions, "|"))
		return exitUsage
	}
	if !taskMoveActionOK(to) {
		inv.setErr("usage", "bad_flag_value", "位置不在闭集（top|bottom|up|down）")
		fmt.Fprintf(stderr, "%s: `--to` 的值要在闭集内（%s）—— 给的是 %q\n",
			progName, strings.Join(taskMoveActions, "|"), to)
		return exitUsage
	}
	id := strings.TrimSpace(inv.args[0])
	path := "/api/tasks/" + url.PathEscape(id) + "/move?action=" + url.QueryEscape(to)
	if inv.dryRun {
		fmt.Fprintln(stdout, schedPlanHeader)
		fmt.Fprintf(stdout, "  动作     : %s task move\n", progName)
		fmt.Fprintf(stdout, "  危险档   : %s（有影响但可逆 —— 再挪回原处即回原态；要 --yes）\n", dangerD2)
		fmt.Fprintf(stdout, "  目标     : 任务 %s → 位置 %s\n", id, to)
		fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, path)
		fmt.Fprintf(stdout, "  它会动   : 改这条任务在队列里的次序（可能插到别人前面）\n")
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n", dangerD2)
		fmt.Fprintf(stdout, "  来源     : §三 D 族 · §5.1 POST /api/tasks/{id}/move · 开工单 T-44 · 同族写面先例 `sched set`（D2）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if rc := guardedControlConfirmGate(inv, stderr, id); rc != exitOK {
		return rc
	}
	status, respBody, err := postJSON(newClient(), path, "")
	if err != nil {
		kind, detail := "unreachable", "dial_failed"
		if he, ok := err.(*httpError); ok {
			status, respBody = he.status, he.body
			kind, detail = "upstream_error", fmt.Sprintf("http_%d", status)
			if status == 401 || status == 403 {
				kind, detail = "unauthenticated", fmt.Sprintf("http_%d", status)
			} else if code := errorCodeOf(respBody); code != "" {
				detail = code
				switch code {
				case "MISSING_ACTION", "TASK_MOVE_FAILED":
					kind = "usage" // 「不是错、是没有」那一族（目标不在/位置非法 ⇒ 用法错 2）
				}
			}
		}
		inv.setErr(kind, detail, fmt.Sprintf("控制面 %s 拒了这次请求", path))
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ %v\n", progName, newClient().base, path, err)
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s\n",
			kind, orDash(detail), retryableOf(kind), remedyOf(kind))
		return codeOfKind(kind)
	}
	var resp jsonObj
	if jerr := json.Unmarshal([]byte(respBody), &resp); jerr != nil {
		inv.setErr("upstream_error", "bad_task_move_payload", jerr.Error())
		fmt.Fprintf(stderr, "%s: 主控回了 2xx 但响应体不是对象 ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	want := []string{"task_id", "action", "ok"}
	row := map[string]string{}
	keys := []string{}
	for _, k := range want {
		if _, ok := resp[k]; ok {
			keys = append(keys, k)
			row[k] = cell(resp[k])
		}
	}
	if len(keys) == 0 {
		inv.setErr("blocked", "unexpected_payload", "载荷里没有期望的键（不给结论）")
		fmt.Fprintf(stderr, "%s: %s 的载荷里没有期望的键（%v）⇒ 不给结论（退码 8）\n", progName, path, want)
		return exitBlocked
	}
	ch := true
	inv.changed = &ch
	return listCmd(inv, stdout, stderr, keys, []map[string]string{row})
}

// ── task pause / task resume / task retry（D2 · 转发主控既有端点）────────────────────────────────
//
// 病 / 治法与第二片（`model start|stop` 等五条）**逐字同源**：`task pause` / `task resume` / `task retry`
// 三条危险动作原先的 `run` 是未开放的 `cmdGuarded`（真跑一律拒执 · 退码 2 = 不给结论）⇒ 命令面够不着
// 它们已在跑的既有端点。三条各配一枚真 runner —— 干跑出计划件（零副作用）、确认档齐就真转发现成端点、
// 命令树登记同批改 `opened: true`。三条共用**同一枚**执行门 `taskQueueActGuarded`：pause/resume 差别
// 只在 `pause=true|false`（主控同一个端点 `POST /api/tasks/{id}/pause?pause=<bool>` 由
// `PauseTask(taskID, pause)` 分派 · `core/internal/api/handlers.go:955` · `master_scheduler.go:1546`）；
// retry 走另一条现役路由 `POST /api/tasks/{id}/retry`（`TaskRetryHandler` · handlers.go:899 ·
// `Scheduler.RetryTask` · failed→queued）。
// 传输层复用现役唯一写面出口 `postJSON`（✗ 不新造传输）；令牌只经 `newClient()` 唯一入口取、
// 只进 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）。三态与 `agent load` / `core reload` 同一条口径、
// 确认档一律走 `guardedControlConfirmGate`（D2 需 `--yes`）。

const (
	taskActPause  = "pause"
	taskActResume = "resume"
	taskActRetry  = "retry"
)

// taskQueueActPath —— 动作 + 任务 id → 端点路径（逐字取现役路由，不新开路径）。
// pause/resume 共用 `POST /api/tasks/{id}/pause`（主控同一个端点按 `pause=<true|false>` 分派暂停/继续 ·
// handlers.go:959 `pause != "false"` ⇒ 默认 true）；retry 走另一条现役路由 `POST /api/tasks/{id}/retry`。
func taskQueueActPath(action, id string) string {
	if action == taskActRetry {
		return "/api/tasks/" + url.PathEscape(id) + "/retry"
	}
	q := "true"
	if action == taskActResume {
		q = "false"
	}
	return "/api/tasks/" + url.PathEscape(id) + "/pause?pause=" + q
}

// taskQueueActEffect —— 三条共用的「它会动」一句（逐字对齐 catalog 的 dangerSpec.Effect · 三档皆 D2）。
func taskQueueActEffect(action string) string {
	switch action {
	case taskActResume:
		return "把暂停的任务放回排队（可能立刻占机器）—— 反动作 = `zerg task pause`"
	case taskActRetry:
		return "把 failed 任务置回 queued（会再占一次机器与模型槽）"
	}
	return "把排队中的任务置为暂停态（可 `zerg task resume` 回来）"
}

// taskQueueActGuarded —— `task pause` / `task resume` 两档**共用**的唯一执行门（差别只在端点 query）。
// 确认档齐 ⇒ 真打主控**既有**路由（✗ 不新开一条路）。报告形状逐字对齐第一片 `agentActGuarded`。
func taskQueueActGuarded(inv *invocation, stdout, stderr io.Writer, action string) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	act := "task " + action
	if len(inv.args) < 1 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺任务 id")
		fmt.Fprintf(stderr, "%s: `%s` 要一个任务 id（先 `%s task ls`）⇒ 退码 2\n", progName, act, progName)
		return exitUsage
	}
	id := strings.TrimSpace(inv.args[0])
	path := taskQueueActPath(action, id)
	// ① 干跑：计划件（零副作用 · 一个请求都不发）。
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未打任何请求）")
		fmt.Fprintf(stdout, "  动作     : %s %s\n", progName, act)
		fmt.Fprintf(stdout, "  目标     : 任务 %s\n", id)
		fmt.Fprintf(stdout, "  请求     : POST %s%s\n", newClient().base, path)
		fmt.Fprintf(stdout, "  它会动   : %s\n", taskQueueActEffect(action))
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（%s 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n", dangerD2)
		fmt.Fprintf(stdout, "  令牌     : 只从凭据链取、走 `X-Auth-Token` 头（✗ 不进 argv / URL / 日志）\n")
		src := "§三 D 族 · §6.3 S5 · 控制面 POST /api/tasks/{id}/pause（命令面不新开一条路）"
		if action == taskActRetry {
			src = "§三 D 族 · §6.3 S5 · 控制面 POST /api/tasks/{id}/retry（命令面不新开一条路）"
		}
		fmt.Fprintf(stdout, "  来源     : %s\n", src)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	// ② 确认档（fail-closed）：D2 缺 `--yes` ⇒ 拒执（零副作用 · 零请求）。
	if rc := guardedControlConfirmGate(inv, stderr, id); rc != exitOK {
		return rc
	}
	// ③ 真转发：一条 POST（**唯一**写面出口 —— `postJSON` 不新开路径 · 不发明传输层）。
	status, respBody, err := postJSON(newClient(), path, "")
	if err != nil {
		kind, detail := "unreachable", "dial_failed"
		if _, ok := err.(*httpError); ok {
			// 主控回了非 2xx：码**逐字透传**（命令面不翻译）。
			kind, detail = "upstream_error", fmt.Sprintf("http_%d", status)
			if code := errorCodeOf(respBody); code != "" {
				detail = code
				switch code {
				case "TASK_PAUSE_FAILED", "TASK_NOT_FOUND", "TASK_RETRY_FAILED":
					kind = "usage" // 「不是错、是没有」那一族（目标不在/不在可重跑态 ⇒ 用法错 2）
				}
			}
		}
		inv.setErr(kind, detail, fmt.Sprintf("控制面 %s 拒了这次请求", path))
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ %v\n", progName, newClient().base, path, err)
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=%t · remedy=%s\n",
			kind, orDash(detail), retryableOf(kind), remedyOf(kind))
		return codeOfKind(kind)
	}
	changed := true
	inv.changed = &changed
	fmt.Fprintf(stderr, "%s: 控制面已受理 `%s`（任务 %s）· 主控原样响应: %s\n",
		progName, act, id, strings.TrimSpace(respBody))
	return exitOK
}

// cmdTaskPauseGuarded —— `zerg task pause <任务 id>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 主控 `POST /api/tasks/{id}/pause?pause=true`）。
func cmdTaskPauseGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return taskQueueActGuarded(inv, stdout, stderr, taskActPause)
}

// cmdTaskResumeGuarded —— `zerg task resume <任务 id>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 主控 `POST /api/tasks/{id}/pause?pause=false`）。
func cmdTaskResumeGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return taskQueueActGuarded(inv, stdout, stderr, taskActResume)
}

// cmdTaskRetryGuarded —— `zerg task retry <任务 id>` 的唯一执行门（D2 · 确认档齐 ⇒ 真转发
// 主控 `POST /api/tasks/{id}/retry`）。
func cmdTaskRetryGuarded(inv *invocation, stdout, stderr io.Writer) int {
	return taskQueueActGuarded(inv, stdout, stderr, taskActRetry)
}
