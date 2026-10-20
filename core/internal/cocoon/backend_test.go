// backend_test.go — 后端契约六格：
//
//	① TestFakeBackendFiveActions        五动作顺序跑通（provision→mount/expose→exec→destroy），入参确实透传
//	② TestExecTimeoutNeverZeroRC        超时 ⇒ TimedOut=true 且 RC=RCTimeout（≠0）；预设把超时写成 RC=0 必被拒
//	③ TestDestroyIdempotent             destroy 两次幂等（第二次 nil）、端口回收；已销毁再 exec 报 ErrInstanceGone
//	④ TestExposePortsUnique             expose 从 20000 递增、不重复、同键幂等；非回环/越界/号段用尽必拒
//	⑤ TestCapabilitiesPerPlatform       三平台能力逐项断言（v3 §F 表），未知平台不许给默认值
//	⑥ TestExecRCAndSignalPassthrough    rc 透传（含非零）＋ 信号透传（128+n）＋ 结果自检的负例表
//
// 每格都断言「断了会怎样」：不是只验 happy path。
package cocoon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

// bkSpec — 用 spec_test.go 的最小模板造一份**真解析过**的契约（不手搓 Spec{}，
// 那样等于绕开解析器，测不到 Provision 拿到的到底是不是合法契约）。
func bkSpec(t *testing.T) *Spec {
	t.Helper()
	spec, err := ParseSpec([]byte(specWithDigest(specValidDigest)))
	if err != nil {
		t.Fatalf("夹具契约应当解析通过，得到错误: %v", err)
	}
	return spec
}

// bkProvision — 造后端 + 造实例（失败即 Fatal）。
func bkProvision(t *testing.T) (*FakeBackend, Instance) {
	t.Helper()
	be := NewFakeBackend()
	inst, err := be.Provision(context.Background(), bkSpec(t))
	if err != nil {
		t.Fatalf("Provision 应当成功，得到错误: %v", err)
	}
	return be, inst
}

// ── ① 五动作顺序 ──────────────────────────────────────────────────────

