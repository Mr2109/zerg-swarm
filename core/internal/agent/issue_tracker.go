package agent

// issue_tracker.go — v2.5.1 错误自愈闭环 P0：失败挂单（SWE-bench 格式）
// 设计：docs/设计-错误自愈闭环.md
// 职责：失败分类 → 写任务区 <issuesDir>/instance_id.md（错误入工作流非知识库；
// 旧目录 docs/issues/ 已于 2026-09-19 分家至 Zerg-内部文档/issues/）

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IssueRecord — 失败问题单（SWE-bench 8 字段——本地任务区 issue 目录）
// v2.5.2 状态机：open→queued→running→fixing→verified→done（+retry→queued, escalated/dead 终态）

// Issue Status — v2.5.2 完整状态枚举
const (
	StatusOpen       = "open"           // 新创建，待分配
	StatusQueued     = "queued"         // 已排队
	StatusRunning    = "running"        // 正在处理
	StatusFixing     = "fixing"         // 正在修复
	StatusVerified   = "verified"       // 已验证通过
	StatusWaitReview = "waiting_review" // C4: 验收通过——等脑确认复查（v2.5.3）
	StatusDone       = "done"           // 已关闭（终态）
	StatusRetry      = "retry"          // 需要重试
	StatusEscalated  = "escalated"      // 已升级（终态）
	StatusDead       = "dead"           // 已放弃（终态）
)

// IsValidStatus — 检查状态是否合法
func IsValidStatus(s string) bool {
	switch s {
	case StatusOpen, StatusQueued, StatusRunning, StatusFixing, StatusVerified, StatusWaitReview, StatusDone, StatusRetry, StatusEscalated, StatusDead:
		return true
	}
	return false
}

