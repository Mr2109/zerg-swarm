package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestZergFullFlow 全流程最小闭环（mock 模型——不调真实网关）
// 任务: 创建文件（模拟代码任务）——方案轮→选定轮→执行轮→封闭轮→复查
// 验证: git 树完整（每轮 commit）——task.jsonl 完整（每轮总结）——报告存在——复查通过
func TestZergFullFlow(t *testing.T) {
	taskDir := t.TempDir()

	// 0. 任务建立: git + task.jsonl + SKILL.md 模板
	git, err := NewZergGit(taskDir)
	if err != nil {
		t.Fatalf("任务建立 git 失败: %v", err)
	}
	zf := NewZergTaskFile(taskDir)
	if err := zf.Init("task-full", "internal", "创建 hello.txt 文件"); err != nil {
		t.Fatalf("任务建立 jsonl 失败: %v", err)
	}
	skill := NewZergSkill(filepath.Join(taskDir, "..", "skills"), taskDir, "")
	skillContent, _ := skill.Get("代码任务")
	_ = skill.Create("代码任务", skillContent)

	// 1. 方案轮（模型 mock——出 2 方案 JSON）
	planRaw := `{"plans":[{"id":"A","title":"直接创建","steps":["写文件","验证"],"approach":"echo"},{"id":"B","title":"用go","steps":["写go程序","运行"],"approach":"go run"}]}`
	planOut, err := ParsePlan(planRaw)
	if err != nil {
		t.Fatalf("方案解析失败: %v", err)
	}
	round1, err := CommitRoundWithFS(zf, git, PhasePlan, "", map[string]interface{}{"plans": planOut.Plans}, "出了2个方案")
	if err != nil || round1 != 1 {
		t.Fatalf("方案轮提交失败: %v (round=%d)", err, round1)
	}

	// 2. 选定轮（模型 mock——选 A）
	selRaw := `{"selected":"A","reason":"直接简单","refined_steps":["写文件","验证"]}`
	selOut, err := ParseSelect(selRaw)
	if err != nil {
		t.Fatalf("选定解析失败: %v", err)
	}
	round2, err := CommitRoundWithFS(zf, git, PhaseSelect, "", map[string]interface{}{"selected": selOut.Selected, "steps": selOut.RefinedSteps}, "选定方案A")
	if err != nil || round2 != 2 {
		t.Fatalf("选定轮提交失败: %v (round=%d)", err, round2)
	}

	// 3. 执行轮（受控循环——模型 mock——写文件——验证文件存在）
	loop := NewControlledLoop(taskDir)
	loop.MaxRounds = 3
	loop.WithVerifyCmd("test", "-f", filepath.Join(taskDir, "hello.txt"))
	stepResult, err := loop.RunStep("写文件", func(step, feedback string) (string, error) {
		_ = os.WriteFile(filepath.Join(taskDir, "hello.txt"), []byte("hello zerg"), 0o644)
		return "文件已写", nil
	})
	if err != nil || !stepResult.Success {
		t.Fatalf("执行轮失败: %v res=%+v", err, stepResult)
	}
	round3, err := CommitRoundWithFS(zf, git, PhaseExecute, "写文件", map[string]interface{}{"file": "hello.txt"}, "完成写文件——验证通过")
	if err != nil || round3 != 3 {
		t.Fatalf("执行轮提交失败: %v (round=%d)", err, round3)
	}

	// 4. 封闭轮（模型 mock——读 git 树写报告）
	zc := NewZergClose(taskDir, git, zf)
	report, err := zc.SaveReport("# 结论报告\n\n创建 hello.txt 成功——验证通过\n")
	if err != nil {
		t.Fatalf("报告保存失败: %v", err)
	}
	if err := zc.CloseCommit(report, "任务完成"); err != nil {
		t.Fatalf("封闭提交失败: %v", err)
	}
	// 封闭轮也写 task.jsonl
	round4, err := CommitRoundWithFS(zf, git, PhaseClose, "", map[string]interface{}{"report": "reports/final.md"}, "封闭——报告提交")
	if err != nil || round4 != 4 {
		t.Fatalf("封闭轮提交失败: %v (round=%d)", err, round4)
	}

	// 5. 复查（mock——跨家族模型——通过）
	zr := NewZergReview(taskDir, git)
	_ = zr // 提示复用
	reviewOut, err := ParseReviewJSON(`{"pass":true,"reason":"产出存在——hello.txt——验证过","issues":[]}`)
	if err != nil {
		t.Fatalf("复查解析失败: %v", err)
	}
	if !reviewOut.Pass {
		t.Fatalf("复查应通过")
	}

	// 6. 完成评定（git 树完整 + 非空检查）
	commitCount := git.CommitCount()
	if commitCount < 6 { // init + r1 + r2 + r3 + close 报告 + close jsonl（+可能 tag）
		t.Fatalf("git 树不完整——commit 数应 >=6: %d", commitCount)
	}
	lines, _ := zf.ReadAll()
	if len(lines) < 5 { // init + r1 + r2 + r3 + r4
		t.Fatalf("task.jsonl 行数不足: %d", len(lines))
	}
	// 每轮总结在
	sum, _ := zf.Summary(10)
	for _, want := range []string{"出了2个方案", "选定方案A", "完成写文件", "封闭"} {
		if !strings.Contains(sum, want) {
			t.Fatalf("task.jsonl 缺总结: %s（缺 %s）", sum, want)
		}
	}
	// 报告存在
	if _, err := os.Stat(filepath.Join(taskDir, "reports", "final.md")); err != nil {
		t.Fatalf("报告应存在: %v", err)
	}
}