func TestFakeBackendFiveActions(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()

	inst, err := be.Provision(ctx, bkSpec(t))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if inst.ID != "fake-1" {
		t.Fatalf("首个实例 id 应为 fake-1，得到 %q", inst.ID)
	}
	if inst.Backend != "fake" {
		t.Fatalf("Backend 应为 fake，得到 %q", inst.Backend)
	}
	if inst.Root == "" {
		t.Fatal("Root 应给出宿主机侧记号（不许为空，否则编排层无法定位）")
	}
	if !be.Alive(inst) {
		t.Fatal("刚 provision 的实例应当存活")
	}

	// 第二个实例：id 不重复、句柄互不相同。
	inst2, err := be.Provision(ctx, bkSpec(t))
	if err != nil {
		t.Fatalf("第二次 Provision: %v", err)
	}
	if inst2.ID == inst.ID {
		t.Fatalf("两次 provision 的 id 不得相同（都是 %q）", inst.ID)
	}

	// 挂载：一读一只写。
	if err := be.Mount(ctx, inst, "/host/app", "/app", true); err != nil {
		t.Fatalf("Mount(ro): %v", err)
	}
	if err := be.Mount(ctx, inst, "/host/data", "/data", false); err != nil {
		t.Fatalf("Mount(rw): %v", err)
	}
	mounts := be.Mounts(inst)
	if len(mounts) != 2 {
		t.Fatalf("应有 2 个挂载，得到 %d: %+v", len(mounts), mounts)
	}
	if mounts[0].Inner != "/app" || mounts[0].Host != "/host/app" || !mounts[0].RO {
		t.Fatalf("首个挂载应为 /host/app→/app(ro=true)，得到 %+v", mounts[0])
	}
	if mounts[1].Inner != "/data" || mounts[1].RO {
		t.Fatalf("第二个挂载应为 /host/data→/data(ro=false)，得到 %+v", mounts[1])
	}
	// 同一 innerPath 重复挂同一源：幂等，不报错也不重复记账。
	if err := be.Mount(ctx, inst, "/host/app", "/app", true); err != nil {
		t.Fatalf("同源重复挂载应当幂等，得到错误: %v", err)
	}
	if n := len(be.Mounts(inst)); n != 2 {
		t.Fatalf("幂等挂载后仍应是 2 个挂载，得到 %d", n)
	}

	// 露端口：bind 空 ⇒ 默认回环。
	hostPort, err := be.Expose(ctx, inst, 8000, "")
	if err != nil {
		t.Fatalf("Expose: %v", err)
	}
	if hostPort != FakeFirstHostPort {
		t.Fatalf("首个宿主端口应为 %d，得到 %d", FakeFirstHostPort, hostPort)
	}

	// exec：入参（argv/stdin/env/dir/tty/timeout）必须原样到后端。
	argv := []string{"uvicorn", "app:api", "--port", "8000"}
	be.Script(argv, ExecResult{RC: 0, Stdout: []byte("listening\n"), MS: 12})
	opts := ExecOpts{
		Timeout: 30 * time.Second,
		TTY:     true,
		Stdin:   []byte("hello\n"),
		Env:     map[string]string{"TZ": "Asia/Shanghai", "APP_ENV": "prod"},
		Dir:     "/app",
	}
	res, err := be.Exec(ctx, inst, argv, opts)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !res.Success() {
		t.Fatalf("预设是成功，Success() 却为 false: %+v", res)
	}
	if string(res.Stdout) != "listening\n" {
		t.Fatalf("stdout 应透传 listening\\n，得到 %q", res.Stdout)
	}
	if res.MS != 12 {
		t.Fatalf("MS 应透传 12，得到 %d", res.MS)
	}
	last, ok := be.LastExec(inst)
	if !ok {
		t.Fatal("LastExec 应记到这次调用")
	}
	if !slices.Equal(last.Argv, argv) {
		t.Fatalf("argv 应原样透传，得到 %v", last.Argv)
	}
	if last.Opts.Dir != "/app" || !last.Opts.TTY || last.Opts.Timeout != 30*time.Second {
		t.Fatalf("ExecOpts 的 dir/tty/timeout 应透传，得到 %+v", last.Opts)
	}
	if string(last.Opts.Stdin) != "hello\n" {
		t.Fatalf("stdin 应透传，得到 %q", last.Opts.Stdin)
	}
	if last.Opts.Env["APP_ENV"] != "prod" || last.Opts.Env["TZ"] != "Asia/Shanghai" {
		t.Fatalf("env 应透传，得到 %+v", last.Opts.Env)
	}

	// 收尾：destroy 之后不再存活，端口映射也随实例走。
	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if be.Alive(inst) {
		t.Fatal("destroy 后不应存活")
	}
	if ids := be.LiveIDs(); !slices.Equal(ids, []string{inst2.ID}) {
		t.Fatalf("活着应只剩 %s，得到 %v", inst2.ID, ids)
	}

	// 调用流水：五动作都发生过，且顺序是 provision → mount/expose → exec → destroy。
	calls := be.Calls()
	order := []string{"provision " + inst.ID, "mount ", "expose ", "exec ", "destroy " + inst.ID}
	at := 0
	for _, want := range order {
		found := false
		for ; at < len(calls); at++ {
			if strings.HasPrefix(calls[at], want) {
				found = true
				at++
				break
			}
		}
		if !found {
			t.Fatalf("调用流水缺少（或顺序错）%q：%v", want, calls)
		}
	}
}

// ── ② 超时：RC 不许伪装成 0 ───────────────────────────────────────────

