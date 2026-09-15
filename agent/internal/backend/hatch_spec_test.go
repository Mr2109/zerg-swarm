package backend

// hatch_spec_test.go —— 批 1 验收：卵声明 → hatch.Spec 的映射（纯函数，跨平台可跑）。
//
// 钉住的语义（任一条被改坏，本文件先红）：
//   - 主线卵与非主线卵各一例映射正确：权重路径改写为空间内路径、EngineRoots/EnginePathInSpace
//     来自「实际会执行的可执行」、Env 含按卵声明的环境变量与卡号；
//   - fail-closed：卵名 / 权重 / 引擎路径拿不到一律报错（不编造默认值）；
//   - 一次性工作目录按卵分开且防路径穿越；
//   - 映射产物能过 hatch.Spec.Validate（即「原件齐的卵真的能孵」）。

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// hatchTestProfile 一份能过 monitor.EggProfile.Validate 的实测档案（calib_runs≥3、字段全正）。
func hatchTestProfile() monitor.EggProfile {
	return monitor.EggProfile{
		WeightSizeGb:         90,
		PeakGttGb:            32.7,
		PeakMemGb:            40,
		LoadSeconds:          120,
		ThroughputTokS:       4.8,
		SuggestedIdleUnloadS: 600,
		SchemaVersion:        1,
		MeasuredAt:           time.Now(),
		Machine:              "x3",
		CalibRuns:            3,
	}
}

// withMainlineEngineProbe 把主线引擎探测换成一个确定值（本机未必装了 llama-server，
// 映射规则本身不该挂在本机装没装引擎上）。
func withMainlineEngineProbe(t *testing.T, hostPath string) {
	t.Helper()
	old := mainlineEngineProbe
	mainlineEngineProbe = func() string { return hostPath }
	t.Cleanup(func() { mainlineEngineProbe = old })
}

// withWorkDirRoot 把一次性工作目录根指到临时目录（单测不许往家目录里写东西）。
func withWorkDirRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(EnvWorkDirRoot, root)
	return root
}

