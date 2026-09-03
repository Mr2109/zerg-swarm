package control

// gate.go — 虫族 v2.4 M3 集中控制层：工具调用拦截三态决策（2026-08-13）
// 设计文档：docs/设计-M3集中控制层.md
// 借鉴：Agent Control（集中控制层）+ MCP Guard（allow/block/require_approval 三态）

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Action 三态决策
type Action string

const (
	ActionAllow            Action = "allow"              // 放行
	ActionBlock            Action = "block"              // 拦截
	ActionRequireApproval  Action = "require_approval"   // 需审批
)

// ToolRule 工具拦截规则
type ToolRule struct {
	Name            string   `yaml:"name"`                       // 工具名（精确匹配）
	Action          Action   `yaml:"action"`                     // 三态
	Scope           []string `yaml:"scope,omitempty"`            // 适用 agent（空=全部）
	ArgsDeny        []string `yaml:"args_deny,omitempty"`        // 参数子串命中即拦截（如 rm -rf）
}

// RulesConfig 规则文件结构
type RulesConfig struct {
	Version int        `yaml:"version"`
	Rules   RulesBody  `yaml:"rules"`
}

// RulesBody 规则主体
type RulesBody struct {
	Tools   []ToolRule `yaml:"tools"`
	Default Action     `yaml:"default"` // 默认策略（allow=放行 / block=保守拒绝）
}

// GateDecision 决策结果
type GateDecision struct {
	Action  Action `json:"action"`
	Rule    string `json:"rule,omitempty"`    // 命中的规则名
	Message string `json:"message,omitempty"` // 说明
}

// Gate 控制层门卫
type Gate struct {
	rules RulesBody
}

// NewGateFromFile 从 YAML 文件加载规则
func NewGateFromFile(path string) (*Gate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取规则文件失败: %w", err)
	}
	return NewGateFromYAML(data)
}

// NewGateFromYAML 从 YAML 字节加载规则
func NewGateFromYAML(data []byte) (*Gate, error) {
	var cfg RulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析规则文件失败: %w", err)
	}
	if cfg.Rules.Default == "" {
		cfg.Rules.Default = ActionAllow // 默认放行（现有行为不变）
	}
	return &Gate{rules: cfg.Rules}, nil
}

// Check 三态决策：工具名 + 参数 + agent
// 返回决策——allow 放行 / block 拦截 / require_approval 需审批
func (g *Gate) Check(toolName, args, agent string) GateDecision {
	// 1. 精确匹配规则
	for _, r := range g.rules.Tools {
		if r.Name != toolName {
			continue
		}
		// scope 过滤：规则指定了 agent 且当前 agent 不在内 → 跳过
		if len(r.Scope) > 0 && !contains(r.Scope, agent) {
			continue
		}
		// 参数子串拦截（args_deny 命中即 block——优先级高于 allow）
		for _, deny := range r.ArgsDeny {
			if strings.Contains(args, deny) {
				return GateDecision{
					Action:  ActionBlock,
					Rule:    r.Name,
					Message: fmt.Sprintf("工具 %s 参数含禁止模式 %q（控制层拦截）", toolName, deny),
				}
			}
		}
		// 按规则 action 返回
		return GateDecision{
			Action:  r.Action,
			Rule:    r.Name,
			Message: fmt.Sprintf("工具 %s 命中规则（action=%s）", toolName, r.Action),
		}
	}
	// 2. 未匹配 → 默认策略
	return GateDecision{
		Action:  g.rules.Default,
		Message: fmt.Sprintf("工具 %s 未匹配规则——默认 %s", toolName, g.rules.Default),
	}
}

// contains 判断 slice 是否含元素
func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
