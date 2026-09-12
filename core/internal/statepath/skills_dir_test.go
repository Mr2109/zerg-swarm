package statepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillsDir_EnvOverride — 环境变量优先（部署到别处时的逃生门）。
func TestSkillsDir_EnvOverride(t *testing.T) {
	t.Setenv("ZERG_SKILLS_DIR", "/tmp/some-skills")
	if got := SkillsDir(); got != "/tmp/some-skills" {
		t.Fatalf("ZERG_SKILLS_DIR 应优先生效，实得 %q", got)
	}
}

// TestSkillsDir_ResolvesRepoDir — 未设环境变量时解析到仓库内技能目录；该目录必须存在，
// 且至少含本次修复涉及的两个技能（这是「资源库能看到 CA 能加载的技能」的根）。
func TestSkillsDir_ResolvesRepoDir(t *testing.T) {
	t.Setenv("ZERG_SKILLS_DIR", "")
	d := SkillsDir()
	if d == "" {
		t.Fatal("未设 ZERG_SKILLS_DIR 时应能解析出仓库内技能目录（仓库根/core/internal/agent/skills）")
	}
	if !strings.HasSuffix(filepath.ToSlash(d), "core/internal/agent/skills") {
		t.Fatalf("解析结果不像技能目录：%s", d)
	}
	st, err := os.Stat(d)
	if err != nil || !st.IsDir() {
		t.Fatalf("解析出的目录不可用：%s (%v)", d, err)
	}
	for _, name := range []string{"code-optimization", "ca-troubleshooting"} {
		if _, err := os.Stat(filepath.Join(d, name, "SKILL.md")); err != nil {
			t.Errorf("技能 %s 应有 SKILL.md：%v", name, err)
		}
	}
}

// TestSkillsDir_NoGuessWhenMissing — 工作区指到空目录时返回空串（不猜、不假装有）。
func TestSkillsDir_NoGuessWhenMissing(t *testing.T) {
	t.Setenv("ZERG_SKILLS_DIR", "")
	t.Setenv("ZERG_WORKSPACE", t.TempDir())
	if got := SkillsDir(); got != "" {
		t.Fatalf("仓库内没有技能目录时应返回空串，实得 %q", got)
	}
}
