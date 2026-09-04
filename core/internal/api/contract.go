package api

// contract.go — TaskContract 任务契约（2026-09-05 评估报告建议 3——治本假完成）
// 根因: 「完成」无唯一权威定义——done 词/报告存在/复查关键词六来源互相打架
// 治本: 任务提交时结构化声明验收项（机器可判定）→ Loop 结束按契约逐条核验
//       复查模型只审机器验不了的质量维度——结论走结构化字段非报告关键词
// 契约来源: 任务描述中的【验收】段（JSON）——无契约任务走旧启发式（向后兼容）

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TaskContract — 任务验收契约
type TaskContract struct {
	// 必须存在的文件（相对 workdir——存在且非空即过）
	MustWriteFiles []string `json:"must_write_files,omitempty"`
	// 必须通过的命令（退出码 0 即过——如 go test ./... / cargo check）
	MustPassCmds []string `json:"must_pass_cmds,omitempty"`
	// 必须有 diff 的路径前缀（git 任务——该路径下须有真实改动）
	MustDiffPaths []string `json:"must_diff_paths,omitempty"`
	// 最少改动文件数（git 任务——防只写报告不改码——0=不检查）
	MinChangedFiles int `json:"min_changed_files,omitempty"`
}

// ParseContract — 从任务描述提取契约（【验收】{...JSON...} 段——无则 nil=旧启发式）
func ParseContract(taskDesc string) *TaskContract {
	idx := strings.Index(taskDesc, "【验收】")
	if idx < 0 {
		return nil
	}
	rest := taskDesc[idx+len("【验收】"):]
	// 提取第一个完整 JSON 对象
	start := strings.Index(rest, "{")
	if start < 0 {
		return nil
	}
	depth := 0
	end := -1
	for i, r := range rest[start:] {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = start + i + 1
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	var c TaskContract
	if err := json.Unmarshal([]byte(rest[start:end]), &c); err != nil {
		return nil
	}
	if len(c.MustWriteFiles) == 0 && len(c.MustPassCmds) == 0 && len(c.MustDiffPaths) == 0 && c.MinChangedFiles == 0 {
		return nil
	}
	return &c
}

// Verify — 按契约逐条核验（全部通过才 Pass）
func (c *TaskContract) Verify(worktreeDir, workdir string) *VerifyResult {
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
			fail(fmt.Sprintf("契约命令不过: %s（%s）", cmdStr, tail(string(out), 200)))
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

// gitChangedFiles — worktree 内改动文件清单（未提交+已提交 vs main）
func gitChangedFiles(dir string) []string {
	var out []string
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	if b, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 状态码两字符 + 空格 + 路径
			if len(line) > 3 {
				out = append(out, strings.TrimPrefix(line[3:], "\""))
			}
		}
	}
	// 已提交的（分支领先 main——重跑场景）
	cmd2 := exec.Command("git", "-C", dir, "diff", "--name-only", "main...HEAD")
	if b, err := cmd2.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
	}
	// 去重
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
