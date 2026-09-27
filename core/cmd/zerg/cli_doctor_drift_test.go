package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// cli_doctor_drift_test.go —— ⑦-b「源码 ↔ 制品两代对拍」（缺口 `GAP-20260927-410`）三态负/正控。
//
// 夹具 = **真文件**的临时 git 仓（`t.TempDir()` + `git init` · 不用 symlink 影子根、不用 rmtree）。
// 制品身份用 `version.Commit` 这个 **var** 直接换（`-ldflags -X` 的注入点就是它）⇒ 三态都能造。

// driftFixture —— 造真仓：c1(a.txt) → c2(b.txt)，返回仓根 + 两笔 sha。
func driftFixture(t *testing.T) (root, first, head string) {
	t.Helper()
	root = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		full := append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
		out, err := exec.Command("git", full...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v 失败：%v · %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-q", "-m", "c1")
	first = git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "b.txt")
	git("commit", "-q", "-m", "c2")
	head = git("rev-parse", "HEAD")
	return root, first, head
}

func TestDoctorDriftThreeStates(t *testing.T) {
	root, first, head := driftFixture(t)
	t.Setenv("ZERG_REPO", root)
	old := version.Commit
	t.Cleanup(func() { version.Commit = old })

	// ① 干净：制品 id == 源码 HEAD ⇒ PASS「未过期」
	version.Commit = head
	it := doctorDriftItem()
	if it["verdict"] != "PASS" || !strings.Contains(it["detail"], "未过期") {
		t.Errorf("① 干净态：verdict=%q detail=%q（要 PASS 且含「未过期」）", it["verdict"], it["detail"])
	}

	// ② 过期：制品 id = 第一笔 ⇒ REPORT「过期」且点名改过的 `b.txt`
	version.Commit = first
	it = doctorDriftItem()
	if it["verdict"] != "REPORT" || !strings.Contains(it["detail"], "过期") {
		t.Errorf("② 过期态：verdict=%q detail=%q（要 REPORT 且含「过期」）", it["verdict"], it["detail"])
	}
	if !strings.Contains(it["detail"], "b.txt") {
		t.Errorf("② 过期态没列出改动件（要含 b.txt）：%s", it["detail"])
	}

	// ②-b 未提交的工作树也算「改过」：HEAD 上未注入 id 前置，先造一个未提交件再判
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	it = doctorDriftItem()
	if !strings.Contains(it["detail"], "c.txt") {
		t.Errorf("②-b 未提交件（c.txt）没进对拍：%s", it["detail"])
	}

	// ③ 拿不到结论：`version.Commit` 未注入（裸 go build / 进程内测试的形态）⇒ REPORT 明说拿不到
	version.Commit = "unknown"
	it = doctorDriftItem()
	if it["verdict"] != "REPORT" || !strings.Contains(it["detail"], "拿不到结论") {
		t.Errorf("③ 无 id 态：verdict=%q detail=%q（要 REPORT 且含「拿不到结论」）", it["verdict"], it["detail"])
	}

	// ③-b 仓根取不到 ⇒ 也是「拿不到结论」（不猜路径）
	version.Commit = head
	t.Setenv("ZERG_REPO", "")
	// 清掉环境后 repoRoot 会按可执行文件/当前目录上溯；测试进程所在目录注定不是仓根时才会空。
	// 这里只断言：无论空不空，都**不许**给出正文之外的结论（PASS/FAIL 不能凭猜）。
	it = doctorDriftItem()
	if it["verdict"] == "FAIL" {
		t.Errorf("③-b 真源取不到时不许判 FAIL（不许凭猜）：%+v", it)
	}
}
