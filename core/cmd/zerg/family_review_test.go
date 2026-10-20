// family_review_test.go —— `zerg review`（脑确认复查归入命令面 · 批 B3）的判据。
//
// 落点 = `package main`（**内部**）：判据要直接读命令树真源（`commands` / `CommandInfoOfForTest`）
// 与解析面入口（`reviewParse`），并要能对**合成盘面**逐字节判「动没动盘」。
// 全部合成盘面（`t.TempDir()` 当工作目录与仓库）：不碰真 `~/.zerg/state`、不碰真 `docs/issues/`、
// 不碰本仓的 git（每个仓库都是测试里 `git init` 出来的临时仓）。
//
// 本件钉住的四格（任务单 §三 B3 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `review`（`usage` 逐字含源件的三段位置参数与 `--repo`；
//	   `arity=any`；说明面 ⇒ 无 `--json` 字段表；已开放）；且**没有**被别族抢名；
//	② **正常路径**：`approve` 真 merge + issue → done（分支没了、分支上的件上了 main）；
//	   `reject` 真丢分支 + issue → retry；`list` 出表（含 `--repo` 与工作目录分家的那一档）；
//	③ **用法面**：缺参 / 缺动作 / 动作认不得 / 多余位置参数 / `list` 带动词 / `--repo` 裸给 /
//	   未知旗标 / 不认的契约主号 ⇒ 一律 2 + stdout 零字节，且**一个字节都不许落盘**；
//	④ **只读面**：`list` 与「状态不是 waiting_review」这两条**逐件 sha256 不变**（不 merge、
//	   不写 issue）；issue 读不到 ⇒ 1 且盘面不变。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// reviewInvoke —— 走**外部**入口跑一次（与 `package main_test` 的 `RunForTest` 同一件）。
func reviewInvoke(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// reviewTestGit —— 测试侧造盘面用（✗ 不是被测实现：被测实现自己走 `reviewGit`）。
func reviewTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out)
	}
	return string(out)
}

// reviewSeedRepo —— 造一个 `main` 上有一件 `seed.txt` 的临时 git 仓（身份走 -c，不落全局配置）。
func reviewSeedRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("本机没有 git —— 正常路径那两格判不了（照实跳过，不冒充通过）")
	}
	repo := t.TempDir()
	reviewTestGit(t, repo, "init", "-b", "main", "-q")
	reviewTestGit(t, repo, "-c", "user.name=zerg-test", "-c", "user.email=test@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewTestGit(t, repo, "add", "seed.txt")
	reviewTestGit(t, repo, "-c", "user.name=zerg-test", "-c", "user.email=test@example.invalid",
		"commit", "-q", "-m", "seed file")
	return repo
}