func TestExecTimeoutNeverZeroRC(t *testing.T) {
	ctx := context.Background()
	be, inst := bkProvision(t)

	argv := []string{"sleep", "60"}
	be.Script(argv, ExecResult{TimedOut: true, RC: RCTimeout, MS: 1000})

	res, err := be.Exec(ctx, inst, argv, ExecOpts{Timeout: time.Second})
	if err != nil {
		// 超时是「可判结果」，不是后端故障（只有拿不到结果才是 err）。
		t.Fatalf("超时不应返回错误（事实应写在 TimedOut/RC 里），得到: %v", err)
	}
	if !res.TimedOut {
		t.Fatal("TimedOut 必须为 true")
	}
	if res.RC == 0 {
		t.Fatal("★ 超时的 RC 不许是 0（那是「成功」的样子）")
	}
	if res.RC != RCTimeout {
		t.Fatalf("超时的 RC 应为 RCTimeout=%d，得到 %d", RCTimeout, res.RC)
	}
	if res.Success() {
		t.Fatal("超时不算成功：Success() 必须为 false")
	}
	if res.Signal != "" {
		t.Fatalf("超时只报 RC=%d，Signal 应为空，得到 %q", RCTimeout, res.Signal)
	}
	// 结果 → 错误 的桥：可 errors.Is 判，且三位事实都在。
	err = res.Err()
	if !errors.Is(err, ErrExecTimeout) {
		t.Fatalf("res.Err() 应可 errors.Is(ErrExecTimeout)，得到 %v", err)
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("res.Err() 应是 *ExitError，得到 %T", err)
	}
	if !ee.TimedOut || ee.RC != RCTimeout {
		t.Fatalf("ExitError 应带 TimedOut=true、RC=%d，得到 %+v", RCTimeout, *ee)
	}

	// ★ 负控：预设自己把超时写成 RC=0（零值）⇒ 后端必须拒，不许照抄给上层。
	bad := []string{"bad-timeout"}
	be.Script(bad, ExecResult{TimedOut: true})
	if _, err := be.Exec(ctx, inst, bad, ExecOpts{}); err == nil {
		t.Fatal("TimedOut=true 且 RC=0 的结果必须被拒（否则上层会把超时当成功）")
	} else if !errors.Is(err, ErrBadExecResult) {
		t.Fatalf("应报 ErrBadExecResult，得到 %v", err)
	}

	// 对照：没超时的零值结果就是真成功——证明上面的拒绝不是「一律拒 RC=0」。
	res, err = be.Exec(ctx, inst, []string{"true"}, ExecOpts{})
	if err != nil {
		t.Fatalf("无预设时应回放 fallback（成功空跑），得到错误: %v", err)
	}
	if !res.Success() || res.TimedOut || res.RC != 0 {
		t.Fatalf("fallback 应为成功零值，得到 %+v", res)
	}
}

// ── ③ destroy 幂等 ────────────────────────────────────────────────────

func TestDestroyIdempotent(t *testing.T) {
	ctx := context.Background()
	be, inst := bkProvision(t)

	if _, err := be.Expose(ctx, inst, 8000, ""); err != nil {
		t.Fatalf("Expose: %v", err)
	}

	// 第一次：真删。
	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("首次 Destroy: %v", err)
	}
	// 第二次：幂等（nil），且流水里记成 noop（可审计「重复删」不是没发生）。
	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("★ 二次 Destroy 必须幂等返回 nil，得到: %v", err)
	}
	calls := be.Calls()
	if len(calls) == 0 || !strings.HasSuffix(calls[len(calls)-1], "(noop)") {
		t.Fatalf("二次 Destroy 应记成 noop，流水末尾是 %q", calls[len(calls)-1])
	}

	// 已销毁：五动作里除 Destroy 外一律 ErrInstanceGone。
	if _, err := be.Exec(ctx, inst, []string{"true"}, ExecOpts{}); !errors.Is(err, ErrInstanceGone) {
		t.Fatalf("销毁后 Exec 应报 ErrInstanceGone，得到 %v", err)
	}
	if err := be.Mount(ctx, inst, "/host/x", "/x", true); !errors.Is(err, ErrInstanceGone) {
		t.Fatalf("销毁后 Mount 应报 ErrInstanceGone，得到 %v", err)
	}
	if _, err := be.Expose(ctx, inst, 8000, ""); !errors.Is(err, ErrInstanceGone) {
		t.Fatalf("销毁后 Expose 应报 ErrInstanceGone，得到 %v", err)
	}

	// 「幂等」不等于「什么都收」：从没 provision 过的 id 必须报 NotFound。
	if err := be.Destroy(ctx, Instance{ID: "fake-999", Backend: "fake"}); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("销毁不存在的实例应报 ErrInstanceNotFound，得到 %v", err)
	}
	if err := be.Destroy(ctx, Instance{}); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("销毁空句柄应报 ErrInstanceNotFound，得到 %v", err)
	}
	// 别的后端的句柄不许在本后端上删（防止跨后端误删）。
	if err := be.Destroy(ctx, Instance{ID: inst.ID, Backend: "nsjail"}); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("跨后端句柄应报 ErrInstanceNotFound，得到 %v", err)
	}

	// 端口随 destroy 回收：新实例能立刻拿回 20000（v2 §C「启动收端口」）。
	inst2, err := be.Provision(ctx, bkSpec(t))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	p, err := be.Expose(ctx, inst2, 8000, "")
	if err != nil {
		t.Fatalf("Expose: %v", err)
	}
	if p != FakeFirstHostPort {
		t.Fatalf("销毁后端口应可回收复用（期望 %d），得到 %d", FakeFirstHostPort, p)
	}
}

