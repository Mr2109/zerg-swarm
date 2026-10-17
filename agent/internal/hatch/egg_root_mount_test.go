// egg_root_mount_test.go —— 卵沙箱第 5 步：**从蛋孵化**时挂载源换成蛋展开目录。
//
// 钉住两件事（任一条被改坏，本文件先红）：
//
//	① **零回归**：不是从蛋孵化时，bwrap argv **逐元素**与改动前逐字相同 —— 基线取自真机现读
//	   （edge-node `/proc/285.../cmdline`，2026-10-18，90 个 argv 元素），不是手抄的「应该长这样」；
//	② **换源生效**：EggRoot/EggLoaderDir 非空时 `/engine` 指向 `<蛋>/engine`、`/lib64` 与 `/lib`
//	   指向蛋内 loader 目录（PT_INTERP 的落点，方案 A：不改二进制），且 usrmerge 那两条
//	   `/lib` `/lib64` 符号链接**不再出现**（否则就是两套落点打架）。
//
// 配方其余部分（`--proc` / `--dev` / `--tmpfs /tmp` / `--ro-bind /sys /sys` / `--dir /models` /
// 设备 `--dev-bind` / `--unshare-pid`）在蛋模式下**一字不动** —— 它们与「引擎从哪来」无关。
package hatch

import (
	"strings"
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
		"--symlink\nusr/bin\n/bin", "--symlink\nusr/sbin\n/sbin",
		"--proc\n/proc", "--dev\n/dev", "--tmpfs\n/tmp", "--ro-bind\n/sys\n/sys",
		"--dir\n" + spaceModelsDir,
		"--dev-bind\n/dev/kfd\n/dev/kfd", "--unshare-pid", "--die-with-parent",
	} {
		if !has(keep) {
			t.Errorf("蛋孵化把配方其余部分改坏了：缺 %q", keep)
		}
	}
	// 宿主构建目录不得再出现在 /engine 上（换源的关键判据）
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--ro-bind" && argv[i+2] == "/engine" && argv[i+1] != eggRoot+"/engine" {
			t.Errorf("蛋孵化时 /engine 仍从 %q 来（应只有 %q）", argv[i+1], eggRoot+"/engine")
		}
	}
}

// TestSpec_ValidateEggPairing 蛋的两个字段同进同出、且与宿主挂载源互斥。
func TestSpec_ValidateEggPairing(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Spec)
		want string
	}{
		{"只给 EggRoot", func(s *Spec) { s.EggRoot = "/e"; s.EngineRoots = nil }, "不完整"},
		{"只给 EggLoaderDir", func(s *Spec) { s.EggLoaderDir = "/e/engine/lib" }, "不完整"},
		{"蛋 ＋ 宿主挂载源并存", func(s *Spec) { s.EggRoot = "/e"; s.EggLoaderDir = "/e/engine/lib" }, "两套引擎挂载源"},
		{"蛋根不是绝对路径", func(s *Spec) { s.EggRoot = "e"; s.EggLoaderDir = "/e/engine/lib"; s.EngineRoots = nil }, "绝对路径"},
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
