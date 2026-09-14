package modeladapter

// ═══ P5 批 1：参数归属 + 预热 / KV 盘接线 ═══════════════════════════════════
// 设计真源：docs/01-设计/设计-子端沙箱化-20260914.md
//   §9.2 参数归属（--ssd-streaming* / --kv-disk-* 是 ds4 的，llama.cpp 是 --moe-stream*）
//   §9.4 预热（无档案不发——§8.4 标定铁律）/ KV 复用（默认关闭）
//   §9.7 KV 落盘四约束（①限大小 ④按卵分目录）
// 这些用例钉住的是**参数归属铁律**与「**无声明不猜数**」——破坏任何一条即破坏本文件。

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ds4 归属参数全集（设计 §9.2 第一行：这五个都属于 ds4，不是 llama.cpp 的）。
var ds4OwnedFlags = []string{
	"--ssd-streaming",
	"--ssd-streaming-preload-experts",
	"--ssd-streaming-cache-experts",
	"--kv-disk-dir",
	"--kv-disk-space-mb",
}

// TestParameterOwnership_DS4ArgsNeverInLlamaLine 参数归属铁律（设计 §9.2）：
// ds4 的五个参数**一个都不许**出现在任何 llama 系适配器的命令行里——
// 即便卵声明把它们全配满也一样（声明只是「菜单」，归属由适配器钉死）。
func TestParameterOwnership_DS4ArgsNeverInLlamaLine(t *testing.T) {
	// 声明配满：预热、KV 盘、ssd 三件套全给——llama 系照样一个都不许吃。
	full := &registry.ModelEntry{
		File:                       "/data/models/x.gguf",
		SsdStreamingPreloadExperts: 512,
		KVDisk:                     &registry.KVDiskDecl{SpaceMB: 3500, Dir: "~/.zerg/kvdisk/x"},
		Custom: map[string]interface{}{
			"ssd":                         true,
			"ssd_streaming_cache_experts": "4GB",
		},
	}
	// 全部 llama 系适配器（generic 兜底 + 各专用适配器）。
	llamaAdapters := []struct {
		name string
		a    ModelAdapter
	}{
		{"generic", &Generic{}},
		{"example-35b-v2", &Ornith{}},
		{"qwen3.6", &Qwen36{}},
		{"qwen3.8-flash", &Qwen38Flash{}},
		{"example-moe-36b", &K2Horizon{}},
		{"gemma", &Gemma{}},
		{"dispatch兜底", Dispatch("完全不认识的模型名")},
	}
	for _, la := range llamaAdapters {
		args := la.a.BuildArgs(full, 9400)
		for _, flag := range ds4OwnedFlags {
			for i, arg := range args {
				if arg == flag {
					t.Fatalf("[%s] 的命令行出现了 ds4 专属参数 %s（归属纠正，设计 §9.2）：%v",
						la.name, flag, args)
				}
				// 值也一起查：--kv-disk-dir 的**目录值**不该泄漏进 llama 命令行。
				if i > 0 && (flag == "--kv-disk-dir" || flag == "--kv-disk-space-mb") &&
					strings.HasPrefix(arg, "/.zerg/kvdisk") {
					t.Fatalf("[%s] 的命令行出现了 KV 盘路径 %s：%v", la.name, arg, args)
				}
			}
		}
	}
}

