// decide.go — 判定入口：规则集 + 档位 + 地板 ⇒ 一个决定（allow/ask/deny）+ 低基数原因码。
//
// 判定优先级**只在这里定义一次**（policy.go 的 Effect.strictness 给出严格度序，本文件按序取用）：
//
//	① 地板 deny 命中  ⇒ deny  （压过任何档位）
//	② 地板 ask  命中  ⇒ ask   （压过任何档位；无人值守档下转 deny）
//	③ 普通 deny 命中  ⇒ deny
//	④ 普通 ask  命中  ⇒ ask
//	⑤ 普通 allow 命中 ⇒ allow（**本轮存在"判不了"的规则时 ⇒ 不放行，改 ask**）
//	⑥ 无命中 ⇒ 档位默认：default ⇒ ask · bypass ⇒ allow · dontask ⇒ deny
//
// **"判不了"（specifier 认不出 / 参数取不到/不是字符串/嵌套过深）只说一件事：不确认安全 ⇒ ask。**
// 它是单向的 —— 只压"放行"，不压 deny（deny 更严，本来就该保留）。
package policy

import (
	"fmt"
	"strings"
)

// Config — 引擎的文本来源。两套规则集**分别**解析：任一套失败 ⇒ 整套策略拒绝启用。
type Config struct {
	Rules string // 普通规则（会话/项目档；可被档位与地板覆盖）
	Floor string // 地板（组织/系统下发：任何档位都不自动批。写在这里的 `allow` 不授予权限）
}

// Engine — 已解析的策略引擎（构造后不可变；判定是纯函数，无 IO、无锁、无全局状态）。
type Engine struct {
	rules []Rule
	floor []Rule
}

// Load — 解析两套规则文本并构造引擎。
// **fail-closed**：任一文本解析失败 ⇒ 返回 (nil, 错误) —— **绝不放回一个"空规则"的引擎**
// （回退成"无规则"等于把整份策略静默降级成按档位放行，这正是要防的静默失效）。
func Load(cfg Config) (*Engine, error) {
	rules, err := ParseRules(cfg.Rules)
	if err != nil {
		return nil, fmt.Errorf("普通规则集解析失败 ⇒ 策略整体拒绝启用（不回退成「无规则」）：%w", err)
	}
	floor, err := ParseRules(cfg.Floor)
	if err != nil {
		return nil, fmt.Errorf("地板规则集解析失败 ⇒ 策略整体拒绝启用（不回退成「无规则」）：%w", err)
	}
	for i := range floor {
		floor[i].Floor = true
	}
	return &Engine{rules: rules, floor: floor}, nil
}

// NewEngine — 用已解析的规则集直接构造（不经文本）。规则集的合法性由调用方保证（本函数不做校验）。
func NewEngine(rules, floor []Rule) *Engine {
	f := make([]Rule, len(floor))
	copy(f, floor)
	for i := range f {
		f[i].Floor = true
	}
	r := make([]Rule, len(rules))
	copy(r, rules)
	return &Engine{rules: r, floor: f}
}

