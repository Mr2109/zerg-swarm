package cocoon

// 后端契约（v2 §F 的 T3 / v3 §F 平台差异表）。
//
// 这一层只管「起、跑、挂、露、毁」五件事。所有**判定**都在控制面：
//   · 身份认证（v2 §A）：后端只报告「这端点的 pid 是谁」，拒不拒由控制面定；
//   · 端口策略：宿主端口只许回环绑定（DefaultBind）；
//   · 契约校验：只认宿主侧那份 cocoon.toml（v2 §B）。
// 后端不许：读茧内的 cocoon.toml、在茧根内跑解释器/git/make、跟符号链接删东西。

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ── 五动作接口 ────────────────────────────────────────────────────────

// Backend — 虫茧后端（nsjail / VZ / HCS / 假后端各实现一份）。
//
// ★ 不变量（四条，测试必须打）：
//  1. 五动作的顺序：Provision → (Mount|Expose)* → Exec* → Destroy；Destroy 幂等。
//  2. Exec 的 RC 只能是**进程真实退出码**或**系统化值**（124 超时 / 128+信号号），
//     绝不许把失败/超时归一成 0（见 ExecResult.Validate）。
//  3. 后端必须按 Instance.ID 校验归属，**不得**信任调用方自行拼出来的 Instance。
//  4. ctx 到期即返回 ctx.Err()；不得留下起的进程/占的端口。
type Backend interface {
	// Provision — 按契约起一个茧，返回句柄（id 由后端分配，全局唯一）。
	// 不在这里跑服务；服务由控制面按 Spec.Order 逐个 Exec 起来。
	Provision(ctx context.Context, spec *Spec) (Instance, error)

	// Exec — 在茧内执行 argv（argv[0] 按 PATH 解析由后端定，不经过宿主 shell）。
	// 超时/被信号杀都不算后端故障：err==nil，事实写在 ExecResult 里（TimedOut / Signal / RC）。
	// 只有「结果拿不到」（实例不在、ctx 取消、后端自身坏）才返回 err。
	Exec(ctx context.Context, inst Instance, argv []string, opts ExecOpts) (ExecResult, error)

	// Mount — 把宿主路径挂进茧内；ro=true 为只读。同一 innerPath 不可挂两次到不同源。
	Mount(ctx context.Context, inst Instance, hostPath, innerPath string, ro bool) error

	// Expose — 把茧内 innerPort 露到宿主 bind 上，返回宿主端口。
	// bind 为空按 DefaultBind；非回环地址一律拒（不许默认暴露到网卡）。
	Expose(ctx context.Context, inst Instance, innerPort int, bind string) (hostPort int, err error)

	// Destroy — 收进程树、收端口、删实例。**幂等**：已销毁的再调返回 nil。
	Destroy(ctx context.Context, inst Instance) error
}

// CapabilityReporter — 可选面：后端自报能力（`cocoon inspect` 打印的就是它）。
//
// ★ 为什么单开门而不是塞进 Backend：VZ/HCS 那两个后端还没接，硬塞进去等于逼它们
// 先写一份「假声明」；门开着，没实现的调用方就退回**平台表**（`PlatformCapabilities`），
// 而不是自己手搓一份。
type CapabilityReporter interface {
	// Capabilities — 本后端在本平台能给什么；给不了就返回 ErrBackendUnsupported
	// （**不许**返回一张全 false 的表：那会被读成「后端在，只是弱」）。
	Capabilities() (Capabilities, error)
}

// PIDReporter — 可选面：该实例当前真在跑的 exec 主进程 pid（宿主侧）。
//
// 用途只有一个：把「后端签发的 pid」落进 meta，好让**另一个进程**的 `close` 能收干净。
// 空实现表示该后端不起进程（假后端）⇒ 调用方不落 pid。
type PIDReporter interface {
	// InstancePIDs — 该实例当前还活着的 exec 主进程 pid（升序；后端不在/已销毁 ⇒ nil）。
	InstancePIDs(inst Instance) []int
}

