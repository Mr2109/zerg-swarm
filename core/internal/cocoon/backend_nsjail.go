//go:build linux

// backend_nsjail.go — Linux 真后端：**拼 nsjail 命令行 + 管进程/目录**（v3 §F 的 linux 行）。
//
// 定位（不要越界）：
//   - 隔离本体由 nsjail 提供（namespace + cgroup + rlimit），本层**不重新实现**；
//   - 判定全在控制面（身份认证、端口策略、契约校验）；本层只报事实；
//   - 不读茧内的 cocoon.toml，不在茧根内跑解释器/git/make，不跟符号链接删东西。
//
// ★ 与任务书原文的两处**有意对齐真 nsjail 语义**（照原文写命令行会被 nsjail 直接拒）：
//
//	① `--cgroup_memory` 不是 nsjail 的旗标；真名 `--cgroup_mem_max`，单位**字节**
//	   （`--rlimit_as` 才是 MB，所以 `mem_gb*1024` 这条换算落在 rlimit_as 上）。
//	② `--iface_no_lo` 的真义是「**不**把 lo 拉起来」；传了茧里就没有回环，AI 端点
//	   （走 127.0.0.1）当场不通。⇒ 这条与 `--disable_proc`、`--disable_clone_newnet`
//	   一样**有意不传**（详见 backend_nsjail_shared.go 文件头）。
//
// ★ 撞硬约束（照实登记，不假装已做）：
//
//	Ⅰ. 换根只认**给定目录**：`[茧].rootfs` 有值 ⇒ `--chroot <overlay 合并视图>`（lower=该目录
//	   只读 · upper=<茧目录>/upper 可写）+ 最小只读面（/proc、/dev/{null,zero,urandom,random}、
//	   系统面顶层目录）+ 可写 /tmp。**镜像拉取/解包（OCI）不在本件**：rootfs 目录得由调用方
//	   先备好。缺省不给 rootfs ⇒ 维持既有档（chroot=宿主根，茧内跑宿主二进制）——这是**既有
//	   档**，给了 rootfs 之后任何一步做不成都报错，不许退回它（见 nsjailRootfsPlanFor）。
//	Ⅰc. 命令面（`cocoon open`）**已**把茧目录交给后端（`CocoonDirSetter.SetCocoonDir(<真源根>/<id>/)`
//	   ⇒ 实例根 = 该目录下的 `<后端 id>/`，upper/work/merged/tmp 全在里面）；`close` 同样先设一遍
//	   （跨进程时按盘上句柄收尾要靠它算茧根）。直接调后端（绕过命令面）时，调用方须自己设 BaseDir。
//	Ⅰb. 网络两档（T5：**缺省断网**，档位来自契约 `[茧].net`，后端无全局开关）：
//	   ① net 缺省/"none" ⇒ 不传 `--disable_clone_newnet`，nsjail 建**新** netns：
//	      茧内只有回环（自己 127.0.0.1:<port> 能通），**出不了网**，宿主与 broker 也够不着茧内端口。
//	   ② net = "host" ⇒ 传 `--disable_clone_newnet`，共享宿主 netns：茧内端口即宿主端口，
//	      代价是**茧能出网**。
//	   `[[端口]]` 只在 ② 下合法（解析期就把 ①+[[端口]] 拒掉）；「既断外网又能被宿主访问」
//	   需要 DNAT/代理，本件未接，照实登记。
//	Ⅱ. 运行时未实测：开发机是 macOS，nsjail 起不来；本件只保证「参数拼装 + 结果契约
//	   + 交叉编译」三格，真机跑通要等 Linux 主机。
//	Ⅲ. 整树回收靠三条腿（killpg + pidns 拆除 + cgroup.kill），实测起决定作用的是 pidns
//	   拆除；cgroup.kill 要 CgroupMount 可写、且 nsjail 建得起 cgroup 才生效。能力表写的是
//	   「平台能给什么」，不是「每次配置都拿到」。
//	Ⅲb. nsjail 自己的日志走 stderr ⇒ ExecResult.Stderr 里混着 nsjail 的 [I]/[W] 行。
//	   不吞掉：那是失败时唯一的现场（吞了就只剩 RC=255 一个数字）。
//	Ⅳ. 不分配伪终端：ExecOpts.TTY=true 一律 ErrBackendUnsupported（见 Exec 里的说明）。
package cocoon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// nsjailRootMode — 茧根目录权限（只给本用户）。
const nsjailRootMode = 0o700

