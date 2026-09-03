// Package api - v2.5.6 程序定量驱动 Agent: Skill 机制
// 每个任务一个 SKILL.md——指导执行——执行后进化（自我完善）
// 类型判断（模型）→ 查/建（程序）→ 执行 → 总结 → 进化
// 2026-08-25 Mr2109——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 10/15 章
// v2.5.6 机制调整（2026-08-27 Mr2109）: skill 归属=任务独属——
//   内部任务（SkillKey=def.ID）无 skill 时按类型给起步——沉淀后优化版成为该任务独属 skill（skills/<key>/）
//   再次运行默认调独属 skill（有独属用独属——没有再回退类型库——最后模板）
// v2.5.6 候选机制（2026-08-28 Mr2109）: 进化产出先存候选（SKILL.candidate.md）——不直接覆盖正式版——
//   下次任务用候选试跑——成功转正（候选覆盖正式）/失败作废（删候选保正式）——防"模型自评进化"越进化越差
package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZergSkill 任务 Skill（SKILL.md 管理）
type ZergSkill struct {
	// SkillsRoot 技能库根（skills/<类型或key>/SKILL.md）
	SkillsRoot string
	// TaskSkillPath 当前任务的 SKILL.md（任务 git 内）
	TaskSkillPath string
	// SkillKey 任务独属 key（内部任务=def.ID——沉淀后独属 skill 存 skills/<key>/；空=外部任务按类型共享）
	SkillKey string
	// UsingCandidate 本次 Get 是否命中候选（试跑标记——任务结束决定转正/作废）
	UsingCandidate bool
	// usedCandidateContent 本次试跑用的候选内容快照（Get 命中时保存——转正写正式版用
	// ——不能读候选文件: 封闭轮进化可能已覆盖新候选——转正的必须是"本次试跑成功的那份"）
	usedCandidateContent string
}

// NewZergSkill 新建 Skill 管理器
// skillsRoot: 技能库根（如 /tmp/zerg-tasks/skills）
// taskDir: 当前任务目录（任务 git 内）
// skillKey: 任务独属 key（内部任务 def.ID——空=外部按类型）
func NewZergSkill(skillsRoot, taskDir, skillKey string) *ZergSkill {
	return &ZergSkill{
		SkillsRoot:    skillsRoot,
		TaskSkillPath: filepath.Join(taskDir, "SKILL.md"),
		SkillKey:      skillKey,
	}
}

// SkillTemplate 初始模板（无历史 Skill 时——模型自举起点）
const SkillTemplate = `# SKILL: %s

## 任务类型
%s

## 目标
（任务要完成什么）

## 步骤
（执行步骤——按什么顺序/方式）

## 方法
（工具选择/最佳实践）

## 输出要求
（报告/产物格式）

## 常见坑
（前人教训——执行后补充）
`