// TestHatchSpec_MainlineEgg 主线卵（cmd: 缺省 ⇒ detectLlamaServerPath 一路）：
// 权重改写、引擎根目录、env（含按卵覆盖 LD_LIBRARY_PATH + 卡号）、设备节点、工作目录。
func TestHatchSpec_MainlineEgg(t *testing.T) {
	const engineHost = "/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server"
	withMainlineEngineProbe(t, engineHost)
	workRoot := withWorkDirRoot(t)

	// 库目录（env_req.lib_paths）必须在宿主上真实存在（2026-09-15 拍：不存在 ⇒ 拒孵）
	// ⇒ 单测用临时目录当库目录，目录名要与断言一致。
	libDir := filepath.Join(t.TempDir(), "build-hip-flash")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/glm-5.3/GLM-5.3-Flash-Q4_K_M.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		EnvReq: &registry.EnvReq{
			Devices:   []string{"1"}, // 用第 1 张卡
			LibPaths:  []string{libDir},
			Env:       map[string]string{"LD_LIBRARY_PATH": "/engine", "HSA_OVERRIDE_GFX_VERSION": "11.0.0"},
			MemlockKB: 1015488,
		},
	}
	entry.SetEggNameForTest("GLM-5.3-Flash")

	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9411, hatchTestProfile())
	if err != nil {
		t.Fatalf("主线卵映射应成功，实得 %v", err)
	}

	if spec.EggID != "GLM-5.3-Flash" {
		t.Errorf("卵名应取注册表键，实得 %q", spec.EggID)
	}
	if spec.SchemaVersion != registry.EggSchemaVersionCurrent {
		t.Errorf("schema_version 应原样带上，实得 %d", spec.SchemaVersion)
	}
	// 权重：WeightPath 仍是权重文件所在**宿主目录**（缺陷 12 之后它只用于出证/日志：hatch 侧的
	// 整目录挂载必须同批去掉）；真正决定「空间里挂了什么」的是下面逐文件的只读绑定。
	if spec.WeightPath != "/data/models/glm-5.3" {
		t.Errorf("WeightPath 应是权重文件所在目录（出证/日志用），实得 %q", spec.WeightPath)
	}
	// 引擎：宿主目录进 EngineRoots，空间内路径 = /engine/<可执行名>
	if len(spec.EngineRoots) != 1 || spec.EngineRoots[0] != "/home/g01/llama.cpp-src/build-hip-flash/bin" {
		t.Errorf("EngineRoots 应是引擎所在宿主目录，实得 %v", spec.EngineRoots)
	}
	if spec.EnginePathInSpace != "/engine/llama-server" {
		t.Errorf("空间内引擎路径应为 /engine/<可执行名>，实得 %q", spec.EnginePathInSpace)
	}
	// 参数：权重路径改写成空间内路径，宿主路径不得出现
	if got := argAfter(spec.EngineArgs, "-m"); got != "/models/GLM-5.3-Flash-Q4_K_M.gguf" {
		t.Errorf("-m 应改写成空间内路径 /models/<basename>，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	if got := argAfter(spec.EngineArgs, "--port"); got != "9411" {
		t.Errorf("--port 应带上本端分配的端口，实得 %q", got)
	}
	for _, a := range spec.EngineArgs {
		if strings.Contains(a, "/data/models/") {
			t.Errorf("参数里不得残留宿主权重路径：%q", a)
		}
	}
	// 环境：按卵声明照单执行 + 卡号补 HIP_VISIBLE_DEVICES（已声明的以声明为准）
	if spec.Env["LD_LIBRARY_PATH"] != "/engine" {
		t.Errorf("按卵覆盖的 LD_LIBRARY_PATH 应原样带上，实得 %q", spec.Env["LD_LIBRARY_PATH"])
	}
	if spec.Env["HSA_OVERRIDE_GFX_VERSION"] != "11.0.0" {
		t.Errorf("卵声明的其它环境变量应照单带上，实得 %v", spec.Env)
	}
	if spec.Env["HIP_VISIBLE_DEVICES"] != "1" {
		t.Errorf("声明了卡号应补 HIP_VISIBLE_DEVICES=1，实得 %q", spec.Env["HIP_VISIBLE_DEVICES"])
	}
	// 库目录落点（env_req.lib_paths → /libs/<目录名>，**只读**）：声明了就必须有一条只读绑定。
	// 这里声明的目录名就是 build-hip-flash ⇒ 落点 /libs/build-hip-flash。
	// 顺序口径（本文件写死）：**文件类绑定在前（本卵权重 → mmproj → 模板），库目录绑定在后**。
	wantWeightBind := entry.File + ":/models/GLM-5.3-Flash-Q4_K_M.gguf"
	wantLibBind := libDir + ":/libs/build-hip-flash"
	if len(spec.ExtraROBinds) != 2 || spec.ExtraROBinds[0] != wantWeightBind || spec.ExtraROBinds[1] != wantLibBind {
		t.Errorf("只读绑定应为 [权重文件 → /models/<基名>, 库目录 → /libs/<目录名>]（%q / %q），实得 %v",
			wantWeightBind, wantLibBind, spec.ExtraROBinds)
	}
	// 只读绑定里不得出现「整目录挂 /models」的形态（缺陷 12：那会让同目录别的模型也进空间）
	for _, b := range spec.ExtraROBinds {
		if strings.HasSuffix(b, ":/models") {
			t.Errorf("不得再有「整目录挂到 /models」的绑定（缺陷 12），实得 %q", b)
		}
	}
	// 设备：卡号 ⇒ /dev/kfd + 该卡渲染节点（第 1 张 = renderD129）
	wantDev := []string{"/dev/kfd", "/dev/dri/renderD129"}
	if len(spec.Devices) != 2 || spec.Devices[0] != wantDev[0] || spec.Devices[1] != wantDev[1] {
		t.Errorf("声明卡号 1 时应挂 renderD129，实得 %v", spec.Devices)
	}
	// 工作目录：WorkDir 是**空间内**路径；宿主每卵目录经可写绑定落进空间（§6.6「一次性」侧）
	hostWork := filepath.Join(workRoot, "GLM-5.3-Flash")
	if spec.WorkDir != "/work" {
		t.Errorf("WorkDir 必须是空间内路径 /work（宿主路径填它 ⇒ 真孵化 --chdir 必失败），实得 %q", spec.WorkDir)
	}
	if st, err := os.Stat(hostWork); err != nil || !st.IsDir() {
		t.Errorf("宿主一次性工作目录应已被创建，stat 结果 err=%v", err)
	}
	if want := hostWork + ":/work"; len(spec.ExtraRWBinds) != 1 || spec.ExtraRWBinds[0] != want {
		t.Errorf("宿主工作目录应可写绑到 /work（%q），实得 %v", want, spec.ExtraRWBinds)
	}
	// 未声明 KV 盘 ⇒ 一个 KV 相关的东西都不许有（不编造）
	for _, b := range spec.ExtraRWBinds {
		if strings.Contains(b, "/kvdisk") {
			t.Errorf("未声明 KV 盘不得出现 /kvdisk 绑定，实得 %v", spec.ExtraRWBinds)
		}
	}
	if got := argAfter(spec.EngineArgs, "--kv-disk-dir"); got != "" {
		t.Errorf("未声明 KV 盘不得出现 --kv-disk-dir，实得 %q", got)
	}
	// mmap 限额：memlock_kb 换算成字节（§6.7 C②：给不足直接崩）
	if spec.MemlockBytes != 1015488*1024 {
		t.Errorf("MemlockBytes 应 = memlock_kb×1024，实得 %d", spec.MemlockBytes)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
	}
}

