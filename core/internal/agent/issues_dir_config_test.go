package agent

// issues_dir_config_test.go — 「内部任务单目录可配」（2026-09-19 Mr2109已拍）用例。
//
// 三条契约（各自可变异自证）:
//   T1 写只写新目录（statepath.IssuesDir()）——旧目录（<仓库>/docs/issues）零新增；
//   T2 未显式覆盖时旧目录里的 open 单仍被扫到（读旧一次 = 迁移三原则之一）；
//   T3 显式覆盖 ZERG_ISSUES_DIR ⇒ 写落该处 且 旧目录不再被扫（覆盖即唯一来源，不退旧）。
//
// 旧文件不搬不删不改: T1 用「旧目录条目逐字不变」钉住（dirFileNames 复用 state_paths_migrate_test.go）。
// 隔离纪律: 一律 t.Setenv（ZERG_STATE_DIR/ZERG_WORKSPACE 指向 t.TempDir()）——
// 不碰真机 ~/.zerg/state、不碰仓内 docs/issues、不依赖 /tmp 真目录、不写死绝对路径。

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// writeConfiguredIssue 在指定目录造一篇**粗体**状态单（能被 ScanIssues 解析的形态）。
// 用任意目录（不是 createTestIssue 的 workDir/docs/issues），便于造「新目录 / 旧目录 / 覆盖目录」三处。
func writeConfiguredIssue(t *testing.T, dir, name, priority, status string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("造单建目录失败 %s: %v", dir, err)
	}
	now := time.Now().Format(time.RFC3339)
	content := fmt.Sprintf(`# Issue: %s

<!-- task-hash: cfg-hash-%s -->
<!-- priority: %s -->
- **任务**: cfg task %s
- **状态**: %s
- **失败类型**: cfg_error
- **重试次数**: 0
- **创建时间**: %s
- **最后尝试**: %s
`, name, name, priority, name, status, now, now)
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写单失败 %s: %v", path, err)
	}
	return path
}

// newWriteProbe 造一个「空 issueDir」（= 派生态，同 main.go:377 注入态）的空闲检测器，
// 触发一次内部任务，返回本次写出的任务单路径。
func newWriteProbe(t *testing.T) (d *IdleDetector, written *string) {
	t.Helper()
	written = new(string)
	d = NewIdleDetector("") // 空 = 走 statepath.IssuesDir() 派生
	d.SetExternalQueue(func() int { return 0 })
	d.SetResourceIdle(func() bool { return true })
	d.SetOnTrigger(func(def InternalTask, issuePath string) { *written = issuePath })
	d.enabled = true
	return d, written
}

// T1 新单只落新目录：造一单 ⇒ 新目录出现、旧目录零新增（旧文件不搬不删不改）。
// 变异自证: 写路径改回 <仓库>/docs/issues ⇒ 新目录 0 篇 ⇒ 必红。
func TestConfiguredIssuesDir_T1_EngineWritesOnlyNewDir(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", "") // 未显式覆盖 ⇒ 默认派生 <ZERG_STATE_DIR>/issues
	repo := filepath.Join(tmp, "repo")
	t.Setenv("ZERG_WORKSPACE", repo)

	// 旧目录（改动前引擎写死的落点 = <仓库>/docs/issues）先放一篇旧单，作「零新增」对照
	legacyDir := filepath.Join(repo, "docs", "issues")
	writeConfiguredIssue(t, legacyDir, "legacy-human-issue", "high", "open")
	before := dirFileNames(t, legacyDir)

	d, written := newWriteProbe(t)
	d.checkAndTrigger()

	newDir := filepath.Join(stateDir, "issues")
	files, _ := filepath.Glob(filepath.Join(newDir, "internal-*.md"))
	if len(files) != 1 {
		t.Fatalf("新单应落新目录 %s，实得 %d 篇（0 篇 = 写回了旧目录 ⇒ 必红）", newDir, len(files))
	}
	if !strings.HasPrefix(*written, newDir+string(filepath.Separator)) {
		t.Fatalf("回传给主调度器的任务单路径应在 %s 内，实得 %q", newDir, *written)
	}
	after := dirFileNames(t, legacyDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("旧目录被写/删/搬了：%v → %v（旧文件不搬不删不改）", before, after)
	}
}

