// orchestrate.go — 多服务编排（B5 · 设计稿 v2 §D ＋ v3 §D）
//
// ★ v3 §D 那几行，逐条落实（抄什么 / 不抄什么，都写在代码形态里）：
//
//	抄 compose：condition 三态（started|healthy|completed）
//	    ⇒ 本文件认三态：started（已起、无判据）、healthy（readiness 探到 200）、
//	      completed（退出且 rc==0）。★ condition **不写进契约**（depends_on 仍是名字数组，
//	      契约面已定稿）：这里只固定「三态怎么判」，不新增字段。
//	抄 systemd：depends_on 拆成两条边 —— 顺序边（After）与存活边（BindsTo）
//	    ⇒ 顺序边：起 X 之前，X 的每个依赖必须已 healthy（未声明 readiness 的服务 = started 即算）。
//	      存活边：依赖事后「死」（非 0 / 信号 / 超时）⇒ 逆序级联停掉全部下游。
//	✗ 不抄 k8s readiness 当启动门
//	    ⇒ readiness 只判「就绪」，排序只走 depends_on（Spec.Order）。
//	✗ 不抄 supervisord startsecs
//	    ⇒ 没有「睡 N 秒算起来」这种判据。
//	自补：任一服务就绪超时 ⇒ 按启动**逆序**停掉本次已起的集合（四家都没有「多服务事务」）
//	    ⇒ Start 失败一律 rollback（逆序停 + Destroy 实例），停掉的名单挂进错误里。
//
// ★ 撞约束（不硬编，照实上报；都是 Backend 面不足，不是编排层想省事）：
//
//	Ⅰ. Backend 没有「非阻塞起服务」动作：Exec 阻塞到进程退出才返回。
//	    ⇒ 编排层自己 goroutine 化（defaultSpawner），并把「起进程」抽成 Spawner 缝，
//	      让测试用可控的 Proc 演「长驻 / 半路死」。真后端必须把 Exec 的 ctx 取消翻译成
//	      SIGTERM → 10s → SIGKILL，否则默认路径的停只能「取消 ctx，等它自己退」。
//	Ⅱ. Backend 没有「停某个服务」动作（只有 Destroy 整个实例）：停 = 取消该服务的 ctx，
//	    靠后端契约 4（ctx 到期即返回、不留进程/端口）兜底。「SIGTERM→宽限→SIGKILL」
//	    的宽限期**不在这一层**，在真后端的 Exec 实现里。
//	Ⅲ. 一个茧一个 Instance（Provision 的粒度）⇒「多服务事务」只能是「同一实例内起/停一组
//	    进程」，没有跨实例原子性可言。
//	Ⅳ. readiness.port 是**茧内**端口，而探测要打宿主回环口 ⇒ Start 会为它调一次 Expose
//	    （后端按 (inner,bind) 幂等）。运行期 net=none 不影响这条：探的是宿主映射口。
//	    OrchestrateOptions.ProbeAddr 可换 ⇒ 测试打 httptest/桩口，不出网。
//	Ⅴ. rc==0 的退出按 compose 的 completed 处理，**不**触发存活边（否则一次性的迁移/初始化
//	    服务会把整条链停掉）。只有非 0 / 信号 / 超时才判「死」。
//	Ⅵ. 顺序边只在「起的那一刻」判一次：起完之后依赖死掉靠存活边（异步 watcher）；
//	    两者之间有窗口——进程模型的固有窗口，不假装能消掉。
//	Ⅶ. 未声明 readiness 的服务只能「起即算就绪」：Backend 没有「进程还活着吗」的查询面
//	    （FakeBackend 的 PID 恒 0），所以这一层不假装能验活。
package cocoon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultProbeInterval — 就绪探测的重试间隔（就绪通常几十毫秒级，20ms 是「不漏拍也不打爆」的折中）。
	DefaultProbeInterval = 20 * time.Millisecond
	// DefaultStopTimeout — 停一个服务的上限（v2 §D：SIGTERM 后宽限 10s → SIGKILL，留 5s 余量）。
	DefaultStopTimeout = 15 * time.Second
	// probeHTTPTimeout — 单次 GET 的上限（不是 readiness.timeout：后者管整轮重试）。
	probeHTTPTimeout = 2 * time.Second
	// probeBodyDrain — 探测响应体最多读这么多（就绪响应体没人要，但不读会挡住连接复用）。
	probeBodyDrain = 4 << 10
)

// ── 错误 ──────────────────────────────────────────────────────────────

var (
	// ErrCycle — 服务依赖成环（解析器已拒；这里防御性再拒一次，防止有人手搓 Spec 绕过解析器）。
	ErrCycle = errors.New("cocoon: 服务依赖有环")
	// ErrDanglingDep — depends_on 指向不存在的服务。
	ErrDanglingDep = errors.New("cocoon: 依赖的服务不存在")
	// ErrBadOrder — Spec.Order 不是这份依赖图的合法拓扑序（缺服务/多服务/顺序反了）。
	ErrBadOrder = errors.New("cocoon: 启动顺序不是合法拓扑序")
	// ErrNoServices — 契约里一个服务都没有。
	ErrNoServices = errors.New("cocoon: 契约里没有服务")
	// ErrReadinessTimeout — 就绪超时（readiness.timeout 到期仍未 200 / 端口仍不可连）。
	ErrReadinessTimeout = errors.New("cocoon: 服务就绪超时")
	// ErrExitBeforeReady — 进程在就绪之前就退出了（起来就崩，不该白等到超时）。
	ErrExitBeforeReady = errors.New("cocoon: 服务在就绪前退出")
	// ErrDepNotReady — 要起 X 时，X 的某个依赖没就绪或已死。
	ErrDepNotReady = errors.New("cocoon: 依赖未就绪")
	// ErrLaunchFailed — 起进程本身失败（Spawner/后端报错）。
	ErrLaunchFailed = errors.New("cocoon: 起服务失败")
)

