// egg_hatch_wiring_test.go —— 卵沙箱第 5 步：**从蛋孵化**时挂载源换成「蛋展开后的目录」。
//
// 夹具是一枚**真形状的小蛋**（1 个假引擎 + loader + libs/bin + rocblas/5.1.0/library），
// 走的是**真链路**：`hatchSpecFor(...)` → `hatch.BuildBwrapArgv`（不另写一份配方副本 ——
// 两份口径必然漂，漂了还都不报红）。钉住的四件事：
//
//	① 挂了什么：`<蛋>/engine` → `/engine`、`<蛋>/libs` → `/libs`、`<蛋>/engine/lib` → `/lib64` 与 `/lib`、
//	   `<蛋>/sysroot/bin` → `/bin`；**且一条宿主系统路径都不挂**（不挂 `/usr`、不留 usrmerge 的
//	   `/bin` `/sbin` 链接 —— 否则「从蛋自包含」是假象，空间里的 libc 会解析到宿主那件，GAP-20261018-11）；
//	② 引擎在空间里叫什么：`/engine/<目录基名>/<可执行名>`（蛋内保留了 `--engine-dir` 的基名）；
//	③ 环境：`LD_LIBRARY_PATH` = 声明库落点 ＋ 引擎侧目录（蛋里的引擎不在 `/engine` 根上）；
//	   `ROCBLAS_TENSILE_LIBPATH` = 空间内 `.hsaco` 目录（DT_NEEDED 看不见的那 556 MB 的入口）；
//	④ fail-closed：蛋里缺库目录 / 同名引擎两份 / 两个 rocBLAS 版本层 / 蛋目录读不到 /
//	   缺 `sysroot/bin/sh`（蛋模式的 `/bin` 无人提供，包装 exec 会 execvp 失败，GAP-20261018-12）
//	   —— 全部具名拒孵。
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// eggFixture 造一枚夹具蛋（形状与 `cocoon pack --engine-dir` 的落点一致）。
func eggFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("建 %s: %v", p, err)
		}
		return p
	}
	file := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.WriteFile(p, []byte("fixture"), 0o755); err != nil {
			t.Fatalf("写 %s: %v", p, err)
		}
	}
	dir("engine/bin")
	file("engine/bin/llama-server")
	dir("engine/lib")
	file("engine/lib/ld-linux-x86-64.so.2")
	file("engine/lib/libc.so.6")
	dir("libs/bin")
	file("libs/bin/libggml-hip.so.0")
	dir("engine/rocblas/5.1.0/library")
	file("engine/rocblas/5.1.0/library/TensileLibrary_gfx1100.hsaco")
	// 蛋内系统镜像里的 sh（GAP-20261018-12）：蛋模式不挂宿主 /usr ⇒ /bin 只能由蛋给，
	// 而包装 exec 的入口就是它。夹具里是**普通可执行件**（真蛋里是真 sh）。
	dir("sysroot/bin")
	file("sysroot/bin/sh")
	return root
}

// eggEntry 一枚声明了 `egg_dir` 的卵（其余字段与主线卵同形）。
func eggEntry(root string) *registry.ModelEntry {
	e := &registry.ModelEntry{
		File:          "/data/models/k2/k2horizon-q4_k_m.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		// 引擎的**声明面**仍写宿主路径：蛋孵化只换「挂载源」，不要求宿主上真有这个件
		// （`eggExeBase` 只取基名，`/engine/<目录基名>/llama-server` 由蛋内布局决定）。
		Cmd: registry.CmdString("/home/<user>/llama.cpp-src/build-hip-flash/bin/llama-server -m {file} --port {port}"),
		EnvReq: &registry.EnvReq{
			Devices:  []string{"0"},
			LibPaths: []string{"/home/<user>/lib/bin"},
			Env:      map[string]string{"HSA_OVERRIDE_GFX_VERSION": "11.0.0"},
		},
		Custom: map[string]interface{}{"egg_dir": root},
	}
	e.SetEggNameForTest("example-moe-36b")
	return e
}

