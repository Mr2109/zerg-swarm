package agent

// worker.go — v2.5.2 独立 Worker 执行体
// 设计：P0 — Worker 独立进程跑 zerg-agent，封装 Worker 类型
// 职责：执行 zerg-agent -issue <path>，根据退出码更新 issue 状态
// 状态链：running → fixing → verified → done（成功）
//        或 running → failed + 挂单重派（失败）

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// 常量

const (
	workerTimeout  = 10 * time.Minute // worker 执行超时
	statusOpen     = "open"
	statusRunning  = "running"
	statusFixing   = "fixing"
	statusVerified = "verified"
	statusDone     = "done"
	statusFailed   = "failed"
	statusRetry    = "retry"
	statusQueued   = "queued"

	maxRetryBeforeDead = 3 // 重试上限，超过标记 dead
	cooldownDuration   = 30 * time.Second // 失败冷却时间
)

// Worker — 独立 worker 执行体
// 封装 zerg-agent 执行 + 状态更新 + 失败处理
type Worker struct {
	mu       sync.Mutex
	runner   WorkerRunner // 可注入 runner（测试 mock——生产默认 exec）
	workDir  string      // 工作目录（用于相对路径解析）
	dispatch map[string]time.Time // 派单时间记录（冷却用）
}

// NewWorker — 创建 worker
func NewWorker(workDir string) *Worker {
	return &Worker{
		workDir:  workDir,
		dispatch: make(map[string]time.Time),
	}
}

// SetRunner — 注入 runner（测试 mock——生产不设置，默认 exec zerg-agent）
func (w *Worker) SetRunner(fn WorkerRunner) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.runner = fn
}

// Run — 执行单个 issue（阻塞直到完成）
// issuePath: issue 文件绝对路径
// 返回: exitCode（0=成功）, error
func (w *Worker) Run(issuePath string) (int, error) {
	// 1. 验证 issue 文件存在
	data, err := os.ReadFile(issuePath)
	if err != nil {
		return -1, fmt.Errorf("读取 issue 失败: %w", err)
	}
	content := string(data)

	// 2. 检查当前状态
	currentStatus := extractStatus(content)
	if currentStatus == "" {
		log.Printf("❌ 无法解析 issue 状态: %s", issuePath)
		return -1, fmt.Errorf("无法解析 issue 状态: %s", issuePath)
	}

	// 3. 标记 running（如果还不是 running）——open→queued→running 两步（状态机要求）
	if currentStatus != statusRunning {
		work := content
		cur := currentStatus
		// open → queued（第一步）
		if cur == statusOpen {
			q, err := TransitionIssue(work, statusOpen, statusQueued)
			if err != nil {
				return -1, fmt.Errorf("标记 queued 失败: %w", err)
			}
			work = q
			cur = statusQueued
		}
		// queued → running（第二步）
		if cur == statusQueued {
			r, err := TransitionIssue(work, statusQueued, statusRunning)
			if err != nil {
				return -1, fmt.Errorf("标记 running 失败: %w", err)
			}
			work = r
			cur = statusRunning
		}
		if work != content {
			if err := os.WriteFile(issuePath, []byte(work), 0o644); err != nil {
				return -1, fmt.Errorf("写入 running 状态失败: %w", err)
			}
			content = work
		}
	}

	// 4. 标记 fixing（running → fixing）
	newContent, err := TransitionIssue(content, statusRunning, statusFixing)
	if err != nil {
		log.Printf("❌ 状态转换 fixing 失败 %s: %v", issuePath, err)
		return -1, fmt.Errorf("标记 fixing 失败: %w", err)
	}
	if newContent != content {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			return -1, fmt.Errorf("写入 fixing 状态失败: %w", err)
		}
		content = newContent
	}
	log.Printf("📝 状态已转换: %s → fixing", issuePath)

	log.Printf("🔧 Worker 开始执行: %s（当前状态: fixing）", issuePath)

	// 5. 执行 zerg-agent
	exitCode, execErr := w.executeAgent(issuePath)
	if execErr != nil {
		log.Printf("❌ Worker 执行异常 %s: %v", issuePath, execErr)
		return exitCode, execErr
	}

	// 6. 根据退出码更新状态
	if exitCode == 0 {
		// 成功：fixing → verified → done
		return w.markSuccess(issuePath, content)
	}

	// 失败：处理失败逻辑
	return w.handleFailure(issuePath, content, exitCode)
}

// executeAgent — 执行 zerg-agent（可注入 mock）
func (w *Worker) executeAgent(issuePath string) (int, error) {
	w.mu.Lock()
	runner := w.runner
	w.mu.Unlock()

	if runner != nil {
		// mock/注入 runner（测试用）
		return runner(issuePath), nil
	}

	// 生产默认：exec zerg-agent
	ctx, cancel := context.WithTimeout(context.Background(), workerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zerg-agent", "-issue", issuePath)
	cmd.Dir = w.workDir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("🐛 zerg-agent 执行失败，退出码 %d（issue: %s）", exitErr.ExitCode(), issuePath)
			return exitErr.ExitCode(), nil // 退出码不是 0 = 失败（正常路径）
		}
		log.Printf("🐛 zerg-agent 执行异常（非退出码错误）: %v", err)
	return -1, fmt.Errorf("执行 zerg-agent 失败: %w", err)
	}

	return 0, nil
}

