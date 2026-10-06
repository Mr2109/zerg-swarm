// check_public_marks_tracked_parity_test.go —— 判据件 `scripts/gates/check-public-marks-tracked-parity.py`
// 的 Go 真搬 **成对负控**测试件。覆盖：
//
//	· cpmtPyJoin / cpmtPyNormpath 三语义差（中间 `..` 原样留 · 后段绝对丢前段 · 空段保留尾 `/` · `//a` 保两前导斜杠）
//	· cpmtJudge 成对正负控（合成 git 夹具：真未跟踪 ⇒ rc=0 · 声明未跟踪↔现读已跟踪 ⇒ rc=1 且点名）
//	· 空转 / 缺件 ⇒ rc=2（fail-closed，不给结论）
//	· selftest 7 条全绿（正控）
//	· 哑化有牙：摘掉 cpmtClaim ⇒ 自检必红（见负控）
package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func cpmtTgRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out)
	}
}

func cpmtTgFixtureRepo(t *testing.T) string {
	t.Helper()
	r := t.TempDir()
	if err := os.WriteFile(cpmtPyJoin(r, "tracked.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cpmtTgRunGit(t, r, "init", "-q")
	cpmtTgRunGit(t, r, "-c", "user.email=s@x", "-c", "user.name=s", "add", "tracked.go")
	cpmtTgRunGit(t, r, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x")
	if err := os.WriteFile(cpmtPyJoin(r, "untracked.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCpmtPyJoin三语义差(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"中间 `..` 原样留", cpmtPyJoin("a", "../b"), "a/../b"},
		{"后段绝对丢前段", cpmtPyJoin("repo", "/abs"), "/abs"},
		{"空段保留尾斜杠", cpmtPyJoin("a", ""), "a/"},
		{"空段中间", cpmtPyJoin("a", "", "b"), "a/b"},
		{"空首段", cpmtPyJoin("", "a"), "a"},
		{"常规拼接", cpmtPyJoin("a", "b"), "a/b"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s：cpmtPyJoin = %q，期望 %q", c.name, c.got, c.want)
		}
	}
}

func TestCpmtPyNormpath保留双前导斜杠(t *testing.T) {
	if got := cpmtPyNormpath("//a/b"); got != "//a/b" {
		t.Errorf("`//a/b` 应保两前导斜杠，得 %q", got)
	}
	if got := cpmtPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("`//a/b/../c` = %q，期望 `//a/c`", got)
	}
	if got := cpmtPyNormpath("///a/b"); got != "/a/b" {
		t.Errorf("`///a/b` 应折成单前导斜杠，得 %q", got)
	}
}

// 核心判定成对正负控（合成 git 夹具 · 确定读面）。
func TestCpmtJudge正负控(t *testing.T) {
	repo := cpmtTgFixtureRepo(t)
	rel := cpmtTableRel
	// 正控：真未跟踪的行宣称「未跟踪」⇒ rc=0、恰对拍 1 行
	cpmtWriteTable(repo, rel, [][2]string{{"untracked.go", "本件现读未跟踪"}})
	rc, stale, checked, reason := cpmtJudge(repo, rel)
	if rc != 0 || reason != "ok" || checked != 1 || len(stale) != 0 {
		t.Fatalf("正控应 rc=0/ok/1，实际 rc=%d reason=%s checked=%d stale=%v", rc, reason, checked, stale)
	}
	// 负控：已跟踪的行宣称「未跟踪」⇒ rc=1 且点名 `1 tracked.go`
	cpmtWriteTable(repo, rel, [][2]string{{"tracked.go", "本件现读未跟踪"}})
	rc, stale, _, reason = cpmtJudge(repo, rel)
	if rc != 1 || reason != "stale" || len(stale) != 1 || stale[0] != (cpmtStale{Line: 1, Path: "tracked.go"}) {
		t.Fatalf("负控应 rc=1/stale/[(1,tracked.go)]，实际 rc=%d reason=%s stale=%v", rc, reason, stale)
	}
	// 换回真未跟踪 ⇒ 又绿
	cpmtWriteTable(repo, rel, [][2]string{{"untracked.go", "本件现读未跟踪"}})
	if rc := cpmtJudgeRc(repo, rel); rc != 0 {
		t.Fatalf("换回后应 rc=0，实际 %d", rc)
	}
	// 空转：0 行「未跟踪」⇒ rc=2（假覆盖）
	cpmtWriteTable(repo, rel, [][2]string{{"tracked.go", "本件现读已跟踪（不进对拍面）"}})
	if rc := cpmtJudgeRc(repo, rel); rc != 2 {
		t.Fatalf("空转应 rc=2，实际 %d", rc)
	}
	// 缺件：表不在盘 ⇒ rc=2
	if rc := cpmtJudgeRc(repo, "scripts/没有这一份.tsv"); rc != 2 {
		t.Fatalf("缺件应 rc=2，实际 %d", rc)
	}
}

// 正控：7 条自检必须全绿。
func TestCpmtSelftest全绿(t *testing.T) {
	var b strings.Builder
	if rc := cpmtSelftest(&b); rc != 0 {
		t.Fatalf("自检应 rc=0，实际 rc=%d\n%s", rc, b.String())
	}
	if !strings.Contains(b.String(), "自检结论：断言 7 条 · 失败 0 条") {
		t.Fatalf("自检条数断言缺失：\n%s", b.String())
	}
}

// 负控有牙：把「未跟踪」标记串摘掉（哑化）⇒ 自检必红（判据失去对拍面 ⇒ ① 由 rc=0 变 rc=2）。
func TestCpmtClaim哑化必红(t *testing.T) {
	saved := cpmtClaim
	cpmtClaim = "zzz-本串绝不出现"
	defer func() { cpmtClaim = saved }()
	var b strings.Builder
	if rc := cpmtSelftest(&b); rc == 0 {
		t.Fatalf("摘掉 cpmtClaim 后自检必须红（负控有牙），实际 rc=0\n%s", b.String())
	}
}
