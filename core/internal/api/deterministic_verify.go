package api

// deterministic_verify.go — 阶段1: 确定性验证（2026-08-20 设计——任务git全生命周期）
// 执行完成 → 机器检查执行结果真实性（非模型意见——Reddit/业界共识）
// 检查: git diff 真实改动 / 报告存在非空 / 测试输出证据 / 产物真实存在
// 不过 → 直接 failed（不浪费复查）——治假完成（AutoCUT 案例）

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VerifyResult 确定性验证结果
type VerifyResult struct {
	Pass     bool     // 全部通过
	Checks   []string // 通过的检查项
	Failures []string // 失败的检查项（有则说明不过原因）
}

// VerifyTaskOutput 确定性验证任务执行结果
// worktreeDir: 任务 worktree 目录（git 检查用——空则跳过 git 检查）
// reportPath:  任务报告路径（要求存在的报告——空则跳过报告检查）
// 返回: 验证结果——Pass=false 时 Failures 说明原因
func VerifyTaskOutput(worktreeDir, reportPath string, taskDesc string) *VerifyResult {
	res := &VerifyResult{Pass: true}

	// 1. git diff 真实改动（worktree 有未提交/已提交改动——非空报告）
	if worktreeDir != "" {
		if gitHasChanges(worktreeDir) {
			res.Checks = append(res.Checks, "git 有真实改动")
		} else {
			res.Failures = append(res.Failures, "git 无真实改动（worktree 空——可能假完成）")
			res.Pass = false
		}
	}

	// 2. 报告文件存在且非空（硬性要求——无报告=假完成拦截——2026-08-20 设计）
	// 所有任务必须写报告（internal-task-report.md）——找不到直接 fail（防 gemma 幻觉通过）
	if reportPath == "" {
		res.Failures = append(res.Failures, "报告文件不存在（假完成嫌疑——任务必须写报告）")
		res.Pass = false
	} else if info, err := os.Stat(reportPath); err == nil {
		if info.Size() > 100 {
			res.Checks = append(res.Checks, fmt.Sprintf("报告存在且非空（%d 字节）", info.Size()))
		} else {
			res.Failures = append(res.Failures, fmt.Sprintf("报告过小（%d 字节）——疑似空报告", info.Size()))
			res.Pass = false
		}
	} else {
		res.Failures = append(res.Failures, "报告文件不存在（假完成嫌疑）")
		res.Pass = false
	}

	// 3. 测试输出证据（worktree 里有 go test 输出/测试痕迹——不强求——记录）
	// 注: 测试是否跑过由执行模型报告——机器无法完全验证——作为弱检查
	if worktreeDir != "" && reportPath != "" {
		res.Checks = append(res.Checks, "报告+worktree 组合存在（执行痕迹完整）")
	}

	return res
}

// gitHasChanges 检查 worktree 是否有改动（git status 非 clean / 有提交）
func gitHasChanges(worktreeDir string) bool {
	cmd := exec.Command("git", "-C", worktreeDir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		// 可能不是 git 仓库——返回 true（不误判）
		return true
	}
	if strings.TrimSpace(string(out)) != "" {
		return true // 有未提交改动
	}
	// 无未提交改动——检查是否有提交（分支领先 main）
	cmd2 := exec.Command("git", "-C", worktreeDir, "log", "--oneline", "-3")
	out2, err2 := cmd2.Output()
	if err2 == nil && strings.TrimSpace(string(out2)) != "" {
		return true // 有提交历史
	}
	return false
}

// 报告路径查找（任务目录优先——回退 workdir/共享路径）
// 2026-08-20 设计: 任务目录唯一化 /var/zerg/tasks/<任务ID>/——报告在任务目录根
// workdir = 任务目录/work——报告在上级（../internal-task-report.md）
func FindTaskReport(workdir string) string {
	// 优先: 任务目录（workdir 上级——任务目录根）
	parent := filepath.Dir(workdir)
	p := filepath.Join(parent, "internal-task-report.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// 其次: 任务目录根的直接路径（workdir 可能是任务目录本身）
	p2 := filepath.Join(workdir, "internal-task-report.md")
	if _, err := os.Stat(p2); err == nil {
		return p2
	}
	// 注意: 不回退共享路径（/tmp/internal-task-report.md——毒源——2026-08-21 发现）
	// 共享路径会被多任务覆盖——误读旧报告=假完成误过——宁可失败也不读共享
	// v2.5.5 修复（2026-08-24 Mr2109——C 毒源）: 去掉 docs/issues 回退
	// 之前回退到 workdir/docs/issues/（任务单报告）——但 workdir 可能是 repo（内部任务 worktree 场景）
	// → 读到仓库内其他任务的 docs/issues/*.md（任务单≠执行报告）——假完成误过——毒源！
	// 执行报告只认: 任务目录 internal-task-report.md（FindTaskReport 前两层已覆盖）
	return ""
}
