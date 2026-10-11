// cli_help_skill_test.go —— 缺口 `GAP-20260926-49`（技能库体检应有 CLI 正门）的机检。
//
// 判据（成对四档）：
//
//	① 正控：`zerg help skill` ⇒ rc=0，出体检格（顶层条目 / SKILL.md / .md 三个**现读**读数）。
//	② 负控：根不可达 ⇒ 仍 rc=0（帮助面），但**照实标「不判」**、**不伪造读数**（不当绿）。
//	③ 负控：认不出的主题仍走「未知帮助主题」rc=2（本面**没把出口放宽**）。
//	④ 对界：注入了技能库根也不影响 `help dangerous` 那类既有主题（本面只**加一个主题**）。
//
// 跑的是**当前源码**（`zerg.RunForTest`，不起子进程、不碰状态目录）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillFixtureForTest —— 造一棵**合成**技能库（✗ 不碰真技能库）：顶层 3 条 · SKILL.md 2 件 · .md 3 件。
func skillFixtureForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("造夹具失败：%v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("造夹具失败：%v", err)
		}
	}
	mk("alpha/SKILL.md", "# alpha\n")
	mk("beta/SKILL.md", "# beta\n")
	mk("beta/notes.md", "notes\n")
	mk("top.md", "top\n")
	return root
}

// ① 正控：体检格三个读数额。
func TestHelpSkill_DoctorGrid(t *testing.T) {
	t.Setenv("ZERG_SKILLS_ROOT", skillFixtureForTest(t))
	rc, out, errb := helpRun(t, "help", "skill")
	if rc != 0 {
		t.Fatalf("`zerg help skill` 要 rc=0（改前 rc=2「未知帮助主题 skill」），得到 rc=%d · stderr=%q", rc, errb)
	}
	for _, want := range []string{"技能库体检", "体检格", "顶层条目   3", "SKILL.md   2", ".md        4"} {
		if !strings.Contains(out, want) {
			t.Errorf("`zerg help skill` 体检格缺 %q：\n%s", want, out)
		}
	}
}

// ② 负控：根不可达 ⇒ 不判、不给结论、不伪造读数。
func TestHelpSkill_UnreachableRootNoVerdict(t *testing.T) {
	t.Setenv("ZERG_SKILLS_ROOT", filepath.Join(t.TempDir(), "nope-zz"))
	rc, out, errb := helpRun(t, "help", "skill")
	if rc != 0 {
		t.Fatalf("根不可达时帮助面仍要 rc=0（判红档在 `-48` 门），得到 rc=%d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "不判") {
		t.Errorf("根不可达时**要照实标「不判」**（✗ 不静默当绿）：\n%s", out)
	}
	if strings.Contains(out, "顶层条目   ") {
		t.Errorf("根不可达 ⇒ **不许伪造体检读数**（「顶层条目」那一行不该出现）：\n%s", out)
	}
}

// ③ 负控：认不出的主题仍走「未知帮助主题」rc=2（出口没放宽）。
func TestHelpSkill_NegativeControlUnknownTopicRefused(t *testing.T) {
	rc, _, errb := helpRun(t, "help", "skillzz-没有这个主题-zz")
	if rc != 2 || !strings.HasPrefix(errb, "zerg: 未知帮助主题") {
		t.Errorf("认不出的主题仍要 rc=2 + 「未知帮助主题」（出口没放宽），得到 rc=%d · stderr=%q", rc, errb)
	}
}

// ④ 对界：注入技能库根不影响既有主题（只加一个主题，既有面一字不动）。
func TestHelpSkill_ExistingTopicsUntouched(t *testing.T) {
	t.Setenv("ZERG_SKILLS_ROOT", skillFixtureForTest(t))
	rc, out, errb := helpRun(t, "help", "dangerous")
	if rc != 0 || !strings.Contains(out, "危险动作清单") {
		t.Errorf("`zerg help dangerous` 应一字不动：rc=%d · stderr=%q · out首行=%q", rc, errb, firstLineForSkillTest(out))
	}
}

func firstLineForSkillTest(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
