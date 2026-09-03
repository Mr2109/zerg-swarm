package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillTemplateFallback(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "")
	// 无类型库——无任务 Skill——返回模板
	content, err := s.Get("代码修bug")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !strings.Contains(content, "代码修bug") {
		t.Fatalf("模板应含类型名: %s", content)
	}
}

func TestSkillCreateAndGet(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "")
	// 模型建立 Skill（类型判断后）
	content := "# SKILL: 代码修bug\n\n## 步骤\n1. 复现\n2. 修复\n3. 测试\n"
	if err := s.Create("代码修bug", content); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	// 任务内 Get——返回刚建的
	got, _ := s.Get("代码修bug")
	if !strings.Contains(got, "复现") {
		t.Fatalf("任务 Skill 应返回创建内容: %s", got)
	}
}

func TestSkillEvolve(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "")
	_ = s.Create("代码修bug", "# SKILL: 代码修bug\n\n## 步骤\n1. 复现\n")
	// 进化（经验回流）
	if err := s.Evolve("代码修bug", "教训: 先跑测试确认复现——再改代码"); err != nil {
		t.Fatalf("Evolve 失败: %v", err)
	}
	// 任务内 SKILL.md 含经验
	data, _ := os.ReadFile(filepath.Join(taskDir, "SKILL.md"))
	if !strings.Contains(string(data), "教训") {
		t.Fatalf("任务 Skill 应含经验")
	}
	// 类型库建了（同类共享）
	libPath := filepath.Join(root, sanitizeType("代码修bug"), "SKILL.md")
	if _, err := os.Stat(libPath); err != nil {
		t.Fatalf("类型库 SKILL.md 应存在: %v", err)
	}
	// Lookup 能找到进化版
	if p := s.Lookup("", "代码修bug"); p == "" {
		t.Fatalf("Lookup 应找到")
	}
}

func TestSanitizeType(t *testing.T) {
	if got := sanitizeType("代码/修 bug"); got != "代码-修-bug" {
		t.Fatalf("清洗不对: %s", got)
	}
}

func TestSkillCandidateGetAndPromote(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "task-abc")
	// 正式版已存在（上次沉淀）
	formal := "# SKILL: 测试\n\n## 步骤\n1. 旧步骤\n"
	if err := s.SaveOwned(formal); err != nil {
		t.Fatalf("SaveOwned 失败: %v", err)
	}
	// 进化产出存候选（不覆盖正式版）
	cand := "# SKILL: 测试\n\n## 步骤\n1. 新步骤（进化）\n"
	if err := s.SaveCandidate("task-abc", "测试", cand); err != nil {
		t.Fatalf("SaveCandidate 失败: %v", err)
	}
	// Get 应命中候选（试跑）——UsingCandidate=true
	got, _ := s.Get("测试")
	if !strings.Contains(got, "新步骤") {
		t.Fatalf("Get 应返回候选: %s", got)
	}
	if !s.UsingCandidate {
		t.Fatalf("UsingCandidate 应为 true")
	}
	// 正式版没被覆盖
	formalData, _ := os.ReadFile(filepath.Join(root, sanitizeType("task-abc"), "SKILL.md"))
	if !strings.Contains(string(formalData), "旧步骤") {
		t.Fatalf("正式版不应被候选覆盖")
	}
	// 候选文件还在（等待试跑结果）
	candPath := filepath.Join(root, sanitizeType("task-abc"), "SKILL.candidate.md")
	if _, err := os.Stat(candPath); err != nil {
		t.Fatalf("候选文件应存在: %v", err)
	}
	// 任务成功——转正（候选覆盖正式版）
	promoted, err := s.PromoteCandidate("task-abc", "测试")
	if err != nil {
		t.Fatalf("PromoteCandidate 失败: %v", err)
	}
	if !promoted {
		t.Fatalf("有候选快照应转正")
	}
	formalData2, _ := os.ReadFile(filepath.Join(root, sanitizeType("task-abc"), "SKILL.md"))
	if !strings.Contains(string(formalData2), "新步骤") {
		t.Fatalf("转正后正式版应为候选内容: %s", string(formalData2))
	}
	if s.UsingCandidate {
		t.Fatalf("转正后 UsingCandidate 应复位")
	}
}

