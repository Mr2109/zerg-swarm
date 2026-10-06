package main

// check_i18n_drift_test.go —— 成对负控（✗ 禁恒绿）：正控（干净 ⇒ rc=0）· 各造 violation（exist/stale
// ⇒ rc=2 · comment ⇒ rc=1）· 空转（⇒ rc=2 不给结论）· 取不到输入（非 git ⇒ rc=2）· JSON 面逐字节 ·
// --self-test · 帮助/未知旗标。全部断言以 py 原件现读读数为基准（见回执对拍表：27 格三面逐字节同）。
//
// 负控「有牙」实证（复核法）：把主件 `ci18nTrOutOfRepo` / `ci18nFmValue` / `ci18nMarkerValue`
// 任一哑化（换成恒返回 / 直读整行）⇒ 对应用例 FAIL 并打出失配正文（带行号）；原版同命令 PASS。

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ci18nTRunIn —— 进程内复用入口（同 argv 口径：不含程序名）；进 dir 跑、跑后回原 cwd。
func ci18nTRunIn(t *testing.T, dir string, argv ...string) (int, string, string) {
	t.Helper()
	old, _ := os.Getwd()
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { _ = os.Chdir(old) }()
	var o, e bytes.Buffer
	rc := checkI18nDriftGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

func ci18nTGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v 失败：%v\n%s", args, err, out.String())
	}
	return strings.TrimRight(out.String(), "\n")
}

func ci18nTWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ci18nTRepo —— 建一棵夹具 git 仓（返 repo 根 + 首提交 sc）。
func ci18nTRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	ci18nTGit(t, d, "init", "-q", ".")
	ci18nTGit(t, d, "config", "user.email", "t@local")
	ci18nTGit(t, d, "config", "user.name", "t")
	return d
}

func ci18nTCommit(t *testing.T, d string, files map[string]string, msg string) string {
	t.Helper()
	for p, body := range files {
		ci18nTWrite(t, filepath.Join(d, filepath.FromSlash(p)), body)
	}
	ci18nTGit(t, d, "add", "-A")
	ci18nTGit(t, d, "commit", "-qm", msg)
	return ci18nTGit(t, d, "rev-parse", "HEAD")
}

// 正控：双侧齐 · 无过期 · 有注释 ⇒ rc=0 · 判定档 PASS。
func TestCi18nGreenFixture(t *testing.T) {
	d := ci18nTRepo(t)
	sc := ci18nTCommit(t, d, map[string]string{"docs/zh/s.md": "---\ntitle: s\n---\nzh\n"}, "zh")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\n<!-- x -->\ntr\n",
	}, "en")
	rc, out, _ := ci18nTRunIn(t, d, "--phase", "all")
	if rc != 0 {
		t.Fatalf("全绿应 rc=0，得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "判定档：PASS（两格 = exist=PASS · stale=PASS）") {
		t.Fatalf("判定档失配：%q (第 %d 行)", out, 1)
	}
}

// violation①：exist 缺一侧 ⇒ rc=2 · 逐条点名。
func TestCi18nExistMissingIsRed(t *testing.T) {
	d := ci18nTRepo(t)
	sc := ci18nTCommit(t, d, map[string]string{
		"docs/zh/s.md": "---\ntitle: s\n---\nzh\n",
		"docs/zh/t.md": "---\ntitle: t\n---\nzh t\n",
	}, "zh")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\n<!-- x -->\ntr\n",
	}, "en")
	rc, out, _ := ci18nTRunIn(t, d)
	if rc != 2 {
		t.Fatalf("缺一侧应 rc=2，得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "缺 en 侧：docs/en/t.md") {
		t.Fatalf("未逐条点名缺件：%q (第 %d 行)", out, 1)
	}
}

// violation②：stale 过期 ⇒ rc=2（commit 级）。
func TestCi18nStaleIsRed(t *testing.T) {
	d := ci18nTRepo(t)
	sc := ci18nTCommit(t, d, map[string]string{"docs/zh/s.md": "---\ntitle: s\n---\nzh v1\n"}, "zh")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\n<!-- x -->\ntr\n",
	}, "en")
	ci18nTCommit(t, d, map[string]string{"docs/zh/s.md": "---\ntitle: s\n---\nzh v2\n"}, "zh v2")
	rc, out, _ := ci18nTRunIn(t, d)
	if rc != 2 {
		t.Fatalf("过期应 rc=2，得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "过期：docs/zh/s.md 在 "+sc+"..HEAD 之间改动过") {
		t.Fatalf("过期正文失配：%q (第 %d 行)", out, 1)
	}
}

