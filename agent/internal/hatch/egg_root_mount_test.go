// egg_root_mount_test.go —— 卵沙箱第 5 步：**从蛋孵化**时挂载源换成蛋展开目录。
//
// 钉住两件事（任一条被改坏，本文件先红）：
//
//	① **零回归**：不是从蛋孵化时，bwrap argv **逐元素**与改动前逐字相同 —— 基线取自真机现读
//	   （edge-node `/proc/285.../cmdline`，2026-10-18，90 个 argv 元素），不是手抄的「应该长这样」；
//	② **换源生效**：EggRoot/EggLoaderDir 非空时 `/engine` 指向 `<蛋>/engine`、`/lib64` 与 `/lib`
//	   指向蛋内 loader 目录（PT_INTERP 的落点，方案 A：不改二进制），且 usrmerge 那两条
//	   `/lib` `/lib64` 符号链接**不再出现**（否则就是两套落点打架）；
//	③ **真自包含**（GAP-20261018-11/12）：蛋模式**不挂宿主 `/usr`**、不留 usrmerge 的 `usr/bin`
//	   `usr/sbin` 两条别名，`/bin` 改由蛋内 `sysroot/bin` 提供（里面那枚 `sh` 是包装 exec 的入口，
//	   没有它空根档逐字 `bwrap: execvp /bin/sh: No such file or directory`）。
//	   真机读数（edge-node）：带 `--ro-bind /usr /usr` 时 maps 里 libc 是**宿主**那件（dev 103:06
//	   inode 14288394），剔掉后变成 `dev 00:2e inode 17349` —— 逐值等于展开目录里那件。
//
// 配方其余部分（`--proc` / `--dev` / `--tmpfs /tmp` / `--ro-bind /sys /sys` / `--dir /models` /
// 设备 `--dev-bind` / `--unshare-pid`）在蛋模式下**一字不动** —— 它们与「引擎从哪来」无关。
package hatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// liveX3Spec 2026-10-18 真机现读的那张 Spec（edge-node 正在跑的 Qwen3.8-Flash-Next 沙箱）。
//
// 来源：`/proc/<bwrap>/cmdline`（NUL 分隔，90 个元素）⟷ 本包 BuildBwrapArgv 的输出**逐元素相同**
// （改动前实测：`MATCH(逐元素): True`）。把它冻在这里 = 零回归的机器判据。
func liveX3Spec() Spec {
	s := goodSpec()
	s.EggID = "Qwen3.8-Flash-Next"
	s.EnginePathInSpace = "/engine/llama-server"
	s.EngineRoots = []string{"/home/<user>/llama.cpp-src/build-hip-flash/bin"}
	s.EngineArgs = strings.Fields("-m /models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf -c 32768 -ngl 999 -ctk q8_0 -ctv q8_0 -fa true --cache-prompt --cache-idle-slots -np 1 -cb --reasoning-preserve --reasoning-budget 256 --host 127.0.0.1 --port 9400 --jinja --cache-reuse 256")
	s.WeightPath = "/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL"
	s.WeightFiles = []string{
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00002-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00003-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00004-of-00004.gguf",
	}
	s.ExtraROBinds = []string{
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf:/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00002-of-00004.gguf:/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00002-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00003-of-00004.gguf:/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00003-of-00004.gguf",
		"/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00004-of-00004.gguf:/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00004-of-00004.gguf",
		"/home/<user>/llama.cpp-src/build-hip-flash/bin:/libs/bin",
	}
	s.ExtraRWBinds = []string{"/home/<user>/.zerg/work/Qwen3.8-Flash-Next:/work"}
	s.Env = map[string]string{"LD_LIBRARY_PATH": "/libs/bin:/engine"}
	s.WorkDir = "/work"
	s.Profile = goodProfile()
	return s
}

