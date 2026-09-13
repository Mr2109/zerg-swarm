package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSkillManagerSkipsAppleDoubleDirs —— ._* 目录与隐藏目录不得被当成技能。
// 场景来源：2026-09-14 跨机 tar 中转在 X3 留下 785 个 AppleDouble 文件（含 ._ca-troubleshooting 这类目录名）。
func TestSkillManagerSkipsAppleDoubleDirs(t *testing.T) {
	dir := t.TempDir()
	mk := func(sub string) {
		d := filepath.Join(dir, sub)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + sub + "\ndescription: 测试\n---\n正文\n"
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("real-skill")
	mk("._fake-skill")  // AppleDouble 残留
	mk(".hidden-skill") // 隐藏目录

	sm := NewSkillManager(dir)
	names := sm.SkillNames()
	if len(names) != 1 {
		t.Fatalf("应只加载 1 个技能，实际 %d 个: %v", len(names), names)
	}
	if names[0] != "real-skill" {
		t.Fatalf("加载到的应是 real-skill，实际 %q", names[0])
	}
}