// TestDS4PreloadExperts_DeclaredOnly 无档案不预热（设计 §9.4 / §8.4）：
// 未声明 ssd_streaming_preload_experts ⇒ **不发** --ssd-streaming-preload-experts；
// 声明了 ⇒ 原值发出（值来自实测档案，适配器只透传、不猜）。
func TestDS4PreloadExperts_DeclaredOnly(t *testing.T) {
	// 无声明：不预热。
	got := (&DS4{}).BuildArgs(&registry.ModelEntry{File: "/m.gguf"}, 9400)
	for _, arg := range got {
		if arg == "--ssd-streaming-preload-experts" {
			t.Fatalf("未声明预热数不应发出该参数，实得 %v", got)
		}
	}
	// 声明 512（实测档案口径）：原值发出。
	got = (&DS4{}).BuildArgs(&registry.ModelEntry{
		File: "/m.gguf", SsdStreamingPreloadExperts: 512,
	}, 9400)
	if !hasPair(got, "--ssd-streaming-preload-experts", "512") {
		t.Fatalf("声明 512 应发出 --ssd-streaming-preload-experts 512，实得 %v", got)
	}
	// 负数是声明错误（校验拒孵），适配器侧兜住不发出。
	got = (&DS4{}).BuildArgs(&registry.ModelEntry{
		File: "/m.gguf", SsdStreamingPreloadExperts: -1,
	}, 9400)
	for _, arg := range got {
		if arg == "--ssd-streaming-preload-experts" {
			t.Fatalf("负预热数不应发出该参数，实得 %v", got)
		}
	}
}

// TestDS4KVDisk_PerEggDir KV 盘按卵分目录（设计 §9.7④：~/.zerg/kvdisk/<卵名>/）：
// ① 声明 kv_disk 但没写 dir ⇒ 用卵名分目录；② 显式 dir 优先（支持 ~ 前缀展开）；
// ③ 两条都落不了地 ⇒ 不发 --kv-disk-dir（不许猜路径）。
func TestDS4KVDisk_PerEggDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("取不到用户主目录，跳过: %v", err)
	}
	// ① 按卵名分目录：不同卵 ⇒ 不同目录，互不踩（§9.7④ 的「换卵不互相踩」）。
	eggA := &registry.ModelEntry{File: "/m.gguf", KVDisk: &registry.KVDiskDecl{SpaceMB: 3500}}
	eggA.SetEggNameForTest("DeepSeek-V4-Flash")
	gotA := (&DS4{}).BuildArgs(eggA, 9400)
	wantA := filepath.Join(home, ".zerg", "kvdisk", "DeepSeek-V4-Flash")
	if !hasPair(gotA, "--kv-disk-dir", wantA) {
		t.Fatalf("按卵分目录应为 %s，实得 %v", wantA, gotA)
	}
	if !hasPair(gotA, "--kv-disk-space-mb", "3500") {
		t.Fatalf("声明了 KV 盘应带上限 --kv-disk-space-mb 3500，实得 %v", gotA)
	}
	eggB := &registry.ModelEntry{File: "/m.gguf", KVDisk: &registry.KVDiskDecl{SpaceMB: 3500}}
	eggB.SetEggNameForTest("GLM-5.3-Flash")
	gotB := (&DS4{}).BuildArgs(eggB, 9401)
	wantB := filepath.Join(home, ".zerg", "kvdisk", "GLM-5.3-Flash")
	if !hasPair(gotB, "--kv-disk-dir", wantB) {
		t.Fatalf("第二枚卵应有自己的 KV 目录 %s，实得 %v", wantB, gotB)
	}
	if wantA == wantB {
		t.Fatal("两枚卵的 KV 目录不该相同（按卵分目录失效）")
	}
	// ② 显式 dir 优先 + ~ 前缀展开。
	eggC := &registry.ModelEntry{File: "/m.gguf",
		KVDisk: &registry.KVDiskDecl{SpaceMB: 3500, Dir: "~/kvdata/ds4"}}
	gotC := (&DS4{}).BuildArgs(eggC, 9402)
	if !hasPair(gotC, "--kv-disk-dir", filepath.Join(home, "kvdata", "ds4")) {
		t.Fatalf("显式 dir 应展开 ~ 并优先，实得 %v", gotC)
	}
	// ③ 无卵名、无显式 dir ⇒ 不发（不许猜路径）。
	eggD := &registry.ModelEntry{File: "/m.gguf", KVDisk: &registry.KVDiskDecl{SpaceMB: 3500}}
	gotD := (&DS4{}).BuildArgs(eggD, 9403)
	for i, arg := range gotD {
		if arg == "--kv-disk-dir" {
			t.Fatalf("无卵名且无显式目录不应发出 --kv-disk-dir，实得 %v", gotD)
		}
		if i > 0 && strings.Contains(arg, "kvdisk") {
			t.Fatalf("猜出来的 KV 路径 %s 不该出现（不许猜路径）：%v", arg, gotD)
		}
	}
	// ④ space_mb 非正（校验层会拒孵；适配器兜住不发上限）。
	eggE := &registry.ModelEntry{File: "/m.gguf",
		KVDisk: &registry.KVDiskDecl{SpaceMB: 0, Dir: "/tmp/kv"}}
	gotE := (&DS4{}).BuildArgs(eggE, 9404)
	if hasPair(gotE, "--kv-disk-space-mb", "0") {
		t.Fatalf("space_mb=0 不应发出上限参数，实得 %v", gotE)
	}
	if !hasPair(gotE, "--kv-disk-dir", "/tmp/kv") {
		t.Fatalf("显式目录仍应发出，实得 %v", gotE)
	}
}

