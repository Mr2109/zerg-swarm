// Package api - v2.5.6 程序定量驱动 Agent: Zerg-Git（适合虫族的 git 用法）
// 传统 git 存储引擎保留——虫族用法（每轮 commit/结构化 message/时间线）
// 2026-08-25 Mr2109——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 12 章
package api

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ZergGit 任务 git 管理器（每轮 commit——记忆=git 树）
type ZergGit struct {
	RepoDir string // 任务 git 仓库目录
}

// NewZergGit 新建（目录即仓库——不存在则 init）
func NewZergGit(repoDir string) (*ZergGit, error) {
	z := &ZergGit{RepoDir: repoDir}
	if err := z.Ensure(); err != nil {
		return nil, err
	}
	return z, nil
}

// Ensure 确保是 git 仓库（不是则 init + 初始 commit）
func (z *ZergGit) Ensure() error {
	// v2.5.6 修复: 目录不存在先建（git -C 不存在目录会失败）
	if err := os.MkdirAll(z.RepoDir, 0o755); err != nil {
		return fmt.Errorf("创建仓库目录失败: %w", err)
	}
	if _, err := z.git("rev-parse", "--is-inside-work-tree"); err != nil {
		if _, initErr := z.git("init", "-q"); initErr != nil {
			return fmt.Errorf("git init 失败: %w", initErr)
		}
		// v2.5.6 修复（2026-08-29 q1 日志查漏）: git add/commit 失败必须报——任务 git 底座关键操作
		if _, addErr := z.git("add", "-A"); addErr != nil {
			return fmt.Errorf("git add 失败: %w", addErr)
		}
		if _, cmErr := z.git("commit", "-q", "-m", "init: 任务建立（zerg-git）", "--allow-empty"); cmErr != nil {
			return fmt.Errorf("git init commit 失败: %w", cmErr)
		}
	}
	return nil
}

// CommitRound 每轮提交（Zerg-Git 核心——每轮一个 commit——git 树=记忆）
// round: 轮次——phase: 阶段——summary: 总结（commit message 结构化）
func (z *ZergGit) CommitRound(round int, phase, summary string) error {
	msg := fmt.Sprintf("r%02d [%s]: %s", round, phase, summary)
	// v2.5.6 修复（2026-08-29 q1）: git add 失败必须报
	if _, addErr := z.git("add", "-A"); addErr != nil {
		return fmt.Errorf("git add 失败: %w", addErr)
	}
	if out, err := z.git("commit", "-q", "-m", msg); err != nil {
		// v2.5.6 放宽: 空提交（无新产出——验证/检查类步骤）——用 --allow-empty（模型已响应——步骤完成）
		// 防假完成靠"模型响应了"（受控循环验证）——不是强制文件变化
		if strings.Contains(string(out), "nothing to commit") || strings.Contains(string(out), "no changes") || strings.Contains(string(out), "无文件要提交") {
			if _, err2 := z.git("commit", "-q", "--allow-empty", "-m", msg); err2 != nil {
				return fmt.Errorf("提交失败: %w", err2)
			}
			return nil
		}
		return fmt.Errorf("commit 失败: %w", err)
	}
	return nil
}

// Log 任务时间线（git log——程序直接读——结构化）
func (z *ZergGit) Log() (string, error) {
	out, err := z.git("log", "--oneline", "--no-decorate")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CommitCount 提交次数（任务进展）
func (z *ZergGit) CommitCount() int {
	out, err := z.git("rev-list", "--count", "HEAD")
	if err != nil {
		return 0
	}
	n := strings.TrimSpace(string(out))
	count := 0
	fmt.Sscanf(n, "%d", &count)
	return count
}

// RevertTo 回溯（出问题——git 回溯重做——回到指定轮次 commit）
// round: 回溯到哪轮（保留该轮——之后的重做）
// 返回: 回溯的 commit hash（失败反馈用）
func (z *ZergGit) RevertTo(round int) (string, error) {
	// 找 r%02d 开头的 commit（round 对应的）
	out, err := z.git("log", "--oneline", "--no-decorate")
	if err != nil {
		return "", err
	}
	target := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, fmt.Sprintf("r%02d ", round)) {
			target = strings.Fields(line)[0]
			break
		}
	}
	if target == "" {
		return "", fmt.Errorf("找不到 r%02d 的 commit（无法回溯）", round)
	}
	// hard reset 到该 commit（之后的重做）
	if _, err := z.git("reset", "--hard", target); err != nil {
		return "", fmt.Errorf("回溯失败: %w", err)
	}
	return target, nil
}

// Status 工作区状态（程序检查非空——完成评定）
func (z *ZergGit) Status() (string, error) {
	out, err := z.git("status", "--porcelain")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// git 执行（包装——输出）
func (z *ZergGit) git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", z.RepoDir}, args...)...)
	// S11: 强制英文输出——输出文案判断(如 "nothing to commit")不能依赖系统 locale(中文 git 会误判)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	return cmd.CombinedOutput()
}

// GitTagRound 标记轮次（可选——git tag r03——方便回溯）
func (z *ZergGit) TagRound(round int) {
	tag := fmt.Sprintf("r%02d", round)
	// v2.5.6 修复（2026-08-29 q1）: tag 失败记日志（不阻塞——辅助功能）
	if _, err := z.git("tag", "-f", tag); err != nil {
		log.Printf("⚠️ git tag %s 失败: %v", tag, err)
	}
}

// RoundMarker 轮次标记格式（commit message 前缀——结构化）
func RoundMarker(round int, phase string) string {
	return fmt.Sprintf("r%02d [%s]: ", round, phase)
}

// ZergFS JSONL 落盘（Zerg-FS——task.jsonl 单文件）
// 已实现（zerg_task_file.go）——此处补充: 每轮落盘 + commit 联动

// CommitRoundWithFS 每轮落盘 + git commit（Zerg-FS + Zerg-Git 联动）
// file: task.jsonl——git: 任务仓库——round/phase/summary: 本轮
func CommitRoundWithFS(file *ZergTaskFile, git *ZergGit, phase ZergPhase, step string, output map[string]interface{}, summary string) (int, error) {
	// 1. 写 task.jsonl（Zerg-FS——包罗一切）
	round := NewZergRound(file, phase, step)
	n, err := round.Commit(output, summary)
	if err != nil {
		return 0, fmt.Errorf("task.jsonl 写入失败: %w", err)
	}
	// 2. git commit（Zerg-Git——每轮一个——结构化 message）
	if err := git.CommitRound(n, phase.String(), summary); err != nil {
		return 0, fmt.Errorf("git commit 失败: %w", err)
	}
	// 3. 打 tag（回溯点）
	git.TagRound(n)
	return n, nil
}

// GitStatusNonEmpty 工作区是否有未提交变化（防假完成——非空检查）
func GitStatusNonEmpty(repoDir string) bool {
	z := &ZergGit{RepoDir: repoDir}
	out, err := z.Status()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

var _ = time.Now // 保持 time 导入（后续用）
