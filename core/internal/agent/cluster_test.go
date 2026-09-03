package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockIssueContent — 构造一个 mock issue markdown 内容
func mockIssueContent(id, status, errorType, task, taskHash string) string {
	var sb strings.Builder
	sb.WriteString("# Issue: " + id + "\n\n")
	sb.WriteString("<!-- task-hash: " + taskHash + " -->\n")
	sb.WriteString("- **任务**: " + task + "\n")
	sb.WriteString("- **状态**: " + status + "（" + errorType + "）\n")
	sb.WriteString("- **失败类型**: " + errorType + "\n")
	sb.WriteString("- **重试次数**: 0\n")
	sb.WriteString("- **创建时间**: 2026-01-01\n")
	sb.WriteString("- **派单 agent**: zerg-agent\n")
	sb.WriteString("\n## 修复结论\n\n")
	sb.WriteString("- 根因: \n- 修复: \n- 验证: \n")
	return sb.String()
}

// writeMockIssues — 写入一组 mock issue 文件到临时目录，返回目录路径
func writeMockIssues(t *testing.T, issues map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range issues {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write mock issue %s: %v", name, err)
		}
	}
	return dir
}

// TestClusterFailures_EmptyDir — 空目录返回空列表
func TestClusterFailures_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	clusters, err := ClusterFailures(dir)
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}
	if clusters != nil {
		t.Errorf("expected nil/empty clusters, got %d", len(clusters))
	}
}

// TestClusterFailures_NonExistentDir — 不存在的目录返回空列表
func TestClusterFailures_NonExistentDir(t *testing.T) {
	clusters, err := ClusterFailures("/nonexistent/path/to/issues")
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}
	if clusters != nil {
		t.Errorf("expected nil/empty clusters, got %d", len(clusters))
	}
}

// TestClusterFailures_GroupingAndCounting — 测试聚类分组和计数
func TestClusterFailures_GroupingAndCounting(t *testing.T) {
	issues := map[string]string{
		"20260101-120000-abcd.md": mockIssueContent(
			"20260101-120000-abcd",
			StatusEscalated,
			"model_error",
			"write go test file for cluster module",
			"abcd1234"),
		"20260101-120001-abce.md": mockIssueContent(
			"20260101-120001-abce",
			StatusDead,
			"model_error",
			"write go test for cluster module again",
			"abcd5678"),
		"20260101-120002-dead.md": mockIssueContent(
			"20260101-120002-dead",
			StatusEscalated,
			"context_overflow",
			"process large json data context overflow",
			"dead1000"),
		"20260101-120003-deaf.md": mockIssueContent(
			"20260101-120003-deaf",
			StatusDead,
			"context_overflow",
			"handle big file reading",
			"dead2000"),
		"20260101-120004-0000.md": mockIssueContent(
			"20260101-120004-0000",
			StatusEscalated,
			"reasoning_collapse",
			"reasoning stuck on math problem",
			"00001111"),
		"20260101-120005-0001.md": mockIssueContent(
			"20260101-120005-0001",
			StatusDead,
			"reasoning_collapse",
			"another math reasoning stuck",
			"00002222"),
		"20260101-120006-ffff.md": mockIssueContent(
			"20260101-120006-ffff",
			StatusEscalated,
			"resource_exhaustion",
			"max turns reached on complex task",
			"ffff0000"),
	}

	// Add a non-final-status issue (should be ignored)
	issues["20260101-120007-open.md"] = mockIssueContent(
		"20260101-120007-open",
		StatusOpen,
		"model_error",
		"this is open status issue",
		"open0000")

	dir := writeMockIssues(t, issues)

	clusters, err := ClusterFailures(dir)
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}

	// 应排除 StatusOpen 的 issue，剩余 7 个失败 issue
	// 按 errorType + hash前4位 聚类：
	// model_error + abcd → 2个 (abcd1234, abcd5678)
	// context_overflow + dead → 2个 (dead1000, dead2000)
	// reasoning_collapse + 0000 → 2个 (00001111, 00002222)
	// resource_exhaustion + ffff → 1个 (ffff0000)
	// 共 4 个簇
	if len(clusters) != 4 {
		t.Fatalf("expected 4 clusters, got %d", len(clusters))
	}

	// 验证每个簇的 Count
	countMap := make(map[string]int)
	for _, c := range clusters {
		countMap[c.Key] = c.Count
	}

	if countMap["model_error:abcd"] != 2 {
		t.Errorf("model_error:abcd expected count 2, got %d", countMap["model_error:abcd"])
	}
	if countMap["context_overflow:dead"] != 2 {
		t.Errorf("context_overflow:dead expected count 2, got %d", countMap["context_overflow:dead"])
	}
	if countMap["reasoning_collapse:0000"] != 2 {
		t.Errorf("reasoning_collapse:0000 expected count 2, got %d", countMap["reasoning_collapse:0000"])
	}
	if countMap["resource_exhaustion:ffff"] != 1 {
		t.Errorf("resource_exhaustion:ffff expected count 1, got %d", countMap["resource_exhaustion:ffff"])
	}

	// 验证 IssueIDs 包含正确的 issue ID
	for _, c := range clusters {
		if c.Count == 2 {
			if len(c.IssueIDs) != 2 {
				t.Errorf("cluster %s expected 2 IDs, got %d", c.Key, len(c.IssueIDs))
			}
		}
		if c.Count == 1 {
			if len(c.IssueIDs) != 1 {
				t.Errorf("cluster %s expected 1 ID, got %d", c.Key, len(c.IssueIDs))
			}
		}
	}

	// 验证按 Count 降序排列
	for i := 1; i < len(clusters); i++ {
		if clusters[i-1].Count < clusters[i].Count {
			t.Errorf("clusters not sorted by count descending: %d < %d", clusters[i-1].Count, clusters[i].Count)
		}
	}

	// 验证 SuggestedRoot 非空
	for _, c := range clusters {
		if c.SuggestedRoot == "" {
			t.Errorf("cluster %s has empty SuggestedRoot", c.Key)
		}
	}
}

