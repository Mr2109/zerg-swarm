package api

// resource_trust_migrate_test.go — 资源信任表路径统一（2026-09-18 修 /tmp 硬编码缺陷）用例。
//
// 缺陷: resource_trust.go 原写死 `const resTrustFile = "/tmp/zerg-resources.json"` ——
// macOS 重启 /tmp 即清 + tmp_cleaner 3 天未访问即删（实测本机两者都在）⇒ 使用次数/故障次数/转正标记
// 全部归零（用了 99 次的资源又回到「🆕」）；多实例还共用同一份文件互相覆盖。
//
// 修后契约（本文件逐条钉死）:
//  ① 写只写统一状态目录（statepath.File → ZERG_STATE_DIR → ~/.zerg/state/zerg-resources.json），目录首次自动建；
//  ② 新路径存在 ⇒ 旧 /tmp 路径完全不看（内容不夹除）；
//  ③ 新路径无、旧 /tmp 有 ⇒ 读旧一次（不丢计数），旧文件不删、不改（sha256 + mtime 双证）；
//  ④ 两都没有 ⇒ 空信任表、不报错、不建文件；
//  ⑤ 显式覆盖（测试隔离）时不退旧路径（防测试读真机 /tmp）；
//  ⑥ 端到端不丢计数：读旧 → 写新 ⇒ 新文件里有旧计数，旧文件原封不动。
//
// 隔离纪律: 一律用 setResTrustPaths 切包级变量（t.Cleanup 严格恢复）+ resetResourceTrust 换全局表
// （t.Cleanup 恢复）+ ZERG_STATE_DIR 指 temp，绝不让用例碰真机 /tmp/zerg-resources.json
// 或 ~/.zerg/state/zerg-resources.json。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setResTrustPaths 切包级信任表路径（新/旧），用例结束严格恢复。
func setResTrustPaths(t *testing.T, primary, legacy string) {
	t.Helper()
	oldPrimary, oldLegacy := resTrustFile, legacyResTrustFile
	resTrustFile, legacyResTrustFile = primary, legacy
	t.Cleanup(func() {
		resTrustFile, legacyResTrustFile = oldPrimary, oldLegacy
	})
}

// resetResourceTrust 换掉全局信任表（用例间互不污染），用例结束恢复原表。
func resetResourceTrust(t *testing.T) *ResourceTrust {
	t.Helper()
	old := resourceTrust
	fresh := &ResourceTrust{
		Models: map[string]ResTrustEntry{},
		Tools:  map[string]ResTrustEntry{},
		Skills: map[string]ResTrustEntry{},
		Mcps:   map[string]ResTrustEntry{},
	}
	resourceTrust = fresh
	t.Cleanup(func() { resourceTrust = old })
	return fresh
}

// writeResTrustFile 造一份落盘的信任表（models 桶放给定条目——验计数是否被夹除/保留）。
func writeResTrustFile(t *testing.T, path string, models map[string]ResTrustEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("造测试文件失败（建目录）: %v", err)
	}
	doc := &ResourceTrust{
		Models: models,
		Tools:  map[string]ResTrustEntry{},
		Skills: map[string]ResTrustEntry{},
		Mcps:   map[string]ResTrustEntry{},
	}
	buf, err := json.MarshalIndent(doc, "", "  ") // 指针入参——不复制 sync.Mutex（go vet copylocks）
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("写测试文件失败 %s: %v", path, err)
	}
	// 固定过去 mtime（复用 tasks_persist_migrate_test.go 的 pastTime）——被改写必然变
	if err := os.Chtimes(path, pastTime, pastTime); err != nil {
		t.Fatalf("设旧文件 mtime 失败 %s: %v", path, err)
	}
}

// modelNames 信任表 models 桶的键快照（稳定排序便于失败信息可读）。
func modelNames(m map[string]ResTrustEntry) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sortStrings(names)
	return names
}

// sortStrings 极简插入排序（避免为本用例引入 sort 依赖的额外语义差异）。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ① 新路径存在 ⇒ 只读新（旧路径有不同内容也不夹除、旧路径完全不看）。
func TestResourceTrust_ReadPrefersStateFile_LegacyIgnored(t *testing.T) {
	tmp := t.TempDir()
	newPath := filepath.Join(tmp, "state", "zerg-resources.json")
	legacy := filepath.Join(tmp, "legacy-resources.json")
	writeResTrustFile(t, newPath, map[string]ResTrustEntry{
		"new-model": {Status: "新", Uses: 7},
	})
	writeResTrustFile(t, legacy, map[string]ResTrustEntry{
		"legacy-model": {Status: "新", Uses: 42},
	})
	setResTrustPaths(t, newPath, legacy)
	rt := resetResourceTrust(t)

	LoadResourceTrust()

	if _, ok := rt.Models["new-model"]; !ok {
		t.Fatalf("应从新路径读到 new-model；实得 %v", modelNames(rt.Models))
	}
	if _, ok := rt.Models["legacy-model"]; ok {
		t.Fatalf("新路径已存在时旧路径必须完全不看——legacy-model 不该被夹进来；实得 %v", modelNames(rt.Models))
	}
	if got := resTrustReadPath(); got != newPath {
		t.Fatalf("读路径应为新路径 %s，实得 %s", newPath, got)
	}
}

