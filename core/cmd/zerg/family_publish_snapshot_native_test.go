// family_publish_snapshot_native_test.go —— `zerg publish snapshot-self-test`（拟名 · 待命令树登记）Go 真身的
// 定向单测：确定性面（workflow 行式解析 / 需要仓机检 / json 文本 / 尾行 / 按码点截断 / 自指消解）
// ＋ 一条**默认跳过**的两态对拍（`PUBLISH_SNAPSHOT_PARITY_LIVE=1` 才跑 · 仓外夹具 · 真起 bash/python3）。
//
// ★ 两态对拍口径：同一只**仓外夹具**（`t.TempDir()`）下，旧 py（原件副本置于夹具 `scripts/build/`，
// 由 `__file__` 上溯取夹具根）⟷ 本真身（显式传同一夹具为 root）的 rc/stdout/stderr 三面逐字同；
// 时间面（`secs`/`wall_secs`/降序表）按件头口径归一。夹具落仓外（TempDir 自清）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// pssSha16 —— 输出指纹（前 16 个 hex，两态对拍证据面）。
func pssSha16(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

const pssTestCIYML = `name: CI
on:
  push:
    branches: [main]

jobs:
  core:
    name: core
    runs-on: ubuntu-24.04
    steps:
      - name: pass multiline
        run: |
          echo hello
          echo world
      - name: ctx skipped
        run: echo github ${{ secrets.TOKEN }}
      - name: git needs repo
        run: git status --porcelain
      - name: git config ok
        run: git config --global --get user.email || true
      - name: fail rc3
        run: |
          echo top
          echo mid
          exit 3
      - name: workdir present
        working-directory: subdir
        run: pwd
      - name: workdir absent
        working-directory: missingdir
        run: echo nope
  ui:
    name: ui
    runs-on: macos-14
    steps:
      - name: ui pass
        run: echo ui-ok
`

const pssTestReleaseYML = `name: Release
on:
  push:
    tags: ['v*']

jobs:
  gate:
    name: gate
    runs-on: ubuntu-24.04
    steps:
      - name: release pass
        run: echo release-ok
`

func TestPublishSnapshotParseWorkflow(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ci.yml")
	if err := os.WriteFile(p, []byte(pssTestCIYML), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := pssParseWorkflow(p)
	if len(jobs) != 2 || jobs[0].name != "core" || jobs[1].name != "ui" {
		t.Fatalf("作业序/名不对：%+v", jobs)
	}
	if len(jobs[0].steps) != 7 {
		t.Fatalf("core 步骤数 %d ≠ 7", len(jobs[0].steps))
	}
	if got := strings.Join(jobs[0].steps[0].run, "\n"); got != "echo hello\necho world" {
		t.Fatalf("块 run 解析不对：%q", got)
	}
	if jobs[0].steps[5].cwd != "subdir" {
		t.Fatalf("working-directory 解析不对：%q", jobs[0].steps[5].cwd)
	}
	if jobs[0].steps[6].cwd != "missingdir" {
		t.Fatalf("working-directory 解析不对：%q", jobs[0].steps[6].cwd)
	}
	if got := strings.Join(jobs[0].steps[4].run, "\n"); got != "echo top\necho mid\nexit 3" {
		t.Fatalf("失败步 run 解析不对：%q", got)
	}
}

func TestPublishSnapshotNeedsGitRepo(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"echo hi", false},
		{"git status --porcelain", true},
		{"git rev-parse HEAD", true},
		{"git config --global --get user.email || true", false}, // config 不算需要仓
		{"env git -C /x rev-list HEAD", true},                   // -C 形态
		{"python3 scripts/gates/check-manifest-freshness.py", true},
		{"echo x\n git push origin main", true},
	}
	for _, c := range cases {
		if got := pssNeedsGitRepo(c.body); got != c.want {
			t.Fatalf("needs_git_repo(%q)=%v want %v", c.body, got, c.want)
		}
	}
}

