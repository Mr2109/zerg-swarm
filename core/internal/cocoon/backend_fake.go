package cocoon

// 假后端：**不落盘、不起进程**，只用内存记账。
//
// 两个用途：
//  1. 编排层（起服务的顺序、readiness、级联停、RC 判定）在没有真后端时能整链跑通；
//  2. 测试要造「超时」「被信号杀」「非零退出」这些真跑很难稳定复现的情形。
//
// ★ 它**不许**顺手把不合契约的结果改好：结果照预设原样返回，返回前过一遍
// ExecResult.Validate；预设本身违反契约（如 TimedOut=true 且 RC=0）就报错——
// 这样「超时不许伪装成 0」这条规则是被测试打住的，不是靠注释自觉。

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

// fakeBackendName — 假后端写进 Instance.Backend 的名字。
const fakeBackendName = "fake"

// FakeFirstHostPort — 假后端从 20000 起分配宿主端口（与设计稿 §D 的样例量级一致）。
const FakeFirstHostPort = 20000

// FakeBackend — 内存假后端（实现 Backend）。并发安全。
type FakeBackend struct {
	mu sync.Mutex

	nextID   int
	nextPort int

	insts map[string]*fakeInstance
	// provisioned — provision 顺序（诊断用；不等于存活集合）。
	provisioned []string
	// calls — 五动作流水（"provision fake-1"、"expose fake-1 8000->20000" …）。
	calls []string
	// script — argv 键 → 待回放的结果队列（FIFO，用尽后落 fallback）。
	script map[string][]ExecResult
	// fallback — 没预设时用的结果（零值 = RC=0 正常退出）。
	fallback ExecResult
	// usedPorts — 已发出去的宿主端口（destroy 时回收）。
	usedPorts map[int]bool
	// freePorts — 回收来的端口（升序；分配时优先复用最小的，模拟「端口随实例归还」）。
	freePorts []int
}

type fakeInstance struct {
	id   string
	spec *Spec
	root string

	mounts     map[string]FakeMount // innerPath → 挂载
	mountOrder []string
	ports      map[string]int // "inner/bind" → 宿主端口
	execs      int
	last       *FakeExec
	gone       bool
}

// FakeMount — 假后端记下的一次挂载。
type FakeMount struct {
	Host  string
	Inner string
	RO    bool
}

// FakeExec — 假后端记下的一次 exec 入参（供断言「确实透传了」）。
type FakeExec struct {
	Instance Instance
	Argv     []string
	Opts     ExecOpts
}

// NewFakeBackend — 造一个空的假后端（端口从 FakeFirstHostPort 起）。
func NewFakeBackend() *FakeBackend {
	return &FakeBackend{
		nextPort:  FakeFirstHostPort,
		insts:     make(map[string]*fakeInstance),
		script:    make(map[string][]ExecResult),
		usedPorts: make(map[int]bool),
	}
}

// 编译期钉住：假后端必须满足接口（接口一改，这里立刻红）。
var (
	_ Backend            = (*FakeBackend)(nil)
	_ CapabilityReporter = (*FakeBackend)(nil)
)

// Name — 后端名（Instance.Backend 同字面量；与 NsjailBackend.Name 对称）。
func (b *FakeBackend) Name() string { return fakeBackendName }

// Capabilities — 假后端的能力面：**没有隔离、没有对端身份、不起进程**。
//
// ★ 这里照实写 none/false，✗ 不借用平台表（`LinuxCapabilities()` 说的是「真 nsjail 能给
// 什么」，与本节的后端不是一件事）。Expose 是唯一一个 true：它真会分配一个宿主端口号
// ——但只是记账（不转发），所以 ExposeMechanism 必须把话说明白。
func (b *FakeBackend) Capabilities() (Capabilities, error) {
	c := Capabilities{
		Platform:            HostPlatform(),
		Isolation:           "fake（脚手架 · 无隔离）",
		PeerIdentity:        PeerIdentityNone,
		PeerMechanism:       "无（假后端不签发对端身份）",
		ProcessTreeKill:     false,
		KillMechanism:       "无内核级回收（假后端不起进程）",
		SymlinkNoFollow:     false,
		SymlinkMechanism:    "待接真后端",
		CrossMountGuard:     false,
		CrossMountMechanism: "待接真后端",
		Expose:              true,
		ExposeMechanism:     "仅记账（假后端分配宿主端口号，不做真转发）",
		Overlay:             false,
		OverlayMechanism:    "无（假后端不起进程、不碰文件系统：没有镜像根可叠）",
	}
	if err := c.Validate(); err != nil {
		return Capabilities{}, err
	}
	return c, nil
}

// ExecKey — argv 的脚本键（用 NUL 拼，避免 "a b" 与 "a"+"b" 撞在一起）。
func ExecKey(argv []string) string { return strings.Join(argv, "\x00") }

// ── 预设（测试/编排用）────────────────────────────────────────────────

// Script — 给某条 argv 排一串结果：第 n 次调用回放第 n 个，用尽后落 fallback。
func (b *FakeBackend) Script(argv []string, results ...ExecResult) *FakeBackend {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := ExecKey(argv)
	b.script[key] = append(b.script[key], results...)
	return b
}

