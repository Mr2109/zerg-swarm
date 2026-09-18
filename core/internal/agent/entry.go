package agent

// entry.go — v2.5.2 任务入口统一
// 职责：四类源（git/cron/api/a2a）统一 SubmitTask 入口 + TaskSpec 校验 + 去重

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Task Type — 任务类型枚举
const (
	TaskTypeCode     = "code"     // 代码任务（修复/重构/新功能）
	TaskTypeReport   = "report"   // 报告任务（分析/调研/文档）
	TaskTypeResearch = "research" // 研究任务（技术调研/方案验证）
	TaskTypeFix      = "fix"      // 修复任务（bug 修复/错误自愈）
)

// IsValidTaskType — 检查任务类型是否合法
func IsValidTaskType(t string) bool {
	switch t {
	case TaskTypeCode, TaskTypeReport, TaskTypeResearch, TaskTypeFix:
		return true
	}
	return false
}

// Priority — 优先级枚举
const (
	TaskPriorityHigh   = "high"   // 高优先级（阻塞/紧急）
	TaskPriorityNormal = "normal" // 正常优先级
	TaskPriorityLow    = "low"    // 低优先级
)

// IsValidPriority — 检查优先级是否合法
func IsValidPriority(p string) bool {
	switch p {
	case TaskPriorityHigh, TaskPriorityNormal, TaskPriorityLow:
		return true
	}
	return false
}

// Source — 任务来源枚举
const (
	SourceGit  = "git"  // Git 事件触发（push/PR/issue）
	SourceCron = "cron" // 定时任务触发
	SourceAPI  = "api"  // API 调用触发
	SourceA2A  = "a2a"  // Agent-to-Agent 触发
)

// IsValidSource — 检查来源是否合法
func IsValidSource(s string) bool {
	switch s {
	case SourceGit, SourceCron, SourceAPI, SourceA2A:
		return true
	}
	return false
}

// TaskSpec — 任务规格（四类源统一入口）
type TaskSpec struct {
	Task     string `json:"task"`     // 任务描述
	Type     string `json:"type"`     // 任务类型：code/report/research/fix
	Priority string `json:"priority"` // 优先级：high/normal/low
	Repo     string `json:"repo"`     // 关联仓库（可选）
	Source   string `json:"source"`   // 来源：git/cron/api/a2a
}

// Validate — 校验 TaskSpec 合法性
func (s TaskSpec) Validate() error {
	if strings.TrimSpace(s.Task) == "" {
		return fmt.Errorf("task description is required")
	}
	if !IsValidTaskType(s.Type) {
		return fmt.Errorf("invalid task type %q (valid: code/report/research/fix)", s.Type)
	}
	if !IsValidPriority(s.Priority) {
		return fmt.Errorf("invalid priority %q (valid: high/normal/low)", s.Priority)
	}
	if !IsValidSource(s.Source) {
		return fmt.Errorf("invalid source %q (valid: git/cron/api/a2a)", s.Source)
	}
	return nil
}

// taskPriority — 根据优先级映射到初始状态
// high → queued（高优先级直接排队）
// normal → queued（正常优先级排队）
// low → queued（低优先级排队，调度器决定顺序）
func taskPriorityStatus(priority string) string {
	return StatusQueued // 所有优先级统一进入队列
}

// SubmitTask — 统一任务提交入口（四类源：git/cron/api/a2a）
//
// 流程：
//  1. 校验 TaskSpec
//  2. 计算 task hash，检查去重（同 hash 且未关闭 → 返回已有路径）
//  3. 生成 issue 文件写入任务区（旧目录 <工作区>/docs/issues/ 已于 2026-09-19 分家至 Zerg-内部文档/issues/；
//     「内部任务单目录可配」落地后写单落点见 statepath.IssuesDir()）
//
// 返回：issue 文件路径
func SubmitTask(spec TaskSpec) (string, error) {
	// 1. 校验 TaskSpec
	if err := spec.Validate(); err != nil {
		return "", fmt.Errorf("invalid task spec: %w", err)
	}

	issuesDir := filepath.Join(".", "docs", "issues")
	// 绝对路径（不依赖 CWD——调度器/API 调用安全——先去重一致）
	if abs, err := filepath.Abs(issuesDir); err == nil {
		issuesDir = abs
	}
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		return "", fmt.Errorf("create issues dir: %w", err)
	}

	// 2. 计算 task hash 并检查去重
	taskHash := fmt.Sprintf("%08x", hashString(spec.Task))
	if existing := findOpenIssueByTaskHash(issuesDir, taskHash); existing != "" {
		return existing, nil // 去重：已有未关闭 issue
	}

	// 3. 生成 issue 文件
	now := time.Now()
	hash := fmt.Sprintf("%x", hashString(spec.Task)[:4])
	instanceID := fmt.Sprintf("%s-%s", now.Format("20060102-150405"), hash)

	status := taskPriorityStatus(spec.Priority)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Task: %s\n\n", instanceID))
	sb.WriteString(fmt.Sprintf("<!-- task-hash: %s -->\n", taskHash))
	sb.WriteString(fmt.Sprintf("- **任务**: %s\n", truncate(spec.Task, 200)))
	sb.WriteString(fmt.Sprintf("- **状态**: %s\n", status))
	sb.WriteString(fmt.Sprintf("- **类型**: %s\n", spec.Type))
	sb.WriteString(fmt.Sprintf("- **优先级**: %s\n", spec.Priority))
	sb.WriteString(fmt.Sprintf("- **来源**: %s\n", spec.Source))
	if spec.Repo != "" {
		sb.WriteString(fmt.Sprintf("- **仓库**: %s\n", spec.Repo))
	}
	// v2.5.2 补字段（调度器 parseIssue 依赖——重试次数/失败类型——否则死信不触发）
	sb.WriteString("- **失败类型**: \n")
	sb.WriteString("- **重试次数**: 0\n")
	// v2.5.3 C4: 分支字段（开发模式——worker 容器内分支名——脑确认 merge 用）
	sb.WriteString(fmt.Sprintf("- **分支**: task-%s\n", instanceID))
	sb.WriteString(fmt.Sprintf("- **创建时间**: %s\n", now.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **派单 agent**: %s\n", "zerg-agent"))
	sb.WriteString("\n## 执行记录\n\n")
	sb.WriteString("- 开始: \n- 完成: \n- 结论: \n")

	path := filepath.Join(issuesDir, instanceID+".md")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("write issue: %w", err)
	}

	return path, nil
}
