// repo_commit_test.go —— 提交面（`zerg repo commit` · D3b 第三步）的机检（外部测试包）。
//
// 判据逐条对应两处纪律：
//
//	① **按文件名暂存 · 禁批量**：`-A` / `--all` / `.` / 通配 ⇒ rc 2（不给「顺手带进去」的写法开口子）；
//	② **禁 `--no-verify`**：命令面**一律不接受**（§17.3 铁律④③）—— 对所有命令在 dispatch 里拒；
//	③ **过快速档才放行**（真 e2e）：合成仓里放一枚「按标定的退码退出」的门禁脚本 ⇒ rc=0 时提交落地、
//	   rc=1 时**不提交且索引面逐字未动**；
//	④ 提交信息模板 = 固定形态（题 + 提案/由/trace/判据/逐件/门禁），机器可回读；
//	⑤ 暂存清单与给的清单**逐件相同**才放行（多 / 少都点名）；
//	⑥ `repo status --root <仓>`：指到不是 git 树的地方 ⇒ rc 8（读不到不当没有）。
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// fakeGateRepo 造一个「有仓根判据件 + 一枚**按标定退码**退出的门禁脚本」的合成仓（不碰真目标）。
func fakeGateRepo(t *testing.T, gateRC string) string {
	t.Helper()
	root := gitRepo(t) // 复用 cli_repo_status_test.go 的合成仓（含 core/internal/version/version.go）
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	gate := "#!/usr/bin/env bash\n# 合成门禁（本测试的夹具）：按标定退码退出\nexit " + gateRC + "\n"
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), []byte(gate), 0o755); err != nil {
		t.Fatal(err)
	}
	// git 身份（提交要它；不依赖本机全局配置）
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "测试", "GIT_AUTHOR_EMAIL": "t@example.invalid",
		"GIT_COMMITTER_NAME": "测试", "GIT_COMMITTER_EMAIL": "t@example.invalid",
	} {
		t.Setenv(k, v)
	}
	t.Setenv("ZERG_REPO", root)
	return root
}

// repoHead 读合成仓的 HEAD（说不存在 ⇒ 空串）。
func repoHead(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestRepoCommitRefusesBulkStaging(t *testing.T) {
	for _, bad := range []string{"-A", "--all", "-a", ".", "*"} {
		var out, errb strings.Builder
		rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", bad}, &out, &errb)
		if rc != 2 {
			t.Errorf("`--file %s` ⇒ rc=%d（要 2 · 禁批量暂存）", bad, rc)
		}
		if out.String() != "" {
			t.Errorf("`--file %s` 的 stdout 必须 0 字节，实测 %q", bad, out.String())
		}
	}
}

func TestRepoCommitRefusesNoVerify(t *testing.T) {
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", "a.go", "--yes", "--no-verify"}, &out, &errb)
	if rc != 2 {
		t.Errorf("`--no-verify` ⇒ rc=%d（要 2 · 命令面一律不接受它）", rc)
	}
	if !strings.Contains(errb.String(), "--no-verify") {
		t.Errorf("stderr 要点名 `--no-verify`：%q", errb.String())
	}
}

func TestRepoCommitUsageFaceNeedsMessageAndFiles(t *testing.T) {
	var out, errb strings.Builder
	if rc := zerg.RunForTest([]string{"repo", "commit", "--file", "a.go"}, &out, &errb); rc != 2 {
		t.Errorf("缺 --message ⇒ rc=%d（要 2）", rc)
	}
	out.Reset()
	errb.Reset()
	if rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题"}, &out, &errb); rc != 2 {
		t.Errorf("缺 --file ⇒ rc=%d（要 2）", rc)
	}
}

func TestRepoCommitDryRunIsZeroSideEffect(t *testing.T) {
	root := fakeGateRepo(t, "0")
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", "dirty.txt", "--dry-run"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`--dry-run` ⇒ rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	for _, want := range []string{"计划件", "git add -A", "precommit-gates.sh --fast"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("计划件里应有 %q：%q", want, out.String())
		}
	}
	// 零副作用：没提交、索引面空（`git diff --cached` 无输出）
	if h := repoHead(t, root); h != strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD")) {
		t.Errorf("--dry-run 不许动 HEAD（%s）", h)
	}
	if s := mustGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(s) != "" {
		t.Errorf("--dry-run 不许动索引面，实测暂存：%q", s)
	}
	// 缺 --yes ⇒ 计划件 + rc 2（fail-closed）
	out.Reset()
	errb.Reset()
	rc = zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", "dirty.txt"}, &out, &errb)
	if rc != 2 || !strings.Contains(out.String(), "计划件") {
		t.Errorf("缺 --yes ⇒ rc=%d（要 2）且要出计划件：%q", rc, out.String())
	}
}

