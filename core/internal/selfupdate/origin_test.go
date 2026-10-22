// origin_test.go —— 构建期源提交（`-X …Origin=`）从**目标树**取值的回归牙（GAP-20261021-08）。
//
// 判据：① 私有树（无 trailer）⇒ HEAD 前 12 位；② 镜像树（带 `GitOrigin-RevId`）⇒ trailer 前
// 12 位（不是镜像 tip）；③ 非 git 树 ⇒ 空串 = 判不了。
package selfupdate

import (
	"strings"
	"testing"
)

func TestBuildOrigin_PrivateTree(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	head := commitFile(t, dir, "a.txt", "1\n", "私有树的一笔")
	if got := buildOrigin(dir); got != head[:BuildOriginShortLen] {
		t.Fatalf("私有树应取 HEAD 前 %d 位：got %q want %q", BuildOriginShortLen, got, head[:BuildOriginShortLen])
	}
}

func TestBuildOrigin_MirrorTree(t *testing.T) {
	const priv = "fedcba9876543210fedcba9876543210fedcba98"
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	mirrorTip := commitFile(t, dir, "a.txt", "x\n", "mirror(priv1)\n\nGitOrigin-RevId: "+priv)
	got := buildOrigin(dir)
	if got != priv[:BuildOriginShortLen] {
		t.Fatalf("镜像树应取 trailer 前 %d 位：got %q want %q", BuildOriginShortLen, got, priv[:BuildOriginShortLen])
	}
	if got == mirrorTip[:BuildOriginShortLen] {
		t.Fatalf("✗ 取成了镜像 tip（两套命名空间混成一句 = 假判决）：%q", got)
	}
}

func TestBuildOrigin_NonRepoIsEmpty(t *testing.T) {
	if got := buildOrigin(t.TempDir()); got != "" {
		t.Fatalf("非 git 树必须空串（判不了），实得 %q", got)
	}
	if got := buildOrigin(""); got != "" {
		t.Fatalf("空路径必须空串，实得 %q", got)
	}
}

func TestBuildOrigin_TruncationShape(t *testing.T) {
	if got := originPrefix("0123456789abcdef"); got != "0123456789ab" {
		t.Fatalf("超长应截 %d 位：%q", BuildOriginShortLen, got)
	}
	if got := originPrefix("abc"); got != "abc" {
		t.Fatalf("短值应原样：%q", got)
	}
	if !strings.HasPrefix("0123456789abcdef", originPrefix("0123456789abcdef")) {
		t.Fatal("截出的前缀必须是原值的真前缀")
	}
}
