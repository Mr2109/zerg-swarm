// family_gate_reap.go —— `zerg gate reap`：**陈旧/卡死门跑根的合法回收**（本单 · lane B258）。
//
// 病（现读 2026-09-30）：闸口的「别的根数」判据（`scripts/gates/precommit-gates.sh` 的
// `gate_root_count_check`）把**任何**别的 `precommit-gates.sh` 根都算成「有别的门跑在飞」⇒
// 一次被孤儿化的门跑（父壳退出、`ppid=1`）会让**此后每一趟**门跑都退 2、拿不到任何读数
// —— 而正门里**没有**任何一条能把这种残儿根收掉的口令（人手 `kill` 是不可追溯的野路子，
// 且会误伤真活工）。
//
// 本件补的就是那一条口令。判据面**只读**（只读进程表）；动作面**只收根**（只终止根**及其门谱系**，
// 绝不碰非门进程 —— `go test`/`go build` 一类在飞工一个都不动）。
//
// 四条口径：
//
//	① 分类（`--stale`）—— 一个门根算**陈旧** iff 它不是本趟自身树，且满足其一：
//	   · 门谱系令牌里那个门 pid **已不在进程表**（= 死趟残儿）；或
//	   · 祖链已断（`ppid == 1` —— 拉起它的那一层壳已经没了 ⇒ 这趟门不可能再被正确收集）。
//	   其余（父壳还在 = 真在飞的门跑）一律**跳过并如实报**，一个字节都不动。
//	② 回收单位 = 根**及其门谱系**（`precommit-gates.sh` 后代，递归）—— 只掐根会立刻把子门
//	   变成新的根（实测：43 根之下另有 43 个门），等式收不干净。非门后代（真活工）**不碰**。
//	③ 每个收回动作落审计（append-only jsonl · 仓外 `<锁目录>/reap-audit.jsonl`），失败即出声。
//	④ 绝不 `kill` 通配、绝不删锁文件、绝不 `ZERG_GATE_NO_LOCK`/`ZERG_GATE_LOCK_HELD` 硬绕 ——
//	   本命令**只**按 pid 逐枚发信号。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const gateReapScriptTag = "precommit-gates.sh"

// gateReapFields —— `gate reap --json` 的全部字段（K1：机器面先定）。
var gateReapFields = []string{"pid", "ppid", "token", "orphan", "verdict", "lineage", "work", "reason", "cmd"}

// gateProc —— 进程表里一行门进程。
type gateProc struct {
	pid  int
	ppid int
	cmd  string
	tok  string
}

// gateReapScan —— 读进程表（`ps -Ao pid=,ppid=,command=`），只挑门进程。
// 取不到 ⇒ 第二个返回值为假（调用方判「读不到不当没有」⇒ 不给结论）。
func gateReapScan() (map[int]gateProc, bool) {
	out, err := exec.Command("ps", "-Ao", "pid=,ppid=,command=").Output()
	if err != nil || len(out) == 0 {
		return nil, false
	}
	m := map[int]gateProc{}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 3 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil {
			continue
		}
		cmd := strings.TrimSpace(ln[strings.Index(ln, f[2]):])
		if !strings.Contains(cmd, gateReapScriptTag) {
			continue
		}
		gp := gateProc{pid: pid, ppid: ppid, cmd: cmd}
		if k := strings.Index(cmd, "--zerg-lane-token="); k >= 0 {
			rest := cmd[k+len("--zerg-lane-token="):]
			if j := strings.IndexAny(rest, " \t"); j >= 0 {
				rest = rest[:j]
			}
			gp.tok = rest
		}
		m[pid] = gp
	}
	return m, true
}

// gateReapSelfPids —— 本趟自身树（`os.Getpid()` 起沿 `ppid` 链，含 `$PPID` 及上一切祖先）+
// 本趟门根自己（判据只数**别的**根，本条树永不在列）。
func gateReapSelfPids(gate map[int]gateProc, allPPID map[int]int) map[int]bool {
	self := map[int]bool{}
	p := os.Getpid()
	for i := 0; i < 256 && p > 1; i++ {
		self[p] = true
		pp, ok := allPPID[p]
		if !ok {
			break
		}
		p = pp
	}
	return self
}

// gateReapAllPPID —— 全表 pid ⇒ ppid（判自身树要全表，不只门表）。
func gateReapAllPPID() map[int]int {
	out, _ := exec.Command("ps", "-Ao", "pid=,ppid=").Output()
	m := map[int]int{}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		if e1 == nil && e2 == nil {
			m[pid] = ppid
		}
	}
	return m
}

