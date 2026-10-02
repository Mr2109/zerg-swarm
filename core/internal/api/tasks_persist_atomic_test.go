package api

// tasks_persist_atomic_test.go — 任务落盘的原子性与失败可观测性（GAP-20261003-13 / -14 修复的判据件）。
//
// 修复前的两处病（本件逐条钉死）:
//  ① saveTasksLocked **无返回值** —— marshal / MkdirAll / WriteFile 三种失败全被静默吞掉
//     （只 log 或直接 return），调用方完全不知落盘成败：内存已改、盘上仍旧值（静默分叉）。
//     ⇒ 本件第一条用例在修复前**根本写不出来**（函数没有可接收的返回值），这是「缺口存在」的最强形态。
//  ② 落盘**非原子** —— 原先 os.WriteFile 直写目标路径，无 temp / 无 fsync / 无 rename / 无目录 fsync；
//     崩溃或断电会在同一 inode 上留下半文件或 0 字节，而读侧 Unmarshal 失败又静默 return
//     ⇒ 整份 history + queue 无声全丢。
//
// 修后契约（本件逐条断言）:
//  A. 落盘成功 ⇒ 返回 nil，且目标件是**完整可解析**的 persistedTasks；
//  B. 落盘失败 ⇒ 返回**非 nil error**（不再静默），且**不留临时件**、**不产生半个目标件**；
//  C. 覆盖写 ⇒ 目标件原子替换（先有旧内容、后写新内容 ⇒ 读到的一定是新的完整内容）；
//  D. 读侧遇损坏件 ⇒ 不 panic、不恢复任何任务，且**损坏件原样保留**（sha 不变，未被覆盖）。
//
// 隔离纪律：一律 setTasksPaths 切包级变量（t.Cleanup 严格恢复）+ 路径全部落在 t.TempDir()，
// 绝不碰真机 ~/.zerg/state 或旧 /tmp/zerg-tasks.json。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// atomicTestPaths 把主/旧路径都切到临时目录，返回（目标件路径，目录）。
func atomicTestPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "zerg-tasks.json")
	setTasksPaths(t, dst, filepath.Join(dir, "legacy-never-used.json"))
	return dst, dir
}

// tmpLeftovers 目录里残留的原子写临时件（正常路径下必须恒为 0 个）。
func tmpLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败 %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tasks-") && strings.HasSuffix(e.Name(), ".tmp") {
			out = append(out, e.Name())
		}
	}
	return out
}

// A + C：落盘成功 ⇒ 返回 nil、目标件完整可解析、无临时件残留。
func TestSaveTasksLocked_SuccessIsAtomicAndParseable(t *testing.T) {
	dst, dir := atomicTestPaths(t)

	hist := map[string]*Task{"t-ok": {ID: "t-ok", Status: "done", Model: "m", CreatedAt: time.Now()}}
	if err := saveTasksLocked(TaskQueue{}, map[string]*Task{}, hist, map[string]*Task{}); err != nil {
		t.Fatalf("A: 正常落盘必须返回 nil，实得 %v", err)
	}
	if got := tmpLeftovers(t, dir); len(got) != 0 {
		t.Fatalf("A: 落盘成功却留下临时件 %v", got)
	}
	buf, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("A: 目标件应存在: %v", err)
	}
	var back persistedTasks
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatalf("A: 目标件必须是完整可解析的 persistedTasks（半文件即此处红）: %v", err)
	}
	if _, ok := back.History["t-ok"]; !ok {
		t.Fatalf("A: 历史未落盘，实得 %d 条", len(back.History))
	}
}

