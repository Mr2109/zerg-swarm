// cli_childtree_test.go —— 缺口 `GAP-20260930-04` 的**聚焦判据**：到点 / 取消收**整棵**子进程树。
//
// 缺口逐字（`zerg gap show GAP-20260930-04`）：墙钟到点（`--timeout` / Ctrl-C
// `--cancel-on-interrupt`）只杀**直接**子进程、漏孙进程 ⇒ 命令面起的 bash 脚本再 fork 的门步骤 /
// `go test` 变孤儿继续跑。
//
// 为什么现有判据销不掉这账：`cli_wallclock_test.go` 的 `TestWallclockDeadlineStopsOwnChild`
// 只起**直接**子进程（一枚 `sleep`）· **不起孙进程** ⇒ 「收整棵」那一半无可读数。
//
// 本件补两半（**成对** · 负控有牙，不许写成永远绿）：
//
//	① 正控 `TestTerminateChildTreeTimeoutReapsGrandchild`：真起 P→C→G 一棵树（父子孙各 `sleep`），
//	   走生产路径 `wallclockGuard → cancelRunningChild → terminateChildTree`；到点后断言
//	   **子与孙都不再存活**（两路判：进程是否在表 ＋ 整组是否还在）。
//	② 负控 `TestTerminateChildTreeNegativeControlOldKillLeavesResidue`：同一棵树、但按**旧行为**
//	   只打**直接子进程**（不按组收）⇒ 断言**残留能被检出**（C/G 仍活 · `childTreeAlive` 报活）；
//	   再一条 `TestTerminateChildTreeNegativeControlUngroupedBlindSpot` 钉住「**不起组**」时
//	   改嫁的孙进程如何漏网（旧探针 `descendantPids` 对已重认养者失明）—— 两条都断言残留存在，
//	   绝不写成恒绿：负控若「残留被收干净」反而要**判红**（说明负控没生效/被验件被误改）。
//
// 反假绿：每一条正控在收尾**之前**先断言「C/G 真的起来了且活着」——否则「都没了」可能是「没起来」。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ── 两路探针（判「在不在」）──────────────────────────────────────────────────
//
// ① 进程是否存在：读 `ps` 的 state（空 = 不在表；`Z` = 僵尸、等父收尸 ⇒ 也算不在）。
// ② 整组是否还在：对**进程组**发信号 0（`ESRCH` = 整组已散）。
// ③ ppid：原父子关系是否已被重认养（真实 pid > 1 ⇒ 关系断了/改了嫁）。
func pidState(pid int) string {
	out, err := exec.Command("ps", "-o", "state=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func pidLive(pid int) bool {
	st := pidState(pid)
	return st != "" && !strings.HasPrefix(st, "Z")
}

func groupLive(pgid int) bool { return syscall.Kill(-pgid, 0) == nil }

func ppidOf(pid int) int {
	out, err := exec.Command("ps", "-Ao", "pid=,ppid=").Output()
	if err != nil {
		return -1
	}
	for _, ln := range strings.Split(string(out), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil {
			continue
		}
		if p == pid {
			return pp
		}
	}
	return 0
}

func waitPidFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			if pid, e := strconv.Atoi(strings.TrimSpace(string(b))); e == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等 pid 文件落定超时：%s", path)
	return 0
}

func waitNotLive(t *testing.T, pid int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !pidLive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeChildTreeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("造脚本失败 %s：%v", path, err)
	}
}