// TestDS4KVDisk_DefaultOff KV 盘默认关闭（设计 §9.4）：nil = 一个 --kv-disk-* 都不发。
func TestDS4KVDisk_DefaultOff(t *testing.T) {
	got := (&DS4{}).BuildArgs(&registry.ModelEntry{File: "/m.gguf"}, 9400)
	for _, arg := range got {
		if strings.HasPrefix(arg, "--kv-disk") {
			t.Fatalf("未声明 KV 盘不应发出 %s，实得 %v", arg, got)
		}
	}
}

// TestRegistryStampsEggName 注册表加载时把键（卵名）盖进条目——
// 这是「按卵分目录」的取数通路（YAML 条目名 → entry.EggName()）。
func TestRegistryStampsEggName(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("取不到用户主目录，跳过: %v", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "agent_models.yaml")
	doc := "DeepSeek-V4-Flash:\n  file: /data/models/v4.gguf\n  backend: ds4-server\n" +
		"  ssd: true\n  kv_disk:\n    space_mb: 3500\n"
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatalf("写临时注册表失败: %v", err)
	}
	reg, err := registry.New(p)
	if err != nil {
		t.Fatalf("加载注册表失败: %v", err)
	}
	e, ok := reg.Get("DeepSeek-V4-Flash")
	if !ok {
		t.Fatal("未取到条目")
	}
	if e.EggName() != "DeepSeek-V4-Flash" {
		t.Fatalf("卵名未盖进条目: %q", e.EggName())
	}
	// kv_disk 声明落字段（不掉进 Custom 兜底）。
	if e.KVDisk == nil || e.KVDisk.SpaceMB != 3500 {
		t.Fatalf("kv_disk 未落字段: %+v (Custom=%v)", e.KVDisk, e.Custom)
	}
	if _, ok := e.Custom["kv_disk"]; ok {
		t.Fatal("kv_disk 掉进了 Custom 兜底（说明字段没声明）")
	}
	// Reload 之后卵名不丢（热更新场景）。
	if err := reg.Reload(); err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	if e, _ = reg.Get("DeepSeek-V4-Flash"); e.EggName() != "DeepSeek-V4-Flash" {
		t.Fatalf("重载后卵名丢失: %q", e.EggName())
	}
	// 端到端：注册表条目直接喂适配器，KV 目录按卵名落地。
	got := (&DS4{}).BuildArgs(e, 9400)
	if !hasPair(got, "--kv-disk-dir", filepath.Join(home, ".zerg", "kvdisk", "DeepSeek-V4-Flash")) {
		t.Fatalf("端到端 KV 目录不对: %v", got)
	}
}

// TestDS4BaselineUnchanged 防回归：无任何新声明的老条目，参数与批 1 之前逐元素相同。
func TestDS4BaselineUnchanged(t *testing.T) {
	got := (&DS4{}).BuildArgs(mkDS4("/m.gguf", map[string]interface{}{
		"ssd":                         true,
		"ssd_streaming_cache_experts": "4GB",
	}), 9400)
	want := []string{"--model", "/m.gguf", "--port", "9400", "--host", "127.0.0.1",
		"--ssd-streaming", "--ssd-streaming-cache-experts", "4GB"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("老条目参数不应变化\n got=%v\nwant=%v", got, want)
	}
}
