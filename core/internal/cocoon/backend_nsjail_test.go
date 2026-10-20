// backend_nsjail_test.go — nsjail 后端三格（+ env 防泄漏 + linux 记账面）：
//
//	① TestNsjailArgsFromContract   参数纯函数：契约 → 旗标（资源换算 / 回环与 proc / 缺省位）
//	② TestNsjailNonLinuxUnsupported 非 linux：五动作 + Capabilities 一律 ErrBackendUnsupported，且不 panic
//	③ TestNsjailResultContract      结果拼装：超时 RC=124、信号 128+n、正常退出透传，且过 Validate
//	③b TestNsjailCgroupPaths        cgroup 目录按 nsjail 真实布局（v2 <mount>/NSJAIL.<pid>）
//	④ TestNsjailEnvNoHostInherit    env 只由「最小 PATH + 契约 [环境] + 调用方 env」合成
//	⑤ TestNsjailLinuxBookkeeping    linux 记账面（本机跳过 · **未在本机跑过**）：建目录/挂载冲突/
//	                               Expose 拒/Destroy 幂等/nsjail 缺失时报错而非假结果
//
// ★ 本机（macOS）跑不了 nsjail ⇒ 第①②③④格都不许依赖「真起进程」：①③④打的是纯函数，
// ②打的是「没能力时要诚实报错」。真机跑通的证据只能来自 Linux 主机（见文件尾注）。
package cocoon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ── 小工具 ────────────────────────────────────────────────────────────

// argValue — 取旗标后面的那个值（找不到返回 ok=false）。
func argValue(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

// argCount — 某旗标出现几次（挂载类旗标会重复出现）。
func argCount(args []string, flag string) int {
	n := 0
	for _, a := range args {
		if a == flag {
			n++
		}
	}
	return n
}

// envValue — 从 "K=V" 环境列表里取值（env 不是「旗标 + 值」对，不能用 argValue）。
func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return strings.TrimPrefix(kv, key+"="), true
		}
	}
	return "", false
}

// ── ① 参数纯函数 ──────────────────────────────────────────────────────

func TestNsjailArgsFromContract(t *testing.T) {
	// 用 spec_test.go 的完整样例（cpu=2, mem_gb=4）——真解析过的契约，不是手搓 Spec{}。
	sp := mustParseSpec(t, specFull)
	if sp.Resources.CPU != 2 || sp.Resources.MemGB != 4 {
		t.Fatalf("夹具契约资源不合预期：%+v", sp.Resources)
	}

	args := nsjailArgs(sp)

	// 资源换算：内存两处单位不同（cgroup 字节 / rlimit_as MB），CPU 两处（核数 / 每秒毫秒）。
	if got, ok := argValue(args, "--cgroup_mem_max"); !ok || got != "4294967296" {
		t.Fatalf("--cgroup_mem_max = %q ok=%v，want %q（4GiB 的字节数）", got, ok, "4294967296")
	}
	if got, ok := argValue(args, "--rlimit_as"); !ok || got != "4096" {
		t.Fatalf("--rlimit_as = %q ok=%v，want %q（4GiB 的 MB 数）", got, ok, "4096")
	}
	if got, ok := argValue(args, "--cgroup_cpu_ms_per_sec"); !ok || got != "2000" {
		t.Fatalf("--cgroup_cpu_ms_per_sec = %q ok=%v，want %q", got, ok, "2000")
	}
	if got, ok := argValue(args, "--max_cpus"); !ok || got != "2" {
		t.Fatalf("--max_cpus = %q ok=%v，want %q", got, ok, "2")
	}
	if got, ok := argValue(args, "--rlimit_nofile"); !ok || got != strconv.Itoa(nsjailDefaultNoFile) {
		t.Fatalf("--rlimit_nofile = %q ok=%v，want %d", got, ok, nsjailDefaultNoFile)
	}

	// 一次性模式 + 不让 nsjail 自己 600s 墙钟杀进程 + 环境透传。
	if got, ok := argValue(args, "--mode"); !ok || got != "o" {
		t.Fatalf("--mode = %q ok=%v，want o", got, ok)
	}
	if got, ok := argValue(args, "--time_limit"); !ok || got != "0" {
		t.Fatalf("--time_limit = %q ok=%v，want 0（超时只认 ExecOpts.Timeout）", got, ok)
	}
	if argCount(args, "--keep_env") != 1 {
		t.Fatalf("应恰好带一次 --keep_env，得到 args=%v", args)
	}

	// ★ 三条「有意为空」：传了就走反方向（无回环 / 无 /proc / 直通宿主网络）。
	for _, banned := range []string{"--iface_no_lo", "--disable_proc", "--disable_clone_newnet"} {
		if argCount(args, banned) != 0 {
			t.Fatalf("不该出现 %s（会把茧的回环/proc/netns 弄反），args=%v", banned, args)
		}
	}

	// 纯函数：同一 spec 两次调用必须逐字相等。
	if again := nsjailArgs(sp); !slices.Equal(args, again) {
		t.Fatalf("同一 spec 两次拼出不同 argv：\n%v\n%v", args, again)
	}

	// 每个旗标都必须有值（`--x` 结尾少值 = nsjail 起不来）。
	if len(args)%2 != 0 && args[len(args)-1] != "--keep_env" {
		t.Fatalf("argv 尾部落单，args=%v", args)
	}
}