// OrchestrateError — 编排错误：点名「哪个服务、哪一步、为什么」，回滚时带上「停了谁」。
//
// 为什么不用裸 fmt.Errorf：就绪超时必须能让调用方分辨「超时 / 就绪前崩 / 依赖没了」，
// 且回滚名单必须机器可读（T3 要把它写进 meta 的日志索引里）。
type OrchestrateError struct {
	// Op — 失败在哪一步：plan / launch / ready / dep / rollback。
	Op string
	// Service — 涉事服务名（plan 阶段的环错误可能为空）。
	Service string
	// Msg — 人读的一句话。
	Msg string
	// Cause — 哨兵（ErrCycle / ErrReadinessTimeout / …），供 errors.Is。
	Cause error
	// Stopped — 本次已按**启动逆序**停掉的服务（回滚填；没回滚就是 nil）。
	Stopped []string
}

func (e *OrchestrateError) Error() string {
	var b strings.Builder
	b.WriteString("cocoon: 编排")
	if e.Op != "" {
		b.WriteString("[" + e.Op + "]")
	}
	if e.Service != "" {
		b.WriteString(fmt.Sprintf(" 服务 %q", e.Service))
	}
	if e.Msg != "" {
		b.WriteString(": " + e.Msg)
	}
	if len(e.Stopped) > 0 {
		fmt.Fprintf(&b, "；已按启动逆序停掉 %d 个：%s", len(e.Stopped), strings.Join(e.Stopped, ", "))
	}
	return b.String()
}

// Unwrap — errors.Is(err, ErrReadinessTimeout) 之类可用。
func (e *OrchestrateError) Unwrap() error { return e.Cause }

// ── 进程缝（Spawner / Proc）────────────────────────────────────────────

// Proc — 一个已起的服务进程。
//
// ★ Wait 只许被调一次（编排层起一个 goroutine 专门等）；Stop 允许多次（幂等）。
type Proc interface {
	// Wait — 阻塞到进程退出，返回观测值（被 Stop 叫停时返回的也是最终退出值）。
	Wait() ExecResult
	// Stop — 叫停该进程；ctx 到期前没停干净就返回 ctx.Err()。
	Stop(ctx context.Context) error
}

// Spawner — 「把一条 argv 起成长驻进程」的策略。
//
// 存在的唯一理由见头部撞约束 Ⅰ：Backend.Exec 是阻塞的，编排层要起 N 个长驻服务，
// 就必须自己 goroutine 化；而测试要演「半路死」，就必须能换掉这个实现。
type Spawner interface {
	Spawn(ctx context.Context, be Backend, inst Instance, svc ServiceSpec, opts ExecOpts) (Proc, error)
}

// SpawnFunc — 把函数适配成 Spawner（测试/上层自定义用）。
type SpawnFunc func(ctx context.Context, be Backend, inst Instance, svc ServiceSpec, opts ExecOpts) (Proc, error)

// Spawn — 实现 Spawner。
func (f SpawnFunc) Spawn(ctx context.Context, be Backend, inst Instance, svc ServiceSpec, opts ExecOpts) (Proc, error) {
	return f(ctx, be, inst, svc, opts)
}

// defaultSpawner — 缺省实现：把阻塞的 be.Exec 丢进 goroutine，停 = 取消该服务的 ctx。
//
// ★ 后台档（ExecOpts.Detach）走同一个 goroutine，只多一步：Exec 若**真签出了进程**
// （返回 res.PID > 0），这条 goroutine 不把「Exec 返回」当退出，而是转去 DetachedProbe
// 轮询那个 pid —— 直到它真退（或 ctx 到期）。这样：
//
//	· 「起完就退」与「起住长跑」在存活边上仍然分得开（前者 pid 为 0 ⇒ 立刻记退出，
//	  编排层当场报「进程在就绪前退出」，而不是白等到就绪超时）；
//	· 不支持后台档的后端（Exec 一直阻塞到退出、PID 恒 0）行为**逐字不变**；
//	· 测试用的 Spawner 缝照旧可换。
type defaultSpawner struct{}

func (defaultSpawner) Spawn(ctx context.Context, be Backend, inst Instance, svc ServiceSpec, opts ExecOpts) (Proc, error) {
	if be == nil {
		return nil, errors.New("cocoon: 缺后端")
	}
	pctx, cancel := context.WithCancel(ctx)
	p := &execProc{cancel: cancel, done: make(chan struct{}), be: be}
	argv := append([]string(nil), svc.Entry...)
	go func() {
		defer close(p.done)
		res, err := be.Exec(pctx, inst, argv, opts)
		p.res, p.err = res, err
		if err == nil && opts.Detach && res.PID > 0 {
			// 真签出了：从这里开始由探针观测退出（不把「签出」当「退出」）。
			p.res = p.observeDetached(pctx, res.PID)
		}
	}()
	return p, nil
}

// execProc — defaultSpawner 的进程句柄：Exec 的返回值就是「退出观测」。
type execProc struct {
	cancel context.CancelFunc
	done   chan struct{}
	// be — 后台档要用它取 DetachedProbe（签名出之后只有探针能说「还在不在」）。
	be Backend
	// res/err 在 close(done) 之前写、之后读 ⇒ 有 happens-before，无需再加锁。
	res ExecResult
	err error
}

// DetachedPollInterval — 后台档进程的判活轮询间隔。
//
// ★ 为什么不是「一次探完就算数」：进程刚 Start 的那一瞬谁都在，「起住没起住」要跨一个
// 小窗才看得出来（实测坏档 '/bin/sh -c exit 0' 在几十毫秒内就没了）。200ms 是「不漏判」
// 与「不白等」的折中；命令面另有自己的验活窗口（见 cocoon open）。
const DetachedPollInterval = 200 * time.Millisecond