// startChildTree 起一棵**真·三代**树：P（本件起的 bash）→ C（P fork 的 bash）→ G（C fork 的 `sleep`）。
//
//	C 的 pid 由 P 用 `$!` 落到 child.pid；G 的 pid 由 C 用 `$!` 落到 grand.pid。
//	armed=true 走 `armChildGroup`（被验形态）；armed=false 不起组（旧行为对照）。
//	★ 两枚 pid 文件都落定才返回 —— 否则「都没了」可能是「还没起来」= 假绿。
func startChildTree(t *testing.T, dir string, armed bool) (cmd *exec.Cmd, childPid, grandPid int) {
	t.Helper()
	childPidFile := filepath.Join(dir, "child.pid")
	grandPidFile := filepath.Join(dir, "grand.pid")
	inner := filepath.Join(dir, "inner.sh")
	outer := filepath.Join(dir, "outer.sh")
	// inner = C：起后台 `sleep`=G 并把 G 的 pid 落盘，然后 `wait`（C 保持存活）。
	writeChildTreeScript(t, inner, "sleep 40 &\necho $! > \"$ZERG_GRAND_PID_F\"\nwait\n")
	// outer = P：起后台 bash=inner(C)，把 C 的 pid 落盘，然后 `wait`（P 保持存活）。
	//   ★ 用 `wait`（内建）而非 `sleep`：树里**只有** P/C/G 三个进程，不泄漏第 4 个无人认领的子件
	//   （成组收尾不成问题，但「不成组」的负控只按 pid 点名收 C/G ⇒ 多一个就收不净）。
	writeChildTreeScript(t, outer, "bash \"$ZERG_INNER\" &\necho $! > \"$ZERG_CHILD_PID_F\"\nwait\n")

	cmd = exec.Command("bash", outer)
	cmd.Env = append(os.Environ(),
		"ZERG_INNER="+inner,
		"ZERG_CHILD_PID_F="+childPidFile,
		"ZERG_GRAND_PID_F="+grandPidFile,
	)
	if armed {
		armChildGroup(cmd)
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("起不了 bash 靶树（本机形态不同）：%v", err)
	}
	childPid = waitPidFile(t, childPidFile)
	grandPid = waitPidFile(t, grandPidFile)
	return cmd, childPid, grandPid
}

// ── ① 正控：到点走生产路径 ⇒ 整棵树（含孙）收干净 ────────────────────────────

func TestTerminateChildTreeTimeoutReapsGrandchild(t *testing.T) {
	// 压短到点后的宽限（否则每条超时用例真等满 2s）；用完复原。
	saved := childKillGrace
	childKillGrace = 500 * time.Millisecond
	defer func() { childKillGrace = saved }()

	dir := t.TempDir()
	cmd, childPid, grandPid := startChildTree(t, dir, true)
	leader := cmd.Process.Pid
	defer func() { runningChild = nil }()

	// ★ 反假绿：收尾**之前**先断言「子与孙真的起来了且活着」。
	if !pidLive(childPid) || !pidLive(grandPid) {
		t.Fatalf("靶树没起来（C=%d live=%v · G=%d live=%v）⇒ 本用例无效",
			childPid, pidLive(childPid), grandPid, pidLive(grandPid))
	}
	// 成组落点核实：C 在 P 的进程组里（armChildGroup 真的生效）。
	pg, err := syscall.Getpgid(childPid)
	if err != nil || pg != leader {
		t.Fatalf("C(pid=%d) 不在 P(pid=%d) 的组（pgid=%d err=%v）⇒ 成组落点没生效", childPid, leader, pg, err)
	}
	t.Logf("【新行为·靶树】P=%d C=%d G=%d · C.group=%d（=P ⇒ 成组生效）· 收尾前 C/G 都活", leader, childPid, grandPid, pg)

	// 走生产路径：到点 ⇒ cancelRunningChild → terminateChildTree（TERM→宽限→KILL）。
	runningChild = cmd
	blocked := blockForever()
	var stderr bytes.Buffer
	rc := wallclockGuard(300*time.Millisecond, func() int { <-blocked; return exitOK }, &stderr)
	if rc != exitTimeout {
		t.Fatalf("到点该退 %d（超时），拿到 %d", exitTimeout, rc)
	}
	_ = cmd.Wait()

	// 两路判：① 进程是否在表 ② 整组是否还在。
	waitNotLive(t, childPid, 3*time.Second)
	waitNotLive(t, grandPid, 3*time.Second)
	if pidLive(childPid) {
		t.Errorf("**子进程残留**：超时后 C(pid=%d) 还活着（state=%q）", childPid, pidState(childPid))
	}
	if pidLive(grandPid) {
		t.Errorf("**孙进程残留（缺口本身）**：超时后 G(pid=%d) 还活着（state=%q · ppid=%d）",
			grandPid, pidState(grandPid), ppidOf(grandPid))
	}
	if groupLive(leader) {
		t.Errorf("整组没收净：groupLive(%d)=true（kill -0 -pgid 仍打得到）", leader)
	}
	if childTreeAlive(leader) {
		t.Errorf("整棵子进程树没收净：childTreeAlive(%d)=true", leader)
	}
	if !strings.Contains(stderr.String(), "先复原再报") {
		t.Errorf("到点那一格措辞该点明「先复原再报」，实到：%s", stderr.String())
	}
	t.Logf("【新行为·收尾后】C=%d live=%v · G=%d live=%v · groupLive(P=%d)=%v · childTreeAlive(P)=%v ⇒ 整树收净",
		childPid, pidLive(childPid), grandPid, pidLive(grandPid), leader, groupLive(leader), childTreeAlive(leader))
}

