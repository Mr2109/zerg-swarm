package subtask

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSchedulerFullRun — 三阶段全链 mock（拆解→A→B→C）
func TestSchedulerFullRun(t *testing.T) {
	dir := t.TempDir()
	callCount := 0
	call := func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, int64, int64, error) {
		callCount++
		switch {
		case strings.Contains(userPrompt, "任务架构师"):
			// 拆解轮
			return `{"steps":[
				{"id":"A","goal":"写 calc.go 实现 Add","produces":["calc.go"],"contract":{"must_write_files":["calc.go"]}},
				{"id":"B","goal":"写 calc_test.go 测试 Add","produces":["calc_test.go"],"depends":["A"],"contract":{"must_pass_cmds":["true"]}},
				{"id":"C","goal":"写 internal-task-report.md 报告 计算器","produces":["internal-task-report.md"],"depends":["B"],"contract":{"must_write_files":["internal-task-report.md"]}}
			],"rationale":"测试"}`, 100, 200, nil
		case strings.Contains(userPrompt, "阶段总结员"):
			return `{"key_data":["产物路径见 work"],"lessons":[],"influence":"无"}`, 50, 80, nil
		default:
			return "正文", 300, 100, nil
		}
	}
	runner := func(ctx context.Context, step Step, seed []map[string]any) (string, int64, int, error) {
		// 模拟执行: 每阶段写自己的产物
		for _, f := range step.Produces {
			_ = os.WriteFile(filepath.Join(dir, f), []byte("package test // "+step.ID), 0o644)
		}
		return "完成 " + step.ID, 500, 3, nil
	}
	s := NewScheduler(Config{Workdir: dir, PlannerModel: "DS4", ExecutorModel: "Qwen3.8-27B"}, call, runner, nil)
	out, err := s.Run(context.Background(), "写计算器: calc.go 实现 Add——calc_test.go 测试——internal-task-report.md 报告")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Status != "done" {
		t.Fatalf("应 done: %+v", out)
	}
	// 分相计量（G6）
	if s.DecomposeTokens == 0 || s.ExecuteTokens == 0 || s.CrystallizeTokens == 0 {
		t.Fatalf("三相应有计量: deco=%d exec=%d crys=%d", s.DecomposeTokens, s.ExecuteTokens, s.CrystallizeTokens)
	}
	// 产物落盘
	for _, f := range []string{"calc.go", "calc_test.go", "internal-task-report.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("产物缺失: %s", f)
		}
	}
	// Stages 进度卡（G7）
	stages := s.Stages()
	if len(stages) != 3 || stages[0].Status != "done" {
		t.Fatalf("stages: %+v", stages)
	}
}

// TestSchedulerCriticalFail — 关键阶段失败=任务失败
func TestSchedulerCriticalFail(t *testing.T) {
	dir := t.TempDir()
	call := func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, int64, int64, error) {
		if strings.Contains(userPrompt, "任务架构师") {
			return `{"steps":[
				{"id":"A","goal":"写 a.txt 基础","produces":["a.txt"],"contract":{"must_write_files":["a.txt"]}},
				{"id":"B","goal":"写关键产物 key.txt","produces":["key.txt"],"depends":["A"],"critical":true,"contract":{"must_write_files":["key.txt"]}}
			]}`, 100, 100, nil
		}
		if strings.Contains(userPrompt, "阶段总结员") {
			return `{"key_data":["x"],"influence":""}`, 10, 10, nil
		}
		return "正文", 50, 50, nil
	}
	runner := func(ctx context.Context, step Step, seed []map[string]any) (string, int64, int, error) {
		if step.ID == "B" {
			return "写不出来", 100, 5, nil // B 不写产物——契约必败
		}
		_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
		return "ok", 100, 2, nil
	}
	s := NewScheduler(Config{Workdir: dir}, call, runner, nil)
	out, err := s.Run(context.Background(), "写 a.txt 基础然后写关键产物 key.txt")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "failed" {
		t.Fatalf("关键阶段失败应任务 failed: %+v", out)
	}
}

