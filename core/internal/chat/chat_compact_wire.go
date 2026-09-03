// chat_compact_wire.go — 丙批 §4.2 接线（2026-09-10）：压缩单一入口的两个可注入依赖
//
// 设计：`MaybeCompact(ctx, sessionID, model, msgs, lingua, sum, force)`
//   - lingua（LLMLingua-2 删除式、快）由 main.go 启动时注入（与网关同一 ONNX 实例）；
//   - sum（LLM 结构化摘要，6 段模板）复用既有 CompactRequest——这里包成 SummarizeFn。
// 两者都可空：lingua 空则直接走 LLM 摘要；两者都不可用 → 本次压缩判失败（触发冷却/熔断，绝不阻塞对话）。

package chat

import (
	"context"
	"sync"
)

var (
	linguaMu sync.RWMutex
	linguaFn CompactFn
)

// SetLinguaCompressor — 注入 LLMLingua-2 压缩器（main.go 启动时调用；nil=未加载）
func SetLinguaCompressor(fn CompactFn) {
	linguaMu.Lock()
	defer linguaMu.Unlock()
	linguaFn = fn
}

// LinguaFn — 取注入的 LLMLingua-2 压缩器（可能为 nil）
func LinguaFn() CompactFn {
	linguaMu.RLock()
	defer linguaMu.RUnlock()
	return linguaFn
}

// SummarizeWithInfer — 用对话推理器构造 LLM 结构化摘要函数（api 层接线用）
// 复用 CompactRequest（既有 6 段模板 + 旧摘要迭代更新），不另写模板。
func SummarizeWithInfer(infer *ChatInfer, sessionID, model string) SummarizeFn {
	return func(ctx context.Context, msgs []map[string]any) (string, error) {
		src := make([]Message, 0, len(msgs))
		for _, m := range msgs {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			src = append(src, Message{Role: role, Content: content})
		}
		return CompactRequest(ctx, infer.GatewayURL, infer.AuthToken, sessionID, model, src)
	}
}
