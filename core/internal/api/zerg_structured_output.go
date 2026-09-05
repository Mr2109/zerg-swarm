// Package api - v2.5.6 程序定量驱动 Agent: 结构化输出校验器
// 方案轮/选定轮——JSON schema 校验——不合重试（不给模型自由发挥）
// 2026-08-25——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 3.1/11.2
package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlanOutput 方案轮输出（模型必须遵守——固定格式）
type PlanOutput struct {
	Plans []PlanItem `json:"plans"`
}

// PlanItem 一个方案
type PlanItem struct {
	ID       string   `json:"id"`       // A/B/C
	Title    string   `json:"title"`    // 方案名
	Steps    []string `json:"steps"`    // 拆解步骤（小任务列表）
	Approach string   `json:"approach"` // 方法/思路
}

// SelectOutput 选定轮输出（固定格式）
type SelectOutput struct {
	Selected     string   `json:"selected"`      // 选哪个（plans 里的 id）
	Reason       string   `json:"reason"`        // 理由
	RefinedSteps []string `json:"refined_steps"` // 细化步骤（小任务列表）
}

// SummaryOutput 每轮总结（固定格式——记忆接力）
type SummaryOutput struct {
	Summary string `json:"summary"` // 一句话总结（下轮记忆输入）
}

// ParsePlan 解析方案轮输出（JSON 校验）
// 规则: plans 1-3 个——每项 id/title/steps/approach 非空——steps 非空
func ParsePlan(content string) (*PlanOutput, error) {
	var out PlanOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("JSON 解析失败（需要 {plans:[{id,title,steps,approach}]}）: %v", err)
	}
	if len(out.Plans) == 0 {
		return nil, fmt.Errorf("plans 为空（需要 1-3 个方案）")
	}
	if len(out.Plans) > 3 {
		return nil, fmt.Errorf("plans 超过 3 个（最多 3 个——聚焦大方向）")
	}
	seen := map[string]bool{}
	for i, p := range out.Plans {
		// v2.5.6 统一: 所有 id 规范化为 A/B/C（不管模型给什么——plan-1/plan-a/任意）
		// 选定轮 selected 与方案 id 必须一致——程序统一最简单
		out.Plans[i].ID = string(rune('A' + i))
		p = out.Plans[i]
		seen[p.ID] = true
		if p.Title == "" {
			return nil, fmt.Errorf("方案 %s 缺 title", p.ID)
		}
		// v2.5.6 放宽: steps/approach 缺时程序兜底（方案是方向——细化在选定轮）
		if len(p.Steps) == 0 {
			out.Plans[i].Steps = []string{p.Title} // 兜底: title 作为唯一步骤
		}
		if p.Approach == "" {
			out.Plans[i].Approach = "执行中细化" // 兜底
		}
	}
	return &out, nil
}

// ParseSelect 解析选定轮输出
// 规则: selected 在 plans 内（调用方校验）——reason 非空——refined_steps 非空
func ParseSelect(content string) (*SelectOutput, error) {
	var out SelectOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("JSON 解析失败（需要 {selected,reason,refined_steps}）: %w", err)
	}
	if out.Selected == "" {
		return nil, fmt.Errorf("缺 selected（选哪个方案）")
	}
	if out.Reason == "" {
		return nil, fmt.Errorf("缺 reason（选择理由）")
	}
	if len(out.RefinedSteps) == 0 {
		return nil, fmt.Errorf("缺 refined_steps（细化步骤）")
	}
	return &out, nil
}

// ParseSummary 解析每轮总结
// 规则: summary 非空（一句话）
func ParseSummary(content string) (*SummaryOutput, error) {
	var out SummaryOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("JSON 解析失败（需要 {summary}）: %w", err)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return nil, fmt.Errorf("summary 为空（一句话总结）")
	}
	return &out, nil
}

// PlanSchemaPrompt 方案轮格式提示（拼进模型请求——引导固定格式）
func PlanSchemaPrompt() string {
	return `请以 JSON 格式输出（不要多余文字——只要 JSON）:
{"plans":[{"id":"A","title":"方案名","steps":["步骤1","步骤2"],"approach":"方法思路"},{"id":"B",...},{"id":"C",...}]}
规则: 1-3 个方案——每个方案 id(A/B/C)/title/steps(拆解步骤)/approach(方法) 必填`
}

// SelectSchemaPrompt 选定轮格式提示
func SelectSchemaPrompt() string {
	return `请以 JSON 格式输出（不要多余文字——只要 JSON）:
{"selected":"A","reason":"选择理由","refined_steps":["小任务1","小任务2"]}
规则: selected 是上面方案的 id——refined_steps 是选定方案的细化（执行轮的小任务列表）`
}

// SummarySchemaPrompt 总结格式提示
func SummarySchemaPrompt() string {
	return `请以 JSON 格式输出（不要多余文字——只要 JSON）:
{"summary":"本轮做了什么的一句话总结"}`
}
