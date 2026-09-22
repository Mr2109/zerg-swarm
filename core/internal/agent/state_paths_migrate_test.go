package agent

// state_paths_migrate_test.go — core/internal/agent 三处硬编码状态路径统一（2026-09-18）用例。
//
// 缺陷（口径同 api/tasks_persist.go 上一批）:
//   ① idle_persist.go   const idleStateFile   = "/tmp/zerg-idle-state.json"
//   ② net_tools.go      const searchCacheFile = "/tmp/zerg-search-cache.json"
//   ③ bash_v101.go      const BashOverflowDir = "/tmp/zerg-bash-overflow"（目录）
//   /tmp = macOS 重启即清 + tmp_cleaner 3 天未访问即删（状态/缓存/溢出原文丢）；多实例共用同一份。
//
// 修后契约（本文件逐条钉死）:
//   ① 写只写统一状态目录（statepath → ZERG_STATE_DIR → ~/.zerg/state/<name>），新目录首次自动建；
//   ② 新路径存在 ⇒ 旧 /tmp 完全不看（不读、不夹除）；目录类: 旧目录只作**只读访问许可**保留；
//   ③ 新无旧有 ⇒ 读旧一次；旧文件/目录不删、不改（sha256 + mtime 双证）；
//   ④ 两都无 ⇒ 空启动不报错、不建文件、读路径 == 写路径；
//   ⑤ 显式覆盖（测试隔离）时不退旧路径（防测试读真机 /tmp）。
//
// 隔离纪律: 一律切包级变量（t.Cleanup 严格恢复）+ ZERG_STATE_DIR 指 temp——
// 绝不碰真机 /tmp/zerg-{idle-state,search-cache}.json 与 /tmp/zerg-bash-overflow。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// zergPastTime 旧文件/旧目录内容的固定 mtime（2020-01-01T00:00:00Z）。
// 固定过去时间：若代码改写了旧物，mtime 必变（纳秒粒度，不会漏判）。
var zergPastTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// ───────── 通用断言工具 ─────────

func stateFileSHA256(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件失败 %s: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func stateFileMtime(t *testing.T, path string) time.Time {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败 %s: %v", path, err)
	}
	return st.ModTime()
}

// dirFileNames 目录下的文件名（排序——旧目录没被写/删的硬证据）。
func dirFileNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("列目录失败 %s: %v", dir, err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// mustNotExist 断言路径不存在（读只读、不凭空建文件）。
func mustNotExist(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s：%s 不该存在", why, path)
	}
}

// ───────── ① 断点状态（idle_persist.go）─────────

func setIdleStatePaths(t *testing.T, primary, legacy string) {
	t.Helper()
	oldPrimary, oldLegacy := idleStateFile, legacyIdleStateFile
	idleStateFile, legacyIdleStateFile = primary, legacy
	t.Cleanup(func() { idleStateFile, legacyIdleStateFile = oldPrimary, oldLegacy })
}

// writeIdleStateFileAt 造一份断点状态文件（key → RFC3339 时间），mtime 打成固定过去时间。
func writeIdleStateFileAt(t *testing.T, path string, entries map[string]time.Time) {
	t.Helper()
	data := make(map[string]string, len(entries))
	for k, v := range entries {
		data[k] = v.Format(time.RFC3339)
	}
	buf, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("造测试文件失败（建目录）: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("写测试文件失败 %s: %v", path, err)
	}
	if err := os.Chtimes(path, zergPastTime, zergPastTime); err != nil {
		t.Fatalf("设旧文件 mtime 失败 %s: %v", path, err)
	}
}