func TestNsjailArgsFractionalAndNil(t *testing.T) {
	// 小数核/小数 GB：核数向上取整（0.5 核也要给 1 个 CPU），毫秒与 MB 按四舍五入。
	sp := &Spec{Resources: ResourceSpec{CPU: 1.5, MemGB: 0.5, DiskGB: 1}}
	args := nsjailArgs(sp)

	if got, _ := argValue(args, "--max_cpus"); got != "2" {
		t.Fatalf("1.5 核 --max_cpus = %q，want 2（向上取整）", got)
	}
	if got, _ := argValue(args, "--cgroup_cpu_ms_per_sec"); got != "1500" {
		t.Fatalf("1.5 核 --cgroup_cpu_ms_per_sec = %q，want 1500", got)
	}
	if got, _ := argValue(args, "--cgroup_mem_max"); got != "536870912" {
		t.Fatalf("0.5GB --cgroup_mem_max = %q，want 536870912", got)
	}
	if got, _ := argValue(args, "--rlimit_as"); got != "512" {
		t.Fatalf("0.5GB --rlimit_as = %q，want 512", got)
	}

	// 资源全 0（缺省即无上限）：不给死限额，但不许 panic 也不许拼出 0 值旗标（0 = 不限，
	// 写出来会把「没设」伪装成「设了个 0」）。
	zero := nsjailArgs(&Spec{})
	for _, flag := range []string{"--cgroup_mem_max", "--rlimit_as", "--max_cpus", "--cgroup_cpu_ms_per_sec"} {
		if argCount(zero, flag) != 0 {
			t.Fatalf("资源为 0 时不该出现 %s，args=%v", flag, zero)
		}
	}

	// nil spec：只报路径/模式/env，不 panic。
	nilArgs := nsjailArgs(nil)
	if _, ok := argValue(nilArgs, "--mode"); !ok {
		t.Fatalf("nil spec 也要给出 --mode，args=%v", nilArgs)
	}
	if argCount(nilArgs, "--cgroup_mem_max") != 0 {
		t.Fatalf("nil spec 不该有资源旗标，args=%v", nilArgs)
	}
}

// ── ①b 挂载与 argv 边界 ───────────────────────────────────────────────

func TestNsjailMountArgsAndExecArgv(t *testing.T) {
	mounts := map[string]nsjailMount{
		"/data":            {Host: "/srv/data", Inner: "/data", RO: false},
		"/app":             {Host: "/srv/app", Inner: "/app", RO: true},
		"/app/config.toml": {Host: "/host/cocoon.toml", Inner: "/app/config.toml", RO: true},
	}
	got := nsjailMountArgs(mounts)

	// 按 inner 升序（输出稳定，日志/测试可逐字比）。
	want := []string{
		"--bindmount_ro", "/srv/app:/app",
		"--bindmount_ro", "/host/cocoon.toml:/app/config.toml",
		"--bindmount", "/srv/data:/data",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("挂载旗标不符：\n got=%v\nwant=%v", got, want)
	}

	argv := []string{"python", "-c", "print(1)", "--bindmount", "/:/"}
	full := nsjailExecArgv(&Spec{Resources: ResourceSpec{CPU: 1, MemGB: 1, DiskGB: 1}},
		mounts, "/", "/app", argv)

	// 用户 argv 必须**逐字**落在 `--` 之后：`--bindmount /:/` 只能是茧内命令的参数，
	// 一旦漏了 `--`，它就成了 nsjail 的旗标（茧的挂载面被用户命令改掉）。
	sep := slices.Index(full, "--")
	if sep < 0 {
		t.Fatalf("argv 里没有 `--` 分隔符：%v", full)
	}
	if !slices.Equal(full[sep+1:], argv) {
		t.Fatalf("`--` 之后应逐字等于用户 argv：\n got=%v\nwant=%v", full[sep+1:], argv)
	}
	if slices.Contains(full[:sep], "/:/") {
		t.Fatalf("用户 argv 泄漏到 `--` 之前：%v", full[:sep])
	}
	// --chroot 必给：没它 nsjail 只挂 /proc，茧内连 /bin/sh 都找不到（实测退 255）。
	if got, ok := argValue(full[:sep], "--chroot"); !ok || got != "/" {
		t.Fatalf("--chroot = %q ok=%v，want /", got, ok)
	}
	if got, ok := argValue(full[:sep], "--cwd"); !ok || got != "/app" {
		t.Fatalf("--cwd = %q ok=%v，want /app", got, ok)
	}

	// 无挂载、无 dir：`--` 之前只剩模式/限额类旗标。
	bare := nsjailExecArgv(nil, nil, "/", "", []string{"true"})
	if got, ok := argValue(bare, "--cwd"); ok {
		t.Fatalf("dir 为空时不该有 --cwd，得到 %q", got)
	}
	if bare[len(bare)-2] != "--" || bare[len(bare)-1] != "true" {
		t.Fatalf("尾部应是 `-- true`，得到 %v", bare[len(bare)-2:])
	}
}

