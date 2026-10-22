package agent

// childtree.go — 工具面起子进程的**统一收尾 ＋ 限额**（2026-10-23 拍板「落」）。
//
// 为什么三件事必须一起做：
//
//	① 每个子进程自成一组（`Setpgid`）：到点/取消时能一条信号打到**整棵**。只打直接子进程的
//	   先例有账 —— 命令面 `GAP-20260930-04`：`bash <脚本>` 被 `kill -9` 后，它 fork 的孙进程
//	   （门步骤 / `go test` / 子脚本）PPID 改嫁成 1，照跑。
//	② 收尾两段（GNU `timeout -k` 先例）：整组 `SIGTERM` → 宽限 `childKillGrace`（默认 2s）→
//	   整组 `SIGKILL`。`cmd.WaitDelay` 兜住另一半：子进程退了、孙进程还攥着 stdout 管道时，
//	   `cmd.Wait` 会在管道拷贝上悬挂 —— WaitDelay 到点强关管道并杀掉幸存者。
//	③ 限额（macOS 没有 cgroup）：以 `ulimit -u 64` / `ulimit -t 600` 前缀进子 shell。跑飞的一棵
//	   子进程树既不能扇出无限进程，也不能把一个核烧穿整段墙钟。
//
// 与命令面的关系：`cmd/zerg/main.go` 的 `armChildGroup`/`terminateChildTree` 是同语义的
// 第一份实现（本包不能 import main，故按同语义再收一份；两处探针/信号口径逐字对齐，
// 改一处请对齐另一处）。差异只有两点：本包**同时**挂限额前缀，且收尾多了 WaitDelay。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// 限额默认值（拍板数：64 进程 / 600 CPU 秒）。
const (
	DefaultChildProcLimit  = 64
	DefaultChildCPUSeconds = 600
)

var (
	// childProcLimit — 单棵子进程树的进程数上限（`ulimit -u`）。
	childProcLimit = DefaultChildProcLimit
	// childCPUSeconds — 单个进程的 CPU 时间上限（`ulimit -t`；POSIX 语义是**每进程**）。
	childCPUSeconds = DefaultChildCPUSeconds
	// childKillGrace — 两段收尾的宽限：TERM 之后等这么久，幸存者再吃 KILL。
	// 是变量：判据件要把它压短（否则每条超时用例都要真等满宽限）。
	childKillGrace = 2 * time.Second
)

// childProcLimitValue — 本次要钉的 `RLIMIT_NPROC` 值，以及它的基线（该 uid 当前进程数）。
//
// ★ `RLIMIT_NPROC` **按 uid 计**（Darwin 与 Linux 都是：sysctl/kernel 认的是 real uid 的进程总数，
// 不是「这棵子树的进程数」）。所以「上限 = 64」这种**绝对值**写法在真实机器上是毒药：
// 实测本机该 uid 常驻 399 个进程，`ulimit -u 64`（乃至 200）之后每一个 fork 都直接
// `Resource temporarily unavailable` —— 连 `ls` 都起不来，工具面全瞎。
//
// 于是取「本次额度」的语义：**基线 ＋ childProcLimit**，即这棵子树最多再扇出 childProcLimit 个
// 进程；同时不越过硬限（超了 ulimit 本身会失败，软限原地不动，等于没限额）。
func childProcLimitValue() (base, limit int) {
	base = countUserProcesses()
	if base < 1 {
		base = 1
	}
	// 不另读硬限：若算出来的值越过硬限，`ulimit -u` 自己会失败（前缀里 `|| :` 不吞命令），
	// 等价于「本机不允许再收」—— 那就照旧跑，不假装夹住了（报话里也照实写额度）。
	return base, base + childProcLimit
}

// countUserProcesses —— 该 uid 当下有多少个进程（读不到进程表 ⇒ 1，等价于「只加额度」）。
func countUserProcesses() int {
	out, err := exec.Command("ps", "-Ao", "uid=").Output()
	if err != nil {
		return 1
	}
	me := strconv.Itoa(os.Getuid())
	n := 0
	for _, f := range strings.Fields(string(out)) {
		if f == me {
			n++
		}
	}
	return n
}

// childLimitPrefix — 进子 shell 的第一段：把两枚限额钉上。
//
// 用 `ulimit` 而不是 cgroup：macOS 无 cgroup，工具面又要能在别人的机器上直接跑。
// `|| :` 让「本机不支持某项」不至于把命令本身带崩（拿不到限额 ≠ 命令该失败）。
// 进程数那一枚是**相对**的（基线 ＋ 额度），理由见 childProcLimitValue；
// CPU 那一枚是**绝对**的（`RLIMIT_CPU` 本来就是每进程）。
func childLimitPrefix() string {
	_, limit := childProcLimitValue()
	return fmt.Sprintf("ulimit -u %d 2>/dev/null || :; ulimit -t %d 2>/dev/null || :; ",
		limit, childCPUSeconds)
}

