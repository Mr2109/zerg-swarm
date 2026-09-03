package agent

// worker_test.go — v2.5.2 T3 worker 测试（我补——CA 未完成）
// 覆盖: 成功路径/失败路径/状态更新/文件缺失

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createWorkerIssue — 创建测试 issue（返回路径）
func createWorkerIssue(t *testing.T, workDir, name, status string) string {
	t.Helper()
	issuesDir := filepath.Join(workDir, "docs", "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "- **状态**: " + status + "\n- **任务**: test\n"
	path := filepath.Join(issuesDir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestWorker_Run_Success — 成功路径（mock runner 返回 0 → fixing→verified→done）
func TestWorker_Run_Success(t *testing.T) {
	workDir := t.TempDir()
	path := createWorkerIssue(t, workDir, "w-success", "open")

	w := NewWorker(workDir)
	w.SetRunner(func(issuePath string) int { return 0 }) // mock 成功

	exitCode, err := w.Run(path)
	if err != nil {
		t.Fatalf("Run 成功路径应无错误: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("期望退出码 0，实际 %d", exitCode)
	}

	// 验证状态为 done
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "- **状态**: done") {
		t.Fatalf("期望状态 done，实际:\n%s", string(data))
	}
}

// TestWorker_Run_Failure — 失败路径（mock runner 返回 1 → 状态保留/失败处理）
func TestWorker_Run_Failure(t *testing.T) {
	workDir := t.TempDir()
	path := createWorkerIssue(t, workDir, "w-fail", "open")

	w := NewWorker(workDir)
	w.SetRunner(func(issuePath string) int { return 1 }) // mock 失败

	exitCode, err := w.Run(path)
	if err != nil {
		t.Fatalf("Run 失败路径不应返回错误（错误留给上层）: %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("期望退出码 1，实际 %d", exitCode)
	}
}

// TestWorker_Run_FileMissing — 文件缺失（返回错误）
func TestWorker_Run_FileMissing(t *testing.T) {
	workDir := t.TempDir()
	w := NewWorker(workDir)

	_, err := w.Run(filepath.Join(workDir, "docs", "issues", "nonexist.md"))
	if err == nil {
		t.Fatal("文件缺失应返回错误")
	}
}

// TestWorker_Run_InvalidStatus — 无法解析状态（返回错误）
func TestWorker_Run_InvalidStatus(t *testing.T) {
	workDir := t.TempDir()
	issuesDir := filepath.Join(workDir, "docs", "issues")
	os.MkdirAll(issuesDir, 0o755)
	path := filepath.Join(issuesDir, "bad.md")
	os.WriteFile(path, []byte("无状态字段\n"), 0o644)

	w := NewWorker(workDir)
	_, err := w.Run(path)
	if err == nil {
		t.Fatal("无法解析状态应返回错误")
	}
}