// ── ② 负控甲：旧行为（只打直接子进程）⇒ 残留**能被检出** ─────────────────────

func TestTerminateChildTreeNegativeControlOldKillLeavesResidue(t *testing.T) {
	saved := childKillGrace
	childKillGrace = 500 * time.Millisecond
	defer func() { childKillGrace = saved }()

	dir := t.TempDir()
	cmd, childPid, grandPid := startChildTree(t, dir, true)
	leader := cmd.Process.Pid

	if !pidLive(childPid) || !pidLive(grandPid) {
		t.Fatalf("靶树没起来 ⇒ 负控无效（C=%d live=%v · G=%d live=%v）",
			childPid, pidLive(childPid), grandPid, pidLive(grandPid))
	}

	// 旧行为：**只**打直接子进程（P），不按组收、不两段。
	// 这个形态就是缺口正文说的「只杀直接子进程」⇒ 孙进程（及其父 C）该留下来。
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatalf("打不到直接子进程 %d：%v", leader, err)
	}
	_ = cmd.Wait()

	// ★ 负控的断言是「**残留存在且可检**」——这是本负控的靶心。
	waitNotLive(t, leader, 2*time.Second)
	if pidLive(childPid) != true {
		t.Errorf("负控无效：旧行为只打直接子进程后，C(pid=%d) 竟已不在 ⇒ 负控没生效（不构成反例）", childPid)
	}
	if pidLive(grandPid) != true {
		t.Errorf("负控无效：旧行为后孙进程 G(pid=%d) 竟已不在 ⇒ 负控没生效（不构成反例）", grandPid)
	}
	// 重认养：P 死后 C 应挂到 pid 1（真实父关系已被改写 ⇒ 旧探针沿 ppid 树会漏网）。
	if pp := ppidOf(childPid); pp != 1 {
		t.Errorf("旧行为后 C(pid=%d) 的 ppid=%d（期望 1 = 被重认养）—— 残留形态与缺口正文不符", childPid, pp)
	}
	// 生产探针在旧行为下应报「整组还在」（= 正控那条「整组收净」断言有牙：旧行为下必红）。
	//   ★ 只用**组存在性** `kill(-pgid,0)`：P 一死，`childTreeTargets` 里的 `Getpgid(P)` 也失败
	//   ⇒ `childTreeAlive` 会退化成「只认已死的 P」而误报 false（探针自身对死组长失明）——
	//   所以这一格认的是组还在不在，不拿 childTreeAlive 当旧行为的残留判据。
	if !groupLive(leader) {
		t.Errorf("负控无效：旧行为后 groupLive(%d)=false —— 那正控的断言无从有牙", leader)
	}

	t.Logf("【旧行为·只打直接子进程】P=%d 已死 · C=%d live=%v · G=%d live=%v · ppidOf(C)=%d(=1 重认养) · groupLive(P)=%v ⇒ 残留可检",
		leader, childPid, pidLive(childPid), grandPid, pidLive(grandPid), ppidOf(childPid), groupLive(leader))

	// 清自己的靶子（只有本用例起的这一棵）：直接对**组**发 KILL（组长已死，按组收）。
	if err := syscall.Kill(-leader, syscall.SIGKILL); err != nil {
		t.Fatalf("负控自清：对组 -%d 发 KILL 失败：%v", leader, err)
	}
	waitNotLive(t, childPid, 3*time.Second)
	waitNotLive(t, grandPid, 3*time.Second)
	if pidLive(childPid) || pidLive(grandPid) {
		t.Fatalf("负控自清失败：C=%v · G=%v 仍在（本用例靶子必须收干净）",
			pidLive(childPid), pidLive(grandPid))
	}
}