// Lookup 查 Skill 路径（正式版——有独属用独属——没有再查类型库）
// key: 任务独属 key（内部任务 def.ID——空则跳过）
// taskType: 类型（回退——外部任务/未沉淀时）
// 返回: 找到的 Skill 路径——空=没有（用模板）
func (s *ZergSkill) Lookup(key, taskType string) string {
	if key != "" {
		p := filepath.Join(s.SkillsRoot, sanitizeType(key), "SKILL.md")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := filepath.Join(s.SkillsRoot, sanitizeType(taskType), "SKILL.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// LookupCandidate 查候选 Skill 路径（进化产出待验证——SKILL.candidate.md）
// key/taskType 语义同 Lookup——候选优先独属（该任务试跑）——没有再查类型库候选
func (s *ZergSkill) LookupCandidate(key, taskType string) string {
	if key != "" {
		p := filepath.Join(s.SkillsRoot, sanitizeType(key), "SKILL.candidate.md")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := filepath.Join(s.SkillsRoot, sanitizeType(taskType), "SKILL.candidate.md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// Get 获取当前任务的 Skill 内容
// 优先级: 任务 git 内 SKILL.md（本次任务已建/进化）→ 候选（SKILL.candidate.md——进化待验证——试跑）
//        → 独属 skill（skills/<key>/——该任务沉淀的进化版）→ 类型库 Skill（同类共享进化版）→ 模板
// v2.5.6 候选机制（2026-08-28）: 命中候选——置 UsingCandidate=true——任务结束成功=转正/失败=作废
func (s *ZergSkill) Get(taskType string) (string, error) {
	// 1. 任务内已有（本次执行建过）
	if data, err := os.ReadFile(s.TaskSkillPath); err == nil && len(data) > 0 {
		return string(data), nil
	}
	// 2. 候选（进化待验证——本次任务试跑——Mr2109 2026-08-28）
	if p := s.LookupCandidate(s.SkillKey, taskType); p != "" {
		data, err := os.ReadFile(p)
		if err == nil {
			s.UsingCandidate = true
			s.usedCandidateContent = string(data) // 快照——转正用这份（封闭轮进化会覆盖候选文件）
			return string(data), nil
		}
	}
	// 3. 独属 skill（内部任务——该任务自己的进化版——Mr2109 2026-08-27）
	if s.SkillKey != "" {
		if p := s.Lookup(s.SkillKey, ""); p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				return string(data), nil
			}
		}
	}
	// 4. 类型库（进化版）
	if p := s.Lookup("", taskType); p != "" {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
	}
	// 5. 模板（自举起点）
	return fmt.Sprintf(SkillTemplate, taskType, taskType), nil
}

// SaveCandidate 存候选 skill（进化产出——待下次任务试跑验证）
// content: 模型产出的优化版完整 skill 内容——写 skills/<key>/SKILL.candidate.md（内部任务）
//          外部任务（key 空）→ skills/<类型>/SKILL.candidate.md（同类共享候选）
// 不覆盖正式版 SKILL.md——试跑成功才转正
func (s *ZergSkill) SaveCandidate(key, taskType, content string) error {
	libKey := key
	if libKey == "" {
		libKey = taskType
	}
	dir := filepath.Join(s.SkillsRoot, sanitizeType(libKey))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建候选 skill 目录失败: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "SKILL.candidate.md"), []byte(content), 0o644)
}

// PromoteCandidate 候选转正（试跑成功——候选覆盖正式版）
// key: 独属 key（内部任务）——空=类型库（外部任务）
// 转正内容 = 本次试跑用的候选快照（usedCandidateContent）——不是候选文件——
// 封闭轮进化可能已写入新候选（下次任务试跑）——不能误转正
// 返回: 是否真的转正了（有候选快照才转）
func (s *ZergSkill) PromoteCandidate(key, taskType string) (bool, error) {
	if !s.UsingCandidate || s.usedCandidateContent == "" {
		return false, nil // 本次没用候选——无需转正
	}
	libKey := key
	if libKey == "" {
		libKey = taskType
	}
	dir := filepath.Join(s.SkillsRoot, sanitizeType(libKey))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("候选转正创建目录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(s.usedCandidateContent), 0o644); err != nil {
		return false, fmt.Errorf("候选转正写正式版失败: %w", err)
	}
	// 只删快照（本次试跑完）——不删候选文件（可能是封闭轮新进化——留给下次任务）
	s.UsingCandidate = false
	s.usedCandidateContent = ""
	return true, nil
}

// DiscardCandidate 候选作废（试跑失败/任务失败——删候选——保留正式版）
// 注意: 只作废快照与候选文件（本次试跑失败的候选——不再留给下次）
// 但封闭轮可能已写新候选（本次失败一般没到封闭轮——文件还是旧候选——删掉正确）
func (s *ZergSkill) DiscardCandidate(key, taskType string) error {
	libKey := key
	if libKey == "" {
		libKey = taskType
	}
	candPath := filepath.Join(s.SkillsRoot, sanitizeType(libKey), "SKILL.candidate.md")
	_ = os.Remove(candPath) // 不存在也无妨
	s.UsingCandidate = false
	s.usedCandidateContent = ""
	return nil
}

// Create 创建任务 Skill（模型建立初版——类型判断后）
// content: 模型写的 Skill 内容（步骤/方法——指导执行）
func (s *ZergSkill) Create(taskType, content string) error {
	if err := os.MkdirAll(filepath.Dir(s.TaskSkillPath), 0o755); err != nil {
		return fmt.Errorf("创建任务目录失败: %w", err)
	}
	return os.WriteFile(s.TaskSkillPath, []byte(content), 0o644)
}

// SaveOwned 沉淀独属 skill（v2.5.6 Mr2109——优化后的 skill 成为该任务自己的 skill）
// content: 模型产出的优化版完整 skill 内容——覆盖写 skills/<SkillKey>/SKILL.md
// 仅内部任务（SkillKey 非空）使用——外部任务走 Evolve 类型库共享
func (s *ZergSkill) SaveOwned(content string) error {
	if s.SkillKey == "" {
		return nil
	}
	dir := filepath.Join(s.SkillsRoot, sanitizeType(s.SkillKey))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建独属 skill 目录失败: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}

// Evolve 进化（执行后——经验回流——SKILL.md 更新）
// experience: 本次执行的经验（总结/教训/好方法）
// 更新任务 SKILL.md（append 经验段）——并同步到独属目录（内部任务）或类型库（外部任务）
func (s *ZergSkill) Evolve(taskType, experience string) error {
	// 1. 更新任务内 SKILL.md（append 经验）
	entry := fmt.Sprintf("\n## 经验（%s）\n%s\n", time.Now().Format("2006-01-02"), experience)
	f, err := os.OpenFile(s.TaskSkillPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString(entry)
		_ = f.Close()
	}
	// 2. 同步库（内部任务→独属目录 skills/<key>/；外部→类型库 skills/<类型>/）
	libKey := s.SkillKey
	if libKey == "" {
		libKey = taskType
	}
	libDir := filepath.Join(s.SkillsRoot, sanitizeType(libKey))
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return fmt.Errorf("创建 skill 库失败: %w", err)
	}
	libPath := filepath.Join(libDir, "SKILL.md")
	// 已有进化版——append 新经验（保留历史）
	lf, err := os.OpenFile(libPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = lf.WriteString(entry)
		_ = lf.Close()
	}
	return nil
}

// sanitizeType 类型名清洗（安全目录名——去特殊字符）
func sanitizeType(t string) string {
	repl := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "", "（", "", "）", "")
	return repl.Replace(strings.TrimSpace(t))
}