// TestIdleStatePath_ReadPrefersStateFile_LegacyIgnored
// 新路径存在 ⇒ 只读新（旧路径内容不夹除、旧路径完全不看）。
func TestIdleStatePath_ReadPrefersStateFile_LegacyIgnored(t *testing.T) {
	tmp := t.TempDir()
	newPath := filepath.Join(tmp, "state", "zerg-idle-state.json")
	legacy := filepath.Join(tmp, "legacy-idle-state.json")
	writeIdleStateFileAt(t, newPath, map[string]time.Time{"new-task": zergPastTime})
	writeIdleStateFileAt(t, legacy, map[string]time.Time{"legacy-task": zergPastTime})
	setIdleStatePaths(t, newPath, legacy)
	sumBefore := stateFileSHA256(t, legacy)

	d := NewIdleDetector(t.TempDir())

	if _, ok := d.lastTriggered["new-task"]; !ok {
		t.Fatalf("应从新路径读到 new-task；实得 %v", d.lastTriggered)
	}
	if _, ok := d.lastTriggered["legacy-task"]; ok {
		t.Fatalf("新路径已存在时旧路径必须完全不看——legacy-task 不该被夹进来")
	}
	if got := idleReadPath(); got != newPath {
		t.Fatalf("读路径应为新路径 %s，实得 %s", newPath, got)
	}
	if got := idleWritePath(); got != newPath {
		t.Fatalf("写路径应为新路径 %s，实得 %s", newPath, got)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("只是读——旧文件不该被动：%s → %s", sumBefore, got)
	}
}

// TestIdleStatePath_FirstRunReadsLegacyOnce_KeepsLegacyIntact
// 新无旧有 ⇒ 读旧一次（断点不丢）；旧文件不删、不改（sha256 + mtime 双证）；读不建新文件。
func TestIdleStatePath_FirstRunReadsLegacyOnce_KeepsLegacyIntact(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-idle-state.json")
	writeIdleStateFileAt(t, legacy, map[string]time.Time{
		"legacy-a": zergPastTime,
		"legacy-b": zergPastTime.Add(time.Minute),
	})
	setIdleStatePaths(t, "", legacy)
	sumBefore, mtimeBefore := stateFileSHA256(t, legacy), stateFileMtime(t, legacy)

	d := NewIdleDetector(t.TempDir())

	for _, k := range []string{"legacy-a", "legacy-b"} {
		if _, ok := d.lastTriggered[k]; !ok {
			t.Fatalf("新路径缺失 + 旧路径存在 ⇒ 必须读旧一次（断点不丢），缺 %s；实得 %v", k, d.lastTriggered)
		}
	}
	if got := d.lastTriggered["legacy-b"].Unix(); got != zergPastTime.Add(time.Minute).Unix() {
		t.Fatalf("旧断点时间戳应原样恢复，实得 %v", d.lastTriggered["legacy-b"])
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("旧文件内容被改了！sha256 %s → %s", sumBefore, got)
	}
	if got := stateFileMtime(t, legacy); !got.Equal(mtimeBefore) {
		t.Fatalf("旧文件被改写了！mtime %v → %v（旧文件必须不删、不改）", mtimeBefore, got)
	}
	mustNotExist(t, filepath.Join(stateDir, idleStateFileName), "读路径不得建文件（只是读旧一次）")
}

// TestIdleStatePath_NeitherExists_EmptyNoError
// 两都没有 ⇒ 空断点、不报错、不建文件；读路径 == 写路径（新路径）。
func TestIdleStatePath_NeitherExists_EmptyNoError(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "not-exist.json")
	setIdleStatePaths(t, "", legacy)

	d := NewIdleDetector(t.TempDir())

	if len(d.lastTriggered) != 0 {
		t.Fatalf("无任何断点文件时应为空状态，实得 %v", d.lastTriggered)
	}
	if got, want := idleReadPath(), idleWritePath(); got != want {
		t.Fatalf("两都没有时读路径应等于写路径（新路径）：%s vs %s", got, want)
	}
	mustNotExist(t, filepath.Join(stateDir, idleStateFileName), "首次运行只读不建")
}

