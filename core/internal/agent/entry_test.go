package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- 1. SubmitTask 正常生成 issue ---

func TestSubmitTask_ValidCodeTask(t *testing.T) {
	tmpDir := t.TempDir()
	// 临时切换工作目录
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec := TaskSpec{
		Task:     "fix null pointer in worker.go",
		Type:     TaskTypeCode,
		Priority: TaskPriorityHigh,
		Repo:     "core",
		Source:   SourceGit,
	}

	path, err := SubmitTask(spec)
	if err != nil {
		t.Fatalf("SubmitTask returned error: %v", err)
	}
	if path == "" {
		t.Fatal("SubmitTask returned empty path")
	}
	if !strings.HasSuffix(path, ".md") {
		t.Errorf("path should end with .md, got %s", path)
	}

	// 验证文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	content := string(data)

	// 验证字段写入
	if !strings.Contains(content, "fix null pointer in worker.go") {
		t.Error("task description not in file")
	}
	if !strings.Contains(content, "code") {
		t.Error("type 'code' not in file")
	}
	if !strings.Contains(content, "high") {
		t.Error("priority 'high' not in file")
	}
	if !strings.Contains(content, "git") {
		t.Error("source 'git' not in file")
	}
	if !strings.Contains(content, "core") {
		t.Error("repo 'core' not in file")
	}
	if !strings.Contains(content, "task-hash:") {
		t.Error("task-hash not in file")
	}
}

// --- 2. 优先级写入验证 ---

