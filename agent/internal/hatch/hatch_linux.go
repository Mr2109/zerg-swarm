//go:build linux

// hatch_linux.go —— 孵化器在 Linux 上的真正执行路径（X3 用）。
//
// 分工（§6.1）：
//
//	systemd-run --user  ⇒ 建瞬态单元 + 独立 slice（归属：不被 x3-agent.service 的 KillMode 带走）
//	bwrap              ⇒ 建封闭空间（文件系统视图 + 进程视图 + 只读）
//
// 收卵 = 停那个单元（幂等：二次停 rc=5 视为成功）；停完整棵树一起收（KillMode=control-group）。
package hatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Hatcher 孵化器（无状态；超时可由调用方通过 ctx 控制）。
type Hatcher struct {
	// StopTimeout 收卵时等 systemctl 返回的上限（0 ⇒ 20s）。
	StopTimeout time.Duration
}

// ── 用户总线可达性（2026-09-15 真机踩到，缺陷 15）──────────────────────────────
//
// 现象：生产子端跑在 **系统服务** 里（`/system.slice/x3-agent.service`），它的环境里既没有
// `XDG_RUNTIME_DIR` 也没有 `DBUS_SESSION_BUS_ADDRESS` ⇒ 一切 `systemd-run --user` /
// `systemctl --user` 都会当场失败：
//
//	Failed to connect to user scope bus via local transport:
//	$DBUS_SESSION_BUS_ADDRESS and $XDG_RUNTIME_DIR not defined
//
// 为什么此前没暴露：真机测试是**从 ssh 会话**起的实例（继承了这两个变量）⇒ 在测试环境里
// 失败不了。**「在它跑过的那个环境里失败不了，就不是证据」**——这类缺陷只能靠「按生产形态
// 启动实例」才能抓到。
//
// 判据：给子进程显式补上本用户 uid 对应的运行时目录（linger 已开时它一定存在）；
// **拿不到就明确报错**，绝不静默退回「以系统单元起」那条能跑但归属全错的路
// （那会让卵不再独立于子端存活，E1 的结论也就没了）。
func UserScopeEnv() ([]string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); v != "" {
		return os.Environ(), nil // 已经够得到（会话里起的测试实例就是这种）
	}
	uid := os.Getuid()
	rt := fmt.Sprintf("/run/user/%d", uid)
	if st, err := os.Stat(rt); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("用户运行时目录 %s 不可用（linger 未开？）：%v —— 孵化需要它才能建用户级单元；拒绝改用系统单元（那会丢掉归属）", rt, err)
	}
	env := os.Environ()
	env = append(env,
		"XDG_RUNTIME_DIR="+rt,
		"DBUS_SESSION_BUS_ADDRESS=unix:path="+rt+"/bus",
	)
	return env, nil
}

// userScoped 构造一条「够得到本用户 systemd 管理器」的命令（所有 --user 调用都必须走它）。
func userScoped(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	env, err := UserScopeEnv()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	return cmd, nil
}