// observeDetached — 等后台档进程真退出（返回退出观测）。ctx 到期即返回（不装死）。
func (p *execProc) observeDetached(ctx context.Context, pid int) ExecResult {
	probe, ok := p.be.(DetachedProbe)
	if !ok {
		// 没有探针 ⇒ 只能等 Stop。★ 这条**不假装能判活**：等就绪的那条路照常走，
		// 但「中途死掉」这件事本层观测不到（能力缺口，如实写在接口注释里）。
		<-ctx.Done()
		return ExecResult{RC: 0, PID: pid}
	}
	tick := time.NewTicker(DetachedPollInterval)
	defer tick.Stop()
	for {
		if _, exited, res := probe.DetachedStatus(pid); exited {
			return res
		}
		select {
		case <-ctx.Done():
			return ExecResult{RC: 0, PID: pid}
		case <-tick.C:
		}
	}
}

func (p *execProc) Wait() ExecResult {
	<-p.done
	return p.res
}

func (p *execProc) Stop(ctx context.Context) error {
	p.cancel()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ── 选项 ──────────────────────────────────────────────────────────────

// OrchestrateOptions — Start 的可替换面。零值即「全用缺省」，Start 就是它的空选项版本。
type OrchestrateOptions struct {
	// Spawn — 起进程的策略（nil = defaultSpawner）。测试用可控 Proc 演长驻/半路死。
	Spawn Spawner
	// ProbeAddr — 把 (服务, readiness) 解析成宿主侧 "host:port"（nil = 走 be.Expose 拿宿主端口）。
	// 存在理由：探测必须能指向 httptest 桩（测试不许碰真网络）。
	ProbeAddr func(svc ServiceSpec, rd Readiness) (string, error)
	// ProbeInterval — 就绪重试间隔（nil/0 = DefaultProbeInterval）。
	ProbeInterval time.Duration
	// StopTimeout — 单个服务停的上限（0 = DefaultStopTimeout）。
	StopTimeout time.Duration
	// HTTPClient — 探测用客户端（nil = 只打回环、不走代理、2s 上限的缺省客户端）。
	HTTPClient *http.Client
	// Logf — 事件日志落点（nil = 只落 Running.Log()）。T3 用它把日志并进茧的日志流。
	//
	// ★ 它在**持内部锁**时被调用：实现里不许回调 Running 的任何方法（会自锁死）。
	// 只许往文件/通道写一行，别在里头做判活之类的编排动作。
	Logf func(line string)
	// LogDir — 若给了，服务的 stdout/stderr 落 `<LogDir>/<服务名>.log`（不占内存管道）。
	//
	// ★ 为什么必须有这一格：起服务的那个进程（`cocoon open`）**会退出**，管道读端跟着消失
	// ⇒ 长驻服务下一次写 stdout/stderr 就是 EPIPE。落文件后服务活得过父进程，日志也留得住。
	// 空 = 保持原样（内存管道；只适合「起完不退」的进程内用法与假后端）。
	LogDir string
	// Detach — 后台档：服务进程与起它的进程**脱钩**（setsid + 不等退出），见 ExecOpts.Detach。
	//
	// ★ 命令面（`cocoon open`）必须置 true：它是一次性进程，阻塞档里服务会被它一起带走
	// （T3b 实测：三次真后端 open 全是 pid:0、无 nsjail 进程，集成测试能过只因为测试进程一直活着）。
	// ★ 置 true 时 LogDir 也必须有（后台档必须落文件，否则 Validate 拒执）。
	Detach bool
}

func (o OrchestrateOptions) withDefaults() OrchestrateOptions {
	if o.Spawn == nil {
		o.Spawn = defaultSpawner{}
	}
	if o.ProbeInterval <= 0 {
		o.ProbeInterval = DefaultProbeInterval
	}
	if o.StopTimeout <= 0 {
		o.StopTimeout = DefaultStopTimeout
	}
	if o.HTTPClient == nil {
		o.HTTPClient = newProbeClient()
	}
	return o
}

// newProbeClient — 探测客户端：★ Proxy=nil（不读 HTTP_PROXY：走代理会把「就绪」判到别人头上）。
func newProbeClient() *http.Client {
	tr := &http.Transport{
		Proxy:                 nil,
		DisableKeepAlives:     true,
		DialContext:           (&net.Dialer{Timeout: probeHTTPTimeout}).DialContext,
		ResponseHeaderTimeout: probeHTTPTimeout,
		TLSHandshakeTimeout:   probeHTTPTimeout,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   probeHTTPTimeout,
		// 就绪探测不跟重定向：3xx 不是 200，就该算没就绪（跟了会把登录页当健康页）。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// ── 结果 ──────────────────────────────────────────────────────────────

// StopEvent — 一次停：谁、因为谁、原因原话（「因为谁停的」就是 Because）。
type StopEvent struct {
	Service string // 被停的服务
	// Because — 直接肇因：上游服务名（级联）、失败的服务名（回滚）、或 "Stop"（人工停）。
	Because string
	// Reason — 日志原话（形如 `因为 db 已退出（rc=1）`）。
	Reason string
}

// ServiceState — 某个服务此刻的三态快照（started / healthy / completed / dead / stopped）。
type ServiceState struct {
	Name string
	// Started — 已起（launch 成功）。
	Started bool
	// Ready — healthy（readiness 探到，或未声明 readiness 的 started 即算）。
	Ready bool
	// Exited — 进程已退出（Exit 是观测值；ExitErr 非空表示连结果都没拿到）。
	Exited bool
	// Dead — 退出且非 completed（非 0 / 信号 / 超时）——存活边认这个位。
	Dead bool
	// Stopped — 被编排层停掉（人工或级联/回滚）；Stopped 的服务不再参与级联。
	Stopped bool
	// StopReason — 上面那次停的日志原话。
	StopReason string
	Exit       ExecResult
	ExitErr    error
}

// Running — 一次成功 Start 的结果（零值 = 没起过，Stop 幂等 no-op）。
//
// 内部只有一个指针，按值传即可；所有读写走同一把锁。
type Running struct {
	st *runState
}

// Valid — 是不是一次真起过的编排（零值 = 没起过）。
func (r Running) Valid() bool { return r.st != nil }

// Instance — 本次编排所在的茧实例（销毁由调用方负责：Stop 只停服务，不 Destroy）。
func (r Running) Instance() Instance {
	if r.st == nil {
		return Instance{}
	}
	return r.st.inst
}

// Started — 启动顺序（launch 成功的顺序，含尚未就绪的）。
func (r Running) Started() []string {
	if r.st == nil {
		return nil
	}
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return append([]string(nil), r.st.order...)
}

// Log — 事件日志（顺序、无时间戳：时间戳归 T3 的日志层；这里要的是可断言的确定性）。
func (r Running) Log() []string {
	if r.st == nil {
		return nil
	}
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return append([]string(nil), r.st.log...)
}

// Stops — 停的记录（按发生顺序）。
func (r Running) Stops() []StopEvent {
	if r.st == nil {
		return nil
	}
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return append([]StopEvent(nil), r.st.stops...)
}

// States — 各服务状态快照（按启动顺序）。
func (r Running) States() []ServiceState {
	if r.st == nil {
		return nil
	}
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	out := make([]ServiceState, 0, len(r.st.order))
	for _, name := range r.st.order {
		sr := r.st.svcs[name]
		if sr == nil {
			continue
		}
		out = append(out, ServiceState{
			Name:       name,
			Started:    sr.launched,
			Ready:      sr.ready,
			Exited:     sr.exited,
			Dead:       sr.exited && !sr.exitOK,
			Stopped:    sr.stopped,
			StopReason: sr.stopReason,
			Exit:       sr.exit,
			ExitErr:    sr.exitErr,
		})
	}
	return out
}

// Stop — Running 的方法形态（等价于包级 Stop）。
func (r Running) Stop(ctx context.Context) error { return Stop(ctx, r) }

// ── 内部状态 ──────────────────────────────────────────────────────────

type runState struct {
	mu   sync.Mutex
	be   Backend
	inst Instance
	spec *Spec
	opts OrchestrateOptions

	order []string           // 启动顺序（launch 成功即入列，含未就绪）
	svcs  map[string]*svcRun // 名字 → 状态
	log   []string           // 事件日志
	stops []StopEvent        // 停的记录

	// life / lifeCancel — 本次编排的**服务生命周期**上下文（见 Start 的注释：
	// 不是 Start 的入参 ctx）。Stop/rollback 收尾时取消，兜住那些不认 Stop 的进程。
	life       context.Context
	lifeCancel context.CancelFunc

	stopping bool          // Stop/rollback 已开始（watcher 见到它就不再级联）
	stopDone chan struct{} // Stop 完成信号（并发重复 Stop 等它）
	stopErr  error
}

type svcRun struct {
	spec     ServiceSpec
	proc     Proc
	launched bool
	ready    bool
	exited   bool
	// exitOK — 退出且视为 completed（rc==0、没信号、没超时）⇒ 不触发存活边。
	exitOK     bool
	stopped    bool
	stopReason string
	exit       ExecResult
	exitErr    error
}

// exitSummary — 「怎么没的」一句话（日志/报错共用，保证两处口径一致）。
func (s *svcRun) exitSummary() string {
	switch {
	case s.exitErr != nil:
		return "结果拿不到（" + s.exitErr.Error() + "）"
	case s.exit.TimedOut:
		return fmt.Sprintf("超时（RC=%d）", s.exit.RC)
	case s.exit.Signal != "":
		return fmt.Sprintf("被 %s 杀死（RC=%d）", s.exit.Signal, s.exit.RC)
	default:
		return fmt.Sprintf("rc=%d", s.exit.RC)
	}
}

// ── 入口 ──────────────────────────────────────────────────────────────

// Start — 按 spec.Order 逐服务起，顺序边就绪才放行；任一步失败 ⇒ 已起的逆序停掉。
//
// ★ ctx 只管**启动阶段**（起进程、探就绪）。起来的服务挂在 Running 自己的生命周期上，
// 只有 Stop / 级联停会停它们——否则调用方 `defer cancel()` 一句就会把服务全杀。
//
// 成功返回的 Running 里已经挂着每个服务一个 watcher goroutine：依赖死后它们负责级联停。
func Start(ctx context.Context, be Backend, spec *Spec) (Running, error) {
	return StartWith(ctx, be, spec, OrchestrateOptions{})
}

// StartWith — 带可替换面的 Start（Start = 空选项版本）。
func StartWith(ctx context.Context, be Backend, spec *Spec, opts OrchestrateOptions) (Running, error) {
	if be == nil {
		return Running{}, &OrchestrateError{Op: "plan", Cause: ErrNilSpec, Msg: "缺后端（nil）"}
	}
	if spec == nil {
		return Running{}, &OrchestrateError{Op: "plan", Cause: ErrNilSpec, Msg: "缺契约（nil）"}
	}
	// ★ 先算序、先判环：任何**副作用之前**拒绝（Provision 都没调，测试断言 be.Calls() 为空）。
	order, err := orchestratePlan(spec)
	if err != nil {
		return Running{}, err
	}
	if err := ctx.Err(); err != nil {
		return Running{}, &OrchestrateError{Op: "plan", Cause: err, Msg: "启动前 ctx 已到期"}
	}
	opts = opts.withDefaults()

	inst, err := be.Provision(ctx, spec)
	if err != nil {
		return Running{}, &OrchestrateError{Op: "plan", Cause: err, Msg: "Provision 失败"}
	}
	// ★ 服务挂在**自己的**生命周期 ctx 上，不挂在 Start 的入参 ctx 上：
	// 否则调用方一句 `defer cancel()` 就会把刚起来的服务全杀掉（Start 返回 ≠ 服务结束）。
	life, lifeCancel := context.WithCancel(context.Background())
	st := &runState{
		be:         be,
		inst:       inst,
		spec:       spec,
		opts:       opts,
		svcs:       make(map[string]*svcRun, len(order)),
		life:       life,
		lifeCancel: lifeCancel,
	}
	r := Running{st: st}

	if err := st.startAll(ctx, order); err != nil {
		stopped := st.rollback(err)
		var oe *OrchestrateError
		if errors.As(err, &oe) {
			oe.Stopped = stopped
		}
		// 回滚要收得干净：实例是 Start 自己 Provision 的，失败路径上没人能拿到它 ⇒ 这里 Destroy。
		// 用独立 ctx：调用方的 ctx 很可能正是失败原因（取消了），拿它收尾等于不收。
		dctx, cancel := context.WithTimeout(context.Background(), opts.StopTimeout)
		defer cancel()
		if derr := be.Destroy(dctx, inst); derr != nil {
			return Running{}, errors.Join(err,
				fmt.Errorf("cocoon: 回滚时 Destroy(%s) 失败: %w", inst.ID, derr))
		}
		return Running{}, err
	}
	return r, nil
}

// Stop — 逆序停掉所有还活着的服务（幂等：重复调用返回同一结果、不再停第二遍）。
func Stop(ctx context.Context, r Running) error {
	if r.st == nil {
		return nil // 零值 = 没起过；停一个不存在的东西是 no-op，不是错误（defer 里好用）
	}
	return r.st.stopAll(ctx)
}

// ── 起 ────────────────────────────────────────────────────────────────

func (st *runState) startAll(ctx context.Context, order []string) error {
	for _, name := range order {
		if err := ctx.Err(); err != nil {
			return &OrchestrateError{Op: "launch", Service: name, Cause: err, Msg: "启动中途 ctx 到期"}
		}
		svc, ok := st.spec.Service(name)
		if !ok {
			// plan 已保证 order ⊆ 服务集；走到这里说明 Spec 在 Start 期间被改了（并发写契约）。
			return &OrchestrateError{Op: "plan", Service: name, Cause: ErrBadOrder, Msg: "Order 里的服务在契约里找不到"}
		}
		// 顺序边（After）：依赖必须已 healthy 且没死。
		if err := st.checkDeps(svc); err != nil {
			return err
		}
		sr, err := st.launch(svc)
		if err != nil {
			return err
		}
		if svc.Readiness == nil {
			st.markReady(svc.Name, "未声明 readiness（started 即就绪）")
			continue
		}
		if err := st.waitReady(ctx, svc, sr); err != nil {
			return err
		}
	}
	return nil
}

// checkDeps — 顺序边的判定（只判**此刻**：起完之后的变化归存活边，见撞约束 Ⅵ）。
func (st *runState) checkDeps(svc ServiceSpec) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, dep := range svc.DependsOn {
		d := st.svcs[dep]
		if d == nil {
			return &OrchestrateError{Op: "dep", Service: svc.Name, Cause: ErrDepNotReady,
				Msg: fmt.Sprintf("依赖 %q 还没起（Order 与依赖图不一致？）", dep)}
		}
		switch {
		case d.stopped:
			return &OrchestrateError{Op: "dep", Service: svc.Name, Cause: ErrDepNotReady,
				Msg: fmt.Sprintf("依赖 %q 已被停（%s），不能在其之上起 %q", dep, d.stopReason, svc.Name)}
		case d.exited:
			return &OrchestrateError{Op: "dep", Service: svc.Name, Cause: ErrDepNotReady,
				Msg: fmt.Sprintf("依赖 %q 已退出（%s），不能在其之上起 %q", dep, d.exitSummary(), svc.Name)}
		case !d.ready:
			return &OrchestrateError{Op: "dep", Service: svc.Name, Cause: ErrDepNotReady,
				Msg: fmt.Sprintf("依赖 %q 未就绪，不能在其之上起 %q", dep, svc.Name)}
		}
	}
	return nil
}

func (st *runState) launch(svc ServiceSpec) (*svcRun, error) {
	if len(svc.Entry) == 0 {
		return nil, &OrchestrateError{Op: "launch", Service: svc.Name, Cause: ErrBadArgv, Msg: "entry 为空（argv[0] 是入口程序）"}
	}
	opts := ExecOpts{
		// ★ 服务的 Exec **不设 Timeout**：服务是长驻的，Timeout>0 等于给它判死期。
		Env: cloneEnv(st.spec.Env),
		Dir: st.spec.Cocoon.Workdir,
	}
	if st.opts.LogDir != "" {
		// 服务输出落文件（见 OrchestrateOptions.LogDir）：起服务的那条命令会退出。
		opts.LogPath = filepath.Join(st.opts.LogDir, svc.Name+".log")
	}
	// 后台档（见 OrchestrateOptions.Detach）：命令面起的一次性进程必须走这一档。
	opts.Detach = st.opts.Detach
	if err := opts.Validate(); err != nil {
		return nil, &OrchestrateError{Op: "launch", Service: svc.Name, Cause: ErrLaunchFailed, Msg: err.Error()}
	}
	proc, err := st.opts.Spawn.Spawn(st.life, st.be, st.inst, svc, opts)
	if err != nil {
		return nil, &OrchestrateError{Op: "launch", Service: svc.Name, Cause: ErrLaunchFailed, Msg: err.Error()}
	}
	sr := &svcRun{spec: svc, proc: proc, launched: true}
	st.mu.Lock()
	st.svcs[svc.Name] = sr
	st.order = append(st.order, svc.Name)
	st.logLocked(fmt.Sprintf("start %s: %s", svc.Name, strings.Join(svc.Entry, " ")))
	st.mu.Unlock()

	go st.watch(sr)
	return sr, nil
}

// watch — 存活边的源头：一个服务一个 goroutine，等它退出。
func (st *runState) watch(sr *svcRun) {
	res := sr.proc.Wait()
	st.onExit(sr, res, nil)
}

func (st *runState) markReady(name, why string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if sr := st.svcs[name]; sr != nil {
		sr.ready = true
	}
	st.logLocked("ready " + name + ": " + why)
}

// ── 就绪 ──────────────────────────────────────────────────────────────

// waitReady — 就绪判定：readiness.timeout 到期仍未 200（path 为空则端口仍不可连）⇒ 失败。
//
// ★ 进程在就绪前退出要**立刻**报，不能白等到超时（「起就崩」和「起得慢」是两回事）。
func (st *runState) waitReady(ctx context.Context, svc ServiceSpec, sr *svcRun) error {
	rd := *svc.Readiness
	timeout := rd.Timeout
	if timeout <= 0 {
		timeout = DefaultReadinessTimeout
	}
	if rd.Port < 1 || rd.Port > 65535 {
		return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: ErrReadinessTimeout,
			Msg: fmt.Sprintf("readiness.port=%d 越界（1..65535）", rd.Port)}
	}
	addr, err := st.probeAddr(ctx, svc, rd)
	if err != nil {
		return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: err,
			Msg: fmt.Sprintf("拿不到就绪探测的宿主地址（%v）", err)}
	}
	target := "http://" + addr + rd.Path
	if rd.Path == "" {
		target = "tcp://" + addr
	}

	deadline := time.Now().Add(timeout)
	// lastErr 只收「说得出话」的失败（连不上 / 非 200）；deadline 收任何失败。
	// 分开的理由：最后一次探测常被预算切掉，它的 context deadline exceeded 会盖掉
	// 真正有用的那句「返回 503（只认 200）」。
	var lastErr, lastAnyErr error
	attempts := 0
	for {
		if err := ctx.Err(); err != nil {
			return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: err,
				Msg: fmt.Sprintf("等就绪时 ctx 到期（探 %s）", target)}
		}
		if st.hasExited(sr) {
			st.mu.Lock()
			summary := sr.exitSummary()
			st.mu.Unlock()
			return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: ErrExitBeforeReady,
				Msg: fmt.Sprintf("进程在就绪前退出（%s），探 %s", summary, target)}
		}
		attempts++
		// 单次探测也要吃 readiness 的预算：否则一个卡住的 GET 会越过 deadline
		//（客户端有 2s 硬上限，而 readiness.timeout 可能只有 120ms）。
		budget := probeHTTPTimeout
		if remain := time.Until(deadline); remain < budget {
			budget = remain
		}
		pctx, pcancel := context.WithTimeout(ctx, budget)
		ok, perr := st.probeOnce(pctx, addr, rd)
		pcancel()
		if ok {
			st.markReady(svc.Name, fmt.Sprintf("探 %s 第 %d 次成功", target, attempts))
			return nil
		}
		lastAnyErr = perr
		if perr != nil && !errors.Is(perr, context.DeadlineExceeded) && !errors.Is(perr, context.Canceled) {
			lastErr = perr
		}
		remain := time.Until(deadline)
		if remain <= 0 {
			report := lastErr
			if report == nil {
				report = lastAnyErr
			}
			return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: ErrReadinessTimeout,
				Msg: fmt.Sprintf("%s 内没就绪（探 %s 共 %d 次，最后一次：%v）", timeout, target, attempts, report)}
		}
		if d := st.opts.ProbeInterval; d < remain {
			remain = d
		}
		if err := sleepCtx(ctx, remain); err != nil {
			return &OrchestrateError{Op: "ready", Service: svc.Name, Cause: err,
				Msg: fmt.Sprintf("等就绪时 ctx 到期（探 %s）", target)}
		}
	}
}