// liveX3Argv 改动前真机 argv（90 个元素，一行一个；空行不是元素）。
//
// ★ 这是**冻结值**：改配方就要在这里显式改一行，并在变更账里说明为什么 —— 不许「顺手」漂。
const liveX3Argv = `--ro-bind
/usr
/usr
--symlink
usr/bin
/bin
--symlink
usr/sbin
/sbin
--symlink
usr/lib
/lib
--symlink
usr/lib64
/lib64
--proc
/proc
--dev
/dev
--tmpfs
/tmp
--ro-bind
/sys
/sys
--dir
/models
--ro-bind
/home/<user>/llama.cpp-src/build-hip-flash/bin
/engine
--ro-bind
/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf
/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf
--ro-bind
/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00002-of-00004.gguf
/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00002-of-00004.gguf
--ro-bind
/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00003-of-00004.gguf
/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00003-of-00004.gguf
--ro-bind
/data/models/qwen/Qwen3.8-Flash-Next/UD-Q4_K_XL/Qwen3.8-Flash-Next-UD-Q4_K_XL-00004-of-00004.gguf
/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00004-of-00004.gguf
--ro-bind
/home/<user>/llama.cpp-src/build-hip-flash/bin
/libs/bin
--bind
/home/<user>/.zerg/work/Qwen3.8-Flash-Next
/work
--dev-bind
/dev/kfd
/dev/kfd
--dev-bind
/dev/dri/renderD128
/dev/dri/renderD128
--unshare-pid
--die-with-parent
--chdir
/work
--
/bin/sh
-c
export LD_LIBRARY_PATH='/libs/bin:/engine'; exec "$@"
zerg-egg
/engine/llama-server
-m
/models/Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf
-c
32768
-ngl
999
-ctk
q8_0
-ctv
q8_0
-fa
true
--cache-prompt
--cache-idle-slots
-np
1
-cb
--reasoning-preserve
--reasoning-budget
256
--host
127.0.0.1
--port
9400
--jinja
--cache-reuse
256`

func TestBuildBwrapArgv_LiveX3BaselineUnchanged(t *testing.T) {
	argv, err := BuildBwrapArgv(liveX3Spec())
	if err != nil {
		t.Fatalf("BuildBwrapArgv: %v", err)
	}
	want := strings.Split(liveX3Argv, "\n")
	if len(argv) != len(want) {
		t.Fatalf("argv 元素数 %d ≠ 基线 %d（零回归破了）\n实得：%v", len(argv), len(want), argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] 与真机基线不同（零回归破了）：\n  实得 %q\n  基线 %q", i, argv[i], want[i])
		}
	}
}

// TestBuildBwrapArgv_EggRootMounts 从蛋孵化：三个/四个落点整体换源，usrmerge 的 /lib /lib64 退场。
func TestBuildBwrapArgv_EggRootMounts(t *testing.T) {
	s := goodSpec()
	const (
		eggRoot   = "/var/lib/zerg/eggs/example-moe-36b"
		eggLoader = "/var/lib/zerg/eggs/example-moe-36b/engine/lib"
	)
	s.EngineRoots = nil // 蛋孵化时引擎源只有一种：蛋
	s.EggRoot = eggRoot
	s.EggLoaderDir = eggLoader
	s.EggBinDir = eggRoot + "/sysroot/bin"
	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("BuildBwrapArgv: %v", err)
	}
	joined := strings.Join(argv, "\n")
	has := func(s string) bool { return strings.Contains(joined, s) }
	for _, want := range []string{
		"--ro-bind\n" + eggRoot + "/engine\n/engine",
		"--ro-bind\n" + eggLoader + "\n/lib64",
		"--ro-bind\n" + eggLoader + "\n/lib",
		"--ro-bind\n" + eggRoot + "/sysroot/bin\n/bin",
	} {
		if !has(want) {
			t.Errorf("蛋孵化缺绑定 %q（实得：%s）", want, joined)
		}
	}
	// usrmerge 的两条库链接**必须退场**（留着就是两套 /lib 落点，谁盖谁说不清）
	for _, gone := range []string{"--symlink\nusr/lib\n/lib\n", "--symlink\nusr/lib64\n/lib64\n"} {
		if has(gone) {
			t.Errorf("蛋孵化时仍出现 usrmerge 库链接 %q —— /lib 与 /lib64 必须由蛋提供", gone)
		}
	}
	// 其余配方一字不动
	for _, keep := range []string{
		"--proc\n/proc", "--dev\n/dev", "--tmpfs\n/tmp", "--ro-bind\n/sys\n/sys",
		"--dir\n" + spaceModelsDir,
		"--dev-bind\n/dev/kfd\n/dev/kfd", "--unshare-pid", "--die-with-parent",
	} {
		if !has(keep) {
			t.Errorf("蛋孵化把配方其余部分改坏了：缺 %q", keep)
		}
	}
	// 宿主系统路径一条都不挂（GAP-20261018-11 的机器判据）：/usr 与两条 usrmerge 别名全退场。
	for _, gone := range []string{
		"--ro-bind\n/usr\n/usr",
		"--symlink\nusr/bin\n/bin",
		"--symlink\nusr/sbin\n/sbin",
	} {
		if has(gone) {
			t.Errorf("蛋模式仍出现宿主系统路径 %q —— 空间里的 libc 会解析到宿主那件（假自包含）", gone)
		}
	}
	// 宿主构建目录不得再出现在 /engine 上（换源的关键判据）
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--ro-bind" && argv[i+2] == "/engine" && argv[i+1] != eggRoot+"/engine" {
			t.Errorf("蛋孵化时 /engine 仍从 %q 来（应只有 %q）", argv[i+1], eggRoot+"/engine")
		}
	}
}

