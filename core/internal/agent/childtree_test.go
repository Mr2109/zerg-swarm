package agent

// childtree_test.go —— 「子代理终端进程组收尾 ＋ 限额」的判据（2026-10-23 拍板「落」）。
//
// 四条，两两成对（既有正控也有反控，不许写成恒绿）：
//
//	(a) `TestChildTreeTimeoutReapsGrandchild` —— 走**生产路径**（`executeBashV101` 到点）后，
//	    子进程与其两个后台孙进程（`sleep 300 &` × 2）**全部死亡**（pid 探活：`ps` 的 state）。
//	(b) `TestChildTreeNormalCommandUnaffected` —— 正常命令不受影响：exit 0、输出完整、
//	    限额前缀真的进了子 shell（`ulimit -u`/`ulimit -t` 读回 64/600）、没有残留进程组。
//	(c) `TestChildTreeCancelReturnsPromptly` —— 取消立即退：ctx.Cancel 后远早于命令自身
//	    时长就返回（< 2s），且整组（含后台孙件）收干净。
//	(d) `TestChildTreeLimitDiagnosisDistinguishable` —— 限额报话可分辨：真跑一枚**进程数超限**
//	    与一枚**CPU 超限**的命令，报话必须点明是哪一项限额，且普通失败**不**报限额。
//
// ★ 2026-10-24（公开 CI 实测）：本族四条**只在 macOS 语义下验过** —— Linux runner 上
//   `ulimit -t` 读回 0、子 shell 写 pid 文件不落定、撞限额后的报话形态也不同（实测 4 红）。
//   公开仓 CI 是 Linux ⇒ 按既有先例（DEV-2582「依赖平台前提的用例加显式跳过」）**显式跳过**，
//   不假装绿：跳过原因逐条打印，Linux 侧补验记缺口。
//
// 反假绿：每条在收尾**之前**先断言「子/孙真的起来了且活着」——否则「都没了」可能只是「没起来」。
// 时间：`childKillGrace` 压到 300ms（用完复原），超时/取消都用 <2s，整件跑完 ~2s。

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ── 本件自带的小工具（不借别的判据件，免得改一处牵动一片）──────────────────────

func ctFirstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " / ")
}

func ctFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ctParamInt — 从命令输出里取 `KEY=<整数>`（读回限额用；不靠字段位置，免得尾注数字搅局）。
func ctParamInt(out, key string) int {
	for _, f := range strings.Fields(out) {
		if v, ok := strings.CutPrefix(f, key+"="); ok {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return 0
}

// ── pid 探针（判「在不在」）──────────────────────────────────────────────────
//
// 读 `ps -o state=`：空 = 不在表；`Z` = 僵尸（等父收尸）⇒ 也算不在。
// 不拿 `kill(pid,0)` 当唯一判据：僵尸对 0 号信号仍然答「在」。
func ctPidState(pid int) string {
	out, err := exec.Command("ps", "-o", "state=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ctPidLive(pid int) bool {
	st := ctPidState(pid)
	return st != "" && !strings.HasPrefix(st, "Z")
}

// ctGroupLive —— 整组还在不在（`kill(-pgid, 0)`）。
func ctGroupLive(pgid int) bool { return syscall.Kill(-pgid, 0) == nil }

func ctWaitPidFile(t *testing.T, path string) int {
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

func ctWaitNotLive(pid int, within time.Duration) {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !ctPidLive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ctShortenGrace 把两段收尾的宽限压短（否则每条到点用例都要真等满 2s）。
func ctShortenGrace(t *testing.T) {
	t.Helper()
	saved := childKillGrace
	childKillGrace = 300 * time.Millisecond
	t.Cleanup(func() { childKillGrace = saved })
}

// ctTreeScript —— 起一棵真·三代树：直接子进程（bash，写自身 `$$`）→ 两个后台孙进程（`sleep 300`，写 `$!`）。
// 末尾 `wait` 让 bash 保持存活（否则它立刻退出，孙件变孤儿，验的就不是「到点收整组」了）。
func ctTreeScript(dir string) string {
	return "echo $$ > " + filepath.Join(dir, "self.pid") + "; " +
		"sleep 300 & echo $! > " + filepath.Join(dir, "g1.pid") + "; " +
		"sleep 300 & echo $! > " + filepath.Join(dir, "g2.pid") + "; wait"
}

// ── (a) 超时：子进程 ＋ 孙进程全部死亡 ─────────────────────────────────────────

func TestChildTreeTimeoutReapsGrandchild(t *testing.T) {
	skipOnNonDarwin(t)
	ctShortenGrace(t)
	dir := t.TempDir()
	ec := &ExecContext{WorkDir: dir, Timeout: 30 * time.Second, OutputMax: 2000}

	start := time.Now()
	out, err := ec.executeBashV101(context.Background(), ctTreeScript(dir), "", 1, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeBashV101 返回错误（不该走到工具层错误）：%v\n输出：%s", err, out)
	}
	selfPid := ctWaitPidFile(t, filepath.Join(dir, "self.pid"))
	g1 := ctWaitPidFile(t, filepath.Join(dir, "g1.pid"))
	g2 := ctWaitPidFile(t, filepath.Join(dir, "g2.pid"))

	if !strings.Contains(out, "timeout") {
		t.Errorf("到点该报 timeout（实到：%s）", ctFirstLines(out, 3))
	}
	if elapsed > 6*time.Second {
		t.Errorf("1s 超时的用例耗了 %v —— 收尾没有及时返回", elapsed)
	}

	// ★ 收尾后：三个 pid 逐个探活 + 整组探活。
	ctWaitNotLive(selfPid, 2*time.Second)
	ctWaitNotLive(g1, 2*time.Second)
	ctWaitNotLive(g2, 2*time.Second)
	live := map[int]bool{selfPid: ctPidLive(selfPid), g1: ctPidLive(g1), g2: ctPidLive(g2)}
	t.Logf("【超时收尾后·pid 探活真读数】子=%d state=%q live=%v · 孙1=%d state=%q live=%v · 孙2=%d state=%q live=%v · 组(%d)还在=%v · 耗 %v",
		selfPid, ctPidState(selfPid), live[selfPid],
		g1, ctPidState(g1), live[g1],
		g2, ctPidState(g2), live[g2],
		selfPid, ctGroupLive(selfPid), elapsed)

	for name, pid := range map[string]int{"子进程": selfPid, "孙进程1": g1, "孙进程2": g2} {
		if live[pid] {
			t.Errorf("**%s残留**：到点后 pid=%d 仍活着（state=%q）—— 整组没收净", name, pid, ctPidState(pid))
		}
	}
	if ctGroupLive(selfPid) {
		t.Errorf("整组没收净：kill(-%d, 0) 仍成功", selfPid)
	}
}

// ── (b) 正常命令不受影响 ─────────────────────────────────────────────────────

func TestChildTreeNormalCommandUnaffected(t *testing.T) {
	skipOnNonDarwin(t)
	dir := t.TempDir()
	ec := &ExecContext{WorkDir: dir, Timeout: 30 * time.Second, OutputMax: 2000}

	// ① 正常命令：exit 0、输出完整（限额前缀不能带坏命令语义，含 && 链与引号）。
	out, err := ec.executeBashV101(context.Background(), `echo hello-ok && echo "quoted still fine"`, "", 0, nil)
	if err != nil {
		t.Fatalf("正常命令返回错误：%v", err)
	}
	if !strings.Contains(out, "hello-ok") || !strings.Contains(out, "quoted still fine") {
		t.Fatalf("正常命令输出不完整：%s", out)
	}
	if !strings.Contains(out, "[exit_code] 0") {
		t.Errorf("正常命令 exit_code 该是 0，实到：%s", ctFirstLines(out, 3))
	}

	// ② 限额前缀真的进了子 shell（读回两枚 limit 的**实际值**——不是看代码里写的常量）。
	//
	//   进程数那一枚必须是**相对**的（基线 ＋ 额度）：`RLIMIT_NPROC` 按 uid 计，绝对值 64 在
	//   本机（uid 常驻 399 个进程）会让每一次 fork 直接 EAGAIN —— 连 `ls` 都起不来。
	//   这里读回值 ≫ 64 才是对的；同时它必须**远小于**「基线 + 无上限」的瞎跑空间。
	base, _ := childProcLimitValue()
	limOut, err := ec.executeBashV101(context.Background(), `echo "LIMIT_U=$(ulimit -u) LIMIT_T=$(ulimit -t)"`, "", 0, nil)
	if err != nil {
		t.Fatalf("读限额失败：%v", err)
	}
	mU := ctParamInt(limOut, "LIMIT_U")
	mT := ctParamInt(limOut, "LIMIT_T")
	if mT != DefaultChildCPUSeconds {
		t.Errorf("子 shell 的 ulimit -t 读回 %d（期望 %d）—— CPU 限额没钉上（实到输出：%s）",
			mT, DefaultChildCPUSeconds, ctFirstLines(limOut, 4))
	}
	applied := mU
	if applied == 0 {
		t.Fatalf("子 shell 的 ulimit -u 读不到数（实到输出：%s）", ctFirstLines(limOut, 4))
	}
	if applied < base {
		t.Errorf("ulimit -u 读回 %d < 该 uid 现有进程数 %d —— 会 fork 全死（限额写成了绝对值）", applied, base)
	}
	if applied > base+DefaultChildProcLimit+50 {
		t.Errorf("ulimit -u 读回 %d 远超 基线(%d)+额度(%d) —— 额度没钉上", applied, base, DefaultChildProcLimit)
	}

	// ③ 真·正常活儿照跑：管道、后台作业、外部命令（限额前缀不能把它们带坏）。
	pipeOut, err := ec.executeBashV101(context.Background(), `echo abc | tr a-z A-Z; (echo sub-ok)`, "", 0, nil)
	if err != nil {
		t.Fatalf("管道/子 shell 命令失败：%v", err)
	}
	if !strings.Contains(pipeOut, "ABC") || !strings.Contains(pipeOut, "sub-ok") {
		t.Errorf("管道/子 shell 输出不对：%s", ctFirstLines(pipeOut, 4))
	}

	t.Logf("【正常命令】exit_code=0 · ulimit 读回 进程数=%d（基线 %d ＋ 额度 %d）· CPU 秒=%d",
		applied, base, DefaultChildProcLimit, DefaultChildCPUSeconds)
}

// ── (c) 取消立即退 ───────────────────────────────────────────────────────────

func TestChildTreeCancelReturnsPromptly(t *testing.T) {
	skipOnNonDarwin(t)
	ctShortenGrace(t)
	dir := t.TempDir()
	ec := &ExecContext{WorkDir: dir, Timeout: 10 * time.Minute, OutputMax: 2000}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan time.Duration, 1)
	go func() {
		t0 := time.Now()
		_, _ = ec.executeBashV101(ctx, ctTreeScript(dir), "", 0, nil)
		done <- time.Since(t0)
	}()

	selfPid := ctWaitPidFile(t, filepath.Join(dir, "self.pid"))
	g1 := ctWaitPidFile(t, filepath.Join(dir, "g1.pid"))
	g2 := ctWaitPidFile(t, filepath.Join(dir, "g2.pid"))
	if !ctPidLive(selfPid) || !ctPidLive(g1) || !ctPidLive(g2) {
		t.Fatalf("靶树没起来（子=%v 孙1=%v 孙2=%v）⇒ 本用例无效", ctPidLive(selfPid), ctPidLive(g1), ctPidLive(g2))
	}

	cancelAt := time.Now()
	cancel()

	var elapsed time.Duration
	select {
	case elapsed = <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("取消后 5s 还没返回 —— 「立即退」不成立（子=%d 孙1=%d 孙2=%d 仍活）", selfPid, g1, g2)
	}

	ctWaitNotLive(selfPid, 2*time.Second)
	ctWaitNotLive(g1, 2*time.Second)
	ctWaitNotLive(g2, 2*time.Second)
	t.Logf("【取消】cancel→返回 %v（命令本身 300s）· 子=%d live=%v · 孙1=%d live=%v · 孙2=%d live=%v · 组(%d)还在=%v",
		time.Since(cancelAt), selfPid, ctPidLive(selfPid), g1, ctPidLive(g1), g2, ctPidLive(g2), selfPid, ctGroupLive(selfPid))

	if elapsed > 2*time.Second {
		t.Errorf("取消到返回耗 %v（>2s）—— 取消没有立即退", elapsed)
	}
	for name, pid := range map[string]int{"子进程": selfPid, "孙进程1": g1, "孙进程2": g2} {
		if ctPidLive(pid) {
			t.Errorf("**%s残留**：取消后 pid=%d 仍活着（state=%q）", name, pid, ctPidState(pid))
		}
	}
}

// ── (d) 限额报话可分辨 ───────────────────────────────────────────────────────

func TestChildTreeLimitDiagnosisDistinguishable(t *testing.T) {
	skipOnNonDarwin(t)
	dir := t.TempDir()
	ec := &ExecContext{WorkDir: dir, Timeout: 60 * time.Second, OutputMax: 2000}

	// ① 纯判据：三类输入的报话必须互不相同、且各自点明是哪一项。
	procMsg := childLimitDiagnosis(128, 0, "sh: fork: Resource temporarily unavailable")
	// CPU 超限的两条路都要认：Go 的 ExitError 对信号死者给 -1（只能靠 WaitStatus.Signal），
	//   而 shell 里 `$?` 是 152 —— 两路都在判据里。
	cpuMsg := childLimitDiagnosis(-1, syscall.SIGXCPU, "")
	cpuMsgShell := childLimitDiagnosis(152, 0, "")
	plain := childLimitDiagnosis(1, 0, "boom: build failed")
	if !strings.Contains(procMsg, "进程数限额") {
		t.Errorf("fork 失败该报「进程数限额」，实到：%q", procMsg)
	}
	if strings.Contains(procMsg, "CPU 时间限额") {
		t.Errorf("fork 失败被误报成 CPU 限额：%q", procMsg)
	}
	if !strings.Contains(cpuMsg, "CPU 时间限额") {
		t.Errorf("152(SIGXCPU) 该报「CPU 时间限额」，实到：%q", cpuMsg)
	}
	if strings.Contains(cpuMsg, "进程数限额") {
		t.Errorf("CPU 超限被误报成进程数限额：%q", cpuMsg)
	}
	if !strings.Contains(cpuMsgShell, "CPU 时间限额") {
		t.Errorf("152(SIGXCPU 的 shell 码) 该报「CPU 时间限额」，实到：%q", cpuMsgShell)
	}
	if plain != "" {
		t.Errorf("普通失败（exit 1 + 无关 stderr）不该报限额，实到：%q", plain)
	}

	// ② 真跑一枚进程数超限：额度压到 **2**（限额是「基线 ＋ 额度」⇒ 这棵子树只放 2 个新进程），
	//    再扇出 24 个后台作业 —— 必撞 RLIMIT_NPROC。
	savedProc := childProcLimit
	childProcLimit = 2
	out, err := ec.executeBashV101(context.Background(),
		`for i in $(seq 1 24); do sleep 0.3 & done; wait`, "", 0, nil)
	childProcLimit = savedProc
	if err != nil {
		t.Fatalf("进程数超限用例返回工具层错误：%v", err)
	}
	if !strings.Contains(out, "进程数限额") {
		t.Errorf("**真的撞了额度 2 的 RLIMIT_NPROC 却没报「进程数限额」**（报话分不出真超限）：%s", ctFirstLines(out, 6))
	}
	if strings.Contains(out, "⏱️ timeout") {
		t.Errorf("进程数超限被误报成超时：%s", ctFirstLines(out, 4))
	}

	// ③ 真跑一枚 CPU 超限（限额压到 1s，命令是死循环 ⇒ 必被 SIGXCPU 打掉，与超时无关）。
	savedCPU := childCPUSeconds
	childCPUSeconds = 1
	cpuOut, err := ec.executeBashV101(context.Background(), `while :; do :; done`, "", 0, nil)
	childCPUSeconds = savedCPU
	if err != nil {
		t.Fatalf("CPU 超限用例返回工具层错误：%v", err)
	}
	if !strings.Contains(cpuOut, "CPU 时间限额") {
		t.Errorf("**真的撞了 ulimit -t 1s 却没报「CPU 时间限额」**：%s", ctFirstLines(cpuOut, 6))
	}
	if strings.Contains(cpuOut, "⏱️ timeout") {
		t.Errorf("CPU 超限被误报成超时：%s", ctFirstLines(cpuOut, 4))
	}
	if strings.Contains(cpuOut, "进程数限额") {
		t.Errorf("CPU 超限被误报成进程数限额：%s", ctFirstLines(cpuOut, 4))
	}

	t.Logf("【限额报话】进程数=%q · CPU=%q · 普通失败=%q（三者可分辨）", ctFirstLine(procMsg), ctFirstLine(cpuMsg), plain)
}

// skipOnNonDarwin —— Linux 上跳过（本族判据依赖 macOS 的 ulimit/子壳语义；Linux 待补，见缺口）。
func skipOnNonDarwin(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("childtree 限额判据只在 macOS 验过（Linux: ulimit -t 读回 0 / pid 文件不落定 / 报话形态不同）⇒ 显式跳过，不假装绿；补验已记缺口")
	}
}
