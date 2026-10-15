package modeladapter

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// Qwen38Flash Qwen3.8-Flash-Next 适配器。
// 125B MoE（6B 激活）/ qwen4 架构（GDN+QSA+PLE） / 262K 训练上下文 / 多模态 / 思考模型。
// 2026-08-29 接入——node-a 首测：UD-Q4_K_XL 111GB，21.7 t/s，reasoning+content 正常。
// 注意: 不能用 -c 0（自动 262K）——node-a 内存 122G 装 111GB 模型后 KV 空间极小——显式 -c 32768。
// 必须用新 llama-server（build-hip-flash——旧版不支持 qwen4exp 架构），由 yaml cmd 指定。
type Qwen38Flash struct{}

func (a *Qwen38Flash) Name() string { return "qwen3.8-flash" } // 前缀匹配 Qwen3.8-Flash-Next

func (a *Qwen38Flash) BuildArgs(entry *registry.ModelEntry, port int) []string {
	return []string{
		"-m", entry.File,
		"-c", "32768", // 内存受限——262K 会 OOM（模型 111GB + KV 空间只剩 ~10G）
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		"--cache-idle-slots",
		"-np", "1",
		"-cb",
		"--reasoning-preserve", // 思考保留（日志提示：template supports preserving reasoning）
		// GAP-49 口径（2026-09-26 对照实验定性）：思考模型必须给 --reasoning-budget，
		// 否则短 max_tokens 的调用会在分析通道里被截断 ⇒ content 空串（实测 256 = finish_reason=stop、正文可读）。
		// 铁律「思考不能关」⇒ 只给思考一个上界，✗ 不关 thinking。
		"--reasoning-budget", "256",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}
}

func (a *Qwen38Flash) ToolCallStyle() string { return "json" }

func (a *Qwen38Flash) NeedsToolReminder() bool { return true }

func (a *Qwen38Flash) ReminderPrompt() string {
	return "Always call tools directly. Never describe what you plan to do — just call the tool immediately."
}

func (a *Qwen38Flash) ThinkingFormat() string { return "deepseek" }

func (a *Qwen38Flash) ContextWindow() int { return 262144 }
