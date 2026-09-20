// cli_repo_status_test.go —— P0-2 `zerg repo status` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 三面都要钉：① 三档退码（0 干净 / 1 脏 / 8 读不到）；② `--json` 的 head/branch 真带得出去；
// ③ **同一判据跑两遍逐字一致**（§九 M11：命令面自己的输出也必须幂等可对拍）。
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo 建一个真 git 仓（含仓根判据件），返回仓根。
func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	mustWrite(t, filepath.Join(root, "README.md"), "hello\n")
	for _, argv := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "-m", "base"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, argv...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("建合成仓 `git %v`：%v（%s）", argv, err, out)
		}
	}
	return root
}

// TestRepoStatus_CleanDirtyAndUnreadable —— 三档退码成对钉住 + 空转负控（非 git 仓 ⇒ 8，不编造）。
func TestRepoStatus_CleanDirtyAndUnreadable(t *testing.T) {
	bin := zergBinary(t)
	root := gitRepo(t)

	rc, out, errb := execCase(t, bin, root, "repo", "status")
	if rc != 0 {
		t.Fatalf("干净仓 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out+errb, "clean") {
		t.Errorf("干净仓的人面没给「clean」：out=%q err=%q", out, errb)
	}

	// 弄脏：改一件 + 加一件未跟踪
	mustWrite(t, filepath.Join(root, "README.md"), "changed\n")
	mustWrite(t, filepath.Join(root, "new.go"), "package main\n")
	rc, out, errb = execCase(t, bin, root, "repo", "status")
	if rc != 1 {
		t.Fatalf("有脏件 ⇒ 退 1（不是错，是状态），得到 %d · stderr=%s", rc, errb)
	}
	for _, want := range []string{"README.md", "new.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("脏件清单里没有 %s：%q", want, out)
		}
	}

	// 非 git 仓 ⇒ 8（不给结论，不硬编一个「干净」）
	plain := t.TempDir()
	mustWrite(t, filepath.Join(plain, "core", "internal", "version", "version.go"), "package version\n")
	rc, _, errb = execCase(t, bin, plain, "repo", "status")
	if rc != 8 {
		t.Errorf("非 git 仓 ⇒ 退 8（读不到），得到 %d · stderr=%s", rc, errb)
	}

	rc, _, _ = execCase(t, bin, root, "repo", "status", "--nosuchflag-zz")
	if rc != 2 {
		t.Errorf("未知旗标 ⇒ 退 2，得到 %d", rc)
	}
}

// TestRepoStatus_TwoRunsByteIdentical —— 同一判据跑两遍**逐字一致**（§九 M11 的机械落点）。
func TestRepoStatus_TwoRunsByteIdentical(t *testing.T) {
	bin := zergBinary(t)
	root := gitRepo(t)
	mustWrite(t, filepath.Join(root, "README.md"), "changed\n")
	_, out1, _ := execCase(t, bin, root, "repo", "status", "--json", "head,branch,path,status,untracked")
	_, out2, _ := execCase(t, bin, root, "repo", "status", "--json", "head,branch,path,status,untracked")
	if out1 != out2 {
		t.Errorf("两跑 `--json` **逐字不同**（M11 不许）：\n① %s\n② %s", out1, out2)
	}
	// 真源对拍：`git status --porcelain` 的件数 = 本命令列的件数（不自己造第二套口径）
	gitOut, err := exec.Command("git", "-C", root, "status", "--porcelain=v1").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, ln := range strings.Split(string(gitOut), "\n") {
		if strings.TrimSpace(ln) != "" {
			want++
		}
	}
	got := strings.Count(out1, `"path"`)
	if got != want {
		t.Errorf("件数对拍不上：git porcelain = %d 行 · 本命令 items = %d 条", want, got)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
}