// Provision — 按契约登记一个茧（**不起进程**；服务由控制面按 Spec.Order 逐个 Exec）。
//
// 落盘面：BaseDir 非空 ⇒ 建 <BaseDir>/<id>（0700）；空 ⇒ 纯 namespace 隔离、不落盘。
//
// ★ 这里**不建 cgroup 目录**：nsjail 自己按 `<CgroupMount>/NSJAIL.<pid>` 建与收
// （见 backend_nsjail_shared.go 的 CgroupMount 注释）。我们抢着建一个它不用的目录，
// 只会在 Destroy 里删一个空壳，制造「cgroup 已隔离」的错觉。
func (b *NsjailBackend) Provision(ctx context.Context, spec *Spec) (Instance, error) {
	if spec == nil {
		return Instance{}, ErrNilSpec
	}
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.insts == nil {
		b.insts = make(map[string]*nsjailInstance)
	}
	b.nextID++
	id := fmt.Sprintf("%s-%d", nsjailBackendName, b.nextID)

	st := &nsjailInstance{
		id:     id,
		spec:   spec,
		mounts: make(map[string]nsjailMount),
		pids:   make(map[int]struct{}),
	}

	if b.BaseDir != "" {
		st.root = filepath.Join(b.BaseDir, id)
		if err := os.MkdirAll(st.root, nsjailRootMode); err != nil {
			return Instance{}, fmt.Errorf("cocoon: nsjail 建茧根 %s 失败: %w", st.root, err)
		}
		// 建了目录之后的任何报错都要把它收掉：留一个空茧根，`ls` 会看见一个不存在的实例。
		ok := false
		defer func() {
			if !ok {
				_ = os.RemoveAll(st.root)
			}
		}()
		// 换根面在 Provision 就定死（别等 Exec 才发现落点算不出来）：给了 [茧].rootfs 却没茧目录、
		// 或者那个目录根本不在/不是目录，现在报比跑到一半报有用。
		if _, err := nsjailRootfsPlanFor(spec, b.Chroot, st.root); err != nil {
			return Instance{}, err
		}
		if _, err := nsjailCheckRootfs(spec); err != nil {
			return Instance{}, err
		}
		ok = true
	} else if _, err := nsjailRootfsPlanFor(spec, b.Chroot, ""); err != nil {
		// 没配 BaseDir 但契约要 rootfs ⇒ 同一条红线（不许退回宿主根），只是没有目录可收。
		return Instance{}, err
	}

	b.insts[id] = st
	return Instance{ID: id, Backend: nsjailBackendName, Root: st.root}, nil
}