// DetachedProbe — 可选面：**后台档**（ExecOpts.Detach）起的进程现在还在不在。
//
// ★ 为什么必须有这一面：后台档的 Exec 只报「进程已签出」，而 `cocoon open` 不许把
// 「Start 没报错」当「服务在跑」（T3b 的假绿就是这么来的）。这一面给命令面一个
// **可判活**的读数：活着 / 已退（带退出观测）。没有它，「验活」就只能靠猜。
//
// ★ 实现要求（两条都直接决定真假）：
//
//	① 僵尸不算活着：父进程还没 wait 掉的孩子在 /proc 里还在，但已经什么都不做了 ——
//	   判活必须看进程状态（Linux `/proc/<pid>/stat` 的 state，Z/X 一律算死）。
//	② 不是本进程起的 pid（`close` 场景 / CLI 早退场）只许按「/proc 在不在且非僵尸」答，
//	   ✗ 不许猜退出码；拿不到真退出码时如实返回 RC=1 而不是 0（0 会被读成「正常完成」）。
type DetachedProbe interface {
	// DetachedStatus — 后台档进程 pid 的现读状态。
	//
	//	活着   ⇒ alive=true,  exited=false
	//	已退   ⇒ alive=false, exited=true, res=退出观测（拿不到真退出码时 RC=1）
	//	都不是 ⇒ 不可能（实现必须给一个确定的答案；拿不准按「已退」报）
	DetachedStatus(pid int) (alive bool, exited bool, res ExecResult)
}

// HandleReaper — 可选面：**按盘上句柄**收尾（跨进程 `close` 的唯一入口）。
//
// ★ 与 Destroy 的分工：Destroy 认的是「我内存里记过这一号」，跨进程时那份记账不存在；
// ReapPersisted 认的是「控制面把**后端自己签发的**句柄（id + pid）原样存进了 meta」。
// 因此实现必须自己验明 pid 归属（本后端的可执行文件、活的进程），**不许**拿调用方
// 拼出来的 pid 直接下刀 —— 否则一句 `close <id>` 就能借后端之手杀无关进程。
type HandleReaper interface {
	// ReapPersisted — 收掉 inst 对应的 pid 树 + cgroup + 茧根。幂等（收过再收返回 nil）。
	// 空 pids 且茧根不存在 ⇒ nil。
	ReapPersisted(ctx context.Context, inst Instance, pids []int) error
}

// CocoonDirSetter — 可选面：「茧目录」接线口（命令面把一个宿主目录交给后端当落点父目录）。
//
// ★ 为什么单开门：落点（NsjailBackend.BaseDir）是**部署参数**，不是后端的身份，所以
// 不进 Backend 五动作的签名；但命令面必须在 Provision 之前把它设上，否则
// ① 后端的 overlay/upper/merged 与茧内 /tmp 无处可落 ⇒ 给了 rootfs 只能报错；
// ② 实例根落不到茧目录里 ⇒ `close --purge` 删茧目录时收不掉这一层。
//
// ★ 语义（测试要打的三条）：
//
//	① 给的是**茧目录本身**（真源根下的 `<id>/`），不是它的父目录：后端自己的实例根是
//	   `<该目录>/<后端签发的 id>`（见 NsjailBackend.Provision）⇒ 一座茧的全部产物
//	   （upper / work / merged / tmp）都在这棵下面，删茧目录时一次收干净、也不碰兄弟茧。
//	② 目录**必须已经存在**：调用方先建目录再传（本方法只校验，不建盘）。
//	③ 空串 / 相对路径 / 不存在 / 不是目录 ⇒ 错。✗ 不许悄悄当成「没给」：那会让
//	   「契约给了 rootfs」退回宿主根，正是本层要消灭的静默降级。
type CocoonDirSetter interface {
	// SetCocoonDir — 记下这座茧的宿主目录（幂等：同一个值设几次都一样）。
	//
	// ★ 形态限定（如实说）：这是「一个进程同时只管一座茧」的接法 —— 命令面是
	// 一次一茧的一次性进程，所以够了。要在同一进程里**并发**开多座茧，就得把它改成
	// Provision 的入参（每次调用带自己的茧目录），那是接口变更，本版未做。
	SetCocoonDir(dir string) error
}

