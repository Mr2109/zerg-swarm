package agent

// cluster.go — v2.5.2 失败任务聚类（根因聚类——同类失败合并）
// 职责：扫描任务区（issuesDir）的终态 issue → 按失败类型+任务关键词聚类 → 输出簇列表

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FailureCluster — 失败簇（同类失败合并后的分组）
type FailureCluster struct {
	Key           string   `json:"key"`            // 簇标识（失败类型:任务hash前4位）
	ErrorType     string   `json:"error_type"`     // 失败类型
	Count         int      `json:"count"`          // 簇内 issue 数量
	IssueIDs      []string `json:"issue_ids"`      // 簇内所有 issue ID
	SuggestedRoot string   `json:"suggested_root"` // 建议根因
}

// clusterKey — 聚类键（失败类型 + 任务 hash 前 4 位）
type clusterKey struct {
	errorType string
	taskHash  string
}

// issueParsed — 解析后的 issue 字段
type issueParsed struct {
	id        string
	status    string
	errorType string
	task      string
	taskHash  string
}

// cleanStatus — 清理状态值（去掉括号后缀：escalated（reason）→ escalated）
func cleanStatus(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "（"); idx != -1 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// parseIssueFile — 从 markdown issue 文件中提取关键字段
func parseIssueFile(path string) (*issueParsed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	info := &issueParsed{}
	info.id = strings.TrimSuffix(filepath.Base(path), ".md")

	// 逐行解析
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// 状态行：- **状态**: escalated / dead（可能带括号：escalated（reason））
		if strings.HasPrefix(line, "- **状态**:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.status = cleanStatus(parts[1])
			}
		}
		// 失败类型：- **失败类型**: model_error ...
		if strings.HasPrefix(line, "- **失败类型**:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.errorType = cleanStatus(parts[1])
			}
		}
		// 任务：- **任务**: xxx
		if strings.HasPrefix(line, "- **任务**:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.task = strings.TrimSpace(parts[1])
			}
		}
		// 任务 hash：<!-- task-hash: abcdef01 -->
		if strings.HasPrefix(line, "<!-- task-hash:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimSuffix(val, "-->")
				info.taskHash = val
			}
		}
	}

	return info, nil
}

// ClusterFailures — 扫描 issuesDir 中的失败 issue（escalated/dead），按失败类型+任务关键词聚类
// issuesDir: 任务区目录路径（旧目录 docs/issues/ 已于 2026-09-19 分家至 Zerg-内部文档/issues/）
// 返回: 簇列表（按 Count 降序排列）
func ClusterFailures(issuesDir string) ([]FailureCluster, error) {
	// 1. 读取目录下所有 issue 文件
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read issues dir: %w", err)
	}

	// 2. 筛选终态失败 issue（escalated / dead）
	var failures []*issueParsed
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(issuesDir, e.Name())
		info, err := parseIssueFile(path)
		if err != nil {
			continue // 解析失败跳过
		}
		if info.status != StatusEscalated && info.status != StatusDead {
			continue
		}
		failures = append(failures, info)
	}

	// 3. 按失败类型+任务hash前4位聚类
	groups := make(map[clusterKey][]*issueParsed)
	for _, f := range failures {
		hash4 := f.taskHash
		if len(hash4) > 4 {
			hash4 = hash4[:4]
		}
		key := clusterKey{errorType: f.errorType, taskHash: hash4}
		groups[key] = append(groups[key], f)
	}

	// 4. 构建 FailureCluster 列表
	var clusters []FailureCluster
	for key, items := range groups {
		keyStr := fmt.Sprintf("%s:%s", key.errorType, key.taskHash)

		var ids []string
		for _, item := range items {
			ids = append(ids, item.id)
		}

		suggestedRoot := inferSuggestedRoot(items)

		clusters = append(clusters, FailureCluster{
			Key:           keyStr,
			ErrorType:     key.errorType,
			Count:         len(items),
			IssueIDs:      ids,
			SuggestedRoot: suggestedRoot,
		})
	}

	// 5. 按 Count 降序排列
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})

	return clusters, nil
}

// inferSuggestedRoot — 基于簇内任务关键词推断建议根因
func inferSuggestedRoot(items []*issueParsed) string {
	keywordCount := make(map[string]int)
	for _, item := range items {
		words := extractKeywords(item.task)
		for _, kw := range words {
			keywordCount[kw]++
		}
	}

	if len(keywordCount) == 0 {
		return fmt.Sprintf("%s 类失败（%d 个实例）", items[0].errorType, len(items))
	}

	var topKW string
	topCount := 0
	for kw, cnt := range keywordCount {
		if cnt > topCount {
			topKW = kw
			topCount = cnt
		}
	}

	if topCount >= len(items)/2 {
		return fmt.Sprintf("%s 类失败，高频任务关键词: %s", items[0].errorType, topKW)
	}
	return fmt.Sprintf("%s 类失败（%d 个实例）", items[0].errorType, len(items))
}

// extractKeywords — 从任务描述中提取关键词（按标点和空格切分）
func extractKeywords(task string) []string {
	var words []string
	var buf strings.Builder
	for _, r := range task {
		if r == ' ' || r == '\t' || r == '\n' || r == ':' || r == '：' ||
			r == '(' || r == '（' || r == ')' || r == '）' ||
			r == ',' || r == '，' || r == '.' || r == '。' {
			if buf.Len() > 0 {
				words = append(words, buf.String())
				buf.Reset()
			}
		} else {
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		words = append(words, buf.String())
	}
	return words
}

// ClusterFailuresReport — 生成聚类报告（调试用）
func ClusterFailuresReport(clusters []FailureCluster) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 失败任务聚类报告（共 %d 个簇）===\n\n", len(clusters)))
	for i, c := range clusters {
		sb.WriteString(fmt.Sprintf("#%d [%s] 数量: %d\n", i+1, c.Key, c.Count))
		sb.WriteString(fmt.Sprintf("  建议根因: %s\n", c.SuggestedRoot))
		sb.WriteString(fmt.Sprintf("  Issue IDs: %s\n", strings.Join(c.IssueIDs, ", ")))
		sb.WriteString("\n")
	}
	return sb.String()
}
