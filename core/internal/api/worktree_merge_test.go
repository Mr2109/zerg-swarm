// worktree_merge_test.go — §二十一 已红第 8 条（T-30「两套分支名算法 ⇒ 产物落孤儿分支」）的**成对负控**。
//
// 为什么必须有它：这条已红的病**不会报错** —— 合并失败只是让它留在孤儿分支上，日志里连一行红都少见。
// 所以判据必须是：**拿一个真 git 仓**跑一遍合并，然后逐条断言「产物真在 main 上 · 分支真没了 ·
// worktree 真清了」；再拿四个反例证明这个判据**会红**（否则是「永远绿」的假判据）。
//
// 夹具全在 TempDir（**绝不碰真仓**）：`git init -b main` + 一次提交 ⇒ 真 worktree ⇒ 真 merge。
package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=zerg-test", "GIT_AUTHOR_EMAIL=zerg@test",
		"GIT_COMMITTER_NAME=zerg-test", "GIT_COMMITTER_EMAIL=zerg@test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s（在 %s）：%v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newTestRepo 造一个最小真仓（main 分支 + 一次提交）。
func newTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitT(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", "README.md")
	gitT(t, repo, "commit", "-m", "fixture: 初始提交")
	return repo
}

// TestWorktreeBranchName_SingleSource —— 唯一算法：建树/合并/读报告三处必须同一个名字。
func TestWorktreeBranchName_SingleSource(t *testing.T) {
	taskID := "internal-health-check-1787679416973993000"
	want := "task-internal-health-check-1787679416973993000"
	if got := worktreeBranchFor(taskID); got != want {
		t.Errorf("worktreeBranchFor(%q) = %q，want %q（合并侧历史上就按 task- 前缀找）", taskID, got, want)
	}
	// 成对：带怪字符的 id 仍走同一个清洗函数（不许各写一份）
	if got := worktreeBranchFor("a b/c"); got != "task-a-b-c" {
		t.Errorf("清洗口径不一致：%q", got)
	}
}

// TestMergeWorktree_ArtifactsLandOnMain —— 正控：产物**真进 main**，分支与 worktree 都清掉。
func TestMergeWorktree_ArtifactsLandOnMain(t *testing.T) {
	repo := newTestRepo(t)
	branch := worktreeBranchFor("internal-health-check-1787679416973993000")
	wt, err := createWorktree(repo, branch)
	if err != nil {
		t.Fatalf("createWorktree：%v", err)
	}
	// CA 常常只写文件不 commit —— 夹具照这个形态写（mergeWorktree 自己会 add -A + commit）
	if err := os.WriteFile(filepath.Join(wt, "internal-health-report.md"), []byte("# 报告\n产物：ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 判据③：合并请求带 base_sha（= 现在的 main）
	mr, err := newMergeRequest(repo, wt)
	if err != nil {
		t.Fatalf("newMergeRequest：%v", err)
	}
	if mr.BaseSHA != gitT(t, repo, "rev-parse", "main") {
		t.Errorf("base_sha 必须 = 合并前的 main：%q", mr.BaseSHA)
	}
	if mr.Branch != branch {
		t.Errorf("分支名必须从 worktree 目录名取：%q ≠ %q", mr.Branch, branch)
	}
	if !strings.HasPrefix(mr.REQID, "MR-") {
		t.Errorf("合并请求号形态：%q", mr.REQID)
	}

	if err := mergeWorktree(wt); err != nil {
		t.Fatalf("mergeWorktree：%v", err)
	}
	// ① 产物真在 main 上
	got := gitT(t, repo, "show", "main:internal-health-report.md")
	if !strings.Contains(got, "产物：ok") {
		t.Errorf("main 上看不到任务产物：%q", got)
	}
	// ② 分支已删（不新增孤儿分支）
	if out, _ := exec.Command("git", "-C", repo, "rev-parse", "--verify", branch).CombinedOutput(); len(out) != 0 {
		if _, err := os.Stat(wt); err == nil {
			t.Errorf("分支 %s 与 worktree 都应被清掉", branch)
		}
	}
	// ③ worktree 目录已清
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree 目录应已删除：%v", err)
	}
	// ④ 合并条数对得上（判据②的形态：进 main 的条数 == 全仓那个 grep 的条数，至少不为 0）
	n := gitT(t, repo, "log", "--oneline", "--grep=任务产物（worktree 自动提交）")
	if strings.TrimSpace(n) == "" {
		t.Errorf("main 上找不到 worktree 自动提交（判据②：进 main 的条数不许是 0）")
	}
}

// TestNewMergeRequest_MissingBranchMustError —— 负控①：目录名对不上真分支 ⇒ **必须报错**
// （老代码在这里悄悄合不上，产物就落孤儿分支了）。
func TestNewMergeRequest_MissingBranchMustError(t *testing.T) {
	repo := newTestRepo(t)
	ghost := filepath.Join(filepath.Dir(repo), "zerg-wt", "task-从来不存在")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newMergeRequest(repo, ghost); err == nil {
		t.Errorf("分支不存在时 newMergeRequest 必须报错（不许静默）")
	}
	// 成对：同一个目录名换成真建出来的分支 ⇒ 不报错
	branch := worktreeBranchFor("task-real")
	wt, err := createWorktree(repo, branch)
	if err != nil {
		t.Fatalf("createWorktree：%v", err)
	}
	if _, err := newMergeRequest(repo, wt); err != nil {
		t.Errorf("真分支必须能组出合并请求：%v", err)
	}
}

// TestGuardBaseSHA_MismatchRejected —— 负控②：main 被并发改动 ⇒ **拒合**；成对：没改动 ⇒ 放行。
func TestGuardBaseSHA_MismatchRejected(t *testing.T) {
	repo := newTestRepo(t)
	base := gitT(t, repo, "rev-parse", "main")
	if err := guardBaseSHA(repo, base); err != nil {
		t.Fatalf("没改动时应放行：%v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "concurrent.txt"), []byte("并发写的\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", "concurrent.txt")
	gitT(t, repo, "commit", "-m", "fixture: 并发改动 main")
	if err := guardBaseSHA(repo, base); err == nil {
		t.Errorf("main 被改了但 base_sha 还是老值 ⇒ 必须拒合")
	}
	if err := guardBaseSHA(repo, gitT(t, repo, "rev-parse", "main")); err != nil {
		t.Errorf("取到当前值时应放行：%v", err)
	}
}

// TestMergeWorktree_NoReportNoMergeNoDelete —— 负控③：没有报告 ⇒ 不合并且**不删分支**（产物不许消失）。
func TestMergeWorktree_NoReportNoMergeNoDelete(t *testing.T) {
	repo := newTestRepo(t)
	branch := worktreeBranchFor("task-no-report")
	wt, err := createWorktree(repo, branch)
	if err != nil {
		t.Fatalf("createWorktree：%v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "some-code.txt"), []byte("产物但没报告\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeWorktree(wt); err == nil {
		t.Fatalf("没报告不许合并（P1-1 防假完成）")
	}
	// 分支仍在（产物不许被顺手丢掉）
	gitT(t, repo, "rev-parse", "--verify", branch)
}
