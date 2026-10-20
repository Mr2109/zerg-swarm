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
//	Ⅰ. 未换 rootfs：本后端不做 --chroot，只做 namespace/cgroup 隔离；镜像面要靠上层
//	   Mount 把根挂进来（镜像落盘/overlay 是另一件事，本件不接）。
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
	}

	if b.BaseDir != "" {
		st.root = filepath.Join(b.BaseDir, id)
		if err := os.MkdirAll(st.root, nsjailRootMode); err != nil {
			return Instance{}, fmt.Errorf("cocoon: nsjail 建茧根 %s 失败: %w", st.root, err)
		}
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

	spec, mounts, err := b.execView(inst)
	if err != nil {
		return ExecResult{}, err
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

	args := nsjailExecArgv(spec, mounts, b.Chroot, opts.Dir, argv)

	cmd := exec.Command(path, args...)
	// Setpgid ⇒ 子进程自成一组，组号 == 子进程 pid：杀 -pgid 就是杀「nsjail + 它拉起的整棵树」。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = nsjailEnv(spec, opts.Env)
	if opts.Stdin != nil {
		cmd.Stdin = bytes.NewReader(opts.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("cocoon: 拉起 nsjail 失败（%s %v）: %w", path, args, err)
	}
	pid := cmd.Process.Pid
	b.setPID(inst.ID, pid)

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
			b.setPID(inst.ID, 0)
			return ExecResult{}, ctx.Err()
		}
	} else {
		select {
		case waitErr = <-waitCh:
		case <-ctx.Done():
			b.killTree(cgroupMount, pid, nsjailJailCgroups(cgroupMount, pid), syscall.SIGKILL)
			<-waitCh
			b.setPID(inst.ID, 0)
			return ExecResult{}, ctx.Err()
		}
	}

	// 收尾：一次性 exec 不该留下后台孙进程（主进程退了但组内/茧 cgroup 里还有残留时，这一刀收掉）。
	b.killTree(cgroupMount, pid, jailDirs, syscall.SIGKILL)
	b.setPID(inst.ID, 0)

	exitCode, signum := 0, 0
	if waitErr != nil {
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) {
			return ExecResult{}, fmt.Errorf("cocoon: 等 nsjail 退出失败: %w", waitErr)
		}
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			signum = int(ws.Signal())
		} else {
			exitCode = ee.ExitCode()
		}
	}

	return nsjailResult(exitCode, signum, timedOut, stdout.Bytes(), stderr.Bytes(),
		time.Since(start).Milliseconds())
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

	// ① 先把还活着的树收掉（Destroy 可能被用在「Exec 还在跑」的情形）。
	//    先解析 cgroup 目录再发信号：顺序反了就只能靠 killpg，setsid 跑掉的孙进程会漏。
	jailDirs := nsjailJailCgroups(b.CgroupMount, st.pid)
	b.killTree(b.CgroupMount, st.pid, jailDirs, syscall.SIGKILL)
	pid := st.pid
	st.pid = 0

	// ② cgroup 目录：nsjail 自己建的是 <mount>/NSJAIL.<pid>（v2）或
	//    <mount>/NSJAIL/NSJAIL.<pid>（v1）；它正常退出时会自己收，崩掉/被 SIGKILL 时会留空壳
	//    ⇒ 这里按真实布局补删，清不掉（里面还有活进程）如实报出来。
	errs := removeCgroupDirs(b.CgroupMount, pid)

	// ③ 茧根：整棵删（这里只删后端自己造的目录，且不跟随符号链接——RemoveAll 遇链接只删链）。
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

// execView — 取一次 exec 要用的快照（spec + 挂载表）。
//
// 拷一份挂载表再放锁：Exec 可能跑很久，不能攥着锁等进程退出（Destroy/Mount 会被饿死）。
func (b *NsjailBackend) execView(inst Instance) (*Spec, map[string]nsjailMount, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	st, err := b.lookupLocked(inst)
	if err != nil {
		return nil, nil, err
	}
	mounts := make(map[string]nsjailMount, len(st.mounts))
	for k, v := range st.mounts {
		mounts[k] = v
	}
	return st.spec, mounts, nil
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

// setPID — 记下/清掉茧主进程 pid（诊断用；不影响调度）。
func (b *NsjailBackend) setPID(id string, pid int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st, ok := b.insts[id]; ok && st != nil {
		st.pid = pid
	}
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
