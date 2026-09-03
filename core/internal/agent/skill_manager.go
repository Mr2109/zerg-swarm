package agent

// skill_manager.go — v2.5.1 CA 接入 skill（P1——Claude Code SKILL.md 标准）
// 设计：docs/设计-v2.5.1-CA接入skill-MCP.md
// 机制：启动扫描技能目录（name+description 注入系统提示）——skill_load 工具触发读正文

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill — 一个技能（SKILL.md）
type Skill struct {
	Name        string // frontmatter name
	Description string // frontmatter description（触发器）
	Path        string // SKILL.md 路径
	Body        string // 正文（触发时读取）
}

// SkillManager — 技能管理器
type SkillManager struct {
	skills map[string]*Skill
	dir    string // 技能目录
}

// NewSkillManager — 扫描技能目录
func NewSkillManager(dir string) *SkillManager {
	sm := &SkillManager{
		skills: make(map[string]*Skill),
		dir:    dir,
	}
	sm.scan()
	return sm
}

// scan — 扫描技能目录（每个子目录的 SKILL.md）
func (sm *SkillManager) scan() {
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(sm.dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		content := string(data)
		name, desc := parseFrontmatter(content)
		if name == "" {
			name = e.Name()
		}
		sm.skills[name] = &Skill{
			Name:        name,
			Description: desc,
			Path:        skillPath,
			Body:        content,
		}
	}
}

// parseFrontmatter — 解析 SKILL.md 的 YAML frontmatter（name/description）
func parseFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", ""
	}
	end := strings.Index(content, "\n---\n")
	if end == -1 {
		return "", ""
	}
	fm := content[4:end]
	var name, desc string
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		} else if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return name, desc
}

// ListDescriptions — 返回所有技能的 name+description（启动注入系统提示——轻量）
func (sm *SkillManager) ListDescriptions() string {
	if len(sm.skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【可用技能（任务匹配时用 skill_load 加载）】\n")
	names := make([]string, 0, len(sm.skills))
	for n := range sm.skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := sm.skills[n]
		sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
	}
	return sb.String()
}

// Load — 加载技能正文（skill_load 工具调用）
func (sm *SkillManager) Load(name string) (string, error) {
	s, ok := sm.skills[name]
	if !ok {
		return "", fmt.Errorf("技能 %s 不存在（可用: %s）", name, strings.Join(sm.SkillNames(), ", "))
	}
	return s.Body, nil
}

// SkillNames — 技能名列表
func (sm *SkillManager) SkillNames() []string {
	names := make([]string, 0, len(sm.skills))
	for n := range sm.skills {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