// TestSpec_ValidateEggPairing 蛋的字段：`EggRoot`/`EggLoaderDir` 同进同出、与宿主挂载源互斥，
// 且 `EggBinDir`（蛋内 `/bin` 的来源）**蛋模式必填、非蛋模式不许给**（GAP-20261018-12）。
func TestSpec_ValidateEggPairing(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Spec)
		want string
	}{
		{"只给 EggRoot", func(s *Spec) { s.EggRoot = "/e"; s.EngineRoots = nil }, "不完整"},
		{"只给 EggLoaderDir", func(s *Spec) { s.EggLoaderDir = "/e/engine/lib" }, "不完整"},
		{"蛋 ＋ 宿主挂载源并存", func(s *Spec) {
			s.EggRoot = "/e"
			s.EggLoaderDir = "/e/engine/lib"
			s.EggBinDir = "/e/sysroot/bin"
		}, "两套引擎挂载源"},
		{"蛋根不是绝对路径", func(s *Spec) {
			s.EggRoot = "e"
			s.EggLoaderDir = "/e/engine/lib"
			s.EggBinDir = "/e/sysroot/bin"
			s.EngineRoots = nil
		}, "绝对路径"},
		// ★ 蛋模式缺 egg_bin_dir ⇒ 空间里没有 /bin/sh（包装 exec 的入口）⇒ 声明面就拒。
		{"蛋模式缺 egg_bin_dir", func(s *Spec) {
			s.EggRoot = "/e"
			s.EggLoaderDir = "/e/engine/lib"
			s.EngineRoots = nil
		}, "缺 egg_bin_dir"},
		{"egg_bin_dir 不是绝对路径", func(s *Spec) {
			s.EggRoot = "/e"
			s.EggLoaderDir = "/e/engine/lib"
			s.EggBinDir = "e/sysroot/bin"
			s.EngineRoots = nil
		}, "绝对路径"},
		{"非蛋模式却给了 egg_bin_dir", func(s *Spec) { s.EggBinDir = "/e/sysroot/bin" }, "只在蛋孵化时有意义"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := goodSpec()
			c.mut(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("应拒孵（%s），却过了 Validate", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("拒孵理由应含 %q，实得：%v", c.want, err)
			}
		})
	}
}

