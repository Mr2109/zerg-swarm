// cmd/zerg-review/main.go — v2.5.3 C4 脑确认接口
// 用法: zerg-review <workdir> <issue-id> [approve|reject]
//   approve: 确认合并（分支 merge 回 main → issue done）
//   reject:  拒绝（丢弃分支 → issue retry）
// 查看待确认: zerg-review <workdir> list

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("用法: zerg-review <workdir> <issue-id|list> [approve|reject] [-repo <repoDir>]")
		os.Exit(1)
	}
	workDir := os.Args[1]
	// 支持 -repo（DevMode 架构分离：merge 用 repoDir，读 issue 用 workDir）
	// 注意: flag.Parse 遇第一个非flag参数停止——-repo 可能在后——手动解析
	repoDir := workDir // 默认兼容（非 DevMode）
	for i, a := range os.Args {
		if a == "-repo" && i+1 < len(os.Args) {
			repoDir = os.Args[i+1]
		}
	}
	issueID := os.Args[2]

	if issueID == "list" {
		listWaiting(workDir)
		return
	}

	if len(os.Args) < 4 {
		fmt.Println("需要 approve 或 reject")
		os.Exit(1)
	}
	action := os.Args[3]

	issuePath := filepath.Join(workDir, "docs", "issues", issueID+".md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		fmt.Printf("❌ 读 issue 失败: %v\n", err)
		os.Exit(1)
	}
	text := string(content)

	// 读任务信息（分支名——issue 里找）
	branch := extractBranch(text)
	status := extractStatus(text)
	if status != agent.StatusWaitReview {
		fmt.Printf("⚠️ issue 状态=%s（需要 waiting_review）\n", status)
		os.Exit(1)
	}

	// 仓库路径（issue 里找或默认 workdir）
	repoPath := repoDir

	switch action {
	case "approve":
		// merge 分支回 main（分支不存在则报错——不标 done）
		if branch == "" {
			fmt.Printf("❌ issue 无分支字段（无法合并）——请检查任务是否开发模式\n")
			os.Exit(1)
		}
		if err := mergeBranch(repoPath, branch, issueID); err != nil {
			fmt.Printf("❌ 合并失败: %v（请检查分支 %s 是否存在）\n", err, branch)
			os.Exit(1)
		}
		// issue → done
		newContent, err := agent.TransitionIssue(text, agent.StatusWaitReview, agent.StatusDone)
		if err != nil {
			fmt.Printf("❌ 标记 done 失败: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(issuePath, []byte(newContent), 0o644)
		fmt.Printf("✅ 已确认合并: %s → main（issue done）\n", issueID)
	case "reject":
		if branch != "" {
			discardBranch(repoPath, branch)
		}
		newContent, err := agent.TransitionIssue(text, agent.StatusWaitReview, agent.StatusRetry)
		if err != nil {
			fmt.Printf("❌ 标记 retry 失败: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(issuePath, []byte(newContent), 0o644)
		fmt.Printf("❌ 已拒绝: %s（分支丢弃——issue retry）\n", issueID)
	default:
		fmt.Println("未知动作（approve/reject）")
		os.Exit(1)
	}
}

func listWaiting(workDir string) {
	dir := filepath.Join(workDir, "docs", "issues")
	entries, _ := os.ReadDir(dir)
	fmt.Println("📋 等待脑确认（waiting_review）:")
	found := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if strings.Contains(string(data), "- **状态**: waiting_review") {
			id := strings.TrimSuffix(e.Name(), ".md")
			task := extractField(string(data), "- **任务**:")
			fmt.Printf("  %s: %s\n", id, task)
			found = true
		}
	}
	if !found {
		fmt.Println("  （无待确认任务）")
	}
}

func extractBranch(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "- **分支**:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "- **分支**:"))
		}
	}
	return ""
}

func extractStatus(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "- **状态**:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "- **状态**:"))
		}
	}
	return ""
}

func extractField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func mergeBranch(repoPath, branch, issueID string) error {
	// 架构分离后：开发仓库纯代码（无任务文件）——checkout main 无冲突——不需 stash（根治）
	cmd := fmt.Sprintf("cd %s && git checkout main && git merge --no-ff %s -m 'merge: %s → main (PR %s)' && git branch -d %s", repoPath, branch, branch, issueID, branch)
	fmt.Printf("🔀 %s\n", cmd)
	return runShell(cmd)
}

func discardBranch(repoPath, branch string) {
	cmd := fmt.Sprintf("cd %s && git checkout main && git branch -D %s", repoPath, branch)
	fmt.Printf("🗑 %s\n", cmd)
	runShell(cmd)
}

func runShell(cmd string) error {
	out, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("%s（输出: %s）", err.Error(), out)
	}
	return nil
}

func runCmd(cmd string) (string, error) {
	// 用 os/exec 执行
	exe := "sh"
	args := []string{"-c", cmd}
	c := exec.Command(exe, args...)
	out, err := c.CombinedOutput()
	return string(out), err
}
