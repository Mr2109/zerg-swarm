package backend

// bwrap_bindings_test.go —— 卵沙箱第 4 步：GPU 设备与 sysfs 的 bwrap 绑定清单**契约**
// （后端产 Spec ⇒ 孵化器 BuildBwrapArgv 产 argv，跨件断言，纯数据/字符串面，任何平台可跑）。
//
// 钉住的六条一手结论（任何一条被改坏，本文件先红）：
//
//	① 设备节点只能用 `--dev-bind`：bwrap 的 `--ro-bind` / `--bind` 在源码里给挂载点加了
//	   `MOUNT_ATTR_NODEV` ⇒ 设备节点即便出现在空间里也**打不开**，且引擎报的是
//	   `no usable GPU` 而不是权限错（归因极难）。故 /dev/* 一律不得走 ro-bind/bind。
//	② 基础面：`--proc /proc`（libhsakmt 读 /proc/cpuinfo）、`--dev /dev`、`--tmpfs /tmp`，
//	   以及 KFD 的 sysfs 拓扑（`/sys/devices/virtual/kfd/kfd`，libhsakmt 的 KFD_SYSFS_PATH）
//	   —— 缺它 topology 初始化直接失败。本仓实现是**整个 /sys 只读**（覆盖该前缀，见下）。
//	③ 顺序：bwrap 的文件系统类选项**按序应用** ⇒ `--dev /dev` 必须排在设备 `--dev-bind` **之前**
//	   （反了就会被后挂的 tmpfs 盖掉，设备在空间里消失）。
//	④ 不得有 `--clearenv`（只留 PWD）/ `--as-pid-1`（SIGNAL_UNKILLABLE ⇒ 信号黑洞）。
//	⑤ 网络不得 `--unshare-net`（它会把回环也拿走，引擎与探针都连不上）。
//	⑥ 环境变量**不经** `--setenv` 下发（本机 bwrap 0.11.1 上必失败，见 hatch.go 坑④），
//	   改由包装 shell 的 `export …; exec "$@"` —— 故 argv 里出现 --setenv 就是配方走偏。
//
// 每条断言都配一枚**合成负控**（把错误形态直接喂给同一个判据函数，必须红）：
// 「判据能判绿」不算有牙，「喂错误形态必红」才算。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ── bwrap argv 的轻量解析（够本文件用：只认「选项 + 固定元数」）──────────────

type bwrapOp struct {
	name string
	args []string
	idx  int // 在 argv 里的下标（选项名所在位置）
}

// bwrapOpArity 各选项吃几个值（我们这一层只关心挂载/隔离类；未知选项按 0 元当开关）。
var bwrapOpArity = map[string]int{
	"--ro-bind": 2, "--bind": 2, "--dev-bind": 2,
	"--ro-bind-try": 2, "--bind-try": 2, "--dev-bind-try": 2,
	"--symlink": 2, "--setenv": 2,
	"--dev": 1, "--proc": 1, "--tmpfs": 1, "--dir": 1, "--chdir": 1, "--mqueue": 1,
}

// parseBwrapOps 把 argv 切成「选项 + 值」序列（只切 `--` 之前的那段：之后是入口命令）。
func parseBwrapOps(argv []string) []bwrapOp {
	var ops []bwrapOp
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--" {
			break
		}
		if !strings.HasPrefix(argv[i], "--") {
			continue // 值被上一个选项吃掉时不会走到这里；真走到也只当噪声跳过
		}
		n := bwrapOpArity[argv[i]]
		op := bwrapOp{name: argv[i], idx: i}
		if i+n < len(argv) {
			op.args = argv[i+1 : i+1+n]
			i += n
		}
		ops = append(ops, op)
	}
	return ops
}

// indexOfOp 首个「名字与全部值都相等」的选项下标；缺 ⇒ -1。
func indexOfOp(argv []string, name string, args ...string) int {
	for _, op := range parseBwrapOps(argv) {
		if op.name != name || len(op.args) != len(args) {
			continue
		}
		same := true
		for k := range args {
			if op.args[k] != args[k] {
				same = false
				break
			}
		}
		if same {
			return op.idx
		}
	}
	return -1
}

// countOp 统计「名字与全部值都相等」的选项个数。
func countOp(argv []string, name string, args ...string) int {
	n := 0
	for _, op := range parseBwrapOps(argv) {
		if op.name != name || len(op.args) != len(args) {
			continue
		}
		same := true
		for k := range args {
			if op.args[k] != args[k] {
				same = false
				break
			}
		}
		if same {
			n++
		}
	}
	return n
}

