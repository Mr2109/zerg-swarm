// backend_nsjail_shared.go — nsjail 后端的**两平台共用**部分：类型、构造、纯函数
// （参数拼装 / 结果拼装 / 信号名 / env 组装）。
//
// ★ 为什么不跟实现一起放进 backend_nsjail.go：那份文件带 `//go:build linux`，而
// 「参数拼得对不对」这件事**必须在开发机（macOS）上就能判**（跑不了 nsjail 不等于
// 判不了 argv）。把纯函数留在这里，三格测试里第①格才不是在 linux 上才跑的空转。
//
// ★ 旗标名按 nsjail 官方 cmdline.cc 对齐（不是照抄设计稿的文字）：
//
//	--cgroup_mem_max        单位**字节**（设计稿写的 --cgroup_memory 不是 nsjail 的旗标）
//	--cgroup_cpu_ms_per_sec 单位「每秒可用 CPU 毫秒」数
//	--rlimit_as             单位 **MB**
//	--max_cpus              个数，0 = 不限
//	--iface_no_lo           真义是「**不**把 lo 拉起来」⇒ 要回环就**不许**传
//	--disable_proc          真义是「不挂 procfs」⇒ 语言运行时要用 /proc 就**不许**传
//	--disable_clone_newnet  真义是「**不**建 netns，直接用宿主网络」⇒ 看 HostNetwork 位（见下）
//
// 前两条「不传」是**有意为空**的：它们不是漏了，而是传了就与设计目标相反
// （茧内要回环给 AI 端点、要有 /proc）。测试里逐条断言「不含」。
//
// ★ `--disable_clone_newnet` 与上面两条不同，它由 **NsjailBackend.HostNetwork** 决定
// （缺省不传 = 茧有独立 netns ⇒ 断外网；命令面置 true = 共享宿主 netns ⇒ 茧内端口即宿主端口）。
// 两条承诺互斥，只能选一条：要「能出网之外的隔离」就得接 DNAT/代理，本件未接
// （见 backend_nsjail.go 文件头撞约束 Ⅰb）。
package cocoon

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// nsjailBackendName — 写进 Instance.Backend 的后端名（诊断/审计用）。
	nsjailBackendName = "nsjail"
	// nsjailBinaryDefault — 默认按 PATH 找 nsjail（可用 NsjailBackend.Path 覆盖）。
	nsjailBinaryDefault = "nsjail"
	// nsjailDefaultNoFile — RLIMIT_NOFILE 默认值。nsjail 自己缺省只有 32，
	// 起任何带连接池/线程池的服务都不够，所以显式抬到 1024。
	nsjailDefaultNoFile = 1024
	// nsjailDefaultKillGrace — 超时先 SIGTERM，宽限这么久再 SIGKILL（v2 §D 的 10s）。
	nsjailDefaultKillGrace = 10 * time.Second
	// nsjailDefaultPATH — 茧内最小 PATH（不继承宿主 PATH，避免把宿主工具链带进去）。
	nsjailDefaultPATH = "/usr/local/bin:/usr/bin:/bin"
	// nsjailDefaultCgroupMount — cgroup 挂载点（v2 统一层级在多数发行版都挂这里）。
	nsjailDefaultCgroupMount = "/sys/fs/cgroup"
	// nsjailDefaultChroot — 缺省根文件系统（宿主根；真镜像根由上层给）。
	nsjailDefaultChroot = "/"
)