// TestIdleStatePath_WriteOnlyToStateDir_LegacyUntouched
// 写只落新路径（目录自动建）；旧文件内容/mtime 不变；读旧→写新不丢断点。
func TestIdleStatePath_WriteOnlyToStateDir_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录尚不存在——验证自动创建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-idle-state.json")
	writeIdleStateFileAt(t, legacy, map[string]time.Time{"legacy-a": zergPastTime})
	setIdleStatePaths(t, "", legacy)
	sumBefore, mtimeBefore := stateFileSHA256(t, legacy), stateFileMtime(t, legacy)

	d := NewIdleDetector(t.TempDir()) // 读旧一次
	d.lastTriggered["new-b"] = zergPastTime.Add(time.Hour)
	d.saveIdleState()

	newPath := filepath.Join(stateDir, idleStateFileName)
	buf, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("写必须落到新路径 %s: %v", newPath, err)
	}
	var data map[string]string
	if err := json.Unmarshal(buf, &data); err != nil {
		t.Fatalf("新文件不是合法断点 JSON: %v", err)
	}
	if _, ok := data["new-b"]; !ok {
		t.Fatalf("新文件应含刚保存的 new-b，实得 %v", data)
	}
	if _, ok := data["legacy-a"]; !ok {
		t.Fatalf("读旧一次后首次写盘应带上旧断点（不丢），实得 %v", data)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("旧文件内容被写回了！sha256 %s → %s", sumBefore, got)
	}
	if got := stateFileMtime(t, legacy); !got.Equal(mtimeBefore) {
		t.Fatalf("旧文件被写回了！mtime %v → %v", mtimeBefore, got)
	}
}

// TestIdleStatePath_ExplicitOverrideSkipsLegacyFallback
// 显式覆盖（测试隔离）时不退旧路径——防测试读真机 /tmp。
func TestIdleStatePath_ExplicitOverrideSkipsLegacyFallback(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), "zerg-idle-state.json")
	writeIdleStateFileAt(t, legacy, map[string]time.Time{"legacy-a": zergPastTime})
	missing := filepath.Join(t.TempDir(), "isolated-missing.json") // 覆盖路径不存在
	setIdleStatePaths(t, missing, legacy)
	sumBefore := stateFileSHA256(t, legacy)

	d := NewIdleDetector(t.TempDir())

	if len(d.lastTriggered) != 0 {
		t.Fatalf("显式覆盖路径不存在时不该退旧路径，实得 %v", d.lastTriggered)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("覆盖态下旧文件更不该被动：%s → %s", sumBefore, got)
	}
}