// ── ② 负控乙：**不起组**时，改嫁的孙进程对旧探针失明（钉住「为什么必须成组」）───

func TestTerminateChildTreeNegativeControlUngroupedBlindSpot(t *testing.T) {
	dir := t.TempDir()
	cmd, childPid, grandPid := startChildTree(t, dir, false) // ← 不起组
	leader := cmd.Process.Pid

	if !pidLive(childPid) || !pidLive(grandPid) {
		t.Fatalf("靶树没起来 ⇒ 负控无效（C=%d live=%v · G=%d live=%v）",
			childPid, pidLive(childPid), grandPid, pidLive(grandPid))
	}
	// 不起组：P 与测试进程同组 ⇒ childTreeTargets 走 descendantPids（认当下父子关系）。
	if pg, err := syscall.Getpgid(leader); err == nil && pg == leader {
		t.Fatalf("用例无效：未成组的 P(pid=%d) 竟是组长（pgid=%d）", leader, pg)
	}

	// 旧行为：只打直接子进程 ⇒ C 与 G 变孤儿、重认养。
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatalf("打不到直接子进程 %d：%v", leader, err)
	}
	_ = cmd.Wait()
	waitNotLive(t, leader, 2*time.Second)

	// 残留确实存在（两路：进程在表 + 重认养）。
	if !pidLive(grandPid) {
		t.Errorf("负控无效：不起组 + 只打直接子进程后，孙进程 G(pid=%d) 竟已不在", grandPid)
	}
	if pp := ppidOf(childPid); pp != 1 {
		t.Errorf("旧行为后 C(pid=%d) 的 ppid=%d（期望 1 = 被重认养）", childPid, pp)
	}
	// ★ 靶心：C 已改嫁 ⇒ 旧探针（沿 ppid 树认当下关系）对残留**失明**。
	//   这正是缺口正文「只杀直接子进程、漏孙进程」的机理 —— 也是必须成组的理由。
	if childTreeAlive(leader) {
		t.Errorf("负控乙的前提不成立：不起组时旧探针 childTreeAlive(%d)=true（期望失明 false）", leader)
	}

	t.Logf("【旧行为·不成组】P=%d 已死 · C=%d live=%v ppid=%d · G=%d live=%v · childTreeAlive(P)=%v（旧探针失明）⇒ 残留可检",
		leader, childPid, pidLive(childPid), ppidOf(childPid), grandPid, pidLive(grandPid), childTreeAlive(leader))

	// 清自己的靶子（无组可收 ⇒ 逐 pid 点名，先孙后子）。
	_ = syscall.Kill(grandPid, syscall.SIGKILL)
	_ = syscall.Kill(childPid, syscall.SIGKILL)
	waitNotLive(t, grandPid, 3*time.Second)
	waitNotLive(t, childPid, 3*time.Second)
	if pidLive(grandPid) || pidLive(childPid) {
		t.Fatalf("负控乙自清失败：C=%v · G=%v 仍在", pidLive(childPid), pidLive(grandPid))
	}
}