// ── ①c 网络位：HostNetwork ⇒ `--disable_clone_newnet` 落 `--` 之前 ─────────

func TestNsjailHostNetworkFlag(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "true"}
	base := nsjailExecArgv(nil, nil, "/", "", argv)
	if argCount(base, "--disable_clone_newnet") != 0 {
		t.Fatalf("缺省（独立 netns）不该带 --disable_clone_newnet：%v", base)
	}

	with := nsjailWithHostNet(base)
	if argCount(with, "--disable_clone_newnet") != 1 {
		t.Fatalf("HostNetwork=true 应恰好带一次 --disable_clone_newnet：%v", with)
	}
	// ★ 必须在 `--` 之前：插到后面就只是茧内命令的一个参数（网络位形同虚设）。
	sep := -1
	for i, a := range with {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		t.Fatalf("argv 里没有 `--`：%v", with)
	}
	inserted := -1
	for i, a := range with[:sep] {
		if a == "--disable_clone_newnet" {
			inserted = i
		}
	}
	if inserted < 0 {
		t.Fatalf("--disable_clone_newnet 落到了 `--` 之后（成了茧内命令的参数）：%v", with)
	}
	// 用户 argv 逐字不变，且仍在 `--` 之后。
	if !slices.Equal(with[sep+1:], argv) {
		t.Fatalf("`--` 之后的 argv 被改动了：\n got=%v\nwant=%v", with[sep+1:], argv)
	}
	// 纯函数：不改入参（两次调用不叠旗标）。
	if argCount(nsjailWithHostNet(with), "--disable_clone_newnet") != 2 {
		t.Fatal("插旗函数应当只做一次插入（幂等性由调用点保证，这里只验它不就地改入参）")
	}
	if argCount(base, "--disable_clone_newnet") != 0 {
		t.Fatalf("nsjailWithHostNet 就地改了入参：%v", base)
	}
}

// ── ①d argv[0] 解析（nsjail 不走 PATH，实测退 255）────────────────────

func TestNsjailResolveArgv0(t *testing.T) {
	// 绝对路径：原样返回（不碰盘、不改写）。
	abs := []string{"/bin/sh", "-c", "true"}
	if got, err := nsjailResolveArgv0(abs, nsjailDefaultPATH, "/"); err != nil || !slices.Equal(got, abs) {
		t.Fatalf("绝对路径应当原样返回：got=%v err=%v", got, err)
	}

	// 裸命令名：按茧内 PATH 解析（用本机真实存在的命令，解析 = 宿主 stat，chroot="/" 时等价）。
	got, err := nsjailResolveArgv0([]string{"sh", "-c", "true"}, nsjailDefaultPATH, "/")
	if err != nil {
		t.Fatalf("裸命令名 sh 应当能解析：%v", err)
	}
	if !strings.Contains(got[0], "/") || filepath.Base(got[0]) != "sh" || !strings.HasPrefix(got[0], "/") {
		t.Fatalf("解析结果应当是绝对路径且 basename 不变：%v", got)
	}
	// 参数逐字不变（只换 argv[0]）。
	if !slices.Equal(got[1:], []string{"-c", "true"}) {
		t.Fatalf("argv[1:] 被改动了：%v", got[1:])
	}
	// 不改入参。
	if abs[0] != "/bin/sh" {
		t.Fatal("nsjailResolveArgv0 就地改了入参")
	}

	// 找不到 ⇒ ErrBadArgv（不是「交给 nsjail 退 255 让人猜」）。
	if _, err := nsjailResolveArgv0([]string{"definitely-not-here-xyz"}, nsjailDefaultPATH, "/"); !errors.Is(err, ErrBadArgv) {
		t.Fatalf("找不到的裸命令名应 ErrBadArgv，得到 %v", err)
	}
	// 换了 rootfs ⇒ 裸命令名一律拒（宿主 stat 证明不了茧内有什么）。
	if _, err := nsjailResolveArgv0([]string{"sh"}, nsjailDefaultPATH, "/some/rootfs"); !errors.Is(err, ErrBadArgv) {
		t.Fatalf("非 / 的 chroot 下裸命令名应 ErrBadArgv，得到 %v", err)
	}
	// 空 argv。
	if _, err := nsjailResolveArgv0(nil, nsjailDefaultPATH, "/"); !errors.Is(err, ErrBadArgv) {
		t.Fatalf("空 argv 应 ErrBadArgv，得到 %v", err)
	}
	// 契约 [环境].PATH 优先。
	sp := &Spec{Env: map[string]string{"PATH": "/opt/x/bin:/bin"}}
	if got := nsjailJailPATH(sp); got != "/opt/x/bin:/bin" {
		t.Fatalf("nsjailJailPATH 没优先用契约 PATH：%q", got)
	}
	if got := nsjailJailPATH(&Spec{}); got != nsjailDefaultPATH {
		t.Fatalf("契约没写 PATH 时应当用默认 %q，得到 %q", nsjailDefaultPATH, got)
	}
}