// TestHatchSpec_LibPathsLandingAndEnvRewrite env_req.lib_paths 的三件事（2026-09-15 拍，
// 落点在缺陷 5 修正后为 /libs/<目录名>）：
//
//	① 每个宿主库目录**只读**落到 /libs/<目录名>（目录名 = filepath.Base(清理后路径)；独立只读
//	   挂载点 —— 原来落 /engine/lib/<名>，而 /engine 是只读挂载，bwrap 建不出中间目录）；
//	② 环境变量的**值**里出现的库宿主路径（按 `:` 切段）改写成对应空间内路径 —— 整段精确匹配，
//	   不做前缀/子串替换；空串保持清空语义；没在 lib_paths 里声明的宿主路径不管；
//	③ 卵**自己**显式声明了 LD_LIBRARY_PATH ⇒ 生成逻辑一句都不补（不出现「两头都写」）。
func TestHatchSpec_LibPathsLandingAndEnvRewrite(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	root := t.TempDir()
	libA := filepath.Join(root, "build-k2") // 目录名 build-k2
	libB := filepath.Join(root, "hipflash") // 目录名 hipflash
	undeclared := filepath.Join(root, "not-declared")
	for _, d := range []string{libA, libB, undeclared} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/k2/k2horizon.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		EnvReq: &registry.EnvReq{
			LibPaths: []string{libA, libB},
			Env: map[string]string{
				"LD_LIBRARY_PATH": libA + ":/shared/other:" + libB,
				"K2_CLEAR":        "",                          // 故意清空（§6.7 C③）⇒ 原样、不改写、不报错
				"SINGLE":          libA,                        // 单段同样改写
				"UNRELATED":       undeclared + ":" + libA,     // 未声明的那一段不管，声明的那一段照改
				"PREFIX_TRAP":     libA + "x:" + libA + "/sub", // 前缀 / 子串形态一律不得被改
			},
		},
	}
	entry.SetEggNameForTest("K2-Lib-Test")

	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9000, hatchTestProfile())
	if err != nil {
		t.Fatalf("映射应成功，实得 %v", err)
	}

	// ① 落点：按声明顺序各占一格，且是**只读**通道；权重文件绑定在前（逐文件挂载，缺陷 12）
	want := []string{
		"/data/models/k2/k2horizon.gguf:/models/k2horizon.gguf",
		libA + ":/libs/build-k2",
		libB + ":/libs/hipflash",
	}
	if len(spec.ExtraROBinds) != 3 || spec.ExtraROBinds[0] != want[0] ||
		spec.ExtraROBinds[1] != want[1] || spec.ExtraROBinds[2] != want[2] {
		t.Errorf("只读绑定应为 %v，实得 %v", want, spec.ExtraROBinds)
	}
	// 命令行那一侧真跑一遍：必须落成 --ro-bind，不得落成可写绑定
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("映射产物应能构造出 bwrap 命令行，实得 %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, wantBind := range []string{
		"--ro-bind " + libA + " /libs/build-k2",
		"--ro-bind " + libB + " /libs/hipflash",
		"--ro-bind /data/models/k2/k2horizon.gguf /models/k2horizon.gguf",
	} {
		if !strings.Contains(joined, wantBind) {
			t.Errorf("bwrap 命令行缺 %q，实得 %q", wantBind, joined)
		}
	}
	// 旧落点（嵌在只读 /engine 之下）必须彻底消失：bwrap 在那里建不出中间目录（缺陷 5 原文
	// `Can't mkdir parents for /engine/lib/bin: Read-only file system`）
	if strings.Contains(joined, "/engine/lib/") {
		t.Errorf("不得再出现 /engine/lib/<名> 落点（缺陷 5：只读挂载之下建不出中间目录）：%q", joined)
	}
	if strings.Contains(joined, "--bind "+libA+" ") || strings.Contains(joined, "--bind "+libB+" ") {
		t.Errorf("库目录是只读输入，不得落成可写绑定：%q", joined)
	}

	// ② 值改写：整段精确匹配（③ 一并核：显式声明过 ⇒ 生成逻辑不许掺进来）
	cases := map[string]string{
		"LD_LIBRARY_PATH": "/libs/build-k2:/shared/other:/libs/hipflash",
		"K2_CLEAR":        "",
		"SINGLE":          "/libs/build-k2",
		"UNRELATED":       undeclared + ":/libs/build-k2",
		"PREFIX_TRAP":     libA + "x:" + libA + "/sub",
	}
	for k, wantV := range cases {
		if got := spec.Env[k]; got != wantV {
			t.Errorf("%s 应为 %q，实得 %q", k, wantV, got)
		}
	}
	if got := spec.Env["LD_LIBRARY_PATH"]; strings.Contains(got, spaceEngineDir) {
		t.Errorf("卵自己显式声明了 LD_LIBRARY_PATH ⇒ 生成逻辑一句都不许补（不得出现 %s），实得 %q",
			spaceEngineDir, got)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
	}
}

// TestHatchSpec_LibPathsGeneratesLDLibraryPath 缺陷 6（2026-09-15 真机实测）：
// 引擎 RUNPATH 写死宿主构建目录（`readelf -d` → `RUNPATH [/home/g01/…/build-hip-flash/bin:]`）
// ⇒ 空间内必须有人补上 LD_LIBRARY_PATH，否则 `libllama-server-impl.so` 找不到 + 单元 127。
// 口径三条（逐条钉住）：
//
//	① 卵声明了 lib_paths 且**没**自己写 LD_LIBRARY_PATH ⇒ 孵化器生成「声明的库落点（按声明顺序）
//	   在前、/engine 垫尾」；
//	② 卵自己显式写了（含空串 = 清空）⇒ **一句都不补**（不许两头都写，谁赢说不清）；
//	③ 没声明 lib_paths ⇒ 一个变量都不发明。
func TestHatchSpec_LibPathsGeneratesLDLibraryPath(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	root := t.TempDir()
	libA := filepath.Join(root, "a", "build-hip-flash")
	libB := filepath.Join(root, "b", "vendor-libs")
	for _, d := range []string{libA, libB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name    string
		envReq  *registry.EnvReq
		wantLDP string
		wantSet bool // LD_LIBRARY_PATH 该不该出现在 Env 里
	}{
		{
			name:    "声明了库目录、没自己写 ⇒ 生成（声明在前、/engine 垫尾）",
			envReq:  &registry.EnvReq{LibPaths: []string{libA, libB}},
			wantLDP: "/libs/build-hip-flash:/libs/vendor-libs:/engine",
			wantSet: true,
		},
		{
			name:    "卵自己写了 ⇒ 以卵的为准，一句都不补",
			envReq:  &registry.EnvReq{LibPaths: []string{libA}, Env: map[string]string{"LD_LIBRARY_PATH": "/自己写的:/engine"}},
			wantLDP: "/自己写的:/engine",
			wantSet: true,
		},
		{
			name:    "卵显式清空（空串）⇒ 保持清空语义，不补",
			envReq:  &registry.EnvReq{LibPaths: []string{libA}, Env: map[string]string{"LD_LIBRARY_PATH": ""}},
			wantLDP: "",
			wantSet: true,
		},
		{
			name:    "没声明 lib_paths ⇒ 不发明这个变量",
			envReq:  &registry.EnvReq{Env: map[string]string{"HSA_OVERRIDE_GFX_VERSION": "11.0.0"}},
			wantLDP: "",
			wantSet: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := &registry.ModelEntry{
				Backend:       "llama-server",
				File:          "/data/models/k2/k2horizon.gguf",
				SchemaVersion: registry.EggSchemaVersionCurrent,
				EnvReq:        c.envReq,
			}
			e.SetEggNameForTest("K2-Lib-Test")
			spec, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
			if err != nil {
				t.Fatalf("映射应成功，实得 %v", err)
			}
			got, ok := spec.Env["LD_LIBRARY_PATH"]
			if ok != c.wantSet {
				t.Fatalf("LD_LIBRARY_PATH 是否该出现：want %v，实得 %v（Env=%v）", c.wantSet, ok, spec.Env)
			}
			if got != c.wantLDP {
				t.Errorf("LD_LIBRARY_PATH 应为 %q，实得 %q", c.wantLDP, got)
			}
			if err := spec.Validate(); err != nil {
				t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
			}
		})
	}
}