// TestClusterFailures_DifferentErrorTypes — 不同失败类型不应合并
func TestClusterFailures_DifferentErrorTypes(t *testing.T) {
	issues := map[string]string{
		"20260101-aaaa-0000.md": mockIssueContent(
			"20260101-aaaa-0000",
			StatusEscalated,
			"model_error",
			"write test code",
			"aaaa1111"),
		"20260101-bbbb-0000.md": mockIssueContent(
			"20260101-bbbb-0000",
			StatusDead,
			"context_overflow",
			"write test code same task",
			"aaaa1111"), // 相同 hash 但不同 errorType
	}

	dir := writeMockIssues(t, issues)

	clusters, err := ClusterFailures(dir)
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}

	// 不同 errorType 应分为不同簇
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	// 每个簇 1 个
	for _, c := range clusters {
		if c.Count != 1 {
			t.Errorf("expected count 1, got %d for %s", c.Count, c.Key)
		}
	}
}

// TestClusterFailures_Report — 测试 ClusterFailuresReport 输出
func TestClusterFailures_Report(t *testing.T) {
	issues := map[string]string{
		"20260101-0001.md": mockIssueContent(
			"20260101-0001",
			StatusEscalated,
			"model_error",
			"write test code",
			"abcd1234"),
		"20260101-0002.md": mockIssueContent(
			"20260101-0002",
			StatusDead,
			"model_error",
			"read file code",
			"abcd5678"),
	}

	dir := writeMockIssues(t, issues)

	clusters, err := ClusterFailures(dir)
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}

	report := ClusterFailuresReport(clusters)
	if !strings.Contains(report, "共 1 个簇") {
		t.Errorf("report should contain '共 1 个簇', got: %s", report)
	}
	if !strings.Contains(report, "model_error") {
		t.Errorf("report should contain 'model_error', got: %s", report)
	}
	if !strings.Contains(report, "20260101-0001") {
		t.Errorf("report should contain issue ID '20260101-0001', got: %s", report)
	}
}

// TestClusterFailures_AllNonFinalStatus — 所有 issue 都是非终态
func TestClusterFailures_AllNonFinalStatus(t *testing.T) {
	issues := map[string]string{
		"20260101-0001.md": mockIssueContent(
			"20260101-0001",
			StatusOpen,
			"model_error",
			"write test code",
			"abcd1234"),
		"20260101-0002.md": mockIssueContent(
			"20260101-0002",
			StatusRunning,
			"context_overflow",
			"read file code",
			"abcd5678"),
	}

	dir := writeMockIssues(t, issues)

	clusters, err := ClusterFailures(dir)
	if err != nil {
		t.Fatalf("ClusterFailures returned error: %v", err)
	}

	// 没有终态 issue，应返回空列表
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}