// ── ② 非 linux：诚实报不支持 ─────────────────────────────────────────

func TestNsjailNonLinuxUnsupported(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("linux 上这份后端是真实现，本格只验非 linux 的退让")
	}

	ctx := context.Background()
	be := NewNsjailBackend()
	if be.Name() != "nsjail" {
		t.Fatalf("Name() = %q，want nsjail", be.Name())
	}

	// 五个动作一个都不许「假装成功」；也不许 panic（类型在、能力不在）。
	if inst, err := be.Provision(ctx, mustParseSpec(t, specFull)); !errors.Is(err, ErrBackendUnsupported) || !inst.IsZero() {
		t.Fatalf("Provision：inst=%+v err=%v，want 零值 + ErrBackendUnsupported", inst, err)
	}
	// nil 契约也走不支持（不是 ErrNilSpec）：本平台连「收契约」这一步都不做。
	if _, err := be.Provision(ctx, nil); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Provision(nil)：err=%v，want ErrBackendUnsupported", err)
	}
	if _, err := be.Exec(ctx, Instance{ID: "x", Backend: "nsjail"}, []string{"true"}, ExecOpts{}); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Exec：err=%v，want ErrBackendUnsupported", err)
	}
	if err := be.Mount(ctx, Instance{ID: "x", Backend: "nsjail"}, "/a", "/a", true); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Mount：err=%v，want ErrBackendUnsupported", err)
	}
	if port, err := be.Expose(ctx, Instance{ID: "x", Backend: "nsjail"}, 8000, ""); !errors.Is(err, ErrBackendUnsupported) || port != 0 {
		t.Fatalf("Expose：port=%d err=%v，want 0 + ErrBackendUnsupported", port, err)
	}
	// Destroy 也不许 nil：这台机器上什么都没 provision 过，nil 会被读成「销毁成功」。
	if err := be.Destroy(ctx, Instance{ID: "x", Backend: "nsjail"}); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Destroy：err=%v，want ErrBackendUnsupported", err)
	}
	if caps, err := be.Capabilities(); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Capabilities：err=%v，want ErrBackendUnsupported", err)
	} else if caps.Platform != "" {
		t.Fatalf("Capabilities 不许给出「全 false 的表」（会被误读成后端在但弱）：%+v", caps)
	}
}

// ── ③ 结果契约（超时 124 / 信号 128+n / 正常透传）────────────────────