// Instance — 后端签发的不透明句柄。
//
// 只有后端能造出有意义的 ID；调用方只许把它原样传回，不许自己拼。
type Instance struct {
	// ID — 实例 id（后端分配，全局唯一；也是目录名/寻址键）。
	ID string
	// Backend — 后端名（"fake"/"nsjail"/"vz"/"hcs"），只用于诊断与审计。
	Backend string
	// Root — 宿主侧茧根；空表示该后端不落盘（如假后端）。
	Root string
	// PID — 茧主进程 pid；0 表示还没起（或已停）。
	PID int
}

// IsZero — 句柄是否为空（未 provision）。
func (i Instance) IsZero() bool { return i.ID == "" && i.Backend == "" && i.Root == "" && i.PID == 0 }

// String — 日志/报错用：`fake/fake-1(pid=1234)`。
func (i Instance) String() string {
	pid := ""
	if i.PID > 0 {
		pid = fmt.Sprintf("(pid=%d)", i.PID)
	}
	name := i.Backend
	if name == "" {
		name = "?"
	}
	id := i.ID
	if id == "" {
		id = "<空>"
	}
	return name + "/" + id + pid
}

// ── exec 入参/出参 ────────────────────────────────────────────────────

// ExecOpts — 一次 exec 的可选项（v2 §D 的 exec 契约）。
type ExecOpts struct {
	// Timeout — 硬超时；0 表示不设上限（调用方自己负责）。到期按
	// SIGTERM → 宽限 10s → SIGKILL 停（v2 §D）。
	Timeout time.Duration
	// TTY — 要不要分配伪终端（影响 stdout 的行缓冲与 ^C 语义）。
	TTY bool
	// Detach — **后台档**：起进程即返回，不等它退出（阻塞档 = 缺省）。
	//
	// ★ 存在理由（这条是「茧要活过 CLI」的唯一实现面）：`cocoon open` 是个一次性进程，
	// 而它起的服务要一直活着。阻塞档里服务 spawn 在**它自己的 goroutine** 里，main 一返回
	// 整棵进程树就被带走 —— 实测（T3b）三次真后端 open 全是 pid:0、无 logs/、无 nsjail 进程。
	// 后台档让子进程**自成会话**（setsid：进程组号 == pid），CLI 退出后它被 init 收养、继续跑；
	// 收尾由另一个进程的 `close` 按 meta 里的 pid 整组收（见 HandleReaper）。
	//
	// ★ 三条硬约束（实现方必须照做，否则等于换了个假绿）：
	//  ① 必须要求 LogPath 非空：管道读端随起它的进程一起消失 ⇒ 服务下一次写 stdout 就 EPIPE；
	//  ② 必须把**真进程 pid**放进 ExecResult.PID 并实现 DetachedProbe（命令面靠它验活）；
	//  ③ 返回不表示「服务起来了」，只表示「进程已签出」——「真在」由调用方验（探活/就绪）。
	Detach bool
	// Stdin — 一次性喂给进程的输入；nil 表示立即 EOF（不挂管道等输入）。
	Stdin []byte
	// Env — 追加/覆盖的环境变量（不与 Spec.Env 冲突时由控制面合并后传入）。
	Env map[string]string
	// Dir — 工作目录（茧内绝对路径）；空表示用镜像的默认 workdir。
	Dir string
	// LogPath — 把 stdout/stderr 重定向到宿主侧这个文件（追加 · 0644）；空 = 走内存管道。
	//
	// ★ 存在理由（不是优化）：长驻服务由 `cocoon open` 起，而 open 这个进程**会退出**。
	// 管道读端随它一起消失 ⇒ 服务下一次往 stdout/stderr 写就是 EPIPE/EPIPE 引发的死法。
	// 落文件后，服务在 open 退场后照样能写、也能被排障看见（日志在茧目录里）。
	// ★ 代价（如实说）：设了它，ExecResult.Stdout/Stderr 恒空（输出在文件里，不另留一份）。
	LogPath string
}

