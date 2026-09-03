package api

import (
	"testing"
)

func TestParsePlan(t *testing.T) {
	// 合法（3 方案）
	ok := `{"plans":[{"id":"A","title":"方案1","steps":["s1","s2"],"approach":"方法1"},{"id":"B","title":"方案2","steps":["s1"],"approach":"方法2"},{"id":"C","title":"方案3","steps":["s1","s2","s3"],"approach":"方法3"}]}`
	p, err := ParsePlan(ok)
	if err != nil {
		t.Fatalf("合法解析失败: %v", err)
	}
	if len(p.Plans) != 3 || p.Plans[0].ID != "A" {
		t.Fatalf("解析不对: %+v", p)
	}

	// 空 plans
	if _, err := ParsePlan(`{"plans":[]}`); err == nil {
		t.Fatalf("空 plans 应报错")
	}
	// 缺 steps（v2.5.6 放宽——方案是方向——选定轮补细化——不再报错）
	if _, err := ParsePlan(`{"plans":[{"id":"A","title":"t","approach":"m"}]}`); err != nil {
		t.Fatalf("缺 steps 应允许（v2.5.6 放宽——选定轮细化）: %v", err)
	}
	// 非 JSON
	if _, err := ParsePlan(`不是JSON`); err == nil {
		t.Fatalf("非 JSON 应报错")
	}
	// 4 个方案（超限）
	four := `{"plans":[{"id":"A","title":"1","steps":["s"],"approach":"m"},{"id":"B","title":"2","steps":["s"],"approach":"m"},{"id":"C","title":"3","steps":["s"],"approach":"m"},{"id":"D","title":"4","steps":["s"],"approach":"m"}]}`
	if _, err := ParsePlan(four); err == nil {
		t.Fatalf("4 方案应报错（最多 3）")
	}
}

func TestParseSelect(t *testing.T) {
	ok := `{"selected":"A","reason":"理由","refined_steps":["小任务1","小任务2"]}`
	s, err := ParseSelect(ok)
	if err != nil {
		t.Fatalf("合法解析失败: %v", err)
	}
	if s.Selected != "A" || len(s.RefinedSteps) != 2 {
		t.Fatalf("解析不对: %+v", s)
	}
	if _, err := ParseSelect(`{"selected":"","reason":"r","refined_steps":["s"]}`); err == nil {
		t.Fatalf("缺 selected 应报错")
	}
	if _, err := ParseSelect(`{"selected":"A","reason":"r"}`); err == nil {
		t.Fatalf("缺 refined_steps 应报错")
	}
}

func TestParseSummary(t *testing.T) {
	ok := `{"summary":"完成s1——测试过"}`
	s, err := ParseSummary(ok)
	if err != nil {
		t.Fatalf("合法解析失败: %v", err)
	}
	if s.Summary == "" {
		t.Fatalf("summary 空")
	}
	if _, err := ParseSummary(`{"summary":""}`); err == nil {
		t.Fatalf("空 summary 应报错")
	}
}