// T2 旧目录只读一次：未覆盖时旧目录里的 open 单仍能被扫到（新 ∪ 旧）。
// 变异自证: 把旧目录从扫描面拿掉 ⇒ 只剩新目录 1 篇 ⇒ 必红。
func TestConfiguredIssuesDir_T2_LegacyStillScannedWhenNotOverridden(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", "")
	// ③ 隔离纪律（2026-09-19 12:31 实测教训）：ZERG_WORKSPACE 也必须指到临时目录——
	// 否则任何把写/扫路径改成 statepath.WorkspaceRoot() 派生的变异/回归都会**直接往真仓库
	// docs/issues/ 写单**（本轮 M2 变异自证时真发生了，产物已清理）。用例自己不许能碰到真仓。
	t.Setenv("ZERG_WORKSPACE", filepath.Join(tmp, "repo"))
	workDir := filepath.Join(tmp, "work")
	createTestWorkersConfig(t, workDir, 2)

	legacyPath := writeConfiguredIssue(t, filepath.Join(workDir, "docs", "issues"), "legacy-open", "high", "open")
	newPath := writeConfiguredIssue(t, filepath.Join(stateDir, "issues"), "new-open", "normal", "open")

	s := NewScheduler(workDir)
	s.SetRunner(func(string) int { return 0 })
	issues, err := s.ScanIssues()
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	got := map[string]string{}
	for _, is := range issues {
		got[is.InstanceID] = is.Path
	}
	if len(issues) != 2 {
		t.Fatalf("未覆盖时扫描面应为 新 ∪ 旧 = 2，实得 %d: %v", len(issues), got)
	}
	if got["legacy-open"] != legacyPath {
		t.Fatalf("旧目录活单应被扫到（读旧一次）：期望 %s，实得 %q", legacyPath, got["legacy-open"])
	}
	if got["new-open"] != newPath {
		t.Fatalf("新目录活单应被扫到：期望 %s，实得 %q", newPath, got["new-open"])
	}
}

// T3 显式覆盖生效且不退旧：设 ZERG_ISSUES_DIR ⇒ 写落该处 且 旧目录（及其派生源）不再被扫。
// 变异自证: 把「覆盖即短路」的回退逻辑加回去 ⇒ 扫到 3 篇 ⇒ 必红。
func TestConfiguredIssuesDir_T3_ExplicitOverrideWinsAndSkipsLegacy(t *testing.T) {
	tmp := t.TempDir()
	overrideDir := filepath.Join(tmp, "override-issues")
	stateDir := filepath.Join(tmp, "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	t.Setenv("ZERG_ISSUES_DIR", overrideDir) // 显式覆盖 = 唯一来源
	// ③ 隔离纪律（同 T2）：ZERG_WORKSPACE 必须指到临时目录——覆盖态下若实现回退到
	// statepath.WorkspaceRoot()/docs/issues，不许把单写进真仓库（M2 变异自证实测过）。
	t.Setenv("ZERG_WORKSPACE", filepath.Join(tmp, "repo"))
	workDir := filepath.Join(tmp, "work")
	createTestWorkersConfig(t, workDir, 2)

	// 覆盖态下这两处**都不得**被扫：旧目录（<workDir>/docs/issues）与派生源（<ZERG_STATE_DIR>/issues）
	writeConfiguredIssue(t, filepath.Join(workDir, "docs", "issues"), "legacy-open", "high", "open")
	writeConfiguredIssue(t, filepath.Join(stateDir, "issues"), "derived-open", "high", "open")
	writeConfiguredIssue(t, overrideDir, "override-open", "high", "open")

	// 写侧：引擎只写覆盖目录
	d, written := newWriteProbe(t)
	d.checkAndTrigger()
	if files, _ := filepath.Glob(filepath.Join(overrideDir, "internal-*.md")); len(files) != 1 {
		t.Fatalf("写应落覆盖目录 %s，实得 %d 篇（路径=%q）", overrideDir, len(files), *written)
	}
	if !strings.HasPrefix(*written, overrideDir+string(filepath.Separator)) {
		t.Fatalf("写路径应在覆盖目录内，实得 %q", *written)
	}
	if files, _ := filepath.Glob(filepath.Join(stateDir, "issues", "internal-*.md")); len(files) != 0 {
		t.Fatalf("覆盖态下不得回退派生目录：%v", files)
	}

	// 扫侧：只扫覆盖目录（旧目录 + 派生目录完全不看）
	s := NewScheduler(workDir)
	s.SetRunner(func(string) int { return 0 })
	issues, err := s.ScanIssues()
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(issues) != 1 || issues[0].InstanceID != "override-open" {
		ids := make([]string, 0, len(issues))
		for _, is := range issues {
			ids = append(ids, is.InstanceID)
		}
		t.Fatalf("覆盖态应只扫到 [override-open]，实得 %v", ids)
	}
}