// boundedShell —— 「用 shell 跑一段脚本」的唯一入口：限额前缀 ＋ 自成一组 ＋ 两段收尾。
//
// script 原样拼在限额前缀之后（`;` 分隔，故 `a && b` / `cd x; y` 语义不变）。
func boundedShell(ctx context.Context, shell, script string) *exec.Cmd {
	return armChildGroup(exec.CommandContext(ctx, shell, "-c", childLimitPrefix()+script))
}

// boundedExec —— 「直接跑一个可执行件」的唯一入口（本机没有 shell 可言时用）。
//
// 不拼字符串、不做引号转义：`sh -c '<limits>; exec "$0" "$@"' <name> <args...>` —— `$0`/`$@`
// 由内核原样传进来的 argv 填，含空格/引号的参数也一个不差（等价于直接 exec）。
func boundedExec(ctx context.Context, name string, args ...string) *exec.Cmd {
	argv := make([]string, 0, len(args)+4)
	argv = append(argv, "-c", childLimitPrefix()+`exec "$0" "$@"`, name)
	argv = append(argv, args...)
	return armChildGroup(exec.CommandContext(ctx, "sh", argv...))
}

// armChildGroup 在**起子进程之前**把它自成一组（组长 = 它自己），并挂上两段收尾。
//
// 返回同一个 cmd 便于链式使用。`Setpgid` 是这套东西的地基：没有它，「收整棵」只能沿
// 当下的 ppid 树猜（已改嫁的漏网）。
func armChildGroup(cmd *exec.Cmd) *exec.Cmd {
	if cmd == nil {
		return cmd
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	// 默认 Cancel 只 Kill 直接子进程（`CommandContext` 的语义）—— 换成一整组 TERM，
	// 并在宽限后补一整组 KILL（goroutine 里做，Cancel 必须立刻返回，否则 Wait 卡住）。
	cmd.Cancel = func() error { return cancelChildGroup(cmd) }
	// WaitDelay 兜住「子进程退了、孙进程还开着 stdout/stderr 管道」：到点强关管道，
	// 让 `cmd.Wait` 一定能返回，绝不在管道拷贝上吊死。
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = childKillGrace + 500*time.Millisecond
	}
	return cmd
}

// cancelChildGroup —— ctx 到点时走的路：先整组 TERM，宽限后整组 KILL（异步）。
//
// 返回 os.ErrProcessDone 表示「组已经不在了」—— 按 `exec.Cmd.Cancel` 的契约，这不改写
// 子进程自身的退出码（否则一次干净收尾会被上层认成 Cancel 失败）。
func cancelChildGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return os.ErrProcessDone
	}
	pid := cmd.Process.Pid
	if err := signalChildTree(childTreeTargets(pid), syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	pgid := pid
	go func() {
		deadline := time.Now().Add(childKillGrace)
		for time.Now().Before(deadline) {
			if !childTreeAlive(pgid) {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		// 宽限过了还有活的 ⇒ 对**重算过**的靶清单发 KILL（宽限里可能又冒出新后代）。
		_ = signalChildTree(childTreeTargets(pgid), syscall.SIGKILL)
	}()
	return nil
}

// terminateChildTree 两段收尾的**同步**版（到点/取消之后由各落点补一刀，保证返回即收净）。
//
// 先 TERM 整棵 → 宽限内轮询 → 没走干净再 KILL 整棵。正常进程（含 `bash`/`sleep`）第一段
// 就死，轮询 20ms 即返回；只有硬扛信号的（或已变孤儿仍攥着资源的）才真等满宽限。
func terminateChildTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	pid := cmd.Process.Pid
	_ = signalChildTree(childTreeTargets(pid), syscall.SIGTERM)
	if childTreeGone(pid) {
		return
	}
	_ = signalChildTree(childTreeTargets(pid), syscall.SIGKILL)
}

// childTreeTargets —— 这一趟收尾要打的靶：**成组**（组长 = 自己）⇒ 只给 `-pgid`（一条信号
// 管全树，且不碰调用者与别人的组）；**未成组** ⇒ 本 pid ＋ 沿 `ppid` 树收全部后代。
func childTreeTargets(pid int) []int {
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid {
		return []int{-pid}
	}
	return descendantPids(pid)
}

// descendantPids —— 沿 `ps -Ao pid=,ppid=` 的 ppid 树从 root 收全部后代（含 root 自己）。
// 读不到进程表 ⇒ 只给 root（宁可弱一拍，也不误伤别人）。
// 只用于**没走 armChildGroup** 的少数落点：这种收法认当下的父子关系，已改嫁的漏网。
func descendantPids(root int) []int {
	out := []int{root}
	raw, err := exec.Command("ps", "-Ao", "pid=,ppid=").Output()
	if err != nil {
		return out
	}
	kids := map[int][]int{}
	for _, ln := range strings.Split(string(raw), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		p, e1 := strconv.Atoi(f[0])
		pp, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil {
			continue
		}
		kids[pp] = append(kids[pp], p)
	}
	seen := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, c := range kids[p] {
			if seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}

// signalChildTree 逐靶发一枚信号（负靶 = 整组；单靶的 `ESRCH` 当已不在，不当失败，
// 但**全部**靶都打空时把最后一个 ESRCH 返回给调用方，好让它认「组已不在」）。
func signalChildTree(targets []int, sig syscall.Signal) error {
	var lastErr error
	delivered := false
	for _, tgt := range targets {
		if err := syscall.Kill(tgt, sig); err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				return err
			}
			lastErr = err
			continue
		}
		delivered = true
	}
	if !delivered {
		return lastErr
	}
	return nil
}

