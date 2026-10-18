// egg_weights_hatch_test.go —— GAP-20261018-16：**从蛋孵化时权重也走蛋内**。
//
// 卡点（改前真机逐字）：`spaceInputMappings` 对**宿主**路径 os.Stat 后出 `--ro-bind`，而
// `eggWiringFor` 只把 engine / libs / sysroot-bin / rocblas 换成蛋内源 ⇒ 把宿主权重目录改名后
// 孵化直接 `502 hatch spec unavailable`（spec 阶段即拒）。本文件把新分支的两态钉成机器判据：
//
//	① 蛋内有权重 ⇒ **挂载源 = `<egg>/models/<基名>`**（片组同形），WeightFiles / WeightPath 同步；
//	② 蛋内没有   ⇒ **具名拒孵**（✗ 不静默回落宿主）；
//	③ 非蛋模式   ⇒ 逐字不动（零回归：宿主路径原样进绑定与清单）。
package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// weightBindHost 从 spec.ExtraROBinds 里取「落到 space」那条的**挂载源**。
func weightBindHost(t *testing.T, binds []string, space string) string {
	t.Helper()
	for _, b := range binds {
		host, sp, err := edgeSplitBind(b)
		if err != nil {
			t.Fatalf("绑定格式坏：%q（%v）", b, err)
		}
		if sp == space {
			return host
		}
	}
	return ""
}

// edgeSplitBind 只读地拆 `<host>:<space>`（本文件自己的探针，别动 hatch 的私货）。
func edgeSplitBind(b string) (host, space string, err error) {
	i := strings.LastIndex(b, ":")
	if i <= 0 || i == len(b)-1 {
		return "", "", os.ErrInvalid
	}
	return b[:i], b[i+1:], nil
}

// TestEggHatch_WeightsComeFromEgg ① 蛋内有 ⇒ 挂载源、清单、出证面全部指向蛋内副本；
// 且宿主那份**根本不存在**（夹具里从没造过它）—— 这正是「原目录可删」的形状。
func TestEggHatch_WeightsComeFromEgg(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	e := eggEntry(root)

	hostWeight := expandTilde(e.File)
	if _, err := os.Stat(hostWeight); err == nil {
		t.Fatalf("夹具前提失效：宿主权重 %s 竟然存在（本用例要证「宿主没有也能孵」）", hostWeight)
	}
	eggWeight := filepath.Join(root, "models", filepath.Base(hostWeight))
	if _, err := os.Stat(eggWeight); err != nil {
		t.Fatalf("夹具前提失效：蛋内权重 %s 不在：%v", eggWeight, err)
	}

	spec, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err != nil {
		t.Fatalf("hatchSpecFor（蛋内有权重 ⇒ 应能孵）：%v", err)
	}

	// ① 绑定源 = 蛋内副本（逐字）
	space := "/models/" + filepath.Base(hostWeight)
	if got := weightBindHost(t, spec.ExtraROBinds, space); got != eggWeight {
		t.Errorf("落点 %s 的挂载源应是蛋内副本 %q，实得 %q\n全部绑定：%v", space, eggWeight, got, spec.ExtraROBinds)
	}
	for _, b := range spec.ExtraROBinds {
		if strings.HasPrefix(b, hostWeight+":") || strings.HasPrefix(b, filepath.Dir(hostWeight)+string(filepath.Separator)) {
			t.Errorf("宿主权重路径仍出现在绑定里（回落了）：%q", b)
		}
	}
	// ② 声明清单与出证面同步到蛋内（hatch 侧 validateWeightFiles 当契约核「绑的 == 声明的」）
	if len(spec.WeightFiles) != 1 || spec.WeightFiles[0] != eggWeight {
		t.Errorf("WeightFiles 应是蛋内副本 [%q]，实得 %v", eggWeight, spec.WeightFiles)
	}
	if want := filepath.Join(root, "models"); spec.WeightPath != want {
		t.Errorf("WeightPath（出证/日志面）应是蛋内权重目录 %q，实得 %q", want, spec.WeightPath)
	}
	// ③ 包封契约：映射产物要过 hatch.Spec.Validate（否则真孵化在 Validate 就拒）
	if err := spec.Validate(); err != nil {
		t.Fatalf("蛋内权重映射产物过不了 Validate：%v", err)
	}
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	if !argvHas(argv, "--ro-bind", eggWeight, space) {
		t.Errorf("argv 里缺 `--ro-bind %s %s`：%v", eggWeight, space, argv)
	}
	if argvHas(argv, "--ro-bind", hostWeight, space) {
		t.Errorf("argv 里仍在挂宿主权重：%v", argv)
	}
}