// TestHatchSpec_OnlyDeclaredWeightFiles 缺陷 12（2026-09-15 真机实测）：口径是「只挂该卵自己的
// 权重」，而旧实现挂的是权重**所在目录** ⇒ 空间内 `ls /models` 能列出同目录的别的模型
// （真机列出 Qwen2.5-VL-32B / Qwen3.6-35B / Flash-Next …）。本用例逐字钉住修法：
//
//	① 每个声明文件各有一条自己的只读绑定（落 /models/<基名>），**没有任何「整目录挂 /models」**；
//	② 同目录的**别的模型**既不出现在绑定里，也不出现在引擎参数/bwrap 命令行里（基名与全路径都查）；
//	③ mmproj 的「必须与权重同目录」旧限制退役（它是整目录挂载的副产物），仍守「基名撞车即拒孵」。
func TestHatchSpec_OnlyDeclaredWeightFiles(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	weightDir := t.TempDir()
	weight := filepath.Join(weightDir, "Qwen3.8-27B-Q4_K_M-vcruz305.gguf")
	mmproj := filepath.Join(weightDir, "mmproj-Qwen3.8-27B-f16.gguf")
	tmpl := filepath.Join(weightDir, "qwen38-chat-template.jinja")
	siblings := []string{
		filepath.Join(weightDir, "Qwen2.5-VL-32B-Instruct-Q4_K_M.gguf"),
		filepath.Join(weightDir, "Qwen3.6-35B-A3B-Q4_K_M.gguf"),
		filepath.Join(weightDir, "Flash-Next-Q4_K_M.gguf"),
	}
	for _, f := range append([]string{weight, mmproj, tmpl}, siblings...) {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          weight,
		MMProj:        mmproj,
		ChatTemplate:  tmpl,
		SchemaVersion: registry.EggSchemaVersionCurrent,
	}
	entry.SetEggNameForTest("Ornith-Egg12-Test")
	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9010, hatchTestProfile())
	if err != nil {
		t.Fatalf("映射应成功，实得 %v", err)
	}

	// ① 逐文件绑定：三个声明文件各有自己的一格，顺序 = 权重 → 投影 → 模板
	want := []string{
		weight + ":/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf",
		mmproj + ":/models/mmproj-Qwen3.8-27B-f16.gguf",
		tmpl + ":/models/qwen38-chat-template.jinja",
	}
	if len(spec.ExtraROBinds) != len(want) {
		t.Fatalf("只读绑定应恰好 %d 条（每声明文件一条），实得 %v", len(want), spec.ExtraROBinds)
	}
	for i, w := range want {
		if spec.ExtraROBinds[i] != w {
			t.Errorf("只读绑定[%d] 应为 %q，实得 %q", i, w, spec.ExtraROBinds[i])
		}
	}
	for _, b := range spec.ExtraROBinds {
		if strings.HasSuffix(b, ":"+spaceWeightsDir) {
			t.Errorf("不得有「整目录挂到 %s」的绑定（缺陷 12 的原形态）：%q", spaceWeightsDir, b)
		}
	}

	// ② 同目录别的模型：绑定、参数、bwrap 命令行里都不许出现（全路径与基名都查）
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("映射产物应能构造出 bwrap 命令行，实得 %v", err)
	}
	haystack := append([]string{}, spec.ExtraROBinds...)
	haystack = append(haystack, spec.ExtraRWBinds...)
	haystack = append(haystack, spec.EngineArgs...)
	haystack = append(haystack, argv...)
	for _, sib := range siblings {
		for _, h := range haystack {
			if strings.Contains(h, sib) {
				t.Errorf("同目录别的模型 %q 不得出现在产物里，却在 %q 里命中了", sib, h)
			}
			if strings.Contains(h, filepath.Base(sib)) {
				t.Errorf("同目录别的模型基名 %q 不得出现在产物里，却在 %q 里命中了", filepath.Base(sib), h)
			}
		}
	}
	// 声明文件本身仍在（没被「顺手一起删干净」）
	for _, declared := range []string{weight, mmproj, tmpl} {
		found := false
		for _, h := range haystack {
			if strings.Contains(h, declared) {
				found = true
			}
		}
		if !found {
			t.Errorf("声明文件 %q 应作为只读绑定源出现", declared)
		}
	}
	// 参数里只认空间内路径（宿主权重路径一个都不许残留，含权重目录本身）
	for _, a := range spec.EngineArgs {
		if strings.Contains(a, weightDir) {
			t.Errorf("引擎参数里不得残留宿主权重目录路径：%q", a)
		}
	}
	if got := argAfter(spec.EngineArgs, "-m"); got != "/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf" {
		t.Errorf("-m 应改写成 /models/<基名>，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	if got := argAfter(spec.EngineArgs, "-mm"); got != "/models/mmproj-Qwen3.8-27B-f16.gguf" {
		t.Errorf("-mm 应改写成 /models/<基名>，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	if got := argAfter(spec.EngineArgs, "--chat-template-file"); got != "/models/qwen38-chat-template.jinja" {
		t.Errorf("--chat-template-file 应改写成 /models/<基名>，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
	}

	t.Run("mmproj 与权重不同目录 ⇒ 逐文件挂载，各自成格（旧限制退役）", func(t *testing.T) {
		other := t.TempDir()
		mmOther := filepath.Join(other, "mmproj-elsewhere-f16.gguf")
		if err := os.WriteFile(mmOther, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          weight,
			MMProj:        mmOther,
			SchemaVersion: registry.EggSchemaVersionCurrent,
		}
		e.SetEggNameForTest("Egg12-mm-Test")
		sp, err := hatchSpecFor(e, EngineImplOf(e), 9011, hatchTestProfile())
		if err != nil {
			t.Fatalf("逐文件挂载之后，投影可以在别的目录（旧限制已退役），实得 %v", err)
		}
		wantB := mmOther + ":/models/mmproj-elsewhere-f16.gguf"
		ok := false
		for _, b := range sp.ExtraROBinds {
			if b == wantB {
				ok = true
			}
		}
		if !ok {
			t.Errorf("投影应自带一条只读绑定 %q，实得 %v", wantB, sp.ExtraROBinds)
		}
		if got := argAfter(sp.EngineArgs, "-mm"); got != "/models/mmproj-elsewhere-f16.gguf" {
			t.Errorf("-mm 应改写成空间内路径，实得 %q", got)
		}
	})
}

// TestHatchSpec_MmapMaxCountNotEffective 缺陷 11（2026-09-15 二选一里取「暂不生效」）：
// `env_req.mmap_max_count`（vm.max_map_count）是**机器级 sysctl**、不是按进程 rlimit，孵化单元里
// 没有可下发的落点 ⇒ 本批**有意不映射**。本用例钉住的正是「不生效」这句话本身：声明与不声明，
// **映射产物逐字段相同**（即它绝不是一个看着像保障、其实没人执行的东西）。
// 将来若给它接上下发通路（或机器级前置校验），本用例会先红 —— 那时要连注释一起改口径。
func TestHatchSpec_MmapMaxCountNotEffective(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	// 同一份档案喂两次（档案里带 MeasuredAt=time.Now()，各造一份会让 DeepEqual 死在时钟上，
	// 那是用例自己的噪声，不是被测语义）
	prof := hatchTestProfile()
	build := func(mmapMaxCount int) hatch.Spec {
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          "/data/models/glm/glm.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent,
			EnvReq: &registry.EnvReq{
				MemlockKB:    8192,
				MmapMaxCount: mmapMaxCount,
			},
		}
		e.SetEggNameForTest("GLM-Mmap-Test")
		sp, err := hatchSpecFor(e, EngineImplOf(e), 9000, prof)
		if err != nil {
			t.Fatalf("映射应成功，实得 %v", err)
		}
		return sp
	}

	declared := build(1048576) // 真机实测值（sysctl -n vm.max_map_count）
	absent := build(0)
	if !reflect.DeepEqual(declared, absent) {
		t.Errorf("mmap_max_count 暂不生效 ⇒ 声明与不声明必须产出同一份孵化声明；差异：\n声明=%+v\n不声明=%+v",
			declared, absent)
	}
	// 逐项直说（与 DeepEqual 双保险：将来有人给它接上半条通路，也能一眼看出是哪一项动了）
	for _, a := range append(append([]string{}, declared.EngineArgs...), declared.ExtraROBinds...) {
		if strings.Contains(a, "max_map_count") || strings.Contains(a, "1048576") {
			t.Errorf("mmap_max_count 当前无下发通路，产物里不得出现它的痕迹：%q", a)
		}
	}
	if got := declared.MemlockBytes; got != 8192*1024 {
		t.Errorf("memlock_kb 仍必须照常下发（它与 mmap_max_count 是两回事），实得 %d", got)
	}
}

// TestHatchSpec_LibPathsFailClosed lib_paths 的五条拒孵（2026-09-15 拍）：
// 同名（两个库目录同名 / 同一目录声明两次）· 宿主上不存在 · 不是目录 · 相对路径 · 空项。
func TestHatchSpec_LibPathsFailClosed(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	root := t.TempDir()
	libA := filepath.Join(root, "a", "shared-libs")
	libB := filepath.Join(root, "b", "shared-libs") // 目录名与 libA 相同 ⇒ 同名
	for _, d := range []string{libA, libB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	notDir := filepath.Join(root, "not-a-dir.so")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		libs []string
		want string
	}{
		{"两个库目录同名", []string{libA, libB}, "同名"},
		{"同一个宿主目录声明两次", []string{libA, libA}, "同名"},
		{"宿主上不存在", []string{filepath.Join(root, "no-such-lib-dir")}, "不存在"},
		{"不是目录", []string{notDir}, "不是目录"},
		{"相对路径", []string{"libs/k2"}, "绝对路径"},
		{"空项", []string{libA, "  "}, "空串"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := &registry.ModelEntry{
				Backend:       "llama-server",
				File:          "/data/models/k2/k2horizon.gguf",
				SchemaVersion: registry.EggSchemaVersionCurrent,
				EnvReq:        &registry.EnvReq{LibPaths: c.libs},
			}
			e.SetEggNameForTest("K2-Lib-Test")
			_, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
			if err == nil {
				t.Fatalf("应拒孵（%s），却映射成功了", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应提到 %q，实得 %q", c.want, err.Error())
			}
		})
	}
}

// TestHatchSpec_NonMainlineEgg 非主线卵（cmd: 载体）：
// 引擎取 cmd 首词（包装脚本），空间内路径 = /engine/<可执行名>，参数照样改写成空间内权重路径。
func TestHatchSpec_NonMainlineEgg(t *testing.T) {
	withWorkDirRoot(t)

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/k2/k2horizon-q4_k_m.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		Cmd: registry.CmdString("/home/g01/agent/run-k2.sh -m {file} -c 32768 " +
			"--host 127.0.0.1 --port {port}"),
	}
	entry.SetEggNameForTest("K2-Horizon-Test")

	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9222, hatchTestProfile())
	if err != nil {
		t.Fatalf("非主线卵映射应成功，实得 %v", err)
	}
	if spec.EnginePathInSpace != "/engine/run-k2.sh" {
		t.Errorf("空间内引擎路径应取 cmd 首词的可执行名，实得 %q", spec.EnginePathInSpace)
	}
	if len(spec.EngineRoots) != 1 || spec.EngineRoots[0] != "/home/g01/agent" {
		t.Errorf("EngineRoots 应是包装脚本所在宿主目录，实得 %v", spec.EngineRoots)
	}
	if got := argAfter(spec.EngineArgs, "-m"); got != "/models/k2horizon-q4_k_m.gguf" {
		t.Errorf("-m 应改写成空间内路径，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	if got := argAfter(spec.EngineArgs, "--port"); got != "9222" {
		t.Errorf("--port 应带上端口，实得 %q", got)
	}
	if spec.Env != nil {
		t.Errorf("未声明 env_req 的卵不得被注入环境变量空壳，实得 %v", spec.Env)
	}
	if spec.Devices != nil {
		t.Errorf("未声明卡号/节点的卵应交缺省设备（nil），实得 %v", spec.Devices)
	}
	if spec.MemlockBytes != 0 {
		t.Errorf("未声明 memlock_kb 时不得下发限额（不猜），实得 %d", spec.MemlockBytes)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
	}
}

