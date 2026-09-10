package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// MoA 配置（2026-08-12 定稿——白眼 zerg-baiyan）
// 参考模型：gemma-4-26B + Qwable（双 MoE——无工具纯文本分析，温度 0.7）
// 聚合器：example-35b（=主模型，带工具——最终决策）

// MoAConfig MoA 参考/聚合配置。
type MoAConfig struct {
	ReferenceModels []string      `json:"reference_models"` // 参考模型列表（并行 fan-out）
	AggregatorModel string        `json:"aggregator_model"` // 聚合器（=主模型）
	RefMaxTokens    int           `json:"ref_max_tokens"`   // 参考输出截断（[:2000]）
	RefTimeout      time.Duration `json:"ref_timeout"`      // 参考超时
	RefTemperature  float64       `json:"ref_temperature"`  // 参考温度（0.7 多样性）
	MinSuccessful   int           `json:"min_successful"`   // MIN_SUCCESSFUL_REFERENCES=1
	MaxRetries      int           `json:"max_retries"`      // 参考重试（2 次退避）
}

// DefaultMoAConfig 默认 MoA 配置（用户 2026-08-12 指定模型选型）。
func DefaultMoAConfig() *MoAConfig {
	return &MoAConfig{
		ReferenceModels: []string{
			"gemma-4-26B",
			"example-8b-quant",
		},
		AggregatorModel: "example-35b",
		RefMaxTokens:    2000,
		RefTimeout:      3 * time.Minute,
		RefTemperature:  0.7,
		MinSuccessful:   1,
		MaxRetries:      2,
	}
}

// MoAReference 单个参考模型输出。
type MoAReference struct {
	Model   string // 模型名
	Output  string // 参考输出（截断后）
	Success bool   // 是否成功
	Latency float64
}

// FanOutMoA 跑所有参考模型（2026-08-12 改：串行队列——不抢算力）。
// 用户指示：怕互相抢算力——参考模型逐个跑（gemma 完 → Qwable 完 → 聚合）
// 牺牲并行延迟，换取每个模型全速（本地资源有限——三模型同时抢 GPU 都慢）
func (o *Orchestrator) FanOutMoA(ctx context.Context, request string) []MoAReference {
	moaCfg := DefaultMoAConfig()
	refs := make([]MoAReference, 0, len(moaCfg.ReferenceModels))

	for _, model := range moaCfg.ReferenceModels {
		start := time.Now()
		out, err := o.callReferenceWithRetry(ctx, model, request, moaCfg)
		latency := time.Since(start).Seconds()
		refs = append(refs, MoAReference{
			Model:   model,
			Output:  out,
			Success: err == nil,
			Latency: latency,
		})
		status := "OK"
		if err != nil {
			status = "FAIL: " + err.Error()
		}
		log.Printf("[moa] 参考 %s 完成 (%.1fs) %s", model, latency, status)
	}

	// 失败容错：MIN_SUCCESSFUL_REFERENCES=1
	successful := 0
	for _, r := range refs {
		if r.Success {
			successful++
		}
	}
	if successful < moaCfg.MinSuccessful {
		log.Printf("[moa] 参考成功 %d/%d < MIN_SUCCESSFUL=%d——退化单模型", successful, len(refs), moaCfg.MinSuccessful)
	}
	return refs
}

// callReferenceWithRetry 参考模型调用（重试 2 次退避——Anthropic 可靠性实践）。
func (o *Orchestrator) callReferenceWithRetry(ctx context.Context, model string, request string, cfg *MoAConfig) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second): // 退避
			}
		}
		// 参考模型无工具 schema——纯文本分析（Hermes 模式）
		// 请求格式：用户问题 + 参考角色提示（proposer——produce useful reference responses）
		prompt := fmt.Sprintf(`你是虫族模型群中的参考分析者（proposer）。请对下面的用户请求给出你的独立分析视角。
不需要调用任何工具，只需要用文本给出你的回答/思路/要点。注意给出与常见答案不同的视角，可以补充、质疑、拓展。

用户请求:
%s`, request)

		refCtx, cancel := context.WithTimeout(ctx, cfg.RefTimeout)
		resp, err := o.executor.ExecuteWithResponse(refCtx, model, prompt, cfg.RefMaxTokens)
		cancel()
		if err == nil {
			// 截断 [:2000]（MoA 实践——上下文预算）
			out := resp.Content
			if resp.Reasoning != "" {
				out = resp.Reasoning
			}
			if len(out) > cfg.RefMaxTokens {
				out = out[:cfg.RefMaxTokens]
			}
			return strings.TrimSpace(out), nil
		}
		lastErr = err
		log.Printf("[moa] 参考 %s 第 %d 次失败: %v", model, attempt+1, err)
	}
	return "", lastErr
}

// AggregateMoA 聚合器（ornith = 主模型）——收到参考输出 → 最终输出 + 工具决策。
// 对齐 MoA 论文聚合 prompt：critically evaluate... synthesize into single high-quality response。
func (o *Orchestrator) AggregateMoA(ctx context.Context, request string, refs []MoAReference, maxTokens int) (*ModelResponse, error) {
	moaCfg := DefaultMoAConfig()

	// 拼接参考输出（Reference 1/2 标签）
	var joined strings.Builder
	joined.WriteString("You have been provided with a set of responses from various models to the latest user query. ")
	joined.WriteString("Your task is to synthesize these responses into a single, high-quality response. ")
	joined.WriteString("It is crucial to critically evaluate the information provided in these responses, recognizing that some of it may be biased or incorrect. ")
	joined.WriteString("Your response should not simply replicate the given answers but should offer a refined, accurate, and comprehensive reply to the instruction.\n\n")
	joined.WriteString("Responses from models:\n")
	idx := 1
	for _, r := range refs {
		if !r.Success {
			joined.WriteString(fmt.Sprintf("\nReference %d — %s: [unavailable]\n", idx, r.Model))
		} else {
			joined.WriteString(fmt.Sprintf("\nReference %d — %s:\n%s\n", idx, r.Model, r.Output))
		}
		idx++
	}
	joined.WriteString(fmt.Sprintf("\n\n用户请求:\n%s\n\n请输出最终综合回答。", request))

	aggCtx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()
	return o.executor.ExecuteWithResponse(aggCtx, moaCfg.AggregatorModel, joined.String(), maxTokens)
}
