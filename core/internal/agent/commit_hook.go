package agent

// commit_hook.go — v2.5.2 生命周期自动 commit（git 全程留痕）
// 设计：P0 状态 6 — 新任务挂单 / 状态变更 / 完成结论 → 各 commit 一次
// 依赖：os/exec（git 命令）、sync（防重复）、strings

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ─── GitRunner 接口（可注入——测试 mock）─────────────────────

// gitRunner — git 命令执行接口（生产用 realGitRunner，测试用 mock）
type gitRunner interface {
	run(repoDir string, args ...string) (string, error)
}

// realGitRunner — 真实 git 命令执行
type realGitRunner struct{}

func (r *realGitRunner) run(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n输出: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// commitHookState — 防重复 commit 状态

// commitHookState — 跟踪每个 issue 的 lastCommittedState（防重复 commit）
type commitHookState struct {
	mu              sync.Mutex
	lastCommittedState map[string]string // issuePath → lastCommittedState
}

var hookState = &commitHookState{
	lastCommittedState: make(map[string]string),
}

// GitCommit — 通用 git commit

// GitCommit — 在指定仓库执行 git add -A + git commit
// repoDir: git 仓库根目录
// message: commit message（支持中文）
// 失败返回 error（不 panic）
func GitCommit(repoDir, message string) error {
	return gitCommitWithRunner(&realGitRunner{}, repoDir, message)
}

// gitCommitWithRunner — 内部实现（注入 gitRunner）
func gitCommitWithRunner(gr gitRunner, repoDir, message string) error {
	if repoDir == "" {
		return fmt.Errorf("repoDir 不能为空")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit message 不能为空")
	}

	// git add -A
	if _, err := gr.run(repoDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add -A 失败: %w", err)
	}

	// git commit -m
	if _, err := gr.run(repoDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit 失败: %w", err)
	}

	return nil
}

// CommitOnStateChange — 状态变更时自动 commit

// CommitOnStateChange — testonly: 导出仅测试用，无外部调用者（见 commit_hook_test.go）
// issue 状态变更时触发 commit
// workDir: 工作区根目录（issue 文件所在仓库）
// issuePath: issue 文件相对路径（如 "docs/issues/20260814-150405-abc1.md"）
// newState: 新状态（如 "queued" / "running" / "done"）
// 返回 (committed bool, error) — committed 表示是否真正执行了 commit
func CommitOnStateChange(workDir, issuePath, newState string) (bool, error) {
	return commitOnStateChangeWithState(hookState, &realGitRunner{}, workDir, issuePath, newState)
}

// commitOnStateChangeWithState — 内部实现（注入 state 和 gitRunner）
func commitOnStateChangeWithState(state *commitHookState, gr gitRunner, workDir, issuePath, newState string) (bool, error) {
	if workDir == "" {
		return false, fmt.Errorf("workDir 不能为空")
	}
	if issuePath == "" {
		return false, fmt.Errorf("issuePath 不能为空")
	}
	if newState == "" {
		return false, fmt.Errorf("newState 不能为空")
	}

	// 防重复 commit（同状态不重复）
	state.mu.Lock()
	lastState, exists := state.lastCommittedState[issuePath]
	if exists && lastState == newState {
		state.mu.Unlock()
		return false, nil // 同状态，跳过 commit
	}
	state.lastCommittedState[issuePath] = newState
	state.mu.Unlock()

	// 构造 commit message：task: issue-xxx open→queued
	issueName := extractIssueName(issuePath)
	message := fmt.Sprintf("task: %s %s→%s", issueName, lastStateOrUnknown(lastState, exists), newState)

	// 执行 commit
	if err := gitCommitWithRunner(gr, workDir, message); err != nil {
		return false, err
	}

	return true, nil
}

// extractIssueName — 从 issue 路径提取短名（如 "20260814-150405-abc1"）
func extractIssueName(issuePath string) string {
	// 取最后一个 "/" 后的部分，去掉 .md 后缀
	name := issuePath
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if strings.HasSuffix(name, ".md") {
		name = name[:len(name)-3]
	}
	return name
}

// lastStateOrUnknown — 返回上次状态或 "unknown"
func lastStateOrUnknown(lastState string, exists bool) string {
	if exists && lastState != "" {
		return lastState
	}
	return "unknown"
}

// ResetCommitHookState — testonly: 仅测试用（见 commit_hook_test.go），不删除
// 测试用：重置防重复状态
func ResetCommitHookState() {
	hookState.mu.Lock()
	defer hookState.mu.Unlock()
	hookState.lastCommittedState = make(map[string]string)
}
