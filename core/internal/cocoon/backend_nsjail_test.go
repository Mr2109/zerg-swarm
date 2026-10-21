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

// ── ①c′ 网络档只来自契约（T5：默认断网，host 才共享宿主 netns）──────────

// TestNsjailNetFromSpec — Exec 的那个纯函数落点：**同一份 argv、同一台后端**，
// 只有 `[茧].net` 不同 ⇒ 旗标不同。这条测试守的是「没有后端级开关」这个性质本身。
func TestNsjailNetFromSpec(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "true"}
	base := nsjailExecArgv(nil, nil, "/", "", argv)

	cases := []struct {
		name string
		spec *Spec
		want int // 期望 `--disable_clone_newnet` 出现次数
	}{
		{"nil 契约（无档位信息）", nil, 0},
		{"缺省（不写 net）", &Spec{}, 0},
		{`显式 none`, &Spec{Cocoon: CocoonSpec{Net: NetNone}}, 0},
		{`空串（笔误形态）`, &Spec{Cocoon: CocoonSpec{Net: ""}}, 0},
		{`未知值（手搓 Spec）`, &Spec{Cocoon: CocoonSpec{Net: "bridge"}}, 0},
		{`host`, &Spec{Cocoon: CocoonSpec{Net: NetHost}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nsjailApplyNet(base, tc.spec)
			if n := argCount(got, "--disable_clone_newnet"); n != tc.want {
				t.Fatalf("--disable_clone_newnet 出现 %d 次，want %d：%v", n, tc.want, got)
			}
			// 无论哪一档，茧内 argv 逐字不变、仍在 `--` 之后（网络档不许碰用户命令）。
			sep := -1
			for i, a := range got {
				if a == "--" {
					sep = i
					break
				}
			}
			if sep < 0 || !slices.Equal(got[sep+1:], argv) {
				t.Fatalf("`--` 之后的 argv 被改动了：%v", got)
			}
			// 纯函数：不改入参。
			if argCount(base, "--disable_clone_newnet") != 0 {
				t.Fatalf("nsjailApplyNet 就地改了入参：%v", base)
			}
		})
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
	// 换了真 rootfs ⇒ 改成在**茧内**解析（见 TestNsjailResolveArgv0InJail）。这里只钉一条：
	// 那个 rootfs 里没有 sh ⇒ 不报错、也**不许**被换成宿主的 /bin/sh（拿宿主凑茧内）。
	if got, err := nsjailResolveArgv0([]string{"sh"}, nsjailDefaultPATH, "/some/rootfs-that-has-no-sh"); err != nil {
		t.Fatalf("茧内找不到不该由本层报错，得到 %v", err)
	} else if got[0] != "sh" {
		t.Fatalf("茧内找不到时 argv[0] 必须原样（不许填宿主路径），得到 %q", got[0])
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
	// ★ 整根挂一律拒：把宿主根挂成茧内 "/" 等于把换根作废（隔离面只剩 namespace），
	//   而调用方会以为「挂了个盘」。
	if err := be.Mount(ctx, inst, "/srv/app", "/", true); !errors.Is(err, ErrBadMount) {
		t.Fatalf("把东西挂到茧内 \"/\" 应 ErrBadMount，得到 %v", err)
	}
	if err := be.Mount(ctx, inst, "/", "/", true); !errors.Is(err, ErrBadMount) {
		t.Fatalf("整根挂应 ErrBadMount，得到 %v", err)
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

// ── ①e 真 rootfs：方案（rootfs/overlay 的落点算术）──────────────────────
//
// 这一格判的是「方案算得对不对」，**不碰 mount**（那在 backend_nsjail_rootfs.go，只有 linux 有）。
// 所以开发机（macOS）上也能判，且判的是真代码路径。
func TestNsjailRootfsPlan(t *testing.T) {
	rootDir := filepath.Join(string(os.PathSeparator), "cocoons", "nsjail-1")

	// ① 既有档：没给 rootfs ⇒ chroot 用后端缺省、不 overlay、不算真镜像根。
	p, err := nsjailRootfsPlanFor(&Spec{}, "/", rootDir)
	if err != nil {
		t.Fatalf("没给 rootfs 不该报错：%v", err)
	}
	if p.Overlay || p.IsJail() {
		t.Fatalf("没给 rootfs 时不该 overlay / 不该当镜像根：%+v", p)
	}
	if p.Chroot != "/" {
		t.Fatalf("没给 rootfs 时 chroot 应当就是后端缺省 \"/\"，得到 %q", p.Chroot)
	}
	// nil spec 同理（不许 panic）。
	if _, err := nsjailRootfsPlanFor(nil, "", rootDir); err != nil {
		t.Fatalf("nil spec 不该报错：%v", err)
	}

	// ② 给了 rootfs ⇒ chroot 指向**合并视图**（不是 rootfs 本身，也不是宿主根），
	//    upper/work/merged/tmp 全落在茧目录下（同盘是 overlay 的硬要求）。
	sp := &Spec{Cocoon: CocoonSpec{Rootfs: "/images/debian"}}
	p, err = nsjailRootfsPlanFor(sp, "/", rootDir)
	if err != nil {
		t.Fatalf("给了 rootfs 应当算得出方案：%v", err)
	}
	if !p.Overlay || !p.IsJail() {
		t.Fatalf("给了真 rootfs 就该 overlay：%+v", p)
	}
	if p.Rootfs != "/images/debian" {
		t.Fatalf("lower 应当是契约给的 rootfs，得到 %q", p.Rootfs)
	}
	wantMerged := filepath.Join(rootDir, nsjailMergedDirName)
	if p.Chroot != wantMerged || p.Merged != wantMerged {
		t.Fatalf("chroot 应当是合并视图 %q，得到 chroot=%q merged=%q", wantMerged, p.Chroot, p.Merged)
	}
	if p.Chroot == p.Rootfs || p.Chroot == "/" {
		t.Fatalf("chroot 不许等于 rootfs 本身或宿主根：%q", p.Chroot)
	}
	if want := filepath.Join(rootDir, nsjailUpperDirName); p.Upper != want {
		t.Fatalf("缺省 upper 应当是 %q（<茧目录>/upper），得到 %q", want, p.Upper)
	}
	if want := filepath.Join(rootDir, nsjailWorkDirName); p.Work != want {
		t.Fatalf("work 应当是 %q，得到 %q", want, p.Work)
	}
	if want := filepath.Join(rootDir, nsjailTmpDirName); p.TmpDir != want {
		t.Fatalf("茧内 /tmp 的宿主来源应当是 %q，得到 %q", want, p.TmpDir)
	}

	// ③ overlay_upper 显式给了 ⇒ 用它（不再拼缺省）。
	explicit := &Spec{Cocoon: CocoonSpec{Rootfs: "/images/debian", OverlayUpper: "/data/upper/shop"}}
	p, err = nsjailRootfsPlanFor(explicit, "/", rootDir)
	if err != nil {
		t.Fatalf("显式 upper 应当通过：%v", err)
	}
	if p.Upper != "/data/upper/shop" {
		t.Fatalf("显式 overlay_upper 没被采用：%q", p.Upper)
	}

	// ④ rootfs="/" ⇒ 与不写等价（下层是宿主根，没得叠）。
	p, err = nsjailRootfsPlanFor(&Spec{Cocoon: CocoonSpec{Rootfs: "/"}}, "/", rootDir)
	if err != nil || p.Overlay || p.IsJail() || p.Chroot != "/" {
		t.Fatalf("rootfs=/ 应当等价于不换根：p=%+v err=%v", p, err)
	}

	// ⑤ 没有茧目录 ⇒ **报错**（这条是本单的判据：不许算不出就退回宿主根）。
	if _, err := nsjailRootfsPlanFor(sp, "/", ""); err == nil {
		t.Fatal("给了 rootfs 但没有茧目录（BaseDir 空）必须报错 —— 退回宿主根正是要消灭的行为")
	} else if !strings.Contains(err.Error(), "BaseDir") {
		t.Fatalf("报错该点名缺什么（BaseDir）：%v", err)
	}

	// ⑥ 相对 rootfs：契约层已经绝对化了，走到这里还相对 ⇒ 有人在绕契约，拒。
	if _, err := nsjailRootfsPlanFor(&Spec{Cocoon: CocoonSpec{Rootfs: "images/debian"}}, "/", rootDir); err == nil {
		t.Fatal("相对 rootfs 应当被拒（相对谁只能有一个答案，那在契约解析里定）")
	}

	// ⑦ 自噬型配置：upper 落在合并视图上 / upper == rootfs ⇒ 拒（跑起来只会诡异）。
	clash := &Spec{Cocoon: CocoonSpec{Rootfs: "/images/debian", OverlayUpper: wantMerged}}
	if _, err := nsjailRootfsPlanFor(clash, "/", rootDir); err == nil {
		t.Fatal("overlay_upper 与合并视图同路径应当被拒")
	}
	same := &Spec{Cocoon: CocoonSpec{Rootfs: "/images/debian", OverlayUpper: "/images/debian"}}
	if _, err := nsjailRootfsPlanFor(same, "/", rootDir); err == nil {
		t.Fatal("overlay_upper 等于 rootfs 应当被拒（上下层是同一个目录）")
	}

	// ⑧ 目录不在 / 不是目录 ⇒ Provision 会拒（报话必须指向「先解包」，不许假装我们拉了镜像）。
	if _, err := nsjailCheckRootfs(&Spec{Cocoon: CocoonSpec{Rootfs: filepath.Join(t.TempDir(), "nope")}}); err == nil {
		t.Fatal("不存在的 rootfs 应当被拒")
	}
	plainFile := filepath.Join(t.TempDir(), "a-file")
	must(t, os.WriteFile(plainFile, nil, 0o600))
	if _, err := nsjailCheckRootfs(&Spec{Cocoon: CocoonSpec{Rootfs: plainFile}}); err == nil {
		t.Fatal("rootfs 指向普通文件应当被拒")
	}
	if _, err := nsjailCheckRootfs(&Spec{}); err != nil {
		t.Fatalf("没给 rootfs 时不该报错（既有档）：%v", err)
	}
}

// ── ①f 真 rootfs：挂载面与 argv（判据：不整根挂、只读面齐全、/tmp 可写）──────
func TestNsjailRootfsArgv(t *testing.T) {
	base := t.TempDir()
	img := filepath.Join(base, "rootfs")
	instDir := filepath.Join(base, "inst")
	// 造一个「像 debian」的最小镜像根：usr 是真目录、bin 是指向 usr/bin 的符号链接。
	must(t, os.MkdirAll(filepath.Join(img, "usr", "bin"), 0o755))
	must(t, os.Symlink("usr/bin", filepath.Join(img, "bin")))

	sp := &Spec{
		Cocoon:    CocoonSpec{Rootfs: img},
		Resources: ResourceSpec{CPU: 1, MemGB: 1, DiskGB: 1},
	}
	p, err := nsjailRootfsPlanFor(sp, "/", instDir)
	if err != nil {
		t.Fatalf("方案算不出来：%v", err)
	}
	// 合并视图在本机（无内核 overlay）用同名目录代替：本格判 argv 拼装，不判 mount。
	must(t, os.MkdirAll(filepath.Join(p.Merged, "usr", "bin"), 0o755))
	must(t, os.Symlink("usr/bin", filepath.Join(p.Merged, "bin")))

	full := nsjailExecArgvForPlan(sp, nil, p, "/app", []string{"/bin/sh", "-c", "true"})
	sep := slices.Index(full, "--")
	if sep < 0 {
		t.Fatalf("argv 里没有 `--`：%v", full)
	}
	flags := full[:sep]

	// chroot 必须是合并视图（宿主路径），不是 rootfs、不是 "/"。
	if got, ok := argValue(flags, "--chroot"); !ok || got != p.Merged {
		t.Fatalf("--chroot = %q ok=%v，want 合并视图 %q", got, ok, p.Merged)
	}

	// ★ nsjail 缺省把 chroot 挂成只读 ⇒ 不给 --rw 的话 overlay 的 upper 永远收不到东西
	//   （「茧内可写」就是假的）。必须恰好一次、且落在 `--` 之前。
	if argCount(flags, "--rw") != 1 {
		t.Fatalf("真 rootfs 下应当恰好一次 --rw（chroot 缺省只读）：%v", flags)
	}
	if slices.Contains(full[sep+1:], "--rw") {
		t.Fatalf("--rw 落到 `--` 之后（成了茧内命令的参数）：%v", full)
	}

	// 只读面：/proc 与 /dev 的三件必须显式挂、且是 ro。
	for _, want := range []string{"/proc:/proc", "/dev/null:/dev/null", "/dev/urandom:/dev/urandom"} {
		if !slices.Contains(flags, want) {
			t.Fatalf("缺只读面 %q：%v", want, flags)
		}
	}
	// /tmp 必须是**可写**的 bind，源是茧目录里的 tmp（不用 tmpfs：壳外要能看见写了什么）。
	if !slices.Contains(flags, p.TmpDir+":/tmp") {
		t.Fatalf("缺可写 /tmp（源 %q）：%v", p.TmpDir, flags)
	}
	if slices.Contains(flags, "/tmp:/tmp") {
		t.Fatalf("不许把宿主 /tmp 直接挂进茧（等于把宿主临时目录送给茧写）：%v", flags)
	}
	// 系统面：usr 是真实目录 ⇒ 只读挂；bin 是符号链接 ⇒ 跳过（对链接做 bind 会把链接解析掉）。
	if !slices.Contains(flags, filepath.Join(p.Merged, "usr")+":/usr") {
		t.Fatalf("系统面 /usr 应当只读挂进来：%v", flags)
	}
	if slices.Contains(flags, filepath.Join(p.Merged, "bin")+":/bin") {
		t.Fatalf("符号链接形态的 /bin 不该被 bind（会被解析成目标，名实不符）：%v", flags)
	}

	// ★ 不许整根挂：任何一条挂载的 inner 都不许是 "/"，宿主侧也不许有单独一个 "/"。
	for i := 0; i+1 < len(flags); i++ {
		if flags[i] != "--bindmount" && flags[i] != "--bindmount_ro" {
			continue
		}
		host, inner, ok := strings.Cut(flags[i+1], ":")
		if !ok {
			t.Fatalf("挂载项格式不认识：%q", flags[i+1])
		}
		if inner == "/" {
			t.Fatalf("不许整根挂：%q", flags[i+1])
		}
		if host == "/" {
			t.Fatalf("不许把宿主根挂进来：%q", flags[i+1])
		}
	}

	// 用户显式挂的优先（Mount 是有意为之）：用户给了 /tmp 就按用户的来，不被缺省顶掉。
	userMounts := map[string]nsjailMount{"/tmp": {Host: "/srv/mytmp", Inner: "/tmp", RO: true}}
	custom := nsjailExecArgvForPlan(sp, userMounts, p, "", []string{"/bin/sh"})
	if !slices.Contains(custom, "/srv/mytmp:/tmp") || slices.Contains(custom, p.TmpDir+":/tmp") {
		t.Fatalf("用户显式挂的 /tmp 没被采用（或与缺省同时出现）：%v", custom)
	}

	// 既有档（没给 rootfs）：一条兜底面都不许冒出来 —— 行为必须与改动前逐字相同。
	plain := nsjailExecArgvForPlan(&Spec{}, nil, nsjailRootfsPlan{Chroot: "/"}, "", []string{"true"})
	if argCount(plain, "--rw") != 0 || argCount(plain, "--bindmount_ro") != 0 {
		t.Fatalf("没给 rootfs 时不该出现 --rw / 兜底面：%v", plain)
	}
}

// must — 测试夹具用（写盘失败直接停）。
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ── ①g 真 rootfs：argv[0] 按**茧内** PATH 解析（不许拿宿主路径凑）────────────
func TestNsjailResolveArgv0InJail(t *testing.T) {
	jail := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(jail, "usr", "bin"), 0o755))
	must(t, os.WriteFile(filepath.Join(jail, "usr", "bin", "tool"), []byte("#!/bin/sh\n"), 0o755))
	// 茧内也有 /bin/sh（用普通文件占位；本格判解析，不判能不能真跑）。
	must(t, os.MkdirAll(filepath.Join(jail, "bin"), 0o755))
	must(t, os.WriteFile(filepath.Join(jail, "bin", "sh"), []byte("#!/bin/sh\n"), 0o755))

	got, err := nsjailResolveArgv0([]string{"tool", "arg"}, "/usr/bin:/bin", jail)
	if err != nil {
		t.Fatalf("茧内的 tool 应当解析成功：%v", err)
	}
	// ★ 写回 argv[0] 的必须是**茧内路径**：把宿主路径塞进去，等于让 nsjail 在茧里 exec 宿主
	//   二进制（本单要消灭的正是这件事）。
	if got[0] != "/usr/bin/tool" {
		t.Fatalf("argv[0] 应当是茧内路径 /usr/bin/tool，得到 %q", got[0])
	}
	if strings.HasPrefix(got[0], jail) {
		t.Fatalf("argv[0] 被写成了宿主路径 %q（拿宿主凑茧内）", got[0])
	}
	if !slices.Equal(got[1:], []string{"arg"}) {
		t.Fatalf("argv[1:] 被改动：%v", got[1:])
	}

	// 茧内 PATH 里找不到（宿主上却有）：**不 error、也不许换成宿主路径**，原样交给 nsjail 报。
	miss, err := nsjailResolveArgv0([]string{"definitely-not-here-xyz"}, nsjailDefaultPATH, jail)
	if err != nil {
		t.Fatalf("茧内找不到不该由本层报错（交给 nsjail 的 execve 日志）：%v", err)
	}
	if miss[0] != "definitely-not-here-xyz" {
		t.Fatalf("茧内找不到时 argv[0] 必须原样，得到 %q", miss[0])
	}

	// 写了路径 ⇒ 原样（那是「茧内这条路径」，解析交给 nsjail/chroot）。
	abs := []string{"/bin/sh", "-c", "true"}
	if got, err := nsjailResolveArgv0(abs, nsjailDefaultPATH, jail); err != nil || !slices.Equal(got, abs) {
		t.Fatalf("绝对路径应当原样：got=%v err=%v", got, err)
	}
}

// ── ①h 「茧目录」接线口：SetCocoonDir 的五态 + 实例根落点 ──────────────────────
//
// 命令面（`cocoon open`）把 `<真源根>/<id>/` 交过来，后端据此把实例根与 overlay 的
// upper/work/merged 一起放进茧目录。这一格打**校验面**（两平台都能跑）：
//
//	① 空串 ⇒ 拒（空串 = 「没给」，✗ 不许静默当默认值）
//	② 相对路径 ⇒ 拒（实例根会跟着 cwd 漂）
//	③ 不存在 ⇒ 拒（点明「先建目录再把目录交给后端」= 时序错在调用方）
//	④ 普通文件 ⇒ 拒（不是目录）
//	⑤ 真目录 ⇒ BaseDir 设成它（尾斜杠被 Clean 掉）
//
// 负控：拒执之后 BaseDir 必须**还是空的**（不许先设上再校验 ⇒ 半生效）。
func TestNsjailSetCocoonDir(t *testing.T) {
	be := NewNsjailBackend()

	// ①②③④：四种非法输入全拒，且拒完 BaseDir 仍是空（不许半生效）。
	bad := []struct {
		name string
		dir  string
		want string
	}{
		{"空串", "", "为空"},
		{"相对路径", filepath.Join("cocoons", "shop"), "不是绝对路径"},
		{"不存在", filepath.Join(t.TempDir(), "nope", "shop"), "先建目录"},
	}
	for _, tc := range bad {
		err := be.SetCocoonDir(tc.dir)
		if err == nil {
			t.Fatalf("%s（%q）应当被拒", tc.name, tc.dir)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s 的报错该点明 %q：%v", tc.name, tc.want, err)
		}
		if be.BaseDir != "" {
			t.Fatalf("%s 被拒后 BaseDir 竟已被设成 %q（拒绝必须零生效）", tc.name, be.BaseDir)
		}
	}
	plain := filepath.Join(t.TempDir(), "a-file")
	must(t, os.WriteFile(plain, nil, 0o600))
	if err := be.SetCocoonDir(plain); err == nil || !strings.Contains(err.Error(), "不是目录") {
		t.Fatalf("普通文件应当被拒且点明「不是目录」：%v", err)
	}
	if be.BaseDir != "" {
		t.Fatalf("文件被拒后 BaseDir 竟已被设成 %q", be.BaseDir)
	}

	// ⑤ 真目录 ⇒ 设上；尾斜杠被 Clean 掉（Provision 拿它拼实例根，形态要稳）。
	dir := filepath.Join(t.TempDir(), "shop-abc12345")
	must(t, os.MkdirAll(dir, 0o700))
	if err := be.SetCocoonDir(dir + string(filepath.Separator)); err != nil {
		t.Fatalf("已存在的目录应当接受：%v", err)
	}
	if be.BaseDir != dir {
		t.Fatalf("BaseDir=%q，want %q（要 Clean 掉尾斜杠）", be.BaseDir, dir)
	}
	// 幂等：同一个值设两次仍是同一个值。
	if err := be.SetCocoonDir(dir); err != nil || be.BaseDir != dir {
		t.Fatalf("重复设置应当幂等：BaseDir=%q err=%v", be.BaseDir, err)
	}

	// 实例根必须在茧目录**下一层**（不在茧目录本身、也不在它的兄弟位置）：
	// 这是「close --purge 删茧目录能一次收干净」与「碰不到兄弟茧」的共同前提。
	if HostPlatform() != PlatformLinux {
		return // 下面这腿要真建目录，只有 linux 的 Provision 会做
	}
	inst, err := be.Provision(context.Background(), &Spec{Cocoon: CocoonSpec{Name: "shop"}})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	wantRoot := filepath.Join(dir, inst.ID)
	if inst.Root != wantRoot {
		t.Fatalf("实例根=%q，want %q（<茧目录>/<后端签发的 id>）", inst.Root, wantRoot)
	}
	if filepath.Dir(inst.Root) != dir {
		t.Fatalf("实例根的父目录应当是茧目录 %q，得到 %q", dir, filepath.Dir(inst.Root))
	}
	if fi, err := os.Stat(wantRoot); err != nil || !fi.IsDir() {
		t.Fatalf("实例根应当真被建出来：stat=%v err=%v", fi, err)
	}
	if err := be.Destroy(context.Background(), inst); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}
