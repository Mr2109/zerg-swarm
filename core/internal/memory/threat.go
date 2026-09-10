// threat.go — 记忆投毒防御:写入前威胁扫描(设计稿 §9.2 出处分级与投毒防御)
//
// 依据:MINJA(仅查询注入 ISR>95%)、AgentPoison、MemoryGraft(少量毒记录占检索 47.9%)、OWASP LLM01/LLM06 + ASI06。
// 记忆条目会注入系统提示并跨会话持久,所以命中即"拒写"(不是警告),并回 FFP 式系统断言——
// 让模型明白"这是系统层的预期拒绝,不是你写错了参数,也不要把原样内容重发"。
//
// 表小而明确(中文+英文),每类一个 id + 中文标签,Error 里点名类别(设计稿 §3.2:不要只写"失败")。
package memory

import (
	"fmt"
	"regexp"
	"strings"
)

// ThreatAssertionPrefix — 系统断言首行。与 FFP(core/internal/ffp)同一纪律:
// 拒绝必须先教"下一步",否则模型会把内容校验拒绝当成自己的错并原样重发(实测:write 自检拒绝被重试 6 次)。
const ThreatAssertionPrefix = "⚠️ 系统断言——预期拒绝"

// threatCategory — 威胁类别(id 稳定、可断言;label 给中文用户/模型看)。
type threatCategory struct {
	ID    string
	Label string
	RE    *regexp.Regexp
}

// 说明:正则表刻意保持"小、明确、低误伤"。不写"宽泛英语祈使句"("you must"在合法笔记里常见),
// 只锚定注入/外泄的明确词汇与形态。base64 长串用长度阈值(200 rune 起)避免误伤普通文本。
var threatCategories = []threatCategory{
	// ① 提示注入:忽略/无视 前文指令(中英)
	{
		ID:    "prompt_injection",
		Label: "提示注入",
		RE: regexp.MustCompile(`(?i)(ignore|disregard|forget)\s+(?:\w+\s+){0,8}(previous|prior|above|all|earlier)\s+(?:\w+\s+){0,8}(instruction|prompt|rule|message)` +
			`|忽略[^\n]{0,24}(之前|先前|前面|上面|以上|历史)[^\n]{0,12}(指令|指示|提示|要求|规则)` +
			`|无视[^\n]{0,24}(指令|规则|提示)`),
	},
	// ② 系统提示泄露:动词 + 系统提示/系统提示词
	{
		ID:    "system_prompt_leak",
		Label: "系统提示泄露",
		RE: regexp.MustCompile(`(?i)(output|reveal|show|print|leak|repeat|disclose|dump|expose)\s+(?:\w+\s+){0,6}(system|initial|hidden)\s+prompt` +
			`|(system|initial)\s+prompt\s+(override|dump|leak)` +
			`|(输出|泄露|导出|展示|打印|复述|告诉我)[^\n]{0,20}(系统提示词|系统提示|系统指令|隐藏提示)`),
	},
	// ③ 数据外泄:发送/上传/回传到 http(s):// 或显式 exfiltrate
	{
		ID:    "exfiltration",
		Label: "数据外泄",
		RE: regexp.MustCompile(`(?i)\bexfiltrat\w*` +
			`|(send|post|upload|transmit|forward)\s+[^\n]{0,64}?\s+(to|at)\s+https?://` +
			`|(发送|上传|传回|外发|回传|投递)[^\n]{0,48}https?://` +
			`|(curl|wget)\s+[^\n]{0,256}\$\{?\w*(key|token|secret|password|credential)`),
	},
	// ④ 长 base64/编码载荷(200 rune 起——正常笔记不会出现)
	{
		ID:    "encoded_payload",
		Label: "编码载荷",
		RE:    regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`),
	},
	// ⑤ 凭证/密钥形态
	{
		ID:    "credential",
		Label: "密钥令牌",
		RE: regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9]{16,}|ghp_[A-Za-z0-9]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16})` +
			`|(api[_-]?key|access[_-]?token|auth[_-]?token|secret|password|passwd|credential)\s*[:=]\s*["']?[A-Za-z0-9+/=_\-]{20,}`),
	},
	// ⑥ 角色劫持:你现在是/扮演/pretend/you are now
	{
		ID:    "role_hijack",
		Label: "角色劫持",
		RE: regexp.MustCompile(`你现在是|你现在扮演|请?扮演[^\n]{0,12}(角色|助手|AI)` +
			`|(?i)you\s+are\s+now\s+(a|an|the)\b` +
			`|(?i)pretend\s+(to\s+be|you\s+are)\b` +
			`|(?i)act\s+as\s+(if|though)\s+\w*\s*(you\s+)?(have\s+no|don'?t\s+have)\s+(restriction|limit|rule)`),
	},
}

// scanThreat — 返回首个命中类别;无命中 → (零值, false)。
// 上界裁剪到 64K:扫描器是"附加防护",不为此付出无界运行成本。
func scanThreat(content string) (threatCategory, bool) {
	if content == "" {
		return threatCategory{}, false
	}
	if len(content) > maxScanBytes {
		content = content[:maxScanBytes]
	}
	for _, c := range threatCategories {
		if c.RE.MatchString(content) {
			return c, true
		}
	}
	return threatCategory{}, false
}

// maxScanBytes — 扫描文本硬上限(对齐 Hermes MAX_SCAN_CHARS = 65_536)。
const maxScanBytes = 65536

// threatError — 组装拒写文本。首行是系统断言,随后点名类别,末尾给可行动改写指引。
func threatError(c threatCategory, opIndex int) string {
	where := ""
	if opIndex > 0 {
		where = fmt.Sprintf("(第 %d 条操作)", opIndex)
	}
	return fmt.Sprintf("%s:记忆写入被拦截%s——命中威胁类别「%s / %s」。\n"+
		"记忆条目会注入系统提示并跨会话持久,禁止写入提示注入、系统提示套取、数据外泄、编码载荷、密钥令牌或角色劫持内容。\n"+
		"这条内容不会被重试:请改写为纯事实陈述(只记录「是什么」,不带任何对模型的指令)后重发。",
		ThreatAssertionPrefix, where, c.ID, c.Label)
}

// ThreatCategories — 暴露类别 id 列表(仅用于文档/测试断言,不参与运行逻辑)。
func ThreatCategories() []string {
	out := make([]string, 0, len(threatCategories))
	for _, c := range threatCategories {
		out = append(out, c.ID)
	}
	return out
}

// scanAll — 返回全部命中类别 id(测试/审计用;写入路径只用首个)。
func scanAll(content string) []string {
	var out []string
	for _, c := range threatCategories {
		if content != "" && c.RE.MatchString(content) {
			out = append(out, c.ID)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// threatLabelOf — 类别 id → 中文标签(测试断言用)。
func threatLabelOf(id string) string {
	for _, c := range threatCategories {
		if c.ID == id {
			return c.Label
		}
	}
	return ""
}

// mustNoThreat — 内部自检:类别表非空且 id 唯一。
func mustNoThreat() {
	seen := map[string]bool{}
	for _, c := range threatCategories {
		if c.ID == "" || c.RE == nil {
			panic("memory: 威胁类别表存在空项")
		}
		if seen[c.ID] {
			panic("memory: 威胁类别 id 重复: " + c.ID)
		}
		seen[c.ID] = true
	}
	if strings.TrimSpace(ThreatAssertionPrefix) == "" {
		panic("memory: 威胁断言前缀为空")
	}
}