// NsjailBackend — Linux 上的真后端：**只负责拼 nsjail 命令行并管进程/目录**，
// 隔离本体由 nsjail 提供（本层不重新实现 namespace/cgroup）。
//
// 非 linux 平台上这份类型照样存在（见 backend_nsjail_other.go）：五个动作一律返回
// ErrBackendUnsupported —— 有类型、没能力，比「类型不存在所以调用方要自己判平台」诚实。
type NsjailBackend struct {
	// Path — nsjail 可执行文件路径/名字（空 = 按 PATH 找 "nsjail"）。
	Path string
	// BaseDir — 宿主侧茧根父目录（空 = 不落盘，Instance.Root 为空）。
	BaseDir string
	// Chroot — 茧的根文件系统（nsjail 的 --chroot 目标；默认 "/"）。
	//
	// ★ 实测（真 nsjail 3.x）：不给 --chroot 时 nsjail 只挂 /proc，**argv[0] 在茧内根本找不到**
	// （execve: No such file or directory，nsjail 退 255）。所以这不是可选项。
	// 缺省 "/" = 「不切换 rootfs」，隔离只剩 namespace + cgroup + rlimit；
	// 真镜像根（镜像落盘/overlay）要由上层把路径塞进来 —— 见文件头撞约束 Ⅰ。
	Chroot string
	// CgroupMount — cgroup 挂载点（默认 /sys/fs/cgroup）。
	//
	// ★ 这里只存**挂载点**，不存「每个茧一个目录」：读 nsjail 源码（cgroup2.cc）确认，
	// v2 模式下每座茧的 cgroup 固定是 `<mount>/NSJAIL.<pid>`，`--cgroup_mem_parent`
	// 在 v2 下**被忽略**（那是 v1 的路径拼法）。所以本层不自己去建 per-茧 目录，
	// 而是按 nsjail 的真实布局去写 cgroup.kill（见 killTree / removeCgroupDirs）。
	// 空串 ⇒ 不碰 cgroup.kill，超时只走 killpg 兜底。
	CgroupMount string
	// KillGrace — 超时后 SIGTERM → 宽限 → SIGKILL 的宽限（0 = 直接 SIGKILL）。
	KillGrace time.Duration
	// HostNetwork — 是否让茧**共享宿主 netns**（传 `--disable_clone_newnet`）。
	//
	// ★ 缺省 false（nsjail 自己建新 netns ⇒ 茧内只有 lo、出不了网，这是「运行期断外网」的
	//   唯一现成实现）。置 true 时茧与宿主同一网络栈：茧内监听 127.0.0.1:<port> 就是宿主
	//   127.0.0.1:<port>（⇒ 不需要也不存在端口映射），代价是**茧能出网**。
	//   两边都要就得接 DNAT/代理，本件未接（见文件头撞约束 Ⅰb）。
	HostNetwork bool

	mu     sync.Mutex
	nextID int
	insts  map[string]*nsjailInstance
}

// nsjailInstance — 一个茧的后端记账（不落契约副本：spec 由调用方持有）。
type nsjailInstance struct {
	id     string
	root   string
	spec   *Spec
	mounts map[string]nsjailMount
	gone   bool
	pid    int
	// pids — 当前还在跑的各次 exec 主进程（宿主 pid 集合）。
	// 用途：`open` 之后把这份 pid 表落进 meta —— 起服务的那个 CLI 进程会退出，
	// 收尾的是**另一个**进程（见 ReapPersisted）。
	pids map[int]struct{}
}

// nsjailMount — 一次挂载：宿主路径 + 茧内路径 + 读写位（同 inner 不许挂两次到不同源）。
type nsjailMount struct {
	Host  string
	Inner string
	RO    bool
}

// NewNsjailBackend — 造一个 nsjail 后端（不落盘；cgroup.kill 走缺省挂载点）。
//
// 落盘面/cgroup 面由**字段**给（BaseDir / CgroupMount）：这两项是部署参数，不是后端的
// 身份；构造器保持无参，免得每个调用点都要先想清楚路径再能造出一个「空后端」。
func NewNsjailBackend() *NsjailBackend {
	return &NsjailBackend{
		Path:        nsjailBinaryDefault,
		Chroot:      nsjailDefaultChroot,
		CgroupMount: nsjailDefaultCgroupMount,
		KillGrace:   nsjailDefaultKillGrace,
		insts:       make(map[string]*nsjailInstance),
	}
}

// ── pid 归属校验（ReapPersisted 的安全前提）───────────────────────────