// reviewAddBranch —— 在 `repo` 上开一条分支，往分支里落一件 `picked.txt`（用来验「真 merge 了」）。
func reviewAddBranch(t *testing.T, repo, branch string) {
	t.Helper()
	reviewTestGit(t, repo, "checkout", "-q", "-b", branch)
	if err := os.WriteFile(filepath.Join(repo, "picked.txt"), []byte(branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewTestGit(t, repo, "add", "picked.txt")
	reviewTestGit(t, repo, "-c", "user.name=zerg-test", "-c", "user.email=test@example.invalid",
		"commit", "-q", "-m", "work on "+branch)
	reviewTestGit(t, repo, "checkout", "-q", "main")
}

// reviewWriteIssue —— 按源件读的那三行格式造一张单（`- **任务**` / `- **状态**` / `- **分支**`）。
func reviewWriteIssue(t *testing.T, workDir, id, task, status, branch string) string {
	t.Helper()
	dir := filepath.Join(workDir, "docs", "issues")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("# " + id + "\n\n")
	b.WriteString("- **任务**: " + task + "\n")
	b.WriteString("- **状态**: " + status + "\n")
	if branch != "" {
		b.WriteString("- **分支**: " + branch + "\n")
	}
	p := filepath.Join(dir, id+".md")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// reviewBranchExists —— `git branch --list <名>` 有没有输出（判「分支真被删了」）。
func reviewBranchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	out := reviewTestGit(t, repo, "branch", "--list", branch)
	return strings.TrimSpace(out) != ""
}

// ── ① 登记面 ────────────────────────────────────────────────────────────────

func TestReviewCommandRegistered(t *testing.T) {
	info, ok := CommandInfoOfForTest("review")
	if !ok {
		t.Fatalf("命令树里没有 `zerg review`（批 B3 的核心那一条）")
	}
	for _, want := range []string{"zerg review", "<workdir>", "<issue-id|list>", "approve|reject", "--repo"} {
		if !strings.Contains(info.Usage, want) {
			t.Errorf("用法串漏了 %q（源件的三段位置参数与 `-repo` 必须逐字保留）：%q", want, info.Usage)
		}
	}
	if info.Passthrough {
		t.Errorf("`review` 不该是透传型（它自己判位置参数与动作语义）")
	}
	if len(info.Fields) != 0 {
		t.Errorf("`review` 是说明面（源件没有机器面）⇒ 不该有 `--json` 字段表：%v", info.Fields)
	}
	if !info.OpenForRun {
		t.Errorf("`review` 要标成**已开放**（真跑 —— 不是占位动作）")
	}

	// arity=any（位置参数是它的主形态）＋ args 三格都写清了（⇒ 不是「声明不收位置参数」）。
	var arity string
	var c *command
	for i := range commands {
		if joinPath(commands[i].path) == "review" {
			arity, c = commands[i].arity, commands[i]
		}
	}
	if arity != arityAny {
		t.Errorf("`review` 的 arity = %q（要 %q —— 它收 2–3 枚位置参数）", arity, arityAny)
	}
	if c == nil {
		t.Fatalf("命令树里取不到 `review` 那条（路径已变？）")
	}
	if len(c.args) != 3 {
		t.Errorf("`review` 的 args 要三段（<workdir> / <issue-id|list> / [approve|reject]），现得 %v", c.args)
	}
	if declaresNoPositional(c) {
		t.Errorf("`review` 收了位置参数 ⇒ 不该被判成「声明不收位置参数」（否则多余参数那一格判不到）")
	}

	// **没被抢名**：`review` 既在册就只该有一条（归入 = 只增一格，不许出现第二条同名路径）。
	n := 0
	for _, cc := range catalog() {
		if joinPath(cc.path) == "review" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("`review` 在命令树里出现 %d 条（要 1 条）", n)
	}
}

// ── ② 正常路径 ──────────────────────────────────────────────────────────────

func TestReviewNormalPath(t *testing.T) {
	// 被测实现 spawn 的 git 会写 merge commit ⇒ 身份从**测试进程环境**传下去（不落任何配置件）。
	t.Setenv("GIT_AUTHOR_NAME", "zerg-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "zerg-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")

	wd := t.TempDir()
	repo := reviewSeedRepo(t)

	// ── approve：真 merge + issue → done ──
	reviewAddBranch(t, repo, "task/T-1")
	issuePath := reviewWriteIssue(t, wd, "T-1", "把 A 改成 B", "waiting_review", "task/T-1")
	rc, out, errb := reviewInvoke(t, "review", wd, "T-1", "approve", "--repo", repo)
	if rc != exitOK {
		t.Fatalf("`review … approve` rc=%d（要 0）· stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(out, "✅ merge confirmed: T-1 → main (issue done)") {
		t.Errorf("approve 结果面判词不对：%q", out)
	}
	if !strings.Contains(errb, "🔀 cd "+repo) {
		t.Errorf("approve 进度面没打源件那行（`🔀 cd <repo> …`）：%q", errb)
	}
	raw, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "- **状态**: done") {
		t.Errorf("approve 后 issue 状态没变成 done：\n%s", raw)
	}
	if reviewBranchExists(t, repo, "task/T-1") {
		t.Errorf("approve 后分支 `task/T-1` 还在（源件 `git branch -d` 那一步没做成）")
	}
	if _, err := os.Stat(filepath.Join(repo, "picked.txt")); err != nil {
		t.Errorf("approve 后 main 上没有分支里那件 `picked.txt`（merge 没真做）：%v", err)
	}

	// ── reject：真丢分支 + issue → retry ──
	reviewAddBranch(t, repo, "task/T-2")
	issuePath2 := reviewWriteIssue(t, wd, "T-2", "把 C 改成 D", "waiting_review", "task/T-2")
	rc, out, errb = reviewInvoke(t, "review", wd, "T-2", "reject", "--repo", repo)
	if rc != exitOK {
		t.Fatalf("`review … reject` rc=%d（要 0）· stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(out, "❌ rejected: T-2 (branch discarded — issue retry)") {
		t.Errorf("reject 结果面判词不对：%q", out)
	}
	if !strings.Contains(errb, "🗑 cd "+repo) {
		t.Errorf("`--repo` 给了 ⇒ 进度面该打 `cd %s`（源件 `-repo` 的落点）：%q", repo, errb)
	}
	raw2, err := os.ReadFile(issuePath2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw2), "- **状态**: retry") {
		t.Errorf("reject 后 issue 状态没变成 retry：\n%s", raw2)
	}
	if reviewBranchExists(t, repo, "task/T-2") {
		t.Errorf("reject 后分支 `task/T-2` 还在（`git branch -D` 没做成）")
	}
}

// reject 的缺省仓库是**工作目录**：`--repo` 不给时 git 命令落在 `<workdir>` 上（源件同档）。
// 单独一格：把仓库就当工作目录，判「分支真被丢 + issue → retry」。
func TestReviewRejectDefaultRepoIsWorkdir(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "zerg-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "zerg-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")

	repo := reviewSeedRepo(t)
	reviewAddBranch(t, repo, "task/T-3")
	issuePath := reviewWriteIssue(t, repo, "T-3", "缺省仓库那一档", "waiting_review", "task/T-3")

	rc, out, errb := reviewInvoke(t, "review", repo, "T-3", "reject")
	if rc != exitOK {
		t.Fatalf("rc=%d（要 0）· stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "🗑 cd "+repo+" && git checkout main && git branch -D task/T-3") {
		t.Errorf("进度面判词不对（要逐字源件的 `cd … && git checkout main && git branch -D …`）：%q", errb)
	}
	if reviewBranchExists(t, repo, "task/T-3") {
		t.Errorf("缺省仓库档没把分支丢掉")
	}
	raw, _ := os.ReadFile(issuePath)
	if !strings.Contains(string(raw), "- **状态**: retry") {
		t.Errorf("缺省仓库档 issue 没变 retry：\n%s", raw)
	}
}

// `list` 档：只列在等脑确认的单（表头 / 行格式 / 空档判词逐字源件）。
func TestReviewList(t *testing.T) {
	wd := t.TempDir()
	reviewWriteIssue(t, wd, "T-10", "任务十", "waiting_review", "task/T-10")
	reviewWriteIssue(t, wd, "T-11", "任务十一", "done", "task/T-11")
	reviewWriteIssue(t, wd, "T-12", "任务十二", "waiting_review", "")

	rc, out, errb := reviewInvoke(t, "review", wd, "list")
	if rc != exitOK {
		t.Fatalf("`review … list` rc=%d（要 0）· stderr=%q", rc, errb)
	}
	for _, want := range []string{
		"📋 awaiting brain approval (waiting_review):",
		"  T-10: 任务十",
		"  T-12: 任务十二",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("list 输出缺 %q\n—— 实得：\n%s", want, out)
		}
	}
	if strings.Contains(out, "T-11") {
		t.Errorf("list 把 `done` 的单也列出来了（源件只列 `waiting_review`）：\n%s", out)
	}

	// 没有任务目录 ⇒ 源件同档：出表头 + 空档判词，退 0（「没有任务目录 = 没有在等的单」是真话）。
	empty := t.TempDir()
	rc, out, _ = reviewInvoke(t, "review", empty, "list")
	if rc != exitOK || !strings.Contains(out, "  (no tasks awaiting approval)") {
		t.Errorf("空工作目录：rc=%d（要 0）· out=%q（要含空档判词）", rc, out)
	}
}

// ── ③ 用法面 ────────────────────────────────────────────────────────────────

func TestReviewUsageFace(t *testing.T) {
	wd := t.TempDir()
	issuePath := reviewWriteIssue(t, wd, "T-20", "用法面那格", "waiting_review", "task/T-20")
	before := schedRunTreeHash(t, wd) // 同包既有助手：逐件 sha256 + 相对路径（只读面复用）

	cases := []struct {
		name string
		argv []string
		want string // stderr 必含
	}{
		{"缺全部位置参数", []string{"review"}, "缺位置参数"},
		{"只有一个位置参数", []string{"review", wd}, "缺位置参数"},
		{"缺动作", []string{"review", wd, "T-20"}, "approve or reject expected"},
		{"动作认不得", []string{"review", wd, "T-20", "maybe"}, "unknown action"},
		{"多余位置参数", []string{"review", wd, "T-20", "approve", "extra"}, "多余位置参数"},
		{"list 带动词", []string{"review", wd, "list", "approve"}, "`list` 不收动作"},
		{"--repo 裸给", []string{"review", wd, "T-20", "approve", "--repo"}, "--repo"},
		{"--repo 空值", []string{"review", wd, "T-20", "approve", "--repo="}, "--repo"},
		{"未知旗标", []string{"review", "--nosuchflag-zz"}, "未知旗标"},
		{"不认的契约主号", []string{"review", "--schema", "zerg/v9"}, "契约主号"},
	}
	for _, c := range cases {
		rc, out, errb := reviewInvoke(t, c.argv...)
		if rc != exitUsage {
			t.Errorf("%s：rc=%d（要 %d · 用法错）", c.name, rc, exitUsage)
		}
		if out != "" {
			t.Errorf("%s：stdout 要零字节（用法类错误不吐包封 · K2 甲档）：%q", c.name, out)
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：stderr 没说到 %q：%q", c.name, c.want, errb)
		}
	}
	// 用法错**一个字节都不许落盘**（连 issue 都没读过）。
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("用法面动了工作目录：\n改前：\n%s\n改后：\n%s", before, after)
	}
	raw, _ := os.ReadFile(issuePath)
	if !strings.Contains(string(raw), "- **状态**: waiting_review") {
		t.Errorf("用法面把 issue 状态改了：\n%s", raw)
	}
}

// ── ④ 只读面 ────────────────────────────────────────────────────────────────

func TestReviewReadOnly(t *testing.T) {
	wd := t.TempDir()
	reviewWriteIssue(t, wd, "T-30", "在等确认的", "waiting_review", "task/T-30")
	reviewWriteIssue(t, wd, "T-31", "已经办结的", "done", "task/T-31")

	// `list` 只读。
	before := schedRunTreeHash(t, wd)
	if rc, _, errb := reviewInvoke(t, "review", wd, "list"); rc != exitOK {
		t.Fatalf("list rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("`review … list` 动了工作目录：\n改前：\n%s\n改后：\n%s", before, after)
	}

	// 状态不是 waiting_review ⇒ 退 1、判词逐字源件、**盘面一个字节不动**（不 merge、不写 issue）。
	rc, out, errb := reviewInvoke(t, "review", wd, "T-31", "approve")
	if rc != exitFail {
		t.Errorf("状态不是 waiting_review 时 rc=%d（要 %d）· stdout=%q", rc, exitFail, out)
	}
	if out != "" {
		t.Errorf("被状态门拒掉时 stdout 要零字节：%q", out)
	}
	if !strings.Contains(errb, "⚠️ issue status=done (waiting_review required)") {
		t.Errorf("状态门判词不对（要逐字源件）：%q", errb)
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("状态门拒掉后盘面变了（它不许 merge、不许写 issue）：\n改前：\n%s\n改后：\n%s", before, after)
	}

	// issue 读不到 ⇒ 退 1（源件同档），盘面不动。
	rc, out, errb = reviewInvoke(t, "review", wd, "T-404", "approve")
	if rc != exitFail || out != "" {
		t.Errorf("issue 不在：rc=%d out=%q（要 %d / 零字节）", rc, out, exitFail)
	}
	if !strings.Contains(errb, "❌ failed to read issue") {
		t.Errorf("issue 不在的判词不对：%q", errb)
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("issue 不在时动了盘面：\n改前：\n%s\n改后：\n%s", before, after)
	}
}