func TestNsjailResultContract(t *testing.T) {
	out := []byte("stdout-bytes")
	errb := []byte("stderr-bytes")

	cases := []struct {
		name     string
		exitCode int
		signum   int
		timedOut bool
		wantRC   int
		wantSig  string
	}{
		{"正常退出 0", 0, 0, false, 0, ""},
		{"非零退出透传", 3, 0, false, 3, ""},
		{"SIGKILL 杀", 0, 9, false, RCSignalBase + 9, "SIGKILL"},
		{"SIGTERM 杀", 0, 15, false, RCSignalBase + 15, "SIGTERM"},
		{"SIGSEGV 杀", 0, 11, false, 139, "SIGSEGV"},
		{"超时", 0, 0, true, RCTimeout, ""},
		// 超时压过信号/退出码：先超时、随后 SIGTERM 才生效，结论只能报 124。
		{"超时且已收到信号", 0, 15, true, RCTimeout, ""},
		{"超时但进程先退 0", 0, 0, true, RCTimeout, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := nsjailResult(tc.exitCode, tc.signum, tc.timedOut, out, errb, 42)
			if err != nil {
				t.Fatalf("拼结果应当通过自检，得到错误: %v", err)
			}
			if res.RC != tc.wantRC {
				t.Fatalf("RC = %d，want %d", res.RC, tc.wantRC)
			}
			if res.Signal != tc.wantSig {
				t.Fatalf("Signal = %q，want %q", res.Signal, tc.wantSig)
			}
			if res.TimedOut != tc.timedOut {
				t.Fatalf("TimedOut = %v，want %v", res.TimedOut, tc.timedOut)
			}
			if res.MS != 42 {
				t.Fatalf("MS = %d，want 42", res.MS)
			}
			// 超时也要把已收到的输出留住（丢了就没法定位为什么超时）。
			if !slices.Equal(res.Stdout, out) || !slices.Equal(res.Stderr, errb) {
				t.Fatalf("输出没保留：stdout=%q stderr=%q", res.Stdout, res.Stderr)
			}
			// 再走一遍契约自检：拼出来的东西必须自己就能过 Validate。
			if err := res.Validate(); err != nil {
				t.Fatalf("Validate 拒了自己拼的结果: %v", err)
			}
			// 超时绝不等于成功。
			if tc.timedOut && res.Success() {
				t.Fatalf("超时不许报成功：%+v", res)
			}
			// Success 与 Err() 两位事实必须一致。
			if (res.Err() == nil) != res.Success() {
				t.Fatalf("Success()=%v 与 Err()=%v 不一致", res.Success(), res.Err())
			}
		})
	}

	// 信号名：非常见/实时信号也要拼出合法形态（`SIG<n>` / `SIGRT<n>`），否则 Validate 会拒。
	for _, sig := range []int{9, 15, 31, 40, 64} {
		name := nsjailSignalName(sig)
		if !validSignalName(name) {
			t.Fatalf("信号 %d 拼出的名字 %q 不合 validSignalName", sig, name)
		}
		if _, err := nsjailResult(0, sig, false, nil, nil, 0); err != nil {
			t.Fatalf("信号 %d（%s）拼结果失败: %v", sig, name, err)
		}
	}

	// 负例：非法退出码不许被「修好」，要原样被 Validate 挡住。
	if _, err := nsjailResult(300, 0, false, nil, nil, 0); !errors.Is(err, ErrBadExecResult) {
		t.Fatalf("退出码 300 应当被 ErrBadExecResult 拒，得到 %v", err)
	}

	// 结果里的缓冲不与调用方共享（后端交了结果之后还能继续写自己的 buffer）。
	buf := []byte("abc")
	res, err := nsjailResult(0, 0, false, buf, nil, 1)
	if err != nil {
		t.Fatalf("nsjailResult: %v", err)
	}
	buf[0] = 'z'
	if strings.HasPrefix(string(res.Stdout), "z") {
		t.Fatal("ExecResult.Stdout 与调用方缓冲共享底层数组（应当拷贝）")
	}
}

// ── ③b cgroup 路径按 nsjail 真实布局 ─────────────────────────────────

func TestNsjailCgroupPaths(t *testing.T) {
	// v2（cgroup2.cc: getCgroupPath）与 v1（cgroup.cc: mem_parent）两种布局都要给出，
	// 且**必须**以 pid 结尾：写错一层就会把 cgroup.kill 写到别人的 cgroup 上。
	got := nsjailCgroupPaths("/sys/fs/cgroup", 4242)
	want := []string{"/sys/fs/cgroup/NSJAIL.4242", "/sys/fs/cgroup/NSJAIL/NSJAIL.4242"}
	if !slices.Equal(got, want) {
		t.Fatalf("cgroup 路径不符 nsjail 布局：\n got=%v\nwant=%v", got, want)
	}
	// 没配挂载点 / pid 未知 ⇒ 一条都不给（宁可不写，也不写到 /NSJAIL.<pid> 这种相对路径上）。
	if p := nsjailCgroupPaths("", 4242); p != nil {
		t.Fatalf("mount 为空应无路径，得到 %v", p)
	}
	if p := nsjailCgroupPaths("/sys/fs/cgroup", 0); p != nil {
		t.Fatalf("pid=0 应无路径，得到 %v", p)
	}
}

// ── ④ env 不继承宿主 ─────────────────────────────────────────────────