// A + C：覆盖写 ⇒ 原子替换，读到的一定是新的完整内容（不是新旧混合）。
func TestSaveTasksLocked_OverwriteReplacesWholeFile(t *testing.T) {
	dst, dir := atomicTestPaths(t)

	first := map[string]*Task{"t-old": {ID: "t-old", Status: "done", Model: "m", CreatedAt: time.Now()}}
	if err := saveTasksLocked(TaskQueue{}, map[string]*Task{}, first, map[string]*Task{}); err != nil {
		t.Fatalf("前置落盘失败: %v", err)
	}
	second := map[string]*Task{"t-new": {ID: "t-new", Status: "failed", Model: "m", CreatedAt: time.Now()}}
	if err := saveTasksLocked(TaskQueue{}, map[string]*Task{}, second, map[string]*Task{}); err != nil {
		t.Fatalf("覆盖落盘失败: %v", err)
	}
	buf, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读目标件失败: %v", err)
	}
	var back persistedTasks
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatalf("覆盖后目标件必须完整可解析: %v", err)
	}
	if _, ok := back.History["t-new"]; !ok {
		t.Fatalf("覆盖后应含新历史 t-new，实得 %v", back.History)
	}
	if _, ok := back.History["t-old"]; ok {
		t.Fatalf("覆盖后不许残留旧历史 t-old（整份替换，不是合并）")
	}
	if got := tmpLeftovers(t, dir); len(got) != 0 {
		t.Fatalf("覆盖写留下临时件 %v", got)
	}
}

// B：落盘失败 ⇒ 返回非 nil error（不再静默），且不留临时件、不产生半个目标件。
// 造法：把目标件的父目录路径位置上摆一个**普通文件** ⇒ MkdirAll 必然失败（ENOTDIR）。
func TestSaveTasksLocked_UnwritableDirReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("造阻塞件失败: %v", err)
	}
	dst := filepath.Join(blocker, "sub", "zerg-tasks.json") // blocker 是文件 ⇒ 建目录必失败
	setTasksPaths(t, dst, filepath.Join(dir, "legacy-never-used.json"))

	err := saveTasksLocked(TaskQueue{}, map[string]*Task{},
		map[string]*Task{"t-x": {ID: "t-x", Status: "done", Model: "m", CreatedAt: time.Now()}},
		map[string]*Task{})
	if err == nil {
		t.Fatalf("B: 落盘失败必须返回非 nil error（原先静默吞掉 —— 本断言在修复前无法编译）")
	}
	if !strings.Contains(err.Error(), "任务持久化") {
		t.Fatalf("B: 错误须带「任务持久化」前缀便于定位，实得: %v", err)
	}
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Fatalf("B: 落盘失败却产生了目标件 %s", dst)
	}
	if got := tmpLeftovers(t, dir); len(got) != 0 {
		t.Fatalf("B: 落盘失败留下临时件 %v（失败路径必须自清）", got)
	}
}

// D：读侧遇损坏件 ⇒ 不 panic、不恢复任何任务，且损坏件原样保留（sha 不变）。
func TestLoadPersistedHistory_CorruptFileKeepsFileAndRecoversNothing(t *testing.T) {
	dst, _ := atomicTestPaths(t)
	garbage := []byte(`{"saved_at":"2026-10-03T00:00:00Z","queue":[{"id":"t-cut`)
	if err := os.WriteFile(dst, garbage, 0o644); err != nil {
		t.Fatalf("造损坏件失败: %v", err)
	}
	before := fileSHA256(t, dst)

	s := &MasterScheduler{
		queue:   TaskQueue{},
		running: make(map[string]*Task),
		history: make(map[string]*Task),
		waiting: make(map[string]*Task),
	}
	s.loadPersistedHistory() // 不许 panic

	if got := historyIDs(s); len(got) != 0 {
		t.Fatalf("D: 损坏件不许恢复任何任务，实得 %v", got)
	}
	if s.queue.Len() != 0 {
		t.Fatalf("D: 损坏件不许入队，实得队列 %d", s.queue.Len())
	}
	if after := fileSHA256(t, dst); after != before {
		t.Fatalf("D: 损坏件必须原样保留待查（sha %s → %s）", before, after)
	}
}