// TestHatchSpec_FailClosed 缺任一必需信息都必须报错——绝不编造默认值。
func TestHatchSpec_FailClosed(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	base := func() *registry.ModelEntry {
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          "/data/models/k2/k2horizon-q4_k_m.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent,
		}
		e.SetEggNameForTest("K2-Horizon-Test")
		return e
	}

	cases := []struct {
		name string
		mut  func(*registry.ModelEntry)
		want string
	}{
		{"卵名拿不到", func(e *registry.ModelEntry) { e.SetEggNameForTest("") }, "卵名拿不到"},
		{"权重未声明", func(e *registry.ModelEntry) { e.File = "" }, "权重文件未声明"},
		{"权重是相对路径", func(e *registry.ModelEntry) { e.File = "models/x.gguf" }, "绝对路径"},
		{"引擎路径拿不到", func(e *registry.ModelEntry) {
			e.Cmd = registry.CmdString("zerg-no-such-engine-binary-xyz --port {port}")
		}, "找不到"},
		{"引擎实现名与执行分叉", func(e *registry.ModelEntry) {
			e.Cmd = registry.CmdString("/home/g01/agent/run-k2.sh -m {file}")
		}, "不一致"},
		{"文件与投影落点撞车（基名相同）", func(e *registry.ModelEntry) {
			// 逐文件挂载后「投影必须与权重同目录」的限制退役了，但**同一空间落点**只能有一份来源
			e.MMProj = "/data/mmproj/other/Qwen3.8-27B-Q4_K_M.gguf" // 与权重基名相同
			e.File = "/data/models/k2/Qwen3.8-27B-Q4_K_M.gguf"
		}, "落点"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := base()
			c.mut(e)
			// 引擎实现名按条目的实际执行口径取（分叉用例正是要让二者不一致）
			engineImpl := EngineImplOf(e)
			if c.name == "引擎实现名与执行分叉" {
				engineImpl = "run-k2-other.sh"
			}
			if _, err := hatchSpecFor(e, engineImpl, 9000, hatchTestProfile()); err == nil {
				t.Fatalf("应拒孵（%s），却映射成功了", c.want)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("错误信息应提到 %q，实得 %q", c.want, err.Error())
			}
		})
	}

	// nil 条目：注册表里没有这枚卵 —— 不替它编一份声明
	if _, err := hatchSpecFor(nil, "llama-server", 9000, hatchTestProfile()); err == nil {
		t.Fatal("卵声明为空时必须报错")
	}
}