func TestNsjailEnvNoHostInherit(t *testing.T) {
	sp := mustParseSpec(t, specFull)
	env := nsjailEnv(sp, map[string]string{"EXTRA": "1", "LANG": "override"})

	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "\x00") {
		t.Fatalf("env 里混进 NUL：%q", joined)
	}
	// 键序固定（升序），便于逐字比。
	if !slices.IsSorted(env) {
		t.Fatalf("env 未按升序排：%v", env)
	}
	// 契约 [环境] 的值都在；调用方 env 覆盖同名键。
	if got, ok := envValue(env, "TZ"); !ok || got != "Asia/Shanghai" {
		t.Fatalf("TZ 未从契约带入：%v", env)
	}
	if got, ok := envValue(env, "LANG"); !ok || got != "override" {
		t.Fatalf("调用方 env 未覆盖契约：%v", env)
	}
	if got, ok := envValue(env, "EXTRA"); !ok || got != "1" {
		t.Fatalf("调用方 env 未带入：%v", env)
	}
	if got, ok := envValue(env, "PATH"); !ok || got != nsjailDefaultPATH {
		t.Fatalf("PATH 应为最小默认值 %q，得到 %q", nsjailDefaultPATH, got)
	}
	// 宿主里设了但契约里没写的变量，不许出现（宿主 env 可能带密钥）。
	t.Setenv("ZERG_HOST_SECRET", "leak")
	if env := nsjailEnv(nil, nil); slices.Contains(env, "ZERG_HOST_SECRET=leak") {
		t.Fatalf("宿主环境被继承进茧：%v", env)
	}
}

// ── ⑤ linux 上的记账面（不需要 nsjail、不需要特权；本机跳过，未在本机跑过）──
//
// 这一格刻意只用 t.TempDir()，**不碰 /sys/fs/cgroup**（那要 root），所以任何 Linux 主机
// 上都能跑。它打的是「真实现的状态机」：Provision 建目录、Mount 冲突拒、Expose 拒、
// Destroy 幂等且清目录、nsjail 不在时 Exec 报「找不到」而不是 panic。
func TestNsjailLinuxBookkeeping(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("只验 linux 实现；本机（" + runtime.GOOS + "）上这份后端是 ErrBackendUnsupported 的桩")
	}

	ctx := context.Background()
	be := NewNsjailBackend()
	be.BaseDir = t.TempDir()
	be.Path = "/nonexistent/nsjail-for-test"

	inst, err := be.Provision(ctx, mustParseSpec(t, specFull))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if inst.Backend != "nsjail" || inst.ID == "" || inst.Root == "" {
		t.Fatalf("Instance 不合预期：%+v", inst)
	}
	if _, err := os.Stat(inst.Root); err != nil {
		t.Fatalf("茧根没建出来：%v", err)
	}

	if err := be.Mount(ctx, inst, "/srv/app", "/app", true); err != nil {
		t.Fatalf("Mount(ro): %v", err)
	}
	if err := be.Mount(ctx, inst, "/srv/app", "/app", true); err != nil {
		t.Fatalf("同源同模式重复 Mount 应当幂等：%v", err)
	}
	if err := be.Mount(ctx, inst, "/srv/other", "/app", true); !errors.Is(err, ErrMountConflict) {
		t.Fatalf("同 inner 不同源应 ErrMountConflict，得到 %v", err)
	}
	if err := be.Mount(ctx, inst, "relative/path", "/x", true); !errors.Is(err, ErrBadMount) {
		t.Fatalf("相对路径应 ErrBadMount，得到 %v", err)
	}

	if _, err := be.Expose(ctx, inst, 8000, ""); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Expose 应 ErrBackendUnsupported（Linux 不映射端口），得到 %v", err)
	}
	if _, err := be.Expose(ctx, inst, 0, ""); !errors.Is(err, ErrBadPort) {
		t.Fatalf("越界端口应 ErrBadPort，得到 %v", err)
	}

	// TTY=true：本后端没有 pty ⇒ 拒，且不许静默当普通管道跑。
	if _, err := be.Exec(ctx, inst, []string{"true"}, ExecOpts{TTY: true}); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("TTY=true 应 ErrBackendUnsupported，得到 %v", err)
	}

	// nsjail 不在 ⇒ 「结果拿不到」⇒ 必须返回 err（不是 RC 非零的假结果）。
	if _, err := be.Exec(ctx, inst, []string{"true"}, ExecOpts{}); err == nil || !strings.Contains(err.Error(), "nsjail") {
		t.Fatalf("nsjail 缺失时 Exec 应报「找不到 nsjail」，得到 %v", err)
	}

	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(inst.Root); !os.IsNotExist(err) {
		t.Fatalf("Destroy 后茧根应当没了，stat err=%v", err)
	}
	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("Destroy 幂等：第二次应当 nil，得到 %v", err)
	}
	if _, err := be.Exec(ctx, inst, []string{"true"}, ExecOpts{}); !errors.Is(err, ErrInstanceGone) {
		t.Fatalf("已销毁实例 Exec 应 ErrInstanceGone，得到 %v", err)
	}
	if _, err := be.Exec(ctx, Instance{ID: inst.ID, Backend: "fake"}, []string{"true"}, ExecOpts{}); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("后端名不符应 ErrInstanceNotFound，得到 %v", err)
	}

	// cgroup 空壳补删（Destroy 的第②步走同一个函数）：临时挂载点下放一个假的 NSJAIL.<pid>，
	// 删掉才算「清 cgroup 目录」；再删一次也要无错（幂等）。
	cgMount := t.TempDir()
	cgDir := filepath.Join(cgMount, "NSJAIL.4242")
	if err := os.MkdirAll(cgDir, 0o755); err != nil {
		t.Fatalf("造 cgroup 空壳失败: %v", err)
	}
	if errs := removeCgroupDirs(cgMount, 4242); len(errs) != 0 {
		t.Fatalf("清 cgroup 目录报错: %v", errs)
	}
	if _, err := os.Stat(cgDir); !os.IsNotExist(err) {
		t.Fatalf("cgroup 空壳应当被删掉，stat err=%v", err)
	}
	if errs := removeCgroupDirs(cgMount, 4242); len(errs) != 0 {
		t.Fatalf("cgroup 目录已不在时应无错（幂等），得到 %v", errs)
	}

	caps, err := be.Capabilities()
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	// v3 §F 的 linux 行四项真 + 回收机制署名。
	if caps.PeerIdentity != PeerIdentityStrong || !caps.ProcessTreeKill || !caps.SymlinkNoFollow || !caps.CrossMountGuard {
		t.Fatalf("能力声明不符 §F：%+v", caps)
	}
	if caps.KillMechanism != "cgroup.kill" || caps.Isolation != "nsjail" {
		t.Fatalf("KillMechanism/Isolation 不符：%+v", caps)
	}
	// 端口面：nsjail 不做 DNAT ⇒ Expose 必须是 false，且理由写清（控制面据此跳过 [[端口]]）。
	if caps.Expose {
		t.Fatalf("linux/nsjail 的 Expose 应为 false，得到 %+v", caps)
	}
	if caps.ExposeMechanism == "" {
		t.Fatal("Expose=false 时 ExposeMechanism 不许为空")
	}
}

