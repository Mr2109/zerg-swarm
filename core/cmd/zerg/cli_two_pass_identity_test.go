// cli_two_pass_identity_test.go —— 「同一判据跑两遍 · 逐字对拍（防假绿）」的 **CLI 面**机检。
//
// 真源（逐字 · 不转述）
// --------------------
// `Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md` §一 序120
// （组4 §二.3 `W-37` · `研-归:70` `C3`/`C4`/`Q-009` · `研-归:71` `C4`/`E4`/`Q-011`）判据栏逐字：
//
//	「① 同一判据跑两遍逐字对拍（两次输出 `shasum -a 256` 同值）
//	 ② 「零副作用」证明：只对只读 / 已封的基准出证明；对可变的件不许假装证明，
//	    只许报「无可比基准 · rc=8」（不许给绿）」
//
// `C4` 的条件逐字（`承接自-v2.5.11/调研-缺口-归档台面-20260923.md:71` · `man hdiutil` 口径）：
//
//	「**条件 = 只对「只读 / 已封」的基准出零副作用证明**」
//
// 件面分工（本波两件 · 同一判据的**两个面**，不各写一份判据）：
//
//	· 本件 = `core/cmd/zerg/` 那一半：**CLI 面**（跑当前源码 `zerg.RunForTest`，不看盘上旧制品）
//	· `scripts/gates/check-two-pass-identity.py` = 判据件那一半（门㊵ · 对 `check-zero-side-effect.py`
//	  的机器读面跑两遍对拍 + 对象口径分档）
//
// 判据（三格 · 每格都能单独失败）
// ------------------------------
//
//	① `TestTwoPassIdentityReadOnlySameBytes`：
//	   一批**只读**命令各跑**两遍** ⇒ 退码相同 · `stdout+stderr` 的 `sha256` **同值** · `stdout` 非空（防空转）。
//	② `TestTwoPassIdentityComparatorNotVacuous`：
//	   **负控** —— 比较器对**不同字节**必报不同（否则① 可能只是「什么都没比」的假绿）。
//	③ `TestTwoPassIdentityMutableFileNoBaseline`：
//	   **可变件不许假装证明** —— 合成 git 仓里：**已封件**（tracked + 与 `HEAD` blob 逐字节相同）
//	   ⇒ 出结论；**可变件**（跟踪件被改过 / 未跟踪件）⇒ 逐字「**无可比基准**」+ 档位 `8`（**不给绿**）。
//
// 纪律：全程**只读**真仓（不起子进程跑 `zerg`；只在本用例自建的临时目录里跑 `git`，且**先清 `GIT_*` 环境**
// —— 波11 序125（`W-43`）那条通用纪律）；不写真仓一个字节；不碰状态目录。
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// rcNoBaseline —— 「**无可比基准**」那一档（与 `check-zero-side-effect.py` / 门㊵ 同值：8）。
const rcNoBaseline = 8

// noBaselineText —— 判据栏逐字要求的那句话（可变件只许报它，不许给绿）。
const noBaselineText = "无可比基准"

// twoPassRun —— 一次 CLI 调用的可对拍快照。
type twoPassRun struct {
	rc   int
	out  string
	errb string
	sha  string // sha256(stdout ‖ stderr)
}

// runOnce —— 用**当前源码**跑一条命令（同 `cli_help_twostate_test.go` 的 `zerg.RunForTest` 一条路）。
func runOnce(argv ...string) twoPassRun {
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	sum := sha256.Sum256(append(append([]byte{}, out.Bytes()...), errb.Bytes()...))
	return twoPassRun{rc: rc, out: out.String(), errb: errb.String(), sha: hex.EncodeToString(sum[:])}
}

// compareRuns —— 逐字对拍器：**只有** sha 同值且退码同才算「同值」。
func compareRuns(a, b twoPassRun) bool { return a.rc == b.rc && a.sha == b.sha }

// TestTwoPassIdentityReadOnlySameBytes —— ① 只读命令两跑逐字同值。
func TestTwoPassIdentityReadOnlySameBytes(t *testing.T) {
	cases := []struct {
		what string
		argv []string
	}{
		{"裸 --help（全局树）", []string{"--help"}},
		{"help（全局树）", []string{"help"}},
		{"help --all（危险动作逐条列全）", []string{"help", "--all"}},
		{"gate ls（只读列门表）", []string{"gate", "ls"}},
	}
	for _, c := range cases {
		first := runOnce(c.argv...)
		second := runOnce(c.argv...)
		if first.out == "" {
			t.Fatalf("%s：stdout 空 ⇒ **空转**（空输出两跑当然同值）—— 不当绿", c.what)
		}
		if !compareRuns(first, second) {
			t.Fatalf("%s：两跑**不同值** ✗ rc %d/%d · sha %s/%s（差异不在「只读」面 ⇒ 该命令算不上只读判据）",
				c.what, first.rc, second.rc, first.sha[:12], second.sha[:12])
		}
		t.Logf("✓ %s：rc=%d · sha256=%s…（两跑同值 · stdout %d 字节）",
			c.what, first.rc, first.sha[:12], len(first.out))
	}
}