func TestPublishSnapshotHelpers(t *testing.T) {
	if pssPyFloat(0) != "0.0" || pssPyFloat(1) != "1.0" || pssPyFloat(0.5) != "0.5" {
		t.Fatalf("pssPyFloat 形态不对：%q %q %q", pssPyFloat(0), pssPyFloat(1), pssPyFloat(0.5))
	}
	if got := pssTailJoined("a\nb\nc\n"); got != "a\nb\nc" {
		t.Fatalf("pssTailJoined=%q", got)
	}
	if got := pssTailJoined(""); got != "" {
		t.Fatalf("空串应空：%q", got)
	}
	long := strings.Repeat("x", 30)
	for i := 0; i < 30; i++ {
		long += "\n" + strings.Repeat("y", 5)
	}
	got := strings.Split(pssTailJoined(long), "\n")
	if len(got) != 25 {
		t.Fatalf("尾行数 %d ≠ 25", len(got))
	}
	if pssClip("一二三四五", 3) != "一二三" {
		t.Fatalf("pssClip 未按码点截断：%q", pssClip("一二三四五", 3))
	}
	if got := pssJSONString("a\"b\\c\n"); got != `"a\"b\\c\n"` {
		t.Fatalf("pssJSONString=%q", got)
	}
	if pssLeadWS("    x") != 4 || pssLeadWS("\t y") != 2 {
		t.Fatalf("pssLeadWS 不对")
	}
}

// pssCanon —— 两态对拍前的归一：掩时间数值 + 把「逐步骤耗时」块内的行**排序**（该块是纯观测面 ·
// 见件头 ★；py 自己声明计时不改任何判据）。其余行保持原序逐字比。
var pssReFloat = regexp.MustCompile(`\d+\.\d+(?:[eE][-+]?\d+)?|\d+[eE][-+]?\d+`)

func pssCanon(s string) string {
	s = pssReFloat.ReplaceAllString(s, "<T>")
	lines := strings.Split(s, "\n")
	start, end := -1, -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "── 逐步骤耗时") {
			start = i
		}
		if start >= 0 && strings.HasPrefix(ln, "⇒ 通过") {
			end = i
			break
		}
	}
	if start >= 0 && end > start {
		mid := append([]string{}, lines[start+1:end]...)
		sort.Strings(mid)
		out := append([]string{}, lines[:start+1]...)
		out = append(out, mid...)
		out = append(out, lines[end:]...)
		lines = out
	}
	return strings.Join(lines, "\n")
}

// TestPublishSnapshotNoPrivatePathLiteral —— 自指消解声明成立吗（✗ 不写连续私有绝对路径原串）。
// 真身件落在公开面（`core/**`）内，而 `check-hardcoded-private-paths` 正扫该面 ⇒ 一条都不能有。
// 探针串按**拼串**构造（✗ 不直写整串 —— 直写本身就命中该门）。
func TestPublishSnapshotNoPrivatePathLiteral(t *testing.T) {
	b, err := os.ReadFile("family_publish_snapshot_native.go")
	if err != nil {
		t.Fatalf("哨兵失守：从 cwd 读不到真身件（✗ 不留 Skip · fail-closed）：%v", err)
	}
	src := string(b)
	banned := []string{
		"/Volumes" + "/AHZ",
		"/Users" + "/Mr2109",
	}
	for _, lit := range banned {
		if strings.Contains(src, lit) {
			t.Fatalf("真身件里出现连续私有绝对路径原串（应拼串/无）：%q", lit)
		}
	}
}