// Rules / Floor — 只读回显（观测、审计、UI 要用**实际生效**的那份，不许各存一份）。
func (e *Engine) Rules() []Rule {
	if e == nil {
		return nil
	}
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// Floor — 见 Rules。
func (e *Engine) Floor() []Rule {
	if e == nil {
		return nil
	}
	out := make([]Rule, len(e.floor))
	copy(out, e.floor)
	return out
}

// Request — 一次判定的输入。
type Request struct {
	Tool string         // 工具名（大小写不敏感；空 ⇒ 判不了 ⇒ ask）
	Args map[string]any // 参数字典（取不出字符串 ⇒ 判不了 ⇒ ask）
	Mode Mode           // 会话档位（空 = 保守档；非法取值 ⇒ 判不了 ⇒ ask）
}

// Verdict — 判定输出。MatchedRule 为空 = 没有规则命中（走档位默认 / 判不了）。
type Verdict struct {
	Decision    Effect // allow | ask | deny
	Reason      Reason // 低基数原因码
	MatchedRule *Rule  // 命中的规则（nil = 无命中）；指向引擎内部规则，调用方**不得修改**
	Note        string // 人类可读说明（取证/审计；**不要**拿它做程序判据，用 Reason）
}

// Decide — 唯一判定入口。纯函数：同样输入必然同样输出（无时间、无随机、无 IO）。
func (e *Engine) Decide(req Request) Verdict {
	if e == nil {
		// 引擎未启用（配置解析失败，或调用方根本没注入）⇒ 判不了 ⇒ ask。
		// 这里**不 panic**：判定的调用点在生产路径上，panic 会把"配置错"放大成"服务崩"。
		return verdict(EffectAsk, ReasonEngineDisabled, nil, "策略引擎未启用（配置解析失败或未注入）⇒ 判不了 ⇒ ask（fail-closed）")
	}
	mode := req.Mode
	if mode == "" {
		mode = ModeDefault
	}
	if !mode.Valid() {
		return verdict(EffectAsk, ReasonInvalidMode, nil,
			fmt.Sprintf("档位取值非法「%s」⇒ 判不了 ⇒ ask（fail-closed；未知档位绝不按最宽解释）", string(req.Mode)))
	}
	tool := NormalizeToolName(req.Tool)
	if tool == "" {
		return verdict(EffectAsk, ReasonInvalidRequest, nil, "工具名为空 ⇒ 判不了 ⇒ ask（fail-closed）")
	}

	var un unresolve

	// ①② 地板优先：组织/系统下发的约束**压过会话档位**（含"全放"档）。
	fh := e.scan(e.floor, tool, req.Args, &un)
	if r, ok := fh.first(EffectDeny); ok {
		return verdict(EffectDeny, ReasonFloorDeny, r, "地板命中 deny：任何档位都不自动批（压过档位）")
	}
	if r, ok := fh.first(EffectAsk); ok {
		return applyMode(EffectAsk, ReasonFloorAsk, r, mode, "地板命中 ask：任何档位都要问人（档位不得放行）")
	}

	// ③④⑤ 普通规则：deny > ask > allow。
	rh := e.scan(e.rules, tool, req.Args, &un)
	if r, ok := rh.first(EffectDeny); ok {
		return verdict(EffectDeny, ReasonRuleDeny, r, "规则命中 deny（deny > ask > allow）")
	}
	if r, ok := rh.first(EffectAsk); ok {
		return applyMode(EffectAsk, ReasonRuleAsk, r, mode, "规则命中 ask（ask > allow）")
	}
	if r, ok := rh.first(EffectAllow); ok {
		if un.any() {
			return verdict(EffectAsk, un.reason(), nil,
				"命中 allow，但本轮存在判不了的规则（"+un.desc()+"）⇒ 不放行，改 ask（fail-closed）")
		}
		return verdict(EffectAllow, ReasonRuleAllow, r, "规则命中 allow（且无 deny/ask 命中、无判不了的规则）")
	}

	// ⑥ 无命中：先看"判不了"，再看档位默认（**默认必须是 ask，不是 allow**）。
	if un.any() {
		return verdict(EffectAsk, un.reason(), nil,
			"无规则命中，且本轮存在判不了的规则（"+un.desc()+"）⇒ 不放行，改 ask（fail-closed）")
	}
	switch mode {
	case ModeBypass:
		return verdict(EffectAllow, ReasonBypassAllow, nil, "无任何规则命中；档位=全放（bypass）⇒ allow")
	case ModeDontAsk:
		return verdict(EffectDeny, ReasonDontAskDeny, nil, "无任何规则命中；无人值守档（dontask：凡会弹窗一律拒）⇒ deny")
	default:
		return verdict(EffectAsk, ReasonDefaultAsk, nil, "无任何规则命中；默认档 ⇒ ask（默认不是 allow）")
	}
}

// unresolve — 本轮"判不了"的两类事实（单向：只让判定更严）。
type unresolve struct {
	spec bool // 有命中的工具名下存在认不出的匹配式
	args bool // 有命中的工具名下参数取不到/不是字符串/嵌套过深
}

func (u unresolve) any() bool { return u.spec || u.args }

// reason — 低基数原因码（同时出现时取"形态认不出"——它更可能是配置写错，先暴露它）。
func (u unresolve) reason() Reason {
	if u.spec {
		return ReasonSpecifierUnrecognized
	}
	return ReasonArgsUnresolved
}

// desc — 人类可读说明（进 Note，仅取证用）。
func (u unresolve) desc() string {
	switch {
	case u.spec && u.args:
		return "匹配式形态认不出 + 参数取不到/不是字符串/嵌套过深"
	case u.spec:
		return "匹配式形态认不出（本批只认前缀通配 / 路径 / 域名三种）"
	default:
		return "参数取不到 / 不是字符串 / 嵌套过深"
	}
}

// hitsByEffect — 按效果归类的命中（保序：同一效果内取**先出现**的那条做 MatchedRule）。
type hitsByEffect struct {
	m map[Effect][]*Rule
}

func (h hitsByEffect) first(e Effect) (*Rule, bool) {
	if l := h.m[e]; len(l) > 0 {
		return l[0], true
	}
	return nil, false
}

// scan — 在**一个规则集**里找命中：先按工具名精确匹配（已归一化），再按匹配式形态取值比较。
// 工具名不匹配的规则**完全不参与**（含"判不了"的记账 —— 否则别的工具的坏规则会污染本次判定）。
func (e *Engine) scan(rules []Rule, tool string, args map[string]any, un *unresolve) hitsByEffect {
	h := hitsByEffect{m: map[Effect][]*Rule{}}
	for i := range rules {
		r := &rules[i]
		if r.Tool != tool {
			continue
		}
		switch r.Kind {
		case SpecNone:
			// 无匹配式：按工具名匹配，直接命中
		case SpecUnknown:
			un.spec = true
			continue
		default:
			v, ok := resolveArg(args, r.Kind)
			if !ok {
				un.args = true
				continue
			}
			if !matchValue(r.Kind, r.Specifier, v) {
				continue
			}
		}
		h.m[r.Effect] = append(h.m[r.Effect], r)
	}
	return h
}

// applyMode — 档位对 ask 的再加工：无人值守档把 ask 转成 deny（批准超时/无人可问 ⇒ 绝不放行）。
func applyMode(dec Effect, reason Reason, r *Rule, mode Mode, note string) Verdict {
	if mode == ModeDontAsk && dec == EffectAsk {
		return verdict(EffectDeny, ReasonDontAskDeny, r, note+"；无人值守档（dontask）把 ask 转成 deny")
	}
	return verdict(dec, reason, r, note)
}

// verdict — 组装输出（命中规则一并回显，取证要能指到"是哪一行规则判的"）。
func verdict(d Effect, reason Reason, r *Rule, note string) Verdict {
	if r != nil {
		note = strings.TrimSpace(note) + "｜规则：" + r.Raw
	}
	return Verdict{Decision: d, Reason: reason, MatchedRule: r, Note: note}
}