// nsjailPidIsOurs — /proc/<pid>/cmdline 的第一个参数是不是「本后端那个 nsjail」。
//
// ★ 为什么必须验：`close <id>` 只凭 meta 里记的 pid 就去杀，等于把「meta 被改过 /
// pid 号被复用」两条都当成不可能。验法是读 cmdline（同 uid 可读），看 argv[0] 的
// basename 是否等于本后端可执行文件名 —— 这不是密码学证据，但足以挡住「杀手无寸铁的
// 无关进程」这唯一的灾难后果。
func nsjailPidIsOurs(pid int, bin string) bool {
	if pid <= 0 {
		return false
	}
	if bin == "" {
		bin = nsjailBinaryDefault
	}
	want := filepath.Base(bin)
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	if len(data) == 0 {
		return false
	}
	arg0 := strings.SplitN(string(data), "\x00", 2)[0]
	return filepath.Base(arg0) == want
}

// nsjailPidAlive — pid 还活着（僵尸也算活着：等它被回收）。
func nsjailPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat("/proc/" + strconv.Itoa(pid))
	return err == nil
}

// Name — 后端名（Instance.Backend 同字面量）。
func (b *NsjailBackend) Name() string { return nsjailBackendName }

// ── 纯函数：参数拼装（第①格测试的对象）───────────────────────────────

// nsjailArgs — 按 spec 的资源/隔离面拼 nsjail 旗标（**纯函数**：同一 spec 恒等输出）。
//
// 顺序固定（测试与日志都要能逐字比）。nil spec 只给「无资源限额」的那几条，不 panic。
func nsjailArgs(spec *Spec) []string {
	args := []string{
		// 一次性模式：一次 exec 起一个进程，跑完即退（nsjail 的缺省也是 o；写出来不靠缺省）。
		"--mode", "o",
		// nsjail 自己的墙钟上限缺省 600s ⇒ 会偷偷杀掉长构建；超时只认 ExecOpts.Timeout。
		"--time_limit", "0",
		// 把 cmd.Env（契约 env + 调用方 env，见 nsjailEnv）交给子进程；
		// nsjail 缺省会清空环境，不清就会连 PATH 都没有。
		"--keep_env",
	}
	// ★ 有意为空的三条（传了就走反方向，见文件头）：--iface_no_lo / --disable_proc /
	//   --disable_clone_newnet。

	if spec == nil {
		return append(args, "--rlimit_nofile", strconv.Itoa(nsjailDefaultNoFile))
	}

	res := spec.Resources

	// CPU：核数（向上取整，至少 1）+ 每秒 CPU 毫秒数（cgroup cpu.max 的语义）。
	if cpus := nsjailMaxCPUs(res.CPU); cpus > 0 {
		args = append(args, "--max_cpus", strconv.Itoa(cpus))
	}
	if ms := nsjailCPUMillis(res.CPU); ms > 0 {
		args = append(args, "--cgroup_cpu_ms_per_sec", strconv.Itoa(ms))
	}

	// 内存：cgroup 硬上限（字节）+ 地址空间 rlimit（MB）。
	// ★ 两个单位不同是 nsjail 的既定事实：cgroup_mem_max 收字节，rlimit_as 收 MB。
	if mb := nsjailMemMB(res.MemGB); mb > 0 {
		args = append(args,
			"--cgroup_mem_max", strconv.FormatInt(nsjailMemBytes(res.MemGB), 10),
			// cgroup v2 是统一层级（现代发行版缺省）⇒ 不打开检测，nsjail 会去 v1 的
			// /sys/fs/cgroup/memory 找路径，限额**静默不生效**。这条不是优化，是生效前提。
			"--detect_cgroupv2",
			"--rlimit_as", strconv.Itoa(mb),
		)
	}

	args = append(args, "--rlimit_nofile", strconv.Itoa(nsjailDefaultNoFile))
	return args
}

// nsjailMemBytes — GB → 字节（cgroup_mem_max 的单位）。
func nsjailMemBytes(gb float64) int64 {
	if gb <= 0 {
		return 0
	}
	return int64(math.Round(gb * (1 << 30)))
}

