package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ═══ P1：虫卵声明字段（EnvReq / schema_version / idle_unload_s）═══════════════
// 这些用例钉住的是**设计真源**里的判据（设计-子端沙箱化-20260914 §4.3 / §6.7 / §6.8 / §13 Q21）：
//   ① 认不得的 schema_version ⇒ 拒孵（不许静默按新格式跑错）；
//   ② 环境需求声明了却缺必填项 ⇒ 拒孵（不许静默跑缺省值）；
//   ③ 遗留条目（三个字段全未声明）⇒ **告警**（不许静默），照孵；
//   ④ 空窗收走阈值缺省 600、小模型 120；**没有「常驻」这个类别**。

// TestEggDeclaration_DefaultConstants 缺省值就是设计里写死的数字——改动即破坏本用例。
func TestEggDeclaration_DefaultConstants(t *testing.T) {
	if EggSchemaVersionCurrent != 1 {
		t.Fatalf("EggSchemaVersionCurrent = %d，设计 §6.8.4 的当前格式版本号为 1", EggSchemaVersionCurrent)
	}
	if DefaultIdleUnloadSeconds != 600 {
		t.Fatalf("DefaultIdleUnloadSeconds = %d，设计 §4.3 第九项为 600", DefaultIdleUnloadSeconds)
	}
	if SmallModelIdleUnloadSeconds != 120 {
		t.Fatalf("SmallModelIdleUnloadSeconds = %d，设计 §13 Q21 的小模型建议为 120", SmallModelIdleUnloadSeconds)
	}
}

// TestEggDeclaration_YAMLTags 三个字段的 yaml 标签必须真的落在**声明字段**上（不能掉进 Custom 兜底）。
func TestEggDeclaration_YAMLTags(t *testing.T) {
	const doc = `
example-moe-36b:
  file: /data/models/k2/k2horizon-q4_k_m.gguf
  backend: llama-server
  mem_gb: 23
  cmd: /home/g01/agent/run-k2.sh -m {file} --port {port}
  schema_version: 1
  idle_unload_s: 120
  env_req:
    devices: ["0"]
    lib_paths: ["/home/g01/llama-k2/build-k2/bin"]
    env:
      LD_LIBRARY_PATH: ""
    weights: ["/data/models/k2"]
    memlock_kb: 67108864
    mmap_max_count: 1048576
  ssd_streaming_cache_experts: 32
`
	var models map[string]*ModelEntry
	if err := yaml.Unmarshal([]byte(doc), &models); err != nil {
		t.Fatalf("解析 YAML 失败: %v", err)
	}
	e, ok := models["example-moe-36b"]
	if !ok {
		t.Fatal("未解析到 example-moe-36b 条目")
	}
	if e.SchemaVersion != 1 {
		t.Fatalf("schema_version 未落字段（got %d）", e.SchemaVersion)
	}
	if e.IdleUnloadS != 120 {
		t.Fatalf("idle_unload_s 未落字段（got %d）", e.IdleUnloadS)
	}
	if e.EnvReq == nil {
		t.Fatal("env_req 未落字段（期望 *EnvReq 非 nil）")
	}
	if len(e.EnvReq.Devices) != 1 || e.EnvReq.Devices[0] != "0" {
		t.Fatalf("env_req.devices 解析错: %v", e.EnvReq.Devices)
	}
	if len(e.EnvReq.LibPaths) != 1 || !strings.Contains(e.EnvReq.LibPaths[0], "llama-k2") {
		t.Fatalf("env_req.lib_paths 解析错: %v", e.EnvReq.LibPaths)
	}
	if len(e.EnvReq.Weights) != 1 {
		t.Fatalf("env_req.weights 解析错: %v", e.EnvReq.Weights)
	}
	if e.EnvReq.MemlockKB != 67108864 || e.EnvReq.MmapMaxCount != 1048576 {
		t.Fatalf("env_req 限额解析错: memlock=%d mmap=%d", e.EnvReq.MemlockKB, e.EnvReq.MmapMaxCount)
	}
	// 「清空 LD_LIBRARY_PATH」必须是**显式声明的空串**（K2 包装脚本的语义）。
	if v, ok := e.EnvReq.Env["LD_LIBRARY_PATH"]; !ok || v != "" {
		t.Fatalf("env_req.env[LD_LIBRARY_PATH] 应为显式空串, got %q (present=%v)", v, ok)
	}
	// 未声明字段仍要落进 Custom（inline 兜底语义未被破坏）。
	if _, ok := e.Custom["ssd_streaming_cache_experts"]; !ok {
		t.Fatalf("未声明字段应落进 Custom，实际 Custom=%v", e.Custom)
	}
	for _, k := range []string{"schema_version", "idle_unload_s", "env_req"} {
		if _, ok := e.Custom[k]; ok {
			t.Fatalf("%s 掉进了 Custom 兜底（说明字段没声明）", k)
		}
	}
}