// workersUnder —— 一棵门根下**非门**的「在飞工」（`go test`/`go build`/`go vet`/`cargo`/`rustc`）。
// 只数、不碰 —— 回收动作**绝不**终止它们。
func workersUnder(root int, allPPID map[int]int, cmds map[int]string) int {
	kids := map[int][]int{}
	for p, pp := range allPPID {
		kids[pp] = append(kids[pp], p)
	}
	n := 0
	var walk func(int)
	seen := map[int]bool{}
	walk = func(p int) {
		for _, c := range kids[p] {
			if seen[c] {
				continue
			}
			seen[c] = true
			cs := cmds[c]
			if strings.Contains(cs, gateReapScriptTag) {
				walk(c)
				continue
			}
			for _, w := range []string{"go test", "go build", "go vet", "cargo ", "rustc", "go-binary"} {
				if strings.Contains(cs, w) {
					n++
					break
				}
			}
			walk(c)
		}
	}
	walk(root)
	return n
}

// gateReapAllCmds —— 全表 pid ⇒ command（判在飞工要用全表）。
func gateReapAllCmds() map[int]string {
	out, _ := exec.Command("ps", "-Ao", "pid=,command=").Output()
	m := map[int]string{}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			continue
		}
		i := strings.Index(ln, f[1])
		if i < 0 {
			continue
		}
		m[pid] = strings.TrimSpace(ln[i:])
	}
	return m
}

// gateReapRoots —— 门根 = 祖链上**没有第二个门**的门进程（与自举件 `gate_root_count_check` 同一算法）。
func gateReapRoots(gate map[int]gateProc) []int {
	roots := []int{}
	for g := range gate {
		p := gate[g].ppid
		isroot := true
		for guard := 0; guard < 256 && p > 1; guard++ {
			if _, ok := gate[p]; ok {
				isroot = false
				break
			}
			next := 0
			if gp, ok := gate[p]; ok {
				next = gp.ppid
			} else {
				// 非门祖先：用全表续走
				next = gateReapPPIDOf(p)
			}
			if next == p {
				break
			}
			p = next
		}
		if isroot {
			roots = append(roots, g)
		}
	}
	sort.Ints(roots)
	return roots
}

var gateReapALLPPIDcache map[int]int

func gateReapPPIDOf(pid int) int {
	if gateReapALLPPIDcache == nil {
		gateReapALLPPIDcache = gateReapAllPPID()
	}
	return gateReapALLPPIDcache[pid]
}

// gateReapLineage —— 根 + 其门谱系后代（递归），升序。
func gateReapLineage(root int, allPPID map[int]int, gate map[int]gateProc) []int {
	kids := map[int][]int{}
	for p, pp := range allPPID {
		kids[pp] = append(kids[pp], p)
	}
	out := []int{root}
	seen := map[int]bool{root: true}
	var walk func(int)
	walk = func(p int) {
		for _, c := range kids[p] {
			if seen[c] {
				continue
			}
			if _, ok := gate[c]; !ok {
				continue // 非门后代（在飞工）不碰、不入谱系
			}
			seen[c] = true
			out = append(out, c)
			walk(c)
		}
	}
	walk(root)
	sort.Ints(out)
	return out
}

// gateReapAuditPath —— 审计落点（仓外 `<锁目录>/reap-audit.jsonl`，与 no-lock 审计同址）。
func gateReapAuditPath() string {
	return filepath.Join(filepath.Dir(gateInstanceLockPath()), "reap-audit.jsonl")
}

