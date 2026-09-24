// longtask.go —— 长任务形态（§九 M8 · 调研-M8 §四 条文 1–12）。
//
// 三档旗标（M8 行 262–267）：`--no-wait`（拿句柄就走）/ 默认（阻塞 + 进度）/ `--follow`（跟事件流）。
// **默认档由「读的人是谁」决定**（§十二 `P-020` 合并 M5-P3 + M8-P1 的定案）：
//
//	TTY     ⇒ 默认 `--wait`（人要看进度）
//	非 TTY  ⇒ 默认 `--no-wait`（拿句柄；脚本/agent 不该被挂住）
//
// 句柄（M8 行 268–279）：含 202 与状态监视器指针，带 `attempt` / `run_seq` 消歧
// （同句柄多次执行时「跟的是哪一次」必须能钉住；`--attempt <n>` 可钉旧次，被覆盖即报错）。
//
// 人打断（M8 行 292–297 · §十二 `P-033`）：**默认只退订、不取消**（退 `130`）；
// 要取消得给显式旗标（`--cancel-on-interrupt`）。**ctl-C 不是「失败」**，所以 `130` 单列。
//
// ★ 契约里**零门禁时长数值**（M8 与 M5 `C5` 同一条纪律）：本文件的文案不写任何秒数 ——
// 数值一律来自标定（`scripts/calib/`），没有标定就不许进契约。
package main

import (
	"fmt"
	"strings"
)

// 长任务三档（互斥 · M8 行 262–267）。
const (
	waitAuto   = "auto"    // 没显式给 ⇒ 按 TTY 定（默认档）
	waitWait   = "wait"    // 阻塞 + 进度
	waitNoWait = "no-wait" // 拿句柄就走
	waitFollow = "follow"  // 跟事件流（与 `zerg <对象> watch` **同源**，不许第二条流）
)

// resolveWaitMode —— 默认档由「读的人是谁」决定（`P-020`）。
func resolveWaitMode(inv *invocation) string {
	switch {
	case inv.follow:
		return waitFollow
	case inv.noWait:
		return waitNoWait
	case inv.wait:
		return waitWait
	}
	if inv.tty {
		return waitWait
	}
	return waitNoWait
}

// waitModeWord 给判词用（回显解析后的档 · S6）。
func waitModeWord(mode, ttyStar string) string {
	switch mode {
	case waitWait:
		return "wait（阻塞 + 进度）" + ttyStar
	case waitNoWait:
		return "no-wait（拿句柄就走）" + ttyStar
	case waitFollow:
		return "follow（跟事件流 —— 与 `zerg <对象> watch` 同源）"
	}
	return "未定"
}

// taskHandle 句柄（M8 行 268–279）：形状固定，`attempt` / `run_seq` 用来消歧。
type taskHandle struct {
	Command string
	Target  string
	Attempt int
	RunSeq  string
	Monitor string
}

// newHandle 造一个句柄（本版写不出真句柄 —— 写面未开 ⇒ `run_seq` 标成「未开档」而不是编一个）。
func newHandle(inv *invocation) taskHandle {
	return taskHandle{
		Command: progName + " " + strings.Join(inv.path, " "),
		Target:  firstArg(inv),
		Attempt: 1,
		RunSeq:  "（本版未开档：写面未开，句柄由主控在 202 响应里给 —— §九 M8 行 268–273）",
		Monitor: "GET /api/tasks/{id}（状态监视器指针 · 同源：事件流与它是一条流）",
	}
}

func firstArg(inv *invocation) string {
	if len(inv.args) > 0 {
		return inv.args[0]
	}
	return ""
}

