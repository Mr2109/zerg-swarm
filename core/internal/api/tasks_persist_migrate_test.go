package api

// tasks_persist_migrate_test.go — 任务持久化路径统一（2026-09-18 修 /tmp 硬编码缺陷）用例。
//
// 缺陷: tasks_persist.go 原写死 `var tasksFile = "/tmp/zerg-tasks.json"` ——
// 多实例共享同一份文件、互相覆盖任务历史（沙箱金丝雀实测把它回写了；生产也读同一份）。
//
// 修后契约（本文件逐条钉死）:
//  ① 写只写统一状态目录（statepath.File → ZERG_STATE_DIR → ~/.zerg/state/zerg-tasks.json）；
//  ② 新路径存在 ⇒ 旧 /tmp 路径完全不看（内容不夹除）；
//  ③ 新路径无、旧 /tmp 有 ⇒ 读旧一次（不丢历史），旧文件不删、不改（sha256 + mtime 双证）；
//  ④ 两都没有 ⇒ 空历史、不报错、不建文件；
//  ⑤ 显式覆盖（测试隔离）时不退旧路径（防测试读真机 /tmp）。
//
// 隔离纪律: 一律用 setTasksPaths 切包级变量（t.Cleanup 严格恢复）+ ZERG_STATE_DIR 指 temp，
// 绝不让用例碰真机 /tmp/zerg-tasks.json 或 ~/.zerg/state/zerg-tasks.json。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pastTime 旧文件的固定 mtime（2020-01-01T00:00:00Z）。
// 用固定过去时间而不是「现在」——若代码改写了旧文件，mtime 必然变（纳秒粒度，不会漏判）。
var pastTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// setTasksPaths 切包级持久化路径（新/旧），用例结束严格恢复。
func setTasksPaths(t *testing.T, primary, legacy string) {
	t.Helper()
	oldPrimary, oldLegacy := tasksFile, legacyTasksFile
	tasksFile, legacyTasksFile = primary, legacy
	t.Cleanup(func() {
		tasksFile, legacyTasksFile = oldPrimary, oldLegacy
	})
}

// writePersistedFile 造一份 persistedTasks 落盘文件（history 里放给定 id，状态 done）。
func writePersistedFile(t *testing.T, path string, ids ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("造测试文件失败（建目录）: %v", err)
	}
	data := persistedTasks{
		SavedAt: time.Now(),
		History: make(map[string]*Task),
		Queue:   []*Task{},
		Running: make(map[string]*Task),
		Waiting: make(map[string]*Task),
	}
	for _, id := range ids {
		data.History[id] = &Task{ID: id, Description: "落盘用例 " + id, Status: "done", Model: "test-model", CreatedAt: time.Now()}
	}
	buf, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("写测试文件失败 %s: %v", path, err)
	}
	if err := os.Chtimes(path, pastTime, pastTime); err != nil {
		t.Fatalf("设旧文件 mtime 失败 %s: %v", path, err)
	}
}

// fileSHA256 文件内容哈希（迁移不得改旧文件的硬证据）。
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件失败 %s: %v", path, err)
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// fileMtime 文件 mtime（迁移不得改旧文件的第二重证据）。
func fileMtime(t *testing.T, path string) time.Time {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败 %s: %v", path, err)
	}
	return st.ModTime()
}

// historyIDs 调度器历史快照（构造期无线程——用例单线程读）。
func historyIDs(s *MasterScheduler) []string {
	ids := make([]string, 0, len(s.history))
	for id := range s.history {
		ids = append(ids, id)
	}
	return ids
}