// cmdGateReap —— `zerg gate reap [--stale] [--dry-run | --yes] [--json <字段>]`。
func cmdGateReap(inv *invocation, stdout, stderr io.Writer) int {
	// 旗标面：只认这几枚；未知 ⇒ 用法错 2（执行前判，零副作用）。
	tail := inv.orig
	if len(inv.path) <= len(inv.orig) {
		tail = inv.orig[len(inv.path):]
	}
	known := map[string]bool{"--stale": true, "--dry-run": true, "--yes": true, "--json": true}
	for i := 0; i < len(tail); i++ {
		a := tail[i]
		if !strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "%s: `gate reap` 不收位置参数（给了 %q）⇒ 退码 2\n", progName, a)
			return exitUsage
		}
		n := a
		if k := strings.Index(a, "="); k >= 0 {
			n = a[:k]
		}
		if !known[n] {
			fmt.Fprintf(stderr, "✗ 未知参数: %s\n", a)
			return exitUsage
		}
	}
	if !gateReapHasFlag(inv, "--stale") {
		inv.setErr("usage", "missing_stale", "缺 --stale")
		fmt.Fprintf(stderr, "%s: `gate reap` 要 `--stale`（本版只收陈旧/卡死根；活的门跑绝不碰）⇒ 退码 2\n", progName)
		return exitUsage
	}
	if inv.dryRun {
		if inv.yes {
			return dryRunYesConflict(inv, stderr)
		}
	} else if !inv.yes {
		inv.setErr("usage", "yes_required", "缺 --yes")
		fmt.Fprintf(stderr, "%s: `gate reap --stale` 是**写面**（真终止残儿根）—— 缺 `--yes` ⇒ 不执行 ⇒ 退码 2\n", progName)
		fmt.Fprintf(stderr, "先看清单：%s gate reap --stale --dry-run\n", progName)
		return exitUsage
	}

	gate, ok := gateReapScan()
	if !ok {
		inv.setErr("blocked", "ps_unreadable", "进程表读不到")
		fmt.Fprintf(stderr, "%s: `ps` 取不到/为空 ⇒ 判不了 ⇒ **不给结论（退码 8）**（读不到不当没有）\n", progName)
		return exitBlocked
	}
	allPPID := gateReapAllPPID()
	cmds := gateReapAllCmds()
	self := gateReapSelfPids(gate, allPPID)
	roots := gateReapRoots(gate)

	rows := []gateReapRow{}
	for _, r := range roots {
		if self[r] {
			continue // 本趟自身树：永不算「别的根」
		}
		gp := gate[r]
		orphan := gp.ppid == 1 || allPPID[gp.ppid] == 0
		stale := orphan
		reason := "祖链已断（ppid=1：拉起它的那一层壳已不在）"
		if gp.tok != "" {
			tp := 0
			if k := strings.Index(gp.tok, "-"); k > 0 {
				tp, _ = strconv.Atoi(gp.tok[:k])
			}
			if tp > 0 && allPPID[tp] == 0 {
				stale = true
				reason = fmt.Sprintf("门谱系令牌 pid=%d 已不在进程表（死趟残儿）", tp)
			}
		}
		v := "stale"
		if !stale {
			v = "live"
			reason = "父壳还在 ⇒ 真在飞的门跑 ⇒ **跳过**（绝不碰）"
		}
		rows = append(rows, gateReapRow{pid: r, ppid: gp.ppid, tok: gp.tok, orphan: orphan, verdict: v,
			lineage: gateReapLineage(r, allPPID, gate), work: workersUnder(r, allPPID, cmds),
			reason: reason, cmd: gp.cmd})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pid < rows[j].pid })

	staleRows := []gateReapRow{}
	for _, x := range rows {
		if x.verdict == "stale" {
			staleRows = append(staleRows, x)
		}
	}

	if inv.dryRun {
		fmt.Fprintf(stdout, "干跑单 `zerg gate reap --stale`（只收陈旧/卡死根 · 活的门跑一个不碰）\n")
		fmt.Fprintf(stdout, "  门根数（别的根）: %d（陈旧 %d · 跳过 %d）\n", len(rows), len(staleRows), len(rows)-len(staleRows))
		for _, x := range rows {
			fmt.Fprintf(stdout, "  [%s] 根 pid=%d ppid=%d 谱系=%v 在飞工=%d token=%s\n        理由：%s\n",
				x.verdict, x.pid, x.ppid, x.lineage, x.work, dashIfEmpty(x.tok), x.reason)
		}
		fmt.Fprintf(stderr, "★ 干跑：一个信号都没发、一个字节都没写（`--dry-run` · 零副作用）。\n")
		return gateReapEmit(inv, stdout, stderr, rows)
	}

	// ── 真收：只发信号（SIGTERM → 短暂等待 → SIGKILL 幸存者），逐根入审计 ──
	audit := gateReapAuditPath()
	_ = os.MkdirAll(filepath.Dir(audit), 0o755)
	auditFailed := false
	reaped, sigSent := 0, 0
	for _, x := range staleRows {
		for _, p := range x.lineage {
			if p == os.Getpid() {
				continue
			}
			if err := syscall.Kill(p, syscall.SIGTERM); err == nil {
				sigSent++
			}
		}
		reaped++
		if err := gateReapAudit(audit, x.pid, x.ppid, x.lineage, x.work, x.tok, x.reason, x.cmd); err != nil {
			auditFailed = true
			fmt.Fprintf(stderr, "%s: ⚠ 审计落不了笔（%s）：%v ⇒ 这一格**照实报**\n", progName, audit, err)
		}
	}
	// 短暂等待（前台、有限）后清幸存者。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		alive := 0
		for _, x := range staleRows {
			for _, p := range x.lineage {
				if syscall.Kill(p, 0) == nil {
					alive++
				}
			}
		}
		if alive == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	killed := 0
	for _, x := range staleRows {
		for _, p := range x.lineage {
			if p == os.Getpid() {
				continue
			}
			if syscall.Kill(p, 0) == nil {
				if syscall.Kill(p, syscall.SIGKILL) == nil {
					killed++
				}
			}
		}
	}

	// 收完复读：根数前后。
	gate2, ok2 := gateReapScan()
	roots2 := []int{}
	if ok2 {
		self2 := gateReapSelfPids(gate2, gateReapAllPPID())
		for _, r := range gateReapRoots(gate2) {
			if !self2[r] {
				roots2 = append(roots2, r)
			}
		}
	}
	fmt.Fprintf(stdout, "`zerg gate reap --stale` 收回结果\n")
	fmt.Fprintf(stdout, "  收前别的根数: %d（陈旧 %d · 跳过 %d）\n", len(rows), len(staleRows), len(rows)-len(staleRows))
	fmt.Fprintf(stdout, "  收后别的根数: %d%s\n", len(roots2), map[bool]string{true: "", false: "（复读不到进程表 ⇒ 不给结论）"}[ok2])
	fmt.Fprintf(stdout, "  发信号: %d（SIGTERM）· 升级 SIGKILL: %d · 审计: %s\n", sigSent, killed, audit)
	for _, x := range rows {
		if x.verdict != "stale" {
			fmt.Fprintf(stderr, "⏭ 跳过（活的门跑）：根 pid=%d —— %s\n", x.pid, x.reason)
		}
	}
	if auditFailed {
		inv.setErr("blocked", "audit_unwritable", "审计落笔失败")
		return exitBlocked
	}
	return gateReapEmit(inv, stdout, stderr, rows)
}

