package api

import "testing"

// mustGit — 测试用 ZergGit 构造：**不忽略错误**。
//
// 为什么需要：NewZergGit 内部会 `git init` + 一次 `git commit`，而 commit 需要 git 身份
// （user.name/user.email）。CI runner 默认没配 → NewZergGit 返回 (nil, err)，而测试里
// `z, _ := NewZergGit(dir)` 把错误丢掉 → 之后在**nil 接收者**上调用方法 → 段错误 panic
// （2026-09-11 CI 实测：core/internal/api TestCloseCommit，栈顶 zerg_git.go:130）。
//
// 纪律：环境不具备就**跳过并写明原因**，绝不把错误丢掉后继续用空指针。
func mustGit(t *testing.T, dir string) *ZergGit {
	t.Helper()
	z, err := NewZergGit(dir)
	if err != nil {
		t.Skipf("git 底座不可用（%v）——跳过（CI 需配置 git 身份）", err)
	}
	return z
}

// 自证：git 底座不可用时必须**跳过**而不是带着 nil 继续（否则就是本文件开头描述的那个段错误）。
// 用必然失败的路径（NUL 字节）触发 NewZergGit 报错。
func TestMustGit_skipsWhenGitUnusable(t *testing.T) {
	mustGit(t, "bad\x00dir")
	t.Error("git 不可用时应跳过，却继续执行了")
}