// ── ④ expose 分配不重复 ───────────────────────────────────────────────

func TestExposePortsUnique(t *testing.T) {
	ctx := context.Background()
	be, inst := bkProvision(t)

	seen := map[int]bool{}
	want := []int{FakeFirstHostPort, FakeFirstHostPort + 1, FakeFirstHostPort + 2}
	for i, innerPort := range []int{8000, 8001, 9000} {
		p, err := be.Expose(ctx, inst, innerPort, "")
		if err != nil {
			t.Fatalf("Expose(%d): %v", innerPort, err)
		}
		if p != want[i] {
			t.Fatalf("第 %d 个宿主端口应为 %d（从 %d 递增），得到 %d", i+1, want[i], FakeFirstHostPort, p)
		}
		if seen[p] {
			t.Fatalf("★ 宿主端口重复分配: %d", p)
		}
		seen[p] = true
	}

	// 同一 (innerPort,bind) 再来：幂等，返回同一端口（真后端也是这个语义）。
	p, err := be.Expose(ctx, inst, 8001, "")
	if err != nil {
		t.Fatalf("重复 Expose: %v", err)
	}
	if p != FakeFirstHostPort+1 {
		t.Fatalf("同键 Expose 应幂等返回 %d，得到 %d", FakeFirstHostPort+1, p)
	}
	// 同 innerPort 不同 bind：是另一条映射，必须新端口且不撞。
	p2, err := be.Expose(ctx, inst, 8001, "127.0.0.2")
	if err != nil {
		t.Fatalf("Expose(不同 bind): %v", err)
	}
	if seen[p2] {
		t.Fatalf("★ 不同 bind 的映射分到了已占端口 %d", p2)
	}

	// 第二个实例不与第一个撞端口。
	inst2, err := be.Provision(ctx, bkSpec(t))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	p3, err := be.Expose(ctx, inst2, 8000, "")
	if err != nil {
		t.Fatalf("Expose(inst2): %v", err)
	}
	if seen[p3] {
		t.Fatalf("★ 跨实例端口重复: %d", p3)
	}

	// 负例：非回环 bind、端口越界、号段用尽。
	if _, err := be.Expose(ctx, inst, 8080, "0.0.0.0"); !errors.Is(err, ErrBindNotLoopback) {
		t.Fatalf("非回环 bind 必须被拒（ErrBindNotLoopback），得到 %v", err)
	}
	if _, err := be.Expose(ctx, inst, 8080, "<cluster-ip>"); !errors.Is(err, ErrBindNotLoopback) {
		t.Fatalf("网卡地址 bind 必须被拒，得到 %v", err)
	}
	for _, bad := range []int{0, -1, 70000} {
		if _, err := be.Expose(ctx, inst, bad, ""); !errors.Is(err, ErrBadPort) {
			t.Fatalf("端口 %d 越界应报 ErrBadPort，得到 %v", bad, err)
		}
	}
	be.mu.Lock()
	be.nextPort = 65536 // 号段用尽（测试内直改，故不加锁断言）
	be.freePorts = nil  // 清掉可复用端口：要测的是「新号段没了」，不是「还能复用」
	be.mu.Unlock()
	if _, err := be.Expose(ctx, inst, 8081, ""); !errors.Is(err, ErrPortExhausted) {
		t.Fatalf("号段用尽应报 ErrPortExhausted，得到 %v", err)
	}
}

// ── ⑤ 三平台能力逐项断言 ──────────────────────────────────────────────