func gateReapHasFlag(inv *invocation, name string) bool {
	tail := inv.orig
	if len(inv.path) <= len(inv.orig) {
		tail = inv.orig[len(inv.path):]
	}
	for _, a := range tail {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

type gateReapRow struct {
	pid     int
	ppid    int
	tok     string
	orphan  bool
	verdict string
	lineage []int
	work    int
	reason  string
	cmd     string
}

// gateReapEmit —— `--json <字段>` 的包封（给字段才出；逐行取自本命令自己读的进程表）。
func gateReapEmit(inv *invocation, stdout, stderr io.Writer, rows []gateReapRow) int {
	if !inv.jsonGiven {
		return exitOK
	}
	if len(inv.fields) == 0 {
		return requireFields(inv, stderr)
	}
	out := make([]map[string]string, 0, len(rows))
	for _, x := range rows {
		ls := make([]string, 0, len(x.lineage))
		for _, p := range x.lineage {
			ls = append(ls, strconv.Itoa(p))
		}
		out = append(out, map[string]string{
			"pid": strconv.Itoa(x.pid), "ppid": strconv.Itoa(x.ppid),
			"token": dashIfEmpty(x.tok), "orphan": boolAbbr(x.orphan),
			"verdict": x.verdict, "lineage": strings.Join(ls, ","),
			"work": strconv.Itoa(x.work), "reason": x.reason, "cmd": x.cmd,
		})
	}
	return listCmd(inv, stdout, stderr, gateReapFields, out)
}

// gateReapAudit —— append-only 一行一事件；写不动 ⇒ 返回错误（调用方不给结论）。
func gateReapAudit(path string, root, ppid int, lineage []int, work int, tok, reason, cmd string) error {
	ps := make([]string, 0, len(lineage))
	for _, p := range lineage {
		ps = append(ps, strconv.Itoa(p))
	}
	line := fmt.Sprintf("{\"t\":\"%s\",\"event\":\"gate_reap_stale_root\",\"root\":%d,\"ppid\":%d,\"lineage\":[%s],\"work_left\":%d,\"token\":\"%s\",\"reason\":\"%s\",\"cmd\":\"%s\"}\n",
		time.Now().UTC().Format(time.RFC3339), root, ppid, strings.Join(ps, ","), work,
		strings.ReplaceAll(tok, "\"", "'"), strings.ReplaceAll(reason, "\"", "'"), strings.ReplaceAll(cmd, "\"", "'"))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