// ═══ 蛋模式的真自包含判据（GAP-20261018-11/12）═══════════════════════════════════════
//
// liveEggArgv 改后**蛋模式**的真机 argv（44 个元素，一行一个；空行不是元素）。
//
// ★ 来源（edge-node，2026-10-18 真机实测）：`cocoon pack` 打一枚带 `sysroot/bin/sh` 的夹具蛋 →
// `eggpkg.ExpandEgg` 真展开到 `/tmp/egg7verify/expanded` → 本包 `BuildBwrapArgv(liveEggSpec(root))`
// **逐元素**打印。同一份 argv 在 edge-node 上真 exec **rc=0**，且空间内 `maps` 里 libc 的
// `(dev,inode)=(00:2e,17398)` 逐值等于蛋内 `engine/lib/libc.so.6`（见 `TestBuildBwrapArgv_EggModeRealBwrap`）。
//
// ★ 它与 `liveX3Argv`（90 元素，**非蛋**模式）的差别只有三处，其余配方逐字同：
//
//	① 没有 `--ro-bind /usr /usr`（-3）：蛋模式一条宿主系统路径都不挂（GAP-20261018-11）；
//	② 没有 `--symlink usr/bin /bin` 与 `--symlink usr/sbin /sbin`（-6）：两条都指向已不存在的
//	   `/usr`，留着等于给「谁提供 /bin」两种说法；
//	③ 多了 `--ro-bind <蛋>/sysroot/bin /bin`（+3）：蛋里那枚 sh 就是空间里的 `/bin/sh`
//	   （GAP-20261018-12；包装 exec 的入口）。
//
// ⇒ 同一枚卵净 -6（第 5 步那枚 49 元素的夹具蛋：49 → 43）。本夹具那枚卵更小（设备只挂
// `/dev/null` 一个 ⇒ 少 3 个元素；它声明了 env ⇒ 入口多 4 个元素），故落成 44 —— 数字差来自**卵**，不来自配方。
const liveEggArgv = `--ro-bind
/tmp/egg7verify/expanded/sysroot/bin
/bin
--ro-bind
/tmp/egg7verify/expanded/engine/lib
/lib64
--ro-bind
/tmp/egg7verify/expanded/engine/lib
/lib
--proc
/proc
--dev
/dev
--tmpfs
/tmp
--ro-bind
/sys
/sys
--dir
/models
--ro-bind
/tmp/egg7verify/expanded/engine
/engine
--ro-bind
/tmp/egg7verify/expanded/model/demo.gguf
/models/demo.gguf
--ro-bind
/tmp/egg7verify/expanded/libs
/libs
--dev-bind
/dev/null
/dev/null
--unshare-pid
--die-with-parent
--chdir
/tmp
--
/bin/sh
-c
export LD_LIBRARY_PATH='/libs/bin:/engine/bin'; exec "$@"
zerg-egg
/engine/bin/llama-server
--egg7-arg1
value1`

// liveEggBaselineRoot 冻进 liveEggArgv 的那枚蛋在 edge-node 上的展开落点（纯字符串比对，不 stat）。
const liveEggBaselineRoot = "/tmp/egg7verify/expanded"

// TestBuildBwrapArgv_LiveX3EggBaselineUnchanged 蛋模式 argv 逐元素冻结（零漂移判据）。
//
// 与 `TestBuildBwrapArgv_LiveX3BaselineUnchanged`（非蛋模式，90 元素）是一对：
//
//	非蛋：**改前改后逐字相同** ⇒ 那条基线一个元素都没动（这就是本单「非蛋模式逐字不动」的机器证据）；
//	蛋模式：本单把它从「含宿主 `/usr` 的 49/46 元素」改成「44 元素、`/bin` 来自蛋」⇒ 新序列冻在这里。
//	★ 不是删测试：旧的 90 元素那条**保留原样**（它守零回归），新的这条守**新形态**。
func TestBuildBwrapArgv_LiveX3EggBaselineUnchanged(t *testing.T) {
	argv, err := BuildBwrapArgv(liveEggSpec(liveEggBaselineRoot))
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	want := strings.Split(liveEggArgv, "\n")
	if len(argv) != len(want) {
		t.Fatalf("蛋模式 argv 元素数 %d ≠ 基线 %d（冻值漂了）\n实得：%v", len(argv), len(want), argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("蛋模式 argv[%d] 与真机基线不同：\n  实得 %q\n  基线 %q", i, argv[i], want[i])
		}
	}
	// 自证这条基线自己就是「真自包含」形态：一条宿主系统路径都不挂。
	if strings.Contains("\n"+liveEggArgv+"\n", "\n--ro-bind\n/usr\n/usr\n") {
		t.Fatal("冻住的蛋模式基线里仍含 `--ro-bind /usr /usr` —— 冻的是一份假自包含")
	}
}