// Exec — 在茧内跑 `nsjail ... -- <argv>`（argv 不经宿主 shell）。
//
// 超时处理（v2 §D）：到期 → 整棵进程树 SIGTERM → 宽限 KillGrace → SIGKILL。
// 「整棵」用两条腿：配了 CgroupRoot 就先写 cgroup.kill（内核把子树全杀，含已 reparent 的
// 孙进程），再 kill(-pgid) 兜底（nsjail 是自己那一组的组长：Setpgid）。
//
// 返回值：超时/信号杀**不算后端故障**（err == nil，事实写在 ExecResult 里）；
// 只有「结果拿不到」（实例不在、ctx 取消、拉不起 nsjail）才返回 err。
//
// ★ Signal 只报**我们对 nsjail 自身**的观测（WaitStatus.Signaled），不猜茧内那层：
// 实测（真 nsjail 3.x）茧内 init 被杀时 nsjail 会以 128+n **正常退出**，所以那种情形
// 表现为 RC=128+n、Signal=""。RC 已经把事实说清楚了，硬把 Signal 填成 SIGxxx 反而是在
// 声称一个我们没观测到的信号。同理 rc=255 可能是「茧内命令退 255」也可能是 nsjail 自己
// 报错，本层不区分（区分要靠 nsjail 的日志文案，那比错报更脆）。
//
// ★ 两条档（ExecOpts.Detach）：
//
//	阻塞档（缺省）—— 起、等、报退出观测。tests / 同步用法。
//	后台档（Detach）—— setsid 起、收尸 goroutine 记账、立刻返回 {RC:0, PID:<nsjail pid>}。
//	   stdout/stderr 必须落文件（Validate 已拒「后台档没日志」）；退出观测留给
//	   DetachedStatus（命令面靠它验活、close 靠 meta 里的 pid 整组收）。
func (b *NsjailBackend) Exec(ctx context.Context, inst Instance, argv []string, opts ExecOpts) (ExecResult, error) {
	if err := opts.Validate(); err != nil {
		return ExecResult{}, err
	}
	if len(argv) == 0 || argv[0] == "" {
		return ExecResult{}, fmt.Errorf("%w: argv 为空或 argv[0] 为空", ErrBadArgv)
	}
	if opts.TTY {
		// 本后端不分配伪终端。TTY=true 却按普通管道跑，会静默改掉「行缓冲 / ^C 语义」
		// 这两件事（调用方以为拿到了 tty）⇒ 直接拒，比假装做到了强。
		return ExecResult{}, fmt.Errorf("%w: nsjail 后端未分配伪终端（ExecOpts.TTY=true 未接）", ErrBackendUnsupported)
	}
	if err := ctx.Err(); err != nil {
		return ExecResult{}, err
	}

	spec, mounts, instRoot, err := b.execView(inst)
	if err != nil {
		return ExecResult{}, err
	}
	// 根文件系统方案：给了 [茧].rootfs 就是真镜像根（overlay 合成），否则是既有档。
	// ★ 方案算不出来就报错 —— 不退回宿主根（那正是本单要消灭的「茧里跑宿主二进制」）。
	plan, err := nsjailRootfsPlanFor(spec, b.Chroot, instRoot)
	if err != nil {
		return ExecResult{}, err
	}
	// argv[0] 裸命令名 → 按**茧内** PATH 解析成绝对路径（nsjail 直接 execve、不走 PATH，
	// 实测见 nsjailResolveArgv0 的注释）。茧内解析的结果也只在茧内有效，绝不拿宿主路径凑。
	argv, err = nsjailResolveArgv0(argv, nsjailJailPATH(spec), plan.Chroot)
	if err != nil {
		return ExecResult{}, err
	}
	// 起进程前把根文件系统备好：overlay 挂不上就**报错**（宁红不假绿），不退回宿主根。
	if plan.Overlay {
		if err := nsjailOverlaySetup(plan); err != nil {
			return ExecResult{}, err
		}
	}
	if plan.IsJail() {
		// bind mount 要求目标存在：/proc、/dev、/tmp 与设备文件都在这里补齐（缺的落进 upper）。
		if err := nsjailEnsureJailDirs(plan); err != nil {
			return ExecResult{}, err
		}
	}
	// 快照一次挂载点：Exec 期间不再读可变字段（配置类字段本应在使用前定型）。
	cgroupMount := b.CgroupMount

	path := b.Path
	if path == "" {
		path = nsjailBinaryDefault
	}
	if _, err := exec.LookPath(path); err != nil {
		return ExecResult{}, fmt.Errorf("cocoon: nsjail 后端不可用：找不到可执行文件 %q（%w）", path, err)
	}

	args := nsjailExecArgvForPlan(spec, mounts, plan, opts.Dir, argv)
	// 网络档**只从契约读**（`[茧].net`）：host ⇒ 共享宿主 netns（茧内 127.0.0.1:<port> 就是
	// 宿主那一个端口）；none（缺省）⇒ 不传这条，nsjail 自己建新 netns ⇒ 茧内只有回环、出不了网。
	// ★ 这里曾经是 `b.HostNetwork`（命令面硬编码 true）：那是「为了让端口直通而默认把茧放上网」，
	//   与「默认断网」正相反。现在没有后端级的开关 —— 一个后端实例服务多个茧，开关只能是**每份契约**的。
	args = nsjailApplyNet(args, spec)

	cmd := exec.Command(path, args...)
	// 进程组：两条档的共同目标是「组号 == 子进程 pid」，这样 kill(-pid) 就是杀整棵树。
	//
	//	阻塞档 Setpgid：子进程自成一组，但仍在**本进程的会话**里（前台语义，超时可整组叫停）。
	//	后台档 Setsid：子进程自成**会话**（顺带自成进程组），与起它的 CLI 彻底脱钩 ——
	//	  CLI 退出（含被 SIGHUP / 终端消失）都不再传到它，它被 init 收养后继续跑。
	//	  ★ 这是「茧活过 CLI」的唯一手段：只 Setpgid 的话，CLI 一退整组仍会被会话清理带走。
	if opts.Detach {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	cmd.Env = nsjailEnv(spec, opts.Env)
	if opts.Stdin != nil {
		cmd.Stdin = bytes.NewReader(opts.Stdin)
	}
	var stdout, stderr bytes.Buffer
	if opts.LogPath != "" {
		// ★ 长驻服务落文件（见 ExecOpts.LogPath）：open 进程退场后管道读端就没了，
		//   服务下一次写 stdout/stderr 会撞 EPIPE —— 而服务本身不该因为「起它的那个 CLI 走了」而死。
		if err := os.MkdirAll(filepath.Dir(opts.LogPath), 0o750); err != nil {
			return ExecResult{}, fmt.Errorf("cocoon: 建日志目录 %s 失败: %w", filepath.Dir(opts.LogPath), err)
		}
		lf, lerr := os.OpenFile(opts.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if lerr != nil {
			return ExecResult{}, fmt.Errorf("cocoon: 打开日志 %s 失败: %w", opts.LogPath, lerr)
		}
		defer lf.Close() // 子进程有自己的 dup，这里关只影响本进程
		cmd.Stdout, cmd.Stderr = lf, lf
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("cocoon: 拉起 nsjail 失败（%s %v）: %w", path, args, err)
	}
	pid := cmd.Process.Pid
	b.addPID(inst.ID, pid)

	// ── 后台档：签出即返回，不等退出 ──────────────────────────────────
	//
	// ★ 这里**不 Wait**：Wait 会阻塞到进程结束，那就又变回「服务 spawn 在 CLI 的调用栈里」。
	// 但完全不 Wait 也不行：子进程退出后没人回收 ⇒ 留一个僵尸（/proc 里有、实际已死），
	// 判活就会把它当「在跑」。所以用一个 goroutine 收尸、把退出观测记进 detached 表。
	if opts.Detach {
		d := &nsjailDetach{done: make(chan struct{})}
		b.trackDetached(pid, d)
		go func() {
			waitErr := cmd.Wait()
			code, sig, werr := nsjailExitOf(waitErr)
			if werr != nil {
				// 解码不了（罕见）：按「非零退出」记，绝不记成「正常完成 0」。
				code, sig = 1, 0
			}
			d.res, _ = nsjailResult(code, sig, false, nil, nil, time.Since(start).Milliseconds())
			close(d.done)
		}()
		// RC=0 + PID：语义是「进程已签出」，不是「服务已就绪」（见 ExecOpts.Detach 硬约束③）。
		return ExecResult{RC: 0, PID: pid, MS: time.Since(start).Milliseconds()}, nil
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var waitErr error
	timedOut := false
	// jailDirs — 本座茧的 cgroup 目录（只有超时/收尾要杀树时才用得着）。
	// ★ 必须在发第一个信号**之前**解析：nsjail 一死，茧内进程的父链就接到 init，
	// 靠父链认不出「这是我那座茧」了（见 nsjailJailCgroups 的说明）。
	var jailDirs []string

	if opts.Timeout > 0 {
		timer := time.NewTimer(opts.Timeout)
		defer timer.Stop()
		select {
		case waitErr = <-waitCh:
		case <-timer.C:
			timedOut = true
			jailDirs = nsjailJailCgroups(cgroupMount, pid)
			b.killTree(cgroupMount, pid, jailDirs, syscall.SIGTERM)
			if b.KillGrace <= 0 {
				b.killTree(cgroupMount, pid, jailDirs, syscall.SIGKILL)
				waitErr = <-waitCh
				break
			}
			grace := time.NewTimer(b.KillGrace)
			defer grace.Stop()
			select {
			case waitErr = <-waitCh:
			case <-grace.C:
				b.killTree(cgroupMount, pid, jailDirs, syscall.SIGKILL)
				waitErr = <-waitCh
			}
		case <-ctx.Done():
			b.killTree(cgroupMount, pid, nsjailJailCgroups(cgroupMount, pid), syscall.SIGKILL)
			<-waitCh
			b.dropPID(inst.ID, pid)
			return ExecResult{}, ctx.Err()
		}
	} else {
		select {
		case waitErr = <-waitCh:
		case <-ctx.Done():
			b.killTree(cgroupMount, pid, nsjailJailCgroups(cgroupMount, pid), syscall.SIGKILL)
			<-waitCh
			b.dropPID(inst.ID, pid)
			return ExecResult{}, ctx.Err()
		}
	}

	// 收尾：一次性 exec 不该留下后台孙进程（主进程退了但组内/茧 cgroup 里还有残留时，这一刀收掉）。
	b.killTree(cgroupMount, pid, jailDirs, syscall.SIGKILL)
	b.dropPID(inst.ID, pid)
	// 一次性模式的 cgroup 目录名同样是茧内 init 的 pid ⇒ Exec 退场后按「pid 死了 + 空的」收壳，
	// 否则每跑一次 exec 就在 /sys/fs/cgroup 攒一个目录。
	_ = nsjailReapEmptyCgroups(cgroupMount)

	exitCode, signum, werr := nsjailExitOf(waitErr)
	if werr != nil {
		return ExecResult{}, werr
	}

	return nsjailResult(exitCode, signum, timedOut, stdout.Bytes(), stderr.Bytes(),
		time.Since(start).Milliseconds())
}

// nsjailExitOf — cmd.Wait 的错 → (退出码, 信号号, 解码错误)。
//
// ★ 两条档共用一份口径（阻塞档与后台档的收尸），免得「后台档的退出码」长成另一套。
// 被信号杀时报信号号、退出码报 0；正常退出报退出码、信号号报 0。
func nsjailExitOf(waitErr error) (exitCode, signum int, err error) {
	if waitErr == nil {
		return 0, 0, nil
	}
	var ee *exec.ExitError
	if !errors.As(waitErr, &ee) {
		return 0, 0, fmt.Errorf("cocoon: 等 nsjail 退出失败: %w", waitErr)
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 0, int(ws.Signal()), nil
	}
	return ee.ExitCode(), 0, nil
}

// trackDetached — 记下一个后台档进程的观测点（pid 复用极不可能，直接覆盖）。
func (b *NsjailBackend) trackDetached(pid int, d *nsjailDetach) {
	if pid <= 0 || d == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.detached == nil {
		b.detached = make(map[int]*nsjailDetach)
	}
	b.detached[pid] = d
}

// DetachedStatus — 实现 DetachedProbe：后台档进程现在还在不在。
//
// 两条路（顺序要紧）：
//
//	① 本进程起的（detached 表里有）⇒ 看收尸 goroutine 的退出观测。★ 这条路比 /proc 更准：
//	   进程刚退、还没被 wait 掉时它在 /proc 里是僵尸，/proc 只能判「死了」而给不出退出码。
//	② 不是本进程起的（`close` 是另一个进程 / CLI 早退场）⇒ 只能按 /proc 判（僵尸算死），
//	   退出码拿不到 ⇒ 如实报 RC=1（✗ 不许猜 0，那会被读成「正常完成」）。
func (b *NsjailBackend) DetachedStatus(pid int) (alive bool, exited bool, res ExecResult) {
	if pid <= 0 {
		return false, true, ExecResult{RC: 1}
	}
	b.mu.Lock()
	d := b.detached[pid]
	b.mu.Unlock()
	if d != nil {
		select {
		case <-d.done:
			return false, true, d.res
		default:
			return true, false, ExecResult{}
		}
	}
	if nsjailPidRunning(pid) {
		return true, false, ExecResult{}
	}
	return false, true, ExecResult{RC: 1}
}

// Mount — 记一次挂载（nsjail 的 --bindmount 只能在起进程时给，所以这里只登记）。
//
// 同一 innerPath 不许挂两次到不同源/不同读写位（ErrMountConflict）；同源同模式重复调用幂等。
func (b *NsjailBackend) Mount(ctx context.Context, inst Instance, hostPath, innerPath string, ro bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hostPath == "" || innerPath == "" {
		return fmt.Errorf("%w: 宿主/茧内路径都不得为空（host=%q inner=%q）", ErrBadMount, hostPath, innerPath)
	}
	// nsjail 的 --bindmount 只认绝对路径；相对路径在它那里是「相对 argv[0] 的工作目录」，
	// 语义与我们想表达的完全不同 ⇒ 提前拒。
	if !filepath.IsAbs(hostPath) || !filepath.IsAbs(innerPath) {
		return fmt.Errorf("%w: 必须是绝对路径（host=%q inner=%q）", ErrBadMount, hostPath, innerPath)
	}
	// ★ `/` 不许整根挂：把宿主根挂成茧内 `/` 等于把「换根」这件事作废（隔离面只剩 namespace），
	// 而调用方会以为自己挂了个数据盘。这条是承诺，不是建议 —— 真要整根，那就不该用茧。
	if filepath.Clean(innerPath) == nsjailDefaultChroot {
		return fmt.Errorf("%w: 不许把 %q 整根挂到茧内 \"/\"（隔离面等于没有；只许挂子目录）",
			ErrBadMount, hostPath)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	st, err := b.lookupLocked(inst)
	if err != nil {
		return err
	}
	if prev, ok := st.mounts[innerPath]; ok {
		if prev.Host != hostPath || prev.RO != ro {
			return fmt.Errorf("%w: 茧内 %q 已挂 %q(ro=%v)，不能再挂 %q(ro=%v)",
				ErrMountConflict, innerPath, prev.Host, prev.RO, hostPath, ro)
		}
		return nil
	}
	st.mounts[innerPath] = nsjailMount{Host: hostPath, Inner: innerPath, RO: ro}
	return nil
}

// Expose — **不支持**：Linux 后端不映射端口。
//
// 理由：茧与调用方在同一个网络栈里（nsjail 只做 CLONE_NEWNET 的隔离，不做 DNAT），
// 茧内监听的端口本来就是宿主端口 ⇒ 没有「映射」这回事，调用方直连 127.0.0.1:<inner> 即可。
// 这里返回 ErrBackendUnsupported 而不是「假装映射成功返回同一个端口号」：后者会让上层
// 以为有转发链路，出问题时查错方向。
func (b *NsjailBackend) Expose(ctx context.Context, inst Instance, innerPort int, bind string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	b.mu.Lock()
	_, err := b.lookupLocked(inst)
	b.mu.Unlock()
	if err != nil {
		return 0, err
	}
	if innerPort < 1 || innerPort > 65535 {
		return 0, fmt.Errorf("%w: 茧内端口 %d 越界（1..65535）", ErrBadPort, innerPort)
	}
	if bind == "" {
		bind = DefaultBind
	}
	return 0, fmt.Errorf("%w: Linux 后端不映射端口（茧与调用方同一宿主网络栈，nsjail 不做 DNAT），请由调用方直连 %s:%d",
		ErrBackendUnsupported, bind, innerPort)
}

// Destroy — 收进程树、清 cgroup 目录（若已建）、删茧根。**幂等**：已销毁的再调返回 nil。
func (b *NsjailBackend) Destroy(ctx context.Context, inst Instance) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	st, err := b.lookupLockedAnyState(inst)
	if err != nil {
		return err
	}
	if st.gone {
		return nil
	}

	// ① 先把还活着的树收掉（Destroy 可能被用在「Exec 还在跑」的情形，且**多服务**茧
	//    同时挂着好几个 exec 主进程 —— 只收 st.pid 那一个会漏掉其余服务）。
	//    先解析 cgroup 目录再发信号：顺序反了就只能靠 killpg，setsid 跑掉的孙进程会漏。
	live := make([]int, 0, len(st.pids)+1)
	for pid := range st.pids {
		if pid > 0 {
			live = append(live, pid)
		}
	}
	if st.pid > 0 && len(live) == 0 {
		live = append(live, st.pid)
	}
	sort.Ints(live)
	type jailOf struct {
		pid  int
		dirs []string
	}
	jails := make([]jailOf, 0, len(live))
	for _, pid := range live {
		jails = append(jails, jailOf{pid: pid, dirs: nsjailJailCgroups(b.CgroupMount, pid)})
	}
	for _, j := range jails {
		b.killTree(b.CgroupMount, j.pid, j.dirs, syscall.SIGKILL)
	}
	st.pid = 0
	st.pids = make(map[int]struct{})

	// ② cgroup 目录：nsjail 自己建的是 <mount>/NSJAIL.<pid>（v2）或
	//    <mount>/NSJAIL/NSJAIL.<pid>（v1）；它正常退出时会自己收，崩掉/被 SIGKILL 时会留空壳
	//    ⇒ 这里按真实布局补删，清不掉（里面还有活进程）如实报出来。
	var errs []error
	for _, pid := range live {
		errs = append(errs, removeCgroupDirs(b.CgroupMount, pid)...)
	}
	// ②b 还活着时**解析出来的**那些目录（目录名是茧内 init 的宿主 pid，不是我们手里的 pid）。
	for _, j := range jails {
		for _, dir := range j.dirs {
			if err := removeCgroupDir(dir); err != nil {
				errs = append(errs, err)
			}
		}
	}
	// ②c 兜底：树已死 ⇒ 按「pid 不在 /proc + cgroup.procs 空」把空壳收掉（见 nsjailReapEmptyCgroups）。
	errs = append(errs, nsjailReapEmptyCgroups(b.CgroupMount)...)

	// ③ 茧根：整棵删（这里只删后端自己造的目录，且不跟随符号链接——RemoveAll 遇链接只删链）。
	// ★ 先卸 overlay 再删目录：挂着的合并视图底下删不动（EBUSY），且不卸就删等于
	//   把「还挂着的挂载点」留成残留（下一次同名目录会被它遮住）。
	if plan, perr := nsjailRootfsPlanFor(st.spec, b.Chroot, st.root); perr == nil && plan.Overlay {
		if uerr := nsjailOverlayTeardown(plan); uerr != nil {
			errs = append(errs, uerr)
		}
	}
	if st.root != "" {
		if err := os.RemoveAll(st.root); err != nil {
			errs = append(errs, fmt.Errorf("cocoon: 删茧根 %s 失败: %w", st.root, err))
		}
	}

	st.gone = true
	return errors.Join(errs...)
}

// Capabilities — v3 §F 的 linux 行（四个 true 逐项来自 backend.go 的表，不在这里另写一份）。
//
// ★ 「能力声明」与「本次配置」是两件事：表说的是**平台能给什么**；cgroup.kill 真正可用
// 还要 CgroupMount 配得上（配空则超时只走 killpg 兜底，见 killTree）。
func (b *NsjailBackend) Capabilities() (Capabilities, error) {
	caps := LinuxCapabilities()
	if err := caps.Validate(); err != nil {
		return Capabilities{}, err
	}
	return caps, nil
}

// ── 内部 ──────────────────────────────────────────────────────────────

// execView — 取一次 exec 要用的快照（spec + 挂载表 + 茧目录）。
//
// 拷一份挂载表再放锁：Exec 可能跑很久，不能攥着锁等进程退出（Destroy/Mount 会被饿死）。
func (b *NsjailBackend) execView(inst Instance) (*Spec, map[string]nsjailMount, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	st, err := b.lookupLocked(inst)
	if err != nil {
		return nil, nil, "", err
	}
	mounts := make(map[string]nsjailMount, len(st.mounts))
	for k, v := range st.mounts {
		mounts[k] = v
	}
	return st.spec, mounts, st.root, nil
}

// lookupLocked — 按 ID + 后端名校验归属（不信任调用方拼出来的 Instance），并拒已销毁实例。
func (b *NsjailBackend) lookupLocked(inst Instance) (*nsjailInstance, error) {
	st, err := b.lookupLockedAnyState(inst)
	if err != nil {
		return nil, err
	}
	if st.gone {
		return nil, fmt.Errorf("%w: %q", ErrInstanceGone, inst.ID)
	}
	return st, nil
}

// lookupLockedAnyState — 同上但不判 gone（Destroy 要能对已销毁实例返回 nil）。
func (b *NsjailBackend) lookupLockedAnyState(inst Instance) (*nsjailInstance, error) {
	if inst.Backend != nsjailBackendName {
		return nil, fmt.Errorf("%w: 句柄后端是 %q，不是 %q", ErrInstanceNotFound, inst.Backend, nsjailBackendName)
	}
	st, ok := b.insts[inst.ID]
	if !ok || st == nil {
		return nil, fmt.Errorf("%w: %q", ErrInstanceNotFound, inst.ID)
	}
	return st, nil
}

// addPID — 记下一次 exec 的主进程 pid（诊断 + 跨进程收尾用）。
func (b *NsjailBackend) addPID(id string, pid int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st, ok := b.insts[id]; ok && st != nil {
		st.pid = pid
		if st.pids == nil {
			st.pids = make(map[int]struct{})
		}
		st.pids[pid] = struct{}{}
	}
}

// dropPID — 该 exec 已退（结果已拿到），从活 pid 表里摘掉。
func (b *NsjailBackend) dropPID(id string, pid int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st, ok := b.insts[id]; ok && st != nil {
		if st.pid == pid {
			st.pid = 0
		}
		delete(st.pids, pid)
	}
}

// InstancePIDs — 该实例当前还在跑的 exec 主进程（升序）。空/nil = 没有活进程。
//
// ★ 这个列表是 `cocoon open` 落进 meta 的那一份：open 进程退出后，收尾只能靠它。
func (b *NsjailBackend) InstancePIDs(inst Instance) []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.insts[inst.ID]
	if !ok || st == nil || st.gone {
		return nil
	}
	out := make([]int, 0, len(st.pids))
	for pid := range st.pids {
		if pid > 0 {
			out = append(out, pid)
		}
	}
	sort.Ints(out)
	return out
}

// ReapPersisted — 按**盘上句柄**收干净（跨进程 `close` 的唯一入口，见 HandleReaper）。
//
// 与 Destroy 的差别只有一条：Destroy 认内存记账，这里认 meta 里那份 pid 表。
// 因此这里**先验明 pid 归属再下刀**：cmdline 不是本后端的 nsjail ⇒ 拒杀（并如实报错）；
// pid 早就没了 ⇒ 当「已经收过」，不算失败（幂等）。
func (b *NsjailBackend) ReapPersisted(ctx context.Context, inst Instance, pids []int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if inst.Backend != nsjailBackendName {
		return fmt.Errorf("%w: 句柄后端是 %q，不是 %q", ErrInstanceNotFound, inst.Backend, nsjailBackendName)
	}

	var errs []error
	ours := make([]int, 0, len(pids))
	jailDirs := make(map[int][]string, len(pids))
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		if !nsjailPidIsOurs(pid, b.Path) {
			// 已不在 = 之前收过了；还在但不是我们的 = 拒绝下刀（pid 复用 / meta 被改）。
			if nsjailPidAlive(pid) {
				errs = append(errs, fmt.Errorf(
					"cocoon: 跳过 pid %d：/proc/%d/cmdline 不是本后端的 nsjail（pid 可能已被复用，拒杀）", pid, pid))
			}
			continue
		}
		// ★ cgroup 目录必须在发信号**之前**解析：nsjail 一死，父链就断了（见 killTree）。
		ours = append(ours, pid)
		jailDirs[pid] = nsjailJailCgroups(b.CgroupMount, pid)
	}

	if len(ours) > 0 {
		for _, pid := range ours {
			b.killTree(b.CgroupMount, pid, jailDirs[pid], syscall.SIGTERM)
		}
		grace := b.KillGrace
		if grace <= 0 {
			grace = 0
		}
		b.waitPidsGone(ctx, ours, grace)
		for _, pid := range ours {
			if nsjailPidRunning(pid) {
				b.killTree(b.CgroupMount, pid, nil, syscall.SIGKILL)
			}
		}
		b.waitPidsGone(ctx, ours, 2*time.Second)
		for _, pid := range ours {
			// ★ 判「还在」用 nsjailPidRunning（僵尸算死），不是 nsjailPidAlive：
			//   后台档起的服务在 CLI 退场后被 init 收养，它被杀之后**可能**以僵尸形态留在
			//   /proc 里等父进程回收（实测：容器里 pid 1 是 sleep，永不回收 ⇒ 僵尸长驻）。
			//   僵尸不占端口、不跑代码，把它报成「收不干净」是错判（曾因此让 close 恒红）。
			if nsjailPidRunning(pid) {
				errs = append(errs, fmt.Errorf("cocoon: pid %d 在 SIGKILL 后仍在（收不干净，如实上报）", pid))
			}
		}
	}

	for _, pid := range pids {
		errs = append(errs, removeCgroupDirs(b.CgroupMount, pid)...)
		for _, dir := range jailDirs[pid] {
			if err := removeCgroupDir(dir); err != nil {
				errs = append(errs, err)
			}
		}
	}
	// 兜底：树已死 ⇒ 收空壳（跨进程时这份 pid 表是盘上那份，可能已经和 cgroup 名字对不上）。
	errs = append(errs, nsjailReapEmptyCgroups(b.CgroupMount)...)
	if inst.Root != "" {
		// 跨进程收尾手里没有 spec ⇒ 只能按**固定的**目录布局找合并视图（`<茧根>/merged`）。
		// 找不到挂载点就是没有（幂等），找到才卸；卸不掉如实报。
		if uerr := nsjailOverlayTeardownAt(filepath.Join(inst.Root, nsjailMergedDirName)); uerr != nil {
			errs = append(errs, uerr)
		}
		if rerr := b.reapRoot(inst.Root); rerr != nil {
			errs = append(errs, rerr)
		}
	}

	// 内存账（同进程先 Destroy 过的话这里已经是 gone；跨进程则本来就没有这一号）。
	b.mu.Lock()
	if st, ok := b.insts[inst.ID]; ok && st != nil {
		st.gone = true
		st.pid = 0
		st.pids = make(map[int]struct{})
	}
	b.mu.Unlock()

	return errors.Join(errs...)
}

// waitPidsGone — 等到这些 pid 全没了或超时（ctx 到期也提前返回，不装死）。
//
// ★ 「没了」的判据是 nsjailPidRunning（僵尸算死）：僵尸不占端口、不跑代码，等它被父进程
// 回收不该算「还没停干净」（容器里 pid 1 不回收僵尸 ⇒ 用 nsjailPidAlive 会一直等到超时）。
func (b *NsjailBackend) waitPidsGone(ctx context.Context, pids []int, d time.Duration) {
	if d <= 0 {
		return
	}
	deadline := time.Now().Add(d)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		alive := false
		for _, pid := range pids {
			if nsjailPidRunning(pid) {
				alive = true
				break
			}
		}
		if !alive || !time.Now().Before(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// reapRoot — 删茧根：只删「BaseDir 下一层」的那个目录（证明不了是我造的就不删）。
func (b *NsjailBackend) reapRoot(root string) error {
	root = filepath.Clean(root)
	if b.BaseDir == "" {
		return fmt.Errorf("cocoon: 拒删茧根 %s：本后端没配 BaseDir（无法证明该目录由我创建）", root)
	}
	if filepath.Dir(root) != filepath.Clean(b.BaseDir) {
		return fmt.Errorf("cocoon: 拒删茧根 %s：不在 BaseDir %s 下一层", root, b.BaseDir)
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("cocoon: 删茧根 %s 失败: %w", root, err)
	}
	return nil
}

// killTree — 杀整棵进程树。三条腿，按「谁真的管用」排序写清（★ 都实测过，别按想象改）：
//
//	① kill(-pgid)：nsjail 与我们之间是 Setpgid 出来的那一组；这一刀同时打掉 nsjail 与茧 init。
//	② pidns 拆除：茧 init 一死，内核会把**整个 pid 名字空间**里的进程清掉 —— 实测里真正
//	   把 setsid 跑出进程组的孙进程收干净的就是它（关掉 cgroup.kill 也照样收干净）。
//	③ cgroup.kill：往茧 cgroup 的 cgroup.kill 写 "1"，让内核把子树全杀。
//	   它是**第三重保险**（例如茧 init 还活着、组里有残留的情形），不是唯一手段 ——
//	   能力表里 KillMechanism 写的 cgroup.kill 说的是「这个平台给得起内核级整树回收」，
//	   不等于「本层只靠它」。
//
// 路径按 nsjail 源码的真实布局取（见 nsjailCgroupPaths / nsjailJailCgroups）：
// v2 = <mount>/NSJAIL.<茧 init 的宿主 pid>。文件不存在就跳过（v1 没有 cgroup.kill；
// 没配 CgroupMount 时 mount 为空）。
func (b *NsjailBackend) killTree(cgroupMount string, pgid int, jailDirs []string, sig syscall.Signal) {
	if sig == syscall.SIGKILL && pgid > 0 {
		// ① 茧 cgroup 的 cgroup.kill（内核级；抓得到 setsid 跑出进程组的孙进程）。
		//    候选 = 调用方解析出来的目录 + 按 pid 拼的两种布局（后者在 nsjail 哪天改成
		//    「按我们手里的 pid 命名」时自动生效，现在多半是空的，写不进去也无害）。
		seen := make(map[string]bool, len(jailDirs)+2)
		for _, dir := range append(append([]string(nil), jailDirs...), nsjailCgroupPaths(cgroupMount, pgid)...) {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			_ = os.WriteFile(filepath.Join(dir, "cgroup.kill"), []byte("1"), 0o644)
		}
	}
	if pgid <= 0 {
		return
	}
	if err := syscall.Kill(-pgid, sig); err != nil {
		// 组已空 ⇒ ESRCH。此时**不**再按正数 pid 补刀：那会打到「pid 号被复用后的另一个
		// 进程」上（误杀无关进程比少杀一条残留糟得多）。
		// ★ 反过来的极端情形（-pgid 打到一个刚被复用的新进程组）要靠「wait 完立刻杀」的
		// 时间窗挡住：复用需要 pid 号绕一整圈，微秒级窗口内做不到。
		if !errors.Is(err, syscall.ESRCH) {
			_ = syscall.Kill(pgid, sig)
		}
	}
}