// childTreeGone 宽限内轮询「整棵收干净了吗」；到点还没干净 ⇒ 假（交给第二段 KILL）。
func childTreeGone(pid int) bool {
	deadline := time.Now().Add(childKillGrace)
	for {
		if !childTreeAlive(pid) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// childTreeAlive —— 成组看整组还有没有活的；未成组看本树逐个。
// 注意：组长已死时 `Getpgid` 失败 ⇒ 退化成只认本 pid（探针自身对死组长失明），
// 判据件要另外用 `kill(-pgid, 0)` 看组在不在。
func childTreeAlive(pid int) bool {
	for _, tgt := range childTreeTargets(pid) {
		if syscall.Kill(tgt, 0) == nil {
			return true
		}
	}
	return false
}

// childExitSignal —— 这次失败若是被信号杀，取出那枚信号（否则 0）。供限额报话认 SIGXCPU。
func childExitSignal(err error) syscall.Signal {
	var ee *exec.ExitError
	if err == nil || !errors.As(err, &ee) || ee.ProcessState == nil {
		return 0
	}
	if ws, ok := ee.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal()
	}
	return 0
}

// ── 限额报话：把「真的超了限额」与「命令自己失败」分开 ─────────────────────────────

// childLimitDiagnosis —— 这次失败是不是限额打的？返回可分辨的报话（"" = 不是限额）。
//
// 两路判据都要，缺一不可：
//   - 进程数限额：fork 失败 ⇒ 内核 EAGAIN，落成 shell 的 "resource temporarily unavailable" /
//     "can't fork"（macOS bash 是前者）；退出码通常是 1 或 127，光看码分不出来。
//   - CPU 时间限额：`RLIMIT_CPU` 到点 ⇒ 内核先发 SIGXCPU（bash 报 "CPU time limit exceeded"，
//     shell 的 `$?` 是 128+24=152；但 Go 的 `ExitError.ExitCode()` 对**被信号杀**的进程一律返回 -1，
//     所以判据要认 `WaitStatus.Signal() == SIGXCPU`，不能只看码）。**不能**拿 SIGKILL 当判据：
//     超时收尾、被外部杀掉都是它。
func childLimitDiagnosis(exitCode int, sig syscall.Signal, stderrText string) string {
	low := strings.ToLower(stderrText)
	procHit := false
	for _, needle := range []string{
		"resource temporarily unavailable",
		"cannot allocate memory",
		"can't fork",
		"cannot fork",
		"unable to fork",
		"fork: retry",
	} {
		if strings.Contains(low, needle) {
			procHit = true
			break
		}
	}
	if procHit {
		base, limit := childProcLimitValue()
		return fmt.Sprintf("⛔ 触及进程数限额（RLIMIT_NPROC=%d = 本 uid 现有 %d ＋ 本次额度 %d，不是命令本身报错）：子进程 fork 被内核拒绝 —— 收敛并发/批量，或调 childProcLimit",
			limit, base, childProcLimit)
	}
	if sig == syscall.SIGXCPU || exitCode == 128+24 || strings.Contains(low, "cpu time limit exceeded") {
		return fmt.Sprintf("⛔ 触及 CPU 时间限额（ulimit -t %ds，不是命令本身报错）：进程 CPU 时间用尽被 SIGXCPU 终止 —— 缩小单次工作量/分批，或调 childCPUSeconds", childCPUSeconds)
	}
	return ""
}