// liveEggSpec 一枚**蛋模式**的声明（形状与 backend 映射侧产出的一致：引擎/库/权重/loader 全从蛋来，
// 只有 `sysroot/bin` 提供空间的 `/bin`）。两处用它：
//
//	① TestBuildBwrapArgv_LiveX3EggBaselineUnchanged —— 冻住真机实测出的**新 argv**；
//	② TestBuildBwrapArgv_EggModeRealBwrap —— 真 bwrap 跑一遍，验 rc=0 ＋ libc 的 (dev,inode) 等于蛋内那件。
//
// WorkDir 用 `/tmp`（空间内由 `--tmpfs /tmp` 提供）：真机夹具不必在宿主上先造一个可写目录，
// 而 `--chdir` 这条落点照样被覆盖（真机基线里它就在这一位）。
func liveEggSpec(root string) Spec {
	s := goodSpec()
	s.EggID = "egg7-live"
	s.EngineRoots = nil // 蛋模式：引擎源只有蛋
	s.EggRoot = root
	s.EggLoaderDir = filepath.Join(root, "engine", "lib")
	s.EggBinDir = filepath.Join(root, "sysroot", "bin")
	s.EnginePathInSpace = "/engine/bin/llama-server"
	s.EngineArgs = []string{"--egg7-arg1", "value1"}
	s.WeightPath = filepath.Join(root, "model")
	s.WeightFiles = []string{filepath.Join(root, "model", "demo.gguf")}
	s.ExtraROBinds = []string{
		filepath.Join(root, "model", "demo.gguf") + ":/models/demo.gguf",
		filepath.Join(root, "libs") + ":/libs",
	}
	s.ExtraRWBinds = nil
	s.Env = map[string]string{"LD_LIBRARY_PATH": "/libs/bin:/engine/bin"}
	s.WorkDir = "/tmp"
	s.Devices = []string{"/dev/null"}
	return s
}

// liveBwrapRoot 真机夹具的蛋展开目录（`ZERG_EGG_LIVE_ROOT`；空 ⇒ 用例跳过）。
func liveBwrapRoot(t *testing.T) string {
	t.Helper()
	root := strings.TrimSpace(os.Getenv("ZERG_EGG_LIVE_ROOT"))
	if root == "" {
		t.Skip("未给 ZERG_EGG_LIVE_ROOT（要一枚**真展开**的蛋目录）⇒ 跳过：自包含读数只能在真 bwrap 上取")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("本机没有 bwrap ⇒ 跳过：蛋模式 argv 的真机实证在带 bwrap 的 Linux 上跑")
	}
	if st, err := os.Stat(filepath.Join(root, "engine", "lib")); err != nil || !st.IsDir() {
		t.Fatalf("ZERG_EGG_LIVE_ROOT=%q 不像一枚展开的蛋（缺 engine/lib）：%v", root, err)
	}
	return root
}