func TestSkillCandidateDiscard(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "task-abc")
	// 正式版 + 候选
	if err := s.SaveOwned("# SKILL: 测试\n\n正式\n"); err != nil {
		t.Fatalf("SaveOwned 失败: %v", err)
	}
	if err := s.SaveCandidate("task-abc", "测试", "# SKILL: 测试\n\n候选\n"); err != nil {
		t.Fatalf("SaveCandidate 失败: %v", err)
	}
	// Get 命中候选
	got, _ := s.Get("测试")
	if !strings.Contains(got, "候选") {
		t.Fatalf("应命中候选: %s", got)
	}
	// 任务失败——作废（删候选——保留正式版）
	if err := s.DiscardCandidate("task-abc", "测试"); err != nil {
		t.Fatalf("DiscardCandidate 失败: %v", err)
	}
	// 候选文件没了
	candPath := filepath.Join(root, sanitizeType("task-abc"), "SKILL.candidate.md")
	if _, err := os.Stat(candPath); err == nil {
		t.Fatalf("候选文件应被删除")
	}
	// 正式版还在——下次 Get 回退正式版
	s2 := NewZergSkill(root, taskDir, "task-abc")
	got2, _ := s2.Get("测试")
	if !strings.Contains(got2, "正式") || s2.UsingCandidate {
		t.Fatalf("作废后应回退正式版且不标记候选: %s", got2)
	}
}

func TestSkillCandidateFormalFallback(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "task-abc")
	// 只有正式版——无候选
	if err := s.SaveOwned("# SKILL: 测试\n\n正式\n"); err != nil {
		t.Fatalf("SaveOwned 失败: %v", err)
	}
	got, _ := s.Get("测试")
	if s.UsingCandidate {
		t.Fatalf("无候选不应标记 UsingCandidate")
	}
	if !strings.Contains(got, "正式") {
		t.Fatalf("应返回正式版: %s", got)
	}
}

func TestSkillCandidatePromoteTwiceKeepsNewCandidate(t *testing.T) {
	root := t.TempDir()
	taskDir := t.TempDir()
	s := NewZergSkill(root, taskDir, "task-abc")
	// 第一次任务: 正式版 A——进化产出候选 B
	if err := s.SaveOwned("A"); err != nil {
		t.Fatalf("SaveOwned 失败: %v", err)
	}
	if err := s.SaveCandidate("task-abc", "测试", "B"); err != nil {
		t.Fatalf("SaveCandidate 失败: %v", err)
	}
	// 第二次任务: Get 命中候选 B（试跑）——任务成功转正
	got, _ := s.Get("测试")
	if got != "B" || !s.UsingCandidate {
		t.Fatalf("应命中候选 B: got=%q cand=%v", got, s.UsingCandidate)
	}
	promoted, _ := s.PromoteCandidate("task-abc", "测试")
	if !promoted {
		t.Fatalf("应转正")
	}
	// 转正后——封闭轮又进化出新候选 C（覆盖候选文件——但本次转正用的是快照 B）
	if err := s.SaveCandidate("task-abc", "测试", "C"); err != nil {
		t.Fatalf("SaveCandidate 失败: %v", err)
	}
	// 正式版应为 B（本次试跑的那份）——不是 C
	formalData, _ := os.ReadFile(filepath.Join(root, sanitizeType("task-abc"), "SKILL.md"))
	if string(formalData) != "B" {
		t.Fatalf("正式版应为本次试跑的 B——实际: %s", string(formalData))
	}
	// 候选文件是 C（留给下次任务试跑）
	s3 := NewZergSkill(root, taskDir, "task-abc")
	got3, _ := s3.Get("测试")
	if got3 != "C" || !s3.UsingCandidate {
		t.Fatalf("下次应命中候选 C: got=%q cand=%v", got3, s3.UsingCandidate)
	}
}