func argvHas(argv []string, want ...string) bool {
	for i := 0; i+len(want)-1 < len(argv); i++ {
		ok := true
		for j, w := range want {
			if argv[i+j] != w {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// TestEggHatch_MountsComeFromExpandedEgg 从蛋孵化：argv 里 /engine 与 /libs 指向**展开目录**。
func TestEggHatch_MountsComeFromExpandedEgg(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)

	spec, err := hatchSpecFor(eggEntry(root), "llama-server", 58100, hatchTestProfile())
	if err != nil {
		t.Fatalf("hatchSpecFor: %v", err)
	}
	if spec.EggRoot != root || spec.EggLoaderDir != filepath.Join(root, "engine", "lib") {
		t.Fatalf("蛋挂载源没接上：EggRoot=%q EggLoaderDir=%q（期望 %q / %q）",
			spec.EggRoot, spec.EggLoaderDir, root, filepath.Join(root, "engine/lib"))
	}
	if spec.EggBinDir != filepath.Join(root, "sysroot", "bin") {
		t.Fatalf("蛋内 /bin 来源没接上：EggBinDir=%q（期望 %q）", spec.EggBinDir, filepath.Join(root, "sysroot/bin"))
	}
	if len(spec.EngineRoots) != 0 {
		t.Errorf("蛋孵化时不该再有宿主 EngineRoots，实得 %v", spec.EngineRoots)
	}
	if spec.EnginePathInSpace != "/engine/bin/llama-server" {
		t.Errorf("引擎在空间内应是 /engine/<目录基名>/<可执行名>，实得 %q", spec.EnginePathInSpace)
	}
	if got, want := spec.Env["LD_LIBRARY_PATH"], "/libs/bin:/engine/bin:/engine/lib"; got != want {
		t.Errorf("LD_LIBRARY_PATH 应是 %q，实得 %q", want, got)
	}
	if got, want := spec.Env["ROCBLAS_TENSILE_LIBPATH"], "/opt/rocm/lib/rocblas/5.1.0/library"; got != want {
		t.Errorf("ROCBLAS_TENSILE_LIBPATH 应是 %q，实得 %q", want, got)
	}
	if spec.Env["HSA_OVERRIDE_GFX_VERSION"] != "11.0.0" {
		t.Errorf("按卵声明的环境变量被弄丢了：%v", spec.Env)
	}

	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("BuildBwrapArgv: %v", err)
	}
	for _, want := range [][]string{
		{"--ro-bind", filepath.Join(root, "engine"), "/engine"},
		{"--ro-bind", filepath.Join(root, "libs"), "/libs"},
		{"--ro-bind", filepath.Join(root, "engine", "lib"), "/lib64"},
		{"--ro-bind", filepath.Join(root, "engine", "lib"), "/lib"},
		{"--ro-bind", filepath.Join(root, "engine", "rocblas"), "/opt/rocm/lib/rocblas"},
		{"--ro-bind", filepath.Join(root, "sysroot", "bin"), "/bin"},
	} {
		if !argvHas(argv, want...) {
			t.Errorf("缺绑定 %v（实得 argv：%v）", want, argv)
		}
	}
	// 蛋模式**一条宿主系统路径都不挂**（GAP-20261018-11「假自包含」）：/usr 与两条 usrmerge 别名退场。
	if argvHas(argv, "--ro-bind", "/usr", "/usr") {
		t.Errorf("蛋模式仍在挂宿主 /usr ⇒ libc 会解析到宿主那件（假自包含）：%v", argv)
	}
	if argvHas(argv, "--symlink", "usr/bin", "/bin") || argvHas(argv, "--symlink", "usr/sbin", "/sbin") {
		t.Errorf("蛋模式不该再有 usrmerge 的 /bin /sbin 链接（指向已不存在的 /usr）：%v", argv)
	}
	// 宿主构建目录不得再出现在空间里（换源的关键判据）
	for _, a := range argv {
		if strings.HasPrefix(a, "/home/<user>/llama.cpp-src") {
			t.Errorf("蛋孵化时宿主引擎目录不该出现在 argv 里：%q", a)
		}
	}
	// usrmerge 的 /lib /lib64 链接必须退场（两套落点会打架）
	if argvHas(argv, "--symlink", "usr/lib", "/lib") || argvHas(argv, "--symlink", "usr/lib64", "/lib64") {
		t.Errorf("/lib 与 /lib64 必须由蛋提供，不该再有 usrmerge 链接：%v", argv)
	}
	// 包封契约：映射产物要过 hatch.Spec.Validate（否则真孵化在 Validate 就拒了）
	if err := spec.Validate(); err != nil {
		t.Errorf("蛋孵化映射产物过不了 Validate：%v", err)
	}
}

// TestEggHatch_NoLibPathsStillGeneratesSearchPath 蛋孵化且**一个库目录都没声明** ⇒
// `LD_LIBRARY_PATH` 仍必须生成（蛋里的引擎在 `/engine/<目录基名>`、不在 `/engine` 根上，
// 它的 RUNPATH 写死宿主构建目录 ⇒ 没有这条变量在空间里就是 127）。
func TestEggHatch_NoLibPathsStillGeneratesSearchPath(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	e := eggEntry(root)
	e.EnvReq = &registry.EnvReq{Devices: []string{"0"}} // 声明了 env_req，但没有 lib_paths
	spec, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err != nil {
		t.Fatalf("hatchSpecFor: %v", err)
	}
	if got, want := spec.Env["LD_LIBRARY_PATH"], "/engine/bin:/engine/lib"; got != want {
		t.Errorf("无 lib_paths 的蛋孵化也要生成搜索路径 %q，实得 %q", want, got)
	}
	// 蛋里带 `libs/` 就挂到 `/libs`（它是**蛋内容的一部分**，与「<蛋>/libs → /libs」这条落点约定一致）；
	// 但**进不进搜索路径**由声明决定 —— 没声明就一个字节都不进 LD_LIBRARY_PATH（上面那条断言）。
	if _, err := hatch.BuildBwrapArgv(spec); err != nil {
		t.Fatalf("BuildBwrapArgv: %v", err)
	}
}

// TestEggHatch_FailClosed 蛋形状不对 ⇒ 具名拒孵（✗ 不退回宿主挂载 —— 静默换源最坏）。
func TestEggHatch_FailClosed(t *testing.T) {
	cases := []struct {
		name string
		prep func(t *testing.T, root string)
		want string
	}{
		{
			name: "蛋里缺 libs/<声明的目录名>",
			prep: func(t *testing.T, root string) {
				if err := os.RemoveAll(filepath.Join(root, "libs", "bin")); err != nil {
					t.Fatal(err)
				}
			},
			want: "libs/bin",
		},
		{
			name: "蛋里两份同名引擎",
			prep: func(t *testing.T, root string) {
				p := filepath.Join(root, "engine", "stage")
				if err := os.MkdirAll(p, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(p, "llama-server"), []byte("x"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "同名引擎",
		},
		{
			name: "蛋里两个 rocBLAS 版本层",
			prep: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "engine", "rocblas", "6.0.0", "library"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "版本层",
		},
		{
			name: "蛋内 loader 目录里没有 loader",
			prep: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "engine", "lib", "ld-linux-x86-64.so.2")); err != nil {
					t.Fatal(err)
				}
			},
			want: "没有 ld-linux",
		},
		{
			// GAP-20261018-12：蛋模式不挂宿主 /usr ⇒ 空间里的 /bin/sh 只能由蛋提供。
			name: "蛋里没有 sysroot/bin 这一层",
			prep: func(t *testing.T, root string) {
				if err := os.RemoveAll(filepath.Join(root, "sysroot")); err != nil {
					t.Fatal(err)
				}
			},
			want: "没有 sysroot/bin",
		},
		{
			name: "蛋里 sysroot/bin 里没有 sh",
			prep: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "sysroot", "bin", "sh")); err != nil {
					t.Fatal(err)
				}
			},
			want: `没有 "sh"`,
		},
		{
			name: "蛋里 sysroot/bin/sh 不可执行",
			prep: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "sysroot", "bin", "sh"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "不是可执行的普通件",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withWorkDirRoot(t)
			root := eggFixture(t)
			c.prep(t, root)
			if _, err := hatchSpecFor(eggEntry(root), "llama-server", 58100, hatchTestProfile()); err == nil {
				t.Fatalf("应拒孵（%s），却产出了声明", c.name)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("拒孵理由应含 %q，实得：%v", c.want, err)
			}
		})
	}
}