// probeAddr — 茧内端口 → 宿主 "host:port"。缺省走 Expose（同时保证端口露出来）。
func (st *runState) probeAddr(ctx context.Context, svc ServiceSpec, rd Readiness) (string, error) {
	if st.opts.ProbeAddr != nil {
		return st.opts.ProbeAddr(svc, rd)
	}
	hostPort, err := st.be.Expose(ctx, st.inst, rd.Port, DefaultBind)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", DefaultBind, hostPort), nil
}

// probeOnce — 一次探测。path 非空 ⇒ HTTP GET **只认 200**；path 为空 ⇒ 端口可连。
func (st *runState) probeOnce(ctx context.Context, addr string, rd Readiness) (bool, error) {
	if rd.Path == "" {
		d := &net.Dialer{Timeout: probeHTTPTimeout}
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return false, err
		}
		_ = c.Close()
		return true, nil
	}
	u := "http://" + addr + rd.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	resp, err := st.opts.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeBodyDrain))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GET %s 返回 %d（只认 200）", u, resp.StatusCode)
	}
	return true, nil
}

// ── 退出与级联停（存活边）────────────────────────────────────────────

// onExit — 一个服务退了：completed 只记一笔；死（非 0/信号/超时）则级联停下游。
func (st *runState) onExit(sr *svcRun, res ExecResult, exitErr error) {
	st.mu.Lock()
	if sr.exited || sr.stopped || st.stopping {
		// 自己停的（proc.Stop 后 Wait 才回来）不算「死」；正在整体停/回滚时也不再级联。
		st.mu.Unlock()
		return
	}
	sr.exited = true
	sr.exit = res
	sr.exitErr = exitErr
	sr.exitOK = exitErr == nil && res.Success()
	dead := !sr.exitOK
	name := sr.spec.Name
	summary := sr.exitSummary()
	var victims []cascadeVictim
	if dead {
		victims = st.cascadePlanLocked(name, summary)
	}
	if sr.exitOK {
		st.logLocked(fmt.Sprintf("exit %s: 正常完成（%s，compose completed）", name, summary))
	} else {
		st.logLocked(fmt.Sprintf("died %s: %s", name, summary))
	}
	st.mu.Unlock()

	// 停在锁外做：proc.Stop 可能慢（真后端要等宽限期），握着锁等会把整个编排卡住。
	ctx, cancel := context.WithTimeout(context.Background(), st.opts.StopTimeout)
	defer cancel()
	for _, v := range victims { // victims 已是逆序（下游在前）
		_ = st.stopService(ctx, v.name, v.because, v.why)
	}
}