// nsjailMemMB — GB → MB（rlimit_as 的单位；整数向下取整到 MB，不小于 1）。
func nsjailMemMB(gb float64) int {
	if gb <= 0 {
		return 0
	}
	mb := int(math.Round(gb * 1024))
	if mb < 1 {
		mb = 1
	}
	return mb
}

// nsjailCPUMillis — 核数 → 每秒可用 CPU 毫秒数（cgroup_cpu_ms_per_sec）。
func nsjailCPUMillis(cpu float64) int {
	if cpu <= 0 {
		return 0
	}
	return int(math.Round(cpu * 1000))
}

// nsjailMaxCPUs — 核数 → --max_cpus（向上取整；0/负 ⇒ 0 = 不限）。
func nsjailMaxCPUs(cpu float64) int {
	if cpu <= 0 {
		return 0
	}
	n := int(math.Ceil(cpu))
	if n < 1 {
		n = 1
	}
	return n
}

// nsjailMountArgs — 挂载表 → --bindmount_ro / --bindmount 对（按 inner 升序，输出稳定）。
func nsjailMountArgs(mounts map[string]nsjailMount) []string {
	if len(mounts) == 0 {
		return nil
	}
	inners := make([]string, 0, len(mounts))
	for in := range mounts {
		inners = append(inners, in)
	}
	sort.Strings(inners)

	out := make([]string, 0, 2*len(inners))
	for _, in := range inners {
		m := mounts[in]
		flag := "--bindmount"
		if m.RO {
			flag = "--bindmount_ro"
		}
		out = append(out, flag, m.Host+":"+m.Inner)
	}
	return out
}

// nsjailCgroupPaths — 一座茧可能的 cgroup 目录（按 nsjail 源码的真实布局，两种层级都给）：
//
//	v2（cgroup2.cc: getCgroupPath）  <mount>/NSJAIL.<pid>
//	v1（cgroup.cc: cgroup_mem_parent）<mount>/NSJAIL/NSJAIL.<pid>
//
// ★ 不猜「这台机器是 v1 还是 v2」：两条都试，存在的才动手（见 killTree / removeCgroupDirs）。
func nsjailCgroupPaths(mount string, pid int) []string {
	if mount == "" || pid <= 0 {
		return nil
	}
	name := "NSJAIL." + strconv.Itoa(pid)
	return []string{
		filepath.Join(mount, name),
		filepath.Join(mount, "NSJAIL", name),
	}
}

