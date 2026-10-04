package api

// resource_skills_desc_test.go —— GAP-20261005-27 乙案：`/api/resources/skills` 透出**有真源**的 desc。
//
// 真源 = SKILL.md frontmatter 的 `description:`（口径同 core/internal/agent/skill_manager.go
// 的 parseFrontmatter）。本组判据（两态 + 负控 · 端点级 · 真走 HTTP）：
//
//	① 有真源（frontmatter `description:` 非空）⇒ item 含 `desc`，逐字等于真源；
//	② 无真源（无 frontmatter / 无 `description:` 行 / 空值）⇒ item **不含** `desc` 键（✗ 不发占位）；
//	③ version 无真源（技能无版本概念）⇒ **任何** item 都不含 `version` 键（✗ 禁伪造 v1.0.0 类）；
//	④ 负控（有牙）：拿掉透传 ⇒ ① 必红；塞回伪造 version ⇒ ③ 必红。
//
// 注：本件只钉**后端发什么字节**；UI 侧「缺字段留空」的诚实化（甲案）另有 ui/src/app.rs 的用例。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkillFixture 造一个技能目录（ResourcesHandler 只认含 SKILL.md 的目录）。
func writeSkillFixture(t *testing.T, skillsDir, name, skillMD string) {
	t.Helper()
	d := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("造技能目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatalf("造 SKILL.md 失败: %v", err)
	}
}

func TestResourcesSkillsDesc_SourcedOnly_GAP20261005_27(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	skillsDir := t.TempDir()
	t.Setenv("ZERG_SKILLS_DIR", skillsDir)
	setResTrustPaths(t, "", filepath.Join(t.TempDir(), "no-legacy.json"))
	_ = resetResourceTrust(t)

	// 甲：有真源（frontmatter description 非空）
	writeSkillFixture(t, skillsDir, "with-desc",
		"---\nname: with-desc\ndescription: 示例技能简介\n---\n\n正文\n")
	// 乙：无 frontmatter
	writeSkillFixture(t, skillsDir, "no-frontmatter", "# 无 frontmatter 的技能\n")
	// 丙：有 frontmatter 但无 description 行
	writeSkillFixture(t, skillsDir, "no-desc-line", "---\nname: no-desc-line\n---\n\n正文\n")
	// 丁：description 行为空
	writeSkillFixture(t, skillsDir, "empty-desc",
		"---\nname: empty-desc\ndescription:   \n---\n\n正文\n")

	h := newTestHandlers()
	r := newTrustRouter(h)

	items, raw := getResources(t, r, "skills")
	// 逐字读数：真发出去的字节（夹具面 · 确定）
	t.Logf("GET /api/resources/skills 响应体（逐字）: %s", raw)
	byName := map[string]map[string]interface{}{}
	for _, it := range items {
		if n, ok := it["name"].(string); ok {
			byName[n] = it
		}
	}
	// 夹具确实列到（空转不算绿）
	for _, n := range []string{"with-desc", "no-frontmatter", "no-desc-line", "empty-desc"} {
		if _, ok := byName[n]; !ok {
			t.Fatalf("skills 面没列到夹具技能 %q ⇒ 端点没跑到（body=%s）", n, raw)
		}
	}

	// ① 有真源 ⇒ desc 键在，且逐字等于真源
	if got, ok := byName["with-desc"]["desc"]; !ok {
		t.Fatalf("有真源的技能缺 desc 键 ⇒ 透传没生效（body=%s）", raw)
	} else if s, _ := got.(string); s != "示例技能简介" {
		t.Fatalf("desc 应为真源逐字 value，实得 %#v", got)
	}

	// ② 无真源 ⇒ **不含** desc 键（✗ 不发占位 / 空串）
	for _, n := range []string{"no-frontmatter", "no-desc-line", "empty-desc"} {
		if v, ok := byName[n]["desc"]; ok {
			t.Fatalf("技能 %q 无 desc 真源 ⇒ 不许发 desc 键（实得 %#v · body=%s）", n, v, raw)
		}
	}

	// ③ version 无真源 ⇒ 任何 item 都不许有 version 键
	for n, it := range byName {
		if v, ok := it["version"]; ok {
			t.Fatalf("技能 %q 无 version 真源 ⇒ 不许发 version 键（实得 %#v · ✗ 禁伪造 v1.0.0 类）", n, v)
		}
	}
	// ④ 响应体里也不许冒出伪造版本字面
	if strings.Contains(raw, "v1.0.0") {
		t.Fatalf("skills 面响应体出现伪造版本号 v1.0.0（body=%s）", raw)
	}
}