// cascadeVictim — 一个要被级联停掉的下游 + 它的「因为谁」。
type cascadeVictim struct {
	name    string
	because string // 直接肇因（最近的那一环）
	why     string // 日志原话
}

// cascadePlanLocked — 逆序的级联名单 + 每条「因为谁」。
//
// 传递闭包按顺序一遍即可：order 是拓扑序（依赖在前），所以扫描到 X 时「X 的依赖是否会跟着停」
// 已经确定——于是 web←api←db 能写成 web 因为 api、api 因为 db，而不是一律记「因为 db」
// （那会把链断在哪一段这件事糊掉）。
func (st *runState) cascadePlanLocked(root, rootSummary string) (victims []cascadeVictim) {
	affected := map[string]bool{}
	causes := map[string]string{}
	for _, name := range st.order {
		sr := st.svcs[name]
		if sr == nil {
			continue
		}
		for _, dep := range sr.spec.DependsOn {
			if dep == root || affected[dep] {
				affected[name] = true
				if dep == root {
					causes[name] = root
				} else {
					causes[name] = dep // 直接肇因是「也要跟着停的那个上游」
				}
				break
			}
		}
	}
	for i := len(st.order) - 1; i >= 0; i-- { // ★ 逆序停
		name := st.order[i]
		if !affected[name] {
			continue
		}
		sr := st.svcs[name]
		if sr == nil || sr.stopped || sr.exited {
			continue
		}
		because := causes[name]
		why := fmt.Sprintf("%s 已退出（%s）", root, rootSummary)
		if because != root {
			why = fmt.Sprintf("%s 已随 %s 停掉（%s 已退出，%s）", because, root, root, rootSummary)
		}
		victims = append(victims, cascadeVictim{name: name, because: because, why: why})
	}
	return victims
}

