// chat_gate.go — v2.5.7 对话安全门（C7——危险命令黑名单——虫族安全理念）
// 只拦绝对不可逆+系统级（endOnly 精确匹配——不误杀 rm /tmp 等安全用法）
// 拦截时给引导建议（教模型换安全方式——非纯拒绝）——与主控 M3 gate 同理念

package chat

import (
	"strings"

	"zerg/core/internal/agent"
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
	for _, sep := range []string{" ", "	", ";", "&&"} {
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
