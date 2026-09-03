package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// helper — 创建临时 issue 文件
func createIssue(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-issue.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("创建 issue 文件失败: %v", err)
	}
	return path
}

// test 1: code 类型 — 有测试文件 → 通过
func TestVerify_Code_WithTestFiles(t *testing.T) {
	dir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(dir, "example_test.go")
	if err := os.WriteFile(testFile, []byte("package example\n\nfunc TestExample(t *testing.T) {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	issueContent := `# Task: test-code

- **任务**: 写一个示例函数
- **类型**: code
- **优先级**: normal
- **来源**: api

## 执行记录
- 开始: 2026-01-01
- 完成: 2026-01-01
- 结论: 完成
`

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if !passed {
		t.Errorf("期望通过，但失败: %s", reason)
	}
}

// test 2: code 类型 — 无测试文件 → 不通过
func TestVerify_Code_WithoutTestFiles(t *testing.T) {
	dir := t.TempDir()

	// 只创建一个普通 Go 文件（非测试文件）
	goFile := filepath.Join(dir, "example.go")
	if err := os.WriteFile(goFile, []byte("package example\n\nfunc Example() {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	issueContent := `# Task: test-code

- **任务**: 写一个示例函数
- **类型**: code
- **优先级**: normal
- **来源**: api

## 执行记录
- 开始: 2026-01-01
- 完成: 2026-01-01
- 结论: 完成
`

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if passed {
		t.Errorf("期望不通过，但通过了: %s", reason)
	}
}

// test 3: report 类型 — 报告文件存在且非空 → 通过
func TestVerify_Report_FileExists(t *testing.T) {
	dir := t.TempDir()

	// 创建报告文件
	reportFile := filepath.Join(dir, "report.md")
	if err := os.WriteFile(reportFile, []byte("# Report\n\n这是一份测试报告。"), 0o644); err != nil {
		t.Fatal(err)
	}

	issueContent := fmt.Sprintf(`# Task: test-report

- **任务**: 写报告 %s
- **类型**: report
- **优先级**: normal
- **来源**: api

## 执行记录
- 开始: 2026-01-01
- 完成: 2026-01-01
- 结论: 完成
`, reportFile)

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if !passed {
		t.Errorf("期望通过，但失败: %s", reason)
	}
}

// test 4: fix 类型 — 有修复描述 → 通过
func TestVerify_Fix_WithDescription(t *testing.T) {
	dir := t.TempDir()

	issueContent := `# Task: test-fix

- **任务**: 修复内存泄漏
- **类型**: fix
- **优先级**: high
- **来源**: cron

## 修复
修复了 buffer 释放逻辑，在 defer 中添加了 Close() 调用。
`

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if !passed {
		t.Errorf("期望通过，但失败: %s", reason)
	}
}

// test 5: fix 类型 — 无修复描述 → 不通过
func TestVerify_Fix_WithoutDescription(t *testing.T) {
	dir := t.TempDir()

	issueContent := `# Task: test-fix

- **任务**: 修复内存泄漏
- **类型**: fix
- **优先级**: high
- **来源**: cron

## 执行记录
- 开始: 2026-01-01
- 完成: 2026-01-01
- 结论: 完成
`

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if passed {
		t.Errorf("期望不通过，但通过了: %s", reason)
	}
}

// test 6: 读取不存在的 issue 文件 → 不通过
func TestVerify_FileNotFound(t *testing.T) {
	passed, reason := Verify("/nonexistent/issue.md", t.TempDir())

	if passed {
		t.Errorf("期望不通过，但通过了: %s", reason)
	}
}

// test 7: research 类型 — 报告文件存在 → 通过
func TestVerify_Research_FileExists(t *testing.T) {
	dir := t.TempDir()

	// 创建报告文件
	researchFile := filepath.Join(dir, "research.md")
	if err := os.WriteFile(researchFile, []byte("# 调研报告\n\n调研了三种方案。"), 0o644); err != nil {
		t.Fatal(err)
	}

	issueContent := fmt.Sprintf(`# Task: test-research

- **任务**: 调研技术方案——写报告 %s
- **类型**: research
- **优先级**: normal
- **来源**: api

## 执行记录
- 开始: 2026-01-01
- 完成: 2026-01-01
- 结论: 完成
`, researchFile)

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if !passed {
		t.Errorf("期望通过，但失败: %s", reason)
	}
}

// test 8: 通用验收 — 从 issue 中提取文件路径并检查
func TestVerify_GeneralTarget_ExtractedPath(t *testing.T) {
	dir := t.TempDir()

	// 创建目标文件
	targetFile := filepath.Join(dir, "output.md")
	if err := os.WriteFile(targetFile, []byte("# 输出文件\n\n这是产出。"), 0o644); err != nil {
		t.Fatal(err)
	}

	issueContent := `# Task: test-general

- **任务**: 输出报告到 ` + filepath.Join(dir, "output.md") + `
- **类型**: report
- **优先级**: normal
- **来源**: api
`

	issuePath := createIssue(t, issueContent)
	passed, reason := Verify(issuePath, dir)

	if !passed {
		t.Errorf("期望通过，但失败: %s", reason)
	}
}