// TestHatchSpec_ChatTemplateInputChannel 模板类只读输入（ExtraROBinds 输入通道）：
// 落在权重目录之外 ⇒ 只读挂进 /templates/<基名> 并把参数改写成空间内路径；
// 落在权重目录之内 ⇒ 与权重一样是**逐文件**绑定到 /models/<基名>
// （缺陷 12 之后「同目录就顺带可见」不再成立：整目录不挂了，同目录的东西也得自己挂进来）。
func TestHatchSpec_ChatTemplateInputChannel(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	// 外部模板（宿主 ~/.zerg/… 形态）
	extDir := t.TempDir()
	extTemplate := filepath.Join(extDir, "ornith_chat_template.jinja")
	if err := os.WriteFile(extTemplate, []byte("{{ }}"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/ornith/ornith-Q4.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		ChatTemplate:  extTemplate,
	}
	entry.SetEggNameForTest("Ornith-Test")
	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9000, hatchTestProfile())
	if err != nil {
		t.Fatalf("外部模板应被只读挂进来，实得 %v", err)
	}
	wantBinds := []string{
		"/data/models/ornith/ornith-Q4.gguf:/models/ornith-Q4.gguf",
		extTemplate + ":/templates/ornith_chat_template.jinja",
	}
	if len(spec.ExtraROBinds) != 2 ||
		spec.ExtraROBinds[0] != wantBinds[0] || spec.ExtraROBinds[1] != wantBinds[1] {
		t.Errorf("只读绑定应为 [权重文件, 外部模板]（%v），实得 %v", wantBinds, spec.ExtraROBinds)
	}
	if got := argAfter(spec.EngineArgs, "--chat-template-file"); got != "/templates/ornith_chat_template.jinja" {
		t.Errorf("模板参数应改写成空间内路径，实得 %q（参数=%v）", got, spec.EngineArgs)
	}

	// 与权重同目录的模板：同为逐文件绑定（/models/<基名>），不再依赖「目录顺带可见」
	weightDir := t.TempDir()
	sameDir := filepath.Join(weightDir, "same.jinja")
	if err := os.WriteFile(sameDir, []byte("{{ }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry2 := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          filepath.Join(weightDir, "ornith-Q4.gguf"),
		SchemaVersion: registry.EggSchemaVersionCurrent,
		ChatTemplate:  sameDir,
	}
	entry2.SetEggNameForTest("Ornith-Same")
	spec2, err := hatchSpecFor(entry2, EngineImplOf(entry2), 9000, hatchTestProfile())
	if err != nil {
		t.Fatalf("同目录模板映射应成功，实得 %v", err)
	}
	want2 := sameDir + ":/models/same.jinja"
	if len(spec2.ExtraROBinds) != 2 || spec2.ExtraROBinds[1] != want2 {
		t.Errorf("同目录模板应逐文件绑到 %q（第二条），实得 %v", want2, spec2.ExtraROBinds)
	}
	if got := argAfter(spec2.EngineArgs, "--chat-template-file"); got != "/models/same.jinja" {
		t.Errorf("同目录模板应改写成 /models/<basename>，实得 %q", got)
	}
}

// TestHatchSpec_WorkDirPerEggAndNoEscape 一次性工作目录：WorkDir 恒为**空间内** /work，宿主目录
// 每卵一份、经可写绑定落进空间；且卵名不许逃出工作根。
func TestHatchSpec_WorkDirPerEggAndNoEscape(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	root := withWorkDirRoot(t)

	// 返回 (空间内工作目录, 宿主工作目录)；宿主那份从可写绑定里取——它才是每卵目录的真身
	mk := func(name string) (string, string) {
		e := &registry.ModelEntry{Backend: "llama-server", File: "/data/models/a.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent}
		e.SetEggNameForTest(name)
		spec, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
		if err != nil {
			t.Fatalf("卵 %q 映射失败: %v", name, err)
		}
		if len(spec.ExtraRWBinds) == 0 {
			t.Fatalf("卵 %q 缺宿主工作目录的可写绑定：%+v", name, spec.ExtraRWBinds)
		}
		b := spec.ExtraRWBinds[0]
		if !strings.HasSuffix(b, ":/work") {
			t.Fatalf("卵 %q 的工作目录可写绑定应以 :/work 结尾，实得 %q", name, b)
		}
		return spec.WorkDir, strings.TrimSuffix(b, ":/work")
	}

	spaceA, hostA := mk("GLM-5.3-Flash")
	spaceB, hostB := mk("Qwen3.8-Flash-Next")
	if spaceA != "/work" || spaceB != "/work" {
		t.Fatalf("WorkDir 应恒为空间内路径 /work，实得 %q / %q", spaceA, spaceB)
	}
	if hostA == hostB {
		t.Fatal("不同卵必须各有一份一次性工作目录")
	}
	// 穿越尝试：宿主工作目录必须仍在工作根下、且是单层目录
	for _, name := range []string{"../../evil", "/etc/passwd", "..", "."} {
		_, host := mk(name)
		if filepath.Dir(host) != filepath.Clean(root) {
			t.Fatalf("卵名 %q 的工作目录逃出了工作根：%q（根=%q）", name, host, root)
		}
	}
}

// TestHatchSpec_KVDiskRWBind KV 盘（§6.6「跨孵化保留」侧 / §9.7④）：宿主按卵分目录
// `<home>/.zerg/kvdisk/<卵名>/` 可写绑到空间内 /kvdisk，引擎参数里的宿主 KV 路径被**精确改写**
// 成 /kvdisk（写盘上限原样保留）；映射产物喂进 hatch.BuildBwrapArgv 必须真的落成 `--bind`
// 且 `--chdir` 用空间内路径（本缺陷正是「映射与命令行配方语义不一致」，故这里两端连起来验）。
func TestHatchSpec_KVDiskRWBind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // KV 目录缺省按 §9.7④ 从 HOME 推——单测不许往真家目录写东西
	withMainlineEngineProbe(t, "/opt/ds4/bin/ds4-server")
	workRoot := withWorkDirRoot(t)

	entry := &registry.ModelEntry{
		Backend:       "ds4-server",
		File:          "/data/models/ds4/deepseek-v4-flash.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		KVDisk:        &registry.KVDiskDecl{SpaceMB: 3500},
	}
	entry.SetEggNameForTest("DeepSeek-V4-Flash")

	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9411, hatchTestProfile())
	if err != nil {
		t.Fatalf("声明了 KV 盘的卵映射应成功，实得 %v", err)
	}
	kvHost := filepath.Join(home, ".zerg", "kvdisk", "DeepSeek-V4-Flash")

	// ① 参数：宿主 KV 路径 → /kvdisk（只改这一个 token），写盘上限原样保留
	if got := argAfter(spec.EngineArgs, "--kv-disk-dir"); got != "/kvdisk" {
		t.Errorf("--kv-disk-dir 应改写成空间内 /kvdisk，实得 %q（参数=%v）", got, spec.EngineArgs)
	}
	for _, a := range spec.EngineArgs {
		if strings.Contains(a, kvHost) {
			t.Errorf("参数里不得残留宿主 KV 盘路径：%q", a)
		}
	}
	if got := argAfter(spec.EngineArgs, "--kv-disk-space-mb"); got != "3500" {
		t.Errorf("写盘上限 --kv-disk-space-mb 应原样保留，实得 %q", got)
	}
	// ② 绑定：宿主 KV 目录可写绑到 /kvdisk；且宿主目录真的建出来了（绑定源不存在 ⇒ bwrap 孵不起来）
	wantKV := kvHost + ":/kvdisk"
	found := false
	for _, b := range spec.ExtraRWBinds {
		if b == wantKV {
			found = true
		}
	}
	if !found {
		t.Errorf("宿主 KV 目录应可写绑到 /kvdisk（%q），实得 %v", wantKV, spec.ExtraRWBinds)
	}
	if st, err := os.Stat(kvHost); err != nil || !st.IsDir() {
		t.Errorf("宿主 KV 目录应已被创建（%q），stat 结果 err=%v", kvHost, err)
	}
	// ③ 顺序：一次性（工作目录）在前、跨孵化保留（KV 盘）在后，逐字可复现
	if want := filepath.Join(workRoot, "DeepSeek-V4-Flash") + ":/work"; len(spec.ExtraRWBinds) != 2 || spec.ExtraRWBinds[0] != want {
		t.Errorf("可写绑定应为 [工作目录, KV 盘]，实得 %v", spec.ExtraRWBinds)
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("映射产物应能过孵化声明校验，实得 %v", err)
	}
	// ④ 命令行这一侧真跑一遍：必须是 --bind（可写）、--chdir 是空间内路径
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("映射产物应能构造出 bwrap 命令行，实得 %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--bind " + kvHost + " /kvdisk", "--bind " + filepath.Join(workRoot, "DeepSeek-V4-Flash") + " /work", "--chdir /work"} {
		if !strings.Contains(joined, want) {
			t.Errorf("bwrap 命令行缺 %q，实得 %q", want, joined)
		}
	}
	if strings.Contains(joined, "--ro-bind "+kvHost) {
		t.Errorf("KV 盘是可写落点，不得落成只读绑定：%q", joined)
	}
}