// TestBuildBwrapArgv_EggModeRealBwrap 蛋模式真跑一格（**真自包含的唯一硬证**）。
//
// 三件事一起证，缺一不可：
//
//	① argv 里**没有** `--ro-bind /usr /usr`（宿主系统路径一条都不挂）；
//	② 真 exec **rc=0**，且空间内 `/proc/self/maps` 里 libc 的 **(dev, inode)** 逐值等于蛋内
//	   loader 目录里那件的 —— 宿主那件在另一块设备上（edge-node 实测宿主 dev 103:06 · 蛋 00:2e），
//	   dev 不同就说明「跑的还是宿主那份」，rc=0 本身证不了自包含；
//	③ 入口 `/bin/sh` 是**蛋里那枚**：空根里 `-- /bin/sh -c 'echo ok'` ⇒ rc=0 输出 ok
//	   （改前逐字 `bwrap: execvp /bin/sh: No such file or directory`，rc=1）。
//
// 跑法（edge-node）：
//
//	ZERG_EGG_LIVE_ROOT=/tmp/egg7verify/expanded go test -v -run EggModeRealBwrap ./internal/hatch/
//
// 加 `ZERG_EGG_LIVE_NEGCTL=1` 顺带跑负控：把蛋内 `libc.so.6` 临时改名 ⇒ 必须 rc=127 且逐字报缺
// `libc.so.6`（证 ② 那个 rc=0 不是又跑回宿主去了）；跑完立刻改回并断言复原。
func TestBuildBwrapArgv_EggModeRealBwrap(t *testing.T) {
	root := liveBwrapRoot(t)
	loader := filepath.Join(root, "engine", "lib")

	// 空间内自证脚本：只用 shell 内建（空根里没有 cat/ls/grep —— 这正是蛋模式的定义）。
	const script = `echo egg-live-ok; ` +
		`while IFS= read -r l; do case "$l" in *libc.so.6*) echo "maps:$l";; esac; done < /proc/self/maps; ` +
		`if [ -d /usr ]; then echo usr:PRESENT; else echo usr:ABSENT; fi; ` +
		`if [ -x /bin/sh ]; then echo sh:PRESENT; else echo sh:ABSENT; fi; ` +
		`if [ -w /dev/null ]; then echo devnull:WRITABLE; else echo devnull:RO; fi; ` +
		`if [ -e /etc/ld.so.cache ]; then echo ldso_cache:PRESENT; else echo ldso_cache:ABSENT; fi`

	s := liveEggSpec(root)
	s.EnginePathInSpace = shPath // 入口就是蛋里那枚 sh（包装层照留：Env 非空）
	s.EngineArgs = []string{"-c", script}

	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--ro-bind" && argv[i+1] == "/usr" && argv[i+2] == "/usr" {
			t.Fatalf("蛋模式 argv 里仍有 `--ro-bind /usr /usr`（假自包含）：%v", argv)
		}
	}
	t.Logf("argv 元素数 = %d（蛋模式；含宿主 /usr 的改前形态是 49）", len(argv))

	// ★ BuildBwrapArgv **不含** argv[0]（"bwrap" 由执行侧补：wall_bridge 的判据就是
	// `wall.argv[0] == "bwrap"` 且 `wall.argv[1:]` 与本函数输出逐条相同）⇒ 这里补上。
	out, err := exec.Command("bwrap", argv...).CombinedOutput()
	if err != nil {
		t.Fatalf("蛋模式真跑失败：%v\nargv=%v\nout=%s", err, argv, out)
	}
	got := string(out)
	for _, want := range []string{"egg-live-ok", "usr:ABSENT", "sh:PRESENT", "devnull:WRITABLE"} {
		if !strings.Contains(got, want) {
			t.Errorf("空间内自证缺 %q（输出=%q）—— 空根里这一条不成立", want, got)
		}
	}

	// ② 的硬证：maps 里的 (dev, inode) 必须等于蛋内那件
	dev, ino, ok := mapsLibcDevIno(got)
	if !ok {
		t.Fatalf("空间内 maps 里没读到 libc.so.6 的行（输出=%q）", got)
	}
	wantDev, wantIno, err := hostFileDevIno(filepath.Join(loader, "libc.so.6"))
	if err != nil {
		t.Fatalf("读蛋内 libc 的 (dev,inode)：%v", err)
	}
	if dev != wantDev || ino != wantIno {
		t.Fatalf("空间内 libc 的 (dev,inode) = (%s,%s)，蛋内那件 = (%s,%s) —— **不是同一件** ⇒ 还在用宿主/别处那份（自包含不成立）\n输出=%q",
			dev, ino, wantDev, wantIno, got)
	}
	t.Logf("真自包含：空间内 libc (dev,inode) = (%s,%s) 逐值等于蛋内 %s", dev, ino, filepath.Join(loader, "libc.so.6"))

	// ③ 那一格单独再跑一遍（入口直接是 /bin/sh，不带包装层）
	shSpec := liveEggSpec(root)
	shSpec.EnginePathInSpace = shPath
	shSpec.EngineArgs = []string{"-c", "echo ok"}
	shSpec.Env = nil
	shArgv, err := BuildBwrapArgv(shSpec)
	if err != nil {
		t.Fatalf("BuildBwrapArgv（sh 格）：%v", err)
	}
	shOut, shErr := exec.Command("bwrap", shArgv...).CombinedOutput()
	if shErr != nil || strings.TrimSpace(string(shOut)) != "ok" {
		t.Fatalf("空根里 `-- /bin/sh -c 'echo ok'` 应是 rc=0 输出 ok，实得 rc=%v 输出=%q", shErr, shOut)
	}
	t.Logf("sh 格：rc=0 输出=%q（改前是 `bwrap: execvp /bin/sh: No such file or directory`）", strings.TrimSpace(string(shOut)))

	// 负控（可选）：蛋内 libc 一改名就必须 rc≠0 且具名报缺 —— 证 rc=0 不是又在用宿主那份
	if os.Getenv("ZERG_EGG_LIVE_NEGCTL") != "1" {
		t.Log("（负控未跑：ZERG_EGG_LIVE_NEGCTL≠1）")
		return
	}
	libc := filepath.Join(loader, "libc.so.6")
	hidden := libc + ".negctl"
	if err := os.Rename(libc, hidden); err != nil {
		t.Fatalf("负控：改名蛋内 libc 失败：%v", err)
	}
	defer func() {
		if rerr := os.Rename(hidden, libc); rerr != nil {
			t.Fatalf("负控复原失败（**夹具已被改动**，要人工把 %s 改回 %s）：%v", hidden, libc, rerr)
		}
	}()
	negOut, negErr := exec.Command("bwrap", shArgv...).CombinedOutput()
	if negErr == nil {
		t.Fatalf("负控失效：蛋内 libc 已改名，空间里却照样 rc=0（说明它在用宿主那份）输出=%q", negOut)
	}
	if !strings.Contains(string(negOut), "libc.so.6") {
		t.Errorf("负控输出应逐字点名缺 libc.so.6，实得 %q", negOut)
	}
	if code := exitCodeOf(negErr); code != 127 {
		t.Errorf("负控退码应是 127（loader 找不到 libc），实得 %d", code)
	}
	t.Logf("负控：蛋内 libc 改名 ⇒ rc=%d 输出=%q", exitCodeOf(negErr), strings.TrimSpace(string(negOut)))
}