// TestTwoPassIdentityComparatorNotVacuous —— ② 负控：比较器对**不同字节**必报不同。
func TestTwoPassIdentityComparatorNotVacuous(t *testing.T) {
	a := twoPassRun{rc: 0, out: "体甲"}
	b := twoPassRun{rc: 0, out: "体乙"}
	sa := sha256.Sum256([]byte(a.out))
	sb := sha256.Sum256([]byte(b.out))
	a.sha = hex.EncodeToString(sa[:])
	b.sha = hex.EncodeToString(sb[:])
	if compareRuns(a, b) {
		t.Fatal("负控失败：**不同**的两跑被比较器报成同值 ⇒ ① 的「同值」是空转的假绿")
	}
	same := twoPassRun{rc: 0, out: "体甲"}
	same.sha = a.sha
	if !compareRuns(a, same) {
		t.Fatal("对界失败：**逐字节相同**的两跑被报成不同值 ⇒ 比较器无差别")
	}
	// 退码也必须在判据里（只比 stdout 会让「两跑不同码」漏过）。
	rcDiff := twoPassRun{rc: 2, out: "体甲", sha: a.sha}
	if compareRuns(a, rcDiff) {
		t.Fatal("负控失败：退码不同（0 vs 2）被报成同值 ⇒ 对拍漏掉了退码面")
	}
	t.Log("✓ 负控：不同字节 ⇒ 报不同 · 同字节 ⇒ 报同值 · 退码不同 ⇒ 报不同")
}

// cleanGitEnv —— 对外仓 / 合成仓的每一次 git 调用都要**先清环境**（波11 序125 · `W-43`）。
func cleanGitEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "GIT_DIR="), strings.HasPrefix(e, "GIT_WORK_TREE="),
			strings.HasPrefix(e, "GIT_INDEX_FILE="), strings.HasPrefix(e, "GIT_PREFIX="),
			strings.HasPrefix(e, "GIT_COMMON_DIR="), strings.HasPrefix(e, "GIT_OBJECT_DIRECTORY="),
			strings.HasPrefix(e, "GIT_ALTERNATE_OBJECT_DIRECTORIES="):
			continue
		default:
			env = append(env, e)
		}
	}
	return append(env,
		"GIT_AUTHOR_NAME=two-pass", "GIT_AUTHOR_EMAIL=two-pass@invalid",
		"GIT_COMMITTER_NAME=two-pass", "GIT_COMMITTER_EMAIL=two-pass@invalid")
}

func gitRun(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cleanGitEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), out.String()
	}
	t.Fatalf("git %v：%v", args, err)
	return -1, ""
}

// classifyBaseline —— 「只读 / 已封」才算有基准（`C4` 条件）。
//   - 未跟踪件（`git status --porcelain` 有 `??`）⇒ **可变** ⇒ 无可比基准
//   - 跟踪件但工作树与 `HEAD` 不等（`M`/`D`/`A`）⇒ **可变** ⇒ 无可比基准
//   - 跟踪件且与 `HEAD` blob 逐字节相同 ⇒ **已封** ⇒ 可出结论
func classifyBaseline(t *testing.T, repo, path string) (int, string) {
	t.Helper()
	_, st := gitRun(t, repo, "status", "--porcelain", "--", path)
	if strings.TrimSpace(st) == "" {
		return 0, "已封基准（tracked · 与 HEAD blob 逐字节相同）"
	}
	return rcNoBaseline, noBaselineText
}

// TestTwoPassIdentityMutableFileNoBaseline —— ③ 可变件不许假装证明。
func TestTwoPassIdentityMutableFileNoBaseline(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("无 git ⇒ 本格不给结论（不假装绿）")
	}
	dir := t.TempDir()
	if rc, out := gitRun(t, dir, "init", "-q"); rc != 0 {
		t.Fatalf("git init 失败：%s", out)
	}
	sealed := filepath.Join(dir, "sealed.txt")
	mutable := filepath.Join(dir, "mutable.txt")
	if err := os.WriteFile(sealed, []byte("已封件体\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutable, []byte("可变件体（初值）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, out := gitRun(t, dir, "add", "sealed.txt", "mutable.txt"); rc != 0 {
		t.Fatalf("git add 失败：%s", out)
	}
	if rc, out := gitRun(t, dir, "commit", "-q", "-m", "夹具"); rc != 0 {
		t.Fatalf("git commit 失败：%s", out)
	}
	// ① 已封件：出结论（有基准）
	if rc, note := classifyBaseline(t, dir, sealed); rc != 0 || strings.Contains(note, noBaselineText) {
		t.Fatalf("已封件被判成「%s」（rc=%d）—— 条件只许对可变件报无可比基准", note, rc)
	}
	// ② 可变件 A：跟踪件被改过
	if err := os.WriteFile(mutable, []byte("可变件体（改过）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, note := classifyBaseline(t, dir, mutable); rc != rcNoBaseline || !strings.Contains(note, noBaselineText) {
		t.Fatalf("被改过的跟踪件没报「%s · rc=8」（rc=%d · note=%q）—— 可变件不许假装证明",
			noBaselineText, rc, note)
	}
	// ③ 可变件 B：未跟踪件
	untracked := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("未跟踪体\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, note := classifyBaseline(t, dir, untracked); rc != rcNoBaseline || !strings.Contains(note, noBaselineText) {
		t.Fatalf("未跟踪件没报「%s · rc=8」（rc=%d · note=%q）", noBaselineText, rc, note)
	}
	t.Logf("✓ 三态可辨：已封件 ⇒ 出结论 · 被改过的跟踪件 ⇒ %s（rc=%d） · 未跟踪件 ⇒ %s（rc=%d）",
		noBaselineText, rcNoBaseline, noBaselineText, rcNoBaseline)
}
