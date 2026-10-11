// family_gate_ps.go —— `zerg gate ps`：门进程 + 门锁的**现读快照**（只读 · 立即返回）。
//
// 面（缺口 `GAP-20261010-204`）：门跑在飞时人只看得见「有/没有」，看不见**谁在跑、跑什么、锁归谁**。
// 本命令把两段现读数据拼成一张表：
//
//	① 进程段：复用 `gateReapScan()`（读 `ps -Ao pid=,ppid=,pcpu=,etime=,command=` 后只挑门进程）
//	   ＋ `gateReapRoots`/`gateReapLineage`/`workersUnder` ⇒ 每根一行（pid/ppid/orphan/etime/pcpu/
//	   token/lineage/work/cmd）。读不到进程表 ⇒ **判不了 ⇒ 退 8**（✗ 不当「零进程」）。
//	② 锁段：读 `<锁目录>/zerg-gate-instance/gate-run.lock` ⇒ 一行（lock_held/lock_pid/lock_start/
//	   lock_step/lock_cmd）。锁不在 ⇒ `lock_held=no`（**非错、不误红**）。
//
// 四条口径：① **只读**——不杀不删不写（动作面归 `gate reap`）；② 拿不准就退 8（✗ 不假绿）；
// ③ `--json` 走现成 `listCmd`（✗ 不自造字段表）；④ 零私有绝对路径。
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// gatePSFields —— `gate ps --json` 的全部字段（进程段 10 格 ＋ 锁段 5 格）。
var gatePSFields = []string{"pid", "ppid", "root", "orphan", "etime", "pcpu", "token", "lineage",
	"work", "cmd", "lock_held", "lock_pid", "lock_start", "lock_step", "lock_cmd"}

// cmdGatePS —— `zerg gate ps [--json <字段>]`。签名与 `cmdGateReap` 同形。
func cmdGatePS(inv *invocation, stdout, stderr io.Writer) int {
	tail := inv.orig
	if len(inv.path) <= len(inv.orig) {
		tail = inv.orig[len(inv.path):]
	}
	for _, a := range tail {
		if !strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "%s: `gate ps` 不收位置参数（给了 %q）⇒ 退码 2\n", progName, a)
			return exitUsage
		}
	}

	gate, ok := gateReapScan()
	if !ok {
		inv.setErr("blocked", "ps_unreadable", "进程表读不到")
		fmt.Fprintf(stderr, "%s: `ps` 取不到/为空 ⇒ 判不了 ⇒ **不给结论（退码 8）**（读不到不当没有）\n", progName)
		return exitBlocked
	}
	allPPID, cmds := gateReapAllPPID(), gateReapAllCmds()
	rows := []map[string]string{}
	for _, r := range gateReapRoots(gate) {
		gp := gate[r]
		ls := []string{}
		for _, p := range gateReapLineage(r, allPPID, gate) {
			ls = append(ls, strconv.Itoa(p))
		}
		rows = append(rows, map[string]string{
			"pid":     strconv.Itoa(r),
			"ppid":    strconv.Itoa(gp.ppid),
			"root":    strconv.Itoa(r),
			"orphan":  boolAbbr(gp.ppid == 1),
			"etime":   gp.etime,
			"pcpu":    strconv.FormatFloat(gp.pcpu, 'f', 1, 64),
			"token":   dashIfEmpty(gp.tok),
			"lineage": strings.Join(ls, ","),
			"work":    strconv.Itoa(workersUnder(r, allPPID, cmds)),
			"cmd":     gp.cmd,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["pid"] < rows[j]["pid"] })
	rows = append(rows, gatePSLockRow())

	if !inv.jsonGiven {
		fmt.Fprintf(stdout, "%s: 门进程 %d 根（现读快照 · 只读）\n", progName, len(rows)-1)
		for _, r := range rows {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n", r["pid"], r["etime"], r["pcpu"], r["cmd"])
		}
		return exitOK
	}
	if len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	return listCmd(inv, stdout, stderr, gatePSFields, rows)
}

// gatePSLockRow —— 读锁件 ⇒ 一条锁行（5 键）；锁不在/读不到 ⇒ `lock_held=no`（非错 · 不误红）。
func gatePSLockRow() map[string]string {
	row := map[string]string{"lock_held": "no", "lock_pid": "-", "lock_start": "-", "lock_step": "-", "lock_cmd": "-"}
	b, err := os.ReadFile(gateInstanceLockPath())
	if err != nil || strings.TrimSpace(string(b)) == "" {
		return row
	}
	row["lock_held"] = "yes"
	for _, ln := range strings.Split(string(b), "\n") {
		k, v, cut := strings.Cut(ln, ":")
		if !cut {
			continue
		}
		switch strings.TrimSpace(k) {
		case "pid":
			row["lock_pid"] = strings.TrimSpace(v)
		case "start":
			row["lock_start"] = strings.TrimSpace(v)
		case "step":
			row["lock_step"] = strings.TrimSpace(v) // 现行锁件无此行 ⇒ 保持 "-"
		case "cmd":
			row["lock_cmd"] = strings.TrimSpace(v)
		}
	}
	return row
}