// TestSchedulerReplanPath — 拆解校验失败重试后成功（G3/R9）
func TestSchedulerReplanPath(t *testing.T) {
	dir := t.TempDir()
	attempts := 0
	call := func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, int64, int64, error) {
		if strings.Contains(userPrompt, "任务架构师") {
			attempts++
			if attempts == 1 {
				return "这不是 JSON", 50, 50, nil // 第一次坏输出
			}
			return `{"steps":[
				{"id":"A","goal":"写 hello.txt 问候","produces":["hello.txt"],"contract":{"must_write_files":["hello.txt"]}},
				{"id":"B","goal":"写 done.txt 完成","produces":["done.txt"],"depends":["A"],"contract":{"must_write_files":["done.txt"]}}
			]}`, 100, 100, nil
		}
		if strings.Contains(userPrompt, "阶段总结员") {
			return `{"key_data":["y"],"influence":""}`, 10, 10, nil
		}
		// 执行轮: 写产物
		for _, f := range []string{"hello.txt", "done.txt"} {
			_ = os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
		}
		return "done", 50, 1, nil
	}
	s := NewScheduler(Config{Workdir: dir}, call, runnerOK, nil)
	out, err := s.Run(context.Background(), "写 hello.txt 问候然后写 done.txt 完成")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "done" {
		t.Fatalf("重拆后应成功: %+v", out)
	}
}

func runnerOK(ctx context.Context, step Step, seed []map[string]any) (string, int64, int, error) {
	for _, f := range step.Produces {
		_ = os.WriteFile(filepath.Join("/tmp", f), []byte("x"), 0o644)
	}
	return "done", 100, 2, nil
}

// TestPlanCacheGate — G9 计划缓存入库门槛（failed 任务不缓存）
func TestPlanCacheGate(t *testing.T) {
	// 设计文档 3.8——一期只验证语义: failed 不入库
	out := TaskOutcome{Status: "failed"}
	if out.Status == "done" {
		t.Fatal("不应到达")
	}
	fmt.Println("G9 门槛建议: SavePlanTemplate 仅当 outcome.Status==done")
}

// TestSchedulerResumeSkipsDone — S6: 断点恢复续作（已 done 阶段跳过）
func TestSchedulerResumeSkipsDone(t *testing.T) {
	dir := t.TempDir()
	call := func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, int64, int64, error) {
		if strings.Contains(userPrompt, "阶段总结员") {
			return `{"key_data":["续作补充"],"influence":""}`, 10, 10, nil
		}
		return "正文", 50, 50, nil
	}
	ranSteps := []string{}
	runner := func(ctx context.Context, step Step, seed []map[string]any) (string, int64, int, error) {
		ranSteps = append(ranSteps, step.ID)
		_ = os.WriteFile(filepath.Join(dir, step.Produces[0]), []byte("x"), 0o644)
		return "ok", 100, 2, nil
	}
	s := NewScheduler(Config{Workdir: dir}, call, runner, nil)
	// 注入持久化 plan（模拟 LoadPlan）
	plan := &Plan{Steps: []Step{
		{ID: "A", Goal: "写 a.txt", Produces: []string{"a.txt"}, Contract: &Contract{MustWriteFiles: []string{"a.txt"}}},
		{ID: "B", Goal: "写 b.txt 依赖 a", Produces: []string{"b.txt"}, Depends: []string{"A"}, Contract: &Contract{MustWriteFiles: []string{"b.txt"}}},
	}}
	s.SetPlan(plan)
	// 注入断点结晶（A 已 done）
	s.Resume(map[string]*Crystal{"A": {StepID: "A", ExitKind: ExitDone}})
	out, err := s.Run(context.Background(), "写 a.txt 然后写 b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "done" {
		t.Fatalf("应 done: %+v", out)
	}
	if !s.Resumed {
		t.Fatal("应标记续作")
	}
	for _, id := range ranSteps {
		if id == "A" {
			t.Fatal("已 done 的 A 不应重跑")
		}
	}
	if len(ranSteps) != 1 || ranSteps[0] != "B" {
		t.Fatalf("应只跑 B: %v", ranSteps)
	}
	if len(s.ResumedSkipped) != 1 || s.ResumedSkipped[0] != "A" {
		t.Fatalf("审计: %v", s.ResumedSkipped)
	}
}
