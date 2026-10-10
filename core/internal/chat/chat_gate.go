// chat_gate.go — v2.5.7 对话安全门（C7——危险命令黑名单——虫族安全理念）
// 只拦绝对不可逆+系统级（endOnly 精确匹配——不误杀 rm /tmp 等安全用法）
// 拦截时给引导建议（教模型换安全方式——非纯拒绝）——与主控 M3 gate 同理念

package chat

import (
	"regexp"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// ChatGate — 对话工具安全门（实现 agent.ToolGater）
type ChatGate struct{}

// Check — 工具调用检查（危险命令黑名单——精确匹配命令尾）
// 返回 agent.Decision{Action: allow/block}——block 带引导建议
func (g *ChatGate) Check(toolName, args string, agentName string) (agent.Decision, error) {
	if toolName != "bash" {
		return agent.Decision{Action: "allow"}, nil
	}
	// 提取命令（bash 工具 args 里 command 字段）
	cmd := extractCommand(args)
	if cmd == "" {
		return agent.Decision{Action: "allow"}, nil
	}
	// 危险命令黑名单（两类：目标类=后边界防误伤 / 动作类=任意位置）
	// 只拦绝对不可逆 + 系统级（rm 根目录/mkfs/shutdown/格式化/未知脚本管道）
	type rule struct {
		pattern string
		suggest string
		endOnly bool // true=目标类（pattern 后须空白/行尾）——false=动作类（任意位置）
	}
	blocked := []rule{
		{"rm -rf /", "删除整个文件系统不可逆——请改为精确路径（如 rm -rf /tmp/xxx）或先确认目标", true},
		{"rm -fr /", "删除整个文件系统不可逆——请改为精确路径", true},
		{"mkfs", "格式化操作不可逆——禁止", false},
		{"shutdown", "关机操作会中断服务——禁止", false},
		{"reboot", "重启操作会中断服务——禁止", false},
		{"poweroff", "关机操作会中断服务——禁止", false},
		{"dd if=/dev/zero", "零填充会摧毁磁盘——禁止", false},
		{"| sh", "禁止管道执行未知脚本——请先查看脚本内容", false},
		{"| bash", "禁止管道执行未知脚本——请先查看脚本内容", false},
	}
	for _, b := range blocked {
		hit := false
		if b.endOnly {
			hit = matchBlocked(cmd, b.pattern)
		} else {
			hit = strings.Contains(cmd, b.pattern)
		}
		if hit {
			return agent.Decision{Action: "block", Message: "危险命令拦截: " + b.pattern + "。" + b.suggest}, nil
		}
	}
	// ★ 缺口 GAP-20260927-67：凭据类路径/键名拦截（与码面侧「点项默认不进」同向——只收紧，不放宽）。
	// 病征（改前）：本门**无凭据规则** ⇒ `cat .env` 类命令照跑；而同仓 grep/glob 侧却默认排点件
	// ⇒ 同仓同一件事两侧口径相反（本缺口的实据）。
	if label := credentialClassHit(cmd); label != "" {
		return agent.Decision{Action: "block", Message: "凭据类路径/键名拦截: 命中「" + label +
			"」。凭据文件与凭据变量不进命令面——请改用凭据链（env / `--token-stdin`），不要 cat/echo/printenv 凭据原件" +
			"（口径为准侧=" + CredentialGatePole + "）"}, nil
	}
	// 事故修复(2026-09-07): rm 参数级家目录防护——endOnly 同款漏洞(~/$HOME/家目录绝对路径漏过)
	// 与 agent/bash_v101.go bashRmTargetGuard 同规则(展开后逐目标判定 / 家目录本身/祖先/内部)
	if err := agent.BashCheckRmHome(cmd); err != nil {
		return agent.Decision{Action: "block", Message: "危险命令拦截: " + err.Error()}, nil
	}
	return agent.Decision{Action: "allow"}, nil
}

// matchBlocked — 命令边界匹配（防 "rm -rf /" 误伤 "rm -rf /tmp"——endOnly 精确判定）
// pattern 后必须是空白/分号/行尾才拦截（说明确实要作用于根目标）
func matchBlocked(cmd, pattern string) bool {
	if cmd == pattern {
		return true
	}
	for _, sep := range []string{" ", "\t", ";", "&&"} {
		if strings.HasPrefix(cmd, pattern+sep) {
			return true
		}
	}
	return false
}

// extractCommand — 提取 bash 工具的 command 参数（args 是 JSON 字符串）
func extractCommand(args string) string {
	// args 形如 {"command":"ls -la"}——简单提取
	if i := strings.Index(args, `"command"`); i >= 0 {
		rest := args[i+len(`"command"`):]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[j+1:]
			var b strings.Builder
			for k := 0; k < len(rest); k++ {
				c := rest[k]
				if c == '\\' && k+1 < len(rest) {
					b.WriteByte(rest[k+1])
					k++
					continue
				}
				if c == '"' {
					break
				}
				b.WriteByte(c)
			}
			return b.String()
		}
	}
	return ""
}

// ── 凭据类路径/键名拦截（缺口 GAP-20260927-67）───────────────────────────────────────────────