// ── 停 ────────────────────────────────────────────────────────────────

// stopAll — 逆序停（人工 Stop 与 rollback 共用同一套「逆序 + 记因为」）。
func (st *runState) stopAll(ctx context.Context) error {
	st.mu.Lock()
	if st.stopping {
		done := st.stopDone
		st.mu.Unlock()
		if done == nil {
			return nil
		}
		select {
		case <-done:
			return st.stopErr // 幂等：重复 Stop 回同一个结果
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	st.stopping = true
	st.stopDone = make(chan struct{})
	st.mu.Unlock()

	var errs []error
	for i := len(st.order) - 1; i >= 0; i-- { // ★ 逆序
		name := st.order[i]
		if err := st.stopService(ctx, name, "Stop", "Stop 请求（按启动逆序）"); err != nil {
			errs = append(errs, err)
		}
	}
	st.lifeCancel() // 兜底：不认 Stop 的进程也会因 ctx 取消而退
	st.mu.Lock()
	if len(st.stops) == 0 {
		st.logLocked("stop: 无存活服务（幂等 no-op）")
	}
	err := errors.Join(errs...)
	st.stopErr = err
	st.mu.Unlock()
	close(st.stopDone)
	return err
}

// rollback — 启动失败的回滚：逆序停掉本次已起的集合，返回停掉的名单（逆序）。
func (st *runState) rollback(cause error) []string {
	st.mu.Lock()
	st.stopping = true // 先把级联关掉：回滚自己会停，不需要 watcher 帮手
	st.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), st.opts.StopTimeout)
	defer cancel()

	failing := ""
	var oe *OrchestrateError
	if errors.As(cause, &oe) {
		failing = oe.Service
	}
	var stopped []string
	for i := len(st.order) - 1; i >= 0; i-- {
		name := st.order[i]
		why := "启动回滚（逆序停）"
		if failing != "" {
			why = fmt.Sprintf("%s 没起来（启动回滚，逆序停）", failing)
		}
		if st.isStoppedOrExited(name) {
			continue
		}
		if err := st.stopService(ctx, name, failing, why); err != nil {
			st.mu.Lock()
			st.logLocked(fmt.Sprintf("rollback stop %s: 失败（%v）", name, err))
			st.mu.Unlock()
			continue
		}
		stopped = append(stopped, name)
	}
	st.lifeCancel() // 兜底：不认 Stop 的进程也会因 ctx 取消而退
	return stopped
}