func TestSubmitTask_PrioritiesWritten(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	for _, prio := range []string{TaskPriorityHigh, TaskPriorityNormal, TaskPriorityLow} {
		spec := TaskSpec{
			Task:     "test priority " + prio,
			Type:     TaskTypeReport,
			Priority: prio,
			Source:   SourceCron,
		}
		path, err := SubmitTask(spec)
		if err != nil {
			t.Fatalf("SubmitTask(%s) error: %v", prio, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(data), prio) {
			t.Errorf("priority %q not found in issue file", prio)
		}
	}
}

// --- 3. 去重测试 ---

func TestSubmitTask_Dedup(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec := TaskSpec{
		Task:     "identical task for dedup",
		Type:     TaskTypeFix,
		Priority: TaskPriorityNormal,
		Source:   SourceAPI,
	}

	path1, err := SubmitTask(spec)
	if err != nil {
		t.Fatalf("first SubmitTask error: %v", err)
	}

	path2, err := SubmitTask(spec)
	if err != nil {
		t.Fatalf("second SubmitTask error: %v", err)
	}

	if path1 != path2 {
		t.Errorf("dedup failed: got %s and %s", path1, path2)
	}
}

// --- 4. 非法 type ---

func TestSubmitTask_InvalidType(t *testing.T) {
	spec := TaskSpec{
		Task:     "some task",
		Type:     "invalid_type",
		Priority: TaskPriorityNormal,
		Source:   SourceGit,
	}
	_, err := SubmitTask(spec)
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid task type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- 5. 非法 priority ---

func TestSubmitTask_InvalidPriority(t *testing.T) {
	spec := TaskSpec{
		Task:     "some task",
		Type:     TaskTypeCode,
		Priority: "ultra_high",
		Source:   SourceGit,
	}
	_, err := SubmitTask(spec)
	if err == nil {
		t.Error("expected error for invalid priority, got nil")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- 6. 非法 source ---

func TestSubmitTask_InvalidSource(t *testing.T) {
	spec := TaskSpec{
		Task:     "some task",
		Type:     TaskTypeCode,
		Priority: TaskPriorityNormal,
		Source:   "invalid_source",
	}
	_, err := SubmitTask(spec)
	if err == nil {
		t.Error("expected error for invalid source, got nil")
	}
	if !strings.Contains(err.Error(), "invalid source") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- 7. 空任务描述 ---

func TestSubmitTask_EmptyTask(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec := TaskSpec{
		Task:     "",
		Type:     TaskTypeCode,
		Priority: TaskPriorityNormal,
		Source:   SourceGit,
	}
	_, err := SubmitTask(spec)
	if err == nil {
		t.Error("expected error for empty task, got nil")
	}
}

// --- 8. 不同任务不去重 ---

func TestSubmitTask_DifferentTasksNotDedup(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec1 := TaskSpec{
		Task:     "task alpha unique",
		Type:     TaskTypeCode,
		Priority: TaskPriorityHigh,
		Source:   SourceGit,
	}
	spec2 := TaskSpec{
		Task:     "task beta unique",
		Type:     TaskTypeReport,
		Priority: TaskPriorityLow,
		Source:   SourceA2A,
	}

	path1, err := SubmitTask(spec1)
	if err != nil {
		t.Fatalf("first SubmitTask error: %v", err)
	}
	path2, err := SubmitTask(spec2)
	if err != nil {
		t.Fatalf("second SubmitTask error: %v", err)
	}

	if path1 == path2 {
		t.Error("different tasks should not dedup")
	}

	// 确认两个文件都存在
	if _, err := os.Stat(path1); err != nil {
		t.Errorf("path1 should exist: %v", err)
	}
	if _, err := os.Stat(path2); err != nil {
		t.Errorf("path2 should exist: %v", err)
	}
}

// --- 9. 状态机校验：新 issue 状态为 queued ---

func TestSubmitTask_StatusQueued(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec := TaskSpec{
		Task:     "new task for state check",
		Type:     TaskTypeResearch,
		Priority: TaskPriorityNormal,
		Source:   SourceCron,
	}

	path, err := SubmitTask(spec)
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, StatusQueued) {
		t.Errorf("issue should have status %q, content:\n%s", StatusQueued, content)
	}
}

// --- 10. 四种来源都合法 ---

func TestSubmitTask_AllSources(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	for _, src := range []string{SourceGit, SourceCron, SourceAPI, SourceA2A} {
		spec := TaskSpec{
			Task:     "task from " + src,
			Type:     TaskTypeCode,
			Priority: TaskPriorityNormal,
			Source:   src,
		}
		path, err := SubmitTask(spec)
		if err != nil {
			t.Errorf("SubmitTask source=%s error: %v", src, err)
			continue
		}
		if path == "" {
			t.Errorf("SubmitTask source=%s returned empty path", src)
		}
		// 验证来源写入
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), src) {
			t.Errorf("source %q not in issue file", src)
		}
	}
}

// --- 11. Validate 直接测试 ---

func TestTaskSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    TaskSpec
		wantErr bool
	}{
		{"valid", TaskSpec{Task: "fix bug", Type: "code", Priority: "high", Repo: "r", Source: "git"}, false},
		{"empty task", TaskSpec{Type: "code", Priority: "high", Source: "git"}, true},
		{"bad type", TaskSpec{Task: "x", Type: "xxx", Priority: "high", Source: "git"}, true},
		{"bad priority", TaskSpec{Task: "x", Type: "code", Priority: "x", Source: "git"}, true},
		{"bad source", TaskSpec{Task: "x", Type: "code", Priority: "high", Source: "x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- 12. 状态流转：queued → running 合法 ---

func TestTransitionIssue_QueuedToRunning(t *testing.T) {
	content := "# Test\n- **状态**: queued\n"
	newContent, err := TransitionIssue(content, StatusQueued, StatusRunning)
	if err != nil {
		t.Fatalf("Transition queued→running error: %v", err)
	}
	if !strings.Contains(newContent, StatusRunning) {
		t.Error("status not updated to running")
	}
}

// --- 13. 状态流转：illegal transition 报错 ---

func TestTransitionIssue_IllegalTransition(t *testing.T) {
	content := "# Test\n- **状态**: done\n"
	_, err := TransitionIssue(content, StatusDone, StatusRunning)
	if err == nil {
		t.Error("expected error for illegal transition done→running")
	}
}

// --- 14. 文件在 docs/issues/ 目录下 ---

func TestSubmitTask_WriteToIssuesDir(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	spec := TaskSpec{
		Task:     "write to issues dir check",
		Type:     TaskTypeCode,
		Priority: TaskPriorityNormal,
		Source:   SourceGit,
	}
	path, err := SubmitTask(spec)
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}

	// 检查路径包含 docs/issues/
	relPath, _ := filepath.Rel(tmpDir, path)
	if !strings.Contains(relPath, "docs"+string(filepath.Separator)+"issues") {
		t.Errorf("file not in docs/issues/: got relative path %s", relPath)
	}
}