// ① 新路径存在 ⇒ 只读新（旧路径有不同内容也不夹除、旧路径完全不看）。
func TestTasksPersist_ReadPrefersStateFile_LegacyIgnored(t *testing.T) {
	tmp := t.TempDir()
	newPath := filepath.Join(tmp, "state", "zerg-tasks.json")
	legacy := filepath.Join(tmp, "legacy-tasks.json")
	writePersistedFile(t, newPath, "new-1")   // 新文件: 只有 new-1
	writePersistedFile(t, legacy, "legacy-1") // 旧文件: 只有 legacy-1（不同内容）
	setTasksPaths(t, newPath, legacy)

	s := NewMasterScheduler("", 1)

	if _, ok := s.history["new-1"]; !ok {
		t.Fatalf("应从新路径读到 new-1；实得历史 %v", historyIDs(s))
	}
	if _, ok := s.history["legacy-1"]; ok {
		t.Fatalf("新路径已存在时旧路径必须完全不看——legacy-1 不该被夹进来；实得历史 %v", historyIDs(s))
	}
	if got := tasksReadPath(); got != newPath {
		t.Fatalf("读路径应为新路径 %s，实得 %s", newPath, got)
	}
}

// ② 新路径无、旧路径有 ⇒ 读到旧内容；旧文件未被删、未被改（sha256 + mtime 双证）。
func TestTasksPersist_FirstRunReadsLegacyOnce_KeepsLegacyIntact(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir) // 新路径派生自统一状态目录（此目录下此刻还没有文件）
	legacy := filepath.Join(t.TempDir(), "zerg-tasks.json")
	writePersistedFile(t, legacy, "legacy-1", "legacy-2")
	setTasksPaths(t, "", legacy)

	sumBefore, mtimeBefore := fileSHA256(t, legacy), fileMtime(t, legacy)
	newPath := filepath.Join(stateDir, "zerg-tasks.json")

	s := NewMasterScheduler("", 1)

	for _, id := range []string{"legacy-1", "legacy-2"} {
		if _, ok := s.history[id]; !ok {
			t.Fatalf("新路径缺失 + 旧路径存在 ⇒ 必须读旧一次（不丢历史），缺 %s；实得 %v", id, historyIDs(s))
		}
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

// ③ 两都没有 ⇒ 空历史、不报错、不建文件（首次运行路径）。
func TestTasksPersist_NeitherExists_EmptyHistoryNoError(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "not-exist.json") // 旧文件不存在
	setTasksPaths(t, "", legacy)

	s := NewMasterScheduler("", 1)

	if len(s.history) != 0 {
		t.Fatalf("无任何持久化文件时应为空历史，实得 %v", historyIDs(s))
	}
	if s.queue.Len() != 0 {
		t.Fatalf("无任何持久化文件时队列应为空，实得 %d", s.queue.Len())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "zerg-tasks.json")); err == nil {
		t.Fatalf("首次运行只读不建——状态目录下不该凭空出现 zerg-tasks.json")
	}
	if got, want := tasksReadPath(), tasksWritePath(); got != want {
		t.Fatalf("两都没有时读路径应等于写路径（新路径）：%s vs %s", got, want)
	}
}

// ④ 写入只写新路径：新文件含数据、目录自动创建，旧文件 mtime/内容不变。
func TestTasksPersist_WriteOnlyToStateFile_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录尚不存在——验证自动创建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-tasks.json")
	writePersistedFile(t, legacy, "legacy-1")
	setTasksPaths(t, "", legacy)

	sumBefore, mtimeBefore := fileSHA256(t, legacy), fileMtime(t, legacy)

	saveTasksLocked(TaskQueue{}, map[string]*Task{}, map[string]*Task{
		"h1": {ID: "h1", Description: "写新路径", Status: "done", Model: "test-model", CreatedAt: time.Now()},
	})

	newPath := filepath.Join(stateDir, "zerg-tasks.json")
	buf, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("写必须落到新路径 %s: %v", newPath, err)
	}
	var data persistedTasks
	if err := json.Unmarshal(buf, &data); err != nil {
		t.Fatalf("新文件不是合法 persistedTasks JSON: %v", err)
	}
	if _, ok := data.History["h1"]; !ok {
		t.Fatalf("新文件应含刚保存的 h1，实得 %v", data.History)
	}
	if sumAfter := fileSHA256(t, legacy); sumAfter != sumBefore {
		t.Fatalf("旧文件内容被写回了！sha256 %s → %s", sumBefore, sumAfter)
	}
	if mtimeAfter := fileMtime(t, legacy); !mtimeAfter.Equal(mtimeBefore) {
		t.Fatalf("旧文件被写回了！mtime %v → %v", mtimeBefore, mtimeAfter)
	}
	if _, ok := data.History["legacy-1"]; ok {
		t.Fatalf("新文件不该夹带旧文件的其它内容")
	}
}