// TestIdleStatePath_DefaultDerivedFromStateDir_NotTmp
// 默认落点由 statepath 统一状态目录派生；旧字面量只在 legacy 常量上保留。
func TestIdleStatePath_DefaultDerivedFromStateDir_NotTmp(t *testing.T) {
	if idleLegacyDefaultPath != "/tmp/zerg-idle-state.json" {
		t.Fatalf("旧路径字面量应保持 /tmp/zerg-idle-state.json（只读来源），实得 %q", idleLegacyDefaultPath)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setIdleStatePaths(t, "", filepath.Join(t.TempDir(), "legacy.json"))

	want := filepath.Join(stateDir, idleStateFileName)
	if got := idleWritePath(); got != want {
		t.Fatalf("写路径应由状态目录派生：want %s, got %s", want, got)
	}
	if got := idleReadPath(); got != want {
		t.Fatalf("无新无旧时读路径应为派生路径：want %s, got %s", want, got)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #7 · §八 待拍 2 裁定）：删掉旧断言
	//   「不以 /tmp/ 开头」—— 它在 TMPDIR=/tmp（Linux runner）下恒假，是自伤而非判据；
	//   真命题已由上两条 `got != want`（写/读路径 = 状态目录派生值）判过 ⇒ 不留第三条。
}

// ───────── ② 搜索缓存（net_tools.go）─────────

func setSearchCachePaths(t *testing.T, primary, legacy string) {
	t.Helper()
	oldPrimary, oldLegacy := searchCacheFile, legacySearchCacheFile
	searchCacheFile, legacySearchCacheFile = primary, legacy
	t.Cleanup(func() { searchCacheFile, legacySearchCacheFile = oldPrimary, oldLegacy })
}

// resetSearchCacheMap 清进程内缓存 map（LoadSearchCache 是 merge 语义，用例间必须归零）。
func resetSearchCacheMap() {
	searchCacheMu.Lock()
	searchCacheMap = map[string]searchCacheEntry{}
	searchCacheMu.Unlock()
}

func searchCacheSize() int {
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	return len(searchCacheMap)
}

// writeSearchCacheFileAt 造一份缓存文件（未过期——expire 在 1 小时后）。
func writeSearchCacheFileAt(t *testing.T, path string, kvs map[string]string) {
	t.Helper()
	data := make(map[string]searchCacheEntry, len(kvs))
	for k, v := range kvs {
		data[k] = searchCacheEntry{Result: v, Expire: time.Now().Add(time.Hour).Unix()}
	}
	buf, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("造测试文件失败（建目录）: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("写测试文件失败 %s: %v", path, err)
	}
	if err := os.Chtimes(path, zergPastTime, zergPastTime); err != nil {
		t.Fatalf("设旧文件 mtime 失败 %s: %v", path, err)
	}
}

// waitForFileContent 等异步落盘（searchCachePut 是 goroutine 写）——超时即失败。
func waitForFileContent(t *testing.T, path string, d time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		if b, err := os.ReadFile(path); err == nil {
			return b
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待落盘超时: %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSearchCachePath_ReadPrefersStateFile_LegacyIgnored
// 新路径存在 ⇒ 只读新（旧缓存不夹除）。
func TestSearchCachePath_ReadPrefersStateFile_LegacyIgnored(t *testing.T) {
	resetSearchCacheMap()
	tmp := t.TempDir()
	newPath := filepath.Join(tmp, "state", "zerg-search-cache.json")
	legacy := filepath.Join(tmp, "legacy-search-cache.json")
	writeSearchCacheFileAt(t, newPath, map[string]string{"k-new": "res-new"})
	writeSearchCacheFileAt(t, legacy, map[string]string{"k-legacy": "res-legacy"})
	setSearchCachePaths(t, newPath, legacy)
	sumBefore := stateFileSHA256(t, legacy)

	LoadSearchCache()

	if got := searchCacheGet("k-new"); got != "res-new" {
		t.Fatalf("应从新路径读到 k-new，实得 %q", got)
	}
	if got := searchCacheGet("k-legacy"); got != "" {
		t.Fatalf("新路径已存在时旧路径必须完全不看，实得 %q", got)
	}
	if got := searchCacheReadPath(); got != newPath {
		t.Fatalf("读路径应为新路径 %s，实得 %s", newPath, got)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("只是读——旧文件不该被动：%s → %s", sumBefore, got)
	}
}

// TestSearchCachePath_FirstRunReadsLegacyOnce_KeepsLegacyIntact
// 新无旧有 ⇒ 读旧一次（缓存命中不丢）；旧文件不删、不改；读不建新文件。
func TestSearchCachePath_FirstRunReadsLegacyOnce_KeepsLegacyIntact(t *testing.T) {
	resetSearchCacheMap()
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-search-cache.json")
	writeSearchCacheFileAt(t, legacy, map[string]string{"k-legacy": "res-legacy"})
	setSearchCachePaths(t, "", legacy)
	sumBefore, mtimeBefore := stateFileSHA256(t, legacy), stateFileMtime(t, legacy)

	LoadSearchCache()

	if got := searchCacheGet("k-legacy"); got != "res-legacy" {
		t.Fatalf("新无旧有 ⇒ 必须读旧一次，实得 %q", got)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("旧文件内容被改了！sha256 %s → %s", sumBefore, got)
	}
	if got := stateFileMtime(t, legacy); !got.Equal(mtimeBefore) {
		t.Fatalf("旧文件被改写了！mtime %v → %v", mtimeBefore, got)
	}
	mustNotExist(t, filepath.Join(stateDir, searchCacheFileName), "读路径不得建文件")
}

// TestSearchCachePath_NeitherExists_EmptyNoError
// 两都没有 ⇒ 空缓存、不报错、不建文件；读路径 == 写路径。
func TestSearchCachePath_NeitherExists_EmptyNoError(t *testing.T) {
	resetSearchCacheMap()
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setSearchCachePaths(t, "", filepath.Join(t.TempDir(), "not-exist.json"))

	LoadSearchCache()

	if n := searchCacheSize(); n != 0 {
		t.Fatalf("无任何缓存文件时缓存应为空，实得 %d 条", n)
	}
	if got, want := searchCacheReadPath(), searchCacheWritePath(); got != want {
		t.Fatalf("两都没有时读路径应等于写路径：%s vs %s", got, want)
	}
	mustNotExist(t, filepath.Join(stateDir, searchCacheFileName), "首次运行只读不建")
}

// TestSearchCachePath_WriteOnlyToStateDir_LegacyUntouched
// 写只落新路径（目录自动建，异步写真实落盘）；旧文件 mtime/内容不变。
func TestSearchCachePath_WriteOnlyToStateDir_LegacyUntouched(t *testing.T) {
	resetSearchCacheMap()
	stateDir := filepath.Join(t.TempDir(), "nested", "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-search-cache.json")
	writeSearchCacheFileAt(t, legacy, map[string]string{"k-legacy": "res-legacy"})
	setSearchCachePaths(t, "", legacy)
	sumBefore, mtimeBefore := stateFileSHA256(t, legacy), stateFileMtime(t, legacy)

	searchCachePut("q-write", "res-write")

	newPath := filepath.Join(stateDir, searchCacheFileName)
	buf := waitForFileContent(t, newPath, 3*time.Second)
	var data map[string]searchCacheEntry
	if err := json.Unmarshal(buf, &data); err != nil {
		t.Fatalf("新缓存文件不是合法 JSON: %v", err)
	}
	if e, ok := data["q-write"]; !ok || e.Result != "res-write" {
		t.Fatalf("新缓存文件应含 q-write，实得 %v", data)
	}
	if _, ok := data["k-legacy"]; ok {
		t.Fatalf("新缓存文件不该夹带旧文件内容")
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("旧文件内容被写回了！sha256 %s → %s", sumBefore, got)
	}
	if got := stateFileMtime(t, legacy); !got.Equal(mtimeBefore) {
		t.Fatalf("旧文件被写回了！mtime %v → %v", mtimeBefore, got)
	}
	resetSearchCacheMap()
}

// TestSearchCachePath_ExplicitOverrideSkipsLegacyFallback
// 显式覆盖时不退旧路径——防测试读真机 /tmp。
func TestSearchCachePath_ExplicitOverrideSkipsLegacyFallback(t *testing.T) {
	resetSearchCacheMap()
	legacy := filepath.Join(t.TempDir(), "zerg-search-cache.json")
	writeSearchCacheFileAt(t, legacy, map[string]string{"k-legacy": "res-legacy"})
	missing := filepath.Join(t.TempDir(), "isolated-missing.json")
	setSearchCachePaths(t, missing, legacy)
	sumBefore := stateFileSHA256(t, legacy)

	LoadSearchCache()

	if got := searchCacheGet("k-legacy"); got != "" {
		t.Fatalf("显式覆盖路径不存在时不该退旧路径，实得 %q", got)
	}
	if n := searchCacheSize(); n != 0 {
		t.Fatalf("覆盖态下缓存应为空，实得 %d 条", n)
	}
	if got := stateFileSHA256(t, legacy); got != sumBefore {
		t.Fatalf("覆盖态下旧文件更不该被动：%s → %s", sumBefore, got)
	}
}

// TestSearchCachePath_DefaultDerivedFromStateDir_NotTmp
// 默认落点由 statepath 统一状态目录派生。
func TestSearchCachePath_DefaultDerivedFromStateDir_NotTmp(t *testing.T) {
	if searchCacheLegacyDefaultPath != "/tmp/zerg-search-cache.json" {
		t.Fatalf("旧路径字面量应保持 /tmp/zerg-search-cache.json，实得 %q", searchCacheLegacyDefaultPath)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setSearchCachePaths(t, "", filepath.Join(t.TempDir(), "legacy.json"))

	want := filepath.Join(stateDir, searchCacheFileName)
	if got := searchCacheWritePath(); got != want {
		t.Fatalf("写路径应由状态目录派生：want %s, got %s", want, got)
	}
	if got := searchCacheReadPath(); got != want {
		t.Fatalf("无新无旧时读路径应为派生路径：want %s, got %s", want, got)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #8 · §八 待拍 2 裁定）：删掉旧断言
	//   「不以 /tmp/ 开头」—— 它在 TMPDIR=/tmp（Linux runner）下恒假，是自伤而非判据；
	//   真命题已由上两条 `got != want`（写/读路径 = 状态目录派生值）判过 ⇒ 不留第三条。
}

// ───────── ③ bash 溢出目录（bash_v101.go，目录类）─────────

func setBashOverflowPaths(t *testing.T, override, legacy string) {
	t.Helper()
	oldOverride, oldLegacy := bashOverflowDirOverride, legacyBashOverflowDir
	bashOverflowDirOverride, legacyBashOverflowDir = override, legacy
	t.Cleanup(func() { bashOverflowDirOverride, legacyBashOverflowDir = oldOverride, oldLegacy })
}

// makeLegacyOverflowDir 造一个「旧版本留下的溢出目录」（含一个旧溢出文件）。
func makeLegacyOverflowDir(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "zerg-bash-overflow")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建旧溢出目录失败: %v", err)
	}
	f := filepath.Join(dir, "bash-overflow-OLD.log")
	if err := os.WriteFile(f, []byte("旧溢出原文\n"), 0o644); err != nil {
		t.Fatalf("写旧溢出文件失败: %v", err)
	}
	if err := os.Chtimes(f, zergPastTime, zergPastTime); err != nil {
		t.Fatalf("设旧溢出文件 mtime 失败: %v", err)
	}
	return dir, f
}

// TestBashOverflowDir_WriteOnlyToStateDir_LegacyUntouched（纯写落点）
// 溢出文件只落新（状态目录派生）路径；旧目录内容/mtime 不变、旧目录不被删。
func TestBashOverflowDir_WriteOnlyToStateDir_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录不存在——验证首次自动建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir, legacyFile := makeLegacyOverflowDir(t)
	setBashOverflowPaths(t, "", legacyDir)
	sumBefore, mtimeBefore := stateFileSHA256(t, legacyFile), stateFileMtime(t, legacyFile)
	namesBefore := dirFileNames(t, legacyDir)

	path := spillBashOutput([]byte("迁移探针正文"), "migrate")

	wantDir := filepath.Join(stateDir, bashOverflowDirName)
	if got := filepath.Dir(path); got != wantDir {
		t.Fatalf("溢出文件必须落新目录 %s，实得 %s", wantDir, got)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "迁移探针正文" {
		t.Fatalf("溢出文件应可读且内容一致: %v", err)
	}
	if got := stateFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("旧溢出文件内容被改了！%s → %s", sumBefore, got)
	}
	if got := stateFileMtime(t, legacyFile); !got.Equal(mtimeBefore) {
		t.Fatalf("旧溢出文件被改写了！mtime %v → %v", mtimeBefore, got)
	}
	if got := dirFileNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("旧溢出目录被写了（必须只读、不删不改）：%v → %v", namesBefore, got)
	}
}

// TestBashOverflowDir_LegacyKeptReadable_NotWritten
// 目录类的迁移兼容 = 旧目录保留为**只读访问许可**（老溢出文件仍可 read 续读）+ 永不写入旧目录。
func TestBashOverflowDir_LegacyKeptReadable_NotWritten(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir, legacyFile := makeLegacyOverflowDir(t)
	setBashOverflowPaths(t, "", legacyDir)
	namesBefore := dirFileNames(t, legacyDir)

	dirs := bashOverflowAllowDirs()
	wantDir := filepath.Join(stateDir, bashOverflowDirName)
	if len(dirs) == 0 || dirs[0] != wantDir {
		t.Fatalf("白名单第一个必须是写落点 %s，实得 %v", wantDir, dirs)
	}
	if len(dirs) != 2 || dirs[1] != legacyDir {
		t.Fatalf("旧溢出目录存在 ⇒ 应保留为只读入口 %s，实得 %v", legacyDir, dirs)
	}

	ec := NewExecContext(t.TempDir())
	for _, want := range []string{wantDir, legacyDir} {
		found := false
		for _, d := range ec.ExtraAllowDirs {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("NewExecContext 白名单应含 %s，实得 %v", want, ec.ExtraAllowDirs)
		}
	}
	// 旧溢出文件仍可被 read 工具读（迁移兼容真实可用）
	if got, err := ec.validatePath(legacyFile); err != nil || got != legacyFile {
		t.Fatalf("旧溢出文件应仍可读: got %q err %v", got, err)
	}
	// 新落点写一次后也能读（真实续读闭环）
	p := spillBashOutput([]byte("new-spill"), "readable")
	if got, err := ec.validatePath(p); err != nil || got != p {
		t.Fatalf("新溢出文件应可读: got %q err %v", got, err)
	}
	if got := dirFileNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("旧溢出目录被写了：%v → %v", namesBefore, got)
	}
}

// TestBashOverflowDir_NeitherExists_NewDirAutoCreated
// 两都没有 ⇒ 空启动不报错；首次使用时新目录（含父目录）自动创建；旧目录路径不被创建。
func TestBashOverflowDir_NeitherExists_NewDirAutoCreated(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir := filepath.Join(t.TempDir(), "legacy-overflow") // 不存在
	setBashOverflowPaths(t, "", legacyDir)

	wantDir := filepath.Join(stateDir, bashOverflowDirName)
	if dirs := bashOverflowAllowDirs(); len(dirs) != 1 || dirs[0] != wantDir {
		t.Fatalf("无旧目录时白名单只应有写落点 %s，实得 %v", wantDir, dirs)
	}
	if got := bashSpillDir(); got != wantDir {
		t.Fatalf("溢出目录应派生自状态目录：want %s, got %s", wantDir, got)
	}
	if st, err := os.Stat(wantDir); err != nil || !st.IsDir() {
		t.Fatalf("溢出目录应首次自动创建: %v", err)
	}
	mustNotExist(t, legacyDir, "无旧目录时不得凭空创建旧 /tmp 目录")
}

// TestBashOverflowDir_ExplicitOverrideSkipsLegacy
// 显式覆盖 ⇒ 写/白名单都只用覆盖路径，不复旧目录（测试隔离口径）。
func TestBashOverflowDir_ExplicitOverrideSkipsLegacy(t *testing.T) {
	override := t.TempDir()
	legacyDir, legacyFile := makeLegacyOverflowDir(t)
	setBashOverflowPaths(t, override, legacyDir)
	sumBefore := stateFileSHA256(t, legacyFile)
	namesBefore := dirFileNames(t, legacyDir)

	if got := BashOverflowDir(); got != override {
		t.Fatalf("显式覆盖应生效：want %s, got %s", override, got)
	}
	if dirs := bashOverflowAllowDirs(); len(dirs) != 1 || dirs[0] != override {
		t.Fatalf("覆盖态白名单只应有覆盖路径，实得 %v", dirs)
	}
	p := spillBashOutput([]byte("override"), "ovr")
	if got := filepath.Dir(p); got != override {
		t.Fatalf("覆盖态溢出应落 %s，实得 %s", override, got)
	}
	if got := stateFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("覆盖态旧溢出文件不该被动：%s → %s", sumBefore, got)
	}
	if got := dirFileNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("覆盖态旧溢出目录被写了：%v → %v", namesBefore, got)
	}
}