// TestRepoCommitEndToEndFastGate — 正控：快速档 rc=0 ⇒ 提交落地，且只提交点名的件、信息走模板。
func TestRepoCommitEndToEndFastGate(t *testing.T) {
	root := fakeGateRepo(t, "0")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repoHead(t, root)
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题 e2e", "--file", "a.txt",
		"--proposal", "DEV-0007", "--by", "老王", "--trace", "T-D3b", "--criterion", "zerg gate run --fast",
		"--yes"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("快速档 rc=0 ⇒ 提交应落地，实测 rc=%d · stderr=%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "a.txt") {
		t.Errorf("stdout 要给暂存清单（逐件）：%q", out.String())
	}
	after := repoHead(t, root)
	if after == before || after == "" {
		t.Fatalf("HEAD 没前进（前 %q · 后 %q）", before, after)
	}
	files := strings.Fields(mustGit(t, root, "show", "--name-only", "--pretty=format:", "HEAD"))
	if len(files) != 1 || files[0] != "a.txt" {
		t.Errorf("只许提交点名的件，实测提交了 %v", files)
	}
	body := mustGit(t, root, "log", "-1", "--format=%B")
	for _, want := range []string{"题 e2e", "提案: DEV-0007", "由: 老王", "trace: T-D3b",
		"判据: zerg gate run --fast", "件: a.txt", "门禁: 快速档 rc=0"} {
		if !strings.Contains(body, want) {
			t.Errorf("提交信息缺 %q：\n%s", want, body)
		}
	}
	// 暂存面用完即空（提交后索引干净）
	if s := mustGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(s) != "" {
		t.Errorf("提交后索引面应为空，实测 %q", s)
	}
}

// TestRepoCommitBlockedByFastGate — 负控：快速档 rc=1 ⇒ **不提交**，且索引面与 HEAD 逐字未动。
func TestRepoCommitBlockedByFastGate(t *testing.T) {
	root := fakeGateRepo(t, "1")
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repoHead(t, root)
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", "c.txt", "--yes"}, &out, &errb)
	if rc != 1 {
		t.Errorf("快速档 rc=1 ⇒ 命令要**直通**退 1，实测 %d · stderr=%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "不提交") {
		t.Errorf("stderr 要点明「不提交」：%q", errb.String())
	}
	if repoHead(t, root) != before {
		t.Errorf("被拦下时 HEAD 不许动")
	}
	if s := mustGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(s) != "" {
		t.Errorf("被拦下时索引面必须逐字未动（实测暂存 %q）—— 门禁在**暂存之前**跑", s)
	}
}

// TestRepoCommitRefusesNonEmptyIndex — 负控：索引面已有别人的暂存 ⇒ 拒执（不许顺手带进去）。
func TestRepoCommitRefusesNonEmptyIndex(t *testing.T) {
	root := fakeGateRepo(t, "0")
	if err := os.WriteFile(filepath.Join(root, "mine.txt"), []byte("m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "theirs.txt"), []byte("t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "--", "theirs.txt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("预置暂存：%v（%s）", err, out)
	}
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "commit", "--message", "题", "--file", "mine.txt", "--yes"}, &out, &errb)
	if rc != 2 {
		t.Errorf("索引面非空 ⇒ rc=%d（要 2 · 拒执）· stderr=%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "暂存面") {
		t.Errorf("stderr 要点名「暂存面」：%q", errb.String())
	}
}

func TestRepoCommitMessageTemplateAndFileSetJudge(t *testing.T) {
	msg := zerg.CommitMessageTemplateForTest("题", "DEV-0007", "老王", "T-D3b", "zerg gate run --fast",
		[]string{"a.go", "b.go"})
	for _, want := range []string{"提案: DEV-0007", "由: 老王", "trace: T-D3b",
		"判据: zerg gate run --fast", "件: a.go · b.go", "门禁: 快速档 rc=0"} {
		if !strings.Contains(msg, want) {
			t.Errorf("模板缺 %q：\n%s", want, msg)
		}
	}
	if m2 := zerg.CommitMessageTemplateForTest("题", "", "", "", "", []string{"a.go"}); strings.Contains(m2, "提案:") {
		t.Errorf("没给的格不该出现：\n%s", m2)
	}
	if ok, _, _ := zerg.CompareFileSetsForTest([]string{"a", "b"}, []string{"b", "a"}); !ok {
		t.Error("同集不同序 ⇒ 应判相同")
	}
	if ok, extra, _ := zerg.CompareFileSetsForTest([]string{"a"}, []string{"a", "b"}); ok || len(extra) != 1 {
		t.Errorf("多一件 ⇒ 应点名 extra（实测 ok=%v extra=%v）", ok, extra)
	}
	if ok, _, missing := zerg.CompareFileSetsForTest([]string{"a", "b"}, []string{"a"}); ok || len(missing) != 1 {
		t.Errorf("少一件 ⇒ 应点名 missing（实测 ok=%v missing=%v）", ok, missing)
	}
}

func TestRepoStatusRootFlag(t *testing.T) {
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "status", "--root", t.TempDir()}, &out, &errb)
	if rc != 8 {
		t.Errorf("`--root <非 git 树>` ⇒ rc=%d（要 8 · 读不到不当没有）· stderr=%s", rc, errb.String())
	}
}

// mustGit 跑一条 git 并返回 stdout（失败即 Fatal —— 夹具坏了不许静默）。
func mustGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v：%v", args, err)
	}
	return string(out)
}