// ⑤ 默认路径由 statepath 统一状态目录派生（ZERG_STATE_DIR 可覆盖）；旧字面量只在 legacy 变量上。
func TestTasksPersist_DefaultPathDerivedFromStateDir(t *testing.T) {
	// 旧路径字面量必须原样保留（迁移兼容的只读来源；测试进程隔离会另切变量，故查常量）
	if tasksLegacyDefaultPath != "/tmp/zerg-tasks.json" {
		t.Fatalf("旧路径字面量应保持 /tmp/zerg-tasks.json（只读来源），实得 %q", tasksLegacyDefaultPath)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setTasksPaths(t, "", filepath.Join(t.TempDir(), "legacy.json")) // 包级覆盖清空 ⇒ 走派生

	want := filepath.Join(stateDir, "zerg-tasks.json")
	if got := tasksWritePath(); got != want {
		t.Fatalf("写路径应由状态目录派生：want %s, got %s", want, got)
	}
	if got := tasksWritePath(); strings.HasPrefix(got, "/tmp/") {
		t.Fatalf("默认落点不该再是 /tmp：%s", got)
	}
	if name := filepath.Base(tasksWritePath()); name != tasksStateFileName {
		t.Fatalf("状态目录下文件名应为 %s，实得 %s", tasksStateFileName, name)
	}
}

// ⑥ 显式覆盖主路径（测试隔离口径）时不退旧路径——防测试读真机 /tmp。
func TestTasksPersist_ExplicitOverrideSkipsLegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	legacy := filepath.Join(tmp, "legacy.json")
	writePersistedFile(t, legacy, "legacy-1")
	missing := filepath.Join(tmp, "isolated-missing.json") // 覆盖路径不存在
	setTasksPaths(t, missing, legacy)

	s := NewMasterScheduler("", 1)

	if len(s.history) != 0 {
		t.Fatalf("显式覆盖路径不存在时不该退旧路径读真机历史；实得 %v", historyIDs(s))
	}
}

// ⑦ 端到端不丢历史：读旧 → 写新 ⇒ 新文件里有旧历史，旧文件依旧原封不动。
func TestTasksPersist_MigrationReadThenWrite_HistoryNotLost(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacy := filepath.Join(t.TempDir(), "zerg-tasks.json")
	writePersistedFile(t, legacy, "legacy-1")
	setTasksPaths(t, "", legacy)

	sumBefore := fileSHA256(t, legacy)
	s := NewMasterScheduler("", 1)
	saveTasksLocked(s.queue, s.running, s.history, s.waiting)

	buf, err := os.ReadFile(filepath.Join(stateDir, "zerg-tasks.json"))
	if err != nil {
		t.Fatalf("读旧一次后首次写盘应落新路径: %v", err)
	}
	var data persistedTasks
	if err := json.Unmarshal(buf, &data); err != nil {
		t.Fatalf("新文件非法 JSON: %v", err)
	}
	if _, ok := data.History["legacy-1"]; !ok {
		t.Fatalf("旧历史必须随首次写盘落到新文件（不丢）；实得 %v", data.History)
	}
	if sumAfter := fileSHA256(t, legacy); sumAfter != sumBefore {
		t.Fatalf("旧文件被改写：%s → %s", sumBefore, sumAfter)
	}
}