// TestEggHatch_WeightMissingInEggRefuses ② 蛋内没有该基名 ⇒ **具名拒**（✗ 不回落宿主）。
func TestEggHatch_WeightMissingInEggRefuses(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	e := eggEntry(root)
	hostWeight := expandTilde(e.File)
	eggWeight := filepath.Join(root, "models", filepath.Base(hostWeight))
	if err := os.Remove(eggWeight); err != nil {
		t.Fatalf("夹具准备（移除蛋内副本）：%v", err)
	}
	// 反证面：把**宿主**那件造出来 —— 若实现回落宿主，这条就会孵成功（本用例就是要它不成功）。
	if err := os.WriteFile(hostWeight, []byte("host-copy"), 0o644); err != nil {
		// 宿主目录不可写（/data/models 在开发机上没有）⇒ 换一个可写宿主路径重建声明
		hostDir := t.TempDir()
		e.File = filepath.Join(hostDir, filepath.Base(hostWeight))
		if werr := os.WriteFile(expandTilde(e.File), []byte("host-copy"), 0o644); werr != nil {
			t.Fatalf("造宿主副本：%v", werr)
		}
		hostWeight = expandTilde(e.File)
		eggWeight = filepath.Join(root, "models", filepath.Base(hostWeight))
	}

	_, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err == nil {
		t.Fatalf("蛋内缺 %s 时必须拒孵（宿主上那份在也不许回落）", eggWeight)
	}
	msg := err.Error()
	if !strings.Contains(msg, "蛋内副本") || !strings.Contains(msg, eggWeight) {
		t.Errorf("拒孵理由必须具名点到蛋内那件 %q 且说明是蛋内副本，实得：%v", eggWeight, msg)
	}
	if strings.Contains(msg, "没有 models/ 这一层") {
		t.Errorf("这一态是「层在、件缺」，不该报成整层缺失：%v", msg)
	}
}

// TestEggHatch_ModelsLayerMissingRefuses 整层缺失 ⇒ 具名拒（信息面比逐件更早、更直）。
func TestEggHatch_ModelsLayerMissingRefuses(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	e := eggEntry(root)
	if err := os.RemoveAll(filepath.Join(root, "models")); err != nil {
		t.Fatalf("夹具准备：%v", err)
	}
	_, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err == nil {
		t.Fatal("蛋里没有 models/ 时必须拒孵")
	}
	if msg := err.Error(); !strings.Contains(msg, "没有 models/ 这一层") || !strings.Contains(msg, "不退回宿主挂载") {
		t.Errorf("拒孵理由应点名「没有 models/ 这一层」且明说不退回宿主，实得：%v", msg)
	}
}

// TestEggHatch_ShardGroupFromEgg 片组同形：四片按原基名平铺在蛋内 models/ 下 ⇒
// 四片逐片从蛋内挂，`-m` 仍指第 1 片；第 1 片也走蛋内（不是宿主）。
func TestEggHatch_ShardGroupFromEgg(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	// 蛋里造一组四片（基名用真机的命名式）
	var names []string
	for n := 1; n <= 4; n++ {
		name := shardNameFor(n, 4)
		names = append(names, name)
		if err := os.WriteFile(filepath.Join(root, "models", name), []byte("GGUF-shard"), 0o644); err != nil {
			t.Fatalf("造蛋内分片：%v", err)
		}
	}
	e := eggEntry(root)
	// 宿主上放一组「影子」分片：实现若回落宿主，绑定就会指向它们 —— 判据靠**源路径**分辨。
	hostDir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(hostDir, name), []byte("GGUF-host"), 0o644); err != nil {
			t.Fatalf("造宿主影子分片：%v", err)
		}
	}
	e.File = filepath.Join(hostDir, names[0])

	spec, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err != nil {
		t.Fatalf("hatchSpecFor（蛋内片组齐 ⇒ 应能孵）：%v", err)
	}
	if len(spec.WeightFiles) != 4 {
		t.Fatalf("四片应各占一格 WeightFiles，实得 %v", spec.WeightFiles)
	}
	for _, name := range names {
		eggShard := filepath.Join(root, "models", name)
		if got := weightBindHost(t, spec.ExtraROBinds, "/models/"+name); got != eggShard {
			t.Errorf("落点 /models/%s 的挂载源应是蛋内 %q，实得 %q", name, eggShard, got)
		}
		found := false
		for _, wf := range spec.WeightFiles {
			if wf == eggShard {
				found = true
			}
		}
		if !found {
			t.Errorf("WeightFiles 应含蛋内分片 %q，实得 %v", eggShard, spec.WeightFiles)
		}
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("片组映射产物过不了 Validate：%v", err)
	}
	// -m 仍指第 1 片（空间内）
	first := false
	for i, a := range spec.EngineArgs {
		if a == "-m" && i+1 < len(spec.EngineArgs) && spec.EngineArgs[i+1] == "/models/"+names[0] {
			first = true
		}
	}
	if !first {
		t.Errorf("-m 应指空间内第 1 片，实得 %v", spec.EngineArgs)
	}
}

