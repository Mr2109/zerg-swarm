package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloseCommit(t *testing.T) {
	dir := t.TempDir()
	z, _ := NewZergGit(dir)
	zf := NewZergTaskFile(dir)
	_ = zf.Init("task-close", "internal", "目标")
	// 方案轮
	_, _ = CommitRoundWithFS(zf, z, PhasePlan, "", map[string]interface{}{"plans": []string{"A"}}, "出方案")

	c := NewZergClose(dir, z, zf)
	// 报告（模型写的）
	report, err := c.SaveReport("# 结论报告\n\n完成方案A\n")
	if err != nil {
		t.Fatalf("SaveReport 失败: %v", err)
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("报告应存在: %v", err)
	}
	// 封闭提交
	if err := c.CloseCommit(report, "任务完成——报告已提交"); err != nil {
		t.Fatalf("CloseCommit 失败: %v", err)
	}
	// 时间线有 close
	log, _ := z.Log()
	if !contains(log, "close") {
		t.Fatalf("时间线应含 close: %s", log)
	}
}

func TestParseReviewJSON(t *testing.T) {
	// 合法
	r, err := ParseReviewJSON(`{"pass":true,"reason":"任务完成","issues":[]}`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !r.Pass {
		t.Fatalf("应 pass")
	}
	// 打回
	r2, err := ParseReviewJSON(`{"pass":false,"reason":"假完成——无产出","issues":["报告空"]}`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if r2.Pass {
		t.Fatalf("应 fail")
	}
	// ```json 包裹
	r3, err := ParseReviewJSON("```json\n{\"pass\":true,\"reason\":\"ok\"}\n```")
	if err != nil {
		t.Fatalf("包裹解析失败: %v", err)
	}
	if !r3.Pass {
		t.Fatalf("应 pass")
	}
	// 非 JSON
	if _, err := ParseReviewJSON(`不是JSON`); err == nil {
		t.Fatalf("非 JSON 应报错")
	}
}

func TestReportPrompt(t *testing.T) {
	dir := t.TempDir()
	z, _ := NewZergGit(dir)
	zf := NewZergTaskFile(dir)
	_ = zf.Init("task-rp", "internal", "目标")
	_, _ = CommitRoundWithFS(zf, z, PhasePlan, "", map[string]interface{}{"plans": []string{"A"}}, "出方案")
	c := NewZergClose(dir, z, zf)
	p := c.ReportPrompt()
	if !contains(p, "出方案") {
		t.Fatalf("报告提示应含轮次总结")
	}
	if !contains(p, filepath.Join(dir, "reports", "final.md")) {
		t.Fatalf("报告提示应含报告路径")
	}
}
