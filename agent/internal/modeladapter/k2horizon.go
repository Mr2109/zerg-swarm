package modeladapter

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// K2Horizon IFM K2-Horizon 家族适配器（k2-horizon 架构）。
// 2026-09-08 接入——X3(gfx1151 APU)实测 MoVA-36B-A4B Q4_K_M：37.9 t/s，reasoning+content 正常。
// 架构新（MoVA 混合价值注意力 / Xllm），主线 llama.cpp 不支持（报 unknown model architecture:
// 'k2-horizon'）——必须用 IFM fork（/home/g01/llama-k2/build-k2，model/K2Horizon 分支），
// 由 agent_models.yaml 的 cmd 指定二进制（同 qwen3.8-flash 挂 build-hip-flash 的先例）。
// 思考模型：reasoning 走 reasoning_content（--reasoning-preserve 保留），ThinkingFormat: deepseek。
// 注意：不能 -c 0（原生 512K 上下文）——X3 内存 122G 装 Q4 22.4G 后 KV 空间有限——显式 -c 32768。
// GGUF 内嵌 chat template 需要 --jinja 启用（同 ornith/qwen4 系）。
type K2Horizon struct{}

func (a *K2Horizon) Name() string { return "k2-horizon" } // 前缀匹配 K2-Horizon-*（MoVA-36B 等）

// RequiresNonMainlineEngine 实现 modeladapter.NonMainlineEngine（P1）：
// 主线 llama.cpp **不支持** k2-horizon 架构（报 unknown model architecture）⇒ 这枚卵**必须**
// 由非主线引擎实现承载（IFM fork 的 llama-server，经 run-k2.sh 清 LD_LIBRARY_PATH）。
// 卵声明里没有 cmd: 时 **拒孵**——绝不静默落回 detectLlamaServerPath() 的主线 llama-server
// （那会起不来或误链，而且不报错）。依据：设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1。
func (a *K2Horizon) RequiresNonMainlineEngine() bool { return true }

func (a *K2Horizon) BuildArgs(entry *registry.ModelEntry, port int) []string {
	return []string{
		"-m", entry.File,
		"-c", "32768", // 内存受限——原生 512K 会 OOM（Q4 22.4G + KV 空间有限）
		"-ngl", "999",
		"-ctk", "q8_0",
		"-ctv", "q8_0",
		"-fa", "on",
		"--cache-prompt",
		"--cache-idle-slots",
		"-np", "1", // v2.5.5 单槽铁律（设计决策——执行层面单槽——GPU全负荷）
		"-cb",
		"--jinja",              // GGUF 内嵌 chat template（K2 模板由 fork 写入）
		"--reasoning-preserve", // 思考保留（reasoning_content 抽取）
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}
}

func (a *K2Horizon) ToolCallStyle() string { return "json" }

func (a *K2Horizon) NeedsToolReminder() bool { return true }

func (a *K2Horizon) ReminderPrompt() string {
	return "Always call tools directly. Never describe what you plan to do — just call the tool immediately."
}

func (a *K2Horizon) ThinkingFormat() string { return "deepseek" }

func (a *K2Horizon) ContextWindow() int { return 32768 }