// violation③：comment 缺 ⇒ 先非阻塞 rc=1。
func TestCi18nCommentMissingWarns(t *testing.T) {
	d := ci18nTRepo(t)
	sc := ci18nTCommit(t, d, map[string]string{"docs/zh/s.md": "---\ntitle: s\n---\nzh\n"}, "zh")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\ntr\n",
	}, "en")
	rc, out, _ := ci18nTRunIn(t, d)
	if rc != 1 {
		t.Fatalf("缺注释应 rc=1（告警），得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "无 HTML 注释块") {
		t.Fatalf("缺注释正文失配：%q (第 %d 行)", out, 1)
	}
}

// 空转 ≠ 绿：无 docs/<lang> ⇒ BLOCKED rc=2。
func TestCi18nEmptyScanBlocked(t *testing.T) {
	d := ci18nTRepo(t)
	ci18nTCommit(t, d, map[string]string{"README.md": "x\n"}, "init")
	rc, out, _ := ci18nTRunIn(t, d)
	if rc != 2 {
		t.Fatalf("空转应 rc=2（不给结论），得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "不给结论") || !strings.Contains(out, "判定档：BLOCKED") {
		t.Fatalf("空转正文失配：%q (第 %d 行)", out, 1)
	}
}

// 取不到输入：非 git 目录 ⇒ rc=2 · stderr 具名。
func TestCi18nNoGitBlocked(t *testing.T) {
	d := t.TempDir()
	rc, out, errS := ci18nTRunIn(t, d)
	if rc != 2 {
		t.Fatalf("非 git 应 rc=2，得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(errS, "✗ 不在 git 仓内") {
		t.Fatalf("非 git stderr 失配：%q (第 %d 行)", errS, 1)
	}
}

// 源页在仓外 ⇒ 单列「不判过期」（不静默绿、不冒充过期）。
func TestCi18nSourceOutOfRepo(t *testing.T) {
	base := t.TempDir()
	ci18nTWrite(t, filepath.Join(base, "outside", "x.md"), "---\ntitle: x\n---\nx\n")
	d := filepath.Join(base, "repo")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	ci18nTGit(t, d, "init", "-q", ".")
	ci18nTGit(t, d, "config", "user.email", "t@local")
	ci18nTGit(t, d, "config", "user.name", "t")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: ../outside/x.md\nsource_commit: abc123\n---\n<!-- x -->\ntr\n",
	}, "en")
	rc, out, _ := ci18nTRunIn(t, d, "--phase", "stale")
	if rc != 0 {
		t.Fatalf("仓外源页不进退码应 rc=0，得 %d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
	if !strings.Contains(out, "源页已在**仓外**") || !strings.Contains(out, "1 篇源页在仓外 ⇒ 不判过期") {
		t.Fatalf("仓外源页正文失配：%q (第 %d 行)", out, 1)
	}
}

// JSON 面逐字节 + 人面转 stderr。
func TestCi18nJSONFace(t *testing.T) {
	d := ci18nTRepo(t)
	sc := ci18nTCommit(t, d, map[string]string{"docs/zh/s.md": "---\ntitle: s\n---\nzh\n"}, "zh")
	ci18nTCommit(t, d, map[string]string{
		"docs/en/s.md": "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\n<!-- x -->\ntr\n",
	}, "en")
	rc, out, errS := ci18nTRunIn(t, d, "--json")
	want := `{"schema":"zerg/i18n-drift/v1","drift":0,"verdict":"PASS","tiers":{"exist":"PASS","stale":"PASS","comment":"PASS"},"counts":{"exist":0,"stale":0,"comment":0},"rc":0}` + "\n"
	if rc != 0 || out != want {
		t.Fatalf("JSON 正文不符：rc=%d\n got=%q\nwant=%q (第 %d 行)", rc, out, want, 1)
	}
	if !strings.Contains(errS, "── 汇总（机器读面") {
		t.Fatalf("人面应在 stderr：%q (第 %d 行)", errS, 1)
	}
}

// 帮助面对齐（逐字）+ 未知旗标。
func TestCi18nHelpAndUnknown(t *testing.T) {
	rc, out, errS := ci18nTRunIn(t, "", "-h")
	if rc != 0 || out != ci18nHelp || errS != "" {
		t.Fatalf("-h 失配 rc=%d out==help:%v err=%q (第 %d 行)", rc, out == ci18nHelp, errS, 1)
	}
	rc, out, errS = ci18nTRunIn(t, "", "--bogus")
	if rc != 2 || out != "" || errS != "✗ 未知参数：--bogus\n" {
		t.Fatalf("未知旗标失配 rc=%d out=%q err=%q (第 %d 行)", rc, out, errS, 1)
	}
	rc, _, errS = ci18nTRunIn(t, "", "--langs", ",")
	if rc != 2 || !strings.Contains(errS, "✗ --langs 需要两个语言码") {
		t.Fatalf("--langs 空失配 rc=%d err=%q (第 %d 行)", rc, errS, 1)
	}
}

// --self-test：6 条全过 ⇒ rc=0（自检前置＝须在 git 仓内）。
func TestCi18nSelfTestGreen(t *testing.T) {
	d := ci18nTRepo(t)
	ci18nTCommit(t, d, map[string]string{"AGENTS.md": "x\n"}, "init")
	rc, out, errS := ci18nTRunIn(t, d, "--self-test")
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d (stderr=%q) (第 %d 行)", rc, errS, 1)
	}
	if !strings.HasSuffix(out, "  自证：通过 ✓（6 条）\n") {
		t.Fatalf("自检结论行失配：%q (第 %d 行)", out, 1)
	}
}

// ---- 原语牙（哑化任一 ⇒ 本组红）----

// fm_value：引号剥离 + 前后空白 + 无 frontmatter 不误判。
func TestCi18nFmValueTeeth(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.md")
	ci18nTWrite(t, p, "---\ntranslation_of:   \"docs/x.md\"  \n---\n")
	if got := ci18nFmValue(p, "translation_of"); got != "docs/x.md" {
		t.Fatalf("fm_value 引号/空白剥离失配：got=%q want=%q (第 %d 行)", got, "docs/x.md", 1)
	}
	ci18nTWrite(t, p, "no frontmatter\n")
	if got := ci18nFmValue(p, "translation_of"); got != "" {
		t.Fatalf("无 frontmatter 应空：got=%q (第 %d 行)", got, 1)
	}
}

// marker_value：三段 sed 链（去 KEY: / 首个 --> 之后 / 首个空白之后）。
func TestCi18nMarkerValueTeeth(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "b.md")
	ci18nTWrite(t, p, "x\n<!-- source_commit: fd69646 -->\n")
	if got := ci18nMarkerValue(p, "source_commit"); got != "fd69646" {
		t.Fatalf("marker 链失配：got=%q want=%q (第 %d 行)", got, "fd69646", 1)
	}
	// frontmatter 优先于标记行
	ci18nTWrite(t, p, "---\nsource_commit: aaaa\n---\n<!-- source_commit: bbbb -->\n")
	if got := ci18nSourceCommitOf(p); got != "aaaa" {
		t.Fatalf("两处回退链失配：got=%q want=%q (第 %d 行)", got, "aaaa", 1)
	}
}