func TestCapabilitiesPerPlatform(t *testing.T) {
	// 期望值 = v3 §F 那张表，逐行抄下来；改表必须同时改这里（否则测试红）。
	cases := []struct {
		platform        string
		cap             Capabilities
		isolation       string
		peer            PeerIdentity
		processTreeKill bool
		symlinkNoFollow bool
		crossMountGuard bool
		killMech        string
	}{
		{
			platform: PlatformLinux, cap: LinuxCapabilities(), isolation: "nsjail",
			peer: PeerIdentityStrong, processTreeKill: true, symlinkNoFollow: true, crossMountGuard: true,
			killMech: "cgroup.kill",
		},
		{
			platform: PlatformDarwin, cap: DarwinCapabilities(), isolation: "VZ VM",
			peer: PeerIdentityWeak, processTreeKill: false, symlinkNoFollow: true, crossMountGuard: false,
			killMech: "kqueue NOTE_EXIT + killpg（无内核级等价物，弱于 cgroup.kill）",
		},
		{
			platform: PlatformWindows, cap: WindowsCapabilities(), isolation: "HCS(hyperv)",
			peer: PeerIdentityWeak, processTreeKill: true, symlinkNoFollow: false, crossMountGuard: false,
			killMech: "Job Object (KILL_ON_JOB_CLOSE)",
		},
	}

	for _, c := range cases {
		t.Run(c.platform, func(t *testing.T) {
			if err := c.cap.Validate(); err != nil {
				t.Fatalf("声明应自检通过: %v", err)
			}
			if c.cap.Platform != c.platform {
				t.Errorf("Platform = %q，期望 %q", c.cap.Platform, c.platform)
			}
			if c.cap.Isolation != c.isolation {
				t.Errorf("Isolation = %q，期望 %q", c.cap.Isolation, c.isolation)
			}
			if c.cap.PeerIdentity != c.peer {
				t.Errorf("PeerIdentity = %q，期望 %q", c.cap.PeerIdentity, c.peer)
			}
			if c.cap.PeerMechanism == "" {
				t.Error("PeerMechanism 不许为空（weak 必须写清弱在哪）")
			}
			if c.cap.ProcessTreeKill != c.processTreeKill {
				t.Errorf("ProcessTreeKill = %v，期望 %v", c.cap.ProcessTreeKill, c.processTreeKill)
			}
			if c.cap.KillMechanism != c.killMech {
				t.Errorf("KillMechanism = %q，期望 %q", c.cap.KillMechanism, c.killMech)
			}
			if c.cap.SymlinkNoFollow != c.symlinkNoFollow {
				t.Errorf("SymlinkNoFollow = %v，期望 %v", c.cap.SymlinkNoFollow, c.symlinkNoFollow)
			}
			if c.cap.CrossMountGuard != c.crossMountGuard {
				t.Errorf("CrossMountGuard = %v，期望 %v", c.cap.CrossMountGuard, c.crossMountGuard)
			}
			if c.cap.SymlinkMechanism == "" || c.cap.CrossMountMechanism == "" {
				t.Error("符号链接/跨挂载的机制字段不许为空（没有就写「待补」，不许留白）")
			}
			// 按名字取到的必须与显式入口一致（防止表与入口漂移）。
			got, ok := PlatformCapabilities(c.platform)
			if !ok {
				t.Fatalf("PlatformCapabilities(%q) 应命中", c.platform)
			}
			if got != c.cap {
				t.Fatalf("PlatformCapabilities(%q) 与显式入口不一致:\n got=%+v\nwant=%+v", c.platform, got, c.cap)
			}
		})
	}

	// 表里那两条「弱」的结论，必须能直接被编排层读出来（不许各层自己解释）。
	if LinuxCapabilities().PeerIdentity.PrivilegedOperationsAllowed() != true {
		t.Error("Linux(strong) 应允许高权操作")
	}
	for _, p := range []PeerIdentity{PeerIdentityWeak, PeerIdentityNone} {
		if p.PrivilegedOperationsAllowed() {
			t.Errorf("%q 必须 fail-closed：禁止高权操作（v3 §A）", p)
		}
	}
	if PeerIdentity("").Valid() || PeerIdentity("STRONG").Valid() {
		t.Error("PeerIdentity 大小写/空值都不算合法")
	}

	// 平台表本身：固定三个，顺序稳定；未知平台不给默认值。
	if got := Platforms(); !slices.Equal(got, []string{PlatformLinux, PlatformDarwin, PlatformWindows}) {
		t.Fatalf("Platforms() = %v", got)
	}
	if _, ok := PlatformCapabilities("plan9"); ok {
		t.Fatal("未知平台必须返回 ok=false（不许给个「默认全 true」）")
	}
	var zero Capabilities
	if err := zero.Validate(); err == nil {
		t.Fatal("零值能力声明应自检失败")
	}
	// 返回的是副本：调用方改了不影响表。
	c := LinuxCapabilities()
	c.ProcessTreeKill = false
	if !LinuxCapabilities().ProcessTreeKill {
		t.Fatal("Capabilities 必须按值返回（改副本不得污染声明表）")
	}
}