// handleLines 句柄的逐行渲染（进计划件；也让 `--json` 面将来有同一处真源）。
func handleLines(h taskHandle) []string {
	return []string{
		fmt.Sprintf("  句柄     : %s · 目标 %s", h.Command, orDash(h.Target)),
		fmt.Sprintf("  attempt  : %d（同句柄多次执行时跟的是哪一次 · `--attempt <n>` 可钉旧次，被覆盖即报错）", h.Attempt),
		fmt.Sprintf("  run_seq  : %s", h.RunSeq),
		fmt.Sprintf("  监视器   : %s", h.Monitor),
	}
}

// helpLongTasks —— `zerg help long-tasks`（§九 M8）。
func helpLongTasks() string {
	var b strings.Builder
	b.WriteString("长任务形态（§九 M8 · 调研-M8 §四）\n\n")
	b.WriteString("三档旗标（互斥）：\n")
	b.WriteString("  --wait          阻塞 + 进度（进度/日志一律走 stderr，**不许污染 stdout** —— M8 行 325–330）\n")
	b.WriteString("  --no-wait       拿句柄就走（202 + 状态监视器指针）\n")
	b.WriteString("  --follow        跟事件流（**与 `zerg <对象> watch` 同源**，不许第二条流 —— M8 行 286–291）\n\n")
	b.WriteString("默认档由「读的人是谁」决定（§十二 `P-020`）：\n")
	b.WriteString("  TTY    ⇒ 默认 --wait（人要看进度）\n")
	b.WriteString("  非 TTY ⇒ 默认 --no-wait（脚本 / agent 不该被挂住）\n")
	b.WriteString("  秒级/十秒级动作两种都等（`P-020` 逐字：孵化一档本来就短）。\n\n")
	b.WriteString("句柄（M8 行 268–279）：`attempt`（跟第几次）+ `run_seq`（这一次的序号）+ 状态监视器指针；\n")
	b.WriteString("`--attempt <n>` 钉旧次，钉不住就报错（不许静默跟最新一次）。\n\n")
	b.WriteString("人打断（M8 行 292–297 · §十二 `P-033`）：\n")
	b.WriteString("  · **默认只退订、不取消** —— Ctrl-C 退 `130`，被跟的任务**继续跑**。\n")
	b.WriteString("  · 要真取消得给显式旗标 `--cancel-on-interrupt`（取消与否是两件事，不许合成一件）。\n")
	b.WriteString("  · 「等到了坏结果」与「等不起」**必须异码**（超时走 `11`，失败走 `1`）。\n\n")
	b.WriteString("墙钟上界（序138 · 组4 §二.4 `W-57` · 上级裁定 **B** —— 超时一律退 `11`）：\n")
	b.WriteString("  · 全局旗标 `--timeout <时长>`（Go 时长写法：`Ns` / `Nm` / `Nh` / `Nms`；**裸数字按秒读**）——\n")
	b.WriteString("    **每条命令通吃**（谁用谁读；不给这一枚 ⇒ 一个字都不加）。\n")
	b.WriteString("  · 非法时长 ⇒ **先于任何动作**退 `2`（用法错 —— 不是 `1`、也不是 `11`）。\n")
	b.WriteString("  · 到点 ⇒ **先复原再报**：① 停本趟**亲手起的**子进程 ② 逐件还原本趟登记过的**写面前像**\n")
	b.WriteString("    （写前不在盘 ⇒ 删残留），**再**报 ⇒ 退 `11`（「报」不许在「复原」之前）。\n")
	b.WriteString("  · 负控：到点后盘上有残留 ⇒ 判红（成对判据件 `core/cmd/zerg/cli_wallclock_test.go`）。\n")
	b.WriteString("  · 那枚 `2` 的来历是**门（脚本）侧**设计（门侧码空间只有 0/1/2 + 四档）—— 命令面照自家退码表取码。\n\n")
	b.WriteString("时长数值（口径 · 防误读）：**本契约不写任何时长**。超时上限、进度刷新间隔这类数字\n")
	b.WriteString("一律来自**标定**（`scripts/calib/`）—— 没标定就不许写进契约（同 §九 M5 `C5` 的三时钟纪律）。\n")
	return b.String()
}