// Validate — 入参自检：键名/路径不合规会在 execve 那一刻才炸，且报错看不懂，所以提前拒。
func (o ExecOpts) Validate() error {
	if o.Timeout < 0 {
		return fmt.Errorf("cocoon: exec 超时不得为负，得到 %s", o.Timeout)
	}
	if o.Dir != "" && !path.IsAbs(o.Dir) {
		return fmt.Errorf("cocoon: exec 工作目录必须是绝对路径，得到 %q", o.Dir)
	}
	if o.LogPath != "" && !filepath.IsAbs(o.LogPath) {
		return fmt.Errorf("cocoon: exec 日志落点必须是宿主绝对路径，得到 %q", o.LogPath)
	}
	// ★ 后台档必须落文件：管道读端属于起它的那个进程，那个进程一退服务下一次写就 EPIPE
	// （见 ExecOpts.Detach 的硬约束①）。宁可在这里拒执，也不许静默退回管道。
	if o.Detach && o.LogPath == "" {
		return fmt.Errorf("cocoon: 后台档（Detach）必须给 LogPath —— 管道读端会随起进程一起消失，" +
			"服务下一次写 stdout/stderr 就是 EPIPE（宁拒执不静默降级）")
	}
	for k := range o.Env {
		if !specEnvKeyRe.MatchString(k) {
			return fmt.Errorf("cocoon: exec 环境变量名非法 %q（必须 [A-Za-z_][A-Za-z0-9_]*）", k)
		}
	}
	return nil
}

// 系统化 RC（真实退出码之外，只允许这几个来源）。
const (
	// RCSignalBase — 被信号杀死时的基值，约定 RC = RCSignalBase + 信号号（shell 惯例）。
	RCSignalBase = 128
	// RCTimeout — 超时被叫停时的 RC（抄 timeout(1) 的 124）。
	//
	// ★ TimedOut=true 时 RC **必须**是它：不许把超时混成「正常退出 0」，
	// 否则上层 `if rc == 0` 就把一次失败的启动当成成功。
	RCTimeout = 124
	// maxRC — 退出码上限（8 位；128+信号号也落在里面）。
	maxRC = 255
	// maxSignalRC — 信号号上限（Linux/macOS 都 ≤ 31；留余量到 64）。
	maxSignalRC = RCSignalBase + 64
)

// RCSignaled — 信号号 → 系统化 RC（后端拼结果时用；信号号取本平台的号）。
func RCSignaled(signum int) int { return RCSignalBase + signum }

// ExecResult — 一次 exec 的观测值（v2 §D：{rc, signal, stdout, stderr, ms, timed_out}）。
//
// RC 的三种合法形态（其余一律非法，见 Validate）：
//
//	正常退出   RC=0..255，Signal=""，TimedOut=false
//	被信号杀   Signal="SIGKILL" 之类，RC=128+信号号
//	超时       TimedOut=true，RC=RCTimeout(124)，Signal=""
type ExecResult struct {
	// RC — 真实退出码或系统化值（见上）。
	RC int
	// PID — 后端签出的进程 pid（**只有后台档**填；阻塞档为 0，因为进程已经退完了）。
	//
	// ★ 它进 meta（`pid` / `backend_pids`），是另一个进程的 `close` 能收干净的唯一依据。
	// 后台档不填它 ⇒ 调用方拿不到「谁的进程」，只能报成功却什么都收不掉。
	PID int
	// Signal — 杀死进程的信号名；"" 表示不是被信号杀的。
	Signal string
	// Stdout / Stderr — 已收到的输出（超时也要保留，不许丢）。
	Stdout []byte
	Stderr []byte
	// MS — 墙钟耗时（毫秒）。
	MS int64
	// TimedOut — 是否因 Timeout 被叫停（与 RC 是两件事：判超时看这个位）。
	TimedOut bool
}

// Success — 真正成功：RC=0、没吃信号、没超时。调用方判成败**只许**用它或 Err()，
// 不许写 `rc == 0`（那样会把「信号杀 + 128+n」之外的历史值放过去，也会漏 TimedOut）。
func (r ExecResult) Success() bool { return r.RC == 0 && r.Signal == "" && !r.TimedOut }