// ── ⑥ rc / 信号透传 ───────────────────────────────────────────────────

func TestExecRCAndSignalPassthrough(t *testing.T) {
	ctx := context.Background()
	be, inst := bkProvision(t)

	t.Run("正常退出码原样透传", func(t *testing.T) {
		argv := []string{"sh", "-c", "exit 7"}
		be.Script(argv, ExecResult{RC: 7, Stdout: []byte("partial\n"), MS: 3})
		res, err := be.Exec(ctx, inst, argv, ExecOpts{})
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		if res.RC != 7 {
			t.Fatalf("RC 应透传 7，得到 %d", res.RC)
		}
		if res.Signal != "" || res.TimedOut {
			t.Fatalf("非信号/非超时的结果不该带 Signal/TimedOut: %+v", res)
		}
		if res.Success() {
			t.Fatal("RC=7 不算成功")
		}
		if string(res.Stdout) != "partial\n" {
			t.Fatalf("非零退出也要保留已收到的输出，得到 %q", res.Stdout)
		}
		if !errors.Is(res.Err(), ErrExecNonZero) {
			t.Fatalf("应可 errors.Is(ErrExecNonZero)，得到 %v", res.Err())
		}
	})

	t.Run("被信号杀死", func(t *testing.T) {
		cases := []struct {
			argv   []string
			signal string
			signum int
		}{
			{[]string{"kill", "-9", "self"}, "SIGKILL", 9},
			{[]string{"segv"}, "SIGSEGV", 11},
			{[]string{"term"}, "SIGTERM", 15},
		}
		for _, c := range cases {
			be.Script(c.argv, ExecResult{RC: RCSignaled(c.signum), Signal: c.signal, Stderr: []byte("boom\n")})
			res, err := be.Exec(ctx, inst, c.argv, ExecOpts{})
			if err != nil {
				t.Fatalf("%v: Exec: %v", c.argv, err)
			}
			if res.Signal != c.signal {
				t.Fatalf("信号名应透传 %q，得到 %q", c.signal, res.Signal)
			}
			if res.RC != RCSignaled(c.signum) {
				t.Fatalf("RC 应为 128+%d=%d，得到 %d", c.signum, RCSignaled(c.signum), res.RC)
			}
			if res.RC == 0 || res.Success() {
				t.Fatalf("★ 被信号杀死不许表现为成功: %+v", res)
			}
			if res.TimedOut {
				t.Fatalf("信号杀不是超时: %+v", res)
			}
			err = res.Err()
			if !errors.Is(err, ErrExecSignaled) {
				t.Fatalf("应可 errors.Is(ErrExecSignaled)，得到 %v", err)
			}
			var ee *ExitError
			if !errors.As(err, &ee) || ee.Signal != c.signal || ee.RC != res.RC {
				t.Fatalf("ExitError 应带 Signal=%q、RC=%d，得到 %+v", c.signal, res.RC, err)
			}
		}
	})

	t.Run("stdout/stderr/ms 原样透传", func(t *testing.T) {
		argv := []string{"echo", "out"}
		be.Script(argv, ExecResult{RC: 0, Stdout: []byte("out\n"), Stderr: []byte("warn\n"), MS: 4242})
		res, err := be.Exec(ctx, inst, argv, ExecOpts{})
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		if string(res.Stdout) != "out\n" || string(res.Stderr) != "warn\n" || res.MS != 4242 {
			t.Fatalf("三件观测值应原样透传，得到 %+v", res)
		}
		if !res.Success() || res.Err() != nil {
			t.Fatalf("RC=0 且无信号/无超时应算成功，得到 %+v", res)
		}
	})

	t.Run("结果自检负例表", func(t *testing.T) {
		bad := []struct {
			name string
			res  ExecResult
			want string // 期望错误文案里出现的片段
		}{
			{"RC 越界（256）", ExecResult{RC: 256}, "越界"},
			{"RC 为负", ExecResult{RC: -1}, "越界"},
			{"MS 为负", ExecResult{MS: -1}, "为负"},
			{"超时但 RC=0", ExecResult{TimedOut: true}, "TimedOut=true"},
			{"超时却带信号名", ExecResult{TimedOut: true, RC: RCTimeout, Signal: "SIGKILL"}, "Signal 必须为空"},
			{"信号杀但 RC=0", ExecResult{RC: 0, Signal: "SIGKILL"}, "128+信号号"},
			{"信号杀但 RC 不是 128+n", ExecResult{RC: 1, Signal: "SIGKILL"}, "128+信号号"},
			{"信号名未带 SIG 前缀", ExecResult{RC: 137, Signal: "kill"}, "信号名非法"},
			{"信号名带小写", ExecResult{RC: 137, Signal: "Sigkill"}, "信号名非法"},
		}
		for _, c := range bad {
			if err := c.res.Validate(); err == nil {
				t.Errorf("%s: Validate 应失败，却通过了: %+v", c.name, c.res)
				continue
			} else if !errors.Is(err, ErrBadExecResult) {
				t.Errorf("%s: 应可 errors.Is(ErrBadExecResult)，得到 %v", c.name, err)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: 文案应含 %q，得到 %q", c.name, c.want, err.Error())
			}
			// 后端必须拒，不许照抄给上层。
			be.Script([]string{"bad-" + c.name}, c.res)
			if _, err := be.Exec(ctx, inst, []string{"bad-" + c.name}, ExecOpts{}); err == nil {
				t.Errorf("%s: 假后端应拒（ErrBadExecResult），却返回了结果", c.name)
			} else if !errors.Is(err, ErrBadExecResult) {
				t.Errorf("%s: 假后端应报 ErrBadExecResult，得到 %v", c.name, err)
			}
		}
		// 正控：这些形态是合法的，不许误杀。
		good := []ExecResult{
			{},                                  // 成功零值
			{RC: 137},                           // 137 当成真实退出码（非信号）也合法
			{RC: 0, Stdout: []byte("x"), MS: 1}, // 带输出
			{RC: RCSignaled(11), Signal: "SIGSEGV"},
		}
		for _, r := range good {
			if err := r.Validate(); err != nil {
				t.Errorf("合法结果被误拒: %+v → %v", r, err)
			}
		}
	})

	t.Run("入参自检", func(t *testing.T) {
		if err := (ExecOpts{Timeout: -time.Second}).Validate(); err == nil {
			t.Error("负超时应被拒")
		}
		if err := (ExecOpts{Dir: "rel/path"}).Validate(); err == nil {
			t.Error("相对工作目录应被拒（execve 前就该炸，别等到茧内）")
		}
		if err := (ExecOpts{Env: map[string]string{"BAD-KEY": "v"}}).Validate(); err == nil {
			t.Error("非法 env 键应被拒")
		}
		if err := (ExecOpts{Timeout: time.Second, Dir: "/app", Env: map[string]string{"A_B": "v"}}).Validate(); err != nil {
			t.Errorf("合法 opts 不该被拒: %v", err)
		}
		if _, err := be.Exec(ctx, inst, nil, ExecOpts{}); !errors.Is(err, ErrBadArgv) {
			t.Errorf("空 argv 应报 ErrBadArgv，得到 %v", err)
		}
		if _, err := be.Exec(ctx, inst, []string{""}, ExecOpts{}); !errors.Is(err, ErrBadArgv) {
			t.Errorf("空 argv[0] 应报 ErrBadArgv，得到 %v", err)
		}
	})

	t.Run("ctx 取消", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := be.Exec(cctx, inst, []string{"true"}, ExecOpts{}); !errors.Is(err, context.Canceled) {
			t.Errorf("ctx 取消应透出 context.Canceled，得到 %v", err)
		}
		if _, err := be.Provision(cctx, bkSpec(t)); !errors.Is(err, context.Canceled) {
			t.Errorf("Provision 应尊重 ctx，得到 %v", err)
		}
	})
}