// TestValidateEggDeclaration_LegacyEntryWarnsNotFatal 遗留条目（P1 之前写的注册表）：
// **告警但照孵**，绝不静默——本仓读不到 X3 本机那份 yaml，清单落地（P7）时才强制补齐。
func TestValidateEggDeclaration_LegacyEntryWarnsNotFatal(t *testing.T) {
	e := &ModelEntry{File: "/data/models/x.gguf", Backend: "llama-server", MemGB: 20}
	res := ValidateEggDeclaration("老条目", e)
	if !res.OK() {
		t.Fatalf("遗留条目不应 Fatal，实际: %v", res.Fatal)
	}
	if len(res.Warnings) < 2 {
		t.Fatalf("遗留条目必须给出告警（schema_version + 卵声明字段），实际只有 %d 条: %v", len(res.Warnings), res.Warnings)
	}
	w := res.WarningString()
	if !strings.Contains(w, "schema_version") {
		t.Fatalf("告警未提 schema_version: %s", w)
	}
	if !strings.Contains(w, "P7") {
		t.Fatalf("告警未写明补齐期限（清单落地 P7）: %s", w)
	}
	if !strings.Contains(w, "600") {
		t.Fatalf("告警未写明缺省空窗收走阈值 600s: %s", w)
	}
	// 缺省访问器：未声明 ⇒ 600。
	if got := e.IdleUnloadSeconds(); got != 600 {
		t.Fatalf("未声明 idle_unload_s 时应取 600，实际 %d", got)
	}
}

// TestValidateEggDeclaration_UnknownSchemaVersionFatal 认不得的版本 ⇒ 拒孵（核心红线）。
func TestValidateEggDeclaration_UnknownSchemaVersionFatal(t *testing.T) {
	for _, v := range []int{2, 99, -1} {
		e := &ModelEntry{File: "/data/models/x.gguf", Backend: "llama-server", MemGB: 20, SchemaVersion: v}
		res := ValidateEggDeclaration("未来卵", e)
		if res.OK() {
			t.Fatalf("schema_version=%d 必须拒孵（认不得的版本不许静默按新格式跑错）", v)
		}
		msg := res.ErrorString()
		if !strings.Contains(msg, "未来卵") {
			t.Fatalf("拒孵理由必须说清是哪一枚卵: %s", msg)
		}
		if !strings.Contains(msg, "schema_version") {
			t.Fatalf("拒孵理由必须点名 schema_version: %s", msg)
		}
		if v > 0 && !strings.Contains(msg, "1") {
			t.Fatalf("拒孵理由必须给出期望版本号 1: %s", msg)
		}
	}
}

// TestValidateEggDeclaration_KnownVersionNoFatal 认得的版本 ⇒ 不因版本拒孵。
func TestValidateEggDeclaration_KnownVersionNoFatal(t *testing.T) {
	e := &ModelEntry{
		File: "/data/models/x.gguf", Backend: "llama-server", MemGB: 20,
		SchemaVersion: EggSchemaVersionCurrent, IdleUnloadS: 600,
	}
	res := ValidateEggDeclaration("当前卵", e)
	if !res.OK() {
		t.Fatalf("schema_version=%d 不应拒孵: %v", EggSchemaVersionCurrent, res.Fatal)
	}
	// 已声明版本、未声明 env_req、阈值 600 ⇒ 无告警（干净路径不该刷日志）。
	if len(res.Warnings) != 0 {
		t.Fatalf("干净声明不应有告警: %v", res.Warnings)
	}
	if got := e.IdleUnloadSeconds(); got != 600 {
		t.Fatalf("显式 600 应原样返回，实际 %d", got)
	}
}

// TestValidateEggDeclaration_EnvReqMissingRequiredFatal 环境需求缺必填项 ⇒ 拒孵，且逐项点名。
func TestValidateEggDeclaration_EnvReqMissingRequiredFatal(t *testing.T) {
	e := &ModelEntry{
		File: "/data/models/x.gguf", Backend: "llama-server", MemGB: 20,
		SchemaVersion: 1, IdleUnloadS: 600,
		// 只给了权重路径：库路径、两项限额都缺。
		EnvReq: &EnvReq{Weights: []string{"/data/models/x"}},
	}
	res := ValidateEggDeclaration("缺项的卵", e)
	if res.OK() {
		t.Fatal("环境需求缺必填项必须拒孵（不许孵化器自己推断缺省值）")
	}
	msg := res.ErrorString()
	for _, want := range []string{"lib_paths", "memlock_kb", "mmap_max_count"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("拒孵理由必须点名缺失项 %s: %s", want, msg)
		}
	}
	if strings.Contains(msg, "weights") {
		t.Fatalf("weights 已声明，不该被列为缺失: %s", msg)
	}
}

