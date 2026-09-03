package agent

// commit_hook_test.go — v2.5.2 T6 生命周期 commit 测试（我重写——CA 测试烂）
// 策略: mock gitRunner（不跑真实 git——只记录调用）
// 覆盖: GitCommit 调用 / 状态变更触发 / 防重复

import (
	"strings"
	"testing"
)

// mockGitRunner — 记录 git 调用（不执行）
type mockGitRunner struct {
	calls [][]string // 每次调用的 args
}

func (m *mockGitRunner) run(repoDir string, args ...string) (string, error) {
	m.calls = append(m.calls, append([]string{repoDir}, args...))
	return "", nil
}

func (m *mockGitRunner) count() int { return len(m.calls) }

// commitCount — 只数 git commit 调用（add 不算）
func (m *mockGitRunner) commitCount() int {
	n := 0
	for _, c := range m.calls {
		for _, a := range c {
			if a == "commit" {
				n++
				break
			}
		}
	}
	return n
}

// TestGitCommit_WithRunner — GitCommit 调用 git add + commit
func TestGitCommit_WithRunner(t *testing.T) {
	mock := &mockGitRunner{}
	err := gitCommitWithRunner(mock, "/repo", "feat: 测试提交")
	if err != nil {
		t.Fatalf("GitCommit 应无错误: %v", err)
	}
	if mock.count() == 0 {
		t.Fatal("应调用 git 命令")
	}
	// 应有 add 和 commit
	all := strings.Join(mock.calls[0], " ")
	if !strings.Contains(all, "add") && !strings.Contains(all, "commit") {
		t.Fatalf("应包含 git add/commit，实际: %v", mock.calls)
	}
}

// TestCommitOnStateChange_Triggers — 状态变更触发 commit
func TestCommitOnStateChange_Triggers(t *testing.T) {
	mock := &mockGitRunner{}
	state := &commitHookState{lastCommittedState: map[string]string{}}
	workDir := "/repo"

	// open→queued 触发
	committed, err := commitOnStateChangeWithState(state, mock, workDir, "/repo/docs/issues/abc.md", "queued")
	if err != nil {
		t.Fatalf("状态变更 commit 应无错误: %v", err)
	}
	if !committed {
		t.Fatal("首次状态变更应触发 commit")
	}
	if mock.count() == 0 {
		t.Fatal("状态变更应调用 git")
	}
}

// TestCommitOnStateChange_Dedup — 同状态不重复 commit
func TestCommitOnStateChange_Dedup(t *testing.T) {
	mock := &mockGitRunner{}
	state := &commitHookState{lastCommittedState: map[string]string{}}
	workDir := "/repo"
	path := "/repo/docs/issues/abc.md"

	// 第一次 queued → commit
	committed1, _ := commitOnStateChangeWithState(state, mock, workDir, path, "queued")
	// 第二次同 queued → 不 commit（防重复）
	committed2, _ := commitOnStateChangeWithState(state, mock, workDir, path, "queued")

	if !committed1 {
		t.Fatal("第一次应 commit")
	}
	if committed2 {
		t.Fatal("同状态第二次不应 commit（防重复）")
	}
	if mock.commitCount() != 1 {
		t.Fatalf("应只 commit 1 次，实际 %d", mock.commitCount())
	}
}

// TestCommitOnStateChange_NewState — 新状态触发新 commit
func TestCommitOnStateChange_NewState(t *testing.T) {
	mock := &mockGitRunner{}
	state := &commitHookState{lastCommittedState: map[string]string{}}
	workDir := "/repo"
	path := "/repo/docs/issues/abc.md"

	commitOnStateChangeWithState(state, mock, workDir, path, "queued")
	committed2, _ := commitOnStateChangeWithState(state, mock, workDir, path, "running")

	if !committed2 {
		t.Fatal("新状态应再次 commit")
	}
	if mock.commitCount() != 2 {
		t.Fatalf("两个状态应 commit 2 次，实际 %d", mock.commitCount())
	}
}