// TestEggHatch_ShardMissingInEggRefuses 片组在蛋内缺一片 ⇒ 具名点到**蛋内**缺的那片，且点名「不回落宿主」。
func TestEggHatch_ShardMissingInEggRefuses(t *testing.T) {
	withWorkDirRoot(t)
	root := eggFixture(t)
	var names []string
	for n := 1; n <= 4; n++ {
		name := shardNameFor(n, 4)
		names = append(names, name)
		if err := os.WriteFile(filepath.Join(root, "models", name), []byte("GGUF-shard"), 0o644); err != nil {
			t.Fatalf("造蛋内分片：%v", err)
		}
	}
	if err := os.Remove(filepath.Join(root, "models", names[2])); err != nil {
		t.Fatalf("移除蛋内第 3 片：%v", err)
	}
	e := eggEntry(root)
	hostDir := t.TempDir()
	for _, name := range names { // 宿主上四片全在（反证：不许回落到它们）
		if err := os.WriteFile(filepath.Join(hostDir, name), []byte("GGUF-host"), 0o644); err != nil {
			t.Fatalf("造宿主影子分片：%v", err)
		}
	}
	e.File = filepath.Join(hostDir, names[0])

	_, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err == nil {
		t.Fatal("蛋内缺一片时必须拒孵（宿主齐全也不许回落）")
	}
	msg := err.Error()
	if !strings.Contains(msg, "**蛋内**") || !strings.Contains(msg, filepath.Join(root, "models", names[2])) {
		t.Errorf("拒孵理由应点名蛋内缺的那片 %q，实得：%v", filepath.Join(root, "models", names[2]), msg)
	}
	if strings.Contains(msg, hostDir) {
		t.Errorf("拒孵理由不该把宿主影子分片当缺片依据：%v", msg)
	}
}

// TestHatchSpecNonEgg_WeightsStayHost ③ 非蛋模式**逐字不动**：绑定源与清单都是声明里的宿主路径。
func TestHatchSpecNonEgg_WeightsStayHost(t *testing.T) {
	withWorkDirRoot(t)
	withMainlineEngineProbe(t, "/home/<user>/llama.cpp-src/build-hip-flash/bin/llama-server")
	libDir := filepath.Join(t.TempDir(), "build-hip-flash")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wDir := t.TempDir()
	var names []string
	for n := 1; n <= 4; n++ {
		name := shardNameFor(n, 4)
		names = append(names, name)
		if err := os.WriteFile(filepath.Join(wDir, name), []byte("GGUF-host"), 0o644); err != nil {
			t.Fatalf("造宿主分片：%v", err)
		}
	}
	e := &registry.ModelEntry{
		File:          filepath.Join(wDir, names[0]),
		SchemaVersion: registry.EggSchemaVersionCurrent,
		EnvReq:        &registry.EnvReq{Devices: []string{"0"}, LibPaths: []string{libDir}},
	}
	e.SetEggNameForTest("no-egg")

	spec, err := hatchSpecFor(e, "llama-server", 58100, hatchTestProfile())
	if err != nil {
		t.Fatalf("hostspec: %v", err)
	}
	if spec.EggRoot != "" || spec.EggBinDir != "" {
		t.Errorf("非蛋模式不该有任何蛋字段：%q %q", spec.EggRoot, spec.EggBinDir)
	}
	if want := filepath.Dir(filepath.Join(wDir, names[0])); spec.WeightPath != want {
		t.Errorf("WeightPath 应是宿主权重目录 %q（逐字不变），实得 %q", want, spec.WeightPath)
	}
	for _, name := range names {
		host := filepath.Join(wDir, name)
		if got := weightBindHost(t, spec.ExtraROBinds, "/models/"+name); got != host {
			t.Errorf("非蛋模式落点 /models/%s 的挂载源应是宿主 %q（逐字不变），实得 %q", name, host, got)
		}
	}
	for _, wf := range spec.WeightFiles {
		if !strings.HasPrefix(wf, wDir) {
			t.Errorf("非蛋模式 WeightFiles 必须全是宿主路径，实得 %q", wf)
		}
	}
	// 反向守卫：非蛋模式不许因新分支凭空多出/少掉绑定（四片 ＝ 四条）
	if got := len(spec.ExtraROBinds); got != 5 { // 4 片 ＋ 1 条 /libs
		t.Errorf("非蛋模式绑定条数应为 5（4 片 ＋ /libs），实得 %d：%v", got, spec.ExtraROBinds)
	}
}

// shardNameFor 真机命名式的第 n 片（与 weightShardRe 同形）。
func shardNameFor(n, total int) string {
	return fmt.Sprintf("Qwen3.8-Flash-Next-UD-Q4_K_XL-%05d-of-%05d.gguf", n, total)
}
