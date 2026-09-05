package subtask

// verify.go — 契约逐条核验（2026-09-05 从 api/contract.go 移植——契约的家在 subtask）
// api 包保留 VerifyTaskOutput 任务级旧逻辑——阶段级核验统一走此处

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VerifyResult — 核验结果
type VerifyResult struct {
	Pass     bool     `json:"pass"`
	Checks   []string `json:"checks"`
	Failures []string `json:"failures"`
}

// Verify — 按契约逐条核验（全部通过才 Pass）
func (c *Contract) Verify(worktreeDir, workdir string) *VerifyResult {
	res := &VerifyResult{Pass: true}
	fail := func(msg string) {
		res.Failures = append(res.Failures, msg)
		res.Pass = false
	}
	base := worktreeDir
	if base == "" {
		base = workdir
	}

	// ① 必写文件（存在且非空）
	for _, f := range c.MustWriteFiles {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, f)
		}
		info, err := os.Stat(p)
		if err != nil {
			fail(fmt.Sprintf("契约文件缺失: %s", f))
			continue
		}
		if info.Size() == 0 {
			fail(fmt.Sprintf("契约文件为空: %s", f))
			continue
		}
		res.Checks = append(res.Checks, fmt.Sprintf("契约文件存在: %s（%d 字节）", f, info.Size()))
	}

	// ② 必过命令（工作目录内执行——退出码 0）
	for _, cmdStr := range c.MustPassCmds {
		cmd := exec.Command("bash", "-c", cmdStr)
		cmd.Dir = base
		out, err := cmd.CombinedOutput()
		if err != nil {
			fail(fmt.Sprintf("契约命令不过: %s（%s）", cmdStr, tailStr(string(out), 200)))
			continue
		}
		res.Checks = append(res.Checks, fmt.Sprintf("契约命令通过: %s", cmdStr))
	}

	// ③ 必有 diff 的路径 + 最少改动文件数（git 任务）
	if len(c.MustDiffPaths) > 0 || c.MinChangedFiles > 0 {
		if base != "" && gitHasChanges(base) {
			changed := gitChangedFiles(base)
			if c.MinChangedFiles > 0 && len(changed) < c.MinChangedFiles {
				fail(fmt.Sprintf("改动文件数不足: %d < %d", len(changed), c.MinChangedFiles))
			}
			for _, pref := range c.MustDiffPaths {
				hit := false
				for _, f := range changed {
					if strings.HasPrefix(f, pref) {
						hit = true
						break
					}
				}
				if hit {
					res.Checks = append(res.Checks, fmt.Sprintf("契约 diff 命中: %s", pref))
				} else {
					fail(fmt.Sprintf("契约路径无改动: %s", pref))
				}
			}
		} else {
			fail("git 无真实改动（契约 diff 检查不过）")
		}
	}
	return res
}

func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func gitHasChanges(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return true // 非 git——不误判
	}
	if strings.TrimSpace(string(out)) != "" {
		return true
	}
	cmd2 := exec.Command("git", "-C", dir, "log", "--oneline", "-3")
	out2, err2 := cmd2.Output()
	return err2 == nil && strings.TrimSpace(string(out2)) != ""
}

func gitChangedFiles(dir string) []string {
	var out []string
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	if b, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 3 {
				out = append(out, strings.TrimPrefix(line[3:], "\""))
			}
		}
	}
	cmd2 := exec.Command("git", "-C", dir, "diff", "--name-only", "main...HEAD")
	if b, err := cmd2.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
	}
	seen := map[string]bool{}
	var uniq []string
	for _, f := range out {
		if !seen[f] {
			seen[f] = true
			uniq = append(uniq, f)
		}
	}
	return uniq
}