// tr_out_of_repo：仓内 ⇒ false、仓外 ⇒ true。
func TestCi18nTrOutOfRepoTeeth(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoReal := filepath.Join(repo, "ci18n-repo")
	if err := os.MkdirAll(repoReal, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outReal := filepath.Join(out, "ci18n-other")
	if err := os.MkdirAll(outReal, 0o755); err != nil {
		t.Fatal(err)
	}
	// repoRoot 取「物理真身」（判据里由 `git rev-parse --show-toplevel` 给，已解析符号链接）。
	if ci18nTrOutOfRepo(filepath.Join(repoReal, "AGENTS.md"), repoReal) {
		t.Fatalf("仓内应 false（第 %d 行）", 1)
	}
	if !ci18nTrOutOfRepo(filepath.Join(outReal, "x.md"), repoReal) {
		t.Fatalf("仓外应 true（第 %d 行）", 1)
	}
	if ci18nTrOutOfRepo("", repoReal) {
		t.Fatalf("空路径应 false（第 %d 行）", 1)
	}
	// 目标目录不存在 ⇒ 判不了 ⇒ 不算仓外（复刻 `cd` 失败 ⇒ return 1）。
	if ci18nTrOutOfRepo(filepath.Join(out, "nope", "x.md"), repoReal) {
		t.Fatalf("目录不存在应 false（第 %d 行）", 1)
	}
}

// is_only_dashes / has_non_hex 原语。
func TestCi18nHexAndDashes(t *testing.T) {
	if !ci18nHasNonHex("zz123") || ci18nHasNonHex("abc123") {
		t.Fatalf("has_non_hex 失配（第 %d 行）", 1)
	}
	if !ci18nIsOnlyDashes("---  ") || ci18nIsOnlyDashes("---x") {
		t.Fatalf("is_only_dashes 失配（第 %d 行）", 1)
	}
}