// TestPublishSnapshotParityLive —— 两态对拍（**默认跳过** · 设 `PUBLISH_SNAPSHOT_PARITY_LIVE=1` 才跑）：
// 仓外夹具上，旧 py 原件 ⟷ 本真身 的 rc/stdout/stderr 三面逐字同（掩时间）。夹具落仓外（TempDir 自清）。
func TestPublishSnapshotParityLive(t *testing.T) {
	if os.Getenv("PUBLISH_SNAPSHOT_PARITY_LIVE") != "1" {
		t.Skip("设 PUBLISH_SNAPSHOT_PARITY_LIVE=1 才跑两态对拍（要 python3 + 旧 py 原件）")
	}
	root := repoRoot()
	if root == "" {
		t.Fatalf("repoRoot() 解析不到仓根")
	}
	realPy := filepath.Join(root, "scripts", "build", "publish-snapshot-self-test.py")
	if _, err := os.Stat(realPy); err != nil {
		t.Fatalf("哨兵失守：旧 py 原件不在盘 %s（✗ 不留 Skip · fail-closed）：%v", realPy, err)
	}
	pySrc, err := os.ReadFile(realPy)
	if err != nil {
		t.Fatal(err)
	}
	pyBin, err := exec.LookPath("python3")
	if err != nil {
		t.Fatalf("哨兵失守：找不到 python3（✗ 不留 Skip · fail-closed）：%v", err)
	}

	mkRepo := func(withRelease bool) string {
		t.Helper()
		fx := t.TempDir()
		if err := os.MkdirAll(filepath.Join(fx, "scripts", "build"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fx, "scripts", "build", "publish-snapshot-self-test.py"), pySrc, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(fx, "publish", "ci"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fx, "publish", "ci", "ci.yml"), []byte(pssTestCIYML), 0o644); err != nil {
			t.Fatal(err)
		}
		if withRelease {
			if err := os.WriteFile(filepath.Join(fx, "publish", "ci", "release-agent.yml"), []byte(pssTestReleaseYML), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return fx
	}
	mkSnap := func(withGit bool) string {
		t.Helper()
		s := t.TempDir()
		if err := os.MkdirAll(filepath.Join(s, "subdir"), 0o755); err != nil {
			t.Fatal(err)
		}
		if withGit {
			c := exec.Command("git", "-C", s, "init", "-q", "-b", "main")
			c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
			if out, cerr := c.CombinedOutput(); cerr != nil {
				t.Fatalf("git init: %v\n%s", cerr, out)
			}
		}
		return s
	}

	type tc struct {
		name    string
		withRel bool
		withGit bool
		argv    []string
	}
	cases := []tc{
		{"人面·无.git", true, false, nil},
		{"人面·有.git", true, true, nil},
		{"json·无.git", true, false, []string{"--json"}},
		{"job=core", true, false, []string{"--job", "core"}},
		{"job=gate(第二件)", true, false, []string{"--job", "gate"}},
		{"job=不存在", true, false, []string{"--job", "nope"}},
	}
	for _, c := range cases {
		fx := mkRepo(c.withRel)
		snap := mkSnap(c.withGit)
		argv := append([]string{snap}, c.argv...)

		pyCmd := exec.Command(pyBin, append([]string{filepath.Join(fx, "scripts", "build", "publish-snapshot-self-test.py")}, argv...)...)
		var pOut, pErr strings.Builder
		pyCmd.Stdout, pyCmd.Stderr = &pOut, &pErr
		prc := pssExitCode(pyCmd.Run())

		var gOut, gErr strings.Builder
		grc := publishSnapshotSelfTestNative(fx, &gOut, &gErr, argv)

		if prc != grc {
			t.Fatalf("[%s] rc 不同：py=%d ⟷ Go=%d\n--py out--\n%s\n--go out--\n%s", c.name, prc, grc, pOut.String(), gOut.String())
		}
		if pssCanon(pOut.String()) != pssCanon(gOut.String()) {
			t.Fatalf("[%s] stdout 不同\n--py--\n%s\n--go--\n%s", c.name, pssCanon(pOut.String()), pssCanon(gOut.String()))
		}
		if pssCanon(pErr.String()) != pssCanon(gErr.String()) {
			t.Fatalf("[%s] stderr 不同\n--py--\n%q\n--go--\n%q", c.name, pssCanon(pErr.String()), pssCanon(gErr.String()))
		}
		t.Logf("[%s] OK rc=%d stdout_sha16=%s stderr_sha16=%s", c.name, prc,
			pssSha16(pssCanon(pOut.String())), pssSha16(pssCanon(pErr.String())))
	}

	// 出口 2 的两格：缺工作流真源 / 快照目录不存在（rc 与 stderr 正文逐字同）。
	{
		fx := mkRepo(false) // 缺 release-agent.yml
		snap := mkSnap(false)
		pyCmd := exec.Command(pyBin, filepath.Join(fx, "scripts", "build", "publish-snapshot-self-test.py"), snap)
		var pOut, pErr strings.Builder
		pyCmd.Stdout, pyCmd.Stderr = &pOut, &pErr
		prc := pssExitCode(pyCmd.Run())
		var gOut, gErr strings.Builder
		grc := publishSnapshotSelfTestNative(fx, &gOut, &gErr, []string{snap})
		if prc != grc || pssCanon(pErr.String()) != pssCanon(gErr.String()) {
			t.Fatalf("[缺工作流] rc py=%d go=%d\n--py err--\n%q\n--go err--\n%q", prc, grc, pErr.String(), gErr.String())
		}
		t.Logf("[缺工作流] OK rc=%d", prc)
	}
	{
		fx := mkRepo(true)
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		pyCmd := exec.Command(pyBin, filepath.Join(fx, "scripts", "build", "publish-snapshot-self-test.py"), missing)
		var pOut, pErr strings.Builder
		pyCmd.Stdout, pyCmd.Stderr = &pOut, &pErr
		prc := pssExitCode(pyCmd.Run())
		var gOut, gErr strings.Builder
		grc := publishSnapshotSelfTestNative(fx, &gOut, &gErr, []string{missing})
		if prc != grc || pssCanon(pErr.String()) != pssCanon(gErr.String()) {
			t.Fatalf("[快照缺] rc py=%d go=%d\n--py err--\n%q\n--go err--\n%q", prc, grc, pErr.String(), gErr.String())
		}
		t.Logf("[快照缺] OK rc=%d", prc)
	}
}