// markSuccess — 成功路径：fixing → verified → done
func (w *Worker) markSuccess(issuePath, content string) (int, error) {
	// fixing → verified
	newContent, err := w.transitionOrLog(issuePath, content, statusFixing, statusVerified)
	if err != nil {
		return 1, err
	}

	// verified → done
	newContent, err = w.transitionOrLog(issuePath, newContent, statusVerified, statusDone)
	if err != nil {
		return 1, err
	}

	if newContent != content {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			log.Printf("⚠️ 写入 done 状态失败 %s: %v", issuePath, err)
			return 1, err
		}
	}

	log.Printf("✅ Worker 完成: %s（→ done）", issuePath)
	return 0, nil
}

// transitionOrLog — 封装 TransitionIssue 调用，统一错误处理
func (w *Worker) transitionOrLog(issuePath string, content, from, to string) (string, error) {
	result, err := TransitionIssue(content, from, to)
	if err != nil {
		log.Printf("⚠️ 标记 %s 失败 %s: %v", to, issuePath, err)
		return "", err
	}
	log.Printf("📝 状态转换: %s → %s（issue: %s）", from, to, issuePath)
	return result, nil
}

// writeContentOrLog — 封装文件写入，统一错误处理
func (w *Worker) writeContentOrLog(issuePath string, content string) error {
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		log.Printf("⚠️ 写入 %s 失败: %v", issuePath, err)
		return err
	}
	return nil
}

// handleFailure — 失败路径：更新重试次数，冷却检查，挂单重派
func (w *Worker) handleFailure(issuePath, content string, exitCode int) (int, error) {
	// 解析当前重试次数
	issue := parseIssue(issuePath, content)
	if issue == nil {
		log.Printf("⚠️ 解析 issue 失败 %s", issuePath)
		return 1, fmt.Errorf("解析 issue 失败")
	}

	retryCount := issue.RetryCount + 1

	// 更新重试次数和最后尝试时间
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "- **重试次数**:") {
			lines[i] = fmt.Sprintf("- **重试次数**: %d", retryCount)
		}
		if strings.HasPrefix(line, "- **最后尝试**:") {
			lines[i] = fmt.Sprintf("- **最后尝试**: %s", time.Now().Format(time.RFC3339))
		}
	}
	newContent := strings.Join(lines, "\n")

	// 重试次数 ≥ maxRetryBeforeDead → 死信
	if retryCount >= maxRetryBeforeDead {
		deadContent, terr := CanDead(newContent, statusRunning)
		if terr != nil {
			log.Printf("⚠️ 标记 dead 失败 %s: %v", issuePath, terr)
		} else {
			if deadContent != newContent {
				if err := w.writeContentOrLog(issuePath, deadContent); err != nil {
					// 写死信失败不影响返回
				}
			}
		}
		log.Printf("📛 死信: %s（重试 %d 次达上限 %d）", issuePath, retryCount, maxRetryBeforeDead)
		return 1, nil
	}

	// 冷却检查
	w.mu.Lock()
	lastTime, existed := w.dispatch[issuePath]
	w.mu.Unlock()

	if existed && time.Since(lastTime) < cooldownDuration {
		// 冷却中——更新重试次数但不重派
		if err := w.writeContentOrLog(issuePath, newContent); err != nil {
			// 冷却中写失败不阻塞
		}
		log.Printf("❄️ 冷却中: %s（距上次 %v < %v）", issuePath, time.Since(lastTime), cooldownDuration)
		return 1, nil
	}

	// 冷却结束 / 首次失败 → 标记 retry → queued（用辅助函数减少重复）
	log.Printf("🔄 冷却结束/首次失败: %s（退出码: %d，重试: %d）", issuePath, exitCode, retryCount)
	newContent, err := w.transitionOrLog(issuePath, newContent, statusRunning, statusRetry)
	if err != nil {
		return 1, err
	}
	newContent, err = w.transitionOrLog(issuePath, newContent, statusRetry, statusQueued)
	if err != nil {
		return 1, err
	}

	// 更新文件 + 记录派单时间
	if newContent != content {
		if err := w.writeContentOrLog(issuePath, newContent); err != nil {
			return 1, err
		}
	}

	// 记录派单时间
	w.mu.Lock()
	w.dispatch[issuePath] = time.Now()
	w.mu.Unlock()

	log.Printf("🔄 重派: %s（第 %d 次重试，→ queued）", issuePath, retryCount)
	return 1, nil
}

// extractStatus — 从 markdown 内容提取状态
func extractStatus(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "- **状态**:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "（"+extractExitStatus(parts[1])+"）"))
			}
		}
	}
	return ""
}

// extractExitStatus — 提取状态后面的括号内容
func extractExitStatus(s string) string {
	if idx := strings.Index(s, "（"); idx != -1 {
		if endIdx := strings.Index(s[idx:], "）"); endIdx != -1 {
			return s[idx+1 : idx+endIdx]
		}
	}
	return ""
}