// ── 判据函数（正面用真 argv，反面喂合成错误形态）────────────────────────────

// deviceNodeVerdict 判一个设备节点是否**只**经 `--dev-bind` 进空间。
//
// 结论① 的机器判据：`--ro-bind`/`--bind`（含 -try 变体）挂 /dev/* 会带 nodev ⇒ 设备打不开。
func deviceNodeVerdict(argv []string, node string) error {
	if n := countOp(argv, "--dev-bind", node, node); n != 1 {
		return errf("设备 %s 应恰好有一条 `--dev-bind %s %s`，实得 %d 条", node, node, node, n)
	}
	for _, op := range parseBwrapOps(argv) {
		switch op.name {
		case "--ro-bind", "--bind", "--ro-bind-try", "--bind-try":
			if len(op.args) == 2 && (op.args[1] == node || op.args[0] == node) {
				return errf("设备 %s 被 `%s` 挂进空间 —— 该类挂载带 nodev（设备打不开、引擎报 no usable GPU），设备节点只许 `--dev-bind`",
					node, op.name)
			}
		}
	}
	return nil
}

// noRoBindUnderDevVerdict 判**任何** /dev 下的落点都没有走 ro-bind/bind（设备只许 --dev-bind）。
func noRoBindUnderDevVerdict(argv []string) error {
	for _, op := range parseBwrapOps(argv) {
		switch op.name {
		case "--ro-bind", "--bind", "--ro-bind-try", "--bind-try":
			if len(op.args) == 2 && strings.HasPrefix(op.args[1], "/dev/") {
				return errf("`%s %s %s` 把设备节点按普通文件挂载（带 nodev ⇒ 打不开）；设备一律 `--dev-bind`",
					op.name, op.args[0], op.args[1])
			}
		}
	}
	return nil
}

// sysfsCoverVerdict 判 want（如 /sys/devices/virtual/kfd/kfd）是否被**只读**绑定覆盖。
//
// 口径：整个 /sys 只读挂载是覆盖该前缀的**正确**实现（更窄的逐节点绑定也接受），
// 但覆盖它的那条必须是 `--ro-bind` —— 可写绑定等于把宿主 /sys 写面放进空间。
func sysfsCoverVerdict(argv []string, want string) error {
	covered := ""
	for _, op := range parseBwrapOps(argv) {
		if len(op.args) != 2 {
			continue
		}
		dest := op.args[1]
		isAncestor := dest == "/" || want == dest || strings.HasPrefix(want, strings.TrimSuffix(dest, "/")+"/")
		if !isAncestor {
			continue
		}
		switch op.name {
		case "--ro-bind", "--ro-bind-try":
			if covered == "" {
				covered = dest
			}
		case "--bind", "--bind-try":
			return errf("%s 被可写绑定 `%s %s %s` 覆盖 —— sysfs 必须只读（宿主 /sys 写面不许进空间）",
				want, op.name, op.args[0], dest)
		}
	}
	if covered == "" {
		return errf("空间里没有任何只读绑定覆盖 %s（KFD 拓扑 sysfs：缺它 libhsakmt topology 初始化直接失败）", want)
	}
	return nil
}

// orderVerdict 判 first 在 second **之前**（bwrap 文件系统类选项按序应用：反了会被后挂的盖掉）。
func orderVerdict(argv []string, first, second bwrapOp) error {
	a := indexOfOp(argv, first.name, first.args...)
	b := indexOfOp(argv, second.name, second.args...)
	if a < 0 {
		return errf("缺 `%s %s`", first.name, strings.Join(first.args, " "))
	}
	if b < 0 {
		return errf("缺 `%s %s`", second.name, strings.Join(second.args, " "))
	}
	if a > b {
		return errf("`%s %s`（idx=%d）必须排在 `%s %s`（idx=%d）之前 —— bwrap 按序应用，反了设备会被后挂的 tmpfs 盖掉",
			first.name, strings.Join(first.args, " "), a, second.name, strings.Join(second.args, " "), b)
	}
	return nil
}

// errf 小工具：判据函数统一返回 error（有值即红）。
func errf(format string, args ...any) error { return fmt.Errorf(format, args...) }

// ── 夹具：真走「后端映射 → hatch 配方」，不另造一份 argv ─────────────────────