func (st *runState) isStoppedOrExited(name string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	sr := st.svcs[name]
	return sr == nil || sr.stopped || sr.exited
}

// stopService — 停一个服务并记「因为谁」。幂等：已停/已退出 ⇒ no-op。
//
// ★ 先在锁里把它标成 stopped，再在锁外叫停：标位是为了让 watcher 见到 Wait 回来时
// 不当成「半路死」（否则一次人工 Stop 会额外触发一轮级联）。
func (st *runState) stopService(ctx context.Context, name, because, why string) error {
	st.mu.Lock()
	sr := st.svcs[name]
	if sr == nil || sr.stopped || sr.exited {
		st.mu.Unlock()
		return nil
	}
	sr.stopped = true
	sr.stopReason = "因为 " + why
	proc := sr.proc
	line := fmt.Sprintf("stop %s: 因为 %s", name, why)
	st.stops = append(st.stops, StopEvent{Service: name, Because: because, Reason: "因为 " + why})
	st.logLocked(line)
	st.mu.Unlock()

	if proc == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := proc.Stop(ctx); err != nil {
		st.mu.Lock()
		st.logLocked(fmt.Sprintf("stop %s: 叫停失败（%v）", name, err))
		st.mu.Unlock()
		return fmt.Errorf("cocoon: 停 %q 失败: %w", name, err)
	}
	return nil
}

