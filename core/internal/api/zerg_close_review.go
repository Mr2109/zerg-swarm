// Package api - v2.5.6 程序定量驱动 Agent: 封闭轮 + 复查
// 封闭: 模型读 git 树 → 写结论报告 → 最后一轮 commit
// 复查: 跨家族模型读 git 树 → 判断质量 → 通过/打回
// 2026-08-25 Mr2109（E 定稿）——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 14/17 章
package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZergClose 封闭轮（任务收尾）
type ZergClose struct {
	TaskDir  string        // 任务目录
	Git      *ZergGit      // 任务 git
	TaskFile *ZergTaskFile // task.jsonl
}

// NewZergClose 新建封闭管理器
func NewZergClose(taskDir string, git *ZergGit, tf *ZergTaskFile) *ZergClose {
	return &ZergClose{TaskDir: taskDir, Git: git, TaskFile: tf}
}

// ReportPrompt 报告生成提示（模型读 git 树写结论——基于真实记录）
func (z *ZergClose) ReportPrompt() string {
	log, _ := z.Git.Log()
	summary, _ := z.TaskFile.Summary(100)
	return fmt.Sprintf(`你是任务的封闭报告生成者。请读取任务 git 树和轮次总结，写结论报告。

【任务时间线（git log）】:
%s

【轮次总结（task.jsonl）】:
%s

【报告要求】:
1. 任务目标（做了什么）
2. 执行过程（方案→选定→执行——每步成果）
3. 最终结果（交付了什么——git 树里的产出）
4. 经验教训（下次同类任务注意什么——反馈给 Skill）

请把报告写到: %s（用 write 工具——markdown 格式）
`, log, summary, filepath.Join(z.TaskDir, "reports", "final.md"))
}

// SaveReport 保存报告（程序兜底——如果模型没写）
func (z *ZergClose) SaveReport(content string) (string, error) {
	dir := filepath.Join(z.TaskDir, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "final.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// CloseCommit 封闭提交（报告 + git 收尾——最后一轮）
// reportPath: 报告路径——summary: 封闭总结
func (z *ZergClose) CloseCommit(reportPath, summary string) error {
	// 报告入 git
	if _, err := z.Git.git("add", "-A"); err != nil {
		return fmt.Errorf("git add 失败: %w", err)
	}
	if out, err := z.Git.git("commit", "-q", "-m", fmt.Sprintf("close: %s", summary)); err != nil {
		// v2.5.6 2026-08-28 修复: 执行轮最后一步已提交——工作树干净——git commit 报 nothing to commit exit 1
		// → 任务假失败（全失败根因之一）——空提交用 --allow-empty 兜底（封闭轮完成即算）
		if strings.Contains(string(out), "nothing to commit") || strings.Contains(string(out), "no changes") {
			if _, err2 := z.Git.git("commit", "-q", "--allow-empty", "-m", fmt.Sprintf("close: %s", summary)); err2 != nil {
				return fmt.Errorf("封闭提交失败: %w", err2)
			}
			return nil
		}
		return fmt.Errorf("封闭提交失败: %w", err)
	}
	return nil
}

// ZergReview 复查（跨家族模型——读 git 树判断质量）
type ZergReview struct {
	TaskDir string
	Git     *ZergGit
}

// NewZergReview 新建复查
func NewZergReview(taskDir string, git *ZergGit) *ZergReview {
	return &ZergReview{TaskDir: taskDir, Git: git}
}

// ReviewPrompt 复查提示（跨家族模型读 git 树——判断）
func (z *ZergReview) ReviewPrompt() string {
	log, _ := z.Git.Log()
	// 报告内容
	report := ""
	if data, err := os.ReadFile(filepath.Join(z.TaskDir, "reports", "final.md")); err == nil {
		report = string(data)
	}
	return fmt.Sprintf(`你是任务复查者（跨家族模型——独立判断）。请读任务 git 树和报告，判断任务质量。

【git 时间线】:
%s

【最终报告】:
%s

【复查要求——输出 JSON】:
{"pass": true/false, "reason": "判断理由", "issues": ["问题1", "问题2"]}
规则:
- pass=true: 任务真完成——产出符合要求——质量可接受
- pass=false: 有严重问题（假完成/产出不对/质量差）——打回重做
- 基于 git 树真实记录判断——不信自报
`, log, report)
}

// ParseReview 解析复查结果
type ReviewResult struct {
	Pass   bool     `json:"pass"`
	Reason string   `json:"reason"`
	Issues []string `json:"issues"`
}

// ParseReviewJSON 解析复查 JSON 输出
func ParseReviewJSON(content string) (*ReviewResult, error) {
	// 兼容: 模型可能输出 ```json 包裹
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var r ReviewResult
	if err := json.Unmarshal([]byte(cleaned), &r); err != nil {
		return nil, fmt.Errorf("复查 JSON 解析失败: %w", err)
	}
	if r.Reason == "" {
		return nil, fmt.Errorf("复查缺 reason")
	}
	return &r, nil
}

var _ = time.Now
