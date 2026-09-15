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

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/glm-5.3/GLM-5.3-Flash-Q4_K_M.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		EnvReq: &registry.EnvReq{
			Devices:   []string{"1"}, // 用第 1 张卡
			LibPaths:  []string{"/home/g01/llama.cpp-src/build-hip-flash/bin"},
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
	// 权重：只挂该卵自己的权重**目录**
	if spec.WeightPath != "/data/models/glm-5.3" {
		t.Errorf("WeightPath 应是权重文件所在目录，实得 %q", spec.WeightPath)
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
	entry.SetEggNameForTest("example-moe-36b-Test")

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
		e.SetEggNameForTest("example-moe-36b-Test")
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
		{"投影与权重不同目录", func(e *registry.ModelEntry) {
			e.MMProj = "/data/mmproj/other/mmproj-k2.gguf"
		}, "静默降级"},
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
// 落在权重目录之外 ⇒ 只读挂进 /templates 并把参数改写成空间内路径；落在权重目录内 ⇒ 顺带可见、不额外挂。
func TestHatchSpec_ChatTemplateInputChannel(t *testing.T) {
	withMainlineEngineProbe(t, "/opt/llama/bin/llama-server")
	withWorkDirRoot(t)

	// 外部模板（宿主 ~/.zerg/… 形态）
	extDir := t.TempDir()
	extTemplate := filepath.Join(extDir, "example-35b-v2_chat_template.jinja")
	if err := os.WriteFile(extTemplate, []byte("{{ }}"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          "/data/models/example-35b-v2/example-35b-v2-Q4.gguf",
		SchemaVersion: registry.EggSchemaVersionCurrent,
		ChatTemplate:  extTemplate,
	}
	entry.SetEggNameForTest("example-35b-v2-Test")
	spec, err := hatchSpecFor(entry, EngineImplOf(entry), 9000, hatchTestProfile())
	if err != nil {
		t.Fatalf("外部模板应被只读挂进来，实得 %v", err)
	}
	wantBind := extTemplate + ":/templates/example-35b-v2_chat_template.jinja"
	if len(spec.ExtraROBinds) != 1 || spec.ExtraROBinds[0] != wantBind {
		t.Errorf("外部模板应只读挂进 /templates，实得 %v", spec.ExtraROBinds)
	}
	if got := argAfter(spec.EngineArgs, "--chat-template-file"); got != "/templates/example-35b-v2_chat_template.jinja" {
		t.Errorf("模板参数应改写成空间内路径，实得 %q（参数=%v）", got, spec.EngineArgs)
	}

	// 与权重同目录的模板：不必额外挂（/models 已经把它带进去了）
	weightDir := t.TempDir()
	sameDir := filepath.Join(weightDir, "same.jinja")
	if err := os.WriteFile(sameDir, []byte("{{ }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry2 := &registry.ModelEntry{
		Backend:       "llama-server",
		File:          filepath.Join(weightDir, "example-35b-v2-Q4.gguf"),
		SchemaVersion: registry.EggSchemaVersionCurrent,
		ChatTemplate:  sameDir,
	}
	entry2.SetEggNameForTest("example-35b-v2-Same")
	spec2, err := hatchSpecFor(entry2, EngineImplOf(entry2), 9000, hatchTestProfile())
	if err != nil {
		t.Fatalf("同目录模板映射应成功，实得 %v", err)
	}
	if len(spec2.ExtraROBinds) != 0 {
		t.Errorf("模板与权重同目录时不应额外绑定，实得 %v", spec2.ExtraROBinds)
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
	kvHost := filepath.Join(t.TempDir(), "kvdisk", "example-moe-36b")
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
		e.SetEggNameForTest("example-moe-36b")
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
		e.SetEggNameForTest("example-moe-36b")
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