// Validate — 结果自检。★ 这是「RC 不许伪装成 0」那条契约的机器可判形态：
// 后端（以及假后端）返回前必须过一遍，否则上层只能靠猜。
func (r ExecResult) Validate() error {
	if r.RC < 0 || r.RC > maxRC {
		return fmt.Errorf("%w: RC=%d 越界（只允许 0..%d）", ErrBadExecResult, r.RC, maxRC)
	}
	if r.MS < 0 {
		return fmt.Errorf("%w: MS=%d 为负", ErrBadExecResult, r.MS)
	}
	if r.TimedOut {
		// 超时优先：它压过「退出码/信号」两说，且不许是 0。
		if r.RC != RCTimeout {
			return fmt.Errorf("%w: TimedOut=true 时 RC 必须是 %d，得到 %d（超时不得伪装成成功）",
				ErrBadExecResult, RCTimeout, r.RC)
		}
		if r.Signal != "" {
			return fmt.Errorf("%w: TimedOut=true 时 Signal 必须为空，得到 %q（超时只报 RC=%d）",
				ErrBadExecResult, r.Signal, RCTimeout)
		}
		return nil
	}
	if r.Signal == "" {
		return nil // 正常退出：RC 就是真实退出码，0 与非 0 都合法
	}
	if !validSignalName(r.Signal) {
		return fmt.Errorf("%w: 信号名非法 %q（约定 SIG* 大写形态）", ErrBadExecResult, r.Signal)
	}
	if r.RC <= RCSignalBase || r.RC > maxSignalRC {
		return fmt.Errorf("%w: 被 %s 杀死时 RC 必须是 %d+信号号（%d..%d），得到 %d",
			ErrBadExecResult, r.Signal, RCSignalBase, RCSignalBase+1, maxSignalRC, r.RC)
	}
	return nil
}