// SetFallback — 没预设时返回什么（零值 = 成功、空输出、0ms）。
func (b *FakeBackend) SetFallback(r ExecResult) *FakeBackend {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallback = r
	return b
}

// ── 观测（测试/编排用）────────────────────────────────────────────────

// Calls — 五动作流水副本（按发生顺序）。
func (b *FakeBackend) Calls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

// Alive — 实例还在（provision 成功且未 destroy）。
func (b *FakeBackend) Alive(inst Instance) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	fi, ok := b.insts[inst.ID]
	return ok && !fi.gone
}

// LiveIDs — 当前存活实例（按 id 排序，便于稳定断言）。
func (b *FakeBackend) LiveIDs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for id, fi := range b.insts {
		if !fi.gone {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// LastExec — 该实例最近一次 exec 的入参。
func (b *FakeBackend) LastExec(inst Instance) (FakeExec, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fi, ok := b.insts[inst.ID]
	if !ok || fi.last == nil {
		return FakeExec{}, false
	}
	out := *fi.last
	out.Argv = append([]string(nil), fi.last.Argv...)
	return out, true
}

// Mounts — 该实例的挂载（按挂载顺序）。
func (b *FakeBackend) Mounts(inst Instance) []FakeMount {
	b.mu.Lock()
	defer b.mu.Unlock()
	fi, ok := b.insts[inst.ID]
	if !ok {
		return nil
	}
	out := make([]FakeMount, 0, len(fi.mountOrder))
	for _, inner := range fi.mountOrder {
		out = append(out, fi.mounts[inner])
	}
	return out
}

// HostPorts — 该实例的端口映射副本（茧内端口 → 宿主端口）。
func (b *FakeBackend) HostPorts(inst Instance) map[int]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[int]int{}
	fi, ok := b.insts[inst.ID]
	if !ok {
		return out
	}
	for key, host := range fi.ports {
		var inner int
		var bind string
		if _, err := fmt.Sscanf(key, "%d/%s", &inner, &bind); err != nil {
			continue
		}
		out[inner] = host
	}
	return out
}

// ProvisionedSpec — 该实例的契约（编排层用）。
func (b *FakeBackend) ProvisionedSpec(inst Instance) (*Spec, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fi, ok := b.insts[inst.ID]
	if !ok {
		return nil, false
	}
	return fi.spec, true
}

// ── 五动作 ────────────────────────────────────────────────────────────

// Provision — 分配 id（fake-1、fake-2…）与一个不落盘的根记号。
// ★ 假后端不起进程 ⇒ PID 恒 0；编排层不许把 PID>0 当「活着」的判据。
func (b *FakeBackend) Provision(ctx context.Context, spec *Spec) (Instance, error) {
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}
	if spec == nil {
		return Instance{}, ErrNilSpec
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := fmt.Sprintf("%s-%d", fakeBackendName, b.nextID)
	fi := &fakeInstance{
		id:     id,
		spec:   spec,
		root:   path.Join("/"+fakeBackendName, id),
		mounts: make(map[string]FakeMount),
		ports:  make(map[string]int),
	}
	b.insts[id] = fi
	b.provisioned = append(b.provisioned, id)
	b.calls = append(b.calls, "provision "+id)

	return Instance{ID: id, Backend: fakeBackendName, Root: fi.root}, nil
}

// Exec — 回放预设脚本；没有预设就是成功空跑。
//
// 返回前过 ExecResult.Validate：预设若把「超时」写成 RC=0，这里直接报
// ErrBadExecResult，而不是照抄给上层（那样上层会当成功）。
func (b *FakeBackend) Exec(ctx context.Context, inst Instance, argv []string, opts ExecOpts) (ExecResult, error) {
	if err := ctx.Err(); err != nil {
		return ExecResult{}, err
	}
	if len(argv) == 0 || argv[0] == "" {
		return ExecResult{}, fmt.Errorf("%w: %q", ErrBadArgv, argv)
	}
	if err := opts.Validate(); err != nil {
		return ExecResult{}, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	fi, err := b.lookupLocked(inst)
	if err != nil {
		return ExecResult{}, err
	}

	key := ExecKey(argv)
	res := b.fallback
	if q := b.script[key]; len(q) > 0 {
		res = q[0]
		if len(q) == 1 {
			delete(b.script, key)
		} else {
			b.script[key] = q[1:]
		}
	}

	fi.execs++
	fi.last = &FakeExec{Instance: inst, Argv: append([]string(nil), argv...), Opts: cloneExecOpts(opts)}
	b.calls = append(b.calls, fmt.Sprintf("exec %s %s", inst.ID, strings.Join(argv, " ")))

	if verr := res.Validate(); verr != nil {
		return ExecResult{}, fmt.Errorf("%s: %w", fakeBackendName, verr)
	}
	return res.clone(), nil
}

// Mount — 记账；同一 innerPath 重复挂同一源是 no-op，挂不同源报 ErrMountConflict。
func (b *FakeBackend) Mount(ctx context.Context, inst Instance, hostPath, innerPath string, ro bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hostPath == "" || !path.IsAbs(hostPath) {
		return fmt.Errorf("%w: 宿主路径必须是绝对路径，得到 %q", ErrBadMount, hostPath)
	}
	if innerPath == "" || !path.IsAbs(innerPath) {
		return fmt.Errorf("%w: 茧内路径必须是绝对路径，得到 %q", ErrBadMount, innerPath)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	fi, err := b.lookupLocked(inst)
	if err != nil {
		return err
	}
	if old, ok := fi.mounts[innerPath]; ok {
		if old.Host == hostPath && old.RO == ro {
			return nil // 幂等
		}
		return fmt.Errorf("%w: %s 已挂 %s(ro=%v)，拒再挂 %s(ro=%v)",
			ErrMountConflict, innerPath, old.Host, old.RO, hostPath, ro)
	}
	fi.mounts[innerPath] = FakeMount{Host: hostPath, Inner: innerPath, RO: ro}
	fi.mountOrder = append(fi.mountOrder, innerPath)
	b.calls = append(b.calls, fmt.Sprintf("mount %s %s->%s ro=%v", inst.ID, hostPath, innerPath, ro))
	return nil
}

// Expose — 从 FakeFirstHostPort 起递增分配，不重复；同一 (innerPort,bind) 再来是
// 幂等（返回同一宿主端口，真后端也是这个语义）。
func (b *FakeBackend) Expose(ctx context.Context, inst Instance, innerPort int, bind string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if innerPort < 1 || innerPort > 65535 {
		return 0, fmt.Errorf("%w: 茧内端口 %d 越界（1..65535）", ErrBadPort, innerPort)
	}
	if bind == "" {
		bind = DefaultBind
	}
	if !isLoopbackBind(bind) {
		return 0, fmt.Errorf("%w: %q（默认/唯一允许 %s）", ErrBindNotLoopback, bind, DefaultBind)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	fi, err := b.lookupLocked(inst)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%d/%s", innerPort, bind)
	if host, ok := fi.ports[key]; ok {
		return host, nil
	}
	host, err := b.allocPortLocked()
	if err != nil {
		return 0, err
	}
	fi.ports[key] = host
	b.calls = append(b.calls, fmt.Sprintf("expose %s %d->%d on %s", inst.ID, innerPort, host, bind))
	return host, nil
}

// Destroy — 标记销毁并回收端口。**幂等**：已销毁的再调返回 nil；
// 但 id 从没 provision 过是 ErrInstanceNotFound（「幂等」不等于「什么都收」）。
func (b *FakeBackend) Destroy(ctx context.Context, inst Instance) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if inst.ID == "" {
		return fmt.Errorf("%w: 空 id", ErrInstanceNotFound)
	}
	if inst.Backend != "" && inst.Backend != fakeBackendName {
		return fmt.Errorf("%w: %s 不属于 %s 后端", ErrInstanceNotFound, inst.ID, fakeBackendName)
	}
	fi, ok := b.insts[inst.ID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrInstanceNotFound, inst.ID)
	}
	if fi.gone {
		b.calls = append(b.calls, "destroy "+inst.ID+" (noop)")
		return nil
	}
	fi.gone = true
	for _, host := range fi.ports {
		delete(b.usedPorts, host)
		b.freePorts = append(b.freePorts, host)
	}
	sort.Ints(b.freePorts)
	b.calls = append(b.calls, "destroy "+inst.ID)
	return nil
}

// ── 内部 ──────────────────────────────────────────────────────────────

func (b *FakeBackend) lookupLocked(inst Instance) (*fakeInstance, error) {
	if inst.ID == "" {
		return nil, fmt.Errorf("%w: 空 id", ErrInstanceNotFound)
	}
	if inst.Backend != "" && inst.Backend != fakeBackendName {
		return nil, fmt.Errorf("%w: %s 不属于 %s 后端", ErrInstanceNotFound, inst.ID, fakeBackendName)
	}
	fi, ok := b.insts[inst.ID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInstanceNotFound, inst.ID)
	}
	if fi.gone {
		return nil, fmt.Errorf("%w: %s", ErrInstanceGone, inst.ID)
	}
	return fi, nil
}

func (b *FakeBackend) allocPortLocked() (int, error) {
	// 先还回来的（最小的优先），再往后递增。
	for len(b.freePorts) > 0 {
		p := b.freePorts[0]
		b.freePorts = b.freePorts[1:]
		if b.usedPorts[p] {
			continue
		}
		b.usedPorts[p] = true
		return p, nil
	}
	for b.nextPort <= 65535 {
		p := b.nextPort
		b.nextPort++
		if b.usedPorts[p] {
			continue
		}
		b.usedPorts[p] = true
		return p, nil
	}
	return 0, ErrPortExhausted
}

func cloneExecOpts(o ExecOpts) ExecOpts {
	out := o
	out.Stdin = append([]byte(nil), o.Stdin...)
	if o.Env != nil {
		out.Env = make(map[string]string, len(o.Env))
		for k, v := range o.Env {
			out.Env[k] = v
		}
	}
	return out
}