// ── 序（拓扑序 + 防御性判环）──────────────────────────────────────────

// orchestratePlan — 校验图并用 Spec.Order；Spec.Order 缺失/不全时自己算。
//
// ★ 解析器已判过环（spec.go validateGraph）；这里**再判一次**是防御性的：Spec 是公开结构体，
// 手搓一份绕过解析器的契约不该让编排层去起一个永远起不完的环。
func orchestratePlan(spec *Spec) ([]string, error) {
	if len(spec.Services) == 0 {
		return nil, &OrchestrateError{Op: "plan", Cause: ErrNoServices, Msg: "至少需要一个 [[服务]]"}
	}
	known := make(map[string]ServiceSpec, len(spec.Services))
	for _, s := range spec.Services {
		if s.Name == "" {
			return nil, &OrchestrateError{Op: "plan", Cause: ErrNoServices, Msg: "服务名为空"}
		}
		if _, dup := known[s.Name]; dup {
			return nil, &OrchestrateError{Op: "plan", Service: s.Name, Cause: ErrBadOrder,
				Msg: "服务名重复（编排层按名字寻址，重名会把两个服务当成一个）"}
		}
		known[s.Name] = s
	}
	for _, s := range spec.Services {
		for _, dep := range s.DependsOn {
			if dep == s.Name {
				return nil, &OrchestrateError{Op: "plan", Service: s.Name, Cause: ErrCycle,
					Msg: "服务依赖自身（循环依赖：" + s.Name + " → " + s.Name + "）"}
			}
			if _, ok := known[dep]; !ok {
				return nil, &OrchestrateError{Op: "plan", Service: s.Name, Cause: ErrDanglingDep,
					Msg: fmt.Sprintf("依赖 %q 不存在（已定义：%s）", dep, strings.Join(namesInDeclOrder(spec.Services), ", "))}
			}
		}
	}
	computed, err := topoOrder(spec.Services, known)
	if err != nil {
		return nil, err
	}
	if len(spec.Order) == 0 {
		return computed, nil
	}
	if err := checkTopoOrder(spec.Order, known); err != nil {
		return nil, err
	}
	return append([]string(nil), spec.Order...), nil
}

// topoOrder — DFS 三色（与 spec.go 同构：灰链上的回边即环，报错带完整环路径）。
// 同层按**声明顺序**展开，保证同一份契约的启动序稳定（不稳定 = 测试与判据都不稳定）。
func topoOrder(svcs []ServiceSpec, known map[string]ServiceSpec) ([]string, error) {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	state := make(map[string]int, len(svcs))
	stack := make([]string, 0, len(svcs))
	order := make([]string, 0, len(svcs))

	var visit func(name string) error
	visit = func(name string) error {
		state[name] = grey
		stack = append(stack, name)
		for _, dep := range known[name].DependsOn {
			switch state[dep] {
			case grey:
				start := 0
				for i, n := range stack {
					if n == dep {
						start = i
						break
					}
				}
				cyc := append(append([]string{}, stack[start:]...), dep)
				return &OrchestrateError{Op: "plan", Service: dep, Cause: ErrCycle,
					Msg: "循环依赖：" + strings.Join(cyc, " → ")}
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = black
		order = append(order, name)
		return nil
	}
	for _, s := range svcs {
		if state[s.Name] == white {
			if err := visit(s.Name); err != nil {
				return nil, err
			}
		}
	}
	return order, nil
}

// checkTopoOrder — Spec.Order 是不是**这份图**的合法拓扑序：覆盖全部服务、且每个依赖在前。
func checkTopoOrder(order []string, known map[string]ServiceSpec) error {
	if len(order) != len(known) {
		return &OrchestrateError{Op: "plan", Cause: ErrBadOrder,
			Msg: fmt.Sprintf("Order 有 %d 项，服务有 %d 个（缺项会让某个服务永远不起）", len(order), len(known))}
	}
	pos := make(map[string]int, len(order))
	for i, name := range order {
		if _, dup := pos[name]; dup {
			return &OrchestrateError{Op: "plan", Service: name, Cause: ErrBadOrder, Msg: "Order 里重复出现"}
		}
		if _, ok := known[name]; !ok {
			return &OrchestrateError{Op: "plan", Service: name, Cause: ErrBadOrder, Msg: "Order 里有不存在的服务"}
		}
		pos[name] = i
	}
	for name, svc := range known {
		for _, dep := range svc.DependsOn {
			if pos[dep] > pos[name] {
				return &OrchestrateError{Op: "plan", Service: name, Cause: ErrBadOrder,
					Msg: fmt.Sprintf("依赖 %q 排在它之后（Order 违反拓扑序：%s）", dep, strings.Join(order, " → "))}
			}
		}
	}
	return nil
}

func namesInDeclOrder(svcs []ServiceSpec) []string {
	out := make([]string, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, s.Name)
	}
	return out
}

// ── 小工具 ────────────────────────────────────────────────────────────

func (st *runState) logLocked(line string) {
	st.log = append(st.log, line)
	if st.opts.Logf != nil {
		st.opts.Logf(line)
	}
}

func (st *runState) hasExited(sr *svcRun) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return sr.exited || sr.stopped
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func cloneEnv(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}