// mapsLibcDevIno 从空间内 `maps:` 行里取 libc.so.6 的 (dev, inode)。
// maps 行形如 `<addr> <perms> <offset> 00:2e 17349 /lib/libc.so.6` ⇒ 第 4、5 个字段。
func mapsLibcDevIno(out string) (dev, ino string, ok bool) {
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimPrefix(strings.TrimSpace(ln), "maps:")
		if !strings.Contains(ln, "libc.so.6") {
			continue
		}
		f := strings.Fields(ln)
		if len(f) < 5 {
			continue
		}
		return f[3], f[4], true
	}
	return "", "", false
}

// hostFileDevIno 宿主侧文件的 (dev, inode)，格式与 `/proc/self/maps` 逐字同（`%02x:%02x` ＋ 十进制）。
// 只在 Linux 上有意义（本用例的前提就是 bwrap）；dev_t 的拆位按 Linux 内核的 GNU 布局算。
func hostFileDevIno(p string) (dev, ino string, err error) {
	fi, serr := os.Stat(p)
	if serr != nil {
		return "", "", serr
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", os.ErrInvalid
	}
	raw := uint64(st.Dev)
	major := uint32((raw >> 8) & 0xfff)
	major |= uint32((raw >> 32) &^ 0xfff)
	minor := uint32(raw & 0xff)
	minor |= uint32((raw >> 12) &^ 0xff)
	return fmt.Sprintf("%02x:%02x", major, minor), strconv.FormatUint(uint64(st.Ino), 10), nil
}

// exitCodeOf 取 exec 的进程退码（不是 `*ExitError` 或没有退码 ⇒ -1）。
func exitCodeOf(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
