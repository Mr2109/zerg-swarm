package subtask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLinearOK(t *testing.T) {
	p := &Plan{Steps: []Step{
		{ID: "A", Goal: "写计算器 calc.go 实现 Add 函数", Produces: []string{"calc.go"}, Contract: &Contract{MustWriteFiles: []string{"calc.go"}}},
		{ID: "B", Goal: "写 calc_test.go 测试 Add 并跑 go test", Depends: []string{"A"}, Contract: &Contract{MustPassCmds: []string{"go test ./..."}}},
		{ID: "C", Goal: "写 internal-task-report.md 报告计算器实现", Depends: []string{"B"}, Contract: &Contract{MustWriteFiles: []string{"internal-task-report.md"}}},
	}}
	errs := p.Validate("写一个 Go 版计算器工具并验证。在工作区创建 calc.go——实现 Add(a,b int) int 函数——并写 calc_test.go 跑 go test 确认通过——最后写报告 internal-task-report.md")
	if len(errs) != 0 {
		t.Fatalf("合法计划应零错误: %v", errs)
	}
	if ids := p.SortedStepIDs(); len(ids) != 3 || ids[0] != "A" || ids[2] != "C" {
		t.Fatalf("拓扑序: %v", ids)
	}
}

func TestValidateRejects(t *testing.T) {
	// 单步骤（没拆）
	p1 := &Plan{Steps: []Step{{ID: "A", Goal: "全部做完", Contract: &Contract{MustWriteFiles: []string{"x"}}}}}
	if len(p1.Validate("做一个大任务")) == 0 {
		t.Fatal("单步骤应报错")
	}
	// 依赖不存在
	p2 := &Plan{Steps: []Step{
		{ID: "A", Goal: "写代码 calc.go", Contract: &Contract{MustWriteFiles: []string{"calc.go"}}},
		{ID: "B", Goal: "测试 calc", Depends: []string{"Z"}, Contract: &Contract{MustPassCmds: []string{"true"}}},
	}}
	errs := p2.Validate("写代码并测试 calc")
	found := false
	for _, e := range errs {
		if strings.Contains(e, "依赖不存在") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报依赖不存在: %v", errs)
	}
	// 环
	p3 := &Plan{Steps: []Step{
		{ID: "A", Goal: "写 a", Depends: []string{"B"}, Contract: &Contract{MustWriteFiles: []string{"a"}}},
		{ID: "B", Goal: "写 b", Depends: []string{"A"}, Contract: &Contract{MustWriteFiles: []string{"b"}}},
	}}
	errs = p3.Validate("写 a 和 b 两个文件")
	found = false
	for _, e := range errs {
		if strings.Contains(e, "成环") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报环: %v", errs)
	}
	// 无契约无产物
	p4 := &Plan{Steps: []Step{
		{ID: "A", Goal: "写代码文件", Contract: &Contract{}},
		{ID: "B", Goal: "写代码文件测试", Contract: &Contract{}},
	}}
	errs = p4.Validate("写代码文件")
	found = false
	for _, e := range errs {
		if strings.Contains(e, "不可机器验收") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报不可机器验收: %v", errs)
	}
}

func TestValidateRangeAnchor(t *testing.T) {
	// G1 范围锚定：goal 偏离原任务（幻觉步骤）应报错
	p := &Plan{Steps: []Step{
		{ID: "A", Goal: "写计算器 calc.go 实现 Add", Contract: &Contract{MustWriteFiles: []string{"calc.go"}}},
		{ID: "B", Goal: "部署到 kubernetes 集群并配置 ingress", Depends: []string{"A"}, Contract: &Contract{MustPassCmds: []string{"true"}}},
		{ID: "C", Goal: "写报告 计算器", Depends: []string{"B"}, Contract: &Contract{MustWriteFiles: []string{"report.md"}}},
	}}
	errs := p.Validate("写一个 Go 计算器 calc.go 并写报告")
	found := false
	for _, e := range errs {
		if strings.Contains(e, "疑似拆解幻觉") && strings.Contains(e, "B") {
			found = true
		}
	}
	if !found {
		t.Fatalf("部署步骤应被范围锚定拦下: %v", errs)
	}
}

func TestDangerousCmd(t *testing.T) {
	p := &Plan{Steps: []Step{
		{ID: "A", Goal: "清理目录", Contract: &Contract{MustPassCmds: []string{"rm -rf /tmp/ok"}}},
		{ID: "B", Goal: "格式化", Depends: []string{"A"}, Contract: &Contract{MustPassCmds: []string{"mkfs.ext4 /dev/sda1"}}},
	}}
	errs := p.Validate("清理并格式化目录 mkfs")
	// rm -rf /tmp/ok 应放行（endOnly 理念——精确路径合法）；mkfs 应拦
	mkfsHit := false
	for _, e := range errs {
		if strings.Contains(e, "格式化") {
			mkfsHit = true
		}
	}
	if !mkfsHit {
		t.Fatalf("mkfs 应被拦: %v", errs)
	}
}

func TestCrystalValidateAndFallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package main"), 0o644)
	step := Step{ID: "A", Goal: "写 calc.go", Produces: []string{"calc.go"}}

	// 正常结晶
	c := &Crystal{StepID: "A", Goal: step.Goal, ExitKind: ExitDone, KeyData: []string{"calc.go 12 字节"}, Artifacts: []string{"calc.go"}}
	if errs := c.Validate(dir); len(errs) != 0 {
		t.Fatalf("合法结晶: %v", errs)
	}
	// 编造产物
	c2 := &Crystal{StepID: "A", Goal: step.Goal, ExitKind: ExitDone, KeyData: []string{"x"}, Artifacts: []string{"ghost.go"}}
	if errs := c2.Validate(dir); len(errs) == 0 {
		t.Fatal("编造产物应被核验逮住")
	}
	// 空关键数据
	c3 := &Crystal{StepID: "A", Goal: step.Goal, ExitKind: ExitDone, Artifacts: []string{"calc.go"}}
	if errs := c3.Validate(dir); len(errs) == 0 {
		t.Fatal("空关键数据应报错")
	}
	// 兜底结晶（程序数据）
	fb := Fallback(step, ExitPartial, []string{"契约文件缺失: ghost.go"}, dir, "Qwen3.8-27B", 1234, 30)
	if fb.ExitKind != ExitPartial || len(fb.Artifacts) == 0 || fb.Influence == "" {
		t.Fatalf("兜底结晶: %+v", fb)
	}
	if errs := fb.Validate(dir); len(errs) != 0 {
		t.Fatalf("兜底结晶应自洽: %v", errs)
	}
}

func TestParsePlanJSON(t *testing.T) {
	// 带 markdown 包裹
	out := "```json\n{\"steps\":[{\"id\":\"A\",\"goal\":\"写文件 a.txt\",\"contract\":{\"must_write_files\":[\"a.txt\"]}}],\"rationale\":\"单步\"}\n```"
	p, err := ParsePlanJSON(out)
	if err != nil || len(p.Steps) != 1 || p.Steps[0].ID != "A" {
		t.Fatalf("解析: %v %+v", err, p)
	}
}