// validSignalName — 信号名形态：`SIG` + 大写字母/数字/下划线。
func validSignalName(s string) bool {
	if !strings.HasPrefix(s, "SIG") || len(s) == 3 {
		return false
	}
	for _, c := range s[3:] {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// ExitError — 把一次「不成功」的结果翻成错误（保留 RC/信号/超时三位事实）。
type ExitError struct {
	RC       int
	Signal   string
	TimedOut bool
	// cause — ErrExecTimeout / ErrExecSignaled / ErrExecNonZero，供 errors.Is 判。
	cause error
}

func (e *ExitError) Error() string {
	switch {
	case e.TimedOut:
		return fmt.Sprintf("cocoon: exec 超时（RC=%d）", e.RC)
	case e.Signal != "":
		return fmt.Sprintf("cocoon: exec 被 %s 杀死（RC=%d）", e.Signal, e.RC)
	default:
		return fmt.Sprintf("cocoon: exec 非零退出（RC=%d）", e.RC)
	}
}

// Unwrap — errors.Is(err, ErrExecTimeout/ErrExecSignaled/ErrExecNonZero) 可用。
func (e *ExitError) Unwrap() error { return e.cause }

// Err — 结果 → 错误：成功返回 nil；否则 *ExitError（可 errors.Is 细分）。
func (r ExecResult) Err() error {
	switch {
	case r.TimedOut:
		return &ExitError{RC: r.RC, TimedOut: true, cause: ErrExecTimeout}
	case r.Signal != "":
		return &ExitError{RC: r.RC, Signal: r.Signal, cause: ErrExecSignaled}
	case r.RC != 0:
		return &ExitError{RC: r.RC, cause: ErrExecNonZero}
	default:
		return nil
	}
}

// clone — 深拷一份（后端把结果交出去后再改自己的缓冲，不该影响调用方）。
func (r ExecResult) clone() ExecResult {
	out := r
	out.Stdout = append([]byte(nil), r.Stdout...)
	out.Stderr = append([]byte(nil), r.Stderr...)
	return out
}

// ── 能力声明（v3 §F 逐项对齐）────────────────────────────────────────

// PeerIdentity — 对端身份的**可证强度**（不是「有没有」，是「能不能防伪」）。
//
// v3 §A 定稿：授权只认一次性令牌；身份只用于归属 + 二次校验。所以 weak 不是致命，
// 但 weak ⇒ 禁止高权操作、只记审计、fail-closed（✗ 不降权放行）。
type PeerIdentity string

const (
	// PeerIdentityStrong — 内核给出的、抗 pid 复用的对端身份（Linux SO_PEERPIDFD/SO_PEERSEC）。
	PeerIdentityStrong PeerIdentity = "strong"
	// PeerIdentityWeak — 只有 VM 级/端点级身份，传输层无防伪（macOS VZ、Windows HvSocket）。
	PeerIdentityWeak PeerIdentity = "weak"
	// PeerIdentityNone — 拿不到任何对端身份（只能靠令牌）。
	PeerIdentityNone PeerIdentity = "none"
)

// Valid — 是不是已知强度。
func (p PeerIdentity) Valid() bool {
	return p == PeerIdentityStrong || p == PeerIdentityWeak || p == PeerIdentityNone
}

// PrivilegedOperationsAllowed — 只有 strong 才允许高权操作（v3 §A：weak ⇒ fail-closed）。
func (p PeerIdentity) PrivilegedOperationsAllowed() bool { return p == PeerIdentityStrong }

// 平台名（与 runtime.GOOS 同字面量，便于直接比对）。
const (
	PlatformLinux   = "linux"
	PlatformDarwin  = "darwin"
	PlatformWindows = "windows"
)

// Capabilities — 一个后端能做什么、弱在哪。编排层据此决定「能不能开这个功能」，
// 不许靠「平台像是支持」猜。
type Capabilities struct {
	// Platform — 平台名（linux/darwin/windows）。
	Platform string
	// Isolation — 隔离本体（nsjail / VZ VM / HCS）。
	Isolation string
	// PeerIdentity — 对端身份强度（strong/weak/none，见 v3 §A）。
	PeerIdentity PeerIdentity
	// PeerMechanism — 靠什么拿身份（写明「VM 级」「无防伪」这类限定，别让人误读成 strong）。
	PeerMechanism string
	// ProcessTreeKill — 有没有**内核级**整树回收。false = 只能 killpg 兜底，可能漏掉已 reparent 的孙进程。
	ProcessTreeKill bool
	// KillMechanism — 回收机制（cgroup.kill / kqueue+killpg / Job Object）。
	KillMechanism string
	// SymlinkNoFollow — 有没有内核强制的「不跟随符号链接」（openat2 NO_SYMLINKS / O_NOFOLLOW_ANY）。
	SymlinkNoFollow bool
	// SymlinkMechanism — 对应机制；没有就写「待补」。
	SymlinkMechanism string
	// CrossMountGuard — 有没有内核强制的「不跨挂载点」（RESOLVE_NO_XDEV）。false = 只能手动比 st_dev。
	CrossMountGuard bool
	// CrossMountMechanism — 对应机制；没有就写「待补」。
	CrossMountMechanism string
	// Expose — 有没有「茧内端口 → 宿主端口」的映射能力。
	//
	// ★ 这是控制面决定「要不要调 Expose」的**唯一**依据（control plane 不许按平台名猜）：
	//   false ⇒ 契约里的 [[端口]] 一律**跳过并说明**，不是「静默丢掉」也不是「假装映射成功」；
	//   同时 readiness 探测改直连（茧与调用方同网络栈时，茧内端口就是宿主端口）。
	//   Linux/nsjail = false（nsjail 只做 CLONE_NEWNET，不做 DNAT）。
	Expose bool
	// ExposeMechanism — 端口怎么露 / 为什么不露（false 时必须写清，空值等于「没说」）。
	ExposeMechanism string
	// Overlay — 能不能把「只读镜像根」变成「可写视图」（overlayfs：lower=rootfs 只读 ·
	// upper 可写 · work 同盘）。
	//
	// ★ 与 Expose 同口径：false 时必须写清为什么（OverlayMechanism）。false 不等于
	// 「不能换根」—— 换根（chroot）与可写层是两件事：只能 chroot 的后端仍可跑只读镜像。
	Overlay bool
	// OverlayMechanism — 怎么做 overlay / 为什么不做。
	OverlayMechanism string
}

// Validate — 声明自检：平台/隔离本体/回收机制都必须写清（空值等于「没说」，
// 上层会把「没说」误当「没问题」）。
func (c Capabilities) Validate() error {
	if c.Platform == "" {
		return errors.New("cocoon: 后端能力缺 Platform")
	}
	if c.Isolation == "" {
		return fmt.Errorf("cocoon: 后端能力（%s）缺 Isolation", c.Platform)
	}
	if !c.PeerIdentity.Valid() {
		return fmt.Errorf("cocoon: 后端能力（%s）PeerIdentity 非法 %q（strong|weak|none）",
			c.Platform, c.PeerIdentity)
	}
	if c.KillMechanism == "" {
		return fmt.Errorf("cocoon: 后端能力（%s）缺 KillMechanism（ProcessTreeKill=false 时必须写清兜底）", c.Platform)
	}
	if !c.Expose && c.ExposeMechanism == "" {
		return fmt.Errorf("cocoon: 后端能力（%s）Expose=false 但缺 ExposeMechanism（「不映射」与「忘了写」必须能分开）", c.Platform)
	}
	if !c.Overlay && c.OverlayMechanism == "" {
		return fmt.Errorf("cocoon: 后端能力（%s）Overlay=false 但缺 OverlayMechanism（「不做可写层」与「忘了写」必须能分开）", c.Platform)
	}
	return nil
}

// capabilitiesByPlatform — v3 §F 那张表的代码形态。**只许照表改，不许按「我觉得」改**。
var capabilitiesByPlatform = map[string]Capabilities{
	PlatformLinux: {
		Platform:            PlatformLinux,
		Isolation:           "nsjail",
		PeerIdentity:        PeerIdentityStrong,
		PeerMechanism:       "SO_PEERPIDFD / SO_PEERSEC",
		ProcessTreeKill:     true,
		KillMechanism:       "cgroup.kill",
		SymlinkNoFollow:     true,
		SymlinkMechanism:    "openat2 RESOLVE_NO_SYMLINKS",
		CrossMountGuard:     true,
		CrossMountMechanism: "openat2 RESOLVE_NO_XDEV",
		// nsjail 只做 CLONE_NEWNET 隔离，不做 DNAT：茧内监听的端口本来就是宿主端口。
		Expose:          false,
		ExposeMechanism: "无 DNAT（茧与调用方同宿主网络栈：本后端有意共享宿主 netns ⇒ 茧内端口即宿主端口，直连 127.0.0.1:<inner>；代价是茧能出网，未接）",
		// overlayfs：lower=镜像根（只读）· upper/work 落在茧目录（同盘）⇒ 茧内可写、
		// 写进去的新文件全部可在 upper 里逐件看见（判据就靠这一条）。
		Overlay:          true,
		OverlayMechanism: "overlayfs（lower=rootfs 只读 · upper/work 在茧目录；upper 的盘必须支持叠层，tmpfs/ext4/xfs 可以，overlay-on-overlay 不行）",
	},
	PlatformDarwin: {
		Platform:            PlatformDarwin,
		Isolation:           "VZ VM",
		PeerIdentity:        PeerIdentityWeak,
		PeerMechanism:       "VM 级（宿主看不到 guest pid）",
		ProcessTreeKill:     false,
		KillMechanism:       "kqueue NOTE_EXIT + killpg（无内核级等价物，弱于 cgroup.kill）",
		SymlinkNoFollow:     true,
		SymlinkMechanism:    "O_NOFOLLOW_ANY",
		CrossMountGuard:     false,
		CrossMountMechanism: "手动比 st_dev（无 NO_XDEV 等价物）",
		Expose:              false,
		ExposeMechanism:     "未接（VZ 后端未接：端口转发是 VZ 的活，本件不做）",
		Overlay:             false,
		OverlayMechanism:    "未接（VZ 后端未接：镜像根由 VM 自己的盘提供，宿主侧不做 overlay）",
	},
	PlatformWindows: {
		Platform:            PlatformWindows,
		Isolation:           "HCS(hyperv)",
		PeerIdentity:        PeerIdentityWeak,
		PeerMechanism:       "SOCKADDR_HV（传输层无防伪）",
		ProcessTreeKill:     true,
		KillMechanism:       "Job Object (KILL_ON_JOB_CLOSE)",
		SymlinkNoFollow:     false,
		SymlinkMechanism:    "待补",
		CrossMountGuard:     false,
		CrossMountMechanism: "待补",
		Expose:              false,
		ExposeMechanism:     "未接（HCS 后端未接：HvSocket 转发是 HCS 的活，本件不做）",
		Overlay:             false,
		OverlayMechanism:    "未接（HCS 后端未接：分层由 HCS 自己的 layer 提供，宿主侧不做 overlay）",
	},
}

// Platforms — 已声明能力的平台（固定顺序，便于报错/文档稳定）。
func Platforms() []string {
	return []string{PlatformLinux, PlatformDarwin, PlatformWindows}
}

// HostPlatform — 本机平台名（runtime.GOOS 与上面三个字面量同形，可直接比对）。
func HostPlatform() string { return runtime.GOOS }

// PlatformCapabilities — 按平台名取声明；未知平台返回 ok=false（不许给个「默认全 true」）。
func PlatformCapabilities(platform string) (Capabilities, bool) {
	c, ok := capabilitiesByPlatform[platform]
	return c, ok
}

// LinuxCapabilities / DarwinCapabilities / WindowsCapabilities — 三平台的显式入口
// （测试与编排层据此逐项断言；返回副本，调用方改了也不影响本表）。
func LinuxCapabilities() Capabilities   { return capabilitiesByPlatform[PlatformLinux] }
func DarwinCapabilities() Capabilities  { return capabilitiesByPlatform[PlatformDarwin] }
func WindowsCapabilities() Capabilities { return capabilitiesByPlatform[PlatformWindows] }

// ── 错误 ──────────────────────────────────────────────────────────────

var (
	// ErrNilSpec — Provision 收到 nil 契约。
	ErrNilSpec = errors.New("cocoon: 后端收到 nil 契约")
	// ErrBadArgv — exec 的 argv 为空或 argv[0] 为空。
	ErrBadArgv = errors.New("cocoon: exec argv 非法")
	// ErrBadExecResult — 后端给出的结果违反 RC/信号/超时契约（见 ExecResult.Validate）。
	ErrBadExecResult = errors.New("cocoon: exec 结果违反契约")
	// ErrInstanceNotFound — 实例不存在（id 从没 provision 过，或不属于该后端）。
	ErrInstanceNotFound = errors.New("cocoon: 茧实例不存在")
	// ErrInstanceGone — 实例已销毁（再次 exec/mount/expose 一律拒）。
	ErrInstanceGone = errors.New("cocoon: 茧实例已销毁")
	// ErrMountConflict — 同一 innerPath 被挂到不同源/不同读写模式。
	ErrMountConflict = errors.New("cocoon: 内挂载点冲突")
	// ErrBadMount — 挂载参数非法（空路径、相对路径）。
	ErrBadMount = errors.New("cocoon: 挂载参数非法")
	// ErrBadPort — 端口号越界（1..65535）。
	ErrBadPort = errors.New("cocoon: 端口越界")
	// ErrBindNotLoopback — Expose 的 bind 不是回环地址（不许默认暴露到网卡）。
	ErrBindNotLoopback = errors.New("cocoon: 端口只许绑回环")
	// ErrPortExhausted — 宿主端口号段用尽。
	ErrPortExhausted = errors.New("cocoon: 宿主端口分配用尽")
	// ErrExecTimeout — exec 超时（ExitError 的 cause）。
	ErrExecTimeout = errors.New("cocoon: exec 超时")
	// ErrExecSignaled — exec 被信号杀死（ExitError 的 cause）。
	ErrExecSignaled = errors.New("cocoon: exec 被信号杀死")
	// ErrExecNonZero — exec 非零退出（ExitError 的 cause）。
	ErrExecNonZero = errors.New("cocoon: exec 非零退出")
	// ErrBackendUnsupported — 该后端在本平台/本能力面上**不做**这件事。
	//
	// 两个来源：非 linux 上的 nsjail 后端（整个后端不可用）、Linux 后端的 Expose
	// （茧与调用方同栈，没有端口映射这回事）。★ 不许拿它当「降级成功」用：
	// 返回它就等于告诉调用方「这条链路不存在，别指望我」。
	ErrBackendUnsupported = errors.New("cocoon: 后端不支持该操作")
)
