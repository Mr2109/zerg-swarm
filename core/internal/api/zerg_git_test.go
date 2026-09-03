package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZergGitEnsureAndCommit(t *testing.T) {
	dir := t.TempDir()
	z, err := NewZergGit(dir)
	if err != nil {
		t.Fatalf("NewZergGit 失败: %v", err)
	}
	// init commit 存在
	if z.CommitCount() < 1 {
		t.Fatalf("应有 init commit: %d", z.CommitCount())
	}
	// 写文件 + 每轮提交
	_ = os.WriteFile(filepath.Join(dir, "task.jsonl"), []byte("{\"round\":1}\n"), 0o644)
	if err := z.CommitRound(1, "plan", "出了3个方案"); err != nil {
		t.Fatalf("CommitRound 失败: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "task.jsonl"), []byte("{\"round\":1}\n{\"round\":2}\n"), 0o644)
	if err := z.CommitRound(2, "select", "选定方案A"); err != nil {
		t.Fatalf("CommitRound2 失败: %v", err)
	}
	if z.CommitCount() != 3 {
		t.Fatalf("应 3 commit（init+r1+r2）: %d", z.CommitCount())
	}
	// 时间线（git log——结构化）
	log, _ := z.Log()
	if !strings.Contains(log, "r01") || !strings.Contains(log, "r02") {
		t.Fatalf("时间线缺轮次: %s", log)
	}
}

func TestZergGitEmptyCommitBlocked(t *testing.T) {
	dir := t.TempDir()
	z, _ := NewZergGit(dir)
	// v2.5.6 放宽: 空提交用 --allow-empty（验证类步骤——模型已响应——git 树前进）
	// 防假完成靠"受控循环验证"（模型响应）——不是强制文件变化
	if err := z.CommitRound(1, "plan", "验证步骤——无新文件"); err != nil {
		t.Fatalf("空提交应允许（v2.5.6 放宽）: %v", err)
	}
	if z.CommitCount() != 2 {
		t.Fatalf("应有 init+1 提交: %d", z.CommitCount())
	}
}

func TestZergGitRevert(t *testing.T) {
	dir := t.TempDir()
	z, _ := NewZergGit(dir)
	// r1: 写对的东西
	_ = os.WriteFile(filepath.Join(dir, "good.txt"), []byte("good"), 0o644)
	_ = z.CommitRound(1, "execute", "完成step1")
	// r2: 写错的东西
	_ = os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("bad"), 0o644)
	_ = z.CommitRound(2, "execute", "完成step2——做错了")
	// 回溯到 r1（git 回溯重做）
	target, err := z.RevertTo(1)
	if err != nil {
		t.Fatalf("RevertTo 失败: %v", err)
	}
	if target == "" {
		t.Fatalf("应返回回溯 commit")
	}
	// bad.txt 应没了（回到 r1 状态）
	if _, err := os.Stat(filepath.Join(dir, "bad.txt")); err == nil {
		t.Fatalf("回溯后 bad.txt 应不存在")
	}
	if _, err := os.Stat(filepath.Join(dir, "good.txt")); err != nil {
		t.Fatalf("回溯后 good.txt 应在: %v", err)
	}
}

func TestCommitRoundWithFS(t *testing.T) {
	dir := t.TempDir()
	z, _ := NewZergGit(dir)
	zf := NewZergTaskFile(dir)
	_ = zf.Init("task-fs", "internal", "目标")
	// 方案轮（JSONL + git 联动）
	n, err := CommitRoundWithFS(zf, z, PhasePlan, "", map[string]interface{}{"plans": []string{"A"}}, "出方案")
	if err != nil {
		t.Fatalf("CommitRoundWithFS 失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("首轮应 1: %d", n)
	}
	// task.jsonl 有内容
	data, _ := os.ReadFile(filepath.Join(dir, "task.jsonl"))
	if !strings.Contains(string(data), "出方案") {
		t.Fatalf("task.jsonl 应含总结")
	}
	// git 时间线有 r01
	log, _ := z.Log()
	if !strings.Contains(log, "r01") {
		t.Fatalf("git 时间线应含 r01: %s", log)
	}
}