// bwrapSpecForTest 用真实映射路径产一枚主线卵的 Spec（设备取声明卡号，库目录用临时目录）。
func bwrapSpecForTest(t *testing.T) hatch.Spec {
	t.Helper()
	const engineHost = "/home/<user>/llama.cpp-src/build-hip-flash/bin/llama-server"
	withMainlineEngineProbe(t, engineHost)
	withWorkDirRoot(t)

	libDir := filepath.Join(t.TempDir(), "build-hip-flash")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/glm-5.3/GLM-5.3-Flash-Q4_K_M.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		EnvReq: &registry.EnvReq{
			Devices:  []string{"1"}, // 第 1 张卡 ⇒ renderD129
			LibPaths: []string{libDir},
		},
	}
	entry.SetEggNameForTest("GLM-5.3-Flash")
	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9411, hatchTestProfile())
	if err != nil {
		t.Fatalf("主线卵映射应成功，实得 %v", err)
	}
	return spec
}

// bwrapArgvForTest 由真 Spec 产真 argv。
func bwrapArgvForTest(t *testing.T, spec hatch.Spec) []string {
	t.Helper()
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("映射产物应能构造出 bwrap 命令行，实得 %v", err)
	}
	return argv
}

// ── ① 设备节点：只许 --dev-bind（不许 ro-bind/bind）─────────────────────────

func TestBwrapBindings_DeviceNodesUseDevBindOnly(t *testing.T) {
	spec := bwrapSpecForTest(t)
	argv := bwrapArgvForTest(t, spec)
	if len(spec.Devices) == 0 {
		t.Fatal("夹具应声明到设备节点（否则本用例什么都没验）")
	}
	for _, node := range spec.Devices {
		if err := deviceNodeVerdict(argv, node); err != nil {
			t.Errorf("%v（argv=%v）", err, argv)
		}
	}
	if err := noRoBindUnderDevVerdict(argv); err != nil {
		t.Errorf("%v（argv=%v）", err, argv)
	}
	// KFD 计算节点必须在场（结论②：--dev /dev 看不到 GPU，必须显式绑）
	if err := deviceNodeVerdict(argv, "/dev/kfd"); err != nil {
		t.Errorf("/dev/kfd 是 ROCm 的必需节点：%v", err)
	}

	// 负控：把设备按普通挂载挂进去（历史错法）⇒ 同一个判据必须红
	bad := []string{"--dev", "/dev", "--ro-bind", "/dev/kfd", "/dev/kfd", "--"}
	if err := deviceNodeVerdict(bad, "/dev/kfd"); err == nil {
		t.Error("负控失效：`--ro-bind /dev/kfd` 这种形态本该被判红（nodev ⇒ 设备打不开）")
	}
	bad2 := []string{"--bind", "/dev/dri/renderD129", "/dev/dri/renderD129", "--"}
	if err := noRoBindUnderDevVerdict(bad2); err == nil {
		t.Error("负控失效：`--bind /dev/dri/...` 本该被判红")
	}
	// 负控反向：正确形态必须绿（否则判据是永红）
	good := []string{"--dev", "/dev", "--dev-bind", "/dev/kfd", "/dev/kfd", "--"}
	if err := deviceNodeVerdict(good, "/dev/kfd"); err != nil {
		t.Errorf("正确形态（--dev-bind）本该判绿，实得 %v", err)
	}
	if err := noRoBindUnderDevVerdict(good); err != nil {
		t.Errorf("正确形态（--dev-bind）本该判绿，实得 %v", err)
	}
}

// ── ② KFD sysfs 拓扑必须在只读覆盖范围内 + 基础伪文件系统齐 ──────────────────

