// policy.go — 「规则层」本体：allow/ask/deny 策略的解析与判定（T5.1，2026-09-17）。
//
// 要治的缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F2）：
// **只有档位 ⇒ 只能全开或全关**。成熟系统的形态是三层 =
//
//	**地板**（组织/系统下发、任何档位都不自动批的清单）
//	+ **档位**（自主阶梯：default / bypass / dontask）
//	+ **规则**（allow/ask/deny，按参数/路径/域名匹配，**deny > ask > allow**）
//
// 且两条安全语义写死：**地板优先于会话档位**；**配置解析失败按最严处理**（fail-closed），不是放行。
//
// 本包的位置：**叶子包** —— 零内部依赖（只 import 标准库，不 import 任何业务包），本批**不接任何执行路径**
// （toolobs / chat / gateway / agent 一行不动）；先把引擎本体与用例钉死，接线另批。
//
// ── 判定优先级（写死在这里，并由用例逐条钉住；`Decide` 是唯一入口）──
//
//	① 地板 deny 命中  ⇒ deny （floor_deny）—— **压过任何档位**（含"全放"档）
//	② 地板 ask  命中  ⇒ ask  （floor_ask） —— 同上（无人值守档下转 deny）
//	③ 普通 deny 命中  ⇒ deny （rule_deny）
//	④ 普通 ask  命中  ⇒ ask  （rule_ask）
//	⑤ 普通 allow 命中 ⇒ allow（rule_allow）—— **但本轮若存在"判不了"的规则 ⇒ 不放行，改 ask**
//	⑥ 全部无命中 ⇒ 按档位默认：default ⇒ **ask**（不是 allow）· bypass ⇒ allow · dontask ⇒ deny
//
// 另有两条派生规则：**档位 dontask 下凡 ask ⇒ deny**（「凡会弹窗一律拒」，CI 用）；
// **地板里的 allow 不授予任何权限**（地板只加约束、不放松 —— `allow` 写在地板里等于没写）。
//
// ── fail-closed 三条（每一条都有用例钉住）──
//
//	· 规则文本解析失败 ⇒ **整套拒绝启用**（返回错误 + nil 引擎），**不回退成"无规则"**
//	· specifier 形式认不出 / 参数取不到或不是字符串 / 嵌套过深 ⇒ **判不了 ⇒ ask**（绝不默认 allow）
//	· nil 引擎（未启用）/ 工具名为空 / 档位取值非法 ⇒ **ask**（不 panic、不放行）
//
// ── 文本语法（一行一条；本批的最小语法，后续批次只加不改）──
//
//	<效果> <工具名>[(匹配式)]
//	效果   = allow | ask | deny（大小写不敏感）
//	工具名 = 字母/数字/_/-/.（**不支持通配**；归一化 = 去首尾空白 + 小写，与仓内工具名匹配纪律一致）
//	匹配式 = 括号内文本（不含括号、不为空）；三种形态见 match.go
//	空行与 `#` 开头的整行注释忽略；**注释必须独占一行**（行尾注释会当语法错误拒掉）
//
// 例：`deny bash(rm -rf *)` · `ask write(**/.env)` · `allow webfetch(domain:example.com)` · `allow read(./src/**)`
//
// 作用域（user/project/session）与授权粒度（once/session/project + TTL）**不在本批**：
// 它们是任务表的 T5.2/T5.3，本批只做「规则集 → 判定」这段本体，先把优先级与 fail-closed 钉死。
package policy

import (
	"fmt"
	"strings"
)

// ── 效果 ────────────────────────────────────────────────────────────────────

// Effect — 规则效果（三类，无第四类）。
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectAsk   Effect = "ask"
	EffectDeny  Effect = "deny"
)

// Valid — 是否三类之一。
func (e Effect) Valid() bool {
	switch e {
	case EffectAllow, EffectAsk, EffectDeny:
		return true
	}
	return false
}

// strictness — 严格度序（数值越大越严）：deny > ask > allow。
// 判定优先级由它派生（decide.go 只按这个序取用），**不另写第二套"谁压谁"的分支**。
func (e Effect) strictness() int {
	switch e {
	case EffectDeny:
		return 2
	case EffectAsk:
		return 1
	case EffectAllow:
		return 0
	}
	return -1 // 非法效果 = 比 allow 还宽 ⇒ 绝不用它做任何判定（由 ReasonInvalidRequest 兜住）
}