// TestHatchSpec_KVDiskOnlyWhenEngineAsks 判据是**执行面**（引擎真会收到的参数）：
//   - 声明了 kv_disk 但适配器按归属铁律没发该参数（llama 系，§9.2）⇒ 不加绑定、不改写、不报错；
//   - cmd: 里手写了宿主绝对路径的 --kv-disk-dir ⇒ 照样绑定与改写（引擎真会往那儿写）；
//   - 手写的值是相对路径 ⇒ 拒孵（空间里会落进一次性工作目录，与 §9.7④「跨孵化保留」冲突）。
func TestHatchSpec_KVDiskOnlyWhenEngineAsks(t *testing.T) {
	kvHost := filepath.Join(t.TempDir(), "kvdisk", "K2-Horizon")
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	t.Run("llama 系没发该参数 ⇒ 不绑不改", func(t *testing.T) {
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          "/data/models/glm/glm.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent,
			KVDisk:        &registry.KVDiskDecl{SpaceMB: 3500}, // 声明了，但适配器按归属铁律不发
		}
		e.SetEggNameForTest("GLM-5.3-Flash")
		spec, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
		if err != nil {
			t.Fatalf("映射应成功（声明未落到执行面 ⇒ 没有可绑的落点，但不是错误），实得 %v", err)
		}
		if got := argAfter(spec.EngineArgs, "--kv-disk-dir"); got != "" {
			t.Errorf("llama 系不得出现 --kv-disk-dir，实得 %q", got)
		}
		for _, b := range spec.ExtraRWBinds {
			if strings.Contains(b, "/kvdisk") {
				t.Errorf("执行面没有 KV 盘 ⇒ 不得编造 /kvdisk 绑定，实得 %v", spec.ExtraRWBinds)
			}
		}
	})

	t.Run("cmd 手写宿主 KV 路径 ⇒ 照样绑定与改写", func(t *testing.T) {
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          "/data/models/k2/k2horizon.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent,
			Cmd:           registry.CmdString("/home/g01/agent/run-k2.sh -m {file} --kv-disk-dir " + kvHost + " --port {port}"),
		}
		e.SetEggNameForTest("K2-Horizon")
		spec, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
		if err != nil {
			t.Fatalf("映射应成功，实得 %v", err)
		}
		if got := argAfter(spec.EngineArgs, "--kv-disk-dir"); got != "/kvdisk" {
			t.Errorf("宿主 KV 路径应改写成 /kvdisk，实得 %q（参数=%v）", got, spec.EngineArgs)
		}
		if want := kvHost + ":/kvdisk"; len(spec.ExtraRWBinds) != 2 || spec.ExtraRWBinds[1] != want {
			t.Errorf("应加上可写绑定 %q，实得 %v", want, spec.ExtraRWBinds)
		}
	})

	t.Run("相对路径 ⇒ 拒孵", func(t *testing.T) {
		e := &registry.ModelEntry{
			Backend:       "llama-server",
			File:          "/data/models/k2/k2horizon.gguf",
			SchemaVersion: registry.EggSchemaVersionCurrent,
			Cmd:           registry.CmdString("/home/g01/agent/run-k2.sh -m {file} --kv-disk-dir kvlocal --port {port}"),
		}
		e.SetEggNameForTest("K2-Horizon")
		_, err := hatchSpecFor(e, EngineImplOf(e), 9000, hatchTestProfile())
		if err == nil {
			t.Fatal("相对 KV 盘目录必须拒孵（空间内会落进一次性工作目录，KV 活不过收卵）")
		}
		if !strings.Contains(err.Error(), "绝对路径") {
			t.Errorf("错误信息应说明必须是宿主绝对路径，实得 %q", err.Error())
		}
	})
}

// TestHatchSpec_NoProfileIsRejectedByValidate 映射只搬档案；档案缺失要在孵化声明校验那一步被拒
// （§8.4：无实测档案不许孵——本包不替它编一份档案）。
func TestHatchSpec_NoProfileIsRejectedByValidate(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	e := &registry.ModelEntry{Backend: "llama-server", File: "/data/models/a.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent}
	e.SetEggNameForTest("NoProfile")
	spec, err := hatchSpecFor(e, EngineImplOf(e), 9000, monitor.EggProfile{})
	if err != nil {
		t.Fatalf("映射本身不该依赖档案（搬进去即可），实得 %v", err)
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("零值档案必须过不了孵化声明校验（无实测档案不许孵）")
	}
}

// argAfter 取 flag 后面的那个参数（找不到返回 ""）。
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