func TestBwrapBindings_KfdSysfsCoveredReadOnly(t *testing.T) {
	spec := bwrapSpecForTest(t)
	argv := bwrapArgvForTest(t, spec)

	// libhsakmt 的 KFD_SYSFS_PATH；缺它 topology 初始化直接失败（结论②）
	const kfdSysfs = "/sys/devices/virtual/kfd/kfd"
	if err := sysfsCoverVerdict(argv, kfdSysfs); err != nil {
		t.Errorf("%v（argv=%v）", err, argv)
	}
	// 同一判据对「没有 /sys 绑定」的形态必须红
	if err := sysfsCoverVerdict([]string{"--proc", "/proc", "--"}, kfdSysfs); err == nil {
		t.Error("负控失效：没有任何 /sys 绑定本该被判红")
	}
	// 可写覆盖必须红（sysfs 只读）
	if err := sysfsCoverVerdict([]string{"--bind", "/sys", "/sys", "--"}, kfdSysfs); err == nil {
		t.Error("负控失效：`--bind /sys /sys`（可写）本该被判红")
	}
	// 更窄的逐节点只读绑定同样接受（判据不锁死实现形态）
	if err := sysfsCoverVerdict([]string{"--ro-bind", kfdSysfs, kfdSysfs, "--"}, kfdSysfs); err != nil {
		t.Errorf("逐节点只读绑定本该判绿，实得 %v", err)
	}

	// 基础面：--proc /proc（libhsakmt 读 /proc/cpuinfo）、--dev /dev、--tmpfs /tmp
	for _, want := range [][]string{{"--proc", "/proc"}, {"--dev", "/dev"}, {"--tmpfs", "/tmp"}} {
		if indexOfOp(argv, want[0], want[1]) < 0 {
			t.Errorf("缺 `%s %s`（结论②：libhsakmt 读 /proc/cpuinfo、设备经 --dev 打底、/tmp 可写）", want[0], want[1])
		}
	}
	// --dev /dev 只许一条：多一条排在设备绑定之后就等于把设备盖掉
	if n := countOp(argv, "--dev", "/dev"); n != 1 {
		t.Errorf("`--dev /dev` 应恰好 1 条，实得 %d 条", n)
	}
}

// ── ③ 顺序：--dev /dev 必须先于设备 --dev-bind ───────────────────────────────

func TestBwrapBindings_DevTmpfsBeforeDeviceBinds(t *testing.T) {
	spec := bwrapSpecForTest(t)
	argv := bwrapArgvForTest(t, spec)

	dev := bwrapOp{name: "--dev", args: []string{"/dev"}}
	first := bwrapOp{name: "--dev-bind", args: []string{"/dev/kfd", "/dev/kfd"}}
	if err := orderVerdict(argv, dev, first); err != nil {
		t.Errorf("%v（argv=%v）", err, argv)
	}
	// 每个声明的设备节点都必须在 --dev /dev 之后（不论先后实现怎么变）
	for _, node := range spec.Devices {
		op := bwrapOp{name: "--dev-bind", args: []string{node, node}}
		if err := orderVerdict(argv, dev, op); err != nil {
			t.Errorf("%v", err)
		}
	}
	// 负控：顺序反了必须红
	bad := []string{"--dev-bind", "/dev/kfd", "/dev/kfd", "--dev", "/dev", "--"}
	if err := orderVerdict(bad, dev, first); err == nil {
		t.Error("负控失效：`--dev-bind` 排在 `--dev` 之前本该被判红（后挂的 tmpfs 会盖掉设备）")
	}
}

// ── ④⑤⑥ 隔离面：不碰 --clearenv / --as-pid-1 / --unshare-net / --setenv ─────

func TestBwrapBindings_IsolationSurfaceStaysGPUCompatible(t *testing.T) {
	spec := bwrapSpecForTest(t)
	argv := bwrapArgvForTest(t, spec)

	// ④⑤⑥ 的机器判据：这几个选项出现即配方走偏
	forbidden := map[string]string{
		"--unshare-net": "会拿走回环（引擎与探针连不上；--unshare-all 里也含它）",
		"--unshare-all": "含 --unshare-net，同上",
		"--clearenv":    "只留 PWD（环境全清，引擎拿不到 LD_LIBRARY_PATH 等）",
		"--as-pid-1":    "SIGNAL_UNKILLABLE ⇒ 信号黑洞（收不掉引擎）",
		"--setenv":      "本机 bwrap 上必失败（hatch.go 坑④），环境变量走包装 shell",
	}
	for name, why := range forbidden {
		if idx := indexOfOp(argv, name); idx >= 0 {
			t.Errorf("配方里不许出现 `%s`（%s），实得 idx=%d（argv=%v）", name, why, idx, argv)
		}
	}
	// --unshare-pid 是**允许**的（结论⑤：--unshare-* 不弄坏 GPU，只有 -net 弄坏）
	if indexOfOp(argv, "--unshare-pid") < 0 {
		t.Error("`--unshare-pid` 应在（进程视图隔离与 GPU 无关，结论⑤）")
	}
}