// CredentialGatePole — 凭据类路径/键名拦截的「口径为准侧」**机读标记**（缺口 GAP-20260927-67）。
//
// 本仓对「凭据类路径/键名」有两种实现，本常量 ＋ CredentialPoleSides 把「哪一侧为准」落成**机读**形态
// （gate / 自检 / 审计读这两个符号，✗ 不解析人面文案）。
//
// 口径：以 **码面侧**（`core/internal/agent` 的 grep/glob「点项默认不进码面」）**为准**；
// 命令面侧（本件 ChatGate 的 bash 门）此前无凭据规则、本次补齐 —— **只许收紧到不宽于**码面侧，
// ✗ 不许反向放宽码面侧来迁就它（安全面只许收紧、或写清口径）。
//
// 另一侧为何不同（机制差异、方向一致）：码面侧靠 grep/glob 的 `hidden` 开关，缺省**整类点件**不进
// （粗粒度、一刀切）；命令面侧（本件）拦的是**凭据类路径段 ＋ 凭据类变量展开**（细粒度、带边界，
// 避免 `--token-stdin` / `x.env` / `printenv PATH` 这类正常命令被误杀）。机制不同 ⇒ 规则表形态不同，
// 但「拒不进」这一方向必须一致。
const CredentialGatePole = "codepath-default-deny"

// CredentialPoleSide — 口径两侧的一格（机读）。
type CredentialPoleSide struct {
	Side   string // 侧名（机读 id）
	Where  string // 该侧拦截规则的真源落点
	Stance string // 该侧对「凭据类路径/键名」的处置：deny / allow
	Pole   bool   // 是否「为准」侧（全清单里恰一格为 true）
	Why    string // 该侧机制的一句话（供审计人读；判定不吃它）
}

// CredentialPoleSides — 口径两侧的机读清单（缺口 GAP-20260927-67）。
// 消费方（gate / 自检 / 审计）读它判「两侧一致」：Stance 必须同值，且恰一格 Pole==true；
// 两侧 Stance 分家 ⇒ 口径哑化，自检必须真红。
var CredentialPoleSides = []CredentialPoleSide{
	{Side: "codepath", Where: "core/internal/agent/exec.go（executeGrep/executeGlob 的 hidden 缺省）",
		Stance: "deny", Pole: true,
		Why: "点项/凭据件缺省不进码面（hidden=true 才放行）——**为准**"},
	{Side: "cmdface", Where: "core/internal/chat/chat_gate.go（ChatGate 的 credentialCmdRules）",
		Stance: "deny", Pole: false,
		Why: "凭据类路径段＋变量展开，边界匹配防误杀；只许收紧、不得宽于 codepath"},
}

// 路径段左右边界字符类（命令里路径段的两侧须是这些之一——防 `x.env` / `--env` / `environment` 假阳性）。
const (
	credLeftBound  = `[\s"'/=(){}\[\]|;&<>]`
	credRightBound = `[\s"';|&<>]`
)

// credentialCmdRules — 凭据类**路径/键名**的高置信形态（缺口 GAP-20260927-67）。
// 每条 = 一个正则 ＋ 中文标签；命中即拒。
// ✗ 不用「裸子串」匹配 token/secret/password 这类键名——那样会把 `--token-stdin`、`X-Auth-Token`
// 这类正常命令整片误杀（假阳性炸弹＝把闸门废掉）。键名只认**变量展开/直读**这一高置信形态。
var credentialCmdRules = []struct {
	re    *regexp.Regexp
	label string
}{
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)\.env(?:\.[A-Za-z0-9_.-]+)?(?:$|` + credRightBound + `)`), "凭据文件 .env 族"},
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)\.(?:netrc|pgpass)(?:$|` + credRightBound + `)`), "凭据文件 .netrc/.pgpass"},
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)id_(?:rsa|ed25519|ecdsa|dsa)(?:$|` + credRightBound + `)`), "SSH 私钥 id_*"},
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)auth\.json(?:$|` + credRightBound + `)`), "凭据文件 auth.json"},
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)credentials(?:\.json)?(?:$|` + credRightBound + `)`), "凭据文件 credentials*"},
	{regexp.MustCompile(`\.zerg/token`), "凭据文件 .zerg/token"},
	{regexp.MustCompile(`\.aws/credentials`), "凭据文件 .aws/credentials"},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`), "内联私钥块"},
	{regexp.MustCompile(`\$\{?[A-Za-z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASSWD|API_KEY|APIKEY|CREDENTIAL|PRIVATE_KEY)[A-Za-z0-9_]*\}?`), "凭据类变量展开 $*TOKEN/$*SECRET…"},
	{regexp.MustCompile(`(?:^|` + credLeftBound + `)(?:printenv|env)\s+[A-Za-z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASSWD|API_KEY|APIKEY|CREDENTIAL)`), "凭据类变量直读 printenv/env <VAR>"},
}

// credentialClassHit — 凭据类路径/键名命中即返回该条标签；无命中返回 ""。
func credentialClassHit(cmd string) string {
	for _, r := range credentialCmdRules {
		if r.re.MatchString(cmd) {
			return r.label
		}
	}
	return ""
}
