// origin_test.go —— 构建期源提交（`version.Origin`）注入面的回归牙（缺口 `GAP-20261021-08`）。
//
// 判据（缺一即错）：
//
//	① 私有权威树（无 trailer）⇒ 取 HEAD，**定长 12 位**、不带 `+dirty`；
//	② 镜像树（HEAD 带 `GitOrigin-RevId` trailer，40 位）⇒ 取 trailer，**同样截 12 位**
//	   —— 形状不归一，主控与节点逐字比必然 `mixed`（假红）；
//	③ 不是 git 仓 ⇒ **空串**（判不了），✗ 不回落成 sha；
//	④ ldflags 组装：值为空 ⇒ 一个字节也不加（不写看着像「配了」的 `Origin=`）。
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// toyRepo 造一个最小 git 仓（提交正文 body），返回目录。
func toyRepo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("本机无 git ⇒ 跳过（本文件断言的就是 git 读法）")
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v 失败：%v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("写文件失败：%v", err)
	}
	run("add", "a.txt")
	run("commit", "-qm", body)
	return dir
}

func TestGitOrigin_PrivateTreeUsesHead(t *testing.T) {
	dir := toyRepo(t, "私有树的一笔")
	got := gitOrigin(dir, os.Environ())
	full := strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD"))
	if got != full[:originShortLen] {
		t.Fatalf("私有树应取 HEAD 前 %d 位：got %q want %q", originShortLen, got, full[:originShortLen])
	}
	if len(got) != originShortLen || strings.Contains(got, "+dirty") {
		t.Fatalf("形状必须是定长 %d 位、不带 +dirty（脏由 Digest 答）：%q", originShortLen, got)
	}
}

func TestGitOrigin_MirrorTreeUsesTrailer(t *testing.T) {
	const priv = "0123456789abcdef0123456789abcdef01234567" // 40 位私有源提交
	dir := toyRepo(t, "mirror(priv1)\n\nGitOrigin-RevId: "+priv)
	got := gitOrigin(dir, os.Environ())
	if got != priv[:originShortLen] {
		t.Fatalf("镜像树应取 trailer 前 %d 位：got %q want %q", originShortLen, got, priv[:originShortLen])
	}
	if strings.HasPrefix(got, strings.TrimSpace(mustGit(t, dir, "rev-parse", "--short=7", "HEAD"))) {
		t.Fatalf("镜像树 ✗ 取镜像 tip（那是另一套命名空间）：%q", got)
	}
}

func TestGitOrigin_NonRepoTreesIsEmpty(t *testing.T) {
	dir := t.TempDir() // 空目录：不是 git 仓
	if got := gitOrigin(dir, os.Environ()); got != "" {
		t.Fatalf("读不出源提交时必须**空串 = 判不了**（✗ 回落成 sha），实得 %q", got)
	}
}

func TestOriginShort(t *testing.T) {
	full := "0123456789abcdef"
	if got := originShort(full); got != full[:originShortLen] {
		t.Fatalf("长 sha 应截 %d 位：%q", originShortLen, got)
	}
	if got := originShort("abc"); got != "abc" {
		t.Fatalf("短值应原样：%q", got)
	}
	if got := originShort("  "); got != "" {
		t.Fatalf("空白应归空：%q", got)
	}
}

func TestWithOriginFlag(t *testing.T) {
	base := "-s -w -X p.Commit=abc"
	if got := withOriginFlag(base, "p", ""); got != base {
		t.Fatalf("值为空 ⇒ 一个字节也不加（未注入 = 空串 = 判不了）：%q", got)
	}
	if got := withOriginFlag(base, "p", "0123456789ab"); got != base+" -X p.Origin=0123456789ab" {
		t.Fatalf("注入串形状不对：%q", got)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v 失败：%v", args, err)
	}
	return string(out)
}
