package agentstate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewState_AddTodo 创建状态 + 添加 todo
func TestNewState_AddTodo(t *testing.T) {
	s := NewState("验证 v2.4 三个关键项", "codex", nil)
	todo := s.AddTodo("测试排除本机", "P0", "shell")
	if todo.Status != TodoOpen {
		t.Fatalf("期望 open，得到 %s", todo.Status)
	}
	if len(s.Todos) != 1 {
		t.Fatalf("期望 1 个 todo，得到 %d", len(s.Todos))
	}
}

// TestSetTodoStatus 状态流转 open→in_progress→completed
func TestSetTodoStatus(t *testing.T) {
	s := NewState("测试", "codex", nil)
	todo := s.AddTodo("任务", "P1", "file")
	if err := s.SetTodoStatus(todo.ID, TodoInProgress); err != nil {
		t.Fatalf("in_progress 失败: %v", err)
	}
	if err := s.SetTodoStatus(todo.ID, TodoCompleted); err != nil {
		t.Fatalf("completed 失败: %v", err)
	}
	if s.Todos[0].Status != TodoCompleted {
		t.Fatalf("期望 completed，得到 %s", s.Todos[0].Status)
	}
	// 不存在的 id
	if err := s.SetTodoStatus("todo_nonexist", TodoCompleted); err == nil {
		t.Fatal("不存在的 todo 应报错")
	}
}

// TestSaveLoad 保存 + 加载（断连恢复核心）
func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "STATE.json")

	s := NewState("断连恢复测试", "codex", nil)
	todo := s.AddTodo("任务A", "P0", "shell")
	s.SetTodoStatus(todo.ID, TodoInProgress)
	s.AddEvidence("改了 config.go", "go build 通过", "无", "测试任务B")
	if err := s.Save(path); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	// 重新加载（模拟断连恢复）
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if loaded.Objective != "断连恢复测试" {
		t.Fatalf("objective 不一致: %s", loaded.Objective)
	}
	if loaded.Todos[0].Status != TodoInProgress {
		t.Fatalf("todo 状态丢失: %s", loaded.Todos[0].Status)
	}
	if len(loaded.Evidence) != 1 {
		t.Fatalf("evidence 丢失")
	}
}

// TestNextTodo 优先级选择（P0 优先）
func TestNextTodo(t *testing.T) {
	s := NewState("测试", "codex", nil)
	p1 := s.AddTodo("P1任务", "P1", "shell")
	p0 := s.AddTodo("P0任务", "P0", "shell")
	_ = p1
	next := s.NextTodo()
	if next.ID != p0.ID {
		t.Fatalf("期望 P0 优先，得到 %s", next.ID)
	}
}

// TestSpendQuota 配额消耗（超限拦截）
func TestSpendQuota(t *testing.T) {
	s := NewState("测试", "codex", nil)
	s.Quota.MaxSlots = 2
	if !s.SpendQuota() {
		t.Fatal("第 1 次消耗应允许")
	}
	if !s.SpendQuota() {
		t.Fatal("第 2 次消耗应允许")
	}
	if s.SpendQuota() {
		t.Fatal("第 3 次消耗应拦截（超限）")
	}
}

// TestAllCompleted 全部完成判断
func TestAllCompleted(t *testing.T) {
	s := NewState("测试", "codex", nil)
	a := s.AddTodo("A", "P0", "shell")
	b := s.AddTodo("B", "P1", "shell")
	if s.AllCompleted() {
		t.Fatal("未完成不应判定全部完成")
	}
	s.SetTodoStatus(a.ID, TodoCompleted)
	s.SetTodoStatus(b.ID, TodoCompleted)
	if !s.AllCompleted() {
		t.Fatal("全部完成应判定 true")
	}
}

// TestLoad_NotExist 文件不存在返回 nil（新任务）
func TestLoad_NotExist(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "NOPE.json"))
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if loaded != nil {
		t.Fatal("不存在应返回 nil")
	}
}

// TestSave_Atomic 原子写（临时文件 + rename）
func TestSave_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "STATE.json")
	s := NewState("原子写测试", "codex", nil)
	if err := s.Save(path); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("文件不存在: %v", err)
	}
	// 不应有 .tmp 残留
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal(".tmp 残留")
	}
}
