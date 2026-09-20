// watch.go —— 事件 / 订阅通道（§九 M1 · 调研-M1 条文 `W1`–`W15` · §十二 `P-001`–`P-005`）。
//
// 形态（`W1` · §十二 `P-001` 的推荐）：**`zerg <对象> watch [<id>]`** —— 动作位，不是对象位。
// ⇒ 事件面**只有一个名字**；`zerg <对象> show --follow` **只是转发**到它，不许长出第二条流
// （`W2`：事件域与请求面同源；`P-005`：`watch` / `show --follow` / `--wait` 三者归一）。
//
// 端点（`W12` · §十二 `P-002`）：**单端点 + `Accept` 内容协商**；不认的媒体类型走 **`406` 兜底**
// （不许「什么都能吃」）。流格式（`W4`）：**行分隔 JSON**（ndjson）。
//
// 保留窗口（`W9` · §十二 `P-003`）：**必须标定、不许估值** —— `P-003` 明文「阻塞契约成文」⇒
// 保留窗口的**数值今天不进契约**；契约里只写「取值来自标定」这条规矩。
//
// 退码（`W11`）：加 `130`（人打断）与 `--exit-on`（跟到某个事件就退）。
//
// ★ 本版状态（如实说）：主控面**今天没有事件端点**（`/api/events` 不存在）⇒ `zerg watch` 出
// **契约声明面 + 不给结论（`2`）**，不假装订阅成功、不拿日志文件当事件流（那是偷偷换源，`O2`）。
package main

import (
	"fmt"
	"io"
	"strings"
)

// 事件面的媒体类型（单端点 + 内容协商 · `W12`；不认的 ⇒ 406 兜底）。
var watchAccepts = []string{"application/x-ndjson", "application/json"}

// 事件 kind 闭集（`W6`：**六名**，第一版只增不改）。
var watchEventKinds = []string{"started", "progress", "succeeded", "failed", "cancelled", "heartbeat"}

// helpWatch —— `zerg help watch`（§九 M1 的自描述面）。
func helpWatch() string {
	var b strings.Builder
	b.WriteString("事件 / 订阅通道（§九 M1 · 调研-M1 条文 `W1`–`W15`）\n\n")
	b.WriteString("**唯一名字**：`zerg <对象> watch [<id>]`（动作位 · §十二 `P-001` 的推荐）\n")
	b.WriteString("  · `zerg <对象> show --follow` **只是转发**到它 —— 不许两套事件语义（`W2`/`P-005`）。\n")
	b.WriteString("  · `--wait` 是**等这一次做完**，`watch` 是**跟这条流**；两者归一，不许各说一套。\n\n")
	b.WriteString("端点：**单端点 + `Accept` 内容协商**（`W12` · §十二 `P-002`）\n")
	b.WriteString("  认的媒体类型: " + strings.Join(watchAccepts, " · ") + "\n")
	b.WriteString("  不认的 ⇒ **`406` 兜底**（不许「什么都能吃」；本版在客户端侧先判，退码 `2`）。\n")
	b.WriteString("流格式（`W4`）：**行分隔 JSON**（一行一事件）；事件字段**第一版只增不改**（`W5`）。\n\n")
	b.WriteString("事件 kind 闭集（`W6` · 六名）：" + strings.Join(watchEventKinds, " · ") + "\n\n")
	b.WriteString("四条不许含糊的规矩：\n")
	b.WriteString("  ① 先拉一次全量、再订阅增量；事件行必带**单调游标**（`W1`/`W3`）。\n")
	b.WriteString("  ② **缺口绝不静默**（`W8`）：保留窗口外的请求**显式报缺口**（照 k8s 410 / etcd ErrCompacted）。\n")
	b.WriteString("  ③ 无事件时**心跳带游标**（`W7`）—— 「没消息」与「断了」必须可区分。\n")
	b.WriteString("  ④ 断线重连带 `start_revision` 一类续点（`W10`）。\n\n")
	b.WriteString("保留窗口（`W9` · §十二 `P-003`）：**必须标定、不许估值** —— 本契约**不写数值**，\n")
	b.WriteString("只写「取值来自标定」这条规矩；`P-003` 明写它**阻塞**契约成文 ⇒ 标定做完才填。\n\n")
	b.WriteString("退码（`W11`）：人打断 ⇒ `130`（默认只退订、不取消 —— 同 §九 M8）；`--exit-on <kind>` 跟到即退。\n")
	b.WriteString("UI 与 CLI：**不许两套事件语义**（§4.3 `U3`）—— UI 读的就是这一条流。\n\n")
	b.WriteString("本版状态（如实说）：主控面**没有**事件端点（`/api/events` 不存在）⇒ `zerg watch` 出\n")
	b.WriteString("**声明面 + 不给结论（`8` · `kind=blocked`）**；**不**拿 `/tmp` 的日志文件当事件流\n")
	b.WriteString("（那是偷偷换源 · §九 M20 `O2`）。\n")
	return b.String()
}

