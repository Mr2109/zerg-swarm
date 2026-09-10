// contract.go — 执行接口约定（多语言 D2——2026-09-11 Mr2109拍板「接口约定进 schema + FFP」）
//
// 依据（`docs/02-调研/raw/调研-多语言-提示语言与模型行为-20260911.md`）：
// MLCL / Lost in Execution（ACL 2026）证明工具调用失败的主因不是「模型看不懂语言」，
// 而是**参数值语言不匹配**——模型选对工具、语义也对，却把非英语 token 直接塞进参数；
// 且部分翻译（保留英文参数串）显著优于全翻译。结论：这是**系统与接口层面**的问题。
//
// 因此本期的做法是**把执行接口约定写进 schema**，再由系统在调用前断言：
//
//  1. 枚举参数用 "enum" 钉死——值域即协议（不给模型猜的余地）
//  2. 需 ASCII 的参数标 "x-zerg-format": "ascii"（机器名/ID/枚举串）
//  3. 日期参数标 "x-zerg-format": "iso-date"（YYYY-MM-DD）
//  4. 只认 schema 里声明的约定——没声明的参数一律放行（不猜、不误伤，如中文路径合法）
//
// 命中违约定时返回 FFP 教学文本（`BuildParamContract`），分类单列「参数值语言不匹配」，
// 并给出**最小合法示例**——即"系统教模型怎么说"，而非把提示词整体翻译。
package ffp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Conventions — 参数值约定（注入提示词/系统提示的固定文本；本身是常量，字节稳定，不破坏前缀缓存）
const Conventions = "【参数值约定】一律 ASCII——枚举用英文小写（create/update/list）；日期用 ISO 8601（2026-09-11）；ID/机器名/命令原样照抄，不要翻译、不要额外空格。"

// isoDateRe — ISO 8601 日期（YYYY-MM-DD）
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// 违约定分类（确定性——系统匹配，不靠模型）
const (
	RuleLangMismatch = "参数值语言不匹配" // 非 ASCII 值出现在约定为 ASCII/英文枚举的参数里（MLCL 主因）
	RuleEnumInvalid  = "枚举值非法"    // ASCII 值但不在 enum 值域内
	RuleDateInvalid  = "日期格式非法"   // 非 ISO 8601
)

// Violation — 一条参数值违约定
type Violation struct {
	Tool     string // 工具名
	Param    string // 参数名
	Rule     string // 分类（RuleLangMismatch / RuleEnumInvalid / RuleDateInvalid）
	Value    string // 模型实际发的值（原文）
	Expected string // parser 期望
	Example  string // 最小合法示例（JSON 片段）
}

// nonASCII — 是否含非 ASCII 字符（中/日/韩/全角标点等）
func nonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// minimalExample — 最小合法示例（{"param": "值"}）
func minimalExample(param, value string) string {
	return fmt.Sprintf(`{"%s": "%s"}`, param, value)
}

// CheckContract — 按 schema 声明的约定校验参数值。
//
// 只看两类声明：`enum`（值域）与 `x-zerg-format`（ascii / iso-date / id）。
// 未声明的参数一律放行——避免"系统自己发明的约定"误伤合法输入（如中文路径、中文查询词）。
// 返回顺序按参数名排序（确定性，便于测试与回放）。
func CheckContract(tool string, schema map[string]any, args map[string]any) []Violation {
	if schema == nil || args == nil {
		return nil
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return nil
	}
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)

	var out []Violation
	for _, name := range names {
		spec, _ := props[name].(map[string]any)
		if spec == nil {
			continue
		}
		raw, present := args[name]
		if !present {
			continue
		}
		s, isStr := raw.(string)

		// ① 枚举（值域即协议）
		if ev, ok := spec["enum"].([]any); ok && len(ev) > 0 {
			allowed := make([]string, 0, len(ev))
			for _, e := range ev {
				allowed = append(allowed, fmt.Sprint(e))
			}
			if !isStr || !containsStr(allowed, s) {
				rule := RuleEnumInvalid
				if isStr && nonASCII(s) {
					rule = RuleLangMismatch // 中文值塞进英文枚举——MLCL 的典型现场
				}
				out = append(out, Violation{
					Tool: tool, Param: name, Rule: rule, Value: fmt.Sprint(raw),
					Expected: "枚举值（英文）: " + strings.Join(allowed, " | "),
					Example:  minimalExample(name, allowed[0]),
				})
				continue
			}
		}

		// ② 格式标记（只在 schema 显式声明时生效）
		format, _ := spec["x-zerg-format"].(string)
		if !isStr || format == "" {
			continue
		}
		switch format {
		case "ascii":
			if nonASCII(s) {
				out = append(out, Violation{
					Tool: tool, Param: name, Rule: RuleLangMismatch, Value: s,
					Expected: "该参数须为 ASCII（英文/数字/符号）——勿翻译、勿用全角",
					Example:  minimalExample(name, "example"),
				})
			}
		case "iso-date":
			if !isoDateRe.MatchString(s) {
				out = append(out, Violation{
					Tool: tool, Param: name, Rule: RuleDateInvalid, Value: s,
					Expected: "日期须为 ISO 8601: YYYY-MM-DD（如 2026-09-11）",
					Example:  minimalExample(name, "2026-09-11"),
				})
			}
		case "id":
			if nonASCII(s) || strings.ContainsAny(s, " \t\n") {
				out = append(out, Violation{
					Tool: tool, Param: name, Rule: RuleLangMismatch, Value: s,
					Expected: "ID 原样照抄（ASCII、无空格）——勿翻译、勿加空格",
					Example:  minimalExample(name, "abc-123"),
				})
			}
		}
	}
	return out
}

// BuildParamContract — 违约定的 FFP 教学文本（沿用统一结构：分类+原文回显+期望+最小合法示例+引导）
func BuildParamContract(v Violation) string {
	kind := v.Rule + ": " + v.Tool + "." + v.Param
	guide := "把该参数值改成上面「最小合法示例」的形式重发本工具调用——枚举/日期/ID 一律原样照抄，不要翻译成中文、不要加空格。"
	return Build(kind, EchoSafe(v.Value), v.Expected, v.Example, guide)
}
