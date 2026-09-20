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
	ActionAllow           Action = "allow"            // 放行
	ActionBlock           Action = "block"            // 拦截
	ActionRequireApproval Action = "require_approval" // 需审批
)

// ToolRule 工具拦截规则
type ToolRule struct {
	Name     string   `yaml:"name"`                // 工具名（精确匹配）
	Action   Action   `yaml:"action"`              // 三态
	Scope    []string `yaml:"scope,omitempty"`     // 适用 agent（空=全部）
	ArgsDeny []string `yaml:"args_deny,omitempty"` // 参数子串命中即拦截（如 rm -rf）
}

// RulesConfig 规则文件结构
type RulesConfig struct {
	Version int       `yaml:"version"`
	Rules   RulesBody `yaml:"rules"`
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
		// ★ 2026-09-20 批 C · T-31（§二十一 已红第 9 条）：**未配置默认 ⇒ 拦**（fail-closed）。
		// 老写法是 `= ActionAllow // 默认放行（现有行为不变）` —— 那就是 fail-open 的一种：
		// 规则文件缺一行（或拼错 `default:`）就静默变成「全放行」，而现象上**什么都不会报**。
		// 现在的口径：缺配置 = 最保守的那一档（block），要放行就显式写 `default: allow`。
		cfg.Rules.Default = ActionBlock
	}
	if cfg.Rules.Default != ActionAllow && cfg.Rules.Default != ActionBlock && cfg.Rules.Default != ActionRequireApproval {
		// 拼错的三态（如 `defualt` / `Allow`）**不许**静默按某种语义跑 —— 一律落到最保守档。
		cfg.Rules.Default = ActionBlock
	}
	for i := range cfg.Rules.Tools {
		switch cfg.Rules.Tools[i].Action {
		case ActionAllow, ActionBlock, ActionRequireApproval:
		default:
			// 规则里的 action 拼错 ⇒ 这条规则按 block 算（fail-closed；不许把拼错的规则当放行）。
			cfg.Rules.Tools[i].Action = ActionBlock
		}
	}
	return &Gate{rules: cfg.Rules}, nil
}

// matchRuleName —— 规则名匹配（精确 + **尾部单个 `*` 的通配**）。
//
// 为什么要通配：MCP 工具名是动态的（`mcp_<服务>_<工具>`），「显式列全」在这类名字上做不到；
// 不给一条兜底，默认 block 就会把 MCP 全掐掉（那是把闸废掉的另一种形态）。
// 口径（防暗权）：**只认尾部一个 `*`** —— `mcp_*` 算通配；`*db*` / `mcp_*_x` 一律当**字面量**
// （多一个通配位就多一处猜不到的放行面，本仓宁可不给）。
func matchRuleName(pattern, toolName string) bool {
	if pattern == toolName {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.Count(pattern, "*") == 1 {
		return strings.HasPrefix(toolName, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

// Check 三态决策：工具名 + 参数 + agent
// 返回决策——allow 放行 / block 拦截 / require_approval 需审批
func (g *Gate) Check(toolName, args, agent string) GateDecision {
	// 1. 精确匹配规则（**优先**：精确条目永远压过通配条目）
	if d, ok := g.checkAgainst(toolName, args, agent, true); ok {
		return d
	}
	// 1′. 尾部通配规则（`mcp_*` 一类；精确没命中才轮到它）
	if d, ok := g.checkAgainst(toolName, args, agent, false); ok {
		return d
	}
	// 2. 未匹配 → 默认策略
	return GateDecision{
		Action:  g.rules.Default,
		Message: fmt.Sprintf("工具 %s 未匹配规则——默认 %s", toolName, g.rules.Default),
	}
}

// checkAgainst 走一遍规则表：`exact=true` 只看逐字相同的名字，`exact=false` 只看通配名。
// 分两遍是为了让「精确条目压过通配条目」这件事有确定答案（不依赖 rules.yaml 里的书写次序）。
func (g *Gate) checkAgainst(toolName, args, agent string, exact bool) (GateDecision, bool) {
	for _, r := range g.rules.Tools {
		if exact {
			if r.Name != toolName {
				continue
			}
		} else {
			if r.Name == toolName || !matchRuleName(r.Name, toolName) {
				continue
			}
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
				}, true
			}
		}
		// 按规则 action 返回
		return GateDecision{
			Action:  r.Action,
			Rule:    r.Name,
			Message: fmt.Sprintf("工具 %s 命中规则（action=%s）", toolName, r.Action),
		}, true
	}
	return GateDecision{}, false
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