// removeCgroupDirs — 按上面的布局补删 cgroup 目录（Destroy 用）；清不掉的作为错误返回。
//
// 放这里而不是 linux 文件：它只用 os.Remove，**平台无关**，而 linux 记账测试要在
// 本机能编译（否则那份测试连类型都过不了）。
func removeCgroupDirs(cgroupMount string, pid int) []error {
	var errs []error
	for _, dir := range nsjailCgroupPaths(cgroupMount, pid) {
		// cgroupfs 上 rmdir 只在空时成功：里面还有活进程 ⇒ EBUSY/ENOTEMPTY，如实报出来
		// （「删不掉」本身就是一条事实：说明这棵树没死透）。
		if err := removeCgroupDir(dir); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// removeCgroupDir — 删一个 cgroup 目录（不存在 = 已经收过，不算错）。
//
// ★ 为什么除了「按 pid 拼路径」还要单开这一条：cgroup v2 下目录名是**茧内 init 的宿主 pid**，
// 它比我们手里的 nsjail pid 大一两号（实测 NSJAIL.73 不存在、真的是 NSJAIL.74）。
// 只按自己的 pid 拼路径会漏掉真正的目录 —— 于是每座茧都在 /sys/fs/cgroup 留一个空壳。
func removeCgroupDir(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cocoon: 清 cgroup 目录 %s 失败: %w", dir, err)
	}
	return nil
}

// nsjailReapEmptyCgroups — 收掉「进程已经不在了、里面也没人」的 `NSJAIL.<pid>` 空壳。
//
// ★ 为什么需要它（实测）：cgroup v2 的目录名是**茧内 init 的宿主 pid**，而我们在收尾时手里
// 只有 nsjail 的 pid；树一死，`/proc` 里的父链也没了 ⇒ 事后无法把「这个目录」和「这座茧」
// 对上号。于是每座茧都会在 `/sys/fs/cgroup` 留一个空目录（实测：三轮集成后攒了 3 个）。
//
// 判据只有两条，且都可证：
//
//	① 目录名里的 pid 在 /proc 里**不存在**（还活着的一律不碰 —— 那是别人的茧正在建）；
//	② `cgroup.procs` 为空（还有进程的，即使父链断了也不动 —— 那说明没死透，该报不该删）。
//
// 不猜、不按时间、不按前缀通配全部删掉：删掉的只能是「已经确定什么都装不下的空壳」。
func nsjailReapEmptyCgroups(mount string) []error {
	if mount == "" {
		return nil
	}
	var errs []error
	for _, base := range []string{mount, filepath.Join(mount, "NSJAIL")} {
		ents, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "NSJAIL.") {
				continue
			}
			pid, perr := strconv.Atoi(strings.TrimPrefix(e.Name(), "NSJAIL."))
			if perr != nil || pid <= 0 {
				continue
			}
			if nsjailPidAlive(pid) {
				continue
			}
			dir := filepath.Join(base, e.Name())
			if procs, rerr := os.ReadFile(filepath.Join(dir, "cgroup.procs")); rerr == nil &&
				strings.TrimSpace(string(procs)) != "" {
				continue
			}
			if err := removeCgroupDir(dir); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// nsjailJailCgroups — 找出「确实挂在我们这个 nsjail 进程下面」的茧 cgroup 目录。
//
// ★ 为什么不能只按 pid 拼：实测（cgroup v2 + nsjail 3.x）目录名是**茧内 init 的宿主 pid**，
// 它比我们手里的直接子进程（nsjail）还低两层。按 pid 拼会拼到一个不存在的路径，
// cgroup.kill 静默失效 ⇒「整树回收」退化成只有 killpg，而 killpg 抓不到 setsid 跑掉的孙进程。
//
// 做法：扫 <mount>/NSJAIL.*（v2）与 <mount>/NSJAIL/NSJAIL.*（v1），读每份 cgroup.procs，
// 只要有某个 pid 的父链能走到 nsjailPID，这个目录就算本茧的。
//
// ★ 必须在「树还活着」时调用：nsjail 一死，茧内进程的父链就接到 init，认不出来了
// （所以 Exec 是先解析、再发信号，见那里的注释）。
func nsjailJailCgroups(mount string, nsjailPID int) []string {
	if mount == "" || nsjailPID <= 0 {
		return nil
	}
	var dirs []string
	for _, base := range []string{mount, filepath.Join(mount, "NSJAIL")} {
		ents, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "NSJAIL.") {
				continue
			}
			dir := filepath.Join(base, e.Name())
			if nsjailCgroupHasAncestor(dir, nsjailPID) {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

// nsjailCgroupHasAncestor — 该 cgroup 里是否有进程的父链能走到 ancestor。
func nsjailCgroupHasAncestor(dir string, ancestor int) bool {
	data, err := os.ReadFile(filepath.Join(dir, "cgroup.procs"))
	if err != nil {
		return false
	}
	for _, f := range strings.Fields(string(data)) {
		pid, err := strconv.Atoi(f)
		if err != nil || pid <= 0 {
			continue
		}
		if nsjailHasAncestor(pid, ancestor) {
			return true
		}
	}
	return false
}

// nsjailHasAncestor — pid 的父链上是否有 ancestor（深度上限防 /proc 里出现环）。
func nsjailHasAncestor(pid, ancestor int) bool {
	for depth := 0; depth < 64 && pid > 1; depth++ {
		if pid == ancestor {
			return true
		}
		ppid, ok := nsjailPPID(pid)
		if !ok || ppid == pid || ppid <= 0 {
			return false
		}
		pid = ppid
	}
	return false
}

// nsjailPPID — /proc/<pid>/stat 的第 4 字段（ppid）。
// ★ 不能整行按空格切：第 2 字段 comm 允许含空格与括号 ⇒ 从**最后一个** ')' 之后切。
func nsjailPPID(pid int) (int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	s := string(data)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return 0, false
	}
	fields := strings.Fields(s[i+2:]) // fields[0]=state(3) fields[1]=ppid(4)
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return ppid, true
}

// nsjailExecArgv — 一次 exec 的完整 argv（nsjail 旗标 + 挂载 + 工作目录 + `--` + 用户 argv）。
//
// ★ `--` 之前的一切都由本层生成，用户 argv 只许出现在 `--` 之后：这样「茧内命令」永远
// 不会被误认成 nsjail 旗标（否则一条 `--bindmount /:/` 就能把茧的挂载面改掉）。
func nsjailExecArgv(spec *Spec, mounts map[string]nsjailMount, chroot, dir string, argv []string) []string {
	out := nsjailArgs(spec)
	// ★ 必须有 --chroot：没它 nsjail 只挂 /proc，茧内连 /bin/sh 都找不到（实测退 255）。
	if chroot != "" {
		out = append(out, "--chroot", chroot)
	}
	out = append(out, nsjailMountArgs(mounts)...)
	if dir != "" {
		out = append(out, "--cwd", dir)
	}
	out = append(out, "--")
	return append(out, argv...)
}

// nsjailWithHostNet — 在 `--` **之前**插入 `--disable_clone_newnet`（缺了 `--` 就退化成追加，
// 那种形态会把旗标塞进用户 argv —— 直接返回原样，让调用方看见「没生效」而不是悄悄改语义）。
func nsjailWithHostNet(args []string) []string {
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		return args
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, args[:sep]...)
	out = append(out, "--disable_clone_newnet")
	out = append(out, args[sep:]...)
	return out
}

// nsjailJailPATH — 茧内 PATH（契约 [环境].PATH 优先，否则最小默认）。
func nsjailJailPATH(spec *Spec) string {
	if spec != nil {
		if p := strings.TrimSpace(spec.Env["PATH"]); p != "" {
			return p
		}
	}
	return nsjailDefaultPATH
}

// nsjailResolveArgv0 — argv[0] 是**裸命令名**时按茧内 PATH 解析成绝对路径。
//
// ★ 为什么必须有这一条（实测，不是猜）：nsjail 对 argv[0] 直接 execve，**不走 PATH**
// —— `entry = ["python3", ...]` 会得到 `execve('python3') failed: No such file or directory`、
// nsjail 退 255（日志在茧的 logs/<服务>.log 里）。契约里 Backend 的注释本来就写了
// 「argv[0] 按 PATH 解析由后端定」，所以解析责任在我们这边。
//
// ★ 只在「没换 rootfs」（chroot 为空或 "/"）时做：那时宿主路径 == 茧内路径，解析可证。
// 换真镜像根之后宿主看不到那份 rootfs，就得在茧内解析 —— 本件未接，此时**拒**，
// 不猜一个宿主上的同名路径塞进去（那会把「宿主的东西」当成「茧内的东西」跑）。
func nsjailResolveArgv0(argv []string, jailPATH, chroot string) ([]string, error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, fmt.Errorf("%w: argv 为空或 argv[0] 为空", ErrBadArgv)
	}
	a0 := argv[0]
	if strings.Contains(a0, "/") {
		return argv, nil
	}
	if chroot != "" && chroot != "/" {
		return nil, fmt.Errorf("%w: argv[0]=%q 是裸命令名，而本后端未在该 rootfs 内解析 PATH（chroot=%q）⇒ 请写茧内绝对路径",
			ErrBadArgv, a0, chroot)
	}
	for _, dir := range strings.Split(jailPATH, ":") {
		if dir == "" || !strings.HasPrefix(dir, "/") {
			continue
		}
		p := filepath.Join(dir, a0)
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		out := append([]string(nil), argv...)
		out[0] = p
		return out, nil
	}
	return nil, fmt.Errorf("%w: argv[0]=%q 在茧内 PATH=%q 里找不到可执行文件（nsjail 不走 PATH，本层替它解析）",
		ErrBadArgv, a0, jailPATH)
}

// nsjailEnv — 交给子进程的环境：**不继承宿主环境**（宿主可能带密钥），
// 只由「最小 PATH + 契约 [环境] + 调用方 env」合成，键序固定。
func nsjailEnv(spec *Spec, extra map[string]string) []string {
	merged := map[string]string{"PATH": nsjailDefaultPATH}
	if spec != nil {
		for k, v := range spec.Env {
			merged[k] = v
		}
	}
	for k, v := range extra {
		merged[k] = v
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+merged[k])
	}
	return out
}