// ── 原因码（低基数：可聚合、可告警；自由文本一律进 Verdict.Note）────────────────

// Reason — 判定的**低基数**原因码（枚举只有一个来源，避免各处自造字符串）。
type Reason string

const (
	ReasonFloorDeny             Reason = "floor_deny"             // ① 地板命中 deny
	ReasonFloorAsk              Reason = "floor_ask"              // ② 地板命中 ask
	ReasonRuleDeny              Reason = "rule_deny"              // ③ 普通规则命中 deny
	ReasonRuleAsk               Reason = "rule_ask"               // ④ 普通规则命中 ask
	ReasonRuleAllow             Reason = "rule_allow"             // ⑤ 普通规则命中 allow
	ReasonDefaultAsk            Reason = "default_ask"            // ⑥ 无命中，默认档 ⇒ ask（默认不是 allow）
	ReasonBypassAllow           Reason = "bypass_allow"           // ⑥ 无命中，全放档 ⇒ allow
	ReasonDontAskDeny           Reason = "dontask_deny"           // 无人值守档：ask ⇒ deny
	ReasonSpecifierUnrecognized Reason = "specifier_unrecognized" // 匹配式形态认不出 ⇒ 判不了
	ReasonArgsUnresolved        Reason = "args_unresolved"        // 参数取不到/不是字符串/嵌套过深 ⇒ 判不了
	ReasonInvalidRequest        Reason = "invalid_request"        // 工具名为空 ⇒ 判不了
	ReasonInvalidMode           Reason = "invalid_mode"           // 档位取值非法 ⇒ 判不了
	ReasonEngineDisabled        Reason = "engine_disabled"        // 引擎未启用（解析失败/未注入）⇒ 判不了
	// ── T5.8（长暂停）新增一条 ──
	// ReasonApprovalTimeout — 工具批准的挂单**超时** ⇒ 按拒绝结算（pending.go 的 timeoutOutcome）。
	// 它是 deny 这一侧的原因码：**超时绝不被读作同意**（没有"同意"这件事发生过），
	// 故绝不与 ReasonRuleAllow / ReasonBypassAllow 混用。
	ReasonApprovalTimeout Reason = "approval_timeout"
)

// ── 档位（自主阶梯）──────────────────────────────────────────────────────────

// Mode — 会话档位。语义与设计稿 F4 对齐（无人值守两档 + 保守默认档）。
type Mode string

const (
	// ModeDefault — 保守档：无规则命中 ⇒ ask（**默认不是 allow**）。
	ModeDefault Mode = "default"
	// ModeBypass — 全放档（yolo）：无规则命中 ⇒ allow；**地板仍然压倒它**。
	ModeBypass Mode = "bypass"
	// ModeDontAsk — 无人值守档：凡 ask ⇒ deny（「凡会弹窗一律拒」，CI 用）。
	ModeDontAsk Mode = "dontask"
)

// Valid — 是否三类档位之一（取值非法一律按最严处理，见 Decide）。
func (m Mode) Valid() bool {
	switch m {
	case ModeDefault, ModeBypass, ModeDontAsk:
		return true
	}
	return false
}

// ParseMode — 解析档位文本；空串 = 保守档；其余认不出一律**返回错误**（不猜档位）。
func ParseMode(s string) (Mode, error) {
	m := Mode(strings.ToLower(strings.TrimSpace(s)))
	if m == "" {
		return ModeDefault, nil
	}
	if !m.Valid() {
		return "", fmt.Errorf("未知档位「%s」（只认 default|bypass|dontask）", strings.TrimSpace(s))
	}
	return m, nil
}

// ── 规则 ────────────────────────────────────────────────────────────────────

// Rule — 一条已解析的规则。字段全部导出：调用方（观测/审计/UI）要能如实回显"是哪一条规则判的"。
type Rule struct {
	// ID — 稳定标识（T5.2）：**内置清单条目**填带命名空间的 id（`any.*` / `never.*`，见 builtin_rules.go），
	// 文本规则为空串（文本没有 id，取证靠 Raw + 行号）。Verdict.MatchedRule.ID 非空即"命中内置条目"，
	// 审计据此回查"为何永不自动批"（BuiltinRuleByID）。
	ID        string
	Effect    Effect   // allow | ask | deny
	Tool      string   // 归一化后的工具名（小写、去首尾空白）
	Specifier string   // 匹配式原文（空 = 无匹配式，按工具名匹配）
	Kind      SpecKind // 匹配式形态（解析期判定；SpecUnknown = 认不出，判定时判不了 ⇒ ask）
	Floor     bool     // 是否属于地板规则集（组织/系统下发）
	Index     int      // 在本规则集内 1 起的序号（取证用）
	Raw       string   // 原始行（取证用；不含行号）
}