// 注：真机（Linux + nsjail）跑通的证据不在本文件 —— 本机是 macOS，起不了 nsjail。
// 交叉编译（GOOS=linux go build ./... && go vet）只证明「能在 linux 上编译」，
// 不证明「运行时行为对」。这一点在交付说明里逐条登记，不当已做。

// ── ⑥ 真 nsjail 集成格（默认跳过；本机 macOS 无 nsjail，必须显式打开）──
//
// 打开方式（Linux + root + nsjail 已装）：
//
//	ZERG_NSJAIL_IT=1 [ZERG_NSJAIL_BIN=/path/to/nsjail] go test ./internal/cocoon -run TestNsjailRealExec -v
//
// ★ cgroup v2 主机上 nsjail 还要求「能写 root cgroup」：容器里跑要给 `--cgroupns=host`，
// 否则 nsjail 会在 enableCgroupSubtree 处退 255（实测：Docker 缺省 cgroupns 下必现）。
// ★ 默认跳过不是我偷懒：这条测试会真起进程、真写 cgroup、真杀进程树，CI 上不该隐式发生。
func TestNsjailRealExec(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("本格只跑 linux（本机 " + runtime.GOOS + "）")
	}
	if os.Getenv("ZERG_NSJAIL_IT") != "1" {
		t.Skip("需真 nsjail：置 ZERG_NSJAIL_IT=1 打开（见本函数注释）")
	}

	ctx := context.Background()
	be := NewNsjailBackend()
	if bin := os.Getenv("ZERG_NSJAIL_BIN"); bin != "" {
		be.Path = bin
	}
	be.BaseDir = t.TempDir()
	be.KillGrace = 2 * time.Second

	inst, err := be.Provision(ctx, mustParseSpec(t, specFull))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	defer func() {
		if err := be.Destroy(ctx, inst); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
	}()

	// 共享工作目录：宿主侧临时目录 rw 挂进茧里（argv 里的 pid 文件、ro 探测都靠它）。
	work := t.TempDir()
	if err := be.Mount(ctx, inst, work, "/work", false); err != nil {
		t.Fatalf("Mount(rw): %v", err)
	}

	// ① 正常退出：RC 透传 + stdout 拿到（argv 逐字过 `--` 的证据）。
	res, err := be.Exec(ctx, inst, []string{"/bin/sh", "-c", "echo hello-from-jail; exit 7"},
		ExecOpts{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("Exec(正常退出): %v", err)
	}
	if res.RC != 7 || !strings.Contains(string(res.Stdout), "hello-from-jail") {
		t.Fatalf("正常退出：RC=%d stdout=%q stderr=%q", res.RC, res.Stdout, res.Stderr)
	}

	// ② 茧内进程被信号杀：RC 必须是 128+n（绝不许是 0）。
	//    ★ 实测事实（写在这里免得后人再猜）：nsjail 的茧内 init 收不到「别的进程发给它的」
	//    未处理信号（内核保护 pid 1），所以这里用 init **自己**吃 SIGINT 的形态；nsjail 本身
	//    以 128+n 正常退出 ⇒ 我们只看到 RC=130、Signal=""（Signal 只报我们对 nsjail 自身的观测，
	//    不猜茧内那层 —— 见 Exec 的说明）。
	res, err = be.Exec(ctx, inst, []string{"/bin/sh", "-c", "kill -INT $$"},
		ExecOpts{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("Exec(信号): %v", err)
	}
	if res.RC != RCSignaled(2) || res.Success() {
		t.Fatalf("信号杀：RC=%d Success=%v，want RC=%d 且不算成功", res.RC, res.Success(), RCSignaled(2))
	}

	// ③ 超时：TimedOut=true 且 RC=124（绝不许是 0），且确实在超时附近返回。
	start := time.Now()
	res, err = be.Exec(ctx, inst, []string{"/bin/sleep", "30"}, ExecOpts{Timeout: time.Second})
	if err != nil {
		t.Fatalf("Exec(超时): %v", err)
	}
	if !res.TimedOut || res.RC != RCTimeout || res.Signal != "" {
		t.Fatalf("超时：TimedOut=%v RC=%d Signal=%q，want true/%d/\"\"", res.TimedOut, res.RC, res.Signal, RCTimeout)
	}
	if el := time.Since(start); el > 20*time.Second {
		t.Fatalf("超时叫停太慢：%s（KillGrace=2s，应当远小于此）", el)
	}

	// ④ 整棵进程树：后台孙进程（死循环心跳写手）在超时后必须一起死 —— **而且要 setsid 跑掉**，
	//    这样 kill(-pgid) 抓不到它（它已离开进程组），只有「pidns 拆除 / cgroup.kill」能收。
	//    ★ 实测补充（免得后人把它当机制证明）：关掉 cgroup.kill 时同一个场景**也**收得干净 ——
	//    真正把树清掉的是「茧 init 死后内核拆 pid 名字空间」。所以本格断言的是**可观测性质**
	//    （树必须死），不是「cgroup.kill 在起作用」；机制排序见 killTree 的注释。
	//    ★ 用「心跳文件还在不在长」判活、不按 pid 判：茧内 pid 与宿主 pid 不是一套（pidns）。
	beat := filepath.Join(work, "beat")
	res, err = be.Exec(ctx, inst, []string{"/bin/sh", "-c",
		"setsid sh -c 'while : ; do echo x >> /work/beat; sleep 0.05; done' & wait"}, ExecOpts{Timeout: time.Second})
	if err != nil {
		t.Fatalf("Exec(孙进程): %v", err)
	}
	if !res.TimedOut {
		t.Fatalf("应当超时，得到 %+v", res)
	}
	s1, err := os.Stat(beat)
	if err != nil {
		t.Fatalf("心跳文件没写出来（rw 挂载或 argv 没生效？）: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	s2, err := os.Stat(beat)
	if err != nil {
		t.Fatalf("心跳文件 stat: %v", err)
	}
	if s2.Size() != s1.Size() || !s2.ModTime().Equal(s1.ModTime()) {
		t.Fatalf("超时后 setsid 孙进程仍在写心跳（%d → %d 字节）：进程树没杀干净", s1.Size(), s2.Size())
	}

	// ⑤ 只读挂载：往 ro 目录里写必须失败（否则「只读」只是注释）。
	roHost := t.TempDir()
	if err := be.Mount(ctx, inst, roHost, "/ro", true); err != nil {
		t.Fatalf("Mount(ro): %v", err)
	}
	res, err = be.Exec(ctx, inst, []string{"/bin/sh", "-c", "touch /ro/x 2>/dev/null; echo $?"},
		ExecOpts{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("Exec(ro 探测): %v", err)
	}
	if got := strings.TrimSpace(string(res.Stdout)); got == "0" {
		t.Fatalf("ro 挂载里竟然写成功了（stdout=%q）", got)
	}

	// ⑥ 端口映射在本后端**不支持**（同栈直连），必须是 ErrBackendUnsupported。
	if _, err := be.Expose(ctx, inst, 8000, ""); !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("Expose 应 ErrBackendUnsupported，得到 %v", err)
	}
}