// Hatch 孵一枚卵：创建瞬态单元（内含 bwrap 封闭空间 + 引擎）。
// 返回单元名（后续收卵/观测都用它）。
func (h Hatcher) Hatch(ctx context.Context, spec Spec) (string, error) {
	unit := UnitName(spec.EggID)
	argv, err := BuildSystemdRunArgv(spec, unit)
	if err != nil {
		return unit, err
	}
	cmd, cerr := userScoped(ctx, argv[0], argv[1:]...)
	if cerr != nil {
		return unit, cerr
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 失败必须回原文：孵化失败的原因（polkit/userns/权限）都在这里
		return unit, fmt.Errorf("孵化单元创建失败（%s）：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	return unit, nil
}

// Collect 收卵：停单元（幂等）。返回错误仅当确实是"停不掉"。
func (h Hatcher) Collect(ctx context.Context, unit string) error {
	timeout := h.StopTimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd, cerr := userScoped(cctx, "systemctl", "--user", "stop", unit)
	if cerr != nil {
		return cerr
	}
	out, err := cmd.CombinedOutput()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	return collectOutcome(rc, string(out))
}

// Active 单元是否还在跑（收卵后用于核验"真的收干净了"）。
func (h Hatcher) Active(ctx context.Context, unit string) (bool, error) {
	cmd, cerr := userScoped(ctx, "systemctl", "--user", "is-active", unit)
	if cerr != nil {
		return false, cerr
	}
	out, _ := cmd.CombinedOutput()
	switch strings.TrimSpace(string(out)) {
	case "active", "activating", "reloading":
		return true, nil
	case "inactive", "failed", "deactivating", "":
		// "" = 单元不存在 ⇒ 视为不在跑（收干净了）
		return false, nil
	default:
		return false, nil
	}
}

// UnitCgroup 单元的 cgroup 路径（观测面：核"它确实不在 x3-agent.service 里"）。
func (h Hatcher) UnitCgroup(ctx context.Context, unit string) (string, error) {
	cmd, cerr := userScoped(ctx, "systemctl", "--user", "show", unit, "-p", "ControlGroup", "--value")
	if cerr != nil {
		return "", cerr
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读单元 cgroup 失败（%s）：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// MainPID 单元主进程 pid（**注意：它是 bwrap 的父进程，留在原命名空间，不是空间内进程**）。
//
// 真机事实（X3，2026-09-15）：MainPID=1379471 的 mnt-ns=mnt:[4026531832]（宿主视图：/models
// 不存在、/data 可见、550 pids），空间内进程是它的子进程 mnt-ns=mnt:[4026532767]。
// ⇒ **不要拿它去读 mountinfo 做封闭性核验**（读到宿主视图 ⇒ 每枚卵都会被判"不符"而收掉，
// 缺陷 2 的形态）。要核验请用 SpacePID / VerifyEnclosureForUnit / VerifyEnclosure
// （后两者会自己把 bwrap 父进程解析成空间内进程）。
// 本方法保留：拿单元主进程 pid 仍是归属层观测（E1 里看 MainPID 是否被换）要用的东西。
// 拿不到（单元不存在 / 还没起 / 输出不是正数）⇒ 返回错误：调用方必须如实记「未核验」，
// **不许**把「没核」当「核过了」。
func (h Hatcher) MainPID(ctx context.Context, unit string) (int, error) {
	if strings.TrimSpace(unit) == "" {
		return 0, fmt.Errorf("缺单元名")
	}
	cmd, cerr := userScoped(ctx, "systemctl", "--user", "show", unit, "-p", "MainPID", "--value")
	if cerr != nil {
		return 0, cerr
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("读单元主进程 pid 失败（%s）：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	raw := strings.TrimSpace(string(out))
	pid, perr := strconv.Atoi(raw)
	if perr != nil || pid <= 0 {
		return 0, fmt.Errorf("单元 %s 的主进程 pid 不是正数（%q）——按「拿不到 pid」处理，不做核验", unit, raw)
	}
	return pid, nil
}

// SpacePID 取**空间内进程** pid（缺陷 2 的正解）：同一单元 cgroup 内、挂载命名空间与宿主不同的
// 那个进程。返回的第二个值是判定依据（写进核验留痕，便于复盘"到底核的是谁"）。
//
// 为什么必须有它：单元的 MainPID 是 bwrap 的**父进程**，留在原命名空间（见 MainPID 注释）；
// 拿它读 mountinfo 得到宿主视图 ⇒ 判据「不符」⇒ 收卵 —— 每枚卵都被自己的核验杀掉。
//
// 找不到空间内进程 ⇒ 返回错误（调用方记「未核验」）；**绝不**退化成宿主视图。
func (h Hatcher) SpacePID(ctx context.Context, unit string) (int, string, error) {
	main, err := h.MainPID(ctx, unit)
	if err != nil {
		return 0, "", err
	}
	self, err := mntNSOf(os.Getpid())
	if err != nil {
		return 0, "", fmt.Errorf("读本进程（子端）挂载命名空间失败：%w", err)
	}
	f, err := procFacts(main)
	if err != nil {
		return 0, "", fmt.Errorf("读单元 %s 的 MainPID=%d 的 /proc 事实失败（进程已死？）：%w", unit, main, err)
	}
	pid, why, err := pickSpacePID(main, f.Cgroup, self, scanProcs())
	if err != nil {
		return 0, "", fmt.Errorf("单元 %s 的空间内进程没找到：%w", unit, err)
	}
	return pid, why, nil
}

// VerifyEnclosureForUnit 一条龙：单元 → **空间内进程** → 核验（调用方只需给单元名）。
//
// 生产路径应当用它（而不是自己 MainPID + VerifyEnclosure）：MainPID 是 bwrap 父进程，
// 见 SpacePID 注释；这里把「选对进程」这件事收在 hatch 层内部，调用方无从选错。
func (h Hatcher) VerifyEnclosureForUnit(ctx context.Context, unit string) (EnclosureReport, error) {
	pid, why, err := h.SpacePID(ctx, unit)
	if err != nil {
		return EnclosureReport{}, err
	}
	rep, err := h.VerifyEnclosure(pid)
	if err != nil {
		return EnclosureReport{}, fmt.Errorf("单元 %s：%w", unit, err)
	}
	// 留痕带上单元名 + 选进程的依据（「判据字符串里能看出校验的是哪个 pid」）
	rep.PIDSource = fmt.Sprintf("unit=%s %s", unit, why)
	return rep, nil
}

// mntNSOf 读 /proc/<pid>/ns/mnt 的链接目标（如 "mnt:[4026532767]"）——挂载命名空间的身份。
func mntNSOf(pid int) (string, error) {
	ns, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/mnt", pid))
	if err != nil {
		return "", fmt.Errorf("读 /proc/%d/ns/mnt 失败：%w", pid, err)
	}
	return ns, nil
}

// procFacts 读一个进程「是不是空间内进程」所需的几项事实（cgroup / 挂载命名空间 / comm）。
func procFacts(pid int) (spaceProc, error) {
	cg, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return spaceProc{}, fmt.Errorf("读 /proc/%d/cgroup 失败：%w", pid, err)
	}
	ns, err := mntNSOf(pid)
	if err != nil {
		return spaceProc{}, err
	}
	comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)) // comm 只用于「优先非 bwrap」，读不到不致命
	return spaceProc{PID: pid, Cgroup: string(cg), MntNS: ns, Comm: strings.TrimSpace(string(comm))}, nil
}

// scanProcs 扫 /proc 取所有读得到的进程事实。
//
// 读不到的单条**跳过**（进程可能在扫描中消失、或属于别的用户 ⇒ /proc/<pid>/ns/mnt 不可读）：
// 我们的目标进程与子端同用户，一定读得到；这里不做"宁可少一条也不错一条"的处理，因为缺一条
// 的后果是"找不到空间内进程"⇒ 记「未核验」（诚实），而不是"读到宿主视图"（危险）。
func scanProcs() []spaceProc {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	out := make([]spaceProc, 0, len(entries))
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		f, err := procFacts(pid)
		if err != nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

// VerifyEnclosure 运行时封闭性核验（§6.9 硬要求）：读 `/proc/<pid>/mountinfo` 实地核，
// **不得以单元状态为凭**。
//
// pid 语义（缺陷 2 修在此）：**给空间内进程就用它；给 bwrap 父进程（systemd MainPID 就是这么
// 个东西）就自动改验同一单元 cgroup 内的空间内进程**。核验对象与依据写进报告（PID / PIDSource），
// 结论话术里也带 pid —— 读日志的人能一眼看出"核了谁"。
//
// 硬不变量：**绝不拿宿主挂载命名空间的视图当核验凭据**。给进来的 pid 若在宿主命名空间、
// 而空间内进程找不到 ⇒ 返回错误（上层记「未核验」），而不是照着宿主视图报"不符"。
// VerifyEnclosure 核验封闭性（**不带**声明清单 ⇒ 逐文件那半条判据退化为结构形态）。
// 真机接线走 VerifyEnclosureDeclared（backend 从卵声明取 spec.WeightFiles 传入）。
func (h Hatcher) VerifyEnclosure(pid int) (EnclosureReport, error) {
	return h.verifyEnclosureDeclared(pid, nil)
}

// VerifyEnclosureDeclared 同 VerifyEnclosure，但带上**卵声明的权重文件清单**：判据从
// 「结构形态（无整目录挂载 + 每个 /models/<…> 都只读 + 至少有一个）」收紧到
// 「正好是这几个声明的文件、各自一条只读挂载」⇒ **挂错文件也会判不符**。
//
// 2026-09-15 接线（父代理拍）：后端 hatcher 接口此前只有 `VerifyEnclosure(pid int)`，
// 这一层只能传 nil ⇒「挂了别的文件」这类差异核验看不见（收口时留下的最后一道缺口）。
func (h Hatcher) VerifyEnclosureDeclared(pid int, declaredWeightFiles []string) (EnclosureReport, error) {
	return h.verifyEnclosureDeclared(pid, declaredWeightFiles)
}

func (h Hatcher) verifyEnclosureDeclared(pid int, declaredWeightFiles []string) (EnclosureReport, error) {
	target, why, err := resolveSpaceTarget(pid)
	if err != nil {
		return EnclosureReport{}, err
	}
	// 硬不变量兜底（缺陷 2）：核验对象**必须**在宿主以外的挂载命名空间里。
	// resolveSpaceTarget 的两条分支已经保证了这一点，这里在读 mountinfo 之前再钉一次 ——
	// 「拿宿主视图当封闭性凭据」是必须不可能发生的事（它会把每一枚好卵判成不符而收掉）。
	//
	// 它依赖的实机事实：bwrap 父进程**留在宿主命名空间**（与子端同一个 mnt-ns，真机
	// =mnt:[4026531832]），空间内进程在新 ns（真机 mnt:[4026532767]）。
	// 残余假设（写清，不藏）：若将来用户管理器给单元自带私有挂载命名空间，bwrap 父进程的 ns
	// 也会与子端不同，这条兜底就认不出它 —— 那时的收紧方向是按"根是不是新造的"判（bwrap 的
	// / 是新建的 tmpfs，宿主根不是），而不是继续加 pid 启发式。
	self, err := mntNSOf(os.Getpid())
	if err != nil {
		return EnclosureReport{}, fmt.Errorf("读本进程（子端）挂载命名空间失败：%w", err)
	}
	if ns, err := mntNSOf(target); err != nil {
		return EnclosureReport{}, fmt.Errorf("读核验对象 pid=%d 的挂载命名空间失败：%w", target, err)
	} else if ns == self {
		return EnclosureReport{}, fmt.Errorf("核验对象 pid=%d 还在宿主挂载命名空间（%s）里 ⇒ "+
			"按「读不到」处置：§6.9 不许拿宿主视图当封闭性凭据（那会把每枚好卵判成不符）", target, ns)
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/mountinfo", target))
	if err != nil {
		return EnclosureReport{}, fmt.Errorf("读 /proc/%d/mountinfo 失败（核验对象 %s）：%w", target, why, err)
	}
	// 「读到了但一行都认不得」也算**读不到**（空报告会被上层误判成「实读且不符」，两者的处置不同）。
	if err := mountinfoParseable(string(b)); err != nil {
		return EnclosureReport{}, fmt.Errorf("pid=%d：%w", target, err)
	}
	// 权重判据：声明清单由 backend 从卵声明（spec.WeightFiles）传入 ⇒ 逐文件那半条是**真判据**；
	// 清单为空时只验结构形态，且不假装验过（判据文本会写明核的是什么）。
	rep := CheckEnclosure(string(b), declaredWeightFiles)
	rep.PID = target
	rep.PIDSource = why
	// 进程视图隔离的间接证据：空间内 /proc 里进程数极少（宿主看不到）
	if n, err := countProcs(target); err == nil {
		rep.NewPIDNamespace = n <= 8 // bwrap+引擎的正常值；宿主上是几百上千
	}
	return rep, nil
}

// resolveSpaceTarget 定出「核验对象」pid 与其依据。
//
// 判定只用一件事：**挂载命名空间与宿主是否相同**（bwrap 必为空间新建挂载命名空间，故"不同"
// 就等于"它进了空间"）。刻意**不**用「mountinfo 里有没有 /models」之类的判据来选进程 ——
// 那正是要被核验的东西，用它选进程会把真"不符"掩盖成选错进程。
func resolveSpaceTarget(pid int) (int, string, error) {
	if pid <= 0 {
		return 0, "", fmt.Errorf("核验对象 pid 不是正数（%d）⇒ 按「读不到」处置，不核验", pid)
	}
	self, err := mntNSOf(os.Getpid())
	if err != nil {
		return 0, "", fmt.Errorf("读本进程（子端）挂载命名空间失败 ⇒ 无法判定核验对象是否在宿主视图里：%w", err)
	}
	f, err := procFacts(pid)
	if err != nil {
		return 0, "", fmt.Errorf("读 pid=%d 的 /proc 事实失败（进程已死？）：%w", pid, err)
	}
	if f.MntNS != "" && f.MntNS != self {
		return pid, fmt.Sprintf("按给定 pid=%d 核（mnt-ns=%s 与宿主 %s 不同 ⇒ 它就在空间内）", pid, f.MntNS, self), nil
	}
	// 实机形态：给进来的就是 bwrap 父进程（systemd MainPID）——它在宿主命名空间里
	target, why, perr := pickSpacePID(pid, f.Cgroup, self, scanProcs())
	if perr != nil {
		return 0, "", fmt.Errorf("给定 pid=%d 在宿主挂载命名空间（%s）里，%w", pid, self, perr)
	}
	return target, why, nil
}

// countProcs 数该 pid 命名空间里可见的进程数（读 /proc/<pid>/root/proc 或 /proc 下的子目录）。
func countProcs(pid int) (int, error) {
	entries, err := os.ReadDir(fmt.Sprintf("/proc/%d/root/proc", pid))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err == nil {
			n++
		}
	}
	return n, nil
}