// cmdWatch —— `zerg watch` 与本族各对象的 `watch` 动作共用这一处（唯一名字 · `W1`）。
func cmdWatch(inv *invocation, stdout, stderr io.Writer) int {
	// ① 内容协商（`W12`）：给了 `--accept` 就判；不认的 ⇒ 406 兜底（客户端侧先拒）
	if inv.acceptWant != "" && !containsStr(watchAccepts, inv.acceptWant) {
		fmt.Fprintf(stderr, "%s: 406 Not Acceptable —— 认不得的媒体类型 %q\n", progName, inv.acceptWant)
		fmt.Fprintf(stderr, "认的类型: %s（单端点 + `Accept` 协商 · §九 M1 `W12`）\n", strings.Join(watchAccepts, " · "))
		inv.setErr("usage", "not_acceptable", "不认的 Accept 媒体类型")
		return exitUsage
	}
	// ② 声明面 + 不给结论（端点今天不在 —— 不许假装订阅成功）
	fmt.Fprint(stdout, helpWatch())
	inv.setErr("blocked", "events_endpoint_absent", "主控面今天没有事件端点（/api/events）")
	fmt.Fprintf(stderr, "%s: 主控面**没有**事件端点 ⇒ 订阅起不来，**不给结论**（退码 8）\n", progName)
	fmt.Fprintf(stderr, "error.kind=blocked · retryable=true · remedy=fix_precondition（端点落地后本命令即通）\n")
	return exitBlocked
}

// cmdTaskShowFollow —— `zerg task show --follow` 的**转发**（不许自己再走一条流 · `W2`/`P-005`）。
func cmdTaskShowFollow(inv *invocation, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: `task show --follow` **只作转发** ⇒ 走 `zerg watch <对象>`（唯一名字 · §九 M1 `W1`/`W2`）\n", progName)
	fwd := &invocation{path: []string{"watch"}, args: inv.args, tty: inv.tty, follow: true, acceptWant: inv.acceptWant}
	return cmdWatch(fwd, stdout, stderr)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// cmdTaskShow —— `zerg task show <id>`（只读）：不给 `--follow` 就把这一条任务投影出来；
// 给了 `--follow` ⇒ **转发**到事件面（唯一名字）而不是自己再走一条流。
func cmdTaskShow(inv *invocation, stdout, stderr io.Writer) int {
	if inv.follow {
		return cmdTaskShowFollow(inv, stdout, stderr)
	}
	if len(inv.args) == 0 {
		fmt.Fprintf(stderr, "%s: `task show` 要一个任务 id\n", progName)
		fmt.Fprintf(stderr, "先看队列: %s task ls\n", progName)
		inv.setErr("usage", "missing_target", "缺任务 id")
		return exitUsage
	}
	want := inv.args[0]
	c := newClient()
	var resp jsonObj
	if rc := fetchInv(inv, c, "/api/tasks", &resp, stderr); rc != exitOK {
		return rc
	}
	fields := []string{"id", "status", "model", "machine", "priority", "created_at", "description"}
	var row map[string]string
	for _, it := range asList(resp["tasks"]) {
		o := asObj(it)
		if o == nil || cell(o["id"]) != want {
			continue
		}
		row = project(o, fields)
		break
	}
	if row == nil {
		fmt.Fprintf(stderr, "%s: 队列里没有任务 %q\n", progName, want)
		inv.setErr("failed", "task_not_found", "队列里没有这个任务 id")
		return exitFail
	}
	return listCmd(inv, stdout, stderr, fields, []map[string]string{row})
}