// TestValidateEggDeclaration_EnvReqCompleteOK 四类必填齐备 ⇒ 可以孵。
func TestValidateEggDeclaration_EnvReqCompleteOK(t *testing.T) {
	e := &ModelEntry{
		File: "/data/models/k2/k2horizon-q4_k_m.gguf", Backend: "llama-server", MemGB: 23,
		SchemaVersion: 1, IdleUnloadS: 120,
		EnvReq: &EnvReq{
			Devices:      []string{"0"},
			LibPaths:     []string{"/home/g01/llama-k2/build-k2/bin"},
			Env:          map[string]string{"LD_LIBRARY_PATH": "", "OMP_NUM_THREADS": "16"},
			Weights:      []string{"/data/models/k2"},
			MemlockKB:    67108864,
			MmapMaxCount: 1048576,
		},
	}
	res := ValidateEggDeclaration("example-moe-36b", e)
	if !res.OK() {
		t.Fatalf("齐备的环境需求不应拒孵: %v", res.Fatal)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("齐备声明不应有告警: %v", res.Warnings)
	}
	if got := e.IdleUnloadSeconds(); got != 120 {
		t.Fatalf("小模型建议 120s 应原样返回，实际 %d", got)
	}
}

// TestValidateEggDeclaration_EmptyCardVarFatal 卡号类变量给了空值 = 等于没声明 ⇒ 拒孵；
// 而 LD_LIBRARY_PATH 的空串是**合法的显式清空**（K2 真实案例），必须不报错。
func TestValidateEggDeclaration_EmptyCardVarFatal(t *testing.T) {
	e := &ModelEntry{
		File: "/data/models/x.gguf", Backend: "llama-server", MemGB: 20,
		SchemaVersion: 1, IdleUnloadS: 600,
		EnvReq: &EnvReq{
			LibPaths:  []string{"/opt/engine/lib"},
			Env:       map[string]string{"HIP_VISIBLE_DEVICES": "", "LD_LIBRARY_PATH": ""},
			Weights:   []string{"/data/models/x"},
			MemlockKB: 1024, MmapMaxCount: 65530,
		},
	}
	res := ValidateEggDeclaration("多卡机器上的卵", e)
	if res.OK() {
		t.Fatal("HIP_VISIBLE_DEVICES 声明为空值必须拒孵（多卡必须声明用哪张卡）")
	}
	if len(res.Fatal) != 1 {
		t.Fatalf("期望 1 条拒孵理由，实际 %d: %v", len(res.Fatal), res.Fatal)
	}
	msg := res.ErrorString()
	if !strings.Contains(msg, "HIP_VISIBLE_DEVICES 为空值") {
		t.Fatalf("拒孵理由必须点名 HIP_VISIBLE_DEVICES: %s", msg)
	}
	// 判据只能看「被判为空值」这个**标记**，不能只看字符串出现——
	// 提示语里本来就会举 LD_LIBRARY_PATH 当合法的清空例子。
	if strings.Contains(msg, "LD_LIBRARY_PATH 为空值") {
		t.Fatalf("LD_LIBRARY_PATH 的空串是合法清空，不该被当成错误: %s", msg)
	}
}

// TestValidateEggDeclaration_NilEntry 空条目也要说清是哪一枚卵（不留半截报错）。
func TestValidateEggDeclaration_NilEntry(t *testing.T) {
	res := ValidateEggDeclaration("", nil)
	if res.OK() {
		t.Fatal("空声明必须拒孵")
	}
	if !strings.Contains(res.ErrorString(), "未命名") {
		t.Fatalf("空名要给可读占位: %s", res.ErrorString())
	}
}

// ═══ P5 批 1：KV 盘声明校验（设计 §9.7①：写盘上限不许留成「无限」）══════════