// ② 新路径无、旧路径有 ⇒ 读到旧计数；旧文件未被删、未被改（sha256 + mtime 双证）。
func TestResourceTrust_FirstRunReadsLegacyOnce_KeepsLegacyIntact(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir) // 新路径派生自统一状态目录（此目录下此刻还没有文件）
	legacy := filepath.Join(t.TempDir(), "zerg-resources.json")
	writeResTrustFile(t, legacy, map[string]ResTrustEntry{
		"legacy-model": {Status: "新", Uses: 42, Faults: 3},
	})
	setResTrustPaths(t, "", legacy)
	rt := resetResourceTrust(t)

	sumBefore, mtimeBefore := fileSHA256(t, legacy), fileMtime(t, legacy)
	newPath := filepath.Join(stateDir, "zerg-resources.json")

	LoadResourceTrust()

	e, ok := rt.Models["legacy-model"]
	if !ok {
		t.Fatalf("新路径缺失 + 旧路径存在 ⇒ 必须读旧一次（不丢计数），缺 legacy-model；实得 %v", modelNames(rt.Models))
	}
	if e.Uses != 42 || e.Faults != 3 {
		t.Fatalf("旧计数必须原样读回：want uses=42 faults=3，实得 uses=%d faults=%d", e.Uses, e.Faults)
	}
	if sumAfter := fileSHA256(t, legacy); sumAfter != sumBefore {
		t.Fatalf("旧文件内容被改了！sha256 %s → %s", sumBefore, sumAfter)
	}
	if mtimeAfter := fileMtime(t, legacy); !mtimeAfter.Equal(mtimeBefore) {
		t.Fatalf("旧文件被改写了！mtime %v → %v（旧文件必须不删、不改）", mtimeBefore, mtimeAfter)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("旧文件被删了！%s: %v（用户要求保留）", legacy, err)
	}
	if _, err := os.Stat(newPath); err == nil {
		t.Fatalf("读路径不得建文件（只是读旧一次）：%s 不该存在", newPath)
	}
}

// ③ 写入只写新路径：新文件含数据、目录自动创建，旧文件 mtime/内容不变。
func TestResourceTrust_WriteOnlyToStatePath_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录尚不存在——验证自动创建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-resources.json")
	writeResTrustFile(t, legacy, map[string]ResTrustEntry{
		"legacy-model": {Status: "新", Uses: 42},
	})
	setResTrustPaths(t, "", legacy)
	rt := resetResourceTrust(t)

	sumBefore, mtimeBefore := fileSHA256(t, legacy), fileMtime(t, legacy)

	rt.UseResource("models", "fresh-model") // 触发 saveResourceTrust

	newPath := filepath.Join(stateDir, "zerg-resources.json")
	buf, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("写必须落到新路径 %s: %v", newPath, err)
	}
	var doc ResourceTrust
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("新文件不是合法信任表 JSON: %v", err)
	}
	if _, ok := doc.Models["fresh-model"]; !ok {
		t.Fatalf("新文件应含刚使用的 fresh-model，实得 %v", modelNames(doc.Models))
	}
	if _, ok := doc.Models["legacy-model"]; ok {
		t.Fatalf("新文件不该夹带旧文件的其它内容")
	}
	if sumAfter := fileSHA256(t, legacy); sumAfter != sumBefore {
		t.Fatalf("旧文件内容被写回了！sha256 %s → %s", sumBefore, sumAfter)
	}
	if mtimeAfter := fileMtime(t, legacy); !mtimeAfter.Equal(mtimeBefore) {
		t.Fatalf("旧文件被写回了！mtime %v → %v", mtimeBefore, mtimeAfter)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("旧文件被删了！%s: %v", legacy, err)
	}
}

// ④ 两都没有 ⇒ 空信任表、不报错、不建文件（首次运行路径）。
func TestResourceTrust_NeitherExists_EmptyAndNoFileCreated(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "not-exist.json") // 旧文件不存在
	setResTrustPaths(t, "", legacy)
	rt := resetResourceTrust(t)

	LoadResourceTrust()

	if len(rt.Models) != 0 || len(rt.Tools) != 0 || len(rt.Skills) != 0 || len(rt.Mcps) != 0 {
		t.Fatalf("无任何持久化文件时应为空信任表，实得 models=%v", modelNames(rt.Models))
	}
	if _, err := os.Stat(filepath.Join(stateDir, "zerg-resources.json")); err == nil {
		t.Fatalf("首次运行只读不建——状态目录下不该凭空出现 zerg-resources.json")
	}
	if got, want := resTrustReadPath(), resTrustWritePath(); got != want {
		t.Fatalf("两都没有时读路径应等于写路径（新路径）：%s vs %s", got, want)
	}
}

