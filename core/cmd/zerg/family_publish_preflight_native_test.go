package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pfTestLeakRepo —— 仓外夹具：造一个临时 git 仓（branch main），把每个 content 各成一笔提交。
// 闸③ 只读 `git rev-list main` 的可达提交 ⇒ 夹具提交都挂在 main 上。
func pfTestLeakRepo(t *testing.T, contents ...string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	for i, c := range contents {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", ".")
		git("commit", "-q", "-m", "c")
	}
	return dir
}

// TestPublishPreflightLeakScanCap —— 闸③ 扫描上限两态：
// (甲) 正常树/小树 ⇒ 判据行为与改前逐字相同（无假阳假阴）；
// (乙) 必然超限的输入 ⇒ 走「判不了」档（不给结论 · fail-closed），绝不静默通过。
func TestPublishPreflightLeakScanCap(t *testing.T) {
	t.Run("甲_正常树行为逐字不变", func(t *testing.T) {
		repo := pfTestLeakRepo(t, "hello\n", "world\n")
		for _, cap := range []int{2, 3, pfLeakScanCommitCap} {
			rc, out := pfLeakScanCapped(repo, cap)
			if rc != 0 {
				t.Fatalf("cap=%d 正常树应 rc=0，得到 rc=%d out=%q", cap, rc, out)
			}
			if !strings.Contains(out, "提交数=2 命中=0（应 0）") {
				t.Fatalf("cap=%d 判据输出与改前不同形：%q", cap, out)
			}
		}
		// 生产入口（恒传常量）同形。
		if rc, out := pfLeakScan(repo); rc != 0 || !strings.Contains(out, "提交数=2 命中=0（应 0）") {
			t.Fatalf("生产入口 pfLeakScan 应 rc=0 同形，得 rc=%d out=%q", rc, out)
		}
	})

	t.Run("甲_泄漏仍报1", func(t *testing.T) {
		repo := pfTestLeakRepo(t, "clean\n", "owner Mr2109 here\n")
		rc, out := pfLeakScanCapped(repo, pfLeakScanCommitCap)
		if rc != 1 {
			t.Fatalf("含泄漏应 rc=1，得到 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "命中=1（应 0）") {
			t.Fatalf("泄漏档输出与改前不同形：%q", out)
		}
	})

	t.Run("乙_超限判不了", func(t *testing.T) {
		repo := pfTestLeakRepo(t, "a\n", "b\n", "c\n") // 3 笔
		rc, out := pfLeakScanCapped(repo, 2)           // cap < 3 ⇒ 必然超限
		if rc != pfLeakScanOverCapRC {
			t.Fatalf("超限应走判不了档 rc=%d，得到 rc=%d out=%q", pfLeakScanOverCapRC, rc, out)
		}
		if rc == 0 {
			t.Fatal("超限不许静默通过（rc=0）")
		}
		if !strings.Contains(out, "超扫描上限 2") || !strings.Contains(out, "判不了") {
			t.Fatalf("超限判词缺：%q", out)
		}
	})

	t.Run("乙_边界等于cap放行", func(t *testing.T) {
		repo := pfTestLeakRepo(t, "a\n", "b\n", "c\n") // 3 笔
		rc, out := pfLeakScanCapped(repo, 3)           // cap == 3 ⇒ 不超限（≤ cap）
		if rc != 0 {
			t.Fatalf("cap=提交数应放行 rc=0，得到 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "提交数=3 命中=0（应 0）") {
			t.Fatalf("边界档输出异形：%q", out)
		}
	})
}