// TestBashOverflowDir_DefaultDerivedFromStateDir_NotTmp
// 默认目录由 statepath 统一状态目录派生；旧字面量只在 legacy 常量上保留。
func TestBashOverflowDir_DefaultDerivedFromStateDir_NotTmp(t *testing.T) {
	if bashOverflowLegacyDefaultDir != "/tmp/zerg-bash-overflow" {
		t.Fatalf("旧目录字面量应保持 /tmp/zerg-bash-overflow（只读来源），实得 %q", bashOverflowLegacyDefaultDir)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setBashOverflowPaths(t, "", filepath.Join(t.TempDir(), "legacy-overflow-missing"))

	want := filepath.Join(stateDir, bashOverflowDirName)
	if got := BashOverflowDir(); got != want {
		t.Fatalf("目录应由状态目录派生：want %s, got %s", want, got)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #9 · §八 待拍 2 裁定）：删掉旧断言
	//   「不以 /tmp/ 开头」—— 它在 TMPDIR=/tmp（Linux runner）下恒假，是自伤而非判据；
	//   真命题已由上一条 `got != want`（= 状态目录派生值）判过 ⇒ 不留第二条。
	// exec.go 白名单接线（NewExecContext 每次都拿派生目录）
	ec := NewExecContext(t.TempDir())
	if len(ec.ExtraAllowDirs) == 0 || ec.ExtraAllowDirs[len(ec.ExtraAllowDirs)-1] != want {
		t.Fatalf("NewExecContext 白名单末尾应为派生溢出目录 %s，实得 %v", want, ec.ExtraAllowDirs)
	}
}