// ⑤ 显式覆盖主路径（测试隔离口径）时不退旧路径——防测试读真机 /tmp 计数。
func TestResourceTrust_ExplicitOverrideSkipsLegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	legacy := filepath.Join(tmp, "legacy.json")
	writeResTrustFile(t, legacy, map[string]ResTrustEntry{
		"legacy-model": {Status: "新", Uses: 42},
	})
	missing := filepath.Join(tmp, "isolated-missing.json") // 覆盖路径不存在
	setResTrustPaths(t, missing, legacy)
	rt := resetResourceTrust(t)

	LoadResourceTrust()

	if len(rt.Models) != 0 {
		t.Fatalf("显式覆盖路径不存在时不该退旧路径读真机计数；实得 %v", modelNames(rt.Models))
	}
	if got := resTrustReadPath(); got != missing {
		t.Fatalf("显式覆盖时读路径应为覆盖值 %s，实得 %s", missing, got)
	}
}

// ⑥ 默认路径由 statepath 统一状态目录派生（ZERG_STATE_DIR 可覆盖）；旧字面量只在 legacy 变量上。
func TestResourceTrust_DefaultPathDerivedFromStateDir(t *testing.T) {
	// 旧路径字面量必须原样保留（迁移兼容的只读来源；测试进程隔离会另切变量，故查常量）
	if resTrustLegacyDefaultPath != "/tmp/zerg-resources.json" {
		t.Fatalf("旧路径字面量应保持 /tmp/zerg-resources.json（只读来源），实得 %q", resTrustLegacyDefaultPath)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setResTrustPaths(t, "", filepath.Join(t.TempDir(), "legacy.json")) // 包级覆盖清空 ⇒ 走派生

	want := filepath.Join(stateDir, "zerg-resources.json")
	if got := resTrustWritePath(); got != want {
		t.Fatalf("写路径应由状态目录派生：want %s, got %s", want, got)
	}
	if got := resTrustWritePath(); strings.HasPrefix(got, "/tmp/") {
		t.Fatalf("默认落点不该再是 /tmp：%s", got)
	}
	if name := filepath.Base(resTrustWritePath()); name != resTrustStateFileName {
		t.Fatalf("状态目录下文件名应为 %s，实得 %s", resTrustStateFileName, name)
	}
}

// ⑦ 端到端不丢计数：读旧 → 写新 ⇒ 新文件里有旧计数，旧文件依旧原封不动。
func TestResourceTrust_MigrationReadThenWrite_CountsNotLost(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-resources.json")
	writeResTrustFile(t, legacy, map[string]ResTrustEntry{
		"legacy-model": {Status: "新", Uses: 99},
	})
	setResTrustPaths(t, "", legacy)
	rt := resetResourceTrust(t)

	sumBefore, mtimeBefore := fileSHA256(t, legacy), fileMtime(t, legacy)

	LoadResourceTrust()                      // 读旧一次
	rt.UseResource("models", "legacy-model") // 用满 100 次路径上的一次使用 → 写新

	buf, err := os.ReadFile(filepath.Join(stateDir, "zerg-resources.json"))
	if err != nil {
		t.Fatalf("读旧一次后首次写盘应落新路径: %v", err)
	}
	var doc ResourceTrust
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("新文件非法 JSON: %v", err)
	}
	e, ok := doc.Models["legacy-model"]
	if !ok {
		t.Fatalf("旧计数必须随首次写盘落到新文件（不丢）；实得 %v", modelNames(doc.Models))
	}
	if e.Uses != 100 {
		t.Fatalf("旧 99 次 + 本次 1 次 = 100（后续 Status 转正式也靠这个数），实得 uses=%d status=%s", e.Uses, e.Status)
	}
	if e.Status != "正式" {
		t.Fatalf("99+1=100 次零故障 ⇒ 应转正式（计数没丢才可能转正），实得 %s", e.Status)
	}
	if sumAfter := fileSHA256(t, legacy); sumAfter != sumBefore {
		t.Fatalf("旧文件被改写：%s → %s", sumBefore, sumAfter)
	}
	if mtimeAfter := fileMtime(t, legacy); !mtimeAfter.Equal(mtimeBefore) {
		t.Fatalf("旧文件被改写：mtime %v → %v", mtimeBefore, mtimeAfter)
	}
}
