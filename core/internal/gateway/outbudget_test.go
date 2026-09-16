package gateway

import "testing"

// 设计稿 E1/E2/E5/E6：动态、保守、超限即拒（不静默、不退回写死值）
func TestDynamicMaxTokens(t *testing.T) {
	// E1 提示很短 ⇒ 额度充足且不超过上限
	out, ctx, pe := DynamicMaxTokens("Qwen3.8-27B", 65536, 1000)
	if out <= 0 || out > maxOutCeiling {
		t.Errorf("E1 额度异常：out=%d（ctx=%d prompt≈%d）", out, ctx, pe)
	}
	if out > 65536-6553 {
		t.Errorf("E1 超过上限：out=%d", out)
	}
	// E2 提示超限 ⇒ 返回 0（调用方必须拒绝，不得静默截断）
	out2, _, _ := DynamicMaxTokens("Qwen3.8-27B", 8192, 8000)
	if out2 != 0 {
		t.Errorf("E2 应返回 0（提示已超上下文），实际 %d", out2)
	}
	// E5 未声明上下文 ⇒ 走保守默认（不得假设 256k）
	out3, ctx3, _ := DynamicMaxTokens("Qwen3.8-27B", 0, 1000)
	if ctx3 != defaultCtxWindow {
		t.Errorf("E5 应使用保守默认 ctx=%d，实际 %d", defaultCtxWindow, ctx3)
	}
	if out3 <= 0 || out3 > maxOutCeiling {
		t.Errorf("E5 额度异常：%d", out3)
	}
	// 思考型放大：同参数下 qwen 的额度不应小于 gemma
	q, _, _ := DynamicMaxTokens("qwen3.8-27b", 65536, 1000)
	g, _, _ := DynamicMaxTokens("gemma-4-26B", 65536, 1000)
	if q < g {
		t.Errorf("思考型应放大：qwen=%d gemma=%d", q, g)
	}
	// E6 估算兜底：字节数 → tokens 单调不减且 0 安全
	if EstimatePromptTokens(0) != 0 || EstimatePromptTokens(4000) <= 0 {
		t.Errorf("E6 估算异常：%d %d", EstimatePromptTokens(0), EstimatePromptTokens(4000))
	}
}