// ── 纯函数：结果拼装（第③格测试的对象）───────────────────────────────

// nsjailSignalNames — 常见信号号 → `SIG*` 名（exec 结果的 Signal 字段要过
// validSignalName 的大写形态校验；`syscall.Signal.String()` 给的是 "killed" 这种小写词，
// 直接用会被 Validate 拒掉）。
var nsjailSignalNames = map[int]string{
	1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 5: "SIGTRAP", 6: "SIGABRT",
	7: "SIGBUS", 8: "SIGFPE", 9: "SIGKILL", 10: "SIGUSR1", 11: "SIGSEGV", 12: "SIGUSR2",
	13: "SIGPIPE", 14: "SIGALRM", 15: "SIGTERM", 16: "SIGSTKFLT", 17: "SIGCHLD",
	18: "SIGCONT", 19: "SIGSTOP", 20: "SIGTSTP", 21: "SIGTTIN", 22: "SIGTTOU",
	23: "SIGURG", 24: "SIGXCPU", 25: "SIGXFSZ", 26: "SIGVTALRM", 27: "SIGPROF",
	28: "SIGWINCH", 29: "SIGIO", 30: "SIGPWR", 31: "SIGSYS",
}

// nsjailSignalName — 信号号 → 名；非常见号退回 `SIG<n>`（合法形态，且不丢信息）。
func nsjailSignalName(sig int) string {
	if n, ok := nsjailSignalNames[sig]; ok {
		return n
	}
	if sig >= 34 && sig <= 64 { // 实时信号：Linux 34..64 对应 SIGRTMIN..SIGRTMAX
		return "SIGRT" + strconv.Itoa(sig-34)
	}
	return "SIG" + strconv.Itoa(sig)
}

// nsjailResult — 把「退出码 / 信号号 / 是否超时」三位事实拼成 ExecResult，**返回前过 Validate**。
//
// 归一规则（与 backend.go 的契约一致，且只在这一处实现，linux 实现与本机测试共用）：
//
//	超时       ⇒ TimedOut=true, RC=124( RCTimeout ), Signal=""（超时压过退出码/信号）
//	被信号杀   ⇒ Signal=SIGxxx, RC=128+信号号
//	正常退出   ⇒ RC=真实退出码，Signal=""
func nsjailResult(exitCode, signum int, timedOut bool, stdout, stderr []byte, ms int64) (ExecResult, error) {
	res := ExecResult{
		Stdout: append([]byte(nil), stdout...),
		Stderr: append([]byte(nil), stderr...),
		MS:     ms,
	}
	switch {
	case timedOut:
		res.RC, res.Signal, res.TimedOut = RCTimeout, "", true
	case signum > 0:
		res.Signal = nsjailSignalName(signum)
		res.RC = RCSignaled(signum)
	default:
		res.RC = exitCode
	}
	if err := res.Validate(); err != nil {
		return ExecResult{}, fmt.Errorf("cocoon: nsjail 后端拼出的结果自检失败: %w", err)
	}
	return res, nil
}
