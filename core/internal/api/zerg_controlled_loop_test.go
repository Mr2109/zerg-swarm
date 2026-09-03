package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestVerifyWithCmd 验证命令（exit 0 = 通过）
func TestVerifyWithCmd(t *testing.T) {
	dir := t.TempDir()
	c := NewControlledLoop(dir)
	// 真命令（echo 成功）
	c.WithVerifyCmd("echo", "ok")
	ok, ev, err := c.Verify()
	if err != nil || !ok {
		t.Fatalf("echo 应通过: ok=%v err=%v ev=%s", ok, err, ev)
	}
	// 假命令（失败）
	c.WithVerifyCmd("false")
	ok, _, _ = c.Verify()
	if ok {
		t.Fatalf("false 应失败")
	}
	// 无命令——v2.5.6 放宽: 模型响应即过（不要求工作区变化）
	c2 := NewControlledLoop(dir)
	ok2, ev2, _ := c2.Verify()
	if !ok2 {
		t.Fatalf("无验证命令应通过（v2.5.6 放宽——提交层兜底）: %s", ev2)
	}
	// 写个文件——有变化
	f := filepath.Join(dir, "x.txt")
	_ = os.WriteFile(f, []byte("hi"), 0o644)
	// 需要 git 仓库才识别变化——先 init
	_ = exec.Command("git", "-C", dir, "init", "-q").Run()
	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	ok3, ev3, _ := c2.Verify()
	if !ok3 {
		t.Fatalf("有文件应通过: %s", ev3)
	}
}

func TestControlledLoopRunStep(t *testing.T) {
	dir := t.TempDir()
	c := NewControlledLoop(dir)
	c.MaxRounds = 3
	c.WithVerifyCmd("test", "-f", filepath.Join(dir, "done.txt"))

	// 模型 2 轮后创建文件（模拟成功）
	attempts := 0
	res, err := c.RunStep("step1", func(step, feedback string) (string, error) {
		attempts++
		if attempts >= 2 {
			_ = os.WriteFile(filepath.Join(dir, "done.txt"), []byte("done"), 0o644)
		}
		return fmt.Sprintf("尝试 %d", attempts), nil
	})
	if err != nil {
		t.Fatalf("RunStep 失败: %v", err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	if res.RoundsUsed != 2 {
		t.Fatalf("应用 2 轮: %d", res.RoundsUsed)
	}
}

func TestControlledLoopRebuild(t *testing.T) {
	dir := t.TempDir()
	c := NewControlledLoop(dir)
	c.MaxRounds = 2
	c.WithVerifyCmd("test", "-f", filepath.Join(dir, "never.txt"))

	// 模型永远不创建文件——超限——NeedRebuild
	res, err := c.RunStep("stepX", func(step, feedback string) (string, error) {
		return "我做了", nil
	})
	if err != nil {
		t.Fatalf("RunStep 失败: %v", err)
	}
	if !res.NeedRebuild {
		t.Fatalf("应 NeedRebuild: %+v", res)
	}
	if res.FailReason == "" {
		t.Fatalf("应有失败原因")
	}
}