// TestValidateEggDeclaration_KVDiskSpaceMBRequired 声明了 KV 盘却不给正数上限 ⇒ 拒孵
// （KV 落盘是持续写，不限大小等于把持续写敞成无限）。
func TestValidateEggDeclaration_KVDiskSpaceMBRequired(t *testing.T) {
	for _, mb := range []int{0, -100} {
		e := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
			KVDisk: &KVDiskDecl{SpaceMB: mb, Dir: "/tmp/kv"}}
		e.SetEggNameForTest("DeepSeek-V4-Flash")
		res := ValidateEggDeclaration("不限大小的卵", e)
		if res.OK() {
			t.Fatalf("kv_disk.space_mb=%d 必须拒孵（设计 §9.7①）", mb)
		}
		if msg := res.ErrorString(); !strings.Contains(msg, "space_mb") {
			t.Fatalf("拒孵理由必须点名 space_mb: %s", msg)
		}
	}
}

// TestValidateEggDeclaration_KVDiskNeedsDirOrEggName 声明了 KV 盘但既无显式目录
// 又无卵名 ⇒ 按卵分目录无从落地 ⇒ 拒孵；有卵名或有显式目录 ⇒ 可孵。
func TestValidateEggDeclaration_KVDiskNeedsDirOrEggName(t *testing.T) {
	// 两者皆无 ⇒ 拒孵。
	e := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
		KVDisk: &KVDiskDecl{SpaceMB: 3500}}
	res := ValidateEggDeclaration("没名也没目录的卵", e)
	if res.OK() {
		t.Fatal("KV 盘既无显式目录又无卵名必须拒孵（按卵分目录无从落地，设计 §9.7④）")
	}
	// 有卵名 ⇒ 可孵（缺省 ~/.zerg/kvdisk/<卵名>/）。
	e2 := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
		KVDisk: &KVDiskDecl{SpaceMB: 3500}}
	e2.SetEggNameForTest("DeepSeek-V4-Flash")
	if res := ValidateEggDeclaration("有名字的卵", e2); !res.OK() {
		t.Fatalf("有卵名即可按卵分目录，不应拒孵: %v", res.Fatal)
	}
	// 有显式目录（无卵名）⇒ 可孵。
	e3 := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
		KVDisk: &KVDiskDecl{SpaceMB: 3500, Dir: "/tmp/kv-x"}}
	if res := ValidateEggDeclaration("有目录的卵", e3); !res.OK() {
		t.Fatalf("有显式目录即可落地，不应拒孵: %v", res.Fatal)
	}
}

// TestValidateEggDeclaration_PreloadExpertsNegative 预热专家数为负 ⇒ 拒孵
// （0 = 未声明 = 无档案不预热，合法；正数 = 实测档案口径，合法）。
func TestValidateEggDeclaration_PreloadExpertsNegative(t *testing.T) {
	e := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
		SsdStreamingPreloadExperts: -8}
	res := ValidateEggDeclaration("负预热的卵", e)
	if res.OK() {
		t.Fatal("预热专家数为负必须拒孵")
	}
	if msg := res.ErrorString(); !strings.Contains(msg, "ssd_streaming_preload_experts") {
		t.Fatalf("拒孵理由必须点名 ssd_streaming_preload_experts: %s", msg)
	}
	// 0（未声明）与正数（实测档案）都合法。
	for _, p := range []int{0, 512} {
		e2 := &ModelEntry{File: "/data/models/v4.gguf", SchemaVersion: 1,
			SsdStreamingPreloadExperts: p}
		if res := ValidateEggDeclaration("正常的卵", e2); !res.OK() {
			t.Fatalf("ssd_streaming_preload_experts=%d 不应拒孵: %v", p, res.Fatal)
		}
	}
}

// TestValidateEgg_RegistryLookup 注册表入口：未登记的模型不能孵；空注册表不能孵。
func TestValidateEgg_RegistryLookup(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agent_models.yaml")
	doc := "当前卵:\n  file: /data/models/x.gguf\n  backend: llama-server\n  mem_gb: 20\n" +
		"  schema_version: 1\n  idle_unload_s: 600\n"
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatalf("写临时注册表失败: %v", err)
	}
	reg, err := New(p)
	if err != nil {
		t.Fatalf("加载注册表失败: %v", err)
	}
	if res := reg.ValidateEgg("当前卵"); !res.OK() {
		t.Fatalf("已登记的合格卵不该拒孵: %v", res.Fatal)
	}
	res := reg.ValidateEgg("不存在的卵")
	if res.OK() {
		t.Fatal("未登记的模型不能孵")
	}
	if !strings.Contains(res.ErrorString(), "不在注册表里") {
		t.Fatalf("拒孵理由应说清未登记: %s", res.ErrorString())
	}
	var nilReg *Registry
	if res := nilReg.ValidateEgg("任意卵"); res.OK() {
		t.Fatal("空注册表不能孵")
	}
}