// TransitionIssue — 校验并执行 issue 状态流转
// content: issue markdown 全文
// from: 当前状态
// to: 目标状态
// 返回: (新 content, error)
// 非法流转返回 error，非法状态也返回 error
func TransitionIssue(content, from, to string) (string, error) {
	// 1. 校验源状态和目标状态是否合法
	if !IsValidStatus(from) {
		return "", fmt.Errorf("invalid source status %q", from)
	}
	if !IsValidStatus(to) {
		return "", fmt.Errorf("invalid target status %q", to)
	}
	// 2. 同态不变
	if from == to {
		return content, nil
	}
	// 3. 校验流转合法性
	if !isLegalTransition(from, to) {
		return "", fmt.Errorf("illegal transition: %s → %s", from, to)
	}
	// 4. 执行状态替换（替换 "- **状态**: xxx" 行）
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **状态**:") {
			lines[i] = fmt.Sprintf("- **状态**: %s", to)
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

// isLegalTransition — 校验状态流转是否合法（v2.5.2 状态机规则）
func isLegalTransition(from, to string) bool {
	switch from {
	case StatusOpen:
		return to == StatusQueued
	case StatusQueued:
		return to == StatusRunning || to == StatusRetry
	case StatusRunning:
		return to == StatusFixing || to == StatusRetry || to == StatusVerified // v2.5.3: DevMode worker 直接完成（不标 fixing）
	case StatusFixing:
		return to == StatusVerified || to == StatusRetry
	case StatusRetry:
		return to == StatusQueued // 重试回队列（v2.5.2 设计）
	case StatusVerified:
		return to == StatusWaitReview || to == StatusDone // C4: verified→waiting_review（等脑确认）→done
	case StatusWaitReview:
		return to == StatusDone || to == StatusRetry // 脑确认合并→done；拒绝→retry
	case StatusDone, StatusEscalated, StatusDead:
		return false // 终态不可再流转
	}
	return false
}

// CanRetry — 标记 issue 进入 retry 状态（fixing→retry）
func CanRetry(content string) (string, error) {
	return TransitionIssue(content, StatusFixing, StatusRetry)
}

// CanEscalate — 将 issue 升级为终态 escalated（任意非终态→escalated）
func CanEscalate(content, currentStatus string) (string, error) {
	if !IsValidStatus(currentStatus) {
		return "", fmt.Errorf("invalid current status %q", currentStatus)
	}
	if isTerminalStatus(currentStatus) {
		return "", fmt.Errorf("status %q is already terminal", currentStatus)
	}
	// 非终态均可升级到 escalated
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **状态**:") {
			lines[i] = fmt.Sprintf("- **状态**: %s", StatusEscalated)
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

// CanDead — 将 issue 标记为 dead（任意非终态→dead）
func CanDead(content, currentStatus string) (string, error) {
	if !IsValidStatus(currentStatus) {
		return "", fmt.Errorf("invalid current status %q", currentStatus)
	}
	if isTerminalStatus(currentStatus) {
		return "", fmt.Errorf("status %q is already terminal", currentStatus)
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **状态**:") {
			lines[i] = fmt.Sprintf("- **状态**: %s", StatusDead)
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

// isTerminalStatus — 判断是否为终态
func isTerminalStatus(s string) bool {
	return s == StatusDone || s == StatusEscalated || s == StatusDead
}

// IssueRecord — 失败问题单（SWE-bench 8 字段——本地任务区 issue 目录）
type IssueRecord struct {
	InstanceID    string   `json:"instance_id"`          // 唯一标识（时间戳+任务hash）
	Task          string   `json:"task"`                 // 任务描述（截断 200）
	ExitStatus    string   `json:"exit_status"`          // failed/timeout/blocked/token_budget/model_error
	ErrorType     string   `json:"error_type"`           // 失败分类枚举
	RetryCount    int      `json:"retry_count"`          // 重试次数
	AssignedAgent string   `json:"assigned_agent"`       // 派单 agent（默认 zerg-agent）
	CreatedAt     string   `json:"created_at"`           // 创建时间
	LastAttemptAt string   `json:"last_attempt_at"`      // 最后尝试
	ToolTrace     []string `json:"tool_trace,omitempty"` // 工具调用轨迹（诊断用）
	Status        string   `json:"status"`               // open/fixing/resolved/escalated
	Resolution    string   `json:"resolution,omitempty"` // 修复结论（agent 填）
}

// ClassifyFailure — 失败分类（LoopResult → error_type——SWE-bench 枚举）
func ClassifyFailure(reason TerminateReason) string {
	switch reason {
	case ReasonModelError:
		return "model_error" // 模型调用失败（网关/超时）
	case ReasonBlocked:
		return "reasoning_collapse" // 无进展终止（推理卡住）
	case ReasonTokenBudget:
		return "context_overflow" // token 耗尽（上下文溢出）
	case ReasonMaxTurns:
		return "resource_exhaustion" // 达到轮次（任务过大/探索过量）
	default:
		return "unknown"
	}
}

// CreateIssue — 挂单（失败 → 任务区 <issuesDir>/instance_id.md——git 提交由调用方）
// 返回 issue 文件路径
func CreateIssue(workDir string, task string, reason TerminateReason, retryCount int, toolTrace []string) (string, error) {
	// 去重检查（缺漏4——一级：任务 hash 相似已存在则复用——简单版：同任务同原因不重复）
	issuesDir := filepath.Join(workDir, "docs", "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		return "", err
	}

	now := time.Now()
	// instance_id: 时间戳 + 任务前 8 字 hash
	hash := fmt.Sprintf("%x", hashString(task)[:4])
	instanceID := fmt.Sprintf("%s-%s", now.Format("20060102-150405"), hash)

	// v2.5.1 去重（一级——精确 hash）：同任务 hash 已存在且 open/fixing → 不重复挂单
	taskHash := fmt.Sprintf("%08x", hashString(task))
	if existing := findOpenIssueByTaskHash(issuesDir, taskHash); existing != "" {
		return existing, nil // 已有未处理问题单——返回它（不重复创建）
	}

	rec := IssueRecord{
		InstanceID:    instanceID,
		Task:          truncate(task, 200),
		ExitStatus:    string(reason),
		ErrorType:     ClassifyFailure(reason),
		RetryCount:    retryCount,
		AssignedAgent: "zerg-agent",
		CreatedAt:     now.Format(time.RFC3339),
		LastAttemptAt: now.Format(time.RFC3339),
		ToolTrace:     toolTrace,
		Status:        "open",
	}

	// 写 markdown 问题单（含任务 hash 标记——去重用）
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Issue: %s\n\n", instanceID))
	sb.WriteString(fmt.Sprintf("<!-- task-hash: %s -->\n", taskHash))
	sb.WriteString(fmt.Sprintf("- **任务**: %s\n", rec.Task))
	sb.WriteString(fmt.Sprintf("- **状态**: %s（%s）\n", rec.Status, rec.ExitStatus))
	sb.WriteString(fmt.Sprintf("- **失败类型**: %s\n", rec.ErrorType))
	sb.WriteString(fmt.Sprintf("- **重试次数**: %d\n", rec.RetryCount))
	sb.WriteString(fmt.Sprintf("- **创建时间**: %s\n", rec.CreatedAt))
	sb.WriteString(fmt.Sprintf("- **派单 agent**: %s\n", rec.AssignedAgent))
	if len(toolTrace) > 0 {
		sb.WriteString("\n## 工具轨迹（诊断）\n\n")
		for _, t := range toolTrace {
			sb.WriteString(fmt.Sprintf("- %s\n", truncate(t, 120)))
		}
	}
	sb.WriteString("\n## 修复结论（agent 接单后填）\n\n")
	sb.WriteString("- 根因: \n- 修复: \n- 验证: \n")

	path := filepath.Join(issuesDir, instanceID+".md")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// hashString — 简单字符串 hash（任务去重用）
func hashString(s string) string {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

// MarkIssueStatus — 更新 issue 状态（open→fixing→resolved/escalated）
func MarkIssueStatus(content string, status string) string {
	return markIssueStatus(content, status)
}

// findOpenIssueByTaskHash — 查找同任务 hash 且状态 open/fixing 的 issue（去重——一级）
func findOpenIssueByTaskHash(issuesDir string, taskHash string) string {
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(issuesDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		// 同 hash 且未 resolved/escalated
		if strings.Contains(content, "task-hash: "+taskHash) &&
			!strings.Contains(content, "resolved") &&
			!strings.Contains(content, "escalated") {
			return path
		}
	}
	return ""
}

// markIssueStatus — 替换 issue 的状态行（- **状态**: xxx → 新状态）
func markIssueStatus(content string, status string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "- **状态**:") {
			lines[i] = fmt.Sprintf("- **状态**: %s", status)
			break
		}
	}
	return strings.Join(lines, "\n")
}
