// docs_issues_not_recreated_test.go — §二十一 已红第 14 条（T-36「`issue_tracker.go:180` 会把 `docs/issues`
// 造回主仓」）的**成对负控**。
//
// 判据原话是：① `sed -n '176,183p'` 里不再无条件 `MkdirAll(workDir/docs/issues)`；② **跑一次挂单路径后**
// `[ -d docs/issues ]` 仍为 `NO`。本件把 ② 落成可复跑的两格：
//
//	① **落点在状态目录**（正控）：真跑 `CreateIssue` / `SubmitTask`，单子必须落在 `statepath.IssuesDir()` 下；
//	② **仓内零新增**（负控的正面表述）：跑完之后 `<workDir>/docs/issues` **不存在**
//	   （T-36 之前：无条件 MkdirAll ⇒ 一定存在）。
//
// 隔离纪律（同 issues_dir_config_test.go）：一律 `t.Setenv`（ZERG_STATE_DIR / ZERG_WORKSPACE → t.TempDir()），
// 不碰真机 ~/.zerg/state、不碰仓内 docs/issues、不写死绝对路径。
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// isolateIssuesState —— 测试卫生：把**落点真源**钉到本测试自己的临时状态目录。
//
// 为什么必须有（T-36 的副作用，必须显式处理）：本包 `TestMain` 会把整包的 `ZERG_STATE_DIR` 指到一个
// **共享**临时目录；落点改到 `statepath.IssuesDir()` 之前，`SubmitTask` 写的是各自 CWD 下的
// `docs/issues`（每个测试一个 t.TempDir ⇒ 天然互不可见）；改到真源之后，若某个测试不隔离，
// 它写的单子就会落在**共享**目录里，被 `TestScanIssues` 这类「读全量」的测试看见 ⇒ 测试间污染。
// 所以：凡真调 `SubmitTask`/`CreateIssue` 的测试，一律先调本助手。
func isolateIssuesState(t *testing.T) {
	t.Helper()
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv("ZERG_ISSUES_DIR", "")
}

func TestCreateIssueLandsInStateDirNotRepo(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", "")
	workDir := t.TempDir() // 假装这是「任务工作区 / 主仓根」

	path, err := CreateIssue(workDir, "T-36 夹具：挂一单", ReasonMaxTurns, 0, []string{"bash"})
	if err != nil {
		t.Fatalf("CreateIssue：%v", err)
	}
	// 正控①：落在状态目录下
	wantPrefix := statepath.IssuesDir()
	if abs, aerr := filepath.Abs(path); aerr == nil {
		path = abs
	}
	if rel, rerr := filepath.Rel(wantPrefix, path); rerr != nil || rel == "" || rel[:1] == "." {
		t.Errorf("单子应落在 statepath.IssuesDir()=%s 下，实际 %s", wantPrefix, path)
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Errorf("单子没真写出来：%v", serr)
	}
	// 负控②：工作区里的 docs/issues **必须不存在**（T-36 的病灶就是它会存在）
	if _, serr := os.Stat(filepath.Join(workDir, "docs", "issues")); serr == nil {
		t.Errorf("`<workDir>/docs/issues` 又被造出来了（已红第 14 条的形态）：%s", filepath.Join(workDir, "docs", "issues"))
	}
	// 成对：工区里连 docs 这一层都不该被建
	if _, serr := os.Stat(filepath.Join(workDir, "docs")); serr == nil {
		t.Errorf("`<workDir>/docs` 不该被创建")
	}
}

func TestSubmitTaskLandsInStateDirNotRepo(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", "")

	path, err := SubmitTask(TaskSpec{Task: "T-36 夹具：派一单", Type: "code", Priority: "normal", Source: "api"})
	if err != nil {
		t.Fatalf("SubmitTask：%v", err)
	}
	if rel, rerr := filepath.Rel(statepath.IssuesDir(), path); rerr != nil || rel[:1] == "." {
		t.Errorf("派单应落在 statepath.IssuesDir() 下，实际 %s", path)
	}
	// 负控：**按 CWD 选址**的老路会在当前工作目录下造 docs/issues（本测试的 CWD = core/internal/agent）
	if _, serr := os.Stat(filepath.Join("docs", "issues")); serr == nil {
		abs, _ := filepath.Abs("docs/issues")
		t.Errorf("CWD 下又被造出 docs/issues（老路是按 CWD 选址）：%s", abs)
	}
}

// TestIssuesDirIsRepoExternal —— 结构格：落点真源必须在**仓外**（状态目录或 ZERG_ISSUES_DIR）。
func TestIssuesDirIsRepoExternal(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", "")
	d := statepath.IssuesDir()
	if !filepath.IsAbs(d) {
		t.Errorf("落点应是绝对路径（不依赖 CWD）：%q", d)
	}
	if rel, err := filepath.Rel(stateDir, d); err != nil || rel[:1] == "." {
		t.Errorf("默认落点应在状态目录内：%q ∉ %q", d, stateDir)
	}
	// 成对：显式覆盖时走覆盖值（覆盖即唯一来源）
	over := t.TempDir()
	t.Setenv("ZERG_ISSUES_DIR", over)
	if got := statepath.IssuesDir(); got != over {
		t.Errorf("显式覆盖必须生效：%q ≠ %q", got, over)
	}
	if !statepath.IssuesDirExplicit() {
		t.Errorf("IssuesDirExplicit 应报「已显式覆盖」")
	}
}

// TestNoRepoLocalIssuesWriteSiteInAgent —— 字面量棘轮（把判据①做成每次 go test 都跑的机检）。
//
// 判据①原话：`sed -n '176,183p' core/internal/agent/issue_tracker.go` 里不再有无条件
// `MkdirAll(workDir/docs/issues)`。按内容扫（不是按行号——行号会漂）。
//
// 口径（**只认写/建，不认只读**）：本包非测试源码里，`"docs", "issues"` 这个落点字符串附近
// 不得再出现 `MkdirAll(` / `os.WriteFile(` / `os.Create(` —— 那才是「造回主仓」。
// `scheduler.go` 里那处是**只读并入**旧目录（2026-09-19 已拍：人类手写单照旧被派），
// 它不建目录、不写文件 ⇒ 不该被这条棘轮判红（第一次跑就误判过它一次，故口径写死在这里）。
func TestNoRepoLocalIssuesWriteSiteInAgent(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	const needle = `"docs", "issues"`
	writeCalls := []string{"MkdirAll(", "os.WriteFile(", "os.Create("}
	var hits []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, rerr := os.ReadFile(name)
		if rerr != nil {
			t.Fatal(rerr)
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || !strings.Contains(line, needle) {
				continue
			}
			lo, hi := i-3, i+3
			if lo < 0 {
				lo = 0
			}
			if hi >= len(lines) {
				hi = len(lines) - 1
			}
			for k := lo; k <= hi; k++ {
				c := strings.TrimSpace(lines[k])
				if strings.HasPrefix(c, "//") {
					continue
				}
				for _, w := range writeCalls {
					if strings.Contains(c, w) {
						hits = append(hits, fmt.Sprintf("%s:%d（附近有 %s）", name, i+1, w))
					}
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("本包非测试源码里仍有「仓内 docs/issues + 写/建」落点（会造回主仓）：%s", strings.Join(hits, " · "))
	}
}
