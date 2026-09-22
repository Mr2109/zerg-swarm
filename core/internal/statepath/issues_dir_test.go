package statepath

// issues_dir_test.go — 「内部任务单目录可配」（2026-09-19 Mr2109已拍）用例。
//
// 契约（逐条钉死）:
//   ① ZERG_ISSUES_DIR 显式设置 ⇒ 原样返回（唯一来源 = 覆盖即已指定，不 stat、不回退旧目录）；
//   ② 未设 ⇒ <ZERG_STATE_DIR>/issues（默认 ~/.zerg/state/issues/）；
//   ③ 不猜、不假装有：默认值既不落仓内 <仓库>/docs/issues，也不写成任何 /tmp 字面量 ——
//      判据形态 = **等于** <ZERG_STATE_DIR>/issues（2026-09-23 波B 改判：旧形态「不以 /tmp/ 开头」
//      在 TMPDIR=/tmp 下恒假；见本件 TestIssuesDir_NoGuess 内注 · 设计-CI适配-v1.1 §五 第二批 #1）。
//
// 变异自证: 把默认值改回 <仓库>/docs/issues ⇒ ② 必红（TestIssuesDir_DefaultDerivedFromStateDir）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIssuesDir_EnvOverride — 显式覆盖优先（部署/沙箱/测试隔离的逃生门）。
func TestIssuesDir_EnvOverride(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", filepath.Join(t.TempDir(), "state")) // 覆盖态下状态目录不参与解析
	want := filepath.Join(t.TempDir(), "custom-issues")
	t.Setenv("ZERG_ISSUES_DIR", want)
	if got := IssuesDir(); got != want {
		t.Fatalf("ZERG_ISSUES_DIR 应优先生效，实得 %q", got)
	}
	if !IssuesDirExplicit() {
		t.Fatal("设了 ZERG_ISSUES_DIR 就该被认定为显式覆盖（否则扫单会并入旧目录）")
	}
}

// TestIssuesDir_DefaultDerivedFromStateDir — 未设 ⇒ <ZERG_STATE_DIR>/issues。
func TestIssuesDir_DefaultDerivedFromStateDir(t *testing.T) {
	t.Setenv("ZERG_ISSUES_DIR", "")
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("ZERG_STATE_DIR", stateDir)

	want := filepath.Join(stateDir, "issues")
	if got := IssuesDir(); got != want {
		t.Fatalf("未设 ZERG_ISSUES_DIR 时应派生 %s，实得 %q", want, got)
	}
	if IssuesDirExplicit() {
		t.Fatal("未设 ZERG_ISSUES_DIR 不该被当成显式覆盖（否则旧目录不再并入 = 人类单漏派）")
	}
}

// TestIssuesDir_NoGuess — 不猜、不假装有、不回退旧目录。
func TestIssuesDir_NoGuess(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", filepath.Join(t.TempDir(), "state"))

	// ① 显式指向不存在的路径 ⇒ 原样返回（不 stat、不建目录、绝不回退仓内 docs/issues）
	missing := filepath.Join(t.TempDir(), "nope", "issues")
	t.Setenv("ZERG_ISSUES_DIR", missing)
	if got := IssuesDir(); got != missing {
		t.Fatalf("覆盖路径不存在时也必须原样返回（不退旧），实得 %q", got)
	}

	// ② 默认值锚点：不是仓内旧目录、也不是 /tmp
	t.Setenv("ZERG_ISSUES_DIR", "")
	def := filepath.ToSlash(IssuesDir())
	if strings.HasSuffix(def, "/docs/issues") {
		t.Fatalf("默认值不得落在仓内旧目录 docs/issues: %s", def)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #1 · §八 待拍 2 裁定）：
	//   旧形态「不以 /tmp/ 开头」在 Linux runner（TMPDIR 未设 ⇒ t.TempDir() 自身就落在 /tmp 下）**恒假**
	//   —— 它不是判据，是自伤（本机 `TMPDIR=/tmp` 复现，9 条家族同机制）。
	//   真命题 = **等于**由 ZERG_STATE_DIR 派生出的值：派生值本身**可以合法落在临时根下**（临时根本身就是
	//   合法状态目录），要判的是「走没走统一状态目录派生」（落仓内旧目录由上面那条判）。
	if want := filepath.ToSlash(filepath.Join(os.Getenv("ZERG_STATE_DIR"), "issues")); def != want {
		t.Fatalf("默认值应等于 <ZERG_STATE_DIR>/issues（派生值）: want %s, got %s", want, def)
	}
	if !strings.HasSuffix(def, "/state/issues") {
		t.Fatalf("默认值应是 <状态目录>/issues，实得 %s", def)
	}
}