// NormalizeToolName — 工具名归一化：**去首尾空白 + 小写**。
// 与 core/internal/agent 的 resolveToolName 纪律同源（行为宽容、契约严格：注册表真名一律小写），
// 规则里的工具名与调用侧工具名**都过这一个函数** ⇒ 工具名匹配大小写不敏感，且只有一处实现。
func NormalizeToolName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ParseRules — 从文本解析规则集。**任何一行不合法 ⇒ 返回错误（带行号）** ——
// 不跳过、不部分启用（调用方 Load 据此把整套策略判为"拒绝启用"，见 decide.go 的 fail-closed 条）。
func ParseRules(text string) ([]Rule, error) {
	var out []Rule
	for i, rawLine := range strings.Split(text, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eff, body, err := splitEffect(line, lineNo)
		if err != nil {
			return nil, err
		}
		tool, spec, err := splitBody(body, lineNo)
		if err != nil {
			return nil, err
		}
		out = append(out, Rule{
			Effect:    eff,
			Tool:      NormalizeToolName(tool),
			Specifier: spec,
			Kind:      classifySpec(spec),
			Index:     len(out) + 1,
			Raw:       line,
		})
	}
	return out, nil
}

// splitEffect — 切出「效果 + 剩余正文」。效果大小写不敏感；未知效果即错误。
func splitEffect(line string, lineNo int) (Effect, string, error) {
	cut := strings.IndexAny(line, " \t")
	if cut < 0 {
		return "", "", fmt.Errorf("第 %d 行：只写了「%s」，缺少工具名（正确形态：`deny bash(rm -rf *)`）", lineNo, line)
	}
	head := strings.TrimSpace(line[:cut])
	body := strings.TrimSpace(line[cut+1:])
	eff := Effect(strings.ToLower(head))
	if !eff.Valid() {
		return "", "", fmt.Errorf("第 %d 行：未知效果「%s」（只认 allow|ask|deny）", lineNo, head)
	}
	if body == "" {
		return "", "", fmt.Errorf("第 %d 行：效果「%s」后面缺少工具名", lineNo, head)
	}
	return eff, body, nil
}

// splitBody — 切出「工具名 + 匹配式」。括号必须配对；匹配式不许嵌套括号、不许为空。
func splitBody(body string, lineNo int) (tool, spec string, err error) {
	if strings.HasSuffix(body, ")") {
		open := strings.Index(body, "(")
		if open < 0 {
			return "", "", fmt.Errorf("第 %d 行：括号不配对（有 `)` 却没有 `(`）：%s", lineNo, body)
		}
		tool = strings.TrimSpace(body[:open])
		spec = strings.TrimSpace(body[open+1 : len(body)-1])
		if strings.ContainsAny(spec, "()") {
			return "", "", fmt.Errorf("第 %d 行：匹配式里不允许再出现括号（不支持嵌套）：%s", lineNo, body)
		}
		if spec == "" {
			return "", "", fmt.Errorf("第 %d 行：匹配式为空（要匹配整个工具请直接写 `<效果> %s`）", lineNo, tool)
		}
	} else {
		if strings.ContainsAny(body, "()") {
			return "", "", fmt.Errorf("第 %d 行：括号不配对（`(` 未闭合，或结尾有多余字符）：%s", lineNo, body)
		}
		tool = strings.TrimSpace(body)
	}
	if tool == "" {
		return "", "", fmt.Errorf("第 %d 行：工具名为空（正确形态：`allow write(src/**)`）", lineNo)
	}
	if strings.ContainsAny(tool, " \t") {
		return "", "", fmt.Errorf("第 %d 行：工具名里有空白「%s」（要匹配参数请写 `%s(%s)`）", lineNo, tool, tool, spec)
	}
	if strings.Contains(tool, "*") {
		return "", "", fmt.Errorf("第 %d 行：工具名不支持通配「%s」（通配写在匹配式里）", lineNo, tool)
	}
	return tool, spec, nil
}
